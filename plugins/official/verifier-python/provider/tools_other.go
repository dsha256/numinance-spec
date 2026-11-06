//go:build !unix

package provider

import (
	"os/exec"
	"time"
)

// isolate bounds the wait for the output pipes of a process that ignores the kill; there is no process group to kill here.
func isolate(cmd *exec.Cmd) { cmd.WaitDelay = 5 * time.Second }
