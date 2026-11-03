package console

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// TestCopyRecordFormats makes each kind of file from a host file (rms/copyformat.go),
// and has DIRECTORY/FULL say what it is and RMS's reader give its records back.
func TestCopyRecordFormats(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)
	dir := t.TempDir()

	lines := filepath.Join(dir, "lines.txt")
	writeHostFile(t, lines, "ONE\nTWO TWO\r\nTHREE\n")

	bytesFile := filepath.Join(dir, "bytes.bin")
	writeHostFile(t, bytesFile, "AAAABBBBCCCC")

	cases := []struct {
		name, host, qualifiers string
		format, attributes     string
		records                []string
	}{
		{"fixed lines", lines, "/RECORD_FORMAT=FIXED/RECORD_SIZE=9", "Fixed length 9 byte records", "Carriage return", []string{"ONE      ", "TWO TWO  ", "THREE    "}},
		{"fixed bytes", bytesFile, "/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=4", "Fixed length 4 byte records", "None", []string{"AAAA", "BBBB", "CCCC"}},
		{"odd fixed bytes", bytesFile, "/BINARY/RECORD_FORMAT=FIX/RECORD_SIZE=3", "Fixed length 3 byte records", "None", []string{"AAA", "ABB", "BBC", "CCC"}},
		{"variable lines", lines, "/RECORD_FORMAT=VARIABLE", "Variable length", "Carriage return", []string{"ONE", "TWO TWO", "THREE"}},
		{"variable bytes", bytesFile, "/BINARY/RECORD_FORMAT=VAR/RECORD_SIZE=5", "Variable length", "None", []string{"AAAAB", "BBBCC", "CC"}},
		{"VFC", lines, "/RECORD_FORMAT=VFC", "VFC", "Print", []string{"ONE", "TWO TWO", "THREE"}},
		{"stream", lines, "/RECORD_FORMAT=STREAM", "Stream", "Carriage return", []string{"ONE", "TWO TWO", "THREE"}},
		{"stream LF", lines, "/RECORD_FORMAT=STREAM_LF/CARRIAGE_CONTROL=NONE", "Stream_LF", "None", []string{"ONE", "TWO TWO", "THREE"}},
		{"stream CR", lines, "/RECORD_FORMAT=STREAM_CR/CARRIAGE_CONTROL=FORTRAN", "Stream_CR", "Fortran", []string{"ONE", "TWO TWO", "THREE"}},
		{"undefined", bytesFile, "/BINARY/RECORD_FORMAT=UNDEFINED", "Undefined", "None", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			shareFindRun(t, d, `COPY/QUIET "`+tc.host+`"/HOST OUT.DAT`+tc.qualifiers)

			if out := buf.String(); out != "" {
				t.Fatalf("COPY said %s", out)
			}

			shareFindRun(t, d, "DIRECTORY/FULL OUT.DAT")

			out := buf.String()
			if !strings.Contains(out, "Record format:      "+tc.format) {
				t.Errorf("not %q:\n%s", tc.format, out)
			}

			if !strings.Contains(out, "Record attributes:  "+tc.attributes) {
				t.Errorf("not %q:\n%s", tc.attributes, out)
			}

			if tc.records != nil {
				records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "OUT.DAT"}, rms.TextRecords)
				if err != nil {
					t.Fatal(err)
				}

				var got []string
				for _, r := range records {
					got = append(got, string(r))
				}

				if strings.Join(got, "|") != strings.Join(tc.records, "|") {
					t.Errorf("records %q, want %q", got, tc.records)
				}
			}

			shareFindRun(t, d, "DELETE OUT.DAT;*")
		})
	}

	refused := []struct{ name, source, qualifiers, says string }{
		{"no format", `"` + lines + `"/HOST`, "/RECORD_SIZE=9", "need /RECORD_FORMAT"},
		{"an unknown format", `"` + lines + `"/HOST`, "/RECORD_FORMAT=NOPE", "one of"},
		{"an ambiguous format", `"` + lines + `"/HOST`, "/RECORD_FORMAT=STREAM_", "one of"},
		{"a fixed record's size", `"` + lines + `"/HOST`, "/RECORD_FORMAT=FIXED", "needs /RECORD_SIZE"},
		{"a line too long", `"` + lines + `"/HOST`, "/RECORD_FORMAT=FIXED/RECORD_SIZE=3", "line 2"},
		{"a record too long", `"` + lines + `"/HOST`, "/RECORD_FORMAT=VAR/RECORD_SIZE=3", "record 2"},
		{"part of a record", `"` + bytesFile + `"/HOST`, "/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=5", "whole number"},
		{"bytes to variable, no size", `"` + bytesFile + `"/HOST`, "/BINARY/RECORD_FORMAT=VAR", "needs /RECORD_SIZE"},
		{"bytes to VFC", `"` + bytesFile + `"/HOST`, "/BINARY/RECORD_FORMAT=VFC", "lines of text"},
		{"lines to undefined", `"` + lines + `"/HOST`, "/RECORD_FORMAT=UNDEFINED", "needs /BINARY"},
		{"a size too large", `"` + lines + `"/HOST`, "/RECORD_FORMAT=FIXED/RECORD_SIZE=40000", "at most"},
		{"print on a stream", `"` + lines + `"/HOST`, "/RECORD_FORMAT=STREAM/CARRIAGE_CONTROL=PRINT", "VFC"},
		{"control bytes elsewhere", `"` + lines + `"/HOST`, "/RECORD_FORMAT=VAR/CONTROL_FIELD_SIZE=2", "VFC"},
		{"from a volume", "DUA1:[000000]MAIN1.EXE", "/RECORD_FORMAT=VAR", "from a host file"},
	}

	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()

			err := d.Dispatch("COPY " + tc.source + " NOPE.DAT" + tc.qualifiers)
			if err == nil || !strings.Contains(err.Error(), tc.says) || !strings.Contains(err.Error(), "BADQUALIFIER") {
				t.Errorf("COPY%s: %v, want a bad qualifier saying %q", tc.qualifiers, err, tc.says)
			}

			if _, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "NOPE.DAT"}, rms.VariableRecords); err == nil {
				t.Error("it made the file")
			}
		})
	}
}

