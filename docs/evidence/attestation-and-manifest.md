# Attestation and signed bundle manifest

Status: normative for `evidence/v1` (spec 1.0.0). Read [canonical form](canonical-form.md) and [log and checkpoints](log-and-checkpoints.md) first.

A change that ended `Closed` has an attestation, signed by the runner at Close, and an export of it has a signed manifest that covers every file of the directory. Both are signed by the runner key, so a verifier that pins that key (and no other) sees any edit. Changes that ended `Abandoned` or `RolledBack` have neither: see [verification](verification.md), level `unsigned`.

## 1. Attestation

`attestation.json` is a canonical object (schema `attestation.schema.json`), at most 1 MiB. Repeated fields hold at most 1024 entries each and are checked while read. Keys are the proto field names of `attestation.proto`.

| Key | Type | Presence | Source and rule |
|---|---|---|---|
| `change_id` | string | required | re-derived from the records |
| `ticket` | string | required | leaf field; see section 2 |
| `repository` | string | optional | leaf field; printable ASCII, at most 256 bytes |
| `base_commit` | string | optional | leaf field; printable ASCII, at most 256 bytes |
| `result_commit` | string | optional | leaf field, but cross-checked: see section 3 |
| `executor_id` | string | optional | re-derived |
| `model_id` | string | optional | re-derived |
| `sandbox_image_digest` | digest | optional | re-derived |
| `policy_bundle_digest` | digest | optional | re-derived |
| `profile_digest` | digest | optional | re-derived |
| `toolchain_digest` | digest | optional | leaf field |
| `verifier_results` | array of artifact | optional | re-derived |
| `ci_run_ids` | array of string | optional | leaf field; printable ASCII, at most 256 bytes each |
| `deployment_ids` | array of string | optional | leaf field; as `ci_run_ids` |
| `actors` | array of `{kind, principal}` | optional | re-derived |
| `credential_scopes` | array of string | optional | leaf field; see section 2 |
| `network_policy` | artifact | optional | leaf field; see section 2 |
| `approval_ev_ids` | array of string | optional | re-derived |
| `first_sequence` | integer | required | re-derived: 1 |
| `last_sequence` | integer | required | re-derived: the terminal record's sequence |
| `head_record_hash` | digest | required | re-derived: the terminal record's `record_hash`; a bound field, never the signed value |
| `signature` | base64 | required | 64 bytes, runner key |
| `signing_key_id` | digest | required | the runner key's id |
| `canon_version` | integer | required | 1 |
| `checkpoint_ref` | artifact | required | names the per-change checkpoint; the shipped writer sets `ref` to `checkpoints/<64 hex>.json`, a verifier requires a valid artifact whose `sha256` is the digest of the bytes of `attestation-checkpoint.json` |
| `inclusion_proof` | inclusion proof | required | the head leaf under that checkpoint |

An artifact (a "Reference") is `{"ref": ..., "sha256": ...}`. Its `artifact_type` is outside the hashed form and is never decision-bearing. A "leaf field" has no source record: the signature, and the attestation leaf's inclusion proof, are all that bind it. The attestation covers the records from `first_sequence` to `last_sequence`. On a sealed chain the seal record that follows the terminal record is outside the range.

Every leaf field a producer cannot source stays empty, and an empty field is bound by the signature as empty: it is never invented. An empty field is a report-only note (see "Empty fields" below), never a failure and never a weakness.

Order at Close: the runner commits the per-change checkpoint, then signs the attestation with `checkpoint_ref` naming it and `inclusion_proof` showing the head leaf under it. The attestation leaf (kind `0x41`, section 1 of the log page) is appended to the log immediately after signing, and sealed by the next checkpoint.

### Signature and decode checks

A verifier decodes `attestation.json` strictly. It then requires the document to be signed (64 bytes), `signing_key_id` to be a trusted runner key (unless signatures are skipped) and the signature to verify. Failures are `attestation_invalid`.

## 2. Leaf field encodings as shipped

These are the encodings of three leaf fields. The proto types are `ticket` string, `credential_scopes` repeated string and `network_policy` `Reference`; no richer type is part of `evidence/v1`, and a richer type would be a separate proposal.

### ticket (shipped)

A string, never empty in a signed attestation, of one of two forms:

- `none`: the change was opened with no ticket.
- `<reference>#<digest>`: the reference of the ticket and the digest of the ticket's canonical bytes. The reference is 1 to 128 bytes of printable ASCII (0x21 to 0x7e) with no space and no `#`; the digest is `sha256:` and 64 lowercase hex. Example: `docs/build/v1a/ticket.md#sha256:...`.

Any other value, including the empty string, makes the attestation invalid (`attestation_invalid`, at decode). The whole field is at most 256 bytes. When `change.json` is present its `ticket` MUST equal the attestation's (`change_invalid` otherwise).

