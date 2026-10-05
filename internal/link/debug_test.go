package link

import (
	"bytes"
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
