# scm conformance kit, v1

Skeleton that runs. The acceptance tests for a provider of `numinance.ports.scm.v1.ScmService` (`proto/numinance/ports/scm/v1/scm.proto`).
The kit version tracks the port minor; this skeleton tracks 1.0.0. The runnable skeleton is the Go package `scmkit` in this
directory (`kit.go`): it runs CORE-001 to CORE-010 against any `Target` with the fixtures a publisher supplies (`Subject`), and
`ValidateRepository`, `ValidatePullRequest` and `ValidateStatus` are the port's response rules as functions a host or a test can
apply to any provider. `kit_test.go` runs it against the replaying build of `plugins/official/scm-github`; the release context's
acceptance (the reference implementation's host-level acceptance test) runs the same provider through a real launch and `Invoke`. Modes are those of
ADR 0015: replay, live publisher-run, live registry-run.

## Core

| Id | Verb | Case | Expect |
| --- | --- | --- | --- |
| CORE-001 | GetRepository | an existing repository | a default branch and the full commit id it points to |
| CORE-002 | GetRepository | a repository that does not exist | NOT_FOUND |
| CORE-003 | any | a malformed repository name (empty, one segment, three, `..`, `.git`, a space, over 100 bytes) | INVALID_ARGUMENT; nothing is sent |
| CORE-004 | GetBranchProtection | a protected and an unprotected branch | `protected` true with at most 64 required checks; false with none |
| CORE-005 | OpenPullRequest | the candidate branch, twice | a pull request open at the requested head and base, then the same number with `existing` true: never opened twice |
| CORE-006 | OpenPullRequest | a branch that does not point at `head_revision` | FAILED with reason CONFLICT |
| CORE-007 | OpenPullRequest | dry run | `pull_request` unset; nothing opened |
| CORE-008 | GetPullRequestStatus | an open pull request with checks and reviews | every field within its bounds (`ValidateStatus`) |
| CORE-009 | MergePullRequest | a moved head; a closed pull request; the verified head, twice | CONFLICT; CONFLICT; a merge revision, then the same revision with `already_merged` |
| CORE-010 | CommentOnPullRequest | one key, twice | one comment id |

Cases the skeleton does not run and the kit must hold: reads of trees, files, branches and search (bounds, truncation, a hit of another
repository is never reported), CreateBranch (replay at the same commit, ALREADY_EXISTS at another), UpdatePullRequest (a reopen is
INVALID_ARGUMENT), MergePullRequest dry run, an answer over 4 MiB (LIMIT_EXCEEDED), an answer out of bounds (UPSTREAM_ERROR), and one `pr-ref`
evidence id per OpenPullRequest that succeeds (PARTIAL without it).

## Host checks

These are enforced by the policy decision point, the plugin host and the release context, not by a provider, and this skeleton does not run them. HOST-001 and HOST-002 are run against the real host by the reference implementation's host-level acceptance test (with a scripted decision that allows a capability only to the identity whose grant holds it; the floors are the policy context's own tests); the others are listed as the cases a host kit must hold.

| Id | Case | Expect |
| --- | --- | --- |
| HOST-001 | MergePullRequest for an actor kind other than release-pipeline; whose principal is the author's; whose `run_id` is any author run's | the host refuses before `Invoke` |
| HOST-002 | OpenPullRequest for an actor that is not the author | the host refuses before `Invoke` |
| HOST-003 | a request whose `repository` is outside the grant's resource scope, or that names none | the host denies before `Invoke` |
| HOST-004 | a manifest declaring `IDEMPOTENCY_NONE` for a mutating verb, or a risk floor below the port's for MergePullRequest | the host refuses it at handshake |
| HOST-005 | a response with a field unknown to the host's descriptor copy | the host rejects it and records the call as failed |

## Extended

None in 1.0.0.
