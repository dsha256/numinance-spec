// Command ticketing-jira-replay is the replaying build of the ticketing-jira provider: the same provider over a recorded,
// redacted cassette (package replay). It never opens a connection and holds no credential; a host launches it in the
// conformance kit and the host-level acceptance, where CI has no network and no token. Its manifest declares no network
// destination and no credential intent, and the dev environment only.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/replay"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ticketing-jira-replay:", err)
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
	transport, err := replay.Transport()
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := ghprovider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "ticketing-jira-replay:", derr)
			rt.Stop()
			return
		}
		srv, nerr := provider.New(provider.ReplayPluginID, replay.Host, digest, transport, host, rt.Stop)
		if nerr != nil {
			fmt.Fprintln(os.Stderr, "ticketing-jira-replay:", nerr)
			rt.Stop()
			return
		}
		srv.Register(s)
	})
}
