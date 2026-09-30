package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/link"
)

// linkMap links with a brief map and returns the map's text.
func linkMap(t *testing.T, c *Console, dir string, opts LinkOptions) string {
	t.Helper()

	opts.Map, opts.Brief, opts.MapFile = true, true, filepath.Join(dir, "link.map")

	if err := c.Link(opts); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	data, err := os.ReadFile(opts.MapFile)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

// TestLink_optionsFile links a program whose subroutine an options file
// names, with STACK=, IDENTIFICATION=, NAME=, and a SYMBOL= the subroutine
// uses. The image has the options' name, ident, and stack, and returns
// the symbol's value plus one.
func TestLink_optionsFile(t *testing.T) {
	c := newBootableConsole(t)
	dir := t.TempDir()

	writeHostFile(t, filepath.Join(dir, "main.mar"), "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n\tCALLS\t#0,SUB\n\tRET\n\t.END\tSTART\n")
	writeHostFile(t, filepath.Join(dir, "sub.mar"), "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSUB,^M<>\n\tMOVL\t#LIMIT,R0\n\tINCL\tR0\n\tRET\n\t.END\n")
	writeHostFile(t, filepath.Join(dir, "prog.opt"), "! The program's options\n"+
		"sub\t\t! the subroutine\nSTACK=30\nIDENTIFICATION=\"V9\"\n"+
		"NAME=PROGRAM\nSYMBOL=LIMIT,%X29\n")

	for _, name := range []string{"main", "sub"} {
		if err := c.Macro(MacroOptions{Source: filepath.Join(dir, name+".mar")}); err != nil {
			t.Fatal(err)
		}
	}

	text := linkMap(t, c, dir, LinkOptions{Files: []link.InputFile{
		{Name: filepath.Join(dir, "main")},
		{Name: "prog", Options: true},
	}})

	for _, want := range []string{
		"Stack size:                                             30. pages",
		"Image name and identification:                    PROGRAM V9",
		" sub.obj ",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in the map:\n%s", want, text)
		}
	}

	if got := runImage(t, c, filepath.Join(dir, "main.exe")); got != 0x2A {
		t.Errorf("R0 = %d, want 42", got)
	}

	// An options file named first names the image.
	if err := c.Link(LinkOptions{Files: []link.InputFile{{Name: filepath.Join(dir, "prog"), Options: true}, {Name: "main"}}}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "prog.exe")); err != nil {
		t.Errorf("no prog.exe: %v", err)
	}
}

// TestLink_inputErrors checks what an input file can't be.
func TestLink_inputErrors(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	obj := assembleFixture(t, c, "entry", dir)

	writeHostFile(t, filepath.Join(dir, "bad.opt"), "entry\nCLUSTER=A,,,ENTRY\n")
	writeHostFile(t, filepath.Join(dir, "none.opt"), "STACK=10\n")

	cases := []struct {
		files []link.InputFile
		want  string
	}{
		{[]link.InputFile{{Name: obj}, {Name: "bad", Options: true}}, "line 2: option CLUSTER: isn't supported"},
		{[]link.InputFile{{Name: filepath.Join(dir, "none"), Options: true}}, "no object modules"},
		{[]link.InputFile{{Name: obj}, {Name: "x", Shareable: true}}, "are named in an options file"},
		{[]link.InputFile{{Name: obj}, {Name: "entry.obj", Library: true}}, "entry.obj"},
	}

	for _, c2 := range cases {
		err := c.Link(LinkOptions{Files: c2.files, NoExecutable: true})
		if err == nil || !strings.Contains(err.Error(), c2.want) {
			t.Errorf("%+v: err = %v, want %q", c2.files, err, c2.want)
		}
	}
}

