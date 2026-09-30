package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
)

// vmsLibDir is testdata/vmslib, where files copied from a VAX are kept
// (only its README is in git).
var vmsLibDir = filepath.Join("..", "..", "testdata", "vmslib")

// vmsLibFile reads a file from testdata/vmslib, or skips the test.
func vmsLibFile(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(vmsLibDir, name))
	if err != nil {
		t.Skipf("no %s: %v", name, err)
	}

	return data
}

// TestShimOffsetsMatchLIBRTL checks each shim for a LIBRTL routine against
// LIBRTL.EXE's global symbol table: a real image calls the routine at that
// offset, and RUN connects the call to the shim registered for it.
func TestShimOffsetsMatchLIBRTL(t *testing.T) {
	src, _, err := link.ReadShareableImage(vmsLibFile(t, "librtl.exe"))
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range shimTable {
		if e.library != "LIBRTL" {
			continue
		}

		d, ok, _ := src.Lookup(e.name)
		if !ok || d.Value != e.offset {
			t.Errorf("%s's shim is at LIBRTL+^X%X; LIBRTL.EXE has %+v, %v", e.name, e.offset, d, ok)
		}
	}
}

// linkHello assembles and links hello with c's LINK, and returns the
// image.
func linkHello(t *testing.T, c *Console, opts LinkOptions) []byte {
	t.Helper()

	dir := t.TempDir()
	assembleFixture(t, c, "hello", dir)

	opts.Objects = []string{filepath.Join(dir, "hello")}
	if err := c.Link(opts); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "hello.exe"))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// sameImage reports whether two images of one program are the same apart
// from their header's link time and ident.
func sameImage(a, b []byte) bool {
	if len(a) != len(b) || len(a) < 512 {
		return false
	}

	a, b = bytes.Clone(a), bytes.Clone(b)

	for _, img := range [][]byte{a, b} {
		clear(img[0x24:0x28]) // IHD$L_IDENT: from the link time
		clear(img[0x98:0xA0]) // IHI$Q_LINKTIME
	}

	return bytes.Equal(a, b)
}

// TestLink_libraryDirectory links hello with IMAGELIB.OLB, STARLET.OLB,
// and LIBRTL.EXE in the host library directory. The image is the one
// govax's own tables give (/NOSYSLIB), and it runs.
func TestLink_libraryDirectory(t *testing.T) {
	vmsLibFile(t, "imagelib.olb")
	vmsLibFile(t, "starlet.olb")
	vmsLibFile(t, "librtl.exe")

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	c.LinkLibrary = vmsLibDir

	withLibs := linkHello(t, c, LinkOptions{})
	without := linkHello(t, c, LinkOptions{NoSysLib: true})

	if !sameImage(withLibs, without) {
		t.Error("the image linked from the VMS libraries differs from /NOSYSLIB's")
	}

	dir := t.TempDir()
	exe := filepath.Join(dir, "hello.exe")

	if err := os.WriteFile(exe, withLibs, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := runImage(t, c, exe); got != 1 || !strings.Contains(out.String(), "Hello, world!") {
		t.Errorf("R0 = %d, output %q", got, out.String())
	}
}

// TestLink_libraryMissingImage has IMAGELIB.OLB but no shareable images.
// LIB$PUT_OUTPUT's offset comes from its shim; a routine in an image with
// no file and no shim is an error naming the image.
func TestLink_libraryMissingImage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "IMAGELIB.OLB"), vmsLibFile(t, "imagelib.olb"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newBootableConsole(t)
	c.LinkLibrary = dir

	withLib := linkHello(t, c, LinkOptions{})
	if !sameImage(withLib, linkHello(t, c, LinkOptions{NoSysLib: true})) {
		t.Error("hello linked through the shim differs from /NOSYSLIB's")
	}

	writeHostFile(t, filepath.Join(dir, "smg.mar"), "\t.PSECT\tC,NOWRT,EXE\n\t.ENTRY\tGO,^M<>\n"+
		"\tCALLS\t#0,G^SMG$CREATE_PASTEBOARD\n\tRET\n\t.END\tGO\n")

	if err := c.Macro(MacroOptions{Source: filepath.Join(dir, "smg.mar")}); err != nil {
		t.Fatal(err)
	}

	err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "smg")}})
	if err == nil || !strings.Contains(err.Error(), "SMGSHR.EXE") {
		t.Errorf("err = %v, want one naming SMGSHR.EXE", err)
	}
}

// TestLink_libraryLogicalNames finds IMAGELIB.OLB and LIBRTL.EXE through
// SYS$LIBRARY and SYS$SHARE on a mounted volume, not the host directory.
func TestLink_libraryLogicalNames(t *testing.T) {
	imagelib := vmsLibFile(t, "imagelib.olb")
	librtl := vmsLibFile(t, "librtl.exe")

	c, _ := newTestConsole(t)

	path := filepath.Join(t.TempDir(), "lib.dsk")
	if err := c.InitializeContainer(path, 2000, "LIBVOL", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA1", path, true); err != nil {
		t.Fatal(err)
	}

	// A COPY/BINARY copy ends at the end-of-file byte; the volume file is
	// whole blocks.
	librtl = append(librtl, make([]byte, (512-len(librtl)%512)%512)...)

	for name, data := range map[string][]byte{"IMAGELIB.OLB": imagelib, "LIBRTL.EXE": librtl} {
		var blocks [][]byte
		for i := 0; i < len(data); i += 512 {
			blocks = append(blocks, data[i:i+512])
		}

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.ImageBlocks, blocks); err != nil {
			t.Fatal(err)
		}
	}

	// With the logical names undefined, and no host directory, IMAGELIB
	// isn't found: SMG$ routines are undefined.
	c.LinkLibrary = t.TempDir()

	src, err := c.linkSources(true)
	if err != nil || len(src) != 1 {
		t.Fatalf("sources %d, %v; want govax's alone", len(src), err)
	}

	for _, name := range []string{"SYS$LIBRARY", "SYS$SHARE"} {
		if err := c.DefineLogicalName("LNM$PROCESS", name, []string{"DUA1:[000000]"}, lnm.Supervisor, 0, false); err != nil {
			t.Fatal(err)
		}
	}

	src, err = c.linkSources(true)
	if err != nil || len(src) != 2 {
		t.Fatalf("sources %d, %v; want IMAGELIB's and govax's", len(src), err)
	}

	d, ok, err := src[0].Lookup("LIB$GET_INPUT")
	if err != nil || !ok || d.Image != "LIBRTL" {
		t.Errorf("LIB$GET_INPUT = %+v, %v, %v", d, ok, err)
	}

	if i, ok := src[0].Image("LIBRTL"); !ok || i.Pages != 264 {
		t.Errorf("LIBRTL: %+v, %v", i, ok)
	}
}
