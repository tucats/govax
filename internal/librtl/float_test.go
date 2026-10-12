package librtl

import (
	"math"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The floating tests lay their arguments out in the fixture's memory:
// two operands, an extension, an integer, a fraction or result, and a
// coefficient table, each with room for H_floating.
const (
	floatA     = 0x2000
	floatB     = 0x2010
	floatExt   = 0x2020
	floatInt   = 0x2024
	floatOut   = 0x2030
	floatTable = 0x2100
	degreeAddr = 0x2040
)

// formats are the four formats, with each one's routines' names.
var formats = []struct {
	f          vaxfloat.Format
	emod, poly string
}{
	{vaxfloat.F, "LIB$EMODF", "LIB$POLYF"},
	{vaxfloat.D, "LIB$EMODD", "LIB$POLYD"},
	{vaxfloat.G, "LIB$EMODG", "LIB$POLYG"},
	{vaxfloat.H, "LIB$EMODH", "LIB$POLYH"},
}

func putFloat(t *testing.T, env *corevms.Environment, f vaxfloat.Format, addr uint32, x float64) {
	t.Helper()

	v, err := vaxfloat.Round(f, vaxfloat.FromFloat64(x))
	if err != nil {
		t.Fatalf("%v doesn't fit format %v: %v", x, f, err)
	}

	if err := storeFloat(env, f, addr, v); err != nil {
		t.Fatal(err)
	}
}

func getFloat(t *testing.T, env *corevms.Environment, f vaxfloat.Format, addr uint32) float64 {
	t.Helper()

	v, err := loadFloat(env, f, addr)
	if err != nil {
		t.Fatalf("loadFloat(%#x): %v", addr, err)
	}

	return v.Float64()
}

// fill writes 16 bytes of 0xEE at addr, so a test can see they're left
// alone.
func fill(t *testing.T, env *corevms.Environment, addr uint32) {
	t.Helper()

	for i := range uint32(4) {
		putLongword(t, env, addr+4*i, 0xEEEEEEEE)
	}
}

// TestFloatLayout: the operands are read and written in the VAX's
// layout, the sign and exponent word first: F_floating 1.5 is the
// longword ^X000040C0.
func TestFloatLayout(t *testing.T) {
	env := fixture(t)
	putFloat(t, env, vaxfloat.F, floatA, 1.5)

	if got := longword(t, env, floatA); got != 0x000040C0 {
		t.Errorf("F 1.5 = %08X, want 000040C0", got)
	}

	if ssFltOvf == 0 || ssFltUnd == 0 || ssRopRand == 0 {
		t.Errorf("a status is missing: FLTOVF %#x, FLTUND %#x, ROPRAND %#x", ssFltOvf, ssFltUnd, ssRopRand)
	}
}

func TestLibEmod(t *testing.T) {
	for _, form := range formats {
		for _, c := range []struct {
			mulr, muld float64
			integer    int32
			fraction   float64
		}{
			{1.5, 2, 3, 0},
			{-2.5, 1.5, -3, -0.75},
			{0.75, 0.5, 0, 0.375},
			{10.25, 4, 41, 0},
		} {
			env := fixture(t)
			putFloat(t, env, form.f, floatA, c.mulr)
			putFloat(t, env, form.f, floatB, c.muld)
			putLongword(t, env, floatExt, 0)

			if got := call(t, env, form.emod, floatA, floatExt, floatB, floatInt, floatOut); got != ssNormal {
				t.Errorf("%s(%v, %v) = %#x, want SS$_NORMAL", form.emod, c.mulr, c.muld, got)
			}

			if i := int32(longword(t, env, floatInt)); i != c.integer {
				t.Errorf("%s(%v, %v): integer %d, want %d", form.emod, c.mulr, c.muld, i, c.integer)
			}

			if f := getFloat(t, env, form.f, floatOut); f != c.fraction {
				t.Errorf("%s(%v, %v): fraction %v, want %v", form.emod, c.mulr, c.muld, f, c.fraction)
			}
		}
	}
}

// TestLibEmodExtension: the extension is a byte for F and D and a word
// for G and H, and its bits take part as the instruction's do. The case
// is 1/3 times 2^20, so the extension's bits, 20 places up, land within
// the fraction's precision.
func TestLibEmodExtension(t *testing.T) {
	for _, form := range formats {
		env := fixture(t)
		third, _ := vaxfloat.Div(form.f, vaxfloat.FromInt(1), vaxfloat.FromInt(3))
		scale := vaxfloat.FromInt(1 << 20)

		if err := storeFloat(env, form.f, floatA, third); err != nil {
			t.Fatal(err)
		}

		putFloat(t, env, form.f, floatB, 1<<20)

		// The word ^X12FF: F and D take only its byte, ^XFF.
		const ext = 0x12FF
		putLongword(t, env, floatExt, ext)

		want := uint16(ext)
		if form.f == vaxfloat.F || form.f == vaxfloat.D {
			want = ext & 0xFF
		}

		integer, fraction, _, _ := cpu.ExtendedModulus(form.f, third, want, scale)
		_, plain, _, _ := cpu.ExtendedModulus(form.f, third, 0, scale)

		if fraction.Cmp(plain) == 0 {
			t.Fatalf("%s: the extension makes no difference to the test's case", form.emod)
		}

		if got := call(t, env, form.emod, floatA, floatExt, floatB, floatInt, floatOut); got != ssNormal {
			t.Errorf("%s = %#x, want SS$_NORMAL", form.emod, got)
		}

		if i := longword(t, env, floatInt); i != integer {
			t.Errorf("%s: integer %d, want %d", form.emod, i, integer)
		}

		if f, _ := loadFloat(env, form.f, floatOut); f.Cmp(fraction) != 0 {
			t.Errorf("%s: fraction %v, want %v", form.emod, f, fraction)
		}
	}
}

// TestLibEmodStatuses: integer overflow stores the low-order longword;
// underflow stores zeros; a reserved operand stores nothing.
func TestLibEmodStatuses(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, floatExt, 0)

	// D: (2^32 + 5.25) x 1 is 5, 0.25, with SS$_INTOVF.
	putFloat(t, env, vaxfloat.D, floatA, math.Ldexp(1, 32)+5.25)
	putFloat(t, env, vaxfloat.D, floatB, 1)

	if got := call(t, env, "LIB$EMODD", floatA, floatExt, floatB, floatInt, floatOut); got != ssIntOvf {
		t.Errorf("overflow: %#x, want SS$_INTOVF", got)
	}

	if i, f := longword(t, env, floatInt), getFloat(t, env, vaxfloat.D, floatOut); i != 5 || f != 0.25 {
		t.Errorf("overflow: %d, %v; want 5, 0.25", i, f)
	}

	// F: 0.5 times F's smallest value underflows; both are zero.
	putFloat(t, env, vaxfloat.F, floatA, 0.5)
	putLongword(t, env, floatB, 0x00000080) // 2^-128
	putLongword(t, env, floatInt, 0xEEEEEEEE)
	fill(t, env, floatOut)

	if got := call(t, env, "LIB$EMODF", floatA, floatExt, floatB, floatInt, floatOut); got != ssFltUnd {
		t.Errorf("underflow: %#x, want SS$_FLTUND", got)
	}

	if i, f := longword(t, env, floatInt), longword(t, env, floatOut); i != 0 || f != 0 {
		t.Errorf("underflow: %08X, %08X; want zeros", i, f)
	}

	// A reserved multiplicand: SS$_ROPRAND, signaled, as VMS does;
	// nothing stored. (This fixture has nothing to signal through, so
	// the signal fails with an error.)
	putLongword(t, env, floatB, 0x00008000)
	putLongword(t, env, floatInt, 0xEEEEEEEE)
	fill(t, env, floatOut)

	argv := []uint32{floatA, floatExt, floatB, floatInt, floatOut}
	if _, err := shim(t, env, "LIB$EMODF")(env, argv); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("reserved: err = %v, want a signal", err)
	}

	if i, f := longword(t, env, floatInt), longword(t, env, floatOut); i != 0xEEEEEEEE || f != 0xEEEEEEEE {
		t.Errorf("reserved: %08X, %08X; want both unchanged", i, f)
	}
}

