package debugger_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// Tests of SET BREAK, SHOW BREAK, and CANCEL BREAK (docs/PHASE-42.md,
// subtask 6), against the sessions the VMS 7.3 debugger ran on the Phase 42
// probe's DBGCMD image (testdata/dbgcmd/vax/break.dlg and brkcls.dlg).
//
// The probe's logs also show the source line after each break, and run
// SHOW CALLS and EXAMINE, which are later subtasks'; these tests check the
// lines the debugger prints for the breakpoint commands alone.

// probeSession runs the probe image under the debugger and returns its
// console, stopped at START's first instruction.
func probeSession(t *testing.T) *console.Console {
	t.Helper()

	c, _ := runImage(t, probeImage(t), console.RunOptions{Debug: console.DebugOn})

	return c
}

// say sends one command line to c's debugger and returns what it printed.
// A command that fails will fail the test.
func say(t *testing.T, c *console.Console, command string) string {
	t.Helper()

	out, err := sayErr(c, command)
	if err != nil {
		t.Fatalf("%s: %v\n%s", command, err, out)
	}

	return out
}

// sayErr is say for a command that may fail.
func sayErr(c *console.Console, command string) (string, error) {
	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	err := c.Debugger.Dispatch(command)

	return buf.String(), err
}

// expect fails the test unless out is exactly want.
func expect(t *testing.T, command, out, want string) {
	t.Helper()

	if out != want {
		t.Errorf("%s:\n got %q\nwant %q", command, out, want)
	}
}

// TestBreakAtRoutine: a routine's name breaks after its entry mask, at
// each recursion, and CANCEL BREAK takes it away (break.dlg's first block).
func TestBreakAtRoutine(t *testing.T) {
	c := probeSession(t)

	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "%DEBUG-I-NOBREAKS, no breakpoints are set\n")

	say(t, c, "SET BREAK FACT")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "breakpoint at routine DBGCMD\\FACT\n")

	// FACT is entered recursively: the stop is after the entry mask each
	// time, 2 bytes in.
	for range 2 {
		expect(t, "GO", say(t, c, "GO"), "break at routine DBGCMD\\FACT\n")
	}

	say(t, c, "CANCEL BREAK FACT")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "%DEBUG-I-NOBREAKS, no breakpoints are set\n")
}

// TestBreakAfter: /AFTER:3 breaks the third time the place is reached and
// every time after (break.dlg).
func TestBreakAfter(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK/AFTER:3 BACK")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "breakpoint at DBGCMD\\FACT\\BACK\n   /after: 3\n")

	// FACT(5) recurses to depth 5 first, so BACK is reached once per
	// return: the third return leaves R0 = 3! = 6, the fourth 4! = 24.
	expect(t, "GO", say(t, c, "GO"), "break at DBGCMD\\FACT\\BACK\n")

	if r0 := c.CPU.GPR(0); r0 != 6 {
		t.Errorf("R0 at the first stop = %d, want 6", r0)
	}

	expect(t, "GO", say(t, c, "GO"), "break at DBGCMD\\FACT\\BACK\n")

	if r0 := c.CPU.GPR(0); r0 != 24 {
		t.Errorf("R0 at the second stop = %d, want 24", r0)
	}
}

// TestBreakAtUnlabeledAddress: an address with no label is named by its
// line (break.dlg).
func TestBreakAtUnlabeledAddress(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK BACK+3")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "breakpoint at DBGCMD\\FACT\\%LINE 62\n")
	expect(t, "GO", say(t, c, "GO"), "break at DBGCMD\\FACT\\%LINE 62\n")
}

// TestBreakListAndTemporary: a list sets one breakpoint per address, and a
// temporary one is gone once it has stopped the program (break.dlg).
//
// The breakpoints are listed in the order the commands set them, and those
// of one command in reverse (break.dlg's SET BREAK LOOP, ODD lists ODD
// first; the one probe of it, so unconfirmed beyond that).
func TestBreakListAndTemporary(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK LAST, ODD")
	say(t, c, "SET BREAK/TEMPORARY LOOP")

	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"),
		"breakpoint at DBGCMD\\START\\ODD\n"+
			"breakpoint at DBGCMD\\START\\LAST\n"+
			"breakpoint at DBGCMD\\START\\LOOP [temporary]\n")

	expect(t, "GO", say(t, c, "GO"), "break at DBGCMD\\START\\LOOP\n")

	// LOOP's one-shot breakpoint is spent; ODD's is not.
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"),
		"breakpoint at DBGCMD\\START\\ODD\nbreakpoint at DBGCMD\\START\\LAST\n")

	expect(t, "GO", say(t, c, "GO"), "break at DBGCMD\\START\\ODD\n")
	expect(t, "GO", say(t, c, "GO"), "break at DBGCMD\\START\\ODD\n")

	say(t, c, "CANCEL BREAK ODD, LAST")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "%DEBUG-I-NOBREAKS, no breakpoints are set\n")
}

