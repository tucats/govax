package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// runIdx88D runs round 4 of testdata/idx88, IDX88D.COM (DCL on indexed files;
// docs/planning/PHASE-88 - keyed index files.md, subtask 9), on govax, on a fresh volume
// of VMS's 3-block clusters as DUA1 with node SIMVAX, as the VAX's was, and
// returns the log it wrote and what reached the terminal.
func runIdx88D(t *testing.T) (log, terminal string) {
	t.Helper()

	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	c.RTL.NodeName = "SIMVAX"
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) {
		t.Helper()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v", line, err)
		}
	}

	dsk := filepath.Join(t.TempDir(), "idx88d.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 /CLUSTER=3 IDX88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range []string{"idx88d.mar", "idx88d.com"} {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "idx88", name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + strings.ToUpper(name) + "/QUIET")
	}

	buf.Reset()
	dispatch("@IDX88D/OUTPUT=IDX88D.LOG")

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]IDX88D.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	for _, r := range records {
		b.Write(r)
		b.WriteByte('\n')
	}

	return b.String(), strings.ReplaceAll(buf.String(), "\r", "")
}

// TestIdx88DProbe rehearses round 4 on govax, so that a change to the
// procedure can't cost the author a VAX run: IDX88D.MAR must assemble, link,
// and make its files, and the procedure must run through every section to its
// end. IDX88D_LOG=file keeps govax's log in the file.
func TestIdx88DProbe(t *testing.T) {
	log, _ := runIdx88D(t)

	if out := os.Getenv("IDX88D_LOG"); out != "" {
		if err := os.WriteFile(out, []byte(log), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{
		"$ RUN IDX88D\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00010001\"\n",
		"  M = \"END\"\n",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("no %q in the log", want)
		}
	}
}

// TestIdx88DOracle holds round 4's log on govax to VMS V7.1's,
// testdata/idx88/vax/idx88d.log, line for line (docs/planning/PHASE-88 - keyed index
// files.md, subtask 9): every WRITE, READ, and WRITE/UPDATE on an indexed file,
// its message and $STATUS, each READ's symbol, TYPE, DUMP/RECORD, DIRECTORY/FULL,
// and F$FILE_ATTRIBUTES. Times are masked (idx88DMask), and VMS's log starts
// earlier, with the procedure's comments, since verification was already on in
// the author's session: both are compared from the first command after SET
// VERIFY.
func TestIdx88DOracle(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "idx88", "vax", "idx88d.log"))
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	got, _ := runIdx88D(t)
	want, have := idx88DMask(string(text)), idx88DMask(got)

	for i := 0; i < max(len(want), len(have)); i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}

		if i < len(have) {
			g = have[i]
		}

		if w != g {
			t.Fatalf("line %d differs:\nVMS:   %q\ngovax: %q\nVMS's lines before it:\n%s", i+1, w, g,
				strings.Join(want[max(0, i-8):i], "\n"))
		}
	}
}

// idx88DTimes are the times DUMP's heading and DIRECTORY/FULL's dates show.
var idx88DTimes = regexp.MustCompile(`[ 0-9]\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d`)

// idx88DMask is a round 4 log's lines from its first command after SET
// VERIFY, carriage returns gone and times made "<time>".
func idx88DMask(text string) []string {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r", ""), "\n"), "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "$ DELETE D%.IDX") {
			lines = lines[i:]

			break
		}
	}

	for i, line := range lines {
		lines[i] = idx88DTimes.ReplaceAllString(line, "<time>")
	}

	return lines
}
