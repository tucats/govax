package console

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// Phase 117's probe of SUMSLP (docs/planning/PHASE-117 - SUMSLP.md;
// testdata/sum117's README). cases.txt is the whole probe: every input file's
// records and every case. sum117Volume makes the probe's volume as
// exchange.cmd and exchange2.cmd do, from cases.txt, and runSum117 runs a set
// of its cases on govax as SUM117.COM runs them on VMS, returning each case's
// part of the log and the files it left in [SUM117.OUT]. The oracle tests
// hold those to VMS's (vax/sum117.log, vax/sum117b.log, and vax/out).

// sum117Dir is the probe's directory.
func sum117Dir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "sum117"))
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

// sum117File is one input file of cases.txt: its name, its FORMAT, and its
// records.
type sum117File struct {
	Name, Format string
	Records      []string
}

// sum117Step is a case's step: a DCL line whose status the log shows, and
// the data lines after it.
type sum117Step struct {
	Command string
	Data    []string
}

// sum117Case is one case of cases.txt.
type sum117Case struct {
	ID, Section string
	Title       string
	Pre, Post   []string
	Steps       []*sum117Step
	Round2      bool
}

// readSum117Cases reads cases.txt.
func readSum117Cases(t *testing.T) ([]*sum117File, []*sum117Case) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(sum117Dir(t), "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}

	var (
		files  []*sum117File
		cases  []*sum117Case
		file   *sum117File
		cas    *sum117Case
		round2 bool
	)

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == "round 2":
			round2 = true
		case strings.HasPrefix(line, "file "):
			name, format, _ := strings.Cut(strings.TrimPrefix(line, "file "), " ")
			file, cas = &sum117File{Name: name, Format: format}, nil
			files = append(files, file)
		case strings.HasPrefix(line, "|"):
			file.Records = append(file.Records, dif116Unescape(line[1:]))
		case strings.HasPrefix(line, "=lines "):
			f := strings.Fields(line)
			n, _ := strconv.Atoi(f[2])

			for i := 1; i <= n; i++ {
				file.Records = append(file.Records, fmt.Sprintf("%s %d", f[1], i))
			}
		case strings.HasPrefix(line, "case "):
			f := strings.Fields(line)
			cas, file = &sum117Case{ID: f[1], Section: f[2], Round2: round2}, nil
			cases = append(cases, cas)
		default:
			key, value, _ := strings.Cut(strings.TrimSpace(line), " ")

			switch key {
			case "title":
				cas.Title = value
			case "pre":
				cas.Pre = append(cas.Pre, value)
			case "step":
				cas.Steps = append(cas.Steps, &sum117Step{Command: value})
			case "data":
				last := cas.Steps[len(cas.Steps)-1]
				last.Data = append(last.Data, dif116Unescape(value))
			case "post":
				cas.Post = append(cas.Post, value)
			default:
				t.Fatalf("cases.txt: %q", line)
			}
		}
	}

	return files, cases
}

// sum117Volume makes the probe's volume as exchange.cmd and exchange2.cmd
// make it, each input file written to the host as gen.go writes in/, and
// mounts it on DUA1 with SUM$TOP, SUM$OUT, and SUM$SCR defined and the
// default directory [SUM117.IN], as SUM117.COM leaves them.
func sum117Volume(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)
	dir := sum117Dir(t)
	files, _ := readSum117Cases(t)

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

	volume := filepath.Join(t.TempDir(), "sum117.dsk")
	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+volume+`" /DEVICE=RD53 SUM117`)

	if err := c.cmd().Mount("DUA5:", volume, true); err != nil {
		t.Fatal(err)
	}

	paths := strings.NewReplacer(`"testdata/sum117/in/`, `"`+in+`/`, `"testdata/sum117/`, `"`+dir+`/`)

	buf.Reset()

	for _, name := range []string{"exchange.cmd", "exchange2.cmd"} {
		script, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

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
	}

	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	shareFindRun(t, d, "DISMOUNT DUA5")
	shareFindRun(t, d, "DISMOUNT DUA1")

	if err := c.cmd().Mount("DUA1:", volume, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "DEFINE SUM$TOP DUA1:")
	shareFindRun(t, d, "DEFINE SUM$OUT SUM$TOP:[SUM117.OUT]")
	shareFindRun(t, d, "DEFINE SUM$SCR SUM$TOP:[SUM117.SCR]")
	shareFindRun(t, d, "SET DEFAULT SUM$TOP:[SUM117.IN]")

	return d, c, buf
}

