package asm

import (
	"strings"
	"testing"
)

// TestOverwriteWithAddress is $FAB's pattern: a field stored as zero,
// then stored again, through ". =", as the address of a string in another
// psect. The object must store the address last, where the linker's
// last store wins.
func TestOverwriteWithAddress(t *testing.T) {
	a := macroAssemble(t, `	.PSECT	DATA, LONG
TAB:	.LONG	0, 0
END:
	.SAVE
	.PSECT	NAMES
NAME:	.ASCII	/X.DAT/
	.RESTORE
	.=TAB+4
	.ADDRESS NAME
	.=END`)

	requireRelocations(t, a, "DATA+4 PIDR NAMES:0")

	text := assemblyDump(t, a)

	back := strings.Index(text, "CTL_AUGRB 0xfffffffc")
	pidr := strings.LastIndex(text, "STO_PIDR")
	zeros := strings.Index(text, "STO_IMM 8 bytes")

	if zeros < 0 || back < 0 || pidr < back || zeros > back {
		t.Errorf("want the zeros stored, then the location moved back, then STO_PIDR:\n%s", text)
	}

	if strings.Count(text, "STO_PIDR") != 1 {
		t.Errorf("want one STO_PIDR:\n%s", text)
	}
}

// TestOverwriteForwardReference stores a forward reference, then stores a
// constant over it: the constant stays, and the reference, when its
// symbol is defined, stores nothing and leaves no relocation.
func TestOverwriteForwardReference(t *testing.T) {
	a := macroAssemble(t, `	.PSECT	DATA, LONG
TAB:	.LONG	LATER, EXTERN
	.=TAB
	.LONG	5, 6
LATER:	.LONG	7`)

	requireRelocations(t, a)
	requireBytes(t, psectBytes(t, a, "DATA"), 5, 0, 0, 0, 6, 0, 0, 0, 7, 0, 0, 0)
}

// TestOverwriteRelocationWithConstant stores a relocatable value, then a
// constant over it: the relocation is dropped.
func TestOverwriteRelocationWithConstant(t *testing.T) {
	a := macroAssemble(t, `	.PSECT	DATA, LONG
TAB:	.ADDRESS TAB
	.=TAB
	.LONG	9`)

	requireRelocations(t, a)
	requireBytes(t, psectBytes(t, a, "DATA"), 9, 0, 0, 0)

	if text := assemblyDump(t, a); strings.Contains(text, "STO_PIDR") {
		t.Errorf("the dropped relocation is in the object:\n%s", text)
	}
}

// TestOverwriteWithinStatement: an instruction's own fixups are never
// cancelled by its own stores.
func TestOverwriteWithinStatement(t *testing.T) {
	a := macroAssemble(t, `	.PSECT	CODE
	MOVL	FWD, R0
FWD:	.LONG	1`)

	if len(a.Relocations()) == 0 {
		t.Error("the operand's relocation is gone")
	}
}

// assemblyDump is the dump of a's object module.
func assemblyDump(t *testing.T, a *Assembler) string {
	t.Helper()

	m, err := a.Object(ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return dumpText(t, m)
}
