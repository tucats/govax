package asm

import (
	"io/fs"
	"testing"
	"time"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/lbr"
)

// govaxStarlet returns govax's own STARLET.MLB, from bootdata.
func govaxStarlet(t *testing.T) MacroLibrary {
	t.Helper()

	data, err := fs.ReadFile(bootdata.FS, bootdata.StarletLibrary)
	if err != nil {
		t.Fatal(err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	lib, err := NewMacroLibrary(l)
	if err != nil {
		t.Fatal(err)
	}

	return lib
}

// starletProgram wraps system macro calls in a program with the data
// they refer to.
func starletProgram(calls string) string {
	return `
	.TITLE	CALLS
IO$_WRITEVBLK = ^X30
	.PSECT	DATA,LONG,NOEXE,WRT
IOSB:	.BLKQ	1
CHAN:	.BLKW	1
MSG:	.ASCII	/Hello/
TTNAME:	.ASCID	/TT/
MBX:	.ASCID	/MBX/
	.PSECT	CODE,EXE,NOWRT
AST:	.WORD	0
	RET
	.ENTRY	START,^M<>
` + calls + `
	.END	START`
}

// starletCalls are calls of every macro in govax's STARLET.MLB, with
// their arguments given in each form the macros treat differently:
// omitted, zero, by value, and by reference in each addressing mode
// $PUSHADR tells apart.
var starletCalls = []string{
	"$EXIT_S",
	"$EXIT_S\tR0",
	"$EXIT_S\tCODE=#44",
	"$ASSIGN_S\tDEVNAM=TTNAME,CHAN=CHAN",
	"$ASSIGN_S\tTTNAME,CHAN,ACMODE=#3,MBXNAM=MBX,FLAGS=#1",
	"$ASSIGN_S\tDEVNAM=(R2),CHAN=(R3)+",
	"$ASSIGN_S\tDEVNAM=-(R2),CHAN=4(R3)[R4]",
	"$ASSIGN_S\tDEVNAM=@#^X200,CHAN=@8(R3)",
	"$ASSIGN_S\tDEVNAM=G^TTNAME,CHAN=@(R3)+",
	"$ASSIGN_S\tTTNAME,CHAN,MBXNAM=MBX",
	"$ASSIGN_S\tTTNAME,CHAN,ACMODE=#1",
	"$DASSGN_S\tCHAN=CHAN",
	"$DASSGN_S\tR1",
	"$QIOW_S\tCHAN=CHAN,FUNC=#IO$_WRITEVBLK,IOSB=IOSB,P1=MSG,P2=#5",
	"$QIOW_S\tCHAN=W^CHAN,FUNC=#IO$_WRITEVBLK,P1=MSG,P2=#5,P5=#1",
	"$QIOW_S\tCHAN=CHAN,FUNC=#IO$_WRITEVBLK,P1=MSG,P2=#5,P6=#1",
	"$QIO_S\tEFN=#1,CHAN=CHAN,FUNC=#IO$_WRITEVBLK,IOSB=(R3)+,ASTADR=AST,ASTPRM=#7,P1=(R4),P2=R5,P3=#1,P4=#2,P5=#3,P6=#4",
	"$QIO_S\tCHAN=CHAN,FUNC=#IO$_WRITEVBLK,ASTADR=AST,P1=4(R5)[R2],P2=#5",
	"$QIO_S\tCHAN=CHAN,FUNC=#IO$_WRITEVBLK,ASTPRM=#1,P1=#5,P2=#5",
	"$QIO_S\tCHAN=CHAN,FUNC=#IO$_WRITEVBLK,P1=#1000,P2=#5,P3=#1,P4=#0",
	"$QIO_S\tCHAN=CHAN,FUNC=#IO$_WRITEVBLK,P1=I^#5,P2=#5,P3=#0,P4=#1",
	"$QIO_S\tCHAN=CHAN,FUNC=#IO$_WRITEVBLK,P1=@#^X200,P2=#5",
}

// TestGovaxStarletAssembles: a program calling each of govax's system
// macros assembles, with no real STARLET.MLB, into an object with no
// warnings that calls the services.
func TestGovaxStarletAssembles(t *testing.T) {
	var calls string
	for _, c := range starletCalls {
		calls += "\t" + c + "\n"
	}

	a := libAssembler(govaxStarlet(t))
	libBytes(t, a, starletProgram(calls))

	if len(a.Warnings()) != 0 || len(a.Messages()) != 0 {
		t.Errorf("warnings %v, messages %q, want none", a.Warnings(), a.Messages())
	}

	if _, err := a.Object(ObjectOptions{}); err != nil {
		t.Fatal(err)
	}

	for _, sym := range []string{"SYS$EXIT", "SYS$ASSIGN", "SYS$DASSGN", "SYS$QIO", "SYS$QIOW"} {
		if s, ok := a.symbols.find(sym); !ok || s.defined() {
			t.Errorf("%s isn't an external reference", sym)
		}
	}
}

// TestGovaxStarletMatchesReal: each call expands from govax's STARLET.MLB
// into the same object module as from the real one (which the test skips
// without).
func TestGovaxStarletMatchesReal(t *testing.T) {
	vms, err := NewMacroLibrary(openStarlet(t))
	if err != nil {
		t.Fatal(err)
	}

	ours := govaxStarlet(t)
	when := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	object := func(t *testing.T, lib MacroLibrary, src string) string {
		t.Helper()

		a := libAssembler(lib)
		libBytes(t, a, src)

		m, err := a.Object(ObjectOptions{Created: when})
		if err != nil {
			t.Fatal(err)
		}

		return dumpText(t, m)
	}

	for _, c := range starletCalls {
		t.Run(c, func(t *testing.T) {
			src := starletProgram("\t" + c)

			if got, want := object(t, ours, src), object(t, vms, src); got != want {
				t.Errorf("govax's STARLET.MLB:\n%s\nreal STARLET.MLB:\n%s", got, want)
			}
		})
	}
}
