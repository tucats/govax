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

var libBadBlockSize = vmsdef.LibrarySymbols["LIB$_BADBLOSIZ"]

// vmStatus is the condition value for one of internal/vmheap's errors:
// LIB$_BADBLOSIZ for a bad size, LIB$_BADBLOADR for a bad address, and
// LIB$_INSVIRMEM when there's no room.
func vmStatus(err error) uint32 {
	switch {
	case errors.Is(err, vmheap.ErrBadSize):
		return libBadBlockSize
	case errors.Is(err, vmheap.ErrBadAddress):
		return libBadBlockAddress
	default:
		return libInsVirMem
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

	count, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if int32(count) <= 0 {
		return libBadBlockSize, nil
	}

	addr, err := env.GetVMPages(count)
	if err != nil {
		return vmStatus(err), nil
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
// back with the zone (unconfirmed: VMS presumably frees them, leaving
// the zone's blocks in free pages).
func libFreeVMPage(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

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
		return vmStatus(err), nil
	}

	return ssNormal, nil
}
