package corevms

import (
	"errors"
	"slices"

	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Page protection and page locking (docs/PHASE-26.md subtask 36):
// $SETPRT, $LCKPAG, $ULKPAG, $LKWSET, and $ULWSET; and $PURGWS, from
// Phase 63's subtask 6 ("docs/PHASE-63 - working sets.md"). They work on
// ranges of pages the way $CRETVA and $DELTVA do (vaspace.go), sharing
// their range handling.
//
// # Page protection
//
// A page's protection, 4 bits of its page table entry, says which access
// modes may read it and which may write it. The codes name the least
// privileged mode allowed each kind of access: UW (user write) lets every
// mode read and write, UR (user read) lets every mode read and none write,
// URKW lets every mode read but only kernel write, NA allows nothing, and
// so on ($PRTDEF, generated as vmsdef.Symbols, uses the same
// encoding as the PTE). $SETPRT changes the protection of a range of
// pages: a program can make its code read-only once it's loaded, or guard
// a buffer, catching stray accesses as access violations.
//
// # Page locking and purging
//
// On VMS, a page can be *locked* in the working set ($LKWSET: the pager
// won't take it away from the process) or in memory ($LCKPAG: it won't
// even be swapped out with the process), for code that must not page
// fault (interrupt-level code, device buffers). With paging on
// (workingset.go), a locked page is faulted in and kept out of the
// working set list's replacement; $PURGWS takes the pages of a range out
// of the working set, as a program does to start a phase of its work
// with only the pages it then touches. With paging off, every page a
// program has touched stays in memory, so locking changes nothing about
// how memory behaves and purging does nothing; the services still check
// their arguments as VMS does and remember which pages are locked,
// because that's what their statuses report.

// Status values the protection services return.
var (
	ssIvProtect = vmsdef.Symbols["SS$_IVPROTECT"]
	ssLenVio    = vmsdef.Symbols["SS$_LENVIO"]
)

// prtReserved is PRT$C_RESERVED, the one 4-bit protection code that
// isn't a protection, and prtKR the kernel-read-only code $SETPRT uses
// for a protection of 0.
var (
	prtReserved = vmsdef.Symbols["PRT$C_RESERVED"]
	prtKR       = vmsdef.Symbols["PRT$C_KR"]
)

// serviceSysSetprt is SYS$SETPRT:
//
//	SYS$SETPRT inadr ,[retadr] ,[acmode] ,prot ,[prvprt]
//
// It gives the pages from inadr's first address to its second the
// protection prot (its low four bits; 0 means kernel read-only, as the
// manual says, and 1 is SS$_IVPROTECT). acmode is maximized with the
// caller's mode and must be at least as privileged as each page's owner
// (else SS$_PAGOWNVIO). The protection the last page had before is stored
// at prvprt (a byte) if given, and retadr receives the range changed, as
// for $CRETVA.
//
// It returns SS$_NORMAL; SS$_NOPRIV for a system-region page;
// SS$_LENVIO for a page beyond the region's page table; SS$_ACCVIO for a
// page that doesn't exist, or if inadr can't be read or retadr or prvprt
// written; SS$_PAGOWNVIO; or SS$_IVPROTECT.
func serviceSysSetprt(env *Environment, argv []uint32) (uint32, error) {
	r, ok := env.readRange(optArg(argv, 0), true)
	if !ok {
		return ssAccVio, nil
	}

	prot := optArg(argv, 3) & 0xF
	if prot == prtReserved {
		return ssIvProtect, nil
	}

	if prot == 0 {
		prot = prtKR
	}

	mode := max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())
	status := uint32(ssNormal)
	previous := vm.Protection(0)

	done := make([]uint32, 0, len(r.pages()))

	for _, addr := range r.pages() {
		old, st := env.pageForChange(addr, mode, ssLenVio)
		if st != ssNormal {
			status = st

			break
		}

		pte := old
		pte.SetProtection(vm.Protection(prot))

		if env.mem.StorePTE(env.cpu, addr, pte) != nil {
			status = ssLenVio

			break
		}

		previous = old.Protection()

		done = append(done, addr)
	}

	if !env.storeRetadr(optArg(argv, 1), done) {
		return ssAccVio, nil
	}

	if prvprt := optArg(argv, 4); prvprt != 0 && len(done) > 0 {
		if env.mem.StoreByte(env.cpu, prvprt, byte(previous)) != nil {
			return ssAccVio, nil
		}
	}

	return status, nil
}

