package provider

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/catalog"
)

// tree is the tree the candidate would leave behind: the files of the base, less the deleted ones, plus the added and changed
// ones. A link resolves when it names a file of it or a directory that holds one.
type tree struct {
	files map[string]bool
	dirs  map[string]bool
}

// newTree overlays the candidate's changes on the base paths.
func newTree(base []string, changes []Change) tree {
	t := tree{files: make(map[string]bool, len(base)+len(changes)), dirs: map[string]bool{".": true}}
	for _, p := range base {
		t.files[p] = true
	}
	for _, c := range changes {
		if c.Status == statusDeleted {
			delete(t.files, c.Path)
			continue
		}
		t.files[c.Path] = true
	}
	for p := range t.files {
		for d := path.Dir(p); d != "." && !t.dirs[d]; d = path.Dir(d) {
			t.dirs[d] = true
		}
	}
	return t
}

func (t tree) has(p string) bool { return t.files[p] || t.dirs[p] }

var (
	schemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	refDefRe = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:[ \t]*(\S+)`)
	// htmlRe matches the quoted href of an <a> and the quoted src of an <img>, the two raw HTML links a document carries.
	htmlRe = regexp.MustCompile(`(?i)<(?:a|img)\b[^>]*?\b(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// fence tracks the fenced code blocks of a Markdown document, line by line, as CommonMark reads them: a fence is opened and closed by a
// line indented at most three columns, so a line indented four or more is content, never a fence.
type fence struct {
	open  bool
	char  byte
	size  int
	start int
}

// indentOf returns the columns of leading blanks of a line (a tab goes to the next multiple of four) and the rest of the line.
func indentOf(line string) (int, string) {
	col := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return col, line[i:]
		}
	}
	return col, ""
}

// step advances the fence state over one line and reports whether the line is inside a code block (or is a fence line).
func (f *fence) step(line string, n int) bool {
	indent, t := indentOf(line)
	if f.open {
		if indent <= 3 && isClosingFence(t, f.char, f.size) {
			f.open = false
		}
		return true
	}
	if indent > 3 {
		return false
	}
	if c, size, ok := openingFence(t); ok {
		*f = fence{open: true, char: c, size: size, start: n}
		return true
	}
	return false
}

// openingFence reports whether t opens a fenced block: three or more backticks or tildes, and for backticks an info string with
// no backtick (else the line is an inline code span).
func openingFence(t string) (char byte, size int, ok bool) {
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return 0, 0, false
	}
	char = t[0]
	for size < len(t) && t[size] == char {
		size++
	}
	if size < 3 || (char == '`' && strings.ContainsRune(t[size:], '`')) {
		return 0, 0, false
	}
	return char, size, true
}

// isClosingFence reports whether t closes a block opened with size characters of char: at least as many, then only blanks.
func isClosingFence(t string, char byte, size int) bool {
	n := 0
	for n < len(t) && t[n] == char {
		n++
	}
	return n >= size && strings.TrimSpace(t[n:]) == ""
}

// splitLines splits a document at LF, CR and CRLF, the three line endings of Markdown.
func splitLines(content []byte) []string {
	s := strings.ReplaceAll(string(content), "\r\n", "\n")
	return strings.Split(strings.ReplaceAll(s, "\r", "\n"), "\n")
}

// backticks is a maximal run of backticks of a line.
type backticks struct{ at, size int }

// codeSpans returns the line with its inline code spans removed and the content of each span. A span opens at a run of backticks
// and closes at the next run of exactly as many; a run with no partner is literal text. The pairing is linear in the line, so a
// line built to defeat a rescan (runs of every length, none matching) costs no more than any other.
func codeSpans(line string) (rest string, spans []string) {
	var runs []backticks
	for i := 0; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		n := 0
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		runs = append(runs, backticks{at: i, size: n})
		i += n
	}
	if len(runs) < 2 {
		return line, nil
	}
	next := make([]int, len(runs))
	last := map[int]int{}
	for k := len(runs) - 1; k >= 0; k-- {
		next[k] = -1
		if j, ok := last[runs[k].size]; ok {
			next[k] = j
		}
		last[runs[k].size] = k
	}
	var b strings.Builder
	pos := 0
	for k := 0; k < len(runs); {
		j := next[k]
		if j < 0 {
			k++
			continue
		}
		b.WriteString(line[pos:runs[k].at])
		spans = append(spans, line[runs[k].at+runs[k].size:runs[j].at])
		pos = runs[j].at + runs[j].size
		k = j + 1
	}
	b.WriteString(line[pos:])
	return b.String(), spans
}

