// The ticketing port: reads and searches issues, comments on an issue, moves it to a state category and links one issue to another as blocking it. Any tracker that has projects, issues with a key, a title, a text body, a workflow of states, comments and issue links can implement every core verb (Jira, GitHub Issues, Linear, GitLab Issues, Azure Boards); a vendor feature beyond them is a capability `ticketing.<feature>` negotiated at handshake, added in a later minor. The query languages of the trackers (JQL, WIQL) are the capability `ticketing.query_language`, never a core field. There is no VendorCall in 1.x, because the host refuses a provider that does not declare every side-effecting RPC of its port, and an escape hatch no provider implements is a PRIVILEGED capability that does nothing. Custom fields and vendor filters are capabilities or typed extensions that the Engine never reads.
// Callers: the change context calls GetIssue at intake through the plugin host and builds the ticket text of the Change from the validated fields; the Engine reconciles a retried write through GetIssue before it retries. The write verbs are reached only by a later write-back card and only under a policy decision, a redaction and a size cap. The host decides every call, on the exact request bytes, against the grant of the identity named in ActionContext. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: every request names its project in `project`, which the Engine marks as the resource of the call (RFC-0002): a call on a project outside the acting grant's resource scope is denied before dispatch. LinkIssue names two projects and marks both. A provider holds no credential of its own: it asks the host for a credential handle for the intent its manifest declares and sends the handle to the egress proxy, never a raw token. A provider never follows a redirect with an injected credential and never fetches a URL found in ticket content or in a response (a media link, a self link, a pagination link, an inline card).
// Untrusted answers: every title, body, type, label, operation id and key that the tracker holds is untrusted data, whoever wrote it on the tracker; anyone who can edit or comment on an issue can write any of them. The change context size-limits it, validates it against this file, stores it by digest and labels and delimits it as untrusted wherever it reaches a prompt, never as instructions. Nothing a provider answers changes policy, a grant, a risk class, an autonomy level, a tool list, a playbook choice or an approval: no field of an issue (label, type, priority, component, custom field) is mapped mechanically to any of them.
// Text hygiene: a provider returns plain text. It renders the tracker's rich text (ADF, wiki markup, HTML, Markdown) to plain text and strips from every string it returns the control characters (Unicode category Cc, except a line feed U+000A and a tab U+0009 inside a body), the bidirectional controls (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069) and the zero-width characters (U+200B to U+200D, U+2060, U+FEFF). The Engine repeats the strip, because the provider is the untrusted side, so a conforming provider returns none of them and a host never relies on that.
// Not found and no permission: a provider answers NOT_FOUND, with the same status and the same message, for an issue that does not exist, for an issue the credential may not see, and for an issue whose current project is not the project the request names (an issue that moved answers only under its new project). The answer never tells which of the three it is, so the port is not an oracle for the existence of issues. The message never contains the key or any text the tracker returned.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; NOT_FOUND as above; FAILED_PRECONDITION for a TransitionIssue whose target category no tracker transition reaches from the current state, and for a write the tracker refuses because of the issue's state; RESOURCE_EXHAUSTED when the tracker rate-limits the call; UNAVAILABLE for a tracker that cannot be reached or a transient failure the caller may retry. A provider never invents a result: an outage is an error, never an empty answer. No error message of any class (INVALID_ARGUMENT, NOT_FOUND, FAILED_PRECONDITION, RESOURCE_EXHAUSTED, UNAVAILABLE) contains the key, the project, a title, the offending value in any encoding or any other text the tracker returned: a provider answers one fixed message per class (or per violated bound).
// Evidence: every write verb (CommentOnIssue, TransitionIssue, LinkIssue) declares one record type, ticketing-action. A provider emits exactly one per call that made the change or found it already made, through HostService.EmitEvidence: a compact receipt of the verb, the project, the issue key and the result (the comment id, the categories before and after, the other key of the link), never a title, a body or a comment text. A call that ends SUCCEEDED without it is PARTIAL. A dry run changes nothing and emits none. The read verbs declare no evidence.
// Reconciliation: the host's idempotency key names the operation, and a replay answers the first result. The Engine reconciles a write through GetIssue before it retries after a lost response. A provider that cannot key a write declares IDEMPOTENCY_NONE in its manifest, and a step that calls it needs a declared compensation or a human approval.
// Bounds: every string is at most 1024 bytes unless stated; over-limit input is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding). Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Formats: a project is the tracker's own spelling of a project: one or two segments separated by one '/', each matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$ (a Jira project key is one segment, a GitHub `<owner>/<repo>` is two). No segment is empty, `.` or `..`, no `..` appears anywhere, and there is no control, white space or non-ASCII character, so a provider that puts a project into a REST path cannot be made to leave it; LinkIssue applies the grammar to both of its projects. An issue key is the tracker's own spelling of an issue within a project, matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$: it never contains a '/', is at most 64 bytes and never has a '#', so that the Engine can build a ticket reference `<binding instance>:<issue key>` that always fits. The Engine builds that reference; it never takes one from a URL a provider returns.
// Every element is since 1.2.0: tag v0.2.0 shipped the 1.1.0 content, so 1.2.0 is the first unreleased spec minor.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.6.2
// - protoc             (unknown)
// source: numinance/ports/ticketing/v1/ticketing.proto

