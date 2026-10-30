package console

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// The FORTRAN command (docs/PHASE-74 - FORTRAN plan.md, subtask 2): it
// reads a FORTRAN source from the host or a volume and writes a MACRO
// file, which so far holds the source as comments and assembles into an
// empty module.

// fortranSource is a small program, with a tab-format line.
const fortranSource = "      PROGRAM SQUARES\n" +
	"C     Squares of the first five integers\n" +
	"      DO 10 I = 1, 5\n" +
	"\t PRINT *, I, I*I\n" +
	"10    CONTINUE\n" +
	"      END\n"

// readMACRO reads the MACRO file at loc as lines.
func readMACRO(t *testing.T, c *Console, loc rms.FileLocation) []string {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		t.Fatalf("reading %s: %v", loc.Name, err)
	}

	lines := make([]string, len(records))
	for i, r := range records {
		lines[i] = string(r)
	}

	return lines
}

// checkMACRO checks a MACRO file the compiler wrote: its module name,
// the source lines among its comments, and that MACRO assembles it
// without a message.
func checkMACRO(t *testing.T, c *Console, buf interface {
	Reset()
	String() string
}, loc rms.FileLocation, module string,
) {
	t.Helper()

	lines := readMACRO(t, c, loc)
	if len(lines) == 0 || lines[0] != "\t.TITLE\t"+module {
		t.Fatalf("%s: first line %q, want .TITLE %s", loc.Name, lines[:min(1, len(lines))], module)
	}

	text := strings.Join(lines, "\n")
	for _, want := range []string{";    1        PROGRAM SQUARES", ";    4  \t PRINT *, I, I*I", "\t.END"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s doesn't hold %q", loc.Name, want)
		}
	}

	buf.Reset()

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: loc.Name, SourceHost: loc.Host, NoObject: true}); err != nil {
		t.Errorf("MACRO of %s: %v", loc.Name, err)
	}

	if out := buf.String(); out != "" {
		t.Errorf("MACRO of %s said:\n%s", loc.Name, out)
	}
}

// TestFortran_host compiles a host source: the MACRO file goes beside it,
// named for it with the type MAR in the source's case, and the source's
// type defaults to FOR.
func TestFortran_host(t *testing.T) {
	cases := []struct{ source, typed, want, module string }{
		{"squares.for", "squares.for", "squares.mar", "SQUARES"},
		{"SQUARES.FOR", "SQUARES.FOR", "SQUARES.MAR", "SQUARES"},
		{"prog.for", "prog", "prog.mar", "PROG"}, // the default type
		{"my-prog.f", "my-prog.f", "my-prog.mar", "MY_PROG"},
	}

	for _, tc := range cases {
		c, buf := newTestConsole(t)
		dir := t.TempDir()
		writeHostFile(t, filepath.Join(dir, tc.source), fortranSource)

		buf.Reset()

		if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{filepath.Join(dir, tc.typed)}}); err != nil {
			t.Errorf("%s: Fortran: %v", tc.typed, err)

			continue
		}

		if buf.Len() != 0 {
			t.Errorf("%s: FORTRAN said %q", tc.typed, buf.String())
		}

		if names := dirNames(t, dir); len(names) != 2 || !contains(names, tc.want) {
			t.Errorf("%s: directory holds %v, want %s beside the source", tc.typed, names, tc.want)

			continue
		}

		checkMACRO(t, c, buf, rms.FileLocation{Host: true, Name: filepath.Join(dir, tc.want)}, tc.module)
	}
}

// TestFortran_header checks the header's version, source, and command.
func TestFortran_header(t *testing.T) {
	saved := dclcmd.BuildVersion
	dclcmd.BuildVersion = "1.0-200"

	defer func() { dclcmd.BuildVersion = saved }()

	c, _ := newTestConsole(t)
	src := filepath.Join(t.TempDir(), "squares.for")
	writeHostFile(t, src, fortranSource)

	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{src}, CommandLine: "FORTRAN SQ"}); err != nil {
		t.Fatal(err)
	}

	text := strings.Join(readMACRO(t, c, rms.FileLocation{Host: true, Name: strings.TrimSuffix(src, ".for") + ".mar"}), "\n")

	for _, want := range []string{"; Compiled by govax FORTRAN V1.0-200 on ", "; from " + src, "; by the command FORTRAN SQ"} {
		if !strings.Contains(text, want) {
			t.Errorf("the header lacks %q:\n%s", want, text)
		}
	}
}

