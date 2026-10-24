package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
)

// floatBits reads format f's bits at addr.
func floatBits(t *testing.T, env *corevms.Environment, f vaxfloat.Format, addr uint32) vaxfloat.Bits {
	t.Helper()

	v, err := loadFloat(env, f, addr)
	if err != nil {
		t.Fatal(err)
	}

	b, _ := vaxfloat.Pack(f, v)

	return b
}

// decimalBits is the bits of the decimal number s rounded to format f.
func decimalBits(t *testing.T, f vaxfloat.Format, s string) vaxfloat.Bits {
	t.Helper()

	v, err := vaxfloat.Parse(s)
	if err != nil {
		t.Fatal(err)
	}

	b, err := vaxfloat.Pack(f, v)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// TestOTSCnvout: OTS$CNVOUT, _G, and _H write FORTRAN's E format,
// right-justified: rounding, negative values, zero, the exponent's three
// digits without the E, the leading zero left out when only that fits,
// and OTS$_OUTCONERR's asterisks.
func TestOTSCnvout(t *testing.T) {
	env := fixture(t)

	cases := []struct {
		routine string
		f       vaxfloat.Format
		value   string
		width   int
		digits  uint32
		want    string
		sts     uint32
	}{
		{"OTS$CNVOUT", vaxfloat.D, "3.14159", 12, 4, "  0.3142E+01", ssNormal},
		{"OTS$CNVOUT", vaxfloat.D, "-0.0012345", 11, 3, " -0.123E-02", ssNormal},
		{"OTS$CNVOUT", vaxfloat.D, "0", 8, 2, "0.00E+00", ssNormal},
		{"OTS$CNVOUT", vaxfloat.D, "9.9996", 10, 3, " 0.100E+02", ssNormal}, // rounds up a decade
		{"OTS$CNVOUT", vaxfloat.D, "0.5", 7, 0, " 0.E+01", ssNormal},
		{"OTS$CNVOUT", vaxfloat.D, "3.14159", 9, 4, ".3142E+01", ssNormal}, // no room for the 0
		{"OTS$CNVOUT", vaxfloat.D, "3.14159", 8, 4, "********", otsOutConErr},
		{"OTS$CNVOUT_G", vaxfloat.G, "1E100", 10, 4, "0.1000+101", ssNormal},
		{"OTS$CNVOUT_G", vaxfloat.G, "-2.5E-200", 12, 5, "-0.25000-199", ssNormal},
		{"OTS$CNVOUT_H", vaxfloat.H, "1.25", 12, 3, "   0.125E+01", ssNormal},
		{"OTS$CNVOUT_H", vaxfloat.H, "1E1000", 14, 4, "**************", otsOutConErr},
	}

	for _, c := range cases {
		putDecimal(t, env, c.f, otsValueAddr, c.value)
		putDescriptor(t, env, otsDescAddr, otsTextAddr, string(make([]byte, c.width)))

		sts := call(t, env, c.routine, otsValueAddr, otsDescAddr, c.digits)
		if got := otsText(t, env, c.width); sts != c.sts || got != c.want {
			t.Errorf("%s(%s, width %d, %d) = %#x %q; want %#x %q", c.routine, c.value, c.width, c.digits, sts, got, c.sts, c.want)
		}
	}

	// A reserved operand: SS$_ROPRAND, the string unchanged.
	putLongword(t, env, otsValueAddr, 0x8000)
	putLongword(t, env, otsValueAddr+4, 0)
	putDescriptor(t, env, otsDescAddr, otsTextAddr, "UNCHANGED")

	if sts := call(t, env, "OTS$CNVOUT", otsValueAddr, otsDescAddr, 2); sts != ssRopRand || otsText(t, env, 9) != "UNCHANGED" {
		t.Errorf("reserved: %#x %q; want SS$_ROPRAND, unchanged", sts, otsText(t, env, 9))
	}
}

// TestOTSTextToFloat: OTS$CVT_T_D, _F, _G, and _H by the manual's
// syntax and flags: its CVT_T_F example's five strings, blanks as zeros
// or ignored, the exponent letters and an exponent begun by its sign,
// the scale factor, tabs, and the range errors.
func TestOTSTextToFloat(t *testing.T) {
	env := fixture(t)

	cases := []struct {
		routine string
		f       vaxfloat.Format
		text    string
		args    []uint32 // digits-in-fraction, scale, flags
		want    string   // the value, a decimal
		sts     uint32
	}{
		// The manual's example (OTS-31): five digits in the fraction.
		{"OTS$CVT_T_F", vaxfloat.F, "1234567+23", []uint32{5}, "1.234567E24", ssNormal},
		{"OTS$CVT_T_F", vaxfloat.F, "8.786534+3", []uint32{5}, "8786.534", ssNormal},
		{"OTS$CVT_T_F", vaxfloat.F, "-983476E-3", []uint32{5}, "-9.83476E-3", ssNormal},
		{"OTS$CVT_T_F", vaxfloat.F, "-23.734532", []uint32{5}, "-23.734532", ssNormal},
		{"OTS$CVT_T_F", vaxfloat.F, "45        ", []uint32{5}, "45000", ssNormal}, // blanks are zeros
		{"OTS$CVT_T_F", vaxfloat.F, "45        ", []uint32{5, 0, cvtFlagIgnoreBlanks}, "0.00045", ssNormal},
		{"OTS$CVT_T_D", vaxfloat.D, "  -1.5D2", nil, "-150", ssNormal},
		{"OTS$CVT_T_D", vaxfloat.D, "1.5q-1", nil, "0.15", ssNormal},
		{"OTS$CVT_T_D", vaxfloat.D, "1.5D2", []uint32{0, 0, cvtFlagOnlyE}, "0", otsInpConErr},
		{"OTS$CVT_T_D", vaxfloat.D, "1.5+2", []uint32{0, 0, cvtFlagNeedLetter}, "0", otsInpConErr},
		{"OTS$CVT_T_D", vaxfloat.D, "1.5", []uint32{0, 2}, "0.015", ssNormal},
		{"OTS$CVT_T_D", vaxfloat.D, "1.5E1", []uint32{0, 2}, "15", ssNormal},
		{"OTS$CVT_T_D", vaxfloat.D, "1.5E1", []uint32{0, 2, cvtFlagAlwaysScale}, "0.15", ssNormal},
		{"OTS$CVT_T_D", vaxfloat.D, "1\t5", nil, "0", otsInpConErr},
		{"OTS$CVT_T_D", vaxfloat.D, "1\t5", []uint32{0, 0, cvtFlagIgnoreTabs}, "15", ssNormal},
		{"OTS$CVT_T_D", vaxfloat.D, "12X", nil, "0", otsInpConErr},
		{"OTS$CVT_T_D", vaxfloat.D, "1..2", nil, "0", otsInpConErr},
		{"OTS$CVT_T_D", vaxfloat.D, "", nil, "0", ssNormal},
		{"OTS$CVT_T_F", vaxfloat.F, "1E40", nil, "0", otsInpConErr},
		{"OTS$CVT_T_F", vaxfloat.F, "1E-40", nil, "0", ssNormal},
		{"OTS$CVT_T_F", vaxfloat.F, "1E-40", []uint32{0, 0, cvtFlagUnderflowErr}, "0", otsInpConErr},
		{"OTS$CVT_T_G", vaxfloat.G, "1E300", nil, "1E300", ssNormal},
		{"OTS$CVT_T_H", vaxfloat.H, "-1.25E4000", nil, "-1.25E4000", ssNormal},
		{"OTS$CVT_T_H", vaxfloat.H, "1E99999999", nil, "0", otsInpConErr},
	}

	for _, c := range cases {
		putDescriptor(t, env, otsDescAddr, otsTextAddr, c.text)

		for i := range 4 {
			putLongword(t, env, otsValueAddr+4*uint32(i), 0xEEEEEEEE)
		}

		sts := call(t, env, c.routine, append([]uint32{otsDescAddr, otsValueAddr}, c.args...)...)
		if got, want := floatBits(t, env, c.f, otsValueAddr), decimalBits(t, c.f, c.want); sts != c.sts || got != want {
			t.Errorf("%s(%q, %v) = %#x %v; want %#x %v (%s)", c.routine, c.text, c.args, sts, got, c.sts, want, c.want)
		}
	}
}

// TestOTSTextToFloatTruncated: flags bit 3 truncates, and extension-bits
// truncates and gives the bits after the value's, left-justified in G's
// and H's word. 0.1 is binary 0.000110011..., its significand 1100
// repeated: F keeps 24 bits (then 11001100), G 53 (then 10011001100).
func TestOTSTextToFloatTruncated(t *testing.T) {
	env := fixture(t)
	putDescriptor(t, env, otsDescAddr, otsTextAddr, "0.1")

	rounded := decimalBits(t, vaxfloat.F, "0.1")

	call(t, env, "OTS$CVT_T_F", otsDescAddr, otsValueAddr, 0, 0, cvtFlagTruncate)

	// F's low fraction word is the longword's high half: one unit in the
	// last place is 0x10000 there.
	if got := floatBits(t, env, vaxfloat.F, otsValueAddr); rounded.Lo-got.Lo != 0x10000 {
		t.Errorf("truncated 0.1 = %08X; want one below the rounded %08X", got.Lo, rounded.Lo)
	}

	extAddr := uint32(otsValueAddr + 0x40)
	putLongword(t, env, extAddr, 0xEEEEEEEE)

	if sts := call(t, env, "OTS$CVT_T_F", otsDescAddr, otsValueAddr, 0, 0, 0, extAddr); sts != ssNormal ||
		rounded.Lo-floatBits(t, env, vaxfloat.F, otsValueAddr).Lo != 0x10000 || longword(t, env, extAddr) != 0xEEEEEECC {
		t.Errorf("F with extension: %#x, ext %08X; want truncated, ext byte CC", sts, longword(t, env, extAddr))
	}

	call(t, env, "OTS$CVT_T_G", otsDescAddr, otsValueAddr, 0, 0, 0, extAddr)

	if got := longword(t, env, extAddr); got != 0xEEEE9980 {
		t.Errorf("G's extension word: %08X; want EEEE9980", got)
	}

	// A value with no more bits: the extension is 0.
	putDescriptor(t, env, otsDescAddr, otsTextAddr, "1.5")
	call(t, env, "OTS$CVT_T_H", otsDescAddr, otsValueAddr, 0, 0, 0, extAddr)

	if got := longword(t, env, extAddr); got != 0xEEEE0000 {
		t.Errorf("H's extension word for 1.5: %08X; want EEEE0000", got)
	}
}

// TestExponentText: the E format's text for values at its edges.
func TestExponentText(t *testing.T) {
	cases := []struct {
		value  string
		digits int
		want   string
	}{
		{"1", 3, "0.100E+01"},
		{"0.1", 3, "0.100E+00"},
		{"0.09999", 3, "0.100E+00"},
		{"1E99", 2, "0.10+100"},
		{"9.99E98", 2, "0.10+100"},
		{"1E-99", 2, "0.10E-98"},
	}

	for _, c := range cases {
		v, err := vaxfloat.Parse(c.value)
		if err != nil {
			t.Fatal(err)
		}

		if got, ok := exponentText(v.Rat(), c.digits); !ok || got != c.want {
			t.Errorf("exponentText(%s, %d) = %q %v; want %q", c.value, c.digits, got, ok, c.want)
		}
	}

	v, _ := vaxfloat.Parse("1E999")
	if got, ok := exponentText(v.Rat(), 2); ok {
		t.Errorf("exponentText(1E999) = %q %v; want not ok", got, ok)
	}
}
