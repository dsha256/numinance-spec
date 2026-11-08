// The ticketing port: reads and searches issues, comments on an issue, moves it to a state category and links one issue to another as blocking it. Any tracker that has projects, issues with a key, a title, a text body, a workflow of states, comments and issue links can implement every core verb (Jira, GitHub Issues, Linear, GitLab Issues, Azure Boards); a vendor feature beyond them is a capability `ticketing.<feature>` negotiated at handshake, added in a later minor. The query languages of the trackers (JQL, WIQL) are the capability `ticketing.query_language`, never a core field. There is no VendorCall in 1.x, because the host refuses a provider that does not declare every side-effecting RPC of its port, and an escape hatch no provider implements is a PRIVILEGED capability that does nothing. Custom fields and vendor filters are capabilities or typed extensions that the Engine never reads.
// Callers: the change context calls GetIssue at intake through the plugin host and builds the ticket text of the Change from the validated fields; the Engine reconciles a retried write through GetIssue before it retries. The write verbs are reached only by a later write-back card and only under a policy decision, a redaction and a size cap. The host decides every call, on the exact request bytes, against the grant of the identity named in ActionContext. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: every request names its project in `project`, which the Engine marks as the resource of the call (RFC-0002): a call on a project outside the acting grant's resource scope is denied before dispatch. LinkIssue names two projects and marks both. A provider holds no credential of its own: it asks the host for a credential handle for the intent its manifest declares and sends the handle to the egress proxy, never a raw token. A provider never follows a redirect with an injected credential and never fetches a URL found in ticket content or in a response (a media link, a self link, a pagination link, an inline card).
// Untrusted answers: every title, body, type, label, operation id and key that the tracker holds is untrusted data, whoever wrote it on the tracker; anyone who can edit or comment on an issue can write any of them. The change context size-limits it, validates it against this file, stores it by digest and labels and delimits it as untrusted wherever it reaches a prompt, never as instructions. Nothing a provider answers changes policy, a grant, a risk class, an autonomy level, a tool list, a playbook choice or an approval: no field of an issue (label, type, priority, component, custom field) is mapped mechanically to any of them.
// Text hygiene: a provider returns plain text. It renders the tracker's rich text (ADF, wiki markup, HTML, Markdown) to plain text and strips from every string it returns the control characters (Unicode category Cc, except a line feed U+000A and a tab U+0009 inside a body), the bidirectional controls (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069) and the zero-width characters (U+200B to U+200D, U+2060, U+FEFF). The Engine repeats the strip, because the provider is the untrusted side, so a conforming provider returns none of them and a host never relies on that.
// Not found and no permission: a provider answers NOT_FOUND, with the same status and the same message, for an issue that does not exist, for an issue the credential may not see, and for an issue whose current project is not the project the request names (an issue that moved answers only under its new project). The answer never tells which of the three it is, so the port is not an oracle for the existence of issues. The message never contains the key or any text the tracker returned.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; NOT_FOUND as above; FAILED_PRECONDITION for a TransitionIssue whose target category no tracker transition reaches from the current state, and for a write the tracker refuses because of the issue's state; RESOURCE_EXHAUSTED when the tracker rate-limits the call; UNAVAILABLE for a tracker that cannot be reached or a transient failure the caller may retry. A provider never invents a result: an outage is an error, never an empty answer.
// Evidence: every write verb (CommentOnIssue, TransitionIssue, LinkIssue) declares one record type, ticketing-action. A provider emits exactly one per call that made the change or found it already made, through HostService.EmitEvidence: a compact receipt of the verb, the project, the issue key and the result (the comment id, the categories before and after, the other key of the link), never a title, a body or a comment text. A call that ends SUCCEEDED without it is PARTIAL. A dry run changes nothing and emits none. The read verbs declare no evidence.
// Reconciliation: the host's idempotency key names the operation, and a replay answers the first result. The Engine reconciles a write through GetIssue before it retries after a lost response. A provider that cannot key a write declares IDEMPOTENCY_NONE in its manifest, and a step that calls it needs a declared compensation or a human approval.
// Bounds: every string is at most 1024 bytes unless stated; over-limit input is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding). Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Formats: a project is the tracker's own spelling of a project: one or two segments separated by one '/', each matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$ (a Jira project key is one segment, a GitHub `<owner>/<repo>` is two). No segment is empty, `.` or `..`, no `..` appears anywhere, and there is no control, white space or non-ASCII character, so a provider that puts a project into a REST path cannot be made to leave it; LinkIssue applies the grammar to both of its projects. An issue key is the tracker's own spelling of an issue within a project, matching ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$: it never contains a '/', is at most 64 bytes and never has a '#', so that the Engine can build a ticket reference `<binding instance>:<issue key>` that always fits. The Engine builds that reference; it never takes one from a URL a provider returns.
// Every element is since 1.2.0: tag v0.2.0 shipped the 1.1.0 content, so 1.2.0 is the first unreleased spec minor.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/ticketing/v1/ticketing.proto (package numinance.ports.ticketing.v1, syntax proto3)
/* eslint-disable */

