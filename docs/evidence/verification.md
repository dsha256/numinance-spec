# Verification: findings, levels, exit codes and trust limits

Status: normative for `evidence/v1` (spec 1.0.0), except where a section is marked NEW. This page describes what `numi evidence verify` in the shipped Numinance build does. Sections marked NEW are additions that the open verifier (`sdk/go/evidenceverify`) and, since the parity run, the reference implementation's own verifier both implement; the two gave the same level, exit code, finding codes, `level_enum`, `weakened_by_flags`, `ok` and notes on every vector and on a deterministic mutation corpus. The engine build's `numi evidence verify` now takes its verdict from the open verifier; the reference implementation's verifier stays as an independent oracle.

A verifier reads an export directory as untrusted input, offline, read-only and size-bounded, and reports every failure it can find. It never opens a network connection. It reports one result level and a set of findings.

The conformance vectors in `numinance-spec/conformance/evidence/v1/` are the test of this page. A verifier conforms when, for every vector of status `shipped`, it reports the exit code, the level and the set of finding codes in `expected.json`, and, when it implements the NEW sections, the same for the vectors of status `target`.

## 1. Inputs

| Option | Meaning |
|---|---|
| `DIR` | the export directory; exactly one |
| `--runner-key FILE` | a pinned runner public key (64 hex characters in the file); repeatable |
| `--trust-bundle-keys` | take the keys `provenance/signers.json` claims as trusted |
| `--skip-signatures` | do not check any signature |
| `--log-key FILE` and `--log-origin ORIGIN` | the pinned transparency log's public key and origin; both or neither |
| `--allow-unanchored` | accept a head not proved under a pinned log |
| `--allow-incomplete` | accept a chain with no terminal record |
| `--allow-unsigned` | accept the export of an `Abandoned` or `RolledBack` change, which has no signed manifest |
| `--complete` | fail when a part of the design list is absent |
| `--json` | print one JSON object instead of the text report |
| `--expect-change CHG_ID`, `--expect-head DIGEST` | NEW, section 8 |

### Key modes

The caller MUST choose exactly one of three key modes:

1. **Pinned.** One or more `--runner-key`. The verifier trusts exactly those keys. A record, the attestation and the manifest each name their signer by key id; the name MUST resolve to one of the pinned keys. Any pinned key may sign any of the three: `evidence/v1` has no per-role separation of keys, so pinning two keys trusts either for everything. A key the directory claims is never added.
2. **`--trust-bundle-keys`.** The keys of `provenance/signers.json` are trusted. This proves only that the directory is consistent with itself: an attacker who builds a whole chain and signs it with a key of their own passes. The best level is `self-consistent`. It is refused together with a pinned key or `--skip-signatures`.
3. **`--skip-signatures`.** No signature is checked. Links, digests and re-derivations still are. The best level is `unauthenticated`.

None, or two or more, is a usage error (exit 2): the verifier never defaults to trusting the directory. A key given on the command line is the only way a result reaches `verified`: **a key that comes from the bundle never reaches `verified`.** The same holds for the log: a log key and origin that the bundle states are never used.

The log pin (`--log-key` with `--log-origin`) is independent of the key mode. With it, the head proof and the attestation proofs are verified; without it they are not, and the result cannot be `verified` unless `--allow-unanchored` is given.

`--runner-key` and `--log-key` read a file of at most 1024 bytes. A file over 1024 bytes is an error and is never read as a truncated key, even when its first bytes are a valid key. The content, with white space trimmed from both ends (every Unicode white space character, such as U+00A0, not only ASCII), is hexadecimal of either case (uppercase is accepted) for exactly 32 bytes. A file that cannot be read, is too long or does not hold a key is a usage error (exit 2).

## 2. What the verifier checks, in order

