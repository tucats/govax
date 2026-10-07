package console

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// whichMacro is a macro library source defining WHICH, which sets the
// global symbol WHICH to n, so a test can tell which library a call's
// definition came from.
func whichMacro(n string) string {
	return "\t.MACRO\tWHICH\nWHICH==" + n + "\n\t.ENDM\tWHICH\n"
}

// makeMacroLibrary makes the host macro library path from macro source.
func makeMacroLibrary(t *testing.T, c *Console, path, source string) {
	t.Helper()

	src := strings.TrimSuffix(path, filepath.Ext(path)) + "_src.mar"
	writeHostFile(t, src, source)

	if err := c.Library(LibraryOptions{Library: path, Create: true, Macro: true, Inputs: []string{src}}); err != nil {
		t.Fatalf("making %s: %v", path, err)
	}
}

// symbolValue is the value of the symbol name in the object at path.
func symbolValue(t *testing.T, c *Console, path, name string) (uint32, bool) {
	t.Helper()

	for _, sym := range readObject(t, c, rms.FileLocation{Host: true, Name: path}).Symbols() {
		if sym.Name == name {
			return sym.Value, true
		}
	}

	return 0, false
}

// hasSymbol reports whether the object at path names the symbol.
func hasSymbol(t *testing.T, c *Console, path, name string) bool {
	t.Helper()

	_, ok := symbolValue(t, c, path, name)

	return ok
}

// TestMacro_govaxStarlet assembles a $EXIT_S call with no VMS library to
// be found: the macro comes from govax's own STARLET.MLB.
func TestMacro_govaxStarlet(t *testing.T) {
	c, _ := newTestConsole(t)
	c.HostLibrary = t.TempDir()

	dir := t.TempDir()
	src := filepath.Join(dir, "exit.mar")
	writeHostFile(t, src, "\t.ENTRY\tSTART,^M<>\n\t$EXIT_S\tR0\n\t.END\tSTART\n")

	if err := c.Macro(MacroOptions{Source: src}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	if !hasSymbol(t, c, filepath.Join(dir, "exit.obj"), "SYS$EXIT") {
		t.Error("the object doesn't call SYS$EXIT")
	}
}

// TestMacro_librarySearchOrder checks MACRO's order: .LIBRARY's libraries
// (the last named first), then /LIBRARY='s (the last named first), then
// STARLET.MLB.
func TestMacro_librarySearchOrder(t *testing.T) {
	c, _ := newTestConsole(t)
	c.HostLibrary = t.TempDir()

	dir := t.TempDir()
	for _, n := range []string{"1", "2", "3", "4"} {
		makeMacroLibrary(t, c, filepath.Join(dir, "lib"+n+".mlb"), whichMacro(n))
	}

	// A library's own $EXIT_S replaces STARLET's.
	makeMacroLibrary(t, c, filepath.Join(dir, "exit.mlb"), "\t.MACRO\t$EXIT_S\tCODE\nMINE==1\n\t.ENDM\t$EXIT_S\n")

	src := filepath.Join(dir, "prog.mar")
	objPath := filepath.Join(dir, "prog.obj")

	cases := []struct {
		name, directives string
		libraries        []string
		want             uint32
	}{
		{"one /LIBRARY", "", []string{"lib1"}, 1},
		{"last /LIBRARY first", "", []string{"lib1", "lib2"}, 2},
		{".LIBRARY before /LIBRARY", "\t.LIBRARY\t/lib3/\n", []string{"lib1", "lib2"}, 3},
		{"last .LIBRARY first", "\t.LIBRARY\t/lib3/\n\t.LIBRARY\t/lib4.mlb/\n", []string{"lib1"}, 4},
	}

	for _, tc := range cases {
		writeHostFile(t, src, tc.directives+"\tWHICH\n\t.END\n")

		if err := c.Macro(MacroOptions{Source: src, Libraries: tc.libraries}); err != nil {
			t.Errorf("%s: Macro: %v", tc.name, err)

			continue
		}

		if v, ok := symbolValue(t, c, objPath, "WHICH"); !ok || v != tc.want {
			t.Errorf("%s: WHICH = %d (%v), want %d", tc.name, v, ok, tc.want)
		}
	}

	writeHostFile(t, src, "\t$EXIT_S\n\t.END\n")

	if err := c.Macro(MacroOptions{Source: src, Libraries: []string{"exit"}}); err != nil {
		t.Fatalf("$EXIT_S from exit.mlb: %v", err)
	}

	if !hasSymbol(t, c, objPath, "MINE") || hasSymbol(t, c, objPath, "SYS$EXIT") {
		t.Error("$EXIT_S didn't come from exit.mlb ahead of STARLET.MLB")
	}
}

// TestMacro_libraryErrors covers libraries that can't be used.
func TestMacro_libraryErrors(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.mar")
	writeHostFile(t, src, "\tNOP\n\t.END\n")

	err := c.Macro(MacroOptions{Source: src, Libraries: []string{"none"}})
	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("missing /LIBRARY file: %v, want SS_NOSUCHFILE", err)
	}

	// An object library isn't a macro library.
	objSrc := filepath.Join(dir, "sub.mar")
	writeHostFile(t, objSrc, "\t.ENTRY\tSUB,^M<>\n\tRET\n\t.END\n")

	if err := c.Macro(MacroOptions{Source: objSrc}); err != nil {
		t.Fatal(err)
	}

	if err := c.Library(LibraryOptions{Library: filepath.Join(dir, "objs.olb"), Create: true, Inputs: []string{filepath.Join(dir, "sub.obj")}}); err != nil {
		t.Fatal(err)
	}

	err = c.Macro(MacroOptions{Source: src, Libraries: []string{"objs.olb"}})
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_LIBRARY)) {
		t.Errorf("object library as /LIBRARY: %v, want CLI_LIBRARY", err)
	}

	// A .LIBRARY file that isn't there is an assembly error.
	writeHostFile(t, src, "\t.LIBRARY\t/none/\n\t.END\n")

	if err := c.Macro(MacroOptions{Source: src}); !errors.Is(err, vmserrors.New(vmserrors.CLI_ASMERRORS)) {
		t.Errorf("missing .LIBRARY file: %v, want CLI_ASMERRORS", err)
	}
}

