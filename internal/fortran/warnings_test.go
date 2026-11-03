package fortran

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
)

// warningsOf is what a program compiled with the /WARNINGS keywords says, each message
// its identifier and its line.
func warningsOf(t *testing.T, opts Options, keywords []string, lines ...string) []string {
	t.Helper()

	warnings, err := diag.Warnings{}.With(keywords)
	if err != nil {
		t.Fatal(err)
	}

	opts.Warnings = warnings

	r := Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: lines})

	said := []string{}

	for _, m := range r.Messages {
		said = append(said, m.Ident+" "+strconv.Itoa(m.Pos.Line)+" "+m.Text)
	}

	return said
}

// TestWarningsDeclarations has each kind of name IMPLICIT NONE refuses, and those it
// doesn't. A statement function's dummy argument is in no table, and is warned of too.
func TestWarningsDeclarations(t *testing.T) {
	program := []string{
		"      FUNCTION FN(ARG, TYPED)",
		"      REAL TYPED, HAS(2)",
		"      PARAMETER (NEWPAR = 1)",
		"      PARAMETER OLDPAR = 2",
		"      EXTERNAL UNCALLED",
		"      INTRINSIC COS",
		"      DIMENSION ARR(3)",
		"      STF(Q) = Q + 1",
		"      VAR = ARG + TYPED + HAS(1) + ARR(1) + NEWPAR + OLDPAR",
		"      FN = STF(VAR) + OTHER(VAR) + COS(VAR) + SIN(VAR)",
		"      CALL SUB",
		"      END",
	}

	got := warningsOf(t, testOptions, []string{"DECLARATIONS"}, program...)

	want := []string{
		"UNDECLARED 1 FN is used but not typed, and its first letter makes it REAL",
		"UNDECLARED 1 ARG is used but not typed, and its first letter makes it REAL",
		"UNDECLARED 3 NEWPAR is used but not typed, and its first letter makes it INTEGER",
		"UNDECLARED 7 ARR is used but not typed, and its first letter makes it REAL",
		"UNDECLARED 8 Q is used but not typed, and its first letter makes it REAL",
		"UNDECLARED 8 STF is used but not typed, and its first letter makes it REAL",
		"UNDECLARED 9 VAR is used but not typed, and its first letter makes it REAL",
		"UNDECLARED 10 OTHER is used but not typed, and its first letter makes it REAL",
	}

	slices.Sort(got)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	if got := warningsOf(t, testOptions, []string{"GENERAL"}, "      IMPLICIT NONE", "      INTEGER I", "      I = 1", "      PRINT *, I", "      END"); len(got) != 0 {
		t.Errorf("under IMPLICIT NONE: %v", got)
	}
}

// TestWarningsImplicitNoneIntrinsic: an INTRINSIC name needs no type, under IMPLICIT
// NONE either, and a name with none is still an error there, whatever /WARNINGS says.
func TestWarningsImplicitNoneIntrinsic(t *testing.T) {
	got := warningsOf(t, testOptions, []string{"NONE"},
		"      IMPLICIT NONE",
		"      REAL X",
		"      INTRINSIC COS",
		"      X = COS(1.0) + Y",
		"      PRINT *, X",
		"      END")

	if len(got) != 1 || !strings.HasPrefix(got[0], "NOTYPE 4 ") {
		t.Errorf("got %v", got)
	}
}

// TestWarningsUnused has a name of each kind only declared, and each kind that isn't
// reported though it looks it. Nothing is said without the keyword.
func TestWarningsUnused(t *testing.T) {
	opts := testOptions
	opts.Includes = memoryIncludes{"I": {"      PARAMETER (SHARED = 1)", "      INTEGER ALSO"}}

	program := []string{
		"      SUBROUTINE SUB(IDLE, USED, EXT, *)",
		"      EXTERNAL EXT, BLOCK",
		"      INTEGER IDLE, USED, LOCAL, ASSIGNED, INCOMN, EQ1, EQ2",
		"      INTEGER KONST, BOUND",
		"      PARAMETER (KONST = 1, BOUND = 3)",
		"      REAL ARRAY(BOUND), DIM2",
		"      DIMENSION DIM2(2)",
		"      SAVE LOCAL",
		"      COMMON /C/ INCOMN",
		"      EQUIVALENCE (EQ1, EQ2)",
		"      INTRINSIC SIN",
		"      INCLUDE 'I'",
		"      REAL TYPER",
		"      ST(Q) = Q",
		"      ST2(TYPER) = TYPER",
		"      ASSIGNED = USED",
		"      PRINT 100, EQ2, ARRAY",
		"100   FORMAT (2I5)",
		"      DO 200 I = 1, 2",
		"200   CONTINUE",
		"300   CONTINUE",
		"400   CONTINUE",
		"      GO TO 400",
		"      DO 500 WHILE (USED .LT. 1)",
		"500   CONTINUE",
		"      RETURN",
		"      ENTRY OTHER(USED)",
		"      END",
	}

	got := warningsOf(t, opts, []string{"UNUSED"}, program...)

	want := []string{
		"UNUSEDDUMMY 1 the dummy argument IDLE is never used",
		"UNUSEDDUMMY 1 the dummy argument EXT is never used",
		"UNUSEDNAME 3 LOCAL is declared but never used",
		"UNUSEDNAME 4 KONST is declared but never used",
		"UNUSEDNAME 6 DIM2 is declared but never used",
		"UNUSEDNAME 11 SIN is declared but never used",
		"UNUSEDNAME 14 ST is declared but never used",
		"UNUSEDNAME 15 ST2 is declared but never used",
		"UNUSEDLABEL 21 statement label 300 is never used",
	}

	if !slices.Equal(got, want) {
		t.Errorf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	opts.Warnings = diag.Warnings{}

	if r := Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: program}); len(r.Messages) != 0 {
		t.Errorf("by default: %v", r.Messages)
	}
}

