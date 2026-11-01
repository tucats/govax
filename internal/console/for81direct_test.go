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

// directCase is one of testdata/for81's cases on direct access, DEFINE FILE, and
// FIND, as a FORTRAN program on unit LU (10 to begin with). Each step is one of
//
//	O keywords	an OPEN, with FILE= the case's name if it has one
//	C		a CLOSE
//	W r n		a direct unformatted WRITE of record r, n bytes of 00, 01, ...
//	R r n		a direct unformatted READ of record r, n bytes
//	w r text	a direct formatted WRITE of record r, the text under (A)
//	r r n		a direct formatted READ of record r, n characters under (A)
//	S n, s n	a sequential unformatted WRITE or READ of n bytes
//	T text		a sequential formatted WRITE of the text
//	F r		a FIND of record r
//	D L|W m n	DEFINE FILE LU(m,n,U,v), v a longword or a word
//	A		the associated variable's longword shown, then set to -1
//	Q		an INQUIRE of OPENED, NEXTREC, and RECL
//	U n		LU set to n
//	V		the associated variable set to 12345678 (hex), as round 2 did
//	P		WALK.DAT made, a sequential file of one record
//	M name		a file the case reads made, on unit 99 (for81relfortran_test.go)
//	f n		a sequential formatted READ of n characters under (A)
//	X r		a DELETE of record r, Z one of the current record
//	K n		an unformatted REWRITE of n bytes (none for 0), k text a
//			formatted one under (A)
//	L		an UNLOCK
//	N, b, E		a REWIND, a BACKSPACE, an ENDFILE
//	Y, y		an unformatted and a formatted keyed READ
//
// A text's _ is a blank. An O step that names its FILE= opens that file.
//
// asv says whether VMS's run shows the associated variable (round 1's
// ASSOCIATEVARIABLE code was IOSTAT's, so its values there are IOSTAT's), and
// records whether its file's records are compared.
type directCase struct {
	run     string
	number  int
	name    string
	asv     bool
	records bool
	steps   []string
}

var directCases = []directCase{
	{"for81-vax.txt", 553, "DIR1.DAT", false, true, []string{"O ACCESS='DIRECT', RECL=2, STATUS='UNKNOWN'",
		"W 1 8", "W 4 8", "R 2 8", "R 3 8", "R 4 8", "R 5 8", "R 0 8", "R -1 8", "W 0 8", "W -1 8",
		"W 2 4", "W 3 12", "R 2 4", "R 2 12", "C"}},
	{"for81-vax.txt", 554, "DIR2.DAT", false, true, []string{"O ACCESS='DIRECT', RECL=11, FORM='FORMATTED', STATUS='UNKNOWN'",
		"w 1 ABC", "w 2 ABCDEFGHIJK", "w 3 ABCDEFGHIJKL", "r 1 11", "r 2 11", "r 3 11", "r 9 11", "C"}},
	{"for81-vax.txt", 555, "DIR3.DAT", false, true, []string{"O ACCESS='DIRECT', RECL=5, FORM='FORMATTED', STATUS='UNKNOWN'",
		"w 1 AAAAA", "w 2 BBBBB", "w 5 CCCCC", "r 2 5", "r 5 5", "C"}},
	{"for81-vax.txt", 556, "DIR4.DAT", false, false, []string{"O ACCESS='DIRECT', RECL=1, MAXREC=3, STATUS='UNKNOWN'",
		"W 3 4", "W 4 4", "R 3 4", "R 4 4", "C", "O ACCESS='DIRECT', RECL=1, MAXREC=2, STATUS='OLD'", "R 3 4"}},
	{"for81-vax.txt", 557, "DIR5.DAT", false, false, []string{"O ACCESS='DIRECT', RECL=1, STATUS='UNKNOWN'",
		"W 2 4", "S 4", "s 4", "R 2 4", "s 4"}},
	{"for81-vax.txt", 558, "DIR6.DAT", false, false, []string{"O RECORDTYPE='FIXED', RECL=4, FORM='UNFORMATTED', STATUS='UNKNOWN'",
		"S 4", "W 2 4", "R 1 4"}},
	{"for81-vax.txt", 559, "DIR7.DAT", false, false, []string{"O RECORDTYPE='VARIABLE', STATUS='UNKNOWN'",
		"T TEXT", "C", "O ACCESS='DIRECT', RECL=4, STATUS='OLD'"}},
	{"for81-vax.txt", 560, "DIR1.DAT", false, false, []string{"O ACCESS='DIRECT', RECL=2, STATUS='OLD', READONLY",
		"R 1 8", "W 1 8"}},
	{"for81-vax.txt", 561, "DIR1.DAT", false, false, []string{"O ACCESS='DIRECT', RECL=2, STATUS='OLD'",
		"F 2", "Q", "R 3 8", "F 99", "F 0"}},
	{"for81-vax.txt", 562, "WALK.DAT", false, false, []string{"P", "O STATUS='OLD'", "F 2", "U 18", "F 2"}},
	{"for81-vax.txt", 564, "", true, false, []string{"U 14", "D W 10 8", "A", "Q", "W 3 16", "A", "Q", "C"}},
	{"for81-vax.txt", 565, "", true, false, []string{"U 15", "D L 10 8", "A", "W 3 16", "A", "Q", "C"}},
	{"for81-vax.txt", 569, "", false, false, []string{"U 16", "D W 10 8", "O DISPOSE='DELETE'"}},
	{"for81f-vax.txt", 9, "ASV9.DAT", true, false, []string{"V", "O ACCESS='DIRECT', RECL=1, ASSOCIATEVARIABLE=ASVL",
		"A", "W 3 4", "A", "R 1 4", "A", "F 2", "A"}},
	{"for81f-vax.txt", 10, "ASV10.DAT", true, false, []string{"V", "O ACCESS='DIRECT', RECL=1, ASSOCIATEVARIABLE=ASVW",
		"A", "W 3 4", "A", "R 1 4", "A", "F 2", "A"}},
}

