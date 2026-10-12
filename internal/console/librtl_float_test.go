package console

import (
	"fmt"
	"strings"
	"testing"
)

// The floating routines (docs/PHASE-53, subtask 8), called from a linked
// program as a program calls them.

// floatCheck is check for a floating comparison in format x (F, D, G,
// or H): if the operands differ, the program returns n.
func floatCheck(n int, x, operands string) string {
	return fmt.Sprintf("\tCMP%s\t%s\n\tBEQL\t%d$\n\tMOVL\t#%d,R0\n\tRET\n%d$:\n", x, operands, n, n, n)
}

// TestLibrtlFloating: LIB$EMODx and LIB$POLYx in each format through
// their stubs. EMOD of -2.5 by 1.5 is -3 and -0.75; POLY is the manual's
// FORTRAN example, x^4 + 2x^3 - x^2 + x - 3 at 2, which is 27. Then
// LIB$POLYF of degree 32 returns SS$_ROPRAND. Each check that fails
// returns its own number; all passing returns 1.
func TestLibrtlFloating(t *testing.T) {
	var data, code strings.Builder

	data.WriteString("\t.PSECT\tDATA,NOEXE,WRT,QUAD\nDEG4:\t.WORD\t4\nDEG32:\t.WORD\t32\nEXT:\t.WORD\t0\nINT:\t.LONG\t0\n")
	code.WriteString("\t.PSECT\tCODE,NOWRT,EXE\n\t.ENTRY\tSTART,^M<>\n")

	n := 2

	for _, x := range []string{"F", "D", "G", "H"} {
		fmt.Fprintf(&data, "\t.ALIGN\tQUAD\n"+
			"%[1]sMULR:\t.%[1]s_FLOATING\t-2.5\n%[1]sMULD:\t.%[1]s_FLOATING\t1.5\n"+
			"%[1]sFRAC:\t.BLKO\t1\n%[1]sWANTF:\t.%[1]s_FLOATING\t-0.75\n"+
			"%[1]sX:\t.%[1]s_FLOATING\t2.0\n"+
			"%[1]sCOEF:\t.%[1]s_FLOATING\t1.0,2.0,-1.0,1.0,-3.0\n"+
			"%[1]sRES:\t.BLKO\t1\n%[1]sWANTP:\t.%[1]s_FLOATING\t27.0\n", x)

		fmt.Fprintf(&code, "\tPUSHAB\t%[1]sFRAC\n\tPUSHAL\tINT\n\tPUSHAB\t%[1]sMULD\n\tPUSHAW\tEXT\n\tPUSHAB\t%[1]sMULR\n"+
			"\tCALLS\t#5,G^LIB$EMOD%[1]s\n", x)
		code.WriteString(check(n, "R0,#1") + check(n+1, "INT,#-3") + floatCheck(n+2, x, x+"FRAC,"+x+"WANTF"))

		fmt.Fprintf(&code, "\tPUSHAB\t%[1]sRES\n\tPUSHAB\t%[1]sCOEF\n\tPUSHAW\tDEG4\n\tPUSHAB\t%[1]sX\n"+
			"\tCALLS\t#4,G^LIB$POLY%[1]s\n", x)
		code.WriteString(check(n+3, "R0,#1") + floatCheck(n+4, x, x+"RES,"+x+"WANTP"))

		n += 5
	}

	code.WriteString("\tPUSHAB\tFRES\n\tPUSHAB\tFCOEF\n\tPUSHAW\tDEG32\n\tPUSHAB\tFX\n" +
		"\tCALLS\t#4,G^LIB$POLYF\n" + check(n, "R0,#^X454") +
		"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	r0, out := runMacroProgram(t, data.String()+code.String())

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (check %d failed)", r0, r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}
