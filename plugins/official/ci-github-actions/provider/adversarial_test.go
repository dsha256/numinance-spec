package provider_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/provider"
)

func jsonResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

// A run of the pipeline workflow dispatched on another ref (so it runs that ref's own copy of the workflow file, which a candidate
// controls) with the predictable operation id is never adopted as the release run.
func TestStartOperation_NeverAdoptsARunOfAnotherBranch(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("key-1"))
	op := hex.EncodeToString(sum[:16])
	rev := strings.Repeat("a", 40)
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost:
			return jsonResp(204, ""), nil
		case strings.Contains(r.URL.Path, "/runs"):
			return jsonResp(200, fmt.Sprintf(`{"workflow_runs":[{"id":777,"display_title":"numinance %s %s","status":"completed","conclusion":"success","head_branch":"attacker-branch","path":".github/workflows/release.yml@refs/heads/attacker-branch"}]}`, op, rev)), nil
		case strings.HasSuffix(r.URL.Path, "/shop"):
			return jsonResp(200, `{"default_branch":"main"}`), nil
		}
		return jsonResp(200, `{}`), nil
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, 0, host{handle: "h"}, nil)
	resp, _, err := s.Direct(t.Context(), rpc+"StartOperation", &pluginv1.ActionContext{IdempotencyKey: "key-1"}, &civ1.StartOperationRequest{Repository: "acme/shop", Pipeline: "release.yml", Revision: rev})
	if err == nil {
		t.Errorf("StartOperation adopted %v, a run whose head_branch is not the default branch: want it ignored", resp)
	}
}

// A run named for another revision, or defined by another workflow file, is not the run asked for.
func TestRuns_AreBoundToTheDefaultBranchWorkflowAndTheRequestedRevision(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("key-1"))
	op := hex.EncodeToString(sum[:16])
	rev, other := strings.Repeat("a", 40), strings.Repeat("b", 40)
	cases := []struct {
		name, title, path string
	}{
		{"another revision in the name", "numinance " + op + " " + other, ".github/workflows/release.yml@refs/heads/main"},
		{"another workflow file", "numinance " + op + " " + rev, ".github/workflows/other.yml@refs/heads/main"},
		{"no path", "numinance " + op + " " + rev, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch {
				case strings.Contains(r.URL.Path, "/runs"):
					return jsonResp(200, fmt.Sprintf(`{"workflow_runs":[{"id":5,"display_title":%q,"status":"queued","head_branch":"main","path":%q}]}`, tc.title, tc.path)), nil
				case strings.HasSuffix(r.URL.Path, "/shop"):
					return jsonResp(200, `{"default_branch":"main"}`), nil
				}
				return jsonResp(200, `{}`), nil
			})
			s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, 0, host{handle: "h"}, nil)
			_, _, err := s.Direct(t.Context(), rpc+"StartOperation", &pluginv1.ActionContext{IdempotencyKey: "key-1"}, &civ1.StartOperationRequest{Repository: "acme/shop", Pipeline: "release.yml", Revision: rev})
			if err == nil {
				t.Errorf("StartOperation(%s) succeeded, want the run refused", tc.name)
			}
		})
	}
}

// A full page of runs that does not hold the operation may hold it further back: the look fails closed and nothing is dispatched,
// so a retry can never start a second run.
func TestStartOperation_AFullPageOfRunsFailsClosedWithoutDispatching(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString(`{"workflow_runs":[`)
	for i := range 100 {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":%d,"display_title":"numinance %s %s","status":"queued","head_branch":"main","path":".github/workflows/release.yml@refs/heads/main"}`, i+1, strings.Repeat("0", 32), strings.Repeat("a", 40))
	}
	b.WriteString("]}")
	dispatched := false
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost:
			dispatched = true
			return jsonResp(204, ""), nil
		case strings.Contains(r.URL.Path, "/runs"):
			return jsonResp(200, b.String()), nil
		case strings.HasSuffix(r.URL.Path, "/shop"):
			return jsonResp(200, `{"default_branch":"main"}`), nil
		}
		return jsonResp(200, `{}`), nil
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, 0, host{handle: "h"}, nil)
	_, _, err := s.Direct(t.Context(), rpc+"StartOperation", &pluginv1.ActionContext{IdempotencyKey: "key-1"}, &civ1.StartOperationRequest{Repository: "acme/shop", Pipeline: "release.yml", Revision: strings.Repeat("a", 40)})
	if got := reason(t, err); got != pluginv1.Reason_REASON_LIMIT_EXCEEDED || dispatched {
		t.Errorf("StartOperation = %v, dispatched = %v, want LIMIT_EXCEEDED and no dispatch", got, dispatched)
	}
}

// The lookup of the run is a query built with url.Values: the parameters of the request are exactly the ones the provider means.
func TestStartOperation_TheRunsQueryIsEncoded(t *testing.T) {
	t.Parallel()
	var seen string
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.Path, "/runs"):
			seen = r.URL.RawQuery
			return jsonResp(200, `{"workflow_runs":[]}`), nil
		case strings.HasSuffix(r.URL.Path, "/shop"):
			return jsonResp(200, `{"default_branch":"main&event=push"}`), nil
		}
		return jsonResp(200, `{}`), nil
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, 0, host{handle: "h"}, nil)
	_, _, _ = s.Direct(t.Context(), rpc+"StartOperation", &pluginv1.ActionContext{IdempotencyKey: "k", DryRun: true}, &civ1.StartOperationRequest{Repository: "acme/shop", Pipeline: "release.yml", Revision: strings.Repeat("a", 40)})
	q, err := url.ParseQuery(seen)
	if err != nil || len(q["event"]) != 1 || q["event"][0] != "workflow_dispatch" || q.Get("branch") != "main&event=push" {
		t.Errorf("runs query %q = %v, %v, want one event of workflow_dispatch and the branch as one value", seen, q, err)
	}
}
