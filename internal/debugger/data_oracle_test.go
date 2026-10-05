package debugger_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
)

// logEntry is one command of a debugger session log (testdata/dbgcmd/vax,
// the VMS 7.3 debugger's own output) and what it printed.
type logEntry struct {
	command string
	output  []string
}

// readSessionLog reads a session log. A command is echoed after "! " and
// its output follows in the first column ("!" and a character that isn't a
// blank). A "%DEBUG-" message line is the error of a command whose own
// echo the log lacks (VMS's logging leaves the echo out when a command
// fails), so those lines are left out of the output and the commands
// aren't matched by them.
func readSessionLog(t *testing.T, dir, name string) []logEntry {
	t.Helper()

	data, err := os.ReadFile(consoletest.RepoPath(t, "testdata", dir, "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	var entries []logEntry

	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		line, ok := strings.CutPrefix(l, "!")
		if !ok {
			continue
		}

		switch {
		case strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "  "):
			if cmd := strings.TrimSpace(line); cmd != "" && !strings.HasPrefix(cmd, "!") {
				entries = append(entries, logEntry{command: cmd})
			}
		case line != "" && !strings.HasPrefix(line, "%") && len(entries) > 0:
			last := &entries[len(entries)-1]
			last.output = append(last.output, line)
		}
	}

	return entries
}

// readScript reads the commands of a probe's command file (.dbg).
func readScript(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(consoletest.RepoPath(t, "testdata", "dbgcmd", name))
	if err != nil {
		t.Fatal(err)
	}

	var cmds []string

	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "!") {
			cmds = append(cmds, l)
		}
	}

	return cmds
}

// TestExamineDataOracle replays the probe's exam.dbg under the debugger and
// compares each EXAMINE and EVALUATE with what VMS 7.3's debugger printed
// (exam.dlg): registers and memory by their types, the current, next, and
// previous locations, ranges and lists, arrays, strings, radixes, and
// MACRO's operators. The commands the other subtasks implement are run for
// their effect only. Commands whose output depends on where VMS put the
// stack (addresses 7FED....) or on running in user mode (the PSL) are
// left out; their formats are tested apart.
func TestExamineDataOracle(t *testing.T) {
	entries := readSessionLog(t, "dbgcmd", "exam.dlg")
	script := readScript(t, "exam.dbg")
	c := stepSession(t)

	next, compared := 0, 0

	for _, e := range entries {
		// Run every script command up to this one, for its effect.
		at := -1

		for i := next; i < len(script); i++ {
			if script[i] == e.command {
				at = i

				break
			}
		}

		if at < 0 {
			continue
		}

		for ; next < at; next++ {
			_, _ = sayErr(c, script[next])
		}

		next = at + 1

		out, err := sayErr(c, e.command)

		if !isCompared(e) {
			continue
		}

		if err != nil {
			t.Errorf("%s: %v", e.command, err)

			continue
		}

		compared++

		want := strings.Join(e.output, "\n")
		if len(e.output) > 0 {
			want += "\n"
		}

		if out != want {
			t.Errorf("%s:\n got %q\nwant %q", e.command, out, want)
		}
	}

	if compared < 40 {
		t.Errorf("only %d commands compared; is the log being read?", compared)
	}
}

// isCompared says whether TestExamineDataOracle checks e's output.
func isCompared(e logEntry) bool {
	cmd := strings.ToUpper(e.command)

	if !strings.HasPrefix(cmd, "EXAMINE") && !strings.HasPrefix(cmd, "EVALUATE") {
		return false
	}

	// EXAMINE/INSTRUCTION is subtask 9's (its own oracle); the numeric
	// layout of SET MODE NOSYMBOLIC is subtask 11's.
	if strings.Contains(cmd, "/INSTRUCTION") || strings.Contains(cmd, "/OPERANDS") {
		return false
	}

	// The stack and the access mode differ from VMS's run.
	for _, word := range []string{" SP", " AP", " FP", ".SP", "@SP", ".AP", "PSL"} {
		if strings.Contains(cmd, word) {
			return false
		}
	}

	for _, line := range e.output {
		if strings.Contains(line, "7FED") {
			return false
		}
	}

	// An EVALUATE whose log has its error under the next command.
	return !strings.HasPrefix(cmd, "EVALUATE .")
}

// TestSymbolizeOracle replays the SYMBOLIZE and EVALUATE/ADDRESS commands
// of Phase 41's debugger sessions (testdata/dbg/vax): every name VMS's
// debugger gave an address, from the module symbols and lines and, for an
// image linked with a global symbol table (dbgdis.exe, not dbgtrc.exe),
// the globals.
func TestSymbolizeOracle(t *testing.T) {
	for _, s := range []struct{ log, image string }{
		{"dbgdis.dlg", "dbgdis.exe"},
		{"dbgtrc.dlg", "dbgtrc.exe"},
	} {
		t.Run(s.log, func(t *testing.T) {
			c, _ := runImage(t, dbgImagePath(t, s.image), console.RunOptions{Debug: console.DebugOn})
			compared := 0

			for _, e := range readSessionLog(t, "dbg", s.log) {
				cmd := strings.ToUpper(e.command)

				switch {
				case strings.HasPrefix(cmd, "SET RADIX"), strings.HasPrefix(cmd, "CANCEL RADIX"):
					_, _ = sayErr(c, e.command)
				case strings.HasPrefix(cmd, "SYMBOLIZE "), strings.HasPrefix(cmd, "EVALUATE/ADDRESS "):
					out, err := sayErr(c, e.command)
					if err != nil {
						t.Errorf("%s: %v", e.command, err)

						continue
					}

					compared++

					if want := strings.Join(e.output, "\n") + "\n"; out != want {
						t.Errorf("%s:\n got %q\nwant %q", e.command, out, want)
					}
				}
			}

			if compared < 3 {
				t.Errorf("only %d commands compared", compared)
			}
		})
	}
}
