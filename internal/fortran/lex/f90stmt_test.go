package lex

import (
	"reflect"
	"testing"
)

// TestF90Statements checks that every Fortran 90 statement is classified, in
// both source forms (docs/planning/PHASE-99 - Fortran 90.md, subtask 5): the
// keyword statements, the :: and => rules, WHERE's two forms, construct names,
// and named ENDs.
func TestF90Statements(t *testing.T) {
	cases := []struct{ text, want string }{
		// The keyword statements, each with its words after the keyword.
		{"ALLOCATABLE A(:)", "ALLOCATABLE: A ( : )"},
		{"ALLOCATE (A(10), STAT=I)", "ALLOCATE: ( A ( 10 ) , STAT = I )"},
		{"DEALLOCATE (A)", "DEALLOCATE: ( A )"},
		{"NULLIFY (P)", "NULLIFY: ( P )"},
		{"SELECT CASE (I)", "SELECT CASE: ( I )"},
		{"CASE (1:3, 5)", "CASE: ( 1 : 3 , 5 )"},
		{"CASE DEFAULT", "CASE: DEFAULT"},
		{"END SELECT", "END SELECT:"},
		{"CYCLE", "CYCLE:"},
		{"EXIT OUTER", "EXIT: OUTER"},
		{"CONTAINS", "CONTAINS:"},
		{"INTENT(IN OUT) A", "INTENT: ( INOUT ) A"},
		{"OPTIONAL B", "OPTIONAL: B"},
		{"POINTER P", "POINTER: P"},
		{"TARGET T", "TARGET: T"},
		{"PUBLIC", "PUBLIC:"},
		{"PRIVATE :: X", "PRIVATE: :: X"},
		{"SEQUENCE", "SEQUENCE:"},
		{"USE M, ONLY: A => B", "USE: M , ONLY : A => B"},
		{"USE M, A => B", "USE: M , A => B"},
		{"INTERFACE AREA", "INTERFACE: AREA"},
		{"INTERFACE OPERATOR(.CROSS.)", "INTERFACE: OPERATOR ( .CROSS. )"},
		{"INTERFACE ASSIGNMENT(=)", "INTERFACE: ASSIGNMENT ( = )"},
		{"END INTERFACE", "END INTERFACE:"},
		{"MODULE SHAPES", "MODULE: SHAPES"},
		{"MODULE PROCEDURES", "MODULE: PROCEDURES"},
		{"END TYPE POINT", "END TYPE: POINT"},
		{"ELSEWHERE", "ELSEWHERE:"},
		{"END WHERE", "END WHERE:"},

		// :: makes a declaration whatever = follows; => with no comma before
		// it is a pointer assignment.
		{"INTEGER, PARAMETER :: N = 10", "INTEGER: , PARAMETER :: N = 10"},
		{"REAL :: X = 1.5", "REAL: :: X = 1.5"},
		{"P => T", "pointer assignment: P => T"},
		{"X%NEXT => Y(1, 2)", "pointer assignment: X % NEXT => Y ( 1 , 2 )"},

		// WHERE as IF: an assignment to an array WHERE, a statement, or a
		// construct's start.
		{"WHERE (A > 0) A = 1", "WHERE: ( A > 0 ) / assignment: A = 1"},
		{"WHERE (A > 0)", "block WHERE: ( A > 0 )"},
		{"WHERE (I) = 1", "assignment: WHERE ( I ) = 1"},

		// The derived type statements beside TYPE's output.
		{"TYPE, PUBLIC :: POINT", "derived TYPE: , PUBLIC :: POINT"},
		{"TYPE :: POINT", "derived TYPE: :: POINT"},
		{"TYPE (POINT) P, Q", "TYPE (type-name): ( POINT ) P , Q"},
		{"TYPE (POINT), POINTER :: P", "TYPE (type-name): ( POINT ) , POINTER :: P"},
		{"TYPE *, X", "TYPE: * , X"},
		{"TYPE 10, X", "TYPE: 10 , X"},
		{"TYPE (10), X", "TYPE: ( 10 ) , X"},

		// Construct names.
		{"OUTER: DO I = 1, N", "OUTER: DO: I = 1 , N"},
		{"OUTER: DO WHILE (I < N)", "OUTER: DO WHILE: WHILE ( I < N )"},
		{"OUTER: DO", "OUTER: DO:"},
		{"TEST: IF (X) THEN", "TEST: IF THEN: ( X ) THEN"},
		{"PICK: SELECT CASE (I)", "PICK: SELECT CASE: ( I )"},
		{"END DO OUTER", "END DO: OUTER"},
		{"ELSE TEST", "ELSE: TEST"},
		{"ELSE IF (Y) THEN TEST", "ELSE IF: ( Y ) THEN TEST"},
		{"END IF TEST", "END IF: TEST"},

		// Named ENDs: the unit's word a keyword token, then its name.
		{"END PROGRAM SQUARES", "END: PROGRAM SQUARES"},
		{"END SUBROUTINE", "END: SUBROUTINE"},
		{"END FUNCTION F", "END: FUNCTION F"},
		{"END MODULE M", "END: MODULE M"},
		{"END BLOCK DATA B", "END: BLOCKDATA B"},
		{"END BLOCKDATA", "END: BLOCKDATA"},
		{"END X", "END: X"},
	}

	for _, c := range cases {
		for _, free := range []bool{false, true} {
			text := c.text
			if !free {
				text = "      " + text
			}

			_, dumps, messages := lexF90(t, free, free, text)
			if len(dumps) != 1 || dumps[0] != c.want || len(messages) != 0 {
				t.Errorf("%s (free %v):\n got %q %v\nwant %q", c.text, free, dumps, messages, c.want)
			}
		}
	}
}

