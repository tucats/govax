package sema

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Structure constructors, type equality, and what operators do with derived
// types (the standard's 4.4.2, 4.4.4, 7.1.4, and 7.5.1; docs/planning/PHASE-105 -
// Fortran 90 derived types.md, findings 8 and 9).
//
// A structure constructor names a derived type and gives a value for each of
// its components, in order:
//
//	TYPE POINT
//	  REAL X, Y
//	END TYPE
//	TYPE(POINT) P
//	P = POINT(1.0, Y + 2.0)
//
// It looks like a function reference, POINT(...), and is told from one by the
// name: a type's name is a local name of class 1 (derived.go's
// checkTypeNames), so a name that is a type in scope and nothing else of the
// unit is a constructor. Each value is converted to its component's type as an
// assignment would convert it; a component of a derived type takes a value of
// that type, an object or another constructor.
//
// A constructor whose values are all constant is a constant itself (a Const
// whose Parts are the components' values and whose Raw is the value's bytes,
// padding zero): it may be a PARAMETER's value or an initial value, and a
// constant's component, ORIGIN%X, is a constant too. Lowering puts such a
// value in $PDATA. Any other constructor is a StructValue, which lowering
// builds a component at a time, in its target when it can or in a temporary.
//
// Fortran 90 gives a derived type only intrinsic assignment, a move of the
// whole value between two objects of one type, and no operators: P == Q has no
// meaning unless an interface block gives it one (NODERIVEDOP). Two types are
// the same type (sameType) when they are one definition, or two SEQUENCE
// definitions with the same name and the same components, which is how an
// interface body, which has no host to see the caller's definition, can name
// the caller's type.

// sameType reports whether two derived types or structures are one type (the
// standard's 4.4.2): the same definition, or two definitions of SEQUENCE types
// with the same name whose components agree in order, name, type, kind,
// length, and bounds. A STRUCTURE is the same only as itself (section 3.4).
func sameType(x, y *Structure) bool {
	switch {
	case x == y:
		return true
	case x == nil || y == nil || !x.Derived || !y.Derived || !x.Sequence || !y.Sequence:
		return false
	case x.Name != y.Name || len(x.Fields) != len(y.Fields):
		return false
	}

	for i, f := range x.Fields {
		g := y.Fields[i]

		switch {
		case f.Name != g.Name || len(f.Dims) != len(g.Dims) || f.Pointer != g.Pointer:
			return false
		case f.Pointer && f.Struct == x && g.Struct == y:
			// Each a pointer to its own type, NODE's NEXT: the same so far.
		case f.Struct != nil || g.Struct != nil:
			if !sameType(f.Struct, g.Struct) {
				return false
			}
		case f.Type != g.Type:
			return false
		}

		for k, d := range f.Dims {
			if d.Lower != g.Dims[k].Lower || d.Upper != g.Dims[k].Upper {
				return false
			}
		}
	}

	return true
}

// sameTypes reports whether two types are one: equal, or two derived types
// that sameType says are one.
func sameTypes(t, u Type) bool {
	if t.Kind == Record && u.Kind == Record && t.Struct != nil && u.Struct != nil {
		return sameType(t.Struct, u.Struct)
	}

	return t == u
}

// argumentStructures reports whether an actual argument's structure may stand
// for a dummy's: the same type, or, for two STRUCTUREs, the same form, VAX
// FORTRAN's rule for records (record.go's form).
func argumentStructures(given, wanted *Structure) bool {
	if sameType(given, wanted) {
		return true
	}

	return given != nil && wanted != nil && !given.Derived && !wanted.Derived && given.form() == wanted.form()
}

// shape is an array field's shape, its extents all known.
func (f *Field) shape() Shape {
	s := make(Shape, len(f.Dims))
	for i, d := range f.Dims {
		s[i] = Extent{Size: max(d.Extent(), 0), Known: true}
	}

	return s
}

