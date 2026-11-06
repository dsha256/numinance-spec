# verifier conformance kit, v1

Skeleton that runs. The acceptance tests for a provider of `numinance.ports.verifier.v1.VerifierService` (`proto/numinance/ports/verifier/v1/verifier.proto`). The kit version tracks the port minor; this skeleton tracks 1.0.0. The runnable skeleton is the Go package `verifierkit` in this directory (`kit.go`): it runs CORE-001, 002, 003, 005, 006, 008, 009, 010 and 014 against any `Target` with the fixtures a publisher supplies (`Subject`), and `ValidateResponse` is the port's response rules (HOST-005 and HOST-006) as a function a host or a test can apply to any provider. `kit_test.go` runs it against the replaying build of `plugins/official/verifier-python`; the verification context's acceptance (the reference implementation's host-level acceptance test) runs the same provider through a real launch and `Invoke`. The other cases need fixtures the skeleton does not carry (the 1025-violation candidate, the multi-line tool output, references the host cannot resolve, a nested sandbox) and stay in the table below as the list the kit must hold. Modes are those of ADR 0015: replay, live publisher-run in an attested runner, live registry-run.

Each case runs the provider with a config the provider's publisher supplies and a candidate from the kit: a clean candidate with no violation, a seeded candidate with known violations, one per severity the provider can report, and a crashing candidate whose content makes one check crash partway through. Every call is an OPERATION call through `PluginService.Invoke`, as the proto file comment maps it.

## Core

| Id | Verb | Case | Expect |
| --- | --- | --- | --- |
| CORE-001 | Run | the clean candidate | SUCCEEDED; no items; `summary` present with no counts |
| CORE-002 | Run | the seeded candidate | SUCCEEDED; every seeded violation reported with its `rule_id`, severity and `location`; counts equal the listed items per severity |
| CORE-003 | Run | the seeded candidate twice | the same set of fingerprints both times |
| CORE-004 | Run | a candidate with more than 1024 violations | exactly 1024 items, most severe first; counts add up to the total; every severity above the least severe listed is listed in full |
| CORE-005 | Run | each item of CORE-002 and CORE-004 | `rule_id`, `message`, `fingerprint` and `location.path` within their bounds; `end_line` at least `start_line`, both 0 or both set; fingerprints unique; no count below its listed items |
| CORE-006 | Run | `candidate_ref` or `config_ref` absent; a malformed reference | INVALID_ARGUMENT for each; no checks run |
| CORE-007 | Run | a `candidate_ref` the host cannot resolve; a `config_ref` the host cannot resolve | NOT_FOUND for each |
| CORE-008 | Run | the crashing candidate | FAILED with no items; never SUCCEEDED, never a partial list |
| CORE-009 | Run | a config naming a check the provider cannot run | UNAVAILABLE, as the proto's status codes state; never SUCCEEDED |
| CORE-010 | Run | the seeded candidate plus in-tree configuration and inline directives that suppress every check | every seeded violation still reported, or each directive reported as a finding |
| CORE-011 | Run | a seeded violation whose tool output is multi-line, longer than 1024 bytes and holds a U+200B | SUCCEEDED; the `message` is shortened at a character boundary, keeps U+000A, and has U+FFFD for U+200B |
| CORE-012 | Run | RUNNING messages of any case | each carries status and diagnostics only; only the last message is terminal |
| CORE-013 | Run | the seeded candidate where the nested sandbox cannot be built (process profile) | UNAVAILABLE; no candidate code ran, nested or not |
| CORE-014 | Run | a seeded test whose output is over the provider's size limit, malformed, or holds an XML entity or include | FAILED for each, the oversized output never truncated to continue; no entity or include expanded; never SUCCEEDED |
| CORE-015 | Run | any case that SUCCEEDS | exactly one `VerifierReport` evidence id in `evidence_ids`, and no findings in it; a call without it is PARTIAL |

## Host checks

The host enforces these on any provider; the kit runs them with a scripted provider unless the row says it runs against the real provider.

| Id | Case | Expect |
| --- | --- | --- |
| HOST-001 | Run for an actor kind other than reviewer-agent; Run whose `actor_principal` is the author run's principal; Run whose `run_id` is the author run's `run_id`; both again for the earlier of two author runs of the Change | the host refuses each before `Invoke` |
| HOST-002 | a verifier whose manifest or grant names an egress destination or a credential intent | the host refuses to dispatch Run |
| HOST-003 | a `config_ref` naming an artifact of the candidate or of the author run | the host refuses before `Invoke` |
| HOST-004 | run against the real provider, not scripted: a test in the seeded candidate that deletes, writes and adds files in the candidate tree, reads its environment and connects to 169.254.169.254 and to an outside host | the stored candidate's digest is unchanged; the canary secret appears nowhere in the sandbox; both connections fail |
| HOST-005 | 1025 items; two items with one fingerprint; items not most severe first; a severity zero or unknown; a count below its listed items, or above them for a severity above the least severe listed; two counts of one severity; a count of 0; fewer items than min(total, 1024), such as counts {CRITICAL: 1, HIGH: 5} with only the CRITICAL item listed | the host rejects the response and records the run as failed |
| HOST-006 | a `location.path` with a leading '/', a '\', or an empty, '.' or '..' segment; `end_line` below `start_line`; `start_line` 0 with `end_line` set; a `message`, `rule_id` or `fingerprint` out of bounds | the host rejects the response and records the run as failed |
| HOST-007 | a deadline shorter than the checks take | the host ends the call; nothing after the deadline is accepted; the run is recorded failed |
| HOST-008 | a terminal message whose `evidence_ids` name an id the call did not emit | the host rejects the response |
| HOST-009 | a verifier manifest declaring an idempotency other than `IDEMPOTENCY_NATURAL` for Run | the host refuses it at install or handshake |
| HOST-010 | run against the real provider, not scripted: a test in the seeded candidate that tries to reach the host channel (inherited descriptors, the plugin socket), call `HostService.EmitEvidence`, rewrite another check's report and signal or ptrace the verifier | the host receives nothing from the test; no evidence the test asked for exists; the other checks' findings equal those of CORE-002 |
| HOST-011 | a `RunResponse`, `Finding` or `FindingsSummary` carrying a field unknown to the host's descriptor copy (plugin/v1 `Invoke` strict decoding), as a provider built against a later minor would send if it ignored `HandshakeRequest.spec_version` | the host rejects the response and records the run as failed |

## Extended

None in 1.0.0.

## Not covered by this kit

- Which findings block a gate is policy and the Verdict, owned by the verification context.
- The sandbox profile and floor are checked by the sandbox suites; HOST-004 and HOST-010 are the part every verifier provider must pass.
- Seeded-defect catch rates in replay are the verification context's metric, not provider conformance.
