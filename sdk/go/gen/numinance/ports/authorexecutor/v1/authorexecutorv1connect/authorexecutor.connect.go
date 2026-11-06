// The authorexecutor port (ADR 0008): one coding harness behind one adapter, driven for one run of one work unit inside the work-unit sandbox.
// Callers: only the execution context calls this port, for a run whose ActionContext.actor_kind is author-agent or reviewer-agent; the host refuses every other actor kind. A reviewer-agent run is read-only: its request has no allowed_paths, no credential_capabilities and no FILESYSTEM_WRITE, SHELL or NETWORK in allowed_capabilities, or the host refuses it; the host builds no candidate from a reviewer-agent run and discards its worktree. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: nothing in a request is advisory, and nothing a provider reports is authority. The sandbox, the egress proxy and the broker are configured from the host's resolved grant, never from the request, which is the harness's copy. Inference egress for the pinned model is configured by the host outside allowed_capabilities, credential_capabilities and network_policy_ref, and only to the endpoint of the pinned model_id validated for residency at Change start (ADR 0019): the proxy enforces it, nothing inside the sandbox redirects it, requests to non-inference methods there are denied, and it counts in network_destinations and in the dispatch risk. Its credential is a proxy-injected handle; a materialized key only when the manifest declares materialized and policy allows it, scoped to inference on that model, expiring by the deadline, never the application key, and recorded in host evidence. Under the PROCESS sandbox profile (sandboxprovider port) there is no egress proxy: the inference credential is a materialized key only where the manifest and policy allow it, nothing stops a redirect of inference egress, and the decision for Execute is PRIVILEGED; without that allowance the host refuses the run. The host recomputes every authority field of a ChangeExecutionRequest from the grant, the router decision and the plan, and refuses to dispatch a request where any differs. Capabilities and every ExecutionEvent are claims of the provider; the host records its own broker, proxy and sandbox decisions and never treats an event as an authorization. EXECUTION_COMPLETED is never a gate signal: gates read verification. The host reports a canary or an issued handle in an event, a FILE_CHANGED outside allowed_paths and a candidate the final-tree check below refuses, as an abnormal action, which starts the incident workflow (the design).
// Payloads: requests and events carry references and digests, never file contents, prompt text or credential material (a platform invariant: payloads carry references, never content). Of the request's references the provider may resolve through HostService.ResolveReference only task_ref, plan_ref, context_bundle_ref and test_requirements_ref; worktree_ref and network_policy_ref are host-only (field option host_only) and refused. Content an event points at is untrusted data.
// Paths: a path is relative to the worktree root, '/'-separated, NFC, printable, with segments of 1 to 255 bytes, no leading '/', no '\' and no '.' or '..' segment; it names a file or a directory with everything below it. Matching is segment-wise and literal (no globs); denied and protected paths match under Unicode simple case folding of the NFC form. The sandbox enforces on the resolved path after following symlinks and refuses hardlinks; the host checks denied and protected paths again on the final tree of the candidate, after NFC normalization and case folding, and rejects a name that is not NFC, a symlink whose target leaves the allowed paths, and a gitlink. The host builds the candidate against the run's base with every masked path (sandboxprovider port, Paths) taken from the base, so a masked path missing from the worktree on disk (a restored run, or PROCESS) is never a deletion; before that substitution it refuses, as an abnormal action, an entry on disk at or under a masked path that differs from the base (present where the base has none, or other content), so a write there is reported, not dropped; and it refuses a candidate that removes, renames or changes an entry at or under a protected or denied path. A segment that case-folds to .git, .gitmodules, .gitattributes or .lfsconfig, at any depth, is outside every grant, whatever allowed_paths says. The host reads the candidate tree with git hooks, fsmonitor, filter and smudge drivers, LFS and submodule recursion disabled, and never applies repository-local git configuration.
// Stream: Execute is server-streaming. A stream holds 1 to 1023 events: its first is EXECUTION_STARTED at sequence 1, sequences are consecutive, and its last and only its last is EXECUTION_COMPLETED or EXECUTION_FAILED. Each ExecuteResponse is at most 32 KiB serialized. On the plugin protocol each event rides one RUNNING Invoke message and the terminal Invoke message is SUCCEEDED after EXECUTION_COMPLETED and FAILED after EXECUTION_FAILED, so the call fits the Invoke bounds for an OPERATION target (1024 messages, 64 KiB per RUNNING message); the mapping is the server-streaming rule of PluginService.Invoke (plugin/v1). The host rejects a stream that breaks a rule and records the run as failed.
// Status codes: a request is checked before the harness starts, and a rejection sends no event. The host refuses with FAILED_PRECONDITION, before any Invoke, a request its recomputation or the pinned ExecutorManifest refuses, or a run_id already dispatched, records the refusal and returns it to the execution context. The provider rejects with INVALID_ARGUMENT a request that breaks a bound and with UNAVAILABLE when the harness cannot start. Once EXECUTION_STARTED is sent, every failure ends the stream with EXECUTION_FAILED. At the deadline or on a cancel the host ends the call and records the run as failed; the provider sends nothing after it. On the plugin protocol a provider rejection is the terminal FAILED message with one Diagnostic reason, INVALID_ARGUMENT with REASON_INVALID_ARGUMENT and UNAVAILABLE with REASON_UPSTREAM_UNAVAILABLE, and the host records the deadline as REASON_TIMEOUT and a cancel as REASON_CANCELLED. A stream that reaches 1023 events without a terminal one is budget exhaustion: the run fails and the execution context pauses the Change and escalates.
// Evidence: the stream is recorded as a provider claim, and evaluation outcomes come from host verification. Execute declares no evidence types: the execution context writes the stream and route records itself as host receipts, never a provider claim, and a declared type the provider does not emit would make the invocation PARTIAL.
// Bounds: every string is at most 1024 bytes and every repeated field at most 256 entries unless stated; over-limit input is rejected, never truncated; duplicate entries in any repeated field are rejected; enums are closed, and zero and unknown numbers are rejected. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Every element is since 1.0.0.

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/authorexecutor/v1/authorexecutor.proto

