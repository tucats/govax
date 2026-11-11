package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// What an indexed file's statements pass FTNRTL (docs/planning/PHASE-88 - keyed index
// files.md, subtask 11). The compiler comes before the run-time library here: a
// program is linked with a MACRO module defining the routines it calls, each of
// which prints what it was given, so its arguments are held whatever FTNRTL
// does with them, with the block's bounds computed when the program runs, and
// in both of the optimizer's modes, whose passes must leave the block's stores.

const ftnKeyedProgram = `      PROGRAM KEYS
      CHARACTER*6 S
      BYTE B(3)
      INTEGER*2 K2
      DATA B /1HX,1HY,1HZ/, S /'ABCDEF'/, K2 /-5/
      N = 3
      OPEN (UNIT=2, FILE='K.DAT', ORGANIZATION='INDEXED',
     1      ACCESS='KEYED', RECL=40, FORM='FORMATTED',
     2      KEY=(1:10, 11:14:INTEGER:DESCENDING, N:N+5,
     3           2*N:2*N+1:INTEGER:ASCENDING))
      READ (2, KEY='AB')
      READ (2, KEYEQ=S, KEYID=2)
      READ (2, KEYGE=K2)
      READ (2, KEYGT=N+1, KEYID=N)
      READ (2, KEYLE=B)
      READ (2, '(F5.1)', KEYLT=7, ERR=10)
      READ (2, KEYNXT=S(2:3), IOSTAT=I)
      READ (2, KEYNXTNE=S)
   10 END
`

// ftnKeyedStub is the routines the program calls, each printing its arguments:
// FTN$OPEN its key block, found by the keyword's code; FTN$READ_KF and _KU the
// key, KEYID='s number, and the match.
const ftnKeyedStub = `	.TITLE	KEYSTUB
	.PSECT	KS_DATA,WRT,NOEXE,LONG
BUF:	.BLKB	132
BUFD:	.LONG	132
	.ADDRESS BUF
OUTD:	.LONG	0
	.ADDRESS BUF
FKEYS:	.ASCID	/OPEN !UL key!%S/
FKEY:	.ASCID	/  !UL:!UL type !UL/
FTEXT:	.ASCID	/READ !AF match !UL id !SL/
FINT:	.ASCID	/READ !SL match !UL id !SL/
	.PSECT	KS_CODE,NOWRT,EXE,LONG
	.ENTRY	PRINT,^M<>
	PUSHAB	8(AP)
	PUSHAQ	BUFD
	PUSHAW	OUTD
	PUSHL	4(AP)
	CALLS	#4,G^SYS$FAOL
	PUSHAQ	OUTD
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.ENTRY	FTN$OPEN,^M<R2,R3,R4>
	MOVL	#3,R2
10$:	MOVZBL	(AP),R0
	CMPL	R2,R0
	BGEQ	90$
	MOVL	(AP)[R2],R0
	CMPB	R0,#23
	BEQL	20$
	ADDL2	#2,R2
	BRB	10$
20$:	MOVL	4(AP)[R2],R3
	MOVZWL	2(R3),R4		; three longwords per key
	DIVL2	#3,R4
	ADDL2	#4,R3
	PUSHL	R4
	PUSHAQ	FKEYS
	CALLS	#2,PRINT
30$:	SOBGEQ	R4,40$
	BRB	90$
40$:	PUSHL	(R3)
	PUSHL	8(R3)
	PUSHL	4(R3)
	PUSHAQ	FKEY
	CALLS	#4,PRINT
	ADDL2	#12,R3
	BRB	30$
90$:	CLRL	R0
	RET
	.ENTRY	FTN$READ_KU,^M<R2,R3,R4,R5>
	MOVAL	12(AP),R2
	BRB	READ
	.ENTRY	FTN$READ_KF,^M<R2,R3,R4,R5>
	MOVAL	16(AP),R2
READ:
	MOVL	(R2),R3			; the key's descriptor
	PUSHL	4(R2)
	PUSHL	8(R2)
	CMPB	2(R3),#14		; text
	BEQL	30$
	CMPB	3(R3),#4		; class A: BYTE array
	BEQL	40$
	CMPB	2(R3),#7		; a word
	BNEQ	20$
	CVTWL	@4(R3),-(SP)
	BRB	25$
20$:	PUSHL	@4(R3)
25$:	PUSHAQ	FINT
	CALLS	#4,PRINT
	BRB	90$
30$:	MOVZWL	(R3),R5
	BRB	50$
40$:	MOVL	12(R3),R5
50$:	PUSHL	4(R3)
	PUSHL	R5
	PUSHAQ	FTEXT
	CALLS	#5,PRINT
90$:	CLRL	R0
	RET
	.ENTRY	FTN$IO_END,^M<>
	CLRL	R0
	RET
	.END
`

var ftnKeyedRun = []string{
	"OPEN 4 keys",
	"  1:10 type 14",
	"  11:14 type 520",
	"  3:8 type 14",
	"  6:7 type 8",
	"READ AB match 0 id -1",
	"READ ABCDEF match 0 id 2",
	"READ -5 match 1 id -1",
	"READ 4 match 2 id 3",
	"READ XYZ match 5 id -1",
	"READ 7 match 6 id -1",
	"READ BC match 3 id -1",
	"READ ABCDEF match 4 id -1",
}

func TestFortranKeyedArguments(t *testing.T) {
	for _, mode := range optimizerModes {
		t.Run(mode.name, func(t *testing.T) {
			d, c, buf := shareFindConsole(t, false)

			for name, text := range map[string]string{"KEYS.FOR": ftnKeyedProgram, "KEYSTUB.MAR": ftnKeyedStub} {
				if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
					t.Fatal(err)
				}
			}

			shareFindRun(t, d, "FORTRAN"+mode.qualifier+" KEYS")
			shareFindRun(t, d, "MACRO KEYS")
			shareFindRun(t, d, "MACRO KEYSTUB")
			shareFindRun(t, d, "LINK KEYS,KEYSTUB")

			if out := buf.String(); out != "" {
				t.Fatalf("FORTRAN, MACRO, and LINK said:\n%s", out)
			}

			var got []string
			for _, line := range strings.Split(strings.TrimSuffix(shareRun(t, d, buf, "KEYS"), "\n"), "\n") {
				if !strings.Contains(line, "$STATUS") {
					got = append(got, strings.TrimRight(line, " "))
				}
			}

			if strings.Join(got, "\n") != strings.Join(ftnKeyedRun, "\n") {
				t.Errorf("KEYS printed:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(ftnKeyedRun, "\n"))
			}
		})
	}
}
