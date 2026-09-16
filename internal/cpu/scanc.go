package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_movc.c's emul_scanc, shared by SCANC and
// SPANC (distinguished by opcode function, same as the C source -- see
// movc.go for MOVC3/MOVC5/MOVTC/MOVTUC, this file's other siblings in the
// same C source file).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x2A, emulScanc) // SCANC
	reg(0x2B, emulScanc) // SPANC
}

// emulScanc is SCANC/SPANC: each byte of the string is used to index a
// 256-byte table, and the table byte selected is ANDed with mask. SCANC
// scans while that AND is zero, stopping (a "match") the first time it's
// nonzero; SPANC scans while it's nonzero, stopping the first time it's
// zero. Z <- 0 if a match was found, Z <- 1 if the string was exhausted
// without one (including a zero-length string, trivially "exhausted"); N/V/C
// are always 0.
//
// Fixes a bug found while porting: emul_scanc has this Z polarity exactly
// backwards -- `vax.pslw.z` starts 0 and is set to 1 the moment a match is
// found (`if ((test && ch) || (!test && !ch)) { vax.pslw.z = 1; break; }`),
// contradicting both instructions' manual entries in their Condition Codes
// section verbatim ("If a nonzero [SCANC] / zero [SPANC] AND result is
// detected, the condition code Z-bit is cleared; otherwise, the Z-bit is
// set") and their own Notes ("If the string has zero length, condition code
// Z is set just as though the entire string were scanned/spanned" -- which
// the C source's un-fixed default of `z = 0` for an empty string, which
// never enters the loop, directly contradicts). See docs/DEVIATIONS.md.
func emulScanc(e *Engine, d *Decoded) error {
	lv, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length := int32(signExtend(lv, d.Operands[0].Size))
	addr := d.Operands[1].Addr
	tbl := d.Operands[2].Addr

	maskv, err := d.Operands[3].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	mask := byte(maskv)
	scanc := d.Opcode.Function == 0x2A // else SPANC

	var n int32

	found := false

	for ; n < length; n++ {
		ch, err := e.mem.LoadByte(e.cpu, addr+uint32(n))
		if err != nil {
			return err
		}

		entry, err := e.mem.LoadByte(e.cpu, tbl+uint32(ch))
		if err != nil {
			return err
		}

		entry &= mask
		if (scanc && entry != 0) || (!scanc && entry == 0) {
			found = true

			break
		}
	}

	e.cpu.SetGPR(vax.R0, uint32(length-n))
	e.cpu.SetGPR(vax.R1, addr+uint32(n))
	e.cpu.SetGPR(vax.R2, 0)
	e.cpu.SetGPR(vax.R3, tbl)

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(!found)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)
	
	return nil
}