// directEvent is what VMS's run shows of one step: a statement's IOSTAT and the
// bytes it read, the associated variable, or an INQUIRE's three answers.
type directEvent struct {
	kind    byte // 's' a statement, 'a' the associated variable, 'q' an INQUIRE
	iostat  int
	text    string
	value   uint32
	opened  int
	nextrec int
	recl    int
}

var (
	directResult  = regexp.MustCompile(`^=(OPEN|CLOSE|IO_END|FIND|DEF_FILE|DEF_FILE_W|DELETE|DELETE_D|UNLOCK|REWIND|BACKSPACE|ENDFILE) r=([0-9A-F]+)`)
	directSignal  = regexp.MustCompile(`^=c ([0-9A-F]+) `)
	directASV     = regexp.MustCompile(`^ asv=([0-9A-F]+)`)
	directInquire = regexp.MustCompile(`^ q(\d+) r=[0-9A-F]+ v=([0-9A-F]+)`)
)

// forNumber is the IOSTAT of a FORRTL condition value: its FOR number, or -1
// for an end of file.
func forNumber(c uint64) int {
	if c == 0x001880C4 {
		return -1
	}

	return int(c>>3) & 0xFFF
}

// directExpected reads a case of VMS's run: its events, and its file's records,
// in hex.
func directExpected(t *testing.T, text string, dc directCase) (events []directEvent, records []string) {
	t.Helper()

	start := strings.Index(text, fmt.Sprintf("\n#%d ", dc.number))
	if start < 0 {
		t.Fatalf("no case #%d", dc.number)
	}

	lines := strings.Split(text[start+1:], "\n")[1:]
	signaled := uint64(0) // an OPEN's or CLOSE's failure, signaled after it returned

	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			break
		}

		if m := directResult.FindStringSubmatch(line); m != nil {
			r, _ := strconv.ParseUint(m[2], 16, 32)
			e := directEvent{kind: 's'}

			if r > 1 {
				e.iostat, signaled = forNumber(r), r
			}

			events = append(events, e)

			continue
		}

		if m := directSignal.FindStringSubmatch(line); m != nil {
			c, _ := strconv.ParseUint(m[1], 16, 32)
			if c == signaled {
				signaled = 0

				continue
			}

			events = append(events, directEvent{kind: 's', iostat: forNumber(c)})

			continue
		}

		if m := directASV.FindStringSubmatch(line); m != nil {
			if dc.asv {
				v, _ := strconv.ParseUint(m[1], 16, 32)
				events = append(events, directEvent{kind: 'a', value: uint32(v)})
			}

			continue
		}

		if m := directInquire.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseUint(m[2], 16, 32)

			switch m[1] {
			case "29":
				events = append(events, directEvent{kind: 'q'})
			case "31":
				events[len(events)-1].opened = int(int32(v))
			case "41":
				events[len(events)-1].recl = int(v)
			case "42":
				events[len(events)-1].nextrec = int(v)
			}

			continue
		}

		if strings.HasPrefix(line, "  0000:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "  0000:"))
			if strings.HasPrefix(lines[i-1], " rec ") {
				records = append(records, data)
			} else {
				events[len(events)-1].text = data
			}
		}
	}

	return events, records
}

