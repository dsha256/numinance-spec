package authorexecutor_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	authorexecutorv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
	"github.com/numinance/numinance-spec/sdk/go/ports/authorexecutor"
)

const (
	started    = authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_STARTED
	toolIntent = authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_TOOL_INTENT
	completed  = authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED
	failedEv   = authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_FAILED

	succeeded = pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED
	failed    = pluginv1.InvokeStatus_INVOKE_STATUS_FAILED
)

func executeMethod() protoreflect.MethodDescriptor {
	return authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto.
		Services().ByName("AuthorExecutorService").Methods().ByName("Execute")
}

func ev(typ authorexecutorv1.ExecutionEventType, seq uint64) *authorexecutorv1.ExecuteResponse {
	return &authorexecutorv1.ExecuteResponse{Event: &authorexecutorv1.ExecutionEvent{
		Type: typ, Sequence: seq, OccurredAt: timestamppb.New(time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)),
	}}
}

// stream is one RUNNING message per event and a terminal message with status.
func stream(t *testing.T, status pluginv1.InvokeStatus, events ...*authorexecutorv1.ExecuteResponse) []*pluginv1.ActionResult {
	t.Helper()
	out := make([]*pluginv1.ActionResult, 0, len(events)+1)
	for _, e := range events {
		a, err := anypb.New(e)
		if err != nil {
			t.Fatalf("anypb.New = %v", err)
		}
		out = append(out, &pluginv1.ActionResult{Status: pluginv1.InvokeStatus_INVOKE_STATUS_RUNNING, Result: a})
	}
	return append(out, &pluginv1.ActionResult{Status: status})
}

func TestExecuteStream_RoundTripThroughInvoke(t *testing.T) {
	sent := stream(t, succeeded, ev(started, 1), ev(toolIntent, 2), ev(completed, 3))
	got := make([]*pluginv1.ActionResult, 0, len(sent))
	for _, r := range sent {
		wire, err := proto.Marshal(&pluginv1.InvokeResponse{Result: r})
		if err != nil {
			t.Fatalf("proto.Marshal(InvokeResponse) = %v", err)
		}
		var resp pluginv1.InvokeResponse
		if err := proto.Unmarshal(wire, &resp); err != nil {
			t.Fatalf("proto.Unmarshal(InvokeResponse) = %v", err)
		}
		got = append(got, resp.GetResult())
	}
	if err := plugin.ValidateInvokeStream(executeMethod(), got, authorexecutor.NewExecuteRule()); err != nil {
		t.Fatalf("ValidateInvokeStream(Execute stream after the wire) = %v, want nil", err)
	}
	for i, r := range got[:3] {
		var event, want authorexecutorv1.ExecuteResponse
		if err := r.GetResult().UnmarshalTo(&event); err != nil {
			t.Fatalf("messages[%d].result.UnmarshalTo = %v", i, err)
		}
		if err := sent[i].GetResult().UnmarshalTo(&want); err != nil {
			t.Fatalf("sent[%d].result.UnmarshalTo = %v", i, err)
		}
		if !proto.Equal(&event, &want) {
			t.Errorf("event %d after the wire = %v, want %v", i, &event, &want)
		}
	}
}

