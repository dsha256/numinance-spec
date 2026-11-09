package provider_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// stream is the server side of one Invoke, as the host sees it.
type stream struct {
	grpc.ServerStream
	ctx  context.Context //nolint:containedctx // a test double of the stream the gRPC server hands a handler
	sent []*pluginv1.InvokeResponse
}

func (s *stream) Context() context.Context { return s.ctx }

func (s *stream) Send(m *pluginv1.InvokeResponse) error {
	s.sent = append(s.sent, m)
	return nil
}

// invoke dispatches one call through Invoke and returns the one terminal result the host reads: status, evidence ids, side effects
// and diagnostics, which Direct does not show.
func invoke(t *testing.T, s *provider.Server, method, key string, req proto.Message) *pluginv1.ActionResult {
	t.Helper()
	packed, err := anypb.New(req)
	if err != nil {
		t.Fatal(err)
	}
	st := &stream{ctx: t.Context()}
	err = s.Invoke(&pluginv1.InvokeRequest{Envelope: &pluginv1.Envelope{
		Method: rpc + method, Request: packed,
		Context: &pluginv1.ActionContext{InvocationId: "inv-1", PolicyDecisionId: "pd-1", IdempotencyKey: key},
	}}, st)
	if err != nil || len(st.sent) != 1 {
		t.Fatalf("Invoke(%s) = %v with %d messages, want one terminal message", method, err, len(st.sent))
	}
	return st.sent[0].GetResult()
}

func failedWith(res *pluginv1.ActionResult) pluginv1.Reason {
	if res.GetStatus() != pluginv1.InvokeStatus_INVOKE_STATUS_FAILED || len(res.GetDiagnostics()) != 1 {
		return pluginv1.Reason_REASON_UNSPECIFIED
	}
	return res.GetDiagnostics()[0].GetReason()
}

func wrote(res *pluginv1.ActionResult, kind pluginv1.SideEffect) bool {
	for _, e := range res.GetSideEffects() {
		if e.GetKind() == kind {
			return true
		}
	}
	return false
}

func TestEmit_AFailedReceiptIsNotDroppedSilently(t *testing.T) {
	t.Parallel()
	t.Run("start: the run is reported with a warning and no evidence id, and a retry records the receipt", func(t *testing.T) {
		t.Parallel()
		f, h := newFake(), &host{emitErr: errors.New("host evidence store is down")}
		s := f.server(h)
		res := invoke(t, s, "StartOperation", "k1", startReq())
		if res.GetStatus() != pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED || len(res.GetEvidenceIds()) != 0 || !wrote(res, pluginv1.SideEffect_SIDE_EFFECT_DEPLOY) {
			t.Fatalf("start = %v, want SUCCEEDED with the DEPLOY side effect and no evidence id (the host derives PARTIAL)", res)
		}
		if d := res.GetDiagnostics(); len(d) != 1 || d[0].GetSeverity() != pluginv1.Severity_SEVERITY_WARNING || d[0].GetReason() != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
			t.Errorf("diagnostics = %v, want one WARNING with UPSTREAM_UNAVAILABLE", d)
		}
		h.emitErr = nil
		again := invoke(t, s, "StartOperation", "k1", startReq())
		if len(again.GetEvidenceIds()) != 1 || len(again.GetDiagnostics()) != 0 || f.posts() != 1 {
			t.Errorf("retry = %v with %d posts, want the receipt now recorded, no warning and no second run", again, f.posts())
		}
	})
	t.Run("cancel: the stop is reported with a warning and no evidence id", func(t *testing.T) {
		t.Parallel()
		f, h := newFake(), &host{}
		s := f.server(h)
		st, _ := start(t, s, "k1")
		f.run()
		h.emitErr = errors.New("host evidence store is down")
		res := invoke(t, s, "CancelOperation", "k", &civ1.CancelOperationRequest{Repository: repo, OperationId: st.GetOperationId()})
		if res.GetStatus() != pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED || len(res.GetEvidenceIds()) != 0 || !wrote(res, pluginv1.SideEffect_SIDE_EFFECT_WRITE) || len(res.GetDiagnostics()) != 1 {
			t.Errorf("cancel = %v, want SUCCEEDED with the WRITE side effect, no evidence id and one warning", res)
		}
	})
	t.Run("a recorded receipt carries no warning", func(t *testing.T) {
		t.Parallel()
		res := invoke(t, newFake().server(&host{}), "StartOperation", "k1", startReq())
		if len(res.GetEvidenceIds()) != 1 || len(res.GetDiagnostics()) != 0 {
			t.Errorf("start = %v, want one evidence id and no diagnostics", res)
		}
	})
}

