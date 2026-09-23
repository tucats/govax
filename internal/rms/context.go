package rms

import (
	"io"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// This file defines Context, the one value every service handler in this
// package (create.go, and — as docs/PHASE-22.md's later subtasks land —
// connect.go/open.go/close.go/get.go/put.go) is called with, plus a
// handful of small helper methods on it that every handler would
// otherwise have to repeat.
//
// # Why not just use internal/rtl.Environment directly?
//
// internal/rtl.Environment already bundles almost exactly this same set
// of things (its own *vm.Memory, *vax.CPU, console-output io.Writer,
// Logicals) — using it directly here would avoid a second, similar-looking
// type. But Environment's mem/cpu/consoleOut fields are unexported,
// reachable only from code inside package rtl itself, and this package's
// handlers need to read and write VAX memory directly (FAB/RAB fields —
// see fab.go/rab.go) the same way internal/rtl's own shims and services
// do. Since package rtl is what will register this package's handlers
// into its ServiceTable (docs/PHASE-22.md's subtask 11 — "registerRMSServices
// shrinks to registering internal/rms's handlers"), the dependency has to
// run rtl -> rms, not rms -> rtl: if this package imported internal/rtl
// too (for example, just to spell out *rtl.Environment as a parameter
// type), the two packages would import each other, which Go simply
// refuses to build at all. Context is this package's own, self-contained
// answer to the same need, built from types (internal/vm, internal/vax,
// internal/io) that don't import internal/rtl either.
type Context struct {
	// Mem/CPU are how a handler reads and writes VAX memory directly —
	// FAB/RAB fields, file-specification strings, and record buffers all
	// live in emulated VAX memory, addressed exactly the same way every
	// other part of this emulator addresses it (translation, protection
	// checking, and so on all still apply — see internal/vm/storage.go).
	Mem *vm.Memory
	CPU *vax.CPU

	// Mounts is which VAX device names currently have a real ODS-2 volume
	// mounted (mount.go) — what a SYS$CREATE/SYS$OPEN targeting a disk
	// device resolves the parsed device name against.
	Mounts *MountTable

	// Files is this "process"'s own table of currently open files
	// (ifi.go) — SYS$CREATE/SYS$OPEN allocate a slot in it; SYS$PUT/
	// SYS$GET/SYS$CLOSE look an existing slot back up by IFI.
	Files *FileTable

	// Logicals is consulted so that a file specification which is itself
	// a defined logical name (for instance "SYS$OUTPUT", which
	// internal/io/logical.go's InitLogicals points at "TTA0:" by default)
	// is translated to what it actually names before being parsed as a
	// device/file spec — matching real RMS's own logical-name
	// translation, and this package's now-deleted Phase 10 predecessor
	// (internal/rtl/rms.go, see git history).
	Logicals *iodev.LogicalNameTable

	// Console is where a file resolved to the terminal pseudo-device
	// (the TTA0: special case — see fab.go's package doc comment and
	// ifi.go's FileHandle) actually writes its output to.
	Console io.Writer
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

func (ctx *Context) storeLongword(addr uint32, v uint32) error {
	return ctx.Mem.StoreLongword(ctx.CPU, addr, v)
}

// loadFixedString reads exactly n bytes starting at addr into a Go
// string. This is deliberately NOT a NUL-terminated scan the way
// internal/rtl's own loadString helper is for ordinary C strings: per
// fab.go's own fabFNS doc comment, RMS file-specification strings are
// never NUL-terminated — the caller always says how long the string is
// via a separate length field (FAB$B_FNS), so reading anything other
// than exactly that many bytes verbatim would either truncate a spec
// that legitimately contains no NUL at all, or silently stop early on
// one that happens to contain a zero byte that isn't a terminator (VMS
// file specs are plain 8-bit text, not C strings — a zero byte there has
// no special meaning).
func (ctx *Context) loadFixedString(addr uint32, n int) (string, error) {
	buf := make([]byte, n)

	for i := range buf {
		b, err := ctx.loadByte(addr + uint32(i))
		if err != nil {
			return "", err
		}

		buf[i] = b
	}

	return string(buf), nil
}
