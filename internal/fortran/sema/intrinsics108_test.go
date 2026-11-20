package sema

import (
	"fmt"
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

		{true, "ACHAR(66)", "'B'"},
		{true, "ACHAR(I)", "(char:CHARACTER*1 ACHAR I)"},
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

// TestPhase108Characters checks subtask 3's character functions: each folded
// of constants, LEN_TRIM, TRIM, and SCAN and VERIFY of one constant character
// and a constant BACK in line, the rest FTNRTL's routines, on arrays too, and
// their messages.
func TestPhase108Characters(t *testing.T) {
	cases := []struct{ text, want string }{
		{"LEN_TRIM('AB  ')", "2"},
		{"TRIM(' AB  ')", "' AB'"},
		{"TRIM('   ')", "''"},
		{"ADJUSTL('  AB')", "'AB  '"},
		{"ADJUSTR('AB  ')", "'  AB'"},
		{"REPEAT('AB', 3)", "'ABABAB'"},
		{"REPEAT('AB', 0)", "''"},
		{"SCAN('ALPHA', 'PH')", "3"},
		{"SCAN('ALPHA', 'PH', BACK=.TRUE.)", "4"},
		{"VERIFY('AABBC', 'AB')", "5"},
		{"VERIFY('AABBC', 'BC', .TRUE.)", "2"},
		{"INDEX('BANANA', 'AN', .TRUE.)", "4"},
		{"INDEX('ABC', '', BACK=.TRUE.)", "4"},

		{"LEN_TRIM(T)", "(len_trim:INTEGER LEN_TRIM T)"},
		{"LEN_TRIM(STRING=S)", "(len_trim:INTEGER LEN_TRIM S)"},
		{"TRIM(T)", "(trim:CHARACTER*(*) TRIM T)"},
		{"ADJUSTL(T)", "(adjustl:CHARACTER*8 FTN$ADJUSTL T)"},
		{"ADJUSTR(S)", "(adjustr:CHARACTER*(*) FTN$ADJUSTR S)"},
		{"REPEAT(T, 2)", "(repeat:CHARACTER*16 FTN$REPEAT T 2)"},
		{"REPEAT(T, I)", "(repeat:CHARACTER*(*) FTN$REPEAT T I)"},
		{"REPEAT(S, 2)", "(repeat:CHARACTER*(*) FTN$REPEAT S 2)"},
		{"SCAN(T, 'A')", "(index:INTEGER SCAN T 'A')"},
		{"SCAN(T, 'A', .TRUE.)", "(scan:INTEGER SCAN T 'A' .TRUE.)"},
		{"VERIFY(T, ' ')", "(verify:INTEGER VERIFY T ' ' .FALSE.)"},
		{"SCAN(T, 'AB')", "(call:INTEGER FTN$SCAN T 'AB' .FALSE.)"},
		{"VERIFY(T, ' ', L)", "(call:INTEGER FTN$VERIFY T ' ' L)"},
		{"VERIFY(T, S, BACK=L2)", "(call:INTEGER FTN$VERIFY T S (LOGICAL L2))"},
		{"INDEX(T, 'A', .TRUE.)", "(call:INTEGER FTN$INDEX_BACK T 'A' .TRUE.)"},
		{"INDEX(T, S, L)", "(call:INTEGER FTN$INDEX_BACK T S L)"},
		{"LEN_TRIM(A)", "(len_trim:INTEGER LEN_TRIM A[])"},
		{"ADJUSTR(A)", "(adjustr:CHARACTER*8 FTN$ADJUSTR A[])"},
		{"T(5:4)", "T(5:4)"},
	}

	unit := []string{
		"      SUBROUTINE E(S)",
		"      INTEGER I",
		"      LOGICAL L",
		"      LOGICAL*2 L2",
		"      CHARACTER*8 T, A(3)",
		"      CHARACTER*(*) S",
	}

	for _, c := range cases {
		lines := append(append([]string{}, unit...), "      PRINT *, "+c.text, "      END")
		prog, ms := analyzeOpts(t, Options{F90: true}, lines...)

		if len(ms) != 0 {
			t.Errorf("%s: %v", c.text, ms)

			continue
		}

		p := prog.Units[0].Body[0].(*IO)
		if got := DumpExpr(p.Items[0].Value); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, got, c.want)
		}
	}

	bad := map[string]string{
		"TRIM(A)":           "BADARG",
		"REPEAT(A, 2)":      "BADARG",
		"REPEAT(T, -1)":     "BADARG",
		"REPEAT(T, 9000)":   "BADARG",
		"REPEAT(T, X)":      "BADARG",
		"LEN_TRIM(I)":       "BADARG",
		"SCAN(T, 1)":        "BADARG",
		"SCAN(T, 'A', 1)":   "BADARG",
		"INDEX(T, 'A', LA)": "NOTIMPL",
	}

	for text, want := range bad {
		lines := append(append([]string{}, unit...), "      REAL X", "      LOGICAL LA(3)", "      PRINT *, "+text, "      END")
		if _, ms := analyzeOpts(t, Options{F90: true}, lines...); len(ms) == 0 || ms[0].Ident != want {
			t.Errorf("%s: %v, want %s", text, ms, want)
		}
	}

	// Of constants in a PARAMETER, under /F90; INDEX, VAX FORTRAN's own, not
	// under /NOF90 (section 4.11).
	prog, ms := analyzeOpts(t, Options{F90: true}, fixed(
		"CHARACTER*(*) P, Q", "INTEGER N", "PARAMETER (P = TRIM(ADJUSTR('AB  ')), Q = REPEAT('-', 3))",
		"PARAMETER (N = INDEX('ABAB', 'B', .TRUE.) + LEN_TRIM(P))", "END")...)
	if len(ms) != 0 {
		t.Fatalf("PARAMETER: %v", ms)
	}

	for name, want := range map[string]string{"P": "'  AB'", "Q": "'---'", "N": "8"} {
		if got := DumpExpr(prog.Units[0].Lookup(name).Value); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}

	if _, ms := analyzeOpts(t, Options{}, fixed("PARAMETER (N = INDEX('AB', 'B'))", "END")...); len(ms) != 1 || ms[0].Ident != "NOTCONST" {
		t.Errorf("/NOF90 INDEX in a PARAMETER: %v", ms)
	}
}