import type { GenEnum, GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { enumDesc, fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/ticketing/v1/ticketing.proto.
 */
export const file_numinance_ports_ticketing_v1_ticketing: GenFile = /*@__PURE__*/
  fileDesc("CixudW1pbmFuY2UvcG9ydHMvdGlja2V0aW5nL3YxL3RpY2tldGluZy5wcm90bxIcbnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MSJ2Cg9HZXRJc3N1ZVJlcXVlc3QSHgoHcHJvamVjdBgBIAEoCUINkrUYCQoFMS4yLjAwARIeCglpc3N1ZV9rZXkYAiABKAlCC5K1GAcKBTEuMi4wEiMKDm1heF9ib2R5X2J5dGVzGAMgASgNQguStRgHCgUxLjIuMCLXAgoQR2V0SXNzdWVSZXNwb25zZRIeCglpc3N1ZV9rZXkYASABKAlCC5K1GAcKBTEuMi4wEhwKB3Byb2plY3QYAiABKAlCC5K1GAcKBTEuMi4wEhoKBXRpdGxlGAMgASgJQguStRgHCgUxLjIuMBIZCgRib2R5GAQgASgJQguStRgHCgUxLjIuMBIeCgl0cnVuY2F0ZWQYBSABKAhCC5K1GAcKBTEuMi4wEkwKBXN0YXRlGAYgASgOMjAubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5Jc3N1ZVN0YXRlQ2F0ZWdvcnlCC5K1GAcKBTEuMi4wEh8KCmlzc3VlX3R5cGUYByABKAlCC5K1GAcKBTEuMi4wEhsKBmxhYmVscxgIIAMoCUILkrUYBwoFMS4yLjASIgoNb3BlcmF0aW9uX2lkcxgJIAMoCUILkrUYBwoFMS4yLjAiwAEKE1NlYXJjaElzc3Vlc1JlcXVlc3QSHgoHcHJvamVjdBgBIAEoCUINkrUYCQoFMS4yLjAwARIZCgR0ZXh0GAIgASgJQguStRgHCgUxLjIuMBJMCgVzdGF0ZRgDIAEoDjIwLm51bWluYW5jZS5wb3J0cy50aWNrZXRpbmcudjEuSXNzdWVTdGF0ZUNhdGVnb3J5QguStRgHCgUxLjIuMBIgCgttYXhfcmVzdWx0cxgEIAEoDUILkrUYBwoFMS4yLjAimAEKDElzc3VlU3VtbWFyeRIeCglpc3N1ZV9rZXkYASABKAlCC5K1GAcKBTEuMi4wEhoKBXRpdGxlGAIgASgJQguStRgHCgUxLjIuMBJMCgVzdGF0ZRgDIAEoDjIwLm51bWluYW5jZS5wb3J0cy50aWNrZXRpbmcudjEuSXNzdWVTdGF0ZUNhdGVnb3J5QguStRgHCgUxLjIuMCJ/ChRTZWFyY2hJc3N1ZXNSZXNwb25zZRJHCgZpc3N1ZXMYASADKAsyKi5udW1pbmFuY2UucG9ydHMudGlja2V0aW5nLnYxLklzc3VlU3VtbWFyeUILkrUYBwoFMS4yLjASHgoJdHJ1bmNhdGVkGAIgASgIQguStRgHCgUxLjIuMCJyChVDb21tZW50T25Jc3N1ZVJlcXVlc3QSHgoHcHJvamVjdBgBIAEoCUINkrUYCQoFMS4yLjAwARIeCglpc3N1ZV9rZXkYAiABKAlCC5K1GAcKBTEuMi4wEhkKBGJvZHkYAyABKAlCC5K1GAcKBTEuMi4wIlgKFkNvbW1lbnRPbklzc3VlUmVzcG9uc2USHwoKY29tbWVudF9pZBgBIAEoCUILkrUYBwoFMS4yLjASHQoIZXhpc3RpbmcYAiABKAhCC5K1GAcKBTEuMi4wIqcBChZUcmFuc2l0aW9uSXNzdWVSZXF1ZXN0Eh4KB3Byb2plY3QYASABKAlCDZK1GAkKBTEuMi4wMAESHgoJaXNzdWVfa2V5GAIgASgJQguStRgHCgUxLjIuMBJNCgZ0YXJnZXQYAyABKA4yMC5udW1pbmFuY2UucG9ydHMudGlja2V0aW5nLnYxLklzc3VlU3RhdGVDYXRlZ29yeUILkrUYBwoFMS4yLjAi1gEKF1RyYW5zaXRpb25Jc3N1ZVJlc3BvbnNlEk8KCHByZXZpb3VzGAEgASgOMjAubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5Jc3N1ZVN0YXRlQ2F0ZWdvcnlCC5K1GAcKBTEuMi4wEkwKBXN0YXRlGAIgASgOMjAubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5Jc3N1ZVN0YXRlQ2F0ZWdvcnlCC5K1GAcKBTEuMi4wEhwKB2NoYW5nZWQYAyABKAhCC5K1GAcKBTEuMi4wIu8BChBMaW5rSXNzdWVSZXF1ZXN0Eh4KB3Byb2plY3QYASABKAlCDZK1GAkKBTEuMi4wMAESHgoJaXNzdWVfa2V5GAIgASgJQguStRgHCgUxLjIuMBImCg9ibG9ja2VkX3Byb2plY3QYAyABKAlCDZK1GAkKBTEuMi4wMAESJgoRYmxvY2tlZF9pc3N1ZV9rZXkYBCABKAlCC5K1GAcKBTEuMi4wEksKCWxpbmtfdHlwZRgFIAEoDjIrLm51bWluYW5jZS5wb3J0cy50aWNrZXRpbmcudjEuSXNzdWVMaW5rVHlwZUILkrUYBwoFMS4yLjAiMgoRTGlua0lzc3VlUmVzcG9uc2USHQoIZXhpc3RpbmcYASABKAhCC5K1GAcKBTEuMi4wKsEBChJJc3N1ZVN0YXRlQ2F0ZWdvcnkSJAogSVNTVUVfU1RBVEVfQ0FURUdPUllfVU5TUEVDSUZJRUQQABIdChlJU1NVRV9TVEFURV9DQVRFR09SWV9PUEVOEAESJAogSVNTVUVfU1RBVEVfQ0FURUdPUllfSU5fUFJPR1JFU1MQAhIdChlJU1NVRV9TVEFURV9DQVRFR09SWV9ET05FEAMSIQodSVNTVUVfU1RBVEVfQ0FURUdPUllfQ0FOQ0VMRUQQBCpMCg1Jc3N1ZUxpbmtUeXBlEh8KG0lTU1VFX0xJTktfVFlQRV9VTlNQRUNJRklFRBAAEhoKFklTU1VFX0xJTktfVFlQRV9CTE9DS1MQATKJCQoQVGlja2V0aW5nU2VydmljZRL1AQoIR2V0SXNzdWUSLS5udW1pbmFuY2UucG9ydHMudGlja2V0aW5nLnYxLkdldElzc3VlUmVxdWVzdBouLm51bWluYW5jZS5wb3J0cy50aWNrZXRpbmcudjEuR2V0SXNzdWVSZXNwb25zZSKJAYq1GIQBCAEQARgBQgUxLjIuMEp1UmV0dXJucyBvbmUgaXNzdWUgb2YgYSBwcm9qZWN0OiBrZXksIHRpdGxlLCBwbGFpbi10ZXh0IGJvZHksIHN0YXRlIGNhdGVnb3J5LCB0eXBlLCBsYWJlbHMgYW5kIHJlY29yZGVkIG9wZXJhdGlvbiBpZHMuEtgBCgxTZWFyY2hJc3N1ZXMSMS5udW1pbmFuY2UucG9ydHMudGlja2V0aW5nLnYxLlNlYXJjaElzc3Vlc1JlcXVlc3QaMi5udW1pbmFuY2UucG9ydHMudGlja2V0aW5nLnYxLlNlYXJjaElzc3Vlc1Jlc3BvbnNlImGKtRhdCAEQARgBQgUxLjIuMEpOU2VhcmNoZXMgdGhlIGlzc3VlcyBvZiBhIHByb2plY3QgZm9yIGEgdGV4dCwgb3B0aW9uYWxseSBpbiBvbmUgc3RhdGUgY2F0ZWdvcnkuEsoBCg5Db21tZW50T25Jc3N1ZRIzLm51bWluYW5jZS5wb3J0cy50aWNrZXRpbmcudjEuQ29tbWVudE9uSXNzdWVSZXF1ZXN0GjQubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5Db21tZW50T25Jc3N1ZVJlc3BvbnNlIk2KtRhJCAIQARgBMhB0aWNrZXRpbmctYWN0aW9uQgUxLjIuMEooQWRkcyBvbmUgcGxhaW4tdGV4dCBjb21tZW50IHRvIGFuIGlzc3VlLhKaAgoPVHJhbnNpdGlvbklzc3VlEjQubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5UcmFuc2l0aW9uSXNzdWVSZXF1ZXN0GjUubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5UcmFuc2l0aW9uSXNzdWVSZXNwb25zZSKZAYq1GJQBCAMQARgBIAEqIFRpY2tldGluZ1NlcnZpY2UuVHJhbnNpdGlvbklzc3VlMhB0aWNrZXRpbmctYWN0aW9uQgUxLjIuMEpPTW92ZXMgYW4gaXNzdWUgdG8gYSB0YXJnZXQgc3RhdGUgY2F0ZWdvcnk6IG9wZW4sIGluIHByb2dyZXNzLCBkb25lIG9yIGNhbmNlbGVkLhK3AQoJTGlua0lzc3VlEi4ubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5MaW5rSXNzdWVSZXF1ZXN0Gi8ubnVtaW5hbmNlLnBvcnRzLnRpY2tldGluZy52MS5MaW5rSXNzdWVSZXNwb25zZSJJirUYRQgCEAEYATIQdGlja2V0aW5nLWFjdGlvbkIFMS4yLjBKJExpbmtzIG9uZSBpc3N1ZSBhcyBibG9ja2luZyBhbm90aGVyLkKeAgogY29tLm51bWluYW5jZS5wb3J0cy50aWNrZXRpbmcudjFCDlRpY2tldGluZ1Byb3RvUAFaV2dpdGh1Yi5jb20vbnVtaW5hbmNlL251bWluYW5jZS1zcGVjL3Nkay9nby9nZW4vbnVtaW5hbmNlL3BvcnRzL3RpY2tldGluZy92MTt0aWNrZXRpbmd2MaICA05QVKoCHE51bWluYW5jZS5Qb3J0cy5UaWNrZXRpbmcuVjHKAhxOdW1pbmFuY2VcUG9ydHNcVGlja2V0aW5nXFYx4gIoTnVtaW5hbmNlXFBvcnRzXFRpY2tldGluZ1xWMVxHUEJNZXRhZGF0YeoCH051bWluYW5jZTo6UG9ydHM6OlRpY2tldGluZzo6VjFiBnByb3RvMw", [file_numinance_annotations_v1_annotations]);

/**
 * GetIssueRequest names an issue.
 *
 * @generated from message numinance.ports.ticketing.v1.GetIssueRequest
 */
export type GetIssueRequest = Message<"numinance.ports.ticketing.v1.GetIssueRequest"> & {
  /**
   * Required: the project, in the project grammar. Marked as the resource of the call.
   *
   * @generated from field: string project = 1;
   */
  project: string;

  /**
   * Required: the issue key, in the issue key grammar.
   *
   * @generated from field: string issue_key = 2;
   */
  issueKey: string;

  /**
   * Required, 1 to 65536: the most body bytes to return.
   *
   * @generated from field: uint32 max_body_bytes = 3;
   */
  maxBodyBytes: number;
};

/**
 * Describes the message numinance.ports.ticketing.v1.GetIssueRequest.
 * Use `create(GetIssueRequestSchema)` to create a new message.
 */
export const GetIssueRequestSchema: GenMessage<GetIssueRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 0);

/**
 * GetIssueResponse is one issue.
 *
 * @generated from message numinance.ports.ticketing.v1.GetIssueResponse
 */
export type GetIssueResponse = Message<"numinance.ports.ticketing.v1.GetIssueResponse"> & {
  /**
   * The issue key, equal to the one requested.
   *
   * @generated from field: string issue_key = 1;
   */
  issueKey: string;

  /**
   * The issue's project now, equal to the one requested.
   *
   * @generated from field: string project = 2;
   */
  project: string;

  /**
   * The title as plain text, 1 to 1024 bytes, without a line feed or a tab; untrusted.
   *
   * @generated from field: string title = 3;
   */
  title: string;

  /**
   * The description as plain text, at most max_body_bytes bytes, cut at a character boundary; untrusted.
   *
   * @generated from field: string body = 4;
   */
  body: string;

  /**
   * True when the description is longer than max_body_bytes. A caller that needs the whole text refuses a truncated answer.
   *
   * @generated from field: bool truncated = 5;
   */
  truncated: boolean;

  /**
   * The state category of the issue now.
   *
   * @generated from field: numinance.ports.ticketing.v1.IssueStateCategory state = 6;
   */
  state: IssueStateCategory;

  /**
   * The tracker's issue type, for example Bug or Story, at most 128 printable bytes; untrusted. No decision rests on it.
   *
   * @generated from field: string issue_type = 7;
   */
  issueType: string;

  /**
   * The labels, at most 64, each 1 to 128 printable bytes; untrusted. No decision rests on them.
   *
   * @generated from field: repeated string labels = 8;
   */
  labels: string[];

  /**
   * The operation ids recorded on the issue by earlier writes, at most 64, each at most 128 printable bytes; untrusted. Anyone who can edit or comment on the issue can forge one: the Engine accepts one only if it equals the operation_id it computed itself for that Change.
   *
   * @generated from field: repeated string operation_ids = 9;
   */
  operationIds: string[];
};

/**
 * Describes the message numinance.ports.ticketing.v1.GetIssueResponse.
 * Use `create(GetIssueResponseSchema)` to create a new message.
 */
export const GetIssueResponseSchema: GenMessage<GetIssueResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 1);

