package isam

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites the golden dumps: go test ./internal/isam -update.
var update = flag.Bool("update", false, "rewrite the golden dumps in testdata")

// TestCompare holds each type's order, both ways.
func TestCompare(t *testing.T) {
	word := func(v int16) []byte { return []byte{byte(v), byte(uint16(v) >> 8)} }
	long := func(v int32) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }
	quad := func(v int64) []byte {
		b := make([]byte, 8)
		for i := range b {
			b[i] = byte(v >> (8 * i))
		}

		return b
	}

	tests := []struct {
		name string
		typ  Type
		low  []byte
		high []byte
	}{
		{"strings", String, []byte("ABC"), []byte("ABD")},
		{"strings compare unsigned", String, []byte{0x7F}, []byte{0x80}},
		{"IN2 signs", Int2, word(-1), word(1)},
		{"BN2 unsigned", Bin2, word(1), word(-1)},
		{"BN2's high byte counts most", Bin2, []byte{0xFF, 0}, []byte{0, 1}},
		{"IN4 signs", Int4, long(-5), long(-4)},
		{"BN4 unsigned", Bin4, long(0x7FFFFFFF), long(-1)},
		{"IN8 signs", Int8, quad(-1), quad(0)},
		{"BN8 unsigned", Bin8, quad(1 << 40), quad(-1)},
		{"packed", Packed, []byte{0x01, 0x2C}, []byte{0x01, 0x3C}},
		{"packed signs", Packed, []byte{0x01, 0x2D}, []byte{0x00, 0x1C}},
		{"packed negatives run backward", Packed, []byte{0x01, 0x3D}, []byte{0x01, 0x2B}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			k := &Key{Type: test.typ}
			if k.Compare(test.low, test.high) >= 0 || k.Compare(test.high, test.low) <= 0 {
				t.Errorf("%X isn't before %X", test.low, test.high)
			}

			if k.Compare(test.low, test.low) != 0 {
				t.Errorf("%X doesn't equal itself", test.low)
			}

			k.Type |= Descending
			if k.Compare(test.low, test.high) <= 0 {
				t.Errorf("descending: %X isn't after %X", test.low, test.high)
			}
		})
	}

	k := &Key{Type: Packed}
	if k.Compare([]byte{0x00, 0x0D}, []byte{0x00, 0x0C}) != 0 {
		t.Error("-0 and +0 differ")
	}
}

// TestValue takes keys out of records, joined and refused.
func TestValue(t *testing.T) {
	k := &Key{
		Type: String, Size: 4, MinRecord: 8,
		Segments: []Segment{{Position: 6, Size: 2}, {Position: 1, Size: 2}},
	}

	got, ok := k.Value([]byte("abcdefgh"))
	if !ok || string(got) != "ghbc" {
		t.Errorf("got %q, %v", got, ok)
	}

	if _, ok := k.Value([]byte("abcdefg")); ok {
		t.Error("a record short of the key held it")
	}
}

// TestIsNull is the two null values.
func TestIsNull(t *testing.T) {
	text := &Key{Type: String, Flags: FlagNull, Null: ' '}
	number := &Key{Type: Bin4, Flags: FlagNull, Null: ' '}
	neither := &Key{Type: String, Null: ' '}

	tests := []struct {
		key   *Key
		value string
		want  bool
	}{
		{text, "    ", true},
		{text, "  x ", false},
		{number, "\x00\x00\x00\x00", true},
		{number, "    ", false},
		{neither, "    ", false},
	}

	for _, test := range tests {
		if got := test.key.IsNull([]byte(test.value)); got != test.want {
			t.Errorf("%q: got %v", test.value, got)
		}
	}
}

// TestAssemble puts a segmented key back where its record had it.
func TestAssemble(t *testing.T) {
	k := &Key{Size: 3, Segments: []Segment{{Position: 4, Size: 2}, {Position: 1, Size: 1}}}

	if got := string(k.assemble([]byte("EFB"), []byte("ACDGH"))); got != "ABCDEFGH" {
		t.Errorf("got %q", got)
	}
}

