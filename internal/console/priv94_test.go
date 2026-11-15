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

// TestPriv94Probe rehearses testdata/priv94's program on govax, so that the
// probe the author runs on the VAX can't lose a round to a mistake of ours
// (docs/planning/PHASE-94 - privileges.md, subtask 2): the three programs
// must assemble and link with no message, and the program must run every
// case: the fixtures and holders (parts F and H), every default part, and
// parts K and R, and the end (parts H and Z), each case's line printed, with no condition
// reaching a case's handler but $SETSWM's, a service govax doesn't have yet.
func TestPriv94Probe(t *testing.T) {
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

	dsk := filepath.Join(t.TempDir(), "priv94.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 PRIV94`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("DEFINE/SYSTEM/EXECUTIVE_MODE PRIV94$DISK DUA1:")
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range []string{"PRIV94", "PRIV94H", "PRIV94I"} {
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
	}

	var out strings.Builder

	for _, parts := range []string{"FH", "", "K", "R", "HZ"} {
		if parts == "" {
			dispatch("DEASSIGN PRIV94_PART")
		} else {
			dispatch(`DEFINE PRIV94_PART "` + parts + `"`)
		}

		out.WriteString(dispatch("RUN PRIV94"))
	}

	text := out.String()

	// Every case ran, H twice.
	ran := map[int]int{}

	for _, line := range strings.Split(text, "\n") {
		var n int
		if _, err := fmt.Sscanf(line, "== %d ", &n); err == nil {
			ran[n]++
		}
	}

	for n := 1; n <= 289; n++ {
		if ran[n] == 0 {
			t.Errorf("case %d didn't run", n)
		}
	}

	for _, want := range []string{
		"== 1 F fixtures",
		"  PRIV94_PMBX 00000001",
		"== 5 L user [SYSNAM] $CRELNM LNM$SYSTEM_TABLE user-mode name",
		"  lnm PRIV94 mode 3 in LNM$SYSTEM_TABLE",
		"== 289 X user [CMEXEC] $CMEXEC of a routine that returns 1",
		"== 3 Z the end",
		"end of cases",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in the run's output", want)
		}
	}

	// $SETSWM is the one service the probe calls that govax lacks; its
	// cases signal (subtask 4 adds it). Nothing else may.
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "  signal ") && !strings.HasPrefix(line, "  signal 00000E4A ") {
			t.Errorf("a condition reached a case's handler: %s", line)
		}
	}
}
