package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Options are the qualifiers on the FORTRAN command that semantic analysis
// needs, which a unit's OPTIONS statements may override (section 1.4.3). The
// zero value is the default.
type Options struct {
	// NoI4 is /NOI4: INTEGER and LOGICAL with no length are 2 bytes, and an
	// integer constant in a word's range is an INTEGER*2 (Table 2-1).
	NoI4 bool

	// GFloating is /G_FLOATING: REAL*8 and COMPLEX*16 are in G_floating,
	// not D_floating (Decision 16 of docs/PHASE-74 - FORTRAN plan.md).
	GFloating bool
}

// Analyze gives the parser's tree its meaning, reporting what is wrong
// with it to messages. mainName is the name of a main program without a
// PROGRAM statement (section 4.12: the source file's name, cut to 26
// characters, with $MAIN after it).
//
// Analysis goes on after an error, a unit at a time, so that one
// compilation reports as much as it can. A unit whose header statement
// the parser rejected is left out: without its header its dummy arguments aren't known, and what
// sema said about the rest would mislead. The Program returned is
// meaningful only if no error was reported.
func Analyze(file *ast.File, mainName string, opts Options, messages *diag.List) *Program {
	prog := &Program{}
	units := unitNames(file, mainName, messages)

	for _, au := range file.Units {
		unitOpts := prologue(au.Prologue, opts, messages)

		if _, bad := au.Header.(*ast.BadStmt); bad {
			continue
		}

		defaultInt, defaultLogical := IntegerType, LogicalType
		if unitOpts.NoI4 {
			defaultInt, defaultLogical = Type{Integer, 2}, Type{Logical, 2}
		}

		a := &analyzer{
			messages:       messages,
			units:          units,
			stmtFuncs:      map[ast.Stmt]*Symbol{},
			implicit:       defaultImplicit(defaultInt),
			rejected:       map[ast.Stmt]bool{},
			boundVars:      map[*Symbol]diag.Position{},
			defaultInt:     defaultInt,
			defaultLogical: defaultLogical,
			gFloating:      unitOpts.GFloating,
		}

		prog.Units = append(prog.Units, a.analyzeUnit(au, unitName(au, mainName)))
	}

	prologue(file.Trailing, opts, messages)
	checkReferences(prog, units, messages)
	checkCommonInitials(prog, messages)

	return prog
}

// prologue is the options a unit is compiled with: the compilation's, as
// the OPTIONS statements before it change them, until the unit's end
// (section 1.4.3). The qualifiers and directives not implemented yet are
// reported.
func prologue(list []ast.Stmt, opts Options, messages *diag.List) Options {
	for _, s := range list {
		switch s := s.(type) {
		case *ast.BadStmt:

		case *ast.OptionsStmt:
			for _, q := range s.Qualifiers {
				switch q.Name {
				case "I4":
					opts.NoI4 = false
				case "NOI4":
					opts.NoI4 = true
				case "G_FLOATING":
					opts.GFloating = true
				case "NOG_FLOATING":
					opts.GFloating = false
				default:
					messages.Report("NOTIMPL", s.Position(), "the OPTIONS statement's /"+q.Name)
				}
			}

		default:
			messages.Report("NOTIMPL", s.Position(), statementWhat(s))
		}
	}

	return opts
}

// unitName is the name of the unit au.
func unitName(au *ast.Unit, mainName string) string {
	if name := au.Name(); name != "" {
		return name
	}

	if au.Kind() == ast.BlockDataUnit {
		return blockDataName(mainName)
	}

	return mainName
}

// blockDataName is the name of a BLOCK DATA statement without one, from the
// name of a main program without one: SQUARES$DATA in SQUARES.FOR. Section 4.12
// mentions such a name without saying what it is (ours, unconfirmed).
func blockDataName(mainName string) string {
	return strings.TrimSuffix(mainName, "$MAIN") + "$DATA"
}

// fileUnit is a program unit of the file, as another unit's references
// to it are checked against it.
type fileUnit struct {
	kind ast.UnitKind
	pos  diag.Position
	unit *Unit // after analysis; nil until then, or if it was left out
}

// unitNames collects the file's units by name, reporting two units with
// one name (DUPUNIT) and a second main program (TWOMAINS).
func unitNames(file *ast.File, mainName string, messages *diag.List) map[string]*fileUnit {
	units := map[string]*fileUnit{}

	var main *ast.Unit

	for _, au := range file.Units {
		kind := au.Kind()

		if kind == ast.MainProgram {
			if main != nil {
				messages.Report("TWOMAINS", au.Position(), main.Position().Line)

				continue
			}

			main = au
		}

		name := unitName(au, mainName)

		if first, ok := units[name]; ok {
			messages.Report("DUPUNIT", au.Position(), name, first.pos.Line)

			continue
		}

		units[name] = &fileUnit{kind: kind, pos: au.Position()}
	}

	return units
}

