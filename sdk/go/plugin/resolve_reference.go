package plugin

import (
	"errors"
	"reflect"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

var (
	// ErrReferenceHostOnly is returned for a reference whose sha256.value equals
	// that of a field the port marks host_only, or of a Reference nested under one.
	ErrReferenceHostOnly = errors.New("reference is host-only")
	// ErrReferenceNotFound is returned for a reference that no field of the
	// request holds, and for a reference with no artifact_ref or no digest.
	ErrReferenceNotFound = errors.New("reference not found in the request")
	// ErrReferenceUnreadable is returned when a Reference in the request cannot be
	// read, so the check cannot tell whether it is host-only; it fails closed.
	ErrReferenceUnreadable = errors.New("a reference in the request cannot be read")
)

// fieldAnnotationsNumber is the number of the (numinance.annotations.v1.field)
// extension, the whole FieldAnnotations message; host_only is field 5 inside it.
// A descriptor decoded without the extension keeps it as unknown bytes, and
// every port field carries it (for since), so the number alone marks nothing.
const fieldAnnotationsNumber protoreflect.FieldNumber = 50002

// hostOnlyNumber is host_only inside FieldAnnotations.
const hostOnlyNumber protoreflect.FieldNumber = 5

// resourceNameNumber is resource_name inside FieldAnnotations (rfcs/0002-resource-name-field-annotation.md).
const resourceNameNumber protoreflect.FieldNumber = 6

// referenceName is the full name of types.v1.Reference, compared with the
// descriptor of each message the walk finds so generated, dynamic and map-entry
// values all match by type, not by Go type.
const referenceName protoreflect.FullName = "numinance.types.v1.Reference"

// CheckResolvable decides whether HostService.ResolveReference may serve ref
// for the invocation whose typed request is req: the unpacked method input
// (generated or dynamic), not the Envelope that carried it. It walks req
// through nested messages, lists and map values, never an Any, and compares ref
// with every types.v1.Reference it finds. A reference whose sha256.value equals
// that of a host_only field, or of a Reference nested under one, is refused even
// when a resolvable field holds the same value or the artifact_type or
// artifact_ref differs: the digest names the object. The option is read through
// the generated annotations whatever resolver built the descriptor; an option
// that does not decode counts as marked. A Reference that cannot be read, and
// any message under req with unknown bytes, fails with ErrReferenceUnreadable,
// but a host_only match wins over it. A reference no field holds (proto.Equal on
// all three fields), a nil req, or one with an empty artifact_ref or digest, is
// not found.
func CheckResolvable(req proto.Message, ref *typesv1.Reference) error {
	if isNil(req) || ref.GetArtifactRef() == "" || ref.GetSha256().GetValue() == "" {
		return ErrReferenceNotFound
	}
	found, hostOnly, unreadable := false, false, false
	walkReferences(req.ProtoReflect(), false, func(v *typesv1.Reference, marked bool) {
		if v == nil {
			unreadable = true
			return
		}
		if marked && v.GetSha256().GetValue() == ref.GetSha256().GetValue() {
			hostOnly = true
		}
		found = found || proto.Equal(v, ref)
	})
	switch {
	case hostOnly:
		return ErrReferenceHostOnly
	case unreadable:
		return ErrReferenceUnreadable
	case !found:
		return ErrReferenceNotFound
	}
	return nil
}

// isHostOnly reports the mark on fd. The option is re-decoded with only the
// generated annotations (see reload), so a descriptor built with any resolver
// reads the same. An option that does not decode, or whose annotations stay in
// unknown bytes (number 50002, which only a wrong wire type leaves there), counts
// as marked: the check never becomes a no-op. A resolved option marks the field
// only when host_only is true; an option with just since marks nothing.
func isHostOnly(fd protoreflect.FieldDescriptor) bool {
	opts, f, err := fieldAnnotations(fd.Options())
	if err != nil || f.GetHostOnly() || duplicateFieldAnnotations(fd.Options()) {
		return true
	}
	// A host_only encoded with another wire type lands in the unknown bytes of
	// FieldAnnotations and reads false; so does any unparsable byte.
	return hasUnknownNumber(opts.ProtoReflect().GetUnknown(), fieldAnnotationsNumber) ||
		hasUnknownNumber(f.ProtoReflect().GetUnknown(), hostOnlyNumber)
}

// duplicateFieldAnnotations reports more than one (numinance.annotations.v1.field)
// entry in the options, or more than one host_only entry in one of them. Merge
// semantics let a later host_only=false strip an earlier mark. The entries are
// counted on the wire form of options, so this sees duplicates only where they
// survive as unknown bytes (a descriptor decoded without numinance.annotations.v1).
// A descriptor decoded with the annotations linked has merged them before
// Options() returns and re-marshals as one entry; the build-time annotation lint
// rejects duplicates in the spec module, and a host that loads descriptor sets at
// runtime calls DuplicateFieldAnnotationsWire on the raw FieldOptions bytes it
// decoded with an empty resolver. An error while scanning counts as duplicate.
func duplicateFieldAnnotations(options proto.Message) bool {
	if options == nil {
		return false
	}
	wire, err := proto.Marshal(options)
	if err != nil {
		return true
	}
	return DuplicateFieldAnnotationsWire(wire)
}

// DuplicateFieldAnnotationsWire reports more than one (numinance.annotations.v1.field)
// entry, or more than one host_only or resource_name entry in one, on the serialized FieldOptions
// (or just their unknown bytes): wire that does not parse counts as duplicate.
// Call it on the raw bytes of a descriptor decoded with an empty resolver; on the
// options of a descriptor decoded with the annotations linked the runtime has
// already merged the entries and the answer is always false.
func DuplicateFieldAnnotationsWire(wire []byte) bool {
	entries := 0
	for b := wire; len(b) > 0; {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return true
		}
		b = b[n:]
		m := protowire.ConsumeFieldValue(num, typ, b)
		if m < 0 {
			return true
		}
		if num == fieldAnnotationsNumber && typ == protowire.BytesType {
			entries++
			inner, _ := protowire.ConsumeBytes(b)
			if countNumber(inner, hostOnlyNumber) > 1 || countNumber(inner, resourceNameNumber) > 1 {
				return true
			}
		}
		b = b[m:]
	}
	return entries > 1
}

