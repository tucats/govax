package ir

import (
	"slices"
	"strings"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Fortran 90's CASE construct (the standard's 8.1.3; docs/planning/PHASE-101 -
// Fortran 90 control constructs.md, finding 7 and subtask 4):
//
//	SELECT CASE (I)
//	CASE (1, 3:5)
//	   ...
//	CASE DEFAULT
//	   ...
//	END SELECT
//
// The SELECT CASE statement is all the dispatch: its expression is computed
// once, then compared with the case values, and control goes to the block of
// the case that matches, or DEFAULT's, or past END SELECT. Each block is laid
// out after it in the order written, and ends with a jump past END SELECT, as
// an IF construct's blocks end. Semantic analysis has made sure no two ranges
// overlap, so the order the comparisons are made in doesn't change which case
// is chosen, only how long it takes to choose it: that freedom is what lets
// an INTEGER construct use a table or a binary search.
//
// How the dispatch is made depends on the expression's type:
//
//   - LOGICAL: one Branch on its bit 0 (appendix C.4 of the language manual),
//     to the .TRUE. case's block and the .FALSE. case's.
//   - INTEGER: the value widened to a longword (so that a value no selector of
//     its kind can hold simply never matches), then a table, a binary search,
//     or a chain of comparisons, as caseForm decides.
//   - CHARACTER: a chain of comparisons, or a binary search when there are
//     many, each a CharCmp (blank-padding the shorter, as every CHARACTER
//     relation does). No table: text has no index.
//
// A constant expression is decided here: a jump to its case.

// caseEnd is one end of a case value range: an INTEGER's value, or a
// CHARACTER constant.
type caseEnd struct {
	n int64
	c *sema.Const
}

// caseRange is a case value range as the dispatch tests it: its ends, which
// are open (no test needed: every value on that side matches) when the
// source left them out or, for INTEGER, when they reach as far as the
// expression's kind; single when it is one value; and the block of the case
// it belongs to.
type caseRange struct {
	lo, hi         caseEnd
	openLo, openHi bool
	single         bool
	to             *Block
}

// tests is the number of comparisons testing r alone takes: one for a single
// value, one for each end that isn't open.
func (r caseRange) tests() int {
	if r.single {
		return 1
	}

	n := 0
	if !r.openLo {
		n++
	}

	if !r.openHi {
		n++
	}

	return n
}

// bounded reports whether both of r's ends are values: a range a table can
// hold.
func (r caseRange) bounded() bool { return !r.openLo && !r.openHi }

// selection is one SELECT CASE's dispatch being lowered: how to compare its
// expression's value with a case value (test, the comparison's IR) and how
// two case values are ordered (order), for INTEGER or CHARACTER.
type selection struct {
	l      *lowerer
	ranges []caseRange
	none   *Block

	test  func(op Operator, v caseEnd) Expr
	order func(x, y caseEnd) int
}

// The forms an INTEGER construct's dispatch can take.
type form int

const (
	chainForm form = iota
	searchForm
	tableForm
)

// caseForm chooses the INTEGER dispatch's form from tests, the comparisons a
// chain would make (one per single value, one per end of a range that isn't
// open), and span, the number of values from the lowest bounded case value
// to the highest (0 when there are no bounded ranges); the author's answer 6
// of the phase's survey:
//
//   - A table (CASEL) when tests >= 4 and span <= 4 * tests. CASEL is one
//     instruction whatever the value, and its table costs a word a value; a
//     chain costs about six bytes a comparison (CMPL and a branch) and runs
//     through half of them on average. Four slots a test keeps the table
//     about as small as the chain while always as fast; below four tests a
//     chain is about as quick as CASEL is.
//   - Otherwise a binary search over the sorted ranges when tests >= 8: about
//     log2(tests) comparisons rather than tests/2.
//   - Otherwise a chain of comparisons in the order the cases are written,
//     since a programmer may well put the commonest case first.
//
// They are thresholds, nothing more: one function, easy to change once real
// programs can be measured.
func caseForm(tests int, span int64) form {
	switch {
	case tests >= 4 && span > 0 && span <= 4*int64(tests):
		return tableForm
	case tests >= 8:
		return searchForm
	}

	return chainForm
}

// selectCase lowers a SELECT CASE construct: the dispatch, then each case's
// block in the order written, each ending with a jump to the block after END
// SELECT (a jump to END SELECT's label goes there too).
func (l *lowerer) selectCase(s *sema.Select) {
	var join *Block

	if s.EndSelect != nil && s.EndSelect.Base().Label != 0 {
		join = l.labelBlock(s.EndSelect.Base().Label)
	} else {
		join = l.newBlock()
	}

	blocks := make([]*Block, len(s.Cases))
	none := join

	for i, c := range s.Cases {
		blocks[i] = l.newBlock()

		if c.Default {
			none = blocks[i]
		}
	}

	switch s.Expr.Type().Kind {
	case sema.Logical:
		l.selectLogical(s, blocks, none)
	case sema.Integer:
		l.selectInteger(s, blocks, none)
	default:
		l.selectCharacter(s, blocks, none)
	}

	l.release()

	for i, c := range s.Cases {
		if i > 0 {
			l.from = c.AST
			l.jump(join)
		}

		l.begin(blocks[i])
		l.stmts(c.Body)
	}

	l.from = s.EndSelect
	l.begin(join)
}

// selectLogical is a LOGICAL construct's dispatch: a branch to the .TRUE.
// case's block or the .FALSE. case's, either of which may be DEFAULT's or
// none at all.
func (l *lowerer) selectLogical(s *sema.Select, blocks []*Block, none *Block) {
	onTrue, onFalse := none, none

	for i, c := range s.Cases {
		for _, r := range c.Ranges {
			if r.Low.Int&1 != 0 {
				onTrue = blocks[i]
			} else {
				onFalse = blocks[i]
			}
		}
	}

	l.branch(l.stmtCond(s.Expr), onTrue, onFalse)
}

// selectInteger is an INTEGER construct's dispatch. The value goes into a
// longword temporary, unless it is already a longword variable, which
// nothing in the dispatch can change; a constant is a jump to its case.
func (l *lowerer) selectInteger(s *sema.Select, blocks []*Block, none *Block) {
	kindLo, kindHi := intRange(l.typeOf(s.Expr.Type()))

	var ranges []caseRange

	for i, c := range s.Cases {
		for _, r := range c.Ranges {
			cr := caseRange{lo: caseEnd{n: kindLo}, hi: caseEnd{n: kindHi}, single: r.Single(), to: blocks[i]}

			if r.Low != nil {
				cr.lo.n = max(kindLo, int64(r.Low.Int))
			}

			if r.High != nil {
				cr.hi.n = min(kindHi, int64(r.High.Int))
			}

			if cr.lo.n > cr.hi.n {
				// An empty range, or one past what the kind holds:
				// CASENEVER's, which matches nothing.
				continue
			}

			cr.openLo, cr.openHi = cr.lo.n == kindLo, cr.hi.n == kindHi
			ranges = append(ranges, cr)
		}
	}

	value := l.stmtExpr(s.Expr)

	if c, ok := value.(*Const); ok {
		to := none

		for _, r := range ranges {
			if n := int64(c.Int()); r.lo.n <= n && n <= r.hi.n {
				to = r.to
			}
		}

		l.jump(to)

		return
	}

	if value.Type() != Long {
		value = arith(Cvt, Long, value, nil)
	}

	load, ok := value.(*Load)
	if !ok || load.Mem.Index != nil || load.Mem.Ptr != nil || load.Mem.Volatile {
		t := l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: t, Src: value})
		load = &Load{t}
	}

	// The value is where the comparisons read it: what the statement must
	// do at its end (freeing a dynamic string a function's argument used)
	// can be done now.
	l.flush()

	// Each comparison loads the value afresh: no two IR nodes share a tree.
	place := *load.Mem
	sel := &selection{
		l: l, ranges: ranges, none: none,
		test: func(op Operator, v caseEnd) Expr {
			m := place

			return &Cmp{op, &Load{&m}, LongConst(int32(v.n))}
		},
		order: func(x, y caseEnd) int {
			switch {
			case x.n < y.n:
				return -1
			case x.n > y.n:
				return 1
			}

			return 0
		},
	}

	// The comparisons a chain would make, and how many values the bounded
	// ranges span.
	tests := 0
	low, high := int64(0), int64(-1)

	for _, r := range ranges {
		tests += r.tests()

		if r.bounded() {
			if high < low {
				low, high = r.lo.n, r.hi.n
			} else {
				low, high = min(low, r.lo.n), max(high, r.hi.n)
			}
		}
	}

	switch caseForm(tests, high-low+1) {
	case tableForm:
		sel.table(place, low, high)
	case searchForm:
		sel.search()
	default:
		sel.chain(ranges)
	}
}

