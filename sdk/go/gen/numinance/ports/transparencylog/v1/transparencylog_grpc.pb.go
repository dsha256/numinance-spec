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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.6.2
// - protoc             (unknown)
// source: numinance/ports/transparencylog/v1/transparencylog.proto

package transparencylogv1

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
	TransparencyLogService_Append_FullMethodName     = "/numinance.ports.transparencylog.v1.TransparencyLogService/Append"
	TransparencyLogService_Checkpoint_FullMethodName = "/numinance.ports.transparencylog.v1.TransparencyLogService/Checkpoint"
	TransparencyLogService_Prove_FullMethodName      = "/numinance.ports.transparencylog.v1.TransparencyLogService/Prove"
	TransparencyLogService_Lookup_FullMethodName     = "/numinance.ports.transparencylog.v1.TransparencyLogService/Lookup"
)

// TransparencyLogServiceClient is the client API for TransparencyLogService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// TransparencyLogService is the transparencylog port a log provider serves.
type TransparencyLogServiceClient interface {
	// Append adds a leaf to the end of the log and returns its 0-based index (a hint until a verified Prove), the exact entry the log hashed and the entry's format; irreversible by design, hence HIGH. Idempotent by the caller's retry rule, not by the log: the caller records the leaf_index it received and never re-appends a recorded leaf; after an unknown outcome (the call may or may not have written) it calls Lookup first where transparencylog.lookup is offered, and appends again only when Lookup answers NOT_FOUND. Without transparencylog.dedup a further Append of the same leaf may add another entry carrying it, which is harmless to verification; with it a replay returns the first index and writes nothing. Where Lookup is not offered, the caller appends again and accepts a possible duplicate entry. idempotent: true here means replay-safe under the caller's retry rule above, as numinance.annotations.v1.ActionOptions.idempotent allows for a caller-owned rule; it must not be read as provider-side dedup, which only transparencylog.dedup declares. No dry run: the leaf is fully built by the caller, there is no plan to preview.
	Append(ctx context.Context, in *AppendRequest, opts ...grpc.CallOption) (*AppendResponse, error)
	// Checkpoint returns the log's own latest checkpoint as a C2SP signed note, unchanged and never re-signed; NOT_FOUND for an empty log.
	Checkpoint(ctx context.Context, in *CheckpointRequest, opts ...grpc.CallOption) (*CheckpointResponse, error)
	// Prove returns one proof, as its request asks. An inclusion request returns the inclusion proof of the leaf at leaf_index, the entry stored there and the checkpoint the proof was made against, whose tree size is at least min_tree_size and above leaf_index. A consistency request returns the consistency proof from the tree of the first old_tree_size leaves to the tree of the first new_tree_size; old_tree_size 0, old_tree_size above new_tree_size or a size above 2^53-1 is INVALID_ARGUMENT, a new_tree_size past the log's size is UNAVAILABLE; equal sizes have the empty path. A request with no kind set is INVALID_ARGUMENT (a oneof cannot carry two). The consistency fields keep the names of ADR 0037 decision 4 and the domain code (old_tree_size, new_tree_size). Consistency lives under Prove, not as a verb of its own.
	Prove(ctx context.Context, in *ProveRequest, opts ...grpc.CallOption) (*ProveResponse, error)
	// Lookup returns the index of the first entry carrying the leaf, or NOT_FOUND. Gated by transparencylog.lookup: a log without a search index does not serve it, and the caller keeps the index Append returned.
	Lookup(ctx context.Context, in *LookupRequest, opts ...grpc.CallOption) (*LookupResponse, error)
}

type transparencyLogServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTransparencyLogServiceClient(cc grpc.ClientConnInterface) TransparencyLogServiceClient {
	return &transparencyLogServiceClient{cc}
}

