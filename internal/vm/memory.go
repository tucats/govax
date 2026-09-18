package vm

import (
	"encoding/binary"
	"fmt"
)

// Memory is a VAX machine's physical memory: a flat byte array addressed
// 0..Size()-1. "Physical" here is the opposite of "virtual" (see the
// package doc comment): this is the actual, real RAM a byte ends up
// stored in, after any virtual-to-physical translation has already
// happened. Physical addresses are always in range 0..Size()-1 by
// definition — there's no further translation step for them, which is why
// the low-level helpers in this file (phys, readPhysLongword,
// writePhysLongword) never fail with a translation fault, only with
// PhysicalAddressError if the address is simply out of bounds.
//
// It owns the raw storage itself (the Phase 02 open question on vm/vax.CPU
// coupling is resolved this way: internal/vax.CPU is registers/PSL only,
// and Memory is independent of it, taking a *vax.CPU as a parameter
// wherever address translation needs to consult MAPEN/base-and-length
// registers/cur_mod — see translate.go and storage.go). A *vax.CPU
// parameter, rather than a field on Memory, means this package doesn't need
// to import (or even know about) most of internal/vax's contents beyond the
// handful of accessors it actually calls — a common Go pattern for keeping
// packages loosely coupled: depend on the smallest interface/parameter you
// actually need, not the whole object.
//
// Only main RAM is modeled here. The C source's get_address also resolves
// console ROM, NVRAM, and a "microvax" special range, and falls through to
// memory-mapped I/O (load_io/store_io) for anything else — those belong to
// the console (Phase 08) and I/O (Phase 09) subsystems that own that state,
// so a physical address outside RAM is simply reported as unbacked for now.
//
// pageMap/vmValid are the Go equivalent of vm.c's global page_map array and
// VMVALID macro: the free/claimed state of every physical page, and whether
// a VMINIT has run and demand-paging (translate.go's Translate, on an
// invalid PTE) is allowed to claim one — see AllocatePage/ReservePage/
// MappedPages and internal/console/vminit.go, which is the only writer of
// vmValid and the initial S0 reservations.
type Memory struct {
	// ram is a Go slice of bytes — a dynamically-sized, contiguous view
	// over an underlying array, essentially "a resizable/sliceable array"
	// in Go's standard vocabulary (see NewMemory below for how it's
	// created). Every physical byte of the emulated machine's RAM lives
	// here; ram[n] is physical address n.
	ram []byte

	// pageMap records, one bool per 512-byte physical page, whether that
	// page is currently claimed by some virtual mapping (true) or still
	// free (false). It's the same idea as ram but at page granularity
	// instead of byte granularity: len(pageMap) == len(ram)/pageSize.
	pageMap []bool

	vmValid              bool
	translationCount     int64
	singlebyteReadCount  int64
	multibyteReadCount   int64
	singlebyteWriteCount int64
	multibyteWriteCount  int64

	// tb is the translation-buffer/sequential-translation-cache state;
	// see tb.go's own doc comment. It's embedded as a plain (non-pointer)
	// field, so a Memory value always has exactly one tb living inside it
	// with no separate allocation, initialized automatically to its
	// zero value (every cache slot empty, every counter zero) the moment
	// the Memory itself is created.
	tb tb
}

// NewMemory returns a Memory with size bytes of zeroed RAM and every
// physical page free.
//
// make([]byte, size) is Go's way of allocating a slice of a given length
// with every element already set to its zero value — for byte, that's 0 —
// which is exactly "zeroed RAM" of the requested size. size/pageSize (an
// integer division, so any partial trailing page is silently dropped) is
// how many whole 512-byte pages fit in that much RAM, and make([]bool, ...)
// likewise allocates that many bools, each defaulting to false ("not yet
// claimed") — see the pageMap field comment above.
func NewMemory(size uint32) *Memory {
	return &Memory{ram: make([]byte, size), pageMap: make([]bool, size/pageSize)}
}

// SetVMValid records whether a VMINIT has established page tables for this
// Memory, matching vax.console.vminit_valid (VMVALID) — gates whether
// Translate may demand-page an invalid PTE via AllocatePage, matching
// validate_page's own "if( !VMVALID ) return VAX_FAULT" early-out.
func (m *Memory) SetVMValid(v bool) { m.vmValid = v }

// ReservePage marks physical page pfn as claimed without searching for a
// free one — used by VMINIT's eager S0 identity map (every S0 page is
// always pre-mapped, DYNVM or not; see vminit.go), matching vm.c's own
// unconditional `page_map[ page ] = 1` in that same loop.
func (m *Memory) ReservePage(pfn uint32) {
	if int(pfn) < len(m.pageMap) {
		m.pageMap[pfn] = true
	}
}

