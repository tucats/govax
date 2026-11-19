package sema

import (
	"strings"
	"testing"
)

// TestPhase108Table checks docs/planning/PHASE-108 - Fortran 90 intrinsic
// procedures.md's subtask 2: keywords to every function by the standard's
// names, a specific name's by its generic's, MAX's arguments past A2 by
// position, ISHFTC's SIZE left out, the bit functions of I's kind under /F90,
// ACHAR and IACHAR, INDEX's BACK=.FALSE., and SELECTED_INT_KIND and
// SELECTED_REAL_KIND of values.
func TestPhase108Table(t *testing.T) {
	cases := []struct {
		f90        bool
		text, want string
	}{
		{true, "IAND(J=I, I=J)", "(.AND.:I J I)"},
		{false, "IAND(I=I, J=5)", "(.AND.:I I 5)"},
		{true, "MOD(P=3, A=I)", "(mod:INTEGER JMOD I 3)"},
		{true, "SQRT(X=Y)", "(call:REAL MTH$SQRT Y)"},
		{true, "ATAN2(X=X, Y=Y)", "(call:REAL MTH$ATAN2 Y X)"},
		{true, "IINT(A=X)", "(INTEGER*2 X)"},
		{true, "MAX(A2=I, A1=J)", "(max:INTEGER JMAX0 J I)"},
		{true, "MAX(I, J, 4, 5)", "(max:INTEGER JMAX0 I J 4 5)"},
		{true, "INDEX(SUBSTRING='A', STRING=T)", "(index:INTEGER INDEX T 'A')"},
		{true, "INDEX(T, 'A', BACK=.FALSE.)", "(index:INTEGER INDEX T 'A')"},
		{true, "LGE(STRING_B=T, STRING_A=S)", "(.GE.:C S T)"},
		{true, "ICHAR(C=T(1:1))", "(ichar:INTEGER ICHAR T(1:1))"},
		{true, "CHAR(I=65, KIND=1)", "'A'"},

		// ISHFTC's SIZE is BIT_SIZE(I) when left out.
		{true, "ISHFTC(I, 3)", "(call:INTEGER FTN$JISHFTC I 3 32)"},
		{true, "ISHFTC(I2, 3)", "(INTEGER*2 (call:INTEGER FTN$IISHFTC I2 3 16))"},
		{false, "ISHFTC(I2, 3)", "(call:INTEGER FTN$IISHFTC I2 3 16)"},
		{true, "ISHFTC(B1, 3)", "(INTEGER*1 (call:INTEGER FTN$IISHFTC (INTEGER*2 B1) 3 8))"},

		// I's kind under /F90; Table D-3's under /NOF90.
		{true, "IBITS(I2, 0, 3)", "(INTEGER*2 (bits:INTEGER IIBITS I2 0 3))"},
		{false, "IBITS(I2, 0, 3)", "(bits:INTEGER IIBITS I2 0 3)"},
		{true, "ISHFT(I2, J)", "(call:INTEGER*2 MTH$IISHFT I2 (INTEGER*2 J))"},
		{false, "ISHFT(I2, J)", "(call:INTEGER MTH$JISHFT (INTEGER I2) J)"},
		{true, "ISHFT(B1, -2)", "(INTEGER*1 (shift:INTEGER*2 IISHFT (zext:INTEGER*2 ZEXT B1) -2))"},

		{true, "ACHAR(66)", "(char:CHARACTER*1 ACHAR 66)"},
		{true, "IACHAR(T(2:2))", "(ichar:INTEGER IACHAR T(2:2))"},

		// SELECTED_INT_KIND and SELECTED_REAL_KIND of values.
		{true, "SELECTED_INT_KIND(I)",
			"(merge:INTEGER MERGE 1 (merge:INTEGER MERGE 2 (merge:INTEGER MERGE 4 -1 (.LE.:I I 9)) (.LE.:I I 4)) (.LE.:I I 2))"},
		{true, "SELECTED_REAL_KIND(R=I)",
			"(merge:INTEGER MERGE 4 (merge:INTEGER MERGE 8 (merge:INTEGER MERGE 16 -2 (.LE.:I I 4931)) (.LE.:I I 38)) (.LE.:I I 38))"},
	}

	unit := []string{
		"      SUBROUTINE E(S)",
		"      INTEGER*4 I, J",
		"      INTEGER*2 I2",
		"      INTEGER*1 B1",
		"      REAL X, Y",
		"      CHARACTER*8 T",
		"      CHARACTER*(*) S",
	}

	for _, c := range cases {
		lines := append(append([]string{}, unit...), "      PRINT *, "+c.text, "      END")
		prog, ms := analyzeOpts(t, Options{F90: c.f90}, lines...)

		if len(ms) != 0 {
			t.Errorf("%s (/F90 %t): %v", c.text, c.f90, ms)

			continue
		}

		p := prog.Units[0].Body[0].(*IO)
		if got := DumpExpr(p.Items[0].Value); got != c.want {
			t.Errorf("%s (/F90 %t):\n got %s\nwant %s", c.text, c.f90, got, c.want)
		}
	}
}

