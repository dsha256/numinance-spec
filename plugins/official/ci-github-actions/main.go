// Command ci-github-actions is the ciexecutor provider for GitHub: a capability provider of numinance.ports.ciexecutor.v1. A plugin host launches
// it as a subprocess; run by hand it refuses and says so. It asks the host for a credential handle and sends the handle to
// the egress proxy, so no token is ever in this process.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/ci-github-actions/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ci-github-actions:", err)
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
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := ghprovider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "ci-github-actions:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.PluginID, digest, ghprovider.Transport{RT: ghprovider.NewLive(host.Routes())}, 3*time.Second, host, rt.Stop).Register(s)
	})
}
