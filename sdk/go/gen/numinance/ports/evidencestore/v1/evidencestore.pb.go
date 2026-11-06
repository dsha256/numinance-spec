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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/evidencestore/v1/evidencestore.proto

package evidencestorev1

import (
	_ "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
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

// StoredObject is one object as the store keeps it: its bytes and their digest.
type StoredObject struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Canonical bytes of the object, never parsed by the provider: 1 to 3145728 bytes.
	Content []byte `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	// sha256 of content; required on a create, which is refused when it is absent, malformed or another digest; a reader refuses a mismatch.
	ContentSha256 *v1.Digest `protobuf:"bytes,2,opt,name=content_sha256,json=contentSha256,proto3" json:"content_sha256,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StoredObject) Reset() {
	*x = StoredObject{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StoredObject) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StoredObject) ProtoMessage() {}

func (x *StoredObject) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StoredObject.ProtoReflect.Descriptor instead.
func (*StoredObject) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{0}
}

func (x *StoredObject) GetContent() []byte {
	if x != nil {
		return x.Content
	}
	return nil
}

func (x *StoredObject) GetContentSha256() *v1.Digest {
	if x != nil {
		return x.ContentSha256
	}
	return nil
}

// AppendLockedRequest names the key of one record and carries its canonical bytes.
type AppendLockedRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Change the record belongs to, ^chg_[0-9a-f]{32}$.
	ChangeId string `protobuf:"bytes,1,opt,name=change_id,json=changeId,proto3" json:"change_id,omitempty"`
	// 1-based position of the record in the chain, 1 to 2^53-1.
	Sequence uint64 `protobuf:"varint,2,opt,name=sequence,proto3" json:"sequence,omitempty"`
	// The record in its canonical form.
	Record *StoredObject `protobuf:"bytes,3,opt,name=record,proto3" json:"record,omitempty"`
	// content_sha256 of the stored record at sequence-1, required above sequence 1 and absent at sequence 1, so the predecessor is compared by content, not only by presence.
	PreviousContentSha256 *v1.Digest `protobuf:"bytes,4,opt,name=previous_content_sha256,json=previousContentSha256,proto3" json:"previous_content_sha256,omitempty"`
	unknownFields         protoimpl.UnknownFields
	sizeCache             protoimpl.SizeCache
}

