package sema

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The intrinsic functions a constant expression may use (section 4.11):
// MIN, MAX, ABS, MOD, ICHAR, NINT, DIM, DPROD, CMPLX, CONJG, and IMAG in
// arithmetic expressions; IAND, IOR, NOT, IEOR, ISHFT, LGE, LGT, LLE, and
// LLT in logical ones; CHAR in character ones. They are the generic
// names, whose result's type follows their arguments'. Section 4.11 says
// IMAG where appendix D's table has AIMAG; both are taken here.

// constFunc is one of them: how many arguments it takes (max -1 for any
// number), as INTRINARGS says it, and its value.
type constFunc struct {
	min, max int
	takes    string
	eval     func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr)
}

// constFuncs are the functions, by name.
//
// Several entries share code through function makers: minMax, bitwise, and
// lexical each return a new function (a closure) with one detail fixed,
// MIN's or MAX's direction, the operator & or | or ^, the comparison. So
// IAND's evaluator is bitwise called with a little function that ANDs two
// integers, and the shared part (checking the arguments are INTEGER, making
// the result) is written once.
var constFuncs = map[string]constFunc{
	"MIN":   {2, -1, "2 or more arguments", minMax(-1)},
	"MAX":   {2, -1, "2 or more arguments", minMax(1)},
	"ABS":   {1, 1, "1 argument", constAbs},
	"MOD":   {2, 2, "2 arguments", constMod},
	"DIM":   {2, 2, "2 arguments", constDim},
	"ICHAR": {1, 1, "1 argument", constIchar},
	"NINT":  {1, 1, "1 argument", constNint},
	"INT":   {1, 1, "1 argument", constInt},
	"DPROD": {2, 2, "2 arguments", constDprod},
	"CMPLX": {1, 2, "1 or 2 arguments", constCmplx},
	"CONJG": {1, 1, "1 argument", constConjg},
	"IMAG":  {1, 1, "1 argument", constImag},
	"AIMAG": {1, 1, "1 argument", constImag},
	"IAND":  {2, 2, "2 arguments", bitwise(func(x, y int32) int32 { return x & y })},
	"IOR":   {2, 2, "2 arguments", bitwise(func(x, y int32) int32 { return x | y })},
	"IEOR":  {2, 2, "2 arguments", bitwise(func(x, y int32) int32 { return x ^ y })},
	"NOT":   {1, 1, "1 argument", constBitNot},
	"ISHFT": {2, 2, "2 arguments", constIshft},
	"LGE":   {2, 2, "2 arguments", lexical(func(o int) bool { return o >= 0 })},
	"LGT":   {2, 2, "2 arguments", lexical(func(o int) bool { return o > 0 })},
	"LLE":   {2, 2, "2 arguments", lexical(func(o int) bool { return o <= 0 })},
	"LLT":   {2, 2, "2 arguments", lexical(func(o int) bool { return o < 0 })},
	"CHAR":  {1, 1, "1 argument", constChar},

	// Fortran 90's, in its initialization expressions (the standard's
	// 7.1.6.1), whose names are intrinsic only under /F90 (apply). INDEX is
	// VAX FORTRAN's too, but not in its constant expressions (section 4.11).
	"ACHAR":    {1, 1, "1 argument", constChar},
	"IACHAR":   {1, 1, "1 argument", constIchar},
	"LEN_TRIM": {1, 1, "1 argument", textFunction(func(s string, pos diag.Position) *Const { return IntConstAt(lenTrim(s), pos) })},
	"TRIM":     {1, 1, "1 argument", textFunction(func(s string, pos diag.Position) *Const { return charConst(s[:lenTrim(s)], pos) })},
	"ADJUSTL":  {1, 1, "1 argument", textFunction(func(s string, pos diag.Position) *Const { return charConst(adjustLeft(s), pos) })},
	"ADJUSTR":  {1, 1, "1 argument", textFunction(func(s string, pos diag.Position) *Const { return charConst(adjustRight(s), pos) })},
	"REPEAT":   {2, 2, "2 arguments", constRepeat},
	"SCAN":     {2, 3, "2 or 3 arguments", searching(scanText)},
	"VERIFY":   {2, 3, "2 or 3 arguments", searching(verifyText)},
	"INDEX":    {2, 3, "2 or 3 arguments", searching(indexText)},
	"CEILING":  {1, 1, "1 argument", wholeFunction(true)},
	"FLOOR":    {1, 1, "1 argument", wholeFunction(false)},
	"MODULO":   {2, 2, "2 arguments", constModulo},

	"EXPONENT":     {1, 1, "1 argument", constModel},
	"FRACTION":     {1, 1, "1 argument", constModel},
	"RRSPACING":    {1, 1, "1 argument", constModel},
	"SPACING":      {1, 1, "1 argument", constModel},
	"NEAREST":      {2, 2, "2 arguments", constModel},
	"SCALE":        {2, 2, "2 arguments", constModel},
	"SET_EXPONENT": {2, 2, "2 arguments", constModel},
}

