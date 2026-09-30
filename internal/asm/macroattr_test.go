package asm

import (
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// Tests for docs/PHASE-28.md subtask 2: created local labels, \symbol
// values, .NARG, .NCHR, .NTYPE, and the string operators. Most follow the
// VAX MACRO manual's own examples.

// TestCreatedLocalLabels is §4.7's POSITIVE: a blank ?L1 gets 30000$,
// then 30001$, and a given one is used as it is.
func TestCreatedLocalLabels(t *testing.T) {
	src := `
	.MACRO	POSITIVE ARG1,?L1
	TSTL	ARG1
	BGEQ	L1
	MNEGL	ARG1,ARG1
L1:	.ENDM	POSITIVE
	.PSECT	CODE
	POSITIVE R0
	POSITIVE R1
	POSITIVE R2,10$
	BRB	30001$
	RSB`

	a := macroAssembler()

	out, err := a.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}

	requireBytes(t, out,
		0xD5, 0x50, 0x18, 0x03, 0xCE, 0x50, 0x50, // POSITIVE R0 (30000$)
		0xD5, 0x51, 0x18, 0x03, 0xCE, 0x51, 0x51, // POSITIVE R1 (30001$)
		0xD5, 0x52, 0x18, 0x03, 0xCE, 0x52, 0x52, // POSITIVE R2,10$
		0x11, 0xF7, // 10$: BRB 30001$, back to offset 14
		0x05, // RSB
	)

	if a.createdLabel != firstCreatedLabel+2 {
		t.Errorf("next created label %d, want %d", a.createdLabel, firstCreatedLabel+2)
	}
}

// TestValueArguments is §4.6's TESTDEF: "\COUNT" passes COUNT's value,
// which concatenation turns into a new symbol's name, and a ^?...?
// delimited default holds brackets.
func TestValueArguments(t *testing.T) {
	src := `
	.MACRO	TESTDEF,TESTNO,ENTRYMASK=^?^M<>?
	.ENTRY	TEST'TESTNO,ENTRYMASK
	.ENDM	TESTDEF
	.PSECT	CODE
COUNT = 2
	TESTDEF	\COUNT
COUNT = COUNT + 1
	TESTDEF	\COUNT,^?^M<R3,R4>?
	TESTDEF	\<COUNT*4>`

	a := macroAssembler()

	out, err := a.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}

	requireBytes(t, out, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00)

	for name, want := range map[string]uint32{"TEST2": 0, "TEST3": 2, "TEST12": 4} {
		if sym, ok := a.symbols.find(name); !ok || sym.value != want {
			t.Errorf("%s = %+v, want %d", name, sym, want)
		}
	}

	requireCode(t, macroErr(t, "\t.MACRO M A\n\t.ENDM\n\tM \\UNDEF"), vmserrors.VAX_UNDEFSYM)
}

// TestNarg is the manual's CNT_ARG (.NARG): positional arguments are
// counted, null ones included, and keyword ones aren't.
func TestNarg(t *testing.T) {
	src := `
	.MACRO	CNT_ARG A1,A2,A3,A4,A5,A6,A7,A8,A9=DEF9,A10=DEF10
	.NARG	COUNTER
	.BYTE	COUNTER
	.ENDM	CNT_ARG
	.PSECT	DATA
	CNT_ARG TEST,FIND,ANS
	CNT_ARG
	CNT_ARG TEST,A2=SYMB2,A3=SY3
	CNT_ARG ,SYMBL,,`

	requireBytes(t, macroBytes(t, src), 3, 0, 1, 3)

	requireCode(t, macroErr(t, "\t.NARG N"), vmserrors.VAX_NOTINMACRO)
}

// TestNchr is the manual's CHAR (.NCHR).
func TestNchr(t *testing.T) {
	src := `
	.MACRO	CHAR	MESS
	.NCHR	CHRCNT,<MESS>
	.WORD	CHRCNT
	.ASCII	/MESS/
	.ENDM	CHAR
	.PSECT	DATA
	CHAR	<HELLO>
	CHAR	<14, 75.39 4>
	.NCHR	N, ABC
	.BYTE	N`

	want := append([]byte{5, 0}, "HELLO"...)
	// The manual says 12 for this one, but the string it prints has 11
	// characters.
	want = append(want, 11, 0)
	want = append(want, "14, 75.39 4"...)
	want = append(want, 3)

	requireBytes(t, macroBytes(t, src), want...)
}

// TestNtype checks .NTYPE's value for each operand form.
func TestNtype(t *testing.T) {
	tests := []struct {
		operand string
		want    uint32
	}{
		{"", 0},
		{"R3", 0x53},
		{"SP", 0x5E},
		{"(R3)", 0x63},
		{"-(R3)", 0x73},
		{"(R3)+", 0x83},
		{"@(R3)+", 0x93},
		{"@(R3)", 0xB3},
		{"#1", 0x00},
		{"#63", 0x00},
		{"#64", 0x1F},
		{"#UNDEF", 0x1F},
		{"S^#1", 0x00},
		{"I^#1", 0x1F},
		{"@#^X200", 0x2F},
		{"G^SYS$EXIT", 0x3F},
		{"B^4(R3)", 0xA3},
		{"@W^4(R3)", 0xD3},
		{"L^4(R3)", 0xE3},
		{"4(R3)", 0xA3},
		{"1000(R3)", 0xC3},
		{"@1000(R3)", 0xD3},
		{"UNDEF(R3)", 0xC3},
		{"HERE", 0xAF},
		{"@HERE", 0xBF},
		{"UNDEF", 0xEF},
		{"W^UNDEF", 0xCF},
		{"(R1)[R4]", 0x6144},
		{"UNDEF[R2]", 0xEF42},
	}

	for _, tc := range tests {
		t.Run(tc.operand, func(t *testing.T) {
			a := macroAssembler()

			_, err := a.Assemble("\t.PSECT CODE\nHERE:\t.NTYPE\tMODE, " + tc.operand + "\n")
			if err != nil {
				t.Fatal(err)
			}

			if sym, ok := a.symbols.find("MODE"); !ok || sym.value != tc.want {
				t.Errorf(".NTYPE %s = %+v, want %#x", tc.operand, sym, tc.want)
			}
		})
	}
}

