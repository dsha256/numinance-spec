package providertemplate_test

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/numinance/numinance-spec/sdk/go/extpkg"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
	"github.com/numinance/numinance-spec/sdk/go/providertemplate"
)

func params() providertemplate.Params {
	return providertemplate.Params{
		PluginID: "com.example.inventory", Version: "0.1.0", Signer: "https://issuer.example#subject.example",
		Capability:  "inventory.get_item",
		Credentials: []string{"inventory.read"},
		Network:     []providertemplate.Network{{Host: "api.example.com", Port: 443}},
		Filesystem:  []providertemplate.Path{{Path: "/work", ReadWrite: false}, {Path: "/work/out", ReadWrite: true}},
		Environment: []string{"dev"},
		Sources:     []string{"bundle/item-1"},
	}
}

func render(t *testing.T, p providertemplate.Params) map[string][]byte {
	t.Helper()
	files, err := providertemplate.Render(p)
	if err != nil {
		t.Fatalf("Render() = %v", err)
	}
	m := map[string][]byte{}
	for _, f := range files {
		m[f.Path] = f.Data
	}
	return m
}

func TestRender_LayoutAndDeterminism(t *testing.T) {
	t.Parallel()
	first, err := providertemplate.Render(params())
	if err != nil {
		t.Fatalf("Render() = %v", err)
	}
	paths := make([]string, 0, len(first))
	for _, f := range first {
		paths = append(paths, f.Path)
	}
	want := []string{"README.md", "main.go", "provider/provider.go", "provider/conformance_test.go", "provider/security_test.go", "SOURCES.md", "manifest.yaml"}
	if !slices.Equal(paths, want) {
		t.Errorf("Render() paths = %v, want %v", paths, want)
	}
	second, _ := providertemplate.Render(params())
	for i := range first {
		if first[i].Path != second[i].Path || !bytes.Equal(first[i].Data, second[i].Data) {
			t.Errorf("Render() is not deterministic at %s", first[i].Path)
		}
	}
}

func TestRender_WithoutSourcesHasNoSourcesFile(t *testing.T) {
	t.Parallel()
	p := params()
	p.Sources = nil
	if _, ok := render(t, p)["SOURCES.md"]; ok {
		t.Error("Render() wrote SOURCES.md without sources")
	}
}

func TestRender_GoFilesParse(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	for path, data := range render(t, params()) {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		if _, err := parser.ParseFile(fset, path, data, parser.AllErrors); err != nil {
			t.Errorf("%s does not parse: %v", path, err)
		}
	}
}

// The manifest decodes strictly as a plugin/v1 Manifest, declares what was asked for and the digest of the other files.
func TestRender_ManifestDeclaresTheRequestAndMeasures(t *testing.T) {
	t.Parallel()
	files := render(t, params())
	js, err := kindyaml.YAMLToJSON(files["manifest.yaml"])
	if err != nil {
		t.Fatalf("YAMLToJSON() = %v", err)
	}
	var m pluginv1.Manifest
	if err := (protojson.UnmarshalOptions{}).Unmarshal(js, &m); err != nil {
		t.Fatalf("manifest does not decode as plugin/v1 Manifest: %v", err)
	}
	if m.GetPluginId() != "com.example.inventory" || m.GetVersion() != "0.1.0" || len(m.GetCapabilities()) != 1 ||
		len(m.GetPorts()) != 1 || m.GetPorts()[0] != "numinance.ports.inventory.v1" || m.GetSignerIdentity() != params().Signer {
		t.Fatalf("manifest = %v, want the identity, the one port and the one capability asked for", &m)
	}
	c := m.GetCapabilities()[0]
	if c.GetName() != "inventory.get_item" || c.GetConformanceSuite() != "conformance/inventory/v1/basic" ||
		!slices.Equal(c.GetCredentialIntents(), []string{"inventory.read"}) || !slices.Equal(c.GetEnvironmentScope(), []string{"dev"}) ||
		len(c.GetNetworkIntents()) != 1 || c.GetNetworkIntents()[0].GetHost() != "api.example.com" || len(c.GetFilesystemIntents()) != 2 {
		t.Errorf("capability = %v, want the intents asked for", c)
	}
	// Worst case, never a guess that understates: the template cannot know the verb.
	if c.GetRiskFloor().String() != "RISK_CLASS_PRIVILEGED" || c.GetSideEffect().String() != "SIDE_EFFECT_DEPLOY" || c.GetIdempotency().String() != "IDEMPOTENCY_NONE" || c.GetDryRun() {
		t.Errorf("risk, side effect, idempotency, dry_run = %v, %v, %v, %v; want the worst case", c.GetRiskFloor(), c.GetSideEffect(), c.GetIdempotency(), c.GetDryRun())
	}
	if m.GetSbomRef() != nil || m.GetProvenanceRef() != nil {
		t.Error("manifest carries an SBOM or provenance reference, which only the build may add")
	}
	tree := make([]extpkg.File, 0, len(files))
	for p, d := range files {
		tree = append(tree, extpkg.File{Path: p, Data: d})
	}
	if got := extpkg.ContentDigest(tree); m.GetDigest().GetValue() != got {
		t.Errorf("manifest digest = %s, want %s measured from the files", m.GetDigest().GetValue(), got)
	}
}

