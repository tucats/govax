package sema

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/source"
)

// update rewrites the golden files from what semantic analysis
// produces: go test ./internal/fortran/sema -update. Review the diff
// before committing it.
var update = flag.Bool("update", false, "rewrite the golden .sema files")

// analyzeLines runs the first four passes over lines as the fixed-format
// file spec, a main program without PROGRAM being named T$MAIN.
func analyzeLines(t *testing.T, spec string, lines ...string) (*Program, []diag.Message) {
	t.Helper()

	var messages diag.List

	program := source.Read(source.Form{}, &source.File{Spec: spec, Lines: lines}, nil, &messages)
	file := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	prog := Analyze(file, "T$MAIN", &messages)

	return prog, messages.Messages()
}

// TestGolden analyzes each program in testdata and compares its dump
// and messages with the .sema file beside it.
func TestGolden(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join("testdata", "*.for"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no programs in testdata: %v", err)
	}

	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		prog, ms := analyzeLines(t, strings.ToUpper(filepath.Base(src)), lines...)

		var b strings.Builder

		b.WriteString(Dump(prog))

		for _, m := range ms {
			b.WriteString("\n" + m.String() + "\n")
		}

		golden := strings.TrimSuffix(src, ".for") + ".sema"

		if *update {
			if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
				t.Fatal(err)
			}

			continue
		}

		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run with -update to write it)", err)
		}

		if b.String() != string(want) {
			t.Errorf("%s:\n%s\nwant:\n%s", src, b.String(), want)
		}
	}
}