// componentType is a component's type: its own, or for a component of a
// derived type the record type of that type.
func componentType(f *Field) Type {
	if f.Struct != nil {
		return recordType(f.Struct)
	}

	return f.Type
}

// constructorType is the derived type the name in name(...) makes a structure
// constructor of, or nil: a type defined in scope (finished, not the one being
// defined), the name being nothing else the unit has classified. s is the
// name's symbol, if it has one.
func (a *analyzer) constructorType(name string, s *Symbol) *Structure {
	if s != nil && (s.Class != Unclassified || s.Dummy != 0) {
		return nil
	}

	st := a.typeNamed(name)
	if st == nil || !st.Derived || st.open {
		return nil
	}

	return st
}

// constructorArgs checks a constructor's list against its type: no keywords
// (Fortran 90's constructors have none) and a value for each component
// (CONSTRCOUNT).
func (ev *evaluator) constructorArgs(st *Structure, e *ast.Apply) *constErr {
	// A type two USEs gave, and a type whose components are private outside
	// its module, have no constructor here (USEAMBIG, COMPPRIVATE).
	if two, ok := ev.a.ambiguousTypeOf(e.Name); ok {
		return ev.report("USEAMBIG", e.Pos, e.Name, two[0], two[1])
	}

	if st.PrivateComponents && !ev.a.insideModule(st.Module) {
		return ev.report("COMPPRIVATE", e.Pos, st.Name, st.Module)
	}

	for _, arg := range e.Args {
		switch x := arg.(type) {
		case *ast.KeywordArg:
			return ev.report("CONSTRKEYWORD", x.Pos, st.Name)
		case nil:
			return ev.report("CONSTRNULL", e.Pos, st.Name)
		}
	}

	if len(e.Args) != len(st.Fields) {
		return ev.report("CONSTRCOUNT", e.Pos, st.What(), count(len(st.Fields), "component"), len(e.Args))
	}

	return nil
}

// structConst is the value of the constructor e of the type st, if its values
// are all constant: each component's value converted to its type, and the
// whole value's bytes.
func (ev *evaluator) structConst(st *Structure, e *ast.Apply) (*Const, *constErr) {
	ev.a.noteOther(whatType, st.Name, e.Pos, RefUsed)

	if err := ev.constructorArgs(st, e); err != nil {
		return nil, err
	}

	c := &Const{exprBase: exprBase{T: recordType(st), Pos: e.Pos}, Parts: make([]Expr, len(st.Fields)), Raw: make([]byte, st.Size)}

	var first *constErr

	for i, f := range st.Fields {
		part, err := ev.componentConst(st, f, e.Args[i])
		if err != nil {
			// The first that isn't constant says why, unless a later one
			// has an error reported already.
			first = worse(first, err)

			continue
		}

		c.Parts[i] = part
		copy(c.Raw[f.Offset:], ev.a.partBytes(part))
	}

	if first != nil {
		return nil, first
	}

	return c, nil
}

// componentConst is the constant value arg gives the component f of the type
// st: a scalar converted to its type, or for an array component a scalar for
// every element or an array of the component's shape, each element converted.
func (ev *evaluator) componentConst(st *Structure, f *Field, arg ast.Expr) (Expr, *constErr) {
	t := componentType(f)

	// A pointer component's value is a target, which has an address only when
	// the program runs (docs/planning/PHASE-107 - Fortran 90 pointers.md,
	// finding 7): a constructor of a type with one is never a constant.
	if f.Pointer {
		return nil, notConst("the component "+f.Name+" of "+st.Name+" is a pointer, whose value is a target", arg.Position())
	}

	if f.Dims == nil {
		c, err := ev.eval(arg)
		if err != nil {
			return nil, err
		}

		return ev.componentConvert(st, f, c, t, arg.Position())
	}

	c, arr, err := ev.evalAny(arg)
	if err != nil {
		return nil, err
	}

	extents := make([]int, len(f.Dims))
	for i, d := range f.Dims {
		extents[i] = d.Extent()
	}

	values := make([]*Const, f.Elements())

	switch {
	case arr == nil:
		for i := range values {
			values[i] = c
		}

	case !slices.Equal(arr.extents(), extents):
		return nil, ev.report("NOTCONFORM", arg.Position(), knownShape(extents...).String(), arr.S.String())

	default:
		copy(values, arr.Elems)
	}

	for i, v := range values {
		cv, err := ev.componentConvert(st, f, v, t, arg.Position())
		if err != nil {
			return nil, err
		}

		values[i] = cv
	}

	return newArrayConst(t, values, arg.Position(), extents...), nil
}

