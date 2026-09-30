package asm

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// Tests for macro definitions and calls (docs/PHASE-28.md, subtask 1).
// Most follow the examples in the VAX MACRO manual's chapter 4, whose
// expansions the manual prints.

// macroBytes assembles src in the MACRO dialect and returns the bytes of
// the psect assembly ended in.
func macroBytes(t *testing.T, src string) []byte {
	t.Helper()

	a := macroAssembler()

	out, err := a.Assemble(src)
	if err != nil {
		t.Fatalf("assemble:\n%s\nerror: %v", src, err)
	}

	return out
}

// macroErr assembles src in the MACRO dialect and returns the error,
// failing the test if there wasn't one.
func macroErr(t *testing.T, src string) error {
	t.Helper()

	_, err := macroAssembler().Assemble(src)
	if err == nil {
		t.Fatalf("assemble %q: expected an error", src)
	}

	return err
}

// store is the manual's STORE macro (§4.1-4.3), with default values.
const store = `
	.MACRO	STORE	ARG1=12,ARG2=0,ARG3=1000
	.LONG	ARG1			; ARG1 is first argument
	.WORD	ARG3			; ARG3 is third argument
	.BYTE	ARG2			; ARG2 is second argument
	.ENDM	STORE
	.PSECT	DATA
`

// storeBytes is what STORE assembles: ARG1 as a longword, ARG3 as a word,
// ARG2 as a byte.
func storeBytes(arg1 uint32, arg3 uint16, arg2 byte) []byte {
	return []byte{
		byte(arg1), byte(arg1 >> 8), byte(arg1 >> 16), byte(arg1 >> 24),
		byte(arg3), byte(arg3 >> 8),
		arg2,
	}
}

func TestMacroArguments(t *testing.T) {
	tests := []struct {
		call string
		want []byte
	}{
		// Positional arguments (§4.1).
		{"STORE 3,2,1", storeBytes(3, 1, 2)},
		// Separated by blanks instead of commas.
		{"STORE 3 2 1", storeBytes(3, 1, 2)},
		// Default values (§4.2).
		{"STORE", storeBytes(12, 1000, 0)},
		{"STORE ,5,7", storeBytes(12, 7, 5)},
		{"STORE 1", storeBytes(1, 1000, 0)},
		// Keyword arguments, in any order (§4.3).
		{"STORE ARG3=27+5/4,ARG2=5,ARG1=9", storeBytes(9, 32/4, 5)},
		// A positional and a keyword argument for the same formal
		// argument: the later one wins.
		{"STORE 1,ARG1=2", storeBytes(2, 1000, 0)},
		{"STORE ARG1=2,1", storeBytes(1, 1000, 0)},
		// A bracketed argument keeps its separators.
		{"STORE <1 + 2>", storeBytes(3, 1000, 0)},
		// Brackets that don't enclose the whole argument are an
		// expression's, and stay.
		{"STORE <1+2>*3", storeBytes(9, 1000, 0)},
		// Lowercase in the call is uppercased like any source.
		{"store arg2=4", storeBytes(12, 1000, 4)},
	}

	for _, tc := range tests {
		t.Run(tc.call, func(t *testing.T) {
			requireBytes(t, macroBytes(t, store+"\t"+tc.call), tc.want...)
		})
	}
}

func TestMacroTooManyArguments(t *testing.T) {
	err := macroErr(t, store+"\tSTORE 1,2,3,4")
	requireCode(t, err, vmserrors.VAX_TOOMNYARGS)
}

// TestMacroStringArguments is §4.4's REPEAT: the argument is substituted
// inside a string, and the unbracketed call has too many arguments.
func TestMacroStringArguments(t *testing.T) {
	const repeat = `
	.MACRO	REPEAT STRNG
	.ASCII	/STRNG/
	.ASCII	/STRNG/
	.ENDM	REPEAT
	.PSECT	DATA
`

	requireBytes(t, macroBytes(t, repeat+"\tREPEAT <A B>"), 'A', ' ', 'B', 'A', ' ', 'B')
	requireBytes(t, macroBytes(t, repeat+"\tREPEAT ^%<X>%"), '<', 'X', '>', '<', 'X', '>')
	requireCode(t, macroErr(t, repeat+"\tREPEAT A B"), vmserrors.VAX_TOOMNYARGS)
}

