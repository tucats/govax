package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/rms"
)

const libMacros = `; Test macros.
	.MACRO	STORE A,B
	MOVL	A,B	; the move
	.ENDM	STORE

	.MACRO	CLEAR X
	CLRL	X
	.ENDM	CLEAR
`

// openLibrary reads a library file.
func openLibrary(t *testing.T, c *Console, loc rms.FileLocation) *lbr.Library {
	t.Helper()

	data, _, err := c.ContainerSession.ReadRawFile(loc)
	if err != nil {
		t.Fatalf("reading %s: %v", loc.Name, err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatalf("opening %s: %v", loc.Name, err)
	}

	return l
}

func moduleNames(l *lbr.Library) []string {
	var out []string
	for _, k := range l.Indexes[0].Keys {
		out = append(out, k.Name)
	}

	return out
}

func moduleText(t *testing.T, l *lbr.Library, name string) string {
	t.Helper()

	rfa, ok := l.Lookup(name)
	if !ok {
		t.Fatalf("no module %s", name)
	}

	m, err := l.Module(rfa)
	if err != nil {
		t.Fatal(err)
	}

	return string(bytes.Join(m.Records, []byte("\n")))
}

// TestLibrary_macroHost creates a macro library on the host, then inserts,
// replaces, deletes, extracts, and lists.
func TestLibrary_macroHost(t *testing.T) {
	c, out := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "macros.mar")
	writeHostFile(t, src, libMacros)

	lib := filepath.Join(dir, "mine")
	if err := c.Library(LibraryOptions{Library: lib, Create: true, Macro: true, Inputs: []string{src}, Log: true}); err != nil {
		t.Fatalf("LIBRARY/CREATE: %v", err)
	}

	libFile := rms.FileLocation{Name: lib + ".mlb", Host: true}

	l := openLibrary(t, c, libFile)
	if l.Type != lbr.TypeMacro || strings.Join(moduleNames(l), ",") != "CLEAR,STORE" {
		t.Fatalf("library: %s %v", l.Type, moduleNames(l))
	}

	// /SQUEEZE, the default, took the comment off.
	if got := moduleText(t, l, "STORE"); got != "\t.MACRO\tSTORE A,B\n\tMOVL\tA,B\n\t.ENDM\tSTORE" {
		t.Errorf("STORE is %q", got)
	}

	if !strings.Contains(out.String(), "module CLEAR inserted in "+libFile.Name) {
		t.Errorf("/LOG: %q", out.String())
	}

	// /INSERT doesn't replace a module already there.
	writeHostFile(t, src, strings.ReplaceAll(libMacros, "CLRL", "CLRQ"))
	out.Reset()

	if err := c.Library(LibraryOptions{Library: libFile.Name, Insert: true, Inputs: []string{src}}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "module CLEAR from "+src+" is already in the library") ||
		strings.Contains(moduleText(t, openLibrary(t, c, libFile), "CLEAR"), "CLRQ") {
		t.Errorf("/INSERT of a module already there: %q", out.String())
	}

	// With no operation named, the inputs replace, and /NOSQUEEZE keeps
	// the comment.
	out.Reset()

	if err := c.Library(LibraryOptions{Library: libFile.Name, Inputs: []string{src}, NoSqueeze: true, Log: true}); err != nil {
		t.Fatal(err)
	}

	l = openLibrary(t, c, libFile)
	if !strings.Contains(moduleText(t, l, "CLEAR"), "CLRQ") || !strings.Contains(moduleText(t, l, "STORE"), "; the move") ||
		!strings.Contains(out.String(), "module STORE replaced in") {
		t.Errorf("replace: %q", out.String())
	}

	// /EXTRACT, by wildcard, to the default output: the library's name
	// with type MAR.
	if err := c.Library(LibraryOptions{Library: libFile.Name, Extract: []string{"st*"}}); err != nil {
		t.Fatal(err)
	}

	if data, err := os.ReadFile(filepath.Join(dir, "mine.mar")); err != nil || !strings.HasPrefix(string(data), "\t.MACRO\tSTORE") {
		t.Errorf("extracted: %v, %q", err, data)
	}

	// /DELETE, then /LIST.
	out.Reset()

	if err := c.Library(LibraryOptions{Library: libFile.Name, Delete: []string{"STORE", "NONE%"}, List: true, Full: true}); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	for _, want := range []string{
		"no module matches NONE%",
		"Directory of MACRO library " + libFile.Name + " on ",
		"Creator:  govax Librarian",
		"Number of modules:      1                 Max. key length:  31",
		"Preallocated index blocks:      9",
		"CLEAR            inserted ",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("listing lacks %q:\n%s", want, text)
		}
	}

	if strings.Contains(text, "STORE") {
		t.Errorf("STORE is still listed:\n%s", text)
	}

	// The library feeds the assembler.
	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)

	ml, err := asm.NewMacroLibrary(openLibrary(t, c, libFile))
	if err != nil {
		t.Fatal(err)
	}

	a.SetMacroLibraries(ml)

	if _, err := a.Assemble("\t.PSECT CODE\n\tCLEAR R0\n\t.END\n"); err != nil {
		t.Errorf("assembling a call to CLEAR: %v", err)
	}
}

