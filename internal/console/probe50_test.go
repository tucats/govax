package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
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

// TestProbe50Verify runs probe50.com's sections A and B as a procedure,
// with the SET VERIFY it starts with, and compares everything it writes
// with VMS 7.3's log line for line: each command echoed after its
// apostrophe substitution, comments included, then its messages and
// output.
func TestProbe50Verify(t *testing.T) {
	procedure, err := os.ReadFile(probe50Procedure)
	if err != nil {
		t.Fatal(err)
	}

	logText, err := os.ReadFile(probe50Log)
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	head, _, _ := strings.Cut(string(procedure), "$ ! DEFINE keeps")
	// The log's records are written LF, text, CR, so it starts with a
	// line feed.
	want, _, _ := strings.Cut(strings.ReplaceAll(string(logText), "\r", ""), "$ ! DEFINE keeps")
	want = strings.TrimPrefix(want, "\n")

	d, _, buf := newCommandDispatcher(t)
	path := writeProcedure(t, t.TempDir(), "probe50.com", strings.TrimSuffix(head, "\n"))

	if err := d.Dispatch("@" + path); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@probe50: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	wantLines := strings.Split(strings.TrimSuffix(want, "\n"), "\n")

	for i := 0; i < max(len(got), len(wantLines)); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}

		if i < len(wantLines) {
			w = wantLines[i]
		}

		if g != w {
			t.Fatalf("line %d:\n got %q\nwant %q", i+1, g, w)
		}
	}
}

// probe50VSections splits a verified run's output into its sections, by
// the "$ ! Vn:" comment that starts each (the lines before the first are
// section "").
func probe50VSections(text string) map[string][]string {
	return probeSections(text, "V")
}

// probeSections splits a verified run's output into its sections, by
// the "$ ! <letter>n:" comment that starts each (the lines before the
// first are section "").
func probeSections(text, letter string) map[string][]string {
	sections := map[string][]string{}
	name := ""

	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if rest, ok := strings.CutPrefix(line, "$ ! "+letter); ok {
			if n, _, found := strings.Cut(rest, ":"); found && n != "" && strings.Trim(n, "0123456789") == "" {
				name = letter + n
			}
		}

		sections[name] = append(sections[name], line)
	}

	return sections
}

// TestProbe50VOracle runs round 2 of testdata/dcl50, probe50v.com (and
// probe50w.com, which it runs), and compares what it writes with VMS
// 7.3's log line for line, section by section: SET VERIFY, SET PREFIX,
// and F$VERIFY. The clock is the run's, for SET PREFIX's time.
func TestProbe50VOracle(t *testing.T) {
	logText, err := os.ReadFile("../../testdata/dcl50/vax/probe50v.log")
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	dir := t.TempDir()

	for _, name := range []string{"probe50v.com", "probe50w.com"} {
		text, err := os.ReadFile(filepath.Join("../../testdata/dcl50", name))
		if err != nil {
			t.Fatal(err)
		}

		writeFile(t, filepath.Join(dir, name), string(text))
	}

	t.Chdir(dir)

	d, c, buf := newCommandDispatcher(t)
	now := vmsdef.Time(time.Date(2026, 10, 11, 14, 13, 0, 0, time.UTC))
	c.RTL.Clock = func() uint64 { return now }

	if err := d.Dispatch("@PROBE50V"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PROBE50V: %v", err)
	}

	// The log's records are written LF, text, CR.
	want := probe50VSections(strings.TrimPrefix(strings.ReplaceAll(string(logText), "\r", ""), "\n"))
	got := probe50VSections(buf.String())

	if len(want) < 11 {
		t.Fatalf("the log has %d sections, want 11", len(want))
	}

	for name, wantLines := range want {
		gotLines := got[name]

		for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}

			if i < len(wantLines) {
				w = wantLines[i]
			}

			if g != w {
				t.Errorf("section %s, line %d:\n got %q\nwant %q", name, i+1, g, w)

				break
			}
		}
	}
}

// probe50IExact are the sections of round 3 govax's run must match line
// for line. I1's output has commands govax lacks or answers differently
// (SET SYMBOL, WAIT, SHOW DEFAULT's device), so only what became of each
// data line is compared (dataOutcomes).
var probe50IExact = []string{"I2", "I3", "I4", "I5", "I6", "I7", "I8", "I9"}

