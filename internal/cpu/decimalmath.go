package cpu

import "math/big"

// ADDP4, ADDP6, SUBP4, SUBP6, MULP, DIVP, and ASHP: packed decimal
// arithmetic (docs/PHASE-35.md, subtask 11). See decimal.go for the
// strings, their signs, overflow, and how a fault part way is handled.
//
// Each computes the exact result from its sources' values, then stores it
// in the destination's length: its low-order digits and V if it doesn't
// fit, and under PSL<DV> a decimal overflow trap after everything is
// stored. N and Z come from the stored result, C is cleared. The
// registers are the manual's: R0, R2, and R4 zero, and R1, R3, and R5
// the addresses of the operand strings in order (R1 and R3 only for the
// 4-operand forms and ASHP).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x20, emulAddp4) // ADDP4
	reg(0x21, emulAddp6) // ADDP6
	reg(0x22, emulSubp4) // SUBP4
	reg(0x23, emulSubp6) // SUBP6
	reg(0x25, emulMulp)  // MULP
	reg(0x27, emulDivp)  // DIVP
	reg(0xF8, emulAshp)  // ASHP
}

// packedOperand is one string operand: its length and address.
type packedOperand struct {
	length int
	addr   uint32
}

// packedOperands loads decoded instruction d's string operands, given as
// the indexes of their length operands (each followed by its address).
// Every length is checked before any string is read.
func (e *Engine) packedOperands(d *Decoded, lengthIndexes ...int) ([]packedOperand, error) {
	ops := make([]packedOperand, len(lengthIndexes))

	for i, li := range lengthIndexes {
		length, err := e.decimalLength(d, li)
		if err != nil {
			return nil, err
		}

		ops[i] = packedOperand{length: length, addr: d.Operands[li+1].Addr}
	}

	return ops, nil
}

// values reads the strings ops and returns their values.
func (e *Engine) values(ops []packedOperand) ([]*big.Int, error) {
	vs := make([]*big.Int, len(ops))

	for i, op := range ops {
		p, err := e.readPacked(op.addr, op.length)
		if err != nil {
			return nil, err
		}

		vs[i] = p.value()
	}

	return vs, nil
}

// storeDecimal stores v in the destination dst, sets the registers to
// zero and the operand addresses alternately (R0 = 0, R1 = the first
// address, R2 = 0, ...), sets the condition codes, and returns the
// decimal overflow trap if one is due.
func (e *Engine) storeDecimal(dst packedOperand, v *big.Int, addrs ...uint32) error {
	digits, neg, overflow := decimalDigits(v, dst.length)

	if err := e.writePacked(dst.addr, digits, neg); err != nil {
		return err
	}

	regs := make([]uint32, 0, 2*len(addrs))
	for _, a := range addrs {
		regs = append(regs, 0, a)
	}

	e.setRegisters(regs...)
	setDecimalCC(e.cpu, digits, neg, overflow, false)

	return e.decimalOverflowTrap(overflow)
}

// emulAddp4 is ADDP4 addlen, addaddr, sumlen, sumaddr: sum <- sum + add.
func emulAddp4(e *Engine, d *Decoded) error {
	return e.decimal4(d, func(add, sum *big.Int) *big.Int { return new(big.Int).Add(sum, add) })
}

// emulSubp4 is SUBP4 sublen, subaddr, diflen, difaddr: dif <- dif - sub.
func emulSubp4(e *Engine, d *Decoded) error {
	return e.decimal4(d, func(sub, dif *big.Int) *big.Int { return new(big.Int).Sub(dif, sub) })
}

// decimal4 is the 4-operand forms: the second string is replaced by
// op(first, second).
func (e *Engine) decimal4(d *Decoded, op func(a, b *big.Int) *big.Int) error {
	ops, err := e.packedOperands(d, 0, 2)
	if err != nil {
		return err
	}

	vs, err := e.values(ops)
	if err != nil {
		return err
	}

	return e.storeDecimal(ops[1], op(vs[0], vs[1]), ops[0].addr, ops[1].addr)
}

