//go:build ignore

// Command gen writes the Phase 80 probe of LIB$TPARSE, LIB$TABLE_PARSE, and
// $FILESCAN (docs/PHASE-80 - FORTRAN formatted IO.md, subtask 10). Run it from
// the repository root:
//
//	go run testdata/tpa80/gen.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A grammar is states of a table, each case an input to run from its first
// label: raw lines of the table, and the inputs.
type grammar struct {
	name   string
	lines  []string
	inputs []input
}

type input struct {
	text    string
	options uint32
}

// in makes inputs with no options.
func in(texts ...string) []input {
	out := make([]input, len(texts))
	for i, t := range texts {
		out[i] = input{text: t}
	}

	return out
}

// with makes inputs with these options.
func with(options uint32, texts ...string) []input {
	out := in(texts...)
	for i := range out {
		out[i].options = options
	}

	return out
}

func join(lists ...[]input) []input {
	var out []input
	for _, l := range lists {
		out = append(out, l...)
	}

	return out
}

const (
	blanks  = 1
	abbrev  = 2
	abbrfm  = 4
	mcount2 = 2 << 24
	mcount3 = 3 << 24
)

// exitWith is a state of one transition of a type, with an action routine and
// msk-adr, so both what it matched and what it stored show.
func exitWith(kind string) []string {
	return []string{"$TRAN " + kind + ",TPA$_EXIT,ACT,,CELL,1"}
}

