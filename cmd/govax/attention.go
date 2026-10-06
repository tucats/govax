package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/chzyer/readline"
	"github.com/tucats/govax/internal/cpu"
)

// The control keys, with the meanings VMS gives them:
//
//   - CTRL/C interrupts what is running and returns to the command line
//     that started it: the console's "$ " or the debugger's "DBG> ". At a
//     prompt it cancels the line being typed. (A program that enabled a
//     CTRL/C AST gets the AST instead; internal/cpu/attention.go.)
//   - CTRL/Y ends govax, returning to the host's shell, as cleanly as it
//     can: the command running is stopped, mounted volumes are dismounted,
//     and the terminal is put back. A second CTRL/Y, while govax is still
//     ending, exits at once. (A program that enabled a CTRL/Y AST gets the
//     AST instead.)
//   - CTRL/Z is end of file. On an empty line, the read that's waiting
//     gets the end of file, and *Exit* is echoed. After some text, it ends
//     the line, as RETURN does, and the end of file is the next read's.
//
// Both CTRL/C and CTRL/Y echo *Interrupt*, as VMS's terminal driver does.
//
// Keys reach govax two ways, depending on the terminal's mode. While the
// command line's editor (github.com/chzyer/readline) reads a line, the
// terminal is in raw mode and every key arrives as a byte (attentionStdin
// below sees them first). The rest of the time, while a program runs or
// reads, it is in its ordinary mode, which vmsTerminalMode
// (terminal_unix.go) has set up so that the terminal driver turns CTRL/C
// into SIGINT, CTRL/Y into SIGQUIT (installKeySignals catches both), and
// CTRL/Z into an end of file on the read.
const (
	charCtrlC = 0x03
	charCtrlY = 0x19
	charCtrlZ = 0x1A
)

// What VMS's terminal driver echoes for the keys.
const (
	echoInterrupt = "*Interrupt*"
	echoExit      = "*Exit*"
)

// errReadInterrupted is what a program's read of the terminal gets when
// CTRL/C or CTRL/Y ends it before a line is typed. The readers in
// internal/rms and internal/corevms take it as the end of the input; the
// program is stopped before its next instruction anyway (unless its AST
// took the key).
var errReadInterrupted = errors.New("terminal read interrupted")

// attentionStdin is the terminal as govax's readers see it. A single
// goroutine (pump) reads the real terminal for the life of the process
// and hands the bytes on through a channel, handling the control keys on
// the way, so that a key is noticed even while nothing reads: while a
// program computes, Engine.Step's loop never reads the terminal. Both
// readers, readline's line editor at a prompt and a program's terminal
// reads (Console.In: RMS's $GET, the terminal driver's $QIO, the console
// device), read this, so there is only ever one real reader of the
// terminal. readline reads through promptReader, which takes bytes only
// while readline is reading a command line, so that no keystroke goes to
// the wrong one: readline's own goroutine may still be waiting to read
// after a line ends (promptKeys.filter can end a line on a key readline
// didn't expect to), and must not take what a program's read is waiting
// for.
//
// getEngine is a func, not a *cpu.Engine, because Console.Engine is
// replaced wholesale by INIT/ZERO/VMINIT (see internal/console/init.go/
// vminit.go): a snapshot taken at construction would go stale.
type attentionStdin struct {
	bytes     chan byte
	done      chan struct{}
	closeOnce sync.Once

	out       io.Writer
	echoMu    sync.Mutex // one echo at a time (the pump's, a signal's)
	getEngine func() *cpu.Engine

	// vmsMode says the terminal is in vmsTerminalMode, so that an empty
	// read is CTRL/Z, not the end of the input.
	vmsMode bool

	// forceExit is called for a second CTRL/Y while govax is still
	// ending after the first.
	forceExit func()

	// atPrompt is true while readline has the terminal in raw mode to
	// read a command line (rawMode). promptOn is closed when it becomes
	// true, and promptOff when it becomes false; each is replaced by a
	// fresh one when the other is closed. mu guards them.
	atPrompt  atomic.Bool
	promptOn  chan struct{}
	promptOff chan struct{}

	// aborting is true once CTRL/Y has asked govax to end.
	aborting atomic.Bool

	// interrupt is closed to end a program's read that is waiting for
	// input (interruptRead), and replaced by a fresh one.
	mu        sync.Mutex
	interrupt chan struct{}
}

