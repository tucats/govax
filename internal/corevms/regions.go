package corevms

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// Created space (docs/PHASE-67 - address space under paging.md, subtask
// 2).
//
// With VMS paging on, a process's P0 and P1 length registers describe
// the pages that exist, as VMS's do (the book's 14.1.1 and 16.2): P0LR is
// the number of P0 pages, from page 0 up, and P1LR the number of the
// lowest P1 page that exists (P1 grows down from the top). A page beyond
// P0's end, or below P1's low end, doesn't exist, and touching it is a
// length violation, an access violation whose reason mask has its length
// bit set. The services that create pages ($CRETVA, $EXPREG, $CRMPSC,
// the image activator, the heap's page pool) lengthen a region to cover
// each page before writing its PTE, and those that delete pages at a
// region's end ($DELTVA, $CNTREG, image rundown) shorten it after. Pages
// inside a region that nothing created (a $CRETVA beyond the end skips
// over some, a $DELTVA in the middle deletes some) have the all-zero PTE:
// no access.
//
// The page tables themselves are still made at their full size when the
// process is (VMINIT, BuildAddressSpace), and every entry past a region's
// length is zero, so a region grows into entries that describe no page.
// The table's size is how far a region can grow: past it is SS$_VASFULL.
//
// A region's length lives in three places, kept together by
// setRegionLength: the process's ProcessSpace, the CPU's length register
// while the process is current, and its hardware PCB's (which the next
// LDPCTX loads) while it isn't. Environment.RegionSize[0] and [1] are
// their byte forms: P0's end and P1's low end.
//
// With paging off nothing here applies: every page of the tables exists
// from the start, the length registers never move, and RegionSize is a
// high-water mark of its own.
//
// Until the user stack is created at image activation (subtask 3), P1's
// permanent part is everything from the bottom of the user stack's
// reserve (userStackBottom) up: those pages exist from the start, and P1
// expansions grow below them.

// errVasFull is storePTE's error for a page its region's page table
// can't reach.
var errVasFull = errors.New("the page table doesn't reach the page")

// userStackBottom is the bottom of the user stack's reserve: the lowest
// address of P1's permanent part, below which P1 expansions grow.
const userStackBottom = UserStackTop - userStackPages*pageSize

// userStackBottomPage is userStackBottom's P1 page number.
const userStackBottomPage = (userStackBottom - p1Base) / pageSize

// createdSpace reports whether env's P0 and P1 are created space: VMS
// paging on, memory mapping enabled, and an address space of the
// process's own (an environment without one, in tests, keeps the
// registers as they are).
func (env *Environment) createdSpace() bool {
	return env.mem != nil && env.mem.Paging() != nil && env.cpu != nil &&
		env.cpu.PR(registers.MAPEN) != 0 && env.Space != nil
}

// initialP1Length is the P1 length register a new address space starts
// with, for a P1 table of p1Pages entries: with paging on, P1's
// permanent part (userStackBottom up), or as much of it as the table
// reaches; with paging off, the whole table.
func initialP1Length(p1Pages uint32, paging bool) uint32 {
	low := uint32(regionPages - p1Pages)
	if paging {
		low = max(low, userStackBottomPage)
	}

	return low
}

// InitialLengths is the P0 and P1 length registers a new address space
// with tables of p0Pages and p1Pages entries starts with, with paging on
// or off: with paging on, no P0 pages and P1's permanent part; with it
// off, both tables whole. VMINIT and BuildAddressSpace use it, and
// InitialPTE says what each entry starts as.
func InitialLengths(p0Pages, p1Pages uint32, paging bool) (p0lr, p1lr uint32) {
	if paging {
		return 0, initialP1Length(p1Pages, true)
	}

	return p0Pages, initialP1Length(p1Pages, false)
}

// InitialPTE is the PTE a new address space's P0 (p1 false) or P1 entry
// for page number page starts with, with paging on or off: ProcessPTE's
// demand-zero page for a page within the initial lengths
// (InitialLengths), and the all-zero PTE, no page, for an entry beyond
// them, into which a region grows.
func InitialPTE(p1 bool, page, p1lr uint32, paging bool) vm.PTE {
	if paging && (!p1 || page < p1lr) {
		return 0
	}

	return ProcessPTE(p1, page)
}

