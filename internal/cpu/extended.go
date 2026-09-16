package cpu

// This is the Go port of emul_extended.c: EMUL and EDIV.
//
// The C source's own product/dividend handling swaps the two halves of its
// `union XLONG` between the quadword and longword-pair views
// (`add = xlong.reg[0]; xlong.reg[0] = xlong.reg[1]; xlong.reg[1] = add;`) --
// a workaround for `QUADWORD`/`LONGWORD` being 8 bytes each on this build
// (see reference/AUDIT.md), not real VAX semantics. This port computes the
// 64-bit product/dividend directly with Go's native int64, so there's no
// pair to swap.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x7A, emulEmul) // EMUL
	reg(0x7B, emulEdiv) // EDIV
}

// emulEmul is EMUL: prod <- mulr*muld + sign_extend(add), a double-length
// (quadword) result. mulr/muld are both longwords, so their exact product
// always fits in 63 bits -- adding a sign-extended longword can't overflow a
// quadword either -- matching the manual's "V <- 0" always.
func emulEmul(e *Engine, d *Decoded) error {
	mulrRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	muldRaw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	addRaw, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	mulr := int64(int32(uint32(mulrRaw)))
	muld := int64(int32(uint32(muldRaw)))
	add := int64(int32(uint32(addRaw)))

	product := mulr*muld + add

	psl := e.cpu.PSL()
	psl.SetN(product < 0)
	psl.SetZ(product == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return d.Operands[3].Store(e.cpu, e.mem, uint64(product))
}

// emulEdiv is EDIV: quo/rem <- divd / divr, divd % divr, a double-length
// (quadword) dividend divided by a longword divisor.
//
// Per Note 3, a zero divisor doesn't fault -- the quotient becomes bits
// 31:0 of the dividend, the remainder becomes zero, and V is set. The
// manual's own second V condition, a genuine quotient overflow ("the
// divisor operand is... small compared to the dividend operand, [and] the
// quotient... will not fit into 32 bits" -- same fallback as Note 3),
// wasn't checked at all by emul_extended.c (it computed and truncated the
// quotient unconditionally); fixed here in Phase 12 using the same
// longMin/longMax bounds check the float->integer conversions already use
// -- see docs/DEVIATIONS.md. No other instruction in this codebase raises
// the architected arithmetic-trap fault for an integer-overflow V yet
// either (see emulDiv in internal/cpu/integermath.go), so this isn't a gap
// unique to EDIV.
func emulEdiv(e *Engine, d *Decoded) error {
	divrRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	dividendRaw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	divr := int32(uint32(divrRaw))
	dividend := int64(dividendRaw)

	var quo, rem int32
	
	v := false

	switch {
	case divr == 0:
		quo = int32(uint32(dividendRaw))
		rem = 0
		v = true

	default:
		q := dividend / int64(divr)
		r := dividend % int64(divr)

		if q < longMin || q > longMax {
			quo = int32(uint32(dividendRaw))
			rem = 0
			v = true
		} else {
			quo = int32(q)
			rem = int32(r)
		}
	}

	psl := e.cpu.PSL()
	psl.SetN(quo < 0)
	psl.SetZ(quo == 0)
	psl.SetV(v)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	if err := d.Operands[2].Store(e.cpu, e.mem, uint64(uint32(quo))); err != nil {
		return err
	}

	return d.Operands[3].Store(e.cpu, e.mem, uint64(uint32(rem)))
}
