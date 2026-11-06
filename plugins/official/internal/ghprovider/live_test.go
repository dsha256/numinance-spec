package ghprovider_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

var t0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // immutable test instant

type clk struct{ now time.Time }

func (c *clk) Now() time.Time { return c.now }

func TestRoutes_SetAndProxyFor(t *testing.T) {
	t.Parallel()
	c := &clk{now: t0}
	r := ghprovider.NewRoutes(c.Now)
	if err := r.Set("h1", "10.200.0.1:34000", t0.Add(time.Minute)); err != nil {
		t.Fatalf("Set() error = %v, want nil", err)
	}
	if got, ok := r.ProxyFor("h1"); !ok || got != netip.MustParseAddrPort("10.200.0.1:34000") {
		t.Errorf("ProxyFor(h1) = %v, %v, want the recorded proxy", got, ok)
	}
	if _, ok := r.ProxyFor("other"); ok {
		t.Error("ProxyFor(unknown handle) = true, want false")
	}
	c.now = t0.Add(time.Minute)
	if _, ok := r.ProxyFor("h1"); ok {
		t.Error("ProxyFor(expired handle) = true, want false")
	}
}

func TestRoutes_SetRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, handle, proxy string
	}{
		{name: "no handle", handle: "", proxy: "10.0.0.1:1"},
		{name: "empty proxy", handle: "h", proxy: ""},
		{name: "host name instead of an address", handle: "h", proxy: "proxy.internal:3128"},
		{name: "no port", handle: "h", proxy: "10.0.0.1"},
		{name: "a URL", handle: "h", proxy: "http://10.0.0.1:3128"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := ghprovider.NewRoutes(func() time.Time { return t0 })
			if err := r.Set(tt.handle, tt.proxy, t0.Add(time.Hour)); !errors.Is(err, ghprovider.ErrNoEgress) {
				t.Errorf("Set(%q, %q) = %v, want ErrNoEgress", tt.handle, tt.proxy, err)
			}
		})
	}
}

func TestRoutes_FullTableRefusesANewHandleAndDropsExpiredOnes(t *testing.T) {
	t.Parallel()
	c := &clk{now: t0}
	r := ghprovider.NewRoutes(c.Now)
	for i := range 1024 {
		if err := r.Set("h"+strconv.Itoa(i), "10.0.0.1:1", t0.Add(time.Minute)); err != nil {
			t.Fatalf("Set(%d) error = %v, want nil below the bound", i, err)
		}
	}
	if err := r.Set("one-more", "10.0.0.1:1", t0.Add(time.Minute)); !errors.Is(err, ghprovider.ErrNoEgress) {
		t.Errorf("Set(past the bound) = %v, want ErrNoEgress", err)
	}
	if err := r.Set("h0", "10.0.0.2:2", t0.Add(time.Minute)); err != nil {
		t.Errorf("Set(known handle at the bound) = %v, want nil", err)
	}
	c.now = t0.Add(2 * time.Minute)
	if err := r.Set("one-more", "10.0.0.1:1", c.now.Add(time.Minute)); err != nil {
		t.Errorf("Set(after the others expired) = %v, want nil", err)
	}
}

// proxyDouble is a plain HTTP listener that records the request the way a proxy receives it.
type proxyDouble struct {
	mu   sync.Mutex
	uris []string
	hdr  []http.Header
	addr string
}

func newProxyDouble(t *testing.T) *proxyDouble {
	t.Helper()
	p := &proxyDouble{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.uris = append(p.uris, r.RequestURI)
		p.hdr = append(p.hdr, r.Header.Clone())
		p.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	p.addr = srv.Listener.Addr().String()
	return p
}

func (p *proxyDouble) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.uris...)
}

func request(t *testing.T, url, handle string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	if handle != "" {
		req.Header.Set(ghprovider.HandleHeader, handle)
	}
	return req
}

