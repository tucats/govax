package isam

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
)

// The tests in this file hold the reader to what VMS made and said:
// testdata/idx88/vax has the probe's files in a save set, ANALYZE/RMS_FILE's
// report on each, and the log of the program that made and read them. See
// that directory's README.md, and the phase doc's "What round 1 settled".
const vaxDir = "../../testdata/idx88/vax"

// memoryBlocks is a file held in memory: every test's Blocks.
type memoryBlocks []byte

func (m memoryBlocks) ReadBlock(vbn uint32, block []byte) error {
	at := int(vbn-1) * BlockSize
	if vbn == 0 || at+len(block) > len(m) {
		return fmt.Errorf("VBN %d is past the file's end", vbn)
	}

	copy(block, m[at:])

	return nil
}

// vmsFile is one file from the save set, not yet opened.
type vmsFile struct {
	name   string // S37, say; E103;1 for E103's older version
	blocks memoryBlocks
	attr   Attributes
}

// attributesOf reads what isam needs from a file's header: the record
// attributes area, whose first byte has the format in its low 4 bits and
// whose word at 16 is the longest record.
func attributesOf(fat [32]byte) Attributes {
	attr := Attributes{Size: int(fat[16]) | int(fat[17])<<8}
	if fat[0]&0x0F == 1 {
		attr.Format = Fixed
	}

	return attr
}

// readSaveSet returns the files of a save set in testdata.
func readSaveSet(t *testing.T, path string) []*vmsFile {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	set, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	var files []*vmsFile

	for _, f := range set.Files {
		name := f.Name[strings.LastIndex(f.Name, "]")+1:]
		base, version, _ := strings.Cut(name, ";")
		base = strings.TrimSuffix(base, ".IDX")

		// E103's two versions are the one case with two: FOP CIF, then
		// $CREATE again without it. The older gets its version in its name.
		if base == "E103" && version == "1" {
			base = "E103;1"
		}

		files = append(files, &vmsFile{name: base, blocks: f.Data(), attr: attributesOf(f.RecordAttributes)})
	}

	return files
}

// probeFiles is every file of the probe's save set.
func probeFiles(t *testing.T) []*vmsFile {
	t.Helper()

	return readSaveSet(t, filepath.Join(vaxDir, "idx88.bck"))
}

// openProbe opens one of the probe's files by name.
func openProbe(t *testing.T, name string) *File {
	t.Helper()

	for _, v := range probeFiles(t) {
		if v.name == name {
			f, err := Open(v.blocks, v.attr)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}

			return f
		}
	}

	t.Fatalf("%s isn't in the save set", name)

	return nil
}

// unreadable lists the files Open must refuse, and why, by their prolog level:
// E85's $CREATE failed and left a prolog of level 0, which VMS's RMS calls a level
// it doesn't support (RMS$_PLV, testdata/vms88), and the others are Prolog 1 and 2,
// the first being what a lower level's keys or MRS gives (the phase doc's Part A).
var unreadable = map[string]int{
	"E85": 0, "E92": 1, "S34": 1, "S59": 1, "S60": 1, "S61": 2,
}

// TestOpenRefuses holds Open to what it can't read.
func TestOpenRefuses(t *testing.T) {
	seen := 0

	for _, v := range probeFiles(t) {
		level, refused := unreadable[v.name]

		_, err := Open(v.blocks, v.attr)
		if !refused {
			if err != nil {
				t.Errorf("%s: %v", v.name, err)
			}

			continue
		}

		seen++

		var wrong *ErrPrologLevel

		switch {
		case !errors.As(err, &wrong):
			t.Errorf("%s: %v, not ErrPrologLevel", v.name, err)
		case wrong.Version != level:
			t.Errorf("%s: level %d, not %d", v.name, wrong.Version, level)
		}
	}

	if seen != len(unreadable) {
		t.Errorf("found %d of the %d files to refuse", seen, len(unreadable))
	}
}