// TestNtypeInMacro uses .NTYPE as the manual's PUSHADR example does, to
// push an address, or the value of an operand that isn't one.
func TestNtypeInMacro(t *testing.T) {
	src := `
	.MACRO	PUSHADR	ADDR
	.NTYPE	A,ADDR
A = A@-4&^XF
	.IF IDENTICAL 0,<ADDR>
	PUSHL	#0
	.MEXIT
	.ENDC
ERR = 0
	.IIF LESS_EQUAL A-1, ERR=1
	.IIF EQUAL A-5, ERR=1
	.IF EQUAL ERR
	PUSHAL	ADDR
	.IFF
	PUSHL	ADDR
	.ENDC
	.ENDM	PUSHADR
	.PSECT	CODE
	PUSHADR	(R0)
	PUSHADR	(R1)[R4]
	PUSHADR	0
	PUSHADR	#1`

	requireBytes(t, macroBytes(t, src),
		0xDF, 0x60, // PUSHAL (R0)
		0xDF, 0x44, 0x61, // PUSHAL (R1)[R4]
		0xDD, 0x00, // PUSHL #0
		0xDD, 0x01, // PUSHL #1
	)
}

// TestStringOperators checks %LENGTH, %LOCATE, and %EXTRACT, with the
// manual's examples.
func TestStringOperators(t *testing.T) {
	src := `
	.MACRO	OPS	ARG
	.BYTE	%LENGTH(ARG), %LENGTH(<ABCDE>), %length(<>)
	.BYTE	%LOCATE(<D>,<ABCDEF>), %LOCATE(<Z>,<ABCDEF>)
	.BYTE	%LOCATE(<ACE>,<SPACE_HOLDER>), %LOCATE(<ACE>,<SPACE_HOLDER>,5)
	.BYTE	%LOCATE(ARG,<DELDFWDLTDMOESC>)
	.ASCII	/%EXTRACT(2,3,<ABCDEF>)/
	.ASCII	/%EXTRACT(4,9,<ABCDEF>)%EXTRACT(6,1,<ABCDEF>)%EXTRACT(1,0,<ABC>)/
	.ENDM	OPS
	.PSECT	DATA
	OPS	ESC`

	want := []byte{3, 5, 0, 3, 6, 2, 12, 12}
	want = append(want, "CDEEF"...)

	requireBytes(t, macroBytes(t, src), want...)
}

// TestReserve is the manual's RESERVE (%EXTRACT): a symbol one line sets
// is used by a string operator on a later line of the same expansion.
func TestReserve(t *testing.T) {
	src := `
	.MACRO	RESERVE	ARG1
XX = %LOCATE(<=>,ARG1)
	.IF EQUAL XX-%LENGTH(ARG1)
	.MEXIT
	.ENDC
%EXTRACT(0,XX,ARG1)::
XX = XX+1
	.BLKB	%EXTRACT(XX,3,ARG1)
	.ENDM	RESERVE
	.PSECT	DATA
	.BYTE	1
	RESERVE	LOCATION=12
	RESERVE	NOEQUALS
	.BYTE	2`

	a := macroAssembler()

	out, err := a.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}

	if len(out) != 14 || out[13] != 2 {
		t.Errorf("bytes % X, want 1, 12 reserved, 2", out)
	}

	if sym, ok := a.symbols.find("LOCATION"); !ok || sym.value != 1 || sym.flags&SymGlobal == 0 {
		t.Errorf("LOCATION = %+v, want global at 1", sym)
	}
}

func TestStringOperatorErrors(t *testing.T) {
	src := "\t.MACRO M\n\t.BYTE %s\n\t.ENDM\n\tM"

	for text, code := range map[string]uint32{
		"%LENGTH(<A>,<B>)":      vmserrors.VAX_BADOPERATOR,
		"%EXTRACT(1,<A>)":       vmserrors.VAX_BADOPERATOR,
		"%LENGTH(<A>":           vmserrors.VAX_NOCLOSE,
		"%EXTRACT(NOPE,1,<A>)":  vmserrors.VAX_UNDEFSYM,
		"%LOCATE(<A>,<B>,NOPE)": vmserrors.VAX_UNDEFSYM,
	} {
		t.Run(text, func(t *testing.T) {
			requireCode(t, macroErr(t, fmtSrc(src, text)), code)
		})
	}

	// A string operator in a line a conditional leaves out isn't
	// evaluated, and one in a comment is left alone.
	macroBytes(t, "\t.MACRO M\n\t.IF EQ 1\n\t.BYTE %EXTRACT(NOPE,1,<A>)\n\t.ENDC\n\t.BYTE 0 ; %LENGTH(\n\t.ENDM\n\t.PSECT DATA\n\tM")
}

// fmtSrc puts text in src's one %s.
func fmtSrc(src, text string) string {
	for i := 0; i+1 < len(src); i++ {
		if src[i] == '%' && src[i+1] == 's' {
			return src[:i] + text + src[i+2:]
		}
	}

	return src
}
