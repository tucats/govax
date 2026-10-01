package asm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 32 oracle (docs/PHASE-32.md, testdata/mar/rms): govax-written
// programs that call VMS's RMS and definition macros, and real VAX MACRO's
// objects for them. govax's own STARLET.MLB, which is written from the RMS
// Reference Manual and these objects, never from VMS's macro library, must
// make the same objects.
var oracleDir = filepath.Join("..", "..", "testdata", "mar", "rms")

// oracleProbes are the probes govax's macros reproduce. A probe real MACRO
// reported an error for isn't one (docs/RMS-MACROS.md, "Oracle answers"),
// and neither is def_state: VAX VMS 7.3 has no $STATEDEF.
var oracleProbes = []string{
	"def_atr", "def_brk", "def_dev", "def_dvi", "def_fab", "def_fib", "def_io",
	"def_jpi", "def_lnm", "def_nam", "def_prt", "def_prv", "def_rab", "def_rms",
	"def_ss", "def_syi", "def_tt", "def_tt2", "def_twice", "def_xab", "def_xaball",
	"def_xabdat", "def_xabfhc", "def_xabitm", "def_xabkey", "def_xabpro",
	"def_xabrdt", "def_xabsum", "def_xabtrm",
	"err_def_global", "err_ss_global",
	"init_fab", "init_fab_all", "init_fab_opt", "init_rab", "init_rab_all",
	"init_rab_opt", "init_nam", "init_nam_all", "init_nam_opt", "init_xaball",
	"init_xaball_all", "init_xaball_opt", "init_xabdat", "init_xabdat_all",
	"init_xabfhc", "init_xabfhc_all", "init_xabitm", "init_xabitm_all",
	"init_xabitm_opt", "init_xabkey", "init_xabkey_all", "init_xabpro",
	"init_xabpro_opt", "init_xabrdt", "init_xabrdt_all", "init_xabsum",
	"init_xabsum_all", "init_xabtrm", "init_xabtrm_all",
}

// TestOracleObjects assembles each oracle probe with govax's own
// STARLET.MLB and checks the object against real MACRO's, record for
// record (apart from traceback records, which govax doesn't write yet).
func TestOracleObjects(t *testing.T) {
	for _, name := range oracleProbes {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(oracleDir, name+".mar"))
			if err != nil {
				t.Fatal(err)
			}

			real, err := filepath.Glob(filepath.Join(oracleDir, "vax", toUpperASCII(name)+".OBJ;*"))
			if err != nil || len(real) != 1 {
				t.Fatalf("real MACRO's object for %s: %v, %v", name, real, err)
			}

			a := macroAssembler()
			a.SetMacroLibraries(govaxStarlet(t))

			if _, err := a.Assemble(string(src)); err != nil {
				t.Fatalf("assemble: %v", err)
			}

			requireSameObject(t, a, readObjectFile(t, real[0]))
		})
	}
}

func toUpperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}

	return string(b)
}

// TestRMSBlockAlignmentMessage: govax's $FAB displays real MACRO's
// informational message for a block that isn't longword aligned, and
// only for that one (testdata/mar/macros/fabalign.mar, whose real MACRO
// log has the message once).
func TestRMSBlockAlignmentMessage(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(macrosDir, "fabalign.mar"))
	if err != nil {
		t.Fatal(err)
	}

	a := macroAssembler()
	a.SetMacroLibraries(govaxStarlet(t))

	if _, err := a.Assemble(string(src)); err != nil {
		t.Fatal(err)
	}

	want := "%MACRO-I-GENINFO, Generated INFO: RMS BLOCK NOT LONGWORD ALIGNED;"
	if msgs := a.Messages(); len(msgs) != 1 || msgs[0] != want {
		t.Errorf("messages = %q, want [%q]", msgs, want)
	}
}

// TestRMSKeywordErrors: an unknown option or choice is an error with real
// MACRO's message (docs/RMS-MACROS.md, O1).
func TestRMSKeywordErrors(t *testing.T) {
	for src, want := range map[string]string{
		"B:\t$FAB\tFAC=<GET,BOGUS>": "UNDEFINED BIT VALUE CODE: BOGUS;",
		"B:\t$FAB\tORG=BOGUS":       "UNDEFINED VALUE FOR FIELD : BOGUS;",
		"B:\t$FAB\tSHR=NQL":         "UNDEFINED BIT VALUE CODE: NQL;",
		"B:\t$XABKEY\tFLG=CHG":      "PRIMARY KEY MAY NOT CHANGE;",
	} {
		a := macroAssembler()
		a.SetMacroLibraries(govaxStarlet(t))

		_, err := a.Assemble("\t.PSECT\tDATA,LONG\n" + src + "\n")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want one with %q", src, err, want)
		}
	}
}
