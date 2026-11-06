// Package provider is the verifier provider for the Python toolchain: a capability provider of numinance.ports.verifier.v1
// that runs ruff, mypy, pytest, Semgrep or import-linter on a candidate and reports their findings. It is written against the
// public SDK only. What it reads (the config, the candidate archive, every tool output) is untrusted: size-limited,
// schema-validated and timeout-bound before use, and what it reports is a claim the verification context checks again.
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// SpecVersion is the spec version the provider was built against.
const SpecVersion = "1.0.0"

const (
	// PluginID is the extension id of the provider that runs the real tools.
	PluginID = "dev.numinance.verifier-python"
	// ReplayPluginID is the extension id of the replaying build: the same provider over recorded tool output.
	ReplayPluginID = "dev.numinance.verifier-python-replay"
	// RecordType is the evidence record type the port declares for Run.
	RecordType = "VerifierReport"
	// Capability is the capability name of the one verb.
	Capability = "verifier.run"
	portName   = "numinance.ports.verifier.v1"
	typePrefix = "type.googleapis.com/"
	runMethod  = "/numinance.ports.verifier.v1.VerifierService/Run"
	maxChunk   = 64 << 10
)

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Server is the PluginService of the provider.
type Server struct {
	pluginv1.UnimplementedPluginServiceServer
	runner   Runner
	host     Host
	manifest *pluginv1.Manifest
	stop     func()
}

// New returns the server. id is PluginID or ReplayPluginID, digest the content digest the manifest declares (main passes
// the digest of its own executable), runner runs the checks, host reaches the HostService of the launch, and stop ends the
// process after Shutdown.
func New(id, digest string, runner Runner, host Host, stop func()) *Server {
	ref := func(key string) *typesv1.Reference {
		sum := sha256.Sum256([]byte("verifier-python:" + key))
		return &typesv1.Reference{ArtifactRef: key, Sha256: &typesv1.Digest{Value: "sha256:" + hex.EncodeToString(sum[:])}, ArtifactType: "sample"}
	}
	return &Server{runner: runner, host: host, stop: stop, manifest: &pluginv1.Manifest{
		PluginId: id, Version: "1.0.0", Digest: &typesv1.Digest{Value: digest}, SignerIdentity: "https://numinance.dev/official|verifier-python",
		Kind: pluginv1.ExtensionClass_EXTENSION_CLASS_CAPABILITY_PROVIDER, Ports: []string{portName},
		Capabilities: []*pluginv1.CapabilityDescriptor{{
			Name: Capability, RiskFloor: annotationsv1.RiskClass_RISK_CLASS_LOW, SideEffect: pluginv1.SideEffect_SIDE_EFFECT_WRITE,
			Idempotency: pluginv1.Idempotency_IDEMPOTENCY_NATURAL, EnvironmentScope: []string{"dev", "staging", "prod"},
			EvidenceDeclared: []string{RecordType}, ConformanceSuite: "conformance/verifier/v1/basic",
			LlmDescription: "Runs one Python toolchain check on a candidate and returns its findings.", Timeout: durationpb.New(120_000_000_000),
		}},
		SbomRef: ref("sbom/verifier-python"), ProvenanceRef: ref("provenance/verifier-python"),
	}}
}

// ExecutableDigest returns "sha256:" plus the hex digest of the running executable.
func ExecutableDigest() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path) //nolint:gosec // the path of the running executable
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// Register registers the PluginService on g.
func (s *Server) Register(g *grpc.Server) { pluginv1.RegisterPluginServiceServer(g, s) }

// Handshake implements PluginService.
func (s *Server) Handshake(_ context.Context, req *pluginv1.HandshakeRequest) (*pluginv1.HandshakeResponse, error) {
	for _, v := range req.GetProtocolVersions() {
		if v == plugin.AppProtocolVersion {
			return &pluginv1.HandshakeResponse{ProtocolVersion: plugin.AppProtocolVersion, SpecVersion: SpecVersion}, nil
		}
	}
	return nil, status.Error(codes.FailedPrecondition, "no common protocol version")
}

