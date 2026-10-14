package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmheap"
	"github.com/tucats/govax/internal/vmsdef"
)

// The page routines, LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE: the middle of
// the 5.0 LIB$ manual's three levels of memory allocation (section 5.2).
// They manage the image's processwide pool of 512-byte pages, which
// takes its pages from the end of P0 as $EXPREG does when none of its
// free ones fit, and which the heap's zones take their areas from too
// (internal/vmheap's PagePool, corevms's GetVMPages and FreeVMPages).

var (
	libBadBlockSize = vmsdef.LibrarySymbols["LIB$_BADBLOSIZ"]
	libPageLimit    = vmsdef.LibrarySymbols["LIB$_PAGLIMEXC"]
	libBadTagValue  = vmsdef.LibrarySymbols["LIB$_BADTAGVAL"]
	libInvOpeZon    = vmsdef.LibrarySymbols["LIB$_INVOPEZON"]
)

// vmFailure is the status for an error from the page pool, a zone, or
// corevms's zone routines: LIB$_BADBLOSIZ for a bad size, LIB$_BADBLOADR
// for a bad address or zone ID (or pages a zone's get-page routine gave
// that it can't use, by govax's choice: unconfirmed), LIB$_INSVIRMEM when
// there's no room, LIB$_PAGLIMEXC past a zone's page limit,
// LIB$_BADTAGVAL for a changed boundary tag, LIB$_INVARG for a zone's bad
// attributes, and a zone's page routine's own failure status. Anything
// else is a memory error (a fill or tag that couldn't be written), a
// signaled access violation.
func vmFailure(env *corevms.Environment, err error) (uint32, error) {
	var failed *corevms.PageRoutineFailure

	switch {
	case errors.As(err, &failed):
		return failed.Status, nil
	case errors.Is(err, vmheap.ErrBadPages), errors.Is(err, corevms.ErrUserZone):
		return libBadBlockAddress, nil
	case errors.Is(err, vmheap.ErrBadSize):
		return libBadBlockSize, nil
	case errors.Is(err, vmheap.ErrBadAddress), errors.Is(err, corevms.ErrBadZone):
		return libBadBlockAddress, nil
	case errors.Is(err, vmheap.ErrNoMemory):
		return libInsVirMem, nil
	case errors.Is(err, vmheap.ErrPageLimit):
		return libPageLimit, nil
	case errors.Is(err, corevms.ErrBadTag):
		return libBadTagValue, nil
	case errors.Is(err, vmheap.ErrBadAttributes):
		return libInvArg, nil
	default:
		return memoryFault(env, err)
	}
}

// libGetVMPage is LIB$GET_VM_PAGE:
//
//	LIB$GET_VM_PAGE number-of-pages ,base-address
//
// It allocates number-of-pages contiguous pages (a signed longword by
// reference) and stores the first one's address at base-address: the
// lowest free pages in the pool that fit, or new ones at P0's end. A
// count of 0 or less is LIB$_BADBLOSIZ, and LIB$_INSVIRMEM means P0
// can't hold them (nothing is allocated then, and base-address is left
// as it was).
func libGetVMPage(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	// Too few arguments, as VMS 7.1 answers them (testdata/lib59).
	if len(argv) == 0 {
		return libBadBlockSize, nil
	}

	count, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if int32(count) <= 0 {
		return libBadBlockSize, nil
	}

	addr, err := env.GetVMPages(count)
	if err != nil {
		return vmFailure(env, err)
	}

	if err := mem.StoreLongword(cpu, arg(argv, 1), addr); err != nil {
		// The caller can't learn where the pages are, so they go back.
		_ = env.FreeVMPages(count, addr)

		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libFreeVMPage is LIB$FREE_VM_PAGE:
//
//	LIB$FREE_VM_PAGE number-of-pages ,base-address
//
// It returns number-of-pages pages, starting at the address in the
// longword at base-address, to the pool. They may be part of a group
// LIB$GET_VM_PAGE gave, or span groups from several calls that lie next
// to each other, as the manual allows. A count of 0 or less is
// LIB$_BADBLOSIZ; an address not on a page boundary, or any page not
// allocated (never, or freed already), is LIB$_BADBLOADR, and then
// nothing is freed. A zone's pages are refused the same way: they go
// back with the zone. VMS 7.1 frees them, the zone's blocks left in free
// pages the pool may hand out again (testdata/lib59); govax keeps its
// heap sound instead (an expected difference of TestLib59Oracle).
func libFreeVMPage(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	if len(argv) < 2 {
		return libBadBlockAddress, nil
	}

	count, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	addr, err := mem.LoadLongword(cpu, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	if int32(count) <= 0 {
		return libBadBlockSize, nil
	}

	if err := env.FreeVMPages(count, addr); err != nil {
		return vmFailure(env, err)
	}

	return ssNormal, nil
}
