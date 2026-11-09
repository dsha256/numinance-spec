package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

const (
	rpc     = "/numinance.ports.ciexecutor.v1.CiExecutorService/"
	rev     = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee01"
	repo    = "acme/shop"
	jobBase = "/job/acme/job/shop/job/release"
)

type host struct {
	mu       sync.Mutex
	emitted  []string
	noHandle bool
	// emitErr makes every evidence submission fail.
	emitErr error
}

func (h *host) Credential(context.Context, string, string) (string, error) {
	if h.noHandle {
		return "", errors.New("no broker")
	}
	return "handle-1", nil
}

func (h *host) Emit(_ context.Context, _, _, recordType string, _ *structpb.Struct) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.emitErr != nil {
		return "", h.emitErr
	}
	h.emitted = append(h.emitted, recordType)
	return "ev_1", nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func answer(status int, body string, hdr ...string) *http.Response {
	h := http.Header{}
	for i := 0; i+1 < len(hdr); i += 2 {
		h.Set(hdr[i], hdr[i+1])
	}
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func reason(t *testing.T, err error) pluginv1.Reason {
	t.Helper()
	var ce *provider.CallError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want a FAILED call", err)
	}
	return ce.Reason
}

// fakeBuild is one build of the fake Jenkins.
type fakeBuild struct {
	n        int
	form     url.Values
	building bool
	result   *string
	duration int
}

type fakeItem struct {
	id   int
	form url.Values
	// exec is the number of the build the item became (0: none yet); cancelled is set by a cancel.
	exec      int
	cancelled bool
}

// fakeArtifact is one artifact of a build and the size a HEAD of it answers.
type fakeArtifact struct {
	path string
	size int
}

// fake is a small stateful Jenkins: one job (acme/shop/release) that takes the contract parameters, a queue and a build list. A test
// moves a queued run to a build with run(), and sets hooks to break one answer.
type fake struct {
	mu     sync.Mutex
	queue  []fakeItem
	builds []fakeBuild
	nextID int
	// left are the items that left the queue (Jenkins keeps them for a few minutes at /queue/item/<id>/api/json).
	left []fakeItem
	// artifacts by build number; extraJobs are further buildable jobs of the repository that have no builds.
	artifacts map[int][]fakeArtifact
	extraJobs []string
	// hideFromList makes the job's list of builds empty while single builds stay readable: the list lags behind the build.
	hideFromList bool
	defs         []string
	buildable    bool
	// crumbStatus is the status of the crumb issuer; crumb is its crumb.
	crumbStatus int
	// crumbField is the header name the issuer asks for; rootURL replaces the root URL Jenkins believes it has in the task URLs.
	crumbField, rootURL string
	// prefix is the context path the fake is mounted at.
	prefix string
	crumb  string
	// override answers a request before the fake does when it returns non-nil.
	override func(*http.Request) *http.Response
	// calls is "METHOD path?query" of every request; headers the headers of each, form the body of each POST.
	calls   []string
	headers []http.Header
	forms   []url.Values
}

func newFake() *fake {
	return &fake{buildable: true, defs: []string{"numinance_revision", "numinance_operation_id", "environment"}, crumbStatus: 200, crumb: "abc123", crumbField: "Jenkins-Crumb", nextID: 40}
}

func str(s string) *string { return &s }

func (f *fake) taskURL() string {
	if f.rootURL != "" {
		return f.rootURL + jobBase + "/"
	}
	return "https://jenkins.example.test" + jobBase + "/"
}

func actions(form url.Values) []map[string]any {
	ps := make([]map[string]any, 0, len(form))
	for k := range form {
		ps = append(ps, map[string]any{"name": k, "value": form.Get(k)})
	}
	return []map[string]any{{"_class": "hudson.model.CauseAction"}, {"_class": "hudson.model.ParametersAction", "parameters": ps}}
}

