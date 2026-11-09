package schemas_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
	"github.com/numinance/numinance-spec/sdk/go/schema"
)

const draft = "https://json-schema.org/draft/2020-12/schema"

// kindSchema is the file name of the generated schema a loader validates kind against.
func kindSchema(kind string) string {
	return "numinance.kinds.v1." + kind + ".jsonschema.strict.bundle.json"
}

// kinds returns the kind messages: the top-level messages of numinance.kinds.v1 with an api_version field.
func kinds(t *testing.T) []protoreflect.MessageDescriptor {
	t.Helper()
	var out []protoreflect.MessageDescriptor
	protoregistry.GlobalFiles.RangeFilesByPackage(kindsPackage(), func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Messages().Len() {
			if m := fd.Messages().Get(i); m.Fields().ByName("api_version") != nil {
				out = append(out, m)
			}
		}
		return true
	})
	if len(out) == 0 {
		t.Fatal("no kind messages registered in numinance.kinds.v1")
	}
	return out
}

// produced must match produced in sdk/go/kindyaml/loader.go: the kinds whose
// envelope is apiVersion, kind and spec.
func produced(kind string) bool { return kind == "ContextBundle" }

func kindsPackage() protoreflect.FullName {
	return kindsv1.File_numinance_kinds_v1_meta_proto.Package()
}

