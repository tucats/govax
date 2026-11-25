package sumslp

import "strconv"

// Update is one update file: its name, as SUMSLP's messages show it (a
// file's full name, or "SYS$INPUT:.;"), and its records, each a line of the
// file.
type Update struct {
	Name    string
	Records [][]byte
}

// Line is one line of the listing.
type Line struct {
	// Field is the 16 columns before the line's number: the audit trail of
	// an inserted line, or "-n" on the first line written after n deleted
	// ones; "" for none.
	Field string

	// Number is the line's number: the input's, or ".n" for the nth line of
	// a run of inserted ones. "" makes the line a message line, written as
	// Text alone.
	Number string

	// Text is the record (or the message line).
	Text []byte
}

// Result is what an update makes: the output file's records and the
// listing's lines.
type Result struct {
	Output [][]byte
	Lines  []Line
}

// Apply updates the input's records with the update files, the first listed
// lowest in precedence.
//
// So far (subtask 3) it applies no edits: the output is the input, and the
// listing numbers its lines, as SUMSLP's does with no update file (the
// probe's C03). The edits are subtask 4's.
func Apply(input [][]byte, updates []Update) *Result {
	_ = updates

	r := &Result{}

	for i, rec := range input {
		r.Output = append(r.Output, rec)
		r.Lines = append(r.Lines, Line{Number: strconv.Itoa(i + 1), Text: rec})
	}

	return r
}
