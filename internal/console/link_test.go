package console

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// marFixture is testdata/mar/name.mar.
func marFixture(name string) string {
	return filepath.Join("..", "..", "testdata", "mar", name+".mar")
}

// assembleFixture assembles a testdata/mar fixture with MACRO into dir,
// returning the object's path.
func assembleFixture(t *testing.T, c *Console, name, dir string) string {
	t.Helper()

	objPath := filepath.Join(dir, name+".obj")
	if err := c.Macro(MacroOptions{Source: marFixture(name), Object: objPath}); err != nil {
		t.Fatalf("MACRO %s: %v", name, err)
	}

	return objPath
}

// runImage runs an image in c, as RUN does but with a bound on the steps,
// and returns R0: the status its main routine returned.
func runImage(t *testing.T, c *Console, path string) uint32 {
	t.Helper()

	return runImageBounded(t, c, path, 100_000)
}

// runImageBounded is runImage, running at most maxSteps instructions.
func runImageBounded(t *testing.T, c *Console, path string, maxSteps int) uint32 {
	t.Helper()

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	runKernelInitialize(t, c)

	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	main, err := c.activateImage(path)
	if err != nil {
		t.Fatalf("activating %s: %v", path, err)
	}

	driver, ok, err := c.buildImageInitDriver(main, false)
	if err != nil || !ok {
		t.Fatalf("buildImageInitDriver: ok=%v err=%v", ok, err)
	}

	c.Engine.SetModeStack(vax.User, false)

	if runErr, hitCap := callBounded(t, c, driver, maxSteps); runErr != nil || hitCap {
		t.Fatalf("running %s: err=%v hitCap=%v", path, runErr, hitCap)
	}

	return c.CPU.GPR(vax.R0)
}

// TestLink_macroLinkRun assembles, links, and runs the self-contained
// fixtures: each main routine returns 1.
func TestLink_macroLinkRun(t *testing.T) {
	for _, name := range []string{"psects", "entry"} {
		t.Run(name, func(t *testing.T) {
			c := newBootableConsole(t)
			dir := t.TempDir()
			assembleFixture(t, c, name, dir)

			// No type: the object is .OBJ, and the image .EXE beside it.
			if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, name)}}); err != nil {
				t.Fatalf("LINK: %v", err)
			}

			exe := filepath.Join(dir, name+".exe")

			info, err := os.Stat(exe)
			if err != nil {
				t.Fatal(err)
			}

			if info.Size()%512 != 0 {
				t.Errorf("image is %d bytes, not whole blocks", info.Size())
			}

			if got := runImage(t, c, exe); got != 1 {
				t.Errorf("R0 = %d, want 1", got)
			}
		})
	}
}

// TestLink_runsRelocatedCode links and runs a program whose status comes
// from a data psect through a relocated reference, and whose code is in
// a second module.
func TestLink_runsRelocatedCode(t *testing.T) {
	c := newBootableConsole(t)
	dir := t.TempDir()

	writeHostFile(t, filepath.Join(dir, "main.mar"), "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n\tCALLS\t#0,GETVAL\n\tRET\n\t.END\tSTART\n")
	writeHostFile(t, filepath.Join(dir, "sub.mar"), "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"VALUE:\t.LONG\t42\n\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tGETVAL,^M<>\n\tMOVL\tVALUE,R0\n\tRET\n\t.END\n")

	for _, name := range []string{"main", "sub"} {
		if err := c.Macro(MacroOptions{Source: filepath.Join(dir, name+".mar")}); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "main"), "sub"}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if got := runImage(t, c, filepath.Join(dir, "main.exe")); got != 42 {
		t.Errorf("R0 = %d, want 42", got)
	}
}

// TestLink_hello assembles, links, and runs hello: its general mode call
// to LIB$PUT_OUTPUT resolves, through govax's own symbol source, to
// LIBRTL's routine, which RUN reaches through the shim for its offset.
func TestLink_hello(t *testing.T) {
	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	dir := t.TempDir()
	assembleFixture(t, c, "hello", dir)

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "hello")}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if got := runImage(t, c, filepath.Join(dir, "hello.exe")); got != 1 {
		t.Errorf("R0 = %d, want 1", got)
	}

	if !strings.Contains(out.String(), "Hello, world!") {
		t.Errorf("output = %q, want Hello, world!", out.String())
	}
}