// TestSchemas_SelfContained checks that every generated schema is JSON Schema
// 2020-12 and that every $ref points into the same file and resolves there.
func TestSchemas_SelfContained(t *testing.T) {
	gen := os.DirFS("gen")
	files, err := fs.Glob(gen, "*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("Glob(gen/*.json) = %v, %v; want at least one schema", files, err)
	}
	for _, f := range files {
		b, err := fs.ReadFile(gen, f)
		if err != nil {
			t.Fatalf("ReadFile(gen/%s) = %v", f, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Errorf("gen/%s is not JSON: %v", f, err)
			continue
		}
		if got := doc["$schema"]; got != draft {
			t.Errorf("gen/%s $schema = %v, want %q", f, got, draft)
		}
		defs, _ := doc["$defs"].(map[string]any)
		for _, ref := range refs(doc) {
			name, ok := strings.CutPrefix(ref, "#/$defs/")
			if !ok {
				t.Errorf("gen/%s: $ref %q is not local to the file (#/$defs/...)", f, ref)
				continue
			}
			if _, ok := defs[name]; !ok {
				t.Errorf("gen/%s: $ref %q does not resolve", f, ref)
			}
		}
	}
}

// refs returns every $ref value in v, at any depth.
func refs(v any) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any:
		for k, e := range v {
			if s, ok := e.(string); ok && k == "$ref" {
				out = append(out, s)
				continue
			}
			out = append(out, refs(e)...)
		}
	case []any:
		for _, e := range v {
			out = append(out, refs(e)...)
		}
	}
	return out
}

func TestRefs_FindsNestedRefs(t *testing.T) {
	doc := map[string]any{
		"$ref": "#/$defs/a",
		"properties": map[string]any{
			"x": map[string]any{"items": map[string]any{"$ref": "other.json"}},
			"y": []any{map[string]any{"$ref": "#/$defs/b"}},
		},
	}
	got := refs(doc)
	slices.Sort(got)
	if want := []string{"#/$defs/a", "#/$defs/b", "other.json"}; !slices.Equal(got, want) {
		t.Errorf("refs(doc) = %q, want %q", got, want)
	}
}

// TestSchemas_FileSet pins the generated files: one bundle per message of
// numinance.kinds.v1 and the dependencies the generator also emits, so a
// new file in the public schemas directory is a deliberate change.
func TestSchemas_FileSet(t *testing.T) {
	want := []string{
		"numinance.annotations.v1.FieldAnnotations.jsonschema.strict.bundle.json", // pulled in by the field options; no loader uses it
		"numinance.types.v1.Digest.jsonschema.strict.bundle.json",                 // pulled in by PinnedTool.hash; no loader uses it
	}
	protoregistry.GlobalFiles.RangeFilesByPackage(kindsPackage(), func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Messages().Len() {
			want = append(want, string(fd.Messages().Get(i).FullName())+".jsonschema.strict.bundle.json")
		}
		return true
	})
	got, err := fs.Glob(os.DirFS("gen"), "*")
	if err != nil {
		t.Fatalf("Glob(gen/*) = %v", err)
	}
	slices.Sort(want) // fs.Glob returns names sorted, so both sides are in name order
	if !slices.Equal(got, want) {
		t.Errorf("gen/ = %q, want %q", got, want)
	}
}

// TestSchemas_Required pins the required keys of each kind schema. Strict
// mode makes every implicit-presence field required, so a spec scalar without
// optional would silently become mandatory; this test catches that.
func TestSchemas_Required(t *testing.T) {
	tests := []struct {
		schema  string
		message string
		want    []string
	}{
		{schema: "BudgetPolicy", message: "BudgetPolicy", want: []string{"apiVersion", "kind"}},
		{schema: "BudgetPolicy", message: "BudgetPolicySpec", want: nil},
		{schema: "BudgetPolicy", message: "Metadata", want: []string{"name", "scope"}},
		{schema: "IndexerPlugin", message: "IndexerPlugin", want: []string{"apiVersion", "kind"}},
		{schema: "IndexerPlugin", message: "IndexerPluginSpec", want: nil},
		{schema: "IndexerPlugin", message: "Indexer", want: []string{"provider"}},
		{schema: "IndexerPlugin", message: "Metadata", want: []string{"name", "scope"}},
		{schema: "StageRegistry", message: "StageRegistrySpec", want: nil},
		{schema: "StageRegistry", message: "StageRegistryEntry", want: []string{"name", "maps_to"}},
		{schema: "StageRegistry", message: "StageGate", want: []string{"kind"}},
		{schema: "StageRegistry", message: "SeparationOfDuties", want: nil},
		{schema: "Playbook", message: "PlaybookSpec", want: []string{"change_type"}},
		{schema: "Playbook", message: "PlaybookStep", want: []string{"name", "capability_set", "risk"}},
		{schema: "Playbook", message: "Qualification", want: nil},
		{schema: "ArchitecturePack", message: "ArchitecturePackSpec", want: nil},
		{schema: "ArchitecturePack", message: "ArchitectureRule", want: []string{"id", "statement", "severity"}},
		{schema: "ArchitecturePack", message: "ArchitectureCheck", want: []string{"verifier", "rule_id"}},
		{schema: "LanguageToolchain", message: "LanguageToolchainSpec", want: []string{"language", "version"}},
		{schema: "LanguageToolchain", message: "ToolchainVerifier", want: []string{"name", "provider"}},
		{schema: "EnvironmentProvider", message: "EnvironmentProviderSpec", want: []string{"provider", "kind"}},
		{schema: "EnvironmentProvider", message: "EnvironmentBudget", want: nil},
		{schema: "AdapterSelector", message: "AdapterSelectorSpec", want: nil},
		{schema: "AdapterSelector", message: "AdapterPorts", want: nil},
		{schema: "AdapterSelector", message: "AdapterBinding", want: []string{"provider", "instance"}},
		{schema: "AutonomyPolicy", message: "AutonomyPolicySpec", want: nil},
		{schema: "AutonomyPolicy", message: "AutonomyDefaults", want: []string{"level"}},
		{schema: "AutonomyPolicy", message: "AutonomyMatch", want: nil},
		{schema: "AutonomyPolicy", message: "AutonomyRule", want: []string{"level"}},
		{schema: "AutonomyPolicy", message: "AutonomyFloors", want: nil},
		{schema: "AutonomyPolicy", message: "AutonomyFloor", want: []string{"max_level"}},
		{schema: "ResidencyPolicy", message: "ResidencyPolicySpec", want: nil},
		{schema: "ResidencyPolicy", message: "ResidencyFloors", want: []string{"floor"}},
		{schema: "ResidencyPolicy", message: "ResidencyMatch", want: nil},
		{schema: "ResidencyPolicy", message: "ResidencyRule", want: []string{"floor"}},
		{schema: "KnowledgeConnection", message: "KnowledgeConnectionSpec", want: []string{"provider", "read_only"}},
		{schema: "KnowledgeConnection", message: "PinnedTool", want: []string{"name"}},
		{schema: "KnowledgeConnection", message: "KnowledgeBudgets", want: nil},
		{schema: "KnowledgeConnection", message: "numinance.types.v1.Digest", want: []string{"value"}},
		{schema: "SDLCProfile", message: "SDLCProfileSpec", want: nil},
		{schema: "SDLCProfile", message: "AdapterPorts", want: nil},
		{schema: "SDLCProfile", message: "AutonomyPolicySpec", want: nil},
		{schema: "SDLCProfile", message: "ResidencyPolicySpec", want: nil},
		{schema: "SDLCProfile", message: "BudgetPolicySpec", want: nil},
		{schema: "ControlPack", message: "ControlPackSpec", want: []string{"jurisdiction", "framework", "version", "effective_date", "counsel_status"}},
		{schema: "ControlPack", message: "Control", want: []string{"id"}},
		{schema: "ControlPack", message: "ControlApproval", want: nil},
		{schema: "ControlPack", message: "ControlFloors", want: nil},
		{schema: "ControlPack", message: "ExportView", want: []string{"name"}},
		{schema: "ExecutorManifest", message: "ExecutorManifestSpec", want: []string{"provider", "cost_class"}},
		{schema: "ExecutorManifest", message: "ExecutorModel", want: []string{"id", "max_context"}},
		{schema: "Lockfile", message: "Lockfile", want: []string{"apiVersion", "kind"}},
		{schema: "Lockfile", message: "LockfileSpec", want: nil},
		{schema: "Lockfile", message: "LockedExtension", want: []string{"id", "version", "h1", "zh", "signer"}},
		{schema: "Lockfile", message: "LockedKnowledgeTool", want: []string{"connection", "name"}},
		{schema: "EvalSet", message: "EvalSet", want: []string{"apiVersion", "kind"}},
		{schema: "EvalSet", message: "EvalSetSpec", want: []string{"version"}},
		{schema: "EvalSet", message: "EvalCase", want: []string{"name"}},
		{schema: "EvalSet", message: "EvalExpectation", want: []string{"acceptance", "merged", "severe"}},
		{schema: "EvalSet", message: "EvalLayerCount", want: []string{"layer", "count"}},
		{schema: "EvalSet", message: "EvalSeeded", want: []string{"defect", "caught_by"}},
		{schema: "ContextBundle", message: "ContextBundle", want: []string{"apiVersion", "kind"}},
		{schema: "ContextBundle", message: "ContextBundleSpec", want: []string{"change_id", "work_unit_id", "stage", "context_generation"}},
		{schema: "ContextBundle", message: "BundleResolver", want: []string{"model_id", "prompt_version"}},
		{schema: "ContextBundle", message: "BundleQuery", want: []string{"tool", "results", "truncated"}},
		{schema: "ContextBundle", message: "BundleSource", want: []string{"ref", "system", "title", "rank", "selection_reason", "classification", "redactions", "access_basis", "staleness", "trust"}},
		{schema: "ContextBundle", message: "AuthoredConstraint", want: []string{"origin", "id", "statement", "trust"}},
		{schema: "ContextBundle", message: "CitedGuidance", want: []string{"ref"}},
		{schema: "ContextBundle", message: "UnresolvedQuestion", want: []string{"question", "blocking"}},
		{schema: "ContextBundle", message: "BundleBudgetUsed", want: []string{"tokens", "calls", "ms"}},
		{schema: "ContextBundle", message: "BundleConflict", want: []string{"topic"}},
	}
	for _, tt := range tests {
		b, err := fs.ReadFile(os.DirFS("gen"), kindSchema(tt.schema))
		if err != nil {
			t.Fatalf("ReadFile(gen/%s) = %v", kindSchema(tt.schema), err)
		}
		var doc struct {
			Defs map[string]struct {
				Required []string `json:"required"`
			} `json:"$defs"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("Unmarshal(gen/%s) = %v", kindSchema(tt.schema), err)
		}
		full := tt.message
		if !strings.Contains(full, ".") {
			full = "numinance.kinds.v1." + full
		}
		def, ok := doc.Defs[full+".jsonschema.strict.json"]
		if !ok {
			t.Errorf("%s schema has no $defs entry for %s", tt.schema, tt.message)
			continue
		}
		if !slices.Equal(def.Required, tt.want) {
			t.Errorf("%s schema: %s required = %q, want %q", tt.schema, tt.message, def.Required, tt.want)
		}
	}
}

// TestKinds_SchemaAndExample checks that every kind has its generated schema
// and at least one example under examples/<Kind>/, and that every example
// directory names a kind.
func TestKinds_SchemaAndExample(t *testing.T) {
	isKind := map[string]bool{}
	for _, m := range kinds(t) {
		kind := string(m.Name())
		isKind[kind] = true
		if _, err := os.Stat(path.Join("gen", kindSchema(kind))); err != nil {
			t.Errorf("kind %s has no schema: %v", kind, err)
		}
		ex, err := fs.Glob(os.DirFS("examples"), kind+"/*.yaml")
		if err != nil {
			t.Fatalf("Glob(examples/%s/*.yaml) = %v", kind, err)
		}
		if len(ex) == 0 {
			t.Errorf("kind %s has no example under examples/%s/", kind, kind)
		}
	}
	dirs, err := os.ReadDir("examples")
	if err != nil {
		t.Fatalf("ReadDir(examples) = %v", err)
	}
	for _, d := range dirs {
		if !d.IsDir() || !isKind[d.Name()] {
			t.Errorf("examples/%s: entries of examples/ are directories named after a kind", d.Name())
		}
	}
}

// TestExamples_Load decodes every example through the kind loader, which
// validates it against its kind's schema and runs the loader checks, checks
// that it names its directory's kind, and that it survives Encode and Decode.
func TestExamples_Load(t *testing.T) {
	// The alpha kinds load here because the examples are the documents of the kind; the default loader refuses them.
	l, err := kindyaml.NewLoader(kindyaml.WithAlphaKinds())
	if err != nil {
		t.Fatalf("kindyaml.NewLoader(WithAlphaKinds) = %v", err)
	}
	examples := os.DirFS("examples")
	files, err := fs.Glob(examples, "*/*")
	if err != nil {
		t.Fatalf("Glob(examples/*/*) = %v", err)
	}
	for _, f := range files {
		if path.Ext(f) != ".yaml" {
			t.Errorf("examples/%s: examples are <Kind>/<name>.yaml files", f)
			continue
		}
		b, err := fs.ReadFile(examples, f)
		if err != nil {
			t.Fatalf("ReadFile(examples/%s) = %v", f, err)
		}
		if produced(path.Dir(f)) {
			checkProduced(t, f, b)
			if m, err := l.Decode(b); !errors.Is(err, kindyaml.ErrKind) {
				t.Errorf("Decode(examples/%s) = %v, %v; want error %v: a produced kind is never loaded", f, m, err, kindyaml.ErrKind)
			}
			continue
		}
		m, err := l.Decode(b)
		if err != nil {
			t.Errorf("Decode(examples/%s) = %v", f, err)
			continue
		}
		if got, want := string(m.ProtoReflect().Descriptor().Name()), path.Dir(f); got != want {
			t.Errorf("examples/%s kind = %q, want %q", f, got, want)
		}
		out, err := l.Encode(m)
		if err != nil {
			t.Errorf("Encode(examples/%s) = %v", f, err)
			continue
		}
		if back, err := l.Decode(out); err != nil || !proto.Equal(back, m) {
			t.Errorf("Decode(Encode(examples/%s)) = %v, %v; want %v", f, back, err, m)
		}
	}
}

// checkProduced validates the example f of a produced kind against its
// generated schema and decodes it with protojson into the kind message.
func checkProduced(t *testing.T, f string, b []byte) {
	t.Helper()
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		t.Fatalf("yaml.Unmarshal(examples/%s) = %v", f, err)
	}
	js, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal(examples/%s) = %v", f, err)
	}
	dec := json.NewDecoder(bytes.NewReader(js))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("json.Decode(examples/%s) = %v", f, err)
	}
	kind := path.Dir(f)
	sb, err := fs.ReadFile(os.DirFS("gen"), kindSchema(kind))
	if err != nil {
		t.Fatalf("ReadFile(gen/%s) = %v", kindSchema(kind), err)
	}
	s, err := schema.Compile(sb)
	if err != nil {
		t.Fatalf("schema.Compile(gen/%s) = %v", kindSchema(kind), err)
	}
	if err := s.Validate(doc); err != nil {
		t.Errorf("Validate(examples/%s) = %v", f, err)
	}
	typ, err := protoregistry.GlobalTypes.FindMessageByName(kindsPackage().Append(protoreflect.Name(kind)))
	if err != nil {
		t.Fatalf("FindMessageByName(%s) = %v", kind, err)
	}
	if err := protojson.Unmarshal(js, typ.New().Interface()); err != nil {
		t.Errorf("protojson.Unmarshal(examples/%s) = %v", f, err)
	}
}

