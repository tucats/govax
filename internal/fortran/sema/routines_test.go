package sema

import (
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestIntrinsicCalls checks references to the functions MTHRTL computes: the
// routine of each format, and ISHFT by a count that isn't constant.
func TestIntrinsicCalls(t *testing.T) {
	cases := []struct {
		g          bool
		text, want string
	}{
		{false, "SQRT(X)", "(call:REAL MTH$SQRT X)"},
		{false, "SQRT(D)", "(call:REAL*8 MTH$DSQRT D)"},
		{true, "SQRT(D)", "(call:REAL*8 MTH$GSQRT D)"},
		{false, "DSQRT(D)", "(call:REAL*8 MTH$DSQRT D)"},
		{true, "DSQRT(D)", "(call:REAL*8 MTH$GSQRT D)"},
		{false, "SQRT(C)", "(call:COMPLEX MTH$CSQRT C)"},
		{false, "SQRT(Z)", "(call:COMPLEX*16 MTH$CDSQRT Z)"},
		{true, "CDSQRT(Z)", "(call:COMPLEX*16 MTH$CGSQRT Z)"},
		{false, "LOG10(D)", "(call:REAL*8 MTH$DLOG10 D)"},
		{false, "ATAN2D(Y, X)", "(call:REAL MTH$ATAND2 Y X)"},
		{true, "ATAN2D(D, 1D0)", "(call:REAL*8 MTH$GATAND2 D 1.0D0)"},
		{false, "ABS(C)", "(call:REAL MTH$CABS C)"},
		{true, "ABS(Z)", "(call:REAL*8 MTH$CGABS Z)"},
		{false, "AINT(X)", "(call:REAL MTH$AINT X)"},
		{true, "ANINT(D)", "(call:REAL*8 MTH$GNINT D)"},
		{false, "MOD(X, 2.0)", "(call:REAL MTH$AMOD X 2.0)"},
		{false, "MOD(D, 2.0D0)", "(call:REAL*8 MTH$DMOD D 2.0D0)"},
		{false, "ISHFT(I, J)", "(call:INTEGER MTH$JISHFT I J)"},
		{false, "ISHFT(I2, J2)", "(call:INTEGER*2 MTH$IISHFT I2 J2)"},
		{false, "ISHFT(I2, 3)", "(shift:INTEGER*2 IISHFT I2 3)"},
		{false, "ISHFTC(I2, 1, 8)", "(call:INTEGER FTN$IISHFTC I2 1 8)"},
	}

	for _, c := range cases {
		got, ms := intrinsicOf(t, Options{GFloating: c.g}, c.text)

		if len(ms) != 0 {
			t.Errorf("%s: %v", c.text, ms)

			continue
		}

		if got != c.want {
			t.Errorf("%s (/G_FLOATING %t): got %s, want %s", c.text, c.g, got, c.want)
		}
	}

	// A REAL*16's routine is H_floating's whatever the format.
	lines := []string{"      REAL*16 Q", "      PRINT *, SQRT(Q), IQINT(Q), QMAX1(Q, Q)", "      END"}

	for _, g := range []bool{false, true} {
		prog, ms := analyzeOpts(t, Options{GFloating: g}, lines...)
		if len(ms) != 0 {
			t.Fatal(ms)
		}

		p := prog.Units[0].Body[0].(*Print)
		if got := DumpExpr(p.Items[0]); got != "(call:REAL*16 MTH$HSQRT Q)" {
			t.Errorf("SQRT(Q) (/G_FLOATING %t): %s", g, got)
		}
	}
}

// passedOf analyzes CALL S(name), name named in an INTRINSIC statement,
// returning the argument's dump.
func passedOf(t *testing.T, opts Options, name string) (string, []msgID) {
	t.Helper()

	prog, ms := analyzeOpts(t, opts, "      INTRINSIC "+name, "      CALL S("+name+")", "      END")
	if len(ms) != 0 {
		var list []msgID
		for _, m := range ms {
			list = append(list, msgID(m.Ident))
		}

		return "", list
	}

	c := prog.Units[0].Body[0].(*Call)

	return DumpExpr(c.Args[0].Expr), nil
}

// msgID is a message's identifier, in passedOf's result.
type msgID string

// TestPassedIntrinsics checks the routine an intrinsic function's name
// passes: a specific name's, a generic name's specific name of the default
// types, FTNRTL's for a name MTHRTL lacks, and none for SIZEOF.
func TestPassedIntrinsics(t *testing.T) {
	cases := []struct {
		opts       Options
		name, want string
	}{
		{Options{}, "SIN", "(procedure:REAL SIN MTH$SIN)"},
		{Options{}, "DSIN", "(procedure:REAL*8 DSIN MTH$DSIN)"},
		{Options{GFloating: true}, "DSIN", "(procedure:REAL*8 DSIN MTH$GSIN)"},
		{Options{}, "QSIN", "(procedure:REAL*16 QSIN MTH$HSIN)"},
		{Options{}, "ABS", "(procedure:REAL ABS MTH$ABS)"},
		{Options{}, "IABS", "(procedure:INTEGER IABS MTH$JIABS)"},
		{Options{NoI4: true}, "IABS", "(procedure:INTEGER*2 IABS MTH$IIABS)"},
		{Options{}, "MAX", "(procedure:INTEGER MAX MTH$JMAX0)"},
		{Options{}, "MOD", "(procedure:INTEGER MOD MTH$JMOD)"},
		{Options{}, "LOG", "(procedure:REAL LOG MTH$ALOG)"},
		{Options{}, "INT", "(procedure:INTEGER INT MTH$JINT)"},
		{Options{NoI4: true}, "INT", "(procedure:INTEGER*2 INT MTH$IINT)"},
		{Options{}, "IDINT", "(procedure:INTEGER IDINT MTH$JIDINT)"},
		{Options{GFloating: true}, "IDINT", "(procedure:INTEGER IDINT MTH$JIGINT)"},
		{Options{}, "IQNINT", "(procedure:INTEGER IQNINT MTH$JIHNNT)"},
		{Options{}, "CMPLX", "(procedure:COMPLEX CMPLX MTH$CMPLX)"},
		{Options{GFloating: true}, "DCMPLX", "(procedure:COMPLEX*16 DCMPLX MTH$GCMPLX)"},
		{Options{GFloating: true}, "DBLE", "(procedure:REAL*8 DBLE MTH$GDBLE)"},
		{Options{GFloating: true}, "SNGL", "(procedure:REAL SNGL MTH$SNGLG)"},
		{Options{}, "LEN", "(procedure:INTEGER LEN FTN$LEN)"},
		{Options{}, "CHAR", "(procedure:CHARACTER*1 CHAR FTN$CHAR)"},
		{Options{}, "BTEST", "(procedure:LOGICAL BTEST FTN$BJTEST)"},
		{Options{NoI4: true}, "IBITS", "(procedure:INTEGER IBITS FTN$IIBITS)"},
		{Options{}, "DBLEQ", "(procedure:REAL*8 DBLEQ FTN$DBLEQ)"},
		{Options{GFloating: true}, "DBLEQ", "(procedure:REAL*8 DBLEQ FTN$GBLEQ)"},
		{Options{GFloating: true}, "QEXTD", "(procedure:REAL*16 QEXTD FTN$QEXTG)"},
		{Options{}, "QFLOAT", "(procedure:REAL*16 QFLOAT FTN$QFLOAT)"},
		{Options{}, "NWORKERS", "(procedure:INTEGER NWORKERS FTN$NWORKERS)"},
	}

	for _, c := range cases {
		got, ms := passedOf(t, c.opts, c.name)
		if ms != nil {
			t.Errorf("%s: %v", c.name, ms)

			continue
		}

		if got != c.want {
			t.Errorf("%s (%+v): got %s, want %s", c.name, c.opts, got, c.want)
		}
	}

	if _, ms := passedOf(t, Options{}, "SIZEOF"); !slices.Equal(ms, []msgID{"NOTPASSABLE"}) {
		t.Errorf("SIZEOF passed: %v", ms)
	}
}

// TestIntrinsicRoutines checks every routine an intrinsic function calls or
// passes, in both formats, against the libraries: each MTH$ routine is an
// entry of VMS 7.1's MTHRTL.EXE, and the FTN$ routines are the ones the
// survey lists (Phase 78, "Routines for names MTHRTL lacks"), which FTNRTL's
// own test finds in its vector.
func TestIntrinsicRoutines(t *testing.T) {
	ftn := map[string]bool{}

	for _, f := range intrinsicForms {
		if f.how != "MTH" && f.specific == "" && f.generic != "CMPLX" && f.generic != "DCMPLX" && f.generic != "QFLOAT" {
			continue // a reference that is a conversion, with no name to pass
		}

		if f.how == "SIZEOF" {
			continue
		}

		for _, g := range []bool{false, true} {
			name := routineName(f, g)

			switch {
			case strings.HasPrefix(name, "FTN$"):
				ftn[name] = true
			case vmsdef.ImageSymbols[name].Image != "MTHRTL":
				t.Errorf("%s/%s (G %t): %s isn't MTHRTL's", f.generic, f.specific, g, name)
			}
		}
	}

	if got := sortedKeys(ftn); !slices.Equal(got, FTNRoutines) {
		t.Errorf("FTN$ routines:\n%v\nwant\n%v", got, FTNRoutines)
	}
}

// sortedKeys is a set's names in order.
func sortedKeys(set map[string]bool) []string {
	var list []string
	for name := range set {
		list = append(list, name)
	}

	slices.Sort(list)

	return list
}
