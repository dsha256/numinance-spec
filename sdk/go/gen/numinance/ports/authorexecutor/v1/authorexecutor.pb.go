// The authorexecutor port (ADR 0008): one coding harness behind one adapter, driven for one run of one work unit inside the work-unit sandbox.
// Callers: only the execution context calls this port, for a run whose ActionContext.actor_kind is author-agent or reviewer-agent; the host refuses every other actor kind. A reviewer-agent run is read-only: its request has no allowed_paths, no credential_capabilities and no FILESYSTEM_WRITE, SHELL or NETWORK in allowed_capabilities, or the host refuses it; the host builds no candidate from a reviewer-agent run and discards its worktree. The port is never a model tool, and no tool_groups or scopes are declared.
// Authority: nothing in a request is advisory, and nothing a provider reports is authority. The sandbox, the egress proxy and the broker are configured from the host's resolved grant, never from the request, which is the harness's copy. Inference egress for the pinned model is configured by the host outside allowed_capabilities, credential_capabilities and network_policy_ref, and only to the endpoint of the pinned model_id validated for residency at Change start (ADR 0019): the proxy enforces it, nothing inside the sandbox redirects it, requests to non-inference methods there are denied, and it counts in network_destinations and in the dispatch risk. Its credential is a proxy-injected handle; a materialized key only when the manifest declares materialized and policy allows it, scoped to inference on that model, expiring by the deadline, never the application key, and recorded in host evidence. Under the PROCESS sandbox profile (sandboxprovider port) there is no egress proxy: the inference credential is a materialized key only where the manifest and policy allow it, nothing stops a redirect of inference egress, and the decision for Execute is PRIVILEGED; without that allowance the host refuses the run. The host recomputes every authority field of a ChangeExecutionRequest from the grant, the router decision and the plan, and refuses to dispatch a request where any differs. Capabilities and every ExecutionEvent are claims of the provider; the host records its own broker, proxy and sandbox decisions and never treats an event as an authorization. EXECUTION_COMPLETED is never a gate signal: gates read verification. The host reports a canary or an issued handle in an event, a FILE_CHANGED outside allowed_paths and a candidate the final-tree check below refuses, as an abnormal action, which starts the incident workflow (the design).
// Payloads: requests and events carry references and digests, never file contents, prompt text or credential material (a platform invariant: payloads carry references, never content). Of the request's references the provider may resolve through HostService.ResolveReference only task_ref, plan_ref, context_bundle_ref and test_requirements_ref; worktree_ref and network_policy_ref are host-only (field option host_only) and refused. Content an event points at is untrusted data.
// Paths: a path is relative to the worktree root, '/'-separated, NFC, printable, with segments of 1 to 255 bytes, no leading '/', no '\' and no '.' or '..' segment; it names a file or a directory with everything below it. Matching is segment-wise and literal (no globs); denied and protected paths match under Unicode simple case folding of the NFC form. The sandbox enforces on the resolved path after following symlinks and refuses hardlinks; the host checks denied and protected paths again on the final tree of the candidate, after NFC normalization and case folding, and rejects a name that is not NFC, a symlink whose target leaves the allowed paths, and a gitlink. The host builds the candidate against the run's base with every masked path (sandboxprovider port, Paths) taken from the base, so a masked path missing from the worktree on disk (a restored run, or PROCESS) is never a deletion; before that substitution it refuses, as an abnormal action, an entry on disk at or under a masked path that differs from the base (present where the base has none, or other content), so a write there is reported, not dropped; and it refuses a candidate that removes, renames or changes an entry at or under a protected or denied path. A segment that case-folds to .git, .gitmodules, .gitattributes or .lfsconfig, at any depth, is outside every grant, whatever allowed_paths says. The host reads the candidate tree with git hooks, fsmonitor, filter and smudge drivers, LFS and submodule recursion disabled, and never applies repository-local git configuration.
// Stream: Execute is server-streaming. A stream holds 1 to 1023 events: its first is EXECUTION_STARTED at sequence 1, sequences are consecutive, and its last and only its last is EXECUTION_COMPLETED or EXECUTION_FAILED. Each ExecuteResponse is at most 32 KiB serialized. On the plugin protocol each event rides one RUNNING Invoke message and the terminal Invoke message is SUCCEEDED after EXECUTION_COMPLETED and FAILED after EXECUTION_FAILED, so the call fits the Invoke bounds for an OPERATION target (1024 messages, 64 KiB per RUNNING message); the mapping is the server-streaming rule of PluginService.Invoke (plugin/v1). The host rejects a stream that breaks a rule and records the run as failed.
// Status codes: a request is checked before the harness starts, and a rejection sends no event. The host refuses with FAILED_PRECONDITION, before any Invoke, a request its recomputation or the pinned ExecutorManifest refuses, or a run_id already dispatched, records the refusal and returns it to the execution context. The provider rejects with INVALID_ARGUMENT a request that breaks a bound and with UNAVAILABLE when the harness cannot start. Once EXECUTION_STARTED is sent, every failure ends the stream with EXECUTION_FAILED. At the deadline or on a cancel the host ends the call and records the run as failed; the provider sends nothing after it. On the plugin protocol a provider rejection is the terminal FAILED message with one Diagnostic reason, INVALID_ARGUMENT with REASON_INVALID_ARGUMENT and UNAVAILABLE with REASON_UPSTREAM_UNAVAILABLE, and the host records the deadline as REASON_TIMEOUT and a cancel as REASON_CANCELLED. A stream that reaches 1023 events without a terminal one is budget exhaustion: the run fails and the execution context pauses the Change and escalates.
// Evidence: the stream is recorded as a provider claim, and evaluation outcomes come from host verification. Execute declares no evidence types: the execution context writes the stream and route records itself as host receipts, never a provider claim, and a declared type the provider does not emit would make the invocation PARTIAL.
// Bounds: every string is at most 1024 bytes and every repeated field at most 256 entries unless stated; over-limit input is rejected, never truncated; duplicate entries in any repeated field are rejected; enums are closed, and zero and unknown numbers are rejected. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Every element is since 1.0.0.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/authorexecutor/v1/authorexecutor.proto

package authorexecutorv1

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

