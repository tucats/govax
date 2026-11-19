package sema

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Internal procedures (the Fortran 90 standard's 12.1.2.2;
// docs/planning/PHASE-103 - Fortran 90 procedures and interfaces.md, findings 2,
// 3, and 6). A main program or a subprogram may end with CONTAINS and one or
// more FUNCTIONs or SUBROUTINEs, its internal procedures:
//
//	      PROGRAM TOTALS
//	      REAL X(3)
//	      DATA X /1.0, 2.0, 3.0/
//	      CALL SHOW(SUM3())
//	      CONTAINS
//	        REAL FUNCTION SUM3()
//	          SUM3 = X(1) + X(2) + X(3)
//	        END FUNCTION
//	        SUBROUTINE SHOW(V)
//	          PRINT *, V
//	        END SUBROUTINE
//	      END PROGRAM
//
// Each internal procedure is a scoping unit of its own, inside its host's
// (scope.go): SUM3 uses its host's X by host association (12.1.2.2.1), as if X
// were its own. A name is the procedure's own if one of its declarations
// declares it (a type statement, a dummy argument, DIMENSION, DATA, ...;
// analyzer.local), hiding any of its host's; any other name it uses is its
// host's if its host has it, wherever in the host the host uses it, and only if
// neither has it is it the procedure's own, with the procedure's implicit type.
//
// That "wherever" is why a unit with internal procedures is analyzed in four
// steps (analyzeUnit): its declarations; then each internal procedure's
// declarations, which are its interface, so that the host's statements can type
// SUM3() and check CALL SHOW(V); then the host's executable statements, which
// enter the names the host uses only there (an undeclared I in a DO); then each
// internal procedure's executable statements, which find all of those. So
//
//	      DO I = 1, 3
//	        CALL STEP
//	      END DO
//	      CONTAINS
//	        SUBROUTINE STEP
//	          DO I = 1, 2
//
// has one I, the host's, and STEP changes the host's loop variable: the
// standard's meaning, and the classic mistake it allows.
//
// An internal procedure's name is its host's (defineInternal), a FUNCTION's
// with the type its own declarations give it, and not the file's: a unit
// elsewhere in the file may have the same name, which the host's procedure
// hides in the host. Its interface is explicit there, so its references are
// checked as no external procedure's can be (checkExplicitCalls). It can't be
// an actual argument (INTERNALARG): it is called only from its host and its
// host's other internal procedures, never once its host has returned.

// Path is the unit's name as the listing and the MACRO file show it: an internal
// procedure's is its host's, a period, and its own (HOST.INNER).
func (u *Unit) Path() string {
	if u.LinkName != "" {
		return u.LinkName
	}

	if u.Host != nil {
		return u.Host.Name + "." + u.Name
	}

	return u.Name
}

// namelist is the NAMELIST group named name, the unit's own or by host
// association its host's, or nil.
func (u *Unit) namelist(name string) *Namelist {
	for ; u != nil; u = u.Host {
		if n := u.namelists[name]; n != nil {
			return n
		}
	}

	return nil
}

// structure is the structure named name, the unit's own or its host's, or nil.
func (u *Unit) structure(name string) *Structure {
	for ; u != nil; u = u.Host {
		if st := u.structs[name]; st != nil {
			return st
		}
	}

	return nil
}

// local returns the symbol named name for a declaration of it: in an internal
// procedure, a name of its own, made now if its declarations haven't declared
// it before, whatever its host has; anywhere else the name as lookup finds it.
func (a *analyzer) local(name string, pos diag.Position) *Symbol {
	if a.unit.Host == nil || a.scope != a.unit.scope || a.scope.byName[name] != nil {
		return a.lookup(name, pos)
	}

	return a.newName(name, pos)
}

// innerAnalyzer is an analyzer for one of this unit's internal procedures, with
// the options the unit started with.
func (a *analyzer) innerAnalyzer() *analyzer {
	ia := &analyzer{
		messages:       a.messages,
		prog:           a.prog,
		units:          a.units,
		defaultInt:     a.defaultInt,
		defaultLogical: a.defaultLogical,
		gFloating:      a.gFloating,
		noF77:          a.noF77,
		f90:            a.f90,
		recursive:      a.recursive,
		align:          a.baseAlign,
		baseAlign:      a.baseAlign,
		warnings:       a.warnings,
		standard:       a.standard,
		checks:         a.checks,
		host:           a,
	}
	ia.makeMaps()

	return ia
}

// internalInterfaces is the second step: each internal procedure's header and
// declarations, analyzed by an analyzer of its own, and its name made the
// host's. It returns those analyzers, for the fourth step.
func (a *analyzer) internalInterfaces(au *ast.Unit) []*analyzer {
	if len(au.Contains) == 0 {
		return nil
	}

	inner := make([]*analyzer, 0, len(au.Contains))

	for _, cu := range au.Contains {
		if _, bad := cu.Header.(*ast.BadStmt); bad {
			continue
		}

		if pending := pendingHeader(cu); pending != nil {
			a.messages.Report("NOTIMPL", pendingPosition(pending), statementWhat(pending))

			continue
		}

		module := a.unit.Kind == ast.ModuleUnit
		if module && hasEntry(cu) {
			a.notImplForm(cu.Position(), "ENTRY in a module procedure")

			continue
		}

		ia := a.innerAnalyzer()
		ia.interfacing = true
		ia.beginUnit(cu, cu.Name())
		ia.interfacing = false

		// A module's procedure is global, by its module's name and its own
		// (Decision 10; module.go).
		if module {
			ia.unit.LinkName = LongName(a.unit.Name + "." + cu.Name())

			if a.reading != nil && a.reading.links[cu.Name()] != "" {
				ia.unit.LinkName = a.reading.links[cu.Name()]
			}
		}

		a.defineInternal(ia.unit)
		inner = append(inner, ia)
	}

	return inner
}

// hasEntry reports whether the unit au has an ENTRY statement.
func hasEntry(au *ast.Unit) bool {
	for _, s := range au.Body {
		if _, ok := s.(*ast.EntryStmt); ok {
			return true
		}
	}

	return false
}

// defineInternal makes the internal procedure iu's name a name of its host,
// standing for the procedure, unless the host has the name already: as its own
// name or another internal procedure's (DUPNAME), or declared by a type
// statement, EXTERNAL, or INTRINSIC, which would give the procedure a second
// interface (DUPINTERFACE).
func (a *analyzer) defineInternal(iu *Unit) {
	pos := iu.AST.Position()

	if old := a.scope.byName[iu.Name]; old != nil {
		switch {
		case old.Internal == nil && old.Class != UnitName && !old.Result && len(old.Dims) == 0 &&
			(old.Class == Intrinsic || old.External || old.Interface != nil || (old.Typed && old.Class == Unclassified)):
			a.report("DUPINTERFACE", old.Pos, old.Name, pos.Line)
		default:
			a.report("DUPNAME", pos, iu.Name, a.describe(old), old.Pos.Line)
		}

		old.broken = true

		return
	}

	s := &Symbol{Name: iu.Name, Class: Subroutine, Offset: -1, Pos: pos, Internal: iu, Typed: true, LinkName: iu.LinkName}

	if iu.Kind == ast.FunctionUnit && iu.Result != nil {
		s.Class, s.Type = Function, iu.Result.Type
	}

	a.scope.enter(s)
	a.unit.Contains = append(a.unit.Contains, iu)
}

// checkProvisional reports each name an internal procedure's declarations used
// before its host had: one the host's executable statements went on to use is
// the host's by the standard's rule, which these declarations, read first,
// couldn't know (finding 2's step 2). Bounds and lengths from such a name aren't
// compiled; a name the host never uses stays the procedure's own.
func (a *analyzer) checkProvisional(inner []*analyzer) {
	for _, ia := range inner {
		for _, s := range ia.provisional {
			if a.scope.byName[s.Name] != nil {
				a.notImplForm(s.Pos, "a name its host uses only in executable statements, in an internal procedure's declarations")
			}
		}

		// A bound's name the host never uses is the procedure's own variable,
		// which a bound can't use.
		for _, b := range ia.provisionalBounds {
			if a.scope.byName[b.n.Name] == nil {
				ia.report("BOUNDVAR", b.n.Pos, b.n.Name, b.where)
			}
		}
	}
}

// hostName reports whether the name is one this internal procedure has by host
// association: its host's (or its host's host's), not its own.
func (a *analyzer) hostName(name string) bool {
	_, sc := a.scope.findIn(name)

	return sc != nil && sc.unit != nil && sc.unit != a.unit
}

// checkExplicitCalls checks each reference through an explicit interface, in
// the unit and in its internal procedures, against the interface: an internal
// procedure's, or an interface body's (interface.go). It is as checkReferences
// checks a reference to another unit of the file, and closer, since the
// interface is explicit (finding 6).
func (a *analyzer) checkExplicitCalls() {
	units := append([]*Unit{a.unit}, a.unit.Contains...)

	for _, u := range units {
		if u.incomplete {
			continue
		}

		for _, r := range u.calls {
			target := explicitInterface(r.sym)

			// A recursive internal procedure's call of itself.
			if target == nil && u.Host != nil && (r.sym.Class == UnitName || r.sym.Result) && r.sym.Name == u.Name {
				target = u
			}

			if target != nil && !target.incomplete && !r.sym.broken {
				checkInterface(r, target, a.messages)
			}
		}
	}
}

// checkInterface checks the reference r against the explicit interface of the
// procedure target: the number of arguments and the places of alternate
// returns, and each argument's type and kind (ARGTYPE: they must be the dummy's,
// 12.4.1.1), rank (ARGRANK: a scalar for a scalar dummy, and for an array dummy
// an array or an element, sequence association, 12.4.1.4), CHARACTER length
// where both are known (ARGLENGTH: the dummy's no longer), and, for an
// INTENT(OUT) or INTENT(INOUT) dummy, that it is a variable (NOTDEFINABLE).
func checkInterface(r routineRef, target *Unit, messages *diag.List) {
	fu := &fileUnit{kind: target.Kind, pos: target.AST.Position(), unit: target}
	line := fu.pos.Line

	// A use-associated procedure's interface is in its module's text: the
	// line that gave it here is the USE's.
	if r.sym.Use != nil {
		line = r.sym.Pos.Line
	}

	n, given := len(target.Params)+len(target.AltPlaces), len(r.args)+len(r.altPlaces)
	if n != given {
		messages.Report("ARGCOUNT", r.pos, r.sym.Name, count(n, "dummy argument"), line, given)

		return
	}

	if !equalInts(target.AltPlaces, r.altPlaces) {
		messages.Report("ALTPLACE", r.pos, r.sym.Name, line)

		return
	}

	constantArguments(r, fu, messages)
	recordArguments(r, fu, messages)

	for i, arg := range r.args {
		p := target.Params[i]

		if ident, rest := argumentMismatch(arg, p); ident != "" {
			messages.Report(ident, arg.Expr.Position(), append(append([]any{i + 1, r.sym.Name}, rest...), line)...)

			continue
		}

		// An argument with an error has had its message.
		if arg.Expr == nil || arg.Expr.Type().Kind == Invalid || p.broken || arg.Mode != ByDefault || arg.Hollerith ||
			arg.Typeless || p.DummyProcedure() {
			continue
		}

		given, wanted := arg.Expr.Type(), p.Type
		rank := argumentRank(arg.Expr)

		if given.Kind == Character && wanted.Kind == Character && rank == 0 && len(p.Dims) == 0 && given.Size > 0 && wanted.Size > given.Size {
			messages.Report("ARGLENGTH", arg.Expr.Position(), i+1, r.sym.Name, given.Size, p.Name, wanted.Size, line)

			continue
		}

		// An assumed-shape dummy's elements are its actual's: the lengths must
		// be the same (12.4.1.1), where both are known.
		if given.Kind == Character && p.AssumedShape() && given.Size > 0 && wanted.Size > 0 && wanted.Size != given.Size {
			messages.Report("ARGLENGTH", arg.Expr.Position(), i+1, r.sym.Name, given.Size, p.Name, wanted.Size, line)

			continue
		}

		if p.Intent == "OUT" || p.Intent == "INOUT" {
			switch base := definedSymbol(arg.Expr); {
			case arg.Copy || !definable(arg.Expr):
				messages.Report("NOTDEFINABLE", arg.Expr.Position(), i+1, r.sym.Name, p.Name, p.Intent, line)
			case base != nil && base.Dummy != 0 && base.Intent == "IN" && !base.broken:
				messages.Report("INTENTIN", arg.Expr.Position(), base.Name)
			}
		}
	}
}

// equalInts reports whether two lists of numbers are the same.
func equalInts(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}

	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}

	return true
}

