package asm

import "testing"

// assembleBytes assembles src with a fresh Assembler and returns the
// resulting P0-region bytes, failing the test on any error.
func assembleBytes(t *testing.T, src string) []byte {
	t.Helper()
	a := New()
	out, err := a.Assemble(src)
	if err != nil {
		t.Fatalf("assemble %q: %v", src, err)
	}
	return out
}

// assembleBytesAt is assembleBytes, but at a chosen origin — needed for
// round-tripping a disassembled PC-relative branch/displacement operand,
// whose encoding depends on the address it's assembled at.
func assembleBytesAt(t *testing.T, origin uint32, src string) []byte {
	t.Helper()
	a := New()
	a.SetOrigin(origin)
	out, err := a.Assemble(src)
	if err != nil {
		t.Fatalf("assemble %q at %#x: %v", src, origin, err)
	}
	return out
}

func requireBytes(t *testing.T, got []byte, want ...byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d bytes % X, want %d bytes % X", len(got), got, len(want), want)
	}
	
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d = %#02X, want %#02X (got % X, want % X)", i, got[i], want[i], got, want)
		}
	}
}

// TestAddressingModes hand-verifies one instruction's encoding per VAX
// addressing mode against the ISA manual's mode-byte layout (mode nibble
// 0-F in the high bits, register number in the low 4 bits, following data
// per mode) — the docs/PHASE-11.md-mandated cross-check, done here per
// addressing mode rather than only against the named small fixtures.
func TestAddressingModes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []byte
	}{
		{"register direct", "CLRL R3", []byte{0xD4, 0x53}},
		{"register deferred", "CLRL (R3)", []byte{0xD4, 0x63}},
		{"autodecrement", "CLRL -(R3)", []byte{0xD4, 0x73}},
		{"autoincrement", "CLRL (R3)+", []byte{0xD4, 0x83}},
		{"autoincrement deferred", "CLRL @(R3)+", []byte{0xD4, 0x93}},
		{"the '@(Rn)' zero-byte-disp-deferred quirk", "CLRL @(R3)", []byte{0xD4, 0xB3, 0x00}},
		{"byte displacement", "CLRL B^7F(R3)", []byte{0xD4, 0xA3, 0x7F}},
		{"byte displacement deferred", "CLRL @B^7F(R3)", []byte{0xD4, 0xB3, 0x7F}},
		{"word displacement", "CLRL W^1234(R3)", []byte{0xD4, 0xC3, 0x34, 0x12}},
		{"word displacement deferred", "CLRL @W^1234(R3)", []byte{0xD4, 0xD3, 0x34, 0x12}},
		{"long displacement", "CLRL L^89ABCDEF(R3)", []byte{0xD4, 0xE3, 0xEF, 0xCD, 0xAB, 0x89}},
		{"long displacement deferred", "CLRL @L^89ABCDEF(R3)", []byte{0xD4, 0xF3, 0xEF, 0xCD, 0xAB, 0x89}},
		{"immediate", "CLRL I^#12345678", []byte{0xD4, 0x8F, 0x78, 0x56, 0x34, 0x12}},
		{"absolute", "CLRL @#12345678", []byte{0xD4, 0x9F, 0x78, 0x56, 0x34, 0x12}},
		{"short literal", "MOVL #5,R0", []byte{0xD0, 0x05, 0x50}},
		{"literal too big becomes immediate", "MOVL #100,R0", []byte{0xD0, 0x8F, 0x00, 0x01, 0x00, 0x00, 0x50}},
		{"explicit short literal", "MOVL S^#5,R0", []byte{0xD0, 0x05, 0x50}},
		{"explicit immediate", "MOVL I^#5,R0", []byte{0xD0, 0x8F, 0x05, 0x00, 0x00, 0x00, 0x50}},
		{"indexed", "CLRL 4(R2)[R3]", []byte{0xD4, 0x43, 0xE2, 0x04, 0x00, 0x00, 0x00}},
		{"bare symbol falls back to absolute", "CLRL FOO\nFOO: .LONG 0", []byte{0xD4, 0x9F, 0x06, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"bare symbol with (Rn) becomes long displacement", "CLRL 4(R2)", []byte{0xD4, 0xE2, 0x04, 0x00, 0x00, 0x00}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assembleBytes(t, tc.src)
			requireBytes(t, got, tc.want...)
		})
	}
}

