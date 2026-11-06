package cli

import (
	"context"
	"fmt"
	"io"
)

// Root dispatches the numi subcommands.
type Root struct {
	Validate Validate
	// Evidence runs "numi evidence ..." with the arguments after "evidence". The
	// open build sets it to RunEvidence. When nil the command is unknown, so a
	// build that has its own evidence command is unaffected.
	Evidence func(ctx context.Context, args []string, stdout, stderr io.Writer) int
	// Version is printed by "numi version".
	Version string
	// Stdout and Stderr receive the output and the errors; nil discards.
	Stdout io.Writer
	Stderr io.Writer
}

// Run runs the subcommand named by args[0] and returns the exit code.
func (r Root) Run(ctx context.Context, args []string) int {
	if r.Stdout == nil {
		r.Stdout = io.Discard
	}
	if r.Stderr == nil {
		r.Stderr = io.Discard
	}
	if len(args) == 0 {
		usage := "usage: numi <validate|version> [args]"
		if r.Evidence != nil {
			usage = "usage: numi <validate|evidence|version> [args]"
		}
		_, _ = fmt.Fprintln(r.Stderr, usage)
		return ExitUsage
	}
	switch args[0] {
	case "validate":
		v := r.Validate
		v.Stdout, v.Stderr = r.Stdout, r.Stderr
		return v.Run(ctx, args[1:])
	case "evidence":
		if r.Evidence != nil {
			return r.Evidence(ctx, args[1:], r.Stdout, r.Stderr)
		}
	case "version":
		_, _ = fmt.Fprintln(r.Stdout, r.Version)
		return ExitOK
	}
	_, _ = fmt.Fprintf(r.Stderr, "numi: unknown command \"%s\"\n", clean(args[0], maxFieldRunes))
	return ExitUsage
}
