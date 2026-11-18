package codegen

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/dbgtable"
	"github.com/tucats/govax/internal/fortran/ir"
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

	// The dummy arguments: found through the argument list (argument N), or
	// in a routine with ENTRY statements through the cell its entry points
	// copy the address to (Through).
	for _, p := range g.r.Params {
		// An assumed-shape dummy is reached through the class NCA copy its
		// entry points make of the caller's descriptor, which has its own lower
		// bounds (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md,
		// finding 14).
		if p.Sym != nil && p.Sym.AssumedShape() {
			for _, c := range g.r.Cells {
				if c.Desc != nil && c.Dummy == p.Name {
					lines = append(lines, g.describedVar(p.Name, p.Sym, c).Line())
				}
			}

			continue
		}

		place := dbgtable.Place{Kind: dbgtable.Argument, N: p.Index}
		switch {
		case p.Through != nil && p.Through.Frame:
			place = dbgtable.Place{Kind: dbgtable.ThroughFrame, N: p.Through.Offset - g.frameSize()}
		case p.Through != nil:
			place = dbgtable.Place{Kind: dbgtable.Through, Symbol: g.names.symbol(p.Through.Name)}
		}

		lines = append(lines, g.debugVar(p.Name, p.Sym, place).Line())
	}

	// The variables: each where its cell is, in a register the optimizer
	// gave it, in the stack frame, or in static storage. A variable kept in
	// a register only while a loop runs gets a loop line too, so the
	// debugger knows its value is in that register during the loop.
	// strings.CutSuffix returns the text without the suffix and whether the
	// suffix was there.
	for _, c := range g.r.Cells {
		if name, inLoop := strings.CutSuffix(c.Holds, ", during a loop"); inLoop && c.Register != 0 {
			lines = append(lines, dbgtable.LoopLine(g.r.Name, name, c.Register))
		}

		// Cells of the compiler's own (no Sym, or a symbol of lowering's own,
		// $G or $W) aren't the program's variables, and names a text library's
		// module declared (INCLUDE '($SSDEF)') that the unit never used would
		// only clutter the table, as they would the storage map. An allocatable
		// array's cell is its class NCA descriptor, an automatic array's its
		// class A one, an automatic CHARACTER scalar's its class S one: each is
		// Described, reached through that descriptor (finding 14).
		switch {
		case c.Sym == nil || c.Sym.UnusedFromLibrary() || strings.HasPrefix(c.Sym.Name, "$"):

		case c.Sym.Allocatable || c.Sym.AutomaticObject:
			lines = append(lines, g.describedVar(c.Sym.Name, c.Sym, c).Line())

		case c.Register != 0:
			lines = append(lines, g.debugVar(c.Sym.Name, c.Sym, dbgtable.Place{Kind: dbgtable.InRegister, N: c.Register}).Line())

		case c.Frame:
			lines = append(lines, g.debugVar(c.Sym.Name, c.Sym, dbgtable.Place{Kind: dbgtable.Frame, N: c.Offset - g.frameSize()}).Line())

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
		switch {
		case c.Desc == nil || c.Dummy != name:
		case c.Frame:
			v.DescriptorInFrame, v.DescriptorOffset = true, c.Offset-g.frameSize()
		default:
			v.Descriptor = g.names.symbol(c.Name)
		}
	}

	return v
}

// describedVar is the entry of a variable whose own storage is the descriptor c
// (dbgtable.Described): its place and its bounds' descriptor are both c.
func (g *routineGen) describedVar(name string, s *sema.Symbol, c *ir.Cell) dbgtable.Var {
	place := dbgtable.Place{Kind: dbgtable.Described, Symbol: g.names.symbol(c.Name)}
	if c.Frame {
		place = dbgtable.Place{Kind: dbgtable.DescribedFrame, N: c.Offset - g.frameSize()}
	}

	v := g.debugVar(name, s, place)
	v.Descriptor, v.DescriptorInFrame, v.DescriptorOffset = "", false, 0

	if c.Frame {
		v.DescriptorInFrame, v.DescriptorOffset = true, c.Offset-g.frameSize()
	} else {
		v.Descriptor = g.names.symbol(c.Name)
	}

	// Every bound is the descriptor's: an allocatable array's are none of them
	// known when compiling, and an assumed-shape dummy's upper ones aren't.
	for i := range v.Bounds {
		v.Bounds[i] = dbgtable.Bound{}
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
		case s.Type.Size == 32:
			return dbgtable.ComplxH
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