// TestPhase108Names checks that Fortran 90's new names are intrinsic under
// /F90, NOTIMPL until their subtask, and the program's own functions under
// /NOF90; and that the intrinsic subroutines take keywords, and are NOTIMPL
// but MVBITS.
func TestPhase108Names(t *testing.T) {
	later := []string{"ADJUSTL(T)", "ADJUSTR(T)", "LEN_TRIM(T)", "SCAN(T, 'A')", "VERIFY(T, ' ', .TRUE.)",
		"TRIM(T)", "REPEAT(T, 2)", "CEILING(X)", "FLOOR(X)", "MODULO(X, Y)", "EXPONENT(X)", "FRACTION(X)",
		"NEAREST(X, 1.0)", "RRSPACING(X)", "SCALE(X, 2)", "SET_EXPONENT(X, 2)", "SPACING(X)", "TRANSFER(X, 0)"}

	for _, text := range later {
		_, ms := analyzeOpts(t, Options{F90: true}, fixed("CHARACTER*8 T", "PRINT *, "+text, "END")...)
		if len(ms) != 1 || ms[0].Ident != "NOTIMPL" || !strings.Contains(ms[0].Text, "the intrinsic function") {
			t.Errorf("/F90 %s: %v", text, ms)
		}

		_, ms = analyzeOpts(t, Options{}, fixed("CHARACTER*8 T", "PRINT *, "+text, "END")...)
		if len(ms) != 0 {
			t.Errorf("/NOF90 %s: %v", text, ms)
		}
	}

	subs := map[string]string{
		"CALL DATE_AND_TIME(VALUES=IV)":     "NOTIMPL: the intrinsic subroutine DATE_AND_TIME",
		"CALL SYSTEM_CLOCK(COUNT_RATE=I)":   "NOTIMPL",
		"CALL RANDOM_NUMBER(X)":             "NOTIMPL",
		"CALL RANDOM_SEED(SIZE=I)":          "NOTIMPL",
		"CALL RANDOM_SEED(SEED=I)":          "KEYWORDNAME: the intrinsic subroutine RANDOM_SEED has no argument SEED; its arguments are SIZE, PUT, and GET",
		"CALL DATE_AND_TIME(T, T, T, IV, I)": "INTRINSUBARGS: the intrinsic subroutine DATE_AND_TIME takes 0 to 4 arguments, not 5",
		"CALL RANDOM_NUMBER()":              "INTRINSUBARGS: the intrinsic subroutine RANDOM_NUMBER takes 1 argument, not 0",
		"CALL SYSTEM_CLOCK(I,,J)":           "NULLINTRINSUB",
		"CALL MVBITS(FROM=I, 0, 2, J, 1)":  "KEYWORDORDER",
		"CALL MVBITS(I, 0, 2, J, LEN=1)":    "KEYWORDTWICE",
	}

	for text, want := range subs {
		_, ms := analyzeOpts(t, Options{F90: true}, fixed("CHARACTER*8 T", "DIMENSION IV(8)", text, "END")...)
		ident, rest, _ := strings.Cut(want, ": ")

		if len(ms) == 0 || ms[0].Ident != ident || !strings.Contains(ms[0].Text, rest) {
			t.Errorf("%s: %v, want %s", text, ms, want)
		}
	}

	// Under /NOF90 DATE_AND_TIME is the program's own, unless an INTRINSIC
	// statement says otherwise.
	if _, ms := analyzeOpts(t, Options{}, fixed("CALL DATE_AND_TIME(I)", "END")...); len(ms) != 0 {
		t.Errorf("/NOF90 DATE_AND_TIME: %v", ms)
	}

	_, ms := analyzeOpts(t, Options{}, fixed("INTRINSIC DATE_AND_TIME", "CALL DATE_AND_TIME(I)", "END")...)
	if len(ms) != 1 || ms[0].Ident != "NOTIMPL" {
		t.Errorf("INTRINSIC DATE_AND_TIME: %v", ms)
	}

	// A unit of the file of the name is the one called.
	_, ms = analyzeOpts(t, Options{F90: true}, fixed("CALL RANDOM_SEED(1)", "END", "SUBROUTINE RANDOM_SEED(I)", "END")...)
	if len(ms) != 0 {
		t.Errorf("own RANDOM_SEED: %v", ms)
	}
}

// TestGenericIntrinsicRanks checks GENERICINTRINSIC against the
// transformational functions' own ranks: a specific of SUM taking a scalar can
// be told from SUM, one taking an array can't.
func TestGenericIntrinsicRanks(t *testing.T) {
	for _, c := range []struct {
		decl string
		want string
	}{
		{"REAL ARRAY", ""},
		{"REAL ARRAY(10)", "GENERICINTRINSIC"},
		{"REAL ARRAY(10, 10)", "GENERICINTRINSIC"},
	} {
		_, ms := analyzeOpts(t, Options{F90: true},
			fixed("PROGRAM P", "INTERFACE SUM", "FUNCTION MYSUM(ARRAY)", c.decl, "END FUNCTION", "END INTERFACE", "END")...)

		got := ""
		if len(ms) > 0 {
			got = ms[0].Ident
		}

		if got != c.want {
			t.Errorf("%s: %v, want %s", c.decl, ms, c.want)
		}
	}
}

// fixed is lines in fixed form's statement field.
func fixed(lines ...string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = "      " + l
	}

	return out
}
