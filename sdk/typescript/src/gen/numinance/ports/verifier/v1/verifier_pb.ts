// The verifier port (ADR 0009): one verifier runs its checks on one candidate and reports findings.
// Callers: only the verification context calls this port, for a run whose ActionContext.actor_kind is reviewer-agent, whose actor_principal and run_id each differ from those of every author run of the Change, not only the latest; the host refuses every other actor kind and a call that breaks either inequality. The verifier runs in its own sandbox with read-only access to the candidate, no write scope and no credential capability. The port is never a model tool, and no tool_groups or scopes are declared.
// Sandbox: Run executes candidate code, such as agent-written tests, so the verifier's sandbox has default-deny egress: no egress destination and no credential intent is granted, IMDS and link-local ranges are denied, and the only connection is the one to the host. The host refuses to dispatch Run to a verifier whose manifest or grant names any egress destination or credential intent, and runs it at least at the sandbox floor's minimum profile, recorded in evidence. What the checks need comes with the candidate or the verifier's image. Candidate code: any check that executes candidate code (tests, build scripts, hooks) runs in a nested sandbox under a separate uid and its own namespaces, with no access to the plugin or host channel or any inherited descriptor, no write access to the verifier's binaries, working files or reports, and no ptrace or signals to the verifier's processes; its output is data for that check only and never changes another check's report, the evidence the verifier emits or the RunResponse. The verifier reads that output as untrusted input: size-limited, where output over the limit ends the call FAILED and is never truncated to continue, parsed without entity or include expansion, and a parse failure ends the call FAILED. Where the nested sandbox cannot be built, as under the process profile, the call ends UNAVAILABLE and candidate code never runs unnested. LOW rests on these rules.
// Configuration: config_ref is the only configuration the checks honour. The host takes it from the LanguageToolchain or ArchitecturePack binding and refuses one that names an artifact of the candidate or of the author run. The provider ignores or overrides configuration and suppression directives inside the candidate (tool settings files, inline suppressions, test hooks); where a tool cannot ignore one, the provider reports each such directive as a finding, so an author cannot switch a check off from the candidate.
// Untrusted answers: every finding is a provider claim and untrusted data. The verification context checks the bounds below, stores the findings as an artifact referenced by digest, and the gate reads the Verdict, never raw findings. A message, path or rule_id is rendered delimited and labelled as untrusted wherever it reaches a prompt, a ticket or a PR, never as instructions.
// Payloads: candidate_ref and config_ref are references the provider resolves through HostService.ResolveReference; no request or response carries candidate content (a platform invariant: payloads carry references, never content).
// Operation: Run is one OPERATION call on PluginService.Invoke: RUNNING messages carry status and diagnostics only, and the terminal message carries RunResponse when SUCCEEDED. There are no start, get and cancel verbs because the host holds the stream: at the deadline or on a cancel the host ends the call and records the run as failed. A lost stream is not reattached; the verification context starts a new Run, which is safe because Run changes nothing outside its sandbox.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound, NOT_FOUND for a reference the host cannot resolve, UNAVAILABLE when the checks cannot start, including a config naming a check the provider cannot run and a nested sandbox that cannot be built. A check that crashes, times out or cannot complete ends the call FAILED with no findings, never SUCCEEDED with a partial list, and the layer is inconclusive, never passed. Findings are results, not failures: a run that finds violations SUCCEEDS.
// Evidence: Run declares one record type, VerifierReport (fixed with the verification context before the first spec tag): a provider emits exactly one per call through HostService.EmitEvidence, a compact receipt of what the tool said, never the findings. Its payload is a google.protobuf.Struct of at most 16 KiB with the keys check (the check the config named), exit_code, output_bytes, output_sha256 (the digest of the raw tool output, so the evidence names what the tool said without storing it), findings (the count after merging by fingerprint) and replayed (true when a recorded output stood in for the tool). A call that ends SUCCEEDED without it is PARTIAL, and the verification context treats the layer as inconclusive. The design's evidence_refs are the ActionResult.evidence_ids of the terminal message (HostService.EmitEvidence), so a provider cannot name an artifact it did not emit.
// Bounds: every string is at most 1024 bytes unless stated; output still out of bounds after the normalisation stated on a field is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding), so a provider sends a field added in a later minor only when HandshakeRequest.spec_version is at least that minor or behind a negotiated capability. Printable means Unicode categories L, M, N, P, S and U+0020. A RunResponse is at most 1024 findings of at most about 2.6 KiB each, so it fits one Invoke message of at most 4 MiB. The host rejects a RunResponse that breaks a rule of this file and records the run as failed.
// Every element is since 1.0.0.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/verifier/v1/verifier.proto (package numinance.ports.verifier.v1, syntax proto3)
/* eslint-disable */

