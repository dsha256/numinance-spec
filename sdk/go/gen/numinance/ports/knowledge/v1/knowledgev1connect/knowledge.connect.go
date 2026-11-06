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

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/knowledge/v1/knowledge.proto

package knowledgev1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/knowledge/v1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion1_13_0

const (
	// KnowledgeServiceName is the fully-qualified name of the KnowledgeService service.
	KnowledgeServiceName = "numinance.ports.knowledge.v1.KnowledgeService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// KnowledgeServiceSearchProcedure is the fully-qualified name of the KnowledgeService's Search RPC.
	KnowledgeServiceSearchProcedure = "/numinance.ports.knowledge.v1.KnowledgeService/Search"
	// KnowledgeServiceFetchProcedure is the fully-qualified name of the KnowledgeService's Fetch RPC.
	KnowledgeServiceFetchProcedure = "/numinance.ports.knowledge.v1.KnowledgeService/Fetch"
	// KnowledgeServiceResolveProcedure is the fully-qualified name of the KnowledgeService's Resolve
	// RPC.
	KnowledgeServiceResolveProcedure = "/numinance.ports.knowledge.v1.KnowledgeService/Resolve"
)

// KnowledgeServiceClient is a client for the numinance.ports.knowledge.v1.KnowledgeService service.
type KnowledgeServiceClient interface {
	// Search runs the source's own search for query within scope and returns ranked references, never document bodies.
	Search(context.Context, *connect.Request[v1.SearchRequest]) (*connect.Response[v1.SearchResponse], error)
	// Fetch returns the content of one document at a version, from its first byte, up to max_bytes.
	Fetch(context.Context, *connect.Request[v1.FetchRequest]) (*connect.Response[v1.FetchResponse], error)
	// Resolve returns the canonical ref of a document, so two refs to the same document compare equal and the cache and receipts key on one ref.
	Resolve(context.Context, *connect.Request[v1.ResolveRequest]) (*connect.Response[v1.ResolveResponse], error)
}

// NewKnowledgeServiceClient constructs a client for the
// numinance.ports.knowledge.v1.KnowledgeService service. By default, it uses the Connect protocol
// with the binary Protobuf Codec, asks for gzipped responses, and sends uncompressed requests. To
// use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or connect.WithGRPCWeb()
// options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewKnowledgeServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) KnowledgeServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	knowledgeServiceMethods := v1.File_numinance_ports_knowledge_v1_knowledge_proto.Services().ByName("KnowledgeService").Methods()
	return &knowledgeServiceClient{
		search: connect.NewClient[v1.SearchRequest, v1.SearchResponse](
			httpClient,
			baseURL+KnowledgeServiceSearchProcedure,
			connect.WithSchema(knowledgeServiceMethods.ByName("Search")),
			connect.WithClientOptions(opts...),
		),
		fetch: connect.NewClient[v1.FetchRequest, v1.FetchResponse](
			httpClient,
			baseURL+KnowledgeServiceFetchProcedure,
			connect.WithSchema(knowledgeServiceMethods.ByName("Fetch")),
			connect.WithClientOptions(opts...),
		),
		resolve: connect.NewClient[v1.ResolveRequest, v1.ResolveResponse](
			httpClient,
			baseURL+KnowledgeServiceResolveProcedure,
			connect.WithSchema(knowledgeServiceMethods.ByName("Resolve")),
			connect.WithClientOptions(opts...),
		),
	}
}

// knowledgeServiceClient implements KnowledgeServiceClient.
type knowledgeServiceClient struct {
	search  *connect.Client[v1.SearchRequest, v1.SearchResponse]
	fetch   *connect.Client[v1.FetchRequest, v1.FetchResponse]
	resolve *connect.Client[v1.ResolveRequest, v1.ResolveResponse]
}

