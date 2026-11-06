// The scm port: reads a source repository and its pull requests, and opens, updates, comments on and merges pull requests. Any code host that has repositories, branches, pull requests, status checks and branch protection can implement every core verb (GitHub, GitLab, Bitbucket, Gitea); a vendor feature beyond them is a capability `scm.<feature>` negotiated at handshake, added in a later minor; there is no VendorCall in 1.0.0, because the host refuses a provider that does not declare every side-effecting RPC of its port, and an escape hatch no provider implements is a PRIVILEGED capability that does nothing.
// Callers: the release context calls the write verbs, the profile and policy side read the base revision at intake through GetRepository, and the knowledge side may read files and trees. The host decides every call, on the exact request bytes, against the grant of the identity named in ActionContext: OpenPullRequest, UpdatePullRequest, CommentOnPullRequest and CreateBranch under the author-agent identity of the Change, MergePullRequest under the release-pipeline identity only. A call that comes from an identity that is neither, and a MergePullRequest from the author of the Change (the same principal or the same run_id as any author run), is refused by the policy floors before Invoke; the provider never decides who may merge. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: every request names its repository in `repository`, which the Engine marks as the resource of the call (RFC-0002): a call on a repository outside the acting grant's resource scope is denied before dispatch. A provider holds no credential of its own: it asks the host for a credential handle for the intent its manifest declares and sends the handle to the egress proxy, never a raw token. A provider never reports an effect it did not make, and reconciles a retry before it acts: the host's idempotency key names the operation, and a replay answers the first result (an open pull request for the same head and base is returned, not opened again; a merge already done is answered as done).
// Untrusted answers: every title, body, branch name, label, URL, check name, check output, commit message and file content that the host holds comes from the code host and is untrusted data, whoever wrote it on the host. The release context size-limits it, validates it against this file, stores it by digest and labels and delimits it as untrusted wherever it reaches a prompt, a ticket or a pull request, never as instructions. Nothing a provider answers changes policy, a grant, a tool list or an approval. A merge decision never rests on a field of a response that a pull request author controls (the text of a body or a comment).
// Protected paths: branch protection is the second line behind the mount. GetBranchProtection reports what the host enforces so that the release context can refuse to merge a pull request whose base branch has no required review or no required check; the provider reports the host's state and never changes it, and no verb of this port edits protection, a workflow file or a ruleset.
// Payloads: requests carry ids, names and bounded text; files are returned inline only up to max_bytes (a repository file is not a Reference because the scm host is the store). No verb returns a diff.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; NOT_FOUND for a repository, revision, branch, file or pull request the host hides or lacks; PERMISSION_DENIED when the host refuses the credential openly; ALREADY_EXISTS for a CreateBranch whose name is taken at another revision; FAILED_PRECONDITION for a pull request that is not mergeable, not open or whose head moved from head_revision, and for a merge the base branch's protection refuses; RESOURCE_EXHAUSTED when the host rate-limits the call; UNAVAILABLE for a host that cannot be reached or a transient failure the caller may retry. A provider never invents a result: an outage is an error, never an empty answer.
// Evidence: OpenPullRequest declares one record type, pr-ref: a provider emits exactly one per call that opened or found the pull request, through HostService.EmitEvidence, a compact receipt of the pull request's reference (repository, number, head and base revisions, never the title or body). A call that ends SUCCEEDED without it is PARTIAL. The other verbs declare no evidence in 1.0.0: the merge's record is the host's capability record, and the release context writes its own facts as events.
// Bounds: every string is at most 1024 bytes unless stated; over-limit input is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding), so a provider sends a field added in a later minor only when HandshakeRequest.spec_version is at least that minor or behind a negotiated capability. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Formats: repository is the repository's full name `<owner>/<name>`, matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,99}/[A-Za-z0-9._-]{1,100}$ with no '.' or '..' name segment. A revision is a full commit id: 40 or 64 lowercase hexadecimal characters; a branch name or a tag never stands for a revision, because it moves. A branch name is valid under git check-ref-format --branch, 1 to 255 bytes, printable ASCII. A path is relative to the repository root, '/'-separated, printable, with no leading '/', no '\' and no empty, '.' or '..' segment.
// Every element is since 1.0.0.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/scm/v1/scm.proto (package numinance.ports.scm.v1, syntax proto3)
/* eslint-disable */