// TestPhase108Numeric checks subtask 4's functions: CEILING, FLOOR, and
// MODULO folded or in line (a REAL MODULO's remainder by MTHRTL's AMOD and its
// kind), in PARAMETERs too; and COMPLEX(16)'s functions and powers, FTNRTL's.
func TestPhase108Numeric(t *testing.T) {
	cases := []struct {
		opts       Options
		text, want string
	}{
		{Options{F90: true}, "CEILING(2.5)", "3"},
		{Options{F90: true}, "FLOOR(-2.5)", "-3"},
		{Options{F90: true}, "CEILING(-0.5)", "0"},
		{Options{F90: true}, "MODULO(-8, 5)", "2"},
		{Options{F90: true}, "MODULO(8, -5)", "-2"},
		{Options{F90: true}, "MODULO(-8.5, 3.0)", "0.5"},
		{Options{F90: true}, "CEILING(X)", "(ceiling:INTEGER CEILING X)"},
		{Options{F90: true}, "FLOOR(A=D)", "(floor:INTEGER FLOOR D)"},
		{Options{F90: true, NoI4: true}, "CEILING(X)", "(ceiling:INTEGER*2 CEILING X)"},
		{Options{F90: true}, "MODULO(I, 5)", "(modulo:INTEGER MODULO I 5)"},
		{Options{F90: true}, "MODULO(X, 2.0)", "(modulo:REAL MTH$AMOD X 2.0)"},
		{Options{F90: true}, "MODULO(D, D)", "(modulo:REAL*8 MTH$DMOD D D)"},
		{Options{F90: true, GFloating: true}, "MODULO(D, D)", "(modulo:REAL*8 MTH$GMOD D D)"},
		{Options{F90: true}, "ABS(Q)", "(call:REAL*16 FTN$CHABS Q)"},
		{Options{F90: true}, "AIMAG(SQRT(Q))", "(imag:REAL*16 AIMAG (call:COMPLEX*32 FTN$CHSQRT Q))"},
		{Options{F90: true}, "AIMAG(COS(X=Q))", "(imag:REAL*16 AIMAG (call:COMPLEX*32 FTN$CHCOS Q))"},
		{Options{F90: true}, "AIMAG(Q ** 2)", "(imag:REAL*16 AIMAG (**:X Q 2))"},
		{Options{F90: true}, "AIMAG(Q ** Q)", "(imag:REAL*16 AIMAG (**:X Q Q))"},
		{Options{F90: true}, "AIMAG(MERGE(Q, Q, .TRUE.))", "(imag:REAL*16 AIMAG (merge:COMPLEX*32 MERGE Q Q .TRUE.))"},
	}

	for _, c := range cases {
		lines := fixed("REAL X", "DOUBLE PRECISION D", "INTEGER I", "COMPLEX(16) Q", "PRINT *, "+c.text, "END")
		prog, ms := analyzeOpts(t, c.opts, lines...)

		if len(ms) != 0 {
			t.Errorf("%s: %v", c.text, ms)

			continue
		}

		p := prog.Units[0].Body[0].(*IO)
		if got := DumpExpr(p.Items[0].Value); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, got, c.want)
		}
	}

	// In a PARAMETER: of INTEGERs, as the standard's 7.1.6.1 has it, and of
	// REALs too, as VAX FORTRAN's NINT and ABS are (section 4.11).
	prog, ms := analyzeOpts(t, Options{F90: true}, fixed(
		"INTEGER M, N, K", "REAL R", "PARAMETER (M = MODULO(-7, 3), N = CEILING(4.2), K = FLOOR(-4.2))",
		"PARAMETER (R = MODULO(7.5, -2.0))", "END")...)
	if len(ms) != 0 {
		t.Fatalf("PARAMETER: %v", ms)
	}

	for name, want := range map[string]string{"M": "2", "N": "5", "K": "-5", "R": "-0.5"} {
		if got := DumpExpr(prog.Units[0].Lookup(name).Value); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}

	bad := map[string]string{
		"CEILING(I)":                   "BADARG",
		"MODULO(I, X)":                 "BADARG",
		"CEILING(Q)":                   "BADARG",
		"PARAMETER (N = MODULO(5, 0))": "DIVZERO",
	}

	for text, want := range bad {
		stmt := "PRINT *, " + text
		if strings.HasPrefix(text, "PARAMETER") {
			stmt = text
		}

		_, ms := analyzeOpts(t, Options{F90: true}, fixed("REAL X", "INTEGER I, N", "COMPLEX(16) Q", stmt, "END")...)
		if len(ms) == 0 || ms[0].Ident != want {
			t.Errorf("%s: %v, want %s", text, ms, want)
		}
	}
}

