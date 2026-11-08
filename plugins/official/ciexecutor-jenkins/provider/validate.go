package provider

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// The grammars of ciexecutor.proto (Formats). A request that breaks one is INVALID_ARGUMENT; an answer of Jenkins that breaks
// one is UPSTREAM_ERROR, never repaired.
var (
	repositoryRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}/[A-Za-z0-9._-]{1,100}$`)
	revisionRe   = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	pipelineRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,255}$`)
	paramKeyRe   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	// operationRe is the id this provider issues: 32 hex characters derived from the host's key, a dot, and the pipeline with
	// '/' written as ':' (the id names the job because GetOperation carries only the repository and the id).
	operationRe = regexp.MustCompile(`^([0-9a-f]{32})\.([A-Za-z0-9][A-Za-z0-9._:-]{0,94})$`)
	// crumbFieldRe and crumbRe bound what the crumb issuer may answer: both end up in a request header.
	// A crumb field name must look like Jenkins's (Jenkins-Crumb, X-...-Crumb): it names a header the client sets after its own, so
	// it must not be able to name Authorization, Cookie, Host, Content-Length, the handle header and the like.
	crumbFieldRe = regexp.MustCompile(`^(?i:[a-z][a-z0-9-]{0,40}crumb[a-z0-9-]{0,20}|crumb)$`)
	crumbRe      = regexp.MustCompile(`^[A-Za-z0-9]{1,128}$`)
)

const (
	// maxPipeline is the longest pipeline name: the operation id is at most 128 bytes and carries 33 more.
	maxPipeline = 95
	maxParams   = 32
	// reservedPrefix starts the names of the parameters the provider sets itself: a caller's parameter may not use it.
	reservedPrefix = "numinance_"
	paramRevision  = "numinance_revision"
	paramOperation = "numinance_operation_id"
)

// reservedHeader reports whether a header name belongs to the client, the proxy or HTTP framing, whatever else it looks like.
func reservedHeader(name string) bool {
	for _, r := range name {
		if r > unicode.MaxASCII {
			return true // case folding is only defined here for ASCII: the Kelvin sign and the long s fold onto k and s
		}
	}
	n := strings.ToLower(name)
	for _, p := range []string{"authorization", "proxy-", "content-", "cookie", "host", "numinance-", "x-forwarded", "transfer-", "connection", "accept", "upgrade"} {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

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

// pipeline checks the job name: no empty, '.' or '..' segment, so it cannot climb out of the repository's folder.
func pipeline(s string) error {
	if !pipelineRe.MatchString(s) || len(s) > maxPipeline {
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

// params checks the caller's parameters against the port's bounds and the parameter-name grammar.
func params(m map[string]string) error {
	if len(m) > maxParams {
		return invalid()
	}
	for k, v := range m {
		if !paramKeyRe.MatchString(k) || strings.HasPrefix(k, reservedPrefix) || len(v) > 1024 || !printable(v) {
			return invalid()
		}
	}
	return nil
}

// segment checks one path segment of an artifact path Jenkins answered.
func segment(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `\`) && printable(s)
}
