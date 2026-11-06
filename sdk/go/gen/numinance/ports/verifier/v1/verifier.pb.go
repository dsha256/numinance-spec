// The verifier port (ADR 0009): one verifier runs its checks on one candidate and reports findings.
// Callers: only the verification context calls this port, for a run whose ActionContext.actor_kind is reviewer-agent, whose actor_principal and run_id each differ from those of every author run of the Change, not only the latest; the host refuses every other actor kind and a call that breaks either inequality. The verifier runs in its own sandbox with read-only access to the candidate, no write scope and no credential capability. The port is never a model tool, and no tool_groups or scopes are declared.
// Sandbox: Run executes candidate code, such as agent-written tests, so the verifier's sandbox has default-deny egress: no egress destination and no credential intent is granted, IMDS and link-local ranges are denied, and the only connection is the one to the host. The host refuses to dispatch Run to a verifier whose manifest or grant names any egress destination or credential intent, and runs it at least at the sandbox floor's minimum profile, recorded in evidence. What the checks need comes with the candidate or the verifier's image. Candidate code: any check that executes candidate code (tests, build scripts, hooks) runs in a nested sandbox under a separate uid and its own namespaces, with no access to the plugin or host channel or any inherited descriptor, no write access to the verifier's binaries, working files or reports, and no ptrace or signals to the verifier's processes; its output is data for that check only and never changes another check's report, the evidence the verifier emits or the RunResponse. The verifier reads that output as untrusted input: size-limited, where output over the limit ends the call FAILED and is never truncated to continue, parsed without entity or include expansion, and a parse failure ends the call FAILED. Where the nested sandbox cannot be built, as under the process profile, the call ends UNAVAILABLE and candidate code never runs unnested. LOW rests on these rules.
// Configuration: config_ref is the only configuration the checks honour. The host takes it from the LanguageToolchain or ArchitecturePack binding and refuses one that names an artifact of the candidate or of the author run. The provider ignores or overrides configuration and suppression directives inside the candidate (tool settings files, inline suppressions, test hooks); where a tool cannot ignore one, the provider reports each such directive as a finding, so an author cannot switch a check off from the candidate.
// Untrusted answers: every finding is a provider claim and untrusted data. The verification context checks the bounds below, stores the findings as an artifact referenced by digest, and the gate reads the Verdict, never raw findings. A message, path or rule_id is rendered delimited and labelled as untrusted wherever it reaches a prompt, a ticket or a PR, never as instructions.
// Payloads: candidate_ref and config_ref are references the provider resolves through HostService.ResolveReference; no request or response carries candidate content (a platform invariant: payloads carry references, never content).
// Operation: Run is one OPERATION call on PluginService.Invoke: RUNNING messages carry status and diagnostics only, and the terminal message carries RunResponse when SUCCEEDED. There are no start, get and cancel verbs because the host holds the stream: at the deadline or on a cancel the host ends the call and records the run as failed. A lost stream is not reattached; the verification context starts a new Run, which is safe because Run changes nothing outside its sandbox.
// Status codes: INVALID_ARGUMENT for a request that breaks a bound, NOT_FOUND for a reference the host cannot resolve, UNAVAILABLE when the checks cannot start, including a config naming a check the provider cannot run and a nested sandbox that cannot be built. A check that crashes, times out or cannot complete ends the call FAILED with no findings, never SUCCEEDED with a partial list, and the layer is inconclusive, never passed. Findings are results, not failures: a run that finds violations SUCCEEDS.
// Evidence: Run declares one record type, VerifierReport (fixed with the verification context before the first spec tag): a provider emits exactly one per call through HostService.EmitEvidence, a compact receipt of what the tool said, never the findings. Its payload is a google.protobuf.Struct of at most 16 KiB with the keys check (the check the config named), exit_code, output_bytes, output_sha256 (the digest of the raw tool output, so the evidence names what the tool said without storing it), findings (the count after merging by fingerprint) and replayed (true when a recorded output stood in for the tool). A call that ends SUCCEEDED without it is PARTIAL, and the verification context treats the layer as inconclusive. The design's evidence_refs are the ActionResult.evidence_ids of the terminal message (HostService.EmitEvidence), so a provider cannot name an artifact it did not emit.
// Bounds: every string is at most 1024 bytes unless stated; output still out of bounds after the normalisation stated on a field is rejected, never truncated by the host; enums are closed, and zero and unknown numbers are rejected; a field unknown to the host's descriptor copy is rejected (plugin/v1 Invoke strict decoding), so a provider sends a field added in a later minor only when HandshakeRequest.spec_version is at least that minor or behind a negotiated capability. Printable means Unicode categories L, M, N, P, S and U+0020. A RunResponse is at most 1024 findings of at most about 2.6 KiB each, so it fits one Invoke message of at most 4 MiB. The host rejects a RunResponse that breaks a rule of this file and records the run as failed.
// Every element is since 1.0.0.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.12
// 	protoc        (unknown)
// source: numinance/ports/verifier/v1/verifier.proto