// analyzer is semantic analysis's state for one unit.
type analyzer struct {
	messages *diag.List
	units    map[string]*fileUnit
	unit     *Unit

	// defaultInt and defaultLogical are the types INTEGER and LOGICAL with
	// no length mean in this unit: 4 bytes, or with /NOI4 2.
	defaultInt, defaultLogical Type

	// gFloating is set when the unit's REAL*8 is G_floating (format).
	gFloating bool

	// implicit are the unit's letters' types (implicit.go), and
	// rejected the IMPLICIT statements left out of them.
	implicit *implicitRules
	rejected map[ast.Stmt]bool

	// group is the group of Figure 1-1 the unit's statements have
	// reached, and firsts the first statement of each group (order.go).
	group  group
	firsts [execGroup + 1]ast.Stmt

	// stmtFuncs are the assignments that are statement function
	// definitions (statementFunction), each with its function's symbol,
	// or nil for one with an error.
	stmtFuncs map[ast.Stmt]*Symbol

	// adjustable are the arrays declared with bounds that aren't
	// constant, and boundVars the names used in those bounds, each with
	// where it was first used so (arrays.go).
	adjustable []*adjustableArray
	boundVars  map[*Symbol]diag.Position

	// equivStmts are the unit's EQUIVALENCE statements, read when every
	// declaration is known (equivalence.go).
	equivStmts []*ast.EquivalenceStmt

	// dataSets are the unit's lists of names and values, and saveStmts its
	// SAVE statements, both read when every declaration is known (data.go,
	// attributes.go).
	dataSets  []dataSet
	saveStmts []*ast.AttrStmt

	// open are the IF blocks and DO loops open at the statement being
	// analyzed, outermost first; nextID numbers them.
	open   []*construct
	nextID int

	// refs are the jumps to statement labels, checked when every label
	// is known.
	refs []labelRef
}

// routineRef is a CALL or a function reference, for checking against
// the routine if the file defines it.
type routineRef struct {
	sym  *Symbol
	args []Arg
	pos  diag.Position
}

// analyzeUnit analyzes one unit.
func (a *analyzer) analyzeUnit(au *ast.Unit, name string) *Unit {
	u := &Unit{
		AST:    au,
		Kind:   au.Kind(),
		Name:   name,
		Labels: map[int]*Label{},
		byName: map[string]*Symbol{},

		GFloating: a.gFloating,
	}
	a.unit = u

	if fu := a.units[name]; fu != nil && fu.unit == nil && fu.kind == u.Kind {
		fu.unit = u
	}

	body := au.Body
	if u.Kind == ast.BlockDataUnit {
		body = a.blockDataBody(body)
	}

	u.incomplete = incompleteDeclarations(body)

	// The IMPLICIT statements first, since they give the header's
	// names their types too.
	a.implicits(body)
	a.header(au)

	// Declarations first, then the executable statements, so that every
	// statement sees every declaration: a name is an array whatever
	// statement uses it. (A declaration after an executable statement is
	// an error, LATEDECL, but it still counts, so the one mistake isn't
	// reported again at each use.)
	a.declarations(body)
	a.layoutCommon()
	a.equivalences()
	a.checkArrays()
	a.checkCommonSizes()
	a.functionResult()
	a.saves()
	a.dataStatements()
	a.statements(body)
	a.checkLabels()
	a.untyped()
	a.passedLengths()
	a.layout()
	a.placeInitials()

	return u
}

// header enters the names the header statement declares: the unit's
// own name and its dummy arguments.
func (a *analyzer) header(au *ast.Unit) {
	u := a.unit

	switch h := au.Header.(type) {
	case *ast.FunctionStmt:
		s := a.lookup(h.Name, h.Pos)
		s.Class = Variable
		s.Result = true
		u.Result = s

		if h.Type != nil {
			if t, ok := a.functionType(*h.Type, s); ok {
				s.Type, s.Typed, s.TypePos = t, t.Kind != Invalid, h.Pos
			} else {
				s.broken = true
			}
		}

		a.params(h.Params)

	case *ast.SubroutineStmt:
		s := a.lookup(h.Name, h.Pos)
		s.Class = UnitName

		a.params(h.Params)

	default:
		// A main program's name, whether its PROGRAM statement gave it
		// or not, names nothing else in it.
		s := a.lookup(u.Name, au.Position())
		s.Class = UnitName
	}
}

