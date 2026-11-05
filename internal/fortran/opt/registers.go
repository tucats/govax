package opt

import (
	"sort"
	"strconv"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/ir"
)

// Register allocation: keeping in a register what a loop uses most.
//
//	      DO 30 I = 1, N
//	         C(I,J) = C(I,J) + A(I,K)*B(K,J)
//	   30 CONTINUE
//
// Every operand here is a memory reference, five bytes of instruction and a trip to
// memory. The VAX has twelve general registers. R0 to R5 go to expressions, within one
// statement, and MOVC and CMPC change them. R6 to R11 are left, which only a routine that
// saves them in its entry mask may use, and every routine does (Decisions 2 and 3 of
// docs/PHASE-84 - FORTRAN optimizer.md). So a value in R6 survives a call, and this pass
// has six registers, sometimes fewer, to give away.
//
// # What gets a register
//
// A pass can't give anything a register without knowing what would be there otherwise, so
// this one lists every candidate first, with what it would gain, and rewrites only the
// winners. The candidates are of five kinds.
//
// The compiler's own temporaries, counts, and steps ($H1, $U1, $C5). They have no home to
// keep up, as nothing reads them after they die (Decision 6), so the cell simply becomes a
// register for the whole routine. Liveness (deadcode.go) says which two can share one.
// A cell whose value lasts from one call to the next can't be in a register: that is any
// live where the routine starts (see deadcode.go's "A place live on entry").
//
// A variable, for the length of a loop. Its home in memory stays what the rest of the
// routine uses, so that VAX FORTRAN's promise that a variable keeps its value holds
// (Decision 6). A new cell, $R1, gets its value in the block before the loop, and if the
// loop changes it, gives it back on each way out:
//
//	      MOVL    S.I,R8             ; before the loop
//	      ... R8 ...                 ; every use in the loop
//	      MOVL    R8,S.I             ; on each way out
//
// Those two moves are of the bits: MOVF would fault on a reserved operand where the program
// didn't touch the variable. It has to be sure nothing else can reach the variable during
// the loop. A local nothing has the address of (Decision 7) qualifies whatever the loop
// does, since a call can't see it. A local whose address has been let out, or a variable
// in COMMON, or a dummy argument, qualifies only in a loop with no call in it. Nothing
// else in the loop may overlap it. Nothing VOLATILE qualifies.
//
// A dummy argument's address. @A(AP)[R1] reads the argument list on every use; with the
// address in R6 that is (R6)[R1], and with a constant subscript 4(R6), which needs no
// index at all. The argument list doesn't change, so this can be done in any loop. A dummy
// that can't have its value in a register for a loop can often have this.
//
// An address that doesn't change: an adjustable array's A0 and a CHARACTER dummy's text
// pointer, which otherwise take a read per use, and the base of an array in
//
//	      C(I,J), with J*60 not changing:     ADDL3 $H1,I,R0  and  C-244[R0]
//
// which with C-244+J*60*4 in R6 is (R6)[R8], one instruction fewer on every trip.
//
// An index that goes up by a constant each trip, computed by a multiplication: induction.go.
//
// A dummy argument's value or text pointer has to be read before the loop, so the loop
// must have been sure to read it on its first trip: an optional argument not passed
// has no address (the same rule as invariant.go's). An output argument the loop stores in
// only some of the time isn't given a register, since writing back on the way out would
// store in what may be a constant.
//
// # Choosing
//
// A candidate's gain is roughly its uses, each counted as ten times more for each loop it
// is in, less what its moves in and out cost. An index counts twice, since it also saves
// the MOVL into R0. In order of gain each takes the first registers that nothing it can be
// live with already has. What's left stays in memory: that is why a pass before this one
// needn't know how it came out (invariant.go, cse.go).
//
// Using a register at all costs the routine a push and a pop per call, in its entry mask,
// so a candidate must gain more than that.
//
// # Registers the expressions need
//
// A few expressions are large enough for the code generator to go past R5, H_floating ones
// above all. Rather than make them fail, the allocator asks which registers the routine's
// expressions use when nothing is in the way (codegen.ExpressionRegisters), and doesn't
// touch those: a routine like that gets fewer register cells (Decision 3).
//
// # What it leaves alone
//
// Anything a variable format expression touches, since its code runs under FTNRTL, whose
// own registers are in R6 to R11 then. A loop with an assigned GO TO in it, whose exits
// can't take a block. A loop whose header can't have a block put before it (invariant.go's
// canPrecede). Straight-line code: a value outside any loop doesn't earn a register unless
// it is the compiler's own and used often.
//
// # What a program can see
//
// A handler that reads a COMMON variable after a fault in a loop with no call reads what the
// loop last wrote back, and a routine unwound from out of a call in a loop leaves its local
// variables as they were when the loop began. The manual gives a handler no such promise:
// VOLATILE is how to say a variable must be right at every moment (Decision 9).
//
// Registers runs after the IR's other passes that look at statements, since they recognize
// places by their forms. Only Rotate (rotate.go), which only moves blocks, comes after it.

