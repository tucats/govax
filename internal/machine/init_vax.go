package machine

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// Init is INIT/VAX: it ends every other process and makes a new machine
// of physBytes (rounded by allocPhysMemory), a new CPU, memory, engine,
// and VMS (newRTL), with no address space: MAPEN is 0. The ROM and NVRAM
// are LOAD/ROM's and LOAD/NVRAM's. The console's radix, verbosity, and
// verification, the devices, the mounts, and process 1's logical names
// belong to the console and outlive it.
//
// The new CPU's registers and PSL are all zero: kernel mode at IPL 0, not
// on the interrupt stack. That isn't the architecture's processor
// initialization, which sets the PSL to 041F0000 (kernel mode on the
// interrupt stack at IPL 31); the console's GO and CALL of code before
// the microkernel has started depend on IPL 0 and the kernel stack
// (docs/VAX-VMS-INTERNALS.md, section 11.11).
func (m *Machine) Init(physBytes uint32) error {
	size := allocPhysMemory(physBytes)

	m.EndOtherProcesses()

	m.CPU = registers.New()
	m.CPU.SetDebugWriter(m.Hooks.Output())
	m.Mem = vm.NewMemory(size)
	m.Engine = cpu.NewEngine(m.CPU, m.Mem)
	m.paging = false
	m.newRTL()
	m.Engine.SetSystemServices(m)

	m.CPU.SetGPR(registers.SP, size-4)
	m.CPU.SetGPR(registers.PC, 0)
	m.CPU.SetPR(registers.KSP, size-4)
	m.CPU.SetPR(registers.ESP, size-4)
	m.CPU.SetPR(registers.SSP, size-4)
	m.CPU.SetPR(registers.ISP, size-4)
	m.CPU.SetPR(registers.USP, size-4)

	m.DepositAddr = 0x200
	m.VMInitValid = false

	return m.Zero()
}

// Zero clears physical memory and every symbol, and resets the region/stack
// registers that only make sense relative to a zeroed, unmapped address
// space — also run by Init, as console_init itself does.
func (m *Machine) Zero() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if err := m.requireKernelMode(); err != nil {
		return err
	}

	m.EndOtherProcesses()

	size := m.Mem.Size()
	m.Mem = vm.NewMemory(size)
	m.Engine = cpu.NewEngine(m.CPU, m.Mem)
	m.paging = false
	m.newRTL()
	m.Engine.SetSystemServices(m)

	m.Symbols.ClearAll()

	m.CPU.SetPR(registers.MAPEN, 0)
	m.CPU.SetPR(registers.SBR, 0)
	m.CPU.SetPR(registers.SLR, 0)
	m.CPU.SetPR(registers.P0BR, 0)
	m.CPU.SetPR(registers.P0LR, 0)
	m.CPU.SetPR(registers.P1BR, 0)
	m.CPU.SetPR(registers.P1LR, 0)

	sp := size - 4
	m.CPU.SetGPR(registers.SP, sp)
	m.CPU.SetGPR(registers.PC, 0)
	m.CPU.SetGPR(registers.FP, sp)
	m.CPU.SetGPR(registers.AP, sp)
	m.CPU.SetPR(registers.ISP, sp)
	m.CPU.SetPR(registers.KSP, sp)
	m.CPU.SetPR(registers.ESP, sp)
	m.CPU.SetPR(registers.SSP, sp)
	m.CPU.SetPR(registers.USP, sp)

	m.VMInitValid = false
	m.asmSession = nil // matching vminit.go's own reset -- a zeroed address space invalidates any prior ASM session's state
	m.assemblerMode = false
	m.kernelPlaced = false

	return nil
}
