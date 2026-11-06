package provider_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
	}{
		{"a config", `{"check":"ruff","settings":"x"}`, true},
		{"an unknown field", `{"check":"ruff","extra":1}`, false},
		{"not json", `check=ruff`, false},
		{"a negative timeout", `{"check":"ruff","timeout_seconds":-1}`, false},
		{"a timeout over the ceiling", `{"check":"ruff","timeout_seconds":601}`, false},
		{"over the size limit", `{"check":"ruff","settings":"` + strings.Repeat("a", 70<<10) + `"}`, false},
	}
	for _, tt := range tests {
		if _, err := provider.ParseConfig([]byte(tt.in)); (err == nil) != tt.ok {
			t.Errorf("%s: ParseConfig error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

func TestExecRunner_RefusesWhatItCannotRunSafely(t *testing.T) {
	r := provider.ExecRunner{Lookup: func(string) (string, error) { return "", errors.New("missing") }}
	for _, check := range []string{"pytest", "ruff", "unknown"} {
		_, err := r.Run(t.Context(), provider.Call{Config: provider.Config{Check: check}, Fetch: func(context.Context) ([]byte, error) { return nil, errors.New("must not fetch") }})
		if !errors.Is(err, provider.ErrUnavailable) {
			t.Errorf("Run(%s) = %v, want ErrUnavailable: pytest needs the nested sandbox, and a missing tool cannot start", check, err)
		}
	}
}

// tarOf archives the files of one candidate directory of testdata.
func tarOf(t *testing.T, dir string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		data, rerr := os.ReadFile(p) //nolint:gosec // a file of the testdata candidate being archived
		if rerr != nil {
			return rerr
		}
		if err := w.WriteHeader(&tar.Header{Name: filepath.ToSlash(rel), Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(data))}); err != nil {
			return err
		}
		_, werr := w.Write(data)
		return werr
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestExecRunner_RealRuff runs the real ruff on the seeded candidate. It is an integration test: it needs ruff on PATH and is
// skipped unless NUMINANCE_VERIFIER_TOOLS=1.
func TestExecRunner_RealRuff(t *testing.T) {
	if os.Getenv("NUMINANCE_VERIFIER_TOOLS") != "1" {
		t.Skip("set NUMINANCE_VERIFIER_TOOLS=1 to run the real tools")
	}
	if _, err := exec.LookPath("ruff"); err != nil {
		t.Skip("ruff is not installed")
	}
	cfg, err := os.ReadFile("../testdata/config/ruff.toml")
	if err != nil {
		t.Fatal(err)
	}
	out, err := provider.ExecRunner{}.Run(t.Context(), provider.Call{
		Config: provider.Config{Check: "ruff", Settings: string(cfg)},
		Fetch:  func(context.Context) ([]byte, error) { return tarOf(t, "../testdata/candidates/seeded"), nil },
	})
	if err != nil {
		t.Fatalf("Run(ruff) = %v, want output", err)
	}
	if out.ExitCode != 1 || !bytes.Contains(out.Stdout, []byte("F401")) || out.Replayed {
		t.Errorf("Run(ruff) = exit %d, %d bytes, replayed %v, want exit 1 with F401", out.ExitCode, len(out.Stdout), out.Replayed)
	}
}

// realTools skips the test unless the real tools are asked for and found on PATH.
func realTools(t *testing.T, names ...string) {
	t.Helper()
	if os.Getenv("NUMINANCE_VERIFIER_TOOLS") != "1" {
		t.Skip("set NUMINANCE_VERIFIER_TOOLS=1 to run the real tools")
	}
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("%s is not installed", n)
		}
	}
}

func runReal(t *testing.T, check, settings string, files map[string]string) (provider.Output, error) {
	t.Helper()
	data := pTarOf(t, files)
	return provider.ExecRunner{}.Run(t.Context(), provider.Call{Config: provider.Config{Check: check, Settings: settings}, Fetch: func(context.Context) ([]byte, error) { return data, nil }})
}

func pTarOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for n, c := range files {
		if err := w.WriteHeader(&tar.Header{Name: n, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(c))}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestExecRunner_ImportLinterAndPytestNeverRunCandidateCode(t *testing.T) {
	// The tool is not needed: the refusal comes before any lookup, so this runs everywhere.
	r := provider.ExecRunner{Lookup: func(string) (string, error) { t.Error("a tool was looked up"); return "", errors.New("x") }}
	for _, check := range []string{"import-linter", "pytest"} {
		_, err := r.Run(t.Context(), provider.Call{Config: provider.Config{Check: check}, Fetch: func(context.Context) ([]byte, error) {
			t.Error("the candidate was fetched")
			return nil, errors.New("x")
		}})
		if !errors.Is(err, provider.ErrUnavailable) {
			t.Errorf("Run(%s) = %v, want ErrUnavailable: it would run candidate code unnested", check, err)
		}
	}
}

func TestExecRunner_ACandidateWithNoPythonFileHasNothingChecked(t *testing.T) {
	realTools(t, "ruff")
	if _, err := runReal(t, "ruff", "", map[string]string{"README.md": "x"}); !errors.Is(err, provider.ErrCrashed) {
		t.Errorf("Run(no python file) = %v, want ErrCrashed: a check that looked at nothing is not a pass", err)
	}
}

// A candidate that ships .git/HEAD and a .gitignore that lists everything would hide every defect from ruff.
func TestExecRunner_RuffReadsNoIgnoreFileOfTheCandidate(t *testing.T) {
	realTools(t, "ruff")
	out, err := runReal(t, "ruff", "[lint]\nselect = [\"E\", \"F\"]\n", map[string]string{
		".git/HEAD": "ref: x\n", ".gitignore": "*\n", ".ignore": "*\n", "shop/a.py": "import os\n", "build/b.py": "import sys\n", "node_modules/c.py": "import re\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"shop/a.py", "build/b.py", "node_modules/c.py"} {
		if !bytes.Contains(out.Stdout, []byte(want)) {
			t.Errorf("ruff reported nothing for %s: it was hidden by an ignore file or a default exclude", want)
		}
	}
	names := map[string]bool{}
	for _, d := range out.Directives {
		names[d.Path] = true
	}
	if !names[".git"] || !names[".gitignore"] || !names[".ignore"] {
		t.Errorf("directives = %v, want .git, .gitignore and .ignore reported", out.Directives)
	}
}

// Semgrep skips a file by the candidate's .semgrepignore, by size and by its default skip list (tests/), and says nothing.
func TestExecRunner_SemgrepSkipsNothingSilently(t *testing.T) {
	realTools(t, "semgrep")
	rules := "rules:\n  - id: no-eval\n    languages: [python]\n    severity: ERROR\n    message: no eval\n    pattern: eval(...)\n"
	big := "# " + strings.Repeat("x", 1100000) + "\neval('1')\n"
	cases := map[string]map[string]string{
		"a semgrepignore": {".semgrepignore": "*\n", "a.py": "eval('1')\n"},
		"a big file":      {"a.py": big},
		"a tests dir":     {"tests/t.py": "eval('1')\n", "a.py": "x = 1\n"},
		"a gitignore":     {".gitignore": "*\n", "a.py": "eval('1')\n"},
	}
	for name, files := range cases {
		out, err := runReal(t, "semgrep", rules, files)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, perr := provider.ParseForTest("semgrep", out)
		if perr != nil || !strings.Contains(got, "no-eval") {
			t.Errorf("%s: semgrep did not report the eval(): %v, findings %q", name, perr, got)
		}
	}
}

// mypy honours a file-level `# mypy: ignore-errors`, which cannot be switched off: it is reported.
func TestExecRunner_MypyInlineConfigIsReported(t *testing.T) {
	realTools(t, "mypy")
	out, err := runReal(t, "mypy", "[mypy]\nstrict = True\n", map[string]string{"a.py": "# mypy: ignore-errors\nx: int = 'a'\n"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Directives) == 0 || out.Directives[0].Name != "mypy-inline-config" {
		t.Errorf("directives = %v, want the inline mypy config reported", out.Directives)
	}
}
