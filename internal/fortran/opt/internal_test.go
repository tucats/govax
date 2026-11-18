package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/ir"
)

// Internal procedures under the optimizer (docs/planning/PHASE-103 - Fortran 90
// procedures and interfaces.md, finding 14): a host's variable that an internal
// procedure reaches is changed by any call in the host, and in an internal
// procedure a host's variable is changed by any call, as COMMON is.

// hostProgram is a subroutine S whose variable K its internal procedure BUMP
// changes, and whose internal procedure USE sets K, calls BUMP, and reads K.
var hostProgram = []string{
	"SUBROUTINE S(N)",
	"INTEGER N, K, J, M",
	"K = 5",
	"CALL BUMP",
	"J = K + 1",
	"CALL USE",
	"N = J + M",
	"CONTAINS",
	"SUBROUTINE BUMP",
	"K = K + 1",
	"END SUBROUTINE",
	"SUBROUTINE USE",
	"K = 1",
	"CALL BUMP",
	"M = K",
	"END SUBROUTINE",
	"END",
}

// optimizedMacro is the MACRO of program's routines (hostProgram's if none), lowered once, each with
// every pass run on it, written together as one module writes them: an internal
// procedure names its host's cells, so the host must be the one lowered with it.
func optimizedMacro(t *testing.T, program ...string) string {
	t.Helper()

	if program == nil {
		program = hostProgram
	}

	lines := make([]string, len(program))
	for i, line := range program {
		lines[i] = "      " + line
	}

	routines, ok := lowered("TEST", lines)
	if !ok {
		t.Fatal("the program doesn't compile")
	}

	for _, r := range routines {
		run([]*ir.Routine{r}, Passes)

		if err := ir.Check(r); err != nil {
			t.Fatal(err)
		}
	}

	var w codegen.Writer

	codegen.Generate(&w, routines, nil, codegen.Options{Improve: Improve})

	return strings.Join(w.Lines(), "\n")
}

// TestHostVariableChangedByCall checks that the host reads K again after calling
// BUMP, rather than forwarding the 5 it stored before the call, and that K's cell
// says an internal procedure reaches it.
func TestHostVariableChangedByCall(t *testing.T) {
	r := routineOf(t, "S", hostProgram...)

	for _, c := range r.Cells {
		if c.Name == "K" && !c.Reached {
			t.Error("K's cell isn't marked as reached")
		}

		if c.Name == "J" && c.Reached {
			t.Error("J's cell is marked as reached")
		}
	}

	text := optimizedMacro(t)

	if strings.Contains(text, "#6") || !strings.Contains(text, "ADDL3\tS.K,#1,S.J") {
		t.Errorf("K's 5 is forwarded past the call of BUMP:\n%s", text)
	}
}

// TestHostVariableInInternal checks that USE reads its host's K after calling
// BUMP, a sibling that changes it, by the host's name for it.
func TestHostVariableInInternal(t *testing.T) {
	text := optimizedMacro(t)
	use := text[strings.Index(text, "S.USE:"):]

	if !strings.Contains(use, "CALLS\t#0,S.BUMP") || !strings.Contains(use, "MOVL\tS.K,S.M") {
		t.Errorf("USE doesn't read its host's K after calling BUMP:\n%s", use)
	}
}

// TestHostFrameChangedByCall checks that a RECURSIVE host reads its frame variable
// MINE again after calling ADD, which changes it through the host's FP, and that
// ADD reaches MINE and N's cell from the register holding that FP.
func TestHostFrameChangedByCall(t *testing.T) {
	text := optimizedMacro(t,
		"RECURSIVE SUBROUTINE S(N)",
		"INTEGER N, MINE",
		"MINE = 5",
		"CALL ADD",
		"N = MINE",
		"CONTAINS",
		"SUBROUTINE ADD",
		"MINE = MINE + N",
		"END SUBROUTINE",
		"END")

	if !strings.Contains(text, "CALLS\t#1,S.ADD") || !strings.Contains(text, "MOVL\tS.MINE(FP),@S.$A1(FP)") {
		t.Errorf("S doesn't pass its FP, or doesn't read MINE after the call:\n%s", text)
	}

	if !strings.Contains(text, "ADDL2\t@S.$A1(R10),S.MINE(R10)") {
		t.Errorf("ADD doesn't reach its host's frame from R10:\n%s", text)
	}
}
