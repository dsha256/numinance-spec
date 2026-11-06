//go:build unix

package main

import "syscall"

// openFlags: never block on a FIFO swapped in after the directory listing, and
// never follow a link swapped in for the file.
const openFlags = syscall.O_NONBLOCK | syscall.O_NOFOLLOW
