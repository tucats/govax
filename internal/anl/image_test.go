package anl

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// imageFixtureDirs hold real LINK's images with VMS 7.3's ANALYZE/IMAGE
// output for each beside them (NAME.EXE and NAME.ANI).
var imageFixtureDirs = []string{
	"../../testdata/link/vax",
	"../../testdata/mar/list/vax",
	"../../testdata/mar/round/vax",
}

// imageFixture is one image and its real analysis.
type imageFixture struct {
	name     string
	data     []byte
	analysis []byte
}

// imageFixtures returns every image that has an analysis beside it.
func imageFixtures(t *testing.T) []imageFixture {
	t.Helper()

	var out []imageFixture

	for _, dir := range imageFixtureDirs {
		anis, err := filepath.Glob(filepath.Join(dir, "*.ani"))
		if err != nil {
			t.Fatal(err)
		}

		for _, ani := range anis {
			data, err := os.ReadFile(strings.TrimSuffix(ani, ".ani") + ".exe")
			if err != nil {
				continue
			}

			analysis, err := os.ReadFile(ani)
			if err != nil {
				t.Fatal(err)
			}

			out = append(out, imageFixture{name: ani, data: data, analysis: analysis})
		}
	}

	if len(out) < 29 {
		t.Fatalf("found %d image fixtures, want at least 29", len(out))
	}

	return out
}

// TestReadImageFixtures decodes every fixture image without a problem.
func TestReadImageFixtures(t *testing.T) {
	for _, f := range imageFixtures(t) {
		img, err := ReadImage(f.data)
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}

		if len(img.Problems) != 0 {
			t.Errorf("%s: %v", f.name, img.Problems)
		}

		if img.Fixups == nil {
			t.Errorf("%s: no fixup section", f.name)
		}
	}
}

// TestReadImageAddr checks every field of ADDR.EXE against what VMS's
// analysis of it (addr.ani) shows.
func TestReadImageAddr(t *testing.T) {
	data, err := os.ReadFile("../../testdata/link/vax/addr.exe")
	if err != nil {
		t.Fatal(err)
	}

	img, err := ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	if img.MajorID != "02" || img.MinorID != "05" || img.Blocks != 1 || img.Type != 1 {
		t.Errorf("fixed header: %q %q %d %d", img.MajorID, img.MinorID, img.Blocks, img.Type)
	}

	if img.LinkFlags&0xFF != 0xA8 {
		t.Errorf("link flags %#x", img.LinkFlags)
	}

	if img.Transfers != [3]uint32{0x7FFEDF68, 0x400, 0} {
		t.Errorf("transfers %#x", img.Transfers)
	}

	if img.DSTVBN != 5 || img.DSTBlocks != 1 || img.GSTVBN != 0 {
		t.Errorf("DST %d/%d, GST %d", img.DSTVBN, img.DSTBlocks, img.GSTVBN)
	}

	if img.Name != "ADDR" || img.FileID != "V1.0" || img.LinkerID != "V11-39" {
		t.Errorf("identification %q %q %q", img.Name, img.FileID, img.LinkerID)
	}

	if got := vmsTime(vmsdef.GoTime(img.LinkTime)); got != "30-SEP-2026 06:01:55.86" {
		t.Errorf("link time %s", got)
	}

	if len(img.ISDs) != 5 {
		t.Fatalf("%d ISDs", len(img.ISDs))
	}

	stack, global := img.ISDs[3], img.ISDs[4]

	if stack.Size != 12 || stack.Pages != 20 || stack.Address() != 0x7FFFD800 || stack.Type() != 253 {
		t.Errorf("stack ISD %+v", stack)
	}

	if global.Size != 31 || global.Pages != 264 || global.GlobalName != "LIBRTL_001" ||
		global.GlobalIdent != 0x0100000E || global.Match() != 2 || global.Type() != 3 {
		t.Errorf("global ISD %+v", global)
	}

	f := img.Fixups
	want := &Fixups{
		VA: 0x600, Base: 0x200, ShareCount: 2,
		GFixOffset: 0x40, DotAddrOffset: 0xDC, ChgPrtOffset: 0x50, ShlOffset: 0x5C,
		Shared:      []string{"", "LIBRTL"},
		GRefs:       []RefList{{Image: 1, Values: []uint32{0x478}}},
		DotAddrRefs: []RefList{{Image: 1, Values: []uint32{0}}},
		Protections: []Protection{{Address: 0x400, Pages: 1, Code: 0x0D}},
	}

	if !reflect.DeepEqual(f, want) {
		t.Errorf("fixups\n got %+v\nwant %+v", f, want)
	}
}

// TestReadImageContinuedISDs checks an ISD list that goes on in a second
// header block, and that a short file isn't an image.
func TestReadImageContinuedISDs(t *testing.T) {
	data := make([]byte, 3*imageBlock)
	le := binary.LittleEndian

	le.PutUint16(data[ihdISDOffset:], 0x1F0)
	data[ihdBlockCount] = 2

	// One ISD at the end of the first block, then a continuation mark.
	le.PutUint16(data[0x1F0:], isdPrivateLength)
	le.PutUint16(data[0x1F2:], 1)
	le.PutUint32(data[0x1F4:], 1)
	le.PutUint16(data[0x1F0+isdPrivateLength:], isdContinue)

	// The next, at the start of the second.
	le.PutUint16(data[imageBlock:], isdDemandZeroLength)
	le.PutUint16(data[imageBlock+2:], 4)
	le.PutUint32(data[imageBlock+4:], 2)

	img, err := ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(img.ISDs) != 2 || img.ISDs[1].Pages != 4 || img.ISDs[1].Address() != 0x400 {
		t.Errorf("ISDs %+v", img.ISDs)
	}

	if len(img.Problems) != 0 {
		t.Errorf("problems %v", img.Problems)
	}

	if _, err := ReadImage(data[:100]); err == nil {
		t.Error("a 100-byte file read as an image")
	}
}
