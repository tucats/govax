package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// TestScopyDxdx: LIB$SCOPY_DXDX copies into each destination class: S
// padded and truncated (LIB$_STRTRU), D given the source's length, VS up
// to its MAXSTRLEN; a varying source is read by CURLEN; a bad class is
// LIB$_INVSTRDES.
func TestScopyDxdx(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "HELLO, WORLD")
	putDesc(t, env, 0x1000, 5, dscClassS, 0x3000)  // "HELLO"
	putDesc(t, env, 0x1010, 12, dscClassS, 0x3000) // "HELLO, WORLD"

	// S, longer: padded.
	putDesc(t, env, 0x1100, 8, dscClassS, 0x4000)
	if got := call(t, env, "LIB$SCOPY_DXDX", 0x1000, 0x1100); got != ssNormal || bytesAt(t, env, 0x4000, 8) != "HELLO   " {
		t.Errorf("into S(8): %#x, %q", got, bytesAt(t, env, 0x4000, 8))
	}

	// S, shorter: truncated.
	putDesc(t, env, 0x1108, 3, dscClassS, 0x4100)
	if got := call(t, env, "LIB$SCOPY_DXDX", 0x1000, 0x1108); got != libStrTru || bytesAt(t, env, 0x4100, 3) != "HEL" {
		t.Errorf("into S(3): %#x, %q; want LIB$_STRTRU, \"HEL\"", got, bytesAt(t, env, 0x4100, 3))
	}

	// D: the source's length.
	putDesc(t, env, dynDesc, 0, dscClassD, 0)
	if got := call(t, env, "LIB$SCOPY_DXDX", 0x1010, dynDesc); got != ssNormal {
		t.Errorf("into D: %#x", got)
	}

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 12 || bytesAt(t, env, ptr, 12) != "HELLO, WORLD" {
		t.Errorf("D after the copy: length %d, %q", length, bytesAt(t, env, ptr, 12))
	}

	// VS with MAXSTRLEN 6: six characters, CURLEN 6, truncated.
	putDesc(t, env, 0x1110, 6, dscClassVS, 0x4200)
	if got := call(t, env, "LIB$SCOPY_DXDX", 0x1010, 0x1110); got != libStrTru {
		t.Errorf("into VS(6): %#x, want LIB$_STRTRU", got)
	}

	if cur, text := longword(t, env, 0x4200)&0xFFFF, bytesAt(t, env, 0x4202, 6); cur != 6 || text != "HELLO," {
		t.Errorf("VS after the copy: CURLEN %d, %q", cur, text)
	}

	// From that VS (CURLEN 6) into an S of 6.
	putDesc(t, env, 0x1118, 6, dscClassS, 0x4300)
	if got := call(t, env, "LIB$SCOPY_DXDX", 0x1110, 0x1118); got != ssNormal || bytesAt(t, env, 0x4300, 6) != "HELLO," {
		t.Errorf("from VS: %#x, %q", got, bytesAt(t, env, 0x4300, 6))
	}

	// Class 3 (V, not a string's), either side.
	putDesc(t, env, 0x1120, 4, 3, 0x3000)
	if got := call(t, env, "LIB$SCOPY_DXDX", 0x1120, 0x1100); got != libInvStrDes {
		t.Errorf("from class 3: %#x, want LIB$_INVSTRDES", got)
	}

	if got := call(t, env, "LIB$SCOPY_DXDX", 0x1000, 0x1120); got != libInvStrDes {
		t.Errorf("into class 3: %#x, want LIB$_INVSTRDES", got)
	}
}

// TestScopyRDx: LIB$SCOPY_R_DX takes the length (a word, by reference)
// and the text's address.
func TestScopyRDx(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "ABCDEFGH")
	putLongword(t, env, 0x1000, 0xFFFF0004) // length 4; the high word isn't part of it
	putDesc(t, env, 0x1100, 6, dscClassS, 0x4000)

	if got := call(t, env, "LIB$SCOPY_R_DX", 0x1000, 0x3002, 0x1100); got != ssNormal || bytesAt(t, env, 0x4000, 6) != "CDEF  " {
		t.Errorf("%#x, %q; want SS$_NORMAL, \"CDEF  \"", got, bytesAt(t, env, 0x4000, 6))
	}
}

