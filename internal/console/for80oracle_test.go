package console

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// TestFor80Oracle holds FTNRTL to VMS's FORTRAN run-time library
// (docs/PHASE-80 - FORTRAN formatted IO.md, Decision 2): testdata/for80's
// for80.for is the probe's output cases to unit 6 as FORTRAN statements,
// each between a line "#n" and a line "=n" with its IOSTAT, and VMS's run
// of the same cases through FORRTL is vax/for80-vax.txt, each case's
// records as FOR80D printed them (between bars, a byte that doesn't print
// as a period) and its e=, FOR$ERRSNS's number of the error. The program
// runs with SYS$OUTPUT a file, so that its records keep their first bytes,
// as VMS's unit 6 file kept them, and each case's records and IOSTAT must
// be VMS's records and e=.
//
// A case whose IOSTAT is 2, FOR$_NOTIMP, uses an edit FTNRTL doesn't have
// yet, and is passed over until the subtask that brings it; the test logs
// how many were compared. expectedFor80Differences lists the cases that
// are known to differ, with the reason.
func TestFor80Oracle(t *testing.T) {
	vms := readFor80VMS(t)

	d, c, buf := shareFindConsole(t, false)

	// The program and its MACRO file are larger than the shared volumes
	// hold: a volume of its own on DUA2.
	shareFindRun(t, d, "DISMOUNT DUA2")

	volume := filepath.Join(t.TempDir(), "for80.dsk")
	if err := c.cmd().InitializeContainer(volume, 20000, "FOR80", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA2:", volume, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	buf.Reset()

	source, err := os.ReadFile(filepath.Join(for80Dir, "for80.for"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA2:[000000]FOR80.FOR"},
		rms.TextRecords, splitSource(string(source))); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"FORTRAN FOR80", "MACRO FOR80", "LINK FOR80"} {
		shareFindRun(t, d, command)

		if out := buf.String(); out != "" {
			t.Fatalf("%s said:\n%s", command, out)
		}
	}

	shareFindRun(t, d, "DEFINE/USER_MODE SYS$OUTPUT FOR80.DAT")

	if out := shareRun(t, d, buf, "FOR80"); out != "  $STATUS == \"%X00000001\"\n" {
		t.Fatalf("RUN FOR80 printed:\n%s", out)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA2:[000000]FOR80.DAT"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	govax := splitFor80Cases(t, records)

	compared, notYet := 0, 0
	stale := map[int]bool{}

	for n := range expectedFor80Differences {
		stale[n] = true
	}

	numbers := make([]int, 0, len(govax))
	for n := range govax {
		numbers = append(numbers, n)
	}

	sort.Ints(numbers)

	for _, n := range numbers {
		got := govax[n]
		if got.status == 2 {
			notYet++

			continue
		}

		want, ok := vms[n]
		if !ok {
			t.Errorf("#%d: no case in VMS's run", n)

			continue
		}

		compared++

		same := got.status == want.status && strings.Join(got.records, "\n") == strings.Join(want.records, "\n")
		if _, known := expectedFor80Differences[n]; known {
			if same {
				t.Errorf("#%d now matches VMS's run: take it out of expectedFor80Differences", n)
			}

			delete(stale, n)

			continue
		}

		if !same {
			t.Errorf("#%d %s: got status %d and records\n%s\nwant status %d and records\n%s", n, want.title,
				got.status, strings.Join(got.records, "\n"), want.status, strings.Join(want.records, "\n"))
		}
	}

	for n := range stale {
		if govax[n].status != 2 {
			t.Errorf("#%d is in expectedFor80Differences but wasn't compared", n)
		}
	}

	t.Logf("%d cases compared with VMS's run, %d not yet (NOTIMP)", compared, notYet)

	if compared == 0 {
		t.Error("no case was compared")
	}
}

// expectedFor80Differences are the cases of TestFor80Oracle known to
// differ from VMS's run, each with the reason.
var expectedFor80Differences = map[int]string{
	68: "(1X,F45.40) of REAL*16 0.1: govax shows the value's exact digits; VMS's differ from the 38th on, past " +
		"the format's precision, where its digit routine isn't exact.",
	263: "(1X,E) of BYTE 42: VMS's library reads 4 bytes, so its value holds what follows the byte; " +
		"FTNRTL takes zeros there.",
	273: "(1X,G) of BYTE 42: as #263.",
	283: "(1X,D) of BYTE 42: as #263.",
	351: "(1X,P,I3): VMS's library finds no data descriptor (INFFORLOO) and writes nothing, where FTNRTL " +
		"takes P alone as 0P and edits I3; VMS's (1X,P) is INFFORLOO too, so perhaps a P with no count " +
		"ends its format, but one case can't say",
}

// for80Dir is the probe's directory.
var for80Dir = filepath.Join("..", "..", "testdata", "for80")

// for80Case is one case's result: its records, as FOR80D shows them, and
// its status (VMS's e=, govax's IOSTAT). title is VMS's line for it.
type for80Case struct {
	title   string
	records []string
	status  int
}

// for80Start and for80End match VMS's lines that begin and end a case.
var (
	for80Start = regexp.MustCompile(`^#(\d+) `)
	for80End   = regexp.MustCompile(`^=(\d+) r=\S+ e=(-?\d+) `)
)

// readFor80VMS reads VMS's run: each case's records and e=.
func readFor80VMS(t *testing.T) map[int]for80Case {
	t.Helper()

	text, err := os.ReadFile(filepath.Join(for80Dir, "vax", "for80-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cases := map[int]for80Case{}
	current := -1

	var c for80Case

	for _, line := range strings.Split(strings.TrimRight(string(text), "\n"), "\n") {
		line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")

		if m := for80Start.FindStringSubmatch(line); m != nil {
			current, _ = strconv.Atoi(m[1])
			c = for80Case{title: line}

			continue
		}

		if m := for80End.FindStringSubmatch(line); m != nil && current >= 0 {
			if n, _ := strconv.Atoi(m[1]); n == current {
				c.status, _ = strconv.Atoi(m[2])
				cases[current] = c
				current = -1
			}

			continue
		}

		if current >= 0 {
			c.records = append(c.records, line)
		}
	}

	return cases
}

// splitFor80Cases splits govax's records into cases, at the lines "#n"
// and "=n status" the program writes around each one. A record shows as
// FOR80D showed VMS's: a byte that doesn't print as a period.
func splitFor80Cases(t *testing.T, records [][]byte) map[int]for80Case {
	t.Helper()

	cases := map[int]for80Case{}
	current := -1

	var c for80Case

	for _, record := range records {
		fields := strings.Fields(string(record))

		if len(fields) == 1 && strings.HasPrefix(fields[0], "#") {
			if n, err := strconv.Atoi(fields[0][1:]); err == nil {
				current, c = n, for80Case{}

				continue
			}
		}

		if len(fields) == 2 && current >= 0 && fields[0] == "="+strconv.Itoa(current) {
			c.status, _ = strconv.Atoi(fields[1])
			cases[current] = c
			current = -1

			continue
		}

		if current >= 0 {
			shown := []byte(string(record))
			for i, b := range shown {
				if b < ' ' || b > '~' {
					shown[i] = '.'
				}
			}

			c.records = append(c.records, string(shown))
		}
	}

	if current >= 0 {
		t.Errorf("case #%d has no end", current)
	}

	return cases
}

// TestFor80InputOracle holds FTNRTL's formatted and list-directed input to VMS's FORTRAN
// run-time library, as TestFor80Oracle holds its output: testdata/for80's
// for80i.for is the probe's formatted and list-directed input cases as FORTRAN READs from
// SYS$INPUT, and for80i.dat their records, each case's under a line "#n"
// (both written by gen.go). Each case prints its IOSTAT and what each
// item's slot holds, which must be VMS's e= and v lines.
//
// VMS read most of these cases from an internal record, which for80i.dat
// gives as a record of SYS$INPUT: the same to the format, but for what
// comes after it, so a case whose internal record ran out on VMS
// (FOR$_ENDDURREA, 24) isn't compared. A case whose IOSTAT is 2,
// FOR$_NOTIMP, waits for its subtask, as in TestFor80Oracle.
func TestFor80InputOracle(t *testing.T) {
	vms := readFor80VMSInput(t)

	d, c, buf := shareFindConsole(t, false)

	shareFindRun(t, d, "DISMOUNT DUA2")

	volume := filepath.Join(t.TempDir(), "for80i.dsk")
	if err := c.cmd().InitializeContainer(volume, 20000, "FOR80I", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA2:", volume, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	buf.Reset()

	source, err := os.ReadFile(filepath.Join(for80Dir, "for80i.for"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA2:[000000]FOR80I.FOR"},
		rms.TextRecords, splitSource(string(source))); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"FORTRAN FOR80I", "MACRO FOR80I", "LINK FOR80I"} {
		if err := d.Dispatch(command); err != nil {
			t.Fatalf("%s: %v\n%s", command, err, buf.String())
		}

		if out := buf.String(); out != "" {
			t.Fatalf("%s said:\n%s", command, out)
		}
	}

	data, err := os.ReadFile(filepath.Join(for80Dir, "for80i.dat"))
	if err != nil {
		t.Fatal(err)
	}

	c.In = strings.NewReader(string(data))
	out := shareRun(t, d, buf, "FOR80I")

	govax := splitFor80InputCases(out)

	compared, notYet := 0, 0
	stale := map[int]bool{}

	for n := range expectedFor80InputDifferences {
		stale[n] = true
	}

	numbers := make([]int, 0, len(govax))
	for n := range govax {
		numbers = append(numbers, n)
	}

	sort.Ints(numbers)

	for _, n := range numbers {
		got := govax[n]
		if got.status == 2 {
			notYet++

			continue
		}

		want, ok := vms[n]
		if !ok {
			t.Errorf("#%d: no case in VMS's run", n)

			continue
		}

		if want.status == 24 {
			continue
		}

		compared++

		same := got.status == want.status && strings.Join(got.records, "\n") == strings.Join(want.records, "\n")
		if _, known := expectedFor80InputDifferences[n]; known {
			if same {
				t.Errorf("#%d now matches VMS's run: take it out of expectedFor80InputDifferences", n)
			}

			delete(stale, n)

			continue
		}

		if !same {
			t.Errorf("#%d %s: got status %d and\n%s\nwant status %d and\n%s", n, want.title,
				got.status, strings.Join(got.records, "\n"), want.status, strings.Join(want.records, "\n"))
		}
	}

	for n := range stale {
		if govax[n].status != 2 {
			t.Errorf("#%d is in expectedFor80InputDifferences but wasn't compared", n)
		}
	}

	t.Logf("%d input cases compared with VMS's run, %d not yet (NOTIMP)", compared, notYet)

	if compared == 0 {
		t.Errorf("no case was compared; the run printed:\n%s", out)
	}
}

// expectedFor80InputDifferences are the cases of TestFor80InputOracle
// known to differ from VMS's run, each with the reason.
var expectedFor80InputDifferences = map[int]string{}

// for80Shown matches VMS's lines showing what an item's slot holds.
var for80Shown = regexp.MustCompile(`^ v\d+=`)

// readFor80VMSInput reads VMS's run as readFor80VMS does, but each case's
// records are its v lines, what it stored, which follow its "=n" line.
func readFor80VMSInput(t *testing.T) map[int]for80Case {
	t.Helper()

	text, err := os.ReadFile(filepath.Join(for80Dir, "vax", "for80-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cases := map[int]for80Case{}
	current := -1

	var c for80Case

	for _, line := range strings.Split(strings.TrimRight(string(text), "\n"), "\n") {
		line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")

		if m := for80Start.FindStringSubmatch(line); m != nil {
			if current >= 0 {
				cases[current] = c
			}

			current, _ = strconv.Atoi(m[1])
			c = for80Case{title: line, status: -999}

			continue
		}

		if m := for80End.FindStringSubmatch(line); m != nil && current >= 0 {
			c.status, _ = strconv.Atoi(m[2])

			continue
		}

		if current >= 0 && for80Shown.MatchString(line) {
			c.records = append(c.records, strings.TrimPrefix(line, " "))
		}
	}

	if current >= 0 {
		cases[current] = c
	}

	return cases
}

// splitFor80InputCases reads for80i.for's output: each case's "=n status"
// line, then its "v j value..." lines, made into VMS's forms, "vj=XXXXXXXX
// XXXXXXXX" (the longwords in hexadecimal) or "vj=|text|" (a byte that
// doesn't print as a period).
func splitFor80InputCases(out string) map[int]for80Case {
	cases := map[int]for80Case{}
	current := -1

	var c for80Case

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.HasPrefix(fields[0], "=") {
			if n, err := strconv.Atoi(fields[0][1:]); err == nil {
				if current >= 0 {
					cases[current] = c
				}

				current = n
				c = for80Case{}
				c.status, _ = strconv.Atoi(fields[1])

				continue
			}
		}

		if current < 0 || len(fields) < 2 || fields[0] != "v" {
			continue
		}

		// List-directed output puts a character value right after the
		// integer before it.
		if first, last := strings.Index(line, "|"), strings.LastIndex(line, "|"); first >= 0 && last > first {
			fields = strings.Fields(line[:first])
			if len(fields) != 2 {
				continue
			}

			text := []byte(line[first+1 : last])
			for i, b := range text {
				if b < ' ' || b > '~' {
					text[i] = '.'
				}
			}

			c.records = append(c.records, "v"+fields[1]+"=|"+string(text)+"|")

			continue
		}

		var words []string

		for _, f := range fields[2:] {
			v, err := strconv.ParseInt(f, 10, 64)
			if err != nil {
				words = append(words, f)

				continue
			}

			words = append(words, strings.ToUpper(strconv.FormatUint(uint64(uint32(v)), 16)))
		}

		for i, w := range words {
			if len(w) < 8 {
				words[i] = strings.Repeat("0", 8-len(w)) + w
			}
		}

		c.records = append(c.records, "v"+fields[1]+"="+strings.Join(words, " "))
	}

	if current >= 0 {
		cases[current] = c
	}

	return cases
}
