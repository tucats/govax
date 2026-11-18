package sema

import (
	"strings"
	"testing"
)

// TestInquiries checks the kind and numeric inquiry functions against Phase
// 100's Part A table, folded in each format setting and /NOI4, and the
// SELECTED_ functions' choices.
func TestInquiries(t *testing.T) {
	type mode struct{ noI4, g bool }

	cases := []struct {
		mode
		text, want string
	}{
		{mode{}, "KIND(0)", "4"},
		{mode{noI4: true}, "KIND(0)", "2"},
		{mode{noI4: true}, "KIND(I)", "4"},
		{mode{}, "KIND(.TRUE.)", "4"},
		{mode{noI4: true}, "KIND(.TRUE.)", "2"},
		{mode{}, "KIND(1.0D0)", "8"},
		{mode{}, "KIND(Z)", "8"},
		{mode{}, "KIND(B)", "1"},
		{mode{}, "KIND(T)", "1"},
		{mode{}, "KIND(N)", "4"},
		{mode{}, "KIND(KIND(X))", "4"},

		{mode{}, "HUGE(1_1)", "127"},
		{mode{}, "HUGE(I2)", "32767"},
		{mode{}, "HUGE(I)", "2147483647"},
		{mode{}, "HUGE(X)", "1.7014117E+38"},
		{mode{}, "HUGE(D)", "1.7014118346046923D38"},
		{mode{g: true}, "HUGE(D)", "8.988465674311579D307"},
		{mode{}, "TINY(X)", "2.938736E-39"},
		{mode{g: true}, "TINY(D)", "5.562684646268003D-309"},
		{mode{}, "TINY(1.0_16)", "8.405257857780234Q-4933"},
		{mode{}, "EPSILON(X)", "1.1920929E-07"},
		{mode{}, "EPSILON(D)", "2.7755575615628914D-17"},
		{mode{g: true}, "EPSILON(D)", "2.220446049250313D-16"},
		{mode{}, "EPSILON(1.0_16)", "1.925929944387236Q-34"},

		{mode{}, "DIGITS(1_1)", "7"},
		{mode{}, "DIGITS(I2)", "15"},
		{mode{}, "DIGITS(I)", "31"},
		{mode{}, "DIGITS(X)", "24"},
		{mode{}, "DIGITS(D)", "56"},
		{mode{g: true}, "DIGITS(D)", "53"},
		{mode{}, "DIGITS(1.0_16)", "113"},
		{mode{}, "PRECISION(X)", "6"},
		{mode{}, "PRECISION(C)", "6"},
		{mode{}, "PRECISION(D)", "16"},
		{mode{g: true}, "PRECISION(Z)", "15"},
		{mode{}, "PRECISION(1.0_16)", "33"},
		{mode{}, "RANGE(1_1)", "2"},
		{mode{}, "RANGE(I2)", "4"},
		{mode{}, "RANGE(I)", "9"},
		{mode{}, "RANGE(X)", "38"},
		{mode{}, "RANGE(D)", "38"},
		{mode{g: true}, "RANGE(D)", "307"},
		{mode{}, "RANGE(1.0_16)", "4931"},
		{mode{}, "RADIX(X)", "2"},
		{mode{}, "RADIX(I)", "2"},
		{mode{}, "MAXEXPONENT(X)", "127"},
		{mode{g: true}, "MAXEXPONENT(D)", "1023"},
		{mode{}, "MINEXPONENT(1.0_16)", "-16383"},
		{mode{}, "BIT_SIZE(1_1)", "8"},
		{mode{}, "BIT_SIZE(I2)", "16"},
		{mode{}, "BIT_SIZE(I)", "32"},

		{mode{}, "SELECTED_INT_KIND(2)", "1"},
		{mode{}, "SELECTED_INT_KIND(3)", "2"},
		{mode{}, "SELECTED_INT_KIND(9)", "4"},
		{mode{}, "SELECTED_INT_KIND(10)", "-1"},
		{mode{}, "SELECTED_REAL_KIND(6)", "4"},
		{mode{}, "SELECTED_REAL_KIND(7)", "8"},
		{mode{}, "SELECTED_REAL_KIND(16)", "8"},
		{mode{g: true}, "SELECTED_REAL_KIND(16)", "16"},
		{mode{}, "SELECTED_REAL_KIND(15, 307)", "16"},
		{mode{g: true}, "SELECTED_REAL_KIND(15, 307)", "8"},
		{mode{}, "SELECTED_REAL_KIND(R=40)", "16"},
		{mode{g: true}, "SELECTED_REAL_KIND(R=40)", "8"},
		{mode{}, "SELECTED_REAL_KIND(P=6, R=38)", "4"},
		{mode{}, "SELECTED_REAL_KIND(R=38, P=6)", "4"},
		{mode{}, "SELECTED_REAL_KIND(34)", "-1"},
		{mode{}, "SELECTED_REAL_KIND(R=5000)", "-2"},
		{mode{}, "SELECTED_REAL_KIND(34, 5000)", "-3"},
	}

	for _, c := range cases {
		got, ms := intrinsicOf(t, Options{F90: true, NoI4: c.noI4, GFloating: c.g}, c.text)

		if len(ms) != 0 || got != c.want {
			t.Errorf("%s %+v: got %s %v, want %s", c.text, c.mode, got, ms, c.want)
		}
	}
}

