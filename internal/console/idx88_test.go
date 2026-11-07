package console_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// runIdx88 assembles, links, and runs testdata/idx88's program on govax, on a
// fresh volume with VMS's 3-block clusters as DUA1 (the exchange volume's in
// VMS's run), and returns its output. MACRO and LINK must say nothing.
func runIdx88(t *testing.T) string {
	t.Helper()

	src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "idx88", "idx88.mar"))
	if err != nil {
		t.Fatal(err)
	}

	// The program's lines as written: the usual dispatcher's joins them with
	// "|", which records hold too.
	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.ReplaceAll(buf.String(), "\r", "")
	}

	dsk := filepath.Join(t.TempDir(), "idx88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 /CLUSTER=3 IDX88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch(`COPY "` + src + `"/HOST IDX88.MAR/QUIET`)

	if out := dispatch("MACRO/NOLIST IDX88"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP IDX88"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	return dispatch("RUN IDX88")
}

// TestIdx88Probe rehearses testdata/idx88's program on govax, so that a probe the
// author runs on a VAX can't lose a round to a mistake of ours: it must assemble
// and link with no message, and run through every case to its end with no
// condition reaching a case's handler (docs/PHASE-88 - keyed index files.md,
// subtask 2). The first case, on a sequential file, is what exercises the
// program's helpers.
func TestIdx88Probe(t *testing.T) {
	out := runIdx88(t)

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

	if i := strings.Index("\n"+out, "\n=c "); i >= 0 {
		t.Errorf("a condition reached a case's handler: %.200s", out[i:])
	}
}

// idx88Cases splits the probe's output into its cases, by number. A
// success's STV, which VMS's $CREATE and $OPEN left the file's channel in
// (A0) and govax leaves 0, is masked.
func idx88Cases(text string) map[int][]string {
	cases := map[int][]string{}
	n := -1

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")

		if strings.HasPrefix(line, "#####") {
			n = -1

			continue
		}

		if strings.HasPrefix(line, "#") {
			if k, err := strconv.Atoi(strings.Fields(line[1:])[0]); err == nil {
				n = k

				continue
			}
		}

		if n >= 0 {
			cases[n] = append(cases[n], strings.Replace(line, "sts=00010001 stv=000000A0", "sts=00010001 stv=00000000", 1))
		}
	}

	return cases
}

// creates is each $CREATE of a case with the FAB the probe showed after it.
func creates(lines []string) []string {
	var out []string

	for i, line := range lines {
		if strings.HasPrefix(line, "=$CREATE ") && i+1 < len(lines) {
			out = append(out, line, lines[i+1])
		}
	}

	return out
}

// idx88CreateDifferences are the cases whose $CREATEs aren't VMS's, and why.
var idx88CreateDifferences = map[int]string{
	34:  "overlapping primary segments: VMS made Prolog 1, govax has only Prolog 3 (RMS$_PLV)",
	59:  "PROLOG=2: Prolog 3 only (RMS$_PLV)",
	60:  "PROLOG=1: Prolog 3 only (RMS$_PLV)",
	61:  "PROLOG=2: Prolog 3 only (RMS$_PLV)",
	92:  "MRS 32225: VMS made Prolog 1, govax has only Prolog 3 (RMS$_PLV)",
}

// TestIdx88CreateOracle holds govax's $CREATE of an indexed file to VMS 7.1's
// run of the probe (testdata/idx88's vax/idx88-vax.txt): every $CREATE of the
// key shape, area, and refusal cases with the FAB after it (status, STV, and
// the FAB$L_ALQ and FAB$B_BKS that show how far VMS got), each refusal case
// whole, and the file of case 2, made with no records, to the byte. The
// $OPEN and $DISPLAY of VMS's own files are internal/rms's
// TestIndexedOpenAsVMS, and the record operations TestIdx88Oracle.
func TestIdx88CreateOracle(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "idx88", "vax", "idx88-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	vms := idx88Cases(string(data))
	ours := idx88Cases(runIdx88(t))

	if len(vms) < 150 || len(ours) != len(vms) {
		t.Fatalf("%d cases in VMS's run, %d in govax's", len(vms), len(ours))
	}

	same := func(n int, what string, want, got []string) {
		for i := range max(len(want), len(got)) {
			var w, g string
			if i < len(want) {
				w = want[i]
			}

			if i < len(got) {
				g = got[i]
			}

			if w != g {
				t.Errorf("#%d %s, line %d:\n vms   %s\n govax %s", n, what, i, w, g)

				return
			}
		}
	}

	for n := 1; n <= 103; n++ {
		if why, ok := idx88CreateDifferences[n]; ok {
			if slices.Equal(creates(vms[n]), creates(ours[n])) {
				t.Errorf("#%d is VMS's now, though listed as different: %s", n, why)
			}

			continue
		}

		same(n, "$CREATE", creates(vms[n]), creates(ours[n]))

		if n >= 67 {
			same(n, "whole", vms[n], ours[n])
		}
	}

	same(2, "whole", vms[2], ours[2])
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
