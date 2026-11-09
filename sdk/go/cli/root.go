package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Root dispatches the numi subcommands.
type Root struct {
	Validate Validate
	// Evidence runs "numi evidence ..." with the arguments after "evidence". The
	// open build sets it to RunEvidence. When nil the command is unknown, so a
	// build that has its own evidence command is unaffected.
	Evidence func(ctx context.Context, args []string, stdout, stderr io.Writer) int
	// Init runs "numi init ..." with the arguments after "init". Both builds set it to RunInit. When nil the
	// command is unknown.
	Init func(ctx context.Context, args []string, stdout, stderr io.Writer) int
	// Suggest runs "numi suggest ..." with the arguments after "suggest". Both builds set it to RunSuggest. When nil
	// the command is unknown.
	Suggest func(ctx context.Context, args []string, stdout, stderr io.Writer) int
	// Extension runs "numi extension ..." with the arguments after "extension". The open build sets it to RunExtension
	// (offline verification of the lockfile pins). When nil the command is unknown.
	Extension func(ctx context.Context, args []string, stdout, stderr io.Writer) int
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
		names := []string{"validate"}
		if r.Init != nil {
			names = append(names, "init")
		}
		if r.Suggest != nil {
			names = append(names, "suggest")
		}
		if r.Evidence != nil {
			names = append(names, "evidence")
		}
		if r.Extension != nil {
			names = append(names, "extension")
		}
		names = append(names, "version")
		_, _ = fmt.Fprintf(r.Stderr, "usage: numi <%s> [args]\n", strings.Join(names, "|"))
		return ExitUsage
	}
	switch args[0] {
	case "validate":
		v := r.Validate
		v.Stdout, v.Stderr = r.Stdout, r.Stderr
		return v.Run(ctx, args[1:])
	case "init":
		if r.Init != nil {
			return r.Init(ctx, args[1:], r.Stdout, r.Stderr)
		}
	case "suggest":
		if r.Suggest != nil {
			return r.Suggest(ctx, args[1:], r.Stdout, r.Stderr)
		}
	case "evidence":
		if r.Evidence != nil {
			return r.Evidence(ctx, args[1:], r.Stdout, r.Stderr)
		}
	case "extension":
		if r.Extension != nil {
			return r.Extension(ctx, args[1:], r.Stdout, r.Stderr)
		}
	case "version":
		_, _ = fmt.Fprintln(r.Stdout, r.Version)
		return ExitOK
	}
	_, _ = fmt.Fprintf(r.Stderr, "numi: unknown command \"%s\"\n", clean(args[0], maxFieldRunes))
	return ExitUsage
}
