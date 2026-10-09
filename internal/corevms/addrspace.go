package corevms

import (
	"encoding/binary"
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// A process's address space (Phase 43): the P0 and P1 page tables that
// give it a program region and a control region of its own.
//
// Every process sees the same S0, but P0 (its image and heap, growing up
// from address 0) and P1 (its stacks and the P1 vector, growing down from
// 0x80000000) are private: each process has its own two page tables, in
// S0 pages from the System's pool, and the CPU's P0BR/P0LR/P1BR/P1LR
// registers name the current process's. Process 1, the console's,
// adopts the tables VMINIT built; BuildAddressSpace lays out the same
// kind of tables for a new process, and TeardownAddressSpace gives back
// everything a process's address space used.

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
		pte.SetOwner(uint8(vax.Kernel))

		return pte
	}

	pte.SetProtection(vm.ProtUW)
	pte.SetOwner(uint8(vax.User))

	return pte
}

// ProcessSpace is one process's address space, and what's needed to give
// it back: where its page tables are, and which of its pages are shared
// with other processes (whose physical pages it mustn't free).
type ProcessSpace struct {
	// AddressSpace is what the CPU's registers hold while the process
	// runs, and what Go code passes to vm.Memory's ...In methods to reach
	// the process's memory while it isn't current.
	vm.AddressSpace

	// P0Pages and P1Pages are how many pages each region's table
	// describes.
	P0Pages, P1Pages uint32

	// P0Table and P1Table are the S0 addresses of the pool allocations
	// holding the tables. Zero for process 1, whose tables are VMINIT's
	// and are never given back.
	P0Table, P1Table uint32

	// shared lists the physical page numbers mapped into this space from
	// another process's (the P1 vector's); teardown leaves them alone.
	shared map[uint32]bool
}

// Owned reports whether the space's page tables came from the S0 pool
// (a process BuildAddressSpace made) rather than from VMINIT.
func (s *ProcessSpace) Owned() bool { return s.P0Table != 0 }

// AdoptAddressSpace returns process 1's address space: the tables VMINIT
// built, as the CPU's registers describe them, p0Pages and p1Pages long.
func AdoptAddressSpace(cpu *vax.CPU, p0Pages, p1Pages uint32) *ProcessSpace {
	return &ProcessSpace{AddressSpace: vm.CurrentAddressSpace(cpu), P0Pages: p0Pages, P1Pages: p1Pages}
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
	if sys.cpu.PR(vax.MAPEN) == 0 {
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

	return nil
}

// MinP1Pages is the smallest P1 table that maps a process's initial user
// stack: UserStackTop (0x7FE00000) is 4096 pages below P1's top, so the
// table must describe those pages and the one below them, where the first
// push lands. (The P1 vector, 0x7FFEDE00 up, is within them.) A smaller
// table is allowed, for a process that never runs user-mode code on that
// stack, but the stack pointer then points at nothing.
const MinP1Pages = (0x80000000-UserStackTop)/pageSize + 1

// BuildAddressSpace makes a new process's address space: P0 and P1 page
// tables of p0Pages and p1Pages entries, in S0 pages from the System's
// pool (charged to pid), each entry a demand-zero page as ProcessPTE
// says. The P1 vector's pages (ShareP1) are mapped onto the physical pages
// process 1's vector uses, read-only to every mode (ProtUR) and owned by
// kernel mode, so the process can call system services through it but
// can neither change it nor delete it.
//
// The tables are written through their S0 addresses, so memory mapping
// must be on (VMINIT has run). On an error, nothing stays allocated.
func (sys *System) BuildAddressSpace(pid, p0Pages, p1Pages uint32) (*ProcessSpace, error) {
	if p0Pages == 0 || p1Pages == 0 || p0Pages > regionPages || p1Pages > regionPages {
		return nil, fmt.Errorf("P0 and P1 need 1 to %d pages each (%d and %d asked for)", regionPages, p0Pages, p1Pages)
	}

	p0Table, err := sys.AllocateS0(tablePages(p0Pages), pid, "P0 page table")
	if err != nil {
		return nil, err
	}

	p1Table, err := sys.AllocateS0(tablePages(p1Pages), pid, "P1 page table")
	if err != nil {
		sys.s0.Free(p0Table)

		return nil, err
	}

	p1lr := regionPages - p1Pages

	s := &ProcessSpace{
		AddressSpace: vm.AddressSpace{
			P0BR: p0Table, P0LR: p0Pages,
			// P1BR is where entry 0 *would* be: the table holds only the
			// entries from page P1LR up, and its first one is P1LR's.
			P1BR: p1Table - p1lr*4, P1LR: p1lr,
		},
		P0Pages: p0Pages, P1Pages: p1Pages,
		P0Table: p0Table, P1Table: p1Table,
		shared: map[uint32]bool{},
	}

	err = sys.writeTable(p0Table, p0Pages, func(i uint32) vm.PTE { return ProcessPTE(false, i) })

	if err == nil {
		err = sys.writeTable(p1Table, p1Pages, func(i uint32) vm.PTE { return ProcessPTE(true, p1lr+i) })
	}

	if err == nil {
		err = sys.mapShared(s)
	}

	if err != nil {
		sys.s0.Free(p0Table)
		sys.s0.Free(p1Table)

		return nil, err
	}

	return s, nil
}

// tablePages is how many pages a page table of n entries (4 bytes each,
// 128 to a page) fills.
func tablePages(n uint32) uint32 {
	return (n*4 + pageMask) / pageSize
}

// writeTable fills the page table at the S0 address table with n entries,
// entry i being pte(i).
func (sys *System) writeTable(table, n uint32, pte func(uint32) vm.PTE) error {
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
		pte.SetOwner(uint8(vax.Kernel))

		if err := sys.mem.StorePTEIn(sys.cpu, s.AddressSpace, sp.va, pte); err != nil {
			return err
		}

		s.shared[sp.pfn] = true
	}

	return nil
}

// TeardownAddressSpace gives back everything s uses: every physical page
// its P0 and P1 pages were given (but not the shared ones; a global
// section's page only loses the section a reference), then its page
// tables' S0 pages. It does nothing for process 1's space, whose tables
// are VMINIT's. The process must not be the current one: the CPU's
// registers, and the translation buffer, would still describe tables
// that no longer exist.
func (sys *System) TeardownAddressSpace(s *ProcessSpace) error {
	if s == nil || !s.Owned() {
		return nil
	}

	cur := vm.CurrentAddressSpace(sys.cpu)
	if cur == s.AddressSpace {
		return fmt.Errorf("can't tear down the current process's address space")
	}

	for _, t := range []struct{ table, n uint32 }{{s.P0Table, s.P0Pages}, {s.P1Table, s.P1Pages}} {
		buf := make([]byte, t.n*4)

		if err := sys.mem.LoadIn(sys.cpu, cur, t.table, buf); err != nil {
			return err
		}

		for i := range t.n {
			pte := vm.PTE(binary.LittleEndian.Uint32(buf[i*4:]))

			if gi, ok := pte.GlobalIndex(); ok {
				sys.unrefGlobalPage(gi)
			} else if pte.Valid() && !s.shared[pte.PFN()] {
				sys.releaseFrame(pte.PFN(), pte.Modified())
			}
		}

		sys.s0.Free(t.table)
	}

	s.P0Table, s.P1Table = 0, 0

	return nil
}
