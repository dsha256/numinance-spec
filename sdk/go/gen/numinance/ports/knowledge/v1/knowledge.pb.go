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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/knowledge/v1/knowledge.proto

package knowledgev1

import (
	_ "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
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

// SearchRequest is one search.
type SearchRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the search text, 1 to 1024 printable bytes; the provider passes it to the source's search as a parameter, never spliced into a query language.
	Query string `protobuf:"bytes,1,opt,name=query,proto3" json:"query,omitempty"`
	// Required, 1 to 256 unique scope entries: the spaces, projects or relative paths to search, each within the connection's spec.scope.
	Scope []string `protobuf:"bytes,2,rep,name=scope,proto3" json:"scope,omitempty"`
	// Most results to return, 1 to 100.
	MaxResults    uint32 `protobuf:"varint,3,opt,name=max_results,json=maxResults,proto3" json:"max_results,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SearchRequest) Reset() {
	*x = SearchRequest{}
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SearchRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchRequest) ProtoMessage() {}

func (x *SearchRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchRequest.ProtoReflect.Descriptor instead.
func (*SearchRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP(), []int{0}
}

func (x *SearchRequest) GetQuery() string {
	if x != nil {
		return x.Query
	}
	return ""
}

func (x *SearchRequest) GetScope() []string {
	if x != nil {
		return x.Scope
	}
	return nil
}

func (x *SearchRequest) GetMaxResults() uint32 {
	if x != nil {
		return x.MaxResults
	}
	return 0
}

// SearchResponse is the ranked results of one search.
type SearchResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// At most max_results, unique by ref, best first; empty when nothing matched.
	Results       []*SearchResult `protobuf:"bytes,1,rep,name=results,proto3" json:"results,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SearchResponse) Reset() {
	*x = SearchResponse{}
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SearchResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchResponse) ProtoMessage() {}

func (x *SearchResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchResponse.ProtoReflect.Descriptor instead.
func (*SearchResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP(), []int{1}
}

func (x *SearchResponse) GetResults() []*SearchResult {
	if x != nil {
		return x.Results
	}
	return nil
}

// SearchResult is one matching document.
type SearchResult struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the document's ref.
	Ref string `protobuf:"bytes,1,opt,name=ref,proto3" json:"ref,omitempty"`
	// The document title, at most 1024 printable bytes; untrusted. The provider replaces any other character with U+FFFD and shortens a longer title at a character boundary.
	Title string `protobuf:"bytes,2,opt,name=title,proto3" json:"title,omitempty"`
	// A matching excerpt, at most 4096 bytes of printable characters, U+0009 and U+000A; untrusted. The provider replaces any other character with U+FFFD and shortens a longer excerpt at a character boundary.
	Snippet string `protobuf:"bytes,3,opt,name=snippet,proto3" json:"snippet,omitempty"`
	// The source's version or ETag of the document, at most 256 printable bytes; empty when the source has none.
	SourceVersion string `protobuf:"bytes,4,opt,name=source_version,json=sourceVersion,proto3" json:"source_version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SearchResult) Reset() {
	*x = SearchResult{}
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SearchResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchResult) ProtoMessage() {}

func (x *SearchResult) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchResult.ProtoReflect.Descriptor instead.
func (*SearchResult) Descriptor() ([]byte, []int) {
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP(), []int{2}
}

func (x *SearchResult) GetRef() string {
	if x != nil {
		return x.Ref
	}
	return ""
}

func (x *SearchResult) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *SearchResult) GetSnippet() string {
	if x != nil {
		return x.Snippet
	}
	return ""
}

func (x *SearchResult) GetSourceVersion() string {
	if x != nil {
		return x.SourceVersion
	}
	return ""
}

// FetchRequest names one document and the most bytes to return.
type FetchRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the document's ref.
	Ref string `protobuf:"bytes,1,opt,name=ref,proto3" json:"ref,omitempty"`
	// The version to fetch, at most 256 printable bytes; empty for the latest.
	Version string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	// Most content bytes to return, 1 to 1048576 (1 MiB); the host sets it from the stage budget and KnowledgeConnection budgets.bytes_per_call.
	MaxBytes uint32 `protobuf:"varint,3,opt,name=max_bytes,json=maxBytes,proto3" json:"max_bytes,omitempty"`
	// Required, 1 to 256 unique scope entries, each within the connection's spec.scope; the provider answers NOT_FOUND for a ref outside them.
	Scope         []string `protobuf:"bytes,4,rep,name=scope,proto3" json:"scope,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FetchRequest) Reset() {
	*x = FetchRequest{}
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FetchRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchRequest) ProtoMessage() {}