// directProgram is a case's FORTRAN program.
func directProgram(dc directCase) []string {
	program := []string{
		"      INTEGER IOS, LU, I, J, N, NX, RL, IO, ASVL",
		"      INTEGER*2 ASVW",
		"      EQUIVALENCE (ASVL, ASVW)",
		"      LOGICAL LO",
		"      CHARACTER*64 BIG, RD",
		"      LU = 10",
		"      ASVL = -1",
		"      DO 1 I = 1, 64",
		"    1 BIG(I:I) = CHAR(I - 1)",
	}

	for k, step := range dc.steps {
		f := strings.Fields(step)
		text := func(i int) string { return strings.ReplaceAll(f[i], "_", " ") }
		args := strings.TrimSpace(step[1:])
		report := fmt.Sprintf("      WRITE (6, '(1X,I3,I5)') %d, IOS", k)
		show := func(n string) []string {
			return []string{
				"      N = MIN(" + n + ", 32)",
				"      IF (IOS .EQ. 0) THEN",
				fmt.Sprintf("      WRITE (6, '(1X,I3,I5,1X,32Z2.2)') %d, IOS, (ICHAR(RD(J:J)), J = 1, N)", k),
				"      ELSE", report, "      END IF",
			}
		}

		switch f[0] {
		case "O":
			name := ""
			if dc.name != "" && !strings.Contains(args, "FILE=") {
				name = "FILE='" + dc.name + "', "
			}

			program = append(program, fmt.Sprintf("      OPEN (LU, %s%s, IOSTAT=IOS)", name, args), report)
		case "C":
			program = append(program, "      CLOSE (LU, IOSTAT=IOS)", report)
		case "W":
			program = append(program, fmt.Sprintf("      WRITE (LU, REC=%s, IOSTAT=IOS) BIG(1:%s)", f[1], f[2]), report)
		case "R":
			program = append(program, "      RD = ' '", fmt.Sprintf("      READ (LU, REC=%s, IOSTAT=IOS) RD(1:%s)", f[1], f[2]))
			program = append(program, show(f[2])...)
		case "w":
			program = append(program, fmt.Sprintf("      WRITE (LU, '(A)', REC=%s, IOSTAT=IOS) '%s'", f[1], text(2)), report)
		case "r":
			program = append(program, "      RD = ' '", fmt.Sprintf("      READ (LU, '(A)', REC=%s, IOSTAT=IOS) RD(1:%s)", f[1], f[2]))
			program = append(program, show(f[2])...)
		case "S":
			program = append(program, fmt.Sprintf("      WRITE (LU, IOSTAT=IOS) BIG(1:%s)", f[1]), report)
		case "s":
			program = append(program, "      RD = ' '", fmt.Sprintf("      READ (LU, IOSTAT=IOS) RD(1:%s)", f[1]))
			program = append(program, show(f[1])...)
		case "T":
			program = append(program, fmt.Sprintf("      WRITE (LU, '(A)', IOSTAT=IOS) '%s'", text(1)), report)
		case "F":
			program = append(program, fmt.Sprintf("      FIND (LU, REC=%s, IOSTAT=IOS)", f[1]), report)
		case "D":
			v := "ASVL"
			if f[1] == "W" {
				v = "ASVW"
			}

			program = append(program, fmt.Sprintf("      DEFINE FILE LU(%s, %s, U, %s)", f[2], f[3], v),
				fmt.Sprintf("      WRITE (6, '(1X,I3,I5)') %d, 0", k))
		case "A":
			program = append(program, fmt.Sprintf("      WRITE (6, '(1X,I3,1X,Z8.8)') %d, ASVL", k), "      ASVL = -1")
		case "Q":
			program = append(program, "      INQUIRE (LU, OPENED=LO, NEXTREC=NX, RECL=RL)",
				"      IO = 0", "      IF (LO) IO = -1",
				fmt.Sprintf("      WRITE (6, '(1X,I3,I3,I5,I5)') %d, IO, NX, RL", k))
		case "U":
			program = append(program, "      LU = "+f[1])
		case "V":
			program = append(program, "      ASVL = 305419896")
		case "P":
			program = append(program, "      OPEN (LU, FILE='WALK.DAT', STATUS='NEW')",
				"      WRITE (LU, '(A)') 'ONE'", "      CLOSE (LU)")
		case "M":
			program = append(program, madeFile(f[1])...)
		case "f":
			program = append(program, "      RD = ' '", fmt.Sprintf("      READ (LU, '(A)', IOSTAT=IOS) RD(1:%s)", f[1]))
			program = append(program, show(f[1])...)
		case "X":
			program = append(program, fmt.Sprintf("      DELETE (LU, REC=%s, IOSTAT=IOS)", f[1]), report)
		case "Z":
			program = append(program, "      DELETE (LU, IOSTAT=IOS)", report)
		case "K":
			list := " BIG(1:" + f[1] + ")"
			if f[1] == "0" {
				list = ""
			}

			program = append(program, "      REWRITE (LU, IOSTAT=IOS)"+list, report)
		case "k":
			program = append(program, fmt.Sprintf("      REWRITE (LU, '(A)', IOSTAT=IOS) '%s'", text(1)), report)
		case "L":
			program = append(program, "      UNLOCK (LU, IOSTAT=IOS)", report)
		case "N":
			program = append(program, "      REWIND (LU, IOSTAT=IOS)", report)
		case "b":
			program = append(program, "      BACKSPACE (LU, IOSTAT=IOS)", report)
		case "E":
			program = append(program, "      ENDFILE (LU, IOSTAT=IOS)", report)
		case "Y":
			program = append(program, "      READ (LU, KEY='KEY', IOSTAT=IOS) RD(1:4)", report)
		case "y":
			program = append(program, "      READ (LU, '(A)', KEY='KEY', IOSTAT=IOS) RD(1:4)", report)
		}
	}

	return append(program, "      END")
}

