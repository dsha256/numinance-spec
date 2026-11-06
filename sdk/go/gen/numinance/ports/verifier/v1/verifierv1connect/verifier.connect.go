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

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/verifier/v1/verifier.proto

package verifierv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
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
	// VerifierServiceName is the fully-qualified name of the VerifierService service.
	VerifierServiceName = "numinance.ports.verifier.v1.VerifierService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// VerifierServiceRunProcedure is the fully-qualified name of the VerifierService's Run RPC.
	VerifierServiceRunProcedure = "/numinance.ports.verifier.v1.VerifierService/Run"
)

// VerifierServiceClient is a client for the numinance.ports.verifier.v1.VerifierService service.
type VerifierServiceClient interface {
	// Run runs the verifier's checks, as config_ref configures them, on the candidate of candidate_ref and returns the findings. LOW, not READ: it executes the checks, and through them candidate code such as tests, inside the verifier's own disposable sandbox with default-deny egress; it changes nothing outside that sandbox and the candidate is read-only. Idempotent, and the manifest declares IDEMPOTENCY_NATURAL: a replay has no effect outside the sandbox, though a flaky check may report other findings, so the host never returns a stored result for a new Run and the verification context records each run. No dry run: running the checks is the only way to learn the findings, and a dry run would be the run itself.
	Run(context.Context, *connect.Request[v1.RunRequest]) (*connect.Response[v1.RunResponse], error)
}

// NewVerifierServiceClient constructs a client for the numinance.ports.verifier.v1.VerifierService
// service. By default, it uses the Connect protocol with the binary Protobuf Codec, asks for
// gzipped responses, and sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply
// the connect.WithGRPC() or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewVerifierServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) VerifierServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	verifierServiceMethods := v1.File_numinance_ports_verifier_v1_verifier_proto.Services().ByName("VerifierService").Methods()
	return &verifierServiceClient{
		run: connect.NewClient[v1.RunRequest, v1.RunResponse](
			httpClient,
			baseURL+VerifierServiceRunProcedure,
			connect.WithSchema(verifierServiceMethods.ByName("Run")),
			connect.WithClientOptions(opts...),
		),
	}
}

// verifierServiceClient implements VerifierServiceClient.
type verifierServiceClient struct {
	run *connect.Client[v1.RunRequest, v1.RunResponse]
}

// Run calls numinance.ports.verifier.v1.VerifierService.Run.
func (c *verifierServiceClient) Run(ctx context.Context, req *connect.Request[v1.RunRequest]) (*connect.Response[v1.RunResponse], error) {
	return c.run.CallUnary(ctx, req)
}

// VerifierServiceHandler is an implementation of the numinance.ports.verifier.v1.VerifierService
// service.
type VerifierServiceHandler interface {
	// Run runs the verifier's checks, as config_ref configures them, on the candidate of candidate_ref and returns the findings. LOW, not READ: it executes the checks, and through them candidate code such as tests, inside the verifier's own disposable sandbox with default-deny egress; it changes nothing outside that sandbox and the candidate is read-only. Idempotent, and the manifest declares IDEMPOTENCY_NATURAL: a replay has no effect outside the sandbox, though a flaky check may report other findings, so the host never returns a stored result for a new Run and the verification context records each run. No dry run: running the checks is the only way to learn the findings, and a dry run would be the run itself.
	Run(context.Context, *connect.Request[v1.RunRequest]) (*connect.Response[v1.RunResponse], error)
}

// NewVerifierServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewVerifierServiceHandler(svc VerifierServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	verifierServiceMethods := v1.File_numinance_ports_verifier_v1_verifier_proto.Services().ByName("VerifierService").Methods()
	verifierServiceRunHandler := connect.NewUnaryHandler(
		VerifierServiceRunProcedure,
		svc.Run,
		connect.WithSchema(verifierServiceMethods.ByName("Run")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.verifier.v1.VerifierService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case VerifierServiceRunProcedure:
			verifierServiceRunHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedVerifierServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedVerifierServiceHandler struct{}

func (UnimplementedVerifierServiceHandler) Run(context.Context, *connect.Request[v1.RunRequest]) (*connect.Response[v1.RunResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.verifier.v1.VerifierService.Run is not implemented"))
}
