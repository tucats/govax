package console

import "testing"

// TestLibrtlVMZones: a linked program makes a fixed-size zone of 24-byte
// blocks that fills new blocks with FF (LIB$CREATE_VM_ZONE), gets a
// 10-byte block and checks its fill, asks for 30 bytes (LIB$_BADBLOSIZ),
// frees the block, resets the zone, deletes it, and deletes it again
// (LIB$_BADBLOADR). It ends with 1 when all of that held, or the number
// of the check that failed, or the failing status.
func TestLibrtlVMZones(t *testing.T) {
	r0, _ := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"ZONE:\t.LONG\t0\nALG:\t.LONG\t4\nSIZE24:\t.LONG\t24\nFLAGS:\t.LONG\t4\n"+
		"TEN:\t.LONG\t10\nTHIRTY:\t.LONG\t30\nADDR:\t.LONG\t0\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAL\tFLAGS\n\tPUSHAL\tSIZE24\n\tPUSHAL\tALG\n\tPUSHAL\tZONE\n"+
		"\tCALLS\t#4,G^LIB$CREATE_VM_ZONE\n\tBLBS\tR0,1$\n\tRET\n"+
		"1$:\tPUSHAL\tZONE\n\tPUSHAL\tADDR\n\tPUSHAL\tTEN\n\tCALLS\t#3,G^LIB$GET_VM\n\tBLBS\tR0,2$\n\tRET\n"+
		"2$:\tMOVL\tADDR,R1\n\tMOVL\t#10,R0\n\tCMPL\t(R1),#-1\n\tBEQL\t3$\n\tRET\n"+
		"3$:\tPUSHAL\tZONE\n\tPUSHAL\tADDR\n\tPUSHAL\tTHIRTY\n\tCALLS\t#3,G^LIB$GET_VM\n"+
		"\tCMPL\tR0,#^X15826C\n\tBEQL\t4$\n\tMOVL\t#20,R0\n\tRET\n"+
		"4$:\tPUSHAL\tZONE\n\tPUSHAL\tADDR\n\tPUSHAL\tTEN\n\tCALLS\t#3,G^LIB$FREE_VM\n\tBLBS\tR0,5$\n\tRET\n"+
		"5$:\tPUSHAL\tZONE\n\tCALLS\t#1,G^LIB$RESET_VM_ZONE\n\tBLBS\tR0,6$\n\tRET\n"+
		"6$:\tPUSHAL\tZONE\n\tCALLS\t#1,G^LIB$DELETE_VM_ZONE\n\tBLBS\tR0,7$\n\tRET\n"+
		"7$:\tPUSHAL\tZONE\n\tCALLS\t#1,G^LIB$DELETE_VM_ZONE\n"+
		"\tCMPL\tR0,#^X158264\n\tBEQL\t8$\n\tMOVL\t#30,R0\n\tRET\n"+
		"8$:\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (10: no fill, 20: 30 bytes allowed, 30: the second delete; otherwise the failing status)", r0)
	}
}
