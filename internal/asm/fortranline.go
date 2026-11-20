package asm

import (
	"github.com/tucats/govax/internal/fortran/dbgtable"
	"github.com/tucats/govax/internal/obj"
)

// This file gives a FORTRAN program's object a traceback line-number table
// (docs/planning/PHASE-110 - Fortran 90 compiler usability.md, subtask 7c).
//
// VAX FORTRAN's objects carry a line-number table in their traceback (TBT)
// records, so that a traceback names the FORTRAN line a PC is in without
// /DEBUG. govax's compiler writes MACRO, and MACRO's traceback records hold
// no line numbers: its line-number table goes in debugger (DBG) records,
// under /DEBUG, and numbers the MACRO file's lines. So the compiler marks
// its MACRO file with a comment, dbgtable.TracebackMarker, and when govax's
// MACRO sees it among the program's own lines, it numbers each line of code
// by the FORTRAN source line whose comment came last before it
// (dbgtable.FortranLine: ";   12  ...") and writes that table in the
// traceback records, after the routine begin records. LINK copies it into
// the image's debug symbol table, where a traceback finds it
// (dbgsym.Program.TraceName).
//
// Without the marker nothing changes, so any other MACRO file assembles to
// what VMS's MACRO makes of it; with it, only the traceback records differ.
// Under /DEBUG the MACRO file's own table, in the DBG records, is the one
// the debugger needs, and a module has only one: then no FORTRAN table is
// written, and the traceback finds the FORTRAN line through the MACRO
// file's comments (the machine's TraceName), as the debugger does.

// noteFortranLine looks at raw, a line of the program's own source: the
// compiler's marker turns FORTRAN numbering on, and a FORTRAN source line's
// comment is the line the code after it came from.
func (a *Assembler) noteFortranLine(raw string) {
	if a.dialect != DialectMACRO {
		return
	}

	if raw == dbgtable.TracebackMarker {
		a.fortranLines = true

		return
	}

	if !a.fortranLines {
		return
	}

	if n, ok := dbgtable.FortranLine(raw); ok {
		a.fortranLine = n
	}
}

// fortranLineRecords returns the traceback records' line-number table for
// a program with the compiler's marker, or nil: none for any other program,
// or under /DEBUG, whose table is the MACRO file's.
func (a *Assembler) fortranLineRecords() []obj.DSTRecord {
	if !a.fortranLines || a.debugSeen {
		return nil
	}

	rows := a.rowsOf(func(l *listLine) (int, bool, bool) {
		return l.fortranLine, false, l.fortranLine > 0
	})
	if len(rows) == 0 {
		return nil
	}

	var cmds []obj.DSTItem
	for i := range rows {
		cmds = append(cmds, lineCommands(rows, i)...)
	}

	cmds = append(cmds, obj.LineEnd(rows[len(rows)-1].size))

	return obj.LineNumberRecords(cmds)
}
