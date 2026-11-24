package librtl

import (
	"math/big"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// The STR$ arithmetic routines' own tests (Phase 113, subtask 7): the manuals'
// examples through the shim table, and the arithmetic's rules alone, as
// testdata/str113's probe found VMS 7.1's (subtask 13).

// number makes a decimal from its three parts.
func number(neg bool, exp int64, digits string) decimal {
	x, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		panic(digits)
	}

	return decimal{neg: neg, exp: exp, digits: x, text: digits}
}

// sameDecimal reports whether d is sign, exponent, and digits.
func sameDecimal(d decimal, neg bool, exp int64, digits string) bool {
	return d.neg == neg && d.exp == exp && d.text == digits
}

// The addresses a call's arguments are laid out at.
const (
	mathA    = 0x2000 // asign, aexp, then adigits' descriptor at +8
	mathB    = 0x2100
	mathC    = 0x2200
	mathMisc = 0x2300 // total-digits, the indicator, places, flags
)

// putNumber lays out a number's sign, exponent, and digits' descriptor at at.
func putNumber(t *testing.T, env *corevms.Environment, at uint32, sign uint32, exp int32, digits string) {
	t.Helper()
	putLongword(t, env, at, sign)
	putLongword(t, env, at+4, uint32(exp))
	putDescriptor(t, env, at+8, at+0x40, digits)
}

// putResult lays out a result's place: sign and exponent of -1, and an empty
// dynamic string.
func putResult(t *testing.T, env *corevms.Environment) {
	t.Helper()
	putLongword(t, env, mathC, 0xFFFFFFFF)
	putLongword(t, env, mathC+4, 0xFFFFFFFF)
	putLongword(t, env, mathC+8, 0x020E0000) // DSC$K_DTYPE_T, DSC$K_CLASS_D
	putLongword(t, env, mathC+12, 0)
}

// result reads the result's sign, exponent, and digits.
func result(t *testing.T, env *corevms.Environment) (uint32, int32, string) {
	t.Helper()

	digits, err := readString(env, mathC+8)
	if err != nil {
		t.Fatal(err)
	}

	return longword(t, env, mathC), int32(longword(t, env, mathC+4)), digits
}

// TestStrMathManualExamples: each manual example's A = -1000 and B = .0002,
// and the answers the manual prints.
func TestStrMathManualExamples(t *testing.T) {
	a := []uint32{mathA, mathA + 4, mathA + 8}
	b := []uint32{mathB, mathB + 4, mathB + 8}
	c := []uint32{mathC, mathC + 4, mathC + 8}

	join := func(parts ...[]uint32) []uint32 {
		var out []uint32
		for _, p := range parts {
			out = append(out, p...)
		}

		return out
	}

	cases := []struct {
		routine string
		args    []uint32
		sign    uint32
		exp     int32
		digits  string
	}{
		{"STR$ADD", join(a, b, c), 1, -4, "9999998"},
		{"STR$MUL", join(a, b, c), 1, -1, "2"},
		{"STR$DIVIDE", join(a, b, []uint32{mathMisc, mathMisc + 4}, c), 1, -3, "5000000000"},
		{"STR$RECIP", join(a, b, c), 1, -3, "1"},
	}

	for _, k := range cases {
		env := fixture(t)
		putNumber(t, env, mathA, 1, 3, "1")
		putNumber(t, env, mathB, 0, -4, "2")
		putLongword(t, env, mathMisc, 3)
		putLongword(t, env, mathMisc+4, 0)
		putResult(t, env)

		if r0 := call(t, env, k.routine, k.args...); r0 != ssNormal {
			t.Errorf("%s: R0 = %#x, want SS$_NORMAL", k.routine, r0)
		}

		if sign, exp, digits := result(t, env); sign != k.sign || exp != k.exp || digits != k.digits {
			t.Errorf("%s: C = %d %d %q, want %d %d %q", k.routine, sign, exp, digits, k.sign, k.exp, k.digits)
		}
	}

	// STR$ROUND's 999.9998 to 3 places, rounded and truncated.
	for flags, want := range []struct {
		exp    int32
		digits string
	}{{1, "100"}, {0, "999"}} {
		env := fixture(t)
		putNumber(t, env, mathA, 0, -4, "9999998")
		putLongword(t, env, mathMisc, 3)
		putLongword(t, env, mathMisc+4, uint32(flags))
		putResult(t, env)

		if r0 := call(t, env, "STR$ROUND", join([]uint32{mathMisc, mathMisc + 4}, a, c)...); r0 != ssNormal {
			t.Errorf("STR$ROUND flags %d: R0 = %#x", flags, r0)
		}

		if sign, exp, digits := result(t, env); sign != 0 || exp != want.exp || digits != want.digits {
			t.Errorf("STR$ROUND flags %d: C = %d %d %q, want 0 %d %q", flags, sign, exp, digits, want.exp, want.digits)
		}
	}
}

