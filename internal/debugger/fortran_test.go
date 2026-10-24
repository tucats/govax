package debugger_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/machine"
)

// This file tests what Phase 72's probe of the VMS debugger on a FORTRAN
// image showed (testdata/legacy72/vax/dun72.dlg), on the MACRO images
// govax can commit: the FORTRAN image itself, DUNGEON.EXE, can't be.

// TestExamineInstructionGoesOn: EXAMINE/INSTRUCTION with no location
// shows the instruction after the last one shown, as VMS's did after
// EXAMINE/INSTRUCTION .PC (DUNGEO+2, then DUNGEO\%LINE 218, then
// %LINE 218+7). Lines 47 to 49 of DBGDIS are an instruction each.
func TestExamineInstructionGoesOn(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgdis.exe"), "%LINE 47")

	for _, want := range []struct{ line, location string }{
		{"EXAMINE/INSTRUCTION .PC", `DBGDIS\START\%LINE 47:`},
		{"EXAMINE/INSTRUCTION", `DBGDIS\START\%LINE 48:`},
		{"EXAMINE/INSTRUCTION", `DBGDIS\START\%LINE 49:`},
	} {
		if got := examineOutput(t, c, want.line); !strings.HasPrefix(got, want.location) {
			t.Errorf("%s: got %q, want it to start %q", want.line, got, want.location)
		}
	}

	// "." is the last instruction shown.
	if got := examineOutput(t, c, "EXAMINE/INSTRUCTION ."); !strings.HasPrefix(got, `DBGDIS\START\%LINE 49:`) {
		t.Errorf("EXAMINE/INSTRUCTION .: got %q", got)
	}
}

// TestShowModuleLanguages: modules of more than one language get a
// language column, and the footer names none, as in dun72.dlg (DUNGEON's
// FORTRAN modules and FOR$IDATE's MACRO). DBGSUB is made a FORTRAN module
// here.
func TestShowModuleLanguages(t *testing.T) {
	c, _ := runImage(t, dbgImagePath(t, "dbgdis.exe"), machine.RunOptions{Debug: machine.DebugOn})

	for _, p := range c.DebugPrograms() {
		for _, m := range p.Modules {
			if m.Name == "DBGSUB" {
				m.Language = 1
			}
		}
	}

	out := say(t, c, "SHOW MODULE")

	for _, want := range []string{
		"module name                     symbols   language   size\n\n",
		"DBGDIS                          yes       MACRO       582\n",
		"DBGSUB                          no        FORTRAN     209\n",
		"\ntotal modules: 2.               bytes allocated: 582.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SHOW MODULE lacks %q:\n%s", want, out)
		}
	}
}
