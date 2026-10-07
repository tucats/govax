package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vaxfloat"
)

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x4F, emulAcbFloat) // ACBF
	reg(0x6F, emulAcbFloat) // ACBD

	regFD(0x4F, emulAcbFloat) // ACBG
	regFD(0x6F, emulAcbFloat) // ACBH
}

// emulAcbFloat is ACBx: the addend is added to the index (rounded to the
// index's format), the index is replaced by the sum, and the branch is
// taken if the addend is positive or zero and the index is now less than
// or equal to the limit, or if the addend is negative and the index is
// now greater than or equal to the limit. N and Z come from the new
// index, V is cleared, and C is unaffected. Overflow is a fault, leaving
// the index unchanged; underflow with PSL<FU> clear makes the index zero,
// and the comparison goes on with it.
func emulAcbFloat(e *Engine, d *Decoded) error {
	limit, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	addend, err := e.loadFloat(d, 1)
	if err != nil {
		return err
	}

	index, err := e.loadFloat(d, 2)
	if err != nil {
		return err
	}

	index, err = vaxfloat.Add(operandFormat(d, 2), index, addend)
	if err = e.floatException(err); err != nil {
		return err
	}

	setFloatCC(e.cpu, index, true)

	if err := e.storeFloat(d, 2, index); err != nil {
		return err
	}

	cmp := index.Cmp(limit)

	if (addend.Sign() >= 0 && cmp <= 0) || (addend.Sign() < 0 && cmp >= 0) {
		e.cpu.SetGPR(vax.PC, d.Operands[3].Addr)
	}

	return nil
}
