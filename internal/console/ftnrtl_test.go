package console

import (
	"bytes"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// FTNRTL, the FORTRAN run-time library (docs/PHASE-74 - FORTRAN
// plan.md, subtask 4): a MACRO program calls its routines, links against
// it with no options file, and runs, with the image built into govax or
// one of the user's own, in both paging modes.

// ftnCaller calls FTN$VERSION with a string 13 characters long (the text
// is 11, so two blanks pad it), says whether its value is V1.0's, then
// ends with STOP's display.
const ftnCaller = `	.TITLE	FTNCALL
	.PSECT	DATA,WRT,NOEXE,LONG
TEXT:	.BLKB	13
DESC:	.WORD	13
	.BYTE	14,1
	.ADDRESS TEXT
OK:	.ASCID	/version ok/
BAD:	.ASCID	/version wrong/
DISP:	.ASCID	/END OF RUN/
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<R2>
	PUSHAQ	DESC
	CALLS	#1,G^FTN$VERSION
	MOVL	R0,R2
	PUSHAQ	DESC
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVAQ	OK,R0
	CMPL	R2,#^X00010000
	BEQL	10$
	MOVAQ	BAD,R0
10$:	PUSHL	R0
	CALLS	#1,G^LIB$PUT_OUTPUT
	PUSHAQ	DISP
	CALLS	#1,G^FTN$STOP
	MOVL	#4,R0
	RET
	.END	START
`

// ftnStopper calls FTN$STOP with no display, and once with an omitted
// argument would do the same.
const ftnStopper = `	.TITLE	FTNSTOP
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	CLRL	-(SP)
	CALLS	#1,G^FTN$STOP
	MOVL	#4,R0
	RET
	.END	START
`

// ftnCallerRun is what FTNCALL writes, and $STATUS after it.
const ftnCallerRun = "FTNRTL V1.0  \nversion ok\nEND OF RUN\n  $STATUS == \"%X00000001\"\n"

func TestFTNRTL(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)
		s := c.ContainerSession

		for name, text := range map[string]string{"FTNCALL.MAR": ftnCaller, "FTNSTOP.MAR": ftnStopper} {
			if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
				t.Fatal(err)
			}
		}

		// SYS$SHARE leads nowhere that holds an FTNRTL, so the locator
		// gets to the built-in one.
		shareFindRun(t, d, "DEFINE SYS$SHARE DUA2:[000000]")

		for _, name := range []string{"FTNCALL", "FTNSTOP"} {
			shareFindRun(t, d, "MACRO "+name)
			shareFindRun(t, d, "LINK "+name)
		}

		if out := buf.String(); out != "" {
			t.Errorf("MACRO and LINK said:\n%s", out)
		}

		if got := shareRun(t, d, buf, "FTNCALL"); got != ftnCallerRun {
			t.Errorf("FTNCALL:\n%s\nwant:\n%s", got, ftnCallerRun)
		}

		if got, want := shareRun(t, d, buf, "FTNSTOP"), "FORTRAN STOP\n  $STATUS == \"%X00000001\"\n"; got != want {
			t.Errorf("FTNSTOP:\n%s\nwant:\n%s", got, want)
		}

		// The program's list of shareable images names FTNRTL, and the
		// image it ran with was govax's own.
		shareFindRun(t, d, "RUN/NOEXECUTE FTNCALL")

		if spec := ftnrtlSpec(c); spec != rms.BuiltinSpec(ftnrtl.Name) {
			t.Errorf("FTNRTL came from %q", spec)
		}

		// FTNRTL built from its sources with govax's commands, as
		// FTNRTL.OPT says, and named by a logical name, is used instead.
		ftnrtlBuild(t, d, c)
		buf.Reset()
		shareFindRun(t, d, "DEFINE FTNRTL DUA1:[000000]FTNRTL.EXE")

		if got := shareRun(t, d, buf, "FTNCALL"); got != ftnCallerRun {
			t.Errorf("FTNCALL with the user's FTNRTL:\n%s\nwant:\n%s", got, ftnCallerRun)
		}

		shareFindRun(t, d, "RUN/NOEXECUTE FTNCALL")

		if spec := ftnrtlSpec(c); !strings.HasPrefix(spec, "DUA1:[000000]FTNRTL.EXE") {
			t.Errorf("with FTNRTL defined, it came from %q", spec)
		}

		// LINK too reads the user's: a link with the logical name is the
		// same program.
		shareFindRun(t, d, "LINK/EXECUTABLE=FTNCALL2 FTNCALL")

		if got := shareRun(t, d, buf, "FTNCALL2"); got != ftnCallerRun {
			t.Errorf("FTNCALL2:\n%s", got)
		}
	})
}

// ftnrtlSpec is where the loaded program's FTNRTL came from.
func ftnrtlSpec(c *Console) string {
	for _, icb := range c.ImageState(nil).ICBList {
		if icb.Name == ftnrtl.Name {
			return icb.Spec.Name
		}
	}

	return ""
}

