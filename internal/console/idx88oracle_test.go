package console_test

import (
	encodinghex "encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/isam"
)

// idx88Differences are the cases of testdata/idx88's run in which govax's output
// isn't VMS's, and why. TestIdx88Oracle checks that each still differs, so the
// list can't outlive its reasons.
var idx88Differences = map[int]string{
	34:  "overlapping primary segments: VMS made Prolog 1, govax has only Prolog 3 (RMS$_PLV)",
	59:  "PROLOG=2: Prolog 3 only (RMS$_PLV)",
	60:  "PROLOG=1: Prolog 3 only (RMS$_PLV)",
	61:  "PROLOG=2: Prolog 3 only (RMS$_PLV)",
	92:  "MRS 32225: VMS made Prolog 1, govax has only Prolog 3 (RMS$_PLV)",
	117: "G117, 500 scattered records compressed: a write more in VBN 4, then other split places (the phase doc's \"What the splits settled\")",
	118: "G118, 500 scattered records: some SIDR buckets split elsewhere, which moves the VBNs after them (the same)",
}

// TestIdx88Oracle holds govax's run of testdata/idx88's probe to VMS 7.1's, every
// case whole: each service's status, the RAB and FAB after it, the records read,
// and the files' blocks. A block dump that isn't VMS's byte for byte must still be
// the same file by isam.Dump, which shows nothing past a bucket's free offset:
// after a split or a deletion VMS leaves stale bytes there that govax's writer
// doesn't reproduce (the phase doc's answer 3, and "What the splits settled").
func TestIdx88Oracle(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "idx88", "vax", "idx88-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	vms := idx88Cases(string(data))
	ours := idx88Cases(runIdx88(t))

	if len(vms) < 150 || len(ours) != len(vms) {
		t.Fatalf("%d cases in VMS's run, %d in govax's", len(vms), len(ours))
	}

	for n := range len(vms) {
		if why, ok := idx88Differences[n]; ok {
			if slices.Equal(vms[n], ours[n]) {
				t.Errorf("#%d is VMS's now, though listed as different: %s", n, why)
			}

			continue
		}

		if msg := sameCase(vms[n], ours[n]); msg != "" {
			t.Errorf("#%d: %s", n, msg)
		}
	}
}

// sameCase compares one case's lines, a block dump at a time where they aren't
// the same bytes, and says where they first differ, or "" if they don't.
func sameCase(want, got []string) string {
	w, wDumps := idx88Sections(want)
	g, gDumps := idx88Sections(got)
	attr := idx88Attributes(want)

	for i := range max(len(w), len(g)) {
		var a, b string
		if i < len(w) {
			a = w[i]
		}

		if i < len(g) {
			b = g[i]
		}

		if a == b {
			continue
		}

		k, ok := dumpIndex(a)
		if j, ok2 := dumpIndex(b); ok && ok2 && j == k && k < len(wDumps) && k < len(gDumps) {
			if msg := sameFile(wDumps[k], gDumps[k], attr); msg != "" {
				return "dump " + strconv.Itoa(k) + ": " + msg
			}

			continue
		}

		return "line " + strconv.Itoa(i) + ":\n vms   " + a + "\n govax " + b
	}

	return ""
}

// dumpLine is one line of a block dump: an offset and 32 bytes in hex.
var dumpLine = regexp.MustCompile(`^   ([0-9A-F]{3}) ([0-9A-F]{64})$`)

// idx88Sections replaces each block dump in a case's lines (from " vbn 1" to
// "=dump $READ") with one line naming it and its bytes' text, and returns the
// dumps' lines apart.
func idx88Sections(lines []string) ([]string, [][]string) {
	var (
		out   []string
		dumps [][]string
	)

	for i := 0; i < len(lines); i++ {
		if lines[i] != " vbn 1" {
			out = append(out, lines[i])

			continue
		}

		var dump []string
		for ; i < len(lines) && !strings.HasPrefix(lines[i], "=dump "); i++ {
			dump = append(dump, lines[i])
		}

		out = append(out, "=blocks "+strconv.Itoa(len(dumps))+" "+hashLines(dump))
		dumps = append(dumps, dump)

		if i < len(lines) {
			out = append(out, lines[i])
		}
	}

	return out, dumps
}

// hashLines is a short stand-in for a dump's lines, equal only for equal lines.
func hashLines(lines []string) string {
	return strconv.Itoa(len(lines)) + ":" + strings.Join(lines, "|")
}

// dumpIndex is which dump an idx88Sections line names.
func dumpIndex(line string) (int, bool) {
	rest, ok := strings.CutPrefix(line, "=blocks ")
	if !ok {
		return 0, false
	}

	n, err := strconv.Atoi(strings.Fields(rest)[0])

	return n, err == nil
}

// fabLine reads a case's FAB report for the record format and size.
var fabLine = regexp.MustCompile(`^ fab org=20 rfm=([0-9A-F]{2}) rat=[0-9A-F]{2} mrs=(\d+) `)

// idx88Attributes is the record format and size of a case's file, from the
// first FAB report of an indexed file in it.
func idx88Attributes(lines []string) isam.Attributes {
	for _, line := range lines {
		if m := fabLine.FindStringSubmatch(line); m != nil {
			size, _ := strconv.Atoi(m[2])
			attr := isam.Attributes{Size: size}

			if m[1] == "01" {
				attr.Format = isam.Fixed
			}

			return attr
		}
	}

	return isam.Attributes{}
}

// dumpBlocks is a file's blocks as a dump shows them: a line for each 32 bytes
// that aren't all zeros.
type dumpBlocks map[uint32][]byte

func (d dumpBlocks) ReadBlock(vbn uint32, block []byte) error {
	b, ok := d[vbn]
	if !ok {
		return errors.New("no such block in the dump")
	}

	copy(block, b)

	return nil
}

// parseDump reads a dump's lines into blocks.
func parseDump(lines []string) (dumpBlocks, error) {
	blocks := dumpBlocks{}

	var cur []byte

	for _, line := range lines {
		if rest, ok := strings.CutPrefix(line, " vbn "); ok {
			vbn, err := strconv.Atoi(rest)
			if err != nil {
				return nil, err
			}

			cur = make([]byte, isam.BlockSize)
			blocks[uint32(vbn)] = cur

			continue
		}

		m := dumpLine.FindStringSubmatch(line)
		if m == nil || cur == nil {
			return nil, errors.New("not a dump line: " + line)
		}

		off, _ := strconv.ParseUint(m[1], 16, 16)

		b, err := encodinghex.DecodeString(m[2])
		if err != nil {
			return nil, err
		}

		copy(cur[off:], b)
	}

	return blocks, nil
}

// sameFile compares two dumps of a file by isam.Dump, saying how they differ.
func sameFile(want, got []string, attr isam.Attributes) string {
	text := func(lines []string) (string, error) {
		blocks, err := parseDump(lines)
		if err != nil {
			return "", err
		}

		f, err := isam.Open(blocks, attr)
		if err != nil {
			return "", err
		}

		var sb strings.Builder
		err = f.Dump(&sb)

		return sb.String(), err
	}

	a, err := text(want)
	if err != nil {
		return "VMS's file: " + err.Error()
	}

	b, err := text(got)
	if err != nil {
		return "govax's file: " + err.Error()
	}

	if a == b {
		return ""
	}

	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range min(len(al), len(bl)) {
		if al[i] != bl[i] {
			return "isam.Dump line " + strconv.Itoa(i) + ":\n vms   " + al[i] + "\n govax " + bl[i]
		}
	}

	return "isam.Dump lengths differ"
}
