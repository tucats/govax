package corevms

import (
	"slices"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Virtual address space services (docs/PHASE-26.md subtask 35): $CRETVA,
// $DELTVA, and the obsolete $CNTREG.
//
// # A process's address space
//
// A VAX process sees 4GB of virtual addresses in four 1GB regions: P0
// (0-3FFFFFFF, the program region, growing up from 0: the image, its
// data, its heap), P1 (40000000-7FFFFFFF, the control region, growing down
// from the top: the stacks, and VMS's per-process data), and the shared
// system region S0 (80000000 up). Each 512-byte page of P0 and P1 has a
// page table entry (PTE) saying whether the page exists, which access
// modes may read and write it (its protection), which physical page holds
// it (if it's valid, in memory right now), and which access mode *owns*
// it (bits 23-24, a field the hardware ignores and VMS uses to stop a
// program deleting pages that belong to a more privileged mode).
//
// The address space isn't fixed: a program can add pages ($CRETVA "create
// virtual address space", $EXPREG) and remove them ($DELTVA "delete
// virtual address space"). A new page is *demand-zero*: it exists and has
// a protection, but no physical page yet; the first touch gives it a
// physical page full of zeros. A deleted page doesn't exist at all, so
// touching it is an access violation.
//
// # How govax does it
//
// VMINIT builds the P0 and P1 page tables once, at their full size. With
// paging off every page in them exists from the start, demand-zero,
// user-owned, and readable and writable by every mode (except P0's page
// 0, which is no-access), and the length registers never move. With
// paging on the length registers describe the pages that exist, as
// VMS's do, and these services lengthen and shorten the regions as they
// create and delete pages (regions.go: created space). internal/vm
// allocates a physical page on first touch. So these services work
// entirely on PTEs:
//
//   - creating a page writes a demand-zero PTE (not valid, no physical
//     page), protected read/write for the requested access mode and more
//     privileged ones, owned by that mode;
//   - deleting a page writes a PTE of all zeros: not valid, no access,
//     owned by kernel mode, the "no such page" PTE;
//
// and either one gives back the physical page the old PTE had, if any
// (vm.Memory.FreePage), whose contents are thereby discarded. Writing a
// PTE through vm.Memory.StorePTE also flushes the page from the
// translation buffer, so the next access sees the change.
//
// The page tables can't grow, so a page beyond the end of P0's or P1's
// table can't be created: SS$_VASFULL. With paging on, a page beyond a
// region's length but within its table can: the region grows to it.

// Status values the address space services return.
var (
	ssPagOwnVio = vmsdef.Symbols["SS$_PAGOWNVIO"]
	ssVasFull   = vmsdef.Symbols["SS$_VASFULL"]
	ssIllPagCnt = vmsdef.Symbols["SS$_ILLPAGCNT"]
)

// pageSize is the VAX page size, and pageMask the bits of an address
// within its page.
const (
	pageSize = 512
	pageMask = pageSize - 1
)

// s0Base is the first system-region address: pages from here up belong
// to the system, not the process.
const s0Base = 0x80000000

// modeProtections is the protection a created page gets for each access
// mode: read/write for that mode and the more privileged ones, nothing
// for the rest (KW, EW, SW, UW).
var modeProtections = [4]vm.Protection{vm.ProtKW, vm.ProtEW, vm.ProtSW, vm.ProtUW}

// pageRange is the pages an inadr array names, in the order a service
// processes them: first to last, by pageSize steps either up or down.
type pageRange struct {
	first, last uint32
}

// pages returns the page addresses of r in order.
func (r pageRange) pages() []uint32 {
	var out []uint32

	step := int64(pageSize)
	if r.last < r.first {
		step = -step
	}

	for a := int64(r.first); ; a += step {
		out = append(out, uint32(a))

		if uint32(a) == r.last {
			return out
		}
	}
}

// readRange reads the two-longword address array at inadr as a page
// range: the pages holding the first and second addresses, processed
// from first to second if forward, second to first if not.
func (env *Environment) readRange(inadr uint32, forward bool) (pageRange, bool) {
	start, err1 := env.mem.LoadLongword(env.cpu, inadr)
	end, err2 := env.mem.LoadLongword(env.cpu, inadr+4)

	if err1 != nil || err2 != nil {
		return pageRange{}, false
	}

	r := pageRange{first: start &^ pageMask, last: end &^ pageMask}
	if !forward {
		r.first, r.last = r.last, r.first
	}

	return r, true
}

// storeRetadr stores, at retadr (if not 0), the range of the pages done
// (a slice of pages in processing order), as the address-range services
// report it: the byte addresses at both ends of what was done, in the
// order it was done, or -1 twice if nothing was. It reports false if
// retadr can't be written.
func (env *Environment) storeRetadr(retadr uint32, done []uint32) bool {
	if retadr == 0 {
		return true
	}

	first, last := uint32(0xFFFFFFFF), uint32(0xFFFFFFFF)

	if n := len(done); n > 0 {
		first, last = done[0], done[n-1]

		// The range covers whole pages: its high end is the last byte of
		// the highest page.
		if first <= last {
			last |= pageMask
		} else {
			first |= pageMask
		}
	}

	return env.mem.StoreLongword(env.cpu, retadr, first) == nil &&
		env.mem.StoreLongword(env.cpu, retadr+4, last) == nil
}

// replacePTE writes pte as the page table entry for the page at addr,
// accounting for the old entry (releasePTE): giving back the physical
// page it had, if it was valid, or, for a section's page, the section's
// reference (and a private file section's modified page goes back to
// its file).
//
// In created space (regions.go) a page beyond its region's end is covered
// first: the region grows to include it.
func (env *Environment) replacePTE(addr uint32, old, pte vm.PTE) bool {
	if env.storePTE(addr, pte) != nil {
		return false
	}

	env.releasePTE(addr, old)

	return true
}

// pageExists reports whether pte describes a page of the address space:
// one that is valid or has some protection. A deleted page (the all-zero
// PTE) or one never created has neither.
func pageExists(pte vm.PTE) bool {
	return pte.Valid() || pte.Protection() != vm.ProtNA
}

// serviceSysCretva is SYS$CRETVA:
//
//	SYS$CRETVA inadr ,[retadr] ,[acmode]
//
// It creates the pages from inadr's first address to its second (either
// may be the higher) as demand-zero pages owned by acmode (maximized with
// the caller's mode) and read/write for it and the more privileged
// modes. A page that already exists is deleted and created afresh, its
// contents lost, unless a more privileged mode owns it (SS$_PAGOWNVIO).
// retadr receives the range created (see storeRetadr), which on an error
// is what was created before it.
//
// It returns SS$_NORMAL; SS$_NOPRIV for a system-region page;
// SS$_VASFULL for a page beyond the region's page table (with paging on,
// for a P1 range the table area can't hold, before creating any page);
// SS$_PAGOWNVIO;
// SS$_EXQUOTA, with paging on, when the job's paging file quota runs out
// (pgflquota.go); or SS$_ACCVIO if inadr can't be read or retadr written.
//
// Creating P0 pages beyond the high-water mark $EXPREG and LIB$GET_VM
// allocate from (Environment.RegionSize[0]) moves the mark past them, so
// those don't hand out the same pages again.
func serviceSysCretva(env *Environment, argv []uint32) (uint32, error) {
	r, ok := env.readRange(optArg(argv, 0), true)
	if !ok {
		return ssAccVio, nil
	}

	retadr := optArg(argv, 1)
	mode := max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())

	// A P1 range the table area can't hold is SS$_VASFULL with nothing
	// created (spaceHolds); otherwise the pages are created one at a
	// time, and the quota or (in P0) the area's end may stop them
	// partway.
	if !env.spaceHolds(r) {
		if !env.storeRetadr(retadr, nil) {
			return ssAccVio, nil
		}

		return ssVasFull, nil
	}

	done := make([]uint32, 0, len(r.pages()))

	status := uint32(ssNormal)

	for _, addr := range r.pages() {
		if st := env.createPage(addr, mode); st != ssNormal {
			status = st

			break
		}

		done = append(done, addr)

		if addr < s0Base>>1 && addr+pageSize > env.RegionSize[0] {
			env.RegionSize[0] = addr + pageSize
		}
	}

	if !env.storeRetadr(retadr, done) {
		return ssAccVio, nil
	}

	return status, nil
}

