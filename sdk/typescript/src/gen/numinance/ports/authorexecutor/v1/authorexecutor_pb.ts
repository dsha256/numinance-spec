// The authorexecutor port (ADR 0008): one coding harness behind one adapter, driven for one run of one work unit inside the work-unit sandbox.
// Callers: only the execution context calls this port, for a run whose ActionContext.actor_kind is author-agent or reviewer-agent; the host refuses every other actor kind. A reviewer-agent run is read-only: its request has no allowed_paths, no credential_capabilities and no FILESYSTEM_WRITE, SHELL or NETWORK in allowed_capabilities, or the host refuses it; the host builds no candidate from a reviewer-agent run and discards its worktree. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: nothing in a request is advisory, and nothing a provider reports is authority. The sandbox, the egress proxy and the broker are configured from the host's resolved grant, never from the request, which is the harness's copy. Inference egress for the pinned model is configured by the host outside allowed_capabilities, credential_capabilities and network_policy_ref, and only to the endpoint of the pinned model_id validated for residency at Change start (ADR 0019): the proxy enforces it, nothing inside the sandbox redirects it, requests to non-inference methods there are denied, and it counts in network_destinations and in the dispatch risk. Its credential is a proxy-injected handle; a materialized key only when the manifest declares materialized and policy allows it, scoped to inference on that model, expiring by the deadline, never the application key, and recorded in host evidence. Under the PROCESS sandbox profile (sandboxprovider port) there is no egress proxy: the inference credential is a materialized key only where the manifest and policy allow it, nothing stops a redirect of inference egress, and the decision for Execute is PRIVILEGED; without that allowance the host refuses the run. The host recomputes every authority field of a ChangeExecutionRequest from the grant, the router decision and the plan, and refuses to dispatch a request where any differs. Capabilities and every ExecutionEvent are claims of the provider; the host records its own broker, proxy and sandbox decisions and never treats an event as an authorization. EXECUTION_COMPLETED is never a gate signal: gates read verification. The host reports a canary or an issued handle in an event, a FILE_CHANGED outside allowed_paths and a candidate the final-tree check below refuses, as an abnormal action, which starts the incident workflow (the design).
// Payloads: requests and events carry references and digests, never file contents, prompt text or credential material (a platform invariant: payloads carry references, never content). Of the request's references the provider may resolve through HostService.ResolveReference only task_ref, plan_ref, context_bundle_ref and test_requirements_ref; worktree_ref and network_policy_ref are host-only (field option host_only) and refused. Content an event points at is untrusted data.
// Paths: a path is relative to the worktree root, '/'-separated, NFC, printable, with segments of 1 to 255 bytes, no leading '/', no '\' and no '.' or '..' segment; it names a file or a directory with everything below it. Matching is segment-wise and literal (no globs); denied and protected paths match under Unicode simple case folding of the NFC form. The sandbox enforces on the resolved path after following symlinks and refuses hardlinks; the host checks denied and protected paths again on the final tree of the candidate, after NFC normalization and case folding, and rejects a name that is not NFC, a symlink whose target leaves the allowed paths, and a gitlink. The host builds the candidate against the run's base with every masked path (sandboxprovider port, Paths) taken from the base, so a masked path missing from the worktree on disk (a restored run, or PROCESS) is never a deletion; before that substitution it refuses, as an abnormal action, an entry on disk at or under a masked path that differs from the base (present where the base has none, or other content), so a write there is reported, not dropped; and it refuses a candidate that removes, renames or changes an entry at or under a protected or denied path. A segment that case-folds to .git, .gitmodules, .gitattributes or .lfsconfig, at any depth, is outside every grant, whatever allowed_paths says. The host reads the candidate tree with git hooks, fsmonitor, filter and smudge drivers, LFS and submodule recursion disabled, and never applies repository-local git configuration.
// Stream: Execute is server-streaming. A stream holds 1 to 1023 events: its first is EXECUTION_STARTED at sequence 1, sequences are consecutive, and its last and only its last is EXECUTION_COMPLETED or EXECUTION_FAILED. Each ExecuteResponse is at most 32 KiB serialized. On the plugin protocol each event rides one RUNNING Invoke message and the terminal Invoke message is SUCCEEDED after EXECUTION_COMPLETED and FAILED after EXECUTION_FAILED, so the call fits the Invoke bounds for an OPERATION target (1024 messages, 64 KiB per RUNNING message); the mapping is the server-streaming rule of PluginService.Invoke (plugin/v1). The host rejects a stream that breaks a rule and records the run as failed.
// Status codes: a request is checked before the harness starts, and a rejection sends no event. The host refuses with FAILED_PRECONDITION, before any Invoke, a request its recomputation or the pinned ExecutorManifest refuses, or a run_id already dispatched, records the refusal and returns it to the execution context. The provider rejects with INVALID_ARGUMENT a request that breaks a bound and with UNAVAILABLE when the harness cannot start. Once EXECUTION_STARTED is sent, every failure ends the stream with EXECUTION_FAILED. At the deadline or on a cancel the host ends the call and records the run as failed; the provider sends nothing after it. On the plugin protocol a provider rejection is the terminal FAILED message with one Diagnostic reason, INVALID_ARGUMENT with REASON_INVALID_ARGUMENT and UNAVAILABLE with REASON_UPSTREAM_UNAVAILABLE, and the host records the deadline as REASON_TIMEOUT and a cancel as REASON_CANCELLED. A stream that reaches 1023 events without a terminal one is budget exhaustion: the run fails and the execution context pauses the Change and escalates.
// Evidence: the stream is recorded as a provider claim, and evaluation outcomes come from host verification. Execute declares no evidence types: the execution context writes the stream and route records itself as host receipts, never a provider claim, and a declared type the provider does not emit would make the invocation PARTIAL.
// Bounds: every string is at most 1024 bytes and every repeated field at most 256 entries unless stated; over-limit input is rejected, never truncated; duplicate entries in any repeated field are rejected; enums are closed, and zero and unknown numbers are rejected. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Every element is since 1.0.0.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/authorexecutor/v1/authorexecutor.proto (package numinance.ports.authorexecutor.v1, syntax proto3)
/* eslint-disable */

