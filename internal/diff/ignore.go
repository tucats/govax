package diff

import (
	"bytes"
	"strings"
)

// What /IGNORE and /COMMENT_DELIMITER do to a file's records before they
// are compared, and what a listing then shows of each, as VMS 7.1 did in
// testdata/dif116's IGNORING section (cases I01 to I42, IC_ and ID_ for each
// file type, IE01 to IE13).
//
// Each record is edited, in the DCL Dictionary's order: comments taken out
// (from a delimiter to the record's end), form feeds taken out, a page's
// header left out, runs of blanks and tabs made one blank, then trailing
// blanks and tabs taken off, and then a record left empty left out
// (BLANK_LINES). Two records match if their edited texts do, compared
// without regard to case under CASE. What a listing shows of a record is its
// edited text (EDITED, the default: I03's "trail " for "trail   "), its
// text as it was (EXACT, I07), or its edited text in the printable form
// (PRETTY, I09, I11), CASE never changing it (I02). A record left out is
// neither compared nor listed, and the records keep their numbers in the
// file (I05's master 5 then 7).

// Shown is what a listing shows of a record (/IGNORE's EDITED, EXACT, and
// PRETTY).
type Shown int

// The forms a record is shown in.
const (
	Edited Shown = iota
	Exact
	PrettyForm
)

// Delimiter is a comment character, and whether it starts a comment only
// in a record's first column (the defaults for .COB and .FOR files: IC_COB,
// IC_FOR).
type Delimiter struct {
	Char        byte
	FirstColumn bool
}

// Ignore is what /IGNORE and /COMMENT_DELIMITER ask to be ignored.
type Ignore struct {
	// Comments are the comment characters; nil ignores no comments.
	Comments []Delimiter

	// FormFeeds, Spacing, TrailingSpaces, BlankLines, and Case are
	// /IGNORE's keywords of those names.
	FormFeeds, Spacing, TrailingSpaces, BlankLines, Case bool

	// Header is HEADER's n, 0 for no HEADER.
	Header int

	// Shown is EDITED, EXACT, or PRETTY.
	Shown Shown
}

// defaultDelimiters are the comment characters of each file type that has
// any, the DCL Dictionary's table as VMS 7.1 applied it (the ID_ cases'
// echoes): "!" for most, "!" and ";" for .CBL and .CMD, "*" and "/" in the
// first column for .COB, "!" and C, D, c, and d in the first column for
// .FOR, ";" for .MAC and .MAR; and govax's own "!" for .F90. Any other type
// has none (ID_TXT, ID_DAT).
var defaultDelimiters = map[string][]Delimiter{
	"B2S": {{Char: '!'}}, "B32": {{Char: '!'}}, "BAS": {{Char: '!'}}, "BLI": {{Char: '!'}},
	"CBL": {{Char: '!'}, {Char: ';'}}, "CMD": {{Char: '!'}, {Char: ';'}},
	"COB": {{Char: '*', FirstColumn: true}, {Char: '/', FirstColumn: true}},
	"COM": {{Char: '!'}}, "COR": {{Char: '!'}},
	"FOR": {
		{Char: '!'}, {Char: 'C', FirstColumn: true}, {Char: 'D', FirstColumn: true},
		{Char: 'c', FirstColumn: true}, {Char: 'd', FirstColumn: true},
	},
	"HLP": {{Char: '!'}}, "MAC": {{Char: ';'}}, "MAR": {{Char: ';'}},

	// .F90 is govax's own: VMS 7.1's table predates Fortran 90. Its free
	// source form has "!" comments anywhere and no comment lines by a
	// letter in column 1, so FOR's C, D, c, and d would cut real
	// statements (docs/DEVIATIONS.md).
	"F90": {{Char: '!'}},
	"R32": {{Char: '!'}}, "REQ": {{Char: '!'}},
}

// DefaultDelimiters are the comment characters of a file of type fileType
// (without its period, any case), none for a type the table doesn't have.
func DefaultDelimiters(fileType string) []Delimiter {
	return defaultDelimiters[strings.ToUpper(fileType)]
}

// Prepare edits in's records by ig: the records to compare, and the input as
// the listing shows it, its records the ones kept, each with its number in
// the file.
func (ig Ignore) Prepare(in Input) (keys [][]byte, shown Input) {
	shown = Input{Name: in.Name, Numbers: []int{}, Records: [][]byte{}}
	headerLeft := 0

	for r, record := range in.Records {
		number := in.Number(r)

		// HEADER is looked for in the record as it was, before FORM_FEEDS
		// takes out its form feed (I25). A record that starts with a form
		// feed starts a page's header: one with nothing else is kept and not
		// counted, one with text is the header's first record (I23, I28).
		// Every page's header is left out, not only the file's first (I22's
		// "Date 2"). A form feed alone inside a header is kept and not
		// counted (the manual; unconfirmed).
		formFeedOnly := len(record) == 1 && record[0] == '\f'

		if ig.Header > 0 && len(record) > 0 && record[0] == '\f' {
			headerLeft = ig.Header
		}

		if headerLeft > 0 && !formFeedOnly {
			headerLeft--

			continue
		}

		edited := ig.edit(record)

		if ig.BlankLines && len(edited) == 0 {
			continue
		}

		key := edited
		if ig.Case {
			key = upperASCII(edited)
		}

		var text []byte

		switch ig.Shown {
		case Exact:
			text = record
		case PrettyForm:
			text = []byte(Pretty(edited))
		default:
			text = edited
		}

		keys = append(keys, key)
		shown.Records = append(shown.Records, text)
		shown.Numbers = append(shown.Numbers, number)
	}

	return keys, shown
}

// edit is a record with the ignored characters taken out: comments, form
// feeds, extra spacing, and trailing blanks, in that order.
func (ig Ignore) edit(record []byte) []byte {
	out := record

	if ig.Comments != nil {
		out = ig.uncomment(out)
	}

	if ig.FormFeeds && bytes.IndexByte(out, '\f') >= 0 {
		out = bytes.ReplaceAll(out, []byte{'\f'}, nil)
	}

	// SPACING makes each run of blanks and tabs one blank, at the start of
	// the record too (I03's "  two  spaces" matched " two spaces", and
	// "tab\there" "tab here").
	if ig.Spacing {
		var b []byte

		for i := 0; i < len(out); i++ {
			if out[i] == ' ' || out[i] == '\t' {
				b = append(b, ' ')

				for i+1 < len(out) && (out[i+1] == ' ' || out[i+1] == '\t') {
					i++
				}

				continue
			}

			b = append(b, out[i])
		}

		out = b
	}

	if ig.TrailingSpaces {
		out = bytes.TrimRight(out, " \t")
	}

	return out
}

// uncomment is a record cut at its first comment character.
func (ig Ignore) uncomment(record []byte) []byte {
	for i, c := range record {
		for _, d := range ig.Comments {
			if c == d.Char && (!d.FirstColumn || i == 0) {
				return record[:i]
			}
		}
	}

	return record
}

// upperASCII is text with its ASCII letters in uppercase, byte for byte.
// (bytes.ToUpper reads text as UTF-8, and would make every byte from %X80 up
// one replacement character, so R15's records 9, which differ, would match.)
// Whether VMS folded DEC's multinational letters too, no case shows: R10's
// and R11's records of them are the same.
func upperASCII(text []byte) []byte {
	out := make([]byte, len(text))

	for i, c := range text {
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}

		out[i] = c
	}

	return out
}