// selectCharacter is a CHARACTER construct's dispatch. The value is compared
// where it is: a variable, an array element, or a substring, whose place the
// comparisons all use; or a temporary a concatenation or a function's value
// was put in. A place found at run time (an index, a dummy's text, a dynamic
// string) is found once, its address and length kept in longword temporaries.
// A dynamic string is freed only once the dispatch is done (finish).
func (l *lowerer) selectCharacter(s *sema.Select, blocks []*Block, none *Block) {
	var ranges []caseRange

	for i, c := range s.Cases {
		for _, r := range c.Ranges {
			cr := caseRange{single: r.Single(), to: blocks[i], openLo: r.Low == nil, openHi: r.High == nil}
			cr.lo.c, cr.hi.c = r.Low, r.High

			if !cr.single && !cr.openLo && !cr.openHi && textOrder(r.Low.Text, r.High.Text) > 0 {
				continue // its low end above its high: CASENEVER's.
			}

			ranges = append(ranges, cr)
		}
	}

	if c, ok := s.Expr.(*sema.Const); ok {
		to := none

		for _, r := range ranges {
			if (r.openLo || textOrder(r.lo.c.Text, c.Text) <= 0) && (r.openHi || textOrder(c.Text, r.hi.c.Text) <= 0) {
				to = r.to
			}
		}

		l.jump(to)

		return
	}

	held := len(l.after)
	value := l.chars(s.Expr)

	// Each comparison has a tree of its own: no two IR nodes share one.
	var text func() Text

	if n, ok := value.Len.(*Const); ok && value.Mem.Index == nil && value.Mem.Ptr == nil {
		text = func() Text {
			m := *value.Mem

			return Text{&m, LongConst(n.Int())}
		}
	} else {
		address, length := l.temp(Long), l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: address, Src: &Addr{value.Mem}})
		l.emit(&Assign{origin: l.origin(), Dst: length, Src: value.Len})

		text = func() Text {
			a, n := *address, *length

			return Text{&Mem{Ptr: &Load{&a}, T: Byte}, &Load{&n}}
		}
	}

	// What the statement must do at its end (freeing a dynamic string) waits
	// until every comparison is made (finish).
	after := l.after[held:]
	l.after = l.after[:held]

	sel := &selection{
		l: l, ranges: ranges, none: none,
		test: func(op Operator, v caseEnd) Expr {
			return &CharCmp{op, text(), l.chars(v.c)}
		},
		order: func(x, y caseEnd) int { return textOrder(x.c.Text, y.c.Text) },
	}

	done := func() {}
	if len(after) > 0 {
		done = sel.numbered(after)
	}

	tests := 0
	for _, r := range sel.ranges {
		tests += r.tests()
	}

	if tests >= 8 {
		sel.search()
	} else {
		sel.chain(sel.ranges)
	}

	done()
}

