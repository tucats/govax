package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIdx88Probe rehearses testdata/idx88's program on govax, so that a probe the
// author runs on a VAX can't lose a round to a mistake of ours: it must assemble
// and link with no message, and run through every case to its end with no
// condition reaching a case's handler (docs/PHASE-88 - keyed index files.md,
// subtask 2). Until RMS has indexed files nearly every case fails with RMS$_ORG,
// and the first, on a sequential file, is what exercises the program's helpers.
func TestIdx88Probe(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "idx88", "idx88.mar"))
	if err != nil {
		t.Fatal(err)
	}

	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "idx88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 IDX88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch(`COPY "` + src + `"/HOST IDX88.MAR/QUIET`)

	if out := dispatch("MACRO/NOLIST IDX88"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP IDX88"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	out := dispatch("RUN IDX88")

	// The helpers' case: a record in the common layout, read by RFA, and its block.
	for _, want := range []string{
		"#0 helpers on a sequential file",
		" puts ok=5 fail=0 last=00010001",
		"|P0000001........A0000000B9999998....................abcdefgh|",
		"   000 5000503030303030303101000000FFFFFFFF4130303030303030423939393939",
		"=dump $READ r=0001827A",
		"##### the end of the cases",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the run's output:\n%.2000s", want, out)
		}
	}

	if i := strings.Index(out, "\n=c "); i >= 0 {
		t.Errorf("a condition reached a case's handler: %.200s", out[i:])
	}
}

// TestXABKEYRefusals holds govax's $XABKEY to the four keys real MACRO's refuses at
// assembly time (testdata/idx88's first run, whose log has these messages), and to
// what it takes beside them.
func TestXABKEYRefusals(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dir := t.TempDir()
	dsk := filepath.Join(dir, "x.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 X`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, c := range []struct{ line, want string }{
		{"DTP=BN4,SIZ=2", "SIZE MUST BE 4 BYTES FOR IN4 AND BN4 DATA TYPES"},
		{"DTP=DIN4,SIZ=8", "SIZE MUST BE 4 BYTES FOR IN4 AND BN4 DATA TYPES"},
		{"DTP=BN2,POS=<0,8>,SIZ=<1,1>", "SEGMENTED KEYS ONLY ALLOWED FOR STG OR COL DATA TYPES"},
		{"DTP=PAC,SIZ=17", "MAXIMUM KEY SIZE (16) EXCEEDED FOR PAC DATA TYPE"},
		{"POS=<0,100>,SIZ=<200,56>", "MAXIMUM SIZE OF KEY FIELD EXCEEDED"},
		{"DTP=BN4,SIZ=4", ""},
		{"DTP=DSTG,POS=<0,100>,SIZ=<200,55>", ""},
		{"DTP=COL,POS=<0,100>,SIZ=<4,4>", ""},
		{"DTP=DPAC,SIZ=16", ""},
	} {
		src := filepath.Join(dir, "x.mar")
		text := "\t.TITLE\tX\n\t$XABKEY\t" + c.line + "\n\t.END\n"

		if err := os.WriteFile(src, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST X.MAR/QUIET`)

		out := dispatch("MACRO/NOLIST X")
		if c.want == "" && out != "" {
			t.Errorf("%s: %s", c.line, out)
		}

		if c.want != "" && !strings.Contains(out, c.want) {
			t.Errorf("%s: want %q, got %q", c.line, c.want, out)
		}
	}
}