var grammars = []grammar{
	{"CHAR", []string{"$TRAN 'A',TPA$_EXIT,ACT,,CELL,1"}, in("A", "AB", "B", "a", " A", "", "\tA")},
	{"KW", []string{"$TRAN 'ABC',TPA$_EXIT,ACT,,CELL,1"}, in("ABC", "ABCD", "ABC+", "ABC D", "AB", "abc", " ABC ", "")},
	{"KW1", []string{"$TRAN 'A*',TPA$_EXIT,ACT,,CELL,1"}, in("A", "AB", "A+", "A B", "B")},
	{"ANY", exitWith("TPA$_ANY"), join(in("A", "+", " A", ""), with(blanks, " A"))},
	{"ALPHA", exitWith("TPA$_ALPHA"), in("A", "z", "1", "+")},
	{"DIGIT", exitWith("TPA$_DIGIT"), in("0", "9", "A", "12")},
	{"STRING", exitWith("TPA$_STRING"), in("AB12+", "a1", "$AB", "_A", "12", "+", "")},
	{"SYMBOL", exitWith("TPA$_SYMBOL"), in("A$B_C+", "$", "1A", "A.B", "_", "+", "")},
	{"BLANK", exitWith("TPA$_BLANK"), join(in(" A"), with(blanks, " A", "  \t A", "A", ""))},
	{"DECIMAL", exitWith("TPA$_DECIMAL"), in("123", "123A", "4294967295", "4294967296", "0", "A", "-1", "007", "")},
	{"OCTAL", exitWith("TPA$_OCTAL"), in("777", "778", "8", "37777777777", "40000000000", "A")},
	{"HEX", exitWith("TPA$_HEX"), in("7FF", "7ff", "G", "FFFFFFFF", "100000000", "7FFG", "abc")},
	{"EOS", exitWith("TPA$_EOS"), in("", "A", " ")},
	{"LAMBDA", exitWith("TPA$_LAMBDA"), in("", "A", " A")},
	{"UIC", exitWith("TPA$_UIC"), in("[1,2]", "<1,2>", "[*,2]", "[1,*]", "[*,*]", "[177777,177777]",
		"[200000,1]", "[1,2", "[1 ,2]", "[8,1]", "[1,2>", "[,2]", "[1,2]X", "1,2")},
	{"IDENT", exitWith("TPA$_IDENT"), in("[1,4]", "[SYSTEM]", "SYSTEM", "%X10", "%X80010000", "NOSUCH", "*",
		"[*,*]", "<SYSTEM>", "[NOSUCH]", "123", "[SYSTEM,SYSTEM]", "%XG")},
	{"FILESPEC", exitWith("TPA$_FILESPEC"), in("A.B;1", "DEV:[DIR]A.B;1 X", "NODE::DEV:A", "A.B.1", "[A.B]", "[A.B",
		"*.*;*", "A,B", "A B", "+", `NODE"u p"::X`, "[A.][B]C", "", "A:B:C", ".B", ";1")},

	// Targets, falling through, and final states.
	{"NEXT", []string{"$TRAN 'A'", "$TRAN 'C'", "$STATE", "$TRAN 'B',TPA$_EXIT"}, in("AB", "CB", "A", "B", "ACB")},
	{"LOOP", []string{"$TRAN 'A',LOOP", "$TRAN TPA$_EOS,TPA$_EXIT"}, in("AAA", "AAB", "")},
	{"FAILT", []string{"$TRAN 'A',TPA$_FAIL", "$TRAN 'B',TPA$_EXIT"}, in("AX", "BX", "CX")},
	{"FAILA", []string{"$TRAN 'ABC',TPA$_FAIL,ACT,,CELL,1"}, in("ABC D", "X")},
	{"FAILS", []string{"$TRAN 'A',TPA$_FAIL,ACT,,,^X1230"}, in("A")},

	// An action routine's answers.
	{"REJECT", []string{"$TRAN 'A',TPA$_EXIT,ACT,,CELL,0", "$TRAN TPA$_ANY,TPA$_EXIT,ACT,,,1"}, in("A", "B", "")},
	{"ALT", []string{"$TRAN 'A',TPA$_EXIT,ACT,,CELL,^X1230"}, in("A", "B")},
	{"ALT2", []string{"$TRAN 'A',TPA$_EXIT,ACT,,,^X1230", "$TRAN 'A',TPA$_EXIT,ACT,,,^X1240"}, in("A")},
	{"ALT3", []string{"$TRAN 'A',TPA$_EXIT,ACT,,,^X1230", "$TRAN 'A',TPA$_EXIT,ACT,,,0"}, in("A")},
	{"ALT4", []string{"$TRAN 'A',,ACT,,,^X1230", "$TRAN 'A'", "$STATE", "$TRAN 'B',TPA$_EXIT"}, in("AB", "AC")},
	{"ODD", []string{"$TRAN 'A',TPA$_EXIT,ACT,,,3"}, in("A")},
	{"SIG", []string{"$TRAN 'A',TPA$_EXIT,ACTSIG"}, in("A")},
	{"SKIP", []string{"$TRAN 'A',,ACTSKIP", "$STATE", "$TRAN 'C',TPA$_EXIT"}, in("ABC", "AC")},
	{"NOARG", []string{"$TRAN 'A',TPA$_EXIT,ACT"}, in("A")},
	{"BLKOFF", []string{"$TRAN TPA$_BLANK,,ACTOFF", "$TRAN TPA$_ANY,BLKOFF", "$STATE", "$TRAN 'X',TPA$_EXIT"},
		with(blanks, "AB  X", "AB X ", "ABX")},
	{"BLKON", []string{"$TRAN 'A',,ACTON", "$STATE", "$TRAN TPA$_BLANK,TPA$_EXIT"}, in("A B", "AB", " A")},

	// Masks and what msk-adr stores.
	{"MASK", []string{"$TRAN 'A',TPA$_EXIT,,^X0F,CELL"}, in("A")},
	{"MASKA", []string{"$TRAN 'A',TPA$_EXIT,ACT,^XF0,CELL,1"}, in("A")},
	{"MASKR", []string{"$TRAN 'A',TPA$_EXIT,ACT,^XF0,CELL,0"}, in("A")},
	{"MASKN", []string{"$TRAN TPA$_DECIMAL,TPA$_EXIT,,^X0F,CELL"}, in("12")},
	{"MASKS", []string{"$TRAN 'AB',TPA$_EXIT,,^X0F,CELL"}, in("AB")},

	// Subexpressions: the manual's two examples, then each way one ends.
	{"QUOTE", []string{"$TRAN TPA$_ANY,,,,QCHAR", "$STATE", "$TRAN !QSTR,,,,CELL", "$STATE", "$TRAN TPA$_ANY,TPA$_EXIT",
		"$STATE QSTR", "$TRAN TPA$_ANY,QSTR,TESTQ", "$TRAN TPA$_LAMBDA,TPA$_EXIT"},
		join(with(blanks, "/ABC/", "//", "/AB"), in("/A B/"))},
	{"LOOK", []string{"$TRAN !OCTNUM,TPA$_EXIT,ACT,,CELL,1", "$TRAN !DECNUM,TPA$_EXIT,ACT,,CELL,1", "$TRAN !HEXNUM,TPA$_EXIT,ACT,,CELL,1",
		"$STATE OCTNUM", "$TRAN TPA$_OCTAL", "$STATE", "$TRAN '/'", "$STATE", "$TRAN 'OCTAL',TPA$_EXIT",
		"$STATE DECNUM", "$TRAN TPA$_DECIMAL", "$STATE", "$TRAN '/'", "$STATE", "$TRAN 'DECIMAL',TPA$_EXIT",
		"$STATE HEXNUM", "$TRAN TPA$_HEX", "$STATE", "$TRAN '/'", "$STATE", "$TRAN 'HEX',TPA$_EXIT"},
		in("10/OCTAL", "32768/DECIMAL", "77AF/HEX", "10/HEX", "99/OCTAL", "ZZ/HEX", "10/")},
	{"SUBEND", []string{"$TRAN !SUB1,,ACT,,CELL,1", "$STATE", "$TRAN TPA$_EOS,TPA$_EXIT", "$STATE SUB1", "$TRAN 'A',TPA$_EXIT"},
		in("A", " A", "A ", "B", "AB")},
	{"SUBFAIL", []string{"$TRAN !SUB2,TPA$_EXIT", "$TRAN TPA$_ANY,TPA$_EXIT,ACT,,,1", "$STATE SUB2", "$TRAN 'A'", "$STATE", "$TRAN 'B',TPA$_FAIL"},
		in("AB", "AC", "XY")},
	{"SUBALT", []string{"$TRAN !SUB3,TPA$_EXIT", "$STATE SUB3", "$TRAN 'A',TPA$_EXIT,ACT,,,^X1230"}, in("A", "B")},
	{"SUBALT2", []string{"$TRAN !SUB4,TPA$_EXIT", "$TRAN 'A',TPA$_EXIT", "$STATE SUB4", "$TRAN 'A',TPA$_EXIT,ACT,,,^X1230"}, in("A")},
	{"SUBSIDE", []string{"$TRAN !SUB5,TPA$_EXIT", "$TRAN TPA$_DECIMAL,TPA$_EXIT", "$STATE SUB5", "$TRAN TPA$_DECIMAL,,ACT,^X0F,CELL,1", "$STATE", "$TRAN 'X',TPA$_EXIT"},
		in("12X", "12Y")},
	{"SUBNEST", []string{"$TRAN !SUB6,,,,CELL", "$STATE", "$TRAN TPA$_EOS,TPA$_EXIT", "$STATE SUB6", "$TRAN '('", "$STATE", "$TRAN !SUB6", "$TRAN 'A'", "$STATE", "$TRAN ')',TPA$_EXIT"},
		in("(A)", "((A))", "(((A)))", "((A)", "A")},
	{"SUBLAST", []string{"$TRAN !SUB7,TPA$_EXIT,ACT,,,1", "$STATE SUB7", "$TRAN TPA$_DECIMAL,,ACT,,,1", "$STATE", "$TRAN 'A',TPA$_EXIT,ACT,,,1"}, in("12A")},

	// Keywords' abbreviations, an ambiguous one, and the next state's flag.
	{"ABR", []string{"$TRAN 'ALPHA',TPA$_EXIT,ACT,,,1", "$TRAN 'BETA',TPA$_EXIT,ACT,,,1", "$TRAN 'BETTER',TPA$_EXIT,ACT,,,1",
		"$TRAN 'CAT',TPA$_EXIT,ACT,,,1", "$TRAN TPA$_SYMBOL,AFTER,ACT,,,1", "$STATE AFTER", "$TRAN TPA$_LAMBDA,TPA$_EXIT,ACT,,,1"},
		func() []input {
			var out []input
			for _, o := range []uint32{0, abbrev, abbrfm, abbrev | abbrfm, mcount2, mcount3, mcount2 | abbrev} {
				out = append(out, with(o, "AL", "A", "BE", "BET", "BETT", "BETTERX", "C", "CATX", "CAT", "BETA", "beta")...)
			}

			return out
		}()},
	{"ABR2", []string{"$TRAN 'GET',TPA$_EXIT,ACT,,,1", "$TRAN 'GETX',TPA$_EXIT,ACT,,,1", "$TRAN 'GETY',TPA$_EXIT,ACT,,,1"},
		with(abbrev, "G", "GE", "GET", "GETX", "GETZ")},
	{"ABR3", []string{"$TRAN 'ONE',TPA$_EXIT,ACT,,,1", "$STATE", "$TRAN 'ONES',TPA$_EXIT,ACT,,,1"}, with(abbrev, "O", "ONE")},
	{"ABR4", []string{"$TRAN 'ONE'", "$TRAN 'ONES'", "$TRAN 'TWO',,ACT,,,1", "$STATE", "$TRAN 'THREE',TPA$_EXIT,ACT,,,1"}, with(abbrev, "T THREE", "O T", "ON TH")},

	// A numeric character code, and symbols for ones a macro call can't hold.
	{"CODES", []string{"$TRAN LETA,TPA$_EXIT,ACT,,,1", "$TRAN COMMA,TPA$_EXIT,ACT,,,1", "$TRAN <','>,TPA$_EXIT,ACT,,,1"}, in("A", ",", "B")},

	// The manual's CREATE/DIRECTORY example, its actions left out.
	{"UFD", []string{"$TRAN TPA$_BLANK,,ACTOFF", "$TRAN TPA$_ANY,UFD",
		"$STATE", "$TRAN TPA$_SYMBOL,,,,CELL", "$STATE", "$TRAN ':'",
		"$STATE", "$TRAN !UFDU", "$TRAN !UFDN",
		"$STATE UFDO", "$TRAN '/'", "$TRAN TPA$_EOS,TPA$_EXIT",
		"$STATE", "$TRAN 'OWNER_UIC',UFDP", "$TRAN 'ENTRIES',UFDE", "$TRAN 'PROTECTION',UFDR",
		"$STATE UFDP", "$TRAN ':'", "$TRAN '='", "$STATE", "$TRAN !UFDU,UFDO",
		"$STATE UFDE", "$TRAN ':'", "$TRAN '='", "$STATE", "$TRAN TPA$_DECIMAL,UFDO,ACT,,,1",
		"$STATE UFDR", "$TRAN ':'", "$TRAN '='", "$STATE", "$TRAN '('",
		"$STATE UFDX", "$TRAN 'SYSTEM',UFDY", "$TRAN 'OWNER',UFDY", "$TRAN 'GROUP',UFDY", "$TRAN 'WORLD',UFDY",
		"$STATE UFDY", "$TRAN ':'", "$TRAN '='",
		"$STATE UFDZ", "$TRAN 'R',UFDZ,,^X1,CELL", "$TRAN 'W',UFDZ,,^X2,CELL", "$TRAN 'E',UFDZ,,^X4,CELL", "$TRAN 'D',UFDZ,,^X8,CELL", "$TRAN TPA$_LAMBDA",
		"$STATE", "$TRAN <','>,UFDX", "$TRAN ')',UFDO",
		"$STATE UFDU", "$TRAN '['", "$STATE", "$TRAN TPA$_OCTAL", "$STATE", "$TRAN <','>", "$STATE", "$TRAN TPA$_OCTAL", "$STATE", "$TRAN ']',TPA$_EXIT",
		"$STATE UFDN", "$TRAN '['", "$STATE UFDM", "$TRAN TPA$_STRING,,ACT,,,1", "$STATE", "$TRAN '.',UFDM", "$TRAN ']',TPA$_EXIT"},
		with(blanks|abbrev, "CREATE/DIR DEV:[A.B.C]", "CREATE/DIR DEV:[202,31]/ENTRIES=100",
			"CREATE/DIR DEV:[A]/OWN=[1,2]/PROT=(SYSTEM:R,OWNER:RWED,GROUP:R,WORLD:R)",
			"CREATE/DIR DEV:[A]/E=1/P=(S:R)", "CREATE/DIR DEV:[A]/X", "CREATE/DIR DEV:A")},
}