// apply is a function reference in a constant expression: one of
// constFuncs, of constant arguments. An array element, or any other
// function, isn't constant.
func (ev *evaluator) apply(e *ast.Apply) (*Const, *constErr) {
	// A(...) is a function reference only if A isn't something else the
	// unit declared: an array (then it's an element), a variable, a
	// dummy argument. A name only mentioned so far (Unclassified) may
	// still be an intrinsic function's. An array constant's element is a
	// constant (constarray.go).
	s := ev.a.scope.find(e.Name)

	// POINT(1.0, 2.0), a structure constructor (structvalue.go).
	if st := ev.a.constructorType(e.Name, s); st != nil {
		return ev.structConst(st, e)
	}

	if s != nil && s.ArrayValue != nil {
		c, _, err := ev.constantSection(s, e)

		return c, err
	}

	if s != nil && s.Class != Intrinsic && (s.Class != Unclassified || s.Dummy != 0) {
		if s.Class == Array {
			return nil, named("the array element "+e.Name+"(...) is not a constant", e.Pos)
		}

		return nil, named(e.Name+" is "+s.Class.String()+", not a constant", e.Pos)
	}

	if !ev.a.intrinsicNamed(e.Name) && f90Intrinsics[e.Name] {
		// Fortran 90's name, not intrinsic under /NOF90 without an INTRINSIC
		// statement: the program's own function.
		return nil, named(e.Name+" is a function, not a constant", e.Pos)
	}

	if e.Name == "TRANSFER" && ev.a.intrinsicNamed(e.Name) {
		// A scalar TRANSFER (isArrayForm said so).
		ev.a.noteIntrinsic(e.Name, e.Pos)

		c, _, err := ev.transferConst(e)

		return c, err
	}

	if arrayFunctions[e.Name] && ev.a.intrinsicNamed(e.Name) {
		// SIZE, and LBOUND and UBOUND with DIM=: RESHAPE, SHAPE, and the
		// others are arrays (isArrayForm).
		ev.a.noteIntrinsic(e.Name, e.Pos)

		return ev.boundsScalar(e)
	}

	forms := formsNamed(e.Name)

	if inq, selected := isInquiry(e.Name); inq || selected {
		return ev.inquiry(e, forms)
	}

	fn, ok := constFuncs[e.Name]
	if !ok {
		if intrinsics[e.Name] {
			return nil, notConst("the intrinsic function "+e.Name+" can't be computed when compiling", e.Pos)
		}

		return nil, named(e.Name+" is a function, not a constant", e.Pos)
	}

	ev.a.noteIntrinsic(e.Name, e.Pos)

	// The arguments in the dummies' order, with KIND taken out
	// (intrinsickeys.go); IMAG, not the table's, has none.
	slots := intrinsicArgs{args: e.Args}
	if len(forms) > 0 {
		if slots, ok = ev.a.intrinsicSlots(e, forms, ev.quiet); !ok {
			return nil, failed
		}
	}

	if n := len(slots.args); n < fn.min || fn.max >= 0 && n > fn.max {
		return nil, ev.report("INTRINARGS", e.Pos, e.Name, fn.takes, n)
	}

	args := make([]*Const, len(slots.args))

	var err *constErr

	for i, arg := range slots.args {
		if arg == nil {
			return nil, ev.report("NULLINTRINARG", e.Pos, e.Name)
		}

		var argErr *constErr

		args[i], argErr = ev.eval(arg)
		err = worse(err, argErr)
	}

	if err != nil {
		return nil, err
	}

	c, err := fn.eval(ev, e, args)
	if err == nil {
		ev.initRule(e, args, c)
	}

	if err != nil || slots.kind == nil {
		return c, err
	}

	return ev.withKind(c, slots.kind)
}