func (x *FetchRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchRequest.ProtoReflect.Descriptor instead.
func (*FetchRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP(), []int{3}
}

func (x *FetchRequest) GetRef() string {
	if x != nil {
		return x.Ref
	}
	return ""
}

func (x *FetchRequest) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *FetchRequest) GetMaxBytes() uint32 {
	if x != nil {
		return x.MaxBytes
	}
	return 0
}

func (x *FetchRequest) GetScope() []string {
	if x != nil {
		return x.Scope
	}
	return nil
}

// FetchResponse is the content of one document.
type FetchResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The document from its first byte, at most min(max_bytes, total_bytes) bytes, and shorter than total_bytes exactly when cut; for a text media type it is valid UTF-8, cut at a character boundary; untrusted. The host rejects content longer than max_bytes or total_bytes, uncut content whose length is not total_bytes, and invalid UTF-8 for a text media type.
	Content []byte `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	// Required: sha256 of content, the returned bytes, which is the document's digest only when content was not cut; the host recomputes it and refuses a mismatch.
	Digest *v1.Digest `protobuf:"bytes,2,opt,name=digest,proto3" json:"digest,omitempty"`
	// Size of the whole document at version in bytes; content is shorter exactly when the document was cut.
	TotalBytes uint64 `protobuf:"varint,3,opt,name=total_bytes,json=totalBytes,proto3" json:"total_bytes,omitempty"`
	// The version served, at most 256 printable bytes; equal to the request's version when one was named, empty when the source has none.
	Version string `protobuf:"bytes,4,opt,name=version,proto3" json:"version,omitempty"`
	// When the source last changed the document; unset when the source does not say.
	LastModified *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=last_modified,json=lastModified,proto3" json:"last_modified,omitempty"`
	// Required: the media type of content, matching ^[a-z0-9][a-z0-9.+-]{0,62}/[a-z0-9][a-z0-9.+-]{0,62}$; the knowledge context refuses a type outside its allow-list.
	MediaType     string `protobuf:"bytes,6,opt,name=media_type,json=mediaType,proto3" json:"media_type,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FetchResponse) Reset() {
	*x = FetchResponse{}
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FetchResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchResponse) ProtoMessage() {}

func (x *FetchResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchResponse.ProtoReflect.Descriptor instead.
func (*FetchResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP(), []int{4}
}

func (x *FetchResponse) GetContent() []byte {
	if x != nil {
		return x.Content
	}
	return nil
}

func (x *FetchResponse) GetDigest() *v1.Digest {
	if x != nil {
		return x.Digest
	}
	return nil
}

func (x *FetchResponse) GetTotalBytes() uint64 {
	if x != nil {
		return x.TotalBytes
	}
	return 0
}

func (x *FetchResponse) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *FetchResponse) GetLastModified() *timestamppb.Timestamp {
	if x != nil {
		return x.LastModified
	}
	return nil
}

func (x *FetchResponse) GetMediaType() string {
	if x != nil {
		return x.MediaType
	}
	return ""
}

// ResolveRequest names one document.
type ResolveRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: a ref of the document.
	Ref string `protobuf:"bytes,1,opt,name=ref,proto3" json:"ref,omitempty"`
	// Required, 1 to 256 unique scope entries, each within the connection's spec.scope; the provider answers NOT_FOUND for a ref outside them.
	Scope         []string `protobuf:"bytes,2,rep,name=scope,proto3" json:"scope,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResolveRequest) Reset() {
	*x = ResolveRequest{}
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResolveRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResolveRequest) ProtoMessage() {}

