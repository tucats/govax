package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// lbr90wPartsStart and lbr90wPartsEnd bound HELP90W.COM's parts C and D,
// LIBRARY's view of the libraries the routines made and what round 1 left of
// LIBRARY/HELP (subtask 11e), in the procedure and in VMS's log.
const (
	lbr90wPartsStart = "$ ! Part C:"
	lbr90wPartsEnd   = "$ ! The libraries and listings"
)

// lbr90wPartFiles are the files parts C and D read, beside what part A and
// part B left on the volume.
var lbr90wPartFiles = []string{
	"one.txt", "two.txt", "h90b.hlp", "digits.hlp", "toolong.hlp", "multi.hlp",
	"notfirst.hlp", "goodbad.hlp",
}

// lbr90wSection returns the lines of text from the line starting start up to
// the line starting end.
func lbr90wSection(t *testing.T, text, start, end string) []string {
	t.Helper()

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	from, to := -1, -1

	for i, line := range lines {
		if from < 0 && strings.HasPrefix(line, start) {
			from = i
		}

		if from >= 0 && strings.HasPrefix(line, end) {
			to = i

			break
		}
	}

	if from < 0 || to < 0 {
		t.Fatalf("no section from %q to %q", start, end)
	}

	return lines[from:to]
}

var (
	// lbr90wDate is a date and time as LIBRARY and DIRECTORY show them.
	lbr90wDate = regexp.MustCompile(`\d{1,2}-[A-Z]{3}-\d{4} \d\d:\d\d(:\d\d(\.\d\d)?)?`)
	// lbr90wCreator is the librarian's name in a listing's heading.
	lbr90wCreator = regexp.MustCompile(`(Creator:  ).*$`)
	// lbr90wAllocated is a file's allocation in DIRECTORY/SIZE=ALL, and
	// lbr90wTotal the total's: VMS's volume had clusters of 3 blocks, and
	// VMS's LIBRARY gave a library it created 100 blocks (/CREATE=BLOCKS),
	// where govax's files take only the blocks they use.
	lbr90wAllocated = regexp.MustCompile(`^(\S+;\d+ +\d+)/\d+$`)
	lbr90wTotal     = regexp.MustCompile(`^(Total of \d+ files?, )\d+/\d+( blocks\.)$`)
	// lbr90wBinary is W13's line in DIRECTORY: VMS 7.1's binary keys
	// crashed LBR$INSERT_KEY and left it empty, where govax's took the
	// manual's keys (subtask 11d). Each file's line is compared, so the
	// total's blocks used are masked as well as its allocation.
	lbr90wBinary = regexp.MustCompile(`^(W13\.ULB;1 +)\d+/`)
)

// lbr90wPartDifferences are the commands of parts C and D whose output
// isn't VMS's, with the reason for each; they aren't compared.
var lbr90wPartDifferences = map[string]string{
	"$ LIBRARY/LIST/FULL W13.ULB": "W13's binary keys: VMS 7.1's LBRSHR crashed on them and left the file empty (subtask 11d)",
}

// lbr90wPartLine masks what may differ from VMS's in a line of parts C and D:
// dates and times, the librarian's name, and the device.
func lbr90wPartLine(line string) string {
	line = strings.TrimRight(line, " ")
	line = lbr90wDate.ReplaceAllString(line, "(date)")
	line = lbr90wCreator.ReplaceAllString(line, "${1}(name)")
	line = lbr90wBinary.ReplaceAllString(line, "${1}(binary keys)/")
	line = lbr90wTotal.ReplaceAllString(line, "${1}(used)/(allocated)${2}")

	return lbr90wDevice.ReplaceAllString(line, "DUAn:")
}