// p0Capacity is how many P0 pages env's P0 table can describe, and
// p1Floor the lowest P1 page number its P1 table can: how far each
// region can grow.
func (env *Environment) p0Capacity() uint32 {
	if env.Space != nil {
		return env.Space.P0Pages
	}

	return env.cpu.PR(registers.P0LR)
}

func (env *Environment) p1Floor() uint32 {
	if env.Space != nil {
		return regionPages - env.Space.P1Pages
	}

	return env.cpu.PR(registers.P1LR)
}

// coverPage, in created space, lengthens the region the P0 or P1 page at
// addr is in so that the page exists, if it doesn't: P0 up to and
// including it, P1 down to it. It reports false if the region's page
// table can't reach the page (SS$_VASFULL, for the caller). Outside
// created space it does nothing.
func (env *Environment) coverPage(addr uint32) bool {
	if !env.createdSpace() || addr >= s0Base {
		return true
	}

	page := (addr &^ p1Base) / pageSize
	sp := env.Space

	if addr < p1Base {
		if page < sp.P0LR {
			return true
		}

		if page >= sp.P0Pages {
			return false
		}

		env.setRegionLength(regionP0, page+1)

		return true
	}

	if page >= sp.P1LR {
		return true
	}

	if page < env.p1Floor() {
		return false
	}

	env.setRegionLength(regionP1, page)

	return true
}

// setRegionLength sets env's P0 length register (region 0) or P1 length
// register (region 1) to lr: in its ProcessSpace, in the CPU if the
// process is current, in its hardware PCB if it isn't, and as
// RegionSize's byte form. A region is shortened only after the pages
// leaving it are deleted: the translation buffer, which a page's
// deletion empties, is consulted before the length check.
func (env *Environment) setRegionLength(region int, lr uint32) {
	sp := env.Space
	current := vm.CurrentAddressSpace(env.cpu).Same(sp.AddressSpace)

	if region == regionP0 {
		sp.P0LR = lr
		env.RegionSize[0] = lr * pageSize

		if current {
			env.cpu.SetPR(registers.P0LR, lr)
		}
	} else {
		sp.P1LR = lr
		env.RegionSize[1] = p1Base + lr*pageSize

		if current {
			env.cpu.SetPR(registers.P1LR, lr)
		}
	}

	if !current && env.Stacks != nil && env.Stacks.PCBB != 0 {
		_ = cpu.WritePCBLengths(env.mem, env.Stacks.PCBB, sp.P0LR, sp.P1LR)
	}

	if env.Process != nil {
		env.Process.VirtualPeak = max(env.Process.VirtualPeak, env.virtualPages())
	}
}

// lookupPTE returns the PTE of the P0 or P1 page at addr for a service
// about to create or replace the page: in created space, through the
// process's own tables, a page beyond its region's length (but within
// the table) being the all-zero PTE it will be covered with; otherwise
// through the CPU's. An error is a page the tables can't reach.
func (env *Environment) lookupPTE(addr uint32) (vm.PTE, error) {
	if !env.createdSpace() {
		_, _, pte, err := env.mem.LookupPTE(env.cpu, addr)

		return pte, err
	}

	if addr < s0Base && !env.inRegion(addr) {
		page := (addr &^ p1Base) / pageSize

		if addr < p1Base && page < env.Space.P0Pages || addr >= p1Base && page >= env.p1Floor() {
			return 0, nil
		}
	}

	return env.mem.LookupPTEIn(env.cpu, env.Space.AddressSpace, addr)
}

// storePTE writes pte as the PTE of the P0 or P1 page at addr, covering
// the page first in created space (coverPage); SS$_VASFULL's error if the
// page table can't reach it.
func (env *Environment) storePTE(addr uint32, pte vm.PTE) error {
	if !env.createdSpace() {
		return env.mem.StorePTE(env.cpu, addr, pte)
	}

	if !env.coverPage(addr) {
		return errVasFull
	}

	return env.mem.StorePTEIn(env.cpu, env.Space.AddressSpace, addr, pte)
}

// inRegion reports whether the P0 or P1 page at addr is within its
// region's length, in created space.
func (env *Environment) inRegion(addr uint32) bool {
	page := (addr &^ p1Base) / pageSize

	if addr < p1Base {
		return page < env.Space.P0LR
	}

	return page >= env.Space.P1LR
}

