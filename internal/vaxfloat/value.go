package vaxfloat

import (
	"errors"
	"math"
	"math/big"
)

// The conditions an operation can report. The CPU turns each into the
// exception the architecture defines: a reserved-operand fault, or a
// floating overflow, underflow, or divide-by-zero fault.
var (
	// ErrReserved is a reserved operand: sign 1, exponent 0.
	ErrReserved = errors.New("reserved operand")
	// ErrOverflow is a result too large for the destination format.
	ErrOverflow = errors.New("floating overflow")
	// ErrUnderflow is a nonzero result too small for the destination
	// format. The value returned with it is zero, which the CPU stores
	// unless PSL<FU> asks for the underflow fault.
	ErrUnderflow = errors.New("floating underflow")
	// ErrDivideByZero is a division by zero.
	ErrDivideByZero = errors.New("floating divide by zero")
)

// Value is a floating value: an exact number, held independently of any
// format. The zero Value is zero. A Value is never changed once made, so
// it can be copied and shared freely.
type Value struct {
	x *big.Float // nil for zero; otherwise nonzero and never modified
}

// newValue wraps x, which the caller won't change again. A zero x
// (of either sign: the VAX has no negative zero) becomes the zero Value.
func newValue(x *big.Float) Value {
	if x.Sign() == 0 {
		return Value{}
	}

	return Value{x: x}
}

// float returns v as a big.Float, which the caller must not modify.
func (v Value) float() *big.Float {
	if v.x == nil {
		return new(big.Float)
	}

	return v.x
}

// IsZero reports whether v is zero.
func (v Value) IsZero() bool { return v.x == nil }

// Sign returns -1, 0, or +1 for a negative, zero, or positive v.
func (v Value) Sign() int {
	if v.x == nil {
		return 0
	}

	return v.x.Sign()
}

// Cmp compares v and w: -1 if v < w, 0 if they're equal, +1 if v > w.
func (v Value) Cmp(w Value) int { return v.float().Cmp(w.float()) }

// Neg returns -v.
func (v Value) Neg() Value {
	if v.x == nil {
		return v
	}

	return Value{x: new(big.Float).Neg(v.x)}
}

// Abs returns |v|.
func (v Value) Abs() Value {
	if v.Sign() < 0 {
		return v.Neg()
	}

	return v
}

// Float64 returns v as the nearest float64, for display and for callers
// that only need an approximation. (H_floating's exponent range is wider
// than a float64's, so a large H value can come back as an infinity.)
func (v Value) Float64() float64 {
	f, _ := v.float().Float64()

	return f
}

// Decimal returns v in plain decimal notation (never an exponent), with
// the fewest digits that identify v among the values of the format it was
// unpacked from or rounded to: Parse of the result, rounded to that format,
// gives v back. For a disassembler, whose text must reassemble to the
// same bits.
func (v Value) Decimal() string {
	if v.x == nil {
		return "0"
	}

	return v.x.Text('f', -1)
}

// String returns v in decimal, with enough digits to show an H value.
func (v Value) String() string { return v.float().Text('g', 36) }

// FromFloat64 returns x as a Value, exactly. It panics on an infinity or
// NaN, which no VAX value is.
func FromFloat64(x float64) Value {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		panic("vaxfloat: FromFloat64 of an infinity or NaN")
	}

	return newValue(new(big.Float).SetFloat64(x))
}

// FromInt returns i as a Value, exactly.
func FromInt(i int64) Value { return newValue(new(big.Float).SetInt64(i)) }

// FromBigInt returns i as a Value, exactly.
func FromBigInt(i *big.Int) Value {
	return newValue(new(big.Float).SetPrec(uint(max(i.BitLen(), 1))).SetInt(i))
}

// parsePrecision is the precision Parse reads a decimal number at, before
// a caller rounds it to a format. It's far beyond H_floating's 113 bits,
// so rounding twice (to this, then to the format) can't change a result
// except for a number constructed to sit within 2^-1000 of a tie.
const parsePrecision = 1024

// Parse reads a decimal floating-point number ("1.5", "-2.5E-3"), as
// MACRO's floating literals are written. The result is exact to
// parsePrecision bits; round it to a format with Round.
func Parse(s string) (Value, error) {
	x, _, err := big.ParseFloat(s, 10, parsePrecision, big.ToNearestEven)
	if err != nil {
		return Value{}, err
	}

	return newValue(x), nil
}

