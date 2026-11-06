// The knowledge port (ADRs 0023 and 0028): read-only search and fetch against one approved KnowledgeConnection.
// Callers: only the knowledge context (the Context Resolver) calls this port. Every verb is READ; the host and the broker refuse any non-READ capability on a knowledge connection, and the port has no VendorCall, since any escape hatch would be a write path. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: the provider reads with the credential handle the broker issued for the connection's auth (KnowledgeConnection.spec.auth), so the source enforces the principal's access at fetch; the port has no access-check verb. The AuthorizationContext is the host's: it is built from the ActionContext and the connection the host dispatched for, its digest goes into the receipt and the cache key, and no field of it travels in a request or is taken from a response. The receipt records access_check server_enforced_at_fetch and access_basis from spec.auth, never from a provider claim. Every request carries scope; its entries must lie within the connection's spec.scope, which the host checks before dispatch by comparing whole segments (an entry equal to a spec.scope entry or below it), never raw string prefixes. Fetch and Resolve carry scope too because a ref is opaque to the host: the provider answers NOT_FOUND for a ref outside scope, so a ref planted in a ticket or produced by a query planner cannot read outside the connection.
// Sandbox and egress: the provider runs sandboxed with default-deny egress; its only destinations are the hosts of its KnowledgeConnection that the manifest allow-list names, and IMDS (169.254.169.254), link-local, loopback and private ranges (except as pinned below) are denied, also when a ref, a redirect or a source answer names them. The egress proxy decides on the resolved address of every connection, every redirect hop included, and connects to the address it checked, never resolving again; it reads every IP literal form (IPv6 such as fd00:ec2::254, IPv4-mapped, integer, octal, a trailing dot) before deciding. HTTPS is required on every connection and every redirect hop, with no downgrade of scheme, unless the connection's approval explicitly allows plain HTTP to a public host, which is then recorded in evidence (a pinned private address never allows it, below). Plain HTTP may be approved only for a connection with no credential, and an approval of plain HTTP for a connection with one is refused; every knowledge connection is HTTPS-only today on two independent grounds: every KnowledgeAuth of 1.0.0 carries a credential, and the loader refuses a KnowledgeConnectionSpec.endpoint that is not "https://" with a DNS host of two or more labels (no IP literal, no .localhost, .local or .internal suffix; numinance/kinds/v1/knowledge_connection.proto), so an on-premises source is a DNS name, as the pinned set assumes. A later credential-free KnowledgeAuth value therefore does not open plain HTTP while that endpoint rule holds; changing it is a contract change of its own. No credential is ever attached to a plain-HTTP request, after a redirect included, so a bearer token never travels in cleartext. IMDS, link-local, loopback and the unspecified addresses (0.0.0.0/8, ::) are always denied, and any IPv6 form that embeds an IPv4 address (IPv4-mapped, IPv4-translated ::ffff:0:0/96, NAT64 64:ff9b::/96 and the local-use 64:ff9b:1::/48 of RFC 8215, 6to4 2002::/16, ISATAP) is judged as that IPv4 address; for a NAT64 prefix the IPv4 address is extracted at the configured prefix length, at the positions of RFC 6052 section 2.2, and the whole 64:ff9b:1::/48 is denied when no prefix length is configured; and a deployment that uses a network-specific NAT64 prefix must configure that prefix in the egress proxy, while Teredo (2001::/32) and IPv4-compatible (::a.b.c.d) addresses are denied outright. A private address, CGNAT 100.64.0.0/10 included, is allowed only as a member of the set of addresses the allow-listed host resolved to when the connection was approved (on-premises sources), each paired with that host; the set comes from the host's trusted resolver, is shown to the approver and holds at most 16 addresses, and an approval resolving to more is refused: the pin admits those addresses for that host name only, never for another destination; HTTPS is required to a pinned private address and is verified against the host name, so plain HTTP to one is refused; each pair is checked at every connect and recorded in evidence; and a connection to an address outside the pinned set fails closed until the connection is re-approved. The sandbox profile is recorded in evidence; a local MCP server runs only under gVisor.
// Query: query is search text only in 1.0.0; vendor query languages such as CQL or JQL are a later capability whose text is parameterised or escaped, never spliced.
// Untrusted answers: every title, snippet, ref, version and content is untrusted data. The knowledge context sanitizes and redacts it before persistence, wraps it as untrusted content, and retrieved text never alters policy, tool lists, grants or approvals.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound, NOT_FOUND for a ref that is absent or that the source hides from the principal, PERMISSION_DENIED when the source refuses the principal openly, FAILED_PRECONDITION for a version the source no longer serves, RESOURCE_EXHAUSTED when the source rate-limits the call, UNAVAILABLE when the source cannot be reached. A provider never invents a result: an outage is an error, never an empty answer.
// Evidence: no verb declares evidence in 1.0.0; the retrieval receipt types are fixed with the knowledge context before the first spec tag. Reads have no dry run because they have no side effect.
// Bounds: every string is at most 1024 bytes unless stated; input or output still out of bounds after the normalisation stated on a field is rejected, never truncated by the host; Fetch content is cut by the provider as max_bytes says, never by the host; repeated fields reject duplicates; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding), so a provider sends a field added in a later minor only when HandshakeRequest.spec_version is at least that minor or behind a negotiated capability. Printable means Unicode categories L, M, N, P, S and U+0020. A ref is 1 to 1024 printable bytes, opaque to the core and never dereferenced by the host as a URL or a path. The provider passes scope entries and refs to the source as literal identifiers, never decodes, unescapes or normalises them, and, when it places a scope entry or a ref into a URL, percent-encodes within each '/'-separated segment every byte outside the RFC 3986 unreserved set, '%' included; '/' stays a separator only where the source's identifiers are paths, and only after the segment rules below hold. A scope entry is 1 to 1024 printable bytes, '/'-separated, with no leading '/', no '\', no '%', no empty, '.' or '..' segment, and no segment whose repeated percent-decoding or NFKC form is empty, '.' or '..' or contains '/' or '\', no segment made only of dots and spaces, with a leading or trailing U+0020, or starting with '..' (such as '..;'); the host rejects any other entry. A text media type is text/*, application/json, application/xml, or a type with a +json or +xml suffix. Every response fits one Invoke message of at most 4 MiB. The host rejects a response that breaks a rule of this file and records the call failed.
// Every element is since 1.0.0.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/knowledge/v1/knowledge.proto (package numinance.ports.knowledge.v1, syntax proto3)
/* eslint-disable */