### credential_scopes (shipped)

A repeated string, the credential broker's statement of the scopes of every handle issued for the change. It is empty when no broker sourced it. When present:

- Entry 0 is the digest line `broker.scopes.v1 sha256:<64 lowercase hex>`.
- Every entry is printable ASCII (0x20 to 0x7e) of at most 256 bytes, with no leading or trailing space; this bound is checked before the grammar. Every other entry is a handle scope line `nhref_<12 lowercase hex> <kind>:<value>`: `kind` is `[a-z][a-z0-9_]{0,31}` and `value` is one or more printable ASCII characters (0x21 to 0x7e). A line names a handle by a prefix of its digest, never the handle, and scope names, never credential material.
- The handle lines are in strictly ascending byte order, without duplicates.
- The digest is `sha256( "numinance.credential_scopes.v1" || for each handle line: "\n" || line )`, written in lowercase hex. The digest of the empty set (no handle lines) is a statement that the broker issued no handle: it differs from an empty field that nobody sourced.

A field that does not start with the digest line, has a line outside the grammar, is unsorted or has a digest that does not match makes the attestation invalid. The entries are otherwise unchecked: their origin rests on the runner's signature.

### network_policy (shipped)

An artifact `{"ref": ..., "sha256": ...}`: the egress proxy's statement of the allow-list it held and the denials it judged for the change, by reference and digest. The `ref` MUST be `egress-policy.v1/` followed by the attestation's own `change_id`, and `sha256` is set: the digest of the proxy's canonical egress policy record. An attestation whose `network_policy` is any other reference (another change, another prefix, no digest) does not decode (`attestation_invalid`). The digest has no content behind it in the bundle: an auditor who holds the proxy's record recomputes it. A verifier's report carries a note that says whether the field is sourced, and that the digest cannot be re-derived from the bundle (its origin rests on the runner's signature).

### Empty fields

A signed attestation with `credential_scopes` empty (no broker source) or `network_policy` absent (no proxy source) verifies. The verifier's report says `credential_scopes` and `network_policy` are unsourced, as notes; the level is unchanged. The ticket field is never empty: `none` is an explicit statement, not an absence. A producer that must refuse an unsourced field makes that choice itself; the format does not.

## 3. Re-derivation: what a verifier recomputes

After the chain verified, a verifier derives the attestation from the records and the stated leaf fields, and requires every re-derived field to equal the stated one. A difference is `attestation_mismatch` and names the group of fields.

Let `span` be the records up to and including the terminal record: the whole chain, or all but the seal record of a sealed chain. The last record of `span` MUST be `ChangeClosed.v1`; otherwise there is no attestation to derive and the result is `attestation_mismatch`. Then:

- `change_id`: the chain's change. `first_sequence`: 1. `last_sequence`: the sequence of the terminal record. `head_record_hash`: its `record_hash`.
- `policy_bundle_digest` and `profile_digest`: the values of the terminal record.
- `actors`: the distinct (`principal`, `kind`) pairs of `span`, ordered by `principal` (byte order) and then by kind number (`author-agent` 1, `reviewer-agent` 2, `release-pipeline` 3, `human-approver` 4, `runner` 5, `break-glass` 6, `broker` 8, `egress-proxy` 9; 7 is the producer-only engine, never an actor).
- Over the records of `span` whose `actor_kind` is `author-agent`: `executor_id` and `model_id` are the one non-empty value they carry (empty when none), and `sandbox_image_digest` is the one value they carry. When any of those records has an `executor_id`, the image is the one value the records that have an `executor_id` carry: a provider call made for the author (`scm.open_pull_request`) states the image of the provider's sandbox, which is not the image the author ran in, and does not count. Empty values are skipped, and the non-empty values must agree: two different values for any of the three make the chain ambiguous and the attestation `attestation_mismatch`. A non-empty `executor_id` of an `author-agent` record must have the form `<name>@sha256:<64 lowercase hex>` (the manifest name and the digest of its ExecutorManifest, [bundle layout](bundle-layout.md) section 6); any other form, a bare name included, means no attestation is made (a recorded gap), so an attestation beside such a record is `attestation_mismatch` (ADR 0052).
- `verifier_results`: the outputs of every record whose action is `verifier.run`, in chain order, concatenated.
- `approval_ev_ids`: the `ev_id` of every record whose action is `PlanGateDecided.v1`, in chain order.
- `result_commit`: when set, `sha256(result_commit text)` MUST equal the digest of some output named `publish/commit` of some record of the chain. (The commit text is not in the chain; a verifier checks a claim by hashing it.) An empty `result_commit` is accepted. Only a `publish/commit` output supports a `result_commit`: the `execution/candidate-commit` output of an `execution.diff` record (see [derivations](derivations.md) section 2) does not, so a `result_commit` that only a candidate commit names is `attestation_mismatch` (vector `attestation-result-commit-candidate-only`).
- Leaf fields are not re-derived (`ticket`, `repository`, `base_commit`, `toolchain_digest`, `ci_run_ids`, `deployment_ids`, `credential_scopes`, `network_policy`), except as decoded above.