import type { GenEnum, GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { enumDesc, fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { file_google_protobuf_timestamp } from "@bufbuild/protobuf/wkt";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/scm/v1/scm.proto.
 */
export const file_numinance_ports_scm_v1_scm: GenFile = /*@__PURE__*/
  fileDesc("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Lm51bWluYW5jZS5wb3J0cy5zY20udjEuUHVsbFJlcXVlc3RCC5K1GAcKBTEuMC4wIngKG0NvbW1lbnRPblB1bGxSZXF1ZXN0UmVxdWVzdBIhCgpyZXBvc2l0b3J5GAEgASgJQg2StRgJCgUxLjAuMDABEhsKBm51bWJlchgCIAEoBEILkrUYBwoFMS4wLjASGQoEYm9keRgDIAEoCUILkrUYBwoFMS4wLjAiPwocQ29tbWVudE9uUHVsbFJlcXVlc3RSZXNwb25zZRIfCgpjb21tZW50X2lkGAEgASgJQguStRgHCgUxLjAuMCJdChtHZXRQdWxsUmVxdWVzdFN0YXR1c1JlcXVlc3QSIQoKcmVwb3NpdG9yeRgBIAEoCUINkrUYCQoFMS4wLjAwARIbCgZudW1iZXIYAiABKARCC5K1GAcKBTEuMC4wIm8KCENoZWNrUnVuEhkKBG5hbWUYASABKAlCC5K1GAcKBTEuMC4wEkgKCmNvbmNsdXNpb24YAiABKA4yJy5udW1pbmFuY2UucG9ydHMuc2NtLnYxLkNoZWNrQ29uY2x1c2lvbkILkrUYBwoFMS4wLjAi2wIKHEdldFB1bGxSZXF1ZXN0U3RhdHVzUmVzcG9uc2USRgoMcHVsbF9yZXF1ZXN0GAEgASgLMiMubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5QdWxsUmVxdWVzdEILkrUYBwoFMS4wLjASRwoMbWVyZ2VhYmlsaXR5GAIgASgOMiQubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5NZXJnZWFiaWxpdHlCC5K1GAcKBTEuMC4wEj0KBmNoZWNrcxgDIAMoCzIgLm51bWluYW5jZS5wb3J0cy5zY20udjEuQ2hlY2tSdW5CC5K1GAcKBTEuMC4wEh4KCWFwcHJvdmFscxgEIAEoDUILkrUYBwoFMS4wLjASJgoRY2hhbmdlc19yZXF1ZXN0ZWQYBSABKAhCC5K1GAcKBTEuMC4wEiMKDm1lcmdlX3JldmlzaW9uGAYgASgJQguStRgHCgUxLjAuMCKxAQoXTWVyZ2VQdWxsUmVxdWVzdFJlcXVlc3QSIQoKcmVwb3NpdG9yeRgBIAEoCUINkrUYCQoFMS4wLjAwARIbCgZudW1iZXIYAiABKARCC5K1GAcKBTEuMC4wEisKFmV4cGVjdGVkX2hlYWRfcmV2aXNpb24YAyABKAlCC5K1GAcKBTEuMC4wEikKFGV4cGVjdGVkX2Jhc2VfYnJhbmNoGAQgASgJQguStRgHCgUxLjAuMCLEAQoYTWVyZ2VQdWxsUmVxdWVzdFJlc3BvbnNlEiMKDm1lcmdlX3JldmlzaW9uGAEgASgJQguStRgHCgUxLjAuMBIjCg5hbHJlYWR5X21lcmdlZBgCIAEoCEILkrUYBwoFMS4wLjASOgoJbWVyZ2VkX2F0GAMgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEILkrUYBwoFMS4wLjASIgoNaGVhZF9yZXZpc2lvbhgEIAEoCUILkrUYBwoFMS4wLjAidAoRU2VhcmNoQ29kZVJlcXVlc3QSIQoKcmVwb3NpdG9yeRgBIAEoCUINkrUYCQoFMS4wLjAwARIaCgVxdWVyeRgCIAEoCUILkrUYBwoFMS4wLjASIAoLbWF4X3Jlc3VsdHMYAyABKA1CC5K1GAcKBTEuMC4wIjAKElNlYXJjaENvZGVSZXNwb25zZRIaCgVwYXRocxgBIAMoCUILkrUYBwoFMS4wLjAqkQEKEFB1bGxSZXF1ZXN0U3RhdGUSIgoeUFVMTF9SRVFVRVNUX1NUQVRFX1VOU1BFQ0lGSUVEEAASGwoXUFVMTF9SRVFVRVNUX1NUQVRFX09QRU4QARIdChlQVUxMX1JFUVVFU1RfU1RBVEVfQ0xPU0VEEAISHQoZUFVMTF9SRVFVRVNUX1NUQVRFX01FUkdFRBADKoABCgxNZXJnZWFiaWxpdHkSHAoYTUVSR0VBQklMSVRZX1VOU1BFQ0lGSUVEEAASGgoWTUVSR0VBQklMSVRZX01FUkdFQUJMRRABEhwKGE1FUkdFQUJJTElUWV9DT05GTElDVElORxACEhgKFE1FUkdFQUJJTElUWV9VTktOT1dOEAMqqwEKD0NoZWNrQ29uY2x1c2lvbhIgChxDSEVDS19DT05DTFVTSU9OX1VOU1BFQ0lGSUVEEAASHAoYQ0hFQ0tfQ09OQ0xVU0lPTl9QRU5ESU5HEAESHAoYQ0hFQ0tfQ09OQ0xVU0lPTl9TVUNDRVNTEAISHAoYQ0hFQ0tfQ09OQ0xVU0lPTl9GQUlMVVJFEAMSHAoYQ0hFQ0tfQ09OQ0xVU0lPTl9ORVVUUkFMEAQytBIKClNjbVNlcnZpY2USyAEKDUdldFJlcG9zaXRvcnkSLC5udW1pbmFuY2UucG9ydHMuc2NtLnYxLkdldFJlcG9zaXRvcnlSZXF1ZXN0Gi0ubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5HZXRSZXBvc2l0b3J5UmVzcG9uc2UiWoq1GFYIARABGAFCBTEuMC4wSkdSZXR1cm5zIHRoZSBkZWZhdWx0IGJyYW5jaCBvZiBhIHJlcG9zaXRvcnkgYW5kIHRoZSBjb21taXQgaXQgcG9pbnRzIHRvLhKsAQoHR2V0VHJlZRImLm51bWluYW5jZS5wb3J0cy5zY20udjEuR2V0VHJlZVJlcXVlc3QaJy5udW1pbmFuY2UucG9ydHMuc2NtLnYxLkdldFRyZWVSZXNwb25zZSJQirUYTAgBEAEYAUIFMS4wLjBKPUxpc3RzIHRoZSBlbnRyaWVzIG9mIGEgZGlyZWN0b3J5IG9mIGEgcmVwb3NpdG9yeSBhdCBhIGNvbW1pdC4SqwEKB0dldEZpbGUSJi5udW1pbmFuY2UucG9ydHMuc2NtLnYxLkdldEZpbGVSZXF1ZXN0GicubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5HZXRGaWxlUmVzcG9uc2UiT4q1GEsIARABGAFCBTEuMC4wSjxSZXR1cm5zIHRoZSBjb250ZW50IG9mIG9uZSBmaWxlIG9mIGEgcmVwb3NpdG9yeSBhdCBhIGNvbW1pdC4SoQEKDExpc3RCcmFuY2hlcxIrLm51bWluYW5jZS5wb3J0cy5zY20udjEuTGlzdEJyYW5jaGVzUmVxdWVzdBosLm51bWluYW5jZS5wb3J0cy5zY20udjEuTGlzdEJyYW5jaGVzUmVzcG9uc2UiNoq1GDIIARABGAFCBTEuMC4wSiNMaXN0cyB0aGUgYnJhbmNoZXMgb2YgYSByZXBvc2l0b3J5LhKxAQoMQ3JlYXRlQnJhbmNoEisubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5DcmVhdGVCcmFuY2hSZXF1ZXN0GiwubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5DcmVhdGVCcmFuY2hSZXNwb25zZSJGirUYQggCEAEYAUIFMS4wLjBKM0NyZWF0ZXMgYSBicmFuY2ggb2YgYSByZXBvc2l0b3J5IGF0IGEgZ2l2ZW4gY29tbWl0LhLNAQoTR2V0QnJhbmNoUHJvdGVjdGlvbhIyLm51bWluYW5jZS5wb3J0cy5zY20udjEuR2V0QnJhbmNoUHJvdGVjdGlvblJlcXVlc3QaMy5udW1pbmFuY2UucG9ydHMuc2NtLnYxLkdldEJyYW5jaFByb3RlY3Rpb25SZXNwb25zZSJNirUYSQgBEAEYAUIFMS4wLjBKOlJlcG9ydHMgdGhlIGJyYW5jaCBwcm90ZWN0aW9uIG9mIGEgYnJhbmNoIG9mIGEgcmVwb3NpdG9yeS4S5AEKD09wZW5QdWxsUmVxdWVzdBIuLm51bWluYW5jZS5wb3J0cy5zY20udjEuT3BlblB1bGxSZXF1ZXN0UmVxdWVzdBovLm51bWluYW5jZS5wb3J0cy5zY20udjEuT3BlblB1bGxSZXF1ZXN0UmVzcG9uc2UicIq1GGwIAxABGAEgASocU2NtU2VydmljZS5VcGRhdGVQdWxsUmVxdWVzdDIGcHItcmVmQgUxLjAuMEo1T3BlbnMgYSBwdWxsIHJlcXVlc3QgZm9yIGEgdmVyaWZpZWQgY2FuZGlkYXRlIGJyYW5jaC4SxQEKEVVwZGF0ZVB1bGxSZXF1ZXN0EjAubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5VcGRhdGVQdWxsUmVxdWVzdFJlcXVlc3QaMS5udW1pbmFuY2UucG9ydHMuc2NtLnYxLlVwZGF0ZVB1bGxSZXF1ZXN0UmVzcG9uc2UiS4q1GEcIAhABGAFCBTEuMC4wSjhFZGl0cyB0aGUgdGl0bGUgb3IgYm9keSBvZiBhIHB1bGwgcmVxdWVzdCwgb3IgY2xvc2VzIGl0LhK5AQoUQ29tbWVudE9uUHVsbFJlcXVlc3QSMy5udW1pbmFuY2UucG9ydHMuc2NtLnYxLkNvbW1lbnRPblB1bGxSZXF1ZXN0UmVxdWVzdBo0Lm51bWluYW5jZS5wb3J0cy5zY20udjEuQ29tbWVudE9uUHVsbFJlcXVlc3RSZXNwb25zZSI2irUYMggCEAEYAUIFMS4wLjBKI0FkZHMgb25lIGNvbW1lbnQgdG8gYSBwdWxsIHJlcXVlc3QuEs4BChRHZXRQdWxsUmVxdWVzdFN0YXR1cxIzLm51bWluYW5jZS5wb3J0cy5zY20udjEuR2V0UHVsbFJlcXVlc3RTdGF0dXNSZXF1ZXN0GjQubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5HZXRQdWxsUmVxdWVzdFN0YXR1c1Jlc3BvbnNlIkuKtRhHCAEQARgBQgUxLjAuMEo4UmV0dXJucyB0aGUgc3RhdGUsIGNoZWNrcyBhbmQgcmV2aWV3cyBvZiBhIHB1bGwgcmVxdWVzdC4S4QEKEE1lcmdlUHVsbFJlcXVlc3QSLy5udW1pbmFuY2UucG9ydHMuc2NtLnYxLk1lcmdlUHVsbFJlcXVlc3RSZXF1ZXN0GjAubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5NZXJnZVB1bGxSZXF1ZXN0UmVzcG9uc2Uiaoq1GGYIBRABGAEgASofU2NtU2VydmljZS5Db21tZW50T25QdWxsUmVxdWVzdEIFMS4wLjBKNE1lcmdlcyBhIHZlcmlmaWVkIHB1bGwgcmVxdWVzdCBpbnRvIGl0cyBiYXNlIGJyYW5jaC4StQEKClNlYXJjaENvZGUSKS5udW1pbmFuY2UucG9ydHMuc2NtLnYxLlNlYXJjaENvZGVSZXF1ZXN0GioubnVtaW5hbmNlLnBvcnRzLnNjbS52MS5TZWFyY2hDb2RlUmVzcG9uc2UiUIq1GEwIARABGAFCBTEuMC4wSj1TZWFyY2hlcyB0aGUgY29kZSBvZiBhIHJlcG9zaXRvcnkgYW5kIHJldHVybnMgbWF0Y2hpbmcgcGF0aHMuQu4BChpjb20ubnVtaW5hbmNlLnBvcnRzLnNjbS52MUIIU2NtUHJvdG9QAVpLZ2l0aHViLmNvbS9udW1pbmFuY2UvbnVtaW5hbmNlLXNwZWMvc2RrL2dvL2dlbi9udW1pbmFuY2UvcG9ydHMvc2NtL3YxO3NjbXYxogIDTlBTqgIWTnVtaW5hbmNlLlBvcnRzLlNjbS5WMcoCFk51bWluYW5jZVxQb3J0c1xTY21cVjHiAiJOdW1pbmFuY2VcUG9ydHNcU2NtXFYxXEdQQk1ldGFkYXRh6gIZTnVtaW5hbmNlOjpQb3J0czo6U2NtOjpWMWIGcHJvdG8z", [file_google_protobuf_timestamp, file_numinance_annotations_v1_annotations]);

/**
 * GetRepositoryRequest names the repository.
 *
 * @generated from message numinance.ports.scm.v1.GetRepositoryRequest
 */
export type GetRepositoryRequest = Message<"numinance.ports.scm.v1.GetRepositoryRequest"> & {
  /**
   * Required: the repository's full name, in the grammar of the file comment (Formats). Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;
};

/**
 * Describes the message numinance.ports.scm.v1.GetRepositoryRequest.
 * Use `create(GetRepositoryRequestSchema)` to create a new message.
 */
export const GetRepositoryRequestSchema: GenMessage<GetRepositoryRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 0);

/**
 * GetRepositoryResponse is the host's answer; every field is a claim.
 *
 * @generated from message numinance.ports.scm.v1.GetRepositoryResponse
 */
export type GetRepositoryResponse = Message<"numinance.ports.scm.v1.GetRepositoryResponse"> & {
  /**
   * The repository's default branch name.
   *
   * @generated from field: string default_branch = 1;
   */
  defaultBranch: string;

  /**
   * The full commit id the default branch points to at the time of the call.
   *
   * @generated from field: string default_branch_head = 2;
   */
  defaultBranchHead: string;

  /**
   * True when the repository is private to its organisation.
   *
   * @generated from field: bool private = 3;
   */
  private: boolean;

  /**
   * True when the host has archived the repository (read-only).
   *
   * @generated from field: bool archived = 4;
   */
  archived: boolean;
};

/**
 * Describes the message numinance.ports.scm.v1.GetRepositoryResponse.
 * Use `create(GetRepositoryResponseSchema)` to create a new message.
 */
export const GetRepositoryResponseSchema: GenMessage<GetRepositoryResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 1);

/**
 * GetTreeRequest names a directory at a revision.
 *
 * @generated from message numinance.ports.scm.v1.GetTreeRequest
 */
export type GetTreeRequest = Message<"numinance.ports.scm.v1.GetTreeRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: a full commit id.
   *
   * @generated from field: string revision = 2;
   */
  revision: string;

  /**
   * The directory, in the path grammar; empty for the repository root.
   *
   * @generated from field: string path = 3;
   */
  path: string;

  /**
   * True to list the whole subtree.
   *
   * @generated from field: bool recursive = 4;
   */
  recursive: boolean;

  /**
   * Required, 1 to 1024: the most entries to return.
   *
   * @generated from field: uint32 max_entries = 5;
   */
  maxEntries: number;
};

