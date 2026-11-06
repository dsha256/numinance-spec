// The transparencylog port (ADR 0020, refined by ADR 0037): an append-only Merkle log that anchors chain heads and attestations, served by a log that signs its own checkpoints: a customer-local ledger, Rekor (public or private) or a Tessera-based C2SP log. No provider re-signs another log's checkpoints.
// Callers: only the evidence context under the runner identity calls this port. Append needs the grant scope transparencylog.append and every read evidence.read; only a runner grant carries them, a kernel floors rule denies them to every other actor kind, and the plugin host refuses an Invoke without the scope and an allowing policy decision. The scope names are reserved (see the evidencestore port). The port is never a model tool, and no tool_groups are declared. Nothing is updated or deleted, and the leaf at an index never changes.
// Leaves: a leaf is exactly 33 bytes, kind 0x48 (chain head) or 0x41 (attestation) then a raw 32-byte hash (evidence/v1 inclusion_proof.proto). The log's Merkle leaf is its own entry, not the 33-byte leaf, so Append and Prove return the exact entry bytes the log hashed and the format that names its encoding. The verifier computes the leaf hash itself from those bytes (RFC 6962 for numinance.leaf.v1, otherwise as the format says) and never takes a hash from the provider.
// Entry formats are a closed list in this spec, one id per wire encoding, each with one strict decoder that rejects duplicate keys and unknown fields: numinance.leaf.v1 (the 33-byte leaf itself, for the customer-local ledger), dev.sigstore.hashedrekord.v0_0_1.json (the Rekor v1 canonical-JSON body) and dev.sigstore.hashedrekord.v0_0_2 (the Rekor v2 encoding). entry_format is compared as exact bytes: ASCII, at most 64 bytes, checked against the closed list before any decoding, and every entry of one log origin uses one format. A format the verifier does not hold is refused. Adding a format is contract-additive. A hashedrekord entry is signed by the separate anchoring key (ECDSA P-256, held in the customer's KMS under the runner identity) over the domain-separated input "numinance.evidence.v1.Anchor" || sha256(leaf); the decoder, not the provider, checks the entry's key, signature, hash algorithm (sha256) and digest. The anchoring key is never held, resolved or used by a provider: the evidence context signs under the runner identity and sends the signature and public key in Append (see AppendRequest); a provider that signs on its own behalf fails the kit.
// Carries the leaf: for a provider, an entry carries a leaf when the leaf bytes (numinance.leaf.v1) or the anchor digest of the leaf (hashedrekord) match, nothing more; the key, signature and algorithm checks are the verifier's. On a shared log a foreign entry may sit at an index before ours, so a verifier's refusal of the entry at a Lookup index is not a provider fault and the index Append returned is the fallback. After an ambiguous Append failure (the call may or may not have written) the evidence context re-resolves through Lookup when the log offers it and otherwise accepts a duplicate entry (the retry rule is on Append); the plugin host never retries Append automatically.
// Checkpoints: Checkpoint and Prove return the log's own checkpoint unchanged, as a C2SP signed note (bytes). The verifier checks it against a log origin and public key (Ed25519 or ECDSA P-256) from its own trust set, never from the provider. A signed note carries no timestamp: log freshness is tree-size monotonicity plus consistency, never a provider timestamp. Time comes from the format (the local ledger's signed extension line, a witness cosignature or an RFC 3161 timestamp). The verifier, not the provider, refuses a note whose tree size is below that of the last note it holds for the origin, since a stateless provider cannot know it. min_tree_size and the sizes of a consistency Prove refer to the tree of the origin of the last-held note the caller holds (a request names no origin; the provider is the caller's handle to one log); on first use the caller passes leaf_index + 1.
// Older shapes: evidence/v1 InclusionProof and Checkpoint still describe the previous checkpoint shape (a Numinance-signed message with a timestamp) until a later release replaces them; this port does not use evidence/v1 Checkpoint.
// Untrusted answers: every note, entry and proof is untrusted until verified. The evidence context accepts an inclusion proof only after its checkpoint is verified against the trust set and proven consistent, through a consistency Prove, with the last checkpoint it holds; a failed check is a fork and an evidence-write failure that trips the circuit breaker. leaf_index is only a hint until a verified Prove, and any verified entry carrying the leaf is enough.
// Durability and growth: a leaf is in every checkpoint taken after Append returns. Every checkpoint extends the earlier ones (its tree holds theirs as a prefix); the tree size never decreases.
// Capabilities, negotiated at handshake and not Core: transparencylog.lookup (the Lookup verb; Rekor v2 has no search index), transparencylog.dedup (a replayed Append returns the first index and writes nothing) and transparencylog.witness (checkpoints carry witness cosignatures, inside the note). Without dedup, a replayed Append may add a further entry with the same leaf and return its index.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound below, NOT_FOUND for Checkpoint on an empty log, for Prove when no entry carrying the leaf is at leaf_index below the returned tree size, and for Lookup of a leaf not in the log, UNAVAILABLE for a log failure the caller may retry and for a size the log does not (yet) serve, on Prove (min_tree_size or leaf_index + 1 above the log's size) and on a consistency Prove (new_tree_size above the log's size); transient, bounded by the caller's deadline and the circuit breaker. INVALID_ARGUMENT only for bound violations, never for a size merely not yet reached.
// Bounds: a signed note is 1 to 65536 bytes with at most 16 signature lines; an entry is 1 to 65536 bytes; a signature is at most 72 bytes and a public key at most 128 bytes; a proof path has at most 64 entries and exactly the length its sizes and index imply; tree sizes are 1 to 2^53-1; leaf_index is 0 to 2^53-2; entry_format is one of the three ids above. Over-limit input is rejected before it is decoded, never truncated, so every response fits one PluginService Invoke message of at most 4 MiB.
// Dry run: no verb supports it; a leaf is fully determined by the caller, there is nothing to preview, and the reads change nothing. No verb declares evidence: the log is itself evidence of record, and the evidence context records each anchor (log origin, entry format, witness status) in the chain.
// Every element is since 1.0.0.

