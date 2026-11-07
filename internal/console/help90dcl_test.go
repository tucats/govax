package console

import (
	"bytes"
	"os"
	"regexp"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// help90DCLLeft are the commands of HELP90.COM's parts B, B2, and C govax
// doesn't run as VMS 7.1 did, by a piece of the command's text:
//
//   - LIBRARY=[]H90A: VMS's SYS$HELP is SYS$COMMON:[SYSHLP], a rooted
//     directory, so "[]H90A" was SYS$COMMON:[000000]H90A.HLB; govax has no
//     rooted directories and no SYS$HELP on the test's volume.
var help90DCLLeft = []string{"LIBRARY=[]H90A"}

// help90FileID is DIRECTORY/FULL's file ID, which the run decides.
var help90FileID = regexp.MustCompile(`File ID:  \(\d+,\d+,\d+\)`)

// help90DCLPart is HELP90.COM's commands from the line starting with from up
// to the one starting with to, without help90DCLLeft's.
func help90DCLPart(lines []string, from, to string) []string {
	var (
		out      []string
		in       bool
		skipping bool
	)

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, from):
			in = true
		case !in:
			continue
		case strings.HasPrefix(line, to):
			return out
		}

		if strings.HasPrefix(line, "$ ") {
			skipping = false

			for _, s := range help90DCLLeft {
				if strings.Contains(line, s) {
					skipping = true
				}
			}
		}

		if !skipping {
			out = append(out, line)
		}
	}

	return out
}

// TestHelp90DCLOracle runs DCL's HELP on the probe's libraries as HELP90.COM
// runs it: part B's requests that name the library in other ways, part B2
// (/OUTPUT), part C (the user libraries in the process, group, and system
// tables, /USERLIBRARY, /LIBLIST, @library, /NOLIBRARY), and part D
// (HELP90D.COM: prompts answered by the procedure's data lines and by a file
// SYS$INPUT names), all in a procedure whose /OUTPUT is a file, as on the
// VAX. The libraries are part A's, made by govax's LIBRARY. What the
// procedure writes is held to VMS 7.1's log line for line, file IDs and
// times masked.
func TestHelp90DCLOracle(t *testing.T) {
	d, c, buf := help90Volume(t)

	// Part A makes the libraries.
	partA := filepath.Join(t.TempDir(), "parta.com")
	writeFile(t, partA, help90PartA(t))

	com := readHelp90Lines(t, help90Dir+"help90.com")
	logLines := readHelp90Lines(t, help90Dir+"vax/round1b/help90.log")

	var proc []string

	proc = append(proc, "$ SET NOON", "$ SET VERIFY",
		`$ L = F$ENVIRONMENT("DEFAULT")`, "$ DEFINE H90DIR 'L'",
		"$ H := HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90A")
	proc = append(proc, help90DCLPart(com, "$ ! The library's name and type.", "$ ! Part E")...)
	proc = append(proc, "$ @HELP90D", "$ EXIT")

	partC := filepath.Join(t.TempDir(), "partc.com")
	writeFile(t, partC, strings.Join(proc, "\n")+"\n")

	for _, line := range []string{
		`COPY "` + partA + `"/HOST PARTA.COM`,
		"@PARTA",
		`COPY "` + partC + `"/HOST PARTC.COM`,
		`COPY "` + mustAbs(t, help90Dir+"help90d.com") + `"/HOST HELP90D.COM`,
		`COPY "` + mustAbs(t, help90Dir+"resp.txt") + `"/HOST RESP.TXT`,
	} {
		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v\n%s", line, err, buf)
		}
	}

	// The VAX ran the probe with its output a file, 132 columns wide; govax's
	// DCL writes its messages at the terminal rather than in a procedure's
	// /OUTPUT file, so the procedure runs at a terminal that wide.
	c.ScreenSize = func() (int, int) { return 66, 132 }

	buf.Reset()

	if err := d.Dispatch("@PARTC"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PARTC: %v\n%s", err, buf)
	}

	got := strings.Split(strings.TrimSuffix(strings.ReplaceAll(buf.String(), "\r", ""), "\n"), "\n")

	// Our procedure's own first lines, and its EXIT.
	got = help90DCLPart(got, "$ ! The library's name and type.", "\x00")

	want := help90DCLPart(logLines, "$ ! The library's name and type.", "$ ! Part E")
	want = append(want, help90DCLPart(logLines, "$ @HELP90D", "$ ! The libraries, for comparing")...)

	// The log's part D ends with HELP90D's EXIT and the comment lines
	// after it in HELP90.COM; ours with HELP90D's EXIT and PARTC's.
	for len(want) > 0 && strings.HasPrefix(want[len(want)-1], "$ !") {
		want = want[:len(want)-1]
	}

	// PARTC's own EXIT.
	if len(got) > 0 && got[len(got)-1] == "$ EXIT" {
		got = got[:len(got)-1]
	}

	got, want = for83Mask(got), for83Mask(want)

	for _, lines := range [][]string{got, want} {
		for i, line := range lines {
			lines[i] = help90FileID.ReplaceAllString(line, "File ID:  (n,n,n)")
		}
	}

	for i := 0; i < len(got) || i < len(want); i++ {
		g, w := "<end>", "<end>"

		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g != w {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
		}
	}
}

