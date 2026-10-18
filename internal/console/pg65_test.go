package console_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/machine"
)

// Phase 65's probe (testdata/pg65, README.md): the program's parts, run
// on govax before VMS's run comes back.

// pg65Dir is the probe's directory.
var pg65Dir = filepath.Join("..", "..", "testdata", "pg65")

// pg65Console is a console with paging on, the probe's program built on
// an RD54 volume on DUA1, which holds the page file too and is the
// default directory, and PG65.DAT made (part M).
func pg65Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "20000")
	setSetting(t, "vax.sysgen.gblsections", "520")
	setSetting(t, "vax.sysgen.gblpages", "50000")
	setSetting(t, "vax.sysgen.gblpagfil", "4096")
	setSetting(t, "vax.sysgen.pfcdefault", "16")
	setSetting(t, "vax.sysgen.mpw_hilimit", "5634")
	setSetting(t, "vax.sysgen.mpw_lolimit", "120")
	setSetting(t, machine.WriteClusterSetting, "120")

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "pg65.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PG65`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)

	src, err := filepath.Abs(filepath.Join(pg65Dir, "pg65.mar"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST DUA1:[000000]PG65.MAR`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, cmd := range []string{"MACRO PG65", "LINK PG65"} {
		if out := dispatch(cmd); out != "" {
			t.Fatalf("%s: %s", cmd, out)
		}
	}

	return c, dispatch
}

// expectedPG65Differences are the program's lines (by their first words)
// that differ from VMS's, and why.
var expectedPG65Differences = map[string]string{
	"G1 in use:":                            "the VAX's other global sections (installed images, DECwindows): govax has none",
	"F1 made:":                              "VMS's process page table pages are in the working set and fault in as P0 grows; govax's aren't paged (Phase 62, Decision 13; Phase 67)",
	"F2 read every page:":                   "as F1 made, with VMS's DIRIO of 2 (a page table page's read, unconfirmed)",
	"R1 mapped:":                            "as F1 made",
	"R2 read every page:":                   "as F2",
	"C1 mapped:":                            "as F1 made",
	"C2 read every page:":                   "as F2",
	"S1 mapped:":                            "as F1 made",
	"S2 purged:":                            "as F1 made: the purge takes page table pages out too",
	"S4 read again:":                        "as F1 made",
	"K1 flags 00020000 pfc=0 up:":           "as F1 made: the first section of part K grows P0's page table",
	"K1 flags 00020000 pfc=64 up: flts=206": "at a small working set VMS's list is full (its page table pages and others), so no entry is free for a cluster; govax's has free entries (pg63: no permanent part)",
	"D1 expreg":                             "the pages written by then: VMS's writer is asynchronous and its working set has fewer dynamic pages (pg64's W3)",
	"P2 read every page:":                   "the working set at the floor plus 40: VMS's floor is above its list's permanent part (pg63), and its page table pages",
	"P3 purged:":                            "as P2",
	"P4 read again:":                        "as P2",
	"P5 100 other pages":                    "as P2",
	"P6 read again:":                        "as P2",
}

// pg65Expected returns the reason line is expected to differ, or "".
func pg65Expected(line string) string {
	for key, why := range expectedPG65Differences {
		if strings.HasPrefix(line, key) {
			return why
		}
	}

	return ""
}

// pg65VAX reads one of VMS's output files.
func pg65VAX(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(pg65Dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n")
}

// TestPG65Oracle runs every part, the holder (part H) in a subprocess as
// PG65.COM runs it, with the VAX's SYSGEN values, and matches VMS
// V7.1's lines (testdata/pg65/vax) but for the expected differences.
func TestPG65Oracle(t *testing.T) {
	_, dispatch := pg65Console(t)

	run := func(part string) []string {
		t.Helper()

		dispatch("DEFINE PG65_PART " + part)

		return strings.Split(dispatch("RUN PG65"), "|")
	}

	order := []string{"M", "G", "F", "D", "R", "P", "C", "U", "K", "S", "H"}
	parts := map[string][]string{}

	for _, part := range order[:9] {
		parts[part] = run(part)
	}

	dispatch("DEFINE PG65_PART H")
	dispatch("SPAWN/NOWAIT/PROCESS=PG65HLD/OUTPUT=PG65H.OUT RUN PG65")
	parts["S"] = run("S")
	dispatch("WAIT 00:00:01")
	parts["H"] = strings.Split(dispatch("TYPE PG65H.OUT"), "|")

	for _, part := range order {
		got := parts[part]
		want := pg65VAX(t, "pg65"+strings.ToLower(part)+".out")

		if len(got) != len(want) {
			t.Errorf("part %s: %d lines, VMS %d\n%s", part, len(got), len(want), strings.Join(got, "\n"))

			continue
		}

		differ := 0

		for i := range want {
			if got[i] == want[i] {
				continue
			}

			differ++

			if pg65Expected(want[i]) == "" {
				t.Errorf("part %s line %d:\n got %s\nwant %s", part, i+1, got[i], want[i])
			}
		}

		t.Logf("part %s: %d lines, %d of them listed differences", part, len(want), differ)
	}
}

// TestGetSYIGlobalSectionItems: F$GETSYI's global section items, and
// PFCDEFAULT, are $GETSYI's (the probe's procedure asks for them).
func TestGetSYIGlobalSectionItems(t *testing.T) {
	c, dispatch := wsDispatcher(t)

	for _, key := range []string{
		"PFCDEFAULT", "PAGTBLPFC", "GBLSECTIONS", "GBLPAGES", "GBLPAGFIL",
		"FREE_GBLSECTS", "FREE_GBLPAGES", "CONTIG_GBLPAGES",
	} {
		data, ok := c.ProcessOne().SYIItem("SYI$_" + key)
		if !ok || len(data) != 4 {
			t.Fatalf("$GETSYI %s: % x (%v)", key, data, ok)
		}

		want := strconv.Itoa(int(binary.LittleEndian.Uint32([]byte(data))))
		if got := dispatch(`WRITE SYS$OUTPUT F$GETSYI("` + key + `")`); got != want {
			t.Errorf("F$GETSYI(%q) = %q, want %s", key, got, want)
		}
	}
}