// TestKeyValueSizes refuses search values of the wrong size.
func TestKeyValueSizes(t *testing.T) {
	f := openProbe(t, "F122")

	for _, test := range []struct {
		ref   int
		value string
		want  error
	}{
		{0, "", ErrKeySize},
		{0, "P00000001", ErrKeySize},
		{1, "ab", ErrKeySize}, // a part of a number
		{8, "P", ErrKeyRef},
		{-1, "P", ErrKeyRef},
		{0, "Z", ErrNotFound},
	} {
		if _, err := f.Find(test.ref, []byte(test.value), Equal); !errors.Is(err, test.want) {
			t.Errorf("key %d, %q: %v", test.ref, test.value, err)
		}
	}

	if err := f.Walk(9, func(Entry) bool { return true }); !errors.Is(err, ErrKeyRef) {
		t.Errorf("Walk: %v", err)
	}

	if _, err := f.First(9); !errors.Is(err, ErrKeyRef) {
		t.Errorf("First: %v", err)
	}
}

// TestEmptyFile reads files with no records: S2, whose index VMS made at
// $CREATE, and S47, whose alternate no record is long enough for.
func TestEmptyFile(t *testing.T) {
	f := openProbe(t, "S2")

	if _, err := f.First(0); !errors.Is(err, ErrEnd) {
		t.Errorf("First: %v", err)
	}

	if _, err := f.Find(0, []byte("P"), EqualOrNext); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find: %v", err)
	}

	if _, err := f.Find(0, []byte("P"), Previous); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find backward: %v", err)
	}

	f = openProbe(t, "S47")

	if _, err := f.First(1); !errors.Is(err, ErrEnd) {
		t.Errorf("First of an unused alternate: %v", err)
	}

	value := make([]byte, f.Keys[1].Size)
	if _, err := f.Find(1, value, Equal); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find in an unused alternate: %v", err)
	}

	if _, err := f.Following(Entry{Ref: 1, Key: value}); !errors.Is(err, ErrEnd) {
		t.Errorf("Following in an unused alternate: %v", err)
	}
}

// TestNoIndexYet reads a key whose index no record has made, as far as a
// file on hand has one: the flag says so, and the root field isn't a VBN.
func TestNoIndexYet(t *testing.T) {
	k := parseKey(append(make([]byte, keyFlags), append([]byte{FlagNoIndexYet}, make([]byte, keyDescSize)...)...))
	if k.HasIndex() {
		t.Error("a key flagged as having no index has one")
	}

	f := &File{Keys: []*Key{k}}

	if _, err := f.First(0); !errors.Is(err, ErrEnd) {
		t.Errorf("First: %v", err)
	}

	if _, err := f.Find(0, []byte("A"), EqualOrNext); !errors.Is(err, ErrKeySize) {
		t.Errorf("Find of a key with no size: %v", err)
	}

	k.Size = 1

	if _, err := f.Find(0, []byte("A"), EqualOrNext); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find: %v", err)
	}

	if _, err := f.Following(Entry{Key: []byte("A")}); !errors.Is(err, ErrEnd) {
		t.Errorf("Following: %v", err)
	}
}

// TestRecordDeleted is VMS's run of case 147: a record split away from its
// bucket, then deleted, whose RFA is RMS$_DEL (the log's last $GET), while a
// neighbor split the same way is found through its RRV. An ID the bucket
// never gave out reads as gone too.
func TestRecordDeleted(t *testing.T) {
	f := openProbe(t, "U137")

	if _, err := f.Record(RFA{VBN: 4, ID: 2}); !errors.Is(err, ErrDeleted) {
		t.Errorf("a deleted record: %v", err)
	}

	r, err := f.Record(RFA{VBN: 4, ID: 1})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(r, []byte("P0000010")) {
		t.Errorf("RFA 4,1 holds %q", r[:8])
	}

	if _, err := f.Record(RFA{VBN: 4, ID: 60000}); !errors.Is(err, ErrDeleted) {
		t.Errorf("an ID never given: %v", err)
	}
}