// TestKinds_Envelope checks that every kind message has the kind envelope
// with the field numbers, types and JSON names the kind envelope requires.
func TestKinds_Envelope(t *testing.T) {
	metadata := kindsv1.File_numinance_kinds_v1_meta_proto.Messages().ByName("Metadata").FullName()
	for _, m := range kinds(t) {
		f := m.Fields()
		check := func(name protoreflect.Name, num protoreflect.FieldNumber, kind protoreflect.Kind, jsonName string) protoreflect.FieldDescriptor {
			fd := f.ByName(name)
			if fd == nil {
				t.Errorf("%s has no field %s", m.Name(), name)
				return nil
			}
			if fd.Number() != num || fd.Kind() != kind || fd.JSONName() != jsonName {
				t.Errorf("%s.%s = (%d, %v, %q), want (%d, %v, %q)", m.Name(), name, fd.Number(), fd.Kind(), fd.JSONName(), num, kind, jsonName)
			}
			return fd
		}
		check("api_version", 1, protoreflect.StringKind, "apiVersion")
		check("kind", 2, protoreflect.StringKind, "kind")
		if produced(string(m.Name())) {
			// No name or scope claim and no extension data; 3 and 5 stay reserved.
			if fd := check("spec", 4, protoreflect.MessageKind, "spec"); fd != nil && string(fd.Message().Name()) != string(m.Name())+"Spec" {
				t.Errorf("%s.spec is %s, want %sSpec", m.Name(), fd.Message().Name(), m.Name())
			}
			if r := m.ReservedRanges(); f.Len() != 3 || !r.Has(3) || !r.Has(5) || !m.ReservedNames().Has("metadata") || !m.ReservedNames().Has("extensions") {
				t.Errorf("%s has %d fields, want api_version, kind and spec with 3, 5, metadata and extensions reserved", m.Name(), f.Len())
			}
			continue
		}
		if fd := check("metadata", 3, protoreflect.MessageKind, "metadata"); fd != nil && fd.Message().FullName() != metadata {
			t.Errorf("%s.metadata is %s, want %s", m.Name(), fd.Message().FullName(), metadata)
		}
		if fd := check("spec", 4, protoreflect.MessageKind, "spec"); fd != nil && string(fd.Message().Name()) != string(m.Name())+"Spec" {
			t.Errorf("%s.spec is %s, want %sSpec", m.Name(), fd.Message().Name(), m.Name())
		}
		if fd := check("extensions", 5, protoreflect.MessageKind, "extensions"); fd != nil &&
			(!fd.IsMap() || fd.MapKey().Kind() != protoreflect.StringKind || fd.MapValue().Message().FullName() != "google.protobuf.Value") {
			t.Errorf("%s.extensions is not map<string, google.protobuf.Value>", m.Name())
		}
		if f.Len() != 5 {
			t.Errorf("%s has %d fields, want the 5 envelope fields", m.Name(), f.Len())
		}
	}
}

