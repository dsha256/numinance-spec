package plugin_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	authorexecutorv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

const (
	running   = pluginv1.InvokeStatus_INVOKE_STATUS_RUNNING
	succeeded = pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED
	failed    = pluginv1.InvokeStatus_INVOKE_STATUS_FAILED
)

// fakeRule accepts every stream message and demands one terminal status.
type fakeRule struct {
	items int
	want  pluginv1.InvokeStatus
}

func (r *fakeRule) Item(protoreflect.Message) error { r.items++; return nil }
func (r *fakeRule) Terminal() (pluginv1.InvokeStatus, error) {
	return r.want, nil
}

func methodOf(t *testing.T, fd protoreflect.FileDescriptor, svc, name string) protoreflect.MethodDescriptor {
	t.Helper()
	s := fd.Services().ByName(protoreflect.Name(svc))
	if s == nil {
		t.Fatalf("service %s not found", svc)
	}
	m := s.Methods().ByName(protoreflect.Name(name))
	if m == nil {
		t.Fatalf("method %s.%s not found", svc, name)
	}
	return m
}

func executeMethod(t *testing.T) protoreflect.MethodDescriptor {
	t.Helper()
	return methodOf(t, authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto, "AuthorExecutorService", "Execute")
}

func runMethod(t *testing.T) protoreflect.MethodDescriptor {
	t.Helper()
	return methodOf(t, verifierv1.File_numinance_ports_verifier_v1_verifier_proto, "VerifierService", "Run")
}

func eventMsg() *authorexecutorv1.ExecuteResponse {
	return &authorexecutorv1.ExecuteResponse{Event: &authorexecutorv1.ExecutionEvent{
		Type:       authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_STARTED,
		Sequence:   1,
		OccurredAt: timestamppb.New(time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)),
	}}
}

func result(t *testing.T, status pluginv1.InvokeStatus, m proto.Message) *pluginv1.ActionResult {
	t.Helper()
	a, err := anypb.New(m)
	if err != nil {
		t.Fatalf("anypb.New = %v", err)
	}
	return &pluginv1.ActionResult{Status: status, Result: a}
}

func bare(status pluginv1.InvokeStatus) *pluginv1.ActionResult {
	return &pluginv1.ActionResult{Status: status, Duration: durationpb.New(time.Second)}
}

// unknownField is field 127, varint 1: a field no message of the protocol defines.
func unknownField() protoreflect.RawFields { return protoreflect.RawFields{0xf8, 0x07, 0x01} }

func checkErr(t *testing.T, call string, err error, want string) {
	t.Helper()
	switch {
	case want == "" && err != nil:
		t.Errorf("%s = %v, want nil", call, err)
	case want != "" && (err == nil || !errors.Is(err, plugin.ErrInvokeStream) || !strings.Contains(err.Error(), want)):
		t.Errorf("%s = %v, want ErrInvokeStream containing %q", call, err, want)
	}
}

