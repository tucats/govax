// Package sumslp is VMS's SUMSLP, the batch editor behind EDIT/SUM and
// MACRO/UPDATE (docs/planning/PHASE-117 - SUMSLP.md): one input file's
// records updated by one or more update files of SLP edit commands, the
// result a new file's records and a listing of what changed.
//
// It is plain Go, knowing nothing of RMS, DCL, or the machine: the command
// (internal/dclcmd/sumslp.go) reads the files and writes the results. Its
// rules are fitted to VMS 7.1's output in the phase's probe (testdata/sum117,
// the phase doc's A.4 and A.5), never written from VMS's code.
//
// # An update file
//
// An update file is lines of text. A line whose first character is an
// operator does something; any other line is a record to insert:
//
//	-locator1,locator2,/audit trail/;comment
//	               an edit command: with one locator, the records that
//	               follow are inserted after that line of the input; with
//	               two, lines locator1 to locator2 are replaced by them
//	               (deleted, if none follow)
//	\              the audit trail off for the lines after
//	%              the audit trail on again
//	/              the end of the update file
//	<              the next character is an ordinary one ("</" is a
//	               record that starts with a slash)
//
// Every line number is the original input's, never the file's as edited so
// far, so that several update files can be merged into one list and applied
// in one pass.
//
// # What this package does so far
//
// The phase builds it a subtask at a time. Subtask 3 is the command's frame
// and the listing's layout (listing.go): Apply numbers the input's lines and
// copies them, applying no edits yet; subtask 4 reads and applies one update
// file, subtask 5 merges several, subtask 6 adds the audit trail and the
// messages the listing carries.
//
// # The listing
//
// A listing is pages of 60 lines: two heading lines, a blank line, and 57
// lines of the file (Listing). The first heading line is a form feed, the
// run's date and time, "VAX-11 SUMSLP V01.00", and the page number; the
// second the input's creation date and time and its full name. Each line of
// the file is a 16-column field (an audit trail, or "-n" on the first line
// after n deleted ones), the line's number right-aligned in 7 columns (".1",
// ".2", ... for inserted lines), a blank, and the record, the whole cut at
// 255 bytes. A page's heading is written with its first line, so an empty
// input's listing has no records at all (L02).
package sumslp
