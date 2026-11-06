# evidenceverify

The open verifier of an `evidence/v1` export directory. It answers one question for an auditor who holds an export and the public keys of the runner and the transparency log: does this directory verify, and how strongly. It needs no Numinance software and no network.

```go
rep, err := evidenceverify.Verify(ctx, dir, evidenceverify.Options{
	RunnerKeys: []ed25519.PublicKey{runnerKey}, // pinned out of band
	LogKey:     logKey,                         // pinned out of band
	LogOrigin:  "example.org/log",
})
if err != nil { /* usage, refused path, not found, unavailable, timeout: no report */ }
if rep.ExitCode() == 0 && rep.LevelEnum == "verified" && len(rep.WeakenedByFlags) == 0 { /* a full pass */ }
```

`numi evidence verify` in `../cli` is a thin wrapper over this package: it parses flags, reads key files and prints the report.

## What it is

| Property | How |
|---|---|
| Written from the public spec only | `docs/evidence/`, `schemas/evidence/v1/`, RFC-0003 and the vectors under `conformance/evidence/v1/`. It does not use the private implementation and has its own canonical-form and hashing code |
| Standard library only | no third-party module, no import of `numinance/`; a test fails on any import that is not the standard library or that is a networking or process package |
| Offline and read-only | the directory is opened through `os.Root`; nothing is written; links are never followed |
| Size-bounded | a file is read at most to its limit (1 MiB, 256 MiB for `records.ndjson` and `tool-events.ndjson`), a record line at most 3 MiB, a chain at most 32768 records, a directory at most 64 files |
| No panics | every decoding step returns an error that becomes a finding |

## Trust

There is no built-in trust root. `verified` needs pinned runner key(s), a pinned log key and a pinned log origin. A key the export states never reaches `verified`: `TrustBundleKeys` gives `self-consistent` at best and `SkipSignatures` gives `unauthenticated` at best. An allow flag suffixes the level only when it covers a weakness that is present, and a fully pinned, anchored, `Closed` export is a bare `verified` whatever the flags. A report's `ok` is true for a pass that carries an allowance; automation that needs a full pass tests `LevelEnum == "verified"` and an empty `WeakenedByFlags`, never a prefix of `Level`. A `verified` pass may still carry parts the index declares absent (`docs/evidence/verification.md` section 7): read `Absent` too, or set `Complete`, when every part is required.

Any pinned runner key may sign a record, the attestation and the manifest: `evidence/v1` has no per-role keys. `ExpectChange` and `ExpectHead` bind the result to a change or a head the caller knows from another source (`docs/evidence/verification.md` section 8).

## Files

| File | One use case |
|---|---|
| `verify.go` | `Verify`: the steps of `verification.md` section 2 in order |
| `options.go` | the options, their validation, key files |
| `report.go`, `level.go` | the report and the level algorithm of section 4 |
| `jsonparse.go`, `canon.go`, `model.go`, `specs.go` | strict decoding and the canonical form; the field rules of each document |
| `record.go`, `step_chain.go` | the record rules and the chain walk |
| `merkle.go`, `step_anchor.go` | inclusion proofs, checkpoints, the head anchor |
| `step_index.go` | index, summary, signers, `diff.digest` |
| `step_views.go`, `step_change.go` | the derived views, `tool-events.ndjson`, `change.json` |
| `step_attestation.go`, `step_manifest.go` | the attestation and the signed manifest |
| `step_parts.go`, `step_expect.go` | the parts of the design list; `--expect-change` and `--expect-head` |
| `bundle.go` | reading the directory |

## Tests

`go test -race ./sdk/go/evidenceverify/` from `numinance-spec/` runs every vector of `conformance/evidence/v1/` (`conformance_test.go`: exit code, level, finding codes, `ok`, notes, and for every vector the NEW `level_enum` and `weakened_by_flags`) and the cases no vector can hold: the record-count bound at its edge (`too_many_records`), deadlines and cancels (exit 8 and 1), option validation, key files, the canonical string rules, the strict parser, the tool-event stream split, Merkle proofs and the import guard.

The shape of a `TrajectoryCompared.v1` record (docs/evidence/trajectory.md) is checked with the other record rules (`trajectory.go`): the runner's succeeded record, inputs numbered `trajectory/stream/<i>` then `trajectory/candidate/<i>`, outputs in the fixed grammar with marker digests, and a result that agrees with its counts. The chain rules check that the `ev_id` is the one the change derives, that there is at most one statement and that it precedes the terminal record. The report carries the result (`trajectory`, a finding to read and never a failure). The comparison and the file set are not recomputed.
