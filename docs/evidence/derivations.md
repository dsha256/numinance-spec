# Derivations a verifier re-derives

Status: normative for `evidence/v1` (spec 1.0.0). Read [records and chain](records-and-chain.md) first.

A part that the chain itself can state is derived from the chain, never trusted. The exporter and the verifier run the same derivation, so a part that differs from its derivation by one byte is a tampered or mismatched file. A part that the chain cannot state (`change.json`) is a read of the Change, cross-checked against the chain where the two overlap, and never holds a claim the verifier cannot check against the records or against an artifact digest the chain cites.

Every derivation below starts from a chain that already verified (every rule of the chain page passed). When the chain did not verify, the derivations are not run and their parts are reported as not checked.

Notation: `records` are the records of the verified chain in order; "the last record" is the record at the highest sequence, including a seal record.

## 1. summary.json

`summary.json` is a canonical object (schema `summary.schema.json`):

| Key | Type | Presence | Derivation |
|---|---|---|---|
| `export_version` | integer | required | 1 |
| `change_id` | string | required | the chain's change; the index MUST name the same change (`change_mismatch`) |
| `records` | integer | required | number of records |
| `first_sequence` | integer | required | 1 |
| `last_sequence` | integer | required | number of records |
| `head_record_hash` | digest | required | `record_hash` of the last record |
| `denied` | integer | required | number of records whose `action_outcome` is `denied` |
| `ended` | boolean | required | true when the chain is sealed, or its last record's action is `ChangeClosed.v1`, `ChangeAbandoned.v1` or `ChangeRolledBack.v1` |
| `terminal_state` | string | present exactly when `ended` | `Closed`, `Abandoned` or `RolledBack`: for a sealed chain the state of the terminal record before the seal record, otherwise the state of the last record |
| `sealed` | boolean | required | true when the last record is a seal record |

Decode rules: `export_version` is 1; `change_id` and `head_record_hash` are set; `first_sequence` is 1; `records` is at least 1; `last_sequence` equals `records`; `ended` is true exactly when `terminal_state` is set; a sealed chain has ended. A summary that does not decode is `summary_invalid`.

Comparison: `records`, `last_sequence`, `head_record_hash`, `denied`, `ended`, `terminal_state` and `sealed` MUST equal the derivation; each difference is a `summary_mismatch`. A summary edited to hide a dropped record disagrees with the chain. (A summary that was rewritten to agree with a truncated chain is caught by the manifest, not by this check: see [verification](verification.md), "Trust limits".)

## 2. diff.digest

The diff digest is the `sha256` of the last record, scanning from the end, whose action is `execution.diff` and which has an output with `ref` `execution/diff`. When no record has one there is no diff digest.

- When a diff digest exists the directory MUST hold `diff.digest`, and its bytes MUST be exactly the digest text followed by one line feed.
- When none exists the directory MUST NOT hold `diff.digest`.

An `execution.diff` record MAY also carry the output `execution/candidate-commit`. Its `sha256` is the digest of the 40 lower-case hex digit commit id the host made the candidate as, with no line feed. The output is optional: the Numinance Engine writes it from the first release after v0.1.0 that carries ADR 0049, and a chain written earlier lacks it. It is distinct from `execution/candidate` (the candidate tar) and from `publish/commit` (the outcome of a publication). It takes part in no derivation: `diff.digest`, `candidates` and the attestation's `result_commit` check read the same refs as before, so a verifier treats a record with the output and one without it alike (vectors `diff-candidate-commit-verified`, `diff-candidate-commit-unpublished-verified` and `attestation-result-commit-candidate-only`). Its meaning is for the policy side, which holds a push of the candidate against it.

Any difference is `diff_mismatch`. When `diff.digest` is absent and a diff digest exists, the part is also required: it is `part_missing` unless the index declares it absent.

## 3. Key ids and signer sets