// TestLink_userLibraries links with the VMS libraries named as the user's
// own: STARLET.OLB searched with /LIBRARY, one of its modules added with
// /INCLUDE, IMAGELIB.OLB searched with /LIBRARY, and LIBRTL.EXE named in
// an options file with /SHAREABLE. Each image is the one govax's own
// tables give.
func TestLink_userLibraries(t *testing.T) {
	vmsLibFile(t, "starlet.olb")
	vmsLibFile(t, "imagelib.olb")
	vmsLibFile(t, "librtl.exe")

	starlet, _ := filepath.Abs(filepath.Join(vmsLibDir, "starlet.olb"))
	imagelib, _ := filepath.Abs(filepath.Join(vmsLibDir, "imagelib.olb"))
	librtl, _ := filepath.Abs(filepath.Join(vmsLibDir, "librtl.exe"))

	c := newBootableConsole(t)
	dir := t.TempDir()

	writeHostFile(t, filepath.Join(dir, "exit.mar"), "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n\tPUSHL\t#1\n\tCALLS\t#1,G^SYS$EXIT\n\tRET\n\t.END\tSTART\n")

	if err := c.Macro(MacroOptions{Source: filepath.Join(dir, "exit.mar")}); err != nil {
		t.Fatal(err)
	}

	exit := filepath.Join(dir, "exit")
	exe := filepath.Join(dir, "exit.exe")

	image := func(opts LinkOptions) []byte {
		t.Helper()

		if err := c.Link(opts); err != nil {
			t.Fatalf("LINK: %v", err)
		}

		data, err := os.ReadFile(exe)
		if err != nil {
			t.Fatal(err)
		}

		return data
	}

	own := image(LinkOptions{Objects: []string{exit}, NoSysLib: true})

	if lib := image(LinkOptions{Files: []link.InputFile{{Name: exit}, {Name: starlet, Library: true}}, NoSysLib: true}); !sameImage(own, lib) {
		t.Error("the image linked with STARLET/LIBRARY differs from govax's tables'")
	}

	text := linkMap(t, c, dir, LinkOptions{Files: []link.InputFile{{Name: exit}, {Name: starlet, Include: []string{"SYS$SSDEF"}}}, NoSysLib: true})
	if !strings.Contains(text, "\nSYS$SSDEF ") {
		t.Errorf("the included module isn't in the map:\n%s", text)
	}

	hello := linkHello(t, c, LinkOptions{NoSysLib: true})

	c.LinkLibrary = vmsLibDir
	withImagelib := linkHello(t, c, LinkOptions{Files: []link.InputFile{{Name: imagelib, Library: true}}, NoSysLib: true})
	c.LinkLibrary = ""

	if !sameImage(hello, withImagelib) {
		t.Error("the image linked with IMAGELIB/LIBRARY differs from govax's tables'")
	}

	opt := filepath.Join(dir, "rtl.opt")
	writeHostFile(t, opt, `"`+librtl+`"/SHAREABLE`+"\n") // a host path is quoted, for its slashes

	if withImage := linkHello(t, c, LinkOptions{Files: []link.InputFile{{Name: opt, Options: true}}, NoSysLib: true}); !sameImage(hello, withImage) {
		t.Error("the image linked with LIBRTL/SHAREABLE differs from govax's tables'")
	}
}

// TestDispatch_linkPositionalQualifiers runs LINK through DCL with an
// options file named by a positional /OPTIONS.
func TestDispatch_linkPositionalQualifiers(t *testing.T) {
	d, c := newTestDispatcher(t)
	dir := t.TempDir()
	entry := assembleFixture(t, c, "entry", dir)
	assembleFixture(t, c, "data", dir)

	writeHostFile(t, filepath.Join(dir, "more.opt"), "data\nIDENT=V5\n")

	mapFile := filepath.Join(dir, "entry.map")
	if err := d.Dispatch(`LINK "` + entry + `","more"/OPTIONS/MAP="` + mapFile + `"/BRIEF`); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	data, err := os.ReadFile(mapFile)
	if err != nil {
		t.Fatal(err)
	}

	if text := string(data); !strings.Contains(text, "\nDATA ") || !strings.Contains(text, "ENTRY V5\n") {
		t.Errorf("map:\n%s", text)
	}
}