// @generated by protoc-gen-es v2.16.0 with parameter "target=ts"
// @generated from file numinance/ports/transparencylog/v1/transparencylog.proto (package numinance.ports.transparencylog.v1, syntax proto3)
/* eslint-disable */

import type { GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import { fileDesc, messageDesc, serviceDesc } from "@bufbuild/protobuf/codegenv2";
import { file_numinance_annotations_v1_annotations } from "../../../annotations/v1/annotations_pb";
import type { InclusionProof } from "../../../evidence/v1/inclusion_proof_pb";
import { file_numinance_evidence_v1_inclusion_proof } from "../../../evidence/v1/inclusion_proof_pb";
import type { Digest } from "../../../types/v1/types_pb";
import { file_numinance_types_v1_types } from "../../../types/v1/types_pb";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file numinance/ports/transparencylog/v1/transparencylog.proto.
 */
export const file_numinance_ports_transparencylog_v1_transparencylog: GenFile = /*@__PURE__*/
  fileDesc("CjhudW1pbmFuY2UvcG9ydHMvdHJhbnNwYXJlbmN5bG9nL3YxL3RyYW5zcGFyZW5jeWxvZy5wcm90bxIibnVtaW5hbmNlLnBvcnRzLnRyYW5zcGFyZW5jeWxvZy52MSJ8Cg1BcHBlbmRSZXF1ZXN0EhkKBGxlYWYYASABKAxCC5K1GAcKBTEuMC4wEiUKEGFuY2hvcl9zaWduYXR1cmUYAiABKAxCC5K1GAcKBTEuMC4wEikKFGFuY2hvcmluZ19wdWJsaWNfa2V5GAMgASgMQguStRgHCgUxLjAuMCJwCg5BcHBlbmRSZXNwb25zZRIfCgpsZWFmX2luZGV4GAEgASgEQguStRgHCgUxLjAuMBIaCgVlbnRyeRgCIAEoDEILkrUYBwoFMS4wLjASIQoMZW50cnlfZm9ybWF0GAMgASgJQguStRgHCgUxLjAuMCITChFDaGVja3BvaW50UmVxdWVzdCIvChJDaGVja3BvaW50UmVzcG9uc2USGQoEbm90ZRgBIAEoDEILkrUYBwoFMS4wLjAi8QEKDFByb3ZlUmVxdWVzdBJWCglpbmNsdXNpb24YBCABKAsyNC5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxLkluY2x1c2lvblJlcXVlc3RCC5K1GAcKBTEuMC4wSAASWgoLY29uc2lzdGVuY3kYBSABKAsyNi5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxLkNvbnNpc3RlbmN5UmVxdWVzdEILkrUYBwoFMS4wLjBIAEIGCgRraW5kSgQIARAEUgpsZWFmX2luZGV4UgRsZWFmUg1taW5fdHJlZV9zaXplInIKEEluY2x1c2lvblJlcXVlc3QSHwoKbGVhZl9pbmRleBgBIAEoBEILkrUYBwoFMS4wLjASGQoEbGVhZhgCIAEoDEILkrUYBwoFMS4wLjASIgoNbWluX3RyZWVfc2l6ZRgDIAEoBEILkrUYBwoFMS4wLjAiXAoSQ29uc2lzdGVuY3lSZXF1ZXN0EiIKDW9sZF90cmVlX3NpemUYASABKARCC5K1GAcKBTEuMC4wEiIKDW5ld190cmVlX3NpemUYAiABKARCC5K1GAcKBTEuMC4wIvYBCg1Qcm92ZVJlc3BvbnNlElUKCWluY2x1c2lvbhgFIAEoCzIzLm51bWluYW5jZS5wb3J0cy50cmFuc3BhcmVuY3lsb2cudjEuSW5jbHVzaW9uUmVzdWx0QguStRgHCgUxLjAuMEgAElgKC2NvbnNpc3RlbmN5GAYgASgLMjQubnVtaW5hbmNlLnBvcnRzLnRyYW5zcGFyZW5jeWxvZy52MS5Db25zaXN0ZW5jeVByb29mQguStRgHCgUxLjAuMEgAQgYKBGtpbmRKBAgBEAVSBXByb29mUgVlbnRyeVIMZW50cnlfZm9ybWF0UgpjaGVja3BvaW50IrQBCg9JbmNsdXNpb25SZXN1bHQSQQoFcHJvb2YYASABKAsyJS5udW1pbmFuY2UuZXZpZGVuY2UudjEuSW5jbHVzaW9uUHJvb2ZCC5K1GAcKBTEuMC4wEhoKBWVudHJ5GAIgASgMQguStRgHCgUxLjAuMBIhCgxlbnRyeV9mb3JtYXQYAyABKAlCC5K1GAcKBTEuMC4wEh8KCmNoZWNrcG9pbnQYBCABKAxCC5K1GAcKBTEuMC4wIpEBChBDb25zaXN0ZW5jeVByb29mEiIKDW9sZF90cmVlX3NpemUYASABKARCC5K1GAcKBTEuMC4wEiIKDW5ld190cmVlX3NpemUYAiABKARCC5K1GAcKBTEuMC4wEjUKBHBhdGgYAyADKAsyGi5udW1pbmFuY2UudHlwZXMudjEuRGlnZXN0QguStRgHCgUxLjAuMCIqCg1Mb29rdXBSZXF1ZXN0EhkKBGxlYWYYASABKAxCC5K1GAcKBTEuMC4wIjEKDkxvb2t1cFJlc3BvbnNlEh8KCmxlYWZfaW5kZXgYASABKARCC5K1GAcKBTEuMC4wMuYHChZUcmFuc3BhcmVuY3lMb2dTZXJ2aWNlEvIBCgZBcHBlbmQSMS5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxLkFwcGVuZFJlcXVlc3QaMi5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxLkFwcGVuZFJlc3BvbnNlIoABirUYfAgEEAEYAUIFMS4wLjBKVUFwcGVuZHMgb25lIDMzLWJ5dGUgbGVhZiB0byB0aGUgdHJhbnNwYXJlbmN5IGxvZyBhbmQgcmV0dXJucyBpdHMgaW5kZXggYW5kIGxvZyBlbnRyeS5aFnRyYW5zcGFyZW5jeWxvZy5hcHBlbmQS4gEKCkNoZWNrcG9pbnQSNS5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxLkNoZWNrcG9pbnRSZXF1ZXN0GjYubnVtaW5hbmNlLnBvcnRzLnRyYW5zcGFyZW5jeWxvZy52MS5DaGVja3BvaW50UmVzcG9uc2UiZYq1GGEIARABGAFCBTEuMC4wSkNSZXR1cm5zIHRoZSB0cmFuc3BhcmVuY3kgbG9nJ3Mgc2lnbmVkIGNoZWNrcG9pbnQgbm90ZSBhcyBpdCBzdGFuZHMuWg1ldmlkZW5jZS5yZWFkEpECCgVQcm92ZRIwLm51bWluYW5jZS5wb3J0cy50cmFuc3BhcmVuY3lsb2cudjEuUHJvdmVSZXF1ZXN0GjEubnVtaW5hbmNlLnBvcnRzLnRyYW5zcGFyZW5jeWxvZy52MS5Qcm92ZVJlc3BvbnNlIqIBirUYnQEIARABGAFCBTEuMC4wSn9SZXR1cm5zIHRoZSBpbmNsdXNpb24gcHJvb2YsIGxvZyBlbnRyeSBhbmQgY2hlY2twb2ludCBmb3Igb25lIGxlYWYsIG9yIHRoZSBjb25zaXN0ZW5jeSBwcm9vZiBiZXR3ZWVuIHR3byBjaGVja3BvaW50IHRyZWUgc2l6ZXMuWg1ldmlkZW5jZS5yZWFkEt0BCgZMb29rdXASMS5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxLkxvb2t1cFJlcXVlc3QaMi5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxLkxvb2t1cFJlc3BvbnNlImyKtRhoCAEQARgBOhZ0cmFuc3BhcmVuY3lsb2cubG9va3VwQgUxLjAuMEoyRmluZHMgdGhlIGluZGV4IG9mIGEgbGVhZiBpbiB0aGUgdHJhbnNwYXJlbmN5IGxvZy5aDWV2aWRlbmNlLnJlYWRCzgIKJmNvbS5udW1pbmFuY2UucG9ydHMudHJhbnNwYXJlbmN5bG9nLnYxQhRUcmFuc3BhcmVuY3lsb2dQcm90b1ABWmNnaXRodWIuY29tL251bWluYW5jZS9udW1pbmFuY2Utc3BlYy9zZGsvZ28vZ2VuL251bWluYW5jZS9wb3J0cy90cmFuc3BhcmVuY3lsb2cvdjE7dHJhbnNwYXJlbmN5bG9ndjGiAgNOUFSqAiJOdW1pbmFuY2UuUG9ydHMuVHJhbnNwYXJlbmN5bG9nLlYxygIiTnVtaW5hbmNlXFBvcnRzXFRyYW5zcGFyZW5jeWxvZ1xWMeICLk51bWluYW5jZVxQb3J0c1xUcmFuc3BhcmVuY3lsb2dcVjFcR1BCTWV0YWRhdGHqAiVOdW1pbmFuY2U6OlBvcnRzOjpUcmFuc3BhcmVuY3lsb2c6OlYxYgZwcm90bzM", [file_numinance_annotations_v1_annotations, file_numinance_evidence_v1_inclusion_proof, file_numinance_types_v1_types]);

/**
 * AppendRequest carries one leaf.
 *
 * @generated from message numinance.ports.transparencylog.v1.AppendRequest
 */
export type AppendRequest = Message<"numinance.ports.transparencylog.v1.AppendRequest"> & {
  /**
   * The leaf, exactly 33 bytes: kind 0x48 (chain head) or 0x41 (attestation), then the raw 32-byte hash.
   *
   * @generated from field: bytes leaf = 1;
   */
  leaf: Uint8Array;

  /**
   * ECDSA P-256 signature, DER, at most 72 bytes, over sha256 of "numinance.evidence.v1.Anchor" || sha256(leaf), made by the evidence context under the runner identity with the anchoring key from the customer's KMS. Required by the hashedrekord formats and ignored by numinance.leaf.v1. It SHOULD be deterministic (RFC 6979) so a replayed Append yields the same entry and transparencylog.dedup can hold. The provider only forwards it and never holds, resolves or uses an anchoring key.
   *
   * @generated from field: bytes anchor_signature = 2;
   */
  anchorSignature: Uint8Array;

  /**
   * The anchoring public key, SubjectPublicKeyInfo DER, at most 128 bytes, that verifies anchor_signature. Required by the hashedrekord formats and ignored by numinance.leaf.v1; the verifier accepts the entry only if this key is the anchoring key in its trust set.
   *
   * @generated from field: bytes anchoring_public_key = 3;
   */
  anchoringPublicKey: Uint8Array;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.AppendRequest.
 * Use `create(AppendRequestSchema)` to create a new message.
 */
export const AppendRequestSchema: GenMessage<AppendRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 0);

/**
 * AppendResponse names where the leaf is and what the log hashed.
 *
 * @generated from message numinance.ports.transparencylog.v1.AppendResponse
 */
export type AppendResponse = Message<"numinance.ports.transparencylog.v1.AppendResponse"> & {
  /**
   * 0-based index of the entry in the log, 0 to 2^53-2; a hint until a verified Prove.
   *
   * @generated from field: uint64 leaf_index = 1;
   */
  leafIndex: bigint;

  /**
   * The exact bytes the log hashed as its Merkle leaf, 1 to 65536 bytes, encoded as entry_format says; the verifier computes the leaf hash from them and checks that they carry the appended leaf.
   *
   * @generated from field: bytes entry = 2;
   */
  entry: Uint8Array;

  /**
   * Id of the entry encoding, one of numinance.leaf.v1, dev.sigstore.hashedrekord.v0_0_1.json and dev.sigstore.hashedrekord.v0_0_2; the verifier refuses any other.
   *
   * @generated from field: string entry_format = 3;
   */
  entryFormat: string;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.AppendResponse.
 * Use `create(AppendResponseSchema)` to create a new message.
 */
export const AppendResponseSchema: GenMessage<AppendResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 1);

/**
 * CheckpointRequest asks for the current checkpoint; it has no fields.
 *
 * @generated from message numinance.ports.transparencylog.v1.CheckpointRequest
 */
export type CheckpointRequest = Message<"numinance.ports.transparencylog.v1.CheckpointRequest"> & {
};

/**
 * Describes the message numinance.ports.transparencylog.v1.CheckpointRequest.
 * Use `create(CheckpointRequestSchema)` to create a new message.
 */
export const CheckpointRequestSchema: GenMessage<CheckpointRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 2);

