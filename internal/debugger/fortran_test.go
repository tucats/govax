package debugger_test

import (
	"os"
	"path/filepath"
	"regexp"
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

	return buildCompiled(t, func(file func(string) string) []string {
		return []string{
			`FORTRAN "` + file("dbg85.for") + `"/HOST/NOOPTIMIZE`,
			`MACRO "` + file("dbg85.mar") + `"/HOST/OBJECT="` + file("dbg85.obj") + `"/DEBUG`,
		}
	})
}

// buildCompiled puts testdata/dbg85's DBG85.FOR in a directory, runs the commands that
// make its object, links it with /DEBUG, and returns the image's path.
func buildCompiled(t *testing.T, commands func(file func(string) string) []string) string {
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

	for _, command := range append(commands(file),
		`LINK "`+file("dbg85.obj")+`"/HOST/DEBUG/NOSYSLIB/EXECUTABLE="`+file("dbg85.exe")+`"`) {
		if err := d.Dispatch(command); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}

	return file("dbg85.exe")
}

// TestFortranDebugQualifier: FORTRAN/DEBUG/OBJECT by itself makes what the debugger
// needs, with no MACRO command and no /DEBUG on it, as the MACRO/DEBUG of
// compiledImage does.
func TestFortranDebugQualifier(t *testing.T) {
	c := stoppedAt(t, buildCompiled(t, func(file func(string) string) []string {
		return []string{`FORTRAN "` + file("dbg85.for") + `"/HOST/NOOPTIMIZE/DEBUG/OBJECT`}
	}), "TXT85")

	if got, want := examineOutput(t, c, "EXAMINE DBG85.I"), "DBG85\\DBG85.I:  0000000B\n"; got != want {
		t.Errorf("EXAMINE DBG85.I: got %q, want %q", got, want)
	}

	if out := say(t, c, "SHOW MODULE"); !strings.Contains(out, "DBG85") || !strings.Contains(out, "MACRO") {
		t.Errorf("SHOW MODULE: %s", out)
	}
}

// fortranDebugSource is a program with a variable of each kind, and a routine with each
// kind of dummy argument, stopped at by TestFortranVariables.
const fortranDebugSource = `      PROGRAM FV
      REAL X, V(3)
      DOUBLE PRECISION DD
      INTEGER*2 H
      INTEGER I, M(2,3), L0(-1:1)
      LOGICAL T, F
      COMPLEX Z
      CHARACTER*8 NAME, NAMES(2)
      COMMON /BLK/ IC, RC
      V(1) = 1.5
      V(2) = 2.5
      V(3) = 3.5
      X = 0.25
      DD = 2.5D0
      H = 7
      I = 11
      T = .TRUE.
      F = .FALSE.
      Z = (1.5, -2.5)
      NAME = 'ABCDEFGH'
      NAMES(2) = 'SECOND'
      IC = 42
      RC = 9.5
      M(1,1) = 11
      M(2,1) = 21
      M(2,3) = 23
      L0(-1) = 5
      CALL SUB(V, 3, X, NAME, M)
      END
      SUBROUTINE SUB(A, N, TOTAL, S, ASSUME)
      REAL A(N), TOTAL
      CHARACTER*(*) S
      INTEGER ASSUME(2,*), LOCAL
      AUTOMATIC IAUTO
      IAUTO = 77
      LOCAL = 5
      TOTAL = A(2) + A(3)
      S(1:1) = 'Z'
      CALL NEST(TOTAL)
      RETURN
      END
      SUBROUTINE NEST(P)
      REAL P
      P = P + 1
      CALL OTHER(P, 'TEXT')
      RETURN
      END
      SUBROUTINE FIRST(Q)
      REAL Q, R
      CHARACTER*(*) C
      R = Q
      RETURN
      ENTRY OTHER(Q, C)
      R = Q + 1
      RETURN
      END
`