// TestExecutorManifest_ResidencyFieldKept checks that field 5 of
// ExecutorManifestSpec stays free for residency, which a later additive change
// adds.
func TestExecutorManifest_ResidencyFieldKept(t *testing.T) {
	spec := (&kindsv1.ExecutorManifestSpec{}).ProtoReflect().Descriptor()
	if fd := spec.Fields().ByNumber(5); fd != nil && fd.Name() != "residency" {
		t.Errorf("ExecutorManifestSpec field 5 is %s, want it free or residency", fd.Name())
	}
	if fd := spec.Fields().ByName("residency"); fd != nil && fd.Number() != 5 {
		t.Errorf("ExecutorManifestSpec.residency is field %d, want 5", fd.Number())
	}
}

// TestKinds_SnakeCaseKeys checks that every field in numinance.kinds.v1 other
// than api_version has its proto name as its JSON name, so YAML keys are
// snake_case in the generated schema and in protojson.
func TestKinds_SnakeCaseKeys(t *testing.T) {
	protoregistry.GlobalFiles.RangeFilesByPackage(kindsPackage(), func(fd protoreflect.FileDescriptor) bool {
		var walk func(protoreflect.MessageDescriptors)
		walk = func(ms protoreflect.MessageDescriptors) {
			for i := range ms.Len() {
				m := ms.Get(i)
				if m.IsMapEntry() {
					continue
				}
				for j := range m.Fields().Len() {
					f := m.Fields().Get(j)
					if f.Name() == "api_version" {
						continue
					}
					if f.JSONName() != string(f.Name()) {
						t.Errorf("%s JSON name = %q, want %q: set json_name", f.FullName(), f.JSONName(), f.Name())
					}
				}
				walk(m.Messages())
			}
		}
		walk(fd.Messages())
		return true
	})
}

