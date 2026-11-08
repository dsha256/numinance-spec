package provider_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/replay"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

func startReq() *civ1.StartOperationRequest {
	return &civ1.StartOperationRequest{Repository: repo, Pipeline: "release", Revision: rev, Params: map[string]string{"environment": "staging"}}
}

func start(t *testing.T, s *provider.Server, key string) (*civ1.StartOperationResponse, error) {
	t.Helper()
	return direct[*civ1.StartOperationResponse](t, s, "StartOperation", key, false, startReq())
}

func get(t *testing.T, s *provider.Server, id string) (*civ1.GetOperationResponse, error) {
	t.Helper()
	return direct[*civ1.GetOperationResponse](t, s, "GetOperation", "", false, &civ1.GetOperationRequest{Repository: repo, OperationId: id})
}

func cancel(t *testing.T, s *provider.Server, id string) (*civ1.CancelOperationResponse, error) {
	t.Helper()
	return direct[*civ1.CancelOperationResponse](t, s, "CancelOperation", "k", false, &civ1.CancelOperationRequest{Repository: repo, OperationId: id})
}

func TestParseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in string
		want     provider.Config
		wantErr  bool
	}{
		{"plain host", "https://jenkins.example.test", provider.Config{Host: "jenkins.example.test", Port: 443}, false},
		{"port and context path", "https://ci.example.test:8443/jenkins/", provider.Config{Host: "ci.example.test", Port: 8443, Prefix: "/jenkins"}, false},
		{"http is refused", "http://jenkins.example.test", provider.Config{}, true},
		{"port 80 is refused", "https://jenkins.example.test:80", provider.Config{}, true},
		{"an IP literal is refused", "https://10.0.0.5", provider.Config{}, true},
		{"a user name is refused", "https://admin@jenkins.example.test", provider.Config{}, true},
		{"a user name and token is refused", "https://admin:tok@jenkins.example.test", provider.Config{}, true},
		{"a query is refused", "https://jenkins.example.test/?x=1", provider.Config{}, true},
		{"a fragment is refused", "https://jenkins.example.test/#x", provider.Config{}, true},
		{"a hex dotted address is refused", "https://0x7f.0x0.0x0.0x1", provider.Config{}, true},
		{"an octal dotted address is refused", "https://0177.0.0.01", provider.Config{}, true},
		{"a decimal address is refused", "https://2130706433", provider.Config{}, true},
		{"a short dotted address is refused", "https://127.1", provider.Config{}, true},
		{"a bare 0x label is a number and is refused", "https://0x.0x.0x.0x", provider.Config{}, true},
		{"0x labels with hex digits are refused", "https://0x7f.0x1.0x0.0x1", provider.Config{}, true},
		{"0xg is an ordinary label", "https://0xg.example.test", provider.Config{Host: "0xg.example.test", Port: 443}, false},
		{"0x1g is an ordinary label", "https://0x1g.0x1g.example.test", provider.Config{Host: "0x1g.0x1g.example.test", Port: 443}, false},
		{"an upper-case host is refused", "https://Jenkins.Example.test", provider.Config{}, true},
		{"a single label is refused", "https://jenkins", provider.Config{}, true},
		{"a wildcard is refused", "https://*.example.test", provider.Config{}, true},
		{"a path that climbs is refused", "https://jenkins.example.test/a/../b", provider.Config{}, true},
		{"empty", "", provider.Config{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := provider.ParseURL(tc.in)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("ParseURL(%q) = %+v, %v, want %+v (error %v)", tc.in, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestManifest_DeclaresOnlyTheConfiguredServer(t *testing.T) {
	t.Parallel()
	cfg, err := provider.ParseURL("https://ci.example.test:8443/jenkins")
	if err != nil {
		t.Fatal(err)
	}
	m, err := provider.New(provider.PluginID, "sha256:0", cfg, provider.Transport{RT: newFake()}, 0, &host{}, nil).GetManifest(t.Context(), &pluginv1.GetManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	caps := m.GetManifest().GetCapabilities()
	if len(caps) != 5 {
		t.Fatalf("capabilities = %d, want the five verbs of the port", len(caps))
	}
	for _, c := range caps {
		nets := c.GetNetworkIntents()
		if len(nets) != 1 || nets[0].GetHost() != "ci.example.test" || nets[0].GetPort() != 8443 {
			t.Errorf("%s network = %v, want exactly ci.example.test:8443", c.GetName(), nets)
		}
		if got := c.GetCredentialIntents(); len(got) != 1 || got[0] != provider.Intent {
			t.Errorf("%s credential intents = %v, want [%s]", c.GetName(), got, provider.Intent)
		}
		if c.GetName() == "ciexecutor.start_operation" && (c.GetRiskFloor().String() != "RISK_CLASS_HIGH" || len(c.GetEvidenceDeclared()) != 1 ||
			c.GetEvidenceDeclared()[0] != "ci-run-ref" || c.GetSideEffect() != pluginv1.SideEffect_SIDE_EFFECT_DEPLOY || c.GetIdempotency() != pluginv1.Idempotency_IDEMPOTENCY_KEYED) {
			t.Errorf("start_operation = %v, want HIGH, DEPLOY, KEYED, [ci-run-ref]", c)
		}
	}
	if m.GetManifest().GetPluginId() != "dev.numinance.ciexecutor-jenkins" {
		t.Errorf("plugin id = %q", m.GetManifest().GetPluginId())
	}
}

func TestManifest_ReplayBuildDeclaresNoNetworkAndNoCredential(t *testing.T) {
	t.Parallel()
	tr, err := replay.Transport()
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := provider.ParseURL(provider.ExampleURL)
	m, err := provider.New(provider.ReplayPluginID, "sha256:0", cfg, tr, 0, &host{}, nil).GetManifest(t.Context(), &pluginv1.GetManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.GetManifest().GetCapabilities() {
		if len(c.GetNetworkIntents()) != 0 || len(c.GetCredentialIntents()) != 0 {
			t.Errorf("%s: replay build declares network %v and credentials %v, want none", c.GetName(), c.GetNetworkIntents(), c.GetCredentialIntents())
		}
	}
}

func TestStartOperation_StartsAQueuedRunWithTheContractParameters(t *testing.T) {
	t.Parallel()
	f, h := newFake(), &host{}
	resp, err := start(t, f.server(h), "chg_0001/release/1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED || resp.GetExisting() || !strings.HasSuffix(resp.GetOperationId(), ".release") {
		t.Errorf("start = %v, want a new QUEUED run named for the pipeline", resp)
	}
	if len(f.queue) != 1 {
		t.Fatalf("queue = %d items, want 1", len(f.queue))
	}
	form := f.queue[0].form
	if form.Get("numinance_revision") != rev || form.Get("environment") != "staging" || !strings.HasPrefix(resp.GetOperationId(), form.Get("numinance_operation_id")+".") {
		t.Errorf("form = %v, want the revision, the caller's parameter and the operation digest", form)
	}
	if len(h.emitted) != 1 || h.emitted[0] != "ci-run-ref" {
		t.Errorf("evidence = %v, want exactly one ci-run-ref", h.emitted)
	}
	// Parameters travel in the body, never in the URL; the handle is the only credential material and the crumb is sent.
	for i, c := range f.calls {
		if strings.Contains(c, "staging") || strings.Contains(c, rev) {
			t.Errorf("call %q carries a parameter value in its URL", c)
		}
		if f.headers[i].Get(ghprovider.HandleHeader) != "handle-1" || f.headers[i].Get("Authorization") != "" {
			t.Errorf("call %q headers = %v, want the handle and no Authorization", c, f.headers[i])
		}
		if strings.HasSuffix(c, "buildWithParameters?") && f.headers[i].Get("Jenkins-Crumb") != "abc123" {
			t.Errorf("the build request carries crumb %q, want abc123", f.headers[i].Get("Jenkins-Crumb"))
		}
	}
}

func TestStartOperation_ARepeatUnderTheSameKeyStartsNothing(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	first, err := start(t, s, "k1")
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"queued", "running"} {
		again, err := start(t, s, "k1")
		if err != nil {
			t.Fatal(err)
		}
		if again.GetOperationId() != first.GetOperationId() || !again.GetExisting() || f.posts() != 1 {
			t.Errorf("repeat while %s = %v after %d posts, want the same run as existing and one POST", stage, again, f.posts())
		}
		if stage == "queued" {
			f.run()
		}
	}
}

func TestStartOperation_ADifferentKeyIsADifferentRun(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	a, _ := start(t, s, "k1")
	b, err := start(t, s, "k2")
	if err != nil || a.GetOperationId() == b.GetOperationId() || f.posts() != 2 {
		t.Errorf("starts = %v, %v (%v), posts %d, want two distinct runs", a, b, err, f.posts())
	}
}

func TestStartOperation_ARunOfTheKeyForAnotherCommitIsNotTheOneAsked(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	if _, err := start(t, s, "k1"); err != nil {
		t.Fatal(err)
	}
	req := startReq()
	req.Revision = strings.Repeat("a", 40)
	_, err := direct[*civ1.StartOperationResponse](t, s, "StartOperation", "k1", false, req)
	if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR || f.posts() != 1 {
		t.Errorf("start for another commit = %s, %d posts, want UPSTREAM_ERROR and no second run", got, f.posts())
	}
}

func TestStartOperation_DryRunStartsNothingAndNeverAsksForACrumb(t *testing.T) {
	t.Parallel()
	f := newFake()
	resp, err := direct[*civ1.StartOperationResponse](t, f.server(&host{}), "StartOperation", "k1", true, startReq())
	if err != nil || resp.GetOperationId() != "" || f.posts() != 0 || len(f.queue) != 0 {
		t.Errorf("dry run = %v, %v, posts %d, want no run and no POST", resp, err, f.posts())
	}
	for _, c := range f.calls {
		if strings.Contains(c, "crumbIssuer") {
			t.Errorf("a dry run asked for a crumb: %s", c)
		}
	}
}

func TestStartOperation_JobMustAcceptTheParameters(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(*fake)
		want pluginv1.Reason
	}{
		{"the job does not declare the revision parameter", func(f *fake) { f.defs = []string{"numinance_operation_id", "environment"} }, pluginv1.Reason_REASON_CONFLICT},
		{"the job does not declare the operation parameter", func(f *fake) { f.defs = []string{"numinance_revision", "environment"} }, pluginv1.Reason_REASON_CONFLICT},
		{"the job does not declare the caller's parameter", func(f *fake) { f.defs = []string{"numinance_revision", "numinance_operation_id"} }, pluginv1.Reason_REASON_CONFLICT},
		{"the job is disabled or a folder", func(f *fake) { f.buildable = false }, pluginv1.Reason_REASON_CONFLICT},
		{"the job does not exist", func(f *fake) { f.override = func(*http.Request) *http.Response { return answer(404, "<html>") } }, pluginv1.Reason_REASON_NOT_FOUND},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			tc.edit(f)
			_, err := start(t, f.server(&host{}), "k1")
			if got := reason(t, err); got != tc.want || f.posts() != 0 {
				t.Errorf("start = %s with %d posts, want %s and nothing started", got, f.posts(), tc.want)
			}
		})
	}
}

func TestStartOperation_RefusesWhatTheBoundsRefuse(t *testing.T) {
	t.Parallel()
	many := map[string]string{}
	for i := range 33 {
		many["p"+strings.Repeat("a", i)] = "v"
	}
	cases := map[string]func(*civ1.StartOperationRequest){
		"a short revision":                    func(r *civ1.StartOperationRequest) { r.Revision = "abc" },
		"a branch name for a revision":        func(r *civ1.StartOperationRequest) { r.Revision = "main" },
		"an upper case revision":              func(r *civ1.StartOperationRequest) { r.Revision = strings.ToUpper(rev) },
		"a pipeline that climbs":              func(r *civ1.StartOperationRequest) { r.Pipeline = "a/../b" },
		"a pipeline with an empty segment":    func(r *civ1.StartOperationRequest) { r.Pipeline = "a//b" },
		"a pipeline with a query":             func(r *civ1.StartOperationRequest) { r.Pipeline = "a?x=1" },
		"a pipeline with a percent escape":    func(r *civ1.StartOperationRequest) { r.Pipeline = "a%2fb" },
		"a pipeline too long for the id":      func(r *civ1.StartOperationRequest) { r.Pipeline = strings.Repeat("a", 96) },
		"a reserved parameter":                func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"numinance_operation_id": "x"} },
		"a parameter name with a dash":        func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"a-b": "x"} },
		"a parameter name with an ampersand":  func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"a&b=c": "x"} },
		"an upper case parameter name":        func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"Env": "x"} },
		"a parameter value over 1024 bytes":   func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"env": strings.Repeat("x", 1025)} },
		"a parameter value with a newline":    func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"env": "a\nb"} },
		"more than 32 parameters":             func(r *civ1.StartOperationRequest) { r.Params = many },
		"a repository that climbs":            func(r *civ1.StartOperationRequest) { r.Repository = "../x" },
		"a repository with a third segment":   func(r *civ1.StartOperationRequest) { r.Repository = "a/b/c" },
		"a repository with a dot-dot name":    func(r *civ1.StartOperationRequest) { r.Repository = "acme/.." },
		"a repository with a query character": func(r *civ1.StartOperationRequest) { r.Repository = "acme/shop?x" },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			req := startReq()
			edit(req)
			_, err := direct[*civ1.StartOperationResponse](t, f.server(&host{}), "StartOperation", "k1", false, req)
			if got := reason(t, err); got != pluginv1.Reason_REASON_INVALID_ARGUMENT || len(f.calls) != 0 {
				t.Errorf("start(%s) = %s after %d calls, want INVALID_ARGUMENT before any request", name, got, len(f.calls))
			}
		})
	}
	t.Run("no idempotency key", func(t *testing.T) {
		t.Parallel()
		_, err := start(t, newFake().server(&host{}), "")
		if got := reason(t, err); got != pluginv1.Reason_REASON_INVALID_ARGUMENT {
			t.Errorf("start without a key = %s, want INVALID_ARGUMENT", got)
		}
	})
}