// destinations returns the link destinations of one line outside code: inline links and images "](dest)", and a reference
// definition "[label]: dest".
func destinations(line string) []string {
	var out []string
	if m := refDefRe.FindStringSubmatch(line); m != nil {
		out = append(out, strings.Trim(m[1], "<>"))
	}
	for _, m := range htmlRe.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1]+m[2])
	}
	for rest := line; ; {
		i := strings.Index(rest, "](")
		if i < 0 {
			return out
		}
		rest = rest[i+2:]
		dest, n := parseDestination(rest)
		if dest != "" {
			out = append(out, dest)
		}
		rest = rest[n:]
	}
}

// parseDestination reads the destination at the start of s (up to the ")" that closes the link) and returns it with the bytes
// consumed. A destination may be written in angle brackets and may hold balanced parentheses; a title after a blank is dropped.
func parseDestination(s string) (string, int) {
	s = strings.TrimLeft(s, " \t")
	if strings.HasPrefix(s, "<") {
		end := strings.IndexByte(s, '>')
		if end < 0 {
			return "", len(s)
		}
		return s[1:end], end + 1
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return s[:i], i + 1
			}
			depth--
		case ' ', '\t':
			return s[:i], i
		}
	}
	return "", len(s)
}

// checkMarkdown reports the unclosed code fence of a changed document and every relative link whose target is not in the tree.
// Anchors are not checked, and a link with a scheme or a protocol-relative one is external and is not followed.
func checkMarkdown(docPath string, content []byte, t tree) []finding {
	var out []finding
	var f fence
	for i, line := range splitLines(content) {
		n := i + 1
		if f.step(line, n) {
			continue
		}
		rest, _ := codeSpans(line)
		for _, d := range destinations(rest) {
			if msg := brokenLink(docPath, d, t); msg != "" {
				out = append(out, finding{rule: catalog.RuleMarkdownLink, severity: sevHigh, path: docPath, line: uint32(n), message: msg}) //nolint:gosec // a line number of a document of at most 1 MiB
			}
		}
	}
	if f.open {
		out = append(out, finding{rule: catalog.RuleMarkdownFence, severity: sevHigh, path: docPath, line: uint32(f.start), message: "the code fence is never closed"}) //nolint:gosec // see above
	}
	return out
}

// resolveLink resolves the destination d of the document docPath to a repository-relative target. A destination that is not ours to
// check (external, anchor-only, empty) gives no target and no problem; one that cannot resolve gives a problem.
func resolveLink(docPath, d string) (target, problem string) {
	if d == "" || strings.HasPrefix(d, "#") || strings.HasPrefix(d, "//") || schemeRe.MatchString(d) {
		return "", ""
	}
	if i := strings.IndexAny(d, "#?"); i >= 0 {
		d = d[:i]
	}
	if d == "" {
		return "", ""
	}
	p, err := url.PathUnescape(d)
	if err != nil {
		return "", "the link target is not a valid escaped path: " + d
	}
	if rest, root := strings.CutPrefix(p, "/"); root {
		target = path.Clean(rest)
	} else {
		target = path.Join(path.Dir(docPath), p)
	}
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", "the link leaves the repository: " + d
	}
	return target, ""
}

// brokenLink returns why the relative destination d of the document docPath does not resolve, or "" when it does or is not
// ours to check.
func brokenLink(docPath, d string, t tree) string {
	target, problem := resolveLink(docPath, d)
	switch {
	case problem != "":
		return problem
	case target != "" && !t.has(target):
		return "the link target does not exist in the candidate tree: " + d
	}
	return ""
}

// LinkTargets returns the repository-relative targets of the relative links of a Markdown document (inline links and images,
// reference definitions, and the href of an <a> and src of an <img>), outside code. It is how the binding loader records what
// the documents of the base tree link to (Context.Links); a link that is external or leaves the repository has no target.
func LinkTargets(docPath string, content []byte) []string {
	var out []string
	var f fence
	for i, line := range splitLines(content) {
		if f.step(line, i+1) {
			continue
		}
		rest, _ := codeSpans(line)
		for _, d := range destinations(rest) {
			if target, _ := resolveLink(docPath, d); target != "" {
				out = append(out, target)
			}
		}
	}
	return out
}
