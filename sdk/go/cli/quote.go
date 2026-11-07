package cli

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maxFieldRunes caps a name, path or field printed from a document.
	maxFieldRunes = 120
	// maxMessageRunes caps an error message printed from a loader or resolver.
	maxMessageRunes = 400
)

// quote returns s cut and escaped by clean, between double quotes. It is the one place a value is escaped for a
// message that is built from fixed text and values: the result is final and is not passed through clean again.
func quote(s string) string { return `"` + clean(s, maxFieldRunes) + `"` }

// safeErrorf is Errorf for a message whose untrusted values were passed through quote.
func safeErrorf(k Kind, format string, args ...any) *Error {
	e := Errorf(k, format, args...)
	e.safe = true
	return e
}

// printable is the text of err for stderr. A message built with safeErrorf is already escaped; any other error is
// cleaned as a whole.
func printable(err error) string {
	var e *Error
	if errors.As(err, &e) && e.safe {
		return e.Msg
	}
	return clean(err.Error(), maxMessageRunes)
}

// clean makes untrusted text safe to print inside double quotes and to put in
// JSON: cut to limit runes, then every control, format (bidi, zero-width) rune
// and invalid UTF-8 byte becomes a visible \u or \x escape, and a backslash or
// double quote becomes \\ or \". Below the limit the mapping is one-to-one: two
// different inputs never give the same output, so a name containing
// the six characters \u0007 cannot pass for one containing BEL. The output is
// meant to be printed between double quotes; the JSON encoder then escapes it
// once more for JSON, which keeps it one-to-one.
func clean(s string, limit int) string {
	var b strings.Builder
	n := 0
	for i := 0; i < len(s); {
		if n == limit {
			b.WriteString("...(cut)")
			break
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\\' || r == '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029':
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
		i += size
		n++
	}
	return b.String()
}
