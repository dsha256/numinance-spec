package kindyaml_test

import (
	"slices"
	"testing"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

func TestDeprecations(t *testing.T) {
	l := newLoader(t)
	tests := []struct {
		name    string
		example string
		want    []kindyaml.Deprecation
	}{
		{name: "a KnowledgeConnection is deprecated as a kind", example: "KnowledgeConnection/confluence-eng.yaml", want: []kindyaml.Deprecation{{
			Kind: "KnowledgeConnection", Path: "spec", Field: "numinance.kinds.v1.KnowledgeConnection.spec",
			DeprecatedIn: "1.3.0", ReplacedBy: "numinance.kinds.v1.AdapterSelectorSpec.tools"}}},
		{name: "knowledge in an AdapterSelector", example: "AdapterSelector/payments.yaml", want: []kindyaml.Deprecation{{
			Kind: "AdapterSelector", Path: "spec.ports.knowledge", Field: "numinance.kinds.v1.AdapterPorts.knowledge",
			DeprecatedIn: "1.3.0", ReplacedBy: "numinance.kinds.v1.AdapterSelectorSpec.tools"}}},
		{name: "knowledge_tools in a Lockfile", example: "Lockfile/legacy-knowledge-tools.yaml", want: []kindyaml.Deprecation{{
			Kind: "Lockfile", Path: "spec.knowledge_tools", Field: "numinance.kinds.v1.LockfileSpec.knowledge_tools",
			DeprecatedIn: "1.3.0", ReplacedBy: "numinance.kinds.v1.LockfileSpec.pinned_tools"}}},
		{name: "pinned_tools and tools are not deprecated", example: "Lockfile/numinance-lock.yaml", want: nil},
		{name: "an AdapterSelector with tools and no knowledge", example: "AdapterSelector/payments-tools.yaml", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := l.Decode([]byte(example(t, tt.example)))
			if err != nil {
				t.Fatalf("Decode(%s) = %v", tt.example, err)
			}
			if got := kindyaml.Deprecations(m); !slices.Equal(got, tt.want) {
				t.Errorf("Deprecations(%s) = %+v, want %+v", tt.example, got, tt.want)
			}
		})
	}
}

func TestDeprecation_Diagnostic(t *testing.T) {
	d := kindyaml.Deprecation{Kind: "Lockfile", Path: "spec.knowledge_tools"}.Diagnostic()
	if d.GetReason() != pluginv1.Reason_REASON_DEPRECATED || d.GetSeverity() != pluginv1.Severity_SEVERITY_WARNING {
		t.Errorf("Diagnostic() = %v, want reason DEPRECATED at warning severity", d)
	}
}

func TestDeprecations_NilMessage(t *testing.T) {
	if got := kindyaml.Deprecations(nil); got != nil {
		t.Errorf("Deprecations(nil) = %v, want nil", got)
	}
}