func TestCancelOperation_AQueuedRunIsCancelledEvenWhenTheWindowOfBuildsIsFull(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	st, err := start(t, s, "k1")
	if err != nil {
		t.Fatalf("start = %v", err)
	}
	// Other runs fill the window of builds after the start.
	for i := range 100 {
		f.builds = append(f.builds, fakeBuild{n: i + 1, form: map[string][]string{"numinance_operation_id": {fmt.Sprintf("%032x", i+1)}}, result: str("SUCCESS")})
	}
	got, err := cancel(t, s, st.GetOperationId())
	if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED || len(f.queue) != 0 {
		t.Errorf("cancel = %v, %v, queue %d, want CANCELLED: the item left the queue by our cancel, and a run that had started would head the window", got, err, len(f.queue))
	}
}

func isBuildRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/buildWithParameters")
}

func TestStartOperation_AnUnknownOutcomeOfTheBuildRequestIsSettledByLookingAtJenkins(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		accepted bool
		err      error
		wantOK   bool
		wantFail pluginv1.Reason
	}{
		{"timed out after Jenkins took the build", true, context.DeadlineExceeded, true, 0},
		{"connection reset after Jenkins took the build", true, errors.New("connection reset by peer"), true, 0},
		{"timed out and Jenkins did not take it", false, context.DeadlineExceeded, false, pluginv1.Reason_REASON_TIMEOUT},
		{"connection reset and Jenkins did not take it", false, errors.New("connection reset by peer"), false, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, h := newFake(), &host{}
			// The answer to the build request is lost; whether Jenkins took the build is the case's.
			rt := &roundTripWrap{f: f, lose: func(r *http.Request) (bool, error) {
				if isBuildRequest(r) {
					return !tc.accepted, tc.err
				}
				return false, nil
			}}
			s := provider.New(provider.PluginID, "sha256:0", mustCfg(t), provider.Transport{RT: rt}, 0, h, nil)
			res := invoke(t, s, "StartOperation", "k1", startReq())
			if tc.wantOK {
				if res.GetStatus() != pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED || !wrote(res, pluginv1.SideEffect_SIDE_EFFECT_DEPLOY) || len(res.GetEvidenceIds()) != 1 {
					t.Errorf("start = %v, want SUCCEEDED with the DEPLOY side effect and the receipt: Jenkins did take the build", res)
				}
				var out civ1.StartOperationResponse
				if err := res.GetResult().UnmarshalTo(&out); err != nil || out.GetOperationId() == "" || out.GetExisting() || out.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED {
					t.Errorf("response = %v (%v), want a new queued run", &out, err)
				}
				if f.posts() != 1 {
					t.Errorf("posts = %d, want 1", f.posts())
				}
				return
			}
			if got := failedWith(res); got != tc.wantFail || wrote(res, pluginv1.SideEffect_SIDE_EFFECT_DEPLOY) {
				t.Errorf("start = %v, want FAILED with %s and no claim of a write: the outcome is not known", res, tc.wantFail)
			}
			// The retry under the same key finds nothing and starts the run once.
			rt.lose = nil
			if _, err := start(t, provider.New(provider.PluginID, "sha256:0", mustCfg(t), provider.Transport{RT: rt}, 0, h, nil), "k1"); err != nil || f.posts() != 1 {
				t.Errorf("retry = %v with %d posts, want one run", err, f.posts())
			}
		})
	}
}

func TestStartOperation_AnAcceptedBuildWithAnOddAnswerIsStillTheRun(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		resp *http.Response
	}{
		{"201 without a Location", answer(201, "")},
		{"a Location that is not a queue item", answer(201, "", "Location", "https://jenkins.example.test/job/x/")},
		{"502 from the proxy in front", answer(502, "<html>bad gateway</html>")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			rt := &roundTripWrap{f: f, replace: func(r *http.Request) *http.Response {
				if isBuildRequest(r) {
					return tc.resp
				}
				return nil
			}}
			s := provider.New(provider.PluginID, "sha256:0", mustCfg(t), provider.Transport{RT: rt}, 0, &host{}, nil)
			res := invoke(t, s, "StartOperation", "k1", startReq())
			if res.GetStatus() != pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED || !wrote(res, pluginv1.SideEffect_SIDE_EFFECT_DEPLOY) || f.posts() != 1 {
				t.Errorf("start = %v with %d posts, want the run Jenkins did take, found by its key", res, f.posts())
			}
		})
	}
}

