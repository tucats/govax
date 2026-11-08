package lbr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
)

// round3Library reads one library from round 3's save set
// (testdata/help90/vax/round3/h90w.bck), the libraries VMS 7.1's LBR$
// routines made.
func round3Library(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(help90Dir, "vax", "round3", "h90w.bck"))
	if err != nil {
		t.Fatal(err)
	}

	set, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range set.Files {
		if strings.TrimPrefix(f.Name, "[000000]") == name {
			return f.Data()
		}
	}

	t.Fatalf("%s isn't in the save set", name)

	return nil
}

// sameAsVMS holds u's file to VMS's, with the header's librarian's name and
// times and each module header's insertion time (at modules, the RFAs u
// gave) taken from VMS's.
func sameAsVMS(t *testing.T, u *Updater, modules []RFA, want []byte) {
	t.Helper()

	got := append([]byte(nil), u.Bytes()...)
	copy(got[lhdLbrVer:lhdMhdUsz], want[lhdLbrVer:lhdMhdUsz])

	for _, rfa := range modules {
		spans, _, _, err := u.record(rfa)
		if err != nil {
			t.Fatalf("module at %v: %v", rfa, err)
		}

		for i := 8; i < 16; i++ {
			at := placeOf(spans, i)
			got[at] = want[at]
		}
	}

	if len(got) != len(want) {
		t.Errorf("%d blocks, VMS's %d", len(got)/blockSize, len(want)/blockSize)
	}

	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			t.Fatalf("block %d differs at byte %d: %02X, VMS's %02X", i/blockSize+1, i%blockSize, got[i], want[i])
		}
	}
}

// placeOf is where byte i of a record whose bytes are spans is in the file.
func placeOf(spans []span, i int) int {
	for _, s := range spans {
		if i < s.n {
			return int(s.vbn-1)*blockSize + s.off + i
		}

		i -= s.n
	}

	panic("past the record")
}

// TestUpdaterW9 makes round 3's W9 again as LBR90W.MAR made it: a text
// library with one preallocated index block, 126 modules of a record each
// keyed in the order 37*i mod 127 (so the index splits three times, its
// new blocks taken between the data blocks), then a module of 300 records
// of 80 bytes. The file is VMS's to the byte.
func TestUpdaterW9(t *testing.T) {
	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 31, Indexes: 1, Entries: 8, History: 20})
	if err != nil {
		t.Fatal(err)
	}

	var modules []RFA

	for i := 1; i <= 126; i++ {
		n := 37 * i % 127

		rfa, err := u.PutRecord(fmt.Appendf(nil, "Record of K%03d.", n))
		if err != nil {
			t.Fatal(err)
		}

		u.PutEnd()

		key, err := u.Key(0, fmt.Sprintf("K%03d", n))
		if err != nil {
			t.Fatal(err)
		}

		if err := u.InsertKey(0, key, rfa); err != nil {
			t.Fatal(err)
		}

		modules = append(modules, rfa)
	}

	var long RFA

	for i := 1; i <= 300; i++ {
		if long, err = u.PutRecord(fmt.Appendf(nil, "L%04d %s", i, strings.Repeat(".", 74))); err != nil {
			t.Fatal(err)
		}
	}

	u.PutEnd()

	if err := u.InsertKey(0, []byte("LONG"), long); err != nil {
		t.Fatal(err)
	}

	sameAsVMS(t, u, append(modules, long), round3Library(t, "W9.TLB;1"))
}

// TestUpdaterKeys inserts and deletes keys of many lengths at random in a
// library whose blocks hold few of them, so that the index grows to
// several levels, and after each change reads the library and holds its
// keys to a model's.
func TestUpdaterKeys(t *testing.T) {
	r := rand.New(rand.NewSource(90))

	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 120, Indexes: 1, Entries: 8, History: 20})
	if err != nil {
		t.Fatal(err)
	}

	rfa, err := u.PutRecord([]byte("The only module."))
	if err != nil {
		t.Fatal(err)
	}

	u.PutEnd()

	model := map[string]bool{}

	for step := range 2000 {
		name := fmt.Sprintf("K%0*d", 1+r.Intn(100), r.Intn(400))
		key, err := u.Key(0, name)
		if err != nil {
			t.Fatal(err)
		}

		switch {
		case model[string(key)] && r.Intn(3) == 0:
			if err := u.DeleteKey(0, key); err != nil {
				t.Fatalf("step %d: deleting %s: %v", step, key, err)
			}

			delete(model, string(key))
		case model[string(key)]:
			if err := u.InsertKey(0, key, rfa); err != ErrDuplicateKey {
				t.Fatalf("step %d: inserting %s again: %v", step, key, err)
			}
		default:
			if err := u.InsertKey(0, key, rfa); err != nil {
				t.Fatalf("step %d: inserting %s: %v", step, key, err)
			}

			model[string(key)] = true
		}

		lib, err := u.Library()
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}

		var got []string
		for _, k := range lib.Indexes[0].Keys {
			got = append(got, k.Name)
		}

		var want []string
		for k := range model {
			want = append(want, k)
		}

		slices.Sort(want)

		if !slices.Equal(got, want) {
			t.Fatalf("step %d (%s): the index has %d keys, the model %d", step, key, len(got), len(want))
		}

		if h, err := lib.Header(rfa); err != nil || int(h.RefCount) != len(model) {
			t.Fatalf("step %d: the module counts %d keys (%v), the model %d", step, h.RefCount, err, len(model))
		}
	}

	if lib, _ := u.Library(); lib.IndexBlocks < 3 {
		t.Errorf("the index has %d blocks: the test didn't split it", lib.IndexBlocks)
	}
}

