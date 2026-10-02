package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vaxfloat"
)

// POLYF, POLYD, POLYG, and POLYH: Polynomial Evaluation.
//
//	POLYx arg.rx, degree.rw, tbladdr.ab
//
// The table holds degree+1 coefficients of the argument's format, the
// highest-order one first. Following the manual's Operation: a degree
// over 31 is a reserved operand; the partial result starts as the first
// coefficient; then each further coefficient takes a step,
// vaxfloat.POLYStep (multiply by the argument and add, each truncated,
// then rounded). Overflow after a step is a fault; underflow is a fault
// with PSL<FU> set, and otherwise makes the partial result zero, and the
// evaluation goes on.
//
// The result goes in R0 (POLYF), R0-R1 (POLYD, POLYG), or R0-R3 (POLYH),
// and the registers the manual lists are set: the address just past the
// table in R3 (R5 for POLYH), and zero in the others up to R3 (POLYF) or
// R5. N and Z come from the result, V and C are cleared. A fault leaves
// every register unchanged (govax doesn't model PSL<FPD>, so a POLY that
// faults part way starts again from the beginning).
//
// POLYH keeps a copy of its argument in the 16 bytes below SP, "in case
// the instruction is interrupted"; govax writes it there too, though it
// never resumes from it.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x55, emulPoly)   // POLYF
	reg(0x75, emulPoly)   // POLYD
	regFD(0x55, emulPoly) // POLYG
	regFD(0x75, emulPoly) // POLYH
}

// polyMaxDegree is the largest degree POLY accepts.
const polyMaxDegree = 31

func emulPoly(e *Engine, d *Decoded) error {
	f := operandFormat(d, 0)
	size := uint32(f.Size())

	arg, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	raw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	degree := uint16(raw)
	if degree > polyMaxDegree {
		return &Fault{Code: ExcReservedOp}
	}

	table := d.Operands[2].Addr

	acc, err := e.loadCoefficient(f, table)
	if err != nil {
		return err
	}

	table += size

	if f == vaxfloat.H {
		bits, _ := vaxfloat.Pack(f, arg)
		sp := e.cpu.GPR(vax.SP)

		if err := e.mem.StoreQuadword(e.cpu, sp-16, bits.Lo); err != nil {
			return err
		}

		if err := e.mem.StoreQuadword(e.cpu, sp-8, bits.Hi); err != nil {
			return err
		}
	}

	for ; degree > 0; degree-- {
		coef, err := e.loadCoefficient(f, table)
		if err != nil {
			return err
		}

		acc, err = vaxfloat.POLYStep(f, arg, acc, coef)
		if err = e.floatException(err); err != nil {
			return err
		}

		table += size
	}

	bits, _ := vaxfloat.Pack(f, acc)
	longwords := []uint32{uint32(bits.Lo), uint32(bits.Lo >> 32), uint32(bits.Hi), uint32(bits.Hi >> 32)}

	// R0 up: the result's longwords, then zeros, with the table address
	// in R3 (R5 for POLYH).
	results := make([]uint32, 4)
	copy(results, longwords[:size/4])

	switch f {
	case vaxfloat.F:
		results[3] = table
	case vaxfloat.H:
		results = append(results, 0, table)
	default:
		results[3] = table
		results = append(results, 0, 0)
	}

	for i, v := range results {
		e.cpu.SetGPR(vax.R0+vax.Reg(i), v)
	}

	setFloatCC(e.cpu, acc, false)

	return nil
}

// loadCoefficient reads the format f coefficient at addr.
func (e *Engine) loadCoefficient(f vaxfloat.Format, addr uint32) (vaxfloat.Value, error) {
	var bits vaxfloat.Bits

	switch f.Size() {
	case 4:
		l, err := e.mem.LoadLongword(e.cpu, addr)
		if err != nil {
			return vaxfloat.Value{}, err
		}

		bits.Lo = uint64(l)
	default:
		q, err := e.mem.LoadQuadword(e.cpu, addr)
		if err != nil {
			return vaxfloat.Value{}, err
		}

		bits.Lo = q

		if f.Size() == 16 {
			if bits.Hi, err = e.mem.LoadQuadword(e.cpu, addr+8); err != nil {
				return vaxfloat.Value{}, err
			}
		}
	}

	v, err := vaxfloat.Unpack(f, bits)
	if err != nil {
		return v, e.floatException(err)
	}

	return v, nil
}