import type { GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { file_google_protobuf_timestamp } from "@bufbuild/protobuf/wkt";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { Digest } from "../../../types/v1/types_pb";
import { file_numinance_types_v1_types } from "../../../types/v1/types_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/knowledge/v1/knowledge.proto.
 */
export const file_numinance_ports_knowledge_v1_knowledge: GenFile = /*@__PURE__*/
  fileDesc("CixudW1pbmFuY2UvcG9ydHMva25vd2xlZGdlL3YxL2tub3dsZWRnZS5wcm90bxIcbnVtaW5hbmNlLnBvcnRzLmtub3dsZWRnZS52MSJpCg1TZWFyY2hSZXF1ZXN0EhoKBXF1ZXJ5GAEgASgJQguStRgHCgUxLjAuMBIaCgVzY29wZRgCIAMoCUILkrUYBwoFMS4wLjASIAoLbWF4X3Jlc3VsdHMYAyABKA1CC5K1GAcKBTEuMC4wIloKDlNlYXJjaFJlc3BvbnNlEkgKB3Jlc3VsdHMYASADKAsyKi5udW1pbmFuY2UucG9ydHMua25vd2xlZGdlLnYxLlNlYXJjaFJlc3VsdEILkrUYBwoFMS4wLjAihwEKDFNlYXJjaFJlc3VsdBIYCgNyZWYYASABKAlCC5K1GAcKBTEuMC4wEhoKBXRpdGxlGAIgASgJQguStRgHCgUxLjAuMBIcCgdzbmlwcGV0GAMgASgJQguStRgHCgUxLjAuMBIjCg5zb3VyY2VfdmVyc2lvbhgEIAEoCUILkrUYBwoFMS4wLjAiggEKDEZldGNoUmVxdWVzdBIYCgNyZWYYASABKAlCC5K1GAcKBTEuMC4wEhwKB3ZlcnNpb24YAiABKAlCC5K1GAcKBTEuMC4wEh4KCW1heF9ieXRlcxgDIAEoDUILkrUYBwoFMS4wLjASGgoFc2NvcGUYBCADKAlCC5K1GAcKBTEuMC4wIocCCg1GZXRjaFJlc3BvbnNlEhwKB2NvbnRlbnQYASABKAxCC5K1GAcKBTEuMC4wEjcKBmRpZ2VzdBgCIAEoCzIaLm51bWluYW5jZS50eXBlcy52MS5EaWdlc3RCC5K1GAcKBTEuMC4wEiAKC3RvdGFsX2J5dGVzGAMgASgEQguStRgHCgUxLjAuMBIcCgd2ZXJzaW9uGAQgASgJQguStRgHCgUxLjAuMBI+Cg1sYXN0X21vZGlmaWVkGAUgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEILkrUYBwoFMS4wLjASHwoKbWVkaWFfdHlwZRgGIAEoCUILkrUYBwoFMS4wLjAiRgoOUmVzb2x2ZVJlcXVlc3QSGAoDcmVmGAEgASgJQguStRgHCgUxLjAuMBIaCgVzY29wZRgCIAMoCUILkrUYBwoFMS4wLjAiNQoPUmVzb2x2ZVJlc3BvbnNlEiIKDWNhbm9uaWNhbF9yZWYYASABKAlCC5K1GAcKBTEuMC4wMucEChBLbm93bGVkZ2VTZXJ2aWNlEsYBCgZTZWFyY2gSKy5udW1pbmFuY2UucG9ydHMua25vd2xlZGdlLnYxLlNlYXJjaFJlcXVlc3QaLC5udW1pbmFuY2UucG9ydHMua25vd2xlZGdlLnYxLlNlYXJjaFJlc3BvbnNlImGKtRhdCAEQARgBQgUxLjAuMEpOU2VhcmNoZXMgb25lIGFwcHJvdmVkIGtub3dsZWRnZSBzb3VyY2UgYW5kIHJldHVybnMgcmFua2VkIGRvY3VtZW50IHJlZmVyZW5jZXMuErsBCgVGZXRjaBIqLm51bWluYW5jZS5wb3J0cy5rbm93bGVkZ2UudjEuRmV0Y2hSZXF1ZXN0GisubnVtaW5hbmNlLnBvcnRzLmtub3dsZWRnZS52MS5GZXRjaFJlc3BvbnNlIlmKtRhVCAEQARgBQgUxLjAuMEpGUmV0dXJucyB0aGUgY29udGVudCBvZiBvbmUgZG9jdW1lbnQgZnJvbSBhbiBhcHByb3ZlZCBrbm93bGVkZ2Ugc291cmNlLhLLAQoHUmVzb2x2ZRIsLm51bWluYW5jZS5wb3J0cy5rbm93bGVkZ2UudjEuUmVzb2x2ZVJlcXVlc3QaLS5udW1pbmFuY2UucG9ydHMua25vd2xlZGdlLnYxLlJlc29sdmVSZXNwb25zZSJjirUYXwgBEAEYAUIFMS4wLjBKUFJldHVybnMgdGhlIGNhbm9uaWNhbCByZWZlcmVuY2Ugb2Ygb25lIGRvY3VtZW50IGluIGFuIGFwcHJvdmVkIGtub3dsZWRnZSBzb3VyY2UuQp4CCiBjb20ubnVtaW5hbmNlLnBvcnRzLmtub3dsZWRnZS52MUIOS25vd2xlZGdlUHJvdG9QAVpXZ2l0aHViLmNvbS9udW1pbmFuY2UvbnVtaW5hbmNlLXNwZWMvc2RrL2dvL2dlbi9udW1pbmFuY2UvcG9ydHMva25vd2xlZGdlL3YxO2tub3dsZWRnZXYxogIDTlBLqgIcTnVtaW5hbmNlLlBvcnRzLktub3dsZWRnZS5WMcoCHE51bWluYW5jZVxQb3J0c1xLbm93bGVkZ2VcVjHiAihOdW1pbmFuY2VcUG9ydHNcS25vd2xlZGdlXFYxXEdQQk1ldGFkYXRh6gIfTnVtaW5hbmNlOjpQb3J0czo6S25vd2xlZGdlOjpWMWIGcHJvdG8z", [file_google_protobuf_timestamp, file_numinance_annotations_v1_annotations, file_numinance_types_v1_types]);

/**
 * SearchRequest is one search.
 *
 * @generated from message numinance.ports.knowledge.v1.SearchRequest
 */
export type SearchRequest = Message<"numinance.ports.knowledge.v1.SearchRequest"> & {
  /**
   * Required: the search text, 1 to 1024 printable bytes; the provider passes it to the source's search as a parameter, never spliced into a query language.
   *
   * @generated from field: string query = 1;
   */
  query: string;

  /**
   * Required, 1 to 256 unique scope entries: the spaces, projects or relative paths to search, each within the connection's spec.scope.
   *
   * @generated from field: repeated string scope = 2;
   */
  scope: string[];

  /**
   * Most results to return, 1 to 100.
   *
   * @generated from field: uint32 max_results = 3;
   */
  maxResults: number;
};

/**
 * Describes the message numinance.ports.knowledge.v1.SearchRequest.
 * Use `create(SearchRequestSchema)` to create a new message.
 */
export const SearchRequestSchema: GenMessage<SearchRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_knowledge_v1_knowledge, 0);

