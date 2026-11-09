// Command ciexecutor-jenkins is the ciexecutor provider for Jenkins: a capability provider of numinance.ports.ciexecutor.v1. A plugin host
// launches it as a subprocess with -url, the base URL of the one Jenkins server it may reach; run by hand it refuses and says so. It
// asks the host for a credential handle and sends the handle to the egress proxy, so no user name or token is ever in this process.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ciexecutor-jenkins:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("ciexecutor-jenkins", flag.ContinueOnError)
	rawURL := fs.String("url", "", "base URL of the Jenkins server: https://host[:port][/context]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := provider.ParseURL(*rawURL)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	digest, err := ghprovider.ExecutableDigest()
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := ghprovider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "ciexecutor-jenkins:", derr)
			rt.Stop()
			return
		}
		transport := provider.Transport{RT: provider.NewLive(cfg, host.Routes())}
		provider.New(provider.PluginID, digest, cfg, transport, 3*time.Second, host, rt.Stop).Register(s)
	})
}
