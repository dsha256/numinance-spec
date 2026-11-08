# ci-github-actions

The ciexecutor provider for GitHub Actions: a capability provider of `numinance.ports.ciexecutor.v1` over the GitHub REST API. It
holds no credential (a handle for the intent `ci.github.actions`, sent to the egress proxy) and starts only a workflow the customer
already defines on the repository's default branch.

## What a pipeline must do

GitHub starts a `workflow_dispatch` run on a branch, never at a commit, and answers with no run id. So:

1. The provider dispatches on the repository's **default branch**, which is protected, so a candidate can never change the pipeline
   definition, with these inputs: `numinance_revision` (the full commit id to build, the merge revision),
   `numinance_operation_id` (derived from the host's idempotency key) and the parameters from the profile (names starting
   `numinance_` are refused).
2. The workflow declares both inputs and names its run `numinance ${{ inputs.numinance_operation_id }} ${{ inputs.numinance_revision }}`.
   The provider finds the run by that name, before it dispatches (so a retry starts no second run) and after, and reports the revision
   from it. A run that does not follow this is not an operation of the provider (`UPSTREAM_ERROR`).
   A run is adopted only if it was dispatched on the default branch and its `path` names the workflow file on that branch
   (a run of another ref runs that ref's own copy of the file, which a candidate controls), and its name carries the
   requested revision. A full page of runs (100) that lacks the operation fails the call closed (`LIMIT_EXCEEDED`) and dispatches nothing.
3. The workflow checks out `numinance_revision` itself.

## Calls

`StartOperation` (HIGH, DEPLOY side effect; declares and emits one `ci-run-ref` receipt; dry run checks the workflow and the commit and
starts nothing), `GetOperation`, `CancelOperation` (an ended run is answered as it is), `ListArtifacts`, `GetLogReference` (the
`github.com` run page, never the log). A run's state is the CI's claim: an unknown ending is `FAILED`, never a success.

## Builds, cassettes, tests

As in `scm-github`: `ci-github-actions` (live, through the egress proxy) and `ci-github-actions-replay` (`cmd/replay`, over
`replay/cassettes/ci.json`, hand-written from the REST documentation, not recorded). `go test ./plugins/official/ci-github-actions/...
./conformance/ciexecutor/...` from `numinance-spec`; the host-level acceptance is the reference implementation's release acceptance test.

## Not done

- A live run and a recording tool (a GitHub token is needed); the host gives the provider no egress-proxy address yet.
- Other CI providers beyond `ciexecutor-jenkins` (replay only, live run owed), the second provider of the port.
- The wait for the dispatched run is five looks (`poll` apart); a run that has not appeared fails the call as unavailable and the host's retry finds it by its key.