func TestOperationID_IsValidatedBeforeAnyRequest(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", "abc", "0", "1 2", strings.Repeat("9", 40), strings.Repeat("a", 32) + ".", strings.Repeat("a", 32) + ".a/b",
		strings.Repeat("a", 32) + ".a..b/../c", strings.Repeat("A", 32) + ".release", strings.Repeat("a", 32) + ".a:..:c", strings.Repeat("a", 32) + ".a::c"} {
		f := newFake()
		_, err := get(t, f.server(&host{}), id)
		if got := reason(t, err); got != pluginv1.Reason_REASON_INVALID_ARGUMENT || len(f.calls) != 0 {
			t.Errorf("GetOperation(%q) = %s after %d calls, want INVALID_ARGUMENT before any request", id, got, len(f.calls))
		}
	}
}

func TestGetOperation_FollowsARunFromTheQueueToItsEnd(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	st, err := start(t, s, "k1")
	if err != nil {
		t.Fatal(err)
	}
	id := st.GetOperationId()

	got, err := get(t, s, id)
	if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED || got.GetRevision() != rev || got.GetStartedAt() != nil || got.GetFinishedAt() != nil {
		t.Errorf("queued = %v, %v, want QUEUED with the revision and no times", got, err)
	}
	f.run()
	got, err = get(t, s, id)
	if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_RUNNING || got.GetStartedAt() == nil || got.GetFinishedAt() != nil {
		t.Errorf("running = %v, %v, want RUNNING with a start and no end", got, err)
	}
	f.finish("SUCCESS", 90000)
	got, err = get(t, s, id)
	if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_SUCCEEDED || got.GetFinishedAt().AsTime().Sub(got.GetStartedAt().AsTime()) != 90*time.Second {
		t.Errorf("ended = %v, %v, want SUCCEEDED lasting 90s", got, err)
	}
}