// directWant is what a case's program prints, by VMS's events.
func directWant(t *testing.T, dc directCase, events []directEvent) string {
	t.Helper()

	var want strings.Builder

	e := 0

	for k, step := range dc.steps {
		switch step[0] {
		case 'U', 'V', 'P', 'M':
			continue
		}

		if e >= len(events) {
			t.Fatalf("#%d: VMS's run has %d events, the case more", dc.number, len(events))
		}

		ev := events[e]
		e++

		switch step[0] {
		case 'A':
			if ev.kind != 'a' {
				t.Fatalf("#%d: step %d is the variable, VMS's event %c", dc.number, k, ev.kind)
			}

			fmt.Fprintf(&want, "%3d %08X\n", k, ev.value)
		case 'Q':
			if ev.kind != 'q' {
				t.Fatalf("#%d: step %d is an INQUIRE, VMS's event %c", dc.number, k, ev.kind)
			}

			fmt.Fprintf(&want, "%3d%3d%5d%5d\n", k, ev.opened, ev.nextrec, ev.recl)
		default:
			if ev.kind != 's' {
				t.Fatalf("#%d: step %d is a statement, VMS's event %c", dc.number, k, ev.kind)
			}

			fmt.Fprintf(&want, "%3d%5d", k, ev.iostat)

			if (step[0] == 'R' || step[0] == 'r' || step[0] == 's' || step[0] == 'f') && ev.iostat == 0 {
				fmt.Fprintf(&want, " %s", ev.text)
			}

			want.WriteByte('\n')
		}
	}

	if e != len(events) {
		t.Fatalf("#%d: VMS's run has %d events, the case %d", dc.number, len(events), e)
	}

	return want.String()
}

