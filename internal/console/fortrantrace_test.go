package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// tracedSource is a program whose third routine stops with a fatal condition, so
// that its traceback has a line for each of the three (docs/planning/PHASE-110 -
// Fortran 90 compiler usability.md, subtask 7c). The line numbers the traceback
// should show are each CALL's: 3, 9, and 14.
const tracedSource = `C     A traceback names each routine's FORTRAN line.
      PROGRAM TRACED
      CALL OUTER(3)
      END

      SUBROUTINE OUTER(N)
      INTEGER N, M
      M = N * 2
      CALL INNER(M)
      END

      SUBROUTINE INNER(M)
      PRINT *, 'STOPPING AT', M
      CALL LIB$STOP(%VAL(44))
      END
`

// TestFortranTracebackLines holds a FORTRAN program's traceback to its FORTRAN
// lines: compiled the usual way (the object's traceback records carry the
// FORTRAN lines, internal/asm/fortranline.go), and compiled and linked /DEBUG and
// run /NODEBUG (the MACRO file's lines, mapped through its comments). Assembled
// /DEBUG and linked without it, the object has no line table a LINK keeps, so the
// column is empty, as it was before.
func TestFortranTracebackLines(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]TRACED.FOR"},
		rms.TextRecords, splitSource(tracedSource)); err != nil {
		t.Fatal(err)
	}

	deleteFilesAfter(t, d, "TRACED")

	withLines := map[string]string{"INNER": "14", "OUTER": "9", "TRACED": "3"}
	noLines := map[string]string{"INNER": "", "OUTER": "", "TRACED": ""}

	for _, tc := range []struct {
		name     string
		commands []string
		run      string
		want     map[string]string
	}{
		{"usual", []string{"FORTRAN TRACED", "MACRO TRACED", "LINK TRACED"}, "RUN TRACED", withLines},
		{"no optimizing", []string{"FORTRAN/NOOPTIMIZE TRACED", "MACRO TRACED", "LINK TRACED"}, "RUN TRACED", withLines},
		{"debug", []string{"FORTRAN/DEBUG/NOOPTIMIZE TRACED", "MACRO TRACED", "LINK/DEBUG TRACED"}, "RUN/NODEBUG TRACED", withLines},
		{"debug object, plain link", []string{"FORTRAN/DEBUG/NOOPTIMIZE TRACED", "MACRO TRACED", "LINK TRACED"}, "RUN TRACED", noLines},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, command := range tc.commands {
				buf.Reset()
				shareFindRun(t, d, command)

				if out := buf.String(); out != "" {
					t.Fatalf("%s said:\n%s", command, out)
				}
			}

			buf.Reset()
			_ = d.Dispatch(tc.run)
			out := buf.String()

			if !strings.Contains(out, "%TRACE-F-TRACEBACK") {
				t.Fatalf("no traceback:\n%s", out)
			}

			for routine, line := range tc.want {
				if got, ok := tracebackLine(out, routine); !ok || got != line {
					t.Errorf("%s's line is %q (found %v), want %q, in:\n%s", routine, got, ok, line, out)
				}
			}
		})
	}
}

// tracebackLine returns the line column of a traceback's line for routine of
// module TRACED.
func tracebackLine(out, routine string) (string, bool) {
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 53 || strings.TrimSpace(line[:15]) != "TRACED" || strings.TrimSpace(line[16:47]) != routine {
			continue
		}

		return strings.TrimSpace(line[47:53]), true
	}

	return "", false
}
