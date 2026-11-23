package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// NULL([MOLD]), Fortran 95's disassociated pointer (the HP manual's 9.4.111;
// docs/planning/PHASE-112 - Fortran 95.md, finding 3).
//
//	INTEGER, POINTER :: P => NULL()    ! P starts disassociated (and is SAVE'd)
//	DATA Q /NULL()/                    ! so does Q
//	P => NULL()                        ! the same as NULLIFY (P)
//	CALL WALK(NULL())                  ! a pointer dummy given a disassociated pointer
//	N = NODE(5, NULL())                ! a pointer component disassociated
//
// NULL isn't a row of the intrinsic table, since its type isn't decided by its
// arguments: a reference's type, kind, and rank are those of the pointer whose
// value it gives (the manual's table: the left side of =>, the entity being
// initialized, the component, the dummy argument). So it is read only where a
// pointer's value goes, and those places each ask for it by name
// (nullReference), with the pointer it is for; anywhere else it is refused,
// NULLPLACE. MOLD, a pointer of any association status, says what NULL is a
// pointer to where the context doesn't; where it does, MOLD must agree with it
// (POINTERTYPE).
//
// Where its value goes it is Phase 107's NULLIFY: a scalar pointer's longword 0,
// a CHARACTER or array pointer's POINTER longword 0. A pointer assignment of it
// is a Nullify statement; an initial value needs no bytes at all, a static
// pointer's storage starting at 0 already (the IR's finding 2), but makes the
// pointer static (Symbol.nullInit); and in a constructor or an argument it is a
// Null, which lowering turns into that 0.

// Null is a reference to NULL in a place where a pointer's value goes: a
// structure constructor's pointer component, or the actual argument of a
// pointer dummy (Arg.Pointer). T is that pointer's type and Rank its rank, so
// lowering knows what disassociated pointer to make.
type Null struct {
	exprBase
	Rank int

	// open says a reference with no MOLD, in a generic reference's
	// arguments before a specific is chosen (genericNull): a pointer of any
	// kind, which fits any pointer dummy.
	open bool
}

// nullReference reports whether x is a reference to the intrinsic function
// NULL, NULL() or NULL(MOLD), and if so gives it. NULL is the intrinsic's in
// a place where a pointer's value goes whatever /F90 says (the author's answer
// 2), unless the unit has made the name something else: a variable, a dummy, or
// an external procedure (a function of the file's own named NULL is the
// intrinsic's unless declared EXTERNAL, as for every intrinsic name).
func (a *analyzer) nullReference(x ast.Expr) (*ast.Apply, bool) {
	app, ok := x.(*ast.Apply)
	if !ok || app.Name != "NULL" {
		return nil, false
	}

	s := a.scope.find("NULL")

	switch {
	case s == nil, s.Class == Intrinsic:
		return app, true
	case s.Class == Unclassified && s.Dummy == 0 && !s.External && s.Dims == nil:
		return app, true
	}

	return nil, false
}

// nullValue analyzes the reference to NULL app as the value of the pointer
// name (for messages), which points at values of type pt and rank rank: its one
// argument, MOLD, if given, must be a pointer (NOTPOINTER) pointing at the same
// (POINTERTYPE). nil after a message.
func (a *analyzer) nullValue(app *ast.Apply, name string, pt Type, rank int) *Null {
	mold, ok := a.nullMold(app)
	if !ok {
		return nil
	}

	if mold != nil {
		m := a.pointerObject(mold, false)
		if m == nil || !a.pointerMatches(name, pt, rank, m, mold.Position()) {
			return nil
		}
	}

	return &Null{exprBase: exprBase{T: pt, Pos: app.Pos}, Rank: rank}
}

// genericNull analyzes the reference to NULL app as an actual argument of a
// generic name, before its specific is chosen (resolveGeneric): of MOLD's type
// and rank, so it fits only a pointer dummy of those, or with no MOLD open,
// fitting any pointer dummy. nil after a message.
func (a *analyzer) genericNull(app *ast.Apply) *Null {
	mold, ok := a.nullMold(app)
	if !ok {
		return nil
	}

	if mold == nil {
		return &Null{exprBase: exprBase{T: invalid, Pos: app.Pos}, open: true}
	}

	m := a.pointerObject(mold, false)
	if m == nil {
		return nil
	}

	return &Null{exprBase: exprBase{T: m.Type(), Pos: app.Pos}, Rank: targetRank(m)}
}

// nullMold is the reference to NULL app's MOLD argument, nil if it has none;
// false after a message about its arguments: more than one (INTRINARGS), a null
// one (NULLINTRINARG), or a keyword but MOLD (KEYWORDNAME).
func (a *analyzer) nullMold(app *ast.Apply) (ast.Expr, bool) {
	var mold ast.Expr

	for i, arg := range app.Args {
		k, isKeyword := arg.(*ast.KeywordArg)

		switch {
		case arg == nil:
			a.report("NULLINTRINARG", app.Pos, "NULL")

			return nil, false
		case i > 0:
			a.report("INTRINARGS", app.Pos, "NULL", "0 or 1 arguments", len(app.Args))

			return nil, false
		case !isKeyword:
			mold = arg
		case k.Name == "MOLD":
			mold = k.X
		default:
			a.report("KEYWORDNAME", arg.Position(), "the intrinsic function NULL", k.Name, "MOLD")

			return nil, false
		}
	}

	return mold, true
}

// nullElsewhere reports a reference to NULL where no pointer's value goes,
// NULLPLACE, giving the expression that stands for it after the error.
func (a *analyzer) nullElsewhere(pos diag.Position) Expr {
	a.report("NULLPLACE", pos)

	return a.bad(pos)
}