// pageForChange looks up the page at addr for a service changing it on
// behalf of mode, returning its PTE, or the status that stops the
// service: SS$_NOPRIV for a system page, beyond for a page beyond the
// page table, SS$_ACCVIO for a page that doesn't exist, SS$_PAGOWNVIO for
// a page a more privileged mode owns.
func (env *Environment) pageForChange(addr uint32, mode registers.AccessMode, beyond uint32) (vm.PTE, uint32) {
	if addr >= s0Base {
		return 0, ssNoPriv
	}

	_, _, pte, err := env.mem.LookupPTE(env.cpu, addr)

	switch {
	case err != nil:
		return 0, beyond
	case !pageExists(pte):
		return 0, ssAccVio
	case registers.AccessMode(pte.Owner()) < mode:
		return 0, ssPagOwnVio
	}

	return pte, ssNormal
}

// pageLocks is one kind of page lock: the pages locked, by address.
type pageLocks map[uint32]bool

// ssLkwsetFul is SS$_LKWSETFUL: locking another page in the working set
// would leave too little of it to page in.
var ssLkwsetFul = vmsdef.Symbols["SS$_LKWSETFUL"]

// serviceSysLckpag is SYS$LCKPAG: lock pages in memory.
//
//	SYS$LCKPAG inadr ,[retadr] ,[acmode]
//
// Locking pages in memory needs the PSWAPM privilege (SS$_NOPRIV).
func serviceSysLckpag(env *Environment, argv []uint32) (uint32, error) {
	if !env.Process.hasPrivilege(privPSWAPM) {
		return ssNoPriv, nil
	}

	return env.lockPages(argv, paging.LockedInMemory, true)
}

// serviceSysUlkpag is SYS$ULKPAG: unlock pages locked in memory.
//
//	SYS$ULKPAG inadr ,[retadr] ,[acmode]
func serviceSysUlkpag(env *Environment, argv []uint32) (uint32, error) {
	return env.lockPages(argv, paging.LockedInMemory, false)
}

// serviceSysLkwset is SYS$LKWSET: lock pages in the working set.
//
//	SYS$LKWSET inadr ,[retadr] ,[acmode]
func serviceSysLkwset(env *Environment, argv []uint32) (uint32, error) {
	return env.lockPages(argv, paging.LockedInWorkingSet, true)
}

// serviceSysUlwset is SYS$ULWSET: unlock pages locked in the working set.
//
//	SYS$ULWSET inadr ,[retadr] ,[acmode]
func serviceSysUlwset(env *Environment, argv []uint32) (uint32, error) {
	return env.lockPages(argv, paging.LockedInWorkingSet, false)
}

// pageLockMap is the process's record of one kind of lock.
func (env *Environment) pageLockMap(kind paging.Locks) *pageLocks {
	if kind == paging.LockedInMemory {
		return &env.Process.memoryLocks
	}

	return &env.Process.workingSetLocks
}