package verifierv1

import (
	_ "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
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

// RunRequest names the candidate and the verifier configuration.
type RunRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The candidate tree to check, read-only; required.
	CandidateRef *v1.Reference `protobuf:"bytes,1,opt,name=candidate_ref,json=candidateRef,proto3" json:"candidate_ref,omitempty"`
	// The verifier configuration the LanguageToolchain or ArchitecturePack binding resolved; required, and the only configuration the checks honour.
	ConfigRef     *v1.Reference `protobuf:"bytes,2,opt,name=config_ref,json=configRef,proto3" json:"config_ref,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RunRequest) Reset() {
	*x = RunRequest{}
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RunRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunRequest) ProtoMessage() {}

func (x *RunRequest) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunRequest.ProtoReflect.Descriptor instead.
func (*RunRequest) Descriptor() ([]byte, []int) {
	return file_numinance_ports_verifier_v1_verifier_proto_rawDescGZIP(), []int{0}
}

func (x *RunRequest) GetCandidateRef() *v1.Reference {
	if x != nil {
		return x.CandidateRef
	}
	return nil
}

func (x *RunRequest) GetConfigRef() *v1.Reference {
	if x != nil {
		return x.ConfigRef
	}
	return nil
}

// RunResponse holds the findings: the findings of one run.
type RunResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Unique by fingerprint and ordered by severity, most severe first. Findings of one fingerprint are merged into one item before counting. The run's total is the sum of the summary counts; items holds every finding when the total is at most 1024, else the 1024 most severe, so len(items) is min(total, 1024).
	Items []*Finding `protobuf:"bytes,1,rep,name=items,proto3" json:"items,omitempty"`
	// Counts of every finding of the run after merging by fingerprint; required, also when there are none.
	Summary       *FindingsSummary `protobuf:"bytes,2,opt,name=summary,proto3" json:"summary,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RunResponse) Reset() {
	*x = RunResponse{}
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RunResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunResponse) ProtoMessage() {}

func (x *RunResponse) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunResponse.ProtoReflect.Descriptor instead.
func (*RunResponse) Descriptor() ([]byte, []int) {
	return file_numinance_ports_verifier_v1_verifier_proto_rawDescGZIP(), []int{1}
}

func (x *RunResponse) GetItems() []*Finding {
	if x != nil {
		return x.Items
	}
	return nil
}

func (x *RunResponse) GetSummary() *FindingsSummary {
	if x != nil {
		return x.Summary
	}
	return nil
}