// Search calls numinance.ports.knowledge.v1.KnowledgeService.Search.
func (c *knowledgeServiceClient) Search(ctx context.Context, req *connect.Request[v1.SearchRequest]) (*connect.Response[v1.SearchResponse], error) {
	return c.search.CallUnary(ctx, req)
}

// Fetch calls numinance.ports.knowledge.v1.KnowledgeService.Fetch.
func (c *knowledgeServiceClient) Fetch(ctx context.Context, req *connect.Request[v1.FetchRequest]) (*connect.Response[v1.FetchResponse], error) {
	return c.fetch.CallUnary(ctx, req)
}

// Resolve calls numinance.ports.knowledge.v1.KnowledgeService.Resolve.
func (c *knowledgeServiceClient) Resolve(ctx context.Context, req *connect.Request[v1.ResolveRequest]) (*connect.Response[v1.ResolveResponse], error) {
	return c.resolve.CallUnary(ctx, req)
}

// KnowledgeServiceHandler is an implementation of the numinance.ports.knowledge.v1.KnowledgeService
// service.
type KnowledgeServiceHandler interface {
	// Search runs the source's own search for query within scope and returns ranked references, never document bodies.
	Search(context.Context, *connect.Request[v1.SearchRequest]) (*connect.Response[v1.SearchResponse], error)
	// Fetch returns the content of one document at a version, from its first byte, up to max_bytes.
	Fetch(context.Context, *connect.Request[v1.FetchRequest]) (*connect.Response[v1.FetchResponse], error)
	// Resolve returns the canonical ref of a document, so two refs to the same document compare equal and the cache and receipts key on one ref.
	Resolve(context.Context, *connect.Request[v1.ResolveRequest]) (*connect.Response[v1.ResolveResponse], error)
}

// NewKnowledgeServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewKnowledgeServiceHandler(svc KnowledgeServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	knowledgeServiceMethods := v1.File_numinance_ports_knowledge_v1_knowledge_proto.Services().ByName("KnowledgeService").Methods()
	knowledgeServiceSearchHandler := connect.NewUnaryHandler(
		KnowledgeServiceSearchProcedure,
		svc.Search,
		connect.WithSchema(knowledgeServiceMethods.ByName("Search")),
		connect.WithHandlerOptions(opts...),
	)
	knowledgeServiceFetchHandler := connect.NewUnaryHandler(
		KnowledgeServiceFetchProcedure,
		svc.Fetch,
		connect.WithSchema(knowledgeServiceMethods.ByName("Fetch")),
		connect.WithHandlerOptions(opts...),
	)
	knowledgeServiceResolveHandler := connect.NewUnaryHandler(
		KnowledgeServiceResolveProcedure,
		svc.Resolve,
		connect.WithSchema(knowledgeServiceMethods.ByName("Resolve")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.knowledge.v1.KnowledgeService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case KnowledgeServiceSearchProcedure:
			knowledgeServiceSearchHandler.ServeHTTP(w, r)
		case KnowledgeServiceFetchProcedure:
			knowledgeServiceFetchHandler.ServeHTTP(w, r)
		case KnowledgeServiceResolveProcedure:
			knowledgeServiceResolveHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedKnowledgeServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedKnowledgeServiceHandler struct{}

func (UnimplementedKnowledgeServiceHandler) Search(context.Context, *connect.Request[v1.SearchRequest]) (*connect.Response[v1.SearchResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.knowledge.v1.KnowledgeService.Search is not implemented"))
}

func (UnimplementedKnowledgeServiceHandler) Fetch(context.Context, *connect.Request[v1.FetchRequest]) (*connect.Response[v1.FetchResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.knowledge.v1.KnowledgeService.Fetch is not implemented"))
}

func (UnimplementedKnowledgeServiceHandler) Resolve(context.Context, *connect.Request[v1.ResolveRequest]) (*connect.Response[v1.ResolveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.knowledge.v1.KnowledgeService.Resolve is not implemented"))
}
