package console

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// macroSource is a small module with code, data, and an external call.
const macroSource = "\t.TITLE\tSMALL\ta small module\n" +
	"\t.IDENT\t/V1.0/\n" +
	"\t.PSECT\tDATA,NOEXE,WRT\n" +
	"VALUE:\t.LONG\t1,2,3\n" +
	"\t.PSECT\tCODE,EXE,NOWRT\n" +
	"\t.ENTRY\tSTART,^M<>\n" +
	"\tMOVL\tVALUE,R0\n" +
	"\tCALLS\t#0,G^SYS$EXIT\n" +
	"\tRET\n" +
	"\t.END\tSTART\n"

func writeHostFile(t *testing.T, path, text string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readObject reads, decodes, and checks an object module.
func readObject(t *testing.T, c *Console, loc rms.FileLocation) *obj.Module {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(loc, rms.VariableRecords)
	if err != nil {
		t.Fatalf("reading %s: %v", loc.Name, err)
	}

	m, err := obj.Decode(records)
	if err != nil {
		t.Fatalf("decoding %s: %v", loc.Name, err)
	}

	if problems := obj.Check(m); len(problems) > 0 {
		t.Errorf("%s: Check found %v", loc.Name, problems)
	}

	return m
}

// dirNames is the names in dir, in their on-disk case.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}

	return names
}

func TestMatchCase(t *testing.T) {
	cases := []struct{ model, word, whole, want string }{
		{"mar", "OBJ", "hello.mar", "obj"},
		{"MAR", "obj", "HELLO.MAR", "OBJ"},
		{"Mar", "OBJ", "Hello.Mar", "Obj"},
		{"mAR", "obj", "x.mAR", "oBJ"},
		{"m", "OBJ", "x.m", "obj"},
		{"M", "obj", "x.M", "OBJ"},
		{"", "OBJ", "prog", "obj"},
		{"", "obj", "Prog", "OBJ"},
	}

	for _, c := range cases {
		if got := matchCase(c.model, c.word, c.whole); got != c.want {
			t.Errorf("matchCase(%q, %q, %q) = %q, want %q", c.model, c.word, c.whole, got, c.want)
		}
	}
}

// TestMacro_hostDefaultObjectName checks the default object name: the
// source's, next to it, with an OBJ extension in the case of the
// source's.
func TestMacro_hostDefaultObjectName(t *testing.T) {
	cases := []struct{ source, typed, want string }{
		{"hello.mar", "hello.mar", "hello.obj"},
		{"HELLO.MAR", "HELLO.MAR", "HELLO.OBJ"},
		{"Hello.Mar", "Hello.Mar", "Hello.Obj"},
		{"prog.mar", "prog", "prog.obj"}, // the default type
		{"PROG.MAR", "PROG", "PROG.OBJ"},
		{"script", "script", "script.obj"}, // exists as named
	}

	for _, tc := range cases {
		c, _ := newTestConsole(t)
		dir := t.TempDir()
		writeHostFile(t, filepath.Join(dir, tc.source), macroSource)

		if err := c.Macro(MacroOptions{Source: filepath.Join(dir, tc.typed)}); err != nil {
			t.Errorf("%s: Macro: %v", tc.typed, err)

			continue
		}

		names := dirNames(t, dir)
		if len(names) != 2 || !contains(names, tc.want) {
			t.Errorf("%s: directory holds %v, want %s beside the source", tc.typed, names, tc.want)

			continue
		}

		readObject(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, tc.want)})
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}

	return false
}

// TestMacro_headers checks the object's module name, language processor
// (with govax's build), and SRC header (the command line).
func TestMacro_headers(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "small.mar")
	writeHostFile(t, src, macroSource)

	saved := BuildVersion
	BuildVersion = "1.2-34"

	defer func() { BuildVersion = saved }()

	if err := c.Macro(MacroOptions{Source: src, CommandLine: `MACRO "small.mar"`}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	m := readObject(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, "small.obj")})

	var mhd *obj.MainHeader

	texts := map[obj.HeaderType]string{}

	for _, r := range m.Records {
		switch h := r.(type) {
		case *obj.MainHeader:
			mhd = h
		case *obj.TextHeader:
			texts[h.Type] = h.Text
		}
	}

	if mhd == nil || mhd.Name != "SMALL" || mhd.Version != "V1.0" {
		t.Errorf("main header = %+v, want SMALL V1.0", mhd)
	}

	want := []string{"govax MACRO V1.2-34", `MACRO "small.mar"`, "a small module"}
	for _, w := range want {
		found := false

		for _, text := range texts {
			found = found || text == w
		}

		if !found {
			t.Errorf("text headers %v don't include %q", texts, w)
		}
	}
}

