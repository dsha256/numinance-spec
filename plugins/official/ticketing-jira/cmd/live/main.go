// Command ticketing-jira is the live build of the ticketing-jira provider: a capability provider of numinance.ports.ticketing.v1
// for one Jira Cloud instance. A plugin host launches it as a subprocess with -host, the exact host name of the one instance it
// may reach; run by hand it refuses and says so. It asks the host for a credential handle and sends every request through the
// egress proxy with that handle, so no e-mail address, token or Authorization header is ever in this process. Without
// -write-back it sends reads only (ADR 0060 decision 9).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ticketing-jira:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("ticketing-jira", flag.ContinueOnError)
	instance := fs.String("host", "", "exact host name of the Jira Cloud instance: lower-case DNS name, no scheme, port or path")
	writeBack := fs.Bool("write-back", false, "let the provider send writes (comment, transition, link); off, it sends reads only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := provider.ValidHost(*instance); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	digest, err := provider.ExecutableDigest()
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := ghprovider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "ticketing-jira:", derr)
			rt.Stop()
			return
		}
		transport := provider.Transport{RT: provider.NewLive(*instance, host.Routes(), *writeBack)}
		srv, nerr := provider.New(provider.PluginID, *instance, digest, transport, host, rt.Stop)
		if nerr != nil {
			fmt.Fprintln(os.Stderr, "ticketing-jira:", nerr)
			rt.Stop()
			return
		}
		srv.Register(s)
	})
}