A key id is `sha256` of the 32 raw bytes of the key. The set of trusted runner keys is built by the caller (see [verification](verification.md), "Key modes") and indexed by key id, never by a name the directory gives. A record, an attestation and a manifest each name their signer by `signing_key_id`; the name MUST resolve in that set, otherwise `signer_untrusted` (for a record) or `attestation_invalid` / `manifest_invalid` (for the other two). The same set serves all three: v1 has no per-role key separation. A key that resolves is then used to verify the signature, and a mismatch is `signature_invalid` (records) or `attestation_invalid` / `manifest_invalid`.

`provenance/signers.json` is decoded when present. Its `key_id` of each signer MUST be the key id of its `public_key`. A decode failure is `signers_invalid`. Its keys enter the trusted set only in `--trust-bundle-keys` mode; with `--runner-key` or `--skip-signatures` they are decoded and then ignored. A signer set can never add a key to a pinned set.

## 4. closed-at-PR-level

A change closed at pull-request level when its last `ChangeClosed.v1` record (scanning from the end) has a `lifecycle` object whose `from` is `merging`. The change lifecycle lets `Merging` exit to `Closed` "(PR level)": such a change was never released, deployed or observed. A change that has no `ChangeClosed.v1` record, or whose record has no `lifecycle` object, did not close at PR level.

For such a change, `deployment.json` and `observation.json` may be declared `not_applicable` in the index and are then never counted as absent. For every other change the declaration is `part_contradiction`.

## 5. verification, review and provenance views

Each view is a canonical JSON document whose bytes the verifier recomputes from the records and compares byte for byte with the file. Each starts with `"export_version": 1` and `"change_id"`. Arrays keep the order below. An empty array is an absent key.

**verification/verdicts.json** (schema `verification.schema.json`). Present exactly when the chain holds at least one verdict or one verifier report; otherwise the file MUST be absent.

