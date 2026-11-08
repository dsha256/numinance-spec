package ghprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// SpecVersion is the spec version the providers were built against.
const SpecVersion = "1.0.0"

const typePrefix = "type.googleapis.com/"

// CallError is a call that ends FAILED with one reason of the closed vocabulary.
type CallError struct{ Reason pluginv1.Reason }

// Error implements error.
func (e *CallError) Error() string { return "the call failed: " + e.Reason.String() }

// Fail returns the error of a call that ends FAILED with reason.
func Fail(r pluginv1.Reason) error { return &CallError{Reason: r} }

// Capability is what the manifest declares of one capability; the host checks it against the port's annotation.
type Capability struct {
	Name         string
	Risk         annotationsv1.RiskClass
	SideEffect   pluginv1.SideEffect
	Idempotency  pluginv1.Idempotency
	DryRun       bool
	Compensation string
	// Evidence is the record types the manifest declares: the descriptor's, and for a side-effecting capability whose
	// descriptor declares none, one the host's rule for side effects asks for.
	Evidence []string
	LLM      string
}

// Method binds one RPC of the port to its handler.
type Method struct {
	// Path is "/<package>.<Service>/<Method>".
	Path string
	Cap  Capability
	// New returns an empty request message.
	New func() proto.Message
	// Do runs the call. It returns the response, or an error that Failure or Fail built.
	Do func(ctx context.Context, c *Call, req proto.Message) (proto.Message, error)
}

// Spec describes one provider.
type Spec struct {
	ID, Version string
	// Port is the proto package of the port, "numinance.ports.scm.v1".
	Port string
	// File is the generated descriptor of the port file.
	File protoreflect.FileDescriptor
	// Intent is the credential intent every capability declares; Host and Suite name the one network destination and the
	// conformance suite.
	Intent, NetworkHost, Suite string
	// NetworkPort is the port of the one network destination; zero means 443.
	NetworkPort uint32
	Methods     []Method
	// Transport is how the provider reaches the code host.
	Transport Transport
	// Timeout is the manifest timeout of every capability.
	Timeout time.Duration
}

// Call is what a handler may use of its invocation.
type Call struct {
	Invocation string
	Context    *pluginv1.ActionContext
	host       Host
	intent     string
	transport  Transport
	evidence   []string
	effects    []*pluginv1.ObservedSideEffect
}

// DryRun reports whether the host asked for no side effects.
func (c *Call) DryRun() bool { return c.Context.GetDryRun() }

// IdempotencyKey is the host's key of the operation.
func (c *Call) IdempotencyKey() string { return c.Context.GetIdempotencyKey() }

// Client returns the REST client of the call: it asks the host for the credential handle of intent first. A host that
// cannot give one fails the call as unavailable; the provider never falls back to an ambient credential.
func (c *Call) Client(ctx context.Context) (Client, error) {
	if c.transport.Replay {
		return Client{Transport: c.transport.RT}, nil
	}
	handle, err := c.host.Credential(ctx, c.Invocation, c.intent)
	if err != nil || handle == "" {
		return Client{}, Fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
	}
	return Client{Transport: c.transport.RT, Handle: handle}, nil
}

// Emit submits the evidence record of the call and remembers its id for the result.
func (c *Call) Emit(ctx context.Context, key, recordType string, payload *structpb.Struct) error {
	id, err := c.host.Emit(ctx, c.Invocation, key, recordType, payload)
	if err != nil {
		return err
	}
	c.evidence = append(c.evidence, id)
	return nil
}

// Wrote records a side effect the call made.
func (c *Call) Wrote(kind pluginv1.SideEffect, target string) {
	c.effects = append(c.effects, &pluginv1.ObservedSideEffect{Kind: kind, Target: target})
}

// Transport says how the provider reaches the code host. The zero value is the live one, the default transport behind the
// egress proxy; a replay build sets RT to the cassette transport and Replay, so no credential handle is asked for.
type Transport struct {
	RT     http.RoundTripper
	Replay bool
}

// Server is the PluginService of a provider.
type Server struct {
	pluginv1.UnimplementedPluginServiceServer
	spec     Spec
	host     Host
	manifest *pluginv1.Manifest
	stop     func()
	methods  map[string]Method
}

