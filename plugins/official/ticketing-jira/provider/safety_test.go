package provider_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/replay"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func answer(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

// counting wraps a transport function and records the requests it saw.
type counting struct {
	mu   sync.Mutex
	seen []*http.Request
	fn   func(*http.Request) (*http.Response, error)
}

func (c *counting) RoundTrip(r *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.seen = append(c.seen, r)
	c.mu.Unlock()
	return c.fn(r)
}

func (c *counting) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.seen)
}

func over(t *testing.T, rt http.RoundTripper) *provider.Server {
	t.Helper()
	srv, err := provider.New(provider.ReplayPluginID, replay.Host, "sha256:0", provider.Transport{RT: rt, Replay: true}, &recordingHost{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func run(srv *provider.Server, method string, o ticketingkit.Options, req proto.Message) (proto.Message, error) {
	resp, _, err := srv.Direct(context.Background(), rpc+method, &pluginv1.ActionContext{DryRun: o.DryRun, IdempotencyKey: o.Key}, req)
	return resp, err
}

func failed(t *testing.T, err error) *provider.CallError {
	t.Helper()
	var ce *provider.CallError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want a FAILED call", err)
	}
	return ce
}

func getReq(project, key string) *ticketingv1.GetIssueRequest {
	return &ticketingv1.GetIssueRequest{Project: project, IssueKey: key, MaxBodyBytes: 4096}
}

var issueKeyShape = regexp.MustCompile(`/issue/[^/?]+`)

// shape is a request without the names the caller chose: the method, the path with the issue key replaced, the query parameter
// names. Two calls that take the same path through the provider before they decide have the same shapes.
func shape(r sent) string {
	u, _ := url.Parse(r.URL)
	names := make([]string, 0, len(u.Query()))
	for k := range u.Query() {
		names = append(names, k)
	}
	slices.Sort(names)
	return r.Method + " " + issueKeyShape.ReplaceAllString(u.Path, "/issue/K") + " ?" + strings.Join(names, ",")
}

func shapesOf(reqs []sent) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, shape(r))
	}
	return out
}

// Timing parity (kit README, "Known gaps"). A target cannot show timing, so the provider's obligation is checked where it can
// be seen: every case of not found (missing, hidden, moved, another project, a forbidden project) makes exactly the requests
// that an issue which exists makes before the provider decides, in the same order and shape. Nothing branches on existence
// before the answer.
func TestTiming_NotFoundMakesTheSameRequestsAsFound(t *testing.T) {
	s := subject(nil)
	const (
		done = ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE
		open = ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN
		blk  = ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS
	)
	get := func(project, key string) func(*target) {
		return func(tg *target) { _, _ = tg.GetIssue(t.Context(), ticketingkit.Options{}, getReq(project, key)) }
	}
	dry := func(project, key string) func(*target) {
		return func(tg *target) {
			_, _, _ = tg.TransitionIssue(t.Context(), ticketingkit.Options{DryRun: true}, &ticketingv1.TransitionIssueRequest{Project: project, IssueKey: key, Target: done})
		}
	}
	trans := func(project, key string) func(*target) {
		return func(tg *target) {
			_, _, _ = tg.TransitionIssue(t.Context(), ticketingkit.Options{}, &ticketingv1.TransitionIssueRequest{Project: project, IssueKey: key, Target: open})
		}
	}
	comment := func(project, key string) func(*target) {
		return func(tg *target) {
			_, _, _ = tg.CommentOnIssue(t.Context(), ticketingkit.Options{Key: "kit-nf"}, &ticketingv1.CommentOnIssueRequest{Project: project, IssueKey: key, Body: "x"})
		}
	}
	link := func(p1, k1, p2, k2 string) func(*target) {
		return func(tg *target) {
			_, _, _ = tg.LinkIssue(t.Context(), ticketingkit.Options{Key: "k"}, &ticketingv1.LinkIssueRequest{Project: p1, IssueKey: k1, BlockedProject: p2, BlockedIssueKey: k2, LinkType: blk})
		}
	}
	tests := []struct {
		verb  string
		found func(*target)
		// n is the number of requests made before the provider decides.
		n      int
		others map[string]func(*target)
	}{
		{"GetIssue", get("PAY", s.Found), 1, map[string]func(*target){
			"missing": get("PAY", s.Missing), "hidden": get("PAY", s.Hidden), "moved": get("PAY", s.Moved),
			"another project": get("OPS", s.Found), "forbidden project": get("SEC", s.ForbiddenIssue),
		}},
		{"TransitionIssue dry run", dry("PAY", s.TransitionStuck), 1, map[string]func(*target){
			"missing": dry("PAY", s.Missing), "hidden": dry("PAY", s.Hidden), "moved": dry("PAY", s.Moved),
			"another project": dry("OPS", s.Found), "forbidden project": dry("SEC", s.ForbiddenIssue),
		}},
		{"TransitionIssue", trans("PAY", s.TransitionOpen), 1, map[string]func(*target){
			"missing": trans("PAY", s.Missing), "hidden": trans("PAY", s.Hidden), "moved": trans("PAY", s.Moved),
			"another project": trans("OPS", s.Found), "forbidden project": trans("SEC", s.ForbiddenIssue),
		}},
		{"CommentOnIssue", comment("PAY", s.Found), 1, map[string]func(*target){
			"missing": comment("PAY", s.Missing), "hidden": comment("PAY", s.Hidden), "moved": comment("PAY", s.Moved),
			"another project": comment("OPS", s.Found), "forbidden project": comment("SEC", s.ForbiddenIssue),
		}},
		{"LinkIssue", link("PAY", s.LinkBlocker, "PAY", s.LinkBlocked), 2, map[string]func(*target){
			"missing blocked": link("PAY", s.LinkBlocker, "PAY", s.Missing), "hidden blocker": link("PAY", s.Hidden, "PAY", s.LinkBlocked),
			"forbidden blocked": link("PAY", s.LinkBlocker, "SEC", s.ForbiddenIssue), "forbidden blocker": link("SEC", s.ForbiddenIssue, "PAY", s.LinkBlocked),
			"moved blocked": link("PAY", s.LinkBlocker, "PAY", s.Moved), "blocked in another project": link("PAY", s.LinkBlocker, "OPS", s.LinkBlocked),
		}},
	}
	shapes := func(call func(*target)) []string {
		tg := newTarget(t)
		call(tg)
		return shapesOf(tg.tap.requests())
	}
	for _, tc := range tests {
		t.Run(tc.verb, func(t *testing.T) {
			want := shapes(tc.found)
			if len(want) < tc.n {
				t.Fatalf("the call on an existing issue made %d requests %v, want at least %d", len(want), want, tc.n)
			}
			want = want[:tc.n]
			for name, call := range tc.others {
				if got := shapes(call); !slices.Equal(got, want) {
					t.Errorf("%s made the requests %v, want %v: the provider branched on existence before it answered", name, got, want)
				}
			}
		})
	}
}