// ExecutionEventType is the type of one event of the canonical execution stream. The stream is a provider claim, recorded as execution evidence and an input to the evaluation corpus.
type ExecutionEventType int32

const (
	// EXECUTION_EVENT_TYPE_UNSPECIFIED is the zero value; it is rejected by the host.
	ExecutionEventType_EXECUTION_EVENT_TYPE_UNSPECIFIED ExecutionEventType = 0
	// EXECUTION_EVENT_TYPE_EXECUTION_STARTED is the first event of every stream, at sequence 1.
	ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_STARTED ExecutionEventType = 1
	// EXECUTION_EVENT_TYPE_PLAN_ACCEPTED reports that the harness took the plan of plan_ref as its own.
	ExecutionEventType_EXECUTION_EVENT_TYPE_PLAN_ACCEPTED ExecutionEventType = 2
	// EXECUTION_EVENT_TYPE_TOOL_INTENT reports a tool call the harness is about to make.
	ExecutionEventType_EXECUTION_EVENT_TYPE_TOOL_INTENT ExecutionEventType = 3
	// EXECUTION_EVENT_TYPE_TOOL_ALLOWED echoes the broker's allow of a tool intent; it authorizes nothing.
	ExecutionEventType_EXECUTION_EVENT_TYPE_TOOL_ALLOWED ExecutionEventType = 4
	// EXECUTION_EVENT_TYPE_TOOL_DENIED echoes the broker's denial of a tool intent.
	ExecutionEventType_EXECUTION_EVENT_TYPE_TOOL_DENIED ExecutionEventType = 5
	// EXECUTION_EVENT_TYPE_TOOL_RESULT reports the result of a tool call.
	ExecutionEventType_EXECUTION_EVENT_TYPE_TOOL_RESULT ExecutionEventType = 6
	// EXECUTION_EVENT_TYPE_FILE_CHANGED reports a file of the worktree created, changed or removed; the candidate is the host's own diff of the worktree, never derived from these events.
	ExecutionEventType_EXECUTION_EVENT_TYPE_FILE_CHANGED ExecutionEventType = 7
	// EXECUTION_EVENT_TYPE_COMMAND_EXECUTED reports a command the harness ran.
	ExecutionEventType_EXECUTION_EVENT_TYPE_COMMAND_EXECUTED ExecutionEventType = 8
	// EXECUTION_EVENT_TYPE_NETWORK_REQUEST reports a network request the harness made through the egress proxy; the proxy's own record names the destination.
	ExecutionEventType_EXECUTION_EVENT_TYPE_NETWORK_REQUEST ExecutionEventType = 9
	// EXECUTION_EVENT_TYPE_CREDENTIAL_REQUEST reports a credential handle the harness asked the broker for.
	ExecutionEventType_EXECUTION_EVENT_TYPE_CREDENTIAL_REQUEST ExecutionEventType = 10
	// EXECUTION_EVENT_TYPE_TEST_RESULT reports the result of a test run.
	ExecutionEventType_EXECUTION_EVENT_TYPE_TEST_RESULT ExecutionEventType = 11
	// EXECUTION_EVENT_TYPE_CHECKPOINT reports a checkpoint the harness can resume from.
	ExecutionEventType_EXECUTION_EVENT_TYPE_CHECKPOINT ExecutionEventType = 12
	// EXECUTION_EVENT_TYPE_BUDGET_EVENT reports budget consumption or a budget reached.
	ExecutionEventType_EXECUTION_EVENT_TYPE_BUDGET_EVENT ExecutionEventType = 13
	// EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED is terminal: the harness finished its work.
	ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED ExecutionEventType = 14
	// EXECUTION_EVENT_TYPE_EXECUTION_FAILED is terminal: the harness stopped without finishing its work.
	ExecutionEventType_EXECUTION_EVENT_TYPE_EXECUTION_FAILED ExecutionEventType = 15
)

// Enum value maps for ExecutionEventType.
var (
	ExecutionEventType_name = map[int32]string{
		0:  "EXECUTION_EVENT_TYPE_UNSPECIFIED",
		1:  "EXECUTION_EVENT_TYPE_EXECUTION_STARTED",
		2:  "EXECUTION_EVENT_TYPE_PLAN_ACCEPTED",
		3:  "EXECUTION_EVENT_TYPE_TOOL_INTENT",
		4:  "EXECUTION_EVENT_TYPE_TOOL_ALLOWED",
		5:  "EXECUTION_EVENT_TYPE_TOOL_DENIED",
		6:  "EXECUTION_EVENT_TYPE_TOOL_RESULT",
		7:  "EXECUTION_EVENT_TYPE_FILE_CHANGED",
		8:  "EXECUTION_EVENT_TYPE_COMMAND_EXECUTED",
		9:  "EXECUTION_EVENT_TYPE_NETWORK_REQUEST",
		10: "EXECUTION_EVENT_TYPE_CREDENTIAL_REQUEST",
		11: "EXECUTION_EVENT_TYPE_TEST_RESULT",
		12: "EXECUTION_EVENT_TYPE_CHECKPOINT",
		13: "EXECUTION_EVENT_TYPE_BUDGET_EVENT",
		14: "EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED",
		15: "EXECUTION_EVENT_TYPE_EXECUTION_FAILED",
	}
	ExecutionEventType_value = map[string]int32{
		"EXECUTION_EVENT_TYPE_UNSPECIFIED":         0,
		"EXECUTION_EVENT_TYPE_EXECUTION_STARTED":   1,
		"EXECUTION_EVENT_TYPE_PLAN_ACCEPTED":       2,
		"EXECUTION_EVENT_TYPE_TOOL_INTENT":         3,
		"EXECUTION_EVENT_TYPE_TOOL_ALLOWED":        4,
		"EXECUTION_EVENT_TYPE_TOOL_DENIED":         5,
		"EXECUTION_EVENT_TYPE_TOOL_RESULT":         6,
		"EXECUTION_EVENT_TYPE_FILE_CHANGED":        7,
		"EXECUTION_EVENT_TYPE_COMMAND_EXECUTED":    8,
		"EXECUTION_EVENT_TYPE_NETWORK_REQUEST":     9,
		"EXECUTION_EVENT_TYPE_CREDENTIAL_REQUEST":  10,
		"EXECUTION_EVENT_TYPE_TEST_RESULT":         11,
		"EXECUTION_EVENT_TYPE_CHECKPOINT":          12,
		"EXECUTION_EVENT_TYPE_BUDGET_EVENT":        13,
		"EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED": 14,
		"EXECUTION_EVENT_TYPE_EXECUTION_FAILED":    15,
	}
)

