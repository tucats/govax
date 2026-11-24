package machine

import (
	"strings"

	"github.com/tucats/govax/internal/dbgsym"
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

// numberFortranLines gives each module of prog whose source is the FORTRAN
// compiler's MACRO file the FORTRAN source's line numbers (dbgsym.Module.Renumber;
// docs/planning/PHASE-113 - Fortran errata.md, subtask 9): a listing line's number is
// that of the FORTRAN line whose comment comes last at or before it in the MACRO file,
// and code before any such comment is in no line. A line of an included file keeps its
// own file's number, as a traceback shows it. Any other module, or one whose source
// can't be read, keeps the listing's numbering. The source is read when an image is
// loaded, and again when the files may have changed (ForgetSources, SET SOURCE).
func (m *Machine) numberFortranLines(prog *dbgsym.Program) {
	if prog == nil {
		return
	}

	for _, mod := range prog.Modules {
		if len(mod.Listing) == 0 || len(mod.Files) == 0 {
			continue
		}

		records := m.ModuleSource(mod)
		if !fortranMade(records) {
			mod.Renumber(nil)

			continue
		}

		// Each record's FORTRAN line, carried down from the last comment.
		numbers := make([]int, len(records))
		current := 0

		for i, r := range records {
			if n, ok := dbgtable.FortranLine(r); ok {
				current = n
			}

			numbers[i] = current
		}

		mod.Renumber(func(listing int) (int, bool) {
			file, record, ok := mod.SourceOf(listing)
			if !ok || file.Spec != mod.Files[0].Spec || record < 1 || record > len(numbers) {
				return 0, false
			}

			return numbers[record-1], numbers[record-1] > 0
		})
	}
}

// numberAllFortranLines is numberFortranLines for every image the current process
// has loaded.
func (m *Machine) numberAllFortranLines() {
	for _, icb := range m.images().ICBList {
		m.numberFortranLines(icb.Debug)
	}
}

// fortranHeaderReach is how far into a MACRO file the compiler's header, with
// the "; from" line naming each FORTRAN source file, can reach.
const fortranHeaderReach = 30

// FortranLines returns the lines of the FORTRAN source a module's lines are
// numbered by (numberFortranLines), by number, or nil for a module whose lines
// are its listing's. They are the FORTRAN file's own when the compiler's header
// names one file and it can be found as a source file is (SET SOURCE); else,
// for a compilation of several files or a file not found, the lines the MACRO
// file has as comments, each with its continuations joined, which are the
// same but for the lines of a file compiled after the first. A line of an
// included file is never among them.
func (m *Machine) FortranLines(mod *dbgsym.Module) map[int]string {
	if !mod.Renumbered() {
		return nil
	}

	records := m.ModuleSource(mod)

	var specs []string

	for _, r := range records[:min(len(records), fortranHeaderReach)] {
		if spec, ok := strings.CutPrefix(r, "; from "); ok {
			specs = append(specs, strings.TrimSpace(spec))
		}
	}

	lines := map[int]string{}

	if len(specs) == 1 {
		if file := m.sourceRecords(specs[0]); len(file) > 0 {
			for i, text := range file {
				lines[i+1] = text
			}

			return lines
		}
	}

	last := 0

	for _, r := range records {
		if n, ok := dbgtable.FortranLine(r); ok {
			last = 0

			if !strings.Contains(strings.TrimSpace(r[1:6]), " ") {
				last = n
				lines[n] = r[dbgtable.FortranLinePrefix:]
			}

			continue
		}

		if text, ok := strings.CutPrefix(r, ";    -  "); ok && last > 0 {
			lines[last] += text
		} else if !strings.HasPrefix(r, ";") {
			last = 0
		}
	}

	return lines
}
