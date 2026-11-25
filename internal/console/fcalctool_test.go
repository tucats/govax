package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/testproc"
)

// fcalc as a tool (docs/planning/PHASE-111 - Fortran 90 milestone.md,
// subtask 12): the system volume holds SYS$SYSTEM:FCALC.EXE (internal/fcalc's
// committed image), its sources in SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC] with
// BUILDFCALC.COM, and HELP FCALC in HELPLIB.HLB, which fcalc's own HELP shows
// too; the supplied SYLOGIN.COM defines FCALC as a foreign command.

// fcalcToolConsole boots a console on a new system volume, its default
// DUA0:[TEST] (libraryConsole's).
func fcalcToolConsole(t *testing.T) (*Console, func(string) string) {
	t.Helper()

	return libraryConsole(t, false)
}

// runShippedFcalc runs the system volume's FCALC.EXE with input as its
// SYS$INPUT, and returns what it printed.
func runShippedFcalc(c *Console, run func(string) string, input string) string {
	c.In = strings.NewReader(input)

	return run("RUN SYS$SYSTEM:FCALC")
}

// TestFcalcTry is a tool for writing fcalc's help: FCALC_TRY names a file
// of fcalc's lines, which the shipped image runs, echoing each.
func TestFcalcTry(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	file := os.Getenv("FCALC_TRY")
	if file == "" {
		t.Skip("FCALC_TRY names no file")
	}

	text, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	c, run := fcalcToolConsole(t)
	t.Logf("\n%s", runShippedFcalc(c, run, "set echo\n"+string(text)))
}

// fcalcHelpFile is HELP FCALC's source, internal/bootdata's.
var fcalcHelpFile = filepath.Join("..", "bootdata", "files", "help", "fcalc.hlp")

// helpTopic is one topic of a help file: its key line's index in the file's
// lines, its path of keys from the top ("FCALC EXAMPLES MATRICES"), and its
// text, the lines up to the next key line.
type helpTopic struct {
	at   int
	path string
	text []string
}

// helpTopics reads a help file's topics.
func helpTopics(lines []string) []helpTopic {
	var (
		topics []helpTopic
		keys   []string
	)

	for i, line := range lines {
		if level, key, ok := lbr.HelpKeyLine([]byte(line)); ok {
			keys = append(keys[:min(level-1, len(keys))], strings.ToUpper(key))
			topics = append(topics, helpTopic{at: i, path: strings.Join(keys, " ")})

			continue
		}

		if len(topics) > 0 {
			topics[len(topics)-1].text = append(topics[len(topics)-1].text, line)
		}
	}

	return topics
}

// helpExampleIndent is how far a help topic's examples are indented.
const helpExampleIndent = "     "

// fcalcHelpSession is what fcalc must print for a topic's examples, and the
// lines they type. An example is an indented block of lines starting with
// one at fcalc's prompt; in a topic under Examples, the whole topic from its
// first prompt is one example. A topic's examples are one session, in order.
func fcalcHelpSession(topic helpTopic) (typed, want []string) {
	whole := strings.HasPrefix(topic.path, "FCALC EXAMPLES ")
	in := false

	for _, line := range topic.text {
		body, indented := strings.CutPrefix(line, helpExampleIndent)

		switch {
		case !in && indented && strings.HasPrefix(body, "fcalc> "):
			in = true
		case in && !whole && (line == "" || !indented):
			in = false
		}

		if !in {
			continue
		}

		want = append(want, strings.TrimRight(body, " "))

		if text, ok := strings.CutPrefix(body, "fcalc> "); ok {
			typed = append(typed, text)
		}
	}

	for len(want) > 0 && want[len(want)-1] == "" {
		want = want[:len(want)-1]
	}

	return typed, want
}