// AllocatePage claims and returns the lowest-numbered free physical page,
// or reports ok=false if none remain. Physical page 0 is never handed out
// (the search starts at 1), matching validate_page's own loop bound — page
// 0 is always claimed directly by VMINIT's S0 identity map instead.
//
// The two return values (pfn uint32, ok bool) are Go's usual idiom for "a
// result that might not exist": rather than using a special sentinel value
// of pfn to mean failure (easy to forget to check, and there's no obviously
// "impossible" page number to reuse for it), the function returns an
// explicit second bool the caller is expected to check, the same pattern
// used throughout the standard library (e.g. `v, ok := someMap[key]`).
// Named return values (pfn and ok are named in the function signature
// itself) also serve as inline documentation of what each position in the
// returned tuple means, which is why you'll see this pattern repeated
// elsewhere in this package (e.g. Memory.Stats below).
//
// Unlike vm.c's validate_page, which sizes its search to mapsize =
// (memsize>>9)+1 — one slot past the last real physical page, letting it
// hand out a PFN that addresses beyond RAM once every real page is taken —
// this bounds the search to actual physical pages. Replicating that
// off-by-one would hand a caller a PFN whose PFN()<<9 physical address is
// out of range, which internal/vm has no use for (Translate would just
// fail the subsequent memory access instead of getting one). See
// docs/DEVIATIONS.md.
func (m *Memory) AllocatePage() (pfn uint32, ok bool) {
	for n := 1; n < len(m.pageMap); n++ {
		if !m.pageMap[n] {
			m.pageMap[n] = true

			return uint32(n), true
		}
	}

	return 0, false
}

// MappedPages returns the number of physical pages currently claimed
// (reserved or demand-paged in), matching mapped_pages() — which, like
// AllocatePage, never counts physical page 0.
func (m *Memory) MappedPages() int {
	count := 0

	for n := 1; n < len(m.pageMap); n++ {
		if m.pageMap[n] {
			count++
		}
	}

	return count
}

// Stats returns the key statistics for memory usage. If the translation
// count is zero, then virtual memory is off.
func (m *Memory) Stats() (translations, reads, writes, pageReads, pageWrites int64) {
	translations = m.translationCount
	reads = m.singlebyteReadCount
	writes = m.singlebyteWriteCount
	pageReads = m.multibyteReadCount
	pageWrites = m.multibyteWriteCount

	return
}

// Size returns the number of bytes of RAM.
func (m *Memory) Size() uint32 {
	return uint32(len(m.ram))
}

// PhysicalAddressError reports an access to a physical address with no
// backing storage in this Memory.
//
// This is Go's standard way of defining a custom error type: any type with
// an `Error() string` method satisfies the built-in `error` interface, so
// a *PhysicalAddressError can be returned anywhere an `error` is expected
// (as phys does just below), while still letting a caller who specifically
// wants the address recover it — e.g. via `errors.As(err, &target)` — rather
// than having to parse it back out of the error's text. Compare with
// translate.go's TranslationFault, which follows the same pattern for
// address-translation failures.
type PhysicalAddressError struct {
	Addr uint32
}

func (e *PhysicalAddressError) Error() string {
	return fmt.Sprintf("vm: no physical storage at %#08x", e.Addr)
}

// phys returns the size-byte window of RAM starting at addr, or a
// PhysicalAddressError if any of it falls outside RAM.
//
// The bounds check adds addr and size as uint64 rather than uint32 before
// comparing: addr and size are both already-validated-elsewhere uint32
// values, but addr+size performed in 32-bit arithmetic could itself
// overflow and wrap around to a small number if addr were near the top of
// the 32-bit range, which would make an out-of-bounds access look
// in-bounds. Widening to uint64 first (which can hold the sum of any two
// uint32s without overflowing) avoids that trap — a good example of why
// it's worth pausing on arithmetic involving a fixed-width integer type
// whenever the inputs could be close to that type's maximum value.
//
// The return value m.ram[addr : addr+size] is a Go slice expression:
// rather than copying bytes, it produces a new slice header that shares
// the *same underlying array* as m.ram, just restricted to the half-open
// range [addr, addr+size). Modifying a byte through the returned slice
// (as readPhysLongword/writePhysLongword's binary.LittleEndian calls do
// below) therefore modifies m.ram itself in place — there is no hidden
// copy.
func (m *Memory) phys(addr uint32, size uint32) ([]byte, error) {
	if uint64(addr)+uint64(size) > uint64(len(m.ram)) {
		return nil, &PhysicalAddressError{Addr: addr}
	}

	return m.ram[addr : addr+size], nil
}

// readPhysLongword and writePhysLongword give translate.go direct,
// untranslated access to a page table entry's storage (PTEs live at
// physical addresses computed from P0BR/P1BR/SBR, not virtual ones).
//
// Both use the standard library's encoding/binary package to convert
// between a 4-byte window of memory and a uint32. This matters because VAX
// (like x86, and unlike some other historical architectures) is a
// little-endian machine: a multi-byte value's *least* significant byte is
// stored at the *lowest* address. binary.LittleEndian.Uint32(b) reads
// b[0], b[1], b[2], b[3] and combines them as
// b[0] | b[1]<<8 | b[2]<<16 | b[3]<<24; PutUint32 is the exact reverse.
// Every multi-byte accessor elsewhere in this package (storage.go's
// LoadWord/LoadLongword/LoadQuadword and their Store counterparts) follows
// the same convention for the same reason — see storage.go's own comments
// for more on why the C source's extra byte-order handling isn't needed
// here.

func (m *Memory) readPhysLongword(addr uint32) (uint32, error) {
	b, err := m.phys(addr, 4)
	if err != nil {
		return 0, err
	}

	m.multibyteReadCount++

	return binary.LittleEndian.Uint32(b), nil
}

func (m *Memory) writePhysLongword(addr uint32, v uint32) error {
	b, err := m.phys(addr, 4)
	if err != nil {
		return err
	}

	m.multibyteWriteCount++

	binary.LittleEndian.PutUint32(b, v)

	return nil
}
