package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// Tests of labels and GOTO (dcllabel.go), IF blocks (dclif.go), and GOSUB
// and CALL (dclcall.go), from the User's Manual's rules and examples
// (13.2, 14.16, 14.17).

// runFlow runs a procedure of lines, as runStatusProcedure does, and
// returns its output.
func runFlow(t *testing.T, lines ...string) string {
	t.Helper()

	_, out, _ := runStatusProcedure(t, lines...)

	return out
}

// TestGotoLoop: a backward GOTO makes a loop (14.19), and a forward one
// passes over lines; a label may have a command after it.
func TestGotoLoop(t *testing.T) {
	out := runFlow(t,
		"$ N = 0",
		"$ LOOP:",
		"$ N = N + 1",
		"$ IF N .LT. 3 THEN GOTO LOOP",
		`$ GOTO SHOW_IT`,
		`$ PRINT "passed over"`,
		`$ SHOW_IT: PRINT "N=''N'"`)

	if out != "N=3\n" {
		t.Errorf("output %q, want N=3", out)
	}
}

// TestGotoDuplicateLabels: of duplicate labels, GOTO goes to the one DCL
// processed last, or, when none has been, to the nearest after it
// (13.2.2's three rules).
func TestGotoDuplicateLabels(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  string
	}{
		{"all follow", []string{
			"$ GOTO A",
			`$ A: PRINT "first"`,
			"$ EXIT",
			`$ A: PRINT "second"`,
		}, "first\n"},
		{"before and after", []string{
			"$ N = 0",
			`$ A: PRINT "first"`,
			"$ N = N + 1",
			"$ IF N .EQ. 2 THEN EXIT",
			"$ GOTO A",
			`$ A: PRINT "second"`,
		}, "first\nfirst\n"},
		{"all precede", []string{
			"$ N = 0",
			`$ A: PRINT "one"`,
			`$ A: PRINT "two"`,
			"$ N = N + 1",
			"$ IF N .LT. 2 THEN GOTO A",
		}, "one\ntwo\ntwo\n"},
	} {
		if out := runFlow(t, tc.lines...); out != tc.want {
			t.Errorf("%s: output %q, want %q", tc.name, out, tc.want)
		}
	}
}

// TestGotoMissingLabel: a label that isn't there is USGOTO, and the
// procedure ends, at its end of file; an ON action can still take it
// somewhere.
func TestGotoMissingLabel(t *testing.T) {
	err, out, status := runStatusProcedure(t, "$ GOTO NOWHERE", `$ PRINT "after"`)
	if !strings.Contains(out, "%DCL-W-USGOTO") || strings.Contains(out, "after") ||
		vmsCondition(vmserrors.CLI_USGOTO) != status || !vmserrors.MessageInhibited(err) {
		t.Errorf("output %q, $STATUS %08X, @ %v; want USGOTO once, and the end", out, status, err)
	}

	out = runFlow(t,
		"$ GOTO START",
		"$ HANDLER:",
		`$ PRINT "handled"`,
		"$ EXIT",
		"$ START:",
		"$ ON WARNING THEN GOTO HANDLER",
		"$ GOTO NOWHERE",
		`$ PRINT "after"`)

	if !strings.Contains(out, "USGOTO") || !strings.HasSuffix(out, "handled\n") {
		t.Errorf("with ON WARNING THEN GOTO: %q, want USGOTO, then handled", out)
	}
}

// TestGotoBlocks: a label inside an IF block the GOTO isn't in isn't a
// target (14.16.5's example), and going out of a block closes it.
func TestGotoBlocks(t *testing.T) {
	out := runFlow(t,
		"$ GOTO TEST_1",
		"$ EXIT",
		"$ IF 1.EQ.1",
		`$       THEN PRINT "What are we doing here?"`,
		"$ TEST_1:",
		`$       PRINT "Got to the label"`,
		"$ ENDIF",
		"$ EXIT")

	if !strings.Contains(out, "USGOTO") || strings.Contains(out, "Got to") {
		t.Errorf("into a block: %q, want USGOTO", out)
	}

	out = runFlow(t,
		"$ IF 1",
		"$ THEN",
		"$     GOTO OUT",
		`$     PRINT "no"`,
		"$ ENDIF",
		"$ OUT:",
		"$ ENDIF")

	if strings.Contains(out, "no") || !strings.Contains(out, "INVIFNEST") {
		t.Errorf("out of a block: %q, want the block closed (the second ENDIF INVIFNEST)", out)
	}

	// Within the block it's in, a label after an inner block's ENDIF, and
	// one already run past, are targets.
	out = runFlow(t,
		"$ N = 0",
		"$ IF 1",
		"$ THEN",
		"$ AGAIN:",
		"$     N = N + 1",
		"$     IF N .LT. 3 THEN GOTO AGAIN",
		"$     GOTO DONE",
		"$     IF 1",
		"$     THEN",
		"$     ENDIF",
		`$ DONE: PRINT "N=''N'"`,
		"$ ENDIF",
		`$ PRINT "end"`)

	if out != "N=3\nend\n" {
		t.Errorf("within a block: %q", out)
	}
}

