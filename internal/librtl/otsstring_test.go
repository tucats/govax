package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// TestOTSMove: OTS$MOVE3 moves as MOVC3 does, overlap and all, and
// OTS$MOVE5 as MOVC5, filling or returning the bytes it didn't move; the
// JSB entries leave the registers as the instructions do.
func TestOTSMove(t *testing.T) {
	env := fixture(t)

	// Overlapping, destination above the source.
	putBytes(t, env, 0x3000, "ABCDEFGH")

	if r0 := call(t, env, "OTS$MOVE3", 5, 0x3000, 0x3002); r0 != 0 || bytesAt(t, env, 0x3000, 8) != "ABABCDEH" {
		t.Errorf("MOVE3 overlapping: R0 %#x, %q; want 0, \"ABABCDEH\"", r0, bytesAt(t, env, 0x3000, 8))
	}

	putBytes(t, env, 0x3100, "ABC")
	putBytes(t, env, 0x3200, "xxxxxxx")

	if r0 := call(t, env, "OTS$MOVE5", 3, 0x3100, '*', 6, 0x3200); r0 != 0 || bytesAt(t, env, 0x3200, 7) != "ABC***x" {
		t.Errorf("MOVE5 filled: R0 %#x, %q", r0, bytesAt(t, env, 0x3200, 7))
	}

	if r0 := call(t, env, "OTS$MOVE5", 3, 0x3100, '*', 2, 0x3300); r0 != 1 || bytesAt(t, env, 0x3300, 2) != "AB" {
		t.Errorf("MOVE5 short: R0 %#x, %q; want 1 not moved", r0, bytesAt(t, env, 0x3300, 2))
	}

	putBytes(t, env, 0x3400, "........")

	regs := callJSB(t, env, "OTS$MOVE3_R5", 3, 0x3100, 0, 0x3400, 0, 0, 0x66)
	if want := (corevms.JSBRegisters{0, 0x3103, 0, 0x3403, 0, 0, 0x66}); regs != want || bytesAt(t, env, 0x3400, 4) != "ABC." {
		t.Errorf("MOVE3_R5: %08X, %q; want %08X", regs, bytesAt(t, env, 0x3400, 4), want)
	}

	regs = callJSB(t, env, "OTS$MOVE5_R5", 3, 0x3100, '-', 5, 0x3500, 0, 0x66)
	if want := (corevms.JSBRegisters{0, 0x3103, 0, 0x3505, 0, 0, 0x66}); regs != want || bytesAt(t, env, 0x3500, 5) != "ABC--" {
		t.Errorf("MOVE5_R5: %08X, %q; want %08X", regs, bytesAt(t, env, 0x3500, 5), want)
	}
}

// TestOTSCopy: OTS$SCOPY_DXDX and OTS$SCOPY_R_DX return the bytes they
// didn't copy (truncation is no error), take the length by value, fill
// a dynamic string, and signal a bad descriptor; the JSB entries leave
// MOVC5's registers, R6 alone.
func TestOTSCopy(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "HELLO, WORLD")
	putDesc(t, env, 0x1000, 12, dscClassS, 0x3000)

	putDesc(t, env, 0x1100, 5, dscClassS, 0x4000)

	if r0 := call(t, env, "OTS$SCOPY_DXDX", 0x1000, 0x1100); r0 != 7 || bytesAt(t, env, 0x4000, 5) != "HELLO" {
		t.Errorf("into S(5): R0 %d, %q; want 7 not copied", r0, bytesAt(t, env, 0x4000, 5))
	}

	putDesc(t, env, dynDesc, 0, dscClassD, 0)

	if r0 := call(t, env, "OTS$SCOPY_R_DX", 5, 0x3007, dynDesc); r0 != 0 {
		t.Errorf("into D: R0 %d", r0)
	}

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 5 || bytesAt(t, env, ptr, 5) != "WORLD" {
		t.Errorf("D after the copy: length %d, %q", length, bytesAt(t, env, ptr, 5))
	}

	putDesc(t, env, 0x1108, 8, dscClassS, 0x4100)

	regs := callJSB(t, env, "OTS$SCOPY_DXDX6", 0x1000, 0x1108, 0, 0, 0, 0, 0x66)
	if want := (corevms.JSBRegisters{4, 0x3008, 0, 0x4108, 0, 0, 0x66}); regs != want {
		t.Errorf("SCOPY_DXDX6: %08X; want %08X", regs, want)
	}

	// A class no string has: signaled. (The fixture can't deliver it,
	// so the signal fails with an error.)
	putDesc(t, env, 0x1110, 3, 99, 0x3000)

	if _, err := shim(t, env, "OTS$SCOPY_DXDX")(env, []uint32{0x1110, 0x1100}); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("bad class: err = %v, want a signal", err)
	}
}

// TestOTSDynamic: OTS$SGET1_DD takes its length by value, and
// OTS$SFREE1_DD and OTS$SFREEN_DD free and empty the descriptors.
func TestOTSDynamic(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, dynDesc, 0, 0, 0)
	putDesc(t, env, dynDesc+8, 0, 0, 0)

	if r0 := call(t, env, "OTS$SGET1_DD", 10, dynDesc); r0 != ssNormal {
		t.Fatalf("SGET1_DD: R0 %#x", r0)
	}

	if length, class, ptr := descriptorAt(t, env, dynDesc); length != 10 || class != dscClassD || ptr == 0 {
		t.Fatalf("after SGET1_DD(10): length %d, class %d, pointer %#x", length, class, ptr)
	}

	regs := callJSB(t, env, "OTS$SGET1_DD_R6", 4, dynDesc+8)
	if length, _, ptr := descriptorAt(t, env, dynDesc+8); regs[0] != ssNormal || length != 4 || ptr == 0 {
		t.Fatalf("SGET1_DD_R6: R0 %#x, length %d, pointer %#x", regs[0], length, ptr)
	}

	if r0 := call(t, env, "OTS$SFREEN_DD", 2, dynDesc); r0 != ssNormal {
		t.Errorf("SFREEN_DD: R0 %#x", r0)
	}

	for _, d := range []uint32{dynDesc, dynDesc + 8} {
		if length, _, ptr := descriptorAt(t, env, d); length != 0 || ptr != 0 {
			t.Errorf("descriptor %#x after SFREEN_DD: length %d, pointer %#x; want both 0", d, length, ptr)
		}
	}

	// Storage that isn't a dynamic string's: signaled.
	putDesc(t, env, dynDesc, 4, dscClassD, 0x3000)

	if _, err := shim(t, env, "OTS$SFREE1_DD")(env, []uint32{dynDesc}); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("static storage: err = %v, want a signal", err)
	}
}
