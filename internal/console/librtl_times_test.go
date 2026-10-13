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

// TestLibrtlTimeConversion: a program turns $NUMTIM's vector for
// 13-JAN-1988 09:13:25.14 into a time with LIB$CVT_VECTIM and writes it,
// then makes a delta of 6 hours with LIB$CVT_TO_INTERNAL_TIME, takes it
// to F-floating days and back (LIB$CVTF_FROM_ and LIB$CVTF_TO_), and
// writes that (a quarter day is exact in F_floating; 5 hours, 5/24 of a
// day, would come back a fraction of a millisecond short); it ends with the date's hour of the year (298) in R0, from
// LIB$CVT_FROM_INTERNAL_TIME (docs/PHASE-56, subtask 3). The operation
// codes are LIB$K_HOUR_OF_YEAR (3), LIB$K_DELTA_HOURS (23), and
// LIB$K_DELTA_DAYS_F (27).
func TestLibrtlTimeConversion(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"VECTOR:\t.WORD\t1988,1,13,9,13,25,14\n"+
		"DATE:\t.QUAD\t0\n"+
		"DELTA:\t.QUAD\t0\n"+
		"HOY:\t.LONG\t3\n"+
		"HOURS:\t.LONG\t23\n"+
		"DAYSF:\t.LONG\t27\n"+
		"SIX:\t.LONG\t6\n"+
		"DAYS:\t.F_FLOATING\t0\n"+
		"RESULT:\t.LONG\t0\n"+
		"TEXT:\t.ASCID\t/......................./\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tDATE\n\tPUSHAW\tVECTOR\n"+
		"\tCALLS\t#2,G^LIB$CVT_VECTIM\n\tBLBC\tR0,DONE\n"+
		"\tMOVZWL\t#23,TEXT\n"+
		"\tPUSHAQ\tDATE\n\tPUSHAQ\tTEXT\n\tBSBW\tSHOW\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAL\tSIX\n\tPUSHAL\tHOURS\n"+
		"\tCALLS\t#3,G^LIB$CVT_TO_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAF\tDAYS\n\tPUSHAL\tDAYSF\n"+
		"\tCALLS\t#3,G^LIB$CVTF_FROM_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		// DONE is mid-way, so every BLBC reaches it.
		"\tBRB\tGO\nDONE:\tRET\nGO:"+
		"\tCLRQ\tDELTA\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAF\tDAYS\n\tPUSHAL\tDAYSF\n"+
		"\tCALLS\t#3,G^LIB$CVTF_TO_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		"\tMOVZWL\t#16,TEXT\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAQ\tTEXT\n\tBSBW\tSHOW\n"+
		"\tPUSHAQ\tDATE\n\tPUSHAL\tRESULT\n\tPUSHAL\tHOY\n"+
		"\tCALLS\t#3,G^LIB$CVT_FROM_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		"\tMOVL\tRESULT,R0\n\tRET\n"+
		// SHOW: $ASCTIM of the time pushed into the descriptor pushed,
		// then LIB$PUT_OUTPUT of it.
		"SHOW:\tMOVL\t(SP)+,R2\n\tMOVQ\t(SP)+,R3\n"+
		"\tCLRL\t-(SP)\n\tPUSHL\tR4\n\tPUSHL\tR3\n\tCLRL\t-(SP)\n"+
		"\tCALLS\t#4,G^SYS$ASCTIM\n"+
		"\tPUSHL\tR3\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tJMP\t(R2)\n"+
		"\t.END\tSTART\n")

	if r0 != 298 {
		t.Errorf("R0 = %#x, want 298", r0)
	}

	if want := "13-JAN-1988 09:13:25.14\n   0 06:00:00.00\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
