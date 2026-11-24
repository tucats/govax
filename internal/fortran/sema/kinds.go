package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Fortran 90's kinds (docs/planning/PHASE-100 - Fortran 90 declarations and
// kinds.md, subtask 4). A type's kind is a number naming one of the ways the
// processor represents values of that type (the standard's 4.3). govax
// numbers them as DEC's later compilers do, by size in bytes (Phase 99's
// Decision 1): INTEGER(2) is INTEGER*2, REAL(8) is REAL*8 (D or G by the
// unit's /G_FLOATING, as REAL*8 is), LOGICAL(1) is LOGICAL*1. A COMPLEX's
// kind is its parts' (4.3.1.3: every real kind has a complex kind of the
// same precision), so COMPLEX(4) is COMPLEX*8 and COMPLEX(8) COMPLEX*16.
// CHARACTER has one kind, 1.
//
// A kind is written in a type's selector, REAL(KIND=8) or REAL(8), and after
// a constant, 1.5_8 or 7_2 or .TRUE._1 or 1_'text'; either may name an
// INTEGER constant instead of writing digits, REAL(DP), 1.0_DP. A kind must
// be one the type has (NOSUCHKIND).

// KindNumber is the type's Fortran 90 kind: its size in bytes, but for
// COMPLEX, whose kind is its parts' size, and CHARACTER, whose kind is 1
// whatever its length.
func (t Type) KindNumber() int {
	switch t.Kind {
	case Complex:
		return t.Size / 2
	case Character:
		return 1
	}

	return t.Size
}

// kindInfo is what kinds a type keyword has: the kind of type it makes,
// and the kinds a selector or a constant may name, as a message lists them.
// (Its NOTIMPL hook for kinds a later subtask was to compile went in Phase
// 113, subtask 11: INTEGER(1) and COMPLEX(16), the last, came in Phase 100.)
type kindInfo struct {
	kind    Kind
	allowed []int
	text    string
}

// typeKinds are the type keywords that take a kind, and their kinds.
// DOUBLE PRECISION, DOUBLE COMPLEX, and BYTE name one type already
// (NOKIND).
var typeKinds = map[ast.BaseType]kindInfo{
	ast.Integer:   {Integer, []int{1, 2, 4}, "1, 2, and 4"},
	ast.Logical:   {Logical, []int{1, 2, 4}, "1, 2, and 4"},
	ast.Real:      {Real, []int{4, 8, 16}, "4, 8, and 16"},
	ast.Complex:   {Complex, []int{4, 8, 16}, "4, 8, and 16"},
	ast.Character: {Character, []int{1}, "1"},
}

// kindProblem is why a kind isn't one a type has: a message's identifier
// and arguments.
type kindProblem struct {
	ident string
	args  []any
}

// kindType is the type keyword base of kind k (for CHARACTER, the kind is
// checked and the type's length is the caller's).
func kindType(base ast.BaseType, k int) (Type, *kindProblem) {
	info, ok := typeKinds[base]
	if !ok {
		return invalid, &kindProblem{ident: "NOKIND", args: []any{base.String()}}
	}

	for _, n := range info.allowed {
		if n != k {
			continue
		}

		if info.kind == Complex {
			return Type{Kind: Complex, Size: 2 * k}, nil
		}

		return Type{Kind: info.kind, Size: k}, nil
	}

	return invalid, &kindProblem{ident: "NOSUCHKIND", args: []any{base.String(), k, info.text}}
}

// reportKind reports a kindProblem at pos.
func (a *analyzer) reportKind(p *kindProblem, pos diag.Position) {
	a.report(p.ident, pos, p.args...)
}

// kindValue is the value of a selector's kind expression: a scalar integer
// constant expression (the standard's R505), not negative (KINDNOTCONST).
func (a *analyzer) kindValue(e ast.Expr) (int, bool) {
	v, kind := a.constInt(e)

	switch {
	case kind == constFailed:
		return 0, false
	case kind != constOK || v < 0:
		a.report("KINDNOTCONST", e.Position())

		return 0, false
	}

	return int(v), true
}