// Registers is the pass.
func Registers(r *ir.Routine) {
	if len(r.Blocks) == 0 {
		return
	}

	// Nothing to do a second time.
	for _, c := range r.Cells {
		if c.Register != 0 {
			return
		}
	}

	a := newAllocator(r)
	a.findTemporaries()
	a.findStepped()
	a.findBases()
	a.findPointers()
	a.findVariables()
	a.findAddresses()

	if a.colour(codegen.ExpressionRegisters(r)) {
		a.apply()
		a.closeUpRegisters()
	}
}

// The registers the allocator gives away.
const (
	firstCellRegister = 6
	lastCellRegister  = 11
)

// enoughGain is the least a candidate must gain: more than the entry mask's push and pop.
const enoughGain = 3

// leavingLoop is the note on a block that gives a variable back on the way out of a loop.
const leavingLoop = "leaving the loop"

// reference is one use of a place.
type reference struct {
	m     *ir.Mem
	role  memRole
	block *ir.Block

	// maybe says it's in the part of a condition that doesn't always run, and afterCall
	// that a call comes before it in its block.
	maybe, afterCall bool
}

// candidateKind is what a candidate is.
type candidateKind int

// The kinds of candidate, in the order they are rewritten, each kind's deepest loops
// first: a base's place must still look as it did when it was found, and so must a
// pointer's, and what the outer loops' rewrites leave alone isn't in their way.
const (
	steppedCandidate candidateKind = iota // an index that goes up with a loop's variable (induction.go)
	baseCandidate                         // an array's address and an index that doesn't change
	pointerCandidate                      // an address in memory that doesn't change
	otherCandidate                        // a variable's value, or a dummy's address
	cellCandidate                         // a compiler's cell, as it is
)

// candidate is something that might have a register.
type candidate struct {
	kind candidateKind

	// gain is what a register would save, and t the type of what it holds.
	gain int
	t    ir.Type

	// cell is a compiler's cell, with blocks where it holds a value and index its place
	// in the table of which of them can share.
	cell   *ir.Cell
	blocks map[*ir.Block]bool
	index  int

	// loop is the loop anything else has the register for, by its header.
	loop *Loop

	// source is what goes in the register before the loop, holds and points what the
	// new cell says, and rewrite the new form of a place in the loop or nil if it isn't
	// one of this candidate's.
	source ir.Expr
	holds  string
	points *ir.Mem
	place  func(c *ir.Cell, m *ir.Mem) *ir.Mem

	// home is a variable's, which it is copied from and, if written, back to.
	home    *ir.Mem
	written bool

	// key is a stepped value's, the expression it takes the place of, and steps what keeps it
	// up to date.
	key   string
	steps []valueStep

	register int
}

// allocator is the pass's state for one routine.
type allocator struct {
	r *ir.Routine
	s *Storage
	g *Graph

	refs []reference

	// expressions are the blocks of variable format expressions.
	expressions map[*ir.Block]bool

	// writes are what each loop's statements write and whether any calls, and touched
	// everything each one reads, writes, or lets out.
	writes  map[*Loop][]Place
	calls   map[*Loop]bool
	touched map[*Loop][]Place

	candidates []*candidate
	claimed    map[*ir.Mem]bool

	// together says which of the compiler's cells can't share a register.
	together [][]bool

	// valued are the loops where a dummy's value may be in a register, so its address
	// needn't be.
	valued map[*ir.Param][]*Loop

	// stepped are the indexes, by expression and loop, that a stepped candidate may
	// replace, so that they aren't split for a base as well.
	stepped map[string]bool

	mover  *mover
	copies int
	exits  int
	exitOf map[*ir.Block]bool
}

// newAllocator looks over routine r.
func newAllocator(r *ir.Routine) *allocator {
	a := &allocator{
		r: r, s: NewStorage(r), g: NewGraph(r),
		expressions: expressionBlocks(r),
		writes:      map[*Loop][]Place{},
		calls:       map[*Loop]bool{},
		touched:     map[*Loop][]Place{},
		claimed:     map[*ir.Mem]bool{},
		valued:      map[*ir.Param][]*Loop{},
		stepped:     map[string]bool{},
		exitOf:      map[*ir.Block]bool{},
	}

	a.mover = newMover(r, a.s)
	a.copies = highestNamed(r, "$R")

	for _, b := range a.g.Order {
		if !a.expressions[b] {
			a.scan(b)
		}
	}

	for _, l := range a.g.Loops {
		for _, b := range l.Blocks {
			e := a.s.BlockEffects(b)
			a.writes[l] = append(a.writes[l], e.Writes...)
			a.touched[l] = append(append(append(a.touched[l], e.Reads...), e.Writes...), e.Addressed...)
			a.calls[l] = a.calls[l] || e.Call
		}
	}

	return a
}

