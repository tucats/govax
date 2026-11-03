package codegen

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/dbgtable"
	"github.com/tucats/govax/internal/fortran/sema"
)

// writeDebugTable writes what the debugger is to know of the routine's variables, as
// comments only it reads (package dbgtable), under /DEBUG. They say what the storage map
// says, in a form made to be read back.
func (g *routineGen) writeDebugTable(w *Writer) {
	if !g.debugTable {
		return
	}

	// A routine with no ENTRY statement lists no entry points, and one with them lists
	// its first too.
	lines := []string{dbgtable.EntryLine(g.r.Name, g.r.Name)}

	for _, e := range g.r.Entries {
		if e.Name != g.r.Name {
			lines = append(lines, dbgtable.EntryLine(g.r.Name, e.Name))
		}
	}

	for _, p := range g.r.Params {
		place := dbgtable.Place{Kind: dbgtable.Argument, N: p.Index}
		if p.Through != nil {
			place = dbgtable.Place{Kind: dbgtable.Through, Symbol: g.names.symbol(p.Through.Name)}
		}

		lines = append(lines, g.debugVar(p.Name, p.Sym, place).Line())
	}

	for _, c := range g.r.Cells {
		if name, inLoop := strings.CutSuffix(c.Holds, ", during a loop"); inLoop && c.Register != 0 {
			lines = append(lines, dbgtable.LoopLine(g.r.Name, name, c.Register))
		}

		switch {
		case c.Sym == nil || c.Sym.UnusedFromLibrary():

		case c.Register != 0:
			lines = append(lines, g.debugVar(c.Sym.Name, c.Sym, dbgtable.Place{Kind: dbgtable.InRegister, N: c.Register}).Line())

		case c.Frame:
			lines = append(lines, g.debugVar(c.Sym.Name, c.Sym, dbgtable.Place{Kind: dbgtable.Frame, N: c.Offset - g.r.FrameSize}).Line())

		default:
			lines = append(lines, g.debugVar(c.Sym.Name, c.Sym, dbgtable.Place{Kind: dbgtable.Static, Symbol: g.names.symbol(c.Name)}).Line())
		}
	}

	w.lines = append(w.lines, lines...)
}

// debugVar is a variable's entry in the table.
func (g *routineGen) debugVar(name string, s *sema.Symbol, place dbgtable.Place) dbgtable.Var {
	v := dbgtable.Var{Unit: g.r.Name, Name: name, Type: debugType(s, g.r.Unit.GFloating), Size: s.Type.Size, Place: place}

	for _, d := range s.Dims {
		bound := dbgtable.Bound{Lower: d.Lower, Upper: d.Upper, Known: d.LowerExpr == nil && d.UpperExpr == nil, Assumed: d.Assumed}
		v.Bounds = append(v.Bounds, bound)
	}

	for _, c := range g.r.Cells {
		if c.Desc != nil && c.Dummy == name {
			v.Descriptor = g.names.symbol(c.Name)
		}
	}

	return v
}

// debugType is a variable's type as the table has it.
func debugType(s *sema.Symbol, gFloating bool) string {
	switch s.Type.Kind {
	case sema.Integer:
		return dbgtable.Integer
	case sema.Logical:
		return dbgtable.Logical
	case sema.Character:
		return dbgtable.Char
	case sema.Real:
		switch {
		case s.Type.Size == 4:
			return dbgtable.RealF
		case s.Type.Size == 16:
			return dbgtable.RealH
		case gFloating:
			return dbgtable.RealG
		}

		return dbgtable.RealD
	case sema.Complex:
		switch {
		case s.Type.Size == 8:
			return dbgtable.ComplxF
		case gFloating:
			return dbgtable.ComplxG
		}

		return dbgtable.ComplxD
	}

	return dbgtable.Record
}


// debugLabel is the label on a FORTRAN line's first instruction, under /DEBUG, which
// the debugger can stop at: SET BREAK SUB.$LINE30. It goes among the source's lines, which no
// optimizer's pass looks at, so that /DEBUG doesn't change the code. A line of an included
// file has none, since its number could be that of a line in the file including it, and
// a line whose statement has been written already (ELSE, say, after its IF's block) has its label.
func (g *routineGen) debugLabel(w *Writer, stmt ast.Stmt, wrote bool) {
	base := stmt.Base()
	if !g.debugTable || !wrote || base.Source == nil || base.Source.Depth != 0 {
		return
	}

	number := g.src.lines[base.Source.First].Number
	if g.lineLabels[number] {
		return
	}

	if g.lineLabels == nil {
		g.lineLabels = map[int]bool{}
	}

	g.lineLabels[number] = true
	w.LabelComment(g.names.symbol("$LINE"+strconv.Itoa(number)), "")
}