// componentConvert is c converted to t, the type of st's component f, as an
// assignment converts (CONSTRVALUE if it can't be).
func (ev *evaluator) componentConvert(st *Structure, f *Field, c *Const, t Type, pos diag.Position) (*Const, *constErr) {
	cv, err := ev.convert(c, t, pos)
	if err == incompatible {
		return nil, ev.report("CONSTRVALUE", pos, valueWhat(c.T), f.Name, st.Name, valueWhat(t))
	}

	return cv, err
}

// valueWhat says what a value of type t is, for a message: "a REAL value", "an
// object of the derived type POINT".
func valueWhat(t Type) string {
	if t.Kind == Record && t.Struct != nil {
		return t.Struct.objectText()
	}

	return article(t.String()) + " value"
}

// partBytes is a component's constant value's bytes as memory holds them: a
// scalar's, a derived type's whole value, or an array's elements in order.
func (a *analyzer) partBytes(part Expr) []byte {
	switch p := part.(type) {
	case *Const:
		if p.T.Kind == Record {
			return p.Raw
		}

		return a.constBytes(p)

	case *ArrayConst:
		var b []byte
		for _, e := range p.Elems {
			b = append(b, a.partBytes(e)...)
		}

		return b
	}

	return nil
}

// structConstructor analyzes the constructor e of the type st in an
// executable statement: a constant if every value is one (structConst, through
// folded), else a StructValue of each component's value converted, a pointer
// component's a target (the place itself, or a pointer, as pointer
// assignment's). An array
// component's value is its bytes if it is constant, else an array value of the
// component's shape or a scalar for every element, which lowering assigns as
// an array assignment's (subtask 5).
func (a *analyzer) structConstructor(st *Structure, e *ast.Apply) Expr {
	if v, ok := a.folded(e); ok {
		return v
	}

	ev := &evaluator{a: a}
	if ev.constructorArgs(st, e) != nil {
		return a.bad(e.Pos)
	}

	v := &StructValue{exprBase: exprBase{T: recordType(st), Pos: e.Pos}, Values: make([]Expr, len(st.Fields)),
		Bytes: make([][]byte, len(st.Fields))}
	ok := true

	for i, f := range st.Fields {
		arg := e.Args[i]

		// A pointer component is pointer-assigned its value (7.5.1.5), which
		// must be a target it may point at (pointer.go).
		if f.Pointer {
			v.Values[i] = a.targetFor(st.Name+"%"+f.Name, componentType(f), 0, arg)
			if v.Values[i] == nil {
				v.Values[i], ok = a.bad(arg.Position()), false
			}

			continue
		}

		if f.Dims != nil {
			if _, err := (&evaluator{a: a, quiet: true}).componentConst(st, f, arg); err != nil && err.kind != constFailed {
				v.Values[i] = a.componentValue(st, f, arg)
				ok = ok && v.Values[i].Type().Kind != Invalid

				continue
			}

			part, err := ev.componentConst(st, f, arg)
			if err != nil {
				ok = false

				continue
			}

			v.Values[i], v.Bytes[i] = part, a.partBytes(part)

			continue
		}

		v.Values[i] = a.componentValue(st, f, arg)
		ok = ok && v.Values[i].Type().Kind != Invalid
	}

	if !ok {
		return a.bad(e.Pos)
	}

	return v
}