const (
	libMain = "\t.TITLE\tMAIN\n\t.PSECT\tCODE,NOWRT,EXE,LONG\n\t.ENTRY\tSTART,^M<>\n" +
		"\tCALLS\t#0,G^SUB_ONE\n\tRET\n\t.END\tSTART\n"
	libSub = "\t.TITLE\tSUB\n\t.IDENT\t/X-2/\n\t.PSECT\tCODE,NOWRT,EXE,LONG\n\t.ENTRY\tSUB_ONE,^M<>\n" +
		"\tMOVL\t#7,R0\n\tRET\n\t.END\n"
)

// TestLibrary_objectLink puts an object module in a library, lists it,
// and links a program that gets its subroutine from the library.
func TestLibrary_objectLink(t *testing.T) {
	c := newBootableConsole(t)
	out := c.Out.(*bytes.Buffer)
	dir := t.TempDir()

	for name, text := range map[string]string{"main": libMain, "sub": libSub} {
		writeHostFile(t, filepath.Join(dir, name+".mar"), text)

		if err := c.Macro(MacroOptions{Source: filepath.Join(dir, name+".mar")}); err != nil {
			t.Fatal(err)
		}
	}

	lib := filepath.Join(dir, "subs.olb")
	if err := c.Library(LibraryOptions{Library: lib, Create: true, Inputs: []string{filepath.Join(dir, "sub")}, Selective: true}); err != nil {
		t.Fatalf("LIBRARY/CREATE: %v", err)
	}

	out.Reset()

	if err := c.Library(LibraryOptions{Library: lib, List: true, Full: true, Names: true}); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"Directory of OBJECT library " + lib,
		"Module SUB              Ident X-2              Inserted ",
		" 1 symbol\n     Selectively searched\nSUB_ONE\n\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("listing lacks %q:\n%s", want, out.String())
		}
	}

	exe := filepath.Join(dir, "main.exe")
	out.Reset()

	err := c.Link(LinkOptions{
		Files:      []link.InputFile{{Name: filepath.Join(dir, "main")}, {Name: lib, Library: true}},
		Executable: exe,
	})
	if err != nil || strings.Contains(out.String(), "undefined") {
		t.Fatalf("LINK: %v\n%s", err, out.String())
	}

	if r0 := runImage(t, c, exe); r0 != 7 {
		t.Errorf("the program returned %d, want 7 from SUB_ONE", r0)
	}

	// A macro source can't go in an object library.
	if err := c.Library(LibraryOptions{Library: lib, Inputs: []string{filepath.Join(dir, "main.mar")}}); err == nil {
		t.Error("inserted a macro source in an object library")
	}

	if err := c.Library(LibraryOptions{Library: lib, Macro: true, List: true}); err == nil {
		t.Error("/MACRO on an object library was accepted")
	}
}

