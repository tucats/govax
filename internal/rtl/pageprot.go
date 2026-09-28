package rtl

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Page protection and page locking (docs/PHASE-26.md subtask 36):
// $SETPRT, $LCKPAG, $ULKPAG, $LKWSET, and $ULWSET. They work on ranges of
// pages the way $CRETVA and $DELTVA do (vaspace.go), sharing their range
// handling.
//
// # Page protection
//
// A page's protection, 4 bits of its page table entry, says which access
// modes may read it and which may write it. The codes name the least
// privileged mode allowed each kind of access: UW (user write) lets every
// mode read and write, UR (user read) lets every mode read and none write,
// URKW lets every mode read but only kernel write, NA allows nothing, and
// so on ($PRTDEF, generated as vmsdef.PRTConstants, uses the same
// encoding as the PTE). $SETPRT changes the protection of a range of
// pages: a program can make its code read-only once it's loaded, or guard
// a buffer, catching stray accesses as access violations.
//
// # Page locking
//
// On VMS, a page can be *locked* in the working set ($LKWSET: the pager
// won't take it away from the process) or in memory ($LCKPAG: it won't
// even be swapped out with the process), for code that must not page
// fault (interrupt-level code, device buffers). govax doesn't page: every
// page a program has touched stays in memory. So locking changes nothing
// about how memory behaves; the services check their arguments as VMS
// does and remember which pages are locked, because that's what their
// status reports: SS$_WASSET if any page in the range was already locked
// (by that service's kind of lock), SS$_WASCLR if none was.

// Status values the protection services return.
var (
	ssIvProtect = vmsdef.SSConstants["SS$_IVPROTECT"]
	ssLenVio    = vmsdef.SSConstants["SS$_LENVIO"]
)

// prtReserved is PRT$C_RESERVED, the one 4-bit protection code that
// isn't a protection, and prtKR the kernel-read-only code $SETPRT uses
// for a protection of 0.
var (
	prtReserved = vmsdef.PRTConstants["PRT$C_RESERVED"]
	prtKR       = vmsdef.PRTConstants["PRT$C_KR"]
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

	mode := max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())
	status := uint32(ssNormal)
	previous := vm.Protection(0)

	var done []uint32

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
func (env *Environment) pageForChange(addr uint32, mode vax.AccessMode, beyond uint32) (vm.PTE, uint32) {
	if addr >= s0Base {
		return 0, ssNoPriv
	}

	_, _, pte, err := env.mem.LookupPTE(env.cpu, addr)

	switch {
	case err != nil:
		return 0, beyond
	case !pageExists(pte):
		return 0, ssAccVio
	case vax.AccessMode(pte.Owner()) < mode:
		return 0, ssPagOwnVio
	}

	return pte, ssNormal
}

// pageLocks is one kind of page lock: the pages locked, by address.
type pageLocks map[uint32]bool

// serviceSysLckpag is SYS$LCKPAG: lock pages in memory.
//
//	SYS$LCKPAG inadr ,[retadr] ,[acmode]
func serviceSysLckpag(env *Environment, argv []uint32) (uint32, error) {
	return env.lockPages(argv, &env.Process.memoryLocks, true)
}

// serviceSysUlkpag is SYS$ULKPAG: unlock pages locked in memory.
//
//	SYS$ULKPAG inadr ,[retadr] ,[acmode]
func serviceSysUlkpag(env *Environment, argv []uint32) (uint32, error) {
	return env.lockPages(argv, &env.Process.memoryLocks, false)
}

// serviceSysLkwset is SYS$LKWSET: lock pages in the working set.
//
//	SYS$LKWSET inadr ,[retadr] ,[acmode]
func serviceSysLkwset(env *Environment, argv []uint32) (uint32, error) {
	return env.lockPages(argv, &env.Process.workingSetLocks, true)
}

// serviceSysUlwset is SYS$ULWSET: unlock pages locked in the working set.
//
//	SYS$ULWSET inadr ,[retadr] ,[acmode]
func serviceSysUlwset(env *Environment, argv []uint32) (uint32, error) {
	return env.lockPages(argv, &env.Process.workingSetLocks, false)
}

// lockPages is the body of the four locking services: it locks (or, if
// not lock, unlocks) the pages from inadr's first address to its second
// in locks, on behalf of acmode maximized with the caller's mode, storing
// the range done at retadr as $CRETVA does.
//
// It returns SS$_WASSET if any page done was locked before, SS$_WASCLR if
// none was; SS$_NOPRIV for a system page; SS$_ACCVIO for a page that
// doesn't exist or lies beyond the page table, or if inadr can't be read
// or retadr written; or SS$_PAGOWNVIO for a page a more privileged mode
// owns. The process holds PSWAPM, which $LCKPAG requires.
func (env *Environment) lockPages(argv []uint32, locks *pageLocks, lock bool) (uint32, error) {
	r, ok := env.readRange(optArg(argv, 0), true)
	if !ok {
		return ssAccVio, nil
	}

	if *locks == nil {
		*locks = pageLocks{}
	}

	mode := max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())
	status := uint32(ssWasClr)

	var done []uint32

	for _, addr := range r.pages() {
		if _, st := env.pageForChange(addr, mode, ssAccVio); st != ssNormal {
			status = st

			break
		}

		if (*locks)[addr] {
			status = ssWasSet
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

	return status, nil
}

// forgetPageLocks removes any locks on the page at addr: it's being
// deleted.
func (env *Environment) forgetPageLocks(addr uint32) {
	delete(env.Process.memoryLocks, addr)
	delete(env.Process.workingSetLocks, addr)
}

// cancelPageLocks is image rundown's locking step: VMS unlocks the pages
// an image locked when it exits.
func (env *Environment) cancelPageLocks() {
	env.Process.memoryLocks = nil
	env.Process.workingSetLocks = nil
}

func registerPageProtectionServices(t *ServiceTable) {
	t.Register("SYS$SETPRT", serviceSysSetprt)
	t.Register("SYS$LCKPAG", serviceSysLckpag)
	t.Register("SYS$ULKPAG", serviceSysUlkpag)
	t.Register("SYS$LKWSET", serviceSysLkwset)
	t.Register("SYS$ULWSET", serviceSysUlwset)
}
