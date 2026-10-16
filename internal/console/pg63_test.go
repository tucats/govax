package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/machine"
)

// Phase 63's probe (testdata/pg63, README.md): govax runs the program and
// the SET WORKING_SET cases VMS V7.1 ran in round 2, and must answer as
// VMS did but for the differences listed here.

// pg63Dir is the probe's directory.
var pg63Dir = filepath.Join("..", "..", "testdata", "pg63")

// expectedPG63Differences are the program's lines (by their first words)
// that differ from VMS's, and why. Most come from the working set list's
// permanent part, which VMS has and govax doesn't (Decision 13): about
// 180 pages of page tables, the process header, and DCL's locked pages,
// in VMS's counts, its floor, and what a purge leaves.
var expectedPG63Differences = map[string]string{
	"A1":                "the counts at the image's start: VMS's include DCL's and the permanent part",
	"A3 syi BORROWLIM:": "SYSGEN's default in govax (300); that VAX had 143",
	"A3 syi PFCDEFAULT:": "SYSGEN's default in govax (32); that VAX had 16",
	"B4":                "the floor: VMS's is MINWSCNT above the permanent part (206 here), govax's MINWSCNT",
	"B6":                "PPGCNT includes VMS's permanent part",
	"B7":                "the first sweep's faults: VMS's limit has the permanent part in it",
	"C N=30:":           "PPGCNT includes VMS's permanent part, and VMS faulted a few of the program's pages back",
	"C N=60:":           "as N=30",
	"C N=75:":           "as N=30",
	"C N=90:":           "as N=30",
	"C N=120:":          "VMS refaulted far fewer pages in the second and third sweeps (TBSKIPWSL on an 8600's large TB)",
	"C N=300:":          "as N=30",
	"D1":                "what a purge of everything leaves: VMS's permanent part (178 pages)",
	"E7":                "VMS stops two or three pages earlier (room for page table pages, which govax's list doesn't hold)",
	"E9":                "as E7",
	"E10":               "GPGCNT: VMS counts a global page once it's touched, govax from the mapping (finding 9)",
	"G1":                "the counts at the end, the limit (floor+230), and the peak",
	"R1":                "the counts at the second image's start; WSSIZE and DFWSCNT agree",
	"R3":                "the peak: VMS's process had a larger one before the probe",
}

// pg63Expected returns the reason line is expected to differ, or "".
func pg63Expected(line string) string {
	for key, why := range expectedPG63Differences {
		if line == key || strings.HasPrefix(line, key+" ") || strings.HasPrefix(line, key) && strings.HasSuffix(key, ":") {
			return why
		}
	}

	return ""
}

// pg63Console is a console with paging on and SYSGEN's parameters as
// the VAX had them, where govax has keys for them.
func pg63Console(t *testing.T) (func(string) string, func(string) []string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")

	for key, value := range map[string]string{
		"vax.sysgen.growlim": "143", "vax.sysgen.freelim": "143", "vax.sysgen.freegoal": "8192",
		"vax.sysgen.mpw_hilimit": "5634", "vax.sysgen.mpw_lolimit": "120", "vax.sysgen.tbskipwsl": "8",
	} {
		setSetting(t, key, value)
	}

	c, dispatch := wsDispatcher(t)

	if c.Mem.Paging() == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	src, err := os.ReadFile(filepath.Join(pg63Dir, "pg63.mar"))
	if err != nil {
		t.Fatal(err)
	}

	exe := buildImage(t, c, "pg63", string(src))

	return dispatch, func(part string) []string {
		t.Helper()

		dispatch("DEFINE PG63_PART " + part)

		return strings.Split(dispatch(`RUN "`+exe+`"`), "|")
	}
}