// fortranDebugSession stops FORTRAN/DEBUG's program in a routine.
func fortranDebugSession(t *testing.T, at string) *console.Console {
	t.Helper()

	c := newRunnableConsole(t)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	dir := t.TempDir()
	file := func(name string) string { return filepath.Join(dir, name) }

	if err := os.WriteFile(file("fv.for"), []byte(fortranDebugSource), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{
		`FORTRAN "` + file("fv.for") + `"/HOST/NOOPTIMIZE/DEBUG/OBJECT`,
		`LINK "` + file("fv.obj") + `"/HOST/DEBUG/NOSYSLIB/EXECUTABLE="` + file("fv.exe") + `"`,
	} {
		if err := d.Dispatch(command); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}

	if at == "" {
		started, _ := runImage(t, file("fv.exe"), machine.RunOptions{Debug: machine.DebugOn})

		return started
	}

	return stoppedAt(t, file("fv.exe"), at)
}

// variableOutput is EXAMINE's output with the blanks after each name made two, since
// how far a value is from its name depends on the name's length, not on what's being tested.
func variableOutput(t *testing.T, c *console.Console, line string) string {
	t.Helper()

	return regexp.MustCompile(`: +`).ReplaceAllString(examineOutput(t, c, line), ":  ")
}

// TestFortranVariables holds EXAMINE of each kind of variable, from the routine it's in
// and by its unit's name from elsewhere, each shown as its own type.
func TestFortranVariables(t *testing.T) {
	c := fortranDebugSession(t, "NEST")

	dispatch(t, c, "SET RADIX DECIMAL")

	for _, want := range []struct{ line, output string }{
		// The caller's, by the unit's name.
		{"EXAMINE FV.X", "FV\\FV.X:  6.000000\n"},
		{"EXAMINE FV.DD", "FV\\FV.DD:  2.500000000000000\n"},
		{"EXAMINE FV.H", "FV\\FV.H:  7\n"},
		{"EXAMINE FV.I", "FV\\FV.I:  11\n"},
		{"EXAMINE FV.T", "FV\\FV.T:  .TRUE.\n"},
		{"EXAMINE FV.F", "FV\\FV.F:  .FALSE.\n"},
		{"EXAMINE FV.Z", "FV\\FV.Z:  (1.500000,-2.500000)\n"},
		{"EXAMINE FV.NAME", "FV\\FV.NAME:  'ZBCDEFGH'\n"},
		{"EXAMINE FV.NAME(2:4)", "FV\\FV.NAME(2:4):  'BCD'\n"},
		{"EXAMINE FV.NAMES(2)", "FV\\FV.NAMES(2):  'SECOND  '\n"},
		{"EXAMINE FV.NAMES(2)(1:3)", "FV\\FV.NAMES(2)(1:3):  'SEC'\n"},
		{"EXAMINE FV.IC", "FV\\FV.IC:  42\n"},
		{"EXAMINE FV.RC", "FV\\FV.RC:  9.500000\n"},
		// Arrays, in FORTRAN's bounds and order.
		{"EXAMINE FV.V(2)", "FV\\FV.V(2):  2.500000\n"},
		{"EXAMINE FV.M(2,3)", "FV\\FV.M(2,3):  23\n"},
		{"EXAMINE FV.M(2,1)", "FV\\FV.M(2,1):  21\n"},
		{"EXAMINE FV.L0(-1)", "FV\\FV.L0(-1):  5\n"},
		{"EXAMINE FV.V", "FV\\FV.V(1):  1.500000\nFV\\FV.V(2):  2.500000\nFV\\FV.V(3):  3.500000\n"},
		// A bare name is the routine's, a dummy argument found through its frame.
		{"EXAMINE P", "FV\\NEST.P:  6.000000\n"},
		// The routine that called it, by the unit's name, through its own frame.
		{"EXAMINE SUB.N", "FV\\SUB.N:  3\n"},
		{"EXAMINE SUB.A(3)", "FV\\SUB.A(3):  3.500000\n"},
		{"EXAMINE SUB.A", "FV\\SUB.A(1):  1.500000\nFV\\SUB.A(2):  2.500000\nFV\\SUB.A(3):  3.500000\n"},
		{"EXAMINE SUB.S", "FV\\SUB.S:  'ZBCDEFGH'\n"},
		{"EXAMINE SUB.S(1:1)", "FV\\SUB.S(1:1):  'Z'\n"},
		{"EXAMINE SUB.ASSUME(1,1)", "FV\\SUB.ASSUME(1,1):  11\n"},
		{"EXAMINE SUB.ASSUME(2,3)", "FV\\SUB.ASSUME(2,3):  23\n"},
		{"EXAMINE SUB.LOCAL", "FV\\SUB.LOCAL:  5\n"},
		{"EXAMINE SUB.IAUTO", "FV\\SUB.IAUTO:  77\n"},
		// EVALUATE gives what it holds.
		{"EVALUATE FV.X", "6.000000\n"},
		{"EVALUATE FV.M(2,3)", "23\n"},
		// A qualifier still decides how it's shown, under the variable's name.
		{"EXAMINE/LONGWORD/HEXADECIMAL FV.I", "FV\\FV.I:  0000000B\n"},
		{"EXAMINE/ASCII:3 FV.NAME", "FV\\FV.NAME:  'ZBC'\n"},
	} {
		if got := variableOutput(t, c, want.line); got != want.output {
			t.Errorf("%s: got %q, want %q", want.line, got, want.output)
		}
	}

	for _, want := range []struct{ line, says string }{
		{"EXAMINE FV.M(1)", "too few subscripts, array has 2 dimensions"},
		{"EXAMINE FV.V(1,2)", "too many subscripts, array has 1 dimension"},
		{"EXAMINE FV.V(4)", "subscript 1 is out of bounds, value is 4, bounds are 1:3"},
		{"EXAMINE FV.NAME(7:9)", "out of bounds"},
		{"EXAMINE FV.NOSUCH", "FV.NOSUCH"},
	} {
		if _, err := sayErr(c, want.line); err == nil || !strings.Contains(err.Error(), want.says) {
			t.Errorf("%s: %v, want %q", want.line, err, want.says)
		}
	}

	// A DEPOSIT reads the value as the variable's type.
	for _, want := range []struct{ line, examine, output string }{
		{"DEPOSIT FV.X=6.5", "EXAMINE FV.X", "FV\\FV.X:  6.500000\n"},
		{"DEPOSIT FV.V(2)=0.5", "EXAMINE FV.V(2)", "FV\\FV.V(2):  0.5000000\n"},
		{"DEPOSIT FV.M(1,1)=99", "EXAMINE FV.M(1,1)", "FV\\FV.M(1,1):  99\n"},
		{"DEPOSIT FV.DD=0.25", "EXAMINE FV.DD", "FV\\FV.DD:  0.2500000000000000\n"},
		{"DEPOSIT FV.F=.TRUE.", "EXAMINE FV.F", "FV\\FV.F:  .TRUE.\n"},
		{"DEPOSIT FV.NAME='Hi'", "EXAMINE FV.NAME", "FV\\FV.NAME:  'Hi      '\n"},
		{"DEPOSIT FV.NAME(2:3)='xy'", "EXAMINE FV.NAME", "FV\\FV.NAME:  'Hxy     '\n"},
		{"DEPOSIT P=2.5", "EXAMINE P", "FV\\NEST.P:  2.500000\n"},
		{"DEPOSIT SUB.N=2", "EXAMINE SUB.N", "FV\\SUB.N:  2\n"},
	} {
		dispatch(t, c, want.line)

		if got := variableOutput(t, c, want.examine); got != want.output {
			t.Errorf("%s: got %q, want %q", want.line, got, want.output)
		}
	}

	// In an expression EVALUATE takes a name's value, and a location's address, as with a
	// MACRO label; so too for a bare name, which is no MACRO symbol.
	dispatch(t, c, "DEPOSIT SUB.N=3")

	for _, want := range []struct{ line, output string }{
		{"EVALUATE FV.I + 1", "12\n"},
		{"EVALUATE SUB.N + 1", "4\n"},
	} {
		if got := examineOutput(t, c, want.line); got != want.output {
			t.Errorf("%s: %q, want %q", want.line, got, want.output)
		}
	}
}

// TestFortranVariablesEntry: in a routine stopped at through an ENTRY statement, a dummy
// argument is found through the cell the entry point filled in, a CHARACTER one by its
// descriptor, and the routine is the unit's whichever name it was called by.
func TestFortranVariablesEntry(t *testing.T) {
	// Past the two MOVLs, of eight bytes each, that fill the cells.
	c := fortranDebugSession(t, "OTHER+12")

	dispatch(t, c, "SET RADIX DECIMAL")

	for _, want := range []struct{ line, output string }{
		{"EXAMINE Q", "FV\\FIRST.Q:  7.000000\n"},
		{"EXAMINE FIRST.Q", "FV\\FIRST.Q:  7.000000\n"},
		{"EXAMINE C", "FV\\FIRST.C:  'TEXT'\n"},
		{"EXAMINE C(2:3)", "FV\\FIRST.C(2:3):  'EX'\n"},
	} {
		if got := variableOutput(t, c, want.line); got != want.output {
			t.Errorf("%s: got %q, want %q", want.line, got, want.output)
		}
	}
}

// TestFortranStatements holds the labels on FORTRAN's lines, the FORTRAN line at a stop,
// and a step going a line of FORTRAN at a time. Line 37 is TOTAL = A(2) + A(3), in SUB.
func TestFortranStatements(t *testing.T) {
	c := fortranDebugSession(t, "SUB.$LINE37")
	noUserStep(c)

	at := say(t, c, "EXAMINE/INSTRUCTION .PC")
	if !strings.Contains(at, "SUB.$LINE37") {
		t.Errorf("at SUB.$LINE37: %q", at)
	}

	out := say(t, c, "STEP")
	if !strings.Contains(out, "FORTRAN line 38:") || !strings.Contains(out, "S(1:1) = 'Z'") {
		t.Errorf("a step should go to line 38: %q", out)
	}

	out = say(t, c, "STEP/INSTRUCTION")
	if !strings.Contains(out, "FORTRAN line 38:") {
		t.Errorf("a step by instruction stays with its line: %q", out)
	}

	// A bare label is the current routine's, and a break there says where it is.
	say(t, c, "SET BREAK $LINE40")

	out = say(t, c, "GO")
	if !strings.Contains(out, "SUB.$LINE40") || !strings.Contains(out, "FORTRAN line 40:") || !strings.Contains(out, "RETURN") {
		t.Errorf("break at $LINE40: %q", out)
	}
}

// TestFortranLoopRegister: a variable /OPTIMIZE keeps in a register during a loop says so.
func TestFortranLoopRegister(t *testing.T) {
	c := newRunnableConsole(t)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	dir := t.TempDir()
	file := func(name string) string { return filepath.Join(dir, name) }

	source := `      PROGRAM LR
      INTEGER I, S
      S = 0
      DO 10 I = 1, 5
        S = S + I
 10   CONTINUE
      IF (S .GT. 0) CALL DONE(S)
      END
      SUBROUTINE DONE(K)
      RETURN
      END
`

	if err := os.WriteFile(file("lr.for"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{
		`FORTRAN "` + file("lr.for") + `"/HOST/DEBUG/OBJECT`,
		`LINK "` + file("lr.obj") + `"/HOST/DEBUG/NOSYSLIB/EXECUTABLE="` + file("lr.exe") + `"`,
	} {
		if err := d.Dispatch(command); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}

	c = stoppedAt(t, file("lr.exe"), "DONE")

	dispatch(t, c, "SET RADIX DECIMAL")

	if got := say(t, c, "EXAMINE LR.S"); !strings.Contains(got, "during a loop, R") {
		t.Errorf("EXAMINE LR.S: %q", got)
	}
}

// TestFortranShowModule: SHOW MODULE is the current image's, and /SHARE names the rest. A
// program using FTNRTL doesn't list FTNRTL's modules (the survey's D8).
func TestFortranShowModule(t *testing.T) {
	c := newRunnableConsole(t)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	dir := t.TempDir()
	file := func(name string) string { return filepath.Join(dir, name) }

	if err := os.WriteFile(file("rt.for"), []byte("      PRINT *, 1\n      END\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{
		`FORTRAN "` + file("rt.for") + `"/HOST/DEBUG/NOOPTIMIZE/OBJECT`,
		`LINK "` + file("rt.obj") + `"/HOST/DEBUG/EXECUTABLE="` + file("rt.exe") + `"`,
	} {
		if err := d.Dispatch(command); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}

	started, _ := runImage(t, file("rt.exe"), machine.RunOptions{Debug: machine.DebugOn})

	out := say(t, started, "SHOW MODULE")
	if strings.Contains(out, "FTN_") || !strings.Contains(out, "total MACRO modules: 1.") {
		t.Errorf("SHOW MODULE:\n%s", out)
	}

	out = say(t, started, "SHOW MODULE/SHARE")
	if !strings.Contains(out, "SHARE$FTNRTL") || !strings.Contains(out, "Image") || strings.Contains(out, "FTN_") {
		t.Errorf("SHOW MODULE/SHARE:\n%s", out)
	}

	say(t, started, "SET IMAGE FTNRTL")

	if out = say(t, started, "SHOW MODULE"); !strings.Contains(out, "FTN_") {
		t.Errorf("SHOW MODULE of FTNRTL:\n%s", out)
	}

	if out = say(t, started, "SHOW IMAGE"); !strings.Contains(out, "*FTNRTL") {
		t.Errorf("SHOW IMAGE:\n%s", out)
	}

	if _, err := sayErr(started, "SET IMAGE NOSUCH"); err == nil {
		t.Error("SET IMAGE NOSUCH: no error")
	}

	if out = say(t, started, "SHOW MODULE FTN_F*"); !strings.Contains(out, "FTN_FORMAT") || strings.Contains(out, "FTN_UNITS") {
		t.Errorf("SHOW MODULE FTN_F*:\n%s", out)
	}
}

// TestFortranVariablesNoFrame: what needs a frame says so when there's none.
func TestFortranVariablesNoFrame(t *testing.T) {
	c := fortranDebugSession(t, "")

	if _, err := sayErr(c, "EXAMINE SUB.N"); err == nil || !strings.Contains(err.Error(), "SUB.N has no active call frame") {
		t.Errorf("EXAMINE SUB.N: %v", err)
	}

	// A bare name in the main program, and the static variable of a routine not running.
	dispatch(t, c, "SET RADIX DECIMAL")

	if got := variableOutput(t, c, "EVALUATE I + 1"); got == "" {
		t.Error("EVALUATE I + 1 said nothing")
	}

	if got := examineOutput(t, c, "EXAMINE I"); !strings.HasPrefix(got, "FV\\FV.I:") {
		t.Errorf("EXAMINE I: %q", got)
	}

	if got := examineOutput(t, c, "EXAMINE SUB.LOCAL"); !strings.HasPrefix(got, "FV\\SUB.LOCAL:") {
		t.Errorf("EXAMINE SUB.LOCAL: %q", got)
	}
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
