//go:build darwin || linux

package main

import (
	"sync"

	"golang.org/x/sys/unix"
)

// vmsTerminalMode gives the host terminal on fd VMS's control keys for
// the time it is in its ordinary ("cooked") mode, which is whenever a
// program, not the command line's editor, is reading or running (the
// editor, github.com/chzyer/readline, switches to raw mode only while it
// reads a command line, and back to this mode after). The terminal driver
// still edits lines, echoes, and turns keys into signals, but with VMS's
// keys:
//
//   - CTRL/C is the interrupt character (SIGINT), as before.
//   - CTRL/Y is the quit character (SIGQUIT), which govax takes as VMS's
//     CTRL/Y: it ends govax.
//   - CTRL/Z is the end-of-file character. Typed at the start of a line, a
//     read returns nothing, which attentionStdin turns into VMS's CTRL/Z.
//   - Nothing suspends govax: the host's suspend (CTRL/Z) and delayed
//     suspend (CTRL/Y, on macOS) characters are off.
//   - Control characters aren't echoed as ^X: govax echoes VMS's
//     *Interrupt* and *Exit* itself.
//
// It returns a func that puts the terminal back as it was (safe to call
// more than once), and false, with nothing changed, if fd isn't a
// terminal.
func vmsTerminalMode(fd int) (restore func(), ok bool) {
	saved, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return func() {}, false
	}

	t := *saved
	t.Cc[unix.VINTR] = charCtrlC
	t.Cc[unix.VQUIT] = charCtrlY
	t.Cc[unix.VEOF] = charCtrlZ
	t.Cc[unix.VSUSP] = posixVDisable
	disableDelayedSuspend(&t)

	t.Lflag |= unix.ISIG
	t.Lflag &^= unix.ECHOCTL

	if err := unix.IoctlSetTermios(fd, ioctlSetTermios, &t); err != nil {
		return func() {}, false
	}

	var once sync.Once

	return func() {
		once.Do(func() { _ = unix.IoctlSetTermios(fd, ioctlSetTermios, saved) })
	}, true
}
