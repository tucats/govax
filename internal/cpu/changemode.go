package cpu

import "github.com/tucats/govax/internal/vax"

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0xBC, emulChmx) // CHMK
	reg(0xBD, emulChmx) // CHME
	reg(0xBE, emulChmx) // CHMS
	reg(0xBF, emulChmx) // CHMU
}

// emulChmx is CHMK/CHME/CHMS/CHMU's single shared handler, matching
// emul_chmx: read the one word operand (the "change mode code", sign-
// extended -- the C source reads it through a `short *`), then raise the
// corresponding synchronous exception (EXC$CHMK + 4*(function-0xBC), i.e.
// EXC$CHMK/CHME/CHMS/CHMU for K/E/S/U respectively) with that code as the
// exception's one signal argument -- HandleFault does the rest (vector
// lookup, mode/stack switch, PC/PSL/args push).
//
// Two of emul_chmx's own checks aren't ported: refusing to execute on the
// interrupt stack (VAX_HALT -- no current fixture executes CHMx from there,
// and this port has no other "halt the machine" outcome to map that to yet)
// and the SRM's 12-byte new-stack probe before committing to the mode
// switch (this port's HandleFault doesn't have a way to fail a fault
// partway through and fall back to the old mode either). Both are real,
// specified VAX behavior, not guessed at, but neither is exercised by any
// current test fixture; revisit if that changes.
func emulChmx(e *Engine, d *Decoded) error {
	raw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	code := uint32(int32(int16(uint16(raw))))

	// CHMx is a synchronous "system call" exception, not a retry-the-faulting-
	// instruction one: its own return address must be the instruction *after*
	// CHMx.
	e.instructionPC = e.cpu.GPR(vax.PC)

	modeIndex := d.Opcode.Function - 0xBC // 0=K, 1=E, 2=S, 3=U

	return &Fault{Code: ExcChangeModeK + Exception(modeIndex*4), Args: []uint32{code}}
}