func (c *transparencyLogServiceClient) Append(ctx context.Context, in *AppendRequest, opts ...grpc.CallOption) (*AppendResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AppendResponse)
	err := c.cc.Invoke(ctx, TransparencyLogService_Append_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *transparencyLogServiceClient) Checkpoint(ctx context.Context, in *CheckpointRequest, opts ...grpc.CallOption) (*CheckpointResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CheckpointResponse)
	err := c.cc.Invoke(ctx, TransparencyLogService_Checkpoint_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *transparencyLogServiceClient) Prove(ctx context.Context, in *ProveRequest, opts ...grpc.CallOption) (*ProveResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ProveResponse)
	err := c.cc.Invoke(ctx, TransparencyLogService_Prove_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *transparencyLogServiceClient) Lookup(ctx context.Context, in *LookupRequest, opts ...grpc.CallOption) (*LookupResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LookupResponse)
	err := c.cc.Invoke(ctx, TransparencyLogService_Lookup_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TransparencyLogServiceServer is the server API for TransparencyLogService service.
// All implementations must embed UnimplementedTransparencyLogServiceServer
// for forward compatibility.
//
// TransparencyLogService is the transparencylog port a log provider serves.
type TransparencyLogServiceServer interface {
	// Append adds a leaf to the end of the log and returns its 0-based index (a hint until a verified Prove), the exact entry the log hashed and the entry's format; irreversible by design, hence HIGH. Idempotent by the caller's retry rule, not by the log: the caller records the leaf_index it received and never re-appends a recorded leaf; after an unknown outcome (the call may or may not have written) it calls Lookup first where transparencylog.lookup is offered, and appends again only when Lookup answers NOT_FOUND. Without transparencylog.dedup a further Append of the same leaf may add another entry carrying it, which is harmless to verification; with it a replay returns the first index and writes nothing. Where Lookup is not offered, the caller appends again and accepts a possible duplicate entry. idempotent: true here means replay-safe under the caller's retry rule above, as numinance.annotations.v1.ActionOptions.idempotent allows for a caller-owned rule; it must not be read as provider-side dedup, which only transparencylog.dedup declares. No dry run: the leaf is fully built by the caller, there is no plan to preview.
	Append(context.Context, *AppendRequest) (*AppendResponse, error)
	// Checkpoint returns the log's own latest checkpoint as a C2SP signed note, unchanged and never re-signed; NOT_FOUND for an empty log.
	Checkpoint(context.Context, *CheckpointRequest) (*CheckpointResponse, error)
	// Prove returns one proof, as its request asks. An inclusion request returns the inclusion proof of the leaf at leaf_index, the entry stored there and the checkpoint the proof was made against, whose tree size is at least min_tree_size and above leaf_index. A consistency request returns the consistency proof from the tree of the first old_tree_size leaves to the tree of the first new_tree_size; old_tree_size 0, old_tree_size above new_tree_size or a size above 2^53-1 is INVALID_ARGUMENT, a new_tree_size past the log's size is UNAVAILABLE; equal sizes have the empty path. A request with no kind set is INVALID_ARGUMENT (a oneof cannot carry two). The consistency fields keep the names of ADR 0037 decision 4 and the domain code (old_tree_size, new_tree_size). Consistency lives under Prove, not as a verb of its own.
	Prove(context.Context, *ProveRequest) (*ProveResponse, error)
	// Lookup returns the index of the first entry carrying the leaf, or NOT_FOUND. Gated by transparencylog.lookup: a log without a search index does not serve it, and the caller keeps the index Append returned.
	Lookup(context.Context, *LookupRequest) (*LookupResponse, error)
	mustEmbedUnimplementedTransparencyLogServiceServer()
}

// UnimplementedTransparencyLogServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedTransparencyLogServiceServer struct{}

func (UnimplementedTransparencyLogServiceServer) Append(context.Context, *AppendRequest) (*AppendResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Append not implemented")
}
func (UnimplementedTransparencyLogServiceServer) Checkpoint(context.Context, *CheckpointRequest) (*CheckpointResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Checkpoint not implemented")
}
func (UnimplementedTransparencyLogServiceServer) Prove(context.Context, *ProveRequest) (*ProveResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Prove not implemented")
}
func (UnimplementedTransparencyLogServiceServer) Lookup(context.Context, *LookupRequest) (*LookupResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Lookup not implemented")
}
func (UnimplementedTransparencyLogServiceServer) mustEmbedUnimplementedTransparencyLogServiceServer() {
}
func (UnimplementedTransparencyLogServiceServer) testEmbeddedByValue() {}

// UnsafeTransparencyLogServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TransparencyLogServiceServer will
// result in compilation errors.
type UnsafeTransparencyLogServiceServer interface {
	mustEmbedUnimplementedTransparencyLogServiceServer()
}

func RegisterTransparencyLogServiceServer(s grpc.ServiceRegistrar, srv TransparencyLogServiceServer) {
	// If the following call panics, it indicates UnimplementedTransparencyLogServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&TransparencyLogService_ServiceDesc, srv)
}

func _TransparencyLogService_Append_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AppendRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TransparencyLogServiceServer).Append(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TransparencyLogService_Append_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TransparencyLogServiceServer).Append(ctx, req.(*AppendRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TransparencyLogService_Checkpoint_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckpointRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TransparencyLogServiceServer).Checkpoint(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TransparencyLogService_Checkpoint_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TransparencyLogServiceServer).Checkpoint(ctx, req.(*CheckpointRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TransparencyLogService_Prove_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProveRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TransparencyLogServiceServer).Prove(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TransparencyLogService_Prove_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TransparencyLogServiceServer).Prove(ctx, req.(*ProveRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TransparencyLogService_Lookup_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LookupRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TransparencyLogServiceServer).Lookup(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TransparencyLogService_Lookup_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TransparencyLogServiceServer).Lookup(ctx, req.(*LookupRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// TransparencyLogService_ServiceDesc is the grpc.ServiceDesc for TransparencyLogService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TransparencyLogService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "numinance.ports.transparencylog.v1.TransparencyLogService",
	HandlerType: (*TransparencyLogServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Append",
			Handler:    _TransparencyLogService_Append_Handler,
		},
		{
			MethodName: "Checkpoint",
			Handler:    _TransparencyLogService_Checkpoint_Handler,
		},
		{
			MethodName: "Prove",
			Handler:    _TransparencyLogService_Prove_Handler,
		},
		{
			MethodName: "Lookup",
			Handler:    _TransparencyLogService_Lookup_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "numinance/ports/transparencylog/v1/transparencylog.proto",
}