// initRule is /STANDARD's judgment of an intrinsic function computed in a constant
// expression, of value c. FORTRAN-77's constant expressions have no function
// references at all (its 6.7). Fortran 90's initialization expressions have an elemental
// function only of type INTEGER or CHARACTER, each argument INTEGER or CHARACTER, and of
// the others REPEAT, TRIM, and the inquiry and SELECTED_ functions (its 7.1.6.1): so
// CEILING(2.5) or MAX(1.0, X) there is in neither standard. govax computes every
// function in constFuncs, as VAX FORTRAN computed its own, so it's a remark, never an
// error. KIND= isn't among args, and an INTEGER kind is no matter.
func (ev *evaluator) initRule(e *ast.Apply, args []*Const, c *Const) {
	if ev.quiet || !ev.init {
		return
	}

	textual := func(t Type) bool {
		return t.Kind == Integer || t.Kind == Character
	}

	x := ev.a.extensions

	if !textual(c.T) {
		x.add(e.Pos, "the intrinsic function "+e.Name+" of type "+c.T.String()+" in a constant expression", 0)

		return
	}

	for _, arg := range args {
		if !textual(arg.T) {
			x.add(arg.Pos, article(arg.T.String())+" argument to "+e.Name+" in a constant expression", 0)

			return
		}
	}

	x.add(e.Pos, "the intrinsic function "+e.Name+" in a constant expression", f90)
}

// withKind is c, a function's value, converted to the kind of its type that
// the reference's KIND= asks for (intrinsickeys.go): NINT(X, KIND=2), CMPLX(X,
// Y, 8).
func (ev *evaluator) withKind(c *Const, kind ast.Expr) (*Const, *constErr) {
	k, err := ev.eval(kind)

	switch {
	case err != nil && err.kind == constFailed:
		return nil, err
	case err != nil, k.T.Kind != Integer, k.Int < 0:
		return nil, ev.report("KINDNOTCONST", kind.Position())
	}

	t, problem := kindType(baseTypes[c.T.Kind], int(k.Int))
	if problem != nil {
		if !ev.quiet {
			ev.a.reportKind(problem, kind.Position())
		}

		return nil, failed
	}

	return ev.convert(c, t, c.Pos)
}

// badArg reports an argument of a type or value the function doesn't
// take (BADARG): want is what it takes, and the argument is shown by its
// type.
func (ev *evaluator) badArg(e *ast.Apply, want string, arg *Const) *constErr {
	return ev.report("BADARG", arg.Pos, e.Name, want, arg.T.String())
}

// alike converts arguments of the kinds allowed to one type, the highest
// of theirs. Every argument must be of one kind, the first's (a generic
// function's arguments must agree, as VAX FORTRAN's do).
func (ev *evaluator) alike(e *ast.Apply, args []*Const, kinds ...Kind) ([]*Const, *constErr) {
	allowed := false

	for _, k := range kinds {
		allowed = allowed || args[0].T.Kind == k
	}

	if !allowed {
		want := ""

		for i, k := range kinds {
			switch {
			case i == 0:
			case i == len(kinds)-1:
				want += " or "
			default:
				want += ", "
			}

			want += Type{Kind: k, Size: 4}.String()
		}

		return nil, ev.badArg(e, want, args[0])
	}

	t := args[0].T

	for _, arg := range args[1:] {
		if arg.T.Kind != t.Kind {
			return nil, ev.badArg(e, t.String()+", like the first", arg)
		}

		t = higher(t, arg.T)
	}

	out := make([]*Const, len(args))

	for i, arg := range args {
		c, err := ev.convert(arg, t, arg.Pos)
		if err != nil {
			return nil, err
		}

		out[i] = c
	}

	return out, nil
}