// componentValue analyzes arg, the value a constructor gives st's component f,
// converted to its type as an assignment's value is to its target (exec.go's
// assignedValue): a component of a derived type takes an object of that type, a
// CHARACTER one any CHARACTER value, a numeric or LOGICAL one a value of those
// types or a character constant (section 2.2.1.6). A scalar component takes a
// scalar; an array one an array of its shape, or a scalar for every element
// (4.4.4).
func (a *analyzer) componentValue(st *Structure, f *Field, arg ast.Expr) Expr {
	t := componentType(f)

	var v Expr
	if f.Struct != nil {
		v = a.aggregateValue(arg, f.Dims != nil)
	} else {
		v = a.assignedExpr(arg, t)
	}

	vt := v.Type()

	switch {
	case vt.Kind == Invalid:
		return v

	case rankOf(v) > 0 && f.Dims == nil:
		a.report("NOTSCALAR", v.Position())

		return a.bad(v.Position())

	case rankOf(v) > 0:
		if _, ok := a.conform(v.Position(), f.shape(), v.Shape()); !ok {
			return a.bad(v.Position())
		}
	}

	switch {
	case f.Struct != nil:
		if vt.Kind == Record && sameType(vt.Struct, f.Struct) {
			return v
		}

	case vt.Kind == Record:

	case t.Kind == Character && vt.Kind == Character:
		return v

	case vt.Kind == Character:
		if c, ok := v.(*Const); ok {
			if moved := a.movedText(c, t); moved != nil {
				return moved
			}

			return a.bad(v.Position())
		}

	case t.Kind == Character:

	default:
		return a.assigned(v, t)
	}

	a.report("CONSTRVALUE", v.Position(), valueWhat(vt), f.Name, st.Name, valueWhat(t))

	return a.bad(v.Position())
}

// component is the constant value of a component of a constant, ORIGIN%X or
// ORIGIN%C%Y, with constant subscripts for an array component's element. A
// whole array component, or one whose subscripts aren't constant, isn't a
// scalar constant: field (record.go) analyzes it, and lowering reads it from
// the constant's datum.
func (ev *evaluator) component(e *ast.FieldRef) (*Const, *constErr) {
	base, err := ev.componentBase(e.X)
	if err != nil {
		return nil, err
	}

	st := base.T.Struct

	i := slices.IndexFunc(st.Fields, func(f *Field) bool { return f.Name == e.Field })
	if i < 0 {
		return nil, ev.report("NOFIELD", e.Pos, st.What(), st.fieldWord(), e.Field)
	}

	f := st.Fields[i]

	switch part := base.Parts[i].(type) {
	case *Const:
		if e.Args != nil {
			return nil, notConst("the component "+e.Field+" isn't an array", e.Pos)
		}

		return part.at(e.Pos), nil

	case *ArrayConst:
		if len(e.Args) != len(f.Dims) {
			return nil, notConst("an array component as a whole is not a scalar constant", e.Pos)
		}

		n, multiplier := 0, 1

		for k, sub := range e.Args {
			c, err := ev.eval(sub)
			if err != nil {
				return nil, err
			}

			d := f.Dims[k]
			if c.T.Kind != Integer || int(c.Int) < d.Lower || int(c.Int) > d.Upper {
				return nil, notConst("its subscript isn't a constant in the component's bounds", sub.Position())
			}

			n += (int(c.Int) - d.Lower) * multiplier
			multiplier *= d.Extent()
		}

		return part.Elems[n].at(e.Pos), nil
	}

	return nil, failed
}

