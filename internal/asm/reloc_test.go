package asm

import (
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// macroAssemble assembles src in the MACRO dialect, failing the test on
// an error.
func macroAssemble(t *testing.T, src string) *Assembler {
	t.Helper()

	a := macroAssembler()
	if _, err := a.Assemble(src); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	return a
}

func requireRelocations(t *testing.T, a *Assembler, want ...string) {
	t.Helper()

	got := a.Relocations()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("relocations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func psectBytes(t *testing.T, a *Assembler, name string) []byte {
	t.Helper()

	s := a.findSection(name)
	if s == nil {
		t.Fatalf("no psect %s", name)
	}

	return s.img.Bytes(0, s.hi)
}

// TestRelocationsMatchRealMACRO assembles testdata/mar/exprs.mar and checks
// each value it leaves for the linker has the shape of the TIR program
// real MACRO wrote for it (testdata/mar/vax/exprs.anl): EXT1+4 is
// STA_GBL EXT1, STA_UB 4, OPR_ADD; B-A across psects is STA_PB 2, STA_PB 1,
// OPR_SUB; and <C-A>*2, though C and A are in one psect, is left to the
// linker too, because C was a forward reference.
func TestRelocationsMatchRealMACRO(t *testing.T) {
	src, err := os.ReadFile("../../testdata/mar/exprs.mar")
	if err != nil {
		t.Fatal(err)
	}

	a := macroAssemble(t, string(src))

	requireRelocations(t, a,
		"DATA+0 L EXT1 4 +",
		"DATA+4 L EXT1 EXT2 -",
		"DATA+8 L OTHER:0 DATA:0 -",
		"DATA+C L DATA:14 DATA:0 - 2 *",
		"DATA+10 L EXT1 2 *",
		"DATA+14 W EXT2",
		"DATA+16 B EXT2 255 &",
		"OTHER+0 L DATA:0",
	)

	if got := a.Externals(); got != "EXT1,EXT2" {
		t.Errorf("externals = %s, want EXT1,EXT2", got)
	}

	// The psect's allocation, as real MACRO's PSC record gives it.
	if got := a.findSection("DATA").hi; got != 23 {
		t.Errorf("DATA allocation = %d, want 23", got)
	}
}

// TestAbsoluteExpressions checks the values MACRO finishes itself: a
// difference of two labels already defined in one psect is absolute
// (the manual, §3.5), and so is anything built only from constants.
func TestAbsoluteExpressions(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
A:	.LONG 1
B:	.LONG 2
	.LONG B-A
	.WORD <B-A>*3
	.BLKB B-A
	.BYTE 7`)

	requireRelocations(t, a)
	requireBytes(t, psectBytes(t, a, "DATA"),
		1, 0, 0, 0,
		2, 0, 0, 0,
		4, 0, 0, 0,
		12, 0,
		0, 0, 0, 0,
		7)
}

// TestRelocatableExpressions checks values that use a psect's base: a
// label, a label plus a constant (kept as written, as MACRO does for
// ITEM+4), a direct assignment's relocatable symbol, and a difference
// across psects.
func TestRelocatableExpressions(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
ITEM:	.LONG 42
PTR:	.LONG ITEM
	.LONG ITEM+4
X = ITEM+8
	.LONG X
	.PSECT CODE
START:	.LONG START-ITEM`)

	requireRelocations(t, a,
		"DATA+4 L DATA:0",
		"DATA+8 L DATA:0 4 +",
		"DATA+C L DATA:8",
		"CODE+0 L CODE:0 DATA:0 -",
	)
}

// TestRelocatableOperands checks instruction operands that refer to
// another psect: relative mode becomes a longword displacement the linker
// finishes (STO_LD), and an immediate a longword value. Each relocation
// is at its displacement or value, just past the mode byte.
func TestRelocatableOperands(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
ITEM:	.LONG 42
	.PSECT CODE
	MOVAL ITEM, R0
	MOVL ITEM+4, R2
	MOVL #ITEM, R1`)

	requireRelocations(t, a,
		"CODE+2 LD DATA:0",
		"CODE+9 LD DATA:0 4 +",
		"CODE+10 L DATA:0",
	)

	requireBytes(t, psectBytes(t, a, "CODE"),
		0xDE, 0xEF, 0, 0, 0, 0, 0x50,
		0xD0, 0xEF, 0, 0, 0, 0, 0x52,
		0xD0, 0x8F, 0, 0, 0, 0, 0x51)
}

// TestBranchesInOnePsect checks that branches to labels in the same psect,
// backward or forward, are finished by the assembler, whatever the psect's
// base turns out to be.
func TestBranchesInOnePsect(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
10$:	DECL R2
	BNEQ 10$
	BRB 20$
	NOP
20$:	RSB`)

	requireRelocations(t, a)
	requireBytes(t, psectBytes(t, a, "CODE"),
		0xD7, 0x52, 0x12, 0xFC, 0x11, 0x01, 0x01, 0x05)
}

// TestASCIDPointer checks that .ASCID's address field is relocatable in
// the MACRO dialect: real MACRO writes it as ".+4", position independent.
func TestASCIDPointer(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
	.BYTE 0
MSG:	.ASCID /Hi/`)

	requireRelocations(t, a, "DATA+5 PIDR DATA:5 4 +")
}

