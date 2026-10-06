package main

import "golang.org/x/sys/unix"

// The terminal-mode requests, and the value that turns a control
// character off (_POSIX_VDISABLE), on macOS.
const (
	ioctlGetTermios = unix.TIOCGETA
	ioctlSetTermios = unix.TIOCSETA
	posixVDisable   = 0xff
)

// disableDelayedSuspend turns off macOS's DSUSP character, which is
// CTRL/Y by default: typed while a program reads, it would stop govax.
func disableDelayedSuspend(t *unix.Termios) { t.Cc[unix.VDSUSP] = posixVDisable }
