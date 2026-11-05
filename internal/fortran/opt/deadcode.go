package opt

import (
	"sort"

	"github.com/tucats/govax/internal/fortran/ir"
)

// Dead stores: a value nothing will read.
//
//	      FUNCTION F(X)
//	      F = 0.0
//	      RETURN
//	      END
//
// Forward has the RETURN give 0.0 without reading F back, and then nothing reads what
// was stored in F, so the store goes too. All that's left is CLRF R0 and RET.
//
// # Which places can die
//
// A variable can't, as VAX FORTRAN keeps every variable's value from one call to the
// next (Decision 6 of docs/PHASE-84 - FORTRAN optimizer.md, from the note to SAVE in
// section 4.14). The compiler's own temporaries, DO counts, and DO steps can, and a
// function's value, which the RETURN hands over. Anything whose address has been let
// out can't, nor anything VOLATILE (Storage.Dies).
//
// # Liveness
//
// A place is live at a point if some path from there reads it before writing it. That
// is worked out backward: a block's reads make a place live above them, a write that
// covers it makes it dead above that, and what's live at a block's end is what's live
// at the start of any block it can go to. A loop makes that circular, so it goes round
// until nothing changes; an answer can only grow, so that ends. A store to a place
// that isn't live after it reads nothing later, and can go.
//
// # A place live on entry
//
// What does it mean if a place is live where the routine starts? A path reads it with
// nothing written first, so it reads what the last call left. In
//
//	      FUNCTION KEEP(L)
//	      LOGICAL L
//	      IF (L) KEEP = 7
//	      END
//
// a call with L false returns the value of the last call with L true. That's no more
// than an accident of static storage, but it is what the program has always done, and
// such a program isn't stopped from working by turning on /OPTIMIZE. So a place live
// at any entry is taken to be live at every exit too, and its stores stay. Everything
// else that can die is written before it is read, however control gets there. The same
// rule covers a DO count read after a call comes in by an ENTRY statement in the loop.
//
// # What a dead store still has to do
//
// Removing T = A/B removes a fault if B is 0, and a program's handler may be counting
// those: Decision 8 lets a pass merge two identical operations, not lose the only one.
// So a store goes only if computing its value can't fault (mayFault).
//
// # The function's value in one piece
//
// F = A/B followed by RETURN is one value computed, stored, and read back at once.
// When the store comes just before the RETURN and F then dies, the RETURN can have the
// expression. Nothing is moved past anything, so even an expression that may fault
// can go there.
//
// # Cells nothing uses
//
// A temporary, a DO count, or a DO step no statement touches any more has no reason to
// be in $LOCAL, so it is taken out and the compiler's cells after it close up.

// DeadCode is the pass.
func DeadCode(r *ir.Routine) {
	if len(r.Blocks) == 0 {
		return
	}

	d := newLiveness(r)
	d.solve()
	d.remove()
	removeUnusedCells(r)
}

// findDying lists the cells that can die: a compiler's cell that holds values, or a
// function's value, whose address hasn't been let out and that isn't VOLATILE. A
// function's names may share one place with each other, but with nothing else.
func (s *Storage) findDying() {
	for _, c := range s.r.Cells {
		switch c.Kind {
		case ir.TempCell, ir.CountCell, ir.LimitCell, ir.StepCell, ir.ResultCell:
		default:
			continue
		}

		whole := s.Whole(&ir.Mem{Cell: c, T: c.T})

		if c.Common == nil && !s.Taken(whole) && !s.Volatile(whole) && !s.sharedWithVariable(c, whole) {
			s.dying = append(s.dying, c)
		}
	}
}

// sharedWithVariable reports whether a function's value c overlaps a cell of another
// kind.
func (s *Storage) sharedWithVariable(c *ir.Cell, whole Place) bool {
	if c.Kind != ir.ResultCell {
		return false
	}

	for _, o := range s.r.Cells {
		if o.Kind != ir.ResultCell && Conflict(whole, s.Whole(&ir.Mem{Cell: o, T: o.T})) {
			return true
		}
	}

	return false
}

// liveSet is which of the cells that can die are live, by each one's place in
// Storage.dying.
type liveSet []bool

