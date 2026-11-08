// The ticketing port: reads and searches issues, comments on an issue, moves it to a state category and links one issue to another as blocking it. Any tracker that has projects, issues with a key, a title, a text body, a workflow of states, comments and issue links can implement every core verb (Jira, GitHub Issues, Linear, GitLab Issues, Azure Boards); a vendor feature beyond them is a capability `ticketing.<feature>` negotiated at handshake, added in a later minor. The query languages of the trackers (JQL, WIQL) are the capability `ticketing.query_language`, never a core field. There is no VendorCall in 1.x, because the host refuses a provider that does not declare every side-effecting RPC of its port, and an escape hatch no provider implements is a PRIVILEGED capability that does nothing. Custom fields and vendor filters are capabilities or typed extensions that the Engine never reads.
// Callers: the change context calls GetIssue at intake through the plugin host and builds the ticket text of the Change from the validated fields; the Engine reconciles a retried write through GetIssue before it retries. The write verbs are reached only by a later write-back card and only under a policy decision, a redaction and a size cap. The host decides every call, on the exact request bytes, against the grant of the identity named in ActionContext. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: every request names its project in `project`, which the Engine marks as the resource of the call (RFC-0002): a call on a project outside the acting grant's resource scope is denied before dispatch. LinkIssue names two projects and marks both. A provider holds no credential of its own: it asks the host for a credential handle for the intent its manifest declares and sends the handle to the egress proxy, never a raw token. A provider never follows a redirect with an injected credential and never fetches a URL found in ticket content or in a response (a media link, a self link, a pagination link, an inline card).
// Untrusted answers: every title, body, type, label, operation id and key that the tracker holds is untrusted data, whoever wrote it on the tracker; anyone who can edit or comment on an issue can write any of them. The change context size-limits it, validates it against this file, stores it by digest and labels and delimits it as untrusted wherever it reaches a prompt, never as instructions. Nothing a provider answers changes policy, a grant, a risk class, an autonomy level, a tool list, a playbook choice or an approval: no field of an issue (label, type, priority, component, custom field) is mapped mechanically to any of them.
// Text hygiene: a provider returns plain text. It renders the tracker's rich text (ADF, wiki markup, HTML, Markdown) to plain text and strips from every string it returns the control characters (Unicode category Cc, except a line feed U+000A and a tab U+0009 inside a body), the bidirectional controls (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069) and the zero-width characters (U+200B to U+200D, U+2060, U+FEFF). The Engine repeats the strip, because the provider is the untrusted side, so a conforming provider returns none of them and a host never relies on that.
// Not found and no permission: a provider answers NOT_FOUND, with the same status and the same message, for an issue that does not exist, for an issue the credential may not see, and for an issue whose current project is not the project the request names (an issue that moved answers only under its new project). The answer never tells which of the three it is, so the port is not an oracle for the existence of issues. The message never contains the key or any text the tracker returned.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; NOT_FOUND as above; FAILED_PRECONDITION for a TransitionIssue whose target category no tracker transition reaches from the current state, and for a write the tracker refuses because of the issue's state; RESOURCE_EXHAUSTED when the tracker rate-limits the call; UNAVAILABLE for a tracker that cannot be reached or a transient failure the caller may retry. A provider never invents a result: an outage is an error, never an empty answer.
// Evidence: every write verb (CommentOnIssue, TransitionIssue, LinkIssue) declares one record type, ticketing-action. A provider emits exactly one per call that made the change or found it already made, through HostService.EmitEvidence: a compact receipt of the verb, the project, the issue key and the result (the comment id, the categories before and after, the other key of the link), never a title, a body or a comment text. A call that ends SUCCEEDED without it is PARTIAL. A dry run changes nothing and emits none. The read verbs declare no evidence.
// Reconciliation: the host's idempotency key names the operation, and a replay answers the first result. The Engine reconciles a write through GetIssue before it retries after a lost response. A provider that cannot key a write declares IDEMPOTENCY_NONE in its manifest, and a step that calls it needs a declared compensation or a human approval.
// Bounds: every string is at most 1024 bytes unless stated; over-limit input is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding). Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Formats: a project is the tracker's own spelling of a project: one or two segments separated by one '/', each matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$ (a Jira project key is one segment, a GitHub `<owner>/<repo>` is two). No segment is empty, `.` or `..`, no `..` appears anywhere, and there is no control, white space or non-ASCII character, so a provider that puts a project into a REST path cannot be made to leave it; LinkIssue applies the grammar to both of its projects. An issue key is the tracker's own spelling of an issue within a project, matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$: it never contains a '/', is at most 64 bytes and never has a '#', so that the Engine can build a ticket reference `<binding instance>:<issue key>` that always fits. The Engine builds that reference; it never takes one from a URL a provider returns.
// Every element is since 1.2.0: tag v0.2.0 shipped the 1.1.0 content, so 1.2.0 is the first unreleased spec minor.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/ticketing/v1/ticketing.proto

