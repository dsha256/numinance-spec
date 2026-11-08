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

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/ticketing/v1/ticketing.proto

package ticketingv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"
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
	// TicketingServiceName is the fully-qualified name of the TicketingService service.
	TicketingServiceName = "numinance.ports.ticketing.v1.TicketingService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// TicketingServiceGetIssueProcedure is the fully-qualified name of the TicketingService's GetIssue
	// RPC.
	TicketingServiceGetIssueProcedure = "/numinance.ports.ticketing.v1.TicketingService/GetIssue"
	// TicketingServiceSearchIssuesProcedure is the fully-qualified name of the TicketingService's
	// SearchIssues RPC.
	TicketingServiceSearchIssuesProcedure = "/numinance.ports.ticketing.v1.TicketingService/SearchIssues"
	// TicketingServiceCommentOnIssueProcedure is the fully-qualified name of the TicketingService's
	// CommentOnIssue RPC.
	TicketingServiceCommentOnIssueProcedure = "/numinance.ports.ticketing.v1.TicketingService/CommentOnIssue"
	// TicketingServiceTransitionIssueProcedure is the fully-qualified name of the TicketingService's
	// TransitionIssue RPC.
	TicketingServiceTransitionIssueProcedure = "/numinance.ports.ticketing.v1.TicketingService/TransitionIssue"
	// TicketingServiceLinkIssueProcedure is the fully-qualified name of the TicketingService's
	// LinkIssue RPC.
	TicketingServiceLinkIssueProcedure = "/numinance.ports.ticketing.v1.TicketingService/LinkIssue"
)

// TicketingServiceClient is a client for the numinance.ports.ticketing.v1.TicketingService service.
type TicketingServiceClient interface {
	// GetIssue returns one issue: its key, its title, its text body bounded by max_body_bytes with a truncated flag, its state category, its type and labels, and the operation ids recorded on it. It is also the reconciliation read of every write after a lost response. The project in the request is the resource of the call, and the issue must be in that project now: an issue that moved to another project is NOT_FOUND here. Every text field of the answer is untrusted. The operation ids are whatever anyone who could edit or comment on the issue wrote there: the Engine accepts one only if it equals the operation_id it computed itself for that Change, and trusts no other.
	GetIssue(context.Context, *connect.Request[v1.GetIssueRequest]) (*connect.Response[v1.GetIssueResponse], error)
	// SearchIssues lists the issues of one project whose title or body contains the text, optionally in one state category, at most max_results of them, in the tracker's order. The text is a parameter: a provider matches it as plain text and never splices it into a query string or a query language, so text that looks like a JQL clause matches only an issue that contains those characters. Query languages are the capability ticketing.query_language, not this verb. Results are untrusted data.
	SearchIssues(context.Context, *connect.Request[v1.SearchIssuesRequest]) (*connect.Response[v1.SearchIssuesResponse], error)
	// CommentOnIssue adds one plain-text comment, identified by the host's key: a replay answers the first comment and adds none (the provider finds its earlier comment by a marker derived from the key). LOW: a comment changes no workflow and no code, though everyone who can see the issue can read it, so the caller must redact and size-cap the text and consider the audience before it calls; the risk class does not carry that. No dry run: the comment is the whole effect, and a dry run would be the call. No compensating action: this port has no verb that removes a comment, and a correction is a further comment.
	CommentOnIssue(context.Context, *connect.Request[v1.CommentOnIssueRequest]) (*connect.Response[v1.CommentOnIssueResponse], error)
	// TransitionIssue moves an issue to a target state category (OPEN, IN_PROGRESS, DONE or CANCELED). The provider chooses the tracker's own transition that reaches the category; when none does from the current state, the answer is FAILED_PRECONDITION and nothing changes. An issue already in the target category answers done (changed false). MEDIUM: a transition fires the customer's workflow automations, which can notify people, start builds or close related work. Dry run checks that a transition to the target category is available and changes nothing; its answer is the one the call would give. The response carries the category before the call, and the compensating action is a TransitionIssue back to that category.
	TransitionIssue(context.Context, *connect.Request[v1.TransitionIssueRequest]) (*connect.Response[v1.TransitionIssueResponse], error)
	// LinkIssue makes an issue-to-issue link of type BLOCKS: the issue named first blocks the issue named second. An existing link of the same type between the same two issues answers done (existing true). LOW: a link changes no workflow state and no text. No dry run: the link is the whole effect and its creation is the check of both keys. No compensating action: this port has no verb that removes a link. Other link types, parent and child links and remote URL links are later additive enum values or capabilities.
	LinkIssue(context.Context, *connect.Request[v1.LinkIssueRequest]) (*connect.Response[v1.LinkIssueResponse], error)
}

