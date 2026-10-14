package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The heap routines, LIB$GET_VM and LIB$FREE_VM, and the zone routines
// that empty and remove a zone, LIB$RESET_VM_ZONE and LIB$DELETE_VM_ZONE
// (5.0 LIB$ manual, chapter 5). Blocks come from the zone a zone ID names
// (corevms's ZoneAllocate and ZoneFree; LIB$CREATE_VM_ZONE is vmzone.go),
// or from the default zone, the image's heap, for zone ID 0 or none.

// Statuses the virtual-memory routines, and others, return.
var (
	ssNormal   = vmsdef.Symbols["SS$_NORMAL"]
	ssAccVio   = vmsdef.Symbols["SS$_ACCVIO"]
	ssNoSignal = vmsdef.Symbols["SS$_NOSIGNAL"]
)

// libGetVM is LIB$GET_VM:
//
//	LIB$GET_VM number-of-bytes ,base-address [,zone-id]
//
// It allocates number-of-bytes (a signed longword by reference) from the
// zone whose ID is the longword at zone-id (the default zone when it's 0
// or omitted) and stores the block's address at base-address. The block
// is rounded up to the zone's block size and aligned by its alignment,
// and filled if the zone says so. A size of 0 or less, or more than a
// fixed-size zone's blocks, is LIB$_BADBLOSIZ; an invalid zone ID
// LIB$_BADBLOADR; no room LIB$_INSVIRMEM, or LIB$_PAGLIMEXC past the
// zone's page limit. After an error, nothing is allocated and
// base-address is left as it was.
func libGetVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	size, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	zone, err := optionalLongword(env, arg(argv, 2), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	if int32(size) <= 0 {
		return libBadBlockSize, nil
	}

	addr, err := env.ZoneAllocate(zone, size)
	if err != nil {
		return vmFailure(env, err)
	}

	if err := mem.StoreLongword(cpu, arg(argv, 1), addr); err != nil {
		// The caller can't learn where the block is, so it goes back.
		_ = env.ZoneFree(zone, addr, 0, false)

		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libFreeVM is LIB$FREE_VM:
//
//	LIB$FREE_VM [number-of-bytes] ,base-address [,zone-id]
//
// It frees the block whose address is the longword at base-address, in
// the zone whose ID is the longword at zone-id (the default zone when
// it's 0 or omitted), filling it if the zone says so. number-of-bytes is
// the block's size as allocated, which may be omitted (the manual allows
// that only in a zone with boundary tags; govax allows it in any,
// knowing every block's size). Its statuses: LIB$_BADBLOSIZ for a size
// of 0 or less, or one that doesn't round to the block's (unconfirmed:
// VMS presumably trusts it, freeing less or more), or more than a
// fixed-size zone's; LIB$_BADBLOADR for an invalid zone ID or an
// address that isn't one of the zone's blocks (inside one, freed
// already, or another zone's); LIB$_BADTAGVAL for a block whose tags the
// program changed (the VSI manual's status; unconfirmed for VMS 7.3).
// A block freed again while it waits on a lookaside list is SS$_NORMAL,
// as the manual's chapter 6 says, but leaves the zone as it was.
func libFreeVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	var size uint32

	sized := arg(argv, 0) != 0
	if sized {
		var err error
		if size, err = mem.LoadLongword(cpu, arg(argv, 0)); err != nil {
			return memoryFault(env, err)
		}
	}

	addr, err := mem.LoadLongword(cpu, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	zone, err := optionalLongword(env, arg(argv, 2), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	if sized && int32(size) <= 0 {
		return libBadBlockSize, nil
	}

	if err := env.ZoneFree(zone, addr, size, sized); err != nil {
		return vmFailure(env, err)
	}

	return ssNormal, nil
}

// libDeleteVMZone is LIB$DELETE_VM_ZONE:
//
//	LIB$DELETE_VM_ZONE zone-id
//
// It deletes the zone whose ID is the longword at zone-id: its blocks
// are freed (and filled if the zone says so) and its pages go back to
// the processwide page pool. An ID that names no zone, and the default
// zone's, 0, are LIB$_BADBLOADR (the manual's status for an invalid ID;
// for zone 0 it says only "an error status", section 5.3.3).
func libDeleteVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	return zoneOperation(env, argv, env.DeleteZone)
}

// libResetVMZone is LIB$RESET_VM_ZONE:
//
//	LIB$RESET_VM_ZONE zone-id
//
// It frees every block of the zone whose ID is the longword at zone-id
// (filling them if the zone says so), keeping its pages: "the memory is
// not returned to the processwide page pool". Its statuses are
// LIB$DELETE_VM_ZONE's.
func libResetVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	return zoneOperation(env, argv, env.ResetZone)
}

// zoneOperation reads the zone ID at argv's first argument and applies
// op to it.
func zoneOperation(env *corevms.Environment, argv []uint32, op func(uint32) error) (uint32, error) {
	zone, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if err := op(zone); err != nil {
		return vmFailure(env, err)
	}

	return ssNormal, nil
}
