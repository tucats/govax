package cpu

// This is the Go port of emul_clr.c.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x94, emulClr) // CLRB
	reg(0xB4, emulClr) // CLRW
	reg(0xD4, emulClr) // CLRL
	reg(0x7C, emulClr) // CLRQ
}

// emulClr is CLR{B,W,L,Q}: the destination operand is replaced by zero.
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
