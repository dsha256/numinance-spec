package provider

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseCandidate_ReadsTheChangesOfAnArchive(t *testing.T) {
	c := parse(t, add("docs/a.md", "one\ntwo"), mod("Justfile", "check:\n"), del("old.txt"), add("bin/blob.dat", lines(3)),
		file{path: "link", status: "A", mode: "120000", link: "target"})
	if c.Base != testBase || len(c.Changes) != 5 {
		t.Fatalf("ParseCandidate() = %+v, want the base and five changes", c)
	}
	byPath := map[string]Change{}
	for _, ch := range c.Changes {
		byPath[ch.Path] = ch
	}
	if a := byPath["docs/a.md"]; a.Lines != 2 || string(a.Content) != "one\ntwo" {
		t.Errorf("docs/a.md = %+v, want 2 lines (a last line without a newline counts) and its content", a)
	}
	if b := byPath["bin/blob.dat"]; b.Lines != 3 || b.Content != nil {
		t.Errorf("bin/blob.dat = %+v, want 3 lines counted and no content kept", b)
	}
	if l := byPath["link"]; !l.Symlink || l.Lines != 0 {
		t.Errorf("link = %+v, want a symlink that is never followed", l)
	}
	if d := byPath["old.txt"]; d.Status != "D" || d.Lines != 0 {
		t.Errorf("old.txt = %+v, want a deletion", d)
	}
}

// raw writes a tar archive from headers and bodies, without the helper's manifest, so a test can break any rule.
type raw struct {
	h    tar.Header
	body string
}

func rawTar(t *testing.T, entries ...raw) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		e.h.Size = int64(len(e.body))
		if err := tw.WriteHeader(&e.h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func manifestOf(entries string) raw {
	return raw{h: tar.Header{Name: ManifestName, Typeflag: tar.TypeReg}, body: `{"version":1,"base":"` + testBase + `","entries":[` + entries + `]}`}
}

func regular(name, body string) raw {
	return raw{h: tar.Header{Name: name, Typeflag: tar.TypeReg}, body: body}
}

const entryA = `{"path":"a.md","status":"A","mode":"100644"}`

func TestParseCandidate_RefusesWhatIsNotACandidate(t *testing.T) {
	tests := []struct {
		name    string
		archive []byte
		want    error
	}{
		{"an empty archive", nil, ErrCandidate},
		{"garbage that is not tar", []byte(strings.Repeat("not a tar archive", 100)), ErrCandidate},
		{"a first entry that is not the manifest", rawTar(t, regular("files/a.md", "x"), manifestOf(entryA)), ErrCandidate},
		{"a manifest that is a directory", rawTar(t, raw{h: tar.Header{Name: ManifestName, Typeflag: tar.TypeDir}}), ErrCandidate},
		{"a manifest with an unknown field", rawTar(t, raw{h: tar.Header{Name: ManifestName, Typeflag: tar.TypeReg}, body: `{"version":1,"base":"` + testBase + `","entries":[],"x":1}`}), ErrCandidate},
		{"a manifest of another version", rawTar(t, raw{h: tar.Header{Name: ManifestName, Typeflag: tar.TypeReg}, body: `{"version":2,"base":"` + testBase + `","entries":[]}`}), ErrCandidate},
		{"a manifest with a short base", rawTar(t, raw{h: tar.Header{Name: ManifestName, Typeflag: tar.TypeReg}, body: `{"version":1,"base":"HEAD","entries":[]}`}), ErrCandidate},
		{"a manifest with trailing data", rawTar(t, raw{h: tar.Header{Name: ManifestName, Typeflag: tar.TypeReg}, body: `{"version":1,"base":"` + testBase + `","entries":[]} {}`}), ErrCandidate},
		{"a manifest that is not json", rawTar(t, raw{h: tar.Header{Name: ManifestName, Typeflag: tar.TypeReg}, body: `<xml/>`}), ErrCandidate},
		{"a manifest path that climbs out", rawTar(t, manifestOf(`{"path":"../x","status":"A","mode":"100644"}`)), ErrCandidate},
		{"a manifest path that is absolute", rawTar(t, manifestOf(`{"path":"/etc/passwd","status":"A","mode":"100644"}`)), ErrCandidate},
		{"a manifest path with a backslash", rawTar(t, manifestOf(`{"path":"a\\b","status":"A","mode":"100644"}`)), ErrCandidate},
		{"a manifest path that repeats", rawTar(t, manifestOf(entryA+","+entryA), regular("files/a.md", "x")), ErrCandidate},
		{"an unknown status", rawTar(t, manifestOf(`{"path":"a.md","status":"R","mode":"100644"}`)), ErrCandidate},
		{"a deletion with a mode", rawTar(t, manifestOf(`{"path":"a.md","status":"D","mode":"100644"}`)), ErrCandidate},
		{"a gitlink mode", rawTar(t, manifestOf(`{"path":"a","status":"A","mode":"160000"}`)), ErrCandidate},
		{"no mode on an added file", rawTar(t, manifestOf(`{"path":"a.md","status":"A"}`)), ErrCandidate},
		{"a name outside files/", rawTar(t, manifestOf(entryA), regular("a.md", "x")), ErrCandidate},
		{"zip slip through the files prefix", rawTar(t, manifestOf(entryA), regular("files/../../etc/cron.d/x", "x")), ErrCandidate},
		{"zip slip in the middle", rawTar(t, manifestOf(entryA), regular("files/a/../../b", "x")), ErrCandidate},
		{"an absolute name after the prefix", rawTar(t, manifestOf(entryA), regular("files//etc/passwd", "x")), ErrCandidate},
		{"a name with a backslash", rawTar(t, manifestOf(entryA), regular(`files/a\b`, "x")), ErrCandidate},
		{"a name with a control character", rawTar(t, manifestOf(entryA), regular("files/a\x01b", "x")), ErrCandidate},
		{"a repeated name", rawTar(t, manifestOf(entryA), regular("files/a.md", "x"), regular("files/a.md", "y")), ErrCandidate},
		{"a file the manifest does not list", rawTar(t, manifestOf(entryA), regular("files/a.md", "x"), regular("files/b.md", "x")), ErrCandidate},
		{"a file of a deleted path", rawTar(t, manifestOf(`{"path":"a.md","status":"D"}`), regular("files/a.md", "x")), ErrCandidate},
		{"a listed file the archive lacks", rawTar(t, manifestOf(entryA)), ErrCandidate},
		{"a directory entry", rawTar(t, manifestOf(entryA), raw{h: tar.Header{Name: "files/dir/", Typeflag: tar.TypeDir}}), ErrCandidate},
		{"a hard link", rawTar(t, manifestOf(entryA), raw{h: tar.Header{Name: "files/a.md", Typeflag: tar.TypeLink, Linkname: "files/b"}}), ErrCandidate},
		{"a device", rawTar(t, manifestOf(entryA), raw{h: tar.Header{Name: "files/a.md", Typeflag: tar.TypeChar}}), ErrCandidate},
		{"a fifo", rawTar(t, manifestOf(entryA), raw{h: tar.Header{Name: "files/a.md", Typeflag: tar.TypeFifo}}), ErrCandidate},
		{"a symlink where the manifest says file", rawTar(t, manifestOf(entryA), raw{h: tar.Header{Name: "files/a.md", Typeflag: tar.TypeSymlink, Linkname: "x"}}), ErrCandidate},
		{"a file where the manifest says link", rawTar(t, manifestOf(`{"path":"a.md","status":"A","mode":"120000"}`), regular("files/a.md", "x")), ErrCandidate},
		{"a sparse entry (PAX record)", sparsePAX(t), ErrCandidate},
		{"a sparse entry (typeflag S)", rawTar(t, manifestOf(entryA), raw{h: tar.Header{Name: "files/a.md", Typeflag: tar.TypeGNUSparse}}), ErrCandidate},
		{"a document over its bound", rawTar(t, manifestOf(entryA), regular("files/a.md", strings.Repeat("x", maxDocBytes+1))), ErrTooLarge},
		{"a file over its bound", rawTar(t, manifestOf(`{"path":"a.bin","status":"A","mode":"100644"}`), regular("files/a.bin", strings.Repeat("x", maxFileBytes+1))), ErrTooLarge},
		{"an archive over the limit", bytes.Repeat([]byte{0}, MaxCandidateBytes+1), ErrTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ParseCandidate(tc.archive)
			if !errors.Is(err, tc.want) {
				t.Errorf("ParseCandidate() = %+v, %v, want %v", c, err, tc.want)
			}
		})
	}
}

