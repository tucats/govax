package console

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// Indexed files end to end (docs/planning/PHASE-88 - keyed index files.md, subtask 14;
// testdata/rms88's README): programs in MACRO and FORTRAN, and DCL, one after
// another on the files they make, in both paging modes. FORTRAN's own programs
// on indexed files are testdata/fortran's parts.for and shared.for, which
// TestFortranPrograms runs.

// rms88Dir is the programs' directory.
var rms88Dir = filepath.Join("..", "..", "testdata", "rms88")

// rms88Copy puts testdata/rms88's files on the volume by their upper-case names.
func rms88Copy(t *testing.T, c *Console, names ...string) {
	t.Helper()

	for _, name := range names {
		text, err := os.ReadFile(filepath.Join(rms88Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + strings.ToUpper(name)},
			rms.TextRecords, splitSource(string(text))); err != nil {
			t.Fatal(err)
		}
	}
}

// rms88Silent runs commands that must say nothing, as a clean build does.
func rms88Silent(t *testing.T, d *Dispatcher, buf *bytes.Buffer, commands ...string) {
	t.Helper()

	buf.Reset()

	for _, command := range commands {
		err := d.Dispatch(command)

		if out := buf.String(); err != nil || out != "" {
			t.Fatalf("%s returned %v and said:\n%s", command, err, out)
		}
	}
}

// rms88Want holds an image's run to an expected file, with the $STATUS line
// shareRun adds, and what it printed may go on to a procedure's.
func rms88Want(t *testing.T, got, file string) {
	t.Helper()

	want, err := os.ReadFile(filepath.Join(rms88Dir, file))
	if err != nil {
		t.Fatal(err)
	}

	if got != string(want) {
		t.Errorf("%s: got:\n%s\nwant:\n%s", file, got, want)
	}
}

// rms88Check holds a file to ANALYZE/RMS_FILE/CHECK, which walks every key.
func rms88Check(t *testing.T, d *Dispatcher, buf *bytes.Buffer, file string) {
	t.Helper()

	buf.Reset()
	shareFindRun(t, d, "ANALYZE/RMS_FILE/CHECK/OUTPUT=SYS$OUTPUT "+file)

	if out := buf.String(); !strings.Contains(out, "The analysis uncovered NO errors.") {
		t.Errorf("ANALYZE/RMS_FILE/CHECK %s:\n%s", file, out)
	}

	buf.Reset()
}

// TestIndexedPhones is one file through everything that can use it: a MACRO
// program makes it, by every record service; two processes share it; a FORTRAN
// program that doesn't declare its keys reads and writes it; and DCL reads it and
// makes another from ANALYZE's description of it.
func TestIndexedPhones(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		for _, mode := range optimizerModes {
			t.Run(mode.name, func(t *testing.T) {
				d, c, buf := shareFindConsole(t, paging)

				rms88Copy(t, c, "phutil.mar", "phones.mar", "phonesl.mar", "phonesc.mar", "phonesf.for", "phones.com")
				shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
				rms88Silent(t, d, buf,
					"MACRO PHUTIL", "MACRO PHONES", "MACRO PHONESL", "MACRO PHONESC",
					"FORTRAN"+mode.qualifier+" PHONESF", "MACRO PHONESF",
					"LINK PHONES,PHUTIL", "LINK PHONESL,PHUTIL", "LINK PHONESC,PHUTIL", "LINK PHONESF")

				const success = "  $STATUS == \"%X00000001\"\n"

				got := shareRun(t, d, buf, "PHONES")
				rms88Want(t, strings.TrimSuffix(got, success), "phones.out")
				rms88Check(t, d, buf, "PHONES.IDX")

				got = shareRun(t, d, buf, "PHONESL")
				rms88Want(t, strings.TrimSuffix(got, success), "phonesl.out")
				rms88Check(t, d, buf, "PHONES.IDX")

				got = shareRun(t, d, buf, "PHONESF")
				rms88Want(t, strings.TrimSuffix(got, success), "phonesf.out")
				rms88Check(t, d, buf, "PHONES.IDX")

				buf.Reset()
				shareFindRun(t, d, "@PHONES")
				rms88Want(t, buf.String(), "phonesd.out")
				rms88Check(t, d, buf, "PHONES2.IDX")
			})
		}
	})
}

// TestIndexedLoad runs load.for, enough records that buckets split and index
// levels grow, on a volume with room for them, and has ANALYZE/RMS_FILE/CHECK
// walk both files it leaves.
func TestIndexedLoad(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		for _, mode := range optimizerModes {
			t.Run(mode.name, func(t *testing.T) {
				d, c, buf := shareFindConsole(t, paging)

				shareFindRun(t, d, "DISMOUNT DUA1:")
				mountFreshContainerSized(t, c, "DUA1:", 20000)
				rms88Copy(t, c, "load.for")
				shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
				rms88Silent(t, d, buf, "FORTRAN"+mode.qualifier+" LOAD", "MACRO LOAD", "LINK LOAD")

				got := shareRun(t, d, buf, "LOAD")
				rms88Want(t, strings.TrimSuffix(got, "  $STATUS == \"%X00000001\"\n"), "load.out")
				rms88Check(t, d, buf, "LOAD.IDX")
				rms88Check(t, d, buf, "LOADV.IDX")

				// The point of so many records: the primary key's root has
				// overflowed, so its tree has grown a level.
				buf.Reset()
				shareFindRun(t, d, "ANALYZE/RMS_FILE/STATISTICS/OUTPUT=SYS$OUTPUT LOAD.IDX")

				levels := regexp.MustCompile(`Number of Index Levels:\s+(\d+)`).FindStringSubmatch(buf.String())
				if levels == nil || levels[1] != "2" {
					t.Errorf("LOAD.IDX's primary key, index levels %v:\n%s", levels, buf.String())
				}
			})
		}
	})
}

// TestCopyIndexedNoEndOfFile copies an indexed file RMS hasn't extended, whose header
// has no end of file: its buckets are all through its allocation, so a copy that stopped
// at the header's end would be empty, which RMS can't open (testdata/vms88's rehearsal).
func TestCopyIndexedNoEndOfFile(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	fdl := "FILE\n\tORGANIZATION\t\tindexed\nRECORD\n\tFORMAT\t\t\tfixed\n\tSIZE\t\t\t20\n" +
		"KEY 0\n\tPOSITION\t\t0\n\tLENGTH\t\t\t4\n\tTYPE\t\t\tstring\n"
	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]EMPTY.FDL"},
		rms.TextRecords, splitSource(fdl)); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	rms88Silent(t, d, buf, "CREATE/FDL=EMPTY.FDL EMPTY.IDX", "COPY/QUIET EMPTY.IDX COPY.IDX")
	rms88Check(t, d, buf, "COPY.IDX")

	host := filepath.Join(t.TempDir(), "empty.idx")
	shareFindRun(t, d, `COPY/QUIET/BINARY EMPTY.IDX "`+host+`"/HOST`)

	info, err := os.Stat(host)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() == 0 || info.Size()%512 != 0 {
		t.Errorf("the host's copy is %d bytes, want whole blocks of the file", info.Size())
	}
}
