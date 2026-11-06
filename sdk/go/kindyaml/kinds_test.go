package kindyaml_test

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

// TestLoader_Encode_DynamicControlPack checks that Encode runs the ControlPack
// checks on a dynamicpb message as it does on the generated type.
func TestLoader_Encode_DynamicControlPack(t *testing.T) {
	l := newLoader(t)
	tests := []struct {
		name string
		edit func(p *kindsv1.ControlPack)
		want error
	}{
		{name: "valid pack is accepted", edit: func(*kindsv1.ControlPack) {}, want: nil},
		{name: "approver group without a human", edit: func(p *kindsv1.ControlPack) {
			p.GetSpec().GetControls()[0].GetApproval().Human = nil
		}, want: kindyaml.ErrInvalid},
		{name: "dual control without a human", edit: func(p *kindsv1.ControlPack) {
			p.GetSpec().GetControls()[1].GetApproval().Human = nil
		}, want: kindyaml.ErrInvalid},
		{name: "export view of an unknown control", edit: func(p *kindsv1.ControlPack) {
			p.GetSpec().GetExportViews()[0].Controls = []string{"req-9.9.9"}
		}, want: kindyaml.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := l.Decode([]byte(example(t, "ControlPack/pci-dss-4.0.1.yaml")))
			if err != nil {
				t.Fatalf("Decode(example) = %v", err)
			}
			p := m.(*kindsv1.ControlPack)
			tt.edit(p)
			b, err := proto.Marshal(p)
			if err != nil {
				t.Fatalf("Marshal(%v) = %v", p, err)
			}
			dyn := dynamicpb.NewMessage(p.ProtoReflect().Descriptor())
			if err := proto.Unmarshal(b, dyn); err != nil {
				t.Fatalf("Unmarshal into dynamicpb = %v", err)
			}
			if out, err := l.Encode(dyn); !errors.Is(err, tt.want) {
				t.Errorf("Encode(dynamic %v) = %q, %v; want error %v", dyn, out, err, tt.want)
			}
		})
	}
}

// example reads one example document of numinance-spec/schemas/examples.
func example(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS("../../../schemas/examples"), name)
	if err != nil {
		t.Fatalf("ReadFile(%s) = %v", name, err)
	}
	return string(b)
}

// spec returns the spec block of an example, which ends the document.
func spec(t *testing.T, doc string) string {
	t.Helper()
	_, after, ok := strings.Cut(doc, "\nspec:")
	if !ok {
		t.Fatal("example has no spec")
	}
	return "spec:" + after
}