// TestMacro_starletReadWhenNeeded puts a STARLET.MLB that isn't a library
// in the host library directory: a program with no library macros never
// reads it, and one with a library macro reports it.
func TestMacro_starletReadWhenNeeded(t *testing.T) {
	c, _ := newTestConsole(t)
	c.HostLibrary = t.TempDir()
	writeHostFile(t, filepath.Join(c.HostLibrary, "STARLET.MLB"), "not a library")

	dir := t.TempDir()
	src := filepath.Join(dir, "prog.mar")
	writeHostFile(t, src, "\tNOP\n\t.END\n")

	if err := c.Macro(MacroOptions{Source: src}); err != nil {
		t.Fatalf("a program without library macros: %v", err)
	}

	writeHostFile(t, src, "\t$EXIT_S\n\t.END\n")

	var out strings.Builder
	c.Out = &out

	if err := c.Macro(MacroOptions{Source: src}); !errors.Is(err, vmserrors.New(vmserrors.CLI_ASMERRORS)) {
		t.Fatalf("$EXIT_S with a bad STARLET.MLB: %v, want CLI_ASMERRORS", err)
	}

	if !strings.Contains(out.String(), "STARLET.MLB") {
		t.Errorf("the error doesn't name STARLET.MLB:\n%s", out.String())
	}
}

// TestMacro_starletSources finds STARLET.MLB in the host library
// directory, then through SYS$LIBRARY on a mounted volume, which comes
// first.
func TestMacro_starletSources(t *testing.T) {
	c, _ := newTestConsole(t)
	c.HostLibrary = t.TempDir()
	makeMacroLibrary(t, c, filepath.Join(c.HostLibrary, "starlet.mlb"), whichMacro("1"))

	dir := t.TempDir()
	src := filepath.Join(dir, "prog.mar")
	objPath := filepath.Join(dir, "prog.obj")
	
	writeHostFile(t, src, "\tWHICH\n\t.END\n")

	if err := c.Macro(MacroOptions{Source: src}); err != nil {
		t.Fatalf("host STARLET.MLB: %v", err)
	}

	if v, _ := symbolValue(t, c, objPath, "WHICH"); v != 1 {
		t.Errorf("WHICH = %d, want 1 from the host directory's STARLET.MLB", v)
	}

	mountFreshContainer(t, c, "DUA0")
	makeMacroLibrary(t, c, filepath.Join(dir, "vol.mlb"), whichMacro("2"))

	data, err := os.ReadFile(filepath.Join(dir, "vol.mlb"))
	if err != nil {
		t.Fatal(err)
	}

	var blocks [][]byte
	for i := 0; i < len(data); i += 512 {
		blocks = append(blocks, data[i:i+512])
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]STARLET.MLB"}, rms.ImageBlocks, blocks); err != nil {
		t.Fatal(err)
	}

	if err := c.DefineLogicalName("LNM$PROCESS", "SYS$LIBRARY", []string{"DUA0:[000000]"}, lnm.Supervisor, 0, false); err != nil {
		t.Fatal(err)
	}

	if err := c.Macro(MacroOptions{Source: src}); err != nil {
		t.Fatalf("SYS$LIBRARY:STARLET.MLB: %v", err)
	}

	if v, _ := symbolValue(t, c, objPath, "WHICH"); v != 2 {
		t.Errorf("WHICH = %d, want 2 from SYS$LIBRARY:STARLET.MLB", v)
	}
}

