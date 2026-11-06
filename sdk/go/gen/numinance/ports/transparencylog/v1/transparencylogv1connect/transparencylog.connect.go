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

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/transparencylog/v1/transparencylog.proto

package transparencylogv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/transparencylog/v1"
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
	// TransparencyLogServiceName is the fully-qualified name of the TransparencyLogService service.
	TransparencyLogServiceName = "numinance.ports.transparencylog.v1.TransparencyLogService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// TransparencyLogServiceAppendProcedure is the fully-qualified name of the TransparencyLogService's
	// Append RPC.
	TransparencyLogServiceAppendProcedure = "/numinance.ports.transparencylog.v1.TransparencyLogService/Append"
	// TransparencyLogServiceCheckpointProcedure is the fully-qualified name of the
	// TransparencyLogService's Checkpoint RPC.
	TransparencyLogServiceCheckpointProcedure = "/numinance.ports.transparencylog.v1.TransparencyLogService/Checkpoint"
	// TransparencyLogServiceProveProcedure is the fully-qualified name of the TransparencyLogService's
	// Prove RPC.
	TransparencyLogServiceProveProcedure = "/numinance.ports.transparencylog.v1.TransparencyLogService/Prove"
	// TransparencyLogServiceLookupProcedure is the fully-qualified name of the TransparencyLogService's
	// Lookup RPC.
	TransparencyLogServiceLookupProcedure = "/numinance.ports.transparencylog.v1.TransparencyLogService/Lookup"
)

// TransparencyLogServiceClient is a client for the
// numinance.ports.transparencylog.v1.TransparencyLogService service.
type TransparencyLogServiceClient interface {
	// Append adds a leaf to the end of the log and returns its 0-based index (a hint until a verified Prove), the exact entry the log hashed and the entry's format; irreversible by design, hence HIGH. Idempotent by the caller's retry rule, not by the log: the caller records the leaf_index it received and never re-appends a recorded leaf; after an unknown outcome (the call may or may not have written) it calls Lookup first where transparencylog.lookup is offered, and appends again only when Lookup answers NOT_FOUND. Without transparencylog.dedup a further Append of the same leaf may add another entry carrying it, which is harmless to verification; with it a replay returns the first index and writes nothing. Where Lookup is not offered, the caller appends again and accepts a possible duplicate entry. idempotent: true here means replay-safe under the caller's retry rule above, as numinance.annotations.v1.ActionOptions.idempotent allows for a caller-owned rule; it must not be read as provider-side dedup, which only transparencylog.dedup declares. No dry run: the leaf is fully built by the caller, there is no plan to preview.
	Append(context.Context, *connect.Request[v1.AppendRequest]) (*connect.Response[v1.AppendResponse], error)
	// Checkpoint returns the log's own latest checkpoint as a C2SP signed note, unchanged and never re-signed; NOT_FOUND for an empty log.
	Checkpoint(context.Context, *connect.Request[v1.CheckpointRequest]) (*connect.Response[v1.CheckpointResponse], error)
	// Prove returns one proof, as its request asks. An inclusion request returns the inclusion proof of the leaf at leaf_index, the entry stored there and the checkpoint the proof was made against, whose tree size is at least min_tree_size and above leaf_index. A consistency request returns the consistency proof from the tree of the first old_tree_size leaves to the tree of the first new_tree_size; old_tree_size 0, old_tree_size above new_tree_size or a size above 2^53-1 is INVALID_ARGUMENT, a new_tree_size past the log's size is UNAVAILABLE; equal sizes have the empty path. A request with no kind set is INVALID_ARGUMENT (a oneof cannot carry two). The consistency fields keep the names of ADR 0037 decision 4 and the domain code (old_tree_size, new_tree_size). Consistency lives under Prove, not as a verb of its own.
	Prove(context.Context, *connect.Request[v1.ProveRequest]) (*connect.Response[v1.ProveResponse], error)
	// Lookup returns the index of the first entry carrying the leaf, or NOT_FOUND. Gated by transparencylog.lookup: a log without a search index does not serve it, and the caller keeps the index Append returned.
	Lookup(context.Context, *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error)
}