// rawTypes are type bytes no macro makes, each in a transition of its own, the
// last of its state, with TPA$_EXIT as its target.
var rawTypes = []int{219, 220, 227, 228, 229, 230, 231, 232, 233, 249, 250, 255}

// rawInputs are what each is run on.
var rawInputs = []string{"12", "NODE::A", `NODE"U P"::A`}

// counts are argument blocks' counts other than 8.
var counts = []int{0, 1, 7, 9, 16, 255, 256, -1}

// fileScans are $FILESCAN's cases.
var fileScans = []string{
	"", "A", "A.B", "A.B;1", "A.B.1", "A;1", ";1", ".B", ".", ";",
	"DEV:", "DEV:A", "DEV:[DIR]", "DEV:[DIR]A.B;1", "[DIR]A", "<DIR>A", "[A.B.C]", "[A.B", "[A,B]", "[1,2]X",
	"[ROOT.][DIR]A", "[ROOT.]A", "[.A]", "[-.A]", "[...]", "[A...]",
	"NODE::", "NODE::A", "NODE::DEV:[DIR]A.B;1", "N1::N2::A", `NODE"U P"::A`, `NODE"U P"::N2"X"::A`, `NODE::"FILE SPEC"`, `NODE"U`,
	"A B", "A,B", "A+B", "+", "*.*;*", "A%.B*", "A.B;-1", "A.B;*", "A:B:C", "::", "_DEV:A", "$A_B-C.D",
	"a.b;1", "A.B;1X", "A.B.C.D", "A..B", "DEV:DEV2:A", "[A][B]",
}

// symbols are the definitions the program shows, whether or not VMS has them.
var symbols = strings.Fields(`TPA$K_COUNT0 TPA$K_LENGTH0 TPA$L_COUNT TPA$L_OPTIONS TPA$B_MCOUNT TPA$L_STRINGCNT
TPA$L_STRINGPTR TPA$L_TOKENCNT TPA$L_TOKENPTR TPA$B_CHAR TPA$L_NUMBER TPA$L_PARAM
TPA$V_BLANKS TPA$M_BLANKS TPA$V_ABBREV TPA$M_ABBREV TPA$V_ABBRFM TPA$M_ABBRFM TPA$V_AMBIG TPA$M_AMBIG
TPA$_EXIT TPA$_FAIL TPA$_KEYWORD TPA$_NODE_ACS TPA$_NODE_PRIMARY TPA$_NODE TPA$_FILESPEC TPA$_UIC TPA$_IDENT
TPA$_ANY TPA$_ALPHA TPA$_DIGIT TPA$_STRING TPA$_SYMBOL TPA$_BLANK TPA$_DECIMAL TPA$_OCTAL TPA$_HEX TPA$_LAMBDA
TPA$_EOS TPA$_SUBEXPR TPA$_DECIMAL_64 TPA$_OCTAL_64 TPA$_HEX_64 TPA64$K_COUNT0 TPA64$K_LENGTH0
FSCN$_FILESPEC FSCN$_NODE FSCN$_DEVICE FSCN$_ROOT FSCN$_DIRECTORY FSCN$_NAME FSCN$_TYPE FSCN$_VERSION
FSCN$_NODE_PRIMARY FSCN$_NODE_ACS FSCN$_NODE_SECONDARY
FSCN$M_NODE FSCN$V_NODE FSCN$M_DEVICE FSCN$V_DEVICE FSCN$M_ROOT FSCN$V_ROOT FSCN$M_DIRECTORY FSCN$V_DIRECTORY
FSCN$M_NAME FSCN$V_NAME FSCN$M_TYPE FSCN$V_TYPE FSCN$M_VERSION FSCN$V_VERSION FSCN$M_NODE_PRIMARY FSCN$V_NODE_PRIMARY
FSCN$M_NODE_ACS FSCN$V_NODE_ACS FSCN$M_NODE_SECONDARY FSCN$V_NODE_SECONDARY`)