package ticketingv1

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
	TicketingService_GetIssue_FullMethodName        = "/numinance.ports.ticketing.v1.TicketingService/GetIssue"
	TicketingService_SearchIssues_FullMethodName    = "/numinance.ports.ticketing.v1.TicketingService/SearchIssues"
	TicketingService_CommentOnIssue_FullMethodName  = "/numinance.ports.ticketing.v1.TicketingService/CommentOnIssue"
	TicketingService_TransitionIssue_FullMethodName = "/numinance.ports.ticketing.v1.TicketingService/TransitionIssue"
	TicketingService_LinkIssue_FullMethodName       = "/numinance.ports.ticketing.v1.TicketingService/LinkIssue"
)

// TicketingServiceClient is the client API for TicketingService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// TicketingService is the ticketing port an issue-tracker provider serves.
type TicketingServiceClient interface {
	// GetIssue returns one issue: its key, its title, its text body bounded by max_body_bytes with a truncated flag, its state category, its type and labels, and the operation ids recorded on it. It is also the reconciliation read of every write after a lost response. The project in the request is the resource of the call, and the issue must be in that project now: an issue that moved to another project is NOT_FOUND here. Every text field of the answer is untrusted. The operation ids are whatever anyone who could edit or comment on the issue wrote there: the Engine accepts one only if it equals the operation_id it computed itself for that Change, and trusts no other.
	GetIssue(ctx context.Context, in *GetIssueRequest, opts ...grpc.CallOption) (*GetIssueResponse, error)
	// SearchIssues lists the issues of one project whose title or body contains the text, optionally in one state category, at most max_results of them, in the tracker's order. The text is a parameter: a provider matches it as plain text and never splices it into a query string or a query language, so text that looks like a JQL clause matches only an issue that contains those characters. Query languages are the capability ticketing.query_language, not this verb. Results are untrusted data. A project the credential may not use and a project that does not exist are one answer: NOT_FOUND with the one fixed message of the not-found paragraph, never PERMISSION_DENIED, so a search is not an oracle for the existence of a project.
	SearchIssues(ctx context.Context, in *SearchIssuesRequest, opts ...grpc.CallOption) (*SearchIssuesResponse, error)
	// CommentOnIssue adds one plain-text comment, identified by the host's key: a replay answers the first comment and adds none (the provider finds its earlier comment by a marker derived from the key). LOW: a comment changes no workflow and no code, though everyone who can see the issue can read it, so the caller must redact and size-cap the text and consider the audience before it calls; the risk class does not carry that. No dry run: the comment is the whole effect, and a dry run would be the call. No compensating action: this port has no verb that removes a comment, and a correction is a further comment.
	CommentOnIssue(ctx context.Context, in *CommentOnIssueRequest, opts ...grpc.CallOption) (*CommentOnIssueResponse, error)
	// TransitionIssue moves an issue to a target state category (OPEN, IN_PROGRESS, DONE or CANCELED). The provider chooses the tracker's own transition that reaches the category; when none does from the current state, the answer is FAILED_PRECONDITION and nothing changes. An issue already in the target category answers done (changed false). MEDIUM: a transition fires the customer's workflow automations, which can notify people, start builds or close related work. Dry run checks that a transition to the target category is available and changes nothing; its answer is the one the call would give. The response carries the category before the call, and the compensating action is a TransitionIssue back to that category.
	TransitionIssue(ctx context.Context, in *TransitionIssueRequest, opts ...grpc.CallOption) (*TransitionIssueResponse, error)
	// LinkIssue makes an issue-to-issue link of type BLOCKS: the issue named first blocks the issue named second. An existing link of the same type between the same two issues answers done (existing true). LOW: a link changes no workflow state and no text. No dry run: the link is the whole effect and its creation is the check of both keys. No compensating action: this port has no verb that removes a link. Other link types, parent and child links and remote URL links are later additive enum values or capabilities.
	LinkIssue(ctx context.Context, in *LinkIssueRequest, opts ...grpc.CallOption) (*LinkIssueResponse, error)
}

type ticketingServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTicketingServiceClient(cc grpc.ClientConnInterface) TicketingServiceClient {
	return &ticketingServiceClient{cc}
}

func (c *ticketingServiceClient) GetIssue(ctx context.Context, in *GetIssueRequest, opts ...grpc.CallOption) (*GetIssueResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetIssueResponse)
	err := c.cc.Invoke(ctx, TicketingService_GetIssue_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ticketingServiceClient) SearchIssues(ctx context.Context, in *SearchIssuesRequest, opts ...grpc.CallOption) (*SearchIssuesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SearchIssuesResponse)
	err := c.cc.Invoke(ctx, TicketingService_SearchIssues_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ticketingServiceClient) CommentOnIssue(ctx context.Context, in *CommentOnIssueRequest, opts ...grpc.CallOption) (*CommentOnIssueResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CommentOnIssueResponse)
	err := c.cc.Invoke(ctx, TicketingService_CommentOnIssue_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ticketingServiceClient) TransitionIssue(ctx context.Context, in *TransitionIssueRequest, opts ...grpc.CallOption) (*TransitionIssueResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TransitionIssueResponse)
	err := c.cc.Invoke(ctx, TicketingService_TransitionIssue_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ticketingServiceClient) LinkIssue(ctx context.Context, in *LinkIssueRequest, opts ...grpc.CallOption) (*LinkIssueResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LinkIssueResponse)
	err := c.cc.Invoke(ctx, TicketingService_LinkIssue_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TicketingServiceServer is the server API for TicketingService service.
// All implementations must embed UnimplementedTicketingServiceServer
// for forward compatibility.
//
// TicketingService is the ticketing port an issue-tracker provider serves.
type TicketingServiceServer interface {
	// GetIssue returns one issue: its key, its title, its text body bounded by max_body_bytes with a truncated flag, its state category, its type and labels, and the operation ids recorded on it. It is also the reconciliation read of every write after a lost response. The project in the request is the resource of the call, and the issue must be in that project now: an issue that moved to another project is NOT_FOUND here. Every text field of the answer is untrusted. The operation ids are whatever anyone who could edit or comment on the issue wrote there: the Engine accepts one only if it equals the operation_id it computed itself for that Change, and trusts no other.
	GetIssue(context.Context, *GetIssueRequest) (*GetIssueResponse, error)
	// SearchIssues lists the issues of one project whose title or body contains the text, optionally in one state category, at most max_results of them, in the tracker's order. The text is a parameter: a provider matches it as plain text and never splices it into a query string or a query language, so text that looks like a JQL clause matches only an issue that contains those characters. Query languages are the capability ticketing.query_language, not this verb. Results are untrusted data. A project the credential may not use and a project that does not exist are one answer: NOT_FOUND with the one fixed message of the not-found paragraph, never PERMISSION_DENIED, so a search is not an oracle for the existence of a project.
	SearchIssues(context.Context, *SearchIssuesRequest) (*SearchIssuesResponse, error)
	// CommentOnIssue adds one plain-text comment, identified by the host's key: a replay answers the first comment and adds none (the provider finds its earlier comment by a marker derived from the key). LOW: a comment changes no workflow and no code, though everyone who can see the issue can read it, so the caller must redact and size-cap the text and consider the audience before it calls; the risk class does not carry that. No dry run: the comment is the whole effect, and a dry run would be the call. No compensating action: this port has no verb that removes a comment, and a correction is a further comment.
	CommentOnIssue(context.Context, *CommentOnIssueRequest) (*CommentOnIssueResponse, error)
	// TransitionIssue moves an issue to a target state category (OPEN, IN_PROGRESS, DONE or CANCELED). The provider chooses the tracker's own transition that reaches the category; when none does from the current state, the answer is FAILED_PRECONDITION and nothing changes. An issue already in the target category answers done (changed false). MEDIUM: a transition fires the customer's workflow automations, which can notify people, start builds or close related work. Dry run checks that a transition to the target category is available and changes nothing; its answer is the one the call would give. The response carries the category before the call, and the compensating action is a TransitionIssue back to that category.
	TransitionIssue(context.Context, *TransitionIssueRequest) (*TransitionIssueResponse, error)
	// LinkIssue makes an issue-to-issue link of type BLOCKS: the issue named first blocks the issue named second. An existing link of the same type between the same two issues answers done (existing true). LOW: a link changes no workflow state and no text. No dry run: the link is the whole effect and its creation is the check of both keys. No compensating action: this port has no verb that removes a link. Other link types, parent and child links and remote URL links are later additive enum values or capabilities.
	LinkIssue(context.Context, *LinkIssueRequest) (*LinkIssueResponse, error)
	mustEmbedUnimplementedTicketingServiceServer()
}

// UnimplementedTicketingServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedTicketingServiceServer struct{}

func (UnimplementedTicketingServiceServer) GetIssue(context.Context, *GetIssueRequest) (*GetIssueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetIssue not implemented")
}
func (UnimplementedTicketingServiceServer) SearchIssues(context.Context, *SearchIssuesRequest) (*SearchIssuesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method SearchIssues not implemented")
}
func (UnimplementedTicketingServiceServer) CommentOnIssue(context.Context, *CommentOnIssueRequest) (*CommentOnIssueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method CommentOnIssue not implemented")
}
func (UnimplementedTicketingServiceServer) TransitionIssue(context.Context, *TransitionIssueRequest) (*TransitionIssueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method TransitionIssue not implemented")
}
func (UnimplementedTicketingServiceServer) LinkIssue(context.Context, *LinkIssueRequest) (*LinkIssueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method LinkIssue not implemented")
}
func (UnimplementedTicketingServiceServer) mustEmbedUnimplementedTicketingServiceServer() {}
func (UnimplementedTicketingServiceServer) testEmbeddedByValue()                          {}

// UnsafeTicketingServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TicketingServiceServer will
// result in compilation errors.
type UnsafeTicketingServiceServer interface {
	mustEmbedUnimplementedTicketingServiceServer()
}

func RegisterTicketingServiceServer(s grpc.ServiceRegistrar, srv TicketingServiceServer) {
	// If the following call panics, it indicates UnimplementedTicketingServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&TicketingService_ServiceDesc, srv)
}

func _TicketingService_GetIssue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetIssueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TicketingServiceServer).GetIssue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TicketingService_GetIssue_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TicketingServiceServer).GetIssue(ctx, req.(*GetIssueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TicketingService_SearchIssues_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SearchIssuesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TicketingServiceServer).SearchIssues(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TicketingService_SearchIssues_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TicketingServiceServer).SearchIssues(ctx, req.(*SearchIssuesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TicketingService_CommentOnIssue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CommentOnIssueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TicketingServiceServer).CommentOnIssue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TicketingService_CommentOnIssue_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TicketingServiceServer).CommentOnIssue(ctx, req.(*CommentOnIssueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TicketingService_TransitionIssue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransitionIssueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TicketingServiceServer).TransitionIssue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TicketingService_TransitionIssue_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TicketingServiceServer).TransitionIssue(ctx, req.(*TransitionIssueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TicketingService_LinkIssue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LinkIssueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TicketingServiceServer).LinkIssue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TicketingService_LinkIssue_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TicketingServiceServer).LinkIssue(ctx, req.(*LinkIssueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// TicketingService_ServiceDesc is the grpc.ServiceDesc for TicketingService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TicketingService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "numinance.ports.ticketing.v1.TicketingService",
	HandlerType: (*TicketingServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetIssue",
			Handler:    _TicketingService_GetIssue_Handler,
		},
		{
			MethodName: "SearchIssues",
			Handler:    _TicketingService_SearchIssues_Handler,
		},
		{
			MethodName: "CommentOnIssue",
			Handler:    _TicketingService_CommentOnIssue_Handler,
		},
		{
			MethodName: "TransitionIssue",
			Handler:    _TicketingService_TransitionIssue_Handler,
		},
		{
			MethodName: "LinkIssue",
			Handler:    _TicketingService_LinkIssue_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "numinance/ports/ticketing/v1/ticketing.proto",
}
