package sema

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Arrays (sections 2.2.3, 4.5, 6.1.1). An array declarator gives a name
// one to seven dimensions, each with a lower and an upper bound, [dl:]du.
// The bounds come in three kinds:
//
//   - constant: DIMENSION A(10), B(0:9, -1:1), C(2*N) where N is a
//     PARAMETER constant. The array's size is known, and if it isn't a
//     dummy argument it gets storage of that size.
//   - adjustable: SUBROUTINE S(A, M, N) with DIMENSION A(M, N). The bound
//     is an expression of constants, dummy arguments, and variables in
//     common blocks, computed when the subprogram is entered. Only a dummy
//     array may have one, since only a dummy array has its storage from
//     somewhere else.
//   - assumed-size: DIMENSION A(M, *). The last upper bound is *: the
//     array is as long as the caller's. The other bounds are found as they
//     would be for any array, since each element's place depends on every
//     dimension but the last.
//
// Elements are stored in column-major order (section 2.2.3.3): the first
// subscript varies fastest. In A(l1:u1, l2:u2, l3:u3), with n1 = u1-l1+1
// and n2 = u2-l2+1, the element A(s1, s2, s3) is number
//
//	(s1 - l1) + (s2 - l2)*n1 + (s3 - l3)*n1*n2
//
// counting from 0. So in DIMENSION BOS(3,3,3), BOS(2,1,3) is number
// 1 + 0 + 2*9 = 19, the 20th, as the manual's Figure 2-1 draws it.
// ElementNumber is this sum for constant subscripts, which EQUIVALENCE
// and DATA need; lowering (ir's mem) makes the same sum into code for
// the others. The last dimension's extent is never in the sum, which is
// why an assumed-size array's can be unknown.

// maxDimensions is the most dimensions an array can have (section 2.2.3).
const maxDimensions = 7

// maxArrayBytes is the largest array: a VAX address reaches no further,
// and a class A descriptor's size wouldn't hold it as a positive number.
const maxArrayBytes = 1<<31 - 1

// adjustableArray is an array declared with bounds that aren't constant,
// to be checked when every declaration has been read (checkArrays). Its
// names' declarations may follow it: DIMENSION A(N) may come before
// COMMON N, and N's type may be declared after it, if that doesn't change
// the type.
type adjustableArray struct {
	sym *Symbol

	// names are the names in those bounds, as they are written; inquired are
	// those an inquiry function asks about (SIZE(A), LEN(S)), which may be
	// arrays.
	names    []*ast.Name
	inquired map[*ast.Name]bool

	// components are the names whose components the bounds read, D in D%N
	// (the Fortran 90 standard's 7.1.6.2: a subobject of a dummy argument):
	// a record, whose component is checked for its type once it is analyzed.
	components map[*ast.Name]bool

	// length is set when the expression is sym's CHARACTER length, not a
	// bound (charlength.go): the same rules, other words in the messages.
	length bool
}

// where is what the messages call the place a name is in: an array's bounds,
// or a CHARACTER length; whereOne is a bound alone.
func (adj *adjustableArray) where() string {
	if adj.length {
		return "a CHARACTER length"
	}

	return "an array's bounds"
}

func (adj *adjustableArray) whereOne() string {
	if adj.length {
		return "a CHARACTER length"
	}

	return "an array's bound"
}

// automaticAllowed reports whether the unit may have automatic arrays: a
// function or subroutine may, a main program or block data subprogram may not
// (the Fortran 90 standard's 5.1.2.4.1).
func (a *analyzer) automaticAllowed() bool {
	return a.unit.Kind == ast.FunctionUnit || a.unit.Kind == ast.SubroutineUnit
}

// boundInquiries are the inquiry functions a bound may use, of an array's
// bounds or a string's length (the Fortran 90 standard's 7.1.6.2(8)); their
// first argument may be an array. boundElementals are the elemental intrinsic
// functions of integer arguments it may use (7.1.6.2(7)).
var (
	boundInquiries  = map[string]bool{"SIZE": true, "LBOUND": true, "UBOUND": true, "LEN": true}
	boundElementals = map[string]bool{
		"ABS": true, "IABS": true, "MAX": true, "MIN": true, "MAX0": true, "MIN0": true,
		"MOD": true, "INT": true, "NINT": true, "IFIX": true, "IDINT": true, "IDNINT": true,
		"DIM": true, "IDIM": true, "SIGN": true, "ISIGN": true,
	}
)

