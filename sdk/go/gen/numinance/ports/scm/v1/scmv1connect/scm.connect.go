// The scm port: reads a source repository and its pull requests, and opens, updates, comments on and merges pull requests. Any code host that has repositories, branches, pull requests, status checks and branch protection can implement every core verb (GitHub, GitLab, Bitbucket, Gitea); a vendor feature beyond them is a capability `scm.<feature>` negotiated at handshake, added in a later minor; there is no VendorCall in 1.0.0, because the host refuses a provider that does not declare every side-effecting RPC of its port, and an escape hatch no provider implements is a PRIVILEGED capability that does nothing.
// Callers: the release context calls the write verbs, the profile and policy side read the base revision at intake through GetRepository, and the knowledge side may read files and trees. The host decides every call, on the exact request bytes, against the grant of the identity named in ActionContext: OpenPullRequest, UpdatePullRequest, CommentOnPullRequest and CreateBranch under the author-agent identity of the Change, MergePullRequest under the release-pipeline identity only. A call that comes from an identity that is neither, and a MergePullRequest from the author of the Change (the same principal or the same run_id as any author run), is refused by the policy floors before Invoke; the provider never decides who may merge. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: every request names its repository in `repository`, which the Engine marks as the resource of the call (RFC-0002): a call on a repository outside the acting grant's resource scope is denied before dispatch. A provider holds no credential of its own: it asks the host for a credential handle for the intent its manifest declares and sends the handle to the egress proxy, never a raw token. A provider never reports an effect it did not make, and reconciles a retry before it acts: the host's idempotency key names the operation, and a replay answers the first result (an open pull request for the same head and base is returned, not opened again; a merge already done is answered as done).
// Untrusted answers: every title, body, branch name, label, URL, check name, check output, commit message and file content that the host holds comes from the code host and is untrusted data, whoever wrote it on the host. The release context size-limits it, validates it against this file, stores it by digest and labels and delimits it as untrusted wherever it reaches a prompt, a ticket or a pull request, never as instructions. Nothing a provider answers changes policy, a grant, a tool list or an approval. A merge decision never rests on a field of a response that a pull request author controls (the text of a body or a comment).
// Protected paths: branch protection is the second line behind the mount. GetBranchProtection reports what the host enforces so that the release context can refuse to merge a pull request whose base branch has no required review or no required check; the provider reports the host's state and never changes it, and no verb of this port edits protection, a workflow file or a ruleset.
// Payloads: requests carry ids, names and bounded text; files are returned inline only up to max_bytes (a repository file is not a Reference because the scm host is the store). No verb returns a diff.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; NOT_FOUND for a repository, revision, branch, file or pull request the host hides or lacks; PERMISSION_DENIED when the host refuses the credential openly; ALREADY_EXISTS for a CreateBranch whose name is taken at another revision; FAILED_PRECONDITION for a pull request that is not mergeable, not open or whose head moved from head_revision, and for a merge the base branch's protection refuses; RESOURCE_EXHAUSTED when the host rate-limits the call; UNAVAILABLE for a host that cannot be reached or a transient failure the caller may retry. A provider never invents a result: an outage is an error, never an empty answer.
// Evidence: OpenPullRequest declares one record type, pr-ref: a provider emits exactly one per call that opened or found the pull request, through HostService.EmitEvidence, a compact receipt of the pull request's reference (repository, number, head and base revisions, never the title or body). A call that ends SUCCEEDED without it is PARTIAL. The other verbs declare no evidence in 1.0.0: the merge's record is the host's capability record, and the release context writes its own facts as events.
// Bounds: every string is at most 1024 bytes unless stated; over-limit input is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding), so a provider sends a field added in a later minor only when HandshakeRequest.spec_version is at least that minor or behind a negotiated capability. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Formats: repository is the repository's full name `<owner>/<name>`, matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,99}/[A-Za-z0-9._-]{1,100}$ with no '.' or '..' name segment. A revision is a full commit id: 40 or 64 lowercase hexadecimal characters; a branch name or a tag never stands for a revision, because it moves. A branch name is valid under git check-ref-format --branch, 1 to 255 bytes, printable ASCII. A path is relative to the repository root, '/'-separated, printable, with no leading '/', no '\' and no empty, '.' or '..' segment.
// Every element is since 1.0.0.

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/scm/v1/scm.proto