// params enters the dummy arguments.
func (a *analyzer) params(params []*ast.Name) {
	for _, p := range params {
		if p.Name == "*" {
			// An alternate return's place (Phase 79). The unit's dummy
			// arguments are then not what its calls count, so it is
			// left incomplete: only its structure is checked.
			a.notImplForm(p.Pos, "an alternate return")
			a.unit.incomplete = true

			continue
		}

		s := a.lookup(p.Name, p.Pos)

		switch {
		case s.Class == UnitName || s.Result:
			a.badUse(s, p.Pos, "be a dummy argument")
		case s.Dummy != 0:
			a.report("DUPDUMMY", p.Pos, p.Name, a.unit.Kind.String())
		default:
			a.unit.Params = append(a.unit.Params, s)
			s.Dummy = len(a.unit.Params)
		}
	}
}

// lookup returns the symbol named name, entering it, with its implicit
// type, if the unit hasn't used the name before; pos is where it is
// used, which is its first appearance if it is new.
func (a *analyzer) lookup(name string, pos diag.Position) *Symbol {
	if s := a.unit.byName[name]; s != nil {
		return s
	}

	t, untyped := a.implicit.typeOf(name)
	s := &Symbol{Name: name, Type: t, untyped: untyped, Offset: -1, Pos: pos}
	a.unit.byName[name] = s
	a.unit.Symbols = append(a.unit.Symbols, s)

	return s
}

// describe says what s is, for BADUSE: "a variable", "the name of this
// subroutine".
func (a *analyzer) describe(s *Symbol) string {
	switch {
	case s.Result:
		return "the value of this function"
	case s.Class == UnitName && a.unit.Kind == ast.MainProgram:
		return "the name of this program"
	case s.Class == UnitName && a.unit.Kind == ast.BlockDataUnit:
		return "the name of this block data subprogram"
	case s.Class == UnitName:
		return "the name of this subroutine"
	case s.External:
		return External.String()
	}

	return s.Class.String()
}

// badUse reports that s, being what it is, can't do what (BADUSE).
func (a *analyzer) badUse(s *Symbol, pos diag.Position, what string) {
	a.report("BADUSE", pos, s.Name, a.describe(s), what)
}

// notImpl reports that what isn't implemented yet (NOTIMPL), when the
// question depends on the declarations: a name that may be an intrinsic
// function's, a dummy procedure. A unit whose declarations are incomplete
// doesn't get it (see report).
func (a *analyzer) notImpl(pos diag.Position, what string) {
	a.report("NOTIMPL", pos, what)
}

// notImplForm reports that what, a statement or a form the program wrote
// (a COMMON statement, a Hollerith constant, an implied-DO list), isn't
// implemented yet (NOTIMPL). No declaration changes that, so every unit
// gets it, whatever its declarations.
func (a *analyzer) notImplForm(pos diag.Position, what string) {
	a.messages.Report("NOTIMPL", pos, what)
}

// report reports the message ident about pos, unless the unit's
// declarations are incomplete and the message is one they could have
// prevented (see incompleteDeclarations).
func (a *analyzer) report(ident string, pos diag.Position, args ...any) {
	if a.unit.incomplete && !structural[ident] {
		return
	}

	a.messages.Report(ident, pos, args...)
}

// structural are the messages about a unit's structure (its labels,
// blocks, loops, and statement order), which no declaration changes.
var structural = map[string]bool{
	"DUPDUMMY": true, "LATEDECL": true, "ORDER": true, "MAINRETURN": true,
	"IMPLNONE": true, "DUPIMPLICIT": true,
	"NOIF": true, "NOENDIF": true, "ELSEORDER": true,
	"NODO": true, "DOTERM": true, "DONEST": true,
	"DUPLABEL": true, "UNDEFLBL": true, "NOTEXEC": true,
	"INTOBLOCK": true, "SHAREDTERM": true,
}

// incompleteDeclarations reports whether a unit's body holds a
// declaration this phase can't read: a specification statement that is
// not implemented yet (VOLATILE, RECORD, ...), or one the parser
// rejected. Without it, what semantic analysis would say about the unit's
// names would mostly be wrong (a name in RECORD isn't a variable), so
// such a unit gets only the messages about its structure, and its statements'
// own NOTIMPLs.
func incompleteDeclarations(body []ast.Stmt) bool {
	for _, s := range body {
		if _, bad := s.(*ast.BadStmt); !bad && handled(s) {
			continue
		}

		switch kind := s.Base().Kind; {
		case kind == lex.Format, kind == lex.Directive, kind == lex.Options, kind == lex.Unknown:
			// These declare nothing.
		case !kind.Executable():
			return true
		}
	}

	return false
}