// createPage makes the page at addr a new demand-zero page of mode's.
func (env *Environment) createPage(addr uint32, mode registers.AccessMode) uint32 {
	if addr >= s0Base {
		return ssNoPriv
	}

	old, err := env.lookupPTE(addr)
	if err != nil {
		return ssVasFull
	}

	if pageExists(old) && registers.AccessMode(old.Owner()) < mode {
		return ssPagOwnVio
	}

	// A new demand-zero page is charged against the paging file quota
	// (pgflquota.go); one already charged keeps its charge.
	if !env.canChargePage(addr) {
		return ssExQuota
	}

	var pte vm.PTE

	pte.SetProtection(modeProtections[mode])
	pte.SetOwner(uint8(mode))

	if !env.replacePTE(addr, old, pte) {
		return ssVasFull
	}

	env.chargePage(addr, true)

	return ssNormal
}

// serviceSysDeltva is SYS$DELTVA:
//
//	SYS$DELTVA inadr ,[retadr] ,[acmode]
//
// It deletes the pages from inadr's *second* address to its first (the
// reverse of $CRETVA's order, so the same array undoes a $CRETVA in
// reverse), after which touching them is an access violation. A page
// that doesn't exist, or lies beyond the region's page table, is passed
// over as though deleted. A page a mode more privileged than acmode
// (maximized with the caller's) owns can't be deleted (SS$_PAGOWNVIO).
// retadr receives the range deleted, as for $CRETVA.
//
// It returns SS$_NORMAL; SS$_NOPRIV for a system-region page;
// SS$_PAGOWNVIO; or SS$_ACCVIO if inadr can't be read or retadr written.
func serviceSysDeltva(env *Environment, argv []uint32) (uint32, error) {
	r, ok := env.readRange(optArg(argv, 0), false)
	if !ok {
		return ssAccVio, nil
	}

	mode := max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())
	status, done := env.deletePages(r.pages(), mode)

	if !env.storeRetadr(optArg(argv, 1), done) {
		return ssAccVio, nil
	}

	return status, nil
}

