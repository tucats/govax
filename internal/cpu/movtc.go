package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_movc.c's emul_movtc/emul_movtuc (MOVTC/MOVTUC
// -- both live in emul_movc.c despite the filename, alongside MOVC3/MOVC5;
// see movc.go).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x2E, emulMovtc)
	reg(0x2F, emulMovtuc)
}

// emulMovtc is MOVTC: the source string, translated byte-by-byte through a
// 256-entry table, replaces the destination string; a destination longer
// than the source is padded with fill, a destination shorter than the
// source leaves the source's excess untranslated. Copy direction mirrors
// MOVC5's overlap handling (movc.go's emulMovc5): forward when src > dst,
// backward otherwise. N/Z/C come from comparing the original 16-bit source
// and destination lengths; V <- 0.
//
// Fixes one bug found while porting: emul_movtc's backward-copy branch
// translates each byte via `load_byte(tbladdr + tmp2, &ch)` -- tmp2 being
// the *source address*, not the source byte it just loaded into (and
// promptly discarded) -- instead of `tbladdr + ch`, the correct index its
// own forward-copy branch uses two lines above it in the same file. See
// docs/DEVIATIONS.md.
func emulMovtc(e *Engine, d *Decoded) error {
	l1v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	srcLen := int16(signExtend(l1v, d.Operands[0].Size))

	l2v, err := d.Operands[4].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	dstLen := int16(signExtend(l2v, d.Operands[4].Size))

	fillv, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	fill := byte(fillv)

	src := d.Operands[1].Addr
	tbl := d.Operands[3].Addr
	dst := d.Operands[5].Addr

	translate := func(addr uint32) (byte, error) {
		ch, err := e.mem.LoadByte(e.cpu, addr)
		if err != nil {
			return 0, err
		}
		return e.mem.LoadByte(e.cpu, tbl+uint32(ch))
	}

	len1, len2 := int32(srcLen), int32(dstLen)

	if src > dst {
		for len1 != 0 && len2 != 0 {
			ch, err := translate(src)
			if err != nil {
				return err
			}
			if err := e.mem.StoreByte(e.cpu, dst, ch); err != nil {
				return err
			}
			len1--
			src++
			len2--
			dst++
		}
		for len2 != 0 {
			if err := e.mem.StoreByte(e.cpu, dst, fill); err != nil {
				return err
			}
			len2--
			dst++
		}
	} else {
		min := len1
		if uint32(len2) < uint32(len1) {
			min = len2
		}
		len2Saved := len2
		src = uint32(int32(src) + min)
		dst = uint32(int32(dst) + len2Saved)

		for len2 > len1 {
			len2--
			dst--
			if err := e.mem.StoreByte(e.cpu, dst, fill); err != nil {
				return err
			}
		}
		for len2 != 0 {
			len1--
			src--
			len2--
			dst--
			ch, err := translate(src)
			if err != nil {
				return err
			}
			if err := e.mem.StoreByte(e.cpu, dst, ch); err != nil {
				return err
			}
		}
		src = uint32(int32(src) + min)
		dst = uint32(int32(dst) + len2Saved)
	}

	e.cpu.SetGPR(vax.R0, uint32(len1))
	e.cpu.SetGPR(vax.R1, src)
	e.cpu.SetGPR(vax.R2, 0)
	e.cpu.SetGPR(vax.R3, tbl)
	e.cpu.SetGPR(vax.R4, 0)
	e.cpu.SetGPR(vax.R5, dst)

	result, _, c := subResult(uint64(uint16(srcLen)), uint64(uint16(dstLen)), 2)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 2))
	psl.SetZ(isZero(result, 2))
	psl.SetV(false)
	psl.SetC(c)
	e.cpu.SetPSL(psl)
	return nil
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
// family. See docs/DEVIATIONS.md.
func emulMovtuc(e *Engine, d *Decoded) error {
	l1v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	srcLen := int16(signExtend(l1v, d.Operands[0].Size))

	l2v, err := d.Operands[4].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	dstLen := int16(signExtend(l2v, d.Operands[4].Size))

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

	e.cpu.SetGPR(vax.R0, uint32(len1))
	e.cpu.SetGPR(vax.R1, src)
	e.cpu.SetGPR(vax.R2, 0)
	e.cpu.SetGPR(vax.R3, tbl)
	e.cpu.SetGPR(vax.R4, uint32(len2))
	e.cpu.SetGPR(vax.R5, dst)

	result, _, c := subResult(uint64(uint16(srcLen)), uint64(uint16(dstLen)), 2)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 2))
	psl.SetZ(isZero(result, 2))
	psl.SetV(escaped)
	psl.SetC(c)
	e.cpu.SetPSL(psl)
	return nil
}
