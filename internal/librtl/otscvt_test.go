package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// Addresses the OTS$ tests use: the value, the descriptor, its text.
const (
	otsValueAddr = 0x2000
	otsDescAddr  = 0x2100
	otsTextAddr  = 0x2200
)

// otsText reads n bytes at otsTextAddr.
func otsText(t *testing.T, env *corevms.Environment, n int) string {
	t.Helper()

	s, err := readText(env, otsTextAddr, uint32(n))
	if err != nil {
		t.Fatal(err)
	}

	return s
}

// TestOTSIntegerToText: OTS$CVT_L_TI, _TU, _TB, _TO, _TZ, and _TL into
// fixed-length strings: right-justified, int-digits, value sizes, the
// plus flag, and OTS$_OUTCONERR's asterisks.
func TestOTSIntegerToText(t *testing.T) {
	env := fixture(t)

	cases := []struct {
		routine string
		value   uint32
		width   int
		args    []uint32 // after the value and string
		want    string
		sts     uint32
	}{
		{"OTS$CVT_L_TI", 0, 4, nil, "   0", ssNormal},       // DUNGEON's score
		{"OTS$CVT_L_TI", 585, 4, nil, " 585", ssNormal},     // and its total
		{"OTS$CVT_L_TI", 0xFFFFFFD6, 5, nil, "  -42", ssNormal},
		{"OTS$CVT_L_TI", 42, 6, []uint32{4}, "  0042", ssNormal},
		{"OTS$CVT_L_TI", 0, 3, []uint32{0}, "   ", ssNormal}, // zero, no digits
		{"OTS$CVT_L_TI", 7, 3, []uint32{1, 4, 1}, " +7", ssNormal},
		{"OTS$CVT_L_TI", 0x80, 4, []uint32{1, 1}, "-128", ssNormal}, // a byte, sign-extended
		{"OTS$CVT_L_TI", 0xFFFF, 4, []uint32{1, 2}, "  -1", ssNormal},
		{"OTS$CVT_L_TI", 12345, 3, nil, "***", otsOutConErr},
		{"OTS$CVT_L_TU", 0xFFFFFFFF, 10, nil, "4294967295", ssNormal},
		{"OTS$CVT_L_TU", 32857, 7, []uint32{7}, "0032857", ssNormal}, // the manual's example
		{"OTS$CVT_L_TU", 0x1FF, 4, []uint32{1, 1}, " 255", ssNormal},
		{"OTS$CVT_L_TB", 13, 4, nil, "1101", ssNormal}, // the manual's example
		{"OTS$CVT_L_TB", 13, 3, nil, "***", otsOutConErr},
		{"OTS$CVT_L_TO", 0o777, 5, nil, "  777", ssNormal},
		{"OTS$CVT_L_TZ", 256, 11, nil, "        100", ssNormal}, // the manual's example
		{"OTS$CVT_L_TZ", 0xDEAD, 8, []uint32{8}, "0000DEAD", ssNormal},
		{"OTS$CVT_L_TL", 1, 3, nil, "  T", ssNormal},
		{"OTS$CVT_L_TL", 10, 1, nil, "F", ssNormal}, // the manual's example
	}

	for _, c := range cases {
		putLongword(t, env, otsValueAddr, c.value)
		putDescriptor(t, env, otsDescAddr, otsTextAddr, string(make([]byte, c.width)))

		sts := call(t, env, c.routine, append([]uint32{otsValueAddr, otsDescAddr}, c.args...)...)
		if got := otsText(t, env, c.width); sts != c.sts || got != c.want {
			t.Errorf("%s(%#x, width %d, %v) = %#x %q; want %#x %q", c.routine, c.value, c.width, c.args, sts, got, c.sts, c.want)
		}
	}

	// An octaword in hexadecimal: arbitrary length.
	for i := range 16 {
		putBytes(t, env, otsValueAddr+uint32(i), string(rune(i+1)))
	}

	putDescriptor(t, env, otsDescAddr, otsTextAddr, string(make([]byte, 32)))

	if sts := call(t, env, "OTS$CVT_L_TZ", otsValueAddr, otsDescAddr, 1, 16); sts != ssNormal ||
		otsText(t, env, 32) != "100F0E0D0C0B0A090807060504030201" {
		t.Errorf("an octaword: %#x %q", sts, otsText(t, env, 32))
	}

	// No room for L_TL's letter.
	putDescriptor(t, env, otsDescAddr, otsTextAddr, "")

	if sts := call(t, env, "OTS$CVT_L_TL", otsValueAddr, otsDescAddr); sts != otsOutConErr {
		t.Errorf("L_TL into an empty string = %#x, want OTS$_OUTCONERR", sts)
	}
}

