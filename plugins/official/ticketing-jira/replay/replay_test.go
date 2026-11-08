package replay_test

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/replay"
)

var update = flag.Bool("update", false, "rewrite testdata/jira.json from the world of world_test.go") //nolint:gochecknoglobals // the -update flag of a golden test

func TestCassette_IsTheGeneratedOne(t *testing.T) {
	want := marshal(build())
	if *update {
		if err := os.WriteFile("testdata/jira.json", want, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := replay.Embedded(); !bytes.Equal(got, want) {
		t.Errorf("testdata/jira.json differs from the cassette of world_test.go; run go test -run TestCassette_IsTheGeneratedOne -update and review the diff")
	}
}

func TestCassette_ParsesAndNamesTheReplayHost(t *testing.T) {
	c, err := replay.Parse(replay.Embedded())
	if err != nil {
		t.Fatalf("Parse(embedded) error = %v, want nil", err)
	}
	if c.Host != replay.Host || len(c.Interactions) < 90 {
		t.Errorf("embedded cassette has host %q and %d exchanges, want %q and the whole world", c.Host, len(c.Interactions), replay.Host)
	}
}

// The scan of ADR 0060 decision 10 over this plugin's own fixture and README with the plugin's own closed set of names: a host
// outside example.test, a tenant name, an e-mail address or an IP literal fails here.
func TestCassette_OnlyReservedHostsAndTenants(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	allow := ticketingkit.Allow{
		Tokens: vocabulary(),
		Keys:   keys(),
		// The dotted names of the README: the plugin id, the file and identifier names it mentions. Never a host.
		Dotted: []string{"dev.numinance.ticketing-jira", "jira.json", "numinance.ports.ticketing.v1", "PluginService.Invoke", "ticketing.proto",
			"ticketing.query_language", "ticketingkit.Observer", "ticketingkit.ScanDirWith", "world_test.go"},
	}
	problems, err := ticketingkit.ScanDirWith(fstest.MapFS{
		"testdata/jira.json": {Data: replay.Embedded()},
		"README.md":          {Data: readme},
	}, allow)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// The scan rejects what it exists to reject, with this plugin's allowlist: a real-looking host and a tenant name each fail.
func TestCassette_ScanRejectsOtherHosts(t *testing.T) {
	allow := ticketingkit.Allow{Tokens: vocabulary(), Keys: keys()}
	for _, mutate := range []func(string) string{
		func(s string) string {
			return strings.Replace(s, `"host": "acme.example.test"`, `"host": "acme.example.com"`, 1)
		},
		func(s string) string {
			return strings.Replace(s, `"host": "acme.example.test"`, `"host": "mycompany.example.test.evil"`, 1)
		},
		func(s string) string { return strings.Replace(s, "Fix the cart total", "Fix the mycompany total", 1) },
		func(s string) string { return strings.Replace(s, "invoice totals", "ask me@mycompany.example.test", 1) },
		func(s string) string { return strings.Replace(s, "invoice export", "see 10.0.0.1", 1) },
	} {
		mutated := mutate(string(replay.Embedded()))
		problems, err := ticketingkit.ScanDirWith(fstest.MapFS{"testdata/jira.json": {Data: []byte(mutated)}}, allow)
		if err != nil || len(problems) == 0 {
			t.Errorf("scan of a mutated cassette = %v, %v, want a problem", problems, err)
		}
	}
}

func newReplay(t *testing.T) *replay.Replay {
	t.Helper()
	c, err := replay.Parse(replay.Embedded())
	if err != nil {
		t.Fatal(err)
	}
	r, err := replay.New(c)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func roundTrip(t *testing.T, r http.RoundTripper, method, target string, body string, header map[string]string) (*http.Response, error) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	return r.RoundTrip(req)
}

func TestReplay_AnswersByStateAndFailsClosed(t *testing.T) {
	r := newReplay(t)
	base := "https://" + replay.Host + "/rest/api/3/issue/PAY-6?" + readQuery
	read := func() string {
		resp, err := roundTrip(t, r, http.MethodGet, base, "", nil)
		if err != nil {
			t.Fatalf("GET PAY-6 error = %v, want an answer", err)
		}
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if got := read(); !strings.Contains(got, `"To Do"`) {
		t.Errorf("PAY-6 before the move = %.120s, want To Do", got)
	}
	move := `{"transition":{"id":"31"}}`
	if resp, err := roundTrip(t, r, http.MethodPost, "https://"+replay.Host+"/rest/api/3/issue/PAY-6/transitions", move, nil); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST transition = %v, %v, want 204", resp, err)
	}
	if got := read(); !strings.Contains(got, `"Done"`) {
		t.Errorf("PAY-6 after the move = %.120s, want Done: the exchange did not change the state", got)
	}
	for name, c := range map[string]struct {
		method, target, body string
		header               map[string]string
	}{
		"an unrecorded path":      {http.MethodGet, "https://" + replay.Host + "/rest/api/3/issue/PAY-77?" + readQuery, "", nil},
		"an unrecorded query":     {http.MethodGet, "https://" + replay.Host + "/rest/api/3/issue/PAY-1?fields=all", "", nil},
		"another host":            {http.MethodGet, "https://other.example.test/rest/api/3/issue/PAY-1?" + readQuery, "", nil},
		"plain http":              {http.MethodGet, "http://" + replay.Host + "/rest/api/3/issue/PAY-1?" + readQuery, "", nil},
		"an unrecorded body":      {http.MethodPost, "https://" + replay.Host + "/rest/api/3/issue/PAY-6/transitions", `{"transition":{"id":"99"}}`, nil},
		"a credential header":     {http.MethodGet, "https://" + replay.Host + "/rest/api/3/issue/PAY-1?" + readQuery, "", map[string]string{"Authorization": "Bearer x"}},
		"a handle header":         {http.MethodGet, "https://" + replay.Host + "/rest/api/3/issue/PAY-1?" + readQuery, "", map[string]string{"Numinance-Handle": "h"}},
		"a state not yet reached": {http.MethodPost, "https://" + replay.Host + "/rest/api/3/issue/PAY-6/transitions", move, nil},
	} {
		if resp, err := roundTrip(t, r, c.method, c.target, c.body, c.header); !errors.Is(err, replay.ErrNoRecording) {
			t.Errorf("%s: RoundTrip = %v, %v, want ErrNoRecording", name, resp, err)
		}
	}
}

func TestParse_RefusesAnIncompleteCassette(t *testing.T) {
	for name, in := range map[string]string{
		"no redaction note": `{"host":"acme.example.test","interactions":[]}`,
		"no host":           `{"redaction":"x","interactions":[]}`,
		"an unknown field":  `{"redaction":"x","host":"acme.example.test","extra":1,"interactions":[]}`,
		"a second value":    `{"redaction":"x","host":"acme.example.test","interactions":[]} {}`,
		"a bad status":      `{"redaction":"x","host":"acme.example.test","interactions":[{"method":"GET","path":"/a","status":99}]}`,
	} {
		if _, err := replay.Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%s) error = nil, want an error", name)
		}
	}
}
