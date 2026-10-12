package console

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
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

	m.endOtherProcesses()

	m.CPU = vax.New()
	m.CPU.SetDebugWriter(m.Hooks.Output())
	m.Mem = vm.NewMemory(size)
	m.Engine = cpu.NewEngine(m.CPU, m.Mem)
	m.newRTL()
	m.Engine.SetSystemServices(m)

	m.CPU.SetGPR(vax.SP, size-4)
	m.CPU.SetGPR(vax.PC, 0)
	m.CPU.SetPR(vax.KSP, size-4)
	m.CPU.SetPR(vax.ESP, size-4)
	m.CPU.SetPR(vax.SSP, size-4)
	m.CPU.SetPR(vax.ISP, size-4)
	m.CPU.SetPR(vax.USP, size-4)

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

	m.endOtherProcesses()

	size := m.Mem.Size()
	m.Mem = vm.NewMemory(size)
	m.Engine = cpu.NewEngine(m.CPU, m.Mem)
	m.newRTL()
	m.Engine.SetSystemServices(m)

	m.Symbols.ClearAll()

	m.CPU.SetPR(vax.MAPEN, 0)
	m.CPU.SetPR(vax.SBR, 0)
	m.CPU.SetPR(vax.SLR, 0)
	m.CPU.SetPR(vax.P0BR, 0)
	m.CPU.SetPR(vax.P0LR, 0)
	m.CPU.SetPR(vax.P1BR, 0)
	m.CPU.SetPR(vax.P1LR, 0)

	sp := size - 4
	m.CPU.SetGPR(vax.SP, sp)
	m.CPU.SetGPR(vax.PC, 0)
	m.CPU.SetGPR(vax.FP, sp)
	m.CPU.SetGPR(vax.AP, sp)
	m.CPU.SetPR(vax.ISP, sp)
	m.CPU.SetPR(vax.KSP, sp)
	m.CPU.SetPR(vax.ESP, sp)
	m.CPU.SetPR(vax.SSP, sp)
	m.CPU.SetPR(vax.USP, sp)

	m.VMInitValid = false
	m.asmSession = nil // matching vminit.go's own reset -- a zeroed address space invalidates any prior ASM session's state
	m.assemblerMode = false
	m.kernelPlaced = false

	return nil
}
