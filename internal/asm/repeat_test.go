package asm

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// Tests for repeat blocks (docs/PHASE-28.md, subtask 3). The first few
// are the examples in the VAX MACRO manual's descriptions of .IRP, .IRPC,
// and .REPEAT, whose expansions the manual prints.

// TestRepeatCopies is the manual's COPIES macro (.REPEAT): the count may
// be a symbol, and the block's lines get the macro's arguments.
func TestRepeatCopies(t *testing.T) {
	src := `
	.MACRO	COPIES	STRING,NUM
	.REPEAT	NUM
	.ASCII	/STRING/
	.ENDR
	.BYTE	0
	.ENDM	COPIES
	.PSECT	DATA
	COPIES	<AB>,3
VARB = 2
	COPIES	<X Y>,VARB`

	requireBytes(t, macroBytes(t, src), 'A', 'B', 'A', 'B', 'A', 'B', 0, 'X', ' ', 'Y', 'X', ' ', 'Y', 0)
}

// TestRepeatCount: .REPT is .REPEAT, and a count of zero or less
// assembles nothing.
func TestRepeatCount(t *testing.T) {
	src := `
	.PSECT	DATA
	.REPT	2
	.BYTE	1
	.ENDR
	.REPEAT	0
	.BYTE	2
	.ENDR
	.REPEAT	-1
	.BYTE	3
	.ENDR
	.BYTE	4`

	requireBytes(t, macroBytes(t, src), 1, 1, 4)
}

// TestIrpCallSub is the manual's CALL_SUB macro (.IRP): the list, with
// its null arguments, comes from the macro's arguments, and .IIF
// NOT_BLANK leaves the blank ones out.
func TestIrpCallSub(t *testing.T) {
	src := `
	.MACRO	CALL_SUB	SUBR,A1,A2,A3,A4,A5,A6,A7,A8,A9,A10
	.NARG	COUNT
	.IRP	ARG,<A10,A9,A8,A7,A6,A5,A4,A3,A2,A1>
	.IIF	NOT_BLANK ,	ARG,	PUSHL ARG
	.ENDR
	CALLS	#<COUNT-1>,SUBR		; Note SUBR is counted
	.ENDM	CALL_SUB
	.PSECT	CODE
	CALL_SUB	@(R6),R1,R2,#5`

	requireBytes(t, macroBytes(t, src),
		0xDD, 0x05, // PUSHL #5
		0xDD, 0x52, // PUSHL R2
		0xDD, 0x51, // PUSHL R1
		0xFB, 0x03, 0xB6, 0x00) // CALLS #3,@0(R6)
}

// TestIrpcHashSym is the manual's HASH_SYM macro (.IRPC): one repetition
// per character, the character substituted even between ^A's delimiters.
func TestIrpcHashSym(t *testing.T) {
	src := `
	.MACRO	HASH_SYM	SYMBOL
	.NCHR	HV,<SYMBOL>
	.IRPC	CHR,<SYMBOL>
HV = HV+^A?CHR?
	.ENDR
	.ENDM	HASH_SYM
	.PSECT	DATA
	HASH_SYM	<MOVC5>
	.WORD	HV`

	hv := 5 + 'M' + 'O' + 'V' + 'C' + '5'
	requireBytes(t, macroBytes(t, src), byte(hv), byte(hv>>8))
}

// TestIrpArguments: .IRP's list is read as a macro call's arguments are
// (blanks separate, commas make null arguments, <...> groups, \ passes a
// value), and an empty list assembles nothing.
func TestIrpArguments(t *testing.T) {
	src := `
	.PSECT	DATA
N = 7
	.IRP	X,<1 2,,<3+1>,\N>
	.BYTE	X+0
	.ENDR
	.IRP	X,<>
	.BYTE	9
	.ENDR
	.IRP	X,5
	.BYTE	X
	.ENDR`

	requireBytes(t, macroBytes(t, src), 1, 2, 0, 4, 7, 5)
}

// TestIrpConcatenation: the formal argument is substituted as a macro's
// is, apostrophes included.
func TestIrpConcatenation(t *testing.T) {
	src := `
	.PSECT	DATA
	.IRP	S,<A,B>
LAB_'S:	.BYTE	^A/S/
	.ENDR
	.LONG	LAB_B-LAB_A`

	requireBytes(t, macroBytes(t, src), 'A', 'B', 1, 0, 0, 0)
}

