package console

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/rtl"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// Init allocates a fresh CPU/Memory/Engine of physBytes (rounded per
// allocPhysMemory), matching console_init.c/initialization.c's
// alloc_vax — minus ROM/NVRAM allocation (INIT/ROM, INIT/NVRAM; deferred to
// Phase 09, see doc.go) and everything alloc_vax sets up that belongs to a
// later phase (assembler flags, debug/quantum settings with no consumer
// yet). Radix/Verbose/Verify are preserved across a re-INIT, matching
// console_init's own save/restore of vax.console.radix/disasm/verify around
// its free_vax+alloc_vax call.
func (c *Console) Init(physBytes uint32) error {
	size := allocPhysMemory(physBytes)

	c.CPU = vax.New()
	c.CPU.SetDebugWriter(c.Out)
	c.Mem = vm.NewMemory(size)
	c.Engine = cpu.NewEngine(c.CPU, c.Mem)
	c.RTL = rtl.NewEnvironment(c.CPU, c.Mem, c.Devices, c.Logicals, c.In, c.Out)
	c.Engine.SetSystemServices(c)

	c.CPU.SetGPR(vax.SP, size-4)
	c.CPU.SetGPR(vax.PC, 0)
	c.CPU.SetPR(vax.KSP, size-4)
	c.CPU.SetPR(vax.ESP, size-4)
	c.CPU.SetPR(vax.SSP, size-4)
	c.CPU.SetPR(vax.ISP, size-4)
	c.CPU.SetPR(vax.USP, size-4)

	c.DepositAddr = 0x200
	c.VMInitValid = false

	return c.Zero()
}

// Zero clears physical memory and every symbol, and resets the region/stack
// registers that only make sense relative to a zeroed, unmapped address
// space — matching console_zero.c's ZERO command (also run by Init, as
// console_init itself does).
func (c *Console) Zero() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	
	if err := c.requireKernelMode(); err != nil {
		return err
	}

	size := c.Mem.Size()
	c.Mem = vm.NewMemory(size)
	c.Engine = cpu.NewEngine(c.CPU, c.Mem)
	c.RTL = rtl.NewEnvironment(c.CPU, c.Mem, c.Devices, c.Logicals, c.In, c.Out)
	c.Engine.SetSystemServices(c)

	c.Symbols.ClearAll()

	c.CPU.SetPR(vax.MAPEN, 0)
	c.CPU.SetPR(vax.SBR, 0)
	c.CPU.SetPR(vax.SLR, 0)
	c.CPU.SetPR(vax.P0BR, 0)
	c.CPU.SetPR(vax.P0LR, 0)
	c.CPU.SetPR(vax.P1BR, 0)
	c.CPU.SetPR(vax.P1LR, 0)

	sp := size - 4
	c.CPU.SetGPR(vax.SP, sp)
	c.CPU.SetGPR(vax.PC, 0)
	c.CPU.SetGPR(vax.FP, sp)
	c.CPU.SetGPR(vax.AP, sp)
	c.CPU.SetPR(vax.ISP, sp)
	c.CPU.SetPR(vax.KSP, sp)
	c.CPU.SetPR(vax.ESP, sp)
	c.CPU.SetPR(vax.SSP, sp)
	c.CPU.SetPR(vax.USP, sp)

	c.VMInitValid = false
	c.asmSession = nil // matching vminit.go's own reset -- a zeroed address space invalidates any prior ASM session's state
	c.assemblerMode = false

	return nil
}