/**
 * Describes the message numinance.ports.scm.v1.GetTreeRequest.
 * Use `create(GetTreeRequestSchema)` to create a new message.
 */
export const GetTreeRequestSchema: GenMessage<GetTreeRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 2);

/**
 * TreeEntry is one entry of a tree.
 *
 * @generated from message numinance.ports.scm.v1.TreeEntry
 */
export type TreeEntry = Message<"numinance.ports.scm.v1.TreeEntry"> & {
  /**
   * The path from the repository root, in the path grammar.
   *
   * @generated from field: string path = 1;
   */
  path: string;

  /**
   * True when the entry is a directory.
   *
   * @generated from field: bool directory = 2;
   */
  directory: boolean;

  /**
   * The object id of the entry on the host: 40 or 64 lowercase hexadecimal characters.
   *
   * @generated from field: string object_id = 3;
   */
  objectId: string;

  /**
   * Size in bytes of a file; 0 for a directory.
   *
   * @generated from field: uint64 size = 4;
   */
  size: bigint;
};

/**
 * Describes the message numinance.ports.scm.v1.TreeEntry.
 * Use `create(TreeEntrySchema)` to create a new message.
 */
export const TreeEntrySchema: GenMessage<TreeEntry> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 3);

/**
 * GetTreeResponse lists a tree.
 *
 * @generated from message numinance.ports.scm.v1.GetTreeResponse
 */
