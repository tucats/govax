package corevms

import (
	"errors"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// The table area backed on demand (docs/PHASE-67 - address space under
// paging.md, subtask 4).
//
// A process's two page tables share one area of S0 (ProcessSpace): P0's
// entries from the area's bottom up, P1's from its top down. Every P0 or
// P1 address the CPU translates is looked up there, through the area's
// own S0 addresses, so an area page holding the entry of a page that
// exists must have a frame behind it. With paging off every page of the
// area has one from the start, since every page of the regions exists.
// With paging on only the regions' created pages exist (regions.go), and
// most of the area holds nothing but zero entries beyond the regions'
// lengths: VMINIT's default sizes give process 1 an area of 192 pages
// when a small image needs 33 of them. So with paging on an area page has
// a frame only while it holds an entry of a page inside a region's length
// (tablePageNeeded): setRegionLength backs the pages a growing region
// reaches *before* the length register lets the hardware reach them (so
// page tables never fault, PHASE-62's Decision 13), and gives back those
// a shrinking region leaves after the length goes down. An area page
// without a frame has the all-zero S0 PTE, no access, so a stray
// reference to it is an access violation rather than a page fault.
//
// A table page's frame is a system page, tied to no virtual page: never
// in a working set, never paged, never on a list until it's given back.
// On VMS the tables are in the process header, whose pages are in the
// working set and can be paged (the book's 14.1.1); govax's never are,
// so its working sets and page counts differ from VMS's by those pages
// (testdata/pg65's and pg66's listed differences).

// errNoTableFrame is the error for a table page no frame could be found
// for: physical memory has none free.
var errNoTableFrame = errors.New("no free page frame for a page table page")

// tablePageNeeded reports whether the area page at S0 address va holds
// the entry of a page that exists with P0 and P1 length registers p0lr
// and p1lr: one of P0's pages 0 to p0lr-1, or of P1's from p1lr up.
func (s *ProcessSpace) tablePageNeeded(va, p0lr, p1lr uint32) bool {
	page := va &^ pageMask

	if p0lr > 0 && page >= s.P0BR&^pageMask && page <= (s.P0BR+p0lr*4-1)&^pageMask {
		return true
	}

	return p1lr < regionPages && page >= (s.P1BR+p1lr*4)&^pageMask && page < s.areaEnd()
}

// fitTableArea gives the pages of s's table area frames or takes them
// away to suit its present lengths: with paging off every page has a
// cleared frame (each region has every one of its pages), and with
// paging on the pages tablePageNeeded names have and the rest don't.
// BuildAddressSpace uses it on a new area, whose pool pages may have
// frames or not from an earlier use.
func (sys *System) fitTableArea(s *ProcessSpace) error {
	paging := sys.mem.Paging() != nil

	for va := s.P0BR &^ pageMask; va < s.areaEnd(); va += pageSize {
		if paging && !s.tablePageNeeded(va, s.P0LR, s.P1LR) {
			sys.unbackS0Page(va)

			continue
		}

		if err := sys.backS0Page(va, true); err != nil {
			return err
		}
	}

	return nil
}

// TrimTableArea, with paging on, takes the frames away from the pages of
// s's table area that hold no entry of a page inside a region's length:
// VMINIT's tables for process 1, laid out on frames VMINIT reserved,
// whose regions start with no P0 pages and only P1's permanent part.
// Those pages' entries are all zero, so nothing is lost. With paging off
// it does nothing.
func (sys *System) TrimTableArea(s *ProcessSpace) {
	if sys.mem.Paging() == nil || s == nil {
		return
	}

	for va := s.P0BR &^ pageMask; va < s.areaEnd(); va += pageSize {
		if !s.tablePageNeeded(va, s.P0LR, s.P1LR) {
			sys.unbackS0Page(va)
		}
	}
}

// releaseTableArea gives back s's table area to the pool, and with
// paging on the frames behind its pages (FreeS0).
func (sys *System) releaseTableArea(s *ProcessSpace) {
	if s.Table == 0 {
		return
	}

	sys.FreeS0(s.Table)
	s.Table = 0
}

// changedTablePages returns the S0 addresses of the first and last area
// pages holding the entries a region's length register passes over as it
// goes from old to lr: P0's (region 0) or P1's (region 1) entries
// between the two. ok is false when the length doesn't change.
func (s *ProcessSpace) changedTablePages(region int, old, lr uint32) (first, last uint32, ok bool) {
	if old == lr {
		return 0, 0, false
	}

	base := s.P0BR
	if region == regionP1 {
		base = s.P1BR
	}

	lo, hi := min(old, lr), max(old, lr)

	return (base + lo*4) &^ pageMask, (base + hi*4 - 1) &^ pageMask, true
}

// backTablePages, with paging on, gives a frame to each area page from
// first to last that holds an entry of a page inside the lengths p0lr
// and p1lr and has none, before the length registers change to them. It
// reports false if physical memory has no frame for one (the pages
// backed before it keep theirs, holding zero entries, and are given back
// when a later change finds them unneeded).
func (sys *System) backTablePages(s *ProcessSpace, first, last, p0lr, p1lr uint32) bool {
	for va := first; va <= last; va += pageSize {
		if s.tablePageNeeded(va, p0lr, p1lr) && sys.backS0Page(va, false) != nil {
			return false
		}
	}

	return true
}

// unbackTablePages, with paging on, takes the frame away from each area
// page from first to last that holds no entry of a page inside the
// lengths p0lr and p1lr, after the length registers have changed to
// them.
func (sys *System) unbackTablePages(s *ProcessSpace, first, last, p0lr, p1lr uint32) {
	for va := first; va <= last; va += pageSize {
		if !s.tablePageNeeded(va, p0lr, p1lr) {
			sys.unbackS0Page(va)
		}
	}
}

// backS0Page makes sure the S0 page at va has a frame behind it, a
// system page tied to nothing, and S0's default protection, URKW (every
// mode reads, kernel writes). A page without one gets a frame that reads
// as zeros; with clear set, one that had a frame is cleared too.
func (sys *System) backS0Page(va uint32, clear bool) error {
	_, _, pte, err := sys.mem.LookupPTE(sys.cpu, va)
	if err != nil {
		return err
	}

	if pte.Valid() {
		if clear {
			var zero [pageSize]byte

			if err := sys.mem.StorePhysical(pte.PFN()*pageSize, zero[:]); err != nil {
				return err
			}
		}

		if pte.Protection() == vm.ProtURKW {
			return nil
		}

		pte.SetProtection(vm.ProtURKW)

		return sys.mem.StorePTE(sys.cpu, va, pte)
	}

	pfn, ok := sys.mem.AllocateFrame(vm.FrameClaim{})
	if !ok {
		return errNoTableFrame
	}

	return sys.mem.StorePTE(sys.cpu, va, vm.ValidPTE(pfn, vm.ProtURKW, uint8(registers.Kernel)))
}

// unbackS0Page takes the frame away from the S0 page at va, if it has
// one: the page gets the all-zero PTE (no access; storing it empties the
// page's translation buffer entry) and its frame is freed.
func (sys *System) unbackS0Page(va uint32) {
	_, _, pte, err := sys.mem.LookupPTE(sys.cpu, va)
	if err != nil || !pte.Valid() {
		return
	}

	if sys.mem.StorePTE(sys.cpu, va, 0) == nil {
		sys.mem.FreePage(pte.PFN())
	}
}

// TableFrames is how many pages of s's table area have frames behind
// them for each region: those holding entries of P0's pages, and of P1's
// (a page holding both, where the regions meet, counts for each). SHOW
// MEMORY shows them.
func (sys *System) TableFrames(s *ProcessSpace) (p0, p1 int) {
	for va := s.P0BR &^ pageMask; va < s.areaEnd(); va += pageSize {
		_, _, pte, err := sys.mem.LookupPTE(sys.cpu, va)
		if err != nil || !pte.Valid() {
			continue
		}

		if s.tablePageNeeded(va, s.P0LR, regionPages) {
			p0++
		}

		if s.tablePageNeeded(va, 0, s.P1LR) {
			p1++
		}
	}

	return p0, p1
}
