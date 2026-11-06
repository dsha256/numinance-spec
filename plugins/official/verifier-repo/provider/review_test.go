package provider

import (
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/catalog"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
)

// These tests are the permanent form of the adversarial cases of the earlier review of this provider.

func TestFence_FollowsCommonMarkIndentation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"an opener indented three columns is a fence", "   ```\nx\n```\n", nil},
		{"an opener indented four columns is not a fence", "    ```\n[a](gone.md)\n", []string{catalog.RuleMarkdownLink}},
		{"a closer indented four columns does not close", "```\nx\n    ```\n", []string{catalog.RuleMarkdownFence}},
		{"a closer indented by a tab does not close", "```\nx\n\t```\n", []string{catalog.RuleMarkdownFence}},
		{"a closer indented three columns closes", "```\nx\n   ```\n[a](gone.md)\n", []string{catalog.RuleMarkdownLink}},
		{"an indented closer cannot hide the link after an unclosed fence", "```\n    ```\n[a](gone.md)\n", []string{catalog.RuleMarkdownFence}},
		{"a closer shorter than the opener does not close", "````\nx\n   ```\n", []string{catalog.RuleMarkdownFence}},
		{"a closer of another character does not close", "~~~\nx\n```\n", []string{catalog.RuleMarkdownFence}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules(checkMarkdown("docs/a.md", []byte(tc.body), treeOf())); !slices.Equal(got, tc.want) {
				t.Errorf("checkMarkdown() rules = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseRecipes_ReadsNoRecipeOutOfAString(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		want    []string
		notWant []string
	}{
		{"a triple single-quoted string", "x := '''\nfake:\n'''\nreal:\n", []string{"real"}, []string{"fake", "x"}},
		{"a triple double-quoted string", "x := \"\"\"\nfake:\n\"\"\"\nreal:\n", []string{"real"}, []string{"fake"}},
		{"a triple backtick string", "x := ```\nfake:\n```\nreal:\n", []string{"real"}, []string{"fake"}},
		{"a string that opens and closes on one line", "x := '''one'''\nreal:\n", []string{"real"}, nil},
		{"a string closed and another opened on one line", "x := '''a\n''' + '''b\nfake:\n'''\nreal:\n", []string{"real"}, []string{"fake"}},
		{"a hash inside a string is not a comment", "x := \"a # b '''\"\nreal:\n", []string{"real"}, nil},
		{"a hash starts a comment that hides a quote", "x := 'a' # '''\nreal:\n", []string{"real"}, nil},
		{"an escaped quote does not close a double-quoted triple string", "x := \"\"\"\na \\\"\"\" still inside\nfake:\n\"\"\"\nreal:\n", []string{"real"}, []string{"fake"}},
		{"a comment line is no recipe", "# fake:\nreal:\n", []string{"real"}, []string{"fake"}},
		{"a string that never closes hides what follows", "x := '''\nfake:\n", nil, []string{"fake"}},
		{"private recipes exist", "_helper:\n    echo\n\n[private]\nhidden:\n    echo\n", []string{"_helper", "hidden"}, nil},
		{"an alias", "build:\nalias b := build\n", []string{"build", "b"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRecipes(tc.file)
			for _, n := range tc.want {
				if !got.names[n] {
					t.Errorf("parseRecipes() lacks %q, has %v", n, got.names)
				}
			}
			for _, n := range tc.notWant {
				if got.names[n] {
					t.Errorf("parseRecipes() has %q, which only a string holds", n)
				}
			}
		})
	}
}

// A private recipe still runs (`just _x`), so a document that names it names a recipe that exists; only an absent one is a finding.
func TestCheckJustRecipes_APrivateRecipeExists(t *testing.T) {
	r := parseRecipes("_helper:\n    echo\n[private]\nhidden:\n    echo\n")
	if got := checkJustRecipes("a.md", []byte("`just _helper` `just hidden` `just _gone`\n"), r); len(got) != 1 || !strings.Contains(got[0].message, "_gone") {
		t.Errorf("checkJustRecipes() = %+v, want only `just _gone` reported", got)
	}
}

func TestJustfile_TheCandidatesOwnCountsOnlyInsideTheScope(t *testing.T) {
	cfg := func(allow, protected []string) Config {
		s := scope()
		s.AllowPaths, s.ProtectedPaths = allow, protected
		return Config{Scope: s, Context: Context{Justfile: "base"}}
	}
	tests := []struct {
		name  string
		files []file
		cfg   Config
		want  string
	}{
		{"unchanged: the base's", nil, cfg([]string{"Justfile"}, nil), "base"},
		{"modified inside the scope: the candidate's", []file{mod("Justfile", "own:\n")}, cfg([]string{"Justfile"}, nil), "own:\n"},
		{"modified outside the scope: the base's", []file{mod("Justfile", "own:\n")}, cfg([]string{"docs/"}, nil), "base"},
		{"modified under a protected path: the base's", []file{mod("Justfile", "own:\n")}, cfg([]string{"Justfile"}, []string{"justfile"}), "base"},
		{"deleted: none", []file{del("Justfile")}, cfg([]string{"Justfile"}, nil), ""},
	}
	for _, tc := range tests {
		if got := justfile(parse(t, tc.files...), tc.cfg); got != tc.want {
			t.Errorf("%s: justfile() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRunChecks_ADocumentOutsideTheCandidateScopeRecipeIsJudgedByTheBase(t *testing.T) {
	s := scope()
	s.MaxFiles, s.AllowPaths, s.AllowModify = ptr(2), []string{"docs/runbooks/", "Justfile"}, true
	s.ProtectedPaths = nil
	cfg := Config{Scope: s, Context: Context{Base: testBase, Justfile: "gen:\n"}}
	c := parse(t, add("docs/runbooks/a.md", "`just brandnew`\n"), mod("Justfile", "brandnew:\n"))
	if got := rules(runChecks(c, cfg)); len(got) != 0 {
		t.Errorf("runChecks(recipe defined in an allowed Justfile) = %v, want none", got)
	}
	cfg.Scope.AllowPaths = []string{"docs/runbooks/"}
	if got := rules(runChecks(c, cfg)); !slices.Contains(got, catalog.RuleJustRecipe) {
		t.Errorf("runChecks(recipe defined only in a Justfile the scope keeps out) = %v, want %s", got, catalog.RuleJustRecipe)
	}
}

func TestIsMarkdown_IgnoresCase(t *testing.T) {
	for p, want := range map[string]bool{"a.md": true, "a.MD": true, "a.Md": true, "a.markdown": true, "a.MARKDOWN": true, "a.mdx": false, "md": false, "a.txt": false} {
		if got := IsMarkdown(p); got != want {
			t.Errorf("IsMarkdown(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestRunChecks_ADocumentWithAnUpperCaseExtensionIsChecked(t *testing.T) {
	s := scope()
	s.AllowPaths, s.MaxFiles = []string{"docs/"}, ptr(5)
	cfg := Config{Scope: s, Context: Context{Base: testBase, Justfile: "check:\n"}}
	for _, name := range []string{"docs/x.MD", "docs/x.Markdown"} {
		got := rules(runChecks(parse(t, add(name, "[a](gone.md)\n```\nx\n`just nope`\n")), cfg))
		for _, want := range []string{catalog.RuleMarkdownLink, catalog.RuleMarkdownFence} {
			if !slices.Contains(got, want) {
				t.Errorf("runChecks(%s) rules = %v, want %s", name, got, want)
			}
		}
	}
}

func TestCheckDangling(t *testing.T) {
	links := map[string][]string{"docs/a.md": {"docs/b.md", "docs/c.md"}, "docs/d.md": {"docs/b.md"}}
	tests := []struct {
		name  string
		files []file
		want  []string // the documents named, in order
	}{
		{"a deleted file an unchanged document links to", []file{del("docs/a.md")}, []string{"docs/b.md", "docs/c.md"}},
		{"the linking document is deleted too", []file{del("docs/a.md"), del("docs/b.md")}, []string{"docs/c.md"}},
		{"the linking document is modified: its own check reads it", []file{del("docs/a.md"), mod("docs/b.md", "x\n")}, []string{"docs/c.md"}},
		{"a deleted file nothing links to", []file{del("docs/zzz.md")}, nil},
		{"nothing deleted", []file{add("docs/new.md", "x\n")}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, f := range checkDangling(parse(t, tc.files...), links) {
				if f.rule != catalog.RuleLinkDangling {
					t.Errorf("rule = %s, want %s", f.rule, catalog.RuleLinkDangling)
				}
				got = append(got, f.path)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("checkDangling() documents = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunChecks_ADeletionBreaksTheLinkOfAnUnchangedDocument(t *testing.T) {
	s := scope()
	s.AllowPaths, s.MaxFiles, s.MaxDeletedFiles = []string{"docs/"}, ptr(5), ptr(5)
	cfg := Config{Scope: s, Context: Context{Base: testBase, Paths: []string{"docs/a.md", "docs/b.md"}, Links: map[string][]string{"docs/a.md": {"docs/b.md"}}}}
	if got := rules(runChecks(parse(t, del("docs/a.md")), cfg)); !slices.Equal(got, []string{catalog.RuleLinkDangling}) {
		t.Errorf("runChecks(delete a linked file) rules = %v, want %s", got, catalog.RuleLinkDangling)
	}
}

func TestCheckMarkdown_RawHTMLLinksAreChecked(t *testing.T) {
	tr := treeOf("docs/ok.md", "img/ok.png")
	tests := []struct {
		name string
		body string
		want int
	}{
		{"a resolving href and src", "<a href=\"ok.md\">x</a> <img src='../img/ok.png' alt=y>\n", 0},
		{"a broken href", "<a href=\"gone.md\">x</a>\n", 1},
		{"a broken src", "<img alt=\"a\" src=\"../img/none.png\">\n", 1},
		{"an upper case tag and attribute", "<A HREF=\"gone.md\">x</A>\n", 1},
		{"external and anchor targets", "<a href=\"https://example.com\">x</a> <a href=\"#top\">y</a>\n", 0},
		{"two tags on a line", "<a href=\"gone1.md\">x</a> <a href=\"gone2.md\">y</a>\n", 2},
		{"in a code span", "`<a href=\"gone.md\">`\n", 0},
		{"in a fenced block", "```html\n<a href=\"gone.md\">x</a>\n```\n", 0},
		{"a tag that is not a link", "<div href=\"gone.md\">x</div>\n", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkMarkdown("docs/x.md", []byte(tc.body), tr); len(got) != tc.want {
				t.Errorf("checkMarkdown() = %d findings %v, want %d", len(got), rules(got), tc.want)
			}
		})
	}
}

func TestLinkTargets(t *testing.T) {
	doc := "[r]: c.md\n[a](b.md#x) ![i](../img/p.png)\n<a href=\"d.md\">x</a> [e](https://e.com) `[f](f.md)` [g](/top.md) [h](../../out.md)\n```\n[k](k.md)\n```\n"
	got := LinkTargets("docs/x.md", []byte(doc))
	slices.Sort(got)
	want := []string{"docs/b.md", "docs/c.md", "docs/d.md", "img/p.png", "top.md"}
	if !slices.Equal(got, want) {
		t.Errorf("LinkTargets() = %v, want %v: relative links, images, definitions and HTML, none in code, none external or leaving the repository", got, want)
	}
}

func TestCountLines_EveryLineBreakCounts(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"no ending", "a", 1},
		{"lf", "a\nb\n", 2},
		{"crlf is one break", "a\r\nb\r\n", 2},
		{"cr alone is a break", strings.Repeat("x\r", 50), 50},
		{"cr and no ending", "a\rb", 2},
		{"line separator", "a b c", 3},
		{"a bare E2 80 prefix is not a break", "a†b", 1},
		{"blank lines", "\n\n\n", 3},
	}
	for _, tc := range tests {
		if got := countLines([]byte(tc.in)); got != tc.want {
			t.Errorf("%s: countLines(%q) = %d, want %d", tc.name, tc.in, got, tc.want)
		}
		// fed one byte at a time: a break split across two reads is still one
		if got, err := countStream(iotest.OneByteReader(strings.NewReader(tc.in)), int64(len(tc.in))); err != nil || got != tc.want {
			t.Errorf("%s: countStream(one byte at a time) = %d, %v, want %d", tc.name, got, err, tc.want)
		}
	}
}

func TestCheckScope_ABoundOnBytesAndOnEveryKindOfLine(t *testing.T) {
	s := scope()
	s.MaxAddedLines, s.MaxAddedBytes = ptr(10), ptr(4096)
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"one line of eight mebibytes passes a bound on lines and fails the bytes", strings.Repeat("x", 8<<20), []string{catalog.RuleScopeAddedBytes}},
		{"fifty lines ended by CR", strings.Repeat("x\r", 50), []string{catalog.RuleScopeAddedLines}},
		{"fifty lines ended by CRLF", strings.Repeat("x\r\n", 50), []string{catalog.RuleScopeAddedLines}},
		{"fifty lines ended by U+2028", strings.Repeat("x ", 50), []string{catalog.RuleScopeAddedLines}},
		{"within both bounds", strings.Repeat("x\n", 10), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := parse(t, add("docs/runbooks/a.bin", tc.body))
			got := rules(checkScope(c, s))
			if !slices.Equal(got, tc.want) {
				t.Errorf("checkScope() rules = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckMarkdown_CRLineEndingsAreLineEndings(t *testing.T) {
	fs := checkMarkdown("a.md", []byte("```\rcode\r"), treeOf())
	if len(fs) != 1 || fs[0].rule != catalog.RuleMarkdownFence || fs[0].line != 1 {
		t.Errorf("checkMarkdown(CR only, unclosed) = %+v, want the fence at line 1", fs)
	}
	fs = checkMarkdown("a.md", []byte("```\rx\r```\r[a](gone.md)\r"), treeOf())
	if len(fs) != 1 || fs[0].rule != catalog.RuleMarkdownLink || fs[0].line != 4 {
		t.Errorf("checkMarkdown(CR only) = %+v, want the link at line 4", fs)
	}
}

// Every construct that once rescanned a line is linear: a megabyte of one line holding all of them finishes at once.
func TestCheckMarkdown_ALongLineIsLinear(t *testing.T) {
	var b strings.Builder
	for b.Len() < 1<<20 {
		b.WriteString("[a](x.md) `c` ![i](<y z>) [r](( ](] <a href=\"h.md\"> ")
	}
	line := b.String()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = checkMarkdown("a.md", []byte(line), treeOf())
		_ = checkJustRecipes("a.md", []byte("`"+line+"`"), parseRecipes(""))
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("checking a one-megabyte line did not finish in 20s: a scan is not linear")
	}
}

func TestServer_Run_AFailedEmitClaimsNoEvidence(t *testing.T) {
	h := newHost()
	h.emitErr = errTestEmit
	cand := h.put("candidate/x", tarOf(t, testBase, add("docs/runbooks/a.md", cleanDoc)))
	resp, ids, err := newServer(h).Run(t.Context(), "inv", &verifierv1.RunRequest{CandidateRef: cand, ConfigRef: configRef(t, h, nil)})
	if err != nil || resp == nil || len(ids) != 0 {
		t.Errorf("Run(Emit fails) = %v, %v, %v, want the findings, no evidence id and no error (the host records the call PARTIAL)", resp, ids, err)
	}
	if len(h.emitted) != 0 {
		t.Errorf("emitted %d records, want none", len(h.emitted))
	}
}