func TestRunInTheGapBetweenTheQueueAndTheBuildList(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*fake, *provider.Server, string) {
		t.Helper()
		f := newFake()
		s := f.server(&host{})
		st, err := start(t, s, "k1")
		if err != nil {
			t.Fatal(err)
		}
		f.leave() // off the queue, no build in the list yet
		return f, s, st.GetOperationId()
	}
	t.Run("a repeat start finds the run and starts no second one", func(t *testing.T) {
		t.Parallel()
		f, s, id := setup(t)
		got, err := start(t, s, "k1")
		if err != nil || got.GetOperationId() != id || !got.GetExisting() || f.posts() != 1 {
			t.Errorf("repeat = %v, %v, posts %d, want the same run as existing and one POST", got, err, f.posts())
		}
	})
	t.Run("get answers QUEUED, then the build once it exists", func(t *testing.T) {
		t.Parallel()
		f, s, id := setup(t)
		got, err := get(t, s, id)
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED || got.GetRevision() != rev {
			t.Fatalf("get in the gap = %v, %v, want QUEUED at the revision started", got, err)
		}
		f.begin()
		got, err = get(t, s, id)
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_RUNNING {
			t.Errorf("get after the build began = %v, %v, want RUNNING", got, err)
		}
	})
	t.Run("a build that began but is not in the list yet is read from the item", func(t *testing.T) {
		t.Parallel()
		f, s, id := setup(t)
		f.begin()
		f.hideFromList = true
		got, err := get(t, s, id)
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_RUNNING {
			t.Errorf("get = %v, %v, want RUNNING from the item's executable", got, err)
		}
	})
	t.Run("cancel in the gap ends up stopping the build", func(t *testing.T) {
		t.Parallel()
		f, s, id := setup(t)
		f.begin()
		got, err := cancel(t, s, id)
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
			t.Errorf("cancel = %v, %v, want CANCELLED", got, err)
		}
	})
	t.Run("cancel while the item is still in the gap is not reported as cancelled", func(t *testing.T) {
		t.Parallel()
		f, s, id := setup(t)
		got, err := cancel(t, s, id)
		// The item has left the queue, so the cancel did nothing; Jenkins has not said what became of the run.
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED {
			t.Errorf("cancel = %v, %v, want QUEUED: the item is neither cancelled nor a build", got, err)
		}
		f.begin()
		got, err = cancel(t, s, id)
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
			t.Errorf("cancel once the build began = %v, %v, want CANCELLED", got, err)
		}
	})
	t.Run("an item Jenkins no longer knows falls back to not found", func(t *testing.T) {
		t.Parallel()
		f, s, id := setup(t)
		f.left = nil
		_, err := get(t, s, id)
		if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
			t.Errorf("get = %s, want NOT_FOUND", got)
		}
	})
	t.Run("a cancelled item is not found afterwards", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		if got, err := cancel(t, s, st.GetOperationId()); err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
			t.Fatalf("cancel = %v, %v", got, err)
		}
		_, err := get(t, s, st.GetOperationId())
		if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
			t.Errorf("get after a cancel = %s, want NOT_FOUND", got)
		}
	})
	t.Run("an item of another job under our queue id is not adopted", func(t *testing.T) {
		t.Parallel()
		f, s, id := setup(t)
		f.override = func(r *http.Request) *http.Response {
			if strings.HasPrefix(r.URL.EscapedPath(), "/queue/item/") {
				return answer(200, `{"id":41,"cancelled":false,"task":{"name":"other","url":"https://jenkins.example.test/job/other/"},"actions":[]}`)
			}
			return nil
		}
		_, err := get(t, s, id)
		if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
			t.Errorf("get = %s, want NOT_FOUND: the item named by our queue id is not our run", got)
		}
	})
}

func TestParseURL_HostForms(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		"https://jenkins.example.test:8443",                    // only 443
		"https://jenkins.example.test:0443",                    // a spelling of 443 that the manifest would not write
		"https://jenkins.example.test:",                        // an empty port
		"https://jenkins.example.test.",                        // a trailing dot is another name for the same host
		"https://jenkins%2eexample.test",                       // percent-encoding in the host
		"https://jenk%69ns.example.test",                       // the same, in a letter
		"https://jenkins.example.test/jenk%69ns",               // percent-encoding in the context path
		"https://jenkins.example.test/jenkins%2fx",             // an encoded slash
		"https://jenkins.example.test\\@evil.test",             // a backslash
		"https://jenkins.example.test @evil.test",              // a space
		"https://jenkins.example.test\t",                       // a control character
		"https://жenkins.example.test",                         // a non-ASCII letter
		"https://jenkins.localhost",                            // the loopback space
		"https://jenkins.local",                                // mDNS
		"https://jenkins.internal",                             // a private space
		"https://jenkins.lan",                                  // a private space
		"https://jenkins.home",                                 // a private space
		"https://jenkins.corp",                                 // a private space
		"https://jenkins.localdomain",                          // a private space
		"https://jenkins.invalid",                              // never resolves
		"https://1.2.3.4.in-addr.arpa",                         // reverse DNS space
		"https://[::1]",                                        // an IPv6 literal
		"https://[::ffff:127.0.0.1]:443",                       // an IPv4-mapped literal
		"https://jenkins.example.test:99999",                   // out of range
		"https://" + strings.Repeat("a", 64) + ".example.test", // a label over 63
		"https://" + strings.Repeat("a.", 130) + "test",        // a name over 253
		"https://-jenkins.example.test",                        // a label that starts with a hyphen
		"https://jenkins-.example.test",                        // a label that ends with a hyphen
		"https://jenkins..example.test",                        // an empty label
		"https://jenkins.example.test//jenkins",                // an empty path segment
		"https://jenkins.example.test/jenkins/./x",             // a dot segment
	} {
		if got, err := provider.ParseURL(in); err == nil {
			t.Errorf("ParseURL(%q) = %+v, want it refused", in, got)
		}
	}
	for _, in := range []string{"https://jenkins.example.test", "https://jenkins.example.test:443", "https://jenkins.example.test/jenkins", "https://ci.corp.example.com/ci/"} {
		if _, err := provider.ParseURL(in); err != nil {
			t.Errorf("ParseURL(%q) = %v, want it accepted", in, err)
		}
	}
}

