package diff

import "fmt"

// SLP writes /SLP's script: the edits SLP, VMS's batch editor, would make to
// turn the master into the revision (testdata/dif116's F35, L03, SAnn,
// SGnnn, R14). For each section a command line, then the revision's records
// that take the place of the master's. F35's script is
//
//	|-    5,    5
//	|data line 5, revised
//	|-   15
//	|inserted one
//	|inserted two
//	|-   26,   27
//	|/
//
// (each line shown after a "|"): the master's records 5 to 5 replaced by the
// revision's, two records inserted after the master's record 15, records 26
// to 27 deleted (no records follow), and "/", the script's end.
//
// The numbers are five columns wide, more if they need them; the records are
// their text, never cut; an insertion before the master's first record is a
// bare "-" (SA06). Files that don't differ get only the "/" (SG137).
// A comparison /MAXIMUM_DIFFERENCES stopped ends with its cut last section
// (L03).
func SLP(revision Input, res Result) []string {
	var lines []string

	for _, s := range res.Sections {
		switch {
		case s.Master.Len() == 0 && s.Master.Start == 0:
			lines = append(lines, "-")
		case s.Master.Len() == 0:
			lines = append(lines, fmt.Sprintf("-%5d", s.Master.Start))
		default:
			lines = append(lines, fmt.Sprintf("-%5d,%5d", s.Master.Start+1, s.Master.End))
		}

		for r := s.Revision.Start; r < min(s.Revision.End, len(revision.Records)); r++ {
			lines = append(lines, string(revision.Records[r]))
		}
	}

	return append(lines, "/")
}
