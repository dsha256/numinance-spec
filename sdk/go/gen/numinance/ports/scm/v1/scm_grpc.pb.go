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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.6.2
// - protoc             (unknown)
// source: numinance/ports/scm/v1/scm.proto

package scmv1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	ScmService_GetRepository_FullMethodName        = "/numinance.ports.scm.v1.ScmService/GetRepository"
	ScmService_GetTree_FullMethodName              = "/numinance.ports.scm.v1.ScmService/GetTree"
	ScmService_GetFile_FullMethodName              = "/numinance.ports.scm.v1.ScmService/GetFile"
	ScmService_ListBranches_FullMethodName         = "/numinance.ports.scm.v1.ScmService/ListBranches"
	ScmService_CreateBranch_FullMethodName         = "/numinance.ports.scm.v1.ScmService/CreateBranch"
	ScmService_GetBranchProtection_FullMethodName  = "/numinance.ports.scm.v1.ScmService/GetBranchProtection"
	ScmService_OpenPullRequest_FullMethodName      = "/numinance.ports.scm.v1.ScmService/OpenPullRequest"
	ScmService_UpdatePullRequest_FullMethodName    = "/numinance.ports.scm.v1.ScmService/UpdatePullRequest"
	ScmService_CommentOnPullRequest_FullMethodName = "/numinance.ports.scm.v1.ScmService/CommentOnPullRequest"
	ScmService_GetPullRequestStatus_FullMethodName = "/numinance.ports.scm.v1.ScmService/GetPullRequestStatus"
	ScmService_MergePullRequest_FullMethodName     = "/numinance.ports.scm.v1.ScmService/MergePullRequest"
	ScmService_SearchCode_FullMethodName           = "/numinance.ports.scm.v1.ScmService/SearchCode"
)