func TestLive_OnlyTheRequestsTheProviderMakesLeave(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var arrived []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		arrived = append(arrived, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(proxy.Close)
	routes := newRoutes()
	if err := routes.Set("h1", proxy.Listener.Addr().String(), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	cfg, _ := provider.ParseURL("https://ci.example.test/jenkins")
	live := provider.NewLive(cfg, routes)
	send := func(method, target string) error {
		req, err := http.NewRequestWithContext(t.Context(), method, target, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(ghprovider.HandleHeader, "h1")
		resp, err := live.RoundTrip(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		return err
	}
	for _, ok := range []struct{ method, target string }{
		{http.MethodGet, "https://ci.example.test/jenkins/queue/api/json?tree=items"},
		{http.MethodGet, "https://ci.example.test/jenkins/crumbIssuer/api/json"},
		{http.MethodGet, "https://ci.example.test/jenkins/queue/item/7/api/json"},
		{http.MethodGet, "https://ci.example.test/jenkins/job/acme/job/shop/job/release/api/json"},
		{http.MethodHead, "https://ci.example.test/jenkins/job/acme/job/shop/job/release/3/artifact/out/a%20b.txt"},
		{http.MethodPost, "https://ci.example.test/jenkins/job/acme/job/shop/job/release/buildWithParameters"},
		{http.MethodPost, "https://ci.example.test/jenkins/queue/cancelItem?id=7"},
		{http.MethodPost, "https://ci.example.test/jenkins/job/acme/job/shop/job/release/3/stop"},
	} {
		if err := send(ok.method, ok.target); err != nil {
			t.Errorf("%s %s = %v, want it sent", ok.method, ok.target, err)
		}
	}
	mu.Lock()
	sent := len(arrived)
	mu.Unlock()
	if sent != 8 {
		t.Fatalf("the proxy saw %d requests, want the 8 the provider makes", sent)
	}
	for _, bad := range []struct{ name, method, target string }{
		{"a path outside the API surface", http.MethodGet, "https://ci.example.test/jenkins/script"},
		{"the script console", http.MethodPost, "https://ci.example.test/jenkins/scriptText"},
		{"a path outside the context path", http.MethodGet, "https://ci.example.test/queue/api/json"},
		{"credentials of the server", http.MethodGet, "https://ci.example.test/jenkins/credentials/store/system/api/json"},
		{"a method the provider never uses", http.MethodDelete, "https://ci.example.test/jenkins/job/a/job/b/job/c"},
		{"a put", http.MethodPut, "https://ci.example.test/jenkins/queue/cancelItem"},
		{"user information", http.MethodGet, "https://admin:x@ci.example.test/jenkins/queue/api/json"},
		{"a dot segment", http.MethodGet, "https://ci.example.test/jenkins/job/../script"},
		{"an encoded dot segment", http.MethodGet, "https://ci.example.test/jenkins/job/%2e%2e/script"},
		{"another host", http.MethodGet, "https://evil.example.test/jenkins/queue/api/json"},
		{"another port", http.MethodGet, "https://ci.example.test:8443/jenkins/queue/api/json"},
		{"plain http", http.MethodGet, "http://ci.example.test/jenkins/queue/api/json"},
	} {
		if err := send(bad.method, bad.target); err == nil {
			t.Errorf("%s: RoundTrip succeeded, want a refusal before anything is sent", bad.name)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(arrived) != sent {
		t.Errorf("the proxy saw %d requests after the refused ones, want %d: a refused request must never leave", len(arrived), sent)
	}
}
