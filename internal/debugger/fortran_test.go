package debugger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
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

// compiledImage compiles, assembles, and links testdata/dbg85's DBG85.FOR with
// /DEBUG, as a programmer would, returning the image's path. The compiler's names
// hold periods (DBG85.X), which testdata/dbg85's probe says MACRO's rules read whole.
func compiledImage(t *testing.T) string {
	t.Helper()

	c := newRunnableConsole(t)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	src, err := os.ReadFile(consoletest.RepoPath(t, "testdata", "dbg85", "dbg85.for"))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	file := func(name string) string { return filepath.Join(dir, name) }

	if err := os.WriteFile(file("dbg85.for"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{
		`FORTRAN "` + file("dbg85.for") + `"/HOST/NOOPTIMIZE`,
		`MACRO "` + file("dbg85.mar") + `"/HOST/OBJECT="` + file("dbg85.obj") + `"/DEBUG`,
		`LINK "` + file("dbg85.obj") + `"/HOST/DEBUG/NOSYSLIB/EXECUTABLE="` + file("dbg85.exe") + `"`,
	} {
		if err := d.Dispatch(command); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}

	return file("dbg85.exe")
}

// TestDottedNamesOnCompiledProgram: the names the FORTRAN compiler writes can be
// read back by every command that takes a location, and a language that makes
// "." a record's component reads them as VMS's FORTRAN does.
func TestDottedNamesOnCompiledProgram(t *testing.T) {
	c := stoppedAt(t, compiledImage(t), "TXT85")

	for _, want := range []struct{ line, output string }{
		{"EXAMINE DBG85.I", "DBG85\\DBG85.I:  0000000B\n"},
		{`EXAMINE DBG85\DBG85.I`, "DBG85\\DBG85.I:  0000000B\n"},
		{"EXAMINE/FLOAT DBG85.V[1]", "DBG85\\DBG85.V[1]:       2.500000\n"},
		{`EXAMINE TXT85\DBG85.I`, "DBG85\\DBG85.I:  0000000B\n"},
		{`EXAMINE DBG85\TXT85\DBG85.I`, "DBG85\\DBG85.I:  0000000B\n"},
	} {
		if got := examineOutput(t, c, want.line); got != want.output {
			t.Errorf("%s: got %q, want %q", want.line, got, want.output)
		}
	}

	dispatch(t, c, "DEPOSIT DBG85.I=2A")

	if got := examineOutput(t, c, "EXAMINE DBG85.I"); !strings.HasSuffix(got, "0000002A\n") {
		t.Errorf("after DEPOSIT: %q", got)
	}

	if out, err := sayErr(c, "EXAMINE DBG85.NOSUCH"); err == nil || !strings.Contains(err.Error(), "DBG85.NOSUCH") {
		t.Errorf("a missing name: %q, %v", out, err)
	}

	dispatch(t, c, "SET LANGUAGE FORTRAN")

	if out := examineOutput(t, c, "SHOW RADIX"); !strings.Contains(out, "decimal") {
		t.Errorf("SHOW RADIX under FORTRAN: %q", out)
	}

	if _, err := sayErr(c, "EXAMINE DBG85.X"); err == nil || !strings.Contains(err.Error(), `DBG85\X`) {
		t.Errorf("EXAMINE DBG85.X under FORTRAN: %v", err)
	}

	dispatch(t, c, "SET LANGUAGE MACRO")

	if got := examineOutput(t, c, "EXAMINE DBG85.I"); !strings.HasSuffix(got, "0000002A\n") {
		t.Errorf("back in MACRO: %q", got)
	}

	if _, err := sayErr(c, "SET LANGUAGE KLINGON"); err == nil {
		t.Error("SET LANGUAGE KLINGON: no error")
	}
}
