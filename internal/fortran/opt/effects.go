package opt

import "github.com/tucats/govax/internal/fortran/ir"

// Effects: what a statement reads and writes, and what a call may change that the
// statement doesn't name (Decision 7 of docs/planning/PHASE-84 - FORTRAN optimizer.md).
//
// # What a call may change
//
// A call says what it passes, and nothing about what the routine called does with
// it or remembers. So
//
//	      OPEN (1, ACCESS='DIRECT', RECL=4, ASSOCIATEVARIABLE=NREC)
//	      NREC = 1
//	      READ (1'NREC) X
//	      K = NREC
//
// has NREC changed by the READ, whose list doesn't name it: FTNRTL was given its
// address by the OPEN and keeps it. A $QIO's status block is written the same way,
// at some later moment. So a local whose address was ever given to anything is
// "taken", for the whole unit and for good, wherever in the unit that was. It is
// taken if it is passed by reference or by descriptor, named by %LOC, in an I/O
// list or a NAMELIST, or used by a variable format expression, which FTNRTL calls
// in the middle of the statement.
//
// Any call may read or change a taken local, anything in COMMON (which the routine
// called may declare too), and a dummy argument (whose real home may be in either
// of those in the caller). A local that isn't taken survives a call.
//
// # Why that's enough: who else could change a local
//
// A local that isn't taken could only be changed by this same routine running
// again, during the call. Whether that can happen depends on the unit.
//
// An ordinary unit can't run twice at once. Section 6.2 of the language manual rules
// it out: "A subprogram can refer to other subprograms, but it cannot refer to itself,
// either directly or indirectly." So none of its locals, static or in the frame, can
// change during a call. An AST routine or a condition handler that calls back into
// such a routine is doing what that sentence forbids, and it may arrive between any
// two instructions, not only at a call. No analysis could see that coming; the way to
// tell the compiler is the VOLATILE statement (section 4.16: "a value is entirely
// unpredictable, based on information local to the current program unit"). So
// VOLATILE is the only rule for such a variable. asts.for reenters a routine from an
// AST during $DCLAST and prints a static variable it changed, and that still holds:
// the variable is in an I/O list, so taken, and the AST came during a call.
// optasts.for is the same with VOLATILE variables no call may touch.
//
// A RECURSIVE unit (ir.Routine.Reentrant; docs/planning/PHASE-89 - reentrant FORTRAN.md) is
// the case section 6.2 leaves out: any call may lead back into it, directly or through
// other routines. The activation that call starts is a fresh one, with a frame of its
// own below the caller's, so it can't change this activation's frame, and a local
// there survives the call as an ordinary unit's does. But every activation shares the
// unit's static locals, SAVE'd, given initial values, or STATIC, so any call may
// change any of them, taken or not. That is ChangedByCall's one difference between
// the two kinds of unit, and every pass that asks what a call does asks it there.
// An AST or handler that enters a RECURSIVE unit is only another activation: it
// leaves this one's frame alone, and while it may change a static local between any
// two instructions, as in an ordinary unit, that is still VOLATILE's business.

// Effects is what a statement or terminator does to memory.
type Effects struct {
	// Reads are the places it reads, including those read just to compute an
	// address: an index, or a dummy's address in a routine with ENTRY statements.
	Reads []Place

	// Writes are the places it writes.
	Writes []Place

	// Addressed are the places whose addresses it lets out, each the whole of what
	// the address reaches.
	Addressed []Place

	// Labels are the blocks an ASSIGN in it takes the addresses of.
	Labels []*ir.Block

	// Call says it calls a routine, so it may also read and change whatever
	// Storage.ChangedByCall says.
	Call bool
}

// Storage is what's known of a routine's storage as a whole: which of its places
// have had their addresses taken, and which are volatile. Like a Graph, it is a
// picture of the routine when it was made.
type Storage struct {
	r *ir.Routine

	// taken and volatile are the ranges of $LOCAL and the frame area that are so.
	taken, volatile []Place

	// anyVolatile says something in the routine is VOLATILE.
	anyVolatile bool

	// dying are the cells whose values can die (deadcode.go).
	dying []*ir.Cell
}