func TestGetOperation_MapsEveryJenkinsResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		result string
		want   civ1.OperationState
	}{
		{"SUCCESS", civ1.OperationState_OPERATION_STATE_SUCCEEDED},
		{"FAILURE", civ1.OperationState_OPERATION_STATE_FAILED},
		{"UNSTABLE", civ1.OperationState_OPERATION_STATE_FAILED},
		{"NOT_BUILT", civ1.OperationState_OPERATION_STATE_FAILED},
		{"ABORTED", civ1.OperationState_OPERATION_STATE_CANCELLED},
		{"SOMETHING_NEW", civ1.OperationState_OPERATION_STATE_FAILED},
		{"success", civ1.OperationState_OPERATION_STATE_FAILED},
	}
	for _, tc := range cases {
		t.Run(tc.result, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			s := f.server(&host{})
			st, _ := start(t, s, "k1")
			f.run()
			f.finish(tc.result, 1000)
			got, err := get(t, s, st.GetOperationId())
			if err != nil || got.GetState() != tc.want {
				t.Errorf("result %q = %v, %v, want %s: an unknown or unstable ending is never a success", tc.result, got.GetState(), err, tc.want)
			}
		})
	}
}

func TestGetOperation_ARunJenkinsDoesNotShowIsNotFound(t *testing.T) {
	t.Parallel()
	_, err := get(t, newFake().server(&host{}), strings.Repeat("a", 32)+".release")
	if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
		t.Errorf("GetOperation of an unknown run = %s, want NOT_FOUND", got)
	}
}

// Jenkins builds the task URL of a queue item from its own root URL setting, which behind a reverse proxy may differ from the path of
// -url. The run is matched by its operation id parameter, so a retry under the same key still finds it and starts no second run.
func TestQueuedRun_IsFoundWhateverRootURLJenkinsBelieves(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"https://jenkins.example.test/ci", "http://internal-jenkins:8080/mounted/here", "https://other.example.test"} {
		f := newFake()
		f.rootURL = root
		s := f.server(&host{})
		first, err := start(t, s, "k1")
		if err != nil {
			t.Fatal(err)
		}
		again, err := start(t, s, "k1")
		if err != nil || !again.GetExisting() || again.GetOperationId() != first.GetOperationId() || f.posts() != 1 {
			t.Errorf("root %q: retry = %v, %v after %d posts, want the same run as existing and no second POST", root, again, err, f.posts())
		}
		got, err := get(t, s, first.GetOperationId())
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED {
			t.Errorf("root %q: GetOperation = %v, %v, want QUEUED, not NOT_FOUND", root, got, err)
		}
	}
}

