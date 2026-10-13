package console

import (
	"regexp"
	"testing"
)

// TestLibrtlTimers: a program times three demand-zero page touches with
// a heap timer block, through the routines' stubs (docs/PHASE-55,
// subtask 5): LIB$STAT_TIMER's page faults grow by the three pages
// (the calls fault a stack page in too, so a first reading is taken
// before the touches), and
// LIB$SHOW_TIMER passes its line to an action routine, which writes it
// and returns its user argument, SHOW_TIMER's status then; LIB$FREE_TIMER
// zeroes the handle. The times vary with the run, so the line is matched
// by its form.
func TestLibrtlTimers(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"HANDLE:\t.LONG\t0\nCODE:\t.LONG\t5\nFAULTS:\t.LONG\t0\nBEFORE:\t.LONG\t0\nRANGE:\t.BLKL\t2\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"CTRL:\t.ASCID\t/!XL !UL !XL !XL !XL !XL/\n"+
		"S:\t.BLKL\t4\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tACTION,^M<>\n"+
		"\tPUSHL\t4(AP)\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n\tMOVL\t8(AP),R0\n\tRET\n"+
		"\t.ENTRY\tSTART,^M<R2>\n"+
		"\tPUSHAL\tHANDLE\n\tCALLS\t#1,G^LIB$INIT_TIMER\n\tMOVL\tR0,S\n"+
		"\t$EXPREG_S\tPAGCNT=#4,RETADR=RANGE\n"+
		"\tPUSHAL\tHANDLE\n\tPUSHAL\tBEFORE\n\tPUSHAL\tCODE\n\tCALLS\t#3,G^LIB$STAT_TIMER\n"+
		"\tMOVL\tRANGE,R2\n\tCLRL\t(R2)\n\tCLRL\t512(R2)\n\tCLRL\t1024(R2)\n"+
		"\tPUSHAL\tHANDLE\n\tPUSHAL\tFAULTS\n\tPUSHAL\tCODE\n\tCALLS\t#3,G^LIB$STAT_TIMER\n\tMOVL\tR0,S+4\n"+
		"\tSUBL2\tBEFORE,FAULTS\n"+
		"\tPUSHL\t#^X12345671\n\tPUSHAL\tACTION\n\tPUSHL\t#0\n\tPUSHAL\tHANDLE\n"+
		"\tCALLS\t#4,G^LIB$SHOW_TIMER\n\tMOVL\tR0,S+8\n"+
		"\tPUSHAL\tHANDLE\n\tCALLS\t#1,G^LIB$FREE_TIMER\n\tMOVL\tR0,S+12\n"+
		"\tPUSHL\tHANDLE\n\tPUSHL\tS+12\n\tPUSHL\tS+8\n\tPUSHL\tS+4\n\tPUSHL\tFAULTS\n\tPUSHL\tS\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#9,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"DONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	want := regexp.MustCompile(`^ ELAPSED:    0 00:00:\d\d\.\d\d  CPU: 0:00:\d\d\.\d\d  BUFIO: 0  DIRIO: 0  FAULTS: \d+ \n` +
		`00000001 3 00000001 12345671 00000001 00000000\n$`)
	if !want.MatchString(out) {
		t.Errorf("output %q, want a match for %s", out, want)
	}
}
