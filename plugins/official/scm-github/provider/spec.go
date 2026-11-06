// Package provider is the scm provider for GitHub: a capability provider of numinance.ports.scm.v1 over the GitHub REST
// API. It is written against the public SDK only. It holds no credential: it asks the host for a handle and sends it to the
// egress proxy, which injects the token. Everything the code host answers is untrusted data, size-limited before it is
// decoded, checked against the grammar of scm.proto and never repaired or truncated.
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	scmv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

const (
	// PluginID is the extension id of the provider that talks to GitHub through the egress proxy.
	PluginID = "dev.numinance.scm-github"
	// ReplayPluginID is the extension id of the replaying build: the same provider over recorded, redacted exchanges.
	ReplayPluginID = "dev.numinance.scm-github-replay"
	// Intent is the credential intent the manifest declares for every capability; the broker catalog resolves it.
	Intent = "scm.github.api"
	// RecordType is the evidence record type OpenPullRequest declares.
	RecordType = "pr-ref"
	service    = "/numinance.ports.scm.v1.ScmService/"
)

const (
	sideWrite      = pluginv1.SideEffect_SIDE_EFFECT_WRITE
	notFoundReason = pluginv1.Reason_REASON_NOT_FOUND
	limitReason    = pluginv1.Reason_REASON_LIMIT_EXCEEDED
	conflictReason = pluginv1.Reason_REASON_CONFLICT
)

func isStatus(err error, status int) bool {
	var ae *ghprovider.APIError
	return errors.As(err, &ae) && ae.Status == status
}

func asAPI(err error, target **ghprovider.APIError) bool { return errors.As(err, target) }

func shortDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

// prRef is the pr-ref receipt: the reference of the pull request, never its title or body.
func prRef(repository string, pr *scmv1.PullRequest) *structpb.Struct {
	return &structpb.Struct{Fields: map[string]*structpb.Value{
		"repository":    structpb.NewStringValue(repository),
		"number":        structpb.NewNumberValue(float64(pr.GetNumber())),
		"head_revision": structpb.NewStringValue(pr.GetHeadRevision()),
		"base_branch":   structpb.NewStringValue(pr.GetBaseBranch()),
	}}
}

// receipt submits the compact receipt of a write that was made: the capability, the repository and one reference, never text a
// caller or the host wrote. A submission that fails leaves the call PARTIAL, which the release context treats as no receipt.
func receipt(ctx context.Context, c *ghprovider.Call, capability, repository, ref string) {
	_ = c.Emit(ctx, "action", scmAction, &structpb.Struct{Fields: map[string]*structpb.Value{
		"capability": structpb.NewStringValue(capability),
		"repository": structpb.NewStringValue(repository),
		"ref":        structpb.NewStringValue(ref),
	}})
}

