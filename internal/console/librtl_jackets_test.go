package console

import (
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestLibrtlJackets: a program calls LIB$SYS_FAO and LIB$SYS_GETMSG
// through their stubs with a dynamic string, and writes each result with
// LIB$PUT_OUTPUT. The jackets' scratch buffers are in their own frames:
// the caller's SP is the same after each call (docs/PHASE-55, subtask 2).
func TestLibrtlJackets(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+ // a dynamic string, empty
		"CTRL:\t.ASCID\t/!UL jacket!%S, !AS/\n"+
		"WORD:\t.ASCID\t/ok/\n"+
		"MSG:\t.LONG\t1\n"+ // SS$_NORMAL
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<R2>\n"+
		"\tMOVL\tSP,R2\n"+
		"\tPUSHAQ\tWORD\n\tPUSHL\t#3\n\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#5,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tCMPL\tSP,R2\n\tBNEQ\tMOVED\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAL\tMSG\n"+
		"\tCALLS\t#3,G^LIB$SYS_GETMSG\n\tBLBC\tR0,DONE\n"+
		"\tCMPL\tSP,R2\n\tBNEQ\tMOVED\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#1,R0\nDONE:\tRET\n"+
		"MOVED:\tMOVL\t#2,R0\n\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (2: SP moved)", r0)
	}

	want := "3 jackets, ok\n%SYSTEM-S-NORMAL, normal successful completion\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestLibrtlItemRoutines: a program asks LIB$GETJPI, LIB$GETSYI, and
// LIB$GETDVI for one item each through their stubs, as text in a dynamic
// string, and writes each with LIB$PUT_OUTPUT (docs/PHASE-55, subtask
// 3). LIB$GETJPI's numeric value goes to a longword too, which the
// program checks against JPI$K_INTERACTIVE, and LIB$GETDVI asks about
// a mailbox by the channel $CREMBX assigned it.
func TestLibrtlItemRoutines(t *testing.T) {
	code := func(name string) string { return fmt.Sprintf("%d", vmsdef.Symbols[name]) }

	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"MODE:\t.LONG\t"+code("JPI$_MODE")+"\n"+
		"NODE:\t.LONG\t"+code("SYI$_NODENAME")+"\n"+
		"MBXITM:\t.LONG\t"+code("DVI$_MBX")+"\n"+
		"CHAN:\t.WORD\t0\n"+
		"VALUE:\t.LONG\t0\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tOUT\n\tPUSHAL\tVALUE\n\tCLRL\t-(SP)\n\tCLRL\t-(SP)\n\tPUSHAL\tMODE\n"+
		"\tCALLS\t#5,G^LIB$GETJPI\n\tBLBC\tR0,DONE\n"+
		"\tCMPL\tVALUE,#"+code("JPI$K_INTERACTIVE")+"\n\tBNEQ\tWRONG\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tPUSHAQ\tOUT\n\tCLRL\t-(SP)\n\tPUSHAL\tNODE\n"+
		"\tCALLS\t#3,G^LIB$GETSYI\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tBRB\tASKDVI\nDONE:\tRET\n"+ // in reach of every BLBC
		"ASKDVI:\tCLRQ\t-(SP)\n\tCLRQ\t-(SP)\n\tCLRL\t-(SP)\n\tPUSHAW\tCHAN\n\tCLRL\t-(SP)\n"+
		"\tCALLS\t#7,G^SYS$CREMBX\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCLRQ\t-(SP)\n\tPUSHAW\tCHAN\n\tPUSHAL\tMBXITM\n"+
		"\tCALLS\t#5,G^LIB$GETDVI\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#1,R0\n\tRET\n"+
		"WRONG:\tMOVL\t#2,R0\n\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (2: wrong JPI$_MODE value)", r0)
	}

	want := "INTERACTIVE\nGOVAX\nTRUE\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}
