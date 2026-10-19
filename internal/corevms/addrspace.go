package corevms

import (
	"encoding/binary"
	"fmt"

	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// A process's address space (Phase 43): the P0 and P1 page tables that
// give it a program region and a control region of its own.
//
// Every process sees the same S0, but P0 (its image and heap, growing up
// from address 0) and P1 (its stacks and the P1 vector, growing down from
// 0x80000000) are private: each process has its own two page tables, in
// one table area of S0 pages from the System's pool (ProcessSpace), and
// the CPU's P0BR/P0LR/P1BR/P1LR registers name the current process's.
// Process 1, the console's, adopts the tables VMINIT built;
// BuildAddressSpace lays out the same kind of tables for a new process,
// and TeardownAddressSpace gives back everything a process's address
// space used.

// regionPages is how many pages each of P0 and P1 has: each region is a
// gigabyte, 0x40000000 bytes. A P1 page table describes only the
// region's top pages, so P1LR, the lowest page it describes, is
// regionPages minus the table's length.
const regionPages = 0x40000000 / pageSize

// ProcessPTE is the page-table entry a process's page starts with, for P0
// (p1 false) or P1 (p1 true) page number page: no physical page yet (the
// valid bit is clear, so the first touch allocates a zeroed page, "demand
// zero"), every access mode may read and write it, and it belongs to user
// mode (the PTE's owner field, which $CRETVA and $DELTVA check before
// replacing or deleting a page; the hardware ignores it). P0's page 0 is
// the exception: no mode may touch it, so that a program following a null
// pointer faults instead of reading whatever is at address 0.
//
// VMINIT builds process 1's tables from it, and BuildAddressSpace every
// other process's, so all processes start alike.
func ProcessPTE(p1 bool, page uint32) vm.PTE {
	var pte vm.PTE

	if !p1 && page == 0 {
		pte.SetProtection(vm.ProtNA)
		pte.SetOwner(uint8(registers.Kernel))

		return pte
	}

	pte.SetProtection(vm.ProtUW)
	pte.SetOwner(uint8(registers.User))

	return pte
}

// ProcessSpace is one process's address space, and what's needed to give
// it back: where its page tables are, and which of its pages are shared
// with other processes (whose physical pages it mustn't free).
//
// Its two page tables share one area of S0, as on VMS (the book's
// 14.1.1 and figure 14-2): P0's table starts at the area's bottom (P0BR)
// and grows up, P1's ends at the area's top and grows down, and either
// region may grow into the room between them. Entries is how many pages
// the two regions may have together (VMS's VIRTUALPAGECNT): P0 can grow
// as far as P1's size leaves room for, and P1 as far as P0's does
// (P0Capacity, P1Floor). With paging off neither region moves, and the
// area is exactly the two tables. With paging on only the area's pages
// that hold entries of the regions' pages have frames behind them
// (tablearea.go).
type ProcessSpace struct {
	// AddressSpace is what the CPU's registers hold while the process
	// runs, and what Go code passes to vm.Memory's ...In methods to reach
	// the process's memory while it isn't current.
	vm.AddressSpace

	// Entries is how many PTEs the table area holds for the two regions
	// together.
	Entries uint32

	// Table is the S0 address of the pool allocation holding the table
	// area. Zero for process 1, whose tables are VMINIT's and are never
	// given back.
	Table uint32

	// PID is the process the space was built for (0 for process 1's):
	// the owner its pages' frames are tied to, with paging on.
	PID uint32

	// shared lists the physical page numbers mapped into this space from
	// another process's (the P1 vector's); teardown leaves them alone.
	shared map[uint32]bool
}

// Owned reports whether the space's page tables came from the S0 pool
// (a process BuildAddressSpace made) rather than from VMINIT.
func (s *ProcessSpace) Owned() bool { return s.Table != 0 }

// P1Pages is how many pages P1 has now: those from P1LR up.
func (s *ProcessSpace) P1Pages() uint32 { return regionPages - min(s.P1LR, regionPages) }

// P0Capacity is how many P0 pages the table area can describe with P1
// as long as it is now: how far P0 can grow.
func (s *ProcessSpace) P0Capacity() uint32 {
	return s.Entries - min(s.P1Pages(), s.Entries)
}

// P1Floor is the lowest P1 page number the table area can describe with
// P0 as long as it is now: how far P1 can grow down.
func (s *ProcessSpace) P1Floor() uint32 {
	return regionPages - (s.Entries - min(s.P0LR, s.Entries))
}

// areaEnd is the S0 address just past the table area: where P1's table,
// whose last entry is P1's top page, ends.
func (s *ProcessSpace) areaEnd() uint32 { return s.P1BR + regionPages*4 }

// AdoptAddressSpace returns process 1's address space: the tables VMINIT
// built, as the CPU's registers describe them, for p0Pages and p1Pages
// pages (an area of their sum).
func AdoptAddressSpace(cpu *registers.CPU, p0Pages, p1Pages uint32) *ProcessSpace {
	return &ProcessSpace{AddressSpace: vm.CurrentAddressSpace(cpu), Entries: p0Pages + p1Pages}
}

// sharedPage is one page every process maps to the same physical page.
type sharedPage struct {
	va  uint32 // its virtual (P1) address
	pfn uint32 // the physical page
}

// ShareP1 records the P1 pages from base up to (not including) end, in
// the current process's address space, as pages every new process maps
// too: the P1 vector, whose trampolines (an entry mask, an XFC, and a RET
// for each system service) are the same for every process, so one copy
// in physical memory serves all of them. The console calls it when it
// deposits `.P1VECTOR`'s pages. Each page must already have a physical
// page (the deposit gave it one); a page without is skipped. Without
// memory mapping (before VMINIT) nothing is recorded.
//
// The vector also holds two data cells, SYS$GL_ASTRET and SYS$GL_COMMON,
// that would be per process on VMS; nothing in govax reads or writes
// them, so sharing them is harmless, and a new process maps the pages
// read-only (see BuildAddressSpace).
func (sys *System) ShareP1(base, end uint32) error {
	sys.sharedP1 = nil

	// Before VMINIT there are no page tables, and no processes to share
	// pages with.
	if sys.cpu.PR(registers.MAPEN) == 0 {
		return nil
	}

	as := vm.CurrentAddressSpace(sys.cpu)

	for va := base &^ pageMask; va < end; va += pageSize {
		if va>>30 != 1 {
			return fmt.Errorf("shared page %08X is not in P1", va)
		}

		pte, err := sys.mem.LookupPTEIn(sys.cpu, as, va)
		if err != nil {
			return err
		}

		if pte.Valid() {
			sys.sharedP1 = append(sys.sharedP1, sharedPage{va: va, pfn: pte.PFN()})
		}
	}

	sys.makeSharedSystem(as)

	return nil
}

// makeSharedSystem, with paging on, makes the shared P1 pages system
// pages (the phase doc's Decision 13): they were faulted in as pages of
// the process whose address space as is, but every process maps their
// frames, so they leave its working set and their frames are tied to no
// page, never to be paged.
func (sys *System) makeSharedSystem(as vm.AddressSpace) {
	db := sys.mem.Paging()
	if db == nil {
		return
	}

	owner := sys.processForSpace(as)

	for _, sp := range sys.sharedP1 {
		if owner != nil {
			owner.leaveWorkingSet(sp.va)
		}

		_ = db.MakeSystem(paging.PFN(sp.pfn))
	}
}

// MinP1Pages is the smallest P1 table that maps a process's initial user
// stack: UserStackTop (0x7FE00000) is 4096 pages below P1's top, so the
// table must describe those pages and the one below them, where the first
// push lands. (The P1 vector, 0x7FFEDE00 up, is within them.) A smaller
// table is allowed, for a process that never runs user-mode code on that
// stack, but the stack pointer then points at nothing. (With paging on,
// the user stack grows into the table area's room below P1 instead.)
const MinP1Pages = (0x80000000-UserStackTop)/pageSize + 1

// BuildAddressSpace makes a new process's address space: a table area
// for p0Pages and p1Pages pages, in S0 pages from the System's pool
// (charged to pid), P0's table at its bottom and P1's at its top. With
// paging off each region has exactly its pages, every entry a
// demand-zero page as ProcessPTE says. With paging on the two regions
// share the area's p0Pages + p1Pages entries as they grow (the split
// says only where P1 starts, InitialLengths): only P1's permanent part
// exists at first, every other entry zero (InitialPTE; regions.go), and
// only the area's pages that hold those entries have frames behind them
// (tablearea.go). The P1 vector's pages (ShareP1) are mapped onto the
// physical pages process 1's vector uses, read-only to every mode
// (ProtUR) and owned by kernel mode, so the process can call system
// services through it but can neither change it nor delete it.
//
// The tables are written through their S0 addresses, so memory mapping
// must be on (VMINIT has run). On an error, nothing stays allocated.
func (sys *System) BuildAddressSpace(pid, p0Pages, p1Pages uint32) (*ProcessSpace, error) {
	paging := sys.mem.Paging() != nil

	if p0Pages > regionPages || p1Pages > regionPages || p0Pages+p1Pages == 0 ||
		!paging && (p0Pages == 0 || p1Pages == 0) {
		return nil, fmt.Errorf("P0 and P1 need 1 to %d pages each (%d and %d asked for)", regionPages, p0Pages, p1Pages)
	}

	// The area holds each region's table, a whole number of pages each.
	pages := tablePages(p0Pages) + tablePages(p1Pages)

	table, err := sys.s0.Allocate(pages, pid, "page tables")
	if err != nil {
		return nil, err
	}

	// With paging on the regions start shorter than their tables
	// (InitialLengths).
	p0lr, p1lr := InitialLengths(p0Pages, p1Pages, paging)

	s := &ProcessSpace{
		AddressSpace: vm.AddressSpace{
			P0BR: table, P0LR: p0lr,
			// P1BR is where entry 0 *would* be: P1's table holds only the
			// entries up to page regionPages-1, the last in the area.
			P1BR: table + pages*pageSize - regionPages*4, P1LR: p1lr,
		},
		Entries: p0Pages + p1Pages,
		Table:   table,
		PID:     pid,
		shared:  map[uint32]bool{},
	}

	// The area's pages, backed or not, then the entries of the pages that
	// exist.
	err = sys.fitTableArea(s)

	if err == nil {
		err = sys.writeTable(s.P0BR, p0lr, func(i uint32) vm.PTE { return InitialPTE(false, i, p1lr, paging) })
	}

	if err == nil {
		err = sys.writeTable(s.P1BR+p1lr*4, regionPages-p1lr, func(i uint32) vm.PTE { return InitialPTE(true, p1lr+i, p1lr, paging) })
	}

	if err == nil {
		err = sys.mapShared(s)
	}

	if err != nil {
		sys.releaseTableArea(s)

		return nil, err
	}

	return s, nil
}

// tablePages is how many pages a page table of n entries (4 bytes each,
// 128 to a page) fills.
func tablePages(n uint32) uint32 {
	return (n*4 + pageMask) / pageSize
}

// writeTable writes n page table entries from the S0 address table up,
// entry i being pte(i).
func (sys *System) writeTable(table, n uint32, pte func(uint32) vm.PTE) error {
	if n == 0 {
		return nil
	}

	buf := make([]byte, n*4)

	for i := range n {
		binary.LittleEndian.PutUint32(buf[i*4:], uint32(pte(i)))
	}

	return sys.mem.StoreIn(sys.cpu, vm.CurrentAddressSpace(sys.cpu), table, buf)
}

// mapShared points s's PTEs for the shared P1 pages at their physical
// pages (see BuildAddressSpace).
func (sys *System) mapShared(s *ProcessSpace) error {
	for _, sp := range sys.sharedP1 {
		// A P1 table too short to reach the vector's pages leaves them
		// unmapped: an access is a length violation, as it would be with
		// no vector at all.
		if sp.va&0x3FFFFFFF/pageSize < s.P1LR {
			continue
		}

		var pte vm.PTE

		pte.SetValid(true)
		pte.SetPFN(sp.pfn)
		pte.SetProtection(vm.ProtUR)
		pte.SetOwner(uint8(registers.Kernel))

		if err := sys.mem.StorePTEIn(sys.cpu, s.AddressSpace, sp.va, pte); err != nil {
			return err
		}

		s.shared[sp.pfn] = true
	}

	return nil
}

// TeardownAddressSpace gives back everything s uses: every physical page
// its P0 and P1 pages were given, valid or in transition (but not the
// shared ones; a global section's page only loses the section a
// reference), then its table area's S0 pages and their frames. It does
// nothing for process 1's space, whose tables are VMINIT's. The process
// must not be the current one: the CPU's registers, and the translation
// buffer, would still describe tables that no longer exist.
func (sys *System) TeardownAddressSpace(s *ProcessSpace) error {
	if s == nil || !s.Owned() {
		return nil
	}

	cur := vm.CurrentAddressSpace(sys.cpu)
	if cur.Same(s.AddressSpace) {
		return fmt.Errorf("can't tear down the current process's address space")
	}

	// The entries of the pages that exist: P0's from page 0, P1's from
	// P1LR. (Every entry past a region's length is zero.)
	tables := []struct{ entries, n, base uint32 }{
		{s.P0BR, s.P0LR, 0},
		{s.P1BR + s.P1LR*4, s.P1Pages(), p1Base + s.P1LR*pageSize},
	}

	for _, t := range tables {
		buf := make([]byte, t.n*4)

		if err := sys.mem.LoadIn(sys.cpu, cur, t.entries, buf); err != nil {
			return err
		}

		for i := range t.n {
			pte := vm.PTE(binary.LittleEndian.Uint32(buf[i*4:]))

			if gi, ok := pte.GlobalIndex(); ok {
				sys.unrefGlobalPage(gi)

				continue
			}

			// The page's page file block, then a valid page's frame, or
			// one a page in transition holds.
			sys.releasePageFileBlock(s.PID, t.base+i*pageSize, pte)

			if pfn, modified, ok := sys.heldFrame(s.PID, t.base+i*pageSize, pte); ok && !s.shared[pfn] {
				sys.releaseFrame(pfn, modified)
			}
		}
	}

	sys.releaseTableArea(s)

	return nil
}
