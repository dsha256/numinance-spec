// The sandboxprovider port (ADR 0006 as amended by ADR 0033): creates, runs, snapshots and destroys the one sandbox of one run of one work unit on the runner's host. One sandbox per run: a work unit's later run gets a new sandbox, restored from a snapshot.
// Callers: only the sandbox context in the runner calls this port, under the runner identity; the host refuses every other actor kind. The port is never a model tool, and no tool_groups or scopes are declared. The provider runs on the runner's host with authority over mounts, so only a provider bound to the runtime deployment serves it; the host refuses one installed from the registry. The worktree, artifact and output directories are host directories; the host reads them itself.
// Authority: nothing a provider reports is authority. The host builds every SandboxSpec from the run's grant, the policy decision and the plan, and refuses to dispatch a request whose change_id, work_unit_id or run_id differs from the ActionContext, whose deadline is later than the ActionContext deadline, or whose profile is below the floor minimum (compared through the floors' explicit table, never by number) or the work unit's ceiling. ADR 0033's ceiling bounds the ladder level and risk class of the work unit, checked before Create; the port's verbs act inside an admitted work unit, and the ceiling never lowers the risk class decided for an action (an Exec under PROCESS stays PRIVILEGED). The provider creates exactly the requested profile or refuses; it never downgrades. Every answer is a provider claim: the host recomputes the digests of the files it reads, and after a restore the tree digests of both directories against the ones Snapshot returned, and treats a mismatch as a provider fault; that a capture matched the run's state rests on the deployment-bound provider.
// Isolation, under every profile: the image is the one the runner holds under image_digest and is never pulled; the environment holds only the Exec env and TRACEPARENT, nothing of the host environment; the run's processes run as an unprivileged OS user dedicated to the run, which can read no runner, broker, egress proxy, provider or other run state, no host credential (cloud, SSH agent, git, instance role) and no canary. The host's worktree holds no credential: its git configuration has no credential helper, token-bearing URL or extra header. Under CONTAINER and GVISOR also: read-only root filesystem, writable tmpfs, all capabilities dropped, seccomp, no-new-privileges, a user namespace, the limits of SandboxLimits, and no host path but the worktree and artifact directories; GVISOR adds the user-space kernel. Their write model: at Create the provider hands every entry at or under allowed_paths, and the artifact tree, to the run's uid as in the PROCESS Create handover, so whatever the run creates or changes is owned on disk by the run's uid, never through a mapping to the host's user; every other entry stays host-owned and read-only for the run. An allowed path missing at Create is writable only under another allowed path, so the host refuses a request with an allowed path that neither exists nor lies under another (allow the directory to create a file in it). Their network namespace reaches only egress_proxy, which is unique to the run (the proxy binds it to this sandbox); every other address, DNS included, is unreachable, and with no egress_proxy there is no network. PROCESS is a direct child process: freeze, kill and Destroy act on every process of the run's OS user, never only on a process tree; the run gets a private temporary directory as TMPDIR; the worktree, artifact and TMPDIR roots are 0700 and owned by the run's user, under a work root no other run's user can traverse, and at Create the provider hands the host-created trees to the run's user without following links, with modes normalized as at restore and the Create rules for ACLs, extended attributes, labels and file flags (Destroy comment); Destroy removes every file the user owns outside the worktree and artifact directories, which the host keeps, and the user is never reused. Under every profile, neither the run's user nor its numeric uid is reused while any directory it owns is kept, which after a complete Destroy handover is none, so this binds only while a Destroy is unfinished. Host paths readable by the run's user and the host network are reachable, egress_proxy must be empty, the provider enforces the deadline and the limits the OS supports, and a provider that cannot run each run as its own OS user refuses PROCESS. The host offers PROCESS only on a host where, at each Create, an active probe of the metadata addresses, made as the run's user, fails and no cloud credential source is present, and the decision for an Exec under PROCESS counts its egress as unclassified, so PRIVILEGED. Under every profile the run's user can connect to no runner, broker, egress proxy or container runtime socket other than its own proxy, and those endpoints authenticate their caller.
// Paths: worktree paths follow the path rules of the authorexecutor port (file comment, Paths); overlap checks and masking compare them under NFC and Unicode simple case folding. Under CONTAINER and GVISOR only allowed_paths are writable, enforced on the resolved path after following symlinks, and hardlink creation is refused; every other worktree path, .numinance/context/ included (the host neither allows nor masks it), is read-only. Each masked path is absent: its content is not readable and nothing can be created at or under it. Under GVISOR the provider also refuses to create, at any depth, an entry whose name case-folds to .git, .gitmodules, .gitattributes or .lfsconfig. Under PROCESS the sandbox enforces no worktree path rule: the host leaves every masked path out of the worktree it creates, so it is absent on disk, and its candidate check covers the rest. A masked path absent on disk (PROCESS, or after a restore) is never a deletion: the candidate takes it from the run's base (authorexecutor port, Paths). Whatever a profile does not enforce in the sandbox, and floor patterns under every profile, the host's check of the candidate tree enforces (authorexecutor port, Paths). Host paths are absolute, '/'-separated and printable, with no '.' or '..' segment, resolved with no symlink component under the provider's configured work root (FAILED_PRECONDITION); one equal to, above or under a path of a live sandbox, or one that held a directory of a destroyed run, is refused (FAILED_PRECONDITION): every run gets new, never-used worktree, artifact and output directories. The host creates the trees with no non-directory of more than one link (a clone copies objects, never hardlinks them) and every entry owned by its own user; under every profile the provider refuses Create (FAILED_PRECONDITION) of a tree holding either. The last segment of each host path is the run_id, which keys one sandbox, so reuse is detectable without state; the provider's refusal of a destroyed run's path is best-effort within its key retention, and the host's check (HOST-001) is the durable one. A grammar breach, a last segment other than run_id, or an overlap within one spec is INVALID_ARGUMENT.
// Output: the provider writes each exec's standard output and error to exec/<exec_seq>/stdout and exec/<exec_seq>/stderr under output_path, a host directory never mounted in the sandbox and, with its parents, neither readable nor writable by the run's user under every profile, creating each with exclusive create and resolving every component with no symlink, and kills an exec whose output of the run exceeds disk_bytes (LIMIT_EXCEEDED). The host reads them, and computes every directory digest, with no symlink following and through a size-limited reader. Output is untrusted data: the host scans it for issued handles and canaries, and labels and delimits it before any prompt use.
// Payloads: requests and responses carry ids, paths, digests and counts, never file contents or output (a platform invariant: payloads carry references, never content).
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; ALREADY_EXISTS for a Create or Exec whose key is taken by a different request; FAILED_PRECONDITION for a profile the host does not offer or MICROVM, an image or snapshot the provider does not hold or that fails its digest, a host path whose resolution, work root, live overlap or earlier use by a destroyed run the Paths rule refuses, a host tree holding a multi-link file or another user's entry at Create, a restore into non-empty directories or from a snapshot holding an entry at or under a masked path of the new spec, a Create of a destroyed run, and an Exec or Snapshot of a run that is not running; NOT_FOUND for a Wait of an unknown exec; RESOURCE_EXHAUSTED when the host has no capacity, a run has 16 execs running or 64 snapshots, or a snapshot exceeds disk_bytes or the per-host byte cap; UNAVAILABLE for a runtime failure the caller may retry, which creates nothing. A provider never reports an effect it did not make.
// Evidence: the host records the requested profile, image digest and limits, the provider's claims and its own checks. No RPC declares evidence yet; its record types are fixed with the sandbox context, and no spec tag ships while they are missing.
// Bounds: every string is at most 1024 bytes and every repeated field at most 256 entries unless stated; over-limit input is rejected, never truncated; duplicate entries in any repeated field are rejected; enums are closed, and zero and unknown numbers are rejected; a duration is never negative. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Every element is since 1.0.0.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/sandboxprovider/v1/sandboxprovider.proto

