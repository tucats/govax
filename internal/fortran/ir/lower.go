package ir

import (
	"fmt"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Lower lowers each program unit of a program semantic analysis found no
// error in into a Routine, a unit's internal procedures after it.
func Lower(p *sema.Program) []*Routine {
	routines := make([]*Routine, 0, len(p.Units))

	for _, u := range p.Units {
		// An internal procedure is lowered with its host (lowerHost).
		if u.Host == nil {
			routines = append(routines, lowerHost(u, nil)...)
		}
	}

	return routines
}

// lowerer is lowering's state for one routine. As with sema's analyzer,
// lowering's functions are its methods, and its fields are what they share.
//
// The main idea: lowering appends IR statements to the current block, cur.
// Code that changes the flow of control (an IF, a DO, a GO TO) ends cur with
// a terminator and starts a new block. So lowering a FORTRAN statement is
// mostly "emit some statements into cur, perhaps end it and start another".
type lowerer struct {
	// byComponents is set while a formatted statement's items are lowered:
	// an object of a derived type is then its components (objectItems).
	byComponents bool

	unit *sema.Unit
	r    *Routine

	// host is an internal procedure's host's lowerer, whose places for its
	// names the procedure uses (lowerinternal.go); nil for any other routine.
	// reachedParams are, in a host, its dummy arguments its internal
	// procedures reach.
	host          *lowerer
	reachedParams map[*Param]bool

	// hidden is 1 for a function whose value goes through a hidden first
	// argument, else 0. valueParam is that argument when it is the class NCA
	// descriptor of a value whose bounds are known only when the program runs,
	// which the function fills in (lowervalue.go).
	hidden     int
	valueParam *Param

	// cells and params are the routine's places for the unit's
	// symbols.
	cells  map[*sema.Symbol]*Cell
	params map[*sema.Symbol]*Param

	// bound holds the places of the statement functions' dummy arguments,
	// while a reference is being expanded (lowerstmtfunc.go).
	bound map[*sema.Symbol]*Mem

	// loopStack is the DO loops being lowered, innermost last: where their
	// EXIT and CYCLE statements go (lowerloops.go).
	loopStack []*loopTargets

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

	// freeIndex are the array statements' loop indexes not in use, and
	// indexes how many the routine has (lowerarray.go).
	freeIndex []*Cell
	indexes   int

	// freeArray are the array temporaries not in use, by element type and
	// size, arrays how many the routine has, and boundArrays the symbols
	// that stand for those the statement being lowered uses (lowerarray.go).
	freeArray   map[arrayTempKey][]*Cell
	arrays      int
	boundArrays []*sema.Symbol

	// freeRuntime are the descriptor cells of run-time temporaries not in
	// use, by their descriptors' form, runtimes how many the routine has, and
	// runtime what the statement being lowered must give back (lowertemps.go).
	freeRuntime map[string][]*Cell
	runtimes    int
	runtime     runtimeTemps

	// freeValues are the descriptor cells of array-valued functions' values of
	// run-time shape not in use, by their descriptors' form, values how many the
	// routine has, and entryValues those in the frame, which the main entry
	// point must clear (lowervalue.go).
	freeValues  map[string][]*Cell
	values      int
	entryValues entryFill

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

	// lengths is each dummy argument's cell of its CHARACTER length, when
	// that isn't constant (lowerlength.go).
	lengths map[*sema.Symbol]*Cell

	// inVFE is set while a variable format expression is lowered: code that
	// runs with another routine's FP (lowervfe.go).
	inVFE bool

	// returns are the blocks that return a value of each type, in a function
	// whose entry points' types differ (entries, above; lowerentry.go).
	returns []*returnBlock

	// vfes are the FORMAT statements with variable format expressions that
	// statements used, each with its routine.
	vfes []formatVFE

	// The numbers of the compiler's names: temporaries ($T), constants
	// and texts ($K), descriptors ($D), control blocks ($IO), and DO
	// loops (the $C and $S cells).
	temps, konsts, descs, controls, loops, keyLists int

	// namelists are the cells of the NAMELIST groups whose blocks the
	// statements fill in (namelistCell), one per group.
	namelists map[*sema.Namelist]*Cell
}

// newLowerer starts lowering the unit u, an internal procedure of the routine
// host lowers if host isn't nil.
func newLowerer(u *sema.Unit, host *lowerer) *lowerer {
	r := &Routine{Name: u.Name, Unit: u, Checks: u.Checks, LocalSize: u.LocalSize, FrameSize: u.FrameSize, Initials: u.Initials,
		Reentrant: u.Recursive}

	if host != nil {
		r.Host, r.Local, r.Name = host.r, u.Name, host.r.Name+"."+u.Name
	}

	// A module procedure's name is global, M.P or its hashed form.
	if u.LinkName != "" {
		r.Name = u.LinkName
	}

	return &lowerer{
		unit:        u,
		r:           r,
		host:        host,
		cells:       map[*sema.Symbol]*Cell{},
		params:      map[*sema.Symbol]*Param{},
		bound:       map[*sema.Symbol]*Mem{},
		labels:      map[int]*Block{},
		free:        map[Type][]*Cell{},
		freeDesc:    map[string][]*Cell{},
		freeText:    map[int][]*Cell{},
		freeArray:   map[arrayTempKey][]*Cell{},
		freeRuntime: map[string][]*Cell{},
		freeValues:  map[string][]*Cell{},
		data:        map[string]*Datum{},
	}
}

// prepare makes the routine's places: its dummy arguments, its variables'
// cells, and those of its entry points and adjustable arrays.
func (l *lowerer) prepare() {
	u, r := l.unit, l.r

	// A function whose value doesn't fit in R0 and R1 (CHARACTER, REAL*16,
	// COMPLEX*16, an array) returns it through a hidden first argument: the
	// caller passes the address of a place for the result (for CHARACTER, of a
	// descriptor of one, which also gives its length), and the function
	// stores there (sema.Symbol.HiddenValue). That argument moves the dummy
	// arguments along by one. An array value is then a dummy array in all
	// but name.
	switch u.Kind {
	case ast.FunctionUnit:
		r.Kind = Function

		if u.Result.HiddenValue() {
			l.hidden = 1
			p := &Param{Name: u.Result.Name, Index: 1, T: l.typeOf(u.Result.Type), Sym: u.Result}
			r.Params = append(r.Params, p)

			if u.Result.AutomaticObject {
				l.valueCell(p)
			} else {
				l.params[u.Result] = p
			}
		} else {
			r.Result = l.resultType(u.Result.Type)
		}
	case ast.SubroutineUnit:
		r.Kind = Subroutine
	case ast.BlockDataUnit:
		r.Kind = BlockData
	case ast.ModuleUnit:
		r.Kind = Module
	default:
		r.Kind = Main
	}

	for _, s := range u.Dummies {
		p := &Param{Name: s.Name, Index: s.Dummy + l.hidden, T: l.typeOf(s.Type), Sym: s}
		r.Params = append(r.Params, p)
		l.params[s] = p
	}

	// The unit's variables and arrays keep the places semantic analysis
	// gave them; the compiler's cells come after them.
	for _, s := range u.Symbols {
		if s.Linked() {
			r.Externals = append(r.Externals, s.Name)
		}

		if s.Offset < 0 {
			continue
		}

		c := &Cell{Name: s.Name, Kind: VariableCell, T: l.typeOf(s.Type), Size: s.Size(), Offset: s.Offset, Common: s.Common, Frame: s.Automatic, Sym: s}

		switch {
		case s.Result:
			c.Kind = ResultCell
		case s.Class == sema.Array:
			c.Kind = ArrayCell
		}

		switch {
		case s.Allocatable:
			l.allocatableCell(s, c)
		case s.AutomaticObject:
			l.automaticCell(s, c)
		}

		r.Cells = append(r.Cells, c)
		l.cells[s] = c
	}

	if r.Kind.NoCode() {
		return
	}

	if len(u.Entries) > 0 {
		l.prepareEntries(l.hidden)
	} else {
		l.prepareOptionals()
	}

	l.prepareBounds()
}

// lowerBody lowers the unit's statements, after prepare, and returns the
// routine.
func (l *lowerer) lowerBody() *Routine {
	u, r := l.unit, l.r

	if r.Kind.NoCode() {
		return r
	}

	l.begin(l.newBlock())
	l.computeBounds(u.Params)
	l.clearAllocatables()
	l.entryValues = entryFill{block: l.cur, at: len(l.cur.Stmts)}
	l.stmts(u.Body)
	l.fillEntryValues()

	// The order of the work: the entry points' and adjustable arrays'
	// preparation, then the first block, the code computing the main entry's
	// array bounds, and every statement of the body. Code that must come
	// after the body (shared return blocks, each ENTRY's own bound
	// computations, variable format expressions) is laid out last.
	if l.cur != nil {
		// Only a unit whose END semantic analysis didn't find (an error)
		// can end without a terminator; but lowering never leaves a
		// block open.
		l.returnFromRoutine()
	}

	l.layOutReturns()
	l.layOutEntryBounds()
	l.layOutVFEs()

	// A panic, not an error message: a block made and never placed is a
	// bug in lowering, not in the program, and must not go unnoticed.
	if l.made != len(r.Blocks) {
		panic(fmt.Sprintf("ir: %s: %d blocks made, %d laid out", r.Name, l.made, len(r.Blocks)))
	}

	// Name the blocks in layout order, B1 first, for the dump and the
	// code generator's labels.
	for i, b := range r.Blocks {
		b.Name = "B" + strconv.Itoa(i+1)
	}

	return r
}

// typeOf is the machine type of a FORTRAN type: an INTEGER or LOGICAL is a
// byte, a word, or a longword by its size; a REAL*4 F_floating, a REAL*8
// D_floating, or G_floating if the unit says so (/G_FLOATING), and a
// REAL*16 H_floating. A COMPLEX value as a whole is a Quad or an Octa, which
// only moves, or a COMPLEX*32's a Hexa, which moves as two Octas; its parts
// have the type partOf says. A CHARACTER value is
// bytes, its place a Byte place (Text).
func (l *lowerer) typeOf(t sema.Type) Type {
	switch {
	case t.Kind == sema.Character || t.Kind == sema.Record:
		// A record is bytes, as a CHARACTER value is: a field's place
		// has the field's own type (fieldMem).
		return Byte
	case t.Kind == sema.Complex && t.Size == 8:
		return Quad
	case t.Kind == sema.Complex && t.Size == 32:
		return Hexa
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
		case HFloat:
			return DTypeHC
		}

		return DTypeFC
	case sema.Character:
		return DTypeT
	case sema.Record:
		return DTypeZ
	}

	switch t.Size {
	case 1:
		return DTypeB
	case 2:
		return DTypeW
	}

	return DTypeL
}