// TestBreakWhenAndDo: WHEN keeps a breakpoint from stopping until its
// condition holds, and DO runs commands when it stops. SHOW BREAK shows
// both clauses as typed.
func TestBreakWhenAndDo(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK LOOP WHEN (.COUNT EQL 1)")
	say(t, c, "SET BREAK ODD DO (SHOW BREAK)")

	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"),
		"breakpoint at DBGCMD\\START\\LOOP\n   when (.COUNT EQL 1)\n"+
			"breakpoint at DBGCMD\\START\\ODD\n   do (SHOW BREAK)\n")

	// ODD's DO runs after its break message, as if typed at the prompt.
	out := say(t, c, "GO")
	if !strings.HasPrefix(out, "break at DBGCMD\\START\\ODD\nbreakpoint at DBGCMD\\START\\LOOP\n") {
		t.Errorf("GO:\n%s", out)
	}

	say(t, c, "CANCEL BREAK ODD")

	// COUNT is incremented at LOOP, so on LOOP's second arrival it is 1,
	// where the condition first holds.
	expect(t, "GO", say(t, c, "GO"), "break at DBGCMD\\START\\LOOP\n")

	addr, err := c.EvalWhole(`COUNT`)
	if err != nil {
		t.Fatal(err)
	}

	if v, _ := c.Mem.LoadLongword(c.CPU, addr); v != 1 {
		t.Errorf("COUNT at the stop = %d, want 1", v)
	}
}

// TestBreakWhenUnreadable: a WHEN condition that can't be evaluated shows
// its error, and the breakpoint stops anyway (break.dlg's NOACCESSR).
func TestBreakWhenUnreadable(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK LOOP WHEN (.0 EQL 1)")

	expect(t, "GO", say(t, c, "GO"),
		"%DEBUG-E-NOACCESSR, no read access to address 00000000\nbreak at DBGCMD\\START\\LOOP\n")
}

// TestBreakCalls: /CALL stops before each call and return instruction
// (brkcls.dlg).
func TestBreakCalls(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK/CALL")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"),
		"breakpoint on calls:\n"+
			" BSBB     BSBW     CALLG    CALLS    JSB      RET      RSB     \n")

	expect(t, "GO", say(t, c, "GO"), "break on calls at DBGCMD\\START\\%LINE 35\n")
	expect(t, "GO", say(t, c, "GO"), "break on calls at DBGCMD\\FACT\\%LINE 60\n")

	say(t, c, "CANCEL BREAK/CALL")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "%DEBUG-I-NOBREAKS, no breakpoints are set\n")
}

// TestBreakReturn: /RETURN stops at each RET of the routine, in every
// recursion (brkcls.dlg).
func TestBreakReturn(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK/RETURN FACT")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "breakpoint on return from routine DBGCMD\\FACT\n")

	expect(t, "GO", say(t, c, "GO"),
		"break on return from routine DBGCMD\\FACT at DBGCMD\\FACT\\%LINE 58\n")
	expect(t, "GO", say(t, c, "GO"),
		"break on return from routine DBGCMD\\FACT at DBGCMD\\FACT\\%LINE 62\n")
}

// TestBreakBranchLineInstruction: /BRANCH, /LINE, and /INSTRUCTION
// (brkcls.dlg), with SHOW BREAK's opcode listings.
func TestBreakBranchLineInstruction(t *testing.T) {
	c := probeSession(t)

	// START calls FACT first; the loop that follows is where the probe's
	// session was.
	say(t, c, "SET BREAK LOOP")
	say(t, c, "GO")
	say(t, c, "CANCEL BREAK LOOP")

	say(t, c, "SET BREAK/BRANCH")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"),
		"breakpoint on branches:\n"+
			" ACBB     ACBD     ACBF     ACBG     ACBH     ACBL     ACBW     AOBLEQ  \n"+
			" AOBLSS   BBC      BBCC     BBCCI    BBCS     BBS      BBSC     BBSS    \n"+
			" BBSSI    BCC      BCS      BEQL     BEQLU    BGEQ     BGEQU    BGTR    \n"+
			" BGTRU    BLBC     BLBS     BLEQ     BLEQU    BLSS     BLSSU    BNEQ    \n"+
			" BNEQU    BRB      BRW      BVC      BVS      CASEB    CASEL    CASEW   \n"+
			" JMP      SOBGEQ   SOBGTR  \n")
	expect(t, "GO", say(t, c, "GO"), "break on branches at DBGCMD\\START\\%LINE 39\n")
	expect(t, "GO", say(t, c, "GO"), "break on branches at DBGCMD\\START\\ODD\n")
	expect(t, "GO", say(t, c, "GO"), "break on branches at DBGCMD\\START\\%LINE 39\n")
	say(t, c, "CANCEL BREAK/BRANCH")

	say(t, c, "SET BREAK/LINE")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "breakpoint on lines\n")
	expect(t, "GO", say(t, c, "GO"), "break on lines at DBGCMD\\START\\%LINE 40\n")
	expect(t, "GO", say(t, c, "GO"), "break on lines at DBGCMD\\START\\ODD\n")
	expect(t, "GO", say(t, c, "GO"), "break on lines at DBGCMD\\START\\LOOP\n")
	say(t, c, "CANCEL BREAK/LINE")

	say(t, c, "SET BREAK/INSTRUCTION=(MOVB,MOVC3)")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"),
		"breakpoint on instruction(s): \n MOVB     MOVC3   \n")
	expect(t, "GO", say(t, c, "GO"), "break on instruction(s) at DBGCMD\\START\\%LINE 42\n")
	expect(t, "GO", say(t, c, "GO"), "break on instruction(s) at DBGCMD\\START\\%LINE 43\n")
	say(t, c, "CANCEL BREAK/INSTRUCTION")

	say(t, c, "SET BREAK/INSTRUCTION")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "breakpoint on instructions\n")
	expect(t, "GO", say(t, c, "GO"), "break on instruction at DBGCMD\\START\\%LINE 44\n")
	expect(t, "GO", say(t, c, "GO"), "break on instruction at DBGCMD\\FACT\\BUMP\n")
	say(t, c, "CANCEL BREAK/INSTRUCTION")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "%DEBUG-I-NOBREAKS, no breakpoints are set\n")
}

