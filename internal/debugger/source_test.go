package debugger_test

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// Tests of the source lines the debugger shows below a break or a step
// (docs/PHASE-42.md, subtask 8). The expected text is what the VMS 7.3
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
	expect(t, "SHOW SOURCE", say(t, c, "SHOW SOURCE"), "%DEBUG-I-NOSOURCEDIR, no source directory search list is in effect\n")
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
// of the RET it stopped at.
func TestSourceStepReturn(t *testing.T) {
	c := sourceSession(t)

	say(t, c, "SET BREAK FACT")
	say(t, c, "GO")
	say(t, c, "CANCEL BREAK/ALL")

	expect(t, "STEP/RETURN", say(t, c, "STEP/RETURN"),
		"stepped on return from DBGCMD\\FACT\\%LINE 54 to DBGCMD\\FACT\\%LINE 62: RET     \n    62:         RET\n")
}

// TestSetSourceList: SET SOURCE takes a list, SHOW SOURCE lists it, and
// CANCEL SOURCE takes it away, so the line is no longer found.
func TestSetSourceList(t *testing.T) {
	c := stepSession(t)
	dir := filepath.Dir(filepath.Dir(probeImage(t)))

	say(t, c, `SET SOURCE nowhere,"`+dir+`"`)
	expect(t, "SHOW SOURCE", say(t, c, "SHOW SOURCE"),
		"source directory search list for all modules:\n    NOWHERE\n    "+dir+"\n")

	say(t, c, "CANCEL SOURCE")
	expect(t, "STEP", say(t, c, "STEP"), "stepped to DBGCMD\\START\\%LINE 34\n")
}
