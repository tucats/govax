package sema

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
)

// intrinsicUnit declares the names TestIntrinsicRefs's expressions use.
var intrinsicUnit = []string{
	"      SUBROUTINE E(S, DA)",
	"      INTEGER*4 I, J, DA(*), N(4)",
	"      INTEGER*2 I2, J2",
	"      REAL X, Y",
	"      DOUBLE PRECISION D",
	"      COMPLEX C",
	"      COMPLEX*16 Z",
	"      CHARACTER*8 T",
	"      CHARACTER*(*) S",
	"      BYTE B",
}

// intrinsicOf analyzes text as the one item of a PRINT statement in
// intrinsicUnit with the options given, returning the item's dump.
func intrinsicOf(t *testing.T, opts Options, text string) (string, []diag.Message) {
	t.Helper()

	lines := append(append([]string{}, intrinsicUnit...), "      PRINT *, "+text, "      END")
	prog, ms := analyzeOpts(t, opts, lines...)

	p, ok := prog.Units[0].Body[0].(*Print)
	if !ok || len(p.Items) != 1 {
		t.Fatalf("%s: no item: %s", text, Dump(prog))
	}

	return DumpExpr(p.Items[0]), ms
}

// TestIntrinsicRefs checks the selection of each reference's row of Table
// D-3 (generic names by their arguments' type, footnote 10's default
// integer), its arguments' conversions, and what it becomes.
func TestIntrinsicRefs(t *testing.T) {
	cases := []struct {
		noI4       bool
		text, want string
	}{
		// Choosing: computed in the arguments' type, MAX1 made an INTEGER.
		{false, "ABS(I)", "(abs:INTEGER JIABS I)"},
		{false, "ABS(X)", "(abs:REAL ABS X)"},
		{false, "IABS(I2)", "(abs:INTEGER*2 IIABS I2)"},
		{false, "MAX(I, J, N(I))", "(max:INTEGER JMAX0 I J N[I])"},
		{false, "MAX(I2, 5)", "(max:INTEGER*2 IMAX0 I2 5)"},
		{false, "MAX(I2, 40000)", "(max:INTEGER JMAX0 (INTEGER I2) 40000)"},
		{false, "MAX(I2, J)", "(max:INTEGER JMAX0 (INTEGER I2) J)"},
		{false, "MAX1(X, Y)", "(INTEGER (max:REAL JMAX1 X Y))"},
		{false, "AMAX0(I, J)", "(REAL (max:INTEGER AJMAX0 I J))"},
		{false, "DMIN1(D, 1D0)", "(min:REAL*8 DMIN1 D 1.0D0)"},
		{false, "DIM(X, 1.0)", "(dim:REAL DIM X 1.0)"},
		{false, "SIGN(I, J)", "(sign:INTEGER JISIGN I J)"},
		{false, "MOD(I2, J2)", "(mod:INTEGER*2 IMOD I2 J2)"},
		{false, "IIABS(5)", "(abs:INTEGER*2 IIABS 5)"},

		// Conversions (footnote 11: INT of an integer, REAL of a REAL*4,
		// are the value).
		{false, "INT(X)", "(INTEGER X)"},
		{false, "INT(I2)", "(INTEGER I2)"},
		{false, "INT(I)", "I"},
		{false, "REAL(X)", "X"},
		{false, "REAL(C)", "(REAL C)"},
		{false, "REAL(Z)", "(REAL Z)"},
		{false, "DREAL(Z)", "(REAL*8 Z)"},
		{false, "DBLE(I2)", "(REAL*8 I2)"},
		{false, "QEXT(D)", "(REAL*16 D)"},
		{false, "IFIX(X)", "(INTEGER X)"},
		{false, "CMPLX(I)", "(COMPLEX I)"},
		{false, "DCMPLX(X, I)", "(cmplx:COMPLEX*16 DCMPLX (REAL*8 X) (REAL*8 I))"},
		{false, "AIMAG(C)", "(imag:REAL AIMAG C)"},
		{false, "CONJG(Z)", "(conjg:COMPLEX*16 DCONJG Z)"},
		{false, "DPROD(X, Y)", "(*:R (REAL*8 X) (REAL*8 Y))"},

		// Footnote 10: the default integer.
		{false, "NINT(X)", "(nint:INTEGER JNINT X)"},
		{true, "NINT(X)", "(nint:INTEGER*2 ININT X)"},
		{true, "INT(X)", "(INTEGER*2 X)"},
		{true, "INT(I)", "(INTEGER*2 I)"},
		{true, "MAX1(X, Y)", "(INTEGER*2 (max:REAL IMAX1 X Y))"},
		{true, "IDNINT(D)", "(nint:INTEGER*2 IIDNNT D)"},
		{false, "ZEXT(B)", "(zext:INTEGER JZEXT B)"},
		{true, "ZEXT(B)", "(zext:INTEGER*2 IZEXT B)"},
		{true, "ZEXT(I)", "(zext:INTEGER JZEXT I)"},

		// The bit functions: the higher argument decides (D.5.2).
		{false, "IAND(I, J)", "(.AND.:I I J)"},
		{false, "IOR(I2, 255)", "(.OR.:I I2 255)"},
		{false, "IEOR(I2, J)", "(.XOR.:I (INTEGER I2) J)"},
		{false, "NOT(I2)", "(.NOT.:I I2)"},
		{false, "ISHFT(I2, 3)", "(shift:INTEGER*2 IISHFT I2 3)"},
		{false, "IBITS(I2, J, 2)", "(bits:INTEGER JIBITS (INTEGER I2) J 2)"},
		{false, "IBITS(I2, J2, 2)", "(bits:INTEGER IIBITS I2 J2 2)"},
		{false, "IBCLR(I2, 1)", "(bitclear:INTEGER IIBCLR I2 1)"},
		{false, "IBSET(I2, 1)", "(bitset:INTEGER*2 IIBSET I2 1)"},
		{false, "BTEST(I2, 1)", "(bittest:LOGICAL*2 BITEST I2 1)"},
		{false, "ISHFTC(I, 1, 8)", "(call:INTEGER FTN$JISHFTC I 1 8)"},

		// The character functions.
		{false, "LEN(T)", "8"},
		{false, "LEN(T(I:))", "(len:INTEGER LEN T(I:))"},
		{false, "LEN(S)", "(len:INTEGER LEN S)"},
		{false, "INDEX('ABCB', 'CB')", "3"},
		{false, "INDEX(S, 'B')", "(index:INTEGER INDEX S 'B')"},
		{false, "ICHAR(S)", "(ichar:INTEGER ICHAR S)"},
		{false, "CHAR(I)", "(char:CHARACTER*1 CHAR I)"},
		{false, "LLT(S, T)", "(.LT.:C S T)"},
		{false, "LGE('B', 'A')", ".TRUE."},

		// The constants.
		{false, "NWORKERS()", "1"},
		{false, "SIZEOF(N)", "16"},
		{false, "SIZEOF(T(2:3))", "2"},
		{false, "SIZEOF(D + 1)", "8"},
		{false, "SIZEOF(Z)", "16"},
	}

	for _, c := range cases {
		got, ms := intrinsicOf(t, Options{NoI4: c.noI4}, c.text)

		if len(ms) != 0 {
			t.Errorf("%s: %v", c.text, ms)

			continue
		}

		if got != c.want {
			t.Errorf("%s (/NOI4 %t): got %s, want %s", c.text, c.noI4, got, c.want)
		}
	}
}

