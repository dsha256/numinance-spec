package evidenceverify

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// rawNum is a number kept as its canonical text.
type rawNum string

const hexDigits = "0123456789abcdef"

// canonical returns the canonical JSON bytes of v. v is built from
// map[string]any, []any, string, bool, uint64, int and rawNum.
func canonical(v any) []byte { return appendCanon(nil, v) }

func appendCanon(b []byte, v any) []byte {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys) // byte order of the UTF-8 keys
		b = append(b, '{')
		for i, k := range keys {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendString(b, k)
			b = append(b, ':')
			b = appendCanon(b, x[k])
		}
		return append(b, '}')
	case []any:
		b = append(b, '[')
		for i, e := range x {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendCanon(b, e)
		}
		return append(b, ']')
	case string:
		return appendString(b, x)
	case bool:
		return strconv.AppendBool(b, x)
	case uint64:
		return strconv.AppendUint(b, x, 10)
	case int:
		return strconv.AppendInt(b, int64(x), 10)
	case rawNum:
		return append(b, x...)
	}
	return append(b, "null"...)
}

// appendString writes s as Go's encoding/json writes it with HTML escaping off.
func appendString(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '"' || c == '\\':
				b = append(b, '\\', c)
			case c == '\n':
				b = append(b, '\\', 'n')
			case c == '\r':
				b = append(b, '\\', 'r')
			case c == '\t':
				b = append(b, '\\', 't')
			case c == '\b':
				b = append(b, '\\', 'b')
			case c == '\f':
				b = append(b, '\\', 'f')
			case c < 0x20:
				b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
			default:
				b = append(b, c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b = append(b, `�`...)
		case r == ' ' || r == ' ':
			b = append(b, `\u202`...)
			b = append(b, hexDigits[r&0xf])
		default:
			b = append(b, s[i:i+size]...)
		}
		i += size
	}
	return append(b, '"')
}

// toAny converts a decoded node to the value form that canonical writes.
func (n *node) toAny() any {
	switch n.kind {
	case nObject:
		m := make(map[string]any, len(n.members))
		for k, v := range n.members {
			m[k] = v.toAny()
		}
		return m
	case nArray:
		a := make([]any, len(n.items))
		for i, v := range n.items {
			a[i] = v.toAny()
		}
		return a
	case nString:
		return n.str
	case nNumber:
		return rawNum(n.str)
	case nBool:
		return n.b
	}
	return nil
}

// canonicalWithout is the canonical form of an object node with the named
// keys removed.
func (n *node) canonicalWithout(drop ...string) []byte {
	m := n.toAny().(map[string]any)
	for _, k := range drop {
		delete(m, k)
	}
	return canonical(m)
}

func sha256Digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func digestBytes(b []byte) [32]byte { return sha256.Sum256(b) }

const zeroDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func isDigest(s string) bool {
	return len(s) == 71 && strings.HasPrefix(s, "sha256:") && isLowerHex(s[7:])
}

// rawDigest returns the 32 bytes of a digest text.
func rawDigest(s string) ([]byte, bool) {
	if !isDigest(s) {
		return nil, false
	}
	b, err := hex.DecodeString(s[7:])
	return b, err == nil
}

// hasID reports whether s is prefix followed by 32 lowercase hexadecimal characters.
func hasID(s, prefix string) bool {
	return len(s) == len(prefix)+32 && strings.HasPrefix(s, prefix) && isLowerHex(s[len(prefix):])
}

func isChangeID(s string) bool   { return hasID(s, "chg_") }
func isEvID(s string) bool       { return hasID(s, "ev_") }
func isRunID(s string) bool      { return hasID(s, "run_") }
func isDecisionID(s string) bool { return hasID(s, "dec_") }
func isGrantID(s string) bool    { return hasID(s, "grant_") }

// isBase64Len reports whether s is standard padded base64 of exactly n bytes.
func isBase64Len(s string, n int) bool {
	b, err := base64.StdEncoding.Strict().DecodeString(s)
	return err == nil && len(b) == n
}

func decodeBase64(s string) []byte {
	b, _ := base64.StdEncoding.Strict().DecodeString(s)
	return b
}

func isTimestamp(s string) bool {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || t.Year() < 1 || t.Year() > 9999 {
		return false
	}
	return t.UTC().Format(time.RFC3339Nano) == s
}

// isPrintableText is the rule for executor_id, model_id and a log origin: at
// most limit bytes of valid UTF-8, every character printable by Go's
// unicode.IsPrint, no leading or trailing ASCII space.
func isPrintableText(s string, limit int) bool {
	if len(s) > limit || !utf8.ValidString(s) {
		return false
	}
	if s != "" && (s[0] == ' ' || s[len(s)-1] == ' ') {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// isASCIIText is 1 to 256 bytes of printable ASCII (0x20 to 0x7e) without a
// leading or trailing space.
func isASCIIText(s string) bool {
	if len(s) < 1 || len(s) > 256 || s[0] == ' ' || s[len(s)-1] == ' ' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func isRef(s string) bool {
	if len(s) < 1 || len(s) > 1024 {
		return false
	}
	c := s[0]
	if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '.' || c == '_' || c == '/' || c == '-'
		if !ok {
			return false
		}
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// isTicket is the ticket leaf: "none", or a reference of 1 to 128 printable
// ASCII characters without space or '#', then '#' and a digest.
func isTicket(s string) bool {
	if s == "none" {
		return true
	}
	ref, dig, ok := strings.Cut(s, "#")
	if !ok || len(ref) < 1 || len(ref) > 128 || len(s) > 256 || !isDigest(dig) {
		return false
	}
	for i := 0; i < len(ref); i++ {
		if ref[i] < 0x21 || ref[i] > 0x7e || ref[i] == '#' {
			return false
		}
	}
	return true
}
