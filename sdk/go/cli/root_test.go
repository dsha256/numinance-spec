package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

func TestRoot_Run_Dispatch(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
		out  string
	}{
		{name: "no command", args: nil, want: cli.ExitUsage, out: ""},
		{name: "unknown command", args: []string{"frob"}, want: cli.ExitUsage, out: ""},
		{name: "version", args: []string{"version"}, want: cli.ExitOK, out: "1.2.3\n"},
		{name: "validate", args: []string{"validate"}, want: cli.ExitUnavailable, out: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			r := cli.Root{Validate: cli.Validate{Source: oneDoc(goToolchain)}, Version: "1.2.3", Stdout: &out, Stderr: &errb}
			if got := r.Run(context.Background(), tc.args); got != tc.want {
				t.Errorf("Run(%v) = %d, want %d", tc.args, got, tc.want)
			}
			if tc.name == "validate" && !strings.Contains(errb.String(), "engine build") {
				t.Errorf("Run(%v) stderr = %q, want the engine-build message", tc.args, errb.String())
			}
			if tc.out != "" && out.String() != tc.out {
				t.Errorf("stdout = %q, want %q", out.String(), tc.out)
			}
		})
	}
}
