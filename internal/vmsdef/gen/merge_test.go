package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

func TestMergeSymbols(t *testing.T) {
	symbols := map[string]uint32{"SS$_NORMAL": 1, "SS$_ACCVIO": 12}
	defs := map[string]uint32{"SS$_NORMAL": 1, "SS$_BADPARAM": 20, "SS$_ACCVIO": 13}

	r := mergeSymbols(symbols, defs, false)

	if !reflect.DeepEqual(r.added, []string{"SS$_BADPARAM = 0x14"}) || r.same != 1 || len(r.changed) != 0 {
		t.Errorf("mergeSymbols = %+v", r)
	}

	if !reflect.DeepEqual(r.conflicts, []string{"SS$_ACCVIO: 0xc, not 0xd"}) {
		t.Errorf("conflicts = %q", r.conflicts)
	}

	if want := map[string]uint32{"SS$_NORMAL": 1, "SS$_ACCVIO": 12, "SS$_BADPARAM": 20}; !reflect.DeepEqual(symbols, want) {
		t.Errorf("symbols = %v, want %v (a conflict mustn't change the table)", symbols, want)
	}
}

func TestMergeSymbols_replace(t *testing.T) {
	symbols := map[string]uint32{"SS$_ACCVIO": 12}

	r := mergeSymbols(symbols, map[string]uint32{"SS$_ACCVIO": 13}, true)

	if len(r.conflicts) != 0 || !reflect.DeepEqual(r.changed, []string{"SS$_ACCVIO: 0xc -> 0xd"}) {
		t.Errorf("mergeSymbols = %+v", r)
	}

	if symbols["SS$_ACCVIO"] != 13 {
		t.Errorf("SS$_ACCVIO = %d, want 13", symbols["SS$_ACCVIO"])
	}
}

func TestAddSource(t *testing.T) {
	got := addSource(addSource([]string{"ssdef.txt"}, "iodef.sdl"), "ssdef.txt")

	if want := []string{"ssdef.txt", "iodef.sdl"}; !reflect.DeepEqual(got, want) {
		t.Errorf("addSource = %q, want %q", got, want)
	}
}

// TestGenerateSymbols_roundTrip: with nothing merged, gen writes the
// committed file back unchanged, so the file is exactly what gen makes of
// the table it holds.
func TestGenerateSymbols_roundTrip(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", symbolsFile))
	if err != nil {
		t.Fatal(err)
	}

	if got := generateSymbols(vmsdef.Symbols, vmsdef.SymbolSources); !bytes.Equal(got, want) {
		t.Errorf("generateSymbols doesn't reproduce %s", symbolsFile)
	}
}

func TestParseDefines_prefix(t *testing.T) {
	src := "#define FAB$C_BID 3\n#define RAB$C_BID 0x1\n#define fab$w_ifi fab$r_ifi_overlay.fab$w_ifi\n#define NAM$C_BID 2\n"

	if got, want := parseDefines(src, ""), map[string]uint32{"FAB$C_BID": 3, "RAB$C_BID": 1, "NAM$C_BID": 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("parseDefines = %v, want %v", got, want)
	}

	if got, want := parseDefines(src, "RAB$"), map[string]uint32{"RAB$C_BID": 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("parseDefines(RAB$) = %v, want %v", got, want)
	}
}

// TestInputFlags: -prefix, -sdl-stop, and -into apply to the inputs after them,
// in command-line order.
func TestInputFlags(t *testing.T) {
	var list inputList

	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.Var(inputFlag{&list, "h"}, "h", "")
	fs.Var(inputFlag{&list, "sdl"}, "sdl", "")
	fs.Var(inputFlag{&list, "bliss"}, "bliss", "")
	fs.Var(settingFlag{&list.prefix}, "prefix", "")
	fs.Var(settingFlag{&list.stop}, "sdl-stop", "")
	fs.Var(settingFlag{&list.into}, "into", "")
	fs.Var(inputFlag{&list, "olb"}, "olb", "")

	if err := fs.Parse([]string{"-h", "a.h", "-prefix", "SS$_", "-bliss", "b.txt", "-sdl-stop", "$X", "-prefix", "", "-sdl", "c.sdl",
		"-olb", "d.olb", "-into", "symbols", "-olb", "e.olb"}); err != nil {
		t.Fatal(err)
	}

	want := []input{
		{kind: "h", path: "a.h"},
		{kind: "bliss", path: "b.txt", prefix: "SS$_"},
		{kind: "sdl", path: "c.sdl", stop: "$X"},
		{kind: "olb", path: "d.olb", stop: "$X"},
		{kind: "olb", path: "e.olb", stop: "$X", into: "symbols"},
	}

	if !reflect.DeepEqual(list.inputs, want) {
		t.Errorf("inputs = %+v, want %+v", list.inputs, want)
	}
}

