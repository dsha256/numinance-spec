package replay_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	"context"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	scmv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/provider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/replay"
)

const (
	headA = "1111111111111111111111111111111111111111"
	headB = "2222222222222222222222222222222222222222"
)

func scenario() replay.PullRequestScenario {
	return replay.PullRequestScenario{
		Repository: "acme/numinance", BaseBranch: "main", HeadBranch: "numinance/chg_0001", Head: headA,
		Title: "Numinance change chg_0001", Body: "Opened for change chg_0001 at candidate " + headA + ".", Number: 7,
	}
}

type host struct{}

func (host) Credential(context.Context, string, string) (string, error) { return "handle", nil }
func (host) Emit(context.Context, string, string, string, *structpb.Struct) (string, error) {
	return "ev_1", nil
}

// serve is the provider over the embedded cassettes and the scenario's own, written to a file as the dry run does.
func serve(t *testing.T, s replay.PullRequestScenario) *provider.Server {
	t.Helper()
	b, err := replay.PullRequestCassette(s)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	tr, err := replay.Transport(path)
	if err != nil {
		t.Fatal(err)
	}
	return provider.New(provider.ReplayPluginID, "sha256:0", tr, host{}, nil)
}

func open(s *provider.Server, r replay.PullRequestScenario, mutate func(*scmv1.OpenPullRequestRequest)) (*scmv1.OpenPullRequestResponse, error) {
	req := &scmv1.OpenPullRequestRequest{
		Repository: r.Repository, HeadBranch: r.HeadBranch, HeadRevision: r.Head, BaseBranch: r.BaseBranch, Title: r.Title, Body: r.Body,
	}
	if mutate != nil {
		mutate(req)
	}
	resp, _, err := s.Direct(context.Background(), "/numinance.ports.scm.v1.ScmService/OpenPullRequest", &pluginv1.ActionContext{IdempotencyKey: "k"}, req)
	if err != nil {
		return nil, err
	}
	return resp.(*scmv1.OpenPullRequestResponse), nil //nolint:forcetypeassert // the method's own response type
}

func TestPullRequestCassette_AnswersTheExactRequest(t *testing.T) {
	t.Parallel()
	s := scenario()
	got, err := open(serve(t, s), s, nil)
	if err != nil {
		t.Fatalf("OpenPullRequest(the scenario) error = %v, want nil", err)
	}
	if pr := got.GetPullRequest(); pr.GetNumber() != 7 || pr.GetHeadRevision() != headA || pr.GetHeadBranch() != s.HeadBranch || pr.GetBaseBranch() != "main" {
		t.Errorf("OpenPullRequest(the scenario) = %v, want pull request 7 for the scenario's head and base", pr)
	}
}

// A request that differs from the scenario in any field that matters has no recording, or meets a head the branch does not hold:
// the replay cannot be satisfied by a wrong request.
func TestPullRequestCassette_AWrongRequestFails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*scmv1.OpenPullRequestRequest)
		want   pluginv1.Reason
	}{
		{"a different head revision", func(r *scmv1.OpenPullRequestRequest) { r.HeadRevision = headB }, pluginv1.Reason_REASON_CONFLICT},
		{"a different head branch", func(r *scmv1.OpenPullRequestRequest) { r.HeadBranch = "numinance/chg_0002" }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"a different base branch", func(r *scmv1.OpenPullRequestRequest) { r.BaseBranch = "release" }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"a different repository", func(r *scmv1.OpenPullRequestRequest) { r.Repository = "acme/other" }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"a different owner", func(r *scmv1.OpenPullRequestRequest) { r.Repository = "evil/numinance" }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"a different title", func(r *scmv1.OpenPullRequestRequest) { r.Title = "Another title" }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"a different body", func(r *scmv1.OpenPullRequestRequest) { r.Body = "Another body" }, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := scenario()
			_, err := open(serve(t, s), s, tc.mutate)
			var ce *provider.CallError
			if !errors.As(err, &ce) || ce.Reason != tc.want {
				t.Errorf("OpenPullRequest(%s) error = %v, want a FAILED call with reason %s", tc.name, err, tc.want)
			}
		})
	}
}

