package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Overlap (docs/planning/PHASE-102 - Fortran 90 array expressions.md,
// findings 6 and 7). An array assignment must act as if its whole right side
// were computed before any element of its target is defined (Fortran 90's
// 7.5.1.5); the scalarizer computes it an element at a time, as it assigns, so
// that is right only where no element the loops read was written by an
// earlier pass. This is the test, and for each array the value reads it has
// four answers:
//
//   - apart: different storage (different names not joined by COMMON or
//     EQUIVALENCE, two dummy arguments), or the same array where a subscript
//     shows the two never meet (A(:, 1) = A(:, 2)); two fixed subscripts not
//     known to differ (A(I, :) = A(K, :)) are taken as equal, since either
//     answer, the same element or apart, needs nothing of the loops;
//   - the same element at each index of the loops (A = A + 1, A(1:N) =
//     2 * A(1:N));
//   - ordered: the same array, the same subscripts in every dimension but
//     some, where the strides are the same constant and the starts differ by
//     a constant. Then the element the leaf reads at the indexes k is the one
//     the target writes at k + q, and running the outermost loop whose q isn't
//     0 the right way (up if q is positive there, down if not) reads it before
//     it is written: A(2:N) = A(1:N-1) runs down, A(1:N-1) = A(2:N) up;
//   - unknown: anything else (different names that share storage, a section of
//     another form). Then the value goes into a temporary first, an array of
//     the assignment's shape, and the temporary is assigned to the target: two
//     nests. A temporary's size must be known when compiling (finding 7); one
//     that isn't is Phase 104's.
//
// Two arrays that need opposite directions in one dimension need a temporary
// too. A scalar the loops compute at each element (Inline) that reads the
// target's storage (A = A(1)) is computed once before them instead
// (ArrayOrder.Hoisted), unless it is CHARACTER, whose text is read where it is:
// then a temporary.

// ArrayOrder is how the scalarizer runs an array assignment (sema's answer to
// whether its sides overlap): Down says, for each loop (the target's
// dimensions, the leftmost first), whether it runs from the extent down to 1;
// Temporary that the value goes into an array of the extents TempShape first;
// Fill that the value, a Constructor that reads nothing of the target's, is
// stored straight into the target an item at a time, with no loops of the
// target's (finding 10: its extent then needn't be known when compiling).
type ArrayOrder struct {
	Down      []bool
	Temporary bool
	TempShape []int
	Fill      bool

	// target is the array the assignment writes, for Hoisted.
	target *Symbol

	// need is each loop's direction as the value's arrays ask it, 1 up, -1
	// down, 0 either, for a WHERE's mask to add its own to (where.go).
	need []int
}

// Hoisted reports whether lowering must compute the scalar e, one it would
// compute at each element (Inline), once before the loops: it reads storage
// the target may share.
func (o *ArrayOrder) Hoisted(e Expr) bool {
	if o == nil || o.Temporary || e.Type().Kind == Character {
		return false
	}

	for _, s := range readSymbols(e) {
		if mayShare(o.target, s) {
			return true
		}
	}

	return false
}

// String is the order as the dump shows it after an assignment: nothing for
// loops all running up with no temporary.
func (o *ArrayOrder) String() string {
	if o == nil {
		return ""
	}

	if o.Temporary {
		return "  [through a temporary]"
	}

	if o.Fill {
		return "  [filled]"
	}

	var down []string

	for i, d := range o.Down {
		if d {
			down = append(down, strconv.Itoa(i+1))
		}
	}

	if down == nil {
		return ""
	}

	return "  [down in " + strings.Join(down, ", ") + "]"
}

// overlapRelation is what overlap found between an assignment's target and one array
// its value reads.
type overlapRelation int

const (
	relApart   overlapRelation = iota // never the same element
	relSame                    // the same element at each index
	relOrdered                 // read before written, if one loop runs one way
	relUnknown                 // anything else: a temporary
)

// arrayOrder is how the array assignment target = value is to run, or false
// after NOTIMPL (a temporary whose size isn't known when compiling).
func (a *analyzer) arrayOrder(pos diag.Position, target, value Expr) (*ArrayOrder, bool) {
	rank := rankOf(target)
	ts := baseSymbol(target)
	o := &ArrayOrder{Down: make([]bool, rank), target: ts}

	// need is each loop's direction, 1 up, -1 down, 0 either.
	need := make([]int, rank)

	if !a.checkManyOne(target) {
		return nil, false
	}

	if c, ok := value.(*Constructor); ok && rank == 1 && !a.noFill && !reads(c, ts) {
		o.Fill = true

		return o, true
	}

	if !a.constructorSizes(value) {
		return nil, false
	}

	// The target's subscripts must be computed before any element is defined
	// (7.5.1.5); a vector subscript's elements are read as the loops go.
	if vectorsRead(target, ts) {
		a.notImplForm(pos, "a vector subscript in an assignment's target that reads the target's own array")

		return nil, false
	}

	o.Temporary = !relateValue(target, value, need)

	if !o.Temporary {
		o.need = need

		for i, d := range need {
			o.Down[i] = d < 0
		}

		return o, true
	}

	shape, _ := a.conform(pos, target.Shape(), value.Shape())

	if _, known := shape.Size(); !known || (value.Type().Kind == Character && value.Type().Size == 0) {
		a.notImplForm(pos, "an array assignment that needs a temporary of a size not known when compiling (Phase 104)")

		return nil, false
	}

	for _, e := range shape {
		o.TempShape = append(o.TempShape, e.Size)
	}

	return o, true
}

// relateValue relates each array the value reads to the target, adding the
// direction each loop must run in to need (1 up, -1 down, 0 either). False if
// the value must go into a temporary: an array related to the target in no way
// the loops can keep, two that need opposite directions, a vector subscript
// that reads the target's storage, or a CHARACTER scalar that does.
func relateValue(target, value Expr, need []int) bool {
	ts := baseSymbol(target)

	// A vector subscript in the value that reads the target's storage is read
	// as the loops go too: a temporary.
	temporary := vectorsRead(value, ts)

	Scalarize(value, func(leaf Expr) Expr {
		// An array function computed at each element (a reduction with DIM=,
		// MATMUL, TRANSPOSE) that reads the target needs a temporary; any
		// other is computed before the loops.
		if f, ok := leaf.(*ArrayFunc); ok && f.Element() && funcReads(f, ts) {
			temporary = true
		}

		if temporary || !mayShare(ts, baseSymbol(leaf)) {
			return leaf
		}

		switch rel, loop, dir := relate(target, leaf); rel {
		case relOrdered:
			if need[loop] == -dir {
				temporary = true
			}

			need[loop] = dir

		case relUnknown:
			temporary = true
		}

		return leaf
	}, func(x Expr) Expr {
		if Inline(x) && x.Type().Kind == Character {
			for _, s := range readSymbols(x) {
				if mayShare(ts, s) {
					temporary = true
				}
			}
		}

		return x
	})

	return !temporary
}

// checkManyOne reports a section in an assignment's target whose vector
// subscript is a constant with a value twice (Fortran 90's 6.2.2.3.2: a
// many-one section may not be defined), MANYONE: false after it. A vector that
// isn't constant is the program's to get right, as the standard leaves it.
func (a *analyzer) checkManyOne(target Expr) bool {
	if s, ok := target.(*Substring); ok {
		target = s.X
	}

	sec, ok := target.(*Section)
	if !ok {
		return true
	}

	for _, sub := range sec.Subs {
		v, ok := sub.Vector.(*ArrayConst)
		if !ok {
			continue
		}

		seen := map[int32]bool{}

		for _, c := range v.Elems {
			if seen[c.Int] {
				a.report("MANYONE", v.Pos, sec.Sym.Name, c.Int)

				return false
			}

			seen[c.Int] = true
		}
	}

	return true
}

// constructorSizes reports a Constructor among the value's arrays whose extent
// isn't known when compiling: it is built into an array temporary, which
// must have a size known when compiling (finding 7), unless it is the whole
// of an assignment's value (ArrayOrder.Fill). False after NOTIMPL.
func (a *analyzer) constructorSizes(value Expr) bool {
	for _, leaf := range Leaves(value) {
		if c, ok := leaf.(*Constructor); ok && !c.S[0].Known {
			a.notImplForm(c.Pos, "an array constructor of a size not known when compiling, but as an array's whole value, "+
				"(Phase 104)")

			return false
		}
	}

	return true
}

// reads reports whether the constructor c reads storage ts may share, in any
// value or implied DO's bound.
func reads(c *Constructor, ts *Symbol) bool {
	found := false

	check := func(e Expr) {
		if rankOf(e) == 0 {
			for _, s := range readSymbols(e) {
				found = found || mayShare(ts, s)
			}

			if !Inline(e) {
				// A function's value, or an operation's: what it reads.
				found = found || readsAny(e, ts)
			}

			return
		}

		found = found || vectorsRead(e, ts)

		Scalarize(e, func(leaf Expr) Expr {
			found = found || leafReads(leaf, ts)

			return leaf
		}, func(x Expr) Expr {
			found = found || readsAny(x, ts)

			return x
		})
	}

	var walk func([]CtorItem)

	walk = func(items []CtorItem) {
		for _, it := range items {
			if it.Do == nil {
				check(it.Value)

				continue
			}

			check(it.Do.Start)
			check(it.Do.Limit)
			check(it.Do.Step)
			walk(it.Do.Items)
		}
	}

	walk(c.Items)

	return found
}

// leafReads reports whether the array leaf reads storage ts may share: a
// constructor's or an array function's values, or an array's own storage.
func leafReads(leaf Expr, ts *Symbol) bool {
	switch leaf := leaf.(type) {
	case *Constructor:
		return reads(leaf, ts)
	case *ArrayFunc:
		return funcReads(leaf, ts)
	case *ArrayConst:
		return false
	}

	return mayShare(ts, baseSymbol(leaf))
}

// funcReads reports whether the array function f reads storage ts may share,
// in any of its arguments.
func funcReads(f *ArrayFunc, ts *Symbol) bool {
	found := false

	for _, x := range []Expr{f.Array, f.Other, f.Mask, f.DimExpr} {
		if x != nil {
			found = found || arrayReads(x, ts)
		}
	}

	return found
}

// arrayReads reports whether the expression x, an array or a scalar, reads
// storage ts may share: its arrays, its vector subscripts, its scalars.
func arrayReads(x Expr, ts *Symbol) bool {
	if rankOf(x) == 0 {
		return readsAny(x, ts)
	}

	found := vectorsRead(x, ts)

	Scalarize(x, func(leaf Expr) Expr {
		found = found || leafReads(leaf, ts)

		return leaf
	}, func(s Expr) Expr {
		found = found || readsAny(s, ts)

		return s
	})

	return found
}

// readsAny reports whether the scalar e may read storage ts may share: its
// variables and elements, and anything a function it calls is given (a
// function may read its arguments, or a common block: an EXTERNAL function's
// reference is taken to read anything).
func readsAny(e Expr, ts *Symbol) bool {
	found := false

	var walk func(Expr)

	walk = func(e Expr) {
		switch e := e.(type) {
		case nil:
		case *Var:
			found = found || mayShare(ts, e.Sym)
		case *Elem:
			found = found || mayShare(ts, e.Sym)

			for _, s := range e.Subs {
				walk(s)
			}
		case *FieldRef:
			found = found || mayShare(ts, e.Sym)
		case *Substring:
			walk(e.X)
			walk(e.Lower)
			walk(e.Upper)
		case *Concat:
			for _, p := range e.Parts {
				walk(p)
			}
		case *Convert:
			walk(e.X)
		case *Unary:
			walk(e.X)
		case *Binary:
			walk(e.X)
			walk(e.Y)
		case *Compare:
			walk(e.X)
			walk(e.Y)
		case *Const, *Procedure:
		case *ArrayRef, *Section:
			found = found || mayShare(ts, baseSymbol(e))
		case *ArrayFunc:
			found = found || funcReads(e, ts)
		case *IntrinsicRef:
			for _, x := range e.Args {
				walk(x)
			}
		case *StmtFuncRef:
			// Its expression may name anything of the unit's.
			found = true
		case *FuncRef:
			// What it is given, and what it can reach otherwise: a common
			// block, or a dummy argument's storage, the caller's.
			found = found || ts.Common != nil || ts.Dummy != 0 || ts.group != nil

			for _, x := range e.Args {
				if x.Expr != nil {
					found = found || mayShare(ts, baseSymbol(x.Expr)) || readsAny(x.Expr, ts)
				}
			}
		default:
			found = true
		}
	}

	walk(e)

	return found
}

// vectorsRead reports whether a vector subscript anywhere in the array
// expression e (one of its sections', or theirs in turn) reads storage ts may
// share: an array, or a scalar in it.
func vectorsRead(e Expr, ts *Symbol) bool {
	found := false

	var vectors func(Expr)

	reads := func(v Expr) {
		Scalarize(v, func(leaf Expr) Expr {
			if mayShare(ts, baseSymbol(leaf)) {
				found = true
			}

			vectors(leaf)

			return leaf
		}, func(x Expr) Expr {
			for _, s := range readSymbols(x) {
				if mayShare(ts, s) {
					found = true
				}
			}

			return x
		})
	}

	vectors = func(leaf Expr) {
		if s, ok := leaf.(*Section); ok {
			for _, sub := range s.Subs {
				if sub.Vector != nil {
					reads(sub.Vector)
				}
			}
		}
	}

	for _, leaf := range Leaves(e) {
		vectors(leaf)
	}

	return found
}

// baseSymbol is the array an array-valued leaf or target is of.
func baseSymbol(e Expr) *Symbol {
	switch e := e.(type) {
	case *ArrayRef:
		return e.Sym
	case *Section:
		return e.Sym
	case *Substring:
		return baseSymbol(e.X)
	}

	return nil
}

// mayShare reports whether x and y may be the same storage, wholly or in
// part: the same name; two names in one common block whose places meet; or
// two names an EQUIVALENCE ties together. Two dummy arguments are taken to be
// apart, by the rule the optimizer keeps too (the standard's 12.5.2.9: a
// program may not change what one name refers to through another).
func mayShare(x, y *Symbol) bool {
	switch {
	case x == nil || y == nil:
		return false
	case x == y:
		return true
	case x.Dummy != 0 || y.Dummy != 0:
		return false
	case x.Common != nil || y.Common != nil:
		return x.Common == y.Common && x.Offset < y.Offset+y.Size() && y.Offset < x.Offset+x.Size()
	}

	return x.group != nil && x.group == y.group
}

// slot is one subscript of an array leaf or target, as overlap compares them:
// a fixed subscript, or a triplet's start and stride (its end doesn't matter:
// the two sides conform). A nil lower is the dimension's own lower bound, an
// adjustable array's, the same on both sides.
type slot struct {
	triplet       bool
	index         Expr
	vector        Expr
	lower, stride Expr
}

// slots are e's subscripts, e a whole array or a section (a substring's
// characters don't matter: an element's own characters are moved as if
// through a temporary, by MOVC3 and MOVC5).
func slots(e Expr) []slot {
	switch e := e.(type) {
	case *Substring:
		return slots(e.X)

	case *ArrayRef:
		list := make([]slot, len(e.Sym.Dims))

		for i, d := range e.Sym.Dims {
			list[i] = slot{triplet: true, stride: IntConstAt(1, e.Pos)}
			if d.LowerExpr == nil {
				list[i].lower = IntConstAt(d.Lower, e.Pos)
			}
		}

		return list

	case *Section:
		list := make([]slot, len(e.Subs))

		for i, s := range e.Subs {
			switch {
			case s.Vector != nil:
				list[i] = slot{vector: s.Vector}
			case s.Triplet():
				list[i] = slot{triplet: true, lower: s.Lower, stride: s.Stride}
			default:
				list[i] = slot{index: s.Index}
			}
		}

		return list
	}

	return nil
}

// relate is how the array leaf the value reads stands to the target, which may
// share its storage: for relOrdered also which loop must run which way (1 up,
// -1 down).
func relate(target, leaf Expr) (rel overlapRelation, loop, dir int) {
	if baseSymbol(target) != baseSymbol(leaf) {
		return relUnknown, 0, 0
	}

	t, l := slots(target), slots(leaf)
	if len(t) != len(l) {
		return relUnknown, 0, 0
	}

	rel, loop = relSame, -1
	j := 0

	for i := range t {
		x, y := t[i], l[i]

		switch {
		case x.vector != nil || y.vector != nil:
			// The same vector subscript is the same subscript at each index
			// (a vector with a value twice may not be assigned through:
			// Fortran 90's 6.2.2.3.2); any other is anything.
			if x.vector == nil || y.vector == nil || DumpExpr(x.vector) != DumpExpr(y.vector) {
				return relUnknown, 0, 0
			}

			j++

		case !x.triplet && !y.triplet:
			if DumpExpr(x.index) == DumpExpr(y.index) {
				continue
			}

			if c, ok := difference(y.index, x.index); ok && c != 0 {
				return relApart, 0, 0
			}

			// Two fixed subscripts whose difference isn't known when
			// compiling (A(I, K:N) and A(K, K:N)) are computed once, before
			// the loops: when the program runs they are equal, and this
			// dimension is the same element's, or they differ and the two
			// never meet, which any order of the loops is right for. So the
			// other dimensions decide, as if they were equal.
			continue

		case x.triplet && y.triplet:
			if DumpExpr(x.stride) != DumpExpr(y.stride) {
				return relUnknown, 0, 0
			}

			c, ok := difference(y.lower, x.lower)
			if !ok {
				return relUnknown, 0, 0
			}

			if c != 0 {
				st, known := constantInt(x.stride)
				if !known {
					return relUnknown, 0, 0
				}

				if c%st != 0 {
					return relApart, 0, 0
				}

				// The outermost loop with a difference decides (loops are
				// numbered from the leftmost, the innermost).
				rel, loop, dir = relOrdered, j, 1
				if c/st < 0 {
					dir = -1
				}
			}

			j++

		default:
			return relUnknown, 0, 0
		}
	}

	return rel, loop, dir
}

// difference is x - y when it is a constant: both constants, or the same
// expression plus or minus constants (N+1 and N-1 differ by 2), or both nil
// (the dimension's own bound).
func difference(x, y Expr) (int, bool) {
	if x == nil || y == nil {
		return 0, x == nil && y == nil
	}

	xs, xc := offsetForm(x)
	ys, yc := offsetForm(y)

	if xs != ys {
		return 0, false
	}

	return xc - yc, true
}

// offsetForm splits e into an expression's text and a constant added to it:
// N+1 is "N" and 1, 3 is "" and 3.
func offsetForm(e Expr) (string, int) {
	if c, ok := constantInt(e); ok {
		return "", c
	}

	if b, ok := e.(*Binary); ok {
		if c, ok := constantInt(b.Y); ok {
			switch b.Op {
			case ast.Add:
				s, k := offsetForm(b.X)

				return s, k + c
			case ast.Sub:
				s, k := offsetForm(b.X)

				return s, k - c
			}
		}

		if c, ok := constantInt(b.X); ok && b.Op == ast.Add {
			s, k := offsetForm(b.Y)

			return s, k + c
		}
	}

	return DumpExpr(e), 0
}

// Inline reports whether the scalarizer computes the scalar e in its loops,
// at each element, rather than once before them: a constant, a variable, an
// element whose subscripts are constants, and any CHARACTER value, whose text
// it reads where it is (the CHARACTER values it can't, a function's or a
// concatenation's of a length known only when the program runs, are NOTIMPL:
// checkArrayParts).
func Inline(e Expr) bool {
	switch e := e.(type) {
	case *Const, *Var:
		return true

	case *Elem:
		for _, s := range e.Subs {
			if _, ok := s.(*Const); !ok {
				return false
			}
		}

		return true
	}

	return e.Type().Kind == Character
}

// readSymbols are the variables and arrays a scalar inline expression reads.
func readSymbols(e Expr) []*Symbol {
	var list []*Symbol

	var walk func(Expr)

	walk = func(e Expr) {
		switch e := e.(type) {
		case *Var:
			list = append(list, e.Sym)
		case *Elem:
			list = append(list, e.Sym)

			for _, s := range e.Subs {
				walk(s)
			}
		case *FieldRef:
			list = append(list, e.Sym)
		case *Substring:
			walk(e.X)
			walk(e.Lower)
			walk(e.Upper)
		case *Concat:
			for _, p := range e.Parts {
				walk(p)
			}
		case *Convert:
			walk(e.X)
		}
	}

	walk(e)

	return list
}

// checkArrayParts reports the parts of an array assignment's value the
// scalarizer can't compute yet: a CHARACTER value made by a function, which
// would be called at each element, and a concatenation in an array expression
// whose length is known only when the program runs, whose dynamic string would
// be made at each element. False after NOTIMPL.
func (a *analyzer) checkArrayParts(value Expr) bool {
	ok := true

	var visit func(Expr)

	visit = func(e Expr) {
		if !ok || e == nil {
			return
		}

		switch e := e.(type) {
		case *FuncRef, *StmtFuncRef:
			if e.Type().Kind == Character {
				a.notImplForm(e.Position(), "a CHARACTER function's value in an array expression")
				ok = false
			}

		case *Concat:
			if e.T.Size == 0 {
				a.notImplForm(e.Position(), "a concatenation of a length not constant in an array expression")
				ok = false

				return
			}

			for _, p := range e.Parts {
				visit(p)
			}

		case *Substring:
			visit(e.X)

		case *Unary:
			visit(e.X)

		case *Binary:
			visit(e.X)
			visit(e.Y)

		case *Compare:
			visit(e.X)
			visit(e.Y)

		case *Convert:
			visit(e.X)

		case *IntrinsicRef:
			for _, x := range e.Args {
				visit(x)
			}
		}
	}

	visit(value)

	return ok
}
