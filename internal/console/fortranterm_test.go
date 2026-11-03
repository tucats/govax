package console

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// buildFortranProgram writes lines as DUA1:[000000]name.FOR and compiles,
// assembles, and links it, none of which may say anything; the files go
// when the test ends.
func buildFortranProgram(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer, name string, lines []string) {
	t.Helper()

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".FOR"},
		rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
		t.Fatal(err)
	}

	deleteFilesAfter(t, d, name)

	for _, command := range []string{"FORTRAN ", "MACRO ", "LINK "} {
		shareFindRun(t, d, command+name)

		if out := buf.String(); out != "" {
			t.Fatalf("%s%s said:\n%s", command, name, out)
		}
	}
}

// TestFortranTerminal runs the terminal statements end to end (Phase 80,
// subtask 13), as an interactive program such as DUNGEON uses them: a
// prompt written with $ and its reply read with A; TYPE, with a $ record
// and a + record that make one line; a WRITE to unit 5 and a READ of unit
// 6, which a terminal takes (VMS's units 5 and 6 are both SYS$INPUT and
// SYS$OUTPUT, the terminal); ACCEPT, READ f, READ *, and READ (*,f); the
// input's end, Ctrl/Z, taken by END=; and a prompt still open when the
// image ends, which ends its line so that DCL's output starts its own.
//
// The test's terminal doesn't echo, so a reply doesn't show after its
// prompt: what follows the prompt is the program's next record.
func TestFortranTerminal(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		buildFortranProgram(t, d, c, buf, "TERM", []string{
			"      PROGRAM TERM",
			"      CHARACTER*20 LINE",
			"      INTEGER N, M",
			"   10 FORMAT (' >',$)",
			"   20 FORMAT (A)",
			"   30 FORMAT (' Score: ',I3,$)",
			"   40 FORMAT ('+ points')",
			"   50 FORMAT (1X,2I5)",
			"      WRITE (6,10)",
			"      READ (5,20,END=90) LINE",
			"      TYPE *, 'GOT ', LINE",
			"      TYPE 30, 7",
			"      TYPE 40",
			"      WRITE (5,'('' to unit 5'')')",
			"      WRITE (5,'('' prompt 5: '',$)')",
			"      READ (6,*) N",
			"      PRINT *, 'N', N",
			"      TYPE '('' Two numbers: '',$)'",
			"      ACCEPT *, N, M",
			"      PRINT 50, N, M",
			"      ACCEPT 20, LINE",
			"      PRINT *, LINE",
			"      READ 20, LINE",
			"      PRINT *, LINE",
			"      READ *, N",
			"      PRINT *, N",
			"      READ (*,'(I3)') N",
			"      WRITE (*,'(1X,I3)') N",
			"      WRITE (6,10)",
			"      READ (5,20,END=90) LINE",
			"      PRINT *, 'NOT HERE'",
			"   90 TYPE *, 'END'",
			"      WRITE (6,10)",
			"      END",
		})

		c.In = strings.NewReader("look\n42\n3 4\nACCEPTED\nHELLO\n17\n123\n")

		want := ">GOT look                \n" +
			"Score:   7 points\n" +
			"to unit 5\n" +
			"prompt 5: N          42\n" +
			"Two numbers:     3    4\n" +
			"ACCEPTED            \n" +
			"HELLO               \n" +
			"         17\n" +
			"123\n" +
			">END\n" +
			">\n" +
			"  $STATUS == \"%X00000001\"\n"

		if out := shareRun(t, d, buf, "TERM"); out != want {
			t.Errorf("RUN TERM printed:\n%q\nwant\n%q", out, want)
		}
	})
}

// TestFortranTerminalControl is testdata/for80's FOR80T, the carriage
// control session VMS 7.1's terminal showed (vax/for80t.log), as FORTRAN
// statements. govax's terminal is written a line at a time (rms/ftncc.go),
// so it shows VMS's lines but for overprinting: where a + record follows a
// line that ended, VMS's terminal writes it over that line (the first one,
// and " after $", aren't in VMS's log for that reason) and govax's starts
// a line of its own. A + record after a $ record continues its line on
// both, and a 1 is a form feed.
func TestFortranTerminalControl(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	formats := []string{
		"' blank control'",
		"'+plus: over the last line'",
		"'0zero: a blank line before'",
		"'1one: a form feed before'",
		"' blank control'",
		"'$dollar: no return after'",
		"' after dollar'",
		"'Xother: as blank'",
		"' with $',$",
		"' after $'",
		"'+plus with $',$",
		"'+ after plus with $'",
		"'0zero with $',$",
		"' blank control'",
		"'1one with $',$",
		"' blank control'",
		"/,' a blank record first'",
		"' two records',/,' here'",
		"",
		"' blank control'",
		"' Type ABC: ',$",
	}

	lines := []string{"      PROGRAM FOR80T", "      CHARACTER*20 ANS"}
	for i, f := range formats {
		lines = append(lines, "      WRITE (6,"+strconv.Itoa(100+i)+")")
		lines = append(lines, "  "+strconv.Itoa(100+i)+" FORMAT ("+f+")")
	}

	lines = append(lines,
		"      READ (5,'(A)') ANS",
		"      WRITE (6,'('' You typed |'',A,''|'')') ANS",
		"      END")

	buildFortranProgram(t, d, c, buf, "FOR80T", lines)

	c.In = strings.NewReader("ABC\n")

	want := "blank control\n" +
		"plus: over the last line\n" +
		"\nzero: a blank line before\n" +
		"\fone: a form feed before\n" +
		"blank control\n" +
		"dollar: no return after\n" +
		"after dollar\n" +
		"other: as blank\n" +
		"with $\n" +
		"after $\n" +
		"plus with $ after plus with $\n" +
		"\nzero with $\n" +
		"blank control\n" +
		"\fone with $\n" +
		"blank control\n" +
		"\na blank record first\n" +
		"two records\n" +
		"here\n" +
		"\n" +
		"blank control\n" +
		"Type ABC: You typed |ABC                 |\n" +
		"  $STATUS == \"%X00000001\"\n"

	if out := shareRun(t, d, buf, "FOR80T"); out != want {
		t.Errorf("RUN FOR80T printed:\n%q\nwant\n%q", out, want)
	}
}

