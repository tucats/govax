package debugger

import "github.com/tucats/govax/internal/dbgsym"

// A FORTRAN program's lines (docs/planning/PHASE-113 - Fortran errata.md, subtask 9).
//
// A program compiled /DEBUG is assembled from the MACRO file the compiler wrote, whose
// line-number table numbers that file's lines. The machine gives such a module the
// FORTRAN source's numbering instead (machine.numberFortranLines), so %LINE n, SHOW
// CALLS's line column, SET BREAK/LINE, and a location's name all count FORTRAN lines, as
// UNIT.$LINEn and the FORTRAN line a stop shows always have. TYPE and EXAMINE/SOURCE
// show the FORTRAN lines too: the source file's, or where it can't be found, those the
// MACRO file has as comments. A stop still shows the MACRO line it is at under the
// FORTRAN line, numbered as the MACRO file's.

// typeFortranLines is TYPE's lines first to last of a FORTRAN source. A line the source
// doesn't have is passed over.
func (d *Debugger) typeFortranLines(mod *dbgsym.Module, lines map[int]string, first, last int) {
	d.Machine.Printf("module %s\n", mod.Name)

	for n := max(first, 1); n <= last; n++ {
		text, ok := lines[n]
		if !ok {
			continue
		}

		d.Machine.Printf("%s", formatSource(n, text))
		d.lastSource.module, d.lastSource.line = mod, n
	}
}
