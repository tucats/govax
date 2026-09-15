package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_branch.c's emul_acb (ACBB/ACBW/ACBL; ACBF is
// Phase 05's) and emul_case (CASEB/CASEW/CASEL) -- more involved control
// flow than the simple/generic branches in branch.go, kept separate.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x9D, emulAcb(1)) // ACBB
	reg(0x3D, emulAcb(2)) // ACBW
	reg(0xF1, emulAcb(4)) // ACBL

	reg(0x8F, emulCase(1)) // CASEB
	reg(0xAF, emulCase(2)) // CASEW
	reg(0xCF, emulCase(4)) // CASEL
}

// emulAcb builds ACBB/ACBW/ACBL's Handler for the given operand size: the
// addend is added to the index (same overflow formula as ADD -- and, per
// the manual, "on integer overflow, the index operand is replaced by the
// low-order bits of the true result," exactly the wraparound addResult
// already produces), then the (updated) index is compared with the limit
// to decide whether to branch. N/Z/V come from the updated index; C is
// unaffected (never written), matching both the manual and the C source
// (which never assigns vax.pslw.c in emul_acb at all).
//
// The branch condition fixes a confirmed bug in emul_acb.c: for a
// non-negative addend, the manual specifies branching when index is less
// than *or equal to* the limit, but the C source uses a strict `<`,
// missing the case where a loop's index lands exactly on its limit (e.g.
// counting up to and including limit). The negative-addend case (`index >=
// limit`) was already correct in the C source. See docs/DEVIATIONS.md.
func emulAcb(size int) Handler {
	return func(e *Engine, d *Decoded) error {
		limit, err := d.Operands[0].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}

		addend, err := d.Operands[1].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}

		index, err := d.Operands[2].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}

		result, v, _ := addResult(index, addend, size)
		psl := e.cpu.PSL()
		psl.SetN(signBit(result, size))
		psl.SetZ(isZero(result, size))
		psl.SetV(v)
		e.cpu.SetPSL(psl)

		if err := d.Operands[2].Store(e.cpu, e.mem, result); err != nil {
			return err
		}

		newIndex := signExtend(result, size)
		limitSigned := signExtend(limit, size)

		var branch bool

		if signExtend(addend, size) >= 0 {
			branch = newIndex <= limitSigned
		} else {
			branch = newIndex >= limitSigned
		}

		if branch {
			e.cpu.SetGPR(vax.PC, d.Operands[3].Addr)
		}

		return nil
	}
}

// emulCase builds CASEB/CASEW/CASEL's Handler for the given operand size:
// the base operand is subtracted from the selector, giving a temporary
// value compared (unsigned) against the limit to select a branch
// displacement from the table immediately following the instruction (one
// entry per value 0..limit), or to skip past the whole table if the
// temporary exceeds the limit.
//
// Matches emul_case.c's choice to sign-extend the byte/word selector/base/
// limit operands to full 32 bits before the subtraction/comparison/
// indexing, rather than working at the operand's own narrower width the
// way most of this phase's other instructions do. The manual's own note
// ("the selector and base operands can both be considered as either signed
// or unsigned") doesn't settle which width the internal arithmetic
// actually happens at, and the two choices produce different results
// exactly when an operand's own high bit is set -- this wasn't resolved
// either way, so the C source's behavior is replicated rather than guessed
// at; a genuine open question, not logged as a docs/DEVIATIONS.md finding
// (no confirmed mismatch to log).
func emulCase(size int) Handler {
	return func(e *Engine, d *Decoded) error {
		selRaw, err := d.Operands[0].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}

		baseRaw, err := d.Operands[1].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}

		limitRaw, err := d.Operands[2].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}

		selector := signExtend(selRaw, size)
		baseVal := signExtend(baseRaw, size)
		limit := uint32(signExtend(limitRaw, size))
		idx := uint32(selector - baseVal)

		psl := e.cpu.PSL()
		psl.SetN(int32(idx) < int32(limit))
		psl.SetV(false)

		// After operand decode, PC already points at displ[0] -- Engine.Step
		// writes Decoded.NextPC into PC before dispatching to this handler,
		// and CASE's branch-displacement table isn't decoded as an operand
		// at all (walked directly here instead), so NextPC lands exactly
		// where the manual's own note says PC should be.
		tableBase := e.cpu.GPR(vax.PC)

		if idx > limit {
			psl.SetZ(false)
			psl.SetC(false)
			e.cpu.SetPSL(psl)
			e.cpu.SetGPR(vax.PC, tableBase+2+limit*2)
			
			return nil
		}

		psl.SetZ(idx == limit)
		psl.SetC(idx < limit)
		e.cpu.SetPSL(psl)

		branchword, err := e.mem.LoadWord(e.cpu, tableBase+idx*2)
		if err != nil {
			return err
		}

		e.cpu.SetGPR(vax.PC, uint32(int32(tableBase)+int32(int16(branchword))))

		return nil
	}
}
