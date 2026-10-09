package console

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 50's probe (testdata/dcl50): probe50.com run on VMS 7.3, its log
// in vax/probe50.log. Sections A (expressions) and B (substitution) are
// cases of the form
//
//	$ X = "-"
//	$ command
//	$ SHOW SYMBOL $STATUS
//	$ SHOW SYMBOL X
//
// and TestProbe50Oracle runs each case's command through the console and
// compares its messages and X with VMS's.

const (
	probe50Procedure = "../../testdata/dcl50/probe50.com"
	probe50Log       = "../../testdata/dcl50/vax/probe50.log"
)

// probe50Case is one case: its command, and what VMS showed for it: the
// messages, and SHOW SYMBOL $STATUS's and SHOW SYMBOL X's lines.
type probe50Case struct {
	command  string
	messages []string
	status   string
	x        string
}

// probe50Expected lists the cases whose result govax doesn't match yet,
// by command, with why.
var probe50Expected = map[string]string{}

// readProbe50 returns the setup commands and cases of probe50.com's
// sections A and B, in order (a setup command has no messages or x),
// with VMS's results from its log.
func readProbe50(t *testing.T) []probe50Case {
	t.Helper()

	procedure, err := os.ReadFile(probe50Procedure)
	if err != nil {
		t.Fatal(err)
	}

	logText, err := os.ReadFile(probe50Log)
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	// The procedure's commands, without "$ ", through section B's cases.
	var commands []string

	for _, line := range strings.Split(string(procedure), "\n") {
		if strings.Contains(line, "DEFINE keeps") {
			break
		}

		if command, ok := strings.CutPrefix(line, "$ "); ok && !strings.HasPrefix(command, "!") {
			commands = append(commands, command)
		}
	}

	// The log's blocks: each case's lines from its X = "-" to its SHOW
	// SYMBOL X's output.
	logLines := strings.Split(strings.ReplaceAll(string(logText), "\r", ""), "\n")

	var blocks [][]string

	for i := 0; i < len(logLines); i++ {
		if logLines[i] != `$ X = "-"` {
			continue
		}

		j := i + 1
		for j < len(logLines) && logLines[j] != "$ SHOW SYMBOL X" {
			j++
		}

		if j+1 < len(logLines) {
			blocks = append(blocks, logLines[i+1:j+2])
		}

		i = j
	}

	var cases []probe50Case

	for i := 0; i < len(commands); i++ {
		if commands[i] != `X = "-"` {
			if !strings.HasPrefix(commands[i], "SET ") {
				cases = append(cases, probe50Case{command: commands[i]})
			}

			continue
		}

		if len(blocks) == 0 || i+3 >= len(commands) {
			t.Fatalf("the log has fewer cases than the procedure, at %q", commands[i+1])
		}

		block := blocks[0]
		blocks = blocks[1:]

		c := probe50Case{
			command: commands[i+1],
			status:  strings.TrimSpace(block[len(block)-3]),
			x:       strings.TrimSpace(block[len(block)-1]),
		}

		// The block: the command's echo (after substitution, and missing
		// when substitution failed), its messages, SHOW SYMBOL $STATUS
		// and its line, SHOW SYMBOL X and its line.
		for _, line := range block[:len(block)-4] {
			if !strings.HasPrefix(line, "$ ") {
				c.messages = append(c.messages, strings.TrimSpace(line))
			}
		}

		cases = append(cases, c)
		i += 3
	}

	return cases
}

// probe50Runner returns a function that runs one command line through
// d and returns what it showed: its output's lines and its message, as
// the terminal would show them.
func probe50Runner(d *Dispatcher, buf *bytes.Buffer) func(string) []string {
	return func(command string) []string {
		buf.Reset()

		var lines []string

		err := d.Dispatch(command)
		if out := strings.TrimSpace(buf.String()); out != "" {
			lines = append(lines, strings.Split(out, "\n")...)
		}

		if err != nil && !vmserrors.MessageInhibited(err) {
			for _, line := range strings.Split("%"+err.Error(), "\n") {
				lines = append(lines, strings.TrimSpace(line))
			}
		}

		return lines
	}
}