// typed adapts a handler over concrete messages to ghprovider.Method.Do.
func typed[Req, Resp any, PReq interface {
	*Req
	proto.Message
}, PResp interface {
	*Resp
	proto.Message
}](f func(context.Context, *ghprovider.Call, PReq) (PResp, error)) func(context.Context, *ghprovider.Call, proto.Message) (proto.Message, error) {
	return func(ctx context.Context, c *ghprovider.Call, m proto.Message) (proto.Message, error) {
		resp, err := f(ctx, c, m.(PReq)) //nolint:forcetypeassert // New builds exactly this request type
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
}

func method[Req, Resp any, PReq interface {
	*Req
	proto.Message
}, PResp interface {
	*Resp
	proto.Message
}](name, rpc string, capability ghprovider.Capability, f func(context.Context, *ghprovider.Call, PReq) (PResp, error)) ghprovider.Method {
	capability.Name = name
	return ghprovider.Method{
		Path: service + rpc, Cap: capability, New: func() proto.Message { return PReq(new(Req)) }, Do: typed[Req, Resp](f),
	}
}

const (
	read  = pluginv1.SideEffect_SIDE_EFFECT_READ
	write = pluginv1.SideEffect_SIDE_EFFECT_WRITE
	// scmAction is the record type a side-effecting capability with no receipt of its own declares: the host's rule asks every
	// side-effecting capability to declare evidence. No record of it is required, and none is emitted.
	scmAction = "scm-action"
)

// NewSpec returns the provider's spec over the given transport.
func NewSpec(id string, t ghprovider.Transport) ghprovider.Spec {
	natural, keyed := pluginv1.Idempotency_IDEMPOTENCY_NATURAL, pluginv1.Idempotency_IDEMPOTENCY_KEYED
	ro := func(llm string) ghprovider.Capability {
		return ghprovider.Capability{Risk: annotationsv1.RiskClass_RISK_CLASS_READ, SideEffect: read, Idempotency: natural, LLM: llm}
	}
	return ghprovider.Spec{
		ID: id, Version: "1.0.0", Port: "numinance.ports.scm.v1", File: scmv1.File_numinance_ports_scm_v1_scm_proto,
		Intent: Intent, NetworkHost: "api.github.com", Suite: "conformance/scm/v1/basic", Transport: t, Timeout: 60 * time.Second,
		Methods: []ghprovider.Method{
			method("scm.get_repository", "GetRepository", ro("Returns the default branch of a repository and the commit it points to."), getRepository),
			method("scm.get_tree", "GetTree", ro("Lists the entries of a directory of a repository at a commit."), getTree),
			method("scm.get_file", "GetFile", ro("Returns the content of one file of a repository at a commit."), getFile),
			method("scm.list_branches", "ListBranches", ro("Lists the branches of a repository."), listBranches),
			method("scm.get_branch_protection", "GetBranchProtection", ro("Reports the branch protection of a branch of a repository."), getBranchProtection),
			method("scm.get_pull_request_status", "GetPullRequestStatus", ro("Returns the state, checks and reviews of a pull request."), getPullRequestStatus),
			method("scm.search_code", "SearchCode", ro("Searches the code of a repository and returns matching paths."), searchCode),
			method("scm.create_branch", "CreateBranch", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_LOW, SideEffect: write, Idempotency: keyed, Evidence: []string{scmAction},
				LLM: "Creates a branch of a repository at a given commit.",
			}, createBranch),
			method("scm.open_pull_request", "OpenPullRequest", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_MEDIUM, SideEffect: write, Idempotency: keyed, DryRun: true,
				Compensation: "scm.update_pull_request", Evidence: []string{RecordType}, LLM: "Opens a pull request for a verified candidate branch.",
			}, openPullRequest),
			method("scm.update_pull_request", "UpdatePullRequest", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_LOW, SideEffect: write, Idempotency: keyed, Evidence: []string{scmAction},
				LLM: "Edits the title or body of a pull request, or closes it.",
			}, updatePullRequest),
			method("scm.comment_on_pull_request", "CommentOnPullRequest", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_LOW, SideEffect: write, Idempotency: keyed, Evidence: []string{scmAction},
				LLM: "Adds one comment to a pull request.",
			}, commentOnPullRequest),
			method("scm.merge_pull_request", "MergePullRequest", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_PRIVILEGED, SideEffect: write, Idempotency: keyed, DryRun: true,
				Compensation: "scm.comment_on_pull_request", Evidence: []string{scmAction}, LLM: "Merges a verified pull request into its base branch.",
			}, mergePullRequest),
		},
	}
}

// New returns the server of the provider.
func New(id, digest string, t ghprovider.Transport, host ghprovider.Host, stop func()) *ghprovider.Server {
	return ghprovider.New(NewSpec(id, t), digest, host, stop)
}

// The types of the shared skeleton a caller of this package names: aliases, so a harness outside plugins/official (the
// conformance kit, the host-level acceptance) needs nothing of the internal package.
type (
	// Server is the PluginService of the provider.
	Server = ghprovider.Server
	// Host is what a call needs of the HostService of its launch.
	Host = ghprovider.Host
	// Transport says how the provider reaches the code host.
	Transport = ghprovider.Transport
	// CallError is a call that ends FAILED with one reason.
	CallError = ghprovider.CallError
)

// ExecutableDigest returns "sha256:" plus the hex digest of the running executable.
func ExecutableDigest() (string, error) { return ghprovider.ExecutableDigest() }
