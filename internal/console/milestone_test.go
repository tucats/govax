package console_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 48's acceptance tests: the multiprocessing program's milestone
// (docs/PHASE-43 - processes.md, "The milestone"; docs/PHASE-48 -
// LIB_SPAWN.md, subtask 6). testdata/mp/msparent.mar starts
// testdata/mp/mschild.mar, by $CREPRC or by LIB$SPAWN; the two exchange
// messages through mailboxes, and both write files, one of them shared,
// on an ODS-2 volume. Each run is checked line by line, file by file,
// and then the volume itself, under several quanta, so that the two
// processes interleave at different instructions.

// milestoneRounds is msparent.mar's number of rounds.
const milestoneRounds = 8

// milestoneQuanta are the quanta the milestone runs under: a very short
// one, so that switches land everywhere (in the services' retries, in
// RMS's appends); a medium one; and the default.
var milestoneQuanta = []string{"7", "500", "20000"}

// milestoneMachine boots a scheduled console at quantum with a fresh
// volume mounted on DUA0 as the default directory, and MSPARENT.EXE and
// MSCHILD.EXE assembled and linked onto it by govax's MACRO and LINK.
// It returns the console, its output, and the container's path.
func milestoneMachine(t *testing.T, quantum string) (*console.Console, *bytes.Buffer, string) {
	t.Helper()

	c, out := scheduledConsole(t, quantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 2000, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	for _, name := range []string{"mschild", "msparent"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		mar, obj := filepath.Join(dir, name+".mar"), filepath.Join(dir, name+".obj")
		if err := os.WriteFile(mar, src, 0o644); err != nil {
			t.Fatal(err)
		}

		for _, cmd := range []string{
			`MACRO "` + mar + `"/OBJECT="` + obj + `"`,
			`LINK "` + obj + `"/EXECUTABLE=DUA0:[000000]` + strings.ToUpper(name) + `.EXE`,
		} {
			if err := d.Dispatch(cmd); err != nil {
				t.Fatalf("%s: %v\n%s", cmd, err, out.String())
			}
		}
	}

	if err := d.Dispatch("SET DEFAULT DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	return c, out, path
}

// milestoneWant is the output msparent.mar's header gives, for how.
func milestoneWant(how string) []string {
	want := []string{"Parent: starting the child by " + how, "Child: running"}

	for n := 1; n <= milestoneRounds; n++ {
		want = append(want, fmt.Sprintf("Parent: round %d acknowledged", n))
	}

	return append(want,
		"Child: done",
		"Parent: child ended with status 00000003",
		fmt.Sprintf("Parent: SHARED.DAT has %d records from the parent and %d from the child", milestoneRounds, milestoneRounds),
	)
}

// checkMilestoneFiles reads the three files back and checks them:
// PARENT.DAT and CHILD.DAT each have their process's records in order;
// SHARED.DAT has every record of both, each process's in its own order.
// It returns how many times SHARED.DAT's records change from one
// process's to the other's.
func checkMilestoneFiles(t *testing.T, c *console.Console) int {
	t.Helper()

	read := func(name string) []string {
		t.Helper()

		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]" + name}, rms.TextRecords)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		lines := make([]string, len(records))
		for i, r := range records {
			lines[i] = string(r)
		}

		return lines
	}

	own := func(who string) []string {
		var lines []string
		for n := 1; n <= milestoneRounds; n++ {
			lines = append(lines, fmt.Sprintf("%s %d", who, n))
		}

		return lines
	}

	for name, who := range map[string]string{"PARENT.DAT": "PARENT", "CHILD.DAT": "CHILD"} {
		if got, want := read(name), own(who); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s:\n%s\nwant:\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}

	next := map[string]int{"PARENT": 1, "CHILD": 1}
	switches := 0
	last := ""

	for i, r := range read("SHARED.DAT") {
		who, n, ok := strings.Cut(r, " ")
		if _, known := next[who]; !ok || !known || n != fmt.Sprint(next[who]) {
			t.Fatalf("SHARED.DAT record %d is %q; want %s %d next", i+1, r, who, next[who])
		}

		next[who]++

		if last != "" && last != who {
			switches++
		}

		last = who
	}

	if next["PARENT"] != milestoneRounds+1 || next["CHILD"] != milestoneRounds+1 {
		t.Errorf("SHARED.DAT has %d parent and %d child records, want %d of each",
			next["PARENT"]-1, next["CHILD"]-1, milestoneRounds)
	}

	return switches
}

// verifyMilestoneVolume dismounts the volume (every file closed, its
// header written), mounts it again, and runs ods2's volume analysis: no
// lost, free-but-used, or multiply allocated blocks; headers,
// directories, and the index file consistent.
func verifyMilestoneVolume(t *testing.T, c *console.Console, path string) {
	t.Helper()

	if err := c.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	problems, err := c.Mounts.VerifyVolume("DUA0")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range problems {
		t.Errorf("volume: %s", p)
	}
}

// TestMilestone runs the milestone both ways, under each quantum.
func TestMilestone(t *testing.T) {
	for _, how := range []struct{ name, mode, want string }{
		{"creprc", "CREPRC", "$CREPRC"},
		{"spawn", "SPAWN", "LIB$SPAWN"},
	} {
		for _, quantum := range milestoneQuanta {
			t.Run(how.name+"/"+quantum, func(t *testing.T) {
				c, out, path := milestoneMachine(t, quantum)

				opts := console.RunOptions{CommandLine: how.mode + " DUA0:[000000]MSCHILD.EXE"}
				if err := c.Run("DUA0:[000000]MSPARENT.EXE", opts); err != nil {
					t.Fatalf("RUN: %v\n%s", err, out.String())
				}

				got := programLines(out.String())
				if want := milestoneWant(how.want); strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
				}

				if n := len(c.RTL.Processes()); n != 1 {
					t.Errorf("%d processes left, want only process 1", n)
				}

				switches := checkMilestoneFiles(t, c)
				t.Logf("SHARED.DAT changes writer %d times", switches)

				verifyMilestoneVolume(t, c, path)
			})
		}
	}
}