package ticketingv1

import (
	_ "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
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

// IssueStateCategory is the closed, vendor-neutral category of an issue's workflow state. A provider maps the tracker's own statuses to one of them; the name of the tracker's status is not part of the port.
type IssueStateCategory int32

const (
	// Zero value; rejected in a request and never returned.
	IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED IssueStateCategory = 0
	// Not started: backlog, to do, new.
	IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN IssueStateCategory = 1
	// Being worked on, including in review.
	IssueStateCategory_ISSUE_STATE_CATEGORY_IN_PROGRESS IssueStateCategory = 2
	// Finished as intended: done, resolved, closed as completed.
	IssueStateCategory_ISSUE_STATE_CATEGORY_DONE IssueStateCategory = 3
	// Finished without being done: canceled, won't do, duplicate.
	IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED IssueStateCategory = 4
)

// Enum value maps for IssueStateCategory.
var (
	IssueStateCategory_name = map[int32]string{
		0: "ISSUE_STATE_CATEGORY_UNSPECIFIED",
		1: "ISSUE_STATE_CATEGORY_OPEN",
		2: "ISSUE_STATE_CATEGORY_IN_PROGRESS",
		3: "ISSUE_STATE_CATEGORY_DONE",
		4: "ISSUE_STATE_CATEGORY_CANCELED",
	}
	IssueStateCategory_value = map[string]int32{
		"ISSUE_STATE_CATEGORY_UNSPECIFIED": 0,
		"ISSUE_STATE_CATEGORY_OPEN":        1,
		"ISSUE_STATE_CATEGORY_IN_PROGRESS": 2,
		"ISSUE_STATE_CATEGORY_DONE":        3,
		"ISSUE_STATE_CATEGORY_CANCELED":    4,
	}
)

func (x IssueStateCategory) Enum() *IssueStateCategory {
	p := new(IssueStateCategory)
	*p = x
	return p
}

func (x IssueStateCategory) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (IssueStateCategory) Descriptor() protoreflect.EnumDescriptor {
	return file_numinance_ports_ticketing_v1_ticketing_proto_enumTypes[0].Descriptor()
}

func (IssueStateCategory) Type() protoreflect.EnumType {
	return &file_numinance_ports_ticketing_v1_ticketing_proto_enumTypes[0]
}

func (x IssueStateCategory) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use IssueStateCategory.Descriptor instead.
func (IssueStateCategory) EnumDescriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{0}
}

// IssueLinkType is the closed type of an issue-to-issue link.
type IssueLinkType int32

const (
	// Zero value; rejected in a request and never returned.
	IssueLinkType_ISSUE_LINK_TYPE_UNSPECIFIED IssueLinkType = 0
	// The first issue blocks the second.
	IssueLinkType_ISSUE_LINK_TYPE_BLOCKS IssueLinkType = 1
)

// Enum value maps for IssueLinkType.
var (
	IssueLinkType_name = map[int32]string{
		0: "ISSUE_LINK_TYPE_UNSPECIFIED",
		1: "ISSUE_LINK_TYPE_BLOCKS",
	}
	IssueLinkType_value = map[string]int32{
		"ISSUE_LINK_TYPE_UNSPECIFIED": 0,
		"ISSUE_LINK_TYPE_BLOCKS":      1,
	}
)

func (x IssueLinkType) Enum() *IssueLinkType {
	p := new(IssueLinkType)
	*p = x
	return p
}

