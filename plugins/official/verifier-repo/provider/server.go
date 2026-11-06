// Package provider is the verifier provider for repository-level static checks: a capability provider of
// numinance.ports.verifier.v1 that reads the candidate of a change as data and reports what breaks the repository's conventions
// (the scope of the change, its Markdown, the `just` recipes its documents name). It runs no candidate code and no tool: the
// candidate is never extracted or executed, it is parsed in memory. It is written against the public SDK only. What it reads
// (the config and the candidate archive) is untrusted: size-limited, schema-validated and bounded before use, and what it
// reports is a claim the verification context checks again.
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/catalog"
	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// SpecVersion is the spec version the provider was built against.
const SpecVersion = "1.0.0"

const (
	// PluginID is the extension id of the provider.
	PluginID = "dev.numinance.verifier-repo"
	// RecordType is the evidence record type the port declares for Run.
	RecordType = "VerifierReport"
	// Capability is the capability name of the one verb.
	Capability = "verifier.run"
	portName   = "numinance.ports.verifier.v1"
	typePrefix = "type.googleapis.com/"
	runMethod  = "/numinance.ports.verifier.v1.VerifierService/Run"
)

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Server is the PluginService of the provider.
type Server struct {
	pluginv1.UnimplementedPluginServiceServer
	host     Host
	manifest *pluginv1.Manifest
	stop     func()
}

