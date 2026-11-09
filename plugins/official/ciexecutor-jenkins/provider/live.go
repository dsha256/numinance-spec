package provider

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// Live is the http.RoundTripper of a launched provider. It sends a request for the configured Jenkins server to the egress
// proxy listener of the handle the request carries, as an absolute-form plain HTTP request for host:443 (the proxy terminates
// it, checks the destination against the allow-list the broker pushed for the step, injects the credential it holds for the
// handle and re-originates it over TLS). It never opens a connection to Jenkins or any other address, and never sends a request
// that lacks a handle the host gave a proxy for, that is for any destination other than the configured one, or that is not one
// of the requests this provider makes (see allowed).
type Live struct {
	cfg Config
	rt  *http.Transport
}

// NewLive returns the transport over routes for the server cfg names.
func NewLive(cfg Config, routes *ghprovider.Routes) *Live {
	return &Live{cfg: cfg, rt: &http.Transport{
		// Per request, so a connection is only ever made to the proxy of that request's handle; there is no ambient proxy.
		Proxy: func(req *http.Request) (*url.URL, error) {
			ap, ok := routes.ProxyFor(req.Header.Get(ghprovider.HandleHeader))
			if !ok {
				return nil, ghprovider.ErrNoEgress
			}
			return &url.URL{Scheme: "http", Host: ap.String()}, nil
		},
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
		ResponseHeaderTimeout: callTimeout,
	}}
}

// RoundTrip implements http.RoundTripper.
func (l *Live) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get(ghprovider.HandleHeader) == "" || req.URL.Scheme != "https" || req.URL.Host != l.cfg.Authority() || req.URL.User != nil ||
		(req.Host != "" && req.Host != req.URL.Host) || !l.allowed(req) {
		return nil, ghprovider.ErrNoEgress
	}
	out := req.Clone(req.Context())
	// The scheme is http because the sandbox side of the proxy speaks plain HTTP; the port says the proxy must use TLS upstream.
	out.URL.Scheme = "http"
	out.URL.Host = net.JoinHostPort(l.cfg.Host, strconv.FormatUint(uint64(l.cfg.Port), 10))
	out.Host = out.URL.Host
	return l.rt.RoundTrip(out)
}

// allowed reports whether the request is of the kind this provider makes: a GET, a HEAD or a POST for a path under the context
// path that begins /job/, /queue/ or /crumbIssuer/, with no dot segment and no escaped dot, slash, backslash or NUL. The
// injected credential is the operator's Jenkins account, and an injection rule can not restrict a method or a path inside the
// host, so the provider keeps itself off the rest of the server (the script console, credentials, plugin manager, users).
func (l *Live) allowed(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodPost:
	default:
		return false
	}
	path := req.URL.EscapedPath()
	rest, ok := strings.CutPrefix(path, l.cfg.Prefix)
	if !ok || !plainEscapes(rest) {
		return false
	}
	for _, seg := range strings.Split(rest, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return strings.HasPrefix(rest, "/job/") || strings.HasPrefix(rest, "/queue/") || strings.HasPrefix(rest, "/crumbIssuer/")
}

// plainEscapes reports whether every percent escape of the path is one PathEscape writes for a name: none of a dot, slash,
// backslash or NUL, which a server may decode into a segment of its own.
func plainEscapes(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] != '%' {
			continue
		}
		if i+2 >= len(path) {
			return false
		}
		switch strings.ToUpper(path[i+1 : i+3]) {
		case "2E", "2F", "5C", "00":
			return false
		}
	}
	return true
}