func (x IssueLinkType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (IssueLinkType) Descriptor() protoreflect.EnumDescriptor {
	return file_numinance_ports_ticketing_v1_ticketing_proto_enumTypes[1].Descriptor()
}

func (IssueLinkType) Type() protoreflect.EnumType {
	return &file_numinance_ports_ticketing_v1_ticketing_proto_enumTypes[1]
}

func (x IssueLinkType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use IssueLinkType.Descriptor instead.
func (IssueLinkType) EnumDescriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{1}
}

// GetIssueRequest names an issue.
type GetIssueRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the project, in the project grammar. Marked as the resource of the call.
	Project string `protobuf:"bytes,1,opt,name=project,proto3" json:"project,omitempty"`
	// Required: the issue key, in the issue key grammar.
	IssueKey string `protobuf:"bytes,2,opt,name=issue_key,json=issueKey,proto3" json:"issue_key,omitempty"`
	// Required, 1 to 65536: the most body bytes to return.
	MaxBodyBytes  uint32 `protobuf:"varint,3,opt,name=max_body_bytes,json=maxBodyBytes,proto3" json:"max_body_bytes,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetIssueRequest) Reset() {
	*x = GetIssueRequest{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetIssueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetIssueRequest) ProtoMessage() {}

func (x *GetIssueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetIssueRequest.ProtoReflect.Descriptor instead.
func (*GetIssueRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{0}
}

func (x *GetIssueRequest) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *GetIssueRequest) GetIssueKey() string {
	if x != nil {
		return x.IssueKey
	}
	return ""
}

func (x *GetIssueRequest) GetMaxBodyBytes() uint32 {
	if x != nil {
		return x.MaxBodyBytes
	}
	return 0
}

// GetIssueResponse is one issue.
type GetIssueResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The issue key, equal to the one requested.
	IssueKey string `protobuf:"bytes,1,opt,name=issue_key,json=issueKey,proto3" json:"issue_key,omitempty"`
	// The issue's project now, equal to the one requested.
	Project string `protobuf:"bytes,2,opt,name=project,proto3" json:"project,omitempty"`
	// The title as plain text, 1 to 1024 bytes, without a line feed or a tab; untrusted.
	Title string `protobuf:"bytes,3,opt,name=title,proto3" json:"title,omitempty"`
	// The description as plain text, at most max_body_bytes bytes, cut at a character boundary; untrusted.
	Body string `protobuf:"bytes,4,opt,name=body,proto3" json:"body,omitempty"`
	// True when the description is longer than max_body_bytes. A caller that needs the whole text refuses a truncated answer.
	Truncated bool `protobuf:"varint,5,opt,name=truncated,proto3" json:"truncated,omitempty"`
	// The state category of the issue now.
	State IssueStateCategory `protobuf:"varint,6,opt,name=state,proto3,enum=numinance.ports.ticketing.v1.IssueStateCategory" json:"state,omitempty"`
	// The tracker's issue type, for example Bug or Story, at most 128 printable bytes; untrusted. No decision rests on it.
	IssueType string `protobuf:"bytes,7,opt,name=issue_type,json=issueType,proto3" json:"issue_type,omitempty"`
	// The labels, at most 64, each 1 to 128 printable bytes; untrusted. No decision rests on them.
	Labels []string `protobuf:"bytes,8,rep,name=labels,proto3" json:"labels,omitempty"`
	// The operation ids recorded on the issue by earlier writes, at most 64, each at most 128 printable bytes; untrusted. Anyone who can edit or comment on the issue can forge one: the Engine accepts one only if it equals the operation_id it computed itself for that Change.
	OperationIds  []string `protobuf:"bytes,9,rep,name=operation_ids,json=operationIds,proto3" json:"operation_ids,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetIssueResponse) Reset() {
	*x = GetIssueResponse{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetIssueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetIssueResponse) ProtoMessage() {}

func (x *GetIssueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetIssueResponse.ProtoReflect.Descriptor instead.
func (*GetIssueResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{1}
}

func (x *GetIssueResponse) GetIssueKey() string {
	if x != nil {
		return x.IssueKey
	}
	return ""
}

func (x *GetIssueResponse) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *GetIssueResponse) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *GetIssueResponse) GetBody() string {
	if x != nil {
		return x.Body
	}
	return ""
}

func (x *GetIssueResponse) GetTruncated() bool {
	if x != nil {
		return x.Truncated
	}
	return false
}

func (x *GetIssueResponse) GetState() IssueStateCategory {
	if x != nil {
		return x.State
	}
	return IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED
}

func (x *GetIssueResponse) GetIssueType() string {
	if x != nil {
		return x.IssueType
	}
	return ""
}

func (x *GetIssueResponse) GetLabels() []string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *GetIssueResponse) GetOperationIds() []string {
	if x != nil {
		return x.OperationIds
	}
	return nil
}

// SearchIssuesRequest asks for issues of one project.
type SearchIssuesRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the project, in the project grammar. Marked as the resource of the call.
	Project string `protobuf:"bytes,1,opt,name=project,proto3" json:"project,omitempty"`
	// The text to find, at most 256 printable bytes, matched as plain text and never as a query; empty matches every issue of the project.
	Text string `protobuf:"bytes,2,opt,name=text,proto3" json:"text,omitempty"`
	// Only issues in this category; the unspecified value means any.
	State IssueStateCategory `protobuf:"varint,3,opt,name=state,proto3,enum=numinance.ports.ticketing.v1.IssueStateCategory" json:"state,omitempty"`
	// Required, 1 to 50: the most issues to return.
	MaxResults    uint32 `protobuf:"varint,4,opt,name=max_results,json=maxResults,proto3" json:"max_results,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SearchIssuesRequest) Reset() {
	*x = SearchIssuesRequest{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SearchIssuesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchIssuesRequest) ProtoMessage() {}

func (x *SearchIssuesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchIssuesRequest.ProtoReflect.Descriptor instead.
func (*SearchIssuesRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{2}
}

func (x *SearchIssuesRequest) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *SearchIssuesRequest) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

func (x *SearchIssuesRequest) GetState() IssueStateCategory {
	if x != nil {
		return x.State
	}
	return IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED
}

func (x *SearchIssuesRequest) GetMaxResults() uint32 {
	if x != nil {
		return x.MaxResults
	}
	return 0
}

// IssueSummary is one search hit.
type IssueSummary struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The issue key, in the issue key grammar.
	IssueKey string `protobuf:"bytes,1,opt,name=issue_key,json=issueKey,proto3" json:"issue_key,omitempty"`
	// The title as plain text, as in GetIssueResponse; untrusted.
	Title string `protobuf:"bytes,2,opt,name=title,proto3" json:"title,omitempty"`
	// The state category of the issue now.
	State         IssueStateCategory `protobuf:"varint,3,opt,name=state,proto3,enum=numinance.ports.ticketing.v1.IssueStateCategory" json:"state,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *IssueSummary) Reset() {
	*x = IssueSummary{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IssueSummary) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IssueSummary) ProtoMessage() {}

