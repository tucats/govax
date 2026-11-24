package debugger_test

import (
	"os"
	"strings"
	"testing"
)

// This file holds the debugger counting a FORTRAN program's lines as the FORTRAN source
// does, where it counted the MACRO file's (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 9). In fortranDebugSource line 28 is FV's CALL SUB, 37 SUB's TOTAL = A(2) +
// A(3), 39 its CALL NEST, and 40 its RETURN.

// TestFortranLineNumbers: %LINE n, SET BREAK at one, SHOW CALLS's line column, TYPE, and
// EXAMINE/SOURCE are FORTRAN lines.
func TestFortranLineNumbers(t *testing.T) {
	c := fortranDebugSession(t, "SUB.$LINE37")
	noUserStep(c)

	say(t, c, "SET RADIX DECIMAL")

	// %LINE 38 is FORTRAN's line 38, S(1:1) = 'Z', which begins with its label.
	if out := say(t, c, "EVALUATE %LINE 38 - SUB.$LINE38"); out != "0\n" {
		t.Errorf("%%LINE 38 less SUB.$LINE38: %q", out)
	}

	if out := say(t, c, "EXAMINE/INSTRUCTION %LINE 38"); !strings.HasPrefix(out, `FV\SUB\%LINE 38:`) {
		t.Errorf("EXAMINE/INSTRUCTION %%LINE 38: %q", out)
	}

	// SHOW CALLS: SUB at line 37, FV at its CALL, line 28.
	calls := say(t, c, "SHOW CALLS")
	for _, row := range []string{"SUB                 37", "FV                  28"} {
		if !strings.Contains(calls, row) {
			t.Errorf("SHOW CALLS lacks %q:\n%s", row, calls)
		}
	}

	if out := say(t, c, "EXAMINE/SOURCE .PC"); out != "module FV\n    37:       TOTAL = A(2) + A(3)\n" {
		t.Errorf("EXAMINE/SOURCE .PC: %q", out)
	}

	// TYPE shows every line of the source, declarations too, and TYPE alone the next.
	if out := say(t, c, "TYPE 33:34"); out != "module FV\n    33:       INTEGER ASSUME(2,*), LOCAL\n    34:       AUTOMATIC IAUTO\n" {
		t.Errorf("TYPE 33:34: %q", out)
	}

	if out := say(t, c, "TYPE"); out != "module FV\n    35:       IAUTO = 77\n" {
		t.Errorf("TYPE: %q", out)
	}

	// A break at a FORTRAN line, and SHOW CALLS there.
	say(t, c, "SET BREAK %LINE 40")

	if out := say(t, c, "GO"); !strings.Contains(out, "FORTRAN line 40:") {
		t.Errorf("GO to %%LINE 40: %q", out)
	}

	if out := say(t, c, "SHOW CALLS"); !strings.Contains(out, "SUB                 40") {
		t.Errorf("SHOW CALLS at line 40:\n%s", out)
	}

	// SET BREAK/LINE stops at each FORTRAN line, not at each MACRO line: after
	// RETURN, FV's END, line 29.
	say(t, c, "CANCEL BREAK/ALL")
	say(t, c, "SET BREAK/LINE")

	if out := say(t, c, "GO"); !strings.Contains(out, "FORTRAN line 29:") {
		t.Errorf("SET BREAK/LINE, GO from line 40: %q", out)
	}
}

// TestFortranLinesWithoutSource: with the FORTRAN file gone, TYPE shows the lines the
// MACRO file has as comments, which are the same.
func TestFortranLinesWithoutSource(t *testing.T) {
	c := fortranDebugSession(t, "SUB.$LINE37")

	if err := os.Remove("fv.for"); err != nil {
		t.Fatal(err)
	}

	say(t, c, "SET RADIX DECIMAL")
	say(t, c, "SET SOURCE []") // the files are read again

	want := "module FV\n    33:       INTEGER ASSUME(2,*), LOCAL\n    34:       AUTOMATIC IAUTO\n    35:       IAUTO = 77\n"
	if out := say(t, c, "TYPE 33:35"); out != want {
		t.Errorf("TYPE 33:35: %q", out)
	}

	// The lines are still FORTRAN's.
	if out := say(t, c, "SHOW CALLS"); !strings.Contains(out, "SUB                 37") {
		t.Errorf("SHOW CALLS:\n%s", out)
	}
}