// GetManifest implements PluginService.
func (s *Server) GetManifest(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

// GetDescriptors implements PluginService: the verifier port file and every file it imports.
func (s *Server) GetDescriptors(context.Context, *pluginv1.GetDescriptorsRequest) (*pluginv1.GetDescriptorsResponse, error) {
	set, err := plugin.DescriptorSet(verifierv1.File_numinance_ports_verifier_v1_verifier_proto)
	if err != nil {
		return nil, status.Error(codes.Internal, "descriptors")
	}
	return &pluginv1.GetDescriptorsResponse{DescriptorSet: set}, nil
}

// Health implements PluginService.
func (s *Server) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return &pluginv1.HealthResponse{Status: pluginv1.HealthStatus_HEALTH_STATUS_SERVING}, nil
}

// Shutdown implements PluginService: it answers, then stops the process.
func (s *Server) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	if s.stop != nil {
		go s.stop()
	}
	return &pluginv1.ShutdownResponse{}, nil
}

// Invoke implements PluginService: one terminal message per call, SUCCEEDED with the findings or FAILED with one reason.
func (s *Server) Invoke(call *pluginv1.InvokeRequest, stream grpc.ServerStreamingServer[pluginv1.InvokeResponse]) error {
	env := call.GetEnvelope()
	inv := env.GetContext().GetInvocationId()
	if inv == "" || env.GetContext().GetPolicyDecisionId() == "" {
		return status.Error(codes.InvalidArgument, "no host-assigned context")
	}
	if env.GetMethod() != runMethod {
		return status.Error(codes.Unimplemented, "method")
	}
	var req verifierv1.RunRequest
	if env.GetRequest().GetTypeUrl() != typePrefix+string(req.ProtoReflect().Descriptor().FullName()) || proto.Unmarshal(env.GetRequest().GetValue(), &req) != nil || len(req.ProtoReflect().GetUnknown()) > 0 {
		return stream.Send(failed(pluginv1.Reason_REASON_INVALID_ARGUMENT))
	}
	resp, evidence, err := s.Run(stream.Context(), inv, &req)
	var res *pluginv1.ActionResult
	var failure *CallError
	switch {
	case err == nil:
		packed, merr := anypb.New(resp)
		if merr != nil {
			return status.Error(codes.Internal, "marshal")
		}
		res = &pluginv1.ActionResult{Status: pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED, Result: packed, EvidenceIds: evidence}
	case errors.As(err, &failure):
		res = failed(failure.Reason).GetResult()
	default:
		return err
	}
	return stream.Send(&pluginv1.InvokeResponse{Result: res})
}

// CallError is a call that ends FAILED with one reason of the closed vocabulary.
type CallError struct{ Reason pluginv1.Reason }

// Error implements error.
func (e *CallError) Error() string { return "the call failed: " + e.Reason.String() }

func fail(r pluginv1.Reason) error { return &CallError{Reason: r} }

func failed(r pluginv1.Reason) *pluginv1.InvokeResponse {
	return &pluginv1.InvokeResponse{Result: &pluginv1.ActionResult{Status: pluginv1.InvokeStatus_INVOKE_STATUS_FAILED, Diagnostics: []*pluginv1.Diagnostic{{Reason: r, Severity: pluginv1.Severity_SEVERITY_ERROR}}}}
}

func validRef(r *typesv1.Reference) bool {
	return r != nil && r.GetArtifactRef() != "" && len(r.GetArtifactRef()) <= 1024 && digestRe.MatchString(r.GetSha256().GetValue())
}