// minMax is MIN (sign -1) or MAX (1) of INTEGER or REAL arguments.
func minMax(sign int) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		args, err := ev.alike(e, args, Integer, Real)
		if err != nil {
			return nil, err
		}

		// order is -1, 0, or 1 as arg is less than, equal to, or greater
		// than best; multiplying by sign (1 for MAX, -1 for MIN) makes
		// "greater than 0" mean "better" for both.
		best := args[0]

		for _, arg := range args[1:] {
			if order(arg, best)*sign > 0 {
				best = arg
			}
		}

		return best.at(e.Pos), nil
	}
}

// constAbs is ABS: an INTEGER's or REAL's magnitude, or a COMPLEX's
// modulus, the square root of the sum of its parts' squares, a REAL of
// the parts' type.
func constAbs(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	c := &Const{exprBase: exprBase{T: x.T, Pos: e.Pos}}

	switch x.T.Kind {
	case Integer:
		v := int64(x.Int)
		if v < 0 {
			v = -v
		}

		if _, hi := x.T.intRange(); v > hi {
			return nil, ev.intRange(e.Pos, valueText, x.T)
		}

		c.Int = int32(v)

	case Real:
		c.Real = x.Real.Abs()

	case Complex:
		t := x.T.part()
		sum := vaxfloat.AddExact(vaxfloat.MulExact(x.Real, x.Real), vaxfloat.MulExact(x.Imag, x.Imag))

		v, err := vaxfloat.Sqrt(ev.a.format(t), sum)
		if err != nil {
			return nil, ev.realRange(e.Pos, valueText, t)
		}

		c.T, c.Real = t, v

	default:
		return nil, ev.badArg(e, "INTEGER, REAL, or COMPLEX", x)
	}

	return c, nil
}

// constMod is MOD(a, b), a - INT(a/b)*b: the remainder of a division
// truncated toward zero, with a's sign. A REAL remainder is computed
// exactly and rounded once.
func constMod(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	args, err := ev.alike(e, args, Integer, Real)
	if err != nil {
		return nil, err
	}

	x, y := args[0], args[1]
	c := &Const{exprBase: exprBase{T: x.T, Pos: e.Pos}}

	if x.T.Kind == Integer {
		if y.Int == 0 {
			return nil, ev.report("DIVZERO", e.Pos)
		}

		// Go's % has the dividend's sign, as MOD does.
		c.Int = int32(int64(x.Int) % int64(y.Int))

		return c, nil
	}

	if y.Real.IsZero() {
		return nil, ev.report("DIVZERO", e.Pos)
	}

	q := new(big.Rat).Quo(x.Real.Rat(), y.Real.Rat())
	whole := new(big.Int).Quo(q.Num(), q.Denom()) // truncated toward zero
	rem := new(big.Rat).Sub(x.Real.Rat(), new(big.Rat).Mul(new(big.Rat).SetInt(whole), y.Real.Rat()))

	v, rerr := round(ev.a.format(x.T), vaxfloat.FromRat(rem))
	if rerr != nil {
		return nil, ev.realRange(e.Pos, valueText, x.T)
	}

	c.Real = v

	return c, nil
}

// wholeFunction makes CEILING (up) or FLOOR: the least integer not less
// than a REAL, or the greatest not greater, of the unit's default integer
// type (Fortran 90's 13.13.17 and 13.13.38).
func wholeFunction(up bool) func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		x := args[0]
		if x.T.Kind != Real {
			return nil, ev.badArg(e, "REAL", x)
		}

		t := ev.a.defaultInt
		lo, hi := t.intRange()

		i := floorOf(x.Real.Rat())
		if up && !x.Real.Rat().IsInt() {
			i.Add(i, big.NewInt(1))
		}

		if !i.IsInt64() || i.Int64() < lo || i.Int64() > hi {
			return nil, ev.intRange(e.Pos, valueText, t)
		}

		return &Const{exprBase: exprBase{T: t, Pos: e.Pos}, Int: int32(i.Int64())}, nil
	}
}

// floorOf is the greatest integer not greater than r. big.Int's Div is
// Euclidean division, whose quotient for a positive divisor (a Rat's
// denominator is always positive) is the floor.
func floorOf(r *big.Rat) *big.Int {
	return new(big.Int).Div(r.Num(), r.Denom())
}