// newAttentionStdin starts the pump on r, the real terminal, and returns
// the reader. out is where the keys' echoes go; vmsMode is whether
// vmsTerminalMode set the terminal up.
func newAttentionStdin(r io.Reader, out io.Writer, vmsMode bool, getEngine func() *cpu.Engine, forceExit func()) *attentionStdin {
	s := &attentionStdin{
		// Buffered generously so that keys typed ahead, with nobody
		// reading yet, never hold up the pump: it must stay free to
		// notice a CTRL/C typed after them.
		bytes:     make(chan byte, 4096),
		done:      make(chan struct{}),
		out:       out,
		getEngine: getEngine,
		vmsMode:   vmsMode,
		forceExit: forceExit,
		interrupt: make(chan struct{}),
		promptOn:  make(chan struct{}),
		promptOff: make(chan struct{}),
	}

	close(s.promptOff) // not at a prompt

	go s.pump(r)

	return s
}

// pump reads the terminal until its input ends.
//
// In raw mode (at a prompt) each read returns the keys typed so far. In
// the terminal's ordinary mode, each returns a whole line, ending in a
// newline, except when CTRL/Z, the end-of-file character, ended it: then
// the line has no newline, and a CTRL/Z typed on an empty line returns
// nothing at all. pump turns each of those into VMS's CTRL/Z for the
// readers: the line's text and a CTRL/Z that ends it, then a CTRL/Z that
// is the next read's end of file; or just that one, echoing *Exit*.
func (s *attentionStdin) pump(r io.Reader) {
	defer close(s.bytes)

	buf := make([]byte, 4096)

	var lastEmpty time.Time

	for {
		n, err := r.Read(buf)
		prompt := s.atPrompt.Load()

		for _, b := range buf[:n] {
			if !s.key(b, prompt) {
				return
			}
		}

		if n > 0 && s.vmsMode && !prompt && buf[n-1] != '\n' && n < len(buf) {
			s.echo("")

			// One CTRL/Z ends the line; the other is the next read's.
			for range 2 {
				if !s.send(charCtrlZ) {
					return
				}
			}
		}

		if err == nil {
			continue
		}

		if !s.vmsMode || prompt || !errors.Is(err, io.EOF) {
			return
		}

		// A terminal that has gone away (the window closed) returns
		// nothing at once, again and again; nobody types CTRL/Z that fast.
		if time.Since(lastEmpty) < 10*time.Millisecond {
			return
		}

		lastEmpty = time.Now()

		s.echo(echoExit)

		if !s.send(charCtrlZ) {
			return
		}
	}
}

// key handles one byte typed, and reports whether the pump goes on.
func (s *attentionStdin) key(b byte, prompt bool) bool {
	switch {
	case b == charCtrlY && prompt:
		// readline echoes *Interrupt* for the CTRL/C that ends its read
		// (promptKeys.filter), so ctrlY doesn't.
		s.ctrlY(false)

		return s.send(charCtrlC)

	case b == charCtrlC && !prompt:
		// Only when the terminal isn't in vmsTerminalMode, which turns
		// CTRL/C into SIGINT outside the prompt.
		s.ctrlC()

		return true
	}

	return s.send(b)
}

// send hands b to the readers, and reports false if the reader is closed.
func (s *attentionStdin) send(b byte) bool {
	select {
	case s.bytes <- b:
		return true
	case <-s.done:
		return false
	}
}

// echo writes text, and a new line, to the terminal.
func (s *attentionStdin) echo(text string) {
	if s.out != nil {
		s.echoMu.Lock()
		fmt.Fprint(s.out, text+"\r\n")
		s.echoMu.Unlock()
	}
}