// countNumber counts the fields numbered num in raw; bytes that do not parse
// stop the count.
func countNumber(raw []byte, num protoreflect.FieldNumber) int {
	count := 0
	for b := raw; len(b) > 0; {
		got, _, n := protowire.ConsumeField(b)
		if n < 0 {
			return count
		}
		if got == num {
			count++
		}
		b = b[n:]
	}
	return count
}

// hasUnknownNumber reports field num in raw, or bytes that do not parse.
func hasUnknownNumber(raw []byte, num protoreflect.FieldNumber) bool {
	for b := raw; len(b) > 0; {
		got, _, n := protowire.ConsumeField(b)
		if n < 0 || got == num {
			return true
		}
		b = b[n:]
	}
	return false
}

// walkReferences calls visit for every Reference under m. marked is true when
// the Reference sits in a host_only field or under a message in one. A
// Reference that cannot be copied, and any message with unknown bytes, is
// reported as a nil Reference.
func walkReferences(m protoreflect.Message, inherited bool, visit func(ref *typesv1.Reference, marked bool)) {
	if len(m.GetUnknown()) > 0 {
		visit(nil, inherited)
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		marked := inherited
		if isHostOnly(fd) {
			marked = true
		}
		each := func(item protoreflect.Value) {
			sub := item.Message()
			if sub.Descriptor().FullName() != referenceName {
				walkReferences(sub, marked, visit)
				return
			}
			if len(sub.GetUnknown()) > 0 {
				visit(nil, marked)
				return
			}
			if r, ok := sub.Interface().(*typesv1.Reference); ok {
				visit(r, marked)
				return
			}
			// A dynamic message: copy it through its wire form into the generated
			// type; one that does not copy is reported as nil, so the check fails closed.
			var r typesv1.Reference
			if b, err := proto.Marshal(sub.Interface()); err == nil && proto.Unmarshal(b, &r) == nil {
				visit(&r, marked)
				return
			}
			visit(nil, marked)
		}
		switch {
		case fd.IsMap():
			if fd.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
					each(mv)
					return true
				})
			}
		case fd.IsList():
			if fd.Message() != nil {
				for i := range v.List().Len() {
					each(v.List().Get(i))
				}
			}
		case fd.Message() != nil:
			each(v)
		}
		return true
	})
}

// isNil reports a nil interface or a typed nil pointer (generated or dynamic).
func isNil(m proto.Message) bool {
	if m == nil {
		return true
	}
	v := reflect.ValueOf(m)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
