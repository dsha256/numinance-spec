package cli

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Bounds on what "numi suggest" reads. They are fixed so the same repository gives the same answer, and so a hostile
// repository cannot make the scan large or slow.
const (
	maxScanEntries   = 20000
	maxScanFiles     = 2000
	maxScanFileBytes = 256 << 10
	maxScanBytes     = 32 << 20
	maxScanLines     = 20000
	maxSigLines      = 12
	maxTodoRunes     = 120
	maxIdentBytes    = 60
)

// Finding kinds, in the order the selection prefers them.
const (
	kindUntested  = "untested-function"
	kindTypes     = "missing-type-hints"
	kindDocstring = "missing-docstring"
	kindTodo      = "todo-comment"
)

func findingKinds() []string { return []string{kindUntested, kindTypes, kindDocstring, kindTodo} }

// finding is one signal found in one Python source file. Every text field is repository data and untrusted: Name is an
// ASCII identifier by construction, Text is cut and escaped when it is rendered.
type finding struct {
	Kind string
	// File is the repository-relative slash path; Line is 1-based.
	File string
	Line int
	// Name is the function name, or the marker (TODO or FIXME) of a comment.
	Name string
	// Detail says what is missing (type hints) or holds the comment text (TODO).
	Detail string
}

// scanResult is what one scan saw.
type scanResult struct {
	Findings []finding
	Files    int // Python files read
	Sources  int
	Tests    int
	Bytes    int64
	// Truncated is set when a bound stopped the scan early.
	Truncated bool
	// TestDir is "tests" or "test": where a new test file goes.
	TestDir string
}

