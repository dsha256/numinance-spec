package provider

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// The grammars of ciexecutor.proto (Formats). A request that breaks one is INVALID_ARGUMENT; an answer of the code host
// that breaks one is UPSTREAM_ERROR, never repaired.
var (
	repositoryRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}/[A-Za-z0-9._-]{1,100}$`)
	revisionRe   = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	pipelineRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,255}$`)
	operationRe  = regexp.MustCompile(`^\d{1,19}$`)
	paramKeyRe   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// reservedPrefix starts the names of the inputs the provider sets itself: a caller's parameter may not use it.
const reservedPrefix = "numinance_"

func invalid() error  { return ghprovider.Fail(pluginv1.Reason_REASON_INVALID_ARGUMENT) }
func upstream() error { return ghprovider.Fail(pluginv1.Reason_REASON_UPSTREAM_ERROR) }

func repository(s string) (owner, name string, err error) {
	if !repositoryRe.MatchString(s) {
		return "", "", invalid()
	}
	owner, name, _ = strings.Cut(s, "/")
	if owner == "." || owner == ".." || name == "." || name == ".." {
		return "", "", invalid()
	}
	return owner, name, nil
}

// pipeline checks the workflow file name: no '..' segment, so it cannot climb out of /actions/workflows/.
func pipeline(s string) error {
	if !pipelineRe.MatchString(s) {
		return invalid()
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return invalid()
		}
	}
	return nil
}

func printable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r != ' ' && !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsNumber(r) && !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			return false
		}
	}
	return true
}

// httpsURL returns s when it is an absolute https URL of at most 1024 bytes without a user name or control character.
func httpsURL(s string) (string, error) {
	if len(s) > 1024 || !strings.HasPrefix(s, "https://") || strings.ContainsAny(s, " @\\") || !printable(s) {
		return "", upstream()
	}
	return s, nil
}
