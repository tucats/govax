package opt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ir"
)

// Common subexpressions: not computing again what a block has just computed.
//
//	      X = A(I,J)*B + 1.0
//	      Y = A(I,J)*B - 1.0
//	      Z = A(I,J)*B
//
// Nothing between these statements changes A, I, J, or B, so all three want one
// value. Computing it once into a temporary ($U1) and reading that twice more leaves
// the index arithmetic and the multiplication done once. This looks only inside a
// basic block: nothing jumps into the middle of one, so whatever is computed early in
// it has been computed by the time anything later runs. Across blocks the survey
// found no case worth the machinery (docs/PHASE-84 - FORTRAN optimizer.md).
//
// # Which uses are the same value
//
// Two expressions are the same if their trees are, with A+B the same as B+A. What
// they read has to be unchanged between them, which is places.go's and effects.go's
// question: a store that Conflict says can touch something it reads ends the value's
// run, so does a call that may change something it reads (Storage.ChangedByCall), and
// the same expression after that is a new value. Nothing that reads something VOLATILE
// is ever merged (Decision 7).
//
// Two identical divisions may become one, so a handler can see one condition where it
// would have seen two: the program asked for the same value twice (Decision 8).
//
// # Where the value is computed
//
// Just before the first statement that uses it, which was going to compute it anyway,
// so no fault is moved onto a path that had none, with one exception to look out for.
// In a branch's condition .AND. and .OR. don't always run their second operand:
//
//	      IF (N .NE. 0 .AND. M/N .GT. 2) ...
//
// M/N mustn't be computed ahead of that test when N is 0. So such an operand can use a
// value computed earlier, but its use can't be the first.
//
// # A value already somewhere
//
//	      K = J*60 + I
//	      A(J*60 + I) = 0.0
//
// needs no temporary, since the value is in K for as long as K isn't changed. The later
// use reads K, with no store added.
//
// # Finding a place once
//
//	      C(I,J) = C(I,J) + X
//
// is the commonest case of all, and needs no temporary. The VAX can do it with one
// operand, ADDF2 X,C-244[R1], where the place is read and written, so the subscript is
// worked out once rather than twice. That is what Assign.InPlace asks of the code
// generator. It is done first, so the two subscripts aren't also counted as a common
// subexpression. C's index in the survey's matrix multiplication is this.
//
// # What it's worth
//
// A temporary in memory isn't free: it takes a store, and a read per use, and if the
// value is wanted in a register, as an index is, an instruction per use as well. VAX
// indexed mode takes only a register, so A(I,J) and B(I,J), one subscript in two
// arrays, gain nothing:
//
//	      MULL3   J,#60,R0                MULL3   J,#60,R0
//	      ADDL2   I,R0                    ADDL3   I,R0,$U1
//	      ... A[R0] ...                   MOVL    $U1,R0
//	      MULL3   J,#60,R0                ... A[R0] ...
//	      ADDL2   I,R0                    MOVL    $U1,R0
//	      ... B[R0] ...                   ... B[R0] ...
//
// Four instructions and four reads on the left, four instructions, four reads, and a
// store on the right. So worth counts what each choice would run, and a temporary is
// made only if that saves an instruction and costs no more memory references than it
// saves instructions.
//
// In a loop the temporary most likely has a register (registers.go), which costs none of
// that: a use is no instruction and no read. So in a block of a loop worth counts it so.
// Outside loops a value isn't run often enough to earn a register, and the sum above
// stands.
//
// # What it leaves alone
//
// A variable format expression's blocks run in the middle of another block's I/O
// statement, and would share temporaries with it, so they get none here. A dummy's or
// descriptor's address (Mem.Ptr) stays as it is, since places.go recognizes a place by
// its form. A function's Result is stored after its call, so its index isn't this
// block's to change.

// CommonSubexpressions is the pass.
func CommonSubexpressions(r *ir.Routine) {
	c := &cse{r: r, s: NewStorage(r), spare: map[ir.Type][]*ir.Cell{}}
	expressions := expressionBlocks(r)
	g := NewGraph(r)

	c.named = highestNamed(r, "$U")

	for _, b := range r.Blocks {
		c.markInPlace(b)

		if expressions[b] {
			continue
		}

		c.inLoop = g.LoopOf(b) != nil

		var used []*ir.Cell

		for {
			best := c.best(b)
			if best == nil {
				break
			}

			if t := c.apply(b, best); t != nil {
				used = append(used, t)
			}
		}

		// A temporary holds its value only inside its block, so the next block can
		// use it again.
		for _, t := range used {
			c.spare[t.T] = append(c.spare[t.T], t)
		}
	}
}

