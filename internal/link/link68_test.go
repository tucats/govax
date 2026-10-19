package link

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// link68Dir is Phase 68's probe (testdata/link68), whose vax directory
// holds real MACRO's objects and real LINK's images.
var link68Dir = filepath.Join("..", "..", "testdata", "link68")

// link68Object reads the object real MACRO made of one of the probe's
// sources.
func link68Object(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join(link68Dir, "vax", name+".obj"))
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

// TestLinkTransferVectorExecutable links the probe's E2, a transfer
// vector (XFR1, whose .TRANSFERs are OPR_REDEF commands) and SHR1's
// routines as an executable: the redefinitions mean nothing there, and
// the image, which has no user transfer address, is real LINK's byte
// for byte. It is linked from real MACRO's objects and from govax's.
func TestLinkTransferVectorExecutable(t *testing.T) {
	want, opts := realImage(t, filepath.Join(link68Dir, "vax", "e2.exe"))
	opts.Sources = []SymbolSource{govaxTables()}

	link := func(xfr1, shr1 *obj.Module) {
		t.Helper()

		img, err := Link([]Input{
			{File: "DUA1:[000000]XFR1.OBJ;1", Module: xfr1},
			{File: "DUA1:[000000]SHR1.OBJ;1", Module: shr1},
		}, opts)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(img.Bytes, want) {
			t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
		}
	}

	link(link68Object(t, "xfr1"), link68Object(t, "shr1"))

	var govax []*obj.Module

	for _, name := range []string{"xfr1", "shr1"} {
		src, err := os.ReadFile(filepath.Join(link68Dir, name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		govax = append(govax, macroModule(t, string(src)))
	}

	link(govax[0], govax[1])
}
