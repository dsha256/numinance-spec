# ciexecutor conformance kit, v1

Skeleton that runs. The acceptance tests for a provider of `numinance.ports.ciexecutor.v1.CiExecutorService`
(`proto/numinance/ports/ciexecutor/v1/ciexecutor.proto`). The kit version tracks the port minor; this skeleton tracks 1.0.0. The
runnable skeleton is the Go package `cikit` in this directory (`kit.go`): it runs CORE-001 to CORE-008 against any `Target` with the
fixtures a publisher supplies (`Subject`), and `ValidateOperation` is the port's response rule as a function. `kit_test.go` runs it
against the replaying builds of `plugins/official/ci-github-actions` and `plugins/official/ciexecutor-jenkins`; the release context's acceptance
(the reference implementation's host-level acceptance test) runs the same provider through a real launch and `Invoke`.

## Core

| Id | Verb | Case | Expect |
| --- | --- | --- | --- |
| CORE-001 | StartOperation | a pipeline at a revision with parameters, twice under one key | a run with a well formed id, then the same id with `existing` true: never started twice |
| CORE-002 | StartOperation | a repository that does not exist | NOT_FOUND |
| CORE-003 | StartOperation, GetOperation | a short revision; a branch for a revision; a pipeline that climbs; a reserved or upper-case parameter; a value over 1024 bytes; more than 32 parameters; a malformed repository; a malformed operation id | INVALID_ARGUMENT for each |
| CORE-004 | StartOperation | dry run | no `operation_id`; nothing started |
| CORE-005 | GetOperation | a run seen twice | a state of the enum, the revision that was started, an end time only after the run ends, an end state at the second look |
| CORE-006 | CancelOperation | a running run, twice | CANCELLED both times |
| CORE-007 | ListArtifacts | an ended run | names unique and at most 256 bytes, digests `sha256:<hex>` |
| CORE-008 | GetLogReference | an ended run | a bounded https URL |

Cases the skeleton does not run and the kit must hold: a run that does not carry the operation (UPSTREAM_ERROR), an unknown ending is FAILED,
an answer over the bound, one `ci-run-ref` evidence id per StartOperation that succeeds (PARTIAL without it), and a start that finds its
run only after a delay.

## Host checks

Enforced by the policy decision point, the plugin host and the release context, not by a provider; this skeleton does not run them. HOST-001 is run against the real host by the reference implementation's host-level acceptance test (with a scripted decision that allows a capability only to the identity whose grant holds it); the others are the cases a host kit must hold.

| Id | Case | Expect |
| --- | --- | --- |
| HOST-001 | StartOperation for an actor kind other than release-pipeline; whose principal is the author's; whose `run_id` is any author run's | the host refuses before `Invoke` |
| HOST-002 | a request whose `repository` is outside the grant's resource scope, or that names none | the host denies before `Invoke` |
| HOST-003 | a manifest declaring `IDEMPOTENCY_NONE` for StartOperation, or a risk floor below HIGH | the host refuses it at handshake |

## Extended

None in 1.0.0.
