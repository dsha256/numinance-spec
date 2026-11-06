// The evidencestore port (ADR 0020, refined by ADR 0037): the write-once store of record for every Change's evidence chain, on storage the customer already owns.
// Callers: only the evidence context under the runner identity calls this port. Writes need the grant scope evidence.write and reads evidence.read; only a runner grant carries them, a kernel floors rule denies them to every other actor kind, and the plugin host refuses an Invoke without the scope and an allowing policy decision. The scope names evidence.write, evidence.read and transparencylog.append are reserved to this port and transparencylog: the host refuses to register any other service descriptor that declares them. The port is never a model tool, and no tool_groups are declared. Nothing is updated or deleted: every verb adds an object or reads one.
// Objects: the store keeps opaque bytes. A record is its canonical form (evidence/v1 record.proto, canon_version 2), keyed by (change_id, sequence); the seal of a Change is its last record, an ordinary record whose action is EvidenceSealed.v1 and whose single output is the checkpoint_ref (sealing is a use case of the evidence context, not a verb of this port). The provider never parses, canonicalizes or repairs them, so it cannot tell a seal from another record.
// Writers: the evidence context holds a fenced single-writer lease per Change across the cluster for every create; before each create it checks Head against its own head and the last anchored leaf, and after each create, the seal record included, it reads Head back. Waiting for the lease is normal; a lease not acquired by the step deadline, a lost or fenced lease, or a failed freshness or read-back check is an evidence-write failure and trips the circuit breaker. The provider's conditional create and digest comparisons are a second guard, not the lease.
// Write-once: a create stores the object only if no object exists at its key. Bytes equal to the stored ones are a replay and succeed without writing; any other bytes at a taken key fail with ALREADY_EXISTS and leave the stored object unchanged. Of concurrent creates at one key exactly one is stored, and every create of the stored bytes succeeds. An object is visible to every read that starts after its create returns.
// Order of checks on a create: first the bounds and the integrity rule (INVALID_ARGUMENT); then a taken key, where identical bytes are a replay and other bytes ALREADY_EXISTS, also after the seal record; only then, for a create at a free key, the order rule: FAILED_PRECONDITION for a record at sequence n above 1 unless the stored record at n-1 has previous_content_sha256 as its digest. The order check and the create are one atomic step per Change, so of two creates at one sequence that race, exactly one succeeds. That nothing follows the seal record is the evidence context's rule under its lease, not the provider's, which holds opaque bytes: every reader re-verifies the chain and treats a record after the seal as a provider fault.
// Integrity: every object travels with content_sha256, the sha256 of its bytes. The provider refuses a create whose content_sha256 is absent, malformed or not the digest of its bytes; a write response returns the content_sha256 of the stored object; a provider whose stored bytes no longer hash to their digest answers DATA_LOSS, never the bytes.
// Untrusted answers: the evidence context decodes every object it reads strictly and re-verifies the chain (record_hash, previous_record_hash, signatures, the seal record) before it trusts any of it. It also checks that a decoded record's change_id and sequence are the requested key, that HeadResponse.sequence is the decoded head record's sequence, and that each ListRange page continues the previous one (its first record's previous_record_hash is the record_hash of the last record read). A failed check is a provider fault, never a shorter chain.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound below or the integrity rule, ALREADY_EXISTS for a taken key with other bytes, FAILED_PRECONDITION for a create out of order, NOT_FOUND for an absent record, DATA_LOSS for stored bytes that fail their digest, UNAVAILABLE for a storage failure the caller may retry. A provider never reports a write it did not make.
// Bounds: change_id matches ^chg_[0-9a-f]{32}$; sequence is 1 to 2^53-1; previous_content_sha256 is a well-formed digest, required above sequence 1 and absent at sequence 1; these bounds are checked first, before the taken-key step; a record object, the seal record included, is 1 to 3145728 bytes (3 MiB, the evidence domain's MaxRecordBytes, checked before signing); a ListRange span is at most 256 sequences. So every response fits one PluginService Invoke message of at most 4 MiB. Over-limit input is rejected, never truncated.
// Dry run: no verb supports it, as each write RPC states. No verb declares evidence: the store is the evidence of record, and the evidence context records its own writes in the chain.
// Every element is since 1.0.0.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/evidencestore/v1/evidencestore.proto (package numinance.ports.evidencestore.v1, syntax proto3)
/* eslint-disable */

