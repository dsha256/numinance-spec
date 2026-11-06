// The verifier port (ADR 0009): one verifier runs its checks on one candidate and reports findings.
// Callers: only the verification context calls this port, for a run whose ActionContext.actor_kind is reviewer-agent, whose actor_principal and run_id each differ from those of every author run of the Change, not only the latest; the host refuses every other actor kind and a call that breaks either inequality. The verifier runs in its own sandbox with read-only access to the candidate, no write scope and no credential capability. The port is never a model tool, and no tool_groups or scopes are declared.
// Sandbox: Run executes candidate code, such as agent-written tests, so the verifier's sandbox has default-deny egress: no egress destination and no credential intent is granted, IMDS and link-local ranges are denied, and the only connection is the one to the host. The host refuses to dispatch Run to a verifier whose manifest or grant names any egress destination or credential intent, and runs it at least at the sandbox floor's minimum profile, recorded in evidence. What the checks need comes with the candidate or the verifier's image. Candidate code: any check that executes candidate code (tests, build scripts, hooks) runs in a nested sandbox under a separate uid and its own namespaces, with no access to the plugin or host channel or any inherited descriptor, no write access to the verifier's binaries, working files or reports, and no ptrace or signals to the verifier's processes; its output is data for that check only and never changes another check's report, the evidence the verifier emits or the RunResponse. The verifier reads that output as untrusted input: size-limited, where output over the limit ends the call FAILED and is never truncated to continue, parsed without entity or include expansion, and a parse failure ends the call FAILED. Where the nested sandbox cannot be built, as under the process profile, the call ends UNAVAILABLE and candidate code never runs unnested. LOW rests on these rules.
// Configuration: config_ref is the only configuration the checks honour. The host takes it from the LanguageToolchain or ArchitecturePack binding and refuses one that names an artifact of the candidate or of the author run. The provider ignores or overrides configuration and suppression directives inside the candidate (tool settings files, inline suppressions, test hooks); where a tool cannot ignore one, the provider reports each such directive as a finding, so an author cannot switch a check off from the candidate.
// Untrusted answers: every finding is a provider claim and untrusted data. The verification context checks the bounds below, stores the findings as an artifact referenced by digest, and the gate reads the Verdict, never raw findings. A message, path or rule_id is rendered delimited and labelled as untrusted wherever it reaches a prompt, a ticket or a PR, never as instructions.
// Payloads: candidate_ref and config_ref are references the provider resolves through HostService.ResolveReference; no request or response carries candidate content (a platform invariant: payloads carry references, never content).
// Operation: Run is one OPERATION call on PluginService.Invoke: RUNNING messages carry status and diagnostics only, and the terminal message carries RunResponse when SUCCEEDED. There are no start, get and cancel verbs because the host holds the stream: at the deadline or on a cancel the host ends the call and records the run as failed. A lost stream is not reattached; the verification context starts a new Run, which is safe because Run changes nothing outside its sandbox.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound, NOT_FOUND for a reference the host cannot resolve, UNAVAILABLE when the checks cannot start, including a config naming a check the provider cannot run and a nested sandbox that cannot be built. A check that crashes, times out or cannot complete ends the call FAILED with no findings, never SUCCEEDED with a partial list, and the layer is inconclusive, never passed. Findings are results, not failures: a run that finds violations SUCCEEDS.
// Evidence: Run declares one record type, VerifierReport (fixed with the verification context before the first spec tag): a provider emits exactly one per call through HostService.EmitEvidence, a compact receipt of what the tool said, never the findings. Its payload is a google.protobuf.Struct of at most 16 KiB with the keys check (the check the config named), exit_code, output_bytes, output_sha256 (the digest of the raw tool output, so the evidence names what the tool said without storing it), findings (the count after merging by fingerprint) and replayed (true when a recorded output stood in for the tool). A call that ends SUCCEEDED without it is PARTIAL, and the verification context treats the layer as inconclusive. The design's evidence_refs are the ActionResult.evidence_ids of the terminal message (HostService.EmitEvidence), so a provider cannot name an artifact it did not emit.
// Bounds: every string is at most 1024 bytes unless stated; output still out of bounds after the normalisation stated on a field is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding), so a provider sends a field added in a later minor only when HandshakeRequest.spec_version is at least that minor or behind a negotiated capability. Printable means Unicode categories L, M, N, P, S and U+0020. A RunResponse is at most 1024 findings of at most about 2.6 KiB each, so it fits one Invoke message of at most 4 MiB. The host rejects a RunResponse that breaks a rule of this file and records the run as failed.
// Every element is since 1.0.0.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.6.2
// - protoc             (unknown)
// source: numinance/ports/verifier/v1/verifier.proto

