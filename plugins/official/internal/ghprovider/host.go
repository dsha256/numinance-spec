// Package ghprovider is the code the two GitHub providers share: the PluginService skeleton, the client of the host's
// HostService, a size-limited REST client and the cassette transport that replays recorded, redacted HTTP exchanges.
// It is internal to plugins/official: a provider is written against the public SDK, and this package only keeps the two of
// them from repeating one another.
package ghprovider

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

// Host is what a call needs of the HostService of its launch: a credential handle for the intent the manifest declares, and
// the evidence record the call declares. The provider reaches no other service and never holds credential material.
type Host interface {
	// Credential returns the handle for intent. The handle is opaque and usable only through the egress proxy the host named with it; it is never logged.
	Credential(ctx context.Context, invocation, intent string) (string, error)
	// Emit submits one evidence record of a declared type and returns its id.
	Emit(ctx context.Context, invocation, key, recordType string, payload *structpb.Struct) (string, error)
}

// GRPCHost is the Host of a launched provider: a client of the HostService at the address the launch passed.
type GRPCHost struct {
	client pluginv1.HostServiceClient
	conn   *grpc.ClientConn
	routes *Routes
}

// DialHost connects to the HostService at addr ("unix:" plus an absolute path).
func DialHost(addr string) (*GRPCHost, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}
	return &GRPCHost{client: pluginv1.NewHostServiceClient(conn), conn: conn, routes: NewRoutes(time.Now)}, nil
}

// Routes is the table Credential fills with the egress proxy of each handle; the live transport reads it.
func (h *GRPCHost) Routes() *Routes { return h.routes }

// Close closes the connection.
func (h *GRPCHost) Close() error { return h.conn.Close() }

// Credential implements Host.
func (h *GRPCHost) Credential(ctx context.Context, invocation, intent string) (string, error) {
	resp, err := h.client.RequestCredential(ctx, &pluginv1.RequestCredentialRequest{InvocationId: invocation, Intent: intent})
	if err != nil {
		return "", err
	}
	// A handle without a usable proxy address is not returned: the provider could only send it nowhere, and it must never be
	// sent directly.
	if err := h.routes.Set(resp.GetHandle(), resp.GetEgressProxy(), resp.GetExpiresAt().AsTime()); err != nil {
		return "", err
	}
	return resp.GetHandle(), nil
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