/**
 * SearchIssuesRequest asks for issues of one project.
 *
 * @generated from message numinance.ports.ticketing.v1.SearchIssuesRequest
 */
export type SearchIssuesRequest = Message<"numinance.ports.ticketing.v1.SearchIssuesRequest"> & {
  /**
   * Required: the project, in the project grammar. Marked as the resource of the call.
   *
   * @generated from field: string project = 1;
   */
  project: string;

  /**
   * The text to find, at most 256 printable bytes, matched as plain text and never as a query; empty matches every issue of the project.
   *
   * @generated from field: string text = 2;
   */
  text: string;

  /**
   * Only issues in this category; the unspecified value means any.
   *
   * @generated from field: numinance.ports.ticketing.v1.IssueStateCategory state = 3;
   */
  state: IssueStateCategory;

  /**
   * Required, 1 to 50: the most issues to return.
   *
   * @generated from field: uint32 max_results = 4;
   */
  maxResults: number;
};

/**
 * Describes the message numinance.ports.ticketing.v1.SearchIssuesRequest.
 * Use `create(SearchIssuesRequestSchema)` to create a new message.
 */
export const SearchIssuesRequestSchema: GenMessage<SearchIssuesRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 2);

/**
 * IssueSummary is one search hit.
 *
 * @generated from message numinance.ports.ticketing.v1.IssueSummary
 */