func TestPullRequestCassette_RefusesWhatItCannotWriteSafely(t *testing.T) {
	t.Parallel()
	token := "ghp_" + strings.Repeat("a", 30)
	tests := []struct {
		name   string
		mutate func(*replay.PullRequestScenario)
	}{
		{"a repository with no owner", func(s *replay.PullRequestScenario) { s.Repository = "numinance" }},
		{"a repository with two slashes", func(s *replay.PullRequestScenario) { s.Repository = "a/b/c" }},
		{"no base branch", func(s *replay.PullRequestScenario) { s.BaseBranch = "" }},
		{"no head branch", func(s *replay.PullRequestScenario) { s.HeadBranch = "" }},
		{"no title", func(s *replay.PullRequestScenario) { s.Title = "" }},
		{"a short commit id", func(s *replay.PullRequestScenario) { s.Head = "abc123" }},
		{"an upper case commit id", func(s *replay.PullRequestScenario) { s.Head = strings.Repeat("A", 40) }},
		{"number zero", func(s *replay.PullRequestScenario) { s.Number = 0 }},
		{"a token in the repository owner", func(s *replay.PullRequestScenario) { s.Repository = token + "/numinance" }},
		{"an e-mail address in the base branch", func(s *replay.PullRequestScenario) { s.BaseBranch = "a@corp.example.com" }},
		{"a token in the head branch", func(s *replay.PullRequestScenario) { s.HeadBranch = "numinance/" + token }},
		{"a token in the title", func(s *replay.PullRequestScenario) { s.Title = token }},
		{"an e-mail address in the body", func(s *replay.PullRequestScenario) { s.Body = "ask someone@corp.example.com" }},
		{"a token in the body", func(s *replay.PullRequestScenario) { s.Body = "see " + token }},
		{"an e-mail address in the title", func(s *replay.PullRequestScenario) { s.Title = "by someone@corp.example.com" }},
	}
	for _, tc := range tests {
		s := scenario()
		tc.mutate(&s)
		if b, err := replay.PullRequestCassette(s); err == nil {
			t.Errorf("PullRequestCassette(%s) = %s, want an error", tc.name, b)
		}
	}
}

func TestPullRequestCassette_IsMarkedSyntheticAndParses(t *testing.T) {
	t.Parallel()
	b, err := replay.PullRequestCassette(scenario())
	if err != nil {
		t.Fatal(err)
	}
	c, err := ghprovider.ParseCassette(b)
	if err != nil {
		t.Fatalf("ParseCassette(the scenario's cassette) error = %v, want nil", err)
	}
	if !strings.HasPrefix(c.Redaction, "Synthetic, not recorded") || len(c.Interactions) != 4 {
		t.Errorf("cassette = redaction %q with %d exchanges, want a synthetic marker and four exchanges", c.Redaction, len(c.Interactions))
	}
}

// Every committed cassette is already what the recorder's redaction would leave: no credential pattern, no e-mail address.
func TestEmbeddedCassettes_AreRedactedAndSayHowTheyWereMade(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("cassettes")
	if err != nil || len(entries) == 0 {
		t.Fatalf("ReadDir(cassettes) = %v, %v, want the committed cassettes", entries, err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join("cassettes", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if scrubbed := (ghprovider.Redactor{}).Scrub(string(b)); scrubbed != string(b) {
			t.Errorf("%s holds a credential pattern or an e-mail address the redaction would replace", e.Name())
		}
		c, err := ghprovider.ParseCassette(b)
		if err != nil {
			t.Fatalf("ParseCassette(%s) error = %v, want nil", e.Name(), err)
		}
		if !strings.Contains(c.Redaction, "not recorded") && !strings.HasPrefix(c.Redaction, "Recorded from the live service") {
			t.Errorf("%s redaction = %q, want it to say it is synthetic or recorded", e.Name(), c.Redaction)
		}
	}
}

func TestTransport_RefusesABadCassetteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	big := write("big.json", `{"redaction":"x","interactions":[],"pad":"`+strings.Repeat("a", 5<<20)+`"}`)
	tests := []struct{ name, path string }{
		{"a relative path", "run.json"},
		{"a missing file", filepath.Join(dir, "missing.json")},
		{"not JSON", write("bad.json", "{")},
		{"an unknown field", write("unknown.json", `{"redaction":"x","interactions":[],"extra":1}`)},
		{"no redaction statement", write("plain.json", `{"interactions":[]}`)},
		{"over the size bound", big},
		{"a second JSON value after the first", write("two.json", `{"redaction":"x","interactions":[]} {"redaction":"y","interactions":[]}`)},
		{"trailing text", write("trail.json", `{"redaction":"x","interactions":[]} x`)},
		{"a directory", dir},
	}
	for _, tc := range tests {
		if _, err := replay.Transport(tc.path); err == nil {
			t.Errorf("Transport(%s) error = nil, want a refusal", tc.name)
		}
	}
}
