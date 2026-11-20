package debugger_test

import (
	"os"
	"strings"
	"testing"
)

// This file holds the debugger on Fortran 90's records and sections (docs/planning/
// PHASE-110 - Fortran 90 compiler usability.md, subtask 7a, the survey's D1 and D2).

// recordSource has a derived type of each shape: plain, nested, with an array
// component, an array of them, an allocatable array of them, and a section of them
// given to an assumed-shape dummy; and a DEC record with a nested structure.
const recordSource = `      OPTIONS /F90
      MODULE SHAPES
      TYPE POINT
        REAL X, Y
      END TYPE
      TYPE SHAPE
        CHARACTER(LEN=4) TAG
        TYPE(POINT) P(3)
        INTEGER N
      END TYPE
      END MODULE

      PROGRAM GEO
      USE SHAPES
      INTERFACE
        SUBROUTINE WALK(Q)
        USE SHAPES
        TYPE(POINT) Q(:)
        END SUBROUTINE
      END INTERFACE
      STRUCTURE /DATE/
        INTEGER*2 DAY
        STRUCTURE TIME
          INTEGER*2 H, M
        END STRUCTURE
      END STRUCTURE
      RECORD /DATE/ D
      TYPE(POINT) PT, PTS(5)
      TYPE(POINT), ALLOCATABLE :: HEAP(:)
      TYPE(SHAPE) S
      INTEGER I
      PT = POINT(1.5, 2.5)
      DO I = 1, 5
        PTS(I) = POINT(REAL(I), REAL(10 * I))
      END DO
      S%TAG = 'TRI'
      S%P(2) = PT
      S%N = 3
      D.DAY = 7
      D.TIME.H = 9
      D.TIME.M = 30
      ALLOCATE (HEAP(0:1))
      HEAP(1) = POINT(-1.0, -2.0)
      CALL WALK(PTS(1:5:2))
      END

      SUBROUTINE WALK(Q)
      USE SHAPES
      TYPE(POINT) Q(:)
   10 CONTINUE
      END
`

