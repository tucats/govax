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
		{"64-bit hex literal", ".QUAD 0123456789ABCDEF", []byte{0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01}},
		{"^X prefix", ".QUAD ^X1", []byte{1, 0, 0, 0, 0, 0, 0, 0}},
		{"decimal", ".QUAD ^D10", []byte{10, 0, 0, 0, 0, 0, 0, 0}},
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
	requireBytes(t, assembleBytes(t, ".BYTE -1, 0FF\n.WORD -2"), 0xFF, 0xFF, 0xFE, 0xFF)
	requireCode(t, assembleErr(t, ".BYTE 100"), vmserrors.VAX_DATARANGE)
	requireCode(t, assembleErr(t, ".WORD -8001"), vmserrors.VAX_DATARANGE)
}

func TestDirectAssignment(t *testing.T) {
	requireBytes(t, assembleBytes(t, "X = 5\nY == X+1\n.BYTE X, Y"), 5, 6)
	// ". =" moves the location counter.
	requireBytes(t, assembleBytes(t, ".BYTE 1\n. = .+2\n.BYTE 2"), 1, 0, 0, 2)
}

func TestDisplacementRange(t *testing.T) {
	requireCode(t, assembleErr(t, "CLRL B^100(R0)"), vmserrors.VAX_DATARANGE)
	requireCode(t, assembleErr(t, "FOO: .BLKB 100\nCLRL B^FOO"), vmserrors.VAX_DATARANGE)
}