// scan notes every reference in block b.
func (a *allocator) scan(b *ir.Block) {
	afterCall := false

	w := memWalker{visit: func(m *ir.Mem, role memRole, maybe bool) {
		a.refs = append(a.refs, reference{m, role, b, maybe, afterCall})
	}}

	for _, st := range b.Stmts {
		w.stmt(st)

		if _, calls := st.(*ir.Call); calls {
			afterCall = true
		}
	}

	w.terminator(b.End)
}

// weight is what one use in block b counts for: ten times more for each loop it's in.
func (a *allocator) weight(b *ir.Block) int {
	if l := a.g.LoopOf(b); l != nil {
		return loopWeight(l.Depth)
	}

	return 1
}

// loopWeight is the weight of a use depth loops down.
func loopWeight(depth int) int {
	w := 1

	for range min(depth, 4) {
		w *= 10
	}

	return w
}

// roleGain is what a register saves on one use: the reference, and for an index or an
// address the MOVL that gets it into an expression's register too.
func roleGain(role memRole) int {
	if role == roleIndex || role == rolePointer {
		return 2
	}

	return 1
}

// exact reports whether m is the whole of a value of type t, as a register can be.
func exact(m *ir.Mem, t ir.Type) bool {
	return m.Offset == 0 && m.Index == nil && m.Ptr == nil && m.T == t
}

// findTemporaries lists the compiler's cells that can be registers, by liveness.
func (a *allocator) findTemporaries() {
	d := newLiveness(a.r)
	d.solve()

	uses := map[*ir.Cell][]reference{}

	for _, ref := range a.refs {
		if ref.m.Cell != nil {
			uses[ref.m.Cell] = append(uses[ref.m.Cell], ref)
		}
	}

	byIndex := make([]*candidate, len(d.places))

	for i, c := range d.s.dying {
		// A function's names with ENTRY statements share one place (a store to one
		// is a store to the others), which no register can be.
		if d.exit[i] || c.Size != c.T.Size() || a.overlapsAnother(c) {
			continue
		}

		gain, whole := 0, true

		for _, ref := range uses[c] {
			gain += a.weight(ref.block) * roleGain(ref.role)

			if !exact(ref.m, c.T) || ref.role == roleAddress || ref.role == roleText {
				whole = false
			}
		}

		if !whole || gain < enoughGain {
			continue
		}

		cand := &candidate{kind: cellCandidate, gain: gain, t: c.T, cell: c, index: i, blocks: map[*ir.Block]bool{}}
		byIndex[i] = cand

		for _, ref := range uses[c] {
			cand.blocks[ref.block] = true
		}

		a.candidates = append(a.candidates, cand)
	}

	a.together = make([][]bool, len(d.places))
	for i := range a.together {
		a.together[i] = make([]bool, len(d.places))
	}

	for _, b := range d.g.Order {
		a.liveTogether(d, b, byIndex)
	}
}

// liveTogether notes, going back through block b, which cells are live at once, and in
// which blocks each is live: a store to a cell is what must not touch another live then.
func (a *allocator) liveTogether(d *liveness, b *ir.Block, byIndex []*candidate) {
	live := d.atEnd(b)

	note := func() {
		for i, on := range live {
			if on && byIndex[i] != nil {
				byIndex[i].blocks[b] = true
			}
		}
	}

	note()
	d.through(d.s.TermEffects(b.End), live)
	note()

	for j := len(b.Stmts) - 1; j >= 0; j-- {
		e := d.s.StmtEffects(b.Stmts[j])

		for _, p := range e.Writes {
			for i, q := range d.places {
				if !Conflict(p, q) {
					continue
				}

				for k, on := range live {
					if on && k != i {
						a.together[i][k], a.together[k][i] = true, true
					}
				}
			}
		}

		d.through(e, live)
		note()
	}
}

// usable reports whether a block can be put before loop l, and no variable format
// expression is involved.
func (a *allocator) usable(l *Loop) bool {
	return !a.expressions[l.Header] && canPrecede(a.g, l)
}

// unchanged reports whether nothing in loop l changes what e reads.
func (a *allocator) unchanged(l *Loop, e ir.Expr) bool {
	return a.mover.unchanged(e, a.writes[l], a.calls[l])
}