// Finding is one violation a check reported.
type Finding struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the rule that produced the finding, matching ^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$; an ArchitecturePack binds a convention to it.
	RuleId string `protobuf:"bytes,1,opt,name=rule_id,json=ruleId,proto3" json:"rule_id,omitempty"`
	// Required: the severity of the finding; what it blocks is policy.
	Severity v1.FindingSeverity `protobuf:"varint,2,opt,name=severity,proto3,enum=numinance.types.v1.FindingSeverity" json:"severity,omitempty"`
	// Where in the candidate the finding is; unset for a finding about the candidate as a whole.
	Location *Location `protobuf:"bytes,3,opt,name=location,proto3" json:"location,omitempty"`
	// Required: what the check reported, 1 to 1024 bytes of printable characters, U+0009 and U+000A; untrusted. The provider replaces any other character with U+FFFD and shortens a longer report at a character boundary; the full tool output belongs in evidence, not here.
	Message string `protobuf:"bytes,4,opt,name=message,proto3" json:"message,omitempty"`
	// Required: a stable identity of the finding across runs of the same check on related candidates, 1 to 256 bytes of printable ASCII without U+0020; the verification context uses it to compare runs, never to suppress a finding.
	Fingerprint   string `protobuf:"bytes,5,opt,name=fingerprint,proto3" json:"fingerprint,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Finding) Reset() {
	*x = Finding{}
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Finding) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Finding) ProtoMessage() {}

func (x *Finding) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Finding.ProtoReflect.Descriptor instead.
func (*Finding) Descriptor() ([]byte, []int) {
	return file_numinance_ports_verifier_v1_verifier_proto_rawDescGZIP(), []int{2}
}

func (x *Finding) GetRuleId() string {
	if x != nil {
		return x.RuleId
	}
	return ""
}

func (x *Finding) GetSeverity() v1.FindingSeverity {
	if x != nil {
		return x.Severity
	}
	return v1.FindingSeverity(0)
}

func (x *Finding) GetLocation() *Location {
	if x != nil {
		return x.Location
	}
	return nil
}

func (x *Finding) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *Finding) GetFingerprint() string {
	if x != nil {
		return x.Fingerprint
	}
	return ""
}

// Location is a position in the candidate tree.
type Location struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: a path relative to the candidate root, '/'-separated, printable, with no leading '/', no '\' and no empty, '.' or '..' segment.
	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	// First line, 1-based; 0 when the finding is about the whole file.
	StartLine uint32 `protobuf:"varint,2,opt,name=start_line,json=startLine,proto3" json:"start_line,omitempty"`
	// Last line, at least start_line; 0 exactly when start_line is 0.
	EndLine       uint32 `protobuf:"varint,3,opt,name=end_line,json=endLine,proto3" json:"end_line,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Location) Reset() {
	*x = Location{}
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Location) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Location) ProtoMessage() {}

func (x *Location) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Location.ProtoReflect.Descriptor instead.
func (*Location) Descriptor() ([]byte, []int) {
	return file_numinance_ports_verifier_v1_verifier_proto_rawDescGZIP(), []int{3}
}

func (x *Location) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *Location) GetStartLine() uint32 {
	if x != nil {
		return x.StartLine
	}
	return 0
}

func (x *Location) GetEndLine() uint32 {
	if x != nil {
		return x.EndLine
	}
	return 0
}

// FindingsSummary counts the findings of one run by severity.
type FindingsSummary struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// One entry per severity with at least one finding, unique by severity; a severity with none has no entry. Each count is at least the number of listed items of that severity, and equals it for every severity above the least severe one listed; the number of items equals min(sum of counts, 1024); the host rejects a response that breaks any of these three rules.
	Counts        []*SeverityCount `protobuf:"bytes,1,rep,name=counts,proto3" json:"counts,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FindingsSummary) Reset() {
	*x = FindingsSummary{}
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FindingsSummary) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FindingsSummary) ProtoMessage() {}

func (x *FindingsSummary) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FindingsSummary.ProtoReflect.Descriptor instead.
func (*FindingsSummary) Descriptor() ([]byte, []int) {
	return file_numinance_ports_verifier_v1_verifier_proto_rawDescGZIP(), []int{4}
}

func (x *FindingsSummary) GetCounts() []*SeverityCount {
	if x != nil {
		return x.Counts
	}
	return nil
}

// SeverityCount is the number of findings of one severity.
type SeverityCount struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required: the severity counted.
	Severity v1.FindingSeverity `protobuf:"varint,1,opt,name=severity,proto3,enum=numinance.types.v1.FindingSeverity" json:"severity,omitempty"`
	// Number of findings of that severity, at least 1 and at most 2^53-1.
	Count         uint64 `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SeverityCount) Reset() {
	*x = SeverityCount{}
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SeverityCount) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SeverityCount) ProtoMessage() {}