// numbered makes a selection whose value must be freed after the
// comparisons (after: a dynamic string's LIB$SFREE1_DD) go to its cases
// through a longword temporary: each match stores a number for its case,
// then one block does what after does and goes on by a Switch on the
// number. It returns what lays those blocks out, once the dispatch is.
func (s *selection) numbered(after []Stmt) func() {
	l := s.l
	number := l.temp(Long).Cell
	join := l.newBlock()

	var targets, stores []*Block

	store := map[*Block]*Block{}
	storeFor := func(to *Block) *Block {
		if b := store[to]; b != nil {
			return b
		}

		b := l.newBlock()
		store[to] = b
		targets = append(targets, to)
		stores = append(stores, b)

		return b
	}

	final := s.none
	s.none = storeFor(s.none)

	for i := range s.ranges {
		s.ranges[i].to = storeFor(s.ranges[i].to)
	}

	return func() {
		for i, b := range stores {
			l.begin(b)
			l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: number, T: Long}, Src: LongConst(int32(i + 1))})
			l.jump(join)
		}

		l.begin(join)

		for _, st := range after {
			l.emit(st)
		}

		l.terminate(&Switch{l.origin(), &Load{&Mem{Cell: number, T: Long}}, 1, targets, final})
	}
}

// chain is a chain of comparisons, one range after another in the order
// given: each a branch to its case's block, or on to the next test, and
// after the last to none.
func (s *selection) chain(ranges []caseRange) {
	l := s.l

	for _, r := range ranges {
		cond := s.condition(r, nil, nil)
		if cond == nil {
			// A range every value matches.
			l.jump(r.to)

			return
		}

		next := l.newBlock()
		l.branch(cond, r.to, next)
		l.begin(next)
	}

	l.jump(s.none)
}