// dataOutcomes lists, for each data line that is a WRITE in a section,
// whether DCL read it as a command ("read": its text follows) or skipped
// it (SKPDAT follows, or nothing).
func dataOutcomes(lines []string) []string {
	var out []string

	for i, line := range lines {
		arg, ok := strings.CutPrefix(line, `WRITE SYS$OUTPUT "`)
		if !ok {
			continue
		}

		outcome := "skipped"
		if i+1 < len(lines) && lines[i+1] == strings.TrimSuffix(arg, `"`) {
			outcome = "read"
		}

		out = append(out, line+": "+outcome)
	}

	return out
}

// TestProbe50IOracle runs round 3 of testdata/dcl50, probe50i.com (and
// probe50j.com and probe50k.com, which it runs), and compares what it
// writes with VMS 7.3's log: SYS$INPUT from a procedure, DECK, and EOD
// (subtask 16). DIRECTORY's /NOHEADING and /NOTRAILING, which govax's
// DIRECTORY doesn't have, are left out, so that it runs, as on VMS.
func TestProbe50IOracle(t *testing.T) {
	logText, err := os.ReadFile("../../testdata/dcl50/vax/probe50i.log")
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	dir := t.TempDir()

	for _, name := range []string{"probe50i.com", "probe50j.com", "probe50k.com"} {
		text, err := os.ReadFile(filepath.Join("../../testdata/dcl50", name))
		if err != nil {
			t.Fatal(err)
		}

		writeFile(t, filepath.Join(dir, name), strings.ReplaceAll(string(text), "/NOHEADING/NOTRAILING", ""))
	}

	t.Chdir(dir)

	d, _, buf := newCommandDispatcher(t)

	if err := d.Dispatch("@PROBE50I"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PROBE50I: %v", err)
	}

	want := probeSections(strings.TrimPrefix(strings.ReplaceAll(string(logText), "\r", ""), "\n"), "I")
	got := probeSections(buf.String(), "I")

	if len(want) < 10 {
		t.Fatalf("the log has %d sections, want 10", len(want))
	}

	for _, name := range probe50IExact {
		gotLines, wantLines := got[name], want[name]

		for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}

			if i < len(wantLines) {
				w = wantLines[i]
			}

			if g != w {
				t.Errorf("section %s, line %d:\n got %q\nwant %q", name, i+1, g, w)

				break
			}
		}
	}

	gotOutcomes, wantOutcomes := dataOutcomes(got["I1"]), dataOutcomes(want["I1"])
	if len(wantOutcomes) < 28 {
		t.Fatalf("I1 has %d data lines in the log, want 28", len(wantOutcomes))
	}

	if strings.Join(gotOutcomes, "\n") != strings.Join(wantOutcomes, "\n") {
		t.Errorf("I1's data lines:\ngot:\n%s\nwant:\n%s", strings.Join(gotOutcomes, "\n"), strings.Join(wantOutcomes, "\n"))
	}
}

// probe50FSkipped are the commands of round 4 whose output is outside
// the probe's question, and left out of the comparison: DELETE's,
// DIRECTORY's, and DUMP's (which govax hasn't) layouts are their own.
var probe50FSkipped = []string{"$ DELETE ", "$ DIRECTORY", "$ DUMP"}