// TestErrors checks each of semantic analysis's messages: each case is a
// program (the statements of a main program, in fixed form, unless the
// case gives its own header) and the messages it gets, each as
// IDENT@line:column.
func TestErrors(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		// Program units.
		{"two units, one name", []string{"SUBROUTINE S", "END", "SUBROUTINE S", "END"}, "DUPUNIT@3:7"},
		{"two main programs", []string{"PROGRAM A", "END", "PROGRAM B", "END"}, "TWOMAINS@3:7"},
		{"a dummy twice", []string{"SUBROUTINE S(A, B, A)", "END"}, "DUPDUMMY@1:26"},
		{"the routine's own name as a dummy", []string{"SUBROUTINE S(S)", "END"}, "BADUSE@1:20"},

		// Declarations.
		{"typed twice", []string{"INTEGER I", "REAL I", "END"}, "DUPDECL@2:12"},
		{"typed twice in one statement", []string{"INTEGER I, J, I", "END"}, "DUPDECL@1:21"},
		{"a typed function's type declared", []string{"REAL FUNCTION F(X)", "INTEGER F", "END"}, "DUPDECL@2:15"},
		{"dimensioned twice", []string{"DIMENSION A(5)", "INTEGER A(5)", "END"}, "DUPDIM@2:15"},
		{"declaration after an executable", []string{"I = 1", "INTEGER J", "END"}, "LATEDECL@2:7"},
		{"a length INTEGER doesn't have", []string{"INTEGER*3 I, J", "END"}, "BADLENGTH@1:7"},
		{"a length after the name", []string{"REAL X*5", "END"}, "BADLENGTH@1:12"},
		{"every type and length", []string{"BYTE B", "LOGICAL*1 L1", "LOGICAL*2 L2", "LOGICAL*4 L4",
			"INTEGER*2 I2", "INTEGER*4 I4", "REAL*4 R4", "REAL*8 R8", "REAL*16 R16", "DOUBLE PRECISION D",
			"COMPLEX*8 X8", "COMPLEX*16 X16", "DOUBLE COMPLEX DX", "CHARACTER C", "CHARACTER*65535 C2", "END"}, ""},
		{"a length not compiled yet, used", []string{"INTEGER*2 I", "I = 1", "X = I", "END"}, "NOTIMPL@2:7"},
		{"LOGICAL*8", []string{"LOGICAL*8 L", "END"}, "BADLENGTH@1:7"},
		{"COMPLEX*4", []string{"COMPLEX C*4", "END"}, "BADLENGTH@1:15"},
		{"BYTE with a length", []string{"BYTE*1 B", "END"}, "NOLENGTH@1:7"},
		{"DOUBLE PRECISION with a length", []string{"DOUBLE PRECISION D*8", "END"}, "NOLENGTH@1:24"},
		{"CHARACTER lengths", []string{"CHARACTER*(2*4) C", "CHARACTER C2*(10/2), C3(2)*3", "END"}, ""},
		{"a CHARACTER length too long", []string{"CHARACTER*70000 C", "END"}, "BADCHARLEN@1:7"},
		{"a CHARACTER length of zero", []string{"CHARACTER*(1-1) C", "END"}, "BADCHARLEN@1:19"},
		{"a CHARACTER length naming a variable", []string{"CHARACTER*(N) C", "END"}, "BADCHARLEN@1:18"},
		{"a passed length on a local", []string{"CHARACTER*(*) C", "END"}, "PASSEDLEN@1:21"},
		{"a passed length on a dummy", []string{"SUBROUTINE S(C)", "CHARACTER*(*) C", "END"}, ""},
		{"a passed-length function", []string{"CHARACTER*(*) FUNCTION F(X)", "END"}, "NOTIMPL@1:7"},
		{"a passed-length external function", []string{"CHARACTER*(*) F", "CALL S(F(1))", "END"}, "NOTIMPL@2:14"},
		{"a bound naming a variable", []string{"DIMENSION A(N)", "END"}, "ADJNOTDUMMY@1:19"},
		{"a variable bound on a local array", []string{"SUBROUTINE S(N)", "DIMENSION A(N)", "END"}, "ADJNOTDUMMY@2:19"},
		{"a REAL bound is converted", []string{"DIMENSION A(2.5)", "A(2) = 1", "END"}, ""},
		{"a LOGICAL bound", []string{"DIMENSION A(.TRUE.)", "END"}, "BADBOUND@1:19"},
		{"a CHARACTER bound", []string{"DIMENSION A('X')", "END"}, "BADBOUND@1:19"},
		{"an adjustable array", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N)", "END"}, "NOTIMPL@2:19"},
		{"an adjustable lower bound", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N:5)", "END"}, "NOTIMPL@2:19"},
		{"both bounds, and an expression", []string{"SUBROUTINE S(X, I, J)", "DIMENSION X(-I/2:I/2, J)", "END"}, "NOTIMPL@2:19"},
		{"adjustable and assumed-size", []string{"SUBROUTINE S(A, N)", "DIMENSION A(1:N, 1:*)", "END"}, "NOTIMPL@2:21"},
		{"adjustable arrays in a type statement", []string{"SUBROUTINE S(A, N)", "REAL A(N)", "END"}, "NOTIMPL@2:14"},
		{"a REAL dummy in a bound", []string{"SUBROUTINE S(A, X)", "DIMENSION A(X)", "END"}, "NOTIMPL@2:19"},
		{"a bound's name declared after, with its type", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N)", "INTEGER N", "END"}, "NOTIMPL@2:19"},
		{"a bound's type changed", []string{"SUBROUTINE S(A, X)", "DIMENSION A(X)", "INTEGER X", "END"}, "BOUNDTYPE@3:15 NOTIMPL@2:19"},
		{"a bound's name not a dummy", []string{"SUBROUTINE S(A)", "DIMENSION A(N)", "END"}, "BOUNDVAR@2:19"},
		{"a bound's name an array", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N), N(3)", "END"}, "BOUNDREF@2:19"},
		{"a bound's name its own array", []string{"SUBROUTINE S(A)", "DIMENSION A(A)", "END"}, "BOUNDREF@2:19"},
		{"a bound's name a CHARACTER dummy", []string{"SUBROUTINE S(A, C)", "CHARACTER C", "DIMENSION A(C)", "END"}, "BADBOUND@3:19"},
		{"a bound's name a constant defined after", []string{"SUBROUTINE S(A)", "DIMENSION A(N)", "PARAMETER (N = 2)", "END"}, "BOUNDVAR@2:19"},
		{"a function in a bound", []string{"SUBROUTINE S(A, N)", "DIMENSION A(F(N))", "END"}, "BOUNDREF@2:19"},
		{"an array element in a bound", []string{"SUBROUTINE S(A, N)", "DIMENSION N(3), A(N(1))", "END"}, "BOUNDREF@2:25"},
		{"a comparison in a bound", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N .GT. 1)", "END"}, "BADBOUND@2:21"},
		{"a real power, not constant, no name", []string{"SUBROUTINE S(A)", "DIMENSION A(2.0 ** 0.5)", "END"}, "BADBOUND@2:23"},
		{"an adjustable array's subscripts counted", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N)", "A(1, 2) = 0", "END"}, "NOTIMPL@2:19 SUBSCRIPTS@3:7"},
		{"a bound's name in COMMON", []string{"SUBROUTINE S(A)", "COMMON N", "DIMENSION A(N)", "END"}, "NOTIMPL@3:19"},
		{"a bound's name in COMMON after it", []string{"SUBROUTINE S(A)", "DIMENSION A(N)", "COMMON N", "END"}, "NOTIMPL@2:19"},
		{"a bound's name EQUIVALENCE puts in COMMON", []string{"SUBROUTINE S(A)", "COMMON K", "DIMENSION A(N)", "EQUIVALENCE (N, K)", "END"}, "NOTIMPL@3:19"},
		{"a bound's array in COMMON", []string{"SUBROUTINE S(A)", "COMMON N(2)", "DIMENSION A(N)", "END"}, "BOUNDREF@3:19"},
		{"an ENTRY's dummy, unchecked yet", []string{"SUBROUTINE S", "DIMENSION A(N), B(*)", "RETURN", "ENTRY E(A, B, N)", "END"}, "NOTIMPL@4:7"},
		{"seven dimensions", []string{"DIMENSION A(2,2,2,2,2,2,2)", "A(1,1,1,1,1,1,1) = 1", "END"}, ""},
		{"eight dimensions", []string{"DIMENSION A(2,2,2,2,2,2,2,2)", "END"}, "TOOMANYDIMS@1:17"},
		{"too large an array", []string{"DIMENSION A(1000000000)", "END"}, "ARRAYSIZE@1:17"},
		{"too large once its type is known", []string{"DIMENSION A(300000000)", "REAL*8 A", "END"}, "ARRAYSIZE@1:17"},
		{"too large in seven dimensions", []string{"DIMENSION A(5000,5000,5000,5000,5000,5000,5000)", "END"}, "ARRAYSIZE@1:17"},
		{"the largest array", []string{"BYTE A(2147483647)", "END"}, ""},
		{"too large before its assumed size", []string{"SUBROUTINE S(A)", "DIMENSION A(1000000000, *)", "END"}, "ARRAYSIZE@2:17"},
		{"bounds out of order", []string{"DIMENSION A(5:1)", "END"}, "BOUNDORDER@1:21"},
		{"assumed size, not a dummy", []string{"DIMENSION A(*)", "END"}, "ASSUMEDSIZE@1:17"},
		{"assumed size, not the last", []string{"SUBROUTINE S(A)", "DIMENSION A(*, 2)", "END"}, "ASSUMEDSIZE@2:17"},
		{"a constant bound folded", []string{"DIMENSION A(2*3-1:10/2)", "A(5) = 1", "END"}, ""},
		{"the program's name typed", []string{"PROGRAM P", "INTEGER P", "END"}, "BADUSE@2:15"},
		{"the result dimensioned", []string{"FUNCTION F(X)", "DIMENSION F(3)", "END"}, "BADUSE@2:17"},

		// IMPLICIT.
		{"IMPLICIT types names", []string{"IMPLICIT INTEGER (A-C), LOGICAL (L)", "A = 1.5", "IF (L) STOP", "END"}, ""},
		{"IMPLICIT types a dummy", []string{"SUBROUTINE S(D)", "IMPLICIT LOGICAL (D)", "IF (D) RETURN", "END"}, ""},
		{"IMPLICIT and a type statement", []string{"IMPLICIT LOGICAL (X)", "REAL X", "X = 1.5", "END"}, ""},
		{"IMPLICIT of a type not compiled", []string{"IMPLICIT DOUBLE PRECISION (D)", "D = 1", "END"}, "NOTIMPL@2:7"},
		{"IMPLICIT CHARACTER", []string{"IMPLICIT CHARACTER*8 (T-V)", "PRINT *, T", "END"}, "NOTIMPL@2:16"},
		{"IMPLICIT with a bad length", []string{"IMPLICIT INTEGER*3 (A)", "END"}, "BADLENGTH@1:16"},
		{"a letter typed twice", []string{"IMPLICIT INTEGER (A-C), REAL (B)", "END"}, "DUPIMPLICIT@1:37"},
		{"a letter typed twice by two statements", []string{"IMPLICIT INTEGER (A)", "IMPLICIT REAL (A)", "END"}, "DUPIMPLICIT@2:22"},
		{"IMPLICIT NONE and IMPLICIT", []string{"IMPLICIT NONE", "IMPLICIT REAL (A)", "END"}, "IMPLNONE@2:7"},
		{"IMPLICIT and IMPLICIT NONE", []string{"IMPLICIT REAL (A)", "IMPLICIT NONE", "END"}, "IMPLNONE@2:7"},
		{"IMPLICIT NONE twice", []string{"IMPLICIT NONE", "IMPLICIT NONE", "END"}, "IMPLNONE@2:7"},
		{"IMPLICIT NONE, all declared", []string{"IMPLICIT NONE", "INTEGER I", "REAL X", "I = 1", "X = I", "CALL S(X)", "END"}, ""},
		{"an undeclared name", []string{"IMPLICIT NONE", "INTEGER I", "I = J + 1", "I = J", "END"}, "NOTYPE@3:11"},
		{"an undeclared DO variable", []string{"IMPLICIT NONE", "DO 10 K = 1, 2", "10 CONTINUE", "END"}, "NOTYPE@2:13"},
		{"an undeclared function", []string{"IMPLICIT NONE", "REAL X", "X = F(1.0)", "END"}, "NOTYPE@3:11"},
		{"undeclared dummies and result", []string{"FUNCTION F(X)", "IMPLICIT NONE", "END"}, "NOTYPE@1:7 NOTYPE@1:18"},
		{"an undeclared statement function", []string{"IMPLICIT NONE", "F(X) = 1", "END"}, "NOTIMPL@2:7 NOTYPE@2:7"},
		{"an untyped function's length, IMPLICIT NONE", []string{"FUNCTION F*4(X)", "IMPLICIT NONE", "REAL X", "END"}, "NOTYPE@1:7"},

		// Statement order (Figure 1-1).
		{"IMPLICIT after a type statement", []string{"INTEGER I", "IMPLICIT REAL (A)", "END"}, "ORDER@2:7"},
		{"IMPLICIT NONE after PARAMETER", []string{"PARAMETER (N = 1)", "IMPLICIT NONE", "END"}, "ORDER@2:7 NOTYPE@1:18"},
		{"PARAMETER between IMPLICITs", []string{"IMPLICIT REAL (A)", "PARAMETER (N = 1)", "IMPLICIT REAL (B)", "END"}, ""},
		{"IMPLICIT after an executable", []string{"X = 1", "IMPLICIT REAL (A)", "END"}, "LATEDECL@2:7"},
		{"DATA before IMPLICIT", []string{"DATA X /1.0/", "IMPLICIT INTEGER (I)", "END"}, "ORDER@2:7 NOTIMPL@1:7"},
		{"DATA among the specifications", []string{"REAL X", "DATA X /1.0/", "INTEGER I", "END"}, "NOTIMPL@2:7"},
		{"DATA among the executables", []string{"X = 1", "DATA Y /2.0/", "END"}, "NOTIMPL@2:7"},
		{"a declaration after a statement function", []string{"F(X) = X", "REAL Y", "END"}, "ORDER@2:7 NOTIMPL@1:7"},
		{"FORMAT and NAMELIST anywhere", []string{"10 FORMAT (I5)", "IMPLICIT REAL (A)", "NAMELIST /G/ A", "X = 1", "END"}, "NOTIMPL@1:7 NOTIMPL@3:7"},
		{"COMMON after an executable", []string{"X = 1", "COMMON /C/ Y", "END"}, "LATEDECL@2:7"},

		// PARAMETER and constant expressions (section 4.11, appendix A.4).
		{"constants in declarations", []string{"PARAMETER (N = 10, L = N * 2)", "DIMENSION A(N), B(-N:L)", "CHARACTER*(L) C", "END"}, ""},
		{"constants in expressions", []string{"PARAMETER (N = 10, PI = 3.1415927)", "X = PI * N", "PRINT *, N, 2 ** N, MAX(N, 3)", "END"}, ""},
		{"a CHARACTER constant printed", []string{"CHARACTER*(*) T", "PARAMETER (T = 'HI')", "PRINT *, T, T // '!'", "END"}, ""},
		{"a REAL*8 constant used", []string{"DOUBLE PRECISION D", "PARAMETER (D = 1D0)", "X = D", "Y = D", "END"}, "NOTIMPL@3:11"},
		{"a COMPLEX constant expression used", []string{"X = CMPLX(1.0, 2.0)", "END"}, "NOTIMPL@1:11"},
		{"a constant defined twice", []string{"PARAMETER (N = 1)", "PARAMETER (N = 2)", "END"}, "DUPPARAM@2:18"},
		{"a constant assigned", []string{"PARAMETER (N = 1)", "N = 2", "END"}, "BADUSE@2:7"},
		{"a constant as a DO variable", []string{"PARAMETER (N = 1)", "DO 10 N = 1, 2", "10 CONTINUE", "END"}, "BADUSE@2:13"},
		{"a constant called", []string{"PARAMETER (N = 1)", "CALL N", "END"}, "BADUSE@2:12"},
		{"a constant with subscripts", []string{"PARAMETER (N = 1)", "I = N(1)", "END"}, "BADUSE@2:11"},
		{"a constant dimensioned", []string{"PARAMETER (N = 1)", "DIMENSION N(3)", "END"}, "BADUSE@2:17"},
		{"an array as a constant", []string{"DIMENSION N(3)", "PARAMETER (N = 1)", "END"}, "BADUSE@2:18"},
		{"a dummy as a constant", []string{"SUBROUTINE S(N)", "PARAMETER (N = 1)", "END"}, "BADUSE@2:18"},
		{"a constant's type after it", []string{"PARAMETER (N = 1)", "REAL N", "END"}, "TYPEAFTER@2:12"},
		{"the old form's constant typed", []string{"REAL P", "PARAMETER P = 1", "END"}, "OLDPARAMTYPE@2:17"},
		{"the old form's constant typed after", []string{"PARAMETER P = 1", "REAL P", "END"}, "OLDPARAMTYPE@2:12"},
		{"the old form's type is its value's", []string{"IMPLICIT NONE", "PARAMETER P = 1, Q = 2.5", "INTEGER I", "I = P", "END"}, ""},
		{"a variable in a value", []string{"PARAMETER (N = M + 1)", "END"}, "NOTCONST@1:22"},
		{"a function in a value", []string{"PARAMETER (X = SQRT(2.0))", "END"}, "NOTCONST@1:22"},
		{"a REAL exponent in a value", []string{"PARAMETER (X = 2.0 ** 0.5)", "END"}, "NOTCONST@1:29"},
		{"a constant's value too large", []string{"PARAMETER (N = 2 ** 31)", "END"}, "INTRANGE@1:24"},
		{"zero to the power zero", []string{"PARAMETER (N = 0 ** 0)", "END"}, "UNDEFPOW@1:24"},
		{"a division by zero", []string{"PARAMETER (X = 1.0 / 0)", "END"}, "DIVZERO@1:26"},
		{"a REAL*8 value out of range", []string{"PARAMETER (D = 1D39)", "END"}, "REALRANGE@1:22"},
		{"a CHARACTER value for an INTEGER", []string{"PARAMETER (N = 'AB')", "END"}, "NOTIMPL@1:22"},
		{"an INTEGER value for a CHARACTER", []string{"CHARACTER*2 C", "PARAMETER (C = 1)", "END"}, "PARAMTYPE@2:22"},
		{"a REAL value for a LOGICAL", []string{"LOGICAL L", "PARAMETER (L = 1.0)", "END"}, "PARAMTYPE@2:22"},
		{"a variable in a COMPLEX constant", []string{"PARAMETER (C = (X, 1.0))", "END"}, "BADCOMPLEX@1:23"},
		{"an expression in a COMPLEX constant", []string{"PARAMETER (C = (1.0 + 1, 1.0))", "END"}, "BADCOMPLEX@1:27"},
		{"constants in a COMPLEX constant", []string{"COMPLEX C", "PARAMETER (R = 1.0, C = (R, -R))", "END"}, ""},
		{"a variable in a COMPLEX constant, executed", []string{"Y = 1", "C = (Y, 1.0)", "END"}, "BADCOMPLEX@2:12"},
		{"a COMPLEX comparison", []string{"PARAMETER (L = (1.0, 2.0) .LT. (1.0, 2.0))", "END"}, "BADOPERAND@1:33"},
		{"an intrinsic's arguments counted", []string{"PARAMETER (N = MOD(1))", "END"}, "INTRINARGS@1:22"},
		{"an intrinsic's arguments of two types", []string{"PARAMETER (N = MAX(1, 2.0))", "END"}, "BADARG@1:29"},
		{"ICHAR of a long string", []string{"PARAMETER (N = ICHAR('AB'))", "END"}, "BADARG@1:28"},
		{"CHAR out of range", []string{"CHARACTER C", "PARAMETER (C = CHAR(256))", "END"}, "BADARG@2:27"},
		{"a reserved operand's bits", []string{"PARAMETER (X = '8000'X)", "END"}, "BADTYPELESS@1:22"},
		{"a typeless constant executed", []string{"I = '1F'X", "END"}, "NOTIMPL@1:11"},
		{"a constant under IMPLICIT NONE", []string{"IMPLICIT NONE", "PARAMETER (N = 1)", "END"}, "NOTYPE@2:18"},
		{"a constant as an adjustable bound", []string{"SUBROUTINE S(A)", "PARAMETER (N = 3)", "DIMENSION A(N)", "END"}, ""},

		// Names.
		{"a variable called", []string{"X = 1", "CALL X", "END"}, "BADUSE@2:12"},
		{"an array called", []string{"DIMENSION A(3)", "CALL A(1)", "END"}, "BADUSE@2:12"},
		{"a subroutine as a variable", []string{"CALL S", "S = 1", "END"}, "BADUSE@2:7"},
		{"a subroutine as a function", []string{"CALL S", "X = S(1)", "END"}, "BADUSE@2:11"},
		{"a function called", []string{"X = F(1)", "CALL F(2)", "END"}, "BADUSE@2:12"},
		{"a function as a variable", []string{"X = F(1)", "Y = F", "END"}, "BADUSE@2:11"},
		{"the program's name as a variable", []string{"PROGRAM P", "P = 1", "END"}, "BADUSE@2:7"},
		{"a subroutine calling itself", []string{"SUBROUTINE S", "CALL S", "END"}, "RECURSIVE@2:12"},
		{"a function referring to itself", []string{"FUNCTION F(X)", "F = F(X - 1)", "END"}, "RECURSIVE@2:11"},
		{"a function calling itself", []string{"FUNCTION F(X)", "CALL F", "END"}, "RECURSIVE@2:12"},
		{"a variable with subscripts", []string{"X = 1", "Y = X(2)", "END"}, "NOTARRAY@2:11"},
		{"assignment to a non-array element", []string{"X = 1", "B(2) = 1", "END"}, "NOTARRAY@2:7"},
		{"a statement function", []string{"F(X) = X + 1", "Y = F(2.0)", "END"}, "NOTIMPL@1:7"},
		{"a statement function after declarations", []string{"DIMENSION A(3)", "F(X) = X + 1", "A(1) = F(2.0)", "END"}, "NOTIMPL@2:7"},
		{"a statement function with a constant argument", []string{"AVG(A, B, 3.) = (A + B) / 3.", "END"}, "NOTARRAY@1:7"},
		{"a statement function after an executable", []string{"Y = 1", "F(X) = X", "END"}, "NOTARRAY@2:7"},
		{"a statement function defined twice", []string{"F(X) = X", "F(Y) = Y", "END"}, "DUPSTMTFUNC@2:7 NOTIMPL@1:7"},
		{"a statement function's dummy twice", []string{"F(X, X) = X", "END"}, "DUPDUMMY@1:12 NOTIMPL@1:7"},
		{"a statement function called", []string{"F(X) = X", "CALL F(1.0)", "END"}, "NOTIMPL@1:7 BADUSE@2:12"},
		{"a statement function as a variable", []string{"F(X) = X", "Y = F", "END"}, "NOTIMPL@1:7 BADUSE@2:11"},
		{"a statement function assigned", []string{"F(X) = X", "Y = 1", "F(2) = 3", "END"}, "NOTIMPL@1:7 BADUSE@3:7"},
		{"an intrinsic's name as a statement function", []string{"SINH(X) = X", "Y = SINH(1.0)", "END"}, "NOTIMPL@1:7"},
		{"the result as a statement function", []string{"FUNCTION F(X)", "F(Y) = Y", "END"}, "BADUSE@2:7"},
		{"a dummy's element assigned first", []string{"SUBROUTINE S(A)", "A(1) = 2", "END"}, "NOTARRAY@2:7"},
		{"a whole array in an expression", []string{"DIMENSION A(3)", "X = A + 1", "END"}, "NEEDSUBSCRIPT@2:11"},
		{"a whole array assigned", []string{"DIMENSION A(3)", "A = 1", "END"}, "BADTARGET@2:7"},
		{"too many subscripts", []string{"DIMENSION A(3)", "X = A(1, 2)", "END"}, "SUBSCRIPTS@2:11"},
		{"too few subscripts", []string{"DIMENSION A(3, 3)", "X = A(1)", "END"}, "SUBSCRIPTS@2:11"},
		{"an intrinsic function", []string{"X = SQRT(2.0)", "END"}, "NOTIMPL@1:11"},
		{"an intrinsic as an array", []string{"DIMENSION SQRT(3)", "X = SQRT(2)", "END"}, ""},
		{"a system subroutine", []string{"CALL EXIT", "END"}, "NOTIMPL@1:12"},
		{"a system function", []string{"X = RAN(I)", "END"}, "NOTIMPL@1:11"},
		{"a dummy procedure called", []string{"SUBROUTINE S(P)", "CALL P", "END"}, "NOTIMPL@2:12"},
		{"a dummy function", []string{"SUBROUTINE S(P)", "X = P(1)", "END"}, "NOTIMPL@2:11"},

		// Expressions.
		{"**", []string{"X = Y ** 3", "END"}, "NOTIMPL@1:13"},
		{"//", []string{"PRINT *, 'A' // C", "END"}, "NOTIMPL@1:20"},
		{".EQV.", []string{"L = L .EQV. L", "END"}, "NOTIMPL@1:13"},
		{".AND. of REALs", []string{"L = X .AND. Y", "END"}, "BADOPERAND@1:13"},
		{".NOT. of a REAL", []string{"L = .NOT. X", "END"}, "BADOPERAND@1:11"},
		{"a character constant assigned", []string{"X = 'AB'", "END"}, "NOTIMPL@1:11"},
		{"a character constant added", []string{"X = 1 + 'AB'", "END"}, "NOTIMPL@1:15"},
		{"a character constant passed", []string{"CALL S('AB')", "END"}, "NOTIMPL@1:14"},
		{"a REAL condition", []string{"IF (X) STOP", "END"}, "NOTLOGICAL@1:11"},
		{"an INTEGER too big", []string{"I = 2147483648", "END"}, "INTRANGE@1:11"},
		{"INTEGER's smallest", []string{"I = -2147483648", "END"}, ""},
		{"an overflow folded", []string{"I = 2147483647 + 1", "END"}, "INTRANGE@1:22"},
		{"a REAL too big", []string{"X = 1E39", "END"}, "REALRANGE@1:11"},
		{"a REAL too small", []string{"X = 1E-39", "END"}, "REALRANGE@1:11"},
		{"a REAL*8 constant", []string{"X = 1D0", "END"}, "NOTIMPL@1:11"},
		{"a REAL product too big", []string{"X = 1E38 * 10", "END"}, "REALRANGE@1:16"},
		{"a REAL quotient too small", []string{"X = 1E-38 / 1E10", "END"}, ""},
		{"divide by zero", []string{"I = 1 / 0", "END"}, "DIVZERO@1:13"},
		{"divide by zero, REAL", []string{"X = 1.0 / 0", "END"}, "DIVZERO@1:15"},
		{"a REAL too big for INTEGER", []string{"I = 3E9", "END"}, "INTRANGE@1:11"},

		// Statements.
		{"RETURN in a main program", []string{"RETURN", "END"}, "MAINRETURN@1:7"},
		{"an alternate return", []string{"SUBROUTINE S", "RETURN 1", "END"}, "NOTIMPL@2:14"},
		{"formatted PRINT", []string{"PRINT 10, X", "10 FORMAT (F8.2)", "END"}, "NOTIMPL@1:13 NOTIMPL@2:7"},
		{"a unit number", []string{"WRITE (6,*) X", "END"}, "NOTIMPL@1:14"},
		{"unformatted WRITE", []string{"WRITE (*) X", "END"}, "NOTIMPL@1:7"},
		{"a format expression", []string{"WRITE (*,F) X", "END"}, "NOTIMPL@1:16"},
		{"an assumed-size array written", []string{"SUBROUTINE S(A)", "DIMENSION A(*)", "PRINT *, A", "END"}, "NOSIZE@3:16"},

		// IF blocks.
		{"ELSE with no IF", []string{"ELSE", "END"}, "NOIF@1:7"},
		{"END IF with no IF", []string{"END IF", "END"}, "NOIF@1:7"},
		{"an IF with no END IF", []string{"IF (L) THEN", "X = 1", "END"}, "NOENDIF@1:7"},
		{"ELSE IF after ELSE", []string{"IF (L) THEN", "ELSE", "ELSE IF (L) THEN", "END IF", "END"}, "ELSEORDER@3:7"},
		{"two ELSEs", []string{"IF (L) THEN", "ELSE", "ELSE", "END IF", "END"}, "ELSEORDER@3:7"},
		{"a loop across ELSE", []string{"IF (L) THEN", "DO 10 I = 1, 2", "ELSE", "10 CONTINUE", "END IF", "END"}, "DONEST@3:7"},
		{"a block IF with an error", []string{"IF (X .GT.) THEN", "X = 1", "END IF", "END"}, "SYNTAX@1:17"},

		// DO loops.
		{"a loop whose label never comes", []string{"DO 10 I = 1, 2", "END"}, "NODO@1:7"},
		{"a loop whose label is behind it", []string{"10 CONTINUE", "DO 10 I = 1, 2", "END"}, "NODO@2:7"},
		{"a loop ending at GO TO", []string{"DO 10 I = 1, 2", "10 GO TO 20", "20 END"}, "DOTERM@2:7"},
		{"a loop ending at RETURN", []string{"SUBROUTINE S", "DO 10 I = 1, 2", "10 RETURN", "END"}, "DOTERM@3:7"},
		{"a loop ending at END IF", []string{"IF (L) THEN", "DO 10 I = 1, 2", "10 END IF", "END"}, "DOTERM@3:7"},
		{"a loop ending at END", []string{"DO 10 I = 1, 2", "10 END"}, "DOTERM@2:7"},
		{"a loop ending at a declaration", []string{"DO 10 I = 1, 2", "10 INTEGER J", "END"}, "LATEDECL@2:7 DOTERM@2:7"},
		{"a loop ending at a logical IF", []string{"DO 10 I = 1, 2", "10 IF (I .EQ. 1) X = 1", "END"}, ""},
		{"loops crossing", []string{"DO 10 I = 1, 2", "DO 20 J = 1, 2", "10 CONTINUE", "20 CONTINUE", "END"}, "DONEST@3:7"},
		{"a loop ending in an IF block", []string{"DO 10 I = 1, 2", "IF (L) THEN", "10 CONTINUE", "END IF", "END"}, "DONEST@3:7"},
		{"a LOGICAL DO variable", []string{"LOGICAL L", "DO 10 L = 1, 2", "10 CONTINUE", "END"}, "DOVAR@2:13"},
		{"an array as DO variable", []string{"DIMENSION A(3)", "DO 10 A = 1, 2", "10 CONTINUE", "END"}, "DOVAR@2:13"},
		{"a DO variable assigned", []string{"DO 10 I = 1, 2", "I = 5", "10 CONTINUE", "END"}, "DOVARSET@2:7"},
		{"a DO variable reused", []string{"DO 10 I = 1, 2", "DO 10 I = 1, 2", "10 CONTINUE", "END"}, "DOVARSET@2:13"},
		{"a zero step", []string{"DO 10 I = 1, 2, 0", "10 CONTINUE", "END"}, "DOSTEP0@1:23"},
		{"a zero REAL step", []string{"DO 10 X = 1, 2, 0.0", "10 CONTINUE", "END"}, "DOSTEP0@1:23"},
		{"a step folded to zero", []string{"DO 10 I = 1, 2, 1-1", "10 CONTINUE", "END"}, "DOSTEP0@1:24"},

		// Labels.
		{"a label twice", []string{"10 CONTINUE", "10 CONTINUE", "END"}, "DUPLABEL@2:1"},
		{"a jump to nowhere", []string{"GO TO 20", "END"}, "UNDEFLBL@1:13"},
		{"a jump to a declaration", []string{"10 INTEGER I", "GO TO 10", "END"}, "NOTEXEC@2:13"},
		{"a jump to a FORMAT", []string{"10 FORMAT (I5)", "GO TO 10", "END"}, "NOTIMPL@1:7 NOTEXEC@2:13"},
		{"a jump to ELSE", []string{"IF (L) THEN", "GO TO 10", "10 ELSE", "END IF", "END"}, "NOTEXEC@2:13"},
		{"a jump to a statement with an error", []string{"10 READ *, X", "GO TO 10", "END"}, "NOTIMPL@1:7"},
		{"a jump into an IF block", []string{"GO TO 10", "IF (L) THEN", "10 CONTINUE", "END IF", "END"}, "INTOBLOCK@1:13"},
		{"a jump between blocks", []string{"IF (L) THEN", "10 X = 1", "ELSE", "GO TO 10", "END IF", "END"}, "INTOBLOCK@4:13"},
		{"a jump to END IF from an earlier block", []string{"IF (L) THEN", "GO TO 10", "ELSE", "X = 1", "10 END IF", "END"}, "INTOBLOCK@2:13"},
		{"a jump to END IF from the last block", []string{"IF (L) THEN", "X = 2", "ELSE", "GO TO 10", "10 END IF", "END"}, ""},
		{"a jump out of a block", []string{"IF (L) THEN", "GO TO 10", "END IF", "10 CONTINUE", "END"}, ""},
		{"a jump into a loop", []string{"GO TO 10", "DO 20 I = 1, 2", "10 X = 1", "20 CONTINUE", "END"}, ""},
		{"a jump to a shared terminal from outside", []string{"DO 20 I = 1, 2", "GO TO 20", "DO 20 J = 1, 2", "20 CONTINUE", "END"}, "SHAREDTERM@2:13"},
		{"a jump to a shared terminal from inside", []string{"DO 20 I = 1, 2", "DO 20 J = 1, 2", "GO TO 20", "20 CONTINUE", "END"}, ""},

		// COMMON (section 4.2).
		{"COMMON's forms", []string{"COMMON HEAT,X /BLK1/KILO,Q / /Y", "COMMON /BLK1/R(10)", "END"}, ""},
		{"a block named as a variable", []string{"COMMON /X/ Y", "X = Y", "END"}, ""},
		{"an array's dimensions after COMMON", []string{"COMMON A", "DIMENSION A(4)", "A(1) = 1", "END"}, ""},
		{"an array's dimensions in COMMON and DIMENSION", []string{"COMMON A(3)", "DIMENSION A(4)", "END"}, "DUPDIM@2:17"},
		{"a dummy in COMMON", []string{"SUBROUTINE S(A)", "COMMON A", "A = 1", "END"}, "COMMONDUMMY@2:14"},
		{"a function's result in COMMON", []string{"FUNCTION F(X)", "COMMON F", "F = X", "END"}, "BADUSE@2:14"},
		{"a subroutine's name in COMMON", []string{"SUBROUTINE S", "COMMON S", "END"}, "BADUSE@2:14"},
		{"a constant in COMMON", []string{"PARAMETER (N = 1)", "COMMON N", "END"}, "BADUSE@2:14"},
		{"a name in two blocks", []string{"COMMON /A/ X /B/ X", "X = 1", "END"}, "DUPCOMMON@1:24"},
		{"a name twice in one block", []string{"COMMON /A/ X", "COMMON /A/ X", "END"}, "DUPCOMMON@2:18"},
		{"a name in COMMON isn't a function", []string{"COMMON F", "X = F(1.0)", "END"}, "NOTARRAY@2:11"},
		{"a name in COMMON isn't a statement function", []string{"COMMON F", "F(X) = X", "END"}, "BADUSE@2:7"},
		{"a statement function in COMMON", []string{"F(X) = X", "COMMON F", "END"}, "ORDER@2:7 BADUSE@2:14 NOTIMPL@1:7"},
		{"an adjustable array in COMMON", []string{"SUBROUTINE S(N)", "COMMON A(N)", "END"}, "ADJNOTDUMMY@2:16"},
		{"an assumed-size array in COMMON", []string{"COMMON A(*)", "END"}, "ASSUMEDSIZE@1:14"},
		{"a passed length in COMMON", []string{"CHARACTER*(*) C", "COMMON C", "END"}, "PASSEDLEN@1:21"},
		{"a block as large as can be", []string{"COMMON /A/ B(2147483647)", "LOGICAL*1 B", "END"}, ""},
		{"a block too large", []string{"COMMON /A/ B(2147483647), C", "LOGICAL*1 B, C", "END"}, "COMMONSIZE@1:15"},
		{"a block named for a subroutine in the file", []string{"COMMON /S/ X", "END", "SUBROUTINE S", "END"}, "COMMONNAME@1:15"},
		{"a block named for its own function", []string{"FUNCTION F(X)", "COMMON /F/ Y", "F = X", "END"}, "COMMONNAME@2:15"},
		{"a block named for the main program", []string{"PROGRAM P", "COMMON /P/ X", "END"}, "COMMONNAME@2:15"},
		{"a block named for a routine it calls", []string{"COMMON /S/ X", "CALL S", "END"}, "COMMONNAME@1:15"},
		{"a block named for a function it references", []string{"COMMON /F/ X", "X = F(1.0)", "END"}, "COMMONNAME@1:15"},

		// EQUIVALENCE (section 4.6).
		{"the manual's forms", []string{"DOUBLE PRECISION DVAR", "INTEGER*2 IARR(4)", "EQUIVALENCE (DVAR, IARR(1))", "CHARACTER KEY*16, STAR*10", "EQUIVALENCE (KEY, STAR)", "END"}, ""},
		{"one item", []string{"EQUIVALENCE (A)", "END"}, "EQUIVONE@1:20"},
		{"a dummy", []string{"SUBROUTINE S(A)", "EQUIVALENCE (A, B)", "A = 1", "END"}, "EQUIVDUMMY@2:20"},
		{"a function's result", []string{"FUNCTION F(X)", "EQUIVALENCE (F, Y)", "END"}, "BADUSE@2:20"},
		{"a constant", []string{"PARAMETER (N = 1)", "EQUIVALENCE (N, Y)", "END"}, "BADUSE@2:20"},
		{"a record's field", []string{"EQUIVALENCE (R.F, Y)", "END"}, "EQUIVFORM@1:22"},
		{"a name in a list isn't a function", []string{"EQUIVALENCE (F, Y)", "X = F(1.0)", "END"}, "NOTARRAY@2:11"},
		{"subscripts on a scalar", []string{"EQUIVALENCE (A(1), B)", "END"}, "NOTARRAY@1:20"},
		{"a subscript that isn't constant", []string{"DIMENSION A(4)", "EQUIVALENCE (A(I), B)", "END"}, "EQUIVCONST@2:22"},
		{"a real subscript", []string{"DIMENSION A(4)", "EQUIVALENCE (A(1.5), B)", "END"}, "EQUIVCONST@2:22"},
		{"a constant's subscript", []string{"PARAMETER (K = 2)", "DIMENSION A(4)", "EQUIVALENCE (A(K+1), B)", "END"}, ""},
		{"a subscript too large", []string{"DIMENSION A(4)", "EQUIVALENCE (A(5), B)", "END"}, "EQUIVRANGE@2:20"},
		{"a subscript too small", []string{"DIMENSION A(2:4)", "EQUIVALENCE (A(1), B)", "END"}, "EQUIVRANGE@2:20"},
		{"a linear element number", []string{"DIMENSION TABLE(2,2), TRIPLE(2,2,2)", "EQUIVALENCE (TABLE(4), TRIPLE(7))", "END"}, ""},
		{"a linear element number too large", []string{"DIMENSION TABLE(2,2)", "EQUIVALENCE (TABLE(5), B)", "END"}, "EQUIVRANGE@2:20"},
		{"a linear element number from 1", []string{"DIMENSION TABLE(0:1,0:1)", "EQUIVALENCE (TABLE(0), B)", "END"}, "EQUIVRANGE@2:20"},
		{"too many subscripts", []string{"DIMENSION TABLE(2,2)", "EQUIVALENCE (TABLE(1,1,1), B)", "END"}, "SUBSCRIPTS@2:20"},
		{"a substring of a number", []string{"EQUIVALENCE (A(1:2), B)", "END"}, "EQUIVSUBSTR@1:20"},
		{"a substring of an array", []string{"CHARACTER A(3)*4", "EQUIVALENCE (A(1:2), B)", "END"}, "NEEDSUBSCRIPT@2:20"},
		{"a substring too long", []string{"CHARACTER A*4", "EQUIVALENCE (A(2:5), B)", "END"}, "EQUIVSUBRANGE@2:20"},
		{"a substring backward", []string{"CHARACTER A*4", "EQUIVALENCE (A(3:2), B)", "END"}, "EQUIVSUBRANGE@2:20"},
		{"two elements of an array", []string{"DIMENSION A(4)", "EQUIVALENCE (A(1), A(2))", "END"}, "EQUIVCONFLICT@2:26"},
		{"one element twice", []string{"DIMENSION A(4)", "EQUIVALENCE (A(2), A(2))", "END"}, ""},
		{"the manual's conflict", []string{"DIMENSION A(7), B(9)", "EQUIVALENCE (A(1), B(1)), (A(4), B(7))", "END"}, "EQUIVCONFLICT@2:40"},
		{"the same association twice", []string{"DIMENSION A(7), B(9)", "EQUIVALENCE (A(1), B(1)), (A(4), B(4))", "END"}, ""},
		{"two substrings of one variable", []string{"CHARACTER C*8", "EQUIVALENCE (C(1:2), D), (C(2:3), D)", "END"}, "EQUIVCONFLICT@2:41"},
		{"a conflict through a third name", []string{"EQUIVALENCE (A, B), (B, C), (C, A)", "END"}, ""},
		{"the manual's valid extension", []string{"DIMENSION A(4), B(6)", "COMMON A", "EQUIVALENCE (A(2), B(1))", "END"}, ""},
		{"the manual's invalid extension", []string{"DIMENSION A(4), B(6)", "COMMON A", "EQUIVALENCE (A(2), B(3))", "END"}, "EQUIVBEFORE@3:26"},
		{"two blocks", []string{"COMMON /A/ X /B/ Y", "EQUIVALENCE (X, Y)", "END"}, "EQUIVBLOCKS@2:23"},
		{"blank common and a block", []string{"COMMON X /B/ Y", "EQUIVALENCE (X, Z), (Z, Y)", "END"}, "EQUIVBLOCKS@2:31"},
		{"two names of a block", []string{"COMMON X, Y", "EQUIVALENCE (X, Y)", "END"}, "EQUIVCOMMON@2:23"},
		{"one name of a block, through an array", []string{"COMMON X, Y", "DIMENSION Z(2)", "EQUIVALENCE (X, Z(1)), (Y, Z(2))", "END"}, ""},
		{"a list's other items after an error", []string{"DIMENSION A(4)", "EQUIVALENCE (A(9), B, C), (B, D)", "B = 1", "END"}, "EQUIVRANGE@2:20"},

		// Recovery: a name whose declaration had an error gets no more
		// messages, and a unit with a declaration this phase can't read
		// gets only the messages about its structure.
		{"a bound with an error", []string{"DIMENSION A(N)", "A(1) = 2", "X = A(2) + A", "END"}, "ADJNOTDUMMY@1:19"},
		{"a type not compiled yet", []string{"LOGICAL*1 B", "L = B .AND. .TRUE.", "CALL S(B)", "END"}, "NOTIMPL@2:11"},
		{"a declaration not read yet", []string{"EXTERNAL A", "X = A(1)", "GO TO 99", "END"}, "NOTIMPL@1:7 UNDEFLBL@3:13"},
		{"a declaration with a syntax error", []string{"INTEGER A(", "X = A(1)", "END IF", "END"}, "SYNTAX@1:17 NOIF@3:7"},
		{"a FORMAT declares nothing", []string{"10 FORMAT (I5)", "X = 1", "Y = X(2)", "END"}, "NOTIMPL@1:7 NOTARRAY@3:11"},
		{"an incomplete unit's references", []string{"SAVE X", "I = F(1.0)", "END", "REAL FUNCTION F(X)", "F = X", "END"}, "NOTIMPL@1:7"},

		// What the parser parses and semantic analysis doesn't compile
		// yet (Phase 76): statements, reported where they are, and the
		// forms inside statements, reported where analysis reaches them.
		{"a READ statement", []string{"READ *, X", "END"}, "NOTIMPL@1:7"},
		{"a TYPE statement", []string{"TYPE *, X", "END"}, "NOTIMPL@1:7"},
		{"PAUSE", []string{"PAUSE", "END"}, "NOTIMPL@1:7"},
		{"an arithmetic IF", []string{"IF (X) 10, 10, 10", "10 END"}, "NOTIMPL@1:7"},
		{"a logical IF's READ", []string{"IF (X .GT. 0) READ *, Y", "END"}, "NOTIMPL@1:21"},
		{"a DO loop without a label", []string{"DO I = 1, 10", "END DO", "END"}, "NOTIMPL@1:7 NOTIMPL@2:7"},
		{"the computed GO TO", []string{"GO TO (10, 20), I", "10 CONTINUE", "20 END"}, "NOTIMPL@1:7"},
		{"SAVE", []string{"SAVE /B/, X", "END"}, "NOTIMPL@1:7"},
		{"an OPTIONS statement", []string{"OPTIONS /G_FLOATING", "PROGRAM P", "END"}, "NOTIMPL@1:7"},
		{"BLOCK DATA", []string{"BLOCK DATA B", "COMMON /C/ X", "END"}, "NOTIMPL@1:7"},
		{"a CHARACTER declaration", []string{"CHARACTER*10 S", "S = 'ABC'", "PRINT *, S", "END"}, "NOTIMPL@2:7"},
		{"a DOUBLE PRECISION function", []string{"DOUBLE PRECISION FUNCTION D(X)", "D = X", "END"}, "NOTIMPL@1:7"},
		{"two names of a type not compiled", []string{"COMPLEX A, B", "A = B", "END"}, "NOTIMPL@2:7 NOTIMPL@2:11"},
		{"a length after an untyped function", []string{"FUNCTION F*8(X)", "F = X", "END"}, "NOTIMPL@1:7"},
		{"a length after an untyped INTEGER function", []string{"FUNCTION IF*4(X)", "IF = X", "END"}, ""},
		{"a bad length after an untyped function", []string{"FUNCTION IF*8(X)", "IF = X", "END"}, "BADLENGTH@1:18"},
		{"a function typed by a type statement", []string{"FUNCTION F(X)", "INTEGER*2 F", "F = X", "END"}, "NOTIMPL@1:7"},
		{"initial values", []string{"INTEGER A(10)/10*0/", "A(1) = 2", "END"}, "NOTIMPL@1:20"},
		{"an alternate return's dummy", []string{"SUBROUTINE S(A, *)", "CALL T(*10)", "10 END"}, "NOTIMPL@1:23 NOTIMPL@2:14"},
		{"a Hollerith constant", []string{"X = 5HHELLO", "END"}, "NOTIMPL@1:11"},
		{"an octal constant", []string{"I = '17'O", "END"}, "NOTIMPL@1:11"},
		{"a COMPLEX constant", []string{"X = (1.0, 2.0)", "END"}, "NOTIMPL@1:11"},
		{"a substring assigned", []string{"C(1:3) = 'ABC'", "END"}, "NOTIMPL@1:7"},
		{"a substring compared", []string{"IF (C(I)(1:1) .EQ. 'X') STOP", "END"}, "NOTIMPL@1:11"},
		{"a record's field", []string{"X = R.F + 1", "END"}, "NOTIMPL@1:13"},
		{"%VAL", []string{"CALL S(%VAL(X))", "END"}, "NOTIMPL@1:14"},
		{"an implied-DO list", []string{"PRINT *, (I, I = 1, 3)", "END"}, "NOTIMPL@1:16"},
		{"WRITE with ERR=", []string{"WRITE (*, *, ERR=10) X", "10 END"}, "NOTIMPL@1:20"},
		{"direct access", []string{"WRITE (1'10) X", "END"}, "NOTIMPL@1:16"},
		{"a Hollerith bound", []string{"DIMENSION A(1H1)", "END"}, "NOTIMPL@1:19"},
		{"an octal integer", []string{"I = \"17 + 1", "DIMENSION A(\"10)", "A(8) = I", "END"}, "LATEDECL@2:7"},
		{"an octal integer too big", []string{"I = \"777777777777", "END"}, "INTRANGE@1:11"},
		{"NOTIMPL in an incomplete unit", []string{"SAVE X", "Y = 3HABC", "END"}, "NOTIMPL@1:7 NOTIMPL@2:11"},

		// References between units.
		{"a function's type", []string{"I = IFUN(1.0)", "END", "REAL FUNCTION IFUN(X)", "IFUN = X", "END"}, "FUNCTYPE@1:11"},
		{"a function's type agreeing", []string{"REAL IFUN", "X = IFUN(1.0)", "END", "REAL FUNCTION IFUN(X)", "IFUN = X", "END"}, ""},
		{"a function called", []string{"CALL F(1.0)", "END", "FUNCTION F(X)", "F = X", "END"}, "BADUSE@1:12"},
		{"a subroutine referenced", []string{"X = S(1.0)", "END", "SUBROUTINE S(X)", "END"}, "BADUSE@1:11"},
		{"a unit as a variable", []string{"S = 1", "END", "SUBROUTINE S", "END"}, "BADUSE@1:7"},
		{"the main program called", []string{"PROGRAM P", "END", "SUBROUTINE S", "CALL P", "END"}, "BADUSE@4:12"},
		{"too few arguments", []string{"CALL S(1)", "END", "SUBROUTINE S(A, B)", "END"}, "ARGCOUNT@1:12"},
		{"too many arguments", []string{"X = F(1.0, 2.0)", "END", "FUNCTION F(A)", "F = A", "END"}, "ARGCOUNT@1:11"},
		{"null arguments count", []string{"CALL S(1,)", "END", "SUBROUTINE S(A, B)", "END"}, ""},
	}

	for _, c := range cases {
		lines := make([]string, len(c.lines))
		for i, l := range c.lines {
			lines[i] = fixedLine(l)
		}

		_, ms := analyzeLines(t, "T.FOR", lines...)

		var got []string
		for _, m := range ms {
			got = append(got, fmt.Sprintf("%s@%d:%d", m.Ident, m.Pos.Line, m.Pos.Column))
		}

		if g := strings.Join(got, " "); g != c.want {
			t.Errorf("%s: got %q, want %q", c.name, g, c.want)

			for _, m := range ms {
				t.Log(m.String())
			}
		}
	}
}

