package ir

import (
	"fmt"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Lower lowers each program unit of a program semantic analysis found no
// error in into a Routine.
func Lower(p *sema.Program) []*Routine {
	routines := make([]*Routine, 0, len(p.Units))

	for _, u := range p.Units {
		routines = append(routines, lowerUnit(u))
	}

	return routines
}

// lowerer is lowering's state for one routine.
type lowerer struct {
	unit *sema.Unit
	r    *Routine

	// cells and params are the routine's places for the unit's
	// symbols.
	cells  map[*sema.Symbol]*Cell
	params map[*sema.Symbol]*Param

	// bound holds the places of the statement functions' dummy arguments,
	// while a reference is being expanded (lowerstmtfunc.go).
	bound map[*sema.Symbol]*Mem

	// cur is the block statements are being added to, or nil when the
	// code being lowered can't be reached (after a GO TO, a STOP, a
	// RETURN), until a block starts.
	cur *Block

	// from is the FORTRAN statement being lowered.
	from ast.Stmt

	// labels are the blocks that start at FORTRAN statement labels,
	// made at the first jump to them or when the statement is reached.
	labels map[int]*Block

	// made counts the blocks made, and placed those laid out, so that a
	// block made and never laid out (a mistake of lowering's) is found.
	made int

	// free are the temporaries and descriptor cells not in use, by type
	// or by descriptor; busy are those the statement being lowered uses,
	// given back at its end.
	free     map[Type][]*Cell
	freeDesc map[string][]*Cell
	freeText map[int][]*Cell
	busy     []*Cell

	// after are statements to be done at the end of the statement being
	// lowered, before its block's terminator if it has one: the freeing
	// of its dynamic strings.
	after []Stmt

	// data are the routine's data by their contents, so that each
	// constant, text, descriptor, and control block is made once.
	data map[string]*Datum

	// entries are the routine's entry points by ENTRY statement, and
	// returns the blocks that return a value of each type in a function whose
	// entry points' types differ (lowerentry.go).
	entries map[*sema.Entry]*Entry

	// adjustable is each adjustable array's descriptor cell, and
	// adjustableOrder the arrays in the unit's order (lowerbounds.go).
	adjustable      map[*sema.Symbol]*Cell
	adjustableOrder []*sema.Symbol

	returns []*returnBlock

	// The numbers of the compiler's names: temporaries ($T), constants
	// and texts ($K), descriptors ($D), control blocks ($IO), and DO
	// loops (the $C and $S cells).
	temps, konsts, descs, controls, loops int
}

// lowerUnit lowers one program unit.
func lowerUnit(u *sema.Unit) *Routine {
	r := &Routine{Name: u.Name, Unit: u, LocalSize: u.LocalSize, Initials: u.Initials}

	l := &lowerer{
		unit:     u,
		r:        r,
		cells:    map[*sema.Symbol]*Cell{},
		params:   map[*sema.Symbol]*Param{},
		bound:    map[*sema.Symbol]*Mem{},
		labels:   map[int]*Block{},
		free:     map[Type][]*Cell{},
		freeDesc: map[string][]*Cell{},
		freeText: map[int][]*Cell{},
		data:     map[string]*Datum{},
	}

	// A hidden argument moves the dummy arguments along by one.
	hidden := 0

	switch u.Kind {
	case ast.FunctionUnit:
		r.Kind = Function

		if u.Result.Type.HiddenResult() {
			hidden = 1
			p := &Param{Name: u.Result.Name, Index: 1, T: l.typeOf(u.Result.Type), Sym: u.Result}
			r.Params = append(r.Params, p)
			l.params[u.Result] = p
		} else {
			r.Result = l.typeOf(u.Result.Type)
		}
	case ast.SubroutineUnit:
		r.Kind = Subroutine
	case ast.BlockDataUnit:
		r.Kind = BlockData
	default:
		r.Kind = Main
	}

	for _, s := range u.Dummies {
		p := &Param{Name: s.Name, Index: s.Dummy + hidden, T: l.typeOf(s.Type), Sym: s}
		r.Params = append(r.Params, p)
		l.params[s] = p
	}

	// The unit's variables and arrays keep the places semantic analysis
	// gave them; the compiler's cells come after them.
	for _, s := range u.Symbols {
		if s.External && s.Dummy == 0 {
			r.Externals = append(r.Externals, s.Name)
		}

		if s.Offset < 0 {
			continue
		}

		c := &Cell{Name: s.Name, Kind: VariableCell, T: l.typeOf(s.Type), Size: s.Size(), Offset: s.Offset, Common: s.Common, Sym: s}

		switch {
		case s.Result:
			c.Kind = ResultCell
		case s.Class == sema.Array:
			c.Kind = ArrayCell
		}

		r.Cells = append(r.Cells, c)
		l.cells[s] = c
	}

	if r.Kind == BlockData {
		return r
	}

	if len(u.Entries) > 0 {
		l.prepareEntries(hidden)
	}

	l.prepareBounds()

	l.begin(l.newBlock())
	l.computeBounds(u.Params)
	l.stmts(u.Body)

	if l.cur != nil {
		// Only a unit whose END semantic analysis didn't find (an error)
		// can end without a terminator; but lowering never leaves a
		// block open.
		l.returnFromRoutine()
	}

	l.layOutReturns()
	l.layOutEntryBounds()

	if l.made != len(r.Blocks) {
		panic(fmt.Sprintf("ir: %s: %d blocks made, %d laid out", r.Name, l.made, len(r.Blocks)))
	}

	for i, b := range r.Blocks {
		b.Name = "B" + strconv.Itoa(i+1)
	}

	return r
}

// typeOf is the machine type of a FORTRAN type: an INTEGER or LOGICAL is a
// byte, a word, or a longword by its size; a REAL*4 F_floating, a REAL*8
// D_floating, or G_floating if the unit says so (/G_FLOATING), and a
// REAL*16 H_floating. A COMPLEX value as a whole is a Quad or an Octa, which
// only moves; its parts have the type partOf says. A CHARACTER value is
// bytes, its place a Byte place (Text).
func (l *lowerer) typeOf(t sema.Type) Type {
	switch {
	case t.Kind == sema.Character:
		return Byte
	case t.Kind == sema.Complex && t.Size == 8:
		return Quad
	case t.Kind == sema.Complex:
		return Octa
	case t.Kind == sema.Real && t.Size == 8 && l.unit.GFloating:
		return GFloat
	case t.Kind == sema.Real && t.Size == 8:
		return Double
	case t.Kind == sema.Real && t.Size == 16:
		return HFloat
	case t.Kind == sema.Real:
		return Float
	case t.Kind != sema.Integer && t.Kind != sema.Logical:
		return Long
	case t.Size == 1:
		return Byte
	case t.Size == 2:
		return Word
	}

	return Long
}

// dtypeOf is a FORTRAN type's descriptor data type.
func (l *lowerer) dtypeOf(t sema.Type) DType {
	switch t.Kind {
	case sema.Logical:
		switch t.Size {
		case 1:
			return DTypeBU
		case 2:
			return DTypeWU
		}

		return DTypeLU
	case sema.Real:
		switch l.typeOf(t) {
		case Double:
			return DTypeD
		case GFloat:
			return DTypeG
		case HFloat:
			return DTypeH
		}

		return DTypeF
	case sema.Complex:
		switch l.partOf(t) {
		case Double:
			return DTypeDC
		case GFloat:
			return DTypeGC
		}

		return DTypeFC
	case sema.Character:
		return DTypeT
	}

	if t.Size == 2 {
		return DTypeW
	}

	return DTypeL
}

// origin is the statement being lowered, for the IR statement made from
// it.
func (l *lowerer) origin() origin {
	return origin{l.from}
}

// newBlock makes a block, to be laid out later by begin.
func (l *lowerer) newBlock() *Block {
	l.made++

	return &Block{}
}

// begin lays out b after the blocks laid out so far and makes it the
// current block. If the block before it is still open, control falls
// from it into b: it ends with a jump to b, which the code generator
// writes as nothing, b being next.
func (l *lowerer) begin(b *Block) {
	if l.cur != nil {
		l.cur.End = &Jump{l.origin(), b}
	}

	l.r.Blocks = append(l.r.Blocks, b)
	l.cur = b
}

// labelBlock is the block that starts at FORTRAN label n.
func (l *lowerer) labelBlock(n int) *Block {
	b := l.labels[n]
	if b == nil {
		b = l.newBlock()
		b.Label = n
		l.labels[n] = b
	}

	return b
}

// startLabel starts the block for the statement labeled n, which a jump
// may go to. A block just begun that nothing has been put in yet (a DO
// loop's body, an IF block's) takes the label itself, unless a jump to
// the label has already made its block.
func (l *lowerer) startLabel(n int) {
	if c := l.cur; c != nil && len(c.Stmts) == 0 && c.Label == 0 && l.labels[n] == nil {
		c.Label = n
		l.labels[n] = c

		return
	}

	l.begin(l.labelBlock(n))
}

// emit adds s to the current block. Code that can't be reached (an
// unlabeled statement after a GO TO) still gets a block of its own, so
// that every statement has its code; a later optimization may drop it.
func (l *lowerer) emit(s Stmt) {
	if l.cur == nil {
		l.begin(l.newBlock())
	}

	l.cur.Stmts = append(l.cur.Stmts, s)
}

// terminate ends the current block with t, after the statements to be done
// at the statement's end. Until another block begins, what follows can't be
// reached.
func (l *lowerer) terminate(t Terminator) {
	l.flush()

	if l.cur == nil {
		l.begin(l.newBlock())
	}

	l.cur.End = t
	l.cur = nil
}

// flush adds the statements to be done at the end of the statement being
// lowered (after) to the current block.
func (l *lowerer) flush() {
	list := l.after
	l.after = nil

	for _, s := range list {
		l.emit(s)
	}
}

// jump ends the current block, if there is one, with a jump to b.
func (l *lowerer) jump(b *Block) {
	if l.cur != nil {
		l.terminate(&Jump{l.origin(), b})
	}
}

// branch ends the current block with a branch on cond: to t if it
// holds, f if not. A constant condition is a jump.
func (l *lowerer) branch(cond Expr, t, f *Block) {
	if c, ok := cond.(*Const); ok {
		to := f
		if c.Bits&1 != 0 {
			to = t
		}

		l.terminate(&Jump{l.origin(), to})

		return
	}

	l.terminate(&Branch{l.origin(), cond, t, f})
}

// newCell adds a compiler's cell to the routine's storage, after the
// cells before it, on a longword boundary.
func (l *lowerer) newCell(name string, kind CellKind, t Type, size int) *Cell {
	c := &Cell{Name: name, Kind: kind, T: t, Size: size, Offset: l.r.LocalSize}
	l.r.Cells = append(l.r.Cells, c)
	l.r.LocalSize += (size + 3) &^ 3

	return c
}

// temp is a temporary of type t for the statement being lowered: one
// another statement used and gave back, or a new one. A routine has as
// many temporaries of a type as its busiest statement needs.
func (l *lowerer) temp(t Type) *Mem {
	var c *Cell

	if free := l.free[t]; len(free) > 0 {
		c = free[len(free)-1]
		l.free[t] = free[:len(free)-1]
	} else {
		l.temps++
		c = l.newCell("$T"+strconv.Itoa(l.temps), TempCell, t, t.Size())
	}

	l.busy = append(l.busy, c)

	return &Mem{Cell: c, T: t}
}

// descriptorCell is a descriptor cell for the statement being lowered,
// holding d (whose addresses the statement stores).
func (l *lowerer) descriptorCell(d *Descriptor) *Cell {
	key := descriptorKey(d)

	var c *Cell

	if free := l.freeDesc[key]; len(free) > 0 {
		c = free[len(free)-1]
		l.freeDesc[key] = free[:len(free)-1]
	} else {
		l.descs++
		c = l.newCell("$D"+strconv.Itoa(l.descs), DescriptorCell, Long, d.Size())
		c.Desc = d
	}

	l.busy = append(l.busy, c)

	return c
}

// release gives back the temporaries and descriptor cells the statement
// just lowered used, for the next statement to use again.
func (l *lowerer) release() {
	for _, c := range l.busy {
		switch c.Kind {
		case DescriptorCell:
			key := descriptorKey(c.Desc)
			l.freeDesc[key] = append(l.freeDesc[key], c)
		case TextCell:
			l.freeText[c.Size] = append(l.freeText[c.Size], c)
		default:
			l.free[c.T] = append(l.free[c.T], c)
		}
	}

	l.busy = l.busy[:0]
}

// loopCell is a DO loop's count ($Cn) or step ($Sn), loop n, named name
// (loopName). Each loop has its own: they live as long as the loop does.
func (l *lowerer) loopCell(kind CellKind, n int, t Type, name string) *Mem {
	prefix := "$C"
	if kind == StepCell {
		prefix = "$S"
	}

	c := l.newCell(prefix+strconv.Itoa(n), kind, t, t.Size())
	c.Loop = name

	return &Mem{Cell: c, T: t}
}

// datum is the routine's datum with the contents key, made by build the
// first time.
func (l *lowerer) datum(key string, build func() *Datum) *Datum {
	if d := l.data[key]; d != nil {
		return d
	}

	d := build()
	l.data[key] = d
	l.r.Data = append(l.r.Data, d)

	return d
}

// constant is a datum holding the constant c, for passing it by
// reference.
func (l *lowerer) constant(c *Const) *Mem {
	key := fmt.Sprintf("K %s %x %x", c.T, c.High, c.Bits)

	d := l.datum(key, func() *Datum {
		l.konsts++

		return &Datum{Name: "$K" + strconv.Itoa(l.konsts), Kind: ConstantDatum, Value: c}
	})

	return &Mem{Datum: d, T: c.T}
}

// text is a datum holding a character constant's text, a Byte place.
func (l *lowerer) text(s string) *Mem {
	d := l.datum("text "+s, func() *Datum {
		l.konsts++

		return &Datum{Name: "$K" + strconv.Itoa(l.konsts), Kind: TextDatum, Text: s}
	})

	return &Mem{Datum: d, T: Byte}
}

// descriptor is a datum holding the descriptor d, whose Target's address
// the linker knows.
func (l *lowerer) descriptor(d *Descriptor) *Mem {
	key := "desc " + descriptorKey(d) + " routine " + d.Routine

	if d.Target != nil {
		key = "desc " + descriptorKey(d) + " " + memKey(d.Target)
	}

	datum := l.datum(key, func() *Datum {
		l.descs++

		return &Datum{Name: "$D" + strconv.Itoa(l.descs), Kind: DescriptorDatum, Desc: d}
	})

	return &Mem{Datum: datum}
}

// control is the datum holding an I/O statement's control block.
func (l *lowerer) control(c Control) *Mem {
	key := fmt.Sprintf("control %d", c.Flags)

	d := l.datum(key, func() *Datum {
		l.controls++

		return &Datum{Name: "$IO" + strconv.Itoa(l.controls), Kind: ControlDatum, Control: &c}
	})

	return &Mem{Datum: d}
}

// descriptorKey is what a descriptor says, but for where its value is.
func descriptorKey(d *Descriptor) string {
	return fmt.Sprintf("%s %s %d %v", d.DType, d.Class, d.Length, d.Dims)
}

// memKey identifies a place whose address the linker knows (a cell or
// datum and an offset), for finding a descriptor of it already made.
func memKey(m *Mem) string {
	switch {
	case m.Cell != nil:
		return fmt.Sprintf("cell %s%+d", m.Cell.Name, m.Offset)
	case m.Datum != nil:
		return fmt.Sprintf("datum %s%+d", m.Datum.Name, m.Offset)
	}

	panic("ir: a descriptor datum of a place without a fixed address")
}