package sandboxproviderv1

import (
	_ "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/evidence/v1"
	v11 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
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

// ExecState is the state of one exec.
type ExecState int32

const (
	// EXEC_STATE_UNSPECIFIED is the zero value; it is rejected.
	ExecState_EXEC_STATE_UNSPECIFIED ExecState = 0
	// EXEC_STATE_RUNNING: the command has not ended.
	ExecState_EXEC_STATE_RUNNING ExecState = 1
	// EXEC_STATE_EXITED: the command ended by itself; exit_code is its status.
	ExecState_EXEC_STATE_EXITED ExecState = 2
	// EXEC_STATE_TIMED_OUT: the provider killed the command at its timeout.
	ExecState_EXEC_STATE_TIMED_OUT ExecState = 3
	// EXEC_STATE_LIMIT_EXCEEDED: the provider killed the command at a limit of SandboxLimits or the output bound.
	ExecState_EXEC_STATE_LIMIT_EXCEEDED ExecState = 4
	// EXEC_STATE_KILLED: Destroy or the sandbox deadline ended the command.
	ExecState_EXEC_STATE_KILLED ExecState = 5
	// EXEC_STATE_START_FAILED: the command did not start (for example, argv[0] does not exist).
	ExecState_EXEC_STATE_START_FAILED ExecState = 6
)

// Enum value maps for ExecState.
var (
	ExecState_name = map[int32]string{
		0: "EXEC_STATE_UNSPECIFIED",
		1: "EXEC_STATE_RUNNING",
		2: "EXEC_STATE_EXITED",
		3: "EXEC_STATE_TIMED_OUT",
		4: "EXEC_STATE_LIMIT_EXCEEDED",
		5: "EXEC_STATE_KILLED",
		6: "EXEC_STATE_START_FAILED",
	}
	ExecState_value = map[string]int32{
		"EXEC_STATE_UNSPECIFIED":    0,
		"EXEC_STATE_RUNNING":        1,
		"EXEC_STATE_EXITED":         2,
		"EXEC_STATE_TIMED_OUT":      3,
		"EXEC_STATE_LIMIT_EXCEEDED": 4,
		"EXEC_STATE_KILLED":         5,
		"EXEC_STATE_START_FAILED":   6,
	}
)

func (x ExecState) Enum() *ExecState {
	p := new(ExecState)
	*p = x
	return p
}

func (x ExecState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ExecState) Descriptor() protoreflect.EnumDescriptor {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_enumTypes[0].Descriptor()
}

func (ExecState) Type() protoreflect.EnumType {
	return &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_enumTypes[0]
}

func (x ExecState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ExecState.Descriptor instead.
func (ExecState) EnumDescriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{0}
}

// CreateRequest carries the sandbox specification of one run.
type CreateRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// spec is required.
	Spec          *SandboxSpec `protobuf:"bytes,1,opt,name=spec,proto3" json:"spec,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateRequest) Reset() {
	*x = CreateRequest{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateRequest) ProtoMessage() {}

func (x *CreateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateRequest.ProtoReflect.Descriptor instead.
func (*CreateRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{0}
}

func (x *CreateRequest) GetSpec() *SandboxSpec {
	if x != nil {
		return x.Spec
	}
	return nil
}

// CreateResponse is the provider's answer to Create, or to its dry run; a claim.
type CreateResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// profile is the profile the provider created; it equals spec.profile, or the host destroys the sandbox and records a provider fault.
	Profile v1.SandboxProfile `protobuf:"varint,1,opt,name=profile,proto3,enum=numinance.evidence.v1.SandboxProfile" json:"profile,omitempty"`
	// runtime_version is the provider's runtime name and version, printable, 1 to 128 bytes; the host checks it against the runner's host attestation.
	RuntimeVersion string `protobuf:"bytes,2,opt,name=runtime_version,json=runtimeVersion,proto3" json:"runtime_version,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *CreateResponse) Reset() {
	*x = CreateResponse{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateResponse) ProtoMessage() {}

func (x *CreateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateResponse.ProtoReflect.Descriptor instead.
func (*CreateResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{1}
}

func (x *CreateResponse) GetProfile() v1.SandboxProfile {
	if x != nil {
		return x.Profile
	}
	return v1.SandboxProfile(0)
}

func (x *CreateResponse) GetRuntimeVersion() string {
	if x != nil {
		return x.RuntimeVersion
	}
	return ""
}

// SandboxSpec is everything one sandbox is given. Every field is required unless its comment says it may be unset or empty.
type SandboxSpec struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// change_id is ^chg_[0-9a-f]{32}$.
	ChangeId string `protobuf:"bytes,1,opt,name=change_id,json=changeId,proto3" json:"change_id,omitempty"`
	// work_unit_id is ^wu_[0-9a-f]{32}$.
	WorkUnitId string `protobuf:"bytes,2,opt,name=work_unit_id,json=workUnitId,proto3" json:"work_unit_id,omitempty"`
	// run_id is ^run_[0-9a-f]{32}$ and keys the sandbox; a retry of a run is a new run with a new sandbox.
	RunId string `protobuf:"bytes,3,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	// profile is PROCESS, CONTAINER or GVISOR; MICROVM is refused (ADR 0033).
	Profile v1.SandboxProfile `protobuf:"varint,4,opt,name=profile,proto3,enum=numinance.evidence.v1.SandboxProfile" json:"profile,omitempty"`
	// image_digest is the digest of the image the runner pulled and verified (signature and lockfile pin) before Create; required under CONTAINER and GVISOR, unset under PROCESS.
	ImageDigest *v11.Digest `protobuf:"bytes,5,opt,name=image_digest,json=imageDigest,proto3" json:"image_digest,omitempty"`
	// worktree_path is the host path of the run's worktree (file comment, Paths); mounted as the sandbox's working directory.
	WorktreePath string `protobuf:"bytes,6,opt,name=worktree_path,json=worktreePath,proto3" json:"worktree_path,omitempty"`
	// artifacts_path is the host path of the run's artifact directory, not equal to, above or under another path of this spec; mounted writable.
	ArtifactsPath string `protobuf:"bytes,7,opt,name=artifacts_path,json=artifactsPath,proto3" json:"artifacts_path,omitempty"`
	// allowed_paths are the only writable worktree paths; empty for a read-only run. None equals, contains or lies under a masked path.
	AllowedPaths []string `protobuf:"bytes,8,rep,name=allowed_paths,json=allowedPaths,proto3" json:"allowed_paths,omitempty"`
	// masked_paths are the worktree paths absent from the sandbox: the host's literal protected set and the run's denied paths, at least one.
	MaskedPaths []string `protobuf:"bytes,9,rep,name=masked_paths,json=maskedPaths,proto3" json:"masked_paths,omitempty"`
	// egress_proxy is the address of the run's own egress proxy, an IP literal in canonical form (IPv4 dotted-quad with no leading zeros; IPv6 in RFC 5952 form, in brackets, with no zone id) and a port 1 to 65535; never loopback, link-local, unspecified, multicast or broadcast, a metadata address (169.254.169.254, 169.254.170.2, fd00:ec2::254, 168.63.129.16), or the IPv4-mapped or NAT64 (64:ff9b::/96) form of any of those. Empty for no network, and under PROCESS.
	EgressProxy string `protobuf:"bytes,10,opt,name=egress_proxy,json=egressProxy,proto3" json:"egress_proxy,omitempty"`
	// limits are the run's resource limits from its budget.
	Limits *SandboxLimits `protobuf:"bytes,11,opt,name=limits,proto3" json:"limits,omitempty"`
	// deadline is when the sandbox ends, in the future at Create; the provider destroys it then, as Destroy without release_snapshots.
	Deadline *timestamppb.Timestamp `protobuf:"bytes,12,opt,name=deadline,proto3" json:"deadline,omitempty"`
	// traceparent is the W3C trace context, ^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$, set as TRACEPARENT in every exec; may be empty.
	Traceparent string `protobuf:"bytes,13,opt,name=traceparent,proto3" json:"traceparent,omitempty"`
	// restore_from is a snapshot of an earlier run of the same work unit, whose reference is still held (the provider refuses another with FAILED_PRECONDITION); the provider fills the empty worktree and artifact directories from it before the sandbox starts, creating every entry beneath its directory without following any symlink and never writing through a restored link, and refuses (FAILED_PRECONDITION) a snapshot holding an entry at or under this spec's masked_paths. Restore copies content and never hardlinks to provider storage; it creates directories 0755 and files 0644, or 0755 with the owner execute bit, every entry owned by the run's user, with no setuid, setgid or sticky bit and no group or other write. The host then recomputes both tree digests and compares them with the ones Snapshot returned, and checks that no masked path is present on disk, before the first Exec. Unset for a fresh sandbox.
	RestoreFrom *v11.Digest `protobuf:"bytes,14,opt,name=restore_from,json=restoreFrom,proto3" json:"restore_from,omitempty"`
	// output_path is the host path of the run's output directory (file comment, Output), not equal to, above or under another path of this spec, and empty at Create; never mounted in the sandbox.
	OutputPath    string `protobuf:"bytes,15,opt,name=output_path,json=outputPath,proto3" json:"output_path,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SandboxSpec) Reset() {
	*x = SandboxSpec{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SandboxSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SandboxSpec) ProtoMessage() {}

func (x *SandboxSpec) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SandboxSpec.ProtoReflect.Descriptor instead.
func (*SandboxSpec) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{2}
}

func (x *SandboxSpec) GetChangeId() string {
	if x != nil {
		return x.ChangeId
	}
	return ""
}

func (x *SandboxSpec) GetWorkUnitId() string {
	if x != nil {
		return x.WorkUnitId
	}
	return ""
}

func (x *SandboxSpec) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *SandboxSpec) GetProfile() v1.SandboxProfile {
	if x != nil {
		return x.Profile
	}
	return v1.SandboxProfile(0)
}