func TestParseCandidate_ARunOfEntriesOverTheCountIsTooLarge(t *testing.T) {
	entries := make([]string, 0, maxEntries+1)
	for i := range maxEntries + 1 {
		entries = append(entries, `{"path":"f`+strings.Repeat("0", 0)+itoa(i)+`","status":"D"}`)
	}
	archive := rawTar(t, manifestOf(strings.Join(entries, ",")))
	if _, err := ParseCandidate(archive); !errors.Is(err, ErrTooLarge) {
		t.Errorf("ParseCandidate(%d changed paths) error = %v, want ErrTooLarge", maxEntries+1, err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestParseCandidate_ATruncatedArchiveIsRefused(t *testing.T) {
	full := tarOf(t, testBase, add("a.bin", lines(2000)))
	for _, cut := range []int{len(full) / 2, 700, 1100} {
		if _, err := ParseCandidate(full[:cut]); err == nil {
			t.Errorf("ParseCandidate(first %d of %d bytes) = nil error, want a refusal", cut, len(full))
		}
	}
}

func TestParseCandidate_EveryRefusalLeavesNoPartialResult(t *testing.T) {
	c, err := ParseCandidate(rawTar(t, manifestOf(entryA)))
	if err == nil || len(c.Changes) != 0 || c.Base != "" {
		t.Errorf("ParseCandidate(a manifest with no file) = %+v, %v, want the zero Candidate and an error", c, err)
	}
}

// sparsePAX is a candidate whose file entry is preceded by a PAX extended header with a GNU.sparse record. The tar writer
// refuses to write such a header, so the header block of a plain entry is retyped by hand.
func sparsePAX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	write := func(h tar.Header, body string) int {
		if err := tw.Flush(); err != nil {
			t.Fatal(err)
		}
		at := buf.Len()
		h.Size = int64(len(body))
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
		return at
	}
	m := manifestOf(entryA)
	write(m.h, m.body)
	at := write(tar.Header{Name: "pax", Typeflag: tar.TypeReg}, "30 GNU.sparse.size=1000000000\n")
	write(tar.Header{Name: "files/a.md", Typeflag: tar.TypeReg}, "x")
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	block := out[at : at+512]
	block[156] = tar.TypeXHeader
	for i := 148; i < 156; i++ {
		block[i] = ' '
	}
	var sum int
	for _, b := range block {
		sum += int(b)
	}
	copy(block[148:], fmt.Sprintf("%06o\x00 ", sum))
	return out
}
