package anl

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/obj"
)

// fixtureDirs hold real VMS objects with VMS 7.3's ANALYZE/OBJECT output
// for each beside them (NAME.OBJ and NAME.ANL).
var fixtureDirs = []string{
	"../../testdata/mar/vax",
	"../../testdata/mar/macros/vax",
	"../../testdata/mar/list/vax",
	"../../testdata/mar/round/vax",
	"../../testdata/mar/dst/vax",
}

// objectFixture is one object and its real analysis.
type objectFixture struct {
	name     string
	records  [][]byte
	analysis []byte
}

// objectFixtures returns every object that has an analysis beside it.
func objectFixtures(t *testing.T) []objectFixture {
	t.Helper()

	var out []objectFixture

	for _, dir := range fixtureDirs {
		anls, err := filepath.Glob(filepath.Join(dir, "*.anl"))
		if err != nil {
			t.Fatal(err)
		}

		for _, anl := range anls {
			objName := strings.TrimSuffix(anl, ".anl") + ".obj"

			f, err := os.Open(objName)
			if err != nil {
				continue
			}

			records, err := obj.ReadRecords(f)
			f.Close()

			if err != nil {
				t.Fatalf("%s: %v", objName, err)
			}

			analysis, err := os.ReadFile(anl)
			if err != nil {
				t.Fatal(err)
			}

			out = append(out, objectFixture{name: anl, records: records, analysis: analysis})
		}
	}

	if len(out) < 65 {
		t.Fatalf("found %d object fixtures, want at least 65", len(out))
	}

	return out
}

// pageHeaderRE matches a page header of real ANALYZE's output: the form
// feed, title, file, version, and blank line.
var pageHeaderRE = regexp.MustCompile("\f\nAnalyze Object File[^\n]*\n[^\n]*\nANALYZ V07-04\n\n")

// pageContent is an analysis's text without its page layout: the page
// headers and the closing command line (ANALYZE/OBJECT, or an
// abbreviation such as ANAL/OBJ) removed.
func pageContent(analysis []byte) string {
	text := pageHeaderRE.ReplaceAllString(string(analysis), "")

	if i := strings.LastIndex(text, "\nANAL"); i >= 0 {
		text = text[:i+1]
	}

	return text
}

// TestObjectContent checks every line ANALYZE/OBJECT shows for each
// fixture, ignoring how the lines are laid out on pages.
func TestObjectContent(t *testing.T) {
	for _, f := range objectFixtures(t) {
		rep := AnalyzeObject(f.records, ObjectOptions{})

		var b bytes.Buffer
		if err := WriteText(&b, rep.Lines); err != nil {
			t.Fatal(err)
		}

		if diff := firstDiff(pageContent(f.analysis), b.String()); diff != "" {
			t.Errorf("%s: %s", f.name, diff)
		}
	}
}

// firstDiff describes the first line where got differs from want, or
// returns "" when they're the same.
func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")

	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string

		if i < len(w) {
			wl = w[i]
		}

		if i < len(g) {
			gl = g[i]
		}

		if wl != gl {
			return "line " + strconv.Itoa(i+1) + ":\n  want " + quoteLine(wl) + "\n  got  " + quoteLine(gl)
		}
	}

	return ""
}

func quoteLine(s string) string {
	return strings.ReplaceAll(`"`+s+`"`, "\t", `\t`)
}

// TestHexDump checks a dump's partial last row and its character rule.
func TestHexDump(t *testing.T) {
	var r report

	r.hexDump("", []byte{'H', 'i', 0x00, 0x7F, 0xA0, 0xA1, 0xFE, 0xFF, 'x'})

	want := []string{
		dumpHeading,
		dumpUnderline,
		" FF FE A1 A0 7F 00 69 48|  0000  |Hi...\xA1\xFE.|",
		"                      78|  0008  |x       |",
	}

	if len(r.lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(r.lines), len(want))
	}

	for i, w := range want {
		if r.lines[i].Text != w {
			t.Errorf("line %d: got %q, want %q", i+1, r.lines[i].Text, w)
		}
	}
}

// headerTimeRE matches the date and time in a page header.
var headerTimeRE = regexp.MustCompile(`(?m)^(Analyze Object File {1,30})[ 0-9]{2}-[A-Z]{3}-[0-9]{4} [0-9:.]{11}`)

// TestObjectPages checks each fixture's whole analysis, page layout and
// all, with only the page headers' times masked.
func TestObjectPages(t *testing.T) {
	when := time.Date(2026, time.October, 4, 9, 8, 7, 650_000_000, time.UTC)

	for _, f := range objectFixtures(t) {
		text := string(f.analysis)

		// The file analyzed and the command, as the fixture shows them.
		lines := strings.Split(text, "\n")
		file := lines[2]
		command := strings.TrimRight(lines[len(lines)-2], " ")

		rep := AnalyzeObject(f.records, ObjectOptions{})

		var b bytes.Buffer

		p := NewPager(&b, TitleObject, file, command)
		p.Now = func() time.Time { return when }

		if err := p.Write(rep.Lines); err != nil {
			t.Fatal(err)
		}

		if err := p.Close(); err != nil {
			t.Fatal(err)
		}

		want := headerTimeRE.ReplaceAllString(text, "${1}"+vmsTime(when))
		if diff := firstDiff(want, b.String()); diff != "" {
			t.Errorf("%s: %s", f.name, diff)
		}
	}
}
