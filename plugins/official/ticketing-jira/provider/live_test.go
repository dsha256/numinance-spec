package provider_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/replay"
)

// The live build, end to end on loopback and with no tenant: the provider sends its requests through a fake egress proxy (plain
// HTTP, absolute form, the handle in a header) that injects a credential only it holds and re-originates them over TLS to a
// fake Jira. The fake Jira answers from the replay cassette, so the kit's whole run sees one consistent tracker. The instance
// host is the reserved replay.Host; it resolves to nothing, and only the proxy of this file dials a loopback listener for it.

// injected is the credential the fake proxy holds. The provider never sees it.
const injected = "Basic kit-proxy-held-credential"

type fakeJira struct {
	srv  *httptest.Server
	pool *x509.CertPool
	rt   http.RoundTripper // the cassette

	mu   sync.Mutex
	seen []string // "METHOD path" of each request, with the header checks already applied
	bad  []string // requests whose credential header was wrong
}

func newFakeJira(tb testing.TB) *fakeJira {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: replay.Host}, DNSNames: []string{replay.Host},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		tb.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		tb.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	tr, err := replay.Transport()
	if err != nil {
		tb.Fatal(err)
	}
	f := &fakeJira{pool: pool, rt: tr.RT}
	f.srv = httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	f.srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	f.srv.StartTLS()
	tb.Cleanup(f.srv.Close)
	return f
}

func (f *fakeJira) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.seen = append(f.seen, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != injected || r.Header.Get(ghprovider.HandleHeader) != "" {
		f.bad = append(f.bad, r.Method+" "+r.URL.Path)
	}
	f.mu.Unlock()
	if r.Header.Get("Authorization") != injected {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	// The cassette carries no credential and names the instance host.
	req, err := http.NewRequestWithContext(r.Context(), r.Method, "https://"+replay.Host+r.URL.RequestURI(), bytes.NewReader(body)) //nolint:gosec // the fake tracker rebuilds the request for the cassette; the host is the constant reserved name
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	resp, err := f.rt.RoundTrip(req)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (f *fakeJira) requests() (seen, bad []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...), append([]string(nil), f.bad...)
}

// fakeProxy is the egress proxy of the test: it judges the destination against the manifest's allow-list (the instance host on
// 443), strips the handle, injects the credential and re-originates the request over TLS to the fake Jira.
type fakeProxy struct {
	srv *httptest.Server

	mu       sync.Mutex
	handles  map[string]bool
	reqs     []string // "METHOD host:port"
	refusals int
}

func newFakeProxy(tb testing.TB, jira *fakeJira) *fakeProxy {
	tb.Helper()
	p := &fakeProxy{handles: map[string]bool{}}
	upstream := &http.Transport{
		// The one place the reserved name is resolved: to the loopback listener of the fake Jira.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, jira.srv.Listener.Addr().String())
		},
		TLSClientConfig: &tls.Config{RootCAs: jira.pool, ServerName: replay.Host, MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{Transport: upstream, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handle := r.Header.Get(ghprovider.HandleHeader)
		p.mu.Lock()
		p.reqs = append(p.reqs, r.Method+" "+r.URL.Host)
		known := p.handles[handle]
		if !known || r.URL.Scheme != "http" || r.URL.Host != replay.Host+":443" {
			p.refusals++
			p.mu.Unlock()
			w.WriteHeader(http.StatusForbidden)
			return
		}
		p.mu.Unlock()
		out, err := http.NewRequestWithContext(r.Context(), r.Method, "https://"+replay.Host+r.URL.RequestURI(), r.Body) //nolint:gosec // the fake proxy re-originates to the constant reserved name
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		for k, v := range r.Header {
			if !strings.EqualFold(k, ghprovider.HandleHeader) {
				out.Header[k] = v
			}
		}
		out.Header.Set("Authorization", injected)
		resp, err := client.Do(out) //nolint:gosec // the destination was checked above and is the constant reserved name
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	tb.Cleanup(p.srv.Close)
	return p
}

func (p *fakeProxy) issue(handle string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handles[handle] = true
}

func (p *fakeProxy) requests() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.reqs...)
}

// liveHost is the HostService of the live test: it issues a handle per call and tells the transport which proxy serves it, as
// the real host does in RequestCredential.
type liveHost struct {
	*recordingHost
	routes *ghprovider.Routes
	proxy  *fakeProxy
	fail   bool

	mu sync.Mutex
	n  int
}

func (h *liveHost) Credential(context.Context, string, string) (string, error) {
	if h.fail {
		return "", errors.New("no credential")
	}
	h.mu.Lock()
	h.n++
	handle := fmt.Sprintf("handle-%d", h.n)
	h.mu.Unlock()
	h.proxy.issue(handle)
	if err := h.routes.Set(handle, h.proxy.srv.Listener.Addr().String(), time.Now().Add(time.Minute)); err != nil {
		return "", err
	}
	return handle, nil
}

// credentialSpy records whether any request that reached it carried a header that authenticates (a handle excepted).
type credentialSpy struct {
	rt   http.RoundTripper
	mu   sync.Mutex
	held []string
}

func (s *credentialSpy) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Proxy-Authorization") != "" {
		s.mu.Lock()
		s.held = append(s.held, r.Method+" "+r.URL.Path)
		s.mu.Unlock()
	}
	return s.rt.RoundTrip(r)
}

