package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeIn(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func filesBelow(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func runWithHooks(args []string, hooks discoverHooks) (code int, stderr string) {
	var errb bytes.Buffer
	code = runDiscoverCommand(context.Background(), args, nil, &errb, hooks)
	return code, errb.String()
}

func TestConfidenceOf_Evidence_Levels(t *testing.T) {
	tests := []struct {
		ev   evidence
		want confidence
	}{
		{evSettingsFile, confHigh}, {evPyprojectTable, confHigh}, {evLockfile, confHigh}, {evVersionFile, confHigh},
		{evClonedBranch, confHigh}, {evCodeownersFile, confHigh}, {evCIConfig, confHigh},
		{evDependency, confMedium}, {evRequiresPython, confMedium}, {evOtherSection, confMedium}, {evHintFile, confMedium},
		{evDispatchFlow, confMedium}, {evFlowNoDispatch, confMedium}, {evSourceFiles, confMedium},
		{evRequirementsUse, confLow}, {evTextUse, confLow}, {evCurrentBranch, confLow}, {evNothingFound, confLow},
		{"", confLow}, {"something-new", confLow},
	}
	for _, tc := range tests {
		if got := confidenceOf(tc.ev); got != tc.want {
			t.Errorf("confidenceOf(%q) = %s, want %s", tc.ev, got, tc.want)
		}
	}
}

func TestLowest_Levels_PicksTheWeakest(t *testing.T) {
	tests := []struct {
		in   []confidence
		want confidence
	}{
		{[]confidence{confHigh, confHigh}, confHigh},
		{[]confidence{confHigh, confMedium}, confMedium},
		{[]confidence{confMedium, confLow, confHigh}, confLow},
	}
	for _, tc := range tests {
		if got := lowest(tc.in...); got != tc.want {
			t.Errorf("lowest(%v) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestScanWorkflow_Text_FindsJobsAndTriggers(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		jobs       []string
		odd        int
		pr, push   bool
		dispatched bool
	}{
		{name: "block events", text: "on:\n  pull_request:\n  push:\n    branches: [main]\njobs:\n  a:\n    runs-on: x\n  b-c:\n    runs-on: y\n", jobs: []string{"a", "b-c"}, pr: true, push: true},
		{name: "inline list", text: "on: [push, workflow_dispatch]\njobs:\n  a:\n    steps:\n      - run: x\n", jobs: []string{"a"}, push: true, dispatched: true},
		{name: "inline scalar", text: "on: pull_request\njobs:\n  a:\n    runs-on: x\n", jobs: []string{"a"}, pr: true},
		{name: "flow mapping", text: "on: {push: {branches: [main]}}\njobs: {}\n", push: true},
		{name: "quoted key", text: "\"on\":\n  push:\n", push: true},
		{name: "target events are not pull_request", text: "on:\n  pull_request_target:\n  workflow_run:\n"},
		{name: "commented events", text: "on:\n  # push:\n  workflow_dispatch: # push\n", dispatched: true},
		{name: "events outside on", text: "name: push\nenv:\n  X: pull_request\non:\n  schedule:\n"},
		{name: "unsafe job ids", text: "jobs:\n  \"a b\":\n    runs-on: x\n  ok:\n    runs-on: x\n  \"x\\u202e\":\n    runs-on: x\n", jobs: []string{"ok"}, odd: 2},
		{name: "steps are not jobs", text: "jobs:\n  a:\n    steps:\n      - name: build:\n        run: x\n    env:\n      Z:\n", jobs: []string{"a"}},
		{name: "windows line ends", text: "on:\r\n  push:\r\njobs:\r\n  a:\r\n    runs-on: x\r\n", jobs: []string{"a"}, push: true},
		{name: "tab indent", text: "jobs:\n\ta:\n\t\truns-on: x\n", jobs: []string{"a"}},
		{name: "empty", text: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scanWorkflow("w.yml", []byte(tc.text))
			if !slices.Equal(got.Jobs, tc.jobs) || got.OddJobs != tc.odd || got.PullRequest != tc.pr || got.Push != tc.push || got.Dispatched != tc.dispatched {
				t.Errorf("scanWorkflow(%q) = jobs %v odd %d pr %v push %v dispatch %v, want jobs %v odd %d pr %v push %v dispatch %v",
					tc.text, got.Jobs, got.OddJobs, got.PullRequest, got.Push, got.Dispatched, tc.jobs, tc.odd, tc.pr, tc.push, tc.dispatched)
			}
		})
	}
}

func TestScanHooks_Text_FindsSafeIDs(t *testing.T) {
	text := "repos:\n- repo: x\n  hooks:\n  - id: ruff\n  - id: ruff-format # fmt\n  - id: \"quoted\"\n  - id: a;b\n  -   id:   spaced\n  - id: \n  - repo: id: y\n"
	ids, odd := scanHooks([]byte(text))
	if want := []string{"ruff", "ruff-format", "spaced"}; !slices.Equal(ids, want) || odd != 2 {
		t.Errorf("scanHooks = %v, %d odd, want %v, 2 odd", ids, odd, want)
	}
	var many strings.Builder
	for range maxHookIDs + 20 {
		many.WriteString("  - id: h\n")
	}
	if ids, _ := scanHooks([]byte(many.String())); len(ids) != maxHookIDs {
		t.Errorf("scanHooks kept %d ids, want the bound %d", len(ids), maxHookIDs)
	}
}

func TestAnnotate_Document_AddsOneBlockAfterMetadata(t *testing.T) {
	doc := []byte("# c\napiVersion: numinance.dev/v1\nkind: X\nmetadata: {name: a}\nspec:\n  k: v\n")
	got := string(annotate(doc, []annotation{{Field: "spec.k", Level: confLow, Evidence: "a\"b\n; rm -rf"}}))
	if want := "metadata: {name: a}\n# The extensions block"; !strings.Contains(got, want) {
		t.Errorf("annotate = %q, want the block right after metadata", got)
	}
	if !strings.Contains(got, `spec.k: {confidence: low, evidence: "a?b?; rm -rf"}`) {
		t.Errorf("annotate did not neutralise the evidence: %q", got)
	}
	if got := annotate([]byte("kind: X\n"), []annotation{{Field: "f"}}); string(got) != "kind: X\n" {
		t.Errorf("annotate of a document without metadata = %q, want it unchanged", got)
	}
}

func discoverRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	writeIn(t, repo, "pyproject.toml", "[tool.ruff]\n[tool.pytest.ini_options]\n")
	writeIn(t, repo, ".git/HEAD", "ref: refs/heads/main\n")
	writeIn(t, repo, ".git/refs/heads/main", "0123456789abcdef0123456789abcdef01234567\n")
	return repo
}

func TestDiscover_SelfTest_FailedDocumentWritesNothing(t *testing.T) {
	repo := discoverRepo(t)
	base := t.TempDir()
	out := filepath.Join(base, "draft")
	corrupt := func(files []outFile) []outFile {
		files = slices.Clone(files)
		for i := range files {
			if strings.HasSuffix(files[i].Path, "toolchain-python.yaml") {
				files[i].Data = bytes.Replace(files[i].Data, []byte("language: python"), []byte("language: cobol\n  surprise: 1"), 1)
			}
		}
		return files
	}
	code, stderr := runWithHooks([]string{"--out", out, repo}, discoverHooks{mutateDraft: corrupt})
	if code != ExitInvalid || !strings.Contains(stderr, "self-test failed") || !strings.Contains(stderr, "toolchain-python.yaml") {
		t.Errorf("discover = %d, stderr %q, want %d and a self-test failure naming the document", code, stderr, ExitInvalid)
	}
	if got := filesBelow(t, base); len(got) != 0 {
		t.Errorf("files left after a failed self-test: %v", got)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("the out directory was left behind")
	}
}

func TestDiscover_SelfTest_RepositoryChangedDuringTheRunIsAConflict(t *testing.T) {
	tests := []struct {
		name string
		// write is the files that appear or change before the second read; remove deletes the repository instead.
		write  map[string]string
		remove bool
		want   string
	}{
		{name: "a tool appears", write: map[string]string{"mypy.ini": ""}, want: "verifier"},
		{name: "the package manager changes", write: map[string]string{"poetry.lock": ""}, want: "package manager"},
		{name: "the python version changes", write: map[string]string{".python-version": "3.10\n"}, want: "python"},
		{name: "CI appears", write: map[string]string{"Jenkinsfile": "pipeline {}"}, want: "ci"},
		{name: "CODEOWNERS appears", write: map[string]string{"CODEOWNERS": "all @a\n"}, want: "CODEOWNERS"},
		{name: "the branch changes", write: map[string]string{".git/refs/remotes/origin/HEAD": "ref: refs/remotes/origin/trunk\n"}, want: "default branch"},
		{name: "a commit lands", write: map[string]string{".git/refs/heads/main": "fedcba9876543210fedcba9876543210fedcba98\n"}, want: "HEAD"},
		{name: "the repository disappears", remove: true, want: "changed while discover ran"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := discoverRepo(t)
			base := t.TempDir()
			out := filepath.Join(base, "draft")
			change := func() {
				if tc.remove {
					_ = os.RemoveAll(repo)
				}
				for name, content := range tc.write {
					writeIn(t, repo, name, content)
				}
			}
			code, stderr := runWithHooks([]string{"--out", out, repo}, discoverHooks{beforeReread: change})
			if code != ExitConflict || !strings.Contains(stderr, "changed while discover ran") || !strings.Contains(stderr, tc.want) {
				t.Errorf("discover = %d, stderr %q, want %d naming %q", code, stderr, ExitConflict, tc.want)
			}
			if got := filesBelow(t, base); len(got) != 0 {
				t.Errorf("files left after a failed self-test: %v", got)
			}
		})
	}
}

func TestDiscover_Write_FailureUndoesWhatWasCreated(t *testing.T) {
	repo := discoverRepo(t)
	out := t.TempDir()
	// A temporary file left by an earlier run makes the third document fail after two were written.
	writeIn(t, out, ".numinance/policy/stage-registry.yaml"+tmpSuffix, "left over\n")
	code, stderr := runWithHooks([]string{"--force", "--out", out, repo}, discoverHooks{})
	if code != ExitConflict || !strings.Contains(stderr, "left over") {
		t.Errorf("discover = %d, stderr %q, want %d naming the left-over file", code, stderr, ExitConflict)
	}
	want := []string{".numinance/policy/stage-registry.yaml" + tmpSuffix}
	if got := filesBelow(t, out); !slices.Equal(got, want) {
		t.Errorf("files after the failed write = %v, want only %v", got, want)
	}
}

func TestDiscover_Run_InitOutputIsUnchangedByTheNewInspection(t *testing.T) {
	// The extra enforcement artifacts are read, but init writes no line about them.
	repo := discoverRepo(t)
	writeIn(t, repo, ".github/workflows/ci.yml", "on: [push]\njobs:\n  a:\n    runs-on: x\n")
	writeIn(t, repo, ".pre-commit-config.yaml", "- id: ruff\n")
	var out, errb bytes.Buffer
	if code := RunInit(context.Background(), []string{repo}, &out, &errb); code != ExitOK {
		t.Fatalf("init = %d, stderr %q", code, errb.String())
	}
	for _, bad := range []string{"jobs", "hook", "confidence", "artifact"} {
		if strings.Contains(out.String(), bad) {
			t.Errorf("init output mentions %q:\n%s", bad, out.String())
		}
	}
}
