package console

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// fixedForm wraps each statement longer than the statement field (column 72)
// onto continuation lines, so that the tests may write statements whole.
func fixedForm(lines []string) []string {
	var out []string

	for _, line := range lines {
		for len(line) > 72 {
			out = append(out, line[:72])
			line = "     1" + line[72:]
		}

		out = append(out, line)
	}

	return out
}

// positionCase is one of testdata/for81's cases, as a FORTRAN program. Each
// step is W and a record to write, Rn a read of n characters, N a REWIND, B a
// BACKSPACE, or E an ENDFILE; a step of - is one of the case's statements that
// waits for its subtask. Name is the file whose records VMS's run lists.
type positionCase struct {
	run    string // the VMS run
	number int
	name   string
	open   string // the OPEN's keywords, or "" for none
	unit   int
	steps  []string
}

var positionCases = []positionCase{
	{"for81-vax.txt", 543, "WALK.DAT", "STATUS='UNKNOWN'", 10,
		[]string{"W ONE", "W TWO", "W THREE", "B", "R8", "N", "R8", "R8", "B", "B", "B", "R8", "W REPLACED"}},
	{"for81-vax.txt", 544, "WALK2.DAT", "STATUS='UNKNOWN'", 10,
		[]string{"W ONE", "W TWO", "W THREE", "N", "R8", "W NEW", "R8"}},
	{"for81-vax.txt", 545, "EOFS.DAT", "STATUS='UNKNOWN'", 10,
		[]string{"W ONE", "E", "W TWO", "E", "E", "N", "R8", "R8", "R8", "R8", "R8", "R8", "R8", "B", "R8"}},
	{"for81-vax.txt", 547, "", "ACCESS='DIRECT', RECL=4, STATUS='UNKNOWN'", 10, []string{"-", "N", "B", "E"}},
	{"for81-vax.txt", 549, "", "", 120, []string{"N", "B", "E"}},
	{"for81-vax.txt", 550, "", "STATUS='OLD', READONLY", 10, []string{"R8", "B", "N", "E"}},
	{"for81f-vax.txt", 21, "AV.DAT", "ACCESS='APPEND', STATUS='OLD'", 10,
		[]string{"W APPENDED", "B", "R12", "N", "R12", "W AFTER REWIND"}},
	{"for81f-vax.txt", 22, "AV.DAT", "ACCESS='APPEND', STATUS='UNKNOWN'", 10,
		[]string{"W APPENDED", "B", "R12", "N", "R12", "W AFTER REWIND"}},
	{"for81f-vax.txt", 26, "AN2.DAT", "ACCESS='APPEND', STATUS='UNKNOWN'", 10,
		[]string{"W APPENDED", "B", "R12", "N", "R12", "W AFTER REWIND"}},
	{"for81f-vax.txt", 27, "AF.DAT", "ACCESS='APPEND', STATUS='OLD'", 10,
		[]string{"W APPENDED", "B", "R12", "N", "R12", "W AFTER REWIND"}},
}

// positionFile names the file a case's OPEN has: #550 opens #543's, and
// #547's has no records to see.
func (oc positionCase) file() string {
	switch oc.number {
	case 547:
		return "DIRW.DAT"
	case 550:
		return "WALK.DAT"
	}

	return oc.name
}

// positionAnswer is what VMS's run says of a statement: its IOSTAT and the text
// of a read.
type positionAnswer struct {
	iostat int
	text   string
}

var (
	positionRecord = regexp.MustCompile(`^ rec \d+: r=([0-9A-F]+) n=(\d+)`)
	positionResult = regexp.MustCompile(`^=(IO_END|BACKSPACE|REWIND|ENDFILE) r=([0-9A-F]+)`)
	positionSignal = regexp.MustCompile(`^=c ([0-9A-F]+) `)
)

