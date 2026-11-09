package cli_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
	"github.com/numinance/numinance-spec/sdk/go/extpkg"
)

func zipOf(t *testing.T, files ...extpkg.File) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, f := range files {
		h, err := w.Create(f.Path)
		if err != nil {
			t.Fatalf("Create(%s) = %v", f.Path, err)
		}
		if _, err := h.Write(f.Data); err != nil {
			t.Fatalf("Write(%s) = %v", f.Path, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	return b.Bytes()
}

// extRepo writes a store with one installed extension and a lockfile that pins it; it returns the lockfile and store paths.
func extRepo(t *testing.T) (lock, store string, files []extpkg.File) {
	t.Helper()
	dir := t.TempDir()
	files = []extpkg.File{{Path: "bin/provider", Data: []byte("elf")}, {Path: extpkg.ManifestFile, Data: []byte("plugin_id: com.example.scm\n")}}
	archive := zipOf(t, files...)
	store = filepath.Join(dir, "store")
	if err := os.MkdirAll(filepath.Join(store, "com.example.scm"), 0o700); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	if err := os.WriteFile(filepath.Join(store, "com.example.scm", "1.0.0.zip"), archive, 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	doc := fmt.Sprintf(`apiVersion: numinance.dev/v1
kind: Lockfile
metadata: {name: numinance-lock, scope: SCOPE_REPO}
spec:
  extensions:
    - id: com.example.scm
      version: 1.0.0
      digest: {value: %q}
      h1: %q
      zh: %q
      signer: "https://issuer.example#subject"
      permissions_digest: {value: "sha256:7c4e2a1b9d8f6e5c3a2b1d0f9e8c7b6a5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a"}
`, extpkg.ContentDigest(files), extpkg.TreeHash(files), extpkg.ArchiveHash(archive))
	lock = filepath.Join(dir, ".numinance.lock.yaml")
	if err := os.WriteFile(lock, []byte(doc), 0o600); err != nil {
		t.Fatalf("WriteFile(lock) = %v", err)
	}
	return lock, store, files
}

func runExt(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := cli.RunExtension(context.Background(), args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunExtension_Verify_Ok(t *testing.T) {
	t.Parallel()
	lock, store, _ := extRepo(t)
	code, out, errb := runExt("verify", "--lockfile", lock, "--store", store)
	if code != cli.ExitOK || !strings.Contains(out, "1 verified, 0 failed") {
		t.Errorf("verify = %d, %q, %q; want ok with one verified", code, out, errb)
	}
}

func TestRunExtension_Verify_Json(t *testing.T) {
	t.Parallel()
	lock, store, _ := extRepo(t)
	code, out, _ := runExt("verify", "--lockfile", lock, "--store", store, "--json")
	var got struct {
		OK         bool `json:"ok"`
		Extensions []struct {
			ID string `json:"id"`
			OK bool   `json:"ok"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != cli.ExitOK || !got.OK || len(got.Extensions) != 1 || got.Extensions[0].ID != "com.example.scm" {
		t.Errorf("verify --json = %d, %q (%v); want ok with the one extension", code, out, err)
	}
}

func TestRunExtension_Verify_Detects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		tamper func(t *testing.T, store string, files []extpkg.File)
		want   string
	}{
		{name: "binary replaced", tamper: func(t *testing.T, store string, files []extpkg.File) {
			t.Helper()
			evil := []extpkg.File{{Path: "bin/provider", Data: []byte("evil")}, files[1]}
			if err := os.WriteFile(filepath.Join(store, "com.example.scm", "1.0.0.zip"), zipOf(t, evil...), 0o600); err != nil {
				t.Fatalf("WriteFile() = %v", err)
			}
		}, want: "zh, h1, digest"},
		{name: "manifest replaced only", tamper: func(t *testing.T, store string, files []extpkg.File) {
			t.Helper()
			other := []extpkg.File{files[0], {Path: extpkg.ManifestFile, Data: []byte("plugin_id: com.example.evil\n")}}
			if err := os.WriteFile(filepath.Join(store, "com.example.scm", "1.0.0.zip"), zipOf(t, other...), 0o600); err != nil {
				t.Fatalf("WriteFile() = %v", err)
			}
		}, want: "zh, h1"},
		{name: "archive missing", tamper: func(t *testing.T, store string, _ []extpkg.File) {
			t.Helper()
			if err := os.Remove(filepath.Join(store, "com.example.scm", "1.0.0.zip")); err != nil {
				t.Fatalf("Remove() = %v", err)
			}
		}, want: "missing or unreadable"},
		{name: "archive not a zip", tamper: func(t *testing.T, store string, _ []extpkg.File) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(store, "com.example.scm", "1.0.0.zip"), []byte("junk"), 0o600); err != nil {
				t.Fatalf("WriteFile() = %v", err)
			}
		}, want: "not a valid package"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lock, store, files := extRepo(t)
			tt.tamper(t, store, files)
			code, out, _ := runExt("verify", "--lockfile", lock, "--store", store)
			if code != cli.ExitConflict || !strings.Contains(out, "FAILED") || !strings.Contains(out, tt.want) {
				t.Errorf("verify(%s) = %d, %q; want a conflict naming %q", tt.name, code, out, tt.want)
			}
		})
	}
}

func TestRunExtension_Verify_Refuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile() = %v", err)
		}
		return p
	}
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "no subcommand", args: nil, want: cli.ExitUsage},
		{name: "unknown subcommand", args: []string{"add"}, want: cli.ExitUsage},
		{name: "unknown flag", args: []string{"verify", "--nope"}, want: cli.ExitUsage},
		{name: "extra argument", args: []string{"verify", "x"}, want: cli.ExitUsage},
		{name: "missing lockfile", args: []string{"verify", "--lockfile", filepath.Join(dir, "absent.yaml")}, want: cli.ExitNotFound},
		{name: "invalid lockfile", args: []string{"verify", "--lockfile", write("bad.yaml", "kind: Lockfile\n")}, want: cli.ExitInvalid},
		{name: "another kind", args: []string{"verify", "--lockfile", write("other.yaml", "apiVersion: numinance.dev/v1\nkind: BudgetPolicy\nmetadata: {name: b, scope: SCOPE_ORG}\nspec: {}\n")}, want: cli.ExitInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if code, out, errb := runExt(tt.args...); code != tt.want {
				t.Errorf("RunExtension(%v) = %d, %q, %q; want %d", tt.args, code, out, errb, tt.want)
			}
		})
	}
}

func TestRoot_Extension(t *testing.T) {
	t.Parallel()
	var errb bytes.Buffer
	if code := (cli.Root{Stderr: &errb}).Run(context.Background(), []string{"extension", "verify"}); code != cli.ExitUsage {
		t.Errorf("Root without Extension: extension verify = %d, want usage (the command is unknown)", code)
	}
	var got []string
	r := cli.Root{Extension: func(_ context.Context, args []string, _, _ io.Writer) int { got = args; return 42 }}
	if code := r.Run(context.Background(), []string{"extension", "verify", "--json"}); code != 42 || len(got) != 2 || got[0] != "verify" {
		t.Errorf("Root with Extension: = %d with args %v, want the handler's code and the arguments after extension", code, got)
	}
}