// liveRig is the live provider with its fakes.
type liveRig struct {
	target *target
	jira   *fakeJira
	proxy  *fakeProxy
	spy    *credentialSpy
	host   *liveHost
}

func newLiveRig(tb testing.TB, writeBack bool) *liveRig {
	tb.Helper()
	jira := newFakeJira(tb)
	proxy := newFakeProxy(tb, jira)
	rec := &recordingHost{}
	routes := ghprovider.NewRoutes(time.Now)
	host := &liveHost{recordingHost: rec, routes: routes, proxy: proxy}
	spy := &credentialSpy{rt: provider.NewLive(replay.Host, routes, writeBack)}
	tp := &tap{rt: spy}
	srv, err := provider.New(provider.PluginID, replay.Host, "sha256:0", provider.Transport{RT: tp}, host, nil)
	if err != nil {
		tb.Fatal(err)
	}
	return &liveRig{target: &target{srv: srv, host: rec, tap: tp}, jira: jira, proxy: proxy, spy: spy, host: host}
}

// The kit's whole run, all 24 cases and none skipped, against the live build: TLS to a fake Jira on a reserved name, through a
// proxy that injects the credential. Alongside it, the properties that make the build safe to run: the provider never holds a
// credential, the tracker never sees a handle, every request is for the instance host on 443 and nothing is refused.
func TestLive_ConformanceKitPassesThroughTheProxyToATLSFake(t *testing.T) {
	rig := newLiveRig(t, true)
	s := subject(rig.target)
	cases := ticketingkit.Cases()
	if len(cases) != 24 {
		t.Fatalf("the kit has %d cases, want 24", len(cases))
	}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			err := c.Check(t.Context(), s)
			if errors.Is(err, ticketingkit.ErrNotApplicable) {
				t.Fatalf("%s (%s) was skipped: %v; the live Jira provider must run every case", c.ID, c.Name, err)
			}
			if err != nil {
				t.Errorf("%s (%s): %v", c.ID, c.Name, err)
			}
		})
	}
	seen, bad := rig.jira.requests()
	if len(seen) < 100 {
		t.Fatalf("the fake Jira saw %d requests, want the whole run", len(seen))
	}
	if len(bad) != 0 {
		t.Errorf("%d requests reached the tracker without the injected credential or with a handle, first %q", len(bad), bad[0])
	}
	if len(rig.spy.held) != 0 {
		t.Errorf("the provider held a credential header on %d requests, first %q", len(rig.spy.held), rig.spy.held[0])
	}
	reqs := rig.proxy.requests()
	if len(reqs) != len(seen) || rig.proxy.refusals != 0 {
		t.Errorf("the proxy saw %d requests (%d refused) for %d at the tracker, want the same and none refused", len(reqs), rig.proxy.refusals, len(seen))
	}
	for _, r := range reqs {
		if !strings.HasSuffix(r, " "+replay.Host+":443") {
			t.Fatalf("the proxy was asked for %q, want the instance host on port 443 only", r)
		}
	}
}

// Without write-back the live transport is a read-only egress rule: the reads and the search work, every write ends
// PERMISSION_DENIED and nothing but a GET or the search POST ever leaves the provider (ADR 0060 decision 9).
func TestLive_WithoutWriteBackNothingButReadsLeaves(t *testing.T) {
	rig := newLiveRig(t, false)
	ctx := t.Context()
	tg := rig.target
	if _, err := tg.GetIssue(ctx, ticketingkit.Options{}, getReq("PAY", "PAY-1")); err != nil {
		t.Fatalf("GetIssue = %v, want it to work", err)
	}
	if _, err := tg.SearchIssues(ctx, ticketingkit.Options{}, &ticketingv1.SearchIssuesRequest{Project: "PAY", Text: "invoice", MaxResults: 50}); err != nil {
		t.Fatalf("SearchIssues = %v, want it to work", err)
	}
	const done = ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE
	if _, ev, err := tg.TransitionIssue(ctx, ticketingkit.Options{DryRun: true}, &ticketingv1.TransitionIssueRequest{Project: "PAY", IssueKey: "PAY-6", Target: done}); err != nil || len(ev) != 0 {
		t.Fatalf("a dry-run TransitionIssue = %v, %v, want it to work and emit nothing", ev, err)
	}
	writes := map[string]func() error{
		"CommentOnIssue": func() error {
			_, _, err := tg.CommentOnIssue(ctx, ticketingkit.Options{Key: "kit-comment-1"}, &ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: "Checks passed."})
			return err
		},
		"TransitionIssue": func() error {
			_, _, err := tg.TransitionIssue(ctx, ticketingkit.Options{Key: "k"}, &ticketingv1.TransitionIssueRequest{Project: "PAY", IssueKey: "PAY-6", Target: done})
			return err
		},
		"LinkIssue": func() error {
			_, _, err := tg.LinkIssue(ctx, ticketingkit.Options{Key: "k"}, &ticketingv1.LinkIssueRequest{
				Project: "PAY", IssueKey: "PAY-8", BlockedProject: "PAY", BlockedIssueKey: "PAY-9", LinkType: ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS})
			return err
		},
	}
	for name, call := range writes {
		var ce *ticketingkit.CallError
		if err := call(); !errors.As(err, &ce) || ce.Reason != pluginv1.Reason_REASON_PERMISSION_DENIED {
			t.Errorf("%s without write-back = %v, want PERMISSION_DENIED", name, err)
		}
	}
	seen, _ := rig.jira.requests()
	for _, r := range seen {
		if !strings.HasPrefix(r, "GET ") && r != "POST /rest/api/3/search/jql" {
			t.Errorf("the tracker saw %q, want reads only", r)
		}
	}
}