func readHelp90Lines(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n")
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	return abs
}

// help90DCLVolume is help90Volume with libraries H90A, H90B, and H90C made
// from the probe's modules, and H90DIR naming the volume's [000000].
func help90DCLVolume(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c, buf := help90Volume(t)

	for _, line := range []string{
		"LIBRARY/CREATE/HELP H90A H90A1,H90A2",
		"LIBRARY/CREATE/HELP H90B H90B",
		"LIBRARY/CREATE/HELP H90C H90C",
		"DEFINE H90DIR DUA1:[000000]",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, buf)
		}
	}

	c.ScreenSize = func() (int, int) { return 24, 80 }

	buf.Reset()

	return d, c, buf
}

// TestHelp90DCLTerminal is HELP/PROMPT at the terminal: each prompt is
// written there and the response read after it; Return goes up a level, and
// at "Topic?" ends HELP (round 2's terminal session, which internal/help's
// TestHelp90Terminal holds the Session to). A Ctrl/Z ends it at once.
func TestHelp90DCLTerminal(t *testing.T) {
	d, c, buf := help90DCLVolume(t)

	c.In = strings.NewReader("Parameters\n\n\n")

	if err := d.Dispatch("HELP/NOUSERLIBRARY/LIBRARY=H90DIR:H90A ALPHA"); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{
		"\nALPHA\n\n     Text of ALPHA, level 1.",
		"ALPHA Subtopic? \nALPHA\n\n  Parameters\n",
		"ALPHA Subtopic? Topic? ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}

	if strings.Count(out, "Subtopic? ") != 2 || strings.Count(out, "\nTopic? ")+strings.Count(out, " Topic? ") != 1 {
		t.Errorf("the prompts:\n%s", out)
	}

	// Ctrl/Z ends HELP at a prompt; the input after it is DCL's again.
	buf.Reset()
	c.In = strings.NewReader("\x1a")

	if err := d.Dispatch("HELP/NOUSERLIBRARY/LIBRARY=H90DIR:H90A ALPHA"); err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(buf.String(), "ALPHA Subtopic? ") {
		t.Errorf("Ctrl/Z: %q", buf.String())
	}
}

// TestHelp90DCLMessages: the qualifiers that wait for a later phase, no
// library at all, and a library whose file isn't a help library.
func TestHelp90DCLMessages(t *testing.T) {
	d, _, buf := help90DCLVolume(t)

	if err := d.Dispatch("LIBRARY/CREATE/TEXT H90T T1,T2"); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ line, want string }{
		{"HELP/NOPROMPT/SEARCH=X", "HELP /SEARCH is not implemented yet"},
		{"HELP/NOPROMPT/EXACT ALPHA", "HELP /EXACT is not implemented yet"},
		{"HELP/NOPROMPT/PAGE=SAVE=4 ALPHA", "HELP /PAGE=SAVE is not implemented yet"},
		{"HELP/NOPROMPT/NOLIBRARY/NOUSERLIBRARY ALPS", "%LBR-E-NOHLPLIBS, no default help libraries can be opened"},
		{"HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90T.TLB ALPS",
			"HELP-E-OPENIN, error opening DUA1:[000000]H90T.TLB;1 as input\n-LBR-E-NOTHLPLIB, not a HELP library"},
	}

	for _, tc := range cases {
		buf.Reset()

		err := d.Dispatch(tc.line)

		text := buf.String()
		if err != nil {
			text += err.Error()
		}

		if !strings.Contains(text, tc.want) {
			t.Errorf("%s: %q, want %q", tc.line, text, tc.want)
		}
	}
}

