# evidence/v1 schemas

Hand-authored JSON Schema (draft 2020-12) for the files of an evidence export, version 1. They are not generated from protos: the export files are produced by the evidence code, and these schemas describe the shape of what it writes. The prose spec is in `../../../docs/evidence/`; the golden exports and conformance vectors are in `../../../conformance/evidence/v1/`.

| Schema | Validates |
| --- | --- |
| `record.schema.json` | one line of `records.ndjson` |
| `attestation.schema.json` | `attestation.json` |
| `bundle-manifest.schema.json` | `bundle-manifest.json` |
| `checkpoint.schema.json` | `head-checkpoint.json`, `attestation-checkpoint.json` |
| `inclusion-proof.schema.json` | `head-inclusion-proof.json` (the same shape is embedded in the attestation and the manifest) |
| `summary.schema.json` | `summary.json` |
| `change.schema.json` | `change.json` |
| `export-index.schema.json` | `export-index.json` |
| `signers.schema.json` | `provenance/signers.json` |
| `attestation-gap.schema.json` | `attestation-gap.json` |
| `verification.schema.json` | `verification/verdicts.json` |
| `review.schema.json` | `review/review.json` |
| `provenance.schema.json` | `provenance/provenance.json` |
| `route-performance.schema.json` | `provenance/route-performance.json` (spec 1.1.0) |
| `vector-expected.schema.json` | `conformance/evidence/v1/vectors/<name>/expected.json` |
| `vectors-index.schema.json` | `conformance/evidence/v1/vectors/index.json` |

`tool-events.ndjson` and `diff.digest` have no schema here: they are streams and a digest line, defined in the prose spec.

## Unknown keys are errors

Every object sets `additionalProperties: false`, because the shipped decoders refuse unknown keys. A key a later minor adds is therefore a new schema version, not a silent extension. `required` lists the keys the writer always emits; keys the writer omits when empty or zero are optional.

## What a schema cannot say

The schemas use only the keyword subset of the validator in `sdk/go/schema` (see its README): no `const`, `oneOf`, `allOf`, length or count keywords, and a restricted pattern syntax. So these are checked by the verifier, and each schema's `description` lists them:

- canonical form of the file (key order, whitespace, escapes) and duplicate keys;
- hash and signature re-derivation, and the agreement of a file with the records;
- ordering, uniqueness and contiguity of sequences and entries;
- size bounds (string, array and file lengths).

A document that passes a schema is well shaped, not verified. Run an evidence verifier for that.

## Receipt lines

The outputs of a receipt record are strings of the `artifact` shape, so a schema cannot check the grammar of a `receipt/request/...` line. The verifier does (`docs/evidence/receipts.md`). The line has ten fields, or eleven when the last is the literal `injected` (added in 1.1.0); the schemas are unchanged.

## Tests

`go test ./schemas/...` from `numinance-spec/` compiles every schema, resolves every `$ref`, validates every file of the verifying golden exports and every conformance vector against its schema, and checks that hand-made bad documents are rejected. Exports that are tampered on purpose are not validated.
