package rtl

import (
	"bufio"
	"strings"
)

// Port of librtl_input.c's decc_gets/exe_input, plus the shared console-line
// reader they (and file.go's EXE$READ fid-0 case) all build on.

// consoleReader lazily wraps Environment.consoleIn in a *bufio.Reader,
// cached so successive reads don't lose already-buffered-ahead bytes —
// there is no equivalent concern in the C source, which reads directly
// from the process's real stdin each time.
func (env *Environment) consoleReader() *bufio.Reader {
	if env.consoleInBuf == nil {
		src := env.consoleIn
		if src == nil {
			src = strings.NewReader("")
		}
		env.consoleInBuf = bufio.NewReader(src)
	}
	return env.consoleInBuf
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

// shimDeccGets is DECC$GETS: reads one line (including its trailing
// newline, if any — unlike exe_input below, decc_gets doesn't strip it)
// into a caller buffer, returning that same buffer's address.
func shimDeccGets(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) != 1 {
		return 0, nil
	}
	line, err := readConsoleLine(env, 511)
	if err != nil {
		line = ""
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

	line, err := readConsoleLine(env, int(buffLen))
	if err != nil {
		line = ""
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