// highestNamed is the highest number of a cell of routine r named prefix and a number.
func highestNamed(r *ir.Routine, prefix string) int {
	highest := 0

	for _, cell := range r.Cells {
		if n, err := strconv.Atoi(strings.TrimPrefix(cell.Name, prefix)); err == nil && n > highest {
			highest = n
		}
	}

	return highest
}

// cse is the pass's state for one routine.
type cse struct {
	r *ir.Routine
	s *Storage

	// spare are temporaries free for use, by type, and named how many have been
	// made.
	spare map[ir.Type][]*ir.Cell
	named int

	// inLoop says the block being worked on is in a loop.
	inLoop bool
}

// markInPlace marks the statements of block b that can change their own place where
// it is. A place with neither an index nor a computed address needs no marking: the
// code generator sees the two operands are one (arithmetic), and finds nothing twice.
func (c *cse) markInPlace(b *ir.Block) {
	for _, st := range b.Stmts {
		a, ok := st.(*ir.Assign)
		if !ok || a.InPlace {
			continue
		}

		if a.Dst.Index == nil && a.Dst.Ptr == nil && (a.Dst.Param == nil || a.Dst.Offset == 0) {
			continue
		}

		if ir.InPlaceOperand(a) != nil && !c.anyVolatile(a) {
			a.InPlace = true
		}
	}
}

// anyVolatile reports whether statement a touches something VOLATILE.
func (c *cse) anyVolatile(a *ir.Assign) bool {
	e := c.s.StmtEffects(a)

	return c.volatile(e.Reads) || c.volatile(e.Writes)
}

// volatile reports whether any of the places is VOLATILE.
func (c *cse) volatile(list []Place) bool {
	for _, p := range list {
		if c.s.Volatile(p) {
			return true
		}
	}

	return false
}

// value is one run of an expression through a block: where it's first computed, to
// its last use, with nothing it reads changed between.
type value struct {
	key   string
	e     ir.Expr
	reads []Place

	// first and last are statements' places in the block, the terminator's being
	// the number of statements.
	first, last int

	// uses is how many times it is computed; moved how many of those want it in a
	// register or moved to a destination; and instructions what those uses cost as
	// they are.
	uses, moved, instructions int

	// holder is a place that already holds it, the destination of the assignment
	// that first computes it: then uses counts only those after that.
	holder *ir.Mem
}

// worth is what using one computation of v in all its places would save: instructions,
// and memory references (a read or a write), which may be negative. See the top of the
// file.
func (c *cse) worth(v *value) (instructions, references int) {
	cost := costOf(v.e)

	def := cost.instructions
	if !computed(v.e) {
		def++
	}

	switch {
	case v.holder != nil && c.inLoop:
		return v.instructions, v.uses * cost.reads
	case v.holder != nil:
		return v.instructions - v.moved, v.uses * (cost.reads - 1)
	case c.inLoop:
		return v.instructions - def, (v.uses - 1) * cost.reads
	}

	return v.instructions - def - v.moved, (v.uses-1)*cost.reads - v.uses - 1
}

// best is the value in block b most worth a single computation, or nil. It is looked
// for again after each change, since merging an expression leaves one fewer use of
// each expression inside it.
func (c *cse) best(b *ir.Block) *value {
	var (
		best  *value
		saved int
	)

	for _, v := range c.scan(b) {
		if v.uses < 1 || (v.holder == nil && v.uses < 2) {
			continue
		}

		instructions, references := c.worth(v)
		if instructions < 1 || instructions+references < 0 {
			continue
		}

		if total := instructions + references; best == nil || total > saved {
			best, saved = v, total
		}
	}

	return best
}

// scanner walks a block to find its values.
type scanner struct {
	c    *cse
	at   int
	open map[string]*value
	all  []*value
}

// visit notes e, if it could be computed once, and goes on inside it.
func (w *scanner) visit(e ir.Expr, moved, maybe bool) bool {
	if w.c.candidate(e) {
		w.use(e, moved, maybe)
	}

	return true
}

