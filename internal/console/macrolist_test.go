package console

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// These test docs/PHASE-29.md subtask 5: MACRO's /LIST qualifier. The
// listing's layout is internal/asm's (and its tests'); these check where
// the console writes it and what it hands the assembler for the heading.

// listingLines reads the listing at loc as its lines.
func listingLines(t *testing.T, c *Console, loc rms.FileLocation) []string {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		t.Fatalf("reading the listing %s: %v", loc.Name, err)
	}

	lines := make([]string, len(records))
	for i, r := range records {
		lines[i] = string(r)
	}

	return lines
}

// lineWith returns the first of lines holding s, or "".
func lineWith(lines []string, s string) string {
	for _, l := range lines {
		if strings.Contains(l, s) {
			return l
		}
	}

	return ""
}

// TestMacro_listHost writes a host source's listing beside it, and checks
// what the heading and the closing lines show: the source's absolute path
// and modification time, the command line, and the object's records.
func TestMacro_listHost(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "small.mar")
	writeHostFile(t, src, macroSource)

	revised := time.Date(2025, time.March, 7, 8, 9, 10, 0, time.Local)
	if err := os.Chtimes(src, revised, revised); err != nil {
		t.Fatal(err)
	}

	command := `MACRO "` + src + `"/LIST`
	if err := c.Macro(MacroOptions{Source: src, List: true, CommandLine: command}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	if names := dirNames(t, dir); !contains(names, "small.lis") || !contains(names, "small.obj") {
		t.Fatalf("files = %v, want small.lis and small.obj", names)
	}

	lines := listingLines(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, "small.lis")})

	if len(lines) < 2 || !strings.HasPrefix(lines[0], "\fSMALL ") {
		t.Fatalf("first line = %q, want a form feed and the module's name", lines[0])
	}

	if !strings.Contains(lines[0], "govax MACRO V"+BuildVersion) {
		t.Errorf("heading = %q, want it to name govax MACRO", lines[0])
	}

	abs, _ := filepath.Abs(src)
	if !strings.HasPrefix(lines[1], "V1.0") || !strings.Contains(lines[1], " 7-MAR-2025 08:09:10  "+abs+" (1)") {
		t.Errorf("second heading line = %q, want the ident, the source's revision date, and its path", lines[1])
	}

	if last := lines[len(lines)-1]; last != command {
		t.Errorf("last line = %q, want the command line %q", last, command)
	}

	// The object was written first, so the listing counts its records.
	if l := lineWith(lines, "source lines were read"); l == "" || strings.Contains(l, "producing 0 object") {
		t.Errorf("statistics line = %q, want the object's records counted", l)
	}

	if l := lineWith(lines, "govax STARLET.MLB"); l == "" {
		t.Error("the library statistics don't name govax's own system library")
	}
}

// TestMacro_listNamed checks /LIST=file: a bare name goes beside the
// source and gets the type LIS, and a host path goes where it says. No
// /LIST writes no listing.
func TestMacro_listNamed(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "small.mar")
	writeHostFile(t, src, macroSource)

	if err := c.Macro(MacroOptions{Source: src, NoObject: true}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("no /LIST left %v", names)
	}

	if err := c.Macro(MacroOptions{Source: src, NoObject: true, List: true, ListFile: "named.lis"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	lines := listingLines(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, "named.lis")})

	// /NOOBJECT: no object records, as real MACRO's listing says.
	if l := lineWith(lines, "source lines were read"); !strings.Contains(l, "producing 0 object records") {
		t.Errorf("statistics line = %q, want 0 object records", l)
	}

	other := filepath.Join(t.TempDir(), "elsewhere.lis")
	if err := c.Macro(MacroOptions{Source: src, NoObject: true, List: true, ListFile: other}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	listingLines(t, c, rms.FileLocation{Host: true, Name: other})
}