// TestKindArguments checks KIND= on the conversions, and keyword arguments.
func TestKindArguments(t *testing.T) {
	cases := []struct {
		noI4       bool
		text, want string
	}{
		{false, "INT(X, KIND=2)", "(INTEGER*2 X)"},
		{false, "INT(X, 1)", "(INTEGER*1 X)"},
		{false, "INT(KIND=1, A=X)", "(INTEGER*1 X)"},
		{false, "INT(A=X)", "(INTEGER X)"},
		{true, "INT(A=X)", "(INTEGER*2 X)"},
		{true, "INT(X, KIND=4)", "(INTEGER X)"},
		{false, "REAL(I, KIND=8)", "(REAL*8 I)"},
		{false, "REAL(Z, 4)", "(REAL Z)"},
		{false, "REAL(C, KIND=16)", "(REAL*16 C)"},
		{false, "CMPLX(X, Y, KIND=8)", "(cmplx:COMPLEX*16 CMPLX (REAL*8 X) (REAL*8 Y))"},
		{false, "CMPLX(I, KIND=8)", "(COMPLEX*16 I)"},
		{false, "CMPLX(Y=X, X=I)", "(cmplx:COMPLEX CMPLX (REAL I) X)"},
		{false, "NINT(X, KIND=2)", "(nint:INTEGER*2 ININT X)"},
		{false, "NINT(X, KIND=1)", "(INTEGER*1 (nint:INTEGER JNINT X))"},
		{false, "NINT(D, KIND=KIND(0))", "(nint:INTEGER JIDNNT D)"},
		{false, "AINT(X, KIND=8)", "(REAL*8 (call:REAL MTH$AINT X))"},
		{false, "ANINT(A=D, KIND=4)", "(REAL (call:REAL*8 MTH$DNINT D))"},
		{false, "LOGICAL(B)", "(LOGICAL B)"},
		{true, "LOGICAL(B)", "(LOGICAL*2 B)"},
		{false, "LOGICAL(B, KIND=2)", "(LOGICAL*2 B)"},
		{false, "LOGICAL(.TRUE._2, 1)", ".TRUE."},
		{false, "INT(7, KIND=2)", "7"},
		{true, "INT(-2.5)", "-2"},

		// Folded, as constants are.
		{false, "NINT(1.5, KIND=2)", "2"},
		{false, "CMPLX(1.0, KIND=8)", "(1.0D0, 0.0D0)"},
	}

	for _, c := range cases {
		got, ms := intrinsicOf(t, Options{F90: true, NoI4: c.noI4}, c.text)

		if len(ms) != 0 || got != c.want {
			t.Errorf("%s (/NOI4 %t): got %s %v, want %s", c.text, c.noI4, got, ms, c.want)
		}
	}
}

// TestInquiryErrors checks the messages of this subtask's references.
func TestInquiryErrors(t *testing.T) {
	cases := map[string]string{
		"INT(X, KIND=3)":          "NOSUCHKIND: INTEGER has no kind 3",
		"INT(X, KIND=I)":          "KINDNOTCONST",
		"INT(X, B=2)":             "KEYWORDNAME: the intrinsic function INT has no argument B; its arguments are A and KIND",
		"INT(A=X, A=Y)":           "KEYWORDTWICE: the argument A of the intrinsic function INT is given twice",
		"INT(KIND=2, X)":          "KEYWORDORDER",
		"INT(KIND=2)":             "MISSINGARG: the intrinsic function INT needs its argument A",
		"INT(X, 2, 3)":            "INTRINARGS: the intrinsic function INT takes 1 or 2 arguments, not 3",
		"SELECTED_REAL_KIND()":    "MISSINGARG: the intrinsic function SELECTED_REAL_KIND needs its argument P or R",
		"SELECTED_INT_KIND(I)":    "NOTIMPL: SELECTED_INT_KIND of a value not constant",
		"SELECTED_INT_KIND(1.5)":  "BADARG: the intrinsic function SELECTED_INT_KIND's arguments must be INTEGER, and this one is REAL",
		"HUGE(C)":                 "BADARG: the intrinsic function HUGE's arguments must be INTEGER or REAL, and this one is COMPLEX",
		"TINY(I)":                 "BADARG: the intrinsic function TINY's arguments must be REAL, and this one is INTEGER",
		"BIT_SIZE(X)":             "BADARG",
		"KIND()":                  "MISSINGARG",
		"SQRT(X=Y)":               "NOTIMPL: a keyword argument to the intrinsic function SQRT",
		"IINT(A=X)":               "NOTIMPL: a keyword argument to the intrinsic function IINT",
		"F(A=X)":                  "KEYWORDNOINTERFACE: F has no explicit interface here, so its arguments can't be given by keyword",
		"N(I=1)":                  "NOTIMPL: a keyword argument here",
		"LOGICAL(I)":              "BADARG",
		"LOGICAL(B, KIND=8)":      "NOSUCHKIND",
		"REAL(I, KIND=KIND(0)+5)": "NOSUCHKIND: REAL has no kind 9",
	}

	for text, want := range cases {
		_, ms := intrinsicOf(t, Options{F90: true}, text)

		if len(ms) == 0 {
			t.Errorf("%s: no message, want %s", text, want)

			continue
		}

		ident, rest, _ := strings.Cut(want, ": ")
		if ms[0].Ident != ident || rest != "" && !strings.Contains(ms[0].Text, rest) {
			t.Errorf("%s: got %s: %s, want %s", text, ms[0].Ident, ms[0].Text, want)
		}
	}
}

