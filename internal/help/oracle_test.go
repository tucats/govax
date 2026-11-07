package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/lbr"
)

var help90Dir = filepath.Join("..", "..", "testdata", "help90")

// help90Libraries returns the probe's help libraries by name ("H90A"):
// those VMS 7.1 made in round 1b, from its save set, and H90Y, made from
// deeper.hlp as round 1c's LIBRARY/CREATE/HELP made it (internal/lbr's
// libraries are VMS's to the byte, TestHelp90NewLibraries).
func help90Libraries(t *testing.T) map[string]Library {
	t.Helper()

	f, err := os.Open(filepath.Join(help90Dir, "vax", "round1b", "h90.bck"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	set, err := backup.Read(f, info.Size(), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	libs := map[string]Library{}

	for _, file := range set.Files {
		name := strings.TrimPrefix(file.Name, "[000000]")
		if !strings.HasSuffix(name, ".HLB;1") && name != "H90R.HLB;2" {
			continue
		}

		l, err := lbr.Open(file.Data())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		libs[strings.TrimSuffix(name[:strings.Index(name, ";")], ".HLB")] = NewLBRLibrary(l)
	}

	b, err := lbr.Create(lbr.TypeHelp)
	if err != nil {
		t.Fatal(err)
	}

	mods, bad := b.HelpModules(fileRecords(t, filepath.Join(help90Dir, "deeper.hlp")))
	if bad != nil {
		t.Fatal(bad)
	}

	for _, m := range mods {
		if err := b.Insert(m); err != nil {
			t.Fatal(err)
		}
	}

	y, err := lbr.Open(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	libs["H90Y"] = NewLBRLibrary(y)

	return libs
}

// fileRecords reads a text file as its lines.
func fileRecords(t *testing.T, path string) [][]byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
}

// helpCase is one HELP command of a probe log and what VMS showed.
type helpCase struct {
	line    int    // the command's line in the log
	command string // as the log has it
	library string
	opts    Options
	words   string
	want    []string
	listed  bool // compare only up to the list of topics
}

// helpCases reads the HELP commands of a log between the lines starting
// with from and to. sym is the symbol defined by from's line, whose
// library is lib; a HELP command names its own library.
func helpCases(t *testing.T, path, from, to, sym, lib string) []helpCase {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")

	var (
		cases []helpCase
		in    bool
		c     *helpCase
	)

	for i, line := range lines {
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
			if c != nil {
				c.want = append(c.want, line)
			}

			continue
		}

		c = nil

		command := line[2:]

		verb, words, _ := strings.Cut(command, " ")
		if !strings.HasPrefix(verb, sym+"/") && verb != sym && !strings.HasPrefix(verb, "HELP/") {
			continue
		}

		hc := helpCase{line: i + 1, command: line, library: lib, opts: Options{Instructions: true}, words: words}

		for _, q := range strings.Split(verb, "/")[1:] {
			switch {
			case q == "INSTRUCTIONS":
				hc.opts.Instructions = true
			case q == "NOINSTRUCTIONS":
				hc.opts.Instructions = false
			case strings.HasPrefix(q, "LIBRARY="):
				name := strings.TrimPrefix(q, "LIBRARY=")
				name = strings.TrimSuffix(strings.TrimPrefix(name, "H90DIR:"), ".HLB")
				hc.library = name
			}
		}

		cases = append(cases, hc)
		c = &cases[len(cases)-1]
	}

	t.Fatalf("%s: no line starting %q", path, to)

	return nil
}

// TestHelp90Oracle holds Lookup and Render to VMS 7.1's HELP: round 1b's
// part B (126 requests on the probe's libraries, testdata/help90/vax/
// round1b/help90.log) and round 1c's (DEEPER's levels, vax/round2/
// help90c.log), each run as HELP/NOPROMPT with its output in a file (132
// columns). A request naming a library that isn't a help library, or isn't
// there, is DCL's HELP's (subtask 6) and is passed over here.
func TestHelp90Oracle(t *testing.T) {
	libs := help90Libraries(t)

	var cases []helpCase
	cases = append(cases, helpCases(t, filepath.Join(help90Dir, "vax", "round1b", "help90.log"),
		"$ H := HELP", "$ ! Part B2", "H", "H90A")...)
	cases = append(cases, helpCases(t, filepath.Join(help90Dir, "vax", "round2", "help90c.log"),
		"$ H := HELP", "$ ! Prompting", "H", "H90Y")...)

	if len(cases) < 150 {
		t.Fatalf("only %d cases", len(cases))
	}

	compareHelp(t, libs, cases)
}

// compareHelp runs each case on its library, if there is one, and
// compares the output with VMS's.
func compareHelp(t *testing.T, libs map[string]Library, cases []helpCase) {
	t.Helper()

	ran := 0

	for _, c := range cases {
		lib, ok := libs[c.library]
		if !ok {
			continue
		}

		ran++

		var got []string

		displays, err := Lookup(lib, Split(c.words), c.opts)
		switch err {
		case nil:
			got = Render(displays, 132)
		case ErrInvalidKey:
			got = []string{"%LBR-E-INVKEY, invalid key"}
		default:
			t.Fatalf("line %d: %v", c.line, err)
		}

		want := c.want
		if c.listed {
			want, got = beforeList(want), beforeList(got)
		}

		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("line %d: %s\n%s", c.line, c.command, lineDiff(want, got))
		}
	}

	t.Logf("%d requests compared", ran)
}

// TestHelp90VMSModules holds Lookup and Render to VMS 7.1's HELP on its
// own EXCHANGE, SUBMIT, and ANALYZE modules: round 1's part G. Its log and
// VMS's HELPLIB.HLB hold DIGITAL's text, so they stay in testdata/disks/
// help90-vms/, which git ignores (the phase doc's Decision 9), and the
// test skips without them. HELPLIB.HLB is copied there from the local
// VMS 7.1 system disk:
//
//	MOUNT DUA5 "testdata/disks/rq0-ra92.dsk"
//	COPY DUA5:[VMS$COMMON.SYSHLP]HELPLIB.HLB "testdata/disks/help90-vms/HELPLIB.HLB"/HOST
func TestHelp90VMSModules(t *testing.T) {
	local := filepath.Join("..", "..", "testdata", "disks", "help90-vms")

	data, err := os.ReadFile(filepath.Join(local, "HELPLIB.HLB"))
	if err != nil {
		t.Skip("no local copy of VMS's HELPLIB.HLB")
	}

	logPath := filepath.Join(local, "help90v.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Skip("no local log of part G")
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	// The VAX's HELPLIB.HLB has modules the system disk's copy lacks
	// (DECnet-Plus, ON, RECALL, ...), so the two requests that list every
	// module are compared only up to their lists.
	cases := helpCases(t, logPath, "$ HV := HELP", "$ ! /LIBRARY without", "HV", "HELPLIB")
	for i, c := range cases {
		if c.command == "$ HV" || c.command == "$ HV NOSUCH_H90" {
			cases[i].listed = true
		}
	}

	libs := map[string]Library{"HELPLIB": NewLBRLibrary(l)}
	compareHelp(t, libs, cases)
}

// beforeList is lines up to the first "... information available:".
func beforeList(lines []string) []string {
	for i, line := range lines {
		if strings.HasSuffix(line, "information available:") {
			return lines[:i]
		}
	}

	return lines
}

// lineDiff shows two lists of lines side by side from the first that
// differs.
func lineDiff(want, got []string) string {
	first := 0
	for first < len(want) && first < len(got) && want[first] == got[first] {
		first++
	}

	var b strings.Builder

	for i := first; i < max(len(want), len(got)) && i < first+12; i++ {
		w, g := "<none>", "<none>"
		if i < len(want) {
			w = want[i]
		}

		if i < len(got) {
			g = got[i]
		}

		mark := " "
		if w != g {
			mark = "*"
		}

		b.WriteString(mark + " want |" + w + "|\n" + mark + " got  |" + g + "|\n")
	}

	return b.String()
}
