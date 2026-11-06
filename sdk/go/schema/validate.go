package schema

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// MaxViolations bounds the violations one Validate call collects.
	MaxViolations = 100
	// MaxPathBytes bounds each Violation.Path; a longer path is cut and ends in "~".
	MaxPathBytes = 256
	// MaxValueDepth bounds value nesting as encoding/json does; deeper containers are a type violation.
	MaxValueDepth = 10000
	// MaxValueNodes bounds the values one Validate call walks, even through a Go value holding itself;
	// the container whose children would pass it is a type violation.
	MaxValueNodes = 1 << 20
)

// Violation is one way a value fails its schema. It never holds a value.
type Violation struct {
	Path    string // JSON Pointer of the failing value, cut at MaxPathBytes (then ending in "~"); built from untrusted keys
	Keyword string // additionalProperties (an unknown field), the failing keyword, or false
}

// ValidationError lists the violations of one Validate call in a stable order.
type ValidationError struct {
	Violations []Violation
	Truncated  bool // more than MaxViolations were found
}

// Error names the first violation, its path quoted, and the number of violations.
func (e *ValidationError) Error() string {
	if len(e.Violations) == 0 {
		return "schema: value does not match"
	}
	v, more := e.Violations[0], ""
	if e.Truncated {
		more = " or more"
	}
	return fmt.Sprintf("schema: value does not match (violations: %d%s): %s: %s", len(e.Violations), more, strconv.Quote(v.Path), v.Keyword)
}

// Validate returns nil or a *ValidationError; its recursion is bounded by the
// schema, not by v. A value outside the JSON data model anywhere in v, reached
// by the schema or not, is a type violation and stops validation.
func (s *Schema) Validate(v any) error {
	if s == nil || s.root == nil {
		return &ValidationError{Violations: []Violation{{Keyword: "false"}}} // not from Compile: fail closed
	}
	var e ValidationError
	vals := e.model(v)
	if len(e.Violations) == 0 {
		c := checker{vals: vals, out: &e}
		c.check(s.root, &vals[0], "")
	}
	if len(e.Violations) == 0 {
		return nil
	}
	return &e
}

func (e *ValidationError) fail(path, keyword string) {
	if len(e.Violations) == MaxViolations {
		e.Truncated = true
		return
	}
	e.Violations = append(e.Violations, Violation{Path: path, Keyword: keyword})
}

// val is one value of the walked tree. The walk sorts each object's keys and
// parses each number once, so the up to MaxVisits checks of a value do neither.
type val struct {
	v      any      // as given; containers are read through first and n
	num    *decimal // set for a number
	key    string   // the key of an object member
	parent int      // index of the container, -1 at the root
	first  int      // children are vals[first:first+count], objects in key order
	count  int
}

func (x *val) jsonType() string {
	if x.num != nil {
		return "number"
	}
	return jsonType(x.v)
}

// model walks v without recursion into a tree of vals, the root first, and
// fails each value outside the JSON data model.
func (e *ValidationError) model(v any) []val {
	type item struct {
		at    int
		depth int
	}
	vals, stack := []val{{v: v, parent: -1}}, []item{{}}
	for len(stack) > 0 && !e.Truncated {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		arr, isArr := vals[it.at].v.([]any)
		obj, isObj := vals[it.at].v.(map[string]any)
		over := len(vals)+len(arr)+len(obj) > MaxValueNodes // children count before they are added
		switch {
		case (isArr || isObj) && (over || it.depth >= MaxValueDepth):
		case isArr || isObj:
			first := len(vals)
			if isArr {
				for _, x := range arr {
					vals = append(vals, val{v: x, parent: it.at})
				}
			} else {
				for _, k := range slices.Sorted(maps.Keys(obj)) {
					vals = append(vals, val{v: obj[k], key: k, parent: it.at})
				}
			}
			vals[it.at].first, vals[it.at].count = first, len(vals)-first
			for i := len(vals) - 1; i >= first; i-- {
				stack = append(stack, item{at: i, depth: it.depth + 1})
			}
			continue
		default:
			if d, ok := numberOf(vals[it.at].v); ok {
				vals[it.at].num = &d
				continue
			}
			if jsonType(vals[it.at].v) != "" {
				continue
			}
		}
		e.fail(pathOf(vals, it.at), "type")
		if over {
			break
		}
	}
	return vals
}

