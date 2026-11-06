// Command scm-github-record is the recording build of the scm-github provider: the live provider (handle, egress proxy,
// no token in the process) whose HTTP client also writes every exchange, redacted, to a cassette file that the replay build
// can later serve. A plugin host launches it with its arguments (-out, -rename); run by hand it refuses.
//
// A recording is made only against an operator's own repository and written outside the tree; its redaction (renamed names,
// credential patterns and e-mail addresses replaced, no header recorded) is described in the file and checked by a person
// before it is committed (README, "Recording cassettes").
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/scm-github/provider"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

type renames []ghprovider.Rename

func (r *renames) String() string { return fmt.Sprint([]ghprovider.Rename(*r)) }

func (r *renames) Set(v string) error {
	rn, err := ghprovider.ParseRename(v)
	if err != nil {
		return err
	}
	*r = append(*r, rn)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "scm-github-record:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var rn renames
	fs := flag.NewFlagSet("scm-github-record", flag.ContinueOnError)
	out := fs.String("out", "", "required: the absolute path of the cassette file to write; its directory must exist")
	fs.Var(&rn, "rename", "from=to, repeatable: a name replaced everywhere in the recording (repository, owner, login)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !filepath.IsAbs(*out) {
		return errors.New("-out must be an absolute path")
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
			fmt.Fprintln(os.Stderr, "scm-github-record:", derr)
			rt.Stop()
			return
		}
		rec := &ghprovider.Recorder{Inner: ghprovider.NewLive(host.Routes()), Out: *out, Redactor: ghprovider.Redactor{Renames: rn}}
		provider.New(provider.PluginID, digest, ghprovider.Transport{RT: rec}, host, rt.Stop).Register(s)
	})
}