// add makes other's members members too, and reports whether that added one.
func (l liveSet) add(other liveSet) bool {
	grew := false

	for i, live := range other {
		if live && !l[i] {
			l[i] = true
			grew = true
		}
	}

	return grew
}

// liveness is the analysis's state.
type liveness struct {
	r *ir.Routine
	s *Storage
	g *Graph

	// places are where each of the cells that can die is.
	places []Place

	// in are what's live at each block's start, and exit what's live when the
	// routine returns.
	in   map[*ir.Block]liveSet
	exit liveSet
}

// newLiveness looks over routine r.
func newLiveness(r *ir.Routine) *liveness {
	d := &liveness{r: r, s: NewStorage(r), g: NewGraph(r), in: map[*ir.Block]liveSet{}}

	for _, c := range d.s.dying {
		d.places = append(d.places, d.s.Whole(&ir.Mem{Cell: c, T: c.T}))
	}

	d.exit = d.set()

	for _, b := range d.g.Order {
		d.in[b] = d.set()
	}

	return d
}

// set is a new empty set.
func (d *liveness) set() liveSet {
	return make(liveSet, len(d.places))
}

// solve finds what's live at each block's start: see the top of the file.
func (d *liveness) solve() {
	for grew := true; grew; {
		for changed := true; changed; {
			changed = false

			for i := len(d.g.Order) - 1; i >= 0; i-- {
				b := d.g.Order[i]
				if d.in[b].add(d.atStart(b)) {
					changed = true
				}
			}
		}

		grew = false

		for _, b := range d.g.Entries {
			if d.exit.add(d.in[b]) {
				grew = true
			}
		}
	}
}

// atEnd is what's live after block b's terminator.
func (d *liveness) atEnd(b *ir.Block) liveSet {
	live := d.set()

	succs := d.g.Succs(b)
	if len(succs) == 0 {
		live.add(d.exit)
	}

	for _, s := range succs {
		live.add(d.in[s])
	}

	return live
}

// atStart is what's live before block b's first statement, from what's live after
// the blocks it can go to.
func (d *liveness) atStart(b *ir.Block) liveSet {
	live := d.atEnd(b)
	d.through(d.s.TermEffects(b.End), live)

	for j := len(b.Stmts) - 1; j >= 0; j-- {
		d.through(d.s.StmtEffects(b.Stmts[j]), live)
	}

	return live
}

// through changes live from what's live after the effects to what's live before them.
func (d *liveness) through(e Effects, live liveSet) {
	for _, p := range e.Writes {
		for i, q := range d.places {
			if p.Region == q.Region && p.Cell == q.Cell && p.Lo <= q.Lo && p.Hi >= q.Hi {
				live[i] = false
			}
		}
	}

	for _, p := range e.Reads {
		for i, q := range d.places {
			if Conflict(p, q) {
				live[i] = true
			}
		}
	}
}

// remove takes out each store whose value isn't read, and puts a value's last store
// into the RETURN that reads it.
func (d *liveness) remove() {
	for _, b := range d.g.Order {
		d.sink(b)

		live := d.atEnd(b)
		d.through(d.s.TermEffects(b.End), live)

		kept := make([]ir.Stmt, 0, len(b.Stmts))

		for j := len(b.Stmts) - 1; j >= 0; j-- {
			st := b.Stmts[j]

			if a, ok := st.(*ir.Assign); ok && d.dead(a, live) {
				continue
			}

			kept = append(kept, st)
			d.through(d.s.StmtEffects(st), live)
		}

		for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
			kept[i], kept[j] = kept[j], kept[i]
		}

		b.Stmts = kept
	}
}

// dead reports whether a stores what nothing will read, and can go.
func (d *liveness) dead(a *ir.Assign, live liveSet) bool {
	if a.Dst.Cell == nil || a.Dst.Index != nil || a.Dst.Ptr != nil {
		return false
	}

	p := d.s.Place(a.Dst)
	found := false

	for i, q := range d.places {
		if Conflict(p, q) {
			found = true

			if live[i] {
				return false
			}
		}
	}

	overflow := d.r.Checks.Overflow

	return found && !mayFault(a.Src, overflow) && !memMayFault(a.Dst, overflow)
}