func TestQueuedRun_AnItemWithAnotherOperationIdIsNotTheRun(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	a, _ := start(t, s, "k1")
	b, err := start(t, s, "k2")
	if err != nil || a.GetOperationId() == b.GetOperationId() {
		t.Fatalf("starts = %v, %v, %v", a, b, err)
	}
	// Only k2's item is left in the queue: k1's operation is not found, and is not answered with k2's run.
	f.mu.Lock()
	f.queue = f.queue[1:]
	f.mu.Unlock()
	if _, err := get(t, s, a.GetOperationId()); reason(t, err) != pluginv1.Reason_REASON_NOT_FOUND {
		t.Errorf("GetOperation of k1 = %v, want NOT_FOUND: another item is not the run", err)
	}
	if got, err := get(t, s, b.GetOperationId()); err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED {
		t.Errorf("GetOperation of k2 = %v, %v, want QUEUED", got, err)
	}
}

// withQueue makes the fake answer the global queue with its own items plus the given extra items, first.
func withQueue(f *fake, extra ...map[string]any) {
	f.override = func(r *http.Request) *http.Response {
		if r.URL.EscapedPath() != "/queue/api/json" {
			return nil
		}
		items := append([]any{}, anySlice(extra)...)
		for _, it := range f.queue {
			items = append(items, map[string]any{"id": it.id, "task": map[string]string{"name": "release", "url": f.taskURL()}, "actions": actions(it.form)})
		}
		return answer(200, js(map[string]any{"items": items}))
	}
}

func anySlice(in []map[string]any) []any {
	out := make([]any, 0, len(in))
	for _, m := range in {
		out = append(out, m)
	}
	return out
}

// foreign is a queue item of another job that carries our operation id (the id is not secret).
func foreign(id int, form url.Values, name, taskURL string) map[string]any {
	task := map[string]string{}
	if name != "" {
		task["name"] = name
	}
	if taskURL != "" {
		task["url"] = taskURL
	}
	return map[string]any{"id": id, "task": task, "actions": actions(form)}
}

func TestQueuedRun_AForeignItemWithOurOperationIdNeverShadowsTheRealRun(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	st, _ := start(t, s, "k1")
	f.run() // the real run is now a RUNNING build, the queue is empty
	// Same short name, another job: only the job path of the task URL tells them apart.
	withQueue(f, foreign(99, f.builds[0].form, "release", "https://jenkins.example.test/job/evil/job/shop/job/release/"))
	got, err := get(t, s, st.GetOperationId())
	if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_RUNNING {
		t.Errorf("GetOperation = %v, %v, want RUNNING: a foreign queued item must not shadow the real build", got, err)
	}
}

func TestQueuedRun_AForeignItemOfAnotherNameNeitherBlocksNorIsUsed(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	st, _ := start(t, s, "k1")
	realID := f.queue[0].id
	withQueue(f, foreign(99, f.queue[0].form, "other", "https://jenkins.example.test/job/evil/job/other/"))
	posts := f.posts()
	again, err := start(t, s, "k1")
	if err != nil || !again.GetExisting() || f.posts() != posts {
		t.Errorf("retry = %v, %v after %d new posts, want the real run as existing", again, err, f.posts()-posts)
	}
	if got, err := get(t, s, st.GetOperationId()); err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED {
		t.Errorf("GetOperation = %v, %v, want QUEUED", got, err)
	}
	if got, err := cancel(t, s, st.GetOperationId()); err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
		t.Errorf("cancel = %v, %v, want CANCELLED", got, err)
	}
	joined := strings.Join(f.calls, "\n")
	if !strings.Contains(joined, "cancelItem?id="+strconv.Itoa(realID)) || strings.Contains(joined, "id=99") {
		t.Errorf("calls:\n%s\nwant the cancel of the real item %d and never of the foreign item 99", joined, realID)
	}
}

// The job of a queue item is told by the END of its task URL path. A root mount path that contains /job/ is fine when it is the
// configured context path; when it is not, it can not be told from a parent folder, so the call fails closed.
func TestQueuedRun_RootMountPathsAndNestedJobs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, serverURL, prefix, root string
		wantFound                     bool
		wantErr                       bool
	}{
		{"no root mount", provider.ExampleURL, "", "https://jenkins.example.test", true, false},
		{"a root mount without /job/, not configured", provider.ExampleURL, "", "https://jenkins.example.test/ci", true, false},
		{"a root mount with /job/ inside, configured as the context path", "https://jenkins.example.test/ci/job/x", "/ci/job/x", "https://jenkins.example.test/ci/job/x", true, false},
		{"a root mount with /job/ inside, not configured, is ambiguous", provider.ExampleURL, "", "https://jenkins.example.test/ci/job/x", false, true},
		{"a parent folder before our job is ambiguous", provider.ExampleURL, "", "https://jenkins.example.test/job/other", false, true},
		{"a deeper folder nesting is ambiguous", provider.ExampleURL, "", "https://jenkins.example.test/job/x/job/y", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			f.prefix, f.rootURL = tc.prefix, tc.root
			s := f.serverAt(&host{}, tc.serverURL)
			st, err := start(t, s, "k1")
			if err != nil {
				t.Fatal(err)
			}
			posts := f.posts()
			again, err := start(t, s, "k1")
			if tc.wantErr {
				if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR || f.posts() != posts {
					t.Errorf("retry = %s with %d new posts, want UPSTREAM_ERROR and no second run", got, f.posts()-posts)
				}
				return
			}
			if err != nil || !again.GetExisting() || again.GetOperationId() != st.GetOperationId() || f.posts() != posts {
				t.Errorf("retry = %v, %v with %d new posts, want the same run as existing", again, err, f.posts()-posts)
			}
		})
	}
}

func TestQueuedRun_ADifferentJobThatEndsLikeOursIsSkipped(t *testing.T) {
	t.Parallel()
	for _, other := range []string{
		"https://jenkins.example.test/job/xacme/job/shop/job/release", // ends with /job/shop/job/release, not with ours
		"https://jenkins.example.test/job/acme/job/shop/job/release-extra",
		"https://jenkins.example.test/job/acme/job/shop/job/release/job/sub",
		"https://jenkins.example.test/job/acme/job/other/job/release",
	} {
		f := newFake()
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		f.run() // the real run is a build; the foreign item carries our operation id
		withQueue(f, foreign(99, f.builds[0].form, "release", other))
		got, err := get(t, s, st.GetOperationId())
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_RUNNING {
			t.Errorf("foreign %s: GetOperation = %v, %v, want RUNNING", other, got, err)
		}
	}
}

