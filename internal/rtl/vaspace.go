package rtl

import (
	"github.com/tucats/govax/internal/vax"
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
// VMINIT builds the P0 and P1 page tables once, at their full size, with
// every page demand-zero, user-owned, and readable and writable by every
// mode (except P0's page 0, which is no-access). internal/vm allocates a
// physical page on first touch. So these services work entirely on PTEs:
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
// table can't be created: SS$_VASFULL.

// Status values the address space services return.
var (
	ssPagOwnVio = vmsdef.SSConstants["SS$_PAGOWNVIO"]
	ssVasFull   = vmsdef.SSConstants["SS$_VASFULL"]
	ssIllPagCnt = vmsdef.SSConstants["SS$_ILLPAGCNT"]
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
// giving back the physical page the old entry had, if it was valid.
func (env *Environment) replacePTE(addr uint32, old, pte vm.PTE) bool {
	if env.mem.StorePTE(env.cpu, addr, pte) != nil {
		return false
	}

	if old.Valid() {
		env.mem.FreePage(old.PFN())
	}

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
// SS$_VASFULL for a page beyond the region's page table; SS$_PAGOWNVIO; or
// SS$_ACCVIO if inadr can't be read or retadr written.
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
	mode := max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())

	var done []uint32

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
func (env *Environment) createPage(addr uint32, mode vax.AccessMode) uint32 {
	if addr >= s0Base {
		return ssNoPriv
	}

	_, _, old, err := env.mem.LookupPTE(env.cpu, addr)
	if err != nil {
		return ssVasFull
	}

	if pageExists(old) && vax.AccessMode(old.Owner()) < mode {
		return ssPagOwnVio
	}

	var pte vm.PTE

	pte.SetProtection(modeProtections[mode])
	pte.SetOwner(uint8(mode))

	if !env.replacePTE(addr, old, pte) {
		return ssVasFull
	}

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

	mode := max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())
	status, done := env.deletePages(r.pages(), mode)

	if !env.storeRetadr(optArg(argv, 1), done) {
		return ssAccVio, nil
	}

	return status, nil
}

// deletePages deletes pages, in order, on behalf of mode, stopping at the
// first that can't be. It returns the status and the pages it did.
func (env *Environment) deletePages(pages []uint32, mode vax.AccessMode) (uint32, []uint32) {
	var done []uint32

	for _, addr := range pages {
		if addr >= s0Base {
			return ssNoPriv, done
		}

		_, _, old, err := env.mem.LookupPTE(env.cpu, addr)
		if err == nil && pageExists(old) {
			if vax.AccessMode(old.Owner()) < mode {
				return ssPagOwnVio, done
			}

			if !env.replacePTE(addr, old, 0) {
				return ssAccVio, done
			}
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
// inwards, and retadr receives the range deleted.
//
// It returns SS$_NORMAL; SS$_ILLPAGCNT for a count of 0 or more pages
// than the region has; SS$_PAGOWNVIO; SS$_BADPARAM for another region; or
// SS$_ACCVIO if retadr can't be written.
func serviceSysCntreg(env *Environment, argv []uint32) (uint32, error) {
	pagcnt, retadr := optArg(argv, 0), optArg(argv, 1)
	mode := max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())

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
		bottom := p1Base + (env.cpu.PR(vax.P1LR)+1)*pageSize
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

	if optArg(argv, 3) == regionP0 && len(done) > 0 {
		env.RegionSize[0] = done[len(done)-1]
	}

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