// Redirects. The provider never follows one: the hop would carry whatever the egress proxy injects to a host the manifest never
// named. The 3xx is an unexpected answer, and no second request is made.
func TestRedirect_IsNeverFollowed(t *testing.T) {
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		rt := &counting{fn: func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != replay.Host {
				t.Errorf("a request went to %q, a host the instance did not name", r.URL.Host)
			}
			resp := answer(status, "")
			resp.Header.Set("Location", "https://other.example.test/rest/api/3/issue/PAY-1")
			return resp, nil
		}}
		_, err := run(over(t, rt), "GetIssue", ticketingkit.Options{}, getReq("PAY", "PAY-1"))
		if got := failed(t, err).Reason; got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
			t.Errorf("GetIssue after a %d = %s, want UPSTREAM_ERROR", status, got)
		}
		if n := rt.count(); n != 1 {
			t.Errorf("a %d redirect led to %d requests, want 1: it was followed", status, n)
		}
	}
}

const hostileIssue = `{"key":"PAY-1","self":"https://other.example.test/self","expand":"renderedFields,names","fields":{
 "summary":"Title","project":{"key":"PAY","self":"https://other.example.test/project"},
 "status":{"name":"To Do","self":"https://other.example.test/status","statusCategory":{"key":"new"}},
 "description":{"type":"doc","version":1,"content":[
  {"type":"paragraph","content":[{"type":"text","text":"see","marks":[{"type":"link","attrs":{"href":"https://other.example.test/href"}}]}]},
  {"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"x","type":"external","url":"https://other.example.test/pixel.png"}}]},
  {"type":"paragraph","content":[{"type":"inlineCard","attrs":{"url":"https://other.example.test/card"}},{"type":"mention","attrs":{"id":"u1","text":"@Dana"}}]}]}}}`