// TestLink_helloPageProtection checks that RUN gives each section of a
// linked image its own page protection: hello.mar's writable DATA psect
// UW, its read-only CODE psect UR (so not even kernel mode may write it).
// A second activation must be able to load over the read-only pages.
func TestLink_helloPageProtection(t *testing.T) {
	c := newBootableConsole(t)
	dir := t.TempDir()
	assembleFixture(t, c, "hello", dir)

	exe := filepath.Join(dir, "hello.exe")
	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "hello")}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	runImage(t, c, exe)

	check := func() {
		t.Helper()

		var sawWRT, sawRO bool

		main := c.findMainICB()
		for _, isd := range main.ISDList {
			if isdType(isd.Flags) == isdUsrStack || isd.Flags&isdGBL != 0 {
				continue
			}

			want := vm.ProtUR
			if isd.Flags&isdWRT != 0 {
				want, sawWRT = vm.ProtUW, true
			} else {
				sawRO = true
			}

			for n := uint32(0); n < uint32(isd.Pages); n++ {
				addr := main.Base + (uint32(isd.VPN)+n)<<9

				_, _, pte, err := c.Mem.LookupPTE(c.CPU, addr)
				if err != nil {
					t.Fatalf("LookupPTE(%#x): %v", addr, err)
				}

				if got := pte.Protection(); got != want {
					t.Errorf("page %#x (ISD flags %#x): PROT = %s, want %s", addr, isd.Flags, got, want)
				}
			}
		}

		if !sawWRT || !sawRO {
			t.Errorf("hello.exe: writable section %v, read-only section %v; want both", sawWRT, sawRO)
		}
	}

	check()

	if _, _, pte, _ := c.Mem.LookupPTE(c.CPU, 0x400); pte.Protection().Allows(vax.Kernel, vm.AccessWrite) {
		t.Errorf("code page 0x400 (PROT %s) is writable", pte.Protection())
	}

	c.resetICBList()

	if _, err := c.activateImage(exe); err != nil {
		t.Fatalf("second activation: %v", err)
	}

	check()
}

// TestLink_systemService links a general mode call to a system service,
// which resolves to its address in the P1 vector: absolute mode.
func TestLink_systemService(t *testing.T) {
	c := newBootableConsole(t)
	dir := t.TempDir()

	writeHostFile(t, filepath.Join(dir, "svc.mar"), "\t.PSECT\tC,NOWRT,EXE\n\t.ENTRY\tGO,^M<>\n"+
		"\tPUSHL\t#9\n\tCALLS\t#1,G^SYS$EXIT\n\tRET\n\t.END\tGO\n")

	if err := c.Macro(MacroOptions{Source: filepath.Join(dir, "svc.mar")}); err != nil {
		t.Fatal(err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "svc")}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "svc.exe"))
	if err != nil {
		t.Fatal(err)
	}

	exit, _ := vmsdefP1Address("SYS$EXIT")

	// The code: entry mask, PUSHL S^#9, CALLS S^#1, then absolute mode.
	code := data[512:]
	want := []byte{0, 0, 0xDD, 0x09, 0xFB, 0x01, 0x9F, byte(exit), byte(exit >> 8), byte(exit >> 16), byte(exit >> 24)}

	if !bytes.Equal(code[:len(want)], want) {
		t.Errorf("code = % x, want % x", code[:len(want)], want)
	}
}

// TestLink_imageHeaderNames checks the name and linker ID in the header.
func TestLink_imageHeaderNames(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	obj := assembleFixture(t, c, "entry", dir)

	saved := BuildVersion
	BuildVersion = "1.0-118"

	defer func() { BuildVersion = saved }()

	exe := filepath.Join(dir, "My_Prog.exe")
	if err := c.Link(LinkOptions{Objects: []string{obj}, Executable: exe}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}

	ihi := data[0x60:]
	if name := string(ihi[1 : 1+ihi[0]]); name != "MY_PROG" {
		t.Errorf("image name = %q, want MY_PROG", name)
	}

	if id := string(ihi[41 : 41+ihi[40]]); id != "V1.0" {
		t.Errorf("image ID = %q, want the module's V1.0", id)
	}

	if id := string(ihi[65 : 65+ihi[64]]); id != "govax V1.0-118" {
		t.Errorf("linker ID = %q", id)
	}
}