// TestMilestone_schedulerOff: with vax.process.scheduler off, $CREPRC
// and LIB$SPAWN both fail with SS$_UNSUPPORTED, which the parent reports
// before it ends with that status; no other process is made.
func TestMilestone_schedulerOff(t *testing.T) {
	for _, mode := range []string{"CREPRC", "SPAWN"} {
		t.Run(mode, func(t *testing.T) {
			c, out, _ := milestoneMachine(t, longQuantum)
			c.RTL.ProcessSettings.Scheduler = false

			opts := console.RunOptions{CommandLine: mode + " DUA0:[000000]MSCHILD.EXE"}
			if err := c.Run("DUA0:[000000]MSPARENT.EXE", opts); err != nil {
				t.Fatalf("RUN: %v\n%s", err, out.String())
			}

			got := programLines(out.String())
			if len(got) < 2 || got[1] != fmt.Sprintf("Parent: failed with status %08X", vmsdef.Symbols["SS$_UNSUPPORTED"]) {
				t.Errorf("output:\n%s\nwant the parent's failure, SS$_UNSUPPORTED, on its second line", strings.Join(got, "\n"))
			}

			if n := len(c.RTL.Processes()); n != 1 {
				t.Errorf("%d processes, want only process 1", n)
			}
		})
	}
}

// TestMilestone_vmsLog holds TestMilestone's expectations to VMS's run of
// the same two programs (testdata/mp/vax/milestone.log, from
// testdata/mp/run48's MILESTONE.COM): each way, the parent's lines are
// milestoneWant's, and the files TYPE showed have the records
// checkMilestoneFiles looks for. In the $CREPRC run the child's two
// lines went to the terminal, not the log (its SYS$OUTPUT is the
// terminal, as $CREPRC's caller gave it none), so they're left out of
// that comparison; in govax the console's output is both.
func TestMilestone_vmsLog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", "vax", "milestone.log"))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")

	// Each run: the program's lines, then TYPE's: SHARED.DAT's records,
	// and for each of PARENT.DAT and CHILD.DAT a blank line (a space),
	// its name, a blank line, and its records.
	runs := map[string][]string{}
	how := ""

	for _, line := range lines {
		if h, ok := strings.CutPrefix(line, "Parent: starting the child by "); ok {
			how = h
		}

		if how != "" && line != "" {
			runs[how] = append(runs[how], line)
		}
	}

	records := func(who string) []string {
		var r []string
		for n := 1; n <= milestoneRounds; n++ {
			r = append(r, fmt.Sprintf("%s %d", who, n))
		}

		return r
	}

	for _, how := range []string{"$CREPRC", "LIB$SPAWN"} {
		run := runs[how]

		var want []string

		for _, line := range milestoneWant(how) {
			if how == "LIB$SPAWN" || !strings.HasPrefix(line, "Child: ") {
				want = append(want, line)
			}
		}

		if len(run) < len(want) || strings.Join(run[:len(want)], "\n") != strings.Join(want, "\n") {
			t.Errorf("%s: VMS's lines:\n%s\nwant:\n%s", how, strings.Join(run, "\n"), strings.Join(want, "\n"))

			continue
		}

		typed := strings.Join(run[len(want):], "\n")
		files := strings.Split(typed, "\n \nDUA1:[000000]")

		if len(files) != 3 {
			t.Errorf("%s: TYPE's output:\n%s", how, typed)

			continue
		}

		// SHARED.DAT: both processes' records, each's in order.
		next := map[string]int{"PARENT": 1, "CHILD": 1}

		for _, r := range strings.Split(files[0], "\n") {
			who, n, _ := strings.Cut(r, " ")
			if n != fmt.Sprint(next[who]) {
				t.Errorf("%s: SHARED.DAT record %q, want %s %d next", how, r, who, next[who])
			}

			next[who]++
		}

		if next["PARENT"] != milestoneRounds+1 || next["CHILD"] != milestoneRounds+1 {
			t.Errorf("%s: SHARED.DAT's records: %v", how, next)
		}

		for i, who := range []string{"PARENT", "CHILD"} {
			want := who + ".DAT;1\n \n" + strings.Join(records(who), "\n")
			if files[i+1] != want {
				t.Errorf("%s: %s.DAT:\n%s\nwant:\n%s", how, who, files[i+1], want)
			}
		}
	}
}
