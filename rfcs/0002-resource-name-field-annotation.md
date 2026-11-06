# RFC-0002: resource_name field annotation

- Status: accepted (2026-10-31)
- Author: orchestrator
- Opened: 2026-10-31
- Change class: contract-additive and security-sensitive (touches `numinance/annotations/v1`, a protected file, so it needs this RFC and a security review)

## Problem
The policy input's `action.resource_scope[]` is checked against the acting grant's resource scope before the engine runs. Today the Engine states it beside the request, so a call can touch a resource the list does not name. The action's risk floor and side-effect flag already derive from the annotated RPC; the resources a call touches do not, because the annotation names no request field.

## Proposal
Add `bool resource_name = 6` (since 1.0.0) to `FieldAnnotations`. A string or repeated string field marked `resource_name` carries resource names in the spelling of a grant's resource scope. The Engine reads the marks from the descriptor of the RPC's input message, decodes the request message it dispatches, and takes `action.resource_scope` from the marked values at any depth through singular and repeated message fields (bounded depth, bounded count). A caller-stated scope is ignored. The annotation lint (`RESOURCE_NAME_INVALID`) accepts the mark only on a `string` or `repeated string` field, of a message that an RPC input reaches, in a file of a `numinance.ports.*` package. The mark follows the field, so a rename or a move keeps it (the reason RFC-0001 rejected a method-level list of field paths).

## Alternatives
- A method-level `repeated string resource_fields` in `ActionOptions`: names fields by path, so a rename or a nested move silently unmarks them (RFC-0001).
- Keep the Engine's resolution of the request: the Engine would trust agent-influenced values and the policy input would not be a function of the annotated RPC and the request.
- A CEL expression per method: a language in the protocol for a list of names.

## Compatibility and migration
Additive: an optional field with a false default, in a package not yet tagged. A consumer that does not know the option ignores it. A port that names resources must mark the fields at introduction; a port that marks none declares that its calls touch no resource, and the grant check then has nothing to compare. No registered port marks a field today, so no behavior of a registered port changes.

## Security considerations
The request bytes the policy decides are the bytes the Engine dispatches (the plugin host owes that assertion). An undecodable request, a request with unknown fields at any depth, an empty request to a method that marks fields, a request over the byte bound, a nesting deeper than the bound or more marked values than the bound is a recorded deny, never an empty scope; a marked method called with no resource name is a deny too, so a port never marks an optional filter field. The walk follows singular, repeated, group and map-value message fields, as the lint's reachability does. An empty marked string names nothing and is never defaulted to an implicit resource. Messages never echo a value. The mark cannot be set by a request: it lives in the descriptor of the registered port. Tests: `adapters/descriptors`, the `Decide` cases in `policy/app/commands`, and the annotation lint fixture.

## Port authoring checklist
A port that introduces an RPC says in its comment whether the call names resources: it marks the fields that name them, or states that the call touches none. An unmarked method derives an empty scope, so a missing mark is a missing check; a reviewer of a new port asks for one or the other. (A registration rule that refuses an unmarked method is a later hardening; it needs a way for a method to declare "touches no resource", which the annotation does not have.)

## Open questions
Whether a future port needs a resource that is not a string (a typed reference); not needed now.
