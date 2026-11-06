//go:build unix

package replay_test

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/numinance/numinance-spec/plugins/official/scm-github/replay"
)

// A FIFO is refused without being opened, so a launch cannot be made to block on it.
func TestTransport_RefusesAFifoWithoutBlocking(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "fifo.json")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skipf("no FIFO here: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := replay.Transport(p); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("Transport(a FIFO) error = nil, want a refusal")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Transport(a FIFO) blocked")
	}
}
