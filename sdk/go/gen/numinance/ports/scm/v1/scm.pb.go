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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/scm/v1/scm.proto

package scmv1

import (
	_ "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// PullRequestState is the state of a pull request on the host.
type PullRequestState int32

const (
	// PULL_REQUEST_STATE_UNSPECIFIED is the zero value; it is rejected.
	PullRequestState_PULL_REQUEST_STATE_UNSPECIFIED PullRequestState = 0
	// PULL_REQUEST_STATE_OPEN: open for review and merge.
	PullRequestState_PULL_REQUEST_STATE_OPEN PullRequestState = 1
	// PULL_REQUEST_STATE_CLOSED: closed without a merge.
	PullRequestState_PULL_REQUEST_STATE_CLOSED PullRequestState = 2
	// PULL_REQUEST_STATE_MERGED: merged into its base branch.
	PullRequestState_PULL_REQUEST_STATE_MERGED PullRequestState = 3
)

// Enum value maps for PullRequestState.
var (
	PullRequestState_name = map[int32]string{
		0: "PULL_REQUEST_STATE_UNSPECIFIED",
		1: "PULL_REQUEST_STATE_OPEN",
		2: "PULL_REQUEST_STATE_CLOSED",
		3: "PULL_REQUEST_STATE_MERGED",
	}
	PullRequestState_value = map[string]int32{
		"PULL_REQUEST_STATE_UNSPECIFIED": 0,
		"PULL_REQUEST_STATE_OPEN":        1,
		"PULL_REQUEST_STATE_CLOSED":      2,
		"PULL_REQUEST_STATE_MERGED":      3,
	}
)

func (x PullRequestState) Enum() *PullRequestState {
	p := new(PullRequestState)
	*p = x
	return p
}

func (x PullRequestState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PullRequestState) Descriptor() protoreflect.EnumDescriptor {
	return file_numinance_ports_scm_v1_scm_proto_enumTypes[0].Descriptor()
}

func (PullRequestState) Type() protoreflect.EnumType {
	return &file_numinance_ports_scm_v1_scm_proto_enumTypes[0]
}

func (x PullRequestState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use PullRequestState.Descriptor instead.
func (PullRequestState) EnumDescriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{0}
}

// Mergeability is whether the host can merge a pull request now.
type Mergeability int32

const (
	// MERGEABILITY_UNSPECIFIED is the zero value; it is rejected.
	Mergeability_MERGEABILITY_UNSPECIFIED Mergeability = 0
	// MERGEABILITY_MERGEABLE: the host reports no conflict.
	Mergeability_MERGEABILITY_MERGEABLE Mergeability = 1
	// MERGEABILITY_CONFLICTING: the head conflicts with the base.
	Mergeability_MERGEABILITY_CONFLICTING Mergeability = 2
	// MERGEABILITY_UNKNOWN: the host has not computed it yet; ask again.
	Mergeability_MERGEABILITY_UNKNOWN Mergeability = 3
)

// Enum value maps for Mergeability.
var (
	Mergeability_name = map[int32]string{
		0: "MERGEABILITY_UNSPECIFIED",
		1: "MERGEABILITY_MERGEABLE",
		2: "MERGEABILITY_CONFLICTING",
		3: "MERGEABILITY_UNKNOWN",
	}
	Mergeability_value = map[string]int32{
		"MERGEABILITY_UNSPECIFIED": 0,
		"MERGEABILITY_MERGEABLE":   1,
		"MERGEABILITY_CONFLICTING": 2,
		"MERGEABILITY_UNKNOWN":     3,
	}
)

func (x Mergeability) Enum() *Mergeability {
	p := new(Mergeability)
	*p = x
	return p
}

func (x Mergeability) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Mergeability) Descriptor() protoreflect.EnumDescriptor {
	return file_numinance_ports_scm_v1_scm_proto_enumTypes[1].Descriptor()
}

func (Mergeability) Type() protoreflect.EnumType {
	return &file_numinance_ports_scm_v1_scm_proto_enumTypes[1]
}

func (x Mergeability) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Mergeability.Descriptor instead.
func (Mergeability) EnumDescriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{1}
}

// CheckConclusion is the outcome of one status check on the head of a pull request.
type CheckConclusion int32

const (
	// CHECK_CONCLUSION_UNSPECIFIED is the zero value; it is rejected.
	CheckConclusion_CHECK_CONCLUSION_UNSPECIFIED CheckConclusion = 0
	// CHECK_CONCLUSION_PENDING: queued or running.
	CheckConclusion_CHECK_CONCLUSION_PENDING CheckConclusion = 1
	// CHECK_CONCLUSION_SUCCESS: passed.
	CheckConclusion_CHECK_CONCLUSION_SUCCESS CheckConclusion = 2
	// CHECK_CONCLUSION_FAILURE: failed, timed out or needs action.
	CheckConclusion_CHECK_CONCLUSION_FAILURE CheckConclusion = 3
	// CHECK_CONCLUSION_NEUTRAL: ended with no verdict (neutral, skipped, cancelled).
	CheckConclusion_CHECK_CONCLUSION_NEUTRAL CheckConclusion = 4
)

// Enum value maps for CheckConclusion.
var (
	CheckConclusion_name = map[int32]string{
		0: "CHECK_CONCLUSION_UNSPECIFIED",
		1: "CHECK_CONCLUSION_PENDING",
		2: "CHECK_CONCLUSION_SUCCESS",
		3: "CHECK_CONCLUSION_FAILURE",
		4: "CHECK_CONCLUSION_NEUTRAL",
	}
	CheckConclusion_value = map[string]int32{
		"CHECK_CONCLUSION_UNSPECIFIED": 0,
		"CHECK_CONCLUSION_PENDING":     1,
		"CHECK_CONCLUSION_SUCCESS":     2,
		"CHECK_CONCLUSION_FAILURE":     3,
		"CHECK_CONCLUSION_NEUTRAL":     4,
	}
)

func (x CheckConclusion) Enum() *CheckConclusion {
	p := new(CheckConclusion)
	*p = x
	return p
}

func (x CheckConclusion) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (CheckConclusion) Descriptor() protoreflect.EnumDescriptor {
	return file_numinance_ports_scm_v1_scm_proto_enumTypes[2].Descriptor()
}

func (CheckConclusion) Type() protoreflect.EnumType {
	return &file_numinance_ports_scm_v1_scm_proto_enumTypes[2]
}

func (x CheckConclusion) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use CheckConclusion.Descriptor instead.
func (CheckConclusion) EnumDescriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{2}
}

// GetRepositoryRequest names the repository.
type GetRepositoryRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the repository's full name, in the grammar of the file comment (Formats). Marked as the resource of the call.
	Repository    string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetRepositoryRequest) Reset() {
	*x = GetRepositoryRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetRepositoryRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRepositoryRequest) ProtoMessage() {}

