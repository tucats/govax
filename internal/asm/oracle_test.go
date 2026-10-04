package asm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
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
	"store_xabdat", "store_xabfhc", "store_xabkey", "store_xabpro", "store_xabrdt",
	"store_xabsum", "store_xabtrm", "services",
	"r2_order_fab", "r2_order_nam", "r2_order_rab", "r2_order_xaball",
	"r2_order_xabdat", "r2_order_xabkey", "r2_order_xabpro", "r2_order_xabrdt",
	"r2_order_xabtrm", "r2_pro_one", "r2_pro_all", "r2_pro_sym", "r2_uic_one",
}

// TestOracleObjects assembles each oracle probe with govax's own
// STARLET.MLB and checks the object against real MACRO's, record for
// record, traceback records included.
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

			want := readObjectFile(t, real[0])

			// Real MACRO's $RENAME, called with no arguments, emits its
			// CALLG and then reports an unrecognized statement
			// (testdata/mar/rms/vax/RMS.LOG, at location 0A7F of
			// services.mar), so its object's severity is ERROR. govax's
			// $RENAME doesn't (docs/DEVIATIONS.md); the code is the same.
			if name == "services" {
				for _, rec := range want.Records {
					if eom, ok := rec.(*obj.EOM); ok {
						eom.Severity = obj.SeveritySuccess
					}
				}
			}

			requireSameObject(t, a, want)
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
		"B:\t$FAB\tFAC=<GET,BOGUS>":    "UNDEFINED BIT VALUE CODE: BOGUS;",
		"B:\t$FAB\tORG=BOGUS":          "UNDEFINED VALUE FOR FIELD : BOGUS;",
		"B:\t$FAB\tSHR=NQL":            "UNDEFINED BIT VALUE CODE: NQL;",
		"B:\t$XABKEY\tFLG=CHG":         "PRIMARY KEY MAY NOT CHANGE;",
		"\t$RAB_STORE\tRAB=B, RFA=R12": "ILLEGAL USE OF REGISTER : R12 ;",
		"B:\t$XABPRO\tUIC=<377>":       "INVALID UIC_FIELD;",
		"\t$NAM_STORE\tNAM=B, DVI=R2":  "** R2 ** -- ILLEGAL ADDRESSING MODE FOR _DVI;",
		"\t$NAM_STORE\tNAM=B, DVI=#B":  "** #B ** -- ILLEGAL ADDRESSING MODE FOR _DVI;",
	} {
		a := macroAssembler()
		a.SetMacroLibraries(govaxStarlet(t))

		_, err := a.Assemble("\t.PSECT\tDATA,LONG\n" + src + "\n")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want one with %q", src, err, want)
		}
	}
}

// codeStream is the CODE psect's contents, in order: each run of
// immediate bytes in hex, and each relocated longword as <command psect
// +offset>. Positions are left out, so two modules whose code differs
// only by a line's code removed can be compared.
func codeStream(t *testing.T, m *obj.Module) string {
	t.Helper()

	psects := m.Psects()
	code := -1

	for i, p := range psects {
		if p.Name == "CODE" {
			code = i
		}
	}

	var (
		b       strings.Builder
		stack   []string
		current = -1
	)

	for _, rec := range m.Records {
		tir, ok := rec.(*obj.TIR)
		if !ok || tir.Type != obj.RecTIR {
			continue
		}

		for _, c := range tir.Commands {
			switch op := c.Op.String(); {
			case op == "STA_PB" || op == "STA_PW" || op == "STA_PL":
				stack = append(stack, fmt.Sprintf("%d|%s+%#x", c.Psect, psects[c.Psect].Name, c.Value))
			case op == "STA_GBL":
				stack = append(stack, "-1|"+c.Name)
			case strings.HasPrefix(op, "STA_"):
				stack = append(stack, fmt.Sprintf("-1|%#x", c.StackedValue()))
			case op == "CTL_SETRB":
				if _, err := fmt.Sscanf(stack[len(stack)-1], "%d|", &current); err != nil {
					t.Fatal(err)
				}
				stack = stack[:len(stack)-1]
			case op == "STO_IMM":
				if current == code {
					for _, x := range c.Data {
						fmt.Fprintf(&b, "%02x ", x)
					}
				}
			case strings.HasPrefix(op, "STO_"):
				x := stack[len(stack)-1]
				stack = stack[:len(stack)-1]

				if current == code {
					fmt.Fprintf(&b, "<%s %s> ", op, x[strings.Index(x, "|")+1:])
				}
			}
		}
	}

	return b.String()
}