/**
 * CheckpointResponse carries the log's own checkpoint.
 *
 * @generated from message numinance.ports.transparencylog.v1.CheckpointResponse
 */
export type CheckpointResponse = Message<"numinance.ports.transparencylog.v1.CheckpointResponse"> & {
  /**
   * The C2SP signed note exactly as the log signed it, 1 to 65536 bytes with at most 16 signature lines; untrusted until checked against a trust-set origin and key.
   *
   * @generated from field: bytes note = 1;
   */
  note: Uint8Array;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.CheckpointResponse.
 * Use `create(CheckpointResponseSchema)` to create a new message.
 */
export const CheckpointResponseSchema: GenMessage<CheckpointResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 3);

/**
 * ProveRequest asks for one proof: of a leaf's inclusion or of the consistency of two tree sizes.
 *
 * @generated from message numinance.ports.transparencylog.v1.ProveRequest
 */
export type ProveRequest = Message<"numinance.ports.transparencylog.v1.ProveRequest"> & {
  /**
   * Exactly one kind is set.
   *
   * @generated from oneof numinance.ports.transparencylog.v1.ProveRequest.kind
   */
  kind: {
    /**
     * The inclusion proof of one leaf.
     *
     * @generated from field: numinance.ports.transparencylog.v1.InclusionRequest inclusion = 4;
     */
    value: InclusionRequest;
    case: "inclusion";
  } | {
    /**
     * The consistency proof between two checkpoint tree sizes the caller holds.
     *
     * @generated from field: numinance.ports.transparencylog.v1.ConsistencyRequest consistency = 5;
     */
    value: ConsistencyRequest;
    case: "consistency";
  } | { case: undefined; value?: undefined };
};

