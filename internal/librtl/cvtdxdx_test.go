package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
)

// putItem writes a descriptor at addr: length, data type, class, pointer,
// and for class SD the scale.
func putItem(t *testing.T, env *corevms.Environment, addr uint32, length uint16, dtype, class byte, ptr uint32, scale int8) {
	t.Helper()

	putLongword(t, env, addr, uint32(length)|uint32(dtype)<<16|uint32(class)<<24)
	putLongword(t, env, addr+4, ptr)
	putLongword(t, env, addr+8, uint32(uint8(scale)))
}

// putDecimal stores s, a decimal number, in format f at addr.
func putDecimal(t *testing.T, env *corevms.Environment, f vaxfloat.Format, addr uint32, s string) {
	t.Helper()

	v, err := vaxfloat.Parse(s)
	if err != nil {
		t.Fatal(err)
	}

	if v, err = vaxfloat.Round(f, v); err != nil {
		t.Fatal(err)
	}

	if err := storeFloat(env, f, addr, v); err != nil {
		t.Fatal(err)
	}
}

// Addresses the CVT_DX_DX tests use: the source descriptor and its data,
// the destination descriptor and its data, and the length output.
const (
	cvtSrc     = 0x1000
	cvtSrcData = 0x3000
	cvtDst     = 0x1100
	cvtDstData = 0x4000
	cvtLength  = 0x2000
)

// cvt runs LIB$CVT_DX_DX on the test's two descriptors.
func cvt(t *testing.T, env *corevms.Environment) uint32 {
	t.Helper()
	putLongword(t, env, cvtLength, 0xFFFF)

	return call(t, env, "LIB$CVT_DX_DX", cvtSrc, cvtDst, cvtLength)
}

// TestCvtIntegerToText: integers to text, left-justified and padded in a
// fixed string, a scaled one with its decimal point, into a dynamic
// string; the length output excludes the padding.
func TestCvtIntegerToText(t *testing.T) {
	env := fixture(t)

	cases := []struct {
		dtype  byte
		value  uint32
		scale  int8
		want   string
		length uint32
	}{
		{dtypeL, 1234, 0, "1234    ", 4},
		{dtypeL, 0xFFFFFFD6, 0, "-42     ", 3},
		{dtypeL, 1234, -2, "12.34   ", 5},
		{dtypeL, 5, -3, "0.005   ", 5},
		{dtypeW, 12, 2, "1200    ", 4},
		{dtypeWU, 0xFFFF, 0, "65535   ", 5},
		{dtypeB, 0x80, 0, "-128    ", 4},
		{dtypeBU, 0x80, 0, "128     ", 3},
	}

	for _, c := range cases {
		class := byte(dscClassS)
		if c.scale != 0 {
			class = dscClassSD
		}

		putItem(t, env, cvtSrc, 4, c.dtype, class, cvtSrcData, c.scale)
		putLongword(t, env, cvtSrcData, c.value)
		putItem(t, env, cvtDst, 8, dtypeT, dscClassS, cvtDstData, 0)

		if got := cvt(t, env); got != ssNormal || bytesAt(t, env, cvtDstData, 8) != c.want || longword(t, env, cvtLength) != c.length {
			t.Errorf("type %d %#x scale %d: %#x, %q, length %d; want %q, %d", c.dtype, c.value, c.scale, got, bytesAt(t, env, cvtDstData, 8), longword(t, env, cvtLength), c.want, c.length)
		}
	}

	// A quadword, into a dynamic string.
	putItem(t, env, cvtSrc, 8, dtypeQ, dscClassS, cvtSrcData, 0)
	putLongword(t, env, cvtSrcData, 0)
	putLongword(t, env, cvtSrcData+4, 0x80000000)
	putItem(t, env, dynDesc, 0, dtypeT, dscClassD, 0, 0)

	if got := call(t, env, "LIB$CVT_DX_DX", cvtSrc, dynDesc); got != ssNormal {
		t.Fatalf("Q into D: %#x", got)
	}

	if length, _, ptr := descriptorAt(t, env, dynDesc); bytesAt(t, env, ptr, int(length)) != "-9223372036854775808" {
		t.Errorf("Q into D: %q", bytesAt(t, env, ptr, int(length)))
	}
}