// TestPhase108ZeroLengths checks finding 4's lengths of zero: declared under
// /F90 (BADCHARLEN under /NOF90), a PARAMETER's value cut to none, and a
// substring past its upper bound (SUBRANGE and SUBORDER under /NOF90).
func TestPhase108ZeroLengths(t *testing.T) {
	for _, c := range []struct {
		f90        bool
		lines      []string
		want, dump string
	}{
		{true, []string{"CHARACTER(LEN=0) Z", "PRINT *, Z"}, "", "Z"},
		{true, []string{"CHARACTER*0 Z", "PRINT *, Z"}, "", "Z"},
		{true, []string{"CHARACTER*(-4) Z", "PRINT *, Z"}, "", "Z"},
		{false, []string{"CHARACTER*0 Z", "PRINT *, Z"}, "BADCHARLEN", ""},
		{true, []string{"CHARACTER(LEN=0), PARAMETER :: Z = 'ABC'", "PRINT *, Z"}, "", "''"},
		{true, []string{"CHARACTER*8 S", "PRINT *, S(5:4)"}, "", "S(5:4)"},
		{true, []string{"CHARACTER*8 S", "PRINT *, S(20:3)"}, "", "S(20:19)"},
		{false, []string{"CHARACTER*8 S", "PRINT *, S(5:4)"}, "SUBRANGE", ""},
		{true, []string{"CHARACTER(LEN=0) FUNCTION F()", "F = ''"}, "NOTIMPL", ""},
	} {
		_, ms := analyzeOpts(t, Options{F90: c.f90}, fixed(append(c.lines, "END")...)...)

		got := ""
		if len(ms) > 0 {
			got = ms[0].Ident
		}

		if got != c.want {
			t.Errorf("%v (/F90 %t): %v, want %s", c.lines, c.f90, ms, c.want)
		}
	}
}