// TestStrMathTruncation: a fixed destination too short for the digits is
// STR$_TRU, the exponent and sign still written.
func TestStrMathTruncation(t *testing.T) {
	env := fixture(t)
	putNumber(t, env, mathA, 0, 0, "12345")
	putNumber(t, env, mathB, 0, 0, "1")
	putLongword(t, env, mathC, 9)
	putLongword(t, env, mathC+4, 9)
	putDescriptor(t, env, mathC+8, mathC+0x40, "---")

	r0 := call(t, env, "STR$ADD", mathA, mathA+4, mathA+8, mathB, mathB+4, mathB+8, mathC, mathC+4, mathC+8)
	if r0 != strTru {
		t.Errorf("R0 = %#x, want STR$_TRU", r0)
	}

	if sign, exp, digits := result(t, env); sign != 0 || exp != 0 || digits != "123" {
		t.Errorf("C = %d %d %q, want 0 0 \"123\"", sign, exp, digits)
	}
}

func TestAddDecimals(t *testing.T) {
	cases := []struct {
		a, b   decimal
		neg    bool
		exp    int64
		digits string
	}{
		{number(false, 0, "5"), number(false, 0, "7"), false, 0, "12"},
		{number(false, 2, "5"), number(true, 0, "7"), false, 0, "493"},
		{number(true, 2, "5"), number(false, 0, "7"), true, 0, "493"},
		{number(false, 0, "7"), number(true, 0, "7"), false, 0, "0"},
		{number(false, 3, "7"), number(true, 3, "7"), false, 0, "0"},
		{number(false, 0, "0"), number(true, -3, "7"), true, -3, "7"},
		{number(false, 1, "10"), number(false, 1, "0"), false, 2, "1"},
		{number(false, 0, "007"), number(false, 0, "3"), false, 1, "1"},
		{number(false, 0, "999"), number(false, 0, "1"), false, 3, "1"},
		{number(true, 2, "0"), number(false, -2, "000"), false, 0, "0"},
	}

	for _, c := range cases {
		got, err := addDecimals(c.a, c.b)
		if err != nil {
			t.Fatal(err)
		}

		if !sameDecimal(got, c.neg, c.exp, c.digits) {
			t.Errorf("%+v + %+v = %v %d %s, want %v %d %s", c.a, c.b, got.neg, got.exp, got.text, c.neg, c.exp, c.digits)
		}
	}

	if _, err := addDecimals(number(false, 70000, "1"), number(false, 0, "1")); err == nil {
		t.Error("a sum of 70,001 digits: want an error")
	}
}

func TestMulDecimals(t *testing.T) {
	cases := []struct {
		a, b   decimal
		neg    bool
		exp    int64
		digits string
	}{
		{number(false, 0, "25"), number(false, 0, "4"), false, 2, "1"},
		{number(true, 0, "3"), number(false, 0, "0"), true, 0, "0"},
		{number(false, 5, "0"), number(true, -2, "0"), true, 3, "0"},
		{number(false, 2, "12"), number(false, -1, "5"), false, 2, "6"},
		{number(false, 2147483647, "1"), number(false, 1, "1"), false, 2147483648, "1"},
	}

	for _, c := range cases {
		got := mulDecimals(c.a, c.b)
		if !sameDecimal(got, c.neg, c.exp, c.digits) {
			t.Errorf("%+v * %+v = %v %d %s, want %v %d %s", c.a, c.b, got.neg, got.exp, got.text, c.neg, c.exp, c.digits)
		}
	}
}

func TestDivideDecimals(t *testing.T) {
	cases := []struct {
		a, b      decimal
		places    int64
		indicator int64
		neg       bool
		exp       int64
		digits    string
	}{
		{number(false, 0, "2"), number(false, 0, "3"), 4, 0, false, -4, "6666"},
		{number(false, 0, "2"), number(false, 0, "3"), 4, 1, false, -4, "6667"},
		{number(false, 0, "2"), number(false, 0, "3"), 4, 2, false, -4, "666666"},
		{number(false, 0, "2"), number(false, 0, "3"), 4, 3, false, -4, "6666666"},
		{number(false, 0, "2"), number(false, 0, "3"), 4, -1, false, -4, "666"},
		{number(false, 0, "1"), number(false, 0, "8"), 2, 1, false, -2, "13"},
		{number(true, 0, "1"), number(false, 0, "8"), 2, 1, true, -2, "13"},
		{number(false, 0, "10"), number(false, 0, "4"), 0, 1, false, 0, "3"},
		{number(false, 0, "1"), number(false, 0, "3"), 0, 0, false, 0, "0"},
		{number(true, 0, "1"), number(false, 0, "3"), 0, 0, true, 0, "0"},
		{number(false, 0, "12345"), number(false, 0, "1"), -2, 0, false, 2, "123"},
		{number(false, 0, "6"), number(false, 0, "2"), 2, 0, false, -2, "300"},
		{number(false, -9, "1"), number(false, 0, "1"), 2, 1, false, -2, "0"},
		{number(true, 0, "0"), number(false, 0, "5"), 2, 0, false, -2, "00"},
	}

	for _, c := range cases {
		got, err := divideDecimals(c.a, c.b, c.places, c.indicator)
		if err != nil {
			t.Fatal(err)
		}

		if !sameDecimal(got, c.neg, c.exp, c.digits) {
			t.Errorf("%v / %v to %d, %d: %v %d %s, want %v %d %s", c.a.digits, c.b.digits, c.places, c.indicator,
				got.neg, got.exp, got.text, c.neg, c.exp, c.digits)
		}
	}

	if _, err := divideDecimals(number(false, 0, "1"), number(false, 0, "3"), 70000, 0); err == nil {
		t.Error("70,000 places: want an error")
	}
}