// The transport sends nothing it should not: no handle, a handle the host gave no proxy for, another host, plain HTTP and user
// information are all refused before any connection is opened.
func TestLive_TransportRefusesWhatIsNotTheInstanceWithAHandle(t *testing.T) {
	rig := newLiveRig(t, true)
	routes := ghprovider.NewRoutes(time.Now)
	if err := routes.Set("known", rig.proxy.srv.Listener.Addr().String(), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	rig.proxy.issue("known")
	live := provider.NewLive(replay.Host, routes, true)
	tests := []struct{ name, target, handle string }{
		{"no handle", "https://" + replay.Host + "/rest/api/3/myself", ""},
		{"a handle with no proxy", "https://" + replay.Host + "/rest/api/3/myself", "unknown"},
		{"another host", "https://other.example.test/rest/api/3/myself", "known"},
		{"the host with a port", "https://" + replay.Host + ":8443/rest/api/3/myself", "known"},
		{"plain HTTP", "http://" + replay.Host + "/rest/api/3/myself", "known"},
		{"user information", "https://user@" + replay.Host + "/rest/api/3/myself", "known"},
		{"an address", "https://127.0.0.1/rest/api/3/myself", "known"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			if tc.handle != "" {
				req.Header.Set(ghprovider.HandleHeader, tc.handle)
			}
			resp, err := live.RoundTrip(req)
			if err == nil {
				_ = resp.Body.Close()
				t.Fatalf("RoundTrip(%s) succeeded, want a refusal", tc.target)
			}
		})
	}
	if n := len(rig.proxy.requests()); n != 0 {
		t.Errorf("the proxy saw %d requests, want none", n)
	}
	// The one thing that is allowed, to show the table above refuses for the right reason.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+replay.Host+"/rest/api/3/myself", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(ghprovider.HandleHeader, "known")
	resp, err := live.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip(the instance with a known handle) = %v, want it sent", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// A host that gives no credential handle ends the call as unavailable, and nothing is sent.
func TestLive_NoCredentialHandleSendsNothing(t *testing.T) {
	rig := newLiveRig(t, true)
	rig.host.fail = true
	_, err := rig.target.GetIssue(t.Context(), ticketingkit.Options{}, getReq("PAY", "PAY-1"))
	var ce *ticketingkit.CallError
	if !errors.As(err, &ce) || ce.Reason != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
		t.Errorf("GetIssue with no credential = %v, want UPSTREAM_UNAVAILABLE", err)
	}
	if n := len(rig.proxy.requests()); n != 0 {
		t.Errorf("the proxy saw %d requests, want none", n)
	}
}

// A redirect is never followed: the 3xx answer is an unexpected answer, and the second request, which would carry the handle to
// another destination, is never made.
func TestLive_RedirectIsNotFollowedAndTheAnswerIsFixed(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.Host)
		mu.Unlock()
		w.Header().Set("Location", "http://other.example.test/steal")
		w.WriteHeader(http.StatusFound)
	}))
	defer proxy.Close()
	routes := ghprovider.NewRoutes(time.Now)
	host := &stubHost{recordingHost: &recordingHost{}, routes: routes, proxy: proxy.Listener.Addr().String()}
	srv, err := provider.New(provider.PluginID, replay.Host, "sha256:0", provider.Transport{RT: provider.NewLive(replay.Host, routes, true)}, host, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = run(srv, "GetIssue", ticketingkit.Options{}, getReq("PAY", "PAY-1"))
	ce := failed(t, err)
	if ce.Reason != pluginv1.Reason_REASON_UPSTREAM_ERROR {
		t.Errorf("GetIssue behind a redirect = %s, want UPSTREAM_ERROR", ce.Reason)
	}
	if e := ticketingkit.ValidateErrorMessage(&ticketingkit.CallError{Reason: ce.Reason, Message: ce.Message}); e != nil {
		t.Error(e)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hits) != 1 {
		t.Errorf("the proxy saw %d requests (%v), want exactly the first", len(hits), hits)
	}
}

type stubHost struct {
	*recordingHost
	routes *ghprovider.Routes
	proxy  string
}

func (h *stubHost) Credential(context.Context, string, string) (string, error) {
	if err := h.routes.Set("stub", h.proxy, time.Now().Add(time.Minute)); err != nil {
		return "", err
	}
	return "stub", nil
}