// TestTypeReadings checks TYPE name alone: a derived type's definition under
// /F90, VAX FORTRAN's output statement without it (the phase doc's table).
func TestTypeReadings(t *testing.T) {
	for _, c := range []struct {
		f90  bool
		want string
	}{{false, "TYPE: FMT"}, {true, "derived TYPE: FMT"}} {
		_, dumps, messages := lexF90(t, false, c.f90, "      TYPE FMT")
		if dumps[0] != c.want || len(messages) != 0 {
			t.Errorf("/F90 %v: got %q %v, want %q", c.f90, dumps, messages, c.want)
		}
	}
}

// TestF90StatementState checks what the lexer follows from statement to
// statement: MODULE PROCEDURE only in an interface block, and typed FUNCTION
// statements after CONTAINS and INTERFACE, and with a kind selector.
func TestF90StatementState(t *testing.T) {
	_, dumps, messages := lexF90(t, true, true,
		"module m",
		"interface g",
		"module procedure a, b",
		"moduleprocedure d",
		"integer function f(x)",
		"end function",
		"end interface",
		"module procedure c",
		"contains",
		"integer(4) function h(x)",
		"end function h",
		"end module m",
	)

	want := []string{
		"MODULE: M",
		"INTERFACE: G",
		"MODULE PROCEDURE: A , B",
		"MODULE PROCEDURE: D",
		"FUNCTION (INTEGER): FUNCTION F ( X )",
		"END: FUNCTION",
		"END INTERFACE:",
		"MODULE: PROCEDUREC",
		"CONTAINS:",
		"FUNCTION (INTEGER): ( 4 ) FUNCTION H ( X )",
		"END: FUNCTION H",
		"END: MODULE M",
	}

	if !reflect.DeepEqual(dumps, want) {
		t.Errorf("got\n%q\nwant\n%q", dumps, want)
	}

	// MODULE PROCEDURE's blank is required; MODULE PROCEDUREC outside an
	// interface block is a MODULE statement, whose name the blank inside is
	// an error in.
	if !reflect.DeepEqual(messages, []string{"NEEDBLANK@007", "BLANKINNAME@018"}) {
		t.Errorf("messages %v", messages)
	}
}

// TestF90StatementErrors checks the messages: a construct name before a
// statement that starts no construct, and in free form the blanks a keyword
// may and may not have.
func TestF90StatementErrors(t *testing.T) {
	cases := []struct {
		text string
		free bool
		want []string
	}{
		{"X: Y = 1", true, []string{"NOTCONSTRUCT@001"}},
		{"      X: CALL S", false, []string{"NOTCONSTRUCT@007"}},
		{"OUT ER: DO", true, []string{"BLANKINNAME@005"}},

		// Free form's keywords: the standard's blanks optional, and none
		// elsewhere; MODULE PROCEDURE's required.
		{"GO TO 10", true, nil},
		{"G O TO 10", true, []string{"BLANKINNAME@003"}},
		{"END DO", true, nil},
		{"ENDDO", true, nil},
		{"EN DDO", true, []string{"BLANKINNAME@004"}},
		{"SELECT CASE (I)", true, nil},
		{"SELECTCASE (I)", true, nil},
		{"SEL ECT CASE (I)", true, []string{"BLANKINNAME@005"}},
		{"ELSE IF (X) THEN", true, nil},
		{"EL SE IF (X) THEN", true, []string{"BLANKINNAME@004"}},
		{"I F (X) STOP", true, []string{"BLANKINNAME@003"}},
		{"END BLOCK DATA B", true, nil},
		{"ENDBLOCKDATA B", true, nil},
		{"END PROGRAMP", true, []string{"NEEDBLANK@012"}},
		{"CONT AINS", true, []string{"BLANKINNAME@006"}},
		{"D O I = 1, 2", true, []string{"BLANKINNAME@003"}},

		// Fixed form ignores blanks as it always has.
		{"      G O T O 10", false, nil},
		{"      SEL ECT CASE (I)", false, nil},
	}

	for _, c := range cases {
		_, dumps, messages := lexF90(t, c.free, c.free, c.text)
		if !reflect.DeepEqual(messages, c.want) {
			t.Errorf("%q: messages %v, want %v (%q)", c.text, messages, c.want, dumps)
		}
	}
}