/**
 * Describes the message numinance.ports.transparencylog.v1.ProveRequest.
 * Use `create(ProveRequestSchema)` to create a new message.
 */
export const ProveRequestSchema: GenMessage<ProveRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 4);

/**
 * InclusionRequest names an entry by its index hint, its leaf and the smallest tree size the caller accepts.
 *
 * @generated from message numinance.ports.transparencylog.v1.InclusionRequest
 */
export type InclusionRequest = Message<"numinance.ports.transparencylog.v1.InclusionRequest"> & {
  /**
   * 0-based index of the entry, 0 to 2^53-2, as returned by Append or Lookup.
   *
   * @generated from field: uint64 leaf_index = 1;
   */
  leafIndex: bigint;

  /**
   * The leaf, exactly 33 bytes; the entry returned must carry it.
   *
   * @generated from field: bytes leaf = 2;
   */
  leaf: Uint8Array;

  /**
   * The returned checkpoint has at least this tree size, 1 to 2^53-1; the caller passes the size of the last checkpoint it holds, so a stale answer is detectable.
   *
   * @generated from field: uint64 min_tree_size = 3;
   */
  minTreeSize: bigint;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.InclusionRequest.
 * Use `create(InclusionRequestSchema)` to create a new message.
 */
export const InclusionRequestSchema: GenMessage<InclusionRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 5);

