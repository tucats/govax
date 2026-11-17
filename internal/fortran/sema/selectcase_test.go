package sema

import (
	"fmt"
	"strings"
	"testing"
)

// TestSelectCase checks the CASE construct's analysis (selectcase.go): each
// case is a main program's statements in fixed form and the messages it
// gets, as TestErrors's are. The construct's code is subtask 4's, so its
// NOTIMPL, which every SELECT CASE gets until then, isn't listed.
func TestSelectCase(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"INTEGER cases", []string{"SELECT CASE (I)", "CASE (1, 3:5)", "X = 1", "CASE DEFAULT", "X = 2", "CASE (:0, 10:)", "X = 3",
			"CASE (6)", "END SELECT", "END"}, ""},
		{"CHARACTER cases", []string{"CHARACTER*4 C", "SELECT CASE (C(1:2))", "CASE ('A':'M', 'Z')", "CASE ('N':'Y')", "CASE ('ZZ':)",
			"END SELECT", "END"}, ""},
		{"LOGICAL cases", []string{"LOGICAL L", "SELECT CASE (L)", "CASE (.TRUE.)", "CASE (.FALSE.)", "END SELECT", "END"}, ""},
		{"no cases", []string{"SELECT CASE (I)", "END SELECT", "END"}, ""},
		{"a named construct", []string{"PICK: SELECT CASE (I)", "CASE (1) PICK", "CASE DEFAULT PICK", "END SELECT PICK", "END"}, ""},
		{"nested constructs", []string{"SELECT CASE (I)", "CASE (1)", "SELECT CASE (J)", "CASE (1)", "END SELECT", "CASE (2)",
			"DO K = 1, 2", "IF (K .EQ. 2) THEN", "END IF", "END DO", "END SELECT", "END"}, ""},
		{"constant expressions", []string{"PARAMETER (N = 10)", "CHARACTER*(*) A", "PARAMETER (A = 'AB')", "SELECT CASE (I)",
			"CASE (N, N+1:2*N)", "CASE (-N:-1)", "END SELECT", "SELECT CASE (A)", "CASE (A // 'C')", "END SELECT", "END"}, ""},
		{"another INTEGER kind", []string{"INTEGER*2 K", "PARAMETER (M = 100000)", "SELECT CASE (K)", "CASE (1, 32767)", "END SELECT",
			"SELECT CASE (M)", "CASE (K)", "END SELECT", "SELECT CASE (I)", "CASE (2**20)", "END SELECT", "END"}, "CASEVALUE@7:13"},

		// The construct's statements in their places.
		{"a statement before the first CASE", []string{"SELECT CASE (I)", "X = 1", "Y = 2", "CASE (1)", "END SELECT", "END"},
			"CASEFIRST@2:7"},
		{"a FORMAT before the first CASE", []string{"SELECT CASE (I)", "10 FORMAT (I5)", "CASE (1)", "END SELECT", "END"},
			"CASEFIRST@2:7"},
		{"CASE outside", []string{"CASE (1)", "END"}, "NOSELECT@1:7"},
		{"END SELECT outside", []string{"END SELECT", "END"}, "NOSELECT@1:7"},
		{"CASE in an IF block", []string{"IF (L) THEN", "CASE (1)", "END IF", "END"}, "NOSELECT@2:7"},
		{"no END SELECT", []string{"SELECT CASE (I)", "CASE (1)", "END"}, "NOENDSELECT@1:7"},
		{"a loop open at CASE", []string{"SELECT CASE (I)", "CASE (1)", "DO J = 1, 2", "CASE (2)", "END SELECT", "END"},
			"DONEST@4:7"},
		{"an IF block open at END SELECT", []string{"SELECT CASE (I)", "CASE (1)", "IF (L) THEN", "END SELECT", "END"},
			"DONEST@4:7"},
		{"a construct open at END IF", []string{"IF (L) THEN", "SELECT CASE (I)", "CASE (1)", "END IF", "END"}, "DONEST@4:7"},
		{"a construct open at END DO", []string{"DO J = 1, 2", "SELECT CASE (I)", "CASE (1)", "END DO", "END"}, "DONEST@4:7"},
		{"a labeled loop ending in a CASE", []string{"DO 10 J = 1, 2", "SELECT CASE (I)", "10 CASE (1)", "END SELECT", "END"},
			"DOTERM@3:7"},
		{"a labeled loop ending at END SELECT", []string{"SELECT CASE (I)", "CASE (1)", "DO 10 J = 1, 2", "10 END SELECT", "END"},
			"DOTERM@4:7"},
		{"a logical IF's CASE", []string{"SELECT CASE (I)", "IF (L) CASE (1)", "CASE (1)", "END SELECT", "END"}, "BADIFSTMT@2:14 CASEFIRST@2:7"},

		// The expression and the values.
		{"a REAL expression", []string{"SELECT CASE (X)", "CASE (1)", "END SELECT", "END"}, "CASEEXPR@1:20"},
		{"a COMPLEX expression", []string{"COMPLEX Z", "SELECT CASE (Z)", "CASE DEFAULT", "END SELECT", "END"}, "CASEEXPR@2:20"},
		{"an array", []string{"DIMENSION K(3)", "SELECT CASE (K)", "CASE (1)", "END SELECT", "END"}, "NEEDSUBSCRIPT@2:20"},
		{"a value that isn't constant", []string{"SELECT CASE (I)", "CASE (J)", "CASE (1:N)", "END SELECT", "END"},
			"CASEVALUE@2:13 CASEVALUE@3:15"},
		{"a CHARACTER value for an INTEGER", []string{"SELECT CASE (I)", "CASE ('A')", "END SELECT", "END"}, "CASETYPE@2:13"},
		{"an INTEGER value for a CHARACTER", []string{"CHARACTER C", "SELECT CASE (C)", "CASE (1:2)", "END SELECT", "END"},
			"CASETYPE@3:13 CASETYPE@3:15"},
		{"a REAL value for an INTEGER", []string{"SELECT CASE (I)", "CASE (1.5)", "END SELECT", "END"}, "CASETYPE@2:13"},
		{"a LOGICAL value for an INTEGER", []string{"SELECT CASE (I)", "CASE (.TRUE.)", "END SELECT", "END"}, "CASETYPE@2:13"},
		{"an INTEGER value for a LOGICAL", []string{"LOGICAL L", "SELECT CASE (L)", "CASE (1)", "END SELECT", "END"}, "CASETYPE@3:13"},
		{"a number for a BYTE", []string{"BYTE B", "SELECT CASE (B)", "CASE (1)", "CASE (.TRUE.)", "END SELECT", "END"}, "CASEBYTE@3:13"},
		{"INTEGER(1) for a number", []string{"INTEGER*1 B", "SELECT CASE (B)", "CASE (1)", "CASE (-128:-1)", "END SELECT", "END"}, ""},
		{"a range of LOGICAL values", []string{"LOGICAL L", "SELECT CASE (L)", "CASE (.FALSE.:)", "END SELECT", "END"}, "CASERANGE@3:13"},
		{"two DEFAULTs", []string{"SELECT CASE (I)", "CASE DEFAULT", "CASE (1)", "CASE DEFAULT", "END SELECT", "END"}, "TWODEFAULT@4:7"},
		{"no case value", []string{"SELECT CASE (I)", "CASE (:)", "END SELECT", "END"}, "SYNTAX@2:14"},
		{"CASE DEFAULT with values", []string{"SELECT CASE (I)", "CASE DEFAULT (1)", "END SELECT", "END"}, "EXTRATEXT@2:20"},

		// Overlaps.
		{"two equal values", []string{"SELECT CASE (I)", "CASE (1)", "CASE (2, 1)", "END SELECT", "END"}, "CASEOVERLAP@3:16"},
		{"a value in a range", []string{"SELECT CASE (I)", "CASE (1:5)", "CASE (3)", "END SELECT", "END"}, "CASEOVERLAP@3:13"},
		{"overlapping ranges", []string{"SELECT CASE (I)", "CASE (1:5)", "CASE (5:9)", "END SELECT", "END"}, "CASEOVERLAP@3:13"},
		{"open ranges", []string{"SELECT CASE (I)", "CASE (10:)", "CASE (:10)", "END SELECT", "END"}, "CASEOVERLAP@3:13"},
		{"open ranges apart", []string{"SELECT CASE (I)", "CASE (11:)", "CASE (:10)", "END SELECT", "END"}, ""},
		{"an overlap in one CASE", []string{"SELECT CASE (I)", "CASE (1:3, 2)", "END SELECT", "END"}, "CASEOVERLAP@2:18"},
		{"an overlap past the kind", []string{"INTEGER*2 K", "SELECT CASE (K)", "CASE (32767:)", "CASE (32767:99999)", "END SELECT", "END"},
			"CASEOVERLAP@4:13"},
		{"CHARACTER values padded", []string{"CHARACTER*2 C", "SELECT CASE (C)", "CASE ('A')", "CASE ('A ')", "END SELECT", "END"},
			"CASEOVERLAP@4:13"},
		{"CHARACTER ranges", []string{"CHARACTER*2 C", "SELECT CASE (C)", "CASE ('A':'M')", "CASE ('L')", "CASE ('MA':)", "END SELECT", "END"},
			"CASEOVERLAP@4:13"},
		{"CHARACTER ranges apart", []string{"CHARACTER*2 C", "SELECT CASE (C)", "CASE ('A':'M')", "CASE ('M ':'Z')", "END SELECT", "END"},
			"CASEOVERLAP@4:13"},
		{"LOGICAL values", []string{"LOGICAL L", "SELECT CASE (L)", "CASE (.TRUE.)", "CASE (.NOT. .FALSE.)", "END SELECT", "END"},
			"CASEOVERLAP@4:13"},
		{"nested constructs don't overlap", []string{"SELECT CASE (I)", "CASE (1)", "SELECT CASE (I)", "CASE (1)", "END SELECT",
			"END SELECT", "END"}, ""},

		// Ranges that can never match: a warning.
		{"a reversed range", []string{"SELECT CASE (I)", "CASE (5:1)", "CASE (3)", "END SELECT", "END"}, "CASENEVER@2:13"},
		{"a reversed CHARACTER range", []string{"CHARACTER C", "SELECT CASE (C)", "CASE ('Z':'A')", "END SELECT", "END"}, "CASENEVER@3:13"},
		{"a value past the kind", []string{"INTEGER*2 K", "SELECT CASE (K)", "CASE (100000)", "CASE (-40000:-32769)", "CASE (-40000:-32768)",
			"END SELECT", "END"}, "CASENEVER@3:13 CASENEVER@4:13"},
		{"INTEGER*1's range", []string{"INTEGER*1 K", "SELECT CASE (K)", "CASE (128:)", "CASE (127:)", "END SELECT", "END"}, "CASENEVER@3:13"},

		// Construct names.
		{"CASE naming another construct", []string{"PICK: SELECT CASE (I)", "CASE (1) OTHER", "END SELECT PICK", "END"}, "WRONGNAME@2:16"},
		{"END SELECT without its name", []string{"PICK: SELECT CASE (I)", "CASE (1)", "END SELECT", "END"}, "MISSINGNAME@3:7"},
		{"END SELECT naming an unnamed construct", []string{"SELECT CASE (I)", "END SELECT PICK", "END"}, "WRONGNAME@2:18"},
		{"CASE DEFAULT's name", []string{"PICK: SELECT CASE (I)", "CASE DEFAULT OTHER", "END SELECT PICK", "END"}, "WRONGNAME@2:20"},
		{"EXIT naming a SELECT CASE", []string{"DO J = 1, 2", "PICK: SELECT CASE (I)", "CASE (1)", "EXIT PICK", "END SELECT PICK",
			"END DO", "END"}, "NOTALOOP@4:12"},
		{"EXIT in a SELECT CASE in a loop", []string{"DO J = 1, 2", "SELECT CASE (I)", "CASE (1)", "EXIT", "CASE (2)", "CYCLE",
			"END SELECT", "END DO", "END"}, ""},
		{"a name used twice", []string{"PICK: SELECT CASE (I)", "END SELECT PICK", "PICK: DO", "END DO PICK", "END"}, "DUPCONSTRUCT@3:7"},
		{"a construct's name a variable's", []string{"PICK: SELECT CASE (I)", "END SELECT PICK", "PICK = 1", "END"}, "NAMECLASH@1:7"},

		// Jumps.
		{"a jump into a block", []string{"GO TO 10", "SELECT CASE (I)", "CASE (1)", "10 X = 1", "END SELECT", "END"}, "INTOBLOCK@1:13"},
		{"a jump between blocks", []string{"SELECT CASE (I)", "CASE (1)", "10 X = 1", "CASE (2)", "GO TO 10", "END SELECT", "END"},
			"INTOBLOCK@5:13"},
		{"a jump within a block", []string{"SELECT CASE (I)", "CASE (1)", "10 X = 1", "GO TO 10", "END SELECT", "END"}, ""},
		{"a jump to END SELECT from any block", []string{"SELECT CASE (I)", "CASE (1)", "GO TO 10", "CASE (2)", "GO TO 10",
			"CASE DEFAULT", "GO TO 10", "10 END SELECT", "END"}, ""},
		{"a jump to END SELECT from outside", []string{"GO TO 10", "SELECT CASE (I)", "CASE (1)", "10 END SELECT", "END"}, "INTOBLOCK@1:13"},
		{"a jump to END SELECT from a nested block", []string{"SELECT CASE (I)", "CASE (1)", "IF (L) THEN", "GO TO 10", "END IF",
			"10 END SELECT", "END"}, ""},
		{"a jump to SELECT CASE", []string{"10 SELECT CASE (I)", "CASE (1)", "GO TO 10", "END SELECT", "GO TO 10", "END"}, ""},
		{"a jump to CASE", []string{"SELECT CASE (I)", "10 CASE (1)", "GO TO 10", "END SELECT", "END"}, "NOTEXEC@3:13"},
	}

	for _, c := range cases {
		lines := make([]string, len(c.lines))
		for i, l := range c.lines {
			lines[i] = fixedLine(l)
		}

		_, ms := analyzeLines(t, "T.FOR", lines...)

		var got []string

		for _, m := range ms {
			if m.Ident == "NOTIMPL" && strings.Contains(m.Text, "SELECT CASE") {
				continue
			}

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

// TestIntoBlockWording checks that INTOBLOCK names the kind of construct a
// jump goes into.
func TestIntoBlockWording(t *testing.T) {
	for want, lines := range map[string][]string{
		"an IF block":             {"      GO TO 10", "      IF (L) THEN", "   10 X = 1", "      END IF", "      END"},
		"a SELECT CASE construct": {"      GO TO 10", "      SELECT CASE (I)", "      CASE (1)", "   10 X = 1", "      END SELECT", "      END"},
	} {
		_, ms := analyzeLines(t, "T.FOR", lines...)

		found := false

		for _, m := range ms {
			found = found || m.Ident == "INTOBLOCK" && strings.Contains(m.Text, "goes into "+want+" from outside it")
		}

		if !found {
			t.Errorf("%s: %v", want, ms)
		}
	}
}
