package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ast"
)

// TestF95Forms checks Fortran 95's statements parsed (docs/planning/PHASE-112
// - Fortran 95.md, subtask 2): PURE and ELEMENTAL headers, FORALL's statement
// and construct, construct names on WHERE, ELSEWHERE, and END WHERE, and END
// INTERFACE's generic name.
func TestF95Forms(t *testing.T) {
	file, ms := parseFree(t, "F.F90",
		"program p",
		"  interface swap",
		"    pure subroutine s(a, b)",
		"      integer, intent(inout) :: a, b",
		"    end subroutine s",
		"  end interface swap",
		"  interface operator(.x.)",
		"    elemental real function cross(a, b)",
		"      real, intent(in) :: a, b",
		"    end function cross",
		"  end interface operator(.x.)",
		"  forall (i = 1:n, j = 1:n:2, a(i,j) /= 0.0) b(i,j) = 1.0 / a(i,j)",
		"  forall (j = 1:8) pat(j)%p => obj(j)",
		"  outer: forall (i = 1:n)",
		"    a(i,i) = 0",
		"  end forall outer",
		"  w: where (a > 0)",
		"    a = 1",
		"  elsewhere (a < 0) w",
		"    a = 2",
		"  elsewhere w",
		"    a = 3",
		"  end where w",
		"end program p",
		"recursive pure integer function f(n) result(r)",
		"  integer, intent(in) :: n",
		"  r = n",
		"end function f",
	)

	if len(ms) != 0 {
		t.Fatalf("messages: %v", ms)
	}

	want := strings.Join([]string{
		"== main program P",
		"1           PROGRAM P",
		"2           INTERFACE SWAP",
		"              == SUBROUTINE S",
		"3             PURE SUBROUTINE S(A, B)",
		"4             INTEGER, INTENT(INOUT) :: A, B",
		"5             END SUBROUTINE S",
		"6           END INTERFACE SWAP",
		"7           INTERFACE OPERATOR(.X.)",
		"              == FUNCTION CROSS",
		"8             ELEMENTAL REAL FUNCTION CROSS(A, B)",
		"9             REAL, INTENT(IN) :: A, B",
		"10            END FUNCTION CROSS",
		"11          END INTERFACE OPERATOR(.X.)",
		"12          FORALL (I=1:N, J=1:N:2, (.NE. A(I, J) 0.0)) B(I, J) = (/ 1.0 A(I, J))",
		"13          FORALL (J=1:8) PAT(J)%P => OBJ(J)",
		"14          FORALL (I=1:N) OUTER",
		"15          A(I, I) = 0",
		"16          END FORALL OUTER",
		"17          WHERE (.GT. A 0) W",
		"18          A = 1",
		"19          ELSEWHERE (.LT. A 0) W",
		"20          A = 2",
		"21          ELSEWHERE W",
		"22          A = 3",
		"23          END WHERE W",
		"24          END PROGRAM P",
		"== FUNCTION F",
		"25          RECURSIVE PURE INTEGER FUNCTION F(N) RESULT(R)",
		"26          INTEGER, INTENT(IN) :: N",
		"27          R = N",
		"28          END FUNCTION F",
		"",
	}, "\n")

	if got := ast.Dump(file); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	errors := []struct{ line, want string }{
		{"forall (i = 1:n) call s(i)", "BADFORALLSTMT"},
		{"forall (i = 1:n) if (x) y = 1", "BADFORALLSTMT"},
		{"forall (a(i) > 0) b = 1", "SYNTAX"},
		{"forall (i = 1:n, a(i) > 0, j = 1:2) b = 1", "SYNTAX"},
		{"forall (i = 1) b = 1", "SYNTAX"},
		{"end forall a, b", "EXTRATEXT"},
	}

	for _, e := range errors {
		_, ms := parseFree(t, "E.F90", e.line, "end")
		if got := strings.Join(idents(ms), " "); got != e.want {
			t.Errorf("%s: got %v, want %s", e.line, ms, e.want)
		}
	}

	for _, c := range []struct{ open, end, want string }{
		{"interface swap", "end interface swop", "ENDINTERFACE"},
		{"interface", "end interface swap", "ENDINTERFACE"},
		{"interface operator(+)", "end interface operator(-)", "ENDINTERFACE"},
		{"interface assignment(=)", "end interface assignment(=)", ""},
		{"interface swap", "end interface", ""},
	} {
		_, ms := parseFree(t, "I.F90", "program q", c.open, c.end, "end")
		if got := strings.Join(idents(ms), " "); got != c.want {
			t.Errorf("%s ... %s: got %v, want %s", c.open, c.end, ms, c.want)
		}
	}
}

// TestF95Probes checks that the survey's probe programs (testdata/f95), which
// lost their units' structure at the first PURE or FORALL before subtask 2,
// parse without a message.
func TestF95Probes(t *testing.T) {
	for _, name := range []string{"pure", "forall", "where95", "misc95", "elemasg", "nullinit", "intrin95", "zerosize", "zerowid"} {
		text, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "f95", name+".f90"))
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(strings.TrimRight(string(text), "\n"), "\n")

		if _, ms := parseFree(t, strings.ToUpper(name)+".F90", lines...); len(ms) != 0 {
			t.Errorf("%s: %v", name, ms)
		}
	}
}
