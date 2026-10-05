package link

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/anl"
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

// TestLinkGlobalSymbolTable checks the global symbol table (GST) of each
// image linked /DEBUG against real LINK's, record for record, with the
// IHS's VBN and record counts for it (docs/PHASE-29.md, subtask 18). Real
// LINK's GST ends the file mid-block; the comparison is of the records.
func TestLinkGlobalSymbolTable(t *testing.T) {
	for _, c := range debugCases() {
		t.Run(c.image, func(t *testing.T) {
			got, want := linkDebugCase(t, c)

			for _, f := range []struct {
				name     string
				from, to int
			}{{"IHS$L_GSTVBN", 4, 8}, {"IHS$W_GSTRECS", 10, 12}, {"IHS$L_GSTRECS", 24, 28}} {
				if a, b := got[ihsOffset+f.from:ihsOffset+f.to], want[ihsOffset+f.from:ihsOffset+f.to]; !bytes.Equal(a, b) {
					t.Errorf("%s is % x, want % x", f.name, a, b)
				}
			}

			records := func(img []byte) [][]byte {
				t.Helper()

				vbn := int(binary.LittleEndian.Uint32(img[ihsOffset+4:]))
				count := int(binary.LittleEndian.Uint32(img[ihsOffset+24:]))

				if vbn == 0 || (vbn-1)*blockSize >= len(img) {
					t.Fatalf("GST at VBN %d, in an image of %d bytes", vbn, len(img))
				}

				all, err := obj.ReadRecords(bytes.NewReader(img[(vbn-1)*blockSize:]))
				if err != nil && len(all) < count {
					t.Fatal(err)
				}

				if len(all) < count {
					t.Fatalf("GST has %d records, the IHS says %d", len(all), count)
				}

				return all[:count]
			}

			g, w := records(got), records(want)
			for i := 0; i < len(g) || i < len(w); i++ {
				switch {
				case i >= len(g):
					t.Errorf("record %d missing: % x", i+1, w[i])
				case i >= len(w):
					t.Errorf("record %d extra: % x", i+1, g[i])
				case !bytes.Equal(g[i], w[i]):
					t.Errorf("record %d:\n got % x\nwant % x", i+1, g[i], w[i])
				}
			}
		})
	}
}

// TestLinkDebugImagesMatchRealLINK links the three images real LINK linked
// /DEBUG, from real MACRO's objects and, for TRLNKDBG, from govax's object
// of TRACE, and checks each is real LINK's byte for byte: header, image
// sections, DST, DMT, and GST (docs/PHASE-29.md, subtask 19). The one
// difference is Decision 7's: real LINK ends the file at the GST's last
// byte, and govax pads that block with zeros.
func TestLinkDebugImagesMatchRealLINK(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(probeDir, "trace.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range debugCases() {
		t.Run(c.image, func(t *testing.T) {
			check := func(from string, got, want []byte) {
				t.Helper()

				if len(got) != int(pageUp(uint32(len(want)))) {
					t.Fatalf("%s: image is %d bytes, want %d padded to a block", from, len(got), len(want))
				}

				if !bytes.Equal(got[:len(want)], want) {
					t.Errorf("%s: image differs from real LINK's:\n%s", from, diffBlocks(got[:len(want)], want))
				}

				if n := len(got) - len(want); bytes.Count(got[len(want):], []byte{0}) != n {
					t.Errorf("%s: the GST's padding isn't zeros", from)
				}

				// ANALYZE/IMAGE reports the padded image as it does
				// real LINK's.
				if a, b := analyzeImage(t, got), analyzeImage(t, want); a != b {
					t.Errorf("%s: ANALYZE/IMAGE differs:\n%s", from, firstLineDiff(a, b))
				}
			}

			got, want := linkDebugCase(t, c)
			check("real MACRO's objects", got, want)

			// govax's objects of TRDEBUG and FORTH are compared with
			// real MACRO's by internal/asm's TestDebugRecords and
			// TestDebugRecordsForth: TRDEBUG's matches whole, given the
			// source file's attributes, and FORTH's can't (govax's
			// STARLET defines other $$ symbols).
			if c.image != "trlnkdbg" {
				return
			}

			m := macroModule(t, string(src))

			_, opts := realImage(t, filepath.Join(c.dir, c.image+".exe"))
			opts.Debug, opts.Sources = true, c.sources

			img, err := Link([]Input{{File: "TRACE.OBJ", Module: m}}, opts)
			if err != nil {
				t.Fatal(err)
			}

			check("govax's object", img.Bytes, want)
		})
	}
}

// analyzeImage returns ANALYZE/IMAGE's report of an image, unpaged.
func analyzeImage(t *testing.T, data []byte) string {
	t.Helper()

	img, err := anl.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	if err := anl.WriteText(&b, anl.AnalyzeImage(img, anl.ImageOptions{}).Lines); err != nil {
		t.Fatal(err)
	}

	return b.String()
}

// firstLineDiff shows the first line where two texts differ.
func firstLineDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")

	for i := 0; i < len(g) || i < len(w); i++ {
		var a, b string
		if i < len(g) {
			a = g[i]
		}

		if i < len(w) {
			b = w[i]
		}

		if a != b {
			return "line " + strconv.Itoa(i+1) + ":\n got " + a + "\nwant " + b
		}
	}

	return ""
}
