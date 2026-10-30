package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// declarations is the first walk over a unit's statements: the type
// statements and DIMENSION statements, and the order of every statement
// (Figure 1-1, order.go). It also finds the statement function
// definitions, which look like assignments to array elements and can be
// told from them only here, where it is known which names are arrays.
func (a *analyzer) declarations(body []ast.Stmt) {
	for _, s := range body {
		stmtFunc := false

		if st, ok := s.(*ast.AssignStmt); ok {
			stmtFunc = a.statementFunction(st)
		}

		a.order(s, stmtFunc)

		switch s := s.(type) {
		case *ast.TypeStmt:
			a.typeStatement(s)

		case *ast.DimensionStmt:
			for _, d := range s.Entities {
				a.dimension(a.lookup(d.Name, d.Pos), d)
			}
		}
	}
}

// statementFunction reports whether the assignment s is a statement
// function's definition, f(p[,p]...) = e (section 6.2.1), and enters f
// as one if it is. It is one if it comes before the unit's executable
// statements (an assignment can't come before the specification
// statements, so the unit is in their group or the statement functions'),
// its target's name isn't an array or a dummy argument, and each
// parenthesized item is a name, a dummy argument: AVG(A, B, 3.) = ...
// is an assignment to an element of an array AVG (NOTARRAY), as F(X) =
// ... is after the first executable statement.
//
// The statement function's code, and its references', are Phase 79's: a
// definition is NOTIMPL where the statement walk reaches it, and a
// reference an expression with an error.
func (a *analyzer) statementFunction(s *ast.AssignStmt) bool {
	ap, ok := s.Target.(*ast.Apply)
	if !ok || a.group > stmtFuncGroup {
		return false
	}

	if sym := a.unit.byName[ap.Name]; sym != nil && (sym.Class == Array || sym.Dummy != 0) {
		return false
	}

	for _, arg := range ap.Args {
		if _, ok := arg.(*ast.Name); !ok {
			return false
		}
	}

	sym := a.lookup(ap.Name, ap.Pos)

	switch {
	case sym.Class == StmtFunction:
		a.report("DUPSTMTFUNC", ap.Pos, ap.Name, sym.defPos.Line)
		a.stmtFuncs[s] = nil

	case sym.Class == UnitName || sym.Result:
		a.badUse(sym, ap.Pos, "be a statement function")
		a.stmtFuncs[s] = nil

	default:
		sym.Class, sym.defPos = StmtFunction, ap.Pos
		a.stmtFuncs[s] = sym

		seen := map[string]bool{}

		for _, arg := range ap.Args {
			n := arg.(*ast.Name)
			if seen[n.Name] {
				a.report("DUPDUMMY", n.Pos, n.Name, "statement function")
			}

			seen[n.Name] = true
		}
	}

	return true
}

// typeStatement gives each name a type statement declares its type, and
// an array declarator's name its dimensions (sections 4.4.1, 4.4.2). A
// length after a name is that name's (INTEGER*2 IVEC*4(10) makes IVEC
// INTEGER*4); the statement's is every other name's, checked once, at
// the statement.
func (a *analyzer) typeStatement(s *ast.TypeStmt) {
	var (
		stmtType Type
		stmtOK   bool
		checked  bool
	)

	for _, d := range s.Entities {
		sym := a.lookup(d.Name, d.Pos)

		t, ok := stmtType, stmtOK

		switch {
		case d.HasLength():
			t, ok = a.typeOf(s.Type, d.Len, d.Pos)
		case !checked:
			stmtType, stmtOK = a.typeOf(s.Type, s.Type.Len, s.Type.Pos)
			t, ok, checked = stmtType, stmtOK, true
		}

		if d.Init != nil {
			a.notImplForm(d.InitPos, "a type statement's initial values")
		}

		switch {
		case sym.Class == UnitName:
			a.badUse(sym, d.Pos, "be given a type")
		case sym.Typed:
			a.report("DUPDECL", d.Pos, d.Name, sym.TypePos.Line)
		default:
			sym.Type, sym.Typed, sym.TypePos, sym.broken = t, true, d.Pos, sym.broken || !ok
		}

		if d.Dims != nil {
			a.dimension(sym, d)
		}
	}
}

// lengths are the lengths each numeric type may be given (Table 2-1), as
// a message lists them, and the type's own default.
var lengths = map[ast.BaseType]struct {
	def     Type
	allowed []int
	text    string
}{
	ast.Integer: {IntegerType, []int{2, 4}, "2 and 4"},
	ast.Logical: {LogicalType, []int{1, 2, 4}, "1, 2, and 4"},
	ast.Real:    {RealType, []int{4, 8, 16}, "4, 8, and 16"},
	ast.Complex: {Type{Complex, 8}, []int{8, 16}, "8 and 16"},
}

