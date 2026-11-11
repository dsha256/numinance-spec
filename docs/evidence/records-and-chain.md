# Records and the per-change chain

Status: normative for `evidence/v1` (spec 1.0.0). Read [canonical form](canonical-form.md) first.

## 1. The record

`records.ndjson` holds one canonical record per line, in chain order, each line ended by a line feed (U+000A). A last line without its line feed is accepted: a line ends at a line feed or at the end of the file. A line feed is the only line separator: a carriage return before it is trailing data of the record and the record is refused. An empty line is refused. A line is at most 3 MiB (3145728 bytes) and the file at most 256 MiB.

A record is a JSON object. The JSON Schema is `schemas/evidence/v1/record.schema.json`. The keys are:

| Key | Type | Presence | Rule |
|---|---|---|---|
| `ev_id` | string | required | `ev_` and 32 lowercase hexadecimal characters |
| `change_id` | string | required | `chg_` and 32 lowercase hexadecimal characters |
| `sequence` | integer | required | 1 to 2^53-1 |
| `action` | string | required | a capability `port.verb` (`^[a-z][a-z0-9]*\.[a-z][a-z0-9_]*$`), a tool capability `tool.<plugin>.<name>` of the tool gateway (`^tool\.[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?\.[a-z][a-z0-9_]*$`: the plugin is one lowercase DNS label, the name lowercase snake_case) or a versioned lifecycle event name (`^[A-Z][A-Za-z0-9]*\.v[1-9][0-9]*$`, with a major version that fits 31 bits: 1 to 2147483647), at most 128 bytes |
| `actor_principal` | string | required | 1 to 256 bytes of printable ASCII (0x20 to 0x7e), no leading or trailing space |
| `actor_kind` | string | required | one of `author-agent`, `reviewer-agent`, `release-pipeline`, `human-approver`, `runner`, `break-glass`, or, on a receipt record only, `broker` or `egress-proxy` ([receipts](receipts.md)) |
| `actor_run_id` | string | optional | `run_` and 32 hex; required when `actor_kind` is `author-agent` or `reviewer-agent` |
| `policy_decision_id` | string | optional | `dec_` and 32 hex; required on a capability record |
| `grant_id` | string | optional | `grant_` and 32 hex; required on a capability record unless `action_outcome` is `denied` |
| `risk_class` | string | required | `READ`, `LOW`, `MEDIUM`, `HIGH` or `PRIVILEGED` |
| `profile_digest` | digest | required | the profile in force |
| `policy_bundle_digest` | digest | required | the policy bundle in force |
| `plan_digest` | digest | optional | |
| `executor_id` | string | optional | at most 256 bytes of printable UTF-8, no leading or trailing space |
| `model_id` | string | optional | as `executor_id` |
| `sandbox_image_digest` | digest | optional | set exactly when `sandbox_profile` is set |
| `sandbox_profile` | string | optional | `process`, `container`, `gvisor` or `microvm`; set exactly when `sandbox_image_digest` is set |
| `inputs` | array of artifact | optional | at most 1024 entries |
| `outputs` | array of artifact | optional | at most 1024 entries |
| `action_outcome` | string | required | `succeeded`, `partial`, `failed` or `denied` |
| `corrects_record_hash` | digest | optional | a correction: names an earlier record's `record_hash` |
| `previous_record_hash` | digest | optional | absent exactly at sequence 1 |
| `record_hash` | digest | required in an export | see [canonical form](canonical-form.md) section 4 |
| `timestamp` | string | required | RFC 3339 UTC, year 1 to 9999 |
| `canon_version` | integer | required | 2 |
| `signature` | string (base64) | optional | 64 bytes; absent only in a chain that was never signed. A verifier in `--skip-signatures` mode accepts such a record; in every other mode it is `signature_invalid` (or `signer_untrusted` when its key id is not trusted) |
| `lifecycle` | object | optional | see below |
| `signing_key_id` | digest | required in an export | the key id of the runner key that signed |

An artifact is `{"ref": ..., "sha256": ...}`, nothing else. `ref` is 1 to 1024 bytes of `[A-Za-z0-9._/-]`, starts with a letter or digit, and has no `..` segment. `sha256` is a digest.

`lifecycle` is `{"from": ..., "to": ..., "row_version": ..., "author_run_id": ..., "max_repair_attempts": ...}`. `from` and `to` are 1 to 32 bytes of `[a-z_]`. `row_version` is 1 to 2^53-1. `author_run_id` (a run id) and `max_repair_attempts` (0 to 65536) are optional. It is accepted only when the action is one of `ChangeTransitioned.v1`, `ChangeUnderstood.v1`, `ChangeClassified.v1`, `PlanProduced.v1`, `PlanGateDecided.v1`, `CandidateAssembled.v1`, `ChangeEscalated.v1`, `ChangeClosed.v1`, `ChangeAbandoned.v1` or `ChangeRolledBack.v1`, and only when `action_outcome` is `succeeded`.

### Record validity rules

