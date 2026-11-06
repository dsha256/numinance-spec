package provider_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
)

func archive(t *testing.T, entries ...tar.Header) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for _, h := range entries {
		h := h
		if err := w.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := w.Write(make([]byte, h.Size)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestExtract_RefusesWhatCouldReachOutside(t *testing.T) {
	file := func(name string) tar.Header {
		return tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}
	}
	tests := map[string][]tar.Header{
		"a parent segment":   {file("../x.py")},
		"an absolute name":   {file("/etc/x.py")},
		"a symlink":          {{Name: "l", Typeflag: tar.TypeSymlink, Linkname: "/etc"}},
		"a hard link":        {{Name: "l", Typeflag: tar.TypeLink, Linkname: "x"}},
		"a device":           {{Name: "d", Typeflag: tar.TypeChar}},
		"a repeated name":    {file("a.py"), file("a.py")},
		"a backslash":        {file(`a\b.py`)},
		"a file over limits": {{Name: "big.py", Typeflag: tar.TypeReg, Size: 17 << 20}},
	}
	for name, entries := range tests {
		dir := t.TempDir()
		var data []byte
		if name == "a file over limits" {
			// the header alone states the size: Extract refuses it before reading the body
			data = archive(t, tar.Header{Name: "big.py", Typeflag: tar.TypeReg, Size: 17 << 20})
		} else {
			data = archive(t, entries...)
		}
		if err := provider.Extract(data, dir); !errors.Is(err, provider.ErrCandidate) {
			t.Errorf("Extract(%s) = %v, want ErrCandidate", name, err)
		}
	}
}

func TestExtract_WritesFilesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	data := archive(t,
		tar.Header{Name: "shop/", Typeflag: tar.TypeDir, Mode: 0o755},
		tar.Header{Name: "shop/a.py", Typeflag: tar.TypeReg, Mode: 0o755, Size: 3},
	)
	if err := provider.Extract(data, dir); err != nil {
		t.Fatalf("Extract = %v, want nil", err)
	}
	st, err := os.Stat(filepath.Join(dir, "shop", "a.py"))
	if err != nil || st.Size() != 3 || st.Mode().Perm() != 0o600 {
		t.Errorf("stat = %v, %v, want a 3 byte file with mode 0600 (never executable)", st, err)
	}
}

func TestScanDirectives(t *testing.T) {
	got, err := provider.ScanDirectives("../testdata/candidates/suppressed", "mypy")
	if err != nil {
		t.Fatal(err)
	}
	want := []provider.Directive{{Name: "type-ignore", Path: "shop/pricing.py", Line: 5}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("ScanDirectives(mypy, suppressed) = %v, want %v", got, want)
	}
	if got, _ := provider.ScanDirectives("../testdata/candidates/suppressed", "ruff"); len(got) != 0 {
		t.Errorf("ScanDirectives(ruff) = %v, want none: ruff is run with --ignore-noqa", got)
	}
}

// rawEntry returns a tar block for name of the given typeflag, built through the writer and patched, because the writer refuses
// to encode the typeflags a hostile archive uses.
func rawEntry(t *testing.T, name string, typeflag byte, body string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	if err := w.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	blk := append([]byte(nil), b.Bytes()[:512]...)
	blk[156] = typeflag
	for i := 148; i < 156; i++ {
		blk[i] = ' '
	}
	sum := 0
	for _, c := range blk {
		sum += int(c)
	}
	copy(blk[148:], fmt.Sprintf("%06o\x00 ", sum))
	out := blk
	out = append(out, body...)
	out = append(out, make([]byte, (512-len(body)%512)%512)...)
	return out
}

func TestExtract_RefusesSparseEntriesInEveryForm(t *testing.T) {
	end := make([]byte, 1024)
	file := archive(t, tar.Header{Name: "a.py", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1})
	file = file[:len(file)-1024]
	tests := map[string][]byte{
		"a PAX record GNU.sparse.size":  append(append(rawEntry(t, "PaxHeader/a", 'x', paxRecord("GNU.sparse.size", "16777216")), file...), end...),
		"the GNU sparse typeflag":       append(rawEntry(t, "a.py", 'S', "x"), end...),
		"a PAX record GNU.sparse.major": append(append(rawEntry(t, "PaxHeader/a", 'x', paxRecord("GNU.sparse.major", "1")), file...), end...),
	}
	for name, data := range tests {
		if err := provider.Extract(data, t.TempDir()); !errors.Is(err, provider.ErrCandidate) {
			t.Errorf("Extract(%s) = %v, want ErrCandidate", name, err)
		}
	}
}

func TestExtract_RefusesMoreEntriesThanTheLimit(t *testing.T) {
	hs := make([]tar.Header, 0, 20001)
	for i := range 20001 {
		hs = append(hs, tar.Header{Name: fmt.Sprintf("d%d/", i), Typeflag: tar.TypeDir, Mode: 0o755})
	}
	if err := provider.Extract(archive(t, hs...), t.TempDir()); !errors.Is(err, provider.ErrCandidate) {
		t.Errorf("Extract(20001 entries) = %v, want ErrCandidate", err)
	}
}

func TestScanDirectives_ReportsWhatCouldHideAViolation(t *testing.T) {
	tests := map[string]struct{ check, file, body string }{
		"mypy file-level ignore-errors": {"mypy", "a.py", "# mypy: ignore-errors\nx: int = 'a'\n"},
		"mypy disable-error-code":       {"mypy", "a.py", "# mypy: disable-error-code=\"assignment\"\nx: int = 'a'\n"},
		"type: ignore in a stub":        {"mypy", "a.pyi", "x: int = 'a'  # type: ignore\n"},
		"a semgrepignore file":          {"semgrep", ".semgrepignore", "*\n"},
		"a gitignore file":              {"ruff", ".gitignore", "*\n"},
		"a dot ignore file":             {"ruff", ".ignore", "*\n"},
		"a report hook":                 {"pytest", "conftest.py", "import pytest\n@pytest.hookimpl(hookwrapper=True)\ndef pytest_runtest_makereport(item, call):\n    out = yield\n"},
		"a hook that is not allowed":    {"pytest", "conftest.py", "def pytest_collection_modifyitems(items):\n    items.clear()\n"},
		"a skip":                        {"pytest", "test_a.py", "import pytest\n@pytest.mark.skip\ndef test_x(): pass\n"},
	}
	for name, c := range tests {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, c.file), []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := provider.ScanDirectives(dir, c.check); err != nil || len(got) == 0 {
			t.Errorf("%s: ScanDirectives = %v, %v, want a directive", name, got, err)
		}
	}
	dir := t.TempDir()
	body := "def pytest_addoption(parser):\n    pass\n\n\ndef pytest_generate_tests(metafunc):\n    pass\n"
	if err := os.WriteFile(filepath.Join(dir, "conftest.py"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := provider.ScanDirectives(dir, "pytest"); len(got) != 0 {
		t.Errorf("ScanDirectives(allowed hooks) = %v, want none", got)
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, _ := provider.ScanDirectives(dir, "ruff"); len(got) != 1 || got[0].Path != ".git" {
		t.Errorf("ScanDirectives(.git) = %v, want the .git directory reported", got)
	}
}

// paxRecord is one PAX extended header record: its decimal length counts itself.
func paxRecord(k, v string) string {
	rest := " " + k + "=" + v + "\n"
	n := len(rest)
	for {
		rec := fmt.Sprintf("%d%s", n, rest)
		if len(rec) == n {
			return rec
		}
		n = len(rec)
	}
}