// TestNestedRepeatBlocks: repeat blocks nest, in each other and in
// macros, and a block can define a macro.
func TestNestedRepeatBlocks(t *testing.T) {
	src := `
	.PSECT	DATA
	.IRP	X,<1,2>
	.IRPC	Y,<AB>
	.BYTE	X, ^A/Y/
	.ENDR
	.ENDR
	.IRP	N,<ONE,TWO>
	.MACRO	N	V
	.BYTE	V
	.ENDM	N
	.ENDR
	ONE	8
	TWO	9`

	requireBytes(t, macroBytes(t, src), 1, 'A', 1, 'B', 2, 'A', 2, 'B', 8, 9)
}

// TestRepeatMexit: .MEXIT in a repeat block ends the repetition and the
// ones after it, and ends only the innermost block (the manual, .MEXIT's
// notes 1 and 2); the macro around it goes on.
func TestRepeatMexit(t *testing.T) {
	src := `
	.MACRO	UPTO	LIMIT,LIST
	.IRP	X,<LIST>
	.IF	GT	X-LIMIT
	.MEXIT
	.ENDC
	.BYTE	X
	.ENDR
	.BYTE	0
	.ENDM	UPTO
	.PSECT	DATA
	UPTO	3,<1,2,5,3>
	.BYTE	9`

	requireBytes(t, macroBytes(t, src), 1, 2, 0, 9)
}

// TestRepeatEndm: a repeat block may end with .ENDM, as some of VMS's
// STARLET.MLB macros' do, in a macro and outside one.
func TestRepeatEndm(t *testing.T) {
	src := `
	.MACRO	M	LIST
	.IRP	X,<LIST>
	.BYTE	X
	.ENDM
	.ENDM	M
	.PSECT	DATA
	M	<1,2>
	.IRP	X,<3>
	.BYTE	X
	.ENDM`

	requireBytes(t, macroBytes(t, src), 1, 2, 3)
}

// TestRepeatErrors: the errors a repeat block can make.
func TestRepeatErrors(t *testing.T) {
	cases := map[string]uint32{
		"\t.ENDR":                        vmserrors.VAX_NOTINREPEAT,
		"\t.REPEAT\t2\n\t.BYTE\t1":       vmserrors.VAX_NOENDR,
		"\t.IRP\t,<1>\n.ENDR":            vmserrors.VAX_BADFORMAL,
		"\t.IRP\tX,<1> 2\n.ENDR":         vmserrors.VAX_EXTRATEXT,
		"\t.REPEAT\tLATER\n.ENDR\nLATER=1": vmserrors.VAX_UNDEFSYM,
	}

	for src, code := range cases {
		requireCode(t, macroErr(t, src), code)
	}
}

// TestRepeatErrorLocation: an error in a repetition names the block's
// directive line, the directive, and the repetition, in both dialects.
func TestRepeatErrorLocation(t *testing.T) {
	src := strings.Join([]string{
		"\t.PSECT\tDATA",  // 1
		"\t.IRP\tX,<1,Q>", // 2
		"\t.BYTE\t0",      // 3 (the range's line 1)
		"\t.BYTE\tX(",     // 4 (the range's line 2)
		"\t.ENDR",         // 5
	}, "\n")

	err := macroErr(t, src)

	var line *Error
	if !errors.As(err, &line) || line.Line != 2 {
		t.Fatalf("error %v: want one at line 2", err)
	}

	block, ok := line.Err.(*ExpansionError)
	if !ok || block.Block != ".IRP" || block.Repetition != 1 || block.Line != 2 {
		t.Fatalf("error %v: want one in .IRP's repetition 1, line 2", err)
	}

	want := "line 2: in repetition 1 of .IRP, line 2: "
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error text %q, want it to start %q", err.Error(), want)
	}

	_, err = New(false).Assemble(strings.Replace(src, "\t.PSECT\tDATA", "\t.BYTE\t0", 1))
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("console error %v, want it to start %q", err, want)
	}
}

// TestConsoleDialectRepeat: the console's ASM has repeat blocks too, with
// or without the directives' ".", and at the interactive prompt.
func TestConsoleDialectRepeat(t *testing.T) {
	requireBytes(t, assembleBytes(t, "IRP X,<1,2>\nBYTE X\nENDR\nREPT 2\nBYTE 3\nENDR"), 1, 2, 3, 3)

	a := New(false)
	a.BeginInteractive()

	for _, line := range []string{".IRPC C,<AB>", ".BYTE ^A/C/", ".ENDR"} {
		if _, err := a.AssembleLine(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	requireBytes(t, a.Bytes(), 'A', 'B')
}
