package cpu

import (
	"errors"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vm"
)

// The CPU's side of floating point: loading and storing floating operands,
// and turning the floating core's conditions into exceptions. The values
// themselves, their formats, and their arithmetic are internal/vaxfloat's
// (docs/PHASE-35.md, subtasks 4 and 5); this file only moves bits between
// operands and that package.
//
// An operand's format comes from its data type in the instruction table
// (Instruction.DataType), never from its size: D_floating and G_floating
// are both 8 bytes. A floating short literal or immediate operand carries
// its value as bits in its own format, exactly as if it had been read from
// memory (decodeOperand does that), so every operand loads the same way.
//
// Before Phase 35 the CPU held F and D values as Go float64s. A float64's
// fraction is 52 bits, three short of D_floating's 55, and it rounds ties
// to even where the VAX rounds them away from zero, so D values lost
// their low bits and some results rounded the wrong way
// (docs/DEVIATIONS.md).

// Arithmetic exception type codes: the parameter of an arithmetic
// exception (SCB vector ^X34), from the VAX Architecture Reference
// Manual's table of arithmetic exception types. 1 to 7 are traps (the
// instruction has completed), 8 to 10 are faults (it hasn't).
const (
	trapIntOvf  = 0x01 // integer overflow
	faultFltOvf = 0x08 // floating overflow
	faultFltDiv = 0x09 // floating divide by zero
	faultFltUnd = 0x0A // floating underflow (only when PSL<FU> is set)
)

// Integer bounds, for conversions and quotients that must fit a longword.
const (
	longMin = -2147483648
	longMax = 2147483647
)

// floatFormat returns the floating format of data type t. It panics for a
// type that isn't floating: the instruction table gives every floating
// operand a floating type, so that would be a table error.
func floatFormat(t DataType) vaxfloat.Format {
	if !t.IsFloat() {
		panic("cpu: " + t.String() + " isn't a floating data type")
	}

	return t.FloatFormat()
}

// operandFormat returns the floating format of decoded instruction d's
// operand i.
func operandFormat(d *Decoded, i int) vaxfloat.Format {
	return floatFormat(d.Instruction.DataType[i])
}

// loadFloatBits reads operand op's raw bits: 16 bytes for H_floating, 8
// or 4 otherwise.
func loadFloatBits(cpu *vax.CPU, mem *vm.Memory, op Operand) (vaxfloat.Bits, error) {
	if op.Size == 16 {
		o, err := op.LoadOctaword(cpu, mem)

		return vaxfloat.Bits{Lo: o.Lo, Hi: o.Hi}, err
	}

	raw, err := op.Load(cpu, mem)

	return vaxfloat.Bits{Lo: raw}, err
}

// loadFloat reads decoded instruction d's operand i as a floating value in
// its own format. A reserved operand is a reserved-operand fault.
func (e *Engine) loadFloat(d *Decoded, i int) (vaxfloat.Value, error) {
	bits, err := loadFloatBits(e.cpu, e.mem, d.Operands[i])
	if err != nil {
		return vaxfloat.Value{}, err
	}

	v, err := vaxfloat.Unpack(operandFormat(d, i), bits)
	if err != nil {
		return v, e.floatException(err)
	}

	return v, nil
}

// storeFloat writes v to decoded instruction d's operand i, in that
// operand's format. v should already be rounded to the format (by an
// arithmetic operation or by roundFloat); if it isn't, it's rounded here.
func (e *Engine) storeFloat(d *Decoded, i int, v vaxfloat.Value) error {
	op := d.Operands[i]

	bits, err := vaxfloat.Pack(operandFormat(d, i), v)
	if err != nil {
		if err = e.floatException(err); err != nil {
			return err
		}
	}

	if op.Size == 16 {
		return op.StoreOctaword(e.cpu, e.mem, Octaword{Lo: bits.Lo, Hi: bits.Hi})
	}

	return op.Store(e.cpu, e.mem, bits.Lo)
}

// roundFloat rounds v to decoded instruction d's operand i's format, as a
// conversion does, returning the exception the result causes, if any.
func (e *Engine) roundFloat(d *Decoded, i int, v vaxfloat.Value) (vaxfloat.Value, error) {
	r, err := vaxfloat.Round(operandFormat(d, i), v)

	return r, e.floatException(err)
}

// floatException turns a condition from the floating core into the
// exception the architecture defines, or nil if there's none: an
// underflow with PSL<FU> clear isn't an exception, and the result (which
// the core has already made zero) is stored.
func (e *Engine) floatException(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, vaxfloat.ErrReserved):
		return &Fault{Code: ExcReservedOp}
	case errors.Is(err, vaxfloat.ErrOverflow):
		return &Fault{Code: ExcArithmetic, Args: []uint32{faultFltOvf}}
	case errors.Is(err, vaxfloat.ErrDivideByZero):
		return &Fault{Code: ExcArithmetic, Args: []uint32{faultFltDiv}}
	case errors.Is(err, vaxfloat.ErrUnderflow):
		if e.cpu.PSL().FU() {
			return &Fault{Code: ExcArithmetic, Args: []uint32{faultFltUnd}}
		}

		return nil
	}

	return err
}

// arithmeticTrap returns an arithmetic trap of type code: an exception
// taken after the instruction has completed (its results stored and its
// condition codes set), so the PC saved for it is the next instruction's,
// not this one's. A handler that continues goes on from there. (A fault,
// by contrast, saves the instruction's own PC, so it runs again.)
func (e *Engine) arithmeticTrap(code uint32) error {
	e.instructionPC = e.cpu.GPR(vax.PC)

	return &Fault{Code: ExcArithmetic, Args: []uint32{code}}
}

// setFloatCC sets N and Z from v and clears V; it clears C too unless
// keepC (MOV and ACB leave C alone; the other floating instructions clear
// it).
func setFloatCC(cpu *vax.CPU, v vaxfloat.Value, keepC bool) {
	psl := cpu.PSL()
	psl.SetN(v.Sign() < 0)
	psl.SetZ(v.IsZero())
	psl.SetV(false)

	if !keepC {
		psl.SetC(false)
	}

	cpu.SetPSL(psl)
}
