// Command ciexecutor-jenkins-replay is the replaying build of the ciexecutor-jenkins provider: the same provider over recorded, redacted
// exchanges (package replay). It never opens a connection; a host launches it in the conformance kit, where CI has no network and
// no credential. Its server is the made-up jenkins.example.test.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/replay"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ciexecutor-jenkins-replay:", err)
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
	cfg, err := provider.ParseURL(provider.ExampleURL)
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := ghprovider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "ciexecutor-jenkins-replay:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.ReplayPluginID, digest, cfg, transport, 0, host, rt.Stop).Register(s)
	})
}