package scmv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion1_13_0

const (
	// ScmServiceName is the fully-qualified name of the ScmService service.
	ScmServiceName = "numinance.ports.scm.v1.ScmService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// ScmServiceGetRepositoryProcedure is the fully-qualified name of the ScmService's GetRepository
	// RPC.
	ScmServiceGetRepositoryProcedure = "/numinance.ports.scm.v1.ScmService/GetRepository"
	// ScmServiceGetTreeProcedure is the fully-qualified name of the ScmService's GetTree RPC.
	ScmServiceGetTreeProcedure = "/numinance.ports.scm.v1.ScmService/GetTree"
	// ScmServiceGetFileProcedure is the fully-qualified name of the ScmService's GetFile RPC.
	ScmServiceGetFileProcedure = "/numinance.ports.scm.v1.ScmService/GetFile"
	// ScmServiceListBranchesProcedure is the fully-qualified name of the ScmService's ListBranches RPC.
	ScmServiceListBranchesProcedure = "/numinance.ports.scm.v1.ScmService/ListBranches"
	// ScmServiceCreateBranchProcedure is the fully-qualified name of the ScmService's CreateBranch RPC.
	ScmServiceCreateBranchProcedure = "/numinance.ports.scm.v1.ScmService/CreateBranch"
	// ScmServiceGetBranchProtectionProcedure is the fully-qualified name of the ScmService's
	// GetBranchProtection RPC.
	ScmServiceGetBranchProtectionProcedure = "/numinance.ports.scm.v1.ScmService/GetBranchProtection"
	// ScmServiceOpenPullRequestProcedure is the fully-qualified name of the ScmService's
	// OpenPullRequest RPC.
	ScmServiceOpenPullRequestProcedure = "/numinance.ports.scm.v1.ScmService/OpenPullRequest"
	// ScmServiceUpdatePullRequestProcedure is the fully-qualified name of the ScmService's
	// UpdatePullRequest RPC.
	ScmServiceUpdatePullRequestProcedure = "/numinance.ports.scm.v1.ScmService/UpdatePullRequest"
	// ScmServiceCommentOnPullRequestProcedure is the fully-qualified name of the ScmService's
	// CommentOnPullRequest RPC.
	ScmServiceCommentOnPullRequestProcedure = "/numinance.ports.scm.v1.ScmService/CommentOnPullRequest"
	// ScmServiceGetPullRequestStatusProcedure is the fully-qualified name of the ScmService's
	// GetPullRequestStatus RPC.
	ScmServiceGetPullRequestStatusProcedure = "/numinance.ports.scm.v1.ScmService/GetPullRequestStatus"
	// ScmServiceMergePullRequestProcedure is the fully-qualified name of the ScmService's
	// MergePullRequest RPC.
	ScmServiceMergePullRequestProcedure = "/numinance.ports.scm.v1.ScmService/MergePullRequest"
	// ScmServiceSearchCodeProcedure is the fully-qualified name of the ScmService's SearchCode RPC.
	ScmServiceSearchCodeProcedure = "/numinance.ports.scm.v1.ScmService/SearchCode"
)