// NewStorage looks over routine r.
func NewStorage(r *ir.Routine) *Storage {
	s := &Storage{r: r}

	for _, c := range r.Cells {
		if c.Sym != nil && c.Sym.Volatile {
			s.anyVolatile = true
			s.volatile = append(s.volatile, s.Whole(&ir.Mem{Cell: c, T: c.T}))
		}
	}

	for _, p := range r.Params {
		if p.Sym != nil && p.Sym.Volatile {
			s.anyVolatile = true
		}
	}

	s.findTaken()
	s.findDying()

	return s
}

// findTaken lists every place whose address leaves the routine's own code: from any
// block, reachable or not, since a pass mustn't be able to change the answer by
// deleting one; from the read-only data, whose descriptors and FORMAT blocks hold
// addresses the linker fills in, whether or not any statement uses them now; and
// everything a variable format expression touches.
func (s *Storage) findTaken() {
	expressions := ir.ExpressionBlocks(s.r)

	for _, b := range s.r.Blocks {
		e := s.BlockEffects(b)
		s.take(e.Addressed)

		if expressions[b] {
			s.take(e.Reads)
			s.take(e.Writes)
		}
	}

	for _, d := range s.r.Data {
		if d.Desc != nil && d.Desc.Target != nil {
			s.take([]Place{s.Whole(d.Desc.Target)})
		}

		if d.Format != nil && d.Format.Cache != nil {
			c := d.Format.Cache
			s.take([]Place{s.Whole(&ir.Mem{Cell: c, T: c.T})})
		}
	}
}

// take adds the local ones of the places to the list of those taken: the rest a
// call may change anyway.
func (s *Storage) take(list []Place) {
	for _, p := range list {
		if p.Region == LocalRegion || p.Region == FrameRegion {
			s.taken = append(s.taken, p)
		}
	}
}

// Taken reports whether p overlaps a local whose address has left the routine.
func (s *Storage) Taken(p Place) bool {
	return overlapsAny(p, s.taken)
}

// Volatile reports whether p is, or overlaps, something VOLATILE, which no pass may
// move, merge, or remove. An address the analysis can't follow is taken to be if
// anything is.
func (s *Storage) Volatile(p Place) bool {
	if p.Volatile || overlapsAny(p, s.volatile) {
		return true
	}

	switch p.Region {
	case UnknownRegion:
		return s.anyVolatile
	case DummyRegion:
		return p.Param.Sym != nil && p.Param.Sym.Volatile
	}

	return false
}

// overlapsAny reports whether p conflicts with any of the list.
func overlapsAny(p Place, list []Place) bool {
	for _, q := range list {
		if Conflict(p, q) {
			return true
		}
	}

	return false
}

// ChangedByCall reports whether a call may read or change p, though the call's
// arguments don't name it.
//
// A RECURSIVE unit's static locals are the difference between the two kinds of unit
// (see the top of the file): a call may come back into the unit and change any of
// them. Its frame is its own either way: a call that recurses changes the new
// activation's frame, never this one's.
func (s *Storage) ChangedByCall(p Place) bool {
	switch p.Region {
	case DataRegion, DescriptorRegion:
		return false
	case LocalRegion:
		return s.r.Reentrant || s.Taken(p) || s.Volatile(p)
	case FrameRegion:
		return s.Taken(p) || s.Volatile(p)
	}

	return true
}

// EntryWrites are the places each entry point writes before its first block: the
// dummies' address cells and which entry point it was.
func (s *Storage) EntryWrites() []Place {
	var list []Place

	add := func(c *ir.Cell) {
		if c != nil {
			list = append(list, s.Whole(&ir.Mem{Cell: c, T: c.T}))
		}
	}

	for _, p := range s.r.Params {
		add(p.Through)
	}

	add(s.r.Selector)

	return list
}

// StmtEffects is what statement st does.
func (s *Storage) StmtEffects(st ir.Stmt) Effects {
	c := collector{s: s}

	switch st := st.(type) {
	case *ir.Assign:
		c.write(st.Dst)
		c.expr(st.Src)

	case *ir.CharMove:
		c.textWrite(st.Dst)
		c.textRead(st.Src)

	case *ir.Call:
		c.e.Call = true

		if st.Through != nil {
			c.read(st.Through)
		}

		for _, a := range st.Args {
			switch {
			case a.Ref != nil:
				c.e.Addressed = append(c.e.Addressed, s.Whole(a.Ref))
				c.address(a.Ref)
			case a.Value != nil:
				c.expr(a.Value)
			}
		}

		if st.Result != nil {
			c.write(st.Result)
		}
	}

	return c.e
}