// lockPages is the body of the four locking services: it locks (or, if
// not lock, unlocks) the pages from inadr's first address to its second
// with one kind of lock, on behalf of acmode maximized with the caller's
// mode, storing the range done at retadr as $CRETVA does.
//
// The process's maps record the locks in both modes, and decide the
// statuses. With paging on, the working set list holds them too: a page
// is faulted into the working set before it's locked (lockInWorkingSet),
// and a locked page leaves the list's dynamic part, so it's never
// replaced.
//
// The statuses run opposite ways for locking and unlocking, as both
// manuals give them (the phase doc's Part A). Locking returns SS$_WASSET
// if at least one page done was locked before, SS$_WASCLR if every page
// was unlocked; unlocking returns SS$_WASSET if every page done was
// locked, SS$_WASCLR if at least one wasn't. The errors are SS$_NOPRIV
// for a system page or, locking in the working set, a writable global
// page (writableGlobalPage); SS$_ACCVIO for a page that doesn't exist,
// lies beyond the page table, or can't be faulted in, or if inadr can't
// be read or retadr written; SS$_PAGOWNVIO for a page a more privileged
// mode owns; and SS$_LKWSETFUL for a working set too full of locked
// pages. An error stops the service at its page, and retadr receives the
// pages done before it; a retadr that can't be written stops it before
// the first.
func (env *Environment) lockPages(argv []uint32, kind paging.Locks, lock bool) (uint32, error) {
	r, ok := env.readRange(optArg(argv, 0), true)
	if !ok {
		return ssAccVio, nil
	}

	// A retadr that can't be written stops the service before any page
	// changes: VMS 7.1's $LKWSET locked nothing then (testdata/pg63,
	// round 2, E14; the other three services by the same rule,
	// unconfirmed).
	if retadr := optArg(argv, 1); retadr != 0 && (!env.canWriteLongword(retadr) || !env.canWriteLongword(retadr+4)) {
		return ssAccVio, nil
	}

	locks := env.pageLockMap(kind)
	if *locks == nil {
		*locks = pageLocks{}
	}

	mode := max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())
	status := uint32(ssNormal)
	anySet, anyClear := false, false

	done := make([]uint32, 0, len(r.pages()))

	for _, addr := range r.pages() {
		pte, st := env.pageForChange(addr, mode, ssAccVio)

		switch {
		case st != ssNormal:
		case !lock:
			env.unlockInWorkingSet(addr, kind)
		case kind == paging.LockedInWorkingSet && env.writableGlobalPage(addr, pte):
			st = ssNoPriv
		default:
			st = env.lockInWorkingSet(addr, kind)
		}

		if st != ssNormal {
			status = st

			break
		}

		if (*locks)[addr] {
			anySet = true
		} else {
			anyClear = true
		}

		if lock {
			(*locks)[addr] = true
		} else {
			delete(*locks, addr)
		}

		done = append(done, addr)
	}

	if !env.storeRetadr(optArg(argv, 1), done) {
		return ssAccVio, nil
	}

	switch {
	case status != ssNormal:
		return status, nil
	case lock && anySet, !lock && !anyClear:
		return ssWasSet, nil
	default:
		return ssWasClr, nil
	}
}

// lockInWorkingSet does a lock's work on the page at addr beyond the
// process's record of it, and returns SS$_NORMAL or the status that
// stops the service.
//
// With paging on, the page is faulted into the working set first, if it
// isn't there (SS$_ACCVIO if it can't be), and locked in the list
// (paging.WorkingSet.Lock), which fails with SS$_LKWSETFUL if too few of
// the list's entries would be left unlocked. A valid page outside every
// working set (the P1 vector's, a system page in a process's space) has
// nothing to pin and is only recorded.
//
// With paging off, nothing pages, so only the SS$_LKWSETFUL rule
// applies, counted from the process's records: a lock in the working set
// of a page not yet locked fails if it would leave fewer than MINWSCNT
// of the working set limit's pages unlocked (the phase doc's finding 6;
// unconfirmed, the probe's question). A lock in memory has no such
// limit.
func (env *Environment) lockInWorkingSet(addr uint32, kind paging.Locks) uint32 {
	ws := env.WorkingSet()
	if ws == nil {
		p := env.Process
		if kind == paging.LockedInWorkingSet && !p.memoryLocks[addr] && !p.workingSetLocks[addr] &&
			int(p.WSLimit)-(env.lockedPageCount()+1) < int(p.MinWSCount) {
			return ssLkwsetFul
		}

		return ssNormal
	}

	if _, ok := ws.Lookup(addr); !ok {
		if _, err := env.mem.TranslateIn(env.cpu, env.pageSpace(), addr, vm.AccessRead); err != nil {
			return ssAccVio
		}
	}

	if _, err := ws.Lock(addr, kind); errors.Is(err, paging.ErrLockedSetFull) {
		return ssLkwsetFul
	}

	return ssNormal
}

// unlockInWorkingSet takes a lock of one kind off the page at addr in the
// working set list, with paging on, if the list holds the page. A page
// left with no lock goes back to the list's dynamic part.
func (env *Environment) unlockInWorkingSet(addr uint32, kind paging.Locks) {
	if ws := env.WorkingSet(); ws != nil {
		if _, ok := ws.Lookup(addr); ok {
			_, _ = ws.Unlock(addr, kind)
		}
	}
}

// lockedPageCount is how many of the process's pages are locked, in the
// working set or in memory (a page locked both ways counts once).
func (env *Environment) lockedPageCount() int {
	p := env.Process
	n := len(p.workingSetLocks)

	for addr := range p.memoryLocks {
		if !p.workingSetLocks[addr] {
			n++
		}
	}

	return n
}

