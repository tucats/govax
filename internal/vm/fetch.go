package vm

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/vax"
)

// This file is the instruction-fetch window (Study 1, R4 in
// docs/PERFORMANCE.md): a fast path for reading the *instruction stream*,
// the bytes of the program itself that the CPU decodes one instruction at
// a time.
//
// # Why the instruction stream gets its own path
//
// A VAX instruction is a variable-length run of bytes: an opcode (one
// byte, or two for an "extended" opcode), then one *operand specifier* per
// operand (a byte saying which addressing mode the operand uses), and,
// for some modes, a displacement or immediate value after it (1, 2, 4, or
// more bytes). Decoding one instruction therefore reads about four
// separate small pieces of memory, all in a row, almost always in the same
// 512-byte page as the previous instruction.
//
// Before this window existed, each of those pieces was an ordinary memory
// read (LoadByte and friends in storage.go): a call into Translate, which
// checks whether virtual memory is on, consults the one-slot sequential
// translation cache (STC, see tb.go), maybe the 128-entry TB, then checks
// the physical address against RAM, ROM, and NVRAM. That is a lot of work
// to repeat four times per instruction for the same page. Worse, the STC
// is shared with *data* reads and writes, so an instruction whose operand
// lives on another page evicts the code page from it, and the next
// instruction byte has to go to the TB to get it back.
//
// The window remembers one page of the instruction stream, translated
// once: the virtual address of the page's first byte, and where that page
// starts in physical RAM. A fetch that falls inside it is then a single
// subtraction, a comparison, and a direct read of the RAM slice: no call
// chain, no translation. Data accesses never touch it, so they can't evict
// it. When the PC moves to another page (sequential flow across a page
// boundary, or a jump), the first fetch there misses, takes the old slow
// path through Translate, and refills the window for the new page.
//
// Real VAX processors do something similar: they prefetch the instruction
// stream into a small buffer, separate from the data path, rather than
// translating each instruction byte on its own.
//
// # Keeping it correct
//
// A cached translation is only valid while nothing that went into it has
// changed. The window is emptied (so the next fetch takes the slow path
// and translates afresh) whenever:
//
//   - the STC is emptied (stcFlush in tb.go). That covers every TB
//     invalidation (TBIA, TBIS, a direct PTE write by the console), every
//     change of access mode (InvalidateProtection, called at REI, CHMx,
//     exception and interrupt delivery, and AST delivery), and every
//     translation fault. The window is therefore never trusted longer
//     than the STC itself would be.
//   - the CPU's MAPEN register (memory mapping on/off) or its current
//     access mode differ from what they were when the window was filled.
//     MAPEN is set directly in several places (exception delivery
//     switches it off for a moment to read the system control block, the
//     console's VMINIT and SET MAPEN), none of which goes through this
//     package, so SyncFetchWindow compares both before each instruction
//     rather than relying on a call. The mode check repeats what
//     InvalidateProtection already guarantees, as a cheap safety net.
//
// The window holds a physical *address*, not a copy of the bytes, so a
// program that writes into its own code (or the console depositing into
// it) is seen at once: the next fetch reads RAM as it is now.
//
// Only a page wholly inside main RAM is ever put in the window. Code
// running from ROM or NVRAM (which live in separate buffers, see
// memory.go's phys) always takes the slow path.
//
// # What a fetch may and may not differ in
//
// A fetch through the window returns exactly what LoadByte (or LoadWord,
// and so on) would have, and faults exactly where they would have: a
// fetch in the window's page is one the slow path already translated
// successfully, under the same MAPEN, the same mode, and with no TB
// invalidation since. Anything else, including a multi-byte value that
// runs off the end of the window's page, takes the slow path, which is
// the old code unchanged. Only the statistics differ (fewer STC tries and
// single-byte reads; see FetchStats).

