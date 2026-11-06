// Package annotationlint checks the governance annotations of Numinance protos
// (ADR 0012): every RPC carries a complete (numinance.annotations.v1.action) and
// every field a strict x.y.z (numinance.annotations.v1.field).since. Buf's own
// rules cannot require a custom option, so `just lint` runs this check over the
// `buf build` output.
package annotationlint

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
)

// Rule IDs follow buf's UPPER_SNAKE convention.
const (
	// RuleActionRequired reports an RPC without (numinance.annotations.v1.action).
	RuleActionRequired = "ACTION_ANNOTATION_REQUIRED"
	// RuleActionRisk reports an action whose risk is RISK_CLASS_UNSPECIFIED.
	RuleActionRisk = "ACTION_RISK_REQUIRED"
	// RuleActionExec reports an action whose exec is EXEC_MODE_UNSPECIFIED.
	RuleActionExec = "ACTION_EXEC_REQUIRED"
	// RuleActionSince reports an action whose since is not a strict x.y.z version.
	RuleActionSince = "ACTION_SINCE_REQUIRED"
	// RuleActionLLMDescription reports an action with an empty llm_description.
	RuleActionLLMDescription = "ACTION_LLM_DESCRIPTION_REQUIRED"
	// RuleActionScope reports an action.scopes entry that is not a scope name of authority.Grant.Validate
	// (dotted lower-case, at most 256 bytes) or that repeats another entry.
	RuleActionScope = "ACTION_SCOPE_INVALID"
	// RuleFieldSince reports a field without a strict x.y.z field since.
	RuleFieldSince = "FIELD_SINCE_REQUIRED"
	// RuleHostOnly reports host_only on anything but a Reference field, or a
	// wrapper message that holds one, of a message that an RPC input reaches in a
	// numinance.ports.* file.
	RuleHostOnly = "HOST_ONLY_INVALID"
	// RuleResourceName reports resource_name on anything but a string or repeated
	// string field of a message that an RPC input reaches in a numinance.ports.* file.
	RuleResourceName = "RESOURCE_NAME_INVALID"
)

// referenceType is the only leaf type host_only may mark
// (rfcs/0001-host-only-field-annotation.md).
const referenceType = ".numinance.types.v1.Reference"

// annotationsFile declares the options. Only its two extension declarations are
// exempt from RuleFieldSince: the file states every element is since 1.0.0 and
// changes only through an RFC. Its message fields and sibling files are checked.
const annotationsFile = "numinance/annotations/v1/annotations.proto"

// Field numbers of descriptor.proto used to build source code info paths.
const (
	fileMessageTag      = 4 // FileDescriptorProto.message_type
	fileServiceTag      = 6 // FileDescriptorProto.service
	fileExtensionTag    = 7 // FileDescriptorProto.extension
	messageFieldTag     = 2 // DescriptorProto.field
	messageNestedTag    = 3 // DescriptorProto.nested_type
	messageExtensionTag = 6 // DescriptorProto.extension
	serviceMethodTag    = 2 // ServiceDescriptorProto.method
)

// Violation is one lint finding.
type Violation struct {
	File         string // proto file name as recorded in the descriptor set
	Line, Column int    // 1-based; 0 when the set has no source code info for the element
	Rule         string // one of the Rule* IDs
	Message      string // names the element and what is missing; never echoes option values
}

// String formats v like buf's text output: file:line:column:RULE message.
func (v Violation) String() string {
	return fmt.Sprintf("%s:%d:%d:%s %s", v.File, v.Line, v.Column, v.Rule, v.Message)
}

// Check returns the violations in every file of set, ordered by file, line,
// column and rule. Files are checked as given; build the set without imports
// (buf build --exclude-imports) to lint only the module's own files.
func Check(set *descriptorpb.FileDescriptorSet) []Violation {
	var out []Violation
	messages := indexMessages(set)
	inputs := reachableFromInputs(set, messages)
	for _, f := range set.GetFile() {
		c := fileChecker{file: f, lines: sourceLines(f), messages: messages, inputs: inputs}
		c.run()
		out = append(out, c.out...)
	}
	return sortViolations(out)
}

func sortViolations(out []Violation) []Violation {
	slices.SortStableFunc(out, func(a, b Violation) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column), cmp.Compare(a.Rule, b.Rule))
	})
	return out
}

// position is a 1-based line and column.
type position struct{ line, column int }

// sourceLines maps a source code info path, keyed by fmt.Sprint, to the start
// of its first span.
func sourceLines(f *descriptorpb.FileDescriptorProto) map[string]position {
	m := make(map[string]position)
	for _, loc := range f.GetSourceCodeInfo().GetLocation() {
		key, span := fmt.Sprint(loc.GetPath()), loc.GetSpan()
		if _, ok := m[key]; !ok && len(span) >= 3 {
			m[key] = position{line: int(span[0]) + 1, column: int(span[1]) + 1}
		}
	}
	return m
}