// TestLink_severalObjects links two modules: one with code and the
// transfer address, one with only data.
func TestLink_severalObjects(t *testing.T) {
	c := newBootableConsole(t)
	dir := t.TempDir()
	entry := assembleFixture(t, c, "entry", dir)
	assembleFixture(t, c, "data", dir)

	// A later object's bare name is found beside the one before it.
	if err := c.Link(LinkOptions{Objects: []string{entry, "data"}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if got := runImage(t, c, filepath.Join(dir, "entry.exe")); got != 1 {
		t.Errorf("R0 = %d, want 1", got)
	}
}

func TestLink_errors(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()

	// A symbol nothing defines is a warning, not an error, as in real LINK
	// (TestLink_warnings).

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "none.obj")}}); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("missing object: error = %v", err)
	}

	bad := filepath.Join(dir, "bad.obj")
	if err := os.WriteFile(bad, []byte{4, 0, 9, 9, 9, 9}, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Link(LinkOptions{Objects: []string{bad}}); !errors.Is(err, vmserrors.New(vmserrors.CLI_LINKING)) {
		t.Errorf("not an object: error = %v", err)
	}

	if err := c.Link(LinkOptions{}); err == nil {
		t.Error("LINK with no objects succeeded")
	}

	// /NOEXECUTABLE writes nothing.
	entry := assembleFixture(t, c, "entry", dir)
	if err := c.Link(LinkOptions{Objects: []string{entry}, NoExecutable: true}); err != nil {
		t.Fatal(err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "entry.exe")); statErr == nil {
		t.Error("/NOEXECUTABLE wrote an image")
	}
}

// TestLink_noTraceback checks /NOTRACEBACK: the user transfer address is
// the first.
func TestLink_noTraceback(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	obj := assembleFixture(t, c, "entry", dir)

	for _, tb := range []bool{false, true} {
		if err := c.Link(LinkOptions{Objects: []string{obj}, NoTraceback: !tb}); err != nil {
			t.Fatal(err)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "entry.exe"))
		first := uint32(data[0x30]) | uint32(data[0x31])<<8 | uint32(data[0x32])<<16 | uint32(data[0x33])<<24

		want := uint32(0x200)
		if tb {
			want = 0x7FFEDF68
		}

		if first != want {
			t.Errorf("traceback %v: first transfer address = %08X, want %08X", tb, first, want)
		}
	}
}

// TestLink_volume links objects on a mounted volume into an image there,
// with real LINK's file attributes, and reads it back.
func TestLink_volume(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.Macro(MacroOptions{Source: marFixture("psects"), Object: "DUA0:[000000]PSECTS.OBJ"}); err != nil {
		t.Fatal(err)
	}

	if err := c.Link(LinkOptions{Objects: []string{"DUA0:[000000]PSECTS"}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	blocks, found, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]PSECTS.EXE"}, rms.ImageBlocks)
	if err != nil {
		t.Fatal(err)
	}

	// Five blocks, as real LINK's PSECTS.EXE: the header, the image
	// sections, the fixup section, and the debug symbol table.
	if found.Name != "DUA0:[000000]PSECTS.EXE;1" || len(blocks) != 5 {
		t.Errorf("image %s has %d blocks, want 5", found.Name, len(blocks))
	}

	// To the host, then compare.
	exe := filepath.Join(t.TempDir(), "psects.exe")
	if err := c.Link(LinkOptions{Objects: []string{"DUA0:[000000]PSECTS"}, Executable: exe}); err != nil {
		t.Fatal(err)
	}

	host, _ := os.ReadFile(exe)
	if !bytes.Equal(host[0x100:], bytes.Join(blocks, nil)[0x100:]) {
		t.Error("the volume image and the host image differ past the header")
	}
}

// TestDispatch_linkViaDCL runs LINK through the DCL grammar: a list of
// quoted objects, /EXECUTABLE=, and /NOTRACEBACK.
func TestDispatch_linkViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)
	dir := t.TempDir()
	entry := assembleFixture(t, c, "entry", dir)
	data := assembleFixture(t, c, "data", dir)
	exe := filepath.Join(dir, "both.exe")

	if err := d.Dispatch(`LINK "` + entry + `","` + data + `"/EXECUTABLE="` + exe + `"/NOTRACEBACK`); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if _, err := os.Stat(exe); err != nil {
		t.Errorf("no image: %v", err)
	}

	if err := d.Dispatch(`LINK "` + entry + `"/NOEXECUTABLE`); err != nil {
		t.Fatalf("LINK/NOEXECUTABLE: %v", err)
	}

	mapFile := filepath.Join(dir, "both.lis")
	if err := d.Dispatch(`LINK "` + entry + `"/NOEXECUTABLE/MAP="` + mapFile + `"/BRIEF`); err != nil {
		t.Fatalf("LINK/MAP=: %v", err)
	}

	if data, err := os.ReadFile(mapFile); err != nil || !strings.Contains(string(data), "BRIEF in file") {
		t.Errorf("map: %v\n%s", err, data)
	}

	if err := d.Dispatch(`LINK "` + entry + `"/NOEXECUTABLE/MAP`); err != nil {
		t.Fatalf("LINK/MAP: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "entry.map")); err != nil {
		t.Errorf("no default map: %v", err)
	}

	// Without /MAP there's no map, though the grammar gives /MAP a
	// default value.
	if err := os.Remove(filepath.Join(dir, "entry.map")); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch(`LINK "` + entry + `"`); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "entry.map")); err == nil {
		t.Error("LINK without /MAP wrote a map")
	}
}