// emulAddp6 is ADDP6 add1len, add1addr, add2len, add2addr, sumlen,
// sumaddr: sum <- add1 + add2.
func emulAddp6(e *Engine, d *Decoded) error {
	return e.decimal6(d, func(a, b *big.Int) (*big.Int, bool) { return new(big.Int).Add(a, b), true })
}

// emulSubp6 is SUBP6 sublen, subaddr, minlen, minaddr, diflen, difaddr:
// dif <- min - sub.
func emulSubp6(e *Engine, d *Decoded) error {
	return e.decimal6(d, func(sub, minValue *big.Int) (*big.Int, bool) { return new(big.Int).Sub(minValue, sub), true })
}

// emulMulp is MULP mulrlen, mulraddr, muldlen, muldaddr, prodlen,
// prodaddr: prod <- muld * mulr.
func emulMulp(e *Engine, d *Decoded) error {
	return e.decimal6(d, func(mulr, muld *big.Int) (*big.Int, bool) { return new(big.Int).Mul(muld, mulr), true })
}

// emulDivp is DIVP divrlen, divraddr, divdlen, divdaddr, quolen, quoaddr:
// quo <- divd / divr, truncated toward zero (so the remainder, which is
// lost, is smaller than the divisor); a zero quotient is positive. A zero
// divisor is the divide-by-zero trap, with nothing changed: VMS left the
// quotient, the registers, and the condition codes as they were
// (testdata/insn35). The manual's 16-byte workspace below SP is left
// alone; its contents are UNPREDICTABLE afterward anyway.
func emulDivp(e *Engine, d *Decoded) error {
	return e.decimal6(d, func(divr, divd *big.Int) (*big.Int, bool) {
		if divr.Sign() == 0 {
			return nil, false
		}

		return new(big.Int).Quo(divd, divr), true
	})
}

// decimal6 is the 6-operand forms: the third string is replaced by
// op(first, second). An op that reports false (DIVP's zero divisor) is
// the divide-by-zero trap, with nothing stored.
func (e *Engine) decimal6(d *Decoded, op func(a, b *big.Int) (*big.Int, bool)) error {
	ops, err := e.packedOperands(d, 0, 2, 4)
	if err != nil {
		return err
	}

	vs, err := e.values(ops[:2])
	if err != nil {
		return err
	}

	result, ok := op(vs[0], vs[1])
	if !ok {
		return e.arithmeticTrap(trapDivideByZero)
	}

	return e.storeDecimal(ops[2], result, ops[0].addr, ops[1].addr, ops[2].addr)
}

// emulAshp is ASHP cnt, srclen, srcaddr, round, dstlen, dstaddr: dst <-
// src * 10^cnt, cnt a signed byte. A negative count divides, rounding by
// adding bits 3:0 of round to the most significant digit discarded: if
// they reach 10, the result's magnitude goes up by one (round is normally
// 5; 0 truncates). The manual makes a round digit over 9 UNPREDICTABLE;
// VMS carried one for 5 + 12, as this does.
func emulAshp(e *Engine, d *Decoded) error {
	rawCount, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	rawRound, err := d.Operands[3].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	ops, err := e.packedOperands(d, 1, 4)
	if err != nil {
		return err
	}

	vs, err := e.values(ops[:1])
	if err != nil {
		return err
	}

	count := int(int8(rawCount))
	v := vs[0]
	magnitude := new(big.Int).Abs(v)
	ten := big.NewInt(10)

	if count >= 0 {
		magnitude.Mul(magnitude, new(big.Int).Exp(ten, big.NewInt(int64(count)), nil))
	} else {
		// The digit just below the new last one, and the shifted value.
		discarded := new(big.Int).Quo(magnitude, new(big.Int).Exp(ten, big.NewInt(int64(-count-1)), nil))
		digit := new(big.Int).Rem(discarded, ten).Int64()

		magnitude.Quo(discarded, ten)

		if digit+int64(rawRound&0xF) >= 10 {
			magnitude.Add(magnitude, big.NewInt(1))
		}
	}

	if v.Sign() < 0 {
		magnitude.Neg(magnitude)
	}

	return e.storeDecimal(ops[1], magnitude, ops[0].addr, ops[1].addr)
}
