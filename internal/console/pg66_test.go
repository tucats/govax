package console_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
)

// Phase 66's probe (testdata/pg66, README.md): govax runs VMS V7.1's own
// images of the probe (vax/pg66.exe, and vax/pg66c.exe, whose 100-page
// read-only section has a cluster factor of 8) as PG66.COM ran them, and
// must answer as VMS did but for the differences listed here.

// pg66Dir is the probe's directory.
var pg66Dir = filepath.Join("..", "..", "testdata", "pg66")

// expectedPG66Differences are the program's lines (by their first words)
// that differ from VMS's, and why. Most come from what VMS has and govax
// doesn't: LIBRTL and RMS as installed shareable images and P1 code,
// whose pages fault and are charged, where govax's are shims; and a
// working set list holding DCL's pages and its permanent part (page
// tables, the process header), so that the image's pages replace each
// other there and not in govax's (testdata/pg63's differences).
var expectedPG66Differences = map[string]string{
	"A1 at the first instruction:":  "activation's faults and charges: VMS's image activator, LIBRTL's writable pages, and P1's image pieces; govax charges the image's writable and demand-zero pages and its user stack alone",
	"A2 LIB$INIT_TIMER:":            "LIBRTL's pages fault on VMS (global pages); govax's LIB$ routines are shims",
	"A2 the program's start:":       "RMS's own pages and its I/O to the SYS$OUTPUT file PG66.COM defines; govax's RMS is in Go, and its output goes to the console",
	"A5 BIG read: flts":             "the clusters as govax's (PFCDEFAULT 16, or the section's 8), but VMS's working set list, full of DCL's and its permanent part's pages, replaced BIG's own pages as they came in (PPGCNT 38 to 80, not 100), and some faulted again",
	"A6 BIG read again:":            "as A5: BIG's pages that left VMS's working set fault again",
	"A7 ZERO read: flts":            "the 20 demand-zero faults as govax's, but PPGCNT and GPGCNT as A5's",
	"A7 ZERO written:":              "as A5: a page that left VMS's working set faults again",
	"A8 LIB$STAT_TIMER":             "LIB$STAT_TIMER's own page fault, which it counts: govax's is a shim",
	"A9 open to read, others read:": "the STV of a successful $OPEN: VMS's (%XA0, perhaps the file's channel) where govax's holds the status",
	"A11 the whole image:":          "the sum of the lines above",
	"W1 BIG read: flts":             "at the floor plus 10, VMS's working set list holds its permanent part, and VMS read clusters by replacing BIG's older pages, where govax's list, with no permanent part, takes a cluster only into free entries (the book's 15.5.1.1; testdata/pg65's K2 found VMS not clustering at all)",
	"W2 BIG read again:":            "as W1: VMS's soft faults include its other pages' (DCL's, page tables)",
	"W3 and a third time:":          "as W2",
}

// pg66Expected returns the reason line is expected to differ, or "".
func pg66Expected(line string) string {
	for key, why := range expectedPG66Differences {
		if strings.HasPrefix(line, key) {
			return why
		}
	}

	return ""
}

// pg66VAX reads one of VMS's output files.
func pg66VAX(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(pg66Dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n")
}

// pg66Console is a console with paging on, VMS's two images of the probe
// copied to an RD54 volume on DUA1, which holds the page file too and is
// the default directory. PFCDEFAULT is 16, as on the VAX.
func pg66Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "20000")
	setSetting(t, "vax.sysgen.pfcdefault", "16")

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "pg66.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PG66`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, image := range []string{"PG66", "PG66C"} {
		src, err := filepath.Abs(filepath.Join(pg66Dir, "vax", strings.ToLower(image)+".exe"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST/BINARY DUA1:[000000]` + image + ".EXE")
	}

	return c, dispatch
}

// pg66RunA is PG66.COM's RUNA: part A from image, with the counts before
// RUN in the logical names the program reads. It returns the program's
// lines, and the procedure's line for the status and charges across the
// RUN.
func pg66RunA(t *testing.T, dispatch func(string) string, image, id string) ([]string, string) {
	t.Helper()

	dispatch("DEFINE PG66_PART A")
	dispatch("DEFINE PG66_IMAGE " + image + ".EXE")
	dispatch(`f0 = F$GETJPI("","PAGEFLTS")`)
	dispatch(`c0 = F$GETJPI("","PAGFILCNT")`)
	dispatch(`g0 = F$GETJPI("","PPGCNT")`)
	dispatch("DEFINE PG66_FLTS 'f0'")
	dispatch("DEFINE PG66_CNT 'c0'")
	dispatch("DEFINE PG66_PPG 'g0'")

	lines := strings.Split(dispatch("RUN "+image), "|")

	dispatch(`s = $STATUS`)
	dispatch(`c1 = F$GETJPI("","PAGFILCNT")`)

	return lines, dispatch(`WRITE SYS$OUTPUT "` + id + ` ` + image +
		` around RUN: status=", s, " charged=", c0 - c1`)
}

