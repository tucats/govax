package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_movc.c's MOVC3/MOVC5 (the file's namesake
// instructions; its MOVTC/MOVTUC and SCANC/SPANC are movtc.go's and
// scanc.go's, respectively -- all four live in emul_movc.c despite the
// filename).
//
// A register-mode source or destination address operand already faults
// EXC_RESADDR at decode time (AccessAddress, see operand.go and
// docs/DEVIATIONS.md's "Register mode used where OP_AD/OP_VA access is
// required" finding), replacing emul_movc3/emul_movc5's own
// is_register[]/EXC_RESOP checks -- the same decode-time enforcement already
// covers every OP_AD consumer project-wide, not just this phase's.
//
// The length operands are read as signed 16-bit values, matching the C
// source's `short len`/`short len1, len2` -- see docs/DEVIATIONS.md's open
// question on whether VAX character-string length operands are meant to be
// unsigned words instead (this project's whole string-instruction family
// shares the question; not resolved here).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x28, emulMovc3)
	reg(0x2C, emulMovc5)
}

// emulMovc3 is MOVC3: length bytes are copied from src to dst. Copies
// forward (ascending addresses) when src > dst, backward otherwise -- so
// that overlapping strings sharing a destination is copied in the direction
// that doesn't corrupt not-yet-read source bytes, matching emul_movc3's own
// overlap handling and the manual's "overlap ... does not affect the
// result" guarantee. N <- 0, Z <- 1, V <- 0, C <- 0 always, per the manual
// and emul_movc3 (both unconditional, regardless of length).
func emulMovc3(e *Engine, d *Decoded) error {
	lv, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length := int32(signExtend(lv, d.Operands[0].Size))
	src := d.Operands[1].Addr
	dst := d.Operands[2].Addr

	if src > dst {
		for length > 0 {
			b, err := e.mem.LoadByte(e.cpu, src)
			if err != nil {
				return err
			}

			if err := e.mem.StoreByte(e.cpu, dst, b); err != nil {
				return err
			}

			length--
			src++
			dst++
		}
	} else {
		n := length
		src = uint32(int32(src) + n)
		dst = uint32(int32(dst) + n)

		for length > 0 {
			length--
			src--
			dst--

			b, err := e.mem.LoadByte(e.cpu, src)
			if err != nil {
				return err
			}

			if err := e.mem.StoreByte(e.cpu, dst, b); err != nil {
				return err
			}
		}

		src = uint32(int32(src) + n)
		dst = uint32(int32(dst) + n)
	}

	e.cpu.SetGPR(vax.R0, 0)
	e.cpu.SetGPR(vax.R1, src)
	e.cpu.SetGPR(vax.R2, 0)
	e.cpu.SetGPR(vax.R3, dst)
	e.cpu.SetGPR(vax.R4, 0)
	e.cpu.SetGPR(vax.R5, 0)

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(true)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}

// emulMovc5 is MOVC5: the destination string is replaced by the source
// string; if the destination is longer, its excess highest-addressed bytes
// are set to fill, and if the source is longer, its excess bytes are simply
// not moved (reported back via R0). Copy direction (forward/backward) is
// chosen the same way as MOVC3, for the same overlap-safety reason. N/Z/C
// come from comparing the *original* source and destination lengths as
// 16-bit values (signed for N, unsigned for C, per the manual's LSS/LSSU and
// emul_movc5's own explicit `(unsigned short)` cast) -- not the post-copy
// remaining lengths. V <- 0.
func emulMovc5(e *Engine, d *Decoded) error {
	l1v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	srcLen := int16(signExtend(l1v, d.Operands[0].Size))

	l2v, err := d.Operands[3].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	dstLen := int16(signExtend(l2v, d.Operands[3].Size))

	fillv, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	fill := byte(fillv)
	src := d.Operands[1].Addr
	dst := d.Operands[4].Addr
	len1, len2 := int32(srcLen), int32(dstLen)

	if src > dst {
		for len1 != 0 && len2 != 0 {
			b, err := e.mem.LoadByte(e.cpu, src)
			if err != nil {
				return err
			}

			if err := e.mem.StoreByte(e.cpu, dst, b); err != nil {
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
		minLength := len1
		if uint32(len2) < uint32(len1) {
			minLength = len2
		}

		len2Saved := len2
		src = uint32(int32(src) + minLength)
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

			b, err := e.mem.LoadByte(e.cpu, src)
			if err != nil {
				return err
			}

			if err := e.mem.StoreByte(e.cpu, dst, b); err != nil {
				return err
			}
		}
		src = uint32(int32(src) + minLength)
		dst = uint32(int32(dst) + len2Saved)
	}

	e.cpu.SetGPR(vax.R0, uint32(len1))
	e.cpu.SetGPR(vax.R1, src)
	e.cpu.SetGPR(vax.R2, 0)
	e.cpu.SetGPR(vax.R3, dst)
	e.cpu.SetGPR(vax.R4, 0)
	e.cpu.SetGPR(vax.R5, 0)

	result, _, c := subResult(uint64(uint16(srcLen)), uint64(uint16(dstLen)), 2)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 2))
	psl.SetZ(isZero(result, 2))
	psl.SetV(false)
	psl.SetC(c)
	e.cpu.SetPSL(psl)

	return nil
}