func (x *SeverityCount) ProtoReflect() protoreflect.Message {
	mi := &file_numinance_ports_verifier_v1_verifier_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SeverityCount.ProtoReflect.Descriptor instead.
func (*SeverityCount) Descriptor() ([]byte, []int) {
	return file_numinance_ports_verifier_v1_verifier_proto_rawDescGZIP(), []int{5}
}

func (x *SeverityCount) GetSeverity() v1.FindingSeverity {
	if x != nil {
		return x.Severity
	}
	return v1.FindingSeverity(0)
}

func (x *SeverityCount) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

var File_numinance_ports_verifier_v1_verifier_proto protoreflect.FileDescriptor

const file_numinance_ports_verifier_v1_verifier_proto_rawDesc = "" +
	"\n" +
	"*numinance/ports/verifier/v1/verifier.proto\x12\x1bnuminance.ports.verifier.v1\x1a*numinance/annotations/v1/annotations.proto\x1a\x1enuminance/types/v1/types.proto\"\xa8\x01\n" +
	"\n" +
	"RunRequest\x12O\n" +
	"\rcandidate_ref\x18\x01 \x01(\v2\x1d.numinance.types.v1.ReferenceB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\fcandidateRef\x12I\n" +
	"\n" +
	"config_ref\x18\x02 \x01(\v2\x1d.numinance.types.v1.ReferenceB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tconfigRef\"\xab\x01\n" +
	"\vRunResponse\x12G\n" +
	"\x05items\x18\x01 \x03(\v2$.numinance.ports.verifier.v1.FindingB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05items\x12S\n" +
	"\asummary\x18\x02 \x01(\v2,.numinance.ports.verifier.v1.FindingsSummaryB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\asummary\"\xa3\x02\n" +
	"\aFinding\x12$\n" +
	"\arule_id\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06ruleId\x12L\n" +
	"\bseverity\x18\x02 \x01(\x0e2#.numinance.types.v1.FindingSeverityB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bseverity\x12N\n" +
	"\blocation\x18\x03 \x01(\v2%.numinance.ports.verifier.v1.LocationB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\blocation\x12%\n" +
	"\amessage\x18\x04 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\amessage\x12-\n" +
	"\vfingerprint\x18\x05 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\vfingerprint\"\x7f\n" +
	"\bLocation\x12\x1f\n" +
	"\x04path\x18\x01 \x01(\tB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x04path\x12*\n" +
	"\n" +
	"start_line\x18\x02 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\tstartLine\x12&\n" +
	"\bend_line\x18\x03 \x01(\rB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\aendLine\"b\n" +
	"\x0fFindingsSummary\x12O\n" +
	"\x06counts\x18\x01 \x03(\v2*.numinance.ports.verifier.v1.SeverityCountB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x06counts\"\x80\x01\n" +
	"\rSeverityCount\x12L\n" +
	"\bseverity\x18\x01 \x01(\x0e2#.numinance.types.v1.FindingSeverityB\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\bseverity\x12!\n" +
	"\x05count\x18\x02 \x01(\x04B\v\x92\xb5\x18\a\n" +
	"\x051.0.0R\x05count2\xd4\x01\n" +
	"\x0fVerifierService\x12\xc0\x01\n" +
	"\x03Run\x12'.numinance.ports.verifier.v1.RunRequest\x1a(.numinance.ports.verifier.v1.RunResponse\"f\x8a\xb5\x18b\b\x02\x10\x02\x18\x012\x0eVerifierReportB\x051.0.0JCRuns one verifier's checks on a candidate and returns its findings.B\x96\x02\n" +
	"\x1fcom.numinance.ports.verifier.v1B\rVerifierProtoP\x01ZUgithub.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1;verifierv1\xa2\x02\x03NPV\xaa\x02\x1bNuminance.Ports.Verifier.V1\xca\x02\x1bNuminance\\Ports\\Verifier\\V1\xe2\x02'Numinance\\Ports\\Verifier\\V1\\GPBMetadata\xea\x02\x1eNuminance::Ports::Verifier::V1b\x06proto3"

var (
	file_numinance_ports_verifier_v1_verifier_proto_rawDescOnce sync.Once
	file_numinance_ports_verifier_v1_verifier_proto_rawDescData []byte
)

func file_numinance_ports_verifier_v1_verifier_proto_rawDescGZIP() []byte {
	file_numinance_ports_verifier_v1_verifier_proto_rawDescOnce.Do(func() {
		file_numinance_ports_verifier_v1_verifier_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_numinance_ports_verifier_v1_verifier_proto_rawDesc), len(file_numinance_ports_verifier_v1_verifier_proto_rawDesc)))
	})
	return file_numinance_ports_verifier_v1_verifier_proto_rawDescData
}