// fetchWindow is the instruction-fetch window's state. The zero value is
// an empty window: limit is 0, so no offset is ever less than it and no
// fetch can hit.
type fetchWindow struct {
	// vbase is the virtual address of the first byte of the cached page.
	vbase uint32

	// pbase is the physical address of that same byte: its index in
	// Memory.ram.
	pbase uint32

	// limit is the number of bytes of the page the window covers: pageSize
	// (512) when it holds a page, 0 when it is empty. Keeping the
	// emptiness in limit rather than in a separate bool lets the hit test
	// be one comparison (see FetchByte).
	limit uint32

	// mapen and mode are the CPU's MAPEN register and current access mode
	// when the window was filled; SyncFetchWindow empties the window if
	// either has changed since.
	mapen uint32
	mode  vax.AccessMode

	// hits counts fetches the window served, and fills counts the times
	// the slow path refilled it; reported by FetchStats.
	hits, fills int64
}

// flush empties the window, so the next fetch takes the slow path.
func (w *fetchWindow) flush() { w.limit = 0 }

// SyncFetchWindow empties the instruction-fetch window if the CPU's MAPEN
// register or current access mode has changed since the window was
// filled (see this file's comment for why both matter). The CPU engine
// calls it once before fetching each instruction, which is cheaper than
// checking both on every byte fetched. It is small enough for the Go
// compiler to inline into its caller.
func (m *Memory) SyncFetchWindow(cpu *vax.CPU) {
	if cpu.PR(vax.MAPEN) != m.tb.fetch.mapen || cpu.PSL().CurMod() != m.tb.fetch.mode {
		m.tb.fetch.flush()
	}
}

// FetchByte reads one byte of the instruction stream at virtual address
// addr: an opcode, an operand specifier, or a byte displacement or
// immediate. It returns what LoadByte would, but reads straight from RAM
// when addr is in the instruction-fetch window's page.
//
// `off := addr - m.tb.fetch.vbase` is computed in unsigned 32-bit arithmetic:
// for an addr below vbase the subtraction wraps around to a huge number,
// which fails `off < m.tb.fetch.limit` just as an addr past the end of the
// page does. So one comparison checks both ends of the page.
//
// The fast path is kept this short so that the Go compiler inlines it
// into the decoder: the common case then costs no function call at all.
// Everything else is in fetchByteSlow.
func (m *Memory) FetchByte(cpu *vax.CPU, addr uint32) (byte, error) {
	if off := addr - m.tb.fetch.vbase; off < m.tb.fetch.limit {
		m.tb.fetch.hits++

		return m.ram[m.tb.fetch.pbase+off], nil
	}

	return m.fetchByteSlow(cpu, addr)
}

// TryFetchByte is FetchByte's fast path alone: the byte at addr and ok
// true if addr is in the instruction-fetch window, or ok false (and no
// read at all) if it isn't, in which case the caller must use FetchByte.
//
// It exists because FetchByte can't be inlined: the Go compiler inlines
// only small functions, and FetchByte's call to its slow path alone takes
// most of that allowance. TryFetchByte has no call in it, so it is
// inlined, and the decoder's two busiest fetches (the opcode and each
// operand specifier) use it to read a byte with no function call at all,
// falling back to FetchByte on a miss.
func (m *Memory) TryFetchByte(addr uint32) (b byte, ok bool) {
	if off := addr - m.tb.fetch.vbase; off < m.tb.fetch.limit {
		m.tb.fetch.hits++

		return m.ram[m.tb.fetch.pbase+off], true
	}

	return 0, false
}

// fetchByteSlow is FetchByte's slow path: an ordinary LoadByte, which
// translates addr (faulting exactly as any read would), followed by
// pointing the window at addr's page so that the following fetches hit.
func (m *Memory) fetchByteSlow(cpu *vax.CPU, addr uint32) (byte, error) {
	b, err := m.LoadByte(cpu, addr)
	if err != nil {
		return 0, err
	}

	m.fillFetchWindow(cpu, addr)

	return b, nil
}