func TestValidateInvokeStream_StreamingTarget(t *testing.T) {
	item := func() *pluginv1.ActionResult { return result(t, running, eventMsg()) }
	nested := eventMsg()
	nested.Event.ProtoReflect().SetUnknown(unknownField())
	envelope := item()
	envelope.ProtoReflect().SetUnknown(unknownField())
	bloated := item()
	bloated.Result.Value = append(bloated.Result.Value, make([]byte, 64<<10)...)
	badWire := item()
	badWire.Result.Value = []byte{0xff}
	terminalWithResult := bare(succeeded)
	terminalWithResult.Result = item().Result
	costOnRunning := item()
	costOnRunning.Cost = &pluginv1.ResourceBudget{CpuMillicores: 1}
	noResult := &pluginv1.ActionResult{Status: running}
	otherType := result(t, running, &pluginv1.HealthResponse{})
	many := make([]*pluginv1.ActionResult, 0, 1026)
	for range 1025 {
		many = append(many, item())
	}

	tests := []struct {
		name    string
		msgs    []*pluginv1.ActionResult
		want    pluginv1.InvokeStatus
		wantErr string
	}{
		{name: "items then the status the rule names", msgs: []*pluginv1.ActionResult{item(), item(), bare(succeeded)}, want: succeeded},
		{name: "terminal status the rule does not name", msgs: []*pluginv1.ActionResult{item(), bare(failed)}, want: succeeded, wantErr: "disagrees"},
		{name: "provider rejection with no item ends FAILED", msgs: []*pluginv1.ActionResult{bare(failed)}, want: succeeded},
		{name: "no item and SUCCEEDED", msgs: []*pluginv1.ActionResult{bare(succeeded)}, want: succeeded, wantErr: "ends FAILED"},
		{name: "terminal message carries a result", msgs: []*pluginv1.ActionResult{item(), terminalWithResult}, want: succeeded, wantErr: "carries no result"},
		{name: "RUNNING without a result", msgs: []*pluginv1.ActionResult{noResult, bare(failed)}, want: failed, wantErr: "output type"},
		{name: "result of another type", msgs: []*pluginv1.ActionResult{otherType, bare(failed)}, want: failed, wantErr: "output type"},
		{name: "stream ends on RUNNING", msgs: []*pluginv1.ActionResult{item()}, want: succeeded, wantErr: "without a terminal message"},
		{name: "empty stream", msgs: nil, want: succeeded, wantErr: "without a terminal message"},
		{name: "message after the terminal one", msgs: []*pluginv1.ActionResult{item(), bare(succeeded), item()}, want: succeeded, wantErr: "after the terminal message"},
		{name: "unspecified status", msgs: []*pluginv1.ActionResult{{}}, want: failed, wantErr: "zero or unknown status"},
		{name: "terminal-only field on RUNNING", msgs: []*pluginv1.ActionResult{costOnRunning, bare(succeeded)}, want: succeeded, wantErr: "terminal-only"},
		{name: "wire bytes that do not parse", msgs: []*pluginv1.ActionResult{badWire, bare(failed)}, want: failed, wantErr: "does not decode"},
		{name: "unknown field inside a nested message of the result", msgs: []*pluginv1.ActionResult{result(t, running, nested), bare(failed)}, want: failed, wantErr: "unknown field"},
		{name: "unknown field on the ActionResult envelope", msgs: []*pluginv1.ActionResult{envelope, bare(failed)}, want: failed, wantErr: "unknown field"},
		{name: "RUNNING message over 64 KiB", msgs: []*pluginv1.ActionResult{bloated, bare(failed)}, want: failed, wantErr: "65536 bytes"},
		{name: "more than 1024 messages", msgs: append(many, bare(failed)), want: failed, wantErr: "over 1024"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.ValidateInvokeStream(executeMethod(t), tt.msgs, &fakeRule{want: tt.want})
			checkErr(t, "ValidateInvokeStream("+tt.name+")", err, tt.wantErr)
		})
	}
}

func TestStreamCheck_Incremental(t *testing.T) {
	c, err := plugin.NewStreamCheck(executeMethod(t), &fakeRule{want: failed})
	if err != nil {
		t.Fatalf("NewStreamCheck(Execute) = %v", err)
	}
	if err := c.Next(result(t, running, eventMsg())); err != nil {
		t.Fatalf("Next(item) = %v, want nil with no lookahead", err)
	}
	bad := bare(succeeded)
	checkErr(t, "Next(wrong terminal status)", c.Next(bad), "disagrees")
	checkErr(t, "End() after a failure", c.End(), "disagrees")
	checkErr(t, "Next after a failure", c.Next(bare(failed)), "disagrees")
}

