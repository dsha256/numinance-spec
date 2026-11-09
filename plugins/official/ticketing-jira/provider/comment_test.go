package provider_test

import (
	"net/http"
	"strings"
	"testing"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
)

// The property that records a comment is written by anyone who can edit the issue. The call answers Existing only when the
// comment it names is on the issue, was written by the account of the credential in use and says what was asked; otherwise it
// fails closed and posts nothing (review finding of PR 582).
func TestComment_ExistingNeedsTheRecordedCommentToMatch(t *testing.T) {
	const doc = `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"line one"}]},{"type":"paragraph","content":[{"type":"text","text":"line two"}]}]}`
	name := provider.CommentProperty("k1")
	issue := issueWith(`"properties":{"` + name + `":{"commentId":"77"}}`)
	comment := func(id, account, body string) string {
		return `{"id":"` + id + `","author":{"accountId":"` + account + `"},"body":` + body + `}`
	}
	tests := []struct {
		name    string
		comment string // the answer for the recorded comment; "" is a 404
		myself  int
		want    pluginv1.Reason // UNSPECIFIED means Existing
	}{
		{"the comment is ours and says what was asked", comment("77", "me", doc), 200, pluginv1.Reason_REASON_UNSPECIFIED},
		{"another account wrote it", comment("77", "mallory", doc), 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"the text differs", comment("77", "me", strings.Replace(doc, "line two", "line 2", 1)), 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"an extra paragraph", comment("77", "me", strings.Replace(doc, `]}]}`, `]},{"type":"paragraph"}]}`, 1)), 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"a mention in the text", comment("77", "me", strings.Replace(doc, `{"type":"text","text":"line one"}`, `{"type":"mention","attrs":{"id":"x"}}`, 1)), 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"a mark on the text", comment("77", "me", strings.Replace(doc, `"text":"line one"`, `"text":"line one","marks":[{"type":"strong"}]`, 1)), 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"the answer names another comment", comment("78", "me", doc), 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"no author", `{"id":"77","body":` + doc + `}`, 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"the comment is gone", "", 200, pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"the tracker cannot say who we are", comment("77", "me", doc), 503, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotComment, gotMyself bool
			rt := rtFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					t.Errorf("a %s request %s was made: an unverified record must post nothing", r.Method, r.URL.Path)
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/comment/77"):
					gotComment = true
					if tc.comment == "" {
						return answer(404, `{}`), nil
					}
					return answer(200, tc.comment), nil
				case strings.HasSuffix(r.URL.Path, "/myself"):
					gotMyself = true
					return answer(tc.myself, `{"accountId":"me"}`), nil
				}
				return answer(200, issue), nil
			})
			resp, err := run(over(t, rt), "CommentOnIssue", ticketingkit.Options{Key: "k1"},
				&ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: "line one\nline two"})
			if !gotComment {
				t.Error("the recorded comment was never fetched: the property alone was believed")
			}
			if tc.want == pluginv1.Reason_REASON_UNSPECIFIED {
				got, _ := resp.(*ticketingv1.CommentOnIssueResponse)
				if err != nil || !got.GetExisting() || got.GetCommentId() != "77" || !gotMyself {
					t.Errorf("CommentOnIssue = %v, %v, want existing comment 77", got, err)
				}
				return
			}
			if got := failed(t, err).Reason; got != tc.want {
				t.Errorf("CommentOnIssue = %s, want %s", got, tc.want)
			}
		})
	}
}
