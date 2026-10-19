package console_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/machine"
)

// Phase 66's probe (testdata/pg66, README.md): the program's parts, run
// on govax before VMS's run comes back.

// pg66Dir is the probe's directory.
var pg66Dir = filepath.Join("..", "..", "testdata", "pg66")

// pg66Console is a console with paging on, the probe's program built
// (with govax's MACRO and LINK/NOTRACEBACK, as PG66.EXE) on an RD54
// volume on DUA1, which holds the page file too and is the default
// directory. PFCDEFAULT is 16, as on the VAX Phase 65's probe ran on.
func pg66Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "20000")
	setSetting(t, "vax.sysgen.pfcdefault", "16")

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "pg66.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PG66`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)

	src, err := filepath.Abs(filepath.Join(pg66Dir, "pg66.mar"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST DUA1:[000000]PG66.MAR`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, cmd := range []string{"MACRO PG66", "LINK/NOTRACEBACK PG66"} {
		if out := dispatch(cmd); out != "" {
			t.Fatalf("%s: %s", cmd, out)
		}
	}

	return c, dispatch
}

// pg66RunA is PG66.COM's RUNA: part A from image, with the counts before
// RUN in the logical names the program reads; it returns the program's
// lines and the procedure's line for the counts across the RUN.
func pg66RunA(t *testing.T, dispatch func(string) string, image, id string) []string {
	t.Helper()

	dispatch("DEFINE PG66_PART A")
	dispatch("DEFINE PG66_IMAGE " + image + ".EXE")
	dispatch(`f0 = F$GETJPI("","PAGEFLTS")`)
	dispatch(`c0 = F$GETJPI("","PAGFILCNT")`)
	dispatch(`g0 = F$GETJPI("","PPGCNT")`)
	dispatch("DEFINE PG66_FLTS 'f0'")
	dispatch("DEFINE PG66_CNT 'c0'")
	dispatch("DEFINE PG66_PPG 'g0'")

	lines := strings.Split(dispatch("RUN "+image), "|")

	dispatch(`s = $STATUS`)
	dispatch(`f1 = F$GETJPI("","PAGEFLTS")`)
	dispatch(`c1 = F$GETJPI("","PAGFILCNT")`)
	dispatch(`g1 = F$GETJPI("","PPGCNT")`)

	return append(lines, dispatch(`WRITE SYS$OUTPUT "`+id+` `+image+
		` around RUN: status=", s, " flts=", f1 - f0, " charged=", c0 - c1, " ppg=", g1 - g0`))
}

// pg66Check fails the test for a line of part id that says a page held
// the wrong value, or a status that isn't want's.
func pg66Check(t *testing.T, id string, lines []string, want map[string]string) {
	t.Helper()

	for _, line := range lines {
		if strings.Contains(line, "bad=") && !strings.HasSuffix(line, "bad=0") {
			t.Errorf("%s: %s", id, line)
		}

		if strings.Contains(line, "zero=") && !strings.HasSuffix(line, "zero=20") {
			t.Errorf("%s: %s", id, line)
		}
	}

	for prefix, value := range want {
		found := false

		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				found = true

				if !strings.Contains(line, value) {
					t.Errorf("%s: %q, want %q in it", id, line, value)
				}
			}
		}

		if !found {
			t.Errorf("%s: no line %q\n%s", id, prefix, strings.Join(lines, "\n"))
		}
	}
}

// TestPG66DryRun runs parts A (twice), W, and D as PG66.COM runs them,
// before VMS's run comes back, and checks what doesn't depend on VMS's
// answers: every page read holds its value, demand-zero pages are zero,
// the image's charges come back at its exit, its file may be opened to
// be read but not otherwise while it runs (RMS$_FLK; question 2), and a
// copy of the image deleted while it runs still reads its pages.
func TestPG66DryRun(t *testing.T) {
	_, dispatch := pg66Console(t)

	for _, id := range []string{"R1", "R2"} {
		lines := pg66RunA(t, dispatch, "PG66", id)
		pg66Check(t, id, lines, map[string]string{
			"A9 open to read, others read:":  "00010001",
			"A9 open to read, no sharing:":   "0001828A",
			"A9 open to write, others read:": "0001828A",
			"A11 the whole image:":           "charged=0",
			id + " PG66 around RUN:":         "status=%X00000001",
		})

		if id == "R2" {
			pg66Check(t, id, lines, map[string]string{id + " PG66 around RUN:": "charged=0 "})
		}

		t.Logf("%s:\n%s", id, strings.Join(lines, "\n"))
	}

	dispatch("DEFINE PG66_PART W")
	lines := strings.Split(dispatch("RUN PG66"), "|")
	pg66Check(t, "W", lines, map[string]string{"W3 and a third time: bad=": "bad=0"})
	t.Logf("W:\n%s", strings.Join(lines, "\n"))

	dispatch("COPY PG66.EXE PG66D.EXE")
	dispatch("DEFINE PG66_PART D")
	dispatch("SPAWN/NOWAIT/PROCESS=PG66DEL/OUTPUT=PG66D.OUT RUN PG66D")

	await := func(file string) {
		t.Helper()

		for range 60 {
			if dispatch(`WRITE SYS$OUTPUT F$SEARCH("`+file+`")`) != "" {
				return
			}

			dispatch("WAIT 00:00:01")
		}

		t.Fatalf("the image never made %s", file)
	}

	await("PG66D.RDY")

	if out := dispatch("DELETE PG66D.EXE;*"); out != "" {
		t.Errorf("DELETE of the running image's file: %s", out)
	}

	if out := dispatch(`WRITE SYS$OUTPUT F$SEARCH("PG66D.EXE")`); out != "" {
		t.Errorf("after DELETE, F$SEARCH finds %s", out)
	}

	await("PG66D.END")
	dispatch("WAIT 00:00:02")

	lines = strings.Split(dispatch("TYPE PG66D.OUT"), "|")
	pg66Check(t, "D", lines, map[string]string{
		"D1 PG66D.RDY made:":          "00010001",
		"D2 BIG read after the wait: bad=": "bad=0",
		"D3 PG66D.END made:":          "00010001",
	})
	t.Logf("D:\n%s", strings.Join(lines, "\n"))
}
