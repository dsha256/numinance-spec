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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.6.2
// - protoc             (unknown)
// source: numinance/ports/knowledge/v1/knowledge.proto

package knowledgev1

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
	KnowledgeService_Search_FullMethodName  = "/numinance.ports.knowledge.v1.KnowledgeService/Search"
	KnowledgeService_Fetch_FullMethodName   = "/numinance.ports.knowledge.v1.KnowledgeService/Fetch"
	KnowledgeService_Resolve_FullMethodName = "/numinance.ports.knowledge.v1.KnowledgeService/Resolve"
)

// KnowledgeServiceClient is the client API for KnowledgeService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// KnowledgeService is the knowledge port a knowledge provider serves.
type KnowledgeServiceClient interface {
	// Search runs the source's own search for query within scope and returns ranked references, never document bodies.
	Search(ctx context.Context, in *SearchRequest, opts ...grpc.CallOption) (*SearchResponse, error)
	// Fetch returns the content of one document at a version, from its first byte, up to max_bytes.
	Fetch(ctx context.Context, in *FetchRequest, opts ...grpc.CallOption) (*FetchResponse, error)
	// Resolve returns the canonical ref of a document, so two refs to the same document compare equal and the cache and receipts key on one ref.
	Resolve(ctx context.Context, in *ResolveRequest, opts ...grpc.CallOption) (*ResolveResponse, error)
}

type knowledgeServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewKnowledgeServiceClient(cc grpc.ClientConnInterface) KnowledgeServiceClient {
	return &knowledgeServiceClient{cc}
}

func (c *knowledgeServiceClient) Search(ctx context.Context, in *SearchRequest, opts ...grpc.CallOption) (*SearchResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SearchResponse)
	err := c.cc.Invoke(ctx, KnowledgeService_Search_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *knowledgeServiceClient) Fetch(ctx context.Context, in *FetchRequest, opts ...grpc.CallOption) (*FetchResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(FetchResponse)
	err := c.cc.Invoke(ctx, KnowledgeService_Fetch_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *knowledgeServiceClient) Resolve(ctx context.Context, in *ResolveRequest, opts ...grpc.CallOption) (*ResolveResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ResolveResponse)
	err := c.cc.Invoke(ctx, KnowledgeService_Resolve_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// KnowledgeServiceServer is the server API for KnowledgeService service.
// All implementations must embed UnimplementedKnowledgeServiceServer
// for forward compatibility.
//
// KnowledgeService is the knowledge port a knowledge provider serves.
type KnowledgeServiceServer interface {
	// Search runs the source's own search for query within scope and returns ranked references, never document bodies.
	Search(context.Context, *SearchRequest) (*SearchResponse, error)
	// Fetch returns the content of one document at a version, from its first byte, up to max_bytes.
	Fetch(context.Context, *FetchRequest) (*FetchResponse, error)
	// Resolve returns the canonical ref of a document, so two refs to the same document compare equal and the cache and receipts key on one ref.
	Resolve(context.Context, *ResolveRequest) (*ResolveResponse, error)
	mustEmbedUnimplementedKnowledgeServiceServer()
}

// UnimplementedKnowledgeServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedKnowledgeServiceServer struct{}

func (UnimplementedKnowledgeServiceServer) Search(context.Context, *SearchRequest) (*SearchResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Search not implemented")
}
func (UnimplementedKnowledgeServiceServer) Fetch(context.Context, *FetchRequest) (*FetchResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Fetch not implemented")
}
func (UnimplementedKnowledgeServiceServer) Resolve(context.Context, *ResolveRequest) (*ResolveResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Resolve not implemented")
}
func (UnimplementedKnowledgeServiceServer) mustEmbedUnimplementedKnowledgeServiceServer() {}
func (UnimplementedKnowledgeServiceServer) testEmbeddedByValue()                          {}

// UnsafeKnowledgeServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KnowledgeServiceServer will
// result in compilation errors.
type UnsafeKnowledgeServiceServer interface {
	mustEmbedUnimplementedKnowledgeServiceServer()
}

func RegisterKnowledgeServiceServer(s grpc.ServiceRegistrar, srv KnowledgeServiceServer) {
	// If the following call panics, it indicates UnimplementedKnowledgeServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&KnowledgeService_ServiceDesc, srv)
}

func _KnowledgeService_Search_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SearchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KnowledgeServiceServer).Search(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KnowledgeService_Search_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KnowledgeServiceServer).Search(ctx, req.(*SearchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KnowledgeService_Fetch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FetchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KnowledgeServiceServer).Fetch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KnowledgeService_Fetch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KnowledgeServiceServer).Fetch(ctx, req.(*FetchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KnowledgeService_Resolve_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ResolveRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KnowledgeServiceServer).Resolve(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KnowledgeService_Resolve_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KnowledgeServiceServer).Resolve(ctx, req.(*ResolveRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// KnowledgeService_ServiceDesc is the grpc.ServiceDesc for KnowledgeService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KnowledgeService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "numinance.ports.knowledge.v1.KnowledgeService",
	HandlerType: (*KnowledgeServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Search",
			Handler:    _KnowledgeService_Search_Handler,
		},
		{
			MethodName: "Fetch",
			Handler:    _KnowledgeService_Fetch_Handler,
		},
		{
			MethodName: "Resolve",
			Handler:    _KnowledgeService_Resolve_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "numinance/ports/knowledge/v1/knowledge.proto",
}
