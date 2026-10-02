package vaxfloat

import (
	"encoding/hex"
	"errors"
	"math/big"
	"testing"
)

// bitsOf returns the Bits of an operand's bytes, as stored in memory.
func bitsOf(t *testing.T, s string) Bits {
	t.Helper()

	raw, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	var b Bits

	for i, c := range raw {
		if i < 8 {
			b.Lo |= uint64(c) << (8 * i)
		} else {
			b.Hi |= uint64(c) << (8 * (i - 8))
		}
	}

	return b
}

// bytesOf returns b's first size bytes, in memory order, as hex.
func bytesOf(b Bits, size int) string {
	raw := make([]byte, size)

	for i := range raw {
		if i < 8 {
			raw[i] = byte(b.Lo >> (8 * i))
		} else {
			raw[i] = byte(b.Hi >> (8 * (i - 8)))
		}
	}

	return hex.EncodeToString(raw)
}

var formats = map[string]Format{"F": F, "D": D, "G": G, "H": H}

var intSizes = map[string]int{"B": 1, "W": 2, "L": 4}

// TestVMSVectors checks the core against VMS's results for the probes'
// floating instructions (vms_vectors_test.go): each operation's result
// bits, or its condition. A result that underflowed with PSL<FU> clear is
// zero on VMS; the core reports ErrUnderflow, with zero, for the CPU to
// store.
func TestVMSVectors(t *testing.T) {
	for _, v := range vmsVectors {
		got, gotV, err := runVector(t, v.op, v.dst, v.src, v.a, v.b)

		if errors.Is(err, ErrUnderflow) && v.err == nil {
			// VMS stored zero (FU clear).
			err, got = nil, bytesOf(Bits{}, sizeOf(v.dst))
		}

		if !errors.Is(err, v.err) && (err != nil || v.err != nil) {
			t.Errorf("%s %d %s: error %v, VMS %v", v.probe, v.n, v.name, err, v.err)

			continue
		}

		if v.err != nil {
			continue
		}

		if got != v.want || gotV != v.v {
			t.Errorf("%s %d %s: got %s (V %v), VMS %s (V %v)", v.probe, v.n, v.name, got, gotV, v.want, v.v)
		}
	}
}

func sizeOf(format string) int {
	if f, ok := formats[format]; ok {
		return f.Size()
	}

	return intSizes[format]
}

// runVector runs one vector's operation, returning the result's bytes and
// whether it overflowed an integer destination (V).
func runVector(t *testing.T, op, dst, src, a, b string) (string, bool, error) {
	t.Helper()

	switch op {
	case "FromIntB", "FromIntW", "FromIntL":
		raw := bitsOf(t, a).Lo
		size := intSizes[src]
		i := int64(raw<<(64-8*size)) >> (64 - 8*size) // sign-extend

		bits, err := Pack(formats[dst], FromInt(i))

		return bytesOf(bits, sizeOf(dst)), false, err
	}

	x, err := Unpack(formats[src], bitsOf(t, a))
	if err != nil {
		return "", false, err
	}

	var result Value

	switch op {
	case "Mov", "Cvt":
		result = x
	case "Neg":
		result = x.Neg()
	case "ToInt", "ToIntR":
		i := x.Int(op == "ToIntR")
		size := intSizes[dst]
		limit := new(big.Int).Lsh(big.NewInt(1), uint(8*size-1))
		overflow := i.Cmp(limit) >= 0 || i.Cmp(new(big.Int).Neg(limit)) < 0

		// The low-order bits of the two's complement integer.
		low := new(big.Int).And(i, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(8*size)), big.NewInt(1)))

		return bytesOf(Bits{Lo: low.Uint64()}, size), overflow, nil
	default:
		y, err := Unpack(formats[src], bitsOf(t, b))
		if err != nil {
			return "", false, err
		}

		f := formats[dst]

		switch op {
		case "Add":
			result, err = Add(f, x, y)
		case "Sub":
			result, err = Sub(f, x, y)
		case "Mul":
			result, err = Mul(f, x, y)
		case "Div":
			result, err = Div(f, x, y)
		default:
			t.Fatalf("unknown op %s", op)
		}

		if err != nil {
			return "", false, err
		}
	}

	bits, err := Pack(formats[dst], result)

	return bytesOf(bits, sizeOf(dst)), false, err
}