func (x *GetRepositoryRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRepositoryRequest.ProtoReflect.Descriptor instead.
func (*GetRepositoryRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{0}
}

func (x *GetRepositoryRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

// GetRepositoryResponse is the host's answer; every field is a claim.
type GetRepositoryResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The repository's default branch name.
	DefaultBranch string `protobuf:"bytes,1,opt,name=default_branch,json=defaultBranch,proto3" json:"default_branch,omitempty"`
	// The full commit id the default branch points to at the time of the call.
	DefaultBranchHead string `protobuf:"bytes,2,opt,name=default_branch_head,json=defaultBranchHead,proto3" json:"default_branch_head,omitempty"`
	// True when the repository is private to its organisation.
	Private bool `protobuf:"varint,3,opt,name=private,proto3" json:"private,omitempty"`
	// True when the host has archived the repository (read-only).
	Archived      bool `protobuf:"varint,4,opt,name=archived,proto3" json:"archived,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetRepositoryResponse) Reset() {
	*x = GetRepositoryResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetRepositoryResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRepositoryResponse) ProtoMessage() {}

func (x *GetRepositoryResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRepositoryResponse.ProtoReflect.Descriptor instead.
func (*GetRepositoryResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{1}
}

func (x *GetRepositoryResponse) GetDefaultBranch() string {
	if x != nil {
		return x.DefaultBranch
	}
	return ""
}

func (x *GetRepositoryResponse) GetDefaultBranchHead() string {
	if x != nil {
		return x.DefaultBranchHead
	}
	return ""
}

func (x *GetRepositoryResponse) GetPrivate() bool {
	if x != nil {
		return x.Private
	}
	return false
}

func (x *GetRepositoryResponse) GetArchived() bool {
	if x != nil {
		return x.Archived
	}
	return false
}

// GetTreeRequest names a directory at a revision.
type GetTreeRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: a full commit id.
	Revision string `protobuf:"bytes,2,opt,name=revision,proto3" json:"revision,omitempty"`
	// The directory, in the path grammar; empty for the repository root.
	Path string `protobuf:"bytes,3,opt,name=path,proto3" json:"path,omitempty"`
	// True to list the whole subtree.
	Recursive bool `protobuf:"varint,4,opt,name=recursive,proto3" json:"recursive,omitempty"`
	// Required, 1 to 1024: the most entries to return.
	MaxEntries    uint32 `protobuf:"varint,5,opt,name=max_entries,json=maxEntries,proto3" json:"max_entries,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetTreeRequest) Reset() {
	*x = GetTreeRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetTreeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTreeRequest) ProtoMessage() {}

func (x *GetTreeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTreeRequest.ProtoReflect.Descriptor instead.
func (*GetTreeRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{2}
}

func (x *GetTreeRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *GetTreeRequest) GetRevision() string {
	if x != nil {
		return x.Revision
	}
	return ""
}

func (x *GetTreeRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *GetTreeRequest) GetRecursive() bool {
	if x != nil {
		return x.Recursive
	}
	return false
}

func (x *GetTreeRequest) GetMaxEntries() uint32 {
	if x != nil {
		return x.MaxEntries
	}
	return 0
}

// TreeEntry is one entry of a tree.
type TreeEntry struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The path from the repository root, in the path grammar.
	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	// True when the entry is a directory.
	Directory bool `protobuf:"varint,2,opt,name=directory,proto3" json:"directory,omitempty"`
	// The object id of the entry on the host: 40 or 64 lowercase hexadecimal characters.
	ObjectId string `protobuf:"bytes,3,opt,name=object_id,json=objectId,proto3" json:"object_id,omitempty"`
	// Size in bytes of a file; 0 for a directory.
	Size          uint64 `protobuf:"varint,4,opt,name=size,proto3" json:"size,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TreeEntry) Reset() {
	*x = TreeEntry{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TreeEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TreeEntry) ProtoMessage() {}

func (x *TreeEntry) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TreeEntry.ProtoReflect.Descriptor instead.
func (*TreeEntry) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{3}
}

func (x *TreeEntry) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *TreeEntry) GetDirectory() bool {
	if x != nil {
		return x.Directory
	}
	return false
}

func (x *TreeEntry) GetObjectId() string {
	if x != nil {
		return x.ObjectId
	}
	return ""
}

func (x *TreeEntry) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

// GetTreeResponse lists a tree.
type GetTreeResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// At most max_entries entries, ordered by path.
	Entries []*TreeEntry `protobuf:"bytes,1,rep,name=entries,proto3" json:"entries,omitempty"`
	// True when the host holds more entries than were returned.
	Truncated     bool `protobuf:"varint,2,opt,name=truncated,proto3" json:"truncated,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetTreeResponse) Reset() {
	*x = GetTreeResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetTreeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTreeResponse) ProtoMessage() {}

func (x *GetTreeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTreeResponse.ProtoReflect.Descriptor instead.
func (*GetTreeResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{4}
}

func (x *GetTreeResponse) GetEntries() []*TreeEntry {
	if x != nil {
		return x.Entries
	}
	return nil
}

func (x *GetTreeResponse) GetTruncated() bool {
	if x != nil {
		return x.Truncated
	}
	return false
}

// GetFileRequest names a file at a revision.
type GetFileRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: a full commit id.
	Revision string `protobuf:"bytes,2,opt,name=revision,proto3" json:"revision,omitempty"`
	// Required: the file, in the path grammar.
	Path string `protobuf:"bytes,3,opt,name=path,proto3" json:"path,omitempty"`
	// Required, 1 to 262144: the most content bytes to return.
	MaxBytes      uint32 `protobuf:"varint,4,opt,name=max_bytes,json=maxBytes,proto3" json:"max_bytes,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetFileRequest) Reset() {
	*x = GetFileRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetFileRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetFileRequest) ProtoMessage() {}

func (x *GetFileRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetFileRequest.ProtoReflect.Descriptor instead.
func (*GetFileRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{5}
}

func (x *GetFileRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *GetFileRequest) GetRevision() string {
	if x != nil {
		return x.Revision
	}
	return ""
}

func (x *GetFileRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *GetFileRequest) GetMaxBytes() uint32 {
	if x != nil {
		return x.MaxBytes
	}
	return 0
}

// GetFileResponse is a file's head.
type GetFileResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The object id of the file on the host.
	ObjectId string `protobuf:"bytes,1,opt,name=object_id,json=objectId,proto3" json:"object_id,omitempty"`
	// The size in bytes of the whole file.
	Size uint64 `protobuf:"varint,2,opt,name=size,proto3" json:"size,omitempty"`
	// The first min(size, max_bytes) bytes; untrusted.
	Content []byte `protobuf:"bytes,3,opt,name=content,proto3" json:"content,omitempty"`
	// True when size is above max_bytes.
	Truncated     bool `protobuf:"varint,4,opt,name=truncated,proto3" json:"truncated,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetFileResponse) Reset() {
	*x = GetFileResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetFileResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetFileResponse) ProtoMessage() {}

func (x *GetFileResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetFileResponse.ProtoReflect.Descriptor instead.
func (*GetFileResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{6}
}

func (x *GetFileResponse) GetObjectId() string {
	if x != nil {
		return x.ObjectId
	}
	return ""
}

func (x *GetFileResponse) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *GetFileResponse) GetContent() []byte {
	if x != nil {
		return x.Content
	}
	return nil
}

func (x *GetFileResponse) GetTruncated() bool {
	if x != nil {
		return x.Truncated
	}
	return false
}

// ListBranchesRequest asks for one page of branches.
type ListBranchesRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required, 1 to 256: the most branches to return.
	PageSize uint32 `protobuf:"varint,2,opt,name=page_size,json=pageSize,proto3" json:"page_size,omitempty"`
	// The token a previous answer returned, at most 1024 printable bytes; empty for the first page.
	PageToken     string `protobuf:"bytes,3,opt,name=page_token,json=pageToken,proto3" json:"page_token,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListBranchesRequest) Reset() {
	*x = ListBranchesRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListBranchesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListBranchesRequest) ProtoMessage() {}

func (x *ListBranchesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListBranchesRequest.ProtoReflect.Descriptor instead.
func (*ListBranchesRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{7}
}

func (x *ListBranchesRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *ListBranchesRequest) GetPageSize() uint32 {
	if x != nil {
		return x.PageSize
	}
	return 0
}

func (x *ListBranchesRequest) GetPageToken() string {
	if x != nil {
		return x.PageToken
	}
	return ""
}

// Branch is a branch and the commit it points to.
type Branch struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The branch name, in the branch grammar.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The full commit id the branch points to.
	HeadRevision  string `protobuf:"bytes,2,opt,name=head_revision,json=headRevision,proto3" json:"head_revision,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Branch) Reset() {
	*x = Branch{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Branch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Branch) ProtoMessage() {}

func (x *Branch) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Branch.ProtoReflect.Descriptor instead.
func (*Branch) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{8}
}

func (x *Branch) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Branch) GetHeadRevision() string {
	if x != nil {
		return x.HeadRevision
	}
	return ""
}

