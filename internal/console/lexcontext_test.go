package console_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
)

// TestLexicalContext_subprocess: F$CONTEXT selects among several
// processes (Phase 51): a subprocess by its owner's PID and by its name,
// and process 1 alone by NEQ.
func TestLexicalContext_subprocess(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	forever := buildImage(t, c, "forever", strings.Replace(spinSource, "COUNT", "0", 1))

	proc := filepath.Join(t.TempDir(), "spin.com")
	if err := os.WriteFile(proc, []byte(`$ RUN "`+forever+`"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch(`SPAWN/NOWAIT/PROCESS=SPIN @"` + proc + `"`); err != nil {
		t.Fatal(err)
	}

	procs := c.RTL.Processes()
	if len(procs) != 2 {
		t.Fatalf("%d processes, want 2", len(procs))
	}

	one, spin := fmt.Sprintf("%08X", procs[0].Process.PID), fmt.Sprintf("%08X", procs[1].Process.PID)

	lines := []string{
		`A = ""`,
		`X = F$CONTEXT("PROCESS", A, "OWNER", "` + one + `", "EQL")`,
		`B = ""`,
		`X = F$CONTEXT("PROCESS", B, "PRCNAM", "SP*,NOBODY", "EQL")`,
		`X = F$CONTEXT("PROCESS", C, "PRCNAM", "SPIN", "NEQ")`,
		`X = F$CONTEXT("PROCESS", D, "PRCCNT", 1, "GEQ")`,
		`WRITE SYS$OUTPUT F$PID(A), "/", F$PID(A), "/", F$PID(B), "/", F$PID(C), "/", F$PID(C), "/", F$PID(D), "/", F$PID(D)`,
	}

	out.Reset()

	for _, line := range lines {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	want := spin + "//" + spin + "/" + one + "//" + one + "/"
	if got := strings.Join(programLines(out.String()), "|"); got != want {
		t.Errorf("PIDs %q, want %q", got, want)
	}

	if err := d.Dispatch("STOP SPIN"); err != nil {
		t.Fatalf("STOP SPIN: %v", err)
	}
}
