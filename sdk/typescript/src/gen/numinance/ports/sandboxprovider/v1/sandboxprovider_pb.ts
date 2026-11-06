// The sandboxprovider port (ADR 0006 as amended by ADR 0033): creates, runs, snapshots and destroys the one sandbox of one run of one work unit on the runner's host. One sandbox per run: a work unit's later run gets a new sandbox, restored from a snapshot.
// Callers: only the sandbox context in the runner calls this port, under the runner identity; the host refuses every other actor kind. The port is never a model tool, and no tool_groups or scopes are declared. The provider runs on the runner's host with authority over mounts, so only a provider bound to the runtime deployment serves it; the host refuses one installed from the registry. The worktree, artifact and output directories are host directories; the host reads them itself.
// Authority: nothing a provider reports is authority. The host builds every SandboxSpec from the run's grant, the policy decision and the plan, and refuses to dispatch a request whose change_id, work_unit_id or run_id differs from the ActionContext, whose deadline is later than the ActionContext deadline, or whose profile is below the floor minimum (compared through the floors' explicit table, never by number) or the work unit's ceiling. ADR 0033's ceiling bounds the ladder level and risk class of the work unit, checked before Create; the port's verbs act inside an admitted work unit, and the ceiling never lowers the risk class decided for an action (an Exec under PROCESS stays PRIVILEGED). The provider creates exactly the requested profile or refuses; it never downgrades. Every answer is a provider claim: the host recomputes the digests of the files it reads, and after a restore the tree digests of both directories against the ones Snapshot returned, and treats a mismatch as a provider fault; that a capture matched the run's state rests on the deployment-bound provider.
// Isolation, under every profile: the image is the one the runner holds under image_digest and is never pulled; the environment holds only the Exec env and TRACEPARENT, nothing of the host environment; the run's processes run as an unprivileged OS user dedicated to the run, which can read no runner, broker, egress proxy, provider or other run state, no host credential (cloud, SSH agent, git, instance role) and no canary. The host's worktree holds no credential: its git configuration has no credential helper, token-bearing URL or extra header. Under CONTAINER and GVISOR also: read-only root filesystem, writable tmpfs, all capabilities dropped, seccomp, no-new-privileges, a user namespace, the limits of SandboxLimits, and no host path but the worktree and artifact directories; GVISOR adds the user-space kernel. Their write model: at Create the provider hands every entry at or under allowed_paths, and the artifact tree, to the run's uid as in the PROCESS Create handover, so whatever the run creates or changes is owned on disk by the run's uid, never through a mapping to the host's user; every other entry stays host-owned and read-only for the run. An allowed path missing at Create is writable only under another allowed path, so the host refuses a request with an allowed path that neither exists nor lies under another (allow the directory to create a file in it). Their network namespace reaches only egress_proxy, which is unique to the run (the proxy binds it to this sandbox); every other address, DNS included, is unreachable, and with no egress_proxy there is no network. PROCESS is a direct child process: freeze, kill and Destroy act on every process of the run's OS user, never only on a process tree; the run gets a private temporary directory as TMPDIR; the worktree, artifact and TMPDIR roots are 0700 and owned by the run's user, under a work root no other run's user can traverse, and at Create the provider hands the host-created trees to the run's user without following links, with modes normalized as at restore and the Create rules for ACLs, extended attributes, labels and file flags (Destroy comment); Destroy removes every file the user owns outside the worktree and artifact directories, which the host keeps, and the user is never reused. Under every profile, neither the run's user nor its numeric uid is reused while any directory it owns is kept, which after a complete Destroy handover is none, so this binds only while a Destroy is unfinished. Host paths readable by the run's user and the host network are reachable, egress_proxy must be empty, the provider enforces the deadline and the limits the OS supports, and a provider that cannot run each run as its own OS user refuses PROCESS. The host offers PROCESS only on a host where, at each Create, an active probe of the metadata addresses, made as the run's user, fails and no cloud credential source is present, and the decision for an Exec under PROCESS counts its egress as unclassified, so PRIVILEGED. Under every profile the run's user can connect to no runner, broker, egress proxy or container runtime socket other than its own proxy, and those endpoints authenticate their caller.
// Paths: worktree paths follow the path rules of the authorexecutor port (file comment, Paths); overlap checks and masking compare them under NFC and Unicode simple case folding. Under CONTAINER and GVISOR only allowed_paths are writable, enforced on the resolved path after following symlinks, and hardlink creation is refused; every other worktree path, .numinance/context/ included (the host neither allows nor masks it), is read-only. Each masked path is absent: its content is not readable and nothing can be created at or under it. Under GVISOR the provider also refuses to create, at any depth, an entry whose name case-folds to .git, .gitmodules, .gitattributes or .lfsconfig. Under PROCESS the sandbox enforces no worktree path rule: the host leaves every masked path out of the worktree it creates, so it is absent on disk, and its candidate check covers the rest. A masked path absent on disk (PROCESS, or after a restore) is never a deletion: the candidate takes it from the run's base (authorexecutor port, Paths). Whatever a profile does not enforce in the sandbox, and floor patterns under every profile, the host's check of the candidate tree enforces (authorexecutor port, Paths). Host paths are absolute, '/'-separated and printable, with no '.' or '..' segment, resolved with no symlink component under the provider's configured work root (FAILED_PRECONDITION); one equal to, above or under a path of a live sandbox, or one that held a directory of a destroyed run, is refused (FAILED_PRECONDITION): every run gets new, never-used worktree, artifact and output directories. The host creates the trees with no non-directory of more than one link (a clone copies objects, never hardlinks them) and every entry owned by its own user; under every profile the provider refuses Create (FAILED_PRECONDITION) of a tree holding either. The last segment of each host path is the run_id, which keys one sandbox, so reuse is detectable without state; the provider's refusal of a destroyed run's path is best-effort within its key retention, and the host's check (HOST-001) is the durable one. A grammar breach, a last segment other than run_id, or an overlap within one spec is INVALID_ARGUMENT.
// Output: the provider writes each exec's standard output and error to exec/<exec_seq>/stdout and exec/<exec_seq>/stderr under output_path, a host directory never mounted in the sandbox and, with its parents, neither readable nor writable by the run's user under every profile, creating each with exclusive create and resolving every component with no symlink, and kills an exec whose output of the run exceeds disk_bytes (LIMIT_EXCEEDED). The host reads them, and computes every directory digest, with no symlink following and through a size-limited reader. Output is untrusted data: the host scans it for issued handles and canaries, and labels and delimits it before any prompt use.
// Payloads: requests and responses carry ids, paths, digests and counts, never file contents or output (a platform invariant: payloads carry references, never content).
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; ALREADY_EXISTS for a Create or Exec whose key is taken by a different request; FAILED_PRECONDITION for a profile the host does not offer or MICROVM, an image or snapshot the provider does not hold or that fails its digest, a host path whose resolution, work root, live overlap or earlier use by a destroyed run the Paths rule refuses, a host tree holding a multi-link file or another user's entry at Create, a restore into non-empty directories or from a snapshot holding an entry at or under a masked path of the new spec, a Create of a destroyed run, and an Exec or Snapshot of a run that is not running; NOT_FOUND for a Wait of an unknown exec; RESOURCE_EXHAUSTED when the host has no capacity, a run has 16 execs running or 64 snapshots, or a snapshot exceeds disk_bytes or the per-host byte cap; UNAVAILABLE for a runtime failure the caller may retry, which creates nothing. A provider never reports an effect it did not make.
// Evidence: the host records the requested profile, image digest and limits, the provider's claims and its own checks. No RPC declares evidence yet; its record types are fixed with the sandbox context, and no spec tag ships while they are missing.
// Bounds: every string is at most 1024 bytes and every repeated field at most 256 entries unless stated; over-limit input is rejected, never truncated; duplicate entries in any repeated field are rejected; enums are closed, and zero and unknown numbers are rejected; a duration is never negative. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Every element is since 1.0.0.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/sandboxprovider/v1/sandboxprovider.proto (package numinance.ports.sandboxprovider.v1, syntax proto3)
/* eslint-disable */

