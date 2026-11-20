package lex

import (
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/source"
)

// lexF90 reads lines in the given form, with /F90 as given, and lexes them,
// returning each statement's dump and the messages, each as its identifier
// and column.
func lexF90(t *testing.T, free, f90 bool, lines ...string) ([]*Statement, []string, []string) {
	t.Helper()

	var messages diag.List

	spec := "T.FOR"
	if free {
		spec = "T.F90"
	}

	program := source.Read(source.Form{F90: f90}, &source.File{Spec: spec, Lines: lines, Free: free}, nil, &messages)
	if messages.Failed() {
		t.Fatalf("the source pass failed: %v", messages.Messages())
	}

	statements := Lex(program.Statements, &messages)

	dumps := make([]string, len(statements))
	for i, s := range statements {
		dumps[i] = Dump(s)
	}

	var found []string
	for _, m := range messages.Messages() {
		found = append(found, m.Ident+"@"+itoa(m.Pos.Column))
	}

	return statements, dumps, found
}

func itoa(n int) string {
	return string(rune('0'+n/100%10)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}

// TestF90Tokens checks Fortran 90's new tokens, read the same in either form
// and with or without /F90 (docs/planning/PHASE-99 - Fortran 90.md, subtask 4).
func TestF90Tokens(t *testing.T) {
	cases := []struct{ text, want string }{
		// The new operators, each one token, and an = in one no assignment.
		{"X = A == B", "assignment: X = A == B"},
		{"PRINT *, A == B", "PRINT: * , A == B"},
		{"PRINT *, A /= B, A <= B, A >= B, A < B, A > B", "PRINT: * , A /= B , A <= B , A >= B , A < B , A > B"},
		{"IF (A /= B) X = 1", "logical IF: ( A /= B ) / assignment: X = 1"},
		{"V = (/ 1, 2, 3 /)", "assignment: V = (/ 1 , 2 , 3 /)"},
		{"V = (/ (I, I = 1, 3) /)", "assignment: V = (/ ( I , I = 1 , 3 ) /)"},
		{"INTEGER :: I", "INTEGER: :: I"},
		{"P => X", "pointer assignment: P => X"},
		{"Y = X%A", "assignment: Y = X % A"},

		// Kind parameters.
		{"X = 1.5_8 + 2_K - 3E2_4", "assignment: X = 1.5_8 + 2_K - 3E2_4"},
		{"L = .TRUE._1 .OR. .FALSE._K", "assignment: L = .TRUE._1 .OR. .FALSE._K"},
		{"C = K_'TEXT' // 1_'MORE'", "assignment: C = K_'TEXT' // 1_'MORE'"},
		{"CALL F(K_'TEXT', 1_'X')", "CALL: F ( K_'TEXT' , 1_'X' )"},

		// Binary, octal, and hexadecimal constants, their letter first, at
		// level 0 and inside parentheses; DEC's forms as before.
		{"DATA I, J, K /B'101', O'17', Z'1F'/", "DATA: I , J , K / B'101' , O'17' , Z'1F' /"},
		{"CALL F(Z'1F', B'10', '1F'X, '17'O)", "CALL: F ( Z'1F' , B'10' , '1F'X , '17'O )"},

		// The record separator still follows a unit named B, O, or Z.
		{"READ (Z'10) X", "READ: ( Z ' 10 ) X"},
		{"READ (B'1+N) X", "READ: ( B ' 1 + N ) X"},

		// Defined operators, where no field reference can stand; after a
		// name or ), the periods stay separate for the parser.
		{"X = .INVERSE. A", "assignment: X = .INVERSE. A"},
		{"X = 2.CROSS.Y", "assignment: X = 2 .CROSS. Y"},
		{"X = (.NEG.A) + 1.TIMES.B", "assignment: X = ( .NEG. A ) + 1 .TIMES. B"},
		{"X = A.B.C", "assignment: X = A . B . C"},
		{"X = R(1).F.EQ.2", "assignment: X = R ( 1 ) . F .EQ. 2"},

		// The period closing such a word after a name or ) closes it, and
		// doesn't start a number (R .AT. 2.0, Phase 106's subtask 7).
		{"X = R.AT.2.0_8", "assignment: X = R . AT . 2.0_8"},
		{"X = F(Y).AT.2", "assignment: X = F ( Y ) . AT . 2"},
		{"X = R.AT..5", "assignment: X = R . AT . .5"},

		// Old numbers are read as before.
		{"X = 1.5 + 2. + .5 + 1E3 + 2.5D0", "assignment: X = 1.5 + 2. + .5 + 1E3 + 2.5D0"},
		{"IF (I.EQ.1.AND.J.EQ.2) X = 1.E5", "logical IF: ( I .EQ. 1 .AND. J .EQ. 2 ) / assignment: X = 1.E5"},
	}

	for _, c := range cases {
		for _, free := range []bool{false, true} {
			text := c.text
			if !free {
				text = fixed(text)
			}

			_, dumps, found := lexF90(t, free, free, text)
			if len(dumps) != 1 || dumps[0] != c.want || len(found) != 0 {
				t.Errorf("%s (free %v):\n got %q %v\nwant %q", c.text, free, dumps, found, c.want)
			}
		}
	}
}

// TestF90TokenValues checks what Dump doesn't show: kinds, bases, and values.
func TestF90TokenValues(t *testing.T) {
	statements, _, found := lexF90(t, true, true,
		`c = "IT'S" // "SAY ""HI""" // K_"Q" // z'ff' // 'A"B'`)
	if len(found) != 0 {
		t.Fatalf("messages: %v", found)
	}

	var got []string
	for _, tok := range statements[0].Tokens {
		got = append(got, tok.String())
	}

	want := []string{
		"name C", "operator =", `character "IT'S"`, "operator //", `character "SAY \"HI\""`,
		"operator //", `character "Q" kind K`, "operator //", `typeless "ff" radix 16`, "operator //", `character "A\"B"`,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokens:\n got %q\nwant %q", got, want)
	}

	if tok := statements[0].Tokens[8]; !tok.Prefixed {
		t.Errorf("z'ff' isn't Prefixed: %+v", tok)
	}

	if tok := statements[0].Tokens[6]; tok.Offset != len("c = \"IT'S\" // \"SAY \"\"HI\"\"\" // ") {
		t.Errorf("K_\"Q\" is at offset %d", tok.Offset)
	}
}

// TestF90Quotes checks the quotation mark: a character constant under /F90,
// appendix A.5's octal constant without it, in either form.
func TestF90Quotes(t *testing.T) {
	cases := []struct {
		free, f90 bool
		want      string
	}{
		{free: false, f90: false, want: `assignment: I = "17 + 1`},
		{free: false, f90: true, want: `assignment: I = '17' + 1`},
		{free: true, f90: false, want: `assignment: I = "17 + 1`},
		{free: true, f90: true, want: `assignment: I = '17' + 1`},
	}

	for _, c := range cases {
		text := `I = "17" + 1`
		if !c.f90 {
			text = `I = "17 + 1`
		}

		if !c.free {
			text = fixed(text)
		}

		_, dumps, found := lexF90(t, c.free, c.f90, text)
		if len(dumps) != 1 || dumps[0] != c.want || len(found) != 0 {
			t.Errorf("free %v, F90 %v: got %q %v, want %q", c.free, c.f90, dumps, found, c.want)
		}
	}

	// An unclosed constant says which mark it lacks.
	var messages diag.List

	program := source.Read(source.Form{F90: true}, &source.File{Spec: "T.F90", Lines: []string{`c = "abc`}, Free: true}, nil, &messages)
	Lex(program.Statements, &messages)

	if ms := messages.Messages(); len(ms) != 1 || ms[0].Text != "character constant has no closing quotation mark" {
		t.Errorf("unclosed: %v", ms)
	}
}

// TestFreeFormBlanks checks free form's rules for blanks (the standard's
// 3.3.1): none inside a name, keyword, constant, or operator, and one between
// two words. Fixed form has neither rule.
func TestFreeFormBlanks(t *testing.T) {
	cases := []struct {
		text string
		want []string // messages, as ident@column
	}{
		{"x = a + b", nil},
		{"x = ab c", []string{"BLANKINNAME@008"}},
		{"x = 1 .5", []string{"BLANKINNAME@007"}},
		{"x = 1.5 _8", []string{"BLANKINNAME@009"}},
		{"l = a . eq. b", []string{"BLANKINNAME@009"}},
		{"l = a = = b", []string{"BLANKINNAME@009"}},
		{"v = ( / 1 / )", []string{"BLANKINNAME@007", "BLANKINNAME@013"}},
		{"call sub(x)", nil},
		{"callsub(x)", []string{"NEEDBLANK@005"}},
		{"goto10", []string{"NEEDBLANK@005"}},
		{"go to 10", nil},
		{"if (x) goto 10", nil},
		{"do 10 i = 1, 5", nil},
		{"do10 i = 1, 5", []string{"NEEDBLANK@003"}},
		{"do 10i = 1, 5", []string{"NEEDBLANK@006"}},
		{"integer*2 i, j", nil},
		{"real*8d1", []string{"NEEDBLANK@007"}},
		{"implicit double precision (a-h)", nil},
		{"implicit double complex (z)", nil},
		{"print *, 'a b' // 'c'", nil},
		{"c = k_ 'text'", []string{"BLANKINNAME@008"}},
		{"i = z '1f'", []string{"BLANKINNAME@007"}},
		{"assign 10 to lab", nil},
		{"assign 10 tolab", []string{"NEEDBLANK@013"}},
		{"assign 10 t o lab", []string{"BLANKINNAME@013"}},
	}

	for _, c := range cases {
		_, dumps, found := lexF90(t, true, true, c.text)
		if !reflect.DeepEqual(found, c.want) {
			t.Errorf("%s: got %v, want %v (%q)", c.text, found, c.want, dumps)
		}

		// The same text in fixed form reads without a message.
		if _, _, found := lexF90(t, false, true, fixed(c.text)); len(found) != 0 {
			t.Errorf("%s in fixed form: %v", c.text, found)
		}
	}
}

// TestBadKind checks a kind parameter that is neither digits nor a name.
func TestBadKind(t *testing.T) {
	_, _, found := lexF90(t, true, true, "x = 1.5_ + 1")
	if !reflect.DeepEqual(found, []string{"BADKIND@008"}) {
		t.Errorf("got %v", found)
	}
}