// FetchWord reads a 2-byte (word) value from the instruction stream at
// addr, as LoadWord would: a word displacement, branch offset, or
// immediate. Like FetchByte, it reads RAM directly when the whole word is
// inside the window's page.
//
// The second comparison, `off+2 <= limit`, keeps a word that starts in
// the page's last byte (and so ends in the next page, which may map
// somewhere else entirely, or not at all) off the fast path. It can't
// overflow: the first comparison has already established off < limit,
// and limit is at most 512.
//
// binary.LittleEndian.Uint16 assembles the two bytes low byte first, the
// VAX's byte order (see memory.go's readPhysLongword).
func (m *Memory) FetchWord(cpu *vax.CPU, addr uint32) (uint16, error) {
	if off := addr - m.tb.fetch.vbase; off < m.tb.fetch.limit && off+2 <= m.tb.fetch.limit {
		m.tb.fetch.hits++

		return binary.LittleEndian.Uint16(m.ram[m.tb.fetch.pbase+off:]), nil
	}

	return m.fetchWordSlow(cpu, addr)
}

// fetchWordSlow is FetchWord's slow path: LoadWord, which handles a word
// that crosses a page boundary, then a window refill for addr's page.
func (m *Memory) fetchWordSlow(cpu *vax.CPU, addr uint32) (uint16, error) {
	w, err := m.LoadWord(cpu, addr)
	if err != nil {
		return 0, err
	}

	m.fillFetchWindow(cpu, addr)

	return w, nil
}

// FetchLongword reads a 4-byte (longword) value from the instruction
// stream at addr, as LoadLongword would: a longword displacement, an
// absolute address, or an immediate. See FetchWord for the page-boundary
// test.
func (m *Memory) FetchLongword(cpu *vax.CPU, addr uint32) (uint32, error) {
	if off := addr - m.tb.fetch.vbase; off < m.tb.fetch.limit && off+4 <= m.tb.fetch.limit {
		m.tb.fetch.hits++

		return binary.LittleEndian.Uint32(m.ram[m.tb.fetch.pbase+off:]), nil
	}

	return m.fetchLongwordSlow(cpu, addr)
}

// fetchLongwordSlow is FetchLongword's slow path: LoadLongword, then a
// window refill for addr's page.
func (m *Memory) fetchLongwordSlow(cpu *vax.CPU, addr uint32) (uint32, error) {
	v, err := m.LoadLongword(cpu, addr)
	if err != nil {
		return 0, err
	}

	m.fillFetchWindow(cpu, addr)

	return v, nil
}

// fillFetchWindow points the window at the page holding addr, which the
// caller has just read successfully through the slow path. It translates
// addr again to learn the page's physical address; that translation hits
// the STC the read just filled, so it costs little and can't fault (and if
// it somehow did, the window is simply left empty).
//
// A page that isn't wholly in main RAM (one in ROM or NVRAM) isn't put in
// the window: the fast path reads only Memory.ram.
func (m *Memory) fillFetchWindow(cpu *vax.CPU, addr uint32) {
	m.tb.fetch.flush()

	paddr, err := m.Translate(cpu, addr, AccessRead)
	if err != nil {
		return
	}

	// &^ is Go's "and not" (bit clear) operator: x &^ (pageSize-1) clears
	// the low 9 bits, giving the address of the first byte of x's page.
	pbase := paddr &^ uint32(pageSize-1)
	if uint64(pbase)+pageSize > uint64(len(m.ram)) {
		return
	}

	m.tb.fetch.vbase = addr &^ uint32(pageSize-1)
	m.tb.fetch.pbase = pbase
	m.tb.fetch.mapen = cpu.PR(vax.MAPEN)
	m.tb.fetch.mode = cpu.PSL().CurMod()
	m.tb.fetch.limit = pageSize
	m.tb.fetch.fills++
}

// FetchStats reports the instruction-fetch window's counters: hits is the
// number of instruction-stream reads it served straight from RAM, and
// fills the number of times the slow path refilled it (a new page, or
// the first fetch after something emptied it). Fetches the window
// doesn't serve are counted as ordinary reads (see Stats and STCStats).
func (m *Memory) FetchStats() (hits, fills int64) {
	return m.tb.fetch.hits, m.tb.fetch.fills
}