// TestNestedMacroCalls is §4.4's CNTRPT and CNTRPT2: a macro passes its
// argument on to another, either bracketed in the definition or with
// nested brackets in the call.
func TestNestedMacroCalls(t *testing.T) {
	const macros = `
	.MACRO	REPEAT STRNG
	.ASCII	/STRNG/
	.ASCII	/STRNG/
	.ENDM	REPEAT
	.MACRO	CNTRPT LAB1,LAB2,STR_ARG
LAB1:	.BYTE	LAB2-LAB1-1
	REPEAT	<STR_ARG>
LAB2:
	.ENDM	CNTRPT
	.MACRO	CNTRPT2 LAB1,LAB2,STR_ARG
LAB1:	.BYTE	LAB2-LAB1-1
	REPEAT	STR_ARG
LAB2:
	.ENDM	CNTRPT2
	.PSECT	DATA
`
	// The length byte, then the string twice. The length is a forward
	// reference (LAB2 comes after it), which the one-pass assembler
	// leaves to the linker as a relocation, so Bytes has 0 there.
	want := append([]byte{0}, "AB CAB C"...)

	requireBytes(t, macroBytes(t, macros+"\tCNTRPT ST,FIN,<AB C>"), want...)
	requireBytes(t, macroBytes(t, macros+"\tCNTRPT2 BEG,TERM,<<AB C>>"), want...)
}

// TestMacroConcatenation is §4.5's CONCAT: an apostrophe joins an argument
// to the text beside it, and two join two arguments.
func TestMacroConcatenation(t *testing.T) {
	a := macroAssembler()

	out, err := a.Assemble(`
	.MACRO CONCAT INST,SIZE,NUM
TEST'NUM':
	INST''SIZE	R0,R'NUM
TEST'NUM'X:
	.ENDM CONCAT
	.PSECT CODE
	CONCAT MOV,L,5`)
	if err != nil {
		t.Fatal(err)
	}

	requireBytes(t, out, 0xD0, 0x50, 0x55) // MOVL R0,R5

	for name, want := range map[string]uint32{"TEST5": 0, "TEST5X": 3} {
		if sym, ok := a.symbols.find(name); !ok || sym.value != want {
			t.Errorf("%s = %+v, want %d", name, sym, want)
		}
	}
}

// TestMacroSubstitutionRules checks what substitute replaces: whole names
// only, in any case, everywhere in the line.
func TestMacroSubstitutionRules(t *testing.T) {
	m := &macroDef{name: "M", formals: []formal{{name: "A"}, {name: "B"}}}
	values := []string{"X", "Y"}

	for line, want := range map[string]string{
		"MOVL A,B":           "MOVL X,Y",
		"MOVL a,b":           "MOVL X,Y",
		"AB A.B A$ 1A":       "AB A.B A$ 1A",
		"P'A'Q":              "PXQ",
		"A''B":               "XY",
		"A''C":               "X'C",
		".ASCII /A/ ; A B":   ".ASCII /X/ ; X Y",
		".IF DF FAB$C_'A":    ".IF DF FAB$C_X",
		"$EQU PREFIX''SYM,1": "$EQU PREFIX''SYM,1",
	} {
		if got := substitute(line, m, values); got != want {
			t.Errorf("substitute(%q) = %q, want %q", line, got, want)
		}
	}
}

// TestMacroDefinesMacro checks a macro that defines another macro when
// it's called, as VMS's $GBLINI does, including an apostrophe meant for
// the inner macro (PREFIX”SYM, from $EQULST).
func TestMacroDefinesMacro(t *testing.T) {
	src := `
	.MACRO	DEFSYM	PREFIX
	.MACRO	SETSYM	SYM,VAL
	PREFIX''SYM = VAL
	.ENDM	SETSYM
	.ENDM	DEFSYM
	DEFSYM	ABC_
	SETSYM	ONE,1
	SETSYM	TWO,2
	.PSECT	DATA
	.LONG	ABC_ONE, ABC_TWO`

	requireBytes(t, macroBytes(t, src), 1, 0, 0, 0, 2, 0, 0, 0)
}

// TestMacroRedefinesItself is the manual's USERDEF (.MACRO note 3): the
// first call redefines the macro to nothing, so a second call assembles
// nothing.
func TestMacroRedefinesItself(t *testing.T) {
	src := `
	.MACRO USERDEF
	.BYTE 1
	.MACRO USERDEF
	.ENDM USERDEF
	.ENDM USERDEF
	.PSECT DATA
	USERDEF
	USERDEF
	.BYTE 2`

	requireBytes(t, macroBytes(t, src), 1, 2)
}

