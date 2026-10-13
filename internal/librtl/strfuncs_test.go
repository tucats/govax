package librtl

import "testing"

// TestLen: LIB$LEN by class; an invalid descriptor is 0.
func TestLen(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, 0x1000, 7, dscClassS, 0x3000)
	putDesc(t, env, 0x1010, 40, dscClassVS, 0x3100)
	putLongword(t, env, 0x3100, 3) // CURLEN
	putDesc(t, env, 0x1020, 7, 3, 0x3000)

	for _, c := range []struct{ desc, want uint32 }{{0x1000, 7}, {0x1010, 3}, {0x1020, 0}} {
		if got := call(t, env, "LIB$LEN", c.desc); got != c.want {
			t.Errorf("LIB$LEN(%#x) = %d, want %d", c.desc, got, c.want)
		}
	}
}

// TestChar: LIB$CHAR writes one character, blank-filled to a fixed
// destination, length 1 for a dynamic one; LIB$_STRTRU for a fixed one
// of length 0.
func TestChar(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, 0x2000, 0xC1) // the code (a byte); Á in DEC Multinational
	putDesc(t, env, 0x1000, 3, dscClassS, 0x3000)
	putDesc(t, env, 0x1008, 0, dscClassS, 0x3100)
	putDesc(t, env, dynDesc, 0, dscClassD, 0)

	if got := call(t, env, "LIB$CHAR", 0x1000, 0x2000); got != ssNormal || bytesAt(t, env, 0x3000, 3) != "\xC1  " {
		t.Errorf("into S(3): %#x, %q", got, bytesAt(t, env, 0x3000, 3))
	}

	if got := call(t, env, "LIB$CHAR", 0x1008, 0x2000); got != libStrTru {
		t.Errorf("into S(0): %#x, want LIB$_STRTRU", got)
	}

	call(t, env, "LIB$CHAR", dynDesc, 0x2000)

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 1 || bytesAt(t, env, ptr, 1) != "\xC1" {
		t.Errorf("into D: length %d", length)
	}
}

// TestIchar: LIB$ICHAR is the first character, unsigned, or 0; no case
// change (the manual's Pascal example: 80 and 112).
func TestIchar(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "Pencil")
	putBytes(t, env, 0x3100, "pencil")
	putBytes(t, env, 0x3200, "\xE9")
	putDesc(t, env, 0x1000, 6, dscClassS, 0x3000)
	putDesc(t, env, 0x1008, 6, dscClassS, 0x3100)
	putDesc(t, env, 0x1010, 1, dscClassS, 0x3200)
	putDesc(t, env, 0x1018, 0, dscClassS, 0x3000)

	for _, c := range []struct{ desc, want uint32 }{{0x1000, 80}, {0x1008, 112}, {0x1010, 0xE9}, {0x1018, 0}} {
		if got := call(t, env, "LIB$ICHAR", c.desc); got != c.want {
			t.Errorf("LIB$ICHAR(%#x) = %d, want %d", c.desc, got, c.want)
		}
	}
}

// TestIndex: LIB$INDEX's positions, and the manual's rules for empty
// strings.
func TestIndex(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "ONE DAY AT A TIME")
	putBytes(t, env, 0x3100, "DAYTIMEXQ")
	putDesc(t, env, 0x1000, 17, dscClassS, 0x3000) // the source
	putDesc(t, env, 0x1008, 0, dscClassS, 0x3000)  // empty

	sub := func(addr, length uint32) uint32 {
		desc := 0x1100 + 0x10*(addr-0x3100)
		putDesc(t, env, desc, uint16(length), dscClassS, addr)

		return desc
	}

	cases := []struct {
		name     string
		src, sub uint32
		want     uint32
	}{
		{"DAY", 0x1000, sub(0x3100, 3), 5},
		{"TIME", 0x1000, sub(0x3103, 4), 14},
		{"XQ", 0x1000, sub(0x3107, 2), 0},
		{"empty substring", 0x1000, 0x1008, 1},
		{"both empty", 0x1008, 0x1008, 1},
		{"empty source", 0x1008, sub(0x3100, 3), 0},
	}

	for _, c := range cases {
		if got := call(t, env, "LIB$INDEX", c.src, c.sub); got != c.want {
			t.Errorf("%s: LIB$INDEX = %d, want %d", c.name, got, c.want)
		}
	}
}