// TestF90NamesUnderNoF90 checks that Fortran 90's new names are intrinsic
// under /NOF90 only when an INTRINSIC statement says so, and are a program's
// own functions otherwise (Decision 2), and that they can't be passed.
func TestF90NamesUnderNoF90(t *testing.T) {
	got, ms := intrinsicOf(t, Options{}, "KIND(X) + RANGE(I)")
	if len(ms) != 0 || got != "(+:R (REAL KIND(X)) RANGE(I))" {
		t.Errorf("/NOF90: got %s %v", got, ms)
	}

	prog, ms := analyzeOpts(t, Options{},
		"      SUBROUTINE E(X)",
		"      INTRINSIC KIND, HUGE",
		"      PRINT *, KIND(X), HUGE(X)",
		"      END")
	if len(ms) != 0 {
		t.Fatalf("INTRINSIC: %v", ms)
	}

	if p := prog.Units[0].Body[0].(*IO); DumpExpr(p.Items[0].Value) != "4" || DumpExpr(p.Items[1].Value) != "1.7014117E+38" {
		t.Errorf("INTRINSIC: got %s and %s", DumpExpr(p.Items[0].Value), DumpExpr(p.Items[1].Value))
	}

	// A kind selector under /NOF90 can't name the program's own KIND.
	_, ms = analyzeOpts(t, Options{},
		"      SUBROUTINE E",
		"      REAL(KIND(1.0D0)) V",
		"      END")
	if len(ms) != 1 || ms[0].Ident != "KINDNOTCONST" {
		t.Errorf("/NOF90 selector: %v", ms)
	}

	_, ms = analyzeOpts(t, Options{F90: true},
		"      SUBROUTINE E",
		"      INTRINSIC HUGE",
		"      CALL G(HUGE)",
		"      END")
	if len(ms) != 1 || ms[0].Ident != "NOTPASSABLE" {
		t.Errorf("passed: %v", ms)
	}
}

// TestKindsInDeclarations checks the functions where a declaration needs a
// constant: a PARAMETER and a kind selector, by each format setting.
func TestKindsInDeclarations(t *testing.T) {
	lines := []string{
		"      SUBROUTINE E",
		"      INTEGER, PARAMETER :: DP = SELECTED_REAL_KIND(15, 307)",
		"      INTEGER, PARAMETER :: SP = KIND(1.0), IK = SELECTED_INT_KIND(R=3)",
		"      INTEGER, PARAMETER :: BIG = HUGE(K9), K2 = DIGITS(K9)",
		"      REAL(DP) V",
		"      REAL(KIND(1.0D0)) U",
		"      REAL(SP) S",
		"      INTEGER(IK) J",
		"      INTEGER(SELECTED_INT_KIND(9)) K9",
		"      REAL(KIND=SELECTED_REAL_KIND(P=6)) R",
		"      INTEGER, PARAMETER :: N = INT(7, KIND=2)",
		"      END",
	}

	want := map[bool]map[string]string{
		false: {"V": "REAL*16", "U": "REAL*8", "S": "REAL", "J": "INTEGER*2", "K9": "INTEGER", "R": "REAL", "N": "INTEGER"},
		true:  {"V": "REAL*8", "U": "REAL*8", "S": "REAL", "J": "INTEGER*2", "K9": "INTEGER", "R": "REAL", "N": "INTEGER"},
	}

	for _, g := range []bool{false, true} {
		prog, ms := analyzeOpts(t, Options{F90: true, GFloating: g}, lines...)
		if len(ms) != 0 {
			t.Fatalf("/G_FLOATING %t: %v", g, ms)
		}

		u := prog.Units[0]
		for name, typ := range want[g] {
			if got := u.Lookup(name).Type.String(); got != typ {
				t.Errorf("/G_FLOATING %t: %s is %s, want %s", g, name, got, typ)
			}
		}

		// K9 is INTEGER, IMPLICIT's, when HUGE asks; declared so later.
		if big := u.Lookup("BIG").Value; big == nil || big.Int != 2147483647 {
			t.Errorf("BIG = %v", big)
		}

		if k2 := u.Lookup("K2").Value; k2 == nil || k2.Int != 31 {
			t.Errorf("K2 = %v", k2)
		}
	}
}
