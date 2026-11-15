package console_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// runPriv94 builds testdata/priv94's programs (names, without .MAR) on a
// fresh volume named PRIV94$DISK, as the VAX's exchange volume is, then runs
// the first of them once for each set of parts ("" for the default), and
// returns the runs' output. MACRO and LINK must say nothing.
func runPriv94(t *testing.T, programs []string, runs []string) string {
	t.Helper()

	var out strings.Builder

	runPriv94Each(t, programs, runs, nil, func(s string) { out.WriteString(s) })

	return out.String()
}

// runPriv94Each is runPriv94 calling before (if not nil) ahead of each run,
// with the run's index and the console, and each with the run's output.
func runPriv94Each(t *testing.T, programs []string, runs []string,
	before func(int, *console.Console), each func(string),
) {
	t.Helper()

	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.ReplaceAll(buf.String(), "\r", "")
	}

	buildPriv94(t, dispatch, programs)

	for i, parts := range runs {
		if before != nil {
			before(i, c)
		}

		if parts == "" {
			dispatch("DEASSIGN PRIV94_PART")
		} else {
			dispatch(`DEFINE PRIV94_PART "` + parts + `"`)
		}

		each(dispatch("RUN " + programs[0]))
	}
}

// buildPriv94 makes the probe's volume as PRIV94.COM does: a container
// mounted on DUA1 and named PRIV94$DISK, the default directory, and the
// programs assembled, linked, and protected so the accounts can run them.
func buildPriv94(t *testing.T, dispatch func(string) string, programs []string) {
	t.Helper()

	dsk := filepath.Join(t.TempDir(), "priv94.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 PRIV94`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("DEFINE/SYSTEM/EXECUTIVE_MODE PRIV94$DISK DUA1:")
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range programs {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "priv94", strings.ToLower(name)+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + name + ".MAR/QUIET")

		if out := dispatch("MACRO/NOLIST " + name); out != "" {
			t.Fatalf("MACRO %s: %s", name, out)
		}

		if out := dispatch("LINK/NOMAP " + name); out != "" {
			t.Fatalf("LINK %s: %s", name, out)
		}

		// As PRIV94.COM does, so the accounts can run the images.
		if out := dispatch("SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) " + name + ".EXE"); out != "" {
			t.Fatalf("SET SECURITY %s: %s", name, out)
		}
	}
}

// checkPriv94Run fails the test for each of cases 1 to n that didn't run,
// each wanted line missing, and each condition that reached a case's handler
// but the one allowed (a status's hex, or "" for none).
func checkPriv94Run(t *testing.T, text string, n int, want []string, allowed string) {
	t.Helper()

	ran := map[int]int{}

	for _, line := range strings.Split(text, "\n") {
		var k int
		if _, err := fmt.Sscanf(line, "== %d ", &k); err == nil {
			ran[k]++
		}
	}

	for k := 1; k <= n; k++ {
		if ran[k] == 0 {
			t.Errorf("case %d didn't run", k)
		}
	}

	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("no %q in the run's output", w)
		}
	}

	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "  signal ") && (allowed == "" || !strings.HasPrefix(line, "  signal "+allowed+" ")) {
			t.Errorf("a condition reached a case's handler: %s", line)
		}
	}
}

// TestPriv94Probe rehearses testdata/priv94's round 1 program on govax, so
// that the probe the author runs on the VAX can't lose a round to a mistake
// of ours (docs/planning/PHASE-94 - privileges.md, subtask 2): the three
// programs must assemble and link with no message, and the program must run
// every case: the fixtures and holders (parts F and H), every default part,
// parts K and R, and the end (parts H and Z), with no condition reaching a
// case's handler. TestPriv94Oracle holds the cases' answers to VMS's.
func TestPriv94Probe(t *testing.T) {
	text := runPriv94(t, []string{"PRIV94", "PRIV94H", "PRIV94I"}, []string{"FH", "", "K", "R", "HZ"})

	checkPriv94Run(t, text, 289, []string{
		"== 1 F fixtures",
		"  PRIV94_PMBX 00000001",
		"== 5 L user [SYSNAM] $CRELNM LNM$SYSTEM_TABLE user-mode name",
		"  lnm PRIV94 mode 3 in LNM$SYSTEM_TABLE",
		"== 289 X user [CMEXEC] $CMEXEC of a routine that returns 1",
		"== 3 Z the end",
		"end of cases",
	}, "")
}

// TestPriv94BProbe rehearses round 2's program, PRIV94B, the same way: its
// fixtures, every default part, and its end.
func TestPriv94BProbe(t *testing.T) {
	text := runPriv94(t, []string{"PRIV94B"}, []string{"F", "", "Z"})

	checkPriv94Run(t, text, 40, []string{
		"== 1 F fixtures",
		"== 3 M user [PRMMBX] $CREMBX prmflg 1 with no logical name",
		"== 17 U user [ALL] $ULKPAG of a page it locked",
		"  at 0: ",
		"== 2 Z the end",
		"  PRIV94_L21 ",
		"end of cases",
	}, "")
}