import type { GenEnum, GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { enumDesc, fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import type { Duration, Timestamp } from "@bufbuild/protobuf/wkt";
import { file_google_protobuf_duration, file_google_protobuf_timestamp } from "@bufbuild/protobuf/wkt";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { SandboxProfile } from "../../../evidence/v1/record_pb";
import { file_numinance_evidence_v1_record } from "../../../evidence/v1/record_pb";
import type { Digest } from "../../../types/v1/types_pb";
import { file_numinance_types_v1_types } from "../../../types/v1/types_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/sandboxprovider/v1/sandboxprovider.proto.
 */
export const file_numinance_ports_sandboxprovider_v1_sandboxprovider: GenFile = /*@__PURE__*/
  fileDesc("CjhudW1pbmFuY2UvcG9ydHMvc2FuZGJveHByb3ZpZGVyL3YxL3NhbmRib3hwcm92aWRlci5wcm90bxIibnVtaW5hbmNlLnBvcnRzLnNhbmRib3hwcm92aWRlci52MSJbCg1DcmVhdGVSZXF1ZXN0EkoKBHNwZWMYASABKAsyLy5udW1pbmFuY2UucG9ydHMuc2FuZGJveHByb3ZpZGVyLnYxLlNhbmRib3hTcGVjQguStRgHCgUxLjAuMCJ7Cg5DcmVhdGVSZXNwb25zZRJDCgdwcm9maWxlGAEgASgOMiUubnVtaW5hbmNlLmV2aWRlbmNlLnYxLlNhbmRib3hQcm9maWxlQguStRgHCgUxLjAuMBIkCg9ydW50aW1lX3ZlcnNpb24YAiABKAlCC5K1GAcKBTEuMC4wIrIFCgtTYW5kYm94U3BlYxIeCgljaGFuZ2VfaWQYASABKAlCC5K1GAcKBTEuMC4wEiEKDHdvcmtfdW5pdF9pZBgCIAEoCUILkrUYBwoFMS4wLjASGwoGcnVuX2lkGAMgASgJQguStRgHCgUxLjAuMBJDCgdwcm9maWxlGAQgASgOMiUubnVtaW5hbmNlLmV2aWRlbmNlLnYxLlNhbmRib3hQcm9maWxlQguStRgHCgUxLjAuMBI9CgxpbWFnZV9kaWdlc3QYBSABKAsyGi5udW1pbmFuY2UudHlwZXMudjEuRGlnZXN0QguStRgHCgUxLjAuMBIiCg13b3JrdHJlZV9wYXRoGAYgASgJQguStRgHCgUxLjAuMBIjCg5hcnRpZmFjdHNfcGF0aBgHIAEoCUILkrUYBwoFMS4wLjASIgoNYWxsb3dlZF9wYXRocxgIIAMoCUILkrUYBwoFMS4wLjASIQoMbWFza2VkX3BhdGhzGAkgAygJQguStRgHCgUxLjAuMBIhCgxlZ3Jlc3NfcHJveHkYCiABKAlCC5K1GAcKBTEuMC4wEk4KBmxpbWl0cxgLIAEoCzIxLm51bWluYW5jZS5wb3J0cy5zYW5kYm94cHJvdmlkZXIudjEuU2FuZGJveExpbWl0c0ILkrUYBwoFMS4wLjASOQoIZGVhZGxpbmUYDCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQguStRgHCgUxLjAuMBIgCgt0cmFjZXBhcmVudBgNIAEoCUILkrUYBwoFMS4wLjASPQoMcmVzdG9yZV9mcm9tGA4gASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjASIAoLb3V0cHV0X3BhdGgYDyABKAlCC5K1GAcKBTEuMC4wIpMBCg1TYW5kYm94TGltaXRzEiMKDmNwdV9taWxsaWNvcmVzGAEgASgNQguStRgHCgUxLjAuMBIhCgxtZW1vcnlfYnl0ZXMYAiABKARCC5K1GAcKBTEuMC4wEhkKBHBpZHMYAyABKA1CC5K1GAcKBTEuMC4wEh8KCmRpc2tfYnl0ZXMYBCABKARCC5K1GAcKBTEuMC4wIoUCCgtFeGVjUmVxdWVzdBIbCgZydW5faWQYASABKAlCC5K1GAcKBTEuMC4wEh0KCGV4ZWNfc2VxGAIgASgNQguStRgHCgUxLjAuMBIZCgRhcmd2GAMgAygJQguStRgHCgUxLjAuMBIgCgt3b3JraW5nX2RpchgEIAEoCUILkrUYBwoFMS4wLjASRAoDZW52GAUgAygLMioubnVtaW5hbmNlLnBvcnRzLnNhbmRib3hwcm92aWRlci52MS5FbnZWYXJCC5K1GAcKBTEuMC4wEjcKB3RpbWVvdXQYBiABKAsyGS5nb29nbGUucHJvdG9idWYuRHVyYXRpb25CC5K1GAcKBTEuMC4wIj8KBkVudlZhchIZCgRuYW1lGAEgASgJQguStRgHCgUxLjAuMBIaCgV2YWx1ZRgCIAEoCUILkrUYBwoFMS4wLjAiDgoMRXhlY1Jlc3BvbnNlIocBCgtXYWl0UmVxdWVzdBIbCgZydW5faWQYASABKAlCC5K1GAcKBTEuMC4wEh0KCGV4ZWNfc2VxGAIgASgNQguStRgHCgUxLjAuMBI8Cgx3YWl0X3RpbWVvdXQYAyABKAsyGS5nb29nbGUucHJvdG9idWYuRHVyYXRpb25CC5K1GAcKBTEuMC4wIvkBCgxXYWl0UmVzcG9uc2USSQoFc3RhdGUYASABKA4yLS5udW1pbmFuY2UucG9ydHMuc2FuZGJveHByb3ZpZGVyLnYxLkV4ZWNTdGF0ZUILkrUYBwoFMS4wLjASHgoJZXhpdF9jb2RlGAIgASgFQguStRgHCgUxLjAuMBI+Cg1zdGRvdXRfc2hhMjU2GAMgASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjASPgoNc3RkZXJyX3NoYTI1NhgEIAEoCzIaLm51bWluYW5jZS50eXBlcy52MS5EaWdlc3RCC5K1GAcKBTEuMC4wIi4KD1NuYXBzaG90UmVxdWVzdBIbCgZydW5faWQYASABKAlCC5K1GAcKBTEuMC4wItIBChBTbmFwc2hvdFJlc3BvbnNlEjkKCHNuYXBzaG90GAEgASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjASQAoPd29ya3RyZWVfZGlnZXN0GAIgASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjASQQoQYXJ0aWZhY3RzX2RpZ2VzdBgDIAEoCzIaLm51bWluYW5jZS50eXBlcy52MS5EaWdlc3RCC5K1GAcKBTEuMC4wIlUKDkRlc3Ryb3lSZXF1ZXN0EhsKBnJ1bl9pZBgBIAEoCUILkrUYBwoFMS4wLjASJgoRcmVsZWFzZV9zbmFwc2hvdHMYAiABKAhCC5K1GAcKBTEuMC4wIhEKD0Rlc3Ryb3lSZXNwb25zZSrDAQoJRXhlY1N0YXRlEhoKFkVYRUNfU1RBVEVfVU5TUEVDSUZJRUQQABIWChJFWEVDX1NUQVRFX1JVTk5JTkcQARIVChFFWEVDX1NUQVRFX0VYSVRFRBACEhgKFEVYRUNfU1RBVEVfVElNRURfT1VUEAMSHQoZRVhFQ19TVEFURV9MSU1JVF9FWENFRURFRBAEEhUKEUVYRUNfU1RBVEVfS0lMTEVEEAUSGwoXRVhFQ19TVEFURV9TVEFSVF9GQUlMRUQQBjL3BwoWU2FuZGJveFByb3ZpZGVyU2VydmljZRLuAQoGQ3JlYXRlEjEubnVtaW5hbmNlLnBvcnRzLnNhbmRib3hwcm92aWRlci52MS5DcmVhdGVSZXF1ZXN0GjIubnVtaW5hbmNlLnBvcnRzLnNhbmRib3hwcm92aWRlci52MS5DcmVhdGVSZXNwb25zZSJ9irUYeQgCEAEYASABKh5TYW5kYm94UHJvdmlkZXJTZXJ2aWNlLkRlc3Ryb3lCBTEuMC4wSkhDcmVhdGVzIHRoZSBpc29sYXRlZCBzYW5kYm94IG9mIG9uZSBydW4gZnJvbSBhIGhvc3QtYnVpbHQgc3BlY2lmaWNhdGlvbi4SqAEKBEV4ZWMSLy5udW1pbmFuY2UucG9ydHMuc2FuZGJveHByb3ZpZGVyLnYxLkV4ZWNSZXF1ZXN0GjAubnVtaW5hbmNlLnBvcnRzLnNhbmRib3hwcm92aWRlci52MS5FeGVjUmVzcG9uc2UiPYq1GDkIAxACGAFCBTEuMC4wSipTdGFydHMgb25lIGNvbW1hbmQgaW5zaWRlIGEgcnVuJ3Mgc2FuZGJveC4SsgEKBFdhaXQSLy5udW1pbmFuY2UucG9ydHMuc2FuZGJveHByb3ZpZGVyLnYxLldhaXRSZXF1ZXN0GjAubnVtaW5hbmNlLnBvcnRzLnNhbmRib3hwcm92aWRlci52MS5XYWl0UmVzcG9uc2UiR4q1GEMIARABGAFCBTEuMC4wSjRSZXR1cm5zIHRoZSBzdGF0ZSBvZiBvbmUgY29tbWFuZCBpbiBhIHJ1bidzIHNhbmRib3guEs0BCghTbmFwc2hvdBIzLm51bWluYW5jZS5wb3J0cy5zYW5kYm94cHJvdmlkZXIudjEuU25hcHNob3RSZXF1ZXN0GjQubnVtaW5hbmNlLnBvcnRzLnNhbmRib3hwcm92aWRlci52MS5TbmFwc2hvdFJlc3BvbnNlIlaKtRhSCAIQAUIFMS4wLjBKRVRha2VzIGEgY29udGVudC1hZGRyZXNzZWQgc25hcHNob3Qgb2YgYSBydW4ncyB3b3JrdHJlZSBhbmQgYXJ0aWZhY3RzLhK7AQoHRGVzdHJveRIyLm51bWluYW5jZS5wb3J0cy5zYW5kYm94cHJvdmlkZXIudjEuRGVzdHJveVJlcXVlc3QaMy5udW1pbmFuY2UucG9ydHMuc2FuZGJveHByb3ZpZGVyLnYxLkRlc3Ryb3lSZXNwb25zZSJHirUYQwgCEAEYAUIFMS4wLjBKNERlc3Ryb3lzIGEgcnVuJ3Mgc2FuZGJveCBhbmQgcmVsZWFzZXMgaXRzIHJlc291cmNlcy5CzgIKJmNvbS5udW1pbmFuY2UucG9ydHMuc2FuZGJveHByb3ZpZGVyLnYxQhRTYW5kYm94cHJvdmlkZXJQcm90b1ABWmNnaXRodWIuY29tL251bWluYW5jZS9udW1pbmFuY2Utc3BlYy9zZGsvZ28vZ2VuL251bWluYW5jZS9wb3J0cy9zYW5kYm94cHJvdmlkZXIvdjE7c2FuZGJveHByb3ZpZGVydjGiAgNOUFOqAiJOdW1pbmFuY2UuUG9ydHMuU2FuZGJveHByb3ZpZGVyLlYxygIiTnVtaW5hbmNlXFBvcnRzXFNhbmRib3hwcm92aWRlclxWMeICLk51bWluYW5jZVxQb3J0c1xTYW5kYm94cHJvdmlkZXJcVjFcR1BCTWV0YWRhdGHqAiVOdW1pbmFuY2U6OlBvcnRzOjpTYW5kYm94cHJvdmlkZXI6OlYxYgZwcm90bzM", [file_google_protobuf_duration, file_google_protobuf_timestamp, file_numinance_annotations_v1_annotations, file_numinance_evidence_v1_record, file_numinance_types_v1_types]);

/**
 * CreateRequest carries the sandbox specification of one run.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.CreateRequest
 */
export type CreateRequest = Message<"numinance.ports.sandboxprovider.v1.CreateRequest"> & {
  /**
   * spec is required.
   *
   * @generated from field: numinance.ports.sandboxprovider.v1.SandboxSpec spec = 1;
   */
  spec?: SandboxSpec | undefined;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.CreateRequest.
 * Use `create(CreateRequestSchema)` to create a new message.
 */
export const CreateRequestSchema: GenMessage<CreateRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 0);

/**
 * CreateResponse is the provider's answer to Create, or to its dry run; a claim.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.CreateResponse
 */
export type CreateResponse = Message<"numinance.ports.sandboxprovider.v1.CreateResponse"> & {
  /**
   * profile is the profile the provider created; it equals spec.profile, or the host destroys the sandbox and records a provider fault.
   *
   * @generated from field: numinance.evidence.v1.SandboxProfile profile = 1;
   */
  profile: SandboxProfile;

  /**
   * runtime_version is the provider's runtime name and version, printable, 1 to 128 bytes; the host checks it against the runner's host attestation.
   *
   * @generated from field: string runtime_version = 2;
   */
  runtimeVersion: string;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.CreateResponse.
 * Use `create(CreateResponseSchema)` to create a new message.
 */
export const CreateResponseSchema: GenMessage<CreateResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 1);

/**
 * SandboxSpec is everything one sandbox is given. Every field is required unless its comment says it may be unset or empty.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.SandboxSpec
 */
export type SandboxSpec = Message<"numinance.ports.sandboxprovider.v1.SandboxSpec"> & {
  /**
   * change_id is ^chg_[0-9a-f]{32}$.
   *
   * @generated from field: string change_id = 1;
   */
  changeId: string;

  /**
   * work_unit_id is ^wu_[0-9a-f]{32}$.
   *
   * @generated from field: string work_unit_id = 2;
   */
  workUnitId: string;

  /**
   * run_id is ^run_[0-9a-f]{32}$ and keys the sandbox; a retry of a run is a new run with a new sandbox.
   *
   * @generated from field: string run_id = 3;
   */
  runId: string;

  /**
   * profile is PROCESS, CONTAINER or GVISOR; MICROVM is refused (ADR 0033).
   *
   * @generated from field: numinance.evidence.v1.SandboxProfile profile = 4;
   */
  profile: SandboxProfile;

  /**
   * image_digest is the digest of the image the runner pulled and verified (signature and lockfile pin) before Create; required under CONTAINER and GVISOR, unset under PROCESS.
   *
   * @generated from field: numinance.types.v1.Digest image_digest = 5;
   */
  imageDigest?: Digest | undefined;

  /**
   * worktree_path is the host path of the run's worktree (file comment, Paths); mounted as the sandbox's working directory.
   *
   * @generated from field: string worktree_path = 6;
   */
  worktreePath: string;

  /**
   * artifacts_path is the host path of the run's artifact directory, not equal to, above or under another path of this spec; mounted writable.
   *
   * @generated from field: string artifacts_path = 7;
   */
  artifactsPath: string;

  /**
   * allowed_paths are the only writable worktree paths; empty for a read-only run. None equals, contains or lies under a masked path.
   *
   * @generated from field: repeated string allowed_paths = 8;
   */
  allowedPaths: string[];

  /**
   * masked_paths are the worktree paths absent from the sandbox: the host's literal protected set and the run's denied paths, at least one.
   *
   * @generated from field: repeated string masked_paths = 9;
   */
  maskedPaths: string[];

  /**
   * egress_proxy is the address of the run's own egress proxy, an IP literal in canonical form (IPv4 dotted-quad with no leading zeros; IPv6 in RFC 5952 form, in brackets, with no zone id) and a port 1 to 65535; never loopback, link-local, unspecified, multicast or broadcast, a metadata address (169.254.169.254, 169.254.170.2, fd00:ec2::254, 168.63.129.16), or the IPv4-mapped or NAT64 (64:ff9b::/96) form of any of those. Empty for no network, and under PROCESS.
   *
   * @generated from field: string egress_proxy = 10;
   */
  egressProxy: string;

  /**
   * limits are the run's resource limits from its budget.
   *
   * @generated from field: numinance.ports.sandboxprovider.v1.SandboxLimits limits = 11;
   */
  limits?: SandboxLimits | undefined;

  /**
   * deadline is when the sandbox ends, in the future at Create; the provider destroys it then, as Destroy without release_snapshots.
   *
   * @generated from field: google.protobuf.Timestamp deadline = 12;
   */
  deadline?: Timestamp | undefined;

  /**
   * traceparent is the W3C trace context, ^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$, set as TRACEPARENT in every exec; may be empty.
   *
   * @generated from field: string traceparent = 13;
   */
  traceparent: string;

  /**
   * restore_from is a snapshot of an earlier run of the same work unit, whose reference is still held (the provider refuses another with FAILED_PRECONDITION); the provider fills the empty worktree and artifact directories from it before the sandbox starts, creating every entry beneath its directory without following any symlink and never writing through a restored link, and refuses (FAILED_PRECONDITION) a snapshot holding an entry at or under this spec's masked_paths. Restore copies content and never hardlinks to provider storage; it creates directories 0755 and files 0644, or 0755 with the owner execute bit, every entry owned by the run's user, with no setuid, setgid or sticky bit and no group or other write. The host then recomputes both tree digests and compares them with the ones Snapshot returned, and checks that no masked path is present on disk, before the first Exec. Unset for a fresh sandbox.
   *
   * @generated from field: numinance.types.v1.Digest restore_from = 14;
   */
  restoreFrom?: Digest | undefined;

  /**
   * output_path is the host path of the run's output directory (file comment, Output), not equal to, above or under another path of this spec, and empty at Create; never mounted in the sandbox.
   *
   * @generated from field: string output_path = 15;
   */
  outputPath: string;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.SandboxSpec.
 * Use `create(SandboxSpecSchema)` to create a new message.
 */
export const SandboxSpecSchema: GenMessage<SandboxSpec> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 2);

/**
 * SandboxLimits are the resource limits of one sandbox; every field is non-zero. The wall clock is SandboxSpec.deadline.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.SandboxLimits
 */
export type SandboxLimits = Message<"numinance.ports.sandboxprovider.v1.SandboxLimits"> & {
  /**
   * cpu_millicores is the CPU the sandbox may use, in thousandths of a core.
   *
   * @generated from field: uint32 cpu_millicores = 1;
   */
  cpuMillicores: number;

  /**
   * memory_bytes is the memory the sandbox may use.
   *
   * @generated from field: uint64 memory_bytes = 2;
   */
  memoryBytes: bigint;

  /**
   * pids is the number of processes and threads the sandbox may hold at once.
   *
   * @generated from field: uint32 pids = 3;
   */
  pids: number;

  /**
   * disk_bytes bounds what the sandbox may write to the worktree, the artifact directory and its tmpfs together, and separately the run's output.
   *
   * @generated from field: uint64 disk_bytes = 4;
   */
  diskBytes: bigint;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.SandboxLimits.
 * Use `create(SandboxLimitsSchema)` to create a new message.
 */
export const SandboxLimitsSchema: GenMessage<SandboxLimits> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 3);

/**
 * ExecRequest starts one command.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.ExecRequest
 */
export type ExecRequest = Message<"numinance.ports.sandboxprovider.v1.ExecRequest"> & {
  /**
   * run_id is the sandbox, ^run_[0-9a-f]{32}$.
   *
   * @generated from field: string run_id = 1;
   */
  runId: string;

  /**
   * exec_seq is the host's number for this exec within the run, 1 to 1024.
   *
   * @generated from field: uint32 exec_seq = 2;
   */
  execSeq: number;

  /**
   * argv is the command and its arguments, 1 to 256 entries, each printable; run directly, never through a shell. Untrusted content; the host scans it for issued handles and canaries before dispatch.
   *
   * @generated from field: repeated string argv = 3;
   */
  argv: string[];

  /**
   * working_dir is a worktree path the command starts in, not at or under a masked path; empty for the worktree root.
   *
   * @generated from field: string working_dir = 4;
   */
  workingDir: string;

  /**
   * env is the command's whole environment besides TRACEPARENT, at most 64 entries, unique by name; never a credential, a handle or a token, and scanned by the host like argv.
   *
   * @generated from field: repeated numinance.ports.sandboxprovider.v1.EnvVar env = 5;
   */
  env: EnvVar[];

  /**
   * timeout is how long the command may run, 1 second up to the time left before the sandbox deadline, checked only for a new key (Exec).
   *
   * @generated from field: google.protobuf.Duration timeout = 6;
   */
  timeout?: Duration | undefined;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.ExecRequest.
 * Use `create(ExecRequestSchema)` to create a new message.
 */
export const ExecRequestSchema: GenMessage<ExecRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 4);

/**
 * EnvVar is one environment variable.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.EnvVar
 */
export type EnvVar = Message<"numinance.ports.sandboxprovider.v1.EnvVar"> & {
  /**
   * name is ^[A-Za-z_][A-Za-z0-9_]{0,127}$ and not TRACEPARENT.
   *
   * @generated from field: string name = 1;
   */
  name: string;

  /**
   * value is printable; may be empty.
   *
   * @generated from field: string value = 2;
   */
  value: string;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.EnvVar.
 * Use `create(EnvVarSchema)` to create a new message.
 */
export const EnvVarSchema: GenMessage<EnvVar> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 5);

/**
 * ExecResponse is empty: the exec is addressed by (run_id, exec_seq).
 *
 * @generated from message numinance.ports.sandboxprovider.v1.ExecResponse
 */
export type ExecResponse = Message<"numinance.ports.sandboxprovider.v1.ExecResponse"> & {
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.ExecResponse.
 * Use `create(ExecResponseSchema)` to create a new message.
 */
export const ExecResponseSchema: GenMessage<ExecResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 6);

/**
 * WaitRequest asks for the state of one exec.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.WaitRequest
 */
export type WaitRequest = Message<"numinance.ports.sandboxprovider.v1.WaitRequest"> & {
  /**
   * run_id is ^run_[0-9a-f]{32}$.
   *
   * @generated from field: string run_id = 1;
   */
  runId: string;

  /**
   * exec_seq is 1 to 1024.
   *
   * @generated from field: uint32 exec_seq = 2;
   */
  execSeq: number;

  /**
   * wait_timeout is how long Wait may block, 0 to 60 seconds; unset or 0 answers at once.
   *
   * @generated from field: google.protobuf.Duration wait_timeout = 3;
   */
  waitTimeout?: Duration | undefined;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.WaitRequest.
 * Use `create(WaitRequestSchema)` to create a new message.
 */
export const WaitRequestSchema: GenMessage<WaitRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 7);

/**
 * WaitResponse is the state of one exec; a claim.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.WaitResponse
 */
export type WaitResponse = Message<"numinance.ports.sandboxprovider.v1.WaitResponse"> & {
  /**
   * state is required.
   *
   * @generated from field: numinance.ports.sandboxprovider.v1.ExecState state = 1;
   */
  state: ExecState;

  /**
   * exit_code is the command's exit status for EXITED, and 0 for every other state.
   *
   * @generated from field: int32 exit_code = 2;
   */
  exitCode: number;

  /**
   * stdout_sha256 is the digest of exec/<exec_seq>/stdout once the exec has ended, set for EXITED, TIMED_OUT, LIMIT_EXCEEDED and KILLED and unset otherwise. The host recomputes it from the file.
   *
   * @generated from field: numinance.types.v1.Digest stdout_sha256 = 3;
   */
  stdoutSha256?: Digest | undefined;

  /**
   * stderr_sha256 is the same for exec/<exec_seq>/stderr.
   *
   * @generated from field: numinance.types.v1.Digest stderr_sha256 = 4;
   */
  stderrSha256?: Digest | undefined;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.WaitResponse.
 * Use `create(WaitResponseSchema)` to create a new message.
 */
export const WaitResponseSchema: GenMessage<WaitResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 8);

/**
 * SnapshotRequest names the run to snapshot.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.SnapshotRequest
 */
export type SnapshotRequest = Message<"numinance.ports.sandboxprovider.v1.SnapshotRequest"> & {
  /**
   * run_id is ^run_[0-9a-f]{32}$.
   *
   * @generated from field: string run_id = 1;
   */
  runId: string;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.SnapshotRequest.
 * Use `create(SnapshotRequestSchema)` to create a new message.
 */
export const SnapshotRequestSchema: GenMessage<SnapshotRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 9);

/**
 * SnapshotResponse names the snapshot taken.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.SnapshotResponse
 */
export type SnapshotResponse = Message<"numinance.ports.sandboxprovider.v1.SnapshotResponse"> & {
  /**
   * snapshot is the content address of the snapshot in the provider's storage.
   *
   * @generated from field: numinance.types.v1.Digest snapshot = 1;
   */
  snapshot?: Digest | undefined;

  /**
   * worktree_digest is the tree digest of the worktree in the copy: sha256 of one LF-terminated line per entry, sorted bytewise by path, "d <n> <path>" for a directory, "f <n> <path> <hex>" or "x <n> <path> <hex>" for a file without or with the owner execute bit (no other mode bit is digested) with the hex sha256 of its content, "l <n> <path> <hex>" for a symlink with the hex sha256 of its target, where <n> is the path's length in bytes in decimal; paths relative and NFC; the root directory has no line; hex is lowercase; a symlink's target is its raw link bytes. Golden vector: conformance/sandboxprovider/v1/testdata/tree-digest/. A claim the host checks at restore.
   *
   * @generated from field: numinance.types.v1.Digest worktree_digest = 2;
   */
  worktreeDigest?: Digest | undefined;

  /**
   * artifacts_digest is the same for the artifact directory.
   *
   * @generated from field: numinance.types.v1.Digest artifacts_digest = 3;
   */
  artifactsDigest?: Digest | undefined;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.SnapshotResponse.
 * Use `create(SnapshotResponseSchema)` to create a new message.
 */
export const SnapshotResponseSchema: GenMessage<SnapshotResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 10);

/**
 * DestroyRequest names the run to destroy.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.DestroyRequest
 */
export type DestroyRequest = Message<"numinance.ports.sandboxprovider.v1.DestroyRequest"> & {
  /**
   * run_id is ^run_[0-9a-f]{32}$.
   *
   * @generated from field: string run_id = 1;
   */
  runId: string;

  /**
   * release_snapshots also releases every snapshot reference this run holds; the host sets it when the work unit ends or a later run of it holds its own snapshot.
   *
   * @generated from field: bool release_snapshots = 2;
   */
  releaseSnapshots: boolean;
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.DestroyRequest.
 * Use `create(DestroyRequestSchema)` to create a new message.
 */
export const DestroyRequestSchema: GenMessage<DestroyRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 11);

/**
 * DestroyResponse is empty.
 *
 * @generated from message numinance.ports.sandboxprovider.v1.DestroyResponse
 */
export type DestroyResponse = Message<"numinance.ports.sandboxprovider.v1.DestroyResponse"> & {
};

/**
 * Describes the message numinance.ports.sandboxprovider.v1.DestroyResponse.
 * Use `create(DestroyResponseSchema)` to create a new message.
 */
export const DestroyResponseSchema: GenMessage<DestroyResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 12);

/**
 * ExecState is the state of one exec.
 *
 * @generated from enum numinance.ports.sandboxprovider.v1.ExecState
 */
export enum ExecState {
  /**
   * EXEC_STATE_UNSPECIFIED is the zero value; it is rejected.
   *
   * @generated from enum value: EXEC_STATE_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * EXEC_STATE_RUNNING: the command has not ended.
   *
   * @generated from enum value: EXEC_STATE_RUNNING = 1;
   */
  RUNNING = 1,

  /**
   * EXEC_STATE_EXITED: the command ended by itself; exit_code is its status.
   *
   * @generated from enum value: EXEC_STATE_EXITED = 2;
   */
  EXITED = 2,

  /**
   * EXEC_STATE_TIMED_OUT: the provider killed the command at its timeout.
   *
   * @generated from enum value: EXEC_STATE_TIMED_OUT = 3;
   */
  TIMED_OUT = 3,

  /**
   * EXEC_STATE_LIMIT_EXCEEDED: the provider killed the command at a limit of SandboxLimits or the output bound.
   *
   * @generated from enum value: EXEC_STATE_LIMIT_EXCEEDED = 4;
   */
  LIMIT_EXCEEDED = 4,

  /**
   * EXEC_STATE_KILLED: Destroy or the sandbox deadline ended the command.
   *
   * @generated from enum value: EXEC_STATE_KILLED = 5;
   */
  KILLED = 5,

  /**
   * EXEC_STATE_START_FAILED: the command did not start (for example, argv[0] does not exist).
   *
   * @generated from enum value: EXEC_STATE_START_FAILED = 6;
   */
  START_FAILED = 6,
}

/**
 * Describes the enum numinance.ports.sandboxprovider.v1.ExecState.
 */
export const ExecStateSchema: GenEnum<ExecState> = /*@__PURE__*/
  enumDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 0);

