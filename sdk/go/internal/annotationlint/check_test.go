package annotationlint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
)

func cleanAction() *annotationsv1.ActionOptions {
	return &annotationsv1.ActionOptions{
		Risk:           annotationsv1.RiskClass_RISK_CLASS_READ,
		Exec:           annotationsv1.ExecMode_EXEC_MODE_SYNC,
		Since:          "1.0.0",
		LlmDescription: "Reads a thing.",
	}
}

func method(name string, a *annotationsv1.ActionOptions) *descriptorpb.MethodDescriptorProto {
	m := &descriptorpb.MethodDescriptorProto{Name: proto.String(name)}
	if a != nil {
		m.Options = new(descriptorpb.MethodOptions)
		proto.SetExtension(m.Options, annotationsv1.E_Action, a)
	}
	return m
}

func field(name, since string) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{Name: proto.String(name)}
	if since != "" {
		f.Options = new(descriptorpb.FieldOptions)
		proto.SetExtension(f.Options, annotationsv1.E_Field, &annotationsv1.FieldAnnotations{Since: since})
	}
	return f
}

func file(pkg string) *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String(strings.ReplaceAll(pkg, ".", "/") + "/x.proto"),
		Package: proto.String(pkg),
	}
}

func withService(f *descriptorpb.FileDescriptorProto, ms ...*descriptorpb.MethodDescriptorProto) *descriptorpb.FileDescriptorProto {
	f.Service = append(f.Service, &descriptorpb.ServiceDescriptorProto{Name: proto.String("S"), Method: ms})
	return f
}

func withMessage(f *descriptorpb.FileDescriptorProto, m *descriptorpb.DescriptorProto) *descriptorpb.FileDescriptorProto {
	f.MessageType = append(f.MessageType, m)
	return f
}

// rules returns "RULE element" pairs, so a test pins both what fired and where.
func rules(vs []Violation) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Rule+" "+strings.Fields(v.Message)[1])
	}
	return out
}