// mapTypes are the symbol types tried on every character.
var mapTypes = []string{"TPA$_ANY", "TPA$_ALPHA", "TPA$_DIGIT", "TPA$_STRING", "TPA$_SYMBOL", "TPA$_BLANK", "TPA$_DECIMAL", "TPA$_OCTAL", "TPA$_HEX", "TPA$_FILESPEC", "TPA$_IDENT"}

const prologue = `	.TITLE	TPA80	Phase 80 LIB$TPARSE and $FILESCAN probe
	.IDENT	/V1.0/

; Written by testdata/tpa80/gen.go: do not edit. Runs LIB$TPARSE and
; LIB$TABLE_PARSE on state tables made by the macros and on bytes it holds
; itself, and $FILESCAN, printing what each returned and stored.

	$SSDEF
	$TPADEF
	$FSCNDEF

COMMA = 44
EIGHT = 8			; round 1: a type starting with a digit is refused
TWO = 1+1
LETA = 65

	.MACRO	P CTR,A,B,C,D,E,F,G,H
	.SAVE_PSECT LOCAL_BLOCK
	.PSECT	TEXT,NOEXE,NOWRT
	.NCHR	$$$PN,<CTR>
$$$PD = .
	.LONG	$$$PN
	.ADDRESS $$$PD+8
	.ASCII	~CTR~
	.RESTORE_PSECT
	$$$PC = 0
	.IRP	X,<H,G,F,E,D,C,B,A>
	.IF	NOT_BLANK <X>
	PUSHL	X
	$$$PC = $$$PC+1
	.ENDC
	.ENDR
	PUSHAB	$$$PD
	CALLS	#$$$PC+1,PRINT
	.ENDM	P

	.PSECT	DATA,NOEXE,WRT,QUAD
BLOCK:	.BLKL	12		; the argument block, and room past it
CELL:	.BLKL	2		; msk-adr's place
QCHAR:	.BLKL	1		; the quote example's character
SIGCOND:	.LONG	0		; the condition HANDLER was given
STRBASE:	.LONG	0		; the input string's address
OUTLEN:	.WORD	0
OUTD:	.LONG	200
	.ADDRESS OUTB
OUTB:	.BLKB	200
LINED:	.LONG	0
	.ADDRESS OUTB
CH:	.BLKB	1
	.ALIGN	LONG
MAPL:	.BLKB	64
MAPD:	.LONG	64
	.ADDRESS MAPL
SCANI:	.WORD	0,1
	.LONG	0
	.WORD	0,2
	.LONG	0
	.WORD	0,3
	.LONG	0
	.WORD	0,4
	.LONG	0
	.WORD	0,5
	.LONG	0
	.WORD	0,6
	.LONG	0
	.WORD	0,7
	.LONG	0
	.WORD	0,8
	.LONG	0
	.WORD	0,9
	.LONG	0
	.WORD	0,10
	.LONG	0
	.WORD	0,11
	.LONG	0
	.LONG	0
SCANBAD:	.WORD	0,12
	.LONG	0
	.LONG	0
SCANF:	.LONG	0
AUXB:	.BLKB	64
AUXD:	.LONG	64
	.ADDRESS AUXB
AUXL:	.WORD	0

	.PSECT	CODE,EXE,NOWRT
; PRINT writes a line from a $FAO control string and its parameters.
	.ENTRY	PRINT,^M<>
	PUSHAL	8(AP)
	PUSHAQ	OUTD
	PUSHAW	OUTLEN
	PUSHL	4(AP)
	CALLS	#4,G^SYS$FAOL
	MOVZWL	OUTLEN,LINED
	PUSHAQ	LINED
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; HANDLER records a condition, and unwinds to the case's caller with it in R0.
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	MOVL	8(AP),R1
	CMPL	4(R0),#SS$_UNWIND
	BEQL	10$
	MOVL	4(R0),SIGCOND
	MOVL	4(R0),12(R1)
	$UNWIND_S
10$:	MOVL	#SS$_RESIGNAL,R0
	RET

; ACTBLOCK leaves the argument block's address in R1: AP, or the one argument
; if the routine was called with one.
	.ENTRY	ACTLOG,^M<R2,R3,R4,R5>
	MOVL	4(AP),R2	; the caller's AP
	MOVL	R2,R3
	CMPL	(R2),#1
	BNEQ	10$
	MOVL	4(R2),R3
10$:	MOVAL	BLOCK,R4
	SUBL3	R4,R3,R5	; 0 when it's the block
	PUSHL	R5
	PUSHL	(R2)
	PUSHL	8(AP)		; the low bit of R0 on entry
	PUSHAB	ACTLOGF
	CALLS	#4,PRINT
	SUBL3	STRBASE,TPA$L_TOKENPTR(R3),R4
	SUBL3	STRBASE,TPA$L_STRINGPTR(R3),R5
	P	<    rest=!UL@!SL tok=!UL@!SL char=!XB num=!XL param=!XL opt=!XL>,-
		TPA$L_STRINGCNT(R3),R5,TPA$L_TOKENCNT(R3),R4,TPA$B_CHAR(R3),TPA$L_NUMBER(R3),TPA$L_PARAM(R3),TPA$L_OPTIONS(R3)
	MOVL	R3,R0
	RET

; ACT logs its call and returns its argument.
	.ENTRY	ACT,^M<R2>
	BICL3	#^XFFFFFFFE,R0,R2
	PUSHL	R2
	PUSHL	AP
	CALLS	#2,ACTLOG
	MOVL	TPA$L_PARAM(R0),R0
	RET

; ACTOFF and ACTON turn the blanks flag off and on.
	.ENTRY	ACTOFF,^M<R2>
	BICL3	#^XFFFFFFFE,R0,R2
	PUSHL	R2
	PUSHL	AP
	CALLS	#2,ACTLOG
	BICL2	#TPA$M_BLANKS,TPA$L_OPTIONS(R0)
	MOVL	#1,R0
	RET
	.ENTRY	ACTON,^M<R2>
	BICL3	#^XFFFFFFFE,R0,R2
	PUSHL	R2
	PUSHL	AP
	CALLS	#2,ACTLOG
	BISL2	#TPA$M_BLANKS,TPA$L_OPTIONS(R0)
	MOVL	#1,R0
	RET

; ACTSKIP takes a character off the string itself.
	.ENTRY	ACTSKIP,^M<R2>
	BICL3	#^XFFFFFFFE,R0,R2
	PUSHL	R2
	PUSHL	AP
	CALLS	#2,ACTLOG
	DECL	TPA$L_STRINGCNT(R0)
	INCL	TPA$L_STRINGPTR(R0)
	MOVL	#1,R0
	RET

; ACTSIG signals through the parser.
	.ENTRY	ACTSIG,^M<>
	PUSHL	#^X1234
	CALLS	#1,G^LIB$SIGNAL
	MOVL	#1,R0
	RET

; TESTQ is the manual's: it refuses the quoting character.
	.ENTRY	TESTQ,^M<R2>
	MOVL	AP,R2
	CMPL	(AP),#1
	BNEQ	5$
	MOVL	4(AP),R2
5$:	MOVL	#1,R0
	CMPB	TPA$B_CHAR(R2),QCHAR
	BNEQ	10$
	CLRL	R0
10$:	RET

; RUN(routine, block, state, keys) makes the call under HANDLER.
	.ENTRY	RUN,^M<>
	MOVAB	HANDLER,(FP)
	PUSHL	16(AP)
	PUSHL	12(AP)
	PUSHL	8(AP)
	CALLS	#3,@4(AP)
	RET

; CASE(name, state, string, options, count, routine): one case.
	.ENTRY	CASE,^M<R2,R3,R4,R5,R6>
	MOVAL	BLOCK,R2
	MOVQ	@12(AP),R3	; length (low word), address
	MOVZWL	R3,R3
	MOVL	R4,STRBASE
	MOVL	20(AP),(R2)
	MOVL	16(AP),TPA$L_OPTIONS(R2)
	MOVL	R3,TPA$L_STRINGCNT(R2)
	MOVL	R4,TPA$L_STRINGPTR(R2)
	MOVL	#^X55555555,R0
	MOVL	R0,TPA$L_TOKENCNT(R2)
	MOVL	R4,TPA$L_TOKENPTR(R2)
	MOVL	R0,TPA$B_CHAR(R2)
	MOVL	R0,TPA$L_NUMBER(R2)
	MOVL	R0,TPA$L_PARAM(R2)
	MOVL	R0,CELL
	MOVL	R0,CELL+4
	CLRL	SIGCOND
	PUSHL	4(AP)
	PUSHAB	CASEF
	CALLS	#2,PRINT
	PUSHAB	KT
	PUSHL	8(AP)
	PUSHL	R2
	PUSHL	24(AP)
	CALLS	#4,RUN
	MOVL	R0,R6
	P	<  R0=!XL signal=!XL>,R6,SIGCOND
	CMPL	TPA$L_TOKENCNT(R2),#^X55555555
	BEQL	10$
	SUBL3	R4,TPA$L_TOKENPTR(R2),R5
	P	<  tok=!UL@!SL "!AF">,TPA$L_TOKENCNT(R2),R5,TPA$L_TOKENCNT(R2),TPA$L_TOKENPTR(R2)
10$:	SUBL3	R4,TPA$L_STRINGPTR(R2),R5
	P	<  rest=!UL@!SL "!AF">,TPA$L_STRINGCNT(R2),R5,TPA$L_STRINGCNT(R2),TPA$L_STRINGPTR(R2)
	P	<  char=!XL num=!XL param=!XL opt=!XL>,TPA$B_CHAR(R2),TPA$L_NUMBER(R2),TPA$L_PARAM(R2),TPA$L_OPTIONS(R2)
	CMPL	CELL+4,#^X55555555
	BEQL	20$
	SUBL3	R4,CELL+4,R5
	P	<  cell=!XL desc @!SL>,CELL,R5
	RET
20$:	P	<  cell=!XL>,CELL
	RET

; MAPIT(state, type name): every character tried on a state, a line of 64
; characters at a time, 1 for one that matched with the whole string consumed.
	.ENTRY	MAPIT,^M<R2,R3,R4,R5>
	PUSHL	8(AP)
	PUSHAB	MAPF
	CALLS	#2,PRINT
	MOVAL	BLOCK,R2
	CLRL	R3
10$:	MOVB	R3,CH
	MOVL	#8,(R2)
	MOVL	#TPA$M_BLANKS,TPA$L_OPTIONS(R2)
	MOVL	#1,TPA$L_STRINGCNT(R2)
	MOVAB	CH,TPA$L_STRINGPTR(R2)
	PUSHAB	KT
	PUSHL	4(AP)
	PUSHL	R2
	PUSHAB	G^LIB$TPARSE
	CALLS	#4,RUN
	MOVB	#^A/0/,R5
	BLBC	R0,20$
	MOVB	#^A/1/,R5
	TSTL	TPA$L_STRINGCNT(R2)
	BEQL	20$
	MOVB	#^A/2/,R5
20$:	BICL3	#^XFFFFFFC0,R3,R4
	MOVB	R5,MAPL[R4]
	CMPL	R4,#63
	BNEQ	30$
	P	<  !AS>,#MAPD
30$:	AOBLSS	#256,R3,10$
	RET

; DUMP(address, length): bytes in hexadecimal, 16 a line.
	.ENTRY	DUMP,^M<R2,R3,R4>
	MOVL	4(AP),R2
	MOVL	8(AP),R3
10$:	TSTL	R3
	BLEQ	30$
	MOVL	#16,R4
	CMPL	R3,R4
	BGEQ	20$
	MOVL	R3,R4
20$:	PUSHL	R2
	PUSHL	R4
	PUSHAB	DUMPF
	CALLS	#3,PRINTHEX
	ADDL2	R4,R2
	SUBL2	R4,R3
	BRB	10$
30$:	RET

; PRINTHEX(control, n, address): n bytes as hexadecimal.
	.ENTRY	PRINTHEX,^M<R2,R3,R4,R5>
	MOVL	8(AP),R2
	MOVL	12(AP),R3
	MOVAB	HEXBUF,R4
	MOVL	R2,R5
10$:	TSTL	R5
	BEQL	20$
	MOVZBL	(R3)+,R0
	EXTZV	#4,#4,R0,R1
	MOVB	HEXC[R1],(R4)+
	BICL3	#^XFFFFFFF0,R0,R1
	MOVB	HEXC[R1],(R4)+
	MOVB	#^A/ /,(R4)+
	DECL	R5
	BRB	10$
20$:	MULL3	#3,R2,R0
	MOVAB	HEXBUF,R1
	PUSHL	R1
	PUSHL	R0
	PUSHL	4(AP)
	CALLS	#3,PRINT
	RET

; SCAN(string): $FILESCAN of a string, every part, and the flags.
	.ENTRY	SCAN,^M<R2,R3,R4,R5>
	MOVL	4(AP),R2
	MOVZWL	(R2),R3
	MOVL	4(R2),R4
	P	<!/scan "!AF">,R3,R4
	$FILESCAN_S SRCSTR=(R2),VALUELST=SCANI,FLDFLAGS=SCANF
	MOVL	R0,R5
	P	<  R0=!XL flags=!XL>,R5,SCANF
	MOVAL	SCANI,R2
10$:	TSTL	(R2)
	BEQL	20$
	MOVZWL	2(R2),R0
	MOVZWL	(R2),R1
	CLRL	R5
	TSTL	4(R2)
	BEQL	15$
	SUBL3	R4,4(R2),R5
15$:	PUSHL	R5
	PUSHL	R1
	PUSHL	R0
	PUSHAB	SCANF2
	CALLS	#4,PRINT
	ADDL2	#8,R2
	BRB	10$
20$:	RET

	.PSECT	TEXT
ACTLOGF:	.ASCID	\  action r0low=!UL argc=!XL block=!SL\
CASEF:	.ASCID	\!/!AS\
MAPF:	.ASCID	\!/map !AS\
DUMPF:	.ASCID	\  !AD\
HEXC:	.ASCII	/0123456789ABCDEF/
SCANF2:	.ASCID	\  item !UL len=!UL at=!UL\
	.PSECT	DATA
HEXBUF:	.BLKB	64

`

