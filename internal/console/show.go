package console

import "github.com/tucats/govax/internal/vax"

// This file implements the subset of console_show.c's dozens of SHOW
// sub-displays this port covers: registers, PSL, memory size, symbols,
// breakpoints, radix, region base registers, the four privileged-mode
// stack pointers, and a version banner. Device-dependent (SHOW DEVICE,
// SHOW NVRAM/ROM's contents), assembler-dependent (SHOW INSTRUCTIONS, SHOW
// TRACE), and RTL/microkernel-dependent (SHOW IMAGES, SHOW CALL_FRAMES,
// SHOW SCB) sub-displays are deferred — see doc.go and
// docs/PHASE-08.md's progress log.

// ShowRegisters prints R0-R15 (with the AP/FP/SP/PC aliases), matching
// SHOW REGISTERS.
func (c *Console) ShowRegisters() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	for r := vax.R0; r <= vax.R11; r++ {
		c.Printf("R%-3d = %08X\n", int(r), c.CPU.GPR(r))
	}
	c.Printf("AP   = %08X\n", c.CPU.GPR(vax.AP))
	c.Printf("FP   = %08X\n", c.CPU.GPR(vax.FP))
	c.Printf("SP   = %08X\n", c.CPU.GPR(vax.SP))
	c.Printf("PC   = %08X\n", c.CPU.GPR(vax.PC))
	return nil
}

// ShowPSL prints the processor status longword and its named fields,
// matching SHOW PSL.
func (c *Console) ShowPSL() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	p := c.CPU.PSL()
	c.Printf("PSL = %08X\n", uint32(p))
	c.Printf("  CurMod=%d PrvMod=%d IPL=%d IS=%v FPD=%v TP=%v CM=%v\n",
		p.CurMod(), p.PrvMod(), p.IPL(), p.IS(), p.FPD(), p.TP(), p.CM())
	c.Printf("  DV=%v FU=%v IV=%v T=%v  N=%v Z=%v V=%v C=%v\n",
		p.DV(), p.FU(), p.IV(), p.T(), p.N(), p.Z(), p.V(), p.C())
	return nil
}

// ShowMemory prints the size of physical memory, matching SHOW MEMORY.
func (c *Console) ShowMemory() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	size := c.Mem.Size()
	c.Printf("Physical memory: %08X bytes (%d pages)\n", size, size/512)
	return nil
}

// ShowSymbols prints every defined symbol, matching SHOW SYMBOLS/ALL.
func (c *Console) ShowSymbols() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	for _, s := range c.Symbols.All() {
		kind := "user"
		if s.Kind == SymbolSystem {
			kind = "system"
		}
		c.Printf("%-31s = %08X  (%s)\n", s.Name, s.Value, kind)
	}
	return nil
}

// ShowBreakpoints prints every active breakpoint, matching SHOW
// BREAKPOINTS.
func (c *Console) ShowBreakpoints() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	if len(c.Breakpoints) == 0 {
		c.Printf("No breakpoints set\n")
		return nil
	}
	for _, bp := range c.Breakpoints {
		c.Printf("Breakpoint at %08X\n", bp.Addr)
	}
	return nil
}

// ShowRadix prints the console's current default radix, matching SHOW
// RADIX.
func (c *Console) ShowRadix() error {
	c.Printf("Radix = %d\n", c.Radix)
	return nil
}

// ShowBase prints the region base/length registers, matching SHOW BASE.
func (c *Console) ShowBase() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	c.Printf("P0BR = %08X  P0LR = %08X\n", c.CPU.PR(vax.P0BR), c.CPU.PR(vax.P0LR))
	c.Printf("P1BR = %08X  P1LR = %08X\n", c.CPU.PR(vax.P1BR), c.CPU.PR(vax.P1LR))
	c.Printf("SBR  = %08X  SLR  = %08X\n", c.CPU.PR(vax.SBR), c.CPU.PR(vax.SLR))
	return nil
}

// StackKind selects which privileged-mode stack pointer ShowStack reports.
type StackKind int

const (
	StackKSP StackKind = iota
	StackESP
	StackSSP
	StackISP
	StackUSP
)

// ShowStack prints one privileged-mode stack pointer's value, matching SHOW
// KSP/ESP/SSP/ISP/USP.
func (c *Console) ShowStack(kind StackKind) error {
	if err := c.requireInit(); err != nil {
		return err
	}
	var name string
	var reg vax.PrivReg
	switch kind {
	case StackESP:
		name, reg = "ESP", vax.ESP
	case StackSSP:
		name, reg = "SSP", vax.SSP
	case StackISP:
		name, reg = "ISP", vax.ISP
	case StackUSP:
		name, reg = "USP", vax.USP
	default:
		name, reg = "KSP", vax.KSP
	}
	c.Printf("%s = %08X\n", name, c.CPU.PR(reg))
	return nil
}

// ShowCPU prints a one-line machine status summary, matching SHOW
// CPU_STATUS.
func (c *Console) ShowCPU() error {
	if err := c.requireInit(); err != nil {
		return err
	}
	state := "running"
	if c.Engine.Halted() {
		state = "halted"
	}
	c.Printf("CPU is %s, PC = %08X\n", state, c.CPU.GPR(vax.PC))
	return nil
}

// ShowVersion prints a version banner, matching SHOW VERSION (and ABOUT,
// which in the C source shares the same /entry= routine — see doc.go on
// why that indirection isn't ported).
func (c *Console) ShowVersion() error {
	c.Printf("govax — a Go port of eVAX (docs/PLAN.md)\n")
	return nil
}