import type { GenEnum, GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { enumDesc, fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { file_google_protobuf_timestamp } from "@bufbuild/protobuf/wkt";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { Digest, ExecutorCapability, Reference } from "../../../types/v1/types_pb";
import { file_numinance_types_v1_types } from "../../../types/v1/types_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/authorexecutor/v1/authorexecutor.proto.
 */
export const file_numinance_ports_authorexecutor_v1_authorexecutor: GenFile = /*@__PURE__*/
  fileDesc("CjZudW1pbmFuY2UvcG9ydHMvYXV0aG9yZXhlY3V0b3IvdjEvYXV0aG9yZXhlY3V0b3IucHJvdG8SIW51bWluYW5jZS5wb3J0cy5hdXRob3JleGVjdXRvci52MSIVChNDYXBhYmlsaXRpZXNSZXF1ZXN0ImEKFENhcGFiaWxpdGllc1Jlc3BvbnNlEkkKDGNhcGFiaWxpdGllcxgBIAMoDjImLm51bWluYW5jZS50eXBlcy52MS5FeGVjdXRvckNhcGFiaWxpdHlCC5K1GAcKBTEuMC4wImkKDkV4ZWN1dGVSZXF1ZXN0ElcKB3JlcXVlc3QYASABKAsyOS5udW1pbmFuY2UucG9ydHMuYXV0aG9yZXhlY3V0b3IudjEuQ2hhbmdlRXhlY3V0aW9uUmVxdWVzdEILkrUYBwoFMS4wLjAiZgoLUGxhblJlcXVlc3QSVwoHcmVxdWVzdBgBIAEoCzI5Lm51bWluYW5jZS5wb3J0cy5hdXRob3JleGVjdXRvci52MS5DaGFuZ2VFeGVjdXRpb25SZXF1ZXN0QguStRgHCgUxLjAuMCJdCgxQbGFuUmVzcG9uc2USTQoFZXZlbnQYASABKAsyMS5udW1pbmFuY2UucG9ydHMuYXV0aG9yZXhlY3V0b3IudjEuRXhlY3V0aW9uRXZlbnRCC5K1GAcKBTEuMC4wImAKD0V4ZWN1dGVSZXNwb25zZRJNCgVldmVudBgBIAEoCzIxLm51bWluYW5jZS5wb3J0cy5hdXRob3JleGVjdXRvci52MS5FeGVjdXRpb25FdmVudEILkrUYBwoFMS4wLjAi/QkKFkNoYW5nZUV4ZWN1dGlvblJlcXVlc3QSHgoJY2hhbmdlX2lkGAEgASgJQguStRgHCgUxLjAuMBIhCgx3b3JrX3VuaXRfaWQYAiABKAlCC5K1GAcKBTEuMC4wEhsKBnJ1bl9pZBgDIAEoCUILkrUYBwoFMS4wLjASHQoIZ3JhbnRfaWQYBCABKAlCC5K1GAcKBTEuMC4wEkIKEXJlcG9zaXRvcnlfZGlnZXN0GAUgASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjASQgoMd29ya3RyZWVfcmVmGAYgASgLMh0ubnVtaW5hbmNlLnR5cGVzLnYxLlJlZmVyZW5jZUINkrUYCQoFMS4wLjAoARI8Cgh0YXNrX3JlZhgHIAEoCzIdLm51bWluYW5jZS50eXBlcy52MS5SZWZlcmVuY2VCC5K1GAcKBTEuMC4wEjwKCHBsYW5fcmVmGAggASgLMh0ubnVtaW5hbmNlLnR5cGVzLnYxLlJlZmVyZW5jZUILkrUYBwoFMS4wLjASRgoSY29udGV4dF9idW5kbGVfcmVmGAkgASgLMh0ubnVtaW5hbmNlLnR5cGVzLnYxLlJlZmVyZW5jZUILkrUYBwoFMS4wLjASIAoLZXhlY3V0b3JfaWQYCiABKAlCC5K1GAcKBTEuMC4wEh0KCG1vZGVsX2lkGAsgASgJQguStRgHCgUxLjAuMBIiCg1hbGxvd2VkX3BhdGhzGAwgAygJQguStRgHCgUxLjAuMBIhCgxkZW5pZWRfcGF0aHMYDSADKAlCC5K1GAcKBTEuMC4wElEKFGFsbG93ZWRfY2FwYWJpbGl0aWVzGA4gAygOMiYubnVtaW5hbmNlLnR5cGVzLnYxLkV4ZWN1dG9yQ2FwYWJpbGl0eUILkrUYBwoFMS4wLjASSAoSbmV0d29ya19wb2xpY3lfcmVmGA8gASgLMh0ubnVtaW5hbmNlLnR5cGVzLnYxLlJlZmVyZW5jZUINkrUYCQoFMS4wLjAoARIsChdjcmVkZW50aWFsX2NhcGFiaWxpdGllcxgQIAMoCUILkrUYBwoFMS4wLjASTwoGYnVkZ2V0GBEgASgLMjIubnVtaW5hbmNlLnBvcnRzLmF1dGhvcmV4ZWN1dG9yLnYxLkV4ZWN1dGlvbkJ1ZGdldEILkrUYBwoFMS4wLjASOQoIZGVhZGxpbmUYEiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQguStRgHCgUxLjAuMBJJChV0ZXN0X3JlcXVpcmVtZW50c19yZWYYEyABKAsyHS5udW1pbmFuY2UudHlwZXMudjEuUmVmZXJlbmNlQguStRgHCgUxLjAuMBIoChNwcm90ZWN0ZWRfcmVzb3VyY2VzGBQgAygJQguStRgHCgUxLjAuMBJDChJlbnZpcm9ubWVudF9kaWdlc3QYFSABKAsyGi5udW1pbmFuY2UudHlwZXMudjEuRGlnZXN0QguStRgHCgUxLjAuMBI+Cg1wb2xpY3lfZGlnZXN0GBYgASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjASPwoOcHJvZmlsZV9kaWdlc3QYFyABKAsyGi5udW1pbmFuY2UudHlwZXMudjEuRGlnZXN0QguStRgHCgUxLjAuMCLCAQoPRXhlY3V0aW9uQnVkZ2V0EhsKBnRva2VucxgBIAEoDUILkrUYBwoFMS4wLjASHwoKdG9vbF9jYWxscxgCIAEoDUILkrUYBwoFMS4wLjASIgoNZmlsZXNfY2hhbmdlZBgDIAEoDUILkrUYBwoFMS4wLjASIgoNbGluZXNfY2hhbmdlZBgEIAEoDUILkrUYBwoFMS4wLjASKQoUbmV0d29ya19kZXN0aW5hdGlvbnMYBSABKA1CC5K1GAcKBTEuMC4wIp0CCg5FeGVjdXRpb25FdmVudBJQCgR0eXBlGAEgASgOMjUubnVtaW5hbmNlLnBvcnRzLmF1dGhvcmV4ZWN1dG9yLnYxLkV4ZWN1dGlvbkV2ZW50VHlwZUILkrUYBwoFMS4wLjASHQoIc2VxdWVuY2UYAiABKARCC5K1GAcKBTEuMC4wEjwKC29jY3VycmVkX2F0GAMgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEILkrUYBwoFMS4wLjASHAoHc3ViamVjdBgEIAEoCUILkrUYBwoFMS4wLjASPgoKcmVmZXJlbmNlcxgFIAMoCzIdLm51bWluYW5jZS50eXBlcy52MS5SZWZlcmVuY2VCC5K1GAcKBTEuMC4wKpsFChJFeGVjdXRpb25FdmVudFR5cGUSJAogRVhFQ1VUSU9OX0VWRU5UX1RZUEVfVU5TUEVDSUZJRUQQABIqCiZFWEVDVVRJT05fRVZFTlRfVFlQRV9FWEVDVVRJT05fU1RBUlRFRBABEiYKIkVYRUNVVElPTl9FVkVOVF9UWVBFX1BMQU5fQUNDRVBURUQQAhIkCiBFWEVDVVRJT05fRVZFTlRfVFlQRV9UT09MX0lOVEVOVBADEiUKIUVYRUNVVElPTl9FVkVOVF9UWVBFX1RPT0xfQUxMT1dFRBAEEiQKIEVYRUNVVElPTl9FVkVOVF9UWVBFX1RPT0xfREVOSUVEEAUSJAogRVhFQ1VUSU9OX0VWRU5UX1RZUEVfVE9PTF9SRVNVTFQQBhIlCiFFWEVDVVRJT05fRVZFTlRfVFlQRV9GSUxFX0NIQU5HRUQQBxIpCiVFWEVDVVRJT05fRVZFTlRfVFlQRV9DT01NQU5EX0VYRUNVVEVEEAgSKAokRVhFQ1VUSU9OX0VWRU5UX1RZUEVfTkVUV09SS19SRVFVRVNUEAkSKwonRVhFQ1VUSU9OX0VWRU5UX1RZUEVfQ1JFREVOVElBTF9SRVFVRVNUEAoSJAogRVhFQ1VUSU9OX0VWRU5UX1RZUEVfVEVTVF9SRVNVTFQQCxIjCh9FWEVDVVRJT05fRVZFTlRfVFlQRV9DSEVDS1BPSU5UEAwSJQohRVhFQ1VUSU9OX0VWRU5UX1RZUEVfQlVER0VUX0VWRU5UEA0SLAooRVhFQ1VUSU9OX0VWRU5UX1RZUEVfRVhFQ1VUSU9OX0NPTVBMRVRFRBAOEikKJUVYRUNVVElPTl9FVkVOVF9UWVBFX0VYRUNVVElPTl9GQUlMRUQQDzKnBQoVQXV0aG9yRXhlY3V0b3JTZXJ2aWNlEtgBCgxDYXBhYmlsaXRpZXMSNi5udW1pbmFuY2UucG9ydHMuYXV0aG9yZXhlY3V0b3IudjEuQ2FwYWJpbGl0aWVzUmVxdWVzdBo3Lm51bWluYW5jZS5wb3J0cy5hdXRob3JleGVjdXRvci52MS5DYXBhYmlsaXRpZXNSZXNwb25zZSJXirUYUwgBEAEYAUIFMS4wLjBKRFJldHVybnMgdGhlIGV4ZWN1dG9yIGNhcGFiaWxpdGllcyBhbiBhdXRob3IgZXhlY3V0b3IgYWRhcHRlciBjbGFpbXMuEuEBCgdFeGVjdXRlEjEubnVtaW5hbmNlLnBvcnRzLmF1dGhvcmV4ZWN1dG9yLnYxLkV4ZWN1dGVSZXF1ZXN0GjIubnVtaW5hbmNlLnBvcnRzLmF1dGhvcmV4ZWN1dG9yLnYxLkV4ZWN1dGVSZXNwb25zZSJtirUYaQgDEAJCBTEuMC4wSlxSdW5zIGEgY29kaW5nIGhhcm5lc3MgZm9yIG9uZSB3b3JrIHVuaXQgaW5zaWRlIGl0cyBzYW5kYm94IGFuZCBzdHJlYW1zIHRoZSBleGVjdXRpb24gZXZlbnRzLjABEs4BCgRQbGFuEi4ubnVtaW5hbmNlLnBvcnRzLmF1dGhvcmV4ZWN1dG9yLnYxLlBsYW5SZXF1ZXN0Gi8ubnVtaW5hbmNlLnBvcnRzLmF1dGhvcmV4ZWN1dG9yLnYxLlBsYW5SZXNwb25zZSJjirUYXwgBEAJCBTEuMC4wSlJSdW5zIGEgY29kaW5nIGhhcm5lc3MgcmVhZC1vbmx5IHRvIHBsYW4gYSBjaGFuZ2UgYW5kIHN0cmVhbXMgdGhlIGV4ZWN1dGlvbiBldmVudHMuMAFCxgIKJWNvbS5udW1pbmFuY2UucG9ydHMuYXV0aG9yZXhlY3V0b3IudjFCE0F1dGhvcmV4ZWN1dG9yUHJvdG9QAVphZ2l0aHViLmNvbS9udW1pbmFuY2UvbnVtaW5hbmNlLXNwZWMvc2RrL2dvL2dlbi9udW1pbmFuY2UvcG9ydHMvYXV0aG9yZXhlY3V0b3IvdjE7YXV0aG9yZXhlY3V0b3J2MaICA05QQaoCIU51bWluYW5jZS5Qb3J0cy5BdXRob3JleGVjdXRvci5WMcoCIU51bWluYW5jZVxQb3J0c1xBdXRob3JleGVjdXRvclxWMeICLU51bWluYW5jZVxQb3J0c1xBdXRob3JleGVjdXRvclxWMVxHUEJNZXRhZGF0YeoCJE51bWluYW5jZTo6UG9ydHM6OkF1dGhvcmV4ZWN1dG9yOjpWMWIGcHJvdG8z", [file_google_protobuf_timestamp, file_numinance_annotations_v1_annotations, file_numinance_types_v1_types]);

/**
 * CapabilitiesRequest has no fields; the adapter answers for itself.
 *
 * @generated from message numinance.ports.authorexecutor.v1.CapabilitiesRequest
 */
export type CapabilitiesRequest = Message<"numinance.ports.authorexecutor.v1.CapabilitiesRequest"> & {
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.CapabilitiesRequest.
 * Use `create(CapabilitiesRequestSchema)` to create a new message.
 */
export const CapabilitiesRequestSchema: GenMessage<CapabilitiesRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 0);

/**
 * CapabilitiesResponse carries the claimed executor capabilities.
 *
 * @generated from message numinance.ports.authorexecutor.v1.CapabilitiesResponse
 */
export type CapabilitiesResponse = Message<"numinance.ports.authorexecutor.v1.CapabilitiesResponse"> & {
  /**
   * capabilities are the claimed executor capabilities, unique, at most 64.
   *
   * @generated from field: repeated numinance.types.v1.ExecutorCapability capabilities = 1;
   */
  capabilities: ExecutorCapability[];
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.CapabilitiesResponse.
 * Use `create(CapabilitiesResponseSchema)` to create a new message.
 */
export const CapabilitiesResponseSchema: GenMessage<CapabilitiesResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 1);

/**
 * ExecuteRequest carries the request of one run.
 *
 * @generated from message numinance.ports.authorexecutor.v1.ExecuteRequest
 */
export type ExecuteRequest = Message<"numinance.ports.authorexecutor.v1.ExecuteRequest"> & {
  /**
   * request is required.
   *
   * @generated from field: numinance.ports.authorexecutor.v1.ChangeExecutionRequest request = 1;
   */
  request?: ChangeExecutionRequest | undefined;
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.ExecuteRequest.
 * Use `create(ExecuteRequestSchema)` to create a new message.
 */
export const ExecuteRequestSchema: GenMessage<ExecuteRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 2);

/**
 * PlanRequest is the request of the planning run. Its ChangeExecutionRequest has no work unit and no plan yet: work_unit_id is empty, plan_ref names the task (there is no plan to name), allowed_paths is empty and the rest of the read-only rules of Plan hold.
 *
 * @generated from message numinance.ports.authorexecutor.v1.PlanRequest
 */
export type PlanRequest = Message<"numinance.ports.authorexecutor.v1.PlanRequest"> & {
  /**
   * request is required.
   *
   * @generated from field: numinance.ports.authorexecutor.v1.ChangeExecutionRequest request = 1;
   */
  request?: ChangeExecutionRequest | undefined;
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.PlanRequest.
 * Use `create(PlanRequestSchema)` to create a new message.
 */
export const PlanRequestSchema: GenMessage<PlanRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 3);

/**
 * PlanResponse carries one event of the stream of a planning run, the same events as ExecuteResponse.
 *
 * @generated from message numinance.ports.authorexecutor.v1.PlanResponse
 */
export type PlanResponse = Message<"numinance.ports.authorexecutor.v1.PlanResponse"> & {
  /**
   * event is required.
   *
   * @generated from field: numinance.ports.authorexecutor.v1.ExecutionEvent event = 1;
   */
  event?: ExecutionEvent | undefined;
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.PlanResponse.
 * Use `create(PlanResponseSchema)` to create a new message.
 */
export const PlanResponseSchema: GenMessage<PlanResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 4);

/**
 * ExecuteResponse carries one event of the stream.
 *
 * @generated from message numinance.ports.authorexecutor.v1.ExecuteResponse
 */
export type ExecuteResponse = Message<"numinance.ports.authorexecutor.v1.ExecuteResponse"> & {
  /**
   * event is required.
   *
   * @generated from field: numinance.ports.authorexecutor.v1.ExecutionEvent event = 1;
   */
  event?: ExecutionEvent | undefined;
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.ExecuteResponse.
 * Use `create(ExecuteResponseSchema)` to create a new message.
 */
export const ExecuteResponseSchema: GenMessage<ExecuteResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 5);

/**
 * ChangeExecutionRequest is everything one run of one work unit is given. Every field is required unless its comment says it may be unset or empty. The host refuses to dispatch one whose change_id, run_id, grant_id or profile_digest differs from the ActionContext, whose deadline is later than the ActionContext deadline, whose plan_ref.sha256 differs from ActionContext.plan_digest, or whose authority fields (work_unit_id, allowed_paths, denied_paths, allowed_capabilities, network_policy_ref, credential_capabilities, budget, executor_id, model_id, policy_digest) differ from the host's recomputation; each protected_resources entry must be in the host's protected set.
 *
 * @generated from message numinance.ports.authorexecutor.v1.ChangeExecutionRequest
 */
export type ChangeExecutionRequest = Message<"numinance.ports.authorexecutor.v1.ChangeExecutionRequest"> & {
  /**
   * change_id is ^chg_[0-9a-f]{32}$.
   *
   * @generated from field: string change_id = 1;
   */
  changeId: string;

  /**
   * work_unit_id is ^wu_[0-9a-f]{32}$, the work unit of the grant.
   *
   * @generated from field: string work_unit_id = 2;
   */
  workUnitId: string;

  /**
   * run_id is ^run_[0-9a-f]{32}$; one Execute per run_id.
   *
   * @generated from field: string run_id = 3;
   */
  runId: string;

  /**
   * grant_id is ^grant_[0-9a-f]{32}$: the grant the run acts under.
   *
   * @generated from field: string grant_id = 4;
   */
  grantId: string;

  /**
   * repository_digest is the digest of the repository content the worktree was created from.
   *
   * @generated from field: numinance.types.v1.Digest repository_digest = 5;
   */
  repositoryDigest?: Digest | undefined;

  /**
   * worktree_ref is the worktree the sandbox mounts; host-only, never resolvable by the provider.
   *
   * @generated from field: numinance.types.v1.Reference worktree_ref = 6;
   */
  worktreeRef?: Reference | undefined;

  /**
   * task_ref is the work unit's task.
   *
   * @generated from field: numinance.types.v1.Reference task_ref = 7;
   */
  taskRef?: Reference | undefined;

  /**
   * plan_ref is the gated plan the work unit belongs to; its sha256 equals ActionContext.plan_digest.
   *
   * @generated from field: numinance.types.v1.Reference plan_ref = 8;
   */
  planRef?: Reference | undefined;

  /**
   * context_bundle_ref is the ContextBundle of the run; unset when the run has none.
   *
   * @generated from field: numinance.types.v1.Reference context_bundle_ref = 9;
   */
  contextBundleRef?: Reference | undefined;

  /**
   * executor_id is the metadata.name of the ExecutorManifest pinned in the lockfile for this adapter, as the router decided.
   *
   * @generated from field: string executor_id = 10;
   */
  executorId: string;

  /**
   * model_id is one models[].id of that ExecutorManifest, as the router decided.
   *
   * @generated from field: string model_id = 11;
   */
  modelId: string;

  /**
   * allowed_paths are the only worktree paths the run may write; empty for a read-only run. None equals, contains or lies under a denied path or a literal path of the host's full protected set, or the request is refused; a floor pattern that may match below an allowed path is enforced by the host's check of the candidate tree. Under CONTAINER and GVISOR a path missing at the start must lie under another allowed path (sandboxprovider port, Paths); plan validation may check this before Create.
   *
   * @generated from field: repeated string allowed_paths = 12;
   */
  allowedPaths: string[];

  /**
   * denied_paths are worktree paths the run may neither read nor write; may be empty.
   *
   * @generated from field: repeated string denied_paths = 13;
   */
  deniedPaths: string[];

  /**
   * allowed_capabilities are the executor capabilities the run may use, unique, each declared by the pinned ExecutorManifest, at most 64; may be empty.
   *
   * @generated from field: repeated numinance.types.v1.ExecutorCapability allowed_capabilities = 14;
   */
  allowedCapabilities: ExecutorCapability[];

  /**
   * network_policy_ref is the egress allow-list the proxy enforces for the run; host-only, never resolvable by the provider. Link-local ranges and metadata endpoints are denied whatever it lists.
   *
   * @generated from field: numinance.types.v1.Reference network_policy_ref = 15;
   */
  networkPolicyRef?: Reference | undefined;

  /**
   * credential_capabilities are the broker intent names, ^[a-z][a-z0-9._-]{0,127}$, the run may ask a handle for, each in the grant and the manifest's credential_intents; at most 16; may be empty. The broker never returns credential material.
   *
   * @generated from field: repeated string credential_capabilities = 16;
   */
  credentialCapabilities: string[];

  /**
   * budget is the effective budget of the run.
   *
   * @generated from field: numinance.ports.authorexecutor.v1.ExecutionBudget budget = 17;
   */
  budget?: ExecutionBudget | undefined;

  /**
   * deadline is when the run must end, in the future at dispatch; the host cancels the call then.
   *
   * @generated from field: google.protobuf.Timestamp deadline = 18;
   */
  deadline?: Timestamp | undefined;

  /**
   * test_requirements_ref is the tests the work unit must pass; unset when it names none.
   *
   * @generated from field: numinance.types.v1.Reference test_requirements_ref = 19;
   */
  testRequirementsRef?: Reference | undefined;

  /**
   * protected_resources are the worktree paths outside every grant (policy, CI definitions, IAM, workflow files, the kernel), at least one; the harness's copy of the protected set, which the sandbox takes from the kernel and the floors and masks whatever the request says (sandboxprovider port, Paths); its floor patterns are enforced by the host's check of the candidate tree.
   *
   * @generated from field: repeated string protected_resources = 20;
   */
  protectedResources: string[];

  /**
   * environment_digest is the digest of the sandbox environment the run executes in.
   *
   * @generated from field: numinance.types.v1.Digest environment_digest = 21;
   */
  environmentDigest?: Digest | undefined;

  /**
   * policy_digest is the digest of the policy bundle of the decision that allowed the dispatch.
   *
   * @generated from field: numinance.types.v1.Digest policy_digest = 22;
   */
  policyDigest?: Digest | undefined;

  /**
   * profile_digest is the digest of the SDLC profile in force.
   *
   * @generated from field: numinance.types.v1.Digest profile_digest = 23;
   */
  profileDigest?: Digest | undefined;
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.ChangeExecutionRequest.
 * Use `create(ChangeExecutionRequestSchema)` to create a new message.
 */
export const ChangeExecutionRequestSchema: GenMessage<ChangeExecutionRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 6);

/**
 * ExecutionBudget is the effective budget of one run, resolved by the host from the grant (BudgetPolicy names); every field is non-zero. The deadline is the hard bound. The host measures files_changed and lines_changed on its own diff of the worktree and network_destinations at the egress proxy; tokens and tool_calls are measured by the host where inference and tool permission pass through it, and otherwise enforced by the provider and recorded as claims.
 *
 * @generated from message numinance.ports.authorexecutor.v1.ExecutionBudget
 */
export type ExecutionBudget = Message<"numinance.ports.authorexecutor.v1.ExecutionBudget"> & {
  /**
   * tokens is the model tokens the run may consume.
   *
   * @generated from field: uint32 tokens = 1;
   */
  tokens: number;

  /**
   * tool_calls is the tool calls the run may make, at most 300; a stream that still reaches the event cap is budget exhaustion (file comment).
   *
   * @generated from field: uint32 tool_calls = 2;
   */
  toolCalls: number;

  /**
   * files_changed is the worktree files the run may change.
   *
   * @generated from field: uint32 files_changed = 3;
   */
  filesChanged: number;

  /**
   * lines_changed is the worktree lines the run may change.
   *
   * @generated from field: uint32 lines_changed = 4;
   */
  linesChanged: number;

  /**
   * network_destinations is the distinct egress destinations the run may reach.
   *
   * @generated from field: uint32 network_destinations = 5;
   */
  networkDestinations: number;
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.ExecutionBudget.
 * Use `create(ExecutionBudgetSchema)` to create a new message.
 */
export const ExecutionBudgetSchema: GenMessage<ExecutionBudget> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 7);

/**
 * ExecutionEvent is one event of the canonical execution stream; the stream rules are in the file comment.
 *
 * @generated from message numinance.ports.authorexecutor.v1.ExecutionEvent
 */
export type ExecutionEvent = Message<"numinance.ports.authorexecutor.v1.ExecutionEvent"> & {
  /**
   * type is required.
   *
   * @generated from field: numinance.ports.authorexecutor.v1.ExecutionEventType type = 1;
   */
  type: ExecutionEventType;

  /**
   * sequence is the 1-based position of the event in the stream, 1 to 1023.
   *
   * @generated from field: uint64 sequence = 2;
   */
  sequence: bigint;

  /**
   * occurred_at is required: when the provider says the event happened; the host's receive time is authoritative.
   *
   * @generated from field: google.protobuf.Timestamp occurred_at = 3;
   */
  occurredAt?: Timestamp | undefined;

  /**
   * subject is set only for these types, and empty for every other: TOOL_INTENT, TOOL_ALLOWED, TOOL_DENIED and TOOL_RESULT a tool name of the run's tool inventory, ^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$; FILE_CHANGED a path in the file comment's grammar, under allowed_paths and present in the host's diff of the worktree; CREDENTIAL_REQUEST a member of credential_capabilities. Untrusted: the host rejects any other value, scans subjects for issued handles and canaries and redacts them before anything is persisted, and renders them delimited and labelled.
   *
   * @generated from field: string subject = 4;
   */
  subject: string;

  /**
   * references point at the event's content (a diff, tool input and output, a command log, a test report), unique by artifact_ref, at most 16; each equals the request's task_ref, plan_ref, context_bundle_ref or test_requirements_ref, or names an artifact the host stored for this invocation and handed to the provider, and any other is rejected. plugin/v1 has no upload and hands no such reference yet, so until a plugin/v1 minor adds it only request references occur.
   *
   * @generated from field: repeated numinance.types.v1.Reference references = 5;
   */
  references: Reference[];
};

/**
 * Describes the message numinance.ports.authorexecutor.v1.ExecutionEvent.
 * Use `create(ExecutionEventSchema)` to create a new message.
 */
export const ExecutionEventSchema: GenMessage<ExecutionEvent> = /*@__PURE__*/
  messageDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 8);

/**
 * ExecutionEventType is the type of one event of the canonical execution stream. The stream is a provider claim, recorded as execution evidence and an input to the evaluation corpus.
 *
 * @generated from enum numinance.ports.authorexecutor.v1.ExecutionEventType
 */
export enum ExecutionEventType {
  /**
   * EXECUTION_EVENT_TYPE_UNSPECIFIED is the zero value; it is rejected by the host.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * EXECUTION_EVENT_TYPE_EXECUTION_STARTED is the first event of every stream, at sequence 1.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_EXECUTION_STARTED = 1;
   */
  EXECUTION_STARTED = 1,

  /**
   * EXECUTION_EVENT_TYPE_PLAN_ACCEPTED reports that the harness took the plan of plan_ref as its own.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_PLAN_ACCEPTED = 2;
   */
  PLAN_ACCEPTED = 2,

  /**
   * EXECUTION_EVENT_TYPE_TOOL_INTENT reports a tool call the harness is about to make.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_TOOL_INTENT = 3;
   */
  TOOL_INTENT = 3,

  /**
   * EXECUTION_EVENT_TYPE_TOOL_ALLOWED echoes the broker's allow of a tool intent; it authorizes nothing.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_TOOL_ALLOWED = 4;
   */
  TOOL_ALLOWED = 4,

  /**
   * EXECUTION_EVENT_TYPE_TOOL_DENIED echoes the broker's denial of a tool intent.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_TOOL_DENIED = 5;
   */
  TOOL_DENIED = 5,

  /**
   * EXECUTION_EVENT_TYPE_TOOL_RESULT reports the result of a tool call.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_TOOL_RESULT = 6;
   */
  TOOL_RESULT = 6,

  /**
   * EXECUTION_EVENT_TYPE_FILE_CHANGED reports a file of the worktree created, changed or removed; the candidate is the host's own diff of the worktree, never derived from these events.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_FILE_CHANGED = 7;
   */
  FILE_CHANGED = 7,

  /**
   * EXECUTION_EVENT_TYPE_COMMAND_EXECUTED reports a command the harness ran.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_COMMAND_EXECUTED = 8;
   */
  COMMAND_EXECUTED = 8,

  /**
   * EXECUTION_EVENT_TYPE_NETWORK_REQUEST reports a network request the harness made through the egress proxy; the proxy's own record names the destination.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_NETWORK_REQUEST = 9;
   */
  NETWORK_REQUEST = 9,

  /**
   * EXECUTION_EVENT_TYPE_CREDENTIAL_REQUEST reports a credential handle the harness asked the broker for.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_CREDENTIAL_REQUEST = 10;
   */
  CREDENTIAL_REQUEST = 10,

  /**
   * EXECUTION_EVENT_TYPE_TEST_RESULT reports the result of a test run.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_TEST_RESULT = 11;
   */
  TEST_RESULT = 11,

  /**
   * EXECUTION_EVENT_TYPE_CHECKPOINT reports a checkpoint the harness can resume from.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_CHECKPOINT = 12;
   */
  CHECKPOINT = 12,

  /**
   * EXECUTION_EVENT_TYPE_BUDGET_EVENT reports budget consumption or a budget reached.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_BUDGET_EVENT = 13;
   */
  BUDGET_EVENT = 13,

  /**
   * EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED is terminal: the harness finished its work.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED = 14;
   */
  EXECUTION_COMPLETED = 14,

  /**
   * EXECUTION_EVENT_TYPE_EXECUTION_FAILED is terminal: the harness stopped without finishing its work.
   *
   * @generated from enum value: EXECUTION_EVENT_TYPE_EXECUTION_FAILED = 15;
   */
  EXECUTION_FAILED = 15,
}

/**
 * Describes the enum numinance.ports.authorexecutor.v1.ExecutionEventType.
 */
export const ExecutionEventTypeSchema: GenEnum<ExecutionEventType> = /*@__PURE__*/
  enumDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 0);

