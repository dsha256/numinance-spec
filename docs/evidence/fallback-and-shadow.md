# Fallback and shadow

Status: normative for `evidence/v1` from spec 1.1.0 (the unreleased minor that also carries `postgres_transport` and route performance). The change is additive: an export that holds none of these records verifies as before, and a verifier of 1.0.0 refuses the new file as `unknown_file` and the new records are plain records to it. Read [records](records-and-chain.md), [bundle layout](bundle-layout.md) section 6 and [attestation and manifest](attestation-and-manifest.md) section 3 first.

The names in this page are the `evidence/v1` names of the spec: the file `provenance/executor-manifests.json`, the inputs `execution/considered`, `execution/fallback-of` and `execution/fallback-cause`, the action `ShadowCompared.v1` and its refs, and the derivation rule of section 3. Each is new in 1.1.0 (`since` 1.1.0). The verifiers and the conformance vectors land with the producers, before any export holds these records.

A Change can hold records of more than one executor: a fallback run that takes over from a failed one, and a shadow that runs beside the primary without authority. This page fixes the record that opens a fallback run, how the attestation names the executor after one, the record of a shadow, and what each does to the export.

## 1. Terms

- **Route record.** A record of action `execution.stream` or `execution.route` that carries the input `execution/considered`: the record of the route a run was started under. Its `actor_run_id` is the run it routes.
- **Fallback run.** A run started after an earlier run of the same Change ended failed or could not start, under a new run id, a new grant and a new sandbox at the same start tree. A fallback never restarts the failed run and never uses its worktree or its snapshots. Restarting a run from a checkpoint (the environments section of the technical design, on checkpoints) is unchanged and is not a fallback.
- **Shadow.** An executor that ran the same work unit without authority. It has no grant that lets it write, and it is never an author.

## 2. The fallback route record

A fallback run is opened by one record: action `execution.route`, outcome `succeeded`, `actor_kind` `runner`, `actor_run_id` the new run, `policy_decision_id` the decision of the `IssueGrant` that issued the new run's grant, `grant_id` that grant, and no `executor_id` (so it is no author-agent record and states no executor). Its inputs:

| Input ref | sha256 |
|---|---|
| `execution/run-kind` | the sha256 of the run kind text |
| `execution/considered` | the digest of the candidates still considered, in order |
| `execution/fallback-of` | the `record_hash` of the route record of the route being left |
| `execution/fallback-cause` | the sha256 of the cause text, `executor_failed` or `provider_unavailable` |

The order of the candidates is the **first recorded ordering** minus the candidates already used. The first recorded ordering is the `execution/considered` of the route record found by following `execution/fallback-of` from record to record back to a route record that is not itself a fallback record (the root of the walk). Every `execution/fallback-of` names a record of a lower sequence in the same Change chain, so the walk ends.

A record is a **valid fallback record** when all of these hold:

1. its action is `execution.route`, its outcome is `succeeded`, its `actor_kind` is `runner`, it states no `executor_id`, its `policy_decision_id` and `grant_id` are not zero, and its `actor_run_id` is not zero;
2. it carries `execution/considered`;
3. it has exactly one `execution/fallback-of` input, and that is the `record_hash` of an earlier route record of the Change chain (lower sequence, carries `execution/considered`) whose `actor_run_id` is not zero and is another run;
4. it has exactly one `execution/fallback-cause` input, and that is the sha256 of one of the two causes. A cause digest outside that closed set makes the record no fallback record; a verifier never reads provider text;
5. no record of the Change chain with a lower sequence has the `actor_run_id` the record opens: a fallback opens a fresh run, so A to B and then B to A is a gap.

A record that does not have this shape is not a fallback record and is not an error (the denied `execution.route` record of a refused route is the other use of the action).

A fallback run is not started when the Change chain holds a succeeded `execution.diff` record or a publish record, or when the chain cannot be read; the refusal is a deny, not an error. A fallback never restarts the failed run.

## 3. The attestation after a fallback

