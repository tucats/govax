package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 113's FORRTL round (docs/planning/PHASE-113 - Fortran errata.md, subtask
// 12b): round 3 of testdata/for81, FOR81G, asks VMS's FORRTL what Phases 80, 81,
// 88, and 112 left unconfirmed. It goes to the VAX on testdata/err113's volume.
// TestFor81GProcedure rehearses it on VMS's FORRTL.EXE over govax, as FOR81G.COM
// runs it, so a mistake in the program shows before the VAX's run; once VMS's
// output is in testdata/for81/vax, it logs each case that differs, for subtask 13
// to hold govax to.

// runFor81G runs FOR81G on FORRTL over govax with units -1 to -4's names
// defined as FOR81G.COM defines them, and returns its output and the files the
// procedure TYPEs after it.
func runFor81G(t *testing.T) (out string, files map[string]string) {
	t.Helper()

	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for81g.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR81G`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	dir := filepath.Join("..", "..", "testdata", "for81")
	host := t.TempDir()

	for name, text := range map[string]string{
		"FOR81.OPT": "FORRTL/SHAREABLE\n",
		"ACC.DAT":   "FROM FOR$ACCEPT\n",
		"RDF.DAT":   "FROM FOR$READ\n",
	} {
		path := filepath.Join(host, name)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + path + `"/HOST ` + name)
	}

	// FOR81T is run by hand at a terminal: here it is only built.
	for _, name := range []string{"FOR81G", "FOR81T"} {
		src, err := filepath.Abs(filepath.Join(dir, strings.ToLower(name)+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + name + `.MAR`)

		if out := dispatch("MACRO/NOLIST " + name); out != "" {
			t.Fatalf("MACRO %s: %s", name, out)
		}

		if out := dispatch("LINK/NOMAP " + name + ",FOR81/OPTIONS"); out != "" {
			t.Fatalf("LINK %s: %s", name, out)
		}
	}

	dispatch("DEFINE/NOLOG FOR$PRINT PRT.DAT")
	dispatch("DEFINE/NOLOG FOR$TYPE TYP.DAT")
	dispatch("DEFINE/NOLOG FOR$ACCEPT ACC.DAT")
	dispatch("DEFINE/NOLOG FOR$READ RDF.DAT")

	out = dispatch("RUN FOR81G")
	files = map[string]string{}

	for _, name := range []string{"PRT.DAT", "TYP.DAT", "FOR013.DAT"} {
		files[name] = dispatch("TYPE " + name)
	}

	return out, files
}

func TestFor81GProcedure(t *testing.T) {
	out, files := runFor81G(t)

	if path := os.Getenv("FOR81G_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(out, "##### the end of the cases") {
		t.Fatalf("the program stopped before its end; its last lines:\n%s", lastLines(out, 30))
	}

	cases := for88Cases(out)
	if len(cases) < 60 {
		t.Errorf("%d cases in govax's run", len(cases))
	}

	// What the rehearsal must show: each section reached, unit 6's records among
	// the cases' lines, and ERRSNS called.
	for _, want := range []string{"#0 make the relative files", "=c ", " errsns n=", " pc b=", "I0 0 |", " rms $GET RC1.DAT key -1"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the output", want)
		}
	}

	for name, text := range files {
		t.Logf("TYPE %s:\n%s", name, text)
	}

	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81g-vax.txt"))
	if os.IsNotExist(err) {
		t.Log("no VMS run yet (testdata/for81/vax/for81g-vax.txt): the rehearsal only")

		return
	} else if err != nil {
		t.Fatal(err)
	}

	want := for88Cases(string(text))
	for title, w := range want {
		if g, ok := cases[title]; !ok {
			t.Logf("%s: not run on govax", title)
		} else if g != w {
			t.Logf("%s:\nVMS:\n%s\ngovax:\n%s", title, w, g)
		}
	}
}
