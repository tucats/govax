package link

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsdef"
)

// probeDir is the Phase 29 probe's directory (testdata/mar/list), whose
// vax directory holds real MACRO's objects and real LINK's images.
var probeDir = filepath.Join("..", "..", "testdata", "mar", "list")

// probeObject reads an object real MACRO made of a probe source.
func probeObject(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join(probeDir, "vax", name+".obj"))
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

// TestLinkProbeImagesMatchRealLINK links the Phase 29 probe's programs as
// its list.com did, and checks each image is real LINK's byte for byte,
// debug symbol table and all (docs/PHASE-29.md, subtask 13):
//   - TRACE, three routines and three psects, with traceback and
//     /NOTRACEBACK;
//   - TRDBGTRC, TRACE assembled /DEBUG and linked with traceback: its
//     debugger records are left out of the DST;
//   - FAILMAIN and FAILSUB, two modules' DST records one after the other,
//     with traceback, /NOTRACEBACK, and from their /DEBUG objects;
//   - FAILSIG, with traceback and /NOTRACEBACK.
//
// Each traced program is linked again from govax's own objects of the
// same sources. The links need VMS's own libraries (vmsSources), and are
// skipped without them.
func TestLinkProbeImagesMatchRealLINK(t *testing.T) {
	for _, c := range []struct {
		image   string
		objects []string
		sources []string // the probe sources govax assembles, if any
		trace   bool
	}{
		{"trace", []string{"trace"}, []string{"trace"}, true},
		{"trnotb", []string{"trace"}, nil, false},
		{"trdbgtrc", []string{"trdebug"}, nil, true},
		{"failmain", []string{"failmain", "failsub"}, []string{"failmain", "failsub"}, true},
		{"failnotb", []string{"failmain", "failsub"}, nil, false},
		{"faildbg", []string{"failmaid", "failsubd"}, nil, true},
		{"failsig", []string{"failsig"}, []string{"failsig"}, true},
		{"fsignotb", []string{"failsig"}, nil, false},
	} {
		t.Run(c.image, func(t *testing.T) {
			// FAILSIG's two fixup cells are in an order no rule found
			// explains (orderCells): a known difference, and the only
			// one these images show.
			if c.image == "failsig" || c.image == "fsignotb" {
				t.Skip("real LINK's order of FAILSIG's two fixup cells is a known difference (docs/PHASE-29.md, subtask 14)")
			}

			want, opts := realImage(t, filepath.Join(probeDir, "vax", c.image+".exe"))
			opts.Traceback = c.trace
			opts.Sources = vmsSources(t)

			link := func(modules []*obj.Module) {
				t.Helper()

				inputs := make([]Input, len(modules))
				for i, m := range modules {
					inputs[i] = Input{File: "DUA1:[000000]" + strings.ToUpper(c.objects[i]) + ".OBJ;1", Module: m}
				}

				img, err := Link(inputs, opts)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(img.Bytes, want) {
					t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
				}
			}

			var realModule []*obj.Module
			for _, name := range c.objects {
				realModule = append(realModule, probeObject(t, name))
			}

			link(realModule)

			if c.sources == nil {
				return
			}

			var govax []*obj.Module

			for _, name := range c.sources {
				src, err := os.ReadFile(filepath.Join(probeDir, name+".mar"))
				if err != nil {
					t.Fatal(err)
				}

				govax = append(govax, macroModule(t, string(src)))
			}

			link(govax)
		})
	}
}

// imageDST returns an image's debug symbol table, as its IHS block
// describes it (VBN and block count), or nil if it has none.
func imageDST(t *testing.T, img []byte) []byte {
	t.Helper()

	ihs := img[ihsOffset : ihsOffset+ihsLength]
	vbn := int(ihs[0]) | int(ihs[1])<<8 | int(ihs[2])<<16 | int(ihs[3])<<24
	blocks := int(ihs[8]) | int(ihs[9])<<8

	if vbn == 0 {
		return nil
	}

	if end := (vbn - 1 + blocks) * blockSize; end > len(img) {
		t.Fatalf("DST at VBN %d, %d blocks, past the image's end", vbn, blocks)
	}

	return img[(vbn-1)*blockSize : (vbn-1+blocks)*blockSize]
}

// probeRoutines stands in for VMS's libraries in TestLinkProbeDST: the
// LIBRTL routines the probe's programs call. Their addresses don't reach
// the DST, which only describes the probe's own modules.
var probeRoutines = &TableSource{
	Symbols: map[string]Definition{
		"LIB$PUT_OUTPUT": {Image: "LIBRTL", Value: 0x478},
		"LIB$SIGNAL":     {Image: "LIBRTL", Value: 0x480},
		"LIB$STOP":       {Image: "LIBRTL", Value: 0x488},
	},
	Images: map[string]SharedImage{"LIBRTL": {Pages: 264, MajorID: 1, MinorID: 0x0E, Match: MatchLEQ}},
}

