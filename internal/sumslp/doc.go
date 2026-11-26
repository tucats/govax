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
// # Reading a file (parse.go)
//
// The command's parts are positional and blanks between them are ignored; a
// locator is a number read modulo 65,536 as a signed word (so 99,999 is
// -31,073), or "." for the current line, which an empty locator1 is too. A
// bad command (a sign, a letter, a third comma, an unclosed trail, a
// locator2 below locator1) is SLPSYNERR: it is numbered as an inserted line
// of the edit before it but not written, the message listed in its place,
// and the text after it goes to that edit (an insert before line 1 if there
// was none). An insert with no text is no edit at all: it only moves the
// current line, or gives the trail (U03). A file without its "/" is PRMEOF,
// listed after the last edit's text. EDOUTSEQ marks a command whose locator1
// isn't above the one before it in its file, or a file's first command with
// a negative one.
//
// # The pass (sumslp.go)
//
// Every file's edits are sorted by locator1, the higher precedence first
// among equals (a later file, then a later line of a file), and applied in
// that order as the pass reads the input:
//
//   - An insert after line n writes line n if the pass hasn't, then its text.
//     If the pass is already past n, the text goes at the current place
//     (O05, O08); after a line past the input's end it is lost (Y12, E08).
//     After a negative locator it goes after line 1 (N01, Y05), and a range
//     at 0 or below is an insert before line 1 (Y24, N10).
//   - A replace or delete of m to n deletes those lines at once, and stays
//     open: inserts after the lines it deleted are held, and its text, then
//     theirs, is written when the next edit lies beyond it or the run ends
//     (A.5.2, G05, O07).
//   - A range whose first line the pass has read is dropped (O02, O06, O09),
//     and if it starts inside the open range, every insert that range holds
//     is dropped with it (G14, G14R, the CLASH2 section).
//   - A range the input ends in closes there, its text written, and the input
//     is read again from its start, its lines numbered on, once; a later edit
//     reaches those lines by their new numbers (E01 to E10). Otherwise the
//     input's end ends the run.
//   - Edits that meet (an insert after n meets edits of n; adjacent ranges
//     don't) are EDITSCLSH, one message naming them all in the sorted order,
//     listed before the first line of the first of them. EDOUTSEQ is listed
//     just before its edit is applied.
//   - An input line written after a range that closed normally shows "-n", n
//     the lines deleted since an input line was last written; a range the
//     input ended marks nothing, though its lines count toward the next mark
//     (E10). Inserted lines are numbered ".1", ".2", ... from the last input
//     line written.
//   - An inserted line shows its file's audit trail, the last one a command
//     of the file gave, unless a "\" line turned it off before the line was
//     read (U01 to U10, G12).
//
// Not settled by any case, and govax's choice: "." after a replace is its
// locator2; a range starting two or more lines past the input's end is lost,
// as an insert there is; EDOUTSEQ of a dropped or held edit is listed when
// the pass reaches it; PRMEOF takes no line number; a range open at the
// second end closes there. With several update files a range that runs out
// is the same: the other files' edits reach the second reading by its
// numbers, and a range starting inside the one that ran out is dropped
// (TestMergeChoices pins each).
//
// # Records
//
// Any byte goes through as it is, control and 8-bit bytes, form feeds, tabs,
// trailing blanks, and empty records (F03, F08, F09), whatever the input's
// record format (F01 to F07: the command reads and writes the files). SUMSLP
// reads records of up to 255 bytes (MaxRecord). A longer one in an update
// file is the command's READERR, that file's edits none of them applied
// (B30, W256 to W513); a longer one in the input stops the pass where it
// reaches it: the output and listing hold what came before, an open range's
// text and every later edit unwritten, and Result.Long is its length, which
// the command reports as READERR with RMS's RTB, its status (X00256 to
// X32767). Unconfirmed: a long record inside an open range (govax stops
// there too, the range's text unwritten).
//
// # /HEADER
//
// With /HEADER the output's records are VFC, each with a 12-byte fixed
// control (Result.Headed): a word with the input line's number, 0 for an
// inserted line; a word with an inserted line's number in its run, or for
// an input line the count its "-n" shows, 0 for none; and the audit trail
// the listing shows, cut or zero-filled to 8 bytes. The VAX's own files had
// them (testdata/sum117/vax/headers.txt, U11 to U14).
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