// TermEffects is what terminator t does. A Return's Value is in R0, so a terminator
// never writes memory.
func (s *Storage) TermEffects(t ir.Terminator) Effects {
	c := collector{s: s}

	switch t := t.(type) {
	case *ir.Branch:
		c.expr(t.Cond)
	case *ir.Switch:
		c.expr(t.Index)
	case *ir.Sign:
		c.expr(t.Value)
	case *ir.IndirectJump:
		c.read(t.Through)
	case *ir.Return:
		if t.Value != nil {
			c.expr(t.Value)
		}
	}

	return c.e
}

// BlockEffects is everything block b's statements and terminator do, all together.
func (s *Storage) BlockEffects(b *ir.Block) Effects {
	var all Effects

	add := func(e Effects) {
		all.Reads = append(all.Reads, e.Reads...)
		all.Writes = append(all.Writes, e.Writes...)
		all.Addressed = append(all.Addressed, e.Addressed...)
		all.Labels = append(all.Labels, e.Labels...)
		all.Call = all.Call || e.Call
	}

	for _, st := range b.Stmts {
		add(s.StmtEffects(st))
	}

	if b.End != nil {
		add(s.TermEffects(b.End))
	}

	return all
}

// collector gathers one statement's effects.
type collector struct {
	s *Storage
	e Effects
}

// expr gathers what evaluating e does.
func (c *collector) expr(e ir.Expr) {
	switch e := e.(type) {
	case *ir.Load:
		c.read(e.Mem)

	case *ir.Addr:
		c.e.Addressed = append(c.e.Addressed, c.s.Whole(e.Mem))
		c.address(e.Mem)

	case *ir.LabelAddr:
		c.e.Labels = append(c.e.Labels, e.Block)

	case *ir.ArgPointer:
		c.through(e.Param)

	case *ir.Op:
		c.expr(e.X)

		if e.Y != nil {
			c.expr(e.Y)
		}

	case *ir.Cmp:
		c.expr(e.X)
		c.expr(e.Y)

	case *ir.Field:
		c.expr(e.X)
		c.expr(e.Pos)
		c.expr(e.Size)

	case *ir.InRange:
		c.expr(e.X)
		c.expr(e.Low)
		c.expr(e.High)

	case *ir.CharCmp:
		c.textRead(e.X)
		c.textRead(e.Y)

	case *ir.CharIndex:
		c.textRead(e.X)
		c.textRead(e.Y)
		c.expr(e.Base)
	}
}

// read gathers reading m.
func (c *collector) read(m *ir.Mem) {
	c.e.Reads = append(c.e.Reads, c.s.Place(m))
	c.address(m)
}

// write gathers writing m.
func (c *collector) write(m *ir.Mem) {
	c.e.Writes = append(c.e.Writes, c.s.Place(m))
	c.address(m)
}

// textRead gathers reading a CHARACTER value.
func (c *collector) textRead(t ir.Text) {
	c.e.Reads = append(c.e.Reads, c.s.TextPlace(t))
	c.address(t.Mem)
	c.expr(t.Len)
}

// textWrite gathers writing a CHARACTER value.
func (c *collector) textWrite(t ir.Text) {
	c.e.Writes = append(c.e.Writes, c.s.TextPlace(t))
	c.address(t.Mem)
	c.expr(t.Len)
}

// address gathers what finding m's address reads. An address computed only to reach
// m (a packed field's) goes no further than the instruction, so it isn't let out.
func (c *collector) address(m *ir.Mem) {
	if m.Index != nil {
		c.expr(m.Index)
	}

	if m.Param != nil {
		c.through(m.Param)
	}

	if a, ok := m.Ptr.(*ir.Addr); ok {
		c.address(a.Mem)
	} else if m.Ptr != nil {
		c.expr(m.Ptr)
	}
}

// through gathers reading a dummy's address from its cell, in a routine with ENTRY
// statements; elsewhere it is in the argument list, which nothing changes.
func (c *collector) through(p *ir.Param) {
	if p.Through != nil {
		c.e.Reads = append(c.e.Reads, c.s.Whole(&ir.Mem{Cell: p.Through, T: ir.Long}))
	}
}
