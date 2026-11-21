package sema

import (
	"math"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Fortran 90's kind and numeric inquiry functions (the standard's 13.7.1,
// 13.13; docs/planning/PHASE-100 - Fortran 90 declarations and kinds.md,
// subtask 6, and its Part A's table), folded when compiling.
//
// An inquiry function asks about its argument's type, never its value:
// HUGE(X) is the largest number of X's type, whatever X holds, and X needn't
// have a value at all. So the argument is analyzed only for its type
// (inquiryArgType), and the answer is a constant, which is why KIND(1.0D0)
// may name a kind in a declaration. Each VAX format is exactly the
// standard's model of a real number (vaxfloat's model.go), and each integer
// type the model of an integer with 8, 16, or 32 bits less the sign's, so
// the answers are exact. A REAL*8's are D_floating's or G_floating's by the
// unit's /G_FLOATING, as its arithmetic is.
//
// SELECTED_INT_KIND(R) and SELECTED_REAL_KIND(P, R) choose a kind from the
// same model: the kind of least range (or precision) that meets the request.
// Their arguments must be constant here; a kind chosen when the program runs
// is of no use to a declaration, and of little to anything else (NOTIMPL).

// inquiry is one inquiry function: the kinds of type its argument may have,
// as BADARG names them, and its answer for an argument of type t.
type inquiry struct {
	kinds  []Kind
	want   string
	answer func(a *analyzer, t Type, pos diag.Position) *Const
}

// inquiries are the inquiry functions, by name.
var inquiries = map[string]inquiry{
	"KIND": {[]Kind{Integer, Real, Complex, Logical, Character}, "INTEGER, REAL, COMPLEX, LOGICAL, or CHARACTER",
		func(a *analyzer, t Type, pos diag.Position) *Const { return a.intAnswer(int64(t.KindNumber()), pos) }},
	"HUGE": {[]Kind{Integer, Real}, "INTEGER or REAL", func(a *analyzer, t Type, pos diag.Position) *Const {
		if t.Kind == Integer {
			return intConst(t, int64(1)<<intDigits(t)-1, pos)
		}

		return realConst(t, a.format(t).Huge(), pos)
	}},
	"TINY": {[]Kind{Real}, "REAL", func(a *analyzer, t Type, pos diag.Position) *Const {
		return realConst(t, a.format(t).Tiny(), pos)
	}},
	"EPSILON": {[]Kind{Real}, "REAL", func(a *analyzer, t Type, pos diag.Position) *Const {
		return realConst(t, a.format(t).Epsilon(), pos)
	}},
	"DIGITS": {[]Kind{Integer, Real}, "INTEGER or REAL", func(a *analyzer, t Type, pos diag.Position) *Const {
		if t.Kind == Integer {
			return a.intAnswer(int64(intDigits(t)), pos)
		}

		return a.intAnswer(int64(a.format(t).Digits()), pos)
	}},
	"PRECISION": {[]Kind{Real, Complex}, "REAL or COMPLEX", func(a *analyzer, t Type, pos diag.Position) *Const {
		return a.intAnswer(int64(realPrecision(a.format(t))), pos)
	}},
	"RANGE": {[]Kind{Integer, Real, Complex}, "INTEGER, REAL, or COMPLEX", func(a *analyzer, t Type, pos diag.Position) *Const {
		if t.Kind == Integer {
			return a.intAnswer(int64(intRangeDigits(t)), pos)
		}

		return a.intAnswer(int64(modelRange(a.format(t))), pos)
	}},
	"RADIX": {[]Kind{Integer, Real}, "INTEGER or REAL", func(a *analyzer, _ Type, pos diag.Position) *Const {
		return a.intAnswer(2, pos)
	}},
	"MAXEXPONENT": {[]Kind{Real}, "REAL", func(a *analyzer, t Type, pos diag.Position) *Const {
		return a.intAnswer(int64(a.format(t).MaxExponent()), pos)
	}},
	"MINEXPONENT": {[]Kind{Real}, "REAL", func(a *analyzer, t Type, pos diag.Position) *Const {
		return a.intAnswer(int64(a.format(t).MinExponent()), pos)
	}},
	"BIT_SIZE": {[]Kind{Integer}, "INTEGER", func(_ *analyzer, t Type, pos diag.Position) *Const {
		// Of the argument's own kind (13.13.16).
		return intConst(t, int64(8*t.Size), pos)
	}},
}

// intDigits is the integer model's q for t: its bits less the sign's.
func intDigits(t Type) int { return 8*t.Size - 1 }

// intRangeDigits is RANGE of an integer type, INT(LOG10(HUGE)): the digits
// of its largest value less one, 2, 4, or 9.
func intRangeDigits(t Type) int {
	return len(strconv.FormatInt(int64(1)<<intDigits(t)-1, 10)) - 1
}

// realPrecision is PRECISION of a real format, INT((p - 1) × LOG10(2)): 6 for
// F, 16 for D, 15 for G, 33 for H. No format's product is near a whole
// number, so float64's logarithm is exact enough.
func realPrecision(f vaxfloat.Format) int {
	return int(float64(f.Digits()-1) * math.Log10(2))
}

// modelRange is RANGE of a real format, INT(MIN(LOG10(HUGE), -LOG10(TINY))):
// 38 for F and D, 307 for G, 4931 for H. LOG10(HUGE) is emax × LOG10(2) less
// a little (HUGE is 2**emax less a part in 2**p), and -LOG10(TINY) is
// (1 - emin) × LOG10(2); neither is near a whole number for any format.
func modelRange(f vaxfloat.Format) int {
	huge := float64(f.MaxExponent()) * math.Log10(2)
	tiny := float64(1-f.MinExponent()) * math.Log10(2)

	return int(math.Min(huge, tiny))
}

// intAnswer is an inquiry's answer of the default INTEGER type.
func (a *analyzer) intAnswer(v int64, pos diag.Position) *Const {
	return intConst(a.defaultInt, v, pos)
}

// intConst is the constant v of the INTEGER type t.
func intConst(t Type, v int64, pos diag.Position) *Const {
	return &Const{exprBase: exprBase{T: t, Pos: pos}, Int: int32(v)}
}

// realConst is the constant v of the REAL type t.
func realConst(t Type, v vaxfloat.Value, pos diag.Position) *Const {
	return &Const{exprBase: exprBase{T: t, Pos: pos}, Real: v}
}

// isInquiry reports whether the table's row for name is answered when
// compiling from its argument's type, or (selected) a SELECTED_ function's.
func isInquiry(name string) (inq, selected bool) {
	forms := formsNamed(name)
	if len(forms) == 0 {
		return false, false
	}

	return forms[0].how == "INQ", forms[0].how == "SIK" || forms[0].how == "SRK"
}

// inquire is the inquiry function name's answer for an argument of type t at
// argPos, or BADARG (reported unless quiet) for a type it doesn't take.
func (a *analyzer) inquire(name string, t Type, pos, argPos diag.Position, quiet bool) (*Const, bool) {
	q := inquiries[name]

	for _, k := range q.kinds {
		if t.Kind == k {
			return q.answer(a, t, pos), true
		}
	}

	if !quiet && t.Kind != Invalid {
		a.report("BADARG", argPos, name, q.want, t.String())
	}

	return nil, false
}

// inquiryArgType is the type an inquiry function's argument has, analyzed as
// a reference whose value isn't used: a whole array's elements' type (HUGE(A)
// of a REAL array A is REAL's), a function reference's result type. An
// INTEGER or LOGICAL constant written without a kind is of the default
// kind, so KIND(0) is 2 under /NOI4.
func (a *analyzer) inquiryArgType(x ast.Expr) Type {
	if t, ok := a.literalType(x); ok {
		return t
	}

	var e Expr

	switch x := a.substringForm(x).(type) {
	case *ast.Name:
		e = a.name(x, true)
	case *ast.Apply:
		e = a.apply(x)
	case *ast.FieldRef:
		e = a.field(x, true)
	default:
		return a.value(x).Type()
	}

	a.noteInquired(e)

	return e.Type()
}

// literalType is the type of an INTEGER or LOGICAL constant written without a
// kind, parentheses aside: the unit's default INTEGER or LOGICAL.
func (a *analyzer) literalType(x ast.Expr) (Type, bool) {
	switch x := x.(type) {
	case *ast.Paren:
		return a.literalType(x.X)
	case *ast.IntConst:
		return a.defaultInt, true
	case *ast.LogicalConst:
		return a.defaultLogical, true
	}

	return invalid, false
}

// inquiryArgType is the evaluator's: the type of a name it doesn't enter (a
// declaration may ask HUGE(X) before X is used anywhere else, and entering X
// would change where it is stored), as declared or as its first letter makes
// it; and a constant's type otherwise.
func (ev *evaluator) inquiryArgType(x ast.Expr) (Type, *constErr) {
	if t, ok := ev.a.literalType(x); ok {
		return t, nil
	}

	name := ""

	switch x := x.(type) {
	case *ast.Name:
		name = x.Name
	case *ast.Apply:
		if s := ev.a.scope.find(x.Name); s != nil && s.Class == Array {
			name = x.Name
		}
	}

	if name != "" {
		s := ev.a.scope.find(name)

		switch {
		case s != nil && s.Class != Constant && s.Type.Kind != Invalid:
			return s.Type, nil
		case s == nil:
			if t, untyped := ev.a.scope.rules().typeOf(name); !untyped {
				return t, nil
			}

			return invalid, named(name+" has no type", x.Position())
		}
	}

	c, err := ev.eval(x)
	if err != nil {
		return invalid, err
	}

	return c.T, nil
}

// inquiry is an inquiry or SELECTED_ function's reference in a constant
// expression.
func (ev *evaluator) inquiry(e *ast.Apply, forms []form) (*Const, *constErr) {
	ev.a.noteIntrinsic(e.Name, e.Pos)

	slots, ok := ev.a.intrinsicSlots(e, forms, ev.quiet)
	if !ok {
		return nil, failed
	}

	if forms[0].how != "INQ" {
		return ev.selectedKind(e, forms[0].how, slots.args)
	}

	t, err := ev.inquiryArgType(slots.args[0])
	if err != nil {
		return nil, err
	}

	c, ok := ev.a.inquire(e.Name, t, e.Pos, slots.args[0].Position(), ev.quiet)
	if !ok {
		return nil, failed
	}

	return c, nil
}

// selectedKind is SELECTED_INT_KIND(R) (how SIK) or SELECTED_REAL_KIND(P, R)
// (SRK) of constant INTEGER arguments.
func (ev *evaluator) selectedKind(e *ast.Apply, how string, args []ast.Expr) (*Const, *constErr) {
	values := make([]*int64, len(args))

	for i, x := range args {
		if x == nil {
			continue
		}

		c, err := ev.eval(x)
		if err != nil {
			return nil, err
		}

		if c.T.Kind != Integer {
			return nil, ev.report("BADARG", x.Position(), e.Name, "INTEGER", c.T.String())
		}

		v := int64(c.Int)
		values[i] = &v
	}

	if how == "SIK" {
		return ev.a.intAnswer(int64(selectedIntKind(*values[0])), e.Pos), nil
	}

	if len(values) == 0 {
		return nil, ev.report("MISSINGARG", e.Pos, "the intrinsic function "+e.Name, "P or R")
	}

	var p, r *int64

	p = values[0]
	if len(values) > 1 {
		r = values[1]
	}

	return ev.a.intAnswer(int64(ev.a.selectedRealKind(p, r)), e.Pos), nil
}

// selectedIntKind is SELECTED_INT_KIND(r): the kind of least range holding
// every integer of r digits, or -1 if none does.
func selectedIntKind(r int64) int {
	for _, k := range []int{1, 2, 4} {
		if int64(intRangeDigits(Type{Kind: Integer, Size: k})) >= r {
			return k
		}
	}

	return -1
}

// selectedRealKind is SELECTED_REAL_KIND(p, r), either left out (nil): the
// kind of least precision with at least p decimal digits and a range of at
// least r, the unit's REAL*8 D or G; else -1 if no kind has the precision, -2
// if none has the range, -3 if neither (13.13.93). Under /NOG_FLOATING a
// range past 38 needs H, kind 16, though G would have held it (the plan's
// open question 2).
func (a *analyzer) selectedRealKind(p, r *int64) int {
	precision, rng := false, false

	for _, k := range []int{4, 8, 16} {
		f := a.format(Type{Kind: Real, Size: k})
		okP := p == nil || int64(realPrecision(f)) >= *p
		okR := r == nil || int64(modelRange(f)) >= *r

		if okP && okR {
			return k
		}

		precision, rng = precision || okP, rng || okR
	}

	switch {
	case !precision && !rng:
		return -3
	case !precision:
		return -1
	}

	return -2
}

// selectedValue is SELECTED_INT_KIND(R) (how SIK) or SELECTED_REAL_KIND(P, R)
// (SRK) of values not constant (docs/planning/PHASE-108 - Fortran 90
// intrinsic procedures.md, finding 5): the same choice as selectedKind's, made
// when the program runs by a chain of comparisons, written as MERGEs of
// constants, which lowering writes in line:
//
//	SELECTED_INT_KIND(R) = MERGE(1, MERGE(2, MERGE(4, -1, R <= 9), R <= 4), R <= 2)
//
// SELECTED_REAL_KIND's kinds are the unit's REAL*8 D or G, as when compiling.
// Each argument is read as many times as the chain compares it, so one with a
// side effect (a function's reference) has it more than once.
func (a *analyzer) selectedValue(e *ast.Apply, how string, list []ast.Expr) Expr {
	values := make([]Expr, len(list))

	for i, x := range list {
		if x == nil {
			continue
		}

		v := a.value(x)

		switch t := v.Type(); {
		case t.Kind == Invalid:
			return a.bad(e.Pos)
		case t.Kind != Integer:
			a.report("BADARG", x.Position(), e.Name, "INTEGER", t.String())

			return a.bad(e.Pos)
		}

		values[i] = a.convert(v, IntegerType)
	}

	pos := e.Pos
	answer := func(k int) Expr { return a.intAnswer(int64(k), pos) }

	// atMost is x <= n, or nil (always true) for an argument left out.
	atMost := func(x Expr, n int) Expr {
		if x == nil {
			return nil
		}

		return &Compare{exprBase{T: LogicalType, Pos: pos}, ast.LE, x, IntConstAt(n, pos)}
	}

	both := func(x, y Expr) Expr {
		switch {
		case x == nil:
			return y
		case y == nil:
			return x
		}

		return &Binary{exprBase{T: LogicalType, Pos: pos}, ast.And, x, y}
	}

	not := func(x Expr) Expr { return &Unary{exprBase{T: LogicalType, Pos: pos}, ast.Not, x} }

	merge := func(yes, no, cond Expr) Expr {
		if cond == nil {
			return yes
		}

		return &IntrinsicRef{exprBase{T: a.defaultInt, Pos: pos}, "MERGE", MergeOp, []Expr{yes, no, cond}, ""}
	}

	if how == "SIK" {
		v := answer(-1)

		for _, k := range []int{4, 2, 1} {
			v = merge(answer(k), v, atMost(values[0], intRangeDigits(Type{Kind: Integer, Size: k})))
		}

		return v
	}

	var p, r Expr

	switch len(values) {
	case 0:
		a.report("MISSINGARG", pos, "the intrinsic function "+e.Name, "P or R")

		return a.bad(pos)
	case 2:
		r = values[1]
	}

	p = values[0]

	// What none has: precision (-1), range (-2), or both (-3).
	maxP, maxR := 0, 0
	for _, k := range []int{4, 8, 16} {
		f := a.format(Type{Kind: Real, Size: k})
		maxP, maxR = max(maxP, realPrecision(f)), max(maxR, modelRange(f))
	}

	someP, someR := atMost(p, maxP), atMost(r, maxR)

	var v Expr = answer(-2)
	if someP != nil {
		v = merge(answer(-1), v, not(someP))
	}

	if someP != nil && someR != nil {
		v = merge(answer(-3), v, both(not(someP), not(someR)))
	}

	for _, k := range []int{16, 8, 4} {
		f := a.format(Type{Kind: Real, Size: k})
		v = merge(answer(k), v, both(atMost(p, realPrecision(f)), atMost(r, modelRange(f))))
	}

	return v
}
