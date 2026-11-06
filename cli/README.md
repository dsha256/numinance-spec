# numi

The Numinance command line. This directory is the open build; the command logic lives in the public Go package `sdk/go/cli`, so the engine build runs the same code and adds the parts that need the private profile context.

## Build

| Build | Where | `numi validate` does |
|---|---|---|
| Open | `numinance-spec/cli/numi` | Schema-validates every kind document of the org, repo and component trees, then exits 7 with "profile resolution needs the Numinance engine build". A CI that uses it cannot mistake a schema pass for a full one. With `--files` it exits 0 after the schema check only: no resolver, no floors, not a substitute for repository validation. |
| Engine | the engine build's `numi` | Reads a pinned commit (not the working tree), resolves the profile, prints the digest and the provenance of each field (with the reason when a contribution was ignored). Exit 7 there means a dependency was unavailable. |

## Usage

```
numi validate [--rev REV] [--org DIR] [--component ROOT] [--json] [path]
numi validate --files [--json] path
numi version
```

`path` is the repository (default `.`). `--org` is the organization policy repository. `--component` is the repository-relative component root, for example `svc/api`. Flags may come before or after the path. `--json` prints one JSON object; the default output is for people.

`--rev` defaults to empty, meaning the source's own default: the working tree in the open build, `HEAD` of a hardened mirror in the engine build. The open build refuses any non-empty `--rev` as a usage error, exit 2 (it cannot read a commit; `--rev ""` is the default), so `--rev` never silently means something else. It reads `.numinance/` of the directories given, so uncommitted edits are seen; the output says "working tree". It refuses a symbolic link as the config directory, anywhere on the `--component` path, or as a document, and any file that is not regular (checked on the open descriptor, without blocking on a FIFO). Files are read through `os.Root`. Bounds across all trees: 4096 entries, 256 documents, 16 MiB, and `kindyaml.MaxBytes` per document. Names, paths and messages from documents are cut and escaped before printing (control, bidi and zero-width characters become `\u` escapes, and `\` and `"` are written `\\` and `\"`, so a name cannot pass for another); schema errors name a path and keyword and never a value. A panic or an error of no kind exits 1, never 0.

## Example validator (`--files`)

`numi validate --files PATH` schema-checks kind documents that are not a repository: `PATH` is one YAML file or a directory, and every `.yaml` or `.yml` file below it is a document. It checks each against its generated kind schema (unknown fields are errors unless under `extensions:`) and the loader checks, runs no resolver, and exits 0 when all pass, 3 when any fails, 4 when the path is missing or holds no document, 2 for a usage error (no path, more than one, or combined with `--rev`, `--org` or `--component`). A produced kind (`ContextBundle`) is never loaded as configuration; it is checked against its own strict schema, `apiVersion`, `spec`, the required fields of the spec and of `BundleSource`, digests, enums and the `BundleSource` trust pin instead; the other constraints of the bundle (`AuthoredConstraint` trust, origin and origin digest, `CitedGuidance.ref`, `UnresolvedQuestion.question`, unique `BundleSource.ref`, at least two claims per conflict) are left to the ContextBundle producer. This is a schema and loader check only: floors and profile resolution are not checked, and a pass is not a substitute for validating a repository. A single file must be named `*.yaml` or `*.yml` (exit 2 otherwise); in a directory, the repository mode ignores such files, this mode refuses them. The path is cleaned once (`a/../b` means `b`) and the directory opened is the one that was checked and walked. A link is refused for the path itself (a trailing slash does not hide it) and for everything below it; links in parent folders of the path are followed. In a directory, a name that mentions `.yaml` or `.yml` in any letter case but does not end exactly in lowercase `.yaml` or `.yml` (`.YAML`, `.Yml`, `.yaml.bak`, `.yaml-old`, a trailing space) is invalid (exit 3); other names (`README.md~`) are ignored. Reading has the limits of the working-tree source: `os.Root`, no links, regular files only, 4096 entries, 256 documents, 16 MiB, `kindyaml.MaxBytes` per document. `just schema-check` runs it on `schemas/examples`, so CI validates every example. The engine build does not set a files source and exits 2 for `--files`.

## Exit codes (stable)

| Code | Meaning |
|---|---|
| 0 | OK |
| 1 | Internal, and any unknown kind |
| 2 | Usage (bad flags or arguments; in the open build also any non-empty `--rev`) |
| 3 | Invalid (schema errors, floor violations) |
| 4 | NotFound (also: no kind documents found) |
| 5 | Conflict |
| 6 | PolicyDenied |
| 7 | Unavailable. In the open build exactly one meaning: the schema pass succeeded and profile resolution is not available. A cancel (SIGINT, SIGTERM) exits 1, never 7 |
| 8 | Timeout |

`numi validate --help` prints the usage and exits 0. Flags may follow the path.

