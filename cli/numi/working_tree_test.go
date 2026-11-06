package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

const toolchain = `apiVersion: numinance.dev/v1
kind: LanguageToolchain
metadata: {name: go-1.27, scope: SCOPE_ORG}
spec:
  language: go
  version: "1.27"
  verifiers:
    - {name: unit-tests, provider: dev.numinance.official/go-test}
  formatters: [gofmt]
  package_manager: go-modules
`

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshot(dir string, mod func(*cli.Request)) (cli.Snapshot, error) {
	req := cli.Request{RepoDir: dir}
	if mod != nil {
		mod(&req)
	}
	return workingTree{}.Snapshot(context.Background(), req)
}

func kindOf(err error) cli.Kind { return cli.DefaultErrorKind(err) }

func TestWorkingTree_Snapshot_ReadsThreeTrees(t *testing.T) {
	org, repo := t.TempDir(), t.TempDir()
	write(t, org, ".numinance/policy/go.yaml", toolchain)
	write(t, repo, ".numinance/policy/a.yaml", toolchain)
	write(t, repo, "svc/api/.numinance/profile.yaml", toolchain)
	write(t, repo, ".numinance/notes.txt", "ignored")
	snap, err := snapshot(repo, func(r *cli.Request) { r.OrgDir = org; r.Component = "svc/api" })
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(snap.Documents))
	for _, d := range snap.Documents {
		got = append(got, d.Scope+":"+d.Path)
	}
	want := "org:.numinance/policy/go.yaml repo:.numinance/policy/a.yaml component:svc/api/.numinance/profile.yaml"
	if strings.Join(got, " ") != want {
		t.Errorf("Snapshot documents = %v, want %s", got, want)
	}
	if !strings.Contains(snap.Revision, "working tree") {
		t.Errorf("Snapshot Revision = %q, want it to say working tree", snap.Revision)
	}
}

func TestWorkingTree_Snapshot_Refusals(t *testing.T) {
	linkTo := func(target func(t *testing.T) string, name string) func(t *testing.T, dir string) {
		return func(t *testing.T, dir string) {
			t.Helper()
			if err := os.Symlink(target(t), filepath.Join(dir, name)); err != nil {
				t.Skip(err)
			}
		}
	}
	otherDir := func(t *testing.T) string {
		t.Helper()
		d := t.TempDir()
		write(t, d, "keep.yaml", toolchain)
		return d
	}
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		mod   func(*cli.Request)
		want  cli.Kind
	}{
		{name: "rev given", mod: func(r *cli.Request) { r.Rev = "HEAD" }, want: cli.KindUsage},
		{name: "missing repo directory", mod: func(r *cli.Request) { r.RepoDir = filepath.Join(r.RepoDir, "nope") }, want: cli.KindNotFound},
		{name: "missing org directory", mod: func(r *cli.Request) { r.OrgDir = filepath.Join(r.RepoDir, "nope") }, want: cli.KindNotFound},
		{name: "component with dot-dot", mod: func(r *cli.Request) { r.Component = "../x" }, want: cli.KindInvalid},
		{name: "component absolute", mod: func(r *cli.Request) { r.Component = "/etc" }, want: cli.KindInvalid},
		{name: "component names the config dir", mod: func(r *cli.Request) { r.Component = "a/.numinance" }, want: cli.KindInvalid},
		{name: "component names the config dir in capitals", mod: func(r *cli.Request) { r.Component = ".NUMINANCE" }, want: cli.KindInvalid},
		{name: "component with a backslash", mod: func(r *cli.Request) { r.Component = `a\.numinance` }, want: cli.KindInvalid},
		{
			name: "component path crosses a link that stays inside the repository",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				write(t, dir, "real/api/.numinance/p.yaml", toolchain)
				if err := os.Symlink("real", filepath.Join(dir, "svc")); err != nil {
					t.Skip(err)
				}
			},
			mod:  func(r *cli.Request) { r.Component = "svc/api" },
			want: cli.KindInvalid,
		},
		{
			name: "config directory is a regular file",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				write(t, dir, ".numinance", "not a directory")
			},
			want: cli.KindInvalid,
		},
		{
			name: "config directory is a link to a directory inside the repository",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				write(t, dir, "real/policy/go.yaml", toolchain)
				if err := os.Symlink("real", filepath.Join(dir, ".numinance")); err != nil {
					t.Skip(err)
				}
			},
			want: cli.KindInvalid,
		},
		{
			name: "component path crosses a regular file",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				write(t, dir, "svc", "not a directory")
			},
			mod:  func(r *cli.Request) { r.Component = "svc/api" },
			want: cli.KindInvalid,
		},
		{
			name: "component directory is a regular file",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				write(t, dir, "svc/api", "not a directory")
			},
			mod:  func(r *cli.Request) { r.Component = "svc/api" },
			want: cli.KindInvalid,
		},
		{name: "config directory is a link", setup: linkTo(otherDir, ".numinance"), want: cli.KindInvalid},
		{
			name: "component path crosses a link",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				d := otherDir(t)
				write(t, d, "api/.numinance/p.yaml", toolchain)
				linkTo(func(*testing.T) string { return d }, "svc")(t, dir)
			},
			mod:  func(r *cli.Request) { r.Component = "svc/api" },
			want: cli.KindInvalid,
		},
		{
			name: "document is a link",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				write(t, dir, ".numinance/keep.txt", "x")
				d := otherDir(t)
				if err := os.Symlink(filepath.Join(d, "keep.yaml"), filepath.Join(dir, ".numinance", "l.yaml")); err != nil {
					t.Skip(err)
				}
			},
			want: cli.KindInvalid,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, dir)
			}
			_, err := snapshot(dir, tc.mod)
			if err == nil || kindOf(err) != tc.want {
				t.Errorf("Snapshot(%s) err = %v (kind %d), want kind %d", tc.name, err, kindOf(err), tc.want)
			}
		})
	}
}