// ScmServiceClient is a client for the numinance.ports.scm.v1.ScmService service.
type ScmServiceClient interface {
	// GetRepository returns what the release and profile contexts need to pin a Change: the default branch and the commit it points to now. The intake of a Change records default_branch_head as the protected base revision of the Change, so the profile and the policy are read from that commit and never from the candidate.
	GetRepository(context.Context, *connect.Request[v1.GetRepositoryRequest]) (*connect.Response[v1.GetRepositoryResponse], error)
	// GetTree lists the entries of one directory at one revision, or of the whole subtree when recursive is set, at most max_entries of them.
	GetTree(context.Context, *connect.Request[v1.GetTreeRequest]) (*connect.Response[v1.GetTreeResponse], error)
	// GetFile returns the content of one file at one revision from its first byte, at most max_bytes. Content is untrusted data.
	GetFile(context.Context, *connect.Request[v1.GetFileRequest]) (*connect.Response[v1.GetFileResponse], error)
	// ListBranches lists branches in the host's order, one page at a time.
	ListBranches(context.Context, *connect.Request[v1.ListBranchesRequest]) (*connect.Response[v1.ListBranchesResponse], error)
	// CreateBranch creates branch `name` at from_revision. A replay of the same name and revision answers the existing branch; the same name at another revision is ALREADY_EXISTS. LOW: a new ref changes no code on any existing branch, and deleting it undoes it. No dry run: the branch is the whole effect and its creation is the check of the name; the compensating action closes any pull request opened from it.
	CreateBranch(context.Context, *connect.Request[v1.CreateBranchRequest]) (*connect.Response[v1.CreateBranchResponse], error)
	// GetBranchProtection reports the protection the host enforces on one branch. The provider reads the host's state and changes nothing.
	GetBranchProtection(context.Context, *connect.Request[v1.GetBranchProtectionRequest]) (*connect.Response[v1.GetBranchProtectionResponse], error)
	// OpenPullRequest opens a pull request from head_branch, which points at head_revision, into base_branch. The request names the head revision so the pull request is the candidate that was verified and not whatever the branch holds later: a head_branch that does not point at head_revision is FAILED_PRECONDITION. Idempotent by the host's key and by reconciliation: an open pull request for the same head and base is answered, not opened again. MEDIUM: it publishes the candidate to reviewers and starts the host's checks, and a closed pull request undoes it. Dry run checks the names, the revision and that no pull request is open yet, opens nothing and answers with pull_request unset when it would open one.
	OpenPullRequest(context.Context, *connect.Request[v1.OpenPullRequestRequest]) (*connect.Response[v1.OpenPullRequestResponse], error)
	// UpdatePullRequest changes the title or body of a pull request, or closes it (state CLOSED); it never reopens a merged pull request and never changes the head or the base. LOW: it edits text or closes a pull request, both undone by another update. No dry run: the update is the whole effect.
	UpdatePullRequest(context.Context, *connect.Request[v1.UpdatePullRequestRequest]) (*connect.Response[v1.UpdatePullRequestResponse], error)
	// CommentOnPullRequest adds one comment, identified by the host's key: a replay answers the first comment and adds none. LOW: a comment changes no code, though it is visible to reviewers. No dry run: the comment is the whole effect, and a dry run would be the call.
	CommentOnPullRequest(context.Context, *connect.Request[v1.CommentOnPullRequestRequest]) (*connect.Response[v1.CommentOnPullRequestResponse], error)
	// GetPullRequestStatus returns the state of a pull request, its head, its checks and the reviews it holds. It is also the reconciliation read of OpenPullRequest and MergePullRequest after a lost response. Review counts are the host's claim: the release context never merges on them alone.
	GetPullRequestStatus(context.Context, *connect.Request[v1.GetPullRequestStatusRequest]) (*connect.Response[v1.GetPullRequestStatusResponse], error)
	// MergePullRequest merges a pull request into its base branch. PRIVILEGED: it changes the protected branch every later Change is pinned to. It is allowed only to the release-pipeline identity under a grant of level Merge or above, after a ReleaseGate allow bound to this candidate; the policy floors refuse the author of the Change. expected_head_revision is the head the caller verified: a pull request whose head moved is FAILED_PRECONDITION, so the merge is of the verified candidate and no other. A merge already done under the same key is answered as done with the merge commit, never merged again. Dry run answers, with an empty merge_revision, that the pull request is open at expected_head_revision and not in conflict, or fails as the merge would, and merges nothing.
	MergePullRequest(context.Context, *connect.Request[v1.MergePullRequestRequest]) (*connect.Response[v1.MergePullRequestResponse], error)
	// SearchCode searches the default branch for text and returns paths, never content. The query is search text only, passed to the host as a parameter and never spliced into its query language.
	SearchCode(context.Context, *connect.Request[v1.SearchCodeRequest]) (*connect.Response[v1.SearchCodeResponse], error)
}