func TestCheck(t *testing.T) {
	incomplete := &annotationsv1.ActionOptions{Since: "v1.0", LlmDescription: " \t"}
	mapEntry := &descriptorpb.DescriptorProto{
		Name:    proto.String("LabelsEntry"),
		Field:   []*descriptorpb.FieldDescriptorProto{field("key", ""), field("value", "")},
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
	}
	tests := []struct {
		name string
		file *descriptorpb.FileDescriptorProto
		want []string
	}{
		{
			name: "clean RPC and field",
			file: withMessage(withService(file("numinance.x.v1"), method("Get", cleanAction())),
				&descriptorpb.DescriptorProto{Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{field("id", "1.2.3")}}),
		},
		{
			name: "RPC without options",
			file: withService(file("numinance.x.v1"), method("Get", nil)),
			want: []string{"ACTION_ANNOTATION_REQUIRED numinance.x.v1.S.Get"},
		},
		{
			name: "RPC with other options but no action",
			file: withService(file("numinance.x.v1"), &descriptorpb.MethodDescriptorProto{
				Name: proto.String("Get"), Options: &descriptorpb.MethodOptions{Deprecated: proto.Bool(true)},
			}),
			want: []string{"ACTION_ANNOTATION_REQUIRED numinance.x.v1.S.Get"},
		},
		{
			name: "action missing every required value",
			file: withService(file("numinance.x.v1"), method("Put", incomplete)),
			want: []string{
				"ACTION_EXEC_REQUIRED numinance.x.v1.S.Put",
				"ACTION_LLM_DESCRIPTION_REQUIRED numinance.x.v1.S.Put",
				"ACTION_RISK_REQUIRED numinance.x.v1.S.Put",
				"ACTION_SINCE_REQUIRED numinance.x.v1.S.Put",
			},
		},
		{
			name: "field without since",
			file: withMessage(file("numinance.x.v1"), &descriptorpb.DescriptorProto{
				Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{field("id", "")},
			}),
			want: []string{"FIELD_SINCE_REQUIRED numinance.x.v1.M.id"},
		},
		{
			name: "field with other options but no field annotation",
			file: withMessage(file("numinance.x.v1"), &descriptorpb.DescriptorProto{
				Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{{
					Name: proto.String("id"), Options: &descriptorpb.FieldOptions{Deprecated: proto.Bool(true)},
				}},
			}),
			want: []string{"FIELD_SINCE_REQUIRED numinance.x.v1.M.id"},
		},
		{
			name: "field with a malformed since",
			file: withMessage(file("numinance.x.v1"), &descriptorpb.DescriptorProto{
				Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{field("id", "1.0")},
			}),
			want: []string{"FIELD_SINCE_REQUIRED numinance.x.v1.M.id"},
		},
		{
			name: "nested message field and message extension are checked, map entries are not",
			file: withMessage(file("numinance.x.v1"), &descriptorpb.DescriptorProto{
				Name:       proto.String("M"),
				Field:      []*descriptorpb.FieldDescriptorProto{field("labels", "1.0.0")},
				Extension:  []*descriptorpb.FieldDescriptorProto{field("ext", "")},
				NestedType: []*descriptorpb.DescriptorProto{mapEntry, {Name: proto.String("N"), Field: []*descriptorpb.FieldDescriptorProto{field("n", "")}}},
			}),
			want: []string{"FIELD_SINCE_REQUIRED numinance.x.v1.M.ext", "FIELD_SINCE_REQUIRED numinance.x.v1.M.N.n"},
		},
		{
			name: "file-level extension outside the annotations package is checked",
			file: func() *descriptorpb.FileDescriptorProto {
				f := file("numinance.kinds.v1")
				f.Extension = []*descriptorpb.FieldDescriptorProto{field("merge", "")}
				return f
			}(),
			want: []string{"FIELD_SINCE_REQUIRED numinance.kinds.v1.merge"},
		},
		{
			name: "annotations.proto: extensions exempt, message fields checked",
			file: func() *descriptorpb.FileDescriptorProto {
				f := withMessage(file("numinance.annotations.v1"), &descriptorpb.DescriptorProto{
					Name: proto.String("ActionOptions"), Field: []*descriptorpb.FieldDescriptorProto{field("risk", "")},
				})
				f.Name = proto.String(annotationsFile)
				f.Extension = []*descriptorpb.FieldDescriptorProto{field("action", ""), field("field", "")}
				return f
			}(),
			want: []string{"FIELD_SINCE_REQUIRED numinance.annotations.v1.ActionOptions.risk"},
		},
		{
			name: "sibling file in the annotations package: extensions checked",
			file: func() *descriptorpb.FileDescriptorProto {
				f := file("numinance.annotations.v1")
				f.Extension = []*descriptorpb.FieldDescriptorProto{field("other", "")}
				return f
			}(),
			want: []string{"FIELD_SINCE_REQUIRED numinance.annotations.v1.other"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules(Check(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{tt.file}}))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Check() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheck_Positions(t *testing.T) {
	f := withMessage(withService(file("numinance.x.v1"), method("Get", nil)),
		&descriptorpb.DescriptorProto{Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{field("id", "")}})
	f.SourceCodeInfo = &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{
		{Path: []int32{fileServiceTag, 0, serviceMethodTag, 0}, Span: []int32{9, 2, 30}},
		{Path: []int32{fileMessageTag, 0, messageFieldTag, 0}, Span: []int32{19, 4, 20, 1}},
		{Path: []int32{fileMessageTag, 0, messageFieldTag, 0}, Span: []int32{99, 0, 1}}, // later duplicates are ignored
	}}
	g := withService(file("numinance.a.v1"), method("Get", nil)) // no source info
	got := Check(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{f, g}})
	want := []string{
		"numinance/a/v1/x.proto:0:0:ACTION_ANNOTATION_REQUIRED RPC numinance.a.v1.S.Get has no (numinance.annotations.v1.action) option.",
		"numinance/x/v1/x.proto:10:3:ACTION_ANNOTATION_REQUIRED RPC numinance.x.v1.S.Get has no (numinance.annotations.v1.action) option.",
		"numinance/x/v1/x.proto:20:5:FIELD_SINCE_REQUIRED field numinance.x.v1.M.id has no (numinance.annotations.v1.field).since or it is not a strict x.y.z version.",
	}
	lines := make([]string, 0, len(got))
	for _, v := range got {
		lines = append(lines, v.String())
	}
	if !slices.Equal(lines, want) {
		t.Errorf("Check() =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestIsStrictVersion(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{in: "1.0.0", want: true},
		{in: "0.0.0", want: true},
		{in: "10.20.30", want: true},
		{in: "", want: false},
		{in: "1.0", want: false},
		{in: "1.0.0.0", want: false},
		{in: "v1.0.0", want: false},
		{in: "01.0.0", want: false},
		{in: "1.00.0", want: false},
		{in: "1.0.0-alpha.1", want: false},
		{in: "1.0.0+build", want: false},
		{in: "1..0", want: false},
		{in: " 1.0.0", want: false},
		{in: "1.0.x", want: false},
		{in: "1.0.٣", want: false},
		{in: "1.0.0\n", want: false},
	}
	for _, tt := range tests {
		if got := isStrictVersion(tt.in); got != tt.want {
			t.Errorf("isStrictVersion(%q) = %t, want %t", tt.in, got, tt.want)
		}
	}
}

func TestDecode_ResolvesOptions(t *testing.T) {
	// Decode must turn the serialized options back into typed extensions; if it
	// left them as unknown fields every annotated RPC would be reported.
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		withMessage(withService(file("numinance.x.v1"), method("Get", cleanAction()), method("Put", nil)),
			&descriptorpb.DescriptorProto{Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{field("id", "1.0.0")}}),
	}}
	b, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	decoded, err := Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	got := rules(Check(decoded))
	want := []string{"ACTION_ANNOTATION_REQUIRED numinance.x.v1.S.Put"}
	if !slices.Equal(got, want) {
		t.Errorf("Check(Decode()) = %q, want %q", got, want)
	}
}

