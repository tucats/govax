package corevms

import (
	"bufio"
	"io"
	"slices"
	"time"

	"github.com/tucats/govax/internal/sched"
)

// The shared terminal (docs/PHASE-46.md, subtask 7): a process reading
// the terminal waits in the scheduler, not in a host read, so the other
// processes run while it waits for someone to type.
//
// # How a read waits
//
// Every process reads the one terminal through the same buffer
// (System.terminalReader), so nothing one process's read takes ahead is
// lost to another's. A read (a terminal $QIO read, RMS's $GET of the
// terminal, LIB$GET_INPUT) goes ahead only when a whole line is there to
// read: its terminator, CTRL/Z, as many characters as it wants, or the
// end of the input. Until then it waits, in LEF, and its service is
// called again when the line has come (its wait's test is the same
// question). Whether more input can be had without waiting is the input
// stream's to say (TerminalSource): govax's front end knows whether keys
// it has read from the host are waiting (cmd/govax/attention.go).
//
// The terminal gives its reads in the order they were made, as VMS's
// terminal driver queues them: a process whose read is behind another's
// waits for that one to finish, even if a line is there. A read's prompt
// is written when the read's turn comes, before anyone types its line.
//
// This is only with the scheduler on (vax.process.scheduler), and an
// input stream that is a TerminalSource. Otherwise a read blocks, as it
// always has: with one process there is nobody else to run.
//
// A $QIO read that must wait makes the $QIO itself wait (the request is
// made again when the line has come), rather than completing later: a
// program that issues a read and then waits for its event flag waits the
// same, but one that goes on to do other work before waiting can't.
// *Unconfirmed simplification.*

// TerminalSource is an input stream that can say whether a read would
// get something at once: a byte, or the end of the input.
type TerminalSource interface {
	Ready() bool
}

// terminalRead is one process's read in the terminal's queue.
type terminalRead struct {
	env      *Environment
	prompt   string
	prompted bool
}

// terminalReader returns the buffer every process reading src shares,
// making it on first use.
func (sys *System) terminalReader(src io.Reader) *bufio.Reader {
	if sys.terminalBuffers == nil {
		sys.terminalBuffers = map[io.Reader]*bufio.Reader{}
	}

	r, ok := sys.terminalBuffers[src]
	if !ok {
		r = bufio.NewReader(src)
		sys.terminalBuffers[src] = r
	}

	return r
}

// awaitTerminal is the first step of a terminal read wanting up to
// maxLen characters, a line ending at a character for which ends is
// true: it reports whether the read must wait (ErrWait, the process
// waiting in LEF), or may read now (nil). prompt is the read's prompt,
// written once, when the read's turn comes; a read that doesn't wait has
// written it when this returns. A read that went ahead calls
// terminalDone when it has its line.
func (env *Environment) awaitTerminal(maxLen int, prompt string, ends func(byte) bool) error {
	src, ok := env.consoleIn.(TerminalSource)
	if env.engine == nil || !ok {
		env.writeConsole(prompt)

		return nil
	}

	sys := env.System

	i := slices.IndexFunc(sys.terminalQueue, func(t *terminalRead) bool { return t.env == env })
	if i < 0 {
		sys.terminalQueue = append(sys.terminalQueue, &terminalRead{env: env, prompt: prompt})
		sys.promptTerminal()
	}

	r := env.consoleReader()
	ready := func() bool {
		return sys.terminalQueue[0].env == env && lineReady(r, src, maxLen, ends)
	}

	if ready() {
		return nil
	}

	return env.waitOn(sched.StateLEF, sched.ResourceNone, sched.ClassTerminalInput, func() bool {
		return len(sys.terminalQueue) > 0 && ready()
	})
}

// promptTerminal writes the prompt of the read at the head of the
// terminal's queue, if it hasn't been written.
func (sys *System) promptTerminal() {
	if len(sys.terminalQueue) == 0 {
		return
	}

	t := sys.terminalQueue[0]
	if !t.prompted {
		t.prompted = true
		t.env.writeConsole(t.prompt)
	}
}

// terminalDone takes env's read out of the terminal's queue: it has its
// line, or the process is going. The next read's turn comes.
func (env *Environment) terminalDone() {
	sys := env.System

	i := slices.IndexFunc(sys.terminalQueue, func(t *terminalRead) bool { return t.env == env })
	if i < 0 {
		return
	}

	sys.terminalQueue = slices.Delete(sys.terminalQueue, i, i+1)

	if i == 0 {
		sys.promptTerminal()
	}
}

// inTerminalQueue reports whether env has a read in the terminal's
// queue: a read made again after waiting.
func (env *Environment) inTerminalQueue() bool {
	return slices.ContainsFunc(env.terminalQueue, func(t *terminalRead) bool { return t.env == env })
}

// terminalPoll is how often waitForTerminalInput looks at the terminal.
const terminalPoll = time.Millisecond

// waitForTerminalInput waits, up to limit, for the terminal to have
// input for the read at the head of its queue: the idle loop's wait
// when nothing but a key can end a wait. It reports false at once if no
// read waits, or its input stream can't say.
func (sys *System) waitForTerminalInput(limit time.Duration) bool {
	if len(sys.terminalQueue) == 0 {
		return false
	}

	src, ok := sys.terminalQueue[0].env.consoleIn.(TerminalSource)
	if !ok {
		return false
	}

	for deadline := time.Now().Add(limit); !src.Ready() && time.Now().Before(deadline); {
		time.Sleep(terminalPoll)
	}

	return true
}

// lineReady reports whether r holds a whole line for a read of up to
// maxLen characters ending at a character for which ends is true, or
// the end of the input: whether the read can go ahead without waiting.
// It brings into r whatever src has without waiting for.
func lineReady(r *bufio.Reader, src TerminalSource, maxLen int, ends func(byte) bool) bool {
	for {
		n := r.Buffered()
		if n >= maxLen || n >= r.Size() {
			return true
		}

		if n > 0 {
			b, _ := r.Peek(n)
			if slices.ContainsFunc(b, ends) {
				return true
			}
		}

		if !src.Ready() {
			return false
		}

		// One more character (src says it won't wait), or the end of the
		// input, which the read gets at once.
		if _, err := r.Peek(n + 1); err != nil {
			return true
		}
	}
}

// lineEnd is the end of an ordinary line, as RMS's $GET and
// LIB$GET_INPUT read one: RETURN, a host line feed, or CTRL/Z.
func lineEnd(b byte) bool {
	return b == ttCarriageReturn || b == '\n' || b == ctrlZ
}