package authorexecutorv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1"
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
	// AuthorExecutorServiceName is the fully-qualified name of the AuthorExecutorService service.
	AuthorExecutorServiceName = "numinance.ports.authorexecutor.v1.AuthorExecutorService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// AuthorExecutorServiceCapabilitiesProcedure is the fully-qualified name of the
	// AuthorExecutorService's Capabilities RPC.
	AuthorExecutorServiceCapabilitiesProcedure = "/numinance.ports.authorexecutor.v1.AuthorExecutorService/Capabilities"
	// AuthorExecutorServiceExecuteProcedure is the fully-qualified name of the AuthorExecutorService's
	// Execute RPC.
	AuthorExecutorServiceExecuteProcedure = "/numinance.ports.authorexecutor.v1.AuthorExecutorService/Execute"
	// AuthorExecutorServicePlanProcedure is the fully-qualified name of the AuthorExecutorService's
	// Plan RPC.
	AuthorExecutorServicePlanProcedure = "/numinance.ports.authorexecutor.v1.AuthorExecutorService/Plan"
)

// AuthorExecutorServiceClient is a client for the
// numinance.ports.authorexecutor.v1.AuthorExecutorService service.
type AuthorExecutorServiceClient interface {
	// Capabilities returns the executor capabilities the adapter claims (ExecutorCapabilities). The host calls it just before Execute, as its own READ invocation with its own decision under the run's grant, and keeps the answer for that run only; the router reads the ExecutorManifest pinned in the lockfile, never this answer. An answer that names a capability the pinned manifest does not declare is a provider fault and the host dispatches no Execute for the run; a request whose allowed_capabilities name one the answer omits is refused.
	Capabilities(context.Context, *connect.Request[v1.CapabilitiesRequest]) (*connect.Response[v1.CapabilitiesResponse], error)
	// Execute runs the harness for one run of one work unit and streams its ExecutionEvents. MEDIUM: the harness writes only inside the disposable work-unit sandbox, and its effects outside it pass the egress proxy and the broker under the run's grant; the decision for a dispatch takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. Not idempotent: a harness is not deterministic, so the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. No dry run: the plan was gated before Execute (PlanGate), the sandbox is discarded unless the candidate is assembled, and running the harness is the only way to learn what it would change.
	Execute(context.Context, *connect.Request[v1.ExecuteRequest]) (*connect.ServerStreamForClient[v1.ExecuteResponse], error)
	// Plan runs the harness as the planning run of a Change: read only, in state Classified, before any plan exists, and streams the same ExecutionEvents as Execute, in PlanResponse. READ: the run writes nothing the Change keeps. The host runs it with a read-only worktree, allowed_paths empty, no credential_capabilities and no capability that writes, and refuses any request that breaks one of these before it dispatches; it never assembles a candidate from a Plan run; a FILE_CHANGED event of a Plan run breaks the stream. The plan it produces is a document the host stores as an artifact of the run, which the Engine parses as untrusted input and freezes as the plan digest before any write grant (and so any Execute) can be issued. The decision for a dispatch takes the higher of READ and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED, exactly as for Execute, so a planner that reaches a model through the egress proxy is decided at that risk. It is a verb of its own, and not an Execute without a work unit, because the policy decision point derives side effect from the verb alone and Execute is MEDIUM and bound to a gated plan (the plan floor), which a planning run precedes. Not idempotent and no dry run, for the reasons of Execute: a harness is not deterministic, the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. Plan declares no evidence types, as Execute declares none: the host records the stream of the run and the document it stores as its own evidence, and a provider claim is never a receipt. Plan is a capability, not a Core verb: an adapter that serves it claims EXECUTOR_CAPABILITY_STRUCTURED_PLAN in its Capabilities answer (negotiated through that verb, as the host does just before a run), and the host dispatches Plan only to an adapter whose answer contains it (a request whose allowed_capabilities name it is refused when the answer omits it, as for any capability). An adapter that does not serve it answers UNIMPLEMENTED even so, which the host treats as an executor that cannot plan: the run fails closed and no write grant is issued. Not yet done: no host code dispatches this RPC. Today a planning run is dispatched through Execute, on the fake executor only (the policy decision is still taken under the READ verb and the grant capability authorexecutor.plan); the Plan dispatch, the host refusal of an adapter that omits STRUCTURED_PLAN and the UNIMPLEMENTED fail-closed defence are owed with the read-only worktree prerequisite (ADR 0046, C4).
	Plan(context.Context, *connect.Request[v1.PlanRequest]) (*connect.ServerStreamForClient[v1.PlanResponse], error)
}