// ftnrtlBuild copies FTNRTL's sources to DUA1:[000000] and builds the
// image there with MACRO and LINK/SHAREABLE, as FTNRTL.OPT's comment
// says; the image is the built-in one but for the link time, the
// linker's name, and the modules' creation times.
func ftnrtlBuild(t *testing.T, d *Dispatcher, c *Console) {
	t.Helper()

	s := c.ContainerSession

	copyIn := func(file, name string) {
		data, err := fs.ReadFile(ftnrtl.Files, file)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(string(data))); err != nil {
			t.Fatal(err)
		}
	}

	copyIn(ftnrtl.OptionsFile, "FTNRTL.OPT")

	sources, err := fs.ReadDir(ftnrtl.Files, ftnrtl.SourceDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range sources {
		name := strings.ToUpper(e.Name())
		copyIn(path.Join(ftnrtl.SourceDir, e.Name()), name)
		shareFindRun(t, d, "MACRO "+strings.TrimSuffix(name, ".MAR"))
		shareFindRun(t, d, "DELETE "+name+";*") // the volume is small
	}

	if err := d.Dispatch("LINK/SHAREABLE=FTNRTL/NOSYSLIB FTNRTL/OPTIONS"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("LINK/SHAREABLE: %v", err)
	}

	built, _, err := s.ReadRawFile(rms.FileLocation{Name: "DUA1:[000000]FTNRTL.EXE"})
	if err != nil {
		t.Fatal(err)
	}

	builtin, _ := ftnrtl.Image(ftnrtl.Name)
	if len(built) != len(builtin) {
		t.Fatalf("the commands' FTNRTL is %d bytes, the built-in one %d", len(built), len(builtin))
	}

	differ := 0

	for i := range built {
		if built[i] != builtin[i] {
			differ++
		}
	}

	// The link time (8 bytes), the linker's name (up to 15), and each
	// module's creation time in the debug symbol table account for the
	// differences; anything more would mean the commands build another
	// image.
	if differ > 32 {
		t.Errorf("the commands' FTNRTL differs from the built-in one in %d bytes", differ)
	}

	if !bytes.Contains(built, []byte("FTN$STOP")) {
		t.Error("the commands' FTNRTL has no FTN$STOP")
	}
}

// ftnLister calls FTNRTL's list-directed output routines as a compiled
// PRINT * or WRITE (*,*) does (docs/PHASE-75 - FORTRAN first light.md,
// subtask 7): integers at I12's limits, logicals, texts, REAL values on
// each side of G editing's ranges, six REALs (a record that wraps), whole
// arrays, a text longer than a record, and a statement with no items.
const ftnLister = `	.TITLE	FTNLIST
	.MACRO	DESC	NAME,TYPE,LEN,ADDR
NAME:	.WORD	LEN
	.BYTE	TYPE,1
	.ADDRESS ADDR
	.ENDM
	.MACRO	ARRAY	NAME,TYPE,LEN,COUNT,ADDR
NAME:	.WORD	LEN
	.BYTE	TYPE,4
	.ADDRESS ADDR
	.BYTE	0,0,^XE0,1
	.LONG	LEN*COUNT
	.ADDRESS ADDR
	.LONG	COUNT
	.LONG	1,COUNT
	.ENDM
	.MACRO	STATEMENT START,ITEMS
	PUSHAB	ICB
	CALLS	#1,G^START
	.IRP	X,<ITEMS>
	PUSHAQ	X
	CALLS	#1,G^FTN$IO_ITEM
	.ENDR
	CALLS	#0,G^FTN$IO_END
	.ENDM
	.PSECT	DATA,WRT,NOEXE,LONG
ICB:	.WORD	8,0
	.LONG	-1
I1:	.LONG	42
I2:	.LONG	-7
I3:	.LONG	0
I4:	.LONG	2147483647
I5:	.LONG	^X80000000
LT:	.LONG	-1
LF:	.LONG	0
L3:	.LONG	3
R1:	.F_FLOATING 3.4
R2:	.F_FLOATING 1234567.0
R3:	.F_FLOATING 0.25
R4:	.F_FLOATING 0.0009765625
R5:	.F_FLOATING 0.0
R6:	.F_FLOATING -3.4
R7:	.F_FLOATING 9999999.0
R8:	.F_FLOATING 10000000.0
R9:	.F_FLOATING 0.1
R10:	.F_FLOATING 0.09999
R11:	.F_FLOATING 1.0E38
R12:	.F_FLOATING -7.888609052210118E-31
R13:	.F_FLOATING 123.456
R14:	.F_FLOATING 99999.996
IA:	.LONG	1,-2,3
RA:	.F_FLOATING 1.5,-0.5
T1:	.ASCII	/TEXT/
T2:	.ASCII	/IT'S/
T3:	.ASCII	/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQR/
	DESC	DI1,8,4,I1
	DESC	DI2,8,4,I2
	DESC	DI3,8,4,I3
	DESC	DI4,8,4,I4
	DESC	DI5,8,4,I5
	DESC	DLT,4,4,LT
	DESC	DLF,4,4,LF
	DESC	DL3,4,4,L3
	DESC	DR1,10,4,R1
	DESC	DR2,10,4,R2
	DESC	DR3,10,4,R3
	DESC	DR4,10,4,R4
	DESC	DR5,10,4,R5
	DESC	DR6,10,4,R6
	DESC	DR7,10,4,R7
	DESC	DR8,10,4,R8
	DESC	DR9,10,4,R9
	DESC	DR10,10,4,R10
	DESC	DR11,10,4,R11
	DESC	DR12,10,4,R12
	DESC	DR13,10,4,R13
	DESC	DR14,10,4,R14
	DESC	DT1,14,4,T1
	DESC	DT2,14,4,T2
	DESC	DT3,14,100,T3
	ARRAY	DIA,8,4,3,IA
	ARRAY	DRA,10,4,2,RA
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	STATEMENT FTN$PRINT_LIST,<DI1,DI2,DI3,DI4,DI5>
	STATEMENT FTN$WRITE_LIST,<DLT,DLF,DL3,DT1,DT2,DI1>
	STATEMENT FTN$PRINT_LIST,<DR1,DR2,DR3,DR4,DR5>
	STATEMENT FTN$PRINT_LIST,<DR6,DR7,DR8,DR9,DR10,DR11>
	STATEMENT FTN$PRINT_LIST,<DR12,DR13,DR14>
	STATEMENT FTN$PRINT_LIST,<DIA,DRA>
	STATEMENT FTN$PRINT_LIST,<DI1,DT3,DT1>
	STATEMENT FTN$PRINT_LIST,<>
	STATEMENT FTN$WRITE_LIST,<DT1>
	MOVL	#1,R0
	RET
	.END	START
`