// TestLabelsAtTerminal: at command level 0 a label is ignored, with
// NOLBLS, and GOTO has nothing to search.
func TestLabelsAtTerminal(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	if err := d.Dispatch("HERE:"); !hasStatus(err, vmserrors.CLI_NOLBLS) {
		t.Errorf("a label: %v, want NOLBLS", err)
	}

	if err := d.Dispatch(`HERE: PRINT "run"`); err != nil || buf.String() != "%DCL-W-NOLBLS, label ignored - use only within command procedures\nrun\n" {
		t.Errorf("a label and a command: %v, %q", err, buf.String())
	}

	if err := d.Dispatch("GOTO HERE"); !hasStatus(err, vmserrors.CLI_USGOTO) {
		t.Errorf("GOTO: %v, want USGOTO", err)
	}

	// An assignment isn't a label.
	if err := d.Dispatch("X:=1"); err != nil {
		t.Errorf("X:=1: %v", err)
	}
}

// TestSearchLabel: the forward search follows the blocks it passes.
func TestSearchLabel(t *testing.T) {
	p := newProcedureSource("S.COM", strings.Join([]string{
		"$ A: PRINT 1", // 0
		"$ IF 1",       // 1
		"$ THEN",       // 2
		"$ B:",         // 3: inside a block the search enters
		"$ ELSE",       // 4
		"$ ENDIF",      // 5
		"$ ELSE",       // 6: the other branch of a block open at the start
		"$ C:",         // 7
		"$ ENDIF",      // 8
		"data",         // 9
		"$ S: SUBROUTINE",
		"$ D:", // 11: inside a subroutine
		"$ ENDSUBROUTINE",
		"$ E: ENDIF", // 13
		"$ F: SUBROUTINE",
	}, "\n"))

	for _, tc := range []struct {
		label   string
		open    int
		found   bool
		pos     int
		depth   int
		unended bool
	}{
		{"A", 0, true, 0, 0, false},
		{"B", 0, false, 0, 0, true},
		{"C", 1, false, 0, 0, true},
		{"C", 0, true, 7, 0, false},
		{"D", 0, false, 0, 0, true},
		{"S", 0, true, 10, 0, false},
		{"E", 2, true, 13, 1, false},
		{"NONE", 0, false, 0, 0, true},
	} {
		target, found, unended := p.searchLabel(tc.label, 0, tc.open, nil)
		if found != tc.found || (found && (target.pos != tc.pos || target.depth != tc.depth)) || (!found && unended != tc.unended) {
			t.Errorf("%s with %d open: %+v, %v, %v; want %v at %d, depth %d, unended %v",
				tc.label, tc.open, target, found, unended, tc.found, tc.pos, tc.depth, tc.unended)
		}
	}

	if end, ok := p.subroutineEnd(11); !ok || end != 12 {
		t.Errorf("subroutineEnd: %d, %v; want 12", end, ok)
	}
}

// TestIfBlocks: the THEN branch or the ELSE branch runs, THEN and ELSE
// may carry a command, blocks nest, and the lines of a branch that
// doesn't run are passed over unread: not substituted, data and all.
func TestIfBlocks(t *testing.T) {
	out := runFlow(t,
		"$ IF 0",
		"$ THEN",
		`$     PRINT "'F$BOGUS()'"`,
		"some data",
		"$     IF 1",
		"$     THEN",
		`$         PRINT "inner"`,
		"$     ELSE",
		`$         PRINT "inner else"`,
		"$     ENDIF",
		`$ ELSE PRINT "else command"`,
		`$     PRINT "else"`,
		"$ ENDIF",
		`$ IF "yes"`,
		`$ THEN PRINT "then command"`,
		`$     PRINT "then"`,
		"$     IF 0",
		"$     THEN",
		`$         PRINT "no"`,
		"$     ENDIF",
		"$ ELSE",
		`$     PRINT "no"`,
		"$ ENDI",
		`$ PRINT "done"`)

	if out != "else command\nelse\nthen command\nthen\ndone\n" {
		t.Errorf("output:\n%s", out)
	}
}

