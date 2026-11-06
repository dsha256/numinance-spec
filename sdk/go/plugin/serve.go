package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"

	"google.golang.org/grpc"
)

// Runtime is what Serve tells a provider about its launch.
type Runtime struct {
	// HostAddr is the gRPC target of the host's HostService, "" when the host passed none.
	HostAddr string
	// Stop ends Serve; PluginService.Shutdown calls it after it answers.
	Stop func()
}

// ErrNotLaunchedByHost is returned by Serve when the cookie is absent or wrong: the binary was started by hand.
var ErrNotLaunchedByHost = errors.New("this binary is a Numinance provider and is started by the plugin host")

// Serve runs a provider launched by the host. It checks CookieEnv against MagicCookie, picks the highest app
// protocol both sides speak (ProtocolsEnv against AppProtocolVersion), listens on a Unix socket inside a new 0700
// directory, prints the handshake line to out and serves the gRPC server register fills until ctx ends or
// Runtime.Stop is called. The socket directory is removed on return.
func Serve(ctx context.Context, getenv func(string) string, out io.Writer, register func(*grpc.Server, Runtime)) error {
	if getenv(CookieEnv) != MagicCookie {
		return ErrNotLaunchedByHost
	}
	versions, err := ParseProtocols(getenv(ProtocolsEnv))
	if err != nil {
		return err
	}
	if !slices.Contains(versions, AppProtocolVersion) {
		return fmt.Errorf("the host serves no protocol version this provider speaks (%d)", AppProtocolVersion)
	}
	dir, err := os.MkdirTemp("", "nump")
	if err != nil {
		return fmt.Errorf("socket directory: %w", err)
	}
	// Announce the canonical name: a host refuses an address that passes through a symlink (macOS temporary directories do).
	if canonical, rerr := filepath.EvalSymlinks(dir); rerr == nil {
		dir = canonical
	}
	defer func() { _ = os.RemoveAll(dir) }()     // best effort: the socket directory is private and temporary
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // a directory needs the search bit; 0700 is the owner-only mode
		return fmt.Errorf("socket directory mode: %w", err)
	}
	path := filepath.Join(dir, "p.sock")
	lis, err := (&net.ListenConfig{}).Listen(ctx, NetworkUnix, path)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srv := grpc.NewServer()
	register(srv, Runtime{HostAddr: getenv(HostAddrEnv), Stop: cancel})
	line := Line{Core: CoreProtocolVersion, App: AppProtocolVersion, Network: NetworkUnix, Address: path, Protocol: ProtocolGRPC}
	if _, err := fmt.Fprintln(out, line.String()); err != nil {
		return fmt.Errorf("handshake line: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(lis) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		srv.GracefulStop()
		return nil
	}
}
