package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
)

// Phase 113's FORRTL round (docs/planning/PHASE-113 - Fortran errata.md, subtask
// 12b): round 3 of testdata/for81, FOR81G, asks VMS's FORRTL what Phases 80, 81,
// 88, and 112 left unconfirmed. It goes to the VAX on testdata/err113's volume.
// TestFor81GProcedure runs it on VMS's FORRTL.EXE over govax, as FOR81G.COM runs
// it, and holds it to VMS's run, testdata/for81/vax/for81g-vax.txt, which tests
// govax's RMS; TestFor81GOnFTNRTL holds FTNRTL to the same run (subtask 13e).

// runFor81G runs FOR81G with units -1 to -4's names defined as FOR81G.COM
// defines them, and returns its output and the files the procedure TYPEs after
// it. On FORRTL it runs on VMS's FORRTL.EXE over govax; otherwise on FTNRTL, its
// FOR$ calls passed on by testdata/for88's for88f.mar and for81gf.mar.
func runFor81G(t *testing.T, onFORRTL bool) (out string, files map[string]string) {
	t.Helper()

	var disk, file string
	if onFORRTL {
		disk, file = findFORRTL(t)
	}

	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for81g.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR81G`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("CREATE/DIRECTORY DUA1:[FOR81G]")
	dispatch("SET DEFAULT DUA1:[FOR81G]")

	if onFORRTL {
		dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
		dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)
	}

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

	sources := map[string]string{"FOR81G": dir, "FOR81T": dir}
	if !onFORRTL {
		sources = map[string]string{"FOR81G": dir, "FOR81GF": dir, "FOR88F": for88Dir}
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

	// FOR81T is run by hand at a terminal: here it is only built.
	links := []string{"FOR81G,FOR81/OPTIONS", "FOR81T,FOR81/OPTIONS"}
	if !onFORRTL {
		links = []string{"FOR81G,FOR81GF,FOR88F"}
	}

	for _, link := range links {
		if out := dispatch("LINK/NOMAP " + link); out != "" {
			t.Fatalf("LINK %s: %s", link, out)
		}
	}

	dispatch("DEFINE/NOLOG FOR$PRINT PRT.DAT")
	dispatch("DEFINE/NOLOG FOR$TYPE TYP.DAT")
	dispatch("DEFINE/NOLOG FOR$ACCEPT ACC.DAT")
	dispatch("DEFINE/NOLOG FOR$READ RDF.DAT")

	// FOR81G_DEBUG traces the run's RMS calls (SET DEBUG RMS).
	if os.Getenv("FOR81G_DEBUG") != "" {
		dispatch("SET DEBUG RMS")
	}

	out = dispatch("RUN FOR81G")
	files = map[string]string{}

	for _, name := range []string{"PRT.DAT", "TYP.DAT", "FOR013.DAT"} {
		files[name] = dispatch("TYPE " + name)
	}

	return out, files
}

func TestFor81GProcedure(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	out, files := runFor81G(t, true)

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

	compareFor81G(t, cases, string(text), for81GDifferences, 64)
}

// TestFor81GOnFTNRTL runs round 3's cases on FTNRTL, by for88f.mar's and
// for81gf.mar's FOR$ names, and holds each to VMS's run of them on FORRTL.
func TestFor81GOnFTNRTL(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	out, _ := runFor81G(t, false)

	if path := os.Getenv("FOR81G_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81g-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	compareFor81G(t, for88Cases(out), string(text), for81GFTNRTLDifferences, 64)
}

// for81GDifferences are the cases whose output on VMS's FORRTL over govax differs
// from the VAX's, with why.
var for81GDifferences = map[string]string{}

// for81GFTNRTLDifferences are the cases whose output on FTNRTL differs from VMS's
// on FORRTL, with why.
var for81GFTNRTLDifferences = map[string]string{}

// compareFor81G holds each case of VMS's run, text, of count cases, to got's,
// masked by for81GMasked, but for those differences lists.
func compareFor81G(t *testing.T, got map[string]string, text string, differences map[string]string, count int) {
	t.Helper()

	want := for88Cases(text)
	if len(want) != count {
		t.Errorf("%d cases in VMS's run, where gen.go wrote %d", len(want), count)
	}

	for title, w := range want {
		g, ok := got[title]
		_, expected := differences[title]
		w, g = for81GMasked(w), for81GMasked(g)

		switch {
		case !ok:
			t.Errorf("%s: not run", title)
		case g != w && !expected:
			t.Errorf("%s:\nVMS:\n%s\ngovax:\n%s", title, w, g)
		case g == w && expected:
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from the differences", title)
		}
	}
}

// for81GMasked is a case's output with each " pc b= i= e=" line's numbers
// taken out: they are the condition's PC, in VMS's FORRTL or govax's FTNRTL,
// wherever that was loaded, less places in the program. The user PC's line
// ("user pc") is the program's own, and is compared.
func for81GMasked(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, " pc b=") {
			lines[i] = " pc b=_ i=_ e=_"
		}
	}

	return strings.Join(lines, "\n")
}
