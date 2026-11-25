package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Round 3b of testdata/for81 (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 13e's follow-up): FOR81K asks VMS's FORRTL what round 3 left open.
// TestFor81KProcedure rehearses it on VMS's FORRTL.EXE over govax, and
// TestFor81KOnFTNRTL on FTNRTL, so a mistake in the program shows before the
// VAX's run; each logs every case whose output differs from VMS's once
// vax/for81k-vax.txt is here.

// runFor81K builds and runs FOR81K in [FOR81K] on a fresh volume, on FORRTL if
// onFORRTL, else on FTNRTL through testdata/for88's for88f.mar and
// for81gf.mar, and returns its output.
func runFor81K(t *testing.T, onFORRTL bool) string {
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

	sources := map[string]string{"FOR81K": dir, "FOR81W": dir}
	links := []string{"FOR81K,FOR81/OPTIONS", "FOR81W,FOR81/OPTIONS"}

	if !onFORRTL {
		sources = map[string]string{"FOR81K": dir, "FOR81GF": dir, "FOR88F": for88Dir}
		links = []string{"FOR81K,FOR81GF,FOR88F"}
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

	return dispatch("RUN FOR81K")
}

// checkFor81K checks a rehearsal's output and logs how it differs from VMS's run,
// if that is here.
func checkFor81K(t *testing.T, out string) {
	t.Helper()

	if path := os.Getenv("FOR81K_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(out, "##### the end of the cases") {
		t.Fatalf("the program stopped before its end; its last lines:\n%s", lastLines(out, 30))
	}

	cases := for88Cases(out)
	if len(cases) != 28 {
		t.Errorf("%d cases in govax's run, where gen.go wrote 28", len(cases))
	}

	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81k-vax.txt"))
	if os.IsNotExist(err) {
		t.Log("no VMS run yet (testdata/for81/vax/for81k-vax.txt): the rehearsal only")

		return
	} else if err != nil {
		t.Fatal(err)
	}

	for title, w := range for88Cases(string(text)) {
		w = for81GMasked(w)
		if g, ok := cases[title]; !ok {
			t.Logf("%s: not run", title)
		} else if g = for81GMasked(g); g != w {
			t.Logf("%s:\nVMS:\n%s\ngovax:\n%s", title, w, g)
		}
	}
}

func TestFor81KProcedure(t *testing.T) { checkFor81K(t, runFor81K(t, true)) }

func TestFor81KOnFTNRTL(t *testing.T) { checkFor81K(t, runFor81K(t, false)) }