func TestQueuedRun_ItemsWithoutAJobAreSkipped(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	op := strings.Repeat("e", 32)
	form := url.Values{"numinance_operation_id": {op}, "numinance_revision": {rev}}
	withQueue(f, foreign(98, form, "", ""))
	if _, err := get(t, s, op+".release"); reason(t, err) != pluginv1.Reason_REASON_NOT_FOUND {
		t.Errorf("GetOperation = %v, want NOT_FOUND: an item with neither task URL nor name is not ours", err)
	}
	// With no URL the short name decides.
	withQueue(f, foreign(97, form, "release", ""))
	if got, err := get(t, s, op+".release"); err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_QUEUED {
		t.Errorf("GetOperation by task name = %v, %v, want QUEUED", got, err)
	}
	withQueue(f, foreign(96, form, "other", ""))
	if _, err := get(t, s, op+".release"); reason(t, err) != pluginv1.Reason_REASON_NOT_FOUND {
		t.Errorf("GetOperation with another task name = %v, want NOT_FOUND", err)
	}
}

func TestGetOperation_AnswersOutsideTheGrammarAreUpstreamErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a revision that is a branch":  `{"builds":[{"number":1,"building":false,"result":"SUCCESS","timestamp":1,"duration":1,"actions":[{"parameters":[{"name":"numinance_operation_id","value":"OP"},{"name":"numinance_revision","value":"main"}]}]}]}`,
		"no revision":                  `{"builds":[{"number":1,"building":false,"result":"SUCCESS","timestamp":1,"duration":1,"actions":[{"parameters":[{"name":"numinance_operation_id","value":"OP"}]}]}]}`,
		"a build number of zero":       `{"builds":[{"number":0,"building":false,"result":"SUCCESS","timestamp":1,"duration":1,"actions":[{"parameters":[{"name":"numinance_operation_id","value":"OP"},{"name":"numinance_revision","value":"` + rev + `"}]}]}]}`,
		"an ended build with no times": `{"builds":[{"number":3,"building":false,"result":"SUCCESS","timestamp":0,"duration":0,"actions":[{"parameters":[{"name":"numinance_operation_id","value":"OP"},{"name":"numinance_revision","value":"` + rev + `"}]}]}]}`,
		"not JSON":                     `<html>login</html>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			op := strings.Repeat("c", 32)
			f.override = func(r *http.Request) *http.Response {
				if strings.HasPrefix(r.URL.Query().Get("tree"), "builds") {
					return answer(200, strings.ReplaceAll(body, "OP", op))
				}
				return nil
			}
			_, err := get(t, f.server(&host{}), op+".release")
			if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
				t.Errorf("GetOperation(%s) = %s, want UPSTREAM_ERROR", name, got)
			}
		})
	}
}

func TestFailures_MapToTheReasonOfTheStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		want   pluginv1.Reason
	}{
		{"401 unauthenticated", 401, pluginv1.Reason_REASON_PERMISSION_DENIED},
		{"403 forbidden", 403, pluginv1.Reason_REASON_PERMISSION_DENIED},
		{"404 job", 404, pluginv1.Reason_REASON_NOT_FOUND},
		{"429 too many requests", 429, pluginv1.Reason_REASON_RATE_LIMITED},
		{"500", 500, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"503 getting ready", 503, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"302 to a login page", 302, pluginv1.Reason_REASON_UPSTREAM_ERROR},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			f.override = func(*http.Request) *http.Response {
				return answer(tc.status, "<html>"+strings.Repeat("x", 100)+"</html>", "Location", "https://jenkins.example.test/login")
			}
			s := f.server(&host{})
			_, err := get(t, s, strings.Repeat("a", 32)+".release")
			if got := reason(t, err); got != tc.want {
				t.Errorf("GetOperation with %d = %s, want %s", tc.status, got, tc.want)
			}
			_, err = start(t, s, "k1")
			if got := reason(t, err); got != tc.want {
				t.Errorf("StartOperation with %d = %s, want %s", tc.status, got, tc.want)
			}
		})
	}
}

func TestStartOperation_AnErrorBodyNeverReachesTheCaller(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.override = func(*http.Request) *http.Response { return answer(500, "secret-token-in-a-stack-trace") }
	_, err := start(t, f.server(&host{}), "k1")
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Errorf("error = %v, want a bare reason", err)
	}
}

func TestStartOperation_TheBuildRequestFailing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		resp *http.Response
		want pluginv1.Reason
	}{
		{"400 job takes no such parameters", answer(400, "Nothing is submitted"), pluginv1.Reason_REASON_CONFLICT},
		{"403 crumb refused", answer(403, "No valid crumb was included in the request"), pluginv1.Reason_REASON_PERMISSION_DENIED},
		{"201 without a Location", answer(201, ""), pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"200 instead of 201", answer(200, "", "Location", "https://jenkins.example.test/queue/item/5/"), pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"a Location that is not a queue item", answer(201, "", "Location", "https://jenkins.example.test/job/x/"), pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"a Location with letters in the id", answer(201, "", "Location", "https://jenkins.example.test/queue/item/abc/"), pluginv1.Reason_REASON_UPSTREAM_ERROR},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			f.override = func(r *http.Request) *http.Response {
				if r.Method == http.MethodPost {
					return tc.resp
				}
				return nil
			}
			_, err := start(t, f.server(&host{}), "k1")
			if got := reason(t, err); got != tc.want {
				t.Errorf("start = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCrumb(t *testing.T) {
	t.Parallel()
	t.Run("a server with crumbs off gets no crumb header", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		f.crumbStatus = 404
		if _, err := start(t, f.server(&host{}), "k1"); err != nil {
			t.Fatal(err)
		}
		for i, c := range f.calls {
			if strings.HasPrefix(c, "POST") && f.headers[i].Get("Jenkins-Crumb") != "" {
				t.Errorf("the build request carries a crumb %q although the issuer answered 404", f.headers[i].Get("Jenkins-Crumb"))
			}
		}
	})
	for name, tc := range map[string]struct {
		status int
		crumb  string
		want   pluginv1.Reason
	}{
		"the issuer answers 500":            {500, "", pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		"the issuer refuses the credential": {403, "", pluginv1.Reason_REASON_PERMISSION_DENIED},
		"a crumb with a line break":         {200, "abc\r\nX-Evil: 1", pluginv1.Reason_REASON_UPSTREAM_ERROR},
		"an empty crumb":                    {200, "", pluginv1.Reason_REASON_UPSTREAM_ERROR},
		"a crumb over its bound":            {200, strings.Repeat("a", 129), pluginv1.Reason_REASON_UPSTREAM_ERROR},
		"a crumb with a space":              {200, "ab cd", pluginv1.Reason_REASON_UPSTREAM_ERROR},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			f.crumbStatus, f.crumb = tc.status, tc.crumb
			_, err := start(t, f.server(&host{}), "k1")
			if got := reason(t, err); got != tc.want || f.posts() != 0 {
				t.Errorf("start = %s with %d posts, want %s and no build request", got, f.posts(), tc.want)
			}
		})
	}
}

func TestCrumb_FieldNameCannotNameAReservedHeader(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"Authorization", "Proxy-Authorization", "Content-Type", "Content-Length", "Cookie", "Host", "Numinance-Handle",
		"X-Forwarded-For", "Transfer-Encoding", "Accept", "Authorization-Crumb", "Content-Crumb", "Host-Crumb", "Numinance-Crumb", "X-Forwarded-Crumb",
		"Jenkins-Crumb\r\nX: y", "X-Crumb-\u212a", "X-Crumb-\u017f", "", "crumb header", strings.Repeat("a", 70) + "-Crumb"} {
		f := newFake()
		f.crumbField = field
		_, err := start(t, f.server(&host{}), "k1")
		if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR || f.posts() != 0 {
			t.Errorf("crumb field %q = %s with %d posts, want UPSTREAM_ERROR and no build request", field, got, f.posts())
		}
	}
	for _, field := range []string{"Jenkins-Crumb", "X-CSRF-Crumb", "crumb", ".crumb"[1:]} {
		f := newFake()
		f.crumbField = field
		if _, err := start(t, f.server(&host{}), "k1"); err != nil {
			t.Errorf("crumb field %q refused: %v", field, err)
		}
	}
}

func TestStartOperation_DryRunOverAnExistingRunEmitsNoReceipt(t *testing.T) {
	t.Parallel()
	f, h := newFake(), &host{}
	s := f.server(h)
	if _, err := start(t, s, "k1"); err != nil {
		t.Fatal(err)
	}
	h.emitted = nil
	resp, err := direct[*civ1.StartOperationResponse](t, s, "StartOperation", "k1", true, startReq())
	if err != nil || resp.GetOperationId() != "" || len(h.emitted) != 0 || f.posts() != 1 {
		t.Errorf("dry run over an existing run = %v, %v, evidence %v, posts %d, want no id, no receipt, no POST", resp, err, h.emitted, f.posts())
	}
}

func TestCancelOperation_ARunThatBecameABuildIsNeverReportedCancelledByDefault(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	st, _ := start(t, s, "k1")
	// The cancel of the queue item is accepted, but the item had already become a build that then vanishes from the list: nothing
	// shows the run any more, which is an upstream error and not a cancel Jenkins confirmed.
	f.override = func(r *http.Request) *http.Response {
		joined := strings.Join(f.calls, " ")
		switch {
		case r.URL.EscapedPath() == "/queue/cancelItem":
			it := f.queue[0] // the fake holds its lock here: move the item to a running build by hand
			f.queue = f.queue[1:]
			f.builds = append([]fakeBuild{{n: 1, form: it.form, building: true}}, f.builds...)
			return answer(302, "")
		case strings.HasPrefix(r.URL.Query().Get("tree"), "builds") && strings.Contains(joined, "/stop"):
			return answer(200, `{"builds":[]}`)
		case strings.HasSuffix(r.URL.EscapedPath(), "/stop"):
			return answer(302, "") // accepted, and the run is gone from the list afterwards
		}
		return nil
	}
	got, err := cancel(t, s, st.GetOperationId())
	if err == nil {
		t.Errorf("cancel = %v, want an error: the run became a build and then disappeared, which Jenkins did not confirm as a cancel", got)
	}
}

func TestCancelOperation(t *testing.T) {
	t.Parallel()
	t.Run("a queued run is removed from the queue", func(t *testing.T) {
		t.Parallel()
		f, h := newFake(), &host{}
		s := f.server(h)
		st, _ := start(t, s, "k1")
		got, err := cancel(t, s, st.GetOperationId())
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED || len(f.queue) != 0 {
			t.Errorf("cancel = %v, %v, queue %d, want CANCELLED and an empty queue", got, err, len(f.queue))
		}
		if len(h.emitted) != 2 || h.emitted[1] != "ci-action" {
			t.Errorf("evidence = %v, want the start receipt and one ci-action", h.emitted)
		}
	})
	t.Run("a running build is stopped", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		f.run()
		got, err := cancel(t, s, st.GetOperationId())
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
			t.Errorf("cancel = %v, %v, want CANCELLED", got, err)
		}
		again, err := cancel(t, s, st.GetOperationId())
		if err != nil || again.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
			t.Errorf("a repeat = %v, %v, want CANCELLED", again, err)
		}
		stops := 0
		for _, c := range f.calls {
			if strings.HasSuffix(strings.SplitN(c, "?", 2)[0], "/stop") {
				stops++
			}
		}
		if stops != 1 {
			t.Errorf("stop requests = %d, want 1: a repeat on an ended run changes nothing", stops)
		}
	})
	t.Run("an ended run is answered as it is and nothing is posted", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		f.run()
		f.finish("FAILURE", 10)
		before := f.posts()
		got, err := cancel(t, s, st.GetOperationId())
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_FAILED || f.posts() != before {
			t.Errorf("cancel of an ended run = %v, %v, posts %d->%d, want FAILED and no POST", got, err, before, f.posts())
		}
	})
	t.Run("a build that does not stop yet is answered as it is, not as cancelled", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		f.run()
		f.override = func(r *http.Request) *http.Response {
			if strings.HasSuffix(r.URL.EscapedPath(), "/stop") {
				return answer(302, "") // accepted, but the build keeps running
			}
			return nil
		}
		got, err := cancel(t, s, st.GetOperationId())
		if err != nil || got.GetState() != civ1.OperationState_OPERATION_STATE_RUNNING {
			t.Errorf("cancel = %v, %v, want RUNNING: Jenkins did not report an end", got, err)
		}
	})
	t.Run("the stop is refused", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		f.run()
		f.override = func(r *http.Request) *http.Response {
			if strings.HasSuffix(r.URL.EscapedPath(), "/stop") {
				return answer(403, "")
			}
			return nil
		}
		_, err := cancel(t, s, st.GetOperationId())
		if got := reason(t, err); got != pluginv1.Reason_REASON_PERMISSION_DENIED {
			t.Errorf("cancel = %s, want PERMISSION_DENIED", got)
		}
	})
	t.Run("an unknown run is not found", func(t *testing.T) {
		t.Parallel()
		_, err := cancel(t, newFake().server(&host{}), strings.Repeat("b", 32)+".release")
		if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
			t.Errorf("cancel = %s, want NOT_FOUND", got)
		}
	})
}

func TestListArtifacts(t *testing.T) {
	t.Parallel()
	artifactsFor := func(list string, sizes map[string]string) *fake {
		f := newFake()
		f.override = func(r *http.Request) *http.Response {
			p := r.URL.EscapedPath()
			switch {
			case strings.HasSuffix(p, "/1/api/json"):
				return answer(200, list)
			case r.Method == http.MethodHead && strings.Contains(p, "/1/artifact/"):
				name := strings.SplitN(p, "/1/artifact/", 2)[1]
				if n, ok := sizes[name]; ok {
					return answer(200, "", "Content-Length", n)
				}
				return answer(200, "")
			}
			return nil
		}
		return f
	}
	run := func(f *fake) (*civ1.ListArtifactsResponse, error) {
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		f.run()
		f.finish("SUCCESS", 5)
		return direct[*civ1.ListArtifactsResponse](t, s, "ListArtifacts", "", false, &civ1.ListArtifactsRequest{Repository: repo, OperationId: st.GetOperationId()})
	}
	t.Run("names, sizes from Content-Length, no digest, sorted", func(t *testing.T) {
		t.Parallel()
		resp, err := run(artifactsFor(`{"artifacts":[{"relativePath":"z/b.txt"},{"relativePath":"a.tar.gz"}]}`, map[string]string{"z/b.txt": "7", "a.tar.gz": "2048"}))
		if err != nil || len(resp.GetArtifacts()) != 2 || resp.GetArtifacts()[0].GetName() != "a.tar.gz" || resp.GetArtifacts()[0].GetSizeBytes() != 2048 ||
			resp.GetArtifacts()[1].GetSizeBytes() != 7 || resp.GetArtifacts()[0].GetDigest() != "" {
			t.Errorf("artifacts = %v, %v", resp, err)
		}
	})
	t.Run("a size Jenkins does not give is an error, never a guess", func(t *testing.T) {
		t.Parallel()
		_, err := run(artifactsFor(`{"artifacts":[{"relativePath":"a.tar.gz"}]}`, nil))
		if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
			t.Errorf("ListArtifacts without a length = %s, want UPSTREAM_ERROR", got)
		}
	})
	for name, list := range map[string]string{
		"a path that climbs":    `{"artifacts":[{"relativePath":"../etc/passwd"}]}`,
		"an absolute path":      `{"artifacts":[{"relativePath":"/etc/passwd"}]}`,
		"a backslash":           `{"artifacts":[{"relativePath":"a\\b"}]}`,
		"an empty name":         `{"artifacts":[{"relativePath":""}]}`,
		"a repeated name":       `{"artifacts":[{"relativePath":"a"},{"relativePath":"a"}]}`,
		"a control character":   `{"artifacts":[{"relativePath":"a\u0007b"}]}`,
		"a name over 256 bytes": `{"artifacts":[{"relativePath":"` + strings.Repeat("a", 257) + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := run(artifactsFor(list, map[string]string{"a": "1"}))
			if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
				t.Errorf("ListArtifacts(%s) = %s, want UPSTREAM_ERROR", name, got)
			}
		})
	}
	t.Run("more than 256 artifacts", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		b.WriteString(`{"artifacts":[`)
		for i := range 257 {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"relativePath":"f` + strings.Repeat("a", i%10) + string(rune('a'+i%26)) + `"}`)
		}
		b.WriteString("]}")
		_, err := run(artifactsFor(b.String(), nil))
		if got := reason(t, err); got != pluginv1.Reason_REASON_LIMIT_EXCEEDED {
			t.Errorf("ListArtifacts of 257 = %s, want LIMIT_EXCEEDED", got)
		}
	})
	t.Run("a queued run has none", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		s := f.server(&host{})
		st, _ := start(t, s, "k1")
		resp, err := direct[*civ1.ListArtifactsResponse](t, s, "ListArtifacts", "", false, &civ1.ListArtifactsRequest{Repository: repo, OperationId: st.GetOperationId()})
		if err != nil || len(resp.GetArtifacts()) != 0 {
			t.Errorf("artifacts of a queued run = %v, %v, want none", resp, err)
		}
	})
}

func TestGetLogReference(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := f.server(&host{})
	st, _ := start(t, s, "k1")
	req := &civ1.GetLogReferenceRequest{Repository: repo, OperationId: st.GetOperationId()}
	_, err := direct[*civ1.GetLogReferenceResponse](t, s, "GetLogReference", "", false, req)
	if got := reason(t, err); got != pluginv1.Reason_REASON_CONFLICT {
		t.Errorf("log of a queued run = %s, want CONFLICT: there is no log yet", got)
	}
	f.run()
	resp, err := direct[*civ1.GetLogReferenceResponse](t, s, "GetLogReference", "", false, req)
	if err != nil || resp.GetUrl() != "https://jenkins.example.test"+jobBase+"/1/console" {
		t.Errorf("log reference = %v, %v, want the console of build 1 on the configured server", resp, err)
	}
}

func TestResponses_OverTheLimitAreRefusedAndNeverDecoded(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.override = func(*http.Request) *http.Response {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(strings.NewReader(`{"items":[`), newZeros(ghprovider.MaxResponseBytes+10)))}
	}
	_, err := get(t, f.server(&host{}), strings.Repeat("a", 32)+".release")
	if got := reason(t, err); got != pluginv1.Reason_REASON_LIMIT_EXCEEDED {
		t.Errorf("GetOperation with an oversized answer = %s, want LIMIT_EXCEEDED", got)
	}
}

type zeros int

func newZeros(n int) *zeros { z := zeros(n); return &z }

func (z *zeros) Read(p []byte) (int, error) {
	if *z <= 0 {
		return 0, io.EOF
	}
	n := min(len(p), int(*z))
	for i := range n {
		p[i] = ' '
	}
	*z -= zeros(n)
	return n, nil
}

func TestGetOperation_AFullWindowOfBuildsFailsClosed(t *testing.T) {
	t.Parallel()
	f := newFake()
	var b strings.Builder
	b.WriteString(`{"builds":[`)
	for i := range 100 {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"number":` + string(rune('1'+i%9)) + `,"building":true,"result":null,"timestamp":1,"duration":0,"actions":[]}`)
	}
	b.WriteString("]}")
	f.override = func(r *http.Request) *http.Response {
		if strings.HasPrefix(r.URL.Query().Get("tree"), "builds") {
			return answer(200, b.String())
		}
		return nil
	}
	s := f.server(&host{})
	_, err := get(t, s, strings.Repeat("a", 32)+".release")
	if got := reason(t, err); got != pluginv1.Reason_REASON_LIMIT_EXCEEDED {
		t.Errorf("GetOperation = %s, want LIMIT_EXCEEDED: the run may be further back", got)
	}
	_, err = start(t, s, "k1")
	if got := reason(t, err); got != pluginv1.Reason_REASON_LIMIT_EXCEEDED || f.posts() != 0 {
		t.Errorf("StartOperation = %s with %d posts, want LIMIT_EXCEEDED and no second run", got, f.posts())
	}
}

func TestTimeoutsAndOutages(t *testing.T) {
	t.Parallel()
	cfg, _ := provider.ParseURL(provider.ExampleURL)
	t.Run("a server that never answers ends with the caller's deadline", func(t *testing.T) {
		t.Parallel()
		rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})
		s := provider.New(provider.PluginID, "sha256:0", cfg, provider.Transport{RT: rt}, 0, &host{}, nil)
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		_, _, err := s.Direct(ctx, rpc+"GetOperation", &pluginv1.ActionContext{}, &civ1.GetOperationRequest{Repository: repo, OperationId: strings.Repeat("a", 32) + ".release"})
		if got := reason(t, err); got != pluginv1.Reason_REASON_TIMEOUT {
			t.Errorf("GetOperation against a silent server = %s, want TIMEOUT", got)
		}
	})
	t.Run("a connection error is an outage, not an empty answer", func(t *testing.T) {
		t.Parallel()
		rt := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("connection refused") })
		s := provider.New(provider.PluginID, "sha256:0", cfg, provider.Transport{RT: rt}, 0, &host{}, nil)
		_, err := get(t, s, strings.Repeat("a", 32)+".release")
		if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
			t.Errorf("GetOperation = %s, want UPSTREAM_UNAVAILABLE", got)
		}
	})
	t.Run("no credential handle means no request", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		_, err := get(t, f.server(&host{noHandle: true}), strings.Repeat("a", 32)+".release")
		if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE || len(f.calls) != 0 {
			t.Errorf("GetOperation = %s after %d calls, want UPSTREAM_UNAVAILABLE and nothing sent", got, len(f.calls))
		}
	})
}

