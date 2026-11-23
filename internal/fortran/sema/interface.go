package sema

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Interface blocks (the Fortran 90 standard's 12.3.2.1; docs/planning/PHASE-103
// - Fortran 90 procedures and interfaces.md, finding 7). An interface block
// with no generic specification describes procedures defined elsewhere:
//
//	      PROGRAM ROOTS
//	      INTERFACE
//	        REAL FUNCTION SOLVE(F, X0, TOL)
//	          INTERFACE
//	            REAL FUNCTION F(X)
//	              REAL X
//	            END FUNCTION
//	          END INTERFACE
//	          REAL X0, TOL
//	        END FUNCTION
//	      END INTERFACE
//	      ...
//	      R = SOLVE(G, 1.0, 1.0E-6)
//
// Each interface body is a FUNCTION's or SUBROUTINE's header and
// declarations, with no executable part: the procedure's interface, which the
// unit holding the block now knows explicitly, as it knows an internal
// procedure's (internal.go). A reference to SOLVE is checked against it as a
// reference to an internal procedure is (checkInterface): how many arguments,
// and each one's type, kind, rank, length, and whether it can be given a
// value. SOLVE's type is the body's, whatever the unit's IMPLICIT rules say.
//
// A body is a scope with no host (12.1.2.2.1 gives host association only to
// internal procedures and module procedures): its names are its own, typed by
// the default implicit rules and its own IMPLICIT statements, and nothing of
// the unit holding the block reaches it. So a body can't use the unit's
// PARAMETER for a kind or a bound.
//
// What the body names is one of two things (12.3.2.1): a dummy argument of the
// unit holding the block, which becomes a dummy procedure with that interface
// (F, inside SOLVE's body, is SOLVE's dummy procedure), or otherwise an
// external procedure. A body may not name the unit holding it (INTERFACESELF)
// or anything else the unit declares (INTERFACEDECL): a procedure has one
// explicit interface in a scoping unit, and a body gives all of its
// characteristics, its type among them.
//
// The file may hold the external procedure itself. Its definition and the body
// must then agree (12.3.2.1's "consistent"), which checkReferences compares,
// naming the first difference (INTERFACEDIFF). And a procedure the file defines
// with an OPTIONAL dummy can be referenced only through an explicit interface
// (12.3.1.1), which a reference through an implicit one breaks (EXPLICITREQ).
//
// A generic interface block (INTERFACE SWAP) is generic.go's, and one for an
// operator or assignment operators.go's; their bodies are read here as any
// others, and make external procedures too. MODULE PROCEDURE waits for Phase
// 106.

// interfaceWhat names an interface block this phase doesn't compile, for its
// NOTIMPL message: one with MODULE PROCEDURE statements (Phase 106).
func interfaceWhat(*ast.InterfaceBlock) string {
	return "the MODULE PROCEDURE statement"
}

// interfaceBodies analyzes the bodies of the unit's interface blocks, which
// the declarations walk gathered (a.interfaces), once every other declaration
// is read, so that a type statement or EXTERNAL for a body's name is known
// wherever in the declarations it is.
func (a *analyzer) interfaceBodies() {
	for _, b := range a.interfaces {
		var specifics []*Symbol

		for _, bu := range b.Bodies {
			if _, bad := bu.Header.(*ast.BadStmt); bad {
				continue
			}

			if pending := pendingHeader(bu); pending != nil {
				a.notImplForm(pendingPosition(pending), statementWhat(pending))

				continue
			}

			ba := a.bodyAnalyzer()
			body := ba.beginUnit(bu, bu.Name())
			ba.untyped()

			a.unit.Interfaces = append(a.unit.Interfaces, body)

			if s := a.declareInterface(body); s != nil {
				specifics = append(specifics, s)
			}
		}

		// A module's own procedures are known only once its declarations are
		// read (analyzeContained), so its MODULE PROCEDURE statements wait.
		if len(b.ModuleProcedures) > 0 && a.unit.Kind == ast.ModuleUnit {
			a.waiting = append(a.waiting, waitingBlock{b, specifics})

			continue
		}

		a.declareBlock(b, append(specifics, a.moduleProcedures(b)...))
	}

	if len(a.waiting) == 0 {
		a.checkGenerics()
		a.checkOperators()
	}
}

// waitingBlock is a module's interface block with MODULE PROCEDURE statements,
// and the specifics its bodies gave, waiting for the module's procedures.
type waitingBlock struct {
	b         *ast.InterfaceBlock
	specifics []*Symbol
}

// declareBlock gives the block's generic name or operator the specifics.
func (a *analyzer) declareBlock(b *ast.InterfaceBlock, specifics []*Symbol) {
	switch b.Generic.Kind {
	case ast.GenericName:
		a.declareGeneric(b.Generic, specifics)
	case ast.GenericOp, ast.GenericAssign:
		a.declareOperator(b.Generic, specifics)
	}
}

// moduleProcedureBlocks declares a module's waiting interface blocks, now that
// its procedures are known, and checks its generic names and operators.
func (a *analyzer) moduleProcedureBlocks() {
	if len(a.waiting) == 0 {
		return
	}

	for _, w := range a.waiting {
		a.declareBlock(w.b, append(w.specifics, a.moduleProcedures(w.b)...))
	}

	a.waiting = nil

	a.checkGenerics()
	a.checkOperators()
}

// moduleProcedures are the procedures the block's MODULE PROCEDURE statements
// name (R1205, 12.3.2.1): each a module procedure this unit can reach, its
// module's own or one a USE gave it (MODPROCNAME otherwise); and a block with
// them must have a generic specification (MODPROCGENERIC).
func (a *analyzer) moduleProcedures(b *ast.InterfaceBlock) []*Symbol {
	var list []*Symbol

	for _, m := range b.ModuleProcedures {
		if b.Generic.Kind == ast.NoGeneric {
			a.report("MODPROCGENERIC", m.Position())

			continue
		}

		for _, n := range m.Names {
			s := a.scope.find(n.Name)

			switch {
			case s == nil:
				a.report("MODPROCNAME", n.Pos, n.Name, "isn't known here")
			case s.broken:
			case !isModuleProcedure(s) || explicitInterface(s) == nil:
				a.report("MODPROCNAME", n.Pos, n.Name, "is "+a.describe(s))
			default:
				a.note(&s.refs, n.Pos, RefUsed)
				list = append(list, s)
			}
		}
	}

	return list
}

// bodyAnalyzer is an analyzer for an interface body of this unit: a unit's
// options, but no host.
func (a *analyzer) bodyAnalyzer() *analyzer {
	ba := a.innerAnalyzer()
	ba.host = nil
	ba.inBody = true

	return ba
}

// declareInterface gives the name of the interface body bu its interface in
// this unit: a dummy argument's, making it a dummy procedure, or else a new
// name of the unit's, an external procedure. It returns the name's symbol, or
// nil after a message.
func (a *analyzer) declareInterface(bu *Unit) *Symbol {
	pos := bu.AST.Position()
	old := a.scope.byName[bu.Name]

	switch {
	case old == nil:
		old = &Symbol{Name: bu.Name, Offset: -1, Pos: pos, External: true}
		a.scope.enter(old)

	case old.broken:
		return nil

	case old.Class == UnitName || old.Result || old.ResultVar != nil:
		// The standard's constraint on R1201: a subprogram's own procedure
		// (its name, or an entry point's) has its interface already.
		a.report("INTERFACESELF", pos, bu.Name)

		return nil

	case !plainDummy(old):
		a.report("INTERFACEDECL", pos, bu.Name, a.describe(old), old.Pos.Line)
		old.broken = true

		return nil
	}

	old.Interface, old.Typed = bu, true
	old.Class, old.Type = Subroutine, Type{}

	// A module's text names an external procedure the module knew by another
	// name by the name it knew it by, with its own on a LINK line.
	if a.reading != nil && a.unit.Kind == ast.ModuleUnit && a.reading.links[bu.Name] != "" {
		old.LinkName = a.reading.links[bu.Name]
	}

	if bu.Kind == ast.FunctionUnit && bu.Result != nil {
		old.Class, old.Type = Function, bu.Result.Type
	}

	return old
}

// plainDummy reports whether s is a dummy argument nothing has said more of
// than OPTIONAL, which an interface body may make a dummy procedure.
func plainDummy(s *Symbol) bool {
	return s.Dummy != 0 && s.Class == Unclassified && s.Interface == nil && !s.Typed && !s.External &&
		len(s.Dims) == 0 && s.Intent == "" && !s.Target && s.Common == nil && !s.saved && !s.Automatic &&
		!s.Volatile && s.Struct == nil
}

// explicitInterface is the explicit interface a reference to s goes through:
// an internal procedure's, or an interface body's; nil if s has none.
func explicitInterface(s *Symbol) *Unit {
	if s.Internal != nil {
		return s.Internal
	}

	return s.Interface
}

// interfaceDifferences compares each interface body for an external procedure
// with the procedure's definition, when the file has it (INTERFACEDIFF).
func interfaceDifferences(prog *Program, units map[string]*fileUnit, messages *diag.List) {
	for _, u := range prog.Units {
		for _, body := range u.Interfaces {
			s := u.scope.byName[body.Name]
			if s == nil || s.Interface != body || s.Dummy != 0 || body.incomplete || u.incomplete {
				continue
			}

			fu := units[body.Name]
			if fu == nil || fu.unit == nil || fu.unit.incomplete {
				continue
			}

			if diff := interfaceDifference(body, fu); diff != "" {
				messages.Report("INTERFACEDIFF", body.AST.Position(), body.Name, fu.pos.Line, diff)
			}
		}
	}
}

// interfaceDifference is the first way the interface body differs from the
// definition fu, as INTERFACEDIFF says it, or "" if they agree: their kinds,
// their dummy arguments' number and alternate returns, a function's type, and
// each dummy's type, rank and constant extents, INTENT, OPTIONAL, and whether
// it is a procedure, and an array's kind (assumed-shape, assumed-size, or
// explicit-shape); and PURE said of a procedure that isn't. The dummies' names may differ (12.3.2.1).
func interfaceDifference(body *Unit, fu *fileUnit) string {
	if fu.kind != body.Kind {
		return "it is " + article(body.Kind.String()) + " here, " + article(fu.kind.String()) + " there"
	}

	// A procedure that isn't PURE can't be declared PURE (the Fortran 95
	// standard's 12.3.2.1); a PURE one may have an interface that doesn't say so.
	if body.Pure && fu.unit != nil && !fu.unit.Pure {
		return "it is PURE here, not there"
	}

	params := fu.params()
	if len(params) != len(body.Params) || !equalInts(fu.altPlaces(), body.AltPlaces) {
		return "it has " + count(len(body.Params), "dummy argument") + dumpAltPlaces(body.AltPlaces) +
			" here, " + count(len(params), "dummy argument") + dumpAltPlaces(fu.altPlaces()) + " there"
	}

	if body.Kind == ast.FunctionUnit {
		if here, there := body.Result.Type, fu.result().Type; !sameTypes(here, there) {
			return "its value is " + here.String() + " here, " + there.String() + " there"
		}

		if diff := resultDifference(body.Result, fu.result()); diff != "" {
			return diff
		}

		if diff := pointerResultDifference(body.Result, fu.result()); diff != "" {
			return diff
		}
	}

	for i, p := range body.Params {
		q := params[i]
		what := "its dummy argument " + strconv.Itoa(i+1) + " (" + p.Name + ")"

		if diff := dummyDifference(p, q); diff != "" {
			return what + " " + diff
		}
	}

	return ""
}

// dummyDifference is how the dummy argument p of an interface body differs
// from q, the same dummy of the definition, or "".
func dummyDifference(p, q *Symbol) string {
	switch {
	case p.DummyProcedure() != q.DummyProcedure():
		return "is " + procedureText(p) + " here, " + procedureText(q) + " there"

	case p.DummyProcedure():
		if p.Class != q.Class && p.Class != External && q.Class != External {
			return "is " + p.Class.String() + " here, " + q.Class.String() + " there"
		}

		return ""

	case !sameTypes(p.Type, q.Type):
		return "is " + p.Type.String() + " here, " + q.Type.String() + " there"

	case len(p.Dims) != len(q.Dims):
		return "is " + rankText(len(p.Dims)) + " here, " + rankText(len(q.Dims)) + " there"

	case len(p.Dims) > 0 && shapeText(p) != shapeText(q):
		return "is " + shapeText(p) + " here, " + shapeText(q) + " there"

	case p.Intent != q.Intent:
		return "is " + intentText(p.Intent) + " here, " + intentText(q.Intent) + " there"

	case p.Optional != q.Optional:
		return "is " + optionalText(p.Optional) + " here, " + optionalText(q.Optional) + " there"

	// POINTER and TARGET are characteristics of a dummy too (12.2.1.1).
	case p.Pointer != q.Pointer:
		return "is " + attributeText(p.Pointer, "a pointer") + " here, " + attributeText(q.Pointer, "a pointer") + " there"

	case p.Target != q.Target:
		return "is " + attributeText(p.Target, "a TARGET") + " here, " + attributeText(q.Target, "a TARGET") + " there"
	}

	for k := range p.Dims {
		dp, dq := p.Dims[k], q.Dims[k]
		if dp.known() && dq.known() && dp.Extent() != dq.Extent() {
			return "has " + strconv.Itoa(dp.Extent()) + " elements in dimension " + strconv.Itoa(k+1) +
				" here, " + strconv.Itoa(dq.Extent()) + " there"
		}
	}

	return ""
}

// procedureText says whether a dummy is a procedure, for INTERFACEDIFF.
func procedureText(s *Symbol) string {
	if s.DummyProcedure() {
		return "a procedure"
	}

	return "data"
}

// intentText is a dummy's INTENT as INTERFACEDIFF says it.
func intentText(intent string) string {
	if intent == "" {
		return "of no INTENT"
	}

	return "INTENT(" + intent + ")"
}

// optionalText says whether a dummy is OPTIONAL, for INTERFACEDIFF.
func optionalText(optional bool) string {
	if optional {
		return "OPTIONAL"
	}

	return "not OPTIONAL"
}

// attributeText says whether a dummy has an attribute, for INTERFACEDIFF:
// "a pointer", or "not a pointer".
func attributeText(has bool, what string) string {
	if has {
		return what
	}

	return "not " + what
}

// firstOptional is the routine's first OPTIONAL dummy argument, or nil.
func firstOptional(params []*Symbol) *Symbol {
	for _, p := range params {
		if p.Optional {
			return p
		}
	}

	return nil
}

// argumentMismatch is how the actual argument arg differs from its dummy p in
// type and kind (ARGTYPE) or rank (ARGRANK), with the message's arguments
// after the argument's number and the routine's name; "" if it doesn't, or if
// either can't be compared. A scalar may go to an array dummy only if it is an
// element, sequence association (12.4.1.4).
func argumentMismatch(arg Arg, p *Symbol) (string, []any) {
	// A pointer dummy's actual has been held to it already (pointerArg).
	if arg.Expr == nil || p.broken || arg.Mode != ByDefault || arg.Hollerith || arg.Typeless || p.DummyProcedure() || arg.Pointer {
		return "", nil
	}

	if _, ok := arg.Expr.(*Procedure); ok {
		return "", nil
	}

	given, wanted := arg.Expr.Type(), p.Type
	if given.Kind == Invalid || wanted.Kind == Invalid {
		return "", nil
	}

	// A record's or a derived type's type is recordArguments' to compare
	// (TYPEARG, RECORDARG); its rank is compared here, as any other's.
	records := given.Kind == Record || wanted.Kind == Record

	if !records && (given.Kind != wanted.Kind || (given.Kind != Character && given.Size != wanted.Size)) {
		return "ARGTYPE", []any{article(given.String()), p.Name, article(wanted.String())}
	}

	rank, element := argumentRank(arg.Expr), isElement(arg.Expr)
	if dims := len(p.Dims); (dims == 0 && rank > 0) || (dims > 0 && rank == 0 && !element) {
		return "ARGRANK", []any{rankText(rank), p.Name, rankText(dims)}
	}

	// An assumed-shape dummy takes its actual's shape, so the ranks must be
	// the same, and an element won't do (12.4.1.1).
	if dims := len(p.Dims); p.AssumedShape() && rank != dims {
		return "ARGRANK", []any{rankText(rank), p.Name, rankText(dims)}
	}

	return "", nil
}

// checkImplicitArguments warns of each argument of the reference r to a
// routine of the file, through an implicit interface, whose type, kind, or
// rank isn't its dummy's (ARGTYPEW, ARGRANKW): /WARNINGS=ARGUMENT_CHECKING
// (the author's answer 1). VAX FORTRAN allowed these, and old programs pass an
// array's first element for a scalar, or the reverse, on purpose; so it is a
// warning, and only when asked for.
func checkImplicitArguments(r routineRef, fu *fileUnit, messages *diag.List) {
	params := fu.params()

	for i, arg := range r.args {
		if i >= len(params) {
			return
		}

		ident, rest := argumentMismatch(arg, params[i])
		if ident == "" {
			continue
		}

		args := append([]any{i + 1, r.sym.Name}, rest...)
		messages.Report(ident+"W", arg.Expr.Position(), append(args, fu.pos.Line)...)
	}
}
