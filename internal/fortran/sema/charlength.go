package sema

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// CHARACTER lengths that aren't constant (the Fortran 90 standard's 5.1.1.5;
// docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, finding 7). In
//
//	SUBROUTINE PAD(S, N)
//	CHARACTER(LEN=N) :: S, T
//	CHARACTER*(2*N) W(3)
//
// each length is a specification expression, made of what an adjustable
// array's bounds may be made of (arrays.go: dummy arguments, names in common
// blocks, a host's variables, the inquiry functions, the integer elementals),
// and computed at each entry point whose list defines it, after which it
// doesn't change. S, a dummy argument, is the first N characters of its actual
// argument, which must have at least that many (12.4.1.1); T and W, the
// subprogram's own, are automatic objects, made on the stack at entry, as an
// automatic array is. A length less than 0 is 0.
//
// Such a name's type is CHARACTER of length 0, as a passed length's,
// CHARACTER*(*), is: either way the length isn't known when compiling, and the
// rules that follow from that (concatenations, substrings, LEN) are the same.
// The difference, where the length comes from, is the IR's (charLength).

// provisionalBound is a name in a bound or a length that an internal
// procedure's declarations made provisionally its own (checkProvisional), and
// what the message calls the place it is in.
type provisionalBound struct {
	n     *ast.Name
	where string
}

// PassedLength reports whether s has a passed length, CHARACTER*(*), its
// length its actual argument's: a CHARACTER type of length 0 not computed from
// a specification expression.
func (s *Symbol) PassedLength() bool {
	return s.Type.Kind == Character && s.Type.Size == 0 && s.LengthExpr == nil
}

// runtimeLength makes e, an expression that isn't constant, the CHARACTER
// length of sym, which a type statement just declared: a dummy argument's, or
// in a subprogram an automatic object's. A function's own value, whose length
// its caller must know (12.3.1.1), is NOTIMPL until subtask 7.
func (a *analyzer) runtimeLength(sym *Symbol, e ast.Expr) {
	switch {
	case sym.Result || sym.Class == UnitName:
		a.notImplForm(e.Position(), "a function whose value has a CHARACTER length that isn't constant")

	case sym.Dummy == 0 && !a.automaticAllowed():
		a.report("LENNOTDUMMY", e.Position(), sym.Name)

	default:
		adj := &adjustableArray{sym: sym, length: true}

		before := len(adj.names)
		if !a.boundOperands(adj, e) {
			break
		}

		if len(adj.names) == before {
			a.report("BADCHARLEN", e.Position())

			break
		}

		sym.LengthExpr = e
		sym.AutomaticObject = sym.AutomaticObject || sym.Dummy == 0
		a.adjustable = append(a.adjustable, adj)

		return
	}

	sym.broken = true
}

// checkLengths reports the names given a length that isn't constant that
// turned out to be what can't have one: a PARAMETER constant, whose value
// gives its length, or a procedure, whose length its callers must know.
func (a *analyzer) checkLengths() {
	for _, s := range a.unit.Symbols {
		if s.LengthExpr == nil || s.broken {
			continue
		}

		switch {
		case s.Class == Constant:
			a.report("BADCHARLEN", s.LengthExpr.Position())
		case s.Class == Function || s.Class == StmtFunction || s.External || s.Class == Intrinsic:
			a.notImplForm(s.LengthExpr.Position(), "a function whose value has a CHARACTER length that isn't constant")
		default:
			continue
		}

		s.broken = true
	}
}

// autoPos is where an automatic object's message points: its array
// declarator, or for a scalar its type statement.
func (s *Symbol) autoPos() diag.Position {
	if s.Dims != nil {
		return s.DimPos
	}

	return s.TypePos
}

// autoAttr reports an automatic object that something would give storage of
// its own: AUTOATTR for an automatic array, AUTOLENATTR for a CHARACTER name of
// a length that isn't constant and no bounds that aren't.
func (a *analyzer) autoAttr(s *Symbol, pos diag.Position, name, what string) {
	if s.LengthExpr != nil && !s.Adjustable() {
		a.report("AUTOLENATTR", pos, name, what)

		return
	}

	a.report("AUTOATTR", pos, name, what)
}

// lengthText is a specification expression as FORTRAN writes it, for the
// storage map's and the dumps' CHARACTER*(LEN(C)+1): the forms a length may
// take (boundOperands), with every parenthesis the program wrote and no other.
func lengthText(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Name:
		return e.Name
	case *ast.IntConst:
		return e.Text
	case *ast.RealConst:
		return e.Text
	case *ast.Paren:
		return "(" + lengthText(e.X) + ")"
	case *ast.Unary:
		return e.Op.String() + lengthText(e.X)
	case *ast.Binary:
		return lengthText(e.X) + e.Op.String() + lengthText(e.Y)
	case *ast.KeywordArg:
		return e.Name + "=" + lengthText(e.X)
	case *ast.Apply:
		args := make([]string, len(e.Args))
		for i, a := range e.Args {
			args[i] = lengthText(a)
		}

		return e.Name + "(" + strings.Join(args, ", ") + ")"
	}

	return ast.DumpExpr(e)
}
