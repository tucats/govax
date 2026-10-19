package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/machine"
)

// Phase 67's probe (testdata/pg67, README.md): govax runs VMS V7.1's own
// images of the probe (vax/pg67.exe, and vax/pg67s.exe, linked with
// STACK=50) as PG67.COM ran them, and must answer as VMS did but for the
// differences listed here.

// pg67Dir is the probe's directory.
var pg67Dir = filepath.Join("..", "..", "testdata", "pg67")

// expectedPG67Differences are the program's lines (by their first words)
// that differ from VMS's, and why. VIRTPEAK's changes (peak=) are masked
// on every line instead (pg67Peak): it's the process's peak, which on
// VMS DCL and the earlier images had already taken above the size the
// program starts at.
var expectedPG67Differences = map[string]string{
	"A1 first instruction:":              "the stack the activation's call used (VMS's 112 bytes, govax's 88), FREPTECNT (VMS's VIRTUALPAGECNT is 65500 and DCL's pages are in its P1), and VIRTPEAK",
	"A1 pgflquota less pagfilcnt:":       "the job's charges at the first instruction: VMS's DCL, LIBRTL's writable pages, and P1's image pieces",
	"A12 cretva freptecnt+100 in P0:":    "the pages made before the quota stopped them: the quota left after each system's own charges",
	"A12 cretva in P0:":                  "as the line before",
	"A13 read beyond frep1va-freptecnt:": "the address's distance below FREP1VA, which is FREPTECNT's",
}

// pg67PeakField is a SPACE line's VIRTPEAK change.
var pg67PeakField = regexp.MustCompile(`peak=-?[0-9]+`)

// pg67Peak masks line's VIRTPEAK change.
func pg67Peak(line string) string {
	return pg67PeakField.ReplaceAllString(line, "peak=*")
}

// pg67Expected returns the reason line is expected to differ, or "".
func pg67Expected(line string) string {
	for key, why := range expectedPG67Differences {
		if strings.HasPrefix(line, key) {
			return why
		}
	}

	return ""
}

// pg67VAX reads one of VMS's output files.
func pg67VAX(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(pg67Dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n")
}

// pg67Compare matches got with VMS's output file name, line for line,
// VIRTPEAK masked, but for the expected differences.
func pg67Compare(t *testing.T, name string, got []string) {
	t.Helper()

	want := pg67VAX(t, name)
	if len(got) != len(want) {
		t.Errorf("%s: %d lines, VMS %d\n%s", name, len(got), len(want), strings.Join(got, "\n"))

		return
	}

	differ := 0

	for i := range want {
		if pg67Peak(got[i]) == pg67Peak(want[i]) {
			continue
		}

		differ++

		if pg67Expected(want[i]) == "" {
			t.Errorf("%s line %d:\n got %s\nwant %s", name, i+1, got[i], want[i])
		}
	}

	t.Logf("%s: %d lines, %d of them listed differences", name, len(want), differ)
}

// pg67Console is a console with paging on, VMS's two images of the probe
// copied to an RD54 volume on DUA1, which holds the page file too and is
// the default directory.
func pg67Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "20000")

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "pg67.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PG67`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, image := range []string{"PG67", "PG67S"} {
		src, err := filepath.Abs(filepath.Join(pg67Dir, "vax", strings.ToLower(image)+".exe"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST/BINARY DUA1:[000000]` + image + ".EXE")
	}

	return c, dispatch
}

// TestPG67Oracle runs VMS's images as PG67.COM did: parts A and S from
// PG67.EXE, T from both images, and C (part Q in a detached process with
// a small quota), and matches VMS V7.1's lines but for the expected
// differences. Process 1's quota is less than its table area's room, as
// SYSTEM's was on the VAX, so A12's P0 $CRETVA stops on the quota as
// VMS's did. And as on VMS, every image gives back what it took: after
// each run FREP0VA is empty, and FREP1VA, FREPTECNT, and PAGFILCNT are
// as after the first.
func TestPG67Oracle(t *testing.T) {
	c, dispatch := pg67Console(t)
	env := c.ProcessOne()

	job := env.Process.Job
	job.Pooled.PGFLQUOTA = job.PageFileCharged + 16384

	va := func() string {
		return dispatch(`WRITE SYS$OUTPUT "va frep0va=", F$GETJPI("","FREP0VA"), " frep1va=", F$GETJPI("","FREP1VA"), ` +
			`" freptecnt=", F$GETJPI("","FREPTECNT"), " pagfilcnt=", F$GETJPI("","PAGFILCNT")`)
	}

	run := func(part, image, out string) {
		t.Helper()

		dispatch("DEFINE PG67_PART " + part)
		pg67Compare(t, out, strings.Split(dispatch("RUN "+image), "|"))

		if st := dispatch("SHOW SYMBOL $STATUS"); !strings.Contains(st, `"%X00000001"`) {
			t.Errorf("%s from %s: %s", part, image, st)
		}
	}

	run("A", "PG67", "pg67a.out")

	after := va()
	if !strings.HasPrefix(after, "va frep0va= frep1va=") {
		t.Errorf("after part A: %q, want P0 empty (FREP0VA \"\") as VMS's", after)
	}

	run("S", "PG67", "pg67s.out")
	run("T", "PG67", "pg67t.out")
	run("T", "PG67S", "pg67t50.out")

	if got := va(); got != after {
		t.Errorf("after S and T: %q, after A %q; want each image's pages and charges given back", got, after)
	}

	// Part C: the image's and output file's names as PG67.COM gives
	// them, with the device's full name. Part Q's recursion outgrows its
	// quota with no stack left to report the access violation on: the
	// handler is passed over and the image ends with SS$_ACCVIO, nothing
	// written (the book's 16.2.1.3; VMS's answer).
	dispatch(`DEV = F$GETDVI(F$PARSE("PG67.EXE",,,"DEVICE"),"FULLDEVNAM")`)
	dispatch(`DIR = F$PARSE("PG67.EXE",,,"DIRECTORY")`)
	dispatch("DEFINE PG67_IMAGE 'DEV''DIR'PG67.EXE")
	dispatch("DEFINE PG67_OUTQ 'DEV''DIR'PG67Q.OUT")

	run("C", "PG67", "pg67c.out")
	pg67Compare(t, "pg67q.out", strings.Split(dispatch("TYPE PG67Q.OUT"), "|"))

	if got := va(); got != after {
		t.Errorf("after C: %q, after A %q", got, after)
	}
}