// NewAuthorExecutorServiceClient constructs a client for the
// numinance.ports.authorexecutor.v1.AuthorExecutorService service. By default, it uses the Connect
// protocol with the binary Protobuf Codec, asks for gzipped responses, and sends uncompressed
// requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewAuthorExecutorServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) AuthorExecutorServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	authorExecutorServiceMethods := v1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto.Services().ByName("AuthorExecutorService").Methods()
	return &authorExecutorServiceClient{
		capabilities: connect.NewClient[v1.CapabilitiesRequest, v1.CapabilitiesResponse](
			httpClient,
			baseURL+AuthorExecutorServiceCapabilitiesProcedure,
			connect.WithSchema(authorExecutorServiceMethods.ByName("Capabilities")),
			connect.WithClientOptions(opts...),
		),
		execute: connect.NewClient[v1.ExecuteRequest, v1.ExecuteResponse](
			httpClient,
			baseURL+AuthorExecutorServiceExecuteProcedure,
			connect.WithSchema(authorExecutorServiceMethods.ByName("Execute")),
			connect.WithClientOptions(opts...),
		),
		plan: connect.NewClient[v1.PlanRequest, v1.PlanResponse](
			httpClient,
			baseURL+AuthorExecutorServicePlanProcedure,
			connect.WithSchema(authorExecutorServiceMethods.ByName("Plan")),
			connect.WithClientOptions(opts...),
		),
	}
}