// scan is every run of an expression through block b.
func (c *cse) scan(b *ir.Block) []*value {
	w := &scanner{c: c, open: map[string]*value{}}
	walk := walker{visit: w.visit}

	for i, st := range b.Stmts {
		w.at = i
		walk.stmt(st)
		w.after(st)
	}

	w.at = len(b.Stmts)
	walk.terminator(b.End)

	return w.all
}

// use notes one computation of e, in the value whose run it's in, or as a new value's
// first.
func (w *scanner) use(e ir.Expr, moved, maybe bool) {
	key := exprKey(e)

	v := w.open[key]
	if v == nil {
		if maybe {
			return
		}

		v = &value{key: key, e: e, reads: w.c.readsOf(e), first: w.at}
		w.open[key] = v
		w.all = append(w.all, v)
	}

	v.uses++
	v.last = w.at
	v.instructions += costOf(e).instructions

	if moved {
		v.moved++

		if !computed(e) {
			v.instructions++
		}
	}
}

// after ends the runs of the values st changes, and notes what its destination now
// holds.
func (w *scanner) after(st ir.Stmt) {
	e := w.c.s.StmtEffects(st)

	for key, v := range w.open {
		if w.c.ended(v, e) {
			delete(w.open, key)
		}
	}

	a, ok := st.(*ir.Assign)
	if !ok || a.InPlace || !w.c.candidate(a.Src) {
		return
	}

	v := w.open[exprKey(a.Src)]
	if v == nil || v.first != w.at || v.uses != 1 || !w.c.canHold(a.Dst) {
		return
	}

	v.holder = a.Dst
	v.uses, v.moved, v.instructions = 0, 0, 0
}

// ended reports whether what a statement does can change v, or the place holding it.
func (c *cse) ended(v *value, e Effects) bool {
	list := v.reads
	if v.holder != nil {
		list = append(list[:len(list):len(list)], c.s.Place(v.holder))
	}

	for _, p := range list {
		if overlapsAny(p, e.Writes) || (e.Call && c.s.ChangedByCall(p)) {
			return true
		}
	}

	return false
}

// canHold reports whether a value left in m can be read from there instead: a whole
// variable or temporary, not VOLATILE, reached by no index or address.
func (c *cse) canHold(m *ir.Mem) bool {
	return m.Cell != nil && m.Index == nil && m.Ptr == nil && !c.s.Volatile(c.s.Place(m))
}

// candidate reports whether e is an expression that could be computed once: an
// operation, or an element of an array or through a dummy, of a type a temporary can
// hold, and reading nothing VOLATILE. A plain variable isn't, as reading a temporary
// instead saves nothing; neither is a comparison, which a test does far better as a
// branch than as a value.
func (c *cse) candidate(e ir.Expr) bool {
	switch e := e.(type) {
	case *ir.Load:
		if e.Mem.Index == nil && e.Mem.Ptr == nil && e.Mem.Param == nil {
			return false
		}

	case *ir.Op, *ir.Field, *ir.InRange:

	default:
		return false
	}

	switch e.Type() {
	case ir.Long, ir.Word, ir.Byte, ir.Float, ir.Double, ir.GFloat, ir.HFloat:
	default:
		return false
	}

	return !hasCallValue(e) && !c.volatile(c.readsOf(e))
}

// hasCallValue reports whether e reads what a call just left in R0, which is gone by
// the time anything else is computed.
func hasCallValue(e ir.Expr) bool {
	switch e := e.(type) {
	case *ir.CallValue:
		return true
	case *ir.Op:
		return hasCallValue(e.X) || (e.Y != nil && hasCallValue(e.Y))
	case *ir.Cmp:
		return hasCallValue(e.X) || hasCallValue(e.Y)
	case *ir.Field:
		return hasCallValue(e.X) || hasCallValue(e.Pos) || hasCallValue(e.Size)
	case *ir.InRange:
		return hasCallValue(e.X) || hasCallValue(e.Low) || hasCallValue(e.High)
	}

	return false
}

// readsOf is every place computing e reads.
func (c *cse) readsOf(e ir.Expr) []Place {
	w := collector{s: c.s}
	w.expr(e)

	return w.e.Reads
}

