package console_test

import (
	"bytes"
	hexenc "encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestIdx88CCreateOracle holds CREATE/FDL (Phase 88's subtask 10b) to VMS
// 7.1's run of round 3 (testdata/idx88's IDX88C.COM): every CREATE/FDL of
// sections C2 to C4 and its commands after it, up to the next, line for
// line, times and file IDs masked and ANALYZE/RMS_FILE's reports left out
// (they're subtask 10d's); then each file those sections made, its header
// and its blocks, against the save set VMS made of them, IDX88C.BCK, by the
// one govax's run makes of its own.
//
// The differences expected are the Prolog 1 and 2 files VMS made, which
// govax refuses (the author's answer 1): CREATE/FDL says RMS$_PLV.
func TestIdx88CCreateOracle(t *testing.T) {
	vax := filepath.Join("..", "..", "testdata", "idx88", "vax")

	text, err := os.ReadFile(filepath.Join(vax, "idx88c.log"))
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	log, c := runIdx88C(t)

	want, have := fdlCases(string(text)), fdlCases(log)
	if len(want) == 0 || len(have) != len(want) {
		t.Fatalf("%d CREATE/FDL cases on VMS, %d on govax", len(want), len(have))
	}

	for i, w := range want {
		g := have[i]
		if w.command != g.command {
			t.Fatalf("case %d is %q on VMS, %q on govax", i+1, w.command, g.command)
		}

		if expectedFDLDifferences[w.command] {
			if strings.Join(w.lines, "\n") == strings.Join(g.lines, "\n") {
				t.Errorf("%s: expected to differ, and doesn't", w.command)
			}

			continue
		}

		for j := 0; j < max(len(w.lines), len(g.lines)); j++ {
			var wl, gl string
			if j < len(w.lines) {
				wl = w.lines[j]
			}

			if j < len(g.lines) {
				gl = g.lines[j]
			}

			if wl != gl {
				t.Errorf("%s, line %d:\nVMS:   %q\ngovax: %q", w.command, j+1, wl, gl)

				break
			}
		}
	}

	vmsSet := readSaveSet(t, filepath.Join(vax, "idx88c.bck"))

	data, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: "DUA1:[000000]IDX88C.BCK"})
	if err != nil {
		t.Fatal(err)
	}

	ours, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	govaxFiles := map[string]*backup.File{}
	for _, f := range ours.Files {
		govaxFiles[f.Name] = f
	}

	compared := 0

	for _, vf := range vmsSet.Files {
		if !fdlMadeFile.MatchString(vf.Name) || expectedFDLFiles[vf.Name] {
			continue
		}

		gf, ok := govaxFiles[vf.Name]
		if !ok {
			t.Errorf("%s: VMS made it, govax didn't", vf.Name)

			continue
		}

		compared++

		compareFDLFile(t, vf, gf)
	}

	compared += fdlFromAnalysis(t, c, vmsSet)

	if compared != 42 {
		t.Errorf("%d files compared, where sections C2 to C5 made 42 (with C3A, C3B, and C3C left out)", compared)
	}
}

