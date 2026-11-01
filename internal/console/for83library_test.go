package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// for83Dir is the Phase 83 probe, whose part D is LIBRARY/TEXT.
const for83Dir = "../../testdata/for83/"

var (
	for83Times   = regexp.MustCompile(`[ 0-9]\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d(\.\d\d)?`)
	for83FileIDs = regexp.MustCompile(`File ID \(\d+,\d+,\d+\)`)
	for83Tails   = regexp.MustCompile(`(Creator: +|Library history records: +).*$`)
)

// for83PartD is part D of FOR83.COM, without the commands govax doesn't have:
// /COMPRESS, and what it does with the libraries that makes.
func for83PartD(t *testing.T) string {
	t.Helper()

	text, err := os.ReadFile(for83Dir + "for83.com")
	if err != nil {
		t.Fatal(err)
	}

	_, part, ok := strings.Cut(string(text), "$ ! Part D")
	if !ok {
		t.Fatal("no part D")
	}

	part, _, ok = strings.Cut(part, "$ DIRECTORY/SIZE=ALL/DATE *.TLB")
	if !ok {
		t.Fatal("part D doesn't end as it did")
	}

	var lines []string

	cut := false // the line was a /COMPRESS, so its SHOW SYMBOL goes too

	for line := range strings.Lines(part) {
		switch {
		case strings.Contains(line, "/COMPRESS"):
			cut = true
		case cut, strings.Contains(line, " S9"), strings.Contains(line, "E6.TXT"):
			cut = false
		default:
			lines = append(lines, strings.TrimRight(line, "\n"))
		}
	}

	return "$ SET NOON\n$ !" + strings.Join(lines, "\n") + "\n"
}

// for83PartDLog is VMS's log of part D, without the same commands' output.
func for83PartDLog(t *testing.T) []string {
	t.Helper()

	text, err := os.ReadFile(for83Dir + "vax/for83.log")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.ReplaceAll(string(text), "\r", ""), "\n")

	first, end := -1, -1

	var kept []string

	for i, line := range lines {
		// The line before the first listing is /CREATE's status.
		if first < 0 && strings.HasPrefix(line, "Directory of TEXT library") {
			first = i - 1
		}

		if first >= 0 && strings.HasPrefix(line, "S1.TLB;1") {
			end = i - 3 // its heading, and the blank lines around that
			break
		}
	}

	if first < 0 || end < 0 {
		t.Fatal("the log's part D isn't there")
	}

	inS9 := false

	for _, line := range lines[first:end] {
		// The two /COMPRESS commands' statuses come just before S9's listing,
		// and that and E6's dump run together, up to T2's status.
		if strings.Contains(line, "S9.TLB") {
			inS9 = true
			kept = kept[:len(kept)-2]
		}

		if inS9 && line != `  $STATUS == "%X10000001"` {
			continue
		}

		inS9 = false

		kept = append(kept, line)
	}

	return kept
}

// for83Mask takes out of a line what the run decides: times, a file's ID, and
// who made the library. The count of history records goes too: LIBRARY writes
// none (internal/dclcmd/library.go).
func for83Mask(lines []string) []string {
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		line = for83Times.ReplaceAllString(line, "<time>")
		line = for83FileIDs.ReplaceAllString(line, "File ID")
		line = strings.TrimRight(line, " ")
		out = append(out, for83Tails.ReplaceAllString(line, "$1<masked>"))
	}

	return out
}

// TestFor83LibraryOracle runs FOR83.COM's part D on a volume like the VAX's and
// holds what it writes to VMS's log: the listings, messages, statuses, and what
// /EXTRACT wrote.
func TestFor83LibraryOracle(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountClusteredContainer(t, c, "DUA2", 3)

	host := filepath.Join(t.TempDir(), "partd.com")
	writeFile(t, host, for83PartD(t))

	for _, line := range []string{"SET DEFAULT DUA2:[000000]", `COPY "` + host + `"/HOST PARTD.COM`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	buf.Reset()

	if err := d.Dispatch("@PARTD"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@PARTD: %v\n%s", err, buf)
	}

	// VMS's COPY says nothing without /LOG, and govax's always reports.
	var run []string

	for line := range strings.SplitSeq(strings.TrimRight(strings.ReplaceAll(buf.String(), "\r", ""), "\n"), "\n") {
		if !strings.HasPrefix(line, "%COPY-S-COPIED") {
			run = append(run, line)
		}
	}

	got := for83Mask(run)
	want := for83Mask(for83PartDLog(t))

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