func TestExecuteRule(t *testing.T) {
	big := ev(started, 1)
	big.Event.Subject = strings.Repeat("a", 32<<10)
	noTime := ev(started, 1)
	noTime.Event.OccurredAt = nil
	unknownType := ev(authorexecutorv1.ExecutionEventType(99), 2)
	noEvent := &authorexecutorv1.ExecuteResponse{}
	long := make([]*authorexecutorv1.ExecuteResponse, 0, 1024)
	long = append(long, ev(started, 1))
	for i := uint64(2); i <= 1023; i++ {
		long = append(long, ev(toolIntent, i))
	}

	tests := []struct {
		name    string
		status  pluginv1.InvokeStatus
		events  []*authorexecutorv1.ExecuteResponse
		wantErr string
	}{
		{name: "started, completed, SUCCEEDED", status: succeeded, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(completed, 2)}},
		{name: "started, failed, FAILED", status: failed, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(failedEv, 2)}},
		{name: "SUCCEEDED after EXECUTION_FAILED", status: succeeded, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(failedEv, 2)}, wantErr: "disagrees"},
		{name: "FAILED after EXECUTION_COMPLETED", status: failed, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(completed, 2)}, wantErr: "disagrees"},
		{name: "last event is not terminal", status: succeeded, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(toolIntent, 2)}, wantErr: "not a terminal event"},
		{name: "FAILED, COMPLETED, SUCCEEDED", status: succeeded, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(failedEv, 2), ev(completed, 3)}, wantErr: "event after a terminal event"},
		{name: "COMPLETED then another event", status: succeeded, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(completed, 2), ev(toolIntent, 3)}, wantErr: "event after a terminal event"},
		{name: "first event is not STARTED", status: failed, events: []*authorexecutorv1.ExecuteResponse{ev(toolIntent, 1), ev(failedEv, 2)}, wantErr: "not EXECUTION_STARTED"},
		{name: "first sequence is not 1", status: failed, events: []*authorexecutorv1.ExecuteResponse{ev(started, 2), ev(failedEv, 3)}, wantErr: "consecutive"},
		{name: "sequence gap", status: failed, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(failedEv, 3)}, wantErr: "consecutive"},
		{name: "repeated sequence", status: failed, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), ev(toolIntent, 1), ev(failedEv, 2)}, wantErr: "consecutive"},
		{name: "type unknown", status: failed, events: []*authorexecutorv1.ExecuteResponse{ev(started, 1), unknownType}, wantErr: "zero or unknown"},
		{name: "event unset", status: failed, events: []*authorexecutorv1.ExecuteResponse{noEvent}, wantErr: "event unset"},
		{name: "occurred_at unset", status: failed, events: []*authorexecutorv1.ExecuteResponse{noTime, ev(failedEv, 2)}, wantErr: "occurred_at"},
		{name: "ExecuteResponse over 32 KiB", status: failed, events: []*authorexecutorv1.ExecuteResponse{big, ev(failedEv, 2)}, wantErr: "over 32768 bytes"},
		{name: "1023 events without a terminal one, then FAILED", status: failed, events: long, wantErr: "not a terminal event"},
		{name: "a 1024th event", status: failed, events: append(long, ev(toolIntent, 1024)), wantErr: "over 1023 events"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.ValidateInvokeStream(executeMethod(), stream(t, tt.status, tt.events...), authorexecutor.NewExecuteRule())
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("ValidateInvokeStream(%s) = %v, want nil", tt.name, err)
			case tt.wantErr != "" && (err == nil || !errors.Is(err, plugin.ErrInvokeStream) || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("ValidateInvokeStream(%s) = %v, want ErrInvokeStream containing %q", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestExecuteRule_WrongMessageType(t *testing.T) {
	// Called straight, as a host that skipped the type URL check would.
	rule := authorexecutor.NewExecuteRule()
	msg := (&authorexecutorv1.CapabilitiesResponse{}).ProtoReflect()
	if err := rule.Item(msg); err == nil || !strings.Contains(err.Error(), "want numinance.ports.authorexecutor.v1.ExecuteResponse") {
		t.Errorf("Item(CapabilitiesResponse) = %v, want a refusal naming ExecuteResponse", err)
	}
}

func TestExecuteRule_BudgetExhausted(t *testing.T) {
	events := []*authorexecutorv1.ExecuteResponse{ev(started, 1)}
	for i := uint64(2); i <= 1023; i++ {
		events = append(events, ev(toolIntent, i))
	}
	err := plugin.ValidateInvokeStream(executeMethod(), stream(t, failed, events...), authorexecutor.NewExecuteRule())
	if !errors.Is(err, authorexecutor.ErrBudgetExhausted) || !errors.Is(err, plugin.ErrInvokeStream) {
		t.Errorf("ValidateInvokeStream(1023 events, no terminal) = %v, want ErrBudgetExhausted", err)
	}
	over := append(slices.Clone(events), ev(toolIntent, 1024))
	err = plugin.ValidateInvokeStream(executeMethod(), stream(t, failed, over...), authorexecutor.NewExecuteRule())
	if !errors.Is(err, authorexecutor.ErrEventBound) || errors.Is(err, authorexecutor.ErrBudgetExhausted) {
		t.Errorf("ValidateInvokeStream(1024th event) = %v, want ErrEventBound and not ErrBudgetExhausted", err)
	}
	if err := plugin.ValidateInvokeStream(executeMethod(), stream(t, failed, events...)[:len(events)], authorexecutor.NewExecuteRule()); !errors.Is(err, authorexecutor.ErrBudgetExhausted) {
		t.Errorf("ValidateInvokeStream(1023 events, no terminal message) = %v, want ErrBudgetExhausted", err)
	}
	short := stream(t, failed, ev(started, 1), ev(toolIntent, 2))
	if err := plugin.ValidateInvokeStream(executeMethod(), short, authorexecutor.NewExecuteRule()); errors.Is(err, authorexecutor.ErrBudgetExhausted) {
		t.Errorf("ValidateInvokeStream(2 events, no terminal) = %v, want a refusal that is not ErrBudgetExhausted", err)
	}
}
