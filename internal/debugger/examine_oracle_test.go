package debugger_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// oracleRange is one EXAMINE/INSTRUCTION command in a debugger session
// log, with the session's mode and radix when it was given, and the
// lines the debugger printed for it.
type oracleRange struct {
	command  string // the range, as typed: "START:LAST", "%LINE 85"
	symbolic bool
	radix    int
	lines    []string
}

// oracleRanges reads every EXAMINE/INSTRUCTION in the session log name
// (testdata/dbg/vax), following SET MODE and SET RADIX. A range at .PC
// is left out, since it depends on where the program had stopped.
func oracleRanges(t *testing.T, name string) []oracleRange {
	t.Helper()

	data, err := os.ReadFile(dbgImagePath(t, name))
	if err != nil {
		t.Fatal(err)
	}

	var (
		out      []oracleRange
		cur      *oracleRange
		symbolic = true
		radix    = 16
	)

	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		line, ok := strings.CutPrefix(l, "!")
		if !ok {
			continue
		}

		// A command is echoed after one space; output starts in the first
		// column, or (a CASE table's entries) after 16 spaces. A blank
		// "!" line is a command procedure's empty comment.
		if (strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "  ")) || line == "" {
			cur = nil
			cmd := strings.TrimSpace(line)

			switch {
			case cmd == "SET MODE NOSYMBOLIC":
				symbolic = false
			case cmd == "SET MODE SYMBOLIC":
				symbolic = true
			case cmd == "SET RADIX DECIMAL":
				radix = 10
			case cmd == "CANCEL RADIX":
				radix = 16
			case strings.HasPrefix(cmd, "EXAMINE/INSTRUCTION ") && cmd != "EXAMINE/INSTRUCTION .PC":
				out = append(out, oracleRange{
					command:  strings.TrimPrefix(cmd, "EXAMINE/INSTRUCTION "),
					symbolic: symbolic,
					radix:    radix,
				})
				cur = &out[len(out)-1]
			}

			continue
		}

		// A %DEBUG message is a later command's, whose own line the log
		// leaves out.
		if cur != nil && !strings.HasPrefix(line, "%") {
			cur.lines = append(cur.lines, line)
		}
	}

	return out
}

// oracleImage is one debugger session and the image it ran: the image
// the session examined, and, where the objects are in testdata, how to
// build the same image with govax's LINK from VMS's own objects.
type oracleImage struct {
	session string   // the session log, in testdata/dbg/vax
	image   string   // the image VMS's debugger ran, in testdata/dbg/vax
	objects []string // VMS MACRO's objects, from the repository root
	debug   bool     // linked /DEBUG (else with traceback only)
}

// TestExamineInstructionOracle (docs/PHASE-41.md, subtask 13, moved to
// the debugger's EXAMINE/INSTRUCTION by docs/PHASE-42.md, subtask 9):
// every range the probe's sessions examined under SET MODE SYMBOLIC,
// run through the debugger's EXAMINE/INSTRUCTION with the session's
// SET MODE and SET RADIX, prints exactly what VMS's debugger printed: the locations, the instructions with their operands named,
// CASE tables, and SET RADIX DECIMAL's offsets. Each session is checked
// on the image VMS's debugger ran (real LINK's, or govax's for GVDBGDIS
// and GVTRACE) and on govax's LINK of VMS MACRO's objects for the same
// image, so the oracle covers govax's linker's debug tables as well as
// its disassembler.
//
// SET MODE NOSYMBOLIC's ranges aren't run here: the numeric layout is
// checked in internal/dbgsym's TestSymbolicInstructions.
func TestExamineInstructionOracle(t *testing.T) {
	images := []oracleImage{
		{"dbgdis.dlg", "dbgdis.exe", []string{"testdata/dbg/vax/dbgdis.obj", "testdata/dbg/vax/dbgsub.obj"}, true},
		{"dbgtrc.dlg", "dbgtrc.exe", []string{"testdata/dbg/vax/dbgdis.obj", "testdata/dbg/vax/dbgsub.obj"}, false},
		{"gvdbgdis.dlg", "gvdbgdis.exe", nil, true},
		{"faillnk.dlg", "faillnk.exe", []string{"testdata/mar/list/vax/failmaid.obj", "testdata/mar/list/vax/failsubd.obj"}, true},
		{"forth.dlg", "forth.exe", []string{"testdata/mar/dst/vax/forth.obj"}, true},
		{"trdbglnk.dlg", "trdbglnk.exe", []string{"testdata/mar/list/vax/trdebug.obj"}, true},
		{"gvtrace.dlg", "gvtrace.exe", nil, true},
	}

	root := filepath.Join(dbgImagePath(t, ""), "..", "..", "..")
	total := 0

	for _, im := range images {
		ranges := oracleRanges(t, im.session)

		symbolic := 0

		for _, r := range ranges {
			if r.symbolic {
				symbolic++
			}
		}

		if symbolic == 0 {
			t.Fatalf("%s: no symbolic EXAMINE/INSTRUCTION ranges", im.session)
		}

		// The image the session ran, then govax's link of its objects.
		builds := []struct{ name, path string }{{"session", dbgImagePath(t, im.image)}}

		if len(im.objects) > 0 {
			builds = append(builds, struct{ name, path string }{"relinked", oracleLink(t, root, im)})
		}

		for _, b := range builds {
			path := b.path

			t.Run(strings.TrimSuffix(im.session, ".dlg")+"/"+b.name, func(t *testing.T) {
				c, _ := runImage(t, path, console.RunOptions{Debug: console.DebugOn})
				buf := c.Out.(*bytes.Buffer)

				for _, r := range ranges {
					if !r.symbolic {
						continue
					}

					command := "EXAMINE/INSTRUCTION " + r.command

					buf.Reset()

					radix := "HEXADECIMAL"
					if r.radix == 10 {
						radix = "DECIMAL"
					}

					if err := c.Debugger.Dispatch("SET RADIX " + radix); err != nil {
						t.Fatal(err)
					}

					if err := c.Debugger.Dispatch(command); err != nil {
						t.Errorf("%s: %v", command, err)

						continue
					}

					got := strings.TrimSuffix(buf.String(), "\n")
					if want := strings.Join(r.lines, "\n"); got != want {
						t.Errorf("%s (radix %d):\ngot:\n%s\nwant:\n%s", command, r.radix, got, want)
					}

					total += len(r.lines)
				}
			})
		}
	}

	t.Logf("%d lines", total)
}

// oracleLink links im's objects with govax's LINK, as the probe's
// command procedure linked them on VMS, and returns the image's path.
func oracleLink(t *testing.T, root string, im oracleImage) string {
	t.Helper()

	d, _ := newTestDispatcher(t)
	exe := filepath.Join(t.TempDir(), im.image)

	quoted := make([]string, len(im.objects))
	for i, o := range im.objects {
		quoted[i] = `"` + filepath.Join(root, filepath.FromSlash(o)) + `"`
	}

	command := "LINK " + strings.Join(quoted, ",") + `/EXECUTABLE="` + exe + `"`
	if im.debug {
		command += "/DEBUG"
	}

	if err := d.Dispatch(command); err != nil {
		t.Fatalf("%s: %v", command, err)
	}

	return exe
}
