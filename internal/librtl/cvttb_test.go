package librtl

import "testing"

// TestCvtTextToBinary: LIB$CVT_DTB, _HTB, and _OTB: digits of the radix,
// a leading sign, 32-bit overflow, and the characters that fail.
func TestCvtTextToBinary(t *testing.T) {
	env := fixture(t)

	cases := []struct {
		routine, text string
		ok            bool
		want          uint32
	}{
		{"LIB$CVT_DTB", "12345", true, 12345},
		{"LIB$CVT_DTB", "-42", true, 0xFFFFFFD6},
		{"LIB$CVT_DTB", "+7", true, 7},
		{"LIB$CVT_DTB", "4294967295", true, 0xFFFFFFFF},
		{"LIB$CVT_DTB", "4294967296", false, 0},
		{"LIB$CVT_DTB", "12 3", false, 0}, // a blank
		{"LIB$CVT_DTB", "1\t", false, 0},  // a tab
		{"LIB$CVT_DTB", "1-2", false, 0},  // a sign after the first character
		{"LIB$CVT_DTB", "1A", false, 0},
		{"LIB$CVT_DTB", "", true, 0},
		{"LIB$CVT_DTB", "-", true, 0},
		{"LIB$CVT_HTB", "7FFFFFFF", true, 0x7FFFFFFF},
		{"LIB$CVT_HTB", "dead", true, 0xDEAD},
		{"LIB$CVT_HTB", "-1", true, 0xFFFFFFFF},
		{"LIB$CVT_HTB", "100000000", false, 0},
		{"LIB$CVT_HTB", "G", false, 0},
		{"LIB$CVT_OTB", "777", true, 0o777},
		{"LIB$CVT_OTB", "37777777777", true, 0xFFFFFFFF},
		{"LIB$CVT_OTB", "8", false, 0},
	}

	for _, c := range cases {
		putBytes(t, env, 0x3000, c.text)
		putLongword(t, env, 0x2000, 0x5555)

		status := call(t, env, c.routine, uint32(len(c.text)), 0x3000, 0x2000)
		got := longword(t, env, 0x2000)

		switch {
		case c.ok && (status != 1 || got != c.want):
			t.Errorf("%s(%q) = %d, result %#x; want 1, %#x", c.routine, c.text, status, got, c.want)
		case !c.ok && (status != 0 || got != 0x5555):
			t.Errorf("%s(%q) = %d, result %#x; want 0, nothing written", c.routine, c.text, status, got)
		}
	}

	// The count is a value: only that many bytes are read.
	putBytes(t, env, 0x3000, "123X")

	if status := call(t, env, "LIB$CVT_DTB", 3, 0x3000, 0x2000); status != 1 || longword(t, env, 0x2000) != 123 {
		t.Errorf("3 bytes of \"123X\": %d, %d", status, longword(t, env, 0x2000))
	}

	// A negative count is no digits: 0, success (VMS).
	if status := call(t, env, "LIB$CVT_DTB", 0xFFFFFFFF, 0x3000, 0x2000); status != 1 || longword(t, env, 0x2000) != 0 {
		t.Errorf("a negative count: %d, %d; want 1, 0", status, longword(t, env, 0x2000))
	}
}