// fixedTypes are the type keywords that name one length, and so may not
// be given another (Table 2-1): BYTE is LOGICAL*1, DOUBLE PRECISION
// REAL*8, DOUBLE COMPLEX COMPLEX*16.
var fixedTypes = map[ast.BaseType]Type{
	ast.Byte:            {Logical, 1},
	ast.DoublePrecision: {Real, 8},
	ast.DoubleComplex:   {Complex, 16},
}

// maxCharLength is the longest CHARACTER length (section 2.2.4).
const maxCharLength = 65535

// typeOf is the type spec with the length l means (Table 2-1): a numeric
// type's length must be one of its own (BADLENGTH; none for the types
// that name a length, NOLENGTH), and a CHARACTER length is characterType's.
// The length 4 (8 for COMPLEX) is the default.
func (a *analyzer) typeOf(spec ast.TypeSpec, l ast.Len, pos diag.Position) (Type, bool) {
	if spec.Type == ast.Character {
		return a.characterType(l, pos)
	}

	if t, ok := fixedTypes[spec.Type]; ok {
		if l.HasLength() {
			a.report("NOLENGTH", pos, spec.Type.String())

			return invalid, false
		}

		return t, true
	}

	info, ok := lengths[spec.Type]
	if !ok {
		return invalid, false
	}

	if l.LenExpr != nil || l.LenStar {
		// Only an untyped function's length can be written so, when
		// its type turns out not to be CHARACTER.
		a.report("NUMLENGTH", pos, spec.Type.String())

		return invalid, false
	}

	if !l.HasLength() {
		return info.def, true
	}

	for _, n := range info.allowed {
		if n == l.Length {
			return Type{info.def.Kind, n}, true
		}
	}

	a.report("BADLENGTH", pos, spec.Type.String(), l.Length, spec.Type.String(), info.text)

	return invalid, false
}

// characterType is the CHARACTER type of length l (section 4.4.2): 1
// when none is written; a constant, or a parenthesized integer constant
// expression, from 1 to 65535 (BADCHARLEN); or (*), a passed length,
// which only some names may have (passedLengths).
func (a *analyzer) characterType(l ast.Len, pos diag.Position) (Type, bool) {
	n := int64(l.Length)

	switch {
	case l.LenStar:
		return Type{Character, 0}, true

	case l.LenExpr != nil:
		v, kind := a.constInt(l.LenExpr)

		switch kind {
		case constFailed:
			return invalid, false
		case constOK:
			n = int64(v)
		default:
			n = 0
		}

		pos = l.LenExpr.Position()

	case !l.HasLength():
		n = 1
	}

	if n < 1 || n > maxCharLength {
		a.report("BADCHARLEN", pos)

		return invalid, false
	}

	return Type{Character, int(n)}, true
}

// baseTypes are the type keywords for the kinds of type, for a length
// written after an untyped function's name (functionType).
var baseTypes = map[Kind]ast.BaseType{
	Integer: ast.Integer, Real: ast.Real, Logical: ast.Logical,
	Complex: ast.Complex, Character: ast.Character,
}

// functionType is the type of the function a FUNCTION statement with a
// type spec declares (section 6.2.2). FUNCTION F*8, a length with no
// type keyword, gives the length to the type F's first letter gives it:
// the manual's form allows the length, and says only that it must be one
// of the type's (ours, unconfirmed).
func (a *analyzer) functionType(spec ast.TypeSpec, s *Symbol) (Type, bool) {
	if spec.Type == 0 {
		if s.untyped {
			// IMPLICIT NONE: NOTYPE will say so.
			return invalid, true
		}

		spec.Type = baseTypes[s.Type.Kind]
	}

	return a.typeOf(spec, spec.Len, spec.Pos)
}

// functionResult reports a function whose type this phase doesn't
// compile (NOTIMPL), at its FUNCTION statement, once its declarations
// are read: the type its header, a type statement, or its first letter
// gave it.
func (a *analyzer) functionResult() {
	r := a.unit.Result
	if r == nil || r.broken || r.Type.Kind == Invalid || r.Type.Compiled() {
		return
	}

	a.notImplForm(a.unit.AST.Header.Position(), article(r.Type.String()+" function"))

	r.broken = true
}

// article is text with "a" or "an" before it: "an INTEGER*2 function".
func article(text string) string {
	switch text[0] {
	case 'A', 'E', 'I', 'O', 'U', 'a', 'e', 'i', 'o', 'u':
		return "an " + text
	}

	return "a " + text
}

