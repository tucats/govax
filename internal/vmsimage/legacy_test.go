package vmsimage

import (
	"encoding/hex"
	"strings"
	"testing"
)

// dungeonHeader is the start of DUNGEON.EXE's header block, a VMS V1
// FORTRAN image (format 02.01, linked 7-DEC-1978 by linker 01.20): the
// fixed header, its activation, symbol table, and identification
// blocks, and its ISDs, the last three VMSRTL's global sections. The
// rest of the block is ^XFF.
var dungeonHeader = "" +
	"7c002800380044000000000030323031" +
	"01010000000000000000000000000000" +
	"000000020000000033a40100b0e10000" +
	"0000000000000000c100000000000000" +
	"120000000744554e47454f4e00000000" +
	"00000000023031000000000000000000" +
	"00000000807e6a5a2b9986000530312e" +
	"32300000000000000000000010000e00" +
	"01000000000000000200000010002800" +
	"0f0000000a000000100000000c000c00" +
	"370000000c0000001000060043000000" +
	"0a000000380000000c00070049000000" +
	"0c00000010000600500000000a000000" +
	"3e00000010007c005600000000000000" +
	"4400000010000100d20000000a000000" +
	"c00000000c001400ecff3f000c0000fd" +
	"24000400d3000001a100000300000000" +
	"e80300010a564d5352544c5f30303100" +
	"0000000024003100d7000000a1000003" +
	"00000000e80300010a564d5352544c5f" +
	"30303200000000002400020008010000" +
	"ab00000400000000e80300010a564d53" +
	"52544c5f30303300000000000000"

// dungeonImage is a file with DUNGEON.EXE's header and, at VBN ^XC0
// (the section at ^X1A400), its image startup stub at ^X1A433; every
// other block is zeros.
func dungeonImage(t *testing.T) []byte {
	t.Helper()

	h, err := hex.DecodeString(dungeonHeader)
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 0xC0*BlockSize)
	copy(data, h)

	for i := len(h); i < BlockSize; i++ {
		data[i] = 0xFF
	}

	stub, _ := hex.DecodeString("0000179f6a010080") // .WORD 0; JMP @#8000016A
	copy(data[(0xC0-1)*BlockSize+0x33:], stub)

	return data
}

// TestLegacyHeader reads a V1 image's header, whose fixed header ends
// at ^X28 and whose blocks are shorter than 02.05's.
func TestLegacyHeader(t *testing.T) {
	img, err := ReadImage(dungeonImage(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range img.Problems {
		t.Errorf("problem: %s", p.Text)
	}

	if !img.Legacy() {
		t.Errorf("format %s.%s isn't legacy", img.MajorID, img.MinorID)
	}

	if img.Transfers != [3]uint32{0x1A433, 0xE1B0, 0} {
		t.Errorf("transfers %X", img.Transfers)
	}

	if img.IAFVA != 0 || img.SysVersion != 0 || img.InitShare != 0 {
		t.Errorf("IAFVA %X, SysVersion %X, InitShare %X: want none", img.IAFVA, img.SysVersion, img.InitShare)
	}

	if img.DSTVBN != 0xC1 || img.DSTBlockCount() != 18 || img.GSTVBN != 0 || img.DMTVBN != 0 || img.DMTBytes != 0 {
		t.Errorf("DST %d/%d, GST %d, DMT %d/%d", img.DSTVBN, img.DSTBlockCount(), img.GSTVBN, img.DMTVBN, img.DMTBytes)
	}

	if img.Name != "DUNGEON" || img.FileID != "01" || img.LinkerID != "01.20" || img.LinkTime != 0x0086992B5A6A7E80 {
		t.Errorf("identification %q %q %q %X", img.Name, img.FileID, img.LinkerID, img.LinkTime)
	}

	var names []string

	for _, d := range img.ISDs {
		if d.GlobalName != "" {
			names = append(names, d.GlobalName)
		}
	}

	if len(img.ISDs) != 12 || strings.Join(names, ",") != "VMSRTL_001,VMSRTL_002,VMSRTL_003" {
		t.Errorf("%d ISDs, global sections %v", len(img.ISDs), names)
	}

	if d := img.ISDs[11]; d.Address() != 0x21000 || d.Pages != 2 || d.Type() != 4 || d.GlobalIdent != 0x010003E8 {
		t.Errorf("VMSRTL_003 at %X, %d pages, type %d, ident %X", d.Address(), d.Pages, d.Type(), d.GlobalIdent)
	}
}

// TestAt reads bytes at an address through the section holding them.
func TestAt(t *testing.T) {
	data := dungeonImage(t)

	img, err := ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	if b := img.At(data, 0x1A433, 4); hex.EncodeToString(b) != "0000179f" {
		t.Errorf("At(1A433) = %x", b)
	}

	// Demand-zero (^X6E00) and global (^X1A600) sections have no
	// blocks to read, and nothing is mapped at ^X100.
	for _, va := range []uint32{0x6E00, 0x1A600, 0x100} {
		if b := img.At(data, va, 4); b != nil {
			t.Errorf("At(%X) = %x, want nil", va, b)
		}
	}

	// Past the file's end.
	if b := img.At(data[:BlockSize], 0x200, 4); b != nil {
		t.Errorf("At(200) of a header alone = %x, want nil", b)
	}
}