// New returns the server. digest is the content digest the manifest declares (main passes the digest of its own
// executable), host reaches the HostService of the launch and stop ends the process after Shutdown.
func New(spec Spec, digest string, host Host, stop func()) *Server {
	ref := func(key string) *typesv1.Reference {
		sum := sha256.Sum256([]byte(spec.ID + ":" + key))
		return &typesv1.Reference{ArtifactRef: key, Sha256: &typesv1.Digest{Value: "sha256:" + hex.EncodeToString(sum[:])}, ArtifactType: "sample"}
	}
	s := &Server{spec: spec, host: host, stop: stop, methods: map[string]Method{}}
	caps := make([]*pluginv1.CapabilityDescriptor, 0, len(spec.Methods))
	// A replay build never opens a connection and never asks for a credential, so its manifest declares no network destination, no
	// credential intent and the dev environment only: it cannot be installed where the live provider runs.
	port := spec.NetworkPort
	if port == 0 {
		port = 443
	}
	intents, network, scope := []string{spec.Intent}, []*pluginv1.NetworkIntent{{Host: spec.NetworkHost, Port: port}}, []string{"dev", "staging", "prod"}
	if spec.Transport.Replay {
		intents, network, scope = nil, nil, []string{"dev"}
	}
	for _, m := range spec.Methods {
		s.methods[m.Path] = m
		caps = append(caps, &pluginv1.CapabilityDescriptor{
			Name: m.Cap.Name, RiskFloor: m.Cap.Risk, SideEffect: m.Cap.SideEffect, Idempotency: m.Cap.Idempotency, DryRun: m.Cap.DryRun,
			Compensation: m.Cap.Compensation, CredentialIntents: intents,
			NetworkIntents:   network,
			EnvironmentScope: scope, EvidenceDeclared: m.Cap.Evidence, ConformanceSuite: spec.Suite,
			LlmDescription: m.Cap.LLM, Timeout: durationpb.New(spec.Timeout),
		})
	}
	s.manifest = &pluginv1.Manifest{
		PluginId: spec.ID, Version: spec.Version, Digest: &typesv1.Digest{Value: digest}, SignerIdentity: "https://numinance.dev/official|" + spec.ID,
		Kind: pluginv1.ExtensionClass_EXTENSION_CLASS_CAPABILITY_PROVIDER, Ports: []string{spec.Port}, Capabilities: caps,
		SbomRef: ref("sbom/" + spec.ID), ProvenanceRef: ref("provenance/" + spec.ID),
	}
	return s
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

// GetDescriptors implements PluginService: the port file and every file it imports.
func (s *Server) GetDescriptors(context.Context, *pluginv1.GetDescriptorsRequest) (*pluginv1.GetDescriptorsResponse, error) {
	set, err := plugin.DescriptorSet(s.spec.File)
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

// Invoke implements PluginService: one terminal message per call, SUCCEEDED with the response or FAILED with one reason.
func (s *Server) Invoke(call *pluginv1.InvokeRequest, stream grpc.ServerStreamingServer[pluginv1.InvokeResponse]) error {
	env := call.GetEnvelope()
	inv := env.GetContext().GetInvocationId()
	if inv == "" || env.GetContext().GetPolicyDecisionId() == "" {
		return status.Error(codes.InvalidArgument, "no host-assigned context")
	}
	m, ok := s.methods[env.GetMethod()]
	if !ok {
		return status.Error(codes.Unimplemented, "method")
	}
	req := m.New()
	if env.GetRequest().GetTypeUrl() != typePrefix+string(req.ProtoReflect().Descriptor().FullName()) ||
		proto.Unmarshal(env.GetRequest().GetValue(), req) != nil || len(req.ProtoReflect().GetUnknown()) > 0 {
		return stream.Send(Failed(pluginv1.Reason_REASON_INVALID_ARGUMENT))
	}
	c := &Call{Invocation: inv, Context: env.GetContext(), host: s.host, intent: s.spec.Intent, transport: s.spec.Transport}
	resp, err := m.Do(stream.Context(), c, req)
	var failure *CallError
	switch {
	case err == nil:
		packed, merr := anypb.New(resp)
		if merr != nil {
			return status.Error(codes.Internal, "marshal")
		}
		return stream.Send(&pluginv1.InvokeResponse{Result: &pluginv1.ActionResult{
			Status: pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED, Result: packed, EvidenceIds: c.evidence, SideEffects: c.effects,
		}})
	case errors.As(err, &failure):
		return stream.Send(Failed(failure.Reason))
	}
	return err
}

// Failed is the terminal message of a call that ends FAILED with one reason.
func Failed(r pluginv1.Reason) *pluginv1.InvokeResponse {
	return &pluginv1.InvokeResponse{Result: &pluginv1.ActionResult{Status: pluginv1.InvokeStatus_INVOKE_STATUS_FAILED, Diagnostics: []*pluginv1.Diagnostic{{Reason: r, Severity: pluginv1.Severity_SEVERITY_ERROR}}}}
}

// Direct runs one method as the host would dispatch it, with no process and no transport: it is what the conformance kit
// and the unit tests drive. The returned error is a *CallError for a call that ends FAILED.
func (s *Server) Direct(ctx context.Context, path string, c *pluginv1.ActionContext, req proto.Message) (proto.Message, []string, error) {
	m, ok := s.methods[path]
	if !ok {
		return nil, nil, Fail(pluginv1.Reason_REASON_INVALID_ARGUMENT)
	}
	call := &Call{Invocation: "inv", Context: c, host: s.host, intent: s.spec.Intent, transport: s.spec.Transport}
	resp, err := m.Do(ctx, call, req)
	return resp, call.evidence, err
}
