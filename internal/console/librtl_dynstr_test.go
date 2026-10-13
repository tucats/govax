package console

import "testing"

// TestLibrtlDynamicStrings: a linked program gets dynamic strings from
// LIB$SGET1_DD (CALL) and LIB$SGET1_DD_R6 (JSB), writes text into them,
// shows them with LIB$PUT_OUTPUT, and frees both with LIB$SFREEN_DD6.
// The program ends with 1 when every status was a success and both
// pointers are 0 again.
func TestLibrtlDynamicStrings(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"DESC:\t.LONG\t0,0\nD2:\t.LONG\t0,0\nLEN:\t.WORD\t5\n"+
		"HELLO:\t.ASCII\t/HELLO/\nBYE:\t.ASCII\t/BYE/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2,R3,R4,R5,R6>\n"+
		"\tPUSHAQ\tDESC\n\tPUSHAW\tLEN\n\tCALLS\t#2,G^LIB$SGET1_DD\n\tBLBC\tR0,10$\n"+
		"\tMOVC3\t#5,HELLO,@DESC+4\n"+
		"\tPUSHAQ\tDESC\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#3,R0\n\tMOVAQ\tD2,R1\n\tJSB\tG^LIB$SGET1_DD_R6\n\tBLBC\tR0,10$\n"+
		"\tMOVC3\t#3,BYE,@D2+4\n"+
		"\tPUSHAQ\tD2\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#2,R0\n\tMOVAQ\tDESC,R1\n\tJSB\tG^LIB$SFREEN_DD6\n\tBLBC\tR0,10$\n"+
		"\tTSTL\tDESC+4\n\tBNEQ\t20$\n\tTSTL\tD2+4\n\tBNEQ\t20$\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (20: a pointer wasn't cleared; otherwise the failing status)", r0)
	}

	if out != "HELLO\nBYE\n" {
		t.Errorf("output %q, want %q", out, "HELLO\nBYE\n")
	}
}

// TestLibrtlCopyAndAnalyze: a linked program copies a string descriptor
// into a dynamic string with LIB$SCOPY_DXDX, copies text by reference
// over it with LIB$SCOPY_R_DX6 (JSB), showing each, and analyzes the
// source with LIB$ANALYZE_SDESC_R2, which returns its length and text's
// address in R1 and R2. It ends with 1 when all of that held.
func TestLibrtlCopyAndAnalyze(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"SRC:\t.ASCID\t/GOVAX/\nDYN:\t.LONG\t^X020E0000,0\nTEXT:\t.ASCII\t/VMS/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2,R3,R4,R5,R6>\n"+
		"\tPUSHAQ\tDYN\n\tPUSHAQ\tSRC\n\tCALLS\t#2,G^LIB$SCOPY_DXDX\n\tBLBC\tR0,10$\n"+
		"\tPUSHAQ\tDYN\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#3,R0\n\tMOVAB\tTEXT,R1\n\tMOVAQ\tDYN,R2\n\tJSB\tG^LIB$SCOPY_R_DX6\n\tBLBC\tR0,10$\n"+
		"\tPUSHAQ\tDYN\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVAQ\tSRC,R0\n\tJSB\tG^LIB$ANALYZE_SDESC_R2\n\tBLBC\tR0,10$\n"+
		"\tCMPL\tR1,#5\n\tBNEQ\t20$\n\tCMPL\tR2,SRC+4\n\tBNEQ\t20$\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (20: ANALYZE_SDESC_R2's R1 or R2; otherwise the failing status)", r0)
	}

	if out != "GOVAX\nVMS\n" {
		t.Errorf("output %q, want %q", out, "GOVAX\nVMS\n")
	}
}

