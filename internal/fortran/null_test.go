package fortran

import (
	"fmt"
	"strings"
	"testing"
)

// TestNullArguments compiles a list with something left out wherever a
// reference can be (Phase 83, Decision 1): a function's arguments may be
// null, as a CALL's may, and the parser can't tell a function from an array,
// so semantic analysis refuses every other use. Each statement is compiled
// through to the MACRO file, so a null that reaches a pass not ready for one
// panics here.
func TestNullArguments(t *testing.T) {
	declarations := []string{
		"      REAL A(2,2)",
		"      CHARACTER*5 S(2,2)",
		"      INTEGER F",
		"      EXTERNAL F",
		"      STRUCTURE /P/",
		"        INTEGER K(2,2)",
		"      END STRUCTURE",
		"      RECORD /P/ R",
		"      SF(X,Y)=X+Y",
	}

	// want is the identifier of a message there must be, "" for no errors.
	// EQUIVALENCE and PARAMETER also get ORDER, being after the statement
	// function.
	cases := []struct{ statement, want string }{
		{"I = F()", ""},
		{"I = F(,)", ""},
		{"I = F(1,)", ""},
		{"I = F(,1)", ""},
		{"I = F(,,)", ""},
		{"I = F(1,,2)", ""},
		{"I = F(,) + F(,1)", ""},
		{"Q = G(1,)", ""},
		{"PRINT *, F(,)", ""},
		{"IF (F(,)) 10,10,10", ""},
		{"IF (F(,)) GO TO 10", ""},
		{"GO TO (10,10) F(,)", ""},
		{"WRITE (F(,),*) 1", ""},
		{"CALL H(%VAL(F(,)),,)", ""},
		{"CALL H(F(,),G(,))", ""},

		{"X = A(1,)", "NULLSUBSCRIPT"},
		{"X = A(,1)**2", "NULLSUBSCRIPT"},
		{"A(1,) = 2", "NULLSUBSCRIPT"},
		{"A(,) = 2", "NULLSUBSCRIPT"},
		{"PRINT *, A(,1)", "NULLSUBSCRIPT"},
		{"READ (5,*) A(1,)", "NULLSUBSCRIPT"},
		{"READ (5,*) (A(I,),I=1,2)", "NULLSUBSCRIPT"},
		{"CALL H(A(,1))", "NULLSUBSCRIPT"},
		{"S(,1)(1:2) = 'A'", "NULLSUBSCRIPT"},
		{"PRINT *, S(1,)(1:2)", "NULLSUBSCRIPT"},
		{"I = R.K(1,)", "NULLSUBSCRIPT"},
		{"R.K(,1) = 1", "NULLSUBSCRIPT"},
		{"DATA A(1,)/1./", "NULLSUBSCRIPT"},
		{"EQUIVALENCE (A(,1),B)", "NULLSUBSCRIPT"},

		{"X = SF(1.,)", "NULLSTMTARG"},
		{"X = SF(,)", "NULLSTMTARG"},
		{"X = SQRT(,)", "NULLINTRINARG"},
		{"PARAMETER (P=MAX(1,))", "NULLINTRINARG"},
	}

	for _, c := range cases {
		t.Run(c.statement, func(t *testing.T) {
			lines := append([]string{"      PROGRAM T"}, declarations...)
			lines = append(lines, "      "+c.statement, "10    CONTINUE", "      END")

			r := Compile(testOptions, File{Spec: "T.FOR;1", Name: "T", Lines: lines})

			switch {
			case c.want == "" && r.Errors != 0:
				t.Errorf("messages: %v", r.Messages)
			case c.want != "" && !strings.Contains(fmt.Sprint(r.Messages), "-"+c.want+","):
				t.Errorf("messages: %v, want %s", r.Messages, c.want)
			}
		})
	}
}
