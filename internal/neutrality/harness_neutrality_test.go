package neutrality_test

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Test V1-AB (design section 24, item 1; ADR 0069): the plugin packages hold
// no harness name. This is the numinance-spec half of the scan; the numinance
// half is numinance/internal/execution/harness_neutrality_test.go, and its
// header explains the split. This module must not import numinance, so the
// matcher is a copy. Keep the tables and the matcher in step.
//
// Cache check: go test caches a result only while every file the test opened
// is unchanged. Appending a harness name comment to
// proto/numinance/plugin/v1/envelope.proto and running `go test ./...` in this
// module, after a cached run, must fail TestHarnessNeutrality_NoHarnessNameInNeutralPackages.
//
// The test has no allow-list, no exemption and no skip list. Files over
// maxFileBytes and binary files (a NUL byte) fail the test instead of being
// truncated or skipped.

// neutralRoots are the trees that must not name a harness, relative to the
// module root of numinance-spec. The numinance roots (kernel, plugin host and
// gateway, broker, credentials) are scanned by the numinance module test
// execution/harness_neutrality_test.go. Every root must exist, so a rename
// cannot silently drop one.
func neutralRoots() []string {
	return []string{
		"sdk/go/plugin",
		"sdk/go/gen/numinance/plugin",
		"proto/numinance/plugin",
		"plugins",
	}
}

const maxFileBytes = 4 << 20

// substringNames are matched case-insensitively anywhere in a line, so
// camelCase and PascalCase identifiers (ClaudeCode, shippedAnthropicCanary) and
// variables (OPENAI_API_KEY) are hits.
func substringNames() []string { return []string{"claude", "codex", "anthropic", "openai"} }

// cursorProduct matches only the product forms of Cursor. The English word
// (a pagination cursor, a text cursor) is everywhere and is not the harness:
// the capitalised proper noun "Cursor", "cursor-agent", "cursor-cli",
// "cursor cli" in any case and "cursor.com".
var cursorProduct = regexp.MustCompile(`\bCursor\b|(?i:\bcursor-agent\b|\bcursor-cli\b|\bcursor cli\b|\bcursor\.com\b)`)

// mayHoldHarnessName is true for a file that could hold a harness name.
func mayHoldHarnessName(b []byte) bool {
	lower := bytes.ToLower(b)
	for _, w := range append([]string{"cursor"}, substringNames()...) {
		if bytes.Contains(lower, []byte(w)) {
			return true
		}
	}
	return false
}

// harnessName returns the harness name found in line, or "".
func harnessName(line string) string {
	lower := strings.ToLower(line)
	for _, w := range substringNames() {
		if strings.Contains(lower, w) {
			return w
		}
	}
	return cursorProduct.FindString(line)
}