// TestCvtTextToInteger: text to integers, blanks and tabs ignored,
// rounded half away from zero, with an exponent; not a number is
// LIB$_INVNBDS, out of range LIB$_INTOVF (the low-order bits stored),
// negative into unsigned LIB$_INVCVT.
func TestCvtTextToInteger(t *testing.T) {
	env := fixture(t)

	cases := []struct {
		text   string
		dtype  byte
		scale  int8
		status uint32
		want   uint32
	}{
		{" -12 34", dtypeL, 0, ssNormal, 0xFFFFFB2E},
		{"12.5", dtypeL, 0, ssNormal, 13},
		{"-12.5", dtypeL, 0, ssNormal, 0xFFFFFFF3},
		{"1E3", dtypeW, 0, ssNormal, 1000},
		{"\t+7", dtypeB, 0, ssNormal, 7},
		{"12.34", dtypeL, -2, ssNormal, 1234},
		{"", dtypeL, 0, ssNormal, 0},
		{"70000", dtypeW, 0, libIntOvf, 0x1170}, // the low-order bits, as VMS
		{"-1", dtypeWU, 0, libInvCvt, 0x5555},
		{"12X", dtypeL, 0, libInvNbds, 0x5555},
		{"1.2.3", dtypeL, 0, libInvNbds, 0x5555},
		{"E5", dtypeL, 0, libInvNbds, 0x5555},
	}

	for _, c := range cases {
		putBytes(t, env, cvtSrcData, c.text)
		putItem(t, env, cvtSrc, uint16(len(c.text)), dtypeT, dscClassS, cvtSrcData, 0)

		class := byte(dscClassS)
		if c.scale != 0 {
			class = dscClassSD
		}

		putItem(t, env, cvtDst, 4, c.dtype, class, cvtDstData, c.scale)
		putLongword(t, env, cvtDstData, 0x5555)

		mask := map[byte]uint32{dtypeB: 0xFF, dtypeW: 0xFFFF, dtypeWU: 0xFFFF, dtypeL: 0xFFFFFFFF}[c.dtype]
		if status := cvt(t, env); status != c.status {
			t.Errorf("%q: status %#x, want %#x", c.text, status, c.status)
		} else if got := longword(t, env, cvtDstData); got&mask != c.want&mask {
			t.Errorf("%q: %#x, want %#x", c.text, got&mask, c.want&mask)
		}

		// The length output is 0 for a numeric destination (VMS), and
		// left alone when the conversion fails.
		want := uint32(0)
		if c.status != ssNormal {
			want = 0xFFFF
		}

		if got := longword(t, env, cvtLength); got != want {
			t.Errorf("%q: the length output %#x, want %#x", c.text, got, want)
		}
	}
}

