package plugin

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

// Bounds of an OPERATION stream (PluginService.Invoke): at most 1024 messages,
// and with a RUNNING message at most 64 KiB and the terminal one at most 4 MiB a
// stream holds at most 64 MiB of RUNNING messages plus the terminal one.
const (
	maxMessages      = 1024
	maxRunningBytes  = 64 << 10
	maxTerminalBytes = 4 << 20
	typeURLPrefix    = "type.googleapis.com/"
)

// ErrInvokeStream is wrapped by every rejection of a StreamCheck. A rejection
// names an index-based path and the rule, never a value.
var ErrInvokeStream = errors.New("invoke stream rejected")

// PortRule is the check a port adds for its server-streaming verb. A rule holds
// the state of one stream; use a new one per Invoke.
type PortRule interface {
	// Item sees each stream message, decoded with the method's output type, in
	// order. A non-nil error rejects the stream. The error names a rule, never a
	// value from the message: a plugin controls the message, and the error
	// reaches logs and evidence.
	Item(msg protoreflect.Message) error
	// Terminal is called when the terminal message arrives after at least one
	// stream message and returns the status that message must carry. An error
	// rejects the stream, for example when the last stream message is not one
	// the port names terminal.
	Terminal() (pluginv1.InvokeStatus, error)
}

// UnterminatedRule is an optional addition to a PortRule. StreamCheck.End calls
// it when the stream ends with stream messages and no terminal message; a
// non-nil result is wrapped into the rejection, so a rule can name a cause such
// as an exhausted budget.
type UnterminatedRule interface {
	PortRule
	// Unterminated returns the cause of a stream that ended without a terminal
	// message, or nil when the rule has none to name.
	Unterminated() error
}

// StreamCheck checks the ActionResult messages of one Invoke as they arrive, so
// a host never buffers the stream. For a server-streaming target each stream
// message rides one RUNNING message in result and the terminal message carries
// no result; for any other target a RUNNING message carries no result. The first
// failure is sticky: every later call returns it. A StreamCheck serves one
// goroutine, the one that reads one Invoke; it is not safe for concurrent use
// and a host must not share it.
// The zero value is not usable and rejects every message; build one with
// NewStreamCheck. A port rule that bounds message size measures the re-serialized
// message (the decoded one), not the bytes the plugin sent.
type StreamCheck struct {
	output    protoreflect.MessageDescriptor
	streaming bool
	operation bool
	rule      PortRule
	wantURL   string
	messages  int
	items     int
	done      bool
	err       error
}

// NewStreamCheck returns the check of one Invoke of method, a descriptor from
// the host's own copy. rule is required for a server-streaming method and
// refused for any other. A client-streaming method is refused: no port verb has
// that shape.
func NewStreamCheck(method protoreflect.MethodDescriptor, rule PortRule) (*StreamCheck, error) {
	if method.IsStreamingClient() {
		return nil, fmt.Errorf("%w: client-streaming target", ErrInvokeStream)
	}
	var methodOpts descriptorpb.MethodOptions
	if err := reload(method.Options(), &methodOpts); err != nil {
		return nil, fmt.Errorf("%w: method options are not readable", ErrInvokeStream)
	}
	opts, _ := proto.GetExtension(&methodOpts, annotationsv1.E_Action).(*annotationsv1.ActionOptions)
	c := &StreamCheck{
		output:    method.Output(),
		streaming: method.IsStreamingServer(),
		operation: opts.GetExec() == annotationsv1.ExecMode_EXEC_MODE_OPERATION,
		rule:      rule,
		wantURL:   typeURLPrefix + string(method.Output().FullName()),
	}
	switch {
	case c.streaming && (!c.operation || rule == nil):
		return nil, fmt.Errorf("%w: server-streaming target needs exec OPERATION and a port rule", ErrInvokeStream)
	case !c.streaming && rule != nil:
		return nil, fmt.Errorf("%w: port rule given for a target that is not server-streaming", ErrInvokeStream)
	}
	return c, nil
}

// Next checks one message of the stream, in arrival order.
func (c *StreamCheck) Next(m *pluginv1.ActionResult) error {
	if c.err == nil {
		c.err = c.next(m)
	}
	return c.err
}

// End checks that the stream is complete: the terminal message arrived.
func (c *StreamCheck) End() error {
	if c.err == nil && c.output == nil {
		c.err = reject("messages: the check was not built by NewStreamCheck")
	}
	if c.err == nil && !c.done {
		c.err = fmt.Errorf("%w: messages: the stream ended without a terminal message", ErrInvokeStream)
		if e, ok := c.rule.(UnterminatedRule); ok && c.items > 0 {
			if cause := e.Unterminated(); cause != nil {
				c.err = fmt.Errorf("%w: messages: the stream ended without a terminal message: %w", ErrInvokeStream, cause)
			}
		}
	}
	return c.err
}

