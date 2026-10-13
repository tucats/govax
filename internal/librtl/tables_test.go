package librtl

import "testing"

// TestTranslationTables: LIB$AB_ASC_EBC is one-to-one on ASCII (00-7F),
// LIB$AB_EBC_ASC is its inverse there, every other ASCII character goes
// to 3F, and every EBCDIC code outside ASC_EBC's image to 5C (FF to FF
// both ways). Spot checks from the manual's tables.
func TestTranslationTables(t *testing.T) {
	image := map[byte]bool{}

	for c := range 0x80 {
		e := asciiToEBCDIC[c]
		if image[e] {
			t.Errorf("ASCII %02X: EBCDIC %02X is another character's too", c, e)
		}

		image[e] = true

		if back := ebcdicToASCII[e]; back != byte(c) {
			t.Errorf("ASCII %02X -> EBCDIC %02X -> ASCII %02X", c, e, back)
		}
	}

	for c := 0x80; c < 0xFF; c++ {
		if asciiToEBCDIC[c] != 0x3F {
			t.Errorf("ASCII %02X -> %02X, want 3F", c, asciiToEBCDIC[c])
		}
	}

	for e := range 0xFF {
		if !image[byte(e)] && ebcdicToASCII[e] != 0x5C {
			t.Errorf("EBCDIC %02X -> %02X, want 5C", e, ebcdicToASCII[e])
		}
	}

	if asciiToEBCDIC[0xFF] != 0xFF || ebcdicToASCII[0xFF] != 0xFF {
		t.Error("FF doesn't translate to FF both ways")
	}

	for _, c := range []struct{ ascii, ebcdic byte }{{'A', 0xC1}, {'z', 0xA9}, {'0', 0xF0}, {' ', 0x40}, {'[', 0x4A}, {'|', 0x6A}, {'\\', 0xE0}} {
		if asciiToEBCDIC[c.ascii] != c.ebcdic {
			t.Errorf("%q -> %02X, want %02X", c.ascii, asciiToEBCDIC[c.ascii], c.ebcdic)
		}
	}
}

// TestUpcaseTable: LIB$AB_UPCASE as the manual prints it.
func TestUpcaseTable(t *testing.T) {
	for _, c := range []struct{ in, want byte }{
		{'a', 'A'}, {'z', 'Z'}, {'A', 'A'}, {'`', '`'}, {'{', '{'}, {0x7F, 0x7F},
		{0xE0, 0xC0}, {0xEF, 0xCF}, {0xF0, 0xF0}, {0xF7, 0xF7}, {0xF9, 0xF9},
		{0xFA, 0xDA}, {0xFD, 0xDD}, {0xFE, 0xFE}, {0xFF, 0xFF}, {0xC5, 0xC5},
	} {
		if upcase[c.in] != c.want {
			t.Errorf("upcase[%02X] = %02X, want %02X", c.in, upcase[c.in], c.want)
		}
	}
}

// TestTraAscEbc: LIB$TRA_ASC_EBC and LIB$TRA_EBC_ASC translate whole
// strings, report untranslatable characters (still translated) with
// LIB$_INVCHA, refuse a fixed destination of another length or a varying
// one too small with LIB$_INVARG (nothing written), and size a dynamic
// one to the source.
func TestTraAscEbc(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "Az09")
	putBytes(t, env, 0x3010, "A\x80\x1A")         // 80 has no EBCDIC; 1A (SUB) does
	putBytes(t, env, 0x3020, "\xC1\xA9\xF0\xF9") // "Az09" in EBCDIC
	putBytes(t, env, 0x3030, "\xC1\x04\xE0")     // 04 has no ASCII; E0 is backslash
	putDesc(t, env, 0x1000, 4, dscClassS, 0x3000)
	putDesc(t, env, 0x1008, 3, dscClassS, 0x3010)
	putDesc(t, env, 0x1010, 4, dscClassS, 0x3020)
	putDesc(t, env, 0x1018, 3, dscClassS, 0x3030)

	putDesc(t, env, 0x1100, 4, dscClassS, 0x4000)
	if got := call(t, env, "LIB$TRA_ASC_EBC", 0x1000, 0x1100); got != ssNormal || bytesAt(t, env, 0x4000, 4) != "\xC1\xA9\xF0\xF9" {
		t.Errorf("Az09: %#x, % X", got, bytesAt(t, env, 0x4000, 4))
	}

	putDesc(t, env, 0x1108, 3, dscClassS, 0x4100)
	if got := call(t, env, "LIB$TRA_ASC_EBC", 0x1008, 0x1108); got != libInvCha || bytesAt(t, env, 0x4100, 3) != "\xC1\x3F\x3F" {
		t.Errorf("A, 80, 1A: %#x, % X; want LIB$_INVCHA, C1 3F 3F", got, bytesAt(t, env, 0x4100, 3))
	}

	putDesc(t, env, 0x1110, 4, dscClassS, 0x4200)
	if got := call(t, env, "LIB$TRA_EBC_ASC", 0x1010, 0x1110); got != ssNormal || bytesAt(t, env, 0x4200, 4) != "Az09" {
		t.Errorf("EBCDIC Az09: %#x, %q", got, bytesAt(t, env, 0x4200, 4))
	}

	putDesc(t, env, 0x1118, 3, dscClassS, 0x4300)
	if got := call(t, env, "LIB$TRA_EBC_ASC", 0x1018, 0x1118); got != libInvCha || bytesAt(t, env, 0x4300, 3) != "A\\\\" {
		t.Errorf("C1, 04, E0: %#x, %q; want LIB$_INVCHA, A\\\\", got, bytesAt(t, env, 0x4300, 3))
	}

	// A fixed destination of another length: LIB$_INVARG, untouched.
	putBytes(t, env, 0x4400, "-----")
	putDesc(t, env, 0x1120, 5, dscClassS, 0x4400)

	if got := call(t, env, "LIB$TRA_ASC_EBC", 0x1000, 0x1120); got != libInvArg || bytesAt(t, env, 0x4400, 5) != "-----" {
		t.Errorf("into S(5): %#x, %q; want LIB$_INVARG, untouched", got, bytesAt(t, env, 0x4400, 5))
	}

	// A varying destination too small, then big enough.
	putDesc(t, env, 0x1128, 3, dscClassVS, 0x4500)
	if got := call(t, env, "LIB$TRA_ASC_EBC", 0x1000, 0x1128); got != libInvArg {
		t.Errorf("into VS(3): %#x, want LIB$_INVARG", got)
	}

	putDesc(t, env, 0x1128, 10, dscClassVS, 0x4500)
	if got := call(t, env, "LIB$TRA_ASC_EBC", 0x1000, 0x1128); got != ssNormal || longword(t, env, 0x4500)&0xFFFF != 4 {
		t.Errorf("into VS(10): %#x, CURLEN %d", got, longword(t, env, 0x4500)&0xFFFF)
	}

	// A dynamic destination takes the source's length.
	putDesc(t, env, dynDesc, 0, dscClassD, 0)
	call(t, env, "LIB$TRA_EBC_ASC", 0x1010, dynDesc)

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 4 || bytesAt(t, env, ptr, 4) != "Az09" {
		t.Errorf("into D: length %d, %q", length, bytesAt(t, env, ptr, int(length)))
	}
}