export type GetTreeResponse = Message<"numinance.ports.scm.v1.GetTreeResponse"> & {
  /**
   * At most max_entries entries, ordered by path.
   *
   * @generated from field: repeated numinance.ports.scm.v1.TreeEntry entries = 1;
   */
  entries: TreeEntry[];

  /**
   * True when the host holds more entries than were returned.
   *
   * @generated from field: bool truncated = 2;
   */
  truncated: boolean;
};

/**
 * Describes the message numinance.ports.scm.v1.GetTreeResponse.
 * Use `create(GetTreeResponseSchema)` to create a new message.
 */
export const GetTreeResponseSchema: GenMessage<GetTreeResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 4);

/**
 * GetFileRequest names a file at a revision.
 *
 * @generated from message numinance.ports.scm.v1.GetFileRequest
 */
export type GetFileRequest = Message<"numinance.ports.scm.v1.GetFileRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: a full commit id.
   *
   * @generated from field: string revision = 2;
   */
  revision: string;

  /**
   * Required: the file, in the path grammar.
   *
   * @generated from field: string path = 3;
   */
  path: string;

  /**
   * Required, 1 to 262144: the most content bytes to return.
   *
   * @generated from field: uint32 max_bytes = 4;
   */
  maxBytes: number;
};

/**
 * Describes the message numinance.ports.scm.v1.GetFileRequest.
 * Use `create(GetFileRequestSchema)` to create a new message.
 */
export const GetFileRequestSchema: GenMessage<GetFileRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 5);

/**
 * GetFileResponse is a file's head.
 *
 * @generated from message numinance.ports.scm.v1.GetFileResponse
 */
export type GetFileResponse = Message<"numinance.ports.scm.v1.GetFileResponse"> & {
  /**
   * The object id of the file on the host.
   *
   * @generated from field: string object_id = 1;
   */
  objectId: string;

  /**
   * The size in bytes of the whole file.
   *
   * @generated from field: uint64 size = 2;
   */
  size: bigint;

  /**
   * The first min(size, max_bytes) bytes; untrusted.
   *
   * @generated from field: bytes content = 3;
   */
  content: Uint8Array;

  /**
   * True when size is above max_bytes.
   *
   * @generated from field: bool truncated = 4;
   */
  truncated: boolean;
};

/**
 * Describes the message numinance.ports.scm.v1.GetFileResponse.
 * Use `create(GetFileResponseSchema)` to create a new message.
 */
export const GetFileResponseSchema: GenMessage<GetFileResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 6);

/**
 * ListBranchesRequest asks for one page of branches.
 *
 * @generated from message numinance.ports.scm.v1.ListBranchesRequest
 */
export type ListBranchesRequest = Message<"numinance.ports.scm.v1.ListBranchesRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required, 1 to 256: the most branches to return.
   *
   * @generated from field: uint32 page_size = 2;
   */
  pageSize: number;

  /**
   * The token a previous answer returned, at most 1024 printable bytes; empty for the first page.
   *
   * @generated from field: string page_token = 3;
   */
  pageToken: string;
};

/**
 * Describes the message numinance.ports.scm.v1.ListBranchesRequest.
 * Use `create(ListBranchesRequestSchema)` to create a new message.
 */
export const ListBranchesRequestSchema: GenMessage<ListBranchesRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 7);

/**
 * Branch is a branch and the commit it points to.
 *
 * @generated from message numinance.ports.scm.v1.Branch
 */
export type Branch = Message<"numinance.ports.scm.v1.Branch"> & {
  /**
   * The branch name, in the branch grammar.
   *
   * @generated from field: string name = 1;
   */
  name: string;

  /**
   * The full commit id the branch points to.
   *
   * @generated from field: string head_revision = 2;
   */
  headRevision: string;
};

/**
 * Describes the message numinance.ports.scm.v1.Branch.
 * Use `create(BranchSchema)` to create a new message.
 */
export const BranchSchema: GenMessage<Branch> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 8);

/**
 * ListBranchesResponse is one page of branches.
 *
 * @generated from message numinance.ports.scm.v1.ListBranchesResponse
 */
export type ListBranchesResponse = Message<"numinance.ports.scm.v1.ListBranchesResponse"> & {
  /**
   * At most page_size branches.
   *
   * @generated from field: repeated numinance.ports.scm.v1.Branch branches = 1;
   */
  branches: Branch[];

  /**
   * The token of the next page; empty at the last page. Opaque and untrusted.
   *
   * @generated from field: string next_page_token = 2;
   */
  nextPageToken: string;
};

/**
 * Describes the message numinance.ports.scm.v1.ListBranchesResponse.
 * Use `create(ListBranchesResponseSchema)` to create a new message.
 */
export const ListBranchesResponseSchema: GenMessage<ListBranchesResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 9);

/**
 * CreateBranchRequest names the branch and its start.
 *
 * @generated from message numinance.ports.scm.v1.CreateBranchRequest
 */
