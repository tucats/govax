package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelp90DebuggerOracle runs part F of testdata/help90 (HELP90F.COM's
// first session) on govax: H90D.HLB made by LIBRARY/HELP, DBGT built
// /DEBUG, and the debugger run on it with DBG$HELP naming the library and
// DBG$INPUT the commands of HELP90D.DBG, whose empty lines answer HELP's
// prompts. The session's log, DBGH.DLG, is held to VMS 7.1's
// (vax/round1b/dbgh.dlg) line for line: what each HELP shows, and, by the
// empty commands left after each, how many prompts it read answers for.
func TestHelp90DebuggerOracle(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "help90")

	com := strings.Join([]string{
		"$ SET NOON",
		"$ LIBRARY/CREATE/HELP H90D H90D",
		"$ MACRO/DEBUG DBGT",
		"$ LINK/DEBUG DBGT",
		"$ DEFINE/USER_MODE DBG$INPUT HELP90D.DBG",
		"$ DEFINE/USER_MODE DBG$HELP DUA1:[000000]H90D",
		"$ RUN DBGT",
		"",
	}, "\n")

	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "help90f.com"), []byte(com), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"h90d.hlp", "dbgt.mar", "help90d.dbg"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := runProbeProcedure(t, work, []string{"h90d.hlp", "dbgt.mar", "help90d.dbg", "help90f.com"},
		"HELP90F", []string{"dbgh"})

	got := readLog(t, filepath.Join(out, "dbgh.dlg"))
	want := readLog(t, filepath.Join(dir, "vax", "round1b", "dbgh.dlg"))

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("DBGH.DLG differs from VMS's:\n%s", firstDifference(strings.Join(want, "\n"), strings.Join(got, "\n")))
	}
}