// TestLink_map writes link maps: by default beside the first object with
// the type MAP, naming the image's file; to a file /MAP= names; brief; and
// on a mounted volume, as a text file naming itself and the image with
// its version.
func TestLink_map(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	obj := assembleFixture(t, c, "psects", dir)

	if err := c.Link(LinkOptions{Objects: []string{obj}, Map: true}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "psects.map"))
	if err != nil {
		t.Fatal(err)
	}

	// A page heading's image name is cut to its 64-character field.
	exe := filepath.Join(dir, "psects.exe")
	text := string(data)

	for _, want := range []string{
		"\f\n" + exe[:min(len(exe), 64)],
		"! Program Section Synopsis !",
		"FIRST           00000600-R",
		"Map format:                                       DEFAULT in file " + filepath.Join(dir, "psects.map") + "\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in the map:\n%s", want, text)
		}
	}

	brief := filepath.Join(dir, "brief.lis")
	if err := c.Link(LinkOptions{Objects: []string{obj}, NoExecutable: true, Map: true, MapFile: brief, Brief: true}); err != nil {
		t.Fatal(err)
	}

	data, err = os.ReadFile(brief)
	if err != nil {
		t.Fatal(err)
	}

	if text := string(data); strings.Contains(text, "Symbols By Name") || !strings.Contains(text, "No image file created") {
		t.Errorf("brief map, no image:\n%s", text)
	}

	mountFreshContainer(t, c, "DUA0")

	if err := c.Macro(MacroOptions{Source: marFixture("psects"), Object: "DUA0:[000000]PSECTS.OBJ"}); err != nil {
		t.Fatal(err)
	}

	if err := c.Link(LinkOptions{Objects: []string{"DUA0:[000000]PSECTS"}, Map: true}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	records, found, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]PSECTS.MAP"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	text = string(bytes.Join(records, []byte("\n")))
	if found.Name != "DUA0:[000000]PSECTS.MAP;1" ||
		!strings.Contains(text, "\nDUA0:[000000]PSECTS.EXE;1 ") ||
		!strings.Contains(text, "PSECTS          V1.0                   94 DUA0:[000000]PSECTS.OBJ;1") ||
		!strings.Contains(text, "in file DUA0:[000000]PSECTS.MAP\n") {
		t.Errorf("volume map %s:\n%s", found.Name, text)
	}
}

// vmsdefP1Address is a P1 vector entry's address.
func vmsdefP1Address(name string) (uint32, bool) {
	for _, e := range vmsdef.P1VectorTable {
		if e.Name == name {
			return e.Addr, true
		}
	}

	return 0, false
}