// contractRegions, in created space, shortens P0 past the nonexistent
// pages at its end and P1 past those at its low end, as a deletion that
// reached a region's end leaves them. $CNTREG's pages go to the same
// deletion routine as $DELTVA's (the book's 16.2.2.3), so both contract a
// region they delete the end of (unconfirmed for $DELTVA: a probe case).
// P1 never shrinks into its permanent part.
func (env *Environment) contractRegions() {
	if !env.createdSpace() {
		return
	}

	sp := env.Space

	p0 := sp.P0LR
	for p0 > 0 && !env.pageAt((p0-1)*pageSize) {
		p0--
	}

	if p0 != sp.P0LR {
		env.setRegionLength(regionP0, p0)
	}

	p1 := sp.P1LR
	for p1 < userStackBottomPage && !env.pageAt(p1Base+p1*pageSize) {
		p1++
	}

	if p1 != sp.P1LR {
		env.setRegionLength(regionP1, p1)
	}
}

// pageAt reports whether env's space has a page at addr: its PTE isn't
// the all-zero one deletion leaves. (A page made no-access by $SETPRT
// still exists, and may hold a section's reference.)
func (env *Environment) pageAt(addr uint32) bool {
	pte, err := env.mem.LookupPTEIn(env.cpu, env.Space.AddressSpace, addr)

	return err == nil && pte != 0
}

// virtualPages is how many P0 and P1 pages env's process has: in created
// space, the two regions' lengths; with paging off, P0 up to its
// high-water mark and P1 from its low end up.
func (env *Environment) virtualPages() uint32 {
	if env.createdSpace() {
		return env.Space.P0LR + regionPages - env.Space.P1LR
	}

	return (env.RegionSize[0]+pageMask)/pageSize + (s0Base-env.p1Low())/pageSize
}

// FreeP0VA is JPI$_FREP0VA: the address of the first page past P0's end,
// where P0 grows (the book's 14.1.1, PHD$L_FREP0VA: P0's table maps 0 to
// FREP0VA-1).
func (env *Environment) FreeP0VA() uint32 {
	if env.createdSpace() {
		return env.Space.P0LR * pageSize
	}

	return (env.RegionSize[0] + pageMask) &^ pageMask
}

// FreeP1VA is JPI$_FREP1VA: the address of the page just below P1's low
// end, the next P1 grows into (PHD$L_FREP1VA: P1's table maps FREP1VA+200
// to 7FFFFFFF). Whether $GETJPI returns this address or the PTE's is
// unconfirmed: the probe's question.
func (env *Environment) FreeP1VA() uint32 {
	return env.p1Low() - pageSize
}

// FreePTECount is JPI$_FREPTECNT: how many more pages the process's page
// tables can describe, P0's and P1's together (PHD$L_FREPTECNT).
func (env *Environment) FreePTECount() uint32 {
	p0 := env.p0Capacity() - min(env.FreeP0VA()/pageSize, env.p0Capacity())
	p1 := (env.p1Low()-p1Base)/pageSize - min(env.p1Floor(), (env.p1Low()-p1Base)/pageSize)

	return p0 + p1
}

// VirtualPeak is JPI$_VIRTPEAK: the most P0 and P1 pages the process has
// had at once (with paging off, only what it has now is known beside the
// changes made with paging on).
func (env *Environment) VirtualPeak() uint32 {
	peak := env.virtualPages()
	if env.Process != nil {
		peak = max(peak, env.Process.VirtualPeak)
	}

	return peak
}

// EnsurePages makes sure the P0 or P1 pages holding the length bytes at
// addr exist, for Go code about to write them into the process's memory
// (a test's program, the console's own writes): in created space each
// page that doesn't exist is created as $CRETVA creates one for user
// mode (demand-zero, read/write, charged against the paging file quota),
// its region growing to cover it; a page that exists is left as it is.
// With paging off every page of the tables exists, and it does nothing.
func (env *Environment) EnsurePages(addr, length uint32) error {
	if !env.createdSpace() || length == 0 {
		return nil
	}

	for page := addr &^ pageMask; page <= (addr+length-1)&^pageMask; page += pageSize {
		if env.inRegion(page) && env.pageAt(page) {
			continue
		}

		if st := env.createPage(page, registers.User); st != ssNormal {
			return vmserrors.New(st)
		}
	}

	return nil
}