func (x *IssueSummary) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IssueSummary.ProtoReflect.Descriptor instead.
func (*IssueSummary) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{3}
}

func (x *IssueSummary) GetIssueKey() string {
	if x != nil {
		return x.IssueKey
	}
	return ""
}

func (x *IssueSummary) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *IssueSummary) GetState() IssueStateCategory {
	if x != nil {
		return x.State
	}
	return IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED
}

// SearchIssuesResponse is the hits of a search.
type SearchIssuesResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// At most max_results hits, all in the requested project and, when set, the requested state category.
	Issues []*IssueSummary `protobuf:"bytes,1,rep,name=issues,proto3" json:"issues,omitempty"`
	// True when the tracker holds more matches than were returned.
	Truncated     bool `protobuf:"varint,2,opt,name=truncated,proto3" json:"truncated,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SearchIssuesResponse) Reset() {
	*x = SearchIssuesResponse{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SearchIssuesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchIssuesResponse) ProtoMessage() {}

func (x *SearchIssuesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchIssuesResponse.ProtoReflect.Descriptor instead.
func (*SearchIssuesResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{4}
}

func (x *SearchIssuesResponse) GetIssues() []*IssueSummary {
	if x != nil {
		return x.Issues
	}
	return nil
}

func (x *SearchIssuesResponse) GetTruncated() bool {
	if x != nil {
		return x.Truncated
	}
	return false
}

// CommentOnIssueRequest is one comment.
type CommentOnIssueRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the project, in the project grammar. Marked as the resource of the call.
	Project string `protobuf:"bytes,1,opt,name=project,proto3" json:"project,omitempty"`
	// Required: the issue key, in the issue key grammar.
	IssueKey string `protobuf:"bytes,2,opt,name=issue_key,json=issueKey,proto3" json:"issue_key,omitempty"`
	// Required, 1 to 8192 bytes of plain text; a provider never interprets it as markup.
	Body          string `protobuf:"bytes,3,opt,name=body,proto3" json:"body,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CommentOnIssueRequest) Reset() {
	*x = CommentOnIssueRequest{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CommentOnIssueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CommentOnIssueRequest) ProtoMessage() {}

func (x *CommentOnIssueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CommentOnIssueRequest.ProtoReflect.Descriptor instead.
func (*CommentOnIssueRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{5}
}

func (x *CommentOnIssueRequest) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *CommentOnIssueRequest) GetIssueKey() string {
	if x != nil {
		return x.IssueKey
	}
	return ""
}

func (x *CommentOnIssueRequest) GetBody() string {
	if x != nil {
		return x.Body
	}
	return ""
}

// CommentOnIssueResponse names the comment.
type CommentOnIssueResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The tracker's id of the comment, 1 to 128 printable bytes; the same id for a replay under the same key.
	CommentId string `protobuf:"bytes,1,opt,name=comment_id,json=commentId,proto3" json:"comment_id,omitempty"`
	// True when the comment already existed under this key and nothing was added.
	Existing      bool `protobuf:"varint,2,opt,name=existing,proto3" json:"existing,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CommentOnIssueResponse) Reset() {
	*x = CommentOnIssueResponse{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CommentOnIssueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CommentOnIssueResponse) ProtoMessage() {}

func (x *CommentOnIssueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CommentOnIssueResponse.ProtoReflect.Descriptor instead.
func (*CommentOnIssueResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{6}
}

func (x *CommentOnIssueResponse) GetCommentId() string {
	if x != nil {
		return x.CommentId
	}
	return ""
}

func (x *CommentOnIssueResponse) GetExisting() bool {
	if x != nil {
		return x.Existing
	}
	return false
}

// TransitionIssueRequest asks for a state category.
type TransitionIssueRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the project, in the project grammar. Marked as the resource of the call.
	Project string `protobuf:"bytes,1,opt,name=project,proto3" json:"project,omitempty"`
	// Required: the issue key, in the issue key grammar.
	IssueKey string `protobuf:"bytes,2,opt,name=issue_key,json=issueKey,proto3" json:"issue_key,omitempty"`
	// Required: the category to move the issue to.
	Target        IssueStateCategory `protobuf:"varint,3,opt,name=target,proto3,enum=numinance.ports.ticketing.v1.IssueStateCategory" json:"target,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TransitionIssueRequest) Reset() {
	*x = TransitionIssueRequest{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TransitionIssueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransitionIssueRequest) ProtoMessage() {}

