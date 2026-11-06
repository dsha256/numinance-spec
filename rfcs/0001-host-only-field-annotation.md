# RFC-0001: host_only field annotation

- Status: accepted (2026-10-22)
- Author: spec-steward agent
- Opened: 2026-10-22
- Change class: contract-additive (touches `numinance/annotations/v1`, a protected file, so it needs this RFC and a security review)

## Problem
`HostService.ResolveReference` lets a plugin read an artifact named by a Reference in the request it serves. Two references of the authorexecutor request, `worktree_ref` and `network_policy_ref`, must never reach the provider: the first is the mounted worktree and the second the egress allow-list the proxy enforces. The host reads options from descriptors and never parses proto text (annotations.proto header), so a comment that says "host-only" cannot be enforced or tested.

## Proposal
Add `bool host_only = 5` (since 1.0.0) to `FieldAnnotations`. The host and the SDK refuse `ResolveReference` for a Reference whose `sha256.value` equals that of a `host_only` field (or of a Reference nested under one), and the mark is inherited by every Reference nested under a marked message-typed field. `authorexecutor` sets it on `worktree_ref` and `network_policy_ref`. The annotation lint (`HOST_ONLY_INVALID`) accepts it only, in a message that an RPC input of the set reaches, on a `numinance.types.v1.Reference` field, or a message-typed wrapper that holds a Reference, of a file in a `numinance.ports.*` package.

## Alternatives
- Comments only: unenforceable and untestable, and the host never reads comments.
- A method-level list in `ActionOptions` (for example `host_only_fields: ["request.worktree_ref"]`): names fields by path, so a rename or a nested move silently unmarks them, and it puts a field-level fact on the method.
- A hard-coded list in the SDK or host: the core would know a port's field names, against the rule that a port owns its own meaning.

## Compatibility and migration
Additive: an optional field with a false default, in a package not yet tagged. Consumers that do not know the option ignore it. Marking a field later is a behavior change for providers that resolved it, so ports must mark at introduction.

## Security considerations
Refusal is by artifact identity: a reference whose `sha256.value` equals that of a `host_only` field is refused even when a resolvable field of the same request holds the same value, and even when its `artifact_type` or `artifact_ref` differs, so a task that reuses the worktree artifact as `task_ref`, or names it under another key, does not open it. The SDK check fails closed: the option is read through the generated annotations whatever resolver built the descriptor, so a descriptor decoded without `numinance.annotations.v1` or with a dynamic resolver reads the same (a field that carries only `since` is not marked); an option that does not decode counts as marked, and a Reference that cannot be read refuses the call. So does any message under the request that carries unknown bytes. Host preconditions: the host validates the canonical form of `Digest.value` before the request exists, since the comparison is by string; a refusal of a reference that a resolvable field also holds is final and not retryable and goes into evidence; the engine must not build a request in which a resolvable field shares a digest with a `host_only` one. The host streams the exact bytes whose sha256 it verified before the first chunk (a pinned object version or a buffered copy), because stores key by `artifact_ref` and refusal by digest identity is only as strong as that check. Duplicate `(field)` entries, or more than one `host_only` entry in one, would let a later `false` strip the mark under merge semantics. The SDK counts them only where they survive as unknown bytes (a descriptor decoded without `numinance.annotations.v1`; `plugin.DuplicateFieldAnnotationsWire` serves hosts that load descriptor sets at runtime); a descriptor decoded with the annotations linked has merged them already. The build-time annotation lint (`FIELD_OPTION_DUPLICATE`, over the `buf build` image decoded with an empty resolver) rejects them in the spec module. A Reference inside an `Any` is never resolvable. Tests: `sdk/go/plugin` `CheckResolvable`, and the lint fixture.

## Open questions
Whether a future port needs a host-only value that is not a Reference (a plain string key); not needed now.
