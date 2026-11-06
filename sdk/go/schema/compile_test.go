package schema_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/schema"
)

func TestCompile_Accepts(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{name: "true schema", doc: `true`},
		{name: "false schema", doc: `false`},
		{name: "empty object", doc: `{}`},
		{name: "annotations", doc: `{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "x.json", "title": "T", "description": "D", "format": "duration"}`},
		{name: "type array", doc: `{"type": ["string", "null"]}`},
		{name: "numeric types", doc: `{"type": ["integer", "number"]}`},
		{name: "bounds", doc: `{"minimum": -1.5e3, "maximum": 4294967295}`},
		{name: "bounds at half the exponent saturation", doc: `{"minimum": -1e549755813887, "maximum": 1e549755813887}`},
		{name: "constraint keywords", doc: `{"pattern": "^a+$", "items": true, "propertyNames": {"type": "string"}, "anyOf": [{"type": "string"}, false]}`},
		{name: "ref into anyOf", doc: `{"anyOf": [{"type": "null"}], "items": {"$ref": "#/anyOf/0"}}`},
		{name: "ref with escaped token", doc: `{"$defs": {"a/b~c": {"type": "string"}}, "$ref": "#/$defs/a~1b~0c"}`},
		{name: "ref into properties", doc: `{"properties": {"a": {"type": "string"}, "b": {"$ref": "#/properties/a"}}}`},
		{name: "shared definition (diamond)", doc: `{"$defs": {"s": {"type": "string"}}, "properties": {"a": {"$ref": "#/$defs/s"}, "b": {"$ref": "#/$defs/s"}}}`},
		{name: "enum of scalars", doc: `{"enum": ["A", true, null]}`},
		{name: "unused definition", doc: `{"$defs": {"unused": {"type": "object"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := schema.Compile([]byte(tt.doc)); err != nil {
				t.Errorf("Compile(%s) = %v, want nil", tt.doc, err)
			}
		})
	}
}

func TestCompile_Rejects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want error
	}{
		{name: "unsupported keyword at root", doc: `{"oneOf": [true]}`, want: schema.ErrUnsupported},
		{name: "unsupported keyword in a property", doc: `{"properties": {"a": {"exclusiveMinimum": 0}}}`, want: schema.ErrUnsupported},
		{name: "unsupported keyword in a definition", doc: `{"$defs": {"a": {"not": true}}}`, want: schema.ErrUnsupported},
		{name: "unsupported keyword in items", doc: `{"items": {"minLength": 1}}`, want: schema.ErrUnsupported},
		{name: "unsupported keyword in an anyOf branch", doc: `{"anyOf": [{"const": 1}]}`, want: schema.ErrUnsupported},
		{name: "unsupported keyword in propertyNames", doc: `{"propertyNames": {"maxLength": 3}}`, want: schema.ErrUnsupported},
		{name: "unsupported keyword in additionalProperties", doc: `{"additionalProperties": {"patternProperties": {}}}`, want: schema.ErrUnsupported},
		{name: "unsupported keyword in an unused definition", doc: `{"$defs": {"u": {"allOf": []}}}`, want: schema.ErrUnsupported},
		{name: "default is not an implemented annotation", doc: `{"default": 1}`, want: schema.ErrUnsupported},
		{name: "$anchor", doc: `{"$anchor": "a"}`, want: schema.ErrUnsupported},
		{name: "$dynamicRef", doc: `{"$dynamicRef": "#a"}`, want: schema.ErrUnsupported},
		{name: "older dialect", doc: `{"$schema": "http://json-schema.org/draft-07/schema#"}`, want: schema.ErrUnsupported},
		{name: "$id below the root", doc: `{"$defs": {"a": {"$id": "a.json"}}}`, want: schema.ErrUnsupported},
		{name: "numeric enum value", doc: `{"enum": [1]}`, want: schema.ErrUnsupported},
		{name: "object enum value", doc: `{"enum": [{}]}`, want: schema.ErrUnsupported},
		{name: "ref to another document", doc: `{"$ref": "other.json#/$defs/a"}`, want: schema.ErrRef},
		{name: "ref to a URL", doc: `{"$ref": "https://example.com/s.json"}`, want: schema.ErrRef},
		{name: "ref to an anchor", doc: `{"$ref": "#a"}`, want: schema.ErrRef},
		{name: "percent-encoded ref", doc: `{"$defs": {"a": true}, "$ref": "#/%24defs/a"}`, want: schema.ErrRef},
		{name: "properties beside $ref", doc: `{"$defs": {"a": true}, "$ref": "#/$defs/a", "properties": {"b": true}}`, want: schema.ErrUnsupported},
		{name: "additionalProperties beside $ref", doc: `{"$defs": {"a": true}, "$ref": "#/$defs/a", "additionalProperties": false}`, want: schema.ErrUnsupported},
		{name: "properties beside a $ref to the root", doc: `{"$defs":{"x":{"$ref":"#","properties":{"a":true}}}}`, want: schema.ErrUnsupported},
		{name: "minimum exponent over half the saturation", doc: `{"minimum": 1e600000000000}`, want: schema.ErrUnsupported},
		{name: "maximum exponent under minus half the saturation", doc: `{"maximum": -1e-600000000000}`, want: schema.ErrUnsupported},
		{name: "unresolved ref", doc: `{"$ref": "#/$defs/missing"}`, want: schema.ErrRef},
		{name: "ref to a non-schema location", doc: `{"$defs": {}, "$ref": "#/$defs"}`, want: schema.ErrRef},
		{name: "ref into required", doc: `{"required": ["a"], "$ref": "#/required/0"}`, want: schema.ErrRef},
		{name: "ref is not a string", doc: `{"$ref": 1}`, want: schema.ErrMalformed},
		{name: "not JSON", doc: `{`, want: schema.ErrMalformed},
		{name: "trailing data", doc: `{} {}`, want: schema.ErrMalformed},
		{name: "array schema", doc: `[]`, want: schema.ErrMalformed},
		{name: "number schema", doc: `1`, want: schema.ErrMalformed},
		{name: "unknown type name", doc: `{"type": "int"}`, want: schema.ErrMalformed},
		{name: "minimum not a number", doc: `{"minimum": "0"}`, want: schema.ErrMalformed},
		{name: "maximum not a number", doc: `{"maximum": null}`, want: schema.ErrMalformed},
		{name: "pattern not a string", doc: `{"pattern": 1}`, want: schema.ErrMalformed},
		{name: "pattern outside RE2", doc: `{"pattern": "(a)\\1"}`, want: schema.ErrUnsupported},
		{name: "pattern with lookahead", doc: `{"pattern": "a(?=b)"}`, want: schema.ErrUnsupported},
		{name: "unclosed class", doc: `{"pattern": "[a"}`, want: schema.ErrUnsupported},
		{name: "empty anyOf", doc: `{"anyOf": []}`, want: schema.ErrMalformed},
		{name: "anyOf not an array", doc: `{"anyOf": {}}`, want: schema.ErrMalformed},
		{name: "anyOf branch not a schema", doc: `{"anyOf": [1]}`, want: schema.ErrMalformed},
		{name: "items not a schema", doc: `{"items": [true]}`, want: schema.ErrMalformed},
		{name: "propertyNames not a schema", doc: `{"propertyNames": "string"}`, want: schema.ErrMalformed},
		{name: "unresolved ref in items", doc: `{"items": {"$ref": "#/$defs/x"}}`, want: schema.ErrRef},
		{name: "empty type array", doc: `{"type": []}`, want: schema.ErrMalformed},
		{name: "duplicate type", doc: `{"type": ["string", "string"]}`, want: schema.ErrMalformed},
		{name: "required not an array", doc: `{"required": "a"}`, want: schema.ErrMalformed},
		{name: "required not strings", doc: `{"required": [1]}`, want: schema.ErrMalformed},
		{name: "duplicate required", doc: `{"required": ["a", "a"]}`, want: schema.ErrMalformed},
		{name: "repeated enum value", doc: `{"enum": ["a", "a"]}`, want: schema.ErrMalformed},
		{name: "empty enum", doc: `{"enum": []}`, want: schema.ErrMalformed},
		{name: "properties not an object", doc: `{"properties": []}`, want: schema.ErrMalformed},
		{name: "property schema not a schema", doc: `{"properties": {"a": "string"}}`, want: schema.ErrMalformed},
		{name: "title not a string", doc: `{"title": 1}`, want: schema.ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := schema.Compile([]byte(tt.doc))
			if !errors.Is(err, tt.want) {
				t.Errorf("Compile(%.80s) = %v, want %v", tt.doc, err, tt.want)
			}
			if s != nil {
				t.Errorf("Compile(%.80s) returned a schema with an error", tt.doc)
			}
		})
	}
}

func TestCompile_RefCycle(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{name: "self reference", doc: `{"$ref": "#"}`},
		{name: "definition refers to itself", doc: `{"$defs": {"a": {"$ref": "#/$defs/a"}}, "$ref": "#/$defs/a"}`},
		{name: "two definitions refer to each other", doc: `{"$defs": {"a": {"$ref": "#/$defs/b"}, "b": {"$ref": "#/$defs/a"}}}`},
		{name: "recursion through properties", doc: `{"type": "object", "properties": {"child": {"$ref": "#"}}}`},
		{name: "recursion through additionalProperties", doc: `{"$defs": {"t": {"additionalProperties": {"$ref": "#/$defs/t"}}}}`},
		{name: "recursion through items", doc: `{"items": {"$ref": "#"}}`},
		{name: "recursion through anyOf", doc: `{"$defs": {"t": {"anyOf": [{"type": "null"}, {"$ref": "#/$defs/t"}]}}}`},
		{name: "recursion through propertyNames", doc: `{"propertyNames": {"$ref": "#"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := schema.Compile([]byte(tt.doc)); !errors.Is(err, schema.ErrCycle) {
				t.Errorf("Compile(%s) = %v, want %v", tt.doc, err, schema.ErrCycle)
			}
		})
	}
}

// nestedProperties returns a schema whose properties nest levels deep.
func nestedProperties(levels int) string {
	return strings.Repeat(`{"properties": {"a": `, levels) + `true` + strings.Repeat(`}}`, levels)
}

// refChain returns a schema whose root reaches a leaf through links $refs.
func refChain(links int) string {
	var b strings.Builder
	b.WriteString(`{"$defs": {`)
	for i := 1; i < links; i++ {
		fmt.Fprintf(&b, `"d%d": {"$ref": "#/$defs/d%d"}, `, i, i+1)
	}
	fmt.Fprintf(&b, `"d%d": {"type": "string"}}, "$ref": "#/$defs/d1"}`, links)
	return b.String()
}

// refCycle returns a schema whose definitions form one $ref cycle of links.
func refCycle(links int) string {
	var b strings.Builder
	b.WriteString(`{"$defs": {`)
	for i := range links {
		fmt.Fprintf(&b, `"d%d": {"$ref": "#/$defs/d%d"}, `, i, (i+1)%links)
	}
	return strings.TrimSuffix(b.String(), ", ") + `}}`
}

// sharedAnyOf returns a schema whose definitions each check the value twice
// through the next one: 3*levels+2 nodes, 2^(levels+2)-2 visits.
func sharedAnyOf(levels int) string {
	var b strings.Builder
	b.WriteString(`{"$defs": {`)
	for i := range levels {
		fmt.Fprintf(&b, `"d%d": {"anyOf": [{"$ref": "#/$defs/d%d"}, {"$ref": "#/$defs/d%d"}]}, `, i, i+1, i+1)
	}
	fmt.Fprintf(&b, `"d%d": true}, "$ref": "#/$defs/d0"}`, levels)
	return b.String()
}

func TestCompile_DepthCap(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want error
	}{
		{name: "nested properties at the cap", doc: nestedProperties(schema.MaxDepth), want: nil},
		{name: "nested properties over the cap", doc: nestedProperties(schema.MaxDepth + 1), want: schema.ErrDepth},
		{name: "far over the cap", doc: nestedProperties(10 * schema.MaxDepth), want: schema.ErrDepth},
		{name: "ref chain at the cap", doc: refChain(schema.MaxDepth), want: nil},
		{name: "ref chain over the cap", doc: refChain(schema.MaxDepth + 1), want: schema.ErrDepth},
		{name: "nested $defs over the cap", doc: strings.Repeat(`{"$defs":{"a":`, 5*schema.MaxDepth) + "true" + strings.Repeat("}}", 5*schema.MaxDepth), want: schema.ErrDepth},
		{name: "long ref chain", doc: refChain(10 * schema.MaxDepth), want: schema.ErrDepth},
		{name: "cycle longer than the cap", doc: refCycle(schema.MaxDepth + 2), want: schema.ErrDepth},
		{name: "nested items over the cap", doc: strings.Repeat(`{"items":`, schema.MaxDepth+1) + "true" + strings.Repeat("}", schema.MaxDepth+1), want: schema.ErrDepth},
		{name: "shared anyOf at the visit cap", doc: sharedAnyOf(8), want: nil},
		{name: "shared anyOf over the visit cap", doc: sharedAnyOf(9), want: schema.ErrDepth},
		{name: "shared anyOf far over the visit cap", doc: sharedAnyOf(schema.MaxDepth - 1), want: schema.ErrDepth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := schema.Compile([]byte(tt.doc)); !errors.Is(err, tt.want) {
				t.Errorf("Compile(%s) = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}

// TestCompile_LongRefChainStack checks that a long $ref chain costs Compile no
// deeper recursion than MaxDepth: without the bound it overflows the stack and
// crashes the test binary. debug.SetMaxStack is process-wide, so this test
// and every test of the package must stay serial (no t.Parallel).
func TestCompile_LongRefChainStack(t *testing.T) {
	doc := []byte(refChain(20000))
	defer debug.SetMaxStack(debug.SetMaxStack(1 << 19))
	if _, err := schema.Compile(doc); !errors.Is(err, schema.ErrDepth) {
		t.Errorf("Compile(long ref chain) = %v, want %v", err, schema.ErrDepth)
	}
}

func TestCompile_DepthNamesPointer(t *testing.T) {
	_, err := schema.Compile([]byte(nestedProperties(schema.MaxDepth + 1)))
	if want := `at "/properties/a/properties/a`; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Compile(nested properties) = %v, want an error containing %s", err, want)
	}
}

// TestCompile_GeneratedSchemas pins the validator against every generated
// schema: each one compiles.
func TestCompile_GeneratedSchemas(t *testing.T) {
	const gen = "../../../schemas/gen/"
	entries, err := os.ReadDir(gen)
	if err != nil || len(entries) == 0 {
		t.Fatalf("ReadDir(%s) = %d entries, %v", gen, len(entries), err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(gen + e.Name())
		if err != nil {
			t.Fatalf("ReadFile(%s) = %v", e.Name(), err)
		}
		if _, err := schema.Compile(b); err != nil {
			t.Errorf("Compile(%s) = %v, want nil", e.Name(), err)
		}
	}
}

// TestCompile_PatternSubset pins the patterns RE2 and ECMA-262 read alike;
// everything else fails closed even where RE2 would accept it.
func TestCompile_PatternSubset(t *testing.T) {
	for _, tt := range []struct {
		pattern string
		ok      bool
	}{
		{pattern: `^[a-z][a-z0-9-]*(?:\.[a-z0-9-]+)+$`, ok: true},
		{pattern: `^\d{1,3}\w*\b[^\]\-]$`, ok: true},
		{pattern: `[.(?]\/\t`, ok: true},
		{pattern: `^a{2}b{1,}c{1,3}[}{]\{\}$`, ok: true},
		{pattern: `a{`},
		{pattern: `a{x}`},
		{pattern: `a{,3}`},
		{pattern: `a{1`},
		{pattern: `a}`},
		{pattern: `a]`},
		{pattern: `^a.b$`},
		{pattern: `\s`},
		{pattern: `\Aabc\z`},
		{pattern: `\Qa.b\E`},
		{pattern: `\pL`},
		{pattern: `(?i)abc`},
		{pattern: `(?P<n>a)`},
		{pattern: `[[:alpha:]]`},
		{pattern: `[]a]`},
		{pattern: `[^]a]`},
		{pattern: `[\b]`},
		{pattern: `a\-b`},
		{pattern: `\x41`},
		{pattern: `\u00e9`},
		{pattern: "\u00e9"},
		{pattern: `a\`},
		{pattern: "a\nb"},
	} {
		doc, _ := json.Marshal(map[string]string{"pattern": tt.pattern})
		_, err := schema.Compile(doc)
		if tt.ok && err != nil || !tt.ok && !errors.Is(err, schema.ErrUnsupported) {
			t.Errorf("Compile(pattern %q) = %v, want ok %t", tt.pattern, err, tt.ok)
		}
	}
}

// TestCompile_FailClosedKeywords pins the README table: every 2020-12 keyword
// this package does not implement fails Compile, wherever it appears.
func TestCompile_FailClosedKeywords(t *testing.T) {
	for _, k := range []string{
		"allOf", "oneOf", "not", "if", "then", "else", "dependentSchemas",
		"prefixItems", "contains", "patternProperties", "unevaluatedItems", "unevaluatedProperties",
		"const", "multipleOf", "exclusiveMinimum", "exclusiveMaximum", "minLength", "maxLength",
		"minItems", "maxItems", "uniqueItems", "minContains", "maxContains",
		"minProperties", "maxProperties", "dependentRequired",
		"$anchor", "$dynamicRef", "$dynamicAnchor", "$vocabulary", "$comment",
		"default", "examples", "deprecated", "readOnly", "writeOnly",
		"contentEncoding", "contentMediaType", "contentSchema",
	} {
		for _, doc := range []string{
			fmt.Sprintf(`{%q: true}`, k),
			fmt.Sprintf(`{"items": {"anyOf": [{%q: true}]}}`, k),
		} {
			if _, err := schema.Compile([]byte(doc)); !errors.Is(err, schema.ErrUnsupported) {
				t.Errorf("Compile(%s) = %v, want %v", doc, err, schema.ErrUnsupported)
			}
		}
	}
}