func (x *TransitionIssueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransitionIssueRequest.ProtoReflect.Descriptor instead.
func (*TransitionIssueRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{7}
}

func (x *TransitionIssueRequest) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *TransitionIssueRequest) GetIssueKey() string {
	if x != nil {
		return x.IssueKey
	}
	return ""
}

func (x *TransitionIssueRequest) GetTarget() IssueStateCategory {
	if x != nil {
		return x.Target
	}
	return IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED
}

// TransitionIssueResponse reports the move.
type TransitionIssueResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The category before the call: the one a compensating TransitionIssue returns to.
	Previous IssueStateCategory `protobuf:"varint,1,opt,name=previous,proto3,enum=numinance.ports.ticketing.v1.IssueStateCategory" json:"previous,omitempty"`
	// The category after the call; on a dry run the category the call would leave, with nothing changed.
	State IssueStateCategory `protobuf:"varint,2,opt,name=state,proto3,enum=numinance.ports.ticketing.v1.IssueStateCategory" json:"state,omitempty"`
	// True when the call changed the issue (on a dry run: would change it); false when the issue was already in the target category.
	Changed       bool `protobuf:"varint,3,opt,name=changed,proto3" json:"changed,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TransitionIssueResponse) Reset() {
	*x = TransitionIssueResponse{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TransitionIssueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransitionIssueResponse) ProtoMessage() {}

func (x *TransitionIssueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransitionIssueResponse.ProtoReflect.Descriptor instead.
func (*TransitionIssueResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{8}
}

func (x *TransitionIssueResponse) GetPrevious() IssueStateCategory {
	if x != nil {
		return x.Previous
	}
	return IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED
}

func (x *TransitionIssueResponse) GetState() IssueStateCategory {
	if x != nil {
		return x.State
	}
	return IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED
}

func (x *TransitionIssueResponse) GetChanged() bool {
	if x != nil {
		return x.Changed
	}
	return false
}

// LinkIssueRequest names the blocking issue and the blocked issue.
type LinkIssueRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the project of the first issue. Marked as the resource of the call.
	Project string `protobuf:"bytes,1,opt,name=project,proto3" json:"project,omitempty"`
	// Required: the key of the issue that blocks.
	IssueKey string `protobuf:"bytes,2,opt,name=issue_key,json=issueKey,proto3" json:"issue_key,omitempty"`
	// Required: the project of the second issue. Marked as the resource of the call.
	BlockedProject string `protobuf:"bytes,3,opt,name=blocked_project,json=blockedProject,proto3" json:"blocked_project,omitempty"`
	// Required: the key of the issue that is blocked; not the same issue as issue_key.
	BlockedIssueKey string `protobuf:"bytes,4,opt,name=blocked_issue_key,json=blockedIssueKey,proto3" json:"blocked_issue_key,omitempty"`
	// Required: the type of the link.
	LinkType      IssueLinkType `protobuf:"varint,5,opt,name=link_type,json=linkType,proto3,enum=numinance.ports.ticketing.v1.IssueLinkType" json:"link_type,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LinkIssueRequest) Reset() {
	*x = LinkIssueRequest{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LinkIssueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LinkIssueRequest) ProtoMessage() {}

func (x *LinkIssueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LinkIssueRequest.ProtoReflect.Descriptor instead.
func (*LinkIssueRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{9}
}

func (x *LinkIssueRequest) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *LinkIssueRequest) GetIssueKey() string {
	if x != nil {
		return x.IssueKey
	}
	return ""
}

func (x *LinkIssueRequest) GetBlockedProject() string {
	if x != nil {
		return x.BlockedProject
	}
	return ""
}

func (x *LinkIssueRequest) GetBlockedIssueKey() string {
	if x != nil {
		return x.BlockedIssueKey
	}
	return ""
}

func (x *LinkIssueRequest) GetLinkType() IssueLinkType {
	if x != nil {
		return x.LinkType
	}
	return IssueLinkType_ISSUE_LINK_TYPE_UNSPECIFIED
}

