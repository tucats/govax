package dclcmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Module files and their search (docs/planning/PHASE-106 - Fortran 90
// modules.md, subtask 5; fortranmodules.go).

var (
	shapesModule = []string{"module shapes", "  real :: area = 0.0", "contains", "  subroutine grow(x)", "    real :: x", "    area = area + x", "  end subroutine", "end module"}
	shapesUser   = []string{"program p", "  use shapes", "  call grow(2.0)", "  print *, area", "end"}
)

// volumeFile reads a volume file's lines and its full name, or fails.
func volumeFile(t *testing.T, c *Context, name string) ([]string, string) {
	t.Helper()

	records, found, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: name}, rms.TextRecords)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	return recordLines(records), found.Name
}

// TestModuleFileWritten checks that a module's compilation writes its module
// file in the default directory, that its user compiled apart finds it, and
// that compiling the module again with nothing changed leaves the file's
// version alone, while a change makes a new one.
func TestModuleFileWritten(t *testing.T) {
	c, out := newFortranContext(t, map[string][]string{"SHAPES.F90": shapesModule, "P.F90": shapesUser})

	if err := c.Fortran(FortranOptions{Sources: FortranSources("SHAPES")}); err != nil || out.Len() != 0 {
		t.Fatalf("err %v, output %q", err, out)
	}

	lines, name := volumeFile(t, c, "SHAPES.F90$MOD")
	if !strings.HasSuffix(name, "SHAPES.F90$MOD;1") || lines[0] != "!GOVAX$ MODULE FILE 1" {
		t.Fatalf("%s: %q", name, lines)
	}

	if err := c.Fortran(FortranOptions{Sources: FortranSources("P")}); err != nil || out.Len() != 0 {
		t.Fatalf("the user: err %v, output %q", err, out)
	}

	macro, _ := volumeFile(t, c, "P.MAR")
	if text := strings.Join(macro, "\n"); !strings.Contains(text, ".EXTRN\t$MOD$SHAPES.") {
		t.Errorf("the user refers to no stamp:\n%s", text)
	}

	module, _ := volumeFile(t, c, "SHAPES.MAR")
	if text := strings.Join(module, "\n"); !strings.Contains(text, "$MOD$SHAPES.") || !strings.Contains(text, "==") {
		t.Errorf("the module defines no stamp:\n%s", text)
	}

	// Again, the same: no new version.
	if err := c.Fortran(FortranOptions{Sources: FortranSources("SHAPES")}); err != nil {
		t.Fatal(err)
	}

	if _, name := volumeFile(t, c, "SHAPES.F90$MOD"); !strings.HasSuffix(name, ";1") {
		t.Errorf("an unchanged module made %s", name)
	}

	// A change to a body alone changes nothing a user sees.
	changed := append([]string{}, shapesModule...)
	changed[5] = "    area = area + 2.0 * x"
	writeVolumeFile(t, c, "SHAPES.F90", changed)

	if err := c.Fortran(FortranOptions{Sources: FortranSources("SHAPES")}); err != nil {
		t.Fatal(err)
	}

	if _, name := volumeFile(t, c, "SHAPES.F90$MOD"); !strings.HasSuffix(name, ";1") {
		t.Errorf("a module changed only in a body made %s", name)
	}

	// A new entity is a new text.
	changed = append(changed[:2:2], append([]string{"  integer :: calls"}, changed[2:]...)...)
	writeVolumeFile(t, c, "SHAPES.F90", changed)

	if err := c.Fortran(FortranOptions{Sources: FortranSources("SHAPES")}); err != nil {
		t.Fatal(err)
	}

	if _, name := volumeFile(t, c, "SHAPES.F90$MOD"); !strings.HasSuffix(name, ";2") {
		t.Errorf("a changed module made %s", name)
	}
}

// writeVolumeFile writes a new version of a volume file in the default
// directory.
func writeVolumeFile(t *testing.T, c *Context, name string, lines []string) {
	t.Helper()

	records := make([][]byte, len(lines))
	for i, line := range lines {
		records[i] = []byte(line)
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: name}, rms.TextRecords, records); err != nil {
		t.Fatal(err)
	}
}

