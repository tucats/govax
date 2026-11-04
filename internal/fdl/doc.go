// Package fdl reads the File Definition Language: the text a VMS FDL file
// holds, which describes a file's organization, records, areas, and keys
// (the Record Management Utilities Reference Manual, chapter 4). CREATE/FDL
// makes an empty file from one, CONVERT/FDL the file it loads, and
// ANALYZE/RMS_FILE/FDL writes one (docs/PHASE-88 - keyed index files.md,
// subtask 10).
//
// The package is plain Go: Parse turns the text into a Definition of
// Sections (each primary attribute, FILE, RECORD, KEY 0, ..., and its
// secondary attributes' values) and the messages VMS's FDL parser would
// give, and knows nothing of RMS, which internal/rms fills its FAB and XABs
// from.
//
// # The language
//
// An FDL file is a list of statements: each line is one, or several when
// semicolons separate them, and an exclamation point starts a comment that
// runs to the line's end (neither counts inside a quoted string). Blank
// statements aren't counted. A statement is a keyword and a value, the
// keyword a primary attribute (FILE, KEY 0) or a secondary one of the
// current primary (ALLOCATION 6). Keywords and keyword values may be
// abbreviated to any unique leading part, in any case.
//
// What VMS 7.1's parser did with each kind of fault (round 3 of the probe,
// testdata/idx88's IDX88C.COM, sections C2 to C4; "What round 3 settled"):
//
//   - A statement that isn't a secondary of the current primary is read as
//     a primary, so an unknown secondary, a secondary before any primary,
//     and each secondary after a primary that was refused are all
//     UNPRIKW, an error, the keyword the message's segment (cut after its
//     first digits: SEG0_LENGTH shows as SEG0).
//   - A numbered primary must come in order from 0 (KEY 1 first, or KEY 0
//     then KEY 2, is OUTORDER, an error); one with no number is VALPRI, a
//     warning, and is refused all the same. The next primary after a
//     refused one adds NULLPRI.
//   - A primary given twice is NOQUAL, a warning, and its secondaries go on
//     into the first; a secondary given twice is MULSEC, and the last value
//     is kept (C46's file has the second ALLOCATION's).
//   - A value that isn't one of a keyword's or a switch's is UNQUAKW (the
//     value its segment), an ambiguous abbreviation of a value or a
//     secondary is ABKW, a missing value NOQUAL, a bad date INVDATIM: all
//     warnings, the attribute left at its default. A bad protection is
//     SYNTAX, an error, its segment the text after the keyword.
//   - A string with no closing quotation mark runs to the line's end, and
//     primaries may come in any order (RECORD before FILE).
//
// Any error means no file; warnings alone still make one. The command
// that called Parse shows the messages (internal/dclcmd/createfdl.go).
package fdl
