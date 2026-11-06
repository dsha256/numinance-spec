package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	// MaxDepth bounds the longest chain of subschemas, $ref steps included,
	// and so the recursion of Validate whatever the value.
	MaxDepth = 64
	// MaxVisits bounds how many subschemas can check one value (anyOf branches
	// and $ref each check the same value), so a value costs at most MaxVisits checks.
	MaxVisits = 1024
	draft     = "https://json-schema.org/draft/2020-12/schema"
)

var (
	// ErrMalformed reports a document that is not JSON or a keyword value of the wrong shape.
	ErrMalformed = errors.New("schema: malformed schema")
	// ErrUnsupported reports a keyword, dialect or value this package does not implement.
	ErrUnsupported = errors.New("schema: unsupported")
	// ErrRef reports a $ref that is not local or does not resolve to a schema.
	ErrRef = errors.New("schema: bad $ref")
	// ErrCycle reports a schema that can reach itself through its subschemas.
	ErrCycle = errors.New("schema: $ref cycle")
	// ErrDepth reports a schema deeper than MaxDepth or over MaxVisits.
	ErrDepth = errors.New("schema: schema too deep")
)

// Schema is a document from Compile, safe for concurrent use; the zero value rejects every value.
type Schema struct {
	root *node
}

// node is one compiled subschema. A boolean schema sets only always.
type node struct {
	ptr      string
	always   *bool
	ref      string // JSON Pointer of the $ref target, without the '#'
	target   *node
	types    []string
	props    map[string]*node
	addl     *node
	required []string
	enum     []any // strings, booleans and nil; nil when absent
	min, max *decimal
	pattern  *regexp.Regexp
	items    *node
	names    *node // propertyNames
	anyOf    []*node
}

// size is the height and the visit bound of a measured node; h is -1 while on the DFS stack.
type size struct{ h, visits int }

type compiler struct {
	nodes map[string]*node // by JSON Pointer from the document root
	refs  []*node
	sizes map[*node]size
	from  string // pointer of the node measure started from
}

// Compile compiles a JSON Schema 2020-12 document, a trusted generated file; callers size-limit it.
func Compile(doc []byte) (*Schema, error) {
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: data after the document", ErrMalformed)
	}
	c := &compiler{nodes: map[string]*node{}, sizes: map[*node]size{}}
	root, err := c.compile(raw, "", 0)
	if err != nil {
		return nil, err
	}
	for _, n := range c.refs {
		if n.target = c.nodes[n.ref]; n.target == nil {
			return nil, fmt.Errorf("%w: %q at %q does not resolve to a schema", ErrRef, "#"+n.ref, n.ptr)
		}
	}
	for _, p := range slices.Sorted(maps.Keys(c.nodes)) {
		c.from = p
		if _, err := c.measure(c.nodes[p], 0); err != nil {
			return nil, err
		}
	}
	return &Schema{root: root}, nil
}

func (c *compiler) compile(raw any, ptr string, depth int) (*node, error) {
	if depth > MaxDepth {
		return nil, fmt.Errorf("%w: nesting over %d at %q", ErrDepth, MaxDepth, ptr)
	}
	n := &node{ptr: ptr}
	c.nodes[ptr] = n
	switch v := raw.(type) {
	case bool:
		n.always = &v
		return n, nil
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(v)) {
			if err := c.keyword(n, k, v[k], depth); err != nil {
				return nil, err
			}
		}
		if _, ref := v["$ref"]; ref && (n.props != nil || n.addl != nil) { // never generated; lifting it is additive
			return nil, fmt.Errorf("%w: properties beside $ref at %q", ErrUnsupported, ptr)
		}
		return n, nil
	}
	return nil, fmt.Errorf("%w: schema at %q is not an object or a boolean", ErrMalformed, ptr)
}