// dimension gives sym the dimensions its array declarator d declares
// (sections 2.2.3.1, 4.5). Whatever goes wrong with its bounds, sym is an
// array, so its uses are elements; it is broken, so they get no messages.
func (a *analyzer) dimension(sym *Symbol, d *ast.Declarator) {
	switch {
	case sym.Use != nil:
		// USEATTR, reported by declare.
		return

	// A function's value may be an array (arrayvalue.go); its name with RESULT
	// may not, being the routine's alone.
	case sym.Class == UnitName || sym.Class == Constant || sym.External || sym.Class == Intrinsic:
		a.badUse(sym, d.Pos, "be an array")

		sym.broken = true

		return

	case sym.Dims != nil:
		a.report("DUPDIM", d.Pos, d.Name, sym.DimPos.Line)

		return
	}

	dims := make([]Dim, len(d.Dims))

	sym.Class, sym.DimPos = Array, d.Pos
	sym.Dims = dims

	if len(d.Dims) > maxDimensions {
		a.report("TOOMANYDIMS", d.Pos, len(d.Dims))

		sym.broken = true

		return
	}

	adj := &adjustableArray{sym: sym}

	for i, b := range d.Dims {
		dim, ok := a.dimensionBounds(adj, b, i == len(d.Dims)-1, d.Pos)
		if !ok {
			sym.broken = true

			return
		}

		dims[i] = dim
	}

	if adj.names != nil {
		a.adjustable = append(a.adjustable, adj)
	}
}

// dimensionBounds is one dimension's bounds.
func (a *analyzer) dimensionBounds(adj *adjustableArray, b ast.Bound, last bool, pos diag.Position) (Dim, bool) {
	dim := Dim{Lower: 1}

	var ok bool

	if b.Colon {
		// (:), Fortran 90's deferred shape: an allocatable array's, or a
		// dummy's assumed shape, which checkShapes settles once every
		// declaration is read, ALLOCATABLE among them. (lb:) is only an
		// assumed shape's, whose lower bound is a dummy's specification
		// expression (5.1.2.4.2), read as an adjustable array's is.
		shape := Dim{Deferred: true, Lower: 1, lowerGiven: b.Lower != nil}

		if b.Lower != nil && adj.sym.Dummy != 0 {
			shape.Lower, shape.LowerExpr, ok = a.bound(adj, b.Lower)
			if !ok {
				return dim, false
			}
		}

		return shape, true
	}

	if b.Lower != nil {
		dim.Lower, dim.LowerExpr, ok = a.bound(adj, b.Lower)
		if !ok {
			return dim, false
		}
	}

	if b.Star {
		if adj.sym.Dummy == 0 || !last {
			a.report("ASSUMEDSIZE", pos)

			return dim, false
		}

		dim.Assumed = true

		return dim, true
	}

	dim.Upper, dim.UpperExpr, ok = a.bound(adj, b.Upper)
	if !ok {
		return dim, false
	}

	if dim.LowerExpr == nil && dim.UpperExpr == nil && dim.Upper < dim.Lower {
		a.report("BOUNDORDER", b.Upper.Position(), dim.Upper, dim.Lower)

		return dim, false
	}

	return dim, true
}

// bound is an array bound's value if it is constant, converted to INTEGER
// (section 2.2.3.1: DIMENSION A(2.5) has two elements), and its expression
// if it isn't, for an adjustable array: then its names go on adj.
func (a *analyzer) bound(adj *adjustableArray, e ast.Expr) (int, ast.Expr, bool) {
	ev := a.declEval()

	c, err := ev.eval(e)

	switch {
	case err == nil:
		v, ok := a.constantBound(ev, c, e.Position())

		return v, nil, ok

	case err.kind == constFailed:
		return 0, nil, false

	case adj.sym.Dummy == 0 && !a.automaticAllowed():
		a.report("ADJNOTDUMMY", e.Position(), adj.sym.Name)

		return 0, nil, false

	case adj.sym.Dummy == 0:
		// A subprogram's own array: automatic (finding 7). Its checks wait
		// until every declaration is read (checkAutomatics).
		adj.sym.AutomaticObject = true
	}

	before := len(adj.names)

	if !a.boundOperands(adj, e) {
		return 0, nil, false
	}

	if len(adj.names) == before {
		// Not constant, for a reason that isn't a name: 2.0 ** 0.5.
		a.report("BADBOUND", e.Position(), adj.whereOne())

		return 0, nil, false
	}

	return 0, e, true
}

