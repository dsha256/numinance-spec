package plugin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	authorexecutorv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// digestOf is a digest that differs per key, so refusal by digest identity
// never makes two keys collide in a test.
func digestOf(key string) *typesv1.Digest {
	sum := sha256.Sum256([]byte(key))
	return &typesv1.Digest{Value: "sha256:" + hex.EncodeToString(sum[:])}
}

func ref(key string) *typesv1.Reference {
	return &typesv1.Reference{
		ArtifactRef:  key,
		Sha256:       digestOf(key),
		ArtifactType: "task",
	}
}

func executeRequest() *authorexecutorv1.ExecuteRequest {
	return &authorexecutorv1.ExecuteRequest{Request: &authorexecutorv1.ChangeExecutionRequest{
		TaskRef:          ref("artifacts/task"),
		PlanRef:          ref("artifacts/plan"),
		WorktreeRef:      ref("artifacts/worktree"),
		NetworkPolicyRef: ref("artifacts/netpolicy"),
	}}
}

func TestCheckResolvable(t *testing.T) {
	shared := &authorexecutorv1.ExecuteRequest{Request: &authorexecutorv1.ChangeExecutionRequest{
		TaskRef:     ref("artifacts/shared"),
		WorktreeRef: ref("artifacts/shared"),
	}}
	noType := ref("artifacts/task")
	noType.ArtifactType = "plan"
	noDigest := ref("artifacts/task")
	noDigest.Sha256 = nil

	tests := []struct {
		name string
		req  proto.Message
		ref  *typesv1.Reference
		want error
	}{
		{name: "task_ref resolves", req: executeRequest(), ref: ref("artifacts/task")},
		{name: "plan_ref resolves", req: executeRequest(), ref: ref("artifacts/plan")},
		{name: "worktree_ref is refused", req: executeRequest(), ref: ref("artifacts/worktree"), want: plugin.ErrReferenceHostOnly},
		{name: "network_policy_ref is refused", req: executeRequest(), ref: ref("artifacts/netpolicy"), want: plugin.ErrReferenceHostOnly},
		{name: "a value shared with a resolvable field is refused", req: shared, ref: ref("artifacts/shared"), want: plugin.ErrReferenceHostOnly},
		{name: "a reference no field holds", req: executeRequest(), ref: ref("artifacts/other"), want: plugin.ErrReferenceNotFound},
		{name: "a reference differing in one field", req: executeRequest(), ref: noType, want: plugin.ErrReferenceNotFound},
		{name: "a reference with no digest", req: executeRequest(), ref: noDigest, want: plugin.ErrReferenceNotFound},
		{name: "a reference with no artifact_ref", req: executeRequest(), ref: &typesv1.Reference{Sha256: ref("x").Sha256}, want: plugin.ErrReferenceNotFound},
		{name: "a nil reference", req: executeRequest(), ref: nil, want: plugin.ErrReferenceNotFound},
		{name: "context_bundle_ref resolves", req: &authorexecutorv1.ChangeExecutionRequest{ContextBundleRef: ref("artifacts/bundle")}, ref: ref("artifacts/bundle")},
		{name: "a repeated Reference field resolves", req: &authorexecutorv1.ExecutionEvent{References: []*typesv1.Reference{ref("artifacts/a"), ref("artifacts/b")}}, ref: ref("artifacts/b")},
		{name: "a Reference inside an Any is not found", req: func() proto.Message {
			a, err := anypb.New(&typesv1.Reference{ArtifactRef: "artifacts/any", Sha256: ref("x").Sha256, ArtifactType: "task"})
			if err != nil {
				t.Fatalf("anypb.New = %v", err)
			}
			return a
		}(), ref: &typesv1.Reference{ArtifactRef: "artifacts/any", Sha256: ref("x").Sha256, ArtifactType: "task"}, want: plugin.ErrReferenceNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := plugin.CheckResolvable(tt.req, tt.ref); !errors.Is(err, tt.want) {
				t.Errorf("CheckResolvable(%s) = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}

// holder builds a request type with a repeated Reference, a map of References
// and a host_only wrapper, which no port declares yet.
func holder(t *testing.T, refs, byName []*typesv1.Reference, inner *typesv1.Reference) *dynamicpb.Message {
	t.Helper()
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
	repeated, optional := descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	refType := proto.String(".numinance.types.v1.Reference")
	hostOnly := &descriptorpb.FieldOptions{}
	proto.SetExtension(hostOnly, annotationsv1.E_Field, &annotationsv1.FieldAnnotations{Since: "1.0.0", HostOnly: true})
	fdp := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("numinance/ports/holder/v1/holder.proto"),
		Package:    proto.String("numinance.ports.holder.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"numinance/types/v1/types.proto", "numinance/annotations/v1/annotations.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Holder"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: proto.String("refs"), JsonName: proto.String("refs"), Number: proto.Int32(1), Type: msg, TypeName: refType, Label: repeated},
					{Name: proto.String("by_name"), JsonName: proto.String("byName"), Number: proto.Int32(2), Type: msg, TypeName: proto.String(".numinance.ports.holder.v1.Holder.ByNameEntry"), Label: repeated},
					{Name: proto.String("wrapper"), JsonName: proto.String("wrapper"), Number: proto.Int32(3), Type: msg, TypeName: proto.String(".numinance.ports.holder.v1.Wrapper"), Label: optional, Options: hostOnly},
				},
				NestedType: []*descriptorpb.DescriptorProto{{
					Name: proto.String("ByNameEntry"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{Name: proto.String("key"), JsonName: proto.String("key"), Number: proto.Int32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), Label: optional},
						{Name: proto.String("value"), JsonName: proto.String("value"), Number: proto.Int32(2), Type: msg, TypeName: refType, Label: optional},
					},
					Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				}},
			},
			{Name: proto.String("Wrapper"), Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("inner"), JsonName: proto.String("inner"), Number: proto.Int32(1), Type: msg, TypeName: refType, Label: optional},
			}},
		},
	}
	fd, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("protodesc.NewFile(holder) = %v", err)
	}
	md := fd.Messages().ByName("Holder")
	m := dynamicpb.NewMessage(md)
	set := func(r *typesv1.Reference) protoreflect.Value {
		wire, err := proto.Marshal(r)
		if err != nil {
			t.Fatalf("proto.Marshal(Reference) = %v", err)
		}
		v := dynamicpb.NewMessage(md.Fields().ByName("refs").Message())
		if err := proto.Unmarshal(wire, v); err != nil {
			t.Fatalf("proto.Unmarshal(Reference) = %v", err)
		}
		return protoreflect.ValueOfMessage(v)
	}
	for _, r := range refs {
		m.Mutable(md.Fields().ByName("refs")).List().Append(set(r))
	}
	for _, r := range byName {
		m.Mutable(md.Fields().ByName("by_name")).Map().Set(protoreflect.ValueOfString(r.GetArtifactRef()).MapKey(), set(r))
	}
	if inner != nil {
		w := dynamicpb.NewMessage(md.Fields().ByName("wrapper").Message())
		w.Set(w.Descriptor().Fields().ByName("inner"), set(inner))
		m.Set(md.Fields().ByName("wrapper"), protoreflect.ValueOfMessage(w))
	}
	return m
}

