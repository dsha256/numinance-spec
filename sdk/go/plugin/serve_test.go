package plugin_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func TestServe_RefusesAWrongCookie(t *testing.T) {
	var out bytes.Buffer
	err := plugin.Serve(t.Context(), func(string) string { return "nope" }, &out, func(*grpc.Server, plugin.Runtime) {})
	if !errors.Is(err, plugin.ErrNotLaunchedByHost) || out.Len() != 0 {
		t.Errorf("Serve(wrong cookie) = %v with output %q, want ErrNotLaunchedByHost and no output", err, out.String())
	}
}

func TestServe_RefusesProtocolsWithoutItsOwn(t *testing.T) {
	env := map[string]string{plugin.CookieEnv: plugin.MagicCookie, plugin.ProtocolsEnv: "2"}
	err := plugin.Serve(t.Context(), func(k string) string { return env[k] }, &bytes.Buffer{}, func(*grpc.Server, plugin.Runtime) {})
	if err == nil {
		t.Error("Serve(protocols 2) error = nil, want an error")
	}
}

func TestServe_PrintsTheLineAndStopsOnStop(t *testing.T) {
	env := map[string]string{plugin.CookieEnv: plugin.MagicCookie, plugin.ProtocolsEnv: "1", plugin.HostAddrEnv: "unix:/h"}
	var (
		out  bytes.Buffer
		rt   plugin.Runtime
		done = make(chan error, 1)
		up   = make(chan struct{})
	)
	go func() {
		done <- plugin.Serve(t.Context(), func(k string) string { return env[k] }, &out, func(_ *grpc.Server, r plugin.Runtime) { rt = r; close(up) })
	}()
	<-up
	rt.Stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() = %v, want nil after Stop", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve() did not return after Stop")
	}
	line, err := plugin.ParseLine(strings.TrimSuffix(out.String(), "\n"))
	if err != nil || line.App != plugin.AppProtocolVersion {
		t.Fatalf("handshake line %q = %+v, %v, want a valid line", out.String(), line, err)
	}
	if rt.HostAddr != "unix:/h" {
		t.Errorf("Runtime.HostAddr = %q, want unix:/h", rt.HostAddr)
	}
	if _, err := os.Stat(line.Address); !os.IsNotExist(err) {
		t.Errorf("socket %s after Serve returned: %v, want removed", line.Address, err)
	}
}