// condition is the test that a value lies in r, when it is known to be at
// least *below (if below isn't nil) and less than *above (if above isn't
// nil): only the ends that knowledge doesn't settle are compared. It is nil
// when nothing needs comparing.
func (s *selection) condition(r caseRange, atLeast, under *caseEnd) Expr {
	if r.single {
		return s.test(Eq, r.lo)
	}

	var cond Expr

	if !r.openLo && (atLeast == nil || s.order(*atLeast, r.lo) < 0) {
		cond = s.test(Ge, r.lo)
	}

	if !r.openHi && (under == nil || !s.below(*under, r.hi)) {
		cond = arith(And, Long, cond, s.test(Le, r.hi))
	}

	return cond
}

// below reports whether a value less than x is sure to be no more than y: for
// INTEGER values when x is at most y+1, for text when x is at most y.
func (s *selection) below(x, y caseEnd) bool {
	if x.c == nil {
		return x.n <= y.n+1
	}

	return s.order(x, y) <= 0
}

// search is a binary search over the ranges, sorted by their low ends (no two
// overlap, so that sorts them wholly): the value compared with the middle
// range's low end, then a search of the half it must be in, knowing what that
// comparison said, down to a single range's test.
func (s *selection) search() {
	sorted := slices.Clone(s.ranges)
	slices.SortStableFunc(sorted, func(x, y caseRange) int {
		switch {
		case x.openLo:
			return -1
		case y.openLo:
			return 1
		}

		return s.order(x.lo, y.lo)
	})

	s.half(sorted, nil, nil)
}

// half searches ranges, the value known to be at least *atLeast and less than
// *under where they aren't nil.
func (s *selection) half(ranges []caseRange, atLeast, under *caseEnd) {
	l := s.l

	if len(ranges) == 1 {
		cond := s.condition(ranges[0], atLeast, under)
		if cond == nil {
			l.jump(ranges[0].to)

			return
		}

		l.branch(cond, ranges[0].to, s.none)

		return
	}

	mid := len(ranges) / 2
	split := ranges[mid].lo
	lower, upper := l.newBlock(), l.newBlock()

	l.branch(s.test(Lt, split), lower, upper)

	l.begin(lower)
	s.half(ranges[:mid], atLeast, &split)

	l.begin(upper)
	s.half(ranges[mid:], &split, under)
}

// table is an INTEGER dispatch by CASEL: a slot for each value from low to
// high, each the block of the bounded range holding it or none. A value
// outside the table may still be in an open range, which lies wholly beyond
// the bounded ones (none overlap), so CASEL's out-of-range exit goes to a
// chain of the open ranges' tests, ending at none.
func (s *selection) table(place Mem, low, high int64) {
	l := s.l
	slots := make([]*Block, high-low+1)

	for i := range slots {
		slots[i] = s.none
	}

	var open []caseRange

	for _, r := range s.ranges {
		if !r.bounded() {
			open = append(open, r)

			continue
		}

		for n := r.lo.n; n <= r.hi.n; n++ {
			slots[n-low] = r.to
		}
	}

	outside := s.none
	if len(open) > 0 {
		outside = l.newBlock()
	}

	l.terminate(&Switch{l.origin(), &Load{&place}, int32(low), slots, outside})

	if len(open) > 0 {
		l.begin(outside)
		s.chain(open)
	}
}

// textOrder is -1, 0, or 1 as the text x comes before, is equal to, or comes
// after y by FORTRAN's comparison of CHARACTER values: the shorter padded with
// blanks, then byte by byte in ASCII, as CMPC5 compares at run time.
func textOrder(x, y string) int {
	n := max(len(x), len(y))
	x += strings.Repeat(" ", n-len(x))
	y += strings.Repeat(" ", n-len(y))

	return strings.Compare(x, y)
}