Section 3 of [attestation and manifest](attestation-and-manifest.md) takes the executor, model and sandbox image from the author-agent records, and a disagreement makes the attestation a mismatch. One case resolves it. If the range holds at least one valid fallback record and the author-agent records that state a well-formed `executor_id` disagree, the values are those of the **winning run**:

1. **The final candidate.** Take the verdict records of the range ([derivations](derivations.md) section 5) and apply the chain rule of verdicts: per candidate, a verdict is accepted only when `verdict/overrules` is the `record_hash` of the previous verdict of that candidate (zero for the first); a candidate with a verdict that breaks the rule is forked, and none of its verdicts counts. Of the accepted verdicts the newest by sequence must be a `pass`; its candidate is the subject. This is the "latest PASS" rule of ADR 0049 decision 5.
2. **The receipt.** The run of the last `execution.diff` record of the range with outcome `succeeded` and an `execution/candidate` output equal to the subject is the finalizing run. No `execution.diff` record with outcome `succeeded` for another candidate may have a higher sequence than that receipt (the subject would be superseded).
3. **The winning run** is the run the last valid fallback record opens (its `actor_run_id`), and it must be the finalizing run of step 2.
4. The records of the winning run that state a well-formed `executor_id` give one `executor_id`, at most one `model_id` and at most one `sandbox_image_digest`.
5. Every other run that has an author-agent record stating a well-formed `executor_id` must have such a record with outcome `failed`, must have no `execution.diff` record with outcome `succeeded`, and must be **left** by a valid fallback record: one with a higher sequence than the run's first failed record whose `execution/fallback-of` is the `record_hash` of a route record whose `actor_run_id` is that run.

If any step does not hold, the disagreement stands: the attestation is a mismatch and the Change records a gap. A chain that does not disagree is not affected. Two failed runs and one fallback record leave the first failed run unleft, so they are a gap.

## 4. The shadow record

A shadow leaves one record on the chain: action `ShadowCompared.v1` (a lifecycle-event-shaped record), `actor_kind` `runner`, outcome `succeeded`, `policy_decision_id` the allow decision of the policy check that permitted the shadow, and `grant_id` the shadow's own grant, which has no capability: the shadow is still without authority. When the host holds the shadow's manifest it states the shadow's `executor_id` (the manifest form of [bundle layout](bundle-layout.md) section 6), its `model_id`, and its own `sandbox_profile` and `sandbox_image_digest`; otherwise it states no `executor_id` and makes no manifest claim. Its inputs are digests only: `execution/primary-stream`, `execution/primary-diff` (when the primary left a diff), `execution/shadow-stream` and `execution/shadow-fileset`, the digest of the sorted paths the host found changed in the shadow's own sandbox against the start tree it was created from (measured by the host, not claimed by the shadow's stream; never a patch). Its one output is `execution/shadow-comparison`, the digest of a canonical comparison document (event-type counts, file-set digest, the primary's verdict, cost and latency). No `execution.diff`, no `execution/candidate` output and no `author-agent` record is ever written for a shadow, so a shadow can neither produce a candidate nor change the attestation.

The record is written when the shadow ends, after the primary run ended. The comparison states the verdict the verification context had issued for the primary at that time, or `none` when it had issued none; a verdict issued later is not read back into the record. `execution/shadow-comparison` never satisfies an evidence requirement: no policy, floor or gate counts it as a verifier result, a review or a candidate.

The record is read like any other: its `executor_id` takes part in the manifest rules of [bundle layout](bundle-layout.md) section 6, so a Change with a shadow on another manifest carries `provenance/executor-manifests.json`. A verifier does not read the comparison document; it is the host's statement, bound by the digest in the signed record.

## 5. What it proves

The rules prove that the export carries the manifest of every executor the records name, that the attestation names the executor whose candidate the latest passed verdict names, and that every other executor-stating author run is recorded failed and left by a fallback record that leads back to its own route. They do not prove that the cause is true, that the order of the candidates was the router's, or that a shadow held no authority; the last follows from the absence of any author-agent or `execution.diff` record of it. A verifier older than 1.1.0 does not read these records ([verification](verification.md) section 9, item 11).
