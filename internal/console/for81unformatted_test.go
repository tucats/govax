package console

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// unformattedCase is one of testdata/for81's cases on unformatted sequential
// I/O, as a FORTRAN program. Each step is W and the lengths of a record's
// items (W alone, a record with none), R and the lengths of a read's items, F
// a formatted write of a text, O the unit closed and opened again with these
// keywords, N a REWIND, B a BACKSPACE, or E an ENDFILE. The OPEN's keywords
// are FORM='UNFORMATTED' and STATUS='UNKNOWN' with the case's, or the case's
// alone if they give a STATUS. Each item written is
// that many bytes of 00, 01, 02, ..., FF, 00, ..., as the probe's were. Name
// is the file whose records VMS's run lists, or "" for none.
type unformattedCase struct {
	number int
	name   string
	open   string
	steps  []string
}

var unformattedCases = []unformattedCase{
	{522, "F333.DAT", "", []string{"W1", "W2", "W3", "W4", "W5", "W6", "W7", "W8"}},
	{523, "F334.DAT", "", []string{"W0"}},
	{524, "F335.DAT", "", []string{"W2040", "W2041", "W2042", "W2043", "W2044", "W2045", "W2046", "W2047", "W2048"}},
	{525, "F336.DAT", "", []string{"W4088", "W4092", "W4096"}},
	{526, "F337.DAT", "", []string{"W8000"}},
	{527, "F338.DAT", "", []string{"W32000"}},
	{528, "F339.DAT", "RECL=25", []string{"W50", "W100", "W101", "W400"}},
	{529, "F340.DAT", "RECL=200", []string{"W800", "W801"}},
	{530, "F341.DAT", "RECORDTYPE='VARIABLE'", []string{"W1", "W2", "W3", "W2048"}},
	{531, "F342.DAT", "RECORDTYPE='VARIABLE', RECL=25", []string{"W100", "W101"}},
	{532, "F343.DAT", "RECORDTYPE='VARIABLE'", []string{"W8000"}},
	{533, "F344.DAT", "RECORDTYPE='FIXED', RECL=25", []string{"W50", "W100", "W101"}},
	{534, "F345.DAT", "RECORDTYPE='STREAM'", []string{"W10"}},
	{535, "F346.DAT", "NOSPANBLOCKS", []string{"W600", "W600"}},
	{536, "F347.DAT", "BLOCKSIZE=1024, BUFFERCOUNT=2", []string{"W1500"}},
	{537, "ITEMS.DAT", "", []string{"W4,4,4", "W2000,2000,2000", "W", "N", "R12", "R6000", "R4", "R4"}},
	{538, "", "", []string{"W8", "W8", "W8", "W3000", "N", "R4", "R8", "R12", "R3000", "R8", "R8"}},
	{539, "", "STATUS='OLD'", []string{"R4,4,4"}},
	{540, "BKSEG.DAT", "", []string{"W8", "W5000", "W12", "B", "B", "R5000", "B", "W16"}},
	{541, "MIX.DAT", "", []string{"F TEXT", "O FORM='FORMATTED', STATUS='UNKNOWN'", "W8"}},
	{542, "", "RECORDTYPE='VARIABLE'", []string{"W8", "O FORM='UNFORMATTED', RECORDTYPE='SEGMENTED', STATUS='OLD'", "R8"}},
	{546, "EOFU.DAT", "", []string{"W8", "E", "N", "R8", "R8", "R8"}},
}

// file is the file a case opens: #538's and #539's is RDSEG.DAT, and #542's
// NOTSEG.DAT.
func (uc unformattedCase) file() string {
	switch uc.number {
	case 538, 539:
		return "RDSEG.DAT"
	case 542:
		return "NOTSEG.DAT"
	}

	return uc.name
}

// unformattedRecord is a record as VMS's run lists it: its length and its
// first 32 bytes.
type unformattedRecord struct {
	n    int
	head string
}