export type IssueSummary = Message<"numinance.ports.ticketing.v1.IssueSummary"> & {
  /**
   * The issue key, in the issue key grammar.
   *
   * @generated from field: string issue_key = 1;
   */
  issueKey: string;

  /**
   * The title as plain text, as in GetIssueResponse; untrusted.
   *
   * @generated from field: string title = 2;
   */
  title: string;

  /**
   * The state category of the issue now.
   *
   * @generated from field: numinance.ports.ticketing.v1.IssueStateCategory state = 3;
   */
  state: IssueStateCategory;
};

/**
 * Describes the message numinance.ports.ticketing.v1.IssueSummary.
 * Use `create(IssueSummarySchema)` to create a new message.
 */
export const IssueSummarySchema: GenMessage<IssueSummary> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 3);

/**
 * SearchIssuesResponse is the hits of a search.
 *
 * @generated from message numinance.ports.ticketing.v1.SearchIssuesResponse
 */
export type SearchIssuesResponse = Message<"numinance.ports.ticketing.v1.SearchIssuesResponse"> & {
  /**
   * At most max_results hits, all in the requested project and, when set, the requested state category.
   *
   * @generated from field: repeated numinance.ports.ticketing.v1.IssueSummary issues = 1;
   */
  issues: IssueSummary[];

  /**
   * True when the tracker holds more matches than were returned.
   *
   * @generated from field: bool truncated = 2;
   */
  truncated: boolean;
};