// TestLayouts checks values whose bits are worked out by hand from the
// manual's format definitions.
func TestLayouts(t *testing.T) {
	for _, tc := range []struct {
		f     Format
		value string
		bytes string // memory order
	}{
		// 1.5 = 0.11 * 2^1: exponent 1+bias, fraction .1 after the
		// hidden bit.
		{F, "1.5", "c0400000"},                         // word ^X40C0
		{D, "1.5", "c040000000000000"},                 // ^X40C0, 0, 0, 0
		{G, "1.5", "1840000000000000"},                 // ^X4018: exponent ^X401, fraction ^X8...
		{H, "1.5", "01400080000000000000000000000000"}, // ^X4001, ^X8000
		{F, "-0.5", "00c00000"},                        // ^XC000: sign, exponent ^X80
		{F, "0.5", "00400000"},
		{G, "0.5", "0040000000000000"},                 // ^X4000: exponent ^X400
		{H, "0.5", "00400000000000000000000000000000"}, // ^X4000: exponent ^X4000
		// 1/3 rounds up in D: ^X3FAA, ^XAAAA, ^XAAAA, ^XAAAB.
		{D, "0.333333333333333333333333333333333333", "aa3faaaaaaaaabaa"},
	} {
		v, err := Parse(tc.value)
		if err != nil {
			t.Fatal(err)
		}

		b, err := Pack(tc.f, v)
		if err != nil {
			t.Fatalf("%v %s: %v", tc.f, tc.value, err)
		}

		if got := bytesOf(b, tc.f.Size()); got != tc.bytes {
			t.Errorf("%v %s = %s, want %s", tc.f, tc.value, got, tc.bytes)
		}

		back, err := Unpack(tc.f, b)
		if err != nil {
			t.Fatal(err)
		}

		if again, _ := Pack(tc.f, back); again != b {
			t.Errorf("%v %s: unpack and pack gave %s", tc.f, tc.value, bytesOf(again, tc.f.Size()))
		}
	}
}

// TestReservedAndZero checks the exponent-0 encodings: zero with the sign
// clear (whatever the fraction), a reserved operand with it set.
func TestReservedAndZero(t *testing.T) {
	for _, f := range []Format{F, D, G, H} {
		if _, err := Unpack(f, Reserved(f)); !errors.Is(err, ErrReserved) {
			t.Errorf("%v: Unpack(Reserved) error %v, want ErrReserved", f, err)
		}

		// The sign bit is bit 15 of the first word in every format.
		if got := Reserved(f).Lo & 0xFFFF; got != 0x8000 {
			t.Errorf("%v: Reserved's first word %#x, want 0x8000", f, got)
		}

		// A "dirty zero": exponent 0, sign clear, fraction bits set (in
		// the second and later words, below every format's exponent).
		dirty := Bits{Lo: 0x1234_0001_0000}
		if v, err := Unpack(f, dirty); err != nil || !v.IsZero() {
			t.Errorf("%v: dirty zero = %v, %v; want zero", f, v, err)
		}

		if b, err := Pack(f, Value{}); err != nil || b != (Bits{}) {
			t.Errorf("%v: Pack(0) = %+v, %v", f, b, err)
		}
	}
}