func (x *ResolveRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResolveRequest.ProtoReflect.Descriptor instead.
func (*ResolveRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP(), []int{5}
}

func (x *ResolveRequest) GetRef() string {
	if x != nil {
		return x.Ref
	}
	return ""
}

func (x *ResolveRequest) GetScope() []string {
	if x != nil {
		return x.Scope
	}
	return nil
}

// ResolveResponse carries the canonical ref.
type ResolveResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the canonical ref; resolving it again in the same scope returns it unchanged.
	CanonicalRef  string `protobuf:"bytes,1,opt,name=canonical_ref,json=canonicalRef,proto3" json:"canonical_ref,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResolveResponse) Reset() {
	*x = ResolveResponse{}
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResolveResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResolveResponse) ProtoMessage() {}

func (x *ResolveResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResolveResponse.ProtoReflect.Descriptor instead.
func (*ResolveResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP(), []int{6}
}

func (x *ResolveResponse) GetCanonicalRef() string {
	if x != nil {
		return x.CanonicalRef
	}
	return ""
}

var File_numinance_ports_knowledge_v1_knowledge_proto protoreflect.FileDescriptor

const file_numinance_ports_knowledge_v1_knowledge_proto_rawDesc = "" +
	"\n" +
	",numinance/ports/knowledge/v1/knowledge.proto\x12\x1cnuminance.ports.knowledge.v1\x1a\x1fgoogle/protobuf/timestamp.proto\x1a*numinance/annotations/v1/annotations.proto\x1a\x1enuminance/types/v1/types.proto\"\x83\x01\n" +
	"\rSearchRequest\x12!\n" +
	"\x05query\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05query\x12!\n" +
	"\x05scope\x18\x02 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05scope\x12,\n" +
	"\vmax_results\x18\x03 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"maxResults\"c\n" +
	"\x0eSearchResponse\x12Q\n" +
	"\aresults\x18\x01 \x03(\v2*.numinance.ports.knowledge.v1.SearchResultB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aresults\"\xab\x01\n" +
	"\fSearchResult\x12\x1d\n" +
	"\x03ref\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x03ref\x12!\n" +
	"\x05title\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05title\x12%\n" +
	"\asnippet\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\asnippet\x122\n" +
	"\x0esource_version\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rsourceVersion\"\xa1\x01\n" +
	"\fFetchRequest\x12\x1d\n" +
	"\x03ref\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x03ref\x12%\n" +
	"\aversion\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aversion\x12(\n" +
	"\tmax_bytes\x18\x03 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bmaxBytes\x12!\n" +
	"\x05scope\x18\x04 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05scope\"\xc6\x02\n" +
	"\rFetchResponse\x12%\n" +
	"\acontent\x18\x01 \x01(\fB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\acontent\x12?\n" +
	"\x06digest\x18\x02 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06digest\x12,\n" +
	"\vtotal_bytes\x18\x03 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"totalBytes\x12%\n" +
	"\aversion\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aversion\x12L\n" +
	"\rlast_modified\x18\x05 \x01(\v2\x1a.google.protobuf.TimestampB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\flastModified\x12*\n" +
	"\n" +
	"media_type\x18\x06 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tmediaType\"R\n" +
	"\x0eResolveRequest\x12\x1d\n" +
	"\x03ref\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x03ref\x12!\n" +
	"\x05scope\x18\x02 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05scope\"C\n" +
	"\x0fResolveResponse\x120\n" +
	"\rcanonical_ref\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fcanonicalRef2\xe7\x04\n" +
	"\x10KnowledgeService\x12\xc6\x01\n" +
	"\x06Search\x12+.numinance.ports.knowledge.v1.SearchRequest\x1a,.numinance.ports.knowledge.v1.SearchResponse\"a\x8a\xb5\x18]\b\x01\x10\x01\x18\x01B\x051.0.0JNSearches one approved knowledge source and returns ranked document references.\x12\xbb\x01\n" +
	"\x05Fetch\x12*.numinance.ports.knowledge.v1.FetchRequest\x1a+.numinance.ports.knowledge.v1.FetchResponse\"Y\x8a\xb5\x18U\b\x01\x10\x01\x18\x01B\x051.0.0JFReturns the content of one document from an approved knowledge source.\x12\xcb\x01\n" +
	"\aResolve\x12,.numinance.ports.knowledge.v1.ResolveRequest\x1a-.numinance.ports.knowledge.v1.ResolveResponse\"c\x8a\xb5\x18_\b\x01\x10\x01\x18\x01B\x051.0.0JPReturns the canonical reference of one document in an approved knowledge source.B\x9e\x02\n" +
	" com.numinance.ports.knowledge.v1B\x0eKnowledgeProtoP\x01ZWgithub.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/knowledge/v1;knowledgev1\xa2\x02\x03NPK\xaa\x02\x1cNuminance.Ports.Knowledge.V1\xca\x02\x1cNuminance\\Ports\\Knowledge\\V1\xe2\x02(Numinance\\Ports\\Knowledge\\V1\\GPBMetadata\xea\x02\x1fNuminance::Ports::Knowledge::V1b\x06proto3"

var (
	file_numinance_ports_knowledge_v1_knowledge_proto_rawDescOnce sync.Once
	file_numinance_ports_knowledge_v1_knowledge_proto_rawDescData []byte
)

func file_numinance_ports_knowledge_v1_knowledge_proto_rawDescGZIP() []byte {
	file_numinance_ports_knowledge_v1_knowledge_proto_rawDescOnce.Do(func() {
		file_numinance_ports_knowledge_v1_knowledge_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_knowledge_v1_knowledge_proto_rawDesc), len(file_numinance_ports_knowledge_v1_knowledge_proto_rawDesc)))
	})
	return file_numinance_ports_knowledge_v1_knowledge_proto_rawDescData
}

var file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_numinance_ports_knowledge_v1_knowledge_proto_goTypes = []any{
	(*SearchRequest)(nil),         // 0: numinance.ports.knowledge.v1.SearchRequest
	(*SearchResponse)(nil),        // 1: numinance.ports.knowledge.v1.SearchResponse
	(*SearchResult)(nil),          // 2: numinance.ports.knowledge.v1.SearchResult
	(*FetchRequest)(nil),          // 3: numinance.ports.knowledge.v1.FetchRequest
	(*FetchResponse)(nil),         // 4: numinance.ports.knowledge.v1.FetchResponse
	(*ResolveRequest)(nil),        // 5: numinance.ports.knowledge.v1.ResolveRequest
	(*ResolveResponse)(nil),       // 6: numinance.ports.knowledge.v1.ResolveResponse
	(*v1.Digest)(nil),             // 7: numinance.types.v1.Digest
	(*timestamppb.Timestamp)(nil), // 8: google.protobuf.Timestamp
}
var file_numinance_ports_knowledge_v1_knowledge_proto_depIdxs = []int32{
	2, // 0: numinance.ports.knowledge.v1.SearchResponse.results:type_name -> numinance.ports.knowledge.v1.SearchResult
	7, // 1: numinance.ports.knowledge.v1.FetchResponse.digest:type_name -> numinance.types.v1.Digest
	8, // 2: numinance.ports.knowledge.v1.FetchResponse.last_modified:type_name -> google.protobuf.Timestamp
	0, // 3: numinance.ports.knowledge.v1.KnowledgeService.Search:input_type -> numinance.ports.knowledge.v1.SearchRequest
	3, // 4: numinance.ports.knowledge.v1.KnowledgeService.Fetch:input_type -> numinance.ports.knowledge.v1.FetchRequest
	5, // 5: numinance.ports.knowledge.v1.KnowledgeService.Resolve:input_type -> numinance.ports.knowledge.v1.ResolveRequest
	1, // 6: numinance.ports.knowledge.v1.KnowledgeService.Search:output_type -> numinance.ports.knowledge.v1.SearchResponse
	4, // 7: numinance.ports.knowledge.v1.KnowledgeService.Fetch:output_type -> numinance.ports.knowledge.v1.FetchResponse
	6, // 8: numinance.ports.knowledge.v1.KnowledgeService.Resolve:output_type -> numinance.ports.knowledge.v1.ResolveResponse
	6, // [6:9] is the sub-list for method output_type
	3, // [3:6] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_numinance_ports_knowledge_v1_knowledge_proto_init() }
func file_numinance_ports_knowledge_v1_knowledge_proto_init() {
	if File_numinance_ports_knowledge_v1_knowledge_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_knowledge_v1_knowledge_proto_rawDesc), len(file_numinance_ports_knowledge_v1_knowledge_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_knowledge_v1_knowledge_proto_goTypes,
		DependencyIndexes: file_numinance_ports_knowledge_v1_knowledge_proto_depIdxs,
		MessageInfos:      file_numinance_ports_knowledge_v1_knowledge_proto_msgTypes,
	}.Build()
	File_numinance_ports_knowledge_v1_knowledge_proto = out.File
	file_numinance_ports_knowledge_v1_knowledge_proto_goTypes = nil
	file_numinance_ports_knowledge_v1_knowledge_proto_depIdxs = nil
}
