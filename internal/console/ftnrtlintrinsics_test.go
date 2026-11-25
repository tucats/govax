package console

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/testproc"
)

// FTNRTL's Fortran 90 intrinsics called from MACRO (docs/planning/PHASE-108 -
// Fortran 90 intrinsic procedures.md, finding 12, subtask 8): every routine the
// phase added, its edges as the compiler never reaches them (empty strings, a
// REPEAT too long, 0, +-TINY, +-HUGE, every overflow, (0, 0) to the power 0,
// the seed's first value and its restoring, a zone defined, changed, and gone),
// each value printed as the longwords it is. The program is made from
// ftnrtlCases, one line of output each; the values are worked from the VAX's
// formats by hand:
//
//	F	1.0 is 00004080, 3.0 00004140, TINY 00000080, HUGE FFFF7FFF
//	D	1.0 is 00004080 00000000
//	G	1.0 is 00004010 00000000
//	H	1.0 is 00004001 00000000 00000000 00000000, 3.0 80004002 ...
//
// (the exponent field, the value's exponent plus the bias, is bits 7 to 14 in F
// and D, 4 to 14 in G, 0 to 14 in H, of the first word; the fraction's
// remaining bits follow, the first word's low bits then each later word).

// ftnrtlCase is one line of the program's output: code that leaves the value
// in R0 (and R1) or in RES, how it is shown, and what it must show.
type ftnrtlCase struct {
	label string
	code  string // MACRO lines, tab-led
	show  string // R0: R0 as a signed integer; F, D: R0 (and R1) in hex;
	// H: RES's 4 longwords; C: RES's 8; T: the text OUT describes; Y: DYN's
	want string
}

// trap is code calling routine with the argument list at args under
// LIB$SIG_TO_RET, so a condition it signals comes back as R0.
func trap(routine, args string) string {
	return "\tPUSHAB\t" + args + "\n\tPUSHAB\tG^" + routine + "\n\tCALLS\t#2,TRAP\n"
}

// call is code calling routine with the arguments given, pushed last first.
func call(routine string, args ...string) string {
	var b strings.Builder

	for i := len(args) - 1; i >= 0; i-- {
		if args[i] == "0" {
			b.WriteString("\tCLRL\t-(SP)\n")
		} else {
			b.WriteString("\tPUSHAB\t" + args[i] + "\n")
		}
	}

	fmt.Fprintf(&b, "\tCALLS\t#%d,G^%s\n", len(args), routine)

	return b.String()
}

// text is code making OUT n characters long, then calling routine with OUT
// and the other arguments.
func text(n int, routine string, args ...string) string {
	return fmt.Sprintf("\tMOVW\t#%d,OUT\n", n) + call(routine, append([]string{"OUT"}, args...)...)
}