// lbr90wRunParts runs parts C and D on govax, as a procedure of their own
// with VERIFY on, and holds the log to VMS's, command by command.
func lbr90wRunParts(t *testing.T, d *Dispatcher, c *Console, dir string, vms map[string][]byte) {
	t.Helper()

	for _, name := range lbr90wPartFiles {
		src, err := filepath.Abs(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY "`+src+`"/HOST DUA3:[000000]`+strings.ToUpper(name))
	}

	com, err := os.ReadFile(filepath.Join(dir, "help90w.com"))
	if err != nil {
		t.Fatal(err)
	}

	proc := append([]string{"$ SET NOON", "$ SET VERIFY"}, lbr90wSection(t, string(com), lbr90wPartsStart, lbr90wPartsEnd)...)
	host := filepath.Join(t.TempDir(), "parts.com")

	if err := os.WriteFile(host, []byte(strings.Join(proc, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, `COPY "`+host+`"/HOST DUA3:[000000]PARTS.COM`)
	if err := d.Dispatch("@PARTS/OUTPUT=PARTS.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PARTS: %v", err)
	}

	log := filepath.Join(t.TempDir(), "parts.log")
	shareFindRun(t, d, `COPY DUA3:[000000]PARTS.LOG "`+log+`"/HOST/QUIET`)

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	got := lbr90wSection(t, strings.TrimRight(string(data), "\n")+"\n"+lbr90wPartsEnd, lbr90wPartsStart, lbr90wPartsEnd)

	vmsLog, err := os.ReadFile(filepath.Join(dir, "vax", "round3", "help90w.log"))
	if err != nil {
		t.Fatal(err)
	}

	want := lbr90wSection(t, string(vmsLog), lbr90wPartsStart, lbr90wPartsEnd)

	if out := os.Getenv("LBR90W_PARTS"); out != "" {
		_ = os.WriteFile(out, []byte(strings.Join(got, "\n")+"\n"), 0o644)
	}

	lbr90wCompareCommands(t, got, want)
	lbr90wSameLibraries(t, c, lbr90wPartLibraries, vms)
}

// lbr90wPartLibraries are the libraries parts C and D made, held to VMS's
// (h90w.bck) block for block but for times and the librarian's name.
// WD4.HLB, the object library LIBRARY made from a help library, isn't:
// VMS's took the failed module's records off its blocks' counts and set the
// header's next RFA back to the module's start (51,0), which govax doesn't
// (docs/PHASE-90 - VMS help.md, subtask 11e); its size is VMS's.
var lbr90wPartLibraries = []string{
	"W2L.TLB;1", "WD1.HLB;1", "WD2.HLB;1", "WD3.HLB;1", "WD5.HLB;1", "WD6.HLB;1",
	"WD7.HLB;1", "WD8.HLB;1", "WD8.HLB;2", "WD8.HLB;3",
}

// lbr90wSameLibraries holds each of names, on govax's DUA3:[000000], to
// VMS's library of the same name and version.
func lbr90wSameLibraries(t *testing.T, c *Console, names []string, vms map[string][]byte) {
	t.Helper()

	for _, name := range names {
		data, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: "DUA3:[000000]" + name})
		if err != nil {
			t.Errorf("%s: %v", name, err)

			continue
		}

		if dir := os.Getenv("LBR90W_DUMP"); dir != "" {
			_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
		}

		lbr90wSameLibrary(t, name, lbr90wUnwritten(name, data), lbr90wUnwritten(name, vms[name]))
	}
}

// lbr90wCommands splits a verified log into its commands, each the command's
// line and what follows it.
func lbr90wCommands(lines []string) [][]string {
	var out [][]string

	for _, line := range lines {
		if strings.HasPrefix(line, "$ ") || len(out) == 0 {
			out = append(out, nil)
		}

		out[len(out)-1] = append(out[len(out)-1], lbr90wPartLine(line))
	}

	return out
}

// lbr90wCompareCommands holds each command's output in got to want's.
func lbr90wCompareCommands(t *testing.T, got, want []string) {
	t.Helper()

	g, w := lbr90wCommands(got), lbr90wCommands(want)

	if len(g) != len(w) {
		t.Errorf("parts C and D: %d commands, VMS's %d", len(g), len(w))
	}

	for i := 0; i < min(len(g), len(w)); i++ {
		if why, ok := lbr90wPartDifferences[w[i][0]]; ok {
			t.Logf("%s not compared: %s", w[i][0], why)

			continue
		}

		if strings.Join(g[i], "\n") != strings.Join(w[i], "\n") {
			t.Errorf("%s\n got:\n%s\n\nwant:\n%s", w[i][0], strings.Join(g[i], "\n"), strings.Join(w[i], "\n"))
		}
	}
}
