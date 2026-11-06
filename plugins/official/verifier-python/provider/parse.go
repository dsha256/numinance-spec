package provider

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// errParse is the error of output that is not what the tool prints: the call ends FAILED, never SUCCEEDED with a guess.
var errParse = errors.New("the tool output cannot be parsed")

const (
	sevInfo   = typesv1.FindingSeverity_FINDING_SEVERITY_INFO
	sevLow    = typesv1.FindingSeverity_FINDING_SEVERITY_LOW
	sevMedium = typesv1.FindingSeverity_FINDING_SEVERITY_MEDIUM
	sevHigh   = typesv1.FindingSeverity_FINDING_SEVERITY_HIGH
)

// okExit reports whether code is an exit status the check uses to say it ran to the end: with or without findings. Any
// other status is a crash.
func okExit(check string, code int) bool {
	switch check {
	case checkRuff, checkMypy, checkImportLinter:
		return code == 0 || code == 1
	case checkSemgrep:
		return code == 0
	case checkPytest:
		return code == 0 || code == 1 || code == 5
	}
	return false
}

// parse reads the raw output of check into findings. dir is the directory the tool ran in; it is stripped from paths.
func parse(check, dir string, out []byte) ([]finding, error) {
	switch check {
	case checkRuff:
		return parseRuff(dir, out)
	case checkMypy:
		return parseMypy(dir, out)
	case checkSemgrep:
		return parseSemgrep(dir, out)
	case checkImportLinter:
		return parseImportLinter(out)
	case checkPytest:
		return parsePytest(out)
	}
	return nil, errParse
}

// ruleToken makes a tool's rule name fit the port's rule_id grammar: characters outside it become '_', and the id starts
// with an alphanumeric.
func ruleToken(prefix, name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < 0x80 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:/-", r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		s = "unknown"
	}
	return prefix + "/" + s
}

func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %w", errParse, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing data", errParse)
	}
	return nil
}

func uintOf(n int) uint32 {
	if n < 0 || n > 1<<31 {
		return 0
	}
	return uint32(n) //nolint:gosec // bounded above
}

// ruffSeverity: the bandit family is a security rule, pyflakes and bugbear find defects, the rest is style.
func ruffSeverity(code string) typesv1.FindingSeverity {
	switch {
	case code == "syntax-error", strings.HasPrefix(code, "S"):
		// a file ruff cannot parse was not checked at all
		return sevHigh
	case strings.HasPrefix(code, "F"), strings.HasPrefix(code, "B"), strings.HasPrefix(code, "E9"):
		return sevMedium
	}
	return sevLow
}

func parseRuff(dir string, out []byte) ([]finding, error) {
	var rows []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Filename string `json:"filename"`
		Location struct {
			Row int `json:"row"`
		} `json:"location"`
		End struct {
			Row int `json:"row"`
		} `json:"end_location"`
	}
	if err := decodeStrict(out, &rows); err != nil {
		return nil, err
	}
	var fs []finding
	for _, r := range rows {
		if r.Code == "" {
			r.Code = "syntax-error"
		}
		f := finding{rule: ruleToken("ruff", r.Code), severity: ruffSeverity(r.Code), start: uintOf(r.Location.Row), end: uintOf(r.End.Row), message: r.Message}
		f.path, _ = cleanPath(dir, r.Filename)
		fs = append(fs, f)
	}
	return fs, nil
}

// parseMypy reads mypy's JSON output: one object per line.
func parseMypy(dir string, out []byte) ([]finding, error) {
	var fs []finding
	for line := range bytes.SplitSeq(out, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r struct {
			File     string  `json:"file"`
			Line     int     `json:"line"`
			EndLine  int     `json:"end_line"`
			Message  string  `json:"message"`
			Code     *string `json:"code"`
			Severity string  `json:"severity"`
		}
		if err := decodeStrict(line, &r); err != nil {
			return nil, err
		}
		code, sev := "error", sevMedium
		if r.Code != nil && *r.Code != "" {
			code = *r.Code
		}
		if r.Severity == "note" {
			sev = sevInfo
		}
		f := finding{rule: ruleToken("mypy", code), severity: sev, start: uintOf(r.Line), end: uintOf(r.EndLine), message: r.Message}
		f.path, _ = cleanPath(dir, r.File)
		fs = append(fs, f)
	}
	return fs, nil
}