// apply makes v computed once in block b, and returns the temporary it made, if any.
func (c *cse) apply(b *ir.Block, v *value) *ir.Cell {
	r := &replacer{key: v.key}

	if v.holder != nil {
		r.with = &ir.Load{Mem: v.holder}
		r.rewrite(b, v.first+1, v.last)

		return nil
	}

	t := c.temporary(v.e.Type())
	r.with = &ir.Load{Mem: &ir.Mem{Cell: t, T: t.T}}
	r.rewrite(b, v.first, v.last)

	from := b.End.Source()
	if v.first < len(b.Stmts) {
		from = b.Stmts[v.first].Source()
	}

	b.Stmts = append(b.Stmts, nil)
	copy(b.Stmts[v.first+1:], b.Stmts[v.first:])
	b.Stmts[v.first] = ir.NewAssign(from, &ir.Mem{Cell: t, T: t.T}, v.e)

	return t
}

// temporary is a temporary of type t: one a block before this one used, or a new one,
// placed as lowering places its cells.
func (c *cse) temporary(t ir.Type) *ir.Cell {
	if free := c.spare[t]; len(free) > 0 {
		cell := free[len(free)-1]
		c.spare[t] = free[:len(free)-1]

		return cell
	}

	c.named++

	return newTemporary(c.r, "$U"+strconv.Itoa(c.named), t)
}

// newTemporary is a new temporary of type t, placed as lowering places its cells: in
// $LOCAL, or in a reentrant routine's frame.
func newTemporary(r *ir.Routine, name string, t ir.Type) *ir.Cell {
	return r.NewCell(name, ir.TempCell, t, t.Size())
}

// replacer puts with in place of every expression whose key is key. Lowering may use
// one Mem or tree in several places, so what changes is rebuilt, not changed.
//
// place, if not nil, is asked instead about every place, a call's Result and Through
// among them, and each address, once what's inside it has been seen to: it returns the
// place to use, or nil to leave it.
type replacer struct {
	key   string
	with  ir.Expr
	place func(*ir.Mem) *ir.Mem
}

// rewrite replaces in the statements from first to last, the terminator being the
// number after them.
func (r *replacer) rewrite(b *ir.Block, first, last int) {
	for i := first; i <= last; i++ {
		if i == len(b.Stmts) {
			r.terminator(b.End)

			return
		}

		switch st := b.Stmts[i].(type) {
		case *ir.Assign:
			if st.InPlace {
				r.inPlace(st)
			} else {
				st.Dst, st.Src = r.mem(st.Dst), r.expr(st.Src)
			}

		case *ir.CharMove:
			st.Dst, st.Src = r.text(st.Dst), r.text(st.Src)

		case *ir.Call:
			for j, a := range st.Args {
				switch {
				case a.Ref != nil:
					st.Args[j].Ref = r.mem(a.Ref)
				case a.Value != nil:
					st.Args[j].Value = r.expr(a.Value)
				}
			}

			if r.place != nil {
				if st.Through != nil {
					st.Through = r.mem(st.Through)
				}

				if st.Result != nil {
					st.Result = r.mem(st.Result)
				}
			}
		}
	}
}

// inPlace replaces in an Assign marked InPlace, whose value must go on naming its
// destination, the same place after the replacement as before.
func (r *replacer) inPlace(a *ir.Assign) {
	op := a.Src.(*ir.Op)
	other := ir.InPlaceOperand(a)

	a.Dst = r.mem(a.Dst)
	own, replaced := &ir.Load{Mem: a.Dst}, r.expr(other)

	if other == op.Y {
		a.Src = &ir.Op{Op: op.Op, T: op.T, X: own, Y: replaced}
	} else {
		a.Src = &ir.Op{Op: op.Op, T: op.T, X: replaced, Y: own}
	}
}

// terminator replaces in what t computes.
func (r *replacer) terminator(t ir.Terminator) {
	switch t := t.(type) {
	case *ir.Branch:
		t.Cond = r.expr(t.Cond)
	case *ir.Switch:
		t.Index = r.expr(t.Index)
	case *ir.Sign:
		t.Value = r.expr(t.Value)
	case *ir.IndirectJump:
		t.Through = r.mem(t.Through)
	case *ir.Return:
		if t.Value != nil {
			t.Value = r.expr(t.Value)
		}
	}
}

