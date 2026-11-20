package sema

import "github.com/tucats/govax/internal/fortran/ast"

// Array constructors whose values aren't all constants (Fortran 90's 4.5;
// docs/planning/PHASE-102 - Fortran 90 array expressions.md, finding 10):
// (/ X, A(2:3), (I * Y, I = 1, N) /). A constructor of constants is an
// ArrayConst (constarray.go); any other is a Constructor, which lowering builds
// when the statement runs, an item at a time, into an array temporary or
// straight into an assignment's target (ir/lowerctor.go). Its extent is known
// when compiling if every item's is: a scalar is one element, an array its
// size, an implied DO its trip count, when its bounds are constants, times its
// items' extent.
//
// An implied DO's variable has the implied DO as its scope (the standard's
// 14.1.3): it is a variable of its own, of the type the unit's name would
// have, which the program's variable of that name never sees. So its items are
// analyzed in a scope of their own (scope.go), as a statement function's
// expression is.

// ctorBuilder gathers an array constructor's items: the type of the first
// value, which every other must have, and the extent so far.
type ctorBuilder struct {
	a     *analyzer
	t     Type
	typed bool
	size  int
	known bool

	// lengthOf is the first CHARACTER value whose length isn't known when
	// compiling (Constructor.LengthOf).
	lengthOf Expr
}

// constructor analyzes an array constructor: an ArrayConst if its values are
// constants, else a Constructor.
func (a *analyzer) constructor(e *ast.ArrayConstructor) Expr {
	if v, ok := a.folded(e); ok {
		return v
	}

	b := &ctorBuilder{a: a, known: true}

	items, ok := b.items(e.Values)
	if !ok {
		return a.bad(e.Pos)
	}

	if !b.typed {
		b.t = IntegerType
	}

	c := &Constructor{exprBase: exprBase{T: b.t, Pos: e.Pos}, Items: items}
	if b.t.Kind == Character && b.t.Size == 0 {
		c.LengthOf = b.lengthOf
	}
	c.S = Shape{{Size: b.size, Known: b.known}}

	if !b.known {
		c.S[0] = Extent{Of: c}
	}

	return c
}

// items analyzes a list of a constructor's items.
func (b *ctorBuilder) items(list []ast.Expr) ([]CtorItem, bool) {
	out := make([]CtorItem, 0, len(list))

	for _, x := range list {
		if d, ok := x.(*ast.ImpliedDo); ok {
			do, ok := b.impliedDo(d)
			if !ok {
				return nil, false
			}

			out = append(out, CtorItem{Do: do})

			continue
		}

		// operand, not arrayValue: a value of a derived type, or an array of
		// one, is an item too, (/ POINT(1.0, 2.0), P, PATH(2:3) /).
		v, ok := b.value(b.a.operand(x))
		if !ok {
			return nil, false
		}

		if size, known := v.Shape().Size(); known {
			b.size += size
		} else {
			b.known = false
		}

		out = append(out, CtorItem{Value: v})
	}

	return out, true
}

// value is v, one of the constructor's values, checked against the first's
// type (CONSTRUCTTYPE, CONSTRUCTLEN), an INTEGER of another length converted
// to it.
func (b *ctorBuilder) value(v Expr) (Expr, bool) {
	t := v.Type()

	switch {
	case t.Kind == Invalid:
		return v, false

	case t.Kind == Character && t.Size == 0 && b.lengthOf == nil && (!b.typed || b.t.Size == 0) &&
		!lengthFromNames(v):
		// The elements' length is computed from the first value of a length
		// not constant, before the items are: so it must be one whose length
		// its names give, whatever an implied DO's variable is.
		b.a.notImplForm(v.Position(), "an array constructor whose first CHARACTER value of a length not constant "+
			"isn't a variable's, an array's, or a concatenation of them")

		return v, false

	case !b.typed:
		if t.Kind == Character && t.Size == 0 {
			b.lengthOf = v
		}

		b.t, b.typed = t, true

	case t == b.t:

	case t.Kind == Record && b.t.Kind == Record && sameType(t.Struct, b.t.Struct):
		// Two SEQUENCE definitions of one type (4.4.2).

	case t.Kind == Integer && b.t.Kind == Integer:
		v = b.a.convert(v, b.t)

	case t.Kind == Character && b.t.Kind == Character:
		// Values whose lengths are known when compiling must have one
		// length; one known only when the program runs is the program's to
		// get right (the standard's 4.5), made the elements' length as an
		// assignment would (ours).
		if b.t.Size > 0 && t.Size > 0 {
			b.a.report("CONSTRUCTLEN", v.Position(), b.t.Size, t.Size)

			return v, false
		}

	default:
		b.a.report("CONSTRUCTTYPE", v.Position(), b.t.String(), t.String())

		return v, false
	}

	return v, true
}

// impliedDo analyzes an implied-DO list of a constructor: its bounds in the
// unit's scope, its items in one of its own, where its variable is a new
// INTEGER variable.
func (b *ctorBuilder) impliedDo(d *ast.ImpliedDo) (*CtorDo, bool) {
	a := b.a

	bound := func(x ast.Expr) Expr {
		return a.convert(a.arithmetic(a.value(x)), IntegerType)
	}

	do := &CtorDo{Start: bound(d.Start), Limit: bound(d.Limit), Step: IntConstAt(1, d.Pos)}
	if d.Step != nil {
		do.Step = bound(d.Step)
	}

	for _, x := range []Expr{do.Start, do.Limit, do.Step} {
		if x.Type().Kind == Invalid {
			return nil, false
		}
	}

	if st, ok := constantInt(do.Step); ok && st == 0 {
		a.report("DATADOSTEP", d.Step.Position())

		return nil, false
	}

	t, _ := a.scope.rules().typeOf(d.Var.Name)
	if outer := a.scope.find(d.Var.Name); outer != nil {
		t = outer.Type
		outer.typesEntity = true
	}

	if t.Kind != Integer {
		a.report("CTORDOVAR", d.Var.Pos, d.Var.Name)

		return nil, false
	}

	do.Var = NewSymbol(d.Var.Name, t)

	host := a.scope
	inner := newInnerScope(host)
	inner.define(do.Var)

	a.scope = inner
	size, known := b.size, b.known
	b.size, b.known = 0, true

	items, ok := b.items(d.Items)

	a.scope = host

	if !ok {
		return nil, false
	}

	do.Items = items

	// The list's extent: its trip count times its items', when both are
	// known.
	start, okStart := constantInt(do.Start)
	limit, okLimit := constantInt(do.Limit)
	step, okStep := constantInt(do.Step)

	if okStart && okLimit && okStep && b.known {
		size += max((limit-start+step)/step, 0) * b.size
	} else {
		known = false
	}

	b.size, b.known = size, known

	return do, true
}

// lengthFromNames reports whether the CHARACTER value v's length is one its
// names give, which lowering can compute before the constructor's items: a
// variable, an element, an array or section, or a concatenation of them and
// of values of lengths known when compiling.
func lengthFromNames(v Expr) bool {
	switch v := v.(type) {
	case *Var, *Elem, *ArrayRef, *Section:
		return true
	case *Concat:
		for _, p := range v.Parts {
			if p.Type().Size == 0 && !lengthFromNames(p) {
				return false
			}
		}

		return true
	}

	return false
}
