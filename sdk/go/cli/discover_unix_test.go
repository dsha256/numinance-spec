//go:build unix

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

// TestDiscover_Run_SpecialFilesAreNotOpened puts a FIFO where discover looks for files; opening one would block.
func TestDiscover_Run_SpecialFilesAreNotOpened(t *testing.T) {
	repo := t.TempDir()
	for _, name := range []string{"pyproject.toml", "uv.lock", "CODEOWNERS", ".python-version", ".pre-commit-config.yaml", ".git/HEAD"} {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(p, 0o600); err != nil {
			t.Skipf("fifo unavailable: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Join(repo, ".github", "workflows"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(repo, ".github", "workflows", "ci.yml"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := discoverRun("--out", outDir(t), repo)
	if code != cli.ExitOK {
		t.Fatalf("discover = %d, stderr %q, want 0", code, stderr)
	}
	for _, want := range []string{"self-test: HEAD not resolved (no readable .git/HEAD", "artifact: CODEOWNERS: none found", "chose: package manager uv; confidence low"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "artifact: workflow") {
		t.Errorf("a FIFO was listed as a workflow:\n%s", stdout)
	}
}