func main() {
	dir := filepath.Join("testdata", "tpa80")
	write(filepath.Join(dir, "tpa80.mar"), program())
	write(filepath.Join(dir, "tpa80b.mar"), module())
	write(filepath.Join(dir, "tpa80c.mar"), hardForms)
	write(filepath.Join(dir, "tpa80.com"), procedure)
	write(filepath.Join(dir, "exchange.cmd"), exchange)
	write(filepath.Join(dir, "copyout.cmd"), copyout)
}

func write(path, text string) {
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		panic(err)
	}
}

// program writes the probe.
func program() string {
	var b strings.Builder

	b.WriteString(prologue)
	writeTables(&b)
	writeMain(&b)

	return b.String()
}

// dumpTable is the table whose bytes the program shows: every field and flag
// of a transition, its addresses inside the table so that they don't depend on
// the program's layout.
var dumpTable = []string{
	"$STATE D1", "$TRAN 'A'", "$TRAN 'B',D1", "$TRAN 'C',TPA$_EXIT", "$TRAN 'D',TPA$_FAIL",
	"$STATE", "$TRAN 'KW1'", "$TRAN 'KW2',D1", "$TRAN 'K*'",
	"$STATE", "$TRAN 'KW3'", "$TRAN 'KW1'",
	"$STATE D2", "$TRAN !D1", "$TRAN !D1,D2", "$TRAN !D1,TPA$_EXIT",
	"$STATE", "$TRAN TPA$_ANY,,D1", "$TRAN TPA$_ALPHA,,,^X11223344,D1", "$TRAN TPA$_DIGIT,,,,D1", "$TRAN TPA$_STRING,,,,,^X55667788",
	"$STATE", "$TRAN TPA$_SYMBOL,D2,D1,^X01020304,D2,^X05060708", "$TRAN !D2,D2,D1,^X01020304,D2,^X05060708", "$TRAN 'KW4',D2,D1,^X01020304,D2,^X05060708",
	"$STATE", "$TRAN TPA$_BLANK", "$TRAN TPA$_DECIMAL", "$TRAN TPA$_OCTAL", "$TRAN TPA$_HEX", "$TRAN TPA$_LAMBDA", "$TRAN TPA$_EOS", "$TRAN TPA$_UIC", "$TRAN TPA$_IDENT", "$TRAN TPA$_FILESPEC",
	"$STATE", "$TRAN EIGHT", "$TRAN COMMA", "$TRAN <','>", "$TRAN '/'", "$TRAN '='", "$TRAN TWO",
}

