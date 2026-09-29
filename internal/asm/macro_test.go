package asm

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// assembleErr assembles src with a fresh Assembler and returns the error,
// failing the test if there wasn't one.
func assembleErr(t *testing.T, src string) error {
	t.Helper()

	_, err := New(true).Assemble(src)
	if err == nil {
		t.Fatalf("assemble %q: expected an error", src)
	}

	return err
}

func requireCode(t *testing.T, err error, code uint32) {
	t.Helper()

	if !errors.Is(err, vmserrors.New(code)) {
		t.Fatalf("error %v, want %v", err, vmserrors.New(code))
	}
}

func TestLocalLabelBackwardBranch(t *testing.T) {
	// 1$ is at 0x200; BRB's displacement is measured from 0x204.
	got := assembleBytes(t, "1$: DECL R0\nBRB 1$")
	requireBytes(t, got, 0xD7, 0x50, 0x11, 0xFC)
}

func TestLocalLabelForwardBranch(t *testing.T) {
	got := assembleBytes(t, "BNEQ 2$\nNOP\n2$: RSB")
	requireBytes(t, got, 0x12, 0x01, 0x01, 0x05)
}

// TestLocalLabelBlocks reuses the same local label in two blocks, each
// started by an ordinary label, and checks each branch reaches its own.
func TestLocalLabelBlocks(t *testing.T) {
	src := "A: BRB 1$\n1$: NOP\nB: BRB 1$\nNOP\n1$: RSB"
	got := assembleBytes(t, src)
	requireBytes(t, got,
		0x11, 0x00, // A: BRB 1$ (0x202)
		0x01,       // 1$: NOP
		0x11, 0x01, // B: BRB 1$ (0x206)
		0x01, // NOP
		0x05, // 1$: RSB
	)
}

func TestLocalLabelBlockEndsAtEntry(t *testing.T) {
	src := ".ENTRY A\n1$: RET\n.ENTRY B\n1$: BRB 1$"
	got := assembleBytes(t, src)
	requireBytes(t, got, 0, 0, 0x04, 0, 0, 0x11, 0xFE)
}

func TestLocalLabelDuplicateInBlock(t *testing.T) {
	requireCode(t, assembleErr(t, "1$: NOP\n1$: NOP"), vmserrors.VAX_DUPSYM)
}

func TestLocalLabelUndefinedAtBlockEnd(t *testing.T) {
	requireCode(t, assembleErr(t, "BRB 1$\nA: NOP\n1$: NOP"), vmserrors.VAX_UNDEFSYM)
}

func TestLocalLabelAsOperandAndData(t *testing.T) {
	// A local label works anywhere an ordinary symbol does. The forward
	// reference gets a longword relative displacement: 1$ (0x207) is one
	// byte past its end (0x206).
	got := assembleBytes(t, "MOVAL 1$, R0\n1$: .LONG 1$")
	requireBytes(t, got, 0xDE, 0xEF, 0x01, 0x00, 0x00, 0x00, 0x50, 0x07, 0x02, 0x00, 0x00)
}

func TestLocalLabelsNotExported(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble("A: NOP\n1$: BRB 1$"); err != nil {
		t.Fatal(err)
	}

	syms := a.Symbols()
	if _, ok := syms["A"]; !ok {
		t.Fatal("A missing from Symbols()")
	}

	if len(syms) != 1 {
		t.Fatalf("Symbols() = %v, want only A", syms)
	}
}