// ListBranchesResponse is one page of branches.
type ListBranchesResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// At most page_size branches.
	Branches []*Branch `protobuf:"bytes,1,rep,name=branches,proto3" json:"branches,omitempty"`
	// The token of the next page; empty at the last page. Opaque and untrusted.
	NextPageToken string `protobuf:"bytes,2,opt,name=next_page_token,json=nextPageToken,proto3" json:"next_page_token,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListBranchesResponse) Reset() {
	*x = ListBranchesResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListBranchesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListBranchesResponse) ProtoMessage() {}

func (x *ListBranchesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListBranchesResponse.ProtoReflect.Descriptor instead.
func (*ListBranchesResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{9}
}

func (x *ListBranchesResponse) GetBranches() []*Branch {
	if x != nil {
		return x.Branches
	}
	return nil
}

func (x *ListBranchesResponse) GetNextPageToken() string {
	if x != nil {
		return x.NextPageToken
	}
	return ""
}

// CreateBranchRequest names the branch and its start.
type CreateBranchRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the new branch's name.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// Required: the full commit id the branch starts at.
	FromRevision  string `protobuf:"bytes,3,opt,name=from_revision,json=fromRevision,proto3" json:"from_revision,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateBranchRequest) Reset() {
	*x = CreateBranchRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreateBranchRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateBranchRequest) ProtoMessage() {}

func (x *CreateBranchRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateBranchRequest.ProtoReflect.Descriptor instead.
func (*CreateBranchRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{10}
}

func (x *CreateBranchRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *CreateBranchRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CreateBranchRequest) GetFromRevision() string {
	if x != nil {
		return x.FromRevision
	}
	return ""
}

// CreateBranchResponse is the created or existing branch.
type CreateBranchResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The branch as the host holds it after the call.
	Branch        *Branch `protobuf:"bytes,1,opt,name=branch,proto3" json:"branch,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateBranchResponse) Reset() {
	*x = CreateBranchResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreateBranchResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateBranchResponse) ProtoMessage() {}

func (x *CreateBranchResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateBranchResponse.ProtoReflect.Descriptor instead.
func (*CreateBranchResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{11}
}

func (x *CreateBranchResponse) GetBranch() *Branch {
	if x != nil {
		return x.Branch
	}
	return nil
}

// GetBranchProtectionRequest names a branch.
type GetBranchProtectionRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the branch.
	Branch        string `protobuf:"bytes,2,opt,name=branch,proto3" json:"branch,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetBranchProtectionRequest) Reset() {
	*x = GetBranchProtectionRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[12]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetBranchProtectionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBranchProtectionRequest) ProtoMessage() {}

func (x *GetBranchProtectionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[12]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBranchProtectionRequest.ProtoReflect.Descriptor instead.
func (*GetBranchProtectionRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{12}
}

func (x *GetBranchProtectionRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *GetBranchProtectionRequest) GetBranch() string {
	if x != nil {
		return x.Branch
	}
	return ""
}

// GetBranchProtectionResponse is the protection in force on a branch; every field is the host's claim.
type GetBranchProtectionResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// True when the host protects the branch at all.
	Protected bool `protobuf:"varint,1,opt,name=protected,proto3" json:"protected,omitempty"`
	// The number of approving reviews the host requires before a merge; 0 when none.
	RequiredReviews uint32 `protobuf:"varint,2,opt,name=required_reviews,json=requiredReviews,proto3" json:"required_reviews,omitempty"`
	// The names of the checks that must pass before a merge, at most 64, unique, each at most 256 bytes.
	RequiredChecks []string `protobuf:"bytes,3,rep,name=required_checks,json=requiredChecks,proto3" json:"required_checks,omitempty"`
	// True when the rules bind repository administrators as well.
	EnforceAdmins bool `protobuf:"varint,4,opt,name=enforce_admins,json=enforceAdmins,proto3" json:"enforce_admins,omitempty"`
	// True when the host allows a force push to the branch.
	AllowForcePush bool `protobuf:"varint,5,opt,name=allow_force_push,json=allowForcePush,proto3" json:"allow_force_push,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *GetBranchProtectionResponse) Reset() {
	*x = GetBranchProtectionResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[13]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetBranchProtectionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBranchProtectionResponse) ProtoMessage() {}

func (x *GetBranchProtectionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[13]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBranchProtectionResponse.ProtoReflect.Descriptor instead.
func (*GetBranchProtectionResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{13}
}

func (x *GetBranchProtectionResponse) GetProtected() bool {
	if x != nil {
		return x.Protected
	}
	return false
}

func (x *GetBranchProtectionResponse) GetRequiredReviews() uint32 {
	if x != nil {
		return x.RequiredReviews
	}
	return 0
}

func (x *GetBranchProtectionResponse) GetRequiredChecks() []string {
	if x != nil {
		return x.RequiredChecks
	}
	return nil
}

func (x *GetBranchProtectionResponse) GetEnforceAdmins() bool {
	if x != nil {
		return x.EnforceAdmins
	}
	return false
}

func (x *GetBranchProtectionResponse) GetAllowForcePush() bool {
	if x != nil {
		return x.AllowForcePush
	}
	return false
}

// OpenPullRequestRequest is one pull request to open.
type OpenPullRequestRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the branch that holds the candidate.
	HeadBranch string `protobuf:"bytes,2,opt,name=head_branch,json=headBranch,proto3" json:"head_branch,omitempty"`
	// Required: the full commit id of the candidate; head_branch must point at it.
	HeadRevision string `protobuf:"bytes,3,opt,name=head_revision,json=headRevision,proto3" json:"head_revision,omitempty"`
	// Required: the branch the pull request merges into.
	BaseBranch string `protobuf:"bytes,4,opt,name=base_branch,json=baseBranch,proto3" json:"base_branch,omitempty"`
	// Required: 1 to 256 printable bytes. Text the Engine composes from the Change; rendered as data on the host.
	Title string `protobuf:"bytes,5,opt,name=title,proto3" json:"title,omitempty"`
	// At most 16384 bytes of printable characters, U+0009 and U+000A. The Engine composes it from references and labels any untrusted quotation; never an instruction.
	Body          string `protobuf:"bytes,6,opt,name=body,proto3" json:"body,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *OpenPullRequestRequest) Reset() {
	*x = OpenPullRequestRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[14]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OpenPullRequestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OpenPullRequestRequest) ProtoMessage() {}

func (x *OpenPullRequestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[14]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OpenPullRequestRequest.ProtoReflect.Descriptor instead.
func (*OpenPullRequestRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{14}
}

func (x *OpenPullRequestRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *OpenPullRequestRequest) GetHeadBranch() string {
	if x != nil {
		return x.HeadBranch
	}
	return ""
}

func (x *OpenPullRequestRequest) GetHeadRevision() string {
	if x != nil {
		return x.HeadRevision
	}
	return ""
}

func (x *OpenPullRequestRequest) GetBaseBranch() string {
	if x != nil {
		return x.BaseBranch
	}
	return ""
}

func (x *OpenPullRequestRequest) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *OpenPullRequestRequest) GetBody() string {
	if x != nil {
		return x.Body
	}
	return ""
}

// PullRequest is a pull request as the host reports it.
type PullRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The host's number of the pull request, at least 1 and at most 2^53-1.
	Number uint64 `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	// The pull request's state.
	State PullRequestState `protobuf:"varint,2,opt,name=state,proto3,enum=numinance.ports.scm.v1.PullRequestState" json:"state,omitempty"`
	// The branch that holds the candidate.
	HeadBranch string `protobuf:"bytes,3,opt,name=head_branch,json=headBranch,proto3" json:"head_branch,omitempty"`
	// The full commit id of the head now.
	HeadRevision string `protobuf:"bytes,4,opt,name=head_revision,json=headRevision,proto3" json:"head_revision,omitempty"`
	// The branch the pull request merges into.
	BaseBranch string `protobuf:"bytes,5,opt,name=base_branch,json=baseBranch,proto3" json:"base_branch,omitempty"`
	// The address of the pull request on the host: an absolute https URL of at most 1024 bytes, for people; untrusted, and never fetched by the Engine.
	Url           string `protobuf:"bytes,6,opt,name=url,proto3" json:"url,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PullRequest) Reset() {
	*x = PullRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[15]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PullRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullRequest) ProtoMessage() {}

