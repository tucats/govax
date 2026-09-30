package link

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/obj"
)

// fixtureDir is testdata/mar.
var fixtureDir = filepath.Join("..", "..", "testdata", "mar")

// govaxObject assembles testdata/mar/name.mar with govax's MACRO.
func govaxObject(t *testing.T, name string) *obj.Module {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(fixtureDir, name+".mar"))
	if err != nil {
		t.Fatal(err)
	}

	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)

	if _, err := a.Assemble(string(src)); err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	m, err := a.Object(asm.ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// realObject reads the object real VAX MACRO made from name.mar.
func realObject(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join(fixtureDir, "vax", name+".obj"))
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

// realImage reads an image real LINK made, and the options that give its
// header: its name, link time, and linker ID.
func realImage(t *testing.T, path string) ([]byte, Options) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	ihi := data[ihiOffset:]
	q := binary.LittleEndian.Uint64(ihi[56:])

	return data, Options{
		ImageName: string(ihi[1 : 1+ihi[0]]),
		Time:      time.Unix(int64(q/10_000_000)+vmsEpoch, int64(q%10_000_000)*100).UTC(),
		LinkerID:  string(ihi[65 : 65+ihi[64]]),
		Traceback: true,
	}
}

// TestLinkMatchesRealLINK links the self-contained fixtures, from govax's
// objects and from real MACRO's, and checks each image is byte for byte
// the one real LINK V11-39 made from govax's object (testdata/mar/vax/
// govax/gv_*.exe), given its name, link time, and linker ID. Real LINK
// puts no debug symbol table in those images, since govax's objects have
// no traceback records, and govax's LINK skips real MACRO's.
func TestLinkMatchesRealLINK(t *testing.T) {
	for _, name := range []string{"psects", "entry"} {
		want, opts := realImage(t, filepath.Join(fixtureDir, "vax", "govax", "gv_"+name+".exe"))

		for _, from := range []string{"govax", "real"} {
			t.Run(name+"/"+from, func(t *testing.T) {
				m := realObject(t, name)
				if from == "govax" {
					m = govaxObject(t, name)
				}

				img, err := Link([]Input{{File: name + ".obj", Module: m}}, opts)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(img.Bytes, want) {
					t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
				}
			})
		}
	}
}

// diffBlocks describes where two images differ, a few lines per block.
func diffBlocks(got, want []byte) string {
	var sb strings.Builder

	if len(got) != len(want) {
		fmt.Fprintf(&sb, "%d bytes, want %d\n", len(got), len(want))
	}

	shown := 0

	for i := 0; i < min(len(got), len(want)) && shown < 20; i += 16 {
		g, w := got[i:min(i+16, len(got))], want[i:min(i+16, len(want))]
		if !bytes.Equal(g, w) {
			fmt.Fprintf(&sb, "%04X: % x\n want % x\n", i, g, w)
			shown++
		}
	}

	return sb.String()
}

// librtl is what VMS 7.3's LIBRTL looks like to an image linked against
// it: LIB$PUT_OUTPUT's offset, and the global section ISD's size, ident,
// and match control (ANALYZE/IMAGE of HELLO.EXE, docs/PHASE-30.md).
var librtl = &TableSource{
	Symbols: map[string]Definition{"LIB$PUT_OUTPUT": {Image: "LIBRTL", Value: 0x478}},
	Images:  map[string]SharedImage{"LIBRTL": {Pages: 264, MajorID: 1, MinorID: 0x0E, Match: MatchLEQ}},
}