// TestMacro_errorsWriteNothing checks that every error is reported and no
// object is written, so an existing object survives.
func TestMacro_errorsWriteNothing(t *testing.T) {
	c, buf := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.mar")
	objPath := filepath.Join(dir, "bad.obj")

	writeHostFile(t, src, "\t.PSECT\tCODE\n\tBOGUS\n\tRSB\n\tALSO_BOGUS\n")
	writeHostFile(t, objPath, "old object")

	err := c.Macro(MacroOptions{Source: src})
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_ASMERRORS)) {
		t.Fatalf("Macro error = %v, want CLI_ASMERRORS", err)
	}

	if !strings.Contains(err.Error(), "2 error(s)") {
		t.Errorf("Macro error = %q, want it to count 2 errors", err)
	}

	out := buf.String()
	if strings.Count(out, "%CLI-E-ASSEMBLING") != 2 || !strings.Contains(out, "line 2:") || !strings.Contains(out, "line 4:") {
		t.Errorf("output = %q, want both errors reported", out)
	}

	if data, _ := os.ReadFile(objPath); string(data) != "old object" {
		t.Errorf("the old object became %q", data)
	}
}

// TestMacro_warningsStillWrite checks that a warning is reported and the
// object is still written.
func TestMacro_warningsStillWrite(t *testing.T) {
	c, buf := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "warn.mar")
	writeHostFile(t, src, "\t.ENABLE\tTRUNCATION\n\t.PSECT\tCODE\n\tRSB\n")

	if err := c.Macro(MacroOptions{Source: src}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	if !strings.Contains(buf.String(), "%CLI-W-ASMWARNING") {
		t.Errorf("output = %q, want a warning", buf.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "warn.obj")); err != nil {
		t.Errorf("no object after a warning: %v", err)
	}
}

func TestMacro_noObject(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "small.mar")
	writeHostFile(t, src, macroSource)

	if err := c.Macro(MacroOptions{Source: src, NoObject: true}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("/NOOBJECT left %v", names)
	}
}

// TestMacro_namedObject checks /OBJECT=: a bare name goes next to the
// source, a host path goes where it says.
func TestMacro_namedObject(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "small.mar")
	writeHostFile(t, src, macroSource)

	if err := c.Macro(MacroOptions{Source: src, Object: "other.obj"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, "src", "other.obj")})

	elsewhere := filepath.Join(dir, "out.obj")
	if err := c.Macro(MacroOptions{Source: src, Object: elsewhere}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Host: true, Name: elsewhere})

	// A directory gets the default name.
	outDir := filepath.Join(dir, "objs")
	if err := os.Mkdir(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := c.Macro(MacroOptions{Source: src, Object: outDir}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Host: true, Name: filepath.Join(outDir, "small.obj")})

	err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, "no", "such", "x.obj")})
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_OBJWRITE)) {
		t.Errorf("object in a missing directory: error = %v, want CLI_OBJWRITE", err)
	}
}

func TestMacro_missingSource(t *testing.T) {
	c, _ := newTestConsole(t)

	err := c.Macro(MacroOptions{Source: filepath.Join(t.TempDir(), "none.mar")})
	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("Macro error = %v, want SS_NOSUCHFILE", err)
	}

	if err := c.Macro(MacroOptions{Source: "DUB0:[X]NONE.MAR"}); !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("Macro error = %v, want SS_DEVNOTMOUNT", err)
	}
}

// TestMacro_volume assembles a source on a mounted volume: the object
// goes beside it, each assembly makes the next version, and the source's
// type defaults to MAR.
func TestMacro_volume(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	s := c.ContainerSession
	if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]SMALL.MAR"}, rms.TextRecords, splitSource(macroSource)); err != nil {
		t.Fatal(err)
	}

	if err := c.Macro(MacroOptions{Source: "DUA0:[000000]SMALL"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Name: "DUA0:[000000]SMALL.OBJ;1"})

	// A bare name, with the default on the volume.
	if err := c.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	if err := c.Macro(MacroOptions{Source: "SMALL.MAR"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Name: "DUA0:[000000]SMALL.OBJ;2"})

	// A directory alone gets the default name.
	if err := c.Macro(MacroOptions{Source: "SMALL.MAR", Object: "DUA0:[000000]"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Name: "DUA0:[000000]SMALL.OBJ;3"})

	// A name without a type gets OBJ, as on VMS.
	if err := c.Macro(MacroOptions{Source: "SMALL.MAR", Object: "OTHER"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Name: "DUA0:[000000]OTHER.OBJ;1"})

	// To the host.
	hostObj := filepath.Join(t.TempDir(), "small.obj")
	if err := c.Macro(MacroOptions{Source: "SMALL.MAR", Object: hostObj}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Host: true, Name: hostObj})
}