func (x *SandboxSpec) GetImageDigest() *v11.Digest {
	if x != nil {
		return x.ImageDigest
	}
	return nil
}

func (x *SandboxSpec) GetWorktreePath() string {
	if x != nil {
		return x.WorktreePath
	}
	return ""
}

func (x *SandboxSpec) GetArtifactsPath() string {
	if x != nil {
		return x.ArtifactsPath
	}
	return ""
}

func (x *SandboxSpec) GetAllowedPaths() []string {
	if x != nil {
		return x.AllowedPaths
	}
	return nil
}

func (x *SandboxSpec) GetMaskedPaths() []string {
	if x != nil {
		return x.MaskedPaths
	}
	return nil
}

func (x *SandboxSpec) GetEgressProxy() string {
	if x != nil {
		return x.EgressProxy
	}
	return ""
}

func (x *SandboxSpec) GetLimits() *SandboxLimits {
	if x != nil {
		return x.Limits
	}
	return nil
}

func (x *SandboxSpec) GetDeadline() *timestamppb.Timestamp {
	if x != nil {
		return x.Deadline
	}
	return nil
}

func (x *SandboxSpec) GetTraceparent() string {
	if x != nil {
		return x.Traceparent
	}
	return ""
}

func (x *SandboxSpec) GetRestoreFrom() *v11.Digest {
	if x != nil {
		return x.RestoreFrom
	}
	return nil
}