// TestCvtFloating: text to F, floating to text in Table LIB-3's E
// format with as many digits as the destination's width allows, and
// floating to and from integers and other formats.
func TestCvtFloating(t *testing.T) {
	env := fixture(t)

	// Text to F: 1.5 is ^X000040C0.
	putBytes(t, env, cvtSrcData, "1.5")
	putItem(t, env, cvtSrc, 3, dtypeT, dscClassS, cvtSrcData, 0)
	putItem(t, env, cvtDst, 4, dtypeF, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != ssNormal || longword(t, env, cvtDstData) != 0x000040C0 {
		t.Errorf("1.5 to F: %#x, %#x", got, longword(t, env, cvtDstData))
	}

	toText := []struct {
		f     vaxfloat.Format
		dtype byte
		value string
		width uint16
		want  string
	}{
		{vaxfloat.F, dtypeF, "1.5", 20, "0.1500000E+01"},
		{vaxfloat.F, dtypeF, "1.5", 10, "0.150E+01"},
		{vaxfloat.F, dtypeF, "-0.001", 20, "-0.1000000E-02"},
		{vaxfloat.F, dtypeF, "0", 20, "0.0000000E+00"},
		{vaxfloat.F, dtypeF, "9.9999999", 20, "0.1000000E+02"},
		{vaxfloat.D, dtypeD, "123.25", 30, "0.1232500000000000E+03"},
		{vaxfloat.G, dtypeG, "-123.456", 30, "-0.123456000000000E+003"},
		{vaxfloat.H, dtypeH, "2", 50, "0.200000000000000000000000000000000E+0001"},
	}

	for _, c := range toText {
		putDecimal(t, env, c.f, cvtSrcData, c.value)
		putItem(t, env, cvtSrc, uint16(c.f.Size()), c.dtype, dscClassS, cvtSrcData, 0)
		putItem(t, env, cvtDst, c.width, dtypeT, dscClassS, cvtDstData, 0)

		got := cvt(t, env)
		text := bytesAt(t, env, cvtDstData, len(c.want))

		if got != ssNormal || text != c.want || longword(t, env, cvtLength) != uint32(len(c.want)) {
			t.Errorf("%s %s into %d: %#x, %q (length %d); want %q", c.f, c.value, c.width, got, text, longword(t, env, cvtLength), c.want)
		}
	}

	// F 2.5 to L rounds away from zero; L 7 to H and back.
	putDecimal(t, env, vaxfloat.F, cvtSrcData, "-2.5")
	putItem(t, env, cvtSrc, 4, dtypeF, dscClassS, cvtSrcData, 0)
	putItem(t, env, cvtDst, 4, dtypeL, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != ssNormal || longword(t, env, cvtDstData) != 0xFFFFFFFD {
		t.Errorf("F -2.5 to L: %#x, %#x; want -3", got, longword(t, env, cvtDstData))
	}

	putLongword(t, env, cvtSrcData, 7)
	putItem(t, env, cvtSrc, 4, dtypeL, dscClassS, cvtSrcData, 0)
	putItem(t, env, cvtDst, 16, dtypeH, dscClassS, cvtDstData, 0)
	cvt(t, env)
	putItem(t, env, cvtSrc, 16, dtypeH, dscClassS, cvtDstData, 0)
	putItem(t, env, cvtDst, 4, dtypeL, dscClassS, 0x5000, 0)

	if got := cvt(t, env); got != ssNormal || longword(t, env, 0x5000) != 7 {
		t.Errorf("L 7 to H and back: %#x, %d", got, longword(t, env, 0x5000))
	}
}

// TestCvtRange: values an atomic destination can't hold, and a reserved
// operand.
func TestCvtRange(t *testing.T) {
	env := fixture(t)

	putLongword(t, env, cvtSrcData, 300)
	putItem(t, env, cvtSrc, 4, dtypeL, dscClassS, cvtSrcData, 0)
	putItem(t, env, cvtDst, 1, dtypeB, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != libIntOvf || longword(t, env, cvtDstData)&0xFF != 0x2C {
		t.Errorf("L 300 to B: %#x, %#x; want LIB$_INTOVF, ^X2C", got, longword(t, env, cvtDstData)&0xFF)
	}

	putLongword(t, env, cvtSrcData, 0xFFFFFFFF)
	putItem(t, env, cvtDst, 1, dtypeBU, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != libInvCvt {
		t.Errorf("L -1 to BU: %#x, want LIB$_INVCVT", got)
	}

	for _, c := range []struct {
		value string
		want  uint32
	}{{"1E100", libFltOvf}, {"1E-100", libFltUnd}} {
		putDecimal(t, env, vaxfloat.H, cvtSrcData, c.value)
		putItem(t, env, cvtSrc, 16, dtypeH, dscClassS, cvtSrcData, 0)
		putItem(t, env, cvtDst, 4, dtypeF, dscClassS, cvtDstData, 0)

		if got := cvt(t, env); got != c.want {
			t.Errorf("H %s to F: %#x, want %#x", c.value, got, c.want)
		}
	}

	putLongword(t, env, cvtSrcData, 0x8000) // F: sign set, exponent 0
	putItem(t, env, cvtSrc, 4, dtypeF, dscClassS, cvtSrcData, 0)
	putItem(t, env, cvtDst, 4, dtypeL, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != libROprand {
		t.Errorf("a reserved F: %#x, want LIB$_ROPRAND", got)
	}
}

// TestCvtTextToText: one numeric string to another, unscaled, is a copy
// (LIB$_OUTSTRTRU when cut); a varying text source is read by CURLEN.
func TestCvtTextToText(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, cvtSrcData, "12345")
	putItem(t, env, cvtSrc, 5, dtypeT, dscClassS, cvtSrcData, 0)
	putItem(t, env, cvtDst, 3, dtypeT, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != libOutStrTru || bytesAt(t, env, cvtDstData, 3) != "123" || longword(t, env, cvtLength) != 3 {
		t.Errorf("into S(3): %#x, %q, length %d", got, bytesAt(t, env, cvtDstData, 3), longword(t, env, cvtLength))
	}

	putLongword(t, env, 0x3100, 2) // CURLEN
	putBytes(t, env, 0x3102, "-9xyz")
	putItem(t, env, cvtSrc, 10, dtypeVT, dscClassVS, 0x3100, 0)
	putItem(t, env, cvtDst, 4, dtypeL, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != ssNormal || longword(t, env, cvtDstData) != 0xFFFFFFF7 {
		t.Errorf("VT \"-9\" to L: %#x, %#x", got, longword(t, env, cvtDstData))
	}
}

// TestCvtDescriptors: a class, a data type, or a pair the routine doesn't
// take.
func TestCvtDescriptors(t *testing.T) {
	env := fixture(t)
	putItem(t, env, cvtDst, 4, dtypeL, dscClassS, cvtDstData, 0)

	cases := []struct {
		name         string
		dtype, class byte
		want         uint32
	}{
		{"class V", dtypeL, 3, libInvClaDsc},
		{"packed decimal", 21, dscClassS, libInvDtyDsc},
		{"L, class D", dtypeL, dscClassD, libInvClaDty},
		{"VT, class S", dtypeVT, dscClassS, libInvClaDty},

	}

	for _, c := range cases {
		putItem(t, env, cvtSrc, 4, c.dtype, c.class, cvtSrcData, 0)

		if got := cvt(t, env); got != c.want {
			t.Errorf("%s: %#x, want %#x", c.name, got, c.want)
		}
	}
}

// TestCvtByteArray: an array of unsigned bytes (BU, class A) is a
// numeric byte data string; and a number too long for its text
// destination is LIB$_DESSTROVF, written as far as it fits, the length
// output left alone (VMS's run).
func TestCvtByteArray(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, cvtSrcData, "42")
	putItem(t, env, cvtSrc, 1, dtypeBU, dscClassA, cvtSrcData, 0)
	putLongword(t, env, cvtSrc+12, 2) // ARSIZE
	putItem(t, env, cvtDst, 4, dtypeL, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != ssNormal || longword(t, env, cvtDstData) != 42 {
		t.Errorf("BU array \"42\" to L: %#x, %d", got, longword(t, env, cvtDstData))
	}

	putLongword(t, env, cvtSrcData, 123456)
	putItem(t, env, cvtSrc, 4, dtypeL, dscClassS, cvtSrcData, 0)
	putItem(t, env, cvtDst, 3, dtypeT, dscClassS, cvtDstData, 0)

	if got := cvt(t, env); got != libDesStrOvf || bytesAt(t, env, cvtDstData, 3) != "123" || longword(t, env, cvtLength) != 0xFFFF {
		t.Errorf("L 123456 into 3: %#x, %q, length %#x", got, bytesAt(t, env, cvtDstData, 3), longword(t, env, cvtLength))
	}
}