// TestDCLContinuation: a command typed on several lines, as in a procedure.
func TestDCLContinuation(t *testing.T) {
	d, _, buf := shareFindConsole(t, false)

	shareFindRun(t, d, "A = 1 + -")

	if !d.Continuing() || buf.String() != "" {
		t.Fatalf("not held: %v, said %s", d.Continuing(), buf.String())
	}

	shareFindRun(t, d, "2 + - ! with a comment")
	shareFindRun(t, d, "3")

	if d.Continuing() {
		t.Fatal("still held")
	}

	symbol := func(name string) string {
		t.Helper()
		buf.Reset()
		shareFindRun(t, d, "SHOW SYMBOL "+name)

		return buf.String()
	}

	if got := symbol("A"); !strings.Contains(got, "A = 6") {
		t.Errorf("A is %s", got)
	}

	// A hyphen in quotes, or after a comment's start, isn't one.
	shareFindRun(t, d, `B = "X-"`)
	shareFindRun(t, d, "C = 7 ! not continued -")

	if d.Continuing() {
		t.Fatal("a hyphen in quotes or a comment held the line")
	}

	if got := symbol("B"); !strings.Contains(got, `"X-"`) {
		t.Errorf("B is %s", got)
	}

	if got := symbol("C"); !strings.Contains(got, "C = 7") {
		t.Errorf("C is %s", got)
	}

	// CTRL/C at the next line's prompt drops what was held.
	shareFindRun(t, d, "D = 8 + -")
	d.CancelContinuation()
	shareFindRun(t, d, "E = 9")

	if got := symbol("E"); !strings.Contains(got, "E = 9") {
		t.Errorf("E is %s", got)
	}

	if err := d.Dispatch("SHOW SYMBOL D"); err == nil && strings.Contains(buf.String(), "D = 8") {
		t.Error("the cancelled line ran")
	}

	// A procedure's lines are joined by its own reader, and a command level above 0
	// doesn't hold them. The debugger's and the assembler's lines aren't DCL's.
	path := filepath.Join(t.TempDir(), "cont.com")
	writeHostFile(t, path, "$ F = 1 + -\n2\n$ G == F + 1\n")
	shareFindRun(t, d, `@"`+path+`"`)

	if d.Continuing() {
		t.Fatal("a procedure's line held")
	}

	if got := symbol("G"); !strings.Contains(got, "G == 4") {
		t.Errorf("G is %s", got)
	}
}