// TestIfBlockErrors: the command after a block's IF must be THEN; THEN,
// ELSE, and ENDIF out of place are INVIFNEST; blocks nest 15 deep.
func TestIfBlockErrors(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	for _, line := range []string{"THEN", "ELSE", "ENDIF"} {
		if err := d.Dispatch(line); !hasStatus(err, vmserrors.CLI_INVIFNEST) {
			t.Errorf("%s alone: %v, want INVIFNEST", line, err)
		}
	}

	if err := d.Dispatch("IF 1"); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch(`PRINT "x"`); !hasStatus(err, vmserrors.CLI_NOTHEN) || buf.Len() != 0 {
		t.Errorf("a command after IF: %v, %q; want NOTHEN, and not run", err, buf.String())
	}

	if err := d.Dispatch("IF 1 2"); !hasStatus(err, vmserrors.CLI_NOTHEN) {
		t.Errorf("IF 1 2: %v, want NOTHEN", err)
	}

	if err := d.Dispatch("IF 1 THEN"); !hasStatus(err, vmserrors.CLI_INSFPRM) {
		t.Errorf("IF 1 THEN: %v, want INSFPRM", err)
	}

	for i := range maxIfBlocks + 1 {
		err := d.Dispatch("IF 1")
		if err == nil {
			err = d.Dispatch("THEN")
		}

		if i < maxIfBlocks && err != nil {
			t.Fatalf("block %d: %v", i+1, err)
		}

		if i == maxIfBlocks && !hasStatus(err, vmserrors.CLI_INVIFNEST) {
			t.Errorf("block %d: %v, want INVIFNEST", i+1, err)
		}
	}
}

