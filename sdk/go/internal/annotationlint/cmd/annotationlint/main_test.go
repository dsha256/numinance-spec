package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func writeSet(t *testing.T, set *descriptorpb.FileDescriptorSet) string {
	t.Helper()
	b, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "image.binpb")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func TestRun(t *testing.T) {
	unannotated := writeSet(t, &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{{
		Name:    proto.String("numinance/x/v1/x.proto"),
		Package: proto.String("numinance.x.v1"),
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name:   proto.String("S"),
			Method: []*descriptorpb.MethodDescriptorProto{{Name: proto.String("Get")}},
		}},
	}}})
	empty := writeSet(t, &descriptorpb.FileDescriptorSet{})
	missing := filepath.Join(t.TempDir(), "none")
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "violations exit 1 with the prefix applied",
			args:       []string{"-prefix", "proto/", unannotated},
			wantCode:   1,
			wantStdout: "proto/numinance/x/v1/x.proto:0:0:ACTION_ANNOTATION_REQUIRED RPC numinance.x.v1.S.Get has no (numinance.annotations.v1.action) option.\n",
		},
		{name: "clean set exits 0", args: []string{empty}, wantCode: 0},
		{name: "no argument exits 2", wantCode: 2, wantStderr: "usage:"},
		{name: "unknown flag exits 2", args: []string{"-x", empty}, wantCode: 2, wantStderr: "flag provided but not defined"},
		{name: "missing file exits 2 naming the path", args: []string{missing}, wantCode: 2, wantStderr: missing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("run(%q) = %d, want %d (stderr %q)", tt.args, got, tt.wantCode, stderr.String())
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("run(%q) stdout = %q, want %q", tt.args, got, tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("run(%q) stderr = %q, want it to contain %q", tt.args, stderr.String(), tt.wantStderr)
			}
		})
	}
}
