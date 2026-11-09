// Package provider is the ciexecutor provider for Jenkins: a capability provider of numinance.ports.ciexecutor.v1 over the
// Jenkins remote access API. It is written against the public SDK only. It holds no credential: it asks the host for a handle
// and sends it to the egress proxy, which injects the user name and API token. It starts only a job the customer already
// defines, and everything Jenkins answers is untrusted data, size-limited before it is decoded and checked against the grammar
// of ciexecutor.proto.
package provider

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

const (
	// PluginID is the extension id of the provider that talks to Jenkins through the egress proxy. The `numi init --ci
	// jenkins` template writes this id (v0.2.0 wrote the invalid placeholder dev.numinance.official/jenkins).
	PluginID = "dev.numinance.ciexecutor-jenkins"
	// ReplayPluginID is the extension id of the replaying build.
	ReplayPluginID = "dev.numinance.ciexecutor-jenkins-replay"
	// Intent is the credential intent the manifest declares for every capability; the broker catalog resolves it to the Jenkins
	// user name and API token.
	Intent = "ci.jenkins"
	// RecordType is the evidence record type StartOperation declares.
	RecordType = "ci-run-ref"
	service    = "/numinance.ports.ciexecutor.v1.CiExecutorService/"
	// ciAction is the record type a side-effecting capability with no receipt of its own declares (see the scm provider).
	ciAction = "ci-action"
)

func typed[Req, Resp any, PReq interface {
	*Req
	proto.Message
}, PResp interface {
	*Resp
	proto.Message
}](f func(context.Context, *ghprovider.Call, PReq) (PResp, error)) func(context.Context, *ghprovider.Call, proto.Message) (proto.Message, error) {
	return func(ctx context.Context, c *ghprovider.Call, m proto.Message) (proto.Message, error) {
		resp, err := f(ctx, c, m.(PReq)) //nolint:forcetypeassert // New builds exactly this request type
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
}

func method[Req, Resp any, PReq interface {
	*Req
	proto.Message
}, PResp interface {
	*Resp
	proto.Message
}](name, rpc string, capability ghprovider.Capability, f func(context.Context, *ghprovider.Call, PReq) (PResp, error)) ghprovider.Method {
	capability.Name = name
	return ghprovider.Method{Path: service + rpc, Cap: capability, New: func() proto.Message { return PReq(new(Req)) }, Do: typed[Req, Resp](f)}
}

// NewSpec returns the provider's spec for the Jenkins server cfg names over the given transport. poll is how long CancelOperation
// waits between looks at a build it asked Jenkins to stop; a replay passes zero.
func NewSpec(id string, cfg Config, t ghprovider.Transport, poll time.Duration) ghprovider.Spec {
	p := impl{cfg: cfg, poll: poll, seen: new(queueIndex)}
	natural, keyed := pluginv1.Idempotency_IDEMPOTENCY_NATURAL, pluginv1.Idempotency_IDEMPOTENCY_KEYED
	ro := func(llm string) ghprovider.Capability {
		return ghprovider.Capability{Risk: annotationsv1.RiskClass_RISK_CLASS_READ, SideEffect: pluginv1.SideEffect_SIDE_EFFECT_READ, Idempotency: natural, LLM: llm}
	}
	return ghprovider.Spec{
		ID: id, Version: "1.0.0", Port: "numinance.ports.ciexecutor.v1", File: civ1.File_numinance_ports_ciexecutor_v1_ciexecutor_proto,
		Intent: Intent, NetworkHost: cfg.Host, NetworkPort: cfg.Port, Suite: "conformance/ciexecutor/v1/basic", Transport: t, Timeout: 120 * time.Second,
		Methods: []ghprovider.Method{
			method("ciexecutor.start_operation", "StartOperation", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_HIGH, SideEffect: pluginv1.SideEffect_SIDE_EFFECT_DEPLOY, Idempotency: keyed, DryRun: true,
				Compensation: "ciexecutor.cancel_operation", Evidence: []string{RecordType}, LLM: "Starts one run of a customer pipeline at a given commit.",
			}, p.start),
			method("ciexecutor.get_operation", "GetOperation", ro("Returns the state of one pipeline run."), p.get),
			method("ciexecutor.cancel_operation", "CancelOperation", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_MEDIUM, SideEffect: pluginv1.SideEffect_SIDE_EFFECT_WRITE, Idempotency: keyed,
				Evidence: []string{ciAction}, LLM: "Cancels a pipeline run that has not ended.",
			}, p.cancel),
			method("ciexecutor.list_artifacts", "ListArtifacts", ro("Lists the artifacts a pipeline run produced."), p.artifacts),
			method("ciexecutor.get_log_reference", "GetLogReference", ro("Returns the address of the log of a pipeline run."), p.logReference),
		},
	}
}

// New returns the server of the provider.
func New(id, digest string, cfg Config, t ghprovider.Transport, poll time.Duration, host ghprovider.Host, stop func()) *ghprovider.Server {
	return ghprovider.New(NewSpec(id, cfg, t, poll), digest, host, stop)
}

// The types of the shared skeleton a caller of this package names (see the GitHub providers).
type (
	// Server is the PluginService of the provider.
	Server = ghprovider.Server
	// Host is what a call needs of the HostService of its launch.
	Host = ghprovider.Host
	// Transport says how the provider reaches Jenkins.
	Transport = ghprovider.Transport
	// CallError is a call that ends FAILED with one reason.
	CallError = ghprovider.CallError
)

// ExecutableDigest returns "sha256:" plus the hex digest of the running executable.
func ExecutableDigest() (string, error) { return ghprovider.ExecutableDigest() }