// TestLoader_Decode_KindChecks pins the loader checks of StageRegistry,
// Playbook, ArchitecturePack and LanguageToolchain: each case breaks one
// example in one place.
func TestLoader_Decode_KindChecks(t *testing.T) {
	stages := example(t, "StageRegistry/trunk-with-cab.yaml")
	playbook := example(t, "Playbook/dependency-upgrade.yaml")
	pack := example(t, "ArchitecturePack/layered-service.yaml")
	toolchain := example(t, "LanguageToolchain/go-1.27.yaml")
	tests := []struct {
		name string
		doc  string
		old  string
		new  string
		want error
	}{
		{name: "stage registry without stages", doc: stages, old: spec(t, stages), new: "spec: {stages: []}\n", want: kindyaml.ErrInvalid},
		{name: "empty stage name", doc: stages, old: "name: triage,", new: `name: "",`, want: kindyaml.ErrInvalid},
		{name: "stage without maps_to", doc: stages, old: ", maps_to: CHANGE_STATE_CLASSIFIED", new: "", want: kindyaml.ErrSchema},
		{name: "stage mapped to the zero state", doc: stages, old: "CHANGE_STATE_CLASSIFIED", new: "CHANGE_STATE_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "stage mapped to an unknown state", doc: stages, old: "CHANGE_STATE_CLASSIFIED", new: "CHANGE_STATE_DONE", want: kindyaml.ErrSchema},
		{name: "gate of the zero kind", doc: stages, old: "GATE_KIND_PLAN", new: "GATE_KIND_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "gate timeout not in protobuf JSON form", doc: stages, old: "86400s", new: "1d", want: kindyaml.ErrSchema},
		{name: "empty change type", doc: playbook, old: "change_type: dependency-upgrade", new: `change_type: ""`, want: kindyaml.ErrInvalid},
		{name: "playbook without steps", doc: playbook, old: spec(t, playbook), new: "spec: {change_type: x, steps: []}\n", want: kindyaml.ErrInvalid},
		{name: "pack without rules", doc: pack, old: spec(t, pack), new: "spec: {rules: []}\n", want: kindyaml.ErrInvalid},
		{name: "empty step name", doc: playbook, old: "name: bump,", new: `name: "",`, want: kindyaml.ErrInvalid},
		{name: "empty capability set", doc: playbook, old: "capability_set: scm-write", new: `capability_set: ""`, want: kindyaml.ErrInvalid},
		{name: "step of the zero risk", doc: playbook, old: "RISK_CLASS_LOW", new: "RISK_CLASS_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "step without risk", doc: playbook, old: ", risk: RISK_CLASS_LOW", new: "", want: kindyaml.ErrSchema},
		{name: "fractional rate", doc: playbook, old: "max_rework_rate: 1000", new: "max_rework_rate: 0.1", want: kindyaml.ErrSchema},
		{name: "rule without a check", doc: pack, old: "      check: {verifier: lint, rule_id: transport-imports}\n", new: "", want: kindyaml.ErrInvalid},
		{name: "empty rule id", doc: pack, old: "id: no-domain-io", new: `id: ""`, want: kindyaml.ErrInvalid},
		{name: "empty rule statement", doc: pack, old: "statement: The domain layer performs no I/O.", new: `statement: ""`, want: kindyaml.ErrInvalid},
		{name: "rule of the zero severity", doc: pack, old: "FINDING_SEVERITY_HIGH", new: "FINDING_SEVERITY_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "check with an empty verifier", doc: pack, old: "verifier: lint, rule_id", new: `verifier: "", rule_id`, want: kindyaml.ErrInvalid},
		{name: "check with an empty rule id", doc: pack, old: "rule_id: transport-imports", new: `rule_id: ""`, want: kindyaml.ErrInvalid},
		{name: "check without a rule id", doc: pack, old: ", rule_id: transport-imports", new: "", want: kindyaml.ErrSchema},
		{name: "empty language", doc: toolchain, old: "language: go", new: `language: ""`, want: kindyaml.ErrInvalid},
		{name: "empty version", doc: toolchain, old: `version: "1.27"`, new: `version: ""`, want: kindyaml.ErrInvalid},
		{name: "version as a number", doc: toolchain, old: `version: "1.27"`, new: "version: 1.27", want: kindyaml.ErrSchema},
		{name: "toolchain without verifiers", doc: toolchain, old: "  verifiers:\n    - {name: unit-tests, provider: dev.numinance.official/go-test, config: {race: true}}\n    - {name: lint, provider: dev.numinance.official/golangci-lint}\n", new: "  verifiers: []\n", want: kindyaml.ErrInvalid},
		{name: "empty verifier name", doc: toolchain, old: "name: lint,", new: `name: "",`, want: kindyaml.ErrInvalid},
		{name: "empty verifier provider", doc: toolchain, old: "provider: dev.numinance.official/golangci-lint}", new: `provider: ""}`, want: kindyaml.ErrInvalid},
	}
	l := newLoader(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.doc, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			doc := strings.Replace(tt.doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(doc)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", doc, m, err, tt.want)
			}
		})
	}
}

// TestLoader_Decode_PolicyKindChecks pins the loader checks of
// EnvironmentProvider, AdapterSelector, AutonomyPolicy and ResidencyPolicy:
// each case breaks one example in one place.
func TestLoader_Decode_PolicyKindChecks(t *testing.T) {
	env := example(t, "EnvironmentProvider/postgres-clone.yaml")
	adapters := example(t, "AdapterSelector/payments.yaml")
	autonomy := example(t, "AutonomyPolicy/org-ladder.yaml")
	residency := example(t, "ResidencyPolicy/eu-zdr.yaml")
	_, floors, _ := strings.Cut(autonomy, "\n  floors:")
	floors = "  floors:" + floors // the last block of the example
	tests := []struct {
		name string
		doc  string
		old  string
		new  string
		want error
	}{
		{name: "empty environment provider", doc: env, old: "provider: dev.numinance.official/aurora-clone", new: `provider: ""`, want: kindyaml.ErrInvalid},
		{name: "environment of the zero kind", doc: env, old: "ENVIRONMENT_KIND_DATABASE_CLONE", new: "ENVIRONMENT_KIND_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "environment without ttl", doc: env, old: "  ttl: 14400s\n", new: "", want: kindyaml.ErrInvalid},
		{name: "ttl not in protobuf JSON form", doc: env, old: "14400s", new: "4h", want: kindyaml.ErrSchema},
		{name: "tag value not a string", doc: env, old: "{team: payments}", new: "{team: [payments]}", want: kindyaml.ErrSchema},
		{name: "adapter selector without ports", doc: adapters, old: spec(t, adapters), new: "spec: {}\n", want: kindyaml.ErrInvalid},
		{name: "binding without an instance", doc: adapters, old: ", instance: main", new: "", want: kindyaml.ErrSchema},
		{name: "binding with an empty instance", doc: adapters, old: "instance: main", new: `instance: ""`, want: kindyaml.ErrInvalid},
		{name: "binding with an empty provider", doc: adapters, old: "provider: dev.numinance.official/jenkins", new: `provider: ""`, want: kindyaml.ErrInvalid},
		{name: "port the selector does not know", doc: adapters, old: "    scm:", new: "    secrets: {provider: a, instance: b}\n    scm:", want: kindyaml.ErrSchema},
		{name: "default of the zero level", doc: autonomy, old: "{level: AUTONOMY_LEVEL_PR}", new: "{level: AUTONOMY_LEVEL_UNSPECIFIED}", want: kindyaml.ErrInvalid},
		{name: "rule without a match", doc: autonomy, old: "{match: {playbook: dependency-upgrade, risk: RISK_CLASS_LOW}, level", new: "{level", want: kindyaml.ErrInvalid},
		{name: "rule of the zero level", doc: autonomy, old: ", level: AUTONOMY_LEVEL_MERGE", new: ", level: AUTONOMY_LEVEL_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "floor of the zero max_level", doc: autonomy, old: "max_level: AUTONOMY_LEVEL_SUGGEST", new: "max_level: AUTONOMY_LEVEL_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "match of the zero risk", doc: autonomy, old: "risk: RISK_CLASS_HIGH", new: "risk: RISK_CLASS_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "floors without rules", doc: autonomy, old: floors, new: "  floors: {rules: []}\n", want: kindyaml.ErrInvalid},
		{name: "floor without a match", doc: autonomy, old: "{match: {environment: production}, max_level", new: "{max_level", want: kindyaml.ErrInvalid},
		{name: "floor without max_level", doc: autonomy, old: ", max_level: AUTONOMY_LEVEL_MERGE", new: "", want: kindyaml.ErrSchema},
		{name: "floors as a bare list", doc: autonomy, old: "  floors:\n    rules:\n", new: "  floors:\n", want: kindyaml.ErrSchema},
		{name: "residency floor of the zero class", doc: residency, old: "floor: RESIDENCY_CLASS_VENDOR_ZDR", new: "floor: RESIDENCY_CLASS_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "residency floors without a floor", doc: residency, old: "    floor: RESIDENCY_CLASS_VENDOR_ZDR\n", new: "", want: kindyaml.ErrSchema},
		{name: "residency without regions", doc: residency, old: "[eu-central-1]", new: "[]", want: kindyaml.ErrInvalid},
		{name: "residency rule without a match", doc: residency, old: "{match: {component: payments-api}, floor", new: "{floor", want: kindyaml.ErrInvalid},
		{name: "residency rule of the zero floor", doc: residency, old: "floor: RESIDENCY_CLASS_IN_ACCOUNT", new: "floor: RESIDENCY_CLASS_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "residency floors as a bare list", doc: residency, old: "  floors:\n    floor: RESIDENCY_CLASS_VENDOR_ZDR\n    rules:", new: "  floors:", want: kindyaml.ErrSchema},
		{name: "zero ttl", doc: env, old: "14400s", new: "0s", want: kindyaml.ErrInvalid},
		{name: "negative ttl", doc: env, old: "14400s", new: "-5s", want: kindyaml.ErrInvalid},
		{name: "max_concurrent of 0", doc: env, old: "{max_concurrent: 2}", new: "{max_concurrent: 0}", want: kindyaml.ErrInvalid},
		{name: "source as a URL", doc: env, old: "source: payments-primary", new: "source: https://db.example/payments", want: kindyaml.ErrInvalid},
		{name: "source with an at sign", doc: env, old: "source: payments-primary", new: "source: admin@payments-primary", want: kindyaml.ErrInvalid},
		{name: "empty source", doc: env, old: "source: payments-primary", new: `source: ""`, want: kindyaml.ErrInvalid},
		{name: "seed with whitespace", doc: env, old: "[synthetic-customers]", new: `["synthetic customers"]`, want: kindyaml.ErrInvalid},
		{name: "provider with whitespace", doc: env, old: "provider: dev.numinance.official/aurora-clone", new: `provider: "dev.numinance.official/aurora-clone x"`, want: kindyaml.ErrInvalid},
		{name: "instance with a colon", doc: adapters, old: "instance: main", new: "instance: host:8080", want: kindyaml.ErrInvalid},
		{name: "knowledge name with a bidi character", doc: adapters, old: "[confluence-eng, jira-pay]", new: `["confluence-eng\u202e", jira-pay]`, want: kindyaml.ErrInvalid},
		{name: "empty knowledge name", doc: adapters, old: "[confluence-eng, jira-pay]", new: `["", jira-pay]`, want: kindyaml.ErrInvalid},
		{name: "floor match on an empty environment", doc: autonomy, old: "{environment: production}", new: `{environment: ""}`, want: kindyaml.ErrInvalid},
		{name: "rule match on an empty playbook", doc: autonomy, old: "playbook: dependency-upgrade", new: `playbook: ""`, want: kindyaml.ErrInvalid},
		{name: "residency match on an empty component", doc: residency, old: "{component: payments-api}", new: `{component: ""}`, want: kindyaml.ErrInvalid},
		{name: "residency match on an empty environment", doc: residency, old: "{component: payments-api}", new: `{environment: ""}`, want: kindyaml.ErrInvalid},
		{name: "negative retention_max", doc: residency, old: "  cross_region:", new: "  retention_max: -1s\n  cross_region:", want: kindyaml.ErrInvalid},
		{name: "zero retention_max is accepted", doc: residency, old: "  cross_region:", new: "  retention_max: 0s\n  cross_region:", want: nil},
		{name: "negative fractional retention_max", doc: residency, old: "  cross_region:", new: "  retention_max: -0.5s\n  cross_region:", want: kindyaml.ErrInvalid},
		{name: "region wildcard", doc: residency, old: "[eu-central-1]", new: `["*"]`, want: kindyaml.ErrInvalid},
		{name: "autonomy floors at repo scope", doc: autonomy, old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "autonomy rules at repo scope are accepted", doc: strings.Replace(autonomy, floors, "", 1), old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: nil},
		{name: "residency policy at repo scope", doc: residency, old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "region over the name limit", doc: residency, old: "[eu-central-1]", new: "[" + strings.Repeat("a", kindyaml.MaxNameBytes+1) + "]", want: kindyaml.ErrInvalid},
	}
	l := newLoader(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.doc, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			doc := strings.Replace(tt.doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(doc)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", doc, m, err, tt.want)
			}
		})
	}
}

// TestLoader_Decode_KnowledgeConnectionChecks pins the loader checks of
// KnowledgeConnection: each case breaks the
// example in one place.
func TestLoader_Decode_KnowledgeConnectionChecks(t *testing.T) {
	doc := example(t, "KnowledgeConnection/confluence-eng.yaml")
	const hash = `"sha256:3f1a9c0e5b7d2a4f6e8c1b3d5a7f9e2c4b6d8a0f1e3c5b7d9a2f4e6c8b0d1a3f"`
	_, pins, _ := strings.Cut(doc, "\n  pinned_tools:\n")
	pins, _, _ = strings.Cut(pins, "  read_only:")
	pins = "  pinned_tools:\n" + pins // the block up to read_only
	tests := []struct {
		name string
		old  string
		new  string
		want error
	}{
		{name: "read_only false", old: "read_only: true", new: "read_only: false", want: kindyaml.ErrInvalid},
		{name: "read_only absent", old: "  read_only: true\n", new: "", want: kindyaml.ErrSchema},
		{name: "auth absent", old: "  auth: KNOWLEDGE_AUTH_USER_DELEGATED\n", new: "", want: kindyaml.ErrInvalid},
		{name: "repo scope", old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "component scope", old: "scope: SCOPE_ORG", new: "scope: SCOPE_COMPONENT", want: kindyaml.ErrInvalid},
		{name: "auth of the zero value", old: "KNOWLEDGE_AUTH_USER_DELEGATED", new: "KNOWLEDGE_AUTH_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "service account is accepted", old: "KNOWLEDGE_AUTH_USER_DELEGATED", new: "KNOWLEDGE_AUTH_SERVICE_ACCOUNT", want: nil},
		{name: "provider empty", old: "provider: dev.numinance.official/bridge-mcp", new: `provider: ""`, want: kindyaml.ErrInvalid},
		{name: "provider with whitespace", old: "provider: dev.numinance.official/bridge-mcp", new: `provider: "bridge mcp"`, want: kindyaml.ErrInvalid},
		{name: "scope absent", old: "  scope: [ENG, PAY]\n", new: "", want: kindyaml.ErrInvalid},
		{name: "scope empty", old: "[ENG, PAY]", new: "[]", want: kindyaml.ErrInvalid},
		{name: "scope wildcard", old: "[ENG, PAY]", new: `["*"]`, want: kindyaml.ErrInvalid},
		{name: "scope entry empty", old: "[ENG, PAY]", new: `[ENG, ""]`, want: kindyaml.ErrInvalid},
		{name: "scope path is accepted", old: "[ENG, PAY]", new: "[docs/runbooks]", want: nil},
		{name: "scope duplicate", old: "[ENG, PAY]", new: "[ENG, ENG]", want: kindyaml.ErrInvalid},
		{name: "scope root", old: "[ENG, PAY]", new: "[/]", want: kindyaml.ErrInvalid},
		{name: "scope absolute path", old: "[ENG, PAY]", new: "[/etc]", want: kindyaml.ErrInvalid},
		{name: "scope dot", old: "[ENG, PAY]", new: `["."]`, want: kindyaml.ErrInvalid},
		{name: "scope traversal", old: "[ENG, PAY]", new: "[docs/../..]", want: kindyaml.ErrInvalid},
		{name: "scope trailing slash", old: "[ENG, PAY]", new: "[docs/]", want: kindyaml.ErrInvalid},
		{name: "scope backslash", old: "[ENG, PAY]", new: `["a\\b"]`, want: kindyaml.ErrInvalid},
		{name: "scope with a quote", old: "[ENG, PAY]", new: `['ENG"OR"1']`, want: kindyaml.ErrInvalid},
		{name: "scope with a glob", old: "[ENG, PAY]", new: `["docs/?"]`, want: kindyaml.ErrInvalid},
		{name: "scope duplicate in another case", old: "[ENG, PAY]", new: "[ENG, eng]", want: kindyaml.ErrInvalid},
		{name: "scope with a comma", old: "[ENG, PAY]", new: `["ENG,PAY"]`, want: kindyaml.ErrInvalid},
		{name: "scope with a percent escape", old: "[ENG, PAY]", new: "[docs/%2e%2e]", want: kindyaml.ErrInvalid},
		{name: "scope with a query operator", old: "[ENG, PAY]", new: `["ENG)OR(1"]`, want: kindyaml.ErrInvalid},
		{name: "scope with a tilde", old: "[ENG, PAY]", new: "[~root]", want: kindyaml.ErrInvalid},
		{name: "scope with a fullwidth dot", old: "[ENG, PAY]", new: "[\"docs/\uff0e\uff0e\"]", want: kindyaml.ErrInvalid},
		{name: "scope with a Cyrillic letter", old: "[ENG, PAY]", new: "[\"\u0415NG\"]", want: kindyaml.ErrInvalid},
		{name: "scope over the name limit", old: "[ENG, PAY]", new: "[" + strings.Repeat("a", kindyaml.MaxNameBytes+1) + "]", want: kindyaml.ErrInvalid},
		{name: "scope with inner dots is accepted", old: "[ENG, PAY]", new: "[docs/v1.2_notes-a]", want: nil},
		{name: "pinned tool without a hash", old: ", hash: {value: " + hash + "}", new: "", want: kindyaml.ErrInvalid},
		{name: "pinned tool with an empty hash", old: "{value: " + hash + "}", new: "{}", want: kindyaml.ErrSchema},
		{name: "pinned tool with an empty hash value", old: hash, new: `""`, want: kindyaml.ErrInvalid},
		{name: "hash prefix in upper case", old: hash, new: strings.ToUpper(hash), want: kindyaml.ErrInvalid},
		{name: "hash hex in upper case", old: hash, new: `"sha256:` + strings.ToUpper(hash[len(`"sha256:`):]), want: kindyaml.ErrInvalid},
		{name: "hash of another algorithm", old: hash, new: strings.Replace(hash, "sha256:", "sha512:", 1), want: kindyaml.ErrInvalid},
		{name: "hash one character short", old: hash, new: hash[:len(hash)-2] + `"`, want: kindyaml.ErrInvalid},
		{name: "hash with a non-hex character", old: hash, new: hash[:len(hash)-2] + `g"`, want: kindyaml.ErrInvalid},
		{name: "pinned tool without a name", old: "name: searchConfluenceUsingCql, ", new: "", want: kindyaml.ErrSchema},
		{name: "pinned tool with an empty name", old: "name: searchConfluenceUsingCql", new: `name: ""`, want: kindyaml.ErrInvalid},
		{name: "pinned tool with a wildcard name", old: "name: searchConfluenceUsingCql", new: `name: "*"`, want: kindyaml.ErrInvalid},
		{name: "pinned tool name twice", old: "name: getConfluencePage", new: "name: searchConfluenceUsingCql", want: kindyaml.ErrInvalid},
		{name: "no pinned tools is accepted", old: pins, new: "", want: nil},
		{name: "endpoint absent is accepted", old: "  endpoint: https://mcp.example.com/v1/mcp\n", new: "", want: nil},
		{name: "endpoint over http", old: "https://mcp.example.com", new: "http://mcp.example.com", want: kindyaml.ErrInvalid},
		{name: "endpoint with user information", old: "https://mcp.example.com", new: "https://user@mcp.example.com", want: kindyaml.ErrInvalid},
		{name: "endpoint with a query", old: "/v1/mcp", new: "/v1/mcp?token=x", want: kindyaml.ErrInvalid},
		{name: "endpoint with an empty fragment", old: "/v1/mcp", new: `/v1/mcp#`, want: kindyaml.ErrInvalid},
		{name: "endpoint without a host", old: "https://mcp.example.com/v1/mcp", new: "https:///v1/mcp", want: kindyaml.ErrInvalid},
		{name: "endpoint relative", old: "https://mcp.example.com/v1/mcp", new: "/v1/mcp", want: kindyaml.ErrInvalid},
		{name: "endpoint with whitespace", old: "/v1/mcp", new: "/v1 mcp", want: kindyaml.ErrInvalid},
		{name: "endpoint with an upper-case scheme", old: "https://mcp", new: "HTTPS://mcp", want: kindyaml.ErrInvalid},
		{name: "endpoint with an upper-case host", old: "mcp.example.com", new: "MCP.example.com", want: kindyaml.ErrInvalid},
		{name: "endpoint on a link-local address", old: "mcp.example.com", new: "169.254.169.254", want: kindyaml.ErrInvalid},
		{name: "endpoint on an IPv6 literal", old: "mcp.example.com", new: "[::ffff:169.254.169.254]", want: kindyaml.ErrInvalid},
		{name: "endpoint on a numeric host", old: "mcp.example.com", new: "2130706433", want: kindyaml.ErrInvalid},
		{name: "endpoint on a single label", old: "mcp.example.com", new: "localhost", want: kindyaml.ErrInvalid},
		{name: "endpoint with a trailing dot", old: "mcp.example.com", new: "mcp.example.com.", want: kindyaml.ErrInvalid},
		{name: "endpoint with a non-ASCII host", old: "mcp.example.com", new: "mcp.ex\u0430mple.com", want: kindyaml.ErrInvalid},
		{name: "endpoint with port 0", old: "mcp.example.com", new: "mcp.example.com:0", want: kindyaml.ErrInvalid},
		{name: "endpoint with port 65536", old: "mcp.example.com", new: "mcp.example.com:65536", want: kindyaml.ErrInvalid},
		{name: "endpoint with an empty port", old: "mcp.example.com", new: "mcp.example.com:", want: kindyaml.ErrInvalid},
		{name: "endpoint with a leading-zero port", old: "mcp.example.com", new: "mcp.example.com:08443", want: kindyaml.ErrInvalid},
		{name: "endpoint with port 8443 is accepted", old: "mcp.example.com", new: "mcp.example.com:8443", want: nil},
		{name: "endpoint on a .internal host", old: "mcp.example.com", new: "metadata.google.internal", want: kindyaml.ErrInvalid},
		{name: "endpoint on a .localhost host", old: "mcp.example.com", new: "x.localhost", want: kindyaml.ErrInvalid},
		{name: "endpoint on a .local host", old: "mcp.example.com", new: "printer.local", want: kindyaml.ErrInvalid},
		{name: "endpoint with a 64-byte label", old: "mcp.example.com", new: strings.Repeat("a", 64) + ".example.com", want: kindyaml.ErrInvalid},
		{name: "endpoint with a 63-byte label is accepted", old: "mcp.example.com", new: strings.Repeat("a", 63) + ".example.com", want: nil},
		{name: "endpoint host over 253 bytes", old: "mcp.example.com", new: strings.Repeat("a.", 124) + "example.com", want: kindyaml.ErrInvalid},
		{name: "endpoint host with a leading hyphen", old: "mcp.example.com", new: "-mcp.example.com", want: kindyaml.ErrInvalid},
		{name: "endpoint host with a trailing hyphen", old: "mcp.example.com", new: "mcp-.example.com", want: kindyaml.ErrInvalid},
		{name: "endpoint with a dot-dot segment", old: "/v1/mcp", new: "/../admin", want: kindyaml.ErrInvalid},
		{name: "endpoint with a dot segment", old: "/v1/mcp", new: "/v1/./mcp", want: kindyaml.ErrInvalid},
		{name: "endpoint with a backslash", old: "/v1/mcp", new: `/a\b`, want: kindyaml.ErrInvalid},
		{name: "endpoint with a path parameter", old: "/v1/mcp", new: "/v1/mcp;sid=x", want: kindyaml.ErrInvalid},
		{name: "endpoint with a percent escape", old: "/v1/mcp", new: "/v1/%0d%0aX", want: kindyaml.ErrInvalid},
		{name: "endpoint of exactly the limit is accepted", old: "/v1/mcp", new: "/" + strings.Repeat("a", kindyaml.MaxEndpointBytes-len("https://mcp.example.com/")), want: nil},
		{name: "endpoint with a bidi character", old: "https://mcp.example.com/v1/mcp", new: `"https://mcp.example.com/v1/mcp\u202e"`, want: kindyaml.ErrInvalid},
		{name: "endpoint one byte over the limit", old: "/v1/mcp", new: "/" + strings.Repeat("a", kindyaml.MaxEndpointBytes-len("https://mcp.example.com/")+1), want: kindyaml.ErrInvalid},
		{name: "calls_per_stage of 0", old: "calls_per_stage: 8", new: "calls_per_stage: 0", want: kindyaml.ErrInvalid},
		{name: "bytes_per_call of 0", old: "bytes_per_call: 65536", new: "bytes_per_call: 0", want: kindyaml.ErrInvalid},
		{name: "cache_ttl of 0s disables the cache", old: "2592000s", new: "0s", want: nil},
		{name: "negative cache_ttl", old: "2592000s", new: "-1s", want: kindyaml.ErrInvalid},
		{name: "cache_ttl not in protobuf JSON form", old: "2592000s", new: "30d", want: kindyaml.ErrSchema},
	}
	l := newLoader(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(doc, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			d := strings.Replace(doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(d)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", d, m, err, tt.want)
			}
		})
	}
}

// TestLoader_Decode_ProfileKindChecks pins the loader checks of SDLCProfile
// and ControlPack: each case breaks one example
// in one place.
func TestLoader_Decode_ProfileKindChecks(t *testing.T) {
	profile := example(t, "SDLCProfile/payments-api.yaml")
	org := example(t, "SDLCProfile/org-baseline.yaml")
	pack := example(t, "ControlPack/pci-dss-4.0.1.yaml")
	const source = "[PCI DSS v4.0.1 Requirement 6]"
	const applicability = `risk_at_least(change.risk, "MEDIUM")`
	_, floors, _ := strings.Cut(pack, "\n  floors:\n")
	floors, _, _ = strings.Cut(floors, "  export_views:")
	floors = "  floors:\n" + floors // the block up to export_views
	_, controls, _ := strings.Cut(pack, "\n  controls:\n")
	controls, _, _ = strings.Cut(controls, "  floors:")
	controls = "  controls:\n" + controls // the block up to floors
	views := "  export_views:\n    - {name: pci-dss-req6, controls: [req-6.2.3, req-6.5.1]}\n"
	noViews := strings.Replace(pack, views, "", 1) // so a control id case fails for its own rule
	tests := []struct {
		name string
		doc  string
		old  string
		new  string
		want error
	}{
		{name: "empty profile spec is accepted", doc: profile, old: spec(t, profile), new: "spec: {}\n", want: nil},
		{name: "unknown spec key", doc: profile, old: "  toolchains:", new: "  packs: [layered-service]\n  toolchains:", want: kindyaml.ErrSchema},
		{name: "knowledge outside adapters", doc: profile, old: "  adapters:", new: "  knowledge: [confluence-eng]\n  adapters:", want: kindyaml.ErrSchema},
		{name: "playbook twice", doc: profile, old: "bugfix-bounded,", new: "dependency-upgrade,", want: kindyaml.ErrInvalid},
		{name: "empty playbook name", doc: profile, old: "bugfix-bounded,", new: `"",`, want: kindyaml.ErrInvalid},
		{name: "playbook name with a Cyrillic letter", doc: profile, old: "bugfix-bounded,", new: "\"bugfix-b\u043eunded\",", want: kindyaml.ErrInvalid},
		{name: "architecture pack twice", doc: profile, old: "[layered-service]", new: "[layered-service, layered-service]", want: kindyaml.ErrInvalid},
		{name: "architecture pack name with whitespace", doc: profile, old: "[layered-service]", new: `["layered service"]`, want: kindyaml.ErrInvalid},
		{name: "architecture pack name with a wildcard", doc: profile, old: "[layered-service]", new: `["arch/*"]`, want: kindyaml.ErrInvalid},
		{name: "architecture pack name with a dot-dot segment", doc: profile, old: "[layered-service]", new: "[arch/../controls]", want: kindyaml.ErrInvalid},
		{name: "control pack twice", doc: org, old: "[controls/payments/pci-dss/4.0.1]", new: "[controls/payments/pci-dss/4.0.1, controls/payments/pci-dss/4.0.1]", want: kindyaml.ErrInvalid},
		{name: "control pack name with a leading slash", doc: org, old: "[controls/payments/pci-dss/4.0.1]", new: "[/controls/payments]", want: kindyaml.ErrInvalid},
		{name: "control pack added at component scope is accepted", doc: profile, old: "  toolchains:", new: "  control_packs: [controls/payments/pci-dss/4.0.1]\n  toolchains:", want: nil},
		{name: "toolchain twice", doc: profile, old: "[go-1.27]", new: "[go-1.27, go-1.27]", want: kindyaml.ErrInvalid},
		{name: "toolchain name with a bidi character", doc: profile, old: "[go-1.27]", new: `["go-1.27\u202e"]`, want: kindyaml.ErrInvalid},
		{name: "adapter binding with an empty instance", doc: profile, old: "instance: ci-prod", new: `instance: ""`, want: kindyaml.ErrInvalid},
		{name: "empty knowledge name", doc: profile, old: "[confluence-eng, jira-pay]", new: `["", jira-pay]`, want: kindyaml.ErrInvalid},
		{name: "knowledge name with a Cyrillic letter", doc: profile, old: "[confluence-eng, jira-pay]", new: "[\"cоnfluence-eng\", jira-pay]", want: kindyaml.ErrInvalid},
		{name: "adapter instance with a Cyrillic letter", doc: profile, old: "instance: ci-prod", new: "instance: \"ci-prоd\"", want: kindyaml.ErrInvalid},
		{name: "adapter provider with a Cyrillic letter", doc: profile, old: "official/jenkins", new: "official/jеnkins", want: kindyaml.ErrInvalid},
		{name: "empty stages", doc: org, old: "stages: trunk-with-cab", new: `stages: ""`, want: kindyaml.ErrInvalid},
		{name: "stages with a colon", doc: org, old: "stages: trunk-with-cab", new: "stages: a:b", want: kindyaml.ErrInvalid},
		{name: "stages as a list", doc: org, old: "stages: trunk-with-cab", new: "stages: [trunk-with-cab]", want: kindyaml.ErrSchema},
		{name: "stages at component scope", doc: profile, old: "  playbooks:", new: "  stages: trunk-with-cab\n  playbooks:", want: kindyaml.ErrInvalid},
		{name: "budgets at component scope", doc: profile, old: "  playbooks:", new: "  budgets: {tokens: 1}\n  playbooks:", want: kindyaml.ErrInvalid},
		{name: "org profile at repo scope", doc: org, old: "SCOPE_ORG", new: "SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "autonomy floors at component scope", doc: profile, old: "  playbooks:", new: "  autonomy: {floors: {rules: [{match: {}, max_level: AUTONOMY_LEVEL_MERGE}]}}\n  playbooks:", want: kindyaml.ErrInvalid},
		{name: "autonomy rules at component scope are accepted", doc: profile, old: "  playbooks:", new: "  autonomy: {rules: [{match: {risk: RISK_CLASS_LOW}, level: AUTONOMY_LEVEL_MERGE}]}\n  playbooks:", want: nil},
		{name: "residency regions at component scope", doc: profile, old: "  playbooks:", new: "  residency: {regions: [eu-central-1]}\n  playbooks:", want: kindyaml.ErrInvalid},
		{name: "autonomy rule of the zero level", doc: org, old: "level: AUTONOMY_LEVEL_MERGE", new: "level: AUTONOMY_LEVEL_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "autonomy rule without a match", doc: org, old: "{match: {playbook: dependency-upgrade, risk: RISK_CLASS_LOW}, level", new: "{level", want: kindyaml.ErrInvalid},
		{name: "inline autonomy floors without rules", doc: org, old: "    rules:\n      - {match", new: "    floors: {rules: []}\n    rules:\n      - {match", want: kindyaml.ErrInvalid},
		{name: "inline residency without regions", doc: org, old: ", regions: [eu-central-1]", new: "", want: kindyaml.ErrInvalid},
		{name: "inline residency with a flat floor", doc: org, old: "{floors: {floor: RESIDENCY_CLASS_IN_ACCOUNT}, ", new: "{floor: RESIDENCY_CLASS_IN_ACCOUNT, ", want: kindyaml.ErrSchema},
		{name: "inline residency region wildcard", doc: org, old: "[eu-central-1]", new: `["*"]`, want: kindyaml.ErrInvalid},
		{name: "negative token budget", doc: org, old: "tokens: 400000", new: "tokens: -1", want: kindyaml.ErrSchema},
		{name: "wall_clock not in protobuf JSON form", doc: org, old: "7200s", new: "2h", want: kindyaml.ErrSchema},
		{name: "pack at repo scope", doc: pack, old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "pack at component scope", doc: pack, old: "scope: SCOPE_ORG", new: "scope: SCOPE_COMPONENT", want: kindyaml.ErrInvalid},
		{name: "empty jurisdiction", doc: pack, old: "jurisdiction: global", new: `jurisdiction: ""`, want: kindyaml.ErrInvalid},
		{name: "jurisdiction absent", doc: pack, old: "  jurisdiction: global\n", new: "", want: kindyaml.ErrSchema},
		{name: "empty framework", doc: pack, old: "framework: pci-dss", new: `framework: ""`, want: kindyaml.ErrInvalid},
		{name: "framework with whitespace", doc: pack, old: "framework: pci-dss", new: `framework: "pci dss"`, want: kindyaml.ErrInvalid},
		{name: "empty version", doc: pack, old: "version: 4.0.1", new: `version: ""`, want: kindyaml.ErrInvalid},
		{name: "version with a colon", doc: pack, old: "version: 4.0.1", new: `version: "27001:2022"`, want: kindyaml.ErrInvalid},
		{name: "version as a number", doc: pack, old: "version: 4.0.1", new: "version: 4.0", want: kindyaml.ErrSchema},
		{name: "effective date unquoted is accepted", doc: pack, old: `"2024-03-31"`, new: "2024-03-31", want: nil},
		{name: "effective date not a day", doc: pack, old: "2024-03-31", new: "2024-02-30", want: kindyaml.ErrInvalid},
		{name: "effective date in another form", doc: pack, old: "2024-03-31", new: "31.03.2024", want: kindyaml.ErrInvalid},
		{name: "effective date with a time", doc: pack, old: "2024-03-31", new: "2024-03-31T00:00:00Z", want: kindyaml.ErrInvalid},
		{name: "effective date without zero padding", doc: pack, old: "2024-03-31", new: "2024-3-31", want: kindyaml.ErrInvalid},
		{name: "empty effective date", doc: pack, old: `"2024-03-31"`, new: `""`, want: kindyaml.ErrInvalid},
		{name: "source twice", doc: pack, old: source, new: "[a, a]", want: kindyaml.ErrInvalid},
		{name: "empty source", doc: pack, old: source, new: `[""]`, want: kindyaml.ErrInvalid},
		{name: "source with a line break", doc: pack, old: source, new: `["a\nb"]`, want: kindyaml.ErrInvalid},
		{name: "source with a bidi character", doc: pack, old: source, new: `["a\u202eb"]`, want: kindyaml.ErrInvalid},
		{name: "source of exactly the limit is accepted", doc: pack, old: source, new: "[" + strings.Repeat("a", kindyaml.MaxSourceBytes) + "]", want: nil},
		{name: "source one byte over the limit", doc: pack, old: source, new: "[" + strings.Repeat("a", kindyaml.MaxSourceBytes+1) + "]", want: kindyaml.ErrInvalid},
		{name: "applicability absent is accepted", doc: pack, old: "  applicability: " + applicability + "\n", new: "", want: nil},
		{name: "empty applicability", doc: pack, old: applicability, new: `""`, want: kindyaml.ErrInvalid},
		{name: "applicability with a line break", doc: pack, old: applicability, new: `"a\nb"`, want: kindyaml.ErrInvalid},
		{name: "applicability with a bidi character", doc: pack, old: applicability, new: `"a\u2066b"`, want: kindyaml.ErrInvalid},
		{name: "applicability of exactly the limit is accepted", doc: pack, old: applicability, new: strings.Repeat("a", kindyaml.MaxApplicabilityBytes), want: nil},
		{name: "applicability one byte over the limit", doc: pack, old: applicability, new: strings.Repeat("a", kindyaml.MaxApplicabilityBytes+1), want: kindyaml.ErrInvalid},
		{name: "pack without export views is accepted", doc: pack, old: views, new: "", want: nil},
		{name: "pack without controls", doc: noViews, old: controls, new: "  controls: []\n", want: kindyaml.ErrInvalid},
		{name: "control without an id", doc: noViews, old: "{id: req-6.5.1, ", new: "{", want: kindyaml.ErrSchema},
		{name: "empty control id", doc: noViews, old: "id: req-6.5.1", new: `id: ""`, want: kindyaml.ErrInvalid},
		{name: "control id twice", doc: noViews, old: "id: req-6.5.1", new: "id: req-6.2.3", want: kindyaml.ErrInvalid},
		{name: "control id with whitespace", doc: noViews, old: "id: req-6.5.1", new: `id: "req 6.5.1"`, want: kindyaml.ErrInvalid},
		{name: "evidence named twice", doc: pack, old: "[review-record, diff]", new: "[diff, diff]", want: kindyaml.ErrInvalid},
		{name: "empty evidence name", doc: pack, old: "[review-record, diff]", new: `["", diff]`, want: kindyaml.ErrInvalid},
		{name: "control without evidence is accepted", doc: pack, old: "requires_evidence: [review-record, diff], ", new: "", want: nil},
		{name: "empty approver group", doc: pack, old: "group: payments-reviewers", new: `group: ""`, want: kindyaml.ErrInvalid},
		{name: "approver group with an at sign", doc: pack, old: "group: payments-reviewers", new: "group: admins@example", want: kindyaml.ErrInvalid},
		{name: "approver group with a Cyrillic letter", doc: pack, old: "group: payments-reviewers", new: "group: \"payments-r\u0435viewers\"", want: kindyaml.ErrInvalid},
		{name: "approver group without a human", doc: pack, old: "{human: true, group", new: "{group", want: kindyaml.ErrInvalid},
		{name: "dual control without a human", doc: pack, old: "{human: true, dual_control", new: "{dual_control", want: kindyaml.ErrInvalid},
		{name: "dual control with human false", doc: pack, old: "{human: true, dual_control", new: "{human: false, dual_control", want: kindyaml.ErrInvalid},
		{name: "human approval alone is accepted", doc: pack, old: "{human: true, dual_control: true}", new: "{human: true}", want: nil},
		{name: "approval flag not a boolean", doc: pack, old: "dual_control: true", new: "dual_control: yes", want: kindyaml.ErrSchema},
		{name: "export view of an unknown control", doc: pack, old: "controls: [req-6.2.3, req-6.5.1]", new: "controls: [req-6.2.3, req-9.9.9]", want: kindyaml.ErrInvalid},
		{name: "export view without controls", doc: pack, old: ", controls: [req-6.2.3, req-6.5.1]", new: "", want: kindyaml.ErrInvalid},
		{name: "export view naming a control twice", doc: pack, old: "controls: [req-6.2.3, req-6.5.1]", new: "controls: [req-6.2.3, req-6.2.3]", want: kindyaml.ErrInvalid},
		{name: "export view named twice", doc: pack, old: "    - {name: pci-dss-req6,", new: "    - {name: pci-dss-req6, controls: [req-6.2.3]}\n    - {name: pci-dss-req6,", want: kindyaml.ErrInvalid},
		{name: "empty export view name", doc: pack, old: "name: pci-dss-req6", new: `name: ""`, want: kindyaml.ErrInvalid},
		{name: "counsel status of the zero value", doc: pack, old: "COUNSEL_STATUS_NOT_REVIEWED", new: "COUNSEL_STATUS_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "counsel status absent", doc: pack, old: "  counsel_status: COUNSEL_STATUS_NOT_REVIEWED\n", new: "", want: kindyaml.ErrSchema},
		{name: "pack without floors is accepted", doc: pack, old: floors, new: "", want: nil},
		{name: "pack floors as a bare list", doc: pack, old: floors, new: "  floors: [{match: {environment: production}, max_level: AUTONOMY_LEVEL_PR}]\n", want: kindyaml.ErrSchema},
		{name: "pack autonomy floors without rules", doc: pack, old: "rules: [{match: {environment: production}, max_level: AUTONOMY_LEVEL_PR}]", new: "rules: []", want: kindyaml.ErrInvalid},
		{name: "pack autonomy floor of the zero level", doc: pack, old: "max_level: AUTONOMY_LEVEL_PR", new: "max_level: AUTONOMY_LEVEL_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "pack residency floor of the zero class", doc: pack, old: "RESIDENCY_CLASS_IN_ACCOUNT", new: "RESIDENCY_CLASS_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "pack residency floors without a floor", doc: pack, old: "{floor: RESIDENCY_CLASS_IN_ACCOUNT}", new: "{}", want: kindyaml.ErrSchema},
	}
	l := newLoader(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.doc, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			doc := strings.Replace(tt.doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(doc)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", doc, m, err, tt.want)
			}
		})
	}
}

// TestLoader_Decode_ExecutorManifestChecks pins the loader checks of
// ExecutorManifest: each case breaks the example in one place.
func TestLoader_Decode_ExecutorManifestChecks(t *testing.T) {
	manifest := example(t, "ExecutorManifest/claude-cli-bedrock.yaml")
	const model = "{id: eu.anthropic.claude-sonnet-v1:0, max_context: 200000}"
	tests := []struct {
		name string
		old  string
		new  string
		want error
	}{
		{name: "repo scope", old: "scope: SCOPE_ORG", new: "scope: SCOPE_REPO", want: kindyaml.ErrInvalid},
		{name: "empty provider", old: "provider: dev.numinance.official/claude-cli", new: `provider: ""`, want: kindyaml.ErrInvalid},
		{name: "provider with whitespace", old: "provider: dev.numinance.official/claude-cli", new: `provider: "claude cli"`, want: kindyaml.ErrInvalid},
		{name: "empty capabilities", old: "  capabilities: " + before(after(manifest, "  capabilities: "), "  models:"), new: "  capabilities: []\n", want: kindyaml.ErrInvalid},
		{name: "capability twice", old: "EXECUTOR_CAPABILITY_MCP", new: "EXECUTOR_CAPABILITY_SHELL", want: kindyaml.ErrInvalid},
		{name: "zero capability", old: "EXECUTOR_CAPABILITY_MCP", new: "EXECUTOR_CAPABILITY_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "unknown capability", old: "EXECUTOR_CAPABILITY_MCP", new: "EXECUTOR_CAPABILITY_ROOT", want: kindyaml.ErrSchema},
		{name: "no models key", old: "  models:\n    - " + model + "\n", new: "", want: kindyaml.ErrInvalid},
		{name: "empty models", old: "  models:\n    - " + model, new: "  models: []", want: kindyaml.ErrInvalid},
		{name: "model twice", old: model, new: model + "\n    - {id: eu.anthropic.claude-sonnet-v1:0, max_context: 1000}", want: kindyaml.ErrInvalid},
		{name: "name@version model id is accepted", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: claude-sonnet@20260101", want: nil},
		{name: "model id with a dot-dot segment", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: a/../x", want: kindyaml.ErrInvalid},
		{name: "model id with an empty segment", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: a//b", want: kindyaml.ErrInvalid},
		{name: "model id in user:secret@host form", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: user:secret@host/x", want: kindyaml.ErrInvalid},
		{name: "model id in user:secret@host form without a slash", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: user:secret@host", want: kindyaml.ErrInvalid},
		{name: "model id with a token in the last segment", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: x/user:token@host", want: kindyaml.ErrInvalid},
		{name: "model id ending in @", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: a@", want: kindyaml.ErrInvalid},
		{name: "model id with two @", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: a@b@c", want: kindyaml.ErrInvalid},
		{name: "model id with a colon after @", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: a@b:c", want: kindyaml.ErrInvalid},
		{name: "provider with a Cyrillic letter", old: "official/claude-cli", new: "official/cl\u0430ude-cli", want: kindyaml.ErrInvalid},
		{name: "provider with a dot-dot segment", old: "provider: dev.numinance.official/claude-cli", new: "provider: ../../x", want: kindyaml.ErrInvalid},
		{name: "executor id with whitespace", old: "name: claude-cli-bedrock", new: `name: "claude cli bedrock"`, want: kindyaml.ErrInvalid},
		{name: "executor id with a colon", old: "name: claude-cli-bedrock", new: "name: a:b", want: kindyaml.ErrInvalid},
		{name: "model id starting with @", old: "id: eu.anthropic.claude-sonnet-v1:0", new: `id: "@claude"`, want: kindyaml.ErrInvalid},
		{name: "inference profile ARN is accepted", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: arn:aws:bedrock:eu-central-1:123456789012:inference-profile/eu.anthropic.claude-sonnet-v1:0", want: nil},
		{name: "empty model id", old: "id: eu.anthropic.claude-sonnet-v1:0", new: `id: ""`, want: kindyaml.ErrInvalid},
		{name: "model id with whitespace", old: "id: eu.anthropic.claude-sonnet-v1:0", new: `id: "claude sonnet"`, want: kindyaml.ErrInvalid},
		{name: "model id with a leading slash", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: /claude", want: kindyaml.ErrInvalid},
		{name: "model id with a Cyrillic letter", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: \"clаude\"", want: kindyaml.ErrInvalid},
		{name: "model id over 253 bytes", old: "id: eu.anthropic.claude-sonnet-v1:0", new: "id: " + strings.Repeat("a", 254), want: kindyaml.ErrInvalid},
		{name: "zero max_context", old: "max_context: 200000", new: "max_context: 0", want: kindyaml.ErrInvalid},
		{name: "model without max_context", old: ", max_context: 200000", new: "", want: kindyaml.ErrSchema},
		{name: "zero cost class", old: "EXECUTOR_COST_CLASS_MEDIUM", new: "EXECUTOR_COST_CLASS_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "residency is not a field yet", old: "  cost_class:", new: "  residency: {}\n  cost_class:", want: kindyaml.ErrSchema},
	}
	l := newLoader(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(manifest, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			doc := strings.Replace(manifest, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(doc)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", doc, m, err, tt.want)
			}
		})
	}
}

// TestLoader_RefusesProducedKind checks that a ContextBundle, which Numinance
// produces, is never loaded or written as configuration, even when the
// document carries the metadata of a kind.
func TestLoader_RefusesProducedKind(t *testing.T) {
	l := newLoader(t)
	bundle := example(t, "ContextBundle/implement-stage.yaml")
	for _, doc := range []string{
		bundle,
		strings.Replace(bundle, "kind: ContextBundle\n", "kind: ContextBundle\nmetadata: {name: b, scope: SCOPE_ORG}\n", 1),
	} {
		if m, err := l.Decode([]byte(doc)); !errors.Is(err, kindyaml.ErrKind) {
			t.Errorf("Decode(%q) = %v, %v; want error %v", doc, m, err, kindyaml.ErrKind)
		}
	}
	m := &kindsv1.ContextBundle{ApiVersion: kindyaml.APIVersion, Kind: "ContextBundle", Spec: &kindsv1.ContextBundleSpec{ChangeId: "chg_x"}}
	if out, err := l.Encode(m); !errors.Is(err, kindyaml.ErrKind) {
		t.Errorf("Encode(%v) = %q, %v; want error %v", m, out, err, kindyaml.ErrKind)
	}
}

// after returns doc after the first sep, which the test documents contain.
func after(doc, sep string) string {
	_, a, _ := strings.Cut(doc, sep)
	return a
}
