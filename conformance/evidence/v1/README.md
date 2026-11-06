# evidence verification conformance vectors, v1

Data, not code. The acceptance vectors for any verifier of an `evidence/v1` export directory: the private `numi evidence verify` of the Numinance build and the open verifier. The spec they test is `../../../docs/evidence/` (start with `verification.md`); the shapes are `../../../schemas/evidence/v1/`. The kit version tracks the evidence spec: this is 1.0.0.

The open verifier (`../../../sdk/go/evidenceverify`) runs every vector, `target` ones included, in `conformance_test.go`. The reference implementation's own verifier runs them too, and a differential test runs both over every vector and over a deterministic mutation corpus; the engine build's `numi evidence verify` takes its verdict from the open verifier.

A verifier conforms when, for every vector of status `shipped`, it reports the exit code, the level and the set of finding codes in `expected.json`. It conforms to the NEW parts of the spec when it also does so for the vectors of status `target` and prints the `expect.target` values.

## Layout

```
README.md                   this file
VECTOR-COVERAGE.md          generated: which vectors cover which finding code, level, exit code and rule
keys/                       generated test public keys, one file each: 64 hex characters
exports/<name>/             golden export directories, as a verifier is given them
vectors/index.json          every vector, its status and what it covers
vectors/<name>/expected.json   one vector: the export, the flags and what to expect
```

The keys are public keys of generated test keys. No private key is stored, and none protects anything. A few key files are deliberately odd (`runner-uppercase.pub`, `runner-nbsp.pub`, `runner-oversize.pub`, `runner-short.pub`): they test how a key file is read, and the harness MUST read a key file as the shipped command does (a file over 1024 bytes is an error, never a truncated read).

## One vector

`vectors/<name>/expected.json` (schema `vector-expected.schema.json`):

| Key | Meaning |
|---|---|
| `vector` | the vector's name, equal to its directory |
| `description` | what the vector shows |
| `status` | `shipped`: the private verifier does this today and the Numinance continuous integration proves it. `target`: NEW behaviour (the `--json` fields `level_enum`, `weakened_by_flags` and `anchored_head_record_hash`, and `--expect-change` and `--expect-head`); both verifiers print it |
| `export` | the directory under `exports/` to verify; absent only when the vector is about a path that does not exist |
| `materialize` | steps to apply, in order, to a fresh copy of that export before verifying; see below |
| `flags` | the command line, structured; see below |
| `keys` | the key files under `keys/` the flags name |
| `expect.level` | the `level` string the report carries, or `null` when the run prints no report |
| `expect.exit_code` | the process exit code |
| `expect.finding_codes` | the set of finding codes, sorted, without duplicates; compare as sets, never as a list in order |
| `expect.credential_scopes`, `expect.network_policy` | optional: `sourced` or `unsourced`, the report notes; never a level |
| `expect.target` | NEW: the `level_enum` and `weakened_by_flags` a v2 `--json` report carries |
| `covers` | tags: `finding:<code>`, `level:<level>`, `exit:<n>`, `rule:<name>`, `mode:<name>` |

## Running a vector

1. Make a fresh temporary directory. Copy `exports/<export>` into it as `export`. Apply `materialize` in order.
2. Build the command line from `flags`: `--runner-key <root>/<file>` for each `runner_keys` entry, `--trust-bundle-keys`, `--skip-signatures`, `--log-key <root>/<file>`, `--log-origin <origin>`, `--allow-unanchored`, `--allow-incomplete`, `--allow-unsigned`, `--complete`, then (target vectors only) `--expect-change` and `--expect-head`, then `--json`, then the directory. `<root>` is this directory. Flag order does not matter.
3. Run the verifier. Compare the exit code, `level` (or no report when `expect.level` is null), the set of `findings[].code`, and `ok` (true exactly when the exit code is 0).

Vectors that give two key modes or none (`key-mode-*`) are expected to exit 2 and print no report: the exit code is the whole result.

## materialize

Things a data file cannot hold portably. Each step is an object with an `op`:

| op | Fields | Effect |
|---|---|---|
| `symlink` | `path`, `target` | replace the file `path` by a symbolic link to `target` |
| `mkdir` | `path` | create a directory |
| `pad_file` | `path`, `size` | set the file's length to `size` bytes (extending with zero bytes; a sparse file is fine) |
| `add_named_file` | `path` | create an empty file with that name (a name with a control character, for example) |
| `add_files` | `count` | add `count` empty files named `extra-000.txt`, `extra-001.txt`, ... |
| `export_path_symlink` | none | verify a symbolic link to the copied directory instead of the directory |
| `no_directory` | none | do not create the directory at all |
| `empty_path` | none | give the verifier the empty string as the directory path |

A harness on a system without symbolic links may skip the vectors that use them and says so.

## What the exports are

Each export under `exports/` is generated, deterministically, by the Numinance evidence code over in-memory adapters with fixed keys, fixed identifiers and a fixed clock. The base exports are verifying exports of a Closed change (`closed-basic`, `closed-complete`, `closed-sealed`, `closed-attested-fields`, `closed-ticket-none`, `closed-canon-unicode`, `closed-canon-timestamps`), an Abandoned, a RolledBack and an open (no terminal record) change, and a Closed change with an attestation gap. Every other export is one of them tampered in one stated way, with the index and the signed manifest rewritten or kept as the vector's description says. The generator is private; a vector's meaning is its `description` and its `expect`, not its generator.

The log origin of every vector is `numinance.test/log`. The runner key is `keys/runner.pub` and the log key `keys/log.pub`; `keys/other-runner.pub` signs only the export `closed-basic-other-runner-key` (an attacker's chain); `keys/third-runner.pub` and `keys/other-log.pub` sign nothing in any export and stand for keys a caller pins by mistake.

## What has no vector

`too_many_records` (more than 32768 records) has no vector because a chain that long is about 20 MB; exit codes 1, 6, 7 and 8 depend on the environment. `docs/evidence/verification.md` says how a verifier covers them. The release ordering rule of that page comes before any export leaves the project's own machines. The generated VECTOR-COVERAGE.md lists every case that has no data vector, with the reason.