// authorExecutorServiceClient implements AuthorExecutorServiceClient.
type authorExecutorServiceClient struct {
	capabilities *connect.Client[v1.CapabilitiesRequest, v1.CapabilitiesResponse]
	execute      *connect.Client[v1.ExecuteRequest, v1.ExecuteResponse]
	plan         *connect.Client[v1.PlanRequest, v1.PlanResponse]
}

// Capabilities calls numinance.ports.authorexecutor.v1.AuthorExecutorService.Capabilities.
func (c *authorExecutorServiceClient) Capabilities(ctx context.Context, req *connect.Request[v1.CapabilitiesRequest]) (*connect.Response[v1.CapabilitiesResponse], error) {
	return c.capabilities.CallUnary(ctx, req)
}

// Execute calls numinance.ports.authorexecutor.v1.AuthorExecutorService.Execute.
func (c *authorExecutorServiceClient) Execute(ctx context.Context, req *connect.Request[v1.ExecuteRequest]) (*connect.ServerStreamForClient[v1.ExecuteResponse], error) {
	return c.execute.CallServerStream(ctx, req)
}

// Plan calls numinance.ports.authorexecutor.v1.AuthorExecutorService.Plan.
func (c *authorExecutorServiceClient) Plan(ctx context.Context, req *connect.Request[v1.PlanRequest]) (*connect.ServerStreamForClient[v1.PlanResponse], error) {
	return c.plan.CallServerStream(ctx, req)
}

// AuthorExecutorServiceHandler is an implementation of the
// numinance.ports.authorexecutor.v1.AuthorExecutorService service.
type AuthorExecutorServiceHandler interface {
	// Capabilities returns the executor capabilities the adapter claims (ExecutorCapabilities). The host calls it just before Execute, as its own READ invocation with its own decision under the run's grant, and keeps the answer for that run only; the router reads the ExecutorManifest pinned in the lockfile, never this answer. An answer that names a capability the pinned manifest does not declare is a provider fault and the host dispatches no Execute for the run; a request whose allowed_capabilities name one the answer omits is refused.
	Capabilities(context.Context, *connect.Request[v1.CapabilitiesRequest]) (*connect.Response[v1.CapabilitiesResponse], error)
	// Execute runs the harness for one run of one work unit and streams its ExecutionEvents. MEDIUM: the harness writes only inside the disposable work-unit sandbox, and its effects outside it pass the egress proxy and the broker under the run's grant; the decision for a dispatch takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. Not idempotent: a harness is not deterministic, so the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. No dry run: the plan was gated before Execute (PlanGate), the sandbox is discarded unless the candidate is assembled, and running the harness is the only way to learn what it would change.
	Execute(context.Context, *connect.Request[v1.ExecuteRequest], *connect.ServerStream[v1.ExecuteResponse]) error
	// Plan runs the harness as the planning run of a Change: read only, in state Classified, before any plan exists, and streams the same ExecutionEvents as Execute, in PlanResponse. READ: the run writes nothing the Change keeps. The host runs it with a read-only worktree, allowed_paths empty, no credential_capabilities and no capability that writes, and refuses any request that breaks one of these before it dispatches; it never assembles a candidate from a Plan run; a FILE_CHANGED event of a Plan run breaks the stream. The plan it produces is a document the host stores as an artifact of the run, which the Engine parses as untrusted input and freezes as the plan digest before any write grant (and so any Execute) can be issued. The decision for a dispatch takes the higher of READ and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED, exactly as for Execute, so a planner that reaches a model through the egress proxy is decided at that risk. It is a verb of its own, and not an Execute without a work unit, because the policy decision point derives side effect from the verb alone and Execute is MEDIUM and bound to a gated plan (the plan floor), which a planning run precedes. Not idempotent and no dry run, for the reasons of Execute: a harness is not deterministic, the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. Plan declares no evidence types, as Execute declares none: the host records the stream of the run and the document it stores as its own evidence, and a provider claim is never a receipt. Plan is a capability, not a Core verb: an adapter that serves it claims EXECUTOR_CAPABILITY_STRUCTURED_PLAN in its Capabilities answer (negotiated through that verb, as the host does just before a run), and the host dispatches Plan only to an adapter whose answer contains it (a request whose allowed_capabilities name it is refused when the answer omits it, as for any capability). An adapter that does not serve it answers UNIMPLEMENTED even so, which the host treats as an executor that cannot plan: the run fails closed and no write grant is issued. Not yet done: no host code dispatches this RPC. Today a planning run is dispatched through Execute, on the fake executor only (the policy decision is still taken under the READ verb and the grant capability authorexecutor.plan); the Plan dispatch, the host refusal of an adapter that omits STRUCTURED_PLAN and the UNIMPLEMENTED fail-closed defence are owed with the read-only worktree prerequisite (ADR 0046, C4).
	Plan(context.Context, *connect.Request[v1.PlanRequest], *connect.ServerStream[v1.PlanResponse]) error
}

