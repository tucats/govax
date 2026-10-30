package sema

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Analyze gives the parser's tree its meaning, reporting what is wrong
// with it to messages. mainName is the name of a main program without a
// PROGRAM statement (section 4.12: the source file's name, cut to 26
// characters, with $MAIN after it).
//
// Analysis goes on after an error, a unit at a time, so that one
// compilation reports as much as it can. A unit whose header statement
// the parser rejected (or BLOCK DATA, which is a later phase's) is left
// out: without its header its dummy arguments aren't known, and what
// sema said about the rest would mislead. The Program returned is
// meaningful only if no error was reported.
func Analyze(file *ast.File, mainName string, messages *diag.List) *Program {
	prog := &Program{}
	units := unitNames(file, mainName, messages)

	for _, au := range file.Units {
		switch au.Header.(type) {
		case *ast.BadStmt, *ast.NotImplStmt:
			continue
		}

		a := &analyzer{
			messages:  messages,
			units:     units,
			stmtFuncs: map[ast.Stmt]bool{},
		}

		prog.Units = append(prog.Units, a.analyzeUnit(au, unitName(au, mainName)))
	}

	checkReferences(prog, units, messages)

	return prog
}

// unitName is the name of the unit au.
func unitName(au *ast.Unit, mainName string) string {
	if name := au.Name(); name != "" {
		return name
	}

	return mainName
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
		if kind == ast.BlockDataUnit {
			continue
		}

		if kind == ast.MainProgram {
			if main != nil {
				messages.Report("TWOMAINS", au.Position(), main.Position().Line)

				continue
			}

			main = au
		}

		name := au.Name()
		if name == "" && kind == ast.MainProgram {
			name = mainName
		}

		if name == "" {
			continue
		}

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

	// firstExec is the unit's first executable statement, after which no
	// declaration may come; nil until there is one.
	firstExec ast.Stmt

	// stmtFuncs are the assignments that are statement function
	// definitions: F(X) = ..., F not an array, before the first
	// executable statement.
	stmtFuncs map[ast.Stmt]bool

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
	args int
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
	}
	a.unit = u

	if fu := a.units[name]; fu != nil && fu.unit == nil && fu.kind == u.Kind {
		fu.unit = u
	}

	u.incomplete = incompleteDeclarations(au.Body)

	a.header(au)

	// Declarations first, then the executable statements, so that every
	// statement sees every declaration: a name is an array whatever
	// statement uses it. (A declaration after an executable statement is
	// an error, LATEDECL, but it still counts, so the one mistake isn't
	// reported again at each use.)
	a.declarations(au.Body)
	a.statements(au.Body)
	a.checkLabels()
	a.layout()

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
			if t, ok := a.typeOf(*h.Type, h.Type.Length, h.Type.Pos); ok {
				s.Type, s.Typed, s.TypePos = t, true, h.Pos
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

	s := &Symbol{Name: name, Type: implicitType(name), Offset: -1, Pos: pos}
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
	case s.Class == UnitName:
		return "the name of this subroutine"
	}

	return s.Class.String()
}

// badUse reports that s, being what it is, can't do what (BADUSE).
func (a *analyzer) badUse(s *Symbol, pos diag.Position, what string) {
	a.report("BADUSE", pos, s.Name, a.describe(s), what)
}

// notImpl reports that what isn't implemented yet (NOTIMPL).
func (a *analyzer) notImpl(pos diag.Position, what string) {
	a.report("NOTIMPL", pos, what)
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
	"DUPDUMMY": true, "LATEDECL": true, "MAINRETURN": true,
	"NOIF": true, "NOENDIF": true, "ELSEORDER": true,
	"NODO": true, "DOTERM": true, "DONEST": true,
	"DUPLABEL": true, "UNDEFLBL": true, "NOTEXEC": true,
	"INTOBLOCK": true, "SHAREDTERM": true,
}

// incompleteDeclarations reports whether a unit's body holds a
// declaration this phase can't read: a specification statement that is
// NOTIMPL (COMMON, PARAMETER, IMPLICIT, CHARACTER, ...), or a type
// statement or DIMENSION the parser rejected. Without it, what semantic
// analysis would say about the unit's names would mostly be wrong (an
// array declared in COMMON isn't an array, a PARAMETER constant in a
// bound is a variable), so such a unit gets only the messages about its
// structure. The parser has reported the statement itself.
func incompleteDeclarations(body []ast.Stmt) bool {
	for _, s := range body {
		var kind lex.Kind

		switch s := s.(type) {
		case *ast.NotImplStmt:
			kind = s.Kind
		case *ast.BadStmt:
			kind = s.Kind
		default:
			continue
		}

		switch {
		case kind == lex.Format, kind == lex.Data, kind == lex.Unknown:
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
				if t := target.Result.Type; t != s.Type {
					messages.Report("FUNCTYPE", s.Pos, s.Name, t.String(), fu.pos.Line, s.Type.String())
				}

			case s.Class == Subroutine && target.Kind == ast.SubroutineUnit:

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
		for _, r := range u.calls {
			fu := units[r.sym.Name]
			if fu == nil || fu.unit == nil || fu.unit == u || fu.kind == ast.MainProgram || u.incomplete || fu.unit.incomplete {
				continue
			}

			if n := len(fu.unit.Params); n != r.args {
				messages.Report("ARGCOUNT", r.pos, r.sym.Name, count(n, "dummy argument"), fu.pos.Line, r.args)
			}
		}
	}
}

// count is n things: "1 subscript", "2 subscripts".
func count(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}

	return strconv.Itoa(n) + " " + thing + "s"
}