// constantBound is the INTEGER value of a constant bound of value c.
func (a *analyzer) constantBound(ev *evaluator, c *Const, pos diag.Position) (int, bool) {
	if c.T.Kind != Integer && c.T.Kind != Real {
		a.report("BADBOUND", pos, "an array's bound")

		return 0, false
	}

	c, err := ev.convert(c, IntegerType, pos)
	if err != nil {
		if err.kind != constFailed {
			a.report("BADBOUND", pos, "an array's bound")
		}

		return 0, false
	}

	return int(c.Int), true
}

// boundOperands checks the form of a bound that isn't constant: it is
// made of the arithmetic operators, constants, and names (section
// 2.2.3.1), and "array references and references to user-defined
// functions" are not allowed. The names go on adj for checkArrays, once
// it is known what each is, and on boundVars for checkBoundType.
func (a *analyzer) boundOperands(adj *adjustableArray, e ast.Expr) bool {
	if c, err := (&evaluator{a: a, quiet: true}).eval(e); err == nil {
		if c.T.Kind != Integer && c.T.Kind != Real {
			a.report("BADBOUND", e.Position(), adj.whereOne())

			return false
		}

		return true
	}

	switch e := e.(type) {
	case *ast.Paren:
		return a.boundOperands(adj, e.X)

	case *ast.Unary:
		if e.Op == ast.Add || e.Op == ast.Sub {
			return a.boundOperands(adj, e.X)
		}

	case *ast.Binary:
		switch e.Op {
		case ast.Add, ast.Sub, ast.Mul, ast.Div, ast.Pow:
			// Both, to find what's wrong with each.
			x := a.boundOperands(adj, e.X)
			y := a.boundOperands(adj, e.Y)

			return x && y
		}

	case *ast.Name:
		adj.names = append(adj.names, e)

		s := a.lookup(e.Name, e.Pos)
		if _, seen := a.boundVars[s]; !seen {
			a.boundVars[s] = e.Pos
		}

		return true

	case *ast.Apply:
		return a.boundFunction(adj, e)

	case *ast.FieldRef:
		return a.boundComponent(adj, e)
	}

	a.report("BADBOUND", e.Position(), adj.whereOne())

	return false
}

// boundComponent checks a component in a bound that isn't constant, D%N or
// D%P%N(2): its base is a name, checked as a bound's names are, and any
// subscript along the way is a bound's expression itself. Its type is
// checked once it is analyzed (boundValues).
func (a *analyzer) boundComponent(adj *adjustableArray, e *ast.FieldRef) bool {
	ok := true

	var x ast.Expr = e

	for {
		f, more := x.(*ast.FieldRef)
		if !more {
			break
		}

		for _, sub := range f.Args {
			ok = a.boundOperands(adj, sub) && ok
		}

		x = f.X
	}

	n, named := x.(*ast.Name)
	if !named {
		a.report("BADBOUND", e.Position(), adj.whereOne())

		return false
	}

	adj.names = append(adj.names, n)

	if adj.components == nil {
		adj.components = map[*ast.Name]bool{}
	}

	adj.components[n] = true

	return ok
}

// boundFunction checks a function reference in a bound that isn't constant: an
// intrinsic function the unit hasn't made something else, one of the inquiry
// functions or elemental functions of integers a specification expression may use
// (7.1.6.2). An inquiry's first argument is a name, which may be an array's.
func (a *analyzer) boundFunction(adj *adjustableArray, e *ast.Apply) bool {
	s := a.scope.find(e.Name)
	free := s == nil || (s.Class == Unclassified && s.Dummy == 0 && !s.External) || s.Class == Intrinsic

	switch {
	case !free || !a.intrinsicNamed(e.Name) || len(e.Args) == 0:
	case boundInquiries[e.Name]:
		n, ok := e.Args[0].(*ast.Name)
		if !ok {
			break
		}

		adj.names = append(adj.names, n)

		if adj.inquired == nil {
			adj.inquired = map[*ast.Name]bool{}
		}

		adj.inquired[n] = true

		ok = true

		for _, arg := range e.Args[1:] {
			if k, keyed := arg.(*ast.KeywordArg); keyed {
				arg = k.X
			}

			ok = a.boundOperands(adj, arg) && ok
		}

		return ok

	case boundElementals[e.Name]:
		ok := true

		for _, arg := range e.Args {
			if k, keyed := arg.(*ast.KeywordArg); keyed {
				arg = k.X
			}

			ok = a.boundOperands(adj, arg) && ok
		}

		return ok
	}

	a.report("BOUNDREF", e.Pos, e.Name, adj.where())

	return false
}