export type CreateBranchRequest = Message<"numinance.ports.scm.v1.CreateBranchRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the new branch's name.
   *
   * @generated from field: string name = 2;
   */
  name: string;

  /**
   * Required: the full commit id the branch starts at.
   *
   * @generated from field: string from_revision = 3;
   */
  fromRevision: string;
};

/**
 * Describes the message numinance.ports.scm.v1.CreateBranchRequest.
 * Use `create(CreateBranchRequestSchema)` to create a new message.
 */
export const CreateBranchRequestSchema: GenMessage<CreateBranchRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 10);

/**
 * CreateBranchResponse is the created or existing branch.
 *
 * @generated from message numinance.ports.scm.v1.CreateBranchResponse
 */
export type CreateBranchResponse = Message<"numinance.ports.scm.v1.CreateBranchResponse"> & {
  /**
   * The branch as the host holds it after the call.
   *
   * @generated from field: numinance.ports.scm.v1.Branch branch = 1;
   */
  branch?: Branch | undefined;
};

/**
 * Describes the message numinance.ports.scm.v1.CreateBranchResponse.
 * Use `create(CreateBranchResponseSchema)` to create a new message.
 */
export const CreateBranchResponseSchema: GenMessage<CreateBranchResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 11);

/**
 * GetBranchProtectionRequest names a branch.
 *
 * @generated from message numinance.ports.scm.v1.GetBranchProtectionRequest
 */
export type GetBranchProtectionRequest = Message<"numinance.ports.scm.v1.GetBranchProtectionRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the branch.
   *
   * @generated from field: string branch = 2;
   */
  branch: string;
};

/**
 * Describes the message numinance.ports.scm.v1.GetBranchProtectionRequest.
 * Use `create(GetBranchProtectionRequestSchema)` to create a new message.
 */
export const GetBranchProtectionRequestSchema: GenMessage<GetBranchProtectionRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 12);

/**
 * GetBranchProtectionResponse is the protection in force on a branch; every field is the host's claim.
 *
 * @generated from message numinance.ports.scm.v1.GetBranchProtectionResponse
 */
export type GetBranchProtectionResponse = Message<"numinance.ports.scm.v1.GetBranchProtectionResponse"> & {
  /**
   * True when the host protects the branch at all.
   *
   * @generated from field: bool protected = 1;
   */
  protected: boolean;

  /**
   * The number of approving reviews the host requires before a merge; 0 when none.
   *
   * @generated from field: uint32 required_reviews = 2;
   */
  requiredReviews: number;

  /**
   * The names of the checks that must pass before a merge, at most 64, unique, each at most 256 bytes.
   *
   * @generated from field: repeated string required_checks = 3;
   */
  requiredChecks: string[];

  /**
   * True when the rules bind repository administrators as well.
   *
   * @generated from field: bool enforce_admins = 4;
   */
  enforceAdmins: boolean;

  /**
   * True when the host allows a force push to the branch.
   *
   * @generated from field: bool allow_force_push = 5;
   */
  allowForcePush: boolean;
};

/**
 * Describes the message numinance.ports.scm.v1.GetBranchProtectionResponse.
 * Use `create(GetBranchProtectionResponseSchema)` to create a new message.
 */
export const GetBranchProtectionResponseSchema: GenMessage<GetBranchProtectionResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 13);

/**
 * OpenPullRequestRequest is one pull request to open.
 *
 * @generated from message numinance.ports.scm.v1.OpenPullRequestRequest
 */
export type OpenPullRequestRequest = Message<"numinance.ports.scm.v1.OpenPullRequestRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the branch that holds the candidate.
   *
   * @generated from field: string head_branch = 2;
   */
  headBranch: string;

  /**
   * Required: the full commit id of the candidate; head_branch must point at it.
   *
   * @generated from field: string head_revision = 3;
   */
  headRevision: string;

  /**
   * Required: the branch the pull request merges into.
   *
   * @generated from field: string base_branch = 4;
   */
  baseBranch: string;

  /**
   * Required: 1 to 256 printable bytes. Text the Engine composes from the Change; rendered as data on the host.
   *
   * @generated from field: string title = 5;
   */
  title: string;

  /**
   * At most 16384 bytes of printable characters, U+0009 and U+000A. The Engine composes it from references and labels any untrusted quotation; never an instruction.
   *
   * @generated from field: string body = 6;
   */
  body: string;
};

/**
 * Describes the message numinance.ports.scm.v1.OpenPullRequestRequest.
 * Use `create(OpenPullRequestRequestSchema)` to create a new message.
 */
export const OpenPullRequestRequestSchema: GenMessage<OpenPullRequestRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 14);

/**
 * PullRequest is a pull request as the host reports it.
 *
 * @generated from message numinance.ports.scm.v1.PullRequest
 */
export type PullRequest = Message<"numinance.ports.scm.v1.PullRequest"> & {
  /**
   * The host's number of the pull request, at least 1 and at most 2^53-1.
   *
   * @generated from field: uint64 number = 1;
   */
  number: bigint;

  /**
   * The pull request's state.
   *
   * @generated from field: numinance.ports.scm.v1.PullRequestState state = 2;
   */
  state: PullRequestState;

  /**
   * The branch that holds the candidate.
   *
   * @generated from field: string head_branch = 3;
   */
  headBranch: string;

  /**
   * The full commit id of the head now.
   *
   * @generated from field: string head_revision = 4;
   */
  headRevision: string;

  /**
   * The branch the pull request merges into.
   *
   * @generated from field: string base_branch = 5;
   */
  baseBranch: string;

  /**
   * The address of the pull request on the host: an absolute https URL of at most 1024 bytes, for people; untrusted, and never fetched by the Engine.
   *
   * @generated from field: string url = 6;
   */
  url: string;
};

/**
 * Describes the message numinance.ports.scm.v1.PullRequest.
 * Use `create(PullRequestSchema)` to create a new message.
 */
export const PullRequestSchema: GenMessage<PullRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 15);

/**
 * OpenPullRequestResponse is the opened or found pull request.
 *
 * @generated from message numinance.ports.scm.v1.OpenPullRequestResponse
 */
export type OpenPullRequestResponse = Message<"numinance.ports.scm.v1.OpenPullRequestResponse"> & {
  /**
   * The pull request.
   *
   * @generated from field: numinance.ports.scm.v1.PullRequest pull_request = 1;
   */
  pullRequest?: PullRequest | undefined;

  /**
   * True when an open pull request for the same head and base already existed and nothing was opened.
   *
   * @generated from field: bool existing = 2;
   */
  existing: boolean;
};

/**
 * Describes the message numinance.ports.scm.v1.OpenPullRequestResponse.
 * Use `create(OpenPullRequestResponseSchema)` to create a new message.
 */
