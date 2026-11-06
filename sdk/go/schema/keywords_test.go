package schema_test

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/schema"
)

// TestValidate_Keywords gives each keyword a passing and a failing case.
func TestValidate_Keywords(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		value  string
		want   []schema.Violation
	}{
		{name: "integer accepts an integer", schema: `{"type": "integer"}`, value: `7`},
		{name: "integer accepts an integral fraction and exponent", schema: `{"type": "integer"}`, value: `1.50e1`},
		{name: "integer rejects a fraction", schema: `{"type": "integer"}`, value: `1.5`, want: []schema.Violation{{Path: "", Keyword: "type"}}},
		{name: "integer rejects a string", schema: `{"type": "integer"}`, value: `"1"`, want: []schema.Violation{{Path: "", Keyword: "type"}}},
		{name: "number accepts a fraction", schema: `{"type": "number"}`, value: `-1.5e-3`},
		{name: "number rejects null", schema: `{"type": "number"}`, value: `null`, want: []schema.Violation{{Path: "", Keyword: "type"}}},
		{name: "minimum is inclusive", schema: `{"minimum": 0}`, value: `0`},
		{name: "minimum fails below", schema: `{"minimum": 0}`, value: `-1e-9`, want: []schema.Violation{{Path: "", Keyword: "minimum"}}},
		{name: "maximum is inclusive", schema: `{"maximum": 4294967295}`, value: `4.294967295e9`},
		{name: "maximum fails above", schema: `{"maximum": 4294967295}`, value: `4294967295.1`, want: []schema.Violation{{Path: "", Keyword: "maximum"}}},
		{name: "bounds ignore other types", schema: `{"minimum": 1, "maximum": 0}`, value: `"x"`},
		{name: "pattern matches", schema: `{"pattern": "^a+$"}`, value: `"aaa"`},
		{name: "pattern is not anchored", schema: `{"pattern": "b"}`, value: `"abc"`},
		{name: "pattern fails", schema: `{"pattern": "^a+$"}`, value: `"ab"`, want: []schema.Violation{{Path: "", Keyword: "pattern"}}},
		{name: "pattern ignores other types", schema: `{"pattern": "^a$"}`, value: `1`},
		{name: "items checks each element", schema: `{"items": {"type": "string"}}`, value: `["a", "b"]`},
		{name: "items fails an element", schema: `{"items": {"type": "string"}}`, value: `["a", 1, "c", null]`, want: []schema.Violation{{Path: "/1", Keyword: "type"}, {Path: "/3", Keyword: "type"}}},
		{name: "items ignores objects", schema: `{"items": false}`, value: `{"0": 1}`},
		{name: "anyOf passes on one branch", schema: `{"anyOf": [{"type": "string"}, {"type": "null"}]}`, value: `null`},
		{name: "anyOf fails without branch detail", schema: `{"anyOf": [{"type": "string"}, {"minimum": 2}]}`, value: `1`, want: []schema.Violation{{Path: "", Keyword: "anyOf"}}},
		{name: "propertyNames passes", schema: `{"propertyNames": {"pattern": "^[a-z]+$"}}`, value: `{"ab": 1}`},
		{name: "propertyNames fails a key", schema: `{"propertyNames": {"pattern": "^[a-z]+$"}}`, value: `{"ab": 1, "Ab": 2}`, want: []schema.Violation{{Path: "/Ab", Keyword: "propertyNames"}}},
		{name: "propertyNames ignores arrays", schema: `{"propertyNames": false}`, value: `[1]`},
		{
			name:   "nested paths",
			schema: `{"properties": {"tags": {"items": {"anyOf": [{"type": "string"}]}}, "ext": {"propertyNames": {"pattern": "^x"}}}}`,
			value:  `{"tags": ["a", 1], "ext": {"xa": 1, "b": 2}}`,
			want:   []schema.Violation{{Path: "/ext/b", Keyword: "propertyNames"}, {Path: "/tags/1", Keyword: "anyOf"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := violations(t, mustCompile(t, tt.schema).Validate(decode(t, tt.value)))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Validate(%s) against %s violations = %v, want %v", tt.value, tt.schema, got, tt.want)
			}
		})
	}
}

// TestValidate_NumberOrder compares values with a bound exactly: equal passes
// both minimum and maximum, smaller fails minimum, larger fails maximum.
func TestValidate_NumberOrder(t *testing.T) {
	tests := []struct {
		bound, value string
		want         int
	}{
		{bound: "0", value: "-0", want: 0},
		{bound: "0", value: "0.000e5", want: 0},
		{bound: "1", value: "10e-1", want: 0},
		{bound: "1", value: "0.1E+1", want: 0},
		{bound: "100", value: "1e2", want: 0},
		{bound: "-1.5", value: "-1.50", want: 0},
		{bound: "100", value: "99.99999999999999999999", want: -1},
		{bound: "123456789012345678901234567890", value: "123456789012345678901234567891", want: 1},
		{bound: "-1", value: "-2", want: -1},
		{bound: "-1", value: "1", want: 1},
		{bound: "0", value: "-1e-400", want: -1},
		{bound: "0", value: "1e-400", want: 1},
		{bound: "1e399", value: "1e400", want: 1},
		{bound: "0.5", value: "0.05", want: -1},
		{bound: "1e400", value: "1e99999999999999999999999", want: 1},
		{bound: "-1e400", value: "-1e99999999999999999999999", want: -1},
		{bound: "1e-400", value: "1e-99999999999999999999999", want: -1},
	}
	for _, tt := range tests {
		s := mustCompile(t, `{"minimum": `+tt.bound+`, "maximum": `+tt.bound+`}`)
		var want []schema.Violation
		switch tt.want {
		case -1:
			want = []schema.Violation{{Keyword: "minimum"}}
		case 1:
			want = []schema.Violation{{Keyword: "maximum"}}
		}
		if got := violations(t, s.Validate(json.Number(tt.value))); !slices.Equal(got, want) {
			t.Errorf("Validate(%s) with bound %s violations = %v, want %v", tt.value, tt.bound, got, want)
		}
	}
}