// NewTransparencyLogServiceClient constructs a client for the
// numinance.ports.transparencylog.v1.TransparencyLogService service. By default, it uses the
// Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and sends
// uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewTransparencyLogServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) TransparencyLogServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	transparencyLogServiceMethods := v1.File_numinance_ports_transparencylog_v1_transparencylog_proto.Services().ByName("TransparencyLogService").Methods()
	return &transparencyLogServiceClient{
		append: connect.NewClient[v1.AppendRequest, v1.AppendResponse](
			httpClient,
			baseURL+TransparencyLogServiceAppendProcedure,
			connect.WithSchema(transparencyLogServiceMethods.ByName("Append")),
			connect.WithClientOptions(opts...),
		),
		checkpoint: connect.NewClient[v1.CheckpointRequest, v1.CheckpointResponse](
			httpClient,
			baseURL+TransparencyLogServiceCheckpointProcedure,
			connect.WithSchema(transparencyLogServiceMethods.ByName("Checkpoint")),
			connect.WithClientOptions(opts...),
		),
		prove: connect.NewClient[v1.ProveRequest, v1.ProveResponse](
			httpClient,
			baseURL+TransparencyLogServiceProveProcedure,
			connect.WithSchema(transparencyLogServiceMethods.ByName("Prove")),
			connect.WithClientOptions(opts...),
		),
		lookup: connect.NewClient[v1.LookupRequest, v1.LookupResponse](
			httpClient,
			baseURL+TransparencyLogServiceLookupProcedure,
			connect.WithSchema(transparencyLogServiceMethods.ByName("Lookup")),
			connect.WithClientOptions(opts...),
		),
	}
}

// transparencyLogServiceClient implements TransparencyLogServiceClient.
type transparencyLogServiceClient struct {
	append     *connect.Client[v1.AppendRequest, v1.AppendResponse]
	checkpoint *connect.Client[v1.CheckpointRequest, v1.CheckpointResponse]
	prove      *connect.Client[v1.ProveRequest, v1.ProveResponse]
	lookup     *connect.Client[v1.LookupRequest, v1.LookupResponse]
}

// Append calls numinance.ports.transparencylog.v1.TransparencyLogService.Append.
func (c *transparencyLogServiceClient) Append(ctx context.Context, req *connect.Request[v1.AppendRequest]) (*connect.Response[v1.AppendResponse], error) {
	return c.append.CallUnary(ctx, req)
}

// Checkpoint calls numinance.ports.transparencylog.v1.TransparencyLogService.Checkpoint.
func (c *transparencyLogServiceClient) Checkpoint(ctx context.Context, req *connect.Request[v1.CheckpointRequest]) (*connect.Response[v1.CheckpointResponse], error) {
	return c.checkpoint.CallUnary(ctx, req)
}

// Prove calls numinance.ports.transparencylog.v1.TransparencyLogService.Prove.
func (c *transparencyLogServiceClient) Prove(ctx context.Context, req *connect.Request[v1.ProveRequest]) (*connect.Response[v1.ProveResponse], error) {
	return c.prove.CallUnary(ctx, req)
}

// Lookup calls numinance.ports.transparencylog.v1.TransparencyLogService.Lookup.
func (c *transparencyLogServiceClient) Lookup(ctx context.Context, req *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error) {
	return c.lookup.CallUnary(ctx, req)
}