// deletePages deletes pages, in order, on behalf of mode, stopping at the
// first that can't be. It returns the status and the pages it did. In
// created space a region whose end it deleted is contracted
// (contractRegions).
func (env *Environment) deletePages(pages []uint32, mode registers.AccessMode) (uint32, []uint32) {
	defer env.contractRegions()

	done := make([]uint32, 0, len(pages))

	for _, addr := range pages {
		if addr >= s0Base {
			return ssNoPriv, done
		}

		old, err := env.lookupPTE(addr)
		if err == nil && pageExists(old) {
			if registers.AccessMode(old.Owner()) < mode {
				return ssPagOwnVio, done
			}

			if !env.replacePTE(addr, old, 0) {
				return ssAccVio, done
			}

			env.forgetPageLocks(addr)
		}

		done = append(done, addr)
	}

	return ssNormal, done
}

// Regions $CNTREG and $EXPREG name by number.
const (
	regionP0 = 0
	regionP1 = 1
)

// p1Base is the first P1 address; P1 pages are numbered from it.
const p1Base = 0x40000000

// serviceSysCntreg is SYS$CNTREG, the obsolete predecessor of $DELTVA
// (the VMS 5.0 manual lists it only as replaced by $DELTVA):
//
//	SYS$CNTREG pagcnt ,[retadr] ,[acmode] ,[region]
//
// It contracts region 0 (P0, the default) or 1 (P1) by deleting pagcnt
// pages from the end the region grows at: P0's top, the high-water mark
// $EXPREG allocates from (Environment.RegionSize[0]), which moves down by
// what was deleted; P1's bottom, the lowest page its length register
// P1LR admits. Deleting works as for $DELTVA (acmode maximized, owner
// checked, pages that don't exist passed over), from the region's end
// inwards, and retadr receives the range deleted, in $EXPREG's order.
//
// It returns SS$_NORMAL; SS$_ILLPAGCNT for a count of 0 or more pages
// than the region has; SS$_PAGOWNVIO; SS$_BADPARAM for another region; or
// SS$_ACCVIO if retadr can't be written.
func serviceSysCntreg(env *Environment, argv []uint32) (uint32, error) {
	pagcnt, retadr := optArg(argv, 0), optArg(argv, 1)
	mode := max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())

	var pages []uint32

	switch optArg(argv, 3) {
	case regionP0:
		top := (env.RegionSize[0] + pageMask) &^ pageMask
		if pagcnt == 0 || pagcnt > top/pageSize {
			return ssIllPagCnt, nil
		}

		for i := range pagcnt {
			pages = append(pages, top-(i+1)*pageSize)
		}

	case regionP1:
		bottom := p1Base + env.pageSpace().P1LR*pageSize
		if pagcnt == 0 || pagcnt > (s0Base-bottom)/pageSize {
			return ssIllPagCnt, nil
		}

		for i := range pagcnt {
			pages = append(pages, bottom+i*pageSize)
		}

	default:
		return ssBadParam, nil
	}

	status, done := env.deletePages(pages, mode)

	// In created space deletePages contracted the region; otherwise P0's
	// mark moves down past what was deleted.
	if optArg(argv, 3) == regionP0 && len(done) > 0 && !env.createdSpace() {
		env.RegionSize[0] = done[len(done)-1]
	}

	// retadr is in the order $EXPREG reports a region's pages, P0's low
	// to high and P1's high to low: the reverse of the deletion's, from
	// the region's end inwards (VMS 7.1, testdata/pg67's A7).
	slices.Reverse(done)

	if !env.storeRetadr(retadr, done) {
		return ssAccVio, nil
	}

	return status, nil
}