func TestLive_SendsAnAbsoluteFormRequestToTheHandlesProxyOnly(t *testing.T) {
	t.Parallel()
	p := newProxyDouble(t)
	routes := ghprovider.NewRoutes(func() time.Time { return t0 })
	if err := routes.Set("nh_one", p.addr, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	resp, err := ghprovider.NewLive(routes).RoundTrip(request(t, ghprovider.BaseURL+"/repos/acme/shop?x=1", "nh_one"))
	if err != nil {
		t.Fatalf("RoundTrip() error = %v, want nil", err)
	}
	_ = resp.Body.Close()
	if got, want := p.seen(), []string{"http://api.github.com:443/repos/acme/shop?x=1"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("proxy received %q, want %q (absolute form, port 443: the proxy terminates TLS)", got, want)
	}
	if h := p.hdr[0]; h.Get(ghprovider.HandleHeader) != "nh_one" || h.Get("Authorization") != "" {
		t.Errorf("proxy headers = %v, want the handle and no Authorization", h)
	}
}

func TestLive_RefusesWithoutSendingAnything(t *testing.T) {
	t.Parallel()
	p := newProxyDouble(t)
	routes := ghprovider.NewRoutes(func() time.Time { return t0 })
	if err := routes.Set("nh_one", p.addr, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := routes.Set("nh_old", p.addr, t0.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, url, handle string
	}{
		{name: "no handle", url: ghprovider.BaseURL + "/x"},
		{name: "a handle the host gave no proxy for", url: ghprovider.BaseURL + "/x", handle: "nh_unknown"},
		{name: "an expired handle", url: ghprovider.BaseURL + "/x", handle: "nh_old"},
		{name: "another host", url: "https://example.com/x", handle: "nh_one"},
		{name: "another port", url: "https://api.github.com:8443/x", handle: "nh_one"},
		{name: "plain http to the code host", url: "http://api.github.com/x", handle: "nh_one"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ghprovider.NewLive(routes).RoundTrip(request(t, tt.url, tt.handle)); err == nil {
				t.Errorf("RoundTrip(%s, %q) error = nil, want a refusal", tt.url, tt.handle)
			}
		})
	}
	if got := p.seen(); len(got) != 0 {
		t.Errorf("proxy received %q, want nothing: a refused request is never sent anywhere", got)
	}
}

// hostDouble is a HostService that answers RequestCredential with a fixed response.
type hostDouble struct {
	pluginv1.UnimplementedHostServiceServer
	resp *pluginv1.RequestCredentialResponse
}

func (h hostDouble) RequestCredential(context.Context, *pluginv1.RequestCredentialRequest) (*pluginv1.RequestCredentialResponse, error) {
	return h.resp, nil
}

func serveHost(t *testing.T, resp *pluginv1.RequestCredentialResponse) *ghprovider.GRPCHost {
	t.Helper()
	dir, err := os.MkdirTemp("", "gh") //nolint:usetesting // a short path: a Unix socket path is limited to about 100 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "h.sock")
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pluginv1.RegisterHostServiceServer(srv, hostDouble{resp: resp})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	h, err := ghprovider.DialHost("unix:" + sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func TestGRPCHost_Credential_RecordsTheProxyOfTheHandle(t *testing.T) {
	t.Parallel()
	h := serveHost(t, &pluginv1.RequestCredentialResponse{Handle: "nh_abc", ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)), EgressProxy: "127.0.0.1:34000"})
	got, err := h.Credential(t.Context(), "inv_1", "scm.github.api")
	if err != nil || got != "nh_abc" {
		t.Fatalf("Credential() = %q, %v, want the handle", got, err)
	}
	if ap, ok := h.Routes().ProxyFor("nh_abc"); !ok || ap != netip.MustParseAddrPort("127.0.0.1:34000") {
		t.Errorf("Routes().ProxyFor(handle) = %v, %v, want the proxy the host named", ap, ok)
	}
}

func TestGRPCHost_Credential_RefusesAHandleWithoutAUsableProxy(t *testing.T) {
	t.Parallel()
	exp := timestamppb.New(time.Now().Add(time.Hour))
	tests := []struct {
		name string
		resp *pluginv1.RequestCredentialResponse
	}{
		{name: "no proxy address", resp: &pluginv1.RequestCredentialResponse{Handle: "nh_abc", ExpiresAt: exp}},
		{name: "a host name", resp: &pluginv1.RequestCredentialResponse{Handle: "nh_abc", ExpiresAt: exp, EgressProxy: "evil.example:80"}},
		{name: "no handle", resp: &pluginv1.RequestCredentialResponse{ExpiresAt: exp, EgressProxy: "127.0.0.1:1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := serveHost(t, tt.resp)
			if got, err := h.Credential(t.Context(), "inv_1", "scm.github.api"); err == nil || got != "" {
				t.Errorf("Credential() = %q, %v, want no handle and an error: it could only be sent nowhere", got, err)
			}
		})
	}
}
