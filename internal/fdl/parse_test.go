package fdl

import (
	"strings"
	"testing"
)

// messagesOf is text's messages, a line each, as the utilities show them.
func messagesOf(text string) string {
	_, msgs := Parse(text, Options{ParseTime: func(s string) (uint64, bool) { return 1, !strings.Contains(s, "31-FEB") }})

	var lines []string
	for _, m := range msgs {
		lines = append(lines, m.Lines()...)
	}

	return strings.Join(lines, "\n")
}

// TestMessages holds the parser to what VMS 7.1 said of each fault in round
// 3 of the probe (testdata/idx88's IDX88C.COM, sections C2 to C4), by case.
func TestMessages(t *testing.T) {
	for _, c := range []struct{ name, text, want string }{
		{"C2G", "FILE; ORGANIZATION sequential; CONTIGUOUS true\nRECORD; BLOCK_SPAN 1\n",
			"FDL-W-UNQUAKW, unrecognized qualifier keyword in statement 5 \n \\1\\"},
		{"C3D", "FILE; ORGANIZATION indexed\nRECORD; SIZE 80\nKEY 1; SEG0_LENGTH 4; SEG0_POSITION 10; DUPLICATES yes\nKEY 0; SEG0_LENGTH 8\n",
			"FDL-E-OUTORDER, key or area primary defined out of order in statement 5\n" +
				"FDL-E-UNPRIKW, unrecognized primary keyword in statement 6 \n \\SEG0\\\n" +
				"FDL-E-UNPRIKW, unrecognized primary keyword in statement 7 \n \\SEG0\\\n" +
				"FDL-E-UNPRIKW, unrecognized primary keyword in statement 8 \n \\DUPLICATES\\\n" +
				"FDL-W-NULLPRI, null primary block"},
		{"C3E", "FILE; ORGANIZATION indexed\nRECORD; SIZE 80\nKEY 0; SEG0_LENGTH 8\nKEY 2; SEG0_LENGTH 4; SEG0_POSITION 10; DUPLICATES yes\n",
			"FDL-E-OUTORDER, key or area primary defined out of order in statement 7\n" +
				"FDL-E-UNPRIKW, unrecognized primary keyword in statement 8 \n \\SEG0\\\n" +
				"FDL-E-UNPRIKW, unrecognized primary keyword in statement 9 \n \\SEG0\\\n" +
				"FDL-E-UNPRIKW, unrecognized primary keyword in statement 10 \n \\DUPLICATES\\"},
		{"C41", "FIEL\n    ORGANIZATION sequential\n",
			"FDL-E-UNPRIKW, unrecognized primary keyword in statement 1 \n \\FIEL\\\n" +
				"FDL-E-UNPRIKW, unrecognized primary keyword in statement 2 \n \\ORGANIZATION\\"},
		{"C42", "FILE\n    ORGANISATION sequential\n", "FDL-E-UNPRIKW, unrecognized primary keyword in statement 2 \n \\ORGANISATION\\"},
		{"C43", "FILE\n    ORGANIZATION squential\n", "FDL-W-UNQUAKW, unrecognized qualifier keyword in statement 2 \n \\SQUENTIAL\\"},
		{"C45", "    ORGANIZATION sequential\nFILE\n", "FDL-E-UNPRIKW, unrecognized primary keyword in statement 1 \n \\ORGANIZATION\\"},
		{"C46", "FILE\n    ORGANIZATION sequential\n    ALLOCATION 5\n    ALLOCATION 7\n", "FDL-W-MULSEC, multiple secondary definition in statement 4"},
		{"C47", "FILE\n    ORGANIZATION sequential\nFILE\n    ALLOCATION 5\n", "FDL-W-NOQUAL, no qualifier in statement 3"},
		{"C48", "RECORD\n    SIZE 80\nFILE\n    ORGANIZATION sequential\n", ""},
		{"C49", "FILE\n    ORGANIZATION sequential\n    ALLOCATION\n", "FDL-W-NOQUAL, no qualifier in statement 3"},
		{"C4A", "FILE\n    ORGANIZATION sequential\n    CONT yes\n", "FDL-W-ABKW, ambiguous keyword in statement 3 \n \\CONT\\"},
		{"C4B", "FILE\n    NAME \"C4BNAME.DAT\n    ORGANIZATION sequential\n", ""},
		{"C4C", "FILE\n    CONTIGUOUS maybe\n    ORGANIZATION sequential\n", "FDL-W-UNQUAKW, unrecognized qualifier keyword in statement 2 \n \\MAYBE\\"},
		{"C4D", "FILE; ORGANIZATION indexed\nKEY; SEG0_LENGTH 8\n",
			"FDL-W-VALPRI, value required on primary in statement 3\nFDL-E-UNPRIKW, unrecognized primary keyword in statement 4 \n \\SEG0\\"},
		{"C4E", "FILE; ORGANIZATION indexed\nKEY 300; SEG0_LENGTH 8\n",
			"FDL-E-OUTORDER, key or area primary defined out of order in statement 3\nFDL-E-UNPRIKW, unrecognized primary keyword in statement 4 \n \\SEG0\\"},
		{"C4G", "FILE\n    ORGANIZATION sequential\n    PROTECTION (system:RWQ)\n", "FDL-E-SYNTAX, syntax error in statement 3   \n \\ (SYSTEM:RWQ)\\"},
		{"C4H", "FILE\n    ORGANIZATION sequential\nDATE\n    EXPIRATION \"31-FEB-2030\"\n", "FDL-W-INVDATIM, invalid date/time in statement 4"},
	} {
		if got := messagesOf(c.text); got != c.want {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", c.name, got, c.want)
		}
	}
}

