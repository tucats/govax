package console

import "testing"

// TestLibrtlVMMalloc: a linked program calls the C library's heap
// routines with C's conventions (arguments by value, the result in R0):
// it fills a LIB$VM_MALLOC block, grows it with LIB$VM_REALLOC (which
// moves it with its bytes), gets a LIB$VM_CALLOC array of zeros, and
// frees both, a second free answering 0 where the first answers 1
// (docs/PHASE-59, subtasks 8 and 10).
func TestLibrtlVMMalloc(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"A:\t.LONG\t0\nB:\t.LONG\t0\nC:\t.LONG\t0\nF:\t.LONG\t0\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"CTRL:\t.ASCID\t/!AF !UL !UL !SL/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2>\n"+
		"\tPUSHL\t#8\n\tCALLS\t#1,G^LIB$VM_MALLOC\n\tMOVL\tR0,A\n\tBNEQ\t1$\n\tRET\n1$:"+
		"\tMOVQ\tTEXT,@A\n"+
		"\tPUSHL\t#200\n\tPUSHL\tA\n\tCALLS\t#2,G^LIB$VM_REALLOC\n\tMOVL\tR0,B\n\tBNEQ\t2$\n\tRET\n2$:"+
		"\tPUSHL\t#4\n\tPUSHL\t#10\n\tCALLS\t#2,G^LIB$VM_CALLOC\n\tMOVL\tR0,C\n\tBNEQ\t3$\n\tRET\n3$:"+
		"\tCLRL\tR2\n\tMOVL\tC,R1\n\tMOVL\t#10,R0\n4$:\tADDL2\t(R1)+,R2\n\tSOBGTR\tR0,4$\n"+
		"\tPUSHL\tB\n\tCALLS\t#1,G^LIB$VM_FREE\n"+
		"\tPUSHL\tB\n\tCALLS\t#1,G^LIB$VM_FREE\n\tMOVL\tR0,F\n"+
		"\tPUSHL\tC\n\tCALLS\t#1,G^LIB$VM_FREE\n"+
		"\tSUBL3\tA,B,R0\n\tCMPL\tR0,#0\n\tBEQL\t5$\n\tMOVL\t#1,R0\n5$:"+
		"\tPUSHL\tF\n\tPUSHL\tR2\n\tPUSHL\tR0\n\tPUSHL\tB\n\tPUSHL\t#8\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#8,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n\tMOVL\t#1,R0\n"+
		"DONE:\tRET\n"+
		"TEXT:\t.ASCII\t/GOVAXVMS/\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	// The moved block's text, that it moved, the array's sum, and the
	// second free's 0.
	const want = "GOVAXVMS 1 0 0\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