Then the per-change checkpoint is checked: `attestation-checkpoint.json` MUST exist (`attestation_proof`), its bytes MUST hash to `checkpoint_ref.sha256` (`attestation_proof`) and decode as a stored checkpoint. With a pinned log, the attestation's head leaf MUST verify under it with `inclusion_proof` (`attestation_proof` on failure). Without a pinned log the checkpoint is decoded and reported as NOT CHECKED.

An attestation and an attestation gap together, an attestation with a broken chain, and a gap for a change that did not end Closed are findings: see the finding table in [verification](verification.md).

## 4. Bundle manifest

`bundle-manifest.json` is a canonical object (schema `bundle-manifest.schema.json`), at most 1 MiB, signed with the runner key. It lists every other file of the directory and proves both leaves.

| Key | Type | Presence | Rule |
|---|---|---|---|
| `change_id` | string | required | the change |
| `view` | string | optional | a token `^[a-z0-9][a-z0-9._-]{0,127}$`; the control-pack view an export was made with (none is defined in v1) |
| `exported_at` | string | required | RFC 3339 UTC; not checked against any clock |
| `spec_version` | string | required | strict `x.y.z` (the writer states `1.0.0`) |
| `first_sequence` | integer | required | 1 |
| `last_sequence` | integer | required | the attestation's `last_sequence` |
| `head_record_hash` | digest | required | the attestation's `head_record_hash`; a bound field |
| `entries` | array of `{path, sha256, size}` | required | 1 to 64 entries on the wire; every path is a known file name, not the manifest itself, and unique after lowercasing, so no valid manifest has more than 15 (the 16 known names less the manifest); `size` is an unsigned 64-bit integer |
| `signature` | base64 | required | 64 bytes |
| `signing_key_id` | digest | required | the runner key's id |
| `canon_version` | integer | required | 1 |
| `checkpoint_ref` | artifact | required | a valid artifact whose `sha256` is the digest of the bytes of `head-checkpoint.json` |
| `inclusion_proof` | inclusion proof | required | the head proof; MUST equal `head-inclusion-proof.json` |
| `attestation_inclusion_proof` | inclusion proof | required | the attestation leaf under `head-checkpoint.json` |

The signature is `Ed25519(runner_key, "numinance.evidence.v1.BundleManifest" || sha256(canonical(manifest with signature removed)))`. The manifest is required whenever the change has an attestation (and for every Closed change without a gap): a missing one is `manifest_missing`.

A verifier checks, after decoding strictly and verifying the signature under a trusted runner key (otherwise `manifest_invalid`):

1. Entries (`manifest_mismatch` on any difference; the digests the manifest is compared with are the ones the index step computed, so when the index does not decode no file was hashed and every entry is a mismatch): the entries are exactly the files of the directory but the manifest itself, each with the directory's digest and size, and every entry's file is also one the index lists (the index file itself excepted). A file in the directory that the manifest does not cover is a mismatch.
2. Binding (`manifest_mismatch`): `change_id` is the chain's; `head_record_hash` is the chain's anchored hash; `first_sequence` is 1; `last_sequence` is the sequence of the terminal record (the last record, or the one before the seal record); and all three equal the verified attestation's. A manifest with no verified attestation to bind is a mismatch. (The comparison with the attestation cannot fail on its own: an attestation is accepted only when its range and head equal what the chain derives, and the manifest is compared with the chain first, so it is subsumed by the attestation re-derivation and has no isolating vector.)
3. Anchor files (`manifest_mismatch`): `head-checkpoint.json` and `head-inclusion-proof.json` are both present; the digest of `head-checkpoint.json` is `checkpoint_ref.sha256`; the canonical form of `head-inclusion-proof.json` equals that of `inclusion_proof`.
4. With a pinned log and a verified attestation: `attestation_inclusion_proof` verifies the attestation leaf under `head-checkpoint.json` (`manifest_mismatch` on failure). Together with the head proof of the directory and the attestation's own proof, success is what makes the export fully anchored.

Because only the runner key can sign the manifest, a truncated or edited export that rewrites `summary.json` and the index still disagrees with it. No log key is needed to see that; the log is needed to refuse an export that is wholly replaced by another consistent export (see "Trust limits" in [verification](verification.md)).