type fileChecker struct {
	file     *descriptorpb.FileDescriptorProto
	lines    map[string]position
	messages map[string]*descriptorpb.DescriptorProto // by ".package.Name", every file of the set
	inputs   map[string]bool                          // messages an RPC input of the set reaches
	owner    string                                   // ".package.Name" of the message whose fields are checked
	out      []Violation
}

// reachableFromInputs returns the names (leading dot) of every message that is
// the input of an RPC of set, or is reached from one through message-typed
// fields. A host_only mark elsewhere is never read by a request walk.
func reachableFromInputs(set *descriptorpb.FileDescriptorSet, messages map[string]*descriptorpb.DescriptorProto) map[string]bool {
	seen := make(map[string]bool)
	var visit func(name string)
	visit = func(name string) {
		d := messages[name]
		if d == nil || seen[name] {
			return
		}
		seen[name] = true
		for _, f := range d.GetField() {
			if f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
				visit(f.GetTypeName())
			}
		}
	}
	for _, f := range set.GetFile() {
		for _, s := range f.GetService() {
			for _, m := range s.GetMethod() {
				visit(m.GetInputType())
			}
		}
	}
	return seen
}

// indexMessages maps the fully qualified name (leading dot, as in type_name) of
// every message of set, nested ones included, to its descriptor.
func indexMessages(set *descriptorpb.FileDescriptorSet) map[string]*descriptorpb.DescriptorProto {
	m := make(map[string]*descriptorpb.DescriptorProto)
	var add func(prefix string, d *descriptorpb.DescriptorProto)
	add = func(prefix string, d *descriptorpb.DescriptorProto) {
		name := prefix + "." + d.GetName()
		m[name] = d
		for _, n := range d.GetNestedType() {
			add(name, n)
		}
	}
	for _, f := range set.GetFile() {
		for _, d := range f.GetMessageType() {
			add("."+f.GetPackage(), d)
		}
	}
	return m
}

// holdsReference reports whether message type name is, or reaches through
// message-typed fields, a Reference. Types outside the set are not followed.
func (c *fileChecker) holdsReference(name string, seen map[string]bool) bool {
	if name == referenceType {
		return true
	}
	d := c.messages[name]
	if d == nil || seen[name] {
		return false
	}
	seen[name] = true
	for _, f := range d.GetField() {
		if f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && c.holdsReference(f.GetTypeName(), seen) {
			return true
		}
	}
	return false
}

func (c *fileChecker) run() {
	pkg := c.file.GetPackage()
	// qualify returns the full name of a top-level element; a file without a
	// package has no leading dot.
	qualify := func(name string) string { return strings.TrimPrefix(pkg+"."+name, ".") }
	for si, s := range c.file.GetService() {
		for mi, m := range s.GetMethod() {
			path := []int32{fileServiceTag, int32(si), serviceMethodTag, int32(mi)}
			c.checkMethod(qualify(s.GetName())+"."+m.GetName(), m, path)
		}
	}
	if c.file.GetName() != annotationsFile {
		for ei, e := range c.file.GetExtension() {
			c.checkField(qualify(e.GetName()), e, []int32{fileExtensionTag, int32(ei)})
		}
	}
	for mi, m := range c.file.GetMessageType() {
		c.checkMessage(qualify(m.GetName()), m, []int32{fileMessageTag, int32(mi)})
	}
}

func (c *fileChecker) checkMethod(name string, m *descriptorpb.MethodDescriptorProto, path []int32) {
	opts := m.GetOptions()
	if opts == nil || !proto.HasExtension(opts, annotationsv1.E_Action) {
		c.report(path, RuleActionRequired, "RPC %s has no (numinance.annotations.v1.action) option.", name)
		return
	}
	a, _ := proto.GetExtension(opts, annotationsv1.E_Action).(*annotationsv1.ActionOptions)
	if a.GetRisk() == annotationsv1.RiskClass_RISK_CLASS_UNSPECIFIED {
		c.report(path, RuleActionRisk, "RPC %s action has no risk class.", name)
	}
	if a.GetExec() == annotationsv1.ExecMode_EXEC_MODE_UNSPECIFIED {
		c.report(path, RuleActionExec, "RPC %s action has no exec mode.", name)
	}
	if !isStrictVersion(a.GetSince()) {
		c.report(path, RuleActionSince, "RPC %s action since is missing or not a strict x.y.z version.", name)
	}
	if strings.TrimSpace(a.GetLlmDescription()) == "" {
		c.report(path, RuleActionLLMDescription, "RPC %s action has no llm_description.", name)
	}
	seen := map[string]bool{}
	for i, s := range a.GetScopes() {
		if !isScopeName(s) || seen[s] {
			// The message names the entry's position, not its text, so an option value is never echoed.
			c.report(path, RuleActionScope, "RPC %s action scopes entry %d is not a unique dotted lower-case scope name.", name, i)
		}
		seen[s] = true
	}
}

