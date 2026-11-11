package migrations_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/numinance/numinance-spec/cli/migrations"
	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

func example(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "schemas", "examples", name)) //nolint:gosec // a fixed example under schemas/examples
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func loader(t *testing.T) *kindyaml.Loader {
	t.Helper()
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	return l
}

func plugins() migrations.Plugins {
	return migrations.Plugins{
		SelectorPlugin: map[string]string{"confluence-eng": "dev.numinance.official/bridge-mcp", "jira-pay": "dev.numinance.official/bridge-mcp"},
		LockPlugin:     map[string]string{"confluence-eng": "dev.numinance.official.bridge-mcp"},
	}
}

func TestDocument_AdapterSelector(t *testing.T) {
	l := loader(t)
	out, changed, err := migrations.Document(l, example(t, "AdapterSelector/payments.yaml"), plugins())
	if err != nil || !changed {
		t.Fatalf("Document(payments) = %q, %v, %v; want a rewrite", out, changed, err)
	}
	m, err := l.Decode(out)
	if err != nil {
		t.Fatalf("Decode(rewritten) = %v", err)
	}
	spec := m.(*kindsv1.AdapterSelector).GetSpec()
	if got := spec.GetPorts().GetKnowledge(); len(got) != 0 {
		t.Errorf("knowledge = %v, want it empty", got)
	}
	tools := spec.GetTools()
	if len(tools) != 2 || tools[0].GetPlugin() != "dev.numinance.official/bridge-mcp" || tools[0].GetInstance() != "confluence-eng" || tools[1].GetInstance() != "jira-pay" {
		t.Errorf("tools = %v, want confluence-eng and jira-pay on the bridge plugin", tools)
	}
	if spec.GetPorts().GetScm().GetInstance() != "main" {
		t.Errorf("scm binding = %v, want it kept", spec.GetPorts().GetScm())
	}
	if d := kindyaml.Deprecations(m); len(d) != 0 {
		t.Errorf("Deprecations(rewritten) = %v, want none", d)
	}
	// Running it again changes nothing.
	if again, changed, err := migrations.Document(l, out, plugins()); err != nil || changed || string(again) != string(out) {
		t.Errorf("Document(rewritten) = %q, %v, %v; want it unchanged", again, changed, err)
	}
}

func TestDocument_AdapterSelectorKeepsExistingTools(t *testing.T) {
	l := loader(t)
	doc := string(example(t, "AdapterSelector/payments.yaml")) +
		"  tools:\n    - {plugin: dev.numinance.official/bridge-mcp, instance: jira-pay}\n"
	out, changed, err := migrations.Document(l, []byte(doc), plugins())
	if err != nil || !changed {
		t.Fatalf("Document = %q, %v, %v", out, changed, err)
	}
	m, _ := l.Decode(out)
	if n := len(m.(*kindsv1.AdapterSelector).GetSpec().GetTools()); n != 2 {
		t.Errorf("tools = %d entries, want 2 (jira-pay is not added twice)", n)
	}
}

func TestDocument_Lockfile(t *testing.T) {
	l := loader(t)
	out, changed, err := migrations.Document(l, example(t, "Lockfile/legacy-knowledge-tools.yaml"), plugins())
	if err != nil || !changed {
		t.Fatalf("Document(legacy lockfile) = %q, %v, %v; want a rewrite", out, changed, err)
	}
	m, err := l.Decode(out)
	if err != nil {
		t.Fatalf("Decode(rewritten) = %v", err)
	}
	spec := m.(*kindsv1.Lockfile).GetSpec()
	if len(spec.GetKnowledgeTools()) != 0 || len(spec.GetPinnedTools()) != 1 {
		t.Fatalf("spec = %v, want one pinned tool and no knowledge tool", spec)
	}
	want := m.(*kindsv1.Lockfile).GetSpec().GetPinnedTools()[0]
	if want.GetPlugin() != "dev.numinance.official.bridge-mcp" || want.GetConnection() != "confluence-eng" || want.GetName() != "getConfluencePage" ||
		want.GetHash().GetValue() != "sha256:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d" {
		t.Errorf("pinned tool = %v, want the legacy pin carried over with its hash", want)
	}
	if len(spec.GetExtensions()) != 1 {
		t.Errorf("extensions = %d entries, want them kept", len(spec.GetExtensions()))
	}
}

func TestDocument_Unmapped(t *testing.T) {
	l := loader(t)
	for _, name := range []string{"AdapterSelector/payments.yaml", "Lockfile/legacy-knowledge-tools.yaml"} {
		if out, changed, err := migrations.Document(l, example(t, name), migrations.Plugins{}); !errors.Is(err, migrations.ErrUnmapped) || changed {
			t.Errorf("Document(%s, no plugins) = %q, %v, %v; want ErrUnmapped", name, out, changed, err)
		}
	}
}

func TestDocument_LeavesOtherDocumentsAlone(t *testing.T) {
	l := loader(t)
	for _, name := range []string{"Lockfile/numinance-lock.yaml", "AdapterSelector/payments-tools.yaml", "KnowledgeConnection/confluence-eng.yaml", "Playbook/dependency-upgrade.yaml"} {
		data := example(t, name)
		if out, changed, err := migrations.Document(l, data, plugins()); err != nil || changed || string(out) != string(data) {
			t.Errorf("Document(%s) = %v, %v; want it unchanged", name, changed, err)
		}
	}
}

func TestDocument_Invalid(t *testing.T) {
	if _, _, err := migrations.Document(loader(t), []byte("kind: Nope\n"), plugins()); !errors.Is(err, kindyaml.ErrKind) {
		t.Errorf("Document(unknown kind) error = %v, want ErrKind", err)
	}
}

func TestPluginsByConnection(t *testing.T) {
	l := loader(t)
	m, err := l.Decode(example(t, "KnowledgeConnection/confluence-eng.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := migrations.PluginsByConnection(m.(*kindsv1.KnowledgeConnection))
	if got["confluence-eng"] != "dev.numinance.official/bridge-mcp" {
		t.Errorf("PluginsByConnection = %v, want confluence-eng on dev.numinance.official/bridge-mcp", got)
	}
}
