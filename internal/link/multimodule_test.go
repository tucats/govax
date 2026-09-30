package link

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/obj"
)

// linkFixtureDir is testdata/link/vax: what real VAX MACRO and LINK made
// of the Phase 30 link fixtures (testdata/link/README.md).
var linkFixtureDir = filepath.Join("..", "..", "testdata", "link", "vax")

// vaxObject reads an object real VAX MACRO made of a link fixture.
func vaxObject(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join(linkFixtureDir, name+".obj"))
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

// realLinks are the links testdata/link/link.com made: each image's
// name and the modules linked, in order. EXTERNL searches MYLIB.OLB (which
// holds DEFS), and PROG is PROG.OPT: SHARE1 and SHARE2, STACK=30,
// IDENTIFICATION="V9", and SYMBOL=LIMIT,41.
var realLinks = []struct {
	name    string
	modules []string
}{
	{"extern", []string{"extern", "defs"}},
	{"exprs", []string{"exprs", "defs"}},
	{"modes", []string{"modes", "defs"}},
	{"general", []string{"general", "defs"}},
	{"globals", []string{"globals", "defs"}},
	{"externu", []string{"extern"}},
	{"externl", []string{"extern"}},
	{"share", []string{"share1", "share2"}},
	{"sharer", []string{"share2", "share1"}},
	{"addr", []string{"addr"}},
	{"prog", []string{"share1", "share2"}},
}

// linkReal links one of realLinks as real LINK did, with sources ahead of
// MYLIB for EXTERNL, and PROG's options.
func linkReal(t *testing.T, name string, modules []string, sources []SymbolSource) (*Image, []byte) {
	t.Helper()

	want, opts := realImage(t, filepath.Join(linkFixtureDir, name+".exe"))
	opts.Sources = sources

	switch name {
	case "externl":
		data, err := os.ReadFile(filepath.Join(linkFixtureDir, "mylib.olb"))
		if err != nil {
			t.Fatal(err)
		}

		lib, err := lbr.Open(data)
		if err != nil {
			t.Fatal(err)
		}

		opts.Sources = append([]SymbolSource{&ObjectLibrarySource{File: "DUA1:[000000]MYLIB.OLB;1", Library: lib}}, sources...)

	case "prog":
		opts.StackPages, opts.Ident, opts.Symbols = 30, "V9", []Symbol{{"LIMIT", 41}}
	}

	inputs := make([]Input, len(modules))
	for i, m := range modules {
		inputs[i] = Input{File: "DUA1:[000000]" + strings.ToUpper(m) + ".OBJ;1", Module: vaxObject(t, m)}
	}

	img, err := Link(inputs, opts)
	if err != nil {
		t.Fatal(err)
	}

	return img, want
}

// p1Source defines SYS$EXIT where the P1 vector has it, as govax's
// tables and STARLET.OLB do.
func p1Source(t *testing.T) SymbolSource {
	return &TableSource{Symbols: map[string]Definition{"SYS$EXIT": {Value: p1Address(t, "SYS$EXIT")}}}
}

// withoutDST is a real image as govax would write it: without its debug
// symbol table, which real LINK builds from real MACRO's traceback
// records and govax doesn't (IHS zero, and no blocks after the image's
// own).
func withoutDST(img []byte, blocks int) []byte {
	img = bytes.Clone(img[:min(len(img), blocks)])
	clear(img[ihsOffset : ihsOffset+ihsLength])

	return img
}

// TestLinkMultiModuleMatchesRealLINK links each of realLinks from real
// MACRO's objects with only LIBRTL's offsets as a source, and checks the
// image is real LINK's byte for byte, its debug symbol table aside:
//   - externals defined by another module, and by a user library's module;
//   - undefined symbols, which are 0, and a weak reference nothing
//     defines;
//   - no transfer address (IHD$V_LNKNOTFR);
//   - concatenated and overlaid psects across modules, in both orders;
//   - a .ADDRESS of a shareable image routine (a .ADDRESS fixup, and a
//     cell for the routine);
//   - an options file's STACK=, IDENTIFICATION=, and SYMBOL=.
func TestLinkMultiModuleMatchesRealLINK(t *testing.T) {
	for _, c := range realLinks {
		t.Run(c.name, func(t *testing.T) {
			img, want := linkReal(t, c.name, c.modules, []SymbolSource{librtl, p1Source(t)})

			if want = withoutDST(want, len(img.Bytes)); !bytes.Equal(img.Bytes, want) {
				t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
			}
		})
	}
}

// TestMapMultiModuleMatchesRealLINK checks each of realLinks' maps line
// for line against real LINK's, up to its run statistics, with real LINK's
// libraries: modules from a user library, messages about undefined symbols
// and a missing transfer address, a weak reference, SYMBOL= as a cross
// reference, .ADDRESS fixups, and the pages that name the image in full.
func TestMapMultiModuleMatchesRealLINK(t *testing.T) {
	for _, c := range realLinks {
		t.Run(c.name, func(t *testing.T) {
			want, _, image, mapFile := realMap(t, filepath.Join(linkFixtureDir, c.name+".map"))
			img, _ := linkReal(t, c.name, c.modules, vmsSources(t))

			got := img.Map(MapOptions{ImageFile: image, ImageText: strings.ToUpper(c.name) + ".EXE", MapFile: mapFile})

			for i := range max(len(got), len(want)) {
				var g, w string
				if i < len(got) {
					g = got[i]
				}

				if i < len(want) {
					w = want[i]
				}

				if g != w {
					t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
				}
			}
		})
	}
}

// TestLinkUndefinedMessages checks the messages for EXTERN linked alone:
// real LINK's NUDFSYMS, a UDFSYM for each symbol, and a USEUNDEF for each
// reference, where the operand is.
func TestLinkUndefinedMessages(t *testing.T) {
	img, _ := linkReal(t, "externu", []string{"extern"}, []SymbolSource{p1Source(t)})

	var got []string
	for _, m := range img.Messages {
		got = append(got, m.String())
	}

	want := []string{
		"%LINK-W-NUDFSYMS, 2 undefined symbols:",
		"%LINK-I-UDFSYM, \tEXTDATA ",
		"%LINK-I-UDFSYM, \tSUB_ONE ",
	}

	for _, at := range []struct {
		name   string
		offset string
	}{{"SUB_ONE", "04"}, {"SUB_ONE", "0B"}, {"EXTDATA", "11"}, {"EXTDATA", "18"}, {"EXTDATA", "1F"}} {
		want = append(want, "%LINK-W-USEUNDEF, undefined symbol "+at.name+" referenced\n\tin psect CODE offset %X000000"+at.offset+
			"\n\tin module EXTERN file DUA1:[000000]EXTERN.OBJ;1")
	}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("messages:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
