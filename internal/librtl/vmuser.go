package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
)

// User-defined zones: LIB$CREATE_USER_VM_ZONE, and the calls of the
// program's routines that LIB$GET_VM, LIB$FREE_VM, LIB$RESET_VM_ZONE, and
// LIB$DELETE_VM_ZONE make for such a zone (5.0 LIB$ manual, section 5.6
// and the routine's entry; docs/PHASE-59 - LIB VM zones.md, subtask 6).
//
// Each routine's status is the heap routine's, so the call is the
// shim's tail call (corevms.CallRequest's Tail), as LIB$SHOW_TIMER calls
// its action routine (the phase doc's Decision 4): the heap routine
// returns when the program's routine does, with its R0.

// libCreateUserVMZone is LIB$CREATE_USER_VM_ZONE:
//
//	LIB$CREATE_USER_VM_ZONE zone-id [,user-argument]
//	    [,user-allocation-procedure] [,user-deallocation-procedure]
//	    [,user-reset-procedure] [,user-delete-procedure] [,zone-name]
//
// It makes a user-defined zone and writes its ID to zone-id. The zone
// keeps a copy of the longword at user-argument (0 when omitted), in its
// control block, and passes its address to each of its routines. The
// four procedures (entry masks' addresses, by value) are called for
// LIB$GET_VM, LIB$FREE_VM, LIB$RESET_VM_ZONE, and LIB$DELETE_VM_ZONE of
// the zone: the first two with number-of-bytes, base-address, and the
// user argument, the others with the user argument alone, all by
// reference. A heap routine whose procedure wasn't given returns
// LIB$_INVOPEZON. zone-name is a string of any class. The statuses are
// SS$_NORMAL, LIB$_INSVIRMEM (no room for the control block), and
// LIB$_INVSTRDES; after an error no zone is made.
func libCreateUserVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	value, err := optionalLongword(env, arg(argv, 1), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	name, _, err := optionalName(env, arg(argv, 6))
	if err != nil {
		return stringFailure(env, err)
	}

	id, err := env.CreateUserZone(corevms.UserZone{
		Argument: value,
		Allocate: arg(argv, 2),
		Free:     arg(argv, 3),
		Reset:    arg(argv, 4),
		Delete:   arg(argv, 5),
		Name:     name,
	})
	if err != nil {
		return vmFailure(env, err)
	}

	if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 0), id); err != nil {
		_ = env.DeleteUserZone(id)

		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// userRoutine calls a user-defined zone's routine at routine with args
// as the shim's tail call, or returns LIB$_INVOPEZON when the zone has
// none.
func userRoutine(env *corevms.Environment, routine uint32, args ...uint32) (uint32, error) {
	if routine == 0 {
		return libInvOpeZon, nil
	}

	return tailCall(env, routine, args...)
}

// deleteUserZone is LIB$DELETE_VM_ZONE of a user-defined zone: the zone
// is forgotten and its delete routine called, its status
// LIB$DELETE_VM_ZONE's. The zone is gone whatever the routine returns
// (unconfirmed). Its control block goes with it, so the routine is given
// the address of a copy of the user argument, below SP with the argument
// list (unconfirmed: VMS's address is a probe case). A zone without a
// delete routine is LIB$_INVOPEZON, and stays.
func deleteUserZone(env *corevms.Environment, zone uint32, u corevms.UserZone, argAddr uint32) (uint32, error) {
	if u.Delete == 0 {
		return libInvOpeZon, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	value, err := mem.LoadLongword(cpu, argAddr)
	if err != nil {
		return memoryFault(env, err)
	}

	copied := cpu.GPR(registers.SP) - 4

	if err := mem.StoreLongword(cpu, copied, value); err != nil {
		return memoryFault(env, writing(err))
	}

	cpu.SetGPR(registers.SP, copied)

	if err := env.DeleteUserZone(zone); err != nil {
		return vmFailure(env, err)
	}

	return tailCall(env, u.Delete, copied)
}
