package corevms

import (
	"fmt"
	"slices"
)

// The S0 pool (Phase 43, subtask 6): the system-space pages a new process's
// structures are carved from.
//
// System space (S0, virtual addresses from 0x80000000 up) is the one part
// of the address space every process maps the same way, so it's where
// anything the system must reach whichever process is running has to
// live. VMINIT lays out what process 1 needs there (its page tables, its
// privileged stacks) and the console's ASM puts the microkernel after
// that. Each further process needs the same kinds of structures of its
// own -- P0 and P1 page tables, kernel, executive, and supervisor stacks,
// and a hardware PCB -- and they come from the S0 pages left over, which
// this pool hands out.
//
// VMINIT maps every S0 page to a physical page up front, so a pool page
// already has memory behind it: allocating one only decides who uses it.
// Allocations are contiguous runs of pages (a page table must be), taken
// first fit from the bottom of the pool, and each records the process it
// belongs to and what it's for, so a process's pages can all be freed
// when it's deleted, and SHOW MEMORY can list them.
//
// How many processes fit, with vax.init's VMINIT /P0=16384 /P1=8192
// /S0=8192 /KSP=20: S0 is 8192 pages (4 MB). VMINIT's own layout takes
// pages 0-311 and the microkernel the next 11, so the pool is pages
// 323-8191, 7869 pages (TestS0PoolAfterVaxInit measures it; the start
// moves with the microkernel's size). A process with VMINIT's sizes needs
// 129 P0 page-table pages (16384 PTEs, 4 bytes each, 128 to a page, plus
// the page VMINIT's layout rounds up by), 65 P1 page-table pages, and 38
// stack pages (20 kernel, 1 guard and 8 executive, 1 guard and 8
// supervisor), and a page for its PCB: 233 pages, so 33 processes fit
// beside process 1. Smaller page tables for subprocesses would let more
// fit (an open question for subtask 8).

// S0Allocation is one run of pool pages handed out by the S0 pool.
type S0Allocation struct {
	Addr    uint32 // virtual address of the first page
	Pages   uint32 // length in pages
	PID     uint32 // the process it belongs to
	Purpose string // what it holds ("P0 page table", "kernel stack", ...)
}

// End returns the virtual address just past the allocation's last page.
func (a S0Allocation) End() uint32 { return a.Addr + a.Pages*pageSize }

// S0ExhaustedError reports that the S0 pool had no free run of pages long
// enough for an allocation.
type S0ExhaustedError struct {
	Pages   uint32 // pages asked for
	Largest uint32 // the longest free run there was
}

func (e *S0ExhaustedError) Error() string {
	return fmt.Sprintf("S0 exhausted: %d contiguous pages needed, %d free at most", e.Pages, e.Largest)
}

// S0Pool hands out runs of S0 pages from [base, limit). The zero value is
// unusable; NewS0Pool makes one.
type S0Pool struct {
	base, limit uint32

	// allocs is every run handed out, in address order, so the free runs
	// are the gaps between neighbors (and before the first and after the
	// last).
	allocs []S0Allocation
}

// NewS0Pool returns a pool of the S0 pages from virtual address base up to
// (not including) limit. base is rounded up to a page boundary.
func NewS0Pool(base, limit uint32) *S0Pool {
	return &S0Pool{base: roundToPage(base), limit: limit}
}

func roundToPage(addr uint32) uint32 {
	return (addr + pageSize - 1) &^ (pageSize - 1)
}

// Range returns the pool's bounds: its first page's address and the
// address just past its last.
func (p *S0Pool) Range() (base, limit uint32) { return p.base, p.limit }

// Claim moves the start of the pool up to addr (rounded up to a page), for
// S0 the console's ASM has filled since the pool was made: the microkernel
// is assembled into S0 just above VMINIT's layout, where the pool starts.
// Claiming below the current start does nothing; claiming over pages
// already allocated is refused.
func (p *S0Pool) Claim(addr uint32) error {
	addr = roundToPage(addr)
	if addr <= p.base {
		return nil
	}

	if len(p.allocs) > 0 && p.allocs[0].Addr < addr {
		a := p.allocs[0]

		return fmt.Errorf("S0 %08X-%08X is in use by process %08X (%s)", a.Addr, a.End()-1, a.PID, a.Purpose)
	}

	p.base = min(addr, p.limit)

	return nil
}