/**
 * ConsistencyRequest names the tree sizes of two checkpoints the caller holds.
 *
 * @generated from message numinance.ports.transparencylog.v1.ConsistencyRequest
 */
export type ConsistencyRequest = Message<"numinance.ports.transparencylog.v1.ConsistencyRequest"> & {
  /**
   * tree_size of the earlier checkpoint, 1 to new_tree_size.
   *
   * @generated from field: uint64 old_tree_size = 1;
   */
  oldTreeSize: bigint;

  /**
   * tree_size of the later checkpoint, at least old_tree_size and at most 2^53-1.
   *
   * @generated from field: uint64 new_tree_size = 2;
   */
  newTreeSize: bigint;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.ConsistencyRequest.
 * Use `create(ConsistencyRequestSchema)` to create a new message.
 */
export const ConsistencyRequestSchema: GenMessage<ConsistencyRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 6);

/**
 * ProveResponse carries the proof that matches the request kind.
 *
 * @generated from message numinance.ports.transparencylog.v1.ProveResponse
 */
export type ProveResponse = Message<"numinance.ports.transparencylog.v1.ProveResponse"> & {
  /**
   * Exactly one kind is set, the one the request named.
   *
   * @generated from oneof numinance.ports.transparencylog.v1.ProveResponse.kind
   */
  kind: {
    /**
     * Answer to an inclusion request.
     *
     * @generated from field: numinance.ports.transparencylog.v1.InclusionResult inclusion = 5;
     */
    value: InclusionResult;
    case: "inclusion";
  } | {
    /**
     * Answer to a consistency request; its sizes equal the request's.
     *
     * @generated from field: numinance.ports.transparencylog.v1.ConsistencyProof consistency = 6;
     */
    value: ConsistencyProof;
    case: "consistency";
  } | { case: undefined; value?: undefined };
};