// TestMacro_hostSourceVolumeObject writes a host source's object onto a
// volume, and checks a read-only volume is refused.
func TestMacro_hostSourceVolumeObject(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	src := filepath.Join(t.TempDir(), "small.mar")
	writeHostFile(t, src, macroSource)

	if err := c.Macro(MacroOptions{Source: src, Object: "DUA0:[000000]SMALL.OBJ"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	readObject(t, c, rms.FileLocation{Name: "DUA0:[000000]SMALL.OBJ"})

	path := filepath.Join(t.TempDir(), "ro.dsk")
	if err := c.InitializeContainer(path, 400, "ROVOL", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA1", path, false); err != nil {
		t.Fatal(err)
	}

	err := c.Macro(MacroOptions{Source: src, Object: "DUA1:[000000]SMALL.OBJ"})
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_OBJWRITE)) {
		t.Errorf("object on a read-only volume: error = %v, want CLI_OBJWRITE", err)
	}
}

// TestMacro_include resolves .INCLUDE relative to the source, across host
// and volume files.
func TestMacro_include(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	dir := t.TempDir()
	src := filepath.Join(dir, "main.mar")
	writeHostFile(t, src, "\t.PSECT\tDATA\n\t.INCLUDE \"host.inc\"\n\t.INCLUDE \"DUA0:[000000]VOL.INC\"\n\t.END\n")
	writeHostFile(t, filepath.Join(dir, "host.inc"), "HOSTSYM::\t.LONG\t1\n")

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]VOL.INC"}, rms.TextRecords,
		[][]byte{[]byte("VOLSYM::\t.LONG\t2")}); err != nil {
		t.Fatal(err)
	}

	if err := c.Macro(MacroOptions{Source: src}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	m := readObject(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, "main.obj")})
	names := make([]string, 0, len(m.Symbols()))

	for _, sym := range m.Symbols() {
		names = append(names, sym.Name)
	}

	if !contains(names, "HOSTSYM") || !contains(names, "VOLSYM") {
		t.Errorf("symbols = %v, want HOSTSYM and VOLSYM from the included files", names)
	}

	// A missing include is an error, and nothing is written.
	writeHostFile(t, src, "\t.INCLUDE \"none.inc\"\n")

	_ = os.Remove(filepath.Join(dir, "main.obj"))

	if err := c.Macro(MacroOptions{Source: src}); err == nil {
		t.Error("Macro with a missing include succeeded")
	}

	if _, err := os.Stat(filepath.Join(dir, "main.obj")); err == nil {
		t.Error("an object was written despite the missing include")
	}
}

// TestMacro_fixtureLadder assembles every testdata/mar fixture with the
// MACRO command and checks each object reads back and passes Check.
func TestMacro_fixtureLadder(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mar", "*.mar"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}

	for _, src := range sources {
		c, _ := newTestConsole(t)
		objPath := filepath.Join(t.TempDir(), "out.obj")

		if err := c.Macro(MacroOptions{Source: src, Object: objPath}); err != nil {
			t.Errorf("%s: %v", src, err)

			continue
		}

		readObject(t, c, rms.FileLocation{Host: true, Name: objPath})
	}
}

// TestDispatch_macroViaDCL runs MACRO through the DCL grammar: a quoted
// host path with /HOST, /OBJECT with and without a value, and /NOOBJECT.
func TestDispatch_macroViaDCL(t *testing.T) {
	d, _ := newTestDispatcher(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "small.mar")
	writeHostFile(t, src, macroSource)

	if err := d.Dispatch(`MACRO "` + src + `"/HOST/NOOBJECT`); err != nil {
		t.Fatalf("MACRO/NOOBJECT: %v", err)
	}

	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("/NOOBJECT left %v", names)
	}

	if err := d.Dispatch(`MACRO "` + src + `" /OBJECT`); err != nil {
		t.Fatalf("MACRO/OBJECT: %v", err)
	}

	m := readObject(t, d.Console, rms.FileLocation{Host: true, Name: filepath.Join(dir, "small.obj")})

	found := false

	for _, r := range m.Records {
		if h, ok := r.(*obj.TextHeader); ok && h.Text == `MACRO "`+src+`" /OBJECT` {
			found = true
		}
	}

	if !found {
		t.Error("the SRC header isn't the command line")
	}

	if err := d.Dispatch(`MACRO "` + src + `"/OBJECT="named.obj"`); err != nil {
		t.Fatalf("MACRO/OBJECT=: %v", err)
	}

	readObject(t, d.Console, rms.FileLocation{Host: true, Name: filepath.Join(dir, "named.obj")})
}

// splitSource splits source text into lines.
func splitSource(text string) [][]byte {
	splits := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	out := make([][]byte, 0, len(splits))

	for _, l := range splits {
		out = append(out, []byte(l))
	}

	return out
}