func TestCheckResolvable_RepeatedMapAndInherited(t *testing.T) {
	req := holder(t, []*typesv1.Reference{ref("artifacts/listed")}, []*typesv1.Reference{ref("artifacts/mapped")}, ref("artifacts/wrapped"))
	tests := []struct {
		name string
		ref  *typesv1.Reference
		want error
	}{
		{name: "repeated element", ref: ref("artifacts/listed")},
		{name: "map value", ref: ref("artifacts/mapped")},
		{name: "Reference under a host_only wrapper", ref: ref("artifacts/wrapped"), want: plugin.ErrReferenceHostOnly},
		{name: "unlisted", ref: ref("artifacts/none"), want: plugin.ErrReferenceNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := plugin.CheckResolvable(req, tt.ref); !errors.Is(err, tt.want) {
				t.Errorf("CheckResolvable(holder, %s) = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}

// TestCheckResolvable_HostBuiltRequest reads host_only through descriptors built
// from a serialized FileDescriptorSet and a dynamic request, as the host does.
func TestCheckResolvable_HostBuiltRequest(t *testing.T) {
	files := hostFiles(t, authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto)
	d, err := files.FindDescriptorByName("numinance.ports.authorexecutor.v1.ExecuteRequest")
	if err != nil {
		t.Fatalf("FindDescriptorByName(ExecuteRequest) = %v", err)
	}
	wire, err := proto.Marshal(executeRequest())
	if err != nil {
		t.Fatalf("proto.Marshal(ExecuteRequest) = %v", err)
	}
	req := dynamicpb.NewMessage(d.(protoreflect.MessageDescriptor))
	if err := proto.Unmarshal(wire, req); err != nil {
		t.Fatalf("proto.Unmarshal(dynamic ExecuteRequest) = %v", err)
	}
	for _, tt := range []struct {
		name string
		ref  *typesv1.Reference
		want error
	}{
		{name: "task_ref", ref: ref("artifacts/task")},
		{name: "worktree_ref", ref: ref("artifacts/worktree"), want: plugin.ErrReferenceHostOnly},
		{name: "network_policy_ref", ref: ref("artifacts/netpolicy"), want: plugin.ErrReferenceHostOnly},
	} {
		if err := plugin.CheckResolvable(req, tt.ref); !errors.Is(err, tt.want) {
			t.Errorf("CheckResolvable(dynamic request, %s) = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestCheckResolvable_NilRequest(t *testing.T) {
	for _, req := range []proto.Message{nil, (*authorexecutorv1.ExecuteRequest)(nil)} {
		if err := plugin.CheckResolvable(req, ref("artifacts/task")); !errors.Is(err, plugin.ErrReferenceNotFound) {
			t.Errorf("CheckResolvable(%T(nil), ref) = %v, want ErrReferenceNotFound", req, err)
		}
	}
}

// TestCheckResolvable_HostOnlyByDigest: the same artifact under another
// artifact_type or key is the same object, so it is refused too.
func TestCheckResolvable_HostOnlyByDigest(t *testing.T) {
	retyped := ref("artifacts/worktree")
	retyped.ArtifactType = "plan"
	rekeyed := ref("artifacts/worktree")
	rekeyed.ArtifactRef = "artifacts/other-key"
	for name, r := range map[string]*typesv1.Reference{"another artifact_type": retyped, "another artifact_ref": rekeyed} {
		if err := plugin.CheckResolvable(executeRequest(), r); !errors.Is(err, plugin.ErrReferenceHostOnly) {
			t.Errorf("CheckResolvable(worktree digest under %s) = %v, want ErrReferenceHostOnly", name, err)
		}
	}
	// A resolvable field whose digest differs from the host_only one still resolves.
	if err := plugin.CheckResolvable(executeRequest(), ref("artifacts/task")); err != nil {
		t.Errorf("CheckResolvable(task_ref) = %v, want nil", err)
	}
}

// TestCheckResolvable_UnresolvedHostOnlyOption decodes the descriptor with a
// resolver that lacks numinance.annotations.v1, so host_only stays in the
// unknown bytes (extension number 50002) of the field options. The extension is
// the whole FieldAnnotations message, which every port field carries for since.
func TestCheckResolvable_UnresolvedHostOnlyOption(t *testing.T) {
	req := holder(t, []*typesv1.Reference{ref("artifacts/listed")}, nil, ref("artifacts/wrapped"))
	wire, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("proto.Marshal(holder) = %v", err)
	}
	fdWire, err := proto.Marshal(protodesc.ToFileDescriptorProto(req.Descriptor().ParentFile()))
	if err != nil {
		t.Fatalf("proto.Marshal(FileDescriptorProto) = %v", err)
	}
	var fdp descriptorpb.FileDescriptorProto
	if err := (proto.UnmarshalOptions{Resolver: &protoregistry.Types{}}).Unmarshal(fdWire, &fdp); err != nil {
		t.Fatalf("Unmarshal(FileDescriptorProto, empty resolver) = %v", err)
	}
	fd, err := protodesc.NewFile(&fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("protodesc.NewFile = %v", err)
	}
	stripped := dynamicpb.NewMessage(fd.Messages().ByName("Holder"))
	if err := proto.Unmarshal(wire, stripped); err != nil {
		t.Fatalf("proto.Unmarshal(holder) = %v", err)
	}
	if got := stripped.Descriptor().Fields().ByName("wrapper").Options().ProtoReflect().GetUnknown(); len(got) == 0 {
		t.Fatalf("host_only option is not in unknown bytes; the test no longer builds the case")
	}
	if err := plugin.CheckResolvable(stripped, ref("artifacts/wrapped")); !errors.Is(err, plugin.ErrReferenceHostOnly) {
		t.Errorf("CheckResolvable(unresolved host_only option) = %v, want ErrReferenceHostOnly", err)
	}
	if err := plugin.CheckResolvable(stripped, ref("artifacts/listed")); err != nil {
		t.Errorf("CheckResolvable(listed, unresolved option on another field) = %v, want nil", err)
	}
}

// TestCheckResolvable_UncopyableReference: a dynamic message named like a
// Reference that does not copy into the generated type fails closed.
func TestCheckResolvable_UncopyableReference(t *testing.T) {
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
	required, optional := descriptorpb.FieldDescriptorProto_LABEL_REQUIRED.Enum(), descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	files := new(protoregistry.Files)
	register := func(fdp *descriptorpb.FileDescriptorProto) protoreflect.FileDescriptor {
		fd, err := protodesc.NewFile(fdp, files)
		if err != nil {
			t.Fatalf("protodesc.NewFile(%s) = %v", fdp.GetName(), err)
		}
		if err := files.RegisterFile(fd); err != nil {
			t.Fatalf("RegisterFile(%s) = %v", fdp.GetName(), err)
		}
		return fd
	}
	// A proto2 Reference with a required field: an empty one does not marshal.
	register(&descriptorpb.FileDescriptorProto{
		Name: proto.String("fake/types.proto"), Package: proto.String("numinance.types.v1"),
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("Reference"), Field: []*descriptorpb.FieldDescriptorProto{
			{Name: proto.String("artifact_ref"), Number: proto.Int32(1), Type: str, Label: required},
		}}},
	})
	fd := register(&descriptorpb.FileDescriptorProto{
		Name: proto.String("fake/req.proto"), Package: proto.String("fake"), Dependency: []string{"fake/types.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("Req"), Field: []*descriptorpb.FieldDescriptorProto{
			{Name: proto.String("r"), Number: proto.Int32(1), Type: msg, TypeName: proto.String(".numinance.types.v1.Reference"), Label: optional},
		}}},
	})
	md := fd.Messages().ByName("Req")
	req := dynamicpb.NewMessage(md)
	req.Set(md.Fields().ByName("r"), protoreflect.ValueOfMessage(dynamicpb.NewMessage(md.Fields().ByName("r").Message())))
	if err := plugin.CheckResolvable(req, ref("artifacts/task")); !errors.Is(err, plugin.ErrReferenceUnreadable) {
		t.Errorf("CheckResolvable(uncopyable dynamic Reference) = %v, want ErrReferenceUnreadable", err)
	}
}

// dynamicFile decodes fd's file descriptor with a dynamic extension resolver
// (dynamicpb.NewTypes), as a host that builds every type from a descriptor set
// does: the numinance options then decode as *dynamicpb.Message.
func dynamicFile(t *testing.T, fd protoreflect.FileDescriptor) protoreflect.FileDescriptor {
	t.Helper()
	wire, err := proto.Marshal(protodesc.ToFileDescriptorProto(fd))
	if err != nil {
		t.Fatalf("proto.Marshal(FileDescriptorProto) = %v", err)
	}
	var fdp descriptorpb.FileDescriptorProto
	if err := (proto.UnmarshalOptions{Resolver: dynamicpb.NewTypes(protoregistry.GlobalFiles)}).Unmarshal(wire, &fdp); err != nil {
		t.Fatalf("Unmarshal(FileDescriptorProto, dynamic resolver) = %v", err)
	}
	out, err := protodesc.NewFile(&fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("protodesc.NewFile = %v", err)
	}
	return out
}

func TestCheckResolvable_DynamicallyResolvedOptions(t *testing.T) {
	fd := dynamicFile(t, authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto)
	md := fd.Messages().ByName("ExecuteRequest")
	wire, err := proto.Marshal(executeRequest())
	if err != nil {
		t.Fatalf("proto.Marshal(ExecuteRequest) = %v", err)
	}
	req := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(wire, req); err != nil {
		t.Fatalf("proto.Unmarshal(ExecuteRequest) = %v", err)
	}
	for _, tt := range []struct {
		name string
		ref  *typesv1.Reference
		want error
	}{
		{name: "task_ref", ref: ref("artifacts/task")},
		{name: "worktree_ref", ref: ref("artifacts/worktree"), want: plugin.ErrReferenceHostOnly},
	} {
		if err := plugin.CheckResolvable(req, tt.ref); !errors.Is(err, tt.want) {
			t.Errorf("CheckResolvable(dynamic options, %s) = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestCheckResolvable_TypedNilDynamicRequest(t *testing.T) {
	var req *dynamicpb.Message
	if err := plugin.CheckResolvable(req, ref("artifacts/task")); !errors.Is(err, plugin.ErrReferenceNotFound) {
		t.Errorf("CheckResolvable((*dynamicpb.Message)(nil), ref) = %v, want ErrReferenceNotFound", err)
	}
}

func TestCheckResolvable_UnknownBytesAreUnreadable(t *testing.T) {
	req := executeRequest()
	req.Request.ProtoReflect().SetUnknown(protoreflect.RawFields{0xf8, 0x07, 0x01})
	if err := plugin.CheckResolvable(req, ref("artifacts/task")); !errors.Is(err, plugin.ErrReferenceUnreadable) {
		t.Errorf("CheckResolvable(unknown bytes under the request) = %v, want ErrReferenceUnreadable", err)
	}
	req = executeRequest()
	req.Request.TaskRef.ProtoReflect().SetUnknown(protoreflect.RawFields{0xf8, 0x07, 0x01})
	if err := plugin.CheckResolvable(req, ref("artifacts/task")); !errors.Is(err, plugin.ErrReferenceUnreadable) {
		t.Errorf("CheckResolvable(unknown bytes in a Reference) = %v, want ErrReferenceUnreadable", err)
	}
}

// withFieldOptions rebuilds req's type as a dynamic one whose file descriptor was
// decoded by decode, and returns req re-read into it.
func withFieldOptions(t *testing.T, req proto.Message, decode func(wire []byte, fdp *descriptorpb.FileDescriptorProto) error) *dynamicpb.Message {
	t.Helper()
	fdWire, err := proto.Marshal(protodesc.ToFileDescriptorProto(req.ProtoReflect().Descriptor().ParentFile()))
	if err != nil {
		t.Fatalf("proto.Marshal(FileDescriptorProto) = %v", err)
	}
	var fdp descriptorpb.FileDescriptorProto
	if err := decode(fdWire, &fdp); err != nil {
		t.Fatalf("decode(FileDescriptorProto) = %v", err)
	}
	fd, err := protodesc.NewFile(&fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("protodesc.NewFile = %v", err)
	}
	wire, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("proto.Marshal(request) = %v", err)
	}
	out := dynamicpb.NewMessage(fd.Messages().ByName(req.ProtoReflect().Descriptor().Name()))
	if err := proto.Unmarshal(wire, out); err != nil {
		t.Fatalf("proto.Unmarshal(request) = %v", err)
	}
	return out
}

// TestCheckResolvable_EmptyResolverRealPort: the options of the real port stay
// in unknown bytes with an empty resolver; since alone must not mark a field.
func TestCheckResolvable_EmptyResolverRealPort(t *testing.T) {
	req := withFieldOptions(t, executeRequest(), func(wire []byte, fdp *descriptorpb.FileDescriptorProto) error {
		return proto.UnmarshalOptions{Resolver: &protoregistry.Types{}}.Unmarshal(wire, fdp)
	})
	tests := []struct {
		name string
		ref  *typesv1.Reference
		want error
	}{
		{name: "task_ref carries only since", ref: ref("artifacts/task")},
		{name: "plan_ref carries only since", ref: ref("artifacts/plan")},
		{name: "worktree_ref is host_only", ref: ref("artifacts/worktree"), want: plugin.ErrReferenceHostOnly},
		{name: "network_policy_ref is host_only", ref: ref("artifacts/netpolicy"), want: plugin.ErrReferenceHostOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := plugin.CheckResolvable(req, tt.ref); !errors.Is(err, tt.want) {
				t.Errorf("CheckResolvable(empty resolver, %s) = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}

// TestCheckResolvable_MalformedOption: option bytes of number 50002 that do not
// parse as FieldAnnotations, or that carry host_only (field 5) with a wire type
// other than varint (it then sits in unknown bytes and reads false), count as marked.
func TestCheckResolvable_MalformedOption(t *testing.T) {
	annotations := func(build func(b []byte) []byte) []byte {
		inner := build(nil)
		b := protowire.AppendTag(nil, 50002, protowire.BytesType)
		return protowire.AppendBytes(b, inner)
	}
	tests := []struct {
		name string
		raw  []byte
	}{
		{name: "bytes that do not parse", raw: protowire.AppendBytes(protowire.AppendTag(nil, 50002, protowire.BytesType), []byte{0xff, 0xff, 0xff})},
		{name: "annotations encoded as a varint", raw: protowire.AppendVarint(protowire.AppendTag(nil, 50002, protowire.VarintType), 1)},
		{name: "host_only encoded as fixed32", raw: annotations(func(b []byte) []byte {
			return protowire.AppendFixed32(protowire.AppendTag(b, 5, protowire.Fixed32Type), 1)
		})},
		{name: "host_only encoded as bytes", raw: annotations(func(b []byte) []byte {
			return protowire.AppendBytes(protowire.AppendTag(b, 5, protowire.BytesType), []byte{1})
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := holder(t, []*typesv1.Reference{ref("artifacts/listed")}, nil, ref("artifacts/wrapped"))
			got := withFieldOptions(t, req, func(wire []byte, fdp *descriptorpb.FileDescriptorProto) error {
				if err := proto.Unmarshal(wire, fdp); err != nil {
					return err
				}
				opts := fdp.GetMessageType()[0].GetField()[2].GetOptions()
				opts.ProtoReflect().SetUnknown(tt.raw)
				proto.ClearExtension(opts, annotationsv1.E_Field)
				return nil
			})
			if err := plugin.CheckResolvable(got, ref("artifacts/wrapped")); !errors.Is(err, plugin.ErrReferenceHostOnly) {
				t.Errorf("CheckResolvable(%s) = %v, want ErrReferenceHostOnly", tt.name, err)
			}
		})
	}
}

// TestCheckResolvable_DuplicateOptionEntries: the host's own descriptor may hold
// several (field) entries, or several host_only entries inside one. Merge
// semantics would let a later host_only=false strip the mark, so more than one
// entry of either counts as marked.
func TestCheckResolvable_DuplicateOptionEntries(t *testing.T) {
	entry := func(build func(b []byte) []byte) []byte {
		b := protowire.AppendTag(nil, 50002, protowire.BytesType)
		return protowire.AppendBytes(b, build(nil))
	}
	hostOnly := func(v uint64) func(b []byte) []byte {
		return func(b []byte) []byte {
			return protowire.AppendVarint(protowire.AppendTag(b, 5, protowire.VarintType), v)
		}
	}
	since := func(b []byte) []byte {
		return protowire.AppendString(protowire.AppendTag(b, 1, protowire.BytesType), "1.0.0")
	}
	tests := []struct {
		name string
		raw  []byte
	}{
		{name: "two entries, the second unmarks", raw: append(entry(hostOnly(1)), entry(hostOnly(0))...)},
		{name: "regression guard (passes without the duplicate check): two entries, the second carries only since", raw: append(entry(hostOnly(1)), entry(since)...)},
		{name: "two host_only entries in one, the last false", raw: entry(func(b []byte) []byte { return hostOnly(0)(hostOnly(1)(b)) })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := holder(t, []*typesv1.Reference{ref("artifacts/listed")}, nil, ref("artifacts/wrapped"))
			got := withFieldOptions(t, req, func(wire []byte, fdp *descriptorpb.FileDescriptorProto) error {
				if err := proto.Unmarshal(wire, fdp); err != nil {
					return err
				}
				opts := fdp.GetMessageType()[0].GetField()[2].GetOptions()
				opts.ProtoReflect().SetUnknown(tt.raw)
				proto.ClearExtension(opts, annotationsv1.E_Field)
				return nil
			})
			if err := plugin.CheckResolvable(got, ref("artifacts/wrapped")); !errors.Is(err, plugin.ErrReferenceHostOnly) {
				t.Errorf("CheckResolvable(%s) = %v, want ErrReferenceHostOnly", tt.name, err)
			}
			if err := plugin.CheckResolvable(got, ref("artifacts/listed")); err != nil {
				t.Errorf("CheckResolvable(%s, listed) = %v, want nil", tt.name, err)
			}
		})
	}
}

// TestCheckResolvable_DuplicateOptionsMergedOnResolvedPath pins a limitation: a
// descriptor decoded with numinance.annotations.v1 linked has merged duplicate
// (field) entries before Options() returns, so a later host_only=false strips the
// mark and the runtime check cannot see it. The build-time annotation lint
// (FIELD_OPTION_DUPLICATE) and DuplicateFieldAnnotationsWire on the raw bytes
// cover that path; this test fails when the behaviour changes so the docs follow.
func TestCheckResolvable_DuplicateOptionsMergedOnResolvedPath(t *testing.T) {
	entry := func(v uint64) []byte {
		inner := protowire.AppendVarint(protowire.AppendTag(nil, 5, protowire.VarintType), v)
		return protowire.AppendBytes(protowire.AppendTag(nil, 50002, protowire.BytesType), inner)
	}
	raw := append(entry(1), entry(0)...)
	if !plugin.DuplicateFieldAnnotationsWire(raw) {
		t.Fatalf("DuplicateFieldAnnotationsWire(two entries) = false, want true")
	}
	req := holder(t, []*typesv1.Reference{ref("artifacts/listed")}, nil, ref("artifacts/wrapped"))
	got := withFieldOptions(t, req, func(wire []byte, fdp *descriptorpb.FileDescriptorProto) error {
		if err := proto.Unmarshal(wire, fdp); err != nil {
			return err
		}
		// The default resolver links the annotations, so the entries merge here.
		opts := new(descriptorpb.FieldOptions)
		if err := proto.Unmarshal(raw, opts); err != nil {
			return err
		}
		fdp.GetMessageType()[0].GetField()[2].Options = opts
		return nil
	})
	if err := plugin.CheckResolvable(got, ref("artifacts/wrapped")); err != nil {
		t.Errorf("CheckResolvable(merged duplicate options) = %v, want nil (the mark is gone after the merge)", err)
	}
}

func TestCheckResolvable_HostOnlyWinsOverUnreadable(t *testing.T) {
	req := executeRequest()
	req.Request.PlanRef.ProtoReflect().SetUnknown(protoreflect.RawFields{0xf8, 0x07, 0x01})
	tests := []struct {
		name string
		ref  *typesv1.Reference
		want error
	}{
		{name: "host_only match with an unreadable Reference present", ref: ref("artifacts/worktree"), want: plugin.ErrReferenceHostOnly},
		{name: "any other reference with an unreadable Reference present", ref: ref("artifacts/task"), want: plugin.ErrReferenceUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := plugin.CheckResolvable(req, tt.ref); !errors.Is(err, tt.want) {
				t.Errorf("CheckResolvable(%s) = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}