func (x *PullRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[15]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullRequest.ProtoReflect.Descriptor instead.
func (*PullRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{15}
}

func (x *PullRequest) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *PullRequest) GetState() PullRequestState {
	if x != nil {
		return x.State
	}
	return PullRequestState_PULL_REQUEST_STATE_UNSPECIFIED
}

func (x *PullRequest) GetHeadBranch() string {
	if x != nil {
		return x.HeadBranch
	}
	return ""
}

func (x *PullRequest) GetHeadRevision() string {
	if x != nil {
		return x.HeadRevision
	}
	return ""
}

func (x *PullRequest) GetBaseBranch() string {
	if x != nil {
		return x.BaseBranch
	}
	return ""
}

func (x *PullRequest) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

// OpenPullRequestResponse is the opened or found pull request.
type OpenPullRequestResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The pull request.
	PullRequest *PullRequest `protobuf:"bytes,1,opt,name=pull_request,json=pullRequest,proto3" json:"pull_request,omitempty"`
	// True when an open pull request for the same head and base already existed and nothing was opened.
	Existing      bool `protobuf:"varint,2,opt,name=existing,proto3" json:"existing,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *OpenPullRequestResponse) Reset() {
	*x = OpenPullRequestResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[16]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OpenPullRequestResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OpenPullRequestResponse) ProtoMessage() {}

func (x *OpenPullRequestResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[16]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OpenPullRequestResponse.ProtoReflect.Descriptor instead.
func (*OpenPullRequestResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{16}
}

func (x *OpenPullRequestResponse) GetPullRequest() *PullRequest {
	if x != nil {
		return x.PullRequest
	}
	return nil
}

func (x *OpenPullRequestResponse) GetExisting() bool {
	if x != nil {
		return x.Existing
	}
	return false
}

// UpdatePullRequestRequest names the fields to change.
type UpdatePullRequestRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the pull request's number.
	Number uint64 `protobuf:"varint,2,opt,name=number,proto3" json:"number,omitempty"`
	// The new title, 1 to 256 printable bytes; empty leaves it as it is.
	Title string `protobuf:"bytes,3,opt,name=title,proto3" json:"title,omitempty"`
	// The new body, at most 16384 bytes; empty leaves it as it is.
	Body string `protobuf:"bytes,4,opt,name=body,proto3" json:"body,omitempty"`
	// The new state: unspecified leaves it, CLOSED closes it; OPEN and MERGED are rejected.
	State         PullRequestState `protobuf:"varint,5,opt,name=state,proto3,enum=numinance.ports.scm.v1.PullRequestState" json:"state,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *UpdatePullRequestRequest) Reset() {
	*x = UpdatePullRequestRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[17]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UpdatePullRequestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdatePullRequestRequest) ProtoMessage() {}

func (x *UpdatePullRequestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[17]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdatePullRequestRequest.ProtoReflect.Descriptor instead.
func (*UpdatePullRequestRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{17}
}

func (x *UpdatePullRequestRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *UpdatePullRequestRequest) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *UpdatePullRequestRequest) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *UpdatePullRequestRequest) GetBody() string {
	if x != nil {
		return x.Body
	}
	return ""
}

func (x *UpdatePullRequestRequest) GetState() PullRequestState {
	if x != nil {
		return x.State
	}
	return PullRequestState_PULL_REQUEST_STATE_UNSPECIFIED
}

// UpdatePullRequestResponse is the pull request after the update.
type UpdatePullRequestResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The pull request.
	PullRequest   *PullRequest `protobuf:"bytes,1,opt,name=pull_request,json=pullRequest,proto3" json:"pull_request,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *UpdatePullRequestResponse) Reset() {
	*x = UpdatePullRequestResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[18]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UpdatePullRequestResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdatePullRequestResponse) ProtoMessage() {}

func (x *UpdatePullRequestResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[18]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdatePullRequestResponse.ProtoReflect.Descriptor instead.
func (*UpdatePullRequestResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{18}
}

func (x *UpdatePullRequestResponse) GetPullRequest() *PullRequest {
	if x != nil {
		return x.PullRequest
	}
	return nil
}

// CommentOnPullRequestRequest is one comment.
type CommentOnPullRequestRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the pull request's number.
	Number uint64 `protobuf:"varint,2,opt,name=number,proto3" json:"number,omitempty"`
	// Required: 1 to 16384 bytes of printable characters, U+0009 and U+000A.
	Body          string `protobuf:"bytes,3,opt,name=body,proto3" json:"body,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CommentOnPullRequestRequest) Reset() {
	*x = CommentOnPullRequestRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[19]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CommentOnPullRequestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CommentOnPullRequestRequest) ProtoMessage() {}

func (x *CommentOnPullRequestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[19]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CommentOnPullRequestRequest.ProtoReflect.Descriptor instead.
func (*CommentOnPullRequestRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{19}
}

func (x *CommentOnPullRequestRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *CommentOnPullRequestRequest) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *CommentOnPullRequestRequest) GetBody() string {
	if x != nil {
		return x.Body
	}
	return ""
}

// CommentOnPullRequestResponse names the comment.
type CommentOnPullRequestResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The host's id of the comment, at most 128 printable bytes; opaque.
	CommentId     string `protobuf:"bytes,1,opt,name=comment_id,json=commentId,proto3" json:"comment_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CommentOnPullRequestResponse) Reset() {
	*x = CommentOnPullRequestResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[20]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CommentOnPullRequestResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CommentOnPullRequestResponse) ProtoMessage() {}

func (x *CommentOnPullRequestResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[20]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CommentOnPullRequestResponse.ProtoReflect.Descriptor instead.
func (*CommentOnPullRequestResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{20}
}

func (x *CommentOnPullRequestResponse) GetCommentId() string {
	if x != nil {
		return x.CommentId
	}
	return ""
}

// GetPullRequestStatusRequest names a pull request.
type GetPullRequestStatusRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the pull request's number.
	Number        uint64 `protobuf:"varint,2,opt,name=number,proto3" json:"number,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetPullRequestStatusRequest) Reset() {
	*x = GetPullRequestStatusRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[21]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetPullRequestStatusRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetPullRequestStatusRequest) ProtoMessage() {}

func (x *GetPullRequestStatusRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[21]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetPullRequestStatusRequest.ProtoReflect.Descriptor instead.
func (*GetPullRequestStatusRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{21}
}

func (x *GetPullRequestStatusRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *GetPullRequestStatusRequest) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

// CheckRun is one status check on the head of a pull request.
type CheckRun struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The check's name, 1 to 256 printable bytes; untrusted.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The check's outcome.
	Conclusion    CheckConclusion `protobuf:"varint,2,opt,name=conclusion,proto3,enum=numinance.ports.scm.v1.CheckConclusion" json:"conclusion,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CheckRun) Reset() {
	*x = CheckRun{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[22]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CheckRun) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CheckRun) ProtoMessage() {}

func (x *CheckRun) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[22]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CheckRun.ProtoReflect.Descriptor instead.
func (*CheckRun) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{22}
}

func (x *CheckRun) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CheckRun) GetConclusion() CheckConclusion {
	if x != nil {
		return x.Conclusion
	}
	return CheckConclusion_CHECK_CONCLUSION_UNSPECIFIED
}

// GetPullRequestStatusResponse is the state of a pull request; every field is the host's claim.
type GetPullRequestStatusResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The pull request as the host holds it now.
	PullRequest *PullRequest `protobuf:"bytes,1,opt,name=pull_request,json=pullRequest,proto3" json:"pull_request,omitempty"`
	// Whether the host can merge it.
	Mergeability Mergeability `protobuf:"varint,2,opt,name=mergeability,proto3,enum=numinance.ports.scm.v1.Mergeability" json:"mergeability,omitempty"`
	// The checks of the head, at most 256, unique by name.
	Checks []*CheckRun `protobuf:"bytes,3,rep,name=checks,proto3" json:"checks,omitempty"`
	// The number of distinct reviewers whose latest review approves the head.
	Approvals uint32 `protobuf:"varint,4,opt,name=approvals,proto3" json:"approvals,omitempty"`
	// True when a reviewer's latest review requests changes.
	ChangesRequested bool `protobuf:"varint,5,opt,name=changes_requested,json=changesRequested,proto3" json:"changes_requested,omitempty"`
	// The merge commit's full id when state is MERGED; empty otherwise.
	MergeRevision string `protobuf:"bytes,6,opt,name=merge_revision,json=mergeRevision,proto3" json:"merge_revision,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetPullRequestStatusResponse) Reset() {
	*x = GetPullRequestStatusResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[23]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetPullRequestStatusResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetPullRequestStatusResponse) ProtoMessage() {}

