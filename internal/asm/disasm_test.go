package asm

import (
	"testing"

	"github.com/tucats/govax/internal/disasm"
)

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
		{0xDD, 0x8F, 0x78, 0x56, 0x34, 0x12}, // PUSHL: CLRL can't write an immediate
		{0xD4, 0x9F, 0x78, 0x56, 0x34, 0x12},
		{0xD0, 0x05, 0x50},
		{0xD4, 0x43, 0xE2, 0x04, 0x00, 0x00, 0x00},
		{0x13, 0x05}, // BEQL, an OP_BR operand
	}
	for _, want := range cases {
		dec, err := disasm.Disassemble(disasm.SliceReader(want), 0)
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

	dec, err := disasm.Disassemble(disasm.SliceReader(want), 0)
	if err != nil {
		t.Fatal(err)
	}

	if dec.Operands[0].String() != "S^#1" {
		t.Errorf("operand = %q, want S^#1", dec.Operands[0].String())
	}

	got := assembleBytes(t, dec.String())
	requireBytes(t, got, want...)
}
