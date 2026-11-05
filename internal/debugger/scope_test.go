package debugger_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console/consoletest"
)

// TestFortranScope: in a recursive FORTRAN function compiled /DEBUG, the
// variables of an outer activation are reached through the scope: SET
// SCOPE n, SET SCOPE/CURRENT n, and a name's own scope, n\K. Stopped at
// FACT(1)'s label 10, the levels are FACT(1) to FACT(4), then the program.
func TestFortranScope(t *testing.T) {
	source, err := os.ReadFile(consoletest.RepoPath(t, "testdata", "dbg89", "dbg89f.for"))
	if err != nil {
		t.Fatal(err)
	}

	c := fortranSessionOf(t, string(source), "FACT.L10")

	dispatch(t, c, "SET RADIX DECIMAL")

	for _, step := range []struct{ line, output string }{
		{"EXAMINE N", "DBG89F\\FACT.N:  1\n"},
		{"EXAMINE K", "DBG89F\\FACT.K:  10\n"},
		{"SET SCOPE 1", ""},
		{"EXAMINE N", "DBG89F\\FACT.N:  2\n"},
		{"EXAMINE K", "DBG89F\\FACT.K:  20\n"},
		{"SET SCOPE 3", ""},
		{"EXAMINE N", "DBG89F\\FACT.N:  4\n"},
		{"EXAMINE K", "DBG89F\\FACT.K:  40\n"},
		{"CANCEL SCOPE", ""},
		{"EXAMINE K", "DBG89F\\FACT.K:  10\n"},
		{"EXAMINE 2\\K", "DBG89F\\FACT.K:  30\n"},
		{"SET SCOPE/CURRENT 2", ""},
		{"EXAMINE N", "DBG89F\\FACT.N:  3\n"},
		// A DEPOSIT goes to that activation's frame.
		{"DEPOSIT K = 33", ""},
		{"EXAMINE K", "DBG89F\\FACT.K:  33\n"},
		{"EXAMINE 0\\K", "DBG89F\\FACT.K:  10\n"},
	} {
		if got := variableOutput(t, c, step.line); got != step.output {
			t.Errorf("%s: got %q, want %q", step.line, got, step.output)
		}
	}

	// The program's own level has no activation of FACT.
	dispatch(t, c, "SET SCOPE 4")

	if _, err := sayErr(c, "EXAMINE K"); err == nil || !strings.Contains(err.Error(), "no active call frame") {
		t.Errorf("EXAMINE K in scope 4: %v, want NOTACTIVE", err)
	}
}

// TestDepositInScope: a DEPOSIT to a register of an outer call level goes
// where the level's value is kept, so the program sees it when that level
// runs again: a register a frame saved into the frame (the RET restores
// it), one nobody saved into the register itself. R0 and R1 of an outer
// level are gone, and its SP is computed, so neither takes a DEPOSIT.
// VMS's own answers are unconfirmed: the probe's DEPOSITs used ^X, which
// VMS's debugger doesn't read (testdata/dbg89).
func TestDepositInScope(t *testing.T) {
	c := stoppedAt(t, consoletest.RepoPath(t, "testdata", "dbg89", "vax", "dbg89.exe"), "INLEAF")

	for _, step := range []struct{ line, output string }{
		// Level 2 is REC(2), whose R2 REC(1)'s frame saved.
		{"SET SCOPE 2", ""},
		{"DEPOSIT R2 = 2222", ""},
		{"EXAMINE R2", "DBG89\\REC 1\\%R2:  00002222\n"},
		// R5 no routine saved: the live register.
		{"DEPOSIT R5 = 5555", ""},
		{"CANCEL SCOPE", ""},
		{"EXAMINE R2", "DBG89\\LEAF\\%R2:  00000102\n"},
		{"EXAMINE R5", "DBG89\\LEAF\\%R5:  00005555\n"},
		// LEAF and REC(1) return: REC(2) runs again with its R2 from the frame.
		{"SET BREAK BACK", ""},
		{"GO", "break at DBG89\\REC\\BACK\n"},
		{"EXAMINE R2", "DBG89\\REC\\%R2:  00002222\n"},
	} {
		if got := variableOutput(t, c, step.line); got != step.output {
			t.Errorf("%s: got %q, want %q", step.line, got, step.output)
		}
	}

	dispatch(t, c, "SET SCOPE 1")

	for _, step := range []struct{ line, says string }{
		{"DEPOSIT R0 = 1", "non-static variable 'DBG89\\REC 1\\%R0' is not active"},
		{"DEPOSIT SP = 0", "not allowed"},
		{"DEPOSIT PSL = 0", "not allowed"},
	} {
		if _, err := sayErr(c, step.line); err == nil || !strings.Contains(err.Error(), step.says) {
			t.Errorf("%s: %v, want %q", step.line, err, step.says)
		}
	}
}
