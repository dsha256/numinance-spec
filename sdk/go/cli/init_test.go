package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

func runInit(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = cli.RunInit(context.Background(), args, &out, &errb)
	return code, out.String(), errb.String()
}

// readFiles returns every file below dir by slash path, directories excluded.
func readFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // walks t.TempDir
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// golden renders the files of the trees as one reviewable text: a header per file, in path order.
func golden(trees ...map[string]string) string {
	var b strings.Builder
	for i, files := range trees {
		if i > 0 {
			b.WriteString("##### organization policy repository\n")
		}
		for _, name := range slices.Sorted(func(yield func(string) bool) {
			for k := range files {
				if !yield(k) {
					return
				}
			}
		}) {
			b.WriteString("=== " + name + " ===\n" + files[name])
		}
	}
	return b.String()
}

func TestInit_Run_GoldenTemplates(t *testing.T) {
	tests := []struct {
		name string
		args []string
		org  bool
	}{
		{name: "python_github-actions", args: []string{"--ci", "github-actions"}},
		{name: "python_jenkins", args: []string{"--ci", "jenkins"}},
		{name: "python_both", args: []string{"--ci", "both"}},
		{name: "python_tickets-file", args: []string{"--ci", "github-actions", "--language", "python", "--scm", "github", "--tickets", "file"}},
		{name: "python_github-actions_org", args: []string{"--ci", "github-actions"}, org: true},
		{name: "python_override", args: []string{"--ci", "both", "--package-manager", "poetry", "--python-version", "3.11", "--verifiers", "lint,typecheck"}, org: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, org := t.TempDir(), t.TempDir()
			args := tc.args
			if tc.org {
				args = append(slices.Clone(args), "--org-dir", org)
			}
			if code, _, errOut := runInit(append(args, repo)...); code != cli.ExitOK {
				t.Fatalf("Run(%v) = %d, stderr %q, want 0", args, code, errOut)
			}
			trees := []map[string]string{readFiles(t, repo)}
			if tc.org {
				trees = append(trees, readFiles(t, org))
			}
			got := golden(trees...)
			path := filepath.Join("testdata", "init_"+tc.name+".golden")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path) //nolint:gosec // constant testdata names
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("generated files differ from %s (run with -update and review the diff):\n%s", path, got)
			}
		})
	}
}

// dirSource is a Source that reads the .numinance YAML files of directories on disk, like the open build's source.
type dirSource struct{ repo, org string }

func (s dirSource) Snapshot(_ context.Context, _ cli.Request) (cli.Snapshot, error) {
	snap := cli.Snapshot{Revision: "test"}
	for _, t := range []struct{ scope, dir string }{{"org", s.org}, {"repo", s.repo}} {
		if t.dir == "" {
			continue
		}
		err := filepath.WalkDir(filepath.Join(t.dir, ".numinance"), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
				return err
			}
			b, err := os.ReadFile(p) //nolint:gosec // walks t.TempDir
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(t.dir, p)
			snap.Documents = append(snap.Documents, cli.Document{Scope: t.scope, Path: filepath.ToSlash(rel), Data: b})
			return err
		})
		if err != nil {
			return cli.Snapshot{}, err
		}
	}
	return snap, nil
}

