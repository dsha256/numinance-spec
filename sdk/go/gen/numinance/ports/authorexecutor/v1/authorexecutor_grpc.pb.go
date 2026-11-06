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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.6.2
// - protoc             (unknown)
// source: numinance/ports/authorexecutor/v1/authorexecutor.proto

package authorexecutorv1

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
	AuthorExecutorService_Capabilities_FullMethodName = "/numinance.ports.authorexecutor.v1.AuthorExecutorService/Capabilities"
	AuthorExecutorService_Execute_FullMethodName      = "/numinance.ports.authorexecutor.v1.AuthorExecutorService/Execute"
	AuthorExecutorService_Plan_FullMethodName         = "/numinance.ports.authorexecutor.v1.AuthorExecutorService/Plan"
)

// AuthorExecutorServiceClient is the client API for AuthorExecutorService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// AuthorExecutorService is the authorexecutor port an executor adapter serves.
type AuthorExecutorServiceClient interface {
	// Capabilities returns the executor capabilities the adapter claims (ExecutorCapabilities). The host calls it just before Execute, as its own READ invocation with its own decision under the run's grant, and keeps the answer for that run only; the router reads the ExecutorManifest pinned in the lockfile, never this answer. An answer that names a capability the pinned manifest does not declare is a provider fault and the host dispatches no Execute for the run; a request whose allowed_capabilities name one the answer omits is refused.
	Capabilities(ctx context.Context, in *CapabilitiesRequest, opts ...grpc.CallOption) (*CapabilitiesResponse, error)
	// Execute runs the harness for one run of one work unit and streams its ExecutionEvents. MEDIUM: the harness writes only inside the disposable work-unit sandbox, and its effects outside it pass the egress proxy and the broker under the run's grant; the decision for a dispatch takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. Not idempotent: a harness is not deterministic, so the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. No dry run: the plan was gated before Execute (PlanGate), the sandbox is discarded unless the candidate is assembled, and running the harness is the only way to learn what it would change.
	Execute(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ExecuteResponse], error)
	// Plan runs the harness as the planning run of a Change: read only, in state Classified, before any plan exists, and streams the same ExecutionEvents as Execute, in PlanResponse. READ: the run writes nothing the Change keeps. The host runs it with a read-only worktree, allowed_paths empty, no credential_capabilities and no capability that writes, and refuses any request that breaks one of these before it dispatches; it never assembles a candidate from a Plan run; a FILE_CHANGED event of a Plan run breaks the stream. The plan it produces is a document the host stores as an artifact of the run, which the Engine parses as untrusted input and freezes as the plan digest before any write grant (and so any Execute) can be issued. The decision for a dispatch takes the higher of READ and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED, exactly as for Execute, so a planner that reaches a model through the egress proxy is decided at that risk. It is a verb of its own, and not an Execute without a work unit, because the policy decision point derives side effect from the verb alone and Execute is MEDIUM and bound to a gated plan (the plan floor), which a planning run precedes. Not idempotent and no dry run, for the reasons of Execute: a harness is not deterministic, the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. Plan declares no evidence types, as Execute declares none: the host records the stream of the run and the document it stores as its own evidence, and a provider claim is never a receipt. Plan is a capability, not a Core verb: an adapter that serves it claims EXECUTOR_CAPABILITY_STRUCTURED_PLAN in its Capabilities answer (negotiated through that verb, as the host does just before a run), and the host dispatches Plan only to an adapter whose answer contains it (a request whose allowed_capabilities name it is refused when the answer omits it, as for any capability). An adapter that does not serve it answers UNIMPLEMENTED even so, which the host treats as an executor that cannot plan: the run fails closed and no write grant is issued. Not yet done: no host code dispatches this RPC. Today a planning run is dispatched through Execute, on the fake executor only (the policy decision is still taken under the READ verb and the grant capability authorexecutor.plan); the Plan dispatch, the host refusal of an adapter that omits STRUCTURED_PLAN and the UNIMPLEMENTED fail-closed defence are owed with the read-only worktree prerequisite (ADR 0046, C4).
	Plan(ctx context.Context, in *PlanRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[PlanResponse], error)
}

type authorExecutorServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAuthorExecutorServiceClient(cc grpc.ClientConnInterface) AuthorExecutorServiceClient {
	return &authorExecutorServiceClient{cc}
}