// TestIntrinsicErrors checks the messages of references the table refuses.
func TestIntrinsicErrors(t *testing.T) {
	cases := map[string]string{
		"MOD(I)":           "INTRINARGS: the intrinsic function MOD takes 2 arguments, not 1",
		"MAX(I)":           "INTRINARGS: the intrinsic function MAX takes 2 or more arguments, not 1",
		"CMPLX(X, Y, X)":   "INTRINARGS: the intrinsic function CMPLX takes 1 or 2 arguments, not 3",
		"NWORKERS(I)":      "INTRINARGS: the intrinsic function NWORKERS takes 0 arguments, not 1",
		"MAX(I, X)":        "BADARG: the intrinsic function MAX's arguments must be INTEGER, like the first, and this one is REAL",
		"IAND(I, X)":       "BADARG: the intrinsic function IAND's arguments must be INTEGER*2 or INTEGER, and this one is REAL",
		"CHAR(I, J)":       "INTRINARGS: the intrinsic function CHAR takes 1 argument, not 2",
		"MIN(X, D, I)":     "BADARG: the intrinsic function MIN's arguments must be REAL, like the first, and this one is INTEGER",
		"IAND(X, I)":       "BADARG: the intrinsic function IAND's arguments must be INTEGER*2 or INTEGER, and this one is REAL",
		"JIABS(I2)":        "BADARG: the intrinsic function JIABS's arguments must be INTEGER, and this one is INTEGER*2",
		"ICHAR(I)":         "BADARG: the intrinsic function ICHAR's arguments must be CHARACTER, and this one is INTEGER",
		"CMPLX(C, X)":      "BADARG: the intrinsic function CMPLX's arguments must be INTEGER or REAL, and this one is COMPLEX",
		"SIZEOF(DA)":       "BADARG: the intrinsic function SIZEOF's arguments must be anything but an assumed-size array or a passed-length value, and this one is an assumed-size array",
		"SIZEOF(S)":        "BADARG: the intrinsic function SIZEOF's arguments must be anything but an assumed-size array or a passed-length value, and this one is CHARACTER*(*)",
		"MAX(I, 2) + I(1)": "NOTARRAY",
	}

	for text, want := range cases {
		_, ms := intrinsicOf(t, Options{}, text)

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

// TestIntrinsicTable checks the table against appendix D: Table D-3's 219
// names, and the constant functions of section 4.11 (constFuncs, Decision 7)
// named in it with the same argument counts.
func TestIntrinsicTable(t *testing.T) {
	if len(intrinsics) != 219 {
		t.Errorf("%d intrinsic function names, want Table D-3's 219", len(intrinsics))
	}

	for name, fn := range constFuncs {
		if name == "IMAG" {
			continue // section 4.11's name for AIMAG, not Table D-3's
		}

		forms := formsNamed(name)
		if len(forms) == 0 {
			t.Errorf("%s: a constant function the table hasn't", name)

			continue
		}

		if got := takes(forms); got != fn.takes {
			t.Errorf("%s: the table takes %s, the constant function %s", name, got, fn.takes)
		}
	}
}
