# verifier-python

The verifier provider for the Python toolchain: a capability provider of `numinance.ports.verifier.v1` that runs ruff, mypy,
pytest, Semgrep or import-linter on a candidate and reports the findings. It is written against the public SDK only
(`sdk/go/plugin`, the generated stubs) and names no vendor in the core: the verification context knows only the port and the
rule ids a toolchain publishes (`catalog`).

## What a call does

`Run(candidate_ref, config_ref)` is one OPERATION call. The provider resolves `config_ref` through `HostService.ResolveReference`
(at most 64 KiB, strict JSON), reads `check` from it, runs that check on the candidate and returns the findings.

- **Config** is JSON `{"check": "ruff", "settings": "<the tool's own settings file text>", "timeout_seconds": 60}`. The settings are
  written to a private directory and handed to the tool; no settings file of the candidate is read. An unknown check is UNAVAILABLE.
- **Candidate** is a tar archive of the tree (regular files and directories only; a symlink, hard link, device, absolute or
  parent-relative name or a repeated name is refused, and so is any sparse entry, a `GNU.sparse.*` PAX record or typeflag `S`; 64 MiB
  declared in all, 20000 entries). The provider hashes the bytes it is given against the reference's digest before it extracts them. It is extracted into a private temporary directory.
- **Candidate code is not run unnested.** Two checks execute it: pytest runs the tests, and import-linter imports the root package of
  the candidate to find its modules, which runs its `__init__` files in the verifier's own process. The port allows that only in a
  nested sandbox, which this provider does not build (not yet built), so both are UNAVAILABLE here and only the replaying build
  serves them. The static checks (ruff, mypy, Semgrep) run from a private working directory outside the candidate with the tree
  passed by path, `PYTHONSAFEPATH=1` and no `PYTHONPATH`, in their own process group that is killed whole at the deadline.
- **Tools** run with no environment but `PATH`, a private `HOME` and those Python variables, a timeout (default 60 s, at most 10 min) and
  an output limit (8 MiB; over the limit ends the call FAILED and is never truncated). Outputs are parsed strictly: a tool that exits with an
  unexpected status, or prints something that is not its own format, ends the call FAILED with no findings. The pytest report is
  JUnit XML read without entity or include expansion; a document type declaration or an unknown processing instruction fails it.
- **Nothing is hidden from a check.** ruff runs with `--ignore-noqa --no-respect-gitignore --no-force-exclude` and its default and extra
  excludes emptied; Semgrep with `--disable-nosem --no-git-ignore --x-ignore-semgrepignore-files` and `--max-target-bytes` at the
  extraction limit (the `--x-` flag is Semgrep 1.183's and may change: a version bump re-runs the tests). Semgrep's `paths.skipped` must be
  empty and `paths.scanned` not, a candidate with no `.py` or `.pyi` file is a failed check, and a tool that exits non-zero with no
  findings crashed. Settings come only from the config.
- **Directives** the tool cannot ignore are reported as findings with a `numinance.directive/...` rule id: mypy `# type: ignore` and
  `# mypy:` inline config (in `.py` and `.pyi`), pytest skip and xfail marks, `hookimpl`, `pytest_plugins` and every pytest hook but an
  allow-list (`pytest_addoption`, `pytest_generate_tests`), and the ignore files a scanning tool would honour (`.git`, `.semgrepignore`,
  `.ignore`, `.ruffignore`, MEDIUM; `.gitignore`, LOW). A pack cannot change their severity.
- **Findings** are normalised: message printable (other characters replaced by U+FFFD) and at most 1024 bytes at a character
  boundary, path relative to the candidate root, a fingerprint of check, rule, path and message (never the line), merged by
  fingerprint, most severe first, at most 1024 items with a summary that counts all of them. Rule ids are namespaced by check
  (`ruff/F401`, `mypy/return-value`, `semgrep/<check_id>`, `import-linter/<contract>`, `pytest/test-failed`).
- **Evidence**: exactly one `VerifierReport` per call through `HostService.EmitEvidence`: the check, exit code, output size,
  the sha256 of the raw output, the finding count and whether the output was replayed. Never the findings.

## Builds

| Build | Package | Use |
|---|---|---|
| `verifier-python` | `.` | runs the installed tools; launched by a plugin host |
| `verifier-python-replay` | `cmd/replay` | the same provider over recorded tool output (`replay/recordings`); no tool and no network |

Build into a temporary directory, never into the tree: `go build -o "$(mktemp -d)/verifier-python-replay" ./plugins/official/verifier-python/cmd/replay`.
Run by hand either refuses (wrong magic cookie): a host starts it.

## Recordings

`testdata/record.sh` runs each tool on each candidate of `testdata/candidates` (`seeded` holds a type error, two lint violations, a
failing test, an `eval` and a layering violation; `clean` holds none; `suppressed` hides a type error behind `# type: ignore`) and
writes the raw output under `replay/recordings/<sha256 of "kit-candidate:<name>">/<check>.{json,out}`. The work directory is fixed,
so paths in a recording are `/candidate` or relative; the home directory and host name are redacted. Recorded with ruff 0.8.4,
mypy 2.4, Semgrep 1.183, import-linter and pytest as `uvx` resolved them on 2026-11-01. `crashing` and `malformed` are synthetic
(written by hand: no real tool is made to crash or to print hostile output). Re-record with `sh testdata/record.sh` (needs network
once); a recording changes only in timings, timestamps and memory addresses.

## Tests

`go test ./plugins/official/verifier-python/...` (from `numinance-spec`): the parsers, the normaliser and its bounds, the tar
extraction, the directive scan, the server over the replay Runner, and the conformance kit in `conformance/verifier/v1`.
`NUMINANCE_VERIFIER_TOOLS=1 go test ./plugins/official/verifier-python/provider -run RealRuff` runs the real ruff on the seeded
candidate; it is skipped otherwise. The host-level acceptance (the real plugin host launching the replaying build) is
the reference implementation's verification acceptance test.

## Not done

- Nested sandbox for candidate code (pytest and import-linter), not yet built. Host checks HOST-004 and HOST-010 of the kit (a real
  provider that reads its environment, connects to metadata addresses and signals the verifier) need that sandbox.
- Tools other than the five; Python tools pinned by digest in an image (the extensions context).
- The SBOM and provenance references in the manifest are placeholders, as in the sample provider; the install-time checks refuse
  them.
