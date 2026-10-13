package console

import "testing"

// TestLibrtlProtection: a program formats a protection mask with
// LIB$FORMAT_SOGW_PROT, parses a protection string with
// LIB$PARSE_SOGW_PROT and an access string with LIB$PARSE_ACCESS_CODE,
// and shows the masks and end positions with LIB$SYS_FAO (docs/PHASE-55,
// subtask 7).
func TestLibrtlProtection(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"CTRL:\t.ASCID\t/!XL !XW !XW !UW !XL !XW !UW/\n"+
		"MASK:\t.WORD\t^XEC00\n"+
		"PSTR:\t.ASCID\t/GROUP=RW, w/\n"+
		"ASTR:\t.ASCID\t/RWE/\n"+
		"GRP:\t.WORD\t^X0F00\n"+
		"PROT:\t.WORD\t0\nOWN:\t.WORD\t0\nPEND:\t.WORD\t0\n"+
		"AMASK:\t.WORD\t0\nAEND:\t.WORD\t0\n"+
		"S:\t.BLKL\t2\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHL\t#0\n\tPUSHAQ\tOUT\n\tCLRQ\t-(SP)\n\tCLRQ\t-(SP)\n\tPUSHAW\tMASK\n"+
		"\tCALLS\t#7,G^LIB$FORMAT_SOGW_PROT\n\tBLBS\tR0,OK\n\tRET\n"+
		"OK:\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAW\tPEND\n\tPUSHAW\tOWN\n\tPUSHAW\tPROT\n\tPUSHL\t#0\n\tPUSHAQ\tPSTR\n"+
		"\tCALLS\t#5,G^LIB$PARSE_SOGW_PROT\n\tMOVL\tR0,S\n"+
		"\tPUSHAW\tAEND\n\tPUSHAW\tAMASK\n\tPUSHAW\tGRP\n\tPUSHL\t#0\n\tPUSHAQ\tASTR\n"+
		"\tCALLS\t#5,G^LIB$PARSE_ACCESS_CODE\n\tMOVL\tR0,S+4\n"+
		"\tMOVZWL\tAEND,-(SP)\n\tMOVZWL\tAMASK,-(SP)\n\tPUSHL\tS+4\n"+
		"\tMOVZWL\tPEND,-(SP)\n\tMOVZWL\tOWN,-(SP)\n\tMOVZWL\tPROT,-(SP)\n\tPUSHL\tS\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#10,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"DONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	want := "System: RWED, Owner: RWED, Group: RW, World: R\n" +
		"00000001 FC00 FF00 11 00000001 0700 3\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