// ftnrtlData is the program's operands: strings by descriptor, integers, and
// each format's values as their longwords.
const ftnrtlData = `S1:	.ASCID	/  ab c /
S3B:	.ASCID	/   /
SNUL:	.LONG	^X010E0000,0		; an empty class S string
SAB:	.ASCID	/ab/
SXYZ:	.ASCID	/xyz/
SFORT:	.ASCID	/fortran/
SRT:	.ASCID	/rt/
SAAX:	.ASCID	/aaxbxaa/
SA:	.ASCID	/a/
SAAA:	.ASCID	/aaa/
SABC:	.ASCID	/abc/
SABCABC: .ASCID	/abcabc/
SBC:	.ASCID	/bc/
STZ:	.ASCID	/SYS$TIMEZONE_DIFFERENTIAL/
SMINUS:	.ASCID	/-18000/
SPLUS:	.ASCID	/+3600/
SJUNK:	.ASCID	/east/
TRUE:	.LONG	1
FALSE:	.LONG	0
N0:	.LONG	0
N1:	.LONG	1
N2:	.LONG	2
N3:	.LONG	3
N5:	.LONG	5
N200:	.LONG	200
NM1:	.LONG	-1
NM200:	.LONG	-200
N1000:	.LONG	1000
N40000:	.LONG	40000
B5:	.BYTE	^XB5			; MVBITS: FROM, FROMPOS, LEN, TO, TOPOS
B2:	.BYTE	2
B3:	.BYTE	3
B4:	.BYTE	4
BTO:	.BYTE	0
	.ALIGN	LONG
F0:	.LONG	0
F1:	.LONG	^X00004080
FM1:	.LONG	^X0000C080
F3:	.LONG	^X00004140
FM3:	.LONG	^X0000C140
FTINY:	.LONG	^X00000080
FMTINY:	.LONG	^X00008080
FHUGE:	.LONG	^XFFFF7FFF
FMHUGE:	.LONG	^XFFFFFFFF
D1:	.LONG	^X00004080,0
D3:	.LONG	^X00004140,0
G1:	.LONG	^X00004010,0
G3:	.LONG	^X00004028,0
H0:	.LONG	0,0,0,0
H1:	.LONG	^X00004001,0,0,0
H3:	.LONG	^X80004002,0,0,0
HHUGE:	.LONG	^XFFFF7FFF,-1,-1,-1
C34:	.LONG	^X80004002,0,0,0,^X00004003,0,0,0	; (3, 4)
C00:	.LONG	0,0,0,0,0,0,0,0
CM40:	.LONG	^X0000C003,0,0,0,0,0,0,0		; (-4, 0)
C10:	.LONG	^X00004001,0,0,0,0,0,0,0		; (1, 0)
C01:	.LONG	0,0,0,0,^X00004001,0,0,0		; (0, 1)
C20:	.LONG	^X00004002,0,0,0,0,0,0,0		; (2, 0)
VALS:	.BLKL	8
WVALS:	.BLKW	8
RATE:	.LONG	0
MAXC:	.LONG	0
COUNT:	.LONG	0
WRATE:	.WORD	0
WMAX:	.WORD	0
SIZE:	.LONG	0
GET:	.LONG	0
PUT5:	.LONG	5
PUTW:	.WORD	3,7			; low word first
X1:	.QUAD	0
X2:	.QUAD	0
XH:	.OCTA	0
MYSEED:	.LONG	1234567
A_REPM:	.LONG	3			; argument lists for TRAP
	.ADDRESS OUT,SAB,NM1
A_REPL:	.LONG	3
	.ADDRESS DYN,SAB,N40000
A_POW00: .LONG	3
	.ADDRESS RES,C00,N0
A_LOG00: .LONG	3
	.ADDRESS RES,C00,C10
A_SCHUGE: .LONG	2
	.ADDRESS FHUGE,N1
A_SCHH:	.LONG	3
	.ADDRESS RES,HHUGE,N1
A_SET200: .LONG	2
	.ADDRESS F3,N200
A_NHUGE: .LONG	2
	.ADDRESS FHUGE,F1
A_NMHUGE: .LONG	2
	.ADDRESS FMHUGE,FM1
`

