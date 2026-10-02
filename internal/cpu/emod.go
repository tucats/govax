package cpu

import "github.com/tucats/govax/internal/vaxfloat"

// EMODF, EMODD, EMODG, and EMODH: Extended Multiply and Integerize. The
// arithmetic is internal/vaxfloat's EMOD (see it for the manual's
// definition); this is the operand handling and the condition codes.
//
//	EMODx mulr.rx, mulrx.rb (F, D) or .rw (G, H), muld.rx, int.wl, fract.wx
//
// N and Z come from the fraction, V is set on integer overflow (the
// integer operand gets the low-order 32 bits of the true integer part,
// and with PSL<IV> set an integer overflow trap follows), and C is
// cleared. A reserved operand faults with both destinations unchanged.
// If the fraction underflows, the integer and fraction are both stored
// as zero when PSL<FU> is clear, and it's a fault leaving both unchanged
// when it's set.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x54, emulEmod)   // EMODF
	reg(0x74, emulEmod)   // EMODD
	regFD(0x54, emulEmod) // EMODG
	regFD(0x74, emulEmod) // EMODH
}

func emulEmod(e *Engine, d *Decoded) error {
	mulr, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	ext, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	muld, err := e.loadFloat(d, 2)
	if err != nil {
		return err
	}

	integer, fract, err := vaxfloat.EMOD(operandFormat(d, 0), mulr, uint16(ext), muld)
	if err = e.floatException(err); err != nil {
		return err
	}

	low, overflow := lowOrderBits(integer, 4)

	if err := d.Operands[3].Store(e.cpu, e.mem, low); err != nil {
		return err
	}

	if err := e.storeFloat(d, 4, fract); err != nil {
		return err
	}

	setFloatCC(e.cpu, fract, false)

	psl := e.cpu.PSL()
	psl.SetV(overflow)
	e.cpu.SetPSL(psl)

	if overflow && psl.IV() {
		return e.arithmeticTrap(trapIntOvf)
	}

	return nil
}
