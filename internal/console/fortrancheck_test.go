package console

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// checkPrograms are small programs that do or don't trip a /CHECK (docs/PHASE-85 -
// FORTRAN compiler usability.md, subtask 5). Each is compiled both ways with each
// optimizer setting, by TestFortranCheck, since what the optimizer may do to a check
// is the riskiest part of it. checked is what it prints or signals compiled with
// qualifier, and plain what it prints without; refused, if not empty, a condition it
// must not raise compiled with qualifier.
var checkPrograms = []struct {
	name, qualifier string
	checked, plain  string
	refused         string
	source          string
}{
	{
		// Forwarding finds the subscript is 7, which without the check becomes T.A+24.
		name: "KNOWNBAD", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      INTEGER A(3), I, J
      I = 7
      J = A(I)
      PRINT *, 'AFTER'
      END`,
	},
	{
		// The value is never used, but its check must still run.
		name: "DEADSTORE", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      INTEGER A(3), I, J
      READ (5, *, END=10) I
   10 I = I + 7
      J = A(I)
      PRINT *, 'AFTER'
      END`,
	},
	{
		// A constant subscript out of range warns, and still traps if it runs.
		name: "CONSTANT", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      INTEGER A(3), J
      J = A(7)
      PRINT *, 'AFTER'
      END`,
	},
	{
		// The second operand of .AND. isn't reached, so neither is its check.
		name: "GUARDED", qualifier: "/CHECK=BOUNDS", checked: "FINE", plain: "FINE",
		refused: "SUBRNG",
		source: `
      INTEGER A(3), I
      DATA A /1, 2, 3/
      I = 0
      IF (I .GE. 1 .AND. A(I) .GT. 0) PRINT *, 'WRONG'
      PRINT *, 'FINE'
      END`,
	},
	{
		// A loop that never runs mustn't have its check moved ahead of it.
		name: "NEVERRUN", qualifier: "/CHECK=BOUNDS", checked: "FINE", plain: "FINE",
		refused: "SUBRNG",
		source: `
      INTEGER A(3), K, N, I, J
      K = 9
      N = 0
      DO 10 I = 1, N
         J = A(K)
   10 CONTINUE
      PRINT *, 'FINE'
      END`,
	},
	{
		name: "TWODIM", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      INTEGER A(3,4), I, J, K
      I = 3
      J = 5
      K = A(I,J)
      PRINT *, 'AFTER'
      END`,
	},
	{
		name: "RECORD", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      STRUCTURE /S/
         INTEGER F(2)
      END STRUCTURE
      RECORD /S/ R(3)
      INTEGER I, J
      I = 3
      J = R(1).F(I)
      PRINT *, 'AFTER'
      END`,
	},
	{
		name: "RECORDS", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      STRUCTURE /S/
         INTEGER F(2)
      END STRUCTURE
      RECORD /S/ R(3)
      INTEGER I, J
      I = 4
      J = R(I).F(1)
      PRINT *, 'AFTER'
      END`,
	},
	{
		name: "ADJUSTABLE", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      INTEGER A(5)
      CALL SUB(A, 2)
      PRINT *, 'AFTER'
      END
      SUBROUTINE SUB(X, N)
      INTEGER N, X(N), I
      I = 3
      X(I) = 0
      END`,
	},
	{
		name: "ADJLOWER", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      INTEGER A(5)
      CALL SUB(A(3), 2)
      PRINT *, 'AFTER'
      END
      SUBROUTINE SUB(X, N)
      INTEGER N, X(N:N+1), I
      I = 1
      X(I) = 0
      END`,
	},
	{
		// An upper bound below the lower, however the array is used.
		name: "ADJDIM", qualifier: "/CHECK=BOUNDS", checked: "%FOR-F-ADJARRDIM", plain: "AFTER",
		source: `
      INTEGER A(5)
      CALL SUB(A, 0)
      PRINT *, 'AFTER'
      END
      SUBROUTINE SUB(X, N)
      INTEGER N, X(N)
      END`,
	},
	{
		// ... and at an ENTRY statement's entry point too.
		name: "ADJENTRY", qualifier: "/CHECK=BOUNDS", checked: "%FOR-F-ADJARRDIM", plain: "AFTER",
		source: `
      INTEGER A(5)
      CALL ALT(A, 0)
      PRINT *, 'AFTER'
      END
      SUBROUTINE SUB
      INTEGER N, X(N)
      RETURN
      ENTRY ALT(X, N)
      END`,
	},
	{
		// An assumed-size array has no upper bound to check, nor a dummy's (1).
		name: "ASSUMED", qualifier: "/CHECK=BOUNDS", checked: "FINE", plain: "FINE",
		refused: "SUBRNG",
		source: `
      INTEGER A(5)
      CALL SUB(A, 4)
      CALL OLD(A, 5)
      PRINT *, 'FINE'
      END
      SUBROUTINE SUB(X, I)
      INTEGER X(*), I
      X(I) = 0
      X(3) = 0
      END
      SUBROUTINE OLD(X, I)
      INTEGER X(1), I
      X(I) = 0
      X(4) = 0
      END`,
	},
	{
		name: "ASSUMEDLOW", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      INTEGER A(5)
      CALL SUB(A(3), 0)
      PRINT *, 'AFTER'
      END
      SUBROUTINE SUB(X, I)
      INTEGER X(*), I
      X(I) = 0
      END`,
	},
	{
		name: "SUBSTRUP", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      CHARACTER*5 S
      INTEGER J
      S = 'ABCDE'
      J = 6
      IF (S(1:J) .EQ. 'X') PRINT *, 'WRONG'
      PRINT *, 'AFTER'
      END`,
	},
	{
		name: "SUBSTRLOW", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      CHARACTER*5 S
      INTEGER I
      S = 'ABCDE'
      I = 0
      IF (S(I:) .EQ. 'X') PRINT *, 'WRONG'
      PRINT *, 'AFTER'
      END`,
	},
	{
		// Each bound is in the string, but they are the wrong way round.
		name: "SUBSTRORDER", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      CHARACTER*5 S
      INTEGER I, J
      S = 'ABCDE'
      I = 4
      J = 2
      IF (S(I:J) .EQ. 'X') PRINT *, 'WRONG'
      PRINT *, 'AFTER'
      END`,
	},
	{
		name: "SUBSTRONE", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      CHARACTER*5 S
      INTEGER I
      S = 'ABCDE'
      I = 6
      IF (S(I:I) .EQ. 'X') PRINT *, 'WRONG'
      PRINT *, 'AFTER'
      END`,
	},
	{
		// A passed-length dummy's length is known only when the program runs.
		name: "SUBSTRPASSED", qualifier: "/CHECK=BOUNDS", checked: "%SYSTEM-F-SUBRNG", plain: "AFTER",
		source: `
      CALL SUB('ABC', 4)
      PRINT *, 'AFTER'
      END
      SUBROUTINE SUB(S, I)
      CHARACTER*(*) S
      INTEGER I
      IF (S(I:I) .EQ. 'X') PRINT *, 'WRONG'
      END`,
	},
	{
		// Forwarding knows both operands, and folding would hide the overflow.
		name: "OVERFLOW", qualifier: "/CHECK=OVERFLOW", checked: "%SYSTEM-F-INTOVF", plain: "AFTER",
		source: `
      INTEGER I, J
      I = 2147483647
      J = I + 1
      PRINT *, 'AFTER'
      END`,
	},
	{
		// The value is never used, but with overflow on its computing may trap.
		name: "OVERDEAD", qualifier: "/CHECK=OVERFLOW", checked: "%SYSTEM-F-INTOVF", plain: "AFTER",
		source: `
      INTEGER I, J
      READ (5, *, END=10) I
   10 I = 2147483647
      J = I * 2
      PRINT *, 'AFTER'
      END`,
	},
	{
		// A loop that never runs mustn't have its arithmetic moved ahead of it.
		name: "OVERNEVER", qualifier: "/CHECK=OVERFLOW", checked: "FINE", plain: "FINE",
		refused: "INTOVF",
		source: `
      INTEGER K, N, I, J
      READ (5, *, END=10) N
   10 K = 2147483647
      N = 0
      DO 20 I = 1, N
         J = K + 1
   20 CONTINUE
      PRINT *, 'FINE'
      END`,
	},
	{
		// A stepped index mustn't run ahead of the program either.
		name: "OVERSTEP", qualifier: "/CHECK=OVERFLOW", checked: "FINE", plain: "FINE",
		refused: "INTOVF",
		source: `
      INTEGER A(1000,1000), I, J, N, M
      READ (5, *, END=10) M
   10 N = 3
      J = 1
      DO 20 I = 1, N
         A(I,I) = I
         J = J + A(I,I)
   20 CONTINUE
      IF (J .EQ. 7) PRINT *, 'FINE'
      END`,
	},
	{
		// Each routine's IV is its own: a callee compiled without it doesn't trap, and
		// RET gives the caller's back.
		name: "OVERUNIT", qualifier: "", checked: "%SYSTEM-F-INTOVF", plain: "%SYSTEM-F-INTOVF",
		source: `
      OPTIONS /CHECK=OVERFLOW
      INTEGER I, J, WRAP
      I = WRAP(2147483647)
      PRINT *, 'WRAPPED'
      IF (I .NE. -2147483648) PRINT *, 'WRONG'
      J = I - 1
      PRINT *, 'AFTER'
      END
      OPTIONS /NOCHECK
      INTEGER FUNCTION WRAP(I)
      INTEGER I
      WRAP = I + 1
      END`,
	},
	{
		// Shifts and bit functions aren't arithmetic. ASHL would set V for each of these.
		name: "OVERBITS", qualifier: "/CHECK=OVERFLOW", checked: "FINE", plain: "FINE",
		refused: "INTOVF",
		source: `
      INTEGER I, J, K, N
      I = 'F0000001'X
      J = ISHFT(I, 4)
      N = 31
      K = IBSET(0, N)
      IF (J .EQ. 16 .AND. K .LT. 0) PRINT *, 'FINE'
      K = IBCLR(-1, N)
      IF (K .NE. 2147483647) PRINT *, 'WRONG'
      END`,
	},
	{
		name: "UNDERFLOW", qualifier: "/CHECK=UNDERFLOW", checked: "%SYSTEM-F-FLTUND_F", plain: "AFTER",
		source: `
      REAL X, Y
      X = 1.0E-30
      Y = X * X
      PRINT *, 'AFTER'
      END`,
	},
	{
		// In a variable format expression, whose routine has an entry point of its own.
		name: "OVERVFE", qualifier: "/CHECK=OVERFLOW", checked: "%SYSTEM-F-INTOVF", plain: "AFTER",
		source: `
      INTEGER I
      READ (5, *, END=10) I
   10 I = 2147483647
      WRITE (6, 100) 1
  100 FORMAT (1X, I<I+I+4>)
      PRINT *, 'AFTER'
      END`,
	},
	{
		// /NOCHECK after /CHECK, and OPTIONS's forms, cut short.
		name: "OPTIONS", qualifier: "/CHECK=ALL", checked: "FINE", plain: "FINE",
		refused: "SUBRNG",
		source: `
      OPTIONS /CHECK=(NOBOU, OVER)
      INTEGER A(3), I, J
      I = 2
      J = I + 1
      I = 3
      J = A(I+1)
      PRINT *, 'FINE'
      END`,
	},
}

// TestFortranCheck compiles and runs each of checkPrograms, with the check and without,
// under both optimizer settings.
func TestFortranCheck(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		for _, p := range checkPrograms {
			t.Run(p.name, func(t *testing.T) {
				if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + p.name + ".FOR"},
					rms.TextRecords, splitSource(strings.TrimPrefix(p.source, "\n"))); err != nil {
					t.Fatal(err)
				}

				deleteFilesAfter(t, d, p.name)

				for _, mode := range optimizerModes {
					for _, run := range []struct{ qualifier, want, refused string }{
						{p.qualifier, p.checked, p.refused},
						{"", p.plain, ""},
					} {
						buf.Reset()
						shareFindRun(t, d, "FORTRAN"+mode.qualifier+run.qualifier+" "+p.name)
						shareFindRun(t, d, "MACRO "+p.name)
						shareFindRun(t, d, "LINK "+p.name)

						c.In = strings.NewReader("")

						got := shareRun(t, d, buf, p.name)
						if !strings.Contains(got, run.want) {
							t.Errorf("FORTRAN%s%s printed:\n%s\nwant %s", mode.qualifier, run.qualifier, got, run.want)
						}

						if run.refused != "" && strings.Contains(got, run.refused) {
							t.Errorf("FORTRAN%s%s printed:\n%s\nwhich has %s", mode.qualifier, run.qualifier, got, run.refused)
						}
					}
				}
			})
		}
	})
}

// TestFortranCheckQualifier is the command's side of /CHECK: its forms, its refusal of a
// keyword that isn't one, and OPTIONS's, and the warning of a constant subscript.
func TestFortranCheckQualifier(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	source := func(name, text string) {
		t.Helper()

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".FOR"},
			rms.TextRecords, splitSource(text)); err != nil {
			t.Fatal(err)
		}

		deleteFilesAfter(t, d, name)
	}

	source("QUAL", "      INTEGER A(3), I\n      I = 2\n      A(I) = 0\n      END\n")

	macro := func() string {
		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]QUAL.MAR"}, rms.TextRecords)
		if err != nil {
			t.Fatal(err)
		}

		var text strings.Builder

		for _, r := range records {
			text.Write(r)
			text.WriteByte('\n')
		}

		return text.String()
	}

	for _, f := range []struct {
		qualifier     string
		index, iv, fu bool
	}{
		{"", false, false, false},
		{"/NOCHECK", false, false, false},
		{"/CHECK", true, true, true},
		{"/CHECK=ALL", true, true, true},
		{"/CHECK=NONE", false, false, false},
		{"/CHECK=BOUNDS", true, false, false},
		{"/CHECK=(BOU,OVER)", true, true, false},
		{"/CHECK=UNDERFLOW", false, false, true},
		{"/CHECK=(ALL,NOBOUNDS)", false, true, true},
	} {
		shareFindRun(t, d, "FORTRAN/NOOPTIMIZE"+f.qualifier+" QUAL")

		text := macro()
		// The header has the command in it, so what's looked for is the instructions.
		code := text[strings.Index(text, ".ENTRY"):]

		if got := strings.Contains(code, "INDEX\t") || strings.Contains(code, "INDEX "); got != f.index {
			t.Errorf("%q: INDEX %v, want %v:\n%s", f.qualifier, got, f.index, code)
		}

		if got := strings.Contains(code, "IV>"); got != f.iv {
			t.Errorf("%q: IV %v, want %v:\n%s", f.qualifier, got, f.iv, code)
		}

		if got := strings.Contains(code, "BISPSW"); got != f.fu {
			t.Errorf("%q: BISPSW %v, want %v:\n%s", f.qualifier, got, f.fu, code)
		}
	}

	if err := d.Dispatch("FORTRAN/CHECK=SOMETIMES QUAL"); !errors.Is(err, vmserrors.New(vmserrors.CLI_FORCHECK)) || !strings.Contains(err.Error(), "SOMETIMES") {
		t.Errorf("a keyword that isn't one: %v", err)
	}

	source("BADOPT", "      OPTIONS /CHECK=SOMETIMES\n      END\n")
	buf.Reset()

	if err := d.Dispatch("FORTRAN BADOPT"); err == nil || !strings.Contains(buf.String(), "%FORT-E-BADCHECK") {
		t.Errorf("OPTIONS's keyword: %v, said %q", err, buf.String())
	}

	source("WARNS", `      STRUCTURE /T/
         INTEGER F(2)
      END STRUCTURE
      RECORD /T/ R(3)
      INTEGER A(3), B(2:4), I
      CHARACTER*2 S(2)
      A(1) = 0
      A(3) = 0
      B(2) = 0
      B(4) = 0
      I = A(4)
      I = A(0)
      I = B(1)
      S(3) = 'X'
      R(4).F(1) = 0
      R(1).F(3) = 0
      END
      SUBROUTINE SUB(X, Y, N)
      INTEGER N, X(*), Y(N)
      X(99) = 0
      Y(99) = 0
      X(0) = 0
      END
`)
	buf.Reset()
	shareFindRun(t, d, "FORTRAN WARNS")

	text := buf.String()
	if got := strings.Count(text, "%FORT-W-SUBSCRIPTRANGE"); got != 7 {
		t.Errorf("%d warnings, want 7:\n%s", got, text)
	}

	for _, want := range []string{"subscript 4 of A is past its upper bound, 3", "subscript 1 of B is past its lower bound, 2", "subscript 0 of X is past its lower bound, 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}

// TestFortranWarningsQualifier is /WARNINGS at the command: what's said, the keywords
// refused, and the status, which follows what's shown.
func TestFortranWarningsQualifier(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]WQUAL.FOR"},
		rms.TextRecords, splitSource("      INTEGER A(3), IDLE\n      A(7) = K\n      END\n")); err != nil {
		t.Fatal(err)
	}

	deleteFilesAfter(t, d, "WQUAL")

	for _, f := range []struct {
		qualifier string
		says      []string
		status    string
	}{
		{"", []string{"SUBSCRIPTRANGE"}, "warning"},
		{"/WARNINGS=GENERAL", []string{"SUBSCRIPTRANGE"}, "warning"},
		{"/NOWARNINGS", nil, "normal"},
		{"/WARNINGS=NONE", nil, "normal"},
		{"/WARNINGS=(NONE,UNUSED)", []string{"UNUSEDNAME"}, "warning"},
		{"/WARNINGS=(NOGENERAL,DECL)", []string{"UNDECLARED"}, "warning"},
		{"/WARNINGS", []string{"SUBSCRIPTRANGE", "UNDECLARED", "UNUSEDNAME"}, "warning"},
		// What's held back includes a message of the command's own.
		{"/CROSS_REFERENCE", []string{"NOLISTING", "SUBSCRIPTRANGE"}, "warning"},
		{"/CROSS_REFERENCE/NOWARNINGS", nil, "normal"},
	} {
		buf.Reset()
		shareFindRun(t, d, "FORTRAN"+f.qualifier+" WQUAL")

		text := buf.String()

		for _, ident := range []string{"NOLISTING", "SUBSCRIPTRANGE", "UNDECLARED", "UNUSEDNAME"} {
			if got, want := strings.Contains(text, "-"+ident+","), slices.Contains(f.says, ident); got != want {
				t.Errorf("%q: %s %v, want %v:\n%s", f.qualifier, ident, got, want, text)
			}
		}

		buf.Reset()
		shareFindRun(t, d, `WRITE SYS$OUTPUT F$INTEGER($SEVERITY)`)

		severity := strings.TrimSpace(buf.String())
		if want := map[string]string{"warning": "0", "normal": "1"}[f.status]; severity != want {
			t.Errorf("%q: $SEVERITY %s, want %s", f.qualifier, severity, want)
		}
	}

	if err := d.Dispatch("FORTRAN/WARNINGS=BOGUS WQUAL"); err == nil || !strings.Contains(err.Error(), "no keyword BOGUS") {
		t.Errorf("/WARNINGS=BOGUS: %v", err)
	}
}
