package main

import "golang.org/x/sys/unix"

// The terminal-mode requests, and the value that turns a control
// character off (_POSIX_VDISABLE), on Linux.
const (
	ioctlGetTermios = unix.TCGETS
	ioctlSetTermios = unix.TCSETS
	posixVDisable   = 0
)

// disableDelayedSuspend does nothing: Linux has no DSUSP character.
func disableDelayedSuspend(*unix.Termios) {}