// NewScmServiceClient constructs a client for the numinance.ports.scm.v1.ScmService service. By
// default, it uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses,
// and sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the
// connect.WithGRPC() or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewScmServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) ScmServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	scmServiceMethods := v1.File_numinance_ports_scm_v1_scm_proto.Services().ByName("ScmService").Methods()
	return &scmServiceClient{
		getRepository: connect.NewClient[v1.GetRepositoryRequest, v1.GetRepositoryResponse](
			httpClient,
			baseURL+ScmServiceGetRepositoryProcedure,
			connect.WithSchema(scmServiceMethods.ByName("GetRepository")),
			connect.WithClientOptions(opts...),
		),
		getTree: connect.NewClient[v1.GetTreeRequest, v1.GetTreeResponse](
			httpClient,
			baseURL+ScmServiceGetTreeProcedure,
			connect.WithSchema(scmServiceMethods.ByName("GetTree")),
			connect.WithClientOptions(opts...),
		),
		getFile: connect.NewClient[v1.GetFileRequest, v1.GetFileResponse](
			httpClient,
			baseURL+ScmServiceGetFileProcedure,
			connect.WithSchema(scmServiceMethods.ByName("GetFile")),
			connect.WithClientOptions(opts...),
		),
		listBranches: connect.NewClient[v1.ListBranchesRequest, v1.ListBranchesResponse](
			httpClient,
			baseURL+ScmServiceListBranchesProcedure,
			connect.WithSchema(scmServiceMethods.ByName("ListBranches")),
			connect.WithClientOptions(opts...),
		),
		createBranch: connect.NewClient[v1.CreateBranchRequest, v1.CreateBranchResponse](
			httpClient,
			baseURL+ScmServiceCreateBranchProcedure,
			connect.WithSchema(scmServiceMethods.ByName("CreateBranch")),
			connect.WithClientOptions(opts...),
		),
		getBranchProtection: connect.NewClient[v1.GetBranchProtectionRequest, v1.GetBranchProtectionResponse](
			httpClient,
			baseURL+ScmServiceGetBranchProtectionProcedure,
			connect.WithSchema(scmServiceMethods.ByName("GetBranchProtection")),
			connect.WithClientOptions(opts...),
		),
		openPullRequest: connect.NewClient[v1.OpenPullRequestRequest, v1.OpenPullRequestResponse](
			httpClient,
			baseURL+ScmServiceOpenPullRequestProcedure,
			connect.WithSchema(scmServiceMethods.ByName("OpenPullRequest")),
			connect.WithClientOptions(opts...),
		),
		updatePullRequest: connect.NewClient[v1.UpdatePullRequestRequest, v1.UpdatePullRequestResponse](
			httpClient,
			baseURL+ScmServiceUpdatePullRequestProcedure,
			connect.WithSchema(scmServiceMethods.ByName("UpdatePullRequest")),
			connect.WithClientOptions(opts...),
		),
		commentOnPullRequest: connect.NewClient[v1.CommentOnPullRequestRequest, v1.CommentOnPullRequestResponse](
			httpClient,
			baseURL+ScmServiceCommentOnPullRequestProcedure,
			connect.WithSchema(scmServiceMethods.ByName("CommentOnPullRequest")),
			connect.WithClientOptions(opts...),
		),
		getPullRequestStatus: connect.NewClient[v1.GetPullRequestStatusRequest, v1.GetPullRequestStatusResponse](
			httpClient,
			baseURL+ScmServiceGetPullRequestStatusProcedure,
			connect.WithSchema(scmServiceMethods.ByName("GetPullRequestStatus")),
			connect.WithClientOptions(opts...),
		),
		mergePullRequest: connect.NewClient[v1.MergePullRequestRequest, v1.MergePullRequestResponse](
			httpClient,
			baseURL+ScmServiceMergePullRequestProcedure,
			connect.WithSchema(scmServiceMethods.ByName("MergePullRequest")),
			connect.WithClientOptions(opts...),
		),
		searchCode: connect.NewClient[v1.SearchCodeRequest, v1.SearchCodeResponse](
			httpClient,
			baseURL+ScmServiceSearchCodeProcedure,
			connect.WithSchema(scmServiceMethods.ByName("SearchCode")),
			connect.WithClientOptions(opts...),
		),
	}
}