// checkReferences checks each unit's uses of the names of the file's
// other units: a unit's name can't be a variable elsewhere in the
// program (section 1.1.3), a SUBROUTINE can't be referenced as a
// function nor a FUNCTION called, a function's references must agree
// with its type, and every reference must give as many arguments as the
// routine has dummy arguments. VAX FORTRAN compiles each unit alone and
// can't see these mistakes; a file holding both units lets us.
func checkReferences(prog *Program, units map[string]*fileUnit, messages *diag.List) {
	for _, u := range prog.Units {
		for _, s := range u.Symbols {
			// A unit's own name (a DUPUNIT's among them) is reported
			// already if it is wrong.
			fu := units[s.Name]
			if fu == nil || fu.unit == u || fu.unit == nil || s.Class == UnitName || s.Result || s.broken {
				continue
			}

			// Without all of either unit's declarations, the types
			// and classes compared would mislead.
			if u.incomplete || fu.unit.incomplete {
				continue
			}

			target := fu.unit
			kind := "a " + target.Kind.String() + " in this file"

			if target.Kind == ast.MainProgram {
				kind = "the main program's name"
			}

			switch {
			case s.Class == Function && target.Kind == ast.FunctionUnit:
				if t := target.Result.Type; t != s.Type && !target.Result.broken {
					messages.Report("FUNCTYPE", s.Pos, s.Name, t.String(), fu.pos.Line, s.Type.String())
				}

			case s.Class == Subroutine && target.Kind == ast.SubroutineUnit:

			case s.Class == External || s.Class == Intrinsic:
				// A name EXTERNAL makes no reference of any kind yet (a
				// BLOCK DATA unit's is meant to be just this), and an
				// INTRINSIC name is FORTRAN's, not the unit of that name.

			case s.Class == Function:
				messages.Report("BADUSE", s.Pos, s.Name, kind, "be used as a function")

			case s.Class == Subroutine:
				messages.Report("BADUSE", s.Pos, s.Name, kind, "be called")

			default:
				messages.Report("BADUSE", s.Pos, s.Name, kind, "be "+s.Class.String())
			}
		}
	}

	for _, u := range prog.Units {
		for _, b := range u.Commons {
			checkBlockName(u, b, units, messages)
		}
	}

	for _, u := range prog.Units {
		for _, r := range u.calls {
			fu := units[r.sym.Name]
			if fu == nil || fu.unit == nil || fu.unit == u || fu.kind == ast.MainProgram || fu.kind == ast.BlockDataUnit || u.incomplete || fu.unit.incomplete {
				continue
			}

			if n := len(fu.unit.Params); n != len(r.args) {
				messages.Report("ARGCOUNT", r.pos, r.sym.Name, count(n, "dummy argument"), fu.pos.Line, len(r.args))

				continue
			}

			// A character constant passed to a numeric dummy argument is a
			// Hollerith constant (section 6.1.1.4), Phase 82's. Without the
			// routine in the file, it is passed by descriptor, as one passed
			// to a CHARACTER dummy must be.
			for i, arg := range r.args {
				c, ok := arg.Expr.(*Const)
				if p := fu.unit.Params[i]; ok && c.T.Kind == Character && p != nil && p.Type.Kind != Character {
					messages.Report("NOTIMPL", c.Pos, hollerith)
				}
			}
		}
	}
}

// checkBlockName reports a common block with the name of a routine, or of
// the main program (section 1.1.3, Table 1-1; section 4.2 says so for
// functions, subroutines, and entries): those of the file's units, and the
// routines the unit that declares the block references. A variable may
// share a block's name.
func checkBlockName(u *Unit, b *CommonBlock, units map[string]*fileUnit, messages *diag.List) {
	if b.Name == "" {
		return
	}

	what := ""

	if fu := units[b.Name]; fu != nil {
		what = "a " + fu.kind.String() + " in this file"

		if fu.kind == ast.MainProgram {
			what = "the main program"
		}
	} else if s := u.Lookup(b.Name); s != nil && !u.incomplete && !s.broken {
		switch s.Class {
		case Function:
			what = "a function this unit references"
		case Subroutine:
			what = "a subroutine this unit calls"
		}
	}

	if what != "" {
		messages.Report("COMMONNAME", b.NamePos, b.String(), what)
	}
}

// count is n things: "1 subscript", "2 subscripts".
func count(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}

	return strconv.Itoa(n) + " " + thing + "s"
}
