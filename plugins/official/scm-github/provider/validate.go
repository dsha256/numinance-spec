package provider

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// The grammars of scm.proto (Formats). A request that breaks one is INVALID_ARGUMENT; an answer of the code host that
// breaks one is UPSTREAM_ERROR, never repaired.
var (
	repositoryRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}/[A-Za-z0-9._-]{1,100}$`)
	revisionRe   = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

const (
	maxString = 1024
	maxTitle  = 256
	maxBody   = 16384
)

func invalid() error  { return ghprovider.Fail(pluginv1.Reason_REASON_INVALID_ARGUMENT) }
func upstream() error { return ghprovider.Fail(pluginv1.Reason_REASON_UPSTREAM_ERROR) }

// repository checks a full name and returns its two segments. A '.' or '..' name segment is refused: it would climb
// out of /repos/<owner>/<name> in the request path.
func repository(s string) (owner, name string, err error) {
	if !repositoryRe.MatchString(s) {
		return "", "", invalid()
	}
	owner, name, _ = strings.Cut(s, "/")
	if owner == "." || owner == ".." || name == "." || name == ".." || strings.HasSuffix(name, ".git") {
		return "", "", invalid()
	}
	return owner, name, nil
}

func revision(s string) error {
	if !revisionRe.MatchString(s) {
		return invalid()
	}
	return nil
}

// branch checks a branch name under git check-ref-format --branch, as scm.proto states it.
func branch(s string) error {
	if s == "" || len(s) > 255 || s[0] == '-' || s[0] == '/' || strings.HasSuffix(s, "/") || strings.HasSuffix(s, ".") ||
		strings.Contains(s, "..") || strings.Contains(s, "//") || strings.Contains(s, "@{") || s == "@" {
		return invalid()
	}
	for _, seg := range strings.Split(s, "/") {
		if strings.HasPrefix(seg, ".") || strings.HasSuffix(seg, ".lock") {
			return invalid()
		}
	}
	for _, r := range s {
		if r <= ' ' || r >= 0x7f || strings.ContainsRune("~^:?*[\\", r) {
			return invalid()
		}
	}
	return nil
}

// repoPath checks a path of the path grammar; empty is allowed only when allowEmpty is set.
func repoPath(s string, allowEmpty bool) error {
	if s == "" {
		if allowEmpty {
			return nil
		}
		return invalid()
	}
	if len(s) > maxString || s[0] == '/' || strings.Contains(s, "\\") || !printable(s, false) {
		return invalid()
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return invalid()
		}
	}
	return nil
}

// printable reports whether s holds only printable characters (categories L, M, N, P, S and U+0020), and U+0009 and U+000A
// when multiline is set.
func printable(s string, multiline bool) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		switch {
		case r == ' ', multiline && (r == '\t' || r == '\n'):
		case unicode.IsLetter(r), unicode.IsMark(r), unicode.IsNumber(r), unicode.IsPunct(r), unicode.IsSymbol(r):
		default:
			return false
		}
	}
	return true
}

func text(s string, hi int, multiline bool) error {
	if s == "" || len(s) > hi || !printable(s, multiline) {
		return invalid()
	}
	return nil
}

// clean checks that s, an answer of the code host, is a printable string of at most hi bytes: it is untrusted and never repaired
// or truncated.
func clean(s string, hi int) error {
	if len(s) > hi || !printable(s, false) {
		return upstream()
	}
	return nil
}

// commit returns s when it is a full commit id.
func commit(s string) (string, error) {
	if !revisionRe.MatchString(s) {
		return "", upstream()
	}
	return s, nil
}

// httpsURL returns s when it is an absolute https URL of at most 1024 bytes without a user name or control character; an
// empty one is returned as empty.
func httpsURL(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) > maxString || !strings.HasPrefix(s, "https://") || strings.ContainsAny(s, " @\\") || !printable(s, false) {
		return "", upstream()
	}
	return s, nil
}
