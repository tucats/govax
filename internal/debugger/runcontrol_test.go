package debugger_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// The run-control bugs docs/PHASE-42.md fixes in subtask 2, each on
// Phase 41's probe image DBGDIS (testdata/dbg/vax/dbgdis.exe). Its
// addresses: SUB2 is at 558, so its first instruction (after the entry
// mask) is at 55A, DBGSUB\SUB2\%LINE 20; START's CALLS of SUB1 is
// DBGDIS\START\%LINE 90, and returns to 4FD, the start of line 91.

// stepBreakpoints counts the one-shot breakpoints STEP/OVER and
// STEP/RETURN set, which should never outlive their STEP.
func stepBreakpoints(c *console.Console) int {
	n := 0

	for _, bp := range dbgOf(c).Breakpoints {
		if bp.Step {
			n++
		}
	}

	return n
}

// TestRunStopsAtBreakpoint: RUN/NODEBUG stops at a breakpoint set
// before it, and GO then finishes the image (bug 1: RUN used to run past
// every breakpoint to "DBGDIS: done").
func TestRunStopsAtBreakpoint(t *testing.T) {
	c := newRunnableConsole(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	dbgOf(c).AddBreakpoint(0x55A)

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	if err := c.Run(dbgImagePath(t, "dbgdis.exe"), console.RunOptions{Debug: console.DebugOff}); err != nil {
		t.Fatalf("RUN: %v", err)
	}

	if got := buf.String(); got != "break at DBGSUB\\SUB2\\%LINE 20\n" {
		t.Fatalf("RUN: got %q, want the break at SUB2", got)
	}

	buf.Reset()

	if err := c.Execute(nil); err != nil {
		t.Fatalf("GO: %v", err)
	}

	// SUB2 is called twice (SUB1 is called by CALLS and by CALLG), so GO
	// stops there once more before the image finishes.
	if got := buf.String(); got != "break at DBGSUB\\SUB2\\%LINE 20\n" {
		t.Fatalf("first GO: got %q, want the second break at SUB2", got)
	}

	dbgOf(c).ClearAllBreakpoints()
	buf.Reset()

	if err := c.Execute(nil); err != nil {
		t.Fatalf("GO: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "DBGDIS: done") {
		t.Errorf("last GO: got %q, want the image to finish", got)
	}
}

// TestCallStopsAtBreakpoint: CALL stops at a breakpoint in the called
// routine, including one on its first instruction (bug 1: CALL ran to
// completion).
func TestCallStopsAtBreakpoint(t *testing.T) {
	c := newRunnableConsole(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.Run(dbgImagePath(t, "dbgdis.exe"), console.RunOptions{NoExecute: true}); err != nil {
		t.Fatalf("RUN/NOEXECUTE: %v", err)
	}

	dbgOf(c).AddBreakpoint(0x55A)

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	if err := c.Call(0x558, false); err != nil {
		t.Fatalf("CALL SUB2: %v", err)
	}

	if got := buf.String(); got != "break at DBGSUB\\SUB2\\%LINE 20\n" {
		t.Errorf("CALL SUB2: got %q, want the break on its first instruction", got)
	}
}

// TestStepOverEndsAtBreakpoint: a STEP/OVER whose call stops at a user
// breakpoint leaves no one-shot breakpoint behind, so a later GO runs on
// rather than stopping with a stray "Stepped to" (bug 2).
func TestStepOverEndsAtBreakpoint(t *testing.T) {
	d, buf, _ := stepImage(t, "dbgdis.exe")

	dispatchOutput(t, d, buf, `SET BREAK DBGSUB\SUB2+2`)
	dispatchOutput(t, d, buf, `SET BREAK DBGDIS\START\%LINE 90`)

	if got := dispatchOutput(t, d, buf, "GO"); got != "break at DBGDIS\\START\\%LINE 90\n" {
		t.Fatalf("GO: got %q", got)
	}

	if got := withoutStack(dispatchOutput(t, d, buf, "STEP/OVER")); !strings.HasSuffix(got, "break at DBGSUB\\SUB2\\%LINE 20\n") {
		t.Fatalf("STEP/OVER: got %q, want it to stop at SUB2's breakpoint", got)
	}

	if n := stepBreakpoints(d.Console); n != 0 {
		t.Errorf("after STEP/OVER stopped elsewhere: %d step breakpoints left", n)
	}

	dispatchOutput(t, d, buf, "CLEAR BREAK/ALL")

	if got := dispatchOutput(t, d, buf, "GO"); strings.Contains(got, "Stepped to") || !strings.Contains(got, "DBGDIS: done") {
		t.Errorf("GO: got %q, want the image to finish", got)
	}
}

// TestStepReturnEndsAtBreakpoint: the same for STEP/RETURN, from SUB1
// with a breakpoint in SUB2, which SUB1 calls (bug 2).
func TestStepReturnEndsAtBreakpoint(t *testing.T) {
	d, buf, _ := stepImage(t, "dbgdis.exe")

	dispatchOutput(t, d, buf, `SET BREAK DBGSUB\SUB1+2`)

	if got := dispatchOutput(t, d, buf, "GO"); !strings.HasPrefix(got, "break at DBGSUB\\SUB1") {
		t.Fatalf("GO: got %q, want the break in SUB1", got)
	}

	dispatchOutput(t, d, buf, `SET BREAK DBGSUB\SUB2+2`)

	if got := dispatchOutput(t, d, buf, "STEP/RETURN"); !strings.HasSuffix(got, "break at DBGSUB\\SUB2\\%LINE 20\n") {
		t.Fatalf("STEP/RETURN: got %q, want it to stop at SUB2's breakpoint", got)
	}

	if n := stepBreakpoints(d.Console); n != 0 {
		t.Errorf("after STEP/RETURN stopped elsewhere: %d step breakpoints left", n)
	}
}

// TestStepOverReturnsToBreakpoint: a STEP/OVER whose call returns to an
// address with a user breakpoint stops there as that breakpoint, and its
// own one-shot breakpoint, which the user's hid, is gone (bug 3).
func TestStepOverReturnsToBreakpoint(t *testing.T) {
	d, buf, _ := stepImage(t, "dbgdis.exe")

	dispatchOutput(t, d, buf, `SET BREAK DBGDIS\START\%LINE 90`)
	dispatchOutput(t, d, buf, `SET BREAK DBGDIS\START\%LINE 91`)

	if got := dispatchOutput(t, d, buf, "GO"); got != "break at DBGDIS\\START\\%LINE 90\n" {
		t.Fatalf("GO: got %q", got)
	}

	if got := withoutStack(dispatchOutput(t, d, buf, "STEP/OVER")); !strings.HasSuffix(got, "break at DBGDIS\\START\\%LINE 91\n") {
		t.Fatalf("STEP/OVER: got %q, want the user breakpoint at line 91", got)
	}

	if n := stepBreakpoints(d.Console); n != 0 {
		t.Errorf("after STEP/OVER returned to a user breakpoint: %d step breakpoints left", n)
	}
}

// TestInstructionBreakNamesLocation: an instruction breakpoint in an image
// with a debug symbol table names where it stopped, as an address
// breakpoint's "break at" does (bug 6): the first JSB is START's line 87.
func TestInstructionBreakNamesLocation(t *testing.T) {
	d, buf, _ := stepImage(t, "dbgdis.exe")

	dispatchOutput(t, d, buf, "SET BREAK/INSTRUCTION JSB")

	if got := dispatchOutput(t, d, buf, "GO"); got != "Instruction break at DBGDIS\\START\\%LINE 87\n" {
		t.Errorf("GO: got %q, want the location named", got)
	}
}
