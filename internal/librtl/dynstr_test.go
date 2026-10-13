package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// dynDesc is where the dynamic-string tests keep their descriptors: well
// above the fixture's heap, which grows up from 0x200.
const dynDesc = 0x80000

// descriptorAt reads the length, class, and pointer of the descriptor at
// addr.
func descriptorAt(t *testing.T, env *corevms.Environment, addr uint32) (length uint32, class byte, ptr uint32) {
	t.Helper()

	first := longword(t, env, addr)

	return first & 0xFFFF, byte(first >> 24), longword(t, env, addr+4)
}

// TestSget1DD: LIB$SGET1_DD sets the class, length, and pointer; storage
// that's big enough is kept, and storage that isn't is freed and replaced;
// a length of 0 still allocates.
func TestSget1DD(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, dynDesc, 0) // length 0, type 0, class 0 (checked by nobody)
	putLongword(t, env, dynDesc+4, 0)
	putLongword(t, env, dynDesc+16, 10) // the length argument (a word)

	if got := call(t, env, "LIB$SGET1_DD", dynDesc+16, dynDesc); got != ssNormal {
		t.Fatalf("status %#x, want SS$_NORMAL", got)
	}

	length, class, first := descriptorAt(t, env, dynDesc)
	if size, ok := env.BlockSize(first); length != 10 || class != dscClassD || !ok || size < 10 {
		t.Fatalf("after SGET1_DD(10): length %d, class %d, pointer %#x (a block of %d: %v)", length, class, first, size, ok)
	}

	// Shorter: the same storage.
	putLongword(t, env, dynDesc+16, 5)
	call(t, env, "LIB$SGET1_DD", dynDesc+16, dynDesc)

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 5 || ptr != first {
		t.Errorf("after SGET1_DD(5): length %d, pointer %#x; want 5 and %#x, kept", length, ptr, first)
	}

	// Within the block's real size (10 rounds up), though longer than the
	// descriptor's length: still the same storage.
	size, _ := env.BlockSize(first)
	putLongword(t, env, dynDesc+16, size)
	call(t, env, "LIB$SGET1_DD", dynDesc+16, dynDesc)

	if _, _, ptr := descriptorAt(t, env, dynDesc); ptr != first {
		t.Errorf("after SGET1_DD(%d), the block's size: pointer %#x, want %#x, kept", size, ptr, first)
	}

	// Longer than the block: new storage, the old freed (the heap may
	// hand the same address back, the freed block joined to its free
	// neighbor).
	putLongword(t, env, dynDesc+16, 200)
	call(t, env, "LIB$SGET1_DD", dynDesc+16, dynDesc)

	length, _, ptr := descriptorAt(t, env, dynDesc)
	if size, ok := env.BlockSize(ptr); length != 200 || !ok || size < 200 {
		t.Errorf("after SGET1_DD(200): length %d, pointer %#x, a block of %d (%v); want 200 bytes' room", length, ptr, size, ok)
	}

	// Length 0, from no storage: some is allocated anyway.
	putLongword(t, env, dynDesc+32, 0)
	putLongword(t, env, dynDesc+36, 0)
	putLongword(t, env, dynDesc+16, 0)
	call(t, env, "LIB$SGET1_DD", dynDesc+16, dynDesc+32)

	if length, class, ptr := descriptorAt(t, env, dynDesc+32); length != 0 || class != dscClassD || ptr == 0 {
		t.Errorf("after SGET1_DD(0): length %d, class %d, pointer %#x; want 0, 2, and storage", length, class, ptr)
	}
}