func (x *SandboxSpec) GetOutputPath() string {
	if x != nil {
		return x.OutputPath
	}
	return ""
}

// SandboxLimits are the resource limits of one sandbox; every field is non-zero. The wall clock is SandboxSpec.deadline.
type SandboxLimits struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// cpu_millicores is the CPU the sandbox may use, in thousandths of a core.
	CpuMillicores uint32 `protobuf:"varint,1,opt,name=cpu_millicores,json=cpuMillicores,proto3" json:"cpu_millicores,omitempty"`
	// memory_bytes is the memory the sandbox may use.
	MemoryBytes uint64 `protobuf:"varint,2,opt,name=memory_bytes,json=memoryBytes,proto3" json:"memory_bytes,omitempty"`
	// pids is the number of processes and threads the sandbox may hold at once.
	Pids uint32 `protobuf:"varint,3,opt,name=pids,proto3" json:"pids,omitempty"`
	// disk_bytes bounds what the sandbox may write to the worktree, the artifact directory and its tmpfs together, and separately the run's output.
	DiskBytes     uint64 `protobuf:"varint,4,opt,name=disk_bytes,json=diskBytes,proto3" json:"disk_bytes,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SandboxLimits) Reset() {
	*x = SandboxLimits{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SandboxLimits) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SandboxLimits) ProtoMessage() {}

func (x *SandboxLimits) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SandboxLimits.ProtoReflect.Descriptor instead.
func (*SandboxLimits) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{3}
}

func (x *SandboxLimits) GetCpuMillicores() uint32 {
	if x != nil {
		return x.CpuMillicores
	}
	return 0
}

func (x *SandboxLimits) GetMemoryBytes() uint64 {
	if x != nil {
		return x.MemoryBytes
	}
	return 0
}

func (x *SandboxLimits) GetPids() uint32 {
	if x != nil {
		return x.Pids
	}
	return 0
}

func (x *SandboxLimits) GetDiskBytes() uint64 {
	if x != nil {
		return x.DiskBytes
	}
	return 0
}

// ExecRequest starts one command.
type ExecRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// run_id is the sandbox, ^run_[0-9a-f]{32}$.
	RunId string `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	// exec_seq is the host's number for this exec within the run, 1 to 1024.
	ExecSeq uint32 `protobuf:"varint,2,opt,name=exec_seq,json=execSeq,proto3" json:"exec_seq,omitempty"`
	// argv is the command and its arguments, 1 to 256 entries, each printable; run directly, never through a shell. Untrusted content; the host scans it for issued handles and canaries before dispatch.
	Argv []string `protobuf:"bytes,3,rep,name=argv,proto3" json:"argv,omitempty"`
	// working_dir is a worktree path the command starts in, not at or under a masked path; empty for the worktree root.
	WorkingDir string `protobuf:"bytes,4,opt,name=working_dir,json=workingDir,proto3" json:"working_dir,omitempty"`
	// env is the command's whole environment besides TRACEPARENT, at most 64 entries, unique by name; never a credential, a handle or a token, and scanned by the host like argv.
	Env []*EnvVar `protobuf:"bytes,5,rep,name=env,proto3" json:"env,omitempty"`
	// timeout is how long the command may run, 1 second up to the time left before the sandbox deadline, checked only for a new key (Exec).
	Timeout       *durationpb.Duration `protobuf:"bytes,6,opt,name=timeout,proto3" json:"timeout,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExecRequest) Reset() {
	*x = ExecRequest{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecRequest) ProtoMessage() {}

func (x *ExecRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecRequest.ProtoReflect.Descriptor instead.
func (*ExecRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{4}
}

func (x *ExecRequest) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *ExecRequest) GetExecSeq() uint32 {
	if x != nil {
		return x.ExecSeq
	}
	return 0
}

func (x *ExecRequest) GetArgv() []string {
	if x != nil {
		return x.Argv
	}
	return nil
}

func (x *ExecRequest) GetWorkingDir() string {
	if x != nil {
		return x.WorkingDir
	}
	return ""
}

func (x *ExecRequest) GetEnv() []*EnvVar {
	if x != nil {
		return x.Env
	}
	return nil
}

func (x *ExecRequest) GetTimeout() *durationpb.Duration {
	if x != nil {
		return x.Timeout
	}
	return nil
}

// EnvVar is one environment variable.
type EnvVar struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// name is ^[A-Za-z_][A-Za-z0-9_]{0,127}$ and not TRACEPARENT.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// value is printable; may be empty.
	Value         string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *EnvVar) Reset() {
	*x = EnvVar{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EnvVar) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnvVar) ProtoMessage() {}

func (x *EnvVar) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnvVar.ProtoReflect.Descriptor instead.
func (*EnvVar) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{5}
}

func (x *EnvVar) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *EnvVar) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

// ExecResponse is empty: the exec is addressed by (run_id, exec_seq).
type ExecResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExecResponse) Reset() {
	*x = ExecResponse{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecResponse) ProtoMessage() {}

func (x *ExecResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecResponse.ProtoReflect.Descriptor instead.
func (*ExecResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{6}
}

// WaitRequest asks for the state of one exec.
type WaitRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// run_id is ^run_[0-9a-f]{32}$.
	RunId string `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	// exec_seq is 1 to 1024.
	ExecSeq uint32 `protobuf:"varint,2,opt,name=exec_seq,json=execSeq,proto3" json:"exec_seq,omitempty"`
	// wait_timeout is how long Wait may block, 0 to 60 seconds; unset or 0 answers at once.
	WaitTimeout   *durationpb.Duration `protobuf:"bytes,3,opt,name=wait_timeout,json=waitTimeout,proto3" json:"wait_timeout,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *WaitRequest) Reset() {
	*x = WaitRequest{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *WaitRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WaitRequest) ProtoMessage() {}

func (x *WaitRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WaitRequest.ProtoReflect.Descriptor instead.
func (*WaitRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{7}
}

func (x *WaitRequest) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *WaitRequest) GetExecSeq() uint32 {
	if x != nil {
		return x.ExecSeq
	}
	return 0
}

func (x *WaitRequest) GetWaitTimeout() *durationpb.Duration {
	if x != nil {
		return x.WaitTimeout
	}
	return nil
}

// WaitResponse is the state of one exec; a claim.
type WaitResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// state is required.
	State ExecState `protobuf:"varint,1,opt,name=state,proto3,enum=numinance.ports.sandboxprovider.v1.ExecState" json:"state,omitempty"`
	// exit_code is the command's exit status for EXITED, and 0 for every other state.
	ExitCode int32 `protobuf:"varint,2,opt,name=exit_code,json=exitCode,proto3" json:"exit_code,omitempty"`
	// stdout_sha256 is the digest of exec/<exec_seq>/stdout once the exec has ended, set for EXITED, TIMED_OUT, LIMIT_EXCEEDED and KILLED and unset otherwise. The host recomputes it from the file.
	StdoutSha256 *v11.Digest `protobuf:"bytes,3,opt,name=stdout_sha256,json=stdoutSha256,proto3" json:"stdout_sha256,omitempty"`
	// stderr_sha256 is the same for exec/<exec_seq>/stderr.
	StderrSha256  *v11.Digest `protobuf:"bytes,4,opt,name=stderr_sha256,json=stderrSha256,proto3" json:"stderr_sha256,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *WaitResponse) Reset() {
	*x = WaitResponse{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *WaitResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WaitResponse) ProtoMessage() {}

