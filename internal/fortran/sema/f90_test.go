package sema

import "testing"

// TestUnitF90 checks /F90 and OPTIONS's /[NO]F90: each unit records its own, and
// neither OPTIONS qualifier is NOTIMPL (docs/planning/PHASE-99 - Fortran 90.md,
// subtask 2).
func TestUnitF90(t *testing.T) {
	lines := []string{
		"      PROGRAM P", "      END",
		"      OPTIONS /F90", "      SUBROUTINE S", "      END",
		"      OPTIONS /NOF90", "      SUBROUTINE T", "      END",
	}

	cases := []struct {
		opts Options
		want []bool
	}{
		{Options{}, []bool{false, true, false}},
		{Options{F90: true}, []bool{true, true, false}},
	}

	for _, c := range cases {
		prog, messages := analyzeOpts(t, c.opts, lines...)
		if len(messages) != 0 {
			t.Fatalf("F90 %v: messages %v", c.opts.F90, messages)
		}

		for i, u := range prog.Units {
			if u.F90 != c.want[i] {
				t.Errorf("F90 %v: unit %s's F90 is %v, want %v", c.opts.F90, u.Name, u.F90, c.want[i])
			}
		}
	}
}

// TestFormatQuotes holds a FORMAT statement's character constants between
// quotation marks to /F90 (Phase 99, subtask 6): accepted under it, and given
// to FTNRTL between apostrophes, its own apostrophes doubled and its doubled
// quotation marks made one, after a variable format expression's number;
// without /F90 still FMTSYNTAX.
func TestFormatQuotes(t *testing.T) {
	format := `   10 FORMAT (1X, "N =", I<N>, " it's ""so""", 2('a', "b"))`

	prog, messages := analyzeOpts(t, Options{}, "      OPTIONS /F90", "      PROGRAM P", "      N = 3",
		"      WRITE (6, 10) N", format, "      END")
	if len(messages) != 0 {
		t.Fatalf("under /F90: messages %v", messages)
	}

	want := `(1X, 'N =', I<1>, ' it''s "so"', 2('a', 'b'))`
	if got := prog.Units[0].Formats[10].Encoded; got != want {
		t.Errorf("under /F90: encoded\n%s\nwant\n%s", got, want)
	}

	_, messages = analyzeOpts(t, Options{}, "      PROGRAM P", "      N = 3",
		"      WRITE (6, 10) N", format, "      END")
	if len(messages) != 1 || messages[0].Ident != "FMTSYNTAX" {
		t.Errorf("without /F90: messages %v, want one FMTSYNTAX", messages)
	}
}