// TestValues holds each kind of value, abbreviations and comments (C2F), a
// repeated secondary's last value (C46), and the sections' order.
func TestValues(t *testing.T) {
	def, msgs := Parse(`! a comment line
fi
	org	SEQ	! a comment after a value

  alloc 6 ; exte 3 ! two attributes and a comment
	contig F
  OWNER [200,201]
  PROTECTION (system:RWED, owner:RWE, group:R, world:)
  NAME "A;B!C.DAT"
  ALLOCATION 7
rec;form STREAM;si 12
  bl T
KEY 0; NULL_VALUE 32; TYPE dbin4
KEY 1; NULL_VALUE " "
`, Options{})

	want := "FDL-W-MULSEC, multiple secondary definition in statement 9"
	if len(msgs) != 1 || msgs[0].Lines()[0] != want {
		t.Fatalf("messages %v, want only %q", msgs, want)
	}

	file := def.Section("FILE")
	if file.Keyword("ORGANIZATION", "") != "SEQUENTIAL" || file.Number("ALLOCATION", 0) != 7 || file.Number("EXTENSION", 0) != 3 || file.Switch("CONTIGUOUS", true) {
		t.Errorf("FILE: %+v", file)
	}

	if v, _ := file.Get("PROTECTION"); v.Protection != 0xFE80 {
		t.Errorf("protection %#x", v.Protection)
	}

	if v, _ := file.Get("OWNER"); v.Text != "[200,201]" {
		t.Errorf("owner %q", v.Text)
	}

	if name, _ := file.String("NAME"); name != "A;B!C.DAT" {
		t.Errorf("name %q", name)
	}

	// "STREAM" is a keyword in full, though STREAM_CR and STREAM_LF begin
	// with it.
	record := def.Section("RECORD")
	if record.Keyword("FORMAT", "") != "STREAM" || record.Number("SIZE", 0) != 12 || !record.Switch("BLOCK_SPAN", false) {
		t.Errorf("RECORD: %+v", record)
	}

	keys := def.Numbered("KEY")
	if len(keys) != 2 || keys[1].ID != 1 || keys[0].Keyword("TYPE", "") != "DBIN4" {
		t.Fatalf("keys: %+v", keys)
	}

	for _, k := range keys {
		if v, _ := k.Get("NULL_VALUE"); v.Char != ' ' {
			t.Errorf("key %d's null value %#x", k.ID, v.Char)
		}
	}
}

// TestPrimaryInFull: a primary's name in full isn't an abbreviation of a
// secondary, so KEY after a CONNECT section, whose KEY_ secondaries it begins,
// is the primary.
func TestPrimaryInFull(t *testing.T) {
	def, msgs := Parse("FILE; ORGANIZATION indexed\nCONNECT; KEY_L yes; KEY_OF 1\nKEY 0; SEG0_LENGTH 8\n", Options{})
	if len(msgs) != 0 {
		t.Fatalf("messages: %v", msgs)
	}

	if len(def.Numbered("KEY")) != 1 || !def.Section("CONNECT").Switch("KEY_LIMIT", false) {
		t.Errorf("%+v", def)
	}
}

// TestEveryKeywordResolves: every value and name in the tables can be written
// in full, which no other name's prefix hides.
func TestEveryKeywordResolves(t *testing.T) {
	for _, p := range primaries {
		names := p.secondaryNames()

		for i, n := range names {
			if got, ambiguous := match(n, names); got != i || ambiguous {
				t.Errorf("%s %s resolves to %d", p.name, n, got)
			}
		}

		for _, a := range p.secondaries {
			for i, k := range a.keywords {
				if got, _ := match(k, a.keywords); got != i {
					t.Errorf("%s %s %s resolves to %d", p.name, a.name, k, got)
				}
			}
		}
	}
}