// TestElementNumber checks an element's place in storage order (section
// 2.2.3.3), for the constant subscripts EQUIVALENCE and DATA use.
func TestElementNumber(t *testing.T) {
	prog, ms := analyzeLines(t, "T.FOR",
		"      SUBROUTINE S(D, E, N)",
		"      DIMENSION BOS(3,3,3), V(0:9), W(-2:2,5:6), D(2:3,0:*), E(N)",
		"      END")

	// E's NOTIMPL is expected: the rest has no messages.
	if len(ms) != 1 || ms[0].Ident != "NOTIMPL" {
		t.Fatalf("messages: %v", ms)
	}

	u := prog.Units[0]

	cases := []struct {
		name string
		subs []int
		want int // -1 if it isn't an element
	}{
		// Figure 2-1's memory positions, each one more than the number.
		{"BOS", []int{1, 1, 1}, 0},
		{"BOS", []int{3, 1, 1}, 2},
		{"BOS", []int{1, 2, 1}, 3},
		{"BOS", []int{2, 1, 3}, 19},
		{"BOS", []int{3, 3, 3}, 26},
		{"V", []int{0}, 0},
		{"V", []int{9}, 9},
		{"W", []int{-2, 5}, 0},
		{"W", []int{2, 5}, 4},
		{"W", []int{-2, 6}, 5},
		{"W", []int{0, 6}, 7},
		{"D", []int{2, 0}, 0},
		{"D", []int{3, 1000}, 2001}, // no upper bound on the last

		{"BOS", []int{4, 1, 1}, -1},
		{"BOS", []int{1, 1}, -1},
		{"V", []int{-1}, -1},
		{"V", []int{10}, -1},
		{"W", []int{0, 4}, -1},
		{"D", []int{4, 0}, -1},
		{"D", []int{2, -1}, -1},
		{"E", []int{1}, -1},
	}

	for _, c := range cases {
		got, ok := u.Lookup(c.name).ElementNumber(c.subs)
		if !ok {
			got = -1
		}

		if got != c.want {
			t.Errorf("%s%v: got %d, want %d", c.name, c.subs, got, c.want)
		}
	}
}

