// Command ci-github-actions-replay is the replaying build of the ci-github-actions provider: the same provider over recorded, redacted
// exchanges (package replay). It never opens a connection; a host launches it in the conformance kit and the host-level
// acceptance, where CI has no network and no token.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/provider"
	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/replay"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ci-github-actions-replay:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	digest, err := ghprovider.ExecutableDigest()
	if err != nil {
		return err
	}
	transport, err := replay.Transport()
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := ghprovider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "ci-github-actions-replay:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.ReplayPluginID, digest, transport, 0, host, rt.Stop).Register(s)
	})
}
