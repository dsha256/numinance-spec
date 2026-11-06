package provider_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	scmv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"

	"github.com/numinance/numinance-spec/plugins/official/scm-github/provider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/replay"
)

type recordingHost struct {
	mu      sync.Mutex
	emitted []string
	handle  string
}

func (h *recordingHost) Credential(context.Context, string, string) (string, error) {
	return h.handle, nil
}

func (h *recordingHost) Emit(_ context.Context, _, _, recordType string, payload *structpb.Struct) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.emitted = append(h.emitted, recordType+":"+payload.GetFields()["repository"].GetStringValue())
	return "ev_1", nil
}

// roundTripFunc is a transport that answers every request with one function.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func answer(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

const rpc = "/numinance.ports.scm.v1.ScmService/"

func reason(t *testing.T, err error) pluginv1.Reason {
	t.Helper()
	var ce *provider.CallError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want a FAILED call", err)
	}
	return ce.Reason
}

func replayServer(t *testing.T, h provider.Host) *provider.Server {
	t.Helper()
	tr, err := replay.Transport()
	if err != nil {
		t.Fatal(err)
	}
	return provider.New(provider.ReplayPluginID, "sha256:0", tr, h, nil)
}

func TestProvider_OpenPullRequest_EmitsPrRefReceiptWithoutTitleOrBody(t *testing.T) {
	t.Parallel()
	h := &recordingHost{}
	s := replayServer(t, h)
	req := &scmv1.OpenPullRequestRequest{Repository: replay.Repository, HeadBranch: "numinance/chg-0001", HeadRevision: replay.Candidate, BaseBranch: "main", Title: "Fix the cart total", Body: "Candidate."}
	_, ids, err := s.Direct(t.Context(), rpc+"OpenPullRequest", &pluginv1.ActionContext{IdempotencyKey: "k"}, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || len(h.emitted) != 1 || h.emitted[0] != "pr-ref:acme/shop" {
		t.Errorf("evidence ids = %v, emitted = %v, want one pr-ref receipt of acme/shop", ids, h.emitted)
	}
}

func TestProvider_Invalid(t *testing.T) {
	t.Parallel()
	s := replayServer(t, &recordingHost{})
	rev, repo := replay.Candidate, replay.Repository
	cases := []struct {
		name   string
		method string
		req    proto.Message
	}{
		{"short revision", "GetTree", &scmv1.GetTreeRequest{Repository: repo, Revision: "abc", MaxEntries: 5}},
		{"a branch name for a revision", "GetFile", &scmv1.GetFileRequest{Repository: repo, Revision: "main", Path: "a", MaxBytes: 5}},
		{"parent path segment", "GetFile", &scmv1.GetFileRequest{Repository: repo, Revision: rev, Path: "a/../b", MaxBytes: 5}},
		{"zero max entries", "GetTree", &scmv1.GetTreeRequest{Repository: repo, Revision: rev, MaxEntries: 0}},
		{"branch with a space", "CreateBranch", &scmv1.CreateBranchRequest{Repository: repo, Name: "a b", FromRevision: rev}},
		{"branch with dot dot", "CreateBranch", &scmv1.CreateBranchRequest{Repository: repo, Name: "a..b", FromRevision: rev}},
		{"title too long", "OpenPullRequest", &scmv1.OpenPullRequestRequest{Repository: repo, HeadBranch: "a", HeadRevision: rev, BaseBranch: "main", Title: strings.Repeat("t", 257)}},
		{"reopen is not allowed", "UpdatePullRequest", &scmv1.UpdatePullRequestRequest{Repository: repo, Number: 7, State: scmv1.PullRequestState_PULL_REQUEST_STATE_OPEN}},
		{"empty update", "UpdatePullRequest", &scmv1.UpdatePullRequestRequest{Repository: repo, Number: 7}},
		{"pull request zero", "GetPullRequestStatus", &scmv1.GetPullRequestStatusRequest{Repository: repo, Number: 0}},
		{"merge without a head", "MergePullRequest", &scmv1.MergePullRequestRequest{Repository: repo, Number: 8}},
		{"comment without a key", "CommentOnPullRequest", &scmv1.CommentOnPullRequestRequest{Repository: repo, Number: 7, Body: "x"}},
		{"search with a qualifier quote", "SearchCode", &scmv1.SearchCodeRequest{Repository: repo, Query: `x" repo:other/secret`, MaxResults: 5}},
		{"page token not a number", "ListBranches", &scmv1.ListBranchesRequest{Repository: repo, PageSize: 2, PageToken: "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := s.Direct(t.Context(), rpc+tc.method, &pluginv1.ActionContext{}, tc.req)
			if got := reason(t, err); got != pluginv1.Reason_REASON_INVALID_ARGUMENT {
				t.Errorf("%s = %s, want INVALID_ARGUMENT", tc.method, got)
			}
		})
	}
}

func TestProvider_SearchCode_DropsHitsOfOtherRepositories(t *testing.T) {
	t.Parallel()
	s := replayServer(t, &recordingHost{})
	resp, _, err := s.Direct(t.Context(), rpc+"SearchCode", &pluginv1.ActionContext{}, &scmv1.SearchCodeRequest{Repository: replay.Repository, Query: "TODO", MaxResults: 5})
	if err != nil {
		t.Fatal(err)
	}
	got := resp.(*scmv1.SearchCodeResponse).GetPaths()
	if len(got) != 1 || got[0] != "src/main.go" {
		t.Errorf("SearchCode paths = %v, want only src/main.go: the hit of other/secret is dropped", got)
	}
}

func TestProvider_GetTree_Filters(t *testing.T) {
	t.Parallel()
	s := replayServer(t, &recordingHost{})
	get := func(path string, recursive bool, limit uint32) *scmv1.GetTreeResponse {
		t.Helper()
		resp, _, err := s.Direct(t.Context(), rpc+"GetTree", &pluginv1.ActionContext{}, &scmv1.GetTreeRequest{Repository: replay.Repository, Revision: replay.BaseRevision, Path: path, Recursive: recursive, MaxEntries: limit})
		if err != nil {
			t.Fatal(err)
		}
		return resp.(*scmv1.GetTreeResponse)
	}
	if got := get("", false, 10); len(got.GetEntries()) != 3 {
		t.Errorf("root entries = %d, want 3 (.numinance, README.md, src)", len(got.GetEntries()))
	}
	if got := get(".numinance", true, 10); len(got.GetEntries()) != 2 {
		t.Errorf("recursive .numinance entries = %d, want 2", len(got.GetEntries()))
	}
	if got := get("", true, 2); len(got.GetEntries()) != 2 || !got.GetTruncated() {
		t.Errorf("max 2 = %d entries truncated=%v, want 2 and truncated", len(got.GetEntries()), got.GetTruncated())
	}
	_, _, err := s.Direct(t.Context(), rpc+"GetTree", &pluginv1.ActionContext{}, &scmv1.GetTreeRequest{Repository: replay.Repository, Revision: replay.BaseRevision, Path: "nope", MaxEntries: 5})
	if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
		t.Errorf("a missing directory = %s, want NOT_FOUND", got)
	}
}

