package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_cmpc.c's CMPC3/CMPC5. Each byte comparison's
// N/Z/C are computed the way the manual's Condition Codes section states --
// N/Z/C from subResult at the compared width (signed LSS for N, unsigned
// LSSU for C, matching vax.h's SETCONDITIONBITS macro's LONGWORD/ULONGWORD
// cast pair) -- rather than porting the macro literally, since it's already
// this package's standard "N/Z from result, C from an unsigned compare"
// shape (condcodes.go).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x29, emulCmpc3)
	reg(0x2D, emulCmpc5)
}

// emulCmpc3 is CMPC3: the two length-byte strings at src1/src2 are compared
// until an inequality is found or the length is exhausted; condition codes
// reflect the last byte comparison made. A zero length compares equal (N/Z/C
// from comparing the length against 0); the manual and emul_cmpc3 agree a
// negative length ("R0=0 zero only if strings are equal" describes the
// length as an unsigned count, but the C source's own `short len` and
// `if (tmp1 > 0)` guard leave condition codes and R0-R3 from before the
// instruction entirely untouched for a negative length -- replicated as-is,
// same open question as movc.go's). V <- 0 unconditionally, regardless of
// which case applies.
func emulCmpc3(e *Engine, d *Decoded) error {
	lv, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	
	length := int32(signExtend(lv, d.Operands[0].Size))
	src1 := d.Operands[1].Addr
	src2 := d.Operands[2].Addr

	psl := e.cpu.PSL()
	if length == 0 {
		psl.SetN(false)
		psl.SetZ(true)
		psl.SetC(false)
	}

	for length > 0 {
		b1, err := e.mem.LoadByte(e.cpu, src1)
		if err != nil {
			return err
		}

		b2, err := e.mem.LoadByte(e.cpu, src2)
		if err != nil {
			return err
		}

		result, _, c := subResult(uint64(b1), uint64(b2), 1)
		psl.SetN(signBit(result, 1))
		psl.SetZ(isZero(result, 1))
		psl.SetC(c)

		if b1 != b2 {
			break
		}

		length--
		src1++
		src2++
	}

	psl.SetV(false)
	e.cpu.SetPSL(psl)

	e.cpu.SetGPR(vax.R0, uint32(length))
	e.cpu.SetGPR(vax.R1, src1)
	e.cpu.SetGPR(vax.R2, uint32(length))
	e.cpu.SetGPR(vax.R3, src2)

	return nil
}

// emulCmpc5 is CMPC5: like CMPC3, but the shorter of the two (independently
// lengthed) strings is conceptually extended with the fill byte to match the
// longer one. Condition codes start from comparing the two original lengths
// (as longwords, matching emul_cmpc5's own SETCONDITIONBITS(tmp1,tmp3) --
// the fallback result for two zero-length strings, per the manual's note 3),
// then are overwritten by whichever byte comparisons actually run.
//
// Ported with one fix and one known-quirk-preserved: emul_cmpc5.c wraps all
// three of its comparison loops (the dual-string loop, and both fill loops)
// in a single `if (tmp1 > 0 && tmp3 > 0)` gate, which -- unlike the
// structurally identical emul_movc5.c, whose fill loops are gated only by
// their own lengths -- skips the fill-padding comparison entirely whenever
// either length starts at exactly 0, contradicting the manual's "shorter
// string is conceptually extended" description for that case; not present
// here (the outer gate is dropped, each loop uses its own condition, mirror
// emulMovc5). Preserved as-is: if the dual-string loop exits because it
// found unequal bytes (not because a length reached zero), the fill loops
// still run afterward using whatever length is left over -- comparing the
// *same already-mismatched* source1 byte against fill rather than stopping
// comparison at the first inequality the way the manual's prose describes.
// See docs/DEVIATIONS.md; not resolved either way, replicated as the
// literal three-loop structure produces.
func emulCmpc5(e *Engine, d *Decoded) error {
	l1v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	len1 := int32(signExtend(l1v, d.Operands[0].Size))

	l2v, err := d.Operands[3].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	len2 := int32(signExtend(l2v, d.Operands[3].Size))

	fillv, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	fill := byte(fillv)
	src1 := d.Operands[1].Addr
	src2 := d.Operands[4].Addr

	result, _, c := subResult(uint64(uint32(len1)), uint64(uint32(len2)), 4)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 4))
	psl.SetZ(isZero(result, 4))
	psl.SetC(c)

	// inequality tracks whether the main loop below stopped because it
	// found a mismatching byte pair, as opposed to running one or both
	// strings to exhaustion -- the manual's own CMPC entry is explicit
	// that "comparison proceeds until inequality is detected or all the
	// bytes of the strings have been examined[;] condition codes are
	// affected by the result of the last byte comparison": once an
	// inequality is found, the whole operation is over, so the two
	// fill-padding loops below must not run and re-run SETCONDITIONBITS
	// against the fill byte, discarding the real mismatch result. See
	// docs/DEVIATIONS.md's own CMPC5 finding, confirmed against the
	// manual and fixed here in Phase 12.
	inequality := false

	for len1 != 0 && len2 != 0 {
		b1, err := e.mem.LoadByte(e.cpu, src1)
		if err != nil {
			return err
		}

		b2, err := e.mem.LoadByte(e.cpu, src2)
		if err != nil {
			return err
		}

		result, _, c := subResult(uint64(b1), uint64(b2), 1)
		psl.SetN(signBit(result, 1))
		psl.SetZ(isZero(result, 1))
		psl.SetC(c)

		if b1 != b2 {
			inequality = true
			
			break
		}

		len1--
		src1++
		len2--
		src2++
	}

	for !inequality && len1 != 0 {
		b, err := e.mem.LoadByte(e.cpu, src1)
		if err != nil {
			return err
		}

		result, _, c := subResult(uint64(b), uint64(fill), 1)
		psl.SetN(signBit(result, 1))
		psl.SetZ(isZero(result, 1))
		psl.SetC(c)

		if b != fill {
			break
		}

		len1--
		src1++
	}

	for !inequality && len2 != 0 {
		b, err := e.mem.LoadByte(e.cpu, src2)
		if err != nil {
			return err
		}

		result, _, c := subResult(uint64(fill), uint64(b), 1)
		psl.SetN(signBit(result, 1))
		psl.SetZ(isZero(result, 1))
		psl.SetC(c)

		if fill != b {
			break
		}

		len2--
		src2++
	}

	psl.SetV(false)
	e.cpu.SetPSL(psl)

	e.cpu.SetGPR(vax.R0, uint32(len1))
	e.cpu.SetGPR(vax.R1, src1)
	e.cpu.SetGPR(vax.R2, uint32(len2))
	e.cpu.SetGPR(vax.R3, src2)

	return nil
}