// TestUpdaterDeleteData deletes a module whose records fill a block of
// their own: the block goes on the deleted blocks' list, as H90T's did in
// round 3, and a module with keys can't be deleted.
func TestUpdaterDeleteData(t *testing.T) {
	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 31, Indexes: 1, Entries: 96, History: 20})
	if err != nil {
		t.Fatal(err)
	}

	first, _ := u.PutRecord(bytes.Repeat([]byte("A"), 400))
	u.PutEnd()

	if err := u.InsertKey(0, []byte("A"), first); err != nil {
		t.Fatal(err)
	}

	if err := u.DeleteData(first); err != ErrStillKeys {
		t.Fatalf("deleting a module with a key: %v", err)
	}

	// A second session's module starts a new block.
	if u, err = OpenUpdater(u.Bytes()); err != nil {
		t.Fatal(err)
	}

	second, _ := u.PutRecord([]byte("B"))
	u.PutEnd()

	if second.VBN != first.VBN+1 || second.Offset != dataData {
		t.Fatalf("the second module is at %v", second)
	}

	if err := u.DeleteKey(0, []byte("A")); err != nil {
		t.Fatal(err)
	}

	if err := u.DeleteData(first); err != nil {
		t.Fatal(err)
	}

	if u.hdr(lhdFreeBlk) != 1 || u.hdr(lhdFreeVBN) != first.VBN || u.block(first.VBN)[0] != 0 {
		t.Errorf("deleted blocks %d, first %d, the block's count %d", u.hdr(lhdFreeBlk), u.hdr(lhdFreeVBN), u.block(first.VBN)[0])
	}

	if u.hdr(lhdModHdrs) != 1 {
		t.Errorf("%d module headers", u.hdr(lhdModHdrs))
	}
}

// TestUpdaterW14 makes round 3's W14 again: a text library in the VMS V2
// format (its own sanity longword, index entries of the key length's 38
// bytes, 9 preallocated index blocks) with one module. The file is VMS's
// to the byte, and reads back.
func TestUpdaterW14(t *testing.T) {
	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 31, Indexes: 1, Entries: 96, History: 20, V2: true})
	if err != nil {
		t.Fatal(err)
	}

	rfa, err := u.PutRecord([]byte("V2's record."))
	if err != nil {
		t.Fatal(err)
	}

	u.PutEnd()

	key, err := u.Key(0, "V2")
	if err != nil {
		t.Fatal(err)
	}

	if err := u.InsertKey(0, key, rfa); err != nil {
		t.Fatal(err)
	}

	sameAsVMS(t, u, []RFA{rfa}, round3Library(t, "W14.TLB;1"))

	lib, err := u.Library()
	if err != nil {
		t.Fatal(err)
	}

	if !lib.V2 {
		t.Error("the library isn't taken for the V2 format")
	}

	if got, ok := lib.Lookup("V2"); !ok || got != rfa {
		t.Errorf("V2 is at %v, %t; want %v", got, ok, rfa)
	}
}

// TestUpdaterBinaryKeys enters binary keys (KEYLEN 0: the manual's 32-bit
// unsigned keys) in an order and with values whose bytes, compared as text,
// would order them otherwise, and reads them back in the numbers' order,
// through enough of them to split the index.
func TestUpdaterBinaryKeys(t *testing.T) {
	u, err := NewUpdater(UpdateOptions{Type: 128, Indexes: 1, Entries: 96})
	if err != nil {
		t.Fatal(err)
	}

	if !u.Binary(0) {
		t.Fatal("index 1 isn't binary")
	}

	rfa, err := u.PutRecord([]byte("A module."))
	if err != nil {
		t.Fatal(err)
	}

	u.PutEnd()

	r := rand.New(rand.NewSource(14))
	want := map[uint32]bool{}

	for len(want) < 200 {
		v := r.Uint32() >> (r.Intn(4) * 8)
		if want[v] || v == 256 {
			continue
		}

		key, err := u.BinaryKey(0, v)
		if err != nil {
			t.Fatal(err)
		}

		if err := u.InsertKey(0, key, rfa); err != nil {
			t.Fatalf("key %d: %v", v, err)
		}

		want[v] = true
	}

	key, _ := u.BinaryKey(0, 256)
	if err := u.InsertKey(0, key, rfa); err != nil {
		t.Fatal(err)
	}

	want[256] = true

	if dup := u.InsertKey(0, key, rfa); !errors.Is(dup, ErrDuplicateKey) {
		t.Errorf("a second 256: %v", dup)
	}

	lib, err := u.Library()
	if err != nil {
		t.Fatal(err)
	}

	x := lib.Indexes[0]
	if !x.Binary() || len(x.Keys) != len(want) {
		t.Fatalf("binary %t, %d keys; want %d", x.Binary(), len(x.Keys), len(want))
	}

	var last uint32

	for i, k := range x.Keys {
		v := binary.LittleEndian.Uint32([]byte(k.Name))
		if !want[v] || (i > 0 && v <= last) {
			t.Fatalf("key %d is %d, after %d", i, v, last)
		}

		last = v
	}

	if lib.IndexBlocks < 2 {
		t.Errorf("%d index blocks: the index didn't split", lib.IndexBlocks)
	}
}
