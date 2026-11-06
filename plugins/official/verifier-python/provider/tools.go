package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The checks of the provider.
const (
	checkRuff         = "ruff"
	checkMypy         = "mypy"
	checkPytest       = "pytest"
	checkSemgrep      = "semgrep"
	checkImportLinter = "import-linter"
)

// Limits of one call: what the provider reads and runs is untrusted and bounded before use.
const (
	maxConfigBytes    = 64 << 10
	maxSettingsBytes  = 32 << 10
	maxCandidateBytes = 64 << 20
	maxOutputBytes    = 8 << 20
	defaultTimeout    = 60 * time.Second
	maxTimeout        = 10 * time.Minute
)

// Errors the Run handler maps to a status of the port.
var (
	// ErrUnavailable: the check cannot start (unknown check, tool not installed, nested sandbox cannot be built).
	ErrUnavailable = errors.New("the check cannot start")
	// ErrCrashed: the check crashed, timed out or cannot complete.
	ErrCrashed = errors.New("the check did not complete")
	// ErrTooLarge: the tool output is over the limit; it is never truncated to continue.
	ErrTooLarge = errors.New("the tool output is over the limit")
)

// Config is the configuration a config_ref names: the only configuration the checks honour. Settings is the text of the
// tool's own settings file (ruff.toml, mypy.ini, pytest.ini, a semgrep rules file, an import-linter contract file); the
// provider writes it to a private directory and passes it to the tool, so no settings file of the candidate is read.
type Config struct {
	Check          string `json:"check"`
	Settings       string `json:"settings"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// ParseConfig decodes and validates config bytes: strict fields, bounded sizes, a known check.
func ParseConfig(b []byte) (Config, error) {
	var c Config
	if len(b) > maxConfigBytes {
		return c, fmt.Errorf("config is %d bytes, the limit is %d", len(b), maxConfigBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	if len(c.Settings) > maxSettingsBytes || c.TimeoutSeconds < 0 || time.Duration(c.TimeoutSeconds)*time.Second > maxTimeout {
		return c, errors.New("config: settings or timeout out of bounds")
	}
	return c, nil
}

// Timeout is the bound of the tool run.
func (c Config) Timeout() time.Duration {
	if c.TimeoutSeconds == 0 {
		return defaultTimeout
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// Known reports whether the provider has this check.
func Known(check string) bool {
	return slices.Contains([]string{checkRuff, checkMypy, checkPytest, checkSemgrep, checkImportLinter}, check)
}

// Directive is a suppression directive or test hook found in the candidate that the tool cannot be told to ignore.
type Directive struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// Call is one tool run as the Run handler asks the Runner for it.
type Call struct {
	Config Config
	// CandidateDigest is the sha256 digest ("sha256:...") of the candidate artifact the host verified.
	CandidateDigest string
	// Fetch returns the candidate artifact bytes; a replaying Runner never calls it.
	Fetch func(context.Context) ([]byte, error)
}

// Output is what a tool run produced.
type Output struct {
	// Stdout is the report the tool wrote (the JUnit file for pytest), at most maxOutputBytes.
	Stdout []byte
	// ExitCode is the tool's exit status.
	ExitCode int
	// Directives are the suppression directives of the candidate the tool cannot ignore.
	Directives []Directive
	// Dir is the directory the tool ran in; the parser strips it from paths. Empty for a replay (paths are /candidate).
	Dir string
	// Replayed is true when a recorded output stood in for the tool.
	Replayed bool
}

// Runner runs a check on a candidate. The exec Runner runs the real tool; the replay Runner (package replay) serves recorded
// output for the seeded-defect tests.
type Runner interface {
	Run(ctx context.Context, c Call) (Output, error)
}

// ExecRunner runs the real tools installed on the host. Every tool runs in a private temporary directory the provider
// creates and removes, with no environment but PATH and a private HOME, a timeout and a bounded output.
//
// Candidate code is not run unnested. Two checks execute it: pytest runs the tests, and import-linter imports the root package of
// the candidate to find its modules, which runs the package's __init__ files in the verifier's own process. The port allows that only
// in a nested sandbox (a separate uid and its own namespaces), which this provider does not build, so both are UNAVAILABLE here and
// only the replaying build serves them. The three static checks (ruff, mypy, Semgrep) run from a working directory outside the
// candidate, with the tree passed by path, no PYTHONPATH and PYTHONSAFEPATH set, so no file of the candidate is on the tool's
// import path; each runs in its own process group that is killed whole at the deadline.
type ExecRunner struct {
	// Lookup finds a tool binary; it is exec.LookPath when nil.
	Lookup func(string) (string, error)
}

// Run implements Runner.
func (r ExecRunner) Run(ctx context.Context, c Call) (Output, error) {
	if c.Config.Check == checkPytest || c.Config.Check == checkImportLinter {
		return Output{}, fmt.Errorf("%w: the nested sandbox for candidate code is not built by this provider", ErrUnavailable)
	}
	argv, err := r.command(c.Config.Check)
	if err != nil {
		return Output{}, err
	}
	work, err := os.MkdirTemp("", "verifier")
	if err != nil {
		return Output{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = os.RemoveAll(work) }() // a private temporary directory
	tree, cfgDir, home := filepath.Join(work, "candidate"), filepath.Join(work, "config"), filepath.Join(work, "home")
	for _, d := range []string{tree, cfgDir, home} {
		if err := os.Mkdir(d, 0o700); err != nil {
			return Output{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
	}
	if dir, derr := filepath.EvalSymlinks(tree); derr == nil {
		tree = dir
	}
	data, err := c.Fetch(ctx)
	if err != nil {
		return Output{}, err
	}
	if err := Extract(data, tree); err != nil {
		return Output{}, err
	}
	if n, err := countPython(tree); err != nil || n == 0 {
		return Output{}, fmt.Errorf("%w: the candidate holds no Python file to check", ErrCrashed)
	}
	cfg := filepath.Join(cfgDir, settingsName(c.Config.Check))
	if err := os.WriteFile(cfg, []byte(c.Config.Settings), 0o600); err != nil {
		return Output{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	args := toolArgs(c.Config.Check, cfg, tree)
	ctx, cancel := context.WithTimeout(ctx, c.Config.Timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], append(argv[1:], args...)...) //nolint:gosec // the binary is one of five fixed tool names found on PATH
	// The working directory is the private config directory: nothing of the candidate is the tool's current directory.
	cmd.Dir = cfgDir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "LANG=C.UTF-8", "PYTHONDONTWRITEBYTECODE=1", "PYTHONNOUSERSITE=1", "PYTHONSAFEPATH=1"}
	if c.Config.Check == checkMypy {
		cmd.Env = append(cmd.Env, "MYPYPATH="+tree)
	}
	isolate(cmd)
	var stdout limitWriter
	stdout.max = maxOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	runErr := cmd.Run()
	if stdout.over {
		return Output{}, ErrTooLarge
	}
	if ctx.Err() != nil {
		return Output{}, fmt.Errorf("%w: %w", ErrCrashed, ctx.Err())
	}
	code := 0
	var ee *exec.ExitError
	switch {
	case errors.As(runErr, &ee):
		code = ee.ExitCode()
	case runErr != nil:
		return Output{}, fmt.Errorf("%w: %w", ErrCrashed, runErr)
	}
	dirs, err := ScanDirectives(tree, c.Config.Check)
	if err != nil {
		return Output{}, fmt.Errorf("%w: %w", ErrCrashed, err)
	}
	return Output{Stdout: stdout.buf.Bytes(), ExitCode: code, Directives: dirs, Dir: tree}, nil
}

// command returns the executable and fixed leading arguments of a check, or ErrUnavailable.
func (r ExecRunner) command(check string) ([]string, error) {
	look := r.Lookup
	if look == nil {
		look = exec.LookPath
	}
	var name string
	var lead []string
	switch check {
	case checkRuff:
		name, lead = "ruff", []string{"check"}
	case checkMypy:
		name = "mypy"
	case checkSemgrep:
		name, lead = "semgrep", []string{"scan"}
	case checkImportLinter:
		name = "lint-imports"
	default:
		return nil, fmt.Errorf("%w: no such check", ErrUnavailable)
	}
	path, err := look(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s is not installed", ErrUnavailable, name)
	}
	return append([]string{path}, lead...), nil
}

func settingsName(check string) string {
	switch check {
	case checkRuff:
		return "ruff.toml"
	case checkMypy:
		return "mypy.ini"
	case checkSemgrep:
		return "rules.yml"
	case checkImportLinter:
		return "importlinter.ini"
	}
	return "settings"
}

// toolArgs are the arguments after the fixed ones: the settings file the provider wrote, machine-readable output, and the
// flags that make the tool ignore the suppression directives of the candidate where it has one.
func toolArgs(check, cfg, tree string) []string {
	switch check {
	case checkRuff:
		// No exclusion of the candidate's choosing and none of ruff's defaults: .gitignore is not read, the default exclude list is
		// emptied, and a path given on the command line is not dropped by an exclude.
		return []string{"--config", cfg, "--config", "exclude = []", "--config", "extend-exclude = []", "--no-respect-gitignore", "--no-force-exclude", "--output-format", "json", "--no-cache", "--ignore-noqa", tree}
	case checkMypy:
		return []string{"--config-file", cfg, "--output", "json", "--no-incremental", "--cache-dir=" + os.DevNull, tree}
	case checkSemgrep:
		// Neither .gitignore nor .semgrepignore, and no default skip list, hides a file; a file over the extraction limit is the
		// only one skipped, and a skip is a failure (parseSemgrep).
		return []string{"--config", cfg, "--json", "--disable-nosem", "--metrics=off", "--disable-version-check", "--quiet", "--no-git-ignore", "--x-ignore-semgrepignore-files", "--max-target-bytes", strconv.Itoa(maxFileSize), tree}
	}
	return nil
}

// countPython counts the .py and .pyi files of the tree: a check that has nothing to look at has verified nothing.
func countPython(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".py") || strings.HasSuffix(p, ".pyi")) {
			n++
		}
		return err
	})
	return n, err
}

// limitWriter buffers up to max bytes and records that more was written; it never truncates silently.
type limitWriter struct {
	buf  bytes.Buffer
	max  int
	over bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.max {
		w.over = true
		return 0, errors.New("output over the limit")
	}
	return w.buf.Write(p)
}