// ctrlC is CTRL/C typed while no command line is being read: the program
// running, if any, is asked to stop (Engine.AttentionKey), and a read
// waiting for a line ends.
func (s *attentionStdin) ctrlC() {
	s.echo(echoInterrupt)

	if e := s.getEngine(); e != nil {
		e.AttentionKey(cpu.AttentionCtrlC)
	}

	s.interruptRead()
}

// ctrlY is CTRL/Y: govax is to end. The program running, if any, is asked
// to stop, and a read waiting for a line ends; the command loop in run
// then ends (abortRequested). A second CTRL/Y before the first was dealt
// with exits at once.
func (s *attentionStdin) ctrlY(echo bool) {
	e := s.getEngine()

	if s.aborting.Swap(true) && (e == nil || e.PendingAttention() == cpu.AttentionCtrlY) {
		s.forceExit()

		return
	}

	if echo {
		s.echo(echoInterrupt)
	}

	if e != nil {
		e.AttentionKey(cpu.AttentionCtrlY)
	}

	s.interruptRead()
}

// interruptRead ends the program's read that is waiting for a line, if
// there is one.
func (s *attentionStdin) interruptRead() {
	s.mu.Lock()
	close(s.interrupt)
	s.interrupt = make(chan struct{})
	s.mu.Unlock()
}

// abortRequested reports whether CTRL/Y has asked govax to end. The
// command loop asks after each command, on the goroutine that runs the
// engine. A CTRL/Y a program's AST took (the key isn't pending, and it
// didn't stop the run) doesn't end govax, and is forgotten.
func (s *attentionStdin) abortRequested() bool {
	if !s.aborting.Load() {
		return false
	}

	e := s.getEngine()
	if e == nil || e.PendingAttention() == cpu.AttentionCtrlY || e.StoppedBy() == cpu.AttentionCtrlY {
		return true
	}

	s.aborting.Store(false)

	return false
}

// Read is the programs' read of the terminal (Console.In), delivering one
// byte per call regardless of len(p), as Console.ConsoleReadByte reads
// them. A read waiting for input ends with errReadInterrupted when CTRL/C
// or CTRL/Y is typed (but not at a prompt, where the terminal is
// readline's).
func (s *attentionStdin) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	var interrupt chan struct{}

	if !s.atPrompt.Load() {
		s.mu.Lock()
		interrupt = s.interrupt
		s.mu.Unlock()
	}

	select {
	case b, ok := <-s.bytes:
		if !ok {
			return 0, io.EOF
		}

		p[0] = b

		return 1, nil

	case <-interrupt: // nil at a prompt: never ready
		return 0, errReadInterrupted

	case <-s.done:
		return 0, io.EOF
	}
}

// Close ends the readers' reads: a read waiting, or any later one, returns
// io.EOF, and the pump stops when it next has a byte to deliver. run calls
// it as govax ends, before readline's Close, which waits for readline's
// goroutine to finish a read it may be waiting in. The real terminal is
// left open: the pump's own read of it returns only with input, or at
// exit.
func (s *attentionStdin) Close() error {
	s.closeOnce.Do(func() { close(s.done) })

	return nil
}

// setPrompt records whether readline is reading a command line.
func (s *attentionStdin) setPrompt(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.atPrompt.Load() == on {
		return
	}

	s.atPrompt.Store(on)

	if on {
		close(s.promptOn)
		s.promptOff = make(chan struct{})
	} else {
		close(s.promptOff)
		s.promptOn = make(chan struct{})
	}
}

// rawMode is readline's raw-mode switch (Config.FuncMakeRaw and
// FuncExitRaw), wrapped to keep atPrompt: true from just before the
// terminal goes raw until just after it is back in its ordinary mode.
func (s *attentionStdin) rawMode() (enter, exit func() error) {
	var rm readline.RawMode

	enter = func() error {
		s.setPrompt(true)

		return rm.Enter()
	}

	exit = func() error {
		err := rm.Exit()
		s.setPrompt(false)

		return err
	}

	return enter, exit
}