// TestHelp90DCLPage is HELP/PAGE: the screen is cleared before each display
// (CLEAR_SCREEN), or not (SCROLL), and a display longer than the screen
// stops for RETURN.
func TestHelp90DCLPage(t *testing.T) {
	d, c, buf := help90DCLVolume(t)

	const clear = "\x1b[H\x1b[2J"

	if err := d.Dispatch("HELP/NOPROMPT/PAGE/NOUSERLIBRARY/LIBRARY=H90DIR:H90A ALPS"); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(buf.String(), clear+"\nALPS\n") || !strings.HasSuffix(buf.String(), "\x1b[m") {
		t.Errorf("/PAGE: %q", buf.String())
	}

	buf.Reset()

	if err := d.Dispatch("HELP/NOPROMPT/PAGE=SCROLL/NOUSERLIBRARY/LIBRARY=H90DIR:H90A ALPS"); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(buf.String(), "\x1b") {
		t.Errorf("/PAGE=SCROLL: %q", buf.String())
	}

	// MANY's list is longer than a screen of 10 lines: RETURN shows the
	// rest, other text ends the display.
	c.ScreenSize = func() (int, int) { return 10, 80 }
	c.In = strings.NewReader("\n")
	buf.Reset()

	if err := d.Dispatch("HELP/NOPROMPT/PAGE/NOUSERLIBRARY/LIBRARY=H90DIR:H90A MANY..."); err != nil {
		t.Fatal(err)
	}

	if strings.Count(buf.String(), "Press RETURN to continue") < 1 {
		t.Errorf("/PAGE didn't stop:\n%s", buf.String())
	}

	buf.Reset()
	c.In = strings.NewReader("q\n")

	if err := d.Dispatch("HELP/NOPROMPT/PAGE/NOUSERLIBRARY/LIBRARY=H90DIR:H90A MANY..."); err != nil {
		t.Fatal(err)
	}

	if n := strings.Count(buf.String(), "\n"); n > 11 {
		t.Errorf("/PAGE went on after q: %d lines\n%s", n, buf.String())
	}
}

// TestHelp90DCLBuiltIn: with no /LIBRARY and no SYS$HELP:HELPLIB.HLB, HELP
// reads govax's own HELPLIB.HLB, built into it: its HELP topic, the list
// of its topics, and the Topic? prompt.
func TestHelp90DCLBuiltIn(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	if err := d.Dispatch("HELP"); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"govax is a VAX emulator", "Additional information available:", "CONFIGURATION", "Topic? "} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("HELP lacks %q: %q", want, buf.String())
		}
	}
}

// TestHelp90DCLSubprocess is HELP in a subprocess's DCL, with the
// subprocess's own user libraries: SPAWN copies the process's logical
// names, HLP$LIBRARY among them.
func TestHelp90DCLSubprocess(t *testing.T) {
	d, _, buf := shareFindConsole(t, false)

	commands := []string{"SET DEFAULT DUA1:[000000]"}
	for _, name := range []string{"h90a1.hlp", "h90a2.hlp", "h90b.hlp"} {
		commands = append(commands, `COPY "`+mustAbs(t, help90Dir+name)+`"/HOST `+strings.ToUpper(name))
	}

	commands = append(commands,
		"LIBRARY/CREATE/HELP H90A H90A1,H90A2",
		"LIBRARY/CREATE/HELP H90B H90B",
		"DEFINE H90DIR DUA1:[000000]",
		"DEFINE HLP$LIBRARY DUA1:[000000]H90B")

	for _, line := range commands {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, buf)
		}
	}

	buf.Reset()

	if err := d.Dispatch("SPAWN HELP/NOPROMPT/LIBRARY=H90DIR:H90A BONLY"); err != nil {
		t.Fatalf("SPAWN: %v\n%s", err, buf)
	}

	if !strings.Contains(buf.String(), "Text of BONLY, in library H90B.") {
		t.Errorf("SPAWN HELP: %q", buf.String())
	}
}
