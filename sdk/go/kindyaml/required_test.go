package kindyaml

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
)

// TestRequiredFields_NameFields keeps a typo in requiredFields from
// silently disabling the check. Only repeated fields (kept non-empty),
// implicit-presence fields (kept non-zero) and fields with presence (kept
// set: a singular message or an optional scalar) qualify; a map does not.
func TestRequiredFields_NameFields(t *testing.T) {
	for _, name := range requiredFields() {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
		if err != nil {
			t.Errorf("requiredFields: %s does not resolve: %v", name, err)
			continue
		}
		if fd, ok := d.(protoreflect.FieldDescriptor); !ok || fd.IsMap() {
			t.Errorf("requiredFields: %s is not a repeated, implicit-presence or presence field", name)
		}
	}
}

// TestValueChecks_NameFields keeps a typo in valueChecks from silently
// disabling a check.
func TestValueChecks_NameFields(t *testing.T) {
	for name := range valueChecks() {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
		if _, ok := d.(protoreflect.FieldDescriptor); err != nil || !ok {
			t.Errorf("valueChecks: %s does not resolve to a field: %v", name, err)
		}
	}
}

// TestUniqueKeys_NameFields keeps a typo in uniqueKeys from silently
// disabling a check, and calls each key on a zero element so a key naming a
// missing inner field fails here instead of panicking in Decode.
func TestUniqueKeys_NameFields(t *testing.T) {
	for name, key := range uniqueKeys() {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
		fd, ok := d.(protoreflect.FieldDescriptor)
		if err != nil || !ok || !fd.IsList() {
			t.Errorf("uniqueKeys: %s does not resolve to a repeated field: %v", name, err)
			continue
		}
		zero := fd.Default()
		switch {
		case fd.Message() != nil:
			zero = protoreflect.ValueOfMessage(dynamicpb.NewMessage(fd.Message()))
		case fd.Enum() != nil:
			// Default is invalid for a repeated enum; the zero element is the first value.
			zero = protoreflect.ValueOfEnum(fd.Enum().Values().Get(0).Number())
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("uniqueKeys: the key of %s panics on a zero element: %v", name, r)
				}
			}()
			key(zero)
		}()
	}
}

