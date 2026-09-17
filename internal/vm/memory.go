package vm

import (
	"encoding/binary"
	"fmt"
)

// Memory is a VAX machine's physical memory: a flat byte array addressed
// 0..Size()-1. It owns the raw storage itself (the Phase 02 open question on
// vm/vax.CPU coupling is resolved this way: internal/vax.CPU is registers/PSL
// only, and Memory is independent of it, taking a *vax.CPU as a parameter
// wherever address translation needs to consult MAPEN/base-and-length
// registers/cur_mod — see translate.go and storage.go).
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
	ram              []byte
	pageMap          []bool
	vmValid          bool
	translationCount int64
	readCount        int64
	writeCount       int64
}

// NewMemory returns a Memory with size bytes of zeroed RAM and every
// physical page free.
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
func (m *Memory) Stats() (translations int64, reads int64, writes int64) {
	translations = m.translationCount
	reads = m.readCount
	writes = m.writeCount

	return
}

// Size returns the number of bytes of RAM.
func (m *Memory) Size() uint32 {
	return uint32(len(m.ram))
}

// PhysicalAddressError reports an access to a physical address with no
// backing storage in this Memory.
type PhysicalAddressError struct {
	Addr uint32
}

func (e *PhysicalAddressError) Error() string {
	return fmt.Sprintf("vm: no physical storage at %#08x", e.Addr)
}

// phys returns the size-byte window of RAM starting at addr, or a
// PhysicalAddressError if any of it falls outside RAM.
func (m *Memory) phys(addr uint32, size uint32) ([]byte, error) {
	if uint64(addr)+uint64(size) > uint64(len(m.ram)) {
		return nil, &PhysicalAddressError{Addr: addr}
	}

	return m.ram[addr : addr+size], nil
}

// readPhysLongword and writePhysLongword give translate.go direct,
// untranslated access to a page table entry's storage (PTEs live at
// physical addresses computed from P0BR/P1BR/SBR, not virtual ones).

func (m *Memory) readPhysLongword(addr uint32) (uint32, error) {
	b, err := m.phys(addr, 4)
	if err != nil {
		return 0, err
	}

	m.readCount++

	return binary.LittleEndian.Uint32(b), nil
}

func (m *Memory) writePhysLongword(addr uint32, v uint32) error {
	b, err := m.phys(addr, 4)
	if err != nil {
		return err
	}

	m.writeCount++

	binary.LittleEndian.PutUint32(b, v)

	return nil
}
