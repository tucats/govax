package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// PURE procedures (the Fortran 95 standard's 12.6; the HP Fortran manual's
// 8.5.1.2; docs/planning/PHASE-112 - Fortran 95.md, subtask 6). A PURE
// procedure is one with no side effects the caller could see: what it does
// it does to its own locals, its function value, and the dummy arguments it
// says it changes. So it can be called anywhere in any order, which FORALL
// and ELEMENTAL procedures count on:
//
//	PURE INTEGER FUNCTION SQ(X)
//	  INTEGER, INTENT(IN) :: X
//	  SQ = X * X
//	END FUNCTION SQ
//
// PURE changes no code. What the compiler checks are the standard's
// constraints, each where it can see it:
//
//   - every dummy argument says its INTENT, and a function's says IN; a
//     pointer or a dummy procedure need not (PUREINTENT);
//   - no local is saved or given an initial value, which would make it
//     outlive the call (NOTPURE), and there is no SAVE with no list;
//   - nothing of the caller's world is changed: a variable in a common block,
//     a module's variable, or a host's (a "global" here) is never given a
//     value, allocated, or given a pointer association, never made a
//     pointer's target, and never passed to an INTENT(OUT) or INTENT(INOUT)
//     dummy or to a pointer dummy (NOTPURE). An INTENT(IN) dummy is already
//     INTENTIN's (intent.go);
//   - no external I/O (a READ or WRITE of a unit, PRINT, TYPE, ACCEPT, OPEN,
//     CLOSE, INQUIRE, and the positioning statements; an internal file,
//     ENCODE, and DECODE are fine), and no STOP or PAUSE;
//   - every procedure referenced is PURE, which only an explicit interface
//     can say (12.3.1.1): an internal or module procedure, an interface
//     body, or the procedure itself. Every intrinsic function is PURE, and of
//     the intrinsic subroutines only MVBITS; the system subroutines and
//     functions (DATE, RAN, ...) aren't.
//
// An interface body that says PURE gets the first check alone: it has no
// statements. An ELEMENTAL procedure is PURE too.
//
// Two rules sit elsewhere: an interface body that says PURE for a procedure
// whose definition doesn't is INTERFACEDIFF (interface.go), and a procedure
// given to a PURE dummy procedure must be PURE itself (PUREACTUAL,
// checkInterface). A PURE function may also stand in an automatic object's
// bound or length, a specification function (7.1.6.1; arrays.go's
// boundFunction).

// checkPure checks a PURE unit's dummy arguments, locals, and references, once
// the unit has been analyzed.
func (a *analyzer) checkPure() {
	if !a.unit.Pure {
		return
	}

	a.pureDummies()

	if a.inBody {
		return
	}

	a.pureLocals()
	a.pureReferences()
}

// pureDummies reports each dummy argument without the INTENT a PURE procedure
// must give it (PUREINTENT).
func (a *analyzer) pureDummies() {
	function := a.unit.Kind == ast.FunctionUnit
	kind := "subroutine"

	if function {
		kind = "function"
	}

	for _, s := range a.unit.Dummies {
		if s.broken || s.Pointer || s.DummyProcedure() {
			continue
		}

		switch {
		case function && s.Intent != "IN":
			a.report("PUREINTENT", s.Pos, s.Name, kind, a.unit.Name, "INTENT(IN)")
		case !function && s.Intent == "":
			a.report("PUREINTENT", s.Pos, s.Name, kind, a.unit.Name, "with an INTENT")
		}
	}
}

// pureLocals reports each local that would outlive a call: saved, STATIC, or
// given an initial value (which implies SAVE), and a SAVE with no list.
func (a *analyzer) pureLocals() {
	if a.saveAll {
		a.report("NOTPURE", a.saveAllPos, "a SAVE statement", "be used")
	}

	initial := map[*Symbol]bool{}
	for _, in := range a.unit.Initials {
		initial[in.Sym] = true
	}

	for _, s := range a.unit.Symbols {
		if s.Dummy != 0 || s.External || s.broken || s.Use != nil || s.Common != nil || s.Result ||
			s.Class != Variable && s.Class != Array && s.Class != Unclassified {
			continue
		}

		var what string

		switch {
		case initial[s] || s.nullInit || s.recordValue != nil || s.recordValues != nil:
			what = "have an initial value"
		case s.saved:
			what = "be saved"
		case s.storageStmt != nil && s.storageStmt.Kind == lex.Static:
			what = "be STATIC"
		default:
			continue
		}

		a.report("NOTPURE", s.Pos, s.Name+", a local variable,", what)
	}
}