// Allocate hands out pages contiguous S0 pages for process pid, recording
// purpose, and returns the first page's virtual address. The lowest free
// run long enough is used. The pages are not cleared; System.AllocateS0
// clears them.
func (p *S0Pool) Allocate(pages, pid uint32, purpose string) (uint32, error) {
	if pages == 0 {
		return 0, fmt.Errorf("S0 allocation of zero pages")
	}

	size := pages * pageSize
	start := p.base
	largest := uint32(0)

	// Walk the gaps in address order: before each allocation, then after
	// the last one. i is where a new allocation goes in the list.
	for i := 0; i <= len(p.allocs); i++ {
		end := p.limit
		if i < len(p.allocs) {
			end = p.allocs[i].Addr
		}

		if end-start >= size {
			a := S0Allocation{Addr: start, Pages: pages, PID: pid, Purpose: purpose}
			p.allocs = slices.Insert(p.allocs, i, a)

			return start, nil
		}

		largest = max(largest, (end-start)/pageSize)

		if i < len(p.allocs) {
			start = p.allocs[i].End()
		}
	}

	return 0, &S0ExhaustedError{Pages: pages, Largest: largest}
}

// Free returns the run that starts at addr to the pool. It reports whether
// there was one.
func (p *S0Pool) Free(addr uint32) bool {
	for i, a := range p.allocs {
		if a.Addr == addr {
			p.allocs = slices.Delete(p.allocs, i, i+1)

			return true
		}
	}

	return false
}

// FreeProcess returns every run belonging to process pid to the pool, and
// reports how many pages that was.
func (p *S0Pool) FreeProcess(pid uint32) uint32 {
	freed := uint32(0)

	p.allocs = slices.DeleteFunc(p.allocs, func(a S0Allocation) bool {
		if a.PID == pid {
			freed += a.Pages

			return true
		}

		return false
	})

	return freed
}

// Allocations returns a copy of every run handed out, in address order.
func (p *S0Pool) Allocations() []S0Allocation { return slices.Clone(p.allocs) }

// FreePages returns how many of the pool's pages aren't allocated.
func (p *S0Pool) FreePages() uint32 {
	free := (p.limit - p.base) / pageSize

	for _, a := range p.allocs {
		free -= a.Pages
	}

	return free
}

// SetS0Pool gives the System its S0 pool. The console makes one at VMINIT,
// which is when S0's layout is known; before that (after INIT or ZERO alone)
// there is none, and AllocateS0 fails.
func (sys *System) SetS0Pool(p *S0Pool) { sys.s0 = p }

// S0Pool returns the System's S0 pool, or nil before VMINIT.
func (sys *System) S0Pool() *S0Pool { return sys.s0 }

// AllocateS0 allocates pages contiguous S0 pages from the pool for process
// pid (see S0Pool.Allocate) and clears them, so a page table made there
// starts with every entry invalid and a stack starts zeroed. Each page is
// cleared through its physical address, found from the S0 page table,
// so it works whatever the current process is.
func (sys *System) AllocateS0(pages, pid uint32, purpose string) (uint32, error) {
	if sys.s0 == nil {
		return 0, fmt.Errorf("no S0 pool: VMINIT has not been run")
	}

	addr, err := sys.s0.Allocate(pages, pid, purpose)
	if err != nil {
		return 0, err
	}

	var zero [pageSize]byte

	for i := uint32(0); i < pages; i++ {
		va := addr + i*pageSize

		_, _, pte, err := sys.mem.LookupPTE(sys.cpu, va)
		if err == nil && !pte.Valid() {
			err = fmt.Errorf("S0 page %08X is not mapped", va)
		}

		if err == nil {
			err = sys.mem.StorePhysical(pte.PFN()*pageSize, zero[:])
		}

		if err != nil {
			sys.s0.Free(addr)

			return 0, err
		}
	}

	return addr, nil
}
