package plugin_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// roundTrip marshals msg in binary and protojson, decodes each into a fresh
// message, checks proto.Equal, and returns the protojson form as a map so the
// caller can pin enum names on the wire.
func roundTrip(t *testing.T, msg proto.Message) map[string]any {
	t.Helper()
	wire, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal(%T) = %v", msg, err)
	}
	got := msg.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(wire, got); err != nil {
		t.Fatalf("proto.Unmarshal(%T) = %v", msg, err)
	}
	if !proto.Equal(got, msg) {
		t.Errorf("binary round trip of %T = %v, want %v", msg, got, msg)
	}

	js, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal(%T) = %v", msg, err)
	}
	got = msg.ProtoReflect().New().Interface()
	if err := protojson.Unmarshal(js, got); err != nil {
		t.Fatalf("protojson.Unmarshal(%T) = %v", msg, err)
	}
	if !proto.Equal(got, msg) {
		t.Errorf("protojson round trip of %T = %v, want %v", msg, got, msg)
	}
	var fields map[string]any
	if err := json.Unmarshal(js, &fields); err != nil {
		t.Fatalf("json.Unmarshal(protojson of %T) = %v", msg, err)
	}
	return fields
}

func TestActionResult_RoundTrip(t *testing.T) {
	result, err := anypb.New(&pluginv1.HealthResponse{Status: pluginv1.HealthStatus_HEALTH_STATUS_SERVING})
	if err != nil {
		t.Fatalf("anypb.New = %v", err)
	}
	msg := &pluginv1.InvokeResponse{Result: &pluginv1.ActionResult{
		Status:      pluginv1.InvokeStatus_INVOKE_STATUS_SUCCEEDED,
		Result:      result,
		EvidenceIds: []string{"ev_01"},
		SideEffects: []*pluginv1.ObservedSideEffect{
			{Kind: pluginv1.SideEffect_SIDE_EFFECT_WRITE, Target: "branch/feature-1"},
		},
		CompensationToken: []byte{0x00, 0x01, 0xfe, 0xff},
		Cost:              &pluginv1.ResourceBudget{CpuMillicores: 120, MemoryBytes: 1 << 20},
		Duration:          durationpb.New(1500 * time.Millisecond),
		Diagnostics: []*pluginv1.Diagnostic{{
			Reason:    pluginv1.Reason_REASON_DEPRECATED,
			Severity:  pluginv1.Severity_SEVERITY_WARNING,
			FieldPath: "request.items[3].name",
			Subject:   "numinance.ports.testport.v1.GetThingRequest.name",
		}},
	}}
	fields := roundTrip(t, msg)

	res, ok := fields["result"].(map[string]any)
	if !ok {
		t.Fatalf("protojson(InvokeResponse) has no result object: %v", fields)
	}
	diags, ok := res["diagnostics"].([]any)
	if !ok || len(diags) != 1 {
		t.Fatalf("protojson(ActionResult).diagnostics = %v, want one entry", res["diagnostics"])
	}
	diag, ok := diags[0].(map[string]any)
	if !ok {
		t.Fatalf("protojson(ActionResult).diagnostics[0] = %v, want an object", diags[0])
	}
	effects, ok := res["sideEffects"].([]any)
	if !ok || len(effects) != 1 {
		t.Fatalf("protojson(ActionResult).sideEffects = %v, want one entry", res["sideEffects"])
	}
	effect, ok := effects[0].(map[string]any)
	if !ok {
		t.Fatalf("protojson(ActionResult).sideEffects[0] = %v, want an object", effects[0])
	}
	pins := []struct {
		name string
		got  any
		want string
	}{
		{name: "status", got: res["status"], want: "INVOKE_STATUS_SUCCEEDED"},
		{name: "sideEffects[0].kind", got: effect["kind"], want: "SIDE_EFFECT_WRITE"},
		{name: "diagnostics[0].reason", got: diag["reason"], want: "REASON_DEPRECATED"},
		{name: "diagnostics[0].severity", got: diag["severity"], want: "SEVERITY_WARNING"},
		{name: "compensationToken", got: res["compensationToken"], want: "AAH+/w=="},
		{name: "duration", got: res["duration"], want: "1.500s"},
	}
	for _, p := range pins {
		if p.got != p.want {
			t.Errorf("protojson(ActionResult).%s = %v, want %q", p.name, p.got, p.want)
		}
	}
}

func TestHostService_RoundTrip(t *testing.T) {
	id := "inv_" + "0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a"
	tests := []struct {
		name string
		msg  proto.Message
		key  string
		want string
	}{
		{
			name: "log request pins severity",
			msg: &pluginv1.LogRequest{
				InvocationId: id,
				Severity:     pluginv1.Severity_SEVERITY_ERROR,
				Reason:       pluginv1.Reason_REASON_UPSTREAM_ERROR,
				Fields:       []*pluginv1.LogField{{Key: "status_code", Value: "503"}},
			},
			key:  "severity",
			want: "SEVERITY_ERROR",
		},
		{
			name: "progress request pins a phase reason",
			msg: &pluginv1.ReportProgressRequest{
				InvocationId: id,
				Completed:    3,
				Total:        7,
				Reason:       pluginv1.Reason_REASON_WAITING,
			},
			key:  "reason",
			want: "REASON_WAITING",
		},
		{
			name: "credential response keeps the handle and expiry",
			msg: &pluginv1.RequestCredentialResponse{
				Handle:    "handle-opaque-1",
				ExpiresAt: timestamppb.New(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)),
			},
			key:  "expiresAt",
			want: "2026-10-16T12:00:00Z",
		},
		{
			name: "evidence request keeps artifacts",
			msg: &pluginv1.EmitEvidenceRequest{
				InvocationId: id,
				EmitKey:      "verifier.report",
				RecordType:   "verifier.report",
				Artifacts: []*typesv1.Reference{{
					ArtifactRef:  "artifacts/report.json",
					Sha256:       &typesv1.Digest{Value: "sha256:" + strings.Repeat("ab", 32)},
					ArtifactType: "report",
				}},
			},
			key:  "recordType",
			want: "verifier.report",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := roundTrip(t, tt.msg)
			if got := fields[tt.key]; got != tt.want {
				t.Errorf("protojson(%T).%s = %v, want %q", tt.msg, tt.key, got, tt.want)
			}
		})
	}
}