// positionExpected reads a case of a VMS run: each statement's answer between
// its OPEN and its CLOSE, then its file's records.
func positionExpected(t *testing.T, text string, number int) (answers []positionAnswer, records []string) {
	t.Helper()

	start := strings.Index(text, fmt.Sprintf("\n#%d ", number))
	if start < 0 {
		t.Fatalf("no case #%d", number)
	}

	lines := strings.Split(text[start+1:], "\n")[1:]

	bytesOf := func(line string) string {
		b, err := hex.DecodeString(strings.TrimSpace(strings.TrimPrefix(line, "  0000:")))
		if err != nil {
			t.Fatalf("#%d: %q: %v", number, line, err)
		}

		return string(b)
	}

	for i, line := range lines {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "#####") {
			break
		}

		if m := positionResult.FindStringSubmatch(line); m != nil {
			answers = append(answers, positionAnswer{})

			continue
		}

		if m := positionSignal.FindStringSubmatch(line); m != nil {
			c, _ := strconv.ParseUint(m[1], 16, 32)

			ios := int(c>>3) & 0xFFF
			if c == 0x001880C4 {
				ios = -1 // an end of file
			}

			answers = append(answers, positionAnswer{iostat: ios})

			continue
		}

		if strings.HasPrefix(line, "  0000:") && !strings.HasPrefix(lines[i-1], " rec ") {
			answers[len(answers)-1].text = bytesOf(line)

			continue
		}

		if m := positionRecord.FindStringSubmatch(line); m != nil && m[1] != "0001827A" {
			if m[2] == "0" {
				records = append(records, "")
			} else {
				records = append(records, bytesOf(lines[i+1]))
			}
		}
	}

	return answers, records
}

// TestFor81PositionOracle holds REWIND, BACKSPACE, and ENDFILE to VMS's
// FORRTL (testdata/for81): each statement's IOSTAT, each read's text, and the
// records left in the file, for the cases on formatted units.
func TestFor81PositionOracle(t *testing.T) {
	runs := map[string]string{}

	for _, name := range []string{"for81-vax.txt", "for81f-vax.txt"} {
		text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", name))
		if err != nil {
			t.Fatal(err)
		}

		runs[name] = string(text)
	}

	d, c, buf := shareFindConsole(t, false)

	// What #21 and #27 find, from the run's #19.
	fortranBuild(t, d, c, buf, "MAKE", fixedForm([]string{
		"      OPEN (11, FILE='AV.DAT', STATUS='UNKNOWN')",
		"      WRITE (11, '(A)') ' ONE'",
		"      WRITE (11, '(A)') ' TWO'",
		"      CLOSE (11)",
		"      OPEN (11, FILE='AF.DAT', STATUS='UNKNOWN', RECORDTYPE='FIXED', RECL=8)",
		"      WRITE (11, '(A)') ' ONE'",
		"      CLOSE (11)",
		"      END",
	}))
	shareRun(t, d, buf, "MAKE")

	t.Cleanup(func() {
		for _, name := range []string{"WALK", "WALK2", "EOFS", "DIRW", "AV", "AF", "AN2"} {
			_ = d.Dispatch("DELETE " + name + ".DAT;*")
		}
	})

	for _, oc := range positionCases {
		answers, records := positionExpected(t, runs[oc.run], oc.number)
		program := []string{"      INTEGER IOS, U", "      CHARACTER*12 T", fmt.Sprintf("      U = %d", oc.unit)}

		if oc.open != "" {
			program = append(program,
				fmt.Sprintf("      OPEN (U, FILE='%s',", oc.file()),
				"     1      "+oc.open+", IOSTAT=IOS)",
				"      IF (IOS .NE. 0) WRITE (6, *) 'OPEN', IOS",
			)
		}

		for k, step := range oc.steps {
			report := fmt.Sprintf("      WRITE (6, '(1X,I3,I5)') %d, IOS", k)

			switch {
			case step == "-":
			case step[0] == 'W':
				program = append(program, "      WRITE (U, '(A)', IOSTAT=IOS) '"+step[1:]+"'", report)
			case step[0] == 'R':
				n, _ := strconv.Atoi(step[1:])
				program = append(program, fmt.Sprintf("      READ (U, '(A)', IOSTAT=IOS) T(1:%d)", n),
					"      IF (IOS .EQ. 0) THEN",
					fmt.Sprintf("      WRITE (6, '(1X,I3,I5,4A)') %d, IOS, ' |', T(1:%d), '|'", k, n),
					"      ELSE", report, "      END IF")
			case step == "N":
				program = append(program, "      REWIND (U, IOSTAT=IOS)", report)
			case step == "B":
				program = append(program, "      BACKSPACE (U, IOSTAT=IOS)", report)
			case step == "E":
				program = append(program, "      ENDFILE (U, IOSTAT=IOS)", report)
			}
		}

		program = append(program, "      CLOSE (U, IOSTAT=IOS)", "      END")
		name := fmt.Sprintf("POS%d", oc.number)
		fortranBuild(t, d, c, buf, name, fixedForm(program))

		out := strings.ReplaceAll(shareRun(t, d, buf, name), "\r\n", "\n")
		shareFindRun(t, d, "DELETE "+name+".EXE;*,"+name+".OBJ;*,"+name+".MAR;*") // the volume is small

		if len(answers) != len(oc.steps) {
			t.Fatalf("#%d: VMS's run has %d statements, the case %d", oc.number, len(answers), len(oc.steps))
		}

		var want strings.Builder

		for k, step := range oc.steps {
			if step == "-" {
				continue
			}

			fmt.Fprintf(&want, "%3d%5d", k, answers[k].iostat)

			if step[0] == 'R' && answers[k].iostat == 0 {
				fmt.Fprintf(&want, " |%s|", answers[k].text)
			}

			want.WriteByte('\n')
		}

		if got := strings.TrimRight(out[:strings.LastIndex(out, "$STATUS")], " "); got != want.String() {
			t.Errorf("#%d printed:\n%s\nVMS's answers:\n%s", oc.number, got, want.String())
		}

		if oc.name == "" {
			continue
		}

		got, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + oc.name}, rms.TextRecords)
		if err != nil {
			t.Fatalf("#%d: %v", oc.number, err)
		}

		var gotText []string
		for _, r := range got {
			gotText = append(gotText, string(r))
		}

		if strings.Join(gotText, "|") != strings.Join(records, "|") {
			t.Errorf("#%d: the file holds %q, VMS's %q", oc.number, gotText, records)
		}
	}
}

