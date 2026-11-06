// Package plugin_test round-trips plugin/v1 Envelopes and resolves them against
// the FileDescriptorSet a plugin returns from GetDescriptors.
package plugin_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// No numinance.ports file exists yet, so the test builds a
// synthetic port file the way a plugin's descriptor set would carry one.
const (
	testPortFile = "numinance/ports/testport/v1/testport.proto"
	testMethod   = "/numinance.ports.testport.v1.TestPort/GetThing"
	portPrefix   = "numinance.ports."
)

const typeURLPrefix = "type.googleapis.com/"

var errEnvelope = errors.New("envelope rejected")

// testPort is a port file with one RPC, GetThing(GetThingRequest) returns (GetThingResponse).
func testPort() *descriptorpb.FileDescriptorProto {
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
	opt := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String(testPortFile),
		Package: proto.String("numinance.ports.testport.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("GetThingRequest"), Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("name"), JsonName: proto.String("name"), Number: proto.Int32(1), Type: str, Label: opt},
			}},
			{Name: proto.String("GetThingResponse")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("TestPort"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("GetThing"),
				InputType:  proto.String(".numinance.ports.testport.v1.GetThingRequest"),
				OutputType: proto.String(".numinance.ports.testport.v1.GetThingResponse"),
			}},
		}},
	}
}

// descriptorSet returns fd and its transitive imports, plus the test port, as a plugin would ship them.
func descriptorSet(fd protoreflect.FileDescriptor) *descriptorpb.FileDescriptorSet {
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{testPort()}}
	seen := map[string]bool{}
	var walk func(protoreflect.FileDescriptor)
	walk = func(f protoreflect.FileDescriptor) {
		if seen[f.Path()] {
			return
		}
		seen[f.Path()] = true
		imports := f.Imports()
		for i := range imports.Len() {
			walk(imports.Get(i).FileDescriptor)
		}
		set.File = append(set.File, protodesc.ToFileDescriptorProto(f))
	}
	walk(fd)
	return set
}

// filesFromGetDescriptors serializes the set through GetDescriptorsResponse, as
// on the wire, and builds a registry from the decoded copy only.
func filesFromGetDescriptors(t *testing.T) *protoregistry.Files {
	t.Helper()
	wire, err := proto.Marshal(&pluginv1.GetDescriptorsResponse{
		DescriptorSet: descriptorSet(pluginv1.File_numinance_plugin_v1_host_proto),
	})
	if err != nil {
		t.Fatalf("proto.Marshal(GetDescriptorsResponse) = %v", err)
	}
	var resp pluginv1.GetDescriptorsResponse
	if err := proto.Unmarshal(wire, &resp); err != nil {
		t.Fatalf("proto.Unmarshal(GetDescriptorsResponse) = %v", err)
	}
	files, err := protodesc.NewFiles(resp.GetDescriptorSet())
	if err != nil {
		t.Fatalf("protodesc.NewFiles = %v", err)
	}
	return files
}

// resolveEnvelope shows the SDK round trip: it checks env against files and decodes its request with the descriptor from
// files, never with the compiled Go type. It is not the host check: the host resolves against its own registry, never
// the plugin-supplied descriptors.
func resolveEnvelope(files *protoregistry.Files, env *pluginv1.Envelope) (*dynamicpb.Message, error) {
	svc, name, ok := strings.Cut(strings.TrimPrefix(env.GetMethod(), "/"), "/")
	if !ok || !strings.HasPrefix(env.GetMethod(), "/") {
		return nil, fmt.Errorf("%w: method is not /<package>.<Service>/<Method>", errEnvelope)
	}
	if !strings.HasPrefix(svc, portPrefix) {
		return nil, fmt.Errorf("%w: method is not in a port package", errEnvelope)
	}
	d, err := files.FindDescriptorByName(protoreflect.FullName(svc))
	if err != nil {
		return nil, fmt.Errorf("%w: service not in descriptor set", errEnvelope)
	}
	sd, ok := d.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%w: not a service", errEnvelope)
	}
	md := sd.Methods().ByName(protoreflect.Name(name))
	if md == nil {
		return nil, fmt.Errorf("%w: method not in service", errEnvelope)
	}
	if got, want := env.GetRequest().GetTypeUrl(), typeURLPrefix+string(md.Input().FullName()); got != want {
		return nil, fmt.Errorf("%w: request type is not the method input", errEnvelope)
	}
	msg := dynamicpb.NewMessage(md.Input())
	if err := proto.Unmarshal(env.GetRequest().GetValue(), msg); err != nil {
		return nil, fmt.Errorf("%w: request does not decode: %w", errEnvelope, err)
	}
	return msg, nil
}

// testRequest is GetThingRequest{name: "thing-1"} built from files, so its type is the descriptor-set copy.
func testRequest(t *testing.T, files *protoregistry.Files) *dynamicpb.Message {
	t.Helper()
	d, err := files.FindDescriptorByName("numinance.ports.testport.v1.GetThingRequest")
	if err != nil {
		t.Fatalf("FindDescriptorByName(GetThingRequest) = %v", err)
	}
	md, ok := d.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatalf("GetThingRequest is %T, want a message descriptor", d)
	}
	msg := dynamicpb.NewMessage(md)
	msg.Set(md.Fields().ByName("name"), protoreflect.ValueOfString("thing-1"))
	return msg
}

