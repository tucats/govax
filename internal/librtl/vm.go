package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Statuses the virtual-memory routines return.
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

// libGetVM is LIB$GET_VM (ported from eVAX):
//
//	LIB$GET_VM number-of-bytes ,base-address [,zone-id]
//
// It allocates number-of-bytes (a longword by reference) from the process
// heap and stores the block's address at base-address. As in eVAX, the
// block gets no zone (0), whatever zone-id says: eVAX's lib_get_vm passed
// the zone where its allocator never read it.
func libGetVM(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	sizeAddr, retAddr := arg(argv, 0), arg(argv, 1)

	size, err := mem.LoadLongword(cpu, sizeAddr)
	if err != nil {
		return ssAccVio, nil
	}

	addr, err := env.AllocateVM(size, 0)
	if err != nil {
		return 0, err
	}

	if addr == 0 {
		return ssInsfMem, nil
	}

	if err := mem.StoreLongword(cpu, retAddr, addr); err != nil {
		return ssAccVio, nil
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
		return ssAccVio, nil
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
// It frees every block tagged with the zone the longword at zone-id names.
func libDeleteVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	zone, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return ssAccVio, nil
	}

	env.FreeVMZone(zone)

	return ssNormal, nil
}