// TestMacro_listAfterErrors checks the listing is written when assembly
// fails, though the object isn't, and the command still fails.
func TestMacro_listAfterErrors(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.mar")
	writeHostFile(t, src, "\t.PSECT\tCODE\n\tBOGUS\n\tRSB\n\t.END\n")

	err := c.Macro(MacroOptions{Source: src, List: true})
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_ASMERRORS)) {
		t.Fatalf("Macro error = %v, want CLI_ASMERRORS", err)
	}

	if names := dirNames(t, dir); contains(names, "bad.obj") || !contains(names, "bad.lis") {
		t.Fatalf("files = %v, want bad.lis and no bad.obj", names)
	}

	lines := listingLines(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, "bad.lis")})
	if lineWith(lines, "BOGUS") == "" {
		t.Error("the listing doesn't show the source")
	}

	if lineWith(lines, "There were no errors") != "" {
		t.Error("the listing's summary says there were no errors")
	}
}

// TestMacro_listVolume writes a volume source's listing onto the volume,
// with the heading showing the source's full specification and its
// header's revision date, and a named listing elsewhere on the volume.
func TestMacro_listVolume(t *testing.T) {
	c, _ := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	s := c.ContainerSession

	srcLoc := rms.FileLocation{Name: "DUA0:[000000]SMALL.MAR"}
	if _, err := s.CreateRecordFile(srcLoc, rms.TextRecords, splitSource(macroSource)); err != nil {
		t.Fatal(err)
	}

	revised, err := s.RevisionDate(srcLoc)
	if err != nil || revised.IsZero() {
		t.Fatalf("RevisionDate = %v, %v; want the header's date", revised, err)
	}

	if err := c.Macro(MacroOptions{Source: "DUA0:[000000]SMALL", List: true}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	lines := listingLines(t, c, rms.FileLocation{Name: "DUA0:[000000]SMALL.LIS;1"})

	date := strings.ToUpper(revised.Format("_2-Jan-2006 15:04:05"))
	if !strings.Contains(lines[1], date+"  DUA0:[000000]SMALL.MAR;1") {
		t.Errorf("second heading line = %q, want %s and the source's specification", lines[1], date)
	}

	if err := c.Macro(MacroOptions{Source: "DUA0:[000000]SMALL", List: true, ListFile: "OTHER"}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	listingLines(t, c, rms.FileLocation{Name: "DUA0:[000000]OTHER.LIS;1"})
}

// TestMacro_listLibraryNames checks the library statistics name a
// /LIBRARY library by the file it was read from.
func TestMacro_listLibraryNames(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	lib := filepath.Join(dir, "mine.mlb")
	makeMacroLibrary(t, c, lib, whichMacro("1"))

	src := filepath.Join(dir, "prog.mar")
	writeHostFile(t, src, "\tWHICH\n\t.END\n")

	if err := c.Macro(MacroOptions{Source: src, NoObject: true, List: true, Libraries: []string{"mine"}}); err != nil {
		t.Fatalf("Macro: %v", err)
	}

	lines := listingLines(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, "prog.lis")})

	l := lineWith(lines, "mine.mlb")
	if fields := strings.Fields(l); len(fields) != 2 || fields[1] != "1" {
		t.Errorf("library line = %q, want mine.mlb with 1 macro defined", l)
	}

	if lineWith(lines, "TOTALS (all libraries)") == "" {
		t.Error("no totals line for two libraries")
	}
}

// TestDispatch_macroList checks MACRO's /LIST, /LIST=, and /NOLIST
// through the DCL grammar.
func TestDispatch_macroList(t *testing.T) {
	d, _ := newTestDispatcher(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "small.mar")
	writeHostFile(t, src, macroSource)

	if err := d.Dispatch(`MACRO "` + src + `"/NOOBJECT/NOLIST`); err != nil {
		t.Fatalf("MACRO/NOLIST: %v", err)
	}

	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("/NOLIST left %v", names)
	}

	command := `MACRO "` + src + `"/NOOBJECT/LIST`
	if err := d.Dispatch(command); err != nil {
		t.Fatalf("MACRO/LIST: %v", err)
	}

	lines := listingLines(t, d.Console, rms.FileLocation{Host: true, Name: filepath.Join(dir, "small.lis")})
	if last := lines[len(lines)-1]; last != command {
		t.Errorf("last line = %q, want the command line", last)
	}

	if err := d.Dispatch(`MACRO "` + src + `"/NOOBJECT/LIST="named.lis"`); err != nil {
		t.Fatalf("MACRO/LIST=: %v", err)
	}

	listingLines(t, d.Console, rms.FileLocation{Host: true, Name: filepath.Join(dir, "named.lis")})
}