// TestIndexedModeFollowedByAnotherOperand hand-verifies a regression this
// project's own Phase 12 integration testing found: assembleOperandRec's
// index-prefix lookahead (an operand written as "BASE[Rx]") writes the
// index byte, then recurses to parse BASE alone -- but several BASE-mode
// branches (register deferred, "(Rn)", among them) returned as soon as the
// base itself was consumed, without checking for and skipping the trailing
// "[Rx]" text still sitting in the cursor. For a single-operand instruction
// (TestAddressingModes' own "indexed" case, CLRL 4(R2)[R3]) there's nothing
// after to corrupt, so this was invisible; testdata/asm/kernel.asm's own
// EXE$DISPATCH ("movl (r3)[r2], r0", a real, working microkernel routine)
// is a genuine multi-operand instance that surfaced it: the leftover
// "[r2]" text got reinterpreted as the start of the destination operand,
// corrupting the whole instruction's encoding.
func TestIndexedModeFollowedByAnotherOperand(t *testing.T) {
	got := assembleBytes(t, "MOVL (R3)[R2], R0")
	requireBytes(t, got, 0xD0, 0x42, 0x63, 0x50)
}

// TestInsv hand-verifies insv.asm's own instructions byte-for-byte against
// the ISA manual, as docs/PHASE-11.md calls for by name.
func TestInsv(t *testing.T) {
	// movl #14, r0 ; insv #0d, r0, #3, @#bits ; ret
	// Every numeric literal here is read under the default hex radix
	// (no ^D/^X prefix): "14" is 0x14=20, and "0D" is 0x0D=13 -- both
	// small enough to encode as short literals rather than I^# immediates.
	got := assembleBytes(t, "MOVL #14, R0\nINSV #0D, R0, #3, @#BITS\nRET\nBITS: .LONG 0\n.LONG 0\n")

	want := []byte{
		0xD0, 0x14, 0x50, // MOVL S^#14(hex 0x14=20), R0
		0xF0,                         // INSV
		0x0D,                         // #0D -> hex 0x0D=13 -> short literal 0x0D
		0x50,                         // R0
		0x03,                         // #3 -> short literal 0x03
		0x9F, 0x0D, 0x02, 0x00, 0x00, // @#BITS (BITS = 0x20D, forward ref resolved)
		0x04,                   // RET
		0, 0, 0, 0, 0, 0, 0, 0, // BITS: .LONG 0 / .LONG 0
	}
	requireBytes(t, got, want...)
}

// TestMovc3Prologue hand-verifies movc3.asm's .ENTRY mask and MOVAL/MOVC3
// instructions, up to (but not including) the forward-referenced SRC/DST
// addresses, matching docs/PHASE-11.md's call to hand-verify movc3.asm.
func TestMovc3Prologue(t *testing.T) {
	src := "\t.entry\tmain, ^m<r2,r3,r4,r5>\n\n\tmoval\t@#src, r1\n\tmovc3\t#5,(r1),(r3)\n\tmovl\t#1, r0\n\tret\n\nsrc:\t.ascii \"x\"\n\n\t.end\tmain\n"
	got := assembleBytes(t, src)

	want := []byte{
		0x3C, 0x00, // .ENTRY mask: R2,R3,R4,R5 = bits 2-5 = 0x3C
		0xDE, 0x9F, 0x11, 0x02, 0x00, 0x00, 0x51, // MOVAL @#SRC,R1 (SRC = 0x211)
		0x28, 0x05, 0x61, 0x63, // MOVC3 #5,(R1),(R3)
		0xD0, 0x01, 0x50, // MOVL #1,R0
		0x04, // RET
		'x',  // SRC: .ASCII "x"
	}
	requireBytes(t, got, want...)
}

func TestOperandCommentsStripped(t *testing.T) {
	got := assembleBytes(t, "CLRL R0 ; comment\n")
	requireBytes(t, got, 0xD4, 0x50)
}