// LinkIssueResponse reports the link.
type LinkIssueResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// True when the link already existed and nothing was added.
	Existing      bool `protobuf:"varint,1,opt,name=existing,proto3" json:"existing,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LinkIssueResponse) Reset() {
	*x = LinkIssueResponse{}
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LinkIssueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LinkIssueResponse) ProtoMessage() {}

func (x *LinkIssueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LinkIssueResponse.ProtoReflect.Descriptor instead.
func (*LinkIssueResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP(), []int{10}
}

func (x *LinkIssueResponse) GetExisting() bool {
	if x != nil {
		return x.Existing
	}
	return false
}

var File_numinance_ports_ticketing_v1_ticketing_proto protoreflect.FileDescriptor

const file_numinance_ports_ticketing_v1_ticketing_proto_rawDesc = "" +
	"\n" +
	",numinance/ports/ticketing/v1/ticketing.proto\x12\x1cnuminance.ports.ticketing.v1\x1a*numinance/annotations/v1/annotations.proto\"\x97\x01\n" +
	"\x0fGetIssueRequest\x12'\n" +
	"\aproject\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.2.00\x01R\aproject\x12(\n" +
	"\tissue_key\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bissueKey\x121\n" +
	"\x0emax_body_bytes\x18\x03 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\fmaxBodyBytes\"\xaa\x03\n" +
	"\x10GetIssueResponse\x12(\n" +
	"\tissue_key\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bissueKey\x12%\n" +
	"\aproject\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\aproject\x12!\n" +
	"\x05title\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x05title\x12\x1f\n" +
	"\x04body\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x04body\x12)\n" +
	"\ttruncated\x18\x05 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\ttruncated\x12S\n" +
	"\x05state\x18\x06 \x01(\x0e20.numinance.ports.ticketing.v1.IssueStateCategoryB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x05state\x12*\n" +
	"\n" +
	"issue_type\x18\a \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\tissueType\x12#\n" +
	"\x06labels\x18\b \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x06labels\x120\n" +
	"\roperation_ids\x18\t \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\foperationIds\"\xe2\x01\n" +
	"\x13SearchIssuesRequest\x12'\n" +
	"\aproject\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.2.00\x01R\aproject\x12\x1f\n" +
	"\x04text\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x04text\x12S\n" +
	"\x05state\x18\x03 \x01(\x0e20.numinance.ports.ticketing.v1.IssueStateCategoryB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x05state\x12,\n" +
	"\vmax_results\x18\x04 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\n" +
	"maxResults\"\xb0\x01\n" +
	"\fIssueSummary\x12(\n" +
	"\tissue_key\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bissueKey\x12!\n" +
	"\x05title\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x05title\x12S\n" +
	"\x05state\x18\x03 \x01(\x0e20.numinance.ports.ticketing.v1.IssueStateCategoryB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x05state\"\x92\x01\n" +
	"\x14SearchIssuesResponse\x12O\n" +
	"\x06issues\x18\x01 \x03(\v2*.numinance.ports.ticketing.v1.IssueSummaryB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x06issues\x12)\n" +
	"\ttruncated\x18\x02 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\ttruncated\"\x8b\x01\n" +
	"\x15CommentOnIssueRequest\x12'\n" +
	"\aproject\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.2.00\x01R\aproject\x12(\n" +
	"\tissue_key\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bissueKey\x12\x1f\n" +
	"\x04body\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x04body\"m\n" +
	"\x16CommentOnIssueResponse\x12*\n" +
	"\n" +
	"comment_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\tcommentId\x12'\n" +
	"\bexisting\x18\x02 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bexisting\"\xc2\x01\n" +
	"\x16TransitionIssueRequest\x12'\n" +
	"\aproject\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.2.00\x01R\aproject\x12(\n" +
	"\tissue_key\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bissueKey\x12U\n" +
	"\x06target\x18\x03 \x01(\x0e20.numinance.ports.ticketing.v1.IssueStateCategoryB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x06target\"\xf0\x01\n" +
	"\x17TransitionIssueResponse\x12Y\n" +
	"\bprevious\x18\x01 \x01(\x0e20.numinance.ports.ticketing.v1.IssueStateCategoryB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bprevious\x12S\n" +
	"\x05state\x18\x02 \x01(\x0e20.numinance.ports.ticketing.v1.IssueStateCategoryB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x05state\x12%\n" +
	"\achanged\x18\x03 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\achanged\"\xad\x02\n" +
	"\x10LinkIssueRequest\x12'\n" +
	"\aproject\x18\x01 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.2.00\x01R\aproject\x12(\n" +
	"\tissue_key\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bissueKey\x126\n" +
	"\x0fblocked_project\x18\x03 \x01(\tB\r\x92\xb5\x18\t\n" +
	"\x051.2.00\x01R\x0eblockedProject\x127\n" +
	"\x11blocked_issue_key\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\x0fblockedIssueKey\x12U\n" +
	"\tlink_type\x18\x05 \x01(\x0e2+.numinance.ports.ticketing.v1.IssueLinkTypeB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\blinkType\"<\n" +
	"\x11LinkIssueResponse\x12'\n" +
	"\bexisting\x18\x01 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.2.0R\bexisting*\xc1\x01\n" +
	"\x12IssueStateCategory\x12$\n" +
	" ISSUE_STATE_CATEGORY_UNSPECIFIED\x10\x00\x12\x1d\n" +
	"\x19ISSUE_STATE_CATEGORY_OPEN\x10\x01\x12$\n" +
	" ISSUE_STATE_CATEGORY_IN_PROGRESS\x10\x02\x12\x1d\n" +
	"\x19ISSUE_STATE_CATEGORY_DONE\x10\x03\x12!\n" +
	"\x1dISSUE_STATE_CATEGORY_CANCELED\x10\x04*L\n" +
	"\rIssueLinkType\x12\x1f\n" +
	"\x1bISSUE_LINK_TYPE_UNSPECIFIED\x10\x00\x12\x1a\n" +
	"\x16ISSUE_LINK_TYPE_BLOCKS\x10\x012\x89\t\n" +
	"\x10TicketingService\x12\xf5\x01\n" +
	"\bGetIssue\x12-.numinance.ports.ticketing.v1.GetIssueRequest\x1a..numinance.ports.ticketing.v1.GetIssueResponse\"\x89\x01\x8a\xb5\x18\x84\x01\b\x01\x10\x01\x18\x01B\x051.2.0JuReturns one issue of a project: key, title, plain-text body, state category, type, labels and recorded operation ids.\x12\xd8\x01\n" +
	"\fSearchIssues\x121.numinance.ports.ticketing.v1.SearchIssuesRequest\x1a2.numinance.ports.ticketing.v1.SearchIssuesResponse\"a\x8a\xb5\x18]\b\x01\x10\x01\x18\x01B\x051.2.0JNSearches the issues of a project for a text, optionally in one state category.\x12\xca\x01\n" +
	"\x0eCommentOnIssue\x123.numinance.ports.ticketing.v1.CommentOnIssueRequest\x1a4.numinance.ports.ticketing.v1.CommentOnIssueResponse\"M\x8a\xb5\x18I\b\x02\x10\x01\x18\x012\x10ticketing-actionB\x051.2.0J(Adds one plain-text comment to an issue.\x12\x9a\x02\n" +
	"\x0fTransitionIssue\x124.numinance.ports.ticketing.v1.TransitionIssueRequest\x1a5.numinance.ports.ticketing.v1.TransitionIssueResponse\"\x99\x01\x8a\xb5\x18\x94\x01\b\x03\x10\x01\x18\x01 \x01* TicketingService.TransitionIssue2\x10ticketing-actionB\x051.2.0JOMoves an issue to a target state category: open, in progress, done or canceled.\x12\xb7\x01\n" +
	"\tLinkIssue\x12..numinance.ports.ticketing.v1.LinkIssueRequest\x1a/.numinance.ports.ticketing.v1.LinkIssueResponse\"I\x8a\xb5\x18E\b\x02\x10\x01\x18\x012\x10ticketing-actionB\x051.2.0J$Links one issue as blocking another.B\x9e\x02\n" +
	" com.numinance.ports.ticketing.v1B\x0eTicketingProtoP\x01ZWgithub.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1;ticketingv1\xa2\x02\x03NPT\xaa\x02\x1cNuminance.Ports.Ticketing.V1\xca\x02\x1cNuminance\\Ports\\Ticketing\\V1\xe2\x02(Numinance\\Ports\\Ticketing\\V1\\GPBMetadata\xea\x02\x1fNuminance::Ports::Ticketing::V1b\x06proto3"

var (
	file_numinance_ports_ticketing_v1_ticketing_proto_rawDescOnce sync.Once
	file_numinance_ports_ticketing_v1_ticketing_proto_rawDescData []byte
)

func file_numinance_ports_ticketing_v1_ticketing_proto_rawDescGZIP() []byte {
	file_numinance_ports_ticketing_v1_ticketing_proto_rawDescOnce.Do(func() {
		file_numinance_ports_ticketing_v1_ticketing_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_ticketing_v1_ticketing_proto_rawDesc), len(file_numinance_ports_ticketing_v1_ticketing_proto_rawDesc)))
	})
	return file_numinance_ports_ticketing_v1_ticketing_proto_rawDescData
}

var file_numinance_ports_ticketing_v1_ticketing_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_numinance_ports_ticketing_v1_ticketing_proto_goTypes = []any{
	(IssueStateCategory)(0),         // 0: numinance.ports.ticketing.v1.IssueStateCategory
	(IssueLinkType)(0),              // 1: numinance.ports.ticketing.v1.IssueLinkType
	(*GetIssueRequest)(nil),         // 2: numinance.ports.ticketing.v1.GetIssueRequest
	(*GetIssueResponse)(nil),        // 3: numinance.ports.ticketing.v1.GetIssueResponse
	(*SearchIssuesRequest)(nil),     // 4: numinance.ports.ticketing.v1.SearchIssuesRequest
	(*IssueSummary)(nil),            // 5: numinance.ports.ticketing.v1.IssueSummary
	(*SearchIssuesResponse)(nil),    // 6: numinance.ports.ticketing.v1.SearchIssuesResponse
	(*CommentOnIssueRequest)(nil),   // 7: numinance.ports.ticketing.v1.CommentOnIssueRequest
	(*CommentOnIssueResponse)(nil),  // 8: numinance.ports.ticketing.v1.CommentOnIssueResponse
	(*TransitionIssueRequest)(nil),  // 9: numinance.ports.ticketing.v1.TransitionIssueRequest
	(*TransitionIssueResponse)(nil), // 10: numinance.ports.ticketing.v1.TransitionIssueResponse
	(*LinkIssueRequest)(nil),        // 11: numinance.ports.ticketing.v1.LinkIssueRequest
	(*LinkIssueResponse)(nil),       // 12: numinance.ports.ticketing.v1.LinkIssueResponse
}
var file_numinance_ports_ticketing_v1_ticketing_proto_depIdxs = []int32{
	0,  // 0: numinance.ports.ticketing.v1.GetIssueResponse.state:type_name -> numinance.ports.ticketing.v1.IssueStateCategory
	0,  // 1: numinance.ports.ticketing.v1.SearchIssuesRequest.state:type_name -> numinance.ports.ticketing.v1.IssueStateCategory
	0,  // 2: numinance.ports.ticketing.v1.IssueSummary.state:type_name -> numinance.ports.ticketing.v1.IssueStateCategory
	5,  // 3: numinance.ports.ticketing.v1.SearchIssuesResponse.issues:type_name -> numinance.ports.ticketing.v1.IssueSummary
	0,  // 4: numinance.ports.ticketing.v1.TransitionIssueRequest.target:type_name -> numinance.ports.ticketing.v1.IssueStateCategory
	0,  // 5: numinance.ports.ticketing.v1.TransitionIssueResponse.previous:type_name -> numinance.ports.ticketing.v1.IssueStateCategory
	0,  // 6: numinance.ports.ticketing.v1.TransitionIssueResponse.state:type_name -> numinance.ports.ticketing.v1.IssueStateCategory
	1,  // 7: numinance.ports.ticketing.v1.LinkIssueRequest.link_type:type_name -> numinance.ports.ticketing.v1.IssueLinkType
	2,  // 8: numinance.ports.ticketing.v1.TicketingService.GetIssue:input_type -> numinance.ports.ticketing.v1.GetIssueRequest
	4,  // 9: numinance.ports.ticketing.v1.TicketingService.SearchIssues:input_type -> numinance.ports.ticketing.v1.SearchIssuesRequest
	7,  // 10: numinance.ports.ticketing.v1.TicketingService.CommentOnIssue:input_type -> numinance.ports.ticketing.v1.CommentOnIssueRequest
	9,  // 11: numinance.ports.ticketing.v1.TicketingService.TransitionIssue:input_type -> numinance.ports.ticketing.v1.TransitionIssueRequest
	11, // 12: numinance.ports.ticketing.v1.TicketingService.LinkIssue:input_type -> numinance.ports.ticketing.v1.LinkIssueRequest
	3,  // 13: numinance.ports.ticketing.v1.TicketingService.GetIssue:output_type -> numinance.ports.ticketing.v1.GetIssueResponse
	6,  // 14: numinance.ports.ticketing.v1.TicketingService.SearchIssues:output_type -> numinance.ports.ticketing.v1.SearchIssuesResponse
	8,  // 15: numinance.ports.ticketing.v1.TicketingService.CommentOnIssue:output_type -> numinance.ports.ticketing.v1.CommentOnIssueResponse
	10, // 16: numinance.ports.ticketing.v1.TicketingService.TransitionIssue:output_type -> numinance.ports.ticketing.v1.TransitionIssueResponse
	12, // 17: numinance.ports.ticketing.v1.TicketingService.LinkIssue:output_type -> numinance.ports.ticketing.v1.LinkIssueResponse
	13, // [13:18] is the sub-list for method output_type
	8,  // [8:13] is the sub-list for method input_type
	8,  // [8:8] is the sub-list for extension type_name
	8,  // [8:8] is the sub-list for extension extendee
	0,  // [0:8] is the sub-list for field type_name
}

func init() { file_numinance_ports_ticketing_v1_ticketing_proto_init() }
func file_numinance_ports_ticketing_v1_ticketing_proto_init() {
	if File_numinance_ports_ticketing_v1_ticketing_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_ticketing_v1_ticketing_proto_rawDesc), len(file_numinance_ports_ticketing_v1_ticketing_proto_rawDesc)),
			NumEnums:      2,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_ticketing_v1_ticketing_proto_goTypes,
		DependencyIndexes: file_numinance_ports_ticketing_v1_ticketing_proto_depIdxs,
		EnumInfos:         file_numinance_ports_ticketing_v1_ticketing_proto_enumTypes,
		MessageInfos:      file_numinance_ports_ticketing_v1_ticketing_proto_msgTypes,
	}.Build()
	File_numinance_ports_ticketing_v1_ticketing_proto = out.File
	file_numinance_ports_ticketing_v1_ticketing_proto_goTypes = nil
	file_numinance_ports_ticketing_v1_ticketing_proto_depIdxs = nil
}
