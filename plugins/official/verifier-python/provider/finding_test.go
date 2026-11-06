package provider

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	verifierkit "github.com/numinance/numinance-spec/conformance/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

func TestNormalizeMessage(t *testing.T) {
	long := strings.Repeat("é", 700) // 1400 bytes, two bytes a character
	tests := []struct {
		name string
		in   string
		want func(string) bool
	}{
		{"keeps a newline and a tab", "a\n\tb", func(s string) bool { return s == "a\n\tb" }},
		{"replaces a zero width space", "a\u200bb", func(s string) bool { return s == "a�b" }},
		{"replaces a control character", "a\x00b\x1b", func(s string) bool { return s == "a�b�" }},
		{"shortens at a character boundary", long, func(s string) bool { return len(s) <= 1024 && utf8.ValidString(s) && len(s) >= 1022 }},
		{"an empty message gets a fixed text", "  ", func(s string) bool { return s != "" }},
	}
	for _, tt := range tests {
		if got := normalizeMessage(tt.in); !tt.want(got) {
			t.Errorf("%s: normalizeMessage(%q) = %q", tt.name, tt.in, got)
		}
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"/work/c/shop/a.py", "shop/a.py", true},
		{"/candidate/shop/a.py", "shop/a.py", true},
		{"./shop/a.py", "shop/a.py", true},
		{"shop/../a.py", "", false},
		{"shop//a.py", "", false},
		{"shop\\a.py", "", false},
		{"", "", false},
		{"shop/a\u200b.py", "", false},
	}
	for _, tt := range tests {
		got, ok := cleanPath("/work/c", tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("cleanPath(%q) = %q, %v, want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestBuildResponse_TheLimitOrdersAndCounts(t *testing.T) {
	in := make([]finding, 0, 1101)
	for i := range 1100 {
		sev := sevLow
		if i < 3 {
			sev = sevHigh
		}
		in = append(in, finding{rule: "ruff/E501", severity: sev, path: "a.py", start: 1, end: 1, message: fmt.Sprintf("message %d", i)})
	}
	in = append(in, in[0]) // a repeat: merged by fingerprint
	resp := buildResponse("ruff", in)
	if len(resp.GetItems()) != 1024 {
		t.Fatalf("len(items) = %d, want 1024", len(resp.GetItems()))
	}
	if err := verifierkit.ValidateResponse(resp); err != nil {
		t.Fatalf("ValidateResponse = %v, want nil", err)
	}
	var total uint64
	for _, c := range resp.GetSummary().GetCounts() {
		total += c.GetCount()
	}
	if total != 1100 {
		t.Errorf("total = %d, want 1100 (the repeat is merged)", total)
	}
	if got := resp.GetItems()[0].GetSeverity(); got != typesv1.FindingSeverity_FINDING_SEVERITY_HIGH {
		t.Errorf("items[0].severity = %v, want HIGH first", got)
	}
}

func TestBuildResponse_ALocationWithoutALineHasNone(t *testing.T) {
	resp := buildResponse("mypy", []finding{{rule: "mypy/x", severity: sevMedium, path: "a.py", end: 9, message: "m"}})
	if loc := resp.GetItems()[0].GetLocation(); loc.GetStartLine() != 0 || loc.GetEndLine() != 0 {
		t.Errorf("location = %v, want lines 0 and 0 together", loc)
	}
}
