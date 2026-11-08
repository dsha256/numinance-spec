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
numi init --ci github-actions|jenkins|both [--org-dir DIR] [--package-manager uv|pip|poetry] [--python-version 3.N] [--verifiers LIST] [--force] [path]
numi version
```

`path` is the repository (default `.`). `--org` is the organization policy repository. `--component` is the repository-relative component root, for example `svc/api`. Flags may come before or after the path. `--json` prints one JSON object; the default output is for people.

`--rev` defaults to empty, meaning the source's own default: the working tree in the open build, `HEAD` of a hardened mirror in the engine build. The open build refuses any non-empty `--rev` as a usage error, exit 2 (it cannot read a commit; `--rev ""` is the default), so `--rev` never silently means something else. It reads `.numinance/` of the directories given, so uncommitted edits are seen; the output says "working tree". It refuses a symbolic link as the config directory, anywhere on the `--component` path, or as a document, and any file that is not regular (checked on the open descriptor, without blocking on a FIFO). Files are read through `os.Root`. Bounds across all trees: 4096 entries, 256 documents, 16 MiB, and `kindyaml.MaxBytes` per document. Names, paths and messages from documents are cut and escaped before printing (control, bidi and zero-width characters become `\u` escapes, and `\` and `"` are written `\\` and `\"`, so a name cannot pass for another); schema errors name a path and keyword and never a value. A panic or an error of no kind exits 1, never 0.

## Example validator (`--files`)

`numi validate --files PATH` schema-checks kind documents that are not a repository: `PATH` is one YAML file or a directory, and every `.yaml` or `.yml` file below it is a document. It checks each against its generated kind schema (unknown fields are errors unless under `extensions:`) and the loader checks, runs no resolver, and exits 0 when all pass, 3 when any fails, 4 when the path is missing or holds no document, 2 for a usage error (no path, more than one, or combined with `--rev`, `--org` or `--component`). A produced kind (`ContextBundle`) is never loaded as configuration; it is checked against its own strict schema, `apiVersion`, `spec`, the required fields of the spec and of `BundleSource`, digests, enums and the `BundleSource` trust pin instead; the other constraints of the bundle (`AuthoredConstraint` trust, origin and origin digest, `CitedGuidance.ref`, `UnresolvedQuestion.question`, unique `BundleSource.ref`, at least two claims per conflict) are left to the ContextBundle producer. This is a schema and loader check only: floors and profile resolution are not checked, and a pass is not a substitute for validating a repository. A single file must be named `*.yaml` or `*.yml` (exit 2 otherwise); in a directory, the repository mode ignores such files, this mode refuses them. The path is cleaned once (`a/../b` means `b`) and the directory opened is the one that was checked and walked. A link is refused for the path itself (a trailing slash does not hide it) and for everything below it; links in parent folders of the path are followed. In a directory, a name that mentions `.yaml` or `.yml` in any letter case but does not end exactly in lowercase `.yaml` or `.yml` (`.YAML`, `.Yml`, `.yaml.bak`, `.yaml-old`, a trailing space) is invalid (exit 3); other names (`README.md~`) are ignored. Reading has the limits of the working-tree source: `os.Root`, no links, regular files only, 4096 entries, 256 documents, 16 MiB, `kindyaml.MaxBytes` per document. `just schema-check` runs it on `schemas/examples`, so CI validates every example. The engine build does not set a files source and exits 2 for `--files`.

## Init (`numi init`)

`numi init --ci github-actions|jenkins|both [path]` writes a `.numinance/` template for a Python repository, so the first repository can be onboarded by hand and checked with `numi validate`. The command is in both builds (it needs no resolver) and is the public function `cli.RunInit`. It names no CI vendor in code: vendor names appear in the template data (`sdk/go/cli/initdata/*.tmpl`) and in the generated documents only.

| Flag | Meaning |
|---|---|
| `--ci SYSTEM` | `github-actions`, `jenkins` or `both`. Required: absent is a usage error (exit 2), because a default CI would wire a system the repository may not have |
| `--org-dir DIR` | also write the org-scoped documents into this organization policy repository (see below) |
| `--package-manager NAME` | `uv`, `pip` or `poetry`. Default: detected from the target directory in this order: `uv.lock`, `poetry.lock`, `pyproject.toml` (`[tool.poetry]`, `[tool.uv]`), `requirements.txt`; with none of them `uv`. Two different managers named by the directory (for example `uv.lock` and `poetry.lock`) are a usage error until the flag decides |
| `--python-version 3.N` | default: the first line of `.python-version` when it is a plain `3.N` or `3.N.P`, else `3.12` |
| `--verifiers LIST` | comma-separated subset of `unit-tests`, `lint`, `typecheck` (pytest, ruff, mypy of the verifier provider); default all three |
| `--language python` | optional; python is the only accepted value and the default. Absent, the report says `chose: language python (default)`. Any other value is a usage error (exit 2): `--language go: only python has a toolchain template` |
| `--scm github` | optional; github is the only accepted value and the default (`chose: scm github (default)`). Any other value is exit 2: `--scm gitlab: only github has a provider` |
| `--tickets file` | optional; `file` is the only accepted value. Absent, no ticket binding (`chose: tickets none (default)`). Given, it writes no ticketing binding and no new kind field (the ticketing port has no file provider yet); it adds one line to `.numinance/README.md`: tickets come from the ticket file the run is given (`numi v1a run --ticket FILE`). Any other value is exit 2: `--tickets jira: no ticketing provider exists yet; use file` |
| `--force` | overwrite the files init writes in an existing `.numinance`; other files are never touched |