// expr is e with the replacement made; e itself if it contains nothing to replace.
func (r *replacer) expr(e ir.Expr) ir.Expr {
	switch e.(type) {
	case *ir.Load, *ir.Op, *ir.Field, *ir.InRange:
		if r.place == nil && exprKey(e) == r.key {
			return r.with
		}
	}

	switch e := e.(type) {
	case *ir.Load:
		if m := r.mem(e.Mem); m != e.Mem {
			return &ir.Load{Mem: m}
		}

	case *ir.Addr:
		if m := r.mem(e.Mem); m != e.Mem {
			return &ir.Addr{Mem: m}
		}

	case *ir.Op:
		x := r.expr(e.X)

		var y ir.Expr
		if e.Y != nil {
			y = r.expr(e.Y)
		}

		if x != e.X || y != e.Y {
			return &ir.Op{Op: e.Op, T: e.T, X: x, Y: y}
		}

	case *ir.Cmp:
		x, y := r.expr(e.X), r.expr(e.Y)
		if x != e.X || y != e.Y {
			return &ir.Cmp{Op: e.Op, X: x, Y: y}
		}

	case *ir.Field:
		x, pos, size := r.expr(e.X), r.expr(e.Pos), r.expr(e.Size)
		if x != e.X || pos != e.Pos || size != e.Size {
			return &ir.Field{X: x, Pos: pos, Size: size}
		}

	case *ir.InRange:
		x, low, high := r.expr(e.X), r.expr(e.Low), r.expr(e.High)
		if x != e.X || low != e.Low || high != e.High {
			return &ir.InRange{X: x, Low: low, High: high}
		}

	case *ir.CharCmp:
		x, y := r.text(e.X), r.text(e.Y)
		if x != e.X || y != e.Y {
			return &ir.CharCmp{Op: e.Op, X: x, Y: y}
		}

	case *ir.CharIndex:
		x, y := r.text(e.X), r.text(e.Y)
		base := e.Base
		if r.place != nil {
			base = r.expr(base)
		}

		if x != e.X || y != e.Y || base != e.Base {
			return &ir.CharIndex{X: x, Y: y, Base: base}
		}
	}

	return e
}

// mem is m with the replacement made in its index, and inside its address, whose own
// form stays.
func (r *replacer) mem(m *ir.Mem) *ir.Mem {
	n := *m
	changed := false

	if m.Index != nil {
		if n.Index = r.expr(m.Index); n.Index != m.Index {
			changed = true
		}
	}

	switch p := m.Ptr.(type) {
	case nil:
	case *ir.Addr:
		if inner := r.mem(p.Mem); inner != p.Mem {
			n.Ptr, changed = &ir.Addr{Mem: inner}, true
		}
	case *ir.Load:
		if inner := r.mem(p.Mem); inner != p.Mem {
			n.Ptr, changed = &ir.Load{Mem: inner}, true
		}
	default:
		if r.place != nil {
			if n.Ptr = r.expr(p); n.Ptr != m.Ptr {
				changed = true
			}
		}
	}

	if r.place != nil {
		if changed {
			m = &n
		}

		if to := r.place(m); to != nil {
			return to
		}

		return m
	}

	if !changed {
		return m
	}

	return &n
}

// text is t with the replacement made.
func (r *replacer) text(t ir.Text) ir.Text {
	return ir.Text{Mem: r.mem(t.Mem), Len: r.expr(t.Len)}
}

