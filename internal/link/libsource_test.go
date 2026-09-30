package link

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/vmsdef"
)

// vmsFile reads a file copied from a VAX (testdata/vmslib, which isn't in
// git), or skips the test.
func vmsFile(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vmslib", name))
	if err != nil {
		t.Skipf("no %s: %v", name, err)
	}

	return data
}

func vmsLibrary(t *testing.T, name string) *lbr.Library {
	t.Helper()

	l, err := lbr.Open(vmsFile(t, name))
	if err != nil {
		t.Fatal(err)
	}

	return l
}

// vmsSources are real LINK's default sources: IMAGELIB.OLB, whose images'
// symbols come from their files (only LIBRTL.EXE is here), then
// STARLET.OLB.
func vmsSources(t *testing.T) []SymbolSource {
	t.Helper()

	librtlExe := vmsFile(t, "librtl.exe")

	return []SymbolSource{
		&ImageLibrarySource{
			File:    "IMAGELIB.OLB",
			Library: vmsLibrary(t, "imagelib.olb"),
			Open: func(image string) (SymbolSource, error) {
				if image != "LIBRTL" {
					return nil, errors.New("no such file")
				}

				src, _, err := ReadShareableImage(librtlExe)

				return src, err
			},
		},
		&ObjectLibrarySource{File: "STARLET.OLB", Library: vmsLibrary(t, "starlet.olb")},
	}
}

// TestReadShareableImage reads LIBRTL.EXE's global symbol table. It agrees
// with ANALYZE/IMAGE of an image real LINK linked against it: LIB$PUT_OUTPUT
// is at ^X478, and the image's global section is 264 pages, ident 1/0x0E,
// matched LEQUAL.
func TestReadShareableImage(t *testing.T) {
	src, name, err := ReadShareableImage(vmsFile(t, "librtl.exe"))
	if err != nil {
		t.Fatal(err)
	}

	if name != "LIBRTL" {
		t.Errorf("name %q", name)
	}

	if d, ok, _ := src.Lookup("LIB$PUT_OUTPUT"); !ok || d != (Definition{Image: "LIBRTL", Value: 0x478}) {
		t.Errorf("LIB$PUT_OUTPUT = %+v, %v", d, ok)
	}

	// IMAGELIB's module header counts 306 references to LIBRTL: its name,
	// and each symbol.
	if len(src.Symbols) != 305 {
		t.Errorf("%d symbols, want 305", len(src.Symbols))
	}

	want := SharedImage{Name: "LIBRTL", Pages: 264, MajorID: 1, MinorID: 0x0E, Match: MatchLEQ}
	if i, ok := src.Image("LIBRTL"); !ok || i != want {
		t.Errorf("image %+v, want %+v", i, want)
	}

	if _, _, err := ReadShareableImage(vmsFile(t, "librtl.exe")[:600]); err == nil {
		t.Error("a truncated image's symbol table was read")
	}
}

// TestLinkFromVMSLibraries links hello with only real LINK's sources, and
// gets GV_HELLO.EXE byte for byte, as with govax's own tables: IMAGELIB
// says LIB$PUT_OUTPUT is in LIBRTL, and LIBRTL.EXE gives its offset and the
// global section ISD's facts.
func TestLinkFromVMSLibraries(t *testing.T) {
	want, opts := realImage(t, filepath.Join(fixtureDir, "vax", "govax", "gv_hello.exe"))
	opts.Sources = vmsSources(t)

	for _, from := range []string{"govax", "real"} {
		t.Run(from, func(t *testing.T) {
			m := realObject(t, "hello")
			if from == "govax" {
				m = govaxObject(t, "hello")
			}

			img, err := Link([]Input{{File: "hello.obj", Module: m}}, opts)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(img.Bytes, want) {
				t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
			}
		})
	}
}