It prints each choice with its source (`flag`, `detected from uv.lock`, `default`), the files it wrote and notes. Exit codes: 0 OK, 2 usage, 3 invalid (the path is not a directory, or the config directory, `policy/` or a document is a symbolic link or not a regular file), 4 the directory is missing, 5 conflict (`.numinance` already has any file and no `--force`; also a leftover `*.numi-init.tmp`), 1 internal.

What it writes (`.numinance/policy/` is the protected location of section 12: a document there carries authority, so protect it with branch protection and CODEOWNERS):

| File | Kind | Content |
|---|---|---|
| `policy/sdlc-profile.yaml` | SDLCProfile (repo) | `playbooks`, `toolchains`, `adapters.scm` (`dev.numinance.scm-github`) and the one `adapters.ciexecutor` |
| `policy/autonomy.yaml` | AutonomyPolicy | `defaults.level: AUTONOMY_LEVEL_PR` |
| `policy/stage-registry.yaml` | StageRegistry `pr-only` | triage, plan gate, implement, a static verify stage, one CI verify stage per CI system (both map to Verifying), a human `pr-gate` at the release gate state. No stage maps to Merging |
| `policy/playbook-bugfix-bounded.yaml` | Playbook | three steps, one per identity kind, with the capability set names the deployment defines: `fix` (`implement`, MEDIUM, evidence `diff` and `pr-ref`, the verifiers of the toolchain; the branch push and the pull request), `review` (`review`) and `pr-status` (`release`, read-only status of the pull request). No step merges |
| `policy/toolchain-python.yaml` | LanguageToolchain | language, version, `package_manager` and the verifiers with provider `dev.numinance.verifier-python` and `config.check` |
| `ci-jenkins.yaml` | AdapterSelector | only for `--ci both` without `--org-dir`; outside `policy/`, so no authority |
| `README.md` | | what the tree is |

**PR only.** The ladder level is `AUTONOMY_LEVEL_PR` (the agent opens pull requests and never merges). `PR_ONLY` is a breaker state of the engine, not a ladder level. Floors are a ceiling and only the org scope and control packs may set them, so a floors block in a repository is refused by the loader and the resolver. With `--org-dir` the org tree gets an AutonomyPolicy whose floors cap every change at PR (`match: {risk: RISK_CLASS_READ}` matches every class; HIGH stays at SUGGEST); without it the engine applies its shipped floor, which is the same cap. The kernel floors (design section 12) are always on in the engine and no document can weaken them; a repository has no way to reference them, so the generated README says so instead.

**Org tree (`--org-dir`).** Only the org scope selects the stage registry (`SDLCProfile.stages`) and grants playbook, toolchain and adapter-binding names to repositories; a repository that asks for an ungranted name gets it recorded as `ignored: "outside the org-granted set"` and not applied. `--org-dir DIR` therefore writes `DIR/.numinance/policy/{sdlc-profile,autonomy,stage-registry,playbook-bugfix-bounded,toolchain-python}.yaml` (and `ci-jenkins.yaml`, an org AdapterSelector that grants the Jenkins binding, for `--ci both`) and the repository keeps its own profile only. Without it all shared documents are repo-scoped, the registry is defined but not selected, and the engine build lists the names and bindings as not granted. Both directories are checked before either is written. `--org-dir` naming the repository under any spelling (dot, relative, symlink) is refused with exit 3, decided on the opened directories. The Jenkins grant is a grant only: the repository profile binds GitHub Actions, rebinding to the placeholder is the operator's explicit edit, and `numi validate` does not check that a provider is installed. Without `--force` a document is created by hard link so a file that appeared after the check is never replaced; where the file system has no hard links the writer falls back to rename and that small window remains.

**CI support.** The adapter schema binds one `ciexecutor` per scope. `github-actions` binds `dev.numinance.ci-github-actions` (exists, replay and live builds, a live run is owed). `jenkins` binds the placeholder `dev.numinance.official/jenkins`: the ciexecutor port exists but no Jenkins provider does (documented in `ci-github-actions`, "Not done"), and `validate` does not check that a provider is installed, so a green validate never means Jenkins works. With `both` the effective binding is GitHub Actions and the Jenkins binding is kept as a grant (org tree) or as an inert document without authority (repository only). The note `init` prints names only the file it wrote in that mode. Error messages quote each value once (`directory "X" not found`); the escaping of `clean` is applied to the value, not again to the message.

**Validating the output.** The open build schema-checks every document and exits 7 ("profile resolution needs the Numinance engine build"); `numi validate --files .numinance` of the open build exits 0 after the schema check only. The engine build (`numinance/cmd/numi`, reads a committed revision) exits 0 and prints the digest and provenance: commit the files first, then `numi validate .` (or `numi validate --org ORG_DIR .`). The `unit-tests` verifier is pytest, which runs candidate code and is UNAVAILABLE in the live verifier provider until its nested sandbox exists.

Files are written with `os.Root` (no path leaves the directory), refusing links, with mode 0644 (directories 0755), each through a temporary name and a rename, and what a failed run created is removed again. Tests: `go test ./sdk/go/cli` (golden files `testdata/init_*.golden`, rewritten deliberately with `-update`).

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