import type { GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { FindingSeverity, Reference } from "../../../types/v1/types_pb";
import { file_numinance_types_v1_types } from "../../../types/v1/types_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/verifier/v1/verifier.proto.
 */
export const file_numinance_ports_verifier_v1_verifier: GenFile = /*@__PURE__*/
  fileDesc("CipudW1pbmFuY2UvcG9ydHMvdmVyaWZpZXIvdjEvdmVyaWZpZXIucHJvdG8SG251bWluYW5jZS5wb3J0cy52ZXJpZmllci52MSKPAQoKUnVuUmVxdWVzdBJBCg1jYW5kaWRhdGVfcmVmGAEgASgLMh0ubnVtaW5hbmNlLnR5cGVzLnYxLlJlZmVyZW5jZUILkrUYBwoFMS4wLjASPgoKY29uZmlnX3JlZhgCIAEoCzIdLm51bWluYW5jZS50eXBlcy52MS5SZWZlcmVuY2VCC5K1GAcKBTEuMC4wIpsBCgtSdW5SZXNwb25zZRJACgVpdGVtcxgBIAMoCzIkLm51bWluYW5jZS5wb3J0cy52ZXJpZmllci52MS5GaW5kaW5nQguStRgHCgUxLjAuMBJKCgdzdW1tYXJ5GAIgASgLMiwubnVtaW5hbmNlLnBvcnRzLnZlcmlmaWVyLnYxLkZpbmRpbmdzU3VtbWFyeUILkrUYBwoFMS4wLjAi8QEKB0ZpbmRpbmcSHAoHcnVsZV9pZBgBIAEoCUILkrUYBwoFMS4wLjASQgoIc2V2ZXJpdHkYAiABKA4yIy5udW1pbmFuY2UudHlwZXMudjEuRmluZGluZ1NldmVyaXR5QguStRgHCgUxLjAuMBJECghsb2NhdGlvbhgDIAEoCzIlLm51bWluYW5jZS5wb3J0cy52ZXJpZmllci52MS5Mb2NhdGlvbkILkrUYBwoFMS4wLjASHAoHbWVzc2FnZRgEIAEoCUILkrUYBwoFMS4wLjASIAoLZmluZ2VycHJpbnQYBSABKAlCC5K1GAcKBTEuMC4wImUKCExvY2F0aW9uEhkKBHBhdGgYASABKAlCC5K1GAcKBTEuMC4wEh8KCnN0YXJ0X2xpbmUYAiABKA1CC5K1GAcKBTEuMC4wEh0KCGVuZF9saW5lGAMgASgNQguStRgHCgUxLjAuMCJaCg9GaW5kaW5nc1N1bW1hcnkSRwoGY291bnRzGAEgAygLMioubnVtaW5hbmNlLnBvcnRzLnZlcmlmaWVyLnYxLlNldmVyaXR5Q291bnRCC5K1GAcKBTEuMC4wIm8KDVNldmVyaXR5Q291bnQSQgoIc2V2ZXJpdHkYASABKA4yIy5udW1pbmFuY2UudHlwZXMudjEuRmluZGluZ1NldmVyaXR5QguStRgHCgUxLjAuMBIaCgVjb3VudBgCIAEoBEILkrUYBwoFMS4wLjAy1AEKD1ZlcmlmaWVyU2VydmljZRLAAQoDUnVuEicubnVtaW5hbmNlLnBvcnRzLnZlcmlmaWVyLnYxLlJ1blJlcXVlc3QaKC5udW1pbmFuY2UucG9ydHMudmVyaWZpZXIudjEuUnVuUmVzcG9uc2UiZoq1GGIIAhACGAEyDlZlcmlmaWVyUmVwb3J0QgUxLjAuMEpDUnVucyBvbmUgdmVyaWZpZXIncyBjaGVja3Mgb24gYSBjYW5kaWRhdGUgYW5kIHJldHVybnMgaXRzIGZpbmRpbmdzLkKWAgofY29tLm51bWluYW5jZS5wb3J0cy52ZXJpZmllci52MUINVmVyaWZpZXJQcm90b1ABWlVnaXRodWIuY29tL251bWluYW5jZS9udW1pbmFuY2Utc3BlYy9zZGsvZ28vZ2VuL251bWluYW5jZS9wb3J0cy92ZXJpZmllci92MTt2ZXJpZmllcnYxogIDTlBWqgIbTnVtaW5hbmNlLlBvcnRzLlZlcmlmaWVyLlYxygIbTnVtaW5hbmNlXFBvcnRzXFZlcmlmaWVyXFYx4gInTnVtaW5hbmNlXFBvcnRzXFZlcmlmaWVyXFYxXEdQQk1ldGFkYXRh6gIeTnVtaW5hbmNlOjpQb3J0czo6VmVyaWZpZXI6OlYxYgZwcm90bzM", [file_numinance_annotations_v1_annotations, file_numinance_types_v1_types]);

/**
 * RunRequest names the candidate and the verifier configuration.
 *
 * @generated from message numinance.ports.verifier.v1.RunRequest
 */
export type RunRequest = Message<"numinance.ports.verifier.v1.RunRequest"> & {
  /**
   * The candidate tree to check, read-only; required.
   *
   * @generated from field: numinance.types.v1.Reference candidate_ref = 1;
   */
  candidateRef?: Reference | undefined;

  /**
   * The verifier configuration the LanguageToolchain or ArchitecturePack binding resolved; required, and the only configuration the checks honour.
   *
   * @generated from field: numinance.types.v1.Reference config_ref = 2;
   */
  configRef?: Reference | undefined;
};

/**
 * Describes the message numinance.ports.verifier.v1.RunRequest.
 * Use `create(RunRequestSchema)` to create a new message.
 */
export const RunRequestSchema: GenMessage<RunRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_verifier_v1_verifier, 0);

