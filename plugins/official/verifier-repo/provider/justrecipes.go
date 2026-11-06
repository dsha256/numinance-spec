package provider

import (
	"regexp"
	"strings"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/catalog"
)

// justfileName is the path of the Justfile the recipes are read from: the repository root's. A Justfile that imports another
// file or declares a module is read as far as its own text goes, and the finding says so.
const justfileName = "Justfile"

var (
	recipeHeaderRe = regexp.MustCompile(`^@?([A-Za-z_][A-Za-z0-9_-]*)(?:[ \t]|:|$)`)
	aliasRe        = regexp.MustCompile(`^alias[ \t]+([A-Za-z_][A-Za-z0-9_-]*)[ \t]*:=`)
	// justCmdRe matches `just <recipe>` where a command can start: the beginning, or after &&, ||, ;, | or ( .
	justCmdRe = regexp.MustCompile(`(?:^|&&|\|\||[;|(])[ \t]*just[ \t]+(\S+)`)
	recipeRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*`)
)

// recipes is what a Justfile defines.
type recipes struct {
	names map[string]bool
	// partial is true when the Justfile imports a file or declares a module, whose recipes are not read.
	partial bool
}

// parseRecipes reads the recipe names of a Justfile: a header at column zero (a name, its parameters, then a colon that is not
// the start of an assignment) and each alias. Recipe bodies are indented and are not read.
func parseRecipes(justfile string) recipes {
	r := recipes{names: map[string]bool{}}
	open := "" // the delimiter of a multi-line string literal that is still open: its lines are text, never headers
	for line := range strings.SplitSeq(justfile, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if open != "" {
			open = closeString(line, open)
			continue
		}
		if line == "" || strings.ContainsAny(line[:1], " \t#[") {
			continue
		}
		r.record(line)
		open = openString(line)
	}
	return r
}

// record reads one line at column zero, outside any string: an import, a module, an alias or a recipe header.
func (r *recipes) record(line string) {
	if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "mod ") {
		r.partial = true
		return
	}
	if m := aliasRe.FindStringSubmatch(line); m != nil {
		r.names[m[1]] = true
		return
	}
	if hasStatementPrefix(line) {
		return
	}
	m := recipeHeaderRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	// `name := value` is an assignment; the header colon is the first colon and is not followed by "=".
	if i := strings.IndexByte(line, ':'); i < 0 || strings.HasPrefix(line[i:], ":=") {
		return
	}
	r.names[m[1]] = true
}

// openString scans a line for string literals and a comment, and returns the triple delimiter of a literal the line opens and does not
// close, or "". A single-quoted, double-quoted or backtick string ends on its own line; text after `#` is a comment.
func openString(line string) string {
	for i := 0; i < len(line); {
		if line[i] == '#' {
			return ""
		}
		if d := tripleAt(line[i:]); d != "" {
			return closeString(line[i+len(d):], d)
		}
		if q := line[i]; q == '\'' || q == '"' || q == '`' {
			i = endOfString(line, i)
			continue
		}
		i++
	}
	return ""
}

// tripleAt returns the triple delimiter s starts with, or "".
func tripleAt(s string) string {
	for _, d := range [...]string{`'''`, `"""`, "```"} { // the delimiters of a Justfile string that can run over several lines
		if strings.HasPrefix(s, d) {
			return d
		}
	}
	return ""
}

// endOfString returns the index after the single-line string that starts at line[i]; a double-quoted string honours backslash
// escapes. An unterminated string runs to the end of the line.
func endOfString(line string, i int) int {
	q := line[i]
	for j := i + 1; j < len(line); j++ {
		switch {
		case q == '"' && line[j] == '\\':
			j++
		case line[j] == q:
			return j + 1
		}
	}
	return len(line)
}

// closeString scans a line inside a string opened with delim: it returns "" when the line closes it and no other multi-line string
// opens after the close, the delimiter of one that does, and delim when the string runs on.
func closeString(line, delim string) string {
	for i := 0; i < len(line); i++ {
		if delim[0] == '"' && line[i] == '\\' {
			i++
			continue
		}
		if strings.HasPrefix(line[i:], delim) {
			return openString(line[i+len(delim):])
		}
	}
	return delim
}

// hasStatementPrefix reports whether the line starts a Justfile statement that is not a recipe.
func hasStatementPrefix(line string) bool {
	for _, s := range [...]string{"set ", "export ", "unexport ", "import ", "mod ", "alias "} {
		if strings.HasPrefix(line, s) {
			return true
		}
	}
	return false
}

// checkJustRecipes reports every `just <recipe>` a changed document names in a code block or a code span that the Justfile
// does not define. Prose is not read: "just" is an ordinary word. A line whose first word after `just` is an option, and a
// placeholder in capitals, are not recipe names and are skipped.
func checkJustRecipes(docPath string, content []byte, r recipes) []finding {
	var out []finding
	report := func(n int, name string) {
		msg := "the document names `just " + name + "`, which the Justfile does not define"
		if r.partial {
			msg += " (imports and modules are not followed)"
		}
		out = append(out, finding{rule: catalog.RuleJustRecipe, severity: sevHigh, path: docPath, line: uint32(n), message: msg}) //nolint:gosec // a line number of a document of at most 1 MiB
	}
	scan := func(n int, text string) {
		for _, name := range recipeNames(text) {
			if !r.names[name] {
				report(n, name)
			}
		}
	}
	var f fence
	for i, line := range splitLines(content) {
		n := i + 1
		wasOpen := f.open
		if f.step(line, n) {
			if wasOpen && f.open {
				scan(n, trimPrompt(strings.TrimLeft(line, " \t")))
			}
			continue
		}
		_, spans := codeSpans(line)
		for _, s := range spans {
			scan(n, strings.TrimSpace(s))
		}
	}
	return out
}

// trimPrompt drops a shell prompt from the start of a code line.
func trimPrompt(s string) string {
	if strings.HasPrefix(s, "#") {
		return ""
	}
	for _, p := range []string{"$ ", "> "} {
		if t, ok := strings.CutPrefix(s, p); ok {
			return strings.TrimSpace(t)
		}
	}
	return s
}

// recipeNames returns the recipe names `just` is asked to run in a line of shell.
func recipeNames(text string) []string {
	var out []string
	for _, m := range justCmdRe.FindAllStringSubmatch(text, -1) {
		word := m[1]
		if strings.HasPrefix(word, "-") {
			continue
		}
		name := recipeRe.FindString(word)
		if name == "" || name == strings.ToUpper(name) {
			continue
		}
		out = append(out, name)
	}
	return out
}