// TestFortranRecords holds EXAMINE of records: a whole one a component at a time, each
// component named with % (a derived type's) or a period (a DEC record's), and a
// component of an array of records being that component of every element.
func TestFortranRecords(t *testing.T) {
	// The MACRO module is named for the file's first unit, SHAPES (the survey's D7).
	c := fortranSessionOf(t, recordSource, "WALK.L10")

	dispatch(t, c, "SET RADIX DECIMAL")

	for _, step := range []struct{ line, output string }{
		{"EXAMINE GEO.PT", "SHAPES\\GEO.PT%X:  1.500000\nSHAPES\\GEO.PT%Y:  2.500000\n"},
		{"EXAMINE GEO.PT%Y", "SHAPES\\GEO.PT%Y:  2.500000\n"},
		{"EXAMINE GEO.PT.Y", "SHAPES\\GEO.PT%Y:  2.500000\n"},
		{"EXAMINE GEO.PTS(2)%Y", "SHAPES\\GEO.PTS(2)%Y:  20.00000\n"},
		{"EXAMINE GEO.PTS(4)", "SHAPES\\GEO.PTS(4)%X:  4.000000\nSHAPES\\GEO.PTS(4)%Y:  40.00000\n"},
		{"EXAMINE GEO.PTS(2:3)%X", "SHAPES\\GEO.PTS(2)%X:  2.000000\nSHAPES\\GEO.PTS(3)%X:  3.000000\n"},
		{"EXAMINE GEO.S%P(2)%X", "SHAPES\\GEO.S%P(2)%X:  1.500000\n"},
		{"EXAMINE GEO.S%TAG(1:2)", "SHAPES\\GEO.S%TAG(1:2):  'TR'\n"},
		{"EXAMINE GEO.S%P%Y", "SHAPES\\GEO.S%P(1)%Y:  0.0000000\nSHAPES\\GEO.S%P(2)%Y:  2.500000\nSHAPES\\GEO.S%P(3)%Y:  0.0000000\n"},
		{"EXAMINE GEO.S", "SHAPES\\GEO.S%TAG:  'TRI '\n" +
			"SHAPES\\GEO.S%P(1)%X:  0.0000000\nSHAPES\\GEO.S%P(1)%Y:  0.0000000\n" +
			"SHAPES\\GEO.S%P(2)%X:  1.500000\nSHAPES\\GEO.S%P(2)%Y:  2.500000\n" +
			"SHAPES\\GEO.S%P(3)%X:  0.0000000\nSHAPES\\GEO.S%P(3)%Y:  0.0000000\n" +
			"SHAPES\\GEO.S%N:  3\n"},
		// A DEC record, its nested structure without a name of its own.
		{"EXAMINE GEO.D", "SHAPES\\GEO.D.DAY:  7\nSHAPES\\GEO.D.TIME.H:  9\nSHAPES\\GEO.D.TIME.M:  30\n"},
		{"EXAMINE GEO.D.TIME.M", "SHAPES\\GEO.D.TIME.M:  30\n"},
		// An allocatable array of records, through its descriptor.
		{"EXAMINE GEO.HEAP(1)%Y", "SHAPES\\GEO.HEAP(1)%Y:  -2.000000\n"},
		// An assumed-shape dummy given every other element, by its descriptor's strides.
		{"EXAMINE Q%X", "SHAPES\\WALK.Q(1)%X:  1.000000\nSHAPES\\WALK.Q(2)%X:  3.000000\nSHAPES\\WALK.Q(3)%X:  5.000000\n"},
		{"EXAMINE Q(2)", "SHAPES\\WALK.Q(2)%X:  3.000000\nSHAPES\\WALK.Q(2)%Y:  30.00000\n"},
		// EVALUATE gives a component's value, in an expression too.
		{"EVALUATE GEO.D.TIME.M", "30\n"},
		{"EVALUATE GEO.D%DAY + 1", "8\n"},
	} {
		if got := variableOutput(t, c, step.line); got != step.output {
			t.Errorf("%s: got %q, want %q", step.line, got, step.output)
		}
	}

	for _, want := range []struct{ line, says string }{
		{"EXAMINE GEO.PT%Z", `SHAPES\GEO.PT%Z`},
		{"EXAMINE GEO.D.YEAR", `SHAPES\GEO.D.YEAR`},
		{"EXAMINE GEO.I%X", `SHAPES\GEO.I%X`},
		{"EXAMINE GEO.PTS(6)%X", "subscript 1 is out of bounds, value is 6, bounds are 1:5"},
	} {
		if _, err := sayErr(c, want.line); err == nil || !strings.Contains(err.Error(), want.says) {
			t.Errorf("%s: %v, want %q", want.line, err, want.says)
		}
	}

	// A DEPOSIT to a component reads the value as its type; to a whole record, it can't.
	dispatch(t, c, "DEPOSIT GEO.PT%X=9.5")
	dispatch(t, c, "DEPOSIT GEO.D.TIME.H=11")

	for _, want := range []struct{ line, output string }{
		{"EXAMINE GEO.PT", "SHAPES\\GEO.PT%X:  9.500000\nSHAPES\\GEO.PT%Y:  2.500000\n"},
		{"EXAMINE GEO.D.TIME.H", "SHAPES\\GEO.D.TIME.H:  11\n"},
	} {
		if got := variableOutput(t, c, want.line); got != want.output {
			t.Errorf("%s: got %q, want %q", want.line, got, want.output)
		}
	}

	if _, err := sayErr(c, "DEPOSIT GEO.PT=1"); err == nil {
		t.Error("DEPOSIT to a whole record: no error")
	}

	// Under SET LANGUAGE FORTRAN, where a period selects a component in an expression.
	dispatch(t, c, "SET LANGUAGE FORTRAN")

	for _, want := range []struct{ line, output string }{
		{"EXAMINE GEO.D.TIME.M", "SHAPES\\GEO.D.TIME.M:  30\n"},
		{"EVALUATE GEO.D.TIME.M + 1", "31\n"},
		{"EVALUATE GEO.D%TIME%M - 1", "29\n"},
	} {
		if got := variableOutput(t, c, want.line); got != want.output {
			t.Errorf("%s: got %q, want %q", want.line, got, want.output)
		}
	}
}

// TestFortranRecordTable holds the table's lines for the records: each structure once,
// a nested one listed after the one naming it, a DEC structure without a name given one.
func TestFortranRecordTable(t *testing.T) {
	fortranSessionOf(t, recordSource, "")

	data, err := os.ReadFile("fv.mar")
	if err != nil {
		t.Fatal(err)
	}

	text := string(data)

	for _, want := range []string{
		";DBG$ V GEO PT REC:POINT 8 S:GEO.PT - -\n",
		";DBG$ V GEO PTS REC:POINT 8 S:GEO.PTS 1:5 -\n",
		";DBG$ S GEO SHAPE 32 D\n",
		";DBG$ C GEO SHAPE P 4 REC:POINT 8 1:3\n",
		";DBG$ S GEO DATE 6 R\n",
		";DBG$ C GEO DATE TIME 2 REC:$",
		";DBG$ C GEO POINT Y 4 RF 4 -\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the table lacks %q", want)
		}
	}

	if n := strings.Count(text, ";DBG$ S GEO POINT "); n != 1 {
		t.Errorf("POINT is described %d times", n)
	}
}

