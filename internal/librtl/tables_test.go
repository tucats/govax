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
