package corevms

import "os"

// exe_open's mode argument is the flags bitmask exe_open passes straight
// through to the host's own POSIX open(2), whose O_WRONLY/O_CREAT/O_TRUNC/
// O_APPEND bit values are platform-defined (the C source's own portability
// wrinkle, not one this port introduces) — mapped below using the common
// Linux/glibc bit values (arch.h's own eVAX build target, see
// reference/CLAUDE.md's build notes) since that's the one concrete platform
// this project's own reference build documents.
const (
	posixOWronly = 0x0001
	posixORdwr   = 0x0002
	posixOCreat  = 0x0040
	posixOTrunc  = 0x0200
	posixOAppend = 0x0400
)

func posixFlagsToGo(mode uint32) int {
	flags := os.O_RDONLY

	switch {
	case mode&posixORdwr != 0:
		flags = os.O_RDWR

	case mode&posixOWronly != 0:
		flags = os.O_WRONLY
	}

	if mode&posixOCreat != 0 {
		flags |= os.O_CREATE
	}

	if mode&posixOTrunc != 0 {
		flags |= os.O_TRUNC
	}

	if mode&posixOAppend != 0 {
		flags |= os.O_APPEND
	}

	return flags
}

// shimExeOpen is EXE$OPEN.
func shimExeOpen(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return 0, nil
	}

	name, err := loadString(env, argv[0], 255)
	if err != nil {
		return 0, err
	}

	flags := posixFlagsToGo(argv[1])

	// The open shares the file with other processes' opens by RMS's rule
	// (rms/hostshare.go), as the access its flags ask for: a read-only
	// open is FAB$M_GET (sharing reading, the default), any other a
	// writer's (sharing nothing, the default). A refused open fails
	// before the file is touched.
	fac := fabFACGet
	if flags&(os.O_WRONLY|os.O_RDWR) != 0 {
		fac = fabFACPut
	}

	claim, err := env.HostOpeners.Claim(name, fac, 0)
	if err != nil {
		return 0xFFFFFFFF, nil // -1, matching a failed open()
	}

	f, err := os.OpenFile(name, flags, 0o644)
	if err != nil {
		claim.Release()

		return 0xFFFFFFFF, nil
	}

	claim.Attach(f)

	fid := env.nextFID
	env.nextFID++
	env.openFiles[fid] = f
	env.hostClaims[fid] = claim

	return fid, nil
}

// shimExeClose is EXE$CLOSE.
func shimExeClose(env *Environment, argv []uint32) (uint32, error) {
	fid := argv[0]
	env.closeHostFile(fid)

	return fid, nil
}

// closeHostFile closes the host file open as descriptor fid, if it is,
// and gives up its place among the file's openers.
func (env *Environment) closeHostFile(fid uint32) {
	if f, ok := env.openFiles[fid]; ok {
		_ = f.Close()
		delete(env.openFiles, fid)
	}

	env.hostClaims[fid].Release()
	delete(env.hostClaims, fid)
}

// shimExeRead is EXE$READ. fid 0 reads a line from the console input
// stream, matching exe_read's own fgets(io_buff, len, stdin) special case.
func shimExeRead(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return 0, nil
	}

	fid, addr, length := argv[0], argv[1], argv[2]

	var data []byte

	if fid == 0 {
		line, err := readConsoleLine(env, int(length))
		if err != nil {
			return 0, nil
		}

		data = []byte(line)

		if n := len(data); n > 0 && data[n-1] == '\r' {
			data = data[:n-1]
		}
	} else {
		f, ok := env.openFiles[fid]
		if !ok {
			return 0, nil
		}

		buf := make([]byte, length)
		n, _ := f.Read(buf)
		data = buf[:n]
	}

	for i, b := range data {
		if err := env.mem.StoreByte(env.cpu, addr+uint32(i), b); err != nil {
			return 0, nil
		}
	}

	return uint32(len(data)), nil
}

// shimExeWrite is EXE$WRITE. fid 1 is special-cased straight to the console,
// matching exe_write's own "cheat for formatting purposes" comment.
func shimExeWrite(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return 0, nil
	}

	fid, addr, length := argv[0], argv[1], argv[2]

	buf := make([]byte, length)
	for i := range buf {
		b, err := env.mem.LoadByte(env.cpu, addr+uint32(i))
		if err != nil {
			return 0, nil
		}

		buf[i] = b
	}

	if fid == 1 {
		if env.consoleOut != nil {
			if _, err := env.consoleOut.Write(buf); err != nil {
				return 0, nil
			}
		}

		return length, nil
	}

	f, ok := env.openFiles[fid]
	if !ok {
		return 0, nil
	}

	n, err := f.Write(buf)
	if err != nil {
		return 0, nil
	}

	return uint32(n), nil
}

func registerFileShims(t *ShimTable) {
	t.Register(4, "EXE$OPEN", shimExeOpen)
	t.Register(5, "EXE$CLOSE", shimExeClose)
	t.Register(6, "EXE$READ", shimExeRead)
	t.Register(7, "EXE$WRITE", shimExeWrite)
}