A stored record MUST satisfy all of these, in addition to the strict decoding of canonical-form section 3. A record that fails any is `record_invalid`.

1. `ev_id`, `change_id`, `profile_digest` and `policy_bundle_digest` are set. `action` is a capability or a lifecycle event name. `risk_class` and `action_outcome` are valid. `canon_version` is 2.
2. The actor is valid: the name rule above, and a `actor_run_id` for the two agent kinds.
3. `executor_id` and `model_id` are empty or at most 256 bytes of valid UTF-8 in which every character is printable by Go's `unicode.IsPrint` (letters, marks, numbers, punctuation, symbols and the ASCII space; no control, format, private-use, surrogate or unassigned character, no non-character such as U+FFFF, no non-ASCII space), and no leading or trailing ASCII space. The table is the Unicode table of the Go release that builds the verifier: a verifier in another language MUST use the same set, and the vectors name code points (U+E000, U+0378, U+FFFF, U+00A0, U+200B) that no release treats as printable. The same rule bounds the checkpoint origin.
4. `sequence` is 1 to 2^53-1 and `previous_record_hash` is empty exactly when `sequence` is 1.
5. `timestamp` is set, UTC, and its year is 1 to 9999.
6. `sandbox_profile` and `sandbox_image_digest` are both set or both empty. A record with an `executor_id` has both.
7. A capability record has `policy_decision_id`, and `grant_id` unless it is `denied`. A lifecycle event record may omit both.
8. `corrects_record_hash` is empty at sequence 1 and never equals the record's own `record_hash`.
9. `inputs` and `outputs` hold at most 1024 entries each and every artifact is valid.
10. `signature` is empty or 64 bytes, and when set so are `signing_key_id` and `record_hash`.
11. `lifecycle`, when present, is valid and only on the actions and the outcome above.
12. A receipt action (`HandleIssued.v1`, `HandleRevoked.v1`, `NetworkRequests.v1`, `ReceiptsOverflow.v1`) and a component `actor_kind` (`broker`, `egress-proxy`) go together, and the outputs have the grammar of [receipts](receipts.md) (a request line has ten fields, or eleven with the `injected` marker added in 1.1.0).
13. A `TrajectoryCompared.v1` record is the runner's succeeded record in the grammar of [trajectory](trajectory.md), its result agrees with its counts, and its `ev_id` is the one its change derives.

Messages of a refusal never echo a field value; a verifier SHOULD NOT either.

## 2. The chain

A chain is the records of one change, in order. A verifier is told the change (`summary.json`'s `change_id`) and walks `records.ndjson` from the first line, applying these rules to record number `n` (1-based position in the file) with the chain so far:

1. The record decodes strictly and is valid (section 1). Otherwise `record_invalid`.
2. `record_hash` is set. Otherwise `chain_broken`.
3. The chain has not ended (rule 9). Otherwise `chain_broken`.
4. `change_id` equals the change the verifier was told. Otherwise `chain_broken`.
5. The `ev_id` is not the `ev_id` of an earlier record. Otherwise `chain_broken`.
6. When `corrects_record_hash` is set it equals the `record_hash` of an earlier record. Otherwise `chain_broken`.
7. `sequence` equals `n`, and `previous_record_hash` equals the `record_hash` of record `n-1` (empty for `n` = 1). The `record_hash` of record `n`, re-derived from the record as stored, equals the stored one. Otherwise `chain_broken`. (A record whose `sequence` is wrong, whose link is wrong or whose content changed after hashing all fail here, and each is a `chain_broken`.)
8. No chain is long enough to reach sequence 2^53-1; a record that states a sequence above it is `record_invalid` (rule 1), never a chain failure.
9. A record whose action is `ChangeClosed.v1`, `ChangeAbandoned.v1` or `ChangeRolledBack.v1` ends the chain: the only record that may follow it is the seal record. A seal record is a record whose action is `EvidenceSealed.v1`; it MUST immediately follow a terminal record, carry exactly one output (the checkpoint reference) and no input, and carry no `corrects_record_hash`. After a seal record nothing may follow.
10. An empty file has no chain: `chain_broken` ("no records").

The terminal state of a chain is `Closed`, `Abandoned` or `RolledBack` by the terminal action. A chain whose last record is not a terminal record or a seal record has not ended: it "may be a prefix of a longer chain", and verification says so (level `incomplete`).

A chain of more than 32768 records (2^15) is refused with `too_many_records` before the 32769th record is kept. This is the exporter's bound; it also bounds a verifier's memory.

### Corrections

A correction is a new record whose `corrects_record_hash` names an earlier record. Nothing is updated or deleted. A verifier does not interpret a correction beyond rule 6.

### What the chain does not say

The chain holds no clock beyond each record's `timestamp`; `timestamp` values are not required to increase and a verifier does not check them against each other. The chain proves that these records, in this order, were signed by the pinned runner key. It does not prove the records are the whole chain: only the terminal record, the signed manifest and the transparency log do (see [verification](verification.md), "Trust limits").