func TestProvider_GetFile_CutsAtMaxBytes(t *testing.T) {
	t.Parallel()
	s := replayServer(t, &recordingHost{})
	resp, _, err := s.Direct(t.Context(), rpc+"GetFile", &pluginv1.ActionContext{}, &scmv1.GetFileRequest{Repository: replay.Repository, Revision: replay.BaseRevision, Path: "README.md", MaxBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	f := resp.(*scmv1.GetFileResponse)
	if string(f.GetContent()) != "# sh" || !f.GetTruncated() || f.GetSize() != 15 {
		t.Errorf("GetFile = %q truncated=%v size=%d, want %q truncated size 15", f.GetContent(), f.GetTruncated(), f.GetSize(), "# sh")
	}
	_, _, err = s.Direct(t.Context(), rpc+"GetFile", &pluginv1.ActionContext{}, &scmv1.GetFileRequest{Repository: replay.Repository, Revision: replay.BaseRevision, Path: "src", MaxBytes: 4})
	if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
		t.Errorf("a directory as a file = %s, want NOT_FOUND", got)
	}
}

func TestProvider_NoRecordingFailsClosed(t *testing.T) {
	t.Parallel()
	s := replayServer(t, &recordingHost{})
	_, _, err := s.Direct(t.Context(), rpc+"GetRepository", &pluginv1.ActionContext{}, &scmv1.GetRepositoryRequest{Repository: "acme/unrecorded"})
	if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
		t.Errorf("GetRepository of an unrecorded repository = %s, want UPSTREAM_UNAVAILABLE", got)
	}
}

func TestProvider_LiveTransport_SendsTheHandleAndNoOtherCredential(t *testing.T) {
	t.Parallel()
	var seen *http.Request
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r
		return answer(`{"default_branch":"main","private":false,"archived":false,"commit":{"sha":"` + replay.BaseRevision + `"}}`), nil
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: rt}, &recordingHost{handle: "h-123"}, nil)
	if _, _, err := s.Direct(t.Context(), rpc+"GetRepository", &pluginv1.ActionContext{}, &scmv1.GetRepositoryRequest{Repository: replay.Repository}); err != nil {
		t.Fatal(err)
	}
	if got := seen.Header.Get("Numinance-Handle"); got != "h-123" {
		t.Errorf("Numinance-Handle = %q, want the handle the host gave", got)
	}
	if seen.Header.Get("Authorization") != "" || seen.URL.Host != "api.github.com" {
		t.Errorf("Authorization = %q host = %q, want no token and api.github.com", seen.Header.Get("Authorization"), seen.URL.Host)
	}
}

