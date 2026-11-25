package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Round 3b of testdata/for81 (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 13e2): FOR81K asks VMS's FORRTL what round 3 left open.
// TestFor81KProcedure runs it on VMS's FORRTL.EXE over govax, which holds
// govax's RMS and OTS$ to VMS 7.1's run, vax/for81k-vax.txt, and
// TestFor81KOnFTNRTL on FTNRTL, which holds FTNRTL to it.

// for81KDifferences are the cases whose output on FORRTL over govax differs
// from VMS's, with why.
var for81KDifferences = map[string]string{}

// for81KFTNRTLDifferences are the cases whose output on FTNRTL differs from
// VMS's, with why.
var for81KFTNRTLDifferences = map[string]string{
	"#19 ENDDURREA at the second item: (A4,/,A4) of a file of one record": "the user PC is VMS's, the " +
		"first item's call; but VMS's FORRTL goes on through the format to the next data descriptor " +
		"during an item's call, and signals there, where FTNRTL walks there at the next item's call " +
		"and signals at FOR$IO_END, so i= and e= differ",
}

// runFor81K builds and runs FOR81K in [FOR81K] on a fresh volume, on FORRTL if
// onFORRTL, else on FTNRTL through testdata/for88's for88f.mar and
// for81gf.mar, and returns its output.
func runFor81K(t *testing.T, onFORRTL bool) string {
	t.Helper()

	return runFor81Program(t, "FOR81K", onFORRTL)
}

// runFor81Program builds and runs one of round 3b's programs (FOR81K, or
// round 2's FOR81M) in [FOR81K] on a fresh volume, as runFor81K says, and
// returns its output.
func runFor81Program(t *testing.T, program string, onFORRTL bool) string {
	t.Helper()

	var disk, file string
	if onFORRTL {
		disk, file = findFORRTL(t)
	}

	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for81k.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR81K`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("CREATE/DIRECTORY DUA1:[FOR81K]")
	dispatch("SET DEFAULT DUA1:[FOR81K]")

	if onFORRTL {
		dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
		dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)
	}

	dir := filepath.Join("..", "..", "testdata", "for81")

	host := filepath.Join(t.TempDir(), "for81.opt")
	if err := os.WriteFile(host, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + host + `"/HOST FOR81.OPT`)

	sources := map[string]string{program: dir, "FOR81W": dir}
	links := []string{program + ",FOR81/OPTIONS", "FOR81W,FOR81/OPTIONS"}

	if !onFORRTL {
		sources = map[string]string{program: dir, "FOR81GF": dir, "FOR88F": for88Dir}
		links = []string{program + ",FOR81GF,FOR88F"}
	}

	for name, from := range sources {
		src, err := filepath.Abs(filepath.Join(from, strings.ToLower(name)+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + name + `.MAR`)

		if out := dispatch("MACRO/NOLIST " + name); out != "" {
			t.Fatalf("MACRO %s: %s", name, out)
		}
	}

	// FOR81W is run by hand at a terminal: here it is only built.
	for _, link := range links {
		if out := dispatch("LINK/NOMAP " + link); out != "" {
			t.Fatalf("LINK %s: %s", link, out)
		}
	}

	return dispatch("RUN " + program)
}

// checkFor81K checks a run's output and holds it to VMS's run but for
// differences.
func checkFor81K(t *testing.T, out string, differences map[string]string) {
	t.Helper()

	if path := os.Getenv("FOR81K_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(out, "##### the end of the cases") {
		t.Fatalf("the program stopped before its end; its last lines:\n%s", lastLines(out, 30))
	}

	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81k-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	compareFor81G(t, for88Cases(out), string(text), differences, 28)
}

func TestFor81KProcedure(t *testing.T) { checkFor81K(t, runFor81K(t, true), for81KDifferences) }

func TestFor81KOnFTNRTL(t *testing.T) { checkFor81K(t, runFor81K(t, false), for81KFTNRTLDifferences) }

// FOR81M (Phase 113's probe round 2, testdata/err113's README) asks the
// associated variable after FIND, WRITE, or READ, then DELETE by number.
// TestFor81MOnFTNRTL holds FTNRTL to VMS 7.1's run, vax/for81m-vax.txt, and
// TestFor81MProcedure VMS's FORRTL over govax (or skips without FORRTL.EXE on a
// local volume).
func TestFor81MOnFTNRTL(t *testing.T) { checkFor81M(t, runFor81Program(t, "FOR81M", false)) }

func TestFor81MProcedure(t *testing.T) { checkFor81M(t, runFor81Program(t, "FOR81M", true)) }

func checkFor81M(t *testing.T, out string) {
	t.Helper()

	if !strings.Contains(out, "##### the end of the cases") {
		t.Fatalf("the program stopped before its end; its last lines:\n%s", lastLines(out, 30))
	}

	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81m-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	compareFor81G(t, for88Cases(out), string(want), map[string]string{}, 9)
}