// TestLimits checks each format's largest and smallest values pack, and
// that just beyond them overflows or underflows.
func TestLimits(t *testing.T) {
	for _, f := range []Format{F, D, G, H} {
		p := int(f.Precision())

		// max = (1 - 2^-p) * 2^maxExp: every bit set but the sign.
		one := new(big.Float).SetPrec(uint(p) + 1).SetInt64(1)
		ulp := new(big.Float).SetMantExp(big.NewFloat(1), -p)
		maxv := new(big.Float).SetPrec(uint(p)+1).Sub(one, ulp)
		maxv.SetMantExp(maxv, f.maxExp())

		b, err := Pack(f, newValue(maxv))
		if err != nil {
			t.Fatalf("%v max: %v", f, err)
		}

		words := f.Size() / 2
		for i := 0; i < words; i++ {
			want := uint16(0xFFFF)
			if i == 0 {
				want = 0x7FFF
			}

			if got := word(b, i); got != want {
				t.Errorf("%v max: word %d = %#x, want %#x", f, i, got, want)
			}
		}

		// max * 2 overflows; max + half an ulp rounds up and overflows.
		if _, err := Mul(f, newValue(maxv), FromInt(2)); !errors.Is(err, ErrOverflow) {
			t.Errorf("%v max*2: %v, want ErrOverflow", f, err)
		}

		halfULP := new(big.Float).SetMantExp(big.NewFloat(1), f.maxExp()-p-1)
		if _, err := Add(f, newValue(maxv), newValue(halfULP)); !errors.Is(err, ErrOverflow) {
			t.Errorf("%v max+half ulp: %v, want ErrOverflow", f, err)
		}

		// min = 0.5 * 2^minExp; half of it underflows.
		minv := newValue(new(big.Float).SetMantExp(big.NewFloat(0.5), f.minExp()))

		b, err = Pack(f, minv)
		if err != nil {
			t.Fatalf("%v min: %v", f, err)
		}

		// Exponent field 1, fraction 0: the first word is the exponent's
		// 1 at the bottom of its field.
		if got, want := word(b, 0), uint16(1)<<(f.layout().fbits%16); got != want {
			t.Errorf("%v min: first word %#x, want %#x", f, got, want)
		}

		if v, err := Mul(f, minv, FromFloat64(0.5)); !errors.Is(err, ErrUnderflow) || !v.IsZero() {
			t.Errorf("%v min/2: %v, %v; want zero and ErrUnderflow", f, v, err)
		}
	}
}

// TestRoundingTies checks a tie rounds away from zero in each format: 1 +
// half an ulp rounds up to 1 + 1 ulp, and 1 + 3/2 ulp to 1 + 2 ulps (even
// rounding would give 1 and 1 + 2 ulps).
func TestRoundingTies(t *testing.T) {
	for _, f := range []Format{F, D, G, H} {
		p := int(f.Precision())
		ulp := newValue(new(big.Float).SetMantExp(big.NewFloat(1), 1-p)) // at 1.0
		half := newValue(new(big.Float).SetMantExp(big.NewFloat(1), -p))

		got, err := Add(f, FromInt(1), half)
		if err != nil {
			t.Fatal(err)
		}

		if want := AddExact(FromInt(1), ulp); got.Cmp(want) != 0 {
			t.Errorf("%v: 1 + half ulp = %v, want %v", f, got, want)
		}

		got, err = Sub(f, FromInt(-1), half)
		if err != nil {
			t.Fatal(err)
		}

		if want := AddExact(FromInt(-1), ulp.Neg()); got.Cmp(want) != 0 {
			t.Errorf("%v: -1 - half ulp = %v, want %v", f, got, want)
		}
	}
}

