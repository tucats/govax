package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// runIdx88C runs round 3 of testdata/idx88, IDX88C.COM (the utilities:
// CREATE/FDL, CONVERT, ANALYZE/RMS_FILE; docs/PHASE-88 - keyed index files.md,
// subtask 10), on govax, on a fresh volume of VMS's 3-block clusters as DUA1
// with node SIMVAX, as the VAX's was, round 1's files restored into [IN] from
// its save set as EXCHANGEC.CMD restores them, and returns the log it wrote
// and the console it ran on.
func runIdx88C(t *testing.T) (string, *console.Console) {
	t.Helper()

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	c.RTL.NodeName = "SIMVAX"
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) {
		t.Helper()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v", line, err)
		}
	}

	source := func(name string) string {
		t.Helper()

		path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "idx88", name))
		if err != nil {
			t.Fatal(err)
		}

		return path
	}

	dsk := filepath.Join(t.TempDir(), "idx88c.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 /CLUSTER=3 IDX88C`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range []string{"idx88c.com", "idx88p.mar"} {
		dispatch(`COPY "` + source(name) + `"/HOST ` + strings.ToUpper(name) + "/QUIET")
	}

	dispatch("CREATE/DIRECTORY [IN]")
	dispatch(`BACKUP "` + source(filepath.Join("vax", "idx88.bck")) + `"/SAVE_SET [IN]`)

	dispatch("@IDX88C/OUTPUT=IDX88C.LOG")

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]IDX88C.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	for _, r := range records {
		b.Write(r)
		b.WriteByte('\n')
	}

	return b.String(), c
}

// TestIdx88CProbe rehearses round 3 on govax, so that a change to the
// procedure can't cost the author a VAX run: IDX88P.MAR must assemble and
// link, each of C12's damaged copies must have the byte meant changed (the
// values are round 1's S35.IDX's), and the procedure must run through every
// section to its end. Until govax has the utilities their commands fail, and
// what follows from them. IDX88C_LOG=file keeps govax's log in the file.
func TestIdx88CProbe(t *testing.T) {
	log, _ := runIdx88C(t)

	if out := os.Getenv("IDX88C_LOG"); out != "" {
		if err := os.WriteFile(out, []byte(log), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{
		"$ LINK/NOMAP IDX88P\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X10000001\"\n",
		"patch X01.IDX vbn 4 offset 0: 05 to 06\n",
		"patch X02.IDX vbn 5 offset 511: 05 to 06\n",
		"patch X03.IDX vbn 1 offset 510: F6 to F7\n",
		"patch X04.IDX vbn 1 offset 12: 07 to 0A\n",
		"patch X05.IDX vbn 4 offset 4: 4E to 50\n",
		"patch X06.IDX vbn 4 offset 6: 06 to 05\n",
		"patch X07.IDX vbn 4 offset 8: 04 to 68\n",
		"patch X08.IDX vbn 3 offset 24: 0F to 41\n",
		"patch X09.IDX vbn 10 offset 20: 80 to 81\n",
		"patch X10.IDX vbn 7 offset 14: 01 to 02\n",
		"patch X11.IDX vbn 4 offset 13: 01 to 02\n",
		"patch X12.IDX vbn 4 offset 12: 00 to 01\n",
		"patch X13.IDX vbn 1 offset 17: 00 to 01\n",
		"patch X14.IDX vbn 2 offset 84: 0A to 0B\n",
		"  M = \"END\"\n",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("no %q in the log", want)
		}
	}
}
