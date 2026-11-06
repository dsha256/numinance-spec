package evidenceverify

import (
	"errors"
	"strconv"
	"unicode/utf8"
)

// The documents of the format are canonical JSON. The parser here is
// deliberately narrow: it accepts no white space (a canonical document has
// none), refuses duplicate keys, refuses invalid UTF-8 and lone surrogates, and
// keeps numbers as their text so that a later byte comparison with the
// canonical form catches every non-canonical spelling.

const maxDepth = 64

type nodeKind int

const (
	nObject nodeKind = iota + 1
	nArray
	nString
	nNumber
	nBool
	nNull
)

type node struct {
	kind    nodeKind
	str     string // string value, or number text
	b       bool
	keys    []string
	members map[string]*node
	items   []*node
}

var errSyntax = errors.New("not canonical JSON")

type parser struct {
	data []byte
	pos  int
}

// parseJSON parses data as exactly one JSON value with nothing after it.
func parseJSON(data []byte) (*node, error) {
	p := &parser{data: data}
	n, err := p.value(0)
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.data) {
		return nil, errSyntax
	}
	return n, nil
}

func (p *parser) value(depth int) (*node, error) {
	if depth > maxDepth || p.pos >= len(p.data) {
		return nil, errSyntax
	}
	switch c := p.data[p.pos]; {
	case c == '{':
		return p.object(depth)
	case c == '[':
		return p.array(depth)
	case c == '"':
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		return &node{kind: nString, str: s}, nil
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	}
	for _, lit := range []struct {
		text string
		n    node
	}{{"true", node{kind: nBool, b: true}}, {"false", node{kind: nBool}}, {"null", node{kind: nNull}}} {
		if len(p.data)-p.pos >= len(lit.text) && string(p.data[p.pos:p.pos+len(lit.text)]) == lit.text {
			p.pos += len(lit.text)
			n := lit.n
			return &n, nil
		}
	}
	return nil, errSyntax
}

func (p *parser) object(depth int) (*node, error) {
	p.pos++ // {
	n := &node{kind: nObject, members: map[string]*node{}}
	if p.pos < len(p.data) && p.data[p.pos] == '}' {
		p.pos++
		return n, nil
	}
	for {
		if p.pos >= len(p.data) || p.data[p.pos] != '"' {
			return nil, errSyntax
		}
		k, err := p.str()
		if err != nil {
			return nil, err
		}
		if _, dup := n.members[k]; dup {
			return nil, errors.New("duplicate key")
		}
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			return nil, errSyntax
		}
		p.pos++
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		n.keys = append(n.keys, k)
		n.members[k] = v
		if p.pos >= len(p.data) {
			return nil, errSyntax
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return n, nil
		default:
			return nil, errSyntax
		}
	}
}

func (p *parser) array(depth int) (*node, error) {
	p.pos++ // [
	n := &node{kind: nArray}
	if p.pos < len(p.data) && p.data[p.pos] == ']' {
		p.pos++
		return n, nil
	}
	for {
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		n.items = append(n.items, v)
		if p.pos >= len(p.data) {
			return nil, errSyntax
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return n, nil
		default:
			return nil, errSyntax
		}
	}
}

func (p *parser) number() (*node, error) {
	start := p.pos
	if p.data[p.pos] == '-' {
		p.pos++
	}
	digits := func() int {
		s := p.pos
		for p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
			p.pos++
		}
		return p.pos - s
	}
	if digits() == 0 {
		return nil, errSyntax
	}
	if p.pos < len(p.data) && p.data[p.pos] == '.' {
		p.pos++
		if digits() == 0 {
			return nil, errSyntax
		}
	}
	if p.pos < len(p.data) && (p.data[p.pos] == 'e' || p.data[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.data) && (p.data[p.pos] == '+' || p.data[p.pos] == '-') {
			p.pos++
		}
		if digits() == 0 {
			return nil, errSyntax
		}
	}
	return &node{kind: nNumber, str: string(p.data[start:p.pos])}, nil
}

func hexVal(c byte) rune {
	switch {
	case c >= '0' && c <= '9':
		return rune(c - '0')
	case c >= 'a' && c <= 'f':
		return rune(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return rune(c-'A') + 10
	}
	return -1
}

func (p *parser) hex4() (rune, bool) {
	if len(p.data)-p.pos < 4 {
		return 0, false
	}
	var r rune
	for i := 0; i < 4; i++ {
		h := hexVal(p.data[p.pos+i])
		if h < 0 {
			return 0, false
		}
		r = r<<4 | h
	}
	p.pos += 4
	return r, true
}

func (p *parser) str() (string, error) {
	p.pos++ // opening quote
	var out []byte
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		switch {
		case c == '"':
			p.pos++
			return string(out), nil
		case c < 0x20:
			return "", errSyntax
		case c == '\\':
			p.pos++
			if p.pos >= len(p.data) {
				return "", errSyntax
			}
			e := p.data[p.pos]
			p.pos++
			switch e {
			case '"', '\\', '/':
				out = append(out, e)
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'u':
				r, ok := p.hex4()
				if !ok {
					return "", errSyntax
				}
				if r >= 0xD800 && r < 0xDC00 {
					if len(p.data)-p.pos < 2 || p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
						return "", errSyntax
					}
					p.pos += 2
					lo, ok := p.hex4()
					if !ok || lo < 0xDC00 || lo >= 0xE000 {
						return "", errSyntax
					}
					r = 0x10000 + (r-0xD800)<<10 + (lo - 0xDC00)
				} else if r >= 0xDC00 && r < 0xE000 {
					return "", errSyntax
				}
				out = utf8.AppendRune(out, r)
			default:
				return "", errSyntax
			}
		case c < utf8.RuneSelf:
			out = append(out, c)
			p.pos++
		default:
			r, size := utf8.DecodeRune(p.data[p.pos:])
			if r == utf8.RuneError && size == 1 {
				return "", errors.New("invalid UTF-8")
			}
			out = append(out, p.data[p.pos:p.pos+size]...)
			p.pos += size
		}
	}
	return "", errSyntax
}

// uintText parses the canonical text of an unsigned integer: "0" or digits
// without a leading zero, no sign, no fraction, no exponent.
func uintText(s string) (uint64, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	v, err := strconv.ParseUint(s, 10, 64)
	return v, err == nil
}