// ftnrtlCases are the program's lines, in order (the random numbers and the
// zone depend on what earlier lines did).
var ftnrtlCases = []ftnrtlCase{
	// ADJUSTL, ADJUSTR, REPEAT: a result of the caller's length, or a
	// dynamic one REPEAT gives storage; the count checked first.
	{"ADJUSTL", text(7, "FTN$ADJUSTL", "S1"), "T", "[ab c   ]"},
	{"ADJUSTR", text(7, "FTN$ADJUSTR", "S1"), "T", "[   ab c]"},
	{"ADJUSTL blanks", text(3, "FTN$ADJUSTL", "S3B"), "T", "[   ]"},
	{"ADJUSTR blanks", text(3, "FTN$ADJUSTR", "S3B"), "T", "[   ]"},
	{"ADJUSTL empty", text(0, "FTN$ADJUSTL", "SNUL"), "T", "[]"},
	{"ADJUSTR empty", text(0, "FTN$ADJUSTR", "SNUL"), "T", "[]"},
	{"REPEAT", text(6, "FTN$REPEAT", "SAB", "N3"), "T", "[ababab]"},
	{"REPEAT 0", text(0, "FTN$REPEAT", "SAB", "N0"), "T", "[]"},
	{"REPEAT dynamic", call("FTN$REPEAT", "DYN", "SXYZ", "N2"), "Y", "[xyzxyz]"},
	{"REPEAT empty", call("FTN$REPEAT", "DYN", "SNUL", "N1000"), "Y", "[]"},
	{"REPEAT -1", trap("FTN$REPEAT", "A_REPM"), "F", " 0018898C"},
	{"REPEAT too long", trap("FTN$REPEAT", "A_REPL"), "F", " 0018898C"},

	// SCAN, VERIFY, INDEX with BACK: positions from 1, or 0.
	{"SCAN", call("FTN$SCAN", "SFORT", "SRT", "FALSE"), "R0", "3"},
	{"SCAN back", call("FTN$SCAN", "SFORT", "SRT", "TRUE"), "R0", "5"},
	{"SCAN empty string", call("FTN$SCAN", "SNUL", "SRT", "TRUE"), "R0", "0"},
	{"SCAN empty set", call("FTN$SCAN", "SABC", "SNUL", "FALSE"), "R0", "0"},
	{"VERIFY", call("FTN$VERIFY", "SAAX", "SA", "FALSE"), "R0", "3"},
	{"VERIFY back", call("FTN$VERIFY", "SAAX", "SA", "TRUE"), "R0", "5"},
	{"VERIFY all in set", call("FTN$VERIFY", "SAAA", "SA", "TRUE"), "R0", "0"},
	{"VERIFY empty set", call("FTN$VERIFY", "SABC", "SNUL", "FALSE"), "R0", "1"},
	{"VERIFY empty set back", call("FTN$VERIFY", "SABC", "SNUL", "TRUE"), "R0", "3"},
	{"INDEX back", call("FTN$INDEX_BACK", "SABCABC", "SBC", "TRUE"), "R0", "5"},
	{"INDEX not back", call("FTN$INDEX_BACK", "SABCABC", "SBC", "FALSE"), "R0", "2"},
	{"INDEX back empty", call("FTN$INDEX_BACK", "SABC", "SNUL", "TRUE"), "R0", "4"},
	{"INDEX back too long", call("FTN$INDEX_BACK", "SAB", "SABC", "TRUE"), "R0", "0"},
	{"INDEX back both empty", call("FTN$INDEX_BACK", "SNUL", "SNUL", "TRUE"), "R0", "1"},

	// MVBITS of bytes: bits 2 to 4 of ^XB5 (101) to bits 4 to 6.
	{"BMVBITS", call("FTN$BMVBITS", "B5", "B2", "B3", "BTO", "B4") + "\tMOVZBL\tBTO,R0\n", "R0", "80"},

	// COMPLEX(16)'s functions and powers, their values through the hidden
	// argument.
	{"CHABS (3,4)", call("FTN$CHABS", "RES", "C34"), "H", " 40004003 00000000 00000000 00000000"},
	{"CHABS (0,0)", call("FTN$CHABS", "RES", "C00"), "H", " 00000000 00000000 00000000 00000000"},
	{"CHSQRT (-4,0)", call("FTN$CHSQRT", "RES", "CM40"), "C", " 00000000 00000000 00000000 00000000 00004002 00000000 00000000 00000000"},
	{"CHEXP (0,0)", call("FTN$CHEXP", "RES", "C00"), "C", " 00004001 00000000 00000000 00000000 00000000 00000000 00000000 00000000"},
	{"CHLOG (1,0)", call("FTN$CHLOG", "RES", "C10"), "C", " 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000"},
	{"CHSIN (0,0)", call("FTN$CHSIN", "RES", "C00"), "C", " 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000"},
	{"CHCOS (0,0)", call("FTN$CHCOS", "RES", "C00"), "C", " 00004001 00000000 00000000 00000000 00000000 00000000 00000000 00000000"},
	{"CHPOWJ (0,1)**2", call("FTN$CHPOWJ", "RES", "C01", "N2"), "C", " 0000C001 00000000 00000000 00000000 00000000 00000000 00000000 00000000"},
	{"CHPOWJ (0,1)**-1", call("FTN$CHPOWJ", "RES", "C01", "NM1"), "C", " 00000000 00000000 00000000 00000000 0000C001 00000000 00000000 00000000"},
	{"CHPOWJ (3,4)**0", call("FTN$CHPOWJ", "RES", "C34", "N0"), "C", " 00004001 00000000 00000000 00000000 00000000 00000000 00000000 00000000"},
	{"CHPOWJ (0,0)**0", trap("FTN$CHPOWJ", "A_POW00"), "F", " 00168294"},
	{"CHPOW (1,0)**(2,0)", call("FTN$CHPOW", "RES", "C10", "C20"), "C", " 00004001 00000000 00000000 00000000 00000000 00000000 00000000 00000000"},
	{"CHPOW (0,0)**(1,0)", trap("FTN$CHPOW", "A_LOG00"), "F", " 0016829C"},

	// The model: SCALE and SET_EXPONENT, SPACING, RRSPACING, NEAREST.
	{"SCALE_F 1,3", call("FTN$SCALE_F", "F1", "N3"), "F", " 00004200"},
	{"SCALE_F 0,5", call("FTN$SCALE_F", "F0", "N5"), "F", " 00000000"},
	{"SCALE_F TINY,-1", call("FTN$SCALE_F", "FTINY", "NM1"), "F", " 00000000"},
	{"SCALE_F HUGE,1", trap("FTN$SCALE_F", "A_SCHUGE"), "F", " 000004B4"},
	{"SCALE_D 1,-1", call("FTN$SCALE_D", "D1", "NM1"), "D", " 00004000 00000000"},
	{"SCALE_G 1,3", call("FTN$SCALE_G", "G1", "N3"), "D", " 00004040 00000000"},
	{"SCALE_H 1,3", call("FTN$SCALE_H", "RES", "H1", "N3"), "H", " 00004004 00000000 00000000 00000000"},
	{"SCALE_H HUGE,1", trap("FTN$SCALE_H", "A_SCHH"), "F", " 000004B4"},
	{"SET_EXPONENT_F 3,1", call("FTN$SET_EXPONENT_F", "F3", "N1"), "F", " 000040C0"},
	{"SET_EXPONENT_F 3,-200", call("FTN$SET_EXPONENT_F", "F3", "NM200"), "F", " 00000000"},
	{"SET_EXPONENT_F 3,200", trap("FTN$SET_EXPONENT_F", "A_SET200"), "F", " 000004B4"},
	{"SET_EXPONENT_D 3,1", call("FTN$SET_EXPONENT_D", "D3", "N1"), "D", " 000040C0 00000000"},
	{"SET_EXPONENT_G 3,1", call("FTN$SET_EXPONENT_G", "G3", "N1"), "D", " 00004018 00000000"},
	{"SET_EXPONENT_H 3,1", call("FTN$SET_EXPONENT_H", "RES", "H3", "N1"), "H", " 80004001 00000000 00000000 00000000"},
	{"SPACING_F 1", call("FTN$SPACING_F", "F1"), "F", " 00003500"},
	{"SPACING_F -3", call("FTN$SPACING_F", "FM3"), "F", " 00003580"},
	{"SPACING_F 0", call("FTN$SPACING_F", "F0"), "F", " 00000080"},
	{"SPACING_F TINY", call("FTN$SPACING_F", "FTINY"), "F", " 00000080"},
	{"SPACING_D 1", call("FTN$SPACING_D", "D1"), "D", " 00002500 00000000"},
	{"SPACING_G 1", call("FTN$SPACING_G", "G1"), "D", " 00003CD0 00000000"},
	{"SPACING_H 1", call("FTN$SPACING_H", "RES", "H1"), "H", " 00003F91 00000000 00000000 00000000"},
	{"RRSPACING_F 1", call("FTN$RRSPACING_F", "F1"), "F", " 00004C00"},
	{"RRSPACING_F -3", call("FTN$RRSPACING_F", "FM3"), "F", " 00004C40"},
	{"RRSPACING_F 0", call("FTN$RRSPACING_F", "F0"), "F", " 00000000"},
	{"RRSPACING_D 1", call("FTN$RRSPACING_D", "D1"), "D", " 00005C00 00000000"},
	{"RRSPACING_G 1", call("FTN$RRSPACING_G", "G1"), "D", " 00004350 00000000"},
	{"RRSPACING_H 1", call("FTN$RRSPACING_H", "RES", "H1"), "H", " 00004071 00000000 00000000 00000000"},
	{"NEAREST_F 1,+", call("FTN$NEAREST_F", "F1", "F1"), "F", " 00014080"},
	{"NEAREST_F 1,-", call("FTN$NEAREST_F", "F1", "FM1"), "F", " FFFF407F"},
	{"NEAREST_F 0,+", call("FTN$NEAREST_F", "F0", "F1"), "F", " 00000080"},
	{"NEAREST_F 0,-", call("FTN$NEAREST_F", "F0", "FM1"), "F", " 00008080"},
	{"NEAREST_F TINY,-", call("FTN$NEAREST_F", "FTINY", "FM1"), "F", " 00000000"},
	{"NEAREST_F -TINY,+", call("FTN$NEAREST_F", "FMTINY", "F1"), "F", " 00000000"},
	{"NEAREST_F HUGE,-", call("FTN$NEAREST_F", "FHUGE", "FM1"), "F", " FFFE7FFF"},
	{"NEAREST_F HUGE,+", trap("FTN$NEAREST_F", "A_NHUGE"), "F", " 000004B4"},
	{"NEAREST_F -HUGE,-", trap("FTN$NEAREST_F", "A_NMHUGE"), "F", " 000004B4"},
	{"NEAREST_D 1,+", call("FTN$NEAREST_D", "D1", "F1"), "D", " 00004080 00010000"},
	{"NEAREST_D 1,-", call("FTN$NEAREST_D", "D1", "FM1"), "D", " FFFF407F FFFFFFFF"},
	{"NEAREST_G 1,+", call("FTN$NEAREST_G", "G1", "F1"), "D", " 00004010 00010000"},
	{"NEAREST_H 1,+", call("FTN$NEAREST_H", "RES", "H1", "F1"), "H", " 00004001 00000000 00000000 00010000"},
	{"NEAREST_H 0,-", call("FTN$NEAREST_H", "RES", "H0", "FM1"), "H", " 00008001 00000000 00000000 00000000"},

	// SYSTEM_CLOCK: the rate and maximum of each kind, a count not negative.
	{"SYSTEM_CLOCK rate", call("FTN$SYSTEM_CLOCK", "0", "RATE", "MAXC") + "\tMOVL\tRATE,R0\n", "R0", "100"},
	{"SYSTEM_CLOCK max", "\tMOVL\tMAXC,R0\n", "R0", "2147483647"},
	{"SYSTEM_CLOCK_W rate", call("FTN$SYSTEM_CLOCK_W", "0", "WRATE", "WMAX") + "\tCVTWL\tWRATE,R0\n", "R0", "100"},
	{"SYSTEM_CLOCK_W max", "\tCVTWL\tWMAX,R0\n", "R0", "32767"},
	{"SYSTEM_CLOCK count", "\tMNEGL\t#1,COUNT\n" + call("FTN$SYSTEM_CLOCK", "COUNT", "0", "0") +
		"\tCLRL\tR0\n\tTSTL\tCOUNT\n\tBLSS\t1$\n\tINCL\tR0\n1$:\n", "R0", "1"},

	// RANDOM_SEED and RANDOM_NUMBER: the seed's size, its first value, a draw
	// MTH$RANDOM's own, a sequence repeated after PUT, the first seed restored.
	{"RANDOM_SEED size", call("FTN$RANDOM_SEED", "SIZE", "0", "0") + "\tMOVL\tSIZE,R0\n", "R0", "1"},
	{"RANDOM_SEED_W size", call("FTN$RANDOM_SEED_W", "SIZE", "0", "0") + "\tMOVL\tSIZE,R0\n", "R0", "2"},
	{"RANDOM_SEED first", call("FTN$RANDOM_SEED", "0", "0", "GET") + "\tMOVL\tGET,R0\n", "R0", "1234567"},
	{"RANDOM_F is MTH$RANDOM", call("FTN$RANDOM_F", "X1") + call("MTH$RANDOM", "MYSEED") +
		"\tCLRL\tR1\n\tCMPL\tR0,X1\n\tBNEQ\t1$\n\tINCL\tR1\n1$:\tMOVL\tR1,R0\n", "R0", "1"},
	{"RANDOM_SEED after one", call("FTN$RANDOM_SEED", "0", "0", "GET") + "\tMOVL\tGET,R0\n", "R0", "-629037796"},
	{"RANDOM_D repeats", call("FTN$RANDOM_SEED", "0", "PUT5", "0") + call("FTN$RANDOM_D", "X1") +
		call("FTN$RANDOM_SEED", "0", "PUT5", "0") + call("FTN$RANDOM_D", "X2") +
		"\tCLRL\tR0\n\tCMPD\tX1,X2\n\tBNEQ\t1$\n\tINCL\tR0\n1$:\n", "R0", "1"},
	{"RANDOM_SEED after three", call("FTN$RANDOM_SEED", "0", "0", "GET") + "\tMOVL\tGET,R0\n", "R0", "1599604512"},
	{"RANDOM_G in [0,1)", call("FTN$RANDOM_G", "X1") +
		"\tCLRL\tR0\n\tTSTG\tX1\n\tBLSS\t1$\n\tCMPG\tX1,#1.0\n\tBGEQ\t1$\n\tINCL\tR0\n1$:\n", "R0", "1"},
	{"RANDOM_H in [0,1)", call("FTN$RANDOM_H", "XH") +
		"\tCLRL\tR0\n\tTSTH\tXH\n\tBLSS\t1$\n\tCMPH\tXH,#1.0\n\tBGEQ\t1$\n\tINCL\tR0\n1$:\n", "R0", "1"},
	{"RANDOM_SEED_W put", call("FTN$RANDOM_SEED_W", "0", "PUTW", "0") + call("FTN$RANDOM_SEED", "0", "0", "GET") +
		"\tMOVL\tGET,R0\n", "R0", "458755"},
	{"RANDOM_SEED () restores", call("FTN$RANDOM_SEED", "0", "0", "0") + call("FTN$RANDOM_SEED", "0", "0", "GET") +
		"\tMOVL\tGET,R0\n", "R0", "1234567"},
	{"RANDOM_SEED none restores", call("FTN$RANDOM_SEED_W", "0", "PUTW", "0") + "\tCALLS\t#0,G^FTN$RANDOM_SEED\n" +
		call("FTN$RANDOM_SEED", "0", "0", "GET") + "\tMOVL\tGET,R0\n", "R0", "1234567"},

	// DATE_AND_TIME's zone: SYS$TIMEZONE_DIFFERENTIAL defined, changed, and
	// nonsense in the process table, then deleted there, so the system
	// table's, the host's offset, is the one translated; ZONE as long as it is, cut short, and longer; the
	// minutes in VALUES(4) of each kind; DATE empty.
	{"zone -18000", call("LIB$SET_LOGICAL", "STZ", "SMINUS") + "\tMOVW\t#5,OUT\n" + call("FTN$DATE_AND_TIME", "0", "0", "OUT", "VALS"), "T", "[-0500]"},
	{"VALUES(4)", "\tMOVL\tVALS+12,R0\n", "R0", "-300"},
	{"zone short", "\tMOVW\t#3,OUT\n" + call("FTN$DATE_AND_TIME", "0", "0", "OUT", "0"), "T", "[-05]"},
	{"zone long", "\tMOVW\t#8,OUT\n" + call("FTN$DATE_AND_TIME", "0", "0", "OUT", "0"), "T", "[-0500   ]"},
	{"DATE empty", "\tMOVW\t#0,OUT\n" + call("FTN$DATE_AND_TIME", "OUT", "0", "0", "0"), "T", "[]"},
	{"zone +3600", call("LIB$SET_LOGICAL", "STZ", "SPLUS") + "\tMOVW\t#5,OUT\n" +
		call("FTN$DATE_AND_TIME_W", "0", "0", "OUT", "WVALS"), "T", "[+0100]"},
	{"VALUES_W(4)", "\tCVTWL\tWVALS+6,R0\n", "R0", "60"},
	{"zone not a number", call("LIB$SET_LOGICAL", "STZ", "SJUNK") + "\tMOVW\t#5,OUT\n" +
		call("FTN$DATE_AND_TIME", "0", "0", "OUT", "VALS"), "T", "[     ]"},
	{"VALUES(4) unknown", "\tMOVL\tVALS+12,R0\n", "R0", "-2147483647"},
	{"zone the host's", call("LIB$DELETE_LOGICAL", "STZ") + "\tMOVW\t#5,OUT\n" +
		call("FTN$DATE_AND_TIME_W", "0", "0", "OUT", "WVALS"), "T", "[%ZONE%]"},
	{"VALUES_W(4) the host's", "\tCVTWL\tWVALS+6,R0\n", "R0", "%MINUTES%"},
}