// selectedType is the type a non-CHARACTER type spec with a selector names,
// REAL(KIND=8) or INTEGER(2).
func (a *analyzer) selectedType(spec ast.TypeSpec) (Type, bool) {
	if spec.Kind == nil {
		// CHARACTER's selector may leave its kind out; no other's can.
		return invalid, false
	}

	k, ok := a.kindValue(spec.Kind)
	if !ok {
		return invalid, false
	}

	t, problem := kindType(spec.Type, k)
	if problem != nil {
		a.reportKind(problem, spec.KindPos)

		return invalid, false
	}

	return t, true
}

// characterKind checks a CHARACTER selector's kind, if it has one.
func (a *analyzer) characterKind(spec ast.TypeSpec) bool {
	if spec.Kind == nil {
		return true
	}

	k, ok := a.kindValue(spec.Kind)
	if !ok {
		return false
	}

	if _, problem := kindType(ast.Character, k); problem != nil {
		a.reportKind(problem, spec.KindPos)

		return false
	}

	return true
}

// constantKind is a constant's kind parameter's value (R404): digits, or the
// name of an INTEGER constant (KINDNAME).
func (ev *evaluator) constantKind(e *ast.KindConst) (int, *constErr) {
	if n, err := strconv.Atoi(e.Kind); err == nil {
		return n, nil
	}

	c, err := ev.eval(&ast.Name{ExprBase: ast.ExprBase{Pos: e.Pos}, Name: e.Kind})

	switch {
	case err != nil && err.kind == constFailed:
		return 0, err
	case err != nil, c.T.Kind != Integer, c.Int < 0:
		return 0, ev.report("KINDNAME", e.Pos, e.Kind)
	}

	return int(c.Int), nil
}

// kindConst is a constant with a kind parameter (the standard's R404, R413,
// R420, R421): the constant read at the kind's type. A real constant's digits
// are read at the kind's precision, so 1.1_8 is 1.1D0 and not 1.1 widened; a
// real with a D or Q exponent names its type already (KINDEXPONENT). An
// integer must be in its kind's range.
func (ev *evaluator) kindConst(e *ast.KindConst) (*Const, *constErr) {
	k, err := ev.constantKind(e)
	if err != nil {
		return nil, err
	}

	base := map[string]ast.BaseType{"int": ast.Integer, "real": ast.Real, "logical": ast.Logical, "char": ast.Character}

	var form string

	switch x := e.X.(type) {
	case *ast.IntConst:
		form = "int"
	case *ast.RealConst:
		if strings.ContainsAny(x.Text, "DQ") {
			return nil, ev.report("KINDEXPONENT", e.Pos, x.Text)
		}

		form = "real"
	case *ast.LogicalConst:
		form = "logical"
	case *ast.CharConst:
		form = "char"
	default:
		return nil, ev.report("KINDCONST", e.Pos)
	}

	t, problem := kindType(base[form], k)
	if problem != nil {
		if !ev.quiet {
			ev.a.reportKind(problem, e.Pos)
		}

		return nil, failed
	}

	switch x := e.X.(type) {
	case *ast.IntConst:
		v, perr := intValue(x)

		lo, hi := t.intRange()
		if perr != nil || v < int64(lo) || v > int64(hi) {
			return nil, ev.intRange(e.Pos, ast.DumpExpr(x)+"_"+e.Kind, t)
		}

		return &Const{exprBase: exprBase{T: t, Pos: e.Pos}, Int: int32(v)}, nil

	case *ast.RealConst:
		return ev.realAs(x, t, x.Text)

	case *ast.LogicalConst:
		return ev.convert(logicalConst(x.Value, e.Pos), t, e.Pos)
	}

	return ev.eval(e.X)
}
