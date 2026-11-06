package annotationlint

import (
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// MaxDescriptorSetBytes caps the input of Decode (the spec module is under 2 MiB).
const MaxDescriptorSetBytes = 64 << 20

// ErrTooLarge is returned when the input exceeds MaxDescriptorSetBytes.
var ErrTooLarge = errors.New("annotationlint: descriptor set exceeds MaxDescriptorSetBytes")

// Decode reads a binary google.protobuf.FileDescriptorSet, as written by
// `buf build --as-file-descriptor-set`, from r.
func Decode(r io.Reader) (*descriptorpb.FileDescriptorSet, error) {
	b, err := readLimited(r)
	if err != nil {
		return nil, err
	}
	return decodeBytes(b)
}

func readLimited(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxDescriptorSetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("annotationlint: read descriptor set: %w", err)
	}
	if len(b) > MaxDescriptorSetBytes {
		return nil, ErrTooLarge
	}
	return b, nil
}

func decodeBytes(b []byte) (*descriptorpb.FileDescriptorSet, error) {
	set := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(b, set); err != nil {
		return nil, fmt.Errorf("annotationlint: decode descriptor set: %w", err)
	}
	return set, nil
}
