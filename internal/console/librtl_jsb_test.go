package console

import "testing"

// jsbProgram calls OTS$MOVE3_R5, a JSB entry govax lacks, with R6 and SP
// noted first, and ends with the JSB's R0 if R6 and SP came back as they
// were, or 4 if not. handler, if it isn't empty, is a condition handler
// established for START.
func jsbProgram(handler string) string {
	establish := ""
	if handler != "" {
		establish = "\tMOVAB\tHANDLER,(FP)\n"
	}

	return "\t.PSECT\tCODE,NOWRT,EXE\n" + handler +
		"\t.ENTRY\tSTART,^M<R6,R7>\n" + establish +
		"\tMOVL\t#5,R6\n\tMOVL\tSP,R7\n" +
		"\tJSB\tG^OTS$MOVE3_R5\n" +
		"\tCMPL\tSP,R7\n\tBNEQ\t10$\n\tCMPL\tR6,#5\n\tBNEQ\t10$\n\tRET\n" +
		"10$:\tMOVL\t#4,R0\n\tRET\n\t.END\tSTART\n"
}

// TestLibrtlMissingJSBEntry: an image that JSBs to a JSB entry govax
// lacks links and runs; the call signals SS$_UNSUPPORTED with the
// routine's name, raised in the caller's frame, which the catch-all
// shows; the condition being an error, the JSB returns, its R0
// SS$_UNSUPPORTED, with the stack and the other registers as they were.
func TestLibrtlMissingJSBEntry(t *testing.T) {
	r0, out := runMacroProgram(t, jsbProgram(""))

	if r0 != 0xE4A { // SS$_UNSUPPORTED
		t.Errorf("R0 = %#x, want SS$_UNSUPPORTED (0xE4A)", r0)
	}

	want := "%SYSTEM-E-UNSUPPORTED, unsupported operation or function\n" +
		"-GOVAX-E-NOTIMPL, routine OTS$MOVE3_R5 is not implemented in govax\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestLibrtlMissingJSBEntryContinued: the caller's own handler sees the
// condition (a JSB routine has no frame, so the search starts at its
// caller's), and continuing it returns from the JSB.
func TestLibrtlMissingJSBEntryContinued(t *testing.T) {
	r0, out := runMacroProgram(t, jsbProgram("\t.ENTRY\tHANDLER,^M<>\n\tMOVL\t#1,R0\n\tRET\n")) // SS$_CONTINUE

	if r0 != 0xE4A {
		t.Errorf("R0 = %#x, want SS$_UNSUPPORTED (0xE4A)", r0)
	}

	if out != "" {
		t.Errorf("output %q, want none: the handler took the condition", out)
	}
}

// TestLibrtlDataTables: LIBRTL's tables are addressable by name. MOVTC
// through LIB$AB_UPCASE uppercases, and through LIB$AB_ASC_EBC and back
// through LIB$AB_EBC_ASC gives the EBCDIC the manual's table shows and
// the text again. The program ends with 1 when all three match.
func TestLibrtlDataTables(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"SRC:\t.ASCII\t/Az09a{/\nUP:\t.ASCII\t/AZ09A{/\n"+
		"EBC:\t.BYTE\t^XC1,^XA9,^XF0,^XF9,^X81,^XC0\n"+
		"DST:\t.BLKB\t6\nBACK:\t.BLKB\t6\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2,R3,R4,R5>\n"+
		"\tMOVTC\t#6,SRC,#^A/ /,G^LIB$AB_UPCASE,#6,DST\n"+
		"\tCMPC3\t#6,DST,UP\n\tBNEQ\t10$\n"+
		"\tMOVTC\t#6,SRC,#^A/ /,G^LIB$AB_ASC_EBC,#6,DST\n"+
		"\tCMPC3\t#6,DST,EBC\n\tBNEQ\t20$\n"+
		"\tMOVTC\t#6,DST,#^A/ /,G^LIB$AB_EBC_ASC,#6,BACK\n"+
		"\tCMPC3\t#6,BACK,SRC\n\tBNEQ\t30$\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tMOVL\t#10,R0\n\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n30$:\tMOVL\t#30,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (10: upcase, 20: to EBCDIC, 30: back to ASCII failed)", r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}