// TestOracleStoreProbesWithErrors covers the store probes real MACRO
// reported errors in (docs/RMS-MACROS.md, "Oracle answers"). Each rejected
// line is an error for govax too, with real MACRO's message where the
// macro reports it; and the probe without those lines assembles to real
// MACRO's code without theirs. A rejected line left only the MOVAL of its
// block's address in real MACRO's object (two MOVALs in a row), and
// DNA=R8 an illegal MOVAL R8.
func TestOracleStoreProbesWithErrors(t *testing.T) {
	for name, rejected := range map[string]map[string]string{
		"store_fab": {"$FAB_STORE\tFAB=FAB1, DNA=R8": ""},
		"store_nam": {
			"$NAM_STORE\tNAM=NAM1, DID=TRIP": "** TRIP ** -- ILLEGAL ADDRESSING MODE FOR _DID;",
			"$NAM_STORE\tNAM=NAM1, DVI=DVIB": "** DVIB ** -- ILLEGAL ADDRESSING MODE FOR _DVI;",
			"$NAM_STORE\tNAM=NAM1, FID=TRIP": "** TRIP ** -- ILLEGAL ADDRESSING MODE FOR _FID;",
		},
		"store_rab":    {"$RAB_STORE\tRAB=RBUF, RFA=TRIP": "** TRIP ** -- ILLEGAL ADDRESSING MODE FOR _RFA;"},
		"store_xaball": {"$XABALL_STORE\tXAB=XAB1, RFI=TRIP": "** TRIP ** -- ILLEGAL ADDRESSING MODE FOR _RFI;"},
	} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(oracleDir, name+".mar"))
			if err != nil {
				t.Fatal(err)
			}

			var kept []string

			for _, line := range strings.Split(string(src), "\n") {
				msg, ok := rejected[strings.TrimPrefix(line, "\t")]
				if !ok {
					kept = append(kept, line)

					continue
				}

				// The line alone, in the probe's place, is an error.
				a := macroAssembler()
				a.SetMacroLibraries(govaxStarlet(t))

				if _, err := a.Assemble(strings.Join(append(append([]string(nil), kept...), line, "\tRET", labelsOf(string(src)), "\t.END"), "\n")); err == nil || !strings.Contains(err.Error(), msg) {
					t.Errorf("%s: error %v, want one with %q", line, err, msg)
				}
			}

			a := macroAssembler()
			a.SetMacroLibraries(govaxStarlet(t))

			if _, err := a.Assemble(strings.Join(kept, "\n")); err != nil {
				t.Fatalf("assemble: %v", err)
			}

			got, err := a.Object(ObjectOptions{})
			if err != nil {
				t.Fatal(err)
			}

			real, err := filepath.Glob(filepath.Join(oracleDir, "vax", toUpperASCII(name)+".OBJ;*"))
			if err != nil || len(real) != 1 {
				t.Fatalf("real MACRO's object: %v, %v", real, err)
			}

			want := strings.Replace(codeStream(t, readObjectFile(t, real[0])), "de 58 a0 30 ", "", 1)

			// Collapse each MOVAL of the block's address that a rejected
			// line left, which the next line's MOVAL follows directly.
			moval := regexp.MustCompile(`(de ef <STO_LD [^>]+> 50 )(de ef <STO_LD [^>]+> 50 )`)
			for {
				next := moval.ReplaceAllStringFunc(want, func(s string) string {
					m := moval.FindStringSubmatch(s)
					if m[1] == m[2] {
						return m[2]
					}

					return s
				})
				if next == want {
					break
				}

				want = next
			}

			if got := codeStream(t, got); got != want {
				t.Errorf("code:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// labelsOf returns the LABELS psect's part of a probe's source.
func labelsOf(src string) string {
	i := strings.Index(src, "\t.PSECT\tLABELS")
	j := strings.LastIndex(src, "\t.END")

	return src[i:j]
}