func TestWorkingTree_Snapshot_OversizedDocumentIsBounded(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".numinance/big.yaml", strings.Repeat("a", 4*kindyaml.MaxBytes))
	snap, err := snapshot(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(snap.Documents[0].Data); n != kindyaml.MaxBytes+1 {
		t.Errorf("Snapshot read %d bytes, want %d (limit plus one)", n, kindyaml.MaxBytes+1)
	}
}

func TestWorkingTree_Snapshot_Limits(t *testing.T) {
	tests := []struct {
		name  string
		files int
		ext   string
		size  int
	}{
		{name: "too many documents", files: maxDocs + 1, ext: ".yaml", size: 10},
		{name: "too many entries", files: maxEntries + 1, ext: ".txt", size: 1},
		{name: "too many bytes", files: maxTotalBytes/kindyaml.MaxBytes + 2, ext: ".yaml", size: kindyaml.MaxBytes},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := strings.Repeat("a", tc.size)
			for i := range tc.files {
				write(t, dir, fmt.Sprintf(".numinance/d%05d%s", i, tc.ext), body)
			}
			if _, err := snapshot(dir, nil); err == nil || kindOf(err) != cli.KindInvalid {
				t.Errorf("Snapshot(%s) err = %v, want Invalid", tc.name, err)
			}
		})
	}
}

func TestRoot_Validate_SchemaOnlyExitsUnavailable(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".numinance/policy/go.yaml", toolchain)
	var out, errb bytes.Buffer
	r := root("test")
	r.Stdout, r.Stderr = &out, &errb
	if code := r.Run(context.Background(), []string{"validate", dir}); code != cli.ExitUnavailable {
		t.Errorf("validate = %d, want %d (stdout %q)", code, cli.ExitUnavailable, out.String())
	}
	if !strings.Contains(out.String(), "LanguageToolchain") || !strings.Contains(errb.String(), "engine build") {
		t.Errorf("stdout %q stderr %q, want the schema pass and the engine-build message", out.String(), errb.String())
	}
}

func TestRoot_Validate_RevInTheOpenBuildIsAUsageError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".numinance/policy/go.yaml", toolchain)
	for _, rev := range []string{"HEAD", "main", "abc123"} {
		r := root("test")
		code := r.Run(context.Background(), []string{"validate", "--rev", rev, dir})
		if code != cli.ExitUsage {
			t.Errorf("validate --rev %s = %d, want %d (never %d)", rev, code, cli.ExitUsage, cli.ExitUnavailable)
		}
	}
}

func TestRoot_Validate_CanceledContextIsNotUnavailable(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".numinance/policy/go.yaml", toolchain)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := root("test")
	if code := r.Run(ctx, []string{"validate", dir}); code != cli.ExitInternal {
		t.Errorf("validate with a canceled context = %d, want %d", code, cli.ExitInternal)
	}
}