// TestLibrtlStringFunctions: a linked program calls LIB$LEN, LIB$ICHAR,
// LIB$INDEX (the manual's "DAY" in "ONE DAY AT A TIME"), and LIB$CHAR,
// showing the character it built. It ends with 1 when each result was
// right, or the number of the check that failed.
func TestLibrtlStringFunctions(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"SRC:\t.ASCID\t/ONE DAY AT A TIME/\nSUB:\t.ASCID\t/DAY/\n"+
		"CODE:\t.BYTE\t^A/*/\nONE:\t.ASCID\t/?/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tSRC\n\tCALLS\t#1,G^LIB$LEN\n\tCMPL\tR0,#17\n\tBNEQ\t10$\n"+
		"\tPUSHAQ\tSUB\n\tCALLS\t#1,G^LIB$ICHAR\n\tCMPL\tR0,#^A/D/\n\tBNEQ\t20$\n"+
		"\tPUSHAQ\tSUB\n\tPUSHAQ\tSRC\n\tCALLS\t#2,G^LIB$INDEX\n\tCMPL\tR0,#5\n\tBNEQ\t30$\n"+
		"\tPUSHAB\tCODE\n\tPUSHAQ\tONE\n\tCALLS\t#2,G^LIB$CHAR\n\tBLBC\tR0,40$\n"+
		"\tPUSHAQ\tONE\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tMOVL\t#10,R0\n\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n30$:\tMOVL\t#30,R0\n\tRET\n40$:\tMOVL\t#40,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (10 LEN, 20 ICHAR, 30 INDEX, 40 CHAR failed)", r0)
	}

	if out != "*\n" {
		t.Errorf("output %q, want %q", out, "*\n")
	}
}

// TestLibrtlCvtTextToBinary: a linked program converts decimal,
// hexadecimal, and octal text, the count by value and the text by
// reference, as MACRO calls them. It ends with 1 when each gave its
// value and a blank made LIB$CVT_DTB fail with 0.
func TestLibrtlCvtTextToBinary(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"DEC:\t.ASCII\t/-1234/\nHEX:\t.ASCII\t/7fF/\nOCT:\t.ASCII\t/777/\nBLANK:\t.ASCII\t/1 2/\nRES:\t.LONG\t0\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAL\tRES\n\tPUSHAB\tDEC\n\tPUSHL\t#5\n\tCALLS\t#3,G^LIB$CVT_DTB\n\tBLBC\tR0,10$\n\tCMPL\tRES,#-1234\n\tBNEQ\t10$\n"+
		"\tPUSHAL\tRES\n\tPUSHAB\tHEX\n\tPUSHL\t#3\n\tCALLS\t#3,G^LIB$CVT_HTB\n\tBLBC\tR0,20$\n\tCMPL\tRES,#^X7FF\n\tBNEQ\t20$\n"+
		"\tPUSHAL\tRES\n\tPUSHAB\tOCT\n\tPUSHL\t#3\n\tCALLS\t#3,G^LIB$CVT_OTB\n\tBLBC\tR0,30$\n\tCMPL\tRES,#^O777\n\tBNEQ\t30$\n"+
		"\tPUSHAL\tRES\n\tPUSHAB\tBLANK\n\tPUSHL\t#3\n\tCALLS\t#3,G^LIB$CVT_DTB\n\tTSTL\tR0\n\tBNEQ\t40$\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tMOVL\t#10,R0\n\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n30$:\tMOVL\t#30,R0\n\tRET\n40$:\tMOVL\t#40,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (10 DTB, 20 HTB, 30 OTB, 40 the blank failed)", r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}

