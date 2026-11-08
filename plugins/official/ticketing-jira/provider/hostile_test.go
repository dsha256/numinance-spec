package provider_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
)

func issueWith(extra string) string {
	return `{"key":"PAY-1","fields":{"summary":"T","project":{"key":"PAY"},"status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":["ok"]},` + extra + `}`
}

// The properties of an issue are written by anyone who can edit it: the operation ids are read leniently and bounded, and a
// comment record that is not a token ends the call UPSTREAM_ERROR rather than flowing into evidence.
func TestOperationIDs_AreBoundedAndNeverRepaired(t *testing.T) {
	long := strings.Repeat("o", 129)
	many := make([]string, 70)
	for i := range many {
		many[i] = fmt.Sprintf(`"op_%d"`, i)
	}
	tests := []struct {
		name, prop string
		want       int
	}{
		{"a list of strings", `["op_1","op_2"]`, 2},
		{"not a list", `{"a":1}`, 0},
		{"a number", `7`, 0},
		{"a list holding a number", `["op_1",2]`, 0},
		{"an id over 128 bytes is dropped", `["op_1","` + long + `"]`, 1},
		{"an empty id after the strip is dropped", `["op_1","\u200b"]`, 1},
		{"more than 64 are cut", "[" + strings.Join(many, ",") + "]", 64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt := rtFunc(func(*http.Request) (*http.Response, error) {
				return answer(200, issueWith(`"properties":{"numinance.operation_ids":`+tc.prop+`}`)), nil
			})
			resp, err := run(over(t, rt), "GetIssue", ticketingkit.Options{}, getReq("PAY", "PAY-1"))
			if err != nil {
				t.Fatal(err)
			}
			got := resp.(*ticketingv1.GetIssueResponse) //nolint:forcetypeassert // the method's own response type
			if len(got.GetOperationIds()) != tc.want {
				t.Errorf("operation ids = %d, want %d", len(got.GetOperationIds()), tc.want)
			}
			if e := ticketingkit.ValidateIssue(got, 4096); e != nil {
				t.Error(e)
			}
		})
	}
}

func TestComment_ARecordThatIsNotATokenIsRefused(t *testing.T) {
	name := provider.CommentProperty("k1")
	for _, prop := range []string{`{"commentId":"../x"}`, `{"commentId":""}`, `{"commentId":7}`, `"x"`, `{"commentId":"` + strings.Repeat("1", 65) + `"}`} {
		rt := rtFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Errorf("a %s request was made after a record that is not a token", r.Method)
			}
			return answer(200, issueWith(`"properties":{"`+name+`":`+prop+`}`)), nil
		})
		_, err := run(over(t, rt), "CommentOnIssue", ticketingkit.Options{Key: "k1"}, &ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: "x"})
		if got := failed(t, err).Reason; got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
			t.Errorf("CommentOnIssue with the record %s = %s, want UPSTREAM_ERROR", prop, got)
		}
	}
}

func TestComment_RefusesBodiesThatCouldDisguiseThemselves(t *testing.T) {
	rt := rtFunc(func(*http.Request) (*http.Response, error) {
		t.Error("a request was made")
		return nil, errors.New("no request expected")
	})
	srv := over(t, rt)
	for _, body := range []string{"", "a\x00b", "a\u202eb", "a\u200bb", "a\rb", strings.Repeat("a", 8193), "a\xffb"} {
		_, err := run(srv, "CommentOnIssue", ticketingkit.Options{Key: "k"}, &ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: body})
		if got := failed(t, err).Reason; got != pluginv1.Reason_REASON_INVALID_ARGUMENT {
			t.Errorf("CommentOnIssue(%q) = %s, want INVALID_ARGUMENT", body, got)
		}
	}
	_, err := run(srv, "CommentOnIssue", ticketingkit.Options{}, &ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: "x"})
	if got := failed(t, err).Reason; got != pluginv1.Reason_REASON_INVALID_ARGUMENT {
		t.Errorf("CommentOnIssue with no idempotency key = %s, want INVALID_ARGUMENT", got)
	}
}

// The evidence of a write is the compact receipt of the port and nothing else, whether the call made the change or found it made.
func TestEvidence_IsAReceiptOfFourFields(t *testing.T) {
	tg := newTarget(t)
	ctx := t.Context()
	secret := "kit-secret-comment-body-qzxj"
	for range 2 {
		_, ev, err := tg.CommentOnIssue(ctx, ticketingkit.Options{Key: "kit-comment-1"}, &ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: secret})
		if err != nil || len(ev) != 1 || ev[0].Type != "ticketing-action" || len(ev[0].Fields) != 4 {
			t.Fatalf("CommentOnIssue = %v, %v, want one ticketing-action record of four fields", ev, err)
		}
		if strings.Contains(strings.Join(ev[0].Fields, " "), "qzxj") || strings.HasPrefix(ev[0].Fields[0], "key=kit") {
			t.Errorf("the record %v holds the comment text or the host key", ev[0].Fields)
		}
	}
}

// A comment that the replay cannot place (an issue with no recorded property) never reports success.
func TestComment_AnUnrecordedPropertyFailsClosed(t *testing.T) {
	tg := newTarget(t)
	_, _, err := tg.CommentOnIssue(t.Context(), ticketingkit.Options{Key: "never-recorded"}, &ticketingv1.CommentOnIssueRequest{Project: "PAY", IssueKey: "PAY-1", Body: "x"})
	var ce *ticketingkit.CallError
	if !errors.As(err, &ce) || ce.Reason != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
		t.Errorf("CommentOnIssue with an unrecorded key = %v, want UPSTREAM_UNAVAILABLE", err)
	}
}
