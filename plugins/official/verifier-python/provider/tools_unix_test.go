//go:build unix

package provider_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
)

// fakeTool installs a script as the "ruff" the runner finds, so the tests of how a tool is run need no real tool.
func fakeTool(t *testing.T, script string) provider.ExecRunner {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ruff")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script), 0o700); err != nil { //nolint:gosec // a test script, executable by design
		t.Fatal(err)
	}
	return provider.ExecRunner{Lookup: func(string) (string, error) { return p, nil }}
}

func callOf(t *testing.T, timeout int) provider.Call {
	t.Helper()
	data := pTarOf(t, map[string]string{"shop/a.py": "x = 1\n"})
	return provider.Call{Config: provider.Config{Check: "ruff", TimeoutSeconds: timeout}, Fetch: func(context.Context) ([]byte, error) { return data, nil }}
}

// The tool runs outside the candidate, with the python path isolated: nothing of the candidate is its current directory or on its
// import path.
func TestExecRunner_TheToolRunsOutsideTheCandidateWithAnIsolatedPythonPath(t *testing.T) {
	report := filepath.Join(t.TempDir(), "env.txt")
	r := fakeTool(t, "{ pwd -P; echo \"SAFE=$PYTHONSAFEPATH\"; echo \"PYPATH=$PYTHONPATH\"; echo \"ARGS=$*\"; } > "+report+"\necho '[]'\n")
	out, err := r.Run(t.Context(), callOf(t, 0))
	if err != nil {
		t.Fatalf("Run = %v, want output", err)
	}
	b, err := os.ReadFile(report) //nolint:gosec // the test's own report file
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	lines := strings.Split(got, "\n")
	cwd := lines[0]
	if strings.HasPrefix(cwd, out.Dir) || out.Dir == "" {
		t.Errorf("cwd = %q inside the candidate %q, want a directory outside it", cwd, out.Dir)
	}
	for _, want := range []string{"SAFE=1", "PYPATH=\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("tool environment = %q, want %q", got, want)
		}
	}
	for _, want := range []string{"--no-respect-gitignore", "--no-force-exclude", "exclude = []", "extend-exclude = []", out.Dir} {
		if !strings.Contains(got, want) {
			t.Errorf("tool arguments = %q, want %q", got, want)
		}
	}
}

// At the deadline the whole process group dies: a child the tool started does not outlive the call.
func TestExecRunner_ADeadlineKillsTheWholeProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	r := fakeTool(t, "sleep 60 &\necho $! > "+pidFile+"\nwait\n")
	start := time.Now()
	_, err := r.Run(t.Context(), callOf(t, 1))
	if !errors.Is(err, provider.ErrCrashed) {
		t.Fatalf("Run = %v, want ErrCrashed at the deadline", err)
	}
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("Run took %v, want it to end shortly after its 1 s deadline", d)
	}
	b, err := os.ReadFile(pidFile) //nolint:gosec // the test's own pid file
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	for range 50 {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("the tool's child %d is still running after the call ended", pid)
}

// A plugin host starts the provider with no PATH. A tool installed for the operator alone (uv tool install) is then found only
// through the directories the operator named at launch, and it runs with those directories as its PATH.
func TestExecRunner_FindsAToolInANamedDirectoryWhenNoPathHasIt(t *testing.T) {
	t.Setenv("PATH", "")
	dir, report := t.TempDir(), filepath.Join(t.TempDir(), "path.txt")
	script := "#!/bin/sh\necho \"PATH=$PATH\" > " + report + "\necho '[]'\n"
	if err := os.WriteFile(filepath.Join(dir, "ruff"), []byte(script), 0o700); err != nil { //nolint:gosec // a test script, executable by design
		t.Fatal(err)
	}
	r := provider.ExecRunner{Dirs: []string{"relative/dir", dir}}
	if _, err := r.Run(t.Context(), callOf(t, 0)); err != nil {
		t.Fatalf("Run with the tool in a named directory = %v, want output", err)
	}
	b, err := os.ReadFile(report) //nolint:gosec // the test's own report file
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != "PATH="+dir {
		t.Errorf("the tool's PATH = %q, want only the named absolute directory %q", got, "PATH="+dir)
	}
}

func TestExecRunner_NoToolInAnyNamedDirectoryIsUnavailable(t *testing.T) {
	t.Setenv("PATH", "")
	notExec := t.TempDir()
	if err := os.WriteFile(filepath.Join(notExec, "ruff"), []byte("#!/bin/sh\necho '[]'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, dirs := range map[string][]string{"none": nil, "a relative directory": {"relative"}, "a file without an execute bit": {notExec}} {
		r := provider.ExecRunner{Dirs: dirs}
		if _, err := r.Run(t.Context(), callOf(t, 0)); !errors.Is(err, provider.ErrUnavailable) {
			t.Errorf("Run with %s = %v, want ErrUnavailable", name, err)
		}
	}
}