// fixedLine is a test's line in fixed form: a leading label of digits
// goes in the label field, the rest in the statement field.
func fixedLine(l string) string {
	i := 0
	for i < len(l) && l[i] >= '0' && l[i] <= '9' {
		i++
	}

	if i > 0 && i < len(l) && l[i] == ' ' {
		return fmt.Sprintf("%-6s%s", l[:i], l[i+1:])
	}

	return "      " + l
}

// expressionUnit is the declarations TestExpressions's expressions are
// analyzed under.
var expressionUnit = []string{
	"      SUBROUTINE E(D, DA)",
	"      INTEGER I, J, IA(0:4), DA(*)",
	"      REAL X, Y, A(5)",
	"      LOGICAL L, M",
}

// expressionOf analyzes text as the one item of a PRINT statement in
// expressionUnit, returning the item's dump.
func expressionOf(t *testing.T, text string) (string, []diag.Message) {
	t.Helper()

	lines := append(append([]string{}, expressionUnit...), "      PRINT *, "+text, "      END")
	prog, ms := analyzeLines(t, "E.FOR", lines...)

	p, ok := prog.Units[0].Body[0].(*Print)
	if !ok || len(p.Items) != 1 {
		t.Fatalf("%s: no item: %s", text, Dump(prog))
	}

	return DumpExpr(p.Items[0]), ms
}

