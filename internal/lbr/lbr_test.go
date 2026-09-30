package lbr

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// libraryBuilder lays out a small library by hand, block by block.
type libraryBuilder struct {
	blocks [][]byte
}

// block returns block vbn, adding blocks up to it.
func (b *libraryBuilder) block(vbn int) []byte {
	for len(b.blocks) < vbn {
		b.blocks = append(b.blocks, make([]byte, blockSize))
	}

	return b.blocks[vbn-1]
}

func (b *libraryBuilder) bytes() []byte { return bytes.Join(b.blocks, nil) }

// indexBlock writes an index block holding entries.
func (b *libraryBuilder) indexBlock(vbn int, entries ...Key) {
	blk := b.block(vbn)
	p := indexEntries

	for _, e := range entries {
		binary.LittleEndian.PutUint32(blk[p:], e.RFA.VBN)
		binary.LittleEndian.PutUint16(blk[p+4:], e.RFA.Offset)
		blk[p+6] = byte(len(e.Name))
		p += 7 + copy(blk[p+7:], e.Name)
	}

	binary.LittleEndian.PutUint16(blk, uint16(p-indexEntries))
}

// syntheticLibrary is an object library with one module, MOD, defining
// SYM. Index 1's root is an upper-level block over a leaf, and the
// module's second record runs from block 4 on into block 6, its chain's
// next block, skipping block 5.
func syntheticLibrary(t *testing.T) ([]byte, []byte) {
	t.Helper()

	b := &libraryBuilder{}
	h := b.block(1)
	h[lhdType] = byte(TypeObject)
	h[lhdNIndex] = 2
	binary.LittleEndian.PutUint32(h[lhdSanity:], saneID3)
	binary.LittleEndian.PutUint16(h[lhdMajorID:], 3)
	h[lhdLbrVer] = byte(copy(h[lhdLbrVer+1:], "Librarian T09-20"))
	h[lhdMhdUsz] = 5
	binary.LittleEndian.PutUint32(h[lhdModCnt:], 1)

	for i, root := range []uint32{7, 3} {
		d := h[lhdIdxDesc+i*iddLength:]
		binary.LittleEndian.PutUint16(d, IndexASCII|IndexVarLength)
		binary.LittleEndian.PutUint16(d[2:], 31)
		binary.LittleEndian.PutUint32(d[4:], root)
	}

	mod := RFA{VBN: 4, Offset: dataData}
	b.indexBlock(7, Key{Name: "MOD", RFA: RFA{VBN: 2, Offset: rfaIndex}})
	b.indexBlock(2, Key{Name: "MOD", RFA: mod})
	b.indexBlock(3, Key{Name: "SYM", RFA: mod})

	long := bytes.Repeat([]byte("0123456789"), 60)

	// The module: its header (an odd length, so the next record is
	// word-aligned), a 600-byte record, then the end-of-text record.
	d := b.block(4)
	binary.LittleEndian.PutUint32(d[dataLink:], 6)

	p := dataData
	binary.LittleEndian.PutUint16(d[p:], mhdLen+5)

	mhd := d[p+2:]
	mhd[1] = mhdID
	binary.LittleEndian.PutUint32(mhd[4:], 2)
	copy(mhd[mhdLen:], []byte{0, 3, 'X', '-', '1'})

	p += 2 + mhdLen + 5 + 1

	binary.LittleEndian.PutUint16(d[p:], uint16(len(long)))
	n := copy(d[p+2:], long)

	d = b.block(6)
	p = dataData + copy(d[dataData:], long[n:])
	p = (p + 1) &^ 1
	binary.LittleEndian.PutUint16(d[p:], 3)
	copy(d[p+2:], eot)

	b.block(5)[0] = 0xEE // not part of the chain

	return b.bytes(), long
}

func TestSyntheticLibrary(t *testing.T) {
	data, long := syntheticLibrary(t)

	l, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}

	if l.Type != TypeObject || l.Librarian != "Librarian T09-20" || l.Modules != 1 || len(l.Indexes) != 2 {
		t.Fatalf("header: %+v", l)
	}

	rfa, ok := l.Lookup("MOD")
	if !ok {
		t.Fatal("no MOD in index 1")
	}

	if sym, ok := l.Indexes[1].Lookup("SYM"); !ok || sym != rfa {
		t.Fatalf("SYM is at %v, %v; want %v", sym, ok, rfa)
	}

	if name, ok := l.ModuleName(rfa); !ok || name != "MOD" {
		t.Errorf("ModuleName = %q, %v", name, ok)
	}

	m, err := l.Module(rfa)
	if err != nil {
		t.Fatal(err)
	}

	if m.Header.RefCount != 2 || m.Header.ObjectIdent() != "X-1" {
		t.Errorf("module header: %+v, ident %q", m.Header, m.Header.ObjectIdent())
	}

	if len(m.Records) != 1 || !bytes.Equal(m.Records[0], long) {
		t.Errorf("records: %d, want the one %d-byte record", len(m.Records), len(long))
	}
}

func TestOpenErrors(t *testing.T) {
	data, _ := syntheticLibrary(t)

	bad := append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(bad[lhdSanity:], 123454321)

	if _, err := Open(bad); err == nil {
		t.Error("an old-format sanity longword was accepted")
	}

	if _, err := Open(data[:100]); err == nil {
		t.Error("a short file was accepted")
	}

	// An index whose root is its own child.
	loop := append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(loop[6*blockSize+indexEntries:], 7)

	if _, err := Open(loop); err == nil {
		t.Error("a looping index was accepted")
	}
}