import type { GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { Digest } from "../../../types/v1/types_pb";
import { file_numinance_types_v1_types } from "../../../types/v1/types_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/evidencestore/v1/evidencestore.proto.
 */
export const file_numinance_ports_evidencestore_v1_evidencestore: GenFile = /*@__PURE__*/
  fileDesc("CjRudW1pbmFuY2UvcG9ydHMvZXZpZGVuY2VzdG9yZS92MS9ldmlkZW5jZXN0b3JlLnByb3RvEiBudW1pbmFuY2UucG9ydHMuZXZpZGVuY2VzdG9yZS52MSJtCgxTdG9yZWRPYmplY3QSHAoHY29udGVudBgBIAEoDEILkrUYBwoFMS4wLjASPwoOY29udGVudF9zaGEyNTYYAiABKAsyGi5udW1pbmFuY2UudHlwZXMudjEuRGlnZXN0QguStRgHCgUxLjAuMCLrAQoTQXBwZW5kTG9ja2VkUmVxdWVzdBIeCgljaGFuZ2VfaWQYASABKAlCC5K1GAcKBTEuMC4wEh0KCHNlcXVlbmNlGAIgASgEQguStRgHCgUxLjAuMBJLCgZyZWNvcmQYAyABKAsyLi5udW1pbmFuY2UucG9ydHMuZXZpZGVuY2VzdG9yZS52MS5TdG9yZWRPYmplY3RCC5K1GAcKBTEuMC4wEkgKF3ByZXZpb3VzX2NvbnRlbnRfc2hhMjU2GAQgASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjAidgoUQXBwZW5kTG9ja2VkUmVzcG9uc2USHQoIcmVwbGF5ZWQYASABKAhCC5K1GAcKBTEuMC4wEj8KDmNvbnRlbnRfc2hhMjU2GAIgASgLMhoubnVtaW5hbmNlLnR5cGVzLnYxLkRpZ2VzdEILkrUYBwoFMS4wLjAiUQoQR2V0UmVjb3JkUmVxdWVzdBIeCgljaGFuZ2VfaWQYASABKAlCC5K1GAcKBTEuMC4wEh0KCHNlcXVlbmNlGAIgASgEQguStRgHCgUxLjAuMCJgChFHZXRSZWNvcmRSZXNwb25zZRJLCgZyZWNvcmQYASABKAsyLi5udW1pbmFuY2UucG9ydHMuZXZpZGVuY2VzdG9yZS52MS5TdG9yZWRPYmplY3RCC5K1GAcKBTEuMC4wIngKEExpc3RSYW5nZVJlcXVlc3QSHgoJY2hhbmdlX2lkGAEgASgJQguStRgHCgUxLjAuMBIiCg1mcm9tX3NlcXVlbmNlGAIgASgEQguStRgHCgUxLjAuMBIgCgt0b19zZXF1ZW5jZRgDIAEoBEILkrUYBwoFMS4wLjAiYQoRTGlzdFJhbmdlUmVzcG9uc2USTAoHcmVjb3JkcxgBIAMoCzIuLm51bWluYW5jZS5wb3J0cy5ldmlkZW5jZXN0b3JlLnYxLkxpc3RlZFJlY29yZEILkrUYBwoFMS4wLjAiegoMTGlzdGVkUmVjb3JkEh0KCHNlcXVlbmNlGAEgASgEQguStRgHCgUxLjAuMBJLCgZyZWNvcmQYAiABKAsyLi5udW1pbmFuY2UucG9ydHMuZXZpZGVuY2VzdG9yZS52MS5TdG9yZWRPYmplY3RCC5K1GAcKBTEuMC4wIi0KC0hlYWRSZXF1ZXN0Eh4KCWNoYW5nZV9pZBgBIAEoCUILkrUYBwoFMS4wLjAiiwEKDEhlYWRSZXNwb25zZRIdCghzZXF1ZW5jZRgBIAEoBEILkrUYBwoFMS4wLjASUAoLaGVhZF9yZWNvcmQYAiABKAsyLi5udW1pbmFuY2UucG9ydHMuZXZpZGVuY2VzdG9yZS52MS5TdG9yZWRPYmplY3RCC5K1GAcKBTEuMC4wSgQIAxAEUgRzZWFsMocHChRFdmlkZW5jZVN0b3JlU2VydmljZRLtAQoMQXBwZW5kTG9ja2VkEjUubnVtaW5hbmNlLnBvcnRzLmV2aWRlbmNlc3RvcmUudjEuQXBwZW5kTG9ja2VkUmVxdWVzdBo2Lm51bWluYW5jZS5wb3J0cy5ldmlkZW5jZXN0b3JlLnYxLkFwcGVuZExvY2tlZFJlc3BvbnNlIm6KtRhqCAQQARgBQgUxLjAuMEpLU3RvcmVzIG9uZSBjYW5vbmljYWwgZXZpZGVuY2UgcmVjb3JkIHdyaXRlLW9uY2UgYXQgaXRzIENoYW5nZSBhbmQgc2VxdWVuY2UuWg5ldmlkZW5jZS53cml0ZRLTAQoJR2V0UmVjb3JkEjIubnVtaW5hbmNlLnBvcnRzLmV2aWRlbmNlc3RvcmUudjEuR2V0UmVjb3JkUmVxdWVzdBozLm51bWluYW5jZS5wb3J0cy5ldmlkZW5jZXN0b3JlLnYxLkdldFJlY29yZFJlc3BvbnNlIl2KtRhZCAEQARgBQgUxLjAuMEo7UmV0dXJucyBvbmUgc3RvcmVkIGV2aWRlbmNlIHJlY29yZCBvZiBhIENoYW5nZSBieSBzZXF1ZW5jZS5aDWV2aWRlbmNlLnJlYWQS4AEKCUxpc3RSYW5nZRIyLm51bWluYW5jZS5wb3J0cy5ldmlkZW5jZXN0b3JlLnYxLkxpc3RSYW5nZVJlcXVlc3QaMy5udW1pbmFuY2UucG9ydHMuZXZpZGVuY2VzdG9yZS52MS5MaXN0UmFuZ2VSZXNwb25zZSJqirUYZggBEAEYAUIFMS4wLjBKSFJldHVybnMgYSBwYWdlIG9mIHN0b3JlZCBldmlkZW5jZSByZWNvcmRzIG9mIGEgQ2hhbmdlIGluIHNlcXVlbmNlIG9yZGVyLloNZXZpZGVuY2UucmVhZBLFAQoESGVhZBItLm51bWluYW5jZS5wb3J0cy5ldmlkZW5jZXN0b3JlLnYxLkhlYWRSZXF1ZXN0Gi4ubnVtaW5hbmNlLnBvcnRzLmV2aWRlbmNlc3RvcmUudjEuSGVhZFJlc3BvbnNlIl6KtRhaCAEQARgBQgUxLjAuMEo8UmV0dXJucyB0aGUgbGFzdCBzdG9yZWQgcmVjb3JkIG9mIGEgQ2hhbmdlJ3MgZXZpZGVuY2UgY2hhaW4uWg1ldmlkZW5jZS5yZWFkQr4CCiRjb20ubnVtaW5hbmNlLnBvcnRzLmV2aWRlbmNlc3RvcmUudjFCEkV2aWRlbmNlc3RvcmVQcm90b1ABWl9naXRodWIuY29tL251bWluYW5jZS9udW1pbmFuY2Utc3BlYy9zZGsvZ28vZ2VuL251bWluYW5jZS9wb3J0cy9ldmlkZW5jZXN0b3JlL3YxO2V2aWRlbmNlc3RvcmV2MaICA05QRaoCIE51bWluYW5jZS5Qb3J0cy5FdmlkZW5jZXN0b3JlLlYxygIgTnVtaW5hbmNlXFBvcnRzXEV2aWRlbmNlc3RvcmVcVjHiAixOdW1pbmFuY2VcUG9ydHNcRXZpZGVuY2VzdG9yZVxWMVxHUEJNZXRhZGF0YeoCI051bWluYW5jZTo6UG9ydHM6OkV2aWRlbmNlc3RvcmU6OlYxYgZwcm90bzM", [file_numinance_annotations_v1_annotations, file_numinance_types_v1_types]);

/**
 * StoredObject is one object as the store keeps it: its bytes and their digest.
 *
 * @generated from message numinance.ports.evidencestore.v1.StoredObject
 */
export type StoredObject = Message<"numinance.ports.evidencestore.v1.StoredObject"> & {
  /**
   * Canonical bytes of the object, never parsed by the provider: 1 to 3145728 bytes.
   *
   * @generated from field: bytes content = 1;
   */
  content: Uint8Array;

  /**
   * sha256 of content; required on a create, which is refused when it is absent, malformed or another digest; a reader refuses a mismatch.
   *
   * @generated from field: numinance.types.v1.Digest content_sha256 = 2;
   */
  contentSha256?: Digest | undefined;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.StoredObject.
 * Use `create(StoredObjectSchema)` to create a new message.
 */
export const StoredObjectSchema: GenMessage<StoredObject> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 0);

/**
 * AppendLockedRequest names the key of one record and carries its canonical bytes.
 *
 * @generated from message numinance.ports.evidencestore.v1.AppendLockedRequest
 */
export type AppendLockedRequest = Message<"numinance.ports.evidencestore.v1.AppendLockedRequest"> & {
  /**
   * Change the record belongs to, ^chg_[0-9a-f]{32}$.
   *
   * @generated from field: string change_id = 1;
   */
  changeId: string;

  /**
   * 1-based position of the record in the chain, 1 to 2^53-1.
   *
   * @generated from field: uint64 sequence = 2;
   */
  sequence: bigint;

  /**
   * The record in its canonical form.
   *
   * @generated from field: numinance.ports.evidencestore.v1.StoredObject record = 3;
   */
  record?: StoredObject | undefined;

  /**
   * content_sha256 of the stored record at sequence-1, required above sequence 1 and absent at sequence 1, so the predecessor is compared by content, not only by presence.
   *
   * @generated from field: numinance.types.v1.Digest previous_content_sha256 = 4;
   */
  previousContentSha256?: Digest | undefined;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.AppendLockedRequest.
 * Use `create(AppendLockedRequestSchema)` to create a new message.
 */
export const AppendLockedRequestSchema: GenMessage<AppendLockedRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 1);

/**
 * AppendLockedResponse reports that the record is stored, by this call or by an identical earlier one.
 *
 * @generated from message numinance.ports.evidencestore.v1.AppendLockedResponse
 */
export type AppendLockedResponse = Message<"numinance.ports.evidencestore.v1.AppendLockedResponse"> & {
  /**
   * True when the call found identical bytes already stored and wrote nothing.
   *
   * @generated from field: bool replayed = 1;
   */
  replayed: boolean;

  /**
   * sha256 of the stored object; the caller refuses a response where it differs from the request's.
   *
   * @generated from field: numinance.types.v1.Digest content_sha256 = 2;
   */
  contentSha256?: Digest | undefined;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.AppendLockedResponse.
 * Use `create(AppendLockedResponseSchema)` to create a new message.
 */
export const AppendLockedResponseSchema: GenMessage<AppendLockedResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 2);

/**
 * GetRecordRequest names one record.
 *
 * @generated from message numinance.ports.evidencestore.v1.GetRecordRequest
 */
export type GetRecordRequest = Message<"numinance.ports.evidencestore.v1.GetRecordRequest"> & {
  /**
   * Change the record belongs to, ^chg_[0-9a-f]{32}$.
   *
   * @generated from field: string change_id = 1;
   */
  changeId: string;

  /**
   * 1-based position of the record in the chain, 1 to 2^53-1.
   *
   * @generated from field: uint64 sequence = 2;
   */
  sequence: bigint;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.GetRecordRequest.
 * Use `create(GetRecordRequestSchema)` to create a new message.
 */
export const GetRecordRequestSchema: GenMessage<GetRecordRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 3);

/**
 * GetRecordResponse carries one stored record.
 *
 * @generated from message numinance.ports.evidencestore.v1.GetRecordResponse
 */
export type GetRecordResponse = Message<"numinance.ports.evidencestore.v1.GetRecordResponse"> & {
  /**
   * The stored record as kept.
   *
   * @generated from field: numinance.ports.evidencestore.v1.StoredObject record = 1;
   */
  record?: StoredObject | undefined;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.GetRecordResponse.
 * Use `create(GetRecordResponseSchema)` to create a new message.
 */
export const GetRecordResponseSchema: GenMessage<GetRecordResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 4);

/**
 * ListRangeRequest names a span of sequences of one Change.
 *
 * @generated from message numinance.ports.evidencestore.v1.ListRangeRequest
 */
export type ListRangeRequest = Message<"numinance.ports.evidencestore.v1.ListRangeRequest"> & {
  /**
   * Change whose records are listed, ^chg_[0-9a-f]{32}$.
   *
   * @generated from field: string change_id = 1;
   */
  changeId: string;

  /**
   * First sequence of the span, 1 to 2^53-1.
   *
   * @generated from field: uint64 from_sequence = 2;
   */
  fromSequence: bigint;

  /**
   * Last sequence of the span, at least from_sequence, less than from_sequence plus 256 and at most 2^53-1.
   *
   * @generated from field: uint64 to_sequence = 3;
   */
  toSequence: bigint;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.ListRangeRequest.
 * Use `create(ListRangeRequestSchema)` to create a new message.
 */
export const ListRangeRequestSchema: GenMessage<ListRangeRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 5);

/**
 * ListRangeResponse carries one page of stored records.
 *
 * @generated from message numinance.ports.evidencestore.v1.ListRangeResponse
 */
export type ListRangeResponse = Message<"numinance.ports.evidencestore.v1.ListRangeResponse"> & {
  /**
   * Stored records from from_sequence on, consecutive and in sequence order.
   *
   * @generated from field: repeated numinance.ports.evidencestore.v1.ListedRecord records = 1;
   */
  records: ListedRecord[];
};

/**
 * Describes the message numinance.ports.evidencestore.v1.ListRangeResponse.
 * Use `create(ListRangeResponseSchema)` to create a new message.
 */
export const ListRangeResponseSchema: GenMessage<ListRangeResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 6);