// TestBreakException: /EXCEPTION stops when a condition is signaled, even
// one a handler will handle, before the handlers hear of it; the
// condition's message comes first (except.dlg).
func TestBreakException(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK/EXCEPTION")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "breakpoint on exception\n")

	expect(t, "GO", say(t, c, "GO"),
		"%SYSTEM-W-ENDOFFILE, end of file\nbreak on exception preceding DBGCMD\\CATCH\\%LINE 71\n")

	// The second signal has no handler: the break for it, and then, once
	// the catch-all has shown its message, the unhandled one.
	expect(t, "GO", say(t, c, "GO"),
		"%SYSTEM-W-ENDOFFILE, end of file\nbreak on exception preceding DBGCMD\\START\\%LINE 48\n")
	expect(t, "GO", say(t, c, "GO"),
		"%SYSTEM-W-ENDOFFILE, end of file\nbreak on unhandled exception preceding DBGCMD\\START\\%LINE 48\n")
}

// TestCancelBreakAll: /ALL cancels breakpoints of every kind, and
// cancelling where none is set is the NOBREAKS message.
func TestCancelBreakAll(t *testing.T) {
	c := probeSession(t)

	say(t, c, "SET BREAK LOOP")
	say(t, c, "SET BREAK/CALL")
	say(t, c, "SET BREAK/FAULT=10")

	say(t, c, "CANCEL BREAK/ALL")
	expect(t, "SHOW BREAK", say(t, c, "SHOW BREAK"), "%DEBUG-I-NOBREAKS, no breakpoints are set\n")
	expect(t, "CANCEL BREAK LOOP", say(t, c, "CANCEL BREAK LOOP"), "%DEBUG-I-NOBREAKS, no breakpoints are set\n")
}

// TestBreakConditions: the operators WHEN accepts, on a program of NOPs
// where a breakpoint's address is reached exactly once.
func TestBreakConditions(t *testing.T) {
	cases := []struct {
		when string
		stop bool
	}{
		{"(1 EQL 1)", true},
		{"(1 NEQ 1)", false},
		{"(2 GTR 3 OR 1 EQL 1)", true},
		{"(2 GTR 3 AND 1 EQL 1)", false},
		{"(NOT 1 LSS 2)", false},
		{"((1 LEQ 1) AND (2 GEQ 3 OR 4 GTR 3))", true},
		{"(5)", true},
		{"(0)", false},
	}

	for _, tc := range cases {
		t.Run(tc.when, func(t *testing.T) {
			c, buf := newTestConsole(t)
			loadProgram(t, c, 0x200, opNop, opNop, opNop, opHalt)

			if err := c.Debugger.Dispatch("SET BREAK 202 WHEN " + tc.when); err != nil {
				t.Fatal(err)
			}

			buf.Reset()

			start := uint32(0x200)

			if err := c.Execute(&start); err != nil {
				t.Fatal(err)
			}

			if got := strings.Contains(buf.String(), "break at 00000202"); got != tc.stop {
				t.Errorf("WHEN %s: stopped = %v, want %v\n%s", tc.when, got, tc.stop, buf.String())
			}
		})
	}
}

// TestSetBreakErrors: what SET BREAK and CANCEL BREAK refuse.
func TestSetBreakErrors(t *testing.T) {
	c := probeSession(t)

	for _, command := range []string{
		"SET BREAK",                        // no address
		"SET BREAK/CALL FACT",              // a class takes no address
		"SET BREAK/CALL/BRANCH",            // one class at a time
		"SET BREAK/INSTRUCTION=(NOSUCH)",   // not an opcode
		"SET BREAK/AFTER:0 FACT",           // a count of at least 1
		"SET BREAK FACT WHEN .COUNT EQL 1", // a clause needs parentheses
		"SET BREAK/RETURN",                 // no routine
		"CANCEL BREAK",                     // nothing named
	} {
		if out, err := sayErr(c, command); err == nil {
			t.Errorf("%s: no error; printed %q", command, out)
		}
	}

	if n := len(dbgOf(c).Breakpoints); n != 0 {
		t.Errorf("%d breakpoints were set by commands that failed", n)
	}
}
