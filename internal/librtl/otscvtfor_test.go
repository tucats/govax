package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/vaxfloat"
)

// TestOTSCvtForDigits: OTS$$CVT_x_T_R8 in a frame laid out as FORRTL's
// (the fields testdata/ots72's study saw at the JSB), against what VMS
// 7.1's routine wrote there (vax/ots72t-vax.txt): the digits from the
// first one on, as far as VMS's go (its buffer holds fewer than govax's),
// the exponent, the sign, and rounding in place, a carry into the guard
// digit among them.
func TestOTSCvtForDigits(t *testing.T) {
	const (
		fp     = 0x4000
		buffer = 0x3F00
		value  = 0x3E00
	)

	cases := []struct {
		routine string
		f       vaxfloat.Format
		value   string
		kind    byte
		size    uint32 // R2
		sig     uint32 // FP-16
		frac    uint32 // FP-36
		digits  string // VMS's, from the first one
		exp     int32
		sign    int32
	}{
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "3.14159", cvtKindF, 0x26, 0x13, 4, "3141689999999999993640642514", 1, 1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "3.14159", cvtKindF, 0x26, 0x13, 9, "3141590000999999993640642514", 1, 1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "-3.14159", cvtKindF, 0x26, 0x13, 4, "3141689999999999993640642514", 1, -1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "1234567.875", cvtKindF, 0x26, 0x13, 2, "1234567885000000000000000", 7, 1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "0.000123456", cvtKindF, 0x26, 0x13, 8, "123465999999999998411378", -3, 1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "1.0E20", cvtKindF, 0x30, 0x1D, 2, "100000000000000000000000000000", 21, 1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "3.14159", cvtKindE, 0x17, 4, 0, "3142589999", 1, 1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "-3.14159E-10", cvtKindE, 0x19, 6, 0, "314159000", -9, -1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "3.14159", cvtKindG, 0x23, 0x10, 0, "3141589999999999993", 1, 1},
		{"OTS$$CVT_D_T_R8", vaxfloat.D, "314159.0", cvtKindG, 0x23, 0x10, 0, "314159000000000000000000", 6, 1},
		{"OTS$$CVT_F_T_R8", vaxfloat.F, "3.14159", cvtKindF, 0x26, 0x13, 4, "3141690118408203125000000000", 1, 1},
		{"OTS$$CVT_G_T_R8", vaxfloat.G, "1.0E200", cvtKindE, 0x17, 4, 0, "1000099999", 201, 1}, // carried into the guard
		{"OTS$$CVT_G_T_R8", vaxfloat.G, "3.14159", cvtKindF, 0x26, 0x13, 4, "314168999999999988261834005", 1, 1},
		{"OTS$$CVT_H_T_R8", vaxfloat.H, "3.14159", cvtKindF, 0x26, 0x13, 4, "314169000000000000000000000", 1, 1},
	}

	for _, c := range cases {
		env := fixture(t)
		putDecimal(t, env, c.f, value, c.value)

		for a := uint32(fp - 64); a < fp; a += 4 {
			putLongword(t, env, a, 0)
		}

		putLongword(t, env, fp-12, uint32(c.kind)<<24)
		putLongword(t, env, fp-16, c.sig)
		putLongword(t, env, fp-20, buffer)
		putLongword(t, env, fp-32, 1)
		putLongword(t, env, fp-36, c.frac)

		callJSB(t, env, c.routine, value, fp, c.size)

		first := longword(t, env, fp-32)
		got := bytesAt(t, env, buffer+first, len(c.digits))

		if got != c.digits || int32(longword(t, env, fp-28)) != c.exp || int32(longword(t, env, fp-24)) != c.sign {
			t.Errorf("%s(%s, kind %d): digits %q, exponent %d, sign %d; want %q, %d, %d", c.routine, c.value, c.kind,
				got, int32(longword(t, env, fp-28)), int32(longword(t, env, fp-24)), c.digits, c.exp, c.sign)
		}

		if first > 0 && bytesAt(t, env, buffer+first-1, 1) != "0" {
			t.Errorf("%s(%s): the digit before the first is %q, want the guard 0", c.routine, c.value, bytesAt(t, env, buffer+first-1, 1))
		}
	}

	// Zero: zeros, the first digit at 0, sign 0, the exponent left alone.
	env := fixture(t)
	putDecimal(t, env, vaxfloat.D, value, "0")
	putLongword(t, env, fp-12, cvtKindF<<24)
	putLongword(t, env, fp-20, buffer)
	putLongword(t, env, fp-24, 7)
	putLongword(t, env, fp-28, 7)
	putLongword(t, env, fp-32, 7)
	putLongword(t, env, fp-36, 4)

	callJSB(t, env, "OTS$$CVT_D_T_R8", value, fp, 0x26)

	if got := bytesAt(t, env, buffer, 0x25); got != "0000000000000000000000000000000000000" ||
		longword(t, env, fp-32) != 0 || longword(t, env, fp-24) != 0 || longword(t, env, fp-28) != 7 {
		t.Errorf("zero: %q, first %d, sign %d, exponent %d", got, longword(t, env, fp-32), longword(t, env, fp-24), longword(t, env, fp-28))
	}
}
