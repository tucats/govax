package console

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 116's probe of DIFFERENCES (docs/planning/PHASE-116 - DIFFERENCES.md;
// testdata/dif116's README). cases.txt is the whole probe: every input file's
// records and every case. dif116Volume makes the probe's volume as
// exchange.cmd does, from cases.txt, and runDif116 runs a set of its cases on
// govax as DIF116.COM runs them on VMS, returning each case's part of the log
// and its listing. The oracle tests hold those to VMS's (vax/dif116.log,
// vax/dif116b.log, and vax/out).

// dif116Dir is the probe's directory.
func dif116Dir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "dif116"))
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

// dif116File is one input file of cases.txt: its name, its FORMAT, and its
// records.
type dif116File struct {
	Name, Format string
	Records      []string
}

// dif116Case is one case of cases.txt.
type dif116Case struct {
	ID, Section    string
	File           bool // its listing is [DIF116.OUT]ID.DIF; else the log
	Title, Command string
	Pre, Post      []string
}

// readDif116Cases reads cases.txt.
func readDif116Cases(t *testing.T) ([]*dif116File, []*dif116Case) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dif116Dir(t), "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}

	var (
		files []*dif116File
		cases []*dif116Case
		file  *dif116File
		cas   *dif116Case
	)

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "file "):
			name, format, _ := strings.Cut(strings.TrimPrefix(line, "file "), " ")
			file, cas = &dif116File{Name: name, Format: format}, nil
			files = append(files, file)
		case strings.HasPrefix(line, "|"):
			file.Records = append(file.Records, dif116Unescape(line[1:]))
		case strings.HasPrefix(line, "=lines "):
			// A big file: "=lines PREFIX N", records "PREFIX 1" to "PREFIX N".
			f := strings.Fields(line)
			n, _ := strconv.Atoi(f[2])

			for i := 1; i <= n; i++ {
				file.Records = append(file.Records, fmt.Sprintf("%s %d", f[1], i))
			}
		case strings.HasPrefix(line, "=at "):
			f := strings.SplitN(line, " ", 3)
			k, _ := strconv.Atoi(f[1])
			file.Records[k-1] = dif116Unescape(f[2])
		case strings.HasPrefix(line, "case "):
			f := strings.Fields(line)
			cas = &dif116Case{ID: f[1], Section: f[2], File: f[3] == "file"}
			cases = append(cases, cas)
		default:
			key, value, _ := strings.Cut(strings.TrimSpace(line), " ")

			switch key {
			case "title":
				cas.Title = value
			case "command":
				cas.Command = value
			case "pre":
				cas.Pre = append(cas.Pre, value)
			case "post":
				cas.Post = append(cas.Post, value)
			default:
				t.Fatalf("cases.txt: %q", line)
			}
		}
	}

	return files, cases
}

// dif116Unescape undoes cases.txt's escapes: \\, \t, and \xNN.
func dif116Unescape(s string) string {
	var b strings.Builder

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])

			continue
		}

		switch s[i+1] {
		case 't':
			b.WriteByte('\t')
			i++
		case 'x':
			v, _ := strconv.ParseUint(s[i+2:i+4], 16, 8)
			b.WriteByte(byte(v))
			i += 3
		default:
			b.WriteByte(s[i+1])
			i++
		}
	}

	return b.String()
}