// checkBoundType reports a type statement giving t to a name already used
// in an array's bounds, if that changes its type (section 2.2.3.1;
// section 6.1.1.1's invalid example has DIMENSION A(X) with INTEGER X).
func (a *analyzer) checkBoundType(s *Symbol, t Type, ok bool, pos diag.Position) {
	if first, used := a.boundVars[s]; used && ok && t != s.Type {
		a.report("BOUNDTYPE", pos, s.Name, first.Line)
	}
}

// checkArrays checks what depends on every declaration, once they have
// been read: each array's size, whose element type may be declared after
// its dimensions, and the names in each adjustable array's bounds, which
// are then analyzed, since now each name's type is known.
func (a *analyzer) checkArrays() {
	for _, s := range a.unit.Symbols {
		if s.Class == Array && !s.broken && tooLarge(s) {
			a.report("ARRAYSIZE", s.DimPos, s.Name)

			s.broken = true
		}
	}

	a.checkShapes()
	a.checkArrayValue()

	for _, adj := range a.adjustable {
		ok := true

		for _, n := range adj.names {
			ok = a.checkBoundName(adj, n) && ok
		}

		if ok {
			a.boundValues(adj)
		} else {
			adj.sym.broken = true
		}
	}

	a.checkEntryValues()
}

// boundValues analyzes an adjustable array's bounds into expressions of type
// INTEGER (section 2.2.3.1: a bound's value is converted to INTEGER), and notes
// the dummy arguments they use.
func (a *analyzer) boundValues(adj *adjustableArray) {
	dims := adj.sym.Dims

	if adj.length {
		adj.sym.LengthValue = a.boundValue(adj, adj.sym.LengthExpr)
		dims = nil
	}

	for i := range dims {
		if e := dims[i].LowerExpr; e != nil {
			dims[i].LowerValue = a.boundValue(adj, e)
		}

		if e := dims[i].UpperExpr; e != nil {
			dims[i].UpperValue = a.boundValue(adj, e)
		}
	}

	// Only the unit's own: an internal procedure reads a host's dummy through
	// its host's cell, whichever way it was called (ir/lowerinternal.go).
	for _, n := range adj.names {
		if s := a.scope.find(n.Name); s.Dummy != 0 && !a.hostName(s.Name) && !slices.Contains(adj.sym.BoundDummies, s) {
			adj.sym.BoundDummies = append(adj.sym.BoundDummies, s)
		}
	}
}

// boundValue is one bound's value, converted to INTEGER. A component the
// bound reads (boundComponent) is first known to be numeric here, its
// structure's declarations having been read: BADBOUND if it isn't.
func (a *analyzer) boundValue(adj *adjustableArray, e ast.Expr) Expr {
	v := a.value(e)

	if len(adj.components) != 0 {
		if k := v.Type().Kind; k != Integer && k != Real && k != Invalid {
			a.report("BADBOUND", e.Position(), adj.whereOne())

			adj.sym.broken = true
		}
	}

	return a.convert(v, IntegerType)
}