/**
 * Describes the message numinance.ports.ticketing.v1.SearchIssuesResponse.
 * Use `create(SearchIssuesResponseSchema)` to create a new message.
 */
export const SearchIssuesResponseSchema: GenMessage<SearchIssuesResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 4);

/**
 * CommentOnIssueRequest is one comment.
 *
 * @generated from message numinance.ports.ticketing.v1.CommentOnIssueRequest
 */
export type CommentOnIssueRequest = Message<"numinance.ports.ticketing.v1.CommentOnIssueRequest"> & {
  /**
   * Required: the project, in the project grammar. Marked as the resource of the call.
   *
   * @generated from field: string project = 1;
   */
  project: string;

  /**
   * Required: the issue key, in the issue key grammar.
   *
   * @generated from field: string issue_key = 2;
   */
  issueKey: string;

  /**
   * Required, 1 to 8192 bytes of plain text; a provider never interprets it as markup.
   *
   * @generated from field: string body = 3;
   */
  body: string;
};

/**
 * Describes the message numinance.ports.ticketing.v1.CommentOnIssueRequest.
 * Use `create(CommentOnIssueRequestSchema)` to create a new message.
 */
export const CommentOnIssueRequestSchema: GenMessage<CommentOnIssueRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 5);

/**
 * CommentOnIssueResponse names the comment.
 *
 * @generated from message numinance.ports.ticketing.v1.CommentOnIssueResponse
 */
