package provider

import (
	"errors"
	"testing"
)

func TestParseRuff_ASyntaxErrorHasNoCodeAndIsHigh(t *testing.T) {
	out := []byte(`[{"code": null, "message": "SyntaxError: invalid syntax", "filename": "/w/a.py", "location": {"row": 1}, "end_location": {"row": 1}}]`)
	got, err := parseRuff("/w", out)
	if err != nil || len(got) != 1 || got[0].rule != "ruff/syntax-error" || got[0].severity != sevHigh || got[0].path != "a.py" {
		t.Errorf("parseRuff(syntax error) = %+v, %v, want one high ruff/syntax-error in a.py", got, err)
	}
}

func TestParseSemgrep_ASkippedFileOrNoScannedFileIsNotAPass(t *testing.T) {
	tests := map[string]string{
		"a skipped file":    `{"results": [], "errors": [], "paths": {"scanned": ["a.py"], "skipped": [{"path": "b.py"}]}}`,
		"nothing scanned":   `{"results": [], "errors": [], "paths": {"scanned": []}}`,
		"no paths at all":   `{"results": [], "errors": []}`,
		"an error reported": `{"results": [], "errors": [{"type": "x"}], "paths": {"scanned": ["a.py"]}}`,
	}
	for name, out := range tests {
		if _, err := parseSemgrep("/w", []byte(out)); !errors.Is(err, errParse) {
			t.Errorf("%s: parseSemgrep error = %v, want errParse", name, err)
		}
	}
	if got, err := parseSemgrep("/w", []byte(`{"results": [], "errors": [], "paths": {"scanned": ["a.py"]}}`)); err != nil || len(got) != 0 {
		t.Errorf("parseSemgrep(clean) = %v, %v, want none", got, err)
	}
}