// TestScopyJSB: the JSB entries take descriptors' addresses, and
// SCOPY_R_DX6 its length by value in R0's low word; only R0 changes.
func TestScopyJSB(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "ABCDEFGH")
	putDesc(t, env, 0x1000, 3, dscClassS, 0x3000)
	putDesc(t, env, 0x1100, 3, dscClassS, 0x4000)
	putDesc(t, env, 0x1108, 2, dscClassS, 0x4100)

	if regs := callJSB(t, env, "LIB$SCOPY_DXDX6", 0x1000, 0x1100, 2, 3, 4, 5, 6); regs != (corevms.JSBRegisters{ssNormal, 0x1100, 2, 3, 4, 5, 6}) || bytesAt(t, env, 0x4000, 3) != "ABC" {
		t.Errorf("SCOPY_DXDX6: registers %v, %q", regs, bytesAt(t, env, 0x4000, 3))
	}

	if regs := callJSB(t, env, "LIB$SCOPY_R_DX6", 0x10005, 0x3003, 0x1108); regs[0] != libStrTru || bytesAt(t, env, 0x4100, 2) != "DE" {
		t.Errorf("SCOPY_R_DX6: R0 %#x, %q; want LIB$_STRTRU, \"DE\"", regs[0], bytesAt(t, env, 0x4100, 2))
	}
}

// TestAnalyzeSdesc: LIB$ANALYZE_SDESC by class (the manual's table): S
// as it is, A by ARSIZE, VS by CURLEN with the text after the length
// word; an invalid class (3, V) writes nothing. The JSB entry returns R1 and R2.
func TestAnalyzeSdesc(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, 0x1000, 5, dscClassS, 0x3000)
	putDesc(t, env, 0x1010, 2, dscClassA, 0x3100)
	putLongword(t, env, 0x101C, 300) // ARSIZE
	putDesc(t, env, 0x1030, 40, dscClassVS, 0x3200)
	putLongword(t, env, 0x3200, 7) // CURLEN
	putDesc(t, env, 0x1040, 5, 3, 0x3000)

	cases := []struct {
		desc, length, text uint32
	}{
		{0x1000, 5, 0x3000},
		{0x1010, 300, 0x3100},
		{0x1030, 7, 0x3202},
	}

	for _, c := range cases {
		putLongword(t, env, 0x2000, 0xFFFFFFFF)
		putLongword(t, env, 0x2004, 0)

		if got := call(t, env, "LIB$ANALYZE_SDESC", c.desc, 0x2000, 0x2004); got != ssNormal {
			t.Errorf("%#x: %#x", c.desc, got)
		}

		// The length is a word: the next word stays as it was.
		if length, text := longword(t, env, 0x2000), longword(t, env, 0x2004); length != 0xFFFF0000|c.length || text != c.text {
			t.Errorf("%#x: length longword %#x, address %#x; want %#x, %#x", c.desc, length, text, 0xFFFF0000|c.length, c.text)
		}

		if regs := callJSB(t, env, "LIB$ANALYZE_SDESC_R2", c.desc, 1, 2, 3); regs != (corevms.JSBRegisters{ssNormal, c.length, c.text, 3}) {
			t.Errorf("%#x: ANALYZE_SDESC_R2 left %v", c.desc, regs)
		}
	}

	putLongword(t, env, 0x2004, 0x1234)

	if got := call(t, env, "LIB$ANALYZE_SDESC", 0x1040, 0x2000, 0x2004); got != libInvStrDes || longword(t, env, 0x2004) != 0x1234 {
		t.Errorf("class 3: %#x, address %#x; want LIB$_INVSTRDES, nothing written", got, longword(t, env, 0x2004))
	}

	if regs := callJSB(t, env, "LIB$ANALYZE_SDESC_R2", 0x1040, 1, 2); regs != (corevms.JSBRegisters{libInvStrDes, 1, 2}) {
		t.Errorf("class 3: ANALYZE_SDESC_R2 left %v", regs)
	}
}
