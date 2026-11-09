package provider

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		name, in string
		layout   bool
		want     string
	}{
		{"strips controls, bidi and zero-width", "a\x00b\u202ec\u200bd\ufeffe\u2066f\u2069g\u061ch", false, "abcdefgh"},
		{"a title loses line feeds and tabs", "a\nb\tc\rd", false, "a b c d"},
		{"a body keeps line feed and tab", "a\nb\tc", true, "a\nb\tc"},
		{"a body turns CRLF and CR into line feeds", "a\r\nb\rc", true, "a\nb\nc"},
		{"other white space becomes a space", "a\u00a0b\u2003c\u2028d", false, "a b c d"},
		{"format and private use characters go", "a\u00adb\u2061c\ue000d\U000e0001e", false, "abcde"},
		{"invalid UTF-8 is replaced", "a\xffb", false, "a\ufffdb"},
		{"the ends are trimmed", "  a b \n", true, "a b"},
		{"letters of every script stay", "\u00e9\u4e2d\u0645", false, "\u00e9\u4e2d\u0645"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitize(tc.in, tc.layout); got != tc.want {
				t.Errorf("sanitize(%q, %v) = %q, want %q", tc.in, tc.layout, got, tc.want)
			}
		})
	}
}

func TestCutAt(t *testing.T) {
	const s = "a\u00e9b" // 1 + 2 + 1 bytes
	tests := []struct {
		limit int
		want  string
		cut   bool
	}{{0, "", true}, {1, "a", true}, {2, "a", true}, {3, "a\u00e9", true}, {4, s, false}, {5, s, false}}
	for _, tc := range tests {
		got, cut := cutAt(s, tc.limit)
		if got != tc.want || cut != tc.cut || !utf8.ValidString(got) {
			t.Errorf("cutAt(%q, %d) = %q, %v, want %q, %v", s, tc.limit, got, cut, tc.want, tc.cut)
		}
	}
}

func TestJQL_TextIsEscapedNeverSpliced(t *testing.T) {
	const unset = ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED
	tests := []struct {
		name, project, text string
		state               ticketingv1.IssueStateCategory
		want                string
	}{
		{"plain text", "PAY", "invoice", unset, `project = "PAY" AND text ~ "invoice" ORDER BY key ASC`},
		{"no text", "PAY", "", unset, `project = "PAY" ORDER BY key ASC`},
		{"a quote cannot end the literal or open a phrase", "PAY", `a" OR project = "OPS`, unset, `project = "PAY" AND text ~ "a\\\" or project = \\\"OPS" ORDER BY key ASC`},
		{"a bare quote is escaped for the text search too", "PAY", `"`, unset, `project = "PAY" AND text ~ "\\\"" ORDER BY key ASC`},
		{"operator words are neutralised, other words are not", "PAY", `fire AND ice OR NOT x ANDROID and NOTE`, unset,
			`project = "PAY" AND text ~ "fire and ice or not x ANDROID and NOTE" ORDER BY key ASC`},
		{"an operator word next to punctuation is neutralised", "PAY", `(a)AND(b) OR_x _NOT`, unset,
			`project = "PAY" AND text ~ "\\(a\\)and\\(b\\) OR_x _NOT" ORDER BY key ASC`},
		{"a backslash is doubled twice", "PAY", `a\b`, unset, `project = "PAY" AND text ~ "a\\\\b" ORDER BY key ASC`},
		{"wildcards and operators are escaped", "PAY", `*?~+-&|!(){}[]^:/`, unset,
			`project = "PAY" AND text ~ "\\*\\?\\~\\+\\-\\&\\|\\!\\(\\)\\{\\}\\[\\]\\^\\:\\/" ORDER BY key ASC`},
		{"open state", "PAY", "x", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN, `project = "PAY" AND text ~ "x" AND statusCategory = "To Do" ORDER BY key ASC`},
		{"in progress", "PAY", "", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_IN_PROGRESS, `project = "PAY" AND statusCategory = "In Progress" ORDER BY key ASC`},
		{"done and canceled share a category", "PAY", "", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED, `project = "PAY" AND statusCategory = "Done" ORDER BY key ASC`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := JQL(tc.project, tc.text, tc.state); got != tc.want {
				t.Errorf("JQL(%q, %q, %v) = %s, want %s", tc.project, tc.text, tc.state, got, tc.want)
			}
		})
	}
}