func js(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// run moves the queued item to a running build.
func (f *fake) run() {
	f.mu.Lock()
	defer f.mu.Unlock()
	it := f.queue[0]
	f.queue = f.queue[1:]
	f.builds = append([]fakeBuild{{n: len(f.builds) + 1, form: it.form, building: true}}, f.builds...)
}

func (f *fake) finish(result string, duration int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builds[0].building, f.builds[0].result, f.builds[0].duration = false, str(result), duration
}

func (f *fake) buildJSON(b fakeBuild) map[string]any {
	return map[string]any{"number": b.n, "building": b.building, "result": b.result, "timestamp": 1793527200000, "duration": b.duration, "actions": actions(b.form)}
}

// extraJob reports whether the path is the api/json of one of the extra jobs.
func (f *fake) extraJob(path string) bool {
	for _, j := range f.extraJobs {
		if path == "/job/acme/job/shop/job/"+j+"/api/json" {
			return true
		}
	}
	return false
}

// buildNumber is the build number of a path <job>/<n>/api/json, or 0.
func (f *fake) buildNumber(path string) int {
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(path, jobBase+"/"), "/api/json"))
	if err != nil {
		return 0
	}
	return n
}

// leave moves the first queued item out of the queue without a build yet: the gap in which Jenkins has taken the item off the
// queue and the build is not in the job's list.
func (f *fake) leave() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.left = append(f.left, f.queue[0])
	f.queue = f.queue[1:]
}

// begin makes the build of the oldest left item appear, running.
func (f *fake) begin() {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.builds) + 1
	f.left[0].exec = n
	f.builds = append([]fakeBuild{{n: n, form: f.left[0].form, building: true}}, f.builds...)
}

func (f *fake) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
	}
	form, _ := url.ParseQuery(string(body))
	f.calls = append(f.calls, r.Method+" "+r.URL.EscapedPath()+"?"+r.URL.RawQuery)
	f.headers = append(f.headers, r.Header.Clone())
	f.forms = append(f.forms, form)
	if f.override != nil {
		if resp := f.override(r); resp != nil {
			return resp, nil
		}
	}
	path, tree := strings.TrimPrefix(r.URL.EscapedPath(), f.prefix), r.URL.Query().Get("tree")
	switch {
	case f.extraJob(path):
		if strings.HasPrefix(tree, "buildable") {
			var defs []map[string]string
			for _, d := range f.defs {
				defs = append(defs, map[string]string{"name": d})
			}
			return answer(200, js(map[string]any{"buildable": true, "property": []any{map[string]any{"parameterDefinitions": defs}}})), nil
		}
		return answer(200, js(map[string]any{"builds": []any{}})), nil
	case strings.HasPrefix(path, "/queue/item/") && strings.HasSuffix(path, "/api/json"):
		id, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(path, "/queue/item/"), "/api/json"))
		for _, it := range append(append([]fakeItem{}, f.queue...), f.left...) {
			if it.id != id {
				continue
			}
			m := map[string]any{"id": it.id, "cancelled": it.cancelled, "task": map[string]string{"name": "release", "url": f.taskURL()}, "actions": actions(it.form)}
			if it.exec > 0 {
				m["executable"] = map[string]any{"number": it.exec}
			}
			return answer(200, js(m)), nil
		}
		return answer(404, "<html>not found</html>"), nil
	case strings.HasPrefix(path, jobBase+"/") && strings.HasSuffix(path, "/api/json") && f.buildNumber(path) > 0:
		n := f.buildNumber(path)
		for _, b := range f.builds {
			if b.n != n {
				continue
			}
			if strings.HasPrefix(tree, "artifacts") {
				arts := []map[string]string{}
				for _, a := range f.artifacts[n] {
					arts = append(arts, map[string]string{"relativePath": a.path})
				}
				return answer(200, js(map[string]any{"artifacts": arts})), nil
			}
			return answer(200, js(f.buildJSON(b))), nil
		}
		return answer(404, "<html>not found</html>"), nil
	case r.Method == http.MethodHead && strings.HasPrefix(path, jobBase+"/") && strings.Contains(path, "/artifact/"):
		buildPath, artifact, _ := strings.Cut(path, "/artifact/")
		n := f.buildNumber(buildPath + "/api/json")
		rel, _ := url.PathUnescape(artifact)
		for _, a := range f.artifacts[n] {
			if a.path == rel {
				return answer(200, "", "Content-Length", strconv.Itoa(a.size)), nil
			}
		}
		return answer(404, "<html>not found</html>"), nil
	case path == "/crumbIssuer/api/json":
		if f.crumbStatus != 200 {
			return answer(f.crumbStatus, "<html>no</html>"), nil
		}
		return answer(200, js(map[string]string{"crumb": f.crumb, "crumbRequestField": f.crumbField})), nil
	case path == jobBase+"/api/json" && strings.HasPrefix(tree, "buildable"):
		var defs []map[string]string
		for _, d := range f.defs {
			defs = append(defs, map[string]string{"name": d})
		}
		return answer(200, js(map[string]any{"buildable": f.buildable, "property": []any{map[string]any{}, map[string]any{"parameterDefinitions": defs}}})), nil
	case path == "/queue/api/json":
		items := []map[string]any{}
		for _, it := range f.queue {
			items = append(items, map[string]any{"id": it.id, "task": map[string]string{"name": "release", "url": f.taskURL()}, "actions": actions(it.form)})
		}
		return answer(200, js(map[string]any{"items": items})), nil
	case path == jobBase+"/api/json" && strings.HasPrefix(tree, "builds"):
		bs := []map[string]any{}
		for _, b := range f.builds {
			if !f.hideFromList {
				bs = append(bs, f.buildJSON(b))
			}
		}
		return answer(200, js(map[string]any{"builds": bs})), nil
	case path == jobBase+"/buildWithParameters" && r.Method == http.MethodPost:
		f.nextID++
		f.queue = append(f.queue, fakeItem{id: f.nextID, form: form})
		return answer(201, "", "Location", "https://jenkins.example.test/queue/item/"+strconv.Itoa(f.nextID)+"/"), nil
	case path == "/queue/cancelItem" && r.Method == http.MethodPost:
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))
		for i, it := range f.queue {
			if it.id == id {
				it.cancelled = true
				f.left = append(f.left, it)
				f.queue = append(f.queue[:i], f.queue[i+1:]...)
				break
			}
		}
		return answer(302, "", "Location", "https://jenkins.example.test/queue/"), nil
	case strings.HasSuffix(path, "/stop") && r.Method == http.MethodPost:
		for i := range f.builds {
			if strings.HasSuffix(path, "/"+strconv.Itoa(f.builds[i].n)+"/stop") {
				f.builds[i].building, f.builds[i].result = false, str("ABORTED")
			}
		}
		return answer(302, ""), nil
	}
	return answer(404, "<html>not found</html>"), nil
}

