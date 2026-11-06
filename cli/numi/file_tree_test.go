package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

func runFiles(t *testing.T, args ...string) (code int, out, errb string) {
	t.Helper()
	var o, e bytes.Buffer
	r := root("test")
	r.Stdout, r.Stderr = &o, &e
	code = r.Run(context.Background(), append([]string{"validate", "--files"}, args...))
	return code, o.String(), e.String()
}

func TestRoot_ValidateFiles_ExitCodes(t *testing.T) {
	good := t.TempDir()
	write(t, good, "LanguageToolchain/go.yaml", toolchain)
	write(t, good, "notes.txt", "ignored")
	bad := t.TempDir()
	write(t, bad, "a.yaml", strings.Replace(toolchain, "language: go", "language: go\n  unknown_field: 1", 1))
	empty := t.TempDir()
	producedBad := t.TempDir()
	write(t, producedBad, "b.yaml", "apiVersion: bogus/v9\nkind: ContextBundle\nmetadata: {name: x}\n")
	producedUnknown := t.TempDir()
	write(t, producedUnknown, "b.yaml", bundle(t)+"surprise: 1\n")
	producedOK := t.TempDir()
	write(t, producedOK, "b.yaml", bundle(t))
	producedNoSpec := t.TempDir()
	write(t, producedNoSpec, "b.yaml", "apiVersion: numinance.dev/v1\nkind: ContextBundle\n")
	upper := t.TempDir()
	write(t, upper, "a.YAML", toolchain)
	mixed := t.TempDir()
	write(t, mixed, "sub/a.YML", toolchain)
	mixed2 := t.TempDir()
	write(t, mixed2, "a.Yml", toolchain)
	backup := t.TempDir()
	write(t, backup, "a.yaml.bak", toolchain)
	ymlOrig := t.TempDir()
	write(t, ymlOrig, "a.yml.orig", toolchain)
	otherTilde := t.TempDir()
	write(t, otherTilde, "a.yaml", toolchain)
	write(t, otherTilde, "README.md~", "notes")
	ymlTilde := t.TempDir()
	write(t, ymlTilde, "a.yml~", toolchain)
	tilde := t.TempDir()
	write(t, tilde, "a.yaml~", toolchain)
	dashOld := t.TempDir()
	write(t, dashOld, "x.yaml-old", toolchain)
	yamlx := t.TempDir()
	write(t, yamlx, "x.yamlx", toolchain)
	spaced := t.TempDir()
	write(t, spaced, "x.yaml ", toolchain)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "valid directory", args: []string{good}, want: cli.ExitOK},
		{name: "valid single file", args: []string{filepath.Join(good, "LanguageToolchain", "go.yaml")}, want: cli.ExitOK},
		{name: "unknown field is invalid", args: []string{bad}, want: cli.ExitInvalid},
		{name: "no documents is not found", args: []string{empty}, want: cli.ExitNotFound},
		{name: "missing path is not found", args: []string{filepath.Join(good, "nope")}, want: cli.ExitNotFound},
		{name: "link is invalid", args: []string{link}, want: cli.ExitInvalid},
		{name: "valid produced kind", args: []string{producedOK}, want: cli.ExitOK},
		{name: "produced kind with a bogus apiVersion", args: []string{producedBad}, want: cli.ExitInvalid},
		{name: "produced kind with an unknown field", args: []string{producedUnknown}, want: cli.ExitInvalid},
		{name: "upper case extension is refused", args: []string{upper}, want: cli.ExitInvalid},
		{name: "nested .YML is refused", args: []string{mixed}, want: cli.ExitInvalid},
		{name: ".Yml is refused", args: []string{mixed2}, want: cli.ExitInvalid},
		{name: ".yaml.bak is refused", args: []string{backup}, want: cli.ExitInvalid},
		{name: ".yml.orig is refused", args: []string{ymlOrig}, want: cli.ExitInvalid},
		{name: "a tilde backup of a non-YAML file is ignored", args: []string{otherTilde}, want: cli.ExitOK},
		{name: "yml backup tilde is refused", args: []string{ymlTilde}, want: cli.ExitInvalid},
		{name: ".yaml-old is refused", args: []string{dashOld}, want: cli.ExitInvalid},
		{name: ".yamlx is refused", args: []string{yamlx}, want: cli.ExitInvalid},
		{name: "trailing space is refused", args: []string{spaced}, want: cli.ExitInvalid},
		{name: "backup tilde is refused", args: []string{tilde}, want: cli.ExitInvalid},
		{name: "link with a trailing slash is invalid", args: []string{link + "/"}, want: cli.ExitInvalid},
		{name: "no path is usage", args: nil, want: cli.ExitUsage},
		{name: "produced kind without a spec", args: []string{producedNoSpec}, want: cli.ExitInvalid},
		{name: "component is usage", args: []string{"--component", "x", good}, want: cli.ExitUsage},
		{name: "single file without a yaml suffix is usage", args: []string{filepath.Join(good, "notes.txt")}, want: cli.ExitUsage},
		{name: "rev is usage", args: []string{"--rev", "HEAD", good}, want: cli.ExitUsage},
		{name: "org is usage", args: []string{"--org", good, good}, want: cli.ExitUsage},
		{name: "two paths is usage", args: []string{good, good}, want: cli.ExitUsage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code, out, errb := runFiles(t, tc.args...); code != tc.want {
				t.Errorf("validate --files %v = %d, want %d (stdout %q stderr %q)", tc.args, code, tc.want, out, errb)
			}
		})
	}
}

