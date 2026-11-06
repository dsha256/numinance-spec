package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

const vectors = "../../../conformance/evidence/v1"

func runEvidence(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	r := cli.Root{Evidence: cli.RunEvidence, Stdout: &out, Stderr: &errb}
	code := r.Run(context.Background(), append([]string{"evidence"}, args...))
	return code, out.String(), errb.String()
}

func key(name string) string    { return filepath.Join(vectors, "keys", name) }
func export(name string) string { return filepath.Join(vectors, "exports", name) }

func TestRunEvidence_Verify(t *testing.T) {
	pins := []string{"--runner-key", key("runner.pub"), "--log-key", key("log.pub"), "--log-origin", "numinance.test/log"}
	tests := []struct {
		name      string
		args      []string
		wantExit  int
		wantLevel string // "" when no report is printed
		wantEnum  string
		wantFlags int
	}{
		{name: "fully pinned export verifies", args: append(append([]string{}, pins...), "--json", export("closed-basic")), wantExit: 0, wantLevel: "verified", wantEnum: "verified"},
		{name: "directory first, flags after", args: append([]string{export("closed-basic"), "--json"}, pins...), wantExit: 0, wantLevel: "verified", wantEnum: "verified"},
		{name: "an allowance exits 0 and is visible in the enum fields", args: []string{"--runner-key", key("runner.pub"), "--allow-unanchored", "--json", export("closed-basic")}, wantExit: 0, wantLevel: "verified, unanchored", wantEnum: "verified", wantFlags: 1},
		{name: "unanchored without the flag is weak", args: []string{"--runner-key", key("runner.pub"), "--json", export("closed-basic")}, wantExit: 5, wantLevel: "unanchored", wantEnum: "unanchored"},
		{name: "a bundle key never reaches verified", args: []string{"--trust-bundle-keys", "--log-key", key("log.pub"), "--log-origin", "numinance.test/log", "--json", export("closed-basic")}, wantExit: 5, wantLevel: "self-consistent", wantEnum: "self-consistent"},
		{name: "skip signatures is unauthenticated", args: []string{"--skip-signatures", "--json", export("closed-basic")}, wantExit: 5, wantLevel: "unauthenticated", wantEnum: "unauthenticated"},
		{name: "a forged export fails", args: append(append([]string{}, pins...), "--json", export("forged-record")), wantExit: 3, wantLevel: "failed", wantEnum: "failed"},
		{name: "expect-change mismatch fails", args: append(append([]string{}, pins...), "--expect-change", "chg_02020202020202020202020202020202", "--json", export("closed-basic")), wantExit: 3, wantLevel: "failed", wantEnum: "failed"},
		{name: "missing directory", args: append(append([]string{}, pins...), export("no-such-export")), wantExit: 4},
		{name: "no key mode is usage", args: []string{export("closed-basic")}, wantExit: 2},
		{name: "two key modes are usage", args: []string{"--skip-signatures", "--trust-bundle-keys", export("closed-basic")}, wantExit: 2},
		{name: "no directory is usage", args: pins, wantExit: 2},
		{name: "two directories are usage", args: append(append([]string{}, pins...), export("closed-basic"), export("closed-sealed")), wantExit: 2},
		{name: "unknown flag is usage", args: append(append([]string{}, pins...), "--frob", export("closed-basic")), wantExit: 2},
		{name: "oversize key file is usage", args: []string{"--runner-key", key("runner-oversize.pub"), export("closed-basic")}, wantExit: 2},
		{name: "malformed expect-head is usage", args: append(append([]string{}, pins...), "--expect-head", "sha256:xyz", export("closed-basic")), wantExit: 2},
		{name: "double dash ends the flags", args: append(append([]string{}, pins...), "--json", "--", export("closed-basic")), wantExit: 0, wantLevel: "verified", wantEnum: "verified"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errb := runEvidence(t, append([]string{"verify"}, tc.args...)...)
			if code != tc.wantExit {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, tc.wantExit, errb)
			}
			if tc.wantLevel == "" {
				if out != "" {
					t.Errorf("stdout = %q, want no report", out)
				}
				return
			}
			var rep struct {
				OK              bool     `json:"ok"`
				Level           string   `json:"level"`
				LevelEnum       string   `json:"level_enum"`
				WeakenedByFlags []string `json:"weakened_by_flags"`
			}
			if err := json.Unmarshal([]byte(out), &rep); err != nil {
				t.Fatalf("stdout is not one JSON object: %v: %q", err, out)
			}
			if rep.Level != tc.wantLevel || rep.LevelEnum != tc.wantEnum || len(rep.WeakenedByFlags) != tc.wantFlags || rep.OK != (tc.wantExit == 0) {
				t.Errorf("report = %+v, want level %q enum %q flags %d ok %v", rep, tc.wantLevel, tc.wantEnum, tc.wantFlags, tc.wantExit == 0)
			}
		})
	}
}

func TestRunEvidence_TextReport(t *testing.T) {
	code, out, _ := runEvidence(t, "verify", "--runner-key", key("runner.pub"), "--allow-unanchored", export("closed-basic"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"level: verified, unanchored", "a pass with an allowance: unanchored", "result: verified, unanchored"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("text report lacks %q:\n%s", want, out)
		}
	}
}

func TestRunEvidence_SymlinkedPathWithTrailingSeparator(t *testing.T) {
	link := filepath.Join(t.TempDir(), "link")
	abs, err := filepath.Abs(export("closed-basic"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(abs, link); err != nil {
		t.Skipf("no symbolic links: %v", err)
	}
	for _, p := range []string{link, link + "/", link + "//", link + "/."} {
		code, out, _ := runEvidence(t, "verify", "--skip-signatures", "--json", p)
		if code != 3 || out != "" {
			t.Errorf("verify %q = exit %d, stdout %q, want exit 3 and no report", p, code, out)
		}
	}
}

func TestRunEvidence_Subcommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "no subcommand", args: nil, want: cli.ExitUsage},
		{name: "unknown subcommand", args: []string{"export"}, want: cli.ExitUsage},
		{name: "help", args: []string{"verify", "--help"}, want: cli.ExitOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, _ := runEvidence(t, tc.args...); code != tc.want {
				t.Errorf("exit code = %d, want %d", code, tc.want)
			}
		})
	}
}

func TestRoot_Evidence_UnsetIsUnknown(t *testing.T) {
	var out, errb bytes.Buffer
	r := cli.Root{Stdout: &out, Stderr: &errb}
	if code := r.Run(context.Background(), []string{"evidence", "verify"}); code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d: a build with no evidence command keeps it unknown", code, cli.ExitUsage)
	}
}
