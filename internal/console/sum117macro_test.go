package console

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// sum117MacroLogs are the MACRO cases whose logs aren't compared, and why.
var sum117MacroLogs = map[string]string{
	"M06": "govax's MACRO shows an assembly's errors in its own form, not VMS's listing line and message",
	"M13": "govax's MACRO shows an assembly's errors in its own form, not VMS's listing line and message",
}

// TestSum117MacroOracle holds MACRO/UPDATE to VMS 7.1's (Phase 117, subtask
// 8; the MACRO section): the update files' defaults, merging, a list and a
// concatenation, an update that makes an error, missing update files,
// /NOUPDATE, /UPDATE after the verb, the audit trail in the listing, SUMSLP's
// warnings, and /DEBUG. The logs are compared but for M06's and M13's
// assembly errors (sum117MacroLogs); the listings up to their performance
// indicators, but for their headings, and their closing summaries of
// messages and lines; the objects record for record, but for their header
// records and the source files' dates (mac113Dump).
func TestSum117MacroOracle(t *testing.T) {
	t.Parallel()

	_, all := readSum117Cases(t)

	var cases []*sum117Case

	for _, cas := range all {
		if cas.Section == "MACRO" {
			cases = append(cases, cas)
		}
	}

	got := runSum117(t, cases)
	want := readVMSSum117(t)
	dir := filepath.Join(sum117Dir(t), "vax", "out")

	for _, cas := range cases {
		g, w := got[cas.ID], want[cas.ID]

		if _, skip := sum117MacroLogs[cas.ID]; !skip {
			if a, b := strings.Join(sum117Mask(g.Log), "\n"), strings.Join(sum117Mask(w.Log), "\n"); a != b {
				t.Errorf("%s (%s): the log differs\ngovax:\n%s\nVMS:\n%s", cas.ID, cas.Title, a, b)
			}
		}

		if a, b := sortedKeys(g.Files), sortedKeys(w.Files); a != b {
			t.Errorf("%s (%s): govax made %s, VMS %s", cas.ID, cas.Title, a, b)

			continue
		}

		for typ := range w.Files {
			name := cas.ID + "." + typ

			switch {
			case strings.HasSuffix(typ, "LIS"):
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}

				a := sum117MacroListing(strings.Join(g.Files[typ], "\n"))
				b := sum117MacroListing(string(data))

				if a != b {
					t.Errorf("%s (%s): %s differs\ngovax:\n%s\nVMS:\n%s", cas.ID, cas.Title, name, a, b)
				}
			case strings.HasSuffix(typ, "OBJ"):
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}

				records, err := obj.ReadRecords(bytes.NewReader(data))
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}

				mine := make([][]byte, len(g.Files[typ]))
				for i, r := range g.Files[typ] {
					mine[i] = []byte(r)
				}

				a := sum117SourceDates.ReplaceAllString(strings.Join(mac113Dump(t, mine), "\n"), "$1 (date)")
				b := sum117SourceDates.ReplaceAllString(strings.Join(mac113Dump(t, records), "\n"), "$1 (date)")

				if a != b {
					t.Errorf("%s (%s): %s differs\ngovax:\n%s\nVMS:\n%s", cas.ID, cas.Title, name, a, b)
				}
			}
		}
	}
}

// sum117SourceDates masks the creation date of the source file a /DEBUG
// object's DST$K_SRC_DECLFILE declares (its record a byte shorter than
// mac113Dates's): govax's volume made the file at another time.
var sum117SourceDates = regexp.MustCompile(`(10 01 [0-9a-f]{2} 00 01 00)( [0-9a-f]{2}){8}`)

// sum117MacroListing is what TestSum117MacroOracle compares of a MACRO
// listing: its source pages without their headings (mac113Pages), and of its
// closing pages the source lines read and the summary of messages, with the
// lines they were on.
func sum117MacroListing(text string) string {
	out := mac113Pages(text)

	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")

	for i, line := range lines {
		switch {
		case strings.Contains(line, "source lines were read"):
			read, _, _ := strings.Cut(line, ",")
			out = append(out, read)
		case strings.HasPrefix(line, "There were ") && strings.Contains(line, "messages"):
			out = append(out, line)

			for _, more := range lines[i+1:] {
				if !strings.HasPrefix(more, " ") {
					break
				}

				out = append(out, more)
			}
		}
	}

	return strings.Join(out, "\n")
}