func TestRender_TestSkeletonsFailUntilWritten(t *testing.T) {
	t.Parallel()
	files := render(t, params())
	for _, p := range []string{"provider/conformance_test.go", "provider/security_test.go"} {
		if !bytes.Contains(files[p], []byte("t.Fatal(")) {
			t.Errorf("%s does not fail, want a skeleton that cannot claim a check nobody made", p)
		}
	}
}

func TestRender_RefusesParametersThatCouldInject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*providertemplate.Params)
	}{
		{"plugin id with a newline", func(p *providertemplate.Params) { p.PluginID = "com.example.x\nkind: y" }},
		{"plugin id with one label", func(p *providertemplate.Params) { p.PluginID = "example" }},
		{"version not semver", func(p *providertemplate.Params) { p.Version = "1.0" }},
		{"signer with a quote", func(p *providertemplate.Params) { p.Signer = `a"b#c` }},
		{"signer with a newline", func(p *providertemplate.Params) { p.Signer = "a\nb#c" }},
		{"capability without a verb", func(p *providertemplate.Params) { p.Capability = "inventory" }},
		{"capability with code", func(p *providertemplate.Params) { p.Capability = "inventory.x\"\nfunc init(){}" }},
		{"credential with a bracket", func(p *providertemplate.Params) { p.Credentials = []string{"a], b: [c"} }},
		{"empty environment", func(p *providertemplate.Params) { p.Environment = nil }},
		{"environment with a comma", func(p *providertemplate.Params) { p.Environment = []string{"dev, prod"} }},
		{"host with a scheme", func(p *providertemplate.Params) {
			p.Network = []providertemplate.Network{{Host: "https://x.example", Port: 443}}
		}},
		{"host with an uppercase letter", func(p *providertemplate.Params) {
			p.Network = []providertemplate.Network{{Host: "API.example.com", Port: 443}}
		}},
		{"port zero", func(p *providertemplate.Params) { p.Network = []providertemplate.Network{{Host: "api.example.com"}} }},
		{"relative path", func(p *providertemplate.Params) { p.Filesystem = []providertemplate.Path{{Path: "work"}} }},
		{"path with dot dot", func(p *providertemplate.Params) { p.Filesystem = []providertemplate.Path{{Path: "/work/../etc"}} }},
		{"path with a brace", func(p *providertemplate.Params) { p.Filesystem = []providertemplate.Path{{Path: "/work}"}} }},
		{"source with a space", func(p *providertemplate.Params) { p.Sources = []string{"a b"} }},
		{"source with markdown", func(p *providertemplate.Params) { p.Sources = []string{"x](http://evil)"} }},
		{"port major out of range", func(p *providertemplate.Params) { p.PortMajor = 100 }},
		{"too many credentials", func(p *providertemplate.Params) { p.Credentials = make([]string, 33) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := params()
			tc.edit(&p)
			if files, err := providertemplate.Render(p); !errors.Is(err, providertemplate.ErrParams) || files != nil {
				t.Errorf("Render() = %d files, %v; want ErrParams", len(files), err)
			}
		})
	}
}