// polyCall lays out a polynomial (degree len(coefficients)-1 unless
// degree is given) and calls the routine.
func polyCall(t *testing.T, env *corevms.Environment, name string, f vaxfloat.Format, x float64, degree uint16, coefficients ...float64) uint32 {
	t.Helper()

	putFloat(t, env, f, floatA, x)

	if err := env.Memory().StoreWord(env.CPU(), degreeAddr, degree); err != nil {
		t.Fatal(err)
	}

	for i, c := range coefficients {
		putFloat(t, env, f, floatTable+uint32(i*f.Size()), c)
	}

	fill(t, env, floatOut)

	return call(t, env, name, floatA, degreeAddr, floatTable, floatOut)
}

func TestLibPoly(t *testing.T) {
	for _, form := range formats {
		for _, c := range []struct {
			x            float64
			coefficients []float64
			want         float64
		}{
			// The manual's examples (LIB-303, LIB-304): x^4 + 2x^3 - x^2
			// + x - 3 at 2, and the Pascal example's coefficients 1, 2,
			// 2 at 1 (its comment calls them 2x^2 + 2x + 1; at 1 the
			// order doesn't matter).
			{2, []float64{1, 2, -1, 1, -3}, 27},
			{1, []float64{1, 2, 2}, 5},
			// Degree 0: the one coefficient.
			{5, []float64{-7.5}, -7.5},
			{0.5, []float64{4, 0, 1}, 2},
		} {
			env := fixture(t)

			got := polyCall(t, env, form.poly, form.f, c.x, uint16(len(c.coefficients)-1), c.coefficients...)
			if got != ssNormal {
				t.Errorf("%s(%v, %v) = %#x, want SS$_NORMAL", form.poly, c.x, c.coefficients, got)
			}

			if r := getFloat(t, env, form.f, floatOut); r != c.want {
				t.Errorf("%s(%v, %v) = %v, want %v", form.poly, c.x, c.coefficients, r, c.want)
			}
		}
	}
}