/**
 * Describes the message numinance.ports.transparencylog.v1.ProveResponse.
 * Use `create(ProveResponseSchema)` to create a new message.
 */
export const ProveResponseSchema: GenMessage<ProveResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 7);

/**
 * InclusionResult carries an inclusion proof with the entry and checkpoint it relates to.
 *
 * @generated from message numinance.ports.transparencylog.v1.InclusionResult
 */
export type InclusionResult = Message<"numinance.ports.transparencylog.v1.InclusionResult"> & {
  /**
   * The proof; its tree_size equals the checkpoint's and its leaf_index is the entry's.
   *
   * @generated from field: numinance.evidence.v1.InclusionProof proof = 1;
   */
  proof?: InclusionProof | undefined;

  /**
   * The exact bytes the log hashed at leaf_index, 1 to 65536 bytes, encoded as entry_format says.
   *
   * @generated from field: bytes entry = 2;
   */
  entry: Uint8Array;

  /**
   * Id of the entry encoding, as in AppendResponse.
   *
   * @generated from field: string entry_format = 3;
   */
  entryFormat: string;

  /**
   * The log's own signed note the proof was made against, 1 to 65536 bytes with at most 16 signature lines; its tree size is at least min_tree_size.
   *
   * @generated from field: bytes checkpoint = 4;
   */
  checkpoint: Uint8Array;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.InclusionResult.
 * Use `create(InclusionResultSchema)` to create a new message.
 */
export const InclusionResultSchema: GenMessage<InclusionResult> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 8);

