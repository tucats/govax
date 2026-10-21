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

	product := ExtendedMultiply(int32(uint32(mulrRaw)), int32(uint32(muldRaw)), int32(uint32(addRaw)))

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
// -- see docs/DEVIATIONS.md. A zero divisor then traps (integer divide by
// zero), and a quotient overflow traps under PSL<IV> (inttrap.go).
func emulEdiv(e *Engine, d *Decoded) error {
	divrRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	dividendRaw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	quo, rem, ok := ExtendedDivide(int32(uint32(divrRaw)), int64(dividendRaw))

	psl := e.cpu.PSL()
	psl.SetN(quo < 0)
	psl.SetZ(quo == 0)
	psl.SetV(!ok)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	if err := d.Operands[2].Store(e.cpu, e.mem, uint64(uint32(quo))); err != nil {
		return err
	}

	if err := d.Operands[3].Store(e.cpu, e.mem, uint64(uint32(rem))); err != nil {
		return err
	}

	if uint32(divrRaw) == 0 {
		return e.integerDivideTrap()
	}

	return nil
}

// ExtendedMultiply is EMUL's arithmetic, shared with LIB$EMUL
// (internal/librtl): multiplier times multiplicand plus the
// sign-extended addend, as a quadword. It can't overflow.
func ExtendedMultiply(multiplier, multiplicand, addend int32) int64 {
	return int64(multiplier)*int64(multiplicand) + int64(addend)
}

// ExtendedDivide is EDIV's arithmetic, shared with LIB$EDIV
// (internal/librtl): the quadword dividend divided by the longword
// divisor, the remainder taking the dividend's sign. ok is false when
// the divisor is 0 or the quotient doesn't fit in a longword; the
// quotient is then the dividend's low longword and the remainder 0.
func ExtendedDivide(divisor int32, dividend int64) (quotient, remainder int32, ok bool) {
	if divisor == 0 {
		return int32(uint32(dividend)), 0, false
	}

	q := dividend / int64(divisor)
	if q < longMin || q > longMax {
		return int32(uint32(dividend)), 0, false
	}

	return int32(q), int32(dividend % int64(divisor)), true
}
