package asm

import (
	"fmt"
	"strconv"
	"strings"
)

// MACRO/UPDATE (docs/planning/PHASE-117 - SUMSLP.md, subtask 8): VMS's MACRO
// runs SUMSLP on a source with /UPDATE before assembling it, and the listing
// shows what SUMSLP did. The command (internal/dclcmd/macro.go) runs
// internal/sumslp and hands the assembler the updated text and, through
// SetUpdate, how SUMSLP numbered and marked each line. The assembler never
// sees the update files.
//
// What the listing shows, as VMS 7.1's did in testdata/sum117's MACRO
// section (the phase doc's A.4.10):
//
//   - Each line of an updated file has SUMSLP's number in the line number's
//     columns: the input's, or ".n" for the nth line of a run of inserted
//     ones (M01, M02).
//   - SUMSLP's 16-column field (an audit trail, or "-n" on the first line
//     after n deleted ones) takes the binary field's first 16 columns on the
//     line's first listing line (M01, M10), so the binary field has 20
//     columns left on all of the line's listing lines, and an instruction
//     that fitted on one line takes two (M02's MOVL VALUE,R0). A file of the
//     source without /UPDATE keeps 36 (M05's SUMA after an updated SUMP).
//   - A SUMSLP warning is listed before the line it came before in SUMSLP's
//     listing, as an empty line, then an empty line followed by the
//     warning's lines, then the location line MACRO lists after a message
//     (M13). MACRO counts it as a warning, on the input's last line (M13's
//     "9 (1)"): VMS's SUMSLP had read the whole input when it reported it.
//   - In the closing summary's list of lines, an inserted line is the
//     input's line before it and its ".n": "6.1", and "0.2" for one before
//     the first (M06, M13).
//
// Unconfirmed: a macro expansion's lines under an updated line keep the full
// binary field and no SUMSLP field (no case expanded a macro there), and
// warnings after the last line are listed after it.

// Update is how SUMSLP updated one file of the source: a line for each of
// the file's lines as the assembler is given them, and the warnings listed
// after its last line.
type Update struct {
	Lines []UpdatedLine
	After []UpdateNote

	// Inputs is the number of the input's last line, which a warning is
	// counted on in the closing summary.
	Inputs int
}

// UpdatedLine is one line of an updated file as SUMSLP numbered it.
type UpdatedLine struct {
	// Line is the input's line number: the line's own, or for an inserted
	// line the input's line before it (0 before the first).
	Line int

	// Insert is "" for an input line, or ".n" for the nth line of a run of
	// inserted ones.
	Insert string

	// Field is SUMSLP's 16 columns: the audit trail, "-n", or "".
	Field string

	// Before are the SUMSLP warnings listed before the line.
	Before []UpdateNote
}

// UpdateNote is a SUMSLP warning: its lines, "%SUM-W-..." and those naming
// the edits it is about.
type UpdateNote struct {
	Lines []string
}

// updateFieldWidth is SUMSLP's field in a listing line, and updatedBinary
// the binary field's width left beside it.
const (
	updateFieldWidth = 16
	updatedBinary    = binaryWidth - updateFieldWidth
)

// SetUpdate says file, the source's file (0 for the first, or the only one;
// SetFileLines numbers MACRO A+B's), is SUMSLP's update of its input, as u
// describes it. It is kept for every later Assemble until SetUpdate gives
// the file a nil u.
func (a *Assembler) SetUpdate(file int, u *Update) {
	if a.updates == nil {
		a.updates = map[int]*Update{}
	}

	if u == nil {
		delete(a.updates, file)

		return
	}

	a.updates[file] = u
}

// updateOf returns how SUMSLP numbered l, a line of the program, or nil if
// its file wasn't updated; last says it is the file's last line.
func (a *Assembler) updateOf(l *listLine) (u *UpdatedLine, last bool) {
	up := a.updates[l.file]
	if up == nil || l.depth > 0 || l.number < 1 || l.number > len(up.Lines) {
		return nil, false
	}

	return &up.Lines[l.number-1], l.number == len(up.Lines)
}

// number is the line's number as the listing's line number column shows it.
func (u *UpdatedLine) number() string {
	if u.Insert != "" {
		return u.Insert
	}

	return strconv.Itoa(u.Line)
}

// binaryWidth is the binary field's width on l's listing lines: 20 columns
// beside SUMSLP's field for a line of an updated file.
func (l *listLine) binaryWidth() int {
	if l.update != nil {
		return updatedBinary
	}

	return binaryWidth
}

// withField puts l's SUMSLP field over the first columns of lines[0], l's
// first listing line, if it is one (.ERROR's and .WARN's line isn't listed).
func (l *listLine) withField(lines []string) []string {
	if l.update == nil || l.update.Field == "" || len(lines) == 0 || len(lines[0]) < updateFieldWidth {
		return lines
	}

	if len(l.notes) > 0 && (l.op == ".ERROR" || l.op == ".WARN") {
		return lines
	}

	field := l.update.Field
	if len(field) > updateFieldWidth {
		field = field[:updateFieldWidth]
	}

	lines[0] = fmt.Sprintf("%-*s", updateFieldWidth, field) + lines[0][updateFieldWidth:]

	return lines
}

// listUpdateNotes adds SUMSLP's warnings to the listing at loc, as VMS's
// MACRO lists them (M13): an empty line, then another with the warning's
// lines after it, then the location again.
func listUpdateNotes(p *listPager, notes []UpdateNote, loc uint32) {
	empty := listColumns("", loc) + strings.Repeat(" ", 7)

	for _, n := range notes {
		p.add(empty)
		p.add(empty)

		for _, line := range n.Lines {
			p.add(line)
		}

		p.add(empty)
	}
}

// updateWarnings is how many SUMSLP warnings the listing shows.
func (a *Assembler) updateWarnings() int {
	n := 0

	for _, u := range a.updates {
		// A file with no lines has no last line to list them after.
		if len(u.Lines) > 0 {
			n += len(u.After)
		}

		for _, l := range u.Lines {
			n += len(l.Before)
		}
	}

	return n
}