// dcxTestMap builds a DCX map. Sub-map 0 codes 'A' as 0, 'B' as 10, and
// the end of the record as 11; after an 'A', sub-map 1, whose 'A' and 'B'
// are swapped, codes the rest.
func dcxTestMap() []byte {
	sub := func(leaf0, leaf1 byte, next []uint16) []byte {
		s := make([]byte, sbmLength)
		s[sbmMinChar] = 'A'
		binary.LittleEndian.PutUint16(s[sbmFlags:], uint16(len(s)))
		s = append(s, 0x05) // nodes 0 and 2 are leaves
		binary.LittleEndian.PutUint16(s[sbmNodes:], uint16(len(s)))
		s = append(s, leaf0, 1, leaf1, 0)

		if next != nil {
			binary.LittleEndian.PutUint16(s[sbmNext:], uint16(len(s)))

			for _, n := range next {
				s = binary.LittleEndian.AppendUint16(s, n)
			}
		}

		binary.LittleEndian.PutUint16(s[sbmSize:], uint16(len(s)))

		return s
	}

	m := make([]byte, dcxMapLength)
	binary.LittleEndian.PutUint32(m[dcxMapSanityOff:], dcxMapSanity)
	binary.LittleEndian.PutUint16(m[dcxMapNSubs:], 2)
	binary.LittleEndian.PutUint16(m[dcxMapSub0:], dcxMapLength)
	m = append(m, sub('A', 'B', []uint16{1, 0})...)
	m = append(m, sub('B', 'A', nil)...)
	binary.LittleEndian.PutUint32(m, uint32(len(m)))

	return m
}

func TestDCXExpand(t *testing.T) {
	m, err := parseDCXMap(dcxTestMap())
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		in   []byte
		want string
	}{
		{[]byte{0x0D}, "B"},  // 1 0: B; then 1 1, the end, in sub-map 0
		{[]byte{0x0C}, "AB"}, // 0: A; then sub-map 1's 0 is B; then 1 1
		{[]byte{0x03}, ""},   // the end, at once
	} {
		got, err := m.expand(tc.in)
		if err != nil || string(got) != tc.want {
			t.Errorf("expand(% X) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}

	if _, err := m.expand([]byte{0x00}); err == nil {
		t.Error("data with no end of record expanded")
	}

	bad := dcxTestMap()
	bad[dcxMapSanityOff] ^= 1

	if _, err := parseDCXMap(bad); err == nil {
		t.Error("a map with a bad sanity longword was accepted")
	}
}

// vmsLibrary reads a library copied from a VAX (testdata/vmslib, which
// isn't in git), or skips the test.
func vmsLibrary(t *testing.T, name string) *Library {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vmslib", name))
	if err != nil {
		t.Skipf("no %s: %v", name, err)
	}

	l, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}

	return l
}

// TestVMSLibraries reads every module of IMAGELIB.OLB and STARLET.OLB, a
// data-reduced library, and decodes each as an object module. The indexes
// hold as many keys as the header's LHD$L_IDXCNT says.
func TestVMSLibraries(t *testing.T) {
	for _, tc := range []struct {
		file    string
		typ     Type
		dcx     bool
		modules int
	}{
		{"imagelib.olb", TypeShareable, false, 61},
		{"starlet.olb", TypeObject, true, 1524},
	} {
		t.Run(tc.file, func(t *testing.T) {
			l := vmsLibrary(t, tc.file)

			if l.Type != tc.typ || (l.dcx != nil) != tc.dcx || int(l.Modules) != tc.modules || l.Librarian != "Librarian T09-20" {
				t.Fatalf("header: type %s, data-reduced %v, %d modules, %q", l.Type, l.dcx != nil, l.Modules, l.Librarian)
			}

			keys := 0
			for _, x := range l.Indexes {
				keys += len(x.Keys)
			}

			if want := int(le32(l.data, 0x6A)); keys != want {
				t.Errorf("%d keys; the header says %d", keys, want)
			}

			for _, k := range l.Indexes[0].Keys {
				m, err := l.Module(k.RFA)
				if err != nil {
					t.Fatal(err)
				}

				om, err := obj.Decode(m.Records)
				if err != nil {
					t.Fatalf("%s: %v", k.Name, err)
				}

				if om.Name() != k.Name {
					t.Errorf("module %s is named %s", k.Name, om.Name())
				}
			}
		})
	}
}

// TestIMAGELIBModule checks what IMAGELIB says of LIBRTL: which image
// defines LIB$PUT_OUTPUT, and the image's global section ident in its
// module header (major 1, minor 0x0E, as ANALYZE/IMAGE showed).
func TestIMAGELIBModule(t *testing.T) {
	l := vmsLibrary(t, "imagelib.olb")

	rfa, ok := l.Indexes[1].Lookup("LIB$PUT_OUTPUT")
	if !ok {
		t.Fatal("no LIB$PUT_OUTPUT")
	}

	if name, _ := l.ModuleName(rfa); name != "LIBRTL" {
		t.Fatalf("LIB$PUT_OUTPUT is in %s", name)
	}

	m, err := l.Module(rfa)
	if err != nil {
		t.Fatal(err)
	}

	if id := m.Header.ObjectIdent(); id != "\x0E\x00\x00\x01" {
		t.Errorf("ident % X", id)
	}
}
