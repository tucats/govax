package console

import (
	"testing"
)

// The character string routines (docs/PHASE-53, subtask 10), called from
// a linked program as a program calls them.

// charsData is the data the program uses: the manual's strings, a table
// for LIB$SCANC (digits have bit 0), an uppercasing table for LIB$MOVTC
// and LIB$MOVTUC, and destinations.
const charsData = "\t.PSECT\tDATA,NOEXE,WRT,LONG\n" +
	"DAY:\t.ASCID\t/DAY/\nTEXT:\t.ASCID\t/ONE DAY AT A TIME/\nBLANK:\t.ASCID\t/ /\n" +
	"ABC:\t.ASCID\t/ ABC/\nTIME:\t.ASCID\t/TIME/\nDIGITS:\t.ASCID\t/AB3C/\n" +
	"STAR:\t.ASCID\t/*/\nDOT:\t.ASCID\t/./\nLOWER:\t.ASCID\t/abc.de/\n" +
	"OUT:\t.ASCID\t/12345678/\nUPDESC:\t.LONG\t256\n\t.ADDRESS\tUPCASE\n" +
	"MASK:\t.BYTE\t1\nLEN5:\t.WORD\t5\nLEN3:\t.WORD\t3\nBUF:\t.ASCII\t/ABCDEFGH/\n" +
	"DIGTAB:\t.BYTE\t0[48]\n\t.BYTE\t1[10]\n\t.BYTE\t0[198]\n" +
	"UPCASE:\n\t.REPT\t97\n\t.BYTE\t.-UPCASE\n\t.ENDR\n" +
	"\t.REPT\t26\n\t.BYTE\t.-UPCASE-32\n\t.ENDR\n" +
	"\t.REPT\t133\n\t.BYTE\t.-UPCASE\n\t.ENDR\n"

// TestLibrtlChars: LIB$LOCC, LIB$SKPC, LIB$MATCHC, LIB$SCANC, LIB$SPANC,
// LIB$MOVC3, LIB$MOVC5, LIB$MOVTC, and LIB$MOVTUC through their stubs.
// Each check that fails returns its own number; all passing returns 1.
func TestLibrtlChars(t *testing.T) {
	r0, out := runMacroProgram(t, charsData+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		// The manual's examples: LOCC of "DAY" is 5, SKPC of " " is 2.
		"\tPUSHAQ\tTEXT\n\tPUSHAQ\tDAY\n\tCALLS\t#2,G^LIB$LOCC\n"+check(2, "R0,#5")+
		"\tPUSHAQ\tABC\n\tPUSHAQ\tBLANK\n\tCALLS\t#2,G^LIB$SKPC\n"+check(3, "R0,#2")+
		"\tPUSHAQ\tTEXT\n\tPUSHAQ\tTIME\n\tCALLS\t#2,G^LIB$MATCHC\n"+check(4, "R0,#14")+
		// The first digit of "AB3C" is at 3; the first non-digit at 1.
		"\tPUSHAB\tMASK\n\tPUSHAB\tDIGTAB\n\tPUSHAQ\tDIGITS\n\tCALLS\t#3,G^LIB$SCANC\n"+check(5, "R0,#3")+
		"\tPUSHAB\tMASK\n\tPUSHAB\tDIGTAB\n\tPUSHAQ\tDIGITS\n\tCALLS\t#3,G^LIB$SPANC\n"+check(6, "R0,#1")+
		// LIB$MOVC3 of 5 bytes two along: ABABCDEH.
		"\tPUSHAB\tBUF+2\n\tPUSHAB\tBUF\n\tPUSHAW\tLEN5\n\tCALLS\t#3,G^LIB$MOVC3\n"+
		check(7, "BUF,#^A/ABAB/")+check(8, "BUF+4,#^A/CDEH/")+
		// LIB$MOVC5 of 3 bytes into 5, filled with "*": ABA**.
		"\tPUSHAB\tBUF\n\tPUSHAW\tLEN5\n\tPUSHAB\tSTAR+8\n\tPUSHAB\tBUF\n\tPUSHAW\tLEN3\n"+
		"\tCALLS\t#5,G^LIB$MOVC5\n"+check(9, "BUF,#^A/ABA*/")+
		// LIB$MOVTC of "abc.de" into 8: ABC.DE**.
		"\tPUSHAQ\tOUT\n\tPUSHAQ\tUPDESC\n\tPUSHAQ\tSTAR\n\tPUSHAQ\tLOWER\n\tCALLS\t#4,G^LIB$MOVTC\n"+
		check(10, "R0,#1")+"\tMOVL\tOUT+4,R1\n"+check(11, "(R1),#^A/ABC./")+check(12, "4(R1),#^A/DE**/")+
		// LIB$MOVTUC until ".": position 4, ABC and the rest filled.
		"\tPUSHAQ\tBLANK\n\tPUSHAQ\tOUT\n\tPUSHAQ\tUPDESC\n\tPUSHAQ\tDOT\n\tPUSHAQ\tLOWER\n"+
		"\tCALLS\t#5,G^LIB$MOVTUC\n"+check(13, "R0,#4")+
		"\tMOVL\tOUT+4,R1\n"+check(14, "(R1),#^A/ABC /")+
		"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (check %d failed)", r0, r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}