// Ticket-supplied URLs. A media link, a self link, a pagination link or an inline card in an answer is data: the provider makes
// exactly the request it was asked for, to the instance host, and no other.
func TestURLsInAnswers_AreNeverFetched(t *testing.T) {
	rt := &counting{fn: func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != replay.Host {
			t.Errorf("a request went to %q, a host that came from an answer", r.URL.Host)
		}
		if strings.HasSuffix(r.URL.Path, "/search/jql") {
			return answer(200, `{"issues":[],"isLast":false,"nextPageToken":"t","nextPage":"https://other.example.test/page2","self":"https://other.example.test/s"}`), nil
		}
		return answer(200, hostileIssue), nil
	}}
	srv := over(t, rt)
	resp, err := run(srv, "GetIssue", ticketingkit.Options{}, getReq("PAY", "PAY-1"))
	if err != nil {
		t.Fatal(err)
	}
	body := resp.(*ticketingv1.GetIssueResponse).GetBody() //nolint:forcetypeassert // the method's own response type
	if strings.Contains(body, "other.example.test") || !strings.Contains(body, "[attachment]") || !strings.Contains(body, "[card]") || !strings.Contains(body, "@Dana") {
		t.Errorf("body = %q, want placeholders for the media and the card, the mention text, and no URL", body)
	}
	sr, err := run(srv, "SearchIssues", ticketingkit.Options{}, &ticketingv1.SearchIssuesRequest{Project: "PAY", Text: "x", MaxResults: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !sr.(*ticketingv1.SearchIssuesResponse).GetTruncated() { //nolint:forcetypeassert // the method's own response type
		t.Error("a search whose answer says there is more answered truncated=false")
	}
	if n := rt.count(); n != 2 {
		t.Errorf("two calls made %d requests, want 2: a URL of an answer was followed", n)
	}
}

// The error classes the kit cannot provoke: each ends with its reason and a fixed message that the kit's own check accepts.
func TestErrorClasses_AreFixedMessagesOfTheClosedVocabulary(t *testing.T) {
	secrets := append([]string{"PAY", "PAY-1"}, trackerTexts...)
	big := `{"key":"PAY-1","pad":"` + strings.Repeat("x", provider.MaxResponseBytes) + `"}`
	good := `{"key":"PAY-1","fields":{"summary":"T","project":{"key":"PAY"},"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`
	tests := []struct {
		name string
		rt   rtFunc
		want pluginv1.Reason
	}{
		{"rate limit", func(*http.Request) (*http.Response, error) { return answer(429, "{}"), nil }, pluginv1.Reason_REASON_RATE_LIMITED},
		{"outage 503", func(*http.Request) (*http.Response, error) { return answer(503, "{}"), nil }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"server error 500", func(*http.Request) (*http.Response, error) { return answer(500, "boom PAY-1"), nil }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"timeout status 408", func(*http.Request) (*http.Response, error) { return answer(408, "{}"), nil }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"connection refused", func(*http.Request) (*http.Response, error) { return nil, errors.New("dial PAY-1 refused") }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"bad credential", func(*http.Request) (*http.Response, error) { return answer(401, "{}"), nil }, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"not json", func(*http.Request) (*http.Response, error) { return answer(200, "<html>PAY-1</html>"), nil }, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"over 4 MiB", func(*http.Request) (*http.Response, error) { return answer(200, big), nil }, pluginv1.Reason_REASON_LIMIT_EXCEEDED},
		{"empty title", func(*http.Request) (*http.Response, error) {
			return answer(200, strings.Replace(good, `"summary":"T"`, `"summary":"\u200b"`, 1)), nil
		}, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"unknown status category", func(*http.Request) (*http.Response, error) {
			return answer(200, strings.Replace(good, `"new"`, `"mystery"`, 1)), nil
		}, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"no project", func(*http.Request) (*http.Response, error) {
			return answer(200, `{"key":"PAY-1","fields":{"summary":"T"}}`), nil
		}, pluginv1.Reason_REASON_NOT_FOUND},
		{"a renamed key", func(*http.Request) (*http.Response, error) {
			return answer(200, strings.Replace(good, `"key":"PAY-1"`, `"key":"PAY-2"`, 1)), nil
		}, pluginv1.Reason_REASON_NOT_FOUND},
		{"forbidden read", func(*http.Request) (*http.Response, error) { return answer(403, "{}"), nil }, pluginv1.Reason_REASON_NOT_FOUND},
		{"deadline", func(r *http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded }, pluginv1.Reason_REASON_TIMEOUT},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := run(over(t, tc.rt), "GetIssue", ticketingkit.Options{}, getReq("PAY", "PAY-1"))
			ce := failed(t, err)
			if ce.Reason != tc.want {
				t.Errorf("GetIssue = %s, want %s", ce.Reason, tc.want)
			}
			if e := ticketingkit.ValidateErrorMessage(&ticketingkit.CallError{Reason: ce.Reason, Message: ce.Message}, secrets...); e != nil {
				t.Error(e)
			}
		})
	}
	t.Run("a write the tracker refuses", func(t *testing.T) {
		rt := rtFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodPost {
				return answer(409, `{"errorMessages":["PAY-6 is closed"]}`), nil
			}
			return answer(200, `{"key":"PAY-6","fields":{"project":{"key":"PAY"},"status":{"name":"To Do","statusCategory":{"key":"new"}}},"transitions":[{"id":"31","to":{"name":"Done","statusCategory":{"key":"done"}}}]}`), nil
		})
		_, err := run(over(t, rt), "TransitionIssue", ticketingkit.Options{}, &ticketingv1.TransitionIssueRequest{Project: "PAY", IssueKey: "PAY-6", Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE})
		ce := failed(t, err)
		if e := ticketingkit.ValidateErrorMessage(&ticketingkit.CallError{Reason: ce.Reason, Message: ce.Message}, secrets...); ce.Reason != pluginv1.Reason_REASON_CONFLICT || e != nil {
			t.Errorf("TransitionIssue after a 409 = %s %q (%v), want CONFLICT with a fixed message", ce.Reason, ce.Message, e)
		}
	})
	t.Run("a search the tracker refuses", func(t *testing.T) {
		rt := rtFunc(func(*http.Request) (*http.Response, error) {
			return answer(400, `{"errorMessages":["no project PAY"]}`), nil
		})
		_, err := run(over(t, rt), "SearchIssues", ticketingkit.Options{}, &ticketingv1.SearchIssuesRequest{Project: "PAY", MaxResults: 5})
		if got := failed(t, err).Reason; got != pluginv1.Reason_REASON_NOT_FOUND {
			t.Errorf("SearchIssues after a 400 = %s, want NOT_FOUND", got)
		}
	})
}