// fdlFromAnalysis is round 3's C5 and C13 on govax: CREATE/FDL from the FDL
// files VMS's ANALYZE/RMS_FILE/FDL wrote of round 1's S35.IDX, S62.IDX, and
// F122.IDX (A35.FDL, A62.FDL, A122.FDL, restored here from VMS's save set into
// [V], so this holds CREATE/FDL to VMS's own FDL files), each file compared
// with VMS's C51.IDX to C53.IDX; then A35.FDL with no file named, which makes
// the file its NAME names, DUA1:[IN]S35.IDX;1, as its next version, ;2. It
// returns how many files it compared.
func fdlFromAnalysis(t *testing.T, c *console.Console, vmsSet *backup.SaveSet) int {
	t.Helper()

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	dispatch := func(line string) {
		t.Helper()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v", line, err)
		}
	}

	save, err := filepath.Abs(filepath.Join("..", "..", "testdata", "idx88", "vax", "idx88c.bck"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch("CREATE/DIRECTORY [V]")
	dispatch(`BACKUP "` + save + `"/SAVE_SET [V]`)
	dispatch("CREATE/DIRECTORY [R5]")

	// The save set's file of no name or type comes back with the rest.
	restored, _, err := c.ContainerSession.Directory("DUA1:[V].;*", rms.DirectoryOptions{})
	if err != nil || !strings.Contains(restored, "\n.;1 ") {
		t.Errorf("[V].;1 wasn't restored (%v):\n%s", err, restored)
	}

	for _, pair := range [][2]string{{"A35", "C51"}, {"A62", "C52"}, {"A122", "C53"}} {
		dispatch("CREATE/FDL=[V]" + pair[0] + " [R5]" + pair[1] + ".IDX")
	}

	// K50.FDL gives fills with no bucket size, so no bucket to take a
	// percentage of: VMS's C73.IDX, which CONVERT made from it, has
	// whole-bucket fills in its key descriptors.
	dispatch("CREATE/FDL=[V]K50 [R5]K50.IDX")
	dispatch("BACKUP [R5]*.*;* R5.BCK/SAVE_SET")

	data, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: "DUA1:[000000]R5.BCK"})
	if err != nil {
		t.Fatal(err)
	}

	ours, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	govaxFiles := map[string]*backup.File{}
	for _, f := range ours.Files {
		govaxFiles[strings.Replace(f.Name, "[R5]", "[R3]", 1)] = f
	}

	compared := 0

	for _, vf := range vmsSet.Files {
		// The fills, at each of the two key descriptors' offset 0x18.
		if vf.Name == "[R3]C73.IDX;1" {
			a, b := vf.Data(), govaxFiles["[R3]K50.IDX;1"].Data()

			for _, off := range []int{0x18, 0x218} {
				if !bytes.Equal(a[off:off+4], b[off:off+4]) {
					t.Errorf("K50's fills at %#x: %x on VMS, %x on govax", off, a[off:off+4], b[off:off+4])
				}
			}
		}

		if !strings.HasPrefix(vf.Name, "[R3]C5") {
			continue
		}

		gf, ok := govaxFiles[vf.Name]
		if !ok {
			t.Errorf("%s: VMS made it, govax didn't", vf.Name)

			continue
		}

		compared++

		compareFDLFile(t, vf, gf)
	}

	dispatch("CREATE/FDL=[V]A35")

	listing, _, err := c.ContainerSession.Directory("DUA1:[IN]S35.IDX;*", rms.DirectoryOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(listing, "S35.IDX;2") {
		t.Errorf("CREATE/FDL=A35 made no [IN]S35.IDX;2:\n%s", listing)
	}

	return compared
}

// expectedFDLDifferences are the CREATE/FDL commands whose output differs
// from VMS's, and expectedFDLFiles the files VMS made that govax doesn't:
// Prologs 1 and 2, which govax refuses with RMS$_PLV (the author's answer
// 1). C3A asked for PROLOG 2, C3B for PROLOG 1, and C3C's primary key's
// segments overlap, which only those can hold.
var (
	expectedFDLDifferences = map[string]bool{
		"$ CREATE/FDL=C3A C3A.IDX": true,
		"$ CREATE/FDL=C3B C3B.IDX": true,
		"$ CREATE/FDL=C3C C3C.IDX": true,
	}

	expectedFDLFiles = map[string]bool{"[R3]C3A.IDX;1": true, "[R3]C3B.IDX;1": true, "[R3]C3C.IDX;1": true}
)

// fdlMadeFile is the name of a file CREATE/FDL made in sections C2 to C4:
// C21.DAT to C4J.DAT, C31.IDX to C3I.IDX, and the C2B names and ".".
var fdlMadeFile = regexp.MustCompile(`^\[R3\](C[234][0-9A-Z]\.(DAT|IDX)|C2B[A-Z]+\.[A-Z]+|\.);\d+$`)

// fdlCase is one CREATE/FDL command of sections C2 to C4 and the lines
// after it, up to the next CREATE/FDL or the end of C4.
type fdlCase struct {
	command string
	lines   []string
}

// fdlCases splits a round 3 log's sections C2 to C4 into their CREATE/FDL
// cases, carriage returns gone, times and file IDs masked, and
// ANALYZE/RMS_FILE's reports (the lines after its command up to the next
// "$" line) left out.
func fdlCases(text string) []fdlCase {
	var (
		cases    []fdlCase
		on, skip bool
	)

	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r", ""), "\n") {
		switch {
		case strings.HasPrefix(line, `$ M = "C2"`):
			on = true
		case strings.HasPrefix(line, `$ M = "C5"`):
			return cases
		}

		if !on {
			continue
		}

		if strings.HasPrefix(line, "$ ") {
			skip = strings.HasPrefix(line, "$ ANALYZE/RMS_FILE")
		} else if skip {
			continue
		}

		if strings.HasPrefix(line, "$ CREATE/FDL") {
			cases = append(cases, fdlCase{command: line})

			continue
		}

		if len(cases) == 0 {
			continue
		}

		line = idx88DTimes.ReplaceAllString(line, "<time>")
		line = fdlFileID.ReplaceAllString(line, "(fid)")

		if strings.Contains(line, "(fid)") {
			line = strings.TrimRight(line, " ")
		}

		cases[len(cases)-1].lines = append(cases[len(cases)-1].lines, line)
	}

	return cases
}