// mergeViolations walks the kind message k from its envelope and returns the
// merge directives it declares and, as "field: reason", each placement that
// breaks the merge-directive rules: a directive outside spec or inside a list
// or map element, where resolution never merges; APPEND off a repeated
// string; FORBID without presence (lists and maps have none), on a oneof
// member (a oneof merges as a unit) or below a REPLACE parent (a floor there
// is replaced with its parent).
func mergeViolations(k protoreflect.MessageDescriptor) (map[protoreflect.FullName]kindsv1.MergeDirective, []string) {
	got := map[protoreflect.FullName]kindsv1.MergeDirective{}
	var bad []string
	seen := map[protoreflect.FullName]bool{}
	var walk func(m protoreflect.MessageDescriptor, outsideSpec, inElement, underReplace bool)
	walk = func(m protoreflect.MessageDescriptor, outsideSpec, inElement, underReplace bool) {
		if seen[m.FullName()] || m.ParentFile().Package() != k.ParentFile().Package() {
			return
		}
		seen[m.FullName()] = true
		defer delete(seen, m.FullName())
		for i := range m.Fields().Len() {
			f := m.Fields().Get(i)
			outside := outsideSpec || m.FullName() == k.FullName() && f.Name() != "spec"
			d := proto.GetExtension(f.Options(), kindsv1.E_Merge).(kindsv1.MergeDirective)
			report := func(reason string) { bad = append(bad, string(f.FullName())+": "+reason) }
			if d != kindsv1.MergeDirective_MERGE_DIRECTIVE_UNSPECIFIED {
				got[f.FullName()] = d
				if outside {
					report("outside spec")
				}
				if inElement {
					report("inside a list or map element")
				}
			}
			if d == kindsv1.MergeDirective_MERGE_DIRECTIVE_APPEND && (!f.IsList() || f.Kind() != protoreflect.StringKind) {
				report("APPEND off a repeated string")
			}
			if d == kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID {
				if !f.HasPresence() {
					report("FORBID without presence")
				}
				if o := f.ContainingOneof(); o != nil && !o.IsSynthetic() {
					report("FORBID on a oneof member")
				}
				if underReplace {
					report("FORBID below a REPLACE parent")
				}
			}
			if f.Message() != nil {
				walk(f.Message(), outside, inElement || f.IsList() || f.IsMap(), underReplace || d == kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE)
			}
		}
	}
	walk(k, false, false, false)
	return got, bad
}