func TestQuad(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []byte
	}{
		{"64-bit hex literal", ".QUAD ^X0123456789ABCDEF", []byte{0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01}},
		{"^X prefix", ".QUAD ^X1", []byte{1, 0, 0, 0, 0, 0, 0, 0}},
		{"decimal", ".QUAD ^D10", []byte{10, 0, 0, 0, 0, 0, 0, 0}},
		{"default radix", ".QUAD 18446744073709551615", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
		{"negative literal", ".QUAD -1", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
		{"list", ".QUAD 1, 2", []byte{1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0}},
		{"expression sign-extends", "X=5\n.QUAD X-6", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
		{"forward reference", ".QUAD L\nL:", []byte{0x08, 0x02, 0, 0, 0, 0, 0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireBytes(t, assembleBytes(t, tc.src), tc.want...)
		})
	}
}

func TestSignedData(t *testing.T) {
	requireBytes(t, assembleBytes(t, ".BYTE -1, 255\n.WORD -2"), 0xFF, 0xFF, 0xFE, 0xFF)
	requireCode(t, assembleErr(t, ".BYTE 256"), vmserrors.VAX_DATARANGE)
	requireCode(t, assembleErr(t, ".WORD -32769"), vmserrors.VAX_DATARANGE)
}

func TestDirectAssignment(t *testing.T) {
	requireBytes(t, assembleBytes(t, "X = 5\nY == X+1\n.BYTE X, Y"), 5, 6)
	// ". =" moves the location counter.
	requireBytes(t, assembleBytes(t, ".BYTE 1\n. = .+2\n.BYTE 2"), 1, 0, 0, 2)
}

func TestDisplacementRange(t *testing.T) {
	requireCode(t, assembleErr(t, "CLRL B^256(R0)"), vmserrors.VAX_DATARANGE)
	requireCode(t, assembleErr(t, "FOO: .BLKB 256\nCLRL B^FOO"), vmserrors.VAX_DATARANGE)
}

// TestForwardExpressions: an expression using symbols not yet defined is
// completed when they are, wherever it appears.
func TestForwardExpressions(t *testing.T) {
	tests := []struct {
		src  string
		want []byte
	}{
		// B-A: A at 0x204, B at 0x208.
		{".LONG B-A\nA: .LONG 0\nB: .LONG 0", []byte{4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{".LONG A+4\nA: .LONG 0", []byte{0x08, 0x02, 0, 0, 0, 0, 0, 0}},
		{".LONG 2*A-1\nA: .LONG 0", []byte{0x07, 0x04, 0, 0, 0, 0, 0, 0}},
		{".LONG <A+B>-B\nA=3\nB=5", []byte{3, 0, 0, 0}},
		// .WORD A-. : "." is 0x200, A 0x202.
		{".WORD A-.\nA: .WORD 0", []byte{2, 0, 0, 0}},
		// Absolute: A at 0x207; A+8 = 0x20F.
		{"MOVL #1, @#A+8\nA: .LONG 0", []byte{0xD0, 0x01, 0x9F, 0x0F, 0x02, 0, 0, 0, 0, 0, 0}},
		// Relative: A+8 = 0x20F, measured from 0x206.
		{"MOVL A+8, R0\nA: .LONG 0", []byte{0xD0, 0xEF, 0x09, 0, 0, 0, 0x50, 0, 0, 0, 0}},
		// Immediate: the fixup follows the 8F mode byte.
		{"MOVL #A+1, R0\nA: .LONG 0", []byte{0xD0, 0x8F, 0x08, 0x02, 0, 0, 0x50, 0, 0, 0, 0}},
		// Displacement: A+4 = 0x20B, from R1.
		{"MOVL A+4(R1), R0\nA: .LONG 0", []byte{0xD0, 0xE1, 0x0B, 0x02, 0, 0, 0x50, 0, 0, 0, 0}},
		{"MOVAL 1$+4, R0\n1$: .LONG 0", []byte{0xDE, 0xEF, 0x05, 0, 0, 0, 0x50, 0, 0, 0, 0}},
	}

	for _, tc := range tests {
		requireBytes(t, assembleBytes(t, tc.src), tc.want...)
	}

	requireCode(t, assembleErr(t, ".LONG A/2\nA: .LONG 0"), vmserrors.VAX_FWDOPERATOR)
	requireCode(t, assembleErr(t, ".LONG A*B\nA: .LONG 0\nB: .LONG 0"), vmserrors.VAX_FWDOPERATOR)
	requireCode(t, assembleErr(t, ".LONG A&1\nA: .LONG 0"), vmserrors.VAX_FWDOPERATOR)
}

// TestCaseTable: every .CASE entry is its label's offset from the table,
// whether the label is defined before or after it.
func TestCaseTable(t *testing.T) {
	got := assembleBytes(t, "X: .CASE X, Y\nY: NOP")
	requireBytes(t, got, 0, 0, 4, 0, 0x01)
}

// TestMacro32Operators covers MACRO-32's <> grouping and its unary and
// binary operators.
func TestMacro32Operators(t *testing.T) {
	tests := []struct {
		src  string
		want uint32
	}{
		{"<1+2>*3", 9},
		{"1+2*3", 9}, // equal priority, left to right
		{"^D<10+10>", 20},
		{"^X<10+10>", 0x20},
		{"^B101", 5},
		{"^B<101+1>", 6},
		{"^O17", 15},
		{"^C0", 0xFFFFFFFF},
		{"^C<^X0F>&^X0FF", 0xF0},
		{"^C<15>&255", 0xF0},
		{"^A/ab/", 0x6261},
		{"^A\"A\"", 0x41},
		{"1@4", 0x10},
		{"^X100@-4", 0x10},
		{"-8@-1", 0xFFFFFFFC},
		{"12&10", 8},
		{"12!3", 15},
		{"12\\10", 6},
		{"^B1100&^B1010", 8},
	}

	for _, tc := range tests {
		got := assembleBytes(t, ".LONG "+tc.src)
		v := uint32(got[0]) | uint32(got[1])<<8 | uint32(got[2])<<16 | uint32(got[3])<<24

		if v != tc.want {
			t.Errorf(".LONG %s = %#x, want %#x", tc.src, v, tc.want)
		}
	}

	requireCode(t, assembleErr(t, ".LONG ^B102"), vmserrors.VAX_BADDIGIT)
	requireCode(t, assembleErr(t, ".LONG 0FF"), vmserrors.VAX_BADDIGIT)
	requireCode(t, assembleErr(t, ".LONG ^X0FG"), vmserrors.VAX_BADDIGIT)
	requireCode(t, assembleErr(t, ".LONG 1 2"), vmserrors.VAX_EXTRATEXT)
	requireCode(t, assembleErr(t, ".LONG 1<2"), vmserrors.VAX_EXTRATEXT) // no comparisons
	requireCode(t, assembleErr(t, ".LONG ^A/abcde/"), vmserrors.VAX_CHARTOOLONG)
	requireCode(t, assembleErr(t, ".LONG <1+2"), vmserrors.VAX_NOCLOSE)
}

func TestExtraText(t *testing.T) {
	requireCode(t, assembleErr(t, "MOVL R0, R1 R2"), vmserrors.VAX_EXTRATEXT)
	requireCode(t, assembleErr(t, "NOP X"), vmserrors.VAX_EXTRATEXT)
}

// TestContinuationLines: a statement ending in "-" continues on the next
// line, before any comment.
func TestContinuationLines(t *testing.T) {
	requireBytes(t, assembleBytes(t, ".LONG 1,-  ; first\n 2"), 1, 0, 0, 0, 2, 0, 0, 0)
	requireBytes(t, assembleBytes(t, ".ascii /ab/-\n /cd/"), 'a', 'b', 'c', 'd')
	requireBytes(t, assembleBytes(t, "MOVL R0,-\n-(SP)"), 0xD0, 0x50, 0x7E)
}

// TestForwardDataRange: a forward reference in .BYTE/.WORD is range
// checked once its value is known, not before.
func TestForwardDataRange(t *testing.T) {
	requireBytes(t, assembleBytes(t, ".BYTE A-300\nA = 301"), 1)
	requireBytes(t, assembleBytes(t, ".BYTE A\nA = 255"), 0xFF)                 //nolint:dupword
	requireCode(t, assembleErr(t, ".BYTE A\nA = 256"), vmserrors.VAX_FWDBYTE)   //nolint:dupword
	requireCode(t, assembleErr(t, ".WORD A\nA = 65536"), vmserrors.VAX_FWDWORD) //nolint:dupword
}