// fdlFileID is a file ID in DIRECTORY/FULL's listing.
var fdlFileID = regexp.MustCompile(`\(\d+,\d+,\d+\)`)

// readSaveSet reads a save set on the host.
func readSaveSet(t *testing.T, path string) *backup.SaveSet {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	s, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	return s
}

// compareFDLFile compares a file CREATE/FDL made on govax with VMS's: its
// allocation, owner, protection, characteristics, record attributes, and
// revision, and its blocks.
func compareFDLFile(t *testing.T, vms, govax *backup.File) {
	t.Helper()

	for _, f := range []struct {
		what      string
		vms, ours uint64
	}{
		{"allocation", uint64(vms.Allocated), uint64(govax.Allocated)},
		{"owner", uint64(vms.Owner), uint64(govax.Owner)},
		{"protection", uint64(vms.Protection), uint64(govax.Protection)},
		{"characteristics", uint64(vms.Characteristics), uint64(govax.Characteristics)},
		{"revision", uint64(vms.Revision), uint64(govax.Revision)},
		{"expiration", vms.Expires, govax.Expires},
	} {
		if f.vms != f.ours {
			t.Errorf("%s: %s %#x on VMS, %#x on govax", vms.Name, f.what, f.vms, f.ours)
		}
	}

	if vms.RecordAttributes != govax.RecordAttributes {
		t.Errorf("%s: record attributes\nVMS:   %s\ngovax: %s", vms.Name,
			hexenc.EncodeToString(vms.RecordAttributes[:]), hexenc.EncodeToString(govax.RecordAttributes[:]))
	}

	// A file CREATE/FDL's DATE section dated keeps its creation date.
	if vms.Name == "[R3]C2E.DAT;1" && vms.Created != govax.Created {
		t.Errorf("%s: created %#x on VMS, %#x on govax", vms.Name, vms.Created, govax.Created)
	}

	if a, b := vms.Data(), govax.Data(); !bytes.Equal(a, b) {
		for i := 0; i < max(len(a), len(b)); i++ {
			if i >= len(a) || i >= len(b) || a[i] != b[i] {
				t.Errorf("%s: blocks differ at byte %d (block %d, offset %d); %d bytes on VMS, %d on govax\nVMS:   %s\ngovax: %s",
					vms.Name, i, i/512+1, i%512, len(a), len(b),
					hexenc.EncodeToString(a[i/16*16:min(len(a), i/16*16+32)]), hexenc.EncodeToString(b[i/16*16:min(len(b), i/16*16+32)]))

				break
			}
		}
	}
}

// TestCreateFDLForms is the command's forms VMS's run doesn't have: /FDL
// anywhere on the line, an FDL file in a directory that isn't there, and a
// file to make in one.
func TestCreateFDLForms(t *testing.T) {
	_, c := runIdx88C(t)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	for _, line := range []string{"CREATE/LOG/FDL=[R3]C31 F1.IDX", "CREATE F2.IDX/FDL=[R3]C31", "CREATE/FDL=[R3]C31 F3.IDX/LOG"} {
		if err := d.Dispatch(line); err != nil {
			t.Errorf("%s: %v", line, err)
		}
	}

	for _, name := range []string{"F1", "F2", "F3"} {
		if _, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: name + ".IDX"}); err != nil {
			t.Errorf("%s.IDX: %v", name, err)
		}
	}

	err := d.Dispatch("CREATE/FDL=[NOSUCH]C31 F4.IDX")
	if err == nil || !strings.Contains(err.Error(), "RMS-E-DNF") || !strings.Contains(err.Error(), "OPENFDL") {
		t.Errorf("a missing directory's FDL file: %v", err)
	}

	err = d.Dispatch("CREATE/FDL=[R3]C31 [NOSUCH]F5.IDX")
	if err == nil || !strings.Contains(err.Error(), "FDL-F-CREATE") || !strings.Contains(err.Error(), "RMS-E-DNF") {
		t.Errorf("a missing directory's file: %v", err)
	}
}