// TestKinds_MergeDirectives checks every kind against the placement rules of
// mergeViolations and pins every declared directive so a new one is deliberate.
func TestKinds_MergeDirectives(t *testing.T) {
	want := map[protoreflect.FullName]kindsv1.MergeDirective{
		"numinance.kinds.v1.LanguageToolchainSpec.formatters": kindsv1.MergeDirective_MERGE_DIRECTIVE_APPEND,
		"numinance.kinds.v1.AdapterPorts.scm":                 kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE,
		"numinance.kinds.v1.AdapterPorts.ticketing":           kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE,
		"numinance.kinds.v1.AdapterPorts.ciexecutor":          kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE,
		"numinance.kinds.v1.AdapterPorts.cdprovider":          kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE,
		"numinance.kinds.v1.AdapterPorts.cloud":               kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE,
		"numinance.kinds.v1.AdapterPorts.database":            kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE,
		"numinance.kinds.v1.AdapterPorts.observability":       kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE,
		"numinance.kinds.v1.AutonomyPolicySpec.floors":        kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.ResidencyPolicySpec.floors":       kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		// Interim until a tighten-only directive.
		"numinance.kinds.v1.ResidencyPolicySpec.cross_region":            kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.ResidencyPolicySpec.retention_max":           kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.ResidencyPolicySpec.require_customer_keying": kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.ResidencyPolicySpec.training_use":            kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.KnowledgeConnectionSpec.endpoint":            kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.KnowledgeConnectionSpec.auth":                kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.KnowledgeConnectionSpec.oauth":               kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.SDLCProfileSpec.playbooks":                   kindsv1.MergeDirective_MERGE_DIRECTIVE_APPEND,
		"numinance.kinds.v1.SDLCProfileSpec.control_packs":               kindsv1.MergeDirective_MERGE_DIRECTIVE_APPEND,
		"numinance.kinds.v1.SDLCProfileSpec.architecture_packs":          kindsv1.MergeDirective_MERGE_DIRECTIVE_APPEND,
		"numinance.kinds.v1.SDLCProfileSpec.stages":                      kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.SDLCProfileSpec.budgets":                     kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
		"numinance.kinds.v1.SDLCProfileSpec.toolchains":                  kindsv1.MergeDirective_MERGE_DIRECTIVE_APPEND,
		"numinance.kinds.v1.ControlPackSpec.floors":                      kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID,
	}
	got := map[protoreflect.FullName]kindsv1.MergeDirective{}
	for _, k := range kinds(t) {
		declared, bad := mergeViolations(k)
		for _, b := range bad {
			t.Errorf("%s: misplaced merge directive %s", k.FullName(), b)
		}
		maps.Copy(got, declared)
	}
	if !maps.Equal(got, want) {
		t.Errorf("declared merge directives = %v, want %v", got, want)
	}
}