// TestLibrary_volume keeps a library on a mounted volume: a change writes
// a new version, and the listing can go to a file there.
func TestLibrary_volume(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	src := filepath.Join(t.TempDir(), "macros.mar")
	writeHostFile(t, src, libMacros)

	if err := c.Library(LibraryOptions{Library: "DUA0:[000000]MINE", Create: true, Macro: true, Inputs: []string{src}}); err != nil {
		t.Fatalf("LIBRARY/CREATE: %v", err)
	}

	if err := c.Library(LibraryOptions{Library: "DUA0:[000000]MINE.MLB", Delete: []string{"CLEAR"}, List: true, ListFile: "MINE"}); err != nil {
		t.Fatalf("LIBRARY/DELETE: %v", err)
	}

	l := openLibrary(t, c, rms.FileLocation{Name: "DUA0:[000000]MINE.MLB;2"})
	if strings.Join(moduleNames(l), ",") != "STORE" {
		t.Errorf("version 2 holds %v", moduleNames(l))
	}

	if l1 := openLibrary(t, c, rms.FileLocation{Name: "DUA0:[000000]MINE.MLB;1"}); len(moduleNames(l1)) != 2 {
		t.Errorf("version 1 holds %v", moduleNames(l1))
	}

	lines, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]MINE.LIS"}, rms.TextRecords)
	if err != nil || !strings.HasPrefix(string(lines[0]), "Directory of MACRO library DUA0:[000000]MINE.MLB;2 on ") {
		t.Errorf("listing file: %v, %q", err, lines)
	}
}

// TestLibrary_errorsLeaveLibrary checks that a failed LIBRARY doesn't
// change the library.
func TestLibrary_errorsLeaveLibrary(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "macros.mar")
	writeHostFile(t, src, libMacros)

	lib := filepath.Join(dir, "mine.mlb")
	if err := c.Library(LibraryOptions{Library: lib, Create: true, Macro: true, Inputs: []string{src}}); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(lib)

	bad := filepath.Join(dir, "bad.mar")
	writeHostFile(t, bad, "\t.MACRO\tNEW\n\tNOP\n")

	for _, opts := range []LibraryOptions{
		{Library: lib, Inputs: []string{src, bad}},                   // an unfinished macro
		{Library: lib, Inputs: []string{filepath.Join(dir, "none")}}, // no such file
		{Library: lib},               // nothing to do
		{Library: lib, Insert: true}, // /INSERT with no inputs
		{Library: filepath.Join(dir, "absent.mlb"), List: true}, // no library
	} {
		if err := c.Library(opts); err == nil {
			t.Errorf("%+v succeeded", opts)
		}
	}

	if after, _ := os.ReadFile(lib); !bytes.Equal(before, after) {
		t.Error("a failed LIBRARY changed the library")
	}
}

// TestDispatch_libraryViaDCL runs LIBRARY through the DCL grammar.
func TestDispatch_libraryViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "macros.mar")
	writeHostFile(t, src, libMacros)

	lib := filepath.Join(dir, "mine.mlb")

	for _, cmd := range []string{
		`LIBRARY/CREATE/MACRO "` + lib + `" "` + src + `"`,
		`LIBRARY/DELETE=(STORE) "` + lib + `"`,
		`LIBRARY "` + lib + `" "` + src + `"/NOSQUEEZE`,
		`LIBRARY/LIST="` + filepath.Join(dir, "mine.lis") + `"/FULL "` + lib + `"`,
	} {
		if err := d.Dispatch(cmd); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}

	l := openLibrary(t, c, rms.FileLocation{Name: lib, Host: true})
	if !strings.Contains(moduleText(t, l, "STORE"), "; the move") {
		t.Error("/NOSQUEEZE didn't reach the library")
	}

	if data, err := os.ReadFile(filepath.Join(dir, "mine.lis")); err != nil || !strings.Contains(string(data), "STORE            inserted") {
		t.Errorf("listing: %v\n%s", err, data)
	}

	if err := d.Dispatch(`LIBRARY/MACRO/OBJECT "` + lib + `"/LIST`); err == nil {
		t.Error("/MACRO/OBJECT was accepted")
	}
}