// exprKey is a text that two expressions share only if they are the same value: their
// trees, with the operands of an operation that commutes in one order. It is the
// dump's text, but for the type of a place, which the dump leaves out.
func exprKey(e ir.Expr) string {
	switch e := e.(type) {
	case *ir.Const:
		return fmt.Sprintf("#%d:%x:%x:%d", e.T, e.Bits, e.High, e.Part)

	case *ir.Load:
		return memKey(e.Mem)

	case *ir.Addr:
		return "&" + memKey(e.Mem)

	case *ir.ArgPointer:
		return "arg:" + e.Param.Name

	case *ir.Op:
		x := exprKey(e.X)
		if e.Y == nil {
			return fmt.Sprintf("%s.%d.%d(%s)", e.Op, e.T, e.X.Type(), x)
		}

		y := exprKey(e.Y)

		switch e.Op {
		case ir.Add, ir.Mul, ir.And, ir.Or, ir.Xor:
			if y < x {
				x, y = y, x
			}
		}

		return fmt.Sprintf("%s.%d(%s,%s)", e.Op, e.T, x, y)

	case *ir.Cmp:
		x, y := exprKey(e.X), exprKey(e.Y)

		if (e.Op == ir.Eq || e.Op == ir.Ne) && y < x {
			x, y = y, x
		}

		return fmt.Sprintf("%s(%s,%s)", e.Op, x, y)

	case *ir.Field:
		return "field(" + exprKey(e.X) + "," + exprKey(e.Pos) + "," + exprKey(e.Size) + ")"

	case *ir.InRange:
		return "inrange(" + exprKey(e.X) + "," + exprKey(e.Low) + "," + exprKey(e.High) + ")"
	}

	// The rest are never part of a candidate's value but as themselves.
	return ir.ExprText(e)
}

// memKey is exprKey's text for a place.
func memKey(m *ir.Mem) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%d:", m.T)

	switch {
	case m.Cell != nil:
		b.WriteString(m.Cell.Name)
	case m.Datum != nil:
		b.WriteString(m.Datum.Name)
	case m.Param != nil:
		b.WriteString("@" + m.Param.Name)
	case m.Ptr != nil:
		b.WriteString("@(" + exprKey(m.Ptr) + ")")
	}

	fmt.Fprintf(&b, "%+d", m.Offset)

	if m.Index != nil {
		b.WriteString("[" + exprKey(m.Index) + "]")
	}

	return b.String()
}

// computed reports whether e's value is worked out into a register or straight to a
// destination, by an operation, rather than read where it is.
func computed(e ir.Expr) bool {
	switch e.(type) {
	case *ir.Op, *ir.Field, *ir.InRange, *ir.Cmp, *ir.Addr, *ir.LabelAddr, *ir.RoutineAddr:
		return true
	}

	return false
}

// price is what computing an expression runs: instructions, and memory reads.
type price struct {
	instructions, reads int
}

// add is the two prices together.
func (p price) add(q price) price {
	return price{p.instructions + q.instructions, p.reads + q.reads}
}

// costOf is what the code generator writes for e, as an operand of an instruction
// that is counted elsewhere: a place in memory takes no instruction but its index's,
// and an operation one of its own.
func costOf(e ir.Expr) price {
	switch e := e.(type) {
	case *ir.Load:
		return memCost(e.Mem).add(price{0, 1})

	case *ir.Addr:
		return memCost(e.Mem).add(price{1, 0})

	case *ir.ArgPointer:
		return price{0, 1}

	case *ir.Op:
		p := costOf(e.X).add(price{1, 0})
		if e.Y != nil {
			p = p.add(costOf(e.Y))
		}

		return p

	case *ir.Cmp:
		// CLRL, the CMP, a branch, and MNEGL.
		return costOf(e.X).add(costOf(e.Y)).add(price{4, 0})

	case *ir.Field:
		return costOf(e.X).add(costOf(e.Pos)).add(costOf(e.Size)).add(price{1, 0})

	case *ir.InRange:
		return costOf(e.X).add(costOf(e.Low)).add(costOf(e.High)).add(price{1, 0})

	case *ir.LabelAddr, *ir.RoutineAddr:
		return price{1, 0}
	}

	return price{}
}

// memCost is what finding m takes: its index in a register, the dummy's address read
// from the argument list, the address it's at.
func memCost(m *ir.Mem) price {
	var p price

	switch {
	case m.Index != nil:
		p = costOf(m.Index)
		if !computed(m.Index) {
			p.instructions++
		}

		// A dummy's offset is added to the index (codegen's index).
		if m.Param != nil && m.Offset != 0 {
			p.instructions++
		}

	case m.Param != nil && m.Offset != 0:
		// MOVL #k,R1 and @A(AP)[R1].
		p.instructions++
	}

	if m.Param != nil {
		p.reads++
	}

	if m.Ptr != nil {
		p = p.add(costOf(m.Ptr))
		if (m.Offset != 0 || m.Index != nil) && !m.Deferred {
			// The address in a register, not reached through where it is.
			p.instructions++
		}
	}

	return p
}