func (c *fileChecker) checkMessage(name string, m *descriptorpb.DescriptorProto, path []int32) {
	// Map entries are synthesized by the compiler and cannot carry options.
	if m.GetOptions().GetMapEntry() {
		return
	}
	c.owner = "." + name
	for fi, f := range m.GetField() {
		c.checkField(name+"."+f.GetName(), f, appendPath(path, messageFieldTag, int32(fi)))
	}
	c.owner = "" // an extension declaration is no field of a request
	for ei, e := range m.GetExtension() {
		c.checkField(name+"."+e.GetName(), e, appendPath(path, messageExtensionTag, int32(ei)))
	}
	for ni, n := range m.GetNestedType() {
		c.checkMessage(name+"."+n.GetName(), n, appendPath(path, messageNestedTag, int32(ni)))
	}
}

func (c *fileChecker) checkField(name string, f *descriptorpb.FieldDescriptorProto, path []int32) {
	var since string
	if opts := f.GetOptions(); opts != nil && proto.HasExtension(opts, annotationsv1.E_Field) {
		fa, _ := proto.GetExtension(opts, annotationsv1.E_Field).(*annotationsv1.FieldAnnotations)
		since = fa.GetSince()
		if fa.GetHostOnly() && !c.hostOnlyAllowed(f) {
			c.report(path, RuleHostOnly, "field %s is host_only but is not a types.v1.Reference field, or a wrapper of References, of a numinance.ports.* message that an RPC input reaches.", name)
		}
		if fa.GetResourceName() && !c.resourceNameAllowed(f) {
			c.report(path, RuleResourceName, "field %s is resource_name but is not a string or repeated string field of a numinance.ports.* message that an RPC input reaches.", name)
		}
	}
	if !isStrictVersion(since) {
		c.report(path, RuleFieldSince, "field %s has no (numinance.annotations.v1.field).since or it is not a strict x.y.z version.", name)
	}
}

// hostOnlyAllowed reports whether f may carry host_only: a message-typed field
// of a port file, in a message an RPC input reaches, whose type is a Reference
// or reaches one.
func (c *fileChecker) hostOnlyAllowed(f *descriptorpb.FieldDescriptorProto) bool {
	return strings.HasPrefix(c.file.GetPackage(), "numinance.ports.") &&
		c.inputs[c.owner] &&
		f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE &&
		c.holdsReference(f.GetTypeName(), map[string]bool{})
}

// resourceNameAllowed reports whether f may carry resource_name: a string field (repeated or not) of a port file, in a
// message an RPC input reaches (rfcs/0002-resource-name-field-annotation.md).
func (c *fileChecker) resourceNameAllowed(f *descriptorpb.FieldDescriptorProto) bool {
	return strings.HasPrefix(c.file.GetPackage(), "numinance.ports.") &&
		c.inputs[c.owner] &&
		f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_STRING
}

func (c *fileChecker) report(path []int32, rule, format string, args ...any) {
	pos := c.lines[fmt.Sprint(path)]
	c.out = append(c.out, Violation{
		File:    c.file.GetName(),
		Line:    pos.line,
		Column:  pos.column,
		Rule:    rule,
		Message: fmt.Sprintf(format, args...),
	})
}

// appendPath returns a new slice so sibling paths never share a backing array.
func appendPath(path []int32, tag, index int32) []int32 {
	return append(slices.Clip(path), tag, index)
}

// strictVersion is MAJOR.MINOR.PATCH without leading zeros, prefix, pre-release
// or build metadata. In RE2 \d is ASCII and $ (no m flag) matches only at the
// end of the text, so a trailing newline is rejected.
var strictVersion = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

func isStrictVersion(s string) bool { return strictVersion.MatchString(s) }

// maxScopeLen is the longest scope entry authority.Grant.Validate accepts.
const maxScopeLen = 256

// scopeName is the grammar of authority.Grant.Validate for a scope entry: two or more dot-separated segments, each
// starting with a lower-case letter. testdata/scope_grammar.json is read by this package's test and by the kernel's
// scope_grammar_mirror_test.go, so the two grammars cannot drift.
var scopeName = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func isScopeName(s string) bool { return len(s) <= maxScopeLen && scopeName.MatchString(s) }