1. Read the directory and refuse what cannot be read (links, bad names, too many entries). [bundle layout](bundle-layout.md) section 1.
2. Name check: every file is a known file.
3. Index: decode, then compare every file with its entry.
4. Summary: decode.
5. Chain: every record decodes, links, is signed by a trusted key and fits the bounds; then the summary and `diff.digest` are compared with what the records give.
6. Anchor: `signers.json` decodes; the two anchor files come as a pair; with a pinned log, they decode and the head proof verifies under it (without one they are not decoded).
7. Derived parts: the three views, `tool-events.ndjson`, `change.json`.
8. Attestation: decode, signature, re-derivation, per-change checkpoint and proof.
9. Ticket cross-check between `change.json` and the attestation.
10. Manifest: decode, signature, entries, binding, anchor files and proofs.
11. Parts: present or declared absent.
12. Level.

Steps that need a verified chain (the anchor proof, the derived parts, the attestation, the ticket cross-check and the manifest's content checks) do not run when the chain did not verify, and are reported as not checked. The other steps do not depend on the chain and always run: the directory and name checks, the index, the summary decode, the signers decode, the check that the two anchor files come as a pair and that a pinned log finds them, the presence of the attestation (or its gap) and of the manifest, and the parts. So one failure can come with others: a broken chain together with a missing file is both findings, and an index that does not decode makes the manifest unable to match any entry. The set of findings is normative (the vectors state it); their order in a report is not. Section 11 states each reading that fixes a case this paragraph leaves open.

## 3. Finding codes

Each finding has a code and a message. The code is normative; the message is for people and is not. A report has a finding exactly when the level is `failed`.

The "vector" column names one vector (directory `vectors/<name>`) that produces the code. [VECTOR-COVERAGE.md](../../conformance/evidence/v1/VECTOR-COVERAGE.md) lists every vector for every code.

| Code | Meaning | Vector |
|---|---|---|
| `directory_refused` | a link, a directory, a device, too many entries or a bad name in the export directory | `directory-refused-link` |
| `unknown_file` | a file the format does not define | `unknown-file` |
| `attestation_missing` | the chain ended Closed and `attestation.json` is not there (and no gap is stated) | `attestation-removed` |
| `attestation_invalid` | `attestation.json` does not decode, is signed by no trusted key, or its signature fails, or its embedded inclusion proof is not well formed (section 11, item 14); or a gap is stated for a change that did not end Closed (a chain that did not verify is not shown to have ended Closed), does not decode or names another change | `attestation-edited` |
| `attestation_mismatch` | the attestation disagrees with the chain (head, range, a re-derived field, a result commit no publication supports, or an attestation beside a gap) | `attestation-another-head` |
| `attestation_proof` | the attestation's checkpoint is missing, is not the one `checkpoint_ref` names, does not decode, or the head leaf does not verify under the pinned log | `attestation-checkpoint-missing` |
| `manifest_missing` | `attestation.json` is present, or the verified chain ended Closed with no valid gap, and `bundle-manifest.json` is not there | `manifest-missing` |
| `manifest_invalid` | `bundle-manifest.json` does not decode (an embedded inclusion proof that is not well formed included), is signed by no trusted key, or its signature fails | `manifest-edited` |
| `manifest_mismatch` | the manifest disagrees with the directory, the chain, the attestation or the checkpoint, or the attestation leaf does not verify under the pinned log | `file-changed-after-manifest` |
| `view_mismatch` | a derived view differs from its derivation, or is present with nothing to derive | `view-edited` |
| `change_invalid` | `change.json` does not decode, or disagrees with the chain or with the attestation's ticket | `change-json-edited` |
| `tool_events_mismatch` | `tool-events.ndjson` is not the concatenation of the streams the chain cites | `tool-events-edited` |
| `index_invalid` | `export-index.json` is missing, unreadable or does not decode | `index-missing` |
| `file_missing` | the index lists a file the directory lacks, or `records.ndjson` is missing or unreadable | `file-missing` |
| `file_unlisted` | the directory holds a known file the index does not list | `file-unlisted` |
| `digest_mismatch` | a file differs from its index entry in size or digest, or cannot be read to be hashed | `digest-mismatch` |
| `summary_invalid` | `summary.json` is missing, unreadable or does not decode | `summary-undecodable` |
| `change_mismatch` | the index and the summary name different changes | `index-names-another-change` |
| `record_invalid` | a record line is not a valid canonical record | `canon-key-order` |
| `chain_broken` | a sequence gap, a bad link, a reordered or edited record, a record after the end, an empty chain | `forged-record` |
| `signature_invalid` | a record's signature does not verify under the key its `signing_key_id` names | `signature-edited-in-place` |
| `signer_untrusted` | a record names a key that is not trusted | `wrong-runner-key` |
| `summary_mismatch` | the summary disagrees with what the records re-derive | `summary-mismatch` |
| `diff_mismatch` | `diff.digest` disagrees with the `execution.diff` record | `diff-digest-edited` |
| `part_missing` | a required part is neither present nor declared absent | `part-required-and-not-declared` |
| `part_contradiction` | a part is both present and declared absent, or declared not applicable when the chain does not bear it out | `part-present-and-declared-absent` |
| `parts_absent` | `--complete` and a part is absent | `complete-refuses-absent-parts` |
| `signers_invalid` | `provenance/signers.json` does not decode or its key ids are not the digests of its keys; missing when it is the trust source | `signers-bad-key-id` |
| `anchor_invalid` | the head checkpoint and proof come together and only one is present; or, with a pinned log, one does not decode or the head leaf does not verify under it | `head-proof-half-present` |
| `anchor_missing` | a log is pinned and the directory holds no head proof | `head-proof-absent-log-pinned` |
| `expectation_mismatch` | NEW: `--expect-change` or `--expect-head` was given and the chain is not the one expected (section 8) | `target-expect-change-mismatch` |
| `too_many_records` | more than 32768 records | none |

`too_many_records` has no shipped data vector: a chain of 32769 valid records is about 20 MB, and the test rules of the repository forbid a fixture that loops over the whole range. The shipped verifier tests the bound at its edge with a lowered record limit, in its own unit test; a verifier of this spec MUST bound the chain at 32768 records and MUST test the boundary the same way, with its own bounded option. The code is unreachable by any other means.

## 4. Result levels

A report has one level. These are the only levels:

| Level | Exit | Meaning | Vector |
|---|---|---|---|
| `verified` | 0 | signatures verified under pinned keys, the chain ended, the head proved under a pinned log, nothing allowed was needed | `closed-basic-verified` |
| `failed` | 3 | at least one finding | `forged-record` |
| `unauthenticated` | 5 | signatures not checked (`--skip-signatures`) | `skip-signatures-unauthenticated` |
| `self-consistent` | 5 | signatures verified only under keys the directory claims (`--trust-bundle-keys`) | `trust-bundle-keys-self-consistent` |
| `incomplete` | 5 | the chain has no terminal record (it may be a prefix of a longer chain), or the attestation of a Closed change was not made | `open-prefix-incomplete` |
| `unsigned` | 5 | the change ended Abandoned or RolledBack: it has no attestation and no signed manifest, so the files are vouched for only by the unsigned index | `abandoned-unsigned` |
| `unanchored` | 5 | the head was not proved under a pinned log | `closed-unanchored` |
| `verified, unanchored` | 0 | a pass in which `--allow-unanchored` covered a missing anchor | `closed-allow-unanchored` |
| `verified, incomplete` | 0 | a pass in which `--allow-incomplete` covered a chain with no terminal record | `open-prefix-allow-incomplete` |
| `verified, unsigned export` | 0 | a pass in which `--allow-unsigned` covered an Abandoned or RolledBack change | `abandoned-allow-unsigned` |
| `verified, unanchored, incomplete` | 0 | both of the first two | `open-prefix-unanchored-allow-both` |
| `verified, unanchored, unsigned export` | 0 | `--allow-unanchored` and `--allow-unsigned` | `abandoned-unanchored-allow-both` |

(`incomplete` and `unsigned export` cannot both apply: one needs a chain with no terminal record, the other a terminal record other than Closed.)

### Precedence and the algorithm

The level is computed as follows. It is the only correct computation: two verifiers that implement it agree on every directory.

```
if any finding:           level = failed
else
    weaknesses = []        # in the order added; the first one names the level
    allowed    = []        # suffixes of a pass, in the order below

    1. if --skip-signatures:         add unauthenticated
       elif --trust-bundle-keys:     add self-consistent
    2. if the directory states an attestation gap for a Closed change:
                                     add incomplete          (no flag covers this)
    3. if the chain has not ended (no terminal record, no seal record):
         if --allow-incomplete:      allowed += "incomplete"
         else:                       add incomplete
    4. if the export is not fully anchored (below):
         if --allow-unanchored:      allowed  = ["unanchored"] + allowed
         else:                       add unanchored
    5. if the chain ended and its terminal state is not Closed:
         if --allow-unsigned:        allowed += "unsigned export"
         else:                       add unsigned

    if weaknesses is empty: level = "verified" + (", " + allowed joined with ", " if allowed is not empty)
    else:                   level = weaknesses[0]
```

Consequences a verifier MUST preserve:

- **A flag covers only a weakness that is present.** A suffix appears exactly when its flag was given and the weakness it covers exists. A flag given but not needed leaves a bare `verified` (vectors `closed-allow-incomplete-not-needed`, `closed-allow-unsigned-not-needed`, `closed-allow-all-not-needed`). A fully pinned, anchored, Closed export verified with every allow flag set is a bare `verified`.
- **Suffix order is fixed:** `unanchored`, then `incomplete`, then `unsigned export`. They are joined by a comma and a space.
- **A flag never rescues a weaker level than the one it covers.** `--allow-unanchored` does not cover a missing terminal record (vector `open-prefix-unanchored-allow-unanchored-only` stays `incomplete`); `--allow-incomplete` does not cover a missing anchor (`open-prefix-unanchored-allow-incomplete-only` stays `unanchored`); `--allow-incomplete` does not cover an attestation gap (`closed-gap-allow-flags` stays `incomplete`).
- **No allow flag lifts `unauthenticated` or `self-consistent`**, whatever the flags (vector `skip-signatures-allow-flags-stay-unauthenticated`). Those levels name the first weakness, so the other weaknesses are reported in `weaknesses` and not in the level.
- **No allow flag turns a failure into anything else.** An export with a finding is `failed` and exits 3 whatever flags were given (vector `forged-record-allow-flags-stay-failed`). `--allow-unanchored` alone also keeps every protection of the signed manifest and of the terminal-record rule: a truncated Closed export is `failed` under it (vector `truncated-closed-allow-unanchored-only`).
- **The level of a pass is never a prefix test.** The strings `verified, unanchored`, `verified, incomplete` and `verified, unsigned export` are passes with a named allowance and exit 0. Automation MUST NOT read a result that starts with `verified` as a full pass: see section 6.

### Fully anchored

An export is fully anchored when the head proof verified under the pinned log (`head-checkpoint.json` and `head-inclusion-proof.json`, [log](log-and-checkpoints.md)) and, for a change that has an attestation or that ended Closed, the attestation's own proof verified under its per-change checkpoint and the manifest's attestation proof verified under the head checkpoint. Without a pinned log nothing is anchored. A Closed change with an attestation gap has no attestation and no manifest to prove, so it is never fully anchored.

### Weaknesses

Independently of the level, the report lists every weakness that the algorithm added, in `weaknesses`, one sentence each, in the order added. The level names the first one. A weakness that an allow flag covered is not a weakness and is not listed; it is named in the level of the pass instead. The sentences are for people and are not normative.

## 5. Exit codes

The exit codes of `numi evidence verify`, checked against the source of the shipped command (its exit-code mapping and its verify subcommand):

| Exit | Name | When | Vector |
|---|---|---|---|
| 0 | OK | a report with no finding whose level starts with `verified`, with or without suffixes | `closed-basic-verified` |
| 1 | internal | an error that is none of the kinds below: an internal error, a cancelled context, a report that cannot be encoded | none |
| 2 | usage | an unknown flag or a flag parse error; not exactly one directory; a key file that cannot be read or is not 64 hex characters; `--log-key` without `--log-origin` or the reverse; no key mode, or two or more key modes; no subcommand or an unknown one | `key-mode-none-is-usage-error` |
| 3 | invalid | a report with at least one finding (level `failed`); or the directory path itself is refused before any report (it is not a directory, it is a link, or it changed while it was opened) | `forged-record`, `directory-path-is-a-link` |
| 4 | not found | the directory path does not exist | `directory-missing` |
| 5 | weak | a report with no finding whose level does not start with `verified`: `unauthenticated`, `self-consistent`, `incomplete`, `unsigned` or `unanchored` | `closed-unanchored` |
| 6 | denied | not produced by verify | none |
| 7 | unavailable | the directory could not be read for a reason that is not a refusal (an I/O error on listing or reading) | none |
| 8 | timeout | the context deadline was exceeded | none |

Exit 5 has a second meaning in the table of `numi`: a conflict. `verify` never produces a conflict. For `verify`, 5 means the directory is consistent and the check was weaker than `verified`.

Exits 1, 6, 7 and 8 depend on the environment and have no data vector. A verifier maps its errors to the same codes.

## 6. Reports

### The text report

The text report lists, in this order: the change, the records and head, the state, the level, the chain, signatures, head anchor and attestation lines, the credential scopes note, the parts that are not applicable, the absent parts with their reasons and owners, every finding as `FAIL <code>: <message>`, every weakness as `WEAK <sentence>`, and a last line `result: ...`. The wording is for people and is not normative.

### The JSON report as shipped

With `--json` the verifier prints one object:

| Key | Type | Meaning |
|---|---|---|
| `ok` | boolean | true exactly when exit is 0: the pass, with or without allowances |
| `change_id`, `records`, `head_record_hash` | string, integer, string | the verified chain (`head_record_hash` is the last record's) |
| `ended`, `terminal_state`, `sealed` | boolean, string, boolean | what the records state |
| `level` | string | the level string of section 4, suffixes included |
| `weaknesses` | array of string | sentences, absent when empty |
| `chain`, `signatures`, `anchor`, `attestation` | string | what was proved of each, for people |
| `absent`, `not_applicable` | array of `{part, reason, owner, not_applicable?}` | the parts |
| `credential_scopes` | string | the broker-statement note: `unsourced...` when no broker statement is in the attestation, otherwise a line starting `digest line and` |
| `network_policy` | string | the egress-proxy note: `unsourced...` when the attestation holds none, otherwise a sentence that starts `reference to this change` and says the digest cannot be re-derived from the bundle |
| `trajectory` | string | the result of the trajectory statement when the chain holds one (`corroborated`, `mismatch (N discrepancies ...)`, `unverifiable (reason)`); absent for a chain without one, which is valid. A finding to read, never a failure |
| `findings` | array of `{code, message}` | absent when empty |

Three things automation MUST know about the shipped report:

- `ok` is true and the exit is 0 for `verified, unanchored`, `verified, incomplete` and `verified, unsigned export`. An allowance-bearing pass exits 0, so exit code and `ok` alone cannot tell it from a plain pass.
- `level` is a display string. A prefix match on `verified` also matches the allowance-bearing passes.
- The vectors assert `level`, `ok`, `findings[].code` and the exit code.

### The JSON report, NEW

NEW behaviour, added with the parity work. A report adds three keys:

| Key | Type | Meaning |
|---|---|---|
| `anchored_head_record_hash` | string | the anchored hash of section 8 |
| `level_enum` | string | one of `verified`, `failed`, `unauthenticated`, `self-consistent`, `incomplete`, `unsigned`, `unanchored`: the level without any suffix |
| `weakened_by_flags` | array of string | the suffixes of the level, in the fixed order, each one of `unanchored`, `incomplete`, `unsigned export`; empty when none |

For every vector the `expect.target` object holds the values the report must carry. A full pass is exactly `exit 0`, `level_enum == "verified"` and `weakened_by_flags == []`. Automation that needs a full pass MUST test those three, never a prefix of `level`, and a pipeline that accepts an allowance MUST say which one it accepts. `ok` keeps its shipped meaning (a pass, possibly allowance-bearing) so no consumer of the shipped report changes behaviour. A report is printed only when the verifier got as far as verifying a directory: an exit 2, 3 (refused path) or 4 prints none.

The text report SHOULD end an allowance-bearing pass with a line that states it, as the shipped text report already does for partial exports.

## 7. Absent parts and `--complete`

A part declared absent never fails verification by itself: the report lists it (`absent`), says its reason and owner, and the last line says `PARTIAL`. `--complete` fails with `parts_absent` when any part is absent; a part that is not applicable (deployment and observation of a change closed at PR level) is not absent. A bundle that holds every part is the only one `--complete` accepts (vector `closed-complete-verified`).

## 8. `--expect-change` and `--expect-head` (NEW)

NEW behaviour, added with the parity work. They let a caller who knows which change, or which head, it is verifying say so, so that a swap for another consistent export is a failure. Without them a verifier cannot tell an export from another genuine export of the same change signed by the same keys (vector `another-chain-of-the-same-change`, which verifies).

- `--expect-change CHG_ID`: after the chain verified, its change id MUST equal `CHG_ID`. Otherwise the report has a finding with the code `expectation_mismatch`, the level is `failed`, and the exit is 3.
- `--expect-head DIGEST`: after the chain verified, `DIGEST` MUST equal either `head_record_hash` of the report (the `record_hash` of the last record, which `summary.json` also states) or the anchored hash (the `record_hash` of the record that the head leaf and the attestation hold). The two are the same for a chain that is not sealed. For a sealed chain they differ: the last record is the seal record, and the anchored hash is the terminal record before it, so a caller who got the head from the log or from an attestation gives the anchored hash and a caller who got it from `summary.json` gives the last record's. A verifier that implements this prints the anchored hash as `anchored_head_record_hash` in its JSON report (NEW, with `level_enum`). Otherwise `expectation_mismatch`, `failed`, exit 3 (vectors `target-expect-head-mismatch`, `target-expect-head-catches-another-chain` and, for a sealed chain, `target-expect-head-sealed-last-record`, `target-expect-head-sealed-terminal-record` and `target-expect-head-sealed-other`).
- An auditor who is given only the terminal record's hash binds nothing about the seal record's content: the head leaf and the attestation commit to the terminal record, and the seal record after it is covered only by the record signatures and the chain links.
- A value that is not a well-formed change id or digest is a usage error (exit 2).
- The flags are checked in every key mode. They add one finding code, `expectation_mismatch`, to the table above.

They are `target` vectors: the open verifier and the private verifier both run them.

## 9. Trust limits

What a verifier can and cannot show, stated so that no reader believes more than the format gives.

1. **Keys come out of band.** `verified` means the signatures verified under keys the caller pinned and the head proved under a log the caller pinned. If the caller pins the wrong key, `verified` says nothing. A key, a log key or a log origin stated by the bundle is never used for `verified`.
2. **No rotation, expiry or revocation in v1.** A pinned key is trusted for every signature it made, whenever it was made. A key that was rotated or revoked is handled by the caller (pin the keys that were valid, repeatedly). A compromised runner key can sign any record, attestation and manifest.
3. **No per-role key separation.** Any pinned runner key may sign records, the attestation and the manifest. The log key is a separate pin.
4. **A customer who holds both keys can equivocate.** The runner key and the log key together build a second consistent chain, a second log and a second export for the same change. A verifier cannot tell them apart: both verify. Only a witness outside the customer's control (a published checkpoint, or the head obtained from a second source and passed as `--expect-head`) can. Witness cosignatures are not part of `evidence/v1`.
5. **An unsigned export is as strong as its index, its record signatures and its log pin.** A change that ended Abandoned or RolledBack has no manifest. A prefix of such a chain, with `summary.json` and the index rewritten to agree, is not detected unless the log is pinned. The level never hides it: it is `incomplete`, or, when the caller allowed it, `verified, unanchored, incomplete` (vector `truncated-abandoned-flags-visible`). A prefix whose own head the log happens to hold verifies at best as `incomplete` or `verified, incomplete`, never as `verified`.
6. **A Closed export's completeness rests on the manifest and the attestation.** They name the first and last sequence and the terminal record's hash, and only the runner key signs them, so a truncation or an edit fails without a log key (vectors `truncated-closed-summary-rewritten-runner-key-only` and `truncated-closed-allow-unanchored-only`). Whole-export replacement by another export of the same change is not detected without the log (and not at all against equivocation, item 4).
7. **The leaf fields are the runner's statements.** `ticket`, `repository`, the commits, `toolchain_digest`, `ci_run_ids`, `deployment_ids`, `credential_scopes`, `network_policy` and `postgres_transport` (`verify-full` or `loopback-plaintext`) are bound by the runner's signature and by the attestation leaf; a verifier checks their form, not their truth. Reconciliation against the SCM, the CI executor, the cloud and IAM is a separate workflow.
8. **Timestamps are statements.** No timestamp is checked against a clock.
9. **Forks.** Two chains for one change, each signed by the pinned key, each verify. The head leaf in the log is what a log pin and `--expect-head` use to choose.

## 10. Release ordering rule

This rule governs, and no other page loosens it. No copy of an export, including the output of the development engine's export directory flag, leaves the machines of the project's own maintainers until all of these hold: this specification with its schemas, RFC and vectors; the open verifier; and the private verifier's parity and delegation are merged, with parity on every vector and every finding code; a release tag chosen by the project exists; and `ticket`, `credential_scopes` and `network_policy` are sourced by their owners (the ticket intake, the credential broker and the egress proxy).

## 11. Readings fixed by the vectors

Added with the open verifier. Each item states a reading that the spec text above leaves open and that a vector fixes; no behaviour of the format changes. The vector named is the one that decides it.

1. **A chain is walked only for a change the summary names.** The chain rules are applied to the change that `summary.json` states. When `summary.json` is missing or does not decode, the chain is not walked: no chain finding is reported, and everything that needs a verified chain is not checked (vector `forged-record-and-missing-summary`: `summary_invalid` and `part_missing`, no `chain_broken`).
2. **The first chain failure ends the walk.** A chain reports one chain finding (`record_invalid`, `chain_broken`, `signature_invalid`, `signer_untrusted`, `file_missing` for `records.ndjson` or `too_many_records`); later records are not read.
3. **Every part of the design list applies, whether or not the chain can derive it,** except `rollback.json`, which applies only to a change that ended `RolledBack`. A part is held or declared absent; a diff digest that cannot be derived because the chain failed still makes a missing `diff.digest` a `part_missing` (vector `forged-record-and-missing-file`).
4. **"Verified attestation".** In the manifest check 4 and in the ticket cross-check, an attestation is verified when it decoded, its signature held and every re-derived field equalled the chain. Its own checkpoint proof is not part of that: a wrong log key gives `attestation_proof` and also `manifest_mismatch` for the attestation leaf under the head checkpoint (vector `wrong-log-key`). When the re-derivation differs, the per-change checkpoint is not checked (vector `attestation-another-head`: `attestation_mismatch` only).
5. **`reason` and `owner` of an absent part** are 1 to 512 bytes of valid UTF-8 and need not be printable (vector `canon-control-escapes-verified`). The `reason` of an attestation gap must be printable text.
6. **A manifest is checked after its signature.** When it does not decode or its signature fails, none of its content checks run (vectors `manifest-edited`, `manifest-signed-by-other-key`: `manifest_invalid` only).
7. **A full pass can still be partial.** `verified` with an empty `weakened_by_flags` may carry parts that the index declares absent (section 7); the report lists them in `absent` and the text report says `PARTIAL`. Automation that needs every part must also read `absent` (or pass `--complete`), not only `level_enum` and `weakened_by_flags`.
8. **A path is refused whatever its spelling.** A symbolic link as the export path is refused (exit 3, no report) however it is written, including with a trailing separator, `/.` or a doubled separator; the path is cleaned before it is checked.

### Added with the parity run

The differential test of the two verifiers found the readings below to be open in the text above, chosen differently by the two implementations or by neither on purpose. Each is fixed by a vector. None changes a canonical byte, a hash, a signature or a record format: only the finding codes of exports that fail anyway, and the handling of a few degenerate inputs, are settled. Where the two implementations differed, the reading is the one that the spec text above says most plainly; the verifier that read it otherwise was changed.

9. **`not_applicable` is for `deployment.json` and `observation.json` only,** and a declaration on any other part that applies to the change is `part_contradiction`, not `index_invalid` (the index decodes). The refused declaration counts as an absent part, so `--complete` adds `parts_absent` (vectors `not-applicable-on-change-json`, `not-applicable-on-change-json-complete`).
10. **A not applicable claim needs a verified chain.** When the chain did not verify, or `summary.json` cannot be read, nothing shows that the change closed at pull-request level, so each claim is `part_contradiction` and counts as absent (vectors `not-applicable-chain-broken`, `not-applicable-chain-broken-complete`).
11. **A part that does not apply to the change is not evaluated.** `rollback.json` applies only to a change that ended `RolledBack`. The ending is read from the verified chain, or, when the chain did not verify, from `terminal_state` of a `summary.json` that decodes; when neither is known the part does not apply. A declaration for a part that does not apply is ignored (vectors `rollback-json-declared-on-a-closed-change`, `rolled-back-rollback-json-not-applicable`, `summary-says-rolled-back-chain-open`).
12. **The presence checks run without a verified chain** (section 2). A gap beside an attestation is `attestation_mismatch` and nothing else about the attestation is checked. A gap beside a chain that did not verify, or that did not end Closed, is `attestation_invalid`. `manifest_missing` is reported when `attestation.json` is present, whatever the chain, and when a verified chain ended Closed without a valid gap. `attestation_missing` needs a verified chain that ended Closed, so with a broken chain it is not reported (vectors `gap-chain-broken`, `attestation-and-gap-chain-broken`, `manifest-removed-chain-broken`, `attestation-and-manifest-removed-chain-broken`, `abandoned-with-an-attestation`).
13. **`provenance/signers.json` is decoded whenever it is present or is the trust source,** whether or not the chain can be read: in `--trust-bundle-keys` mode a missing or invalid file is `signers_invalid` even when `records.ndjson` is missing (vector `signers-missing-no-records`).
14. **The inclusion proofs embedded in the attestation and the manifest are shape-checked when the document is decoded,** with or without a pinned log, by the shape rules of the log page, section 3: a proof that fails them is `attestation_invalid` or `manifest_invalid` (vectors `attestation-proof-path-too-long`, `manifest-proof-path-too-long`). A proof that is well formed and does not verify under the pinned log is `attestation_proof` or `manifest_mismatch` as before.
15. **`change.json` is compared with the records, not with the summary file.** The state it states is the state the verified chain gives; a `summary.json` that disagrees with the chain is `summary_mismatch` and does not move what `change.json` is compared with (vector `change-json-stale-summary`).
16. **`change.json` with no `ticket` beside a verified attestation is `change_invalid`:** the ticket of the two documents must be equal, and an absent ticket is not equal to any stated one (vector `change-json-no-ticket`).
17. **The `credential_scopes` and `network_policy` notes read the attestation as it decoded and, unless signatures are skipped, as a trusted key signed it,** even when it then fails re-derivation or its proof. With no such attestation both notes say `unsourced` (vector `attestation-another-head-sourced-notes`).
18. **The order of the entries of the index, of its declared parts and of the entries of the manifest is not checked.** An exporter writes them sorted by path; the schema descriptions say "sorted by path" as a statement about what an exporter writes, not as a rule a verifier enforces (vector `index-entries-reversed`).
19. **A `diff.digest` file that holds one line feed and nothing else** is a file that states no digest: when the chain holds no `execution.diff` record it is `diff_mismatch` like any other `diff.digest` in that chain (vector `diff-digest-one-newline`).
20. **Every part of the design list applies whether or not the chain can derive it** (item 3), so a view that is neither present nor declared absent is `part_missing` although the chain holds nothing to derive it from (vector `view-required-though-not-derivable`).
21. **An empty directory path is a usage error** (exit 2, no report). It is never read as the current directory (vector `path-empty`).