func TestReverseDNS(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{key: "com.example", want: true},
		{key: "com.example.indexing", want: true},
		{key: "dev.numinance-official.x9", want: true},
		{key: "example", want: false},
		{key: "", want: false},
		{key: ".com.example", want: false},
		{key: "com.example.", want: false},
		{key: "com.Example", want: false},
		{key: "com.-example", want: false},
		{key: "com.example-", want: false},
		{key: "com.9example", want: false},
		{key: "com.exa_mple", want: false},
		{key: "com.exämple", want: false},
	}
	for _, tt := range tests {
		if got := reverseDNS(tt.key); got != tt.want {
			t.Errorf("reverseDNS(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

// testMessage builds numinance.test.M { repeated E list; map<string, E> map;
// map<string, string> extensions } with enum E { E_UNSPECIFIED; E_A }, which
// no kind has yet.
func testMessage(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	opt := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	rep := descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	enum := descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
	entry := func(name string, typ *descriptorpb.FieldDescriptorProto_Type, typeName string) *descriptorpb.DescriptorProto {
		v := &descriptorpb.FieldDescriptorProto{Name: proto.String("value"), Number: proto.Int32(2), Label: opt, Type: typ}
		if typeName != "" {
			v.TypeName = proto.String(typeName)
		}
		return &descriptorpb.DescriptorProto{
			Name:    proto.String(name),
			Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
			Field:   []*descriptorpb.FieldDescriptorProto{{Name: proto.String("key"), Number: proto.Int32(1), Label: opt, Type: str}, v},
		}
	}
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("numinance/test/walk.proto"),
		Package: proto.String("numinance.test"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("E"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("E_UNSPECIFIED"), Number: proto.Int32(0)},
				{Name: proto.String("E_A"), Number: proto.Int32(1)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("M"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("list"), Number: proto.Int32(1), Label: rep, Type: enum, TypeName: proto.String(".numinance.test.E")},
				{Name: proto.String("map"), Number: proto.Int32(2), Label: rep, Type: msg, TypeName: proto.String(".numinance.test.M.MapEntry")},
				{Name: proto.String("extensions"), Number: proto.Int32(3), Label: rep, Type: msg, TypeName: proto.String(".numinance.test.M.ExtensionsEntry")},
			},
			NestedType: []*descriptorpb.DescriptorProto{
				entry("MapEntry", enum, ".numinance.test.E"),
				entry("ExtensionsEntry", str, ""),
			},
		}},
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile(walk.proto) = %v", err)
	}
	return fd.Messages().Get(0)
}

func TestLoader_Walk_ListAndMapValues(t *testing.T) {
	md := testMessage(t)
	fields := md.Fields()
	tests := []struct {
		name string
		set  func(m *dynamicpb.Message)
		want error
	}{
		{name: "accepts non-zero list and map values", set: func(m *dynamicpb.Message) {
			m.Mutable(fields.ByName("list")).List().Append(protoreflect.ValueOfEnum(1))
			m.Mutable(fields.ByName("map")).Map().Set(protoreflect.ValueOfString("k").MapKey(), protoreflect.ValueOfEnum(1))
			m.Mutable(fields.ByName("extensions")).Map().Set(protoreflect.ValueOfString("com.example").MapKey(), protoreflect.ValueOfString("v"))
		}},
		{name: "refuses a zero list element", set: func(m *dynamicpb.Message) {
			m.Mutable(fields.ByName("list")).List().Append(protoreflect.ValueOfEnum(0))
		}, want: ErrInvalid},
		{name: "refuses a zero map value", set: func(m *dynamicpb.Message) {
			m.Mutable(fields.ByName("map")).Map().Set(protoreflect.ValueOfString("k").MapKey(), protoreflect.ValueOfEnum(0))
		}, want: ErrInvalid},
		{name: "refuses a nested extensions key that is not reverse-DNS", set: func(m *dynamicpb.Message) {
			m.Mutable(fields.ByName("extensions")).Map().Set(protoreflect.ValueOfString("nightly").MapKey(), protoreflect.ValueOfString("v"))
		}, want: ErrInvalid},
	}
	l := &Loader{required: map[protoreflect.FullName]bool{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := dynamicpb.NewMessage(md)
			tt.set(m)
			if err := l.walk(m); !errors.Is(err, tt.want) {
				t.Errorf("walk(%v) = %v, want %v", m, err, tt.want)
			}
		})
	}
}

// FuzzEncode_RoundTrip puts one string in metadata.name, a required scalar,
// a list, an extension value and a nested extension key, and checks that
// Decode(Encode(m)) equals m.
func FuzzEncode_RoundTrip(f *testing.F) {
	for _, s := range []string{"x", "<<", "a\u0085b", "\u2028", "\u2029", "\x7f", "\ufeff", "a\n", " a\n\n", "\t", "true", "~", "- a", "#", "'", `"`, `\`, "\U0001F600", "a: b", "2026-10-18", "\x00", "\u202e"} {
		f.Add(s)
	}
	l, err := NewLoader()
	if err != nil {
		f.Fatalf("NewLoader() = %v", err)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) || len(s) > 4096 {
			t.Skip()
		}
		name, provider := s, s
		if s == "" || len(s) > MaxNameBytes || strings.ContainsFunc(s, unsafeNameRune) {
			name = "x"
		}
		if s == "" {
			provider = "p"
		}
		nested, err := structpb.NewStruct(map[string]any{s: s})
		if err != nil {
			t.Fatalf("structpb.NewStruct(%q) = %v", s, err)
		}
		m := &kindsv1.IndexerPlugin{
			ApiVersion: APIVersion,
			Kind:       "IndexerPlugin",
			Metadata:   &kindsv1.Metadata{Name: name, Scope: kindsv1.Scope_SCOPE_REPO},
			Spec:       &kindsv1.IndexerPluginSpec{Indexers: []*kindsv1.Indexer{{Provider: provider, Languages: []string{s}}}},
			Extensions: map[string]*structpb.Value{
				"com.example.a": structpb.NewStringValue(s),
				"com.example.b": structpb.NewStructValue(nested),
			},
		}
		out, err := l.Encode(m)
		if err != nil {
			t.Fatalf("Encode(%v) = %v", m, err)
		}
		back, err := l.Decode(out)
		if err != nil || !proto.Equal(back, m) {
			t.Errorf("Decode(Encode(%v)) = %v, %v, want %v; output %q", m, back, err, m, out)
		}
	})
}