// scmServiceClient implements ScmServiceClient.
type scmServiceClient struct {
	getRepository        *connect.Client[v1.GetRepositoryRequest, v1.GetRepositoryResponse]
	getTree              *connect.Client[v1.GetTreeRequest, v1.GetTreeResponse]
	getFile              *connect.Client[v1.GetFileRequest, v1.GetFileResponse]
	listBranches         *connect.Client[v1.ListBranchesRequest, v1.ListBranchesResponse]
	createBranch         *connect.Client[v1.CreateBranchRequest, v1.CreateBranchResponse]
	getBranchProtection  *connect.Client[v1.GetBranchProtectionRequest, v1.GetBranchProtectionResponse]
	openPullRequest      *connect.Client[v1.OpenPullRequestRequest, v1.OpenPullRequestResponse]
	updatePullRequest    *connect.Client[v1.UpdatePullRequestRequest, v1.UpdatePullRequestResponse]
	commentOnPullRequest *connect.Client[v1.CommentOnPullRequestRequest, v1.CommentOnPullRequestResponse]
	getPullRequestStatus *connect.Client[v1.GetPullRequestStatusRequest, v1.GetPullRequestStatusResponse]
	mergePullRequest     *connect.Client[v1.MergePullRequestRequest, v1.MergePullRequestResponse]
	searchCode           *connect.Client[v1.SearchCodeRequest, v1.SearchCodeResponse]
}

// GetRepository calls numinance.ports.scm.v1.ScmService.GetRepository.
func (c *scmServiceClient) GetRepository(ctx context.Context, req *connect.Request[v1.GetRepositoryRequest]) (*connect.Response[v1.GetRepositoryResponse], error) {
	return c.getRepository.CallUnary(ctx, req)
}

// GetTree calls numinance.ports.scm.v1.ScmService.GetTree.
func (c *scmServiceClient) GetTree(ctx context.Context, req *connect.Request[v1.GetTreeRequest]) (*connect.Response[v1.GetTreeResponse], error) {
	return c.getTree.CallUnary(ctx, req)
}

// GetFile calls numinance.ports.scm.v1.ScmService.GetFile.
func (c *scmServiceClient) GetFile(ctx context.Context, req *connect.Request[v1.GetFileRequest]) (*connect.Response[v1.GetFileResponse], error) {
	return c.getFile.CallUnary(ctx, req)
}

// ListBranches calls numinance.ports.scm.v1.ScmService.ListBranches.
func (c *scmServiceClient) ListBranches(ctx context.Context, req *connect.Request[v1.ListBranchesRequest]) (*connect.Response[v1.ListBranchesResponse], error) {
	return c.listBranches.CallUnary(ctx, req)
}

// CreateBranch calls numinance.ports.scm.v1.ScmService.CreateBranch.
func (c *scmServiceClient) CreateBranch(ctx context.Context, req *connect.Request[v1.CreateBranchRequest]) (*connect.Response[v1.CreateBranchResponse], error) {
	return c.createBranch.CallUnary(ctx, req)
}

// GetBranchProtection calls numinance.ports.scm.v1.ScmService.GetBranchProtection.
func (c *scmServiceClient) GetBranchProtection(ctx context.Context, req *connect.Request[v1.GetBranchProtectionRequest]) (*connect.Response[v1.GetBranchProtectionResponse], error) {
	return c.getBranchProtection.CallUnary(ctx, req)
}

// OpenPullRequest calls numinance.ports.scm.v1.ScmService.OpenPullRequest.
func (c *scmServiceClient) OpenPullRequest(ctx context.Context, req *connect.Request[v1.OpenPullRequestRequest]) (*connect.Response[v1.OpenPullRequestResponse], error) {
	return c.openPullRequest.CallUnary(ctx, req)
}

// UpdatePullRequest calls numinance.ports.scm.v1.ScmService.UpdatePullRequest.
func (c *scmServiceClient) UpdatePullRequest(ctx context.Context, req *connect.Request[v1.UpdatePullRequestRequest]) (*connect.Response[v1.UpdatePullRequestResponse], error) {
	return c.updatePullRequest.CallUnary(ctx, req)
}

// CommentOnPullRequest calls numinance.ports.scm.v1.ScmService.CommentOnPullRequest.
func (c *scmServiceClient) CommentOnPullRequest(ctx context.Context, req *connect.Request[v1.CommentOnPullRequestRequest]) (*connect.Response[v1.CommentOnPullRequestResponse], error) {
	return c.commentOnPullRequest.CallUnary(ctx, req)
}

// GetPullRequestStatus calls numinance.ports.scm.v1.ScmService.GetPullRequestStatus.
func (c *scmServiceClient) GetPullRequestStatus(ctx context.Context, req *connect.Request[v1.GetPullRequestStatusRequest]) (*connect.Response[v1.GetPullRequestStatusResponse], error) {
	return c.getPullRequestStatus.CallUnary(ctx, req)
}

