package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
)

// parameter defines the constant p=c of a PARAMETER statement (section
// 4.11, appendix A.4): p's value is the constant expression c's, and
// writing p anywhere a constant may go is writing the value.
//
// The two forms differ in p's type. In the standard one, PARAMETER
// (p=c), p has the type its declaration or its first letter gives it,
// which must come before the PARAMETER statement (TYPEAFTER), and c is
// converted to it as an assignment would convert it: PARAMETER (MU=1.23)
// makes MU the INTEGER 1, unless MU is declared REAL. A CHARACTER*(*)
// constant takes its value's length. In appendix A.4's, PARAMETER p=c,
// p has c's own type, which no declaration may give it (OLDPARAMTYPE) and
// its first letter doesn't change.
//
// An octal or hexadecimal constant as the whole of c takes p's type
// (section 2.2.1.4: an assignment's other operand), or INTEGER*4 in the
// old form.
func (a *analyzer) parameter(s *ast.ParameterStmt, d *ast.ParamDef) {
	sym := a.lookup(d.Name.Name, d.Name.Pos)
	ev := a.declEval()

	// keep is whether the name becomes this constant: not if it is one
	// already, nor if it is something else. Its value gets its messages
	// either way.
	keep := false

	switch {
	case sym.Class == Constant:
		a.report("DUPPARAM", d.Name.Pos, sym.Name, sym.defPos.Line)

	case sym.Class == UnitName || sym.Result || sym.Class == Array || sym.Class == StmtFunction ||
		sym.External || sym.Class == Intrinsic:
		a.badUse(sym, d.Name.Pos, "be a constant")

	case sym.Dummy != 0:
		a.report("BADUSE", d.Name.Pos, sym.Name, "a dummy argument", "be a constant")

	case s.Old && sym.Typed:
		a.report("OLDPARAMTYPE", d.Name.Pos, sym.Name, "type statement", sym.TypePos.Line)

		sym.broken = true

	default:
		keep = true

		// Whatever is wrong with its value, the name is a constant: its
		// uses are constants' (quiet ones, with no value), not
		// variables'.
		sym.Class, sym.defPos, sym.oldForm = Constant, d.Name.Pos, s.Old
	}

	if s.Old {
		c, err := ev.eval(d.Value)

		switch {
		case err != nil:
			a.notConstant(err)
		case keep:
			// The constant's type is its value's, whatever its first
			// letter.
			sym.Type, sym.untyped = c.T, false
			sym.Value = c.at(d.Name.Pos)
		}

		return
	}

	t := sym.Type

	var (
		c   *Const
		err *constErr
	)

	if tc, ok := d.Value.(*ast.TypelessConst); ok && t.Kind != Character && t.Kind != Invalid {
		c, err = ev.typelessAs(tc, t)
	} else if c, err = ev.eval(d.Value); err == nil && keep && t.Kind != Invalid {
		v := c
		if c, err = ev.convert(c, t, d.Value.Position()); err == incompatible {
			a.report("PARAMTYPE", d.Value.Position(), sym.Name, t.String(), article(v.T.String()))

			return
		}
	}

	switch {
	case err != nil:
		a.notConstant(err)

		return

	case !keep || t.Kind == Invalid || sym.broken:
		// Not this constant; or one with no type (NOTYPE, under
		// IMPLICIT NONE, says so) or a bad one.
		return
	}

	if t.Kind == Character && t.Size == 0 {
		// CHARACTER*(*): the value's length (section 4.4.2's STRING).
		sym.Type = c.T
	}

	sym.Value = c.at(d.Name.Pos)
}

// notConstant reports a PARAMETER value that isn't a constant expression
// (NOTCONST), where the part that isn't constant is, unless its error is
// reported already.
func (a *analyzer) notConstant(err *constErr) {
	if err.kind != constFailed {
		a.report("NOTCONST", err.pos, err.why)
	}
}

// constantUse is a constant's value where an executable statement's
// expression uses its name. A constant whose type the code generator
// doesn't handle yet is NOTIMPL, once (Phase 78's); CHARACTER constants
// are values wherever a character constant is.
func (a *analyzer) constantUse(s *Symbol, n *ast.Name) Expr {
	if s.Value == nil {
		return a.bad(n.Pos)
	}

	if t := s.Type; !t.Compiled() && t.Kind != Character {
		a.notImpl(n.Pos, article(t.String()+" constant"))

		s.broken = true

		return a.bad(n.Pos)
	}

	return s.Value.at(n.Pos)
}