func TestValidProject(t *testing.T) {
	for _, p := range []string{"PAY", "a", "acme/shop", "a.b-c_d", "A1/b2", strings.Repeat("a", 100), strings.Repeat("a", 100) + "/" + strings.Repeat("b", 100)} {
		if !validProject(p) {
			t.Errorf("validProject(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"", "..", ".", "a/..", "../a", "a/./b", "a//b", "a/", "/a", "a/b/c", "a..b", "-a", "a b", "a\nb", "a\x00b", "a\u202eb", "\uff41", strings.Repeat("a", 101), "a/b\n"} {
		if validProject(p) {
			t.Errorf("validProject(%q) = true, want false", p)
		}
	}
}

func TestRenderDescription(t *testing.T) {
	tests := []struct {
		name, in, want string
		fails          bool
	}{
		{"null", `null`, "", false},
		{"absent", ``, "", false},
		{"a plain string", `"hello"`, "hello", false},
		{"paragraphs", `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]},{"type":"paragraph","content":[{"type":"text","text":"two"},{"type":"hardBreak"},{"type":"text","text":"three"}]}]}`, "one\ntwo\nthree", false},
		{"lists", `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]},{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"c"}]}]}]}]}`, "- a\n- b\n1. c", false},
		{"marks are ignored", `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"https://other.example.test/"}}]}]}]}`, "x", false},
		{"a table", `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]}]}`, "a | b", false},
		{"media, cards and mentions", `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"text":"@Dana"}},{"type":"inlineCard","attrs":{"url":"https://other.example.test/"}},{"type":"emoji","attrs":{"shortName":":smile:"}}]},{"type":"mediaSingle","content":[{"type":"media","attrs":{"url":"https://other.example.test/p"}}]}]}`, "@Dana[card]:smile:\n[attachment]", false},
		{"not a document", `[1]`, "", true},
		{"a number", `7`, "", true},
		{"deeper than the bound", strings.Repeat(`{"type":"blockquote","content":[`, maxADFDepth+2) + strings.Repeat(`]}`, maxADFDepth+2), "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderDescription(json.RawMessage(tc.in))
			if (err != nil) != tc.fails || got != tc.want {
				t.Errorf("renderDescription(%s) = %q, %v, want %q, fails %v", tc.in, got, err, tc.want, tc.fails)
			}
		})
	}
}

func TestCommentDocument_IsPlainTextNodesOnly(t *testing.T) {
	body := "[~accountid:abc] @alice {code}x{code}\n\n*bold* <b>y</b>\tz"
	b, err := json.Marshal(commentDocument(body))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version int `json:"version"`
		Content []struct {
			Type    string `json:"type"`
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				Marks json.RawMessage `json:"marks"`
			} `json:"content"`
		} `json:"content"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, len(doc.Content))
	for _, p := range doc.Content {
		if p.Type != "paragraph" {
			t.Errorf("block type %q, want paragraph only", p.Type)
		}
		line := ""
		for _, n := range p.Content {
			if n.Type != "text" || n.Marks != nil || n.Text == "" {
				t.Errorf("inline node %+v, want a plain text node with text and no marks", n)
			}
			line += n.Text
		}
		lines = append(lines, line)
	}
	if got := strings.Join(lines, "\n"); doc.Version != 1 || got != body {
		t.Errorf("document text = %q (version %d), want the body %q unchanged", got, doc.Version, body)
	}
}

func TestCategory_CanceledNeedsAName(t *testing.T) {
	tests := []struct {
		key, status, resolution string
		want                    ticketingv1.IssueStateCategory
	}{
		{"new", "To Do", "", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN},
		{"indeterminate", "In Review", "", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_IN_PROGRESS},
		{"done", "Done", "Done", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE},
		{"done", "Closed", "Won't Do", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED},
		{"done", "Cancelled", "", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED},
		{"done", "Done", "Duplicate", ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED},
	}
	for _, tc := range tests {
		if got, ok := category(tc.key, tc.status, tc.resolution); !ok || got != tc.want {
			t.Errorf("category(%q, %q, %q) = %v, %v, want %v", tc.key, tc.status, tc.resolution, got, ok, tc.want)
		}
	}
	if _, ok := category("undefined", "x", ""); ok {
		t.Error("category(undefined) ok = true, want false: an unknown category is never guessed")
	}
}

func TestPick_ChoosesTheLowestAvailableTransitionWithoutAScreen(t *testing.T) {
	yes, no := true, false
	to := func(name, key string) *struct {
		Name           string         `json:"name"`
		StatusCategory *categoryField `json:"statusCategory"`
	} {
		return &struct {
			Name           string         `json:"name"`
			StatusCategory *categoryField `json:"statusCategory"`
		}{name, &categoryField{key}}
	}
	ts := []transitionDecl{
		{ID: "50", To: to("Done", "done")},
		{ID: "9", To: to("Done", "done"), HasScreen: true},
		{ID: "12", To: to("Done", "done"), IsAvailable: &no},
		{ID: "../x", To: to("Done", "done")},
		{ID: "30", To: to("Done", "done"), IsAvailable: &yes},
		{ID: "7", To: to("Won't Do", "done")},
		{ID: "8", To: to("To Do", "new")},
	}
	if id, ok := pick(ts, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE); !ok || id != "30" {
		t.Errorf("pick(DONE) = %q, %v, want 30", id, ok)
	}
	if id, ok := pick(ts, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED); !ok || id != "7" {
		t.Errorf("pick(CANCELED) = %q, %v, want 7", id, ok)
	}
	if _, ok := pick(ts, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_IN_PROGRESS); ok {
		t.Error("pick(IN_PROGRESS) ok = true, want false")
	}
}

func TestFixedMessages_StayInTheClosedVocabulary(t *testing.T) {
	for _, m := range []string{
		msgProject, msgIssueKey, msgBlockedProject, msgBlockedKey, msgBodyBound, msgResultsBound, msgText, msgState, msgTarget, msgLinkType, msgSelfLink,
		msgComment, msgKey, msgDryRun, msgNotFound, msgNoTrans, msgRefused, msgRateLimited, msgUnavailable, msgUpstream, msgOverLimit, msgTimeout, msgReadOnly,
	} {
		if len(m) > 200 || strings.ContainsAny(m, "'\"\\%+=@") || strings.IndexFunc(m, func(r rune) bool { return r > 0x7e }) >= 0 || strings.ContainsAny(m, "0123456789") {
			t.Errorf("message %q is outside the closed vocabulary of the kit or holds a digit", m)
		}
	}
}
