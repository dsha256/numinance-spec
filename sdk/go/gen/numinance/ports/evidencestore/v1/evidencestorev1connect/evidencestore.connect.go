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

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/evidencestore/v1/evidencestore.proto

package evidencestorev1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/evidencestore/v1"
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
	// EvidenceStoreServiceName is the fully-qualified name of the EvidenceStoreService service.
	EvidenceStoreServiceName = "numinance.ports.evidencestore.v1.EvidenceStoreService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// EvidenceStoreServiceAppendLockedProcedure is the fully-qualified name of the
	// EvidenceStoreService's AppendLocked RPC.
	EvidenceStoreServiceAppendLockedProcedure = "/numinance.ports.evidencestore.v1.EvidenceStoreService/AppendLocked"
	// EvidenceStoreServiceGetRecordProcedure is the fully-qualified name of the EvidenceStoreService's
	// GetRecord RPC.
	EvidenceStoreServiceGetRecordProcedure = "/numinance.ports.evidencestore.v1.EvidenceStoreService/GetRecord"
	// EvidenceStoreServiceListRangeProcedure is the fully-qualified name of the EvidenceStoreService's
	// ListRange RPC.
	EvidenceStoreServiceListRangeProcedure = "/numinance.ports.evidencestore.v1.EvidenceStoreService/ListRange"
	// EvidenceStoreServiceHeadProcedure is the fully-qualified name of the EvidenceStoreService's Head
	// RPC.
	EvidenceStoreServiceHeadProcedure = "/numinance.ports.evidencestore.v1.EvidenceStoreService/Head"
)

// EvidenceStoreServiceClient is a client for the
// numinance.ports.evidencestore.v1.EvidenceStoreService service.
type EvidenceStoreServiceClient interface {
	// AppendLocked creates the record object at (change_id, sequence) while the evidence context holds the Change's fenced single-writer lease; it is irreversible by design, hence HIGH; a replay of identical bytes succeeds without writing. No dry run: the record is fully built and checked by the caller before the call, there is no plan to preview, and a replay is a no-op.
	AppendLocked(context.Context, *connect.Request[v1.AppendLockedRequest]) (*connect.Response[v1.AppendLockedResponse], error)
	// GetRecord returns the record object at (change_id, sequence), or NOT_FOUND.
	GetRecord(context.Context, *connect.Request[v1.GetRecordRequest]) (*connect.Response[v1.GetRecordResponse], error)
	// ListRange returns the stored records from from_sequence to to_sequence in sequence order. The page stops early at the last stored sequence, or before the record that would take the page past 3145728 bytes of content; a non-empty page holds at least one record, and an early stop is not the end of the chain. A page is empty only when from_sequence is past the last stored sequence; an empty page below the head is a provider fault.
	ListRange(context.Context, *connect.Request[v1.ListRangeRequest]) (*connect.Response[v1.ListRangeResponse], error)
	// Head returns the highest stored sequence of a Change, the record stored there (the seal record once the chain is sealed), both from one snapshot; the zero response for a Change with no records.
	Head(context.Context, *connect.Request[v1.HeadRequest]) (*connect.Response[v1.HeadResponse], error)
}