// TestMergeViolations_ReportsEachRule runs mergeViolations on a fixture kind
// with one bad field per rule and two well-placed directives; Elem is reached
// through a list and through a map, so its misplaced floor is reported twice.
func TestMergeViolations_ReportsEachRule(t *testing.T) {
	const (
		replace = kindsv1.MergeDirective_MERGE_DIRECTIVE_REPLACE
		appendD = kindsv1.MergeDirective_MERGE_DIRECTIVE_APPEND
		forbid  = kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID
		none    = kindsv1.MergeDirective_MERGE_DIRECTIVE_UNSPECIFIED
		str     = descriptorpb.FieldDescriptorProto_TYPE_STRING
		msg     = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	)
	field := func(name string, n int32, repeated bool, typ descriptorpb.FieldDescriptorProto_Type, typeName string, d kindsv1.MergeDirective) *descriptorpb.FieldDescriptorProto {
		f := &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Number: proto.Int32(n), Type: typ.Enum(), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()}
		if repeated {
			f.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		}
		if typeName != "" {
			f.TypeName = proto.String(".fixture.v1." + typeName)
		}
		if d != none {
			f.Options = &descriptorpb.FieldOptions{}
			proto.SetExtension(f.Options, kindsv1.E_Merge, d)
		}
		return f
	}
	inOneof := func(f *descriptorpb.FieldDescriptorProto, i int32, synthetic bool) *descriptorpb.FieldDescriptorProto {
		f.OneofIndex = proto.Int32(i)
		f.Proto3Optional = proto.Bool(synthetic)
		return f
	}
	oneofs := func(names ...string) []*descriptorpb.OneofDescriptorProto {
		out := make([]*descriptorpb.OneofDescriptorProto, 0, len(names))
		for _, n := range names {
			out = append(out, &descriptorpb.OneofDescriptorProto{Name: proto.String(n)})
		}
		return out
	}
	floorMsg := func(name string) *descriptorpb.DescriptorProto {
		return &descriptorpb.DescriptorProto{
			Name:      proto.String(name),
			Field:     []*descriptorpb.FieldDescriptorProto{inOneof(field("floor", 1, false, str, "", forbid), 0, true)},
			OneofDecl: oneofs("_floor"),
		}
	}
	mapEntry := func(name string, value *descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
		return &descriptorpb.DescriptorProto{
			Name:    proto.String(name),
			Field:   []*descriptorpb.FieldDescriptorProto{field("key", 1, false, str, "", none), value},
			Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
		}
	}
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("fixture/v1/fixture.proto"),
		Package: proto.String("fixture.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Fixture"), Field: []*descriptorpb.FieldDescriptorProto{
				field("spec", 1, false, msg, "Spec", none),
				field("meta", 2, false, msg, "Plain", replace),
			}},
			{Name: proto.String("Plain")},
			floorMsg("Elem"),
			floorMsg("Parent"),
			{
				Name: proto.String("Spec"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("name", 1, false, str, "", appendD),
					field("bare", 2, false, str, "", forbid),
					field("list", 3, true, str, "", forbid),
					field("dict", 4, true, msg, "Spec.DictEntry", forbid),
					inOneof(field("member", 5, false, str, "", forbid), 0, false),
					field("elems", 6, true, msg, "Elem", none),
					field("parent", 7, false, msg, "Parent", replace),
					inOneof(field("ok", 8, false, str, "", forbid), 1, true),
					field("names", 9, true, str, "", appendD),
					field("by_key", 10, true, msg, "Spec.ByKeyEntry", none),
				},
				OneofDecl: oneofs("choice", "_ok"),
				NestedType: []*descriptorpb.DescriptorProto{
					mapEntry("DictEntry", field("value", 2, false, str, "", none)),
					mapEntry("ByKeyEntry", field("value", 2, false, msg, "Elem", none)),
				},
			},
		},
	}
	fd, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("protodesc.NewFile(fixture) = %v", err)
	}
	_, got := mergeViolations(fd.Messages().ByName("Fixture"))
	want := []string{
		"fixture.v1.Fixture.meta: outside spec",
		"fixture.v1.Spec.name: APPEND off a repeated string",
		"fixture.v1.Spec.bare: FORBID without presence",
		"fixture.v1.Spec.list: FORBID without presence",
		"fixture.v1.Spec.dict: FORBID without presence",
		"fixture.v1.Spec.member: FORBID on a oneof member",
		"fixture.v1.Elem.floor: inside a list or map element", // through elems
		"fixture.v1.Elem.floor: inside a list or map element", // through by_key
		"fixture.v1.Parent.floor: FORBID below a REPLACE parent",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("mergeViolations(Fixture) = %q, want %q", got, want)
	}
}
