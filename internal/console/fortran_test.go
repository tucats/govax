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

// TestFortran_include checks INCLUDE files on the host (subtask 3): found
// beside the file that includes them, with the default type FOR, nested,
// their lines in the MACRO file; and a missing one's fatal message.
func TestFortran_include(t *testing.T) {
	c, buf := newTestConsole(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "squares.for")

	writeHostFile(t, src, "      PROGRAM SQUARES\n      INCLUDE 'inc/limits'\n      END\n")
	writeHostFile(t, filepath.Join(dir, "inc", "limits.for"), "      PARAMETER (N=5)\n      INCLUDE 'more.inc'\n")
	writeHostFile(t, filepath.Join(dir, "inc", "more.inc"), "C     found beside limits.for\n")

	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{src}}); err != nil {
		t.Fatalf("Fortran: %v\n%s", err, buf.String())
	}

	if out := buf.String(); out != "" {
		t.Errorf("FORTRAN said:\n%s", out)
	}

	mar := rms.FileLocation{Host: true, Name: filepath.Join(dir, "squares.mar")}
	text := strings.Join(readMACRO(t, c, mar), "\n")

	for _, want := range []string{
		"; INCLUDE file " + filepath.Join(dir, "inc", "limits.for") + ":",
		";1   1        PARAMETER (N=5)",
		";2   1  C     found beside limits.for",
		"; end of " + filepath.Join(dir, "inc", "more.inc"),
		";    3        END",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the MACRO file doesn't hold %q:\n%s", want, text)
		}
	}

	buf.Reset()

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: mar.Name, SourceHost: true, NoObject: true}); err != nil || buf.Len() != 0 {
		t.Errorf("MACRO: %v\n%s", err, buf.String())
	}

	// A missing INCLUDE file is fatal: the message, and no MACRO file.
	bad := filepath.Join(dir, "bad.for")
	writeHostFile(t, bad, "      PROGRAM BAD\n      INCLUDE 'nowhere'\n      END\n")
	buf.Reset()

	err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{bad}})
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_FORERRORS)) {
		t.Errorf("missing INCLUDE file: %v, want CLI_FORERRORS", err)
	}

	if out := buf.String(); !strings.HasPrefix(out, "%FORT-F-INCLOPEN, can't read the INCLUDE file 'nowhere': ") ||
		!strings.Contains(out, "NOSUCHFILE") || !strings.Contains(out, "at line 2 of "+bad) {
		t.Errorf("message:\n%s", out)
	}

	if names := dirNames(t, dir); len(names) != 4 {
		t.Errorf("a MACRO file was written after a fatal error: %v", names)
	}
}

// TestFortran_includeVolume checks INCLUDE files on a volume: a bare name
// on the including file's device and directory, and a full specification.
func TestFortran_includeVolume(t *testing.T) {
	c, buf := newTestConsole(t)
	mountFreshContainer(t, c, "DUA0")

	s := c.ContainerSession
	for name, text := range map[string]string{
		"DUA0:[000000]SQUARES.FOR": "      PROGRAM SQUARES\n      INCLUDE 'LIMITS'\n      INCLUDE 'DUA0:[000000]OTHER.INC'\n      END\n",
		"DUA0:[000000]LIMITS.FOR":  "      PARAMETER (N=5)\n",
		"DUA0:[000000]OTHER.INC":   "      INTEGER K\n",
	} {
		if _, err := s.CreateRecordFile(rms.FileLocation{Name: name}, rms.TextRecords, splitSource(text)); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.cmd().Fortran(dclcmd.FortranOptions{Sources: []string{"DUA0:[000000]SQUARES"}}); err != nil {
		t.Fatalf("Fortran: %v\n%s", err, buf.String())
	}

	text := strings.Join(readMACRO(t, c, rms.FileLocation{Name: "DUA0:[000000]SQUARES.MAR;1"}), "\n")
	for _, want := range []string{
		"; INCLUDE file DUA0:[000000]LIMITS.FOR;1:",
		";1   1        PARAMETER (N=5)",
		"; INCLUDE file DUA0:[000000]OTHER.INC;1:",
		";1   1        INTEGER K",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the MACRO file doesn't hold %q:\n%s", want, text)
		}
	}
}

// TestDispatch_fortranSource runs FORTRAN through DCL on sources with
// layout errors and with /CONTINUATIONS.
func TestDispatch_fortranSource(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "cont.for")

	// A statement with 20 continuation lines: one more than the default.
	text := "      X = 0\n" + strings.Repeat("     1 + 1\n", 20) + "      END\n"
	writeHostFile(t, src, text)

	if err := d.Dispatch(`FORTRAN/NOMACRO "` + src + `"`); !errors.Is(err, vmserrors.New(vmserrors.CLI_FORERRORS)) {
		t.Errorf("FORTRAN: %v, want CLI_FORERRORS", err)
	}

	if out := buf.String(); !strings.Contains(out, "%FORT-E-TOOMANYCONT, statement has more than 19 continuation lines") ||
		!strings.Contains(out, "at line 21 of "+src) {
		t.Errorf("output:\n%s", out)
	}

	buf.Reset()

	if err := d.Dispatch(`FORTRAN/CONTINUATIONS=20 "` + src + `"`); err != nil {
		t.Fatalf("FORTRAN/CONTINUATIONS=20: %v", err)
	}

	if out := buf.String(); out != "" {
		t.Errorf("FORTRAN/CONTINUATIONS=20 said:\n%s", out)
	}

	// run runs a FORTRAN command, which fails with want (or, for 0,
	// succeeds).
	run := func(command string, want uint32) {
		t.Helper()
		buf.Reset()

		err := d.Dispatch(command)
		if want == 0 && err != nil || want != 0 && !errors.Is(err, vmserrors.New(want)) {
			t.Errorf("%s: %v, want %%X%08X\n%s", command, err, want, buf.String())
		}
	}

	run(`FORTRAN/CONTINUATIONS=100/NOMACRO "`+src+`"`, vmserrors.CLI_FORCONTRANGE)

	if err := d.Dispatch(`FORTRAN/CONTINUATIONS=100/NOMACRO "` + src + `"`); err == nil ||
		!strings.Contains(err.Error(), "/CONTINUATIONS=100 is out of range; give 0 to 99") {
		t.Errorf("/CONTINUATIONS=100: %v", err)
	}

	// /CONTINUATIONS=0 allows none.
	run(`FORTRAN/CONTINUATIONS=0/NOMACRO "`+src+`"`, vmserrors.CLI_FORERRORS)

	if !strings.Contains(buf.String(), "more than 0 continuation lines") {
		t.Errorf("output:\n%s", buf.String())
	}

	// /D_LINES compiles a debugging line, whose label error is then seen.
	dsrc := filepath.Join(dir, "d.for")
	writeHostFile(t, dsrc, "D  X  Y = 1\n      END\n")
	run(`FORTRAN/NOMACRO "`+dsrc+`"`, 0)
	run(`FORTRAN/NOMACRO/D_LINES "`+dsrc+`"`, vmserrors.CLI_FORERRORS)

	if !strings.Contains(buf.String(), "%FORT-E-BADLABEL") {
		t.Errorf("output:\n%s", buf.String())
	}
}