// TestFor81DirectOracle holds direct access, FIND, DEFINE FILE, and
// ASSOCIATEVARIABLE to VMS's FORRTL (testdata/for81, rounds 1 and 2): each
// statement's IOSTAT, the bytes each read returned, the associated variable,
// INQUIRE's NEXTREC and RECL, and the records left in the files.
func TestFor81DirectOracle(t *testing.T) {
	runs := map[string]string{}

	for _, run := range []string{"for81-vax.txt", "for81f-vax.txt"} {
		text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", run))
		if err != nil {
			t.Fatal(err)
		}

		runs[run] = string(text)
	}

	d, c, buf := shareFindConsole(t, false)

	t.Cleanup(func() {
		for _, dc := range directCases {
			if dc.name != "" {
				_ = d.Dispatch("DELETE " + dc.name + ";*")
			}
		}

		_ = d.Dispatch("DELETE FOR014.DAT;*,FOR015.DAT;*")
	})

	// The cases run in order, as the probe's did: #560 and #561 read #553's file.
	for _, dc := range directCases {
		events, records := directExpected(t, runs[dc.run], dc)
		want := directWant(t, dc, events)

		name := fmt.Sprintf("DIR%d", dc.number)
		fortranBuild(t, d, c, buf, name, fixedForm(directProgram(dc)))

		out := strings.ReplaceAll(shareRun(t, d, buf, name), "\r\n", "\n")
		shareFindRun(t, d, "DELETE "+name+".EXE;*,"+name+".OBJ;*,"+name+".MAR;*")

		if got := strings.TrimRight(out[:strings.LastIndex(out, "$STATUS")], " "); got != want {
			t.Errorf("#%d printed:\n%s\nVMS's answers:\n%s", dc.number, got, want)
		}

		if !dc.records {
			continue
		}

		got, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + dc.name}, rms.VariableRecords)
		if err != nil {
			t.Fatalf("#%d: %v", dc.number, err)
		}

		gotRecords := make([]string, len(got))
		for i, r := range got {
			gotRecords[i] = strings.ToUpper(hex.EncodeToString(r))
		}

		if strings.Join(gotRecords, " ") != strings.Join(records, " ") {
			t.Errorf("#%d: the file holds %v, VMS's %v", dc.number, gotRecords, records)
		}
	}
}

// TestFortranDirectSignals is what direct access and DEFINE FILE say of a
// failure with no ERR= or IOSTAT=: FORRTL's messages, naming the unit, with RMS's
// status only where RMS reported it, as testdata/for81's #567 to #570 showed of
// the conditions.
func TestFortranDirectSignals(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	direct := "OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', ACCESS='DIRECT', RECL=1"

	for _, tc := range []struct {
		name       string
		statements []string
		want, not  []string
	}{
		{"past the end", []string{direct + ")", "READ (10'3) I"},
			[]string{"%FOR-F-ATTACCNON", "unit 10  file DUA1:[000000]P.DAT;", "-RMS-E-EOF"}, nil},
		{"record 0", []string{direct + ")", "N = 0", "WRITE (10'N) I"},
			[]string{"%FOR-F-RECNUMOUT", "unit 10"}, []string{"-RMS"}},
		{"no OPEN", []string{"FIND (11'2)"},
			[]string{"%FOR-F-OPEDEFREQ", "unit 11"}, []string{"-RMS"}},
		{"a sequential unit", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN')", "WRITE (10'2) I"},
			[]string{"%FOR-F-OPEDEFREQ", "unit 10"}, []string{"-RMS"}},
		{"DEFINE FILE of an open unit", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN')", "DEFINE FILE 10(10, 8, U, NR)"},
			[]string{"%FOR-F-UNIALROPE", "unit 10"}, nil},
		{"DEFINE FILE twice", []string{"DEFINE FILE 16(10, 8, U, NR)", "DEFINE FILE 16(10, 4, U, NR)"},
			[]string{"%FOR-F-DUPFILSPE", "unit 16"}, nil},
		{"DEFINE FILE of unit 120", []string{"DEFINE FILE 120(10, 8, U, NR)"},
			[]string{"%FOR-F-INVLOGUNI", "unit 120"}, nil},
		{"DEFINE FILE too long", []string{"DEFINE FILE 19(10, 40000, U, NR)"},
			[]string{"%FOR-F-INCRECTYP", "unit 19"}, nil},
		{"DEFINE FILE, then read with no file", []string{"DEFINE FILE 17(10, 8, U, NR)", "READ (17'1) I"},
			[]string{"%FOR-F-FILNOTFOU", "unit 17", "-RMS-E-FNF"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{"      PROGRAM SIGNAL"}

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