export const OpenPullRequestResponseSchema: GenMessage<OpenPullRequestResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 16);

/**
 * UpdatePullRequestRequest names the fields to change.
 *
 * @generated from message numinance.ports.scm.v1.UpdatePullRequestRequest
 */
export type UpdatePullRequestRequest = Message<"numinance.ports.scm.v1.UpdatePullRequestRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the pull request's number.
   *
   * @generated from field: uint64 number = 2;
   */
  number: bigint;

  /**
   * The new title, 1 to 256 printable bytes; empty leaves it as it is.
   *
   * @generated from field: string title = 3;
   */
  title: string;

  /**
   * The new body, at most 16384 bytes; empty leaves it as it is.
   *
   * @generated from field: string body = 4;
   */
  body: string;

  /**
   * The new state: unspecified leaves it, CLOSED closes it; OPEN and MERGED are rejected.
   *
   * @generated from field: numinance.ports.scm.v1.PullRequestState state = 5;
   */
  state: PullRequestState;
};

/**
 * Describes the message numinance.ports.scm.v1.UpdatePullRequestRequest.
 * Use `create(UpdatePullRequestRequestSchema)` to create a new message.
 */
export const UpdatePullRequestRequestSchema: GenMessage<UpdatePullRequestRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 17);

/**
 * UpdatePullRequestResponse is the pull request after the update.
 *
 * @generated from message numinance.ports.scm.v1.UpdatePullRequestResponse
 */
export type UpdatePullRequestResponse = Message<"numinance.ports.scm.v1.UpdatePullRequestResponse"> & {
  /**
   * The pull request.
   *
   * @generated from field: numinance.ports.scm.v1.PullRequest pull_request = 1;
   */
  pullRequest?: PullRequest | undefined;
};

/**
 * Describes the message numinance.ports.scm.v1.UpdatePullRequestResponse.
 * Use `create(UpdatePullRequestResponseSchema)` to create a new message.
 */
export const UpdatePullRequestResponseSchema: GenMessage<UpdatePullRequestResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 18);

/**
 * CommentOnPullRequestRequest is one comment.
 *
 * @generated from message numinance.ports.scm.v1.CommentOnPullRequestRequest
 */
export type CommentOnPullRequestRequest = Message<"numinance.ports.scm.v1.CommentOnPullRequestRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the pull request's number.
   *
   * @generated from field: uint64 number = 2;
   */
  number: bigint;

  /**
   * Required: 1 to 16384 bytes of printable characters, U+0009 and U+000A.
   *
   * @generated from field: string body = 3;
   */
  body: string;
};

/**
 * Describes the message numinance.ports.scm.v1.CommentOnPullRequestRequest.
 * Use `create(CommentOnPullRequestRequestSchema)` to create a new message.
 */
export const CommentOnPullRequestRequestSchema: GenMessage<CommentOnPullRequestRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 19);

/**
 * CommentOnPullRequestResponse names the comment.
 *
 * @generated from message numinance.ports.scm.v1.CommentOnPullRequestResponse
 */
export type CommentOnPullRequestResponse = Message<"numinance.ports.scm.v1.CommentOnPullRequestResponse"> & {
  /**
   * The host's id of the comment, at most 128 printable bytes; opaque.
   *
   * @generated from field: string comment_id = 1;
   */
  commentId: string;
};

/**
 * Describes the message numinance.ports.scm.v1.CommentOnPullRequestResponse.
 * Use `create(CommentOnPullRequestResponseSchema)` to create a new message.
 */
export const CommentOnPullRequestResponseSchema: GenMessage<CommentOnPullRequestResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 20);

/**
 * GetPullRequestStatusRequest names a pull request.
 *
 * @generated from message numinance.ports.scm.v1.GetPullRequestStatusRequest
 */
export type GetPullRequestStatusRequest = Message<"numinance.ports.scm.v1.GetPullRequestStatusRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the pull request's number.
   *
   * @generated from field: uint64 number = 2;
   */
  number: bigint;
};

/**
 * Describes the message numinance.ports.scm.v1.GetPullRequestStatusRequest.
 * Use `create(GetPullRequestStatusRequestSchema)` to create a new message.
 */
export const GetPullRequestStatusRequestSchema: GenMessage<GetPullRequestStatusRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 21);

/**
 * CheckRun is one status check on the head of a pull request.
 *
 * @generated from message numinance.ports.scm.v1.CheckRun
 */
export type CheckRun = Message<"numinance.ports.scm.v1.CheckRun"> & {
  /**
   * The check's name, 1 to 256 printable bytes; untrusted.
   *
   * @generated from field: string name = 1;
   */
  name: string;

  /**
   * The check's outcome.
   *
   * @generated from field: numinance.ports.scm.v1.CheckConclusion conclusion = 2;
   */
  conclusion: CheckConclusion;
};

/**
 * Describes the message numinance.ports.scm.v1.CheckRun.
 * Use `create(CheckRunSchema)` to create a new message.
 */
export const CheckRunSchema: GenMessage<CheckRun> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 22);

/**
 * GetPullRequestStatusResponse is the state of a pull request; every field is the host's claim.
 *
 * @generated from message numinance.ports.scm.v1.GetPullRequestStatusResponse
 */
export type GetPullRequestStatusResponse = Message<"numinance.ports.scm.v1.GetPullRequestStatusResponse"> & {
  /**
   * The pull request as the host holds it now.
   *
   * @generated from field: numinance.ports.scm.v1.PullRequest pull_request = 1;
   */
  pullRequest?: PullRequest | undefined;

  /**
   * Whether the host can merge it.
   *
   * @generated from field: numinance.ports.scm.v1.Mergeability mergeability = 2;
   */
  mergeability: Mergeability;

  /**
   * The checks of the head, at most 256, unique by name.
   *
   * @generated from field: repeated numinance.ports.scm.v1.CheckRun checks = 3;
   */
  checks: CheckRun[];

  /**
   * The number of distinct reviewers whose latest review approves the head.
   *
   * @generated from field: uint32 approvals = 4;
   */
  approvals: number;

  /**
   * True when a reviewer's latest review requests changes.
   *
   * @generated from field: bool changes_requested = 5;
   */
  changesRequested: boolean;

  /**
   * The merge commit's full id when state is MERGED; empty otherwise.
   *
   * @generated from field: string merge_revision = 6;
   */
  mergeRevision: string;
};

/**
 * Describes the message numinance.ports.scm.v1.GetPullRequestStatusResponse.
 * Use `create(GetPullRequestStatusResponseSchema)` to create a new message.
 */
export const GetPullRequestStatusResponseSchema: GenMessage<GetPullRequestStatusResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 23);

/**
 * MergePullRequestRequest is one merge.
 *
 * @generated from message numinance.ports.scm.v1.MergePullRequestRequest
 */
