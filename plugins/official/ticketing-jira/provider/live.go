package provider

import (
	"errors"
	"net"
	"net/http"
	"net/url"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// ErrReadOnly is the error of a request the live transport refuses because write-back is not enabled: a request that is not a
// read. Nothing is sent. ADR 0060 decision 9 asks a live provider to be unable to write through its egress rule; the broker
// injection rule cannot restrict a method, so this transport is that rule, beside the read-scoped credential the operator holds.
var ErrReadOnly = errors.New("the live transport sends reads only")

// Live is the http.RoundTripper of a launched live provider. It sends a request for the configured instance host to the egress
// proxy listener of the handle the request carries, as an absolute-form plain HTTP request for host:443 (the proxy terminates it,
// checks the destination against the manifest's allow-list, injects the credential it holds for the handle and re-originates it
// over TLS). It never opens a connection to the tracker or any other address, never sends a request that has no handle the host
// gave a proxy for or that names any other host, and sends a write only when write-back is enabled.
type Live struct {
	host      string
	writeBack bool
	rt        *http.Transport
}

// NewLive returns the transport over routes for the instance at host. writeBack lets it send a request that is not a read.
func NewLive(host string, routes *ghprovider.Routes, writeBack bool) *Live {
	return &Live{host: host, writeBack: writeBack, rt: &http.Transport{
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
	if req.Header.Get(ghprovider.HandleHeader) == "" || req.URL.Scheme != "https" || req.URL.Host != l.host || req.URL.User != nil {
		return nil, ghprovider.ErrNoEgress
	}
	if !l.writeBack && !isRead(req) {
		return nil, ErrReadOnly
	}
	out := req.Clone(req.Context())
	// The scheme is http because the sandbox side of the proxy speaks plain HTTP; the port says the proxy must use TLS upstream.
	out.URL.Scheme = "http"
	out.URL.Host = net.JoinHostPort(l.host, "443")
	out.Host = out.URL.Host
	return l.rt.RoundTrip(out)
}

// isRead reports whether a request only reads: a GET, or the one POST that is a search (Jira's enhanced search takes its query
// in a body).
func isRead(req *http.Request) bool {
	return req.Method == http.MethodGet || (req.Method == http.MethodPost && req.URL.EscapedPath() == apiRoot+"/search/jql")
}
