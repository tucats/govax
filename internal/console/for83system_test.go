package console

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const for83SystemDir = "../../testdata/for83/"

// expectedFor83SystemDifferences are the lines of part A where FTNRTL's run on
// govax may differ from FORRTL's on VMS: both come from a bad argument, which
// the compiler refuses, and neither from what FTNRTL's routine does with it.
var expectedFor83SystemDifferences = map[string]string{
	// The same read-only page, and the same instruction's write: the address of
	// the field's byte on govax, 1 less, a longword's start, on the VAX.
	"IMVBITS FFFF 0 1 0 -1": "the address INSV's fault reports",
	// The argument past the list's end is what's on the stack, the same on both:
	// the reason mask's length bit is both's, and VMS's also has its write bit.
	"JMVBITS 4 args": "the reason mask of a fault at the stack's value",
}

// for83SystemMasked is a line of part A's output, or VMS's, with what depends on
// when it ran taken out: the date's or time's characters, IDATE's values, and the
// year R0 holds after them.
func for83SystemMasked(line, name string) string {
	switch {
	case strings.HasPrefix(name, "ASCTIM"):
		if head, _, ok := strings.Cut(line, `"`); ok && strings.HasPrefix(line, " ") {
			return head
		}

	case strings.HasPrefix(name, "DATE"), strings.HasPrefix(name, "TIME"):
		head, text, ok := strings.Cut(line, `"`)
		if !ok {
			break
		}

		b := []byte(text)

		for i := 0; i < len(b) && i < 9; i++ {
			if b[i] != '-' && b[i] != ':' && (b[i] != ' ' || i == 0) && b[i] != '"' {
				b[i] = 'x'
			}
		}

		return head + `"` + string(b)

	case strings.HasPrefix(name, "IDATE"), strings.HasPrefix(name, "JDATE"):
		line = for83SystemYear.ReplaceAllString(line, "R0=year")

		if strings.HasPrefix(line, "  I:") {
			return for83SystemStored.ReplaceAllString(line, "stored")
		}
	}

	return line
}

var (
	// for83SystemYear is what R0 holds after IDATE: the year's two digits.
	for83SystemYear = regexp.MustCompile(`R0=000000[0-9A-F]{2}$`)
	// for83SystemStored is a value IDATE stored, as against the guard's 55555555.
	for83SystemStored = regexp.MustCompile(`\b(5555|0000)00[0-9A-F]{2}\b`)
	// for83SystemNow is $ASCTIM's text, the first case's.
	for83SystemNow = regexp.MustCompile(`D: 23 " ?(\d+)-([A-Z]{3})-\d\d(\d\d) (\d\d:\d\d:\d\d)`)
	// for83SystemClock is hh:mm:ss, what TIME writes.
	for83SystemClock = regexp.MustCompile(`^\d\d:\d\d:\d\d`)
)

// TestFor83PartAOnFTNRTL runs the probe's part A with FTNRTL's routines in place
// of FORRTL's, each FOR$ name its FTN$, and holds them to VMS's run; then checks
// that what the mask took out agrees with $ASCTIM's date in this run.
func TestFor83PartAOnFTNRTL(t *testing.T) {
	src, err := os.ReadFile(for83SystemDir + "for83.mar")
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runMacroProgram(t, strings.ReplaceAll(string(src), "FOR$", "FTN$"))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	data, err := os.ReadFile(for83SystemDir + "vax/for83-vax.txt")
	if err != nil {
		t.Fatal(err)
	}

	got := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	want := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")

	if len(got) != len(want) {
		t.Errorf("%d lines of FTNRTL's, %d of VMS's", len(got), len(want))
	}

	name := ""
	seen := map[string]bool{}

	for i := range min(len(got), len(want)) {
		if n, _, ok := strings.Cut(want[i], " R0="); ok && !strings.HasPrefix(want[i], " ") {
			name = n
		}

		if for83SystemMasked(got[i], name) == for83SystemMasked(want[i], name) {
			continue
		}

		if why, ok := expectedFor83SystemDifferences[name]; ok {
			seen[name] = true

			t.Logf("%s, as expected (%s):\n\tFTNRTL %s\n\tVMS    %s", name, why, got[i], want[i])

			continue
		}

		t.Errorf("line %d, %s:\n\tFTNRTL %s\n\tVMS    %s", i+1, name, got[i], want[i])
	}

	for name := range expectedFor83SystemDifferences {
		if !seen[name] {
			t.Errorf("%s no longer differs: take it off the list", name)
		}
	}

	for83CheckToday(t, got)
}

// for83CheckToday checks the first DATE, TIME, IDATE, and JDATE cases against the
// date and time in $ASCTIM's line.
func for83CheckToday(t *testing.T, lines []string) {
	t.Helper()

	m := for83SystemNow.FindStringSubmatch(lines[1])
	if m == nil {
		t.Fatalf("no date in %q", lines[1])
	}

	months := "JANFEBMARAPRMAYJUNJULAUGSEPOCTNOVDEC"
	month := strings.Index(months, m[2])/3 + 1
	day, _ := strconv.Atoi(m[1])
	year, _ := strconv.Atoi(m[3])

	after := func(name string) string {
		for i, line := range lines {
			if strings.HasPrefix(line, name+" R0=") {
				return lines[i+1]
			}
		}

		t.Fatalf("no case %s", name)

		return ""
	}

	wantDate := fmt.Sprintf(`  B: 16 "%2d-%s-%02d---UUUU"`, day, m[2], year)
	if got := after("DATE by ref"); got != wantDate {
		t.Errorf("DATE wrote %q, want %q", got, wantDate)
	}

	_, clock, _ := strings.Cut(after("TIME by ref"), `"`)
	if !for83SystemClock.MatchString(clock) {
		t.Errorf("TIME wrote %q", clock)
	}

	wantWords := fmt.Sprintf("  I: 5555%04X 5555%04X 5555%04X", month, day, year)
	if got := after("IDATE words"); got != wantWords {
		t.Errorf("IDATE stored %q, want %q", got, wantWords)
	}

	wantLongs := fmt.Sprintf("  I: %08X 55555555 %08X 55555555 %08X 55555555", month, day, year)
	if got := after("JDATE longs"); got != wantLongs {
		t.Errorf("JDATE stored %q, want %q", got, wantLongs)
	}
}
