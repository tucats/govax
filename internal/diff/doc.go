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
// "extra dog".
//
// # The search
//
// That is the DCL Dictionary's description. Which place is "nearest", where
// records repeat, the manual doesn't say; the rule here was fitted to VMS
// 7.1's listings of the probe's 570 generated pairs (testdata/dif116's
// GENERATED section, with /MATCH, /WINDOW, /MERGED, and
// /MAXIMUM_DIFFERENCES), and fits every one, and every merged listing of
// the LAYOUT, MATCHING, and LIMITS sections (TestProbeListings).
//
// From the first records that differ, a[i] and b[j], the search looks at the
// places (p, q) on the edge of a square growing a record at a time: at depth
// d, (i+d, j) and (i, j+d), then (i+d, j+1) and (i+1, j+d), and so on to
// (i+d, j+d). Of each pair it tries the master's first. The first place
// where /MATCH records agree, or both files end, ends the section. So the
// search minimizes the larger of the two sides, not their sum, and between
// sides of equal size prefers to take more of the master. In G017 with
// /MATCH=1, after the master's record 3 and the revision's 4 agree,
//
//	master    revision
//	delta     alpha        <- the section starts here
//	bravo     charlie
//	delta     bravo
//	alpha     delta
//
// VMS ends the section at bravo (depth 2: one master record, two revision
// records), not at alpha (depth 3: three master records, none of the
// revision's), though both leave three records in the section; the first
// rule, the fewest records in the section, chose alpha, and was wrong in four
// of the 40 /MATCH=1 pairs.
//
// /WINDOW=w stops the square at depth w. With no place found, the search
// starts again from (i+1, j+1), at depth 0, and so on until both files end.
// So a changed block longer than the window still ends where it would have
// (M03 at every /WINDOW), but a moved block, whose place in step lies
// outside the window on one side, is swallowed into a larger section (H04 at
// /WINDOW=2), and the section may end later than it would have without the
// window (H08 at /WINDOW=2, which a search moving on by w records, the first
// reading, got wrong).
//
// /MAXIMUM_DIFFERENCES=n stops at the section that brings the count to n or
// more: that section lists only the records the count had left, of each file
// from its start, with no matched records after it.
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
