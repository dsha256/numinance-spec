package provider_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/provider"
	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/replay"
)

type host struct{ handle string }

func (h host) Credential(context.Context, string, string) (string, error) { return h.handle, nil }
func (host) Emit(context.Context, string, string, string, *structpb.Struct) (string, error) {
	return "ev_1", nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const rpc = "/numinance.ports.ciexecutor.v1.CiExecutorService/"

func reason(t *testing.T, err error) pluginv1.Reason {
	t.Helper()
	var ce *provider.CallError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want a FAILED call", err)
	}
	return ce.Reason
}

func replayServer(t *testing.T) *provider.Server {
	t.Helper()
	tr, err := replay.Transport()
	if err != nil {
		t.Fatal(err)
	}
	return provider.New(provider.ReplayPluginID, "sha256:0", tr, 0, host{}, nil)
}

func TestProvider_GetOperation_Endings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		id   string
		want civ1.OperationState
	}{
		{"a failure is a failure", "9002", civ1.OperationState_OPERATION_STATE_FAILED},
		{"a timeout is a timeout", "9007", civ1.OperationState_OPERATION_STATE_TIMED_OUT},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, _, err := replayServer(t).Direct(t.Context(), rpc+"GetOperation", &pluginv1.ActionContext{}, &civ1.GetOperationRequest{Repository: replay.Repository, OperationId: tc.id})
			if err != nil {
				t.Fatal(err)
			}
			if got := resp.(*civ1.GetOperationResponse).GetState(); got != tc.want {
				t.Errorf("GetOperation(%s) state = %s, want %s", tc.id, got, tc.want)
			}
		})
	}
}

func TestProvider_GetOperation_ARunOutsideTheContractIsNotAnOperation(t *testing.T) {
	t.Parallel()
	_, _, err := replayServer(t).Direct(t.Context(), rpc+"GetOperation", &pluginv1.ActionContext{}, &civ1.GetOperationRequest{Repository: replay.Repository, OperationId: "9004"})
	if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
		t.Errorf("GetOperation of a run named by someone else = %s, want UPSTREAM_ERROR: a run that does not carry the operation is not trusted", got)
	}
}

func TestProvider_NoRecordingFailsClosed(t *testing.T) {
	t.Parallel()
	_, _, err := replayServer(t).Direct(t.Context(), rpc+"GetOperation", &pluginv1.ActionContext{}, &civ1.GetOperationRequest{Repository: replay.Repository, OperationId: "123456"})
	if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
		t.Errorf("GetOperation of an unrecorded run = %s, want UPSTREAM_UNAVAILABLE", got)
	}
}

func TestProvider_StartOperation_DispatchesOnTheDefaultBranchNotTheCandidate(t *testing.T) {
	t.Parallel()
	var dispatched string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		answer := func(status int, body string) *http.Response {
			return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
		}
		switch {
		case r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			dispatched = string(b)
			return answer(204, ""), nil
		case strings.Contains(r.URL.Path, "/runs") && dispatched == "":
			return answer(200, `{"workflow_runs":[]}`), nil
		case strings.Contains(r.URL.Path, "/runs"):
			return answer(200, `{"workflow_runs":[{"id":77,"display_title":"numinance df787cc1f9bd6c916e2bda8c0698c869 `+replay.Revision+`","status":"queued","head_branch":"trunk","path":".github/workflows/release.yml@refs/heads/trunk"}]}`), nil
		case strings.HasSuffix(r.URL.Path, "/shop"):
			return answer(200, `{"default_branch":"trunk"}`), nil
		}
		return answer(200, `{}`), nil
	})
	s := provider.New(provider.PluginID, "sha256:0", provider.Transport{RT: rt}, 0, host{handle: "h"}, nil)
	req := &civ1.StartOperationRequest{Repository: replay.Repository, Pipeline: "release.yml", Revision: replay.Revision}
	resp, _, err := s.Direct(t.Context(), rpc+"StartOperation", &pluginv1.ActionContext{IdempotencyKey: "chg_0001/release/1"}, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.(*civ1.StartOperationResponse).GetOperationId() != "77" {
		t.Errorf("operation id = %q, want 77", resp.(*civ1.StartOperationResponse).GetOperationId())
	}
	// The pipeline definition comes from the default branch, which the candidate cannot change; the candidate is an input.
	if !strings.Contains(dispatched, `"ref":"trunk"`) || !strings.Contains(dispatched, `"numinance_revision":"`+replay.Revision+`"`) {
		t.Errorf("dispatch body = %s, want ref trunk and the revision as an input", dispatched)
	}
}

func TestProvider_Manifest_StartOperationIsHighAndDeclaresCiRunRef(t *testing.T) {
	t.Parallel()
	m, err := replayServer(t).GetManifest(t.Context(), &pluginv1.GetManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.GetManifest().GetCapabilities() {
		if c.GetName() != "ciexecutor.start_operation" {
			continue
		}
		if c.GetRiskFloor().String() != "RISK_CLASS_HIGH" || len(c.GetEvidenceDeclared()) != 1 || c.GetEvidenceDeclared()[0] != "ci-run-ref" || c.GetSideEffect() != pluginv1.SideEffect_SIDE_EFFECT_DEPLOY {
			t.Errorf("start_operation = risk %s evidence %v side effect %s, want HIGH, [ci-run-ref], DEPLOY", c.GetRiskFloor(), c.GetEvidenceDeclared(), c.GetSideEffect())
		}
		return
	}
	t.Error("no ciexecutor.start_operation capability")
}
