package debugger_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// Tests of STEP and SET/SHOW STEP as the VMS debugger has them
// (docs/PHASE-42.md, subtask 7), against the session the VMS 7.3 debugger
// ran on the probe's DBGCMD image (testdata/dbgcmd/vax/step.dlg). The
// log also shows the source line after each step (subtask 8) and SHOW
// CALLS and EXAMINE (later subtasks); these tests check the lines STEP
// itself prints.

// stepStep is one command of the session and the exact output it prints.
type stepStep struct{ command, want string }

// stepSession is the probe's session with USERSTEP off, since the test
// console runs the image in kernel mode (as stepImage does).
func stepSession(t *testing.T) *console.Console {
	t.Helper()

	c := probeSession(t)
	noUserStep(c)

	return c
}

// runSteps sends each command to the debugger and checks what it printed.
func runSteps(t *testing.T, steps []stepStep) {
	t.Helper()

	c := stepSession(t)

	for _, s := range steps {
		expect(t, s.command, say(t, c, s.command), s.want)
	}
}

// TestStepDefaultsAndUnits: the defaults (by line, over calls), a step by
// instruction, into a routine, and SET STEP/SHOW STEP after each change.
func TestStepDefaultsAndUnits(t *testing.T) {
	runSteps(t, []stepStep{
		{"SHOW STEP", "step type: source, nosilent, by line,\n           over routine calls\n"},
		{"STEP", "stepped to DBGCMD\\START\\%LINE 34\n"},
		{"STEP/INSTRUCTION", "stepped to DBGCMD\\START\\%LINE 35: CALLS    S^#01,L^DBGCMD\\FACT\n"},
		{"STEP/INTO", "stepped to routine DBGCMD\\FACT\n"},
		{"SET STEP INSTRUCTION", ""},
		{"SHOW STEP", "step type: source, nosilent, by instruction,\n           over routine calls\n"},
		{"STEP", "stepped to DBGCMD\\FACT\\%LINE 55: CMPL     R2,S^#01\n"},
		{"STEP", "stepped to DBGCMD\\FACT\\%LINE 56: BGTR     DBGCMD\\FACT\\RECUR\n"},
		{"STEP 2", "stepped to DBGCMD\\FACT\\%LINE 60: CALLS    S^#01,B^DBGCMD\\FACT\n"},
	})
}

// TestStepOverRecursion: STEP/OVER of FACT's recursive CALLS isn't bound
// to the frame: it stops at the first return to BACK, four calls deep,
// and STEP/RETURN then stops at the RET of the frame it was given in.
func TestStepOverRecursion(t *testing.T) {
	c := stepSession(t)

	for _, command := range []string{"STEP", "STEP/INSTRUCTION", "STEP/INTO", "SET STEP INSTRUCTION", "STEP 4"} {
		say(t, c, command)
	}

	expect(t, "STEP/OVER", say(t, c, "STEP/OVER"),
		"stepped to DBGCMD\\FACT\\BACK: MULL2    R2,R0\n")

	// R0 = 1: the innermost call's return, as step.dlg shows.
	if got := c.CPU.GPR(0); got != 1 {
		t.Errorf("R0 after STEP/OVER = %d, want 1", got)
	}

	expect(t, "STEP/RETURN", say(t, c, "STEP/RETURN"),
		"stepped on return from DBGCMD\\FACT\\BACK to DBGCMD\\FACT\\%LINE 62: RET     \n")

	expect(t, "STEP", say(t, c, "STEP"), "stepped to DBGCMD\\FACT\\BACK: MULL2    R2,R0\n")
}

// TestStepClasses: STEP/BRANCH and STEP/CALL stop before the next
// instruction of the class, and the one a step starts at doesn't count.
func TestStepClasses(t *testing.T) {
	c := stepSession(t)

	for _, command := range []string{"STEP", "STEP/INSTRUCTION", "STEP/INTO", "SET STEP INSTRUCTION", "STEP 4", "STEP/OVER", "STEP/RETURN", "STEP"} {
		say(t, c, command)
	}

	expect(t, "STEP/BRANCH", say(t, c, "STEP/BRANCH"),
		"stepped to DBGCMD\\START\\%LINE 39: BLBS     L^DBGCMD\\COUNT,DBGCMD\\START\\ODD\n")
	expect(t, "STEP/BRANCH", say(t, c, "STEP/BRANCH"),
		"stepped to DBGCMD\\START\\ODD: SOBGTR   R3,DBGCMD\\START\\LOOP\n")
	expect(t, "STEP/CALL", say(t, c, "STEP/CALL"),
		"stepped to DBGCMD\\START\\%LINE 44: JSB      L^DBGCMD\\FACT\\BUMP\n")
}