func TestHarnessName(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{name: "lowercase name", line: "the claude harness", want: "claude"},
		{name: "capitalised name", line: "Claude Code runs it", want: "claude"},
		{name: "upper case name", line: "CODEX", want: "codex"},
		{name: "dotted directory", line: `protected: ".claude/rules"`, want: "claude"},
		{name: "host name", line: "dest=api.anthropic.com:443", want: "anthropic"},
		{name: "underscore separated variable", line: "ANTHROPIC_API_KEY=x", want: "anthropic"},
		{name: "hyphenated secret", line: "dev/openai-api-key", want: "openai"},
		{name: "cursor proper noun", line: "Offer Cursor for regulated components", want: "Cursor"},
		{name: "cursor agent binary", line: "exec cursor-agent --print", want: "cursor-agent"},
		{name: "cursor cli prose", line: "the CURSOR CLI adapter", want: "CURSOR CLI"},
		{name: "cursor site", line: "https://cursor.com/docs", want: "cursor.com"},
		{name: "pagination cursor", line: "the pagination cursor is opaque", want: ""},
		{name: "text cursor", line: "move the cursor to the end", want: ""},
		{name: "cursor identifier", line: "nextCursor, cursor_id, Cursors", want: ""},
		{name: "pascal case identifier", line: "type ClaudeCode struct{}", want: "claude"},
		{name: "camel case identifier", line: "shippedAnthropicCanary", want: "anthropic"},
		{name: "exported camel case identifier", line: "ShippedAnthropicSecret", want: "anthropic"},
		{name: "upper case variable", line: "OPENAI_API_KEY", want: "openai"},
		{name: "comment", line: "// ClaudeCode", want: "claude"},
		{name: "neutral identifier", line: "ShippedProviderSecret", want: ""},
		{name: "cursor in prose", line: "pagination cursor", want: ""},
		{name: "empty", line: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := harnessName(tc.line); got != tc.want {
				t.Errorf("harnessName(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

// repoRoot walks up from the test's directory to the directory with go.mod (the module root).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// scanFile reports one finding per line of path that names a harness.
func scanFile(path, rel string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // G304: a path found by walking the fixed neutral roots
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFileBytes {
		return nil, fmt.Errorf("%s is over %d bytes; split it or shrink it", rel, maxFileBytes)
	}
	if bytes.IndexByte(b, 0) >= 0 {
		return []string{fmt.Sprintf("%s: binary file (NUL byte); a neutral root holds text files only", rel)}, nil
	}
	// A cheap whole-file prefilter keeps the race-enabled run fast; the
	// per-line matcher below decides.
	if !mayHoldHarnessName(b) {
		return nil, nil
	}
	var found []string
	for i, line := range strings.Split(string(b), "\n") {
		if name := harnessName(line); name != "" {
			found = append(found, fmt.Sprintf("%s:%d: %q", rel, i+1, name))
		}
	}
	return found, nil
}

// scanRoots scans every root (relative to repoRoot) and returns the findings
// and the problems that stop a scan from being trusted: a missing root, a
// walk error, and any entry that is neither a directory nor a regular file
// (a symlink would hide a name from the scan).
func scanRoots(repoRoot string, roots []string) (findings, problems []string) {
	for _, r := range roots {
		dir := filepath.Join(repoRoot, filepath.FromSlash(r))
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			problems = append(problems, fmt.Sprintf("neutral root %s is missing: %v", r, err))
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(repoRoot, path)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				problems = append(problems, fmt.Sprintf("%s is not a regular file (%v); a neutral root holds only files and directories", rel, d.Type()))
				return nil
			}
			found, err := scanFile(path, rel)
			if err != nil {
				return err
			}
			findings = append(findings, found...)
			return nil
		})
		if err != nil {
			problems = append(problems, fmt.Sprintf("walk %s: %v", r, err))
		}
	}
	return findings, problems
}

func TestHarnessNeutrality_NoHarnessNameInNeutralPackages(t *testing.T) {
	findings, problems := scanRoots(repoRoot(t), neutralRoots())
	for _, p := range problems {
		t.Error(p)
	}
	for _, f := range findings {
		t.Errorf("harness name outside an adapter package or executor manifest: %s", f)
	}
}

func TestScanRoots_SeededCamelCaseHit(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "neutral"), 0o750); err != nil {
		t.Fatal(err)
	}
	src := "package neutral\n\n// ClaudeCode is a harness.\ntype ClaudeCode struct{}\n"
	if err := os.WriteFile(filepath.Join(tmp, "neutral", "x.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	findings, problems := scanRoots(tmp, []string{"neutral"})
	if len(problems) != 0 {
		t.Errorf("scanRoots problems = %v, want none", problems)
	}
	if len(findings) != 2 {
		t.Errorf("scanRoots findings = %v, want 2 (comment and type)", findings)
	}
}

func TestScanRoots_SymlinkFails(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "neutral"), 0o750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "elsewhere.txt")
	if err := os.WriteFile(target, []byte("claude"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(tmp, "neutral", "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, problems := scanRoots(tmp, []string{"neutral"})
	if len(problems) != 1 {
		t.Errorf("scanRoots problems = %v, want one symlink problem", problems)
	}
}

func TestScanRoots_BinaryFileFails(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "neutral"), 0o750); err != nil {
		t.Fatal(err)
	}
	// A NUL byte must not hide a name from the scan.
	body := "\x00\nThe broker reads ANTHROPIC_API_KEY for Claude Code.\n"
	if err := os.WriteFile(filepath.Join(tmp, "neutral", "README.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	findings, problems := scanRoots(tmp, []string{"neutral"})
	if len(problems) != 0 {
		t.Errorf("scanRoots problems = %v, want none", problems)
	}
	if len(findings) != 1 {
		t.Errorf("scanRoots findings = %v, want one binary-file finding", findings)
	}
}