## Evidence

`numi evidence verify [flags] DIR` checks a section 16 export directory offline, read-only and with bounded reads. It runs in the open build and in the engine build; the open build's command is a thin wrapper over the public package `sdk/go/evidenceverify`, which was written from the public specification (`docs/evidence/`, `schemas/evidence/v1/`, RFC-0003) and passes every conformance vector (`conformance/evidence/v1/`). The open `numi` therefore now reports a chain result, and only with keys the caller pins.

```
numi evidence verify --runner-key FILE [--runner-key FILE ...] --log-key FILE --log-origin ORIGIN DIR
```

| Flag | Meaning |
|---|---|
| `--runner-key FILE` | a pinned runner public key (64 hex characters, at most 1024 bytes); repeatable; any pinned key may sign any record, the attestation or the manifest |
| `--trust-bundle-keys` | trust the keys the export states; the best level is `self-consistent` |
| `--skip-signatures` | check no signature; the best level is `unauthenticated` |
| `--log-key FILE`, `--log-origin ORIGIN` | the pinned transparency log, both or neither |
| `--allow-unanchored`, `--allow-incomplete`, `--allow-unsigned` | accept that weakness; the level is suffixed only when the weakness is present |
| `--complete` | fail when a part of the design list is absent |
| `--expect-change CHG_ID`, `--expect-head DIGEST` | fail unless the chain is that change, or its last or anchored head hash is that digest |
| `--json` | print one JSON object, with `level_enum`, `weakened_by_flags` and `anchored_head_record_hash` |

Exactly one key mode is required (none, or two, is exit 2). There is no built-in trust root: `verified` needs the pinned runner key(s), the log key and the log origin, and a key that comes from the export never reaches it. An allowance-bearing pass such as `verified, unanchored` exits 0, so automation that needs a full pass checks `level_enum == "verified"` and an empty `weakened_by_flags`, never a prefix of `level`. A `verified` result may still carry declared-absent parts (`docs/evidence/verification.md` section 7): automation must also read the `absent` list in the JSON report (or pass `--complete`).

Exit codes of `verify`: 0 a pass (with or without an allowance), 1 internal or cancel, 2 usage, 3 a report with a finding or a refused directory path, 4 the directory does not exist, 5 a consistent export weaker than `verified`, 7 an I/O error, 8 timeout. 6 is never produced. The full table, the finding codes and the level algorithm are in `docs/evidence/verification.md`.

`numi evidence export` is not in the open build (an unknown subcommand, exit 2); the engine exports with `-evidence-export-dir` and the engine build's `numi` runs its own `verify` until a later release makes it delegate to the public package. No export leaves the project's own machines before the release ordering rule holds: the specification, the open verifier and a release tag exist, and the ticket, credential scopes and network policy of the export are sourced by their owners (see [verification.md](../docs/evidence/verification.md) section 10).

## Engine build

The engine build's `numi` is private (the profile resolver is the Numinance engine's). It differs from the open build in five ways:

- It reads a private copy of the repository's `HEAD`, refs and objects, never the working tree and never by running git in the repository. Pass the git directory (or a working tree with a `.git` directory); a `.git` file is refused (exit 3). `--rev` defaults to `HEAD` and must be `HEAD` or a plain branch or tag name (letters, digits, `. _ / -`, starting with a letter or digit, no `..`); `refs/heads/X` and `refs/tags/X` disambiguate a name that is both. A full object id is not a revision. Exit codes of this build: 3 for a revision expression or option, a link, hard link, alternates or shared object store, a partial clone, an object that fails fsck or is larger than git may allocate (256 MiB limit), or a repository over 1 GiB or 200000 files; 4 for an unknown revision or no repository; 7 for a missing git binary or a repository that changed during the copy (retry); 8 when the 60 s run, copy and fsck included, expires; 1 for a cancel. Shallow clones work; SHA-256 repositories are not supported. `--org` is read at its `HEAD`.
- `--` ends flag parsing in both builds: everything after it is a path, even `-x` or `--json`. A value-taking flag (`--rev`, `--org`, `--component`) consumes the next argument whatever it looks like.
- The `Resolver` receives the `Snapshot` that passed the schema check (`Resolve(ctx, req, snap)`) and never re-reads the trees, so the documents checked are the documents resolved. This changed the `Resolver` interface of the public Go package `sdk/go/cli`: a breaking change for an implementer, allowed because no tag exists yet and there are no external implementers (`VERSIONING.md`, untagged packages).
- `--files` is not available: the engine build sets no files source and exits 2.
- `FieldSource.Ignored` is the reason a contribution did not take effect (no authority, outside the org-granted set); it is printed as `(ignored: "reason")` and in `--json` as `ignored`.

Tests: `go test ./cli/... ./sdk/go/cli ./sdk/go/evidenceverify` from `numinance-spec/`.
