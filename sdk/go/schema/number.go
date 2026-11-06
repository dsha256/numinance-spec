package schema

import (
	"cmp"
	"encoding/json"
	"strconv"
	"strings"
)

// maxExp saturates exponents; Compile keeps bounds within maxExp/2, so saturation never misorders.
const maxExp = 1 << 40

// decimal is a JSON number read in place as ±0.D × 10^point, where the digits
// D are intp then frac with no leading or trailing zeros; zero has no digits
// and is never negative. Comparing two never allocates, whatever their length.
type decimal struct {
	neg        bool
	intp, frac string
	point      int64
}

// numberOf returns the number v holds; ok is false for an invalid json.Number, NaN or an infinity.
func numberOf(v any) (d decimal, ok bool) {
	switch n := v.(type) {
	case json.Number:
		return parseNumber(string(n))
	case float64:
		return parseNumber(strconv.FormatFloat(n, 'g', -1, 64)) // the shortest form that round-trips
	}
	return decimal{}, false
}

// parseNumber reads the number grammar of RFC 8259 section 6.
func parseNumber(s string) (d decimal, ok bool) {
	s, d.neg = strings.CutPrefix(s, "-")
	i := digitsEnd(s)
	if i == 0 || (s[0] == '0' && i > 1) {
		return decimal{}, false
	}
	d.intp, s = s[:i], s[i:]
	if rest, dot := strings.CutPrefix(s, "."); dot {
		j := digitsEnd(rest)
		if j == 0 {
			return decimal{}, false
		}
		d.frac, s = rest[:j], rest[j:]
	}
	var exp int64
	if s != "" {
		if s[0] != 'e' && s[0] != 'E' {
			return decimal{}, false
		}
		s = s[1:]
		if t := strings.TrimLeft(s, "+-"); len(s)-len(t) > 1 || t == "" || digitsEnd(t) != len(t) {
			return decimal{}, false
		}
		exp, _ = strconv.ParseInt(s, 10, 64) // saturates on overflow
		exp = max(-maxExp, min(maxExp, exp))
	}
	d.intp = strings.TrimLeft(d.intp, "0")
	d.point = int64(len(d.intp)) + exp
	if d.intp == "" {
		t := strings.TrimLeft(d.frac, "0")
		d.point -= int64(len(d.frac) - len(t))
		d.frac = t
	}
	if d.frac = strings.TrimRight(d.frac, "0"); d.frac == "" {
		d.intp = strings.TrimRight(d.intp, "0")
	}
	d.neg = d.neg && d.digits() > 0 // -0 is 0
	return d, true
}

func digitsEnd(s string) int { return len(s) - len(strings.TrimLeft(s, "0123456789")) }

func (d decimal) digits() int { return len(d.intp) + len(d.frac) }

func (d decimal) digit(i int) byte {
	if i < len(d.intp) {
		return d.intp[i]
	}
	return d.frac[i-len(d.intp)]
}

func (d decimal) isInteger() bool { return d.digits() == 0 || d.point >= int64(d.digits()) }

// compareDecimal returns -1, 0 or +1 as a is less than, equal to or greater than b.
func compareDecimal(a, b decimal) int {
	switch {
	case a.neg && b.neg:
		a, b = b, a // compare magnitudes
	case a.neg:
		return -1
	case b.neg:
		return 1
	}
	if a.digits() == 0 || b.digits() == 0 {
		return cmp.Compare(a.digits(), b.digits()) // zero is the smallest magnitude
	}
	if c := cmp.Compare(a.point, b.point); c != 0 {
		return c
	}
	for i := range min(a.digits(), b.digits()) {
		if c := cmp.Compare(a.digit(i), b.digit(i)); c != 0 {
			return c
		}
	}
	return cmp.Compare(a.digits(), b.digits())
}
