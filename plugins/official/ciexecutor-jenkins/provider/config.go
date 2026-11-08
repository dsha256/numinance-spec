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
	// Port is the TCP port of the server, 443 by default. The egress proxy re-originates every port but 80 over TLS.
	Port uint32
	// Prefix is the context path of the server ("/jenkins"), or empty.
	Prefix string
}

var (
	hostRe   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	prefixRe = regexp.MustCompile(`^(/[A-Za-z0-9._~-]{1,64}){1,4}$`)
	hexLabel = regexp.MustCompile(`^0x[0-9a-f]*$`)
	numeric  = regexp.MustCompile(`^\d+$`)
)

// ParseURL reads the base URL of the Jenkins server: https only, a DNS name, an optional port and context path, and no user
// name, query or fragment. Anything else is refused: the manifest's network permission is built from it.
func ParseURL(raw string) (Config, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 512 {
		return Config{}, errors.New("the Jenkins URL is not a URL")
	}
	if u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || strings.ContainsAny(raw, "?#") {
		return Config{}, errors.New("the Jenkins URL must be https with no user name, query or fragment")
	}
	host := u.Hostname()
	labels := strings.Split(host, ".")
	if len(host) > 253 || !hostRe.MatchString(host) || numeric.MatchString(labels[len(labels)-1]) || allNumberLabels(labels) {
		return Config{}, errors.New("the Jenkins host must be a lowercase DNS name, not an IP address")
	}
	port := uint64(443)
	if p := u.Port(); p != "" {
		port, err = strconv.ParseUint(p, 10, 16)
		if err != nil || port == 0 || port == 80 {
			return Config{}, errors.New("the Jenkins port must be 1 to 65535 and not 80")
		}
	}
	prefix := strings.TrimSuffix(u.Path, "/")
	if prefix != "" && (!prefixRe.MatchString(prefix) || strings.Contains(prefix, "..")) {
		return Config{}, errors.New("the Jenkins context path is not a plain path")
	}
	return Config{Host: host, Port: uint32(port), Prefix: prefix}, nil
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