// pathOf returns the JSON Pointer of vals[at].
func pathOf(vals []val, at int) string {
	var trail []string
	for i := at; vals[i].parent >= 0; i = vals[i].parent {
		if p := &vals[vals[i].parent]; jsonType(p.v) == "array" {
			trail = append(trail, strconv.Itoa(i-p.first))
		} else {
			trail = append(trail, vals[i].key)
		}
	}
	path := ""
	for _, k := range slices.Backward(trail) {
		path = child(path, k)
	}
	return path
}

// checker checks the tree of vals from model, appending to out.
type checker struct {
	vals []val
	out  *ValidationError
}

// passes reports whether x matches n, keeping no violations.
func (c checker) passes(n *node, x *val) bool {
	var scratch ValidationError
	checker{vals: c.vals, out: &scratch}.check(n, x, "")
	return len(scratch.Violations) == 0
}

func (c checker) check(n *node, x *val, ptr string) {
	e := c.out
	if e.Truncated {
		return
	}
	if n.always != nil {
		if !*n.always {
			e.fail(ptr, "false")
		}
		return
	}
	if n.target != nil {
		c.check(n.target, x, ptr)
	}
	t := x.jsonType()
	typed := slices.Contains(n.types, t) || (t == "number" && x.num.isInteger() && slices.Contains(n.types, "integer"))
	if n.types != nil && !typed {
		e.fail(ptr, "type")
	}
	if n.enum != nil && !inEnum(x.v, n.enum) {
		e.fail(ptr, "enum")
	}
	if n.min != nil && t == "number" && compareDecimal(*x.num, *n.min) < 0 {
		e.fail(ptr, "minimum")
	}
	if n.max != nil && t == "number" && compareDecimal(*x.num, *n.max) > 0 {
		e.fail(ptr, "maximum")
	}
	if str, ok := x.v.(string); ok && n.pattern != nil && !n.pattern.MatchString(str) {
		e.fail(ptr, "pattern")
	}
	if n.anyOf != nil && !slices.ContainsFunc(n.anyOf, func(b *node) bool { return c.passes(b, x) }) {
		e.fail(ptr, "anyOf")
	}
	kids := c.vals[x.first : x.first+x.count]
	if t == "array" && n.items != nil {
		for i := range kids {
			c.check(n.items, &kids[i], child(ptr, strconv.Itoa(i)))
		}
	}
	if t != "object" {
		return
	}
	obj := x.v.(map[string]any)
	for _, name := range n.required {
		if _, ok := obj[name]; !ok {
			e.fail(child(ptr, name), "required")
		}
	}
	for i := range kids {
		k := kids[i].key // child paths are built only when used: most visits use none
		if n.names != nil && !c.passes(n.names, &val{v: k}) {
			e.fail(child(ptr, k), "propertyNames")
		}
		switch p, ok := n.props[k]; {
		case ok:
			c.check(p, &kids[i], child(ptr, k))
		case n.addl == nil:
		case n.addl.always != nil && !*n.addl.always:
			e.fail(child(ptr, k), "additionalProperties")
		default:
			c.check(n.addl, &kids[i], child(ptr, k))
		}
	}
}

// child returns the path of key k below ptr. A path over MaxPathBytes is cut
// at a character boundary (a key of invalid UTF-8 may be cut back to its
// start) and ends in "~", which no JSON Pointer ends in. A cut path stays as it
// is: extending it could fit under the cap again and yield a pointer with a
// bare "~" in the middle.
func child(ptr, k string) string {
	if strings.HasSuffix(ptr, "~") {
		return ptr
	}
	p := ptr + "/" + escape(k[:min(len(k), MaxPathBytes)])
	if len(p) <= MaxPathBytes {
		return p
	}
	cut := MaxPathBytes - 1
	for cut > 1 && (!utf8.RuneStart(p[cut]) || p[cut-1] == '~') { // keep runes of valid UTF-8 and ~0, ~1 whole
		cut--
	}
	return p[:cut] + "~"
}

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return ""
}

func inEnum(v any, enum []any) bool {
	switch v.(type) {
	case nil, bool, string:
		return slices.Contains(enum, v)
	}
	return false
}
