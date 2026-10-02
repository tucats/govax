package asm

import "testing"

// Floating literals, immediates, and directives in each format (docs/
// PHASE-35.md, subtasks 6 and 7). Each value is rounded once, from its
// decimal form, to the operand's own format, as VAX MACRO rounds it. The
// D_floating bits of 1.1 are VAX MACRO's own (the Phase 35 probe's MOVD
// #1.1, testdata/insn35); the others follow from the formats: 1.1 is
// binary 1.000110011001100..., its fraction bits a repeating ^X9 (from
// the fourth fraction bit on), rounded up in the last place.

var (
	f11 = []byte{0x8C, 0x40, 0xCD, 0xCC}                                            // ^X408C, ^XCCCD
	d11 = []byte{0x8C, 0x40, 0xCC, 0xCC, 0xCC, 0xCC, 0xCD, 0xCC}                    // ^X408C, ^XCCCC, ^XCCCC, ^XCCCD
	g11 = []byte{0x11, 0x40, 0x99, 0x99, 0x99, 0x99, 0x9A, 0x99}                    // ^X4011, ^X9999, ^X9999, ^X999A
	h11 = cat([]byte{0x01, 0x40, 0x99, 0x19}, repeat(0x99, 10), []byte{0x9A, 0x99}) // ^X4001, ^X1999, ^X9999 x5, ^X999A
)

func repeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}

	return out
}

// TestFloatingImmediates checks each format's immediate and short literal
// encodings.
func TestFloatingImmediates(t *testing.T) {
	r0 := []byte{0x50}

	cases := []struct {
		name, src string
		want      []byte
	}{
		{"F immediate", "MOVF #1.1,R0", cat([]byte{0x50, 0x8F}, f11, r0)},
		{"D immediate, rounded once", "MOVD #1.1,R0", cat([]byte{0x70, 0x8F}, d11, r0)},
		{"G immediate", "MOVG #1.1,R0", cat([]byte{0xFD, 0x50, 0x8F}, g11, r0)},
		{"H immediate", "MOVH #1.1,R0", cat([]byte{0xFD, 0x70, 0x8F}, h11, r0)},
		// A short literal is the same 6 bits in every format; the CPU
		// expands it in the operand's format.
		{"G literal", "MOVG #0.5,R0", []byte{0xFD, 0x50, 0x00, 0x50}},
		{"H literal", "MOVH #120.0,R0", []byte{0xFD, 0x70, 0x3F, 0x50}},
		{"G integer value as a literal", "ADDG2 #3,R0", []byte{0xFD, 0x40, 0x14, 0x50}},
		// A literal on an integer operand of a floating instruction is an
		// integer: CVTLG's source, EMODG's extension word.
		{"CVTLG's integer source", "CVTLG #3,R0", []byte{0xFD, 0x4E, 0x03, 0x50}},
		{"EMODG's integer extension", "EMODG #1.0,#5,#1.0,R0,R2",
			[]byte{0xFD, 0x54, 0x08, 0x05, 0x08, 0x50, 0x52}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

// TestFloatingDirectives checks .F_FLOATING through .H_FLOATING.
func TestFloatingDirectives(t *testing.T) {
	cases := []struct {
		src  string
		want []byte
	}{
		{".F_FLOATING 1.1", f11},
		{".D_FLOATING 1.1", d11},
		{".G_FLOATING 1.1, -0.5", cat(g11, []byte{0x00, 0xC0}, zeros(6))},
		{".H_FLOATING 1.1", h11},
		{".H_FLOATING 1.5", cat([]byte{0x01, 0x40, 0x00, 0x80}, zeros(12))},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

// TestDisassembleFloatingImmediates checks the disassembler shows a
// floating immediate in its own format, with the fewest digits that
// reassemble to the same bytes, and a short literal by its value.
func TestDisassembleFloatingImmediates(t *testing.T) {
	cases := []struct {
		bytes []byte
		want  string
	}{
		{cat([]byte{0x70, 0x8F}, d11, []byte{0x50}), "MOVD I^#1.1,R0"},
		{cat([]byte{0xFD, 0x50, 0x8F}, g11, []byte{0x50}), "MOVG I^#1.1,R0"},
		{cat([]byte{0xFD, 0x70, 0x8F}, h11, []byte{0x50}), "MOVH I^#1.1,R0"},
		{[]byte{0xFD, 0x50, 0x0C, 0x50}, "MOVG S^#1.5,R0"},
		// 1/3 in D: the shortest decimal that rounds back to it.
		{[]byte{0x70, 0x8F, 0xAA, 0x3F, 0xAA, 0xAA, 0xAA, 0xAA, 0xAB, 0xAA, 0x50}, "MOVD I^#0.333333333333333336,R0"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			dec, err := Disassemble(SliceReader(tc.bytes), 0)
			if err != nil {
				t.Fatalf("Disassemble: %v", err)
			}

			if got := dec.String(); got != tc.want {
				t.Errorf("Disassemble = %q, want %q", got, tc.want)
			}

			requireBytes(t, assembleBytesAt(t, 0, dec.String()), tc.bytes...)
		})
	}
}

// TestPacked checks .PACKED with the MACRO manual's examples: the digits
// two to a byte, the sign last (^XC, or ^XD for minus), a zero first
// nibble for an even number of digits, and the symbol set to the number
// of digits.
func TestPacked(t *testing.T) {
	cases := []struct {
		src  string
		want []byte
	}{
		{".PACKED -12,PACK_SIZE\n.BYTE PACK_SIZE", []byte{0x01, 0x2D, 0x02}},
		{".PACKED +500", []byte{0x50, 0x0C}},
		{".PACKED 0", []byte{0x0C}},
		{".PACKED -0,SUM_SIZE\n.BYTE SUM_SIZE", []byte{0x0D, 0x01}},
		{".PACKED 1234567890123456789012345678901", []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0x12, 0x34, 0x56, 0x78, 0x90, 0x12, 0x34, 0x56, 0x78, 0x90, 0x1C}},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}

	for _, bad := range []string{".PACKED", ".PACKED 12345678901234567890123456789012", ".PACKED ABC"} {
		if _, err := New(true).Assemble(bad); err == nil {
			t.Errorf("%q assembled, want an error", bad)
		}
	}
}
