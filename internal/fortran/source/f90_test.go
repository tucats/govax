package source

import (
	"slices"
	"testing"
)

// TestStatementF90 checks that each statement records /F90 as the command gave it
// and an OPTIONS statement changes it for one unit, as /EXTEND_SOURCE is followed
// (docs/planning/PHASE-99 - Fortran 90.md, subtask 2).
func TestStatementF90(t *testing.T) {
	lines := []string{
		"      PROGRAM P", "      END",
		"      OPTIONS /F90", "      SUBROUTINE S", "      END",
		"      OPTIONS /NOF90", "      SUBROUTINE T", "      END",
		"      SUBROUTINE U", "      END",
	}

	cases := []struct {
		form Form
		want []bool
	}{
		{Form{}, []bool{false, false, false, true, true, false, false, false, false, false}},
		{Form{F90: true}, []bool{true, true, true, true, true, true, false, false, true, true}},
	}

	for _, c := range cases {
		p, idents := read(t, c.form, nil, lines...)
		if len(idents) != 0 {
			t.Errorf("F90 %v: messages %v", c.form.F90, idents)
		}

		var got []bool
		for _, s := range p.Statements {
			got = append(got, s.F90)
		}

		if !slices.Equal(got, c.want) {
			t.Errorf("F90 %v: statements' F90 %v, want %v", c.form.F90, got, c.want)
		}
	}
}

// TestF90QualifierNames checks OPTIONS's /F90 among the qualifiers, cut short too;
// /F and /NOF are still /F77 and /NOF77.
func TestF90QualifierNames(t *testing.T) {
	for name, want := range map[string]string{"F90": "F90", "F9": "F90", "NOF9": "NOF90", "F7": "F77", "F": "F77", "NOF": "NOF77"} {
		if got := OptionsQualifier(name); got != want {
			t.Errorf("OptionsQualifier(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestF90ConstantsInParentheses checks the apostrophes Fortran 90 adds after a
// name character inside parentheses (ApostropheStartsConstant): a kind
// parameter's and a binary, octal, or hexadecimal constant's start a constant,
// so a ! after them starts a comment; READ (Z'10)'s is still the separator.
func TestF90ConstantsInParentheses(t *testing.T) {
	check(t, Form{}, []string{
		"      CALL F(Z'1F') ! hexadecimal",
		"      CALL F(K_'A!B', 1_'!') ! kind parameters",
		"      CALL F(B'101', O'17') ! binary and octal",
		"      READ (Z'10) X ! a record number",
		"      READ (Z'1G') X ! not hexadecimal: both apostrophes separators",
	},
		"0|CALL F(Z'1F') ",
		"0|CALL F(K_'A!B', 1_'!') ",
		"0|CALL F(B'101', O'17') ",
		"0|READ (Z'10) X ",
		"0|READ (Z'1G') X ",
	)

	for _, c := range []struct {
		last  byte
		depth int
		rest  string
		want  bool
	}{
		{'(', 1, "A'", true},
		{'X', 0, "A'", true},
		{'X', 1, "10)", false},
		{'_', 1, "A'", true},
		{'Z', 1, "1F'", true},
		{'z', 1, "1f'", true},
		{'Z', 1, "'", false},
		{'B', 1, "102'", false},
		{'O', 1, "17'", true},
		{'O', 1, "18'", false},
		{'1', 1, "10)", false},
	} {
		if got := ApostropheStartsConstant(c.last, c.depth, c.rest); got != c.want {
			t.Errorf("%q %d %q: got %v", c.last, c.depth, c.rest, got)
		}
	}
}
