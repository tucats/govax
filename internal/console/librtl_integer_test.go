package console

import (
	"strings"
	"testing"
)

// The integer routines (docs/PHASE-53, subtask 7), called from a linked
// program as a program calls them.

// integerData is the data the integer programs share: the manual's
// examples' operands, and room for results.
const integerData = "\t.PSECT\tDATA,NOEXE,WRT,LONG\n" +
	"ADDEND:\t.LONG\t1,^X10000000\nAUGEND:\t.LONG\t^XFFFFFFFF,0\n" +
	"RESULT:\t.LONG\t0,0\nTWO:\t.LONG\t2\n" +
	"MULR:\t.LONG\t^X10000000\nMULD:\t.LONG\t^X1000\nZERO:\t.LONG\t0\n" +
	"DIVR:\t.LONG\t4096\nDIVD:\t.LONG\t^X12345678,1\nQUO:\t.LONG\t0\nREM:\t.LONG\t0\n" +
	"ADD:\t.WORD\t1\nSUM:\t.WORD\t^X7FFF\nSIGN:\t.WORD\t0\n"

// TestLibrtlIntegers: LIB$ADDX, LIB$SUBX, LIB$EMUL, LIB$EDIV, and
// LIB$ADAWI through their stubs. Each check that fails returns its own
// number; all passing returns 1.
func TestLibrtlIntegers(t *testing.T) {
	r0, out := runMacroProgram(t, integerData+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		// LIB$ADDX: 10000000 00000001 + 00000000 FFFFFFFF.
		"\tPUSHAL\tRESULT\n\tPUSHAL\tAUGEND\n\tPUSHAL\tADDEND\n"+
		"\tCALLS\t#3,G^LIB$ADDX\n"+check(2, "R0,#1")+
		check(3, "RESULT,#0")+check(4, "RESULT+4,#^X10000001")+
		// LIB$SUBX, with the length given: 0FFFFFFF 00000002.
		"\tPUSHAL\tTWO\n"+
		"\tPUSHAL\tRESULT\n\tPUSHAL\tAUGEND\n\tPUSHAL\tADDEND\n"+
		"\tCALLS\t#4,G^LIB$SUBX\n"+check(5, "R0,#1")+
		check(6, "RESULT,#2")+check(7, "RESULT+4,#^X0FFFFFFF")+
		// LIB$EMUL: 2**28 * 2**12 is 00000100 00000000.
		"\tPUSHAL\tRESULT\n\tPUSHAL\tZERO\n\tPUSHAL\tMULD\n\tPUSHAL\tMULR\n"+
		"\tCALLS\t#4,G^LIB$EMUL\n"+check(8, "R0,#1")+
		check(9, "RESULT,#0")+check(10, "RESULT+4,#^X100")+
		// LIB$EDIV: 4,600,387,192 / 4096 is 1,123,141 remainder 1656.
		"\tPUSHAL\tREM\n\tPUSHAL\tQUO\n\tPUSHAQ\tDIVD\n\tPUSHAL\tDIVR\n"+
		"\tCALLS\t#4,G^LIB$EDIV\n"+check(11, "R0,#1")+
		check(12, "QUO,#1123141")+check(13, "REM,#1656")+
		// ... and by zero: SS$_INTDIV.
		"\tPUSHAL\tREM\n\tPUSHAL\tQUO\n\tPUSHAQ\tDIVD\n\tPUSHAL\tZERO\n"+
		"\tCALLS\t#4,G^LIB$EDIV\n"+check(14, "R0,#^X484")+
		check(15, "QUO,#^X12345678")+
		// LIB$ADAWI: 7FFF + 1 overflows to 8000, LIB$_INTOVF, sign -1.
		"\tPUSHAW\tSIGN\n\tPUSHAW\tSUM\n\tPUSHAW\tADD\n"+
		"\tCALLS\t#3,G^LIB$ADAWI\n"+check(16, "R0,#^X158334")+
		"\tCVTWL\tSUM,R1\n"+check(17, "R1,#-32768")+
		"\tCVTWL\tSIGN,R1\n"+check(18, "R1,#-1")+
		"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (check %d failed)", r0, r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}

// TestLibrtlAdawiOddSum: LIB$ADAWI of a sum at an odd address signals
// SS$_ROPRAND, the reserved operand fault the instruction would take; no
// handler takes it, and the image ends with it.
func TestLibrtlAdawiOddSum(t *testing.T) {
	r0, out := runMacroProgram(t, integerData+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAW\tSIGN\n\tPUSHAB\tSUM+1\n\tPUSHAW\tADD\n"+
		"\tCALLS\t#3,G^LIB$ADAWI\n\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 0x10000454 { // SS$_ROPRAND, message shown
		t.Errorf("R0 = %#x, want SS$_ROPRAND with the message inhibited (0x10000454)", r0)
	}

	if prefix := "%SYSTEM-F-ROPRAND, reserved operand fault"; !strings.HasPrefix(out, prefix) {
		t.Errorf("output %q, want it to start %q", out, prefix)
	}
}
