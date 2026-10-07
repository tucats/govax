package cpu

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x94, emulClr) // CLRB
	reg(0xB4, emulClr) // CLRW
	reg(0xD4, emulClr) // CLRL
	reg(0x7C, emulClr) // CLRQ

	// CLRO (also spelled CLRH) is a two-byte opcode, 0xFD 0x7C. Two-byte
	// opcodes start with an "escape" byte (0xFD here) that selects a
	// second opcode table, so the lookup names both bytes.
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Extended: 0xFD, Function: 0x7C}), emulClr)
}

// emulClr is CLR{B,W,L,Q,O}: the destination operand is replaced by zero.
// For CLRO's 16-byte destination, Operand.Store zero-extends the 0 to all
// 128 bits.
// N <- 0, Z <- 1, V <- 0, C unaffected -- matching both the manual and
// emul_clr.c (which sets n/z/v directly and never touches vax.pslw.c).
func emulClr(e *Engine, d *Decoded) error {
	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(true)
	psl.SetV(false)
	e.cpu.SetPSL(psl)

	return d.Operands[0].Store(e.cpu, e.mem, 0)
}
