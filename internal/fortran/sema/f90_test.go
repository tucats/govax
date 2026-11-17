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

// TestConstantFormatQuotes holds a format given as a CHARACTER constant to
// what TestFormatQuotes holds a FORMAT statement to (Phase 101, subtask 5,
// where examples.f90's WRITE (*, '(" QED")') found it): under /F90 its
// character constants between quotation marks reach FTNRTL between
// apostrophes, a PARAMETER's value too; text after the last parenthesis stays;
// a variable's format, a format that doesn't read, and every format without
// /F90 are left as written.
func TestConstantFormatQuotes(t *testing.T) {
	cases := []struct {
		f90    bool
		format string
		want   string
	}{
		{true, `'(" QED")'`, `(' QED')`},
		{true, `'(1X, "it''s ""so""", I3) and more'`, `(1X, 'it''s "so"', I3) and more`},
		{true, `Q`, `(1X, 'from a PARAMETER')`},
		{true, `V`, ``},
		{true, `'(1X, "unended)'`, `(1X, "unended)`},
		{true, `'(1X, ''plain'')'`, `(1X, 'plain')`},
		{false, `'(" QED")'`, `(" QED")`},
	}

	for _, c := range cases {
		lines := []string{"      PROGRAM P", "      CHARACTER*20 V", "      CHARACTER*(*) Q",
			`      PARAMETER (Q = '(1X, "from a PARAMETER")')`, "      V = '(1X)'",
			"      WRITE (6, " + c.format + ")", "      END"}
		if c.f90 {
			lines = append([]string{"      OPTIONS /F90"}, lines...)
		}

		prog, messages := analyzeOpts(t, Options{}, lines...)
		if len(messages) != 0 {
			t.Errorf("%s: messages %v", c.format, messages)

			continue
		}

		var io *IO

		for _, s := range prog.Units[0].Body {
			if found, ok := s.(*IO); ok {
				io = found
			}
		}

		if io == nil {
			t.Fatalf("%s: no WRITE", c.format)
		}

		got := ""
		if k, ok := io.RunFormat.(*Const); ok {
			got = k.Text
		}

		if got != c.want {
			t.Errorf("%s (F90 %v): FTNRTL gets %q, want %q", c.format, c.f90, got, c.want)
		}
	}
}