func TestDecode_Errors(t *testing.T) {
	if _, err := Decode(bytes.NewReader(make([]byte, MaxDescriptorSetBytes+1))); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Decode(oversized) error = %v, want ErrTooLarge", err)
	}
	if _, err := Decode(strings.NewReader("\xff\xff\xff")); err == nil {
		t.Error("Decode(garbage) error = nil, want a decode error")
	}
}

// rpc is a fully annotated method whose input type is the format filled with pkg.
func rpc(name, input, pkg string) *descriptorpb.MethodDescriptorProto {
	m := method(name, &annotationsv1.ActionOptions{
		Risk: annotationsv1.RiskClass_RISK_CLASS_READ, Exec: annotationsv1.ExecMode_EXEC_MODE_SYNC,
		Since: "1.0.0", LlmDescription: "x",
	})
	m.InputType = proto.String(fmt.Sprintf(input, pkg))
	return m
}

func hostOnlyField(name, typeName string, kind descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Type: kind.Enum(), Options: new(descriptorpb.FieldOptions)}
	if typeName != "" {
		f.TypeName = proto.String(typeName)
	}
	proto.SetExtension(f.Options, annotationsv1.E_Field, &annotationsv1.FieldAnnotations{Since: "1.0.0", HostOnly: true})
	return f
}

