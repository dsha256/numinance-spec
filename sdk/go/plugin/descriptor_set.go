package plugin

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// DescriptorSet returns the FileDescriptorSet a provider answers GetDescriptors with: the given files and every file
// they import, each once, dependencies before dependents (the order protodesc.NewFiles needs). Source info is dropped.
func DescriptorSet(files ...protoreflect.FileDescriptor) (*descriptorpb.FileDescriptorSet, error) {
	set := &descriptorpb.FileDescriptorSet{}
	seen := map[string]bool{}
	var add func(fd protoreflect.FileDescriptor) error
	add = func(fd protoreflect.FileDescriptor) error {
		if seen[fd.Path()] {
			return nil
		}
		seen[fd.Path()] = true
		imports := fd.Imports()
		for i := range imports.Len() {
			if err := add(imports.Get(i).FileDescriptor); err != nil {
				return err
			}
		}
		set.File = append(set.File, protodesc.ToFileDescriptorProto(fd))
		return nil
	}
	for _, fd := range files {
		if err := add(fd); err != nil {
			return nil, fmt.Errorf("descriptor set: %w", err)
		}
	}
	for _, f := range set.File {
		f.SourceCodeInfo = nil
	}
	return set, nil
}