func (c *compiler) keyword(n *node, k string, val any, depth int) error {
	at := n.ptr + "/" + escape(k)
	var err error
	if _, isString := val.(string); !isString && slices.Contains([]string{"$schema", "$id", "$ref", "title", "description", "format", "pattern"}, k) {
		return fmt.Errorf("%w: %q is not a string", ErrMalformed, at)
	}
	switch k {
	case "$schema":
		if val != draft {
			return fmt.Errorf("%w: $schema at %q is not %s", ErrUnsupported, at, draft)
		}
	case "$id":
		if n.ptr != "" {
			return fmt.Errorf("%w: $id below the root at %q", ErrUnsupported, at)
		}
	case "title", "description", "format":
	case "$defs", "properties":
		m, ok := val.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %q is not an object", ErrMalformed, at)
		}
		props := map[string]*node{}
		for _, name := range slices.Sorted(maps.Keys(m)) {
			if props[name], err = c.compile(m[name], at+"/"+escape(name), depth+1); err != nil {
				return err
			}
		}
		if k == "properties" {
			n.props = props
		}
	case "additionalProperties":
		n.addl, err = c.compile(val, at, depth+1)
	case "items":
		n.items, err = c.compile(val, at, depth+1)
	case "propertyNames":
		n.names, err = c.compile(val, at, depth+1)
	case "anyOf":
		list, _ := val.([]any)
		if len(list) == 0 {
			return fmt.Errorf("%w: %q is not a non-empty array", ErrMalformed, at)
		}
		for i, sub := range list {
			b, err := c.compile(sub, at+"/"+strconv.Itoa(i), depth+1)
			if err != nil {
				return err
			}
			n.anyOf = append(n.anyOf, b)
		}
	case "minimum", "maximum":
		num, _ := val.(json.Number)
		d, ok := parseNumber(string(num))
		if !ok {
			return fmt.Errorf("%w: %q is not a number", ErrMalformed, at)
		}
		if max(d.point, -d.point) > maxExp/2 {
			return fmt.Errorf("%w: %q has an exponent over 2^39", ErrUnsupported, at)
		}
		if k == "minimum" {
			n.min = &d
		} else {
			n.max = &d
		}
	case "pattern":
		// Go regexp (RE2): linear in the value, limited to the subset ECMA-262 reads alike.
		if why := patternSubset(val.(string)); why != "" {
			return fmt.Errorf("%w: pattern at %q uses %s", ErrUnsupported, at, why)
		}
		if n.pattern, err = regexp.Compile(val.(string)); err != nil {
			return fmt.Errorf("%w: pattern at %q: %w", ErrUnsupported, at, err)
		}
	case "$ref":
		s, _ := val.(string)
		frag, local := strings.CutPrefix(s, "#")
		if !local || (frag != "" && frag[0] != '/') || strings.Contains(frag, "%") {
			return fmt.Errorf("%w: %q at %q is not a local JSON Pointer (#/...)", ErrRef, s, at)
		}
		n.ref = frag
		c.refs = append(c.refs, n)
	case "type":
		if s, ok := val.(string); ok {
			val = []any{s}
		}
		if n.types, err = stringList(val, at); err == nil && len(n.types) == 0 {
			err = fmt.Errorf("%w: %q is empty", ErrMalformed, at)
		}
		for _, t := range n.types {
			switch t {
			case "array", "boolean", "integer", "null", "number", "object", "string":
			default:
				return fmt.Errorf("%w: %q names an unknown type", ErrMalformed, at)
			}
		}
	case "required":
		n.required, err = stringList(val, at)
	case "enum":
		list, _ := val.([]any)
		if len(list) == 0 {
			return fmt.Errorf("%w: %q is not a non-empty array", ErrMalformed, at)
		}
		for i, e := range list {
			switch e.(type) {
			case nil, bool, string:
			default:
				return fmt.Errorf("%w: %s/%d: enum values are strings, booleans or null", ErrUnsupported, at, i)
			}
			if slices.Contains(list[:i], e) {
				return fmt.Errorf("%w: %s/%d repeats a value", ErrMalformed, at, i)
			}
		}
		n.enum = list
	default:
		return fmt.Errorf("%w: keyword %q at %q", ErrUnsupported, k, n.ptr)
	}
	return err
}

// measure returns n's size, failing on a cycle or a size over the caps; it
// recurses once per node and at most MaxDepth deep: n at depth d below the
// first node measured makes that node at least d high. A cycle longer than
// MaxDepth is reported as ErrDepth.
func (c *compiler) measure(n *node, depth int) (size, error) {
	if depth > MaxDepth {
		return size{}, fmt.Errorf("%w: %q is more than %d levels of subschemas below %q", ErrDepth, n.ptr, MaxDepth, c.from)
	}
	if z, seen := c.sizes[n]; seen {
		if z.h < 0 {
			return size{}, fmt.Errorf("%w: through %q", ErrCycle, n.ptr)
		}
		return z, nil
	}
	c.sizes[n] = size{h: -1}
	same := append([]*node{n.target}, n.anyOf...)
	below := []*node{n.addl, n.items, n.names}
	for _, k := range slices.Sorted(maps.Keys(n.props)) {
		below = append(below, n.props[k])
	}
	z, deepest := size{visits: 1}, 0
	for i, ch := range append(same, below...) {
		if ch == nil {
			continue
		}
		zc, err := c.measure(ch, depth+1)
		if err != nil {
			return size{}, err
		}
		z.h = max(z.h, zc.h+1)
		if i < len(same) {
			z.visits += zc.visits
		} else {
			deepest = max(deepest, zc.visits)
		}
	}
	z.visits += deepest // a child value is checked through at most one of below, plus through same
	if z.h > MaxDepth || z.visits > MaxVisits {
		return size{}, fmt.Errorf("%w: %q has %d levels of subschemas and %d visits per value", ErrDepth, n.ptr, z.h, z.visits)
	}
	c.sizes[n] = z
	return z, nil
}

func stringList(val any, at string) ([]string, error) {
	list, _ := val.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok || slices.Contains(out, s) {
			return nil, fmt.Errorf("%w: %q is not an array of unique strings", ErrMalformed, at)
		}
		out = append(out, s)
	}
	if list == nil {
		return nil, fmt.Errorf("%w: %q is not an array", ErrMalformed, at)
	}
	return out, nil
}

// escape encodes one JSON Pointer reference token (RFC 6901).
func escape(token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
}