// TestDispatch_macroShow checks MACRO's /SHOW= and /NOSHOW= through the
// DCL grammar (docs/PHASE-29.md subtask 7): /SHOW=EXPANSIONS lists a
// macro's expansion, /NOSHOW=CALLS leaves its call out, and an option
// MACRO doesn't have fails the command.
// TestDispatch_macroCrossReference checks /CROSS_REFERENCE through DCL:
// symbols and macros by default, the kinds named, none with
// /NOCROSS_REFERENCE, and an unknown kind refused.
func TestDispatch_macroCrossReference(t *testing.T) {
	d, _ := newTestDispatcher(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "xr.mar")

	writeHostFile(t, src, "\t.TITLE\tXR\n\t.PSECT\tD\nVAL = 1\n\t.LONG\tVAL\n\t.END\n")
	
	listing := rms.FileLocation{Host: true, Name: filepath.Join(dir, "xr.lis")}

	for _, tc := range []struct {
		qualifier       string
		symbols, macros bool
		directives      bool
	}{
		{"/CROSS_REFERENCE", true, false, false},
		{"/CROSS_REFERENCE=(DIRECTIVES)", false, false, true},
		{"/CROSS_REFERENCE=ALL", true, false, true},
		{"/NOCROSS_REFERENCE", false, false, false},
		{"", false, false, false},
	} {
		if err := d.Dispatch(`MACRO "` + src + `"/NOOBJECT/LIST` + tc.qualifier); err != nil {
			t.Fatalf("MACRO%s: %v", tc.qualifier, err)
		}

		lines := listingLines(t, d.Console, listing)

		for _, section := range []struct {
			title string
			want  bool
		}{
			{"Symbol Cross Reference", tc.symbols},
			{"Macros Cross Reference", tc.macros},
			{"Directives Cross Reference", tc.directives},
		} {
			if got := lineWith(lines, section.title) != ""; got != section.want {
				t.Errorf("MACRO%s: %s listed %v, want %v", tc.qualifier, section.title, got, section.want)
			}
		}
	}

	if err := d.Dispatch(`MACRO "` + src + `"/NOOBJECT/LIST/CROSS_REFERENCE=(BOGUS)`); err == nil {
		t.Error("/CROSS_REFERENCE=BOGUS: no error")
	}
}

func TestDispatch_macroShow(t *testing.T) {
	d, _ := newTestDispatcher(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "show.mar")
	
	writeHostFile(t, src, "\t.TITLE\tSHOW\n\t.PSECT\tD\n\t.MACRO\tONE\n\t.BYTE\t^X5A\n\t.ENDM\tONE\n\tONE\t\t; the call\n\t.END\n")
	
	listing := rms.FileLocation{Host: true, Name: filepath.Join(dir, "show.lis")}

	if err := d.Dispatch(`MACRO "` + src + `"/NOOBJECT/LIST/SHOW=(EXPANSIONS)`); err != nil {
		t.Fatalf("MACRO/SHOW: %v", err)
	}

	lines := listingLines(t, d.Console, listing)
	if lineWith(lines, "; the call") == "" || lineWith(lines, "5A  ") == "" {
		t.Errorf("/SHOW=EXPANSIONS: the call or its expansion is missing:\n%s", strings.Join(lines, "\n"))
	}

	if err := d.Dispatch(`MACRO "` + src + `"/NOOBJECT/LIST/NOSHOW=(MC)`); err != nil {
		t.Fatalf("MACRO/NOSHOW: %v", err)
	}

	if line := lineWith(listingLines(t, d.Console, listing), "; the call"); line != "" {
		t.Errorf("/NOSHOW=MC listed the call: %q", line)
	}

	if err := d.Dispatch(`MACRO "` + src + `"/NOOBJECT/LIST/SHOW=(SYMBOLS)`); err == nil {
		t.Error("/SHOW=SYMBOLS: no error")
	}
}
