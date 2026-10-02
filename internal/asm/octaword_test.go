package asm

import (
	"bytes"
	"testing"
)

// ones returns n bytes of 0xFF, the bytes of -1 at any width.
func ones(n int) []byte { return bytes.Repeat([]byte{0xFF}, n) }

// zeros returns n zero bytes.
func zeros(n int) []byte { return make([]byte, n) }

// cat joins byte slices, to build an expected encoding from its parts.
func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// TestOcta checks .OCTA, which stores 16-byte values, low-order byte
// first. A single number is read at full width; anything else is a
// longword expression, sign-extended.
func TestOcta(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []byte
	}{
		{"128-bit hex literal", ".OCTA ^X0123456789ABCDEFFEDCBA9876543210",
			[]byte{0x10, 0x32, 0x54, 0x76, 0x98, 0xBA, 0xDC, 0xFE, 0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01}},
		{"small", ".OCTA 1", cat([]byte{1}, zeros(15))},
		{"negative literal", ".OCTA -1", ones(16)},
		{"2^64", ".OCTA 18446744073709551616", cat(zeros(8), []byte{1}, zeros(7))},
		{"list", ".OCTA 1, 2", cat([]byte{1}, zeros(15), []byte{2}, zeros(15))},
		{"expression sign-extends", "X=5\n.OCTA X-6", ones(16)},
		{"forward reference", ".OCTA L\nL:", cat([]byte{0x10, 0x02}, zeros(14))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

// TestOctawordImmediates checks MOVO's immediate operand: a short literal
// when the value is 0-63, otherwise 0x8F (immediate mode) and 16 bytes.
func TestOctawordImmediates(t *testing.T) {
	movo := []byte{0xFD, 0x7D} // MOVO is a two-byte opcode
	r0 := []byte{0x50}

	cases := []struct {
		name, src string
		want      []byte
	}{
		{"short literal", "MOVO #5,R0", cat(movo, []byte{0x05}, r0)},
		{"longword value", "MOVO #1000,R0", cat(movo, []byte{0x8F, 0xE8, 0x03}, zeros(14), r0)},
		{"negative", "MOVO #-1,R0", cat(movo, []byte{0x8F}, ones(16), r0)},
		{"full width", "MOVO #^X1000000000000000000000005,R0",
			cat(movo, []byte{0x8F, 0x05}, zeros(11), []byte{0x01}, zeros(3), r0)},
		{"wider than 64 bits isn't a short literal", "MOVO #^X10000000000000005,R0",
			cat(movo, []byte{0x8F, 0x05}, zeros(7), []byte{0x01}, zeros(7), r0)},
		{"expression sign-extended", "MOVO #<0-2>,R0", cat(movo, []byte{0x8F, 0xFE}, ones(15), r0)},
		{"forward symbol", "MOVO #V,R0\nV=-3", cat(movo, []byte{0x8F, 0xFD, 0xFF, 0xFF, 0xFF}, zeros(12), r0)},
		{"CLRH is CLRO", "CLRH R0", []byte{0xFD, 0x7C, 0x50}},
		{"PUSHAO of an immediate", "PUSHAO I^#5", cat([]byte{0xFD, 0x7F, 0x8F, 0x05}, zeros(15))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

// TestDisassembleWideImmediates checks the disassembler shows every byte
// of a quadword or octaword immediate (it used to show only the low
// longword of a quadword), and that the text reassembles to the same
// bytes.
func TestDisassembleWideImmediates(t *testing.T) {
	cases := []struct {
		bytes []byte
		want  string
	}{
		{cat([]byte{0x7D, 0x8F, 0x89, 0x67, 0x45, 0x23, 0x01}, zeros(3), []byte{0x50}),
			"MOVQ I^#^X0000000123456789,R0"},
		{cat([]byte{0xFD, 0x7D, 0x8F, 0x05}, zeros(11), []byte{0x01}, zeros(3), []byte{0x50}),
			"MOVO I^#^X00000001000000000000000000000005,R0"},
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
