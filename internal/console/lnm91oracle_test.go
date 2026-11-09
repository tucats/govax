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
// vax/lnm91b.log are VMS 7.1's logs of it. TestLnm91ServicesOracle replays
// each log's cases on govax with VMS's own image, on a volume built
// by the same commands, and compares what $PARSE reported, line by line:
// the status, the expanded string, the FNB, the component lengths, and the
// DVI, DID, and FID, for $PARSE, $SEARCH, $OPEN, $CREATE, $ERASE, and
// $RENAME (subtask 5).
//
// A procedure is made from the log itself, where SET VERIFY shows every
// command with its symbols replaced, keeping the commands that make the
// tree and name the logical names, and the cases.

// lnm91Dir is the probe's directory, from this package's.
var lnm91Dir = filepath.Join("..", "..", "testdata", "lnm91")

// lnm91Sections are the sections of each log with service cases to replay.
// The others are the DCL utilities and devices, later subtasks'.
var lnm91Sections = map[string][]string{
	"lnm91.log": {
		"GUIDE TABLE, DEFAULT [JONES] ON THE PLAIN DEVICE", "FORMS OF THE NAME", "CHAINS AND LISTS",
		"SYNTAX", "ROOTED DEFAULT NAME AND RELATED FILE", "SYS$DISK ROOTED", "DEPTH", "FILES",
	},
	"lnm91b.log": {
		"PLAIN NAMES WITH A DIRECTORY", "CONCEALED NAMES WITH A DIRECTORY", "SYS$DISK WITH A DIRECTORY",
		"ANGLE BRACKETS",
	},
}

// lnm91Case is one of a log's cases: the data line, and every line of its
// report.
type lnm91Case struct {
	line   string
	report []string
}

// lnm91ServiceLog walks a log, returning the procedure that replays it and
// its cases in order.
func lnm91ServiceLog(t *testing.T, name, image string) (string, []lnm91Case) {
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
			case strings.HasPrefix(l, "$ CREATE/DIRECTORY"), strings.HasPrefix(l, "$ COPY T.DAT"),
				strings.HasPrefix(l, "$ COPY LNM91.EXE"):
				proc = append(proc, l)
			case l == "$ CREATE T.DAT":
				proc = append(proc, l, lines[i+1])
			case strings.HasPrefix(l, "$ MACRO"):
				// VMS's MACRO left an object file; this stands in for it, so
				// that later file numbers come out the same.
				proc = append(proc, "$ COPY T.DAT DUA1:[000000]LNM91.OBJ",
					fmt.Sprintf(`$ COPY "%s"/HOST DUA1:[000000]LNM91.EXE`, image))
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

		case strings.HasPrefix(l, "$ SET DEFAULT"), strings.HasPrefix(l, "$ DEFINE"), strings.HasPrefix(l, "$ DEASSIGN"),
			strings.HasPrefix(l, "$ CREATE/DIRECTORY"):
			endRun()

			proc = append(proc, l)

		case inRun && strings.HasPrefix(l, "== "):
			if strings.ContainsRune("PSOCERF", rune(l[3])) {
				proc = append(proc, l[3:])
				cases = append(cases, lnm91Case{line: l[3:], report: lnm91Report(lines[i+1:])})
			}
		}
	}

	endRun()

	return strings.Join(proc, "\n") + "\n", cases
}

// lnm91Channel is what a successful $OPEN or $CREATE leaves in VMS's FAB's
// STV: RMS's own channel, which govax's RMS doesn't have.
const lnm91Channel = "stv=00000080"

// lnm91Report is the report at the start of lines: its indented lines.
func lnm91Report(lines []string) []string {
	n := 0
	for n < len(lines) && strings.HasPrefix(lines[n], "  ") {
		n++
	}

	return lines[:n]
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

// TestLnm91ServicesOracle holds RMS's services to VMS's answers in both logs.
func TestLnm91ServicesOracle(t *testing.T) {
	image, err := filepath.Abs(filepath.Join(lnm91Dir, "vax", "lnm91.exe"))
	if err != nil {
		t.Fatal(err)
	}

	for name, label := range map[string]string{"lnm91.log": "LNM91", "lnm91b.log": "LNM91B"} {
		t.Run(name, func(t *testing.T) {
			proc, cases := lnm91ServiceLog(t, name, image)

			out := strings.Split(strings.ReplaceAll(lnm91Run(t, label, proc), "\r", ""), "\n")

			var got [][]string

			for i, l := range out {
				if strings.HasPrefix(l, "== ") {
					got = append(got, lnm91Report(out[i+1:]))
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
					have := "(no line)"
					if j < len(got[i]) {
						have = got[i][j]
					}

					want = strings.Replace(want, lnm91Channel, "stv=00000000", 1)

					if have != want {
						bad++

						t.Errorf("%s\n  want %s\n  got  %s", c.line, want, have)
					}
				}

				if len(got[i]) > len(c.report) {
					bad++

					t.Errorf("%s\n  extra %s", c.line, got[i][len(c.report)])
				}
			}

			t.Logf("%d cases, %d lines differ", len(cases), bad)
		})
	}
}