func TestTransferAddress(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
	NOP
	.ENTRY MAIN, ^M<>
	RET
	.END MAIN`)

	addr, ok := a.Entry()
	if !ok || addr != 1 || a.entrySect == nil || a.entrySect.name != "CODE" {
		t.Fatalf("transfer = %v %d in %v, want CODE+1", ok, addr, a.entrySect)
	}
}

// TestMACROAnyOperator checks that the MACRO dialect hands any operator on
// a value it can't finish to the linker, where the console dialect keeps
// the reference tool's VAX_FWDOPERATOR.
func TestMACROAnyOperator(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
	.LONG FWD/2
	.LONG ^C<EXT>
	.LONG -EXT`)

	requireRelocations(t, a,
		"DATA+0 L FWD 2 /",
		"DATA+4 L EXT COM",
		"DATA+8 L EXT NEG",
	)

	_, err := New(true).Assemble(".LONG FWD/2")
	requireCode(t, err, vmserrors.VAX_FWDOPERATOR)
}

// TestRelocatableNotAllowed checks the places a value must be absolute
// (the manual, §3.5), and that a direct assignment can't use a symbol
// not yet defined.
func TestRelocatableNotAllowed(t *testing.T) {
	for _, src := range []string{
		".PSECT DATA\nA: .BLKB A",
		".PSECT DATA\nA: .BYTE 0\n.PSECT CODE\n. = A",
		".PSECT DATA\nA: .BYTE 0\nX = A*2",
	} {
		_, err := macroAssembler().Assemble(src)
		requireCode(t, err, vmserrors.VAX_RELEXPR)
	}

	_, err := macroAssembler().Assemble("X = LATER+1\nLATER: .BYTE 0")
	requireCode(t, err, vmserrors.VAX_UNDEFSYM)
}

func TestPsectIsMACROOnly(t *testing.T) {
	_, err := New(true).Assemble(".PSECT DATA")
	requireCode(t, err, vmserrors.VAX_MACROONLY)
}

// TestPsectContinues checks that returning to a psect continues where it
// left off.
func TestPsectContinues(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
	.BYTE 1
	.PSECT CODE
	NOP
	.PSECT DATA
	.BYTE 2`)

	requireBytes(t, psectBytes(t, a, "DATA"), 1, 2)
	requireBytes(t, psectBytes(t, a, "CODE"), 0x01)
}

// TestUndefinedLocalLabelIsNotExternal checks that a local label never
// defined is an error even without an .END, rather than an external.
func TestUndefinedLocalLabelIsNotExternal(t *testing.T) {
	_, err := macroAssembler().Assemble(".PSECT CODE\nBRB 10$")
	requireCode(t, err, vmserrors.VAX_UNDEFSYM)
}