func (f *fake) server(h *host) *provider.Server { return f.serverAt(h, provider.ExampleURL) }

func (f *fake) serverAt(h *host, raw string) *provider.Server {
	cfg, err := provider.ParseURL(raw)
	if err != nil {
		panic(err)
	}
	return provider.New(provider.PluginID, "sha256:0", cfg, provider.Transport{RT: f}, 0, h, nil)
}

func (f *fake) posts() int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, "POST ") {
			n++
		}
	}
	return n
}

func direct[Resp any](t *testing.T, s *provider.Server, method string, key string, dry bool, req proto.Message) (Resp, error) {
	t.Helper()
	var zero Resp
	resp, _, err := s.Direct(t.Context(), rpc+method, &pluginv1.ActionContext{IdempotencyKey: key, DryRun: dry}, req)
	if err != nil {
		return zero, err
	}
	out, ok := resp.(Resp)
	if !ok {
		t.Fatalf("%s answered %T", method, resp)
	}
	return out, nil
}

// roundTripWrap is the network between the provider and the fake Jenkins. lose says, for a request, whether it never reaches Jenkins
// and the error the provider sees instead of an answer (an error with false means Jenkins handled it and the answer was lost);
// replace swaps the answer.
type roundTripWrap struct {
	f       *fake
	lose    func(*http.Request) (neverArrived bool, err error)
	replace func(*http.Request) *http.Response
}

func (w *roundTripWrap) RoundTrip(r *http.Request) (*http.Response, error) {
	var lost error
	if w.lose != nil {
		var never bool
		if never, lost = w.lose(r); lost != nil && never {
			return nil, lost
		}
	}
	resp, err := w.f.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if lost != nil {
		_ = resp.Body.Close()
		return nil, lost
	}
	if w.replace != nil {
		if rep := w.replace(r); rep != nil {
			_ = resp.Body.Close()
			return rep, nil
		}
	}
	return resp, nil
}

func mustCfg(t *testing.T) provider.Config {
	t.Helper()
	cfg, err := provider.ParseURL(provider.ExampleURL)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func newRoutes() *ghprovider.Routes { return ghprovider.NewRoutes(time.Now) }
