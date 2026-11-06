// Package plugin holds the reference checks of plugin/v1 rules that a host, a
// provider and a conformance kit apply the same way: the shape of an Invoke
// stream (StreamCheck) and the references HostService.ResolveReference may
// serve (CheckResolvable). It imports no port; a port supplies a PortRule (see
// sdk/go/ports/authorexecutor). The host's own copy of the port descriptors is
// the authority: the checks take descriptors and messages as arguments and read
// the numinance annotations from them.
//
// Decoding: callers decode an ActionResult with unknown-preserving options (the
// default of proto.Unmarshal, never DiscardUnknown), because the checks reject
// unknown fields at any depth of the envelope and of every decoded result; they
// do not look inside an Any other than the result they decode.
//
// What StreamCheck checks: message count (at most 1024), the 64 KiB bound of a
// RUNNING message (its whole serialized ActionResult) and the 4 MiB bound of the
// terminal one, RUNNING and terminal placement, result presence and type, strict
// decoding, terminal-only fields on RUNNING, and the port rule.
//
// What the host must still check on every ActionResult (envelope.proto), because
// they need the host's registry or state: at most 64 diagnostics per invocation,
// a phase reason in a Diagnostic, field_path and subject resolution, evidence_ids
// issued for this invocation, side_effects against the declared side_effect,
// printable strings, string and repeated bounds, duplicates, negative duration,
// and the compensation token bound.
package plugin
