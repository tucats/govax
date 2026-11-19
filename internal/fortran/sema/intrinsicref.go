package sema

import (
	"math"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// A reference to an intrinsic function (section 6.3), by the table in
// intrinsics.go: its arguments are counted and analyzed, their type selects
// the table's row (a generic name's by the arguments, section 6.3.1.1), each
// is converted to the type the row takes, and the row's last column says
// what the reference becomes.

// intrinsicRef analyzes a reference to the intrinsic function e.Name.
//
// FORTRAN's intrinsic functions come in two sorts of name. A specific name
// works on one type: DSQRT takes and returns a DOUBLE PRECISION. A generic
// name stands for a family, and the arguments choose among it: SQRT of a
// REAL is SQRT, of a DOUBLE PRECISION is DSQRT, of a COMPLEX is CSQRT. This
// is overloading resolved at compile time, much as a Go generic function's
// type is chosen by its arguments. So the arguments are analyzed first, and
// their types then select the table's row (selectForm).
func (a *analyzer) intrinsicRef(e *ast.Apply) Expr {
	a.noteIntrinsic(e.Name, e.Pos)

	forms := formsNamed(e.Name)

	// The arguments in the dummies' order, with KIND taken out
	// (intrinsickeys.go).
	slots, ok := a.intrinsicSlots(e, forms, false)
	if !ok {
		return a.bad(e.Pos)
	}

	if hasNull(slots.args) && dummiesOf(e.Name, forms) == nil {
		a.report("NULLINTRINARG", e.Pos, e.Name)

		return a.bad(e.Pos)
	}

	if !argCountFits(forms, len(slots.args)) {
		a.report("INTRINARGS", e.Pos, e.Name, takes(forms), len(slots.args))

		return a.bad(e.Pos)
	}

	switch inq, selected := isInquiry(e.Name); {
	case inq:
		// Answered from the argument's type (inquiry.go).
		t := a.inquiryArgType(slots.args[0])
		if c, ok := a.inquire(e.Name, t, e.Pos, slots.args[0].Position(), false); ok {
			return c
		}

		return a.bad(e.Pos)

	case selected:
		// A constant's was folded (apply); this is a value's.
		a.notImplForm(e.Pos, e.Name+" of a value not constant")

		return a.bad(e.Pos)
	}

	if e.Name == "SIZEOF" {
		return a.sizeOf(e)
	}

	switch forms[0].how {
	case "ARRAY":
		// One whose array's bounds are constants was folded (apply); RESHAPE
		// of values not constant is computed when the program runs.
		if e.Name == "RESHAPE" {
			return a.transform(e, slots.args)
		}

		return a.arrayFunction(e, slots.args)

	case "REDUCE":
		if transformational(e.Name) {
			return a.transform(e, slots.args)
		}

		return a.arrayFunction(e, slots.args)
	}

	array := false

	args := make([]Expr, len(slots.args))
	for i, x := range slots.args {
		// MERGE's sources may be of any type, a derived type's too.
		if e.Name == "MERGE" && i < 2 {
			args[i] = a.operand(x)
		} else {
			args[i] = a.arrayValue(x)
		}

		if args[i].Type().Kind == Invalid {
			return a.bad(e.Pos)
		}

		array = array || rankOf(args[i]) > 0

		if args[i].Type() == complex32 && !takesComplex32(forms) {
			// Only the conversions and parts have rows for one (complex32).
			a.notImplForm(e.Pos, complex32Intrinsic)

			return a.bad(e.Pos)
		}
	}

	// An elemental function of arrays is an array of their shape, a scalar
	// argument standing for each element (Fortran 90's 13.2.1).
	var shape Shape

	if array {
		if shape, ok = a.conforms(e.Pos, args...); !ok {
			return a.bad(e.Pos)
		}
	}

	if e.Name == "MERGE" {
		return a.merge(e, args, shape)
	}

	// KIND= makes the result that kind of the result's type.
	want := invalid
	if slots.kind != nil {
		if want, ok = a.resultKind(slots.kind, forms[0].result); !ok {
			return a.bad(e.Pos)
		}
	}

	f, ok := a.selectForm(e, forms, args, want)
	if !ok {
		return a.bad(e.Pos)
	}

	if want.Kind != Invalid && (f.how == "CVT" || f.how == "CMPLX") {
		// A conversion converts to the kind asked for at once.
		f.result = want
	}

	if array && f.class != 'E' {
		// LEN of a CHARACTER array whose length is known is that length;
		// of a whole array, a section, or a function's value whose length
		// isn't, its elements' length, which the IR knows where to find
		// (charLen): a function that makes its value is called for it.
		if f.how == "LEN" && args[0].Type().Size > 0 {
			return &Const{exprBase: exprBase{T: f.result, Pos: e.Pos}, Int: int32(args[0].Type().Size)}
		}

		switch args[0].(type) {
		case *ArrayRef, *Section, *FuncRef:
			if f.how == "LEN" {
				return &IntrinsicRef{exprBase: exprBase{T: f.result, Pos: e.Pos}, Name: e.Name, Op: LenOp, Args: args[:1]}
			}
		}

		a.notImplForm(e.Pos, "an array argument to the intrinsic function "+e.Name)

		return a.bad(e.Pos)
	}

	v := a.intrinsicValue(e, f, a.intrinsicArgs(f, args))

	switch {
	case v.Type().Kind == Invalid:
	case want.Kind != Invalid:
		v = a.convert(v, want)
	case a.byteResult(e.Name, forms, f, args):
		v = a.convert(v, byteType)
	}

	if shape != nil {
		giveShape(v, shape)
	}

	return v
}

// giveShape gives each node of e, an elemental function's value made of
// array arguments, that has an array below it the shape s.
func giveShape(e Expr, s Shape) bool {
	array := rankOf(e) > 0

	switch e := e.(type) {
	case *IntrinsicRef:
		for _, x := range e.Args {
			array = giveShape(x, s) || array
		}

		if array && e.S == nil {
			e.S = s
		}

	case *Binary:
		array = giveShape(e.X, s) || array
		array = giveShape(e.Y, s) || array

		if array && e.S == nil {
			e.S = s
		}

	case *Compare:
		array = giveShape(e.X, s) || array
		array = giveShape(e.Y, s) || array

		if array && e.S == nil {
			e.S = s
		}

	case *Unary:
		array = giveShape(e.X, s) || array

		if array && e.S == nil {
			e.S = s
		}

	case *Convert:
		array = giveShape(e.X, s) || array

		if array && e.S == nil {
			e.S = s
		}
	}

	return array
}

// merge is MERGE(TSOURCE, FSOURCE, MASK) (Fortran 90's 13.13.67): TSOURCE
// where MASK is true, FSOURCE where it is false, elemental. TSOURCE and
// FSOURCE have one type and length (MERGEARGS), MASK is LOGICAL; the value is
// TSOURCE's type.
func (a *analyzer) merge(e *ast.Apply, args []Expr, shape Shape) Expr {
	ts, fs, mask := args[0], args[1], args[2]

	switch tt, ft := ts.Type(), fs.Type(); {
	case tt.Kind == Record && ft.Kind == Record && sameType(tt.Struct, ft.Struct):
		// Objects of one derived type (4.4.2), or records of one structure.

	case tt != ft:
		a.report("MERGEARGS", e.Pos, tt.String(), ft.String())

		return a.bad(e.Pos)

	case mask.Type().Kind != Logical:
		a.report("BADARG", mask.Position(), "MERGE", "a LOGICAL MASK", mask.Type().String())

		return a.bad(e.Pos)

	case ts.Type() == complex32:
		a.notImplForm(e.Pos, complex32Intrinsic)

		return a.bad(e.Pos)
	}

	return &IntrinsicRef{exprBase{T: ts.Type(), Pos: e.Pos, S: shape}, "MERGE", MergeOp, args, ""}
}

// takesComplex32 reports whether any of the rows takes a COMPLEX*32.
func takesComplex32(forms []form) bool {
	for _, f := range forms {
		if f.arg == complex32 {
			return true
		}
	}

	return false
}

// byteType is INTEGER*1, Fortran 90's INTEGER(1).
var byteType = Type{Kind: Integer, Size: 1}

// byteResult reports whether a generic function's INTEGER*1 arguments give
// an INTEGER*1 result. The table has no INTEGER*1 rows (VAX FORTRAN has no
// such type), so such arguments select the INTEGER*2 row (selectForm) and
// are converted to it; the standard's 13.13 makes ABS, MOD, MAX, IAND, and
// the rest of a kind's arguments of that kind, so their result is converted
// back. A function whose INTEGER*2 rows give other results too, INT by the
// unit's default integer, keeps the row's, as does a specific name.
func (a *analyzer) byteResult(name string, forms []form, f form, args []Expr) bool {
	if f.arg != (Type{Kind: Integer, Size: 2}) || f.result != f.arg || name != f.generic {
		return false
	}

	byteArgs := false

	for _, x := range args {
		byteArgs = byteArgs || x.Type() == byteType
	}

	if !byteArgs {
		return false
	}

	for _, g := range forms {
		if g.arg == f.arg && g.result != f.result {
			return false
		}
	}

	return true
}

// argCountFits reports whether some row takes n arguments.
func argCountFits(forms []form, n int) bool {
	for _, f := range forms {
		if n >= f.min && (f.max < 0 || n <= f.max) {
			return true
		}
	}

	return false
}

// hasNull reports whether an argument list has a null argument, F(A,,B),
// which no intrinsic function takes.
func hasNull(args []ast.Expr) bool {
	for _, x := range args {
		if x == nil {
			return true
		}
	}

	return false
}

// nullSubscript reports, with its message, whether a list of subscripts has one
// left out, A(1,,2). The parser reads a list the same way whatever the name
// is, and only a function or subroutine has null arguments.
func (a *analyzer) nullSubscript(pos diag.Position, args []ast.Expr) bool {
	if !hasNull(args) {
		return false
	}

	a.report("NULLSUBSCRIPT", pos)

	return true
}

// takes is how many arguments a function takes, as INTRINARGS says it.
func takes(forms []form) string {
	lo, hi := forms[0].min, forms[0].max

	for _, f := range forms {
		lo = min(lo, f.min)
		if f.max < 0 || hi >= 0 && f.max > hi {
			hi = f.max
		}
	}

	switch {
	case hi < 0:
		return strconv.Itoa(lo) + " or more arguments"
	case lo == hi:
		return count(lo, "argument")
	}

	return strconv.Itoa(lo) + " or " + count(hi, "argument")
}

// selectForm is the row of forms that args select: the arguments' type
// (argumentType) is the one the row takes. A generic name with two rows for
// that type, differing only in their INTEGER result, takes the one whose
// result is want, the type a KIND= asks for, if one is; else the one of the
// unit's default integer (footnote 10), or LOGICAL's default logical. An
// INTEGER constant may be given to an INTEGER*2 argument if it fits one
// (section 2.2.1.1), so IIABS(5) is IIABS of the word 5. Reports BADARG and
// returns false when no row takes them.
func (a *analyzer) selectForm(e *ast.Apply, forms []form, args []Expr, want Type) (form, bool) {
	t, ok := a.argumentType(e, forms, args)
	if !ok {
		return form{}, false
	}

	var chosen []form

	for _, f := range forms {
		if takesType(f, t, len(args)) {
			chosen = append(chosen, f)
		}
	}

	if len(chosen) == 0 && t == byteType {
		// No row takes INTEGER*1: it is done in INTEGER*2 (byteResult).
		t = Type{Kind: Integer, Size: 2}

		for _, f := range forms {
			if takesType(f, t, len(args)) {
				chosen = append(chosen, f)
			}
		}
	}

	if len(chosen) == 0 && t == IntegerType && wordConstants(args) {
		t = Type{Kind: Integer, Size: 2}

		for _, f := range forms {
			if takesType(f, t, len(args)) {
				chosen = append(chosen, f)
			}
		}
	}

	switch len(chosen) {
	case 0:
		a.report("BADARG", e.Pos, e.Name, wanted(forms), t.String())

		return form{}, false
	case 1:
		return chosen[0], true
	}

	for _, preferred := range []Type{want, a.defaultInt, a.defaultLogical} {
		for _, f := range chosen {
			if f.result == preferred {
				return f, true
			}
		}
	}

	return chosen[0], true
}

// takesType reports whether the row f takes n arguments of type t.
func takesType(f form, t Type, n int) bool {
	if n < f.min || f.max >= 0 && n > f.max {
		return false
	}

	if f.arg.Kind == Character && f.arg.Size == 0 {
		return t.Kind == Character
	}

	return f.arg == t
}

// wordConstants reports whether every argument that isn't an INTEGER*2 is an
// INTEGER constant that fits one.
func wordConstants(args []Expr) bool {
	for _, x := range args {
		if x.Type() == (Type{Kind: Integer, Size: 2}) {
			continue
		}

		if c, ok := x.(*Const); !ok || c.T != IntegerType || c.Int < math.MinInt16 || c.Int > math.MaxInt16 {
			return false
		}
	}

	return true
}

// wanted is the types the rows take, for BADARG: INTEGER*2, INTEGER, or
// REAL.
func wanted(forms []form) string {
	var list []string

	seen := map[string]bool{}

	for _, f := range forms {
		s := f.arg.String()
		if f.arg.Kind == Character {
			s = "CHARACTER"
		}

		if !seen[s] {
			seen[s] = true
			list = append(list, s)
		}
	}

	switch len(list) {
	case 1:
		return list[0]
	case 2:
		return list[0] + " or " + list[1]
	}

	return strings.Join(list[:len(list)-1], ", ") + ", or " + list[len(list)-1]
}

// argumentType is the type the arguments select by: their one kind (a
// function's arguments must agree, but for CMPLX's two, which are each
// converted), and of that kind the highest type, where an INTEGER constant
// that fits a word beside an INTEGER*2 that isn't constant is one, as in any
// other mixed operation (mixedType): MAX(I2, 5) is IMAX0, and IAND(I2, 255)
// IIAND. Section D.5.2 says the same of the bit functions: "if either I or J
// or both were INTEGER*4", ISHFT is JISHFT. Reports BADARG for an argument
// of a kind no row takes, or of another kind than the first's.
func (a *analyzer) argumentType(e *ast.Apply, forms []form, args []Expr) (Type, bool) {
	kinds := map[Kind]bool{}
	for _, f := range forms {
		kinds[f.arg.Kind] = true
	}

	mixed := forms[0].how == "CMPLX" && len(args) == 2

	for _, x := range args {
		k := x.Type().Kind

		switch {
		case !kinds[k], mixed && k == Complex:
			want := wanted(forms)
			if mixed {
				want = "INTEGER or REAL"
			}

			a.report("BADARG", x.Position(), e.Name, want, x.Type().String())

			return invalid, false

		case !mixed && k != args[0].Type().Kind:
			a.report("BADARG", x.Position(), e.Name, args[0].Type().String()+", like the first", x.Type().String())

			return invalid, false
		}
	}

	if len(args) == 0 {
		return invalid, true // NWORKERS()
	}

	// A CHARACTER value of any length, or a LOGICAL (ZEXT's, CHAR's one
	// argument).
	switch t := args[0].Type(); t.Kind {
	case Character:
		return Type{Kind: Character, Size: 0}, true
	case Logical:
		return t, true
	}

	var (
		t       Type
		varying bool
	)

	for _, x := range args {
		if _, ok := x.(*Const); !ok {
			t, varying = higher(t, x.Type()), true
		}
	}

	for _, x := range args {
		if c, ok := x.(*Const); ok {
			fits := c.T == IntegerType && c.Int >= math.MinInt16 && c.Int <= math.MaxInt16
			if !(varying && t == Type{Kind: Integer, Size: 2} && fits) {
				t = higher(t, c.T)
			}
		}
	}

	return t, true
}

// intrinsicArgs are the arguments converted to the types the row takes: to
// its argument type, or for CMPLX to its result's parts' type, the two
// arguments each on its own. A CHARACTER argument, and CHAR's, is as it is.
func (a *analyzer) intrinsicArgs(f form, args []Expr) []Expr {
	out := make([]Expr, len(args))

	for i, x := range args {
		switch {
		case f.how == "CMPLX" && len(args) == 2:
			out[i] = a.convert(x, f.result.part())
		case f.how == "CHAR", f.arg.Kind == Character:
			out[i] = x
		default:
			out[i] = a.convert(x, f.arg)
		}
	}

	return out
}

// operations are the IntrinsicRef operations, by the table's column.
var operations = map[string]IntrinsicOp{
	"ABS": AbsOp, "MAX": MaxOp, "MIN": MinOp, "DIM": DimOp, "SIGN": SignOp, "MOD": ModOp,
	"NINT": NintOp, "ZEXT": ZextOp, "IMAG": ImagOp, "CONJG": ConjgOp,
	"SHIFT": ShiftOp, "BITS": BitsOp, "BSET": BitSetOp, "BCLR": BitClearOp, "BTEST": BitTestOp,
	"LEN": LenOp, "INDEX": IndexOp, "ICHAR": IcharOp, "CHAR": CharOp,
}

// lexicalOps are the lexical comparisons' relational operators (section
// 6.3.2.2: LLT(X,Y) is X .LT. Y).
var lexicalOps = map[string]ast.Op{"LLT": ast.LT, "LLE": ast.LE, "LGT": ast.GT, "LGE": ast.GE}

// bitOps are the bit functions' operators on INTEGERs.
var bitOps = map[string]ast.Op{"AND": ast.And, "OR": ast.Or, "XOR": ast.Xor}

// intrinsicValue is what a reference to the row f of args becomes.
func (a *analyzer) intrinsicValue(e *ast.Apply, f form, args []Expr) Expr {
	name := f.specific
	if name == "" {
		name = f.generic
	}

	ref := func(t Type, op IntrinsicOp) Expr {
		return &IntrinsicRef{exprBase{T: t, Pos: e.Pos}, name, op, args, ""}
	}

	call := func(routine string) Expr {
		return &IntrinsicRef{exprBase{T: f.result, Pos: e.Pos}, name, CallOp, args, routine}
	}

	switch f.how {
	case "MTH":
		return call(a.routineOf(f))

	case "SHIFTC":
		// No instruction does a circular shift, and MTHRTL has none.
		return call("FTN$" + name)

	case "CVT":
		return a.convert(args[0], f.result)

	case "ABS", "MAX", "MIN", "DIM", "SIGN", "MOD":
		// Computed in the arguments' type: MAX1 is a REAL maximum, made an
		// INTEGER.
		return a.convert(ref(f.arg, operations[f.how]), f.result)

	case "CMPLX":
		if len(args) == 1 {
			return a.convert(args[0], f.result)
		}

		return ref(f.result, CmplxOp)

	case "DPROD":
		t := f.result

		return a.fold(&Binary{exprBase{T: t, Pos: e.Pos}, ast.Mul, a.convert(args[0], t), a.convert(args[1], t)})

	case "AND", "OR", "XOR":
		return a.fold(&Binary{exprBase{T: f.result, Pos: e.Pos}, bitOps[f.how], args[0], args[1]})

	case "NOT":
		return a.fold(&Unary{exprBase{T: f.result, Pos: e.Pos}, ast.Not, args[0]})

	case "SHIFT":
		// A count that isn't constant may shift either way: MTH$JISHFT or
		// MTH$IISHFT decides when the program runs.
		if _, ok := args[1].(*Const); !ok {
			return call(a.routineOf(f))
		}

	case "LEN":
		if n := args[0].Type().Size; n > 0 {
			return &Const{exprBase: exprBase{T: f.result, Pos: e.Pos}, Int: int32(n)}
		}

	case "INDEX":
		x, okx := args[0].(*Const)
		y, oky := args[1].(*Const)

		if okx && oky {
			return &Const{exprBase: exprBase{T: f.result, Pos: e.Pos}, Int: int32(strings.Index(x.Text, y.Text) + 1)}
		}

	case "LLT", "LLE", "LGT", "LGE":
		x, okx := args[0].(*Const)
		y, oky := args[1].(*Const)

		if okx && oky {
			return logicalConst(relation(lexicalOps[f.how], compareText(x.Text, y.Text)), e.Pos)
		}

		return &Compare{exprBase{T: f.result, Pos: e.Pos}, lexicalOps[f.how], args[0], args[1]}

	case "ONE":
		return &Const{exprBase: exprBase{T: f.result, Pos: e.Pos}, Int: 1}
	}

	return ref(f.result, operations[f.how])
}

// sizeOf is SIZEOF(X): the bytes X takes, a constant INTEGER*4. X may be
// anything (a whole array among them) but an assumed-size array or a
// CHARACTER value whose length is passed, or otherwise known only when the
// program runs (Table D-3); an adjustable array's size is a run-time value.
func (a *analyzer) sizeOf(e *ast.Apply) Expr {
	var x Expr

	switch n := a.substringForm(e.Args[0]).(type) {
	case *ast.Name:
		x = a.name(n, true)
	case *ast.Apply:
		x = a.apply(n)
	case *ast.FieldRef:
		x = a.field(n, true)
	default:
		x = a.value(e.Args[0])
	}

	t := x.Type()
	if t.Kind == Invalid {
		return a.bad(e.Pos)
	}

	size := t.Size

	if f, ok := x.(*FieldRef); ok && f.WholeArray() {
		size = f.Field().Size()
	}

	if r, ok := x.(*ArrayRef); ok {
		switch {
		case r.Sym.AssumedSize():
			size = 0
		case (r.Sym.Adjustable() || r.Sym.Allocatable || r.Sym.AssumedShape()) && r.Sym.Type.Size != 0:
			// Known only when the program runs, once the bounds are.
			return &IntrinsicRef{exprBase: exprBase{T: IntegerType, Pos: e.Pos}, Op: ArraySizeOp, Args: []Expr{x}}
		case r.Sym.Adjustable():
			size = 0
		default:
			size = r.Sym.Size()
		}
	}

	if size == 0 {
		a.report("BADARG", x.Position(), e.Name, "anything but an assumed-size array or a passed-length value", describeSize(x))

		return a.bad(e.Pos)
	}

	return &Const{exprBase: exprBase{T: IntegerType, Pos: e.Pos}, Int: int32(size)}
}

// describeSize is what SIZEOF's argument is, when it has no size: an
// assumed-size array, or a CHARACTER value or array of a length not known.
func describeSize(x Expr) string {
	if r, ok := x.(*ArrayRef); ok && r.Sym.AssumedSize() {
		return "an assumed-size array"
	}

	return "CHARACTER*(*)"
}
