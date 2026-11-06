// Package authorexecutor holds the plugin protocol rule of the authorexecutor
// port's server-streaming verb Execute, for plugin.StreamCheck. The generic
// plugin package imports no port.
package authorexecutor

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	authorexecutorv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// Stream bounds of the port file comment.
const (
	maxEvents        = 1023
	maxResponseBytes = 32 << 10
)

// ErrBudgetExhausted is wrapped by every rejection that follows from the stream
// holding the most events the port allows without a terminal one: a terminal
// message after them, and the end of the stream after them. A 1024th event is
// not one of these: it breaks the bound and is a plain rule break. The port text
// says the execution context pauses and escalates on the cap, so a host tells
// that stop from a malformed stream with errors.Is.
var ErrBudgetExhausted = fmt.Errorf("the budget of %d events is exhausted", maxEvents)

// ErrEventBound is wrapped by the rejection of a 1024th event. It is a plain rule
// break, not a cap stop, and never matches ErrBudgetExhausted. A host classifies
// every error of this rule, ErrEventBound included, as a non-retryable rule break
// and counts it against the same retry and escalation budget as
// ErrBudgetExhausted; the sentinels differ only so a log or a test can tell the
// cases apart.
var ErrEventBound = fmt.Errorf("over %d events", maxEvents)

// responseName is checked before a stream message is reinterpreted as an
// ExecuteResponse.
const responseName protoreflect.FullName = "numinance.ports.authorexecutor.v1.ExecuteResponse"

// executeRule is the state of one Execute or Plan stream. The two verbs stream the same events; plan selects PlanResponse as the
// message and refuses FILE_CHANGED, which a read-only run never emits.
type executeRule struct {
	plan     bool
	events   uint64
	terminal authorexecutorv1.ExecutionEventType
}

// NewExecuteRule returns the plugin.PortRule of one Execute stream: 1 to 1023
// events, the first EXECUTION_STARTED at sequence 1, consecutive sequences, a
// known type and an occurred_at on each, each ExecuteResponse at most 32 KiB
// (measured on the re-serialized message), and EXECUTION_COMPLETED or
// EXECUTION_FAILED on the last event and only there.
// The terminal Invoke status is SUCCEEDED after EXECUTION_COMPLETED and FAILED
// after EXECUTION_FAILED. The checks that need the host's state (subject
// grammar, the diff, the broker, references) stay with the host (HOST-002).
func NewExecuteRule() plugin.PortRule { return &executeRule{} }

// executeRule is also a plugin.UnterminatedRule.
var _ plugin.UnterminatedRule = (*executeRule)(nil)

// Item checks one ExecuteResponse.
func (r *executeRule) Item(msg protoreflect.Message) error {
	want := responseName
	if r.plan {
		want = planResponseName
	}
	if msg.Descriptor().FullName() != want {
		return fmt.Errorf("stream message is %s, want %s", msg.Descriptor().FullName(), want)
	}
	if r.terminal != authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_UNSPECIFIED {
		return errors.New("event after a terminal event")
	}
	r.events++
	if r.events > maxEvents {
		// A plain error: a stream that sends a 1024th event broke the bound, it did
		// not stop at the cap, so errors.Is(ErrBudgetExhausted) stays false for it.
		return ErrEventBound
	}
	wire, err := proto.Marshal(msg.Interface())
	if err != nil {
		return errors.New("stream message does not marshal")
	}
	if len(wire) > maxResponseBytes {
		return fmt.Errorf("event over %d bytes", maxResponseBytes)
	}
	// The wire form is the same for both verbs; decode into the generated type of the verb.
	var resp interface {
		proto.Message
		GetEvent() *authorexecutorv1.ExecutionEvent
	} = &authorexecutorv1.ExecuteResponse{}
	if r.plan {
		resp = &authorexecutorv1.PlanResponse{}
	}
	if err := proto.Unmarshal(wire, resp); err != nil {
		return fmt.Errorf("stream message is not a %s", want.Name())
	}
	ev := resp.GetEvent()
	switch {
	case ev == nil:
		return errors.New("event unset")
	case authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_UNSPECIFIED.Descriptor().Values().ByNumber(ev.GetType().Number()) == nil ||
		ev.GetType() == authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_UNSPECIFIED:
		return errors.New("event type zero or unknown")
	case ev.GetSequence() != r.events:
		return errors.New("event sequence is not consecutive from 1")
	case r.events == 1 && ev.GetType() != authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_STARTED:
		return errors.New("first event is not EXECUTION_STARTED")
	case ev.GetOccurredAt() == nil:
		return errors.New("event occurred_at unset")
	case r.plan && ev.GetType() == authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_FILE_CHANGED:
		return errors.New("FILE_CHANGED in a Plan stream: a planning run is read only")
	}
	switch ev.GetType() {
	case authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED,
		authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_FAILED:
		r.terminal = ev.GetType()
	}
	return nil
}

// Unterminated implements plugin.UnterminatedRule: the budget, when the stream
// ended on the last event the port allows without a terminal one.
func (r *executeRule) Unterminated() error {
	if r.terminal == authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_UNSPECIFIED && r.events >= maxEvents {
		return ErrBudgetExhausted
	}
	return nil
}

// Terminal returns the Invoke status the terminal message must carry.
func (r *executeRule) Terminal() (pluginv1.InvokeStatus, error) {
	switch r.terminal {
	case authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED:
		return pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED, nil
	case authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_FAILED:
		return pluginv1.InvokeStatus_INVOKE_STATUS_FAILED, nil
	case authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_UNSPECIFIED:
		if r.events >= maxEvents {
			return pluginv1.InvokeStatus_INVOKE_STATUS_UNSPECIFIED, fmt.Errorf("last stream message is not a terminal event: %w", ErrBudgetExhausted)
		}
		fallthrough
	default:
		return pluginv1.InvokeStatus_INVOKE_STATUS_UNSPECIFIED, errors.New("last stream message is not a terminal event")
	}
}
