package sumslp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// probeDir is testdata/sum117, the phase's probe.
const probeDir = "../../testdata/sum117"

// probeFiles reads cases.txt's input files, by name, each record unescaped
// (\\, \t, \xNN) or made from a "=lines PREFIX N" rule.
func probeFiles(t *testing.T) map[string][][]byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(probeDir, "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][][]byte{}

	var name string

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "file "):
			name, _, _ = strings.Cut(strings.TrimPrefix(line, "file "), " ")
			files[name] = [][]byte{}
		case strings.HasPrefix(line, "case "):
			name = ""
		case name != "" && strings.HasPrefix(line, "|"):
			files[name] = append(files[name], unescape(line[1:]))
		case name != "" && strings.HasPrefix(line, "=lines "):
			f := strings.Fields(line)
			n, _ := strconv.Atoi(f[2])

			for i := 1; i <= n; i++ {
				files[name] = append(files[name], fmt.Appendf(nil, "%s %d", f[1], i))
			}
		}
	}

	return files
}

// unescape undoes cases.txt's escapes.
func unescape(s string) []byte {
	var b []byte

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b = append(b, s[i])

			continue
		}

		switch s[i+1] {
		case 't':
			b = append(b, '\t')
			i++
		case 'x':
			v, _ := strconv.ParseUint(s[i+2:i+4], 16, 8)
			b = append(b, byte(v))
			i += 3
		default:
			b = append(b, s[i+1])
			i++
		}
	}

	return b
}

// vmsListing reads a listing VMS wrote, a record a line.
func vmsListing(t *testing.T, name string) [][]byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(probeDir, "vax", "out", name))
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		return nil
	}

	return bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
}

// TestListingWithNoUpdate holds the listing of a file with no update file to
// VMS's, byte for byte (the probe's C03, L01, L02; the run's and the input's
// times are the probe's): the headings, the numbers, records with tabs, and
// an empty file's listing, which has no records.
func TestListingWithNoUpdate(t *testing.T) {
	t.Parallel()

	files := probeFiles(t)

	at := func(clock string) time.Time {
		v, err := time.Parse("2006-01-02 15:04:05", "2026-11-25 "+clock)
		if err != nil {
			t.Fatal(err)
		}

		return v
	}

	for _, c := range []struct{ id, input, run string }{
		{"C03", "S10.SRC", "14:37:46"},
		{"L01", "W10.SRC", "14:37:49"},
		{"L02", "EMPTY.SRC", "14:37:49"},
	} {
		in, ok := files[c.input]
		if !ok {
			t.Fatalf("%s: no file %s in cases.txt", c.id, c.input)
		}

		r := Apply(in, nil)

		if !equalRecords(r.Output, in) {
			t.Errorf("%s: the output isn't the input", c.id)
		}

		got := Listing(r.Lines, Heading{Run: at(c.run), Created: at("17:12:43"), Input: "DUA1:[SUM117.IN]" + c.input + ";1"})
		want := vmsListing(t, c.id+".LIS")

		if !equalRecords(got, want) {
			t.Errorf("%s: the listing differs\ngovax:\n%s\nVMS:\n%s", c.id, bytes.Join(got, []byte("\n")), bytes.Join(want, []byte("\n")))
		}
	}
}

// TestListingPages lays out more than a page: each page's heading with its
// number before its first line, 57 lines a page, and a long line cut at 255.
func TestListingPages(t *testing.T) {
	t.Parallel()

	var lines []Line

	for i := 1; i <= 120; i++ {
		lines = append(lines, Line{Number: strconv.Itoa(i), Text: []byte("x")})
	}

	lines = append(lines, Line{Field: "a trail longer than sixteen", Number: ".1", Text: bytes.Repeat([]byte("y"), 300)})

	out := Listing(lines, Heading{Input: "DUA1:[A_DIRECTORY_NAME_LONGER_THAN_THE_ROOM]FILE.TXT;1"})

	if len(out) != 121+3*3 {
		t.Fatalf("%d records", len(out))
	}

	for i, page := range []int{0, 60, 120} {
		if want := fmt.Sprintf("Page%5d", i+1); !bytes.HasSuffix(out[page], []byte(want)) || len(out[page]) != 130 {
			t.Errorf("record %d: %q", page, out[page])
		}

		if len(out[page+1]) != 129 {
			t.Errorf("record %d is %d bytes", page+1, len(out[page+1]))
		}
	}

	last := out[len(out)-1]
	if len(last) != MaxLine || !bytes.HasPrefix(last, []byte("a trail longer t     .1 yyy")) {
		t.Errorf("the last line: %q", last)
	}
}

// equalRecords says two lists of records are the same.
func equalRecords(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}

	return true
}
