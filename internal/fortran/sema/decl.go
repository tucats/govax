package sema

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// declarations is the first walk over a unit's statements: the type
// statements and DIMENSION statements, and the order rule that puts
// them before the executable statements (Figure 1-1). It also finds the
// statement function definitions, which look like assignments to array
// elements and can be told from them only here, where it is known which
// names are arrays.
func (a *analyzer) declarations(body []ast.Stmt) {
	for _, s := range body {
		switch s := s.(type) {
		case *ast.TypeStmt:
			a.lateDeclaration(s)
			a.typeStatement(s)

		case *ast.DimensionStmt:
			a.lateDeclaration(s)

			for _, d := range s.Entities {
				a.dimension(a.lookup(d.Name, d.Pos), d)
			}

		case *ast.AssignStmt:
			// F(X) = ... before any executable statement, with F not an
			// array, defines the statement function F (section 6.3,
			// Phase 79). After one it can only be an assignment, and F
			// had better be an array.
			if ap, ok := s.Target.(*ast.Apply); ok && a.firstExec == nil {
				if sym := a.unit.byName[ap.Name]; sym == nil || sym.Class != Array {
					a.stmtFuncs[s] = true

					continue
				}
			}

			a.firstExec = s

		default:
			if a.firstExec == nil && ast.Executable(s) {
				a.firstExec = s
			}
		}
	}
}

// lateDeclaration reports a declaration after the first executable
// statement (LATEDECL).
func (a *analyzer) lateDeclaration(s ast.Stmt) {
	if a.firstExec != nil {
		a.report("LATEDECL", s.Position(), a.firstExec.Position().Line)
	}
}

// typeStatement gives each name a type statement declares its type, and
// an array declarator's name its dimensions (section 4.4.1).
func (a *analyzer) typeStatement(s *ast.TypeStmt) {
	for _, d := range s.Entities {
		sym := a.lookup(d.Name, d.Pos)

		length := s.Type.Length
		if d.Length != 0 {
			length = d.Length
		}

		switch {
		case sym.Class == UnitName:
			a.badUse(sym, d.Pos, "be given a type")
		case sym.Typed:
			a.report("DUPDECL", d.Pos, d.Name, sym.TypePos.Line)
		default:
			t, ok := a.typeOf(s.Type, length, d.Pos)
			sym.Type, sym.Typed, sym.TypePos, sym.broken = t, true, d.Pos, sym.broken || !ok
		}

		if d.Dims != nil {
			a.dimension(sym, d)
		}
	}
}

// lengths are the lengths each type may be given (Table 2-1), as a
// message lists them.
var lengths = map[ast.BaseType]struct {
	allowed []int
	text    string
}{
	ast.Integer: {[]int{2, 4}, "2 and 4"},
	ast.Logical: {[]int{1, 2, 4}, "1, 2, and 4"},
	ast.Real:    {[]int{4, 8, 16}, "4, 8, and 16"},
}

// typeOf is the type spec with length (0 for the default) means, if it
// is one this phase compiles. A length the type doesn't have is
// BADLENGTH; one it has but this phase doesn't compile (INTEGER*2,
// REAL*8, ...) is NOTIMPL.
func (a *analyzer) typeOf(spec ast.TypeSpec, length int, pos diag.Position) (Type, bool) {
	if length != 0 && length != 4 {
		l := lengths[spec.Type]

		for _, n := range l.allowed {
			if n == length {
				a.notImpl(pos, spec.Type.String()+"*"+strconv.Itoa(length))

				return invalid, false
			}
		}

		a.report("BADLENGTH", pos, spec.Type.String(), length, spec.Type.String(), l.text)

		return invalid, false
	}

	switch spec.Type {
	case ast.Integer:
		return IntegerType, true
	case ast.Real:
		return RealType, true
	case ast.Logical:
		return LogicalType, true
	}

	return invalid, false
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
		v, err := strconv.ParseInt(e.Text, 10, 64)
		if err != nil || v > maxInt {
			a.report("INTRANGE", e.Pos, e.Text)

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