// constModulo is MODULO(a, p), a - FLOOR(a/p)*p (Fortran 90's 13.13.68): the
// remainder with p's sign, where MOD's has a's. A REAL one is computed
// exactly and rounded once.
func constModulo(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	args, err := ev.alike(e, args, Integer, Real)
	if err != nil {
		return nil, err
	}

	x, y := args[0], args[1]
	c := &Const{exprBase: exprBase{T: x.T, Pos: e.Pos}}

	if x.T.Kind == Integer {
		if y.Int == 0 {
			return nil, ev.report("DIVZERO", e.Pos)
		}

		r := int64(x.Int) % int64(y.Int)
		if r != 0 && (r < 0) != (y.Int < 0) {
			r += int64(y.Int)
		}

		c.Int = int32(r)

		return c, nil
	}

	if y.Real.IsZero() {
		return nil, ev.report("DIVZERO", e.Pos)
	}

	whole := floorOf(new(big.Rat).Quo(x.Real.Rat(), y.Real.Rat()))
	rem := new(big.Rat).Sub(x.Real.Rat(), new(big.Rat).Mul(new(big.Rat).SetInt(whole), y.Real.Rat()))

	v, rerr := round(ev.a.format(x.T), vaxfloat.FromRat(rem))
	if rerr != nil {
		return nil, ev.realRange(e.Pos, valueText, x.T)
	}

	c.Real = v

	return c, nil
}

// constDim is DIM(a, b): a - b if a is greater, else zero.
func constDim(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	args, err := ev.alike(e, args, Integer, Real)
	if err != nil {
		return nil, err
	}

	if order(args[0], args[1]) <= 0 {
		return &Const{exprBase: exprBase{T: args[0].T, Pos: e.Pos}}, nil
	}

	return ev.operate(ast.Sub, args[0], args[1], e.Pos)
}

// constIchar is ICHAR(c): the ASCII code of a CHARACTER*1, an INTEGER.
func constIchar(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Character || len(x.Text) != 1 {
		return nil, ev.badArg(e, "CHARACTER*1", x)
	}

	return &Const{exprBase: exprBase{T: IntegerType, Pos: e.Pos}, Int: int32(x.Text[0])}, nil
}

// constNint is NINT(r): a REAL rounded to the nearest INTEGER, halves
// away from zero, of the unit's default integer type (footnote 10 of Table
// D-3).
func constNint(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Real {
		return nil, ev.badArg(e, "REAL", x)
	}

	t := ev.a.defaultInt
	lo, hi := t.intRange()

	i := x.Real.Int(true)
	if !i.IsInt64() || i.Int64() < lo || i.Int64() > hi {
		return nil, ev.intRange(e.Pos, valueText, t)
	}

	return &Const{exprBase: exprBase{T: t, Pos: e.Pos}, Int: int32(i.Int64())}, nil
}

// constInt is INT(a): an INTEGER as it is, a REAL's or a COMPLEX's real
// part's whole part, toward zero, of the unit's default integer type
// (footnote 10). Section 4.11 hasn't INT, but Fortran 90's initialization
// expressions have it of an INTEGER (the standard's 7.1.6.1), where KIND=
// gives it a use: INT(7, KIND=2). A REAL's is taken too, as NINT's is
// (docs/planning/PHASE-100 - Fortran 90 declarations and kinds.md, subtask 6).
func constInt(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	t := ev.a.defaultInt
	lo, hi := t.intRange()

	var v int64

	switch x.T.Kind {
	case Integer:
		v = int64(x.Int)
	case Real, Complex:
		i := x.Real.Int(false)
		if !i.IsInt64() {
			return nil, ev.intRange(e.Pos, valueText, t)
		}

		v = i.Int64()
	default:
		return nil, ev.badArg(e, "INTEGER, REAL, or COMPLEX", x)
	}

	if v < lo || v > hi {
		return nil, ev.intRange(e.Pos, valueText, t)
	}

	return &Const{exprBase: exprBase{T: t, Pos: e.Pos}, Int: int32(v)}, nil
}

