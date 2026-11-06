package schema_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/numinance/numinance-spec/sdk/go/schema"
)

// kindLike is shaped like a generated kind bundle: a root $ref into $defs,
// strict objects and a string enum.
const kindLike = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "kind.bundle.json",
  "$ref": "#/$defs/Kind",
  "$defs": {
    "Kind": {
      "type": "object",
      "additionalProperties": false,
      "required": ["apiVersion", "kind"],
      "properties": {
        "apiVersion": {"type": "string"},
        "kind": {"type": "string"},
        "metadata": {"$ref": "#/$defs/Metadata", "description": "a sibling of $ref"},
        "spec": {
          "type": "object",
          "additionalProperties": false,
          "properties": {"mode": {"type": "string"}, "tags": {"type": "array"}}
        },
        "extensions": {"type": "object", "additionalProperties": {"type": ["object", "string"]}}
      }
    },
    "Metadata": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "scope"],
      "properties": {
        "name": {"type": "string"},
        "scope": {"enum": ["SCOPE_ORG", "SCOPE_REPO"], "type": "string"}
      }
    }
  }
}`

func mustCompile(t *testing.T, doc string) *schema.Schema {
	t.Helper()
	s, err := schema.Compile([]byte(doc))
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	return s
}

// decode decodes a JSON value the way callers should: numbers as json.Number.
func decode(t *testing.T, doc string) any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(doc))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("Decode(%s) = %v", doc, err)
	}
	return v
}

// violations returns the violations of err, or nil when err is nil.
func violations(t *testing.T, err error) []schema.Violation {
	t.Helper()
	if err == nil {
		return nil
	}
	var ve *schema.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Validate() = %v, want a *schema.ValidationError", err)
	}
	return ve.Violations
}

func TestValidate_KindLike(t *testing.T) {
	s := mustCompile(t, kindLike)
	tests := []struct {
		name string
		doc  string
		want []schema.Violation
	}{
		{
			name: "valid document",
			doc:  `{"apiVersion": "numinance.dev/v1", "kind": "K", "metadata": {"name": "a", "scope": "SCOPE_ORG"}, "spec": {"mode": "m", "tags": []}, "extensions": {"com.example.x": {"y": 1}}}`,
		},
		{
			name: "unknown field at the root",
			doc:  `{"apiVersion": "v", "kind": "K", "extra": 1}`,
			want: []schema.Violation{{Path: "/extra", Keyword: "additionalProperties"}},
		},
		{
			name: "unknown field in a nested object",
			doc:  `{"apiVersion": "v", "kind": "K", "spec": {"mod": "m"}}`,
			want: []schema.Violation{{Path: "/spec/mod", Keyword: "additionalProperties"}},
		},
		{
			name: "unknown field behind a $ref",
			doc:  `{"apiVersion": "v", "kind": "K", "metadata": {"name": "a", "scope": "SCOPE_ORG", "labels": {}}}`,
			want: []schema.Violation{{Path: "/metadata/labels", Keyword: "additionalProperties"}},
		},
		{
			name: "missing required fields",
			doc:  `{"metadata": {"scope": "SCOPE_REPO"}}`,
			want: []schema.Violation{
				{Path: "/apiVersion", Keyword: "required"},
				{Path: "/kind", Keyword: "required"},
				{Path: "/metadata/name", Keyword: "required"},
			},
		},
		{
			name: "enum and type",
			doc:  `{"apiVersion": 1, "kind": "K", "metadata": {"name": "a", "scope": "SCOPE_ANY"}, "spec": {"tags": {}}}`,
			want: []schema.Violation{
				{Path: "/apiVersion", Keyword: "type"},
				{Path: "/metadata/scope", Keyword: "enum"},
				{Path: "/spec/tags", Keyword: "type"},
			},
		},
		{
			name: "additionalProperties schema checks the extra values",
			doc:  `{"apiVersion": "v", "kind": "K", "extensions": {"a": "s", "b": 1}}`,
			want: []schema.Violation{{Path: "/extensions/b", Keyword: "type"}},
		},
		{
			name: "keys are JSON Pointer escaped",
			doc:  `{"apiVersion": "v", "kind": "K", "a/b~c": 1}`,
			want: []schema.Violation{{Path: "/a~1b~0c", Keyword: "additionalProperties"}},
		},
		{
			name: "root of the wrong type",
			doc:  `[]`,
			want: []schema.Violation{{Path: "", Keyword: "type"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := violations(t, s.Validate(decode(t, tt.doc)))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Validate(%s) violations = %v, want %v", tt.doc, got, tt.want)
			}
		})
	}
}

// TestValidate_Type covers the non-numeric types; a number matches none of them.
func TestValidate_Type(t *testing.T) {
	tests := []struct {
		typ   string
		value any
		want  bool
	}{
		{typ: "null", value: nil, want: true},
		{typ: "boolean", value: false, want: true},
		{typ: "string", value: "", want: true},
		{typ: "array", value: []any{}, want: true},
		{typ: "object", value: map[string]any{}, want: true},
		{typ: "string", value: json.Number("1"), want: false},
		{typ: "null", value: 1.0, want: false},
		{typ: "object", value: []any{}, want: false},
		{typ: "null", value: false, want: false},
	}
	for _, tt := range tests {
		s := mustCompile(t, `{"type": "`+tt.typ+`"}`)
		if got := s.Validate(tt.value) == nil; got != tt.want {
			t.Errorf("Validate(%#v) against type %s ok = %t, want %t", tt.value, tt.typ, got, tt.want)
		}
	}
}

// TestValidate_OutsideDataModel checks that Go values encoding/json never
// produces fail at every node, and that numbers match no non-numeric type.
func TestValidate_OutsideDataModel(t *testing.T) {
	for _, doc := range []string{`true`, `{"required": ["a"], "additionalProperties": false}`} {
		for _, v := range []any{1, map[string]string{"zzz": "1"}, []string{}, struct{}{}} {
			if err := mustCompile(t, doc).Validate(v); err == nil {
				t.Errorf("Validate(%#v) against %s = nil, want a type violation", v, doc)
			}
		}
	}
	s := mustCompile(t, `{"type": ["null", "boolean", "string", "object", "array"]}`)
	for _, v := range []any{
		json.Number("1"), 1.5, 1, int64(1), float32(1), []string{}, map[string]string{}, struct{}{},
	} {
		if err := s.Validate(v); err == nil {
			t.Errorf("Validate(%#v) = nil, want a type violation", v)
		}
	}
}

func TestValidate_BooleanSchemas(t *testing.T) {
	if err := mustCompile(t, `true`).Validate("x"); err != nil {
		t.Errorf("true.Validate() = %v, want nil", err)
	}
	got := violations(t, mustCompile(t, `false`).Validate(nil))
	if want := []schema.Violation{{Path: "", Keyword: "false"}}; !slices.Equal(got, want) {
		t.Errorf("false.Validate() violations = %v, want %v", got, want)
	}
}

func TestValidate_EnumScalars(t *testing.T) {
	s := mustCompile(t, `{"enum": ["A", true, null]}`)
	for _, tt := range []struct {
		value any
		want  bool
	}{
		{value: "A", want: true},
		{value: true, want: true},
		{value: nil, want: true},
		{value: "a", want: false},
		{value: false, want: false},
		{value: json.Number("1"), want: false},
		{value: []any{"A"}, want: false},
	} {
		if got := s.Validate(tt.value) == nil; got != tt.want {
			t.Errorf("Validate(%#v) ok = %t, want %t", tt.value, got, tt.want)
		}
	}
}

func TestValidate_TruncatesViolations(t *testing.T) {
	s := mustCompile(t, `{"additionalProperties": false}`)
	obj := map[string]any{}
	for i := range schema.MaxViolations + 5 {
		obj[strings.Repeat("k", i+1)] = nil
	}
	var ve *schema.ValidationError
	if !errors.As(s.Validate(obj), &ve) {
		t.Fatal("Validate() did not return a *schema.ValidationError")
	}
	if len(ve.Violations) != schema.MaxViolations || !ve.Truncated {
		t.Errorf("Validate() = %d violations, truncated %t; want %d, true", len(ve.Violations), ve.Truncated, schema.MaxViolations)
	}
	if msg := ve.Error(); !strings.HasSuffix(msg, "(violations: 100 or more): \"/k\": additionalProperties") {
		t.Errorf("Error() = %q, want the count and the first violation", msg)
	}
}

// TestValidationError_Error checks that the message never echoes a value and
// bounds the untrusted path.
func TestValidationError_Error(t *testing.T) {
	s := mustCompile(t, `{"additionalProperties": false, "properties": {"a": {"type": "boolean"}}}`)
	tests := []struct {
		name  string
		value map[string]any
		want  string
	}{
		{
			name:  "names the first violation, never a value",
			value: map[string]any{"a": "secret-value", "b": json.Number("1")},
			want:  `schema: value does not match (violations: 2): "/a": type`,
		},
		{
			name:  "cuts a long path",
			value: map[string]any{strings.Repeat("x", 500): json.Number("1")},
			want:  `schema: value does not match (violations: 1): "/` + strings.Repeat("x", schema.MaxPathBytes-2) + `~": additionalProperties`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Validate(tt.value).Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
	if got, want := (&schema.ValidationError{}).Error(), "schema: value does not match"; got != want {
		t.Errorf("empty Error() = %q, want %q", got, want)
	}
}

// TestValidate_PathBound checks that deep long keys cannot make the stored
// paths larger than MaxPathBytes each.
func TestValidate_PathBound(t *testing.T) {
	s := mustCompile(t, `{"properties": {"a": {"additionalProperties": {"additionalProperties": false}}}}`)
	inner := map[string]any{}
	for i := range 10 {
		inner[strings.Repeat("y", i+1)] = nil
	}
	got := violations(t, s.Validate(map[string]any{"a": map[string]any{strings.Repeat("k", 1<<16): inner}}))
	if len(got) != 10 {
		t.Fatalf("Validate() = %d violations, want 10", len(got))
	}
	for _, v := range got {
		if len(v.Path) > schema.MaxPathBytes {
			t.Errorf("Violation.Path is %d bytes, want at most %d", len(v.Path), schema.MaxPathBytes)
		}
	}
}

// TestValidate_PathMarker checks that a cut path ends in "~", keeps runes and
// ~0, ~1 escapes whole, and stays as it is for the values below it.
func TestValidate_PathMarker(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		value  map[string]any
		want   string // the path before the marker; "" for any
	}{
		{name: "long key", schema: `{"additionalProperties": false}`, value: map[string]any{strings.Repeat("x", 300): nil}, want: "/" + strings.Repeat("x", schema.MaxPathBytes-2)},
		{name: "escape at the cut", schema: `{"additionalProperties": false}`, value: map[string]any{"a" + strings.Repeat("~", 200): nil}},
		{name: "rune at the cut", schema: `{"additionalProperties": false}`, value: map[string]any{"a" + strings.Repeat("é", 200): nil}},
		{name: "invalid UTF-8 key", schema: `{"additionalProperties": false}`, value: map[string]any{strings.Repeat("\x80", 300): nil}, want: ""},
		{
			name:   "nested invalid UTF-8 key",
			schema: `{"additionalProperties": {"additionalProperties": false}}`,
			value:  map[string]any{"a": map[string]any{strings.Repeat("\x80", 300): nil}},
			want:   "/a",
		},
		{
			name:   "value below a short cut path",
			schema: `{"additionalProperties": {"additionalProperties": {"additionalProperties": false}}}`,
			value:  map[string]any{"a": map[string]any{strings.Repeat("\x80", 300): map[string]any{"b": nil}}},
			want:   "/a",
		},
		{
			name:   "value below a cut path",
			schema: `{"additionalProperties": {"additionalProperties": false}}`,
			value:  map[string]any{strings.Repeat("x", 300): map[string]any{"b": nil}},
			want:   "/" + strings.Repeat("x", schema.MaxPathBytes-2),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := violations(t, mustCompile(t, tt.schema).Validate(tt.value))
			if len(got) != 1 {
				t.Fatalf("Validate() = %v, want one violation", got)
			}
			path, cut := strings.CutSuffix(got[0].Path, "~")
			if !cut || len(got[0].Path) > schema.MaxPathBytes || !utf8.ValidString(path) || (tt.want != "" && path != tt.want) {
				t.Errorf("Violation.Path = %q, want %q and a final ~, at most %d bytes, valid UTF-8", got[0].Path, tt.want, schema.MaxPathBytes)
			}
			if strings.HasSuffix(path, "~") {
				t.Errorf("Violation.Path = %q splits a ~ escape", got[0].Path)
			}
		})
	}
}

// TestValidate_SharedWideValue checks that MaxValueNodes bounds the memory of
// the walk: children count against it before they are added, so a small Go
// value that holds itself many times cannot grow the walked tree past the cap.
func TestValidate_SharedWideValue(t *testing.T) {
	wide := make([]any, 2000)
	for i := range wide {
		wide[i] = wide
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got := violations(t, mustCompile(t, `true`).Validate(wide))
	runtime.ReadMemStats(&after)
	if len(got) != 1 || got[0].Keyword != "type" {
		t.Errorf("Validate(shared wide slice) = %v, want one type violation", got)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 512<<20 {
		t.Errorf("Validate(shared wide slice) allocated %d MiB, want at most 512", alloc>>20)
	}
}

// TestValidate_ValueCaps pins MaxValueDepth and MaxValueNodes at the cap and
// one over it; it fails when either comparison is off by one.
func TestValidate_ValueCaps(t *testing.T) {
	nest := func(levels int) any {
		var v any = []any{}
		for range levels - 1 {
			v = []any{v}
		}
		return v
	}
	nils := func(n int) any { return make([]any, n) }
	tests := []struct {
		name  string
		value any
		fails bool
	}{
		{name: "nesting at MaxValueDepth", value: nest(schema.MaxValueDepth)},
		{name: "nesting over MaxValueDepth", value: nest(schema.MaxValueDepth + 1), fails: true},
		{name: "MaxValueNodes values", value: nils(schema.MaxValueNodes - 1)},
		{name: "one value over MaxValueNodes", value: nils(schema.MaxValueNodes), fails: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := violations(t, mustCompile(t, `true`).Validate(tt.value))
			if tt.fails != (len(got) == 1 && got[0].Keyword == "type") || !tt.fails && got != nil {
				t.Errorf("Validate(%s) = %v, want one type violation: %t", tt.name, got, tt.fails)
			}
		})
	}
}

// visitChain returns a schema that checks the root value links times, once
// through each $ref of a chain, with keywords beside each $ref.
func visitChain(links int, keywords string) string {
	var b strings.Builder
	b.WriteString(`{"$defs": {`)
	for i := 1; i < links; i++ {
		fmt.Fprintf(&b, `"d%d": {"$ref": "#/$defs/d%d", %s}, `, i, i+1, keywords)
	}
	fmt.Fprintf(&b, `"d%d": {%s}}, "$ref": "#/$defs/d1"}`, links, keywords)
	return b.String()
}

// TestValidate_VisitsDoNotAllocate checks that each value's keys are sorted
// and each number parsed once, not once per subschema that checks it: the
// allocations of a valid value do not grow with the visits.
func TestValidate_VisitsDoNotAllocate(t *testing.T) {
	tests := []struct {
		name     string
		keywords string
		value    any
	}{
		{name: "object keys", keywords: `"required": ["a"]`, value: map[string]any{"a": nil, "b": true}},
		{name: "number", keywords: `"minimum": 1, "type": "number"`, value: 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocs := func(links int) float64 {
				s := mustCompile(t, visitChain(links, tt.keywords))
				return testing.AllocsPerRun(20, func() {
					if err := s.Validate(tt.value); err != nil {
						t.Fatalf("Validate() = %v", err)
					}
				})
			}
			if one, many := allocs(1), allocs(50); many != one {
				t.Errorf("Validate() allocations = %v through 50 subschemas, want %v as through one", many, one)
			}
		})
	}
}

// TestValidate_RefSiblings checks the 2020-12 rule that keywords beside
// $ref are asserted too (draft-07 ignored them).
func TestValidate_RefSiblings(t *testing.T) {
	s := mustCompile(t, `{"$defs": {"S": {"type": "string"}}, "$ref": "#/$defs/S", "enum": ["a"]}`)
	got := violations(t, s.Validate("b"))
	if want := []schema.Violation{{Path: "", Keyword: "enum"}}; !slices.Equal(got, want) {
		t.Errorf("Validate(%q) violations = %v, want %v", "b", got, want)
	}
	got = violations(t, s.Validate(nil))
	if want := []schema.Violation{{Path: "", Keyword: "type"}, {Path: "", Keyword: "enum"}}; !slices.Equal(got, want) {
		t.Errorf("Validate(nil) violations = %v, want %v", got, want)
	}
}

func TestSchema_ZeroValueRejects(t *testing.T) {
	var zero schema.Schema
	for _, s := range []*schema.Schema{nil, &zero} {
		if err := s.Validate("x"); err == nil {
			t.Errorf("(%v).Validate() = nil, want an error", s)
		}
	}
}

// TestValidate_DeepValue checks that a deep value costs no deeper recursion
// than the schema: the extra levels are never visited.
func TestValidate_DeepValue(t *testing.T) {
	s := mustCompile(t, `{"type": "array"}`)
	deep := strings.Repeat("[", 5000) + strings.Repeat("]", 5000)
	if err := s.Validate(decode(t, deep)); err != nil {
		t.Errorf("Validate(deep array) = %v, want nil", err)
	}
}

// TestValidate_SelfReferencingValue checks that a Go value holding itself, or
// sharing a container, ends in a type violation instead of walking forever.
func TestValidate_SelfReferencingValue(t *testing.T) {
	m := map[string]any{}
	m["a"] = m
	s := []any{nil}
	s[0] = s
	shared := []any{nil, nil}
	for range 40 {
		shared = []any{shared, shared}
	}
	for name, v := range map[string]any{"map": m, "slice": s, "shared slices": shared} {
		err := mustCompile(t, `true`).Validate(v)
		var ve *schema.ValidationError
		if !errors.As(err, &ve) || ve.Violations[0].Keyword != "type" {
			t.Errorf("Validate(self-referencing %s) = %v, want a type violation", name, err)
		}
	}
}

// TestValidate_GeneratedMetadata validates example metadata against the
// generated Metadata schema.
func TestValidate_GeneratedMetadata(t *testing.T) {
	b, err := os.ReadFile("../../../schemas/gen/numinance.kinds.v1.Metadata.jsonschema.strict.bundle.json")
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}
	s, err := schema.Compile(bytes.TrimSpace(b))
	if err != nil {
		t.Fatalf("Compile(Metadata) = %v", err)
	}
	if err := s.Validate(decode(t, `{"name": "org-default", "scope": "SCOPE_ORG"}`)); err != nil {
		t.Errorf("Validate(valid metadata) = %v, want nil", err)
	}
	got := violations(t, s.Validate(decode(t, `{"name": "x", "scope": "org", "labels": {}}`)))
	want := []schema.Violation{{Path: "/labels", Keyword: "additionalProperties"}, {Path: "/scope", Keyword: "enum"}}
	if !slices.Equal(got, want) {
		t.Errorf("Validate(bad metadata) violations = %v, want %v", got, want)
	}
}
