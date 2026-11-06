//go:build unix

package provider

import (
	"os/exec"
	"syscall"
	"time"
)

// isolate puts the tool in its own process group and kills the whole group at the deadline, so a child the tool started does not
// outlive the call; WaitDelay bounds the wait for the output pipes of a process that ignores the kill.
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
}
