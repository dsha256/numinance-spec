# RFC-0004: the EvalSet kind

- Status: accepted (ADR-0047). The proto, generated schema, example and `kindyaml` loader have landed; the move of `numinance/evals/*/v1/` fixtures to the envelope and the evals context loader are still owed.
- Author: the project maintainers
- Opened: 2026-11-03
- Change class: contract-additive (a new kind message at `numinance.dev/v1alpha1`; no existing message changes)

## Problem
The replay harness reads an eval set file that carries `apiVersion` and `kind` and is checked in Go code only. Every config file must validate against a JSON Schema generated from a kind proto, and a customer or a third party writing eval sets needs the public contract.

## Proposal
Add message `EvalSet` to `numinance.kinds.v1` with the standard envelope and `api_version` documented as `numinance.dev/v1alpha1`.

- `spec.version` (string, since 1.0.0): strict x.y.z; any change to a case is a new version.
- `spec.cases[]` (since 1.0.0), sorted by `name`, 1 to 256: `name` (token), `outcome` (a `Digest`: the replay outcome the recording produces under the set and case), `expect` (`acceptance` enum, `merged`, `severe`, `verification_failures[]` of `{layer, count >= 1}` sorted by layer), optional `category` (enum of the eleven adversarial situations: hidden acceptance, incomplete ticket, conflicting ticket, poisoned README, malicious comment, stale documentation, flaky test, planted security bug, migration hazard, missing permission, rollback) and optional `seeded` `{defect, caught_by}` tokens.
- Tokens are lowercase letters, digits and `. _ -`, at most 64. No field holds free text, a ticket, code or an identifier of a person: a case names a recorded run by digest.
- Loader checks (added to `sdk/go/kindyaml`, `eval_set.go`): sorted unique names, a planted security bug or migration hazard is seeded, a seeded case expects not merged, not severe, not accepted, and its `caught_by` layer among its expected failures.
- Alpha handling: the loader accepts `numinance.dev/v1alpha1` for EvalSet only through an explicit option, off by default, set only by the evals composition root in `cmd/`.
- Scope: `metadata.scope` applies as in section 8. EvalSet takes no part in profile resolution and is not an input to `profile_digest`.
- Example under `schemas/examples/EvalSet/` and a conformance note; `just gen` produces the schema and SDK stubs.

## Alternatives
- Embed the set in the Playbook kind: couples a replay corpus to a playbook release.
- A free-form `description` per case: free text could carry poisoned material into an eval set; the category and defect tokens carry the meaning.

## Compatibility and migration
Additive. The current flat file (`name`, `version`, `cases` at the top level) has no consumer outside this repository; `numinance/evals/*/v1/` fixtures move under `metadata` and `spec` in the PR that lands the kind, with the digests re-pinned. New set versions and bundle digests follow; replay outcomes under the old digests are not catch evidence for the new bundle, so fixtures are replayed again before any proposal uses them.

## Security considerations
Untrusted-input parsing: size-limited, strict, unknown fields refused, errors never quote the input. Reference-only content is the leak control. Tests: one failing case per loader check, the size boundary, and the leak test over the fixtures.

## Open questions
None. The alpha gate is settled: an explicit loader option, off by default (ADR-0047 acceptance).
