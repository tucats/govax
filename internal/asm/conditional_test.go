package asm

import (
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestConditionalTests checks each condition test, long and short form,
// in a block that stores 1 if it is met.
func TestConditionalTests(t *testing.T) {
	tests := []struct {
		cond string
		met  bool
	}{
		{"EQ 0", true}, {"EQUAL 1", false},
		{"NE 1", true}, {"NOT_EQUAL 0", false},
		{"GT 1", true}, {"GREATER 0", false},
		{"LE 0", true}, {"LESS_EQUAL 1", false},
		{"LT -1", true}, {"LESS_THAN 0", false},
		{"GE 0", true}, {"GREATER_EQUAL -1", false},
		{"EQ, X-5", true}, // a comma may separate the test from its argument
		{"DF X", true}, {"DEFINED Y", false},
		{"NDF Y", true}, {"NOT_DEFINED X", false},
		{"DF X&Y", false}, {"DF X!Y", true},
		{"B <>", true}, {"BLANK <A>", false},
		{"NB <A>", true}, {"NOT_BLANK <>", false},
		{"IDN <A B>,<A B>", true}, {"IDENTICAL A,B", false},
		{"DIF A,B", true}, {"DIFFERENT <A>,<A>", false},
	}

	for _, tc := range tests {
		src := "X = 5\n.IF " + tc.cond + "\n.BYTE 1\n.ENDC\n.BYTE 2"

		want := []byte{2}
		if tc.met {
			want = []byte{1, 2}
		}

		requireBytes(t, assembleBytes(t, src), want...)
	}
}

// TestConditionalBlocks follows the VAX MACRO manual's examples of nested
// blocks and subconditionals.
func TestConditionalBlocks(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []byte
	}{
		{"subconditionals", `SYM = 1
.IF DF SYM
.BYTE 1
.IF_FALSE
.BYTE 2
.IF_TRUE
.BYTE 3
.IF_TRUE_FALSE
.BYTE 4
.IFT
.BYTE 5
.ENDC`, []byte{1, 3, 4, 5}},
		{"nested, inner false", `X = 1
.IF DF X
.IF DF Y
.BYTE 1
.IFF
.BYTE 2
.IFT
.BYTE 3
.ENDC
.ENDC`, []byte{2}},
		{"outer false leaves out nested subconditionals", `Y = 1
.IF DF X
.BYTE 1
.IF DF Y
.BYTE 2
.IFF
.BYTE 3
.IFT
.BYTE 4
.ENDC
.IFTF
.BYTE 5
.ENDC`, []byte{5}},
		{"a label in a left-out block isn't defined", `.IF NE 0
L: .BYTE 1
.ENDC
.IIF NDF L, .BYTE 2`, []byte{2}},
		{"IIF", ".IIF DF X, .BYTE 1\nX = 1\n.IIF DF X, .BYTE 2\n.IIF EQ 1, .BYTE 3", []byte{2}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

func TestConditionalErrors(t *testing.T) {
	requireCode(t, assembleErr(t, ".ENDC"), vmserrors.VAX_NOCOND)
	requireCode(t, assembleErr(t, ".IFF"), vmserrors.VAX_NOCOND)
	requireCode(t, assembleErr(t, ".IF EQ 0\n.BYTE 1"), vmserrors.VAX_NOENDC)
	requireCode(t, assembleErr(t, ".IF ZERO 0\n.ENDC"), vmserrors.VAX_BADCOND)
	requireCode(t, assembleErr(t, ".IF EQ X\n.ENDC\nX = 0"), vmserrors.VAX_UNDEFSYM)
	requireCode(t, assembleErr(t, ".IIF EQ 0"), vmserrors.VAX_BADCOND)

	deep := ""
	for range maxCondDepth + 1 {
		deep += ".IF EQ 0\n"
	}

	requireCode(t, assembleErr(t, deep), vmserrors.VAX_CONDDEPTH)
}
