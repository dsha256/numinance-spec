package provider_test

import (
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
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	cikit "github.com/numinance/numinance-spec/conformance/ciexecutor/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// The live build, end to end on loopback and with no Jenkins: the provider sends its requests through a fake egress proxy (plain
// HTTP, absolute form, the handle in a header) that injects a credential only it holds and re-originates them over TLS to a
// fake Jenkins. The instance host is the reserved jenkins.example.test; it resolves to nothing, and only the proxy of this file
// dials a loopback listener for it.

const (
	exampleHost = "jenkins.example.test"
	// injected is the credential the fake proxy holds. The provider never sees it.
	injected = "Basic kit-proxy-held-credential"
)

// tlsJenkins serves the stateful fake over TLS. It refuses a request without the injected credential, and a POST without the crumb
// it issued, as a hardened Jenkins does.
type tlsJenkins struct {
	srv  *httptest.Server
	pool *x509.CertPool
	f    *fake

	mu   sync.Mutex
	seen []string
	bad  []string
}

func newTLSJenkins(tb testing.TB, f *fake) *tlsJenkins {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: exampleHost}, DNSNames: []string{exampleHost},
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
	j := &tlsJenkins{pool: pool, f: f}
	j.srv = httptest.NewUnstartedServer(http.HandlerFunc(j.serve))
	j.srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	j.srv.StartTLS()
	tb.Cleanup(j.srv.Close)
	return j
}