// here is where the statement being lowered is, for the expressions lowering
// makes; before the first statement (an entry point's code computing a bound
// from an inquiry of an assumed-shape dummy, DIMENSION (SIZE (X, 2)), the
// standard's 11.3.3.5), the unit's header.
func (l *lowerer) here() diag.Position {
	if l.from == nil {
		return l.unit.AST.Header.Position()
	}

	return l.from.Position()
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

// newCell adds a compiler's cell to the routine's storage (Routine.NewCell).
func (l *lowerer) newCell(name string, kind CellKind, t Type, size int) *Cell {
	return l.r.NewCell(name, kind, t, size)
}

// NewCell adds a compiler's cell to the routine's storage, after the cells
// before it, on a longword boundary: in $LOCAL, or in a reentrant routine in the
// frame area, which grows by it. Every cell of the compiler's, lowering's and
// the optimizer's, is made here, so that one rule decides where they go. A
// FORMAT's cache ($FCn) is $LOCAL's in any routine, being the same for every
// activation.
func (r *Routine) NewCell(name string, kind CellKind, t Type, size int) *Cell {
	c := &Cell{Name: name, Kind: kind, T: t, Size: size}

	// (size + 3) &^ 3 rounds size up to a multiple of 4: adding 3, then
	// clearing the low two bits (&^ is Go's "and not"). So every cell
	// starts on a longword boundary, which the VAX reads fastest.
	if r.Reentrant && kind != FormatCell {
		c.Frame, c.Offset = true, r.FrameSize
		r.FrameSize += (size + 3) &^ 3
	} else {
		c.Offset = r.LocalSize
		r.LocalSize += (size + 3) &^ 3
	}

	r.Cells = append(r.Cells, c)

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
	l.fillDescriptor(c)

	return c
}

// fillDescriptor stores what the compiler knows of descriptor cell c's
// descriptor (Descriptor.Fixed), if c is in the frame: a frame cell has no
// initial values, and another activation, or another statement's descriptor of
// the same form, may have left anything there. Each use writes it whole, the
// statement storing the addresses after this (Decision 5 of docs/planning/PHASE-89 -
// reentrant FORTRAN.md): cheaper than filling every descriptor at each entry point
// for a routine that only sometimes does I/O.
func (l *lowerer) fillDescriptor(c *Cell) {
	if !c.Frame {
		return
	}

	for _, f := range c.Desc.Fixed() {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: f.Offset, T: Long}, Src: LongConst(int32(f.Value))})
	}
}