// New returns the server. digest is the content digest the manifest declares (main passes the digest of its own executable),
// host reaches the HostService of the launch, and stop ends the process after Shutdown.
//
// The manifest asks for the least a verifier can: no network intent, no credential intent and no filesystem intent, because
// the provider reads nothing but the two references the host resolves for it and writes nothing.
func New(digest string, host Host, stop func()) *Server {
	ref := func(key string) *typesv1.Reference {
		sum := sha256.Sum256([]byte("verifier-repo:" + key))
		return &typesv1.Reference{ArtifactRef: key, Sha256: &typesv1.Digest{Value: "sha256:" + hex.EncodeToString(sum[:])}, ArtifactType: "sample"}
	}
	return &Server{host: host, stop: stop, manifest: &pluginv1.Manifest{
		PluginId: PluginID, Version: "1.0.0", Digest: &typesv1.Digest{Value: digest}, SignerIdentity: "https://numinance.dev/official|verifier-repo",
		Kind: pluginv1.ExtensionClass_EXTENSION_CLASS_CAPABILITY_PROVIDER, Ports: []string{portName},
		Capabilities: []*pluginv1.CapabilityDescriptor{{
			Name: Capability, RiskFloor: annotationsv1.RiskClass_RISK_CLASS_LOW, SideEffect: pluginv1.SideEffect_SIDE_EFFECT_WRITE,
			Idempotency: pluginv1.Idempotency_IDEMPOTENCY_NATURAL, EnvironmentScope: []string{"dev", "staging", "prod"},
			EvidenceDeclared: []string{RecordType}, ConformanceSuite: "conformance/verifier/v1/basic",
			LlmDescription: "Checks the scope, Markdown and just recipes of a candidate and returns the findings.", Timeout: durationpb.New(60_000_000_000),
		}},
		SbomRef: ref("sbom/verifier-repo"), ProvenanceRef: ref("provenance/verifier-repo"),
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

// Run is one Run: it resolves the config, then the candidate, runs the checks and builds the response and the ids of the evidence
// it emitted. A failure is a *CallError: a request or a config that breaks a bound or a candidate that is not of the accepted
// shape is INVALID_ARGUMENT, an input over a limit LIMIT_EXCEEDED, a reference the host cannot resolve NOT_FOUND, and a config
// naming a check the provider does not have UNAVAILABLE. The candidate is read only after the config is accepted.
func (s *Server) Run(ctx context.Context, inv string, req *verifierv1.RunRequest) (*verifierv1.RunResponse, []string, error) {
	if !validRef(req.GetCandidateRef()) || !validRef(req.GetConfigRef()) {
		return nil, nil, fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	raw, err := s.host.Resolve(ctx, inv, req.GetConfigRef(), MaxConfigBytes)
	if err != nil {
		return nil, nil, resolveFailure(err)
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		return nil, nil, fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	if cfg.Check != catalog.Name {
		return nil, nil, fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
	}
	cand := req.GetCandidateRef()
	archive, err := s.host.Resolve(ctx, inv, cand, MaxCandidateBytes)
	if err != nil {
		return nil, nil, resolveFailure(err)
	}
	// The host verified the digest before it streamed; the provider checks again what it is about to read.
	sum := sha256.Sum256(archive)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if digest != cand.GetSha256().GetValue() {
		return nil, nil, fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	parsed, err := ParseCandidate(archive)
	switch {
	case errors.Is(err, ErrTooLarge):
		return nil, nil, fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	case err != nil:
		return nil, nil, fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	// The context was read at one base and the candidate is a change to another: every link and recipe answer would be about a
	// tree the candidate is not a change to.
	if parsed.Base != cfg.Context.Base {
		return nil, nil, fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, fail(pluginv1.Reason_REASON_TIMEOUT)
	}
	resp := buildResponse(catalog.Name, runChecks(parsed, cfg))
	var ids []string
	if id, err := s.host.Emit(ctx, inv, "report", RecordType, report(len(archive), digest, resp)); err == nil {
		ids = append(ids, id)
	}
	return resp, ids, nil
}

// runChecks runs the checks on a parsed candidate under a validated config and returns the findings, unnormalised.
func runChecks(cand Candidate, cfg Config) []finding {
	out := checkScope(cand, cfg.Scope)
	out = append(out, checkDangling(cand, cfg.Context.Links)...)
	t := newTree(cfg.Context.Paths, cand.Changes)
	rec := parseRecipes(justfile(cand, cfg))
	for _, c := range cand.Changes {
		if c.Status == statusDeleted || c.Symlink || c.Content == nil || !IsMarkdown(c.Path) {
			continue
		}
		out = append(out, checkMarkdown(c.Path, c.Content, t)...)
		out = append(out, checkJustRecipes(c.Path, c.Content, rec)...)
	}
	return out
}

// justfile is the Justfile of the tree the recipes are judged against. A candidate's own Justfile counts only when the scope lets the
// change touch it (an author cannot define the recipe a document names by editing a file the scope keeps out of reach, and a
// protected one never counts); a deleted one leaves none; otherwise the base's.
func justfile(cand Candidate, cfg Config) string {
	for _, c := range cand.Changes {
		if c.Path != justfileName {
			continue
		}
		switch {
		case c.Status == statusDeleted || c.Symlink:
			return ""
		case anyUnder(c.Path, cfg.Scope.AllowPaths, under) && !anyUnder(strings.ToLower(c.Path), lowered(cfg.Scope.ProtectedPaths), underFold):
			return string(c.Content)
		}
	}
	return cfg.Context.Justfile
}

// checkDangling reports a file the change deletes that a Markdown document it does not touch links to: the document is not read
// (the candidate does not carry it), so what it links to is the binding's record of the base tree (Context.Links). A document the
// change modifies is checked directly against the tree it leaves, and one it deletes is gone.
func checkDangling(cand Candidate, links map[string][]string) []finding {
	changed := make(map[string]bool, len(cand.Changes))
	for _, c := range cand.Changes {
		changed[c.Path] = true
	}
	var out []finding
	for _, c := range cand.Changes {
		if c.Status != statusDeleted {
			continue
		}
		from := slices.Clone(links[c.Path])
		slices.Sort(from)
		for _, doc := range from {
			if !changed[doc] {
				out = append(out, finding{rule: catalog.RuleLinkDangling, severity: sevHigh, path: doc, message: "the document links to " + c.Path + ", which this change deletes"})
			}
		}
	}
	return out
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

// report is the VerifierReport payload: a receipt of what the check read, never the findings. exit_code is 0 (no process runs),
// output_bytes and output_sha256 are the size and digest of the candidate archive the check read, findings counts the merged
// findings, and replayed is false: the check ran.
func report(size int, digest string, resp *verifierv1.RunResponse) *structpb.Struct {
	var total float64
	for _, c := range resp.GetSummary().GetCounts() {
		total += float64(c.GetCount())
	}
	return &structpb.Struct{Fields: map[string]*structpb.Value{
		"check":         structpb.NewStringValue(catalog.Name),
		"exit_code":     structpb.NewNumberValue(0),
		"output_bytes":  structpb.NewNumberValue(float64(size)),
		"output_sha256": structpb.NewStringValue(digest),
		"findings":      structpb.NewNumberValue(total),
		"replayed":      structpb.NewBoolValue(false),
	}}
}