/**
 * SearchResponse is the ranked results of one search.
 *
 * @generated from message numinance.ports.knowledge.v1.SearchResponse
 */
export type SearchResponse = Message<"numinance.ports.knowledge.v1.SearchResponse"> & {
  /**
   * At most max_results, unique by ref, best first; empty when nothing matched.
   *
   * @generated from field: repeated numinance.ports.knowledge.v1.SearchResult results = 1;
   */
  results: SearchResult[];
};

/**
 * Describes the message numinance.ports.knowledge.v1.SearchResponse.
 * Use `create(SearchResponseSchema)` to create a new message.
 */
export const SearchResponseSchema: GenMessage<SearchResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_knowledge_v1_knowledge, 1);

/**
 * SearchResult is one matching document.
 *
 * @generated from message numinance.ports.knowledge.v1.SearchResult
 */
export type SearchResult = Message<"numinance.ports.knowledge.v1.SearchResult"> & {
  /**
   * Required: the document's ref.
   *
   * @generated from field: string ref = 1;
   */
  ref: string;

  /**
   * The document title, at most 1024 printable bytes; untrusted. The provider replaces any other character with U+FFFD and shortens a longer title at a character boundary.
   *
   * @generated from field: string title = 2;
   */
  title: string;

  /**
   * A matching excerpt, at most 4096 bytes of printable characters, U+0009 and U+000A; untrusted. The provider replaces any other character with U+FFFD and shortens a longer excerpt at a character boundary.
   *
   * @generated from field: string snippet = 3;
   */
  snippet: string;

  /**
   * The source's version or ETag of the document, at most 256 printable bytes; empty when the source has none.
   *
   * @generated from field: string source_version = 4;
   */
  sourceVersion: string;
};

