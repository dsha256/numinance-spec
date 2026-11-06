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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/transparencylog/v1/transparencylog.proto

package transparencylogv1

import (
	_ "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/evidence/v1"
	v11 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// AppendRequest carries one leaf.
type AppendRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The leaf, exactly 33 bytes: kind 0x48 (chain head) or 0x41 (attestation), then the raw 32-byte hash.
	Leaf []byte `protobuf:"bytes,1,opt,name=leaf,proto3" json:"leaf,omitempty"`
	// ECDSA P-256 signature, DER, at most 72 bytes, over sha256 of "numinance.evidence.v1.Anchor" || sha256(leaf), made by the evidence context under the runner identity with the anchoring key from the customer's KMS. Required by the hashedrekord formats and ignored by numinance.leaf.v1. It SHOULD be deterministic (RFC 6979) so a replayed Append yields the same entry and transparencylog.dedup can hold. The provider only forwards it and never holds, resolves or uses an anchoring key.
	AnchorSignature []byte `protobuf:"bytes,2,opt,name=anchor_signature,json=anchorSignature,proto3" json:"anchor_signature,omitempty"`
	// The anchoring public key, SubjectPublicKeyInfo DER, at most 128 bytes, that verifies anchor_signature. Required by the hashedrekord formats and ignored by numinance.leaf.v1; the verifier accepts the entry only if this key is the anchoring key in its trust set.
	AnchoringPublicKey []byte `protobuf:"bytes,3,opt,name=anchoring_public_key,json=anchoringPublicKey,proto3" json:"anchoring_public_key,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *AppendRequest) Reset() {
	*x = AppendRequest{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AppendRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AppendRequest) ProtoMessage() {}

func (x *AppendRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AppendRequest.ProtoReflect.Descriptor instead.
func (*AppendRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{0}
}

func (x *AppendRequest) GetLeaf() []byte {
	if x != nil {
		return x.Leaf
	}
	return nil
}

func (x *AppendRequest) GetAnchorSignature() []byte {
	if x != nil {
		return x.AnchorSignature
	}
	return nil
}

func (x *AppendRequest) GetAnchoringPublicKey() []byte {
	if x != nil {
		return x.AnchoringPublicKey
	}
	return nil
}

// AppendResponse names where the leaf is and what the log hashed.
type AppendResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// 0-based index of the entry in the log, 0 to 2^53-2; a hint until a verified Prove.
	LeafIndex uint64 `protobuf:"varint,1,opt,name=leaf_index,json=leafIndex,proto3" json:"leaf_index,omitempty"`
	// The exact bytes the log hashed as its Merkle leaf, 1 to 65536 bytes, encoded as entry_format says; the verifier computes the leaf hash from them and checks that they carry the appended leaf.
	Entry []byte `protobuf:"bytes,2,opt,name=entry,proto3" json:"entry,omitempty"`
	// Id of the entry encoding, one of numinance.leaf.v1, dev.sigstore.hashedrekord.v0_0_1.json and dev.sigstore.hashedrekord.v0_0_2; the verifier refuses any other.
	EntryFormat   string `protobuf:"bytes,3,opt,name=entry_format,json=entryFormat,proto3" json:"entry_format,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AppendResponse) Reset() {
	*x = AppendResponse{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AppendResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AppendResponse) ProtoMessage() {}

func (x *AppendResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AppendResponse.ProtoReflect.Descriptor instead.
func (*AppendResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{1}
}

func (x *AppendResponse) GetLeafIndex() uint64 {
	if x != nil {
		return x.LeafIndex
	}
	return 0
}

func (x *AppendResponse) GetEntry() []byte {
	if x != nil {
		return x.Entry
	}
	return nil
}

func (x *AppendResponse) GetEntryFormat() string {
	if x != nil {
		return x.EntryFormat
	}
	return ""
}

// CheckpointRequest asks for the current checkpoint; it has no fields.
type CheckpointRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CheckpointRequest) Reset() {
	*x = CheckpointRequest{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CheckpointRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CheckpointRequest) ProtoMessage() {}

func (x *CheckpointRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CheckpointRequest.ProtoReflect.Descriptor instead.
func (*CheckpointRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{2}
}

// CheckpointResponse carries the log's own checkpoint.
type CheckpointResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The C2SP signed note exactly as the log signed it, 1 to 65536 bytes with at most 16 signature lines; untrusted until checked against a trust-set origin and key.
	Note          []byte `protobuf:"bytes,1,opt,name=note,proto3" json:"note,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CheckpointResponse) Reset() {
	*x = CheckpointResponse{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CheckpointResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CheckpointResponse) ProtoMessage() {}

func (x *CheckpointResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CheckpointResponse.ProtoReflect.Descriptor instead.
func (*CheckpointResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{3}
}

func (x *CheckpointResponse) GetNote() []byte {
	if x != nil {
		return x.Note
	}
	return nil
}

// ProveRequest asks for one proof: of a leaf's inclusion or of the consistency of two tree sizes.
type ProveRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Exactly one kind is set.
	//
	// Types that are valid to be assigned to Kind:
	//
	//	*ProveRequest_Inclusion
	//	*ProveRequest_Consistency
	Kind          isProveRequest_Kind `protobuf_oneof:"kind"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProveRequest) Reset() {
	*x = ProveRequest{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProveRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProveRequest) ProtoMessage() {}

func (x *ProveRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProveRequest.ProtoReflect.Descriptor instead.
func (*ProveRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{4}
}

func (x *ProveRequest) GetKind() isProveRequest_Kind {
	if x != nil {
		return x.Kind
	}
	return nil
}

func (x *ProveRequest) GetInclusion() *InclusionRequest {
	if x != nil {
		if x, ok := x.Kind.(*ProveRequest_Inclusion); ok {
			return x.Inclusion
		}
	}
	return nil
}

func (x *ProveRequest) GetConsistency() *ConsistencyRequest {
	if x != nil {
		if x, ok := x.Kind.(*ProveRequest_Consistency); ok {
			return x.Consistency
		}
	}
	return nil
}

type isProveRequest_Kind interface {
	isProveRequest_Kind()
}

type ProveRequest_Inclusion struct {
	// The inclusion proof of one leaf.
	Inclusion *InclusionRequest `protobuf:"bytes,4,opt,name=inclusion,proto3,oneof"`
}

type ProveRequest_Consistency struct {
	// The consistency proof between two checkpoint tree sizes the caller holds.
	Consistency *ConsistencyRequest `protobuf:"bytes,5,opt,name=consistency,proto3,oneof"`
}

func (*ProveRequest_Inclusion) isProveRequest_Kind() {}

func (*ProveRequest_Consistency) isProveRequest_Kind() {}

// InclusionRequest names an entry by its index hint, its leaf and the smallest tree size the caller accepts.
type InclusionRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// 0-based index of the entry, 0 to 2^53-2, as returned by Append or Lookup.
	LeafIndex uint64 `protobuf:"varint,1,opt,name=leaf_index,json=leafIndex,proto3" json:"leaf_index,omitempty"`
	// The leaf, exactly 33 bytes; the entry returned must carry it.
	Leaf []byte `protobuf:"bytes,2,opt,name=leaf,proto3" json:"leaf,omitempty"`
	// The returned checkpoint has at least this tree size, 1 to 2^53-1; the caller passes the size of the last checkpoint it holds, so a stale answer is detectable.
	MinTreeSize   uint64 `protobuf:"varint,3,opt,name=min_tree_size,json=minTreeSize,proto3" json:"min_tree_size,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *InclusionRequest) Reset() {
	*x = InclusionRequest{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InclusionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InclusionRequest) ProtoMessage() {}

func (x *InclusionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InclusionRequest.ProtoReflect.Descriptor instead.
func (*InclusionRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{5}
}

func (x *InclusionRequest) GetLeafIndex() uint64 {
	if x != nil {
		return x.LeafIndex
	}
	return 0
}

func (x *InclusionRequest) GetLeaf() []byte {
	if x != nil {
		return x.Leaf
	}
	return nil
}

func (x *InclusionRequest) GetMinTreeSize() uint64 {
	if x != nil {
		return x.MinTreeSize
	}
	return 0
}

// ConsistencyRequest names the tree sizes of two checkpoints the caller holds.
type ConsistencyRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// tree_size of the earlier checkpoint, 1 to new_tree_size.
	OldTreeSize uint64 `protobuf:"varint,1,opt,name=old_tree_size,json=oldTreeSize,proto3" json:"old_tree_size,omitempty"`
	// tree_size of the later checkpoint, at least old_tree_size and at most 2^53-1.
	NewTreeSize   uint64 `protobuf:"varint,2,opt,name=new_tree_size,json=newTreeSize,proto3" json:"new_tree_size,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConsistencyRequest) Reset() {
	*x = ConsistencyRequest{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConsistencyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConsistencyRequest) ProtoMessage() {}

func (x *ConsistencyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConsistencyRequest.ProtoReflect.Descriptor instead.
func (*ConsistencyRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{6}
}

func (x *ConsistencyRequest) GetOldTreeSize() uint64 {
	if x != nil {
		return x.OldTreeSize
	}
	return 0
}

func (x *ConsistencyRequest) GetNewTreeSize() uint64 {
	if x != nil {
		return x.NewTreeSize
	}
	return 0
}

// ProveResponse carries the proof that matches the request kind.
type ProveResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Exactly one kind is set, the one the request named.
	//
	// Types that are valid to be assigned to Kind:
	//
	//	*ProveResponse_Inclusion
	//	*ProveResponse_Consistency
	Kind          isProveResponse_Kind `protobuf_oneof:"kind"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProveResponse) Reset() {
	*x = ProveResponse{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProveResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProveResponse) ProtoMessage() {}

func (x *ProveResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProveResponse.ProtoReflect.Descriptor instead.
func (*ProveResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{7}
}

func (x *ProveResponse) GetKind() isProveResponse_Kind {
	if x != nil {
		return x.Kind
	}
	return nil
}

func (x *ProveResponse) GetInclusion() *InclusionResult {
	if x != nil {
		if x, ok := x.Kind.(*ProveResponse_Inclusion); ok {
			return x.Inclusion
		}
	}
	return nil
}

func (x *ProveResponse) GetConsistency() *ConsistencyProof {
	if x != nil {
		if x, ok := x.Kind.(*ProveResponse_Consistency); ok {
			return x.Consistency
		}
	}
	return nil
}

type isProveResponse_Kind interface {
	isProveResponse_Kind()
}

type ProveResponse_Inclusion struct {
	// Answer to an inclusion request.
	Inclusion *InclusionResult `protobuf:"bytes,5,opt,name=inclusion,proto3,oneof"`
}

type ProveResponse_Consistency struct {
	// Answer to a consistency request; its sizes equal the request's.
	Consistency *ConsistencyProof `protobuf:"bytes,6,opt,name=consistency,proto3,oneof"`
}

func (*ProveResponse_Inclusion) isProveResponse_Kind() {}

func (*ProveResponse_Consistency) isProveResponse_Kind() {}

// InclusionResult carries an inclusion proof with the entry and checkpoint it relates to.
type InclusionResult struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The proof; its tree_size equals the checkpoint's and its leaf_index is the entry's.
	Proof *v1.InclusionProof `protobuf:"bytes,1,opt,name=proof,proto3" json:"proof,omitempty"`
	// The exact bytes the log hashed at leaf_index, 1 to 65536 bytes, encoded as entry_format says.
	Entry []byte `protobuf:"bytes,2,opt,name=entry,proto3" json:"entry,omitempty"`
	// Id of the entry encoding, as in AppendResponse.
	EntryFormat string `protobuf:"bytes,3,opt,name=entry_format,json=entryFormat,proto3" json:"entry_format,omitempty"`
	// The log's own signed note the proof was made against, 1 to 65536 bytes with at most 16 signature lines; its tree size is at least min_tree_size.
	Checkpoint    []byte `protobuf:"bytes,4,opt,name=checkpoint,proto3" json:"checkpoint,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *InclusionResult) Reset() {
	*x = InclusionResult{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InclusionResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InclusionResult) ProtoMessage() {}

func (x *InclusionResult) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InclusionResult.ProtoReflect.Descriptor instead.
func (*InclusionResult) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{8}
}

func (x *InclusionResult) GetProof() *v1.InclusionProof {
	if x != nil {
		return x.Proof
	}
	return nil
}

func (x *InclusionResult) GetEntry() []byte {
	if x != nil {
		return x.Entry
	}
	return nil
}

func (x *InclusionResult) GetEntryFormat() string {
	if x != nil {
		return x.EntryFormat
	}
	return ""
}

func (x *InclusionResult) GetCheckpoint() []byte {
	if x != nil {
		return x.Checkpoint
	}
	return nil
}

// ConsistencyProof shows that the tree of new_tree_size leaves holds the tree of old_tree_size leaves as a prefix (RFC 9162 section 2.1.4). No evidence record carries a consistency proof: the evidence context verifies it online against the last-held checkpoint of the origin and keeps nothing of it, so it is a message of this port, not of evidence/v1. It is unsigned and checked against two verified notes.
type ConsistencyProof struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// tree_size of the earlier checkpoint, 1 to new_tree_size.
	OldTreeSize uint64 `protobuf:"varint,1,opt,name=old_tree_size,json=oldTreeSize,proto3" json:"old_tree_size,omitempty"`
	// tree_size of the later checkpoint.
	NewTreeSize uint64 `protobuf:"varint,2,opt,name=new_tree_size,json=newTreeSize,proto3" json:"new_tree_size,omitempty"`
	// Subtree hashes, at most 64 and exactly as many as the two sizes imply; empty when the sizes are equal.
	Path          []*v11.Digest `protobuf:"bytes,3,rep,name=path,proto3" json:"path,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConsistencyProof) Reset() {
	*x = ConsistencyProof{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConsistencyProof) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConsistencyProof) ProtoMessage() {}

func (x *ConsistencyProof) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConsistencyProof.ProtoReflect.Descriptor instead.
func (*ConsistencyProof) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{9}
}

func (x *ConsistencyProof) GetOldTreeSize() uint64 {
	if x != nil {
		return x.OldTreeSize
	}
	return 0
}

func (x *ConsistencyProof) GetNewTreeSize() uint64 {
	if x != nil {
		return x.NewTreeSize
	}
	return 0
}

func (x *ConsistencyProof) GetPath() []*v11.Digest {
	if x != nil {
		return x.Path
	}
	return nil
}

// LookupRequest names a leaf.
type LookupRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The leaf, exactly 33 bytes.
	Leaf          []byte `protobuf:"bytes,1,opt,name=leaf,proto3" json:"leaf,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LookupRequest) Reset() {
	*x = LookupRequest{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LookupRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LookupRequest) ProtoMessage() {}

func (x *LookupRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LookupRequest.ProtoReflect.Descriptor instead.
func (*LookupRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{10}
}

func (x *LookupRequest) GetLeaf() []byte {
	if x != nil {
		return x.Leaf
	}
	return nil
}

// LookupResponse names where the leaf is.
type LookupResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// 0-based index of the first entry carrying the leaf, a hint until a verified Prove.
	LeafIndex     uint64 `protobuf:"varint,1,opt,name=leaf_index,json=leafIndex,proto3" json:"leaf_index,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LookupResponse) Reset() {
	*x = LookupResponse{}
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LookupResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LookupResponse) ProtoMessage() {}

func (x *LookupResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LookupResponse.ProtoReflect.Descriptor instead.
func (*LookupResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP(), []int{11}
}

func (x *LookupResponse) GetLeafIndex() uint64 {
	if x != nil {
		return x.LeafIndex
	}
	return 0
}

var File_numinance_ports_transparencylog_v1_transparencylog_proto protoreflect.FileDescriptor

const file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDesc = "" +
	"\n" +
	"8numinance/ports/transparencylog/v1/transparencylog.proto\x12\"numinance.ports.transparencylog.v1\x1a*numinance/annotations/v1/annotations.proto\x1a+numinance/evidence/v1/inclusion_proof.proto\x1a\x1enuminance/types/v1/types.proto\"\xa7\x01\n" +
	"\rAppendRequest\x12\x1f\n" +
	"\x04leaf\x18\x01 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04leaf\x126\n" +
	"\x10anchor_signature\x18\x02 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x0fanchorSignature\x12=\n" +
	"\x14anchoring_public_key\x18\x03 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x12anchoringPublicKey\"\x8f\x01\n" +
	"\x0eAppendResponse\x12*\n" +
	"\n" +
	"leaf_index\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tleafIndex\x12!\n" +
	"\x05entry\x18\x02 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05entry\x12.\n" +
	"\fentry_format\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ventryFormat\"\x13\n" +
	"\x11CheckpointRequest\"5\n" +
	"\x12CheckpointResponse\x12\x1f\n" +
	"\x04note\x18\x01 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04note\"\x89\x02\n" +
	"\fProveRequest\x12a\n" +
	"\tinclusion\x18\x04 \x01(\v24.numinance.ports.transparencylog.v1.InclusionRequestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0H\x00R\tinclusion\x12g\n" +
	"\vconsistency\x18\x05 \x01(\v26.numinance.ports.transparencylog.v1.ConsistencyRequestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0H\x00R\vconsistencyB\x06\n" +
	"\x04kindJ\x04\b\x01\x10\x04R\n" +
	"leaf_indexR\x04leafR\rmin_tree_size\"\x90\x01\n" +
	"\x10InclusionRequest\x12*\n" +
	"\n" +
	"leaf_index\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tleafIndex\x12\x1f\n" +
	"\x04leaf\x18\x02 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04leaf\x12/\n" +
	"\rmin_tree_size\x18\x03 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vminTreeSize\"v\n" +
	"\x12ConsistencyRequest\x12/\n" +
	"\rold_tree_size\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\voldTreeSize\x12/\n" +
	"\rnew_tree_size\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vnewTreeSize\"\x8e\x02\n" +
	"\rProveResponse\x12`\n" +
	"\tinclusion\x18\x05 \x01(\v23.numinance.ports.transparencylog.v1.InclusionResultB\v\x92\xb5\x18\a\n" +
	"\x051.0.0H\x00R\tinclusion\x12e\n" +
	"\vconsistency\x18\x06 \x01(\v24.numinance.ports.transparencylog.v1.ConsistencyProofB\v\x92\xb5\x18\a\n" +
	"\x051.0.0H\x00R\vconsistencyB\x06\n" +
	"\x04kindJ\x04\b\x01\x10\x05R\x05proofR\x05entryR\fentry_formatR\n" +
	"checkpoint\"\xdb\x01\n" +
	"\x0fInclusionResult\x12H\n" +
	"\x05proof\x18\x01 \x01(\v2%.numinance.evidence.v1.InclusionProofB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05proof\x12!\n" +
	"\x05entry\x18\x02 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05entry\x12.\n" +
	"\fentry_format\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ventryFormat\x12+\n" +
	"\n" +
	"checkpoint\x18\x04 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"checkpoint\"\xb1\x01\n" +
	"\x10ConsistencyProof\x12/\n" +
	"\rold_tree_size\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\voldTreeSize\x12/\n" +
	"\rnew_tree_size\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vnewTreeSize\x12;\n" +
	"\x04path\x18\x03 \x03(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04path\"0\n" +
	"\rLookupRequest\x12\x1f\n" +
	"\x04leaf\x18\x01 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04leaf\"<\n" +
	"\x0eLookupResponse\x12*\n" +
	"\n" +
	"leaf_index\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tleafIndex2\xe6\a\n" +
	"\x16TransparencyLogService\x12\xf2\x01\n" +
	"\x06Append\x121.numinance.ports.transparencylog.v1.AppendRequest\x1a2.numinance.ports.transparencylog.v1.AppendResponse\"\x80\x01\x8a\xb5\x18|\b\x04\x10\x01\x18\x01B\x051.0.0JUAppends one 33-byte leaf to the transparency log and returns its index and log entry.Z\x16transparencylog.append\x12\xe2\x01\n" +
	"\n" +
	"Checkpoint\x125.numinance.ports.transparencylog.v1.CheckpointRequest\x1a6.numinance.ports.transparencylog.v1.CheckpointResponse\"e\x8a\xb5\x18a\b\x01\x10\x01\x18\x01B\x051.0.0JCReturns the transparency log's signed checkpoint note as it stands.Z\revidence.read\x12\x91\x02\n" +
	"\x05Prove\x120.numinance.ports.transparencylog.v1.ProveRequest\x1a1.numinance.ports.transparencylog.v1.ProveResponse\"\xa2\x01\x8a\xb5\x18\x9d\x01\b\x01\x10\x01\x18\x01B\x051.0.0J\x7fReturns the inclusion proof, log entry and checkpoint for one leaf, or the consistency proof between two checkpoint tree sizes.Z\revidence.read\x12\xdd\x01\n" +
	"\x06Lookup\x121.numinance.ports.transparencylog.v1.LookupRequest\x1a2.numinance.ports.transparencylog.v1.LookupResponse\"l\x8a\xb5\x18h\b\x01\x10\x01\x18\x01:\x16transparencylog.lookupB\x051.0.0J2Finds the index of a leaf in the transparency log.Z\revidence.readB\xce\x02\n" +
	"&com.numinance.ports.transparencylog.v1B\x14TransparencylogProtoP\x01Zcgithub.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/transparencylog/v1;transparencylogv1\xa2\x02\x03NPT\xaa\x02\"Numinance.Ports.Transparencylog.V1\xca\x02\"Numinance\\Ports\\Transparencylog\\V1\xe2\x02.Numinance\\Ports\\Transparencylog\\V1\\GPBMetadata\xea\x02%Numinance::Ports::Transparencylog::V1b\x06proto3"

var (
	file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescOnce sync.Once
	file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescData []byte
)

func file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescGZIP() []byte {
	file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescOnce.Do(func() {
		file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDesc), len(file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDesc)))
	})
	return file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDescData
}

var file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_numinance_ports_transparencylog_v1_transparencylog_proto_goTypes = []any{
	(*AppendRequest)(nil),      // 0: numinance.ports.transparencylog.v1.AppendRequest
	(*AppendResponse)(nil),     // 1: numinance.ports.transparencylog.v1.AppendResponse
	(*CheckpointRequest)(nil),  // 2: numinance.ports.transparencylog.v1.CheckpointRequest
	(*CheckpointResponse)(nil), // 3: numinance.ports.transparencylog.v1.CheckpointResponse
	(*ProveRequest)(nil),       // 4: numinance.ports.transparencylog.v1.ProveRequest
	(*InclusionRequest)(nil),   // 5: numinance.ports.transparencylog.v1.InclusionRequest
	(*ConsistencyRequest)(nil), // 6: numinance.ports.transparencylog.v1.ConsistencyRequest
	(*ProveResponse)(nil),      // 7: numinance.ports.transparencylog.v1.ProveResponse
	(*InclusionResult)(nil),    // 8: numinance.ports.transparencylog.v1.InclusionResult
	(*ConsistencyProof)(nil),   // 9: numinance.ports.transparencylog.v1.ConsistencyProof
	(*LookupRequest)(nil),      // 10: numinance.ports.transparencylog.v1.LookupRequest
	(*LookupResponse)(nil),     // 11: numinance.ports.transparencylog.v1.LookupResponse
	(*v1.InclusionProof)(nil),  // 12: numinance.evidence.v1.InclusionProof
	(*v11.Digest)(nil),         // 13: numinance.types.v1.Digest
}
var file_numinance_ports_transparencylog_v1_transparencylog_proto_depIdxs = []int32{
	5,  // 0: numinance.ports.transparencylog.v1.ProveRequest.inclusion:type_name -> numinance.ports.transparencylog.v1.InclusionRequest
	6,  // 1: numinance.ports.transparencylog.v1.ProveRequest.consistency:type_name -> numinance.ports.transparencylog.v1.ConsistencyRequest
	8,  // 2: numinance.ports.transparencylog.v1.ProveResponse.inclusion:type_name -> numinance.ports.transparencylog.v1.InclusionResult
	9,  // 3: numinance.ports.transparencylog.v1.ProveResponse.consistency:type_name -> numinance.ports.transparencylog.v1.ConsistencyProof
	12, // 4: numinance.ports.transparencylog.v1.InclusionResult.proof:type_name -> numinance.evidence.v1.InclusionProof
	13, // 5: numinance.ports.transparencylog.v1.ConsistencyProof.path:type_name -> numinance.types.v1.Digest
	0,  // 6: numinance.ports.transparencylog.v1.TransparencyLogService.Append:input_type -> numinance.ports.transparencylog.v1.AppendRequest
	2,  // 7: numinance.ports.transparencylog.v1.TransparencyLogService.Checkpoint:input_type -> numinance.ports.transparencylog.v1.CheckpointRequest
	4,  // 8: numinance.ports.transparencylog.v1.TransparencyLogService.Prove:input_type -> numinance.ports.transparencylog.v1.ProveRequest
	10, // 9: numinance.ports.transparencylog.v1.TransparencyLogService.Lookup:input_type -> numinance.ports.transparencylog.v1.LookupRequest
	1,  // 10: numinance.ports.transparencylog.v1.TransparencyLogService.Append:output_type -> numinance.ports.transparencylog.v1.AppendResponse
	3,  // 11: numinance.ports.transparencylog.v1.TransparencyLogService.Checkpoint:output_type -> numinance.ports.transparencylog.v1.CheckpointResponse
	7,  // 12: numinance.ports.transparencylog.v1.TransparencyLogService.Prove:output_type -> numinance.ports.transparencylog.v1.ProveResponse
	11, // 13: numinance.ports.transparencylog.v1.TransparencyLogService.Lookup:output_type -> numinance.ports.transparencylog.v1.LookupResponse
	10, // [10:14] is the sub-list for method output_type
	6,  // [6:10] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_numinance_ports_transparencylog_v1_transparencylog_proto_init() }
func file_numinance_ports_transparencylog_v1_transparencylog_proto_init() {
	if File_numinance_ports_transparencylog_v1_transparencylog_proto != nil {
		return
	}
	file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[4].OneofWrappers = []any{
		(*ProveRequest_Inclusion)(nil),
		(*ProveRequest_Consistency)(nil),
	}
	file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes[7].OneofWrappers = []any{
		(*ProveResponse_Inclusion)(nil),
		(*ProveResponse_Consistency)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDesc), len(file_numinance_ports_transparencylog_v1_transparencylog_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_transparencylog_v1_transparencylog_proto_goTypes,
		DependencyIndexes: file_numinance_ports_transparencylog_v1_transparencylog_proto_depIdxs,
		MessageInfos:      file_numinance_ports_transparencylog_v1_transparencylog_proto_msgTypes,
	}.Build()
	File_numinance_ports_transparencylog_v1_transparencylog_proto = out.File
	file_numinance_ports_transparencylog_v1_transparencylog_proto_goTypes = nil
	file_numinance_ports_transparencylog_v1_transparencylog_proto_depIdxs = nil
}