func testEnvelope(t *testing.T, req proto.Message) *pluginv1.Envelope {
	t.Helper()
	anyReq, err := anypb.New(req)
	if err != nil {
		t.Fatalf("anypb.New = %v", err)
	}
	return &pluginv1.Envelope{
		Method: testMethod,
		Context: &pluginv1.ActionContext{
			InvocationId:     "inv_" + strings.Repeat("0a", 16),
			ChangeId:         "chg_01",
			RunId:            "run_01",
			ActorKind:        typesv1.PrincipalKind_PRINCIPAL_KIND_AUTHOR_AGENT,
			ActorPrincipal:   "agent-1",
			GrantId:          "grant_01",
			PolicyDecisionId: "dec_01",
			PlanDigest:       &typesv1.Digest{Value: "sha256:" + strings.Repeat("a", 64)},
			ProfileDigest:    &typesv1.Digest{Value: "sha256:" + strings.Repeat("b", 64)},
			IdempotencyKey:   "key-1",
			Deadline:         timestamppb.New(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)),
			Budget:           &pluginv1.ResourceBudget{CpuMillicores: 500, MemoryBytes: 1 << 28, DiskBytes: 1 << 30},
			Residency:        typesv1.ResidencyClass_RESIDENCY_CLASS_IN_ACCOUNT,
			DryRun:           true,
		},
		Request: anyReq,
	}
}

func TestEnvelope_RoundTrip(t *testing.T) {
	files := filesFromGetDescriptors(t)
	req := testRequest(t, files)
	want := testEnvelope(t, req)
	types := dynamicpb.NewTypes(files)

	codecs := []struct {
		name      string
		marshal   func(proto.Message) ([]byte, error)
		unmarshal func([]byte, proto.Message) error
	}{
		{name: "binary", marshal: proto.Marshal, unmarshal: proto.Unmarshal},
		{name: "protojson", marshal: protojson.MarshalOptions{Resolver: types}.Marshal, unmarshal: protojson.UnmarshalOptions{Resolver: types}.Unmarshal},
	}
	for _, c := range codecs {
		t.Run(c.name, func(t *testing.T) {
			wire, err := c.marshal(want)
			if err != nil {
				t.Fatalf("marshal(Envelope) = %v", err)
			}
			got := &pluginv1.Envelope{}
			if err := c.unmarshal(wire, got); err != nil {
				t.Fatalf("unmarshal(Envelope) = %v", err)
			}
			if !proto.Equal(got, want) {
				t.Errorf("%s round trip of Envelope = %v, want %v", c.name, got, want)
			}

			msg, err := resolveEnvelope(files, got)
			if err != nil {
				t.Fatalf("resolveEnvelope(%s decoded Envelope) = %v, want nil", c.name, err)
			}
			det := proto.MarshalOptions{Deterministic: true}
			gotReq, err := det.Marshal(msg)
			if err != nil {
				t.Fatalf("Marshal(dynamic request) = %v", err)
			}
			wantReq, err := det.Marshal(req)
			if err != nil {
				t.Fatalf("Marshal(request) = %v", err)
			}
			if !bytes.Equal(gotReq, wantReq) {
				t.Errorf("resolved request = %x, want %x", gotReq, wantReq)
			}
		})
	}
}

func TestResolveEnvelope_Rejects(t *testing.T) {
	files := filesFromGetDescriptors(t)
	tests := []struct {
		name   string
		mutate func(*pluginv1.Envelope)
	}{
		{name: "unknown type URL", mutate: func(e *pluginv1.Envelope) {
			e.Request.TypeUrl = typeURLPrefix + "numinance.plugin.v1.NoSuchRequest"
		}},
		{name: "type URL of another message", mutate: func(e *pluginv1.Envelope) {
			e.Request.TypeUrl = typeURLPrefix + "numinance.ports.testport.v1.GetThingResponse"
		}},
		{name: "unknown method", mutate: func(e *pluginv1.Envelope) {
			e.Method = "/numinance.ports.testport.v1.TestPort/NoSuchMethod"
		}},
		{name: "service absent from the descriptor set", mutate: func(e *pluginv1.Envelope) {
			e.Method = "/numinance.ports.other.v1.Other/GetThing"
		}},
		{name: "method in plugin/v1, not a port", mutate: func(e *pluginv1.Envelope) {
			e.Method = "/numinance.plugin.v1.HostService/ReportProgress"
			e.Request.TypeUrl = typeURLPrefix + "numinance.plugin.v1.ReportProgressRequest"
		}},
		{name: "method without leading slash", mutate: func(e *pluginv1.Envelope) {
			e.Method = strings.TrimPrefix(testMethod, "/")
		}},
		{name: "message name as service", mutate: func(e *pluginv1.Envelope) {
			e.Method = "/numinance.ports.testport.v1.GetThingRequest/GetThing"
		}},
		{name: "request bytes that do not decode", mutate: func(e *pluginv1.Envelope) {
			e.Request.Value = []byte{0xff}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testEnvelope(t, testRequest(t, files))
			tt.mutate(env)
			if _, err := resolveEnvelope(files, env); !errors.Is(err, errEnvelope) {
				t.Errorf("resolveEnvelope(%s) = %v, want %v", tt.name, err, errEnvelope)
			}
		})
	}
}
