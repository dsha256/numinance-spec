package annotationlint

import (
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// RuleDuplicateOptions reports a field with more than one
// (numinance.annotations.v1.field) entry, or more than one host_only or
// resource_name entry in one. A runtime merges them, so a later false would strip the mark.
const RuleDuplicateOptions = "FIELD_OPTION_DUPLICATE"

// CheckDuplicates decodes raw (a binary FileDescriptorSet) with an empty
// resolver, so every extension stays in the unknown bytes of its options and
// duplicate entries are still visible, and reports each field that carries them.
func CheckDuplicates(raw []byte) ([]Violation, error) {
	set := new(descriptorpb.FileDescriptorSet)
	if err := (proto.UnmarshalOptions{Resolver: new(protoregistry.Types)}).Unmarshal(raw, set); err != nil {
		return nil, fmt.Errorf("annotationlint: decode descriptor set: %w", err)
	}
	var out []Violation
	for _, f := range set.GetFile() {
		c := fileChecker{file: f, lines: sourceLines(f)}
		pkg := f.GetPackage()
		for ei, e := range f.GetExtension() {
			c.checkDuplicate(qualified(pkg, e.GetName()), e, []int32{fileExtensionTag, int32(ei)})
		}
		for mi, m := range f.GetMessageType() {
			c.duplicatesIn(qualified(pkg, m.GetName()), m, []int32{fileMessageTag, int32(mi)})
		}
		out = append(out, c.out...)
	}
	return out, nil
}

func qualified(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

func (c *fileChecker) duplicatesIn(name string, m *descriptorpb.DescriptorProto, path []int32) {
	for fi, f := range m.GetField() {
		c.checkDuplicate(name+"."+f.GetName(), f, appendPath(path, messageFieldTag, int32(fi)))
	}
	for ei, e := range m.GetExtension() {
		c.checkDuplicate(name+"."+e.GetName(), e, appendPath(path, messageExtensionTag, int32(ei)))
	}
	for ni, n := range m.GetNestedType() {
		c.duplicatesIn(name+"."+n.GetName(), n, appendPath(path, messageNestedTag, int32(ni)))
	}
}

func (c *fileChecker) checkDuplicate(name string, f *descriptorpb.FieldDescriptorProto, path []int32) {
	if f.GetOptions() == nil {
		return
	}
	if plugin.DuplicateFieldAnnotationsWire(f.GetOptions().ProtoReflect().GetUnknown()) {
		c.report(path, RuleDuplicateOptions, "field %s has more than one (numinance.annotations.v1.field) entry or more than one host_only or resource_name entry; a runtime merges them and a later false strips the mark.", name)
	}
}

// Lint reads a binary FileDescriptorSet from r once and returns the violations of
// Check and CheckDuplicates, ordered by file, line, column and rule.
func Lint(r io.Reader) ([]Violation, error) {
	raw, err := readLimited(r)
	if err != nil {
		return nil, err
	}
	set, err := decodeBytes(raw)
	if err != nil {
		return nil, err
	}
	dups, err := CheckDuplicates(raw)
	if err != nil {
		return nil, err
	}
	return sortViolations(append(Check(set), dups...)), nil
}
