package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ir"
)

// oneChar is a subroutine's lowered statements after this pass alone.
func oneChar(t *testing.T, lines ...string) *ir.Routine {
	t.Helper()

	all := append([]string{
		"SUBROUTINE S(I, D)",
		"CHARACTER C, E, S1*5, S2*5, D*(*)",
		"LOGICAL L",
		"COMMON /V/ C, E, S1, S2, L",
	}, lines...)

	r := routineOf(t, "S", append(all, "END")...)
	run([]*ir.Routine{r}, []Pass{{"one character", OneCharacter}})

	return r
}

// TestOneCharacterMoves checks which moves become a byte moved.
func TestOneCharacterMoves(t *testing.T) {
	cases := []struct {
		statement string
		byByte    bool
	}{
		{"C = E", true},
		{"C = S1", true},
		{"S1(2:2) = E", true},
		{"S1(I:I) = S2(I:I)", true},
		{"C = D(I:I)", true},
		{"C = 'X'", true},
		{"C = 'XYZ'", true},
		{"C = S1(1:2)", true},
		{"S1 = E", false},
		{"S1 = S2", false},
		{"C = D", false},
		{"S1(1:I) = E", false},
	}

	for _, c := range cases {
		r := oneChar(t, c.statement)

		if _, isAssign := r.Blocks[0].Stmts[0].(*ir.Assign); isAssign != c.byByte {
			t.Errorf("%s is %s", c.statement, codeText(r))
		}
	}
}

// TestOneCharacterComparisons checks what each comparison's condition becomes, and
// that all but equality are the unsigned ones.
func TestOneCharacterComparisons(t *testing.T) {
	cases := []struct {
		cond string
		want string
	}{
		{"C .EQ. E", "eq.B("},
		{"C .NE. E", "ne.B("},
		{"C .LT. E", "ltu.B("},
		{"C .LE. E", "leu.B("},
		{"C .GT. E", "gtu.B("},
		{"C .GE. E", "geu.B("},
		{"C .EQ. S1", ".T("},
		{"S1 .EQ. S2", ".T("},
		{"C .EQ. D", ".T("},
		{"C .LT. 'AB'", ".T("},
		{".NOT. C .LT. E", "ltu.B("},
		{"L .AND. C .GT. E", "gtu.B("},
		{"C .LT. 'A' .OR. C .GT. 'Z'", "gtu.B("},
	}

	for _, c := range cases {
		code := codeText(oneChar(t, "IF ("+c.cond+") CALL HIT"))

		if !strings.Contains(code, c.want) {
			t.Errorf("%s has no %s:\n%s", c.cond, c.want, code)
		}
	}
}

// TestOneCharacterConstants checks that a constant's text goes unless something else
// uses it.
func TestOneCharacterConstants(t *testing.T) {
	r := oneChar(t, "IF (C .EQ. 'X') CALL HIT")
	for _, d := range r.Data {
		if d.Kind == ir.TextDatum {
			t.Errorf("the text %q is still there", d.Text)
		}
	}

	r = oneChar(t, "IF (C .EQ. 'X') CALL HIT('X')")

	found := false

	for _, d := range r.Data {
		found = found || d.Kind == ir.TextDatum && d.Text == "X"
	}

	if !found {
		t.Error("a text an argument's descriptor uses has gone")
	}

	r = oneChar(t, "WRITE (6,100)", "100:FORMAT (1X)", "IF (C .EQ. 'X') CALL HIT('(1X)')")

	found = false

	for _, d := range r.Data {
		found = found || d.Format != nil
	}

	if !found {
		t.Error("the FORMAT has gone")
	}
}

// TestOneCharacterCode checks the instructions: CMPB and unsigned branches, MOVB, CLRB
// for a NUL, TST of what was just moved kept for an unsigned comparison, as a MOVB
// leaves C as it was, and left out for an equality.
func TestOneCharacterCode(t *testing.T) {
	cases := []struct {
		lines []string
		has   []string
		hasnt []string
	}{
		{[]string{"IF (C .GT. E) CALL HIT"}, []string{"CMPB", "GTRU"}, []string{"CMPC"}},
		{[]string{"C = E"}, []string{"MOVB"}, []string{"MOVC"}},
		{[]string{"C = 'X'"}, []string{"MOVB", "#88"}, []string{"MOVC"}},
		{[]string{"C = CHAR(200)"}, []string{"MOVB", "#-56"}, []string{"MOVC"}},
		{[]string{"C = CHAR(0)"}, []string{"CLRB"}, []string{"MOVC"}},
		{[]string{"C = E", "IF (C .GT. CHAR(0)) CALL HIT"}, []string{"TSTB"}, nil},
		{[]string{"C = E", "IF (C .NE. CHAR(0)) CALL HIT"}, nil, []string{"TSTB"}},
	}

	for _, c := range cases {
		text := macroOf(t, append([]string{
			"SUBROUTINE S",
			"CHARACTER C, E",
			"COMMON /V/ C, E",
		}, append(c.lines, "END")...)...)

		for _, want := range c.has {
			if !strings.Contains(text, want) {
				t.Errorf("%v has no %s:\n%s", c.lines, want, text)
			}
		}

		for _, unwanted := range c.hasnt {
			if strings.Contains(text, unwanted) {
				t.Errorf("%v has %s:\n%s", c.lines, unwanted, text)
			}
		}
	}
}

// TestOneCharacterFolds checks that known bytes compare as unsigned: CHAR(200) is
// a negative byte, and comes after 'A'.
func TestOneCharacterFolds(t *testing.T) {
	cases := []struct {
		cond  string
		holds bool
	}{
		{"C .GT. 'A'", true},
		{"C .LE. 'A'", false},
		{"'A' .LT. C", true},
		{"'A' .GE. C", false},
		{"C .EQ. CHAR(200)", true},
	}

	for _, c := range cases {
		text := macroOf(t, "SUBROUTINE S", "CHARACTER C", "C = CHAR(200)", "IF ("+c.cond+") CALL HIT", "END")

		if strings.Contains(text, "HIT") != c.holds {
			t.Errorf("%s should hold: %t:\n%s", c.cond, c.holds, text)
		}
	}
}