// sink makes a RETURN of what the statement before it just stored a RETURN of the
// expression, if the place then dies.
func (d *liveness) sink(b *ir.Block) {
	ret, ok := b.End.(*ir.Return)
	if !ok || ret.Value == nil || len(b.Stmts) == 0 {
		return
	}

	load, ok := ret.Value.(*ir.Load)
	if !ok {
		return
	}

	a, ok := b.Stmts[len(b.Stmts)-1].(*ir.Assign)
	if !ok || a.Dst.Cell == nil || a.Dst.Index != nil || a.Dst.Ptr != nil || load.Mem.Index != nil || load.Mem.Ptr != nil || load.Mem.Cell == nil {
		return
	}

	// A CHARACTER comparison or an index must be the whole of an Assign's value.
	switch a.Src.(type) {
	case *ir.CharCmp, *ir.CharIndex:
		return
	}

	p := d.s.Place(a.Dst)
	if p != d.s.Place(load.Mem) || a.Dst.T != load.Mem.T {
		return
	}

	for i, q := range d.places {
		if Conflict(p, q) {
			if d.exit[i] {
				return
			}

			ret.Value = a.Src
			b.Stmts = b.Stmts[:len(b.Stmts)-1]

			return
		}
	}
}

// mayFault reports whether computing e can fault, by Decision 8's list: a division,
// any floating operation, or a floating load (a reserved operand). A subscript's check
// is on the list, and with overflow, a routine compiled with /CHECK=OVERFLOW, so is an
// integer operation that can overflow.
func mayFault(e ir.Expr, overflow bool) bool {
	switch e := e.(type) {
	case *ir.Load:
		return e.Mem.T.Floating() || memMayFault(e.Mem, overflow)

	case *ir.Addr:
		return memMayFault(e.Mem, overflow)

	case *ir.InRange:
		return true

	case *ir.Op:
		if e.Op == ir.Div || e.T.Floating() || e.X.Type().Floating() || (overflow && ir.MayOverflow(e)) {
			return true
		}

		return mayFault(e.X, overflow) || (e.Y != nil && mayFault(e.Y, overflow))

	case *ir.Cmp:
		return mayFault(e.X, overflow) || mayFault(e.Y, overflow)

	case *ir.Field:
		return mayFault(e.X, overflow) || mayFault(e.Pos, overflow) || mayFault(e.Size, overflow)

	case *ir.CharCmp:
		return textMayFault(e.X, overflow) || textMayFault(e.Y, overflow)

	case *ir.CharIndex:
		return textMayFault(e.X, overflow) || textMayFault(e.Y, overflow) || mayFault(e.Base, overflow)
	}

	return false
}

// memMayFault reports whether finding m's address can fault.
func memMayFault(m *ir.Mem, overflow bool) bool {
	return (m.Index != nil && mayFault(m.Index, overflow)) || (m.Ptr != nil && mayFault(m.Ptr, overflow))
}

// textMayFault reports whether finding a CHARACTER value can fault.
func textMayFault(t ir.Text, overflow bool) bool {
	return memMayFault(t.Mem, overflow) || mayFault(t.Len, overflow)
}

// removeUnusedCells takes out the compiler's temporaries, counts, and steps that no
// statement uses, and closes up the compiler's cells after each, in $LOCAL and in the
// frame. Lowering puts those after the program's own, each on a longword (NewCell),
// and a program's variable, which may have an initial value by offset, never moves.
func removeUnusedCells(r *ir.Routine) {
	used := map[*ir.Cell]bool{}
	walkMems(r, func(m *ir.Mem) { used[m.Cell] = true })

	gone := map[*ir.Cell]bool{}

	for _, c := range r.Cells {
		if c.Common == nil && !used[c] && c.Sym == nil &&
			(c.Kind == ir.TempCell || c.Kind == ir.CountCell || c.Kind == ir.LimitCell || c.Kind == ir.StepCell) {
			gone[c] = true
		}
	}

	if !closeUpBoth(r, gone) {
		return
	}

	kept := r.Cells[:0]

	for _, c := range r.Cells {
		if !gone[c] {
			kept = append(kept, c)
		}
	}

	r.Cells = kept
}

