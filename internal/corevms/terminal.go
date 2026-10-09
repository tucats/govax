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
// A $QIO read that must wait doesn't make the $QIO wait (docs/PHASE-49.md,
// subtask 10): as on VMS, the $QIO returns SS$_NORMAL with the read
// queued, its request in the terminal's queue among the other reads, and
// the program goes on. When the read's turn comes and its line is there,
// the scheduler's look at the terminal before each choice
// (serviceTerminal) reads it into the program's buffer and completes the
// request: IOSB, event flag, AST. A $QIOW waits for that completion as
// it waits for any pending request. The other readers (RMS's $GET of the
// terminal, LIB$GET_INPUT, the input shims) are synchronous calls, so
// they still wait in their service until their line has come.

// TerminalSource is an input stream that can say whether a read would
// get something at once: a byte, or the end of the input.
type TerminalSource interface {
	Ready() bool
}

// terminalRead is one read in the terminal's queue: a process's
// synchronous read (req nil), or a $QIO read left pending.
type terminalRead struct {
	env      *Environment
	prompt   string
	prompted bool

	// For a pending $QIO read: its request; the most characters it
	// takes and which end its line (for lineReady); and read, which
	// reads the line and returns the request's completion status.
	req    *ioRequest
	maxLen int
	ends   func(byte) bool
	read   func(r *bufio.Reader) ioStatus
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

	i := slices.IndexFunc(sys.terminalQueue, env.isSynchronousRead)
	if i < 0 {
		sys.terminalQueue = append(sys.terminalQueue, &terminalRead{env: env, prompt: prompt})
		sys.promptTerminal()
		i = len(sys.terminalQueue) - 1
	}

	entry := sys.terminalQueue[i]
	r := env.consoleReader()
	ready := func() bool {
		return sys.terminalQueue[0] == entry && lineReady(r, src, maxLen, ends)
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

// isSynchronousRead reports whether t is env's synchronous read (not a
// pending $QIO read).
func (env *Environment) isSynchronousRead(t *terminalRead) bool {
	return t.env == env && t.req == nil
}

// terminalDone takes env's synchronous read out of the terminal's queue:
// it has its line, or the process is going. The next read's turn comes.
func (env *Environment) terminalDone() {
	sys := env.System

	i := slices.IndexFunc(sys.terminalQueue, env.isSynchronousRead)
	if i < 0 {
		return
	}

	sys.dropTerminalRead(i)
}

// dropTerminalRead takes the i'th read out of the terminal's queue. If
// it was the head, the next read's turn comes: its prompt is written,
// and a pending $QIO read whose line is already there completes.
func (sys *System) dropTerminalRead(i int) {
	sys.terminalQueue = slices.Delete(sys.terminalQueue, i, i+1)

	if i == 0 {
		sys.promptTerminal()
		sys.serviceTerminal()
	}
}

// queueTerminalRead is a $QIO read's step before reading (ttdriver.go):
// it reports whether the read must be left pending, in the terminal's
// queue, because another read is ahead of it or its line (up to maxLen
// characters, ending at a character for which ends is true) isn't there
// yet. read is how the read finishes, now or later. A read that may go
// ahead has had its prompt written when this returns. Without the
// scheduler or a TerminalSource no read is ever pending.
func (env *Environment) queueTerminalRead(req *ioRequest, maxLen int, prompt string, ends func(byte) bool,
	read func(*bufio.Reader) ioStatus) bool {
	src, ok := env.consoleIn.(TerminalSource)
	if env.engine == nil || !ok {
		env.writeConsole(prompt)

		return false
	}

	sys := env.System
	t := &terminalRead{env: env, prompt: prompt, req: req, maxLen: maxLen, ends: ends, read: read}

	sys.terminalQueue = append(sys.terminalQueue, t)
	sys.promptTerminal()

	if sys.terminalQueue[0] == t && lineReady(env.consoleReader(), src, maxLen, ends) {
		sys.terminalQueue = sys.terminalQueue[1:]

		return false
	}

	return true
}

// serviceTerminal completes the pending $QIO reads at the head of the
// terminal's queue whose lines are there, in order, stopping at the
// first that must still wait or at a synchronous read (whose process
// reads its own line when its service is made again). A request
// completed meanwhile (cancelled) just leaves the queue. The scheduler
// calls it before each choice (pollEvents), so a read completes, and its
// process's wait for it ends, as soon as the line has been typed.
func (sys *System) serviceTerminal() {
	for len(sys.terminalQueue) > 0 {
		t := sys.terminalQueue[0]
		if t.req == nil {
			return
		}

		if !t.req.done {
			src, ok := t.env.consoleIn.(TerminalSource)
			if ok && !lineReady(t.env.consoleReader(), src, t.maxLen, t.ends) {
				return
			}
		}

		sys.terminalQueue = sys.terminalQueue[1:]

		if !t.req.done {
			t.env.completeIO(t.req, t.read(t.env.consoleReader()))
		}

		sys.promptTerminal()
	}
}

// pruneTerminalQueue takes the $QIO reads that have completed without
// being read (cancelled: $CANCEL, $DASSGN, rundown) out of the
// terminal's queue.
func (sys *System) pruneTerminalQueue() {
	for i := len(sys.terminalQueue) - 1; i >= 0; i-- {
		if t := sys.terminalQueue[i]; t.req != nil && t.req.done {
			sys.dropTerminalRead(i)
		}
	}
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
