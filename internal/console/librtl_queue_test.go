package console

import (
	"strings"
	"testing"
)

// The queue and interlock routines (docs/PHASE-53, subtask 9), called
// from a linked program as a program calls them.

// queueData is the data the queue programs share: a header, two
// entries, the remque-address, and a 128-bit array with its positions.
const queueData = "\t.PSECT\tDATA,NOEXE,WRT,QUAD\n" +
	"HEADER:\t.QUAD\t0\nENTRY1:\t.QUAD\t0\nENTRY2:\t.QUAD\t0\nODD:\t.LONG\t0,0,0\nREMOVED:\t.LONG\t0\n" +
	"BITS:\t.LONG\t0,0,0,0\nPOS42:\t.LONG\t42\n"

// TestLibrtlQueues: LIB$INSQHI, LIB$INSQTI, LIB$REMQHI, LIB$REMQTI,
// LIB$BBSSI, and LIB$BBCCI through their stubs. Each check that fails
// returns its own number; all passing returns 1.
func TestLibrtlQueues(t *testing.T) {
	r0, out := runMacroProgram(t, queueData+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		// An empty queue: LIB$_QUEWASEMP, the header's address.
		"\tPUSHAL\tREMOVED\n\tPUSHAQ\tHEADER\n\tCALLS\t#2,G^LIB$REMQHI\n"+
		check(2, "R0,#^X1582EC")+"\tMOVAQ\tHEADER,R1\n"+check(3, "REMOVED,R1")+
		// The first entry: LIB$_ONEENTQUE; the second, SS$_NORMAL.
		"\tPUSHAQ\tHEADER\n\tPUSHAQ\tENTRY1\n\tCALLS\t#2,G^LIB$INSQHI\n"+check(4, "R0,#^X158019")+
		"\tPUSHAQ\tHEADER\n\tPUSHAQ\tENTRY2\n\tCALLS\t#2,G^LIB$INSQTI\n"+check(5, "R0,#1")+
		// The tail is entry 2; then the head, entry 1, leaves it empty.
		"\tPUSHAL\tREMOVED\n\tPUSHAQ\tHEADER\n\tCALLS\t#2,G^LIB$REMQTI\n"+
		check(6, "R0,#1")+"\tMOVAQ\tENTRY2,R1\n"+check(7, "REMOVED,R1")+
		"\tPUSHAL\tREMOVED\n\tPUSHAQ\tHEADER\n\tCALLS\t#2,G^LIB$REMQHI\n"+
		check(8, "R0,#^X158019")+"\tMOVAQ\tENTRY1,R1\n"+check(9, "REMOVED,R1")+
		check(10, "HEADER,#0")+
		// Bit 42: set (was 0), set again (was 1), cleared (was 1).
		"\tPUSHAL\tBITS\n\tPUSHAL\tPOS42\n\tCALLS\t#2,G^LIB$BBSSI\n"+check(11, "R0,#0")+
		check(12, "BITS+4,#^X400")+
		"\tPUSHAL\tBITS\n\tPUSHAL\tPOS42\n\tCALLS\t#2,G^LIB$BBSSI\n"+check(13, "R0,#1")+
		"\tPUSHAL\tBITS\n\tPUSHAL\tPOS42\n\tCALLS\t#2,G^LIB$BBCCI\n"+check(14, "R0,#1")+
		check(15, "BITS+4,#0")+
		"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (check %d failed)", r0, r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}

// TestLibrtlQueueReservedOperand: an entry that isn't quadword aligned
// signals SS$_ROPRAND, the reserved operand fault the instruction would
// take; no handler takes it, and the image ends with it.
func TestLibrtlQueueReservedOperand(t *testing.T) {
	r0, out := runMacroProgram(t, queueData+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tHEADER\n\tPUSHAL\tODD+4\n\tCALLS\t#2,G^LIB$INSQHI\n\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 0x10000454 { // SS$_ROPRAND, message shown
		t.Errorf("R0 = %#x, want SS$_ROPRAND with the message inhibited (0x10000454)", r0)
	}

	if prefix := "%SYSTEM-F-ROPRAND, reserved operand fault"; !strings.HasPrefix(out, prefix) {
		t.Errorf("output %q, want it to start %q", out, prefix)
	}
}