// ftnrtlProgram is the MACRO program of ftnrtlCases: each case's code, then a
// call of the routine that shows its value.
func ftnrtlProgram() string {
	var data, code strings.Builder

	for i, c := range ftnrtlCases {
		fmt.Fprintf(&data, "L%d:\t.ASCID\t\"%s\"\n", i, c.label)

		// A label of its own starts each case's block of local labels.
		fmt.Fprintf(&code, "CASE%d:\n", i)
		code.WriteString(c.code)

		switch c.show {
		case "R0":
			fmt.Fprintf(&code, "\tPUSHL\tR0\n\tPUSHAB\tL%d\n\tCALLS\t#2,SHOWN\n", i)
		case "F":
			fmt.Fprintf(&code, "\tMOVL\tR0,RES\n\tPUSHL\t#1\n\tPUSHAB\tRES\n\tPUSHAB\tL%d\n\tCALLS\t#3,SHOWX\n", i)
		case "D":
			fmt.Fprintf(&code, "\tMOVQ\tR0,RES\n\tPUSHL\t#2\n\tPUSHAB\tRES\n\tPUSHAB\tL%d\n\tCALLS\t#3,SHOWX\n", i)
		case "H":
			fmt.Fprintf(&code, "\tPUSHL\t#4\n\tPUSHAB\tRES\n\tPUSHAB\tL%d\n\tCALLS\t#3,SHOWX\n", i)
		case "C":
			fmt.Fprintf(&code, "\tPUSHL\t#8\n\tPUSHAB\tRES\n\tPUSHAB\tL%d\n\tCALLS\t#3,SHOWX\n", i)
		case "T":
			fmt.Fprintf(&code, "\tPUSHAB\tOUT\n\tPUSHAB\tL%d\n\tCALLS\t#2,SHOWT\n", i)
		case "Y":
			fmt.Fprintf(&code, "\tPUSHAB\tDYN\n\tPUSHAB\tL%d\n\tCALLS\t#2,SHOWT\n", i)
		default:
			panic("no show " + c.show)
		}
	}

	return `	.TITLE	RTL108
	.PSECT	DATA,NOEXE,WRT,LONG
OUT:	.WORD	0			; a class S result of a case's length
	.BYTE	14,1
	.ADDRESS OUTBUF
OUTBUF:	.BLKB	64
DYN:	.WORD	0			; a class D result, empty
	.BYTE	14,2
	.LONG	0
RES:	.BLKL	8
PRM:	.BLKL	10			; $FAOL's parameters
LINE:	.WORD	132
	.BYTE	14,1
	.ADDRESS LINEBUF
LINEBUF: .BLKB	132
CTLX:	.ASCID	/!AS:!#(9XL)/
CTLN:	.ASCID	/!AS: !SL/
CTLT:	.ASCID	/!AS: [!AS]/
` + ftnrtlData + data.String() + `
	.PSECT	CODE,EXE,NOWRT,LONG
	.ENTRY	RTL108,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
` + code.String() + `	MOVL	#1,R0
	RET

; SHOWX(label, address, n): the label and n longwords in hex.
	.ENTRY	SHOWX,^M<R2,R3,R4,R5>
	MOVL	4(AP),PRM
	MOVL	12(AP),PRM+4
	MULL3	#4,12(AP),R2
	MOVC3	R2,@8(AP),PRM+8
	MOVAB	CTLX,R0
	BSBB	OUTPUT
	RET

; SHOWN(label, value): the label and a signed integer.
	.ENTRY	SHOWN,^M<>
	MOVQ	4(AP),PRM
	MOVAB	CTLN,R0
	BSBB	OUTPUT
	RET

; SHOWT(label, descriptor): the label and the text in brackets.
	.ENTRY	SHOWT,^M<>
	MOVQ	4(AP),PRM
	MOVAB	CTLT,R0
	BSBB	OUTPUT
	RET

; OUTPUT: the control string at R0 with PRM, written as a line.
OUTPUT:	MOVW	#132,LINE
	$FAOL_S	CTRSTR=(R0),OUTLEN=LINE,OUTBUF=LINE,PRMLST=PRM
	PUSHAB	LINE
	CALLS	#1,G^LIB$PUT_OUTPUT
	RSB

; TRAP(routine, arguments): the routine called with the argument list, its
; condition, if it signals one, coming back in R0.
	.ENTRY	TRAP,^M<>
	MOVAB	G^LIB$SIG_TO_RET,(FP)
	CALLG	@8(AP),@4(AP)
	RET
	.END	RTL108
`
}

