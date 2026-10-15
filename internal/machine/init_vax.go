package machine

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// Init allocates a fresh CPU/Memory/Engine of physBytes (rounded per
// allocPhysMemory), minus ROM/NVRAM allocation (INIT/ROM, INIT/NVRAM; deferred to
// Phase 09, see doc.go) and everything alloc_vax sets up that belongs to a
// later phase (assembler flags, debug/quantum settings with no consumer
// yet). Radix/Verbose/Verify are preserved across a re-INIT, matching
// console_init's own save/restore of vax.console.radix/disasm/verify around
// its free_vax+alloc_vax call.
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