// TestIfBlocksAtTerminal: blocks work for lines typed at the terminal.
func TestIfBlocksAtTerminal(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	for _, line := range []string{"IF 0", "THEN", `PRINT "a"`, "ELSE", `PRINT "b"`, "ENDIF", `PRINT "c"`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if buf.String() != "b\nc\n" {
		t.Errorf("output %q, want b, c", buf.String())
	}
}

// TestIfKeepsStatus: IF, THEN, ELSE, and ENDIF leave $STATUS alone, so a
// block can test the command before it.
func TestIfKeepsStatus(t *testing.T) {
	_, out, status := runStatusProcedure(t,
		"$ FOOBAR",
		"$ IF .NOT. $STATUS",
		"$ THEN",
		`$     PRINT "failed"`,
		"$     IF 1 THEN CONTINUE",
		"$     X = 1",
		"$ ENDIF",
		"$ IF 1",
		"$ THEN",
		"$ ELSE",
		"$ ENDIF")

	if !strings.HasSuffix(out, "failed\n") || status != cliNormal {
		t.Errorf("output %q, $STATUS %08X", out, status)
	}

	_, _, status = runStatusProcedure(t, "$ FOOBAR", "$ IF 1", "$ THEN", "$ ELSE", "$ ENDIF")
	if status != statusIvverb {
		t.Errorf("$STATUS %08X after the block, want IVVERB's", status)
	}
}

// TestGosub: GOSUB and RETURN, nested, with RETURN's status (14.16.6's
// GOSUB.COM).
func TestGosub(t *testing.T) {
	out := runFlow(t,
		"$ GOSUB TEST1",
		`$ PRINT "back ''$STATUS'"`,
		"$ EXIT",
		"$ TEST1:",
		`$     PRINT "This is GOSUB level 1."`,
		"$     GOSUB TEST2",
		"$     RETURN %X3",
		"$ TEST2:",
		`$     PRINT "This is GOSUB level 2."`,
		"$     RETURN")

	if out != "This is GOSUB level 1.\nThis is GOSUB level 2.\nback %X00000003\n" {
		t.Errorf("output:\n%s", out)
	}

	// RETURN puts back the blocks open at the GOSUB.
	out = runFlow(t,
		"$ IF 1",
		"$ THEN",
		"$     GOSUB S",
		"$ ELSE",
		`$     PRINT "no"`,
		"$ ENDIF",
		`$ PRINT "end"`,
		"$ EXIT",
		"$ S:",
		"$ IF 0",
		"$ THEN",
		"$     RETURN",
		"$ ENDIF",
		"$ RETURN")

	if out != "end\n" {
		t.Errorf("blocks: %q", out)
	}
}

// TestGosubErrors: RETURN without GOSUB, a missing label (the end of the
// procedure), GOSUBs too deep, and GOSUB at the terminal.
func TestGosubErrors(t *testing.T) {
	if out := runFlow(t, "$ RETURN", `$ PRINT "after"`); !strings.Contains(out, "%DCL-W-BADRET") || !strings.Contains(out, "after") {
		t.Errorf("RETURN: %q, want BADRET, and on", out)
	}

	if out := runFlow(t, "$ GOSUB NOWHERE", `$ PRINT "after"`); !strings.Contains(out, "%DCL-W-USGOSUB") || strings.Contains(out, "after") {
		t.Errorf("GOSUB NOWHERE: %q, want USGOSUB, and the end", out)
	}

	out := runFlow(t, "$ N = 0", "$ R:", "$ N = N + 1", "$ GOSUB R", `$ PRINT "N=''N'"`)
	if strings.Count(out, "GOSUBMAX") != 1 || !strings.HasSuffix(out, "N=17\n") {
		t.Errorf("too deep: %q", out)
	}

	d, _, _ := newCommandDispatcher(t)
	if err := d.Dispatch("GOSUB X"); !hasStatus(err, vmserrors.CLI_INVGOSUB) {
		t.Errorf("at the terminal: %v, want INVGOSUB", err)
	}

	if err := d.Dispatch("RETURN"); !hasStatus(err, vmserrors.CLI_BADRET) {
		t.Errorf("RETURN at the terminal: %v, want BADRET", err)
	}
}

// TestCall: CALL runs a subroutine at a new level with its parameters;
// running the procedure line by line passes over subroutines; EXIT's
// status is CALL's (14.17).
func TestCall(t *testing.T) {
	err, out, status := runStatusProcedure(t,
		"$ SHOW_IT: SUBROUTINE",
		`$     PRINT "''P1' ''P2'"`,
		"$     X = 1",
		"$     NESTED: SUBROUTINE",
		`$         PRINT "nested"`,
		"$     ENDSUBROUTINE",
		"$     CALL NESTED",
		"$ ENDSUBROUTINE",
		`$ PRINT "main"`,
		`$ CALL SHOW_IT "Hello there" two`,
		`$ PRINT "after [''P1'] [''X']"`,
		"$ CALL FAILS",
		"$ EXIT",
		"$ FAILS: SUBROUTINE",
		"$     EXIT 3",
		`$     PRINT "no"`,
		"$ ENDSUBROUTINE")

	if err != nil || out != "main\nHello there TWO\nnested\nafter [] []\n" || status != 3|stsInhibitMsg {
		t.Errorf("@: %v, $STATUS %08X, output:\n%s", err, status, out)
	}
}

// TestCallErrors: a subroutine inside another can't be called from
// outside it (14.17.1.2's BAR), nor a label that isn't a SUBROUTINE's;
// labels are local to the subroutine's level; a SUBROUTINE with no
// ENDSUBROUTINE; CALL at the terminal.
func TestCallErrors(t *testing.T) {
	out := runFlow(t,
		"$ CALL BAR",
		"$ CALL PLAIN",
		"$ CALL OUTSIDE",
		`$ PRINT "after"`,
		"$ EXIT",
		"$ PLAIN:",
		"$ MAIN: SUBROUTINE",
		"$     BAR: SUBROUTINE",
		"$     ENDSUBROUTINE",
		"$ ENDSUBROUTINE",
		"$ OUTSIDE: SUBROUTINE",
		"$     GOTO PLAIN",
		"$ ENDSUBROUTINE")

	if strings.Count(out, "%DCL-W-USCALL, CALL target BAR") != 1 || strings.Count(out, "USCALL, CALL target PLAIN") != 1 ||
		!strings.Contains(out, "USGOTO") || !strings.HasSuffix(out, "after\n") {
		t.Errorf("output:\n%s", out)
	}

	if out := runFlow(t, "$ S: SUBROUTINE", `$ PRINT "no"`); !strings.Contains(out, "MSNGENDS") || strings.Contains(out, "no") {
		t.Errorf("no ENDSUBROUTINE: %q", out)
	}

	if out := runFlow(t, "$ ENDSUBROUTINE"); !strings.Contains(out, "INVCALL") {
		t.Errorf("ENDSUBROUTINE alone: %q", out)
	}

	d, _, _ := newCommandDispatcher(t)
	if err := d.Dispatch("CALL X"); !hasStatus(err, vmserrors.CLI_USCALL) {
		t.Errorf("at the terminal: %v, want USCALL", err)
	}
}

// TestCallOutput: CALL/OUTPUT sends the subroutine's output to a file.
func TestCallOutput(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "sub.log")

	out := runFlow(t,
		`$ CALL/OUTPUT="`+log+`" S "to the file"`,
		`$ PRINT "here"`,
		"$ EXIT",
		"$ S: SUBROUTINE",
		`$ PRINT "''P1'"`,
		"$ ENDSUBROUTINE")

	data, err := os.ReadFile(log)
	if err != nil || string(data) != "to the file\n" || out != "here\n" {
		t.Errorf("file %q (%v), output %q", data, err, out)
	}
}
