// Command numi is the open build of the Numinance CLI. It validates kind
// documents against their schemas; profile resolution needs the engine build,
// so "numi validate" on a repository ends with exit code 7 after the schema
// pass, while "numi validate --files" checks example documents only and exits 0
// when they all pass. "numi evidence verify DIR" checks an evidence export
// offline with the open verifier, and reaches "verified" only with keys the
// caller pins.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

// version is set at release time with -ldflags "-X main.version=x.y.z".
var version = "0.0.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := root(version).Run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

func root(version string) cli.Root {
	return cli.Root{
		Validate: cli.Validate{Source: workingTree{}, Files: fileTree{}},
		Evidence: cli.RunEvidence,
		Version:  version,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
}