// unformattedExpected reads a case of VMS's run: each statement's answer, and
// its file's records.
func unformattedExpected(t *testing.T, text string, number int) (answers []positionAnswer, records []unformattedRecord) {
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
		if strings.HasPrefix(line, "#") {
			break
		}

		if positionResult.MatchString(line) {
			answers = append(answers, positionAnswer{})

			continue
		}

		if m := positionSignal.FindStringSubmatch(line); m != nil {
			c, _ := strconv.ParseUint(m[1], 16, 32)

			ios := int(c>>3) & 0xFFF
			if c == 0x001880C4 {
				ios = -1
			}

			answers = append(answers, positionAnswer{iostat: ios})

			continue
		}

		if strings.HasPrefix(line, "  0000:") && !strings.HasPrefix(lines[i-1], " rec ") {
			answers[len(answers)-1].text = bytesOf(line)

			continue
		}

		if m := positionRecord.FindStringSubmatch(line); m != nil && m[1] != "0001827A" {
			n, _ := strconv.Atoi(m[2])
			r := unformattedRecord{n: n}

			if n > 0 {
				r.head = bytesOf(lines[i+1])
			}

			records = append(records, r)
		}
	}

	return answers, records
}

// unformattedItems is a step's items: BIG's first n bytes for each length,
// or RD's for a read.
func unformattedItems(lengths, variable string) string {
	var items []string

	for _, l := range strings.Split(lengths, ",") {
		if l == "" {
			continue
		}

		if l == "0" {
			items = append(items, variable+"(1:NZ)") // a constant (1:0) is no substring
		} else {
			items = append(items, variable+"(1:"+l+")")
		}
	}

	return strings.Join(items, ", ")
}

// TestFor81UnformattedOracle holds unformatted sequential I/O to VMS's FORRTL
// (testdata/for81, round 1): each statement's IOSTAT, the bytes each read
// returned, and the records left in the file (their lengths, and the first 32
// bytes VMS's run shows).
func TestFor81UnformattedOracle(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	d, c, buf := shareFindConsole(t, false)

	t.Cleanup(func() {
		for _, uc := range unformattedCases {
			_ = d.Dispatch("DELETE " + uc.file() + ";*")
		}
	})

	for _, uc := range unformattedCases {
		answers, records := unformattedExpected(t, string(text), uc.number)
		program := []string{
			"      INTEGER IOS, U, NZ, I, J, N",
			"      CHARACTER*32000 BIG, RD",
			"      U = 10",
			"      NZ = 0",
			"      DO 1 I = 1, 32000",
			"    1 BIG(I:I) = CHAR(MOD(I - 1, 256))",
		}

		open := "FORM='UNFORMATTED', STATUS='UNKNOWN'"

		switch {
		case strings.Contains(uc.open, "STATUS="):
			open = uc.open
		case uc.open != "":
			open += ", " + uc.open
		}

		program = append(program, fmt.Sprintf("      OPEN (U, FILE='%s', %s, IOSTAT=IOS)", uc.file(), open),
			"      IF (IOS .NE. 0) WRITE (6, *) 'OPEN', IOS")

		for k, step := range uc.steps {
			report := fmt.Sprintf("      WRITE (6, '(1X,I3,I5)') %d, IOS", k)

			switch step[0] {
			case 'W':
				program = append(program, "      WRITE (U, IOSTAT=IOS) "+unformattedItems(step[1:], "BIG"), report)
			case 'F':
				program = append(program, "      WRITE (U, '(A)', IOSTAT=IOS) '"+step[2:]+"'", report)
			case 'R':
				first := strings.Split(step[1:], ",")[0]
				program = append(program,
					"      RD = ' '",
					"      READ (U, IOSTAT=IOS) "+unformattedItems(step[1:], "RD"),
					"      N = MIN("+first+", 32)",
					"      IF (IOS .EQ. 0) THEN",
					fmt.Sprintf("      WRITE (6, '(1X,I3,I5,1X,32Z2.2)') %d, IOS, (ICHAR(RD(J:J)), J = 1, N)", k),
					"      ELSE", report, "      END IF")
			case 'O':
				program = append(program, "      CLOSE (U)",
					fmt.Sprintf("      OPEN (U, FILE='%s', %s, IOSTAT=IOS)", uc.file(), step[2:]),
					"      IF (IOS .NE. 0) WRITE (6, *) 'OPEN', IOS")
			case 'N':
				program = append(program, "      REWIND (U, IOSTAT=IOS)", report)
			case 'B':
				program = append(program, "      BACKSPACE (U, IOSTAT=IOS)", report)
			case 'E':
				program = append(program, "      ENDFILE (U, IOSTAT=IOS)", report)
			}
		}

		program = append(program, "      CLOSE (U, IOSTAT=IOS)", "      END")
		name := fmt.Sprintf("UNF%d", uc.number)
		fortranBuild(t, d, c, buf, name, fixedForm(program))

		out := strings.ReplaceAll(shareRun(t, d, buf, name), "\r\n", "\n")
		shareFindRun(t, d, "DELETE "+name+".EXE;*,"+name+".OBJ;*,"+name+".MAR;*")

		var want strings.Builder

		a := 0

		for k, step := range uc.steps {
			if step[0] == 'O' {
				continue
			}

			if a >= len(answers) {
				t.Fatalf("#%d: VMS's run has %d statements, the case more", uc.number, len(answers))
			}

			fmt.Fprintf(&want, "%3d%5d", k, answers[a].iostat)

			if step[0] == 'R' && answers[a].iostat == 0 {
				fmt.Fprintf(&want, " %X", answers[a].text)
			}

			want.WriteByte('\n')

			a++
		}

		if a != len(answers) {
			t.Fatalf("#%d: VMS's run has %d statements, the case %d", uc.number, len(answers), a)
		}

		if got := strings.TrimRight(out[:strings.LastIndex(out, "$STATUS")], " "); got != want.String() {
			t.Errorf("#%d printed:\n%s\nVMS's answers:\n%s", uc.number, got, want.String())
		}

		if uc.name == "" {
			continue
		}

		got, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + uc.name}, rms.VariableRecords)
		if err != nil {
			t.Fatalf("#%d: %v", uc.number, err)
		}

		var gotRecords []unformattedRecord

		for _, r := range got {
			head := r
			if len(head) > 32 {
				head = head[:32]
			}

			gotRecords = append(gotRecords, unformattedRecord{len(r), string(head)})
		}

		if fmt.Sprint(gotRecords) != fmt.Sprint(records) {
			t.Errorf("#%d: the file holds %d records:\n%s\nVMS's %d:\n%s", uc.number,
				len(gotRecords), showRecords(gotRecords), len(records), showRecords(records))
		}
	}
}

