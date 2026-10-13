package console

import (
	"regexp"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
)

// TestLibrtlMisc: a program calls the miscellaneous routines through
// their stubs (docs/PHASE-55, subtask 6). LIB$AST_IN_PROG is 0 in the
// main program and 1 in an AST routine, which also $WAKEs the process,
// ending a 10-second LIB$WAIT; a quarter-second LIB$WAIT with
// LIB$K_NOWAKE ends by its timer; LIB$ASN_WTH_MBX assigns a mailbox
// (by its logical name) with a new mailbox, whose name LIB$GETDVI gives; and
// LIB$GET_USERS_LANGUAGE, with no SYS$LANGUAGE, vax.language, or host
// locale, is ENGLISH and LIB$_ENGLUSED.
func TestLibrtlMisc(t *testing.T) {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(v, "")
	}

	if settings.Exists("vax.language") {
		old := settings.Get("vax.language")
		_ = settings.Delete("vax.language")

		t.Cleanup(func() { settings.Set("vax.language", old) })
	}

	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"MAIN:\t.LONG\t0\nINAST:\t.LONG\t-1\nS:\t.BLKL\t4\n"+
		"TEN:\t.F_FLOATING\t10.0\nQUARTER:\t.F_FLOATING\t0.25\nNOWAKE:\t.LONG\t1\n"+
		"DEVCH:\t.WORD\t0\nMBXCH:\t.WORD\t0\nDEVNAM:\t.LONG\t^X20\n"+
		"TNAME:\t.ASCID\t/TARGET/\nTCH:\t.WORD\t0\n"+
		"MBX:\t.LONG\t^X020E0000,0\nLANG:\t.LONG\t^X020E0000,0\n"+
		"OUT:\t.LONG\t^X020E0000,0\n"+
		"CTRL:\t.ASCID\t/!UL !UL !XL !XL !XL !AS !XL !AS/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tAST,^M<>\n"+
		"\tCALLS\t#0,G^LIB$AST_IN_PROG\n\tMOVL\tR0,INAST\n"+
		"\t$WAKE_S\n\tRET\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tCALLS\t#0,G^LIB$AST_IN_PROG\n\tMOVL\tR0,MAIN\n"+
		"\t$DCLAST_S\tASTADR=AST\n"+
		"\tPUSHAF\tTEN\n\tCALLS\t#1,G^LIB$WAIT\n\tMOVL\tR0,S\n"+
		"\tPUSHAL\tNOWAKE\n\tPUSHAF\tQUARTER\n\tCALLS\t#2,G^LIB$WAIT\n\tMOVL\tR0,S+4\n"+
		"\t$CREMBX_S\tCHAN=TCH,LOGNAM=TNAME\n"+
		"\tPUSHAW\tMBXCH\n\tPUSHAW\tDEVCH\n\tPUSHL\t#0\n\tPUSHL\t#0\n\tPUSHAQ\tTNAME\n"+
		"\tCALLS\t#5,G^LIB$ASN_WTH_MBX\n\tMOVL\tR0,S+8\n"+
		"\tPUSHAQ\tMBX\n\tPUSHL\t#0\n\tPUSHL\t#0\n\tPUSHAW\tMBXCH\n\tPUSHAL\tDEVNAM\n"+
		"\tCALLS\t#5,G^LIB$GETDVI\n"+
		"\tPUSHAQ\tLANG\n\tCALLS\t#1,G^LIB$GET_USERS_LANGUAGE\n\tMOVL\tR0,S+12\n"+
		"\tPUSHAQ\tLANG\n\tPUSHL\tS+12\n\tPUSHAQ\tMBX\n\tPUSHL\tS+8\n\tPUSHL\tS+4\n\tPUSHL\tS\n"+
		"\tPUSHL\tINAST\n\tPUSHL\tMAIN\n"+
		"\tPUSHAQ\tOUT\n\tPUSHL\t#0\n\tPUSHAQ\tCTRL\n"+
		"\tCALLS\t#11,G^LIB$SYS_FAO\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tOUT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"DONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	want := regexp.MustCompile(`^0 1 00000001 00000001 00000001 _MBA\d+: 00159028 ENGLISH\n$`)
	if !want.MatchString(out) {
		t.Errorf("output %q, want a match for %s", out, want)
	}
}
