// Command verifier-python-replay is the replaying build of the verifier-python provider: the same provider over the recorded
// tool output of the seeded candidates (package replay). It never starts a tool; a host launches it in the seeded-defect tests
// and the conformance kit, where CI has no network and no Python tools.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
	"github.com/numinance/numinance-spec/plugins/official/verifier-python/replay"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verifier-python-replay:", err)
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
			fmt.Fprintln(os.Stderr, "verifier-python-replay:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.ReplayPluginID, digest, replay.Runner{}, host, rt.Stop).Register(s)
	})
}