// surelyReached reports whether the first trip of loop l surely makes a reference.
func (a *allocator) surelyReached(l *Loop, ref reference) bool {
	return !ref.maybe && !ref.afterCall && surely(a.g, l, ref.block)
}

// findBases lists the array bases worth keeping: a place whose index is the sum of what
// the loop doesn't change and what it does. The base goes as far out as it doesn't change.
func (a *allocator) findBases() {
	found := map[string]*candidate{}

	for _, ref := range a.refs {
		inner := a.g.LoopOf(ref.block)
		if inner == nil {
			continue
		}

		if ref.m.Index != nil && a.stepped[steppedKey(ref.m.Index, inner)] {
			continue
		}

		base := a.splitIndex(inner, ref.m)
		if base == nil || !a.usable(inner) {
			continue
		}

		l := inner
		for l.Parent != nil && a.usable(l.Parent) && a.unchanged(l.Parent, base.Index) && a.addressFixed(l.Parent, base) {
			l = l.Parent
		}

		key := memKey(&ir.Mem{Cell: base.Cell, Param: base.Param, Ptr: base.Ptr, Offset: base.Offset, T: base.T}) +
			"|" + exprKey(base.Index) + "|" + l.Header.Name

		c := found[key]
		if c == nil {
			index := base.Index
			c = &candidate{
				kind: baseCandidate, t: ir.Long, loop: l,
				source: &ir.Addr{Mem: base},
				holds:  "part of the address of " + placeName(base),
				points: &ir.Mem{Cell: base.Cell, Param: base.Param, Ptr: base.Ptr, T: base.T},
				gain:   -loopWeight(l.Depth - 1),
				place: func(cell *ir.Cell, m *ir.Mem) *ir.Mem {
					rest := sameBase(m, base, index)
					if rest == nil {
						return nil
					}

					return &ir.Mem{Ptr: &ir.Load{Mem: &ir.Mem{Cell: cell, T: ir.Long}}, Index: rest, T: m.T}
				},
			}
			found[key] = c
			a.candidates = append(a.candidates, c)
		}

		c.gain += 2 * a.weight(ref.block)
		a.claimed[ref.m] = true
	}
}

// placeName names what a place is in, for the storage map.
func placeName(m *ir.Mem) string {
	switch {
	case m.Cell != nil:
		return m.Cell.Name
	case m.Param != nil:
		return m.Param.Name
	}

	return "an array"
}

// splitIndex is m as a place with only the part of its index that loop l doesn't change,
// if its index is a sum of that and a part it does change: what a register can hold the
// address of.
func (a *allocator) splitIndex(l *Loop, m *ir.Mem) *ir.Mem {
	sum, ok := m.Index.(*ir.Op)
	if !ok || sum.Op != ir.Add || sum.T != ir.Long || m.Volatile || m.Datum != nil || m.T == ir.None {
		return nil
	}

	if m.Cell != nil && m.Cell.Sym != nil && m.Cell.Sym.Volatile {
		return nil
	}

	if m.Param != nil && m.Param.Sym != nil && m.Param.Sym.Volatile {
		return nil
	}

	for _, pair := range [][2]ir.Expr{{sum.X, sum.Y}, {sum.Y, sum.X}} {
		fixed, moving := pair[0], pair[1]

		if !a.fixedPart(l, fixed) || a.unchanged(l, moving) {
			continue
		}

		base := *m
		base.Index = fixed

		if a.addressFixed(l, &base) {
			return &base
		}
	}

	return nil
}

// addressFixed reports whether what m's address is reached through, if anything, is an
// adjustable array's A0 that loop l doesn't change. A CHARACTER dummy's text pointer isn't
// read ahead, as the argument may not be there.
func (a *allocator) addressFixed(l *Loop, m *ir.Mem) bool {
	if m.Ptr == nil {
		return true
	}

	load, ok := m.Ptr.(*ir.Load)

	return ok && load.Mem.Cell != nil && fixedPointer(load.Mem) && a.unchanged(l, load)
}

// fixedPart reports whether e is a value the loop doesn't change, that can be read before
// it.
func (a *allocator) fixedPart(l *Loop, e ir.Expr) bool {
	switch e.(type) {
	case *ir.Load, *ir.Op:
	default:
		return false
	}

	c := a.mover.c

	return !hasCallValue(e) && !unsafeToMove(e, c.r.Checks.Overflow) && !c.volatile(c.readsOf(e)) && a.unchanged(l, e)
}