var (
	identRe  = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	defRe    = regexp.MustCompile(`(?s)^(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)\s*(->\s*.+?)?\s*:$`)
	todoRe   = regexp.MustCompile(`#\s*(TODO|FIXME)\b[:(]?\s*(.*)$`)
	safePath = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,200}$`)
)

// skipDir reports a directory name never entered, besides every name that starts with a dot.
func skipDir(name string) bool {
	switch name {
	case "venv", "node_modules", "site-packages", "__pycache__", "build", "dist", "vendor", "third_party":
		return true
	}
	return strings.HasSuffix(name, ".egg-info")
}

// notSourceFile names build or test entry points, which are not the public API of the project.
func notSourceFile(base string) bool {
	switch base {
	case "setup.py", "noxfile.py", "conftest.py", "__init__.py", "__main__.py":
		return true
	}
	return false
}

// scan walks the repository read-only. It never follows a link, never enters a hidden or vendored directory, reads each
// file through a size-limited reader, never imports or runs repository code, and stops at fixed bounds. The result is
// deterministic: the walk is in lexical order and the findings are sorted.
func scan(ctx context.Context, root *os.Root) (scanResult, error) {
	var res scanResult
	testTokens := map[string]bool{}
	entries := 0
	walkErr := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if err != nil {
			// An unreadable entry is skipped, not fatal.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		entries++
		if entries > maxScanEntries {
			res.Truncated = true
			return fs.SkipAll
		}
		name := d.Name()
		if p != "." && strings.HasPrefix(name, ".") {
			return skipEntry(d)
		}
		if d.IsDir() {
			if p != "." && (skipDir(name) || p == "docs/design") {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(name, ".py") {
			return nil
		}
		if res.Files >= maxScanFiles || res.Bytes >= maxScanBytes {
			res.Truncated = true
			return fs.SkipAll
		}
		text, ok := readSourceFile(root, p)
		if !ok {
			return nil
		}
		res.Files++
		res.Bytes += int64(len(text))
		if isTestFile(p) {
			res.Tests++
			for _, tok := range identRe.FindAllString(text, -1) {
				testTokens[tok] = true
			}
			return nil
		}
		if isSourceFile(p) {
			res.Sources++
			res.Findings = append(res.Findings, scanSource(p, text)...)
		}
		return nil
	})
	if walkErr != nil {
		return scanResult{}, walkErr
	}
	// A public function whose name appears in no test file has no test that can call it.
	kept := res.Findings[:0]
	for _, f := range res.Findings {
		if f.Kind == kindUntested && testTokens[f.Name] {
			continue
		}
		kept = append(kept, f)
	}
	res.Findings = kept
	sort.SliceStable(res.Findings, func(i, j int) bool {
		a, b := res.Findings[i], res.Findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	res.TestDir = "tests"
	if st, err := root.Lstat("tests"); err != nil || !st.IsDir() {
		if st, err := root.Lstat("test"); err == nil && st.IsDir() {
			res.TestDir = "test"
		}
	}
	return res, nil
}

func skipEntry(d fs.DirEntry) error {
	if d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

// readSourceFile reads one regular, non-link file of at most maxScanFileBytes and returns it when it is valid UTF-8.
func readSourceFile(root *os.Root, name string) (string, bool) {
	li, err := root.Lstat(name)
	if err != nil || !li.Mode().IsRegular() || li.Size() > maxScanFileBytes {
		return "", false
	}
	f, err := root.Open(name)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	if fi, err := f.Stat(); err != nil || !os.SameFile(li, fi) {
		return "", false
	}
	b, err := io.ReadAll(io.LimitReader(f, maxScanFileBytes+1))
	if err != nil || len(b) > maxScanFileBytes || !utf8.Valid(b) {
		return "", false
	}
	return string(b), true
}

func isTestFile(p string) bool {
	base := path.Base(p)
	if base == "conftest.py" || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	return hasSegment(path.Dir(p), "tests", "test")
}

func isSourceFile(p string) bool {
	if notSourceFile(path.Base(p)) || hasSegment(path.Dir(p), "docs", "doc", "examples", "scripts", "tests", "test") {
		return false
	}
	return true
}

func hasSegment(dir string, names ...string) bool {
	for seg := range strings.SplitSeq(dir, "/") {
		for _, n := range names {
			if seg == n {
				return true
			}
		}
	}
	return false
}

// scanSource finds the signals in one source file. It reads text only: top-level "def" lines, the lines after them and
// TODO or FIXME comments. A decorated function is skipped because a decorator changes what a test or a hint means.
func scanSource(file, text string) []finding {
	var out []finding
	if !safePath.MatchString(file) {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxScanLines {
		lines = lines[:maxScanLines]
	}
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if m := todoRe.FindStringSubmatch(line); m != nil {
			out = append(out, finding{Kind: kindTodo, File: file, Line: i + 1, Name: m[1], Detail: m[2]})
		}
		if !strings.HasPrefix(line, "def ") && !strings.HasPrefix(line, "async def ") {
			continue
		}
		if i > 0 && strings.HasPrefix(lines[i-1], "@") {
			continue
		}
		sig, end := signature(lines, i)
		m := defRe.FindStringSubmatch(sig)
		if len(m) < 4 || strings.HasPrefix(m[1], "_") || len(m[1]) > maxIdentBytes {
			continue
		}
		out = append(out, finding{Kind: kindUntested, File: file, Line: i + 1, Name: m[1]})
		if missing := missingHints(m[2], m[3] != ""); missing != "" {
			out = append(out, finding{Kind: kindTypes, File: file, Line: i + 1, Name: m[1], Detail: missing})
		}
		if !hasDocstring(lines, end+1) {
			out = append(out, finding{Kind: kindDocstring, File: file, Line: i + 1, Name: m[1]})
		}
	}
	return out
}

// signature joins the lines of a def from lines[i] to the first line that ends with a colon (comments cut), at most
// maxSigLines. It returns the joined text and the index of the last line used. Text that never ends in a colon cannot
// match defRe.
func signature(lines []string, i int) (string, int) {
	var b strings.Builder
	for j := i; j < len(lines) && j < i+maxSigLines; j++ {
		l := strings.TrimSpace(strings.TrimRight(lines[j], "\r"))
		if k := strings.Index(l, " #"); k >= 0 {
			l = strings.TrimSpace(l[:k])
		}
		b.WriteString(l)
		b.WriteByte(' ')
		if strings.HasSuffix(l, ":") {
			return strings.TrimSpace(b.String()), j
		}
	}
	return "", i
}

// missingHints says which parameters lack an annotation, and whether the return does; empty when fully annotated.
func missingHints(params string, hasReturn bool) string {
	var missing []string
	for _, p := range splitTopLevel(params) {
		p = strings.TrimSpace(p)
		p = strings.TrimLeft(p, "*")
		if p == "" || p == "/" || p == "self" || p == "cls" {
			continue
		}
		name, _, _ := strings.Cut(p, "=")
		if !strings.Contains(name, ":") {
			id := identRe.FindString(name)
			if id == "" {
				id = "?"
			}
			missing = append(missing, "parameter "+id)
		}
	}
	if !hasReturn {
		missing = append(missing, "return type")
	}
	if len(missing) > 8 {
		missing = append(missing[:8], "more")
	}
	return strings.Join(missing, ", ")
}

// splitTopLevel splits a parameter list at commas outside brackets.
func splitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// hasDocstring reports whether the first non-blank line from index from starts a string literal.
func hasDocstring(lines []string, from int) bool {
	for j := from; j < len(lines) && j < from+4; j++ {
		l := strings.TrimSpace(lines[j])
		if l == "" {
			continue
		}
		l = strings.TrimLeft(l, "rRuU")
		return strings.HasPrefix(l, `"`) || strings.HasPrefix(l, "'")
	}
	return false
}
