package dclcmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// These tests run the FORTRAN command on a Context of its own, without a console,
// so they stay quick (docs/planning/PHASE-99 - Fortran 90.md, subtask 2).

// newFortranContext is a Context on a fresh volume, DUA0: and the default, holding
// the files given (name and lines), its output in the buffer returned.
func newFortranContext(t *testing.T, files map[string][]string) (*Context, *bytes.Buffer) {
	t.Helper()

	return newFortranContextSized(t, files, 2000)
}

// newFortranContextSized is newFortranContext on a volume of the blocks given.
func newFortranContextSized(t *testing.T, files map[string][]string, blocks uint32) (*Context, *bytes.Buffer) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ftn.dsk")
	if err := rms.InitializeContainer(path, blocks, "FTN", 0, ""); err != nil {
		t.Fatal(err)
	}

	mounts := rms.NewMountTable()
	if err := mounts.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = mounts.Dismount("DUA0") })

	s := rms.NewSession(mounts)
	if err := s.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	for name, lines := range files {
		records := make([][]byte, len(lines))
		for i, line := range lines {
			records[i] = []byte(line)
		}

		if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]" + name}, rms.TextRecords, records); err != nil {
			t.Fatal(name, err)
		}
	}

	out := &bytes.Buffer{}

	return &Context{ContainerSession: s, Out: out}, out
}

// A fixed-form program, and the same in a file the compiler takes for free form.
var (
	fixedProgram = []string{"      PROGRAM P", "      INCLUDE 'INC'", "      END"}
	fixedInclude = []string{"      I = 1"}

	// Only free form reads these cleanly: in fixed form a statement starting in
	// column 1 has letters in its label field.
	freeProgram = []string{"program p; include 'INC'", "end"}
	freeInclude = []string{"i = 1; j = 2"}
)

// readAsFixed reports whether out has the message fixed form gives a free-form
// line, in file.
func readAsFixed(out, file string) bool {
	return strings.Contains(out, "BADLABEL") && strings.Contains(out, file)
}

// TestFortranSourceTypes checks the default types (.FOR, then .F90), the form a
// type gives, and /SOURCE_FORM over the files named but not an INCLUDE'd one.
func TestFortranSourceTypes(t *testing.T) {
	t.Run(".FOR first", func(t *testing.T) {
		c, out := newFortranContext(t, map[string][]string{"P.FOR": fixedProgram, "P.F90": fixedProgram, "INC.FOR": fixedInclude})

		if err := c.Fortran(FortranOptions{Sources: FortranSources("P")}); err != nil || out.Len() != 0 {
			t.Fatalf("err %v, output %q", err, out)
		}
	})

	t.Run(".F90 when there's no .FOR, free form", func(t *testing.T) {
		c, out := newFortranContext(t, map[string][]string{"P.F90": freeProgram, "INC.FOR": fixedInclude})

		if err := c.Fortran(FortranOptions{Sources: FortranSources("P")}); err != nil || out.Len() != 0 {
			t.Fatalf("err %v, output %q", err, out)
		}
	})

	t.Run("/SOURCE_FORM=FIXED", func(t *testing.T) {
		c, out := newFortranContext(t, map[string][]string{"P.F90": fixedProgram, "INC.FOR": fixedInclude})

		if err := c.Fortran(FortranOptions{Sources: FortranSources("P"), SourceForm: "FIXED"}); err != nil || out.Len() != 0 {
			t.Fatalf("err %v, output %q", err, out)
		}

		c, out = newFortranContext(t, map[string][]string{"P.F90": freeProgram, "INC.FOR": fixedInclude})

		err := c.Fortran(FortranOptions{Sources: FortranSources("P"), SourceForm: "FIXED"})

		var vmsErr vmserrors.VMSError
		if !errors.As(err, &vmsErr) || vmsErr.Status != vmserrors.CLI_FORERRORS || !readAsFixed(out.String(), "P.F90") {
			t.Fatalf("err %v, output %q", err, out)
		}
	})

	t.Run("an INCLUDE'd file's own type", func(t *testing.T) {
		c, out := newFortranContext(t, map[string][]string{"P.FOR": fixedProgram, "INC.F90": freeInclude})

		if err := c.Fortran(FortranOptions{Sources: FortranSources("P"), SourceForm: "FIXED"}); err != nil || out.Len() != 0 {
			t.Fatalf("err %v, output %q", err, out)
		}
	})

	t.Run("/SOURCE_FORM=FREE", func(t *testing.T) {
		c, out := newFortranContext(t, map[string][]string{"P.FOR": freeProgram, "INC.FOR": fixedInclude})

		if err := c.Fortran(FortranOptions{Sources: FortranSources("P"), SourceForm: "free"}); err != nil || out.Len() != 0 {
			t.Fatalf("err %v, output %q", err, out)
		}

		c, out = newFortranContext(t, map[string][]string{"P.FOR": freeProgram, "INC.FOR": fixedInclude})

		if err := c.Fortran(FortranOptions{Sources: FortranSources("P")}); err == nil || !readAsFixed(out.String(), "P.FOR") {
			t.Fatalf("without /SOURCE_FORM: err %v, output %q", err, out)
		}
	})

	t.Run("a bad /SOURCE_FORM", func(t *testing.T) {
		c, _ := newFortranContext(t, nil)

		err := c.Fortran(FortranOptions{Sources: FortranSources("P"), SourceForm: "LOOSE"})

		var vmsErr vmserrors.VMSError
		if !errors.As(err, &vmsErr) || vmsErr.Status != vmserrors.CLI_FORSOURCEFORM {
			t.Fatalf("err %v, want FORSOURCEFORM", err)
		}
	})

	t.Run("neither is the .FOR file's error", func(t *testing.T) {
		c, _ := newFortranContext(t, nil)

		err := c.Fortran(FortranOptions{Sources: FortranSources("P")})
		if err == nil || !strings.Contains(err.Error(), "P.FOR") {
			t.Fatalf("err %v, want one naming P.FOR", err)
		}
	})
}

// TestFortranHostF90 checks the same lookup on the host: a lowercase .f90 is free
// form too.
func TestFortranHostF90(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.f90"), []byte("program p; i = 1\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, out := newFortranContext(t, nil)

	err := c.Fortran(FortranOptions{Sources: FortranSources(filepath.Join(dir, "p")), SourceHost: true, NoMacro: true})
	if err != nil || out.Len() != 0 {
		t.Fatalf("err %v, output %q", err, out)
	}
}