// dif116Volume makes the probe's volume as exchange.cmd makes it, each
// input file written to the host as gen.go writes in/ and copied by
// exchange.cmd's own line, and mounts it on DUA1 with DIF$TOP and DIF$OUT
// defined and the default directory [DIF116.IN], as DIF116.COM leaves them.
func dif116Volume(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)
	dir := dif116Dir(t)
	files, _ := readDif116Cases(t)

	in := t.TempDir()

	for i, f := range files {
		var b strings.Builder

		for _, r := range f.Records {
			b.WriteString(r)

			if !strings.HasPrefix(f.Format, "FIX") {
				b.WriteString("\n")
			}
		}

		if err := os.WriteFile(filepath.Join(in, fmt.Sprintf("%04d", i+1)), []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	volume := filepath.Join(t.TempDir(), "dif116.dsk")
	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+volume+`" /DEVICE=RD53 DIF116`)

	if err := c.cmd().Mount("DUA5:", volume, true); err != nil {
		t.Fatal(err)
	}

	script, err := os.ReadFile(filepath.Join(dir, "exchange.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	paths := strings.NewReplacer(`"testdata/dif116/in/`, `"`+in+`/`, `"testdata/dif116/`, `"`+dir+`/`)

	buf.Reset()

	for line := range strings.SplitSeq(string(script), "\n") {
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "INITIALIZE/") ||
			strings.HasPrefix(line, "MOUNT/") || strings.HasPrefix(line, "DISMOUNT") ||
			strings.HasPrefix(line, "DIRECTORY") {
			continue
		}

		line = paths.Replace(line)

		if err := d.Dispatch(line); err != nil || buf.Len() != 0 {
			t.Fatalf("%s returned %v and said:\n%.600s", line, err, buf.String())
		}
	}

	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	shareFindRun(t, d, "DISMOUNT DUA5")
	shareFindRun(t, d, "DISMOUNT DUA1")

	if err := c.cmd().Mount("DUA1:", volume, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "DEFINE DIF$TOP DUA1:")
	shareFindRun(t, d, "DEFINE DIF$OUT DIF$TOP:[DIF116.OUT]")
	shareFindRun(t, d, "SET DEFAULT DIF$TOP:[DIF116.IN]")

	return d, c, buf
}

// dif116Run is one case's outcome: its part of the log, after its "####
// CASE" line, and its listing's records, if it made one.
type dif116Run struct {
	Log     []string
	Listing []string
	Made    bool
}

// runDif116 runs cases as DIF116.COM runs them (round 2's form: the status
// written as the string it is), in one procedure with its log on the volume,
// and returns each case's outcome by its ID.
func runDif116(t *testing.T, cases []*dif116Case) map[string]*dif116Run {
	t.Helper()

	d, c, buf := dif116Volume(t)

	var b strings.Builder

	b.WriteString("$ SET NOON\n$ SET VERIFY\n")

	for _, cas := range cases {
		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"#### CASE %s: %s\"\n", cas.ID, strings.ReplaceAll(cas.Title, `"`, `""`))

		for _, p := range cas.Pre {
			fmt.Fprintf(&b, "$ %s\n", p)
		}

		command := cas.Command
		if cas.File {
			verb, rest, _ := strings.Cut(command, " ")
			command = verb + "/OUTPUT=DIF$OUT:" + cas.ID + ".DIF " + rest
		}

		fmt.Fprintf(&b, "$ %s\n$ S = $STATUS\n$ WRITE SYS$OUTPUT \"STATUS \", S, \" \", F$MESSAGE(S)\n", command)

		for _, p := range cas.Post {
			fmt.Fprintf(&b, "$ %s\n", p)
		}
	}

	b.WriteString("$ SET DEFAULT DIF$TOP:[DIF116.IN]\n$ EXIT\n")

	host := filepath.Join(t.TempDir(), "run.com")
	if err := os.WriteFile(host, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, `COPY "`+host+`"/HOST DIF$TOP:[DIF116]RUN.COM`)
	buf.Reset()
	shareFindRun(t, d, "@DIF$TOP:[DIF116]RUN/OUTPUT=DIF$TOP:[DIF116]RUN.LOG")

	s := c.ContainerSession

	read := func(spec string) ([]string, bool) {
		loc, err := s.Locate(spec, false)
		if err != nil {
			t.Fatal(err)
		}

		records, _, err := s.ReadRecordFile(loc, rms.TextRecords)
		if err != nil {
			return nil, false
		}

		lines := make([]string, len(records))
		for i, r := range records {
			lines[i] = string(r)
		}

		return lines, true
	}

	log, ok := read("DIF$TOP:[DIF116]RUN.LOG")
	if !ok {
		t.Fatal("no RUN.LOG")
	}

	runs := dif116LogCases(log)

	for _, cas := range cases {
		r := runs[cas.ID]
		if r == nil {
			t.Fatalf("case %s isn't in the log:\n%s", cas.ID, strings.Join(log, "\n"))
		}

		if cas.File {
			r.Listing, r.Made = read("DIF$TOP:[DIF116.OUT]" + cas.ID + ".DIF")
		}
	}

	return runs
}

