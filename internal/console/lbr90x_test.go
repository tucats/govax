package console

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// lbr90xCases is how many cases LBR90X.MAR has (round 3b, subtask 11d).
const lbr90xCases = 17

// lbr90xToFirstKey are the cases held to VMS's output only up to their
// first LBR$INSERT_KEY: VMS 7.1's LBRSHR didn't do with binary keys what
// the manual says. Given the address of the key's longword, as the manual
// says (X1, X2, X3, X5), its first INSERT_KEY failed with an access
// violation at address 4; given a descriptor (X4), it took for the key the
// descriptor's length word and two bytes of whatever was on its stack
// (7FED0002 for K5, K1, and K3 alike, so the second and third were
// DUPKEY, and DELETE_KEY of K1 later found nothing). govax does what the
// manual says.
var lbr90xToFirstKey = map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}

// lbr90xLibraries are the libraries round 3b made that are held to VMS's
// (vax/round3b/h90x.bck) byte for byte but for times and the librarian's
// name: those of the preallocation cases, and WX.
var lbr90xLibraries = []string{"X6.TLB;1", "X7.TLB;1", "X8.TLB;1", "X9.TLB;1", "X10.TLB;1", "WX.TLB;1"}

// lbr90xTimes are the lines with times that depend on the run: the time of
// day, and how long an open waited (VMS's 76 seconds for another process's
// hold, govax's as long as the holder's emulated wait; VMS's 120 for the
// image's own, which govax refuses at once).
var lbr90xTimes = regexp.MustCompile(`^  (now|elapsed) .*`)

// TestLBR90XOracle runs round 3b (testdata/help90's LBR90X.MAR) on govax, in
// both paging modes, and holds it to VMS 7.1's run (vax/round3b/): every
// case as HELP90X.COM runs it, X12 in a subprocess holding WX while X13,
// then X14, waits to open it, then the preallocation cases' libraries and
// WX, block for block.
func TestLBR90XOracle(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "help90")
	logPath := filepath.Join(dir, "vax", "round3b", "help90x.log")
	want := lbrVMSCases(t, logPath, "$ LBR90X ", lbr90xCases-1)
	want[12] = lbr90xHolderLog(t, logPath)
	vmsLibraries := lbr90wSaveSet(t, filepath.Join(dir, "vax", "round3b", "h90x.bck"))

	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)
		mountFreshContainerSized(t, c, "DUA3:", 20000)

		src, err := filepath.Abs(filepath.Join(dir, "lbr90x.mar"))
		if err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY "`+src+`"/HOST DUA3:[000000]LBR90X.MAR`)

		for _, cmd := range []string{
			"DEFINE H90DIR DUA3:[000000]",
			"SET DEFAULT DUA3:[000000]",
			"MACRO LBR90X",
			"LINK LBR90X",
			`LBR90X := $DUA3:[000000]LBR90X`,
		} {
			buf.Reset()
			shareFindRun(t, d, cmd)
		}

		run := func(cmd string) []string {
			buf.Reset()
			_ = d.Dispatch(cmd)

			return strings.Split(strings.TrimSuffix(strings.ReplaceAll(buf.String(), "\r\n", "\n"), "\n"), "\n")
		}

		for n := 1; n <= 11; n++ {
			lbr90xCompare(t, n, run(fmt.Sprintf("LBR90X %d", n)), want[n])
		}

		// X12 holds WX in a subprocess while X13 waits for it: the WAIT
		// lets the subprocess open WX first.
		_ = run("SPAWN/NOWAIT/OUTPUT=HOLD1.LOG LBR90X 12")
		_ = run("WAIT 00:00:01")
		lbr90xCompare(t, 13, run("LBR90X 13"), want[13])

		hold := run("TYPE HOLD1.LOG")
		lbr90xCompare(t, 12, hold, want[12])

		// And X14, opening WX to read, waits for a second holder.
		_ = run("SPAWN/NOWAIT/OUTPUT=HOLD2.LOG LBR90X 12")
		_ = run("WAIT 00:00:01")
		lbr90xCompare(t, 14, run("LBR90X 14"), want[14])
		lbr90xCompare(t, 12, run("TYPE HOLD2.LOG"), want[12])

		for n := 15; n <= lbr90xCases; n++ {
			lbr90xCompare(t, n, run(fmt.Sprintf("LBR90X %d", n)), want[n])
		}

		for _, name := range lbr90xLibraries {
			data, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: "DUA3:[000000]" + name})
			if err != nil {
				t.Errorf("%s: %v", name, err)

				continue
			}

			if dir := os.Getenv("LBR90W_DUMP"); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
			}

			lbr90wSameLibrary(t, name, data, vmsLibraries[name])
		}
	})
}

// lbr90xCompare holds case n's lines to VMS's: through the line before
// its first INSERT_KEY for lbr90xToFirstKey's cases, and with the run's
// times masked.
func lbr90xCompare(t *testing.T, n int, got, want []string) {
	t.Helper()

	if lbr90xToFirstKey[n] {
		got, want = lbr90xBeforeKeys(got), lbr90xBeforeKeys(want)
	}

	mask := func(lines []string) []string {
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = lbr90xTimes.ReplaceAllString(l, "  $1 (time)")
		}

		return out
	}

	lbr90wCompare(t, n, mask(got), mask(want))
}

// lbr90xBeforeKeys is a case's lines before its first INSERT_KEY (or
// before VMS's access violation there).
func lbr90xBeforeKeys(lines []string) []string {
	for i, l := range lines {
		if strings.HasPrefix(l, "  INSERT_KEY:") || strings.HasPrefix(l, "  =c ") {
			return lines[:i]
		}
	}

	return lines
}

// lbr90xHolderLog is X12's lines from VMS's log: HOLD1.LOG, which the
// procedure types after X13.
func lbr90xHolderLog(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var lines []string

	in := false

	for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		switch {
		case line == "$ TYPE HOLD1.LOG":
			in = true
		case strings.HasPrefix(line, "$ "):
			in = false
		case in:
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		t.Fatal("no HOLD1.LOG in VMS's log")
	}

	return lines
}