- `verdicts`: one entry for each record that is a verdict record, in chain order: `{sequence, ev_id, record_hash, candidate, status, document, overrules?}`.
- `reports`: one entry for each record whose action is `verifier.run`: `{sequence, ev_id, record_hash, action_outcome, outputs?}` (`outputs` are the record's outputs).

A verdict record is a record with action `VerdictIssued.v1`, `actor_kind` `runner`, outcome `succeeded`, 1 or 2 inputs and exactly 2 outputs, where: the inputs are `verdict/subject` (the candidate digest; exactly once) and optionally `verdict/overrules` (the digest of the record hash this verdict replaces); the outputs are `verdict/document` (the digest of the canonical verdict document; non-zero) and `verdict/status/<status>` for `<status>` one of `pass`, `fail`, `inconclusive`, whose `sha256` is `sha256(status text)`; and no other ref appears. A record of that action that does not have this shape is not a verdict and gets no entry (it is not an error).

**review/review.json** (schema `review.schema.json`). Present exactly when the chain holds at least one gate decision or reviewer record.

- `decisions`: one entry for each record whose action is `PlanGateDecided.v1`.
- `reviewer_records`: one entry for each record whose `actor_kind` is `reviewer-agent`.
- Entry: `{sequence, ev_id, record_hash, action, actor, actor_kind, actor_run_id?, action_outcome}` where `actor` is the record's `actor_principal`.

**provenance/provenance.json** (schema `provenance.schema.json`). Always derivable. Fields:

- `plans`: the distinct non-zero `plan_digest` values, in first-seen order.
- `candidates`: in chain order, the digest of each output with ref `execution/candidate` of each `execution.diff` record, and the `candidate` of each verdict record, distinct, in first-seen order.
- `executors`: the distinct tuples `{executor_id, model_id?, sandbox_profile?, sandbox_image_digest?}` of the records with a non-empty `executor_id`, in first-seen order.
- `runs`: the distinct `actor_run_id` values in first-seen order, each `{run_id, principal, kind}` with the `principal` and kind of the first record that names it.
- `profile_digest` and `policy_bundle_digest`: the values of the last record.

A view file that differs from its derivation is `view_mismatch`. A view file that is present while the chain holds nothing it could be derived from is `view_mismatch`. A view part that the chain can derive and the directory neither holds nor declares absent is a `part_missing` (section 3 of [bundle layout](bundle-layout.md)).

## 6. change.json

`change.json` states the Change as the change context knows it, but only fields the chain vouches for. It is a canonical object (schema `change.schema.json`) with `export_version` 1 and `change_id` always, and optionally:

| Key | Type | What the chain says |
|---|---|---|
| `state` | string | the `to` of the last record that has a `lifecycle` object; replaced by the lowercase terminal token when the chain ended (`closed`, `abandoned`, `rolled_back`) |
| `plan_digest` | digest | the last non-zero `plan_digest` of any record |
| `author` | string | the `actor_principal` of the author-agent records, when exactly one distinct (principal, kind) pair has `actor_kind` `author-agent` |
| `author_kind` | string | `author-agent`, with `author` |
| `author_runs` | array of string | run ids of the change's author runs, each one a run id the chain names |
| `repair_attempts` | integer | the number of records whose `lifecycle.to` is `repairing`; stated only when some record has a `lifecycle` object |
| `max_repair_attempts` | integer | the `max_repair_attempts` of the last record that sets one (0 or absent when none does) |
| `ticket` | string | the ticket leaf of the Change (see [attestation](attestation-and-manifest.md) section 2); no record states it, so the exporter and the verifier require it to equal the attestation's |

Decode rules: `export_version` is 1, `change_id` is set, `state` at most 32 bytes, `author` at most 256 bytes, `author_runs` at most 1024 entries, `repair_attempts` not negative.

A verifier recomputes the facts the chain states (`state`, `plan_digest`, the author, the set of run ids, `repair_attempts`, `max_repair_attempts`) and compares:

- `change_id` MUST be the chain's.
- `state` MUST equal the derived state (an absent `state` equals only an empty derived state). `plan_digest` MUST equal the derived value.
- When `author` is set it MUST be the derived author with the derived kind; `author_kind` set without `author` is a mismatch.
- Every `author_runs` entry MUST be a run id that an author-agent record's `actor_run_id` or a `lifecycle.author_run_id` names.
- When no record has a `lifecycle` object, `repair_attempts` and `max_repair_attempts` MUST both be 0 or absent. Otherwise they MUST equal the derived values.
- When an attestation verified, `ticket` MUST be a well-formed ticket leaf equal to the attestation's.

Any difference, and a `change.json` that does not decode, is `change_invalid`. A field the chain cannot state (risk classes, work units, gate decision ids, the release grant) is never in `change.json`.

## 7. tool-events.ndjson

The chain cites executor event streams by digest: the output with ref `execution/stream` of each record whose action is `execution.stream`. The cited digests are those digests in chain order, each listed once (a repeated digest is dropped). Call the list `cited`.

`tool-events.ndjson` is the cited streams concatenated in that order. Each stream is a run of canonical event lines, each ended by a line feed, the first of which has `seq` 1. A verifier splits the file into streams at every line whose `seq` is 1, requires every line to end with a line feed and to be a JSON object with an unsigned integer `seq` (a file whose first line is not `seq` 1 is refused). The shipped reader finds `seq` with Go's JSON decoding of the line, so the key is matched without regard to case (`SEQ` is `seq`) and, when the key repeats, the last value wins; `1.0`, a missing key, a negative number and anything that is not an object are refused. A reader of this spec MUST do the same, because the split points decide which bytes are hashed, computes the `sha256` of the exact bytes of each stream (line feeds included), and requires that list of digests to equal `cited`, in order, with the same length. A dropped, added, edited or reordered event shows as a different digest. The chain cites no streams when `cited` is empty, and the file is then empty. Any failure is `tool_events_mismatch`.

A line of the stream is not otherwise inspected: the stream's own format is the execution context's, and the digest the chain cites is what binds its bytes.

## 8. What no derivation covers

`record_hash`, `previous_record_hash` and `signature` are the chain's. `timestamp` values are not derived from anything. `exported_at` of the manifest is a statement of the exporter. The transparency log's own contents are known to a verifier only through the checkpoint and the inclusion proofs it is given.
