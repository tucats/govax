package console_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/machine"
)

// Phase 64's probe (testdata/pg64, README.md): govax runs the program's
// parts as VMS V7.1 ran them and must answer as VMS did but for the
// differences listed here.

// pg64Dir is the probe's directory.
var pg64Dir = filepath.Join("..", "..", "testdata", "pg64")

// expectedPG64Differences are the program's lines (by their first words)
// that differ from VMS's, and why.
var expectedPG64Differences = map[string]string{
	"Q1 start:":            "the quota and what's left of it at the first instruction: VMS's DCL and image activation charge far more (P1's control region, LIBRTL's and DCL's pages)",
	"Q1 the job's charges": "as Q1 start",
	"W1 pages=":            "the working set limit: VMS's floor is MINWSCNT above the list's permanent part (testdata/pg63)",
	"W3":                   "the pages written by then: VMS's writer is asynchronous (MPW_IOLIMIT writes in flight) and its working set has fewer dynamic pages, so its count is a cluster or two away from govax's (round 2, TestPG64WriterRound2)",
	"W4":                   "as W3",
	"W5":                   "as W3",
	"W6 deltva:":           "the blocks given back are those written, as W3",
	"Q11 expreg P1":        "the detached run's P1 ends before its quota does (pg64P1End)",
	"V1":                   "the heap's charges: VMS's first LIB$GET_VM charges 136 pages, govax's 129; the second 401 and 396 (the heap's layout, Phase 59)",
}

// pg64QuotaFillDifference is the line govax's detached run adds: its P1
// (8192 pages in all, the user stack's reserve above) runs out before the
// quota does, where VMS's (VIRTUALPAGECNT 65500) doesn't, so the fill
// goes on in P0 after a "Q10 fill: region 1 stopped" line VMS doesn't
// write.
const pg64QuotaFillDifference = "Q10 fill: region 1 stopped:"

// pg64P1End is the start of the detached run's Q11 line for P1 when P1's
// page table ends before the quota does. Since $DELTVA gives P1's pages
// back (Phase 67's created space; Q5's 100 pages were never given back
// before), the fill takes P1 to within 72 pages of its table's end, so
// Q11's request for 210 meets the end of the address space (SS$_VASFULL,
// retadr -1) where VMS's meets the quota, and the program writes no
// "Q11 deltva" line after it. Until each process's P0 and P1 share one
// table area (Phase 67's subtask 4), as VMS's VIRTUALPAGECNT does.
const pg64P1End = "Q11 expreg P1 pagfilcnt+10: 00000244 retadr -1"

// withoutP1End returns VMS's lines want without the "Q11 deltva" line
// after its Q11 line for P1, if govax's lines got have pg64P1End there
// instead.
func withoutP1End(got, want []string) []string {
	if !slices.ContainsFunc(got, func(l string) bool { return strings.HasPrefix(l, pg64P1End) }) {
		return want
	}

	for i, line := range want {
		if strings.HasPrefix(line, "Q11 expreg P1") && i+1 < len(want) && strings.HasPrefix(want[i+1], "Q11 deltva") {
			return slices.Delete(slices.Clone(want), i+1, i+2)
		}
	}

	return want
}

// pg64Expected returns the reason line is expected to differ, or "".
func pg64Expected(line string) string {
	for key, why := range expectedPG64Differences {
		if strings.HasPrefix(line, key) {
			return why
		}
	}

	return ""
}

