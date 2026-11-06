// Command scm-github-replay is the replaying build of the scm-github provider: the same provider over recorded, redacted
// exchanges (package replay). It never opens a connection; a host launches it in the conformance kit and the host-level
// acceptance, where CI has no network and no token.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/provider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/replay"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// files is the repeatable -cassette flag.
type files []string

func (f *files) String() string { return strings.Join(*f, ",") }

func (f *files) Set(v string) error { *f = append(*f, v); return nil }

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "scm-github-replay:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var extra files
	fs := flag.NewFlagSet("scm-github-replay", flag.ContinueOnError)
	fs.Var(&extra, "cassette", "repeatable: the absolute path of one more cassette, whose exchanges are matched exactly like the embedded ones")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	digest, err := ghprovider.ExecutableDigest()
	if err != nil {
		return err
	}
	transport, err := replay.Transport(extra...)
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := ghprovider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "scm-github-replay:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.ReplayPluginID, digest, transport, host, rt.Stop).Register(s)
	})
}