// TestFortranReadOutputFile reads unit 6, and PRINT's unit -1, where
// SYS$OUTPUT is a file: VMS's FORTRAN run-time library gave ERRDURREA (39)
// with RMS$_FAC (testdata/for80, round 2), which FTNRTL keeps for any
// SYS$OUTPUT but a terminal.
func TestFortranReadOutputFile(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	buildFortranProgram(t, d, c, buf, "RDOUT", []string{
		"      PROGRAM RDOUT",
		"      INTEGER I, IOS, IE, IS, IV, IU, IC",
		"      READ (6, *, IOSTAT=IOS) I",
		"      CALL ERRSNS(IE, IS, IV, IU, IC)",
		"      TYPE *, IOS, IE, IU",
		"      TYPE '(1X,Z8.8)', IS",
		"      END",
	})

	shareFindRun(t, d, "DEFINE/USER_MODE SYS$OUTPUT RDOUT.DAT")

	if out := shareRun(t, d, buf, "RDOUT"); out != "  $STATUS == \"%X00000001\"\n" {
		t.Fatalf("RUN RDOUT printed:\n%s", out)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]RDOUT.DAT"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	got := string(bytes.Join(records, []byte("|")))
	if want := "          39          39           6| 00018514"; got != want {
		t.Errorf("RDOUT.DAT holds %q, want %q", got, want)
	}
}

// TestFortranReadInputFile: a program's first READ of unit 5, with SYS$INPUT defined as a
// file, reads that file, as VMS's library does (docs/PHASE-86 - FORTRAN milestone.md): a
// $OPEN, not the $CREATE a first WRITE does, which would make a new, empty version of it.
// ACCEPT reaches SYS$INPUT too, and FOR005 is unit 5's own name for it.
func TestFortranReadInputFile(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	source := "      PROGRAM RDIN\n" +
		"      CHARACTER*20 L\n" +
		"   10 READ (5,'(A)',END=20) L\n" +
		"      WRITE (6,'(1X,A)') L\n" +
		"      GO TO 10\n" +
		"   20 CONTINUE\n" +
		"      END\n"
	accept := "      PROGRAM ACIN\n" +
		"      CHARACTER*20 L\n" +
		"      ACCEPT '(A)', L\n" +
		"      WRITE (6,'(1X,A)') L\n" +
		"      END\n"

	for name, text := range map[string]string{"RDIN.FOR": source, "ACIN.FOR": accept, "IN.TXT": "ONE\nTWO\n"} {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name},
			rms.TextRecords, splitSource(text)); err != nil {
			t.Fatal(err)
		}
	}

	for _, command := range []string{"FORTRAN RDIN,ACIN", "MACRO RDIN,ACIN", "LINK RDIN", "LINK ACIN"} {
		shareFindRun(t, d, command)
	}

	if out := buf.String(); out != "" {
		t.Fatalf("building said: %s", out)
	}

	for _, name := range []string{"SYS$INPUT", "FOR005"} {
		shareFindRun(t, d, "DEFINE/USER_MODE/NOLOG "+name+" IN.TXT")
		shareFindRun(t, d, "RUN RDIN")

		if out := buf.String(); out != "ONE                 \nTWO                 \n" {
			t.Errorf("with %s, it printed %q", name, out)
		}

		buf.Reset()
	}

	shareFindRun(t, d, "DEFINE/USER_MODE/NOLOG SYS$INPUT IN.TXT")
	shareFindRun(t, d, "RUN ACIN")

	if out := buf.String(); out != "ONE                 \n" {
		t.Errorf("ACCEPT printed %q", out)
	}

	// No READ made a new version of what it read.
	buf.Reset()
	shareFindRun(t, d, "DIRECTORY IN.TXT")

	if out := buf.String(); !strings.Contains(out, "IN.TXT;1") || strings.Contains(out, "IN.TXT;2") {
		t.Errorf("the input file is now:\n%s", out)
	}
}