// release gives back the temporaries and descriptor cells the statement
// just lowered used, for the next statement to use again.
func (l *lowerer) release() {
	// The run-time temporaries are given back here, where the statement ends,
	// if control is here; a statement's branch gives them back before it
	// (stmtCond).
	if l.pendingTemps() {
		if l.cur != nil {
			l.endTemps()
		} else {
			l.runtime = runtimeTemps{}
		}
	}

	l.runtime.funcs = nil

	for _, c := range l.busy {
		switch {
		case isValueCell(c):
			key := descriptorKey(c.Desc)
			l.freeValues[key] = append(l.freeValues[key], c)
			continue
		case c.Kind == DescriptorCell && c.Sym != nil && c.Sym.AutomaticObject:
			key := descriptorKey(c.Desc)
			l.freeRuntime[key] = append(l.freeRuntime[key], c)
			continue
		}

		switch c.Kind {
		case DescriptorCell:
			key := descriptorKey(c.Desc)
			l.freeDesc[key] = append(l.freeDesc[key], c)
		case TextCell:
			l.freeText[c.Size] = append(l.freeText[c.Size], c)
		case IndexCell:
			l.freeIndex = append(l.freeIndex, c)
		case ArrayTempCell:
			key := arrayTempKey{c.T, c.Size}
			l.freeArray[key] = append(l.freeArray[key], c)
		default:
			l.free[c.T] = append(l.free[c.T], c)
		}
	}

	l.busy = l.busy[:0]

	for _, sym := range l.boundArrays {
		delete(l.bound, sym)
	}

	l.boundArrays = l.boundArrays[:0]
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