func TestLive_OnlyTheConfiguredServerWithAHandle(t *testing.T) {
	t.Parallel()
	cfg, _ := provider.ParseURL("https://ci.example.test:8443")
	live := provider.NewLive(cfg, ghprovider.NewRoutes(time.Now))
	cases := []struct {
		name, target, handle string
	}{
		{"no handle", "https://ci.example.test:8443/queue/api/json", ""},
		{"a handle the host gave no proxy for", "https://ci.example.test:8443/queue/api/json", "unknown"},
		{"another host", "https://evil.example.test:8443/queue/api/json", "unknown"},
		{"the right host on the default port", "https://ci.example.test/queue/api/json", "unknown"},
		{"plain http", "http://ci.example.test:8443/queue/api/json", "unknown"},
		{"the metadata address", "https://169.254.169.254/latest/meta-data", "unknown"},
	}
	for _, tc := range cases {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if tc.handle != "" {
			req.Header.Set(ghprovider.HandleHeader, tc.handle)
		}
		if resp, err := live.RoundTrip(req); err == nil {
			_ = resp.Body.Close()
			t.Errorf("%s: RoundTrip succeeded, want a refusal and nothing sent", tc.name)
		}
	}
}

func TestReplay_NoRecordingFailsClosed(t *testing.T) {
	t.Parallel()
	tr, err := replay.Transport()
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := provider.ParseURL(provider.ExampleURL)
	s := provider.New(provider.ReplayPluginID, "sha256:0", cfg, tr, 0, &host{}, nil)
	_, err = get(t, s, strings.Repeat("d", 32)+".nothing")
	if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
		t.Errorf("GetOperation of an unrecorded job = %s, want UPSTREAM_UNAVAILABLE", got)
	}
}

