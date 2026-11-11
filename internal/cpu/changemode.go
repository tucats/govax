package cpu

import "github.com/tucats/govax/internal/registers"

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0xBC, emulChmx) // CHMK
	reg(0xBD, emulChmx) // CHME
	reg(0xBE, emulChmx) // CHMS
	reg(0xBF, emulChmx) // CHMU
}

// emulChmx is CHMK, CHME, CHMS, and CHMU: read the one word operand, the
// "change mode code", sign-extended, and raise the change-mode trap
// through the instruction's SCB vector (EXC$CHMK + 4*(function-0xBC)),
// the code its one parameter. HandleFault does the rest: the vector, the
// mode (the instruction's or the current one, whichever is more
// privileged), the stack, and the PSL, PC, and code pushed there.
//
// A CHMx on the interrupt stack halts the processor, as the architecture
// says. Its other check, that the new mode's stack can take the 12 bytes
// pushed (a fault if not), isn't made: HandleFault's push fails instead,
// as govax's stacks are always mapped.
func emulChmx(e *Engine, d *Decoded) error {
	raw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	code := uint32(int32(int16(uint16(raw))))

	if e.cpu.PSL().IS() {
		return ErrHalted // "illegal from I stack"
	}

	// CHMx is a synchronous "system call" exception, not a retry-the-faulting-
	// instruction one: its own return address must be the instruction *after*
	// CHMx.
	e.instructionPC = e.cpu.GPR(registers.PC)

	modeIndex := d.Opcode.Function - 0xBC // 0=K, 1=E, 2=S, 3=U

	// A trap, so a traced CHMx's saved PSL keeps T and TP, and the trace
	// comes after the service's REI (trace.go).
	return &Fault{Code: ExcChangeModeK + Exception(modeIndex*4), Args: []uint32{code}, Trap: true}
}