func (x *AppendLockedRequest) Reset() {
	*x = AppendLockedRequest{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AppendLockedRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AppendLockedRequest) ProtoMessage() {}

func (x *AppendLockedRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AppendLockedRequest.ProtoReflect.Descriptor instead.
func (*AppendLockedRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{1}
}

func (x *AppendLockedRequest) GetChangeId() string {
	if x != nil {
		return x.ChangeId
	}
	return ""
}

func (x *AppendLockedRequest) GetSequence() uint64 {
	if x != nil {
		return x.Sequence
	}
	return 0
}

func (x *AppendLockedRequest) GetRecord() *StoredObject {
	if x != nil {
		return x.Record
	}
	return nil
}

func (x *AppendLockedRequest) GetPreviousContentSha256() *v1.Digest {
	if x != nil {
		return x.PreviousContentSha256
	}
	return nil
}

// AppendLockedResponse reports that the record is stored, by this call or by an identical earlier one.
type AppendLockedResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// True when the call found identical bytes already stored and wrote nothing.
	Replayed bool `protobuf:"varint,1,opt,name=replayed,proto3" json:"replayed,omitempty"`
	// sha256 of the stored object; the caller refuses a response where it differs from the request's.
	ContentSha256 *v1.Digest `protobuf:"bytes,2,opt,name=content_sha256,json=contentSha256,proto3" json:"content_sha256,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AppendLockedResponse) Reset() {
	*x = AppendLockedResponse{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AppendLockedResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AppendLockedResponse) ProtoMessage() {}

func (x *AppendLockedResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AppendLockedResponse.ProtoReflect.Descriptor instead.
func (*AppendLockedResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{2}
}

func (x *AppendLockedResponse) GetReplayed() bool {
	if x != nil {
		return x.Replayed
	}
	return false
}

func (x *AppendLockedResponse) GetContentSha256() *v1.Digest {
	if x != nil {
		return x.ContentSha256
	}
	return nil
}

// GetRecordRequest names one record.
type GetRecordRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Change the record belongs to, ^chg_[0-9a-f]{32}$.
	ChangeId string `protobuf:"bytes,1,opt,name=change_id,json=changeId,proto3" json:"change_id,omitempty"`
	// 1-based position of the record in the chain, 1 to 2^53-1.
	Sequence      uint64 `protobuf:"varint,2,opt,name=sequence,proto3" json:"sequence,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetRecordRequest) Reset() {
	*x = GetRecordRequest{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetRecordRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRecordRequest) ProtoMessage() {}

func (x *GetRecordRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRecordRequest.ProtoReflect.Descriptor instead.
func (*GetRecordRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{3}
}

func (x *GetRecordRequest) GetChangeId() string {
	if x != nil {
		return x.ChangeId
	}
	return ""
}

func (x *GetRecordRequest) GetSequence() uint64 {
	if x != nil {
		return x.Sequence
	}
	return 0
}

// GetRecordResponse carries one stored record.
type GetRecordResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The stored record as kept.
	Record        *StoredObject `protobuf:"bytes,1,opt,name=record,proto3" json:"record,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetRecordResponse) Reset() {
	*x = GetRecordResponse{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetRecordResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRecordResponse) ProtoMessage() {}

func (x *GetRecordResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRecordResponse.ProtoReflect.Descriptor instead.
func (*GetRecordResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{4}
}

func (x *GetRecordResponse) GetRecord() *StoredObject {
	if x != nil {
		return x.Record
	}
	return nil
}

// ListRangeRequest names a span of sequences of one Change.
type ListRangeRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Change whose records are listed, ^chg_[0-9a-f]{32}$.
	ChangeId string `protobuf:"bytes,1,opt,name=change_id,json=changeId,proto3" json:"change_id,omitempty"`
	// First sequence of the span, 1 to 2^53-1.
	FromSequence uint64 `protobuf:"varint,2,opt,name=from_sequence,json=fromSequence,proto3" json:"from_sequence,omitempty"`
	// Last sequence of the span, at least from_sequence, less than from_sequence plus 256 and at most 2^53-1.
	ToSequence    uint64 `protobuf:"varint,3,opt,name=to_sequence,json=toSequence,proto3" json:"to_sequence,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListRangeRequest) Reset() {
	*x = ListRangeRequest{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListRangeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRangeRequest) ProtoMessage() {}

func (x *ListRangeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRangeRequest.ProtoReflect.Descriptor instead.
func (*ListRangeRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{5}
}

func (x *ListRangeRequest) GetChangeId() string {
	if x != nil {
		return x.ChangeId
	}
	return ""
}

func (x *ListRangeRequest) GetFromSequence() uint64 {
	if x != nil {
		return x.FromSequence
	}
	return 0
}

func (x *ListRangeRequest) GetToSequence() uint64 {
	if x != nil {
		return x.ToSequence
	}
	return 0
}

// ListRangeResponse carries one page of stored records.
type ListRangeResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Stored records from from_sequence on, consecutive and in sequence order.
	Records       []*ListedRecord `protobuf:"bytes,1,rep,name=records,proto3" json:"records,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListRangeResponse) Reset() {
	*x = ListRangeResponse{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListRangeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRangeResponse) ProtoMessage() {}

func (x *ListRangeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRangeResponse.ProtoReflect.Descriptor instead.
func (*ListRangeResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{6}
}

func (x *ListRangeResponse) GetRecords() []*ListedRecord {
	if x != nil {
		return x.Records
	}
	return nil
}

// ListedRecord is one stored record with its sequence.
type ListedRecord struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// 1-based position of the record in the chain.
	Sequence uint64 `protobuf:"varint,1,opt,name=sequence,proto3" json:"sequence,omitempty"`
	// The stored record as kept.
	Record        *StoredObject `protobuf:"bytes,2,opt,name=record,proto3" json:"record,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListedRecord) Reset() {
	*x = ListedRecord{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListedRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListedRecord) ProtoMessage() {}

func (x *ListedRecord) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListedRecord.ProtoReflect.Descriptor instead.
func (*ListedRecord) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{7}
}

func (x *ListedRecord) GetSequence() uint64 {
	if x != nil {
		return x.Sequence
	}
	return 0
}

func (x *ListedRecord) GetRecord() *StoredObject {
	if x != nil {
		return x.Record
	}
	return nil
}

// HeadRequest names one Change.
type HeadRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Change whose head is read, ^chg_[0-9a-f]{32}$.
	ChangeId      string `protobuf:"bytes,1,opt,name=change_id,json=changeId,proto3" json:"change_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HeadRequest) Reset() {
	*x = HeadRequest{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HeadRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HeadRequest) ProtoMessage() {}

func (x *HeadRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HeadRequest.ProtoReflect.Descriptor instead.
func (*HeadRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{8}
}

func (x *HeadRequest) GetChangeId() string {
	if x != nil {
		return x.ChangeId
	}
	return ""
}

// HeadResponse is the stored state of one Change's chain.
type HeadResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Highest stored sequence, 0 for a Change with no records.
	Sequence uint64 `protobuf:"varint,1,opt,name=sequence,proto3" json:"sequence,omitempty"`
	// The record at sequence, unset when sequence is 0; the reader takes the head record_hash from it, never from a provider claim.
	HeadRecord    *StoredObject `protobuf:"bytes,2,opt,name=head_record,json=headRecord,proto3" json:"head_record,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HeadResponse) Reset() {
	*x = HeadResponse{}
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HeadResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HeadResponse) ProtoMessage() {}

func (x *HeadResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HeadResponse.ProtoReflect.Descriptor instead.
func (*HeadResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP(), []int{9}
}

func (x *HeadResponse) GetSequence() uint64 {
	if x != nil {
		return x.Sequence
	}
	return 0
}

func (x *HeadResponse) GetHeadRecord() *StoredObject {
	if x != nil {
		return x.HeadRecord
	}
	return nil
}

var File_numinance_ports_evidencestore_v1_evidencestore_proto protoreflect.FileDescriptor

const file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDesc = "" +
	"\n" +
	"4numinance/ports/evidencestore/v1/evidencestore.proto\x12 numinance.ports.evidencestore.v1\x1a*numinance/annotations/v1/annotations.proto\x1a\x1enuminance/types/v1/types.proto\"\x85\x01\n" +
	"\fStoredObject\x12%\n" +
	"\acontent\x18\x01 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\acontent\x12N\n" +
	"\x0econtent_sha256\x18\x02 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rcontentSha256\"\x9e\x02\n" +
	"\x13AppendLockedRequest\x12(\n" +
	"\tchange_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bchangeId\x12'\n" +
	"\bsequence\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bsequence\x12S\n" +
	"\x06record\x18\x03 \x01(\v2..numinance.ports.evidencestore.v1.StoredObjectB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06record\x12_\n" +
	"\x17previous_content_sha256\x18\x04 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x15previousContentSha256\"\x8f\x01\n" +
	"\x14AppendLockedResponse\x12'\n" +
	"\breplayed\x18\x01 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\breplayed\x12N\n" +
	"\x0econtent_sha256\x18\x02 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rcontentSha256\"e\n" +
	"\x10GetRecordRequest\x12(\n" +
	"\tchange_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bchangeId\x12'\n" +
	"\bsequence\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bsequence\"h\n" +
	"\x11GetRecordResponse\x12S\n" +
	"\x06record\x18\x01 \x01(\v2..numinance.ports.evidencestore.v1.StoredObjectB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06record\"\x9c\x01\n" +
	"\x10ListRangeRequest\x12(\n" +
	"\tchange_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bchangeId\x120\n" +
	"\rfrom_sequence\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ffromSequence\x12,\n" +
	"\vto_sequence\x18\x03 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"toSequence\"j\n" +
	"\x11ListRangeResponse\x12U\n" +
	"\arecords\x18\x01 \x03(\v2..numinance.ports.evidencestore.v1.ListedRecordB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\arecords\"\x8c\x01\n" +
	"\fListedRecord\x12'\n" +
	"\bsequence\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bsequence\x12S\n" +
	"\x06record\x18\x02 \x01(\v2..numinance.ports.evidencestore.v1.StoredObjectB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06record\"7\n" +
	"\vHeadRequest\x12(\n" +
	"\tchange_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bchangeId\"\xa1\x01\n" +
	"\fHeadResponse\x12'\n" +
	"\bsequence\x18\x01 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bsequence\x12\\\n" +
	"\vhead_record\x18\x02 \x01(\v2..numinance.ports.evidencestore.v1.StoredObjectB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"headRecordJ\x04\b\x03\x10\x04R\x04seal2\x87\a\n" +
	"\x14EvidenceStoreService\x12\xed\x01\n" +
	"\fAppendLocked\x125.numinance.ports.evidencestore.v1.AppendLockedRequest\x1a6.numinance.ports.evidencestore.v1.AppendLockedResponse\"n\x8a\xb5\x18j\b\x04\x10\x01\x18\x01B\x051.0.0JKStores one canonical evidence record write-once at its Change and sequence.Z\x0eevidence.write\x12\xd3\x01\n" +
	"\tGetRecord\x122.numinance.ports.evidencestore.v1.GetRecordRequest\x1a3.numinance.ports.evidencestore.v1.GetRecordResponse\"]\x8a\xb5\x18Y\b\x01\x10\x01\x18\x01B\x051.0.0J;Returns one stored evidence record of a Change by sequence.Z\revidence.read\x12\xe0\x01\n" +
	"\tListRange\x122.numinance.ports.evidencestore.v1.ListRangeRequest\x1a3.numinance.ports.evidencestore.v1.ListRangeResponse\"j\x8a\xb5\x18f\b\x01\x10\x01\x18\x01B\x051.0.0JHReturns a page of stored evidence records of a Change in sequence order.Z\revidence.read\x12\xc5\x01\n" +
	"\x04Head\x12-.numinance.ports.evidencestore.v1.HeadRequest\x1a..numinance.ports.evidencestore.v1.HeadResponse\"^\x8a\xb5\x18Z\b\x01\x10\x01\x18\x01B\x051.0.0J<Returns the last stored record of a Change's evidence chain.Z\revidence.readB\xbe\x02\n" +
	"$com.numinance.ports.evidencestore.v1B\x12EvidencestoreProtoP\x01Z_github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/evidencestore/v1;evidencestorev1\xa2\x02\x03NPE\xaa\x02 Numinance.Ports.Evidencestore.V1\xca\x02 Numinance\\Ports\\Evidencestore\\V1\xe2\x02,Numinance\\Ports\\Evidencestore\\V1\\GPBMetadata\xea\x02#Numinance::Ports::Evidencestore::V1b\x06proto3"

var (
	file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescOnce sync.Once
	file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescData []byte
)

func file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescGZIP() []byte {
	file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescOnce.Do(func() {
		file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDesc), len(file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDesc)))
	})
	return file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDescData
}