// showRecords lists records as VMS's run does: each one's length and first bytes.
func showRecords(records []unformattedRecord) string {
	var b strings.Builder

	for i, r := range records {
		fmt.Fprintf(&b, " rec %d: n=%d %X\n", i+1, r.n, r.head)
	}

	return b.String()
}

// TestFortranUnformattedSignals is what an unformatted statement with no ERR=,
// END=, or IOSTAT= says of each failure: FORRTL's messages, naming the unit and
// file, with RMS's status only where RMS reported the failure.
func TestFortranUnformattedSignals(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	unformatted := "OPEN (10, FILE='P.DAT', STATUS='UNKNOWN', FORM='UNFORMATTED'"
	file := "  unit 10  file DUA1:[000000]P.DAT;"

	for _, tc := range []struct {
		name       string
		statements []string
		want, not  []string
	}{
		{"past the record", []string{unformatted + ")", "WRITE (10) 1", "REWIND 10", "READ (10) I, J"},
			[]string{"%FOR-F-INPSTAREQ", file}, []string{"-RMS"}},
		{"past the file", []string{unformatted + ")", "READ (10) I"},
			[]string{"%FOR-F-ENDDURREA", file, "-RMS-E-EOF"}, nil},
		{"too long", []string{unformatted + ", RECORDTYPE='VARIABLE', RECL=1)", "WRITE (10) 1, 2"},
			[]string{"%FOR-F-OUTSTAOVE", file}, []string{"-RMS"}},
		{"no segments", []string{unformatted + ", RECORDTYPE='VARIABLE')", "WRITE (10) 1000",
			"CLOSE (10)", unformatted + ", RECORDTYPE='SEGMENTED')", "READ (10) I"},
			[]string{"%FOR-F-SEGRECFOR", file}, []string{"-RMS"}},
		{"formatted unit", []string{"OPEN (10, FILE='P.DAT', STATUS='UNKNOWN')", "WRITE (10) 1"},
			[]string{"%FOR-F-MIXFILACC", file}, []string{"-RMS"}},
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