// TestFortran_volume compiles sources on a mounted volume: the MACRO
// file goes beside the source, each compilation makes its next version,
// /MACRO= names it, and a host MACRO file can come from a volume source.
func TestFortran_volume(t *testing.T) {
	c, buf := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	s := c.ContainerSession
	if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]SQUARES.FOR"}, rms.TextRecords, splitSource(fortranSource)); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{"DUA0:[000000]SQUARES"}}); err != nil {
		t.Fatalf("Fortran: %v", err)
	}

	checkMACRO(t, c, buf, rms.FileLocation{Name: "DUA0:[000000]SQUARES.MAR;1"}, "SQUARES")

	// A bare name with the default on the volume, compiled again.
	if err := c.cmd().SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{"SQUARES.FOR"}}); err != nil {
		t.Fatalf("Fortran: %v", err)
	}

	lines := readMACRO(t, c, rms.FileLocation{Name: "DUA0:[000000]SQUARES.MAR;2"})
	if !strings.Contains(strings.Join(lines, "\n"), "; from DUA0:[000000]SQUARES.FOR;1") {
		t.Error("the header doesn't name the volume source with its version")
	}

	// /MACRO= without a type gets MAR, as /OBJECT= gets OBJ.
	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{"SQUARES"}, Macro: "OTHER"}); err != nil {
		t.Fatalf("Fortran/MACRO=: %v", err)
	}

	checkMACRO(t, c, buf, rms.FileLocation{Name: "DUA0:[000000]OTHER.MAR;1"}, "SQUARES")

	// To the host.
	hostMAR := filepath.Join(t.TempDir(), "squares.mar")
	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{"SQUARES"}, Macro: hostMAR}); err != nil {
		t.Fatalf("Fortran/MACRO=host: %v", err)
	}

	checkMACRO(t, c, buf, rms.FileLocation{Host: true, Name: hostMAR}, "SQUARES")
}

// TestFortran_noMacro checks /NOMACRO: the source is compiled, nothing
// written.
func TestFortran_noMacro(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "squares.for")
	writeHostFile(t, src, fortranSource)

	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{src}, NoMacro: true}); err != nil {
		t.Fatal(err)
	}

	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("/NOMACRO left %v", names)
	}
}

// TestFortran_failures checks a missing source, a volume not mounted, and
// a host .MAR source that would be its own MACRO file.
func TestFortran_failures(t *testing.T) {
	c, _ := newTestConsole(t)
	dir := t.TempDir()

	err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{filepath.Join(dir, "none")}})
	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("missing source: %v, want SS_NOSUCHFILE", err)
	}

	err = c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{"DUB0:[X]NONE.FOR"}})
	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("unmounted volume: %v, want SS_DEVNOTMOUNT", err)
	}

	src := filepath.Join(dir, "prog.mar")
	writeHostFile(t, src, fortranSource)

	err = c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{src}})
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_FORSAMEFILE)) {
		t.Errorf(".MAR source: %v, want CLI_FORSAMEFILE", err)
	}

	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("the refused compilation left %v", names)
	}

	// Named elsewhere, it compiles.
	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{src}, Macro: filepath.Join(dir, "out.mar")}); err != nil {
		t.Errorf("/MACRO= for a .MAR source: %v", err)
	}
}

// TestDispatch_fortran runs FORTRAN through DCL: its qualifiers, a list
// of sources (each compiled, a failure shown and the others still
// compiled), $STATUS, and the abbreviation FOR.
func TestDispatch_fortran(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.for")
	b := filepath.Join(dir, "b.for")
	writeHostFile(t, a, fortranSource)
	writeHostFile(t, b, fortranSource)

	if err := d.Dispatch(`FORTRAN/NOMACRO/EXTEND_SOURCE/D_LINES "` + a + `"`); err != nil {
		t.Fatalf("FORTRAN/NOMACRO: %v", err)
	}

	if names := dirNames(t, dir); len(names) != 2 {
		t.Errorf("/NOMACRO left %v", names)
	}

	if err := d.Dispatch(`FOR "` + a + `"/MACRO="named.mar"`); err != nil {
		t.Fatalf("FOR/MACRO=: %v", err)
	}

	checkMACRO(t, c, buf, rms.FileLocation{Host: true, Name: filepath.Join(dir, "named.mar")}, "A")

	if status := c.Status() &^ stsInhibitMsg; status != 1 {
		t.Errorf("$STATUS after a clean compilation = %%X%08X, want SS$_NORMAL", status)
	}

	buf.Reset()

	missing := filepath.Join(dir, "none.for")
	if err := d.Dispatch(`FORTRAN "` + a + `","` + missing + `","` + b + `"`); err != nil {
		t.Fatalf("FORTRAN of a list: %v", err)
	}

	if !strings.Contains(buf.String(), "NOSUCHFILE") || strings.Count(buf.String(), "%") != 1 {
		t.Errorf("the list's output = %q, want the one failure", buf.String())
	}

	for _, name := range []string{"a.mar", "b.mar"} {
		checkMACRO(t, c, buf, rms.FileLocation{Host: true, Name: filepath.Join(dir, name)}, strings.ToUpper(strings.TrimSuffix(name, ".mar")))
	}

	if status := c.Status() &^ stsInhibitMsg; status != vmserrors.SS_NOSUCHFILE {
		t.Errorf("$STATUS after the list = %%X%08X, want the failure's, SS_NOSUCHFILE", status)
	}
}