// sameBase is the part of m's index other than fixed, if m is base's place with an index
// that is a sum of fixed and something else; nil otherwise.
func sameBase(m, base *ir.Mem, fixed ir.Expr) ir.Expr {
	sum, ok := m.Index.(*ir.Op)
	if !ok || sum.Op != ir.Add || m.Volatile || m.T != base.T || m.Offset != base.Offset || m.Cell != base.Cell || m.Param != base.Param {
		return nil
	}

	if (m.Ptr == nil) != (base.Ptr == nil) || (m.Ptr != nil && exprKey(m.Ptr) != exprKey(base.Ptr)) {
		return nil
	}

	key := exprKey(fixed)

	switch {
	case exprKey(sum.X) == key:
		return sum.Y
	case exprKey(sum.Y) == key:
		return sum.X
	}

	return nil
}

// findPointers lists the addresses in memory that a loop doesn't change: an adjustable
// array's A0, a CHARACTER dummy's text pointer.
func (a *allocator) findPointers() {
	groups := map[string][]reference{}

	var order []string

	for _, ref := range a.refs {
		load, ok := ref.m.Ptr.(*ir.Load)
		if !ok || a.claimed[ref.m] || !fixedPointer(load.Mem) {
			continue
		}

		key := exprKey(load)
		if groups[key] == nil {
			order = append(order, key)
		}

		groups[key] = append(groups[key], ref)
	}

	for _, key := range order {
		refs := groups[key]
		source := refs[0].m.Ptr.(*ir.Load)
		safe := source.Mem.Ptr == nil

		eligible := func(l *Loop) bool {
			if !a.usable(l) || !a.unchanged(l, source) {
				return false
			}

			if safe {
				return true
			}

			for _, ref := range refsIn(refs, l) {
				if a.surelyReached(l, ref) {
					return true
				}
			}

			return false
		}

		a.placeInLoops(refs, eligible, func(l *Loop, in []reference) {
			c := &candidate{
				kind: pointerCandidate, t: ir.Long, loop: l, source: source,
				holds:  "an address in " + placeName(source.Mem),
				points: pointedAt(source.Mem, a.s),
				gain:   -loopWeight(l.Depth - 1),
				place: func(cell *ir.Cell, m *ir.Mem) *ir.Mem {
					if p, ok := m.Ptr.(*ir.Load); !ok || exprKey(p) != key {
						return nil
					}

					n := *m
					n.Ptr, n.Deferred = &ir.Load{Mem: &ir.Mem{Cell: cell, T: ir.Long}}, false

					return &n
				},
			}

			for _, ref := range in {
				c.gain += a.weight(ref.block)
			}

			a.candidates = append(a.candidates, c)
		})
	}
}

// fixedPointer reports whether a place is one of those holding an address that nothing
// but its routine's entry changes: a descriptor's, its own or a CHARACTER dummy's.
func fixedPointer(m *ir.Mem) bool {
	if m.Index != nil {
		return false
	}

	if m.Cell != nil {
		return m.Cell.Kind == ir.DescriptorCell && m.Cell.Dummy != "" && m.Offset == ir.A0Offset
	}

	return isDescriptorPointer(m)
}

// pointedAt is what an address in a place points at, as a place's analysis will want to
// know.
func pointedAt(m *ir.Mem, s *Storage) *ir.Mem {
	if m.Cell != nil {
		if p := s.paramNamed(m.Cell.Dummy); p != nil {
			return &ir.Mem{Param: p, T: ir.Byte}
		}

		return nil
	}

	return &ir.Mem{Param: m.Ptr.(*ir.ArgPointer).Param, T: ir.Byte}
}

// refsIn are the references in loop l.
func refsIn(refs []reference, l *Loop) []reference {
	var in []reference

	for _, ref := range refs {
		if l.Contains(ref.block) {
			in = append(in, ref)
		}
	}

	return in
}

// placeInLoops calls emit for each loop that should have a candidate for these
// references: the outermost eligible for those in it, narrowed to the smallest loop that
// has them all, so that the register is free in the rest of the outer loop. Inside a loop
// that isn't eligible, the loops inside it are tried.
func (a *allocator) placeInLoops(refs []reference, eligible func(*Loop) bool, emit func(*Loop, []reference)) {
	var try func(l *Loop)

	try = func(l *Loop) {
		in := refsIn(refs, l)
		if len(in) == 0 {
			return
		}

		if !eligible(l) {
			for _, child := range l.Children {
				try(child)
			}

			return
		}

		for narrowed := true; narrowed; {
			narrowed = false

			for _, child := range l.Children {
				if len(refsIn(in, child)) == len(in) && eligible(child) {
					l, narrowed = child, true

					break
				}
			}
		}

		emit(l, in)
	}

	for _, l := range a.g.Loops {
		if l.Parent == nil {
			try(l)
		}
	}
}

// valueType is how a copy of a value of type t is moved: as an integer of its width, which
// no reserved operand can stop, and sets nothing a program looks at.
func valueType(t ir.Type) ir.Type {
	switch t.Size() {
	case 4:
		return ir.Long
	case 8:
		return ir.Quad
	case 16:
		return ir.Octa
	}

	return t
}