// NewEvidenceStoreServiceClient constructs a client for the
// numinance.ports.evidencestore.v1.EvidenceStoreService service. By default, it uses the Connect
// protocol with the binary Protobuf Codec, asks for gzipped responses, and sends uncompressed
// requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewEvidenceStoreServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) EvidenceStoreServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	evidenceStoreServiceMethods := v1.File_numinance_ports_evidencestore_v1_evidencestore_proto.Services().ByName("EvidenceStoreService").Methods()
	return &evidenceStoreServiceClient{
		appendLocked: connect.NewClient[v1.AppendLockedRequest, v1.AppendLockedResponse](
			httpClient,
			baseURL+EvidenceStoreServiceAppendLockedProcedure,
			connect.WithSchema(evidenceStoreServiceMethods.ByName("AppendLocked")),
			connect.WithClientOptions(opts...),
		),
		getRecord: connect.NewClient[v1.GetRecordRequest, v1.GetRecordResponse](
			httpClient,
			baseURL+EvidenceStoreServiceGetRecordProcedure,
			connect.WithSchema(evidenceStoreServiceMethods.ByName("GetRecord")),
			connect.WithClientOptions(opts...),
		),
		listRange: connect.NewClient[v1.ListRangeRequest, v1.ListRangeResponse](
			httpClient,
			baseURL+EvidenceStoreServiceListRangeProcedure,
			connect.WithSchema(evidenceStoreServiceMethods.ByName("ListRange")),
			connect.WithClientOptions(opts...),
		),
		head: connect.NewClient[v1.HeadRequest, v1.HeadResponse](
			httpClient,
			baseURL+EvidenceStoreServiceHeadProcedure,
			connect.WithSchema(evidenceStoreServiceMethods.ByName("Head")),
			connect.WithClientOptions(opts...),
		),
	}
}

// evidenceStoreServiceClient implements EvidenceStoreServiceClient.
type evidenceStoreServiceClient struct {
	appendLocked *connect.Client[v1.AppendLockedRequest, v1.AppendLockedResponse]
	getRecord    *connect.Client[v1.GetRecordRequest, v1.GetRecordResponse]
	listRange    *connect.Client[v1.ListRangeRequest, v1.ListRangeResponse]
	head         *connect.Client[v1.HeadRequest, v1.HeadResponse]
}

// AppendLocked calls numinance.ports.evidencestore.v1.EvidenceStoreService.AppendLocked.
func (c *evidenceStoreServiceClient) AppendLocked(ctx context.Context, req *connect.Request[v1.AppendLockedRequest]) (*connect.Response[v1.AppendLockedResponse], error) {
	return c.appendLocked.CallUnary(ctx, req)
}

// GetRecord calls numinance.ports.evidencestore.v1.EvidenceStoreService.GetRecord.
func (c *evidenceStoreServiceClient) GetRecord(ctx context.Context, req *connect.Request[v1.GetRecordRequest]) (*connect.Response[v1.GetRecordResponse], error) {
	return c.getRecord.CallUnary(ctx, req)
}

// ListRange calls numinance.ports.evidencestore.v1.EvidenceStoreService.ListRange.
func (c *evidenceStoreServiceClient) ListRange(ctx context.Context, req *connect.Request[v1.ListRangeRequest]) (*connect.Response[v1.ListRangeResponse], error) {
	return c.listRange.CallUnary(ctx, req)
}

// Head calls numinance.ports.evidencestore.v1.EvidenceStoreService.Head.
func (c *evidenceStoreServiceClient) Head(ctx context.Context, req *connect.Request[v1.HeadRequest]) (*connect.Response[v1.HeadResponse], error) {
	return c.head.CallUnary(ctx, req)
}

// EvidenceStoreServiceHandler is an implementation of the
// numinance.ports.evidencestore.v1.EvidenceStoreService service.
type EvidenceStoreServiceHandler interface {
	// AppendLocked creates the record object at (change_id, sequence) while the evidence context holds the Change's fenced single-writer lease; it is irreversible by design, hence HIGH; a replay of identical bytes succeeds without writing. No dry run: the record is fully built and checked by the caller before the call, there is no plan to preview, and a replay is a no-op.
	AppendLocked(context.Context, *connect.Request[v1.AppendLockedRequest]) (*connect.Response[v1.AppendLockedResponse], error)
	// GetRecord returns the record object at (change_id, sequence), or NOT_FOUND.
	GetRecord(context.Context, *connect.Request[v1.GetRecordRequest]) (*connect.Response[v1.GetRecordResponse], error)
	// ListRange returns the stored records from from_sequence to to_sequence in sequence order. The page stops early at the last stored sequence, or before the record that would take the page past 3145728 bytes of content; a non-empty page holds at least one record, and an early stop is not the end of the chain. A page is empty only when from_sequence is past the last stored sequence; an empty page below the head is a provider fault.
	ListRange(context.Context, *connect.Request[v1.ListRangeRequest]) (*connect.Response[v1.ListRangeResponse], error)
	// Head returns the highest stored sequence of a Change, the record stored there (the seal record once the chain is sealed), both from one snapshot; the zero response for a Change with no records.
	Head(context.Context, *connect.Request[v1.HeadRequest]) (*connect.Response[v1.HeadResponse], error)
}