func (j *tlsJenkins) serve(w http.ResponseWriter, r *http.Request) {
	j.mu.Lock()
	j.seen = append(j.seen, r.Method+" "+r.URL.Path)
	switch {
	case r.Header.Get("Authorization") != injected || r.Header.Get(ghprovider.HandleHeader) != "":
		j.bad = append(j.bad, "credential: "+r.Method+" "+r.URL.Path)
	case r.Method == http.MethodPost && r.Header.Get(j.f.crumbField) != j.f.crumb:
		j.bad = append(j.bad, "crumb: "+r.Method+" "+r.URL.Path)
	}
	bad := len(j.bad) > 0 && strings.HasSuffix(j.bad[len(j.bad)-1], r.Method+" "+r.URL.Path)
	j.mu.Unlock()
	if r.Header.Get("Authorization") != injected {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if bad {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	resp, err := j.f.RoundTrip(r)
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
}

func (j *tlsJenkins) requests() (seen, bad []string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.seen...), append([]string(nil), j.bad...)
}

// tlsProxy is the egress proxy of the test: it judges the destination against the allow-list (the instance host on 443), strips
// the handle, injects the credential and re-originates the request over TLS to the fake Jenkins.
type tlsProxy struct {
	srv *httptest.Server

	mu       sync.Mutex
	handles  map[string]bool
	reqs     []string
	refusals int
}

func newTLSProxy(tb testing.TB, j *tlsJenkins) *tlsProxy {
	tb.Helper()
	p := &tlsProxy{handles: map[string]bool{}}
	upstream := &http.Transport{
		// The one place the reserved name is resolved: to the loopback listener of the fake Jenkins.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, j.srv.Listener.Addr().String())
		},
		TLSClientConfig: &tls.Config{RootCAs: j.pool, ServerName: exampleHost, MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{Transport: upstream, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.reqs = append(p.reqs, r.Method+" "+r.URL.Host)
		known := p.handles[r.Header.Get(ghprovider.HandleHeader)]
		if !known || r.URL.Scheme != "http" || r.URL.Host != exampleHost+":443" {
			p.refusals++
			p.mu.Unlock()
			w.WriteHeader(http.StatusForbidden)
			return
		}
		p.mu.Unlock()
		out, err := http.NewRequestWithContext(r.Context(), r.Method, "https://"+exampleHost+r.URL.RequestURI(), r.Body) //nolint:gosec // the fake proxy re-originates to the constant reserved name
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

func (p *tlsProxy) issue(handle string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handles[handle] = true
}

func (p *tlsProxy) stats() (reqs []string, refusals int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.reqs...), p.refusals
}

// liveHost is the HostService of the live test: it issues a handle per call and tells the transport which proxy serves it, as the
// real host does in RequestCredential.
type liveHost struct {
	routes *ghprovider.Routes
	proxy  *tlsProxy

	mu sync.Mutex
	n  int
}

func (h *liveHost) Credential(context.Context, string, string) (string, error) {
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

func (h *liveHost) Emit(context.Context, string, string, string, *structpb.Struct) (string, error) {
	return "ev_live", nil
}

// credentialSpy records any request that reached it carrying a header that authenticates (the handle excepted).
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

// kitTarget reaches the provider with no host process, as the kit's replay does.
type kitTarget struct {
	s    *provider.Server
	f    *fake
	once sync.Once
}

func kitCall[Resp proto.Message](ctx context.Context, k *kitTarget, method string, o cikit.Options, req proto.Message) (Resp, error) {
	var zero Resp
	resp, _, err := k.s.Direct(ctx, rpc+method, &pluginv1.ActionContext{DryRun: o.DryRun, IdempotencyKey: o.Key}, req)
	var ce *provider.CallError
	if errors.As(err, &ce) {
		return zero, &cikit.CallError{Reason: ce.Reason}
	}
	if err != nil {
		return zero, err
	}
	out, ok := resp.(Resp)
	if !ok {
		return zero, fmt.Errorf("%s answered %T", method, resp)
	}
	return out, nil
}

func (k *kitTarget) StartOperation(ctx context.Context, o cikit.Options, r *civ1.StartOperationRequest) (*civ1.StartOperationResponse, error) {
	return kitCall[*civ1.StartOperationResponse](ctx, k, "StartOperation", o, r)
}

// GetOperation lets the run the kit watches progress between its two looks, as a real pipeline does.
func (k *kitTarget) GetOperation(ctx context.Context, o cikit.Options, r *civ1.GetOperationRequest) (*civ1.GetOperationResponse, error) {
	resp, err := kitCall[*civ1.GetOperationResponse](ctx, k, "GetOperation", o, r)
	if r.GetOperationId() == opID(5) {
		k.once.Do(func() { k.f.finishBuild(5, "SUCCESS", 4000) })
	}
	return resp, err
}

func (k *kitTarget) CancelOperation(ctx context.Context, o cikit.Options, r *civ1.CancelOperationRequest) (*civ1.CancelOperationResponse, error) {
	return kitCall[*civ1.CancelOperationResponse](ctx, k, "CancelOperation", o, r)
}

func (k *kitTarget) ListArtifacts(ctx context.Context, o cikit.Options, r *civ1.ListArtifactsRequest) (*civ1.ListArtifactsResponse, error) {
	return kitCall[*civ1.ListArtifactsResponse](ctx, k, "ListArtifacts", o, r)
}

func (k *kitTarget) GetLogReference(ctx context.Context, o cikit.Options, r *civ1.GetLogReferenceRequest) (*civ1.GetLogReferenceResponse, error) {
	return kitCall[*civ1.GetLogReferenceResponse](ctx, k, "GetLogReference", o, r)
}

// opID is the operation id of the seeded run n: 31 zeros and a digit are a digest, and the pipeline is release.
func opID(n int) string { return fmt.Sprintf("%032d.release", n) }

func (f *fake) finishBuild(n int, result string, duration int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.builds {
		if f.builds[i].n == n {
			f.builds[i].building, f.builds[i].result, f.builds[i].duration = false, str(result), duration
		}
	}
}

// seededFake is a Jenkins with the three runs the kit reads: 3 is running and a cancel ends it, 5 is running and finishes, 6 has
// ended with artifacts. The job "dry" exists for the dry run.
func seededFake() *fake {
	f := newFake()
	seed := func(n int, building bool, result *string) fakeBuild {
		return fakeBuild{n: n, building: building, result: result, duration: 3000, form: url.Values{"numinance_operation_id": {fmt.Sprintf("%032d", n)}, "numinance_revision": {rev}}}
	}
	f.builds = []fakeBuild{seed(6, false, str("SUCCESS")), seed(5, true, nil), seed(3, true, nil)}
	f.artifacts = map[int][]fakeArtifact{6: {{path: "out/app.tar.gz", size: 1234}, {path: "report.txt", size: 12}}}
	f.extraJobs = []string{"dry"}
	return f
}

// The kit's whole run, all eight cases, against the live build: TLS to a fake Jenkins on a reserved name, through a proxy that
// injects the credential. Alongside it, the properties that make the build safe to run: the provider never holds a credential,
// Jenkins never sees a handle, every POST carries the crumb, every request is for the instance host on 443 and nothing is refused.
func TestLive_ConformanceKitPassesThroughTheProxyToATLSFake(t *testing.T) {
	f := seededFake()
	jenkins := newTLSJenkins(t, f)
	proxy := newTLSProxy(t, jenkins)
	routes := ghprovider.NewRoutes(time.Now)
	cfg, err := provider.ParseURL("https://" + exampleHost)
	if err != nil {
		t.Fatal(err)
	}
	spy := &credentialSpy{rt: provider.NewLive(cfg, routes)}
	srv := provider.New(provider.PluginID, "sha256:0", cfg, provider.Transport{RT: spy}, 0, &liveHost{routes: routes, proxy: proxy}, nil)
	cikit.Run(t, cikit.Subject{
		Target: &kitTarget{s: srv, f: f}, Repository: repo, Missing: "acme/missing", Pipeline: "release", DryRunPipeline: "dry",
		Revision: rev, StartKey: "chg_0001/release/1", Params: map[string]string{"environment": "staging"},
		Progressing: opID(5), Cancellable: opID(3), WithArtifacts: opID(6),
	})
	seen, bad := jenkins.requests()
	if len(seen) < 25 {
		t.Fatalf("the fake Jenkins saw %d requests, want the whole run", len(seen))
	}
	if len(bad) != 0 {
		t.Errorf("%d requests were wrong at Jenkins (no injected credential, a handle, or a POST without the crumb), first %q", len(bad), bad[0])
	}
	if len(spy.held) != 0 {
		t.Errorf("the provider held a credential header on %d requests, first %q", len(spy.held), spy.held[0])
	}
	reqs, refusals := proxy.stats()
	if len(reqs) != len(seen) || refusals != 0 {
		t.Errorf("the proxy saw %d requests (%d refused) for %d at Jenkins, want the same and none refused", len(reqs), refusals, len(seen))
	}
	for _, r := range reqs {
		if !strings.HasSuffix(r, " "+exampleHost+":443") {
			t.Fatalf("the proxy was asked for %q, want the instance host on port 443 only", r)
		}
	}
	posts := 0
	for _, s := range seen {
		if strings.HasPrefix(s, "POST ") {
			posts++
		}
	}
	// One build request (the repeat under the same key and the dry run post nothing) and one stop (the repeat of the cancel finds the
	// run ended).
	if posts != 2 {
		t.Errorf("posts at Jenkins = %d, want 2: the build request and the stop (%v)", posts, seen)
	}
}
