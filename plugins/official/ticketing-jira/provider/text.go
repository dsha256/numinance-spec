package provider

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// sanitize makes a string the tracker returned fit the port's text rules: it keeps the printable characters (Unicode categories L,
// M, N, P, S and U+0020), turns other white space into a space, and drops every control, bidirectional, zero-width and other
// format character, private-use and unassigned code point. With layout set (a body) a line feed and a tab survive and a carriage
// return becomes a line feed; without it (a title, a type, a label, an operation id) they become spaces. It never fails: the text
// is untrusted and the Engine strips again.
func sanitize(s string, layout bool) string {
	s = strings.ToValidUTF8(s, "\ufffd")
	if layout {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			if layout {
				b.WriteRune(r)
			} else {
				b.WriteByte(' ')
			}
		case r == '\r':
			b.WriteByte(' ')
		case printableRune(r):
			b.WriteRune(r)
		case unicode.Is(unicode.Zs, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			b.WriteByte(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

// cutAt returns the longest prefix of s of at most limit bytes that ends at a character boundary, and whether s was longer.
func cutAt(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// The bounds of rendering an untrusted Atlassian Document Format tree.
const (
	maxADFDepth = 32
	maxADFNodes = 100_000
	maxADFText  = 1 << 20
)

var errADF = errors.New("the description is not a document this provider renders")

type adfNode struct {
	Type    string                     `json:"type"`
	Text    string                     `json:"text"`
	Content []adfNode                  `json:"content"`
	Attrs   map[string]json.RawMessage `json:"attrs"`
}

// renderDescription turns the description field of an issue into plain text. It accepts a JSON null, a plain string (an older
// answer) or an ADF document. Marks (links, code, colour) are ignored and no URL of the document is read: a link, a media
// reference or a card renders as a fixed placeholder, so nothing in the text can make the provider fetch anything.
func renderDescription(raw json.RawMessage) (string, error) {
	t := strings.TrimSpace(string(raw))
	switch {
	case t == "" || t == "null":
		return "", nil
	case strings.HasPrefix(t, `"`):
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", errADF
		}
		return s, nil
	case strings.HasPrefix(t, "{"):
		var doc adfNode
		if err := json.Unmarshal(raw, &doc); err != nil {
			return "", errADF
		}
		r := &adfRenderer{}
		out := r.block(doc, 0)
		if r.err != nil {
			return "", r.err
		}
		return out, nil
	}
	return "", errADF
}

type adfRenderer struct {
	nodes int
	err   error
}

func (r *adfRenderer) visit(depth int) bool {
	r.nodes++
	if depth > maxADFDepth || r.nodes > maxADFNodes {
		r.err = errADF
	}
	return r.err == nil
}

func (r *adfRenderer) attrText(n adfNode, names ...string) string {
	for _, k := range names {
		var s string
		if json.Unmarshal(n.Attrs[k], &s) == nil && s != "" {
			return s
		}
	}
	return ""
}

// inline renders the content of a node that holds inline nodes (a paragraph, a heading, a code block).
func (r *adfRenderer) inline(n adfNode, depth int) string {
	var b strings.Builder
	for _, c := range n.Content {
		if !r.visit(depth + 1) {
			return ""
		}
		b.WriteString(r.leaf(c, depth+1))
		if b.Len() > maxADFText {
			r.err = errADF
			return ""
		}
	}
	return b.String()
}

// leaf renders one inline node.
func (r *adfRenderer) leaf(n adfNode, depth int) string {
	switch n.Type {
	case "text":
		return n.Text
	case "hardBreak":
		return "\n"
	case "mention":
		if s := r.attrText(n, "text"); s != "" {
			return s
		}
		return "[mention]"
	case "emoji":
		return r.attrText(n, "text", "shortName")
	case "status":
		return r.attrText(n, "text")
	case "date":
		return "[date]"
	case "inlineCard", "blockCard", "embedCard":
		return "[card]"
	case "media", "mediaInline":
		return "[attachment]"
	}
	if n.Text != "" {
		return n.Text
	}
	return r.inline(n, depth)
}

// block renders a node that holds blocks, one per line.
func (r *adfRenderer) block(n adfNode, depth int) string {
	if !r.visit(depth) {
		return ""
	}
	switch n.Type {
	case "paragraph", "heading", "codeBlock", "taskItem", "decisionItem":
		return r.inline(n, depth)
	case "rule":
		return "---"
	case "mediaSingle", "mediaGroup", "media", "mediaInline", "inlineCard", "blockCard", "embedCard":
		return r.leaf(n, depth)
	case "bulletList", "orderedList", "taskList", "decisionList":
		var lines []string
		for i, c := range n.Content {
			item := r.block(c, depth+1)
			if n.Type == "orderedList" {
				item = strconv.Itoa(i+1) + ". " + item
			} else {
				item = "- " + item
			}
			lines = append(lines, item)
		}
		return strings.Join(lines, "\n")
	case "tableRow":
		var cells []string
		for _, c := range n.Content {
			cells = append(cells, r.block(c, depth+1))
		}
		return strings.Join(cells, " | ")
	}
	if n.Text != "" {
		return n.Text
	}
	var parts []string
	for _, c := range n.Content {
		parts = append(parts, r.block(c, depth+1))
		if r.err != nil {
			return ""
		}
	}
	return strings.Join(parts, "\n")
}

// commentDocument builds the Atlassian Document Format body of a comment from plain text: one paragraph per line, each a single
// text node with no marks. Wiki markup, HTML and a mention in the text stay characters of a text node; only a "mention" node
// notifies a person, and this builder never emits one.
func commentDocument(body string) map[string]any {
	paras := make([]any, 0, strings.Count(body, "\n")+1)
	for _, line := range strings.Split(body, "\n") {
		p := map[string]any{"type": "paragraph"}
		if line != "" {
			p["content"] = []any{map[string]any{"type": "text", "text": line}}
		}
		paras = append(paras, p)
	}
	return map[string]any{"type": "doc", "version": 1, "content": paras}
}

// plainParagraphs reads a comment body that is exactly what commentDocument builds: a document of paragraphs whose only inline
// nodes are unmarked text. It returns the paragraphs joined by a line feed, and false for anything else (a mention, a link mark,
// a table, a body that is not a document), so an edited comment is never taken for the one the provider posted.
func plainParagraphs(raw json.RawMessage) (string, bool) {
	var doc struct {
		Type    string `json:"type"`
		Version int    `json:"version"`
		Content []struct {
			Type    string `json:"type"`
			Content []struct {
				Type  string            `json:"type"`
				Text  string            `json:"text"`
				Marks []json.RawMessage `json:"marks"`
			} `json:"content"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Type != "doc" || len(doc.Content) == 0 || len(doc.Content) > maxADFNodes {
		return "", false
	}
	lines := make([]string, 0, len(doc.Content))
	for _, p := range doc.Content {
		if p.Type != "paragraph" || len(p.Content) > 1 {
			return "", false
		}
		line := ""
		for _, n := range p.Content {
			if n.Type != "text" || len(n.Marks) > 0 {
				return "", false
			}
			line = n.Text
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), true
}
