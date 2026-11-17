package source

import (
	"slices"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
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

// TestFreeFormNotYet checks that a free-form file is read as lines and reported
// NOTIMPL at its first line, until subtask 3's reader; a fixed-form file it
// includes, or one beside it, is unaffected.
func TestFreeFormNotYet(t *testing.T) {
	var messages diag.List

	free := &File{Spec: "A.F90", Lines: []string{"program a", "end program a"}, Free: true}
	fixed := &File{Spec: "B.FOR", Lines: []string{"      SUBROUTINE B", "      END"}}

	p := ReadAll(Form{}, []*File{free, fixed}, nil, &messages)

	got := messages.Messages()
	if len(got) != 1 || got[0].Ident != "NOTIMPL" || got[0].Pos.File != "A.F90" || got[0].Pos.Line != 1 {
		t.Fatalf("messages %v, want one NOTIMPL at A.F90's line 1", got)
	}

	if len(p.Lines) != 4 {
		t.Errorf("%d lines kept, want 4", len(p.Lines))
	}

	if got := texts(p); !slices.Equal(got, []string{"0|SUBROUTINE B", "0|END"}) {
		t.Errorf("statements %q, want B.FOR's only", got)
	}
}