/**
 * SandboxProviderService is the sandboxprovider port a sandbox runtime adapter serves.
 *
 * @generated from service numinance.ports.sandboxprovider.v1.SandboxProviderService
 */
export const SandboxProviderService: GenService<{
  /**
   * Create creates the sandbox of spec.run_id and returns when it is ready to Exec. One sandbox per run_id: the provider keeps the key of a created sandbox, across its own restarts, at least until the spec's deadline, and looks it up before any other check but the bounds; a Create with the same spec is a replay that answers the first success and creates nothing, another spec is ALREADY_EXISTS, and a Create of a destroyed run is FAILED_PRECONDITION. LOW: nothing runs until Exec, and Destroy releases all of it; the directories are the host's. Dry run checks the spec and whether the provider can create it on this host (profile, image, paths, capacity), creates nothing, and answers what it would create.
   *
   * @generated from rpc numinance.ports.sandboxprovider.v1.SandboxProviderService.Create
   */
  create: {
    methodKind: "unary";
    input: typeof CreateRequestSchema;
    output: typeof CreateResponseSchema;
  },
  /**
   * Exec starts one command in the sandbox of run_id and returns once it has started; Wait is the get of the operation, a separate READ invocation. There is no cancel: the exec timeout, Destroy and the sandbox deadline end it. On the plugin protocol Exec is one terminal Invoke message, whose success means started. MEDIUM: the command is untrusted code; under CONTAINER and GVISOR it is confined to the disposable sandbox, and its effects outside pass the egress proxy and the broker under the run's grant. The decision for an Exec takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. An Exec with the same (run_id, exec_seq) and the same request is a replay that starts nothing; another request is ALREADY_EXISTS. The provider looks the key up before any check but the bounds that depend on no time or state, so the timeout against the time left and the run's state are checked only for a new key, and a replay near the deadline or after Destroy is still answered. No dry run: running the command is the only way to learn what it does, and the sandbox is discarded unless the host takes a candidate from the worktree.
   *
   * @generated from rpc numinance.ports.sandboxprovider.v1.SandboxProviderService.Exec
   */
  exec: {
    methodKind: "unary";
    input: typeof ExecRequestSchema;
    output: typeof ExecResponseSchema;
  },
  /**
   * Wait returns the state of one exec when it ends or when wait_timeout passes, whichever is first; a RUNNING answer means call again.
   *
   * @generated from rpc numinance.ports.sandboxprovider.v1.SandboxProviderService.Wait
   */
  wait: {
    methodKind: "unary";
    input: typeof WaitRequestSchema;
    output: typeof WaitResponseSchema;
  },
  /**
   * Snapshot freezes every process of run_id (under PROCESS with a cgroup freezer where one exists, otherwise by stopping every process of the run's user, repeated until two passes find no running one, and continuing them after; a provider that cannot freeze refuses Snapshot with FAILED_PRECONDITION), takes a content-addressed copy of the worktree and artifact directories, computes their tree digests on the copy, and thaws the processes before it returns. It holds a reference (run_id, snapshot) to the copy in the provider's storage on the runner's host, outside the run's disk_bytes. The copy leaves out every entry at or under the spec's masked_paths (compared under NFC and case folding), records of each file only its content and owner execute bit (no other mode bit, no owner), records a symlink as a link and never reads through it, refuses with FAILED_PRECONDITION a file with more than one hard link, a special file (FIFO, socket, device), an entry whose name breaks the path rules (printable NFC segments) and two names equal after NFC normalization or after NFC normalization and case folding, and refuses with RESOURCE_EXHAUSTED a copy whose apparent sizes exceed disk_bytes. Content is deleted when its last reference is released: by Destroy with release_snapshots, or by the provider between 7 and 30 days after the run's deadline, after which a later run starts fresh. At most 64 snapshots per run, within a per-host byte cap the provider is configured with. LOW: it adds a copy and changes nothing in the sandbox. Not idempotent: a retry takes a new copy of the current state; equal content has an equal digest, so a retry is harmless. No dry run: there is no plan to preview, and the copy is the only effect.
   *
   * @generated from rpc numinance.ports.sandboxprovider.v1.SandboxProviderService.Snapshot
   */
  snapshot: {
    methodKind: "unary";
    input: typeof SnapshotRequestSchema;
    output: typeof SnapshotResponseSchema;
  },
  /**
   * Destroy kills every process of run_id, removes the sandbox and its tmpfs and releases its resources; the host directories stay. Once no process of the run's user remains, Destroy hands the worktree and artifact roots and every entry under them to the host's user without following any symlink, with modes normalized as at restore (no setuid, setgid or sticky bit, no group or other write). The run's uid is the host uid its processes ran as (under a user namespace, the host uid the run's user maps to). It changes in place an entry owned by the run's uid and, under CONTAINER and GVISOR only, one owned by the host's user (base content there; under PROCESS, where Create hands every entry to the run's user, a host-user entry is foreign). It removes, never changing its owner, mode or content, every other entry (a directory with everything under it, never following links) and every non-directory with more than one link, judged on the link counts before any removal, so a run that hardlinks a file loses every name of it; the link-count test applies to non-directories only. It reports each removal to the host, which records an abnormal action (the report's channel is owed by the sandbox context). At Create and at Destroy the handover also removes every POSIX ACL and every user.*, trusted.* and security.capability extended attribute, and clears every file flag; at Create it gives the run's entries an LSM label unique to the run (a per-run MCS pair, never a shared container label), and at Destroy it resets every LSM label to the work root's default context. The Snapshot copy, the handovers, the removals and the host copy never cross a filesystem boundary (every entry's st_dev equals its root's) and refuse a mount point inside a tree; mount points are found from the mount table or mount ids (statx STATX_MNT_ID, /proc/self/mountinfo), never by a stat that reaches a mounted file system. Under PROCESS, Snapshot freezes first, then detaches every mount under the run's directories (lazy unmount, and abort of every FUSE connection owned by the run's uid), then copies, then thaws; the Destroy handover detaches them after every process is dead. A run counts as destroyed only once the handover is complete: until then every Destroy (a retry of one that failed partway or was cut by a provider restart, or the host's next Destroy after the deadline destroy) completes it and answers OK only after the handover; one that cannot is UNAVAILABLE and the caller retries. The host does not act on that OK: it then copies the kept tree into its own storage, opening each entry without following links (an entry that is not a regular file, directory or symlink is refused unopened), verifying owner, link count, type and mode with fstat on the opened descriptor, and reading content from that descriptor through a reader bounded by disk_bytes of apparent size, within the name rules of Snapshot and an entry bound (1,000,000 by default, at most 10,000,000) and depth bound (at most 256), a configuration above a ceiling being refused, and copying no ACL, extended attribute or file flag; under CONTAINER and GVISOR it accepts a host-owned entry only at its own base path with that path's base digest and refuses every other (a rename or copy by the run is run-owned on disk); any refusal is an abnormal action, and the host builds, checks and uses the candidate only from that copy. It deletes the kept directories once the copy is taken or the run fails, after keeping a refused tree for the incident workflow in an access-restricted quarantine store under bounded retention, which evidence references by digest only (never in an exported evidence bundle), scanned and redacted for handles before any reviewer view. Destroy of a destroyed or unknown run succeeds, and still releases the run's snapshot references when release_snapshots is set. Each exec's key and final state survive Destroy until the spec's deadline, like the Create key, so Wait answers KILLED for an exec Destroy ended. LOW: under every profile, nothing the host keeps is removed, untouched base content included, but names the run had no right to keep (foreign entries, multi-link files), and snapshot content another reference holds stays. No dry run: the release is the whole effect, and the host already knows what the run holds.
   *
   * @generated from rpc numinance.ports.sandboxprovider.v1.SandboxProviderService.Destroy
   */
  destroy: {
    methodKind: "unary";
    input: typeof DestroyRequestSchema;
    output: typeof DestroyResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_sandboxprovider_v1_sandboxprovider, 0);