// Unpack returns the value of b, in format f. A reserved operand returns
// ErrReserved. An exponent of zero with the sign clear is zero, whatever
// the fraction bits hold.
func Unpack(f Format, b Bits) (Value, error) {
	l := f.layout()
	hi, lo := f.logical(b)

	sign := bit128(hi, lo, f.bits()-1)
	_, e := shr128(hi, lo, l.fbits)
	exp := int(e & (1<<l.ebits - 1))

	if exp == 0 {
		if sign {
			return Value{}, ErrReserved
		}

		return Value{}, nil
	}

	// The significand is the fraction with its hidden bit, an integer
	// of f.Precision() bits: the value is it times 2^(exp - bias - p).
	mhi, mlo := mask128(l.fbits)
	fhi, flo := hi&mhi, lo&mlo
	hhi, hlo := shl128(0, 1, l.fbits)
	fhi, flo = or128(fhi, flo, hhi, hlo)

	m := new(big.Int).SetUint64(fhi)
	m.Lsh(m, 64)
	m.Or(m, new(big.Int).SetUint64(flo))

	p := f.Precision()
	x := new(big.Float).SetPrec(p).SetInt(m)
	x.SetMantExp(x, exp-l.bias-int(p))

	if sign {
		x.Neg(x)
	}

	return Value{x: x}, nil
}

func bit128(hi, lo uint64, n int) bool {
	if n >= 64 {
		return hi>>(n-64)&1 != 0
	}

	return lo>>n&1 != 0
}

// Round rounds v to format f's precision, half away from zero, and checks
// the result's exponent against the format's range. It returns ErrOverflow
// for a result too large (and the zero Value), or ErrUnderflow and zero
// for a nonzero result too small.
func Round(f Format, v Value) (Value, error) {
	if v.x == nil {
		return v, nil
	}

	x := new(big.Float).SetMode(big.ToNearestAway).SetPrec(f.Precision()).Set(v.x)

	return check(f, x)
}

// check returns x, already rounded to format f, if its exponent is in f's
// range.
func check(f Format, x *big.Float) (Value, error) {
	if x.Sign() == 0 {
		return Value{}, nil
	}

	e := x.MantExp(nil) // x = m * 2^e, 0.5 <= |m| < 1

	switch {
	case e > f.maxExp():
		return Value{}, ErrOverflow
	case e < f.minExp():
		return Value{}, ErrUnderflow
	}

	return Value{x: x}, nil
}

// Pack rounds v to format f (as Round does) and returns its bits. On
// ErrOverflow it returns zero bits; on ErrUnderflow, the bits of zero,
// which the CPU stores when PSL<FU> is clear.
func Pack(f Format, v Value) (Bits, error) {
	r, err := Round(f, v)
	if err != nil {
		return Bits{}, err
	}

	return pack(f, r), nil
}

// pack lays out v, already rounded to f and in its range.
func pack(f Format, v Value) Bits {
	if v.x == nil {
		return Bits{}
	}

	l := f.layout()
	p := f.Precision()

	m := new(big.Float)
	e := v.x.MantExp(m) // v = m * 2^e
	m.Abs(m)
	m.SetMantExp(m, int(p)) // the significand as an integer of p bits

	sig, _ := m.Int(nil)

	// Drop the hidden bit.
	sig.SetBit(sig, int(p)-1, 0)

	lo := sig.Uint64()
	hi := new(big.Int).Rsh(sig, 64).Uint64()

	eh, el := shl128(0, uint64(e+l.bias), l.fbits)
	hi, lo = or128(hi, lo, eh, el)

	if v.x.Sign() < 0 {
		sh, sl := shl128(0, 1, f.bits()-1)
		hi, lo = or128(hi, lo, sh, sl)
	}

	return f.memory(hi, lo)
}

// Zero returns the bits of zero, which are all zero in every format.
func Zero() Bits { return Bits{} }

// Reserved returns format f's canonical reserved operand: the sign set and
// every other bit clear.
func Reserved(f Format) Bits {
	hi, lo := shl128(0, 1, f.bits()-1)

	return f.memory(hi, lo)
}

