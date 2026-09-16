package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_loop.c: AOBLEQ, AOBLSS, SOBGTR, SOBGEQ. All
// four are longword-only and share the same SETCONDITIONBITS(x, 0L)-idiom
// finding as ROTL/ASHL/ASHQ/MOVQ (docs/DEVIATIONS.md): the C source never
// computes V at all (stale from whatever the previous instruction left),
// and force-clears C where the manual specifies unaffected. Fixed here: V
// reuses addResult/subResult's overflow formula (AOBLEQ/AOBLSS are exactly
// INCL; SOBGTR/SOBGEQ are exactly DECL, sharing their condition-code
// formulas -- only the C treatment differs, per the manual, from INC/DEC's
// real carry/borrow), and C is never written.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0xF3, emulAobleq)
	reg(0xF2, emulAoblss)
	reg(0xF5, emulSobgtr)
	reg(0xF4, emulSobgeq)
}

// stepIndex adds delta (+1 or -1) to the index operand at idx, sets N/Z/V
// (C left untouched), stores the result back, and returns it for the
// caller's branch test.
func stepIndex(e *Engine, d *Decoded, idx int, delta int64) (int64, error) {
	v, err := d.Operands[idx].Load(e.cpu, e.mem)
	if err != nil {
		return 0, err
	}

	var (
		result   uint64
		overflow bool
	)

	if delta > 0 {
		result, overflow, _ = addResult(v, uint64(delta), 4)
	} else {
		result, overflow, _ = subResult(v, uint64(-delta), 4)
	}

	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 4))
	psl.SetZ(isZero(result, 4))
	psl.SetV(overflow)
	e.cpu.SetPSL(psl)

	if err := d.Operands[idx].Store(e.cpu, e.mem, result); err != nil {
		return 0, err
	}

	return signExtend(result, 4), nil
}

// emulAobleq is AOBLEQ: index <- index + 1; branch if index <= limit.
func emulAobleq(e *Engine, d *Decoded) error {
	limit, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	index, err := stepIndex(e, d, 1, 1)
	if err != nil {
		return err
	}

	if index <= signExtend(limit, 4) {
		e.cpu.SetGPR(vax.PC, d.Operands[2].Addr)
	}

	return nil
}

// emulAoblss is AOBLSS: index <- index + 1; branch if index < limit.
func emulAoblss(e *Engine, d *Decoded) error {
	limit, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	index, err := stepIndex(e, d, 1, 1)
	if err != nil {
		return err
	}

	if index < signExtend(limit, 4) {
		e.cpu.SetGPR(vax.PC, d.Operands[2].Addr)
	}

	return nil
}

// emulSobgtr is SOBGTR: index <- index - 1; branch if index > 0.
func emulSobgtr(e *Engine, d *Decoded) error {
	index, err := stepIndex(e, d, 0, -1)
	if err != nil {
		return err
	}

	if index > 0 {
		e.cpu.SetGPR(vax.PC, d.Operands[1].Addr)
	}

	return nil
}

// emulSobgeq is SOBGEQ: index <- index - 1; branch if index >= 0.
func emulSobgeq(e *Engine, d *Decoded) error {
	index, err := stepIndex(e, d, 0, -1)
	if err != nil {
		return err
	}

	if index >= 0 {
		e.cpu.SetGPR(vax.PC, d.Operands[1].Addr)
	}
	
	return nil
}