func TestValidateInvokeStream_NonStreamingTarget(t *testing.T) {
	ok := func(status pluginv1.InvokeStatus) *pluginv1.ActionResult {
		return result(t, status, &verifierv1.RunResponse{})
	}
	badWire := ok(succeeded)
	badWire.Result.Value = []byte{0xff}
	unknown := ok(succeeded)
	unknown.Result.Value = append(unknown.Result.Value, unknownField()...)
	capabilities := methodOf(t, authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto, "AuthorExecutorService", "Capabilities")
	capsOK := result(t, succeeded, &authorexecutorv1.CapabilitiesResponse{})

	tests := []struct {
		name    string
		method  protoreflect.MethodDescriptor
		msgs    []*pluginv1.ActionResult
		wantErr string
	}{
		{name: "RUNNING without result then SUCCEEDED", method: runMethod(t),
			msgs: []*pluginv1.ActionResult{{Status: running}, ok(succeeded)}},
		{name: "RUNNING with a result", method: runMethod(t),
			msgs: []*pluginv1.ActionResult{ok(running), ok(succeeded)}, wantErr: "not server-streaming"},
		{name: "SUCCEEDED without a result", method: runMethod(t),
			msgs: []*pluginv1.ActionResult{bare(succeeded)}, wantErr: "required on SUCCEEDED"},
		{name: "FAILED with a result", method: runMethod(t),
			msgs: []*pluginv1.ActionResult{ok(failed)}, wantErr: "absent otherwise"},
		{name: "terminal result that does not parse", method: runMethod(t),
			msgs: []*pluginv1.ActionResult{badWire}, wantErr: "does not decode"},
		{name: "terminal result with an unknown field", method: runMethod(t),
			msgs: []*pluginv1.ActionResult{unknown}, wantErr: "unknown field"},
		{name: "terminal result of another type", method: runMethod(t),
			msgs: []*pluginv1.ActionResult{result(t, succeeded, &pluginv1.HealthResponse{})}, wantErr: "output type"},
		{name: "SYNC target: one message", method: capabilities, msgs: []*pluginv1.ActionResult{capsOK}},
		{name: "SYNC target: a RUNNING message", method: capabilities,
			msgs: []*pluginv1.ActionResult{result(t, running, &authorexecutorv1.CapabilitiesResponse{}), capsOK}, wantErr: "not OPERATION"},
		{name: "SYNC target: two messages", method: capabilities,
			msgs: []*pluginv1.ActionResult{capsOK, capsOK}, wantErr: "after the terminal message"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkErr(t, "ValidateInvokeStream("+tt.name+")", plugin.ValidateInvokeStream(tt.method, tt.msgs, nil), tt.wantErr)
		})
	}
}

func TestNewStreamCheck_Refusals(t *testing.T) {
	clientStream := testPort()
	clientStream.Service[0].Method[0].ClientStreaming = proto.Bool(true)
	fd, err := protodesc.NewFile(clientStream, new(protoregistry.Files))
	if err != nil {
		t.Fatalf("protodesc.NewFile = %v", err)
	}
	client := fd.Services().Get(0).Methods().Get(0)

	tests := []struct {
		name    string
		method  protoreflect.MethodDescriptor
		rule    plugin.PortRule
		wantErr string
	}{
		{name: "client-streaming target", method: client, wantErr: "client-streaming"},
		{name: "streaming target without a rule", method: executeMethod(t), wantErr: "port rule"},
		{name: "rule for a target that is not streaming", method: runMethod(t), rule: &fakeRule{}, wantErr: "not server-streaming"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := plugin.NewStreamCheck(tt.method, tt.rule)
			checkErr(t, "NewStreamCheck("+tt.name+")", err, tt.wantErr)
		})
	}
}

// TestStreamCheck_HostBuiltDescriptors builds the method as the host does: from
// a serialized FileDescriptorSet, never the compiled Go descriptors.
func TestStreamCheck_HostBuiltDescriptors(t *testing.T) {
	files := hostFiles(t, authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto)
	d, err := files.FindDescriptorByName("numinance.ports.authorexecutor.v1.AuthorExecutorService.Execute")
	if err != nil {
		t.Fatalf("FindDescriptorByName(Execute) = %v", err)
	}
	execute := d.(protoreflect.MethodDescriptor)
	msgs := []*pluginv1.ActionResult{result(t, running, eventMsg()), bare(succeeded)}
	checkErr(t, "ValidateInvokeStream(host-built Execute)", plugin.ValidateInvokeStream(execute, msgs, &fakeRule{want: succeeded}), "")
	checkErr(t, "ValidateInvokeStream(host-built Execute, wrong status)", plugin.ValidateInvokeStream(execute, msgs, &fakeRule{want: failed}), "disagrees")
	// The exec mode is read through the host-built options: a streaming target is refused without OPERATION.
	if _, err := plugin.NewStreamCheck(execute, nil); err == nil {
		t.Errorf("NewStreamCheck(host-built Execute, nil rule) = nil, want a refusal")
	}
}