// TestLibPolyStatuses: a degree over 31, negative included, or a reserved
// operand is SS$_ROPRAND; overflow is SS$_FLTOVF; neither stores a
// result. An underflow makes the partial result zero and goes on.
func TestLibPolyStatuses(t *testing.T) {
	for _, c := range []struct {
		name         string
		x            float64
		degree       uint16
		coefficients []float64
		status       uint32
		want         float64 // when the status is SS$_NORMAL
	}{
		// Any degree is evaluated, as VMS does: 33 ones at x = 1.
		{"degree 32", 1, 32, ones(33), ssNormal, 33},
		{"overflow", math.Ldexp(1, 100), 2, []float64{1, 0, 0}, ssFltOvf, 0},
		{"underflow", math.Ldexp(1, -100), 3, []float64{1, 0, 0, 5}, ssNormal, 5},
		{"degree 31", 1, 31, make([]float64, 32), ssNormal, 0},
	} {
		env := fixture(t)

		got := polyCall(t, env, "LIB$POLYF", vaxfloat.F, c.x, c.degree, c.coefficients...)
		if got != c.status {
			t.Errorf("%s: %#x, want %#x", c.name, got, c.status)

			continue
		}

		if c.status != ssNormal {
			if r := longword(t, env, floatOut); r != 0xEEEEEEEE {
				t.Errorf("%s: result %08X, want it unchanged", c.name, r)
			}
		} else if r := getFloat(t, env, vaxfloat.F, floatOut); r != c.want {
			t.Errorf("%s: %v, want %v", c.name, r, c.want)
		}
	}

	// A reserved coefficient, after a good one.
	env := fixture(t)
	putFloat(t, env, vaxfloat.F, floatA, 1)
	putFloat(t, env, vaxfloat.F, floatTable, 1)
	putLongword(t, env, floatTable+4, 0x00008000)
	putLongword(t, env, degreeAddr, 1)
	fill(t, env, floatOut)

	if got := call(t, env, "LIB$POLYF", floatA, degreeAddr, floatTable, floatOut); got != ssRopRand {
		t.Errorf("reserved coefficient: %#x, want SS$_ROPRAND", got)
	}
}

// ones returns n coefficients of 1.
func ones(n int) []float64 {
	c := make([]float64, n)
	for i := range c {
		c[i] = 1
	}

	return c
}
