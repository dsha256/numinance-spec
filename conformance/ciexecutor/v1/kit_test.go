package cikit_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	cikit "github.com/numinance/numinance-spec/conformance/ciexecutor/v1"
	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/provider"
	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/replay"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"
)

type testHost struct{}

func (testHost) Credential(context.Context, string, string) (string, error) { return "kit-handle", nil }
func (testHost) Emit(context.Context, string, string, string, *structpb.Struct) (string, error) {
	return "ev_kit", nil
}

// direct reaches the provider with no host process: the kit's replay of the replaying build of the reference provider.
type direct struct{ s *provider.Server }

const rpc = "/numinance.ports.ciexecutor.v1.CiExecutorService/"

func call[Resp proto.Message](ctx context.Context, d direct, method string, o cikit.Options, req proto.Message) (Resp, error) {
	var zero Resp
	resp, _, err := d.s.Direct(ctx, rpc+method, &pluginv1.ActionContext{DryRun: o.DryRun, IdempotencyKey: o.Key}, req)
	var ce *provider.CallError
	if errors.As(err, &ce) {
		return zero, &cikit.CallError{Reason: ce.Reason}
	}
	if err != nil {
		return zero, err
	}
	return resp.(Resp), nil //nolint:forcetypeassert // the method's own response type
}

func (d direct) StartOperation(ctx context.Context, o cikit.Options, r *civ1.StartOperationRequest) (*civ1.StartOperationResponse, error) {
	return call[*civ1.StartOperationResponse](ctx, d, "StartOperation", o, r)
}

func (d direct) GetOperation(ctx context.Context, o cikit.Options, r *civ1.GetOperationRequest) (*civ1.GetOperationResponse, error) {
	return call[*civ1.GetOperationResponse](ctx, d, "GetOperation", o, r)
}

func (d direct) CancelOperation(ctx context.Context, o cikit.Options, r *civ1.CancelOperationRequest) (*civ1.CancelOperationResponse, error) {
	return call[*civ1.CancelOperationResponse](ctx, d, "CancelOperation", o, r)
}

func (d direct) ListArtifacts(ctx context.Context, o cikit.Options, r *civ1.ListArtifactsRequest) (*civ1.ListArtifactsResponse, error) {
	return call[*civ1.ListArtifactsResponse](ctx, d, "ListArtifacts", o, r)
}

func (d direct) GetLogReference(ctx context.Context, o cikit.Options, r *civ1.GetLogReferenceRequest) (*civ1.GetLogReferenceResponse, error) {
	return call[*civ1.GetLogReferenceResponse](ctx, d, "GetLogReference", o, r)
}

// TestKit_ReplayingGitHubActionsProvider runs the kit against the replaying build of ci-github-actions.
func TestKit_ReplayingGitHubActionsProvider(t *testing.T) {
	t.Parallel()
	tr, err := replay.Transport()
	if err != nil {
		t.Fatal(err)
	}
	d := direct{provider.New(provider.ReplayPluginID, "sha256:0", tr, 0, testHost{}, nil)}
	cikit.Run(t, cikit.Subject{
		Target: d, Repository: replay.Repository, Missing: "acme/missing", Pipeline: "release.yml", DryRunPipeline: "dry.yml",
		Revision: replay.Revision, StartKey: "chg_0001/release/1", Params: map[string]string{"environment": "staging"},
		Progressing: "9005", Cancellable: "9003", WithArtifacts: "9006",
	})
}
