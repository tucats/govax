// Package diff is the engine of VMS's DIFFERENCES command
// (docs/planning/PHASE-116 - DIFFERENCES.md): two files' records compared,
// the records that don't match gathered into sections, and the listing
// DIFFERENCES writes of them. It is plain Go, knowing nothing of RMS or DCL:
// records in, lines out. The command, which finds and reads the files and
// writes the listing, is internal/dclcmd/differences.go.
//
// # The comparison
//
// DIFFERENCES walks both files a record at a time. While the records agree,
// they are matched. At the first that don't, a difference section begins,
// and DIFFERENCES looks for the nearest place after it where the files are
// in step again: where /MATCH records (3 unless told otherwise) agree in a
// row, or where both files end. Everything before that place is the
// section, records that happened to agree inside it included. So
//
//	master      revision
//	line 1      line 1
//	line 2      line 2
//	            extra cat
//	            extra dog
//	line 3      line 3
//	line 4      line 4
//
// has one section: no master records, and the revision's "extra cat" and
// "extra dog". "Nearest" is the place that leaves the fewest records in the
// section, both files' together (Compare).
//
// This is the DCL Dictionary's description; VMS 7.1's choices where records
// repeat are what the probe's generated pairs show, and the rule is fitted to
// them in the phase's subtask 4.
//
// # The listing
//
// A section is listed in the merged form by default: a row of 12 asterisks,
// "File" and the master's name, the section's master records, 6 asterisks,
// the revision's, and 12 asterisks; each side followed by /MERGED's matched
// records (1 unless told otherwise). A record's line is its number, right
// aligned in five columns, three blanks, and its text, cut at the listing's
// width. Then a blank line, the counts, a blank line, and the command echoed
// with the qualifiers it used (Echo), broken to the width with "-".
package diff