// hexArgument is a hexadecimal argument. A macro call reads ^X... as text
// delimited by X's, and round 1 showed that VMS's $TRAN refuses a bracketed one as
// its last argument (UNTERMARG), so each is a symbol, H_<digits>.
var hexArgument = regexp.MustCompile(`\^X([0-9A-F]+)`)

func bracketed(line string) string {
	return hexArgument.ReplaceAllString(line, "H_$1")
}

// writeHexSymbols defines the symbols bracketed makes.
func writeHexSymbols(b *strings.Builder) {
	seen := map[string]bool{}

	note := func(lines []string) {
		for _, line := range lines {
			for _, m := range hexArgument.FindAllStringSubmatch(line, -1) {
				if !seen[m[1]] {
					seen[m[1]] = true
					fmt.Fprintf(b, "H_%s = ^X%s\n", m[1], m[1])
				}
			}
		}
	}

	note(dumpTable)

	for _, g := range grammars {
		note(g.lines)
	}
}

func writeTables(b *strings.Builder) {
	writeHexSymbols(b)

	fmt.Fprintf(b, "\t$INIT_STATE\tDT,DK\n")

	for _, line := range dumpTable {
		fmt.Fprintf(b, "\t%s\n", bracketed(line))
	}

	b.WriteString("\t$END_STATE\n\n\t$INIT_STATE\tTP,KT\n")

	for _, g := range grammars {
		fmt.Fprintf(b, "\t$STATE\t%s\n", g.name)

		for _, line := range g.lines {
			fmt.Fprintf(b, "\t%s\n", bracketed(line))
		}
	}

	for i, t := range mapTypes {
		fmt.Fprintf(b, "\t$STATE\tMP%d\n\t$TRAN\t%s,TPA$_EXIT\n", i, t)
	}

	// A keyword followed by each character: where a keyword ends.
	b.WriteString("\t$STATE\tMPK\n\t$TRAN\t'AB',TPA$_EXIT\n\t$END_STATE\n\n")

	b.WriteString("\t.PSECT\tDATA\n; Transitions of type bytes no macro makes.\n")

	for _, t := range rawTypes {
		fmt.Fprintf(b, "RAW%d:\t.BYTE\t%d,^X15\n\t.WORD\t-1\n", t, t)
	}

	// A state of two transitions, the first with no "last" bit and a bad type.
	b.WriteString("RAWBAD:\t.BYTE\t250,^X01,^A/A/,^X14\n\t.WORD\t-1\n")
	// A subexpression's type with no pointer, a pointer without the type.
	b.WriteString("RAWSUB:\t.BYTE\t248,^X15\n\t.WORD\t-1\n")
	b.WriteString("RAWPTR:\t.BYTE\t^A/A/,^X1C\n\t.WORD\t0\n\t.WORD\t-1\n")
	b.WriteString("\t.PSECT\tCODE\n\n")
}

// asciid writes a string's descriptor and bytes to the text psect.
func asciid(b *strings.Builder, label, s string) {
	fmt.Fprintf(b, "\t.PSECT\tTEXT\n%s:\t.LONG\t%d\n\t.ADDRESS\t.+4\n", label, len(s))

	if s != "" {
		var bytes []string
		for i := range len(s) {
			bytes = append(bytes, fmt.Sprintf("%d", s[i]))
		}

		for len(bytes) > 0 {
			n := min(len(bytes), 16)
			fmt.Fprintf(b, "\t.BYTE\t%s\n", strings.Join(bytes[:n], ","))
			bytes = bytes[n:]
		}
	}

	b.WriteString("\t.PSECT\tCODE\n")
}

var labels int

func label() string {
	labels++

	return fmt.Sprintf("L%d", labels)
}

// call writes one case.
func call(b *strings.Builder, name, state, text string, options uint32, count int, routine string) {
	nameLabel, textLabel := label(), label()
	asciid(b, nameLabel, name)
	asciid(b, textLabel, text)
	fmt.Fprintf(b, "\tPUSHAB\tG^%s\n\tPUSHL\t#%d\n\tPUSHL\t#^X%X\n\tPUSHAB\t%s\n\tPUSHAB\t%s\n\tPUSHAB\t%s\n\tCALLS\t#6,CASE\n",
		routine, count, options, textLabel, state, nameLabel)
}

