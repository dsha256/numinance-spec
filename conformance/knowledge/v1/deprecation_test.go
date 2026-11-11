package knowledgekit_test

import (
	"os"
	"path/filepath"
	"testing"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

// TestDeprecationMarkers runs DEPR-001 to DEPR-003 of README.md: each deprecated element of the knowledge
// configuration surface still loads and is reported as DEPRECATED with what replaces it.
func TestDeprecationMarkers(t *testing.T) {
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	tests := []struct {
		id, example, path, replacedBy string
	}{
		{"DEPR-001", "KnowledgeConnection/confluence-eng.yaml", "spec", "numinance.kinds.v1.AdapterSelectorSpec.tools"},
		{"DEPR-002", "AdapterSelector/payments.yaml", "spec.ports.knowledge", "numinance.kinds.v1.AdapterSelectorSpec.tools"},
		{"DEPR-003", "Lockfile/legacy-knowledge-tools.yaml", "spec.knowledge_tools", "numinance.kinds.v1.LockfileSpec.pinned_tools"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "examples", tt.example))
			if err != nil {
				t.Fatal(err)
			}
			m, err := l.Decode(b)
			if err != nil {
				t.Fatalf("Decode(%s) = %v, want it to load for the deprecation window", tt.example, err)
			}
			got := kindyaml.Deprecations(m)
			if len(got) != 1 || got[0].Path != tt.path || got[0].DeprecatedIn != "1.3.0" || got[0].ReplacedBy != tt.replacedBy {
				t.Fatalf("Deprecations(%s) = %+v, want %s deprecated in 1.3.0, replaced by %s", tt.example, got, tt.path, tt.replacedBy)
			}
			if d := got[0].Diagnostic(); d.GetReason() != pluginv1.Reason_REASON_DEPRECATED || d.GetSeverity() != pluginv1.Severity_SEVERITY_WARNING {
				t.Errorf("Diagnostic() = %v, want reason DEPRECATED at warning severity", d)
			}
		})
	}
}
