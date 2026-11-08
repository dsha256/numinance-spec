package ticketingkit

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

// messageRe is the closed vocabulary of an error message: ASCII letters, digits, a space and a few separators. A quote, a
// backslash, a percent sign, a plus sign, an equals sign, an ampersand, an at sign and every non-ASCII or control character
// are outside it, so a message that quotes (%q), escapes (URL, JSON, HTML) or carries a character of the input cannot pass.
var messageRe = regexp.MustCompile(`^[A-Za-z0-9 ,.:;()\[\]{}/_^$*|?<>-]{1,200}$`)

// ValidateErrorMessage checks the message of one failed call of any class (INVALID_ARGUMENT, NOT_FOUND, CONFLICT,
// RATE_LIMITED, UPSTREAM_UNAVAILABLE, UPSTREAM_ERROR): it is a fixed message of the closed vocabulary and holds none of the
// secrets (the project, the key, a title or any other tracker text, compared as it is and, for the longer ones, case-folded).
// A provider's own test applies it to the classes the kit cannot provoke (a rate limit, an outage).
func ValidateErrorMessage(f *CallError, secrets ...string) error {
	if !messageRe.MatchString(f.Message) {
		return fmt.Errorf("the %s message %q is not a fixed message of the closed vocabulary (letters, digits, a space and . , : ; ( ) [ ] { } / _ ^ $ * | ? < > -, at most 200 bytes): it may quote, escape or carry a character of the input or of the tracker", f.Reason, f.Message)
	}
	folded := strings.ToLower(f.Message)
	for _, sec := range secrets {
		if sec == "" {
			continue
		}
		if strings.Contains(f.Message, sec) || (len(sec) >= 8 && strings.Contains(folded, strings.ToLower(sec))) {
			return fmt.Errorf("the %s message %q holds %q: it must hold neither the key, the project nor any tracker text", f.Reason, f.Message, sec)
		}
	}
	return nil
}

// callError returns the CallError inside err, or nil.
func callError(err error) *CallError {
	var f *CallError
	if errors.As(err, &f) {
		return f
	}
	return nil
}

// failedWith checks that err is a FAILED call with the reason and a fixed message, and returns the message.
func failedWith(err error, reason pluginv1.Reason, secrets ...string) (string, error) {
	if e := wantReason(err, reason); e != nil {
		return "", e
	}
	f := callError(err)
	if e := ValidateErrorMessage(f, secrets...); e != nil {
		return "", e
	}
	return f.Message, nil
}

// notFound checks that err is a NOT_FOUND with a fixed message that holds none of the secrets, and returns the message.
func notFound(err error, secrets ...string) (string, error) {
	return failedWith(err, pluginv1.Reason_REASON_NOT_FOUND, secrets...)
}

// conflict checks that err is a CONFLICT with a fixed message that holds none of the secrets, and returns the message.
func conflict(err error, secrets ...string) (string, error) {
	return failedWith(err, pluginv1.Reason_REASON_CONFLICT, secrets...)
}

// echoForms are the spellings in which a provider could repeat a value in a message: as it is, case-folded, quoted, escaped
// for a URL, a path or JSON, in base64 and in hex.
func echoForms(v string) []string {
	forms := []string{v, strings.ToLower(v), strconv.Quote(v), url.QueryEscape(v), url.PathEscape(v), hex.EncodeToString([]byte(v))}
	if q := strconv.Quote(v); len(q) >= 2 {
		forms = append(forms, q[1:len(q)-1])
	}
	if j, err := json.Marshal(v); err == nil && len(j) >= 2 {
		forms = append(forms, string(j[1:len(j)-1]))
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		forms = append(forms, enc.EncodeToString([]byte(v)))
	}
	return forms
}

// echoes reports whether msg repeats v: in any of its spellings, or in part (any run of four or more of its characters,
// case-folded).
func echoes(msg, v string) (string, bool) {
	if len(v) < 2 {
		return "", false
	}
	folded := strings.ToLower(msg)
	for _, f := range echoForms(v) {
		if len(f) >= 2 && (strings.Contains(msg, f) || strings.Contains(folded, strings.ToLower(f))) {
			return f, true
		}
	}
	runes := []rune(strings.ToLower(v))
	for i := 0; i+4 <= len(runes); i++ {
		if w := string(runes[i : i+4]); strings.Contains(folded, w) {
			return w, true
		}
	}
	return "", false
}

// twin is v with every letter and digit replaced by the next one of its kind (ASCII and fullwidth, z to a, 9 to 0) and
// every other character kept: it breaks the same bound as v, so a fixed message is the same for both and an echo is not.
func twin(v string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+1)%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+1)%26
		case r >= '0' && r <= '9':
			return '0' + (r-'0'+1)%10
		case r >= 0xFF41 && r <= 0xFF5A:
			return 0xFF41 + (r-0xFF41+1)%26
		case r >= 0xFF21 && r <= 0xFF3A:
			return 0xFF21 + (r-0xFF21+1)%26
		case r >= 0xFF10 && r <= 0xFF19:
			return 0xFF10 + (r-0xFF10+1)%10
		}
		return r
	}, v)
}

// wantInvalid checks that err is INVALID_ARGUMENT with a fixed message of the closed vocabulary that repeats neither the
// offending value (in any spelling, whole or in part) nor any of the secrets.
func wantInvalid(err error, offending string, secrets ...string) error {
	msg, e := failedWith(err, pluginv1.Reason_REASON_INVALID_ARGUMENT, secrets...)
	if e != nil {
		return e
	}
	if form, ok := echoes(msg, offending); ok {
		return fmt.Errorf("the rejection %q repeats the offending value (as %q): a rejection names the field and the bound, never the value", msg, form)
	}
	return nil
}

// wantInvalidTwin is wantInvalid for a call that is made twice, with the offending value and with its twin: the two
// rejections must be the same message, because a message that depends on the value repeats some of it.
func wantInvalidTwin(call func(string) error, offending string, secrets ...string) error {
	err := call(offending)
	if e := wantInvalid(err, offending, secrets...); e != nil {
		return e
	}
	other := twin(offending)
	if other == offending {
		return nil
	}
	errTwin := call(other)
	if e := wantInvalid(errTwin, other, secrets...); e != nil {
		return fmt.Errorf("the twin %q: %w", other, e)
	}
	if a, b := callError(err).Message, callError(errTwin).Message; a != b {
		return fmt.Errorf("the rejections %q and %q (for %q and its twin %q, which break the same bound) differ: a rejection is one fixed message and does not depend on the value", a, b, offending, other)
	}
	return nil
}
