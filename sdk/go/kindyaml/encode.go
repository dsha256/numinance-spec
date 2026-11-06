package kindyaml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Encode writes m as block YAML in field order with the keys of the kind
// schema. Unset optional fields and messages are left out and zero
// implicit-presence fields are written (protojson EmitDefaultValues, never
// EmitUnpopulated, whose nulls the strict schema rejects). It refuses a nil
// message or one that is not a kind of the loader (ErrKind) and a message
// that fails the loader checks (ErrInvalid), so Decode reads the output back
// as m.
func (l *Loader) Encode(m proto.Message) ([]byte, error) {
	if m == nil || !m.ProtoReflect().IsValid() {
		return nil, fmt.Errorf("%w: encode: no message", ErrKind)
	}
	r := m.ProtoReflect()
	if k, ok := l.kinds[string(r.Descriptor().Name())]; !ok || k.typ.Descriptor().FullName() != r.Descriptor().FullName() {
		return nil, fmt.Errorf("%w: encode: %s is not a kind", ErrKind, r.Descriptor().FullName())
	}
	if alphaKind(r.Descriptor().Name()) && !l.alpha {
		return nil, fmt.Errorf("%w: encode: %s", ErrAlpha, r.Descriptor().Name())
	}
	if err := l.check(r); err != nil {
		return nil, err
	}
	js, err := protojson.MarshalOptions{EmitDefaultValues: true}.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("kind: encode: %w", err)
	}
	// The tree is built from JSON tokens rather than by parsing the JSON as
	// YAML, which would fold a raw U+0085, U+2028 or U+2029 in a string.
	dec := json.NewDecoder(bytes.NewReader(js))
	dec.UseNumber()
	root, err := node(dec)
	if err != nil {
		return nil, fmt.Errorf("kind: encode: %w", err)
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}); err != nil {
		return nil, fmt.Errorf("kind: encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("kind: encode: %w", err)
	}
	return out.Bytes(), nil
}

// node reads one JSON value from dec as a block-style YAML node. Untagged
// scalars (numbers, booleans, null) are written plain; a string is tagged
// !!str, so the emitter quotes one that would read back as another type.
func node(dec *json.Decoder) (*yaml.Node, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := t.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.MappingNode}
		if t == '[' {
			n.Kind = yaml.SequenceNode
		}
		for dec.More() {
			if n.Kind == yaml.MappingNode {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				s, _ := k.(string)
				n.Content = append(n.Content, str(s))
			}
			c, err := node(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, c)
		}
		_, err := dec.Token() // the closing delimiter
		return n, err
	case string:
		return str(t), nil
	case json.Number:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: t.String()}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatBool(t)}, nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Value: "null"}, nil
}

// str returns a string node. The merge key << and a string with a rune
// outside YAML's printable set, a tab or a line break other than \n are
// double-quoted, where they are escaped; the emitter's own choice of style
// could write them plain or folded and they would read back as another value.
func str(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if s == "<<" || strings.ContainsFunc(s, unprintable) {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

func unprintable(r rune) bool {
	switch {
	case r == '\n', r >= 0x20 && r <= 0x7E, r >= 0x10000:
		return false
	case r == 0x2028, r == 0x2029, r == 0xFEFF:
		return true
	}
	return (r < 0xA0 || r > 0xD7FF) && (r < 0xE000 || r > 0xFFFD)
}
