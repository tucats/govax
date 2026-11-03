//go:build darwin || linux

package main

import (
	"errors"
	"io"
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
// terminal. hostMode gives a host program the terminal as govax found it,
// with the host's keys, until the func it returns puts govax's back.
func vmsTerminalMode(fd int) (restore func(), hostMode func() (back func()), ok bool) {
	unchanged := func() (back func()) { return func() {} }

	saved, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return func() {}, unchanged, false
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
		return func() {}, unchanged, false
	}

	var once sync.Once

	restore = func() {
		once.Do(func() { _ = unix.IoctlSetTermios(fd, ioctlSetTermios, saved) })
	}

	hostMode = func() (back func()) {
		_ = unix.IoctlSetTermios(fd, ioctlSetTermios, saved)

		return func() { _ = unix.IoctlSetTermios(fd, ioctlSetTermios, &t) }
	}

	return restore, hostMode, true
}

// terminalInput reads the terminal for attentionStdin's pump, in a way that can
// stop so that a host program gets the keys. A plain read would not do: the pump
// is nearly always waiting in one, and that read would take a key from the
// program as readily as any other reader on the terminal, and only a key typed
// could end it. So terminalInput waits with select(2), which takes nothing, and
// reads only what select said is there.
type terminalInput struct {
	fd int

	// mu is held for each read, and paused is changed under it, so that when
	// pause has returned no read is going on and none will start. resumed is
	// closed by resume, for a Read waiting at pause's request.
	mu      sync.Mutex
	paused  bool
	resumed chan struct{}
}

// newTerminalInput returns a reader of the terminal on fd.
func newTerminalInput(fd int) *terminalInput { return &terminalInput{fd: fd} }

// Read returns what the terminal has as soon as it has any. It behaves as the
// file's read does, but that it doesn't read while paused.
func (t *terminalInput) Read(p []byte) (int, error) {
	for {
		// select may see a key that the program it was paused for is about
		// to read, and may see it before a pause: ready is asked again below.
		if _, err := t.ready(nil); err != nil {
			return 0, err
		}

		n, read, err := t.readIfAllowed(p)
		if read {
			return n, err
		}
	}
}

// readIfAllowed reads, if the terminal isn't lent and still has something; read
// says whether it did. While the terminal is lent it waits for its return.
func (t *terminalInput) readIfAllowed(p []byte) (n int, read bool, err error) {
	t.mu.Lock()

	if t.paused {
		resumed := t.resumed
		t.mu.Unlock()
		<-resumed

		return 0, false, nil
	}

	defer t.mu.Unlock()

	// Nothing else reads now, so what is still there will be there for the read.
	ready, err := t.ready(&unix.Timeval{})
	if err != nil {
		return 0, true, err
	}

	if !ready {
		return 0, false, nil
	}

	for {
		n, err := unix.Read(t.fd, p)

		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case err != nil:
			return 0, true, err
		case n == 0:
			return 0, true, io.EOF
		}

		return n, true, nil
	}
}

// ready waits for the terminal to have something to read, as long as timeout
// says (nil is for as long as it takes), and says whether it has.
func (t *terminalInput) ready(timeout *unix.Timeval) (bool, error) {
	for {
		var fds unix.FdSet

		fds.Set(t.fd)

		n, err := unix.Select(t.fd+1, &fds, nil, nil, timeout)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			return false, err
		}

		return n > 0, nil
	}
}

// pause stops the reads. When it returns, the terminal's keys are left for a
// host program.
func (t *terminalInput) pause() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.paused = true
	t.resumed = make(chan struct{})
}

// resume starts the reads again.
func (t *terminalInput) resume() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.paused {
		t.paused = false

		close(t.resumed)
	}
}
