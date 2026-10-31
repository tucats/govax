package sema

import (
	"math"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
)

// A reference to an intrinsic function (section 6.3), by the table in
// intrinsics.go: its arguments are counted and analyzed, their type selects
// the table's row (a generic name's by the arguments, section 6.3.1.1), each
// is converted to the type the row takes, and the row's last column says
// what the reference becomes.

// intrinsicRef analyzes a reference to the intrinsic function e.Name.
func (a *analyzer) intrinsicRef(e *ast.Apply) Expr {
	forms := formsNamed(e.Name)

	if !argCountFits(forms, len(e.Args)) || hasNull(e.Args) {
		a.report("INTRINARGS", e.Pos, e.Name, takes(forms), len(e.Args))

		return a.bad(e.Pos)
	}

	if e.Name == "SIZEOF" {
		return a.sizeOf(e)
	}

	args := make([]Expr, len(e.Args))
	for i, x := range e.Args {
		if args[i] = a.value(x); args[i].Type().Kind == Invalid {
			return a.bad(e.Pos)
		}
	}

	f, ok := a.selectForm(e, forms, args)
	if !ok {
		return a.bad(e.Pos)
	}

	return a.intrinsicValue(e, f, a.intrinsicArgs(f, args))
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
// that type, differing only in their INTEGER result, takes the one of the
// unit's default integer (footnote 10). An INTEGER constant may be given to an
// INTEGER*2 argument if it fits one (section 2.2.1.1), so IIABS(5) is
// IIABS of the word 5. Reports BADARG and returns false when no row takes them.
func (a *analyzer) selectForm(e *ast.Apply, forms []form, args []Expr) (form, bool) {
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

	if len(chosen) == 0 && t == IntegerType && wordConstants(args) {
		t = Type{Integer, 2}

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

	for _, f := range chosen {
		if f.result == a.defaultInt {
			return f, true
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
		if x.Type() == (Type{Integer, 2}) {
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
		return Type{Character, 0}, true
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
			if !(varying && t == Type{Integer, 2} && fits) {
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
		return &IntrinsicRef{exprBase{t, e.Pos}, name, op, args, ""}
	}

	call := func(routine string) Expr {
		return &IntrinsicRef{exprBase{f.result, e.Pos}, name, CallOp, args, routine}
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

		return a.fold(&Binary{exprBase{t, e.Pos}, ast.Mul, a.convert(args[0], t), a.convert(args[1], t)})

	case "AND", "OR", "XOR":
		return a.fold(&Binary{exprBase{f.result, e.Pos}, bitOps[f.how], args[0], args[1]})

	case "NOT":
		return a.fold(&Unary{exprBase{f.result, e.Pos}, ast.Not, args[0]})

	case "SHIFT":
		// A count that isn't constant may shift either way: MTH$JISHFT or
		// MTH$IISHFT decides when the program runs.
		if _, ok := args[1].(*Const); !ok {
			return call(a.routineOf(f))
		}

	case "LEN":
		if n := args[0].Type().Size; n > 0 {
			return &Const{exprBase: exprBase{f.result, e.Pos}, Int: int32(n)}
		}

	case "INDEX":
		x, okx := args[0].(*Const)
		y, oky := args[1].(*Const)

		if okx && oky {
			return &Const{exprBase: exprBase{f.result, e.Pos}, Int: int32(strings.Index(x.Text, y.Text) + 1)}
		}

	case "LLT", "LLE", "LGT", "LGE":
		x, okx := args[0].(*Const)
		y, oky := args[1].(*Const)

		if okx && oky {
			return logicalConst(relation(lexicalOps[f.how], compareText(x.Text, y.Text)), e.Pos)
		}

		return &Compare{exprBase{f.result, e.Pos}, lexicalOps[f.how], args[0], args[1]}

	case "ONE":
		return &Const{exprBase: exprBase{f.result, e.Pos}, Int: 1}
	}

	return ref(f.result, operations[f.how])
}

// sizeOf is SIZEOF(X): the bytes X takes, a constant INTEGER*4. X may be
// anything (a whole array among them) but an assumed-size array or a
// CHARACTER value whose length is passed, or otherwise known only when the
// program runs (Table D-3); an adjustable array's size is a run-time value.
func (a *analyzer) sizeOf(e *ast.Apply) Expr {
	var x Expr

	if n, ok := e.Args[0].(*ast.Name); ok {
		x = a.name(n, true)
	} else {
		x = a.value(e.Args[0])
	}

	t := x.Type()
	if t.Kind == Invalid {
		return a.bad(e.Pos)
	}

	size := t.Size

	if r, ok := x.(*ArrayRef); ok {
		switch {
		case r.Sym.AssumedSize():
			size = 0
		case r.Sym.Adjustable() && r.Sym.Type.Size != 0:
			// Known only when the program runs, once the bounds are.
			return &IntrinsicRef{exprBase: exprBase{IntegerType, e.Pos}, Op: ArraySizeOp, Args: []Expr{x}}
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

	return &Const{exprBase: exprBase{IntegerType, e.Pos}, Int: int32(size)}
}

// describeSize is what SIZEOF's argument is, when it has no size: an
// assumed-size array, or a CHARACTER value or array of a length not known.
func describeSize(x Expr) string {
	if r, ok := x.(*ArrayRef); ok && r.Sym.AssumedSize() {
		return "an assumed-size array"
	}

	return "CHARACTER*(*)"
}
