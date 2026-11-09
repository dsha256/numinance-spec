// Command verifier-python is the verifier provider for the Python toolchain: a capability provider of
// numinance.ports.verifier.v1 over ruff, mypy, Semgrep and import-linter (pytest needs the nested sandbox of the port and is
// UNAVAILABLE here). A plugin host launches it as a subprocess; run by hand it refuses and says so.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verifier-python:", err)
		os.Exit(1)
	}
}

// dirsFlag collects the repeated -tool-dir values: absolute directories that hold tools outside any PATH.
type dirsFlag []string

func (d *dirsFlag) String() string { return fmt.Sprint([]string(*d)) }

func (d *dirsFlag) Set(v string) error {
	if !filepath.IsAbs(v) || filepath.Clean(v) != v {
		return fmt.Errorf("-tool-dir %q is not a clean absolute directory", v)
	}
	*d = append(*d, v)
	return nil
}

func run() error {
	var dirs dirsFlag
	fs := flag.NewFlagSet("verifier-python", flag.ContinueOnError)
	fs.Var(&dirs, "tool-dir", "an absolute directory that holds ruff, mypy or another tool when no PATH does (repeatable); a plugin host starts this provider with no PATH")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	digest, err := provider.ExecutableDigest()
	if err != nil {
		return err
	}
	return plugin.Serve(ctx, os.Getenv, os.Stdout, func(s *grpc.Server, rt plugin.Runtime) {
		host, derr := provider.DialHost(rt.HostAddr)
		if derr != nil {
			fmt.Fprintln(os.Stderr, "verifier-python:", derr)
			rt.Stop()
			return
		}
		provider.New(provider.PluginID, digest, provider.ExecRunner{Dirs: dirs}, host, rt.Stop).Register(s)
	})
}
