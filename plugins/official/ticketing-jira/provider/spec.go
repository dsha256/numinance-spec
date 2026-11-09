// Package provider is the ticketing provider for Jira Cloud: a capability provider of numinance.ports.ticketing.v1 over the Jira
// Cloud platform REST API, version 3. It is written against the public SDK only. Everything the tracker answers is untrusted
// data: size-limited before it is decoded, rendered to plain text, stripped of control, bidirectional and zero-width characters
// and bounded to the port's limits. A ticket never makes the provider fetch anything: it builds every URL from the one instance
// host it was configured with and never follows a redirect.
//
// A replay build reads recorded exchanges and holds no credential. A live build (cmd/live) sends every request through the
// egress proxy with a credential handle from the broker; it holds no token and declares one network destination, its instance
// host on port 443 (decisions 2026-11-09/ticketing-jira-network-intent and 2026-11-09/ticketing-jira-credentials).
package provider

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

const (
	// PluginID is the extension id of the provider that talks to a Jira Cloud instance through the egress proxy.
	PluginID = "dev.numinance.ticketing-jira"
	// ReplayPluginID is the extension id of the replaying build: the same provider over recorded, redacted exchanges.
	ReplayPluginID = "dev.numinance.ticketing-jira-replay"
	// Intent is the credential intent a live manifest declares for every capability; the broker catalog resolves it to the Jira credential (decisions 2026-11-09/ticketing-jira-credentials).
	Intent = "ticketing.jira.api"
	// RecordType is the evidence record type every write declares.
	RecordType = "ticketing-action"
	// Version is the provider's own x.y.z.
	Version = "0.1.0"
	service = "/numinance.ports.ticketing.v1.TicketingService/"
)

var hostLabelRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// internalTLDs are final labels that name no public tracker. The grammar below already refuses an IP literal in any spelling (the
// last label starts with a letter), a wildcard, a port and a single label; these are the names of the local and private spaces.
var internalTLDs = //nolint:gochecknoglobals // read-only table
[]string{"localhost", "local", "internal", "localdomain", "lan", "intranet", "home", "corp", "arpa"}

// ValidHost checks the instance host of a configuration: a lower-case DNS name of at least two labels, no wildcard, no port, no
// IP literal, no name of a local or private space. It is the exact host the manifest declares, on port 443. A host that
// resolves to a loopback, link-local or private address is the network layer's to refuse (ADR 0060 ruling OQ2); this is the
// grammar that keeps a configuration from spelling one.
func ValidHost(h string) error {
	bad := errors.New("provider: the instance host is not an exact lower-case DNS name of two or more labels")
	if len(h) < 4 || len(h) > 253 {
		return bad
	}
	labels := strings.Split(h, ".")
	if len(labels) < 2 {
		return bad
	}
	for _, l := range labels {
		if !hostLabelRe.MatchString(l) {
			return bad
		}
	}
	last := labels[len(labels)-1]
	if last[0] < 'a' || last[0] > 'z' || len(last) < 2 {
		return bad
	}
	for _, t := range internalTLDs {
		if last == t {
			return bad
		}
	}
	return nil
}

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

// NewSpec returns the provider's spec for the instance at host over the given transport. The host is the one network
// destination of a live manifest (port 443, no pattern); a replay manifest declares none.
func NewSpec(id, host string, t ghprovider.Transport) (ghprovider.Spec, error) {
	if err := ValidHost(host); err != nil {
		return ghprovider.Spec{}, err
	}
	natural, keyed := pluginv1.Idempotency_IDEMPOTENCY_NATURAL, pluginv1.Idempotency_IDEMPOTENCY_KEYED
	read, write := pluginv1.SideEffect_SIDE_EFFECT_READ, pluginv1.SideEffect_SIDE_EFFECT_WRITE
	p := &jira{host: host}
	ro := func(llm string) ghprovider.Capability {
		return ghprovider.Capability{Risk: annotationsv1.RiskClass_RISK_CLASS_READ, SideEffect: read, Idempotency: natural, LLM: llm}
	}
	return ghprovider.Spec{
		ID: id, Version: Version, Port: "numinance.ports.ticketing.v1", File: ticketingv1.File_numinance_ports_ticketing_v1_ticketing_proto,
		Intent: Intent, NetworkHost: host, Suite: "conformance/ticketing/v1/basic", Transport: t, Timeout: 60 * time.Second,
		Methods: []ghprovider.Method{
			method("ticketing.get_issue", "GetIssue", ro("Returns one issue of a project: key, title, plain-text body, state category, type, labels and recorded operation ids."), p.getIssue),
			method("ticketing.search_issues", "SearchIssues", ro("Searches the issues of a project for a text, optionally in one state category."), p.searchIssues),
			method("ticketing.comment_on_issue", "CommentOnIssue", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_LOW, SideEffect: write, Idempotency: keyed, Evidence: []string{RecordType},
				LLM: "Adds one plain-text comment to an issue.",
			}, p.commentOnIssue),
			method("ticketing.transition_issue", "TransitionIssue", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_MEDIUM, SideEffect: write, Idempotency: natural, DryRun: true,
				Compensation: "ticketing.transition_issue", Evidence: []string{RecordType},
				LLM: "Moves an issue to a target state category: open, in progress, done or canceled.",
			}, p.transitionIssue),
			method("ticketing.link_issue", "LinkIssue", ghprovider.Capability{
				Risk: annotationsv1.RiskClass_RISK_CLASS_LOW, SideEffect: write, Idempotency: natural, Evidence: []string{RecordType},
				LLM: "Links one issue as blocking another.",
			}, p.linkIssue),
		},
	}, nil
}

// New returns the server of the provider for the instance at host.
func New(id, host, digest string, t ghprovider.Transport, h ghprovider.Host, stop func()) (*ghprovider.Server, error) {
	spec, err := NewSpec(id, host, t)
	if err != nil {
		return nil, err
	}
	return ghprovider.New(spec, digest, h, stop), nil
}

// The types of the shared skeleton a caller of this package names: aliases, so a harness outside plugins/official needs nothing
// of the internal package.
type (
	// Server is the PluginService of the provider.
	Server = ghprovider.Server
	// Host is what a call needs of the HostService of its launch.
	Host = ghprovider.Host
	// Transport says how the provider reaches the tracker.
	Transport = ghprovider.Transport
)

// ExecutableDigest returns "sha256:" plus the hex digest of the running executable.
func ExecutableDigest() (string, error) { return ghprovider.ExecutableDigest() }
