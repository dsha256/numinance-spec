package provider_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	scmv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"

	"github.com/numinance/numinance-spec/plugins/official/scm-github/provider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/replay"
)

func pullBody(state string, extra string) string {
	return fmt.Sprintf(`{"number":7,"state":%q,"html_url":"https://github.com/acme/shop/pull/7","mergeable":true,"user":{"login":"author-bot"},"head":{"ref":"c","sha":%q},"base":{"ref":"main"}%s}`, state, strings.Repeat("a", 40), extra)
}

// More reviews than one page holds are read page by page; when the pages run out of bound the call is refused, never cut.
func TestGetPullRequestStatus_ReviewsArePagedOrRefused(t *testing.T) {
	t.Parallel()
	head := strings.Repeat("a", 40)
	var pages []string
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/pulls/7"):
			return answer(pullBody("open", "")), nil
		case strings.HasSuffix(p, "/check-runs"):
			return answer(`{"total_count":0,"check_runs":[]}`), nil
		case strings.HasSuffix(p, "/reviews"):
			pages = append(pages, r.URL.Query().Get("page"))
			var b strings.Builder
			b.WriteString("[")
			for i := range 100 {
				if i > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, `{"user":{"login":"u%d"},"state":"COMMENTED","commit_id":%q}`, i, head)
			}
			b.WriteString("]")
			return answer(b.String()), nil
		}
		return nil, fmt.Errorf("unexpected %s", p)
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, &recordingHost{handle: "h"}, nil)
	_, _, err := s.Direct(t.Context(), rpc+"GetPullRequestStatus", &pluginv1.ActionContext{}, &scmv1.GetPullRequestStatusRequest{Repository: replay.Repository, Number: 7})
	if got := reason(t, err); got != pluginv1.Reason_REASON_LIMIT_EXCEEDED || len(pages) < 2 {
		t.Errorf("GetPullRequestStatus = %v after pages %v, want pages read and then LIMIT_EXCEEDED", got, pages)
	}
}

// A changes-requested review on the second page is seen.
func TestGetPullRequestStatus_ChangesRequestedOnALaterPageIsSeen(t *testing.T) {
	t.Parallel()
	head := strings.Repeat("a", 40)
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/pulls/7"):
			return answer(pullBody("open", "")), nil
		case strings.HasSuffix(p, "/check-runs"):
			return answer(`{"total_count":0,"check_runs":[]}`), nil
		case strings.HasSuffix(p, "/reviews"):
			if r.URL.Query().Get("page") == "2" {
				return answer(fmt.Sprintf(`[{"user":{"login":"late"},"state":"CHANGES_REQUESTED","commit_id":%q}]`, head)), nil
			}
			var b strings.Builder
			b.WriteString("[")
			for i := range 100 {
				if i > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, `{"user":{"login":"u%d"},"state":"COMMENTED","commit_id":%q}`, i, head)
			}
			b.WriteString("]")
			return answer(b.String()), nil
		}
		return nil, fmt.Errorf("unexpected %s", p)
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, &recordingHost{handle: "h"}, nil)
	resp, _, err := s.Direct(t.Context(), rpc+"GetPullRequestStatus", &pluginv1.ActionContext{}, &scmv1.GetPullRequestStatusRequest{Repository: replay.Repository, Number: 7})
	if err != nil || !resp.(*scmv1.GetPullRequestStatusResponse).GetChangesRequested() {
		t.Errorf("GetPullRequestStatus = %v, %v, want changes_requested from page 2", resp, err)
	}
}

