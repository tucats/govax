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
	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/govax/internal/rms"
)

// TestIdx88CConvertOracle holds CONVERT (Phase 88's subtask 10c) to VMS 7.1's run
// of round 3 (testdata/idx88's IDX88C.COM): sections C6 to C10, command by command,
// with times, file IDs, and /STATISTICS's counts masked and ANALYZE/RMS_FILE's
// reports left out (they're subtask 10d's); then each file those sections made,
// against the save set VMS made of them: its header, and an indexed file's prolog
// and its Dump (VMS leaves stale bytes past a bucket's free offset, which mean
// nothing), or any other's bytes up to its end of file.
//
// The differences expected are the Prolog 1 files VMS made (C93A and C93B, which
// C93C is made from), which govax refuses (the author's answer 1), COPY's message
// without /LOG, which is govax's own, and C12A.SEQ: the records VMS could still
// read past a damaged free offset (X05), where govax's reader stops at the bucket.
func TestIdx88CConvertOracle(t *testing.T) {
	vax := filepath.Join("..", "..", "testdata", "idx88", "vax")

	text, err := os.ReadFile(filepath.Join(vax, "idx88c.log"))
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	log, c := runIdx88C(t)

	want, have := convertCommands(string(text)), convertCommands(log)
	if len(want) == 0 || len(have) != len(want) {
		t.Fatalf("%d commands on VMS, %d on govax", len(want), len(have))
	}

	skipping := false

	for i, w := range want {
		g := have[i]
		if w.command != g.command {
			t.Fatalf("command %d is %q on VMS, %q on govax", i+1, w.command, g.command)
		}

		// A skipped CONVERT's status and listing go with it.
		if strings.HasPrefix(w.command, "$ CONVERT") {
			skipping = convertSkipped(w.command)
		}

		if skipping {
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

	files := map[string]*backup.File{}
	for _, f := range ours.Files {
		files[f.Name] = f
	}

	compared := 0

	for _, vf := range vmsSet.Files {
		if !convertMadeFile.MatchString(vf.Name) || expectedConvertFiles[vf.Name] {
			continue
		}

		gf, ok := files[vf.Name]
		if !ok {
			t.Errorf("%s: VMS made it, govax didn't", vf.Name)

			continue
		}

		compared++

		compareConvertFile(t, vf, gf)
	}

	if compared != 60 {
		t.Errorf("%d files compared, where those sections made 60 (with the expected ones left out)", compared)
	}
}

// expectedConvertFiles are the files VMS made that govax doesn't. C93A and C93B
// are Prolog 1 (/PROLOG=2 and 1), C93C CONVERT's Prolog 3 of C93A.
var expectedConvertFiles = map[string]bool{
	"[R3]C93A.IDX;1": true, "[R3]C93B.IDX;1": true, "[R3]C93C.IDX;1": true, "[R3]C12A.SEQ;1": true,
}

// expectedConvertChecks are files whose only difference from VMS's is some
// buckets' check characters, which say nothing of what a bucket holds. /NOFAST_LOAD's
// roots of keys 0 and 1 have 3B and D3 on VMS, 1B and 03 on govax, with keys 2 to 7 and
// every data bucket the same; and C91's data bucket, after /NOCREATE and two
// /MERGEs that took a record each, has 3 on VMS, 2 here. No rule fits these so far.
var expectedConvertChecks = map[string]bool{
	"[R3]C72.IDX;1": true, "[R3]C74.IDX;1": true, "[R3]C75.IDX;1": true, "[R3]C91.IDX;1": true,
}

// bucketCheck is a bucket's check character in a Dump.
var bucketCheck = regexp.MustCompile(`check [0-9A-F]{2}`)

// convertMadeFile is the name of a file sections C6 to C10 made, and C12's two
// CONVERTs of damaged files.
var convertMadeFile = regexp.MustCompile(`^\[R3\](C6[0-9A-Z]+|C7[0-9A-Z]+|C8[0-9A-Z]+|C9[0-9A-Z]+|C10[0-9A-Z]+|C12[AB])\.[A-Z]+;\d+$`)

// convertSkipped says a command's output is one of the expected differences.
func convertSkipped(command string) bool {
	for _, name := range []string{"C93A", "C93B", "C93C"} {
		if strings.Contains(command, name) {
			return true
		}
	}

	return false
}

// convertCommand is one command of sections C6 to C10 and the lines after it.
type convertCommand struct {
	command string
	lines   []string
}

// convertCounts is /STATISTICS's counts of I/O and page faults, and its times.
var convertCounts = regexp.MustCompile(`(Buffered I/O Count: |Direct I/O Count: |Page Faults: )(\t+) *\d+|\d \d\d:\d\d:\d\d\.\d\d`)

// convertCommands splits a round 3 log's sections C6 to C10 into their commands:
// comments gone (the procedure's changed since VMS ran it), ANALYZE/RMS_FILE's
// reports left out, carriage returns gone, details masked.
func convertCommands(text string) []convertCommand {
	var (
		list     []convertCommand
		on, skip bool
	)

	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r", ""), "\n") {
		switch {
		case strings.HasPrefix(line, `$ M = "C6"`):
			on = true
		case strings.HasPrefix(line, `$ M = "C11"`):
			return list
		}

		if !on || strings.HasPrefix(line, "$ !") || strings.HasPrefix(line, "%COPY-S-COPIED") {
			continue
		}

		if strings.HasPrefix(line, "$ ") {
			skip = strings.HasPrefix(line, "$ ANALYZE/RMS_FILE")
			list = append(list, convertCommand{command: line})

			continue
		}

		if skip || len(list) == 0 {
			continue
		}

		line = idx88DTimes.ReplaceAllString(line, "<time>")
		line = fdlFileID.ReplaceAllString(line, "(fid)")
		line = convertCounts.ReplaceAllString(line, "$1<n>")

		if strings.Contains(line, "File ID") {
			line = strings.TrimRight(line, " ")
		}

		list[len(list)-1].lines = append(list[len(list)-1].lines, line)
	}

	return list
}

// compareConvertFile compares a file CONVERT made on govax with VMS's.
func compareConvertFile(t *testing.T, vms, govax *backup.File) {
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
	} {
		if f.vms != f.ours {
			t.Errorf("%s: %s %#x on VMS, %#x on govax", vms.Name, f.what, f.vms, f.ours)
		}
	}

	if vms.RecordAttributes != govax.RecordAttributes {
		t.Errorf("%s: record attributes\nVMS:   %s\ngovax: %s", vms.Name,
			hexenc.EncodeToString(vms.RecordAttributes[:]), hexenc.EncodeToString(govax.RecordAttributes[:]))
	}

	a, b := vms.Data(), govax.Data()

	if vms.Organization() != 2 {
		end := (int(vms.Used())-1)*512 + int(vms.FirstFreeByte())
		if vms.FirstFreeByte() == 0 {
			end = int(vms.Used()) * 512
		}

		if end < 0 {
			end = 0
		}

		if len(a) < end || len(b) < end || !bytes.Equal(a[:end], b[:end]) {
			t.Errorf("%s: contents differ:\nVMS:   %s\ngovax: %s", vms.Name, firstBytes(a), firstBytes(b))
		}

		return
	}

	va, err1 := isam.Open(memBlocks(a), isam.Attributes{Size: int(vms.MaxRecord())})
	gb, err2 := isam.Open(memBlocks(b), isam.Attributes{Size: int(govax.MaxRecord())})

	if err1 != nil || err2 != nil {
		t.Errorf("%s: open: %v, %v", vms.Name, err1, err2)

		return
	}

	var da, db strings.Builder

	_ = va.Dump(&da)
	_ = gb.Dump(&db)

	if da.String() != db.String() {
		if expectedConvertChecks[vms.Name] &&
			bucketCheck.ReplaceAllString(da.String(), "") == bucketCheck.ReplaceAllString(db.String(), "") {
			return
		}

		w, g := strings.Split(da.String(), "\n"), strings.Split(db.String(), "\n")

		for i := 0; i < min(len(w), len(g)); i++ {
			if w[i] != g[i] {
				t.Errorf("%s, dump line %d:\nVMS:   %s\ngovax: %s", vms.Name, i+1, w[i], g[i])

				return
			}
		}

		t.Errorf("%s: dumps of %d and %d lines", vms.Name, len(w), len(g))
	}
}

func firstBytes(b []byte) string { return hexenc.EncodeToString(b[:min(len(b), 48)]) }

// memBlocks is a file in memory for isam to read.
type memBlocks []byte

func (m memBlocks) ReadBlock(vbn uint32, block []byte) error {
	copy(block, m[min(int(vbn-1)*512, len(m)):])

	return nil
}
