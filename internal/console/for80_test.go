package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Phase 80 probe (docs/PHASE-80 - FORTRAN formatted IO.md, subtask 2):
// testdata/for80's for80.mar calls FORRTL's statement routines, case by case.
// VMS's run of it is FTNRTL's requirement.

var for80Dir = filepath.Join("..", "..", "testdata", "for80")

// for80Programs are the probe's programs that use FORRTL.
var for80Programs = []string{"FOR80", "FOR80E", "FOR80T"}

// copyFor80 copies the probe's sources to the default directory.
func copyFor80(t *testing.T, dispatch func(string) string) {
	t.Helper()

	for _, name := range []string{"for80.mar", "for80d.mar", "for80e.mar", "for80t.mar", "for80r.com", "for80b.mar"} {
		src, err := filepath.Abs(filepath.Join(for80Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + strings.ToUpper(name))
	}
}

// TestFor80Assembles: every program assembles without a message, so the volume
// the VAX runs them from needs no second try.
func TestFor80Assembles(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 FOR80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	copyFor80(t, dispatch)

	for _, name := range append([]string{"FOR80D", "FOR80B"}, for80Programs...) {
		if out := dispatch("MACRO/NOLIST " + name); out != "" {
			t.Errorf("MACRO %s: %s", name, out)
		}
	}
}

// TestFor80RunsOnFORRTL links the programs against VMS's FORRTL.EXE, runs the
// cases, and checks that they reach their end with no driver line lost. FORRTL
// runs over govax's LIBRTL here, so this says nothing of what's right: VMS's run
// does.
func TestFor80RunsOnFORRTL(t *testing.T) {
	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 FOR80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)
	copyFor80(t, dispatch)

	opt := filepath.Join(t.TempDir(), "for80.opt")
	if err := os.WriteFile(opt, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + opt + `"/HOST FOR80.OPT`)

	for _, name := range for80Programs {
		if out := dispatch("MACRO/NOLIST " + name); out != "" {
			t.Fatalf("MACRO %s: %s", name, out)
		}

		if out := dispatch("LINK/NOMAP " + name + ",FOR80/OPTIONS"); out != "" {
			t.Fatalf("LINK %s: %s", name, out)
		}
	}

	dispatch("MACRO/NOLIST FOR80D")
	dispatch("LINK/NOMAP FOR80D")
	dispatch("DEFINE/NOLOG FOR006 FOR80.DAT")

	if out := dispatch("@FOR80R"); strings.Contains(out, "was lost") {
		t.Errorf("a driver line was lost: %s", out)
	}

	dispatch("DEASSIGN FOR006")

	got := dispatch("RUN FOR80D")
	for _, want := range []string{"|##### the end of the cases|", "|M127 001883FA"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in the output", want)
		}
	}

	// A case's values, the manual's own list-directed example (7-19).
	if !strings.Contains(got, "| v7=0000000E|") {
		t.Errorf("no K=14 from the manual's list-directed READ")
	}
}
