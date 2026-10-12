package cpu

import "github.com/tucats/govax/internal/registers"

// MOVTC (Move Translated Characters) and MOVTUC (Move Translated Until
// Character). MOVTC's copy is MoveCharacters (movc.go), with the table.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x2E, emulMovtc)
	reg(0x2F, emulMovtuc)
}

// emulMovtc is MOVTC srclen.rw, srcaddr.ab, fill.rb, tbladdr.ab,
// dstlen.rw, dstaddr.ab: the source string, each byte translated through
// the 256-byte table, replaces the destination string, padded with fill
// or truncated. R0 gets the source bytes not moved, R1 the address of the
// first of them, R2 and R4 0, R3 the table's address, and R5 the address
// just past the destination. N, Z, and C compare the two lengths as
// words; V is cleared.
func emulMovtc(e *Engine, d *Decoded) error {
	return moveCharacters(e, d, moveOperands{srcLen: 0, src: 1, fill: 2, table: 3, dstLen: 4, dst: 5})
}

// emulMovtuc is MOVTUC: like MOVTC, but translation stops as soon as a
// translated byte equals the escape operand (V <- 1), rather than
// continuing to the end of either string (V <- 0). The manual explicitly
// leaves destination-overlaps-table and (unlike MOVTC) source/destination
// overlap other than identical addresses UNPREDICTABLE, so -- matching
// emul_movtuc's own single forward-only loop -- no backward-copy branch is
// needed here.
//
// Fixes two bugs found while porting emul_movtuc: its loop condition is
// `while (tmp1 & tmp3)` -- a bitwise AND of the two remaining lengths, true
// only when they happen to share a set bit (e.g. false for
// remaining-lengths 2 and 1, despite both being nonzero) -- where "both
// lengths still nonzero" was clearly intended, matching every sibling
// string instruction's `!= 0 && ... != 0` loop guard; and it never sets R2,
// though the manual specifies R2 <- 0 like every other instruction in this
// family. See docs/DEVIATIONS.md. Its lengths are unsigned, 0 to 65,535,
// as every string instruction's are.
func emulMovtuc(e *Engine, d *Decoded) error {
	l1v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	srcLen := uint16(l1v)

	l2v, err := d.Operands[4].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	dstLen := uint16(l2v)

	escv, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	esc := byte(escv)
	src := d.Operands[1].Addr
	tbl := d.Operands[3].Addr
	dst := d.Operands[5].Addr
	len1, len2 := int32(srcLen), int32(dstLen)
	escaped := false

	for len1 != 0 && len2 != 0 {
		ch, err := e.mem.LoadByte(e.cpu, src)
		if err != nil {
			return err
		}

		translated, err := e.mem.LoadByte(e.cpu, tbl+uint32(ch))
		if err != nil {
			return err
		}

		if translated == esc {
			escaped = true
			
			break
		}

		if err := e.mem.StoreByte(e.cpu, dst, translated); err != nil {
			return err
		}

		len1--
		src++
		len2--
		dst++
	}

	e.cpu.SetGPR(registers.R0, uint32(len1))
	e.cpu.SetGPR(registers.R1, src)
	e.cpu.SetGPR(registers.R2, 0)
	e.cpu.SetGPR(registers.R3, tbl)
	e.cpu.SetGPR(registers.R4, uint32(len2))
	e.cpu.SetGPR(registers.R5, dst)

	result, _, c := subResult(uint64(uint16(srcLen)), uint64(uint16(dstLen)), 2)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 2))
	psl.SetZ(isZero(result, 2))
	psl.SetV(escaped)
	psl.SetC(c)
	e.cpu.SetPSL(psl)
	
	return nil
}