/**
 * AuthorExecutorService is the authorexecutor port an executor adapter serves.
 *
 * @generated from service numinance.ports.authorexecutor.v1.AuthorExecutorService
 */
export const AuthorExecutorService: GenService<{
  /**
   * Capabilities returns the executor capabilities the adapter claims (ExecutorCapabilities). The host calls it just before Execute, as its own READ invocation with its own decision under the run's grant, and keeps the answer for that run only; the router reads the ExecutorManifest pinned in the lockfile, never this answer. An answer that names a capability the pinned manifest does not declare is a provider fault and the host dispatches no Execute for the run; a request whose allowed_capabilities name one the answer omits is refused.
   *
   * @generated from rpc numinance.ports.authorexecutor.v1.AuthorExecutorService.Capabilities
   */
  capabilities: {
    methodKind: "unary";
    input: typeof CapabilitiesRequestSchema;
    output: typeof CapabilitiesResponseSchema;
  },
  /**
   * Execute runs the harness for one run of one work unit and streams its ExecutionEvents. MEDIUM: the harness writes only inside the disposable work-unit sandbox, and its effects outside it pass the egress proxy and the broker under the run's grant; the decision for a dispatch takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. Not idempotent: a harness is not deterministic, so the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. No dry run: the plan was gated before Execute (PlanGate), the sandbox is discarded unless the candidate is assembled, and running the harness is the only way to learn what it would change.
   *
   * @generated from rpc numinance.ports.authorexecutor.v1.AuthorExecutorService.Execute
   */
  execute: {
    methodKind: "server_streaming";
    input: typeof ExecuteRequestSchema;
    output: typeof ExecuteResponseSchema;
  },
  /**
   * Plan runs the harness as the planning run of a Change: read only, in state Classified, before any plan exists, and streams the same ExecutionEvents as Execute, in PlanResponse. READ: the run writes nothing the Change keeps. The host runs it with a read-only worktree, allowed_paths empty, no credential_capabilities and no capability that writes, and refuses any request that breaks one of these before it dispatches; it never assembles a candidate from a Plan run; a FILE_CHANGED event of a Plan run breaks the stream. The plan it produces is a document the host stores as an artifact of the run, which the Engine parses as untrusted input and freezes as the plan digest before any write grant (and so any Execute) can be issued. The decision for a dispatch takes the higher of READ and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED, exactly as for Execute, so a planner that reaches a model through the egress proxy is decided at that risk. It is a verb of its own, and not an Execute without a work unit, because the policy decision point derives side effect from the verb alone and Execute is MEDIUM and bound to a gated plan (the plan floor), which a planning run precedes. Not idempotent and no dry run, for the reasons of Execute: a harness is not deterministic, the host writes the dispatch of a run_id durably before dispatch, never dispatches a run_id twice, and a retry is a new run. Plan declares no evidence types, as Execute declares none: the host records the stream of the run and the document it stores as its own evidence, and a provider claim is never a receipt. Plan is a capability, not a Core verb: an adapter that serves it claims EXECUTOR_CAPABILITY_STRUCTURED_PLAN in its Capabilities answer (negotiated through that verb, as the host does just before a run), and the host dispatches Plan only to an adapter whose answer contains it (a request whose allowed_capabilities name it is refused when the answer omits it, as for any capability). An adapter that does not serve it answers UNIMPLEMENTED even so, which the host treats as an executor that cannot plan: the run fails closed and no write grant is issued. Not yet done: no host code dispatches this RPC. Today a planning run is dispatched through Execute, on the fake executor only (the policy decision is still taken under the READ verb and the grant capability authorexecutor.plan); the Plan dispatch, the host refusal of an adapter that omits STRUCTURED_PLAN and the UNIMPLEMENTED fail-closed defence are owed with the read-only worktree prerequisite (ADR 0046, C4).
   *
   * @generated from rpc numinance.ports.authorexecutor.v1.AuthorExecutorService.Plan
   */
  plan: {
    methodKind: "server_streaming";
    input: typeof PlanRequestSchema;
    output: typeof PlanResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_authorexecutor_v1_authorexecutor, 0);

