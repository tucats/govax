package sema

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Array constants (Fortran 90's 4.5 and 7.1.6.1; docs/planning/PHASE-102 -
// Fortran 90 array expressions.md, finding 10). A constant expression may be
// an array: an array constructor of constants, (/ 1, 2, (I, I = 3, 10) /); an
// array PARAMETER, or a section of one with constant subscripts; an elemental
// operation or intrinsic function of them; RESHAPE of constants, the only way
// to write a constant of rank 2 or more; and SHAPE, and LBOUND and UBOUND
// without DIM=, of an array whose bounds are constant. Its value is an
// ArrayConst, its elements Consts in array element order, each computed by the
// scalar evaluator's own code (constant.go), so an array's element has exactly
// the value the same scalar expression would have.
//
// The scalar evaluator, eval, refuses an array form; evalAny takes either.
// Which forms are arrays is decided from the syntax tree first (isArrayForm),
// so an expression is evaluated once, and its messages come once.

// maxConstElements is the most elements an array constant may have: more is a
// mistake (an implied DO's limit gone wrong), not a program.
const maxConstElements = 1 << 20

// knownShape is the shape of the given extents, every one known.
func knownShape(extents ...int) Shape {
	s := make(Shape, len(extents))
	for i, n := range extents {
		s[i] = Extent{Size: n, Known: true}
	}

	return s
}

// newArrayConst is an array constant of type t and the given extents.
func newArrayConst(t Type, elems []*Const, pos diag.Position, extents ...int) *ArrayConst {
	return &ArrayConst{exprBase: exprBase{T: t, Pos: pos, S: knownShape(extents...)}, Elems: elems}
}

// extents are an array constant's extents.
func (c *ArrayConst) extents() []int {
	list := make([]int, len(c.S))
	for i, e := range c.S {
		list[i] = e.Size
	}

	return list
}

// arrayFunctions are Fortran 90's array inquiry and construction functions
// the evaluator answers (fortran90Table's ARRAY rows), and TRANSFER, which may
// be a scalar or an array (transferConst).
var arrayFunctions = map[string]bool{
	"RESHAPE": true, "SHAPE": true, "SIZE": true, "LBOUND": true, "UBOUND": true, "TRANSFER": true,
}

// isArrayForm reports whether e, as a constant expression, would be an array:
// an array constructor, an array constant's name, a section of one, RESHAPE,
// SHAPE, LBOUND or UBOUND without DIM=, an elemental intrinsic with an array
// argument, or an operation with an array operand.
func (ev *evaluator) isArrayForm(e ast.Expr) bool {
	switch e := ev.a.substringForm(e).(type) {
	case *ast.ArrayConstructor:
		return true

	case *ast.Name:
		if _, ok := ev.vars[e.Name]; ok {
			return false
		}

		s := ev.a.scope.find(e.Name)

		return s != nil && s.ArrayValue != nil

	case *ast.Apply:
		s := ev.a.scope.find(e.Name)

		// A structure constructor is a scalar (structvalue.go).
		if ev.a.constructorType(e.Name, s) != nil {
			return false
		}

		if s != nil && s.ArrayValue != nil {
			for _, arg := range e.Args {
				if _, ok := arg.(*ast.Triplet); ok || ev.isArrayForm(arg) {
					return true
				}
			}

			return false
		}

		if s != nil && s.Class != Intrinsic && (s.Class != Unclassified || s.Dummy != 0) {
			return false
		}

		if !ev.a.intrinsicNamed(e.Name) {
			return false
		}

		switch e.Name {
		case "RESHAPE", "SHAPE":
			return true
		case "LBOUND", "UBOUND":
			return len(e.Args) == 1
		case "TRANSFER":
			return ev.transferIsArray(e)
		}

		if _, ok := constFuncs[e.Name]; ok {
			for _, arg := range e.Args {
				if k, ok := arg.(*ast.KeywordArg); ok {
					arg = k.X
				}

				if ev.isArrayForm(arg) {
					return true
				}
			}
		}

	case *ast.Paren:
		return ev.isArrayForm(e.X)

	case *ast.Unary:
		return ev.isArrayForm(e.X)

	case *ast.Binary:
		return ev.isArrayForm(e.X) || ev.isArrayForm(e.Y)
	}

	return false
}

// evalAny is the constant expression e's value: a scalar, c, or an array,
// arr.
func (ev *evaluator) evalAny(e ast.Expr) (c *Const, arr *ArrayConst, err *constErr) {
	if !ev.isArrayForm(e) {
		c, err = ev.eval(e)

		return c, nil, err
	}

	arr, err = ev.evalArray(e)

	return nil, arr, err
}

// notScalar is an array form where the scalar evaluator was asked for a
// scalar.
func (ev *evaluator) notScalar(e ast.Expr) *constErr {
	return ev.report("NOTSCALAR", e.Position())
}

// evalArray is the value of e, an array form (isArrayForm).
func (ev *evaluator) evalArray(e ast.Expr) (*ArrayConst, *constErr) {
	switch e := ev.a.substringForm(e).(type) {
	case *ast.ArrayConstructor:
		return ev.constructor(e)

	case *ast.Name:
		s := ev.a.scope.find(e.Name)
		ev.a.note(&s.refs, e.Pos, RefUsed)

		return s.ArrayValue.at(e.Pos), nil

	case *ast.Apply:
		if s := ev.a.scope.find(e.Name); s != nil && s.ArrayValue != nil {
			_, arr, err := ev.constantSection(s, e)

			return arr, err
		}

		ev.a.noteIntrinsic(e.Name, e.Pos)

		switch e.Name {
		case "RESHAPE":
			return ev.reshape(e)
		case "SHAPE", "LBOUND", "UBOUND":
			return ev.boundsArray(e)
		case "TRANSFER":
			_, arr, err := ev.transferConst(e)

			return arr, err
		}

		return ev.elemental(e)

	case *ast.Paren:
		return ev.evalArray(e.X)

	case *ast.Unary:
		x, err := ev.evalArray(e.X)
		if err != nil {
			return nil, err
		}

		return ev.mapped(e.Pos, x.T, x.extents(), len(x.Elems), func(i int) (*Const, *constErr) {
			return ev.unaryOn(e, x.Elems[i])
		})

	case *ast.Binary:
		return ev.binaryArray(e)
	}

	return nil, notConst("this is not a constant", e.Position())
}

// mapped is the array of the given extents whose element i is f(i), of type t
// if it has none to say.
func (ev *evaluator) mapped(pos diag.Position, t Type, extents []int, n int, f func(i int) (*Const, *constErr)) (*ArrayConst, *constErr) {
	elems := make([]*Const, n)

	for i := range n {
		c, err := f(i)
		if err != nil {
			return nil, err
		}

		elems[i] = c.at(pos)
	}

	if n > 0 {
		t = elems[0].T
	}

	return newArrayConst(t, elems, pos, extents...), nil
}

// binaryArray is X op Y with an array operand: the operation on each pair of
// elements, a scalar operand standing for each.
func (ev *evaluator) binaryArray(e *ast.Binary) (*ArrayConst, *constErr) {
	xc, xa, err := ev.evalAny(e.X)
	yc, ya, err2 := ev.evalAny(e.Y)

	if err = worse(err, err2); err != nil {
		return nil, err
	}

	shape := xa
	if shape == nil {
		shape = ya
	}

	if xa != nil && ya != nil && !slices.Equal(xa.extents(), ya.extents()) {
		return nil, ev.report("NOTCONFORM", e.Pos, xa.S.String(), ya.S.String())
	}

	at := func(c *Const, a *ArrayConst, i int) *Const {
		if a != nil {
			return a.Elems[i]
		}

		return c
	}

	t := shape.T
	if e.Op.Relational() {
		t = LogicalType
	}

	return ev.mapped(e.Pos, t, shape.extents(), len(shape.Elems), func(i int) (*Const, *constErr) {
		return ev.binaryOn(e, at(xc, xa, i), at(yc, ya, i))
	})
}

// constructor is the value of an array constructor of constants (Fortran 90's
// 4.5): its values in order, an array's elements in array element order, an
// implied DO's items once for each value of its variable. Every value has one
// type and length (CONSTRUCTTYPE, CONSTRUCTLEN), but an INTEGER of another
// length, which takes the first value's.
func (ev *evaluator) constructor(e *ast.ArrayConstructor) (*ArrayConst, *constErr) {
	var (
		elems []*Const
		t     Type
		typed bool
	)

	add := func(c *Const) *constErr {
		switch {
		case !typed:
			t, typed = c.T, true
		case c.T == t:
		case c.T.Kind == Integer && t.Kind == Integer:
			cv, err := ev.convert(c, t, c.Pos)
			if err != nil {
				return err
			}

			c = cv
		case c.T.Kind == Character && t.Kind == Character:
			return ev.report("CONSTRUCTLEN", c.Pos, t.Size, c.T.Size)
		default:
			return ev.report("CONSTRUCTTYPE", c.Pos, t.String(), c.T.String())
		}

		if len(elems) >= maxConstElements {
			return ev.report("CONSTRUCTSIZE", e.Pos, maxConstElements)
		}

		elems = append(elems, c.at(e.Pos))

		return nil
	}

	var walk func(items []ast.Expr) *constErr

	walk = func(items []ast.Expr) *constErr {
		for _, item := range items {
			if d, ok := item.(*ast.ImpliedDo); ok {
				if err := ev.impliedDo(d, walk); err != nil {
					return err
				}

				continue
			}

			c, arr, err := ev.evalAny(item)
			if err != nil {
				return err
			}

			if arr == nil {
				if err := add(c); err != nil {
					return err
				}

				continue
			}

			for _, el := range arr.Elems {
				if err := add(el); err != nil {
					return err
				}
			}

			if !typed {
				t, typed = arr.T, true
			}
		}

		return nil
	}

	if err := walk(e.Values); err != nil {
		return nil, err
	}

	if !typed {
		// Only implied DOs of no trips: the type their items would have.
		t = ev.emptyType(e.Values)
	}

	return newArrayConst(t, elems, e.Pos, len(elems)), nil
}

// impliedDo runs an implied-DO list of an array constructor of constants,
// calling walk on its items for each value of its variable, which is a
// constant (ev.vars) while they are evaluated.
func (ev *evaluator) impliedDo(d *ast.ImpliedDo, walk func([]ast.Expr) *constErr) *constErr {
	// The variable is an INTEGER variable of the implied DO's own, of the type
	// the unit's name would have (constructor.go).
	t, _ := ev.a.scope.rules().typeOf(d.Var.Name)
	if outer := ev.a.scope.find(d.Var.Name); outer != nil {
		t = outer.Type
	}

	if t.Kind != Integer {
		return ev.report("CTORDOVAR", d.Var.Pos, d.Var.Name)
	}

	bound := func(x ast.Expr) (int64, *constErr) {
		c, err := ev.eval(x)
		if err != nil {
			return 0, err
		}

		if c.T.Kind != Integer {
			return 0, ev.report("CTORDOBOUND", x.Position())
		}

		return int64(c.Int), nil
	}

	start, err := bound(d.Start)
	if err != nil {
		return err
	}

	limit, err := bound(d.Limit)
	if err != nil {
		return err
	}

	step := int64(1)

	if d.Step != nil {
		if step, err = bound(d.Step); err != nil {
			return err
		}

		if step == 0 {
			return ev.report("DATADOSTEP", d.Step.Position())
		}
	}

	saved := ev.vars
	ev.vars = make(map[string]int32, len(saved)+1)

	for k, v := range saved {
		ev.vars[k] = v
	}

	defer func() { ev.vars = saved }()

	for v := start; step > 0 && v <= limit || step < 0 && v >= limit; v += step {
		ev.vars[d.Var.Name] = int32(v)

		if err := walk(d.Items); err != nil {
			return err
		}
	}

	return nil
}

// emptyType is the type of an array constructor's values when it has none:
// its first value's, evaluated quietly with each implied DO's variable at its
// start.
func (ev *evaluator) emptyType(items []ast.Expr) Type {
	quiet := &evaluator{a: ev.a, quiet: true, vars: map[string]int32{}}

	for k, v := range ev.vars {
		quiet.vars[k] = v
	}

	for _, item := range items {
		if d, ok := item.(*ast.ImpliedDo); ok {
			if c, err := quiet.eval(d.Start); err == nil {
				quiet.vars[d.Var.Name] = c.Int
			}

			if t := quiet.emptyType(d.Items); t.Kind != Invalid {
				return t
			}

			continue
		}

		if c, arr, err := quiet.evalAny(item); err == nil {
			if arr != nil {
				return arr.T
			}

			return c.T
		}
	}

	return invalid
}

// constSubscripts are one dimension's subscripts of a constant section: the
// values it takes, and whether the section ranges over the dimension.
type constSubscripts struct {
	values []int
	ranges bool
}

// constantSection is s(subscripts), s an array constant, with constant
// subscripts: an element, c, if no subscript ranges, else a section, arr.
func (ev *evaluator) constantSection(s *Symbol, e *ast.Apply) (c *Const, arr *ArrayConst, err *constErr) {
	ev.a.note(&s.refs, e.Pos, RefUsed)

	if len(e.Args) != len(s.Dims) {
		return nil, nil, ev.report("SUBSCRIPTS", e.Pos, e.Name, count(len(s.Dims), "subscript"), len(e.Args))
	}

	dims := make([]constSubscripts, len(e.Args))

	for i, arg := range e.Args {
		d := s.Dims[i]

		check := func(v int, pos diag.Position) *constErr {
			switch {
			case v < d.Lower:
				return ev.report("SUBSCRIPTRANGE", pos, v, e.Name, "lower", d.Lower)
			case v > d.Upper:
				return ev.report("SUBSCRIPTRANGE", pos, v, e.Name, "upper", d.Upper)
			}

			return nil
		}

		if t, ok := arg.(*ast.Triplet); ok {
			l, u, st := d.Lower, d.Upper, 1

			for _, b := range []struct {
				x    ast.Expr
				into *int
			}{{t.Lower, &l}, {t.Upper, &u}, {t.Stride, &st}} {
				if b.x == nil {
					continue
				}

				v, err := ev.eval(b.x)
				if err != nil {
					return nil, nil, err
				}

				*b.into = int(v.Int)
			}

			if st == 0 {
				return nil, nil, ev.report("ZEROSTRIDE", t.Stride.Position())
			}

			sub := constSubscripts{ranges: true}

			for v := l; st > 0 && v <= u || st < 0 && v >= u; v += st {
				if err := check(v, arg.Position()); err != nil {
					return nil, nil, err
				}

				sub.values = append(sub.values, v)
			}

			dims[i] = sub

			continue
		}

		xc, xa, err := ev.evalAny(arg)
		if err != nil {
			return nil, nil, err
		}

		if xa == nil {
			xa = newArrayConst(xc.T, []*Const{xc}, xc.Pos, 1)
		} else if len(xa.S) != 1 {
			return nil, nil, ev.report("VECTORRANK", arg.Position(), len(xa.S))
		}

		dims[i].ranges = xc == nil

		for _, el := range xa.Elems {
			if el.T.Kind != Integer {
				return nil, nil, ev.report("ARRAYARG", arg.Position(), e.Name, "subscript, an INTEGER value,")
			}

			if err := check(int(el.Int), arg.Position()); err != nil {
				return nil, nil, err
			}

			dims[i].values = append(dims[i].values, int(el.Int))
		}
	}

	var extents []int

	for _, d := range dims {
		if d.ranges {
			extents = append(extents, len(d.values))
		}
	}

	elems := make([]*Const, 0)

	// Every combination of the dimensions' values, the leftmost varying
	// fastest: an odometer.
	at := make([]int, len(dims))

	for {
		empty := false
		number, multiplier := 0, 1

		for i, d := range dims {
			if len(d.values) == 0 {
				empty = true

				break
			}

			number += (d.values[at[i]] - s.Dims[i].Lower) * multiplier
			multiplier *= s.Dims[i].Extent()
		}

		if empty {
			break
		}

		elems = append(elems, s.ArrayValue.Elems[number].at(e.Pos))

		i := 0
		for ; i < len(dims); i++ {
			if at[i]++; at[i] < len(dims[i].values) {
				break
			}

			at[i] = 0
		}

		if i == len(dims) {
			break
		}
	}

	if extents == nil {
		return elems[0], nil, nil
	}

	return nil, newArrayConst(s.ArrayValue.T, elems, e.Pos, extents...), nil
}

// arrayArgs are an array function's arguments in its dummies' order
// (intrinsicSlots), nil for one left out.
func (ev *evaluator) arrayArgs(e *ast.Apply) ([]ast.Expr, *constErr) {
	slots, ok := ev.a.intrinsicSlots(e, formsNamed(e.Name), ev.quiet)
	if !ok {
		return nil, failed
	}

	forms := formsNamed(e.Name)
	if !argCountFits(forms, len(slots.args)) {
		return nil, ev.report("INTRINARGS", e.Pos, e.Name, takes(forms), len(slots.args))
	}

	return slots.args, nil
}

// arrayArg is an array function's argument that must be an array constant.
func (ev *evaluator) arrayArg(e *ast.Apply, x ast.Expr, what string) (*ArrayConst, *constErr) {
	c, arr, err := ev.evalAny(x)
	if err != nil {
		return nil, err
	}

	if arr == nil {
		return nil, ev.report("ARRAYARG", c.Pos, e.Name, what)
	}

	return arr, nil
}

// reshape is RESHAPE(SOURCE, SHAPE, PAD, ORDER) of constants (the standard's
// 13.13.84): an array of the shape SHAPE, its elements SOURCE's in array
// element order, then PAD's over and over, placed in the order ORDER says
// (its first dimension varying fastest), or array element order without it.
func (ev *evaluator) reshape(e *ast.Apply) (*ArrayConst, *constErr) {
	args, err := ev.arrayArgs(e)
	if err != nil {
		return nil, err
	}

	source, err := ev.arrayArg(e, args[0], "SOURCE")
	if err != nil {
		return nil, err
	}

	shape, err := ev.intVector(e, args[1], "SHAPE")
	if err != nil {
		return nil, err
	}

	total := 1

	for _, n := range shape {
		if n < 0 {
			return nil, ev.report("BADRESHAPE", e.Pos, "SHAPE has an extent below zero")
		}

		total *= n
	}

	if total > maxConstElements {
		return nil, ev.report("CONSTRUCTSIZE", e.Pos, maxConstElements)
	}

	sequence := source.Elems

	if len(args) > 2 && args[2] != nil {
		pad, err := ev.arrayArg(e, args[2], "PAD")
		if err != nil {
			return nil, err
		}

		if pad.T != source.T {
			return nil, ev.report("BADRESHAPE", e.Pos, "PAD's type isn't SOURCE's")
		}

		for len(pad.Elems) > 0 && len(sequence) < total {
			sequence = append(slices.Clone(sequence), pad.Elems...)
		}
	}

	if len(sequence) < total {
		return nil, ev.report("BADRESHAPE", e.Pos, "SOURCE has "+strconv.Itoa(len(source.Elems))+
			" elements, too few for the shape, and no PAD")
	}

	order := make([]int, len(shape))
	for i := range order {
		order[i] = i
	}

	if len(args) > 3 && args[3] != nil {
		o, err := ev.intVector(e, args[3], "ORDER")
		if err != nil {
			return nil, err
		}

		seen := make([]bool, len(shape))
		ok := len(o) == len(shape)

		for i := 0; ok && i < len(o); i++ {
			ok = o[i] >= 1 && o[i] <= len(shape) && !seen[o[i]-1]
			if ok {
				seen[o[i]-1] = true
				order[i] = o[i] - 1
			}
		}

		if !ok {
			return nil, ev.report("BADRESHAPE", e.Pos, "ORDER isn't a permutation of 1 to the shape's rank")
		}
	}

	elems := make([]*Const, total)
	subs := make([]int, len(shape))

	for k := range total {
		number, multiplier := 0, 1

		for i, n := range shape {
			number += subs[i] * multiplier
			multiplier *= n
		}

		elems[number] = sequence[k].at(e.Pos)

		// The next subscripts, ORDER's first dimension fastest.
		for _, d := range order {
			if subs[d]++; subs[d] < shape[d] {
				break
			}

			subs[d] = 0
		}
	}

	return newArrayConst(source.T, elems, e.Pos, shape...), nil
}

// intVector is an argument that must be an INTEGER array constant of rank
// one, as Go ints.
func (ev *evaluator) intVector(e *ast.Apply, x ast.Expr, what string) ([]int, *constErr) {
	arr, err := ev.arrayArg(e, x, what)
	if err != nil {
		return nil, err
	}

	if len(arr.S) != 1 || arr.T.Kind != Integer {
		return nil, ev.report("ARRAYARG", arr.Pos, e.Name, what+", an INTEGER array of rank one,")
	}

	list := make([]int, len(arr.Elems))
	for i, c := range arr.Elems {
		list[i] = int(c.Int)
	}

	return list, nil
}

// arrayBounds are the lower and upper bounds of x, an array inquiry
// function's ARRAY or SOURCE: a whole array's whose bounds are constant, or
// any other array constant's, 1 and its extents (the standard's 13.13.52: an
// array expression's lower bounds are 1).
func (ev *evaluator) arrayBounds(e *ast.Apply, x ast.Expr) (lower, upper []int, err *constErr) {
	if n, ok := x.(*ast.Name); ok {
		if s := ev.a.scope.find(n.Name); s != nil && (s.Class == Array || s.ArrayValue != nil) {
			ev.a.note(&s.refs, n.Pos, RefUsed)

			for _, d := range s.Dims {
				if !d.known() {
					return nil, nil, notConst("the bounds of "+n.Name+" aren't constants", n.Pos)
				}

				// An extent of zero has the bounds 1 and 0.
				lo, hi := d.Lower, d.Upper
				if hi < lo {
					lo, hi = 1, 0
				}

				lower, upper = append(lower, lo), append(upper, hi)
			}

			return lower, upper, nil
		}
	}

	arr, err := ev.arrayArg(e, x, "argument")
	if err != nil {
		return nil, nil, err
	}

	for _, n := range arr.extents() {
		lower, upper = append(lower, 1), append(upper, n)
	}

	return lower, upper, nil
}

// boundsArray is SHAPE(SOURCE), LBOUND(ARRAY), or UBOUND(ARRAY) (the
// standard's 13.13.52, 13.13.97, 13.13.111), an INTEGER array of rank one.
func (ev *evaluator) boundsArray(e *ast.Apply) (*ArrayConst, *constErr) {
	args, err := ev.arrayArgs(e)
	if err != nil {
		return nil, err
	}

	lower, upper, err := ev.arrayBounds(e, args[0])
	if err != nil {
		return nil, err
	}

	elems := make([]*Const, len(lower))

	for i := range lower {
		v := upper[i]

		switch e.Name {
		case "SHAPE":
			v = upper[i] - lower[i] + 1
		case "LBOUND":
			v = lower[i]
		}

		elems[i] = IntConstAt(v, e.Pos)
	}

	return newArrayConst(IntegerType, elems, e.Pos, len(elems)), nil
}

// boundsScalar is SIZE(ARRAY[, DIM]), or LBOUND or UBOUND with DIM=, a
// scalar INTEGER.
func (ev *evaluator) boundsScalar(e *ast.Apply) (*Const, *constErr) {
	args, err := ev.arrayArgs(e)
	if err != nil {
		return nil, err
	}

	lower, upper, err := ev.arrayBounds(e, args[0])
	if err != nil {
		return nil, err
	}

	dim := -1

	if len(args) > 1 && args[1] != nil {
		d, err := ev.eval(args[1])
		if err != nil {
			return nil, err
		}

		if d.T.Kind != Integer || d.Int < 1 || int(d.Int) > len(lower) {
			return nil, ev.report("DIMRANGE", args[1].Position(), e.Name, ast.DumpExpr(args[1]), len(lower))
		}

		dim = int(d.Int) - 1
	}

	switch {
	case e.Name == "LBOUND":
		return IntConstAt(lower[dim], e.Pos), nil
	case e.Name == "UBOUND":
		return IntConstAt(upper[dim], e.Pos), nil
	case dim >= 0:
		return IntConstAt(upper[dim]-lower[dim]+1, e.Pos), nil
	}

	n := 1
	for i := range lower {
		n *= upper[i] - lower[i] + 1
	}

	return IntConstAt(n, e.Pos), nil
}

// elemental is one of constFuncs with an array argument: the function of each
// element, a scalar argument standing for each (Fortran 90's 13.2.1).
func (ev *evaluator) elemental(e *ast.Apply) (*ArrayConst, *constErr) {
	fn := constFuncs[e.Name]

	slots := intrinsicArgs{args: e.Args}

	if forms := formsNamed(e.Name); len(forms) > 0 {
		var ok bool
		if slots, ok = ev.a.intrinsicSlots(e, forms, ev.quiet); !ok {
			return nil, failed
		}
	}

	if n := len(slots.args); n < fn.min || fn.max >= 0 && n > fn.max {
		return nil, ev.report("INTRINARGS", e.Pos, e.Name, fn.takes, n)
	}

	scalars := make([]*Const, len(slots.args))
	arrays := make([]*ArrayConst, len(slots.args))

	var shape *ArrayConst

	for i, arg := range slots.args {
		if arg == nil {
			return nil, ev.report("NULLINTRINARG", e.Pos, e.Name)
		}

		c, arr, err := ev.evalAny(arg)
		if err != nil {
			return nil, err
		}

		scalars[i], arrays[i] = c, arr

		switch {
		case arr == nil:
		case shape == nil:
			shape = arr
		case !slices.Equal(shape.extents(), arr.extents()):
			return nil, ev.report("NOTCONFORM", e.Pos, shape.S.String(), arr.S.String())
		}
	}

	return ev.mapped(e.Pos, shape.T, shape.extents(), len(shape.Elems), func(k int) (*Const, *constErr) {
		args := make([]*Const, len(scalars))

		for i := range args {
			if args[i] = scalars[i]; arrays[i] != nil {
				args[i] = arrays[i].Elems[k]
			}
		}

		c, err := fn.eval(ev, e, args)
		if err != nil || slots.kind == nil {
			return c, err
		}

		return ev.withKind(c, slots.kind)
	})
}
