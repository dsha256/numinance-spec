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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.6.2
// - protoc             (unknown)
// source: numinance/ports/evidencestore/v1/evidencestore.proto

package evidencestorev1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	EvidenceStoreService_AppendLocked_FullMethodName = "/numinance.ports.evidencestore.v1.EvidenceStoreService/AppendLocked"
	EvidenceStoreService_GetRecord_FullMethodName    = "/numinance.ports.evidencestore.v1.EvidenceStoreService/GetRecord"
	EvidenceStoreService_ListRange_FullMethodName    = "/numinance.ports.evidencestore.v1.EvidenceStoreService/ListRange"
	EvidenceStoreService_Head_FullMethodName         = "/numinance.ports.evidencestore.v1.EvidenceStoreService/Head"
)

// EvidenceStoreServiceClient is the client API for EvidenceStoreService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// EvidenceStoreService is the evidencestore port a storage provider serves.
type EvidenceStoreServiceClient interface {
	// AppendLocked creates the record object at (change_id, sequence) while the evidence context holds the Change's fenced single-writer lease; it is irreversible by design, hence HIGH; a replay of identical bytes succeeds without writing. No dry run: the record is fully built and checked by the caller before the call, there is no plan to preview, and a replay is a no-op.
	AppendLocked(ctx context.Context, in *AppendLockedRequest, opts ...grpc.CallOption) (*AppendLockedResponse, error)
	// GetRecord returns the record object at (change_id, sequence), or NOT_FOUND.
	GetRecord(ctx context.Context, in *GetRecordRequest, opts ...grpc.CallOption) (*GetRecordResponse, error)
	// ListRange returns the stored records from from_sequence to to_sequence in sequence order. The page stops early at the last stored sequence, or before the record that would take the page past 3145728 bytes of content; a non-empty page holds at least one record, and an early stop is not the end of the chain. A page is empty only when from_sequence is past the last stored sequence; an empty page below the head is a provider fault.
	ListRange(ctx context.Context, in *ListRangeRequest, opts ...grpc.CallOption) (*ListRangeResponse, error)
	// Head returns the highest stored sequence of a Change, the record stored there (the seal record once the chain is sealed), both from one snapshot; the zero response for a Change with no records.
	Head(ctx context.Context, in *HeadRequest, opts ...grpc.CallOption) (*HeadResponse, error)
}

type evidenceStoreServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewEvidenceStoreServiceClient(cc grpc.ClientConnInterface) EvidenceStoreServiceClient {
	return &evidenceStoreServiceClient{cc}
}

