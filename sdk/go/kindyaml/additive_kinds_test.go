package kindyaml_test

import (
	"errors"
	"strings"
	"testing"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

// TestLoader_Decode_TrainingUse pins ResidencyPolicySpec.training_use
// : absent means forbidden, so the examples that omit it still
// decode with the field unset, and a value must be explicit, known and set by
// the org.
func TestLoader_Decode_TrainingUse(t *testing.T) {
	l := newLoader(t)
	for _, name := range []string{"ResidencyPolicy/eu-zdr.yaml", "SDLCProfile/org-baseline.yaml"} {
		t.Run("absent training_use in "+name, func(t *testing.T) {
			m, err := l.Decode([]byte(example(t, name)))
			if err != nil {
				t.Fatalf("Decode(%s) = %v", name, err)
			}
			var spec *kindsv1.ResidencyPolicySpec
			switch k := m.(type) {
			case *kindsv1.ResidencyPolicy:
				spec = k.GetSpec()
			case *kindsv1.SDLCProfile:
				spec = k.GetSpec().GetResidency()
			}
			if spec.TrainingUse != nil {
				t.Errorf("Decode(%s) sets training_use %v, want it absent (absent means forbidden)", name, spec.GetTrainingUse())
			}
			if got := spec.GetTrainingUse(); got == kindsv1.TrainingUse_TRAINING_USE_ALLOWED {
				t.Errorf("absent training_use reads as %v, want not allowed", got)
			}
		})
	}
	m, err := l.Decode([]byte(example(t, "ResidencyPolicy/eu-training-forbidden.yaml")))
	if err != nil {
		t.Fatalf("Decode(eu-training-forbidden) = %v", err)
	}
	if got := m.(*kindsv1.ResidencyPolicy).GetSpec().GetTrainingUse(); got != kindsv1.TrainingUse_TRAINING_USE_FORBIDDEN {
		t.Errorf("training_use = %v, want TRAINING_USE_FORBIDDEN", got)
	}

	doc := example(t, "ResidencyPolicy/eu-training-forbidden.yaml")
	tests := []struct {
		name string
		old  string
		new  string
		want error
	}{
		{name: "allowed at the org", old: "TRAINING_USE_FORBIDDEN", new: "TRAINING_USE_ALLOWED", want: nil},
		{name: "the zero value", old: "TRAINING_USE_FORBIDDEN", new: "TRAINING_USE_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "an unknown value", old: "TRAINING_USE_FORBIDDEN", new: "TRAINING_USE_SOMETIMES", want: kindyaml.ErrSchema},
		{name: "forbidden below the org", old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := strings.Replace(doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(d)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", d, m, err, tt.want)
			}
		})
	}
	prof := example(t, "SDLCProfile/payments-api.yaml")
	inline := strings.Replace(prof, "  playbooks:", "  residency: {training_use: TRAINING_USE_ALLOWED}\n  playbooks:", 1)
	if m, err := l.Decode([]byte(inline)); !errors.Is(err, kindyaml.ErrInvalid) {
		t.Errorf("Decode(inline training_use at component scope) = %v, %v; want error %v", m, err, kindyaml.ErrInvalid)
	}
}

// TestLoader_Decode_RequiredApprovals pins StageGate.required_approvals:
// absent keeps trunk-with-cab
// valid, a present value is at least 1, and the kept gate keys still hold.
func TestLoader_Decode_RequiredApprovals(t *testing.T) {
	l := newLoader(t)
	cab := example(t, "StageRegistry/trunk-with-cab.yaml")
	two := example(t, "StageRegistry/two-person-release.yaml")
	m, err := l.Decode([]byte(two))
	if err != nil {
		t.Fatalf("Decode(two-person-release) = %v", err)
	}
	gate := m.(*kindsv1.StageRegistry).GetSpec().GetStages()[2].GetGate()
	if gate.GetRequiredApprovals() != 2 || gate.GetApproverRole() != "release-managers" {
		t.Errorf("gate = %v, want required_approvals 2 and approver_role release-managers", gate)
	}
	m, err = l.Decode([]byte(cab))
	if err != nil {
		t.Fatalf("Decode(trunk-with-cab) = %v", err)
	}
	for _, s := range m.(*kindsv1.StageRegistry).GetSpec().GetStages() {
		if s.GetGate() != nil && s.GetGate().RequiredApprovals != nil {
			t.Errorf("stage %q sets required_approvals, want the example to omit it", s.GetName())
		}
	}
	tests := []struct {
		name string
		doc  string
		old  string
		new  string
		want error
	}{
		{name: "required_approvals of 0", doc: two, old: "required_approvals: 2", new: "required_approvals: 0", want: kindyaml.ErrInvalid},
		{name: "negative required_approvals", doc: two, old: "required_approvals: 2", new: "required_approvals: -1", want: kindyaml.ErrSchema},
		{name: "fractional required_approvals", doc: two, old: "required_approvals: 2", new: "required_approvals: 1.5", want: kindyaml.ErrSchema},
		{name: "required_approvals as a string", doc: two, old: "required_approvals: 2", new: `required_approvals: "2"`, want: kindyaml.ErrSchema},
		{name: "required_approvals of 1", doc: two, old: "required_approvals: 2", new: "required_approvals: 1", want: nil},
		{name: "gate of the zero kind", doc: two, old: "GATE_KIND_HUMAN", new: "GATE_KIND_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "required_approvals on a plan gate", doc: cab, old: "gate: {kind: GATE_KIND_PLAN}", new: "gate: {kind: GATE_KIND_PLAN, required_approvals: 1}", want: kindyaml.ErrInvalid},
		{name: "required_approvals on a release gate", doc: two, old: "GATE_KIND_HUMAN", new: "GATE_KIND_RELEASE", want: kindyaml.ErrInvalid},
		{name: "required_approvals on a policy gate", doc: two, old: "GATE_KIND_HUMAN", new: "GATE_KIND_POLICY", want: kindyaml.ErrInvalid},
		{name: "a plan gate without required_approvals", doc: cab, old: "gate: {kind: GATE_KIND_PLAN}", new: "gate: {kind: GATE_KIND_POLICY}", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.doc, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			d := strings.Replace(tt.doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(d)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", d, m, err, tt.want)
			}
		})
	}
}

// TestLoader_Decode_KeptConstraints pins the kept constraints: the
// constraints the schema does not repeat still hold.
func TestLoader_Decode_KeptConstraints(t *testing.T) {
	l := newLoader(t)
	env := example(t, "EnvironmentProvider/postgres-clone.yaml")
	know := example(t, "KnowledgeConnection/confluence-eng.yaml")
	res := example(t, "ResidencyPolicy/eu-zdr.yaml")
	cab := example(t, "StageRegistry/trunk-with-cab.yaml")
	tests := []struct {
		name string
		doc  string
		old  string
		new  string
		want error
	}{
		{name: "ttl of 0", doc: env, old: "14400s", new: "0s", want: kindyaml.ErrInvalid},
		{name: "ttl absent", doc: env, old: "  ttl: 14400s\n", new: "", want: kindyaml.ErrInvalid},
		{name: "read_only false", doc: know, old: "read_only: true", new: "read_only: false", want: kindyaml.ErrInvalid},
		{name: "read_only absent", doc: know, old: "  read_only: true\n", new: "", want: kindyaml.ErrSchema},
		{name: "knowledge connection below the org", doc: know, old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "regions empty", doc: res, old: "[eu-central-1]", new: "[]", want: kindyaml.ErrInvalid},
		{name: "regions below the org", doc: res, old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "floors below the org", doc: strings.Replace(res, "  regions: [eu-central-1]\n", "", 1), old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "floors flattened to a scalar", doc: res, old: "  floors:\n    floor: RESIDENCY_CLASS_VENDOR_ZDR\n    rules:", new: "  floors:", want: kindyaml.ErrSchema},
		{name: "approver_role kept", doc: cab, old: "approver_role: change-advisory-board", new: "approver: change-advisory-board", want: kindyaml.ErrSchema},
		{name: "distinct_from kept", doc: cab, old: "sod: {distinct_from:", new: "sod: {different_from:", want: kindyaml.ErrSchema},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.doc, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			d := strings.Replace(tt.doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(d)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", d, m, err, tt.want)
			}
		})
	}
}

