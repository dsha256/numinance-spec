package schema

import (
	"regexp"
	"strings"
)

var quantifier = regexp.MustCompile(`^\{\d+(?:,\d*)?\}`)

// patternSubset names the first construct of p outside the subset where RE2
// and ECMA-262 (u flag) match the same strings, or returns "".
func patternSubset(p string) string {
	inClass := false
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c < 0x20 || c > 0x7e:
			return "a control or non-ASCII byte"
		case c == '\\':
			i++
			allowed := `dDwWtnrfv\.^$*+?()[]{}|/bB`
			if inClass {
				allowed = `dDwWtnrfv\.^$*+?()[]{}|/-`
			}
			if i == len(p) || strings.IndexByte(allowed, p[i]) < 0 {
				return `an escape other than \d \w \b \t \n \r \f \v or punctuation`
			}
		case inClass && c == '[':
			return "[ inside a class"
		case inClass && c == ']':
			inClass = false
		case inClass:
		case c == '[':
			if inClass = true; strings.HasPrefix(p[i+1:], "]") || strings.HasPrefix(p[i+1:], "^]") {
				return "a class starting with ]"
			}
		case c == '{':
			n := len(quantifier.FindString(p[i:]))
			if n == 0 {
				return "a { that does not start {n}, {n,} or {n,m}"
			}
			i += n - 1
		case c == '}' || c == ']':
			return "an unescaped } or ] (ECMA-262 rejects it)"
		case c == '.':
			return "'.' (its line terminators differ)"
		case c == '(' && strings.HasPrefix(p[i+1:], "?") && !strings.HasPrefix(p[i+1:], "?:"):
			return "a group flag or assertion"
		}
	}
	return ""
}
