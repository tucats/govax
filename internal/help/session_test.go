package help

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skpdat is DCL's message for a procedure's data lines HELP didn't read.
const skpdat = `%DCL-W-SKPDAT, image data (records not beginning with "$") ignored`

// procedureCase is one HELP/PROMPT command of a probe procedure: its
// library, its words, the data lines that follow it in the procedure (its
// answers), and what VMS's log shows after it.
type procedureCase struct {
	line    int
	command string
	library string
	words   string
	answers []string
	echo    bool // the answers are the procedure's own lines, which SET VERIFY shows
	want    []string
}

// procedureCases reads the HELP/PROMPT commands of a probe procedure's
// log (from the line starting with from to the one starting with to) and
// each one's answers from the procedure. hp is the symbol the procedure
// defines for HELP/PROMPT on hpLib. A command after DEFINE/USER_MODE
// SYS$INPUT reads resp.txt instead, without SET VERIFY's echo. HELP/OUTPUT
// is DCL's HELP's (subtask 6), and is passed over.
func procedureCases(t *testing.T, logPath, comPath, from, to, hp, hpLib string) []procedureCase {
	t.Helper()

	log := readLines(t, logPath)
	com := readLines(t, comPath)
	resp := readLines(t, filepath.Join(help90Dir, "resp.txt"))

	var (
		cases   []procedureCase
		in      bool
		comAt   int
		userIn  bool
		current *procedureCase
	)

	for i, line := range log {
		switch {
		case strings.HasPrefix(line, from):
			in = true

			continue
		case !in:
			continue
		case strings.HasPrefix(line, to):
			return cases
		}

		if !strings.HasPrefix(line, "$ ") {
			if current != nil {
				current.want = append(current.want, line)
			}

			continue
		}

		current = nil
		command := line[2:]

		if strings.HasPrefix(command, "DEFINE/USER_MODE SYS$INPUT") {
			userIn = true

			continue
		}

		verb, words, _ := strings.Cut(command, " ")

		var lib string

		switch {
		case verb == hp:
			lib = hpLib
		case strings.HasPrefix(verb, "HELP/PROMPT/") && !strings.Contains(verb, "/OUTPUT"):
			_, after, _ := strings.Cut(verb, "/LIBRARY=H90DIR:")
			lib = after
		default:
			continue
		}

		// Find the command in the procedure and its data lines.
		c := procedureCase{line: i + 1, command: line, library: lib, words: words, echo: !userIn}

		for comAt < len(com) && com[comAt] != line {
			comAt++
		}

		if comAt == len(com) {
			t.Fatalf("%s: %q not in %s", logPath, line, comPath)
		}

		for comAt++; comAt < len(com) && !strings.HasPrefix(com[comAt], "$"); comAt++ {
			c.answers = append(c.answers, com[comAt])
		}

		if userIn {
			c.answers = resp
			userIn = false
		}

		cases = append(cases, c)
		current = &cases[len(cases)-1]
	}

	t.Fatalf("%s: no line starting %q", logPath, to)

	return nil
}

// readLines reads a file's lines, without carriage returns.
func readLines(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	text := strings.TrimSuffix(strings.ReplaceAll(string(data), "\r", ""), "\n")

	return strings.Split(text, "\n")
}

// openProbeLibrary is a Session's Open for the probe's libraries:
// "H90DIR:H90B" is H90B.
func openProbeLibrary(libs map[string]Library) func(string) (Library, string, error) {
	return func(spec string) (Library, string, error) {
		name := strings.ToUpper(strings.TrimPrefix(strings.ToUpper(spec), "H90DIR:"))
		if lib, ok := libs[name]; ok {
			return lib, name, nil
		}

		return nil, "", errors.New("no library " + spec)
	}
}

// TestHelp90Prompting holds Session to VMS 7.1's HELP/PROMPT answered by
// a procedure's data lines (testdata/help90's part D, in round 1b's log,
// and round 1c's chain of levels) and by a file (SYS$INPUT defined as
// resp.txt). The procedures ran with SET VERIFY, which shows each data
// line HELP reads; HELP shows no prompts when its input isn't a
// terminal, and the data lines it leaves are DCL's SKPDAT.
func TestHelp90Prompting(t *testing.T) {
	libs := help90Libraries(t)

	var cases []procedureCase
	cases = append(cases, procedureCases(t,
		filepath.Join(help90Dir, "vax", "round1b", "help90.log"), filepath.Join(help90Dir, "help90d.com"),
		"$ HP := HELP/PROMPT", "$ TYPE H90P.LIS", "HP", "H90A")...)
	cases = append(cases, procedureCases(t,
		filepath.Join(help90Dir, "vax", "round2", "help90c.log"), filepath.Join(help90Dir, "help90c.com"),
		"$ ! Prompting", "$ EXIT", "HP", "H90A")...)

	if len(cases) != 10 {
		t.Fatalf("%d cases, want 10", len(cases))
	}

	for _, c := range cases {
		var got []string

		answers := c.answers
		s := &Session{
			Library: libs[c.library],
			Open:    openProbeLibrary(libs),
			Write:   func(line string) { got = append(got, line) },
			Read: func(string) (string, error) {
				if len(answers) == 0 {
					return "", io.EOF
				}

				a := answers[0]
				answers = answers[1:]

				if c.echo {
					got = append(got, a)
				}

				return a, nil
			},
			Width:   132,
			Options: Options{Instructions: true},
			Prompt:  true,
		}

		if err := s.Run(Split(c.words)); err != nil {
			t.Fatalf("line %d: %v", c.line, err)
		}

		if len(answers) > 0 && c.echo {
			got = append(got, skpdat)
		}

		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("line %d: %s\n%s", c.line, c.command, lineDiff(c.want, got))
		}
	}
}