func (x *WaitResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WaitResponse.ProtoReflect.Descriptor instead.
func (*WaitResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{8}
}

func (x *WaitResponse) GetState() ExecState {
	if x != nil {
		return x.State
	}
	return ExecState_EXEC_STATE_UNSPECIFIED
}

func (x *WaitResponse) GetExitCode() int32 {
	if x != nil {
		return x.ExitCode
	}
	return 0
}

func (x *WaitResponse) GetStdoutSha256() *v11.Digest {
	if x != nil {
		return x.StdoutSha256
	}
	return nil
}

func (x *WaitResponse) GetStderrSha256() *v11.Digest {
	if x != nil {
		return x.StderrSha256
	}
	return nil
}

// SnapshotRequest names the run to snapshot.
type SnapshotRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// run_id is ^run_[0-9a-f]{32}$.
	RunId         string `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SnapshotRequest) Reset() {
	*x = SnapshotRequest{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SnapshotRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SnapshotRequest) ProtoMessage() {}

func (x *SnapshotRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SnapshotRequest.ProtoReflect.Descriptor instead.
func (*SnapshotRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{9}
}

func (x *SnapshotRequest) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

// SnapshotResponse names the snapshot taken.
type SnapshotResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// snapshot is the content address of the snapshot in the provider's storage.
	Snapshot *v11.Digest `protobuf:"bytes,1,opt,name=snapshot,proto3" json:"snapshot,omitempty"`
	// worktree_digest is the tree digest of the worktree in the copy: sha256 of one LF-terminated line per entry, sorted bytewise by path, "d <n> <path>" for a directory, "f <n> <path> <hex>" or "x <n> <path> <hex>" for a file without or with the owner execute bit (no other mode bit is digested) with the hex sha256 of its content, "l <n> <path> <hex>" for a symlink with the hex sha256 of its target, where <n> is the path's length in bytes in decimal; paths relative and NFC; the root directory has no line; hex is lowercase; a symlink's target is its raw link bytes. Golden vector: conformance/sandboxprovider/v1/testdata/tree-digest/. A claim the host checks at restore.
	WorktreeDigest *v11.Digest `protobuf:"bytes,2,opt,name=worktree_digest,json=worktreeDigest,proto3" json:"worktree_digest,omitempty"`
	// artifacts_digest is the same for the artifact directory.
	ArtifactsDigest *v11.Digest `protobuf:"bytes,3,opt,name=artifacts_digest,json=artifactsDigest,proto3" json:"artifacts_digest,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *SnapshotResponse) Reset() {
	*x = SnapshotResponse{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SnapshotResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SnapshotResponse) ProtoMessage() {}

func (x *SnapshotResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SnapshotResponse.ProtoReflect.Descriptor instead.
func (*SnapshotResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{10}
}

func (x *SnapshotResponse) GetSnapshot() *v11.Digest {
	if x != nil {
		return x.Snapshot
	}
	return nil
}

func (x *SnapshotResponse) GetWorktreeDigest() *v11.Digest {
	if x != nil {
		return x.WorktreeDigest
	}
	return nil
}

func (x *SnapshotResponse) GetArtifactsDigest() *v11.Digest {
	if x != nil {
		return x.ArtifactsDigest
	}
	return nil
}

// DestroyRequest names the run to destroy.
type DestroyRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// run_id is ^run_[0-9a-f]{32}$.
	RunId string `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	// release_snapshots also releases every snapshot reference this run holds; the host sets it when the work unit ends or a later run of it holds its own snapshot.
	ReleaseSnapshots bool `protobuf:"varint,2,opt,name=release_snapshots,json=releaseSnapshots,proto3" json:"release_snapshots,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *DestroyRequest) Reset() {
	*x = DestroyRequest{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DestroyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DestroyRequest) ProtoMessage() {}

func (x *DestroyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DestroyRequest.ProtoReflect.Descriptor instead.
func (*DestroyRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{11}
}

func (x *DestroyRequest) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *DestroyRequest) GetReleaseSnapshots() bool {
	if x != nil {
		return x.ReleaseSnapshots
	}
	return false
}

// DestroyResponse is empty.
type DestroyResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DestroyResponse) Reset() {
	*x = DestroyResponse{}
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[12]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DestroyResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DestroyResponse) ProtoMessage() {}

func (x *DestroyResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes[12]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DestroyResponse.ProtoReflect.Descriptor instead.
func (*DestroyResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP(), []int{12}
}

var File_numinance_ports_sandboxprovider_v1_sandboxprovider_proto protoreflect.FileDescriptor

const file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDesc = "" +
	"\n" +
	"8numinance/ports/sandboxprovider/v1/sandboxprovider.proto\x12\"numinance.ports.sandboxprovider.v1\x1a\x1egoogle/protobuf/duration.proto\x1a\x1fgoogle/protobuf/timestamp.proto\x1a*numinance/annotations/v1/annotations.proto\x1a\"numinance/evidence/v1/record.proto\x1a\x1enuminance/types/v1/types.proto\"a\n" +
	"\rCreateRequest\x12P\n" +
	"\x04spec\x18\x01 \x01(\v2/.numinance.ports.sandboxprovider.v1.SandboxSpecB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04spec\"\x94\x01\n" +
	"\x0eCreateResponse\x12L\n" +
	"\aprofile\x18\x01 \x01(\x0e2%.numinance.evidence.v1.SandboxProfileB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aprofile\x124\n" +
	"\x0fruntime_version\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x0eruntimeVersion\"\xe2\x06\n" +
	"\vSandboxSpec\x12(\n" +
	"\tchange_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bchangeId\x12-\n" +
	"\fwork_unit_id\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"workUnitId\x12\"\n" +
	"\x06run_id\x18\x03 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05runId\x12L\n" +
	"\aprofile\x18\x04 \x01(\x0e2%.numinance.evidence.v1.SandboxProfileB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aprofile\x12J\n" +
	"\fimage_digest\x18\x05 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vimageDigest\x120\n" +
	"\rworktree_path\x18\x06 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fworktreePath\x122\n" +
	"\x0eartifacts_path\x18\a \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rartifactsPath\x120\n" +
	"\rallowed_paths\x18\b \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fallowedPaths\x12.\n" +
	"\fmasked_paths\x18\t \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vmaskedPaths\x12.\n" +
	"\fegress_proxy\x18\n" +
	" \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vegressProxy\x12V\n" +
	"\x06limits\x18\v \x01(\v21.numinance.ports.sandboxprovider.v1.SandboxLimitsB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06limits\x12C\n" +
	"\bdeadline\x18\f \x01(\v2\x1a.google.protobuf.TimestampB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bdeadline\x12-\n" +
	"\vtraceparent\x18\r \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vtraceparent\x12J\n" +
	"\frestore_from\x18\x0e \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vrestoreFrom\x12,\n" +
	"\voutput_path\x18\x0f \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"outputPath\"\xc0\x01\n" +
	"\rSandboxLimits\x122\n" +
	"\x0ecpu_millicores\x18\x01 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\rcpuMillicores\x12.\n" +
	"\fmemory_bytes\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vmemoryBytes\x12\x1f\n" +
	"\x04pids\x18\x03 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04pids\x12*\n" +
	"\n" +
	"disk_bytes\x18\x04 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tdiskBytes\"\xb5\x02\n" +
	"\vExecRequest\x12\"\n" +
	"\x06run_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05runId\x12&\n" +
	"\bexec_seq\x18\x02 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aexecSeq\x12\x1f\n" +
	"\x04argv\x18\x03 \x03(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04argv\x12,\n" +
	"\vworking_dir\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\n" +
	"workingDir\x12I\n" +
	"\x03env\x18\x05 \x03(\v2*.numinance.ports.sandboxprovider.v1.EnvVarB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x03env\x12@\n" +
	"\atimeout\x18\x06 \x01(\v2\x19.google.protobuf.DurationB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\atimeout\"L\n" +
	"\x06EnvVar\x12\x1f\n" +
	"\x04name\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04name\x12!\n" +
	"\x05value\x18\x02 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05value\"\x0e\n" +
	"\fExecResponse\"\xa4\x01\n" +
	"\vWaitRequest\x12\"\n" +
	"\x06run_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05runId\x12&\n" +
	"\bexec_seq\x18\x02 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aexecSeq\x12I\n" +
	"\fwait_timeout\x18\x03 \x01(\v2\x19.google.protobuf.DurationB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vwaitTimeout\"\xa6\x02\n" +
	"\fWaitResponse\x12P\n" +
	"\x05state\x18\x01 \x01(\x0e2-.numinance.ports.sandboxprovider.v1.ExecStateB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05state\x12(\n" +
	"\texit_code\x18\x02 \x01(\x05B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bexitCode\x12L\n" +
	"\rstdout_sha256\x18\x03 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fstdoutSha256\x12L\n" +
	"\rstderr_sha256\x18\x04 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fstderrSha256\"5\n" +
	"\x0fSnapshotRequest\x12\"\n" +
	"\x06run_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05runId\"\xfd\x01\n" +
	"\x10SnapshotResponse\x12C\n" +
	"\bsnapshot\x18\x01 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bsnapshot\x12P\n" +
	"\x0fworktree_digest\x18\x02 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x0eworktreeDigest\x12R\n" +
	"\x10artifacts_digest\x18\x03 \x01(\v2\x1a.numinance.types.v1.DigestB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x0fartifactsDigest\"n\n" +
	"\x0eDestroyRequest\x12\"\n" +
	"\x06run_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05runId\x128\n" +
	"\x11release_snapshots\x18\x02 \x01(\bB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x10releaseSnapshots\"\x11\n" +
	"\x0fDestroyResponse*\xc3\x01\n" +
	"\tExecState\x12\x1a\n" +
	"\x16EXEC_STATE_UNSPECIFIED\x10\x00\x12\x16\n" +
	"\x12EXEC_STATE_RUNNING\x10\x01\x12\x15\n" +
	"\x11EXEC_STATE_EXITED\x10\x02\x12\x18\n" +
	"\x14EXEC_STATE_TIMED_OUT\x10\x03\x12\x1d\n" +
	"\x19EXEC_STATE_LIMIT_EXCEEDED\x10\x04\x12\x15\n" +
	"\x11EXEC_STATE_KILLED\x10\x05\x12\x1b\n" +
	"\x17EXEC_STATE_START_FAILED\x10\x062\xf7\a\n" +
	"\x16SandboxProviderService\x12\xee\x01\n" +
	"\x06Create\x121.numinance.ports.sandboxprovider.v1.CreateRequest\x1a2.numinance.ports.sandboxprovider.v1.CreateResponse\"}\x8a\xb5\x18y\b\x02\x10\x01\x18\x01 \x01*\x1eSandboxProviderService.DestroyB\x051.0.0JHCreates the isolated sandbox of one run from a host-built specification.\x12\xa8\x01\n" +
	"\x04Exec\x12/.numinance.ports.sandboxprovider.v1.ExecRequest\x1a0.numinance.ports.sandboxprovider.v1.ExecResponse\"=\x8a\xb5\x189\b\x03\x10\x02\x18\x01B\x051.0.0J*Starts one command inside a run's sandbox.\x12\xb2\x01\n" +
	"\x04Wait\x12/.numinance.ports.sandboxprovider.v1.WaitRequest\x1a0.numinance.ports.sandboxprovider.v1.WaitResponse\"G\x8a\xb5\x18C\b\x01\x10\x01\x18\x01B\x051.0.0J4Returns the state of one command in a run's sandbox.\x12\xcd\x01\n" +
	"\bSnapshot\x123.numinance.ports.sandboxprovider.v1.SnapshotRequest\x1a4.numinance.ports.sandboxprovider.v1.SnapshotResponse\"V\x8a\xb5\x18R\b\x02\x10\x01B\x051.0.0JETakes a content-addressed snapshot of a run's worktree and artifacts.\x12\xbb\x01\n" +
	"\aDestroy\x122.numinance.ports.sandboxprovider.v1.DestroyRequest\x1a3.numinance.ports.sandboxprovider.v1.DestroyResponse\"G\x8a\xb5\x18C\b\x02\x10\x01\x18\x01B\x051.0.0J4Destroys a run's sandbox and releases its resources.B\xce\x02\n" +
	"&com.numinance.ports.sandboxprovider.v1B\x14SandboxproviderProtoP\x01Zcgithub.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/sandboxprovider/v1;sandboxproviderv1\xa2\x02\x03NPS\xaa\x02\"Numinance.Ports.Sandboxprovider.V1\xca\x02\"Numinance\\Ports\\Sandboxprovider\\V1\xe2\x02.Numinance\\Ports\\Sandboxprovider\\V1\\GPBMetadata\xea\x02%Numinance::Ports::Sandboxprovider::V1b\x06proto3"

var (
	file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescOnce sync.Once
	file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescData []byte
)

func file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescGZIP() []byte {
	file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescOnce.Do(func() {
		file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDesc), len(file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDesc)))
	})
	return file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDescData
}