// TestInputRead_sdlStop: an SDL source is read only as far as the
// -sdl-stop module, and -prefix filters what's left.
func TestInputRead_sdlStop(t *testing.T) {
	src := `module $AAADEF;
constant "ONE" equals 1 prefix AAA$ tag C;
constant "TWO" equals 2 prefix BBB$ tag C;
end_module $AAADEF;
module $ZZZDEF;
constant "THREE" equals 3 prefix ZZZ$ tag C;
end_module $ZZZDEF;
`
	path := filepath.Join(t.TempDir(), "test.sdl")
	
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := input{kind: "sdl", path: path, stop: "$ZZZDEF"}.read()
	if err != nil {
		t.Fatal(err)
	}

	if want := map[string]uint32{"AAA$C_ONE": 1, "BBB$C_TWO": 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("read = %v, want %v", got, want)
	}

	got, err = input{kind: "sdl", path: path, prefix: "BBB$"}.read()
	if err != nil {
		t.Fatal(err)
	}

	if want := map[string]uint32{"BBB$C_TWO": 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("read with a prefix = %v, want %v", got, want)
	}
}

func TestMergeImage(t *testing.T) {
	images := map[string]vmsdef.SharedImage{}
	symbols := map[string]vmsdef.ImageSymbol{}
	librtl := vmsdef.SharedImage{Pages: 264, MajorID: 1, MinorID: 0xE, Match: 2}
	defs := map[string]vmsdef.ImageSymbol{"LIB$GET_INPUT": {Image: "LIBRTL", Value: 0x410}, "LIB$_X": {Value: 7}}

	if r := mergeImage(images, symbols, "LIBRTL", librtl, defs, false); len(r.added) != 3 || len(r.conflicts) != 0 {
		t.Errorf("mergeImage = %+v, want 3 added", r)
	}

	if r := mergeImage(images, symbols, "LIBRTL", librtl, defs, false); r.same != 3 {
		t.Errorf("merging again = %+v, want 3 the same", r)
	}

	newer := librtl
	newer.MinorID = 0xF

	r := mergeImage(images, symbols, "LIBRTL", newer, map[string]vmsdef.ImageSymbol{"LIB$GET_INPUT": {Image: "LIBRTL", Value: 0x418}}, false)
	if len(r.conflicts) != 2 || images["LIBRTL"] != librtl || symbols["LIB$GET_INPUT"].Value != 0x410 {
		t.Errorf("mergeImage = %+v, and changed the tables without -replace", r)
	}
}

// TestGenerateImagesAndLibrary_roundTrip: with nothing merged, gen writes
// the committed image and library files back unchanged.
func TestGenerateImagesAndLibrary_roundTrip(t *testing.T) {
	for file, got := range map[string][]byte{
		imagesFile:  generateImages(vmsdef.SharedImages, vmsdef.ImageSymbols, vmsdef.ImageSources),
		libraryFile: generateLibrary(vmsdef.LibrarySymbols, vmsdef.LibrarySources),
	} {
		want, err := os.ReadFile(filepath.Join("..", file))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("gen doesn't reproduce %s", file)
		}
	}
}

func TestParseValues(t *testing.T) {
	src := "# a comment\n\n# $XABKEYDEF\nXAB$C_KEY = 0x15\nXAB$B_DTP = 19\nXAB$B_AID undefined\nSS$_NORMAL = 1\nXAB$C_KEY = 0x15\n"

	got, err := parseValues(src, "XAB$")
	if err != nil {
		t.Fatal(err)
	}

	if want := map[string]uint32{"XAB$C_KEY": 0x15, "XAB$B_DTP": 19}; !reflect.DeepEqual(got, want) {
		t.Errorf("parseValues = %v, want %v", got, want)
	}

	if _, err := parseValues("A$X = 1\nA$X = 2\n", ""); err == nil {
		t.Error("parseValues accepted a name listed with two values")
	}
}