/**
 * ListedRecord is one stored record with its sequence.
 *
 * @generated from message numinance.ports.evidencestore.v1.ListedRecord
 */
export type ListedRecord = Message<"numinance.ports.evidencestore.v1.ListedRecord"> & {
  /**
   * 1-based position of the record in the chain.
   *
   * @generated from field: uint64 sequence = 1;
   */
  sequence: bigint;

  /**
   * The stored record as kept.
   *
   * @generated from field: numinance.ports.evidencestore.v1.StoredObject record = 2;
   */
  record?: StoredObject | undefined;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.ListedRecord.
 * Use `create(ListedRecordSchema)` to create a new message.
 */
export const ListedRecordSchema: GenMessage<ListedRecord> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 7);

/**
 * HeadRequest names one Change.
 *
 * @generated from message numinance.ports.evidencestore.v1.HeadRequest
 */
export type HeadRequest = Message<"numinance.ports.evidencestore.v1.HeadRequest"> & {
  /**
   * Change whose head is read, ^chg_[0-9a-f]{32}$.
   *
   * @generated from field: string change_id = 1;
   */
  changeId: string;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.HeadRequest.
 * Use `create(HeadRequestSchema)` to create a new message.
 */
export const HeadRequestSchema: GenMessage<HeadRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 8);