// probe50FCompared is a round 4 log as the comparison sees it: the output
// of the probe50FSkipped commands left out, and the extra blank VMS 7.3's
// SHOW LOGICAL puts before a process-permanent file's name ("   "OUT"")
// taken away, govax's logical names having no process-permanent file
// behind them to show it by (DEVIATIONS.md, "[Phase 50] DCL's file
// I/O").
func probe50FCompared(text string) string {
	var (
		out      []string
		skipping bool
	)

	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "$ ") {
			skipping = false

			for _, prefix := range probe50FSkipped {
				if strings.HasPrefix(line, prefix) {
					skipping = true
				}
			}
		} else if skipping {
			continue
		}

		if rest, ok := strings.CutPrefix(line, `   "`); ok {
			line = `  "` + rest
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

// TestProbe50FOracle runs round 4 of testdata/dcl50, probe50f.com (and
// probe50g.com, which it runs), on a volume mounted on DUA1, the node
// called SIMVAX, as the VMS run's was, and compares what it writes with
// VMS 7.3's log line for line, section by section: OPEN, READ, WRITE, and
// CLOSE.
func TestProbe50FOracle(t *testing.T) {
	logText, err := os.ReadFile("../../testdata/dcl50/vax/probe50f.log")
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA1")
	c.RTL.NodeName = "SIMVAX"

	dir := t.TempDir()

	for _, name := range []string{"probe50f.com", "probe50g.com"} {
		path := filepath.Join(dir, name)
		writeFile(t, path, readTestdata(t, filepath.Join("../../testdata/dcl50", name)))

		if err := d.Dispatch(`COPY "` + path + `"/HOST DUA1:[000000]` + strings.ToUpper(name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.SetDefault("DUA1:[000000]"); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := d.Dispatch("@PROBE50F/OUTPUT=PROBE50F.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PROBE50F: %v", err)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]PROBE50F.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	// What went to the terminal, not the log: WRITE SYS$ERROR's record
	// (VMS 7.3's too, testdata/dcl50's Q11) and the messages, which go
	// to both.
	if !strings.Contains(buf.String(), "to SYS$ERROR\n") {
		t.Errorf("WRITE SYS$ERROR's record didn't go to the terminal; it showed:\n%s", buf.String())
	}

	want := probeSections(probe50FCompared(strings.TrimPrefix(strings.ReplaceAll(string(logText), "\r", ""), "\n")), "F")
	got := probeSections(probe50FCompared(string(joinRecords(records))), "F")

	if len(want) < 12 {
		t.Fatalf("the log has %d sections, want 12", len(want))
	}

	for name, wantLines := range want {
		// Before its first section, VMS's log has the probe's opening
		// comments, verification having been on when it ran.
		if name == "" {
			continue
		}

		gotLines := got[name]

		for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}

			if i < len(wantLines) {
				w = wantLines[i]
			}

			if g != w {
				t.Errorf("section %q, line %d:\n got %q\nwant %q", name, i+1, g, w)

				break
			}
		}
	}
}

// readTestdata returns a testdata file's text.
func readTestdata(t *testing.T, path string) string {
	t.Helper()

	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(text)
}

// probe50QTyped is what the author typed at round 4's interactive
// probe's prompts (probe50q.com's comments say what): Q1 to Q3's
// replies, Ctrl/Z (Q4), Return (Q5), Q6 and Q7's replies, Ctrl/Z twice
// (Q8, Q9), and Q11's reply.
const probe50QTyped = "hello 'X'\"Mixed  Case\"\nyes\nraw\n\x1a\nSome  Text 'X'\nabc\n\x1a\x1anested\n"

// probe50QResults are the lines of an interactive session that say what
// a command did: a symbol's value, $STATUS, and a message. The prompts
// and the replies' echoes are the terminal's, which a test's input
// doesn't make (so a message may follow a prompt on its line).
func probe50QResults(text string) []string {
	var out []string

	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, "%"); i >= 0 && !strings.HasPrefix(line, "$") && !strings.HasPrefix(line, "  ") {
			line = line[i:]
		}

		if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "%") {
			out = append(out, line)
		}
	}

	return out
}

// TestProbe50QOracle runs round 4's interactive probe, probe50q.com (and
// probe50p.com, which it runs with /OUTPUT), with what the author typed
// on VMS, and compares what each INQUIRE and READ left with VMS 7.3's
// session (vax/probe50q.log), and the log probe50p.com wrote with VMS's
// (vax/probe50p.log), line for line.
func TestProbe50QOracle(t *testing.T) {
	session, err := os.ReadFile("../../testdata/dcl50/vax/probe50q.log")
	if err != nil {
		t.Skipf("no VMS session: %v", err)
	}

	vmsLog := readTestdata(t, "../../testdata/dcl50/vax/probe50p.log")

	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA1")
	c.In = strings.NewReader(probe50QTyped)

	dir := t.TempDir()

	for _, name := range []string{"probe50q.com", "probe50p.com"} {
		path := filepath.Join(dir, name)
		writeFile(t, path, readTestdata(t, filepath.Join("../../testdata/dcl50", name)))

		if err := d.Dispatch(`COPY "` + path + `"/HOST DUA1:[000000]` + strings.ToUpper(name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.SetDefault("DUA1:[000000]"); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := d.Dispatch("@PROBE50Q"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PROBE50Q: %v", err)
	}

	got, want := probe50QResults(buf.String()), probe50QResults(strings.ReplaceAll(string(session), "\r", ""))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the session's results:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]PROBE50P.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	log := string(joinRecords(records))
	if want := strings.TrimSpace(strings.ReplaceAll(vmsLog, "\r", "")); strings.TrimSpace(log) != want {
		t.Errorf("PROBE50P.LOG holds:\n%s\nwant:\n%s", log, want)
	}
}