export type CommentOnIssueResponse = Message<"numinance.ports.ticketing.v1.CommentOnIssueResponse"> & {
  /**
   * The tracker's id of the comment, 1 to 128 printable bytes; the same id for a replay under the same key.
   *
   * @generated from field: string comment_id = 1;
   */
  commentId: string;

  /**
   * True when the comment already existed under this key and nothing was added.
   *
   * @generated from field: bool existing = 2;
   */
  existing: boolean;
};

/**
 * Describes the message numinance.ports.ticketing.v1.CommentOnIssueResponse.
 * Use `create(CommentOnIssueResponseSchema)` to create a new message.
 */
export const CommentOnIssueResponseSchema: GenMessage<CommentOnIssueResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 6);

/**
 * TransitionIssueRequest asks for a state category.
 *
 * @generated from message numinance.ports.ticketing.v1.TransitionIssueRequest
 */
export type TransitionIssueRequest = Message<"numinance.ports.ticketing.v1.TransitionIssueRequest"> & {
  /**
   * Required: the project, in the project grammar. Marked as the resource of the call.
   *
   * @generated from field: string project = 1;
   */
  project: string;

  /**
   * Required: the issue key, in the issue key grammar.
   *
   * @generated from field: string issue_key = 2;
   */
  issueKey: string;

  /**
   * Required: the category to move the issue to.
   *
   * @generated from field: numinance.ports.ticketing.v1.IssueStateCategory target = 3;
   */
  target: IssueStateCategory;
};

/**
 * Describes the message numinance.ports.ticketing.v1.TransitionIssueRequest.
 * Use `create(TransitionIssueRequestSchema)` to create a new message.
 */
export const TransitionIssueRequestSchema: GenMessage<TransitionIssueRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 7);

/**
 * TransitionIssueResponse reports the move.
 *
 * @generated from message numinance.ports.ticketing.v1.TransitionIssueResponse
 */
export type TransitionIssueResponse = Message<"numinance.ports.ticketing.v1.TransitionIssueResponse"> & {
  /**
   * The category before the call: the one a compensating TransitionIssue returns to.
   *
   * @generated from field: numinance.ports.ticketing.v1.IssueStateCategory previous = 1;
   */
  previous: IssueStateCategory;

  /**
   * The category after the call; on a dry run the category the call would leave, with nothing changed.
   *
   * @generated from field: numinance.ports.ticketing.v1.IssueStateCategory state = 2;
   */
  state: IssueStateCategory;

  /**
   * True when the call changed the issue (on a dry run: would change it); false when the issue was already in the target category.
   *
   * @generated from field: bool changed = 3;
   */
  changed: boolean;
};