// closeUpBoth closes up $LOCAL and the frame area (closeUp) for the cells gone. A cell
// the allocator made for a register (Holds) has no place in either. It reports whether
// either changed.
func closeUpBoth(r *ir.Routine, gone map[*ir.Cell]bool) bool {
	var local, frame []*ir.Cell

	for _, c := range r.Cells {
		switch {
		case c.Common != nil || c.Holds != "":
		case c.Frame:
			frame = append(frame, c)
		default:
			local = append(local, c)
		}
	}

	byOffset := func(cells []*ir.Cell) {
		sort.SliceStable(cells, func(i, j int) bool { return cells[i].Offset < cells[j].Offset })
	}

	byOffset(local)
	byOffset(frame)

	inLocal := closeUp(local, gone, &r.LocalSize)
	inFrame := closeUp(frame, gone, &r.FrameSize)

	return inLocal || inFrame
}

// closeUp takes the cells gone out of a storage area of *size bytes, whose cells in
// order of offset are cells, and moves those after them down. It reports whether it
// did: a program's variable, which may have an initial value by offset (or, in the
// frame, a place sema's storage map gave it), never moves, so nothing can if one comes
// after a cell that's going, which isn't lowering's order.
func closeUp(cells []*ir.Cell, gone map[*ir.Cell]bool, size *int) bool {
	local := cells

	shift := 0

	for _, c := range local {
		switch {
		case gone[c]:
			shift += (c.Size + 3) &^ 3
		case c.Sym != nil && shift > 0:
			return false
		}
	}

	if shift == 0 {
		return false
	}

	moved := 0

	for _, c := range local {
		if gone[c] {
			moved += (c.Size + 3) &^ 3
		} else {
			c.Offset -= moved
		}
	}

	*size -= shift

	return true
}

// walkMems calls visit for every Mem in the routine's code and data, those inside
// other expressions included.
func walkMems(r *ir.Routine, visit func(*ir.Mem)) {
	var expr func(ir.Expr)

	mem := func(m *ir.Mem) {
		visit(m)

		if m.Index != nil {
			expr(m.Index)
		}

		if m.Ptr != nil {
			expr(m.Ptr)
		}
	}

	text := func(t ir.Text) {
		mem(t.Mem)
		expr(t.Len)
	}

	expr = func(e ir.Expr) {
		switch e := e.(type) {
		case *ir.Load:
			mem(e.Mem)
		case *ir.Addr:
			mem(e.Mem)
		case *ir.Op:
			expr(e.X)

			if e.Y != nil {
				expr(e.Y)
			}
		case *ir.Cmp:
			expr(e.X)
			expr(e.Y)
		case *ir.Field:
			expr(e.X)
			expr(e.Pos)
			expr(e.Size)
		case *ir.InRange:
			expr(e.X)
			expr(e.Low)
			expr(e.High)
		case *ir.CharCmp:
			text(e.X)
			text(e.Y)
		case *ir.CharIndex:
			text(e.X)
			text(e.Y)
			expr(e.Base)
		}
	}

	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			switch st := st.(type) {
			case *ir.Assign:
				mem(st.Dst)
				expr(st.Src)
			case *ir.CharMove:
				text(st.Dst)
				text(st.Src)
			case *ir.Call:
				if st.Through != nil {
					mem(st.Through)
				}

				for _, a := range st.Args {
					if a.Ref != nil {
						mem(a.Ref)
					} else if a.Value != nil {
						expr(a.Value)
					}
				}

				if st.Result != nil {
					mem(st.Result)
				}
			}
		}

		switch t := b.End.(type) {
		case *ir.Branch:
			expr(t.Cond)
		case *ir.Switch:
			expr(t.Index)
		case *ir.Sign:
			expr(t.Value)
		case *ir.IndirectJump:
			mem(t.Through)
		case *ir.Return:
			if t.Value != nil {
				expr(t.Value)
			}
		}
	}

	for _, d := range r.Data {
		if d.Desc != nil && d.Desc.Target != nil {
			mem(d.Desc.Target)
		}
	}
}