// pg64Console is a console with paging on, SYSGEN's writer parameters as
// the VAX had them, and the probe's files on an RD54 volume on DUA1,
// which holds the page file too and is the default directory. The
// programs are built there.
func pg64Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.mpw_hilimit", "5634")
	setSetting(t, "vax.sysgen.mpw_lolimit", "120")
	setSetting(t, machine.WriteClusterSetting, "120")
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "20000")

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "pg64.dsk")

	for _, cmd := range []string{
		`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PG64`,
		`MOUNT/WRITE DUA1 "` + dsk + `"`,
	} {
		dispatch(cmd)
	}

	for _, f := range []string{"pg64.mar", "pg64v.mar"} {
		src, err := filepath.Abs(filepath.Join(pg64Dir, f))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(f))
	}

	dispatch("SET DEFAULT DUA1:[000000]")

	for _, cmd := range []string{"MACRO PG64", "LINK PG64", "MACRO PG64V", "LINK PG64V"} {
		if out := dispatch(cmd); out != "" {
			t.Fatalf("%s: %s", cmd, out)
		}
	}

	return c, dispatch
}

// pg64VAX reads one of VMS's output files.
func pg64VAX(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(pg64Dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n")
}

// comparePG64 matches got with VMS's lines, but for the expected
// differences.
func comparePG64(t *testing.T, name string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: %d lines, VMS %d\n%s", name, len(got), len(want), strings.Join(got, "\n"))
	}

	differ := 0

	for i := range want {
		if got[i] == want[i] {
			continue
		}

		differ++

		if pg64Expected(want[i]) == "" {
			t.Errorf("%s line %d:\n got %s\nwant %s", name, i+1, got[i], want[i])
		}
	}

	t.Logf("%s: %d lines, %d of them listed differences", name, len(want), differ)
}

// pagfilcnt is process 1's JPI$_PAGFILCNT, through F$GETJPI.
func pagfilcnt(t *testing.T, dispatch func(string) string) int {
	t.Helper()

	out := dispatch(`WRITE SYS$OUTPUT F$GETJPI("","PAGFILCNT")`)

	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("PAGFILCNT: %q", out)
	}

	return n
}

// TestPG64Oracle runs the program's parts and compares their lines with
// VMS's, and checks that each image's exit gives its charges back, as
// the VAX's "pf" lines show (PAGFILCNT 39799 after every run).
func TestPG64Oracle(t *testing.T) {
	_, dispatch := pg64Console(t)

	run := func(part, image string) []string {
		t.Helper()

		dispatch("DEFINE PG64_PART " + part)

		before := pagfilcnt(t, dispatch)
		lines := strings.Split(dispatch("RUN "+image), "|")

		if after := pagfilcnt(t, dispatch); after != before {
			t.Errorf("part %s: PAGFILCNT %d after the image, %d before", part, after, before)
		}

		return lines
	}

	// Part T first: a subprocess's pages on VMS (pg64l.log's T1 line).
	// Its image exit also gives back the one P0 page process 1 had
	// charged before any image ran (the console's own use of P0, which
	// VMS's DCL doesn't have), so the runs after it start from PAGFILCNT
	// as an image leaves it.
	dispatch("DEFINE PG64_PART T")

	if t1 := strings.Split(dispatch("RUN PG64"), "|"); len(t1) != 1 || t1[0] != "T1 expreg 300, touched: charged=300" {
		t.Errorf("T: %q", t1)
	}

	// Part Q here: SYSTEM's quota (40960) is more than govax's address
	// space, so from the fill on its lines are compared in the detached
	// runs, which reach the quota as VMS's process did.
	q1, want := run("Q", "PG64"), pg64VAX(t, "pg64q1.out")
	fill := 0

	for i, line := range want {
		if strings.HasPrefix(line, "Q10") {
			fill = i

			break
		}
	}

	comparePG64(t, "Q (to the fill)", q1[:fill], want[:fill])

	// Part C: part Q in a detached process with a small quota, with
	// EXQUOTA and without; the names have the node's prefix, as F$GETDVI's
	// FULLDEVNAM gives them on VMS.
	for _, name := range []string{"IMAGE", "OUT2", "OUT3"} {
		file := map[string]string{"IMAGE": "PG64.EXE", "OUT2": "PG64Q2.OUT", "OUT3": "PG64Q3.OUT"}[name]
		dispatch("DEFINE PG64_" + name + " _GOVAX$DUA1:[000000]" + file)
	}

	comparePG64(t, "C", run("C", "PG64"), pg64VAX(t, "pg64c.out"))

	for _, f := range []string{"pg64q2.out", "pg64q3.out"} {
		var got []string

		for _, line := range strings.Split(dispatch("TYPE "+strings.ToUpper(f)), "|") {
			if !strings.HasPrefix(line, pg64QuotaFillDifference) {
				got = append(got, line)
			}
		}

		comparePG64(t, f, got, withoutP1End(got, pg64VAX(t, f)))
	}

	// LIB$GET_VM: its status past the quota is LIB$_INSVIRMEM.
	v := run("V", "PG64V")
	comparePG64(t, "V", v[:2], pg64VAX(t, "pg64v.out")[:2])

	if len(v) != 3 || !strings.HasPrefix(v[2], "V2 get_vm pagfilcnt+10 pages: 00158214 ") {
		t.Errorf("V2: %q, want LIB$_INSVIRMEM", v)
	}

	// The writer.
	comparePG64(t, "W", run("W", "PG64"), pg64VAX(t, "pg64w.out"))

	// The job's pool, from a waiting subprocess.
	dispatch("SPAWN/NOWAIT/PROCESS=PG64SUB/OUTPUT=NL: WAIT 00:02:00")
	comparePG64(t, "S", run("S", "PG64"), pg64VAX(t, "pg64s.out"))
	dispatch("STOP PG64SUB")
}

