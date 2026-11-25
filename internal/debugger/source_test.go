package debugger_test

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// Tests of the source lines the debugger shows below a break or a step
// (docs/planning/PHASE-42.md, subtask 8). The expected text is what the VMS 7.3
// debugger showed in the probe's logs (testdata/dbgcmd/vax/*.dlg).

// sourceSession is the probe session with SET SOURCE pointing at the
// directory that holds DBGCMD.MAR, which the image's debug symbols name
// by a path that exists only on the VMS machine it was built on.
func sourceSession(t *testing.T) *console.Console {
	t.Helper()

	c := stepSession(t)
	say(t, c, `SET SOURCE "`+filepath.Dir(filepath.Dir(probeImage(t)))+`"`)

	return c
}

// TestSourceNotFoundShowsLocationAlone: until SET SOURCE names the
// directory, the file isn't found and the report is the location line.
func TestSourceNotFoundShowsLocationAlone(t *testing.T) {
	c := stepSession(t)

	expect(t, "STEP", say(t, c, "STEP"), "stepped to DBGCMD\\START\\%LINE 34\n")
	expect(t, "SHOW SOURCE", say(t, c, "SHOW SOURCE"),
		"For EDIT:\n    no directory search list in effect, match the latest source file version\n"+
			"For DISPLAY:\n    no directory search list in effect, match the exact source file version\n")
}

// TestSourceAfterSteps: step.dlg's first steps, with their source lines
// (tabs expanded to every eighth column), and /NOSOURCE and SET STEP
// NOSOURCE turning them off.
func TestSourceAfterSteps(t *testing.T) {
	c := sourceSession(t)

	for _, s := range []stepStep{
		{"STEP", "stepped to DBGCMD\\START\\%LINE 34\n    34:         PUSHL   R2\n"},
		{"STEP/INSTRUCTION", "stepped to DBGCMD\\START\\%LINE 35: CALLS    S^#01,L^DBGCMD\\FACT\n" +
			"    35:         CALLS   #1, FACT                ; FACT(5), recursively\n"},
		{"STEP/INTO", "stepped to routine DBGCMD\\FACT\n    54:         MOVL    4(AP), R2\n"},
		{"STEP/NOSOURCE", "stepped to DBGCMD\\FACT\\%LINE 55\n"},
		{"SET STEP NOSOURCE", ""},
		{"STEP", "stepped to DBGCMD\\FACT\\%LINE 56\n"},
		{"SET STEP SOURCE", ""},
	} {
		expect(t, s.command, say(t, c, s.command), s.want)
	}
}

// TestSourceAfterBreaks: a break at a routine and at a label show the
// line, and an exception break does too (break.dlg, except.dlg).
func TestSourceAfterBreaks(t *testing.T) {
	c := sourceSession(t)

	say(t, c, "SET BREAK FACT")
	expect(t, "GO", say(t, c, "GO"), "break at routine DBGCMD\\FACT\n    54:         MOVL    4(AP), R2\n")
	say(t, c, "CANCEL BREAK/ALL")

	say(t, c, "SET BREAK/EXCEPTION")
	expect(t, "GO", say(t, c, "GO"),
		"%SYSTEM-W-ENDOFFILE, end of file\nbreak on exception preceding DBGCMD\\CATCH\\%LINE 71\n    71:         RET\n")
}

// TestSourceStepReturn: the report of STEP/RETURN is followed by the line
// of the RET it stopped at. Given at a routine's first instruction, it's
// from "routine NAME", and stepping by line it names no instruction (VMS
// 7.1's DBG113R).
func TestSourceStepReturn(t *testing.T) {
	c := sourceSession(t)

	say(t, c, "SET BREAK FACT")
	say(t, c, "GO")
	say(t, c, "CANCEL BREAK/ALL")

	expect(t, "STEP/RETURN", say(t, c, "STEP/RETURN"),
		"stepped on return from routine DBGCMD\\FACT to DBGCMD\\FACT\\%LINE 62\n    62:         RET\n")
}

// TestSetSourceList: SET SOURCE takes a list, SHOW SOURCE lists it, and
// CANCEL SOURCE takes it away, so the line is no longer found.
func TestSetSourceList(t *testing.T) {
	c := stepSession(t)
	dir := filepath.Dir(filepath.Dir(probeImage(t)))

	say(t, c, `SET SOURCE nowhere,"`+dir+`"`)
	expect(t, "SHOW SOURCE", say(t, c, "SHOW SOURCE"),
		"For EDIT:\n    source directory search list for all modules, match the latest source file version\n"+
			"        NOWHERE\n        "+dir+"\n"+
			"For DISPLAY:\n    source directory search list for all modules, match the exact source file version\n"+
			"        NOWHERE\n        "+dir+"\n")

	say(t, c, "CANCEL SOURCE")
	expect(t, "STEP", say(t, c, "STEP"), "stepped to DBGCMD\\START\\%LINE 34\n")
}