// dif116LogCases splits a log by its "#### CASE id:" lines, each case's
// lines with the command lines ("$ ...", and their continuations) left
// out and any CR removed.
func dif116LogCases(lines []string) map[string]*dif116Run {
	runs := map[string]*dif116Run{}

	var (
		cur          *dif116Run
		continuation bool
	)

	for _, line := range lines {
		line = strings.ReplaceAll(line, "\r", "")

		if rest, ok := strings.CutPrefix(line, "#### CASE "); ok {
			id, _, _ := strings.Cut(rest, ":")
			cur = &dif116Run{}
			runs[id] = cur
			continuation = false

			continue
		}

		if cur == nil {
			continue
		}

		if strings.HasPrefix(line, "$ ") || continuation {
			continuation = strings.HasSuffix(line, "-")

			continue
		}

		if strings.HasPrefix(line, "######## ") {
			cur = nil

			continue
		}

		cur.Log = append(cur.Log, line)
	}

	return runs
}

// readVMSDif116 reads VMS's outcome of each case: both rounds' logs, and the
// listings.
func readVMSDif116(t *testing.T, cases []*dif116Case) map[string]*dif116Run {
	t.Helper()

	dir := filepath.Join(dif116Dir(t), "vax")
	runs := map[string]*dif116Run{}

	// Round 1 stopped in N60, which round 2 ran from the start.
	for _, name := range []string{"dif116.log", "dif116b.log"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		for id, r := range dif116LogCases(strings.Split(string(data), "\n")) {
			if _, ok := runs[id]; !ok || name == "dif116b.log" {
				runs[id] = r
			}
		}
	}

	for _, cas := range cases {
		r := runs[cas.ID]
		if r == nil || !cas.File {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, "out", cas.ID+".DIF"))
		if err == nil {
			r.Listing = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			r.Made = true
		}
	}

	return runs
}

// dif116Status masks govax's status lines where VMS's are round 1's: its
// F$FAO printed the address of the string $STATUS (%X7FFEC964), so only
// F$MESSAGE's text says what the status was.
func dif116Status(got, want []string) []string {
	round1 := false

	for _, line := range want {
		if strings.HasPrefix(line, "STATUS %X7FFEC964 ") {
			round1 = true
		}
	}

	if !round1 {
		return got
	}

	out := make([]string, len(got))

	for i, line := range got {
		if rest, ok := strings.CutPrefix(line, "STATUS %X"); ok && len(rest) > 8 {
			line = "STATUS %X7FFEC964" + rest[8:]
		}

		out[i] = line
	}

	return out
}

// dif116Later are the cases of a section an oracle test holds whose listings
// a later subtask writes: only their messages and status are compared.
var dif116Later = map[string]string{}

// dif116IsLater says a case's listing is a later subtask's.
func dif116IsLater(id string) bool {
	_, later := dif116Later[id]

	return later
}

// dif116NotCompared are cases an oracle test leaves out.
var dif116NotCompared = map[string]string{
	"N29": "two directory files' records are the volume's own: govax's directories differ from the VAX's",
	"O20": "its DIRECTORY/FULL and DUMP show file IDs and times",
}

// dif116Secondary is a message's second line: "-RMS-E-FNF, ...".
var dif116Secondary = regexp.MustCompile(`^-[A-Z$]+-[A-Z]-[A-Z]+,`)

// dif116Messages keeps a case's messages and status lines only.
func dif116Messages(lines []string) []string {
	var out []string

	for _, line := range lines {
		if strings.HasPrefix(line, "%") || dif116Secondary.MatchString(line) || strings.HasPrefix(line, "\t") ||
			strings.HasPrefix(line, " \\") || strings.HasPrefix(line, "STATUS ") {
			out = append(out, line)
		}
	}

	return out
}