// bundle is the ContextBundle example of the spec.
func bundle(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "schemas", "examples", "ContextBundle", "implement-stage.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRoot_ValidateFiles_LowercaseExtensionsOnlyInFilesMode(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".numinance/a.YAML", toolchain)
	write(t, dir, ".numinance/b.yaml", toolchain)
	if _, err := snapshot(dir, nil); err != nil {
		t.Errorf("working tree Snapshot = %v, want the upper case file ignored", err)
	}
}

func TestFileTree_Snapshot_Limits(t *testing.T) {
	tests := []struct {
		name  string
		files int
		size  int
	}{
		{name: "too many documents", files: maxDocs + 1, size: 10},
		{name: "too many bytes", files: maxTotalBytes/kindyaml.MaxBytes + 2, size: kindyaml.MaxBytes},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for i := range tc.files {
				write(t, dir, fmt.Sprintf("d%05d.yaml", i), strings.Repeat("a", tc.size))
			}
			_, err := fileTree{}.Snapshot(context.Background(), cli.Request{RepoDir: dir, Files: true})
			if err == nil || kindOf(err) != cli.KindInvalid {
				t.Errorf("Snapshot(%s) err = %v, want Invalid", tc.name, err)
			}
		})
	}
}

func TestFileTree_Snapshot_OversizedDocumentIsInvalid(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "big.yaml", strings.Repeat("a", kindyaml.MaxBytes+1))
	if code, _, _ := runFiles(t, dir); code != cli.ExitInvalid {
		t.Errorf("validate --files oversized = %d, want %d", code, cli.ExitInvalid)
	}
}

func TestOpenSame_RefusesADifferentDirectory(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	fi, err := os.Lstat(a)
	if err != nil {
		t.Fatal(err)
	}
	root, err := openSame(a, fi)
	if err != nil {
		t.Fatalf("openSame(a, Lstat(a)) = %v, want nil", err)
	}
	_ = root.Close()
	if _, err := openSame(b, fi); err == nil || kindOf(err) != cli.KindInvalid {
		t.Errorf("openSame(b, Lstat(a)) = %v, want Invalid", err)
	}
}

func TestRoot_ValidateFiles_ProducedKindHasNoEmptyName(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.yaml", bundle(t))
	code, out, _ := runFiles(t, dir)
	if code != cli.ExitOK || !strings.Contains(out, "ContextBundle\n") || strings.Contains(out, `""`) {
		t.Errorf("validate --files = %d, %q; want 0 and no empty name", code, out)
	}
}

func TestFileTree_Snapshot_DotDotIsResolvedOnceAndWalkedFromTheOpenedRoot(t *testing.T) {
	base := t.TempDir()
	good := filepath.Join(base, "good")
	write(t, good, "a.yaml", toolchain)
	other := filepath.Join(base, "other", "deep")
	write(t, other, "evil.yaml", toolchain)
	if err := os.Symlink(other, filepath.Join(base, "sl")); err != nil {
		t.Fatal(err)
	}
	snap, err := fileTree{}.Snapshot(context.Background(), cli.Request{RepoDir: filepath.Join(base, "sl", "..", "good"), Files: true})
	if err != nil {
		t.Fatalf("Snapshot = %v", err)
	}
	if len(snap.Documents) != 1 || snap.Documents[0].Path != "a.yaml" {
		t.Errorf("Snapshot documents = %+v, want only good/a.yaml (the cleaned path)", snap.Documents)
	}
}

func TestRoot_ValidateFiles_EscapesInFileNamesNeverReachTheTerminal(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "\x1b]0;pwn\x07.YAML", toolchain)
	code, out, errb := runFiles(t, dir)
	if code != cli.ExitInvalid {
		t.Errorf("validate --files = %d, want %d", code, cli.ExitInvalid)
	}
	if strings.ContainsAny(out+errb, "\x1b\x07") {
		t.Errorf("output %q %q has a raw control character", out, errb)
	}
}

func TestFileTree_Snapshot_SingleFileSwappedAfterTheLstatIsRefused(t *testing.T) {
	dir, other := t.TempDir(), t.TempDir()
	write(t, dir, "a.yaml", toolchain)
	write(t, other, "a.yaml", toolchain)
	stale, err := os.Lstat(filepath.Join(other, "a.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ft := fileTree{lstat: func(string) (fs.FileInfo, error) { return stale, nil }}
	_, err = ft.Snapshot(context.Background(), cli.Request{RepoDir: filepath.Join(dir, "a.yaml"), Files: true})
	if err == nil || kindOf(err) != cli.KindInvalid {
		t.Errorf("Snapshot with a stale Lstat = %v, want Invalid", err)
	}
	if _, err := (fileTree{}).Snapshot(context.Background(), cli.Request{RepoDir: filepath.Join(dir, "a.yaml"), Files: true}); err != nil {
		t.Errorf("Snapshot of the file itself = %v, want nil", err)
	}
}
