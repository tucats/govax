package console_test

import (
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