// holdDif116 runs the cases of sections and holds each to VMS's: its log
// lines (status masked), and its listing.
func holdDif116(t *testing.T, sections ...string) {
	t.Helper()

	_, all := readDif116Cases(t)

	var cases []*dif116Case

	for _, cas := range all {
		if _, skip := dif116NotCompared[cas.ID]; skip {
			continue
		}

		for _, s := range sections {
			if cas.Section == s {
				cases = append(cases, cas)
			}
		}
	}

	got := runDif116(t, cases)
	want := readVMSDif116(t, cases)

	for _, cas := range cases {
		g, w := got[cas.ID], want[cas.ID]
		if w == nil {
			t.Errorf("%s: not in VMS's logs", cas.ID)

			continue
		}

		gl, wl := dif116Status(g.Log, w.Log), w.Log
		if dif116IsLater(cas.ID) {
			gl, wl = dif116Messages(gl), dif116Messages(wl)
		}

		if a, b := strings.Join(gl, "\n"), strings.Join(wl, "\n"); a != b {
			t.Errorf("%s (%s): the log differs\ngovax:\n%s\nVMS:\n%s", cas.ID, cas.Title, a, b)
		}

		if !cas.File {
			continue
		}

		if g.Made != w.Made {
			t.Errorf("%s (%s): govax made a listing %v, VMS %v", cas.ID, cas.Title, g.Made, w.Made)

			continue
		}

		if dif116IsLater(cas.ID) {
			continue
		}

		if a, b := strings.Join(g.Listing, "\n"), strings.Join(w.Listing, "\n"); a != b {
			t.Errorf("%s (%s): the listing differs\ngovax:\n%s\nVMS:\n%s", cas.ID, cas.Title, a, b)
		}
	}
}

// TestDif116NamesOracle holds the command's frame to VMS's (subtask 3): the
// files and their names, the errors, the qualifiers' values, the echo, and
// where /OUTPUT puts the listing (the NAMES and OUTPUT sections, but for O20,
// whose DIRECTORY/FULL and DUMP show the volume's file IDs and times).
func TestDif116NamesOracle(t *testing.T) {
	t.Parallel()

	holdDif116(t, "NAMES", "OUTPUT")
}

// TestDif116MergedOracle holds the comparison and the merged form to VMS's
// (subtask 4): the layout, the matching rule with /MATCH and /WINDOW,
// /MAXIMUM_DIFFERENCES, and the 570 generated pairs, each case's messages,
// status, and listing, echo and all. (internal/diff's TestProbeListings holds
// the listings without the command, faster.)
func TestDif116MergedOracle(t *testing.T) {
	t.Parallel()

	holdDif116(t, "LAYOUT", "MATCHING", "LIMITS", "GENERATED")
}

// TestDif116FormsOracle holds the other listing forms to VMS's (subtask 5):
// /SEPARATED, /PARALLEL, /CHANGE_BAR on a file, /NONUMBER, /WIDTH, and
// several forms together, and /MODE's and /SLP's listings (subtask 6): the
// FORMS section.
func TestDif116FormsOracle(t *testing.T) {
	t.Parallel()

	holdDif116(t, "FORMS")
}

// TestDif116IgnoringOracle holds /IGNORE and /COMMENT_DELIMITER to VMS's
// (subtask 7): each keyword alone and together, EDITED, EXACT, and PRETTY,
// HEADER's pages, comment characters given and by each file type, and what
// the echo shows of them: the IGNORING section.
func TestDif116IgnoringOracle(t *testing.T) {
	t.Parallel()

	holdDif116(t, "IGNORING")
}

// TestDif116FormatsOracle holds records that aren't text to VMS's (subtask
// 8): VFC, STREAM_LF, and fixed files compared by their records, and every
// byte from 0 to %XFF in each form: the FORMATS section.
func TestDif116FormatsOracle(t *testing.T) {
	t.Parallel()

	holdDif116(t, "FORMATS")
}

