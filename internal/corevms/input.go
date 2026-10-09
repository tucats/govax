package corevms

import (
	"bufio"
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