// ScmServiceClient is the client API for ScmService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// ScmService is the scm port a source-control provider serves.
type ScmServiceClient interface {
	// GetRepository returns what the release and profile contexts need to pin a Change: the default branch and the commit it points to now. The intake of a Change records default_branch_head as the protected base revision of the Change, so the profile and the policy are read from that commit and never from the candidate.
	GetRepository(ctx context.Context, in *GetRepositoryRequest, opts ...grpc.CallOption) (*GetRepositoryResponse, error)
	// GetTree lists the entries of one directory at one revision, or of the whole subtree when recursive is set, at most max_entries of them.
	GetTree(ctx context.Context, in *GetTreeRequest, opts ...grpc.CallOption) (*GetTreeResponse, error)
	// GetFile returns the content of one file at one revision from its first byte, at most max_bytes. Content is untrusted data.
	GetFile(ctx context.Context, in *GetFileRequest, opts ...grpc.CallOption) (*GetFileResponse, error)
	// ListBranches lists branches in the host's order, one page at a time.
	ListBranches(ctx context.Context, in *ListBranchesRequest, opts ...grpc.CallOption) (*ListBranchesResponse, error)
	// CreateBranch creates branch `name` at from_revision. A replay of the same name and revision answers the existing branch; the same name at another revision is ALREADY_EXISTS. LOW: a new ref changes no code on any existing branch, and deleting it undoes it. No dry run: the branch is the whole effect and its creation is the check of the name; the compensating action closes any pull request opened from it.
	CreateBranch(ctx context.Context, in *CreateBranchRequest, opts ...grpc.CallOption) (*CreateBranchResponse, error)
	// GetBranchProtection reports the protection the host enforces on one branch. The provider reads the host's state and changes nothing.
	GetBranchProtection(ctx context.Context, in *GetBranchProtectionRequest, opts ...grpc.CallOption) (*GetBranchProtectionResponse, error)
	// OpenPullRequest opens a pull request from head_branch, which points at head_revision, into base_branch. The request names the head revision so the pull request is the candidate that was verified and not whatever the branch holds later: a head_branch that does not point at head_revision is FAILED_PRECONDITION. Idempotent by the host's key and by reconciliation: an open pull request for the same head and base is answered, not opened again. MEDIUM: it publishes the candidate to reviewers and starts the host's checks, and a closed pull request undoes it. Dry run checks the names, the revision and that no pull request is open yet, opens nothing and answers with pull_request unset when it would open one.
	OpenPullRequest(ctx context.Context, in *OpenPullRequestRequest, opts ...grpc.CallOption) (*OpenPullRequestResponse, error)
	// UpdatePullRequest changes the title or body of a pull request, or closes it (state CLOSED); it never reopens a merged pull request and never changes the head or the base. LOW: it edits text or closes a pull request, both undone by another update. No dry run: the update is the whole effect.
	UpdatePullRequest(ctx context.Context, in *UpdatePullRequestRequest, opts ...grpc.CallOption) (*UpdatePullRequestResponse, error)
	// CommentOnPullRequest adds one comment, identified by the host's key: a replay answers the first comment and adds none. LOW: a comment changes no code, though it is visible to reviewers. No dry run: the comment is the whole effect, and a dry run would be the call.
	CommentOnPullRequest(ctx context.Context, in *CommentOnPullRequestRequest, opts ...grpc.CallOption) (*CommentOnPullRequestResponse, error)
	// GetPullRequestStatus returns the state of a pull request, its head, its checks and the reviews it holds. It is also the reconciliation read of OpenPullRequest and MergePullRequest after a lost response. Review counts are the host's claim: the release context never merges on them alone.
	GetPullRequestStatus(ctx context.Context, in *GetPullRequestStatusRequest, opts ...grpc.CallOption) (*GetPullRequestStatusResponse, error)
	// MergePullRequest merges a pull request into its base branch. PRIVILEGED: it changes the protected branch every later Change is pinned to. It is allowed only to the release-pipeline identity under a grant of level Merge or above, after a ReleaseGate allow bound to this candidate; the policy floors refuse the author of the Change. expected_head_revision is the head the caller verified: a pull request whose head moved is FAILED_PRECONDITION, so the merge is of the verified candidate and no other. A merge already done under the same key is answered as done with the merge commit, never merged again. Dry run answers, with an empty merge_revision, that the pull request is open at expected_head_revision and not in conflict, or fails as the merge would, and merges nothing.
	MergePullRequest(ctx context.Context, in *MergePullRequestRequest, opts ...grpc.CallOption) (*MergePullRequestResponse, error)
	// SearchCode searches the default branch for text and returns paths, never content. The query is search text only, passed to the host as a parameter and never spliced into its query language.
	SearchCode(ctx context.Context, in *SearchCodeRequest, opts ...grpc.CallOption) (*SearchCodeResponse, error)
}

type scmServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewScmServiceClient(cc grpc.ClientConnInterface) ScmServiceClient {
	return &scmServiceClient{cc}
}

