package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_matchc.c's MATCHC: a naive substring search
// (the source string is scanned for a contiguous run matching the object
// string) with backtracking on mismatch. Traced by hand against
// vax_instr_set.pdf's MATCHC entry and its three Notes (match found,
// zero-length object, zero-length source with nonzero object) while
// porting -- emul_matchc's arithmetic, while terse, is correct for all
// three; nothing to fix here, unlike this phase's other C source files.

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x39}), emulMatchc)
}

// emulMatchc is MATCHC. objLen/srcLen/matched are kept as uint16 (not a
// wider type) deliberately: emul_matchc.c declares its equivalents
// `unsigned short`, and the backtracking arithmetic below relies on the
// same 16-bit truncate-on-assignment behavior after an intermediate value
// that can transiently go negative (cast to int32 for the subtraction,
// truncated back to uint16 immediately after, mirroring C's integer
// promotion of a uint16 operand to a signed int for the expression, then
// implicit truncation back on assignment to the uint16 variable).
func emulMatchc(e *Engine, d *Decoded) error {
	l1v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	objLen := uint16(l1v)
	objAddr := d.Operands[1].Addr

	l2v, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	srcLen := uint16(l2v)
	srcAddr := d.Operands[3].Addr

	remainObj, remainSrc, origObjLen := objLen, srcLen, objLen
	objPtr, srcPtr := objAddr, srcAddr

	for remainObj != 0 && remainSrc >= remainObj {
		a, err := e.mem.LoadByte(e.cpu, objPtr)
		if err != nil {
			return err
		}

		b, err := e.mem.LoadByte(e.cpu, srcPtr)
		if err != nil {
			return err
		}
		
		if a == b {
			remainObj--
			objPtr++
			remainSrc--
			srcPtr++

			continue
		}
		// Mismatch: back the object pointer up to its start, advance the
		// source search position by one from where this attempt began, and
		// restart the object comparison from scratch.
		matched := int32(origObjLen) - int32(remainObj)
		objPtr = uint32(int32(objPtr) - matched)
		remainSrc = uint16(int32(remainSrc) + matched - 1)
		srcPtr = uint32(int32(srcPtr) - (matched - 1))
		remainObj = origObjLen
	}

	if remainSrc < remainObj {
		// Ran out of source before completing a match anywhere: report "not
		// found" (R3 = one past the whole source string, R2 = 0).
		srcPtr += uint32(remainSrc)
		remainSrc = 0
	}

	e.cpu.SetGPR(vax.R0, uint32(remainObj))
	e.cpu.SetGPR(vax.R1, objPtr)
	e.cpu.SetGPR(vax.R2, uint32(remainSrc))
	e.cpu.SetGPR(vax.R3, srcPtr)

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(remainObj == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}