func (x ExecutionEventType) Enum() *ExecutionEventType {
	p := new(ExecutionEventType)
	*p = x
	return p
}

func (x ExecutionEventType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ExecutionEventType) Descriptor() protoreflect.EnumDescriptor {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_enumTypes[0].Descriptor()
}

func (ExecutionEventType) Type() protoreflect.EnumType {
	return &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_enumTypes[0]
}

func (x ExecutionEventType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ExecutionEventType.Descriptor instead.
func (ExecutionEventType) EnumDescriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{0}
}

// CapabilitiesRequest has no fields; the adapter answers for itself.
type CapabilitiesRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CapabilitiesRequest) Reset() {
	*x = CapabilitiesRequest{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CapabilitiesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CapabilitiesRequest) ProtoMessage() {}

func (x *CapabilitiesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CapabilitiesRequest.ProtoReflect.Descriptor instead.
func (*CapabilitiesRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{0}
}

// CapabilitiesResponse carries the claimed executor capabilities.
type CapabilitiesResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// capabilities are the claimed executor capabilities, unique, at most 64.
	Capabilities  []v1.ExecutorCapability `protobuf:"varint,1,rep,packed,name=capabilities,proto3,enum=numinance.types.v1.ExecutorCapability" json:"capabilities,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CapabilitiesResponse) Reset() {
	*x = CapabilitiesResponse{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CapabilitiesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CapabilitiesResponse) ProtoMessage() {}

func (x *CapabilitiesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CapabilitiesResponse.ProtoReflect.Descriptor instead.
func (*CapabilitiesResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{1}
}

func (x *CapabilitiesResponse) GetCapabilities() []v1.ExecutorCapability {
	if x != nil {
		return x.Capabilities
	}
	return nil
}

// ExecuteRequest carries the request of one run.
type ExecuteRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// request is required.
	Request       *ChangeExecutionRequest `protobuf:"bytes,1,opt,name=request,proto3" json:"request,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExecuteRequest) Reset() {
	*x = ExecuteRequest{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecuteRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecuteRequest) ProtoMessage() {}

func (x *ExecuteRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecuteRequest.ProtoReflect.Descriptor instead.
func (*ExecuteRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{2}
}

func (x *ExecuteRequest) GetRequest() *ChangeExecutionRequest {
	if x != nil {
		return x.Request
	}
	return nil
}

// PlanRequest is the request of the planning run. Its ChangeExecutionRequest has no work unit and no plan yet: work_unit_id is empty, plan_ref names the task (there is no plan to name), allowed_paths is empty and the rest of the read-only rules of Plan hold.
type PlanRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// request is required.
	Request       *ChangeExecutionRequest `protobuf:"bytes,1,opt,name=request,proto3" json:"request,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PlanRequest) Reset() {
	*x = PlanRequest{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PlanRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlanRequest) ProtoMessage() {}

func (x *PlanRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PlanRequest.ProtoReflect.Descriptor instead.
func (*PlanRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{3}
}

func (x *PlanRequest) GetRequest() *ChangeExecutionRequest {
	if x != nil {
		return x.Request
	}
	return nil
}

// PlanResponse carries one event of the stream of a planning run, the same events as ExecuteResponse.
type PlanResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// event is required.
	Event         *ExecutionEvent `protobuf:"bytes,1,opt,name=event,proto3" json:"event,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PlanResponse) Reset() {
	*x = PlanResponse{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PlanResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlanResponse) ProtoMessage() {}

func (x *PlanResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PlanResponse.ProtoReflect.Descriptor instead.
func (*PlanResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{4}
}

func (x *PlanResponse) GetEvent() *ExecutionEvent {
	if x != nil {
		return x.Event
	}
	return nil
}

// ExecuteResponse carries one event of the stream.
type ExecuteResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// event is required.
	Event         *ExecutionEvent `protobuf:"bytes,1,opt,name=event,proto3" json:"event,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExecuteResponse) Reset() {
	*x = ExecuteResponse{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecuteResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecuteResponse) ProtoMessage() {}

func (x *ExecuteResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecuteResponse.ProtoReflect.Descriptor instead.
func (*ExecuteResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{5}
}

func (x *ExecuteResponse) GetEvent() *ExecutionEvent {
	if x != nil {
		return x.Event
	}
	return nil
}

// ChangeExecutionRequest is everything one run of one work unit is given. Every field is required unless its comment says it may be unset or empty. The host refuses to dispatch one whose change_id, run_id, grant_id or profile_digest differs from the ActionContext, whose deadline is later than the ActionContext deadline, whose plan_ref.sha256 differs from ActionContext.plan_digest, or whose authority fields (work_unit_id, allowed_paths, denied_paths, allowed_capabilities, network_policy_ref, credential_capabilities, budget, executor_id, model_id, policy_digest) differ from the host's recomputation; each protected_resources entry must be in the host's protected set.
type ChangeExecutionRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// change_id is ^chg_[0-9a-f]{32}$.
	ChangeId string `protobuf:"bytes,1,opt,name=change_id,json=changeId,proto3" json:"change_id,omitempty"`
	// work_unit_id is ^wu_[0-9a-f]{32}$, the work unit of the grant.
	WorkUnitId string `protobuf:"bytes,2,opt,name=work_unit_id,json=workUnitId,proto3" json:"work_unit_id,omitempty"`
	// run_id is ^run_[0-9a-f]{32}$; one Execute per run_id.
	RunId string `protobuf:"bytes,3,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	// grant_id is ^grant_[0-9a-f]{32}$: the grant the run acts under.
	GrantId string `protobuf:"bytes,4,opt,name=grant_id,json=grantId,proto3" json:"grant_id,omitempty"`
	// repository_digest is the digest of the repository content the worktree was created from: the git tree of the commit the worktree of this run is created from, which is the change's pinned base revision, or the head of the change's branch for a later work unit. It is the sha256 of the bytes "repository/tree", NUL, the object format of the repository ("sha1" or "sha256"), NUL and the lower-case hex id of that tree. The host computes it when the run starts and checks it again when the worktree is created.
	RepositoryDigest *v1.Digest `protobuf:"bytes,5,opt,name=repository_digest,json=repositoryDigest,proto3" json:"repository_digest,omitempty"`
	// worktree_ref is the worktree the sandbox mounts; host-only, never resolvable by the provider.
	WorktreeRef *v1.Reference `protobuf:"bytes,6,opt,name=worktree_ref,json=worktreeRef,proto3" json:"worktree_ref,omitempty"`
	// task_ref is the work unit's task.
	TaskRef *v1.Reference `protobuf:"bytes,7,opt,name=task_ref,json=taskRef,proto3" json:"task_ref,omitempty"`
	// plan_ref is the gated plan the work unit belongs to; its sha256 equals ActionContext.plan_digest.
	PlanRef *v1.Reference `protobuf:"bytes,8,opt,name=plan_ref,json=planRef,proto3" json:"plan_ref,omitempty"`
	// context_bundle_ref is the ContextBundle of the run; unset when the run has none.
	ContextBundleRef *v1.Reference `protobuf:"bytes,9,opt,name=context_bundle_ref,json=contextBundleRef,proto3" json:"context_bundle_ref,omitempty"`
	// executor_id is the metadata.name of the ExecutorManifest pinned in the lockfile for this adapter, as the router decided.
	ExecutorId string `protobuf:"bytes,10,opt,name=executor_id,json=executorId,proto3" json:"executor_id,omitempty"`
	// model_id is one models[].id of that ExecutorManifest, as the router decided.
	ModelId string `protobuf:"bytes,11,opt,name=model_id,json=modelId,proto3" json:"model_id,omitempty"`
	// allowed_paths are the only worktree paths the run may write; empty for a read-only run. None equals, contains or lies under a denied path or a literal path of the host's full protected set, or the request is refused; a floor pattern that may match below an allowed path is enforced by the host's check of the candidate tree. Under CONTAINER and GVISOR a path missing at the start must lie under another allowed path (sandboxprovider port, Paths); plan validation may check this before Create.
	AllowedPaths []string `protobuf:"bytes,12,rep,name=allowed_paths,json=allowedPaths,proto3" json:"allowed_paths,omitempty"`
	// denied_paths are worktree paths the run may neither read nor write; may be empty.
	DeniedPaths []string `protobuf:"bytes,13,rep,name=denied_paths,json=deniedPaths,proto3" json:"denied_paths,omitempty"`
	// allowed_capabilities are the executor capabilities the run may use, unique, each declared by the pinned ExecutorManifest, at most 64; may be empty.
	AllowedCapabilities []v1.ExecutorCapability `protobuf:"varint,14,rep,packed,name=allowed_capabilities,json=allowedCapabilities,proto3,enum=numinance.types.v1.ExecutorCapability" json:"allowed_capabilities,omitempty"`
	// network_policy_ref is the egress allow-list the proxy enforces for the run; host-only, never resolvable by the provider. Link-local ranges and metadata endpoints are denied whatever it lists.
	NetworkPolicyRef *v1.Reference `protobuf:"bytes,15,opt,name=network_policy_ref,json=networkPolicyRef,proto3" json:"network_policy_ref,omitempty"`
	// credential_capabilities are the broker intent names, ^[a-z][a-z0-9._-]{0,127}$, the run may ask a handle for, each in the grant and the manifest's credential_intents; at most 16; may be empty. The broker never returns credential material.
	CredentialCapabilities []string `protobuf:"bytes,16,rep,name=credential_capabilities,json=credentialCapabilities,proto3" json:"credential_capabilities,omitempty"`
	// budget is the effective budget of the run.
	Budget *ExecutionBudget `protobuf:"bytes,17,opt,name=budget,proto3" json:"budget,omitempty"`
	// deadline is when the run must end, in the future at dispatch; the host cancels the call then.
	Deadline *timestamppb.Timestamp `protobuf:"bytes,18,opt,name=deadline,proto3" json:"deadline,omitempty"`
	// test_requirements_ref is the tests the work unit must pass; unset when it names none.
	TestRequirementsRef *v1.Reference `protobuf:"bytes,19,opt,name=test_requirements_ref,json=testRequirementsRef,proto3" json:"test_requirements_ref,omitempty"`
	// protected_resources are the worktree paths outside every grant (policy, CI definitions, IAM, workflow files, the kernel), at least one; the harness's copy of the protected set, which the sandbox takes from the kernel and the floors and masks whatever the request says (sandboxprovider port, Paths); its floor patterns are enforced by the host's check of the candidate tree.
	ProtectedResources []string `protobuf:"bytes,20,rep,name=protected_resources,json=protectedResources,proto3" json:"protected_resources,omitempty"`
	// environment_digest is the digest of the sandbox environment the run executes in.
	EnvironmentDigest *v1.Digest `protobuf:"bytes,21,opt,name=environment_digest,json=environmentDigest,proto3" json:"environment_digest,omitempty"`
	// policy_digest is the digest of the policy bundle of the decision that allowed the dispatch.
	PolicyDigest *v1.Digest `protobuf:"bytes,22,opt,name=policy_digest,json=policyDigest,proto3" json:"policy_digest,omitempty"`
	// profile_digest is the digest of the SDLC profile in force.
	ProfileDigest *v1.Digest `protobuf:"bytes,23,opt,name=profile_digest,json=profileDigest,proto3" json:"profile_digest,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChangeExecutionRequest) Reset() {
	*x = ChangeExecutionRequest{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChangeExecutionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChangeExecutionRequest) ProtoMessage() {}

func (x *ChangeExecutionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChangeExecutionRequest.ProtoReflect.Descriptor instead.
func (*ChangeExecutionRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{6}
}

func (x *ChangeExecutionRequest) GetChangeId() string {
	if x != nil {
		return x.ChangeId
	}
	return ""
}

func (x *ChangeExecutionRequest) GetWorkUnitId() string {
	if x != nil {
		return x.WorkUnitId
	}
	return ""
}

func (x *ChangeExecutionRequest) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *ChangeExecutionRequest) GetGrantId() string {
	if x != nil {
		return x.GrantId
	}
	return ""
}

func (x *ChangeExecutionRequest) GetRepositoryDigest() *v1.Digest {
	if x != nil {
		return x.RepositoryDigest
	}
	return nil
}

func (x *ChangeExecutionRequest) GetWorktreeRef() *v1.Reference {
	if x != nil {
		return x.WorktreeRef
	}
	return nil
}

func (x *ChangeExecutionRequest) GetTaskRef() *v1.Reference {
	if x != nil {
		return x.TaskRef
	}
	return nil
}

func (x *ChangeExecutionRequest) GetPlanRef() *v1.Reference {
	if x != nil {
		return x.PlanRef
	}
	return nil
}

func (x *ChangeExecutionRequest) GetContextBundleRef() *v1.Reference {
	if x != nil {
		return x.ContextBundleRef
	}
	return nil
}

func (x *ChangeExecutionRequest) GetExecutorId() string {
	if x != nil {
		return x.ExecutorId
	}
	return ""
}

func (x *ChangeExecutionRequest) GetModelId() string {
	if x != nil {
		return x.ModelId
	}
	return ""
}

func (x *ChangeExecutionRequest) GetAllowedPaths() []string {
	if x != nil {
		return x.AllowedPaths
	}
	return nil
}

func (x *ChangeExecutionRequest) GetDeniedPaths() []string {
	if x != nil {
		return x.DeniedPaths
	}
	return nil
}

func (x *ChangeExecutionRequest) GetAllowedCapabilities() []v1.ExecutorCapability {
	if x != nil {
		return x.AllowedCapabilities
	}
	return nil
}

func (x *ChangeExecutionRequest) GetNetworkPolicyRef() *v1.Reference {
	if x != nil {
		return x.NetworkPolicyRef
	}
	return nil
}

func (x *ChangeExecutionRequest) GetCredentialCapabilities() []string {
	if x != nil {
		return x.CredentialCapabilities
	}
	return nil
}

func (x *ChangeExecutionRequest) GetBudget() *ExecutionBudget {
	if x != nil {
		return x.Budget
	}
	return nil
}

func (x *ChangeExecutionRequest) GetDeadline() *timestamppb.Timestamp {
	if x != nil {
		return x.Deadline
	}
	return nil
}

func (x *ChangeExecutionRequest) GetTestRequirementsRef() *v1.Reference {
	if x != nil {
		return x.TestRequirementsRef
	}
	return nil
}

func (x *ChangeExecutionRequest) GetProtectedResources() []string {
	if x != nil {
		return x.ProtectedResources
	}
	return nil
}

func (x *ChangeExecutionRequest) GetEnvironmentDigest() *v1.Digest {
	if x != nil {
		return x.EnvironmentDigest
	}
	return nil
}

func (x *ChangeExecutionRequest) GetPolicyDigest() *v1.Digest {
	if x != nil {
		return x.PolicyDigest
	}
	return nil
}

func (x *ChangeExecutionRequest) GetProfileDigest() *v1.Digest {
	if x != nil {
		return x.ProfileDigest
	}
	return nil
}

// ExecutionBudget is the effective budget of one run, resolved by the host from the grant (BudgetPolicy names); every field is non-zero. The deadline is the hard bound. The host measures files_changed and lines_changed on its own diff of the worktree and network_destinations at the egress proxy; tokens and tool_calls are measured by the host where inference and tool permission pass through it, and otherwise enforced by the provider and recorded as claims.
type ExecutionBudget struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// tokens is the model tokens the run may consume.
	Tokens uint32 `protobuf:"varint,1,opt,name=tokens,proto3" json:"tokens,omitempty"`
	// tool_calls is the tool calls the run may make, at most 300; a stream that still reaches the event cap is budget exhaustion (file comment).
	ToolCalls uint32 `protobuf:"varint,2,opt,name=tool_calls,json=toolCalls,proto3" json:"tool_calls,omitempty"`
	// files_changed is the worktree files the run may change.
	FilesChanged uint32 `protobuf:"varint,3,opt,name=files_changed,json=filesChanged,proto3" json:"files_changed,omitempty"`
	// lines_changed is the worktree lines the run may change.
	LinesChanged uint32 `protobuf:"varint,4,opt,name=lines_changed,json=linesChanged,proto3" json:"lines_changed,omitempty"`
	// network_destinations is the distinct egress destinations the run may reach.
	NetworkDestinations uint32 `protobuf:"varint,5,opt,name=network_destinations,json=networkDestinations,proto3" json:"network_destinations,omitempty"`
	unknownFields       protoimpl.UnknownFields
	sizeCache           protoimpl.SizeCache
}

func (x *ExecutionBudget) Reset() {
	*x = ExecutionBudget{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecutionBudget) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecutionBudget) ProtoMessage() {}

func (x *ExecutionBudget) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecutionBudget.ProtoReflect.Descriptor instead.
func (*ExecutionBudget) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{7}
}

func (x *ExecutionBudget) GetTokens() uint32 {
	if x != nil {
		return x.Tokens
	}
	return 0
}

func (x *ExecutionBudget) GetToolCalls() uint32 {
	if x != nil {
		return x.ToolCalls
	}
	return 0
}

func (x *ExecutionBudget) GetFilesChanged() uint32 {
	if x != nil {
		return x.FilesChanged
	}
	return 0
}

func (x *ExecutionBudget) GetLinesChanged() uint32 {
	if x != nil {
		return x.LinesChanged
	}
	return 0
}

func (x *ExecutionBudget) GetNetworkDestinations() uint32 {
	if x != nil {
		return x.NetworkDestinations
	}
	return 0
}

// ExecutionEvent is one event of the canonical execution stream; the stream rules are in the file comment.
type ExecutionEvent struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// type is required.
	Type ExecutionEventType `protobuf:"varint,1,opt,name=type,proto3,enum=numinance.ports.authorexecutor.v1.ExecutionEventType" json:"type,omitempty"`
	// sequence is the 1-based position of the event in the stream, 1 to 1023.
	Sequence uint64 `protobuf:"varint,2,opt,name=sequence,proto3" json:"sequence,omitempty"`
	// occurred_at is required: when the provider says the event happened; the host's receive time is authoritative.
	OccurredAt *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=occurred_at,json=occurredAt,proto3" json:"occurred_at,omitempty"`
	// subject is set only for these types, and empty for every other: TOOL_INTENT, TOOL_ALLOWED, TOOL_DENIED and TOOL_RESULT a tool name of the run's tool inventory, ^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$; FILE_CHANGED a path in the file comment's grammar, under allowed_paths and present in the host's diff of the worktree; CREDENTIAL_REQUEST a member of credential_capabilities. Untrusted: the host rejects any other value, scans subjects for issued handles and canaries and redacts them before anything is persisted, and renders them delimited and labelled.
	Subject string `protobuf:"bytes,4,opt,name=subject,proto3" json:"subject,omitempty"`
	// references point at the event's content (a diff, tool input and output, a command log, a test report), unique by artifact_ref, at most 16; each equals the request's task_ref, plan_ref, context_bundle_ref or test_requirements_ref, or names an artifact the host stored for this invocation and handed to the provider, and any other is rejected. plugin/v1 has no upload and hands no such reference yet, so until a plugin/v1 minor adds it only request references occur.
	References    []*v1.Reference `protobuf:"bytes,5,rep,name=references,proto3" json:"references,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExecutionEvent) Reset() {
	*x = ExecutionEvent{}
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecutionEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecutionEvent) ProtoMessage() {}

func (x *ExecutionEvent) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecutionEvent.ProtoReflect.Descriptor instead.
func (*ExecutionEvent) Descriptor() ([]byte, []int) {
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP(), []int{8}
}

func (x *ExecutionEvent) GetType() ExecutionEventType {
	if x != nil {
		return x.Type
	}
	return ExecutionEventType_EXECUTION_EVENT_TYPE_UNSPECIFIED
}

func (x *ExecutionEvent) GetSequence() uint64 {
	if x != nil {
		return x.Sequence
	}
	return 0
}

func (x *ExecutionEvent) GetOccurredAt() *timestamppb.Timestamp {
	if x != nil {
		return x.OccurredAt
	}
	return nil
}

func (x *ExecutionEvent) GetSubject() string {
	if x != nil {
		return x.Subject
	}
	return ""
}

func (x *ExecutionEvent) GetReferences() []*v1.Reference {
	if x != nil {
		return x.References
	}
	return nil
}

var File_numinance_ports_authorexecutor_v1_authorexecutor_proto protoreflect.FileDescriptor

const file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDesc = "" +
	"\n" +
	"6numinance/ports/authorexecutor/v1/authorexecutor.proto\x12!numinance.ports.authorexecutor.v1\x1a\x1fgoogle/protobuf/timestamp.proto\x1a*numinance/annotations/v1/annotations.proto\x1a\x1enuminance/types/v1/types.proto\"\x15\n" +
	"\x13CapabilitiesRequest\"o\n" +
	"\x14CapabilitiesResponse\x12W\n" +
	"\fcapabilities\x18\x01 \x03(\x0e2&.numinance.types.v1.ExecutorCapabilityB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fcapabilities\"r\n" +
	"\x0eExecuteRequest\x12`\n" +
	"\arequest\x18\x01 \x01(\v29.numinance.ports.authorexecutor.v1.ChangeExecutionRequestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\arequest\"o\n" +
	"\vPlanRequest\x12`\n" +
	"\arequest\x18\x01 \x01(\v29.numinance.ports.authorexecutor.v1.ChangeExecutionRequestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\arequest\"d\n" +
	"\fPlanResponse\x12T\n" +
	"\x05event\x18\x01 \x01(\v21.numinance.ports.authorexecutor.v1.ExecutionEventB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05event\"g\n" +
	"\x0fExecuteResponse\x12T\n" +
	"\x05event\x18\x01 \x01(\v21.numinance.ports.authorexecutor.v1.ExecutionEventB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05event\"\xc0\f\n" +
	"\x16ChangeExecutionRequest\x12(\n" +
	"\tchange_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bchangeId\x12-\n" +
	"\fwork_unit_id\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"workUnitId\x12\"\n" +
	"\x06run_id\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05runId\x12&\n" +
	"\bgrant_id\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\agrantId\x12T\n" +
	"\x11repository_digest\x18\x05 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x10repositoryDigest\x12O\n" +
	"\fworktree_ref\x18\x06 \x01(\v2\x1d.numinance.types.v1.ReferenceB\r\x92\xb5\x18\t\n" +
	"\x051.0.0(\x01R\vworktreeRef\x12E\n" +
	"\btask_ref\x18\a \x01(\v2\x1d.numinance.types.v1.ReferenceB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ataskRef\x12E\n" +
	"\bplan_ref\x18\b \x01(\v2\x1d.numinance.types.v1.ReferenceB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aplanRef\x12X\n" +
	"\x12context_bundle_ref\x18\t \x01(\v2\x1d.numinance.types.v1.ReferenceB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x10contextBundleRef\x12,\n" +
	"\vexecutor_id\x18\n" +
	" \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"executorId\x12&\n" +
	"\bmodel_id\x18\v \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\amodelId\x120\n" +
	"\rallowed_paths\x18\f \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fallowedPaths\x12.\n" +
	"\fdenied_paths\x18\r \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vdeniedPaths\x12f\n" +
	"\x14allowed_capabilities\x18\x0e \x03(\x0e2&.numinance.types.v1.ExecutorCapabilityB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x13allowedCapabilities\x12Z\n" +
	"\x12network_policy_ref\x18\x0f \x01(\v2\x1d.numinance.types.v1.ReferenceB\r\x92\xb5\x18\t\n" +
	"\x051.0.0(\x01R\x10networkPolicyRef\x12D\n" +
	"\x17credential_capabilities\x18\x10 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x16credentialCapabilities\x12W\n" +
	"\x06budget\x18\x11 \x01(\v22.numinance.ports.authorexecutor.v1.ExecutionBudgetB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06budget\x12C\n" +
	"\bdeadline\x18\x12 \x01(\v2\x1a.google.protobuf.TimestampB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bdeadline\x12^\n" +
	"\x15test_requirements_ref\x18\x13 \x01(\v2\x1d.numinance.types.v1.ReferenceB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x13testRequirementsRef\x12<\n" +
	"\x13protected_resources\x18\x14 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x12protectedResources\x12V\n" +
	"\x12environment_digest\x18\x15 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x11environmentDigest\x12L\n" +
	"\rpolicy_digest\x18\x16 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fpolicyDigest\x12N\n" +
	"\x0eprofile_digest\x18\x17 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rprofileDigest\"\x86\x02\n" +
	"\x0fExecutionBudget\x12#\n" +
	"\x06tokens\x18\x01 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06tokens\x12*\n" +
	"\n" +
	"tool_calls\x18\x02 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ttoolCalls\x120\n" +
	"\rfiles_changed\x18\x03 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\ffilesChanged\x120\n" +
	"\rlines_changed\x18\x04 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\flinesChanged\x12>\n" +
	"\x14network_destinations\x18\x05 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x13networkDestinations\"\xce\x02\n" +
	"\x0eExecutionEvent\x12V\n" +
	"\x04type\x18\x01 \x01(\x0e25.numinance.ports.authorexecutor.v1.ExecutionEventTypeB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04type\x12'\n" +
	"\bsequence\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bsequence\x12H\n" +
	"\voccurred_at\x18\x03 \x01(\v2\x1a.google.protobuf.TimestampB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"occurredAt\x12%\n" +
	"\asubject\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\asubject\x12J\n" +
	"\n" +
	"references\x18\x05 \x03(\v2\x1d.numinance.types.v1.ReferenceB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"references*\x9b\x05\n" +
	"\x12ExecutionEventType\x12$\n" +
	" EXECUTION_EVENT_TYPE_UNSPECIFIED\x10\x00\x12*\n" +
	"&EXECUTION_EVENT_TYPE_EXECUTION_STARTED\x10\x01\x12&\n" +
	"\"EXECUTION_EVENT_TYPE_PLAN_ACCEPTED\x10\x02\x12$\n" +
	" EXECUTION_EVENT_TYPE_TOOL_INTENT\x10\x03\x12%\n" +
	"!EXECUTION_EVENT_TYPE_TOOL_ALLOWED\x10\x04\x12$\n" +
	" EXECUTION_EVENT_TYPE_TOOL_DENIED\x10\x05\x12$\n" +
	" EXECUTION_EVENT_TYPE_TOOL_RESULT\x10\x06\x12%\n" +
	"!EXECUTION_EVENT_TYPE_FILE_CHANGED\x10\a\x12)\n" +
	"%EXECUTION_EVENT_TYPE_COMMAND_EXECUTED\x10\b\x12(\n" +
	"$EXECUTION_EVENT_TYPE_NETWORK_REQUEST\x10\t\x12+\n" +
	"'EXECUTION_EVENT_TYPE_CREDENTIAL_REQUEST\x10\n" +
	"\x12$\n" +
	" EXECUTION_EVENT_TYPE_TEST_RESULT\x10\v\x12#\n" +
	"\x1fEXECUTION_EVENT_TYPE_CHECKPOINT\x10\f\x12%\n" +
	"!EXECUTION_EVENT_TYPE_BUDGET_EVENT\x10\r\x12,\n" +
	"(EXECUTION_EVENT_TYPE_EXECUTION_COMPLETED\x10\x0e\x12)\n" +
	"%EXECUTION_EVENT_TYPE_EXECUTION_FAILED\x10\x0f2\xa7\x05\n" +
	"\x15AuthorExecutorService\x12\xd8\x01\n" +
	"\fCapabilities\x126.numinance.ports.authorexecutor.v1.CapabilitiesRequest\x1a7.numinance.ports.authorexecutor.v1.CapabilitiesResponse\"W\x8a\xb5\x18S\b\x01\x10\x01\x18\x01B\x051.0.0JDReturns the executor capabilities an author executor adapter claims.\x12\xe1\x01\n" +
	"\aExecute\x121.numinance.ports.authorexecutor.v1.ExecuteRequest\x1a2.numinance.ports.authorexecutor.v1.ExecuteResponse\"m\x8a\xb5\x18i\b\x03\x10\x02B\x051.0.0J\\Runs a coding harness for one work unit inside its sandbox and streams the execution events.0\x01\x12\xce\x01\n" +
	"\x04Plan\x12..numinance.ports.authorexecutor.v1.PlanRequest\x1a/.numinance.ports.authorexecutor.v1.PlanResponse\"c\x8a\xb5\x18_\b\x01\x10\x02B\x051.0.0JRRuns a coding harness read-only to plan a change and streams the execution events.0\x01B\xc6\x02\n" +
	"%com.numinance.ports.authorexecutor.v1B\x13AuthorexecutorProtoP\x01Zagithub.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1;authorexecutorv1\xa2\x02\x03NPA\xaa\x02!Numinance.Ports.Authorexecutor.V1\xca\x02!Numinance\\Ports\\Authorexecutor\\V1\xe2\x02-Numinance\\Ports\\Authorexecutor\\V1\\GPBMetadata\xea\x02$Numinance::Ports::Authorexecutor::V1b\x06proto3"

var (
	file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescOnce sync.Once
	file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescData []byte
)

func file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescGZIP() []byte {
	file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescOnce.Do(func() {
		file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDesc), len(file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDesc)))
	})
	return file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDescData
}

var file_numinance_ports_authorexecutor_v1_authorexecutor_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_numinance_ports_authorexecutor_v1_authorexecutor_proto_goTypes = []any{
	(ExecutionEventType)(0),        // 0: numinance.ports.authorexecutor.v1.ExecutionEventType
	(*CapabilitiesRequest)(nil),    // 1: numinance.ports.authorexecutor.v1.CapabilitiesRequest
	(*CapabilitiesResponse)(nil),   // 2: numinance.ports.authorexecutor.v1.CapabilitiesResponse
	(*ExecuteRequest)(nil),         // 3: numinance.ports.authorexecutor.v1.ExecuteRequest
	(*PlanRequest)(nil),            // 4: numinance.ports.authorexecutor.v1.PlanRequest
	(*PlanResponse)(nil),           // 5: numinance.ports.authorexecutor.v1.PlanResponse
	(*ExecuteResponse)(nil),        // 6: numinance.ports.authorexecutor.v1.ExecuteResponse
	(*ChangeExecutionRequest)(nil), // 7: numinance.ports.authorexecutor.v1.ChangeExecutionRequest
	(*ExecutionBudget)(nil),        // 8: numinance.ports.authorexecutor.v1.ExecutionBudget
	(*ExecutionEvent)(nil),         // 9: numinance.ports.authorexecutor.v1.ExecutionEvent
	(v1.ExecutorCapability)(0),     // 10: numinance.types.v1.ExecutorCapability
	(*v1.Digest)(nil),              // 11: numinance.types.v1.Digest
	(*v1.Reference)(nil),           // 12: numinance.types.v1.Reference
	(*timestamppb.Timestamp)(nil),  // 13: google.protobuf.Timestamp
}
var file_numinance_ports_authorexecutor_v1_authorexecutor_proto_depIdxs = []int32{
	10, // 0: numinance.ports.authorexecutor.v1.CapabilitiesResponse.capabilities:type_name -> numinance.types.v1.ExecutorCapability
	7,  // 1: numinance.ports.authorexecutor.v1.ExecuteRequest.request:type_name -> numinance.ports.authorexecutor.v1.ChangeExecutionRequest
	7,  // 2: numinance.ports.authorexecutor.v1.PlanRequest.request:type_name -> numinance.ports.authorexecutor.v1.ChangeExecutionRequest
	9,  // 3: numinance.ports.authorexecutor.v1.PlanResponse.event:type_name -> numinance.ports.authorexecutor.v1.ExecutionEvent
	9,  // 4: numinance.ports.authorexecutor.v1.ExecuteResponse.event:type_name -> numinance.ports.authorexecutor.v1.ExecutionEvent
	11, // 5: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.repository_digest:type_name -> numinance.types.v1.Digest
	12, // 6: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.worktree_ref:type_name -> numinance.types.v1.Reference
	12, // 7: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.task_ref:type_name -> numinance.types.v1.Reference
	12, // 8: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.plan_ref:type_name -> numinance.types.v1.Reference
	12, // 9: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.context_bundle_ref:type_name -> numinance.types.v1.Reference
	10, // 10: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.allowed_capabilities:type_name -> numinance.types.v1.ExecutorCapability
	12, // 11: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.network_policy_ref:type_name -> numinance.types.v1.Reference
	8,  // 12: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.budget:type_name -> numinance.ports.authorexecutor.v1.ExecutionBudget
	13, // 13: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.deadline:type_name -> google.protobuf.Timestamp
	12, // 14: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.test_requirements_ref:type_name -> numinance.types.v1.Reference
	11, // 15: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.environment_digest:type_name -> numinance.types.v1.Digest
	11, // 16: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.policy_digest:type_name -> numinance.types.v1.Digest
	11, // 17: numinance.ports.authorexecutor.v1.ChangeExecutionRequest.profile_digest:type_name -> numinance.types.v1.Digest
	0,  // 18: numinance.ports.authorexecutor.v1.ExecutionEvent.type:type_name -> numinance.ports.authorexecutor.v1.ExecutionEventType
	13, // 19: numinance.ports.authorexecutor.v1.ExecutionEvent.occurred_at:type_name -> google.protobuf.Timestamp
	12, // 20: numinance.ports.authorexecutor.v1.ExecutionEvent.references:type_name -> numinance.types.v1.Reference
	1,  // 21: numinance.ports.authorexecutor.v1.AuthorExecutorService.Capabilities:input_type -> numinance.ports.authorexecutor.v1.CapabilitiesRequest
	3,  // 22: numinance.ports.authorexecutor.v1.AuthorExecutorService.Execute:input_type -> numinance.ports.authorexecutor.v1.ExecuteRequest
	4,  // 23: numinance.ports.authorexecutor.v1.AuthorExecutorService.Plan:input_type -> numinance.ports.authorexecutor.v1.PlanRequest
	2,  // 24: numinance.ports.authorexecutor.v1.AuthorExecutorService.Capabilities:output_type -> numinance.ports.authorexecutor.v1.CapabilitiesResponse
	6,  // 25: numinance.ports.authorexecutor.v1.AuthorExecutorService.Execute:output_type -> numinance.ports.authorexecutor.v1.ExecuteResponse
	5,  // 26: numinance.ports.authorexecutor.v1.AuthorExecutorService.Plan:output_type -> numinance.ports.authorexecutor.v1.PlanResponse
	24, // [24:27] is the sub-list for method output_type
	21, // [21:24] is the sub-list for method input_type
	21, // [21:21] is the sub-list for extension type_name
	21, // [21:21] is the sub-list for extension extendee
	0,  // [0:21] is the sub-list for field type_name
}

func init() { file_numinance_ports_authorexecutor_v1_authorexecutor_proto_init() }
func file_numinance_ports_authorexecutor_v1_authorexecutor_proto_init() {
	if File_numinance_ports_authorexecutor_v1_authorexecutor_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDesc), len(file_numinance_ports_authorexecutor_v1_authorexecutor_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_authorexecutor_v1_authorexecutor_proto_goTypes,
		DependencyIndexes: file_numinance_ports_authorexecutor_v1_authorexecutor_proto_depIdxs,
		EnumInfos:         file_numinance_ports_authorexecutor_v1_authorexecutor_proto_enumTypes,
		MessageInfos:      file_numinance_ports_authorexecutor_v1_authorexecutor_proto_msgTypes,
	}.Build()
	File_numinance_ports_authorexecutor_v1_authorexecutor_proto = out.File
	file_numinance_ports_authorexecutor_v1_authorexecutor_proto_goTypes = nil
	file_numinance_ports_authorexecutor_v1_authorexecutor_proto_depIdxs = nil
}