func TestRecipDecimal(t *testing.T) {
	cases := []struct {
		a, b   decimal
		neg    bool
		exp    int64
		digits string
	}{
		{number(true, 3, "1"), number(false, -4, "2"), true, -3, "1"},
		{number(false, 0, "3"), number(false, -4, "1"), false, -5, "33333"},
		{number(false, 0, "6"), number(false, -4, "9"), false, -4, "1666"},
		{number(false, 0, "6"), number(false, -4, "25"), false, -4, "1666"},
		{number(false, 0, "8"), number(false, -4, "1"), false, -3, "125"},
		{number(false, 0, "3"), number(false, 2, "1"), false, 0, "0"},
		{number(false, 0, "3"), number(false, 0, "1"), false, -1, "3"},
		{number(false, 0, "1"), number(false, -4, "1"), false, 0, "1"},
	}

	for _, c := range cases {
		got, err := recipDecimal(c.a, c.b)
		if err != nil {
			t.Fatal(err)
		}

		if !sameDecimal(got, c.neg, c.exp, c.digits) {
			t.Errorf("1 / %v to %v e%d: %v %d %s, want %v %d %s", c.a.digits, c.b.digits, c.b.exp,
				got.neg, got.exp, got.text, c.neg, c.exp, c.digits)
		}
	}

	if _, err := recipDecimal(number(false, 0, "6"), number(false, -4, "0")); err == nil {
		t.Error("a precision limit of 0: want an error")
	}
}

func TestRoundDecimal(t *testing.T) {
	cases := []struct {
		a        decimal
		places   int64
		truncate bool
		exp      int64
		digits   string
	}{
		{number(false, -4, "9999998"), 3, false, 1, "100"},
		{number(false, -4, "9999998"), 3, true, 0, "999"},
		{number(false, 0, "12345"), 3, false, 2, "123"},
		{number(false, 0, "12355"), 3, false, 2, "124"},
		{number(false, 0, "125"), 2, false, 1, "13"},
		{number(false, 0, "12"), 5, false, 0, "12"},
		{number(false, 0, "95"), 1, false, 2, "1"},
		{number(false, 0, "0099"), 1, false, 3, "0"},
		{number(false, 5, "0"), 3, false, 5, "0"},
		{number(false, 0, "12345"), 0, false, 0, "0"},
	}

	for _, c := range cases {
		got := roundDecimal(c.a, c.places, c.truncate)
		if !sameDecimal(got, false, c.exp, c.digits) {
			t.Errorf("%v e%d to %d: %d %s, want %d %s", c.a.text, c.a.exp, c.places, got.exp, got.text, c.exp, c.digits)
		}
	}
}

func TestNormal(t *testing.T) {
	if got := number(false, -4, "1000").normal(); !sameDecimal(got, false, -1, "1") {
		t.Errorf("1000e-4 = %d %s, want -1 1", got.exp, got.text)
	}

	if got := number(true, 3, "000").normal(); !sameDecimal(got, true, 3, "0") {
		t.Errorf("-000e3 = %v %d %s, want a zero as it was", got.neg, got.exp, got.text)
	}
}

func TestDecimalDigits(t *testing.T) {
	for s, want := range map[string]string{"": "", "0": "0", "0123": "0123", "12A": "12", " 12": "", "12 ": "12", "+12": "", "1.2": "1"} {
		if got, _ := decimalDigits(s); got != want {
			t.Errorf("decimalDigits(%q) = %q, want %q", s, got, want)
		}
	}
}

// TestStrElementR8: the guessed JSB form gives STR$ELEMENT's answer.
func TestStrElementR8(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, 0x1000, 0x020E0000)
	putLongword(t, env, 0x1004, 0)
	putDescriptor(t, env, 0x1010, 0x1110, "A,BC,D")
	putDescriptor(t, env, 0x1020, 0x1120, ",")

	regs := callJSB(t, env, "STR$ELEMENT_R8", 0x1000, 1, 0x1020, 0x1010)
	if regs[0] != ssNormal {
		t.Errorf("R0 = %#x", regs[0])
	}

	if got, err := readString(env, 0x1000); err != nil || got != "BC" {
		t.Errorf("element 1 = %q, %v; want \"BC\"", got, err)
	}
}