/**
 * RunResponse holds the findings: the findings of one run.
 *
 * @generated from message numinance.ports.verifier.v1.RunResponse
 */
export type RunResponse = Message<"numinance.ports.verifier.v1.RunResponse"> & {
  /**
   * Unique by fingerprint and ordered by severity, most severe first. Findings of one fingerprint are merged into one item before counting. The run's total is the sum of the summary counts; items holds every finding when the total is at most 1024, else the 1024 most severe, so len(items) is min(total, 1024).
   *
   * @generated from field: repeated numinance.ports.verifier.v1.Finding items = 1;
   */
  items: Finding[];

  /**
   * Counts of every finding of the run after merging by fingerprint; required, also when there are none.
   *
   * @generated from field: numinance.ports.verifier.v1.FindingsSummary summary = 2;
   */
  summary?: FindingsSummary | undefined;
};

/**
 * Describes the message numinance.ports.verifier.v1.RunResponse.
 * Use `create(RunResponseSchema)` to create a new message.
 */
export const RunResponseSchema: GenMessage<RunResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_verifier_v1_verifier, 1);

/**
 * Finding is one violation a check reported.
 *
 * @generated from message numinance.ports.verifier.v1.Finding
 */
export type Finding = Message<"numinance.ports.verifier.v1.Finding"> & {
  /**
   * Required: the rule that produced the finding, matching ^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$; an ArchitecturePack binds a convention to it.
   *
   * @generated from field: string rule_id = 1;
   */
  ruleId: string;

  /**
   * Required: the severity of the finding; what it blocks is policy.
   *
   * @generated from field: numinance.types.v1.FindingSeverity severity = 2;
   */
  severity: FindingSeverity;

  /**
   * Where in the candidate the finding is; unset for a finding about the candidate as a whole.
   *
   * @generated from field: numinance.ports.verifier.v1.Location location = 3;
   */
  location?: Location | undefined;

  /**
   * Required: what the check reported, 1 to 1024 bytes of printable characters, U+0009 and U+000A; untrusted. The provider replaces any other character with U+FFFD and shortens a longer report at a character boundary; the full tool output belongs in evidence, not here.
   *
   * @generated from field: string message = 4;
   */
  message: string;

  /**
   * Required: a stable identity of the finding across runs of the same check on related candidates, 1 to 256 bytes of printable ASCII without U+0020; the verification context uses it to compare runs, never to suppress a finding.
   *
   * @generated from field: string fingerprint = 5;
   */
  fingerprint: string;
};

