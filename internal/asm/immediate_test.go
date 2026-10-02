package asm

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestQuadwordImmediates: a quadword operand's immediate is eight bytes.
// A 32-bit value is sign-extended, as .QUAD extends one, a single number
// is read at full width, and a value not yet defined gets a zero high
// longword. eVAX couldn't assemble a quadword immediate at all.
func TestQuadwordImmediates(t *testing.T) {
	cases := []struct {
		name, src string
		want      []byte
	}{
		{"short literal", "MOVQ #5,R0", []byte{0x7D, 0x05, 0x50}},
		{"longword value", "MOVQ #1000,R0", []byte{0x7D, 0x8F, 0xE8, 0x03, 0, 0, 0, 0, 0, 0, 0x50}},
		// An expression, a negative number included, is 32 bits,
		// zero-extended (as VAX MACRO assembled MOVO #-1; see
		// TestOctawordImmediates).
		{"negative", "MOVQ #-1,R0", []byte{0x7D, 0x8F, 0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0x50}},
		{"full width", "MOVQ #^X123456789,R0", []byte{0x7D, 0x8F, 0x89, 0x67, 0x45, 0x23, 0x01, 0, 0, 0, 0x50}},
		{"high bit of a number isn't a sign", "MOVQ #^X80000000,R0", []byte{0x7D, 0x8F, 0, 0, 0, 0x80, 0, 0, 0, 0, 0x50}},
		{"expression zero-extended", "MOVQ #<0-2>,R0", []byte{0x7D, 0x8F, 0xFE, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0x50}},
		{"defined symbol", "V=-3\nMOVQ #V,R0", []byte{0x7D, 0x8F, 0xFD, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0x50}},
		{"forward symbol", "MOVQ #V,R0\nV=-3", []byte{0x7D, 0x8F, 0xFD, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0x50}},
		{"explicit immediate", "MOVQ I^#5,R0", []byte{0x7D, 0x8F, 0x05, 0, 0, 0, 0, 0, 0, 0, 0x50}},
		{"EDIV's quadword dividend", "EDIV #10,#1000,R0,R1", []byte{0x7B, 0x0A, 0x8F, 0xE8, 0x03, 0, 0, 0, 0, 0, 0, 0x50, 0x51}},
		{"PUSHAQ", "PUSHAQ I^#5", []byte{0x7F, 0x8F, 0x05, 0, 0, 0, 0, 0, 0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

// TestLiteralInAddressOperand: "#n" in an address or field operand is
// immediate mode, even when it would fit a short literal, since a literal
// has no address (the architecture manual's table 8-5).
func TestLiteralInAddressOperand(t *testing.T) {
	cases := []struct {
		name, src string
		want      []byte
	}{
		{"PUSHAL", "PUSHAL #5", []byte{0xDF, 0x8F, 0x05, 0, 0, 0}},
		{"PUSHAW", "PUSHAW #6", []byte{0x3F, 0x8F, 0x06, 0}},
		{"MOVAB", "MOVAB #1,R0", []byte{0x9E, 0x8F, 0x01, 0x50}},
		{"field base", "EXTZV #4,#4,#^XA5,R2", []byte{0xEF, 0x04, 0x04, 0x8F, 0xA5, 0x52}},
		{"read operand keeps the literal", "MOVL #5,R0", []byte{0xD0, 0x05, 0x50}},
		{"register field base", "EXTZV #4,#4,R1,R2", []byte{0xEF, 0x04, 0x04, 0x51, 0x52}},
		{"CALLG's register argument list", "CALLG AP,@#^X1000", []byte{0xFA, 0x5C, 0x9F, 0x00, 0x10, 0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

// TestModeAccessErrors: addressing modes the architecture doesn't allow
// for an operand (tables 8-5 and 8-6) are errors, not code that faults.
func TestModeAccessErrors(t *testing.T) {
	for _, src := range []string{
		"PUSHAW S^#6",         // a literal as an address
		"EXTZV #4,#4,S^#5,R2", // a literal as a field base
		"CLRL #5",             // a literal written
		"INCL #5",             // a literal modified
		"MOVL R0,#5",          //
		"CLRL I^#5",           // an immediate written
		"INCL I^#5",           // an immediate modified
		"PUSHAB R0",           // a register as an address
		"JMP R1",              //
		"MOVAL R2,R0",         //
		"CLRL #5[R1]",         // a literal indexed
		"CLRL I^#5[R1]",       // an immediate indexed
		"CLRL R2[R1]",         // a register indexed
	} {
		t.Run(src, func(t *testing.T) {
			requireCode(t, assembleErr(t, src), vmserrors.VAX_MODEACCESS)
		})
	}
}

// TestModeAccessMACRO: the MACRO dialect reports the same errors, one per
// statement.
func TestModeAccessMACRO(t *testing.T) {
	a := macroAssembler()

	_, err := a.Assemble("\t.PSECT\tC\n\tPUSHAW\tS^#6\n\tPUSHAB\tR0\n\tPUSHAL\t#6\n")

	var errs *Errors
	if !errors.As(err, &errs) || len(errs.List) != 2 {
		t.Fatalf("error %v, want two", err)
	}

	for _, e := range errs.List {
		requireCode(t, e, vmserrors.VAX_MODEACCESS)
	}
}