// readable opens every Prolog 3 file of the probe.
func readable(t *testing.T) map[string]*File {
	t.Helper()

	files := map[string]*File{}

	for _, v := range probeFiles(t) {
		if _, refused := unreadable[v.name]; refused {
			continue
		}

		f, err := Open(v.blocks, v.attr)
		if err != nil {
			t.Fatalf("%s: %v", v.name, err)
		}

		files[v.name] = f
	}

	return files
}

// Patterns in ANALYZE/RMS_FILE's report on a file's prolog.
var (
	anaAreaHead = regexp.MustCompile(`^AREA DESCRIPTOR #(\d+) \(VBN (\d+)`)
	anaKeyHead  = regexp.MustCompile(`^KEY DESCRIPTOR #(\d+) \(VBN (\d+), offset %X'([0-9A-F]+)'\)`)
	anaField    = regexp.MustCompile(`([A-Z][A-Za-z0-9 ]*?): +("[^"]*"|%X'[0-9A-F]+'|[a-z ]+[a-z]|\d+)`)
	anaFlag     = regexp.MustCompile(`\(\d\)  KEY\$V_(\w+) +([01])`)
	anaNumbers  = regexp.MustCompile(`\d+`)
)

// anaTypes is ANALYZE's name for each key type.
var anaTypes = map[string]Type{
	"string": String, "signed word": Int2, "unsigned word": Bin2,
	"signed longword": Int4, "unsigned longword": Bin4, "packed decimal": Packed,
	"signed quadword": Int8, "unsigned quadword": Bin8,
}

// anaFlags is ANALYZE's name for each key flag.
var anaFlags = map[string]uint8{
	"DUPKEYS": FlagDuplicates, "CHGKEYS": FlagChanges, "NULKEYS": FlagNull,
	"IDX_COMPR": FlagIndexCompress, "INITIDX": FlagNoIndexYet,
	"KEY_COMPR": FlagKeyCompress, "REC_COMPR": FlagRecordCompress,
}

// anaNumber reads a report's number, decimal or %X'...'.
func anaNumber(text string) int {
	base := 10

	if strings.HasPrefix(text, "%X'") {
		text = strings.Trim(text[2:], "'")
		base = 16
	}

	n, err := strconv.ParseInt(text, base, 64)
	if err != nil {
		return -1
	}

	return int(n)
}

// TestPrologAgainstANALYZE holds every field Open reads to VMS's
// ANALYZE/RMS_FILE/CHECK report on the same file.
func TestPrologAgainstANALYZE(t *testing.T) {
	compared := 0

	for name, f := range readable(t) {
		text, err := os.ReadFile(filepath.Join(vaxDir, "anl", strings.ToLower(name)+".chk"))
		if err != nil {
			continue // only the newest version of a file has a report
		}

		compared++

		compareReport(t, name, f, string(text))
	}

	if compared < 100 {
		t.Errorf("compared only %d reports", compared)
	}
}