// TestLinkAddsLibraryModules links a program that calls SYS$EXIT and uses
// SS$_NORMAL. Only STARLET.OLB defines them, in modules SYS$P1_VECTOR and
// SYS$SSDEF, which the link adds; they hold only absolute symbols and
// empty absolute psects, so the image is the one govax's own tables give.
func TestLinkAddsLibraryModules(t *testing.T) {
	src := `.PSECT CODE,NOWRT,EXE
	.ENTRY START,^M<>
	PUSHL #SS$_NORMAL
	CALLS #1,G^SYS$EXIT
	.END START`
	m := macroModule(t, src)

	opts := Options{ImageName: "T", Time: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC), Sources: vmsSources(t)}

	img, err := Link([]Input{{File: "t.obj", Module: m}}, opts)
	if err != nil {
		t.Fatal(err)
	}

	opts.Sources = []SymbolSource{&TableSource{Symbols: map[string]Definition{
		"SYS$EXIT":   {Value: p1Address(t, "SYS$EXIT")},
		"SS$_NORMAL": {Value: 1},
	}}}

	want, err := Link([]Input{{File: "t.obj", Module: macroModule(t, src)}}, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(img.Bytes, want.Bytes) {
		t.Errorf("image differs from govax's tables':\n%s", diffBlocks(img.Bytes, want.Bytes))
	}
}

// p1Address is a system service's address in the P1 vector.
func p1Address(t *testing.T, name string) uint32 {
	t.Helper()

	for _, e := range vmsdef.P1VectorTable {
		if e.Name == name {
			return e.Addr
		}
	}

	t.Fatalf("no %s in the P1 vector", name)

	return 0
}

// TestImageLibraryNeedsItsImage links a call to SMG$CREATE_PASTEBOARD,
// which IMAGELIB says is in SMGSHR. With no SMGSHR.EXE to give its offset,
// the link fails: it doesn't go on to STARLET.OLB, whose object modules
// would put a private copy of the routine in the image.
func TestImageLibraryNeedsItsImage(t *testing.T) {
	m := macroModule(t, ".PSECT CODE,NOWRT,EXE\n.ENTRY START,^M<>\nCALLS #0,G^SMG$CREATE_PASTEBOARD\nRET\n.END START")

	_, err := Link([]Input{{File: "t.obj", Module: m}}, Options{ImageName: "T", Sources: vmsSources(t)})
	if err == nil || !strings.Contains(err.Error(), "SMGSHR") {
		t.Fatalf("err = %v, want one naming SMGSHR", err)
	}
}

// moduleSource is an object library in memory: each symbol's module.
type moduleSource map[string]*Input

func (s moduleSource) Lookup(name string) (Definition, bool, error) {
	in, ok := s[name]

	return Definition{Module: in}, ok, nil
}

func (moduleSource) Image(string) (SharedImage, bool) { return SharedImage{}, false }

// TestLinkLibraryModulesChain links a program that calls FIRST, from a
// library module that calls SECOND, from another library module. Both
// modules are added, and their code runs where the image puts it.
func TestLinkLibraryModulesChain(t *testing.T) {
	first := &Input{File: "LIB(FIRST)", Module: macroModule(t, ".PSECT CODE,NOWRT,EXE\n.ENTRY FIRST,^M<>\nCALLS #0,G^SECOND\nRET\n.END")}
	second := &Input{File: "LIB(SECOND)", Module: macroModule(t, ".PSECT CODE,NOWRT,EXE\n.ENTRY SECOND,^M<>\nMOVL #VALUE,R0\nRET\n.END")}
	lib := moduleSource{"FIRST": first, "SECOND": second}
	values := &TableSource{Symbols: map[string]Definition{"VALUE": {Value: 42}}}

	m := macroModule(t, ".PSECT CODE,NOWRT,EXE\n.ENTRY START,^M<>\nCALLS #0,G^FIRST\nRET\n.END START")

	img, err := Link([]Input{{File: "t.obj", Module: m}}, Options{ImageName: "T", Sources: []SymbolSource{lib, values}})
	if err != nil {
		t.Fatal(err)
	}

	// Each routine is 10 bytes: START and FIRST are an entry mask, CALLS,
	// #0, a 5-byte G^ operand, and RET; SECOND is a mask, MOVL, a 5-byte
	// I^#VALUE, R0, and RET. All three are in CODE.
	if len(img.Psects) != 1 || img.Psects[0].Length != 10+10+10 {
		t.Fatalf("psects %+v", img.Psects)
	}

	// A module that doesn't define the symbol it was added for.
	lib["THIRD"] = second
	m = macroModule(t, ".PSECT CODE,NOWRT,EXE\n.ENTRY START,^M<>\nCALLS #0,G^THIRD\nRET\n.END START")

	_, err = Link([]Input{{File: "t.obj", Module: m}}, Options{ImageName: "T", Sources: []SymbolSource{lib, values}})
	if err == nil || !strings.Contains(err.Error(), "doesn't define THIRD") {
		t.Errorf("err = %v", err)
	}
}
