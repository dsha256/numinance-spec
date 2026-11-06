package provider

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const testBase = "0123456789abcdef0123456789abcdef01234567"

// file is one changed path of a test candidate.
type file struct {
	path, status, mode, body string
	link                     string
}

func add(path, body string) file { return file{path: path, status: "A", mode: "100644", body: body} }
func mod(path, body string) file { return file{path: path, status: "M", mode: "100644", body: body} }
func del(path string) file       { return file{path: path, status: "D"} }

// tarOf writes a candidate archive the way the host-run executor path does: the manifest first, then each present file under files/.
func tarOf(tb testing.TB, base string, files ...file) []byte {
	tb.Helper()
	type entry struct {
		Path   string `json:"path"`
		Status string `json:"status"`
		Mode   string `json:"mode,omitempty"`
	}
	m := struct {
		Version int     `json:"version"`
		Base    string  `json:"base"`
		Entries []entry `json:"entries"`
	}{Version: 1, Base: base}
	for _, f := range files {
		m.Entries = append(m.Entries, entry{Path: f.path, Status: f.status, Mode: f.mode})
	}
	doc, err := json.Marshal(m)
	if err != nil {
		tb.Fatal(err)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	put := func(h tar.Header, body []byte) {
		h.Size = int64(len(body))
		if err := tw.WriteHeader(&h); err != nil {
			tb.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			tb.Fatal(err)
		}
	}
	put(tar.Header{Name: ManifestName, Typeflag: tar.TypeReg, Mode: 0o644}, doc)
	for _, f := range files {
		switch {
		case f.status == "D":
		case f.mode == "120000":
			put(tar.Header{Name: FilesPrefix + f.path, Typeflag: tar.TypeSymlink, Linkname: f.link, Mode: 0o777}, nil)
		default:
			put(tar.Header{Name: FilesPrefix + f.path, Typeflag: tar.TypeReg, Mode: 0o644}, []byte(f.body))
		}
	}
	if err := tw.Close(); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

func parse(tb testing.TB, files ...file) Candidate {
	tb.Helper()
	c, err := ParseCandidate(tarOf(tb, testBase, files...))
	if err != nil {
		tb.Fatalf("ParseCandidate() error = %v, want nil", err)
	}
	return c
}

func ptr(n int) *int { return &n }

// scope is a scope that allows new files under docs/runbooks and nothing else.
func scope() Scope {
	return Scope{AllowPaths: []string{"docs/runbooks/"}, ProtectedPaths: []string{".claude/", "docs/design/"}, MaxFiles: ptr(1), MaxAddedLines: ptr(50), MaxAddedBytes: ptr(1 << 20), MaxDeletedFiles: ptr(0)}
}

func rules(fs []finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.rule)
	}
	return out
}

func has(fs []finding, rule string) bool {
	for _, f := range fs {
		if f.rule == rule {
			return true
		}
	}
	return false
}

func lines(n int) string { return strings.Repeat("x\n", n) }