// TestValidate_Integer covers integral values in json.Number and float64, and
// float64 values against bounds in their shortest round-trip form.
func TestValidate_Integer(t *testing.T) {
	s := mustCompile(t, `{"type": "integer"}`)
	for _, tt := range []struct {
		value any
		want  bool
	}{
		{value: json.Number("0e-9"), want: true},
		{value: json.Number("-3"), want: true},
		{value: json.Number("1e400"), want: true},
		{value: json.Number("1.25e1"), want: false},
		{value: json.Number("1e-400"), want: false},
		{value: json.Number("123456789012345678901234567890.0000"), want: true},
		{value: 2.0, want: true},
		{value: 2.5, want: false},
	} {
		if got := s.Validate(tt.value) == nil; got != tt.want {
			t.Errorf("Validate(%v) against integer ok = %t, want %t", tt.value, got, tt.want)
		}
	}
	bounds := mustCompile(t, `{"minimum": 0.1, "maximum": 2}`)
	for _, tt := range []struct {
		value float64
		want  bool
	}{
		{value: 0.1, want: true},
		{value: 2, want: true},
		{value: 0.09999999999999999, want: false},
		{value: 2.0000000000000004, want: false},
		{value: 0, want: false},
	} {
		if got := bounds.Validate(tt.value) == nil; got != tt.want {
			t.Errorf("Validate(%v) against [0.1, 2] ok = %t, want %t", tt.value, got, tt.want)
		}
	}
}

// TestValidate_DataModel checks that a value outside the JSON data model fails
// wherever it sits, in an array without items or an object the schema leaves
// open, and that it stops the schema checks.
func TestValidate_DataModel(t *testing.T) {
	for _, n := range []json.Number{"", "01", "1.", ".5", "+1", "1e", "1e+-5", "0x10", "NaN", "1_000", "--1", "1 "} {
		if got := violations(t, mustCompile(t, `true`).Validate(n)); !slices.Equal(got, []schema.Violation{{Keyword: "type"}}) {
			t.Errorf("Validate(json.Number(%q)) violations = %v, want a type violation", n, got)
		}
	}
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if mustCompile(t, `{"type": "number"}`).Validate(f) == nil {
			t.Errorf("Validate(%v) = nil, want a type violation", f)
		}
	}
	tests := []struct {
		name   string
		schema string
		value  any
		want   []schema.Violation
	}{
		{
			name:   "array element without items",
			schema: `{"type": "array"}`,
			value:  []any{"a", json.Number("01"), []any{struct{}{}}},
			want:   []schema.Violation{{Path: "/1", Keyword: "type"}, {Path: "/2/0", Keyword: "type"}},
		},
		{
			name:   "member of an open object, schema checks skipped",
			schema: `{"required": ["x"]}`,
			value:  map[string]any{"b": []any{map[string]any{"c": 1}}, "a": nil},
			want:   []schema.Violation{{Path: "/b/0/c", Keyword: "type"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := violations(t, mustCompile(t, tt.schema).Validate(tt.value)); !slices.Equal(got, tt.want) {
				t.Errorf("Validate(%v) violations = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestValidate_GeneratedBudgetPolicy validates the JSON form of
// schemas/examples/BudgetPolicy/org-default.yaml and variants of it.
func TestValidate_GeneratedBudgetPolicy(t *testing.T) {
	b, err := os.ReadFile("../../../schemas/gen/numinance.kinds.v1.BudgetPolicy.jsonschema.strict.bundle.json")
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}
	s := mustCompile(t, string(b))
	const head = `{"apiVersion": "numinance.dev/v1", "kind": "BudgetPolicy", "metadata": {"name": "org-default", "scope": "SCOPE_ORG"}, `
	tests := []struct {
		name string
		spec string
		want []schema.Violation
	}{
		{
			name: "example",
			spec: `"spec": {"tokens": 400000, "wall_clock": "7200s", "tool_calls": 300, "files_changed": 40, "lines_changed": 2000, "repositories": 1, "services": 2, "network_destinations": 5, "retries": 3, "max_risk_escalation": "RISK_CLASS_MEDIUM"}}`,
		},
		{
			name: "out of range and fractional limits",
			spec: `"spec": {"tokens": 4294967296, "retries": -1, "services": 1.5, "repositories": "1"}}`,
			want: []schema.Violation{
				{Path: "/spec/repositories", Keyword: "type"},
				{Path: "/spec/retries", Keyword: "minimum"},
				{Path: "/spec/services", Keyword: "type"},
				{Path: "/spec/tokens", Keyword: "maximum"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := violations(t, s.Validate(decode(t, head+tt.spec))); !slices.Equal(got, tt.want) {
				t.Errorf("Validate(%s) violations = %v, want %v", tt.spec, got, tt.want)
			}
		})
	}
}
