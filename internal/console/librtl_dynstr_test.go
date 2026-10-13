package console

import "testing"

// TestLibrtlDynamicStrings: a linked program gets dynamic strings from
// LIB$SGET1_DD (CALL) and LIB$SGET1_DD_R6 (JSB), writes text into them,
// shows them with LIB$PUT_OUTPUT, and frees both with LIB$SFREEN_DD6.
// The program ends with 1 when every status was a success and both
// pointers are 0 again.
func TestLibrtlDynamicStrings(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"DESC:\t.LONG\t0,0\nD2:\t.LONG\t0,0\nLEN:\t.WORD\t5\n"+
		"HELLO:\t.ASCII\t/HELLO/\nBYE:\t.ASCII\t/BYE/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2,R3,R4,R5,R6>\n"+
		"\tPUSHAQ\tDESC\n\tPUSHAW\tLEN\n\tCALLS\t#2,G^LIB$SGET1_DD\n\tBLBC\tR0,10$\n"+
		"\tMOVC3\t#5,HELLO,@DESC+4\n"+
		"\tPUSHAQ\tDESC\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#3,R0\n\tMOVAQ\tD2,R1\n\tJSB\tG^LIB$SGET1_DD_R6\n\tBLBC\tR0,10$\n"+
		"\tMOVC3\t#3,BYE,@D2+4\n"+
		"\tPUSHAQ\tD2\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#2,R0\n\tMOVAQ\tDESC,R1\n\tJSB\tG^LIB$SFREEN_DD6\n\tBLBC\tR0,10$\n"+
		"\tTSTL\tDESC+4\n\tBNEQ\t20$\n\tTSTL\tD2+4\n\tBNEQ\t20$\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (20: a pointer wasn't cleared; otherwise the failing status)", r0)
	}

	if out != "HELLO\nBYE\n" {
		t.Errorf("output %q, want %q", out, "HELLO\nBYE\n")
	}
}