// componentBase is the constant of a derived type a component reference starts
// from: a constant's name, or a component of one that is of a derived type.
func (ev *evaluator) componentBase(x ast.Expr) (*Const, *constErr) {
	var c *Const

	switch x := x.(type) {
	case *ast.Name:
		s := ev.a.scope.find(x.Name)

		switch {
		case s == nil || s.Class != Constant:
			return nil, named(x.Name+" is not a constant", x.Pos)
		case s.ArrayValue != nil:
			// PTS%X of an array constant is an array, read from its datum.
			return nil, notConst("an array constant's components are an array", x.Pos)
		case s.Value == nil:
			return nil, failed
		}

		ev.a.note(&s.refs, x.Pos, RefUsed)
		c = s.Value

	case *ast.FieldRef:
		var err *constErr
		if c, err = ev.component(x); err != nil {
			return nil, err
		}

	case *ast.Apply:
		// An element of an array constant of a derived type, PTS(2).
		var err *constErr
		if c, err = ev.eval(x); err != nil {
			return nil, err
		}

	default:
		return nil, notConst("this is not a constant", x.Position())
	}

	if c.T.Kind != Record || c.T.Struct == nil {
		return nil, notConst("it has no components", x.Position())
	}

	return c, nil
}

// constantComponent reports, for a reference e that is to be defined (an
// assignment's target, an input item), whether it is a constant's component,
// ORIGIN%X or PTS(I)%X, after saying so: a constant is never defined.
func (a *analyzer) constantComponent(e *ast.FieldRef) bool {
	if !a.constantBase(e) {
		return false
	}

	x := ast.Expr(e)
	for f, ok := x.(*ast.FieldRef); ok; f, ok = x.(*ast.FieldRef) {
		x = f.X
	}

	var (
		name string
		pos  diag.Position
	)

	switch n := x.(type) {
	case *ast.Name:
		name, pos = n.Name, n.Pos
	case *ast.Apply:
		name, pos = n.Name, n.Pos
	}

	if s := a.scope.find(name); !s.broken {
		a.badUse(s, pos, "be assigned a value")
	}

	return true
}

// constantBase reports whether the reference e, a component reference, starts
// from a named constant, or an element of an array constant.
func (a *analyzer) constantBase(e *ast.FieldRef) bool {
	x := ast.Expr(e)

	for {
		f, ok := x.(*ast.FieldRef)
		if !ok {
			break
		}

		x = f.X
	}

	var name string

	switch n := x.(type) {
	case *ast.Name:
		name = n.Name
	case *ast.Apply:
		name = n.Name
	default:
		return false
	}

	s := a.scope.find(name)

	return s != nil && s.Class == Constant
}

// operand analyzes an operand of an operator: what arrayValue does, but a record
// or an object of a derived type as a whole stays one, for an interface block to
// give the operation a meaning (extendedOp, definedOp), or aggregateOperands to
// refuse.
func (a *analyzer) operand(e ast.Expr) Expr {
	switch x := a.substringForm(e).(type) {
	case *ast.Name:
		s := a.scope.find(x.Name)

		switch {
		case s == nil || s.broken:

		case s.Class == Array && s.Struct != nil:
			// An array of records or of a derived type, whole.
			return a.wholeArray(a.lookup(x.Name, x.Pos), x)

		case s.Struct != nil || s.Class == Constant && s.Value != nil && s.Value.T.Kind == Record ||
			s.Class == Constant && s.ArrayValue != nil && s.ArrayValue.T.Kind == Record:
			return a.name(x, true)
		}

	case *ast.Apply:
		return a.apply(x)

	case *ast.FieldRef:
		return a.field(x, false)

	case *ast.Paren:
		return a.operand(x.X)
	}

	return a.arrayValue(e)
}

// aggregateOperands refuses an operation op, at pos, that an interface block
// didn't take, on a record or a derived-type object as a whole: a derived type
// has no intrinsic operators (NODERIVEDOP), and a record is never a value
// (AGGREGATE, for each such operand, as before derived types). It returns nil
// if no operand is one.
func (a *analyzer) aggregateOperands(op string, pos diag.Position, operands ...Expr) Expr {
	found := false

	for _, x := range operands {
		t := x.Type()
		if t.Kind != Record {
			continue
		}

		if t.Struct != nil && t.Struct.Derived {
			a.report("NODERIVEDOP", pos, op, t.Struct.Display())

			return a.bad(pos)
		}

		a.report("AGGREGATE", x.Position(), aggregateName(x))

		found = true
	}

	if found {
		return a.bad(pos)
	}

	return nil
}

