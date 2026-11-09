package extpkg_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/extpkg"
)

type entry struct {
	name, data string
	mode       fs.FileMode
}

func build(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatalf("CreateHeader(%q) = %v", e.name, err)
		}
		if _, err := f.Write([]byte(e.data)); err != nil {
			t.Fatalf("Write(%q) = %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	return b.Bytes()
}

func TestHashes_Vectors(t *testing.T) {
	t.Parallel()
	files := []extpkg.File{{Path: "a", Data: []byte("hi")}, {Path: extpkg.ManifestFile, Data: []byte("m")}}
	if got, want := extpkg.ContentDigest(files), "sha256:abb119cdfb1c0bdfc48e54c42f89a39d421f44bfac0d605cd9b25c01f1181f0c"; got != want {
		t.Errorf("ContentDigest() = %s, want %s (manifest.yaml excluded)", got, want)
	}
	if got, want := extpkg.TreeHash(files[:1]), "h1:q7EZzfscC9/EjlTEL4mjnUIfRL+sDWBc2bJcAfEYHww="; got != want {
		t.Errorf("TreeHash(a=hi) = %s, want %s", got, want)
	}
	if extpkg.TreeHash(files) == extpkg.TreeHash(files[:1]) {
		t.Error("TreeHash ignores manifest.yaml")
	}
	if got, want := extpkg.ArchiveHash([]byte("zip")), "zh:4a70fe9aa6436e02c2dea340fbd1e352e4ef2d8ce6ca52ad25d4b95471fc8bf2"; got != want {
		t.Errorf("ArchiveHash() = %s, want %s", got, want)
	}
}

func TestUnpack_Accepts(t *testing.T) {
	t.Parallel()
	got, err := extpkg.Unpack(build(t, entry{name: "b/c", data: "2"}, entry{name: "a", data: "1"}, entry{name: "d/"}), extpkg.DefaultLimits())
	if err != nil || len(got) != 2 || got[0].Path != "a" || got[1].Path != "b/c" {
		t.Errorf("Unpack() = %v, %v; want a and b/c sorted", got, err)
	}
}

func TestUnpack_Refuses(t *testing.T) {
	t.Parallel()
	small := extpkg.Limits{MaxFiles: 2, MaxFileBytes: 8, MaxTotal: 12}
	def := extpkg.DefaultLimits()
	tests := []struct {
		name    string
		archive []byte
		limits  extpkg.Limits
	}{
		{name: "not a zip", archive: []byte("nope"), limits: def},
		{name: "parent segment", archive: build(t, entry{name: "../x", data: "x"}), limits: def},
		{name: "absolute", archive: build(t, entry{name: "/x", data: "x"}), limits: def},
		{name: "empty segment", archive: build(t, entry{name: "a//b", data: "x"}), limits: def},
		{name: "backslash", archive: build(t, entry{name: `a\b`, data: "x"}), limits: def},
		{name: "control character", archive: build(t, entry{name: "a\nb", data: "x"}), limits: def},
		{name: "symlink", archive: build(t, entry{name: "l", data: "/etc/passwd", mode: fs.ModeSymlink | 0o777}), limits: def},
		{name: "duplicate", archive: build(t, entry{name: "a", data: "1"}, entry{name: "a", data: "2"}), limits: def},
		{name: "case fold collision", archive: build(t, entry{name: "Readme.md", data: "1"}, entry{name: "README.md", data: "2"}), limits: def},
		{name: "NFC and NFD spellings", archive: build(t, entry{name: "café", data: "1"}, entry{name: "café", data: "2"}), limits: def},
		{name: "non-ASCII path", archive: build(t, entry{name: "café", data: "1"}), limits: def},
		{name: "file and directory", archive: build(t, entry{name: "a", data: "1"}, entry{name: "a/b", data: "2"}), limits: def},
		{name: "file and deeper directory, other case", archive: build(t, entry{name: "A", data: "1"}, entry{name: "a/b/c", data: "2"}), limits: def},
		{name: "too many", archive: build(t, entry{name: "a", data: "1"}, entry{name: "b", data: "1"}, entry{name: "c", data: "1"}), limits: small},
		{name: "bomb", archive: build(t, entry{name: "a", data: strings.Repeat("0", 1<<20)}), limits: small},
		{name: "total", archive: build(t, entry{name: "a", data: "12345678"}, entry{name: "b", data: "12345"}), limits: small},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if files, err := extpkg.Unpack(tt.archive, tt.limits); !errors.Is(err, extpkg.ErrArchive) {
				t.Errorf("Unpack(%s) = %v, %v; want ErrArchive", tt.name, files, err)
			}
		})
	}
}
