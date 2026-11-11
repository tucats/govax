package rms

import (
	"bufio"
	"io"

	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// This file defines Context, the one value every service handler in this
// package is called with, and small helpers on it that every handler
// would otherwise repeat.
//
// # Why not internal/corevms.Environment?
//
// An Environment (one process) has all of this and more, but corevms
// imports this package to register its handlers (corevms/rms.go), so
// this package can't import corevms: Go refuses an import cycle. Context
// is what a handler needs of the process, built from lower packages'
// types (vm, registers, lnm, lck) and function fields, which corevms's
// Environment.rmsContext fills in for each call.
type Context struct {
	// lockedByOpener says an open's RMS$_FLK was another RMS opener's sharing, which
	// has no STV (stvFor).
	lockedByOpener bool

	// Mem/CPU are how a handler reads and writes VAX memory directly —
	// FAB/RAB fields, file-specification strings, and record buffers all
	// live in emulated VAX memory, addressed exactly the same way every
	// other part of this emulator addresses it (translation, protection
	// checking, and so on all still apply — see internal/vm/storage.go).
	Mem *vm.Memory
	CPU *registers.CPU

	// Mounts is which VAX device names currently have a real ODS-2 volume
	// mounted (mount.go) — what a SYS$CREATE/SYS$OPEN targeting a disk
	// device resolves the parsed device name against.
	Mounts *MountTable

	// Files is this "process"'s own table of currently open files
	// (ifi.go) — SYS$CREATE/SYS$OPEN allocate a slot in it; SYS$PUT/
	// SYS$GET/SYS$CLOSE look an existing slot back up by IFI.
	Files *FileTable

	// Logicals is the process's logical-name database, shared with the
	// console and internal/corevms. It is consulted so that a file
	// specification which is itself a defined logical name (for instance
	// "SYS$OUTPUT", which the console points at "_TTA0:") is translated
	// to what it actually names before being parsed as a device/file
	// spec, and so that a spec with no device gets SYS$DISK's (see
	// logicals.go).
	Logicals *lnm.Database

	// Session, when set, is the console's Session, whose default
	// directory a spec with no directory of its own is resolved in (the
	// process default directory SET DEFAULT establishes). nil means the
	// master file directory.
	Session *Session

	// Console is where a file resolved to the terminal pseudo-device
	// (the TTA0: special case — see fab.go's package doc comment and
	// ifi.go's FileHandle) actually writes its output to.
	Console io.Writer

	// ConsoleIn is where a terminal $GET reads its record from (see
	// terminal.go): the console's own input stream, shared with the
	// terminal driver's reads so neither loses what the other buffered.
	// nil reads as end of file.
	ConsoleIn *bufio.Reader

	// NodeName is the system's node name, as $GETSYI reports it: part of
	// NAM$T_DVI's device name.
	NodeName string

	// AwaitTerminal, if set, is asked before a terminal $GET reads a
	// record of up to maxLen characters, with the read's prompt: it
	// writes the prompt (once) and returns nil when the record can be
	// read, or an error (the process waiting: the service is called
	// again). TerminalDone is called when the record has been read.
	// Unset, the prompt is written and the read waits in the host.
	AwaitTerminal func(maxLen int, prompt string) error
	TerminalDone  func()

	// CountIO, if set, counts buffered and direct I/O operations for the
	// process, by the model in iocount.go (corevms's iocount.go keeps
	// the counts).
	CountIO func(buffered, direct uint32)

	// Devices opens the record devices (mailboxes, NL:) a file
	// specification may name (recdevice.go); nil means none can be.
	Devices DeviceOpener

	// Locks is the system's lock database (internal/lck), where RMS takes
	// its record locks (recordlock.go), and PID the process they're
	// taken for. nil Locks: no record locking.
	Locks *lck.Manager
	PID   uint32

	// AwaitLock, if set, makes the process wait until over reports true
	// (a record lock granted, RAB$V_WAT), or, if deadline isn't 0, until
	// the system time (Clock) reaches it (RAB$V_TMO), returning the error
	// the service returns to be called again then.
	AwaitLock func(over func() bool, deadline uint64) error

	// CanLock, if set, reports whether the process may take another lock
	// (its job's ENQLM quota has one left); when it can't, a record lock
	// is RMS$_EXENQLM. nil: no limit.
	CanLock func() bool

	// AssignFileChannel, if set, assigns the process a channel to device
	// with the file f accessed on it (nil: a device that holds no files),
	// for a user file open (FAB$V_UFO, ufo.go), returning the channel's
	// number and a system service status. nil: UFO is RMS$_SUPPORT.
	AssignFileChannel func(device string, f *ACPFile) (uint16, uint32)

	// Clock, if set, is the system time, in VMS's 100-nanosecond units:
	// what a record lock wait's time limit (RAB$V_TMO) is measured by.
	// nil: no time limits.
	Clock func() uint64

	// Waker, if set, is the Data of a record lock the stream waits for:
	// told the lock is granted, it makes the process computable at once.
	Waker lck.Notifier
}

// loadByte/loadWord/loadLongword/storeByte/storeWord/storeLongword are
// thin ctx.Mem+ctx.CPU forwarders, purely so a handler can write, e.g.,
// ctx.loadWord(addr) instead of repeating ctx.Mem.LoadWord(ctx.CPU, addr)
// at every single call site — every one of these calls can fail (for
// instance, addr pointing at an unmapped or protected page), which is
// why each still returns its own error for the caller to check, exactly
// as the underlying *vm.Memory methods do.
func (ctx *Context) loadByte(addr uint32) (byte, error) {
	return ctx.Mem.LoadByte(ctx.CPU, addr)
}

func (ctx *Context) loadWord(addr uint32) (uint16, error) {
	return ctx.Mem.LoadWord(ctx.CPU, addr)
}

func (ctx *Context) loadLongword(addr uint32) (uint32, error) {
	return ctx.Mem.LoadLongword(ctx.CPU, addr)
}

func (ctx *Context) storeByte(addr uint32, v byte) error {
	return ctx.Mem.StoreByte(ctx.CPU, addr, v)
}

func (ctx *Context) storeWord(addr uint32, v uint16) error {
	return ctx.Mem.StoreWord(ctx.CPU, addr, v)
}

// storeBytes writes b at addr, a byte at a time.
func (ctx *Context) storeBytes(addr uint32, b []byte) error {
	for i, v := range b {
		if err := ctx.storeByte(addr+uint32(i), v); err != nil {
			return err
		}
	}

	return nil
}

func (ctx *Context) storeLongword(addr uint32, v uint32) error {
	return ctx.Mem.StoreLongword(ctx.CPU, addr, v)
}

// loadFixedString reads exactly n bytes starting at addr into a Go
// string. This is deliberately NOT a NUL-terminated scan the way a
// helper for C strings would be: per
// fab.go's own fabFNS doc comment, RMS file-specification strings are
// never NUL-terminated — the caller always says how long the string is
// via a separate length field (FAB$B_FNS), so reading anything other
// than exactly that many bytes verbatim would either truncate a spec
// that legitimately contains no NUL at all, or silently stop early on
// one that happens to contain a zero byte that isn't a terminator (VMS
// file specs are plain 8-bit text, not C strings — a zero byte there has
// no special meaning).
func (ctx *Context) loadFixedString(addr uint32, n int) (string, error) {
	buf, err := ctx.loadBytes(addr, n)

	return string(buf), err
}

// loadBytes reads n bytes starting at addr: a control block whose fields
// are read together.
func (ctx *Context) loadBytes(addr uint32, n int) ([]byte, error) {
	buf := make([]byte, n)

	for i := range buf {
		b, err := ctx.loadByte(addr + uint32(i))
		if err != nil {
			return nil, err
		}

		buf[i] = b
	}

	return buf, nil
}