// promptReader is readline's side of attentionStdin (Config.Stdin): its
// reads take bytes only while readline is reading a command line, and
// wait otherwise, until the next one or Close.
type promptReader struct{ s *attentionStdin }

// promptReader returns readline's reader.
func (s *attentionStdin) promptReader() io.ReadCloser { return promptReader{s} }

func (r promptReader) Read(p []byte) (int, error) {
	s := r.s

	if len(p) == 0 {
		return 0, nil
	}

	for {
		s.mu.Lock()
		on, off, at := s.promptOn, s.promptOff, s.atPrompt.Load()
		s.mu.Unlock()

		if !at {
			select {
			case <-on:
				continue
			case <-s.done:
				return 0, io.EOF
			}
		}

		select {
		case b, ok := <-s.bytes:
			if !ok {
				return 0, io.EOF
			}

			p[0] = b

			return 1, nil

		case <-off:
		case <-s.done:
			return 0, io.EOF
		}
	}
}

// Close is attentionStdin's: readline's Close doesn't call it (see run).
func (r promptReader) Close() error { return nil }

// installKeySignals catches the signals the terminal driver makes of
// CTRL/C (SIGINT) and CTRL/Y (SIGQUIT) in its ordinary mode (see
// vmsTerminalMode), while a program runs or reads. Uncaught, either would
// end govax at once (SIGQUIT with a dump of every goroutine). The
// returned func stops catching them.
func installKeySignals(s *attentionStdin) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGQUIT)

	go func() {
		for sig := range ch {
			if sig == syscall.SIGQUIT {
				s.ctrlY(true)
			} else {
				s.ctrlC()
			}
		}
	}()

	return func() { signal.Stop(ch) }
}

// promptKeys gives the command line's editor (readline, in raw mode) the
// VMS keys: CTRL/C cancels the line, echoing *Interrupt*; CTRL/Z on an
// empty line is end of file, echoing *Exit*; CTRL/Z after some text ends
// the line, and the next read is end of file. (CTRL/Y never reaches the
// editor: attentionStdin turns it into a CTRL/C, having asked govax to
// end.) It does this through readline's hooks, on readline's goroutine;
// the command loop reads the results after Readline returns.
type promptKeys struct {
	cfg *readline.Config

	// lineLength is the length of the line being typed, as of the last
	// key (Listener).
	lineLength atomic.Int32

	// eof is set when CTRL/Z ended the read on an empty line; eofNext,
	// when it ended a line of text.
	eof, eofNext atomic.Bool
}

// newPromptKeys installs the keys' handling in cfg, which must be the
// Config that is passed to readline.NewEx.
func newPromptKeys(cfg *readline.Config) *promptKeys {
	k := &promptKeys{cfg: cfg}

	cfg.InterruptPrompt = echoInterrupt
	cfg.FuncFilterInputRune = k.filter
	cfg.Listener = readline.FuncListener(func(line []rune, _ int, _ rune) ([]rune, int, bool) {
		k.lineLength.Store(int32(len(line)))

		return nil, 0, false
	})

	return k
}

// filter is readline's FuncFilterInputRune: it sees each key before
// readline does. readline ends a read with ErrInterrupt for CTRL/C, after
// echoing cfg.InterruptPrompt, which filter sets to the key's echo.
func (k *promptKeys) filter(r rune) (rune, bool) {
	switch r {
	case charCtrlC:
		k.cfg.InterruptPrompt = echoInterrupt

	case charCtrlZ:
		if k.lineLength.Load() > 0 {
			k.eofNext.Store(true)

			return readline.CharEnter, true
		}

		k.eof.Store(true)
		k.cfg.InterruptPrompt = echoExit

		return readline.CharInterrupt, true
	}

	return r, true
}

// endOfFile reports whether CTRL/Z ended the read that just returned err.
func (k *promptKeys) endOfFile(err error) bool {
	return errors.Is(err, readline.ErrInterrupt) && k.eof.Swap(false)
}

// eofPending reports, once, whether the read about to start is end of
// file because CTRL/Z ended the last line.
func (k *promptKeys) eofPending() bool { return k.eofNext.Swap(false) }
