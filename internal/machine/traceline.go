package machine

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/dbgtable"
)

// fortranMarkerReach is how far into a MACRO file the FORTRAN compiler's
// traceback marker can be: it follows the .TITLE and .IDENT and the debug
// directives, at the file's start.
const fortranMarkerReach = 8

// fortranTraceLine is the line a traceback shows for pc, given line, the line
// the image's debug symbol table gives it (0 for none). A FORTRAN program's
// object assembled without /DEBUG numbers its lines by the FORTRAN source
// already (internal/asm/fortranline.go), and has no source correlation, so
// line stands. Assembled with /DEBUG, its table numbers the MACRO file's
// lines, which the debugger needs (docs/planning/PHASE-110 - Fortran 90
// compiler usability.md, subtask 7c): then, if the MACRO file can be read and
// is the compiler's (dbgtable.TracebackMarker), the line is the FORTRAN line
// whose comment comes last at or before the MACRO line, as the debugger shows
// it, or none. A MACRO program's line, or one whose file can't be read, stands.
func (m *Machine) fortranTraceLine(pc uint32, exact bool, line int) int {
	if line == 0 {
		return 0
	}

	at := pc
	if !exact {
		at-- // a return address: the line of the instruction before it
	}

	records, index, ok := m.SourceAt(at)
	if !ok || !fortranMade(records) {
		return line
	}

	for ; index >= 0; index-- {
		if n, isLine := dbgtable.FortranLine(records[index]); isLine {
			return n
		}
	}

	return 0
}

// fortranMade reports whether a MACRO file's records are the FORTRAN
// compiler's, with traceback records: whether its marker is near its start.
func fortranMade(records []string) bool {
	for _, r := range records[:min(len(records), fortranMarkerReach)] {
		if strings.TrimRight(r, " \t\r") == dbgtable.TracebackMarker {
			return true
		}
	}

	return false
}
