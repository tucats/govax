package opt

import "github.com/tucats/govax/internal/fortran/ir"

// Forward: using what a store just said, rather than reading it back.
//
//	      K = 3
//	      A(K) = 1.0
//	      IF (K .GT. 5) THEN
//	         CALL NEVER
//	      END IF
//
// After the first statement K holds 3, and nothing between there and its uses
// changes it, so A(K) is the third element, at an offset known now, and K .GT. 5 is
// false, so the branch needn't be tested and the CALL can't run. Each load replaced
// is a memory reference saved, and sometimes the store then has no reader left
// (deadcode.go) or a block can't be reached any more (unreachable.go).
//
// # What is known, and where
//
// A fact is "this place holds this constant". A store of a constant makes one; any
// store that can touch the place ends it (Conflict, by bytes, so a store to half of
// an INTEGER*4 through EQUIVALENCE ends the fact about the whole), as does a call
// that may change the place (Storage.ChangedByCall, Decision 7 of docs/PHASE-84 -
// FORTRAN optimizer.md). VOLATILE places have no facts.
//
// Facts cross blocks. In
//
//	      IF (L) THEN
//	         K = 1
//	      ELSE
//	         K = 1
//	      END IF
//	      J = K
//
// both arms say K holds 1, so what follows knows it, and with K = 2 in one arm it
// wouldn't. So a block starts with what all the blocks that can go to it agree on.
// An entry knows nothing, since it is reached by a call (a local's value on entry is
// whatever the last call left: Decision 6). A loop makes that circular, as its top
// depends on its bottom, which is why the analysis starts by assuming a block not yet
// looked at agrees with everything, then goes round until no answer changes. An answer
// can only shrink, so that ends. Anything still known then is true of every way of
// getting there.
//
// # Why only constants
//
// A store of another variable's value could make a fact too: after $T = N, a read of
// $T could read N. But that saves no memory reference, the temporary usually stays
// live anyway, and if N is a dummy argument its address is read as well. Measured on
// DUNGEON, forwarding copies took 413 more instructions and 47,604 more reads than
// leaving them. What copies were wanted for, a function's value, deadcode.go does.
//
// # What isn't known
//
// A constant given as bits rather than a number (Const.Raw: REAL R / 'FFFFFFFF'X /)
// may be a reserved operand. A floating load of it faults, and a floating operation
// on it can't be worked out, so it has no facts: the program's own instructions run.
//
// An operation that would fault stays as it is (ir.Fold leaves it), to fault at run
// time (Decision 8): K = 0, then J = 5/K, is still a division.
//
// A store of a value the place already holds goes, as nothing can see it happen.
//
// A length that comes out negative or too long for a character instruction's word
// isn't put in: the code generator chooses MOVC3 or MOVC5 by constant lengths and
// assumes they are lengths. Such a program is wrong, and goes on doing what it did.

// Forward is the pass. A branch it settles may leave facts it couldn't use before,
// where two ways in were one in fact, so it goes round until no branch changes.
func Forward(r *ir.Routine) {
	for {
		g := NewGraph(r)
		f := &forwarder{s: NewStorage(r), g: g, ends: map[*ir.Block]facts{}}

		f.analyze()

		if !f.commit() {
			return
		}
	}
}

// facts are the constants known to be in places, by place.
type facts map[Place]*ir.Const

// clone is a copy of the facts.
func (k facts) clone() facts {
	c := make(facts, len(k))
	for p, v := range k {
		c[p] = v
	}

	return c
}

// same reports whether the facts are the same as other's.
func (k facts) same(other facts) bool {
	if len(k) != len(other) {
		return false
	}

	for p, v := range k {
		if w, ok := other[p]; !ok || *v != *w {
			return false
		}
	}

	return true
}

// keep leaves only what other also says.
func (k facts) keep(other facts) {
	for p, v := range k {
		if w, ok := other[p]; !ok || *v != *w {
			delete(k, p)
		}
	}
}

// forwarder is the pass's state for one routine.
type forwarder struct {
	s *Storage
	g *Graph

	// ends are the facts at the end of each block looked at so far.
	ends map[*ir.Block]facts

	// known are the facts at the statement being looked at.
	known facts
}

// analyze finds what is known at the end of each block.
func (f *forwarder) analyze() {
	for changed := true; changed; {
		changed = false

		for _, b := range f.g.Order {
			f.begin(b)

			for _, st := range b.Stmts {
				f.stmt(st)
			}

			if old, ok := f.ends[b]; !ok || !old.same(f.known) {
				f.ends[b] = f.known
				changed = true
			}
		}
	}
}