/**
 * ConsistencyProof shows that the tree of new_tree_size leaves holds the tree of old_tree_size leaves as a prefix (RFC 9162 section 2.1.4). No evidence record carries a consistency proof: the evidence context verifies it online against the last-held checkpoint of the origin and keeps nothing of it, so it is a message of this port, not of evidence/v1. It is unsigned and checked against two verified notes.
 *
 * @generated from message numinance.ports.transparencylog.v1.ConsistencyProof
 */
export type ConsistencyProof = Message<"numinance.ports.transparencylog.v1.ConsistencyProof"> & {
  /**
   * tree_size of the earlier checkpoint, 1 to new_tree_size.
   *
   * @generated from field: uint64 old_tree_size = 1;
   */
  oldTreeSize: bigint;

  /**
   * tree_size of the later checkpoint.
   *
   * @generated from field: uint64 new_tree_size = 2;
   */
  newTreeSize: bigint;

  /**
   * Subtree hashes, at most 64 and exactly as many as the two sizes imply; empty when the sizes are equal.
   *
   * @generated from field: repeated numinance.types.v1.Digest path = 3;
   */
  path: Digest[];
};

/**
 * Describes the message numinance.ports.transparencylog.v1.ConsistencyProof.
 * Use `create(ConsistencyProofSchema)` to create a new message.
 */
export const ConsistencyProofSchema: GenMessage<ConsistencyProof> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 9);

/**
 * LookupRequest names a leaf.
 *
 * @generated from message numinance.ports.transparencylog.v1.LookupRequest
 */
export type LookupRequest = Message<"numinance.ports.transparencylog.v1.LookupRequest"> & {
  /**
   * The leaf, exactly 33 bytes.
   *
   * @generated from field: bytes leaf = 1;
   */
  leaf: Uint8Array;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.LookupRequest.
 * Use `create(LookupRequestSchema)` to create a new message.
 */
export const LookupRequestSchema: GenMessage<LookupRequest> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 10);

/**
 * LookupResponse names where the leaf is.
 *
 * @generated from message numinance.ports.transparencylog.v1.LookupResponse
 */