// pg64Used is the page file's use on one of part X's lines (X3, X4, X5,
// X6), and the line's kind and count of pages swept, with the time of
// day taken out: "X3 16:58:15.82 written 6000: page file used=479
// flts=6226" is ("X3 written 6000", 479).
func pg64Used(line string) (string, int, bool) {
	f := strings.Fields(line)
	if len(f) < 3 || !strings.HasPrefix(f[0], "X") || f[0] < "X3" || f[0] > "X6" {
		return "", 0, false
	}

	i := strings.Index(line, "used=")
	if i < 0 {
		return "", 0, false
	}

	n, err := strconv.Atoi(strings.Fields(line[i+len("used="):])[0])
	if err != nil {
		return "", 0, false
	}

	return f[0] + " " + strings.SplitN(strings.Join(f[2:], " "), ":", 2)[0], n, true
}

// TestPG64WriterRound2 runs part X, round 2's timed sweeps, and holds the
// writer to VMS 7.1's (vax/round2): woken by the modified list passing
// MPW_HILIMIT, it writes the list down a cluster below it and no
// further, so the page file's use after each chunk of the sweeps, and
// through the rest, is VMS's within two clusters (VMS writes
// asynchronously, and its working set has fewer dynamic pages).
func TestPG64WriterRound2(t *testing.T) {
	_, dispatch := pg64Console(t)

	dispatch("DEFINE PG64_PART X")
	got := strings.Split(dispatch("RUN PG64"), "|")

	data, err := os.ReadFile(filepath.Join(pg64Dir, "vax", "round2", "pg64x.out"))
	if err != nil {
		t.Fatal(err)
	}

	want := strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("%d lines, VMS %d\n%s", len(got), len(want), strings.Join(got, "\n"))
	}

	const slack = 2 * 120 // two clusters of MPW_WRTCLUSTER

	compared := 0

	for i := range want {
		wk, wn, ok := pg64Used(want[i])
		if !ok || strings.HasPrefix(wk, "X6") {
			continue // X6's use includes VMS's other processes' pages
		}

		gk, gn, ok := pg64Used(got[i])
		if !ok || gk != wk {
			t.Errorf("line %d: %q, VMS %q", i+1, got[i], want[i])

			continue
		}

		compared++

		if gn < wn-slack || gn > wn+slack {
			t.Errorf("%s: page file used=%d, VMS %d (more than two clusters apart)", wk, gn, wn)
		}
	}

	if compared == 0 {
		t.Fatal("no lines compared")
	}
}
