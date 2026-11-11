package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLBR90Oracle runs part E of testdata/help90 (LBR90.MAR, docs/planning/PHASE-90 -
// VMS help.md, subtask 10) on govax, in both paging modes: the libraries it
// reads made by LIBRARY as HELP90.COM's part A made them, HLP$LIBRARY and
// HLP$LIBRARY_1 naming H90B and H90C as part C left them, and the program
// assembled, linked, and run. Its output is held to VMS 7.1's
// (vax/round1b/help90.log, from "#1" to the end of the run) line for line,
// but for what lbr90Masked says differs.
func TestLBR90Oracle(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "help90")

	want := lbr90Part(t, filepath.Join(dir, "vax", "round1b", "help90.log"))

	eachLoader(t, func(t *testing.T, paging bool) {
		d, _, buf := shareFindConsole(t, paging)

		for _, name := range []string{"h90a1.hlp", "h90a2.hlp", "h90b.hlp", "h90c.hlp", "t1.txt", "t2.txt", "h90m.mar", "lbr90.mar"} {
			src, err := filepath.Abs(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}

			shareFindRun(t, d, `COPY "`+src+`"/HOST DUA1:[000000]`+strings.ToUpper(name))
		}

		for _, cmd := range []string{
			"DEFINE H90DIR DUA1:[000000]",
			"LIBRARY/CREATE/HELP H90A H90A1,H90A2",
			"LIBRARY/CREATE/HELP H90B H90B",
			"LIBRARY/CREATE/HELP H90C H90C",
			"LIBRARY/CREATE/TEXT H90T T1,T2",
			"LIBRARY/CREATE/MACRO H90M H90M",
			"DEFINE HLP$LIBRARY DUA1:[000000]H90B",
			"DEFINE HLP$LIBRARY_1 DUA1:[000000]H90C",
			"MACRO LBR90",
			"LINK LBR90",
		} {
			_ = d.Dispatch(cmd)
		}

		buf.Reset()
		_ = d.Dispatch("RUN LBR90")

		got := lbr90Lines(buf.String())

		for i := 0; i < max(len(got), len(want)); i++ {
			var g, w string
			if i < len(got) {
				g = got[i]
			}

			if i < len(want) {
				w = want[i]
			}

			if lbr90Masked(g) != lbr90Masked(w) {
				t.Fatalf("line %d differs:\n got: %q\nwant: %q\n\ngot from there:\n%s", i+1, g, w,
					strings.Join(got[i:min(len(got), i+20)], "\n"))
			}
		}
	})
}

// lbr90Part reads VMS's log of the run: the lines from "#1 definitions" to
// the procedure's next command.
func lbr90Part(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	text := string(data)

	start := strings.Index(text, "#1 definitions")
	if start < 0 {
		t.Fatal("no part E in the log")
	}

	text = text[start:]
	if end := strings.Index(text, "\n$ "); end >= 0 {
		text = text[:end+1]
	}

	return lbr90Lines(text)
}

// lbr90Lines splits output into lines, its carriage returns but those in a
// prompt taken out, and starts it at the first case.
func lbr90Lines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")

	if at := strings.Index(text, "#1 definitions"); at >= 0 {
		text = text[at:]
	}

	if at := strings.Index(text, "\n  $STATUS"); at >= 0 {
		text = text[:at+1]
	}

	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

var (
	// lbr90Version and lbr90Times are the header's longwords 4 to 15: the
	// librarian's name, which govax's LIBRARY writes as its own, and the
	// library's creation and update times.
	lbr90Version = regexp.MustCompile(`^(    0: (?:[0-9A-F]{8} ){4})(?:[0-9A-F]{8} ?){4}$`)
	lbr90Times   = regexp.MustCompile(`^(    8: )(?:[0-9A-F]{8} ?){8}$`)
	// lbr90Fault is GET_RECORD's access violation with no buffer: VMS's
	// address and reason came from what followed the argument list.
	lbr90Fault = regexp.MustCompile(`^(  =c 0000000C n=5) a=.*$`)
)

// lbr90Masked is a line with what may differ from VMS's masked: the
// librarian's name and the libraries' times, and the access violation's details.
func lbr90Masked(line string) string {
	line = strings.TrimRight(line, " ")
	line = lbr90Version.ReplaceAllString(line, "${1}(name)")
	line = lbr90Times.ReplaceAllString(line, "${1}(name and times)")

	return lbr90Fault.ReplaceAllString(line, "${1} (address)")
}
