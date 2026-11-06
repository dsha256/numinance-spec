# The evidence chain and export format, evidence/v1

An auditor who has an export directory and the public keys of the runner and the transparency log can check the export offline, without Numinance software. This directory is the public specification of what such a check reads and decides. It records the format that the shipped evidence context writes and verifies today. It adds no behaviour to the format.

Status: `evidence/v1`, spec version 1.0.0. Change class of the specification: `contract-additive` (RFC-0003). A change to hashing or to the canonical form is `contract-breaking` and needs its own ADR and a major version.

## Pages

| Page | What it fixes |
|---|---|
| [canonical-form.md](canonical-form.md) | the canonical JSON bytes, strict decoding, digests, the record hash, signing contexts and keys |
| [records-and-chain.md](records-and-chain.md) | the record, its validity rules, the per-change chain, terminal and seal records |
| [receipts.md](receipts.md) | the receipt records of the broker and the egress proxy |
| [trajectory.md](trajectory.md) | the runner's statement of how the executor's reported tool use compares with the host and proxy records |
| [log-and-checkpoints.md](log-and-checkpoints.md) | leaves, Merkle hashing, inclusion proofs, checkpoints, which hash a head leaf holds |
| [bundle-layout.md](bundle-layout.md) | the files of an export, the index, the design list, absent parts, how a directory is read |
| [attestation-and-manifest.md](attestation-and-manifest.md) | the attestation (with the shipped encodings of `ticket`, `credential_scopes` and `network_policy`) and the signed bundle manifest |
| [derivations.md](derivations.md) | everything a verifier re-derives: summary, diff digest, closed at PR level, `change.json`, the three views, tool events, signer sets |
| [verification.md](verification.md) | finding codes, result levels and their precedence, key modes, exit codes, reports, trust limits, the release ordering rule |

Machine-readable parts:

- JSON Schemas: `../../schemas/evidence/v1/` (README there). They check shape; the pages say what a schema cannot.
- Conformance vectors: `../../conformance/evidence/v1/` (README there): golden export directories and, for each vector, the flags to run with and the exit code, level and finding codes to expect.
- The RFC: `../../rfcs/0003-evidence-chain-format-v1.md`.
- The protos: `../../proto/numinance/evidence/v1/` define the messages and carry the field-level comments.

## Marking

"Shipped" means the first private `numi evidence verify` did it and the vectors prove it. Two things were added afterwards and are marked NEW where they appear:

- the `--json` fields `level_enum`, `weakened_by_flags` and `anchored_head_record_hash` and the flags `--expect-change` and `--expect-head` ([verification.md](verification.md), sections 6 and 8): implemented by the open verifier and by the reference implementation's own verifier;
- the vectors of status `target`, which cover them.

## The open verifier

The open verifier is the Go package `../../sdk/go/evidenceverify` (standard library only, offline, read-only, size-bounded), and `numi evidence verify` in the open `numi` is a thin wrapper over it (`../../cli/README.md`). It was written from these pages, the schemas and the vectors only, and it passes every vector of `../../conformance/evidence/v1/`, those of status `target` included. It has no built-in trust root: it reaches `verified` only with runner keys, a log key and a log origin that the caller pins.

Section 11 of [verification.md](verification.md) lists the readings the vectors fixed while it was written and while the two verifiers were made to agree. The engine build's `numi evidence verify` delegates its verdict to this package; the reference implementation's verifier is kept as an independent oracle, and a differential test runs both over every vector and a mutation corpus.

## What is not here

No export leaves the project's own machines before the ordering rule of [verification.md](verification.md) section 10 holds.
