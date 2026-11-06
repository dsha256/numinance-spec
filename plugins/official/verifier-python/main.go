// Command verifier-python is the verifier provider for the Python toolchain: a capability provider of
// numinance.ports.verifier.v1 over ruff, mypy, Semgrep and import-linter (pytest needs the nested sandbox of the port and is
// UNAVAILABLE here). A plugin host launches it as a subprocess; run by hand it refuses and says so.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verifier-python:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	digest, err := provider.ExecutableDigest()
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := provider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "verifier-python:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.PluginID, digest, provider.ExecRunner{}, host, rt.Stop).Register(s)
	})
}
