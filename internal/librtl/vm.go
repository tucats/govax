package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Statuses the virtual-memory routines, and others, return.
var (
	ssNormal   = vmsdef.Symbols["SS$_NORMAL"]
	ssAccVio   = vmsdef.Symbols["SS$_ACCVIO"]
	ssInsfMem  = vmsdef.Symbols["SS$_INSFMEM"]
	ssNoSignal = vmsdef.Symbols["SS$_NOSIGNAL"]
)

// statusFreeVMBadBlock is what LIB$FREE_VM returns for an address that
// isn't an allocated block: 4042, as eVAX's librtl has it (it isn't
// LIB$_BADBLOADR, which VMS documents); kept as ported.
const statusFreeVMBadBlock = 4042

// libGetVM is LIB$GET_VM:
//
//	LIB$GET_VM number-of-bytes ,base-address [,zone-id]
//
// It allocates number-of-bytes (a longword by reference) from the image's
// heap and stores the block's address at base-address. Until zones exist
// (Phase 59, subtask 4) every block comes from the one heap, whatever
// zone-id says.
func libGetVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	sizeAddr, retAddr := arg(argv, 0), arg(argv, 1)

	size, err := mem.LoadLongword(cpu, sizeAddr)
	if err != nil {
		return memoryFault(env, err)
	}

	addr, err := env.AllocateVM(size)
	if err != nil {
		return 0, err
	}

	if addr == 0 {
		return ssInsfMem, nil
	}

	if err := mem.StoreLongword(cpu, retAddr, addr); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libFreeVM is LIB$FREE_VM:
//
//	LIB$FREE_VM number-of-bytes ,base-address [,zone-id]
//
// It frees the block whose address is the longword at base-address.
func libFreeVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	addr, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	if !env.FreeVM(addr) {
		return statusFreeVMBadBlock, nil
	}

	return ssNormal, nil
}

// libDeleteVMZone is LIB$DELETE_VM_ZONE:
//
//	LIB$DELETE_VM_ZONE zone-id
//
// It deletes the zone the longword at zone-id names. No zone can be made
// yet (LIB$CREATE_VM_ZONE arrives in Phase 59's subtask 4), and the
// default zone, 0, can't be deleted (5.0 LIB$ manual, section 5.3.3), so
// every zone-id is an invalid one: LIB$_BADBLOADR, the manual's status
// for it.
func libDeleteVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if _, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0)); err != nil {
		return memoryFault(env, err)
	}

	return libBadBlockAddress, nil
}