// TestFor81PositionNotOpen is #548: REWIND and BACKSPACE of a unit not connected
// do nothing, and ENDFILE connects it, with no form yet, which its next statement
// gives it (a trial of FORRTL over govax; the phase doc's subtask 6).
func TestFor81PositionNotOpen(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	fortranBuild(t, d, c, buf, "NOTOPEN", fixedForm([]string{
		"      SUBROUTINE ASK",
		"      LOGICAL OP, NM",
		"      CHARACTER*12 AC, FM, FD, UF, BL, RT, CC",
		"      CHARACTER*40 FN",
		"      INTEGER RL",
		"      INQUIRE (18, OPENED=OP, NAMED=NM, NAME=FN, ACCESS=AC, FORM=FM, FORMATTED=FD, UNFORMATTED=UF, BLANK=BL, RECORDTYPE=RT, CARRIAGECONTROL=CC, RECL=RL)",
		"      WRITE (6, '(L2,L2,1X,A,7(1X,A),I5)') OP, NM, FN(1:24), AC, FM, FD, UF, BL, RT, CC, RL",
		"      END",
		"      PROGRAM NOTOPEN",
		"      CHARACTER*8 T",
		"      INTEGER IOS",
		"      REWIND 18",
		"      BACKSPACE 18",
		"      CALL ASK",
		"      ENDFILE 18",
		"      CALL ASK",
		"      WRITE (18, '(A)') ' AFTER'",
		"      CALL ASK",
		"      REWIND 18",
		"      READ (18, '(A)', IOSTAT=IOS) T",
		"      WRITE (6, *) IOS",
		"      READ (18, '(A)') T",
		"      WRITE (6, *) T",
		"      CLOSE (18, DISP='DELETE')",
		"      END",
	}))

	out := strings.ReplaceAll(shareRun(t, d, buf, "NOTOPEN"), "\r\n", "\n")
	// VMS's #548 for the first two lines: every text UNKNOWN but NAME, then
	// the file's own answers with FORM and BLANK still UNKNOWN.
	want := "F F                          UNKNOWN      UNKNOWN      UNKNOWN      UNKNOWN      UNKNOWN      UNKNOWN      UNKNOWN         0\n" +
		"T T DUA1:[000000]FOR018.DAT; SEQUENTIAL   UNKNOWN      YES          YES          UNKNOWN      VARIABLE     NONE          133\n" +
		"T T DUA1:[000000]FOR018.DAT; SEQUENTIAL   FORMATTED    YES          YES          ZERO         VARIABLE     NONE          133\n" +
		"         -1\n" +
		" AFTER  \n"

	if !strings.HasPrefix(out, want) {
		t.Errorf("RUN NOTOPEN printed:\n%s\nwant:\n%s", out, want)
	}
}