// TestExpressions checks expressions' types, the conversions mixed
// operations get (section 2.3.1.2), and folding.
func TestExpressions(t *testing.T) {
	cases := map[string]string{
		// Leaves.
		"I":       "I",
		"X":       "X",
		"L":       "L",
		"D":       "D",
		"A":       "A[]",
		"A(I)":    "A[I]",
		"IA(J)":   "IA[J]",
		"DA(3)":   "DA[3]",
		"A(X)":    "A[(INTEGER X)]",
		"A(L)":    "A[(INTEGER L)]",
		"F(I)":    "F(I)",
		"K(X)":    "K(X)",
		"F(A)":    "F(A[])",
		"F((I))":  "F((copy I))",
		"F(I+1)":  "F((+:I I 1))",
		"F(1)":    "F(1)",
		"'IT''S'": "'IT''S'",

		// Arithmetic: the result has the higher rank's type.
		"I + J":     "(+:I I J)",
		"I + X":     "(+:R (REAL I) X)",
		"X * I":     "(*:R X (REAL I))",
		"I / J * X": "(*:R (REAL (/:I I J)) X)",
		"-I":        "(-:I I)",
		"-X":        "(-:R X)",
		"+X":        "X",
		"L + 1":     "(+:I (INTEGER L) 1)",
		"L + X":     "(+:R (REAL (INTEGER L)) X)",
		"-L":        "(-:I (INTEGER L))",
		"(I + J)*2": "(*:I (+:I I J) 2)",

		// Comparisons: LOGICAL, the operands converted to one type.
		"I .LT. J":   "(.LT.:I I J)",
		"I .GE. X":   "(.GE.:R (REAL I) X)",
		"X+1 .EQ. Y": "(.EQ.:R (+:R X 1.0) Y)",
		"L .NE. M":   "(.NE.:I (INTEGER L) (INTEGER M))",

		// Logical operators: LOGICAL on LOGICALs, bitwise INTEGER
		// otherwise.
		"L .AND. M":       "(.AND.:L L M)",
		"L .OR. I .GT. J": "(.OR.:L L (.GT.:I I J))",
		".NOT. L":         "(.NOT.:L L)",
		"I .AND. J":       "(.AND.:I I J)",
		"I .OR. L":        "(.OR.:I I (INTEGER L))",
		".NOT. I":         "(.NOT.:I I)",
		"L .AND. .NOT. M": "(.AND.:L L (.NOT.:L M))",

		// Folding.
		"2 + 3":          "5",
		"7 / 2":          "3",
		"-7 / 2":         "-3",
		"2 * 3 + 4":      "10",
		"1 + 2.5":        "3.5",
		"1 / 4 + 1 / 4":  "0",
		"1.0 / 3":        "0.33333334",
		"-2147483648":    "-2147483648",
		"- (3)":          "-3",
		"2 .GT. 1":       ".TRUE.",
		"2.5 .LE. 1":     ".FALSE.",
		".NOT. .TRUE.":   ".FALSE.",
		".TRUE. .AND. 5": "5",
		"6 .AND. 3":      "2",
		"I + (2 - 2)":    "(+:I I 0)",
		// 2**24 + 1 is halfway between two F_floating values: the VAX
		// rounds it away from zero, where IEEE arithmetic would round it
		// to even (16777216).
		"16777217 + 0.0": "1.6777218E+07",
		"1E-38 / 1E10":   "0.0",

		// Forms compiled only as constant expressions (constant.go).
		"2 ** 10":               "1024",
		"2.0 ** (-2)":           "0.25",
		"I + 2 ** 3":            "(+:I I 8)",
		"MAX(1, 4) * J":         "(*:I 4 J)",
		"ICHAR('A')":            "65",
		"NINT(2.5D0)":           "3",
		"IMAG((1.0, 2.0))":      "2.0",
		"'AB' // 'C'":           "'ABC'",
		"'A' .LT. 'B'":          ".TRUE.",
		".TRUE. .EQV. .FALSE.":  ".FALSE.",
		"LGE('B', 'A') .AND. L": "(.AND.:L .TRUE. L)",
	}

	for text, want := range cases {
		got, ms := expressionOf(t, text)

		if len(ms) != 0 {
			t.Errorf("%s: %v", text, ms)

			continue
		}

		if got != want {
			t.Errorf("%s: got %s, want %s", text, got, want)
		}
	}
}