func (c *scmServiceClient) GetRepository(ctx context.Context, in *GetRepositoryRequest, opts ...grpc.CallOption) (*GetRepositoryResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetRepositoryResponse)
	err := c.cc.Invoke(ctx, ScmService_GetRepository_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) GetTree(ctx context.Context, in *GetTreeRequest, opts ...grpc.CallOption) (*GetTreeResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetTreeResponse)
	err := c.cc.Invoke(ctx, ScmService_GetTree_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) GetFile(ctx context.Context, in *GetFileRequest, opts ...grpc.CallOption) (*GetFileResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetFileResponse)
	err := c.cc.Invoke(ctx, ScmService_GetFile_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) ListBranches(ctx context.Context, in *ListBranchesRequest, opts ...grpc.CallOption) (*ListBranchesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListBranchesResponse)
	err := c.cc.Invoke(ctx, ScmService_ListBranches_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) CreateBranch(ctx context.Context, in *CreateBranchRequest, opts ...grpc.CallOption) (*CreateBranchResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreateBranchResponse)
	err := c.cc.Invoke(ctx, ScmService_CreateBranch_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) GetBranchProtection(ctx context.Context, in *GetBranchProtectionRequest, opts ...grpc.CallOption) (*GetBranchProtectionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetBranchProtectionResponse)
	err := c.cc.Invoke(ctx, ScmService_GetBranchProtection_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) OpenPullRequest(ctx context.Context, in *OpenPullRequestRequest, opts ...grpc.CallOption) (*OpenPullRequestResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(OpenPullRequestResponse)
	err := c.cc.Invoke(ctx, ScmService_OpenPullRequest_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) UpdatePullRequest(ctx context.Context, in *UpdatePullRequestRequest, opts ...grpc.CallOption) (*UpdatePullRequestResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdatePullRequestResponse)
	err := c.cc.Invoke(ctx, ScmService_UpdatePullRequest_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) CommentOnPullRequest(ctx context.Context, in *CommentOnPullRequestRequest, opts ...grpc.CallOption) (*CommentOnPullRequestResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CommentOnPullRequestResponse)
	err := c.cc.Invoke(ctx, ScmService_CommentOnPullRequest_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) GetPullRequestStatus(ctx context.Context, in *GetPullRequestStatusRequest, opts ...grpc.CallOption) (*GetPullRequestStatusResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetPullRequestStatusResponse)
	err := c.cc.Invoke(ctx, ScmService_GetPullRequestStatus_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) MergePullRequest(ctx context.Context, in *MergePullRequestRequest, opts ...grpc.CallOption) (*MergePullRequestResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(MergePullRequestResponse)
	err := c.cc.Invoke(ctx, ScmService_MergePullRequest_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scmServiceClient) SearchCode(ctx context.Context, in *SearchCodeRequest, opts ...grpc.CallOption) (*SearchCodeResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SearchCodeResponse)
	err := c.cc.Invoke(ctx, ScmService_SearchCode_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ScmServiceServer is the server API for ScmService service.
// All implementations must embed UnimplementedScmServiceServer
// for forward compatibility.
//
// ScmService is the scm port a source-control provider serves.
type ScmServiceServer interface {
	// GetRepository returns what the release and profile contexts need to pin a Change: the default branch and the commit it points to now. The intake of a Change records default_branch_head as the protected base revision of the Change, so the profile and the policy are read from that commit and never from the candidate.
	GetRepository(context.Context, *GetRepositoryRequest) (*GetRepositoryResponse, error)
	// GetTree lists the entries of one directory at one revision, or of the whole subtree when recursive is set, at most max_entries of them.
	GetTree(context.Context, *GetTreeRequest) (*GetTreeResponse, error)
	// GetFile returns the content of one file at one revision from its first byte, at most max_bytes. Content is untrusted data.
	GetFile(context.Context, *GetFileRequest) (*GetFileResponse, error)
	// ListBranches lists branches in the host's order, one page at a time.
	ListBranches(context.Context, *ListBranchesRequest) (*ListBranchesResponse, error)
	// CreateBranch creates branch `name` at from_revision. A replay of the same name and revision answers the existing branch; the same name at another revision is ALREADY_EXISTS. LOW: a new ref changes no code on any existing branch, and deleting it undoes it. No dry run: the branch is the whole effect and its creation is the check of the name; the compensating action closes any pull request opened from it.
	CreateBranch(context.Context, *CreateBranchRequest) (*CreateBranchResponse, error)
	// GetBranchProtection reports the protection the host enforces on one branch. The provider reads the host's state and changes nothing.
	GetBranchProtection(context.Context, *GetBranchProtectionRequest) (*GetBranchProtectionResponse, error)
	// OpenPullRequest opens a pull request from head_branch, which points at head_revision, into base_branch. The request names the head revision so the pull request is the candidate that was verified and not whatever the branch holds later: a head_branch that does not point at head_revision is FAILED_PRECONDITION. Idempotent by the host's key and by reconciliation: an open pull request for the same head and base is answered, not opened again. MEDIUM: it publishes the candidate to reviewers and starts the host's checks, and a closed pull request undoes it. Dry run checks the names, the revision and that no pull request is open yet, opens nothing and answers with pull_request unset when it would open one.
	OpenPullRequest(context.Context, *OpenPullRequestRequest) (*OpenPullRequestResponse, error)
	// UpdatePullRequest changes the title or body of a pull request, or closes it (state CLOSED); it never reopens a merged pull request and never changes the head or the base. LOW: it edits text or closes a pull request, both undone by another update. No dry run: the update is the whole effect.
	UpdatePullRequest(context.Context, *UpdatePullRequestRequest) (*UpdatePullRequestResponse, error)
	// CommentOnPullRequest adds one comment, identified by the host's key: a replay answers the first comment and adds none. LOW: a comment changes no code, though it is visible to reviewers. No dry run: the comment is the whole effect, and a dry run would be the call.
	CommentOnPullRequest(context.Context, *CommentOnPullRequestRequest) (*CommentOnPullRequestResponse, error)
	// GetPullRequestStatus returns the state of a pull request, its head, its checks and the reviews it holds. It is also the reconciliation read of OpenPullRequest and MergePullRequest after a lost response. Review counts are the host's claim: the release context never merges on them alone.
	GetPullRequestStatus(context.Context, *GetPullRequestStatusRequest) (*GetPullRequestStatusResponse, error)
	// MergePullRequest merges a pull request into its base branch. PRIVILEGED: it changes the protected branch every later Change is pinned to. It is allowed only to the release-pipeline identity under a grant of level Merge or above, after a ReleaseGate allow bound to this candidate; the policy floors refuse the author of the Change. expected_head_revision is the head the caller verified: a pull request whose head moved is FAILED_PRECONDITION, so the merge is of the verified candidate and no other. A merge already done under the same key is answered as done with the merge commit, never merged again. Dry run answers, with an empty merge_revision, that the pull request is open at expected_head_revision and not in conflict, or fails as the merge would, and merges nothing.
	MergePullRequest(context.Context, *MergePullRequestRequest) (*MergePullRequestResponse, error)
	// SearchCode searches the default branch for text and returns paths, never content. The query is search text only, passed to the host as a parameter and never spliced into its query language.
	SearchCode(context.Context, *SearchCodeRequest) (*SearchCodeResponse, error)
	mustEmbedUnimplementedScmServiceServer()
}

// UnimplementedScmServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedScmServiceServer struct{}

func (UnimplementedScmServiceServer) GetRepository(context.Context, *GetRepositoryRequest) (*GetRepositoryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetRepository not implemented")
}
func (UnimplementedScmServiceServer) GetTree(context.Context, *GetTreeRequest) (*GetTreeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetTree not implemented")
}
func (UnimplementedScmServiceServer) GetFile(context.Context, *GetFileRequest) (*GetFileResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetFile not implemented")
}
func (UnimplementedScmServiceServer) ListBranches(context.Context, *ListBranchesRequest) (*ListBranchesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListBranches not implemented")
}
func (UnimplementedScmServiceServer) CreateBranch(context.Context, *CreateBranchRequest) (*CreateBranchResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method CreateBranch not implemented")
}
func (UnimplementedScmServiceServer) GetBranchProtection(context.Context, *GetBranchProtectionRequest) (*GetBranchProtectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetBranchProtection not implemented")
}
func (UnimplementedScmServiceServer) OpenPullRequest(context.Context, *OpenPullRequestRequest) (*OpenPullRequestResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method OpenPullRequest not implemented")
}
func (UnimplementedScmServiceServer) UpdatePullRequest(context.Context, *UpdatePullRequestRequest) (*UpdatePullRequestResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method UpdatePullRequest not implemented")
}
func (UnimplementedScmServiceServer) CommentOnPullRequest(context.Context, *CommentOnPullRequestRequest) (*CommentOnPullRequestResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method CommentOnPullRequest not implemented")
}
func (UnimplementedScmServiceServer) GetPullRequestStatus(context.Context, *GetPullRequestStatusRequest) (*GetPullRequestStatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetPullRequestStatus not implemented")
}
func (UnimplementedScmServiceServer) MergePullRequest(context.Context, *MergePullRequestRequest) (*MergePullRequestResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method MergePullRequest not implemented")
}
func (UnimplementedScmServiceServer) SearchCode(context.Context, *SearchCodeRequest) (*SearchCodeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method SearchCode not implemented")
}
func (UnimplementedScmServiceServer) mustEmbedUnimplementedScmServiceServer() {}
func (UnimplementedScmServiceServer) testEmbeddedByValue()                    {}

// UnsafeScmServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ScmServiceServer will
// result in compilation errors.
type UnsafeScmServiceServer interface {
	mustEmbedUnimplementedScmServiceServer()
}

func RegisterScmServiceServer(s grpc.ServiceRegistrar, srv ScmServiceServer) {
	// If the following call panics, it indicates UnimplementedScmServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&ScmService_ServiceDesc, srv)
}

func _ScmService_GetRepository_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRepositoryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).GetRepository(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_GetRepository_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).GetRepository(ctx, req.(*GetRepositoryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_GetTree_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTreeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).GetTree(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_GetTree_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).GetTree(ctx, req.(*GetTreeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_GetFile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetFileRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).GetFile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_GetFile_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).GetFile(ctx, req.(*GetFileRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_ListBranches_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListBranchesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).ListBranches(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_ListBranches_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).ListBranches(ctx, req.(*ListBranchesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_CreateBranch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateBranchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).CreateBranch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_CreateBranch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).CreateBranch(ctx, req.(*CreateBranchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_GetBranchProtection_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetBranchProtectionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).GetBranchProtection(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_GetBranchProtection_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).GetBranchProtection(ctx, req.(*GetBranchProtectionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_OpenPullRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OpenPullRequestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).OpenPullRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_OpenPullRequest_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).OpenPullRequest(ctx, req.(*OpenPullRequestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_UpdatePullRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdatePullRequestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).UpdatePullRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_UpdatePullRequest_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).UpdatePullRequest(ctx, req.(*UpdatePullRequestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_CommentOnPullRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CommentOnPullRequestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).CommentOnPullRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_CommentOnPullRequest_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).CommentOnPullRequest(ctx, req.(*CommentOnPullRequestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_GetPullRequestStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPullRequestStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).GetPullRequestStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_GetPullRequestStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).GetPullRequestStatus(ctx, req.(*GetPullRequestStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_MergePullRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MergePullRequestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).MergePullRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_MergePullRequest_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).MergePullRequest(ctx, req.(*MergePullRequestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ScmService_SearchCode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SearchCodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScmServiceServer).SearchCode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ScmService_SearchCode_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScmServiceServer).SearchCode(ctx, req.(*SearchCodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ScmService_ServiceDesc is the grpc.ServiceDesc for ScmService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ScmService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "numinance.ports.scm.v1.ScmService",
	HandlerType: (*ScmServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetRepository",
			Handler:    _ScmService_GetRepository_Handler,
		},
		{
			MethodName: "GetTree",
			Handler:    _ScmService_GetTree_Handler,
		},
		{
			MethodName: "GetFile",
			Handler:    _ScmService_GetFile_Handler,
		},
		{
			MethodName: "ListBranches",
			Handler:    _ScmService_ListBranches_Handler,
		},
		{
			MethodName: "CreateBranch",
			Handler:    _ScmService_CreateBranch_Handler,
		},
		{
			MethodName: "GetBranchProtection",
			Handler:    _ScmService_GetBranchProtection_Handler,
		},
		{
			MethodName: "OpenPullRequest",
			Handler:    _ScmService_OpenPullRequest_Handler,
		},
		{
			MethodName: "UpdatePullRequest",
			Handler:    _ScmService_UpdatePullRequest_Handler,
		},
		{
			MethodName: "CommentOnPullRequest",
			Handler:    _ScmService_CommentOnPullRequest_Handler,
		},
		{
			MethodName: "GetPullRequestStatus",
			Handler:    _ScmService_GetPullRequestStatus_Handler,
		},
		{
			MethodName: "MergePullRequest",
			Handler:    _ScmService_MergePullRequest_Handler,
		},
		{
			MethodName: "SearchCode",
			Handler:    _ScmService_SearchCode_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "numinance/ports/scm/v1/scm.proto",
}