// sum117Run is one case's outcome: its part of the log, after its "####
// CASE" line, and the files it left in [SUM117.OUT], by type (OUT, LIS, K1,
// ...), each a record per line.
type sum117Run struct {
	Log   []string
	Files map[string][]string
}

// runSum117 runs cases as SUM117.COM runs them, in one procedure with its
// log on the volume, and returns each case's outcome by its ID.
func runSum117(t *testing.T, cases []*sum117Case) map[string]*sum117Run {
	t.Helper()

	d, c, buf := sum117Volume(t)

	var b strings.Builder

	b.WriteString("$ SET NOON\n$ SET VERIFY\n")

	const status = "$ S = $STATUS\n$ WRITE SYS$OUTPUT \"STATUS \", S, \" \", F$MESSAGE(S)\n"

	for _, cas := range cases {
		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"#### CASE %s: %s\"\n", cas.ID, strings.ReplaceAll(cas.Title, `"`, `""`))

		for _, p := range cas.Pre {
			fmt.Fprintf(&b, "$ %s\n", p)
		}

		for _, st := range cas.Steps {
			fmt.Fprintf(&b, "$ %s\n", st.Command)

			for _, line := range st.Data {
				b.WriteString(line + "\n")
			}

			b.WriteString(status)
		}

		for _, p := range cas.Post {
			fmt.Fprintf(&b, "$ %s\n", p)
		}
	}

	b.WriteString("$ SET DEFAULT SUM$TOP:[SUM117.IN]\n$ EXIT\n")

	host := filepath.Join(t.TempDir(), "run.com")
	if err := os.WriteFile(host, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, `COPY "`+host+`"/HOST SUM$TOP:[SUM117]RUN.COM`)
	buf.Reset()
	shareFindRun(t, d, "@SUM$TOP:[SUM117]RUN/OUTPUT=SUM$TOP:[SUM117]RUN.LOG")

	s := c.ContainerSession

	read := func(spec string) []string {
		loc, err := s.Locate(spec, false)
		if err != nil {
			t.Fatal(err)
		}

		records, _, err := s.ReadRecordFile(loc, rms.TextRecords)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}

		lines := make([]string, len(records))
		for i, r := range records {
			lines[i] = string(r)
		}

		return lines
	}

	runs := sum117LogCases(read("SUM$TOP:[SUM117]RUN.LOG"))

	made, _ := s.Search("SUM$TOP:[SUM117.OUT]*.*;*")

	for _, name := range made {
		file, _ := rms.SplitVersion(rms.NameType(name))
		id, typ, _ := strings.Cut(file, ".")

		if r := runs[id]; r != nil {
			r.Files[typ] = read(name)
		}
	}

	for _, cas := range cases {
		if runs[cas.ID] == nil {
			t.Fatalf("case %s isn't in the log", cas.ID)
		}
	}

	return runs
}

// sum117LogCases splits a log by its "#### CASE id:" lines, each case's
// lines with the command lines left out and any CR removed.
func sum117LogCases(lines []string) map[string]*sum117Run {
	runs := map[string]*sum117Run{}

	var cur *sum117Run

	for _, line := range lines {
		line = strings.ReplaceAll(line, "\r", "")

		if rest, ok := strings.CutPrefix(line, "#### CASE "); ok {
			id, _, _ := strings.Cut(rest, ":")
			cur = &sum117Run{Files: map[string][]string{}}
			runs[id] = cur

			continue
		}

		switch {
		case cur == nil || strings.HasPrefix(line, "$ "):
		case strings.HasPrefix(line, "######## "):
			cur = nil
		default:
			cur.Log = append(cur.Log, line)
		}
	}

	return runs
}

// readVMSSum117 reads VMS's outcome of each case: both rounds' logs, and the
// files in vax/out.
func readVMSSum117(t *testing.T) map[string]*sum117Run {
	t.Helper()

	dir := filepath.Join(sum117Dir(t), "vax")
	runs := map[string]*sum117Run{}

	for _, name := range []string{"sum117.log", "sum117b.log"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		maps := sum117LogCases(strings.Split(string(data), "\n"))
		for id, r := range maps {
			runs[id] = r
		}
	}

	entries, err := os.ReadDir(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		id, typ, _ := strings.Cut(e.Name(), ".")

		r := runs[id]
		if r == nil {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, "out", e.Name()))
		if err != nil {
			t.Fatal(err)
		}

		r.Files[typ] = []string{}
		if len(data) > 0 {
			r.Files[typ] = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		}
	}

	return runs
}

// sum117Times are the times a log or a listing shows: DIRECTORY/DATE's, and
// a listing heading's.
var sum117Times = regexp.MustCompile(`\d{1,2}-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d(\.\d\d)?`)

