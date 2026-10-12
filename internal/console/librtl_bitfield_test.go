package console

import (
	"fmt"
	"strings"
	"testing"
)

// The bit field routines (docs/PHASE-53, subtask 6), called from a
// linked program as a program calls them.

// bitFieldData is the data the bit field programs share: a quadword
// field, positions, sizes, a source, and a find-position.
const bitFieldData = "\t.PSECT\tDATA,NOEXE,WRT,LONG\n" +
	"FIELD:\t.LONG\t^X0000000F,0\n" +
	"POS0:\t.LONG\t0\nPOS4:\t.LONG\t4\n" +
	"SIZE3:\t.BYTE\t3\nSIZE4:\t.BYTE\t4\nSIZE33:\t.BYTE\t33\n" +
	"\t.ALIGN\tLONG\nSRC:\t.LONG\t4\nFIND:\t.LONG\t0\n"

// check is MACRO that compares operands (CMPL's) and returns n when they
// differ: a check's number, as the program's status.
func check(n int, operands string) string {
	return fmt.Sprintf("\tCMPL\t%s\n\tBEQL\t%d$\n\tMOVL\t#%d,R0\n\tRET\n%d$:\n", operands, n, n, n)
}

// TestLibrtlBitFields: LIB$EXTV, LIB$EXTZV, LIB$INSV, LIB$FFS, and
// LIB$FFC through their stubs. Each check that fails returns its own
// number; all passing returns 1.
func TestLibrtlBitFields(t *testing.T) {
	r0, out := runMacroProgram(t, bitFieldData+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		// LIB$EXTV of bits 0-3 (1111) is -1.
		"\tPUSHAL\tFIELD\n\tPUSHAB\tSIZE4\n\tPUSHAL\tPOS0\n"+
		"\tCALLS\t#3,G^LIB$EXTV\n"+check(2, "R0,#-1")+
		// LIB$EXTZV of the same bits is 15.
		"\tPUSHAL\tFIELD\n\tPUSHAB\tSIZE4\n\tPUSHAL\tPOS0\n"+
		"\tCALLS\t#3,G^LIB$EXTZV\n"+check(3, "R0,#15")+
		// LIB$INSV of 4 into bits 4-6 makes the field ^X4F.
		"\tPUSHAL\tFIELD\n\tPUSHAB\tSIZE3\n\tPUSHAL\tPOS4\n\tPUSHAL\tSRC\n"+
		"\tCALLS\t#4,G^LIB$INSV\n"+check(4, "FIELD,#^X4F")+
		// LIB$FFS from bit 4 finds bit 6.
		"\tPUSHAL\tFIND\n\tPUSHAL\tFIELD\n\tPUSHAB\tSIZE4\n\tPUSHAL\tPOS4\n"+
		"\tCALLS\t#4,G^LIB$FFS\n"+check(5, "R0,#1")+
		check(6, "FIND,#6")+
		// LIB$FFC of bits 0-3 finds none: LIB$_NOTFOU, at 4.
		"\tPUSHAL\tFIND\n\tPUSHAL\tFIELD\n\tPUSHAB\tSIZE4\n\tPUSHAL\tPOS0\n"+
		"\tCALLS\t#4,G^LIB$FFC\n"+check(7, "R0,#^X158274")+
		check(8, "FIND,#4")+
		"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (check %d failed)", r0, r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}

// TestLibrtlBitFieldReservedOperand: a size above 32 signals SS$_ROPRAND,
// the reserved operand fault the instruction would take; no handler
// takes it, and the image ends with it.
func TestLibrtlBitFieldReservedOperand(t *testing.T) {
	r0, out := runMacroProgram(t, bitFieldData+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAL\tFIELD\n\tPUSHAB\tSIZE33\n\tPUSHAL\tPOS0\n"+
		"\tCALLS\t#3,G^LIB$EXTZV\n\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 0x10000454 { // SS$_ROPRAND, message shown
		t.Errorf("R0 = %#x, want SS$_ROPRAND with the message inhibited (0x10000454)", r0)
	}

	if prefix := "%SYSTEM-F-ROPRAND, reserved operand fault"; !strings.HasPrefix(out, prefix) {
		t.Errorf("output %q, want it to start %q", out, prefix)
	}
}