// TestMacro_realStarlet assembles system macro calls from VMS's own
// STARLET.MLB in the host library directory.
func TestMacro_realStarlet(t *testing.T) {
	vmsLibFile(t, "starlet.mlb")

	c, out := newTestConsole(t)
	c.HostLibrary = vmsLibDir
	defer func() {
		if t.Failed() {
			t.Log(out.String())
		}
	}()

	dir := t.TempDir()
	src := filepath.Join(dir, "prog.mar")
	writeHostFile(t, src, "\t$IODEF\n\t.ENTRY\tSTART,^M<>\n"+
		"\t$QIOW_S\tCHAN=#1,FUNC=#IO$_WRITEVBLK,P1=MSG,P2=#3\n"+
		"\t$EXIT_S\tR0\nMSG:\t.ASCII\t/HI!/\n\t.END\tSTART\n")

	if err := c.Macro(MacroOptions{Source: src}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	for _, name := range []string{"SYS$QIOW", "SYS$EXIT"} {
		if !hasSymbol(t, c, filepath.Join(dir, "prog.obj"), name) {
			t.Errorf("the object doesn't call %s", name)
		}
	}
}

// TestHostLibraryDir checks where the host library directory comes from:
// the console's own, then vax.library, then vax.link.library.
func TestHostLibraryDir(t *testing.T) {
	for _, key := range []string{librarySetting, legacyLibrarySetting} {
		old, had := settings.Get(key), settings.Exists(key)

		t.Cleanup(func() {
			if had {
				settings.Set(key, old)
			} else {
				_ = settings.Delete(key)
			}
		})
	}

	c, _ := newTestConsole(t)

	settings.Set(legacyLibrarySetting, "/old")
	_ = settings.Delete(librarySetting)

	if got := c.hostLibraryDir(); got != "/old" {
		t.Errorf("with only vax.link.library: %q", got)
	}

	settings.Set(librarySetting, "/new")

	if got := c.hostLibraryDir(); got != "/new" {
		t.Errorf("with vax.library too: %q", got)
	}

	c.HostLibrary = "/mine"

	if got := c.hostLibraryDir(); got != "/mine" {
		t.Errorf("with the console's own: %q", got)
	}
}

// TestDispatch_macroLibraryQualifier runs MACRO/LIBRARY= through the DCL
// grammar.
func TestDispatch_macroLibraryQualifier(t *testing.T) {
	d, _ := newTestDispatcher(t)
	d.Console.HostLibrary = t.TempDir()

	dir := t.TempDir()
	makeMacroLibrary(t, d.Console, filepath.Join(dir, "a.mlb"), whichMacro("1"))
	makeMacroLibrary(t, d.Console, filepath.Join(dir, "b.mlb"), whichMacro("2"))

	src := filepath.Join(dir, "prog.mar")
	writeHostFile(t, src, "\tWHICH\n\t.END\n")

	for _, tc := range []struct {
		qualifier string
		want      uint32
	}{
		{`/LIBRARY="a"`, 1},
		{`/LIBRARY=("a","b")`, 2},
		{`/LIBRARY=("b","a")`, 1},
	} {
		if err := d.Dispatch(`MACRO "` + src + `"` + tc.qualifier); err != nil {
			t.Errorf("%s: %v", tc.qualifier, err)

			continue
		}

		if v, _ := symbolValue(t, d.Console, filepath.Join(dir, "prog.obj"), "WHICH"); v != tc.want {
			t.Errorf("%s: WHICH = %d, want %d", tc.qualifier, v, tc.want)
		}
	}

	if err := d.Dispatch(`MACRO "` + src + `"/LIBRARY="none"`); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("a missing library: %v", err)
	}
}