export type MergePullRequestRequest = Message<"numinance.ports.scm.v1.MergePullRequestRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the pull request's number.
   *
   * @generated from field: uint64 number = 2;
   */
  number: bigint;

  /**
   * Required: the full commit id of the head the caller verified; a moved head is FAILED_PRECONDITION.
   *
   * @generated from field: string expected_head_revision = 3;
   */
  expectedHeadRevision: string;

  /**
   * Required: the branch the pull request was opened into; a pull request that now merges into another branch is FAILED_PRECONDITION, so the merge is into the base whose protection was read.
   *
   * @generated from field: string expected_base_branch = 4;
   */
  expectedBaseBranch: string;
};

/**
 * Describes the message numinance.ports.scm.v1.MergePullRequestRequest.
 * Use `create(MergePullRequestRequestSchema)` to create a new message.
 */
export const MergePullRequestRequestSchema: GenMessage<MergePullRequestRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 24);

/**
 * MergePullRequestResponse is the merge outcome.
 *
 * @generated from message numinance.ports.scm.v1.MergePullRequestResponse
 */
export type MergePullRequestResponse = Message<"numinance.ports.scm.v1.MergePullRequestResponse"> & {
  /**
   * The full commit id of the merge on the base branch; empty for a dry run.
   *
   * @generated from field: string merge_revision = 1;
   */
  mergeRevision: string;

  /**
   * True when the pull request was already merged by an earlier call and nothing was merged now.
   *
   * @generated from field: bool already_merged = 2;
   */
  alreadyMerged: boolean;

  /**
   * The time the host merged it; unset for a dry run.
   *
   * @generated from field: google.protobuf.Timestamp merged_at = 3;
   */
  mergedAt?: Timestamp | undefined;

  /**
   * The full commit id of the head the merge was of; equal to expected_head_revision, and empty for a dry run. The caller compares it: a merge already done at another head is not the release of the verified candidate.
   *
   * @generated from field: string head_revision = 4;
   */
  headRevision: string;
};

/**
 * Describes the message numinance.ports.scm.v1.MergePullRequestResponse.
 * Use `create(MergePullRequestResponseSchema)` to create a new message.
 */
export const MergePullRequestResponseSchema: GenMessage<MergePullRequestResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 25);

/**
 * SearchCodeRequest is one search.
 *
 * @generated from message numinance.ports.scm.v1.SearchCodeRequest
 */
export type SearchCodeRequest = Message<"numinance.ports.scm.v1.SearchCodeRequest"> & {
  /**
   * Required, as in GetRepositoryRequest. Marked as the resource of the call.
   *
   * @generated from field: string repository = 1;
   */
  repository: string;

  /**
   * Required: the search text, 1 to 256 printable bytes.
   *
   * @generated from field: string query = 2;
   */
  query: string;

  /**
   * Required, 1 to 100: the most paths to return.
   *
   * @generated from field: uint32 max_results = 3;
   */
  maxResults: number;
};

/**
 * Describes the message numinance.ports.scm.v1.SearchCodeRequest.
 * Use `create(SearchCodeRequestSchema)` to create a new message.
 */
export const SearchCodeRequestSchema: GenMessage<SearchCodeRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 26);

/**
 * SearchCodeResponse lists matching paths.
 *
 * @generated from message numinance.ports.scm.v1.SearchCodeResponse
 */
export type SearchCodeResponse = Message<"numinance.ports.scm.v1.SearchCodeResponse"> & {
  /**
   * At most max_results paths on the default branch, in the path grammar.
   *
   * @generated from field: repeated string paths = 1;
   */
  paths: string[];
};

/**
 * Describes the message numinance.ports.scm.v1.SearchCodeResponse.
 * Use `create(SearchCodeResponseSchema)` to create a new message.
 */
export const SearchCodeResponseSchema: GenMessage<SearchCodeResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_scm_v1_scm, 27);

/**
 * PullRequestState is the state of a pull request on the host.
 *
 * @generated from enum numinance.ports.scm.v1.PullRequestState
 */
export enum PullRequestState {
  /**
   * PULL_REQUEST_STATE_UNSPECIFIED is the zero value; it is rejected.
   *
   * @generated from enum value: PULL_REQUEST_STATE_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * PULL_REQUEST_STATE_OPEN: open for review and merge.
   *
   * @generated from enum value: PULL_REQUEST_STATE_OPEN = 1;
   */
  OPEN = 1,

  /**
   * PULL_REQUEST_STATE_CLOSED: closed without a merge.
   *
   * @generated from enum value: PULL_REQUEST_STATE_CLOSED = 2;
   */
  CLOSED = 2,

  /**
   * PULL_REQUEST_STATE_MERGED: merged into its base branch.
   *
   * @generated from enum value: PULL_REQUEST_STATE_MERGED = 3;
   */
  MERGED = 3,
}

/**
 * Describes the enum numinance.ports.scm.v1.PullRequestState.
 */
export const PullRequestStateSchema: GenEnum<PullRequestState> = /*@__PURE__*/
  enumDesc(file_numinance_ports_scm_v1_scm, 0);

/**
 * Mergeability is whether the host can merge a pull request now.
 *
 * @generated from enum numinance.ports.scm.v1.Mergeability
 */
export enum Mergeability {
  /**
   * MERGEABILITY_UNSPECIFIED is the zero value; it is rejected.
   *
   * @generated from enum value: MERGEABILITY_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * MERGEABILITY_MERGEABLE: the host reports no conflict.
   *
   * @generated from enum value: MERGEABILITY_MERGEABLE = 1;
   */
  MERGEABLE = 1,

  /**
   * MERGEABILITY_CONFLICTING: the head conflicts with the base.
   *
   * @generated from enum value: MERGEABILITY_CONFLICTING = 2;
   */
  CONFLICTING = 2,

  /**
   * MERGEABILITY_UNKNOWN: the host has not computed it yet; ask again.
   *
   * @generated from enum value: MERGEABILITY_UNKNOWN = 3;
   */
  UNKNOWN = 3,
}

/**
 * Describes the enum numinance.ports.scm.v1.Mergeability.
 */
export const MergeabilitySchema: GenEnum<Mergeability> = /*@__PURE__*/
  enumDesc(file_numinance_ports_scm_v1_scm, 1);

/**
 * CheckConclusion is the outcome of one status check on the head of a pull request.
 *
 * @generated from enum numinance.ports.scm.v1.CheckConclusion
 */
export enum CheckConclusion {
  /**
   * CHECK_CONCLUSION_UNSPECIFIED is the zero value; it is rejected.
   *
   * @generated from enum value: CHECK_CONCLUSION_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * CHECK_CONCLUSION_PENDING: queued or running.
   *
   * @generated from enum value: CHECK_CONCLUSION_PENDING = 1;
   */
  PENDING = 1,