// TestMacroLabeledEndm: a label on the .ENDM line is part of the body,
// labeling what follows the expansion (the manual's POSITIVE macro).
func TestMacroLabeledEndm(t *testing.T) {
	src := `
	.MACRO	POSITIVE ARG1,L1
	TSTL	ARG1
	BGEQ	L1
	MNEGL	ARG1,ARG1
L1:	.ENDM	POSITIVE
	.PSECT	CODE
	POSITIVE R0,10$
	RSB`

	requireBytes(t, macroBytes(t, src),
		0xD5, 0x50, // TSTL R0
		0x18, 0x03, // BGEQ 10$
		0xCE, 0x50, 0x50, // MNEGL R0,R0
		0x05, // 10$: RSB
	)
}

// TestMacroContinuedDefinition: a .MACRO line continued onto the next, as
// $QIOW_S's is in STARLET.MLB.
func TestMacroContinuedDefinition(t *testing.T) {
	src := `
	.MACRO	TWO A=1,-
		B=2
	.BYTE	A,B
	.ENDM	TWO
	.PSECT	DATA
	TWO	B=5`

	requireBytes(t, macroBytes(t, src), 1, 5)
}

// TestMexit checks .MEXIT: the rest of the expansion is skipped, the
// conditional block it's in is closed, and assembly goes on after the
// call.
func TestMexit(t *testing.T) {
	src := `
	.MACRO	POLO	N
	.IF	EQ	N
	.BYTE	0
	.MEXIT
	.ENDC
	.BYTE	N
	.ENDM	POLO
	.PSECT	DATA
	POLO	0
	POLO	3
	.BYTE	9`

	requireBytes(t, macroBytes(t, src), 0, 3, 9)

	requireCode(t, macroErr(t, "\t.MEXIT"), vmserrors.VAX_NOTINMACRO)
}

// TestMacroReplacesOpcode: a macro with an instruction's name is used
// instead of the instruction, until it's deleted.
func TestMacroReplacesOpcode(t *testing.T) {
	src := `
	.PSECT	CODE
	.MACRO	NOP
	HALT
	.ENDM	NOP
	NOP
	.MDELETE NOP
	NOP`

	requireBytes(t, macroBytes(t, src), 0x00, 0x01)
}

func TestMdelete(t *testing.T) {
	err := macroErr(t, "\t.MACRO M\n\t.ENDM\n\t.MDELETE M, OTHER\n\tM")
	requireCode(t, err, vmserrors.VAX_BADOPCODE)
}

func TestMacroDefinitionErrors(t *testing.T) {
	requireCode(t, macroErr(t, "\t.MACRO M\n\t.BYTE 1"), vmserrors.VAX_NOENDM)
	requireCode(t, macroErr(t, "\t.MACRO M\n\t.ENDM N"), vmserrors.VAX_ENDMNAME)
	requireCode(t, macroErr(t, "\t.ENDM"), vmserrors.VAX_NOTINDEF)
	requireCode(t, macroErr(t, "\t.MACRO"), vmserrors.VAX_MACRONAME)
	requireCode(t, macroErr(t, "\t.MACRO M ?"), vmserrors.VAX_BADFORMAL)
	requireCode(t, macroErr(t, "\t.MACRO M\n\tM\n\t.ENDM\n\tM"), vmserrors.VAX_MACRODEPTH) //nolint:dupword

	// A definition started in a macro's expansion has to end there. (A
	// .MACRO behind .IIF isn't counted while M is collected, so M's
	// body starts N's definition without ending it.)
	err := macroErr(t, "\t.MACRO M\n\t.IIF EQ 0, .MACRO N\n\t.ENDM M\n\tM\n\t.ENDM N") //nolint:dupword
	requireCode(t, err, vmserrors.VAX_NOENDM)
}

