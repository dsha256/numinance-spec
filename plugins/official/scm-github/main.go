// Command scm-github is the scm provider for GitHub: a capability provider of numinance.ports.scm.v1. A plugin host launches
// it as a subprocess; run by hand it refuses and says so. It asks the host for a credential handle and sends the handle to
// the egress proxy, so no token is ever in this process.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/provider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "scm-github:", err)
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
			fmt.Fprintln(os.Stderr, "scm-github:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.PluginID, digest, ghprovider.Transport{RT: ghprovider.NewLive(host.Routes())}, host, rt.Stop).Register(s)
	})
}