func TestProvider_LiveTransport_NoHandleFailsTheCall(t *testing.T) {
	t.Parallel()
	called := false
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) { called = true; return answer(`{}`), nil })
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: rt}, &recordingHost{handle: ""}, nil)
	_, _, err := s.Direct(t.Context(), rpc+"GetRepository", &pluginv1.ActionContext{}, &scmv1.GetRepositoryRequest{Repository: replay.Repository})
	if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE || called {
		t.Errorf("reason = %s, sent = %v, want UPSTREAM_UNAVAILABLE and no request: the provider never falls back to an ambient credential", got, called)
	}
}

func TestProvider_UntrustedAnswers(t *testing.T) {
	t.Parallel()
	huge := `{"default_branch":"main","pad":"` + strings.Repeat("x", 5<<20) + `"}`
	cases := []struct {
		name string
		body string
		want pluginv1.Reason
	}{
		{"an answer over the limit", huge, pluginv1.Reason_REASON_LIMIT_EXCEEDED},
		{"a default branch that is not a branch name", `{"default_branch":"a b"}`, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"an answer that is not JSON", `<html>`, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) { return answer(tc.body), nil })
			s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: rt}, &recordingHost{handle: "h"}, nil)
			_, _, err := s.Direct(t.Context(), rpc+"GetRepository", &pluginv1.ActionContext{}, &scmv1.GetRepositoryRequest{Repository: replay.Repository})
			if got := reason(t, err); got != tc.want {
				t.Errorf("GetRepository = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestProvider_Merge_DryRunMergesNothing(t *testing.T) {
	t.Parallel()
	var puts int
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			puts++
			return answer(`{"sha":"` + replay.MergeCommit + `","merged":true}`), nil
		}
		return answer(`{"number":8,"state":"open","mergeable":true,"head":{"ref":"x","sha":"` + replay.Candidate + `"},"base":{"ref":"main"},"html_url":"https://github.com/acme/shop/pull/8"}`), nil
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: rt}, &recordingHost{handle: "h"}, nil)
	req := &scmv1.MergePullRequestRequest{Repository: replay.Repository, Number: 8, ExpectedHeadRevision: replay.Candidate, ExpectedBaseBranch: "main"}
	resp, _, err := s.Direct(t.Context(), rpc+"MergePullRequest", &pluginv1.ActionContext{DryRun: true}, req)
	if err != nil {
		t.Fatal(err)
	}
	if puts != 0 || resp.(*scmv1.MergePullRequestResponse).GetMergeRevision() != "" {
		t.Errorf("merges = %d, merge revision = %q, want none for a dry run", puts, resp.(*scmv1.MergePullRequestResponse).GetMergeRevision())
	}
}

func TestProvider_Manifest_DeclaresPrRefAndNoWiderRisk(t *testing.T) {
	t.Parallel()
	s := replayServer(t, &recordingHost{})
	m, err := s.GetManifest(t.Context(), &pluginv1.GetManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var open, merge *pluginv1.CapabilityDescriptor
	for _, c := range m.GetManifest().GetCapabilities() {
		switch c.GetName() {
		case "scm.open_pull_request":
			open = c
		case "scm.merge_pull_request":
			merge = c
		}
	}
	if open == nil || len(open.GetEvidenceDeclared()) != 1 || open.GetEvidenceDeclared()[0] != "pr-ref" {
		t.Errorf("open_pull_request evidence = %v, want [pr-ref]", open.GetEvidenceDeclared())
	}
	if merge == nil || merge.GetRiskFloor().String() != "RISK_CLASS_PRIVILEGED" {
		t.Errorf("merge_pull_request = %v, want a PRIVILEGED capability", merge)
	}
	for _, c := range m.GetManifest().GetCapabilities() {
		if len(c.GetNetworkIntents()) != 0 || len(c.GetCredentialIntents()) != 0 || !slices.Equal(c.GetEnvironmentScope(), []string{"dev"}) {
			t.Errorf("replay %s network = %v credentials = %v scope = %v, want no network, no credential and dev only", c.GetName(), c.GetNetworkIntents(), c.GetCredentialIntents(), c.GetEnvironmentScope())
		}
	}
}

// The live provider's manifest is what it was: one network destination, one credential intent, every environment.
func TestProvider_LiveManifest_DeclaresTheNetworkAndTheCredentialIntent(t *testing.T) {
	t.Parallel()
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{}, &recordingHost{}, nil)
	m, err := s.GetManifest(t.Context(), &pluginv1.GetManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.GetManifest().GetCapabilities()) == 0 {
		t.Fatal("the live manifest declares no capability")
	}
	for _, c := range m.GetManifest().GetCapabilities() {
		if len(c.GetNetworkIntents()) != 1 || c.GetNetworkIntents()[0].GetHost() != "api.github.com" || c.GetNetworkIntents()[0].GetPort() != 443 ||
			len(c.GetCredentialIntents()) != 1 || !slices.Equal(c.GetEnvironmentScope(), []string{"dev", "staging", "prod"}) {
			t.Errorf("%s network = %v intents = %v scope = %v, want api.github.com:443, one credential intent, dev/staging/prod", c.GetName(), c.GetNetworkIntents(), c.GetCredentialIntents(), c.GetEnvironmentScope())
		}
	}
}