// TestModuleFileErrors checks that a compilation with an error writes no
// module file, even of a module that compiled, and that /NOMACRO still writes
// one.
func TestModuleFileErrors(t *testing.T) {
	bad := append(append([]string{}, shapesModule...), "program q", "  x = ", "end")

	c, _ := newFortranContext(t, map[string][]string{"SHAPES.F90": bad})

	if err := c.Fortran(FortranOptions{Sources: FortranSources("SHAPES")}); err == nil {
		t.Fatal("no error")
	}

	if _, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "SHAPES.F90$MOD"}, rms.TextRecords); err == nil {
		t.Error("a module file after an error")
	}

	c, _ = newFortranContext(t, map[string][]string{"SHAPES.F90": shapesModule})

	if err := c.Fortran(FortranOptions{Sources: FortranSources("SHAPES"), NoMacro: true}); err != nil {
		t.Fatal(err)
	}

	volumeFile(t, c, "SHAPES.F90$MOD")

	if _, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "SHAPES.MAR"}, rms.TextRecords); err == nil {
		t.Error("a MACRO file with /NOMACRO")
	}
}

// TestModuleSearch checks the search's order and places on a volume: /MODULE's
// directory, the source's directory, then /MODULE_PATH's or FORT$MODULES's, and
// MODNOTFOUND's list of every name looked for.
func TestModuleSearch(t *testing.T) {
	c, out := newFortranContext(t, nil)
	s := c.ContainerSession

	for _, dir := range []string{"[MODS]", "[SRC]", "[LIB1]", "[LIB2]"} {
		if _, err := s.CreateDirectory("DUA0:"+dir, rms.CreateDirectoryOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	writeVolumeFile(t, c, "DUA0:[SRC]SHAPES.F90", shapesModule)
	writeVolumeFile(t, c, "DUA0:[SRC]P.F90", shapesUser)

	// A module written to [MODS], found there by a user given /MODULE.
	if err := c.Fortran(FortranOptions{Sources: FortranSources("[SRC]SHAPES"), ModuleDir: "[MODS]", NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("err %v, output %q", err, out)
	}

	volumeFile(t, c, "DUA0:[MODS]SHAPES.F90$MOD")

	if err := c.Fortran(FortranOptions{Sources: FortranSources("[SRC]P"), ModuleDir: "[MODS]", NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("with /MODULE: err %v, output %q", err, out)
	}

	// Not found in the default directory or the source's.
	err := c.Fortran(FortranOptions{Sources: FortranSources("[SRC]P"), NoMacro: true})
	if err == nil || !strings.Contains(out.String(), "MODNOTFOUND") ||
		!strings.Contains(out.String(), "DUA0:[000000]SHAPES.F90$MOD;") || !strings.Contains(out.String(), "DUA0:[SRC]SHAPES.F90$MOD;") {
		t.Fatalf("err %v, output %q", err, out)
	}

	// /MODULE_PATH, and FORT$MODULES when it isn't given.
	out.Reset()

	if err := c.Fortran(FortranOptions{Sources: FortranSources("[SRC]P"), ModulePath: []string{"[LIB1]", "[MODS]"}, NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("with /MODULE_PATH: err %v, output %q", err, out)
	}

	if _, err := s.Logicals.Define(lnm.ProcessTableName, fortModulesName, lnm.User, 0, []lnm.Equivalence{{Value: "DUA0:[LIB2]"}, {Value: "DUA0:[MODS]"}}); err != nil {
		t.Fatal(err)
	}

	if err := c.Fortran(FortranOptions{Sources: FortranSources("[SRC]P"), NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("with FORT$MODULES: err %v, output %q", err, out)
	}

	// A module file beside the source.
	if err := c.Fortran(FortranOptions{Sources: FortranSources("[SRC]SHAPES"), ModuleDir: "[SRC]", NoMacro: true}); err != nil {
		t.Fatal(err)
	}

	if err := c.Fortran(FortranOptions{Sources: FortranSources("[SRC]P"), ModulePath: []string{"[LIB1]"}, NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("beside the source: err %v, output %q", err, out)
	}

	// /MODULE naming a file.
	err = c.Fortran(FortranOptions{Sources: FortranSources("[SRC]P"), ModuleDir: "[MODS]X.Y"})

	var vmsErr vmserrors.VMSError
	if !errors.As(err, &vmsErr) || vmsErr.Status != vmserrors.CLI_FORMODULEDIR {
		t.Fatalf("err %v, want FORMODULEDIR", err)
	}
}

// TestModuleSearchHost checks module files on the host: written beside nothing
// but in /MODULE's directory, named in the source type's case, and found in
// either case.
func TestModuleSearchHost(t *testing.T) {
	src, mods := t.TempDir(), t.TempDir()

	write := func(name string, lines []string) {
		if err := os.WriteFile(filepath.Join(src, name), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("shapes.f90", shapesModule)
	write("p.f90", shapesUser)

	c, out := newFortranContext(t, nil)

	if err := c.Fortran(FortranOptions{Sources: FortranSources(filepath.Join(src, "shapes")), SourceHost: true, ModuleDir: mods, NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("err %v, output %q", err, out)
	}

	if _, err := os.Stat(filepath.Join(mods, "shapes.f90$mod")); err != nil {
		t.Fatal(err)
	}

	if err := c.Fortran(FortranOptions{Sources: FortranSources(filepath.Join(src, "p")), SourceHost: true, ModuleDir: mods, NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("the user: err %v, output %q", err, out)
	}

	// An uppercase name is found by a lowercase source.
	if err := os.Rename(filepath.Join(mods, "shapes.f90$mod"), filepath.Join(src, "SHAPES.F90$MOD")); err != nil {
		t.Fatal(err)
	}

	if err := c.Fortran(FortranOptions{Sources: FortranSources(filepath.Join(src, "p")), SourceHost: true, ModuleDir: mods, NoMacro: true}); err != nil || out.Len() != 0 {
		t.Fatalf("in the other case: err %v, output %q", err, out)
	}

	// A host /MODULE directory must exist.
	err := c.Fortran(FortranOptions{Sources: FortranSources(filepath.Join(src, "p")), SourceHost: true, ModuleDir: filepath.Join(mods, "none")})

	var vmsErr vmserrors.VMSError
	if !errors.As(err, &vmsErr) || vmsErr.Status != vmserrors.CLI_FORMODULEDIR {
		t.Fatalf("err %v, want FORMODULEDIR", err)
	}
}

// TestModuleStampLink checks the stamp (finding 9): a user's object links with
// its module's, and after the module changes and is compiled again, linking the
// old user object with the new module object is LINK's undefined symbol, the
// stamp the user's object asks for.
func TestModuleStampLink(t *testing.T) {
	c, out := newFortranContext(t, map[string][]string{"SHAPES.F90": shapesModule, "P.F90": shapesUser})

	for _, name := range []string{"SHAPES", "P"} {
		if err := c.Fortran(FortranOptions{Sources: FortranSources(name), Object: true}); err != nil {
			t.Fatalf("%s: err %v, output %q", name, err, out)
		}
	}

	out.Reset()

	if err := c.Link(LinkOptions{Objects: []string{"P", "SHAPES"}, NoExecutable: true}); err != nil || out.Len() != 0 {
		t.Fatalf("link: err %v, output %q", err, out)
	}

	changed := append(shapesModule[:2:2], append([]string{"  integer :: calls"}, shapesModule[2:]...)...)
	writeVolumeFile(t, c, "SHAPES.F90", changed)

	if err := c.Fortran(FortranOptions{Sources: FortranSources("SHAPES"), Object: true}); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	err := c.Link(LinkOptions{Objects: []string{"P", "SHAPES"}, NoExecutable: true})
	if text := out.String() + errorText(err); !strings.Contains(text, "$MOD$SHAPES.") {
		t.Fatalf("a stale user object linked: err %v, output %q", err, out)
	}
}

// errorText is err's text, or "" for none.
func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
