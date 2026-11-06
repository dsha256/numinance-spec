// Command verifier-repo is the verifier provider for repository-level static checks: a capability provider of
// numinance.ports.verifier.v1 that checks the scope of a change, its Markdown and the `just` recipes its documents name. It
// runs no candidate code. A plugin host launches it as a subprocess; run by hand it refuses and says so.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/provider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verifier-repo:", err)
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
			fmt.Fprintln(os.Stderr, "verifier-repo:", derr)
			rt.Stop()
			return
		}
		provider.New(digest, host, rt.Stop).Register(s)
	})
}