func (c *StreamCheck) next(m *pluginv1.ActionResult) error {
	i := c.messages
	c.messages++
	switch {
	case c.output == nil:
		return reject("messages[%d]: the check was not built by NewStreamCheck", i)
	case c.done:
		return reject("messages[%d]: message after the terminal message", i)
	case c.messages > maxMessages:
		return reject("messages[%d]: over %d messages", i, maxMessages)
	case m == nil:
		return reject("messages[%d]: nil message", i)
	case hasUnknown(m.ProtoReflect()):
		return reject("messages[%d]: unknown field", i)
	}
	switch m.GetStatus() {
	case pluginv1.InvokeStatus_INVOKE_STATUS_RUNNING:
		return c.running(i, m)
	case pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED, pluginv1.InvokeStatus_INVOKE_STATUS_FAILED:
		return c.terminal(i, m)
	default:
		return reject("messages[%d].status: zero or unknown status", i)
	}
}

func (c *StreamCheck) running(i int, m *pluginv1.ActionResult) error {
	switch {
	case !c.operation:
		return reject("messages[%d].status: RUNNING on a target that is not OPERATION", i)
	case proto.Size(m) > maxRunningBytes:
		return reject("messages[%d]: RUNNING message over %d bytes", i, maxRunningBytes)
	case len(m.GetEvidenceIds()) > 0 || len(m.GetSideEffects()) > 0 || len(m.GetCompensationToken()) > 0 ||
		m.GetCost() != nil || m.GetDuration() != nil:
		return reject("messages[%d]: RUNNING message carries terminal-only fields", i)
	case !c.streaming:
		if m.GetResult() != nil {
			return reject("messages[%d].result: RUNNING result on a target that is not server-streaming", i)
		}
		return nil
	}
	item, err := c.decode(m.GetResult())
	if err != nil {
		return reject("messages[%d].result: %w", i, err)
	}
	c.items++
	if err := c.rule.Item(item); err != nil {
		return reject("messages[%d].result: %w", i, err)
	}
	return nil
}

func (c *StreamCheck) terminal(i int, m *pluginv1.ActionResult) error {
	if proto.Size(m) > maxTerminalBytes {
		return reject("messages[%d]: terminal message over %d bytes", i, maxTerminalBytes)
	}
	c.done = true
	succeeded := m.GetStatus() == pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED
	if !c.streaming {
		if succeeded != (m.GetResult() != nil) {
			return reject("messages[%d].result: required on SUCCEEDED, absent otherwise", i)
		}
		if succeeded {
			if _, err := c.decode(m.GetResult()); err != nil {
				return reject("messages[%d].result: %w", i, err)
			}
		}
		return nil
	}
	switch {
	case m.GetResult() != nil:
		return reject("messages[%d].result: the terminal message of a streaming target carries no result", i)
	case c.items == 0:
		// A provider rejection: no stream message, so it can only fail.
		if succeeded {
			return reject("messages[%d].status: a stream with no stream message ends FAILED", i)
		}
		return nil
	}
	want, err := c.rule.Terminal()
	if err != nil {
		return reject("messages[%d].status: %w", i, err)
	}
	if m.GetStatus() != want {
		return reject("messages[%d].status: disagrees with the last stream message", i)
	}
	return nil
}

// decode reads a result strictly: the type URL is the method's output type, the
// bytes parse, and no field at any depth is unknown.
func (c *StreamCheck) decode(a *anypb.Any) (protoreflect.Message, error) {
	if a.GetTypeUrl() != c.wantURL {
		return nil, errors.New("absent or not the method output type")
	}
	msg := dynamicpb.NewMessage(c.output)
	if err := proto.Unmarshal(a.GetValue(), msg); err != nil {
		return nil, errors.New("does not decode")
	}
	if hasUnknown(msg) {
		return nil, errors.New("unknown field")
	}
	return msg, nil
}

func reject(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvokeStream}, args...)...)
}

// ValidateInvokeStream checks a whole stream of messages with a new StreamCheck;
// it is the batch form of Next and End.
func ValidateInvokeStream(method protoreflect.MethodDescriptor, msgs []*pluginv1.ActionResult, rule PortRule) error {
	c, err := NewStreamCheck(method, rule)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		if err := c.Next(m); err != nil {
			return err
		}
	}
	return c.End()
}

// hasUnknown reports an unknown field in m or in any message under it, through
// singular fields, lists and map values.
func hasUnknown(m protoreflect.Message) bool {
	if len(m.GetUnknown()) > 0 {
		return true
	}
	found := false
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			if fd.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
					found = hasUnknown(mv.Message())
					return !found
				})
			}
		case fd.IsList():
			if fd.Message() != nil {
				for i := range v.List().Len() {
					if found = hasUnknown(v.List().Get(i).Message()); found {
						break
					}
				}
			}
		case fd.Message() != nil:
			found = hasUnknown(v.Message())
		}
		return !found
	})
	return found
}
