package sumslp

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// The probe's cases (testdata/sum117/cases.txt) that are one EDIT/SUM of a
// file in [SUM117.IN] with update files there, replayed on Apply and Listing
// and held to VMS's output and listing (vax/out), the listing's times masked.

// probeCase is an EDIT/SUM case: its ID and section, its input, and its
// update files' names.
type probeCase struct {
	ID, Section string
	Input       string
	Updates     []string
}

// editSum reads an EDIT/SUM step of the probe's usual form,
//
//	EDIT/SUM/OUTPUT=SUM$OUT:ID.OUT/LISTING=SUM$OUT:ID.LIS INPUT/UPDATE=(A,B)
//
// and nothing else.
var editSum = regexp.MustCompile(`^EDIT/SUM/OUTPUT=SUM\$OUT:(\w+)\.OUT/LISTING=SUM\$OUT:\w+\.LIS ([\w.]+)/UPDATE=\(?([\w,]+)\)?$`)

// probeCases reads the cases of sections whose one step is editSum's form.
func probeCases(t *testing.T, sections ...string) []probeCase {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(probeDir, "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}

	var (
		cases   []probeCase
		section string
		steps   []string
	)

	flush := func() {
		if len(steps) != 1 || !slices.Contains(sections, section) {
			return
		}

		m := editSum.FindStringSubmatch(steps[0])
		if m == nil {
			return
		}

		cases = append(cases, probeCase{ID: m[1], Section: section, Input: m[2], Updates: strings.Split(m[3], ",")})
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "case "):
			flush()

			section, steps = strings.Fields(line)[2], nil
		case strings.HasPrefix(line, "file "):
			flush()

			section, steps = "", nil
		case strings.HasPrefix(line, "  step "):
			steps = append(steps, strings.TrimPrefix(line, "  step "))
		}
	}

	flush()

	return cases
}

// listingTimes are a listing heading's times.
var listingTimes = regexp.MustCompile(`\d{1,2}-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d`)

// holdProbe runs the cases of sections on Apply and holds each to VMS's,
// but for those later lists, by ID, with the reason they wait.
func holdProbe(t *testing.T, later map[string]string, sections ...string) {
	t.Helper()

	files := probeFiles(t)
	cases := probeCases(t, sections...)

	if len(cases) == 0 {
		t.Fatal("no cases")
	}

	for _, c := range cases {
		if _, ok := later[c.ID]; ok {
			continue
		}

		input, ok := files[c.Input]
		if !ok {
			t.Fatalf("%s: no input %s", c.ID, c.Input)
		}

		var updates []Update

		for _, name := range c.Updates {
			records, ok := files[name+".UPD"]
			if !ok {
				t.Fatalf("%s: no update %s", c.ID, name)
			}

			updates = append(updates, Update{Name: "DUA1:[SUM117.IN]" + name + ".UPD;1", Records: records})
		}

		r := Apply(input, updates)

		if want := vmsListing(t, c.ID+".OUT"); !equalRecords(r.Output, want) {
			t.Errorf("%s (%s): the output differs\ngovax:\n%s\nVMS:\n%s", c.ID, c.Section,
				bytes.Join(r.Output, []byte("\n")), bytes.Join(want, []byte("\n")))

			continue
		}

		when := time.Date(2026, 11, 25, 14, 37, 46, 0, time.UTC)
		got := Listing(r.Lines, Heading{Run: when, Created: when, Input: "DUA1:[SUM117.IN]" + c.Input + ";1"})
		want := vmsListing(t, c.ID+".LIS")

		mask := func(l [][]byte) string {
			return listingTimes.ReplaceAllString(string(bytes.Join(l, []byte("\n"))), "<time>")
		}

		if a, b := mask(got), mask(want); a != b {
			t.Errorf("%s (%s): the listing differs\ngovax:\n%s\nVMS:\n%s", c.ID, c.Section, a, b)
		}
	}
}

// TestProbeOneFile holds one update file's edits to VMS's: every edit
// command and its errors, their order in a file, locators past 32,767, and
// ranges past the input's end (the probe's BASICS, SYNTAX, ORDER, NUMBERS,
// and PASTEND sections).
func TestProbeOneFile(t *testing.T) {
	t.Parallel()

	later := map[string]string{
		"B30": "an update record past 255 bytes is the command's READERR (internal/console's oracle)",
	}

	holdProbe(t, later, "BASICS", "SYNTAX", "ORDER", "NUMBERS", "PASTEND")
}

// TestProbeMerged holds several update files' edits to VMS's: their order,
// their clashes, and a range that discards the inserts it holds (the probe's
// MERGE and CLASH2 sections). The rules are the same pass's as one file's.
func TestProbeMerged(t *testing.T) {
	t.Parallel()

	holdProbe(t, nil, "MERGE", "CLASH2")
}

// TestProbeTrailsAndPages holds the audit trail, the listing's numbers and
// markers, and messages on a page to VMS's (the probe's AUDIT, LISTING, and
// PAGES sections, but /HEADER's cases, which are the command's).
func TestProbeTrailsAndPages(t *testing.T) {
	t.Parallel()

	holdProbe(t, nil, "AUDIT", "LISTING", "PAGES")
}
