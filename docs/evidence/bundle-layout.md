# Export bundle layout

Status: normative for `evidence/v1` (spec 1.0.0). Read [canonical form](canonical-form.md) first.

`numi evidence export CHG_ID --out DIR` writes a directory. `numi evidence verify DIR` reads it offline. The directory is untrusted input: every rule here is one a verifier enforces before it believes anything in it.

## 1. Files

| Path | Required | Content | Read limit |
|---|---|---|---|
| `records.ndjson` | always | the chain, one canonical record per line ([records](records-and-chain.md)) | 256 MiB |
| `summary.json` | always | counts and heads the records re-derive ([derivations](derivations.md) section 1) | 1 MiB |
| `export-index.json` | always | every other file with digest and size, and every absent part | 1 MiB |
| `provenance/signers.json` | when the exporter states keys | the keys the exporter says signed, and the log it says anchors them: a claim | 1 MiB |
| `provenance/provenance.json` | design part `provenance/` | derived view ([derivations](derivations.md) section 5) | 1 MiB |
| `provenance/executor-manifest.json` | when an `executor_id` of the chain names a manifest (section 6) | the ExecutorManifest document, canonical JSON, whose sha256 the `executor_id` names after `@` | 1 MiB |
| `verification/verdicts.json` | design part `verification/`, only when the chain holds a verdict or a verifier report | derived view | 1 MiB |
| `review/review.json` | design part `review/`, only when the chain holds a gate decision or a reviewer record | derived view | 1 MiB |
| `diff.digest` | design part, only when the chain holds an `execution.diff` record | `sha256:<hex>` and a line feed | 1 MiB |
| `tool-events.ndjson` | design part | the canonical executor event streams the chain cites, concatenated; empty when the chain cites none | 256 MiB |
| `change.json` | design part, when a Change reader was wired | the Change as the chain vouches for it ([derivations](derivations.md) section 6) | 1 MiB |
| `attestation.json` | design part, when the change ended Closed and the attestation was made | the signed attestation ([attestation and manifest](attestation-and-manifest.md)) | 1 MiB |
| `attestation-checkpoint.json` | with `attestation.json` | the per-change checkpoint its `checkpoint_ref` names | 1 MiB |
| `attestation-gap.json` | instead of `attestation.json`, for a Closed change whose attestation was not made | `{"change_id", "export_version", "reason"}` | 1 MiB |
| `bundle-manifest.json` | with `attestation.json` | the signed manifest | 1 MiB |
| `head-checkpoint.json` | when the log anchored the head | the checkpoint the head leaf is proved under | 1 MiB |
| `head-inclusion-proof.json` | with `head-checkpoint.json` | the inclusion proof of the head leaf | 1 MiB |

(The decoder of each document also applies its own, smaller bound: a checkpoint is at most 4096 bytes, an inclusion proof 8192, a record 3 MiB, an attestation and a manifest 1 MiB.)

The three directories `verification/`, `review/` and `provenance/` are the only directories. Their file names are fixed as above. No other file name, no other directory and no deeper nesting exists.

The names `deployment.json`, `observation.json` and `rollback.json` are not files of `evidence/v1`: no producer for them exists, so they are always declared absent in the index, and a file with such a name is an `unknown_file`.

### Reading the directory

A verifier MUST read the directory as follows.

- It lists the regular files of the directory and of the three part directories. A symbolic link, a directory that is not one of the three, a device, a FIFO or any other entry is refused as `directory_refused`. A link is never followed.
- At most 64 entries are read from the directory itself (its files and the three part directories count), at most 64 from each part directory, and at most 64 files in all. A 65th is `directory_refused`.
- Every name is a plain file name: 1 to 255 bytes, not `.` or `..`, no `/`, `\` or `:`, no control character (U+0000 to U+001F, U+007F). Another name is `directory_refused`.
- The path given to the verifier is a directory and not a link, and the directory opened is the one checked (a swap between check and open is refused); otherwise the run ends with exit 3 and no report. A missing directory is exit 4, also with no report. Links in the parent folders are followed, as the operator chose them.
- A file is opened as a regular file and read for at most its limit. A file over its limit is refused before the first read (and one that grows while it is read fails at the limit). A refusal of a read inside the checks below is reported as the finding of that check (for `summary.json`, `summary_invalid` and `digest_mismatch`).
- A name that is not a known file is `unknown_file`. This makes a verifier refuse a file it ignores, so an ignored file cannot hide a claim.

## 2. The index

`export-index.json` is a canonical object (schema `export-index.schema.json`):

```
{ "absent": [ {"owner": ..., "part": ..., "reason": ..., "not_applicable": true?} ],
  "change_id": ..., "entries": [ {"path": ..., "sha256": ..., "size": ...} ], "export_version": 1 }