// show makes an input printable in a case's name.
func show(text string, options uint32) string {
	s := strings.ReplaceAll(text, "\t", "<TAB>")
	if options != 0 {
		return fmt.Sprintf("<%s> opt %X", s, options)
	}

	return "<" + s + ">"
}

func writeMain(b *strings.Builder) {
	b.WriteString("\t.ENTRY\tGO,^M<>\n")

	section := func(name string) {
		l := label()
		asciid(b, l, "!/##### "+name)
		fmt.Fprintf(b, "\tPUSHAB\t%s\n\tCALLS\t#1,PRINT\n", l)
	}

	section("Definitions")

	for _, name := range symbols {
		l, u := label(), label()
		asciid(b, l, name+"=!XL")
		asciid(b, u, name+" undefined")
		fmt.Fprintf(b, "\t.IF\tDEFINED %s\n\tPUSHL\t#%s\n\tPUSHAB\t%s\n\tCALLS\t#2,PRINT\n\t.IFF\n\tPUSHAB\t%s\n\tCALLS\t#1,PRINT\n\t.ENDC\n", name, name, l, u)
	}

	section("The table's bytes: states, vector, strings")
	b.WriteString("\tSUBL3\t#DT,#TP,R2\n\tPUSHL\tR2\n\tPUSHAB\tDT\n\tCALLS\t#2,DUMP\n")
	b.WriteString("\tSUBL3\t#DK,#KT,R2\n\tPUSHL\tR2\n\tPUSHAB\tDK\n\tCALLS\t#2,DUMP\n")
	b.WriteString("\tCVTWL\tDK,R2\n\tMOVAB\tDK[R2],R3\n\tCVTWL\tKT,R2\n\tMOVAB\tKT[R2],R4\n\tSUBL3\tR3,R4,R2\n\tPUSHL\tR2\n\tPUSHL\tR3\n\tCALLS\t#2,DUMP\n")

	section("Every character")

	for i, t := range mapTypes {
		l := label()
		asciid(b, l, t)
		fmt.Fprintf(b, "\tPUSHAB\t%s\n\tPUSHAB\tMP%d\n\tCALLS\t#2,MAPIT\n", l, i)
	}

	l := label()
	asciid(b, l, "keyword AB, then the character, as AB<c>")
	fmt.Fprintf(b, "\tPUSHAB\t%s\n\tCALLS\t#1,PRINT\n", l)
	writeKeywordMap(b)

	section("LIB$TPARSE")

	for _, g := range grammars {
		for _, i := range g.inputs {
			call(b, g.name+" "+show(i.text, i.options), g.name, i.text, i.options, 8, "LIB$TPARSE")
		}
	}

	section("LIB$TABLE_PARSE")

	for _, g := range grammars {
		for _, i := range g.inputs {
			if g.name == "ABR" && i.options != abbrev {
				continue
			}

			call(b, g.name+" "+show(i.text, i.options), g.name, i.text, i.options, 8, "LIB$TABLE_PARSE")
		}
	}

	section("Type bytes no macro makes")

	for _, t := range rawTypes {
		for _, text := range rawInputs {
			state := fmt.Sprintf("RAW%d", t)
			call(b, state+" "+show(text, 0), state, text, 0, 8, "LIB$TPARSE")
			call(b, state+" table "+show(text, 0), state, text, 0, 8, "LIB$TABLE_PARSE")
		}
	}

	for _, state := range []string{"RAWBAD", "RAWSUB", "RAWPTR"} {
		call(b, state, state, "A", 0, 8, "LIB$TPARSE")
	}

	section("Argument blocks")

	for _, c := range counts {
		name := fmt.Sprintf("count %d", c)
		call(b, name, "CHAR", "A", 0, c, "LIB$TPARSE")
		call(b, name+" table", "CHAR", "A", 0, c, "LIB$TABLE_PARSE")
		call(b, name+" action", "ALT", "A", 0, c, "LIB$TPARSE")
	}

	section("Bad arguments, each under the handler")
	writeBad(b)

	section("$FILESCAN")

	for _, text := range fileScans {
		lb := label()
		asciid(b, lb, text)
		fmt.Fprintf(b, "\tPUSHAB\t%s\n\tCALLS\t#1,SCAN\n", lb)
	}

	b.WriteString("\t$FILESCAN_S SRCSTR=AUXS,VALUELST=SCANI,AUXOUT=AUXD,RETLEN=AUXL\n\tMOVL\tR0,R2\n\tMOVZWL\tAUXL,R3\n\tP\t<aux R0=!XL len=!UL \"!AF\">,R2,R3,R3,#AUXB\n")
	b.WriteString("\t$FILESCAN_S SRCSTR=AUXS,VALUELST=SCANBAD\n\tMOVL\tR0,R2\n\tP\t<bad item R0=!XL>,R2\n")
	b.WriteString("\t$FILESCAN_S SRCSTR=AUXS\n\tMOVL\tR0,R2\n\tP\t<no list R0=!XL>,R2\n")
	b.WriteString("\t.PSECT\tTEXT\nAUXS:\t.ASCID\t/DEV:[DIR]A.B;1/\n\t.PSECT\tCODE\n")

	section("the end of the cases")
	b.WriteString("\tMOVL\t#1,R0\n\tRET\n\t.END\tGO\n")
}

// writeKeywordMap maps AB followed by each character.
func writeKeywordMap(b *strings.Builder) {
	b.WriteString(`	MOVAL	BLOCK,R2
	CLRL	R3
	MOVB	#^A/A/,AB2
	MOVB	#^A/B/,AB2+1
10$:	MOVB	R3,AB2+2
	MOVL	#8,(R2)
	MOVL	#TPA$M_BLANKS,TPA$L_OPTIONS(R2)
	MOVL	#3,TPA$L_STRINGCNT(R2)
	MOVAB	AB2,TPA$L_STRINGPTR(R2)
	PUSHAB	KT
	PUSHAB	MPK
	PUSHL	R2
	PUSHAB	G^LIB$TPARSE
	CALLS	#4,RUN
	MOVB	#^A/0/,R5
	BLBC	R0,20$
	MOVB	#^A/1/,R5
20$:	BICL3	#^XFFFFFFC0,R3,R4
	MOVB	R5,MAPL[R4]
	CMPL	R4,#63
	BNEQ	30$
	P	<  !AS>,#MAPD
30$:	AOBLSS	#256,R3,10$
	.PSECT	DATA
AB2:	.BLKB	3
	.ALIGN	LONG
	.PSECT	CODE
`)
}

