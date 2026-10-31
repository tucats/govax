package console_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/rms"
)

// testdata/nml80 is Phase 80's probe of NAMELIST I/O (subtask 11): one table
// of cases is nml80.mar, which calls VMS's FORRTL.EXE, and nml80f.for, the
// same cases as FORTRAN statements, for FTNRTL; both print the same lines.
// vax/nml80-forrtl.txt is FORRTL's run over govax, FTNRTL's requirement, as
// round 3 of testdata/for80 is; TestNML80RunsOnFORRTL makes it again.
var nml80Dir = filepath.Join("..", "..", "testdata", "nml80")

// nml80Volume makes a fresh volume as DUA1, the default directory, with the
// named files of testdata/nml80 on it.
func nml80Volume(t *testing.T, files ...string) (*console.Console, func(string) string) {
	t.Helper()

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "nml80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 NML80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range files {
		path, err := filepath.Abs(filepath.Join(nml80Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + path + `"/HOST ` + strings.ToUpper(name))
	}

	return c, dispatch
}

// nml80Build runs each command, which must say nothing.
func nml80Build(t *testing.T, dispatch func(string) string, commands ...string) {
	t.Helper()

	for _, command := range commands {
		if out := dispatch(command); out != "" {
			t.Fatalf("%s said: %s", command, strings.ReplaceAll(out, "|", "\n"))
		}
	}
}

// nml80Cases runs the probe's procedure with image, its output going to a
// file, and returns that file's records, a line each.
func nml80Cases(t *testing.T, c *console.Console, dispatch func(string) string, image string) string {
	t.Helper()

	dispatch("DEFINE/USER_MODE SYS$OUTPUT NML80.TXT")

	if out := dispatch("@NML80R " + image); out != "" {
		t.Errorf("@NML80R %s said: %s", image, strings.ReplaceAll(out, "|", "\n"))
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]NML80.TXT"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var run bytes.Buffer

	for _, record := range records {
		run.Write(record)
		run.WriteByte('\n')
	}

	return run.String()
}

// nml80PromptInput is what a terminal user types at NML80E's READ: each form
// of question in the search for the group and in it, then an assignment.
const nml80PromptInput = " ?\n =?\n $GRP ?\n =?\n I=7 $END\n"

// TestNML80Oracle holds FTNRTL's namelist input and output to FORRTL's: each
// case's lines, the condition a READ ended with, and what it stored, written
// back out as namelist output, the same as vax/nml80-forrtl.txt's.
func TestNML80Oracle(t *testing.T) {
	c, dispatch := nml80Volume(t, "nml80f.for", "nml80r.com")
	nml80Build(t, dispatch, "FORTRAN NML80F", "MACRO NML80F", "LINK NML80F")

	got := nml80Cases(t, c, dispatch, "NML80F")

	want, err := os.ReadFile(filepath.Join(nml80Dir, "vax", "nml80-forrtl.txt"))
	if err != nil {
		t.Fatal(err)
	}

	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")

	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}

		if i < len(wantLines) {
			w = wantLines[i]
		}

		if g != w {
			t.Fatalf("line %d differs from FORRTL's:\n forrtl %q\n ftnrtl %q", i+1, w, g)
		}
	}
}

// nml80Message matches the lines TestNML80Messages compares: a run's number,
// its message's first line, and its $STATUS. The lines after the first (the
// unit and file, the user PC, a secondary message) are the error model's,
// Phase 80's subtask 12.
var nml80Message = regexp.MustCompile(`^(%\d+|%FOR-.*|\d+)$`)

// nml80Messages is the compared lines of a run of NML80E.COM.
func nml80Messages(out string) []string {
	var lines []string

	for line := range strings.SplitSeq(out, "|") {
		if nml80Message.MatchString(line) {
			lines = append(lines, line)
		}
	}

	return lines
}

// TestNML80Messages runs NML80E.FOR, a READ with no handler, on each record
// set of nml80e.com, and holds its messages and statuses to FORRTL's, run as
// nml80e.mar: SYNERRNAM with the text it stopped at, TOOMANVAL with the
// entity's name, INVREFVAR with the name typed or the entity's.
func TestNML80Messages(t *testing.T) {
	_, dispatch := nml80Volume(t, "nml80e.for", "nml80e.com")
	nml80Build(t, dispatch, "FORTRAN NML80E", "MACRO NML80E", "LINK NML80E")

	got := nml80Messages(dispatch("@NML80E NML80E"))

	data, err := os.ReadFile(filepath.Join(nml80Dir, "vax", "nml80e-forrtl.txt"))
	if err != nil {
		t.Fatal(err)
	}

	want := nml80Messages(strings.ReplaceAll(strings.TrimSuffix(string(data), "\n"), "\n", "|"))

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("messages differ from FORRTL's:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestNML80Prompt answers ? and =? at a terminal, as FORRTL does
// (vax/nml80t-forrtl.txt): the group's names, or the group as a WRITE writes
// it, in the search for the group and in it.
func TestNML80Prompt(t *testing.T) {
	c, dispatch := nml80Volume(t, "nml80e.for")
	nml80Build(t, dispatch, "FORTRAN NML80E", "MACRO NML80E", "LINK NML80E")

	c.In = strings.NewReader(nml80PromptInput)
	got := strings.ReplaceAll(dispatch("RUN NML80E"), "|", "\n") + "\n"

	want, err := os.ReadFile(filepath.Join(nml80Dir, "vax", "nml80t-forrtl.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if got != string(want) {
		t.Errorf("the answers differ from FORRTL's:\n%q\nwant\n%q", got, want)
	}
}

// TestNML80RunsOnFORRTL runs the probe's MACRO programs on VMS's FORRTL.EXE
// over govax, from a local-only volume, and holds them to the files the other
// tests read; with NML80_WRITE set it writes those files instead.
func TestNML80RunsOnFORRTL(t *testing.T) {
	disk, file := findFORRTL(t)
	c, dispatch := nml80Volume(t, "nml80.mar", "nml80r.com", "nml80e.mar", "nml80e.com")

	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	opt := filepath.Join(t.TempDir(), "nml80.opt")
	if err := os.WriteFile(opt, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + opt + `"/HOST NML80.OPT`)
	nml80Build(t, dispatch, "MACRO/NOLIST NML80", "LINK/NOMAP NML80,NML80/OPTIONS",
		"MACRO/NOLIST NML80E", "LINK/NOMAP NML80E,NML80/OPTIONS")

	runs := map[string]string{
		"nml80-forrtl.txt":  nml80Cases(t, c, dispatch, "NML80"),
		"nml80e-forrtl.txt": strings.ReplaceAll(dispatch("@NML80E NML80E"), "|", "\n") + "\n",
	}

	c.In = strings.NewReader(nml80PromptInput)
	runs["nml80t-forrtl.txt"] = strings.ReplaceAll(dispatch("RUN NML80E"), "|", "\n") + "\n"

	for name, got := range runs {
		path := filepath.Join(nml80Dir, "vax", name)

		if os.Getenv("NML80_WRITE") != "" {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		if got != string(want) {
			t.Errorf("FORRTL's run differs from %s", path)
		}
	}
}