/**
 * Describes the message numinance.ports.ticketing.v1.TransitionIssueResponse.
 * Use `create(TransitionIssueResponseSchema)` to create a new message.
 */
export const TransitionIssueResponseSchema: GenMessage<TransitionIssueResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 8);

/**
 * LinkIssueRequest names the blocking issue and the blocked issue.
 *
 * @generated from message numinance.ports.ticketing.v1.LinkIssueRequest
 */
export type LinkIssueRequest = Message<"numinance.ports.ticketing.v1.LinkIssueRequest"> & {
  /**
   * Required: the project of the first issue. Marked as the resource of the call.
   *
   * @generated from field: string project = 1;
   */
  project: string;

  /**
   * Required: the key of the issue that blocks.
   *
   * @generated from field: string issue_key = 2;
   */
  issueKey: string;

  /**
   * Required: the project of the second issue. Marked as the resource of the call.
   *
   * @generated from field: string blocked_project = 3;
   */
  blockedProject: string;

  /**
   * Required: the key of the issue that is blocked; not the same issue as issue_key.
   *
   * @generated from field: string blocked_issue_key = 4;
   */
  blockedIssueKey: string;

  /**
   * Required: the type of the link.
   *
   * @generated from field: numinance.ports.ticketing.v1.IssueLinkType link_type = 5;
   */
  linkType: IssueLinkType;
};

/**
 * Describes the message numinance.ports.ticketing.v1.LinkIssueRequest.
 * Use `create(LinkIssueRequestSchema)` to create a new message.
 */
export const LinkIssueRequestSchema: GenMessage<LinkIssueRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 9);

/**
 * LinkIssueResponse reports the link.
 *
 * @generated from message numinance.ports.ticketing.v1.LinkIssueResponse
 */
export type LinkIssueResponse = Message<"numinance.ports.ticketing.v1.LinkIssueResponse"> & {
  /**
   * True when the link already existed and nothing was added.
   *
   * @generated from field: bool existing = 1;
   */
  existing: boolean;
};

/**
 * Describes the message numinance.ports.ticketing.v1.LinkIssueResponse.
 * Use `create(LinkIssueResponseSchema)` to create a new message.
 */
export const LinkIssueResponseSchema: GenMessage<LinkIssueResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_ticketing_v1_ticketing, 10);

/**
 * IssueStateCategory is the closed, vendor-neutral category of an issue's workflow state. A provider maps the tracker's own statuses to one of them; the name of the tracker's status is not part of the port.
 *
 * @generated from enum numinance.ports.ticketing.v1.IssueStateCategory
 */
export enum IssueStateCategory {
  /**
   * Zero value; rejected in a request and never returned.
   *
   * @generated from enum value: ISSUE_STATE_CATEGORY_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * Not started: backlog, to do, new.
   *
   * @generated from enum value: ISSUE_STATE_CATEGORY_OPEN = 1;
   */
  OPEN = 1,

  /**
   * Being worked on, including in review.
   *
   * @generated from enum value: ISSUE_STATE_CATEGORY_IN_PROGRESS = 2;
   */
  IN_PROGRESS = 2,

  /**
   * Finished as intended: done, resolved, closed as completed.
   *
   * @generated from enum value: ISSUE_STATE_CATEGORY_DONE = 3;
   */
  DONE = 3,

  /**
   * Finished without being done: canceled, won't do, duplicate.
   *
   * @generated from enum value: ISSUE_STATE_CATEGORY_CANCELED = 4;
   */
  CANCELED = 4,
}

/**
 * Describes the enum numinance.ports.ticketing.v1.IssueStateCategory.
 */
export const IssueStateCategorySchema: GenEnum<IssueStateCategory> = /*@__PURE__*/
  enumDesc(file_numinance_ports_ticketing_v1_ticketing, 0);

/**
 * IssueLinkType is the closed type of an issue-to-issue link.
 *
 * @generated from enum numinance.ports.ticketing.v1.IssueLinkType
 */