// TestLoader_Decode_KnowledgeOAuth pins KnowledgeConnectionSpec.oauth: the
// client id is printable ASCII of 1 to 2048 bytes, a URL one passes the
// endpoint rule, no secret field exists, and only the org sets it.
func TestLoader_Decode_KnowledgeOAuth(t *testing.T) {
	l := newLoader(t)
	doc := example(t, "KnowledgeConnection/confluence-oauth.yaml")
	m, err := l.Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode(confluence-oauth) = %v", err)
	}
	if got := m.(*kindsv1.KnowledgeConnection).GetSpec().GetOauth().GetClientId(); got != "https://numinance.example.com/oauth/client.json" {
		t.Errorf("oauth.client_id = %q, want the example's", got)
	}
	m, err = l.Decode([]byte(example(t, "KnowledgeConnection/confluence-eng.yaml")))
	if err != nil || m.(*kindsv1.KnowledgeConnection).GetSpec().Oauth != nil {
		t.Errorf("Decode(confluence-eng) = %v, %v; want it valid with oauth absent", m, err)
	}
	const id = `"https://numinance.example.com/oauth/client.json"`
	tests := []struct {
		name string
		new  string
		want error
	}{
		{name: "a plain client id", new: "{client_id: abc-123}", want: nil},
		{name: "an empty client id", new: `{client_id: ""}`, want: kindyaml.ErrInvalid},
		{name: "a client id of 2048 bytes", new: "{client_id: " + strings.Repeat("a", 2048) + "}", want: nil},
		{name: "a client id of 2049 bytes", new: "{client_id: " + strings.Repeat("a", 2049) + "}", want: kindyaml.ErrInvalid},
		{name: "whitespace", new: `{client_id: "a b"}`, want: kindyaml.ErrInvalid},
		{name: "a control character", new: `{client_id: "a\u0007b"}`, want: kindyaml.ErrInvalid},
		{name: "a bidi mark", new: `{client_id: "a\u202eb"}`, want: kindyaml.ErrInvalid},
		{name: "a non-ASCII letter", new: `{client_id: "cl\u00efent"}`, want: kindyaml.ErrInvalid},
		{name: "an http URL", new: `{client_id: "http://numinance.example.com/c"}`, want: kindyaml.ErrInvalid},
		{name: "an https URL with a query", new: `{client_id: "https://numinance.example.com/c?x=1"}`, want: kindyaml.ErrInvalid},
		{name: "an https URL with user information", new: `{client_id: "https://user` + `@numinance.example.com/c"}`, want: kindyaml.ErrInvalid},
		{name: "an https URL on an IP literal", new: `{client_id: "https://10.0.0.1/c"}`, want: kindyaml.ErrInvalid},
		{name: "a single slash after the scheme", new: `{client_id: "https:/127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "no slashes after the scheme", new: `{client_id: "https:169.254.169.254/x"}`, want: kindyaml.ErrInvalid},
		{name: "a scheme-relative URL", new: `{client_id: "//169.254.169.254/x"}`, want: kindyaml.ErrInvalid},
		{name: "http with a single slash", new: `{client_id: "http:/evil.example/x"}`, want: kindyaml.ErrInvalid},
		{name: "backslashes after the scheme", new: `{client_id: "https:\\\\127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "an escaped scheme separator", new: `{client_id: "https:%2F%2F127.0.0.1"}`, want: kindyaml.ErrInvalid},
		{name: "a percent sign in a plain id", new: `{client_id: "abc%41"}`, want: kindyaml.ErrInvalid},
		{name: "a plain id with dots and dashes", new: `{client_id: "app.client-1_x/y"}`, want: nil},
		{name: "an underscore in the scheme", new: `{client_id: "a_b://169.254.169.254/x"}`, want: kindyaml.ErrInvalid},
		{name: "a scheme that is an underscore", new: `{client_id: "_://127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "a scheme that is a digit", new: `{client_id: "1://127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "an empty scheme", new: `{client_id: "://127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "a scheme led by a dash", new: `{client_id: "-https://127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "a scheme led by a digit", new: `{client_id: "1https://127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "an underscore inside https", new: `{client_id: "ht_tps://127.0.0.1/x"}`, want: kindyaml.ErrInvalid},
		{name: "user information before the separator", new: `{client_id: "x@y://z"}`, want: kindyaml.ErrInvalid},
		{name: "a lone colon host", new: `{client_id: ":169.254.169.254"}`, want: kindyaml.ErrInvalid},
		{name: "a dot segment", new: `{client_id: "/./x"}`, want: kindyaml.ErrInvalid},
		{name: "a dot-dot id", new: `{client_id: ".."}`, want: kindyaml.ErrInvalid},
		{name: "a dot-dot segment", new: `{client_id: "a/../x"}`, want: kindyaml.ErrInvalid},
		{name: "a secret field", new: "{client_id: abc, client_secret: x}", want: kindyaml.ErrSchema},
		{name: "oauth as a string", new: "abc", want: kindyaml.ErrSchema},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := strings.Replace(doc, "{client_id: "+id+"}", tt.new, 1)
			if m, err := l.Decode([]byte(d)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", d, m, err, tt.want)
			}
		})
	}
	below := strings.Replace(doc, "scope: SCOPE_ORG", "scope: SCOPE_REPO", 1)
	if m, err := l.Decode([]byte(below)); !errors.Is(err, kindyaml.ErrInvalid) {
		t.Errorf("Decode(oauth below the org) = %v, %v; want error %v", m, err, kindyaml.ErrInvalid)
	}
}

// TestLoader_Decode_UnknownGateKind pins that a gate kind the loader does not
// know is refused, never read as a weaker or absent gate (GateKind may grow; a
// consumer denies an unknown kind by default).
func TestLoader_Decode_UnknownGateKind(t *testing.T) {
	l := newLoader(t)
	doc := example(t, "StageRegistry/two-person-release.yaml")
	tests := []struct {
		name string
		new  string
		want error
	}{
		{name: "an unknown name", new: "GATE_KIND_SOMETIMES", want: kindyaml.ErrSchema},
		{name: "the short lowercase spelling", new: "approve", want: kindyaml.ErrSchema},
		{name: "a number", new: "3", want: kindyaml.ErrSchema},
		{name: "a large number", new: "99", want: kindyaml.ErrSchema},
		{name: "a lowercase full name", new: "gate_kind_human", want: kindyaml.ErrSchema},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := strings.Replace(doc, "GATE_KIND_HUMAN", tt.new, 1)
			if m, err := l.Decode([]byte(d)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", d, m, err, tt.want)
			}
		})
	}
}