// TestMacroErrorLocation: an error in an expansion names the call's line,
// then the macro and the line in its expansion.
func TestMacroErrorLocation(t *testing.T) {
	src := strings.Join([]string{
		"\t.MACRO\tOUTER",   // 1
		"\t.BYTE\t1",        // 2 (OUTER's line 1)
		"\tINNER",           // 3 (OUTER's line 2)
		"\t.ENDM\tOUTER",    // 4
		"\t.MACRO\tINNER",   // 5
		"\t.BYTE\tUNKNOWN(", // 6 (INNER's line 1)
		"\t.ENDM\tINNER",    // 7
		"\t.PSECT\tDATA",    // 8
		"\tOUTER",           // 9
	}, "\n")

	err := macroErr(t, src)

	var line *Error
	if !errors.As(err, &line) || line.Line != 9 {
		t.Fatalf("error %v: want one at line 9", err)
	}

	outer, ok := line.Err.(*ExpansionError)
	if !ok || outer.Macro != "OUTER" || outer.Line != 2 {
		t.Fatalf("error %v: want one in OUTER's line 2", err)
	}

	inner, ok := outer.Err.(*ExpansionError)
	if !ok || inner.Macro != "INNER" || inner.Line != 1 {
		t.Fatalf("error %v: want one in INNER's line 1", err)
	}

	want := "line 9: in expansion of macro OUTER, line 2: in expansion of macro INNER, line 1: "
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error text %q, want it to start %q", err.Error(), want)
	}

	// The console dialect (which has no .PSECT) stops at the error,
	// with the same location.
	_, err = New(false).Assemble(strings.Replace(src, "\t.PSECT\tDATA", "\t.BYTE\t0", 1))
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("console error %v, want it to start %q", err, want)
	}
}

// TestSymbolsWithDots: MACRO-32 symbols may contain ".", even first, as
// VMS's system macros' BIT..., $$.TAB, and .LEN do.
func TestSymbolsWithDots(t *testing.T) {
	src := `
	.PSECT	DATA
BIT... = 3
$$.TAB = .
.LEN = BIT...+1
	.BYTE	BIT..., .LEN
	.LONG	. - $$.TAB`

	requireBytes(t, macroBytes(t, src), 3, 4, 2, 0, 0, 0)
}

// TestConsoleDialectMacros: the console's ASM has macros too, with or
// without the directives' ".", and at the interactive prompt.
func TestConsoleDialectMacros(t *testing.T) {
	requireBytes(t, assembleBytes(t, "MACRO TWICE X\nBYTE X,X\nENDM\nTWICE 7"), 7, 7)

	a := New(false)
	a.BeginInteractive()

	for _, line := range []string{".MACRO THREE X", ".BYTE X,X,X", ".ENDM THREE", "THREE 4"} {
		if _, err := a.AssembleLine(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	requireBytes(t, a.Bytes(), 4, 4, 4)
}

// TestMacroBodyBlocks: a repeat block inside a definition is collected
// with it, whether it ends with .ENDR or, as some of VMS's STARLET.MLB
// macros' do, with .ENDM.
func TestMacroBodyBlocks(t *testing.T) {
	a := macroAssembler()

	_, err := a.Assemble(`
	.MACRO	M	LIST
	.IRP	X,<LIST>
	.BYTE	X
	.ENDM
	.IRPC	X,<LIST>
	.ENDR
	.ENDM	M`)
	if err != nil {
		t.Fatal(err)
	}

	if m := a.macros["M"]; m == nil || len(m.body) != 5 {
		t.Fatalf("M = %+v, want a 5-line body", m)
	}
}

// TestArgumentCase checks that a call's arguments are passed as written,
// as real MACRO passes them: in a string their case shows, and elsewhere
// the expansion is uppercased like any source line. Keyword names, and
// .IRP's and .IRPC's formal names, match in either case.
func TestArgumentCase(t *testing.T) {
	requireBytes(t, macroBytes(t, `	.MACRO	TEXT	S, N=1
	.ASCII	/S/
	.BYTE	N
	.ENDM	TEXT
	.PSECT	DATA
val = 7
	text	<Hi there>, n=val
	TEXT	aBc, -
		N=2
	.IRP	x, <a, B>
	.ASCII	/x/
	.ENDR
	.IRPC	ch, <yZ>
	.ASCII	/CH/
	.ENDR
	TEXT	\val`),
		'H', 'i', ' ', 't', 'h', 'e', 'r', 'e', 7,
		'a', 'B', 'c', 2,
		'a', 'B',
		'y', 'Z',
		'7', 1)
}

// TestArgumentCaseLibraryMacro: a call that loads its macro from a
// library passes its arguments as written too.
func TestArgumentCaseLibraryMacro(t *testing.T) {
	a := macroAssembler()
	a.SetMacroLibraries(newMapLibrary(map[string]string{
		"SAY": "\t.MACRO\tSAY\tS\n\t.ASCII\t/S/\n\t.ENDM\tSAY",
	}))

	out, err := a.Assemble("\t.PSECT\tDATA\n\tSAY\t<Mixed Case>")
	if err != nil {
		t.Fatal(err)
	}

	requireBytes(t, out, 'M', 'i', 'x', 'e', 'd', ' ', 'C', 'a', 's', 'e')
}
