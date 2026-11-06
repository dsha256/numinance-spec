package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// Host is what a Run needs of the HostService of its launch: it resolves a reference the request names, and emits the one
// evidence record the call declares. The provider reaches no other service.
type Host interface {
	// Resolve returns the content of a reference of the invocation's request, at most limit bytes; more is an error and nothing is returned.
	Resolve(ctx context.Context, invocation string, ref *typesv1.Reference, limit int) ([]byte, error)
	// Emit submits one evidence record of the declared type and returns its id.
	Emit(ctx context.Context, invocation, key, recordType string, payload *structpb.Struct) (string, error)
}

// errOverLimit is returned by Resolve when the host streams more than the caller allows.
var errOverLimit = errors.New("the referenced content is over the limit")

// GRPCHost is the Host of a launched provider: a client of the HostService at the address the launch passed.
type GRPCHost struct {
	client pluginv1.HostServiceClient
	conn   *grpc.ClientConn
}

// DialHost connects to the HostService at addr ("unix:" plus an absolute path).
func DialHost(addr string) (*GRPCHost, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}
	return &GRPCHost{client: pluginv1.NewHostServiceClient(conn), conn: conn}, nil
}

// Close closes the connection.
func (h *GRPCHost) Close() error { return h.conn.Close() }

// Resolve implements Host.
func (h *GRPCHost) Resolve(ctx context.Context, invocation string, ref *typesv1.Reference, limit int) ([]byte, error) {
	st, err := h.client.ResolveReference(ctx, &pluginv1.ResolveReferenceRequest{InvocationId: invocation, Reference: ref})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	for {
		m, err := st.Recv()
		if errors.Is(err, io.EOF) {
			return buf.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
		if buf.Len()+len(m.GetChunk()) > limit {
			return nil, errOverLimit
		}
		buf.Write(m.GetChunk())
	}
}

// Emit implements Host.
func (h *GRPCHost) Emit(ctx context.Context, invocation, key, recordType string, payload *structpb.Struct) (string, error) {
	a, err := anypb.New(payload)
	if err != nil {
		return "", err
	}
	resp, err := h.client.EmitEvidence(ctx, &pluginv1.EmitEvidenceRequest{InvocationId: invocation, EmitKey: key, RecordType: recordType, Payload: a})
	if err != nil {
		return "", err
	}
	return resp.GetEvidenceId(), nil
}

// notFound reports whether err is a host answer that the reference does not exist or may not be resolved.
func notFound(err error) bool {
	c := status.Code(err)
	return c == codes.NotFound || c == codes.PermissionDenied
}