// vaxLines reads one of VMS's output files.
func vaxLines(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(pg63Dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n")
}

// TestPG63Oracle runs the program's two parts and compares their lines
// with VMS's.
func TestPG63Oracle(t *testing.T) {
	_, run := pg63Console(t)

	for _, part := range []struct{ name, file string }{{"A", "pg63a.out"}, {"B", "pg63b.out"}} {
		got, want := run(part.name), vaxLines(t, part.file)

		if len(got) != len(want) {
			t.Fatalf("part %s: %d lines, VMS %d\n%s", part.name, len(got), len(want), strings.Join(got, "\n"))
		}

		differ := 0

		for i := range want {
			if got[i] != want[i] {
				differ++
			}

			if got[i] != want[i] && pg63Expected(want[i]) == "" {
				t.Errorf("part %s line %d:\n got %s\nwant %s", part.name, i+1, got[i], want[i])
			}
		}

		t.Logf("part %s: %d lines, %d of them listed differences", part.name, len(want), differ)
	}
}

// expectedPG63SetDifferences are the subprocess cases whose answer is
// VMS's floor (83 pages for a subprocess's DCL), which govax's is not:
// govax's floor is MINWSCNT (20).
var expectedPG63SetDifferences = map[string]bool{
	"/LIMIT=50/LOG": true, "/LIMIT=5/LOG": true, "/LIMIT=0/LOG": true,
	"/QUOTA=5/LOG": true, "/QUOTA=5": true, "/QUOTA=0/LOG": true,
	"/EXTENT=21/LOG": true, "/EXTENT=20/LOG": true, "/EXTENT=19/LOG": true, "/EXTENT=0/LOG": true,
}

// TestPG63SetWorkingSet replays VMS's SET WORKING_SET and SHOW
// WORKING_SET commands from the probe's log: the main process's (the
// command lines as VMS's verification shows them, after substitution),
// then each subprocess case, whose message is compared.
func TestPG63SetWorkingSet(t *testing.T) {
	dispatch, _ := pg63Console(t)
	log := vaxLines(t, "pg63.log")

	// The main process's commands, from the first SET WORKING_SET to the
	// subprocess cases.
	start, end := -1, -1

	for i, line := range log {
		switch {
		case start < 0 && line == "$ SET WORKING_SET":
			start = i
		case start >= 0 && strings.HasPrefix(line, "$ SPAWN/OUTPUT=PG63X.LOG"):
			end = i
		}

		if end >= 0 {
			break
		}
	}

	if start < 0 || end < 0 {
		t.Fatal("the log's SET WORKING_SET section isn't there")
	}

	compared := 0

	for i := start; i < end; i++ {
		cmd := strings.TrimPrefix(log[i], "$ ")
		if !strings.HasPrefix(cmd, "SET WORKING_SET") && !strings.HasPrefix(cmd, "SHOW WORKING_SET") {
			continue
		}

		var want []string

		for j := i + 1; j < end && !strings.HasPrefix(log[j], "$ ") && !strings.HasPrefix(log[j], "ws "); j++ {
			want = append(want, log[j])
		}

		if got := dispatch(cmd); got != strings.Join(want, "|") {
			t.Errorf("%s:\n got %q\nwant %q", cmd, got, strings.Join(want, "|"))
		}

		compared++
	}

	// The subprocess cases (PG63X.COM's and PG63S.COM's, up to the
	// program's MACRO): each SET's message, in a subprocess of its own.
	for i := end; i < len(log) && log[i] != "$ MACRO/NOLIST PG63"; i++ {
		q, ok := strings.CutPrefix(log[i], "$ SET WORKING_SET/")
		if !ok || i+1 >= len(log) {
			continue
		}

		q = "/" + q
		want := log[i+1]

		got := dispatch("SPAWN SET WORKING_SET" + q)
		msg := ""

		for _, l := range strings.Split(got, "|") {
			if strings.HasPrefix(l, "%SET-") {
				msg = l
			}
		}

		switch {
		case expectedPG63SetDifferences[q]:
			if !strings.HasPrefix(msg, "%SET-I-NEWLIMS") {
				t.Errorf("SPAWN SET WORKING_SET%s: %q, want NEWLIMS at govax's floor", q, got)
			}
		case msg != want:
			t.Errorf("SPAWN SET WORKING_SET%s:\n got %q\nwant %q", q, msg, want)
		}

		compared++
	}

	t.Logf("%d commands compared", compared)

	if compared < 40 {
		t.Errorf("only %d commands compared", compared)
	}
}