// TestStepReturnStaysPending: a STEP/RETURN from a JSB subroutine waits
// for the RET of the CALLS frame the subroutine runs in, passing the RSB,
// and an unhandled exception break in between doesn't cancel it. It fires
// when that RET is about to run, whichever command is running.
func TestStepReturnStaysPending(t *testing.T) {
	c := stepSession(t)

	for _, command := range []string{
		"STEP", "STEP/INSTRUCTION", "STEP/INTO", "SET STEP INSTRUCTION", "STEP 4", "STEP/OVER",
		"STEP/RETURN", "STEP", "STEP/BRANCH", "STEP/BRANCH", "STEP/CALL", "SET STEP INTO", "STEP",
	} {
		say(t, c, command)
	}

	// The program signals ENDOFFILE, which nothing handles. By instruction,
	// the break names the instruction too.
	expect(t, "STEP/RETURN", say(t, c, "STEP/RETURN"),
		"%SYSTEM-W-ENDOFFILE, end of file\n"+
			"break on unhandled exception preceding DBGCMD\\START\\%LINE 48: PUSHAQ   L^00000230\n")

	// The break didn't cancel the wait: GO runs on (the warning is
	// continued) and the pending return fires at START's RET.
	got := say(t, c, "GO")
	if want := "stepped on return from DBGCMD\\FACT\\BUMP to DBGCMD\\START\\LAST: RET     \n"; !strings.HasSuffix(got, want) {
		t.Errorf("GO:\n got %q\nwant it to end with %q", got, want)
	}

	// /SILENT runs the RET and shows nothing; GO finishes the image.
	expect(t, "STEP/SILENT", say(t, c, "STEP/SILENT"), "")
	expect(t, "GO", say(t, c, "GO"), "%DEBUG-I-EXITSTATUS, is '%SYSTEM-S-NORMAL, normal successful completion'\n")
}

// TestSetStepKeywords: SET STEP takes several keywords at once, abbreviated,
// and refuses one it doesn't know.
func TestSetStepKeywords(t *testing.T) {
	c := stepSession(t)

	say(t, c, "SET STEP INSTRUCTION,INTO,SILENT,NOSOURCE")
	expect(t, "SHOW STEP", say(t, c, "SHOW STEP"), "step type: nosource, silent, by instruction,\n           into routine calls\n")

	say(t, c, "SET STEP LINE OVER NOSILENT SOURCE")
	expect(t, "SHOW STEP", say(t, c, "SHOW STEP"), "step type: source, nosilent, by line,\n           over routine calls\n")

	if out, err := sayErr(c, "SET STEP BOGUS"); err == nil || !strings.Contains(err.Error(), "BOGUS") {
		t.Errorf("SET STEP BOGUS: out %q, err %v, want a bad-keyword error", out, err)
	}
}

// TestStepLastModeWins: of /INTO, /OVER, and /RETURN the last one typed is
// used, as VMS does.
func TestStepLastModeWins(t *testing.T) {
	c := stepSession(t)

	say(t, c, "STEP")
	say(t, c, "STEP/INSTRUCTION")

	// The CALLS at line 35: /INTO/OVER steps over it, /OVER/INTO into it.
	expect(t, "STEP/INTO/OVER", say(t, c, "STEP/INTO/OVER"), "stepped to DBGCMD\\START\\%LINE 36\n")
}

// TestStepStopsAtBreakpoint: a user breakpoint inside a stepped-over call
// ends the STEP there with the breakpoint's message.
func TestStepStopsAtBreakpoint(t *testing.T) {
	c := stepSession(t)

	say(t, c, "STEP")
	say(t, c, "STEP/INSTRUCTION")
	say(t, c, "SET BREAK FACT")

	expect(t, "STEP/OVER", say(t, c, "STEP/OVER"), "break at routine DBGCMD\\FACT\n")
}