// begin sets the facts to what is known at block b's start: nothing at an entry,
// and otherwise what the blocks looked at so far that can go to it agree on.
func (f *forwarder) begin(b *ir.Block) {
	f.known = nil

	if f.g.IsEntry(b) {
		f.known = facts{}

		return
	}

	for _, p := range f.g.Preds(b) {
		end, ok := f.ends[p]

		switch {
		case !ok:
		case f.known == nil:
			f.known = end.clone()
		default:
			f.known.keep(end)
		}
	}

	if f.known == nil {
		f.known = facts{}
	}
}

// commit rewrites the routine by what analyze found, and reports whether a branch
// became a jump.
func (f *forwarder) commit() bool {
	settled := false

	for _, b := range f.g.Order {
		f.begin(b)

		kept := b.Stmts[:0]

		for _, st := range b.Stmts {
			if !f.redundant(st) {
				kept = append(kept, st)
			}

			f.rewrite(st)
			f.stmt(st)
		}

		b.Stmts = kept

		if f.terminator(b) {
			settled = true
		}
	}

	return settled
}

// redundant reports whether st stores a constant in a place known to hold it.
func (f *forwarder) redundant(st ir.Stmt) bool {
	a, ok := st.(*ir.Assign)
	if !ok {
		return false
	}

	dst := f.mem(a.Dst)
	if !f.isKey(dst) {
		return false
	}

	c, isConst := f.expr(a.Src).(*ir.Const)
	if !isConst {
		return false
	}

	old, known := f.known[f.s.Place(dst)]

	return known && *old == *c
}

// rewrite replaces what st reads by what is known of it.
func (f *forwarder) rewrite(st ir.Stmt) {
	switch st := st.(type) {
	case *ir.Assign:
		st.Dst, st.Src = f.mem(st.Dst), f.expr(st.Src)

	case *ir.CharMove:
		st.Dst, st.Src = f.text(st.Dst), f.text(st.Src)

	case *ir.Call:
		if st.Through != nil {
			st.Through = f.mem(st.Through)
		}

		for i, a := range st.Args {
			switch {
			case a.Ref != nil:
				st.Args[i].Ref = f.mem(a.Ref)
			case a.Value != nil:
				st.Args[i].Value = f.expr(a.Value)
			}
		}

		if st.Result != nil {
			st.Result = f.mem(st.Result)
		}
	}
}

// stmt changes the facts by what st does, whether or not st has been rewritten: a
// rewritten statement does the same.
func (f *forwarder) stmt(st ir.Stmt) {
	switch st := st.(type) {
	case *ir.Assign:
		dst, src := f.mem(st.Dst), f.expr(st.Src)
		p := f.s.Place(dst)

		f.kill(p)

		if !f.isKey(dst) {
			return
		}

		if v, ok := src.(*ir.Const); ok && !v.Raw {
			f.known[p] = v
		}

	case *ir.CharMove:
		f.kill(f.s.TextPlace(f.text(st.Dst)))

	case *ir.Call:
		for p := range f.known {
			if f.s.ChangedByCall(p) {
				delete(f.known, p)
			}
		}

		if st.Result != nil {
			f.kill(f.s.Place(f.mem(st.Result)))
		}
	}
}

// kill ends what is known that a store to p can change.
func (f *forwarder) kill(p Place) {
	for q := range f.known {
		if Conflict(p, q) {
			delete(f.known, q)
		}
	}
}

// isKey reports whether facts can be known about m: a whole value at a place known
// exactly, and not VOLATILE.
func (f *forwarder) isKey(m *ir.Mem) bool {
	return m.Ptr == nil && m.Index == nil && (m.Cell != nil || m.Param != nil) && !f.s.Volatile(f.s.Place(m))
}

// mem is m with its index replaced by what is known of it. A constant index becomes
// part of the offset. The result is m itself if nothing changed: lowering may use one
// Mem in several places, so a new one is made rather than m changed.
func (f *forwarder) mem(m *ir.Mem) *ir.Mem {
	if m.Index == nil {
		return m
	}

	x := f.expr(m.Index)
	if x == m.Index {
		return m
	}

	n := *m

	if c, ok := x.(*ir.Const); ok {
		n.Offset += int(c.Int()) * m.T.Size()
		n.Index = nil
	} else {
		n.Index = x
	}

	return &n
}

// text is t with what is known put in. See the top of the file for the length.
func (f *forwarder) text(t ir.Text) ir.Text {
	n := ir.Text{Mem: f.mem(t.Mem), Len: f.expr(t.Len)}

	if c, ok := n.Len.(*ir.Const); ok && (c.Int() < 0 || c.Int() > maxCharLength) {
		n.Len = t.Len
	}

	return n
}