// TestProbe50Oracle replays probe50.com's sections A and B through the
// console, comparing each case's messages, $STATUS, and value of X with
// VMS 7.3's.
func TestProbe50Oracle(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	run := probe50Runner(d, buf)

	cases := readProbe50(t)
	if len(cases) < 100 {
		t.Fatalf("only %d cases read", len(cases))
	}

	for _, c := range cases {
		if c.x == "" {
			if lines := run(c.command); len(lines) != 0 {
				t.Fatalf("setting up, %s: %q", c.command, lines)
			}

			continue
		}

		run(`X = "-"`)

		messages := run(c.command)
		status := strings.Join(run("SHOW SYMBOL $STATUS"), "\n")
		x := strings.Join(run("SHOW SYMBOL X"), "\n")

		if strings.Join(messages, "\n") == strings.Join(c.messages, "\n") && status == c.status && x == c.x {
			if why, ok := probe50Expected[c.command]; ok {
				t.Errorf("%s: matches VMS now (expected to differ: %s)", c.command, why)
			}

			continue
		}

		if _, ok := probe50Expected[c.command]; ok {
			continue
		}

		t.Errorf("%s:\n  govax: %q, %s, %s\n  VMS:   %q, %s, %s", c.command, messages, status, x, c.messages, c.status, c.x)
	}
}

// probe50Differences are the section D commands whose results govax
// doesn't match, with why; the SHOW SYMBOL commands after each are
// skipped with it.
var probe50Differences = map[string]string{
	"TYPE NOSUCH.TXT": "govax's TYPE reports a missing file with its own message and status",
	"@NOSUCH":         "govax reports a missing procedure with SS$_NOSUCHFILE, not DCL's OPENIN and RMS$_FNF",
}

// TestProbe50Statuses replays probe50.com's section D, statuses, through
// the console: each command, and SHOW SYMBOL $STATUS and $SEVERITY after
// it, against VMS 7.3's log. The log shows each command as SET VERIFY
// echoed it, and its procedure's lines too; those are left out, and the
// rest of each command's lines compared.
func TestProbe50Statuses(t *testing.T) {
	procedure, err := os.ReadFile(probe50Procedure)
	if err != nil {
		t.Fatal(err)
	}

	logText, err := os.ReadFile(probe50Log)
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	section := func(lines []string) []string {
		var out []string

		in := false

		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "$ ! Section D"):
				in = true
			case line == "$ SET NOVERIFY":
				in = false
			case in:
				out = append(out, line)
			}
		}

		return out
	}

	commands := section(strings.Split(string(procedure), "\n"))
	logLines := section(strings.Split(strings.ReplaceAll(string(logText), "\r", ""), "\n"))

	// Each command's lines in the log: those after its echo, up to the
	// next command's, without the echoes of probe50e.com's lines.
	expected := make([][]string, len(commands))
	k := -1

	for _, line := range logLines {
		if k+1 < len(commands) && line == commands[k+1] {
			k++

			continue
		}

		if k >= 0 && !strings.HasPrefix(line, "$ ") {
			expected[k] = append(expected[k], strings.TrimSpace(line))
		}
	}

	if k != len(commands)-1 {
		t.Fatalf("found %d of section D's %d commands in the log", k+1, len(commands))
	}

	d, _, buf := newCommandDispatcher(t)
	run := probe50Runner(d, buf)
	skipping := ""

	for i, command := range commands {
		command = strings.TrimPrefix(command, "$ ")
		if !strings.HasPrefix(command, "SHOW SYMBOL") {
			_, skip := probe50Differences[command]
			skipping = map[bool]string{true: command}[skip]
		}

		line := strings.Replace(command, "@PROBE50E", `@"../../testdata/dcl50/probe50e"`, 1)
		got := run(line)

		if skipping != "" {
			continue
		}

		if strings.Join(got, "\n") != strings.Join(expected[i], "\n") {
			t.Errorf("%s:\n  govax: %q\n  VMS:   %q", command, got, expected[i])
		}
	}
}
