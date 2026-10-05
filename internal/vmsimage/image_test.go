package vmsimage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSymbolTableBlock reads the symbol table block of Phase 41's probe
// images, checking it against what VMS's ANALYZE/IMAGE reported for
// them (testdata/dbg/vax/*.ani), and that the debug tables it points at
// are in the file. Real LINK sets IHD$V_IHSLONG, so the 32-bit sizes are
// used, and they agree with the 16-bit ones.
func TestSymbolTableBlock(t *testing.T) {
	for _, tc := range []struct {
		name               string
		dstVBN, dstBlocks  uint32
		gstVBN, gstRecords uint32
		dmtVBN, dmtBytes   uint32
	}{
		{"dbgdis", 5, 2, 8, 5, 7, 64},
		{"forth", 24, 56, 81, 5, 80, 60},
		{"dbgtrc", 5, 1, 0, 0, 0, 0},
		{"dbgnotb", 0, 0, 0, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "dbg", "vax", tc.name+".exe"))
			if err != nil {
				t.Fatal(err)
			}

			img, err := ReadImage(data)
			if err != nil {
				t.Fatal(err)
			}

			if img.LinkFlags&IHDFlagIHSLONG == 0 {
				t.Error("IHD$V_IHSLONG isn't set")
			}

			got := []uint32{img.DSTVBN, img.DSTBlockCount(), img.GSTVBN, img.GSTRecordCount(), img.DMTVBN, img.DMTBytes}
			want := []uint32{tc.dstVBN, tc.dstBlocks, tc.gstVBN, tc.gstRecords, tc.dmtVBN, tc.dmtBytes}

			for i := range got {
				if got[i] != want[i] {
					t.Errorf("IHS = %v, want %v", got, want)

					break
				}
			}

			if uint32(img.DSTBlocks) != img.DSTBlocksLong || uint32(img.GSTRecords) != img.GSTRecordsLong {
				t.Errorf("16-bit sizes %d, %d differ from 32-bit %d, %d",
					img.DSTBlocks, img.GSTRecords, img.DSTBlocksLong, img.GSTRecordsLong)
			}

			if tc.dstVBN != 0 && Blocks(data, img.DSTVBN, img.DSTBlockCount()) == nil {
				t.Error("the DST isn't in the file")
			}
		})
	}
}

func TestBlocks(t *testing.T) {
	data := make([]byte, 3*BlockSize)
	data[BlockSize] = 7

	if b := Blocks(data, 2, 2); len(b) != 2*BlockSize || b[0] != 7 {
		t.Errorf("Blocks(2, 2) = %d bytes", len(b))
	}

	if Blocks(data, 3, 2) != nil || Blocks(data, 0, 1) != nil {
		t.Error("Blocks past the end, or at VBN 0, returned data")
	}
}