// TestSystemFile reads the file CONVERT made for BACKUP's probe, a file
// not of this probe's making.
func TestSystemFile(t *testing.T) {
	var found *vmsFile

	for _, v := range readSaveSet(t, "../../testdata/backup69/vax/b69.bck") {
		if v.name == "INDEXED" {
			found = v
		}
	}

	if found == nil {
		t.Fatal("INDEXED.IDX isn't in the save set")
	}

	f, err := Open(found.blocks, found.attr)
	if err != nil {
		t.Fatal(err)
	}

	list := entriesOf(t, "INDEXED", f, 0)
	if len(list) == 0 {
		t.Fatal("no records")
	}

	checkOrder(t, "INDEXED", f.Keys[0], list)

	for _, e := range list {
		r, err := f.Record(e.RFA)
		if err != nil {
			t.Fatal(err)
		}

		if value, ok := f.Keys[0].Value(r); !ok || !bytes.Equal(value, e.Key) {
			t.Errorf("%v: value %q, key %q", e.RFA, value, e.Key)
		}
	}
}

// damaged returns a copy of a file with one byte changed.
func damaged(v *vmsFile, offset int, value byte) memoryBlocks {
	blocks := bytes.Clone(v.blocks)
	blocks[offset] = value

	return blocks
}

// TestCorrupt reads damaged files: a wrong answer would be worse than
// an error, and a panic worst.
func TestCorrupt(t *testing.T) {
	var source *vmsFile

	for _, v := range probeFiles(t) {
		if v.name == "S35" {
			source = v
		}
	}

	f, err := Open(source.blocks, source.attr)
	if err != nil {
		t.Fatal(err)
	}

	first := int(f.Keys[0].FirstData-1) * BlockSize
	size := f.Keys[0].DataBucketSize * BlockSize

	for _, test := range []struct {
		name   string
		offset int
		value  byte
	}{
		{"a check character", first, 0xEE},
		{"the VBN sample", first + bktSample, 0xEE},
		{"the free offset", first + bktFree + 1, 0xEE},
		{"a record's control byte", first + bktOverhead, 0x08},
		{"a record's size", first + bktOverhead + 10, 0xFF},
		{"a compressed key's count", first + bktOverhead + 11, 0xFF},
		{"the last byte", first + size - 1, 0xEE},
	} {
		t.Run(test.name, func(t *testing.T) {
			g, err := Open(damaged(source, test.offset, test.value), source.attr)
			if err != nil {
				t.Fatal(err)
			}

			walked := g.Walk(0, func(Entry) bool { return true })
			if walked == nil {
				t.Errorf("no error from the walk")
			}

			var corrupt *CorruptError
			if walked != nil && !errors.As(walked, &corrupt) {
				t.Errorf("%v isn't a CorruptError", walked)
			}
		})
	}

	// A prolog's sum is checked in every block of descriptors.
	if _, err := Open(damaged(source, 3, 0xEE), source.attr); !errors.Is(err, ErrBadProlog) {
		t.Errorf("key 0's block: %v", err)
	}

	var corrupt *CorruptError

	if _, err := Open(damaged(source, BlockSize+3, 0xEE), source.attr); !errors.As(err, &corrupt) {
		t.Errorf("the next block: %v", err)
	}

	if _, err := Open(memoryBlocks(source.blocks[:BlockSize]), source.attr); err == nil {
		t.Error("a file cut short opened")
	}

	if _, err := Open(source.blocks, Attributes{Format: Fixed}); !errors.Is(err, ErrBadProlog) {
		t.Errorf("fixed records of no size: %v", err)
	}
}

// goldenDumps are small files showing each thing Dump shows: compressed
// records (S1), fixed ones (S13), named keys and SIDRs (S37), three areas
// (S62), RRVs and an index of several entries (G105), duplicate alternates
// across data buckets (G111), and a deleted record's gap (U134).
var goldenDumps = []string{"S1", "S13", "S37", "S62", "G105", "G111", "U134"}

// TestDump keeps Dump's output, which later subtasks compare files with.
func TestDump(t *testing.T) {
	for _, name := range goldenDumps {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer

			if err := openProbe(t, name).Dump(&out); err != nil {
				t.Fatal(err)
			}

			path := filepath.Join("testdata", name+".dump")

			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}

				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(out.Bytes(), want) {
				t.Errorf("the dump differs from %s:\n%s", path, out.String())
			}
		})
	}
}

// TestDumpEveryFile dumps every file: nothing in a VMS file may stop it.
func TestDumpEveryFile(t *testing.T) {
	for name, f := range readable(t) {
		var out bytes.Buffer

		if err := f.Dump(&out); err != nil {
			t.Errorf("%s: %v", name, err)
		}

		if out.Len() == 0 {
			t.Errorf("%s: nothing dumped", name)
		}
	}
}