// pg66Compare matches got with VMS's output file name, line for line but
// for the expected differences.
func pg66Compare(t *testing.T, name string, got []string) {
	t.Helper()

	want := pg66VAX(t, name)
	if len(got) != len(want) {
		t.Errorf("%s: %d lines, VMS %d\n%s", name, len(got), len(want), strings.Join(got, "\n"))

		return
	}

	differ := 0

	for i := range want {
		if got[i] == want[i] {
			continue
		}

		differ++

		if pg66Expected(want[i]) == "" {
			t.Errorf("%s line %d:\n got %s\nwant %s", name, i+1, got[i], want[i])
		}
	}

	t.Logf("%s: %d lines, %d of them listed differences", name, len(want), differ)
}

// TestPG66Oracle runs VMS's images as PG66.COM did: part A from PG66.EXE
// twice and from PG66C.EXE, then W, then D in a subprocess from a copy of
// PG66.EXE deleted while it runs; and matches VMS V7.1's lines
// (testdata/pg66/vax) but for the expected differences. As on VMS, every
// part A run gives its charges back at its exit, and the copy's file is
// gone by name once deleted, its image still reading its pages.
func TestPG66Oracle(t *testing.T) {
	c, dispatch := pg66Console(t)

	// The test console's spin loop and counter (scheduledConsole) are in
	// process 1's P0, their pages charged, and what the console ran before the first
	// image expanded process 1's user stack (Phase 67's subtask 3), its
	// pages charged; the first image's activation deletes P0 and its
	// rundown the stack, giving them back, so R1 ends as many pages below
	// where it began. VMS's P0 is empty before the first image, and its
	// DCL runs on its own stack, so its R1 is 0 like the others.
	env := c.ProcessOne()

	harness := 0

	for va := uint32(codeAddr); va < codeAddr+programExtent(brbSelf); va += 512 {
		if env.PageFileChargedAt(va) {
			harness++
		}
	}

	for va := env.FreeP1VA() + 512; va < corevms.UserStackTop; va += 512 {
		if env.PageFileChargedAt(va) {
			harness++
		}
	}

	for _, run := range []struct{ image, id string }{{"PG66", "R1"}, {"PG66", "R2"}, {"PG66C", "R3"}} {
		lines, across := pg66RunA(t, dispatch, run.image, run.id)
		pg66Compare(t, "pg66"+strings.ToLower(run.id)+".out", lines)

		want := run.id + " " + run.image + " around RUN: status=%X00000001 charged=" + strconv.Itoa(-harness)
		if across != want {
			t.Errorf("%q, want %q (VMS's, but for the test console's page)", across, want)
		}

		harness = 0
	}

	dispatch("DEFINE PG66_PART W")
	pg66Compare(t, "pg66w.out", strings.Split(dispatch("RUN PG66"), "|"))

	dispatch("COPY PG66.EXE PG66D.EXE")
	dispatch("DEFINE PG66_PART D")
	dispatch("SPAWN/NOWAIT/PROCESS=PG66DEL/OUTPUT=PG66D.OUT RUN PG66D")

	await := func(file string) {
		t.Helper()

		for range 60 {
			if dispatch(`WRITE SYS$OUTPUT F$SEARCH("`+file+`")`) != "" {
				return
			}

			dispatch("WAIT 00:00:01")
		}

		t.Fatalf("the image never made %s", file)
	}

	await("PG66D.RDY")

	if out := dispatch("DELETE PG66D.EXE;*"); out != "" {
		t.Errorf("DELETE of the running image's file: %s", out)
	}

	if out := dispatch(`WRITE SYS$OUTPUT F$SEARCH("PG66D.EXE")`); out != "" {
		t.Errorf("after DELETE, F$SEARCH finds %s (VMS's found nothing)", out)
	}

	await("PG66D.END")
	dispatch("WAIT 00:00:02")
	pg66Compare(t, "pg66d.out", strings.Split(dispatch("TYPE PG66D.OUT"), "|"))
}
