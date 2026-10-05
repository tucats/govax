package link

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// dstDir holds FORTH's /DEBUG build on VMS 7.3 (testdata/mar/dst/vax):
// real MACRO's object and real LINK's LINK/DEBUG image.
var dstDir = filepath.Join("..", "..", "testdata", "mar", "dst", "vax")

// forthObject reads real MACRO's /DEBUG object of FORTH.
func forthObject(t *testing.T) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join(dstDir, "forth.obj"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	records, err := obj.ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := obj.Decode(records)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// TestLinkForthDST links FORTH's /DEBUG object with LINK/DEBUG and checks
// its debug symbol table, 56 blocks of TBT and DBG records, against real
// LINK's, and the IHS's 32-bit block count (IHS$L_DSTBLKS, +20), which a
// one-block DST can't tell from a constant 1 (docs/PHASE-29.md, subtask
// 16). govax's own symbol tables stand in for VMS's libraries.
func TestLinkForthDST(t *testing.T) {
	want, opts := realImage(t, filepath.Join(dstDir, "forth.exe"))
	opts.Debug = true
	opts.Sources = []SymbolSource{govaxTables()}

	img, err := Link([]Input{{File: "FORTH.OBJ", Module: forthObject(t)}}, opts)
	if err != nil {
		t.Fatal(err)
	}

	got, wantDST := imageDST(t, img.Bytes), imageDST(t, want)
	if !bytes.Equal(got, wantDST) {
		t.Errorf("DST differs from real LINK's:\n%s", diffBlocks(got, wantDST))
	}

	if a, b := img.Bytes[ihsOffset+20:ihsOffset+24], want[ihsOffset+20:ihsOffset+24]; !bytes.Equal(a, b) {
		t.Errorf("IHS$L_DSTBLKS is % x, want % x", a, b)
	}

	if img.Bytes[0x20]&ihdLNKDEBUG == 0 {
		t.Error("IHD$V_LNKDEBUG isn't set")
	}
}

// debugCase is one of the three real images linked /DEBUG: its name, its
// directory, the objects linked, and what stands in for VMS's libraries.
type debugCase struct {
	image   string
	dir     string
	objects []string
	sources []SymbolSource
}

// debugCases are the images real LINK linked /DEBUG: TRDBGLNK (TRACE
// assembled /DEBUG), TRLNKDBG (TRACE, with no DBG records), and FORTH.
func debugCases() []debugCase {
	vax := filepath.Join(probeDir, "vax")

	return []debugCase{
		{"trdbglnk", vax, []string{"trdebug"}, []SymbolSource{probeRoutines}},
		{"trlnkdbg", vax, []string{"trace"}, []SymbolSource{probeRoutines}},
		{"forth", dstDir, []string{"forth"}, []SymbolSource{govaxTables()}},
	}
}

// linkDebugCase links a debugCase's objects (real MACRO's) as real LINK
// did, and returns govax's image and real LINK's.
func linkDebugCase(t *testing.T, c debugCase) (got, want []byte) {
	t.Helper()

	want, opts := realImage(t, filepath.Join(c.dir, c.image+".exe"))
	opts.Debug = true
	opts.Sources = c.sources

	inputs := make([]Input, len(c.objects))

	for i, name := range c.objects {
		data, err := os.ReadFile(filepath.Join(c.dir, name+".obj"))
		if err != nil {
			t.Fatal(err)
		}

		records, err := obj.ReadRecords(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}

		m, err := obj.Decode(records)
		if err != nil {
			t.Fatal(err)
		}

		inputs[i] = Input{File: name + ".OBJ", Module: m}
	}

	img, err := Link(inputs, opts)
	if err != nil {
		t.Fatal(err)
	}

	return img.Bytes, want
}

// TestLinkDebugModuleTable checks the debug module table (DMT) of each
// image linked /DEBUG against real LINK's, block for block, with the
// IHS's VBN and byte count for it (docs/PHASE-29.md, subtask 17).
func TestLinkDebugModuleTable(t *testing.T) {
	for _, c := range debugCases() {
		t.Run(c.image, func(t *testing.T) {
			got, want := linkDebugCase(t, c)

			if a, b := got[ihsOffset+12:ihsOffset+20], want[ihsOffset+12:ihsOffset+20]; !bytes.Equal(a, b) {
				t.Fatalf("IHS DMT fields are % x, want % x", a, b)
			}

			ihs := want[ihsOffset:]
			vbn := int(binary.LittleEndian.Uint32(ihs[12:]))
			size := int(binary.LittleEndian.Uint32(ihs[16:]))
			from, to := (vbn-1)*blockSize, int(pageUp(uint32(vbn*blockSize-blockSize+size)))

			if len(got) < to {
				t.Fatalf("image is %d bytes, the DMT ends at %d", len(got), to)
			}

			if !bytes.Equal(got[from:to], want[from:to]) {
				t.Errorf("DMT differs from real LINK's:\n%s", diffBlocks(got[from:to], want[from:to]))
			}
		})
	}
}