// TestLinkSharedImageMatchesRealLINK links hello, which calls
// LIB$PUT_OUTPUT with a general mode operand, and checks the image is
// byte for byte GV_HELLO.EXE: the code reaches the routine through a cell
// in the fixup section, which lists LIBRTL, and a global section ISD maps
// LIBRTL.
func TestLinkSharedImageMatchesRealLINK(t *testing.T) {
	want, opts := realImage(t, filepath.Join(fixtureDir, "vax", "govax", "gv_hello.exe"))
	opts.Sources = []SymbolSource{librtl}

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

// TestLinkNoTracebackMatchesRealLINK compares LINK/NOTRACEBACK with the
// image real LINK/NOTRACEBACK made from real MACRO's psects.obj
// (testdata/mar/vax/psects-notraceback.exe): the user transfer address
// comes first, and there's no debug symbol table.
func TestLinkNoTracebackMatchesRealLINK(t *testing.T) {
	want, opts := realImage(t, filepath.Join(fixtureDir, "vax", "psects-notraceback.exe"))
	opts.Traceback = false

	for _, from := range []string{"govax", "real"} {
		t.Run(from, func(t *testing.T) {
			m := realObject(t, "psects")
			if from == "govax" {
				m = govaxObject(t, "psects")
			}

			img, err := Link([]Input{{File: "psects.obj", Module: m}}, opts)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(img.Bytes, want) {
				t.Errorf("image differs from real LINK/NOTRACEBACK's:\n%s", diffBlocks(img.Bytes, want))
			}
		})
	}
}

func TestLinkPsectLayout(t *testing.T) {
	img, err := Link([]Input{{File: "psects.obj", Module: govaxObject(t, "psects")}}, Options{Traceback: true})
	if err != nil {
		t.Fatal(err)
	}

	// The map real LINK wrote (testdata/mar/vax/psects.map).
	want := []PsectInfo{
		{Name: "DATA", Base: 0x200, Length: 76},
		{Name: "CODE", Base: 0x400, Length: 14},
		{Name: ". BLANK .", Base: 0x600, Length: 4},
	}

	if len(img.Psects) != len(want) {
		t.Fatalf("psects = %+v", img.Psects)
	}

	for i, w := range want {
		g := img.Psects[i]
		if g.Name != w.Name || g.Base != w.Base || g.Length != w.Length {
			t.Errorf("psect %d = %s at %X, %d bytes; want %s at %X, %d bytes", i, g.Name, g.Base, g.Length, w.Name, w.Base, w.Length)
		}
	}

	if !img.HasTransfer || img.Transfer != 0x400 {
		t.Errorf("transfer = %X, %v; want 400", img.Transfer, img.HasTransfer)
	}
}

// macroModule assembles MACRO source into a module.
func macroModule(t *testing.T, src string) *obj.Module {
	t.Helper()

	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)

	if _, err := a.Assemble(src); err != nil {
		t.Fatal(err)
	}

	m, err := a.Object(asm.ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// linkSources links MACRO sources, one module each.
func linkSources(t *testing.T, srcs ...string) (*Image, error) {
	t.Helper()

	inputs := make([]Input, len(srcs))
	for i, src := range srcs {
		inputs[i] = Input{File: fmt.Sprintf("m%d.obj", i), Module: macroModule(t, src)}
	}

	return Link(inputs, Options{ImageName: "T"})
}

// isds decodes an image's ISDs: page count, VPN, and flags.
func isds(img *Image) [][3]uint32 {
	var out [][3]uint32

	for p := isdOffset; ; {
		size := int(binary.LittleEndian.Uint16(img.Bytes[p:]))
		if size == 0 {
			return out
		}

		out = append(out, [3]uint32{
			uint32(binary.LittleEndian.Uint16(img.Bytes[p+2:])),
			binary.LittleEndian.Uint32(img.Bytes[p+4:]),
			binary.LittleEndian.Uint32(img.Bytes[p+8:]),
		})
		p += size
	}
}

// TestLinkDemandZero checks demand-zero compression: a writable section
// with nothing stored in it is demand-zero, and a run of at least five
// empty pages in a writable section is split off; four aren't.
func TestLinkDemandZero(t *testing.T) {
	img, err := linkSources(t, `.PSECT A,WRT,NOEXE
	.LONG 1
	.BLKB 512*6
	.LONG 2
	.BLKB 512*4
	.LONG 3
.PSECT Z,WRT,EXE
	.BLKB 100
.PSECT C,NOWRT,EXE
	RSB
	.END`)
	if err != nil {
		t.Fatal(err)
	}

	got := isds(img)
	want := [][3]uint32{
		// A is 11 pages: .LONG 1 in page 0, .LONG 2 in page 6, .LONG 3
		// in page 10.
		{1, 1, isdLASTCLU | isdWRT | isdCRF},   // page 0
		{5, 2, isdLASTCLU | isdDZRO | isdWRT},  // pages 1-5, empty
		{5, 7, isdLASTCLU | isdWRT | isdCRF},   // pages 6-10: only 4 empty ones
		{1, 12, isdLASTCLU},                    // C
		{1, 13, isdLASTCLU | isdDZRO | isdWRT}, // Z, nothing stored
		{1, 14, isdFIXUPVEC | isdWRT | isdCRF},
		{20, 1<<22 - 20, isdTypeUserStack | isdLASTCLU | isdDZRO | isdWRT},
	}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("ISDs = %v\nwant   %v", got, want)
	}
}

func TestLinkSymbolsAndTransfer(t *testing.T) {
	main := ".PSECT C,NOWRT,EXE\n.ENTRY START,^M<>\nRET\n.END START"

	if _, err := linkSources(t, main, main); err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Errorf("duplicate definition: error = %v", err)
	}

	if _, err := linkSources(t, main, ".PSECT D,NOEXE\nX:: .LONG 0\n.END X"); err == nil || !strings.Contains(err.Error(), "second transfer") {
		t.Errorf("two transfer addresses: error = %v", err)
	}

	// An undefined symbol is a warning, as in real LINK, and the image is
	// still made (TestLinkUndefinedMessages has the messages).
	if img, err := linkSources(t, ".PSECT C,NOWRT,EXE\nCALLS #0,MISSING\n.END"); err != nil || len(img.Messages) == 0 || img.Messages[1].String() != "%LINK-I-UDFSYM, \tMISSING " {
		t.Errorf("undefined symbol: error = %v", err)
	}

	// A weak definition doesn't clash with a strong one.
	img, err := linkSources(t, main, ".WEAK START\n.PSECT D,NOEXE\nSTART:: .LONG 0")
	if err != nil {
		t.Fatalf("weak and strong definitions: %v", err)
	}

	// D (WRT, NOEXE) comes before C (NOWRT, EXE) in Table 6-1.
	if !img.HasTransfer || img.Transfer != 0x400 {
		t.Errorf("transfer = %X, %v", img.Transfer, img.HasTransfer)
	}
}

