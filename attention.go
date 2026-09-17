package main

import (
	"io"
	"os"
	"os/signal"

	"github.com/tucats/govax/internal/cpu"
)

// charCtrlC is the byte a terminal delivers for Ctrl-C once the terminal's
// own ISIG-driven SIGINT generation is disabled — the case while
// readline.NewEx's Instance.Readline is actually blocked reading a line:
// github.com/chzyer/readline puts the terminal into raw mode (ISIG off,
// among other things) only for the duration of that call (Operation.Runes,
// entered/exited around each Readline, not held for the life of the
// process). attentionStdin (below) is what makes Ctrl-C interrupt a
// running VAX program during that window. The rest of the time — notably
// while runLoop (internal/console/execute.go) drives Engine.Step
// synchronously for a GO/CALL/STEP command, since that's not a Readline
// call at all — the terminal is back in ordinary cooked mode with ISIG
// on, so Ctrl-C generates a real SIGINT instead; installSigintAttention
// (below) is what catches that case, since Go's default SIGINT
// disposition with no handler installed is to terminate the process.
const charCtrlC = 0x03

// attentionStdin wraps the real terminal, watching every byte read from it
// for Ctrl-C and treating that specially: instead of passing 0x03 through
// as ordinary input, it calls Attention on whatever Engine getEngine
// currently returns and drops the byte — the same thing a real terminal's
// ISIG would do (consume the interrupt character, never deliver it as
// data), reimplemented here in software because readline has already
// disabled ISIG for its own line-editing purposes (see charCtrlC's own
// comment). getEngine is a func, not a *cpu.Engine, because Console.Engine
// is replaced wholesale by INIT/ZERO/VMINIT (see internal/console/init.go/
// vminit.go) — capturing a snapshot at construction time would go stale
// the moment any of those ran.
//
// A single background goroutine (pump) owns the real read side of the
// terminal for the life of the process. This matters because
// Engine.Step's own loop (GO/CALL/STEP, internal/console/execute.go) never
// itself calls Read while a VAX program is executing, yet Ctrl-C still
// needs to be noticed promptly during exactly that loop — nothing else is
// reading the terminal at that moment otherwise. Both of the process's own
// consumers of terminal input -- readline's line editor, idle at the "VAX>
// "prompt, and Console.ConsoleReadByte (XFC$CONSOLE_READ), when a running
// VAX program wants console input -- read the already-filtered bytes this
// produces instead of the real os.Stdin directly, so there is still only
// ever one real reader of the terminal, avoiding the lost- or
// stolen-keystroke races two independent readers on the same fd would
// otherwise risk.
type attentionStdin struct {
	bytes chan byte
	done  chan struct{}
}

// newAttentionStdin starts the background pump and returns the wrapper.
// r is the real terminal (os.Stdin); getEngine is consulted fresh on every
// Ctrl-C, never cached.
func newAttentionStdin(r io.Reader, getEngine func() *cpu.Engine) *attentionStdin {
	s := &attentionStdin{
		// Buffered generously so an ordinary keystroke or two typed with
		// nobody currently reading (e.g. just before a GO command's own
		// Step loop starts pulling from ConsoleReadByte) never blocks the
		// pump goroutine's own next Read call -- that would delay
		// noticing a Ctrl-C typed right after.
		bytes: make(chan byte, 4096),
		done:  make(chan struct{}),
	}

	go s.pump(r, getEngine)

	return s
}

func (s *attentionStdin) pump(r io.Reader, getEngine func() *cpu.Engine) {
	defer close(s.bytes)

	buf := make([]byte, 1)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			if buf[0] == charCtrlC {
				if e := getEngine(); e != nil {
					e.Attention()
				}
			} else {
				select {
				case s.bytes <- buf[0]:
				case <-s.done:
					return
				}
			}
		}

		if err != nil {
			return
		}
	}
}

// Read satisfies io.Reader, delivering one already-Ctrl-C-filtered byte per
// call regardless of len(p) -- matching Console.ConsoleReadByte's own
// single-byte reads, and perfectly usable by readline's bufio.Reader (which
// tolerates any short read).
func (s *attentionStdin) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	b, ok := <-s.bytes
	if !ok {
		return 0, io.EOF
	}

	p[0] = b

	return 1, nil
}

// Close stops delivering buffered bytes to a blocked Read (returning io.EOF
// to it instead) and lets the pump goroutine exit on its next iteration.
// The real underlying reader is deliberately left open -- the pump
// goroutine's own in-flight Read on it only returns via real EOF/closure or
// process exit, and since Close is only ever called via readline's own
// shutdown path right before the process exits anyway (see run's `defer
// rl.Close()`), leaking that one goroutine until exit is an accepted
// tradeoff, not an oversight.
func (s *attentionStdin) Close() error {
	close(s.done)

	return nil
}

// installSigintAttention registers a real os/signal handler for SIGINT,
// covering the window attentionStdin's own byte-level Ctrl-C filtering
// can't: whenever the terminal is in ordinary cooked mode rather than
// blocked inside a readline.Instance.Readline call (see charCtrlC's own
// doc comment for why that's most of the time a GO/CALL/STEP command is
// actually running), Ctrl-C generates a real SIGINT rather than arriving
// as a 0x03 byte on stdin. Left uninstalled, that SIGINT has no handler
// and Go's default disposition terminates the whole process instead of
// just interrupting the running VAX program -- this is what was making
// Ctrl-C stop govax entirely rather than returning to the console prompt.
// getEngine is consulted fresh on every signal, never cached, matching
// attentionStdin's own pump for the same reason (Console.Engine is
// replaced wholesale by INIT/ZERO/VMINIT — see internal/console/init.go/
// vminit.go). Only meant to be called once, for real interactive use (see
// run's own in == nil check, matching where attentionStdin itself is
// constructed); the returned func stops the handler and should be
// deferred by the caller.
func installSigintAttention(getEngine func() *cpu.Engine) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		for range ch {
			if e := getEngine(); e != nil {
				e.Attention()
			}
		}
	}()

	return func() { signal.Stop(ch) }
}