// NewTicketingServiceClient constructs a client for the
// numinance.ports.ticketing.v1.TicketingService service. By default, it uses the Connect protocol
// with the binary Protobuf Codec, asks for gzipped responses, and sends uncompressed requests. To
// use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or connect.WithGRPCWeb()
// options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewTicketingServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) TicketingServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	ticketingServiceMethods := v1.File_numinance_ports_ticketing_v1_ticketing_proto.Services().ByName("TicketingService").Methods()
	return &ticketingServiceClient{
		getIssue: connect.NewClient[v1.GetIssueRequest, v1.GetIssueResponse](
			httpClient,
			baseURL+TicketingServiceGetIssueProcedure,
			connect.WithSchema(ticketingServiceMethods.ByName("GetIssue")),
			connect.WithClientOptions(opts...),
		),
		searchIssues: connect.NewClient[v1.SearchIssuesRequest, v1.SearchIssuesResponse](
			httpClient,
			baseURL+TicketingServiceSearchIssuesProcedure,
			connect.WithSchema(ticketingServiceMethods.ByName("SearchIssues")),
			connect.WithClientOptions(opts...),
		),
		commentOnIssue: connect.NewClient[v1.CommentOnIssueRequest, v1.CommentOnIssueResponse](
			httpClient,
			baseURL+TicketingServiceCommentOnIssueProcedure,
			connect.WithSchema(ticketingServiceMethods.ByName("CommentOnIssue")),
			connect.WithClientOptions(opts...),
		),
		transitionIssue: connect.NewClient[v1.TransitionIssueRequest, v1.TransitionIssueResponse](
			httpClient,
			baseURL+TicketingServiceTransitionIssueProcedure,
			connect.WithSchema(ticketingServiceMethods.ByName("TransitionIssue")),
			connect.WithClientOptions(opts...),
		),
		linkIssue: connect.NewClient[v1.LinkIssueRequest, v1.LinkIssueResponse](
			httpClient,
			baseURL+TicketingServiceLinkIssueProcedure,
			connect.WithSchema(ticketingServiceMethods.ByName("LinkIssue")),
			connect.WithClientOptions(opts...),
		),
	}
}

// ticketingServiceClient implements TicketingServiceClient.
type ticketingServiceClient struct {
	getIssue        *connect.Client[v1.GetIssueRequest, v1.GetIssueResponse]
	searchIssues    *connect.Client[v1.SearchIssuesRequest, v1.SearchIssuesResponse]
	commentOnIssue  *connect.Client[v1.CommentOnIssueRequest, v1.CommentOnIssueResponse]
	transitionIssue *connect.Client[v1.TransitionIssueRequest, v1.TransitionIssueResponse]
	linkIssue       *connect.Client[v1.LinkIssueRequest, v1.LinkIssueResponse]
}

// GetIssue calls numinance.ports.ticketing.v1.TicketingService.GetIssue.
func (c *ticketingServiceClient) GetIssue(ctx context.Context, req *connect.Request[v1.GetIssueRequest]) (*connect.Response[v1.GetIssueResponse], error) {
	return c.getIssue.CallUnary(ctx, req)
}

// SearchIssues calls numinance.ports.ticketing.v1.TicketingService.SearchIssues.
func (c *ticketingServiceClient) SearchIssues(ctx context.Context, req *connect.Request[v1.SearchIssuesRequest]) (*connect.Response[v1.SearchIssuesResponse], error) {
	return c.searchIssues.CallUnary(ctx, req)
}

