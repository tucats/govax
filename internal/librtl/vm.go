package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The heap routines, LIB$GET_VM and LIB$FREE_VM, and the zone routines
// that empty and remove a zone, LIB$RESET_VM_ZONE and LIB$DELETE_VM_ZONE
// (5.0 LIB$ manual, chapter 5). Blocks come from the zone a zone ID names
// (corevms's ZoneAllocate and ZoneFree; LIB$CREATE_VM_ZONE is vmzone.go),
// or from the default zone, the image's heap, for zone ID 0 or none. For
// a user-defined zone each calls the program's routine instead
// (vmuser.go). corevms counts their calls for LIB$STAT_VM and
// LIB$SHOW_VM (vmstat.go). What VMS 7.1 answers for a call with too few
// arguments is testdata/lib59's: LIB$GET_VM's with none LIB$_BADBLOSIZ,
// LIB$FREE_VM's without a base-address and LIB$DELETE_VM_ZONE's and
// LIB$RESET_VM_ZONE's with none LIB$_BADBLOADR.

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
// zone's page limit (or, as on VMS 7.1, when the zone's get-page routine
// fails). After an error, nothing is allocated and base-address is left
// as it was.
func libGetVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	if len(argv) == 0 {
		return libBadBlockSize, nil
	}

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

	if u, argAddr, ok := env.UserZoneOf(zone); ok {
		if u.Allocate != 0 {
			env.CountVMCall(true)
		}

		return userRoutine(env, u.Allocate, arg(argv, 0), arg(argv, 1), argAddr)
	}

	// VMS 7.1 reads an unknown zone ID's control block, 8 bytes in: an
	// ID that isn't mapped memory is an access violation there, not
	// LIB$_BADBLOADR (testdata/lib59). Zone 0 is always the default zone,
	// even when there was no room to make it (ZoneAllocate's
	// LIB$_INSVIRMEM).
	if _, known := env.ZoneTagged(zone); !known && zone != 0 {
		if _, err := mem.LoadLongword(cpu, zone+8); err != nil {
			return memoryFault(env, err)
		}
	}

	// A zone with page routines of the program's may call its get-page
	// routine for a new area: that needs a goroutine of the routine's own.
	if env.ZoneCallsBack(zone) && !env.CallingBack() {
		return env.RunCallingBack("LIB$GET_VM", libGetVM, argv)
	}

	addr, err := env.ZoneAllocate(zone, size)

	var failed *corevms.PageRoutineFailure
	if errors.As(err, &failed) {
		return libInsVirMem, nil
	}

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
// the block's size, which may be omitted only in a zone with boundary
// tags: elsewhere VMS 7.1 reads it anyway, an access violation
// (testdata/lib59). An untagged zone trusts it, freeing as much as it
// says (corevms.ZoneFree). Its statuses: LIB$_BADBLOSIZ for a size of 0
// or less, or more than a fixed-size zone's; LIB$_BADBLOADR for an
// invalid zone ID or bytes that aren't the zone's blocks (misaligned,
// outside its areas, freed already); LIB$_BADTAGVAL for a tagged block
// whose first tag doesn't say it's allocated (freed already). A block
// freed again while it waits on a lookaside list is SS$_NORMAL, as the
// manual's chapter 6 says, and damages the list as on VMS.
//
// For a user-defined zone, LIB$GET_VM and LIB$FREE_VM call the zone's
// allocation or deallocation routine with number-of-bytes and
// base-address as they were given and the zone's user argument, and
// return its status; a size of 0 or less is LIB$_BADBLOSIZ first, as on
// VMS 7.1.
func libFreeVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	if len(argv) < 2 {
		return libBadBlockAddress, nil
	}

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

	// Without a size, only a tagged zone's block can be freed: VMS 7.1
	// reads the size's longword at 0 for any other, an access violation
	// (page 0 is no-access).
	_, _, user := env.UserZoneOf(zone)
	if tagged, known := env.ZoneTagged(zone); !sized && (user || known && !tagged) {
		v, err := mem.LoadLongword(cpu, 0)
		if err != nil {
			return memoryFault(env, err)
		}

		size, sized = v, true

		if int32(size) <= 0 {
			return libBadBlockSize, nil
		}
	}

	if u, argAddr, ok := env.UserZoneOf(zone); ok {
		if u.Free != 0 {
			env.CountVMCall(false)
		}

		return userRoutine(env, u.Free, arg(argv, 0), arg(argv, 1), argAddr)
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
// the processwide page pool. An ID that names no zone, 0, and the RTL's
// zones' are LIB$_BADBLOADR, as on VMS 7.1 (testdata/lib59). A zone with
// a free-page routine of the program's calls it for each area, then for
// its control block's page if a get-page routine gave it; a failure
// status it returns is LIB$DELETE_VM_ZONE's, the zone deleted all the
// same. A user-defined zone is forgotten and its delete routine called
// (deleteUserZone).
func libDeleteVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 {
		return libBadBlockAddress, nil
	}

	zone, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if u, argAddr, ok := env.UserZoneOf(zone); ok {
		return deleteUserZone(env, zone, u, argAddr)
	}

	if env.ZoneCallsBack(zone) && !env.CallingBack() {
		return env.RunCallingBack("LIB$DELETE_VM_ZONE", libDeleteVMZone, argv)
	}

	if err := env.DeleteZone(zone); err != nil {
		return vmFailure(env, err)
	}

	return ssNormal, nil
}

// libResetVMZone is LIB$RESET_VM_ZONE:
//
//	LIB$RESET_VM_ZONE zone-id
//
// It frees every block of the zone whose ID is the longword at zone-id
// (filling them if the zone says so), keeping its pages: "the memory is
// not returned to the processwide page pool". Its statuses are
// LIB$DELETE_VM_ZONE's. For a user-defined zone it calls the zone's
// reset routine with the zone's user argument and returns its status.
func libResetVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 {
		return libBadBlockAddress, nil
	}

	zone, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if u, argAddr, ok := env.UserZoneOf(zone); ok {
		return userRoutine(env, u.Reset, argAddr)
	}

	if err := env.ResetZone(zone); err != nil {
		return vmFailure(env, err)
	}

	return ssNormal, nil
}