// TestInit_Run_OutputValidatesAgainstTheSchemas runs the same code "numi validate" uses over what init wrote. Without
// a resolver that code exits 7 after the schema pass (the open build's answer); --files exits 0 on the schema pass alone.
func TestInit_Run_OutputValidatesAgainstTheSchemas(t *testing.T) {
	for _, ci := range []string{"github-actions", "jenkins", "both"} {
		for _, withOrg := range []bool{false, true} {
			t.Run(ci+map[bool]string{false: "", true: " with org"}[withOrg], func(t *testing.T) {
				repo, org := t.TempDir(), t.TempDir()
				args := []string{"--ci", ci, repo}
				if withOrg {
					args = []string{"--ci", ci, "--org-dir", org, repo}
				} else {
					org = ""
				}
				if code, _, errOut := runInit(args...); code != cli.ExitOK {
					t.Fatalf("init = %d, stderr %q, want 0", code, errOut)
				}
				src := dirSource{repo: repo, org: org}
				var out, errb bytes.Buffer
				v := cli.Validate{Source: src, Files: src, Stdout: &out, Stderr: &errb}
				if code := v.Run(context.Background(), []string{"--json", repo}); code != cli.ExitUnavailable {
					t.Errorf("validate = %d, want %d (schema pass, no resolver); stdout %s", code, cli.ExitUnavailable, out.String())
				}
				var rep struct {
					Documents []struct{ Path, Kind, Status string }
				}
				if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
					t.Fatalf("validate output is not JSON: %v\n%s", err, out.String())
				}
				if len(rep.Documents) == 0 {
					t.Fatalf("validate saw no document")
				}
				for _, d := range rep.Documents {
					if d.Status != "ok" {
						t.Errorf("document %s (%s) status = %q, want ok", d.Path, d.Kind, d.Status)
					}
				}
				out.Reset()
				if code := v.Run(context.Background(), []string{"--files", repo}); code != cli.ExitOK {
					t.Errorf("validate --files = %d, want 0; stdout %s", code, out.String())
				}
			})
		}
	}
}

func TestInit_Run_FlagErrorsExitUsage(t *testing.T) {
	repo := t.TempDir()
	conflicted := t.TempDir()
	write(t, conflicted, "uv.lock", "")
	write(t, conflicted, "poetry.lock", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no ci", args: []string{repo}, want: "--ci is required"},
		{name: "unknown ci", args: []string{"--ci", "circleci", repo}, want: "--ci"},
		{name: "unknown package manager", args: []string{"--ci", "both", "--package-manager", "conda", repo}, want: "--package-manager"},
		{name: "python 2", args: []string{"--ci", "both", "--python-version", "2.7", repo}, want: "--python-version"},
		{name: "patch version", args: []string{"--ci", "both", "--python-version", "3.12.1", repo}, want: "--python-version"},
		{name: "unknown verifier", args: []string{"--ci", "both", "--verifiers", "lint,security", repo}, want: "--verifiers"},
		{name: "repeated verifier", args: []string{"--ci", "both", "--verifiers", "lint,lint", repo}, want: "--verifiers repeats"},
		{name: "empty verifier name", args: []string{"--ci", "both", "--verifiers", "lint,", repo}, want: "--verifiers"},
		{name: "language go", args: []string{"--ci", "both", "--language", "go", repo}, want: "--language go: only python has a toolchain template"},
		{name: "language empty", args: []string{"--ci", "both", "--language", "", repo}, want: "--language : only python has a toolchain template"},
		{name: "scm gitlab", args: []string{"--ci", "both", "--scm", "gitlab", repo}, want: "--scm gitlab: only github has a provider"},
		{name: "tickets jira", args: []string{"--ci", "both", "--tickets", "jira", repo}, want: "--tickets jira: no ticketing provider exists yet; use file"},
		{name: "tickets none", args: []string{"--ci", "both", "--tickets", "none", repo}, want: "--tickets none: no ticketing provider exists yet; use file"},
		{name: "two paths", args: []string{"--ci", "both", repo, repo}, want: "at most one path"},
		{name: "unknown flag", args: []string{"--ci", "both", "--bogus", repo}, want: "bogus"},
		{name: "ci value missing", args: []string{repo, "--ci"}, want: "--ci"},
		{name: "conflicting hints", args: []string{"--ci", "both", conflicted}, want: "conflicting package manager hints"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := runInit(tc.args...)
			if code != cli.ExitUsage {
				t.Errorf("Run(%v) = %d, want %d", tc.args, code, cli.ExitUsage)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("Run(%v) stderr = %q, want it to contain %q", tc.args, errOut, tc.want)
			}
			if out != "" {
				t.Errorf("Run(%v) stdout = %q, want none on a usage error", tc.args, out)
			}
		})
	}
	if files := readFiles(t, repo); len(files) != 0 {
		t.Errorf("a usage error wrote %v, want nothing", files)
	}
}