// argumentRank is an actual argument's rank: a whole array's, passed as itself,
// or an expression's.
func argumentRank(e Expr) int {
	switch x := e.(type) {
	case *ArrayRef:
		return len(x.Sym.Dims)

	case *FieldRef:
		// An array field without subscripts is an array name reference.
		if x.WholeArray() {
			return len(x.Field().Dims)
		}
	}

	return rankOf(e)
}

// isElement reports whether an actual argument is an array element, or a
// substring of one, which an array dummy takes by sequence association.
func isElement(e Expr) bool {
	switch e := e.(type) {
	case *Elem:
		return true
	case *Substring:
		return isElement(e.X)
	case *FieldRef:
		return len(e.Subs) > 0
	}

	return false
}

// definable reports whether an actual argument is a variable, which a
// procedure may give a value: a variable, an array, an element, a substring of
// one, or a record's field.
func definable(e Expr) bool {
	switch e := e.(type) {
	case *Var:
		return e.Sym.Class != Constant
	case *Elem, *ArrayRef, *FieldRef:
		return true
	case *Section:
		// One with a vector subscript isn't (12.4.1.1).
		for _, sub := range e.Subs {
			if sub.Vector != nil {
				return false
			}
		}

		return true
	case *Substring:
		return definable(e.X)
	}

	return false
}

// rankText says what a value of the rank is: "a scalar", "an array of rank 2".
func rankText(rank int) string {
	if rank == 0 {
		return "a scalar"
	}

	return "an array of rank " + strconv.Itoa(rank)
}