// TestPhase108Names checks that the intrinsic subroutines take keywords, and
// are NOTIMPL but MVBITS. (Fortran 90's new function names, intrinsic under
// /F90 and the program's own under /NOF90, are each subtask's test's.)
func TestPhase108Names(t *testing.T) {
	subs := map[string]string{
		"CALL DATE_AND_TIME(VALUES=IV)":      "NOTIMPL: the intrinsic subroutine DATE_AND_TIME",
		"CALL SYSTEM_CLOCK(COUNT_RATE=I)":    "NOTIMPL",
		"CALL RANDOM_NUMBER(X)":              "NOTIMPL",
		"CALL RANDOM_SEED(SIZE=I)":           "NOTIMPL",
		"CALL RANDOM_SEED(SEED=I)":           "KEYWORDNAME: the intrinsic subroutine RANDOM_SEED has no argument SEED; its arguments are SIZE, PUT, and GET",
		"CALL DATE_AND_TIME(T, T, T, IV, I)": "INTRINSUBARGS: the intrinsic subroutine DATE_AND_TIME takes 0 to 4 arguments, not 5",
		"CALL RANDOM_NUMBER()":               "INTRINSUBARGS: the intrinsic subroutine RANDOM_NUMBER takes 1 argument, not 0",
		"CALL SYSTEM_CLOCK(I,,J)":            "NULLINTRINSUB",
		"CALL MVBITS(FROM=I, 0, 2, J, 1)":    "KEYWORDORDER",
		"CALL MVBITS(I, 0, 2, J, LEN=1)":     "KEYWORDTWICE",
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

// TestPhase108Model checks finding 7's model functions: folded of constants,
// EXPONENT and FRACTION in line, the rest FTNRTL's routine by X's format, I
// made an INTEGER*4 and S left as it is, and their messages.
func TestPhase108Model(t *testing.T) {
	cases := []struct {
		opts       Options
		text, want string
	}{
		{Options{F90: true}, "EXPONENT(10.0)", "4"},
		{Options{F90: true}, "FRACTION(-10.0)", "-0.625"},
		{Options{F90: true}, "SCALE(3.0, 2)", "12.0"},
		{Options{F90: true}, "SET_EXPONENT(3.0, 1)", "1.5"},
		{Options{F90: true}, "RRSPACING(-3.0)", "1.2582912E+07"},
		{Options{F90: true}, "SCALE(1.0, -200)", "0.0"},
		{Options{F90: true}, "EXPONENT(X)", "(exponent:INTEGER EXPONENT X)"},
		{Options{F90: true, NoI4: true}, "EXPONENT(D)", "(exponent:INTEGER*2 EXPONENT D)"},
		{Options{F90: true}, "FRACTION(X=H)", "(fraction:REAL*16 FRACTION H)"},
		{Options{F90: true}, "SCALE(X, I2)", "(call:REAL FTN$SCALE_F X (INTEGER I2))"},
		{Options{F90: true}, "SET_EXPONENT(I=1, X=D)", "(call:REAL*8 FTN$SET_EXPONENT_D D 1)"},
		{Options{F90: true, GFloating: true}, "SPACING(D)", "(call:REAL*8 FTN$SPACING_G D)"},
		{Options{F90: true}, "RRSPACING(H)", "(call:REAL*16 FTN$RRSPACING_H H)"},
		{Options{F90: true}, "NEAREST(X, H)", "(call:REAL FTN$NEAREST_F X H)"},
	}

	for _, c := range cases {
		lines := fixed("REAL X", "DOUBLE PRECISION D", "REAL*16 H", "INTEGER*2 I2", "PRINT *, "+c.text, "END")
		prog, ms := analyzeOpts(t, c.opts, lines...)

		if len(ms) != 0 {
			t.Errorf("%s: %v", c.text, ms)

			continue
		}

		p := prog.Units[0].Body[0].(*IO)
		if got := DumpExpr(p.Items[0].Value); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, got, c.want)
		}
	}

	bad := map[string]string{
		"EXPONENT(I2)":    "BADARG",
		"SCALE(X, X)":     "BADARG",
		"NEAREST(X, I2)":  "BADARG",
		"SCALE(1.0, I2)":  "",
		"SCALE(1.0)":      "INTRINARGS",
		"SCALE(1.0, 200)": "REALRANGE",
	}

	for text, want := range bad {
		_, ms := analyzeOpts(t, Options{F90: true}, fixed("REAL X", "INTEGER*2 I2", "PRINT *, "+text, "END")...)

		got := ""
		if len(ms) > 0 {
			got = ms[0].Ident
		}

		if got != want {
			t.Errorf("%s: %v, want %q", text, ms, want)
		}
	}
}