// dimension gives sym the dimensions its array declarator d declares
// (sections 2.2.3.1, 4.5). In this phase a bound must be a constant
// (adjustable arrays, whose bounds are dummy arguments, are Phase 79's),
// and only a dummy array's last upper bound may be *, an assumed-size
// array.
func (a *analyzer) dimension(sym *Symbol, d *ast.Declarator) {
	switch {
	case sym.Class == UnitName || sym.Result:
		a.badUse(sym, d.Pos, "be an array")

		sym.broken = true

		return

	case sym.Dims != nil:
		a.report("DUPDIM", d.Pos, d.Name, sym.DimPos.Line)

		return
	}

	dims := make([]Dim, len(d.Dims))

	// Whatever goes wrong, the name is an array: its uses are elements.
	sym.Class, sym.DimPos = Array, d.Pos
	sym.Dims = dims

	for i, b := range d.Dims {
		dim := Dim{Lower: 1}

		if b.Lower != nil {
			lower, ok := a.bound(sym, b.Lower)
			if !ok {
				sym.broken = true

				return
			}

			dim.Lower = lower
		}

		if b.Star {
			if sym.Dummy == 0 || i != len(d.Dims)-1 {
				a.report("ASSUMEDSIZE", d.Pos)

				sym.broken = true

				return
			}

			dim.Assumed = true
		} else {
			upper, ok := a.bound(sym, b.Upper)
			if !ok {
				sym.broken = true

				return
			}

			if upper < dim.Lower {
				a.report("BOUNDORDER", b.Upper.Position(), upper, dim.Lower)

				sym.broken = true

				return
			}

			dim.Upper = upper
		}

		dims[i] = dim
	}
}

// bound is the value of an array bound, which must be an integer
// constant expression; a bound that names a variable is an adjustable
// array's, NOTIMPL for a dummy array in a subprogram and BADBOUND
// anywhere else.
func (a *analyzer) bound(sym *Symbol, e ast.Expr) (int, bool) {
	v, kind := a.constInt(e)

	switch kind {
	case constOK:
		return int(v), true
	case constNamed:
		if sym.Dummy != 0 {
			a.notImpl(e.Position(), "an adjustable array")
		} else {
			a.report("BADBOUND", e.Position())
		}
	case constNotInteger:
		a.report("BADBOUND", e.Position())
	}

	return 0, false
}

// constKind is what constInt found.
type constKind int

const (
	constOK         constKind = iota
	constNamed                // the expression names a variable
	constNotInteger           // it isn't an integer expression
	constFailed               // an error, already reported
)

// constInt is the value of an integer constant expression in a
// declaration, where its names mustn't be entered in the symbol table as
// variables (a later phase's PARAMETER constants will be allowed in
// them).
func (a *analyzer) constInt(e ast.Expr) (int32, constKind) {
	switch e := e.(type) {
	case *ast.IntConst:
		v, err := intValue(e)
		if err != nil || v > maxInt {
			a.report("INTRANGE", e.Pos, ast.DumpExpr(e))

			return 0, constFailed
		}

		return int32(v), constOK

	case *ast.Paren:
		return a.constInt(e.X)

	case *ast.Unary:
		if e.Op != ast.Add && e.Op != ast.Sub {
			return 0, constNotInteger
		}

		x, kind := a.constInt(e.X)
		if kind != constOK || e.Op == ast.Add {
			return x, kind
		}

		return a.checkInt(-int64(x), e.Pos)

	case *ast.Binary:
		switch e.Op {
		case ast.Add, ast.Sub, ast.Mul, ast.Div:
		default:
			return 0, constNotInteger
		}

		x, kx := a.constInt(e.X)
		y, ky := a.constInt(e.Y)

		switch {
		case kx == constFailed || ky == constFailed:
			return 0, constFailed
		case kx != constOK:
			return 0, kx
		case ky != constOK:
			return 0, ky
		}

		v, ok := a.intOp(e.Op, x, y, e.Pos)
		if !ok {
			return 0, constFailed
		}

		return v, constOK

	case *ast.Name, *ast.Apply:
		return 0, constNamed
	}

	if what := formWhat(e); what != "" {
		a.notImplForm(e.Position(), what)

		return 0, constFailed
	}

	return 0, constNotInteger
}

// checkInt is v, if it is in INTEGER's range.
func (a *analyzer) checkInt(v int64, pos diag.Position) (int32, constKind) {
	if v < minInt || v > maxInt {
		a.report("INTRANGE", pos, "the value of this expression")

		return 0, constFailed
	}

	return int32(v), constOK
}