func (c *evidenceStoreServiceClient) AppendLocked(ctx context.Context, in *AppendLockedRequest, opts ...grpc.CallOption) (*AppendLockedResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AppendLockedResponse)
	err := c.cc.Invoke(ctx, EvidenceStoreService_AppendLocked_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *evidenceStoreServiceClient) GetRecord(ctx context.Context, in *GetRecordRequest, opts ...grpc.CallOption) (*GetRecordResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetRecordResponse)
	err := c.cc.Invoke(ctx, EvidenceStoreService_GetRecord_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *evidenceStoreServiceClient) ListRange(ctx context.Context, in *ListRangeRequest, opts ...grpc.CallOption) (*ListRangeResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListRangeResponse)
	err := c.cc.Invoke(ctx, EvidenceStoreService_ListRange_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *evidenceStoreServiceClient) Head(ctx context.Context, in *HeadRequest, opts ...grpc.CallOption) (*HeadResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(HeadResponse)
	err := c.cc.Invoke(ctx, EvidenceStoreService_Head_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EvidenceStoreServiceServer is the server API for EvidenceStoreService service.
// All implementations must embed UnimplementedEvidenceStoreServiceServer
// for forward compatibility.
//
// EvidenceStoreService is the evidencestore port a storage provider serves.
type EvidenceStoreServiceServer interface {
	// AppendLocked creates the record object at (change_id, sequence) while the evidence context holds the Change's fenced single-writer lease; it is irreversible by design, hence HIGH; a replay of identical bytes succeeds without writing. No dry run: the record is fully built and checked by the caller before the call, there is no plan to preview, and a replay is a no-op.
	AppendLocked(context.Context, *AppendLockedRequest) (*AppendLockedResponse, error)
	// GetRecord returns the record object at (change_id, sequence), or NOT_FOUND.
	GetRecord(context.Context, *GetRecordRequest) (*GetRecordResponse, error)
	// ListRange returns the stored records from from_sequence to to_sequence in sequence order. The page stops early at the last stored sequence, or before the record that would take the page past 3145728 bytes of content; a non-empty page holds at least one record, and an early stop is not the end of the chain. A page is empty only when from_sequence is past the last stored sequence; an empty page below the head is a provider fault.
	ListRange(context.Context, *ListRangeRequest) (*ListRangeResponse, error)
	// Head returns the highest stored sequence of a Change, the record stored there (the seal record once the chain is sealed), both from one snapshot; the zero response for a Change with no records.
	Head(context.Context, *HeadRequest) (*HeadResponse, error)
	mustEmbedUnimplementedEvidenceStoreServiceServer()
}

// UnimplementedEvidenceStoreServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedEvidenceStoreServiceServer struct{}

func (UnimplementedEvidenceStoreServiceServer) AppendLocked(context.Context, *AppendLockedRequest) (*AppendLockedResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method AppendLocked not implemented")
}
func (UnimplementedEvidenceStoreServiceServer) GetRecord(context.Context, *GetRecordRequest) (*GetRecordResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetRecord not implemented")
}
func (UnimplementedEvidenceStoreServiceServer) ListRange(context.Context, *ListRangeRequest) (*ListRangeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListRange not implemented")
}
func (UnimplementedEvidenceStoreServiceServer) Head(context.Context, *HeadRequest) (*HeadResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Head not implemented")
}
func (UnimplementedEvidenceStoreServiceServer) mustEmbedUnimplementedEvidenceStoreServiceServer() {}
func (UnimplementedEvidenceStoreServiceServer) testEmbeddedByValue()                              {}

// UnsafeEvidenceStoreServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EvidenceStoreServiceServer will
// result in compilation errors.
type UnsafeEvidenceStoreServiceServer interface {
	mustEmbedUnimplementedEvidenceStoreServiceServer()
}

func RegisterEvidenceStoreServiceServer(s grpc.ServiceRegistrar, srv EvidenceStoreServiceServer) {
	// If the following call panics, it indicates UnimplementedEvidenceStoreServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&EvidenceStoreService_ServiceDesc, srv)
}

func _EvidenceStoreService_AppendLocked_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AppendLockedRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EvidenceStoreServiceServer).AppendLocked(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EvidenceStoreService_AppendLocked_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EvidenceStoreServiceServer).AppendLocked(ctx, req.(*AppendLockedRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EvidenceStoreService_GetRecord_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRecordRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EvidenceStoreServiceServer).GetRecord(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EvidenceStoreService_GetRecord_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EvidenceStoreServiceServer).GetRecord(ctx, req.(*GetRecordRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EvidenceStoreService_ListRange_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRangeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EvidenceStoreServiceServer).ListRange(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EvidenceStoreService_ListRange_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EvidenceStoreServiceServer).ListRange(ctx, req.(*ListRangeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EvidenceStoreService_Head_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HeadRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EvidenceStoreServiceServer).Head(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EvidenceStoreService_Head_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EvidenceStoreServiceServer).Head(ctx, req.(*HeadRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// EvidenceStoreService_ServiceDesc is the grpc.ServiceDesc for EvidenceStoreService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EvidenceStoreService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "numinance.ports.evidencestore.v1.EvidenceStoreService",
	HandlerType: (*EvidenceStoreServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AppendLocked",
			Handler:    _EvidenceStoreService_AppendLocked_Handler,
		},
		{
			MethodName: "GetRecord",
			Handler:    _EvidenceStoreService_GetRecord_Handler,
		},
		{
			MethodName: "ListRange",
			Handler:    _EvidenceStoreService_ListRange_Handler,
		},
		{
			MethodName: "Head",
			Handler:    _EvidenceStoreService_Head_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "numinance/ports/evidencestore/v1/evidencestore.proto",
}
