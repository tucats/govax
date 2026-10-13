package console

import "testing"

// TestLibrtlJackets: a program calls LIB$SYS_FAO and LIB$SYS_GETMSG
// through their stubs with a dynamic string, and writes each result with
// LIB$PUT_OUTPUT. The jackets' scratch buffers are in their own frames:
// the caller's SP is the same after each call (docs/PHASE-55, subtask 2).
func TestLibrtlJackets(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+ // a dynamic string, empty
		"CTRL:\t.ASCID\t/!UL jacket!%S, !AS/\n"+
		"WORD:\t.ASCID\t/ok/\n"+
		"MSG:\t.LONG\t1\n"+ // SS$_NORMAL
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2>\n"+
		"\tMOVL\tSP,R2\n"+
		"\tPUSHAQ\tWORD\n\tPUSHL\t#3\n\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#5,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tCMPL\tSP,R2\n\tBNEQ\tMOVED\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAL\tMSG\n"+
		"\tCALLS\t#3,G^LIB$SYS_GETMSG\n\tBLBC\tR0,DONE\n"+
		"\tCMPL\tSP,R2\n\tBNEQ\tMOVED\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#1,R0\nDONE:\tRET\n"+
		"MOVED:\tMOVL\t#2,R0\n\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (2: SP moved)", r0)
	}

	want := "3 jackets, ok\n%SYSTEM-S-NORMAL, normal successful completion\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