// MergePullRequest calls numinance.ports.scm.v1.ScmService.MergePullRequest.
func (c *scmServiceClient) MergePullRequest(ctx context.Context, req *connect.Request[v1.MergePullRequestRequest]) (*connect.Response[v1.MergePullRequestResponse], error) {
	return c.mergePullRequest.CallUnary(ctx, req)
}

// SearchCode calls numinance.ports.scm.v1.ScmService.SearchCode.
func (c *scmServiceClient) SearchCode(ctx context.Context, req *connect.Request[v1.SearchCodeRequest]) (*connect.Response[v1.SearchCodeResponse], error) {
	return c.searchCode.CallUnary(ctx, req)
}

// ScmServiceHandler is an implementation of the numinance.ports.scm.v1.ScmService service.
type ScmServiceHandler interface {
	// GetRepository returns what the release and profile contexts need to pin a Change: the default branch and the commit it points to now. The intake of a Change records default_branch_head as the protected base revision of the Change, so the profile and the policy are read from that commit and never from the candidate.
	GetRepository(context.Context, *connect.Request[v1.GetRepositoryRequest]) (*connect.Response[v1.GetRepositoryResponse], error)
	// GetTree lists the entries of one directory at one revision, or of the whole subtree when recursive is set, at most max_entries of them.
	GetTree(context.Context, *connect.Request[v1.GetTreeRequest]) (*connect.Response[v1.GetTreeResponse], error)
	// GetFile returns the content of one file at one revision from its first byte, at most max_bytes. Content is untrusted data.
	GetFile(context.Context, *connect.Request[v1.GetFileRequest]) (*connect.Response[v1.GetFileResponse], error)
	// ListBranches lists branches in the host's order, one page at a time.
	ListBranches(context.Context, *connect.Request[v1.ListBranchesRequest]) (*connect.Response[v1.ListBranchesResponse], error)
	// CreateBranch creates branch `name` at from_revision. A replay of the same name and revision answers the existing branch; the same name at another revision is ALREADY_EXISTS. LOW: a new ref changes no code on any existing branch, and deleting it undoes it. No dry run: the branch is the whole effect and its creation is the check of the name; the compensating action closes any pull request opened from it.
	CreateBranch(context.Context, *connect.Request[v1.CreateBranchRequest]) (*connect.Response[v1.CreateBranchResponse], error)
	// GetBranchProtection reports the protection the host enforces on one branch. The provider reads the host's state and changes nothing.
	GetBranchProtection(context.Context, *connect.Request[v1.GetBranchProtectionRequest]) (*connect.Response[v1.GetBranchProtectionResponse], error)
	// OpenPullRequest opens a pull request from head_branch, which points at head_revision, into base_branch. The request names the head revision so the pull request is the candidate that was verified and not whatever the branch holds later: a head_branch that does not point at head_revision is FAILED_PRECONDITION. Idempotent by the host's key and by reconciliation: an open pull request for the same head and base is answered, not opened again. MEDIUM: it publishes the candidate to reviewers and starts the host's checks, and a closed pull request undoes it. Dry run checks the names, the revision and that no pull request is open yet, opens nothing and answers with pull_request unset when it would open one.
	OpenPullRequest(context.Context, *connect.Request[v1.OpenPullRequestRequest]) (*connect.Response[v1.OpenPullRequestResponse], error)
	// UpdatePullRequest changes the title or body of a pull request, or closes it (state CLOSED); it never reopens a merged pull request and never changes the head or the base. LOW: it edits text or closes a pull request, both undone by another update. No dry run: the update is the whole effect.
	UpdatePullRequest(context.Context, *connect.Request[v1.UpdatePullRequestRequest]) (*connect.Response[v1.UpdatePullRequestResponse], error)
	// CommentOnPullRequest adds one comment, identified by the host's key: a replay answers the first comment and adds none. LOW: a comment changes no code, though it is visible to reviewers. No dry run: the comment is the whole effect, and a dry run would be the call.
	CommentOnPullRequest(context.Context, *connect.Request[v1.CommentOnPullRequestRequest]) (*connect.Response[v1.CommentOnPullRequestResponse], error)
	// GetPullRequestStatus returns the state of a pull request, its head, its checks and the reviews it holds. It is also the reconciliation read of OpenPullRequest and MergePullRequest after a lost response. Review counts are the host's claim: the release context never merges on them alone.
	GetPullRequestStatus(context.Context, *connect.Request[v1.GetPullRequestStatusRequest]) (*connect.Response[v1.GetPullRequestStatusResponse], error)
	// MergePullRequest merges a pull request into its base branch. PRIVILEGED: it changes the protected branch every later Change is pinned to. It is allowed only to the release-pipeline identity under a grant of level Merge or above, after a ReleaseGate allow bound to this candidate; the policy floors refuse the author of the Change. expected_head_revision is the head the caller verified: a pull request whose head moved is FAILED_PRECONDITION, so the merge is of the verified candidate and no other. A merge already done under the same key is answered as done with the merge commit, never merged again. Dry run answers, with an empty merge_revision, that the pull request is open at expected_head_revision and not in conflict, or fails as the merge would, and merges nothing.
	MergePullRequest(context.Context, *connect.Request[v1.MergePullRequestRequest]) (*connect.Response[v1.MergePullRequestResponse], error)
	// SearchCode searches the default branch for text and returns paths, never content. The query is search text only, passed to the host as a parameter and never spliced into its query language.
	SearchCode(context.Context, *connect.Request[v1.SearchCodeRequest]) (*connect.Response[v1.SearchCodeResponse], error)
}

