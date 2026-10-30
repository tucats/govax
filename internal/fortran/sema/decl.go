package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// declarations is the first walk over a unit's statements: the type
// statements, DIMENSION statements, COMMON statements, and PARAMETER
// statements, in order (so that a constant may be used in the
// declarations after its definition), the EQUIVALENCE statements, kept
// for when every declaration is known, and the order of every statement
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

		case *ast.CommonStmt:
			a.commonStatement(s)

		case *ast.EquivalenceStmt:
			a.equivStmts = append(a.equivStmts, s)

		case *ast.ParameterStmt:
			for _, d := range s.Defs {
				a.parameter(s, d)
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

	case sym.Class == UnitName || sym.Result || sym.Class == Constant || sym.Common != nil:
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

		a.checkBoundType(sym, t, ok, d.Pos)

		switch {
		case sym.Class == UnitName:
			a.badUse(sym, d.Pos, "be given a type")
		case sym.Class == Constant && sym.oldForm:
			a.report("OLDPARAMTYPE", d.Pos, d.Name, "PARAMETER statement", sym.defPos.Line)
		case sym.Class == Constant:
			a.report("TYPEAFTER", d.Pos, d.Name, sym.defPos.Line)
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

// constInt is the value of an integer constant expression in a
// declaration (an array bound, a CHARACTER length), which may use the
// unit's constants; names that aren't constants aren't entered in the
// symbol table as variables (constNamed).
func (a *analyzer) constInt(e ast.Expr) (int32, constKind) {
	c, err := a.declEval().eval(e)

	switch {
	case err != nil && err.kind == constNot:
		return 0, constNotInteger
	case err != nil:
		return 0, err.kind
	case c.T.Kind != Integer:
		return 0, constNotInteger
	}

	return c.Int, constOK
}

// checkInt is v, if it is in INTEGER's range.
func (a *analyzer) checkInt(v int64, pos diag.Position) (int32, constKind) {
	if v < minInt || v > maxInt {
		a.intRangeError(pos, valueText, IntegerType)

		return 0, constFailed
	}

	return int32(v), constOK
}
