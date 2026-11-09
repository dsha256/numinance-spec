package provider

import (
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// CallError is a call that ends FAILED with one reason and one fixed message. The message is a constant of this file, one per
// class or per violated bound: it never holds the project, the key, the offending value or any text the tracker returned, and it
// stays in the closed vocabulary of the conformance kit (ASCII letters, digits, a space and a few punctuation marks). The wire
// carries the reason only (a Diagnostic has no message); the message is what a test, a log line and the kit read.
type CallError struct {
	Reason  pluginv1.Reason
	Message string
}

// Error implements error; it is the fixed message.
func (e *CallError) Error() string { return e.Message }

// Unwrap lets the shared server see the reason and end the call FAILED with it.
func (e *CallError) Unwrap() error { return ghprovider.Fail(e.Reason) }

// The fixed messages. A rejection of a request names the field, never the value, and never the bound as a number: a number in
// a message can be a run of the offending value.
const (
	msgProject        = "invalid request: project is malformed"
	msgIssueKey       = "invalid request: issue key is malformed"
	msgBlockedProject = "invalid request: blocked project is malformed"
	msgBlockedKey     = "invalid request: blocked issue key is malformed"
	msgBodyBound      = "invalid request: max_body_bytes is out of range"
	msgResultsBound   = "invalid request: max_results is out of range"
	msgText           = "invalid request: search text is malformed"
	msgState          = "invalid request: state is not a known category"
	msgTarget         = "invalid request: target is not a known category"
	msgLinkType       = "invalid request: link type is not known"
	msgSelfLink       = "invalid request: an issue cannot block itself"
	msgComment        = "invalid request: comment body is malformed"
	msgKey            = "invalid request: an idempotency key is required"
	msgDryRun         = "invalid request: this verb has no dry run"

	msgNotFound    = "not found"
	msgNoTrans     = "no transition reaches the target category"
	msgRefused     = "the tracker refused the change because of the state of the issue"
	msgRateLimited = "tracker rate limit reached, retry later"
	msgUnavailable = "tracker unavailable, retry later"
	msgUpstream    = "unexpected answer from the tracker"
	msgOverLimit   = "tracker answer is over the size limit"
	msgTimeout     = "tracker call timed out"
	msgReadOnly    = "this build is read only, write-back is not enabled"
)

func fail(r pluginv1.Reason, msg string) error { return &CallError{Reason: r, Message: msg} }

func invalid(msg string) error { return fail(pluginv1.Reason_REASON_INVALID_ARGUMENT, msg) }

func notFound() error { return fail(pluginv1.Reason_REASON_NOT_FOUND, msgNotFound) }