// TransparencyLogServiceHandler is an implementation of the
// numinance.ports.transparencylog.v1.TransparencyLogService service.
type TransparencyLogServiceHandler interface {
	// Append adds a leaf to the end of the log and returns its 0-based index (a hint until a verified Prove), the exact entry the log hashed and the entry's format; irreversible by design, hence HIGH. Idempotent by the caller's retry rule, not by the log: the caller records the leaf_index it received and never re-appends a recorded leaf; after an unknown outcome (the call may or may not have written) it calls Lookup first where transparencylog.lookup is offered, and appends again only when Lookup answers NOT_FOUND. Without transparencylog.dedup a further Append of the same leaf may add another entry carrying it, which is harmless to verification; with it a replay returns the first index and writes nothing. Where Lookup is not offered, the caller appends again and accepts a possible duplicate entry. idempotent: true here means replay-safe under the caller's retry rule above, as numinance.annotations.v1.ActionOptions.idempotent allows for a caller-owned rule; it must not be read as provider-side dedup, which only transparencylog.dedup declares. No dry run: the leaf is fully built by the caller, there is no plan to preview.
	Append(context.Context, *connect.Request[v1.AppendRequest]) (*connect.Response[v1.AppendResponse], error)
	// Checkpoint returns the log's own latest checkpoint as a C2SP signed note, unchanged and never re-signed; NOT_FOUND for an empty log.
	Checkpoint(context.Context, *connect.Request[v1.CheckpointRequest]) (*connect.Response[v1.CheckpointResponse], error)
	// Prove returns one proof, as its request asks. An inclusion request returns the inclusion proof of the leaf at leaf_index, the entry stored there and the checkpoint the proof was made against, whose tree size is at least min_tree_size and above leaf_index. A consistency request returns the consistency proof from the tree of the first old_tree_size leaves to the tree of the first new_tree_size; old_tree_size 0, old_tree_size above new_tree_size or a size above 2^53-1 is INVALID_ARGUMENT, a new_tree_size past the log's size is UNAVAILABLE; equal sizes have the empty path. A request with no kind set is INVALID_ARGUMENT (a oneof cannot carry two). The consistency fields keep the names of ADR 0037 decision 4 and the domain code (old_tree_size, new_tree_size). Consistency lives under Prove, not as a verb of its own.
	Prove(context.Context, *connect.Request[v1.ProveRequest]) (*connect.Response[v1.ProveResponse], error)
	// Lookup returns the index of the first entry carrying the leaf, or NOT_FOUND. Gated by transparencylog.lookup: a log without a search index does not serve it, and the caller keeps the index Append returned.
	Lookup(context.Context, *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error)
}

// NewTransparencyLogServiceHandler builds an HTTP handler from the service implementation. It
// returns the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewTransparencyLogServiceHandler(svc TransparencyLogServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	transparencyLogServiceMethods := v1.File_numinance_ports_transparencylog_v1_transparencylog_proto.Services().ByName("TransparencyLogService").Methods()
	transparencyLogServiceAppendHandler := connect.NewUnaryHandler(
		TransparencyLogServiceAppendProcedure,
		svc.Append,
		connect.WithSchema(transparencyLogServiceMethods.ByName("Append")),
		connect.WithHandlerOptions(opts...),
	)
	transparencyLogServiceCheckpointHandler := connect.NewUnaryHandler(
		TransparencyLogServiceCheckpointProcedure,
		svc.Checkpoint,
		connect.WithSchema(transparencyLogServiceMethods.ByName("Checkpoint")),
		connect.WithHandlerOptions(opts...),
	)
	transparencyLogServiceProveHandler := connect.NewUnaryHandler(
		TransparencyLogServiceProveProcedure,
		svc.Prove,
		connect.WithSchema(transparencyLogServiceMethods.ByName("Prove")),
		connect.WithHandlerOptions(opts...),
	)
	transparencyLogServiceLookupHandler := connect.NewUnaryHandler(
		TransparencyLogServiceLookupProcedure,
		svc.Lookup,
		connect.WithSchema(transparencyLogServiceMethods.ByName("Lookup")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.transparencylog.v1.TransparencyLogService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case TransparencyLogServiceAppendProcedure:
			transparencyLogServiceAppendHandler.ServeHTTP(w, r)
		case TransparencyLogServiceCheckpointProcedure:
			transparencyLogServiceCheckpointHandler.ServeHTTP(w, r)
		case TransparencyLogServiceProveProcedure:
			transparencyLogServiceProveHandler.ServeHTTP(w, r)
		case TransparencyLogServiceLookupProcedure:
			transparencyLogServiceLookupHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedTransparencyLogServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedTransparencyLogServiceHandler struct{}

func (UnimplementedTransparencyLogServiceHandler) Append(context.Context, *connect.Request[v1.AppendRequest]) (*connect.Response[v1.AppendResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.transparencylog.v1.TransparencyLogService.Append is not implemented"))
}

func (UnimplementedTransparencyLogServiceHandler) Checkpoint(context.Context, *connect.Request[v1.CheckpointRequest]) (*connect.Response[v1.CheckpointResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.transparencylog.v1.TransparencyLogService.Checkpoint is not implemented"))
}

func (UnimplementedTransparencyLogServiceHandler) Prove(context.Context, *connect.Request[v1.ProveRequest]) (*connect.Response[v1.ProveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.transparencylog.v1.TransparencyLogService.Prove is not implemented"))
}

func (UnimplementedTransparencyLogServiceHandler) Lookup(context.Context, *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.transparencylog.v1.TransparencyLogService.Lookup is not implemented"))
}