// TestFortranSections holds EXAMINE of an array section: by bounds, by a stride, with
// a bound left out, in more than one dimension, of CHARACTER elements with a substring,
// and of an assumed-shape dummy given a section, by its descriptor's strides.
func TestFortranSections(t *testing.T) {
	c := fortranDebugSession(t, "NEST")

	dispatch(t, c, "SET RADIX DECIMAL")

	for _, step := range []struct{ line, output string }{
		{"EXAMINE FV.V(2:3)", "FV\\FV.V(2):  2.500000\nFV\\FV.V(3):  3.500000\n"},
		{"EXAMINE FV.V(:2)", "FV\\FV.V(1):  1.500000\nFV\\FV.V(2):  2.500000\n"},
		{"EXAMINE FV.V(1::2)", "FV\\FV.V(1):  1.500000\nFV\\FV.V(3):  3.500000\n"},
		{"EXAMINE FV.V(3:1:-2)", "FV\\FV.V(3):  3.500000\nFV\\FV.V(1):  1.500000\n"},
		{"EXAMINE FV.V(3:2)", ""},
		{"EXAMINE FV.M(1:2,1)", "FV\\FV.M(1,1):  11\nFV\\FV.M(2,1):  21\n"},
		{"EXAMINE FV.M(2,1:3:2)", "FV\\FV.M(2,1):  21\nFV\\FV.M(2,3):  23\n"},
		{"EXAMINE FV.L0(-1:0)", "FV\\FV.L0(-1):  5\nFV\\FV.L0(0):  0\n"},
		{"EXAMINE FV.NAMES(1:2)(1:3)", "FV\\FV.NAMES(1)(1:3):  '...'\nFV\\FV.NAMES(2)(1:3):  'SEC'\n"},
		// An assumed-size array: its last dimension's upper bound must be given.
		{"EXAMINE SUB.ASSUME(1:2,3)", "FV\\SUB.ASSUME(1,3):  0\nFV\\SUB.ASSUME(2,3):  23\n"},
		{"EXAMINE SUB.ASSUME(2,:)", "FV\\SUB.ASSUME(2,1):  21\n"},
		// An adjustable array's bounds are its descriptor's.
		{"EXAMINE SUB.A(2:)", "FV\\SUB.A(2):  2.500000\nFV\\SUB.A(3):  3.500000\n"},
	} {
		if got := variableOutput(t, c, step.line); got != step.output {
			t.Errorf("%s: got %q, want %q", step.line, got, step.output)
		}
	}

	for _, want := range []struct{ line, says string }{
		{"EXAMINE FV.V(0:2)", "subscript 1 is out of bounds, value is 0, bounds are 1:3"},
		{"EXAMINE FV.M(1:2,4)", "subscript 2 is out of bounds, value is 4, bounds are 1:3"},
		{"EXAMINE FV.V(1:3:0)", "1:3:0"},
		{"EXAMINE FV.V(1:2:3:4)", "1:2:3:4"},
	} {
		if _, err := sayErr(c, want.line); err == nil || !strings.Contains(err.Error(), want.says) {
			t.Errorf("%s: %v, want %q", want.line, err, want.says)
		}
	}
}

// TestFortranDescribedSections: a section of an assumed-shape dummy given a section of
// its own, its elements found through the descriptor's strides.
func TestFortranDescribedSections(t *testing.T) {
	const source = `      OPTIONS /F90
      PROGRAM DS
      INTEGER M(4, 3), I, J
      INTERFACE
        SUBROUTINE SHOW(A)
        INTEGER A(:, :)
        END SUBROUTINE
      END INTERFACE
      DO J = 1, 3
        DO I = 1, 4
          M(I, J) = 10 * I + J
        END DO
      END DO
      CALL SHOW(M(2:4:2, 1:3:2))
      END

      SUBROUTINE SHOW(A)
      INTEGER A(:, :)
   10 CONTINUE
      END
`
	c := fortranSessionOf(t, source, "SHOW.L10")

	dispatch(t, c, "SET RADIX DECIMAL")

	for _, step := range []struct{ line, output string }{
		{"EXAMINE A(2,:)", "DS\\SHOW.A(2,1):  41\nDS\\SHOW.A(2,2):  43\n"},
		{"EXAMINE A(:,2)", "DS\\SHOW.A(1,2):  23\nDS\\SHOW.A(2,2):  43\n"},
		{"EXAMINE DS.M(2:4:2,3)", "DS\\DS.M(2,3):  23\nDS\\DS.M(4,3):  43\n"},
	} {
		if got := variableOutput(t, c, step.line); got != step.output {
			t.Errorf("%s: got %q, want %q", step.line, got, step.output)
		}
	}
}