// constDprod is DPROD(x, y): the REAL*8 product of two REAL*4 values,
// which is exact.
func constDprod(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	for _, arg := range args {
		if arg.T != RealType {
			return nil, ev.badArg(e, "REAL*4", arg)
		}
	}

	t := Type{Kind: Real, Size: 8}

	v, err := round(ev.a.format(t), vaxfloat.MulExact(args[0].Real, args[1].Real))
	if err != nil {
		return nil, ev.realRange(e.Pos, valueText, t)
	}

	return &Const{exprBase: exprBase{T: t, Pos: e.Pos}, Real: v}, nil
}

// constCmplx is CMPLX(x) or CMPLX(x, y): a COMPLEX*8 of one number (a
// COMPLEX's value, or a real part with an imaginary part of zero) or of
// two that aren't COMPLEX.
func constCmplx(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	t := Type{Kind: Complex, Size: 8}

	for _, arg := range args {
		if !arg.T.numeric() || len(args) == 2 && arg.T.Kind == Complex {
			want := "INTEGER, REAL, or COMPLEX"
			if len(args) == 2 {
				want = "INTEGER or REAL"
			}

			return nil, ev.badArg(e, want, arg)
		}
	}

	if len(args) == 1 {
		c, err := ev.convert(args[0], t, e.Pos)
		if err != nil {
			return nil, err
		}

		return c.at(e.Pos), nil
	}

	re, err := ev.convert(args[0], RealType, e.Pos)
	if err != nil {
		return nil, err
	}

	im, err := ev.convert(args[1], RealType, e.Pos)
	if err != nil {
		return nil, err
	}

	return &Const{exprBase: exprBase{T: t, Pos: e.Pos}, Real: re.Real, Imag: im.Real}, nil
}

// constConjg is CONJG(z): z with its imaginary part negated.
func constConjg(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Complex {
		return nil, ev.badArg(e, "COMPLEX", x)
	}

	return &Const{exprBase: exprBase{T: x.T, Pos: e.Pos}, Real: x.Real, Imag: x.Imag.Neg()}, nil
}

// constImag is IMAG(z), AIMAG(z): z's imaginary part, a REAL of the
// parts' type.
func constImag(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Complex {
		return nil, ev.badArg(e, "COMPLEX", x)
	}

	return &Const{exprBase: exprBase{T: x.T.part(), Pos: e.Pos}, Real: x.Imag}, nil
}

// bitwise is IAND, IOR, or IEOR of two INTEGERs, op bit by bit.
func bitwise(op func(x, y int32) int32) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		args, err := ev.alike(e, args, Integer)
		if err != nil {
			return nil, err
		}

		return &Const{exprBase: exprBase{T: args[0].T, Pos: e.Pos}, Int: op(args[0].Int, args[1].Int)}, nil
	}
}

// constBitNot is NOT(i): every bit of an INTEGER complemented.
func constBitNot(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Integer {
		return nil, ev.badArg(e, "INTEGER", x)
	}

	return &Const{exprBase: exprBase{T: x.T, Pos: e.Pos}, Int: ^x.Int}, nil
}

// constIshft is ISHFT(i, n): i's bits shifted left n places, or right -n
// places, zeros shifted in at either end (a logical shift); a shift of
// the whole width or more leaves zero.
func constIshft(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x, n := args[0], args[1]

	for _, arg := range args {
		if arg.T.Kind != Integer {
			return nil, ev.badArg(e, "INTEGER", arg)
		}
	}

	width := int32(x.T.Size * 8)
	mask := uint32(1)<<width - 1

	if width == 32 {
		mask = ^uint32(0)
	}

	bits := uint32(x.Int) & mask

	switch shift := n.Int; {
	case shift >= width || shift <= -width:
		bits = 0
	case shift >= 0:
		bits <<= uint(shift)
	default:
		bits >>= uint(-shift)
	}

	return &Const{exprBase: exprBase{T: x.T, Pos: e.Pos}, Int: truncate(int32(bits&mask), x.T.Size)}, nil
}

// lexical is LGE, LGT, LLE, or LLT: two CHARACTER values compared in
// ASCII, the shorter padded with blanks.
func lexical(holds func(order int) bool) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		for _, arg := range args {
			if arg.T.Kind != Character {
				return nil, ev.badArg(e, "CHARACTER", arg)
			}
		}

		return logicalConst(holds(compareText(args[0].Text, args[1].Text)), e.Pos), nil
	}
}