// recordInitialValue gives sym, an object of a derived type declared with = a
// value, that value, which must be a constant of its type: a scalar's, or
// every element's of an array (the standard's 5.1). Its components' bytes
// become Initials when the record initials are (recordInitials).
func (a *analyzer) recordInitialValue(sym *Symbol, d *ast.Declarator) {
	ev := a.declEval()

	if ev.isArrayForm(d.Value) {
		// TYPE(POINT) :: R(2) = (/ POINT(1.0, 2.0), ORIGIN /): an element's
		// value each, of the array's shape.
		arr, err := ev.evalArray(d.Value)
		if err != nil {
			if err.kind != constFailed {
				a.report("DATAVALUE", err.pos, err.why)
			}

			return
		}

		if arr.T.Kind != Record || !sameType(arr.T.Struct, sym.Struct) {
			a.report("INITTYPE", d.ValuePos, article(arr.T.String()), article(sym.Type.String()))

			return
		}

		if !a.initialShape(sym, arr, d.ValuePos) {
			return
		}

		a.initialized = append(a.initialized, sym)
		sym.recordValues = arr.Elems

		return
	}

	c, err := ev.eval(d.Value)
	if err != nil {
		if err.kind != constFailed {
			a.report("DATAVALUE", err.pos, err.why)
		}

		return
	}

	if c.T.Kind != Record || !sameType(c.T.Struct, sym.Struct) {
		a.report("INITTYPE", d.ValuePos, article(c.T.String()), article(sym.Type.String()))

		return
	}

	a.initialized = append(a.initialized, sym)
	sym.recordValue = c
}

// valueInitials is the constant c of a derived type as initial values, a
// value per scalar component (an array component's per element) at its place
// in the value, each item named as Fortran 90 writes it, X%Y(2).
func (a *analyzer) valueInitials(c *Const, pos diag.Position) []*FieldInitial {
	var list []*FieldInitial

	st := c.T.Struct

	for i, f := range st.Fields {
		switch part := c.Parts[i].(type) {
		case *Const:
			list = append(list, a.partInitials(part, f.Offset, f.Name, pos)...)

		case *ArrayConst:
			for n, e := range part.Elems {
				item := f.Name + "(" + elementSubscripts(f.Dims, n) + ")"
				list = append(list, a.partInitials(e, f.Offset+n*f.ElementSize(), item, pos)...)
			}
		}
	}

	return list
}

// partInitials is one component's value, part, at offset in its record, named
// item: a scalar's one value, or a derived type's components'.
func (a *analyzer) partInitials(part *Const, offset int, item string, pos diag.Position) []*FieldInitial {
	if part.T.Kind != Record {
		return []*FieldInitial{{Offset: offset, Type: part.T, Bytes: a.constBytes(part), Item: item, Source: DumpExpr(part), Pos: pos}}
	}

	inner := a.valueInitials(part, pos)
	for _, in := range inner {
		in.Offset += offset
		in.Item = item + "%" + in.Item
	}

	return inner
}

// inPlace reports whether the assignment target = value may build value, a
// structure constructor's, straight in target: none of its values reads what
// target names, P = POINT(P%Y, P%X) being the case that may not.
func inPlace(target, value Expr) bool {
	if _, ok := value.(*StructValue); !ok {
		return false
	}

	var ts *Symbol

	switch t := target.(type) {
	case *Var:
		ts = t.Sym
	case *Elem:
		ts = t.Sym
	case *FieldRef:
		ts = t.Sym
	default:
		return false
	}

	return !readsAny(value, ts)
}