/**
 * Describes the message numinance.ports.knowledge.v1.SearchResult.
 * Use `create(SearchResultSchema)` to create a new message.
 */
export const SearchResultSchema: GenMessage<SearchResult> = /*@__PURE__*/
  messageDesc(file_numinance_ports_knowledge_v1_knowledge, 2);

/**
 * FetchRequest names one document and the most bytes to return.
 *
 * @generated from message numinance.ports.knowledge.v1.FetchRequest
 */
export type FetchRequest = Message<"numinance.ports.knowledge.v1.FetchRequest"> & {
  /**
   * Required: the document's ref.
   *
   * @generated from field: string ref = 1;
   */
  ref: string;

  /**
   * The version to fetch, at most 256 printable bytes; empty for the latest.
   *
   * @generated from field: string version = 2;
   */
  version: string;

  /**
   * Most content bytes to return, 1 to 1048576 (1 MiB); the host sets it from the stage budget and KnowledgeConnection budgets.bytes_per_call.
   *
   * @generated from field: uint32 max_bytes = 3;
   */
  maxBytes: number;

  /**
   * Required, 1 to 256 unique scope entries, each within the connection's spec.scope; the provider answers NOT_FOUND for a ref outside them.
   *
   * @generated from field: repeated string scope = 4;
   */
  scope: string[];
};

/**
 * Describes the message numinance.ports.knowledge.v1.FetchRequest.
 * Use `create(FetchRequestSchema)` to create a new message.
 */
export const FetchRequestSchema: GenMessage<FetchRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_knowledge_v1_knowledge, 3);

/**
 * FetchResponse is the content of one document.
 *
 * @generated from message numinance.ports.knowledge.v1.FetchResponse
 */