// pureReferences reports each procedure the unit references that isn't
// declared PURE, and each global passed where the procedure may change it.
func (a *analyzer) pureReferences() {
	for _, r := range a.unit.calls {
		if r.sym.broken {
			continue
		}

		u := a.interfaceOf(r.sym)
		if u == nil || !u.Pure {
			verb := "be referenced"
			if r.sym.Class == Subroutine || r.sym.Class == UnitName && a.unit.Kind == ast.SubroutineUnit {
				verb = "be called"
			}

			a.report("NOTPURE", r.pos, r.sym.Name+", not declared PURE,", verb)

			continue
		}

		for i, arg := range r.args {
			if i >= len(u.Params) || arg.Expr == nil {
				continue
			}

			switch p := u.Params[i]; {
			case p.Pointer:
				a.pureDefines(pointerOwner(arg.Expr), arg.Expr.Position(), "be passed to a pointer dummy argument")
			case p.Intent == "OUT" || p.Intent == "INOUT":
				a.pureDefines(definedSymbol(arg.Expr), arg.Expr.Position(),
					"be passed to an INTENT("+p.Intent+") dummy argument")
			}
		}
	}
}

// pureDefines reports NOTPURE if the unit is PURE and s, which the statement
// at pos changes as verb says ("be given a value"), is a global: a host's
// variable, a module's, or one in a common block.
func (a *analyzer) pureDefines(s *Symbol, pos diag.Position, verb string) {
	if s == nil || s.broken || !a.unit.Pure || a.readFrom {
		return
	}

	if why := a.globalWhy(s); why != "" {
		a.report("NOTPURE", pos, s.Name+", "+why+",", verb)
	}
}

// globalWhy says what makes s a global for a PURE procedure ("a variable of
// the module M"), or "" if it is the procedure's own.
func (a *analyzer) globalWhy(s *Symbol) string {
	if s.Use != nil {
		return "a variable of the module " + s.Use.Module
	}

	if found, sc := a.scope.findIn(s.Name); found == s && sc != nil && sc.unit != nil && sc.unit != a.unit {
		if sc.unit.Kind == ast.ModuleUnit {
			return "a variable of the module " + sc.unit.Name
		}

		return "a variable of its host " + sc.unit.Name
	}

	switch b := s.Common; {
	case b == nil:
		return ""
	case b.Module != "":
		return "a variable of the module " + b.Module
	case b.Name == "":
		return "a variable in blank common"
	default:
		return "a variable in common block /" + b.Name + "/"
	}
}

// pointerOwner is the variable whose storage holds the pointer p (a reference
// to a pointer): a named pointer's own symbol, or pointerHome's.
func pointerOwner(p Expr) *Symbol {
	if s := PointerSymbol(p); s != nil && s.Component == nil {
		return s
	}

	return pointerHome(p)
}

// pureStatement reports a statement a PURE procedure may not have: STOP,
// PAUSE, and external I/O.
func (a *analyzer) pureStatement(st Stmt) {
	if !a.unit.Pure || st == nil {
		return
	}

	var what string

	switch st := st.(type) {
	case *Stop:
		what = "a STOP statement"
		if st.Pause {
			what = "a PAUSE statement"
		}

	case *IO:
		if st.Internal || st.Code {
			return
		}

		what = article(st.AST.Base().Kind.String()) + " statement of a unit"

	case *FileStmt, *DefineFile, *IOLength:
		what = article(st.Source().Base().Kind.String()) + " statement"

	default:
		return
	}

	a.report("NOTPURE", st.Source().Base().Pos, what, "be used")
}

// pureSystem reports a reference to the system or intrinsic routine name, which
// isn't PURE, in a PURE procedure; what says which it is ("the system
// subroutine"), and verb how it is used.
func (a *analyzer) pureSystem(what, name string, pos diag.Position, verb string) {
	if a.unit.Pure {
		a.report("NOTPURE", pos, what+" "+name, verb)
	}
}

// pureProcedureArgument reports PUREACTUAL if arg, an actual argument, is a
// procedure not declared PURE given to p, a dummy procedure whose interface
// says PURE. An intrinsic function is PURE.
func pureProcedureArgument(arg Arg, p *Symbol, messages *diag.List) {
	pu := explicitInterface(p)
	if pu == nil || !pu.Pure || !p.DummyProcedure() {
		return
	}

	proc, ok := arg.Expr.(*Procedure)
	if !ok || proc.Sym == nil || proc.Sym.broken {
		return
	}

	if u := explicitInterface(proc.Sym); u == nil || !u.Pure {
		messages.Report("PUREACTUAL", proc.Pos, proc.Name, p.Name)
	}
}

// specificationFunction reports whether s is a function a specification
// expression, an automatic object's bound or length, may reference (the Fortran
// 95 standard's 7.1.6.1): PURE by an explicit interface the declarations have
// already given (a module procedure's, or an interface body's), with a scalar
// value, and neither an internal procedure, a statement function, nor a dummy
// procedure. Its value is computed at each entry, before the unit's first
// statement, as any other bound's.
//
// The standard also forbids a recursive one. govax doesn't: /RECURSIVE makes
// every unit recursive, and a module's text says RECURSIVE for each of its
// procedures then, so the rule would make a compiling option change what is
// legal; and a call at entry is a call like any other (ours, unconfirmed).
func (a *analyzer) specificationFunction(s *Symbol) bool {
	if s == nil || s.broken || s.Dummy != 0 || s.Generic != nil || s.Internal != nil && !isModuleProcedure(s) {
		return false
	}

	u := explicitInterface(s)

	return u != nil && u.Pure && u.Kind == ast.FunctionUnit && u.Result != nil &&
		len(u.Result.Dims) == 0
}
