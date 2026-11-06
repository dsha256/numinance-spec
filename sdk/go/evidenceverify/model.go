package evidenceverify

import (
	"bytes"
	"errors"
	"fmt"
)

// A document of the format is read in three steps (canonical-form.md
// section 3): parse strictly, check the value against the document's field
// rules, then require that the canonical form of the value equals the input
// byte for byte. The last step refuses every non-canonical spelling at once.

type vtype int

const (
	tStr vtype = iota + 1
	tUint
	tBool
	tObj
	tArr
)

const maxSafe = 1<<53 - 1

type field struct {
	name     string
	t        vtype
	req      bool
	ok       func(string) bool // tStr: the value rule
	nz       bool              // tStr: a zero digest is an empty field
	min, max uint64            // tUint
	sub      []field           // tObj
	elem     *field            // tArr
	maxItems int               // tArr
}

func str(name string, req bool, ok func(string) bool) field {
	return field{name: name, t: tStr, req: req, ok: ok}
}

func digest(name string, req bool) field {
	return field{name: name, t: tStr, req: req, ok: isDigest, nz: true}
}

func uintf(name string, req bool, lo, hi uint64) field {
	return field{name: name, t: tUint, req: req, min: lo, max: hi}
}

func boolf(name string, req bool) field { return field{name: name, t: tBool, req: req} }

func objf(name string, req bool, sub []field) field {
	return field{name: name, t: tObj, req: req, sub: sub}
}

func arrf(name string, maxItems int, elem field) field {
	elem.req = true
	return field{name: name, t: tArr, maxItems: maxItems, elem: &elem}
}

func oneOf(vals ...string) func(string) bool {
	return func(s string) bool {
		for _, v := range vals {
			if s == v {
				return true
			}
		}
		return false
	}
}

// sigf is the signature field: 64 bytes of standard padded base64.
func sigf(req bool) field {
	return str("signature", req, func(s string) bool { return isBase64Len(s, 64) })
}

var errNotCanonical = errors.New("not the canonical form")

// decodeDoc decodes data as a document with the given root fields. A document
// over limit bytes is refused before it is parsed.
func decodeDoc(data []byte, limit int, root []field) (*node, error) {
	if len(data) > limit {
		return nil, errors.New("document over its size bound")
	}
	n, err := parseJSON(data)
	if err != nil {
		return nil, err
	}
	if n.kind != nObject {
		return nil, errors.New("not an object")
	}
	if err := checkObject(n, root); err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical(n.toAny()), data) {
		return nil, errNotCanonical
	}
	return n, nil
}

func checkObject(n *node, fields []field) error {
	byName := make(map[string]*field, len(fields))
	for i := range fields {
		byName[fields[i].name] = &fields[i]
	}
	for _, k := range n.keys {
		if byName[k] == nil {
			return errors.New("unknown field") // never echo the key
		}
	}
	for i := range fields {
		f := &fields[i]
		v, present := n.members[f.name]
		if !present {
			if f.req {
				return fmt.Errorf("missing field %s", f.name)
			}
			continue
		}
		if err := checkValue(v, f); err != nil {
			return fmt.Errorf("field %s: %w", f.name, err)
		}
	}
	return nil
}

var errEmptyField = errors.New("an empty field is written")

func checkValue(v *node, f *field) error {
	switch f.t {
	case tStr:
		if v.kind != nString {
			return errors.New("not a string")
		}
		if !f.req && (v.str == "" || (f.nz && v.str == zeroDigest)) {
			return errEmptyField
		}
		if f.nz && v.str == zeroDigest {
			return errors.New("zero digest")
		}
		if f.ok != nil && !f.ok(v.str) {
			return errors.New("invalid value")
		}
	case tUint:
		if v.kind != nNumber {
			return errors.New("not a number")
		}
		u, ok := uintText(v.str)
		if !ok {
			return errors.New("not an unsigned integer")
		}
		hi := f.max
		if hi == 0 {
			hi = maxSafe
		}
		if u < f.min || u > hi {
			return errors.New("out of range")
		}
		if !f.req && u == 0 {
			return errEmptyField
		}
	case tBool:
		if v.kind != nBool {
			return errors.New("not a boolean")
		}
		if !f.req && !v.b {
			return errEmptyField
		}
	case tObj:
		if v.kind != nObject {
			return errors.New("not an object")
		}
		if !f.req && len(v.members) == 0 {
			return errEmptyField
		}
		return checkObject(v, f.sub)
	case tArr:
		if v.kind != nArray {
			return errors.New("not an array")
		}
		if !f.req && len(v.items) == 0 {
			return errEmptyField
		}
		if len(v.items) > f.maxItems {
			return errors.New("array over its bound")
		}
		for _, it := range v.items {
			if err := checkValue(it, f.elem); err != nil {
				return err
			}
		}
	}
	return nil
}

// Typed reads of a node that decodeDoc has validated.

func (n *node) s(key string) string {
	if m := n.members[key]; m != nil {
		return m.str
	}
	return ""
}

func (n *node) u(key string) uint64 {
	if m := n.members[key]; m != nil {
		v, _ := uintText(m.str)
		return v
	}
	return 0
}

func (n *node) flag(key string) bool {
	m := n.members[key]
	return m != nil && m.b
}

func (n *node) has(key string) bool { return n.members[key] != nil }

func (n *node) obj(key string) *node { return n.members[key] }

func (n *node) list(key string) []*node {
	if m := n.members[key]; m != nil {
		return m.items
	}
	return nil
}

func (n *node) strs(key string) []string {
	items := n.list(key)
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.str)
	}
	return out
}