// Run is one Run: it resolves the config, runs the check, parses its output and builds the response and the ids of the
// evidence it emitted. A failure is a *CallError.
func (s *Server) Run(ctx context.Context, inv string, req *verifierv1.RunRequest) (*verifierv1.RunResponse, []string, error) {
	if !validRef(req.GetCandidateRef()) || !validRef(req.GetConfigRef()) {
		return nil, nil, fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	raw, err := s.host.Resolve(ctx, inv, req.GetConfigRef(), maxConfigBytes)
	if err != nil {
		return nil, nil, resolveFailure(err)
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		return nil, nil, fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	if !Known(cfg.Check) {
		return nil, nil, fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
	}
	cand := req.GetCandidateRef()
	var fetchErr error
	out, err := s.runner.Run(ctx, Call{Config: cfg, CandidateDigest: cand.GetSha256().GetValue(), Fetch: func(ctx context.Context) ([]byte, error) {
		b, ferr := s.host.Resolve(ctx, inv, cand, maxCandidateBytes)
		fetchErr = ferr
		if ferr == nil {
			// The host verified the digest before it streamed; the provider checks again what it is about to run on.
			sum := sha256.Sum256(b)
			if "sha256:"+hex.EncodeToString(sum[:]) != cand.GetSha256().GetValue() {
				return nil, fmt.Errorf("%w: the content is not the digest the reference names", ErrCandidate)
			}
		}
		return b, ferr
	}})
	if err != nil {
		if fetchErr != nil {
			return nil, nil, resolveFailure(fetchErr)
		}
		return nil, nil, runnerFailure(ctx, err)
	}
	if !okExit(cfg.Check, out.ExitCode) {
		return nil, nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR)
	}
	found, err := parse(cfg.Check, out.Dir, out.Stdout)
	if err != nil {
		return nil, nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR)
	}
	// A tool that exits non-zero with nothing to show for it crashed: silence is not a pass.
	if out.ExitCode != 0 && len(found) == 0 {
		return nil, nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR)
	}
	for _, d := range out.Directives {
		p, _ := cleanPath(out.Dir, d.Path)
		found = append(found, finding{
			rule: ruleToken("numinance.directive", d.Name), severity: directiveSeverity(d.Name), path: p, start: uintOf(d.Line), end: uintOf(d.Line),
			message: fmt.Sprintf("the candidate carries a %s directive, which %s cannot be told to ignore", d.Name, cfg.Check),
		})
	}
	resp := buildResponse(cfg.Check, found)
	var ids []string
	if id, err := s.host.Emit(ctx, inv, "report", RecordType, report(cfg.Check, out, resp)); err == nil {
		ids = append(ids, id)
	}
	return resp, ids, nil
}

func resolveFailure(err error) error {
	switch {
	case errors.Is(err, errOverLimit):
		return fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	case notFound(err):
		return fail(pluginv1.Reason_REASON_NOT_FOUND)
	}
	return fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
}

func runnerFailure(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, ErrUnavailable):
		return fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
	case errors.Is(err, ErrTooLarge):
		return fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	case errors.Is(err, ErrCandidate):
		return fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil:
		return fail(pluginv1.Reason_REASON_TIMEOUT)
	}
	return fail(pluginv1.Reason_REASON_UPSTREAM_ERROR)
}

// report is the VerifierReport payload: a receipt of what the tool said, never the findings.
func report(check string, out Output, resp *verifierv1.RunResponse) *structpb.Struct {
	var total float64
	for _, c := range resp.GetSummary().GetCounts() {
		total += float64(c.GetCount())
	}
	sum := sha256.Sum256(out.Stdout)
	return &structpb.Struct{Fields: map[string]*structpb.Value{
		"check":         structpb.NewStringValue(check),
		"exit_code":     structpb.NewNumberValue(float64(out.ExitCode)),
		"output_bytes":  structpb.NewNumberValue(float64(len(out.Stdout))),
		"output_sha256": structpb.NewStringValue("sha256:" + hex.EncodeToString(sum[:])),
		"findings":      structpb.NewNumberValue(total),
		"replayed":      structpb.NewBoolValue(out.Replayed),
	}}
}

// directiveSeverity is MEDIUM for what hides a violation, and LOW for a .gitignore, which every repository has and no tool here reads.
func directiveSeverity(name string) typesv1.FindingSeverity {
	if name == "gitignore" {
		return sevLow
	}
	return sevMedium
}