// terminalCase is one HELP command of round 2's terminal session.
type terminalCase struct {
	command  string // the line at "$ " or "DBG> ", as typed
	library  string
	words    string
	width    int
	prompt   bool
	debugger bool // the debugger's HELP: the words are looked up under DEBUG
}

// TestHelp90Terminal holds Session to round 2's terminal session
// (testdata/help90/vax/round2/help90-term.log), with the terminal's part
// taken out: the escape sequences (the screen clears of /PAGE, which DCL's
// HELP sends, subtask 6) are removed, a line longer than the terminal is
// wrapped as the terminal driver wraps it, the prompt and the response
// typed after it are one line, and Ctrl/Z is echoed " Exit ". "?" is
// answered at once at a VMS terminal, without a Return, so its line ends
// where the screen is cleared, which comes to the same lines.
func TestHelp90Terminal(t *testing.T) {
	libs := help90Libraries(t)

	data, err := os.ReadFile(filepath.Join(help90Dir, "vax", "round2", "help90-term.log"))
	if err != nil {
		t.Fatal(err)
	}

	log := terminalLines(string(data))

	cases := []terminalCase{
		{command: "$ hp", library: "H90A", width: 80, prompt: true},
		{command: "$ hy deeper", library: "H90Y", words: "deeper", width: 80, prompt: true},
		{command: "$ hy deeper l2 l3", library: "H90Y", words: "deeper l2 l3", width: 80, prompt: true},
		{command: "$ hp alpha", library: "H90A", words: "alpha", width: 80, prompt: true},
		{command: "$ hp nosuch", library: "H90A", words: "nosuch", width: 80, prompt: true},
		{command: "$ hp/nopage alpha ", library: "H90A", words: "alpha", width: 80, prompt: true},
		{command: "$ hp/page texty", library: "H90A", words: "texty", width: 80, prompt: true},
		{command: "$ hp/noprompt wide", library: "H90A", words: "wide", width: 132},
		{command: "DBG> help est", library: "H90D", words: "est", width: 80, prompt: true, debugger: true},
		{command: "DBG> help set", library: "H90D", words: "set", width: 80, prompt: true, debugger: true},
	}

	at := 0
	ran := 0

	for _, c := range cases {
		// Each command's lines run to the next line at "$" or "DBG>". The
		// session typed "hp alpha" twice; each is a case.
		start := at
		for start < len(log) && log[start] != c.command {
			start++
		}

		if start == len(log) {
			t.Fatalf("%q not in the log", c.command)
		}

		end := start + 1
		for end < len(log) && !strings.HasPrefix(log[end], "$") && !strings.HasPrefix(log[end], "DBG>") {
			end++
		}

		want := log[start+1 : end]
		at = end

		// The responses are what follows each prompt.
		var answers []string

		for _, line := range want {
			if i := strings.Index(line, "Topic? "); i >= 0 && (i == 0 || strings.HasSuffix(line[:i], " ")) {
				answers = append(answers, line[i+len("Topic? "):])
			} else if i := strings.Index(line, "Subtopic? "); i >= 0 {
				answers = append(answers, line[i+len("Subtopic? "):])
			}
		}

		var got []string

		s := &Session{
			Library: libs[c.library],
			Open:    openProbeLibrary(libs),
			Write: func(line string) {
				for len(line) > c.width {
					got = append(got, line[:c.width])
					line = line[c.width:]
				}

				got = append(got, line)
			},
			Read: func(prompt string) (string, error) {
				if len(answers) == 0 {
					t.Fatalf("%s: HELP prompted %q after the log's last response", c.command, prompt)
				}

				a := answers[0]
				answers = answers[1:]
				got = append(got, prompt+a)

				if a == " Exit " {
					return "", io.EOF
				}

				return a, nil
			},
			Width:   c.width,
			Options: Options{Instructions: true},
			Prompt:  c.prompt,
		}

		words := Split(c.words)
		if c.debugger {
			words = append([]string{"DEBUG"}, words...)
		}

		if err := s.Run(words); err != nil {
			t.Fatalf("%s: %v", c.command, err)
		}

		if len(answers) > 0 {
			t.Errorf("%s: HELP ended with %d responses left", c.command, len(answers))
		}

		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s\n%s", c.command, lineDiff(want, got))
		}

		ran++
	}

	t.Logf("%d sessions compared", ran)
}

// terminalLines turns a terminal log into lines: escape sequences, NULs,
// and carriage returns are taken out, and a backspace erases the
// character before it (the terminal's "\b \b").
func terminalLines(text string) []string {
	var b []byte

	for i := 0; i < len(text); i++ {
		c := text[i]

		switch {
		case c == 0x1b && i+1 < len(text):
			i++

			switch text[i] {
			case '[':
				for i+1 < len(text) && (text[i+1] < 0x40 || text[i+1] > 0x7e) {
					i++
				}

				i++
			case '(', ')':
				i++
			}
		case c == '\b':
			if len(b) > 0 && b[len(b)-1] != '\n' {
				b = b[:len(b)-1]
			}
		case c == 0, c == '\r', c == 0x02:
		default:
			b = append(b, c)
		}
	}

	return strings.Split(string(b), "\n")
}