// checkBoundName checks a name in an adjustable array's bound: a dummy
// argument or a variable in a common block (section 6.1.1.1), or in an
// internal procedure its host's variable (the Fortran 90 standard's 7.1.6.2), of
// a type that converts to INTEGER. A name an internal procedure's declarations
// made its own because its host hadn't used it yet waits for its host's
// statements (checkProvisional).
//
// An inquiry function's argument may be an array, or a CHARACTER name of any
// length (inquired), but not an allocatable or automatic one, whose bounds aren't
// those of the unit's entry.
func (a *analyzer) checkBoundName(adj *adjustableArray, n *ast.Name) bool {
	s := a.scope.find(n.Name)
	inquired, where := adj.inquired[n], adj.where()
	component := adj.components[n]

	switch {
	case s.broken:
		return false

	case a.interfacing && slices.Contains(a.provisional, s):
		a.provisionalBounds = append(a.provisionalBounds, provisionalBound{n, where})

		return false

	case inquired && (s.Allocatable || s.AutomaticObject || s.ArrayPointer()):
		a.report("BOUNDREF", n.Pos, n.Name, where)

		return false

	case inquired && s.Dummy == 0 && s.Common == nil && !a.hostName(s.Name):
		a.report("BOUNDVAR", n.Pos, n.Name, where)

		return false

	case inquired:
		return s.Class == Array || s.Class == Variable || s.Class == Unclassified ||
			a.badBoundInquiry(n, where)

	case component && (s.Dummy == 0 && s.Common == nil && !a.hostName(s.Name)):
		a.report("BOUNDVAR", n.Pos, n.Name, where)

		return false

	case component:
		// Its component's type is boundValue's to check.
		return true

	case s.Class == Array || s.Class == Function || s.Class == StmtFunction:
		a.report("BOUNDREF", n.Pos, n.Name, where)

		return false

	case s.Dummy == 0 && s.Common == nil && !a.hostName(s.Name):
		// A constant among them, defined after the bound that uses it.
		a.report("BOUNDVAR", n.Pos, n.Name, where)

		return false

	case s.Type.Kind == Invalid:
		// No type (IMPLICIT NONE): NOTYPE says so.
		return true

	case s.Type.Kind != Integer && s.Type.Kind != Real:
		a.report("BADBOUND", n.Pos, adj.whereOne())

		return false
	}

	return true
}

// tooLarge reports whether s's known dimensions alone take more than
// maxArrayBytes. It stops as soon as they do, so that the product of seven
// large extents never overflows.
func tooLarge(s *Symbol) bool {
	n := s.Type.Size
	if n == 0 {
		// CHARACTER*(*): at least a byte each.
		n = 1
	}

	for _, d := range s.Dims {
		if !d.known() {
			continue
		}

		if d.Extent() > maxArrayBytes/n {
			return true
		}

		n *= d.Extent()
	}

	return false
}

// ElementNumber is the place in storage order, from 0, of the element of
// s whose subscripts are the constants subs (the sum above), or false if
// the element isn't in the array: a subscript outside its bounds, or the
// wrong number of them. An assumed-size array's last upper bound is
// unknown, so any subscript at or above its lower bound will do. An
// adjustable array's elements have no constant place. EQUIVALENCE and DATA
// use this for their constant subscripts.
func (s *Symbol) ElementNumber(subs []int) (int, bool) {
	if len(subs) != len(s.Dims) {
		return 0, false
	}

	// multiplier is n1*n2*... of the dimensions before this one: how many
	// elements one step in this subscript skips over. It starts at 1,
	// since a step in the first subscript is the next element.
	number, multiplier := 0, 1

	for i, d := range s.Dims {
		switch {
		case d.LowerExpr != nil || d.UpperExpr != nil || d.Deferred || d.AssumedShape:
			return 0, false
		case subs[i] < d.Lower:
			return 0, false
		case !d.Assumed && subs[i] > d.Upper:
			return 0, false
		}

		number += (subs[i] - d.Lower) * multiplier
		multiplier *= d.Extent()
	}

	return number, true
}

// badBoundInquiry reports an inquiry function's argument that is neither a
// variable nor an array, and returns false.
func (a *analyzer) badBoundInquiry(n *ast.Name, where string) bool {
	a.report("BOUNDREF", n.Pos, n.Name, where)

	return false
}

// checkAutomatics reports the automatic arrays something else would give
// storage of their own (AUTOATTR): a common block, an EQUIVALENCE list, SAVE,
// or STATIC (the Fortran 90 standard's 5.1.2.4.1, 5.2.4, 5.5.1); DATA's and
// NAMELIST's are reported where those are read. A SAVE with no list saves only
// what may be saved, which an automatic array may not.
func (a *analyzer) checkAutomatics() {
	for _, s := range a.unit.Symbols {
		if !s.AutomaticObject || s.broken {
			continue
		}

		switch {
		case s.Common != nil:
			a.autoAttr(s, s.autoPos(), s.Name, "be in a common block")
		case s.Equivalenced:
			a.autoAttr(s, s.equivPos, s.Name, "be in an EQUIVALENCE list")
		case s.saved:
			a.autoAttr(s, s.autoPos(), s.Name, "be saved")
		case s.storageStmt != nil && s.storageStmt.Kind == lex.Static:
			a.autoAttr(s, s.autoPos(), s.Name, "be STATIC")
		default:
			continue
		}

		s.broken = true
	}
}
