package cpu

import "github.com/tucats/govax/internal/vax"

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x00}), emulHalt) // HALT
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x01}), emulNop)  // NOP
}

// emulHalt is the port of emul_misc.c's emul_halt. HALT is privileged: outside
// kernel mode it's a privileged-instruction fault rather than actually
// halting, unless DebugUserHalt is set (default: on, matching
// initialization.c's alloc_vax default and emul_misc.c:215's
// `!(vax.debug & DBG_USERHALT) && vax.pslw.cur_mod > 0` check), which lets
// HALT stop the machine from any mode. See docs/PHASE-17.md sub-phase 2.
func emulHalt(e *Engine, d *Decoded) error {
	if !e.cpu.DebugEnabled(vax.DebugUserHalt) && e.cpu.PSL().CurMod() != vax.Kernel {
		return &Fault{Code: ExcPrivileged}
	}
	
	return ErrHalted
}

// emulNop is the port of emul_misc.c's emul_nop: does nothing.
func emulNop(e *Engine, d *Decoded) error {
	return nil
}