export type LookupResponse = Message<"numinance.ports.transparencylog.v1.LookupResponse"> & {
  /**
   * 0-based index of the first entry carrying the leaf, a hint until a verified Prove.
   *
   * @generated from field: uint64 leaf_index = 1;
   */
  leafIndex: bigint;
};

/**
 * Describes the message numinance.ports.transparencylog.v1.LookupResponse.
 * Use `create(LookupResponseSchema)` to create a new message.
 */
export const LookupResponseSchema: GenMessage<LookupResponse> = /*@__PURE__*/
  messageDesc(file_numinance_ports_transparencylog_v1_transparencylog, 11);

/**
 * TransparencyLogService is the transparencylog port a log provider serves.
 *
 * @generated from service numinance.ports.transparencylog.v1.TransparencyLogService
 */
export const TransparencyLogService: GenService<{
  /**
   * Append adds a leaf to the end of the log and returns its 0-based index (a hint until a verified Prove), the exact entry the log hashed and the entry's format; irreversible by design, hence HIGH. Idempotent by the caller's retry rule, not by the log: the caller records the leaf_index it received and never re-appends a recorded leaf; after an unknown outcome (the call may or may not have written) it calls Lookup first where transparencylog.lookup is offered, and appends again only when Lookup answers NOT_FOUND. Without transparencylog.dedup a further Append of the same leaf may add another entry carrying it, which is harmless to verification; with it a replay returns the first index and writes nothing. Where Lookup is not offered, the caller appends again and accepts a possible duplicate entry. idempotent: true here means replay-safe under the caller's retry rule above, as numinance.annotations.v1.ActionOptions.idempotent allows for a caller-owned rule; it must not be read as provider-side dedup, which only transparencylog.dedup declares. No dry run: the leaf is fully built by the caller, there is no plan to preview.
   *
   * @generated from rpc numinance.ports.transparencylog.v1.TransparencyLogService.Append
   */
  append: {
    methodKind: "unary";
    input: typeof AppendRequestSchema;
    output: typeof AppendResponseSchema;
  },
  /**
   * Checkpoint returns the log's own latest checkpoint as a C2SP signed note, unchanged and never re-signed; NOT_FOUND for an empty log.
   *
   * @generated from rpc numinance.ports.transparencylog.v1.TransparencyLogService.Checkpoint
   */
  checkpoint: {
    methodKind: "unary";
    input: typeof CheckpointRequestSchema;
    output: typeof CheckpointResponseSchema;
  },
  /**
   * Prove returns one proof, as its request asks. An inclusion request returns the inclusion proof of the leaf at leaf_index, the entry stored there and the checkpoint the proof was made against, whose tree size is at least min_tree_size and above leaf_index. A consistency request returns the consistency proof from the tree of the first old_tree_size leaves to the tree of the first new_tree_size; old_tree_size 0, old_tree_size above new_tree_size or a size above 2^53-1 is INVALID_ARGUMENT, a new_tree_size past the log's size is UNAVAILABLE; equal sizes have the empty path. A request with no kind set is INVALID_ARGUMENT (a oneof cannot carry two). The consistency fields keep the names of ADR 0037 decision 4 and the domain code (old_tree_size, new_tree_size). Consistency lives under Prove, not as a verb of its own.
   *
   * @generated from rpc numinance.ports.transparencylog.v1.TransparencyLogService.Prove
   */
  prove: {
    methodKind: "unary";
    input: typeof ProveRequestSchema;
    output: typeof ProveResponseSchema;
  },
  /**
   * Lookup returns the index of the first entry carrying the leaf, or NOT_FOUND. Gated by transparencylog.lookup: a log without a search index does not serve it, and the caller keeps the index Append returned.
   *
   * @generated from rpc numinance.ports.transparencylog.v1.TransparencyLogService.Lookup
   */
  lookup: {
    methodKind: "unary";
    input: typeof LookupRequestSchema;
    output: typeof LookupResponseSchema;
  },
}> = /*@__PURE__*/
  serviceDesc(file_numinance_ports_transparencylog_v1_transparencylog, 0);

