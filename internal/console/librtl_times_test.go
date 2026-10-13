package console

import "testing"

// TestLibrtlTimeArithmetic: a program works a delta time through all four
// arithmetic routines by their stubs, then writes the result with
// $ASCTIM and LIB$PUT_OUTPUT (docs/PHASE-56, subtask 2). One hour times 3
// is 3 hours; plus an hour, 4; times 1.5, 6; minus an hour, 5. Each
// routine returns LIB$_NORMAL, whose low bit the program tests.
func TestLibrtlTimeArithmetic(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"HOUR:\t.LONG\t^X9E3B9800,^XFFFFFFF7\n"+ // a delta of one hour
		"WORK:\t.LONG\t^X9E3B9800,^XFFFFFFF7\n"+
		"SUM:\t.QUAD\t0\n"+
		"DIFF:\t.QUAD\t0\n"+
		"THREE:\t.LONG\t3\n"+
		"HALF:\t.F_FLOATING\t1.5\n"+
		"TEXT:\t.ASCID\t/................/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tWORK\n\tPUSHAL\tTHREE\n"+
		"\tCALLS\t#2,G^LIB$MULT_DELTA_TIME\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tSUM\n\tPUSHAQ\tHOUR\n\tPUSHAQ\tWORK\n"+
		"\tCALLS\t#3,G^LIB$ADD_TIMES\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tSUM\n\tPUSHAF\tHALF\n"+
		"\tCALLS\t#2,G^LIB$MULTF_DELTA_TIME\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tDIFF\n\tPUSHAQ\tHOUR\n\tPUSHAQ\tSUM\n"+
		"\tCALLS\t#3,G^LIB$SUB_TIMES\n\tBLBC\tR0,DONE\n"+
		"\tCLRL\t-(SP)\n\tPUSHAQ\tDIFF\n\tPUSHAQ\tTEXT\n\tCLRL\t-(SP)\n"+
		"\tCALLS\t#4,G^SYS$ASCTIM\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tTEXT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#1,R0\nDONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	if want := "   0 05:00:00.00\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
