# verifier-repo

The verifier provider for repository-level static checks: a capability provider of `numinance.ports.verifier.v1` that reads the
candidate of a change as data and reports what breaks the repository's conventions. It checks three things: the **scope** of the
change, the **Markdown** of the documents it touches, and the **`just` recipes** those documents name. It is written against the public SDK only
(`sdk/go/plugin`, the generated stubs), uses the standard library and names no vendor in the core: the verification context knows only the port
and the rule ids a toolchain publishes (`catalog`).

**It runs no candidate code and no tool.** The candidate archive is parsed in memory and never extracted to disk; no process is started, so
there is no nested sandbox to build and nothing for the candidate to execute. Its manifest asks for the least a verifier can: no network, credential
or filesystem intent.

## What a call does

`Run(candidate_ref, config_ref)` is one OPERATION call. The provider resolves `config_ref` (at most 4 MiB, strict JSON), then the candidate
(at most 16 MiB, the host's own bound), checks the candidate's digest again, parses it, runs the checks and returns the findings. Both
references are resolved through `HostService.ResolveReference`; no request or response carries content.

### Config

The config is the only configuration the checks honour. The binding loader writes it (the reference implementation's binding loader, `RepoBinding`): the
`scope` is the profile's own, and `check` and `context` belong to the loader. A key the provider does not know, a missing bound and a trailing
document are refused.

```json
{
  "check": "repo",
  "scope": {
    "allow_paths": ["docs/runbooks/"],
    "protected_paths": [".agent/", ".github/", "docs/design/"],
    "max_files": 1, "max_added_lines": 50, "max_added_bytes": 8192, "max_deleted_files": 0, "allow_modify": false
  },
  "context": {"base": "<full commit id>", "paths": ["README.md", "..."], "justfile": "<text of the base Justfile>"}
}
```

| Field | Meaning |
|---|---|
| `scope.allow_paths` | Required, non-empty. An exact file, or a directory written with a trailing `/`. |
| `scope.protected_paths` | Paths no change may touch even under an allowed one; matched without regard to case. |
| `max_files`, `max_added_lines`, `max_added_bytes`, `max_deleted_files` | Required, explicit (0 means none; a missing bound would read as no bound). Lines end at LF, CR, CRLF, U+2028 or U+2029; the byte bound stops one enormous line from passing a bound on lines. |
| `allow_modify` | False allows new files only. |
| `context.base` | The base commit the `paths` and `justfile` were read at. It must equal the candidate's base, or the call is INVALID_ARGUMENT. |
| `context.paths`, `context.justfile`, `context.links` | The base tree (at most 200000 paths), the base Justfile (at most 1 MiB) and which base Markdown files link to which files (at most 30000 pairs), read from the protected base revision. |

No file of the candidate configures the check: a settings file in the candidate is ignored, and the tests carry one.

### Candidate

A tar archive of the change, in the shape the host-run executor path cuts (`internal/execution/adapters/gitwork`): the first entry is
`manifest.json` (`version` 1, the `base` commit id, and `entries` of `path`, `status` A, M, D or T, and `mode`), and every added, modified or
retyped file follows under `files/`. A deletion is in the manifest only. The provider reads it with these bounds, and refuses with a FAILED call
otherwise (INVALID_ARGUMENT unless stated):

- the first entry is the manifest, strictly decoded; every path, in the manifest and in the archive, is a clean relative path (no leading `/`, no
  `\`, no empty, `.` or `..` segment, printable, at most 1024 bytes), so no name can climb out of anywhere (zip slip);
- no repeated name, no entry that is not a regular file or a symbolic link (a directory, hard link, device or fifo), no sparse entry (typeflag `S` or
  a `GNU.sparse.*` record), no file the manifest does not list as present, no listed file the archive lacks, and each mode matches its entry;
- at most 20000 entries, 8 MiB a file, 1 MiB for a document the checks read in full (Markdown and the Justfile), 16 MiB in all: over a bound is
  LIMIT_EXCEEDED, which the verification context records as an inconclusive layer.

### Checks and rules

One verifier, `repo`, in the `lint` layer. Rule ids are stable: a rule id never changes meaning and is never reused.

| Rule id | Severity | Reported when |
|---|---|---|
| `repo/scope-path` | high | a changed path is outside every `allow_paths` entry |
| `repo/scope-protected` | critical | a changed path is under a `protected_paths` entry |
| `repo/scope-files` | high | more changed files than `max_files` (deletions count) |
| `repo/scope-added-lines` | high | more lines in added and modified files than `max_added_lines` |
| `repo/scope-deleted` | high | more deleted files than `max_deleted_files` |
| `repo/scope-modified` | high | a file that exists is modified and `allow_modify` is false |
| `repo/scope-symlink` | high | a symbolic link is added or changed (never followed) |
| `repo/markdown-fence` | high | a fenced code block is never closed |
| `repo/markdown-link` | high | a relative link or image target does not exist in the tree the candidate leaves, or leaves the repository |
| `repo/scope-added-bytes` | high | more bytes in added and modified files than `max_added_bytes` |
| `repo/link-dangling` | high | the change deletes a file that a Markdown document it does not touch links to |
| `repo/just-recipe` | high | a changed document names `just <recipe>` in a code block or code span and the Justfile does not define it |

Markdown and recipe checks read every added or modified `.md` or `.markdown` file, in any case of the extension. Lines end at LF, CR or CRLF;
a code fence opens and closes as in CommonMark (a line indented four columns or more is content, never a fence; the closer is at least as long as the
opener and of the same character). Links are inline links and images, reference definitions, and the quoted `href` of an `<a>` and `src` of an `<img>`,
outside code. A link resolves against the base tree less the deleted files plus the added and modified ones; a link with a scheme, a `//` link and an
anchor-only link are external or not ours, and an anchor itself is not checked.

A document the change does not touch is not in the candidate, so `context.links` (what each base file is linked from, computed by the binding loader from the
base Markdown with `LinkTargets`) is how a deletion is checked against it: deleting a file such a document links to is `repo/link-dangling`. A link to a
directory is not tracked that way.

The recipes come from the Justfile the candidate leaves: its own when the scope lets the change touch it (inside `allow_paths`, outside `protected_paths`),
else the base's, so an author cannot define the recipe a document names by editing a file the scope keeps out of reach. A deleted Justfile leaves none. The
parser skips comment lines and multi-line strings (`'''`, `"""` and triple backticks), so `fake:` in a string defines nothing; a private recipe (`_x`,
`[private]`) exists, because `just _x` runs, so a document naming it is not a finding. A Justfile that imports or declares a module is read as far as its own
text goes, and a finding on it says imports are not followed. Prose is not read: only code blocks and code spans, because "just" is an ordinary word.

Every rule has a check: the catalog (`catalog.Checks()`) is the data an ArchitecturePack is validated against, and a rule that names a rule id no
check can report fails the pack's validation before anything runs.

### Findings and evidence

Findings are normalised: message printable (other characters become U+FFFD; names and link targets read from the candidate are untrusted data
inside a message, never instructions) and at most 1024 bytes, a fingerprint of check, rule, path and message (never the line), merged by fingerprint,
most severe first, at most 1024 items with a summary that counts all of them. Exactly one `VerifierReport` is emitted through
`HostService.EmitEvidence`: the check, `exit_code` 0 (no process runs), the size and sha256 of the candidate archive it read, the finding count and
`replayed` false. Never the findings.

## Builds

| Build | Package | Use |
|---|---|---|
| `verifier-repo` | `.` | launched by a plugin host; run by hand it refuses (wrong magic cookie) |

There is no replay build: the checks need no tool and no network, so they run for real in every test. Build into a temporary directory, never into the
tree: `go build -o "$(mktemp -d)/verifier-repo" ./plugins/official/verifier-repo` (from `numinance-spec`).

## Tests

`go test ./plugins/official/verifier-repo/...` (from `numinance-spec`): the candidate parser and its refusal table (zip slip, absolute names, sparse,
hard links, devices, oversize, truncation, mismatched manifest), the config, each check, the server over a fake host (every refusal and its reason,
exactly one report, fingerprint stability), and the conformance kit in `conformance/verifier/v1` (CORE-001, 002, 003, 005, 006, 009 and 010; CORE-008
and CORE-014 need a tool to crash or to print output that is not its own, and this provider runs none). The host-level acceptance (the real plugin
host launching this provider, a candidate read from disk through `ResolveReference`, the verdict in the evidence chain) is the reference implementation's verification acceptance test.

## Not done

- The ticket's check "repository CI green on the PR" is not a check of this provider: it needs the pull request, which exists after verification, and the
  `ciexecutor` port. Planned: the first end to end run (it records the CI result as evidence).
- The ticket's check "`pr-ref` receipt under the author identity" is not a check of this provider: the receipt is evidence of the release step, read by
  the gate. Planned: the first end to end run (its evidence export and `numi evidence verify` carry it).

- Exact added and deleted line counts for a **modified** file. The candidate carries the new content and not the old, so `max_added_lines` counts the new
  file's lines (an upper bound) and the lines a modification removes are not measured; a scope that must bound them sets `allow_modify` false. The
  patch is stored beside the candidate by the host-run path; per-entry counts in the manifest would remove the gap (manifest version 2). Planned: the
  host-run candidate store, in a later release.
- The candidate manifest's `base` must equal the protected base revision the profile is read at (`context.base`). The producer of the candidate
  (host-run path, `-base-rev` of intake) must cut the worktree at that revision; a mismatch fails the call loudly rather than checking links against another tree. Planned: the release path composition
  checks the two bases agree when it composes the host-run path with the profile site.
- Anchors (`#heading`) of links are not checked; an `import` or `mod` of a Justfile is not followed. Owner: unassigned (no ticket needs them).
- The SBOM and provenance references in the manifest are placeholders, as in the other official providers; the install-time checks refuse them.