func TestReplay_CassetteRefusesWhatARecordingMustNotHold(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"an authorization header": `{"redaction":"x","interactions":[{"method":"GET","path":"/a","status":200,"headers":{"Authorization":"Basic x"}}]}`,
		"a Set-Cookie header":     `{"redaction":"x","interactions":[{"method":"GET","path":"/a","status":200,"headers":{"Set-Cookie":"JSESSIONID=x"}}]}`,
		"no redaction note":       `{"interactions":[]}`,
		"an unknown field":        `{"redaction":"x","interactions":[],"token":"x"}`,
		"a status out of range":   `{"redaction":"x","interactions":[{"method":"GET","path":"/a","status":99}]}`,
		"two values in one file":  `{"redaction":"x","interactions":[]} {}`,
	}
	for name, in := range cases {
		if _, err := replay.ParseCassette([]byte(in)); err == nil {
			t.Errorf("ParseCassette(%s) succeeded, want a refusal", name)
		}
	}
}

func TestReplay_CassetteHoldsNoRealHostOrSecret(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../replay/cassettes/ci.json")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, bad := range []string{"Authorization", "Cookie", "Basic ", "Bearer ", "ghp_", "BEGIN PRIVATE", "apiToken", "password", "@"} {
		if strings.Contains(text, bad) {
			t.Errorf("the cassette contains %q, which a redacted recording must not", bad)
		}
	}
	for _, h := range regexp.MustCompile(`https://[^/"\s]+`).FindAllString(text, -1) {
		if h != "https://jenkins.example.test" {
			t.Errorf("the cassette names host %q, want only the made-up jenkins.example.test", h)
		}
	}
}
