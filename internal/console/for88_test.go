package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/isam"
)

// for88Dir is round 2's directory, from this package's.
var for88Dir = filepath.Join("..", "..", "testdata", "for88")

// runFor88OnFORRTL runs round 2 of the Phase 88 probe on VMS's FORRTL.EXE over
// govax (docs/planning/PHASE-88 - keyed index files.md, subtask 12), and returns its
// output. It skips without FORRTL.EXE on a local volume.
func runFor88OnFORRTL(t *testing.T) (out, saved string) {
	t.Helper()

	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	src, err := filepath.Abs(filepath.Join(for88Dir, "for88.mar"))
	if err != nil {
		t.Fatal(err)
	}

	opt := filepath.Join(t.TempDir(), "for88.opt")
	if err := os.WriteFile(opt, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST FOR88.MAR`)
	dispatch(`COPY "` + opt + `"/HOST FOR88.OPT`)

	if out := dispatch("MACRO/NOLIST FOR88"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP FOR88,FOR88/OPTIONS"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	out = dispatch("RUN FOR88")
	saved = filepath.Join(t.TempDir(), "for88.bck")

	dispatch(`BACKUP *.IDX;* FOR88.BCK/SAVE_SET`)
	dispatch(`COPY FOR88.BCK "` + saved + `"/HOST/BINARY`)

	return out, saved
}

// for88Volatile are what varies from run to run or with the volume, not with
// RMS: a status value's stack addresses, and a block size no file here uses.
var for88Volatile = regexp.MustCompile(`stv=[0-9A-F]+|bls=[0-9]+`)

// for88Differences are the cases whose output differs from VMS's, by the case's
// line, with why.
var for88Differences = map[string]string{
	// FORRTL's by-name NAME comes back blank over govax on any organization
	// (docs/planning/PHASE-81 - FORTRAN files.md, "Open"; for81's #479).
	"#145 INQUIRE by file, closed": "NAME by file, on FORRTL only",

	// W7's scattered load splits elsewhere, so it ends up another size.
	"#295 many records, past a bucket: ten buckets' worth": "W7's splits",
}

// for88Cases splits the probe's output into its cases, by their #n lines.
func for88Cases(text string) map[string]string {
	cases := map[string]string{}
	title := ""

	for _, line := range strings.FieldsFunc(strings.NewReplacer("\r", "", "|", "\n").Replace(text), func(r rune) bool { return r == '\n' }) {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "##### ") {
			title = line

			continue
		}

		if title != "" {
			cases[title] += for88Volatile.ReplaceAllString(line, "_") + "\n"
		}
	}

	return cases
}

// TestFor88Oracle runs the probe on VMS's FORRTL.EXE over govax's RMS, and holds
// every case's output, and every file it made, to VMS's run. With FOR88_WRITE set
// it keeps the output in that file.
func TestFor88Oracle(t *testing.T) {
	out, saved := runFor88OnFORRTL(t)

	if path := os.Getenv("FOR88_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(out, "##### the end of the cases") {
		t.Errorf("the probe stopped before its end; its last lines:\n%s", lastLines(out, 30))
	}

	text, err := os.ReadFile(filepath.Join(for88Dir, "vax", "for88-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	got, want := for88Cases(out), for88Cases(string(text))
	if len(want) < 300 {
		t.Fatalf("%d cases in VMS's run", len(want))
	}

	for title, w := range want {
		_, expected := for88Differences[title]

		switch g, ok := got[title]; {
		case !ok:
			t.Errorf("%s: not run", title)
		case g != w && !expected:
			t.Errorf("%s:\nVMS:\n%s\ngovax:\n%s", title, w, g)
		case g == w && expected:
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from for88Differences", title)
		}
	}

	vms := readSaveSet(t, filepath.Join(for88Dir, "vax", "for88.bck"))
	ours := map[string]*backup.File{}

	for _, f := range readSaveSet(t, saved).Files {
		ours[f.Name] = f
	}

	if len(vms.Files) != 46 {
		t.Errorf("%d files in VMS's save set, where its log has 46", len(vms.Files))
	}

	for _, f := range vms.Files {
		g, ok := ours[f.Name]

		switch {
		case !ok:
			t.Errorf("%s: VMS's run made it, govax's not", f.Name)
		case f.Name == "[000000]W7.IDX;1":
			compareRecordsOnly(t, f, g)
		default:
			compareConvertFile(t, f, g)
		}
	}
}

// for88Record is a data record in a Dump: its key, size, and sum.
var for88Record = regexp.MustCompile(`key [0-9A-F]+, size \d+, sum [0-9A-F]+`)

// compareRecordsOnly holds a file whose buckets differ from VMS's to VMS's
// records, in any order, each by its key, size, and sum.
func compareRecordsOnly(t *testing.T, vms, govax *backup.File) {
	t.Helper()

	records := func(f *backup.File) []string {
		m, err := isam.Open(memBlocks(f.Data()), isam.Attributes{Size: int(f.MaxRecord())})
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}

		var b strings.Builder

		if err := m.Dump(&b); err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}

		list := for88Record.FindAllString(b.String(), -1)
		sort.Strings(list)

		return list
	}

	if a, b := records(vms), records(govax); len(a) == 0 || strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Errorf("%s: %d records on VMS, %d on govax, or not the same ones", vms.Name, len(a), len(b))
	}
}

// lastLines is the last n lines of a text.
func lastLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}