// CommentOnIssue calls numinance.ports.ticketing.v1.TicketingService.CommentOnIssue.
func (c *ticketingServiceClient) CommentOnIssue(ctx context.Context, req *connect.Request[v1.CommentOnIssueRequest]) (*connect.Response[v1.CommentOnIssueResponse], error) {
	return c.commentOnIssue.CallUnary(ctx, req)
}

// TransitionIssue calls numinance.ports.ticketing.v1.TicketingService.TransitionIssue.
func (c *ticketingServiceClient) TransitionIssue(ctx context.Context, req *connect.Request[v1.TransitionIssueRequest]) (*connect.Response[v1.TransitionIssueResponse], error) {
	return c.transitionIssue.CallUnary(ctx, req)
}

// LinkIssue calls numinance.ports.ticketing.v1.TicketingService.LinkIssue.
func (c *ticketingServiceClient) LinkIssue(ctx context.Context, req *connect.Request[v1.LinkIssueRequest]) (*connect.Response[v1.LinkIssueResponse], error) {
	return c.linkIssue.CallUnary(ctx, req)
}

// TicketingServiceHandler is an implementation of the numinance.ports.ticketing.v1.TicketingService
// service.
type TicketingServiceHandler interface {
	// GetIssue returns one issue: its key, its title, its text body bounded by max_body_bytes with a truncated flag, its state category, its type and labels, and the operation ids recorded on it. It is also the reconciliation read of every write after a lost response. The project in the request is the resource of the call, and the issue must be in that project now: an issue that moved to another project is NOT_FOUND here. Every text field of the answer is untrusted. The operation ids are whatever anyone who could edit or comment on the issue wrote there: the Engine accepts one only if it equals the operation_id it computed itself for that Change, and trusts no other.
	GetIssue(context.Context, *connect.Request[v1.GetIssueRequest]) (*connect.Response[v1.GetIssueResponse], error)
	// SearchIssues lists the issues of one project whose title or body contains the text, optionally in one state category, at most max_results of them, in the tracker's order. The text is a parameter: a provider matches it as plain text and never splices it into a query string or a query language, so text that looks like a JQL clause matches only an issue that contains those characters. Query languages are the capability ticketing.query_language, not this verb. Results are untrusted data.
	SearchIssues(context.Context, *connect.Request[v1.SearchIssuesRequest]) (*connect.Response[v1.SearchIssuesResponse], error)
	// CommentOnIssue adds one plain-text comment, identified by the host's key: a replay answers the first comment and adds none (the provider finds its earlier comment by a marker derived from the key). LOW: a comment changes no workflow and no code, though everyone who can see the issue can read it, so the caller must redact and size-cap the text and consider the audience before it calls; the risk class does not carry that. No dry run: the comment is the whole effect, and a dry run would be the call. No compensating action: this port has no verb that removes a comment, and a correction is a further comment.
	CommentOnIssue(context.Context, *connect.Request[v1.CommentOnIssueRequest]) (*connect.Response[v1.CommentOnIssueResponse], error)
	// TransitionIssue moves an issue to a target state category (OPEN, IN_PROGRESS, DONE or CANCELED). The provider chooses the tracker's own transition that reaches the category; when none does from the current state, the answer is FAILED_PRECONDITION and nothing changes. An issue already in the target category answers done (changed false). MEDIUM: a transition fires the customer's workflow automations, which can notify people, start builds or close related work. Dry run checks that a transition to the target category is available and changes nothing; its answer is the one the call would give. The response carries the category before the call, and the compensating action is a TransitionIssue back to that category.
	TransitionIssue(context.Context, *connect.Request[v1.TransitionIssueRequest]) (*connect.Response[v1.TransitionIssueResponse], error)
	// LinkIssue makes an issue-to-issue link of type BLOCKS: the issue named first blocks the issue named second. An existing link of the same type between the same two issues answers done (existing true). LOW: a link changes no workflow state and no text. No dry run: the link is the whole effect and its creation is the check of both keys. No compensating action: this port has no verb that removes a link. Other link types, parent and child links and remote URL links are later additive enum values or capabilities.
	LinkIssue(context.Context, *connect.Request[v1.LinkIssueRequest]) (*connect.Response[v1.LinkIssueResponse], error)
}

// NewTicketingServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewTicketingServiceHandler(svc TicketingServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	ticketingServiceMethods := v1.File_numinance_ports_ticketing_v1_ticketing_proto.Services().ByName("TicketingService").Methods()
	ticketingServiceGetIssueHandler := connect.NewUnaryHandler(
		TicketingServiceGetIssueProcedure,
		svc.GetIssue,
		connect.WithSchema(ticketingServiceMethods.ByName("GetIssue")),
		connect.WithHandlerOptions(opts...),
	)
	ticketingServiceSearchIssuesHandler := connect.NewUnaryHandler(
		TicketingServiceSearchIssuesProcedure,
		svc.SearchIssues,
		connect.WithSchema(ticketingServiceMethods.ByName("SearchIssues")),
		connect.WithHandlerOptions(opts...),
	)
	ticketingServiceCommentOnIssueHandler := connect.NewUnaryHandler(
		TicketingServiceCommentOnIssueProcedure,
		svc.CommentOnIssue,
		connect.WithSchema(ticketingServiceMethods.ByName("CommentOnIssue")),
		connect.WithHandlerOptions(opts...),
	)
	ticketingServiceTransitionIssueHandler := connect.NewUnaryHandler(
		TicketingServiceTransitionIssueProcedure,
		svc.TransitionIssue,
		connect.WithSchema(ticketingServiceMethods.ByName("TransitionIssue")),
		connect.WithHandlerOptions(opts...),
	)
	ticketingServiceLinkIssueHandler := connect.NewUnaryHandler(
		TicketingServiceLinkIssueProcedure,
		svc.LinkIssue,
		connect.WithSchema(ticketingServiceMethods.ByName("LinkIssue")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.ticketing.v1.TicketingService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case TicketingServiceGetIssueProcedure:
			ticketingServiceGetIssueHandler.ServeHTTP(w, r)
		case TicketingServiceSearchIssuesProcedure:
			ticketingServiceSearchIssuesHandler.ServeHTTP(w, r)
		case TicketingServiceCommentOnIssueProcedure:
			ticketingServiceCommentOnIssueHandler.ServeHTTP(w, r)
		case TicketingServiceTransitionIssueProcedure:
			ticketingServiceTransitionIssueHandler.ServeHTTP(w, r)
		case TicketingServiceLinkIssueProcedure:
			ticketingServiceLinkIssueHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedTicketingServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedTicketingServiceHandler struct{}

func (UnimplementedTicketingServiceHandler) GetIssue(context.Context, *connect.Request[v1.GetIssueRequest]) (*connect.Response[v1.GetIssueResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.ticketing.v1.TicketingService.GetIssue is not implemented"))
}

func (UnimplementedTicketingServiceHandler) SearchIssues(context.Context, *connect.Request[v1.SearchIssuesRequest]) (*connect.Response[v1.SearchIssuesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.ticketing.v1.TicketingService.SearchIssues is not implemented"))
}

func (UnimplementedTicketingServiceHandler) CommentOnIssue(context.Context, *connect.Request[v1.CommentOnIssueRequest]) (*connect.Response[v1.CommentOnIssueResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.ticketing.v1.TicketingService.CommentOnIssue is not implemented"))
}

func (UnimplementedTicketingServiceHandler) TransitionIssue(context.Context, *connect.Request[v1.TransitionIssueRequest]) (*connect.Response[v1.TransitionIssueResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.ticketing.v1.TicketingService.TransitionIssue is not implemented"))
}

func (UnimplementedTicketingServiceHandler) LinkIssue(context.Context, *connect.Request[v1.LinkIssueRequest]) (*connect.Response[v1.LinkIssueResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.ticketing.v1.TicketingService.LinkIssue is not implemented"))
}