export type FetchResponse = Message<"numinance.ports.knowledge.v1.FetchResponse"> & {
  /**
   * The document from its first byte, at most min(max_bytes, total_bytes) bytes, and shorter than total_bytes exactly when cut; for a text media type it is valid UTF-8, cut at a character boundary; untrusted. The host rejects content longer than max_bytes or total_bytes, uncut content whose length is not total_bytes, and invalid UTF-8 for a text media type.
   *
   * @generated from field: bytes content = 1;
   */
  content: Uint8Array;

  /**
   * Required: sha256 of content, the returned bytes, which is the document's digest only when content was not cut; the host recomputes it and refuses a mismatch.
   *
   * @generated from field: numinance.types.v1.Digest digest = 2;
   */
  digest?: Digest | undefined;

  /**
   * Size of the whole document at version in bytes; content is shorter exactly when the document was cut.
   *
   * @generated from field: uint64 total_bytes = 3;
   */
  totalBytes: bigint;

  /**
   * The version served, at most 256 printable bytes; equal to the request's version when one was named, empty when the source has none.
   *
   * @generated from field: string version = 4;
   */
  version: string;

  /**
   * When the source last changed the document; unset when the source does not say.
   *
   * @generated from field: google.protobuf.Timestamp last_modified = 5;
   */
  lastModified?: Timestamp | undefined;

  /**
   * Required: the media type of content, matching ^[a-z0-9][a-z0-9.+-]{0,62}/[a-z0-9][a-z0-9.+-]{0,62}$; the knowledge context refuses a type outside its allow-list.
   *
   * @generated from field: string media_type = 6;
   */
  mediaType: string;
};

/**
 * Describes the message numinance.ports.knowledge.v1.FetchResponse.
 * Use `create(FetchResponseSchema)` to create a new message.
 */
export const FetchResponseSchema: GenMessage<FetchResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_knowledge_v1_knowledge, 4);

/**
 * ResolveRequest names one document.
 *
 * @generated from message numinance.ports.knowledge.v1.ResolveRequest
 */
export type ResolveRequest = Message<"numinance.ports.knowledge.v1.ResolveRequest"> & {
  /**
   * Required: a ref of the document.
   *
   * @generated from field: string ref = 1;
   */
  ref: string;

  /**
   * Required, 1 to 256 unique scope entries, each within the connection's spec.scope; the provider answers NOT_FOUND for a ref outside them.
   *
   * @generated from field: repeated string scope = 2;
   */
  scope: string[];
};

/**
 * Describes the message numinance.ports.knowledge.v1.ResolveRequest.
 * Use `create(ResolveRequestSchema)` to create a new message.
 */
export const ResolveRequestSchema: GenMessage<ResolveRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_knowledge_v1_knowledge, 5);

/**
 * ResolveResponse carries the canonical ref.
 *
 * @generated from message numinance.ports.knowledge.v1.ResolveResponse
 */
export type ResolveResponse = Message<"numinance.ports.knowledge.v1.ResolveResponse"> & {
  /**
   * Required: the canonical ref; resolving it again in the same scope returns it unchanged.
   *
   * @generated from field: string canonical_ref = 1;
   */
  canonicalRef: string;
};

/**
 * Describes the message numinance.ports.knowledge.v1.ResolveResponse.
 * Use `create(ResolveResponseSchema)` to create a new message.
 */
export const ResolveResponseSchema: GenMessage<ResolveResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_knowledge_v1_knowledge, 6);

/**
 * KnowledgeService is the knowledge port a knowledge provider serves.
 *
 * @generated from service numinance.ports.knowledge.v1.KnowledgeService
 */
export const KnowledgeService: GenService<{
  /**
   * Search runs the source's own search for query within scope and returns ranked references, never document bodies.
   *
   * @generated from rpc numinance.ports.knowledge.v1.KnowledgeService.Search
   */
  search: {
    methodKind: "unary";
    input: typeof SearchRequestSchema;
    output: typeof SearchResponseSchema;
  },
  /**
   * Fetch returns the content of one document at a version, from its first byte, up to max_bytes.
   *
   * @generated from rpc numinance.ports.knowledge.v1.KnowledgeService.Fetch
   */
  fetch: {
    methodKind: "unary";
    input: typeof FetchRequestSchema;
    output: typeof FetchResponseSchema;
  },
  /**
   * Resolve returns the canonical ref of a document, so two refs to the same document compare equal and the cache and receipts key on one ref.
   *
   * @generated from rpc numinance.ports.knowledge.v1.KnowledgeService.Resolve
   */
  resolve: {
    methodKind: "unary";
    input: typeof ResolveRequestSchema;
    output: typeof ResolveResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_knowledge_v1_knowledge, 0);