// constChar is CHAR(i): the CHARACTER*1 whose ASCII code is i, 0 to 255.
func constChar(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]

	switch {
	case x.T.Kind != Integer:
		return nil, ev.badArg(e, "INTEGER", x)
	case x.Int < 0 || x.Int > 255:
		return nil, ev.report("BADARG", x.Pos, e.Name, "INTEGER values from 0 to 255", strconv.Itoa(int(x.Int)))
	}

	return charConst(string([]byte{byte(x.Int)}), e.Pos), nil
}

// textFunction is a function of one CHARACTER argument, its value f's of the
// argument's text.
func textFunction(f func(string, diag.Position) *Const) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		if args[0].T.Kind != Character {
			return nil, ev.badArg(e, "CHARACTER", args[0])
		}

		return f(args[0].Text, e.Pos), nil
	}
}

// searching is SCAN, VERIFY, or INDEX of constants: f's position, with BACK
// (a LOGICAL) .FALSE. when it is left out.
func searching(f func(s, set string, back bool) int) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		if e.Name == "INDEX" && !ev.a.f90 {
			return nil, notConst("the intrinsic function INDEX can't be computed when compiling", e.Pos)
		}

		for _, x := range args[:2] {
			if x.T.Kind != Character {
				return nil, ev.badArg(e, "CHARACTER", x)
			}
		}

		back := false
		if len(args) == 3 {
			if args[2].T.Kind != Logical {
				return nil, ev.badArg(e, "LOGICAL", args[2])
			}

			back = args[2].Truth()
		}

		return IntConstAt(f(args[0].Text, args[1].Text, back), e.Pos), nil
	}
}

// constRepeat is REPEAT(s, n): n copies of s, n an INTEGER of 0 or more, the
// value no longer than 65535 characters.
func constRepeat(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	s, n := args[0], args[1]

	switch {
	case s.T.Kind != Character:
		return nil, ev.badArg(e, "CHARACTER", s)
	case n.T.Kind != Integer:
		return nil, ev.badArg(e, "INTEGER", n)
	case n.Int < 0:
		return nil, ev.report("BADARG", n.Pos, e.Name, "an NCOPIES of 0 or more", strconv.Itoa(int(n.Int)))
	case int64(len(s.Text))*int64(n.Int) > maxCharLength:
		return nil, ev.report("BADARG", e.Pos, e.Name, "a value of 65535 characters or fewer", "a longer one")
	}

	return charConst(strings.Repeat(s.Text, int(n.Int)), e.Pos), nil
}

// constModel is one of the numeric model's functions of constants (model.go),
// computed by vaxfloat's model of X's format: exact, so the value is the one
// FTNRTL's routine or the in-line code computes. A result past the format's
// range is RANGE's message, as any constant's.
func constModel(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Real {
		return nil, ev.badArg(e, "a REAL X", x)
	}

	f := ev.a.format(x.T)
	c := &Const{exprBase: exprBase{T: x.T, Pos: e.Pos}}

	var (
		v   vaxfloat.Value
		err error
	)

	switch e.Name {
	case "EXPONENT":
		return &Const{exprBase: exprBase{T: ev.a.defaultInt, Pos: e.Pos}, Int: int32(vaxfloat.Exponent(x.Real))}, nil
	case "FRACTION":
		v = vaxfloat.Fraction(x.Real)
	case "RRSPACING":
		v = f.RRSpacing(x.Real)
	case "SPACING":
		v = f.Spacing(x.Real)
	case "NEAREST":
		s := args[1]
		if s.T.Kind != Real {
			return nil, ev.badArg(e, "a REAL S", s)
		}

		v, err = f.Nearest(x.Real, s.Real.Sign() >= 0)
	default:
		i := args[1]
		if i.T.Kind != Integer {
			return nil, ev.badArg(e, "an INTEGER I", i)
		}

		if e.Name == "SCALE" {
			v, err = f.Scale(x.Real, int64(i.Int))
		} else {
			v, err = f.SetExponent(x.Real, int64(i.Int))
		}
	}

	if err != nil {
		return nil, ev.realRange(e.Pos, valueText, x.T)
	}

	c.Real = v

	return c, nil
}