// ftnListerRecords is what FTNLIST writes, each record's first byte its
// carriage control (a blank): a numeric or logical field's own first blank,
// or for a character value one of its own. A statement with no items writes a
// record of no bytes.
var ftnListerRecords = []string{
	"          42          -7           0  2147483647 -2147483648",
	" T F TTEXTIT'S          42",
	"   3.400000       1234567.      0.2500000      9.7656250E-04  0.0000000E+00",
	"  -3.400000       9999999.      1.0000000E+07  0.1000000      9.9990003E-02",
	"  9.9999997E+37",
	" -7.8886091E-31   123.4560       99999.99    ",
	"           1          -2           3   1.500000     -0.5000000    ",
	"          42",
	" 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ01234567",
	" 89ABCDEFGHIJKLMNOPQRTEXT",
	"",
	" TEXT",
}

// ftnListerLine is a record of ftnListerRecords as a line shows it, without
// its carriage control.
func ftnListerLine(r string) string {
	if r == "" {
		return r
	}

	return r[1:]
}

// TestFTNRTLListOutput runs FTNLIST at the terminal, where the carriage
// control is the line's (a blank, a new line), and in a command
// procedure with /OUTPUT=, whose log is a print file: there each
// record's carriage control becomes its fixed control (01 8D, a line),
// not data.
func TestFTNRTLListOutput(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)
		s := c.ContainerSession

		files := map[string]string{"FTNLIST.MAR": ftnLister, "FTNLIST.COM": "$ RUN FTNLIST\n"}
		for name, text := range files {
			if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
				t.Fatal(err)
			}
		}

		shareFindRun(t, d, "MACRO FTNLIST")
		shareFindRun(t, d, "LINK FTNLIST")

		if out := buf.String(); out != "" {
			t.Fatalf("MACRO and LINK said:\n%s", out)
		}

		var want strings.Builder
		for _, r := range ftnListerRecords {
			want.WriteString(ftnListerLine(r) + "\n")
		}

		want.WriteString("  $STATUS == \"%X00000001\"\n")

		if got := shareRun(t, d, buf, "FTNLIST"); got != want.String() {
			t.Errorf("FTNLIST:\n%s\nwant:\n%s", got, want.String())
		}

		if err := d.Dispatch("@FTNLIST/OUTPUT=FTNLIST.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("@FTNLIST: %v\n%s", err, buf)
		}

		records, _, err := s.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]FTNLIST.LOG"}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		var got []string
		for _, r := range records {
			if len(r) < 2 || r[0] != 0x01 || r[1] != 0x8D {
				t.Errorf("record %q: fixed control % X, want 01 8D", r, r[:min(len(r), 2)])

				continue
			}

			got = append(got, string(r[2:]))
		}

		var wantLog []string
		for _, r := range ftnListerRecords {
			wantLog = append(wantLog, ftnListerLine(r))
		}

		if strings.Join(got, "\n") != strings.Join(wantLog, "\n") {
			t.Errorf("FTNLIST.LOG:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantLog, "\n"))
		}
	})
}
