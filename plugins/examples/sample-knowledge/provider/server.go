package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	knowledgev1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/knowledge/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// SpecVersion is the spec version the sample was built against.
const SpecVersion = "1.0.0"

const (
	pluginID   = "dev.numinance.sample-knowledge"
	portName   = "numinance.ports.knowledge.v1"
	typePrefix = "type.googleapis.com/"
	svc        = "/numinance.ports.knowledge.v1.KnowledgeService/"
)

// Server is the PluginService of the sample provider.
type Server struct {
	pluginv1.UnimplementedPluginServiceServer
	source   *Source
	manifest *pluginv1.Manifest
	stop     func()
}

// New returns the server. digest is the content digest the manifest declares; main passes the digest of its own
// executable, so the declared digest is a real one. stop ends the process after Shutdown.
func New(digest string, stop func()) *Server {
	ref := func(key string) *typesv1.Reference {
		sum := sha256.Sum256([]byte("sample:" + key))
		return &typesv1.Reference{ArtifactRef: key, Sha256: &typesv1.Digest{Value: "sha256:" + hex.EncodeToString(sum[:])}, ArtifactType: "sample"}
	}
	capability := func(verb string) *pluginv1.CapabilityDescriptor {
		return &pluginv1.CapabilityDescriptor{
			Name: "knowledge." + verb, RiskFloor: annotationsv1.RiskClass_RISK_CLASS_READ, SideEffect: pluginv1.SideEffect_SIDE_EFFECT_READ,
			Idempotency: pluginv1.Idempotency_IDEMPOTENCY_NATURAL, EnvironmentScope: []string{"dev", "staging"},
			ConformanceSuite: "conformance/knowledge/v1/basic", LlmDescription: "Reads documents of a small in-memory sample source.",
			Timeout: durationpb.New(10_000_000_000),
		}
	}
	return &Server{source: NewSource(), stop: stop, manifest: &pluginv1.Manifest{
		PluginId: pluginID, Version: "1.0.0", Digest: &typesv1.Digest{Value: digest}, SignerIdentity: "https://numinance.dev/samples|sample-knowledge",
		Kind: pluginv1.ExtensionClass_EXTENSION_CLASS_CAPABILITY_PROVIDER, Ports: []string{portName},
		Capabilities: []*pluginv1.CapabilityDescriptor{capability("search"), capability("fetch"), capability("resolve")},
		SbomRef:      ref("sbom/sample-knowledge"), ProvenanceRef: ref("provenance/sample-knowledge"),
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

// Register registers the PluginService on s.
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

// GetDescriptors implements PluginService: the knowledge port file and every file it imports.
func (s *Server) GetDescriptors(context.Context, *pluginv1.GetDescriptorsRequest) (*pluginv1.GetDescriptorsResponse, error) {
	set, err := plugin.DescriptorSet(knowledgev1.File_numinance_ports_knowledge_v1_knowledge_proto)
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

// Invoke implements PluginService: one terminal message per call, SUCCEEDED with the typed result or FAILED with one
// diagnostic reason.
func (s *Server) Invoke(req *pluginv1.InvokeRequest, stream grpc.ServerStreamingServer[pluginv1.InvokeResponse]) error {
	env := req.GetEnvelope()
	if env.GetContext().GetInvocationId() == "" || env.GetContext().GetPolicyDecisionId() == "" {
		return status.Error(codes.InvalidArgument, "no host-assigned context")
	}
	resp, err := s.dispatch(env)
	var res *pluginv1.ActionResult
	var failure *CallError
	switch {
	case err == nil:
		packed, merr := anypb.New(resp)
		if merr != nil {
			return status.Error(codes.Internal, "marshal")
		}
		res = &pluginv1.ActionResult{Status: pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED, Result: packed}
	case errors.As(err, &failure):
		res = &pluginv1.ActionResult{Status: pluginv1.InvokeStatus_INVOKE_STATUS_FAILED, Diagnostics: []*pluginv1.Diagnostic{{Reason: failure.Reason, Severity: pluginv1.Severity_SEVERITY_ERROR}}}
	default:
		return err
	}
	return stream.Send(&pluginv1.InvokeResponse{Result: res})
}

func (s *Server) dispatch(env *pluginv1.Envelope) (proto.Message, error) {
	in := func(m proto.Message) error {
		if env.GetRequest().GetTypeUrl() != typePrefix+string(m.ProtoReflect().Descriptor().FullName()) {
			return status.Error(codes.InvalidArgument, "request type")
		}
		return proto.Unmarshal(env.GetRequest().GetValue(), m)
	}
	switch env.GetMethod() {
	case svc + "Search":
		var r knowledgev1.SearchRequest
		if err := in(&r); err != nil {
			return nil, err
		}
		return s.source.Search(&r)
	case svc + "Fetch":
		var r knowledgev1.FetchRequest
		if err := in(&r); err != nil {
			return nil, err
		}
		return s.source.Fetch(&r)
	case svc + "Resolve":
		var r knowledgev1.ResolveRequest
		if err := in(&r); err != nil {
			return nil, err
		}
		return s.source.Resolve(&r)
	}
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("method %d bytes", len(env.GetMethod())))
}