// hostFiles round-trips fd and its imports through a serialized
// FileDescriptorSet and builds a registry from the decoded copy only.
func hostFiles(t *testing.T, fd protoreflect.FileDescriptor) *protoregistry.Files {
	t.Helper()
	set := &descriptorpb.FileDescriptorSet{}
	seen := map[string]bool{}
	var walk func(protoreflect.FileDescriptor)
	walk = func(f protoreflect.FileDescriptor) {
		if seen[f.Path()] {
			return
		}
		seen[f.Path()] = true
		for i := range f.Imports().Len() {
			walk(f.Imports().Get(i).FileDescriptor)
		}
		set.File = append(set.File, protodesc.ToFileDescriptorProto(f))
	}
	walk(fd)
	wire, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal(FileDescriptorSet) = %v", err)
	}
	var decoded descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("proto.Unmarshal(FileDescriptorSet) = %v", err)
	}
	files, err := protodesc.NewFiles(&decoded)
	if err != nil {
		t.Fatalf("protodesc.NewFiles = %v", err)
	}
	return files
}

// errItem is the sentinel errRule returns, to see it through the wrap.
var errItem = errors.New("item sentinel")

// errRule fails every Item with errItem.
type errRule struct{ fakeRule }

func (r *errRule) Item(protoreflect.Message) error { return errItem }

func TestStreamCheck_ZeroValueFailsClosed(t *testing.T) {
	// A plugin-sent empty type URL equals the zero wantURL and used to reach a nil output.
	for name, m := range map[string]*pluginv1.ActionResult{
		"empty type URL": {Status: succeeded, Result: &anypb.Any{}},
		"bare FAILED":    bare(failed),
		"bare RUNNING":   bare(running),
		"nil message":    nil,
		"unknown field 1": func() *pluginv1.ActionResult {
			r := bare(running)
			r.ProtoReflect().SetUnknown(unknownField())
			return r
		}(),
	} {
		var c plugin.StreamCheck
		err := c.Next(m)
		if !errors.Is(err, plugin.ErrInvokeStream) {
			t.Errorf("zero StreamCheck.Next(%s) = %v, want ErrInvokeStream", name, err)
		}
		if err := c.End(); !errors.Is(err, plugin.ErrInvokeStream) {
			t.Errorf("zero StreamCheck.End after %s = %v, want ErrInvokeStream", name, err)
		}
	}
	var c plugin.StreamCheck
	if err := c.End(); !errors.Is(err, plugin.ErrInvokeStream) || !strings.Contains(err.Error(), "not built by NewStreamCheck") {
		t.Errorf("zero StreamCheck.End() = %v, want ErrInvokeStream, not built by NewStreamCheck", err)
	}
}

func TestStreamCheck_PortRuleErrorIsWrapped(t *testing.T) {
	err := plugin.ValidateInvokeStream(executeMethod(t), []*pluginv1.ActionResult{
		result(t, running, eventMsg()), bare(succeeded),
	}, &errRule{})
	if !errors.Is(err, plugin.ErrInvokeStream) || !errors.Is(err, errItem) {
		t.Errorf("ValidateInvokeStream(rule error) = %v, want ErrInvokeStream wrapping the rule error", err)
	}
}

func TestNewStreamCheck_DynamicallyResolvedOptions(t *testing.T) {
	fd := dynamicFile(t, authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto)
	method := fd.Services().ByName("AuthorExecutorService").Methods().ByName("Execute")
	if _, err := plugin.NewStreamCheck(method, &fakeRule{}); err != nil {
		t.Errorf("NewStreamCheck(Execute, dynamic options) = %v, want nil", err)
	}
}

// endRule reports an unterminated stream, as authorexecutor does at the budget.
type endRule struct{ fakeRule }

var errUnterminated = errors.New("unterminated sentinel")

func (r *endRule) Unterminated() error { return errUnterminated }

func TestStreamCheck_EndAsksTheRule(t *testing.T) {
	err := plugin.ValidateInvokeStream(executeMethod(t), []*pluginv1.ActionResult{result(t, running, eventMsg())}, &endRule{})
	if !errors.Is(err, plugin.ErrInvokeStream) || !errors.Is(err, errUnterminated) || !strings.Contains(err.Error(), "without a terminal message") {
		t.Errorf("ValidateInvokeStream(unterminated) = %v, want ErrInvokeStream wrapping the rule error", err)
	}
}