// findVariables lists the variables that can be in a register for a loop, each in the
// outermost loop that allows it.
func (a *allocator) findVariables() {
	refsOf := map[*ir.Cell][]reference{}
	refsOfParam := map[*ir.Param][]reference{}

	for _, ref := range a.refs {
		switch {
		case ref.m.Cell != nil:
			refsOf[ref.m.Cell] = append(refsOf[ref.m.Cell], ref)
		case ref.m.Param != nil:
			refsOfParam[ref.m.Param] = append(refsOfParam[ref.m.Param], ref)
		}
	}

	for _, c := range a.r.Cells {
		if (c.Kind != ir.VariableCell && c.Kind != ir.ResultCell) || c.Size != c.T.Size() || c.T == ir.None {
			continue
		}

		home := &ir.Mem{Cell: c, T: c.T}
		if a.s.Volatile(a.s.Whole(home)) || a.overlapsAnother(c) || a.isCellCandidate(c) {
			continue
		}

		a.findVariable(home, c.Name, refsOf[c])
	}

	for _, p := range a.r.Params {
		if p.Sym == nil || len(p.Sym.Dims) != 0 || isCharacter(p) || p.Sym.Volatile || p.T == ir.None {
			continue
		}

		a.findVariable(&ir.Mem{Param: p, T: p.T}, p.Name, refsOfParam[p])
	}
}

// isCellCandidate reports whether the cell is one of the compiler's that is a candidate as
// it is.
func (a *allocator) isCellCandidate(c *ir.Cell) bool {
	for _, cand := range a.candidates {
		if cand.cell == c {
			return true
		}
	}

	return false
}

// overlapsAnother reports whether any other cell shares storage with c.
func (a *allocator) overlapsAnother(c *ir.Cell) bool {
	whole := a.s.Whole(&ir.Mem{Cell: c, T: c.T})

	for _, o := range a.r.Cells {
		if o != c && Conflict(whole, a.s.Whole(&ir.Mem{Cell: o, T: o.T})) {
			return true
		}
	}

	return false
}

// findVariable lists one variable's loops.
func (a *allocator) findVariable(home *ir.Mem, name string, refs []reference) {
	if len(refs) == 0 {
		return
	}

	eligible := func(l *Loop) bool { return a.variableEligible(l, home, refs) }

	a.placeInLoops(refs, eligible, func(l *Loop, in []reference) {
		written := false
		gain := 0

		for _, ref := range in {
			gain += a.weight(ref.block) * roleGain(ref.role)
			written = written || ref.role == roleWrite
		}

		moves := 1
		if written {
			moves += len(l.Exits)
		}

		gain -= moves * loopWeight(l.Depth-1)
		if gain < enoughGain {
			return
		}

		cell, param := home.Cell, home.Param

		c := &candidate{
			kind: otherCandidate, t: home.T, gain: gain, loop: l, home: home, written: written,
			holds: name + ", during a loop",
			place: func(to *ir.Cell, m *ir.Mem) *ir.Mem {
				if m.Cell != cell || m.Param != param || m.Ptr != nil {
					return nil
				}

				return &ir.Mem{Cell: to, T: m.T}
			},
		}

		if param != nil {
			a.valued[param] = append(a.valued[param], l)
		}

		a.candidates = append(a.candidates, c)
	})
}

// variableEligible reports whether a variable's value can be in a register for all of loop
// l: see the top of the file.
func (a *allocator) variableEligible(l *Loop, home *ir.Mem, refs []reference) bool {
	if !a.usable(l) || a.jumpsIndirectly(l) {
		return false
	}

	self := a.s.Place(home)

	if a.calls[l] {
		if home.Param != nil || (home.Cell != nil && home.Cell.Common != nil) || a.s.Taken(self) {
			return false
		}
	}

	in := refsIn(refs, l)

	surelyRead, surelyWritten, written := false, false, false

	for _, ref := range in {
		if !exact(ref.m, home.T) || ref.role == roleAddress || ref.role == roleText {
			return false
		}

		if a.surelyReached(l, ref) {
			surelyRead = true
			surelyWritten = surelyWritten || ref.role == roleWrite
		}

		written = written || ref.role == roleWrite
	}

	// Nothing else in the loop can touch it.
	for _, p := range a.touched[l] {
		if Conflict(p, self) && (p.Lo != self.Lo || p.Hi != self.Hi) {
			return false
		}
	}

	if home.Param != nil {
		if !surelyRead || (written && !surelyWritten) {
			return false
		}
	}

	if written && home.Cell != nil && home.Cell.Common != nil && home.Cell.Common.Attrs.NoWrite {
		return false
	}

	return true
}

