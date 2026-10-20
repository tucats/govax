package link

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/lbr"
)

// TestLinkThroughShareableImageLibrary links testdata/link68's MAIN1L
// and MAIN1U from real MACRO's object, SHR1 found in VMS's MYSHR.OLB
// (docs/PHASE-68's subtask 12): MAIN1L named the library on the command
// line, and MAIN1U had it as a default user library, which LINK searches
// after the command's own and before IMAGELIB.OLB, so the sources are the
// same. The images are real LINK's byte for byte, SHR1's cells in the
// reverse of its global symbol table's order, and the full maps are real
// LINK's line for line: SHR1's cluster comes before LIBRTL's, though
// MAIN1 calls LIB$PUT_OUTPUT first, since MYSHR.OLB was searched before
// IMAGELIB.OLB.
func TestLinkThroughShareableImageLibrary(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(link68Dir, "vax", "myshr.olb"))
	if err != nil {
		t.Fatal(err)
	}

	lib, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"main1l", "main1u"} {
		t.Run(name, func(t *testing.T) {
			want, opts := realImage(t, filepath.Join(link68Dir, "vax", name+".exe"))
			wantMap, _, image, mapFile := realMap(t, filepath.Join(link68Dir, "vax", name+".map"))

			uses, _ := link68Uses(t, []string{"shr1;8"})
			myshr := &ImageLibrarySource{
				File:    "DUA1:[000000]MYSHR.OLB;1",
				Library: lib,
				Open:    func(string) (SymbolSource, error) { return uses[0], nil },
			}
			opts.Sources = append([]SymbolSource{myshr}, link68VMSSources(t, nil)...)

			img, err := Link([]Input{{File: "DUA1:[000000]MAIN1.OBJ;1", Module: link68Object(t, "main1")}}, opts)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(img.Bytes, want) {
				t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
			}

			got := img.Map(MapOptions{ImageFile: image, ImageText: opts.ImageName, MapFile: mapFile, Full: true})

			for i := range max(len(got), len(wantMap)) {
				var g, w string
				if i < len(got) {
					g = got[i]
				}

				if i < len(wantMap) {
					w = wantMap[i]
				}

				if g != w {
					t.Errorf("map line %d:\n got %q\nwant %q", i+1, g, w)
				}
			}
		})
	}
}
