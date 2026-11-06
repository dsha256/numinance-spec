package scmkit_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	scmkit "github.com/numinance/numinance-spec/conformance/scm/v1"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/provider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/replay"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	scmv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"
)

// testHost is the HostService of a launch as the kit needs it: it emits evidence and gives a handle the replay never uses.
type testHost struct{}

func (testHost) Credential(context.Context, string, string) (string, error) { return "kit-handle", nil }
func (testHost) Emit(context.Context, string, string, string, *structpb.Struct) (string, error) {
	return "ev_kit", nil
}

// direct reaches the provider with no host process: the kit's replay of the replaying build of the reference provider.
type direct struct{ s *provider.Server }

const rpc = "/numinance.ports.scm.v1.ScmService/"

func call[Resp proto.Message](ctx context.Context, d direct, method string, o scmkit.Options, req proto.Message) (Resp, error) {
	var zero Resp
	resp, _, err := d.s.Direct(ctx, rpc+method, &pluginv1.ActionContext{DryRun: o.DryRun, IdempotencyKey: o.Key}, req)
	var ce *provider.CallError
	if errors.As(err, &ce) {
		return zero, &scmkit.CallError{Reason: ce.Reason}
	}
	if err != nil {
		return zero, err
	}
	return resp.(Resp), nil //nolint:forcetypeassert // the method's own response type
}

func (d direct) GetRepository(ctx context.Context, o scmkit.Options, r *scmv1.GetRepositoryRequest) (*scmv1.GetRepositoryResponse, error) {
	return call[*scmv1.GetRepositoryResponse](ctx, d, "GetRepository", o, r)
}

func (d direct) GetBranchProtection(ctx context.Context, o scmkit.Options, r *scmv1.GetBranchProtectionRequest) (*scmv1.GetBranchProtectionResponse, error) {
	return call[*scmv1.GetBranchProtectionResponse](ctx, d, "GetBranchProtection", o, r)
}

func (d direct) OpenPullRequest(ctx context.Context, o scmkit.Options, r *scmv1.OpenPullRequestRequest) (*scmv1.OpenPullRequestResponse, error) {
	return call[*scmv1.OpenPullRequestResponse](ctx, d, "OpenPullRequest", o, r)
}

func (d direct) GetPullRequestStatus(ctx context.Context, o scmkit.Options, r *scmv1.GetPullRequestStatusRequest) (*scmv1.GetPullRequestStatusResponse, error) {
	return call[*scmv1.GetPullRequestStatusResponse](ctx, d, "GetPullRequestStatus", o, r)
}

func (d direct) MergePullRequest(ctx context.Context, o scmkit.Options, r *scmv1.MergePullRequestRequest) (*scmv1.MergePullRequestResponse, error) {
	return call[*scmv1.MergePullRequestResponse](ctx, d, "MergePullRequest", o, r)
}

func (d direct) CommentOnPullRequest(ctx context.Context, o scmkit.Options, r *scmv1.CommentOnPullRequestRequest) (*scmv1.CommentOnPullRequestResponse, error) {
	return call[*scmv1.CommentOnPullRequestResponse](ctx, d, "CommentOnPullRequest", o, r)
}

// TestKit_ReplayingGitHubProvider runs the kit against the replaying build of scm-github.
func TestKit_ReplayingGitHubProvider(t *testing.T) {
	t.Parallel()
	tr, err := replay.Transport()
	if err != nil {
		t.Fatal(err)
	}
	d := direct{provider.New(provider.ReplayPluginID, "sha256:0", tr, testHost{}, nil)}
	scmkit.Run(t, scmkit.Subject{
		Target: d, Repository: replay.Repository, Missing: "acme/missing", Protected: "main", Unprotected: "dev",
		Open:       scmkit.Candidate{Branch: "numinance/chg-0001", Revision: replay.Candidate, Base: "main", Title: "Fix the cart total"},
		DryRunOpen: scmkit.Candidate{Branch: "numinance/chg-0004", Revision: replay.Candidate, Base: "main", Title: "Dry"},
		Moved:      scmkit.Candidate{Branch: "numinance/chg-0003", Revision: replay.Candidate, Base: "main", Title: "Moved"},
		Status:     7, Merge: scmkit.MergeFixture{Number: 8, Head: replay.Candidate, Base: "main"}, MovedHead: scmkit.MergeFixture{Number: 9, Head: replay.Candidate, Base: "main"},
		Closed: scmkit.MergeFixture{Number: 10, Head: replay.Candidate, Base: "main"}, CommentTarget: 7, CommentKey: "chg_0001/comment/1", CommentContent: "Checks passed.",
	})
}