// TestLinkPsectContributions checks concatenated and overlaid psects
// across modules, and alignment.
func TestLinkPsectContributions(t *testing.T) {
	img, err := linkSources(t,
		".PSECT CAT,NOEXE,WRT,LONG\n.BYTE 1\n.PSECT OVL,NOEXE,WRT,OVR\n.BLKB 10",
		".PSECT CAT,NOEXE,WRT,LONG\n.BYTE 2\n.PSECT OVL,NOEXE,WRT,OVR\n.BLKB 30")
	if err != nil {
		t.Fatal(err)
	}

	lengths := map[string]uint32{}
	for _, p := range img.Psects {
		lengths[p.Name] = p.Length
	}

	// The second module's CAT starts at the next longword: 4 + 1 bytes.
	if lengths["CAT"] != 5 || lengths["OVL"] != 30 {
		t.Errorf("lengths = %v, want CAT 5, OVL 30", lengths)
	}

	data := img.Bytes[512:]
	if data[0] != 1 || data[4] != 2 {
		t.Errorf("CAT's bytes = % x", data[:8])
	}
}

// TestLinkSharedReferences checks the fixup section for several targets:
// one cell per target, even when it's called twice, a G^ fixup list per
// shareable image in the order first referred to, and a global section
// ISD for each, after the user stack's.
func TestLinkSharedReferences(t *testing.T) {
	src := &TableSource{
		Symbols: map[string]Definition{
			"A$ONE": {Image: "AIMG", Value: 0x10},
			"A$TWO": {Image: "AIMG", Value: 0x20},
			"B$ONE": {Image: "BIMG", Value: 0x30},
		},
		Images: map[string]SharedImage{"BIMG": {Pages: 3, MajorID: 2, MinorID: 5, Match: MatchEqual}},
	}

	m := macroModule(t, `.PSECT C,NOWRT,EXE
	.ENTRY GO,^M<>
	CALLS #0,G^B$ONE
	CALLS #0,G^A$ONE
	CALLS #0,G^A$TWO
	CALLS #0,G^A$ONE
	RET
	.END GO`)

	img, err := Link([]Input{{File: "m.obj", Module: m}}, Options{ImageName: "T", Sources: []SymbolSource{src}})
	if err != nil {
		t.Fatal(err)
	}

	le := binary.LittleEndian
	fix := img.Bytes[2*blockSize:] // header, C, then the fixup section at 0x400

	// B's list (1 cell), then A's (2 cells), then the zero count.
	wantLists := []uint32{1, 1, 0x30, 2, 2, 0x10, 0x20, 0}
	for i, w := range wantLists {
		if got := le.Uint32(fix[iafFixedLength+4*i:]); got != w {
			t.Errorf("G^ lists longword %d = %X, want %X", i, got, w)
		}
	}

	if got := le.Uint32(fix[0x1C:]); got != 3 {
		t.Errorf("shareable image count = %d, want 3", got)
	}

	// Each CALLS's operand is FF and a displacement to its cell: the 1st
	// call reaches B$ONE's cell, and the 2nd and 4th share A$ONE's.
	code := img.Bytes[blockSize:]
	cell := func(call int) uint32 {
		field := uint32(0x200 + 2 + 7*call + 3) // the displacement, after FB 00 FF
		if code[2+7*call+2] != 0xFF {
			t.Fatalf("call %d's operand mode is %02X, want FF", call, code[2+7*call+2])
		}

		return field + 4 + le.Uint32(code[2+7*call+3:])
	}

	cells := []uint32{0x400 + iafFixedLength + 8, 0x400 + iafFixedLength + 20, 0x400 + iafFixedLength + 24, 0x400 + iafFixedLength + 20}
	for i, w := range cells {
		if got := cell(i); got != w {
			t.Errorf("call %d reaches %X, want %X", i, got, w)
		}
	}

	// The global section ISDs follow the stack's, B first.
	p := isdOffset
	for n := 0; n < 3; n++ { // C, fixup, stack
		p += int(le.Uint16(img.Bytes[p:]))
	}

	for _, want := range []struct {
		name  string
		pages uint16
		flags uint32
		ident uint32
	}{
		{"BIMG_001", 3, isdGBL | uint32(MatchEqual)<<isdMatchShift | isdTypeSharedPIC, 2<<24 | 5},
		{"AIMG_001", 0, isdGBL | uint32(MatchAlways)<<isdMatchShift | isdTypeSharedPIC, 0},
	} {
		isd := img.Bytes[p:]
		size := int(le.Uint16(isd))
		name := string(isd[21 : 21+isd[20]])

		if name != want.name || le.Uint16(isd[2:]) != want.pages || le.Uint32(isd[8:]) != want.flags || le.Uint32(isd[16:]) != want.ident {
			t.Errorf("global section ISD = %s, %d pages, flags %X, ident %X; want %+v", name, le.Uint16(isd[2:]), le.Uint32(isd[8:]), le.Uint32(isd[16:]), want)
		}

		p += size
	}

	if p >= blockSize-1 || le.Uint16(img.Bytes[p:]) != 0 {
		t.Error("the ISD list doesn't end after the global section ISDs")
	}
}

// TestLinkSharedNeedsGeneralMode checks that a reference to a shareable
// image routine other than a general mode operand is an error.
func TestLinkSharedNeedsGeneralMode(t *testing.T) {
	m := macroModule(t, ".PSECT C,NOWRT,EXE\n.ENTRY GO,^M<>\nCALLS #0,L^LIB$PUT_OUTPUT\nRET\n.END GO")

	_, err := Link([]Input{{File: "m.obj", Module: m}}, Options{ImageName: "T", Sources: []SymbolSource{librtl}})
	if err == nil || !strings.Contains(err.Error(), "general mode") {
		t.Errorf("error = %v, want one asking for general mode", err)
	}
}
