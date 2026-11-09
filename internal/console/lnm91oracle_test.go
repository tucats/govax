package console_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// The Phase 91 oracle (docs/PHASE-91, subtask 4): testdata/lnm91's program
// reports what RMS did with a name, case by case, and vax/lnm91.log and
// vax/lnm91b.log are VMS 7.1's logs of it. TestLnm91ParseOracle replays
// each log's $PARSE cases on govax with VMS's own image, on a volume built
// by the same commands, and compares what $PARSE reported, line by line:
// the status, the expanded string, the FNB, the component lengths, and the
// DVI, DID, and FID.
//
// A procedure is made from the log itself, where SET VERIFY shows every
// command with its symbols replaced, keeping the commands that make the
// tree and name the logical names, and the cases. A case that also
// searches ("S") runs here as a $PARSE alone.

// lnm91Dir is the probe's directory, from this package's.
var lnm91Dir = filepath.Join("..", "..", "testdata", "lnm91")

// lnm91Sections are the sections of each log with $PARSE cases to replay.
// The others are the DCL utilities, files, and devices, later subtasks'.
var lnm91Sections = map[string][]string{
	"lnm91.log": {
		"GUIDE TABLE, DEFAULT [JONES] ON THE PLAIN DEVICE", "FORMS OF THE NAME", "CHAINS AND LISTS",
		"SYNTAX", "ROOTED DEFAULT NAME AND RELATED FILE", "SYS$DISK ROOTED", "DEPTH",
	},
	"lnm91b.log": {
		"PLAIN NAMES WITH A DIRECTORY", "CONCEALED NAMES WITH A DIRECTORY", "SYS$DISK WITH A DIRECTORY",
		"ANGLE BRACKETS",
	},
}

// lnm91Case is one of a log's cases: the data line, and the seven lines of
// its $PARSE report.
type lnm91Case struct {
	line   string
	report []string
}

// lnm91ParseLog walks a log, returning the procedure that replays it and
// its cases in order.
func lnm91ParseLog(t *testing.T, name, image string) (string, []lnm91Case) {
	t.Helper()

	text, err := os.ReadFile(filepath.Join(lnm91Dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.ReplaceAll(string(text), "\r", ""), "\n")

	wanted := map[string]bool{}
	for _, s := range lnm91Sections[name] {
		wanted[s] = true
	}

	var (
		proc  []string
		cases []lnm91Case
		sect  string
	)

	proc = append(proc, "$ SET NOON")
	inRun := false

	// endRun closes a RUN's data lines.
	endRun := func() {
		if inRun {
			proc = append(proc, "$ !")

			inRun = false
		}
	}

	for i := 0; i < len(lines); i++ {
		l := lines[i]

		if rest, ok := strings.CutPrefix(l, "######## "); ok {
			endRun()

			sect = rest

			continue
		}

		switch sect {
		case "TREE":
			switch {
			case strings.HasPrefix(l, "$ CREATE/DIRECTORY"), strings.HasPrefix(l, "$ COPY T.DAT"):
				proc = append(proc, l)
			case l == "$ CREATE T.DAT":
				proc = append(proc, l, lines[i+1])
			case strings.HasPrefix(l, "$ MACRO"):
				proc = append(proc, fmt.Sprintf(`$ COPY "%s"/HOST DUA1:[000000]LNM91.EXE`, image))
			}

		case "DEFINITIONS":
			if strings.HasPrefix(l, "$ DEFINE") {
				proc = append(proc, l)
			}
		}

		if !wanted[sect] {
			continue
		}

		switch {
		case strings.HasPrefix(l, "$ RUN "):
			proc = append(proc, l)
			inRun = true

		case strings.HasPrefix(l, "$ SET DEFAULT"), strings.HasPrefix(l, "$ DEFINE"), strings.HasPrefix(l, "$ DEASSIGN"):
			endRun()

			proc = append(proc, l)

		case inRun && strings.HasPrefix(l, "== "):
			if strings.Contains(lines[i+1], "$PARSE ") && (l[3] == 'P' || l[3] == 'S') {
				data := "P" + l[4:]
				proc = append(proc, data)
				cases = append(cases, lnm91Case{line: data, report: lines[i+1 : i+8]})
			}
		}
	}

	endRun()

	return strings.Join(proc, "\n") + "\n", cases
}

// lnm91Run runs a procedure on a fresh RD51 as DUA1, the exchange volume's
// files on it first so that file numbers come out as VMS's did.
func lnm91Run(t *testing.T, label, proc string) string {
	t.Helper()

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	c.RTL.NodeName = "SIMVAX" // the VAX's, which NAM$T_DVI shows

	out := &bytes.Buffer{}
	c.Out = out

	dir := t.TempDir()
	disk := filepath.Join(dir, "lnm91.dsk")

	if err := c.Commands().InitializeContainer(disk, 0, label, 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Mount("DUA1", disk, true); err != nil {
		t.Fatal(err)
	}

	d := c.Dispatcher

	// The replay takes the procedure's place on the volume.
	replay := filepath.Join(dir, "lnm91.com")
	if err := os.WriteFile(replay, []byte(proc), 0o600); err != nil {
		t.Fatal(err)
	}

	source, err := filepath.Abs(filepath.Join(lnm91Dir, "lnm91.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{
		`COPY "` + source + `"/HOST DUA1:[000000]LNM91.MAR`,
		`COPY "` + replay + `"/HOST DUA1:[000000]LNM91.COM`,
		`COPY "` + replay + `"/HOST DUA1:[000000]LNM91.LOG`, // the procedure's /OUTPUT file
		"SET DEFAULT DUA1:[000000]",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatal(err)
		}
	}

	if err := d.Dispatch("@LNM91"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@LNM91: %v\n%s", err, out)
	}

	return out.String()
}

// TestLnm91ParseOracle holds $PARSE to VMS's answers in both logs.
func TestLnm91ParseOracle(t *testing.T) {
	image, err := filepath.Abs(filepath.Join(lnm91Dir, "vax", "lnm91.exe"))
	if err != nil {
		t.Fatal(err)
	}

	for name, label := range map[string]string{"lnm91.log": "LNM91", "lnm91b.log": "LNM91B"} {
		t.Run(name, func(t *testing.T) {
			proc, cases := lnm91ParseLog(t, name, image)

			out := strings.Split(strings.ReplaceAll(lnm91Run(t, label, proc), "\r", ""), "\n")

			var got [][]string

			for i, l := range out {
				if strings.HasPrefix(l, "== ") && i+7 < len(out) {
					got = append(got, out[i+1:i+8])
				}
			}

			if len(got) != len(cases) {
				t.Errorf("%d cases ran, the log has %d", len(got), len(cases))
			}

			bad := 0

			for i, c := range cases {
				if i >= len(got) {
					break
				}

				for j, want := range c.report {
					if got[i][j] != want {
						bad++

						t.Errorf("%s\n  want %s\n  got  %s", c.line, want, got[i][j])
					}
				}
			}

			t.Logf("%d cases, %d lines differ", len(cases), bad)
		})
	}
}
