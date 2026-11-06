package authorexecutor_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	"google.golang.org/protobuf/types/known/anypb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	authorexecutorv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
	"github.com/numinance/numinance-spec/sdk/go/ports/authorexecutor"
)

func planMethod() protoreflect.MethodDescriptor {
	return authorexecutorv1.File_numinance_ports_authorexecutor_v1_authorexecutor_proto.
		Services().ByName("AuthorExecutorService").Methods().ByName("Plan")
}

func planEv(r *authorexecutorv1.ExecuteResponse) *authorexecutorv1.PlanResponse {
	return &authorexecutorv1.PlanResponse{Event: r.GetEvent()}
}

func planStream(t *testing.T, status pluginv1.InvokeStatus, events ...*authorexecutorv1.PlanResponse) []*pluginv1.ActionResult {
	t.Helper()
	out := make([]*pluginv1.ActionResult, 0, len(events)+1)
	for _, e := range events {
		a, err := anypb.New(e)
		if err != nil {
			t.Fatalf("anypb.New = %v", err)
		}
		out = append(out, &pluginv1.ActionResult{Status: pluginv1.InvokeStatus_INVOKE_STATUS_RUNNING, Result: a})
	}
	return append(out, &pluginv1.ActionResult{Status: status})
}

// PLAN-001 and PLAN-002 (the stream half): a Plan stream is a stream of PlanResponse with the rules of Execute, and a FILE_CHANGED event
// of a read-only run breaks it.
func TestPlanRule(t *testing.T) {
	fileChanged := authorexecutorv1.ExecutionEventType_EXECUTION_EVENT_TYPE_FILE_CHANGED
	tests := []struct {
		name    string
		status  pluginv1.InvokeStatus
		events  []*authorexecutorv1.PlanResponse
		wantErr string
	}{
		{name: "started, completed, SUCCEEDED", status: succeeded, events: []*authorexecutorv1.PlanResponse{planEv(ev(started, 1)), planEv(ev(toolIntent, 2)), planEv(ev(completed, 3))}},
		{name: "started, failed, FAILED", status: failed, events: []*authorexecutorv1.PlanResponse{planEv(ev(started, 1)), planEv(ev(failedEv, 2))}},
		{name: "a file changed", status: succeeded, events: []*authorexecutorv1.PlanResponse{planEv(ev(started, 1)), planEv(ev(fileChanged, 2)), planEv(ev(completed, 3))}, wantErr: "read only"},
		{name: "sequence gap", status: failed, events: []*authorexecutorv1.PlanResponse{planEv(ev(started, 1)), planEv(ev(failedEv, 3))}, wantErr: "consecutive"},
		{name: "last event is not terminal", status: succeeded, events: []*authorexecutorv1.PlanResponse{planEv(ev(started, 1)), planEv(ev(toolIntent, 2))}, wantErr: "not a terminal event"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.ValidateInvokeStream(planMethod(), planStream(t, tt.status, tt.events...), authorexecutor.NewPlanRule())
			if tt.wantErr == "" && err != nil {
				t.Errorf("ValidateInvokeStream(%s) = %v, want nil", tt.name, err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("ValidateInvokeStream(%s) = %v, want an error containing %q", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestPlanRule_AnExecuteResponseIsNotAPlanResponse(t *testing.T) {
	err := plugin.ValidateInvokeStream(planMethod(), stream(t, succeeded, ev(started, 1), ev(completed, 2)), authorexecutor.NewPlanRule())
	if err == nil || !strings.Contains(err.Error(), "method output type") {
		t.Errorf("ValidateInvokeStream(Plan stream of ExecuteResponse) = %v, want a refusal: not the output type of Plan", err)
	}
}

func validPlanRequest() *authorexecutorv1.PlanRequest {
	hex := strings.Repeat("a", 32)
	d := &typesv1.Digest{Value: "sha256:" + strings.Repeat("b", 64)}
	return &authorexecutorv1.PlanRequest{Request: &authorexecutorv1.ChangeExecutionRequest{
		ChangeId: "chg_" + hex, RunId: "run_" + hex, GrantId: "grant_" + hex,
		PlanRef:             &typesv1.Reference{ArtifactRef: "task", Sha256: d},
		AllowedCapabilities: []typesv1.ExecutorCapability{typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_FILESYSTEM_READ},
	}}
}

// PLAN-003: the bounds of a planning request.
func TestValidatePlanRequest(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*authorexecutorv1.ChangeExecutionRequest)
		wantErr string
	}{
		{name: "a read-only planning request", mutate: func(*authorexecutorv1.ChangeExecutionRequest) {}},
		{name: "a work unit", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) { r.WorkUnitId = "wu_" + strings.Repeat("c", 32) }, wantErr: "work_unit_id"},
		{name: "allowed paths", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) { r.AllowedPaths = []string{"src"} }, wantErr: "allowed_paths"},
		{name: "a credential capability", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) { r.CredentialCapabilities = []string{"scm.read"} }, wantErr: "credential_capabilities"},
		{name: "filesystem write", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) {
			r.AllowedCapabilities = append(r.AllowedCapabilities, typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_FILESYSTEM_WRITE)
		}, wantErr: "writes"},
		{name: "shell", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) {
			r.AllowedCapabilities = append(r.AllowedCapabilities, typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_SHELL)
		}, wantErr: "writes"},
		{name: "network", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) {
			r.AllowedCapabilities = append(r.AllowedCapabilities, typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_NETWORK)
		}, wantErr: "writes"},
		{name: "an unknown capability", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) {
			r.AllowedCapabilities = append(r.AllowedCapabilities, typesv1.ExecutorCapability(99))
		}, wantErr: "unknown"},
		{name: "a repeated capability", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) {
			r.AllowedCapabilities = append(r.AllowedCapabilities, r.AllowedCapabilities[0])
		}, wantErr: "repeats"},
		{name: "change_id uppercase hex", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) { r.ChangeId = "chg_" + strings.Repeat("A", 32) }, wantErr: "change_id"},
		{name: "run_id with 31 hex", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) { r.RunId = "run_" + strings.Repeat("a", 31) }, wantErr: "run_id"},
		{name: "grant_id with another prefix", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) { r.GrantId = "gr_" + strings.Repeat("a", 32) }, wantErr: "grant_id"},
		{name: "plan_ref unset", mutate: func(r *authorexecutorv1.ChangeExecutionRequest) { r.PlanRef = nil }, wantErr: "plan_ref"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validPlanRequest()
			tt.mutate(req.Request)
			err := authorexecutor.ValidatePlanRequest(req)
			if tt.wantErr == "" && err != nil {
				t.Errorf("ValidatePlanRequest(%s) = %v, want nil", tt.name, err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("ValidatePlanRequest(%s) = %v, want an error containing %q", tt.name, err, tt.wantErr)
			}
		})
	}
	if err := authorexecutor.ValidatePlanRequest(&authorexecutorv1.PlanRequest{}); err == nil {
		t.Error("ValidatePlanRequest(request unset) = nil, want an error")
	}
}