// TestWarningsUnusedAfterError: a statement with an error may have lost its mentions, so
// nothing is said of unused names.
func TestWarningsUnusedAfterError(t *testing.T) {
	got := warningsOf(t, testOptions, []string{"UNUSED"},
		"      INTEGER I, J",
		"      GO TO 10",
		"      I = (J",
		"      END")

	for _, said := range got {
		if strings.HasPrefix(said, "UNUSED") {
			t.Errorf("after an error: %v", got)
		}
	}

	if len(got) == 0 {
		t.Error("no error reported")
	}
}

// TestWarningsGeneral: NOGENERAL holds back warnings and informational messages but not
// errors, or what a keyword of its own asks for; and what's held back doesn't count.
func TestWarningsGeneral(t *testing.T) {
	warned := []string{
		"      INTEGER A(3), IDLE",
		"      A(7) = 0",
		"      END",
	}

	cases := []struct {
		keywords []string
		want     []string
	}{
		{[]string{"GENERAL"}, []string{"SUBSCRIPTRANGE"}},
		{[]string{"NOGENERAL"}, nil},
		{[]string{"NONE"}, nil},
		{[]string{"NONE", "GENERAL"}, []string{"SUBSCRIPTRANGE"}},
		{[]string{"NOGENERAL", "UNUSED"}, []string{"UNUSEDNAME"}},
		{nil, []string{"SUBSCRIPTRANGE", "UNUSEDNAME"}},
		{[]string{"ALL", "NOUNUSED"}, []string{"SUBSCRIPTRANGE"}},
		{[]string{"NOGEN", "UNU"}, []string{"UNUSEDNAME"}},
	}

	for _, c := range cases {
		said := []string{}

		for _, line := range warningsOf(t, testOptions, c.keywords, warned...) {
			said = append(said, line[:strings.IndexByte(line, ' ')])
		}

		slices.Sort(said)

		if !slices.Equal(said, append([]string{}, c.want...)) {
			t.Errorf("%v: said %v, want %v", c.keywords, said, c.want)
		}
	}

	opts := testOptions
	opts.Warnings.NoGeneral = true

	r := Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: warned})
	if r.Warnings != 0 || r.MACRO == nil {
		t.Errorf("held back: %d warnings, MACRO %v", r.Warnings, r.MACRO != nil)
	}

	r = Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: []string{"      GO TO 10", "      END"}})
	if r.Errors != 1 || len(r.Messages) != 1 {
		t.Errorf("an error held back: %v", r.Messages)
	}
}

// TestWarningsAlignment: the keyword is CDEC$ OPTIONS /WARN=ALIGNMENT from each unit's
// start, which a source's own directive can turn off for a group, or on under NONE.
func TestWarningsAlignment(t *testing.T) {
	common := []string{
		"      BYTE B",
		"      INTEGER K",
		"      COMMON /C/ B, K",
		"      K = B",
		"      END",
	}

	got := warningsOf(t, testOptions, []string{"ALIGNMENT"}, common...)
	if len(got) != 1 || !strings.HasPrefix(got[0], "ALIGNCOMMON ") {
		t.Errorf("ALIGNMENT: %v", got)
	}

	if got := warningsOf(t, testOptions, []string{"NOALIGNMENT"}, common...); len(got) != 0 {
		t.Errorf("NOALIGNMENT: %v", got)
	}

	if got := warningsOf(t, testOptions, []string{"NONE", "ALIGNMENT"}, common...); len(got) != 1 {
		t.Errorf("(NONE,ALIGNMENT): %v", got)
	}

	asked := []string{
		"CDEC$ OPTIONS /WARN=ALIGNMENT",
		"      BYTE B",
		"      INTEGER K",
		"      COMMON /C/ B, K",
		"CDEC$ END OPTIONS",
		"      K = B",
		"      END",
	}

	if got := warningsOf(t, testOptions, []string{"NONE"}, asked...); len(got) != 1 {
		t.Errorf("a directive under NONE: %v", got)
	}
}

// TestWarningsListing: a listing has only what's shown.
func TestWarningsListing(t *testing.T) {
	opts := testOptions
	opts.List = true
	opts.Show.Map = true

	program := []string{"      INTEGER A(3)", "      A(7) = 0", "      END"}

	has := func(r Result) bool {
		return strings.Contains(strings.Join(r.Listing, "\n"), "SUBSCRIPTRANGE")
	}

	if !has(Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: program})) {
		t.Error("by default the message isn't in the listing")
	}

	opts.Warnings.NoGeneral = true

	if has(Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: program})) {
		t.Error("under NOGENERAL the message is in the listing")
	}
}

// TestWarningsSystemFunctions: a declared system function that a statement calls is
// used, as a declared intrinsic is, and one nothing calls isn't.
func TestWarningsSystemFunctions(t *testing.T) {
	got := warningsOf(t, Options{}, []string{"UNUSED"},
		"      REAL RAN, SECNDS, X",
		"      INTEGER SEED",
		"      SEED = 5",
		"      X = RAN(SEED) + SECNDS(0.0)",
		"      PRINT *, X",
		"      END")

	if len(got) != 0 {
		t.Errorf("called system functions said %q", got)
	}

	got = warningsOf(t, Options{}, []string{"UNUSED"},
		"      REAL RAN",
		"      END")

	if want := []string{"UNUSEDNAME 1 RAN is declared but never used"}; !slices.Equal(got, want) {
		t.Errorf("an uncalled system function said %q, want %q", got, want)
	}
}