// ---------------------------------------------------------------------
// Arithmetic. Each operation rounds its exact result once, to format f.

// operate returns op's result on a and b, rounded to f.
func operate(f Format, op func(z, a, b *big.Float) *big.Float, a, b Value) (Value, error) {
	z := new(big.Float).SetMode(big.ToNearestAway).SetPrec(f.Precision())
	op(z, a.float(), b.float())

	return check(f, z)
}

// Add returns a + b, rounded to f.
func Add(f Format, a, b Value) (Value, error) { return operate(f, (*big.Float).Add, a, b) }

// Sub returns a - b, rounded to f.
func Sub(f Format, a, b Value) (Value, error) { return operate(f, (*big.Float).Sub, a, b) }

// Mul returns a * b, rounded to f.
func Mul(f Format, a, b Value) (Value, error) { return operate(f, (*big.Float).Mul, a, b) }

// Div returns a / b, rounded to f, or ErrDivideByZero if b is zero.
func Div(f Format, a, b Value) (Value, error) {
	if b.IsZero() {
		return Value{}, ErrDivideByZero
	}

	return operate(f, (*big.Float).Quo, a, b)
}

// MulExact returns a * b, exactly (with as many bits as the product
// needs): for EMOD's extended product and POLY's steps, which round in
// their own ways.
func MulExact(a, b Value) Value {
	if a.IsZero() || b.IsZero() {
		return Value{}
	}

	z := new(big.Float).SetPrec(a.x.MinPrec() + b.x.MinPrec())

	return newValue(z.Mul(a.x, b.x))
}

// AddExact returns a + b, exactly.
func AddExact(a, b Value) Value {
	if a.IsZero() {
		return b
	}

	if b.IsZero() {
		return a
	}

	z := new(big.Float).SetPrec(exactSumPrecision(a.x, b.x))

	return newValue(z.Add(a.x, b.x))
}

// exactSumPrecision returns a precision that holds a + b exactly: enough
// bits to span from the larger operand's top bit to the smaller one's
// bottom bit, plus a carry.
func exactSumPrecision(a, b *big.Float) uint {
	top := max(a.MantExp(nil), b.MantExp(nil))
	bottom := min(a.MantExp(nil)-int(a.MinPrec()), b.MantExp(nil)-int(b.MinPrec()))

	return uint(top-bottom) + 1
}

// ---------------------------------------------------------------------
// Integers.

// Int returns v as an integer: truncated toward zero, or, with round,
// rounded to the nearest integer, half away from zero (CVTRxL's rounding).
// The result is exact, however large; the caller checks it against the
// destination's range.
func (v Value) Int(round bool) *big.Int {
	if v.x == nil {
		return new(big.Int)
	}

	x := v.x

	if round {
		half := new(big.Float).SetFloat64(0.5)
		if x.Sign() < 0 {
			half.Neg(half)
		}

		x = new(big.Float).SetPrec(exactSumPrecision(x, half)).Add(x, half)
	}

	i, _ := x.Int(nil) // truncates toward zero

	return i
}

// Split returns v's integer part, truncated toward zero, and its
// fraction part, v minus the integer part (with v's sign, and exact), as
// EMOD separates them.
func (v Value) Split() (*big.Int, Value) {
	i := v.Int(false)

	return i, AddExact(v, FromBigInt(i).Neg())
}

// ShortLiteral returns the value of a floating short literal: a 6-bit
// operand specifier's literal field, exponent in bits 5:3 and fraction in
// bits 2:0, meaning (1 + fraction/8) * 2^(exponent-1). The 64 values run
// from 0.5 to 120. The value is the same in every format; only its bits
// differ (Pack gives them).
func ShortLiteral(lit byte) Value {
	e := int(lit >> 3 & 7)
	frac := int64(lit & 7)
	x := new(big.Float).SetInt64(8 + frac)

	return newValue(x.SetMantExp(x, e-4))
}

// FindShortLiteral returns the short literal whose value is v, if there is
// one.
func FindShortLiteral(v Value) (byte, bool) {
	for lit := byte(0); lit < 64; lit++ {
		if ShortLiteral(lit).Cmp(v) == 0 {
			return lit, true
		}
	}

	return 0, false
}