func TestInit_Run_ConflictingHintsAreResolvedByTheFlag(t *testing.T) {
	repo := t.TempDir()
	write(t, repo, "uv.lock", "")
	write(t, repo, "poetry.lock", "")
	code, out, errOut := runInit("--ci", "github-actions", "--package-manager", "poetry", repo)
	if code != cli.ExitOK {
		t.Fatalf("init = %d, stderr %q, want 0", code, errOut)
	}
	if !strings.Contains(out, "package manager poetry (flag)") {
		t.Errorf("stdout = %q, want the flag reported as the source", out)
	}
}

func TestInit_Run_HelpExitsOK(t *testing.T) {
	code, out, _ := runInit("--help")
	if code != cli.ExitOK || !strings.Contains(out, "usage: numi init") {
		t.Errorf("Run(--help) = %d, stdout %q, want 0 and the usage", code, out)
	}
}

func TestInit_Run_MissingDirectories(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "afile")
	write(t, dir, "afile", "x")
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "missing repository", args: []string{"--ci", "both", filepath.Join(dir, "missing")}, want: cli.ExitNotFound},
		{name: "repository is a file", args: []string{"--ci", "both", file}, want: cli.ExitInvalid},
		{name: "missing org directory", args: []string{"--ci", "both", "--org-dir", filepath.Join(dir, "missing"), t.TempDir()}, want: cli.ExitNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, errOut := runInit(tc.args...); code != tc.want {
				t.Errorf("Run(%v) = %d, stderr %q, want %d", tc.args, code, errOut, tc.want)
			}
		})
	}
}

