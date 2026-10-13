package console

import "testing"

// TestLibrtlResources: a program allocates event flags and LUNs through
// the routines' stubs and shows what each call wrote and returned with
// LIB$SYS_FAO (docs/PHASE-55, subtask 4): GET_EF gives 63 first, flag 1
// starts reserved (LIB$_EF_ALRRES), and once FREE_EF frees it GET_EF
// still gives the highest free flag, 62; GET_LUN gives 119, which
// FREE_LUN frees once (LIB$_LUNALRFRE the second time). VMS 7.1's order
// (testdata/lib55).
func TestLibrtlResources(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"CTRL:\t.ASCID\t/!SL !XL !XL !XL !SL !XL !SL !XL !XL !XL/\n"+
		"ONE:\t.LONG\t1\n"+
		"EF:\t.LONG\t0\nEF2:\t.LONG\t0\nLUN:\t.LONG\t0\n"+
		"S:\t.BLKL\t7\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAL\tEF\n\tCALLS\t#1,G^LIB$GET_EF\n\tMOVL\tR0,S\n"+
		"\tPUSHAL\tONE\n\tCALLS\t#1,G^LIB$RESERVE_EF\n\tMOVL\tR0,S+4\n"+
		"\tPUSHAL\tONE\n\tCALLS\t#1,G^LIB$FREE_EF\n\tMOVL\tR0,S+8\n"+
		"\tPUSHAL\tEF2\n\tCALLS\t#1,G^LIB$GET_EF\n\tMOVL\tR0,S+12\n"+
		"\tPUSHAL\tLUN\n\tCALLS\t#1,G^LIB$GET_LUN\n\tMOVL\tR0,S+16\n"+
		"\tPUSHAL\tLUN\n\tCALLS\t#1,G^LIB$FREE_LUN\n\tMOVL\tR0,S+20\n"+
		"\tPUSHAL\tLUN\n\tCALLS\t#1,G^LIB$FREE_LUN\n\tMOVL\tR0,S+24\n"+
		"\tPUSHL\tS+24\n\tPUSHL\tS+20\n\tPUSHL\tS+16\n\tPUSHL\tLUN\n"+
		"\tPUSHL\tS+12\n\tPUSHL\tEF2\n\tPUSHL\tS+8\n\tPUSHL\tS+4\n\tPUSHL\tS\n\tPUSHL\tEF\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#13,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"DONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	// SS$_NORMAL, LIB$_EF_ALRRES, LIB$_LUNALRFRE.
	want := "63 00000001 001582A4 00000001 62 00000001 119 00000001 00000001 001582BC\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