// NewEvidenceStoreServiceHandler builds an HTTP handler from the service implementation. It returns
// the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewEvidenceStoreServiceHandler(svc EvidenceStoreServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	evidenceStoreServiceMethods := v1.File_numinance_ports_evidencestore_v1_evidencestore_proto.Services().ByName("EvidenceStoreService").Methods()
	evidenceStoreServiceAppendLockedHandler := connect.NewUnaryHandler(
		EvidenceStoreServiceAppendLockedProcedure,
		svc.AppendLocked,
		connect.WithSchema(evidenceStoreServiceMethods.ByName("AppendLocked")),
		connect.WithHandlerOptions(opts...),
	)
	evidenceStoreServiceGetRecordHandler := connect.NewUnaryHandler(
		EvidenceStoreServiceGetRecordProcedure,
		svc.GetRecord,
		connect.WithSchema(evidenceStoreServiceMethods.ByName("GetRecord")),
		connect.WithHandlerOptions(opts...),
	)
	evidenceStoreServiceListRangeHandler := connect.NewUnaryHandler(
		EvidenceStoreServiceListRangeProcedure,
		svc.ListRange,
		connect.WithSchema(evidenceStoreServiceMethods.ByName("ListRange")),
		connect.WithHandlerOptions(opts...),
	)
	evidenceStoreServiceHeadHandler := connect.NewUnaryHandler(
		EvidenceStoreServiceHeadProcedure,
		svc.Head,
		connect.WithSchema(evidenceStoreServiceMethods.ByName("Head")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.evidencestore.v1.EvidenceStoreService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case EvidenceStoreServiceAppendLockedProcedure:
			evidenceStoreServiceAppendLockedHandler.ServeHTTP(w, r)
		case EvidenceStoreServiceGetRecordProcedure:
			evidenceStoreServiceGetRecordHandler.ServeHTTP(w, r)
		case EvidenceStoreServiceListRangeProcedure:
			evidenceStoreServiceListRangeHandler.ServeHTTP(w, r)
		case EvidenceStoreServiceHeadProcedure:
			evidenceStoreServiceHeadHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedEvidenceStoreServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedEvidenceStoreServiceHandler struct{}

func (UnimplementedEvidenceStoreServiceHandler) AppendLocked(context.Context, *connect.Request[v1.AppendLockedRequest]) (*connect.Response[v1.AppendLockedResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.evidencestore.v1.EvidenceStoreService.AppendLocked is not implemented"))
}

func (UnimplementedEvidenceStoreServiceHandler) GetRecord(context.Context, *connect.Request[v1.GetRecordRequest]) (*connect.Response[v1.GetRecordResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.evidencestore.v1.EvidenceStoreService.GetRecord is not implemented"))
}

func (UnimplementedEvidenceStoreServiceHandler) ListRange(context.Context, *connect.Request[v1.ListRangeRequest]) (*connect.Response[v1.ListRangeResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.evidencestore.v1.EvidenceStoreService.ListRange is not implemented"))
}

func (UnimplementedEvidenceStoreServiceHandler) Head(context.Context, *connect.Request[v1.HeadRequest]) (*connect.Response[v1.HeadResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.evidencestore.v1.EvidenceStoreService.Head is not implemented"))
}
