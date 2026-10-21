package corevms

import (
	"bufio"
	"io"
	"strings"
)

// consoleReader lazily wraps Environment.consoleIn in a *bufio.Reader,
// cached so successive reads don't lose already-buffered-ahead bytes.
// Every process reading the same stream (the terminal) shares one
// buffer (System.terminalReader), so what one process's read took ahead
// is there for the next process's (docs/PHASE-46.md, subtask 7).
func (env *Environment) consoleReader() *bufio.Reader {
	if env.consoleInBuf == nil {
		if env.consoleIn == nil {
			env.consoleInBuf = bufio.NewReader(strings.NewReader(""))
		} else {
			env.consoleInBuf = env.terminalReader(env.consoleIn)
		}
	}

	return env.consoleInBuf
}

// TerminalBuffer is the buffer the processes reading env's input share,
// or nil when none has read it yet: what is in it has been taken from the
// input already, so a reader of the input itself (the console's DCL)
// reads it first.
func (env *Environment) TerminalBuffer() *bufio.Reader {
	if env.consoleIn == nil {
		return nil
	}

	return env.terminalBuffers[env.consoleIn]
}

// ctrlZ is CTRL/Z, the character a terminal user types to end input.
const ctrlZ = 0x1A

// readTerminalLine reads one line from the terminal, as RMS's $GET of a
// terminal does (internal/rms/terminal.go): the line ends at a carriage
// return or line feed (a host "\r\n" pair is one end), neither of which
// is part of it, or when maxLen bytes have been read. CTRL/Z ends a line
// too, but with nothing read before it, it is end of file (ok false), as is
// the end of the host's input with nothing read. (Typed after text, CTRL/Z
// is followed by another for the next read: cmd/govax/attention.go.)
func readTerminalLine(env *Environment, maxLen int) (line string, ok bool) {
	r := env.consoleReader()

	var buf []byte

	for len(buf) < maxLen {
		b, err := readTerminalByte(r)
		if err != nil {
			return string(buf), len(buf) > 0
		}

		switch b {
		case ttCarriageReturn:
			return string(buf), true

		case ctrlZ:
			return string(buf), len(buf) > 0
		}

		buf = append(buf, b)
	}

	return string(buf), true
}

// readConsoleLine reads at most maxLen bytes from the console input stream,
// stopping after a newline (inclusive) if one comes first — matching
// fgets(buf, maxLen+1, stdin)'s own behavior (fgets' size parameter counts
// the NUL terminator it also writes; readConsoleLine has no such
// terminator to budget for, so it takes the content length directly).
func readConsoleLine(env *Environment, maxLen int) (string, error) {
	r := env.consoleReader()
	buf := make([]byte, 0, maxLen)

	for len(buf) < maxLen {
		b, err := r.ReadByte()
		if err != nil {
			if len(buf) == 0 {
				return "", err
			}

			break
		}

		buf = append(buf, b)

		if b == '\n' {
			break
		}
	}

	return string(buf), nil
}

// newlineEnd is the end of a line readConsoleLine reads: a newline.
func newlineEnd(b byte) bool { return b == '\n' }

// readSharedConsoleLine is readConsoleLine on the terminal every process
// shares (terminal.go): with the scheduler on, the read waits its turn,
// and for a whole line, in LEF (ErrWait, the shim run again later), so
// other processes run meanwhile rather than the machine stopping in the
// host's read.
func readSharedConsoleLine(env *Environment, maxLen int) (string, error) {
	if err := env.awaitTerminal(maxLen, "", newlineEnd); err != nil {
		return "", err
	}

	line, err := readConsoleLine(env, maxLen)
	env.terminalDone()
	env.countIO(false)

	if err != nil {
		line = ""
	}

	return line, nil
}

// shimDeccGets is DECC$GETS: reads one line (including its trailing
// newline, if any — unlike exe_input below, decc_gets doesn't strip it)
// into a caller buffer, returning that same buffer's address.
func shimDeccGets(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) != 1 {
		return 0, nil
	}

	line, err := readSharedConsoleLine(env, 511)
	if err != nil {
		return 0, err // waiting for the line: called again
	}

	if err := storeString(env, line+"\x00", argv[0], len(line)+1); err != nil {
		return 0, err
	}

	return argv[0], nil
}

// shimExeInput is EXE$INPUT: reads one line, stripping exactly one trailing
// \r or \n (matching exe_input's own single-character strip, not a full
// \r\n pair), returning the byte count actually copied.
func shimExeInput(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) != 2 {
		return 0, nil
	}

	buffAddr, buffLen := argv[0], argv[1]

	line, err := readSharedConsoleLine(env, int(buffLen))
	if err != nil {
		return 0, err // waiting for the line: called again
	}

	n := len(line)
	if n > 0 && (line[n-1] == '\r' || line[n-1] == '\n') {
		n--
	}

	for i := 0; i < n; i++ {
		if err := env.mem.StoreByte(env.cpu, buffAddr+uint32(i), line[i]); err != nil {
			return 0, nil
		}
	}

	return uint32(n), nil
}

func registerInputShims(t *ShimTable) {
	t.Register(3, "EXE$INPUT", shimExeInput)
	t.Register(14, "DECC$GETS", shimDeccGets)
}

// RedirectInput makes r the process's input stream in place of the
// terminal, until the returned function puts the terminal back: what a
// VMS process's SYS$INPUT is while DCL runs a command procedure (the
// console gives an image the procedure's data lines this way). Every
// reader of the process's input reads r then (a terminal $QIO read,
// RMS's $GET of the terminal, LIB$GET_INPUT, the input shims), through
// a buffer of its own, so nothing taken from r is left in the
// terminal's shared buffer and nothing typed ahead at the terminal is
// read in its place. r should be a TerminalSource whose Ready is true
// when its lines are already at hand, so a read never waits for it.
func (env *Environment) RedirectInput(r io.Reader) (restore func()) {
	saved, outer := &inputStream{env.consoleIn, env.consoleInBuf}, env.terminalIn

	// The first redirection's is the terminal's (ReadCommandLine).
	if outer == nil {
		env.terminalIn = saved
	}

	env.consoleIn, env.consoleInBuf = r, bufio.NewReader(r)

	return func() {
		env.consoleIn, env.consoleInBuf = saved.in, saved.buf
		env.terminalIn = outer
	}
}

// inputStream is a process's input stream: the reader, and its buffer
// (nil until it is first read).
type inputStream struct {
	in  io.Reader
	buf *bufio.Reader
}

// ReadCommandLine is readTerminalInput from the process's terminal,
// SYS$COMMAND, even while RedirectInput has made a command procedure's
// data lines its input (SYS$INPUT): for DCL's reads of the terminal
// during a procedure (INQUIRE, READ SYS$COMMAND, the Ctrl/Y level) in a
// subprocess's CLI.
func (env *Environment) ReadCommandLine(prompt string, maxLen int) (line string, ok bool, err error) {
	if t := env.terminalIn; t != nil {
		in, buf := env.consoleIn, env.consoleInBuf
		env.consoleIn, env.consoleInBuf = t.in, t.buf

		defer func() {
			t.buf = env.consoleInBuf
			env.consoleIn, env.consoleInBuf = in, buf
		}()
	}

	return env.readTerminalInput(prompt, maxLen)
}

// ReadLine reads one line from the process's input stream, as
// readTerminalLine does, without waiting its turn among other processes'
// terminal reads: for the console's own commands (TYPE SYS$INPUT), which
// read while no image runs. ok is false at the end of the input.
func (env *Environment) ReadLine(maxLen int) (line string, ok bool) {
	return readTerminalLine(env, maxLen)
}