// dif116TerminalSkipped are the terminal log's commands the terminal oracle
// doesn't replay, and why.
var dif116TerminalSkipped = map[string]string{
	"WIDTH = F$GETDVI(\"TT\",\"DEVBUFSIZ\")": "the procedure's own bookkeeping",
	"SET TERMINAL/WIDTH='WIDTH'":             "the procedure's own bookkeeping",
	"SET TERMINAL/WIDTH=177":                 "the procedure's own bookkeeping",
	"SET NOON":                               "the procedure's own bookkeeping",
	"SET VERIFY":                             "the procedure's own bookkeeping",
	"SET NOVERIFY":                           "the procedure's own bookkeeping",
	"DIFFERENCES":                            "its prompts are a procedure's INSFPRM, or a person's answers (TestDif116NamesOracle)",
	"differences":                            "its prompts are a person's answers",
	"DIFFERENCES I30.A I30.B":                "the log shows what the terminal did with the records' control characters",
}

// TestDif116TerminalOracle replays DIF116T.COM's session at a terminal
// (testdata/dif116/vax/terminal.log, subtask 8): the listing at the
// terminal's width of 80 and of 132 columns, /WIDTH ignored there but
// shown in the echo, and /OUTPUT naming the terminal, which takes /WIDTH.
// The paging qualifiers VMS 7.1 has are left out (Decision 1).
func TestDif116TerminalOracle(t *testing.T) {
	t.Parallel()

	text, err := os.ReadFile(filepath.Join(dif116Dir(t), "vax", "terminal.log"))
	if err != nil {
		t.Fatal(err)
	}

	type exchange struct {
		command string
		output  []string
	}

	var session []*exchange

	for _, line := range strings.Split(strings.ReplaceAll(string(text), "\r", ""), "\n") {
		if rest, ok := strings.CutPrefix(line, "$ "); ok {
			session = append(session, &exchange{command: strings.TrimSpace(rest)})

			continue
		}

		if len(session) > 0 && !strings.HasPrefix(line, "_File ") {
			last := session[len(session)-1]
			last.output = append(last.output, line)
		}
	}

	d, _, buf := dif116Volume(t)
	shareFindRun(t, d, "SET DEFAULT DIF$TOP:[DIF116]")

	ran, width, skipped := 0, 0, false

	for _, x := range session {
		if x.command == "" || strings.HasPrefix(x.command, "!") || strings.HasPrefix(x.command, "@") {
			continue
		}

		// A skipped command's SHOW SYMBOL $STATUS goes with it.
		if _, skip := dif116TerminalSkipped[x.command]; skip || skipped && x.command == "SHOW SYMBOL $STATUS" {
			skipped = skip

			continue
		}

		skipped = false

		if n, ok := strings.CutPrefix(x.command, "SET TERMINAL/WIDTH="); ok {
			width, _ = strconv.Atoi(n)
		}

		// The paging qualifiers are VMS 7.1's, not govax's (Decision 1).
		if strings.Contains(x.command, "/PAGE") || strings.Contains(x.command, "/SEARCH") ||
			strings.Contains(x.command, "/WRAP") || strings.Contains(x.command, "/HIGHLIGHT") ||
			strings.Contains(x.command, "/EXACT") {
			break
		}

		buf.Reset()

		if err := d.Dispatch(x.command); err != nil && !vmserrors.MessageInhibited(err) {
			t.Errorf("%s: %v", x.command, err)

			continue
		}

		ran++

		got := strings.TrimRight(dif116Wrap(buf.String(), width), "\n")
		want := strings.TrimRight(strings.Join(x.output, "\n"), "\n")

		if got != want {
			t.Errorf("%s: the output differs\ngovax:\n%s\nVMS:\n%s", x.command, got, want)
		}
	}

	if ran < 20 {
		t.Errorf("only %d commands replayed", ran)
	}
}

// dif116Wrap is text as a terminal of width columns set /WRAP shows it, a
// line longer than the width going on at the next line's start: what VMS's
// terminal did with /OUTPUT=TT:'s 132-column records on 80 columns.
func dif116Wrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var b strings.Builder

	for line := range strings.SplitSeq(text, "\n") {
		for len(line) > width {
			b.WriteString(line[:width] + "\n")
			line = line[width:]
		}

		b.WriteString(line + "\n")
	}

	return strings.TrimSuffix(b.String(), "\n")
}