// TestFTNRTLIntrinsics runs the program in both paging modes and holds each
// line to its case.
func TestFTNRTLIntrinsics(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	var want strings.Builder

	for _, c := range ftnrtlCases {
		sep := ": "
		if c.show != "R0" && c.show != "T" && c.show != "Y" {
			sep = ":"
		}

		want.WriteString(c.label + sep + c.want + "\n")
	}

	// The system table's zone is the host's offset, as govax defined it when
	// the console started.
	_, offset := time.Now().Zone()
	minutes := offset / 60
	sign := "+"

	if minutes < 0 {
		sign = "-"
	}

	zone := fmt.Sprintf("%s%02d%02d", sign, abs(minutes)/60, abs(minutes)%60)
	want.WriteString("  $STATUS == \"%X00000001\"\n")

	wanted := strings.NewReplacer("%ZONE%", zone, "%MINUTES%", strconv.Itoa(minutes)).Replace(want.String())

	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]RTL108.MAR"}, rms.TextRecords, splitSource(ftnrtlProgram())); err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, "MACRO RTL108")
		shareFindRun(t, d, "LINK RTL108")

		if out := buf.String(); out != "" {
			t.Fatalf("MACRO and LINK said:\n%s", out)
		}

		got := shareRun(t, d, buf, "RTL108")
		if got == wanted {
			return
		}

		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(wanted, "\n")

		for i := range max(len(gotLines), len(wantLines)) {
			g, w := "", ""
			if i < len(gotLines) {
				g = gotLines[i]
			}

			if i < len(wantLines) {
				w = wantLines[i]
			}

			if g != w {
				t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
			}
		}
	})
}

// abs is n's magnitude.
func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