// TestLibrtlLookupKey: the manual's MACRO example, a table built with
// govax's $LIB_KEY_TABLE macro (ADD 1, DELETE 2, EXIT 3), and
// LIB$LOOKUP_KEY of "DEL" from a linked program, which shows the full
// keyword it found and checks its value; then "XYZ", which is
// LIB$_UNRKEY. It ends with 1 when all of that held.
func TestLibrtlLookupKey(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"KEYTABLE:\t$LIB_KEY_TABLE\t<<ADD,1>,<DELETE,2>,<EXIT,3>>\n"+
		"SEARCH:\t.ASCID\t/DEL/\nBAD:\t.ASCID\t/XYZ/\nFULL:\t.LONG\t^X020E0000,0\nVALUE:\t.LONG\t0\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tCMPL\tKEYTABLE,#6\n\tBNEQ\t20$\n"+
		"\tPUSHAQ\tFULL\n\tPUSHAL\tVALUE\n\tPUSHAL\tKEYTABLE\n\tPUSHAQ\tSEARCH\n\tCALLS\t#4,G^LIB$LOOKUP_KEY\n\tBLBC\tR0,10$\n"+
		"\tCMPL\tVALUE,#2\n\tBNEQ\t30$\n"+
		"\tPUSHAQ\tFULL\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAL\tKEYTABLE\n\tPUSHAQ\tBAD\n\tCALLS\t#2,G^LIB$LOOKUP_KEY\n\tCMPL\tR0,#LIB$_UNRKEY\n\tBNEQ\t40$\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n30$:\tMOVL\t#30,R0\n\tRET\n40$:\tMOVL\t#40,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (20: the table's count, 30: the value, 40: no LIB$_UNRKEY; otherwise the failing status)", r0)
	}

	if out != "DELETE\n" {
		t.Errorf("output %q, want %q", out, "DELETE\n")
	}
}

// TestLibrtlLocale: a linked program shows LIB$DIGIT_SEP's, LIB$RADIX_POINT's,
// and LIB$CURRENCY's symbols (no logical names defined: the United
// States'), and ends with LIB$LP_LINES's 66.
func TestLibrtlLocale(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"SYM:\t.LONG\t^X020E0000,0\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tSYM\n\tCALLS\t#1,G^LIB$DIGIT_SEP\n\tPUSHAQ\tSYM\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAQ\tSYM\n\tCALLS\t#1,G^LIB$RADIX_POINT\n\tPUSHAQ\tSYM\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAQ\tSYM\n\tCALLS\t#1,G^LIB$CURRENCY\n\tPUSHAQ\tSYM\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tCALLS\t#0,G^LIB$LP_LINES\n\tRET\n\t.END\tSTART\n")

	if r0 != 66 {
		t.Errorf("R0 = %d, want LIB$LP_LINES's 66", r0)
	}

	if out != ",\n.\n$\n" {
		t.Errorf("output %q, want %q", out, ",\n.\n$\n")
	}
}

// TestLibrtlCvtDxDx: LIB$CVT_DX_DX, in LIBRTL2.EXE, from a linked
// program: a longword scaled by 10^-2 to text (shown), an F_floating
// value to text in E format (shown), and text to a longword. It ends
// with 1 when the last gave -77.
func TestLibrtlCvtDxDx(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"LDESC:\t.WORD\t4\n\t.BYTE\t8,9\n\t.ADDRESS\tLVAL\n\t.BYTE\t-2,0,0,0\n"+ // DSC$K_DTYPE_L, DSC$K_CLASS_SD, scale -2
		"LVAL:\t.LONG\t123456\n"+
		"FDESC:\t.WORD\t4\n\t.BYTE\t10,1\n\t.ADDRESS\tFVAL\n"+ // DSC$K_DTYPE_F, DSC$K_CLASS_S
		"FVAL:\t.F_FLOATING\t-0.25\n"+
		"TEXT:\t.ASCID\t/ -77 /\n"+
		"RDESC:\t.WORD\t4\n\t.BYTE\t8,1\n\t.ADDRESS\tRVAL\nRVAL:\t.LONG\t0\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tOUT\n\tPUSHAQ\tLDESC\n\tCALLS\t#2,G^LIB$CVT_DX_DX\n\tBLBC\tR0,10$\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAQ\tOUT\n\tPUSHAQ\tFDESC\n\tCALLS\t#2,G^LIB$CVT_DX_DX\n\tBLBC\tR0,10$\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAQ\tRDESC\n\tPUSHAQ\tTEXT\n\tCALLS\t#2,G^LIB$CVT_DX_DX\n\tBLBC\tR0,10$\n"+
		"\tCMPL\tRVAL,#-77\n\tBNEQ\t20$\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"10$:\tRET\n20$:\tMOVL\t#20,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (20: the text's value; otherwise the failing status)", r0)
	}

	if want := "1234.56\n-0.2500000E+00\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