// writeBad writes the cases of bad addresses.
func writeBad(b *strings.Builder) {
	b.WriteString(`	MOVAL	BLOCK,R2
	CLRL	SIGCOND
	PUSHAB	KT
	PUSHAB	CHAR
	PUSHL	#0
	PUSHAB	G^LIB$TPARSE
	CALLS	#4,RUN
	MOVL	R0,R3
	P	<block 0 R0=!XL signal=!XL>,R3,SIGCOND
	CLRL	SIGCOND
	MOVL	#8,(R2)
	CLRL	TPA$L_OPTIONS(R2)
	MOVL	#1,TPA$L_STRINGCNT(R2)
	MOVL	#^X3FFFFF00,TPA$L_STRINGPTR(R2)
	PUSHAB	KT
	PUSHAB	CHAR
	PUSHL	R2
	PUSHAB	G^LIB$TPARSE
	CALLS	#4,RUN
	MOVL	R0,R3
	P	<string unmapped R0=!XL signal=!XL>,R3,SIGCOND
	CLRL	SIGCOND
	MOVL	#8,(R2)
	CLRL	TPA$L_OPTIONS(R2)
	MOVL	#3,TPA$L_STRINGCNT(R2)
	MOVAB	BADK,TPA$L_STRINGPTR(R2)
	PUSHL	#0
	PUSHAB	KW
	PUSHL	R2
	PUSHAB	G^LIB$TPARSE
	CALLS	#4,RUN
	MOVL	R0,R3
	P	<keyword table 0 R0=!XL signal=!XL>,R3,SIGCOND
	CLRL	SIGCOND
	MOVL	#8,(R2)
	MOVL	#1,TPA$L_STRINGCNT(R2)
	MOVAB	BADK,TPA$L_STRINGPTR(R2)
	PUSHAB	KT
	PUSHL	#0
	PUSHL	R2
	PUSHAB	G^LIB$TPARSE
	CALLS	#4,RUN
	MOVL	R0,R3
	P	<state table 0 R0=!XL signal=!XL>,R3,SIGCOND
	.PSECT	TEXT
BADK:	.ASCII	/ABC/
	.PSECT	CODE
`)
}

// module writes a second module for the forms a macro call may not hold, and
// the question of whether $INIT_STATE needs $TPADEF. It isn't run: its listing
// and object are what's looked at.
func module() string {
	return `	.TITLE	TPA80B	Phase 80 state table macro forms
; Written by testdata/tpa80/gen.go: do not edit. No $TPADEF: does $INIT_STATE
; define the symbols? The characters a macro's argument may trouble are each a
; transition. It's assembled for its messages and object, and never run.
	$INIT_STATE	BT,BK
	$STATE	B1
	$TRAN	TPA$_ANY,TPA$_EXIT
	$TRAN	'!'
	$TRAN	<';'>
	$TRAN	'%'
	$TRAN	'^'
	$TRAN	'\'
	$TRAN	'"'
	$TRAN	'('
	$TRAN	')'
	$TRAN	'X',tpa$_exit
	$TRAN	'Y',B1
	$TRAN	'lower'
	$STATE
	$STATE	B2
	$TRAN	'A$B_C'
	$TRAN	'A*'
	$TRAN	'AB*'
	$END_STATE
HERE:	.LONG	0
	.END
`
}

// hardForms holds a transition govax's macro can't take, the character that is
// its delimiter, for VMS's answer. (Round 1 had a blank here too, which VMS refuses.)
const hardForms = `	.TITLE	TPA80C	Phase 80 state table macro forms
; Written by testdata/tpa80/gen.go: do not edit. Assembled for its messages and
; object, and never run. govax refuses this transition.
	$INIT_STATE	CT,CK
	$STATE
	$TRAN	'~'
	$END_STATE
	.END
`

const procedure = `$ ! TPA80.COM - the Phase 80 probe of LIB$TPARSE and $FILESCAN.
$ ! Written by testdata/tpa80/gen.go. Run it on the exchange volume, with its
$ ! [000000] as the default directory: @TPA80/OUTPUT=TPA80.LOG
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEFINE/USER_MODE SYS$OUTPUT TPA80.MLG
$ MACRO/LIST TPA80
$ SHOW SYMBOL $STATUS
$ LINK/MAP/FULL TPA80
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE SYS$OUTPUT TPA80.OUT
$ RUN TPA80
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE SYS$OUTPUT TPA80B.MLG
$ MACRO/LIST TPA80B
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE SYS$OUTPUT TPA80C.MLG
$ MACRO/LIST TPA80C
$ SHOW SYMBOL $STATUS
$ ANALYZE/OBJECT/OUTPUT=TPA80B.ANL TPA80B
$ ANALYZE/OBJECT/OUTPUT=TPA80.ANL TPA80
`

const exchange = `! EXCHANGE.CMD - builds the Phase 80 TPARSE probe's exchange volume with govax.
! Written by testdata/tpa80/gen.go. Run from the repository root:
!
!     govax console < testdata/tpa80/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/tpa80-exchange.dsk" /DEVICE=RD51 TPA80
MOUNT/WRITE DUA2 "testdata/disks/tpa80-exchange.dsk"
COPY "testdata/tpa80/tpa80.mar"/HOST DUA2:[000000]TPA80.MAR
COPY "testdata/tpa80/tpa80b.mar"/HOST DUA2:[000000]TPA80B.MAR
COPY "testdata/tpa80/tpa80c.mar"/HOST DUA2:[000000]TPA80C.MAR
COPY "testdata/tpa80/tpa80.com"/HOST DUA2:[000000]TPA80.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
`

const copyout = `! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/tpa80/gen.go. Run from the repository root, after the VAX has
! dismounted the volume:
!
!     govax console < testdata/tpa80/copyout.cmd
!
MOUNT/NOWRITE DUA2 "testdata/disks/tpa80-exchange.dsk"
COPY DUA2:[000000]TPA80.OUT "testdata/tpa80/vax/tpa80-vax.txt"/HOST
COPY DUA2:[000000]TPA80.LOG "testdata/tpa80/vax/tpa80.log"/HOST
COPY DUA2:[000000]TPA80.MLG "testdata/tpa80/vax/tpa80.mlg"/HOST
COPY DUA2:[000000]TPA80B.MLG "testdata/tpa80/vax/tpa80b.mlg"/HOST
COPY DUA2:[000000]TPA80C.MLG "testdata/tpa80/vax/tpa80c.mlg"/HOST
COPY/BINARY DUA2:[000000]TPA80.OBJ "testdata/tpa80/vax/tpa80.obj"/HOST
COPY/BINARY DUA2:[000000]TPA80B.OBJ "testdata/tpa80/vax/tpa80b.obj"/HOST
COPY DUA2:[000000]TPA80.ANL "testdata/tpa80/vax/tpa80.anl"/HOST
COPY DUA2:[000000]TPA80B.ANL "testdata/tpa80/vax/tpa80b.anl"/HOST
COPY DUA2:[000000]TPA80.MAP "testdata/tpa80/vax/tpa80.map"/HOST
DISMOUNT DUA2
`
