package console_test

import (
	"encoding/binary"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/machine"
)

// Phase 65's probe (testdata/pg65, README.md): the program's parts, run
// on govax before VMS's run comes back.

// pg65Dir is the probe's directory.
var pg65Dir = filepath.Join("..", "..", "testdata", "pg65")

// pg65Console is a console with paging on, the probe's program built on
// an RD54 volume on DUA1, which holds the page file too and is the
// default directory, and PG65.DAT made (part M).
func pg65Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "20000")

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "pg65.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PG65`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)

	src, err := filepath.Abs(filepath.Join(pg65Dir, "pg65.mar"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST DUA1:[000000]PG65.MAR`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, cmd := range []string{"MACRO PG65", "LINK PG65"} {
		if out := dispatch(cmd); out != "" {
			t.Fatalf("%s: %s", cmd, out)
		}
	}

	return c, dispatch
}

// TestPG65DryRun runs every part, the holder (part H) in a subprocess
// as PG65.COM runs it, and checks what doesn't wait for VMS's answers:
// every part writes lines, every page read back holds what it should
// ("bad=0"), and GBLPAGES' counting rule (a section's pages plus 2,
// rounded up to even). The lines are compared with VMS's once its run
// is back.
func TestPG65DryRun(t *testing.T) {
	_, dispatch := pg65Console(t)

	run := func(part string) []string {
		t.Helper()

		dispatch("DEFINE PG65_PART " + part)

		return strings.Split(dispatch("RUN PG65"), "|")
	}

	parts := map[string][]string{}

	for _, part := range []string{"M", "G", "F", "D", "R", "P", "C", "U", "K"} {
		parts[part] = run(part)
	}

	dispatch("DEFINE PG65_PART H")
	dispatch("SPAWN/NOWAIT/PROCESS=PG65HLD/OUTPUT=PG65H.OUT RUN PG65")
	parts["S"] = run("S")
	dispatch("WAIT 00:00:01")
	parts["H"] = strings.Split(dispatch("TYPE PG65H.OUT"), "|")

	t.Cleanup(func() {
		if t.Failed() {
			for _, part := range []string{"M", "G", "F", "D", "R", "P", "C", "U", "K", "S", "H"} {
				t.Logf("part %s:\n%s", part, strings.Join(parts[part], "\n"))
			}
		}
	})

	for part, lines := range parts {
		if !strings.HasPrefix(lines[0], part) {
			t.Errorf("part %s: %q", part, lines)
		}

		for _, line := range lines {
			if i := strings.Index(line, "bad="); i >= 0 && !strings.HasPrefix(line[i:], "bad=0") {
				t.Errorf("part %s: %s", part, line)
			}

			if strings.HasPrefix(line, "?") || strings.HasPrefix(line, "%") {
				t.Errorf("part %s: %s", part, line)
			}
		}
	}

	var made []string

	for _, line := range parts["G"] {
		if strings.HasPrefix(line, "G2 made:") || strings.HasPrefix(line, "G3 made:") {
			made = append(made, line)
		}
	}

	want := []string{
		"G2 made: sects=1 gblpages=4 contig=4", "G2 made: sects=1 gblpages=4 contig=4",
		"G2 made: sects=1 gblpages=6 contig=6", "G2 made: sects=1 gblpages=6 contig=6",
		"G3 made: sects=1 gblpages=6 contig=6",
	}

	if strings.Join(made, "|") != strings.Join(want, "|") {
		t.Errorf("GBLPAGES' counting:\n%s\nwant\n%s", strings.Join(made, "\n"), strings.Join(want, "\n"))
	}

	if h := parts["H"]; len(h) != 4 || h[3] != "H3 flag 67: 1" {
		t.Errorf("the holder: %q", h)
	}
}

// TestGetSYIGlobalSectionItems: F$GETSYI's global section items, and
// PFCDEFAULT, are $GETSYI's (the probe's procedure asks for them).
func TestGetSYIGlobalSectionItems(t *testing.T) {
	c, dispatch := wsDispatcher(t)

	for _, key := range []string{
		"PFCDEFAULT", "GBLSECTIONS", "GBLPAGES", "GBLPAGFIL",
		"FREE_GBLSECTS", "FREE_GBLPAGES", "CONTIG_GBLPAGES",
	} {
		data, ok := c.ProcessOne().SYIItem("SYI$_" + key)
		if !ok || len(data) != 4 {
			t.Fatalf("$GETSYI %s: % x (%v)", key, data, ok)
		}

		want := strconv.Itoa(int(binary.LittleEndian.Uint32([]byte(data))))
		if got := dispatch(`WRITE SYS$OUTPUT F$GETSYI("` + key + `")`); got != want {
			t.Errorf("F$GETSYI(%q) = %q, want %s", key, got, want)
		}
	}
}