func TestInit_Run_DetectsTheToolchainFromTheRepository(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{name: "nothing", want: []string{"package manager uv (default, no hint in the directory)", "python 3.12 (default)"}},
		{name: "uv lock", files: map[string]string{"uv.lock": ""}, want: []string{"package manager uv (detected from uv.lock)"}},
		{name: "poetry lock", files: map[string]string{"poetry.lock": ""}, want: []string{"package manager poetry (detected from poetry.lock)"}},
		{name: "poetry table", files: map[string]string{"pyproject.toml": "[tool.poetry]\nname = \"x\"\n"}, want: []string{"package manager poetry (detected from pyproject.toml [tool.poetry])"}},
		{name: "uv table", files: map[string]string{"pyproject.toml": "[tool.uv]\n"}, want: []string{"package manager uv (detected from pyproject.toml [tool.uv])"}},
		{name: "requirements", files: map[string]string{"requirements.txt": "pytest\n"}, want: []string{"package manager pip (detected from requirements.txt)"}},
		{name: "pyproject alone", files: map[string]string{"pyproject.toml": "[project]\nname = \"x\"\n"}, want: []string{"package manager uv (detected from pyproject.toml with no package manager table, the default)"}},
		{name: "lock beside its table", files: map[string]string{"poetry.lock": "", "pyproject.toml": "[tool.poetry]\n"}, want: []string{"package manager poetry (detected from poetry.lock, pyproject.toml [tool.poetry])"}},
		{name: "lock beats requirements", files: map[string]string{"uv.lock": "", "requirements.txt": ""}, want: []string{"package manager uv (detected from uv.lock)"}},
		{name: "python version file", files: map[string]string{".python-version": "3.11.4\n"}, want: []string{"python 3.11 (detected from .python-version)"}},
		{name: "python version not understood", files: map[string]string{".python-version": "pypy3.10\n"}, want: []string{"python 3.12 (default)"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			for name, content := range tc.files {
				write(t, repo, name, content)
			}
			code, out, errOut := runInit("--ci", "github-actions", repo)
			if code != cli.ExitOK {
				t.Fatalf("init = %d, stderr %q, want 0", code, errOut)
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("stdout lacks %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestInit_Run_DetectedValuesLandInTheToolchain(t *testing.T) {
	repo := t.TempDir()
	write(t, repo, "poetry.lock", "")
	write(t, repo, ".python-version", "3.10\n")
	if code, _, errOut := runInit("--ci", "github-actions", repo); code != cli.ExitOK {
		t.Fatalf("init = %d, stderr %q, want 0", code, errOut)
	}
	tc := readFiles(t, repo)[".numinance/policy/toolchain-python.yaml"]
	for _, want := range []string{"name: python-3.10", "version: \"3.10\"", "package_manager: poetry"} {
		if !strings.Contains(tc, want) {
			t.Errorf("toolchain lacks %q:\n%s", want, tc)
		}
	}
}

func TestInit_Run_ConflictAndForce(t *testing.T) {
	repo := t.TempDir()
	if code, _, errOut := runInit("--ci", "github-actions", repo); code != cli.ExitOK {
		t.Fatalf("first init = %d, stderr %q, want 0", code, errOut)
	}
	first := readFiles(t, repo)

	t.Run("a second run refuses and changes nothing", func(t *testing.T) {
		code, _, errOut := runInit("--ci", "jenkins", repo)
		if code != cli.ExitConflict {
			t.Errorf("second init = %d, stderr %q, want %d", code, errOut, cli.ExitConflict)
		}
		if got := readFiles(t, repo); !equalFiles(got, first) {
			t.Errorf("a refused run changed the files")
		}
	})
	t.Run("force overwrites its own files and keeps the others", func(t *testing.T) {
		write(t, repo, ".numinance/policy/mine.yaml", "mine\n")
		write(t, repo, ".numinance/policy/autonomy.yaml", "edited\n")
		code, _, errOut := runInit("--ci", "jenkins", "--force", repo)
		if code != cli.ExitOK {
			t.Fatalf("forced init = %d, stderr %q, want 0", code, errOut)
		}
		got := readFiles(t, repo)
		if got[".numinance/policy/mine.yaml"] != "mine\n" {
			t.Errorf("force touched a file it does not write")
		}
		if got[".numinance/policy/autonomy.yaml"] == "edited\n" {
			t.Errorf("force did not overwrite a file it writes")
		}
		if !strings.Contains(got[".numinance/policy/sdlc-profile.yaml"], "dev.numinance.official/jenkins") {
			t.Errorf("forced run did not write the jenkins profile:\n%s", got[".numinance/policy/sdlc-profile.yaml"])
		}
	})
	t.Run("an unrelated file counts as existing files", func(t *testing.T) {
		other := t.TempDir()
		write(t, other, ".numinance/notes.txt", "x")
		if code, _, _ := runInit("--ci", "both", other); code != cli.ExitConflict {
			t.Errorf("init over an unrelated file = %d, want %d", code, cli.ExitConflict)
		}
		if _, err := os.Stat(filepath.Join(other, ".numinance", "policy")); err == nil {
			t.Errorf("a refused run created policy/")
		}
	})
	t.Run("an empty config directory is fine", func(t *testing.T) {
		other := t.TempDir()
		if err := os.Mkdir(filepath.Join(other, ".numinance"), 0o750); err != nil {
			t.Fatal(err)
		}
		if code, _, errOut := runInit("--ci", "both", other); code != cli.ExitOK {
			t.Errorf("init in an empty .numinance = %d, stderr %q, want 0", code, errOut)
		}
	})
}

func equalFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestInit_Run_ConflictInTheOrgTreeWritesNothing(t *testing.T) {
	repo, org := t.TempDir(), t.TempDir()
	write(t, org, ".numinance/policy/old.yaml", "x")
	code, _, errOut := runInit("--ci", "both", "--org-dir", org, repo)
	if code != cli.ExitConflict {
		t.Errorf("init = %d, stderr %q, want %d", code, errOut, cli.ExitConflict)
	}
	if files := readFiles(t, repo); len(files) != 0 {
		t.Errorf("a conflict in the org tree left %v in the repository, want nothing", files)
	}
}

func TestInit_Run_RefusesLinks(t *testing.T) {
	link := func(t *testing.T, target, name string) {
		t.Helper()
		if err := os.Symlink(target, name); err != nil {
			t.Skipf("symlinks are not available: %v", err)
		}
	}
	t.Run("config directory is a link", func(t *testing.T) {
		repo, elsewhere := t.TempDir(), t.TempDir()
		link(t, elsewhere, filepath.Join(repo, ".numinance"))
		if code, _, errOut := runInit("--ci", "both", "--force", repo); code != cli.ExitInvalid {
			t.Errorf("init = %d, stderr %q, want %d", code, errOut, cli.ExitInvalid)
		}
		if files := readFiles(t, elsewhere); len(files) != 0 {
			t.Errorf("init wrote through the link: %v", files)
		}
	})
	t.Run("policy directory is a link", func(t *testing.T) {
		repo, elsewhere := t.TempDir(), t.TempDir()
		if err := os.Mkdir(filepath.Join(repo, ".numinance"), 0o750); err != nil {
			t.Fatal(err)
		}
		link(t, elsewhere, filepath.Join(repo, ".numinance", "policy"))
		if code, _, errOut := runInit("--ci", "both", "--force", repo); code != cli.ExitInvalid {
			t.Errorf("init = %d, stderr %q, want %d", code, errOut, cli.ExitInvalid)
		}
		if files := readFiles(t, elsewhere); len(files) != 0 {
			t.Errorf("init wrote through the link: %v", files)
		}
	})
	t.Run("a document is a link", func(t *testing.T) {
		repo, elsewhere := t.TempDir(), t.TempDir()
		write(t, repo, ".numinance/policy/placeholder.yaml", "x")
		write(t, elsewhere, "target.yaml", "untouched\n")
		link(t, filepath.Join(elsewhere, "target.yaml"), filepath.Join(repo, ".numinance", "policy", "autonomy.yaml"))
		if code, _, errOut := runInit("--ci", "both", "--force", repo); code != cli.ExitInvalid {
			t.Errorf("init = %d, stderr %q, want %d", code, errOut, cli.ExitInvalid)
		}
		if got := readFiles(t, elsewhere)["target.yaml"]; got != "untouched\n" {
			t.Errorf("init wrote through the link: %q", got)
		}
	})
	t.Run("a leftover temporary file is not reused", func(t *testing.T) {
		repo := t.TempDir()
		write(t, repo, ".numinance/policy/sdlc-profile.yaml.numi-init.tmp", "x")
		if code, _, _ := runInit("--ci", "both", "--force", repo); code != cli.ExitConflict {
			t.Errorf("init = %d, want %d", code, cli.ExitConflict)
		}
	})
}

func TestInit_Run_FileModes(t *testing.T) {
	repo := t.TempDir()
	if code, _, errOut := runInit("--ci", "both", repo); code != cli.ExitOK {
		t.Fatalf("init = %d, stderr %q, want 0", code, errOut)
	}
	err := filepath.WalkDir(filepath.Join(repo, ".numinance"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		want := fs.FileMode(0o644)
		if d.IsDir() {
			want = 0o755
		}
		if got := fi.Mode().Perm(); got != want {
			t.Errorf("mode of %s = %o, want %o", p, got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInit_Run_FilesAreBoundedAndFree(t *testing.T) {
	repo, org := t.TempDir(), t.TempDir()
	if code, _, errOut := runInit("--ci", "both", "--org-dir", org, repo); code != cli.ExitOK {
		t.Fatalf("init = %d, stderr %q, want 0", code, errOut)
	}
	for _, files := range []map[string]string{readFiles(t, repo), readFiles(t, org)} {
		for name, content := range files {
			if len(content) > 16<<10 {
				t.Errorf("%s is %d bytes, want at most 16 KiB", name, len(content))
			}
			if strings.ContainsAny(content, "\u2014\u2013") {
				t.Errorf("%s has an em or en dash, want plain text", name)
			}
		}
	}
}

// TestInit_Run_AutonomyIsPROnly pins the PR_ONLY mapping: the ladder default is PR and, with an org tree, the floors cap
// every change at PR and never above; no document carries a merge level.
func TestInit_Run_AutonomyIsPROnly(t *testing.T) {
	repo, org := t.TempDir(), t.TempDir()
	if code, _, errOut := runInit("--ci", "both", "--org-dir", org, repo); code != cli.ExitOK {
		t.Fatalf("init = %d, stderr %q, want 0", code, errOut)
	}
	policy := readFiles(t, org)[".numinance/policy/autonomy.yaml"]
	for _, want := range []string{"defaults: {level: AUTONOMY_LEVEL_PR}", "max_level: AUTONOMY_LEVEL_PR}", "floors:"} {
		if !strings.Contains(policy, want) {
			t.Errorf("org autonomy policy lacks %q:\n%s", want, policy)
		}
	}
	for _, files := range []map[string]string{readFiles(t, repo), readFiles(t, org)} {
		for name, content := range files {
			for _, bad := range []string{"AUTONOMY_LEVEL_MERGE", "AUTONOMY_LEVEL_STAGE", "AUTONOMY_LEVEL_RELEASE", "AUTONOMY_LEVEL_OPERATE", "CHANGE_STATE_MERGING"} {
				if strings.Contains(content, bad) {
					t.Errorf("%s mentions %s, want PR only", name, bad)
				}
			}
		}
	}
}

func TestRoot_Run_InitDispatch(t *testing.T) {
	var out, errb bytes.Buffer
	repo := t.TempDir()
	r := cli.Root{Init: cli.RunInit, Stdout: &out, Stderr: &errb}
	if code := r.Run(context.Background(), []string{"init", "--ci", "both", repo}); code != cli.ExitOK {
		t.Errorf("Run(init) = %d, stderr %q, want 0", code, errb.String())
	}
	if code := (cli.Root{Stderr: &errb}).Run(context.Background(), []string{"init", "--ci", "both", repo}); code != cli.ExitUsage {
		t.Errorf("Run(init) without the command set = %d, want %d", code, cli.ExitUsage)
	}
}

func TestInit_Run_ErrorsQuoteValuesOnce(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing repository", args: []string{"--ci", "both", missing}, want: `numi init: directory "` + missing + `" not found`},
		{name: "unknown ci value", args: []string{"--ci", `a"b`, dir}, want: `--ci "a\"b" is not`},
		{name: "backslash in a path", args: []string{"--ci", "both", dir + `/x\y`}, want: `x\\y" not found`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errOut := runInit(tc.args...)
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("Run(%v) stderr = %q, want it to contain %q", tc.args, errOut, tc.want)
			}
			if strings.Contains(errOut, `\\"`) && !strings.Contains(tc.want, `\\"`) {
				t.Errorf("Run(%v) stderr = %q, has an escaped quote that is not part of a value", tc.args, errOut)
			}
		})
	}
}

func TestInit_Run_BothNotesMatchWhatIsWritten(t *testing.T) {
	t.Run("repository only", func(t *testing.T) {
		repo := t.TempDir()
		code, out, errOut := runInit("--ci", "both", repo)
		if code != cli.ExitOK {
			t.Fatalf("Run = %d, stderr %q, want 0", code, errOut)
		}
		if !strings.Contains(out, "the Jenkins binding is in .numinance/ci-jenkins.yaml and is ignored") {
			t.Errorf("stdout lacks the repository note:\n%s", out)
		}
		if _, ok := readFiles(t, repo)[".numinance/ci-jenkins.yaml"]; !ok {
			t.Errorf("the note names .numinance/ci-jenkins.yaml but it was not written")
		}
	})
	t.Run("with an organization directory", func(t *testing.T) {
		repo, org := t.TempDir(), t.TempDir()
		code, out, errOut := runInit("--ci", "both", "--org-dir", org, repo)
		if code != cli.ExitOK {
			t.Fatalf("Run = %d, stderr %q, want 0", code, errOut)
		}
		if strings.Contains(out, "is in .numinance/ci-jenkins.yaml") {
			t.Errorf("stdout names a file that is not written in the repository:\n%s", out)
		}
		if !strings.Contains(out, "org grant in .numinance/policy/ci-jenkins.yaml") {
			t.Errorf("stdout lacks the org note:\n%s", out)
		}
		if _, ok := readFiles(t, repo)[".numinance/ci-jenkins.yaml"]; ok {
			t.Errorf("the repository tree has .numinance/ci-jenkins.yaml in org mode")
		}
		if _, ok := readFiles(t, org)[".numinance/policy/ci-jenkins.yaml"]; !ok {
			t.Errorf("the note names the org grant but it was not written")
		}
	})
}

func TestInit_Run_OrgDirAliasOfTheRepositoryIsRefused(t *testing.T) {
	repo := t.TempDir()
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(repo, link); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}
	tests := []struct {
		name     string
		orgDir   string
		repoPath string
	}{
		{name: "same string", orgDir: repo, repoPath: repo},
		{name: "dot spelling", orgDir: filepath.Join(repo, "."), repoPath: repo},
		{name: "symlink alias", orgDir: link, repoPath: repo},
		{name: "symlink alias of the repository argument", orgDir: repo, repoPath: link},
		{name: "parent segment", orgDir: filepath.Join(repo, "..", filepath.Base(repo)), repoPath: repo},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := runInit("--ci", "both", "--org-dir", tc.orgDir, tc.repoPath)
			if code != cli.ExitInvalid {
				t.Errorf("init = %d, stderr %q, want %d", code, errOut, cli.ExitInvalid)
			}
			if !strings.Contains(errOut, "--org-dir is the repository itself") {
				t.Errorf("stderr = %q, want the same-directory message", errOut)
			}
			if out != "" {
				t.Errorf("stdout = %q, want none", out)
			}
			if files := readFiles(t, repo); len(files) != 0 {
				t.Errorf("a refused run wrote %v, want nothing", files)
			}
		})
	}
	t.Run("relative spelling", func(t *testing.T) {
		t.Chdir(filepath.Dir(repo))
		rel := filepath.Base(repo)
		code, _, errOut := runInit("--ci", "both", "--org-dir", "./"+rel, rel)
		if code != cli.ExitInvalid {
			t.Errorf("init = %d, stderr %q, want %d", code, errOut, cli.ExitInvalid)
		}
		if files := readFiles(t, repo); len(files) != 0 {
			t.Errorf("a refused run wrote %v, want nothing", files)
		}
	})
}

func TestInit_Run_BothNoteSaysTheJenkinsGrantIsNotBound(t *testing.T) {
	repo, org := t.TempDir(), t.TempDir()
	code, out, errOut := runInit("--ci", "both", "--org-dir", org, repo)
	if code != cli.ExitOK {
		t.Fatalf("Run = %d, stderr %q, want 0", code, errOut)
	}
	if !strings.Contains(out, "rebinding it to the placeholder is your explicit edit that validate does not check") {
		t.Errorf("stdout lacks the rebinding caveat:\n%s", out)
	}
}

func TestInit_Run_ClosedChoicesAreReportedWithTheirSource(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		want   []string
		readme bool
	}{
		{
			name: "absent flags print the default choices",
			args: []string{"--ci", "github-actions"},
			want: []string{"chose: language python (default)", "chose: scm github (default)", "chose: tickets none (default)"},
		},
		{
			name:   "given flags print the flag source and bind no tickets",
			args:   []string{"--ci", "github-actions", "--language", "python", "--scm", "github", "--tickets", "file"},
			want:   []string{"chose: language python (flag)", "chose: scm github (flag)", "chose: tickets file (flag)"},
			readme: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			code, out, errOut := runInit(append(tc.args, repo)...)
			if code != cli.ExitOK {
				t.Fatalf("Run(%v) = %d, stderr %q, want 0", tc.args, code, errOut)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("Run(%v) stdout = %q, want it to contain %q", tc.args, out, w)
				}
			}
			files := readFiles(t, repo)
			if got := strings.Contains(files[".numinance/README.md"], "numi v1a run --ticket FILE"); got != tc.readme {
				t.Errorf("README names the ticket file = %v, want %v", got, tc.readme)
			}
			for name, content := range files {
				if strings.Contains(strings.ToLower(content), "ticketing") && name != ".numinance/README.md" {
					t.Errorf("%s mentions a ticketing binding, want none", name)
				}
			}
		})
	}
}

func TestInit_Run_PlaybookHasOneStepPerIdentityKind(t *testing.T) {
	repo := t.TempDir()
	if code, _, errOut := runInit("--ci", "github-actions", repo); code != cli.ExitOK {
		t.Fatalf("Run = %d, stderr %q, want 0", code, errOut)
	}
	pb := readFiles(t, repo)[".numinance/policy/playbook-bugfix-bounded.yaml"]
	for _, want := range []string{"capability_set: implement", "capability_set: review", "capability_set: release"} {
		if !strings.Contains(pb, want) {
			t.Errorf("playbook lacks %q:\n%s", want, pb)
		}
	}
	for _, bad := range []string{"scm-write", "scm-pr", "open-pr", "scm.merge"} {
		if strings.Contains(pb, bad) {
			t.Errorf("playbook contains %q, want PR only with the deployment's set names", bad)
		}
	}
}