// writableGlobalPage reports whether the page at addr, whose PTE is pte,
// is a page of a writable global section, which $LKWSET refuses
// (SS$_NOPRIV): the swapper drops such pages from a process's working set
// when it's outswapped (the book's 16.4.2.1). It's the section that
// counts, not the mapping: VMS 7.1 refused a page of a writable section
// mapped read-only too (testdata/pg63, round 2, E11). A copy-on-reference
// section's page, once touched, is the process's own copy, not the
// section's.
func (env *Environment) writableGlobalPage(addr uint32, pte vm.PTE) bool {
	if !env.sectionPages[addr] {
		return false
	}

	if gi, ok := pte.GlobalIndex(); ok {
		s, _, ok := env.Sections.globalPage(gi)

		return ok && s.Writable && !s.CopyOnRef
	}

	if !pte.Valid() {
		return false
	}

	for _, s := range env.Sections.Sections() {
		if s.Writable && !s.CopyOnRef && slices.Contains(s.Frames, pte.PFN()) {
			return true
		}
	}

	return false
}

// forgetPageLocks removes any locks on the page at addr: it's being
// deleted.
func (env *Environment) forgetPageLocks(addr uint32) {
	delete(env.Process.memoryLocks, addr)
	delete(env.Process.workingSetLocks, addr)
}

// cancelPageLocks is image rundown's locking step: VMS unlocks the pages
// an image locked when it exits. With paging on, the pages go back to
// the working set list's dynamic part, so they can be replaced again (and
// the list shrunk, resetWorkingSet).
func (env *Environment) cancelPageLocks() {
	env.Process.memoryLocks = nil
	env.Process.workingSetLocks = nil

	ws := env.WorkingSet()
	if ws == nil {
		return
	}

	for _, p := range ws.Pages() {
		for _, kind := range []paging.Locks{paging.LockedInWorkingSet, paging.LockedInMemory} {
			if p.Locks&kind != 0 {
				_, _ = ws.Unlock(p.VA, kind)
			}
		}
	}
}

// serviceSysPurgws is SYS$PURGWS: purge the working set.
//
//	SYS$PURGWS inadr
//
// It takes every page from inadr's first address to its second out of
// the process's working set, as the pager takes a page out to make room:
// into transition, the frame to the free or modified list, so a touch
// brings it back with a soft fault. A range of 0 to 7FFFFFFF purges all
// of P0 and P1. The service walks the working set, not the range (the
// phase doc's finding 4). Pages locked in the working set or in memory
// stay (neither manual says; unconfirmed), and so do the pages
// replacement passes over (wsProtected): page-file section pages,
// resident until Phase 65's subtask 4, and the pages the calling
// instruction has translated (its code, its argument list, inadr), which
// the image would fault back at once anyway. A global file section's
// page purged gets its global page table index back in the PTE
// (globalpages.go).
//
// It returns SS$_NORMAL; SS$_ACCVIO if inadr can't be read; SS$_NOPRIV
// if the range reaches system space (VSI's list; the 5.0 manual's has no
// such status; unconfirmed). With paging off there's no working set to
// purge, and it checks its argument only.
func serviceSysPurgws(env *Environment, argv []uint32) (uint32, error) {
	r, ok := env.readRange(optArg(argv, 0), true)
	if !ok {
		return ssAccVio, nil
	}

	lo, hi := min(r.first, r.last), max(r.first, r.last)
	if hi >= s0Base {
		return ssNoPriv, nil
	}

	ws := env.WorkingSet()
	if ws == nil {
		return ssNormal, nil
	}

	for _, p := range ws.RemoveRange(lo, hi, true, env.replacementConditions()) {
		env.evictPage(p)
	}

	return ssNormal, nil
}

func registerPageProtectionServices(t *ServiceTable) {
	t.Register("SYS$SETPRT", serviceSysSetprt)
	t.Register("SYS$LCKPAG", serviceSysLckpag)
	t.Register("SYS$ULKPAG", serviceSysUlkpag)
	t.Register("SYS$LKWSET", serviceSysLkwset)
	t.Register("SYS$ULWSET", serviceSysUlwset)
	t.Register("SYS$PURGWS", serviceSysPurgws)
}
