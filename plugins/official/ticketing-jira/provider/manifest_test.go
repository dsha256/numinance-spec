package provider_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/replay"
)

func snake(name string) string {
	var b strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// The manifest must satisfy the rules the plugin host applies at handshake (pluginhost/domain/manifest.go): an entry for every
// RPC of the port, a risk floor at or above the action's, a side effect of WRITE or more when the action is above READ, the
// idempotency class that the annotation allows, dry run only where the RPC supports it, the compensation named by the
// annotation, the declared evidence, and a conformance suite under conformance/ticketing/v1.
func TestManifest_SatisfiesTheHostsRulesForThePort(t *testing.T) {
	srv, err := provider.New(provider.ReplayPluginID, replay.Host, "sha256:0", provider.Transport{RT: http.DefaultTransport, Replay: true}, &recordingHost{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.GetManifest(t.Context(), &pluginv1.GetManifestRequest{})
	if err != nil {
		t.Fatal(err)
	}
	m := resp.GetManifest()
	if m.GetPorts()[0] != "numinance.ports.ticketing.v1" || len(m.GetPorts()) != 1 {
		t.Errorf("ports = %v, want the ticketing port only", m.GetPorts())
	}
	byName := map[string]*pluginv1.CapabilityDescriptor{}
	for _, c := range m.GetCapabilities() {
		byName[c.GetName()] = c
	}
	methods := ticketingv1.File_numinance_ports_ticketing_v1_ticketing_proto.Services().ByName("TicketingService").Methods()
	if len(byName) != methods.Len() {
		t.Errorf("the manifest has %d capabilities for %d RPCs, want one each", len(byName), methods.Len())
	}
	for i := range methods.Len() {
		md := methods.Get(i)
		a, _ := proto.GetExtension(md.Options(), annotationsv1.E_Action).(*annotationsv1.ActionOptions)
		name := "ticketing." + snake(string(md.Name()))
		c, ok := byName[name]
		if !ok {
			t.Errorf("no capability %s: the host refuses a provider that leaves an RPC of its port without an entry", name)
			continue
		}
		if c.GetRiskFloor() < a.GetRisk() {
			t.Errorf("%s: risk floor %v is below the action risk %v", name, c.GetRiskFloor(), a.GetRisk())
		}
		if a.GetRisk() > annotationsv1.RiskClass_RISK_CLASS_READ && c.GetSideEffect() < pluginv1.SideEffect_SIDE_EFFECT_WRITE {
			t.Errorf("%s: side effect %v is below WRITE for a risk above READ", name, c.GetSideEffect())
		}
		if a.GetIdempotent() == (c.GetIdempotency() == pluginv1.Idempotency_IDEMPOTENCY_NONE) {
			t.Errorf("%s: idempotency %v does not fit an idempotent=%v RPC", name, c.GetIdempotency(), a.GetIdempotent())
		}
		if c.GetDryRun() != a.GetSupportsDryRun() {
			t.Errorf("%s: dry_run %v, want %v", name, c.GetDryRun(), a.GetSupportsDryRun())
		}
		if want := compensationName(md); c.GetCompensation() != want {
			t.Errorf("%s: compensation %q, want %q", name, c.GetCompensation(), want)
		}
		for _, e := range a.GetEvidence() {
			if !slices.Contains(c.GetEvidenceDeclared(), e) {
				t.Errorf("%s: evidence %q is not declared", name, e)
			}
		}
		if c.GetSideEffect() > pluginv1.SideEffect_SIDE_EFFECT_READ && len(c.GetEvidenceDeclared()) == 0 {
			t.Errorf("%s: a side-effecting capability declares no evidence", name)
		}
		if !strings.HasPrefix(c.GetConformanceSuite(), "conformance/ticketing/v1/") {
			t.Errorf("%s: conformance suite %q is not under conformance/ticketing/v1", name, c.GetConformanceSuite())
		}
		if c.GetLlmDescription() == "" || len(c.GetEnvironmentScope()) == 0 {
			t.Errorf("%s: no description or an empty environment scope (which the host rejects)", name)
		}
	}
	// ADR 0060 decision 3: the risk classes the port declares are the ones the manifest floors.
	want := map[string]annotationsv1.RiskClass{
		"ticketing.get_issue": annotationsv1.RiskClass_RISK_CLASS_READ, "ticketing.search_issues": annotationsv1.RiskClass_RISK_CLASS_READ,
		"ticketing.comment_on_issue": annotationsv1.RiskClass_RISK_CLASS_LOW, "ticketing.transition_issue": annotationsv1.RiskClass_RISK_CLASS_MEDIUM,
		"ticketing.link_issue": annotationsv1.RiskClass_RISK_CLASS_LOW,
	}
	for name, risk := range want {
		if byName[name].GetRiskFloor() != risk {
			t.Errorf("%s floors %v, want %v", name, byName[name].GetRiskFloor(), risk)
		}
	}
	if err := ticketingkit.CheckPort(ticketingv1.File_numinance_ports_ticketing_v1_ticketing_proto); err != nil {
		t.Error(err)
	}
}

// compensationName is the capability of the compensating RPC named by an annotation of the form "Service.Method".
func compensationName(md protoreflect.MethodDescriptor) string {
	a, _ := proto.GetExtension(md.Options(), annotationsv1.E_Action).(*annotationsv1.ActionOptions)
	if a.GetCompensatingAction() == "" {
		return ""
	}
	_, rpcName, _ := strings.Cut(a.GetCompensatingAction(), ".")
	return "ticketing." + snake(rpcName)
}