// jumpsIndirectly reports whether an assigned GO TO is in loop l.
func (a *allocator) jumpsIndirectly(l *Loop) bool {
	for _, b := range l.Blocks {
		if _, jumps := b.End.(*ir.IndirectJump); jumps {
			return true
		}
	}

	return false
}

// findAddresses lists the dummy arguments whose addresses are worth a register, in the
// loops where their values aren't.
func (a *allocator) findAddresses() {
	refsOf := map[*ir.Param][]reference{}

	for _, ref := range a.refs {
		p := ref.m.Param

		if arg, ok := ref.m.Ptr.(*ir.ArgPointer); ok {
			p = arg.Param
		}

		if p != nil && !a.claimed[ref.m] && !a.inValued(p, ref.block) {
			refsOf[p] = append(refsOf[p], ref)
		}
	}

	for _, p := range a.r.Params {
		a.placeInLoops(refsOf[p], a.usable, func(l *Loop, in []reference) {
			gain := -loopWeight(l.Depth - 1)

			for _, ref := range in {
				gain += a.weight(ref.block)

				if ref.m.Offset != 0 {
					gain += a.weight(ref.block)
				}
			}

			if gain < enoughGain {
				return
			}

			param := p

			a.candidates = append(a.candidates, &candidate{
				kind: otherCandidate, t: ir.Long, gain: gain, loop: l,
				source: &ir.ArgPointer{Param: p},
				holds:  p.Name + "'s address",
				points: &ir.Mem{Param: p, T: ir.Byte},
				place: func(cell *ir.Cell, m *ir.Mem) *ir.Mem {
					at := &ir.Load{Mem: &ir.Mem{Cell: cell, T: ir.Long}}

					if arg, ok := m.Ptr.(*ir.ArgPointer); ok && arg.Param == param {
						n := *m
						n.Ptr, n.Deferred = at, false

						return &n
					}

					if m.Param != param {
						return nil
					}

					n := *m
					n.Param, n.Ptr, n.Deferred = nil, at, false

					return &n
				},
			})
		})
	}
}

// inValued reports whether block b is in a loop where dummy p's value may be in a register.
func (a *allocator) inValued(p *ir.Param, b *ir.Block) bool {
	for _, l := range a.valued[p] {
		if l.Contains(b) {
			return true
		}
	}

	return false
}

// conflict reports whether two candidates can't have one register.
func (a *allocator) conflict(x, y *candidate) bool {
	switch {
	case x.cell != nil && y.cell != nil:
		return a.together[x.index][y.index]
	case x.cell != nil:
		return occupies(y.loop, x.blocks)
	case y.cell != nil:
		return occupies(x.loop, y.blocks)
	}

	return x.loop.Contains(y.loop.Header) || y.loop.Contains(x.loop.Header)
}

// occupies reports whether any of the blocks is in loop l.
func occupies(l *Loop, blocks map[*ir.Block]bool) bool {
	for b := range blocks {
		if l.Contains(b) {
			return true
		}
	}

	return false
}

// colour gives each candidate in order of gain the lowest registers no candidate it can't
// share with has, of those the expressions leave free, and reports whether any got one.
func (a *allocator) colour(busy [12]bool) bool {
	sort.SliceStable(a.candidates, func(i, j int) bool {
		return a.candidates[i].gain*a.candidates[j].t.Width() > a.candidates[j].gain*a.candidates[i].t.Width()
	})

	assigned := false

	for i, c := range a.candidates {
		if c.gain < enoughGain {
			continue
		}

		var taken [12]bool

		for _, o := range a.candidates[:i] {
			if o.register != 0 && a.conflict(c, o) {
				for k := range o.t.Width() {
					taken[o.register+k] = true
				}
			}
		}

		w := c.t.Width()

		for r := firstCellRegister; r+w-1 <= lastCellRegister; r++ {
			free := true

			for k := r; k < r+w; k++ {
				free = free && !taken[k] && !busy[k]
			}

			if free {
				c.register = r
				assigned = true

				break
			}
		}
	}

	return assigned
}

// apply rewrites the routine for every candidate that got a register.
func (a *allocator) apply() {
	var winners []*candidate

	for _, c := range a.candidates {
		if c.register == 0 {
			continue
		}

		if c.cell != nil {
			c.cell.Register = c.register
		} else {
			winners = append(winners, c)
		}
	}

	// Each kind in turn, deepest loops first: what a deeper loop puts before it is part
	// of the loop outside it, and is rewritten with it.
	sort.SliceStable(winners, func(i, j int) bool {
		x, y := winners[i], winners[j]
		if x.kind != y.kind {
			return x.kind < y.kind
		}

		return x.loop.Depth > y.loop.Depth
	})

	for _, c := range winners {
		a.install(c)
	}
}

