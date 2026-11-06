package ghprovider

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"time"
)

// maxRoutes bounds the handles whose proxy the provider remembers; the host issues one handle per invocation and intent, so
// a provider with this many live handles is already misbehaving or flooded.
const maxRoutes = 1024

// ErrNoEgress is the error of a request the live transport refuses to send: it carries no handle, names a handle the host
// gave no proxy for, or is for a destination other than the code host. Nothing is sent, and nothing falls back to a direct
// connection.
var ErrNoEgress = errors.New("the request has no egress proxy to go through")

// Routes remembers, for each credential handle the host issued, the egress proxy listener the host named with it. The handle
// is the key because it is bound to one invocation and one intent: the request carries it, the transport reads it. It holds
// no credential: a handle is opaque and an address is not a secret.
type Routes struct {
	mu    sync.Mutex
	now   func() time.Time
	byKey map[string]route
}

type route struct {
	proxy   netip.AddrPort
	expires time.Time
}

// NewRoutes returns an empty table. now is the clock that expires entries; time.Now in production.
func NewRoutes(now func() time.Time) *Routes {
	return &Routes{now: now, byKey: map[string]route{}}
}

// Set records the proxy of a handle until expires. A proxy that is not a literal "ip:port", or an empty handle, is refused.
// Expired entries are dropped first, and a full table refuses a new handle rather than evicting a live one.
func (r *Routes) Set(handle, proxy string, expires time.Time) error {
	ap, err := netip.ParseAddrPort(proxy)
	if err != nil || handle == "" {
		return ErrNoEgress
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for k, v := range r.byKey {
		if !v.expires.After(now) {
			delete(r.byKey, k)
		}
	}
	if _, known := r.byKey[handle]; !known && len(r.byKey) >= maxRoutes {
		return ErrNoEgress
	}
	r.byKey[handle] = route{proxy: ap, expires: expires}
	return nil
}

// ProxyFor returns the proxy recorded for a handle that has not expired.
func (r *Routes) ProxyFor(handle string) (netip.AddrPort, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.byKey[handle]
	if !ok || !v.expires.After(r.now()) {
		return netip.AddrPort{}, false
	}
	return v.proxy, true
}

// Live is the http.RoundTripper of a launched provider. It sends a request for the code host to the egress proxy listener of
// the handle the request carries, as an absolute-form plain HTTP request for api.github.com:443 (the proxy terminates it,
// applies the credential it holds for the handle and re-originates it over TLS). It never opens a connection to the code host
// or any other address, and never sends a request that lacks a handle the host gave a proxy for.
type Live struct {
	routes *Routes
	rt     *http.Transport
}

// NewLive returns the transport over routes.
func NewLive(routes *Routes) *Live {
	l := &Live{routes: routes}
	l.rt = &http.Transport{
		// Per request, so a connection is only ever made to the proxy of that request's handle; there is no ambient proxy.
		Proxy: func(req *http.Request) (*url.URL, error) {
			ap, ok := routes.ProxyFor(req.Header.Get(HandleHeader))
			if !ok {
				return nil, ErrNoEgress
			}
			return &url.URL{Scheme: "http", Host: ap.String()}, nil
		},
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
		ResponseHeaderTimeout: CallTimeout,
	}
	return l
}

// RoundTrip implements http.RoundTripper.
func (l *Live) RoundTrip(req *http.Request) (*http.Response, error) {
	base, err := url.Parse(BaseURL)
	if err != nil || req.Header.Get(HandleHeader) == "" || req.URL.Scheme != base.Scheme || req.URL.Host != base.Host {
		return nil, ErrNoEgress
	}
	out := req.Clone(req.Context())
	// The scheme is http because the sandbox side of the proxy speaks plain HTTP; the port says the proxy must use TLS upstream.
	out.URL.Scheme = "http"
	out.URL.Host = net.JoinHostPort(base.Hostname(), "443")
	out.Host = out.URL.Host
	return l.rt.RoundTrip(out)
}