// compareReport compares one report with one file.
func compareReport(t *testing.T, name string, f *File, text string) {
	t.Helper()

	var k *Key

	var a *Area

	wrong := func(what string, got, want any) {
		t.Helper()
		t.Errorf("%s: %s: got %v, VMS has %v", name, what, got, want)
	}

	same := func(what string, got int, text string) {
		t.Helper()

		if want := anaNumber(text); got != want {
			wrong(what, got, want)
		}
	}

	var positions []string

	flagsSeen := map[uint8]bool{}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)

		if m := anaAreaHead.FindStringSubmatch(trimmed); m != nil {
			k, a = nil, &f.Areas[anaNumber(m[1])]

			continue
		}

		if m := anaKeyHead.FindStringSubmatch(trimmed); m != nil {
			a, k = nil, f.Keys[anaNumber(m[1])]
			same("descriptor VBN", int(k.DescriptorVBN), m[2])
			same("descriptor offset", k.DescriptorOffset, "%X'"+m[3]+"'")

			continue
		}

		if m := anaFlag.FindStringSubmatch(trimmed); m != nil && k != nil {
			bit := anaFlags[m[1]]
			flagsSeen[bit] = true

			if (k.Flags&bit != 0) != (m[2] == "1") {
				wrong(fmt.Sprintf("key %d %s", k.Ref, m[1]), k.Flags&bit != 0, m[2])
			}

			continue
		}

		if strings.HasPrefix(trimmed, "Segment Positions:") {
			positions = anaNumbers.FindAllString(trimmed, -1)

			continue
		}

		if strings.HasPrefix(trimmed, "Segment Sizes:") && k != nil {
			sizes := anaNumbers.FindAllString(trimmed, -1)
			if len(sizes) != len(k.Segments) || len(positions) != len(k.Segments) {
				wrong(fmt.Sprintf("key %d segments", k.Ref), len(k.Segments), len(sizes))

				continue
			}

			for i, s := range k.Segments {
				same("segment position", s.Position, positions[i])
				same("segment size", s.Size, sizes[i])
			}

			continue
		}

		for _, m := range anaField.FindAllStringSubmatch(line, -1) {
			switch {
			case m[1] == "Number of Areas":
				same(m[1], len(f.Areas), m[2])
			case m[1] == "VBN of First Descriptor":
				same(m[1], int(f.AreaVBN), m[2])
			case m[1] == "Prolog Version":
				same(m[1], f.Version, m[2])
			case a != nil:
				compareAreaField(same, a, m[1], m[2])
			case k != nil:
				compareKeyField(t, name, same, k, m[1], m[2])
			}
		}
	}
}

// compareAreaField compares one of a report's area fields.
func compareAreaField(same func(string, int, string), a *Area, field, value string) {
	switch field {
	case "Bucket Size":
		same(field, a.BucketSize, value)
	case "Reclaimed Bucket VBN":
		same(field, int(a.Available), value)
	case "Current Extent Start":
		same(field, int(a.ExtentVBN), value)
	case "Blocks":
		same(field, int(a.ExtentSize), value)
	case "Used":
		same(field, int(a.Used), value)
	case "Next":
		same(field, int(a.NextVBN), value)
	case "Default Extend Quantity":
		same(field, a.ExtendSize, value)
	case "Total Allocation":
		same(field, int(a.TotalAllocation), value)
	}
}

// compareKeyField compares one of a report's key fields.
func compareKeyField(t *testing.T, name string, same func(string, int, string), k *Key, field, value string) {
	t.Helper()

	switch field {
	case "Next Key Descriptor VBN":
		same(field, int(k.NextDescriptorVBN), value)
	case "Offset":
		same(field, k.NextDescriptorOffset, value)
	case "Index Area":
		same(field, k.IndexArea, value)
	case "Level 1 Index Area":
		same(field, k.Level1Area, value)
	case "Data Area":
		same(field, k.DataArea, value)
	case "Root Level":
		same(field, k.RootLevel, value)
	case "Index Bucket Size":
		same(field, k.IndexBucketSize, value)
	case "Data Bucket Size":
		same(field, k.DataBucketSize, value)
	case "Root VBN":
		same(field, int(k.RootVBN), value)
	case "Key Segments":
		same(field, len(k.Segments), value)
	case "Key Size":
		same(field, k.Size, value)
	case "Minimum Record Size":
		same(field, k.MinRecord, value)
	case "Index Fill Quantity":
		same(field, k.IndexFill, value)
	case "Data Fill Quantity":
		same(field, k.DataFill, value)
	case "First Data Bucket VBN":
		same(field, int(k.FirstData), value)
	case "Null Character":
		same(field, int(k.Null), value)
	case "Name":
		if want := strings.Trim(value, `"`); k.Name != strings.TrimRight(want, " \x00") {
			t.Errorf("%s: key %d name %q, VMS has %q", name, k.Ref, k.Name, want)
		}
	case "Data Type":
		kind, descending := strings.CutPrefix(value, "descending ")

		want, known := anaTypes[kind]
		if !known {
			t.Errorf("%s: key %d: a type ANALYZE names %q", name, k.Ref, value)

			return
		}

		if descending {
			want |= Descending
		}

		if k.Type != want {
			t.Errorf("%s: key %d type %02X, VMS has %q", name, k.Ref, uint8(k.Type), value)
		}
	}
}
