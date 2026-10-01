package asm

import (
	"os"
	"path/filepath"
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
