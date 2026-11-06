package plugin

import (
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
)

// annotationTypes resolves only the generated numinance annotations, so an
// option re-decoded with it holds the generated type, whatever resolver built
// the descriptor the option came from.
// annotationTypes is built once and never written after: callers only read it.
// The nolint is for that single immutable value.
//
//nolint:gochecknoglobals // read-only registry of two generated extensions
var annotationTypes = sync.OnceValue(func() *protoregistry.Types {
	var t protoregistry.Types
	// Registration of two distinct generated extensions cannot conflict.
	_ = t.RegisterExtension(annotationsv1.E_Field)
	_ = t.RegisterExtension(annotationsv1.E_Action)
	return &t
})

// reload copies options (any message of the same type as dst, generated or
// dynamic) into dst through its wire form, resolving the numinance extensions to
// their generated types. Calling proto.GetExtension with a generated type on a
// descriptor built with a dynamic resolver panics, so the checks never do. An
// error means the options are not readable; the caller fails closed.
func reload(options, dst proto.Message) error {
	if options == nil {
		return nil
	}
	wire, err := proto.Marshal(options)
	if err != nil {
		return err
	}
	return proto.UnmarshalOptions{Resolver: annotationTypes()}.Unmarshal(wire, dst)
}

func fieldAnnotations(options proto.Message) (*descriptorpb.FieldOptions, *annotationsv1.FieldAnnotations, error) {
	var opts descriptorpb.FieldOptions
	if err := reload(options, &opts); err != nil {
		return nil, nil, err
	}
	f, _ := proto.GetExtension(&opts, annotationsv1.E_Field).(*annotationsv1.FieldAnnotations)
	return &opts, f, nil
}