func TestShortLiterals(t *testing.T) {
	for _, tc := range []struct {
		lit  byte
		want float64
	}{
		{0, 0.5}, {7, 0.9375}, {8, 1}, {0o14, 1.5}, {0o77, 120},
	} {
		if got := ShortLiteral(tc.lit).Float64(); got != tc.want {
			t.Errorf("literal %#o = %v, want %v", tc.lit, got, tc.want)
		}

		if lit, ok := FindShortLiteral(FromFloat64(tc.want)); !ok || lit != tc.lit {
			t.Errorf("FindShortLiteral(%v) = %#o, %v", tc.want, lit, ok)
		}
	}

	if _, ok := FindShortLiteral(FromFloat64(1.1)); ok {
		t.Error("1.1 has no short literal")
	}

	// The same literal has different bits in each format: 0.5's exponent
	// field is the format's bias.
	for f, want := range map[Format]uint64{F: 0x4000, D: 0x4000, G: 0x4000, H: 0x4000} {
		b, _ := Pack(f, ShortLiteral(0))
		if b.Lo&0xFFFF != want {
			t.Errorf("%v literal 0.5: first word %#x", f, b.Lo&0xFFFF)
		}
	}

	b, _ := Pack(G, ShortLiteral(0o14)) // 1.5
	if b.Lo != 0x4018 {
		t.Errorf("G literal 1.5 = %#x, want 0x4018", b.Lo)
	}
}

func TestIntAndSplit(t *testing.T) {
	for _, tc := range []struct {
		v          float64
		trunc, rnd int64
		frac       float64
	}{
		{2.5, 2, 3, 0.5},
		{-2.5, -2, -3, -0.5},
		{2.4, 2, 2, 0.3999999999999999},
		{-0.5, 0, -1, -0.5},
		{0.25, 0, 0, 0.25},
		{7, 7, 7, 0},
	} {
		v := FromFloat64(tc.v)

		if got := v.Int(false).Int64(); got != tc.trunc {
			t.Errorf("Int(%v) = %d, want %d", tc.v, got, tc.trunc)
		}

		if got := v.Int(true).Int64(); got != tc.rnd {
			t.Errorf("Int(%v, round) = %d, want %d", tc.v, got, tc.rnd)
		}

		i, frac := v.Split()
		if i.Int64() != tc.trunc || frac.Float64() != tc.frac {
			t.Errorf("Split(%v) = %d, %v; want %d, %v", tc.v, i, frac, tc.trunc, tc.frac)
		}
	}

	// A large H value's integer part is exact.
	big2000 := newValue(new(big.Float).SetMantExp(big.NewFloat(1), 2000))
	if got := big2000.Int(true).BitLen(); got != 2001 {
		t.Errorf("Int(2^2000) has %d bits, want 2001", got)
	}
}

func TestDivideByZero(t *testing.T) {
	if _, err := Div(F, FromInt(1), Value{}); !errors.Is(err, ErrDivideByZero) {
		t.Errorf("1/0: %v, want ErrDivideByZero", err)
	}
}

func TestNoNegativeZero(t *testing.T) {
	v, err := Sub(D, FromFloat64(1.5), FromFloat64(1.5))
	if err != nil || !v.IsZero() {
		t.Fatalf("1.5-1.5 = %v, %v", v, err)
	}

	if b, _ := Pack(D, FromInt(0).Neg()); b != (Bits{}) {
		t.Errorf("-0 packed as %+v, want zero bits", b)
	}
}

// BenchmarkAddF and the others measure one floating operation as the CPU
// will run it: unpack both operands, operate, pack (Decision 2's
// benchmark: big.Float allocates on every operation).
func BenchmarkAddF(b *testing.B) { benchmarkOp(b, F, Add) }
func BenchmarkMulD(b *testing.B) { benchmarkOp(b, D, Mul) }
func BenchmarkDivG(b *testing.B) { benchmarkOp(b, G, Div) }
func BenchmarkMulH(b *testing.B) { benchmarkOp(b, H, Mul) }

func benchmarkOp(b *testing.B, f Format, op func(Format, Value, Value) (Value, error)) {
	x, _ := Pack(f, FromFloat64(1.0/3))
	y, _ := Pack(f, FromFloat64(2.718281828459045))

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		u, _ := Unpack(f, x)
		v, _ := Unpack(f, y)
		r, _ := op(f, u, v)

		if _, err := Pack(f, r); err != nil {
			b.Fatal(err)
		}
	}
}
