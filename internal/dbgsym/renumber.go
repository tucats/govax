package dbgsym

import "sort"

// A module's second numbering (docs/planning/PHASE-113 - Fortran errata.md, subtask 9).
//
// The line-number table numbers the lines of the file the module was assembled from. A
// FORTRAN program compiled with /DEBUG is assembled from the MACRO file the compiler
// wrote, so its table numbers that file's lines, and the FORTRAN line each instruction
// came from is a comment above it there. Renumber gives the module the FORTRAN numbering
// instead: every question about lines (%LINE n, SHOW CALLS's line column, SET BREAK/LINE,
// a location's name, a traceback) is then answered in FORTRAN lines, while Listing keeps
// the table as it was, for finding an instruction's line of the MACRO file.

// Renumber replaces the module's line numbers by number's: number(n) is the line that
// listing line n is to be, or false for a listing line that is to have none (its code is
// then in no line). Rows that become one line and follow each other in memory are one
// row, so that line n+offset counts from the line's first byte. A nil number gives the
// module back the listing's numbering.
func (m *Module) Renumber(number func(listing int) (int, bool)) {
	if number == nil {
		m.Lines = m.Listing

		return
	}

	var rows []Line

	for _, l := range m.Listing {
		n, ok := number(l.Line)
		if !ok {
			continue
		}

		if last := len(rows) - 1; last >= 0 && rows[last].Line == n && rows[last].Address+rows[last].Length == l.Address {
			rows[last].Length += l.Length

			continue
		}

		rows = append(rows, Line{Line: n, Address: l.Address, Length: l.Length})
	}

	m.Lines = rows
}

// Renumbered reports whether the module's lines are numbered other than its listing's.
func (m *Module) Renumbered() bool {
	return len(m.Lines) != len(m.Listing) || len(m.Lines) > 0 && &m.Lines[0] != &m.Listing[0]
}

// ListingAt returns the row of the listing's own table whose code holds addr: which line
// of the file the module was assembled from it is, whatever numbering Lines has.
func (m *Module) ListingAt(addr uint32) (Line, bool) {
	i := sort.Search(len(m.Listing), func(i int) bool { return m.Listing[i].Address > addr })
	if i > 0 && m.Listing[i-1].Contains(addr) {
		return m.Listing[i-1], true
	}

	return Line{}, false
}

// ListingAt returns the listing's row whose code holds addr, and its module.
func (p *Program) ListingAt(addr uint32) (Line, *Module, bool) {
	for _, m := range p.Modules {
		if l, ok := m.ListingAt(addr); ok {
			return l, m, true
		}
	}

	return Line{}, nil, false
}
