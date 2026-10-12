package console

import "testing"

// TestLibrtlCrc: LIB$CRC_TABLE and LIB$CRC through their stubs, with the
// manual's example: CRC-16's polynomial (octal 120001), an initial CRC of
// 0, and "DOVE" and "HOSE", whose CRCs are the same (^X74DB). Each check
// that fails returns its own number; all passing returns 1.
func TestLibrtlCrc(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"POLY:\t.LONG\t^O120001\nZERO:\t.LONG\t0\nTABLE:\t.BLKL\t16\n"+
		"DOVE:\t.ASCID\t/DOVE/\nHOSE:\t.ASCID\t/HOSE/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAL\tTABLE\n\tPUSHAL\tPOLY\n\tCALLS\t#2,G^LIB$CRC_TABLE\n"+
		check(2, "TABLE+4,#^XCC01")+
		"\tPUSHAQ\tDOVE\n\tPUSHAL\tZERO\n\tPUSHAL\tTABLE\n\tCALLS\t#3,G^LIB$CRC\n"+check(3, "R0,#^X74DB")+
		"\tPUSHAQ\tHOSE\n\tPUSHAL\tZERO\n\tPUSHAL\tTABLE\n\tCALLS\t#3,G^LIB$CRC\n"+check(4, "R0,#^X74DB")+
		"\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1 (check %d failed)", r0, r0)
	}

	if out != "" {
		t.Errorf("output %q, want none", out)
	}
}