```

(`absent` is a key only when it has entries.) Rules:

- `export_version` is 1. `change_id` is the change. `entries` has 1 to 64 items.
- Every entry `path` is a valid bundle path (a plain file name, or a plain file name inside `verification`, `review` or `provenance`), a known file name, and neither `export-index.json` nor `bundle-manifest.json`. A path is listed at most once. Every `sha256` is a digest. `size` is an integer.
- Every `absent` entry names a part of the design list (section 3), with a non-empty `reason` and `owner` of at most 512 bytes each.
- `absent` has at most 12 entries, `entries` at most 64 on the wire; every path is a known file name other than the index and the manifest and is listed once, so no valid index has more than 15.

The index must agree with the directory. A verifier reports, each as its own finding: a listed file that is not in the directory (`file_missing`), a known file in the directory that the index does not list (`file_unlisted`; the manifest is exempt), a listed file whose size or sha256 differs (`digest_mismatch`), a missing or undecodable index (`index_invalid`), and an index and a summary that name different changes (`change_mismatch`).

The index is unsigned. It detects a swapped, edited, added or removed file only against what the same directory says. An attacker who can rewrite the directory rewrites the index; the signed manifest of a Closed change, and the record signatures, are what resist that.

## 3. The design list and absent parts

The format lists the parts of an export. The parts are, in this order:

`change.json`, `attestation.json`, `records.ndjson`, `tool-events.ndjson`, `diff.digest`, `verification/`, `review/`, `provenance/`, `deployment.json`, `observation.json`, `rollback.json` (only for a change that ended `RolledBack`), `summary.json`.

For each part that applies, the directory MUST either hold it (a directory part is held when any file inside it is present) or declare it absent in the index with a reason and an owner. An export never fakes a part. A verifier reports:

- `part_missing`: a required part is neither present nor declared absent.
- `part_contradiction`: a part is both present and declared absent, or a part is declared `not_applicable` and the chain does not bear that out.
- `parts_absent`: only under `--complete`, when at least one part is declared absent and not applicable.

`not_applicable: true` is accepted only for `deployment.json` and `observation.json`, and only when the chain shows the change closed at pull-request level (the last `ChangeClosed.v1` record's `lifecycle.from` is `merging`, see [derivations](derivations.md) section 4). Such a part is reported as not applicable and is never counted as absent, so `--complete` accepts it.

A part that is declared absent for an export of a Closed change that has an attestation is a weaker export, not a failed one: the report lists the absent parts and the result line says `PARTIAL`. Only `--complete` turns an absent part into a failure.

## 4. Signers

`provenance/signers.json` is a canonical object `{"export_version": 1, "signers": [{"key_id": ..., "public_key": ...}], "log": {"origin": ..., "public_key": ...}?}` with 1 to 64 signers. Every `key_id` MUST be the digest of its `public_key` (32 bytes), and a log claim has a non-empty origin and a 32-byte key. It is a claim. A verifier decodes it when it is present (a document that does not decode or whose `key_id` is not the digest of its key is `signers_invalid`), and uses its keys only in `--trust-bundle-keys` mode.

## 5. Writing

An exporter writes all files or none. The directory MUST NOT exist before the export, and the writer never follows a link and never overwrites. An exporter checks its own bytes as a verifier would before it writes (links, signatures against the keys it states, the head proof), so an export that would not verify is not written. A Closed change whose attestation is not yet made is not exported (the export is retried), unless a gap is recorded, in which case `attestation-gap.json` is written in its place and no manifest is written.

## 6. The executor manifest

An `executor_id` of the form `<name>@sha256:<64 lowercase hex>` names the ExecutorManifest the host routed the author run under, by the sha256 of the document's canonical JSON. The document is the whole ExecutorManifest (design section 8: capabilities, residency, `image_digest`, `model_ids`), not a projection, and it is in the canonical form of `record_hash` (object keys sorted, no insignificant white space). Any other form of `executor_id` makes no manifest claim and is not read. A verifier applies these rules to the distinct digests that the `executor_id` values of `records.ndjson` name:

- no digest named and `provenance/executor-manifest.json` present: `digest_mismatch`;
- one or more digests named and the file absent: `part_missing`;
- more than one digest named: `digest_mismatch` (an export holds one manifest);
- the file present and its sha256 not equal to the named digest: `digest_mismatch`;
- the file present, with the right digest, but not a JSON object already in canonical form: `digest_mismatch`;
- the file present, with the right digest, and its `name` not equal to the name before `@` in an `executor_id` that names it: `digest_mismatch`.

The file is a JSON object with these keys, all always present: `kind` (`ExecutorManifest`), `name`, `version`, `capabilities` (names), `image_digest`, `model_ids` (in the order of `models`) and `models`, a list of objects `{id, residency}`. `residency` has the keys of design section 8 (`execution`, `inference`, `prompt_egress`, `regions`, `training_use`, `retention`) and the other dimensions of design section 11 the host declares (`model_provider`, `model_endpoint`, `source_region`, `retention_max_seconds`, `customer_keying`, `network_path`, `private_endpoint`). The enumerated values are strings, never numbers: `execution` is `customer_account` or `vendor`; `inference` and `prompt_egress` are `none`, `customer_account` or `vendor`; `training_use` is `none` or `permitted`; `retention` is `none`, `customer_controlled` or `provider`; `network_path` is `none`, `in_account`, `private_endpoint` or `public`; `regions` is sorted and unique. The file is a reduced copy of the ExecutorManifest kind: `provider`, `cost_class`, `max_context`, `extensions` and `api_version` are omitted. A verifier checks only the rules above, not this schema.

The file is not derived from the records. The digest in the signed records is what binds it, and an edited file or an edited `executor_id` fails here, an edited record fails the chain. The check proves which manifest the host routed under. It does not prove the executor obeyed it, and the `model_id` beside it is the executor's own claim, checked only against its grammar (ADR 0052).