// TestSfreeDD: LIB$SFREE1_DD frees and zeroes; a pointer that isn't the
// heap's is LIB$_FATERRLIB, the descriptor untouched; a zero pointer is
// fine. LIB$SFREEN_DD frees a run of descriptors.
func TestSfreeDD(t *testing.T) {
	env := fixture(t)

	for i := range uint32(3) {
		putDesc(t, env, dynDesc+8*i, 0, dscClassD, 0)
		putLongword(t, env, dynDesc+0x100, 20+i)
		call(t, env, "LIB$SGET1_DD", dynDesc+0x100, dynDesc+8*i)
	}

	_, _, first := descriptorAt(t, env, dynDesc)

	if got := call(t, env, "LIB$SFREE1_DD", dynDesc); got != ssNormal {
		t.Errorf("SFREE1_DD: %#x, want SS$_NORMAL", got)
	}

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 0 || ptr != 0 {
		t.Errorf("after SFREE1_DD: length %d, pointer %#x; want 0 and 0", length, ptr)
	}

	if _, ok := env.BlockSize(first); ok {
		t.Error("SFREE1_DD left the block allocated")
	}

	if got := call(t, env, "LIB$SFREE1_DD", dynDesc); got != ssNormal {
		t.Errorf("SFREE1_DD of an empty descriptor: %#x, want SS$_NORMAL", got)
	}

	putDesc(t, env, dynDesc+0x40, 4, dscClassD, 0x90000) // not the heap's
	if got := call(t, env, "LIB$SFREE1_DD", dynDesc+0x40); got != libFatErrLib {
		t.Errorf("SFREE1_DD of static storage: %#x, want LIB$_FATERRLIB", got)
	}

	if length, _, ptr := descriptorAt(t, env, dynDesc+0x40); length != 4 || ptr != 0x90000 {
		t.Errorf("after LIB$_FATERRLIB: length %d, pointer %#x; want it untouched", length, ptr)
	}

	putLongword(t, env, dynDesc+0x100, 2)

	if got := call(t, env, "LIB$SFREEN_DD", dynDesc+0x100, dynDesc+8); got != ssNormal {
		t.Errorf("SFREEN_DD: %#x, want SS$_NORMAL", got)
	}

	for i := uint32(1); i < 3; i++ {
		if length, _, ptr := descriptorAt(t, env, dynDesc+8*i); length != 0 || ptr != 0 {
			t.Errorf("descriptor %d after SFREEN_DD: length %d, pointer %#x", i, length, ptr)
		}
	}
}

// TestDynamicJSB: the JSB entries take their arguments in registers, by
// value, and return the status in R0, leaving the other registers alone.
func TestDynamicJSB(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, dynDesc, 0, 0, 0)
	putDesc(t, env, dynDesc+8, 0, 0, 0)

	// The length is R0's low word: the high word is ignored.
	regs := callJSB(t, env, "LIB$SGET1_DD_R6", 0xABCD0007, dynDesc, 2, 3, 4, 5, 6)
	if regs != (corevms.JSBRegisters{ssNormal, dynDesc, 2, 3, 4, 5, 6}) {
		t.Errorf("SGET1_DD_R6 left %v", regs)
	}

	if length, class, ptr := descriptorAt(t, env, dynDesc); length != 7 || class != dscClassD || ptr == 0 {
		t.Errorf("after SGET1_DD_R6(7): length %d, class %d, pointer %#x", length, class, ptr)
	}

	callJSB(t, env, "LIB$SGET1_DD_R6", 3, dynDesc+8)

	if regs := callJSB(t, env, "LIB$SFREE1_DD6", dynDesc); regs[0] != ssNormal {
		t.Errorf("SFREE1_DD6: R0 %#x", regs[0])
	}

	if _, _, ptr := descriptorAt(t, env, dynDesc); ptr != 0 {
		t.Errorf("after SFREE1_DD6: pointer %#x", ptr)
	}

	callJSB(t, env, "LIB$SGET1_DD_R6", 3, dynDesc)

	if regs := callJSB(t, env, "LIB$SFREEN_DD6", 2, dynDesc); regs[0] != ssNormal {
		t.Errorf("SFREEN_DD6: R0 %#x", regs[0])
	}

	for i := range uint32(2) {
		if _, _, ptr := descriptorAt(t, env, dynDesc+8*i); ptr != 0 {
			t.Errorf("descriptor %d after SFREEN_DD6: pointer %#x", i, ptr)
		}
	}
}

// TestWriteStringDynamicKeepsBlock: writing a dynamic string keeps its
// storage while the heap block holds the text, as LIB$SGET1_DD does.
func TestWriteStringDynamicKeepsBlock(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, dynDesc, 0, dscClassD, 0)

	if _, _, err := writeString(env, dynDesc, "HELLO, WORLD"); err != nil {
		t.Fatal(err)
	}

	_, _, first := descriptorAt(t, env, dynDesc)

	if _, _, err := writeString(env, dynDesc, "HI"); err != nil {
		t.Fatal(err)
	}

	// Longer than the descriptor's length now, but it fits the block.
	if _, _, err := writeString(env, dynDesc, "HELLO, THERE"); err != nil {
		t.Fatal(err)
	}

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 12 || ptr != first || bytesAt(t, env, ptr, 12) != "HELLO, THERE" {
		t.Errorf("length %d, pointer %#x (first %#x); want the same block", length, ptr, first)
	}
}