// A cancelled or expired context ends the call TIMEOUT before the transport's own answer is read.
func TestContext_ExpiredEndsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	rt := rtFunc(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	_, _, err := over(t, rt).Direct(ctx, rpc+"GetIssue", &pluginv1.ActionContext{}, getReq("PAY", "PAY-1"))
	if got := failed(t, err).Reason; got != pluginv1.Reason_REASON_TIMEOUT {
		t.Errorf("GetIssue with an expired context = %s, want TIMEOUT", got)
	}
}

// Egress. A live manifest declares exactly the instance host on port 443, and a replay declares no network at all. A bad host
// is refused when the provider is built, so the configuration cannot spell a wildcard, a port, an IP literal or a local name.
func TestEgress_ManifestDeclaresOnlyTheInstanceHost(t *testing.T) {
	never := rtFunc(func(*http.Request) (*http.Response, error) {
		t.Error("a request was made")
		return nil, errors.New("no")
	})
	for _, replayMode := range []bool{false, true} {
		srv, err := provider.New(provider.PluginID, replay.Host, "sha256:0", provider.Transport{RT: never, Replay: replayMode}, &recordingHost{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		m, err := srv.GetManifest(t.Context(), &pluginv1.GetManifestRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if len(m.GetManifest().GetCapabilities()) != 5 {
			t.Fatalf("manifest has %d capabilities, want the five verbs of the port", len(m.GetManifest().GetCapabilities()))
		}
		for _, c := range m.GetManifest().GetCapabilities() {
			var got []string
			for _, n := range c.GetNetworkIntents() {
				got = append(got, fmt.Sprintf("%s:%d", n.GetHost(), n.GetPort()))
			}
			if replayMode {
				if len(c.GetNetworkIntents()) != 0 || len(c.GetCredentialIntents()) != 0 || !slices.Equal(c.GetEnvironmentScope(), []string{"dev"}) {
					t.Errorf("replay capability %s declares network %v, credentials %v, scope %v, want none and dev only", c.GetName(), got, c.GetCredentialIntents(), c.GetEnvironmentScope())
				}
				continue
			}
			ni := c.GetNetworkIntents()
			if len(ni) != 1 || ni[0].GetHost() != replay.Host || ni[0].GetPort() != 443 || strings.ContainsAny(ni[0].GetHost(), "*?[]") {
				t.Errorf("live capability %s declares network %v, want exactly %s:443", c.GetName(), ni, replay.Host)
			}
			if !slices.Equal(c.GetCredentialIntents(), []string{provider.Intent}) {
				t.Errorf("live capability %s declares credentials %v, want %q", c.GetName(), c.GetCredentialIntents(), provider.Intent)
			}
		}
	}
}

func TestValidHost_RefusesWhatIsNotOneExactPublicName(t *testing.T) {
	for _, h := range []string{
		"", "acme", "*.example.test", "*", "acme.*.test", "ACME.example.test", "acme.example.test.", ".example.test", "acme..example.test",
		"acme.example.test:8443", "acme.example.test:443", "user@acme.example.test", "acme.example.test/path", "acme example.test", "acme_x.example.test",
		"127.0.0.1", "10.0.0.1", "169.254.169.254", "0x7f.1", "2130706433", "[::1]", "::1", "localhost", "a.localhost", "svc.internal", "host.local",
		"acme.example.test\n", "acme.exa\u200bmple.test", "\uff41cme.example.test", "-acme.example.test", "acme-.example.test", "a.1",
	} {
		if err := provider.ValidHost(h); err == nil {
			t.Errorf("ValidHost(%q) = nil, want an error", h)
		}
		if _, err := provider.New(provider.PluginID, h, "sha256:0", provider.Transport{}, &recordingHost{}, nil); err == nil {
			t.Errorf("New(host %q) error = nil, want an error", h)
		}
	}
	for _, h := range []string{"acme.example.test", "globex.example.test", "a-b.c1.example.test"} {
		if err := provider.ValidHost(h); err != nil {
			t.Errorf("ValidHost(%q) = %v, want nil", h, err)
		}
	}
}

// A live provider whose host gives no credential handle (this test host never does) sends nothing, whatever it is asked: it never
// falls back to an unauthenticated request.
func TestLive_WithoutAHandleIsUnavailableAndSendsNothing(t *testing.T) {
	rt := &counting{fn: func(*http.Request) (*http.Response, error) { return answer(200, "{}"), nil }}
	srv, err := provider.New(provider.PluginID, replay.Host, "sha256:0", provider.Transport{RT: rt}, &recordingHost{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]proto.Message{
		"GetIssue":        getReq("PAY", "PAY-1"),
		"SearchIssues":    &ticketingv1.SearchIssuesRequest{Project: "PAY", MaxResults: 5},
		"CommentOnIssue":  &ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: "x"},
		"TransitionIssue": &ticketingv1.TransitionIssueRequest{Project: "PAY", IssueKey: "PAY-1", Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE},
		"LinkIssue": &ticketingv1.LinkIssueRequest{Project: "PAY", IssueKey: "PAY-1", BlockedProject: "PAY", BlockedIssueKey: "PAY-2",
			LinkType: ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS},
	}
	for name, req := range calls {
		_, err := run(srv, name, ticketingkit.Options{Key: "k"}, req)
		if got := failed(t, err).Reason; got != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
			t.Errorf("%s on a live provider = %s, want UPSTREAM_UNAVAILABLE", name, got)
		}
	}
	if n := rt.count(); n != 0 {
		t.Errorf("a live provider made %d requests, want 0", n)
	}
}

// The error of a call unwraps to the shared server's CallError, so Invoke ends the call FAILED with the same reason.
func TestCallError_UnwrapsToTheSharedServersReason(t *testing.T) {
	_, err := run(over(t, rtFunc(func(*http.Request) (*http.Response, error) { return answer(404, "{}"), nil })), "GetIssue", ticketingkit.Options{}, getReq("PAY", "PAY-1"))
	var shared *ghprovider.CallError
	if !errors.As(err, &shared) || shared.Reason != pluginv1.Reason_REASON_NOT_FOUND {
		t.Errorf("errors.As(%v, *ghprovider.CallError) = %v, want NOT_FOUND", err, shared)
	}
}

// Every request of a whole kit run goes to https on the instance host, under the REST root, with no user information, fragment
// or header that authenticates.
func TestRequests_StayOnTheInstanceWithoutCredentials(t *testing.T) {
	tg := newTarget(t)
	s := subject(tg)
	for _, c := range ticketingkit.Cases() {
		if err := c.Check(t.Context(), s); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
	}
	reqs := tg.tap.requests()
	if len(reqs) < 100 {
		t.Fatalf("the kit made %d requests, want the whole run", len(reqs))
	}
	for _, r := range reqs {
		u, err := url.Parse(r.URL)
		if err != nil || u.Scheme != "https" || u.Host != replay.Host || u.User != nil || u.Fragment != "" || !strings.HasPrefix(u.Path, "/rest/api/3/") || r.Auth {
			t.Fatalf("request %s %s (credential header %v) leaves the instance or carries a credential", r.Method, r.URL, r.Auth)
		}
	}
}