func (c *authorExecutorServiceClient) Capabilities(ctx context.Context, in *CapabilitiesRequest, opts ...grpc.CallOption) (*CapabilitiesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CapabilitiesResponse)
	err := c.cc.Invoke(ctx, AuthorExecutorService_Capabilities_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authorExecutorServiceClient) Execute(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ExecuteResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &AuthorExecutorService_ServiceDesc.Streams[0], AuthorExecutorService_Execute_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ExecuteRequest, ExecuteResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type AuthorExecutorService_ExecuteClient = grpc.ServerStreamingClient[ExecuteResponse]

func (c *authorExecutorServiceClient) Plan(ctx context.Context, in *PlanRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[PlanResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &AuthorExecutorService_ServiceDesc.Streams[1], AuthorExecutorService_Plan_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[PlanRequest, PlanResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type AuthorExecutorService_PlanClient = grpc.ServerStreamingClient[PlanResponse]

// AuthorExecutorServiceServer is the server API for AuthorExecutorService service.
// All implementations must embed UnimplementedAuthorExecutorServiceServer
// for forward compatibility.
//
// AuthorExecutorService is the authorexecutor port an executor adapter serves.
type AuthorExecutorServiceServer interface {
	// Capabilities returns the executor capabilities the adapter claims (ExecutorCapabilities). The host calls it just before Execute, as its own READ invocation with its own decision under the run's grant, and keeps the answer for that run only; the router reads the ExecutorManifest pinned in the lockfile, never this answer. An answer that names a capability the pinned manifest does not declare is a provider fault and the host dispatches no Execute for the run; a request whose allowed_capabilities name one the answer omits is refused.
	Capabilities(context.Context, *CapabilitiesRequest) (*CapabilitiesResponse, error)
	// Execute runs the harness for one run of one work unit and streams its ExecutionEvents. MEDIUM: the harness writes only inside the disposable work-unit sandbox, and its effects outside it pass the egress proxy and the broker under the run's grant; the decision for a dispatch takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. Not idempotent: a harness is not deterministic, so the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. No dry run: the plan was gated before Execute (PlanGate), the sandbox is discarded unless the candidate is assembled, and running the harness is the only way to learn what it would change.
	Execute(*ExecuteRequest, grpc.ServerStreamingServer[ExecuteResponse]) error
	// Plan runs the harness as the planning run of a Change: read only, in state Classified, before any plan exists, and streams the same ExecutionEvents as Execute, in PlanResponse. READ: the run writes nothing the Change keeps. The host runs it with a read-only worktree, allowed_paths empty, no credential_capabilities and no capability that writes, and refuses any request that breaks one of these before it dispatches; it never assembles a candidate from a Plan run; a FILE_CHANGED event of a Plan run breaks the stream. The plan it produces is a document the host stores as an artifact of the run, which the Engine parses as untrusted input and freezes as the plan digest before any write grant (and so any Execute) can be issued. The decision for a dispatch takes the higher of READ and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED, exactly as for Execute, so a planner that reaches a model through the egress proxy is decided at that risk. It is a verb of its own, and not an Execute without a work unit, because the policy decision point derives side effect from the verb alone and Execute is MEDIUM and bound to a gated plan (the plan floor), which a planning run precedes. Not idempotent and no dry run, for the reasons of Execute: a harness is not deterministic, the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. Plan declares no evidence types, as Execute declares none: the host records the stream of the run and the document it stores as its own evidence, and a provider claim is never a receipt. Plan is a capability, not a Core verb: an adapter that serves it claims EXECUTOR_CAPABILITY_STRUCTURED_PLAN in its Capabilities answer (negotiated through that verb, as the host does just before a run), and the host dispatches Plan only to an adapter whose answer contains it (a request whose allowed_capabilities name it is refused when the answer omits it, as for any capability). An adapter that does not serve it answers UNIMPLEMENTED even so, which the host treats as an executor that cannot plan: the run fails closed and no write grant is issued. Not yet done: no host code dispatches this RPC. Today a planning run is dispatched through Execute, on the fake executor only (the policy decision is still taken under the READ verb and the grant capability authorexecutor.plan); the Plan dispatch, the host refusal of an adapter that omits STRUCTURED_PLAN and the UNIMPLEMENTED fail-closed defence are owed with the read-only worktree prerequisite (ADR 0046, C4).
	Plan(*PlanRequest, grpc.ServerStreamingServer[PlanResponse]) error
	mustEmbedUnimplementedAuthorExecutorServiceServer()
}

// UnimplementedAuthorExecutorServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedAuthorExecutorServiceServer struct{}

func (UnimplementedAuthorExecutorServiceServer) Capabilities(context.Context, *CapabilitiesRequest) (*CapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Capabilities not implemented")
}
func (UnimplementedAuthorExecutorServiceServer) Execute(*ExecuteRequest, grpc.ServerStreamingServer[ExecuteResponse]) error {
	return status.Error(codes.Unimplemented, "method Execute not implemented")
}
func (UnimplementedAuthorExecutorServiceServer) Plan(*PlanRequest, grpc.ServerStreamingServer[PlanResponse]) error {
	return status.Error(codes.Unimplemented, "method Plan not implemented")
}
func (UnimplementedAuthorExecutorServiceServer) mustEmbedUnimplementedAuthorExecutorServiceServer() {}
func (UnimplementedAuthorExecutorServiceServer) testEmbeddedByValue()                               {}

// UnsafeAuthorExecutorServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AuthorExecutorServiceServer will
// result in compilation errors.
type UnsafeAuthorExecutorServiceServer interface {
	mustEmbedUnimplementedAuthorExecutorServiceServer()
}

func RegisterAuthorExecutorServiceServer(s grpc.ServiceRegistrar, srv AuthorExecutorServiceServer) {
	// If the following call panics, it indicates UnimplementedAuthorExecutorServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&AuthorExecutorService_ServiceDesc, srv)
}

func _AuthorExecutorService_Capabilities_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CapabilitiesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthorExecutorServiceServer).Capabilities(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthorExecutorService_Capabilities_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthorExecutorServiceServer).Capabilities(ctx, req.(*CapabilitiesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthorExecutorService_Execute_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ExecuteRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AuthorExecutorServiceServer).Execute(m, &grpc.GenericServerStream[ExecuteRequest, ExecuteResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type AuthorExecutorService_ExecuteServer = grpc.ServerStreamingServer[ExecuteResponse]

func _AuthorExecutorService_Plan_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(PlanRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AuthorExecutorServiceServer).Plan(m, &grpc.GenericServerStream[PlanRequest, PlanResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type AuthorExecutorService_PlanServer = grpc.ServerStreamingServer[PlanResponse]

// AuthorExecutorService_ServiceDesc is the grpc.ServiceDesc for AuthorExecutorService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AuthorExecutorService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "numinance.ports.authorexecutor.v1.AuthorExecutorService",
	HandlerType: (*AuthorExecutorServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Capabilities",
			Handler:    _AuthorExecutorService_Capabilities_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Execute",
			Handler:       _AuthorExecutorService_Execute_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Plan",
			Handler:       _AuthorExecutorService_Plan_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "numinance/ports/authorexecutor/v1/authorexecutor.proto",
}
