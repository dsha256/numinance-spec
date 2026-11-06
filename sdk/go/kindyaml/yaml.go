package kindyaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	// MaxBytes bounds a kind document. The validator's work is linear in the
	// value times up to schema.MaxVisits, so this cap is what bounds Decode.
	MaxBytes = 128 << 10
	// MaxDepth bounds the nesting of a kind document; the root mapping is level 1.
	MaxDepth = 64
	// MaxNodes bounds the values of a kind document: every mapping, sequence
	// and scalar other than a mapping key, the root mapping included.
	MaxNodes = 16384
)

var (
	// ErrTooLarge reports a document over MaxBytes.
	ErrTooLarge = errors.New("kind: document larger than 128 KiB")
	// ErrSyntax reports input that is not one YAML document.
	ErrSyntax = errors.New("kind: not a YAML document")
	// ErrUnsupported reports YAML a kind document may not use: anchors,
	// aliases, merge keys, tags, more than one document, a non-string key, or
	// a scalar with no JSON form.
	ErrUnsupported = errors.New("kind: unsupported YAML")
	// ErrDuplicateKey reports a key that appears twice in one mapping.
	ErrDuplicateKey = errors.New("kind: duplicate key")
	// ErrDepth reports a document nested deeper than MaxDepth.
	ErrDepth = errors.New("kind: document nested too deep")
	// ErrTooManyNodes reports a document with more than MaxNodes values.
	ErrTooManyNodes = errors.New("kind: document has more than 16384 values")
)

// parse reads b as one YAML document whose root is a mapping and returns it
// as the JSON value encoding/json with UseNumber would produce. Errors carry
// line numbers, never document text. The parser itself refuses text that is
// not UTF-8 (ErrSyntax).
func parse(b []byte) (map[string]any, error) {
	if len(b) > MaxBytes {
		return nil, ErrTooLarge
	}
	// Decoding into a yaml.Node never expands aliases, so an alias bomb costs
	// nothing before convert refuses the alias.
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: empty input", ErrSyntax)
		}
		return nil, syntaxError(err)
	}
	var next yaml.Node
	switch err := dec.Decode(&next); {
	case err == nil:
		return nil, fmt.Errorf("%w: more than one document", ErrUnsupported)
	case !errors.Is(err, io.EOF):
		return nil, syntaxError(err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: the document is not a mapping", ErrUnsupported)
	}
	budget := MaxNodes
	v, err := convert(doc.Content[0], 1, &budget)
	if err != nil {
		return nil, err
	}
	return v.(map[string]any), nil
}

// syntaxError keeps only the line of a parser error: some go-yaml messages
// quote the document, an undefined alias name among them.
func syntaxError(err error) error {
	line, ok := strings.CutPrefix(err.Error(), "yaml: line ")
	if n := strings.IndexFunc(line, func(r rune) bool { return r < '0' || r > '9' }); ok && n > 0 {
		return fmt.Errorf("%w: line %s", ErrSyntax, line[:n])
	}
	return ErrSyntax
}

// convert returns the JSON value of n, which sits at nesting level depth;
// each value it converts spends one of budget.
func convert(n *yaml.Node, depth int, budget *int) (any, error) {
	*budget--
	if *budget < 0 {
		return nil, fmt.Errorf("%w: line %d", ErrTooManyNodes, n.Line)
	}
	if depth > MaxDepth {
		return nil, fmt.Errorf("%w: line %d is nested more than %d levels", ErrDepth, n.Line, MaxDepth)
	}
	if err := plain(n); err != nil {
		return nil, err
	}
	switch n.Kind {
	case yaml.MappingNode:
		obj := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, err := key(n.Content[i])
			if err != nil {
				return nil, err
			}
			if _, dup := obj[k]; dup {
				return nil, fmt.Errorf("%w: line %d", ErrDuplicateKey, n.Content[i].Line)
			}
			if obj[k], err = convert(n.Content[i+1], depth+1, budget); err != nil {
				return nil, err
			}
		}
		return obj, nil
	case yaml.SequenceNode:
		arr := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := convert(c, depth+1, budget)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	case yaml.ScalarNode:
		return scalar(n)
	case yaml.AliasNode:
		return nil, fmt.Errorf("%w: line %d: alias", ErrUnsupported, n.Line)
	}
	return nil, fmt.Errorf("%w: line %d: node kind %d", ErrUnsupported, n.Line, n.Kind)
}

// plain refuses an anchor or an explicit tag on n. The non-specific tag !
// leaves no trace on the node, so it cannot be refused and is ignored.
func plain(n *yaml.Node) error {
	if n.Anchor != "" {
		return fmt.Errorf("%w: line %d: anchor", ErrUnsupported, n.Line)
	}
	if n.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf("%w: line %d: tag", ErrUnsupported, n.Line)
	}
	return nil
}

// key returns the string a mapping key node holds; any other key, the merge
// key << among them, is refused.
func key(n *yaml.Node) (string, error) {
	if err := plain(n); err != nil {
		return "", err
	}
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!str" {
		return "", fmt.Errorf("%w: line %d: key is not a string", ErrUnsupported, n.Line)
	}
	return n.Value, nil
}

// scalar returns the JSON value of a scalar node. A YAML 1.1 timestamp stays
// the string it was written as; numbers must already be JSON numbers.
func scalar(n *yaml.Node) (any, error) {
	switch n.ShortTag() {
	case "!!str", "!!timestamp":
		return n.Value, nil
	case "!!null":
		return nil, nil //nolint:nilnil // a YAML null is the JSON null
	case "!!bool":
		switch n.Value {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return nil, fmt.Errorf("%w: line %d: boolean other than true or false", ErrUnsupported, n.Line)
	case "!!int", "!!float":
		if isJSONNumber(n.Value) {
			return json.Number(n.Value), nil
		}
		return nil, fmt.Errorf("%w: line %d: number is not in JSON form", ErrUnsupported, n.Line)
	}
	return nil, fmt.Errorf("%w: line %d: scalar type", ErrUnsupported, n.Line)
}

// isJSONNumber reports whether s is exactly one RFC 8259 number, so 0x10,
// 1_000, +1, .5 and .inf are not.
func isJSONNumber(s string) bool {
	if s == "" || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) {
		return false
	}
	last := s[len(s)-1]
	return last >= '0' && last <= '9' && json.Valid([]byte(s))
}
