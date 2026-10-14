package console

import "testing"

// TestLibrtlVMPages: a linked program gets three pages with
// LIB$GET_VM_PAGE, checks they're page-aligned, writes the first and last
// longword of each (user mode, so they're user-writable P0 pages), frees
// the middle one and then the other two, and frees the middle one again,
// which is LIB$_BADBLOADR. It ends with 1 when all of that held, or the
// number of the check that failed.
func TestLibrtlVMPages(t *testing.T) {
	r0, _ := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"COUNT:\t.LONG\t3\nONE:\t.LONG\t1\nADDR:\t.LONG\t0\nMID:\t.LONG\t0\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2>\n"+
		"\tPUSHAL\tADDR\n\tPUSHAL\tCOUNT\n\tCALLS\t#2,G^LIB$GET_VM_PAGE\n\tBLBS\tR0,41$\n\tRET\n41$:"+
		"\tMOVL\tADDR,R2\n\tMOVL\t#10,R0\n\tBITL\t#^X1FF,R2\n\tBEQL\t51$\n\tRET\n51$:"+
		"\tMOVL\t#^X11111111,(R2)\n\tMOVL\t#^X22222222,^X5FC(R2)\n"+
		"\tMOVL\t#20,R0\n\tCMPL\t(R2),#^X11111111\n\tBEQL\t52$\n\tRET\n52$:\tCMPL\t^X5FC(R2),#^X22222222\n\tBEQL\t53$\n\tRET\n53$:"+
		"\tADDL3\t#^X200,R2,MID\n"+
		"\tPUSHAL\tMID\n\tPUSHAL\tONE\n\tCALLS\t#2,G^LIB$FREE_VM_PAGE\n\tBLBS\tR0,42$\n\tRET\n42$:"+
		"\tPUSHAL\tADDR\n\tPUSHAL\tONE\n\tCALLS\t#2,G^LIB$FREE_VM_PAGE\n\tBLBS\tR0,43$\n\tRET\n43$:"+
		"\tADDL3\t#^X400,R2,ADDR\n"+
		"\tPUSHAL\tADDR\n\tPUSHAL\tONE\n\tCALLS\t#2,G^LIB$FREE_VM_PAGE\n\tBLBS\tR0,44$\n\tRET\n44$:"+
		"\tPUSHAL\tMID\n\tPUSHAL\tONE\n\tCALLS\t#2,G^LIB$FREE_VM_PAGE\n"+
		"\tCMPL\tR0,#^X158264\n\tBEQL\t54$\n\tMOVL\t#30,R0\n\tRET\n54$:"+
		"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (10: not page-aligned, 20: a write didn't hold, 30: the double free; otherwise the failing status)", r0)
	}
}