var file_numinance_ports_verifier_v1_verifier_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_numinance_ports_verifier_v1_verifier_proto_goTypes = []any{
	(*RunRequest)(nil),      // 0: numinance.ports.verifier.v1.RunRequest
	(*RunResponse)(nil),     // 1: numinance.ports.verifier.v1.RunResponse
	(*Finding)(nil),         // 2: numinance.ports.verifier.v1.Finding
	(*Location)(nil),        // 3: numinance.ports.verifier.v1.Location
	(*FindingsSummary)(nil), // 4: numinance.ports.verifier.v1.FindingsSummary
	(*SeverityCount)(nil),   // 5: numinance.ports.verifier.v1.SeverityCount
	(*v1.Reference)(nil),    // 6: numinance.types.v1.Reference
	(v1.FindingSeverity)(0), // 7: numinance.types.v1.FindingSeverity
}
var file_numinance_ports_verifier_v1_verifier_proto_depIdxs = []int32{
	6, // 0: numinance.ports.verifier.v1.RunRequest.candidate_ref:type_name -> numinance.types.v1.Reference
	6, // 1: numinance.ports.verifier.v1.RunRequest.config_ref:type_name -> numinance.types.v1.Reference
	2, // 2: numinance.ports.verifier.v1.RunResponse.items:type_name -> numinance.ports.verifier.v1.Finding
	4, // 3: numinance.ports.verifier.v1.RunResponse.summary:type_name -> numinance.ports.verifier.v1.FindingsSummary
	7, // 4: numinance.ports.verifier.v1.Finding.severity:type_name -> numinance.types.v1.FindingSeverity
	3, // 5: numinance.ports.verifier.v1.Finding.location:type_name -> numinance.ports.verifier.v1.Location
	5, // 6: numinance.ports.verifier.v1.FindingsSummary.counts:type_name -> numinance.ports.verifier.v1.SeverityCount
	7, // 7: numinance.ports.verifier.v1.SeverityCount.severity:type_name -> numinance.types.v1.FindingSeverity
	0, // 8: numinance.ports.verifier.v1.VerifierService.Run:input_type -> numinance.ports.verifier.v1.RunRequest
	1, // 9: numinance.ports.verifier.v1.VerifierService.Run:output_type -> numinance.ports.verifier.v1.RunResponse
	9, // [9:10] is the sub-list for method output_type
	8, // [8:9] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_numinance_ports_verifier_v1_verifier_proto_init() }
func file_numinance_ports_verifier_v1_verifier_proto_init() {
	if File_numinance_ports_verifier_v1_verifier_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_numinance_ports_verifier_v1_verifier_proto_rawDesc), len(file_numinance_ports_verifier_v1_verifier_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_numinance_ports_verifier_v1_verifier_proto_goTypes,
		DependencyIndexes: file_numinance_ports_verifier_v1_verifier_proto_depIdxs,
		MessageInfos:      file_numinance_ports_verifier_v1_verifier_proto_msgTypes,
	}.Build()
	File_numinance_ports_verifier_v1_verifier_proto = out.File
	file_numinance_ports_verifier_v1_verifier_proto_goTypes = nil
	file_numinance_ports_verifier_v1_verifier_proto_depIdxs = nil
}
