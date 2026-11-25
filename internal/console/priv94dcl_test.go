package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/vmserrors"
)

// priv94DCLSection is one section of testdata/priv94's DCL procedure
// (PRIV94D.COM) as VMS's log has it: the commands after its heading, each
// with the lines it wrote, up to the section's closing "$ !".
type priv94DCLSection struct {
	heading  string
	commands []priv94DCLCommand
}

// priv94DCLCommand is a command of a section and what VMS showed for it.
type priv94DCLCommand struct {
	line string
	out  []string
}

// priv94DCLSections reads the sections called name (the heading's second
// word, "TERMINAL") from the VMS log file, in their order: PRIV94D.COM's
// ("######## D") or round 2's PRIV94E.COM's ("######## E").
func priv94DCLSections(t *testing.T, file, name string) []priv94DCLSection {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "priv94", "vax", file))
	if err != nil {
		t.Fatal(err)
	}

	var (
		sections []priv94DCLSection
		cur      *priv94DCLSection
	)

	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		if strings.HasPrefix(line, "######## D ") || strings.HasPrefix(line, "######## E ") {
			cur = nil

			if f := strings.Fields(line); len(f) > 3 && f[2] == name {
				sections = append(sections, priv94DCLSection{heading: line})
				cur = &sections[len(sections)-1]
			}

			continue
		}

		if cur == nil {
			continue
		}

		if line == "$ !" {
			cur = nil

			continue
		}

		if cmd, ok := strings.CutPrefix(line, "$ "); ok {
			cur.commands = append(cur.commands, priv94DCLCommand{line: cmd})
		} else if n := len(cur.commands); n > 0 {
			cur.commands[n-1].out = append(cur.commands[n-1].out, line)
		}
	}

	if len(sections) == 0 {
		t.Fatalf("%s has no %s sections", file, name)
	}

	return sections
}

// replayPriv94DCL runs the sections' commands as id, each section's in turn,
// and holds what each command shows to VMS's lines, with the VAX's terminal,
// OPA0:, read as govax's. symbols are DCL symbols the procedure had defined
// before the sections (TTW).
func replayPriv94DCL(t *testing.T, id priv94Identity, sections []priv94DCLSection, symbols map[string]string) {
	t.Helper()

	replayPriv94DCLWith(t, id, sections, symbols, nil)
}

// replayPriv94DCLWith is replayPriv94DCL with setup run on the console
// first, as SYSTEM, before it becomes id: the fixtures the sections need,
// made through dispatch, which runs a line and returns what it wrote.
// setup returns strings of VMS's log (the holders' PIDs) and what govax's
// run has in their place, replaced in the commands and the lines VMS showed.
func replayPriv94DCLWith(t *testing.T, id priv94Identity, sections []priv94DCLSection, symbols map[string]string,
	setup func(c *console.Console, dispatch func(string) string) map[string]string) {
	t.Helper()

	replayPriv94DCLMasked(t, id, sections, symbols, setup, nil)
}

// replayPriv94DCLMasked is replayPriv94DCLWith with mask (if not nil) applied
// to what govax showed for each command and to what VMS did, before they
// are compared: given the command (rewritten) and its lines, it returns the
// lines to compare, so what can't be the same on two systems (times, other
// processes, priorities a scheduler boosts) can be left out of both.
func replayPriv94DCLMasked(t *testing.T, id priv94Identity, sections []priv94DCLSection, symbols map[string]string,
	setup func(c *console.Console, dispatch func(string) string) map[string]string,
	mask func(line string, out []string) []string) {
	t.Helper()

	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) []string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		text := strings.TrimSuffix(strings.ReplaceAll(buf.String(), "\r", ""), "\n")
		if text == "" {
			return nil
		}

		return strings.Split(text, "\n")
	}

	terminal, _ := c.RTL.DVIItem("TT:", "DVI$_DEVNAM")
	terminal = strings.TrimPrefix(terminal, "_")

	var replace map[string]string
	if setup != nil {
		replace = setup(c, func(line string) string { return strings.Join(dispatch(line), "\n") })
	}

	rewrite := func(line string) string {
		line = strings.ReplaceAll(line, "OPA0:", terminal)
		for from, to := range replace {
			line = strings.ReplaceAll(line, from, to)
		}

		return line
	}

	id.become(c)

	for name, value := range symbols {
		dispatch(name + " = " + value)
	}

	for _, s := range sections {
		for _, cmd := range s.commands {
			line := rewrite(cmd.line)
			got := dispatch(line)

			want := make([]string, len(cmd.out))
			for i, out := range cmd.out {
				want[i] = rewrite(out)
			}

			if mask != nil {
				got, want = mask(line, got), mask(line, want)
			}

			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("%s: %s:\n got %q\nwant %q", s.heading, line, got, want)
			}
		}
	}
}

// TestPriv94Terminal replays the probe's TERMINAL sections (the phase doc's
// subtask 6): SET TERMINAL and SET TERMINAL/PERMANENT with no privilege, with
// LOG_IO, and with PHY_IO, as SYSTEM, PRIV94, and PRIV94N, whose logs say
// /PERMANENT needs one of them and that PRIV94N, authorized for neither,
// can't enable them.
func TestPriv94Terminal(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	for _, run := range []struct {
		id   priv94Identity
		file string
	}{
		{priv94System, "priv94.log"},
		{priv94Account, "priv94u_priv94.log"},
		{priv94Plain, "priv94u_priv94n.log"},
	} {
		t.Run(run.id.name, func(t *testing.T) {
			replayPriv94DCL(t, run.id, priv94DCLSections(t, run.file, "TERMINAL"),
				map[string]string{"TTW": "207"})
		})
	}
}
