package provider

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/catalog"
)

func TestCheckScope(t *testing.T) {
	tests := []struct {
		name  string
		files []file
		scope func(*Scope)
		want  []string // the rule ids, sorted; nil means clean
	}{
		{"one new file under the allowed path is clean", []file{add("docs/runbooks/local.md", lines(50))}, nil, nil},
		{"a path outside the allowed paths", []file{add("docs/other.md", "x\n")}, nil, []string{catalog.RuleScopePath}},
		{"a protected path, also outside the allowed paths", []file{add(".claude/rules/x.md", "x\n")}, nil, []string{catalog.RuleScopePath, catalog.RuleScopeProtected}},
		{"a protected path under an allowed one", []file{add("docs/runbooks/x.md", "x\n")}, func(s *Scope) { s.ProtectedPaths = []string{"docs/runbooks/"} }, []string{catalog.RuleScopeProtected}},
		{"a protected path in another case", []file{add("DOCS/Design/x.md", "x\n")}, nil, []string{catalog.RuleScopePath, catalog.RuleScopeProtected}},
		{"an allowed prefix is a whole segment", []file{add("docs/runbooks-extra/x.md", "x\n")}, nil, []string{catalog.RuleScopePath}},
		{"an exact allowed file", []file{add("docs/one.md", "x\n")}, func(s *Scope) { s.AllowPaths = []string{"docs/one.md"} }, nil},
		{"an exact allowed file is not a prefix", []file{add("docs/one.md/x", "x\n")}, func(s *Scope) { s.AllowPaths = []string{"docs/one.md"} }, []string{catalog.RuleScopePath}},
		{"a deleted file when deletions are forbidden", []file{del("docs/runbooks/gone.md")}, nil, []string{catalog.RuleScopeDeleted}},
		{"a deletion within the bound", []file{del("docs/runbooks/gone.md")}, func(s *Scope) { s.MaxDeletedFiles = ptr(1) }, nil},
		{"more added lines than the bound", []file{add("docs/runbooks/long.md", lines(51))}, nil, []string{catalog.RuleScopeAddedLines}},
		{"exactly the bound of added lines", []file{add("docs/runbooks/long.md", lines(50))}, nil, nil},
		{"more files than the bound", []file{add("docs/runbooks/a.md", "x\n"), add("docs/runbooks/b.md", "x\n")}, nil, []string{catalog.RuleScopeFiles}},
		{"a modified file where only new files are allowed", []file{mod("docs/runbooks/a.md", "x\n")}, nil, []string{catalog.RuleScopeModified}},
		{"a modified file where modification is allowed", []file{mod("docs/runbooks/a.md", "x\n")}, func(s *Scope) { s.AllowModify = true }, nil},
		{"a symbolic link", []file{{path: "docs/runbooks/l", status: "A", mode: "120000", link: "../../x"}}, nil, []string{catalog.RuleScopeSymlink}},
		{"a deletion counts as a changed file", []file{add("docs/runbooks/a.md", "x\n"), del("docs/runbooks/b.md")}, func(s *Scope) { s.MaxDeletedFiles = ptr(5) }, []string{catalog.RuleScopeFiles}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := scope()
			if tc.scope != nil {
				tc.scope(&s)
			}
			got := rules(checkScope(parse(t, tc.files...), s))
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("checkScope() rules = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckScope_AProtectedPathIsCritical(t *testing.T) {
	fs := checkScope(parse(t, add(".claude/x.md", "x\n")), scope())
	for _, f := range fs {
		if f.rule == catalog.RuleScopeProtected && f.severity != sevCritical {
			t.Errorf("%s severity = %v, want critical", f.rule, f.severity)
		}
	}
}

func treeOf(paths ...string) tree { return newTree(paths, nil) }

func TestCheckMarkdown(t *testing.T) {
	base := treeOf("README.md", "docs/adr/0001.md", "docs/guide/intro.md", "Justfile")
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"balanced backtick fences", "# t\n\n```sh\njust check\n```\n", nil},
		{"balanced tilde fences", "~~~\nx\n~~~\n", nil},
		{"a fence that is never closed", "text\n```go\ncode\n", []string{catalog.RuleMarkdownFence}},
		{"a longer closing fence closes", "```\nx\n`````\n", nil},
		{"a shorter fence does not close", "````\nx\n```\n", []string{catalog.RuleMarkdownFence}},
		{"a closing fence of another character does not close", "```\nx\n~~~\n", []string{catalog.RuleMarkdownFence}},
		{"a closing fence with text after it does not close", "```\nx\n``` y\n", []string{catalog.RuleMarkdownFence}},
		{"three backticks in an inline span are not a fence", "see ```x``` here\n", nil},
		{"an indented fence in a list closes", "- item\n  ```\n  x\n  ```\n", nil},
		{"a resolving relative link", "[a](../adr/0001.md) [b](intro.md#top) [r](/README.md)\n", nil},
		{"a link to a directory that holds files", "[a](../adr/) [b](../adr)\n", nil},
		{"a broken relative link", "[a](missing.md)\n", []string{catalog.RuleMarkdownLink}},
		{"a broken link to a path with an anchor", "[a](missing.md#top)\n", []string{catalog.RuleMarkdownLink}},
		{"a broken image", "![a](img/none.png)\n", []string{catalog.RuleMarkdownLink}},
		{"a broken reference definition", "[a]: missing.md\n", []string{catalog.RuleMarkdownLink}},
		{"a link that leaves the repository", "[a](../../../etc/passwd)\n", []string{catalog.RuleMarkdownLink}},
		{"a root link that leaves the repository", "[a](/../x)\n", []string{catalog.RuleMarkdownLink}},
		{"an escaped path is decoded", "[a](../guide/intro%2Emd) [b](bad%zz)\n", []string{catalog.RuleMarkdownLink}},
		{"external links are not followed", "[a](https://example.com/x) [b](mailto:a@b.c) [c](//host/x) [d](#frag) [e]()\n", nil},
		{"a link in a code span is not checked", "use `[a](missing.md)` here\n", nil},
		{"a link in a fenced block is not checked", "```\n[a](missing.md)\n```\n", nil},
		{"a destination in angle brackets", "[a](<missing file.md>)\n", []string{catalog.RuleMarkdownLink}},
		{"a destination with a title", "[a](intro.md \"title\") [b](gone.md 'title')\n", []string{catalog.RuleMarkdownLink}},
		{"balanced parentheses in a destination", "[a](intro.md) [b](gone_(x).md)\n", []string{catalog.RuleMarkdownLink}},
		{"crlf line endings", "```\r\nx\r\n```\r\n[a](missing.md)\r\n", []string{catalog.RuleMarkdownLink}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rules(checkMarkdown("docs/guide/doc.md", []byte(tc.body), base))
			if !slices.Equal(got, tc.want) {
				t.Errorf("checkMarkdown() rules = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckMarkdown_LinksResolveAgainstTheTreeTheCandidateLeaves(t *testing.T) {
	changes := []Change{{Path: "docs/guide/new.md", Status: "A"}, {Path: "docs/adr/0001.md", Status: "D"}}
	tr := newTree([]string{"docs/adr/0001.md", "docs/adr/0002.md"}, changes)
	got := rules(checkMarkdown("docs/guide/doc.md", []byte("[new](new.md) [gone](../adr/0001.md) [kept](../adr/0002.md)\n"), tr))
	if !slices.Equal(got, []string{catalog.RuleMarkdownLink}) {
		t.Errorf("checkMarkdown() rules = %v, want only the link to the deleted file broken", got)
	}
}

func TestCheckMarkdown_FindingsNameTheLine(t *testing.T) {
	fs := checkMarkdown("a.md", []byte("ok\n\n[a](gone.md)\n```\n"), treeOf())
	if len(fs) != 2 || fs[0].line != 3 || fs[1].line != 4 || fs[0].path != "a.md" {
		t.Errorf("checkMarkdown() = %+v, want the link at line 3 and the fence at line 4 of a.md", fs)
	}
}

const testJustfile = `set shell := ["bash", "-c"]
core := "numinance"
has := "find . : x"

# comment
default:
    @just --list

check: lint test
    echo hi

[positional-arguments]
adr title:
    echo {{title}}

@quiet:
    echo

build target="a b":
    echo

alias b := build
export FOO := "bar"
`

func TestParseRecipes(t *testing.T) {
	r := parseRecipes(testJustfile)
	for _, name := range []string{"default", "check", "adr", "quiet", "build", "b"} {
		if !r.names[name] {
			t.Errorf("parseRecipes() lacks recipe %q, has %v", name, r.names)
		}
	}
	for _, name := range []string{"set", "core", "has", "FOO", "export", "echo", "hi", "positional-arguments", "lint"} {
		if r.names[name] {
			t.Errorf("parseRecipes() has %q, which is not a recipe", name)
		}
	}
	if r.partial {
		t.Errorf("parseRecipes() partial = true for a Justfile with no import or module")
	}
	if !parseRecipes("import 'x.just'\nfoo:\n").partial || !parseRecipes("mod x\n").partial {
		t.Errorf("parseRecipes() partial = false for an import or a module, want true")
	}
}

func TestCheckJustRecipes(t *testing.T) {
	r := parseRecipes(testJustfile)
	tests := []struct {
		name string
		body string
		want int // the findings
	}{
		{"a recipe in a code span", "run `just check` first\n", 0},
		{"a missing recipe in a code span", "run `just nothere` first\n", 1},
		{"a recipe in a fenced block", "```sh\njust check\njust adr \"Title\"\n```\n", 0},
		{"a missing recipe in a fenced block", "```sh\n$ just gone\n```\n", 1},
		{"a chain of commands", "`just build && just nope`\n", 1},
		{"a pipeline and a subshell", "```\nfoo | just gone1\n(just gone2)\nx; just gone3\n```\n", 3},
		{"an option is not a recipe", "`just --list` and `just -f x build` and `just --justfile y gone`\n", 0},
		{"a placeholder in capitals and in angle brackets", "`just RECIPE` and `just <recipe>`\n", 0},
		{"prose is not read", "just run it, just check the docs, just gone\n", 0},
		{"a comment line in a fenced block", "```\n# just gone\n```\n", 0},
		{"just in the middle of a word", "```\nunjust gone\n```\n", 0},
		{"an alias", "`just b`\n", 0},
		{"a recipe followed by punctuation", "`just gone.`\n", 1},
		{"the same missing recipe twice is two findings on two lines", "`just gone`\n`just gone`\n", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkJustRecipes("docs/x.md", []byte(tc.body), r)
			if len(got) != tc.want {
				t.Errorf("checkJustRecipes() = %d findings %v, want %d", len(got), rules(got), tc.want)
			}
			for _, f := range got {
				if f.rule != catalog.RuleJustRecipe || f.path != "docs/x.md" || f.line == 0 {
					t.Errorf("finding %+v: want rule %s at a line of docs/x.md", f, catalog.RuleJustRecipe)
				}
			}
		})
	}
}

func TestCheckJustRecipes_APartialJustfileSaysSo(t *testing.T) {
	r := parseRecipes("import 'other.just'\n")
	got := checkJustRecipes("docs/x.md", []byte("`just gone`\n"), r)
	if len(got) != 1 || !strings.Contains(got[0].message, "imports") {
		t.Errorf("checkJustRecipes() = %+v, want one finding that says imports are not followed", got)
	}
}

func TestCodeSpans(t *testing.T) {
	tests := []struct {
		line, rest string
		spans      []string
	}{
		{"no code", "no code", nil},
		{"a `b` c", "a  c", []string{"b"}},
		{"a ``b ` c`` d", "a  d", []string{"b ` c"}},
		{"a `b", "a `b", nil},
		{"`a` and `b`", " and ", []string{"a", "b"}},
		{"``a` `b``", "", []string{"a` `b"}},
		{"```x``` `y`", " ", []string{"x", "y"}},
		{"` ``", "` ``", nil},
	}
	for _, tc := range tests {
		rest, spans := codeSpans(tc.line)
		if rest != tc.rest || !slices.Equal(spans, tc.spans) {
			t.Errorf("codeSpans(%q) = %q, %q, want %q, %q", tc.line, rest, spans, tc.rest, tc.spans)
		}
	}
}

// A line of backtick runs of every length, none matching another, once cost a rescan to the end of the line for each run.
func TestCodeSpans_ALineOfUnmatchedRunsIsLinear(t *testing.T) {
	var b strings.Builder
	for n := 1; n <= 1400; n++ {
		b.WriteString(strings.Repeat("`", n))
		b.WriteString("a")
	}
	line := b.String()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if rest, spans := codeSpans(line); len(spans) != 0 || rest != line {
			t.Errorf("codeSpans(unmatched runs) found %d spans, want none and the line unchanged", len(spans))
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("codeSpans on a line of 1400 unmatched backtick runs did not finish in 5s: the pairing is not linear")
	}
}