// NewAuthorExecutorServiceHandler builds an HTTP handler from the service implementation. It
// returns the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewAuthorExecutorServiceHandler(svc AuthorExecutorServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	authorExecutorServiceMethods := v1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto.Services().ByName("AuthorExecutorService").Methods()
	authorExecutorServiceCapabilitiesHandler := connect.NewUnaryHandler(
		AuthorExecutorServiceCapabilitiesProcedure,
		svc.Capabilities,
		connect.WithSchema(authorExecutorServiceMethods.ByName("Capabilities")),
		connect.WithHandlerOptions(opts...),
	)
	authorExecutorServiceExecuteHandler := connect.NewServerStreamHandler(
		AuthorExecutorServiceExecuteProcedure,
		svc.Execute,
		connect.WithSchema(authorExecutorServiceMethods.ByName("Execute")),
		connect.WithHandlerOptions(opts...),
	)
	authorExecutorServicePlanHandler := connect.NewServerStreamHandler(
		AuthorExecutorServicePlanProcedure,
		svc.Plan,
		connect.WithSchema(authorExecutorServiceMethods.ByName("Plan")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.authorexecutor.v1.AuthorExecutorService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case AuthorExecutorServiceCapabilitiesProcedure:
			authorExecutorServiceCapabilitiesHandler.ServeHTTP(w, r)
		case AuthorExecutorServiceExecuteProcedure:
			authorExecutorServiceExecuteHandler.ServeHTTP(w, r)
		case AuthorExecutorServicePlanProcedure:
			authorExecutorServicePlanHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedAuthorExecutorServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedAuthorExecutorServiceHandler struct{}

func (UnimplementedAuthorExecutorServiceHandler) Capabilities(context.Context, *connect.Request[v1.CapabilitiesRequest]) (*connect.Response[v1.CapabilitiesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.authorexecutor.v1.AuthorExecutorService.Capabilities is not implemented"))
}

func (UnimplementedAuthorExecutorServiceHandler) Execute(context.Context, *connect.Request[v1.ExecuteRequest], *connect.ServerStream[v1.ExecuteResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.authorexecutor.v1.AuthorExecutorService.Execute is not implemented"))
}

func (UnimplementedAuthorExecutorServiceHandler) Plan(context.Context, *connect.Request[v1.PlanRequest], *connect.ServerStream[v1.PlanResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.authorexecutor.v1.AuthorExecutorService.Plan is not implemented"))
}