// sum117Mask takes the times out of lines.
func sum117Mask(lines []string) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		out[i] = sum117Times.ReplaceAllString(line, "<time>")
	}

	return out
}

// sum117Later are the cases of a section an oracle test holds whose outputs
// a later subtask makes: their files' contents aren't compared yet, only
// which files there are, and their logs.
var sum117Later = map[string]string{}

// sum117Edited are the COMMAND cases whose update files edit the input: the
// edits are subtask 4's.
var sum117Edited = []string{
	"C01", "C02", "C04", "C06", "C07", "C08", "C09", "C10", "C11", "C12", "C13", "C14",
	"C15", "C16", "C19", "C22", "C24", "C29", "C30", "C31",
}

func init() {
	for _, id := range sum117Edited {
		sum117Later[id] = "the edits are subtask 4's"
	}
}

// sum117NotCompared are the cases' parts an oracle test can't hold to VMS's,
// and why: "log" for the log, "files" for the files.
var sum117NotCompared = map[string]map[string]string{
	"C23": {
		"log":   "EDIT/SLP is govax's own name for EDIT/SUM; on VMS 7.1 it is another utility (A.5.1)",
		"files": "EDIT/SLP is govax's own name for EDIT/SUM",
	},
	"C28": {"files": "a directory file's records are the volume's own: govax's directories differ from the VAX's"},
	"C29": {"log": "VMS's SUMSLP reads an update file twice, which the log shows; govax reads it once (Decision 8)"},
	"C30": {"log": "VMS's SUMSLP reads an update file twice, which the log shows; govax reads it once (Decision 8)"},
}

// sum117Known are log lines govax writes differently for reasons outside
// EDIT/SUM, each replaced by VMS's before the logs are compared.
var sum117Known = map[string][][2]string{
	// DELETE's success status is DCL's on govax, SS$_NORMAL with the
	// message inhibited on VMS: the phase's progress log, for a later fix.
	"C31": {{
		"STATUS %X00030001 %CLI-S-NORMAL, normal successful completion",
		"STATUS %X10000001 %SYSTEM-S-NORMAL, normal successful completion",
	}},
}

// holdSum117 runs the cases of sections and holds each to VMS's: its log
// (times masked), which files it made, and their records (the listing's
// times masked).
func holdSum117(t *testing.T, sections ...string) {
	t.Helper()

	_, all := readSum117Cases(t)

	var cases []*sum117Case

	for _, cas := range all {
		for _, s := range sections {
			if cas.Section == s {
				cases = append(cases, cas)
			}
		}
	}

	got := runSum117(t, cases)
	want := readVMSSum117(t)

	for _, cas := range cases {
		g, w := got[cas.ID], want[cas.ID]
		if w == nil {
			t.Errorf("%s: not in VMS's logs", cas.ID)

			continue
		}

		skip := sum117NotCompared[cas.ID]

		for i, line := range g.Log {
			for _, k := range sum117Known[cas.ID] {
				if line == k[0] {
					g.Log[i] = k[1]
				}
			}
		}

		if _, ok := skip["log"]; !ok {
			if a, b := strings.Join(sum117Mask(g.Log), "\n"), strings.Join(sum117Mask(w.Log), "\n"); a != b {
				t.Errorf("%s (%s): the log differs\ngovax:\n%s\nVMS:\n%s", cas.ID, cas.Title, a, b)
			}
		}

		if _, ok := skip["files"]; ok {
			continue
		}

		if a, b := sortedKeys(g.Files), sortedKeys(w.Files); a != b {
			t.Errorf("%s (%s): govax made %s, VMS %s", cas.ID, cas.Title, a, b)

			continue
		}

		if _, later := sum117Later[cas.ID]; later {
			continue
		}

		for typ, wl := range w.Files {
			if a, b := strings.Join(sum117Mask(g.Files[typ]), "\n"), strings.Join(sum117Mask(wl), "\n"); a != b {
				t.Errorf("%s (%s): %s.%s differs\ngovax:\n%s\nVMS:\n%s", cas.ID, cas.Title, cas.ID, typ, a, b)
			}
		}
	}
}

// sortedKeys is a map's keys, sorted and joined.
func sortedKeys(m map[string][]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return "[" + strings.Join(keys, " ") + "]"
}

// TestSum117CommandOracle holds EDIT/SUM's frame to VMS's (subtask 3): the
// files and their names, the qualifiers and where they may go, the errors,
// the messages, and the statuses (the COMMAND section).
func TestSum117CommandOracle(t *testing.T) {
	t.Parallel()

	holdSum117(t, "COMMAND")
}
