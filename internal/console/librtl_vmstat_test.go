package console

import "testing"

// TestLibrtlVMStatistics: a linked program allocates 100 and 20 bytes
// with LIB$GET_VM and frees the 20, then reads the bytes still allocated
// with LIB$STAT_VM (code 3), shows the heap's line with LIB$SHOW_VM, and
// shows code 1's line through an action routine, which writes it and
// returns its user argument, SHOW_VM's status then (docs/PHASE-59,
// subtask 7). The program prints the figure and both statuses.
func TestLibrtlVMStatistics(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"BIG:\t.LONG\t100\nSMALL:\t.LONG\t20\nTHREE:\t.LONG\t3\nONE:\t.LONG\t1\n"+
		"A:\t.LONG\t0\nB:\t.LONG\t0\nBYTES:\t.LONG\t0\nS:\t.BLKL\t2\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"CTRL:\t.ASCID\t/!UL !XL !XL/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tACTION,^M<>\n"+
		"\tPUSHL\t4(AP)\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n\tMOVL\t8(AP),R0\n\tRET\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAL\tA\n\tPUSHAL\tBIG\n\tCALLS\t#2,G^LIB$GET_VM\n\tBLBS\tR0,41$\n\tRET\n41$:"+
		"\tPUSHAL\tB\n\tPUSHAL\tSMALL\n\tCALLS\t#2,G^LIB$GET_VM\n\tBLBS\tR0,42$\n\tRET\n42$:"+
		"\tPUSHAL\tB\n\tPUSHAL\tSMALL\n\tCALLS\t#2,G^LIB$FREE_VM\n\tBLBS\tR0,43$\n\tRET\n43$:"+
		"\tPUSHAL\tBYTES\n\tPUSHAL\tTHREE\n\tCALLS\t#2,G^LIB$STAT_VM\n\tBLBS\tR0,44$\n\tRET\n44$:"+
		"\tCALLS\t#0,G^LIB$SHOW_VM\n\tMOVL\tR0,S\n"+
		"\tPUSHL\t#^X12345671\n\tPUSHAL\tACTION\n\tPUSHAL\tONE\n\tCALLS\t#3,G^LIB$SHOW_VM\n\tMOVL\tR0,S+4\n"+
		"\tPUSHL\tS+4\n\tPUSHL\tS\n\tPUSHL\tBYTES\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#6,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"DONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	const want = " 2 calls to LIB$GET_VM, 1 calls to LIB$FREE_VM, 100 bytes still allocated\n" +
		" 2 calls to LIB$GET_VM\n" +
		"100 00000001 12345671\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