var file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_numinance_ports_evidencestore_v1_evidencestore_proto_goTypes = []any{
	(*StoredObject)(nil),         // 0: numinance.ports.evidencestore.v1.StoredObject
	(*AppendLockedRequest)(nil),  // 1: numinance.ports.evidencestore.v1.AppendLockedRequest
	(*AppendLockedResponse)(nil), // 2: numinance.ports.evidencestore.v1.AppendLockedResponse
	(*GetRecordRequest)(nil),     // 3: numinance.ports.evidencestore.v1.GetRecordRequest
	(*GetRecordResponse)(nil),    // 4: numinance.ports.evidencestore.v1.GetRecordResponse
	(*ListRangeRequest)(nil),     // 5: numinance.ports.evidencestore.v1.ListRangeRequest
	(*ListRangeResponse)(nil),    // 6: numinance.ports.evidencestore.v1.ListRangeResponse
	(*ListedRecord)(nil),         // 7: numinance.ports.evidencestore.v1.ListedRecord
	(*HeadRequest)(nil),          // 8: numinance.ports.evidencestore.v1.HeadRequest
	(*HeadResponse)(nil),         // 9: numinance.ports.evidencestore.v1.HeadResponse
	(*v1.Digest)(nil),            // 10: numinance.types.v1.Digest
}
var file_numinance_ports_evidencestore_v1_evidencestore_proto_depIdxs = []int32{
	10, // 0: numinance.ports.evidencestore.v1.StoredObject.content_sha256:type_name -> numinance.types.v1.Digest
	0,  // 1: numinance.ports.evidencestore.v1.AppendLockedRequest.record:type_name -> numinance.ports.evidencestore.v1.StoredObject
	10, // 2: numinance.ports.evidencestore.v1.AppendLockedRequest.previous_content_sha256:type_name -> numinance.types.v1.Digest
	10, // 3: numinance.ports.evidencestore.v1.AppendLockedResponse.content_sha256:type_name -> numinance.types.v1.Digest
	0,  // 4: numinance.ports.evidencestore.v1.GetRecordResponse.record:type_name -> numinance.ports.evidencestore.v1.StoredObject
	7,  // 5: numinance.ports.evidencestore.v1.ListRangeResponse.records:type_name -> numinance.ports.evidencestore.v1.ListedRecord
	0,  // 6: numinance.ports.evidencestore.v1.ListedRecord.record:type_name -> numinance.ports.evidencestore.v1.StoredObject
	0,  // 7: numinance.ports.evidencestore.v1.HeadResponse.head_record:type_name -> numinance.ports.evidencestore.v1.StoredObject
	1,  // 8: numinance.ports.evidencestore.v1.EvidenceStoreService.AppendLocked:input_type -> numinance.ports.evidencestore.v1.AppendLockedRequest
	3,  // 9: numinance.ports.evidencestore.v1.EvidenceStoreService.GetRecord:input_type -> numinance.ports.evidencestore.v1.GetRecordRequest
	5,  // 10: numinance.ports.evidencestore.v1.EvidenceStoreService.ListRange:input_type -> numinance.ports.evidencestore.v1.ListRangeRequest
	8,  // 11: numinance.ports.evidencestore.v1.EvidenceStoreService.Head:input_type -> numinance.ports.evidencestore.v1.HeadRequest
	2,  // 12: numinance.ports.evidencestore.v1.EvidenceStoreService.AppendLocked:output_type -> numinance.ports.evidencestore.v1.AppendLockedResponse
	4,  // 13: numinance.ports.evidencestore.v1.EvidenceStoreService.GetRecord:output_type -> numinance.ports.evidencestore.v1.GetRecordResponse
	6,  // 14: numinance.ports.evidencestore.v1.EvidenceStoreService.ListRange:output_type -> numinance.ports.evidencestore.v1.ListRangeResponse
	9,  // 15: numinance.ports.evidencestore.v1.EvidenceStoreService.Head:output_type -> numinance.ports.evidencestore.v1.HeadResponse
	12, // [12:16] is the sub-list for method output_type
	8,  // [8:12] is the sub-list for method input_type
	8,  // [8:8] is the sub-list for extension type_name
	8,  // [8:8] is the sub-list for extension extendee
	0,  // [0:8] is the sub-list for field type_name
}

func init() { file_numinance_ports_evidencestore_v1_evidencestore_proto_init() }
func file_numinance_ports_evidencestore_v1_evidencestore_proto_init() {
	if File_numinance_ports_evidencestore_v1_evidencestore_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDesc), len(file_numinance_ports_evidencestore_v1_evidencestore_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_evidencestore_v1_evidencestore_proto_goTypes,
		DependencyIndexes: file_numinance_ports_evidencestore_v1_evidencestore_proto_depIdxs,
		MessageInfos:      file_numinance_ports_evidencestore_v1_evidencestore_proto_msgTypes,
	}.Build()
	File_numinance_ports_evidencestore_v1_evidencestore_proto = out.File
	file_numinance_ports_evidencestore_v1_evidencestore_proto_goTypes = nil
	file_numinance_ports_evidencestore_v1_evidencestore_proto_depIdxs = nil
}