package verifierv1

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
	VerifierService_Run_FullMethodName = "/numinance.ports.verifier.v1.VerifierService/Run"
)

// VerifierServiceClient is the client API for VerifierService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// VerifierService is the verifier port a verifier provider serves.
type VerifierServiceClient interface {
	// Run runs the verifier's checks, as config_ref configures them, on the candidate of candidate_ref and returns the findings. LOW, not READ: it executes the checks, and through them candidate code such as tests, inside the verifier's own disposable sandbox with default-deny egress; it changes nothing outside that sandbox and the candidate is read-only. Idempotent, and the manifest declares IDEMPOTENCY_NATURAL: a replay has no effect outside the sandbox, though a flaky check may report other findings, so the host never returns a stored result for a new Run and the verification context records each run. No dry run: running the checks is the only way to learn the findings, and a dry run would be the run itself.
	Run(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (*RunResponse, error)
}

type verifierServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewVerifierServiceClient(cc grpc.ClientConnInterface) VerifierServiceClient {
	return &verifierServiceClient{cc}
}

func (c *verifierServiceClient) Run(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (*RunResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RunResponse)
	err := c.cc.Invoke(ctx, VerifierService_Run_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// VerifierServiceServer is the server API for VerifierService service.
// All implementations must embed UnimplementedVerifierServiceServer
// for forward compatibility.
//
// VerifierService is the verifier port a verifier provider serves.
type VerifierServiceServer interface {
	// Run runs the verifier's checks, as config_ref configures them, on the candidate of candidate_ref and returns the findings. LOW, not READ: it executes the checks, and through them candidate code such as tests, inside the verifier's own disposable sandbox with default-deny egress; it changes nothing outside that sandbox and the candidate is read-only. Idempotent, and the manifest declares IDEMPOTENCY_NATURAL: a replay has no effect outside the sandbox, though a flaky check may report other findings, so the host never returns a stored result for a new Run and the verification context records each run. No dry run: running the checks is the only way to learn the findings, and a dry run would be the run itself.
	Run(context.Context, *RunRequest) (*RunResponse, error)
	mustEmbedUnimplementedVerifierServiceServer()
}

// UnimplementedVerifierServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedVerifierServiceServer struct{}

func (UnimplementedVerifierServiceServer) Run(context.Context, *RunRequest) (*RunResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Run not implemented")
}
func (UnimplementedVerifierServiceServer) mustEmbedUnimplementedVerifierServiceServer() {}
func (UnimplementedVerifierServiceServer) testEmbeddedByValue()                         {}

// UnsafeVerifierServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to VerifierServiceServer will
// result in compilation errors.
type UnsafeVerifierServiceServer interface {
	mustEmbedUnimplementedVerifierServiceServer()
}

func RegisterVerifierServiceServer(s grpc.ServiceRegistrar, srv VerifierServiceServer) {
	// If the following call panics, it indicates UnimplementedVerifierServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&VerifierService_ServiceDesc, srv)
}

func _VerifierService_Run_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VerifierServiceServer).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VerifierService_Run_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VerifierServiceServer).Run(ctx, req.(*RunRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// VerifierService_ServiceDesc is the grpc.ServiceDesc for VerifierService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var VerifierService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "numinance.ports.verifier.v1.VerifierService",
	HandlerType: (*VerifierServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Run",
			Handler:    _VerifierService_Run_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "numinance/ports/verifier/v1/verifier.proto",
}