// NewScmServiceHandler builds an HTTP handler from the service implementation. It returns the path
// on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewScmServiceHandler(svc ScmServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	scmServiceMethods := v1.File_numinance_ports_scm_v1_scm_proto.Services().ByName("ScmService").Methods()
	scmServiceGetRepositoryHandler := connect.NewUnaryHandler(
		ScmServiceGetRepositoryProcedure,
		svc.GetRepository,
		connect.WithSchema(scmServiceMethods.ByName("GetRepository")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceGetTreeHandler := connect.NewUnaryHandler(
		ScmServiceGetTreeProcedure,
		svc.GetTree,
		connect.WithSchema(scmServiceMethods.ByName("GetTree")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceGetFileHandler := connect.NewUnaryHandler(
		ScmServiceGetFileProcedure,
		svc.GetFile,
		connect.WithSchema(scmServiceMethods.ByName("GetFile")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceListBranchesHandler := connect.NewUnaryHandler(
		ScmServiceListBranchesProcedure,
		svc.ListBranches,
		connect.WithSchema(scmServiceMethods.ByName("ListBranches")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceCreateBranchHandler := connect.NewUnaryHandler(
		ScmServiceCreateBranchProcedure,
		svc.CreateBranch,
		connect.WithSchema(scmServiceMethods.ByName("CreateBranch")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceGetBranchProtectionHandler := connect.NewUnaryHandler(
		ScmServiceGetBranchProtectionProcedure,
		svc.GetBranchProtection,
		connect.WithSchema(scmServiceMethods.ByName("GetBranchProtection")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceOpenPullRequestHandler := connect.NewUnaryHandler(
		ScmServiceOpenPullRequestProcedure,
		svc.OpenPullRequest,
		connect.WithSchema(scmServiceMethods.ByName("OpenPullRequest")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceUpdatePullRequestHandler := connect.NewUnaryHandler(
		ScmServiceUpdatePullRequestProcedure,
		svc.UpdatePullRequest,
		connect.WithSchema(scmServiceMethods.ByName("UpdatePullRequest")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceCommentOnPullRequestHandler := connect.NewUnaryHandler(
		ScmServiceCommentOnPullRequestProcedure,
		svc.CommentOnPullRequest,
		connect.WithSchema(scmServiceMethods.ByName("CommentOnPullRequest")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceGetPullRequestStatusHandler := connect.NewUnaryHandler(
		ScmServiceGetPullRequestStatusProcedure,
		svc.GetPullRequestStatus,
		connect.WithSchema(scmServiceMethods.ByName("GetPullRequestStatus")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceMergePullRequestHandler := connect.NewUnaryHandler(
		ScmServiceMergePullRequestProcedure,
		svc.MergePullRequest,
		connect.WithSchema(scmServiceMethods.ByName("MergePullRequest")),
		connect.WithHandlerOptions(opts...),
	)
	scmServiceSearchCodeHandler := connect.NewUnaryHandler(
		ScmServiceSearchCodeProcedure,
		svc.SearchCode,
		connect.WithSchema(scmServiceMethods.ByName("SearchCode")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.scm.v1.ScmService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case ScmServiceGetRepositoryProcedure:
			scmServiceGetRepositoryHandler.ServeHTTP(w, r)
		case ScmServiceGetTreeProcedure:
			scmServiceGetTreeHandler.ServeHTTP(w, r)
		case ScmServiceGetFileProcedure:
			scmServiceGetFileHandler.ServeHTTP(w, r)
		case ScmServiceListBranchesProcedure:
			scmServiceListBranchesHandler.ServeHTTP(w, r)
		case ScmServiceCreateBranchProcedure:
			scmServiceCreateBranchHandler.ServeHTTP(w, r)
		case ScmServiceGetBranchProtectionProcedure:
			scmServiceGetBranchProtectionHandler.ServeHTTP(w, r)
		case ScmServiceOpenPullRequestProcedure:
			scmServiceOpenPullRequestHandler.ServeHTTP(w, r)
		case ScmServiceUpdatePullRequestProcedure:
			scmServiceUpdatePullRequestHandler.ServeHTTP(w, r)
		case ScmServiceCommentOnPullRequestProcedure:
			scmServiceCommentOnPullRequestHandler.ServeHTTP(w, r)
		case ScmServiceGetPullRequestStatusProcedure:
			scmServiceGetPullRequestStatusHandler.ServeHTTP(w, r)
		case ScmServiceMergePullRequestProcedure:
			scmServiceMergePullRequestHandler.ServeHTTP(w, r)
		case ScmServiceSearchCodeProcedure:
			scmServiceSearchCodeHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedScmServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedScmServiceHandler struct{}

func (UnimplementedScmServiceHandler) GetRepository(context.Context, *connect.Request[v1.GetRepositoryRequest]) (*connect.Response[v1.GetRepositoryResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.GetRepository is not implemented"))
}

func (UnimplementedScmServiceHandler) GetTree(context.Context, *connect.Request[v1.GetTreeRequest]) (*connect.Response[v1.GetTreeResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.GetTree is not implemented"))
}

func (UnimplementedScmServiceHandler) GetFile(context.Context, *connect.Request[v1.GetFileRequest]) (*connect.Response[v1.GetFileResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.GetFile is not implemented"))
}

func (UnimplementedScmServiceHandler) ListBranches(context.Context, *connect.Request[v1.ListBranchesRequest]) (*connect.Response[v1.ListBranchesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.ListBranches is not implemented"))
}

func (UnimplementedScmServiceHandler) CreateBranch(context.Context, *connect.Request[v1.CreateBranchRequest]) (*connect.Response[v1.CreateBranchResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.CreateBranch is not implemented"))
}

func (UnimplementedScmServiceHandler) GetBranchProtection(context.Context, *connect.Request[v1.GetBranchProtectionRequest]) (*connect.Response[v1.GetBranchProtectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.GetBranchProtection is not implemented"))
}

func (UnimplementedScmServiceHandler) OpenPullRequest(context.Context, *connect.Request[v1.OpenPullRequestRequest]) (*connect.Response[v1.OpenPullRequestResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.OpenPullRequest is not implemented"))
}

func (UnimplementedScmServiceHandler) UpdatePullRequest(context.Context, *connect.Request[v1.UpdatePullRequestRequest]) (*connect.Response[v1.UpdatePullRequestResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.UpdatePullRequest is not implemented"))
}

func (UnimplementedScmServiceHandler) CommentOnPullRequest(context.Context, *connect.Request[v1.CommentOnPullRequestRequest]) (*connect.Response[v1.CommentOnPullRequestResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.CommentOnPullRequest is not implemented"))
}

func (UnimplementedScmServiceHandler) GetPullRequestStatus(context.Context, *connect.Request[v1.GetPullRequestStatusRequest]) (*connect.Response[v1.GetPullRequestStatusResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.GetPullRequestStatus is not implemented"))
}

func (UnimplementedScmServiceHandler) MergePullRequest(context.Context, *connect.Request[v1.MergePullRequestRequest]) (*connect.Response[v1.MergePullRequestResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.MergePullRequest is not implemented"))
}

func (UnimplementedScmServiceHandler) SearchCode(context.Context, *connect.Request[v1.SearchCodeRequest]) (*connect.Response[v1.SearchCodeResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.scm.v1.ScmService.SearchCode is not implemented"))
}