var file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes = make([]protoimpl.MessageInfo, 13)
var file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_goTypes = []any{
	(ExecState)(0),                // 0: numinance.ports.sandboxprovider.v1.ExecState
	(*CreateRequest)(nil),         // 1: numinance.ports.sandboxprovider.v1.CreateRequest
	(*CreateResponse)(nil),        // 2: numinance.ports.sandboxprovider.v1.CreateResponse
	(*SandboxSpec)(nil),           // 3: numinance.ports.sandboxprovider.v1.SandboxSpec
	(*SandboxLimits)(nil),         // 4: numinance.ports.sandboxprovider.v1.SandboxLimits
	(*ExecRequest)(nil),           // 5: numinance.ports.sandboxprovider.v1.ExecRequest
	(*EnvVar)(nil),                // 6: numinance.ports.sandboxprovider.v1.EnvVar
	(*ExecResponse)(nil),          // 7: numinance.ports.sandboxprovider.v1.ExecResponse
	(*WaitRequest)(nil),           // 8: numinance.ports.sandboxprovider.v1.WaitRequest
	(*WaitResponse)(nil),          // 9: numinance.ports.sandboxprovider.v1.WaitResponse
	(*SnapshotRequest)(nil),       // 10: numinance.ports.sandboxprovider.v1.SnapshotRequest
	(*SnapshotResponse)(nil),      // 11: numinance.ports.sandboxprovider.v1.SnapshotResponse
	(*DestroyRequest)(nil),        // 12: numinance.ports.sandboxprovider.v1.DestroyRequest
	(*DestroyResponse)(nil),       // 13: numinance.ports.sandboxprovider.v1.DestroyResponse
	(v1.SandboxProfile)(0),        // 14: numinance.evidence.v1.SandboxProfile
	(*v11.Digest)(nil),            // 15: numinance.types.v1.Digest
	(*timestamppb.Timestamp)(nil), // 16: google.protobuf.Timestamp
	(*durationpb.Duration)(nil),   // 17: google.protobuf.Duration
}
var file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_depIdxs = []int32{
	3,  // 0: numinance.ports.sandboxprovider.v1.CreateRequest.spec:type_name -> numinance.ports.sandboxprovider.v1.SandboxSpec
	14, // 1: numinance.ports.sandboxprovider.v1.CreateResponse.profile:type_name -> numinance.evidence.v1.SandboxProfile
	14, // 2: numinance.ports.sandboxprovider.v1.SandboxSpec.profile:type_name -> numinance.evidence.v1.SandboxProfile
	15, // 3: numinance.ports.sandboxprovider.v1.SandboxSpec.image_digest:type_name -> numinance.types.v1.Digest
	4,  // 4: numinance.ports.sandboxprovider.v1.SandboxSpec.limits:type_name -> numinance.ports.sandboxprovider.v1.SandboxLimits
	16, // 5: numinance.ports.sandboxprovider.v1.SandboxSpec.deadline:type_name -> google.protobuf.Timestamp
	15, // 6: numinance.ports.sandboxprovider.v1.SandboxSpec.restore_from:type_name -> numinance.types.v1.Digest
	6,  // 7: numinance.ports.sandboxprovider.v1.ExecRequest.env:type_name -> numinance.ports.sandboxprovider.v1.EnvVar
	17, // 8: numinance.ports.sandboxprovider.v1.ExecRequest.timeout:type_name -> google.protobuf.Duration
	17, // 9: numinance.ports.sandboxprovider.v1.WaitRequest.wait_timeout:type_name -> google.protobuf.Duration
	0,  // 10: numinance.ports.sandboxprovider.v1.WaitResponse.state:type_name -> numinance.ports.sandboxprovider.v1.ExecState
	15, // 11: numinance.ports.sandboxprovider.v1.WaitResponse.stdout_sha256:type_name -> numinance.types.v1.Digest
	15, // 12: numinance.ports.sandboxprovider.v1.WaitResponse.stderr_sha256:type_name -> numinance.types.v1.Digest
	15, // 13: numinance.ports.sandboxprovider.v1.SnapshotResponse.snapshot:type_name -> numinance.types.v1.Digest
	15, // 14: numinance.ports.sandboxprovider.v1.SnapshotResponse.worktree_digest:type_name -> numinance.types.v1.Digest
	15, // 15: numinance.ports.sandboxprovider.v1.SnapshotResponse.artifacts_digest:type_name -> numinance.types.v1.Digest
	1,  // 16: numinance.ports.sandboxprovider.v1.SandboxProviderService.Create:input_type -> numinance.ports.sandboxprovider.v1.CreateRequest
	5,  // 17: numinance.ports.sandboxprovider.v1.SandboxProviderService.Exec:input_type -> numinance.ports.sandboxprovider.v1.ExecRequest
	8,  // 18: numinance.ports.sandboxprovider.v1.SandboxProviderService.Wait:input_type -> numinance.ports.sandboxprovider.v1.WaitRequest
	10, // 19: numinance.ports.sandboxprovider.v1.SandboxProviderService.Snapshot:input_type -> numinance.ports.sandboxprovider.v1.SnapshotRequest
	12, // 20: numinance.ports.sandboxprovider.v1.SandboxProviderService.Destroy:input_type -> numinance.ports.sandboxprovider.v1.DestroyRequest
	2,  // 21: numinance.ports.sandboxprovider.v1.SandboxProviderService.Create:output_type -> numinance.ports.sandboxprovider.v1.CreateResponse
	7,  // 22: numinance.ports.sandboxprovider.v1.SandboxProviderService.Exec:output_type -> numinance.ports.sandboxprovider.v1.ExecResponse
	9,  // 23: numinance.ports.sandboxprovider.v1.SandboxProviderService.Wait:output_type -> numinance.ports.sandboxprovider.v1.WaitResponse
	11, // 24: numinance.ports.sandboxprovider.v1.SandboxProviderService.Snapshot:output_type -> numinance.ports.sandboxprovider.v1.SnapshotResponse
	13, // 25: numinance.ports.sandboxprovider.v1.SandboxProviderService.Destroy:output_type -> numinance.ports.sandboxprovider.v1.DestroyResponse
	21, // [21:26] is the sub-list for method output_type
	16, // [16:21] is the sub-list for method input_type
	16, // [16:16] is the sub-list for extension type_name
	16, // [16:16] is the sub-list for extension extendee
	0,  // [0:16] is the sub-list for field type_name
}

func init() { file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_init() }
func file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_init() {
	if File_numinance_ports_sandboxprovider_v1_sandboxprovider_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDesc), len(file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   13,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_goTypes,
		DependencyIndexes: file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_depIdxs,
		EnumInfos:         file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_enumTypes,
		MessageInfos:      file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_msgTypes,
	}.Build()
	File_numinance_ports_sandboxprovider_v1_sandboxprovider_proto = out.File
	file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_goTypes = nil
	file_numinance_ports_sandboxprovider_v1_sandboxprovider_proto_depIdxs = nil
}