// maxCharLength is the longest length a character instruction takes, in a word.
const maxCharLength = 65535

// expr is e with what is known put in, folded. The result is e itself if nothing
// changed.
func (f *forwarder) expr(e ir.Expr) ir.Expr {
	switch e := e.(type) {
	case *ir.Load:
		m := f.mem(e.Mem)

		if f.isKey(m) {
			if c, ok := f.known[f.s.Place(m)]; ok && c.T == m.T {
				return c
			}
		}

		if m != e.Mem {
			return &ir.Load{Mem: m}
		}

	case *ir.Addr:
		if m := f.mem(e.Mem); m != e.Mem {
			return &ir.Addr{Mem: m}
		}

	case *ir.Op:
		x := f.expr(e.X)

		var y ir.Expr
		if e.Y != nil {
			y = f.expr(e.Y)
		}

		if x != e.X || y != e.Y {
			return ir.Fold(e.Op, e.T, x, y)
		}

	case *ir.Cmp:
		x, y := f.expr(e.X), f.expr(e.Y)

		if v := foldCompare(e.Op, x, y); v != nil {
			return v
		}

		if x != e.X || y != e.Y {
			return &ir.Cmp{Op: e.Op, X: x, Y: y}
		}

	case *ir.Field:
		x, pos, size := f.expr(e.X), f.expr(e.Pos), f.expr(e.Size)
		if x != e.X || pos != e.Pos || size != e.Size {
			return &ir.Field{X: x, Pos: pos, Size: size}
		}

	case *ir.CharCmp:
		x, y := f.text(e.X), f.text(e.Y)
		if x != e.X || y != e.Y {
			return &ir.CharCmp{Op: e.Op, X: x, Y: y}
		}

	case *ir.CharIndex:
		x, y, base := f.text(e.X), f.text(e.Y), f.expr(e.Base)
		if x != e.X || y != e.Y || base != e.Base {
			return &ir.CharIndex{X: x, Y: y, Base: base}
		}
	}

	return e
}

// foldCompare is a comparison's value, a LOGICAL, if both operands are constants
// that are numbers.
func foldCompare(op ir.Operator, x, y ir.Expr) *ir.Const {
	a, ok := x.(*ir.Const)
	b, ok2 := y.(*ir.Const)

	if !ok || !ok2 || a.Raw || b.Raw {
		return nil
	}

	var order int

	switch {
	case a.T.Integer():
		order = compareInts(a.Int(), b.Int())
	case a.T.Floating():
		order = a.Real().Cmp(b.Real())
	default:
		return nil
	}

	holds := false

	switch op {
	case ir.Eq:
		holds = order == 0
	case ir.Ne:
		holds = order != 0
	case ir.Lt:
		holds = order < 0
	case ir.Le:
		holds = order <= 0
	case ir.Gt:
		holds = order > 0
	case ir.Ge:
		holds = order >= 0
	}

	if holds {
		return ir.LongConst(-1)
	}

	return ir.LongConst(0)
}

// compareInts is -1, 0, or 1 as a is less than, equal to, or more than b.
func compareInts(a, b int32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}

	return 0
}

// terminator rewrites block b's terminator, and reports whether it became a jump,
// its way settled.
func (f *forwarder) terminator(b *ir.Block) bool {
	var to *ir.Block

	switch t := b.End.(type) {
	case *ir.Branch:
		t.Cond = f.expr(t.Cond)

		if c, ok := t.Cond.(*ir.Const); ok {
			to = t.False
			if c.Bits&1 != 0 {
				to = t.True
			}
		}

	case *ir.Switch:
		t.Index = f.expr(t.Index)

		if c, ok := t.Index.(*ir.Const); ok {
			to = t.Else
			if n := int(c.Int()); n >= 1 && n <= len(t.Targets) {
				to = t.Targets[n-1]
			}
		}

	case *ir.Sign:
		t.Value = f.expr(t.Value)

		if c, ok := t.Value.(*ir.Const); ok && !c.Raw {
			switch n := constSign(c); {
			case n < 0:
				to = t.Neg
			case n == 0:
				to = t.Zero
			default:
				to = t.Pos
			}
		}

	case *ir.IndirectJump:
		t.Through = f.mem(t.Through)

	case *ir.Return:
		if t.Value != nil {
			t.Value = f.expr(t.Value)
		}
	}

	if to == nil {
		return false
	}

	b.End = ir.NewJump(b.End.Source(), to)

	return true
}

// constSign is the sign of a constant number.
func constSign(c *ir.Const) int {
	if c.T.Floating() {
		return c.Real().Sign()
	}

	return compareInts(c.Int(), 0)
}
