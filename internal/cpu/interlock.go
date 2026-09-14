package cpu

// This is the Go port of emul_interlock.c: ADAWI, the only interlocked
// instruction this emulator implements (the C source's own comment notes
// interlock semantics have no meaning in a uniprocessor emulator, and this
// port doesn't emulate multiple processors either -- same rationale as the
// queue instructions in internal/cpu/queue.go).

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x58}), emulAdawi)
}

// emulAdawi is ADAWI: adds the addend operand to the sum operand in place.
// The sum operand must be memory (not a register) and word-aligned, or a
// reserved-operand fault occurs -- emul_interlock.c's own checks, since the
// table declares this operand AccessModify rather than AccessAddress/
// AccessVarField, so the Phase 03/04 decode-time register-mode fault
// (internal/cpu/operand.go) doesn't apply here.
//
// emul_interlock.c's condition-code handling doesn't match the manual in two
// ways, both replicated as-is here -- see docs/DEVIATIONS.md:
//   - C is supposed to be the carry out of the addition, but the C source's
//     SETCONDITIONBITS(data, 0L) call -- data compared against the constant
//     zero, both cast to ULONGWORD -- can only ever be false (an unsigned
//     value is never less than zero), so C always ends up cleared.
//   - N/Z are computed from the *untruncated* 32-bit sum (data, a LONGWORD
//     local holding addend+sum before it's narrowed to a word result), not
//     from the word actually stored. The two disagree exactly in the
//     overflow case: e.g. 32767+1 = 32768, positive as a 32-bit sum (N
//     clear) but -32768 once truncated to the stored word (N would be set
//     if computed from the truncated result instead).
func emulAdawi(e *Engine, d *Decoded) error {
	if d.Operands[1].Kind != OperandMemory {
		return &Fault{Code: ExcReservedOp}
	}
	if d.Operands[1].Addr&1 != 0 {
		return &Fault{Code: ExcReservedOp}
	}

	addendRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	sumRaw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	addend := int32(int16(uint16(addendRaw)))
	sum := int32(int16(uint16(sumRaw)))
	result := addend + sum

	psl := e.cpu.PSL()
	psl.SetN(result < 0)
	psl.SetZ(result == 0)
	psl.SetV(result > 32767 || result < -32768)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return d.Operands[1].Store(e.cpu, e.mem, uint64(uint16(int16(result))))
}
