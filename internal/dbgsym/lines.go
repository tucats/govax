package dbgsym

import (
	"fmt"
	"sort"
)

// Line is one row of a module's line-number table: listing line Line
// (and, in statement mode, statement Stmt of it) is the Length bytes of
// code at Address.
type Line struct {
	Line    int
	Stmt    int
	Address uint32
	Length  uint32
}

// Contains reports whether addr is in the line's code.
func (l Line) Contains(addr uint32) bool {
	return addr >= l.Address && addr-l.Address < l.Length
}

// The line-number program's commands (docs/DEBUG-RECORDS.md, section
// 13.2). A command byte of zero or less is a one-byte Delta-PC.
const (
	lnDeltaPCW       = 1
	lnIncrLinum      = 2
	lnIncrLinumW     = 3
	lnSetLinumIncr   = 4
	lnSetLinumIncrW  = 5
	lnResetLinumIncr = 6
	lnBegStmtMode    = 7
	lnEndStmtMode    = 8
	lnSetLinum       = 9
	lnSetPC          = 10
	lnSetPCW         = 11
	lnSetPCL         = 12
	lnSetStmtnum     = 13
	lnTerm           = 14
	lnTermW          = 15
	lnSetAbsPC       = 16
	lnDeltaPCL       = 17
	lnIncrLinumL     = 18
	lnSetLinumB      = 19
	lnSetLinumL      = 20
	lnTermL          = 21
)

// lineOperandSize is each command's operand size in bytes (0 for those
// with none); a command not in it is undefined. SET_STMTNUM's
// is a word, as section 13.2's note and 23.3 recommend (no MACRO image
// has one to check against).
var lineOperandSize = map[int8]int{
	lnResetLinumIncr: 0, lnBegStmtMode: 0, lnEndStmtMode: 0,
	lnDeltaPCW: 2, lnIncrLinum: 1, lnIncrLinumW: 2, lnSetLinumIncr: 1,
	lnSetLinumIncrW: 2, lnSetLinum: 2, lnSetPC: 1, lnSetPCW: 2, lnSetPCL: 4,
	lnSetStmtnum: 2, lnTerm: 1, lnTermW: 2, lnSetAbsPC: 4, lnDeltaPCL: 4,
	lnIncrLinumL: 4, lnSetLinumB: 1, lnSetLinumL: 4, lnTermL: 4,
}

// lineTable runs a module's line-number program (section 13.4) into its
// rows, sorted by address. Each Delta-PC starts a row: the line number
// moves on (by the increment, or the statement number in statement
// mode), the PC moves on by the delta, and the new line begins there. A
// row's length is the distance to the next row's start, or, for the last
// row before a TERM, the TERM's operand. startPC is the lowest routine
// address, the base of the relative SET_PC commands; base relocates
// SET_ABS_PC's absolute addresses.
func lineTable(prog []byte, startPC, base uint32) ([]Line, error) {
	var (
		rows   []Line
		line   = 0
		stmt   = 1
		incr   = 1
		stmts  = false
		pc     = startPC
		open   = false // a row is open: its length isn't known yet
		lastPC uint32
	)

	// closeRow ends the open row at end.
	closeRow := func(end uint32) {
		if open {
			rows[len(rows)-1].Length = end - lastPC
			open = false
		}
	}

	for i := 0; i < len(prog); {
		cmd := int8(prog[i])
		i++

		var n uint32

		if cmd > 0 {
			size, ok := lineOperandSize[cmd]
			if !ok {
				return nil, fmt.Errorf("line-number command %d at %d is undefined", cmd, i-1)
			}

			if i+size > len(prog) {
				return nil, fmt.Errorf("line-number command %d at %d is cut short", cmd, i-1)
			}

			for j := size - 1; j >= 0; j-- {
				n = n<<8 | uint32(prog[i+j])
			}

			i += size
		}

		switch cmd {
		case lnDeltaPCW, lnDeltaPCL:
			pc = startRow(&rows, &line, &stmt, incr, stmts, pc, n, closeRow)
			lastPC, open = pc, true

		case lnIncrLinum, lnIncrLinumW, lnIncrLinumL:
			line += int(n)

			if stmts {
				stmt = 1
			}

		case lnSetLinumB, lnSetLinum, lnSetLinumL:
			line = int(n)

		case lnSetLinumIncr, lnSetLinumIncrW:
			incr = int(n)

			if stmts {
				stmt = 1
			}

		case lnResetLinumIncr:
			incr = 1
			
			if stmts {
				stmt = 1
			}

		case lnBegStmtMode:
			stmts, stmt = true, 1

		case lnEndStmtMode:
			stmts, stmt = false, 1

		case lnSetStmtnum:
			stmt = int(n)

		case lnSetPC, lnSetPCW, lnSetPCL:
			closeRow(pc)
			pc = startPC + n

		case lnSetAbsPC:
			closeRow(pc)
			pc = n + base

		case lnTerm, lnTermW, lnTermL:
			pc += n
			closeRow(pc)

		default: // a Delta-PC: the PC moves on by -cmd
			pc = startRow(&rows, &line, &stmt, incr, stmts, pc, uint32(-int32(cmd)), closeRow)
			lastPC, open = pc, true
		}
	}

	if open {
		// No TERM: the last row's length is unknown. Leave it 1 byte,
		// so its first instruction still maps to it.
		rows[len(rows)-1].Length = 1
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Address < rows[j].Address })

	return rows, nil
}

// startRow does a Delta-PC's work: closes the open row where the new one
// starts, moves the line (or statement) on, and adds the new row.
func startRow(rows *[]Line, line, stmt *int, incr int, stmts bool, pc, delta uint32, closeRow func(uint32)) uint32 {
	pc += delta
	closeRow(pc)

	if stmts {
		*stmt++
	} else {
		*line += incr
	}

	*rows = append(*rows, Line{Line: *line, Stmt: *stmt, Address: pc})

	return pc
}

// LineAt returns the module's line whose code holds addr.
func (m *Module) LineAt(addr uint32) (Line, bool) {
	// The last row starting at or below addr is the only candidate.
	i := sort.Search(len(m.Lines), func(i int) bool { return m.Lines[i].Address > addr })
	if i > 0 && m.Lines[i-1].Contains(addr) {
		return m.Lines[i-1], true
	}

	return Line{}, false
}

// AddressOfLine returns the address of listing line n's first
// instruction in the module, if the line has code.
func (m *Module) AddressOfLine(n int) (uint32, bool) {
	found, addr := false, uint32(0)

	for _, l := range m.Lines {
		if l.Line == n && (!found || l.Address < addr) {
			found, addr = true, l.Address
		}
	}

	return addr, found
}

// LineAt returns the line whose code holds addr, and its module.
func (p *Program) LineAt(addr uint32) (Line, *Module, bool) {
	for _, m := range p.Modules {
		if l, ok := m.LineAt(addr); ok {
			return l, m, true
		}
	}

	return Line{}, nil, false
}
