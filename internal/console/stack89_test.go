package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// stack89Dir holds Phase 89's programs of the user stack under recursion
// (docs/planning/PHASE-89 - reentrant FORTRAN.md, subtask 9).
const stack89Dir = "../../testdata/stack89"

// stack89Console boots a console in the paging mode given, with the
// programs named compiled, assembled, and linked on DUA1, and returns
// what runs an image and returns what it prints.
// With paging on it has a page file of the default size, as govax's
// system volume gives one, so that a stack of 10 MB, every page written,
// can be held at all: the test's 8 MB of memory can't.
func stack89Console(t *testing.T, paging bool, names ...string) func(string) string {
	t.Helper()

	if paging {
		withSettings(t, map[string]string{
			"vax.sysgen.paging.file": `"` + filepath.Join(t.TempDir(), "pagefile.sys") + `"`,
			"vax.sysgen.paging.size": "32768",
		})
	}

	d, c, buf := shareFindConsole(t, paging)
	if paging {
		c.StartPageFile()
	}

	for _, name := range names {
		text, err := os.ReadFile(filepath.Join(stack89Dir, strings.ToLower(name)+".for"))
		if err != nil {
			t.Fatal(err)
		}

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".FOR"},
			rms.TextRecords, splitSource(string(text))); err != nil {
			t.Fatal(err)
		}

		for _, command := range []string{"FORTRAN ", "MACRO ", "LINK "} {
			buf.Reset()
			shareFindRun(t, d, command+name)

			if out := buf.String(); out != "" {
				t.Fatalf("%s%s said:\n%s", command, name, out)
			}
		}
	}

	return func(image string) string { return shareRun(t, d, buf, image) }
}

// stack89Depths is how many levels of BIGFRAME's 800,000-byte frame fit:
// 2 in paging off's fixed 2 MB stack (P1's 8,192 pages less the 4,096
// above the stack's top), and 12 in the room the table area leaves P1
// with paging on (VMINIT's 24,576 entries less P0's and the 4,096).
var stack89Depths = map[bool]int{false: 2, true: 12}

// TestStack89BigFrames runs BIGFRAME: each depth's frames written page by
// page and read back intact, until the next depth doesn't fit, which ends
// the image with no message and an access violation's status.
func TestStack89BigFrames(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		run := stack89Console(t, paging, "BIGFRAME")

		var want strings.Builder
		for depth := 1; depth <= stack89Depths[paging]; depth++ {
			fmt.Fprintf(&want, "DEPTH%3d WRONG     0\n", depth)
		}

		want.WriteString("  $STATUS == \"%X1000000C\"\n")

		if got := run("BIGFRAME"); got != want.String() {
			t.Errorf("RUN BIGFRAME printed:\n%s\nwant:\n%s", got, want.String())
		}
	})
}

// stack89Levels are the least RUNAWAY's recursion must reach in each
// paging mode, a little under what the stack holds (some 87,000 and
// 430,000 levels of 24 bytes), and the most.
var stack89Levels = map[bool][2]int{false: {80000, 90000}, true: {420000, 440000}}

// TestStack89Runaway runs RUNAWAY, a recursion with no end: it goes on
// until the stack has no more room, then ends with no message and an
// access violation's status, and the next image runs as usual (SIGNAL,
// whose own frame is 800,000 bytes, its condition reported on the pages
// below it).
func TestStack89Runaway(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		run := stack89Console(t, paging, "RUNAWAY", "SIGNAL")

		got := run("RUNAWAY")
		lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")

		if status := lines[len(lines)-1]; status != `  $STATUS == "%X1000000C"` {
			t.Fatalf("RUN RUNAWAY ended with %q:\n%s", status, got)
		}

		last := 0

		for _, line := range lines[:len(lines)-1] {
			n, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil || n != last+10000 {
				t.Fatalf("RUN RUNAWAY printed %q after %d", line, last)
			}

			last = n
		}

		if limits := stack89Levels[paging]; last < limits[0] || last > limits[1] {
			t.Errorf("RUNAWAY reached %d levels, want %d to %d", last, limits[0], limits[1])
		}

		if got, want := maskCodeAddresses(run("SIGNAL")), stack89Signal; got != want {
			t.Errorf("RUN SIGNAL printed:\n%q\nwant:\n%q", got, want)
		}
	})
}

// stack89Signal is what SIGNAL prints: the handler's report and the
// continued statement in the first call, the traceback in the second.
const stack89Signal = "HANDLER SAW %X00000484\n" +
	"CONTINUED           1   14.00000000000000    \n" + // list-directed output's blanks
	"BACK           7\n" +
	"%SYSTEM-F-INTDIV, arithmetic trap, integer divide by zero at <code>, PSL=03C00002\n" +
	"%TRACE-F-TRACEBACK, symbolic stack dump follows\n" +
	"module name     routine name                     line       rel PC    abs PC\r\n" +
	"\n" +
	"SIGNAL          BIG                                        <code>\n" +
	"SIGNAL          SIGNAL                                     <code>\n" +
	"  $STATUS == \"%X10000484\"\n"