func TestCheck_HostOnly(t *testing.T) {
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING
	wrapper := &descriptorpb.DescriptorProto{Name: proto.String("Refs"), Field: []*descriptorpb.FieldDescriptorProto{
		hostOnlyField("nested", ".numinance.types.v1.Reference", msg),
	}}
	hollow := &descriptorpb.DescriptorProto{Name: proto.String("Hollow"), Field: []*descriptorpb.FieldDescriptorProto{field("id", "1.0.0")}}
	tests := []struct {
		name string
		pkg  string
		f    *descriptorpb.FieldDescriptorProto
		// orphan adds a message with a host_only Reference that no RPC input reaches.
		orphan bool
		want   []string
	}{
		{name: "Reference field of a port file", pkg: "numinance.ports.x.v1", f: hostOnlyField("ref", ".numinance.types.v1.Reference", msg)},
		{name: "wrapper of References in a port file", pkg: "numinance.ports.x.v1", f: hostOnlyField("refs", ".numinance.ports.x.v1.Refs", msg)},
		{name: "wrapper with no Reference", pkg: "numinance.ports.x.v1", f: hostOnlyField("h", ".numinance.ports.x.v1.Hollow", msg),
			want: []string{"HOST_ONLY_INVALID numinance.ports.x.v1.M.h"}},
		{name: "string field", pkg: "numinance.ports.x.v1", f: hostOnlyField("key", "", str),
			want: []string{"HOST_ONLY_INVALID numinance.ports.x.v1.M.key"}},
		{name: "Reference field outside a port file", pkg: "numinance.plugin.v1", f: hostOnlyField("ref", ".numinance.types.v1.Reference", msg),
			want: []string{"HOST_ONLY_INVALID numinance.plugin.v1.M.ref"}},
		{name: "unknown message type", pkg: "numinance.ports.x.v1", f: hostOnlyField("o", ".other.v1.Thing", msg),
			want: []string{"HOST_ONLY_INVALID numinance.ports.x.v1.M.o"}},
		{name: "message no RPC input reaches", pkg: "numinance.ports.x.v1", f: field("plain", "1.0.0"), orphan: true,
			want: []string{"HOST_ONLY_INVALID numinance.ports.x.v1.Orphan.ref"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := file(tt.pkg)
			f.MessageType = []*descriptorpb.DescriptorProto{
				{Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{tt.f}}, wrapper, hollow,
			}
			if tt.orphan {
				f.MessageType = append(f.MessageType, &descriptorpb.DescriptorProto{Name: proto.String("Orphan"), Field: []*descriptorpb.FieldDescriptorProto{
					hostOnlyField("ref", ".numinance.types.v1.Reference", msg),
				}})
			}
			// M and Refs are inputs of RPCs, so the marked fields of both are reachable.
			withService(f, rpc("Put", ".%s.M", tt.pkg), rpc("PutRefs", ".%s.Refs", tt.pkg))
			got := rules(Check(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{f}}))
			// The wrapper's own marked field sits in the file under test, so it is clean only in a port file.
			if tt.pkg != "numinance.ports.x.v1" {
				got = slices.DeleteFunc(got, func(s string) bool { return strings.Contains(s, ".Refs.") })
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Check(host_only %s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func resourceNameField(name string, kind descriptorpb.FieldDescriptorProto_Type, label descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Type: kind.Enum(), Label: label.Enum(), Options: new(descriptorpb.FieldOptions)}
	proto.SetExtension(f.Options, annotationsv1.E_Field, &annotationsv1.FieldAnnotations{Since: "1.0.0", ResourceName: true})
	return f
}

func TestCheck_ResourceName(t *testing.T) {
	str, num := descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_TYPE_UINT32
	one, many := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	tests := []struct {
		name   string
		pkg    string
		f      *descriptorpb.FieldDescriptorProto
		orphan bool
		want   []string
	}{
		{name: "string field of a port file", pkg: "numinance.ports.x.v1", f: resourceNameField("repo", str, one)},
		{name: "repeated string field of a port file", pkg: "numinance.ports.x.v1", f: resourceNameField("repos", str, many)},
		{name: "number field", pkg: "numinance.ports.x.v1", f: resourceNameField("n", num, one),
			want: []string{"RESOURCE_NAME_INVALID numinance.ports.x.v1.M.n"}},
		{name: "string field outside a port file", pkg: "numinance.plugin.v1", f: resourceNameField("repo", str, one),
			want: []string{"RESOURCE_NAME_INVALID numinance.plugin.v1.M.repo"}},
		{name: "message no RPC input reaches", pkg: "numinance.ports.x.v1", f: field("plain", "1.0.0"), orphan: true,
			want: []string{"RESOURCE_NAME_INVALID numinance.ports.x.v1.Orphan.repo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := file(tt.pkg)
			f.MessageType = []*descriptorpb.DescriptorProto{{Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{tt.f}}}
			if tt.orphan {
				f.MessageType = append(f.MessageType, &descriptorpb.DescriptorProto{Name: proto.String("Orphan"), Field: []*descriptorpb.FieldDescriptorProto{
					resourceNameField("repo", str, one),
				}})
			}
			withService(f, rpc("Put", ".%s.M", tt.pkg))
			got := rules(Check(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{f}}))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Check(resource_name %s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestLint_DuplicateOptions: the set is decoded with an empty resolver for the
// duplicate scan, so two (field) entries, or two host_only entries in one, are
// seen before any merge can drop the earlier one.
func TestLint_DuplicateOptions(t *testing.T) {
	entry := func(inner ...[]byte) []byte {
		var body []byte
		for _, in := range inner {
			body = append(body, in...)
		}
		return protowire.AppendBytes(protowire.AppendTag(nil, 50002, protowire.BytesType), body)
	}
	hostOnly := func(v uint64) []byte {
		return protowire.AppendVarint(protowire.AppendTag(nil, 5, protowire.VarintType), v)
	}
	resourceName := func(v uint64) []byte {
		return protowire.AppendVarint(protowire.AppendTag(nil, 6, protowire.VarintType), v)
	}
	since := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "1.0.0")
	tests := []struct {
		name string
		raw  []byte
		want []string
	}{
		{name: "one entry", raw: entry(since, hostOnly(1))},
		{name: "two entries", raw: append(entry(since, hostOnly(1)), entry(hostOnly(0))...), want: []string{"FIELD_OPTION_DUPLICATE numinance.x.v1.M.f"}},
		{name: "two host_only values in one entry", raw: entry(since, hostOnly(1), hostOnly(0)), want: []string{"FIELD_OPTION_DUPLICATE numinance.x.v1.M.f"}},
		{name: "one resource_name", raw: entry(since, resourceName(1))},
		{name: "two resource_name values in one entry", raw: entry(since, resourceName(1), resourceName(0)), want: []string{"FIELD_OPTION_DUPLICATE numinance.x.v1.M.f"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &descriptorpb.FieldDescriptorProto{Name: proto.String("f"), Options: new(descriptorpb.FieldOptions)}
			f.Options.ProtoReflect().SetUnknown(tt.raw)
			set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
				withMessage(file("numinance.x.v1"), &descriptorpb.DescriptorProto{Name: proto.String("M"), Field: []*descriptorpb.FieldDescriptorProto{f}}),
			}}
			b, err := proto.Marshal(set)
			if err != nil {
				t.Fatalf("proto.Marshal() error = %v", err)
			}
			vs, err := Lint(bytes.NewReader(b))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}
			var got []string
			for _, v := range vs {
				if v.Rule == RuleDuplicateOptions {
					got = append(got, v.Rule+" "+strings.Fields(v.Message)[1])
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Lint(%s) duplicate findings = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

type scopeCase struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Valid bool   `json:"valid"`
}

// TestAnnotationLint_ScopeGrammar runs the shared case table of the scope grammar, which the kernel's
// TestScopeGrammar_MirrorsAnnotationLint runs against authority.Grant.Validate, and checks that an RPC with an
// invalid or repeated action.scopes entry is reported without echoing it.
func TestAnnotationLint_ScopeGrammar(t *testing.T) {
	raw, err := os.ReadFile("testdata/scope_grammar.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []scopeCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := isScopeName(tc.Scope); got != tc.Valid {
				t.Errorf("isScopeName(%q) = %t, want %t", tc.Scope, got, tc.Valid)
			}
			a := cleanAction()
			a.Scopes = []string{tc.Scope}
			vs := Check(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{withService(file("numinance.x.v1"), method("Get", a))}})
			if got := len(vs) == 0; got != tc.Valid {
				t.Errorf("Check(scopes [%q]) = %v, want clean: %t", tc.Scope, vs, tc.Valid)
			}
		})
	}
	t.Run("a long entry", func(t *testing.T) {
		if isScopeName("a." + strings.Repeat("b", 255)) {
			t.Error("isScopeName(257 bytes) = true, want false")
		}
		if !isScopeName("a." + strings.Repeat("b", 254)) {
			t.Error("isScopeName(256 bytes) = false, want true")
		}
	})
	t.Run("a repeated entry", func(t *testing.T) {
		a := cleanAction()
		a.Scopes = []string{"change.read", "change.read"}
		vs := Check(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{withService(file("numinance.x.v1"), method("Get", a))}})
		if len(vs) != 1 || vs[0].Rule != RuleActionScope || strings.Contains(vs[0].Message, "change.read") {
			t.Errorf("Check(repeated scope) = %v, want one %s that does not echo the entry", vs, RuleActionScope)
		}
	})
}

// A resource_name mark behind a map value is reachable from the RPC input (the walk follows the map entry message), so
// the lint accepts it, and the Go reader of the policy side follows the same path.
func TestCheck_ResourceNameBehindAMapValue(t *testing.T) {
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	repeated := descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	const pkg = "numinance.ports.x.v1"
	entry := &descriptorpb.DescriptorProto{
		Name: proto.String("ByNameEntry"), Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
		Field: []*descriptorpb.FieldDescriptorProto{
			{Name: proto.String("key"), Type: str.Enum()},
			{Name: proto.String("value"), Type: msg.Enum(), TypeName: proto.String("." + pkg + ".Target")},
		},
	}
	m := &descriptorpb.DescriptorProto{
		Name: proto.String("M"), NestedType: []*descriptorpb.DescriptorProto{entry},
		Field: []*descriptorpb.FieldDescriptorProto{{Name: proto.String("by_name"), Type: msg.Enum(), Label: repeated.Enum(), TypeName: proto.String("." + pkg + ".M.ByNameEntry"), Options: field("x", "1.0.0").Options}},
	}
	target := &descriptorpb.DescriptorProto{Name: proto.String("Target"), Field: []*descriptorpb.FieldDescriptorProto{resourceNameField("name", str, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL)}}
	f := file(pkg)
	f.MessageType = []*descriptorpb.DescriptorProto{m, target}
	withService(f, rpc("Put", ".%s.M", pkg))
	if got := rules(Check(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{f}})); len(got) != 0 {
		t.Errorf("Check(resource_name behind a map value) = %q, want clean", got)
	}
}