export enum IssueLinkType {
  /**
   * Zero value; rejected in a request and never returned.
   *
   * @generated from enum value: ISSUE_LINK_TYPE_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * The first issue blocks the second.
   *
   * @generated from enum value: ISSUE_LINK_TYPE_BLOCKS = 1;
   */
  BLOCKS = 1,
}

/**
 * Describes the enum numinance.ports.ticketing.v1.IssueLinkType.
 */
export const IssueLinkTypeSchema: GenEnum<IssueLinkType> = /*@__PURE__*/
  enumDesc(file_numinance_ports_ticketing_v1_ticketing, 1);

/**
 * TicketingService is the ticketing port an issue-tracker provider serves.
 *
 * @generated from service numinance.ports.ticketing.v1.TicketingService
 */
export const TicketingService: GenService<{
  /**
   * GetIssue returns one issue: its key, its title, its text body bounded by max_body_bytes with a truncated flag, its state category, its type and labels, and the operation ids recorded on it. It is also the reconciliation read of every write after a lost response. The project in the request is the resource of the call, and the issue must be in that project now: an issue that moved to another project is NOT_FOUND here. Every text field of the answer is untrusted. The operation ids are whatever anyone who could edit or comment on the issue wrote there: the Engine accepts one only if it equals the operation_id it computed itself for that Change, and trusts no other.
   *
   * @generated from rpc numinance.ports.ticketing.v1.TicketingService.GetIssue
   */
  getIssue: {
    methodKind: "unary";
    input: typeof GetIssueRequestSchema;
    output: typeof GetIssueResponseSchema;
  },
  /**
   * SearchIssues lists the issues of one project whose title or body contains the text, optionally in one state category, at most max_results of them, in the tracker's order. The text is a parameter: a provider matches it as plain text and never splices it into a query string or a query language, so text that looks like a JQL clause matches only an issue that contains those characters. Query languages are the capability ticketing.query_language, not this verb. Results are untrusted data.
   *
   * @generated from rpc numinance.ports.ticketing.v1.TicketingService.SearchIssues
   */
  searchIssues: {
    methodKind: "unary";
    input: typeof SearchIssuesRequestSchema;
    output: typeof SearchIssuesResponseSchema;
  },
  /**
   * CommentOnIssue adds one plain-text comment, identified by the host's key: a replay answers the first comment and adds none (the provider finds its earlier comment by a marker derived from the key). LOW: a comment changes no workflow and no code, though everyone who can see the issue can read it, so the caller must redact and size-cap the text and consider the audience before it calls; the risk class does not carry that. No dry run: the comment is the whole effect, and a dry run would be the call. No compensating action: this port has no verb that removes a comment, and a correction is a further comment.
   *
   * @generated from rpc numinance.ports.ticketing.v1.TicketingService.CommentOnIssue
   */
  commentOnIssue: {
    methodKind: "unary";
    input: typeof CommentOnIssueRequestSchema;
    output: typeof CommentOnIssueResponseSchema;
  },
  /**
   * TransitionIssue moves an issue to a target state category (OPEN, IN_PROGRESS, DONE or CANCELED). The provider chooses the tracker's own transition that reaches the category; when none does from the current state, the answer is FAILED_PRECONDITION and nothing changes. An issue already in the target category answers done (changed false). MEDIUM: a transition fires the customer's workflow automations, which can notify people, start builds or close related work. Dry run checks that a transition to the target category is available and changes nothing; its answer is the one the call would give. The response carries the category before the call, and the compensating action is a TransitionIssue back to that category.
   *
   * @generated from rpc numinance.ports.ticketing.v1.TicketingService.TransitionIssue
   */
  transitionIssue: {
    methodKind: "unary";
    input: typeof TransitionIssueRequestSchema;
    output: typeof TransitionIssueResponseSchema;
  },
  /**
   * LinkIssue makes an issue-to-issue link of type BLOCKS: the issue named first blocks the issue named second. An existing link of the same type between the same two issues answers done (existing true). LOW: a link changes no workflow state and no text. No dry run: the link is the whole effect and its creation is the check of both keys. No compensating action: this port has no verb that removes a link. Other link types, parent and child links and remote URL links are later additive enum values or capabilities.
   *
   * @generated from rpc numinance.ports.ticketing.v1.TicketingService.LinkIssue
   */
  linkIssue: {
    methodKind: "unary";
    input: typeof LinkIssueRequestSchema;
    output: typeof LinkIssueResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_ticketing_v1_ticketing, 0);