// install has candidate c's register hold what it should through its loop.
func (a *allocator) install(c *candidate) {
	header := c.loop.Header
	g := NewGraph(a.r)

	var l *Loop

	for _, loop := range g.Loops {
		if loop.Header == header {
			l = loop
		}
	}

	a.copies++

	cell := &ir.Cell{
		Name: "$R" + strconv.Itoa(a.copies), Kind: ir.TempCell, T: c.t, Size: c.t.Size(),
		Register: c.register, Holds: c.holds, Points: c.points,
	}
	a.r.Cells = append(a.r.Cells, cell)

	rep := &replacer{key: c.key, with: &ir.Load{Mem: &ir.Mem{Cell: cell, T: c.t}}}
	if c.place != nil {
		rep = &replacer{place: func(m *ir.Mem) *ir.Mem { return c.place(cell, m) }}
	}

	for _, b := range l.Blocks {
		rep.rewrite(b, 0, len(b.Stmts))
	}

	before := a.before(g, l)

	var first ir.Stmt

	if c.home != nil {
		t := valueType(c.t)
		first = ir.NewAssign(before.End.Source(), &ir.Mem{Cell: cell, T: t}, &ir.Load{Mem: bitsOf(c.home, t)})
	} else {
		first = ir.NewAssign(before.End.Source(), &ir.Mem{Cell: cell, T: ir.Long}, c.source)
	}

	before.Stmts = append(before.Stmts, first)

	if c.home != nil && c.written {
		a.writeBack(g, l, c, cell)
	}

	a.keepStepping(l, c, cell)
}

// bitsOf is the place m read as an integer of the type t.
func bitsOf(m *ir.Mem, t ir.Type) *ir.Mem {
	n := *m
	n.T = t

	return &n
}

// before is the block just before loop l, which every way in goes through: the one that's
// there if it does nothing else, or a new one.
func (a *allocator) before(g *Graph, l *Loop) *ir.Block {
	var outside []*ir.Block

	for _, p := range g.Preds(l.Header) {
		if !l.Contains(p) {
			outside = append(outside, p)
		}
	}

	if len(outside) == 1 && l.Header != a.r.Blocks[0] {
		if _, jumps := outside[0].End.(*ir.Jump); jumps {
			return outside[0]
		}
	}

	return a.mover.preheader(g, l)
}

// writeBack gives a variable's value back to its home on each way out of loop l. Each way
// out is an edge, which gets a block of its own, so that the loop's trips pay nothing.
// A way out to what comes next goes straight on through the new block; one that needs its
// branch to stay short and its fall-through as it was has its block after the routine's
// code, as such a way out is seldom the common one.
func (a *allocator) writeBack(g *Graph, l *Loop, c *candidate, cell *ir.Cell) {
	seen := map[Edge]bool{}
	t := valueType(c.t)

	for _, e := range l.Exits {
		if seen[e] {
			continue
		}

		seen[e] = true

		from := e.From.End.Source()
		store := ir.NewAssign(from, bitsOf(c.home, t), &ir.Load{Mem: &ir.Mem{Cell: cell, T: t}})

		if a.exitOf[e.To] && len(g.Preds(e.To)) == 1 {
			e.To.Stmts = append(e.To.Stmts, store)

			continue
		}

		a.exits++

		out := &ir.Block{Name: "W" + strconv.Itoa(a.exits), Note: leavingLoop, Stmts: []ir.Stmt{store}}
		out.End = ir.NewJump(from, e.To)
		a.exitOf[out] = true

		redirect(e.From.End, e.To, out)
		a.place(e, out)
	}
}

// place lays out a way out's block.
func (a *allocator) place(e Edge, out *ir.Block) {
	blocks := a.r.Blocks

	for i, b := range blocks {
		if b != e.From {
			continue
		}

		_, jumps := b.End.(*ir.Jump)
		if jumps || (i+1 < len(blocks) && blocks[i+1] == e.To) {
			a.r.Blocks = append(blocks[:i+1:i+1], append([]*ir.Block{out}, blocks[i+1:]...)...)

			return
		}
	}

	a.r.Blocks = append(blocks, out)
}

// closeUpRegisters takes the compiler's cells that became registers out of $LOCAL. A
// function's value has a symbol, and leaves a hole where it was.
func (a *allocator) closeUpRegisters() {
	gone := map[*ir.Cell]bool{}

	for _, c := range a.r.Cells {
		if c.Common == nil && c.Holds == "" && c.Register != 0 && c.Sym == nil {
			gone[c] = true
		}
	}

	closeUpBoth(a.r, gone)

	for _, c := range a.r.Cells {
		if c.Register != 0 {
			c.Offset = 0
		}
	}
}