// TestFortranPositionSignals is what a statement with no ERR= or IOSTAT=
// says, as VMS's FORRTL said it over govax in the phase doc's subtask 6 (no VMS
// run has shown these messages). A BACKSPACE's second condition has no text yet:
// $PUTMSG has none to find (the phase doc's question).
func TestFortranPositionSignals(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	direct := "OPEN (10, FILE='P.DAT', ACCESS='DIRECT', RECL=4, STATUS='UNKNOWN')"
	file := "DUA1:[000000]P.DAT;"
	made := []string{
		"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN')",
		"WRITE (10, '(A)') ' ONE'",
		"CLOSE (10)",
	}

	for _, tc := range []struct {
		name       string
		statements []string
		want, not  []string
	}{
		{"direct REWIND", []string{direct, "REWIND 10"},
			[]string{"%FOR-F-REWERR, REWIND error\n  unit 10  file " + file}, []string{"-RMS", "NOMSG"}},
		{"direct BACKSPACE", []string{direct, "BACKSPACE 10"},
			[]string{"%FOR-F-BACERR, BACKSPACE error\n  unit 10  file " + file, "NOMSG, Message number 0018884C"}, []string{"-RMS"}},
		{"direct ENDFILE", []string{direct, "ENDFILE 10"},
			[]string{"%FOR-F-ENDFILERR, ENDFILE error\n  unit 10  file " + file}, []string{"-RMS", "NOMSG"}},
		{"readonly ENDFILE", append(append([]string{}, made...), "OPEN (10, FILE='P.DAT', STATUS='OLD', READONLY)", "ENDFILE 10"),
			[]string{"%FOR-F-ENDFILERR", "-RMS-F-FAC"}, nil},
		{"append BACKSPACE", append(append([]string{}, made...), "OPEN (10, FILE='P.DAT', STATUS='OLD', ACCESS='APPEND')", "BACKSPACE 10"),
			[]string{"%FOR-F-BACERR", "NOMSG, Message number 0018884C"}, []string{"-RMS"}},
		{"fixed ENDFILE", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', RECORDTYPE='FIXED', RECL=8)", "ENDFILE 10"},
			[]string{"%FOR-F-ENDFILERR"}, []string{"-RMS"}},
		{"unformatted variable ENDFILE", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', FORM='UNFORMATTED', RECORDTYPE='VARIABLE')", "ENDFILE 10"},
			[]string{"%FOR-F-ENDFILERR"}, []string{"-RMS"}},
		{"unformatted stream ENDFILE", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', FORM='UNFORMATTED', RECORDTYPE='STREAM')", "ENDFILE 10"},
			[]string{"%FOR-F-ENDFILERR"}, []string{"-RMS"}},
		{"unformatted segmented ENDFILE", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', FORM='UNFORMATTED')", "ENDFILE 10"}, nil, []string{"%FOR"}},
		{"stream ENDFILE", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', RECORDTYPE='STREAM')", "ENDFILE 10"}, nil, []string{"%FOR"}},
		{"stream_cr ENDFILE", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', RECORDTYPE='STREAM_CR')", "ENDFILE 10"}, nil, []string{"%FOR"}},
		{"stream_lf ENDFILE", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', RECORDTYPE='STREAM_LF')", "ENDFILE 10"}, nil, []string{"%FOR"}},
		{"reading ENDFILE's record", []string{
			"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN')", "ENDFILE 10", "REWIND 10", "READ (10, '(A)') X"},
			[]string{"%FOR-F-ENDDURREA"}, []string{"-RMS"}},
		{"reading the file's end", []string{
			"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN')", "REWIND 10", "READ (10, '(A)') X"},
			[]string{"%FOR-F-ENDDURREA", "-RMS-E-EOF"}, nil},
		{"the terminal", []string{"WRITE (6, '(A)') ' SIX'", "BACKSPACE 6"},
			[]string{"SIX\n%FOR-F-BACERR", "unit 6  file SYS$OUTPUT:.;\n", "NOMSG, Message number 00188844"}, []string{"-RMS"}},
		{"the terminal's REWIND first", []string{"WRITE (6, '(A)') ' SIX'", "REWIND 6", "BACKSPACE 6"},
			[]string{"%FOR-F-BACERR", "-RMS-F-IOP", "NOMSG, Message number 00188844"}, nil},
		{"TT:", []string{"OPEN (10, FILE='TT:', STATUS='UNKNOWN')", "BACKSPACE 10"},
			[]string{"%FOR-F-BACERR", "NOMSG, Message number 00188844"}, nil},
		{"NL:", []string{"OPEN (10, FILE='NL:', STATUS='UNKNOWN')", "BACKSPACE 10"},
			[]string{"%FOR-F-BACERR", "NOMSG, Message number 00188844"}, nil},
		{"the terminal goes on", []string{"WRITE (6, '(A)') ' SIX'", "REWIND 6", "ENDFILE 6", "WRITE (6, '(A)') ' SEVEN'"},
			[]string{"SIX\n", "SEVEN\n"}, []string{"%FOR"}},
		{"the record number after a REWIND", []string{
			"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN')", "WRITE (10, '(A)') 'ABCD'", "WRITE (10, '(A)') 'EFGH'",
			"REWIND 10", "READ (10, '(I4)') N"},
			[]string{`invalid text is "ABCD" in record number 1`}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{"      PROGRAM SIGNAL", "      CHARACTER*4 X"}

			for _, s := range tc.statements {
				lines = append(lines, "      "+s)
			}

			fortranBuild(t, d, c, buf, "SIGNAL", fixedForm(append(lines, "      END")))
			t.Cleanup(func() { _ = d.Dispatch("DELETE P.DAT;*") })

			out := strings.ReplaceAll(shareRun(t, d, buf, "SIGNAL"), "\r\n", "\n")

			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("no %q in:\n%s", want, out)
				}
			}

			for _, not := range tc.not {
				if strings.Contains(out, not) {
					t.Errorf("%q in:\n%s", not, out)
				}
			}
		})
	}
}

// TestFortranPositionProcedure is units 5's REWIND and BACKSPACE where SYS$INPUT
// is a command procedure: a process-permanent file (the phase doc's subtask 6).
// REWIND says nothing and moves nothing, and BACKSPACE is refused.
func TestFortranPositionProcedure(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	fortranBuild(t, d, c, buf, "ONFIVE", []string{
		"      CHARACTER*8 T",
		"      READ (5, '(A)') T",
		"      WRITE (6, '(A)') T",
		"      REWIND 5",
		"      READ (5, '(A)') T",
		"      WRITE (6, '(A)') T",
		"      BACKSPACE 5",
		"      END",
	})

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]ONFIVE.COM"},
		rms.TextRecords, splitSource("$ SET NOON\n$ RUN ONFIVE\n ONE\n TWO\n THREE\n")); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	_ = d.Dispatch("@ONFIVE")

	out := strings.ReplaceAll(buf.String(), "\r\n", "\n")
	for _, want := range []string{"ONE    \nTWO    \n%FOR-F-BACERR", "unit 5  file SYS$INPUT:.;\n", "NOMSG, Message number 00188844"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
}