// TestConstants checks constants' values: REAL ones rounded to
// F_floating once, as the VAX rounds, and given as the longword the
// MACRO file will hold.
func TestConstants(t *testing.T) {
	cases := map[string]uint32{
		"1.0":       0x00004080,
		"1.5":       0x000040C0,
		"-2.5":      0x0000C120,
		"0.1":       0xCCCD3ECC,
		"1E3":       0x0000457A,
		"1.":        0x00004080,
		".5":        0x00004000,
		"3 * 0.5":   0x000040C0,
		"1 / 3.0":   0xAAAB3FAA,
		"16777217.": 0x00014C80, // halfway, rounded away from zero
		"0.0":       0,
	}

	for text, want := range cases {
		lines := append(append([]string{}, expressionUnit...), "      X = "+text, "      END")
		prog, ms := analyzeLines(t, "E.FOR", lines...)

		if len(ms) != 0 {
			t.Errorf("%s: %v", text, ms)

			continue
		}

		c, ok := prog.Units[0].Body[0].(*Assign).Value.(*Const)
		if !ok {
			t.Errorf("%s: not a constant", text)

			continue
		}

		if got := c.Bits(); got != want {
			t.Errorf("%s: ^X%08X, want ^X%08X", text, got, want)
		}
	}
}

// TestParameterValues checks constant expressions in every type through
// the value a PARAMETER statement gives its constant P: each case is P's
// declaration (or none, for its implicit type, or OLD for appendix A.4's
// form, P taking its value's type), its value's text, and the value
// dumped.
func TestParameterValues(t *testing.T) {
	cases := []struct {
		decl, value, want string
	}{
		// INTEGER, and conversions to it.
		{"INTEGER P", "2 + 3 * 4", "14"},
		{"INTEGER P", "-7 / 2", "-3"},
		{"INTEGER P", "2 ** 10", "1024"},
		{"INTEGER P", "(-2) ** 3", "-8"},
		{"INTEGER P", "2 ** (-1)", "0"},
		{"INTEGER P", "(-1) ** (-3)", "-1"},
		{"INTEGER P", "7 ** 0", "1"},
		{"INTEGER P", "1.23", "1"}, // section 4.11's MU
		{"INTEGER P", "-2.9", "-2"},
		{"INTEGER P", "(3.5, 1.0)", "3"},
		{"INTEGER P", "'17'O", "15"},
		{"INTEGER P", "'FFFFFFFF'X", "-1"},
		{"INTEGER P", "'1FFFFFFFF'X", "-1"},
		{"INTEGER*2 P", "'FFFF'X", "-1"},
		{"INTEGER*2 P", "200 * 300", ""}, // INTRANGE
		{"INTEGER P", "MAX(3, 7, 5)", "7"},
		{"INTEGER P", "MIN(3, -7, 5)", "-7"},
		{"INTEGER P", "ABS(-4)", "4"},
		{"INTEGER P", "MOD(-7, 3)", "-1"},
		{"INTEGER P", "DIM(7, 3) + DIM(3, 7)", "4"},
		{"INTEGER P", "ICHAR('A')", "65"},
		{"INTEGER P", "NINT(2.5) + NINT(-2.5)", "0"},
		{"INTEGER P", "NINT(2.49)", "2"},
		{"INTEGER P", "IAND(12, 10)", "8"},
		{"INTEGER P", "IOR(12, 10)", "14"},
		{"INTEGER P", "IEOR(12, 10)", "6"},
		{"INTEGER P", "NOT(0)", "-1"},
		{"INTEGER P", "ISHFT(1, 4)", "16"},
		{"INTEGER P", "ISHFT(-1, -28)", "15"},
		{"INTEGER P", "ISHFT(1, 32)", "0"},
		{"INTEGER P", "6 .AND. 3", "2"},
		{"INTEGER P", "6 .XOR. 3", "5"},
		{"INTEGER P", "6 .EQV. 3", "-6"},

		// REAL, D_floating, H_floating.
		{"REAL P", "1.23", "1.23"},
		{"REAL P", "1 / 3.0", "0.33333334"},
		{"REAL P", "2.0 ** 3", "8.0"},
		{"REAL P", "2.0 ** (-2)", "0.25"},
		{"REAL P", "MAX(1.5, 2.5)", "2.5"},
		{"REAL P", "MOD(7.5, 2.0)", "1.5"},
		{"REAL P", "ABS((3.0, 4.0))", "5.0"},
		{"REAL P", "IMAG((3.0, 4.0))", "4.0"},
		{"REAL P", "AIMAG((3.0, 4.0))", "4.0"},
		{"REAL P", "3.14159265358979D0", "3.1415927"},
		{"REAL P", "'4080'X", "1.0"},   // F_floating's bits, read as a little-endian longword
		{"REAL*8 P", "1.23", "1.23D0"}, // section 4.11's MU, declared REAL*8
		{"REAL*8 P", "3.141592653589793238D0", "3.1415926535897932D0"},
		{"REAL*8 P", "1D0 + 0.3333333", "1.3333333D0"}, // section 2.3.1.2
		{"REAL*8 P", "1D0 / 3", "0.333333333333333336D0"},
		{"REAL*8 P", "DPROD(0.1, 10.0)", "1.0000000149011612D0"},
		{"REAL*16 P", "1Q0 / 3", "0.3333333333333333333333333333333333Q0"},
		{"REAL*16 P", "1Q4000", "1Q4000"},
		{"DOUBLE PRECISION P", "2D0 ** 100", "1.2676506002282294D30"},

		// COMPLEX.
		{"COMPLEX P", "(1.0, -2.5)", "(1.0, -2.5)"},
		{"COMPLEX P", "(1, 2) * (3, 4)", "(-5.0, 10.0)"},
		{"COMPLEX P", "(1, 2) / (3, 4)", "(0.44, 0.08)"},
		{"COMPLEX P", "(0.0, 1.0) ** 2", "(-1.0, 0.0)"},
		{"COMPLEX P", "(0.0, 1.0) ** (-1)", "(0.0, -1.0)"},
		{"COMPLEX P", "CONJG((1.0, 2.0))", "(1.0, -2.0)"},
		{"COMPLEX P", "CMPLX(1, 2)", "(1.0, 2.0)"},
		{"COMPLEX P", "CMPLX(3.5)", "(3.5, 0.0)"},
		{"COMPLEX P", "2.5", "(2.5, 0.0)"},
		{"COMPLEX*16 P", "(1D0, 0.1)", "(1.0D0, 0.1D0)"},
		{"OLD", "(1.0, 2D0)", "(1.0D0, 2.0D0)"},
		{"OLD", "(1.0, 2.0) * 1D0", "(1.0D0, 2.0D0)"},

		// LOGICAL.
		{"LOGICAL P", ".TRUE. .AND. .NOT. .FALSE.", ".TRUE."},
		{"LOGICAL P", "2 .GT. 1", ".TRUE."},
		{"LOGICAL P", "(1.0, 2.0) .NE. (1.0, 2.0)", ".FALSE."},
		{"LOGICAL P", "'ABC' .EQ. 'ABC  '", ".TRUE."},
		{"LOGICAL P", "'AB' .LT. 'B'", ".TRUE."},
		{"LOGICAL P", "LGT('B', 'AAA')", ".TRUE."},
		{"LOGICAL P", "LLE('A', 'A')", ".TRUE."},
		{"LOGICAL P", "LGE(' ', '')", ".TRUE."},
		{"LOGICAL P", ".TRUE. .EQV. .FALSE.", ".FALSE."},
		{"LOGICAL P", "-1", ".TRUE."},
		{"LOGICAL*1 P", "256", ".FALSE."},

		// CHARACTER.
		{"CHARACTER*(*) P", "'A STRING OF 25 CHARACTERS'", "'A STRING OF 25 CHARACTERS'"},
		{"CHARACTER*5 P", "'AB'", "'AB   '"},
		{"CHARACTER*2 P", "'ABCD'", "'AB'"},
		{"CHARACTER*(*) P", "'AB' // 'CD' // CHAR(69)", "'ABCDE'"},

		// Appendix A.4's form: the value's own type.
		{"OLD", "3.1415927", "3.1415927"},
		{"OLD", "3.141592653589793238D0", "3.1415926535897932D0"},
		{"OLD", ".TRUE.", ".TRUE."},
		{"OLD", "'A STRING OF 25 CHARACTERS'", "'A STRING OF 25 CHARACTERS'"},
		{"OLD", "'FF'X", "255"},
	}

	for _, c := range cases {
		lines := []string{"      PARAMETER (P = " + c.value + ")"}

		switch c.decl {
		case "OLD":
			lines = []string{"      PARAMETER P = " + c.value}
		case "":
		default:
			lines = append([]string{"      " + c.decl}, lines...)
		}

		prog, ms := analyzeLines(t, "T.FOR", append(lines, "      END")...)
		p := prog.Units[0].Lookup("P")

		got := ""
		if p.Value != nil {
			got = dumpConst(p.Value)
		}

		if c.want == "" {
			if len(ms) == 0 {
				t.Errorf("%s = %s: got %s, want an error", c.decl, c.value, got)
			}

			continue
		}

		if len(ms) != 0 {
			t.Errorf("%s = %s: %v", c.decl, c.value, ms)

			continue
		}

		if got != c.want {
			t.Errorf("%s = %s: got %s, want %s", c.decl, c.value, got, c.want)
		}
	}
}
