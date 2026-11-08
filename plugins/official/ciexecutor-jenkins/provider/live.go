package provider

import (
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// Live is the http.RoundTripper of a launched provider. It sends a request for the configured Jenkins server to the egress
// proxy listener of the handle the request carries, as an absolute-form plain HTTP request for host:port (the proxy terminates
// it, injects the credential it holds for the handle and re-originates it over TLS). It never opens a connection to Jenkins
// or any other address, and never sends a request that lacks a handle the host gave a proxy for or that is for any
// destination other than the configured one.
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
	if req.Header.Get(ghprovider.HandleHeader) == "" || req.URL.Scheme != "https" || req.URL.Host != l.cfg.Authority() {
		return nil, ghprovider.ErrNoEgress
	}
	out := req.Clone(req.Context())
	// The scheme is http because the sandbox side of the proxy speaks plain HTTP; every port but 80 is TLS upstream.
	out.URL.Scheme = "http"
	out.URL.Host = net.JoinHostPort(l.cfg.Host, strconv.FormatUint(uint64(l.cfg.Port), 10))
	out.Host = out.URL.Host
	return l.rt.RoundTrip(out)
}