// TestPhase108Transfer checks TRANSFER (finding 8): folded of constants, in
// PARAMETERs too, and otherwise its forms, a scalar's an IntrinsicRef and an
// array's an ArrayFunc of the right extent.
func TestPhase108Transfer(t *testing.T) {
	cases := []struct {
		opts       Options
		text, want string
	}{
		{Options{F90: true}, "TRANSFER(1.0, 0)", "16512"},
		{Options{F90: true}, "TRANSFER('ABCD', 0)", "1145258561"},
		{Options{F90: true}, "TRANSFER(16512, 1.0)", "1.0"},
		{Options{F90: true}, "TRANSFER(1.0D0, (/0/))", "(/:I 16512 0 /) (2)"},
		{Options{F90: true}, "TRANSFER((/1, 2, 3/), 0D0, 2)", "(/:R 0.0D0 0.0D0 /) (2)"},
		{Options{F90: true}, "TRANSFER('AB', 'XYZ')", "'AB\\0'"},
		{Options{F90: true}, "TRANSFER(65, 'Z')", "'A'"},
		{Options{F90: true}, "TRANSFER((/65, 66/), 'Z', SIZE=3)", "(/:C 'A' '\\0' '\\0' /) (3)"},
		{Options{F90: true}, "TRANSFER(1.0, I, 2)", "(/:I 16512 0 /) (2)"},
		{Options{F90: true}, "TRANSFER(X, 0)", "(transfer:INTEGER TRANSFER X)"},
		{Options{F90: true}, "TRANSFER(X, (/0/))", "TRANSFER(X) (1)"},
		{Options{F90: true}, "TRANSFER(IA, 0D0)", "(transfer:REAL*8 TRANSFER IA[])"},
		{Options{F90: true}, "TRANSFER(IA, 0D0, N)", "TRANSFER(IA[], N) (:)"},
		{Options{F90: true}, "TRANSFER(IA(1:N), 'A')", "(transfer:CHARACTER*1 TRANSFER IA[1:N:1])"},
		{Options{F90: true}, "TRANSFER(MOLD=1.0, SOURCE=I)", "(transfer:REAL TRANSFER I)"},
	}

	for _, c := range cases {
		lines := fixed("REAL X", "INTEGER I, N, IA(5)", "PRINT *, "+c.text, "END")
		prog, ms := analyzeOpts(t, c.opts, lines...)

		if len(ms) != 0 {
			t.Errorf("%s: %v", c.text, ms)

			continue
		}

		p := prog.Units[0].Body[0].(*IO)
		v := p.Items[0].Value
		got := strings.ReplaceAll(DumpExpr(v), "\x00", "\\0")
		if len(v.Shape()) > 0 {
			got += fmt.Sprintf(" %v", v.Shape())
		}

		if got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, got, c.want)
		}
	}

	// In a PARAMETER (7.1.6.1 (5)), a scalar and an array.
	lines := fixed("INTEGER K, KA(2)", "PARAMETER (K = TRANSFER('ABCD', 0), KA = TRANSFER(1.0D0, (/0/)))",
		"PRINT *, K, KA", "END")

	prog, ms := analyzeOpts(t, Options{F90: true}, lines...)
	if len(ms) != 0 {
		t.Fatalf("PARAMETER: %v", ms)
	}

	items := prog.Units[0].Body[0].(*IO).Items
	if got := DumpExpr(items[0].Value) + " " + DumpExpr(items[1].Value); got != "1145258561 (/:I 16512 0 /)" {
		t.Errorf("PARAMETER: got %s", got)
	}

	bad := map[string]string{
		"TRANSFER(X)":         "INTRINARGS",
		"TRANSFER(X, 0, 1.0)": "BADARG",
		"TRANSFER(X, C)":      "NOTIMPL",
		"TRANSFER(X, 0, N)":   "",
	}

	for text, want := range bad {
		_, ms := analyzeOpts(t, Options{F90: true}, fixed("SUBROUTINE S(C)", "CHARACTER*(*) C", "REAL X",
			"INTEGER N", "PRINT *, "+text, "END")...)

		got := ""
		if len(ms) > 0 {
			got = ms[0].Ident
		}

		if got != want {
			t.Errorf("%s: %v, want %q", text, ms, want)
		}
	}

	// Under /NOF90 it is the program's own function.
	prog, ms = analyzeOpts(t, Options{}, fixed("X = TRANSFER(1.0, 0)", "END")...)
	if len(ms) != 0 {
		t.Fatalf("/NOF90: %v", ms)
	}

	if got := DumpExpr(prog.Units[0].Body[0].(*Assign).Value); !strings.Contains(got, "TRANSFER") || strings.Contains(got, "transfer:") {
		t.Errorf("/NOF90: got %s", got)
	}
}