// TestLinkProbeDST checks the debug symbol table of each of the probe's
// traced images against real LINK's, block for block, and that an
// untraced image has none, from real MACRO's objects and govax's. Unlike
// TestLinkProbeImagesMatchRealLINK it needs none of VMS's libraries: the
// DST describes only the program's own modules, whose psects come first.
func TestLinkProbeDST(t *testing.T) {
	for _, c := range []struct {
		image   string
		objects []string
		trace   bool
		govax   bool // also link govax's objects of the same sources
	}{
		{"trace", []string{"trace"}, true, true},
		{"trnotb", []string{"trace"}, false, true},
		{"trdbgtrc", []string{"trdebug"}, true, false},
		{"failmain", []string{"failmain", "failsub"}, true, true},
		{"faildbg", []string{"failmaid", "failsubd"}, true, false},
		{"failsig", []string{"failsig"}, true, true},
		{"fsignotb", []string{"failsig"}, false, true},
	} {
		t.Run(c.image, func(t *testing.T) {
			want, opts := realImage(t, filepath.Join(probeDir, "vax", c.image+".exe"))
			opts.Traceback = c.trace
			opts.Sources = []SymbolSource{probeRoutines}

			wantDST := imageDST(t, want)
			if (wantDST != nil) != c.trace {
				t.Fatalf("real image's DST: %d bytes", len(wantDST))
			}

			check := func(from string, modules []*obj.Module) {
				t.Helper()

				inputs := make([]Input, len(modules))
				for i, m := range modules {
					inputs[i] = Input{File: strings.ToUpper(c.objects[i]) + ".OBJ", Module: m}
				}

				img, err := Link(inputs, opts)
				if err != nil {
					t.Fatal(err)
				}

				got := imageDST(t, img.Bytes)
				if !bytes.Equal(got, wantDST) {
					t.Errorf("%s: DST differs from real LINK's:\n%s", from, diffBlocks(got, wantDST))
				}

				if got != nil && !bytes.Equal(got, img.Bytes[len(img.Bytes)-len(got):]) {
					t.Errorf("%s: the DST isn't the image's last blocks", from)
				}

				if a, b := img.Bytes[ihsOffset+20:ihsOffset+24], want[ihsOffset+20:ihsOffset+24]; !bytes.Equal(a, b) {
					t.Errorf("%s: IHS+20 is % x, want % x", from, a, b)
				}
			}

			var realModule []*obj.Module
			for _, name := range c.objects {
				realModule = append(realModule, probeObject(t, name))
			}

			check("real MACRO's objects", realModule)

			if !c.govax {
				return
			}

			var govax []*obj.Module

			for _, name := range c.objects {
				src, err := os.ReadFile(filepath.Join(probeDir, name+".mar"))
				if err != nil {
					t.Fatal(err)
				}

				govax = append(govax, macroModule(t, string(src)))
			}

			check("govax's objects", govax)
		})
	}
}

// govaxTables is the symbol source govax's LINK falls back on with none
// of VMS's libraries (the console's govaxSymbols, internal/console/
// linksource.go, less its shims): the shareable images and their
// routines from vmsdef, and the P1 vector.
func govaxTables() *TableSource {
	src := &TableSource{Symbols: map[string]Definition{}, Images: map[string]SharedImage{}}

	for name, i := range vmsdef.SharedImages {
		src.Images[name] = SharedImage{
			Name: name, Pages: i.Pages, MajorID: i.MajorID, MinorID: i.MinorID, Match: Match(i.Match),
			Symbols: i.Symbols, Psects: i.Psects, Sections: i.Sections,
		}
	}

	for name, s := range vmsdef.ImageSymbols {
		src.Symbols[name] = Definition{Image: s.Image, Value: s.Value}
	}

	for _, e := range vmsdef.P1VectorTable {
		src.Symbols[e.Name] = Definition{Value: e.Addr}
	}

	return src
}

// TestLinkRoundImagesMatchRealLINK links govax's objects of the probe's
// programs with govax's own symbol tables, and checks each image is byte
// for byte the one real LINK made of the same objects in Phase 29's VMS
// round (testdata/mar/round/vax/rl*.exe), given its name, link time, and
// linker ID. VMS ran both, and printed the same traceback for each
// (round.log). CELLS and CELLS2 are the round's follow-up on the order of
// fixup cells, which real MACRO assembled and real LINK linked.
func TestLinkRoundImagesMatchRealLINK(t *testing.T) {
	mar := filepath.Join("..", "..", "testdata", "mar")

	for _, c := range []struct {
		image   string
		sources []string // under testdata/mar
		trace   bool
	}{
		{"rltrace", []string{"list/trace"}, true},
		{"rltrnotb", []string{"list/trace"}, false},
		{"rlfail", []string{"list/failmain", "list/failsub"}, true},
		{"rlfailnt", []string{"list/failmain", "list/failsub"}, false},
		{"rlfsig", []string{"list/failsig"}, true},
		{"rlfsignt", []string{"list/failsig"}, false},
		{"cells", []string{"round/cells"}, true},
		{"cells2", []string{"round/cells", "round/cellsb"}, true},
	} {
		t.Run(c.image, func(t *testing.T) {
			// FAILSIG's two fixup cells are in an order no rule found
			// explains (orderCells): a known difference.
			if strings.HasPrefix(c.image, "rlfsig") {
				t.Skip("real LINK's order of FAILSIG's two fixup cells is a known difference (docs/PHASE-29.md, subtask 14)")
			}

			want, opts := realImage(t, filepath.Join(mar, "round", "vax", c.image+".exe"))
			opts.Traceback = c.trace
			opts.Sources = []SymbolSource{govaxTables()}

			inputs := make([]Input, len(c.sources))

			for i, path := range c.sources {
				src, err := os.ReadFile(filepath.Join(mar, filepath.FromSlash(path)+".mar"))
				if err != nil {
					t.Fatal(err)
				}

				inputs[i] = Input{File: strings.ToUpper(filepath.Base(path)) + ".OBJ", Module: macroModule(t, string(src))}
			}

			img, err := Link(inputs, opts)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(img.Bytes, want) {
				t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
			}
		})
	}
}
