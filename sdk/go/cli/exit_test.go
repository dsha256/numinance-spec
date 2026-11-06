package cli_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

func TestExitCode_ForEveryKind(t *testing.T) {
	tests := []struct {
		kind cli.Kind
		want int
	}{
		{kind: cli.KindNone, want: 0},
		{kind: cli.KindInternal, want: 1},
		{kind: cli.KindInvalid, want: 3},
		{kind: cli.KindNotFound, want: 4},
		{kind: cli.KindConflict, want: 5},
		{kind: cli.KindPolicyDenied, want: 6},
		{kind: cli.KindUnavailable, want: 7},
		{kind: cli.KindTimeout, want: 8},
		{kind: cli.KindUsage, want: 2},
		{kind: cli.Kind(99), want: 1},
		{kind: cli.Kind(-1), want: 1},
	}
	for _, tc := range tests {
		if got := cli.ExitCode(tc.kind); got != tc.want {
			t.Errorf("ExitCode(%d) = %d, want %d", tc.kind, got, tc.want)
		}
	}
}

func TestDefaultErrorKind(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want cli.Kind
	}{
		{name: "nil", err: nil, want: cli.KindNone},
		{name: "typed", err: fmt.Errorf("wrap: %w", cli.Errorf(cli.KindConflict, "x")), want: cli.KindConflict},
		{name: "deadline", err: context.DeadlineExceeded, want: cli.KindTimeout},
		{name: "canceled", err: context.Canceled, want: cli.KindInternal},
		{name: "plain", err: errors.New("x"), want: cli.KindInternal},
	}
	for _, tc := range tests {
		if got := cli.DefaultErrorKind(tc.err); got != tc.want {
			t.Errorf("DefaultErrorKind(%s) = %d, want %d", tc.name, got, tc.want)
		}
	}
}