// Only approvals of reviewers who hold write access and are not the pull request's author count.
func TestGetPullRequestStatus_CountsOnlyWriteApprovalsOfOthersThanTheAuthor(t *testing.T) {
	t.Parallel()
	head := strings.Repeat("a", 40)
	perms := map[string]string{"writer": `{"permission":"write"}`, "reader": `{"permission":"read"}`}
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/pulls/7"):
			return answer(pullBody("open", "")), nil
		case strings.HasSuffix(p, "/check-runs"):
			return answer(`{"total_count":0,"check_runs":[]}`), nil
		case strings.HasSuffix(p, "/reviews"):
			var b strings.Builder
			b.WriteString("[")
			for i, who := range []string{"writer", "reader", "author-bot", "stranger"} {
				if i > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, `{"user":{"login":%q},"state":"APPROVED","commit_id":%q}`, who, head)
			}
			b.WriteString("]")
			return answer(b.String()), nil
		case strings.Contains(p, "/collaborators/"):
			who := strings.TrimSuffix(p[strings.LastIndex(strings.TrimSuffix(p, "/permission"), "/")+1:], "/permission")
			if body, ok := perms[who]; ok {
				return answer(body), nil
			}
			return &http.Response{StatusCode: 404, Header: http.Header{}, Body: http.NoBody}, nil
		}
		return nil, fmt.Errorf("unexpected %s", p)
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, &recordingHost{handle: "h"}, nil)
	resp, _, err := s.Direct(t.Context(), rpc+"GetPullRequestStatus", &pluginv1.ActionContext{}, &scmv1.GetPullRequestStatusRequest{Repository: replay.Repository, Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.(*scmv1.GetPullRequestStatusResponse).GetApprovals(); got != 1 {
		t.Errorf("approvals = %d, want 1: the reader, the author and the stranger do not count", got)
	}
}

// A branch name with '&' or '=' is legal and must stay one value of the lookup query.
func TestOpenPullRequest_TheLookupQueryIsEncoded(t *testing.T) {
	t.Parallel()
	var seen string
	head := strings.Repeat("a", 40)
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/pulls") && r.Method == http.MethodGet {
			seen = r.URL.RawQuery
			return answer(`[]`), nil
		}
		if strings.Contains(r.URL.Path, "/git/ref/heads/") {
			return answer(fmt.Sprintf(`{"object":{"sha":%q}}`, head)), nil
		}
		return nil, fmt.Errorf("stop at %s", r.URL.Path)
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, &recordingHost{handle: "h"}, nil)
	_, _, err := s.Direct(t.Context(), rpc+"OpenPullRequest", &pluginv1.ActionContext{IdempotencyKey: "k", DryRun: true},
		&scmv1.OpenPullRequestRequest{Repository: replay.Repository, HeadBranch: "x&state=all&base=release", HeadRevision: head, BaseBranch: "main", Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	q, err := url.ParseQuery(seen)
	if err != nil || len(q["state"]) != 1 || q["state"][0] != "open" || len(q["base"]) != 1 || q["base"][0] != "main" || q.Get("head") != replay.Repository[:4]+":x&state=all&base=release" {
		t.Errorf("lookup query %q = %v, %v, want one state open, one base main and the whole branch name in head", seen, q, err)
	}
}

// A pull request merged at a head other than the verified one, or into another base, is never answered as the release.
func TestMergePullRequest_AMergedPullRequestIsBoundToTheVerifiedHeadAndBase(t *testing.T) {
	t.Parallel()
	verified := strings.Repeat("a", 40)
	cases := []struct {
		name, body string
		base       string
		reason     pluginv1.Reason
	}{
		{"merged at another head", fmt.Sprintf(`{"number":7,"state":"closed","merged":true,"merged_at":"2026-11-01T10:00:00Z","merge_commit_sha":%q,"html_url":"https://github.com/acme/shop/pull/7","head":{"ref":"c","sha":%q},"base":{"ref":"main"}}`, strings.Repeat("c", 40), strings.Repeat("b", 40)), "main", pluginv1.Reason_REASON_CONFLICT},
		{"merged into another base", fmt.Sprintf(`{"number":7,"state":"closed","merged":true,"merged_at":"2026-11-01T10:00:00Z","merge_commit_sha":%q,"html_url":"https://github.com/acme/shop/pull/7","head":{"ref":"c","sha":%q},"base":{"ref":"staging"}}`, strings.Repeat("c", 40), verified), "main", pluginv1.Reason_REASON_CONFLICT},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := roundTripFunc(func(*http.Request) (*http.Response, error) { return answer(tc.body), nil })
			s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: tr}, &recordingHost{handle: "h"}, nil)
			_, _, err := s.Direct(t.Context(), rpc+"MergePullRequest", &pluginv1.ActionContext{}, &scmv1.MergePullRequestRequest{Repository: replay.Repository, Number: 7, ExpectedHeadRevision: verified, ExpectedBaseBranch: tc.base})
			if got := reason(t, err); got != tc.reason {
				t.Errorf("MergePullRequest(%s) = %v, want %v", tc.name, got, tc.reason)
			}
		})
	}
}