func registerAddressSpaceServices(t *ServiceTable) {
	t.Register("SYS$CRETVA", serviceSysCretva)
	t.Register("SYS$DELTVA", serviceSysDeltva)
	t.Register("SYS$CNTREG", serviceSysCntreg)
}

// DeleteP0 is image rundown's deletion of P0 (the book's 21.3,
// MMG$IMGRESET): every P0 page the image created, loaded, or touched is
// deleted, and the frame it held, valid or in transition, is released as
// $DELTVA releases it (releasePTE). In created space (regions.go) each
// page gets the all-zero PTE and P0's length goes to 0, as VMS's image
// reset deletes all of P0. With paging off govax keeps P0's page table
// at its length and gives each page the form a new address space gives
// it (ProcessPTE: demand-zero, read/write, user's; page 0 a no-access
// guard), which the next image sees the same way. Without this, an
// image's pages stayed after its exit, in memory and in the working set,
// and the next image of the same layout found them there
// (testdata/pg63's dry run; VMS's PPGCNT falls back at image exit).
//
// Image activation calls it too, for the pages a rundown with KeepP0 set
// left.
//
// Every page mapped to a section goes first (unmapSections), wherever it
// is, so the sections, and the files they hold (a mapped image's own,
// Phase 66), go with P0; under KeepP0 they stay with it. It deletes the
// P1 pages the image added too (deleteImageP1).
func (env *Environment) DeleteP0() {
	env.unmapSections()

	if env.cpu == nil || env.cpu.PR(registers.MAPEN) == 0 {
		return
	}

	space := env.pageSpace()
	created := env.createdSpace()

	for i := range space.P0LR {
		addr := i * pageSize

		fresh := ProcessPTE(false, i)
		if created {
			fresh = 0
		}

		env.resetPage(space, addr, fresh)
	}

	// Pages $EXPREG or the heap charged but nothing touched still have
	// the PTE they started with, passed over above.
	env.refundPagesBelow(p1Base)

	env.RegionSize[0] = 0

	if created {
		env.setRegionLength(regionP0, 0)
	}

	env.deleteImageP1()

	// Global sections the image's deletions left waiting are released
	// (gblsec.go's maybeDeleteSection).
	env.ReleasePendingSections()
}

// resetPage gives the page at addr, in space, the PTE fresh, releasing
// what its old PTE held (releasePTE). A page already in that form, or
// whose PTE can't be reached, is passed over.
func (env *Environment) resetPage(space vm.AddressSpace, addr uint32, fresh vm.PTE) {
	old, err := env.mem.LookupPTEIn(env.cpu, space, addr)
	if err != nil || old == fresh {
		return
	}

	if env.mem.StorePTEIn(env.cpu, space, addr, fresh) != nil {
		return
	}

	env.releasePTE(addr, old)
}

// deleteImageP1 is image rundown's deletion of the pages the image added
// to P1 ($EXPREG of region 1, a section mapped with SEC$M_EXPREG there,
// and in created space the user stack). VMS deletes them with the image
// (VMS 7.1 in testdata/pg64: PAGFILCNT is back where it was after an
// image that filled its quota with P1 pages); the process's own P1,
// above, stays. With paging off they're those from P1's low end up to
// the bottom of the user stack's reserve (userStackBottom), which stays
// with the process. In created space they're those from P1's low end up
// to UserStackTop, the stack included, but for the stack's live part
// (imageP1Top): the pages from the one the user stack pointer is in up,
// which hold the frames of what called the image (a subprocess's CLI).
// Each page's frame and page file block are released and its charge
// refunded, as DeleteP0's are; in created space it gets the all-zero PTE
// and P1's length goes up past it, and with paging off it goes back to
// ProcessPTE's form.
func (env *Environment) deleteImageP1() {
	created := env.createdSpace()
	if !created && env.RegionSize[1] == 0 {
		return // P1 never expanded
	}

	top := uint32(userStackBottom)
	if created {
		top = env.imageP1Top()
	}

	low := env.p1Low()
	if low >= top {
		return
	}

	space := env.pageSpace()

	for addr := low; addr < top; addr += pageSize {
		fresh := ProcessPTE(true, (addr-p1Base)/pageSize)
		if created {
			fresh = 0
		}

		env.resetPage(space, addr, fresh)
	}

	env.refundPagesIn(low, top)

	env.RegionSize[1] = 0

	if created {
		env.setRegionLength(regionP1, (top-p1Base)/pageSize)
	}
}
