package ghprovider_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// fakeToken builds a string shaped like a GitHub token at run time, so no token-shaped literal is in the source.
func fakeToken() string { return "gh" + "p_" + strings.Repeat("a1B2", 9) }

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestParseRename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    ghprovider.Rename
		wantErr bool
	}{
		{in: "owner/repo=acme/shop", want: ghprovider.Rename{From: "owner/repo", To: "acme/shop"}},
		{in: "owner/repo", wantErr: true},
		{in: "=x", wantErr: true},
		{in: "x=", wantErr: true},
		{in: "acme=acme-two", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ghprovider.ParseRename(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseRename(%q) = %+v, %v, want %+v (error %v)", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestRedactor_Scrub(t *testing.T) {
	t.Parallel()
	r := ghprovider.Redactor{Renames: []ghprovider.Rename{{From: "alice", To: "author-one"}, {From: "alice/secret-repo", To: "acme/shop"}}}
	tests := []struct {
		name, in, want string
	}{
		{name: "the longest name first", in: "/repos/alice/secret-repo", want: "/repos/acme/shop"},
		{name: "a lone owner", in: "by alice", want: "by author-one"},
		{name: "a token", in: "token " + fakeToken(), want: "token REDACTED"},
		{name: "an e-mail address", in: "mail a.b@corp.example.com now", want: "mail redacted@example.invalid now"},
		{name: "nothing to do", in: "plain", want: "plain"},
	}
	for _, tt := range tests {
		if got := r.Scrub(tt.in); got != tt.want {
			t.Errorf("Scrub(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestRecorder_WritesARedactedCassetteTheReplayAnswers(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "scm.json")
	body := `{"default_branch":"main","owner":"alice","email":"a@b.example.org","note":"` + fakeToken() + `"}`
	rec := &ghprovider.Recorder{
		Out:      out,
		Redactor: ghprovider.Redactor{Renames: []ghprovider.Rename{{From: "alice/secret-repo", To: "acme/shop"}, {From: "alice", To: "author-one"}}},
		Inner: rtFunc(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get(ghprovider.HandleHeader) == "" {
				t.Error("the inner transport got no handle header: the recorder must pass the request through unchanged")
			}
			return respond(200, body), nil
		}),
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ghprovider.BaseURL+"/repos/alice/secret-repo?page=1", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(ghprovider.HandleHeader, "nh_x")
	resp, err := rec.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v, want nil", err)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Errorf("RoundTrip() body = %q, want the answer unchanged for the caller", got)
	}
	raw, err := os.ReadFile(out) //nolint:gosec // the test reads the cassette it just wrote
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{fakeToken(), "alice", "a@b.example.org", "nh_x", ghprovider.HandleHeader} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("the cassette contains %q, want it redacted or never recorded", leak)
		}
	}
	if fi, err := os.Stat(out); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("cassette mode = %v, %v, want 0600", fi.Mode(), err)
	}
	c, err := ghprovider.ParseCassette(raw)
	if err != nil || len(c.Interactions) != 1 || c.Interactions[0].Path != "/repos/acme/shop?page=1" {
		t.Fatalf("ParseCassette() = %+v, %v, want one exchange for the renamed path", c, err)
	}
	replay, err := ghprovider.NewReplay(c)
	if err != nil {
		t.Fatal(err)
	}
	replayReq, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, ghprovider.BaseURL+"/repos/acme/shop?page=1", http.NoBody)
	if r, err := replay.RoundTrip(replayReq); err != nil || r.StatusCode != 200 {
		t.Errorf("replay of the recording = %v, %v, want the recorded answer", r, err)
	}
}

func TestRecorder_RefusesWhatItCannotRecord(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		inner rtFunc
		path  []string // under a fresh temporary directory
		want  error
	}{
		{
			name:  "an answer that is not JSON",
			inner: func(*http.Request) (*http.Response, error) { return respond(200, "<html>"), nil },
			path:  []string{"c.json"},
		},
		{
			name:  "an unwritable cassette directory",
			inner: func(*http.Request) (*http.Response, error) { return respond(200, "{}"), nil },
			path:  []string{"missing", "c.json"},
		},
		{
			name:  "a failing transport",
			inner: func(*http.Request) (*http.Response, error) { return nil, errors.New("boom") },
			path:  []string{"c.json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(append([]string{t.TempDir()}, tt.path...)...)
			rec := &ghprovider.Recorder{Inner: tt.inner, Out: out}
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, ghprovider.BaseURL+"/x", http.NoBody)
			if _, err := rec.RoundTrip(req); err == nil {
				t.Error("RoundTrip() error = nil, want an error: a run that cannot be recorded fails")
			}
			if _, err := os.Stat(out); err == nil {
				t.Error("a cassette was written, want none")
			}
		})
	}
}

func TestRecorder_RecordsARequestBody(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "c.json")
	var sent string
	rec := &ghprovider.Recorder{Out: out, Inner: rtFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		sent = string(b)
		return respond(201, `{"number":1}`), nil
	})}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ghprovider.BaseURL+"/x", strings.NewReader(`{"title":"t"}`))
	if _, err := rec.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(out) //nolint:gosec // the test reads the cassette it just wrote
	c, err := ghprovider.ParseCassette(raw)
	if err != nil || sent != `{"title":"t"}` || len(c.Interactions) != 1 || compact(c.Interactions[0].Request) != `{"title":"t"}` {
		t.Errorf("recorded = %+v, %v; inner saw %q, want the body passed through and recorded", c, err, sent)
	}
}

// compact removes the white space MarshalIndent adds, so a body compares as the client sent it.
func compact(b []byte) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return string(b)
	}
	return buf.String()
}