/**
 * HeadResponse is the stored state of one Change's chain.
 *
 * @generated from message numinance.ports.evidencestore.v1.HeadResponse
 */
export type HeadResponse = Message<"numinance.ports.evidencestore.v1.HeadResponse"> & {
  /**
   * Highest stored sequence, 0 for a Change with no records.
   *
   * @generated from field: uint64 sequence = 1;
   */
  sequence: bigint;

  /**
   * The record at sequence, unset when sequence is 0; the reader takes the head record_hash from it, never from a provider claim.
   *
   * @generated from field: numinance.ports.evidencestore.v1.StoredObject head_record = 2;
   */
  headRecord?: StoredObject | undefined;
};

/**
 * Describes the message numinance.ports.evidencestore.v1.HeadResponse.
 * Use `create(HeadResponseSchema)` to create a new message.
 */
export const HeadResponseSchema: GenMessage<HeadResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_evidencestore_v1_evidencestore, 9);

/**
 * EvidenceStoreService is the evidencestore port a storage provider serves.
 *
 * @generated from service numinance.ports.evidencestore.v1.EvidenceStoreService
 */
export const EvidenceStoreService: GenService<{
  /**
   * AppendLocked creates the record object at (change_id, sequence) while the evidence context holds the Change's fenced single-writer lease; it is irreversible by design, hence HIGH; a replay of identical bytes succeeds without writing. No dry run: the record is fully built and checked by the caller before the call, there is no plan to preview, and a replay is a no-op.
   *
   * @generated from rpc numinance.ports.evidencestore.v1.EvidenceStoreService.AppendLocked
   */
  appendLocked: {
    methodKind: "unary";
    input: typeof AppendLockedRequestSchema;
    output: typeof AppendLockedResponseSchema;
  },
  /**
   * GetRecord returns the record object at (change_id, sequence), or NOT_FOUND.
   *
   * @generated from rpc numinance.ports.evidencestore.v1.EvidenceStoreService.GetRecord
   */
  getRecord: {
    methodKind: "unary";
    input: typeof GetRecordRequestSchema;
    output: typeof GetRecordResponseSchema;
  },
  /**
   * ListRange returns the stored records from from_sequence to to_sequence in sequence order. The page stops early at the last stored sequence, or before the record that would take the page past 3145728 bytes of content; a non-empty page holds at least one record, and an early stop is not the end of the chain. A page is empty only when from_sequence is past the last stored sequence; an empty page below the head is a provider fault.
   *
   * @generated from rpc numinance.ports.evidencestore.v1.EvidenceStoreService.ListRange
   */
  listRange: {
    methodKind: "unary";
    input: typeof ListRangeRequestSchema;
    output: typeof ListRangeResponseSchema;
  },
  /**
   * Head returns the highest stored sequence of a Change, the record stored there (the seal record once the chain is sealed), both from one snapshot; the zero response for a Change with no records.
   *
   * @generated from rpc numinance.ports.evidencestore.v1.EvidenceStoreService.Head
   */
  head: {
    methodKind: "unary";
    input: typeof HeadRequestSchema;
    output: typeof HeadResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_evidencestore_v1_evidencestore, 0);