// TestOTSTextToInteger: OTS$CVT_TI_L, _TU_L, _TB_L, _TO_L, _TZ_L, and
// _TL_L: signs, blanks and tabs by the flags, sizes, overflow, and what
// an error leaves in the result.
func TestOTSTextToInteger(t *testing.T) {
	env := fixture(t)

	const untouched = 0x55555555

	cases := []struct {
		routine, text string
		args          []uint32 // after the string and result
		want          uint32
		sts           uint32
	}{
		{"OTS$CVT_TI_L", "  -42", nil, 0xFFFFFFD6, ssNormal},
		{"OTS$CVT_TI_L", "+7", nil, 7, ssNormal},
		{"OTS$CVT_TI_L", " 1 2", nil, 102, ssNormal},              // an inner blank is a zero
		{"OTS$CVT_TI_L", " 1 2", []uint32{4, 1}, 12, ssNormal},    // or ignored
		{"OTS$CVT_TI_L", "1\t2", nil, 0, otsInpConErr},            // a tab is invalid, the result zero
		{"OTS$CVT_TI_L", "1\t2", []uint32{4, 0x10}, 12, ssNormal}, // or ignored
		{"OTS$CVT_TI_L", "", nil, 0, ssNormal},
		{"OTS$CVT_TI_L", "127", []uint32{1}, 0x5555557F, ssNormal}, // a byte
		{"OTS$CVT_TI_L", "128", []uint32{1}, 0x55555500, otsInpConErr},
		{"OTS$CVT_TI_L", "-32768", []uint32{2}, 0x55558000, ssNormal},
		{"OTS$CVT_TI_L", "2147483648", nil, 0, otsInpConErr},
		{"OTS$CVT_TI_L", "1", []uint32{3}, 0, otsInpConErr}, // a bad size
		{"OTS$CVT_TI_L", "1X", nil, 0, otsInpConErr},
		{"OTS$CVT_TU_L", "4294967295", nil, 0xFFFFFFFF, ssNormal},
		{"OTS$CVT_TU_L", "-1", nil, untouched, otsInpConErr}, // no sign
		{"OTS$CVT_TU_L", "256", []uint32{1}, untouched, otsInpConErr},
		{"OTS$CVT_TU_L", "12", []uint32{3}, 12, ssNormal}, // another size is 4
		{"OTS$CVT_TB_L", "1 111", nil, 0b10111, ssNormal},
		{"OTS$CVT_TB_L", "1 111", []uint32{4, 0x11}, 15, ssNormal}, // the manual's example
		{"OTS$CVT_TB_L", "2", nil, untouched, otsInpConErr},
		{"OTS$CVT_TO_L", "1017346", []uint32{4, 1}, 270054, ssNormal}, // the manual's example, whose 274150 is wrong
		{"OTS$CVT_TO_L", "8", nil, untouched, otsInpConErr},
		{"OTS$CVT_TO_L", "1", []uint32{0}, untouched, otsInpConErr}, // a size of 0
		{"OTS$CVT_TZ_L", "DEADbeef", nil, 0xDEADBEEF, ssNormal},
		{"OTS$CVT_TZ_L", "100000000", nil, untouched, otsInpConErr},
		{"OTS$CVT_TL_L", "  .TRUE.", nil, 0xFFFFFFFF, ssNormal},
		{"OTS$CVT_TL_L", "f", nil, 0, ssNormal},
		{"OTS$CVT_TL_L", "   ", nil, 0, ssNormal},
		{"OTS$CVT_TL_L", "X", nil, 0, otsInpConErr},
	}

	for _, c := range cases {
		putLongword(t, env, otsValueAddr, untouched)
		putDescriptor(t, env, otsDescAddr, otsTextAddr, c.text)

		sts := call(t, env, c.routine, append([]uint32{otsDescAddr, otsValueAddr}, c.args...)...)
		if got := longword(t, env, otsValueAddr); sts != c.sts || got != c.want {
			t.Errorf("%s(%q, %v) = %#x, result %#x; want %#x, %#x", c.routine, c.text, c.args, sts, got, c.sts, c.want)
		}
	}
}
