package asm

import "testing"

// TestDisassembleAddressingModes decodes the same encodings
// TestAddressingModes hand-verified, checking the disassembler's text
// against what parseRegister/asm_operand's own conventions would produce —
// the decode-side half of the cross-check docs/PHASE-11.md calls for.
func TestDisassembleAddressingModes(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		want  string
	}{
		{"register direct", []byte{0xD4, 0x53}, "CLRL R3"},
		{"register deferred", []byte{0xD4, 0x63}, "CLRL (R3)"},
		{"autodecrement", []byte{0xD4, 0x73}, "CLRL -(R3)"},
		{"autoincrement", []byte{0xD4, 0x83}, "CLRL (R3)+"},
		{"autoincrement deferred", []byte{0xD4, 0x93}, "CLRL @(R3)+"},
		{"byte displacement", []byte{0xD4, 0xA3, 0x7F}, "CLRL B^7F(R3)"},
		{"byte displacement deferred", []byte{0xD4, 0xB3, 0x7F}, "CLRL @B^7F(R3)"},
		{"word displacement", []byte{0xD4, 0xC3, 0x34, 0x12}, "CLRL W^1234(R3)"},
		{"long displacement", []byte{0xD4, 0xE3, 0xEF, 0xCD, 0xAB, 0x89}, "CLRL L^89ABCDEF(R3)"},
		{"immediate", []byte{0xD4, 0x8F, 0x78, 0x56, 0x34, 0x12}, "CLRL I^#12345678"},
		{"absolute", []byte{0xD4, 0x9F, 0x78, 0x56, 0x34, 0x12}, "CLRL @#12345678"},
		{"short literal", []byte{0xD0, 0x05, 0x50}, "MOVL S^#05,R0"},
		{"indexed", []byte{0xD4, 0x43, 0xE2, 0x04, 0x00, 0x00, 0x00}, "CLRL L^00000004(R2)[R3]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dec, err := Disassemble(SliceReader(tc.bytes), 0)
			if err != nil {
				t.Fatalf("Disassemble: %v", err)
			}

			if got := dec.String(); got != tc.want {
				t.Errorf("Disassemble(% X) = %q, want %q", tc.bytes, got, tc.want)
			}

			if int(dec.Length) != len(tc.bytes) {
				t.Errorf("Length = %d, want %d", dec.Length, len(tc.bytes))
			}
		})
	}
}

// TestDisassembleBranch decodes a real OP_BR operand (not an addressing
// mode byte at all): the destination is PC-relative, computed from the
// byte following the displacement field.
func TestDisassembleBranch(t *testing.T) {
	// BEQL +5 at address 0: opcode(1)+disp(1)=2 bytes, so PC after the
	// operand is 2; destination = 2+5 = 7.
	bytes := []byte{0x13, 0x05}
	
	dec, err := Disassemble(SliceReader(bytes), 0)
	if err != nil {
		t.Fatal(err)
	}
	
	if got, want := dec.String(), "BEQL 00000007"; got != want {
		t.Errorf("Disassemble = %q, want %q", got, want)
	}
}

// TestRoundTripAddressingModes reassembles every Decoded.String() from
// TestDisassembleAddressingModes and checks it reproduces the same bytes —
// the round-trip property docs/PHASE-11.md's deliverables call for.
func TestRoundTripAddressingModes(t *testing.T) {
	cases := [][]byte{
		{0xD4, 0x53},
		{0xD4, 0x63},
		{0xD4, 0x73},
		{0xD4, 0x83},
		{0xD4, 0x93},
		{0xD4, 0xA3, 0x7F},
		{0xD4, 0xB3, 0x7F},
		{0xD4, 0xC3, 0x34, 0x12},
		{0xD4, 0xD3, 0x34, 0x12},
		{0xD4, 0xE3, 0xEF, 0xCD, 0xAB, 0x89},
		{0xD4, 0xF3, 0xEF, 0xCD, 0xAB, 0x89},
		{0xD4, 0x8F, 0x78, 0x56, 0x34, 0x12},
		{0xD4, 0x9F, 0x78, 0x56, 0x34, 0x12},
		{0xD0, 0x05, 0x50},
		{0xD4, 0x43, 0xE2, 0x04, 0x00, 0x00, 0x00},
		{0x13, 0x05}, // BEQL, an OP_BR operand
	}
	for _, want := range cases {
		dec, err := Disassemble(SliceReader(want), 0)
		if err != nil {
			t.Fatalf("Disassemble(% X): %v", want, err)
		}

		got := assembleBytes(t, dec.String())
		requireBytes(t, got, want...)
	}
}

// TestRoundTripFloatShortLiteral checks a floating short-literal decodes
// and reassembles to the same byte.
func TestRoundTripFloatShortLiteral(t *testing.T) {
	// ADDF2 S^#n, r0 -- the short-literal table's index 8 is 1.0.
	want := []byte{0x40, 0x08, 0x50}

	dec, err := Disassemble(SliceReader(want), 0)
	if err != nil {
		t.Fatal(err)
	}

	if dec.Operands[0] != "S^#1" {
		t.Errorf("operand = %q, want S^#1", dec.Operands[0])
	}

	got := assembleBytes(t, dec.String())
	requireBytes(t, got, want...)
}

func TestDisassembleInvalidOpcode(t *testing.T) {
	if _, err := Disassemble(SliceReader{0xFF, 0xFF}, 0); err == nil {
		t.Fatal("expected an error for an undefined extended opcode")
	}
}