func (x *GetPullRequestStatusResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[23]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetPullRequestStatusResponse.ProtoReflect.Descriptor instead.
func (*GetPullRequestStatusResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{23}
}

func (x *GetPullRequestStatusResponse) GetPullRequest() *PullRequest {
	if x != nil {
		return x.PullRequest
	}
	return nil
}

func (x *GetPullRequestStatusResponse) GetMergeability() Mergeability {
	if x != nil {
		return x.Mergeability
	}
	return Mergeability_MERGEABILITY_UNSPECIFIED
}

func (x *GetPullRequestStatusResponse) GetChecks() []*CheckRun {
	if x != nil {
		return x.Checks
	}
	return nil
}

func (x *GetPullRequestStatusResponse) GetApprovals() uint32 {
	if x != nil {
		return x.Approvals
	}
	return 0
}

func (x *GetPullRequestStatusResponse) GetChangesRequested() bool {
	if x != nil {
		return x.ChangesRequested
	}
	return false
}

func (x *GetPullRequestStatusResponse) GetMergeRevision() string {
	if x != nil {
		return x.MergeRevision
	}
	return ""
}

// MergePullRequestRequest is one merge.
type MergePullRequestRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the pull request's number.
	Number uint64 `protobuf:"varint,2,opt,name=number,proto3" json:"number,omitempty"`
	// Required: the full commit id of the head the caller verified; a moved head is FAILED_PRECONDITION.
	ExpectedHeadRevision string `protobuf:"bytes,3,opt,name=expected_head_revision,json=expectedHeadRevision,proto3" json:"expected_head_revision,omitempty"`
	// Required: the branch the pull request was opened into; a pull request that now merges into another branch is FAILED_PRECONDITION, so the merge is into the base whose protection was read.
	ExpectedBaseBranch string `protobuf:"bytes,4,opt,name=expected_base_branch,json=expectedBaseBranch,proto3" json:"expected_base_branch,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *MergePullRequestRequest) Reset() {
	*x = MergePullRequestRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[24]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MergePullRequestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MergePullRequestRequest) ProtoMessage() {}

func (x *MergePullRequestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[24]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MergePullRequestRequest.ProtoReflect.Descriptor instead.
func (*MergePullRequestRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{24}
}

func (x *MergePullRequestRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *MergePullRequestRequest) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *MergePullRequestRequest) GetExpectedHeadRevision() string {
	if x != nil {
		return x.ExpectedHeadRevision
	}
	return ""
}

func (x *MergePullRequestRequest) GetExpectedBaseBranch() string {
	if x != nil {
		return x.ExpectedBaseBranch
	}
	return ""
}

// MergePullRequestResponse is the merge outcome.
type MergePullRequestResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The full commit id of the merge on the base branch; empty for a dry run.
	MergeRevision string `protobuf:"bytes,1,opt,name=merge_revision,json=mergeRevision,proto3" json:"merge_revision,omitempty"`
	// True when the pull request was already merged by an earlier call and nothing was merged now.
	AlreadyMerged bool `protobuf:"varint,2,opt,name=already_merged,json=alreadyMerged,proto3" json:"already_merged,omitempty"`
	// The time the host merged it; unset for a dry run.
	MergedAt *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=merged_at,json=mergedAt,proto3" json:"merged_at,omitempty"`
	// The full commit id of the head the merge was of; equal to expected_head_revision, and empty for a dry run. The caller compares it: a merge already done at another head is not the release of the verified candidate.
	HeadRevision  string `protobuf:"bytes,4,opt,name=head_revision,json=headRevision,proto3" json:"head_revision,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MergePullRequestResponse) Reset() {
	*x = MergePullRequestResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[25]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MergePullRequestResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MergePullRequestResponse) ProtoMessage() {}

func (x *MergePullRequestResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[25]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MergePullRequestResponse.ProtoReflect.Descriptor instead.
func (*MergePullRequestResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{25}
}

func (x *MergePullRequestResponse) GetMergeRevision() string {
	if x != nil {
		return x.MergeRevision
	}
	return ""
}

func (x *MergePullRequestResponse) GetAlreadyMerged() bool {
	if x != nil {
		return x.AlreadyMerged
	}
	return false
}

func (x *MergePullRequestResponse) GetMergedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.MergedAt
	}
	return nil
}

func (x *MergePullRequestResponse) GetHeadRevision() string {
	if x != nil {
		return x.HeadRevision
	}
	return ""
}

// SearchCodeRequest is one search.
type SearchCodeRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required, as in GetRepositoryRequest. Marked as the resource of the call.
	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	// Required: the search text, 1 to 256 printable bytes.
	Query string `protobuf:"bytes,2,opt,name=query,proto3" json:"query,omitempty"`
	// Required, 1 to 100: the most paths to return.
	MaxResults    uint32 `protobuf:"varint,3,opt,name=max_results,json=maxResults,proto3" json:"max_results,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SearchCodeRequest) Reset() {
	*x = SearchCodeRequest{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[26]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SearchCodeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchCodeRequest) ProtoMessage() {}

func (x *SearchCodeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[26]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchCodeRequest.ProtoReflect.Descriptor instead.
func (*SearchCodeRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{26}
}

func (x *SearchCodeRequest) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *SearchCodeRequest) GetQuery() string {
	if x != nil {
		return x.Query
	}
	return ""
}

func (x *SearchCodeRequest) GetMaxResults() uint32 {
	if x != nil {
		return x.MaxResults
	}
	return 0
}

// SearchCodeResponse lists matching paths.
type SearchCodeResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// At most max_results paths on the default branch, in the path grammar.
	Paths         []string `protobuf:"bytes,1,rep,name=paths,proto3" json:"paths,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SearchCodeResponse) Reset() {
	*x = SearchCodeResponse{}
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[27]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SearchCodeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchCodeResponse) ProtoMessage() {}

func (x *SearchCodeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_scm_v1_scm_proto_msgTypes[27]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchCodeResponse.ProtoReflect.Descriptor instead.
func (*SearchCodeResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_scm_v1_scm_proto_rawDescGZIP(), []int{27}
}

func (x *SearchCodeResponse) GetPaths() []string {
	if x != nil {
		return x.Paths
	}
	return nil
}

var File_numinance_ports_scm_v1_scm_proto protoreflect.FileDescriptor

const file_numinance_ports_scm_v1_scm_proto_rawDesc = "" +
	"\n" +
	" numinance/ports/scm/v1/scm.proto\x12\x16numinance.ports.scm.v1\x1a\x1fgoogle/protobuf/timestamp.proto\x1a*numinance/annotations/v1/annotations.proto\"E\n" +
	"\x14GetRepositoryRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\"\xd8\x01\n" +
	"\x15GetRepositoryResponse\x122\n" +
	"\x0edefault_branch\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rdefaultBranch\x12;\n" +
	"\x13default_branch_head\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x11defaultBranchHead\x12%\n" +
	"\aprivate\x18\x03 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aprivate\x12'\n" +
	"\barchived\x18\x04 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\barchived\"\xe2\x01\n" +
	"\x0eGetTreeRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12'\n" +
	"\brevision\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\brevision\x12\x1f\n" +
	"\x04path\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04path\x12)\n" +
	"\trecursive\x18\x04 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\trecursive\x12,\n" +
	"\vmax_entries\x18\x05 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"maxEntries\"\xa2\x01\n" +
	"\tTreeEntry\x12\x1f\n" +
	"\x04path\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04path\x12)\n" +
	"\tdirectory\x18\x02 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tdirectory\x12(\n" +
	"\tobject_id\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bobjectId\x12\x1f\n" +
	"\x04size\x18\x04 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04size\"\x86\x01\n" +
	"\x0fGetTreeResponse\x12H\n" +
	"\aentries\x18\x01 \x03(\v2!.numinance.ports.scm.v1.TreeEntryB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aentries\x12)\n" +
	"\ttruncated\x18\x02 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ttruncated\"\xb3\x01\n" +
	"\x0eGetFileRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12'\n" +
	"\brevision\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\brevision\x12\x1f\n" +
	"\x04path\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04path\x12(\n" +
	"\tmax_bytes\x18\x04 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bmaxBytes\"\xae\x01\n" +
	"\x0fGetFileResponse\x12(\n" +
	"\tobject_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bobjectId\x12\x1f\n" +
	"\x04size\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04size\x12%\n" +
	"\acontent\x18\x03 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\acontent\x12)\n" +
	"\ttruncated\x18\x04 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ttruncated\"\x9a\x01\n" +
	"\x13ListBranchesRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12(\n" +
	"\tpage_size\x18\x02 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bpageSize\x12*\n" +
	"\n" +
	"page_token\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tpageToken\"[\n" +
	"\x06Branch\x12\x1f\n" +
	"\x04name\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04name\x120\n" +
	"\rhead_revision\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fheadRevision\"\x94\x01\n" +
	"\x14ListBranchesResponse\x12G\n" +
	"\bbranches\x18\x01 \x03(\v2\x1e.numinance.ports.scm.v1.BranchB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bbranches\x123\n" +
	"\x0fnext_page_token\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rnextPageToken\"\x97\x01\n" +
	"\x13CreateBranchRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12\x1f\n" +
	"\x04name\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04name\x120\n" +
	"\rfrom_revision\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ffromRevision\"[\n" +
	"\x14CreateBranchResponse\x12C\n" +
	"\x06branch\x18\x01 \x01(\v2\x1e.numinance.ports.scm.v1.BranchB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06branch\"p\n" +
	"\x1aGetBranchProtectionRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12#\n" +
	"\x06branch\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06branch\"\xa1\x02\n" +
	"\x1bGetBranchProtectionResponse\x12)\n" +
	"\tprotected\x18\x01 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tprotected\x126\n" +
	"\x10required_reviews\x18\x02 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x0frequiredReviews\x124\n" +
	"\x0frequired_checks\x18\x03 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x0erequiredChecks\x122\n" +
	"\x0eenforce_admins\x18\x04 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\renforceAdmins\x125\n" +
	"\x10allow_force_push\x18\x05 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x0eallowForcePush\"\x99\x02\n" +
	"\x16OpenPullRequestRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12,\n" +
	"\vhead_branch\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"headBranch\x120\n" +
	"\rhead_revision\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fheadRevision\x12,\n" +
	"\vbase_branch\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"baseBranch\x12!\n" +
	"\x05title\x18\x05 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05title\x12\x1f\n" +
	"\x04body\x18\x06 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04body\"\xac\x02\n" +
	"\vPullRequest\x12#\n" +
	"\x06number\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06number\x12K\n" +
	"\x05state\x18\x02 \x01(\x0e2(.numinance.ports.scm.v1.PullRequestStateB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05state\x12,\n" +
	"\vhead_branch\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"headBranch\x120\n" +
	"\rhead_revision\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fheadRevision\x12,\n" +
	"\vbase_branch\x18\x05 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"baseBranch\x12\x1d\n" +
	"\x03url\x18\x06 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x03url\"\x97\x01\n" +
	"\x17OpenPullRequestResponse\x12S\n" +
	"\fpull_request\x18\x01 \x01(\v2#.numinance.ports.scm.v1.PullRequestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vpullRequest\x12'\n" +
	"\bexisting\x18\x02 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bexisting\"\xff\x01\n" +
	"\x18UpdatePullRequestRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12#\n" +
	"\x06number\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06number\x12!\n" +
	"\x05title\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05title\x12\x1f\n" +
	"\x04body\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04body\x12K\n" +
	"\x05state\x18\x05 \x01(\x0e2(.numinance.ports.scm.v1.PullRequestStateB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05state\"p\n" +
	"\x19UpdatePullRequestResponse\x12S\n" +
	"\fpull_request\x18\x01 \x01(\v2#.numinance.ports.scm.v1.PullRequestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vpullRequest\"\x92\x01\n" +
	"\x1bCommentOnPullRequestRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12#\n" +
	"\x06number\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06number\x12\x1f\n" +
	"\x04body\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04body\"J\n" +
	"\x1cCommentOnPullRequestResponse\x12*\n" +
	"\n" +
	"comment_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tcommentId\"q\n" +
	"\x1bGetPullRequestStatusRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12#\n" +
	"\x06number\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06number\"\x81\x01\n" +
	"\bCheckRun\x12\x1f\n" +
	"\x04name\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04name\x12T\n" +
	"\n" +
	"conclusion\x18\x02 \x01(\x0e2'.numinance.ports.scm.v1.CheckConclusionB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"conclusion\"\xaa\x03\n" +
	"\x1cGetPullRequestStatusResponse\x12S\n" +
	"\fpull_request\x18\x01 \x01(\v2#.numinance.ports.scm.v1.PullRequestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vpullRequest\x12U\n" +
	"\fmergeability\x18\x02 \x01(\x0e2$.numinance.ports.scm.v1.MergeabilityB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fmergeability\x12E\n" +
	"\x06checks\x18\x03 \x03(\v2 .numinance.ports.scm.v1.CheckRunB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06checks\x12)\n" +
	"\tapprovals\x18\x04 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tapprovals\x128\n" +
	"\x11changes_requested\x18\x05 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x10changesRequested\x122\n" +
	"\x0emerge_revision\x18\x06 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rmergeRevision\"\xef\x01\n" +
	"\x17MergePullRequestRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12#\n" +
	"\x06number\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06number\x12A\n" +
	"\x16expected_head_revision\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x14expectedHeadRevision\x12=\n" +
	"\x14expected_base_branch\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x12expectedBaseBranch\"\xfa\x01\n" +
	"\x18MergePullRequestResponse\x122\n" +
	"\x0emerge_revision\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rmergeRevision\x122\n" +
	"\x0ealready_merged\x18\x02 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ralreadyMerged\x12D\n" +
	"\tmerged_at\x18\x03 \x01(\v2\x1a.google.protobuf.TimestampB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bmergedAt\x120\n" +
	"\rhead_revision\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fheadRevision\"\x93\x01\n" +
	"\x11SearchCodeRequest\x12-\n" +
	"\n" +
	"repository\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.0.00\x01R\n" +
	"repository\x12!\n" +
	"\x05query\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05query\x12,\n" +
	"\vmax_results\x18\x03 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"maxResults\"7\n" +
	"\x12SearchCodeResponse\x12!\n" +
	"\x05paths\x18\x01 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05paths*\x91\x01\n" +
	"\x10PullRequestState\x12\"\n" +
	"\x1ePULL_REQUEST_STATE_UNSPECIFIED\x10\x00\x12\x1b\n" +
	"\x17PULL_REQUEST_STATE_OPEN\x10\x01\x12\x1d\n" +
	"\x19PULL_REQUEST_STATE_CLOSED\x10\x02\x12\x1d\n" +
	"\x19PULL_REQUEST_STATE_MERGED\x10\x03*\x80\x01\n" +
	"\fMergeability\x12\x1c\n" +
	"\x18MERGEABILITY_UNSPECIFIED\x10\x00\x12\x1a\n" +
	"\x16MERGEABILITY_MERGEABLE\x10\x01\x12\x1c\n" +
	"\x18MERGEABILITY_CONFLICTING\x10\x02\x12\x18\n" +
	"\x14MERGEABILITY_UNKNOWN\x10\x03*\xab\x01\n" +
	"\x0fCheckConclusion\x12 \n" +
	"\x1cCHECK_CONCLUSION_UNSPECIFIED\x10\x00\x12\x1c\n" +
	"\x18CHECK_CONCLUSION_PENDING\x10\x01\x12\x1c\n" +
	"\x18CHECK_CONCLUSION_SUCCESS\x10\x02\x12\x1c\n" +
	"\x18CHECK_CONCLUSION_FAILURE\x10\x03\x12\x1c\n" +
	"\x18CHECK_CONCLUSION_NEUTRAL\x10\x042\xb4\x12\n" +
	"\n" +
	"ScmService\x12\xc8\x01\n" +
	"\rGetRepository\x12,.numinance.ports.scm.v1.GetRepositoryRequest\x1a-.numinance.ports.scm.v1.GetRepositoryResponse\"Z\x8a\xb5\x18V\b\x01\x10\x01\x18\x01B\x051.0.0JGReturns the default branch of a repository and the commit it points to.\x12\xac\x01\n" +
	"\aGetTree\x12&.numinance.ports.scm.v1.GetTreeRequest\x1a'.numinance.ports.scm.v1.GetTreeResponse\"P\x8a\xb5\x18L\b\x01\x10\x01\x18\x01B\x051.0.0J=Lists the entries of a directory of a repository at a commit.\x12\xab\x01\n" +
	"\aGetFile\x12&.numinance.ports.scm.v1.GetFileRequest\x1a'.numinance.ports.scm.v1.GetFileResponse\"O\x8a\xb5\x18K\b\x01\x10\x01\x18\x01B\x051.0.0J<Returns the content of one file of a repository at a commit.\x12\xa1\x01\n" +
	"\fListBranches\x12+.numinance.ports.scm.v1.ListBranchesRequest\x1a,.numinance.ports.scm.v1.ListBranchesResponse\"6\x8a\xb5\x182\b\x01\x10\x01\x18\x01B\x051.0.0J#Lists the branches of a repository.\x12\xb1\x01\n" +
	"\fCreateBranch\x12+.numinance.ports.scm.v1.CreateBranchRequest\x1a,.numinance.ports.scm.v1.CreateBranchResponse\"F\x8a\xb5\x18B\b\x02\x10\x01\x18\x01B\x051.0.0J3Creates a branch of a repository at a given commit.\x12\xcd\x01\n" +
	"\x13GetBranchProtection\x122.numinance.ports.scm.v1.GetBranchProtectionRequest\x1a3.numinance.ports.scm.v1.GetBranchProtectionResponse\"M\x8a\xb5\x18I\b\x01\x10\x01\x18\x01B\x051.0.0J:Reports the branch protection of a branch of a repository.\x12\xe4\x01\n" +
	"\x0fOpenPullRequest\x12..numinance.ports.scm.v1.OpenPullRequestRequest\x1a/.numinance.ports.scm.v1.OpenPullRequestResponse\"p\x8a\xb5\x18l\b\x03\x10\x01\x18\x01 \x01*\x1cScmService.UpdatePullRequest2\x06pr-refB\x051.0.0J5Opens a pull request for a verified candidate branch.\x12\xc5\x01\n" +
	"\x11UpdatePullRequest\x120.numinance.ports.scm.v1.UpdatePullRequestRequest\x1a1.numinance.ports.scm.v1.UpdatePullRequestResponse\"K\x8a\xb5\x18G\b\x02\x10\x01\x18\x01B\x051.0.0J8Edits the title or body of a pull request, or closes it.\x12\xb9\x01\n" +
	"\x14CommentOnPullRequest\x123.numinance.ports.scm.v1.CommentOnPullRequestRequest\x1a4.numinance.ports.scm.v1.CommentOnPullRequestResponse\"6\x8a\xb5\x182\b\x02\x10\x01\x18\x01B\x051.0.0J#Adds one comment to a pull request.\x12\xce\x01\n" +
	"\x14GetPullRequestStatus\x123.numinance.ports.scm.v1.GetPullRequestStatusRequest\x1a4.numinance.ports.scm.v1.GetPullRequestStatusResponse\"K\x8a\xb5\x18G\b\x01\x10\x01\x18\x01B\x051.0.0J8Returns the state, checks and reviews of a pull request.\x12\xe1\x01\n" +
	"\x10MergePullRequest\x12/.numinance.ports.scm.v1.MergePullRequestRequest\x1a0.numinance.ports.scm.v1.MergePullRequestResponse\"j\x8a\xb5\x18f\b\x05\x10\x01\x18\x01 \x01*\x1fScmService.CommentOnPullRequestB\x051.0.0J4Merges a verified pull request into its base branch.\x12\xb5\x01\n" +
	"\n" +
	"SearchCode\x12).numinance.ports.scm.v1.SearchCodeRequest\x1a*.numinance.ports.scm.v1.SearchCodeResponse\"P\x8a\xb5\x18L\b\x01\x10\x01\x18\x01B\x051.0.0J=Searches the code of a repository and returns matching paths.B\xee\x01\n" +
	"\x1acom.numinance.ports.scm.v1B\bScmProtoP\x01ZKgithub.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1;scmv1\xa2\x02\x03NPS\xaa\x02\x16Numinance.Ports.Scm.V1\xca\x02\x16Numinance\\Ports\\Scm\\V1\xe2\x02\"Numinance\\Ports\\Scm\\V1\\GPBMetadata\xea\x02\x19Numinance::Ports::Scm::V1b\x06proto3"

var (
	file_numinance_ports_scm_v1_scm_proto_rawDescOnce sync.Once
	file_numinance_ports_scm_v1_scm_proto_rawDescData []byte
)

func file_numinance_ports_scm_v1_scm_proto_rawDescGZIP() []byte {
	file_numinance_ports_scm_v1_scm_proto_rawDescOnce.Do(func() {
		file_numinance_ports_scm_v1_scm_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_scm_v1_scm_proto_rawDesc), len(file_numinance_ports_scm_v1_scm_proto_rawDesc)))
	})
	return file_numinance_ports_scm_v1_scm_proto_rawDescData
}

var file_numinance_ports_scm_v1_scm_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
var file_numinance_ports_scm_v1_scm_proto_msgTypes = make([]protoimpl.MessageInfo, 28)
var file_numinance_ports_scm_v1_scm_proto_goTypes = []any{
	(PullRequestState)(0),                // 0: numinance.ports.scm.v1.PullRequestState
	(Mergeability)(0),                    // 1: numinance.ports.scm.v1.Mergeability
	(CheckConclusion)(0),                 // 2: numinance.ports.scm.v1.CheckConclusion
	(*GetRepositoryRequest)(nil),         // 3: numinance.ports.scm.v1.GetRepositoryRequest
	(*GetRepositoryResponse)(nil),        // 4: numinance.ports.scm.v1.GetRepositoryResponse
	(*GetTreeRequest)(nil),               // 5: numinance.ports.scm.v1.GetTreeRequest
	(*TreeEntry)(nil),                    // 6: numinance.ports.scm.v1.TreeEntry
	(*GetTreeResponse)(nil),              // 7: numinance.ports.scm.v1.GetTreeResponse
	(*GetFileRequest)(nil),               // 8: numinance.ports.scm.v1.GetFileRequest
	(*GetFileResponse)(nil),              // 9: numinance.ports.scm.v1.GetFileResponse
	(*ListBranchesRequest)(nil),          // 10: numinance.ports.scm.v1.ListBranchesRequest
	(*Branch)(nil),                       // 11: numinance.ports.scm.v1.Branch
	(*ListBranchesResponse)(nil),         // 12: numinance.ports.scm.v1.ListBranchesResponse
	(*CreateBranchRequest)(nil),          // 13: numinance.ports.scm.v1.CreateBranchRequest
	(*CreateBranchResponse)(nil),         // 14: numinance.ports.scm.v1.CreateBranchResponse
	(*GetBranchProtectionRequest)(nil),   // 15: numinance.ports.scm.v1.GetBranchProtectionRequest
	(*GetBranchProtectionResponse)(nil),  // 16: numinance.ports.scm.v1.GetBranchProtectionResponse
	(*OpenPullRequestRequest)(nil),       // 17: numinance.ports.scm.v1.OpenPullRequestRequest
	(*PullRequest)(nil),                  // 18: numinance.ports.scm.v1.PullRequest
	(*OpenPullRequestResponse)(nil),      // 19: numinance.ports.scm.v1.OpenPullRequestResponse
	(*UpdatePullRequestRequest)(nil),     // 20: numinance.ports.scm.v1.UpdatePullRequestRequest
	(*UpdatePullRequestResponse)(nil),    // 21: numinance.ports.scm.v1.UpdatePullRequestResponse
	(*CommentOnPullRequestRequest)(nil),  // 22: numinance.ports.scm.v1.CommentOnPullRequestRequest
	(*CommentOnPullRequestResponse)(nil), // 23: numinance.ports.scm.v1.CommentOnPullRequestResponse
	(*GetPullRequestStatusRequest)(nil),  // 24: numinance.ports.scm.v1.GetPullRequestStatusRequest
	(*CheckRun)(nil),                     // 25: numinance.ports.scm.v1.CheckRun
	(*GetPullRequestStatusResponse)(nil), // 26: numinance.ports.scm.v1.GetPullRequestStatusResponse
	(*MergePullRequestRequest)(nil),      // 27: numinance.ports.scm.v1.MergePullRequestRequest
	(*MergePullRequestResponse)(nil),     // 28: numinance.ports.scm.v1.MergePullRequestResponse
	(*SearchCodeRequest)(nil),            // 29: numinance.ports.scm.v1.SearchCodeRequest
	(*SearchCodeResponse)(nil),           // 30: numinance.ports.scm.v1.SearchCodeResponse
	(*timestamppb.Timestamp)(nil),        // 31: google.protobuf.Timestamp
}
var file_numinance_ports_scm_v1_scm_proto_depIdxs = []int32{
	6,  // 0: numinance.ports.scm.v1.GetTreeResponse.entries:type_name -> numinance.ports.scm.v1.TreeEntry
	11, // 1: numinance.ports.scm.v1.ListBranchesResponse.branches:type_name -> numinance.ports.scm.v1.Branch
	11, // 2: numinance.ports.scm.v1.CreateBranchResponse.branch:type_name -> numinance.ports.scm.v1.Branch
	0,  // 3: numinance.ports.scm.v1.PullRequest.state:type_name -> numinance.ports.scm.v1.PullRequestState
	18, // 4: numinance.ports.scm.v1.OpenPullRequestResponse.pull_request:type_name -> numinance.ports.scm.v1.PullRequest
	0,  // 5: numinance.ports.scm.v1.UpdatePullRequestRequest.state:type_name -> numinance.ports.scm.v1.PullRequestState
	18, // 6: numinance.ports.scm.v1.UpdatePullRequestResponse.pull_request:type_name -> numinance.ports.scm.v1.PullRequest
	2,  // 7: numinance.ports.scm.v1.CheckRun.conclusion:type_name -> numinance.ports.scm.v1.CheckConclusion
	18, // 8: numinance.ports.scm.v1.GetPullRequestStatusResponse.pull_request:type_name -> numinance.ports.scm.v1.PullRequest
	1,  // 9: numinance.ports.scm.v1.GetPullRequestStatusResponse.mergeability:type_name -> numinance.ports.scm.v1.Mergeability
	25, // 10: numinance.ports.scm.v1.GetPullRequestStatusResponse.checks:type_name -> numinance.ports.scm.v1.CheckRun
	31, // 11: numinance.ports.scm.v1.MergePullRequestResponse.merged_at:type_name -> google.protobuf.Timestamp
	3,  // 12: numinance.ports.scm.v1.ScmService.GetRepository:input_type -> numinance.ports.scm.v1.GetRepositoryRequest
	5,  // 13: numinance.ports.scm.v1.ScmService.GetTree:input_type -> numinance.ports.scm.v1.GetTreeRequest
	8,  // 14: numinance.ports.scm.v1.ScmService.GetFile:input_type -> numinance.ports.scm.v1.GetFileRequest
	10, // 15: numinance.ports.scm.v1.ScmService.ListBranches:input_type -> numinance.ports.scm.v1.ListBranchesRequest
	13, // 16: numinance.ports.scm.v1.ScmService.CreateBranch:input_type -> numinance.ports.scm.v1.CreateBranchRequest
	15, // 17: numinance.ports.scm.v1.ScmService.GetBranchProtection:input_type -> numinance.ports.scm.v1.GetBranchProtectionRequest
	17, // 18: numinance.ports.scm.v1.ScmService.OpenPullRequest:input_type -> numinance.ports.scm.v1.OpenPullRequestRequest
	20, // 19: numinance.ports.scm.v1.ScmService.UpdatePullRequest:input_type -> numinance.ports.scm.v1.UpdatePullRequestRequest
	22, // 20: numinance.ports.scm.v1.ScmService.CommentOnPullRequest:input_type -> numinance.ports.scm.v1.CommentOnPullRequestRequest
	24, // 21: numinance.ports.scm.v1.ScmService.GetPullRequestStatus:input_type -> numinance.ports.scm.v1.GetPullRequestStatusRequest
	27, // 22: numinance.ports.scm.v1.ScmService.MergePullRequest:input_type -> numinance.ports.scm.v1.MergePullRequestRequest
	29, // 23: numinance.ports.scm.v1.ScmService.SearchCode:input_type -> numinance.ports.scm.v1.SearchCodeRequest
	4,  // 24: numinance.ports.scm.v1.ScmService.GetRepository:output_type -> numinance.ports.scm.v1.GetRepositoryResponse
	7,  // 25: numinance.ports.scm.v1.ScmService.GetTree:output_type -> numinance.ports.scm.v1.GetTreeResponse
	9,  // 26: numinance.ports.scm.v1.ScmService.GetFile:output_type -> numinance.ports.scm.v1.GetFileResponse
	12, // 27: numinance.ports.scm.v1.ScmService.ListBranches:output_type -> numinance.ports.scm.v1.ListBranchesResponse
	14, // 28: numinance.ports.scm.v1.ScmService.CreateBranch:output_type -> numinance.ports.scm.v1.CreateBranchResponse
	16, // 29: numinance.ports.scm.v1.ScmService.GetBranchProtection:output_type -> numinance.ports.scm.v1.GetBranchProtectionResponse
	19, // 30: numinance.ports.scm.v1.ScmService.OpenPullRequest:output_type -> numinance.ports.scm.v1.OpenPullRequestResponse
	21, // 31: numinance.ports.scm.v1.ScmService.UpdatePullRequest:output_type -> numinance.ports.scm.v1.UpdatePullRequestResponse
	23, // 32: numinance.ports.scm.v1.ScmService.CommentOnPullRequest:output_type -> numinance.ports.scm.v1.CommentOnPullRequestResponse
	26, // 33: numinance.ports.scm.v1.ScmService.GetPullRequestStatus:output_type -> numinance.ports.scm.v1.GetPullRequestStatusResponse
	28, // 34: numinance.ports.scm.v1.ScmService.MergePullRequest:output_type -> numinance.ports.scm.v1.MergePullRequestResponse
	30, // 35: numinance.ports.scm.v1.ScmService.SearchCode:output_type -> numinance.ports.scm.v1.SearchCodeResponse
	24, // [24:36] is the sub-list for method output_type
	12, // [12:24] is the sub-list for method input_type
	12, // [12:12] is the sub-list for extension type_name
	12, // [12:12] is the sub-list for extension extendee
	0,  // [0:12] is the sub-list for field type_name
}

func init() { file_numinance_ports_scm_v1_scm_proto_init() }
func file_numinance_ports_scm_v1_scm_proto_init() {
	if File_numinance_ports_scm_v1_scm_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_scm_v1_scm_proto_rawDesc), len(file_numinance_ports_scm_v1_scm_proto_rawDesc)),
			NumEnums:      3,
			NumMessages:   28,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_scm_v1_scm_proto_goTypes,
		DependencyIndexes: file_numinance_ports_scm_v1_scm_proto_depIdxs,
		EnumInfos:         file_numinance_ports_scm_v1_scm_proto_enumTypes,
		MessageInfos:      file_numinance_ports_scm_v1_scm_proto_msgTypes,
	}.Build()
	File_numinance_ports_scm_v1_scm_proto = out.File
	file_numinance_ports_scm_v1_scm_proto_goTypes = nil
	file_numinance_ports_scm_v1_scm_proto_depIdxs = nil
}
