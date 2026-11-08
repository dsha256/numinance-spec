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

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
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
}

// fake is a small stateful Jenkins: one job (acme/shop/release) that takes the contract parameters, a queue and a build list. A test
// moves a queued run to a build with run(), and sets hooks to break one answer.
type fake struct {
	mu        sync.Mutex
	queue     []fakeItem
	builds    []fakeBuild
	nextID    int
	defs      []string
	buildable bool
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
			bs = append(bs, map[string]any{"number": b.n, "building": b.building, "result": b.result, "timestamp": 1793527200000, "duration": b.duration, "actions": actions(b.form)})
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
				f.queue = append(f.queue[:i], f.queue[i+1:]...)
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