  /**
   * CHECK_CONCLUSION_SUCCESS: passed.
   *
   * @generated from enum value: CHECK_CONCLUSION_SUCCESS = 2;
   */
  SUCCESS = 2,

  /**
   * CHECK_CONCLUSION_FAILURE: failed, timed out or needs action.
   *
   * @generated from enum value: CHECK_CONCLUSION_FAILURE = 3;
   */
  FAILURE = 3,

  /**
   * CHECK_CONCLUSION_NEUTRAL: ended with no verdict (neutral, skipped, cancelled).
   *
   * @generated from enum value: CHECK_CONCLUSION_NEUTRAL = 4;
   */
  NEUTRAL = 4,
}

/**
 * Describes the enum numinance.ports.scm.v1.CheckConclusion.
 */
export const CheckConclusionSchema: GenEnum<CheckConclusion> = /*@__PURE__*/
  enumDesc(file_numinance_ports_scm_v1_scm, 2);

/**
 * ScmService is the scm port a source-control provider serves.
 *
 * @generated from service numinance.ports.scm.v1.ScmService
 */
export const ScmService: GenService<{
  /**
   * GetRepository returns what the release and profile contexts need to pin a Change: the default branch and the commit it points to now. The intake of a Change records default_branch_head as the protected base revision of the Change, so the profile and the policy are read from that commit and never from the candidate.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.GetRepository
   */
  getRepository: {
    methodKind: "unary";
    input: typeof GetRepositoryRequestSchema;
    output: typeof GetRepositoryResponseSchema;
  },
  /**
   * GetTree lists the entries of one directory at one revision, or of the whole subtree when recursive is set, at most max_entries of them.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.GetTree
   */
  getTree: {
    methodKind: "unary";
    input: typeof GetTreeRequestSchema;
    output: typeof GetTreeResponseSchema;
  },
  /**
   * GetFile returns the content of one file at one revision from its first byte, at most max_bytes. Content is untrusted data.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.GetFile
   */
  getFile: {
    methodKind: "unary";
    input: typeof GetFileRequestSchema;
    output: typeof GetFileResponseSchema;
  },
  /**
   * ListBranches lists branches in the host's order, one page at a time.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.ListBranches
   */
  listBranches: {
    methodKind: "unary";
    input: typeof ListBranchesRequestSchema;
    output: typeof ListBranchesResponseSchema;
  },
  /**
   * CreateBranch creates branch `name` at from_revision. A replay of the same name and revision answers the existing branch; the same name at another revision is ALREADY_EXISTS. LOW: a new ref changes no code on any existing branch, and deleting it undoes it. No dry run: the branch is the whole effect and its creation is the check of the name; the compensating action closes any pull request opened from it.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.CreateBranch
   */
  createBranch: {
    methodKind: "unary";
    input: typeof CreateBranchRequestSchema;
    output: typeof CreateBranchResponseSchema;
  },
  /**
   * GetBranchProtection reports the protection the host enforces on one branch. The provider reads the host's state and changes nothing.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.GetBranchProtection
   */
  getBranchProtection: {
    methodKind: "unary";
    input: typeof GetBranchProtectionRequestSchema;
    output: typeof GetBranchProtectionResponseSchema;
  },
  /**
   * OpenPullRequest opens a pull request from head_branch, which points at head_revision, into base_branch. The request names the head revision so the pull request is the candidate that was verified and not whatever the branch holds later: a head_branch that does not point at head_revision is FAILED_PRECONDITION. Idempotent by the host's key and by reconciliation: an open pull request for the same head and base is answered, not opened again. MEDIUM: it publishes the candidate to reviewers and starts the host's checks, and a closed pull request undoes it. Dry run checks the names, the revision and that no pull request is open yet, opens nothing and answers with pull_request unset when it would open one.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.OpenPullRequest
   */
  openPullRequest: {
    methodKind: "unary";
    input: typeof OpenPullRequestRequestSchema;
    output: typeof OpenPullRequestResponseSchema;
  },
  /**
   * UpdatePullRequest changes the title or body of a pull request, or closes it (state CLOSED); it never reopens a merged pull request and never changes the head or the base. LOW: it edits text or closes a pull request, both undone by another update. No dry run: the update is the whole effect.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.UpdatePullRequest
   */
  updatePullRequest: {
    methodKind: "unary";
    input: typeof UpdatePullRequestRequestSchema;
    output: typeof UpdatePullRequestResponseSchema;
  },
  /**
   * CommentOnPullRequest adds one comment, identified by the host's key: a replay answers the first comment and adds none. LOW: a comment changes no code, though it is visible to reviewers. No dry run: the comment is the whole effect, and a dry run would be the call.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.CommentOnPullRequest
   */
  commentOnPullRequest: {
    methodKind: "unary";
    input: typeof CommentOnPullRequestRequestSchema;
    output: typeof CommentOnPullRequestResponseSchema;
  },
  /**
   * GetPullRequestStatus returns the state of a pull request, its head, its checks and the reviews it holds. It is also the reconciliation read of OpenPullRequest and MergePullRequest after a lost response. Review counts are the host's claim: the release context never merges on them alone.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.GetPullRequestStatus
   */
  getPullRequestStatus: {
    methodKind: "unary";
    input: typeof GetPullRequestStatusRequestSchema;
    output: typeof GetPullRequestStatusResponseSchema;
  },
  /**
   * MergePullRequest merges a pull request into its base branch. PRIVILEGED: it changes the protected branch every later Change is pinned to. It is allowed only to the release-pipeline identity under a grant of level Merge or above, after a ReleaseGate allow bound to this candidate; the policy floors refuse the author of the Change. expected_head_revision is the head the caller verified: a pull request whose head moved is FAILED_PRECONDITION, so the merge is of the verified candidate and no other. A merge already done under the same key is answered as done with the merge commit, never merged again. Dry run answers, with an empty merge_revision, that the pull request is open at expected_head_revision and not in conflict, or fails as the merge would, and merges nothing.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.MergePullRequest
   */
  mergePullRequest: {
    methodKind: "unary";
    input: typeof MergePullRequestRequestSchema;
    output: typeof MergePullRequestResponseSchema;
  },
  /**
   * SearchCode searches the default branch for text and returns paths, never content. The query is search text only, passed to the host as a parameter and never spliced into its query language.
   *
   * @generated from rpc numinance.ports.scm.v1.ScmService.SearchCode
   */
  searchCode: {
    methodKind: "unary";
    input: typeof SearchCodeRequestSchema;
    output: typeof SearchCodeResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_scm_v1_scm, 0);