// TestFcalcHelpExamples runs every example in HELP FCALC on the system
// volume's FCALC.EXE, each topic's as one session, and holds what fcalc
// prints to the text. FCALC_HELP_WRITE=1 writes the Examples topics' sessions
// into the help file from fcalc's output instead (rebuild the help libraries
// after: go generate ./internal/bootdata).
func TestFcalcHelpExamples(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	data, err := os.ReadFile(fcalcHelpFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	topics := helpTopics(lines)
	write := os.Getenv("FCALC_HELP_WRITE") != ""

	c, run := fcalcToolConsole(t)
	checked := 0

	// From the last topic back, so a rewritten topic leaves the earlier
	// ones' places alone.
	for k := len(topics) - 1; k >= 0; k-- {
		topic := topics[k]

		typed, want := fcalcHelpSession(topic)
		if len(typed) == 0 {
			continue
		}

		checked++

		out := runShippedFcalc(c, run, "set echo\n"+strings.Join(typed, "\n")+"\n")
		got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")

		// Without the first line, SET ECHO's, and the last prompt, at the end
		// of the input.
		if len(got) > 0 && got[0] == "fcalc> set echo" {
			got = got[1:]
		}

		if len(got) > 0 && strings.TrimSpace(got[len(got)-1]) == "fcalc>" {
			got = got[:len(got)-1]
		}

		for i := range got {
			got[i] = strings.TrimRight(got[i], " ")
		}

		if write && strings.HasPrefix(topic.path, "FCALC EXAMPLES ") {
			first := topic.at + 1
			for !strings.HasPrefix(lines[first], helpExampleIndent+"fcalc> ") {
				first++
			}

			end := topic.at + 1 + len(topic.text)
			for end > first && lines[end-1] == "" {
				end--
			}

			var session []string
			for _, line := range got {
				session = append(session, strings.TrimRight(helpExampleIndent+line, " "))
			}

			lines = append(lines[:first], append(session, lines[end:]...)...)

			continue
		}

		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("HELP %s's examples: fcalc printed\n%s\nwant\n%s", topic.path, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}

	if checked < 10 {
		t.Errorf("only %d topics have examples", checked)
	}

	if write {
		if err := os.WriteFile(fcalcHelpFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fcalcHelpLimit is how many lines of text a topic of HELP FCALC may have, so
// that each is read in a screen or two; a topic under Examples, a session
// with its tables and graphs, may be longer.
const fcalcHelpLimit = 40

// TestFcalcHelpTopicsShort holds every topic of HELP FCALC but the examples
// to fcalcHelpLimit lines.
func TestFcalcHelpTopicsShort(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(fcalcHelpFile)
	if err != nil {
		t.Fatal(err)
	}

	topics := helpTopics(strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"))
	if len(topics) < 30 {
		t.Fatalf("HELP FCALC has %d topics", len(topics))
	}

	for _, topic := range topics {
		text := topic.text
		for len(text) > 0 && text[len(text)-1] == "" {
			text = text[:len(text)-1]
		}

		if len(text) > fcalcHelpLimit && !strings.HasPrefix(topic.path, "FCALC EXAMPLES ") {
			t.Errorf("HELP %s has %d lines, more than %d", topic.path, len(text), fcalcHelpLimit)
		}
	}
}

// TestFcalcOnSystemVolume: SYLOGIN.COM's FCALC symbol runs the system
// volume's image as a foreign command; fcalc's HELP shows what DCL's HELP
// FCALC shows; and BUILDFCALC.COM builds FCALC.EXE from the sources beside
// it, which runs a script as the milestone's build does, and with INSTALL
// makes a new version of SYS$SYSTEM:FCALC.EXE, leaving nothing else behind.
func TestFcalcOnSystemVolume(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	c, run := fcalcToolConsole(t)

	quietly(t, run, "@SYS$MANAGER:SYLOGIN")

	if got := run("FCALC 2^10 + 1"); got != "1025\n" {
		t.Errorf("FCALC 2^10 + 1 printed %q", got)
	}

	t.Run("help", func(t *testing.T) {
		for _, topic := range []string{"", "COMMANDS SOLVE", "FUNCTIONS SPECIAL", "NOSUCH"} {
			dcl := run("HELP/NOPROMPT FCALC " + topic)
			own := runShippedFcalc(c, run, "help "+topic+"\n")

			// fcalc's output is its prompt, the help, and its prompt again at
			// the end of the input.
			own = strings.TrimPrefix(own, "fcalc> ")
			own = strings.TrimSuffix(strings.TrimRight(own, "\n"), "fcalc> ")

			if own != dcl {
				t.Errorf("fcalc's HELP %s printed:\n%s\nDCL's HELP FCALC %[1]s:\n%[3]s", topic, own, dcl)
			}
		}
	})

	t.Run("build", func(t *testing.T) {
		out := run("@SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC]BUILDFCALC INSTALL")
		if out != "FCALC.EXE built\n" {
			t.Fatalf("BUILDFCALC printed:\n%s", out)
		}

		// The image in the examples' directory, and a new version in
		// SYS$SYSTEM, and nothing else made.
		for spec, want := range map[string]bool{
			"SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC]FCALC.EXE": true,
			"SYS$SYSTEM:FCALC.EXE;2":                       true,
			"SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC]*.MAR":     false,
			"SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC]*.OBJ":     false,
			"SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC]*.F90$MOD": false,
		} {
			if got := run(`WRITE SYS$OUTPUT F$SEARCH("` + spec + `") .NES. ""`); got != map[bool]string{true: "1\n", false: "0\n"}[want] {
				t.Errorf("F$SEARCH of %s: %s", spec, got)
			}
		}

		// The new version runs a script as the milestone's build does.
		script, err := os.ReadFile(filepath.Join(fcalcDirectory, "basics.in"))
		if err != nil {
			t.Fatal(err)
		}

		want, err := os.ReadFile(filepath.Join(fcalcDirectory, "basics.out"))
		if err != nil {
			t.Fatal(err)
		}

		if got := runShippedFcalc(c, run, string(script)); got != string(want) {
			t.Errorf("the new FCALC.EXE on basics.in printed:\n%s", got)
		}
	})
}
