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
	// its first too. A module has none: it has no code. Its variables are listed as
	// its own, each at its symbol in the module's section, as a COMMON block's are in
	// any unit, and a unit that uses the module lists them too, under its own names
	// (docs/planning/PHASE-106 - Fortran 90 modules.md, finding 13).
	var lines []string
	if g.r.Kind != ir.Module {
		lines = append(lines, dbgtable.EntryLine(g.r.Name, g.r.Name))
	}

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

		// A pointer's cell holds its target's address, not its value, which
		// the debugger doesn't follow yet (docs/planning/PHASE-107 - Fortran
		// 90 pointers.md, finding 12: Phase 110's), so it is left out rather
		// than shown as a value it isn't.
		case c.Sym.Pointer:

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

	// The layouts of the records listed, and of the records in them: the list grows
	// as each structure names a nested one.
	for i := 0; i < len(g.debugStructs); i++ {
		lines = append(lines, g.structureLines(g.debugStructs[i])...)
	}

	w.lines = append(w.lines, lines...)
}

// structName is a structure's name in the routine's table, which lists its layout
// (structureLines) once the variables are written. A DEC structure nested without a
// name is given one beginning with $, as is a structure whose name another structure
// of the table has already (a host's type and an internal procedure's own, say).
func (g *routineGen) structName(st *sema.Structure) string {
	if st == nil {
		return ""
	}

	if name, ok := g.debugStructNames[st]; ok {
		return name
	}

	if g.debugStructNames == nil {
		g.debugStructNames = map[*sema.Structure]string{}
	}

	name := st.Name
	taken := func(name string) bool {
		for _, n := range g.debugStructNames {
			if n == name {
				return true
			}
		}

		return false
	}

	if name == "" || taken(name) {
		name = "$" + strconv.Itoa(len(g.debugStructs)+1)
	}

	g.debugStructNames[st] = name
	g.debugStructs = append(g.debugStructs, st)

	return name
}

// structureLines are a structure's S line and a C line for each component. A %FILL
// field has no name to show, and a pointer component isn't followed by the debugger
// yet (subtask 7b), so neither has a line.
func (g *routineGen) structureLines(st *sema.Structure) []string {
	name := g.structName(st)
	unit := g.r.Name
	lines := []string{dbgtable.StructureLine(unit, &dbgtable.Structure{Name: name, Size: st.Size, Derived: st.Derived})}

	for _, f := range st.Fields {
		if f.Name == "%FILL" || f.Pointer || f.Rank > 0 {
			continue
		}

		c := &dbgtable.Component{Name: f.Name, Offset: f.Offset, Bounds: constantBounds(f.Dims)}

		if f.Struct != nil {
			c.Type, c.Struct, c.Size = dbgtable.Record, g.structName(f.Struct), f.Struct.Size
		} else {
			c.Type, c.Size = debugTypeOf(f.Type, g.r.Unit.GFloating), f.Type.Size
		}

		lines = append(lines, dbgtable.ComponentLine(unit, name, c))
	}

	return lines
}

// constantBounds are a field's dimensions, which are always constant.
func constantBounds(dims []sema.Dim) []dbgtable.Bound {
	var bounds []dbgtable.Bound

	for _, d := range dims {
		bounds = append(bounds, dbgtable.Bound{Lower: d.Lower, Upper: d.Upper, Known: true})
	}

	return bounds
}

// debugVar is a variable's entry in the table.
func (g *routineGen) debugVar(name string, s *sema.Symbol, place dbgtable.Place) dbgtable.Var {
	v := dbgtable.Var{Unit: g.r.Name, Name: name, Type: debugTypeOf(s.Type, g.r.Unit.GFloating), Size: s.Type.Size, Place: place}

	// A record's structure, whose layout the table gives so that the debugger can show
	// it a component at a time.
	if v.Type == dbgtable.Record {
		st := s.Type.Struct
		if st == nil {
			st = s.Struct
		}

		v.Struct = g.structName(st)
	}

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

// debugTypeOf is a type as the table has it.
func debugTypeOf(t sema.Type, gFloating bool) string {
	switch t.Kind {
	case sema.Integer:
		return dbgtable.Integer
	case sema.Logical:
		return dbgtable.Logical
	case sema.Character:
		return dbgtable.Char
	case sema.Real:
		switch {
		case t.Size == 4:
			return dbgtable.RealF
		case t.Size == 16:
			return dbgtable.RealH
		case gFloating:
			return dbgtable.RealG
		}

		return dbgtable.RealD
	case sema.Complex:
		switch {
		case t.Size == 8:
			return dbgtable.ComplxF
		case t.Size == 32:
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
