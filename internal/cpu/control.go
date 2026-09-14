package cpu

import "github.com/tucats/govax/internal/vax"

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x00}), emulHalt) // HALT
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x01}), emulNop)  // NOP
}

// emulHalt is the port of emul_misc.c's emul_halt. HALT is privileged: outside
// kernel mode it's a privileged-instruction fault rather than actually
// halting. The C source has a `vax.debug & DBG_USERHALT` escape hatch that
// lets the console allow HALT from any mode for debugging convenience — a
// Phase 08 console concern with no `vax.debug` equivalent yet, so this always
// enforces the architected kernel-mode check (the same behavior as that debug
// flag being off, which is the correct default anyway).
func emulHalt(e *Engine, d *Decoded) error {
	if e.cpu.PSL().CurMod() != vax.Kernel {
		return &Fault{Code: ExcPrivileged}
	}
	return ErrHalted
}

// emulNop is the port of emul_misc.c's emul_nop: does nothing.
func emulNop(e *Engine, d *Decoded) error {
	return nil
}