func parseSemgrep(dir string, out []byte) ([]finding, error) {
	var doc struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			End struct {
				Line int `json:"line"`
			} `json:"end"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
			} `json:"extra"`
		} `json:"results"`
		Errors []json.RawMessage `json:"errors"`
		Paths  struct {
			Scanned []string          `json:"scanned"`
			Skipped []json.RawMessage `json:"skipped"`
		} `json:"paths"`
	}
	if err := decodeStrict(out, &doc); err != nil {
		return nil, err
	}
	if len(doc.Errors) > 0 {
		return nil, fmt.Errorf("%w: the tool reported %d errors", errParse, len(doc.Errors))
	}
	// A file the tool skipped, for a size or an ignore rule, was not checked, and a run that scanned nothing checked nothing: neither
	// is a pass.
	if len(doc.Paths.Skipped) > 0 || len(doc.Paths.Scanned) == 0 {
		return nil, fmt.Errorf("%w: the tool skipped files or scanned none", errParse)
	}
	var fs []finding
	for _, r := range doc.Results {
		sev := sevLow
		switch r.Extra.Severity {
		case "ERROR":
			sev = sevHigh
		case "WARNING":
			sev = sevMedium
		}
		f := finding{rule: ruleToken("semgrep", r.CheckID), severity: sev, start: uintOf(r.Start.Line), end: uintOf(r.End.Line), message: r.Extra.Message}
		f.path, _ = cleanPath(dir, r.Path)
		fs = append(fs, f)
	}
	return fs, nil
}

var (
	ilSummary = regexp.MustCompile(`(?m)^Contracts: (\d+) kept, (\d+) broken\.\s*$`)
	ilImport  = regexp.MustCompile(`^-\s+([A-Za-z0-9_.]+) -> ([A-Za-z0-9_.]+) \(l\.(\d+)\)`)
	ilRule    = regexp.MustCompile(`^-{3,}$`)
)

// parseImportLinter reads lint-imports' text report: the summary line must be there, and a broken contract must be listed.
func parseImportLinter(out []byte) ([]finding, error) {
	text := string(out)
	m := ilSummary.FindStringSubmatch(text)
	if m == nil {
		return nil, fmt.Errorf("%w: no contract summary", errParse)
	}
	broken, _ := strconv.Atoi(m[2])
	_, section, found := strings.Cut(text, "Broken contracts")
	if broken == 0 {
		return nil, nil
	}
	if !found {
		return nil, fmt.Errorf("%w: broken contracts are not listed", errParse)
	}
	lines := strings.Split(section, "\n")
	var fs []finding
	var contract string
	var listed int
	flush := func() {
		if contract != "" && listed == 0 {
			fs = append(fs, finding{rule: ruleToken("import-linter", slug(contract)), severity: sevHigh, message: contract + ": the contract is broken"})
		}
	}
	for i, raw := range lines {
		line := strings.TrimRight(raw, " \r")
		if i+1 < len(lines) && line != "" && !ilRule.MatchString(line) && ilRule.MatchString(strings.TrimSpace(lines[i+1])) {
			flush()
			contract, listed = line, 0
			continue
		}
		if im := ilImport.FindStringSubmatch(line); im != nil && contract != "" {
			n, _ := strconv.Atoi(im[3])
			fs = append(fs, finding{
				rule: ruleToken("import-linter", slug(contract)), severity: sevHigh, path: strings.ReplaceAll(im[1], ".", "/") + ".py",
				start: uintOf(n), end: uintOf(n), message: contract + ": " + im[1] + " imports " + im[2],
			})
			listed++
		}
	}
	flush()
	if len(fs) == 0 {
		return nil, fmt.Errorf("%w: broken contracts are listed without a finding", errParse)
	}
	return fs, nil
}

func slug(s string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, s), "-")
}

// parsePytest reads a JUnit XML report. The decoder of encoding/xml expands no external entity and includes nothing, and a
// DOCTYPE or processing instruction other than the XML declaration is refused outright: the report is untrusted output of
// candidate code.
func parsePytest(out []byte) ([]finding, error) {
	dec := xml.NewDecoder(bytes.NewReader(out))
	dec.Strict = true
	var fs []finding
	var cur *finding
	var tests int
	sawSuite := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errParse, err)
		}
		switch t := tok.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("%w: the report declares a document type", errParse)
		case xml.ProcInst:
			if t.Target != "xml" {
				return nil, fmt.Errorf("%w: a processing instruction", errParse)
			}
		case xml.StartElement:
			switch t.Name.Local {
			case "testsuite":
				sawSuite = true
				tests += atoiAttr(t, "tests")
			case "testcase":
				tc := finding{severity: sevHigh}
				classname, name := attr(t, "classname"), attr(t, "name")
				if classname != "" {
					tc.path = strings.ReplaceAll(classname, ".", "/") + ".py"
				}
				tc.message = classname + "::" + name
				cur = &tc
			case "failure", "error":
				if cur != nil {
					cur.rule = "pytest/test-failed"
					if t.Name.Local == "error" {
						cur.rule = "pytest/test-error"
					}
					cur.message += ": " + attr(t, "message")
					fs = append(fs, *cur)
				}
			case "skipped":
				fs = append(fs, finding{rule: "numinance.directive/pytest-skip", severity: sevMedium, message: "a test was skipped: " + attr(t, "message")})
			}
		case xml.EndElement:
			if t.Name.Local == "testcase" {
				cur = nil
			}
		}
	}
	if !sawSuite {
		return nil, fmt.Errorf("%w: no test suite", errParse)
	}
	if tests == 0 {
		fs = append(fs, finding{rule: "pytest/no-tests", severity: sevMedium, message: "no test was collected"})
	}
	return fs, nil
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func atoiAttr(t xml.StartElement, name string) int {
	n, _ := strconv.Atoi(attr(t, name))
	return n
}
