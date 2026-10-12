package dclshell

import (
	"strings"
	"testing"
)

// Tests of labels and GOTO (dcllabel.go), IF blocks (dclif.go), and GOSUB
// and CALL (dclcall.go), from the User's Manual's rules and examples
// (13.2, 14.16, 14.17).

// TestSearchLabel: the forward search follows the blocks it passes.
func TestSearchLabel(t *testing.T) {
	p := newProcedureSource("S.COM", strings.Join([]string{
		"$ A: PRINT 1", // 0
		"$ IF 1",       // 1
		"$ THEN",       // 2
		"$ B:",         // 3: inside a block the search enters
		"$ ELSE",       // 4
		"$ ENDIF",      // 5
		"$ ELSE",       // 6: the other branch of a block open at the start
		"$ C:",         // 7
		"$ ENDIF",      // 8
		"data",         // 9
		"$ S: SUBROUTINE",
		"$ D:", // 11: inside a subroutine
		"$ ENDSUBROUTINE",
		"$ E: ENDIF", // 13
		"$ F: SUBROUTINE",
	}, "\n"))

	for _, tc := range []struct {
		label   string
		open    int
		found   bool
		pos     int
		depth   int
		unended bool
	}{
		{"A", 0, true, 0, 0, false},
		{"B", 0, false, 0, 0, true},
		{"C", 1, false, 0, 0, true},
		{"C", 0, true, 7, 0, false},
		{"D", 0, false, 0, 0, true},
		{"S", 0, true, 10, 0, false},
		{"E", 2, true, 13, 1, false},
		{"NONE", 0, false, 0, 0, true},
	} {
		target, found, unended := p.searchLabel(tc.label, 0, tc.open, nil)
		if found != tc.found || (found && (target.pos != tc.pos || target.depth != tc.depth)) || (!found && unended != tc.unended) {
			t.Errorf("%s with %d open: %+v, %v, %v; want %v at %d, depth %d, unended %v",
				tc.label, tc.open, target, found, unended, tc.found, tc.pos, tc.depth, tc.unended)
		}
	}

	if end, ok := p.subroutineEnd(11); !ok || end != 12 {
		t.Errorf("subroutineEnd: %d, %v; want 12", end, ok)
	}
}