/**
 * Describes the message numinance.ports.verifier.v1.Finding.
 * Use `create(FindingSchema)` to create a new message.
 */
export const FindingSchema: GenMessage<Finding> = /*@__PURE__*/
  messageDesc(file_numinance_ports_verifier_v1_verifier, 2);

/**
 * Location is a position in the candidate tree.
 *
 * @generated from message numinance.ports.verifier.v1.Location
 */
export type Location = Message<"numinance.ports.verifier.v1.Location"> & {
  /**
   * Required: a path relative to the candidate root, '/'-separated, printable, with no leading '/', no '\' and no empty, '.' or '..' segment.
   *
   * @generated from field: string path = 1;
   */
  path: string;

  /**
   * First line, 1-based; 0 when the finding is about the whole file.
   *
   * @generated from field: uint32 start_line = 2;
   */
  startLine: number;

  /**
   * Last line, at least start_line; 0 exactly when start_line is 0.
   *
   * @generated from field: uint32 end_line = 3;
   */
  endLine: number;
};

/**
 * Describes the message numinance.ports.verifier.v1.Location.
 * Use `create(LocationSchema)` to create a new message.
 */
export const LocationSchema: GenMessage<Location> = /*@__PURE__*/
  messageDesc(file_numinance_ports_verifier_v1_verifier, 3);

/**
 * FindingsSummary counts the findings of one run by severity.
 *
 * @generated from message numinance.ports.verifier.v1.FindingsSummary
 */
export type FindingsSummary = Message<"numinance.ports.verifier.v1.FindingsSummary"> & {
  /**
   * One entry per severity with at least one finding, unique by severity; a severity with none has no entry. Each count is at least the number of listed items of that severity, and equals it for every severity above the least severe one listed; the number of items equals min(sum of counts, 1024); the host rejects a response that breaks any of these three rules.
   *
   * @generated from field: repeated numinance.ports.verifier.v1.SeverityCount counts = 1;
   */
  counts: SeverityCount[];
};

/**
 * Describes the message numinance.ports.verifier.v1.FindingsSummary.
 * Use `create(FindingsSummarySchema)` to create a new message.
 */
export const FindingsSummarySchema: GenMessage<FindingsSummary> = /*@__PURE__*/
  messageDesc(file_numinance_ports_verifier_v1_verifier, 4);

/**
 * SeverityCount is the number of findings of one severity.
 *
 * @generated from message numinance.ports.verifier.v1.SeverityCount
 */
export type SeverityCount = Message<"numinance.ports.verifier.v1.SeverityCount"> & {
  /**
   * Required: the severity counted.
   *
   * @generated from field: numinance.types.v1.FindingSeverity severity = 1;
   */
  severity: FindingSeverity;

  /**
   * Number of findings of that severity, at least 1 and at most 2^53-1.
   *
   * @generated from field: uint64 count = 2;
   */
  count: bigint;
};

/**
 * Describes the message numinance.ports.verifier.v1.SeverityCount.
 * Use `create(SeverityCountSchema)` to create a new message.
 */
export const SeverityCountSchema: GenMessage<SeverityCount> = /*@__PURE__*/
  messageDesc(file_numinance_ports_verifier_v1_verifier, 5);

/**
 * VerifierService is the verifier port a verifier provider serves.
 *
 * @generated from service numinance.ports.verifier.v1.VerifierService
 */
export const VerifierService: GenService<{
  /**
   * Run runs the verifier's checks, as config_ref configures them, on the candidate of candidate_ref and returns the findings. LOW, not READ: it executes the checks, and through them candidate code such as tests, inside the verifier's own disposable sandbox with default-deny egress; it changes nothing outside that sandbox and the candidate is read-only. Idempotent, and the manifest declares IDEMPOTENCY_NATURAL: a replay has no effect outside the sandbox, though a flaky check may report other findings, so the host never returns a stored result for a new Run and the verification context records each run. No dry run: running the checks is the only way to learn the findings, and a dry run would be the run itself.
   *
   * @generated from rpc numinance.ports.verifier.v1.VerifierService.Run
   */
  run: {
    methodKind: "unary";
    input: typeof RunRequestSchema;
    output: typeof RunResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_verifier_v1_verifier, 0);

