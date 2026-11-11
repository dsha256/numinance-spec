package kindyaml

import (
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

// Deprecation is one use of a deprecated field in a decoded kind document: the field carries
// (numinance.annotations.v1.field).deprecated_in in its proto and is set in the document. A deprecated field still loads
// for the deprecation window (VERSIONING.md); it is reported, never refused.
type Deprecation struct {
	// Kind is the kind of the document, such as "Lockfile".
	Kind string
	// Path is the document path of the field in proto field names, with list positions, such as
	// "spec.knowledge_tools[0]" or "spec.ports.knowledge". It names a field and never a value.
	Path string
	// Field is the full name of the proto field, such as "numinance.kinds.v1.LockfileSpec.knowledge_tools".
	Field string
	// DeprecatedIn is the x.y.z spec version that deprecated the field.
	DeprecatedIn string
	// ReplacedBy names the field that supersedes it, or is empty.
	ReplacedBy string
}

// Diagnostic returns the structured note a host emits for the use: reason DEPRECATED at warning severity. The
// Diagnostic of plugin/v1 resolves its field_path and subject against a port or plugin message, which a kind document is
// not, so both stay empty and the Deprecation carries the path.
func (d Deprecation) Diagnostic() *pluginv1.Diagnostic {
	return &pluginv1.Diagnostic{Reason: pluginv1.Reason_REASON_DEPRECATED, Severity: pluginv1.Severity_SEVERITY_WARNING}
}

// Deprecations returns every use of a deprecated field in m, a message Decode returned, in field order. A deprecated
// field counts as used when it is set (a non-empty list or map, a message or optional that is present, or a
// non-zero singular). The result is empty for a document that uses none.
func Deprecations(m proto.Message) []Deprecation {
	if m == nil || !m.ProtoReflect().IsValid() {
		return nil
	}
	r := m.ProtoReflect()
	var out []Deprecation
	collectDeprecations(r, "", string(r.Descriptor().Name()), &out)
	return out
}

func collectDeprecations(m protoreflect.Message, path, kind string, out *[]Deprecation) {
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if !m.Has(fd) {
			continue
		}
		p := string(fd.Name())
		if path != "" {
			p = path + "." + p
		}
		if opts := fd.Options(); opts != nil {
			if a, _ := proto.GetExtension(opts, annotationsv1.E_Field).(*annotationsv1.FieldAnnotations); a.GetDeprecatedIn() != "" {
				*out = append(*out, Deprecation{Kind: kind, Path: p, Field: string(fd.FullName()), DeprecatedIn: a.GetDeprecatedIn(), ReplacedBy: a.GetReplacedBy()})
			}
		}
		if fd.Message() == nil || fd.IsMap() || fd.Name() == "extensions" {
			continue
		}
		v := m.Get(fd)
		if fd.IsList() {
			for j := range v.List().Len() {
				collectDeprecations(v.List().Get(j).Message(), p+"["+strconv.Itoa(j)+"]", kind, out)
			}
			continue
		}
		collectDeprecations(v.Message(), p, kind, out)
	}
}
