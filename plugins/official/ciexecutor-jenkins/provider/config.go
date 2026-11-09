package provider

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Config is the instance configuration of the provider: the one Jenkins server it may reach. It comes from the launch
// argument -url, which the operator's install records beside the manifest, never from a request, so a caller can not
// point the provider at another host.
type Config struct {
	// Host is an exact lowercase DNS name; an IP literal is refused.
	Host string
	// Port is the TCP port of the server: always 443 (the manifest's one network intent is the host on 443).
	Port uint32
	// Prefix is the context path of the server ("/jenkins"), or empty.
	Prefix string
}

var (
	labelRe  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	prefixRe = regexp.MustCompile(`^(/[A-Za-z0-9._~-]{1,64}){1,4}$`)
	hexLabel = regexp.MustCompile(`^0x[0-9a-f]*$`)
	numeric  = regexp.MustCompile(`^\d+$`)
	// printableASCII bounds the whole argument before it is parsed: no control character, space or non-ASCII character, so the
	// text read is the text parsed.
	printableASCII = regexp.MustCompile(`^[\x21-\x7e]+$`)
)

// internalTLD reports whether the final label names no public host: a local, private or reserved space. The grammar already
// refuses an IP literal in any spelling; this refuses names that resolve inside the machine or the site by convention. A private
// server is named by a fully qualified name of the customer's own domain. (The egress proxy decides what a name resolves to.)
func internalTLD(label string) bool {
	switch label {
	case "localhost", "local", "internal", "localdomain", "lan", "intranet", "home", "corp", "arpa", "invalid", "onion":
		return true
	}
	return false
}

// ParseURL reads the base URL of the Jenkins server: https only, a DNS name, port 443 (written or not), an optional context
// path, and no user name, query or fragment. Anything else is refused: the manifest's network permission is built from it.
func ParseURL(raw string) (Config, error) {
	// A percent sign or a backslash is refused outright: url.Parse decodes a percent-encoded host, so the name the manifest
	// declares would differ from the text the operator wrote.
	if len(raw) > 512 || !printableASCII.MatchString(raw) || strings.ContainsAny(raw, "?#%\\") {
		return Config{}, errors.New("the Jenkins URL is not a plain https URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Config{}, errors.New("the Jenkins URL is not a URL")
	}
	if u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.ForceQuery {
		return Config{}, errors.New("the Jenkins URL must be https with no user name, query or fragment")
	}
	if p := u.Port(); (p != "" && p != "443") || strings.HasSuffix(u.Host, ":") {
		return Config{}, errors.New("the Jenkins port must be 443")
	}
	host := u.Hostname()
	if err := validHost(host); err != nil {
		return Config{}, err
	}
	prefix := strings.TrimSuffix(u.Path, "/")
	if prefix != "" && (!prefixRe.MatchString(prefix) || strings.Contains(prefix+"/", "/./") || strings.Contains(prefix, "..")) {
		return Config{}, errors.New("the Jenkins context path is not a plain path")
	}
	return Config{Host: host, Port: 443, Prefix: prefix}, nil
}

// validHost checks the host of the URL: a lowercase DNS name of two or more valid labels, at most 253 bytes, that is not an
// address in any spelling and does not end in a label of a local or private space.
func validHost(host string) error {
	bad := errors.New("the Jenkins host must be a lowercase DNS name of a public domain, not an address or a local name")
	if len(host) < 4 || len(host) > 253 {
		return bad
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return bad
	}
	for _, l := range labels {
		if !labelRe.MatchString(l) {
			return bad
		}
	}
	last := labels[len(labels)-1]
	if numeric.MatchString(last) || allNumberLabels(labels) || last[0] < 'a' || last[0] > 'z' || internalTLD(last) {
		return bad
	}
	return nil
}

// allNumberLabels reports whether every label is a number inet_aton accepts (decimal or 0x hex), as in 0x7f.0x0.0x0.0x1:
// getaddrinfo resolves such a name as an address, which a DNS-name-only configuration must not allow.
func allNumberLabels(labels []string) bool {
	for _, l := range labels {
		if !numeric.MatchString(l) && !hexLabel.MatchString(l) {
			return false
		}
	}
	return true
}

// Authority is host or host:port as it appears in a request URL.
func (c Config) Authority() string {
	if c.Port == 443 {
		return c.Host
	}
	return c.Host + ":" + strconv.FormatUint(uint64(c.Port), 10)
}

// ExampleURL is the made-up server the replay build and the tests use: it never resolves.
const ExampleURL = "https://jenkins.example.test"
