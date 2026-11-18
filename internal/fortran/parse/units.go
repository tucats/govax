package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Units inside units (docs/planning/PHASE-103 - Fortran 90 procedures and
// interfaces.md, subtask 2). In VAX FORTRAN a file is a list of units, each
// ended by its END. Fortran 90 nests them two ways:
//
//   - a unit may end with CONTAINS and its internal procedures, each a
//     FUNCTION or SUBROUTINE with an END of its own, before the unit's own
//     END (the standard's R210 and 12.1.2.2):
//
//     SUBROUTINE HOST
//     CALL INNER
//     CONTAINS            <- the host's ContainsStmt
//     SUBROUTINE INNER    <- HOST's Contains[0]
//     END SUBROUTINE
//     END                 <- the host's END
//
//   - an interface block holds interface bodies, each a FUNCTION or
//     SUBROUTINE statement, declarations, and END (R1201 to R1204).
//
// So the parser keeps a stack of the units open at the current statement,
// innermost last (unitStack). A FUNCTION or SUBROUTINE statement starts a
// unit inside the innermost one if that one is past its CONTAINS or has an
// interface block open; otherwise it ends the units open (each without its
// END, NOEND) and starts one of the file's. END ends the innermost.

// unitRole is where a unit is.
type unitRole int

const (
	fileUnit     unitRole = iota // one of the file's units
	internalUnit                 // an internal procedure, after its host's CONTAINS
	moduleUnit                   // a module procedure, after its module's CONTAINS
	bodyUnit                     // an interface body, in an interface block
)

// openFrame is one open unit: the unit being built, what its END is checked
// against, and the Fortran 90 statements open in it.
type openFrame struct {
	unit  *ast.Unit
	check openUnit
	role  unitRole

	// contains is the unit's CONTAINS statement once it has come, after
	// which only internal procedures and the unit's END may.
	contains ast.Stmt

	// iface is the interface block open in the unit, between its
	// INTERFACE and END INTERFACE statements: only interface bodies and
	// MODULE PROCEDURE statements may be there.
	iface *ast.InterfaceBlock

	// typeDef is a derived type's definition open in the unit, which
	// gathers its statements up to END TYPE (Phase 105 will parse them).
	typeDef *ast.PendingStmt

	// records are the names RECORD statements of the unit declare, with
	// its host's (an interface body has no host).
	records *recordScope
}

// unitStack is Parse's state: the file being built, the units open, and
// what waits for the next unit.
type unitStack struct {
	file     *ast.File
	messages *diag.List
	blocks   *blockStack
	open     []*openFrame

	prologue []ast.Stmt     // OPTIONS and directives waiting for a unit
	last     *lex.Statement // the statement before the current one, for NOEND
}

// top is the innermost open unit, or nil between units.
func (u *unitStack) top() *openFrame {
	if len(u.open) == 0 {
		return nil
	}

	return u.open[len(u.open)-1]
}

// records are the record names where the next statement is.
func (u *unitStack) records() *recordScope {
	if f := u.top(); f != nil {
		return f.records
	}

	return nil
}

// holdsProcedure reports whether a FUNCTION or SUBROUTINE statement after
// the statements so far starts a unit inside f: f is past its CONTAINS, or
// has an interface block open.
func (f *openFrame) holdsProcedure(k lex.Kind) bool {
	return (k == lex.Function || k == lex.Subroutine) && (f.iface != nil || f.contains != nil)
}

// start opens the unit whose header is s (nil for a main program without a
// PROGRAM statement), inside the innermost open unit if that one holds it.
func (u *unitStack) start(s ast.Stmt, k lex.Kind) {
	for f := u.top(); f != nil && !f.holdsProcedure(k); f = u.top() {
		u.close(f)
		noEnd(f.unit, u.last, u.messages)
	}

	unit := &ast.Unit{Header: s}
	frame := &openFrame{unit: unit, records: &recordScope{}}

	switch host := u.top(); {
	case host == nil:
		unit.Prologue, u.prologue = u.prologue, nil
		u.file.Units = append(u.file.Units, unit)

	case host.iface != nil:
		// An interface body sees nothing of its host (12.1.2.2.1).
		frame.role = bodyUnit
		host.iface.Bodies = append(host.iface.Bodies, unit)

	default:
		// A module's procedures are module procedures, which may have
		// internal procedures and ENTRY statements of their own (Phase
		// 106); any other unit's are internal procedures.
		frame.role = internalUnit
		if host.unit.Kind() == ast.ModuleUnit {
			frame.role = moduleUnit
		}

		frame.records.host = host.records
		host.unit.Contains = append(host.unit.Contains, unit)
	}

	frame.check = unitOf(s, frame.role != fileUnit)
	u.open = append(u.open, frame)
}

// close ends the innermost unit, f, closing what is open in it.
func (u *unitStack) close(f *openFrame) {
	u.blocks.closeAll()

	if f.iface != nil {
		u.messages.Report("NOINTERFACEEND", f.iface.Position())
	}

	if f.contains != nil && len(f.unit.Contains) == 0 && f.role != internalUnit {
		// (An internal procedure's CONTAINS was NESTEDCONTAINS already.)
		u.messages.Report("EMPTYCONTAINS", f.contains.Position())
	}

	u.open = u.open[:len(u.open)-1]
}

// finish ends the units still open at the file's end, each without its END.
func (u *unitStack) finish() {
	for f := u.top(); f != nil; f = u.top() {
		u.close(f)
		noEnd(f.unit, u.last, u.messages)
	}
}

// place puts the statement s, which the lexer lexed as ls, where it belongs.
func (u *unitStack) place(s ast.Stmt, ls *lex.Statement) {
	k := ls.Kind

	if isHeader(k) {
		if f := u.top(); f != nil {
			f.typeDef = nil
		}

		u.blocks.closeAll()
		u.start(s, k)

		return
	}

	if u.top() == nil {
		u.start(nil, lex.Unknown)
	}

	f := u.top()

	// A derived type's definition takes every statement to its END TYPE;
	// the unit's END, which it never reached, closes it too.
	if f.typeDef != nil && k != lex.End {
		f.typeDef.Inner = append(f.typeDef.Inner, s)

		if k == lex.EndType {
			f.typeDef = nil
		}

		return
	}

	f.typeDef = nil

	switch {
	case k == lex.End:
		u.close(f)
		f.unit.Body = append(f.unit.Body, s)
		checkEnd(s, f.check, u.messages)

	case f.iface != nil:
		u.inInterface(f, s, k)

	case f.contains != nil:
		u.messages.Report("AFTERCONTAINS", s.Position(), aStatement(k))

	case f.role == bodyUnit && notInBody(k):
		u.messages.Report("BADINBODY", s.Position(), aStatement(k))

	case f.role == internalUnit && k == lex.Entry:
		u.messages.Report("ENTRYINTERNAL", s.Position(), f.check.what())

	case u.blocks.take(s):
		// Inside a structure declaration: blocks.go's business.

	default:
		u.inUnit(f, s, k)
	}
}

// notInBody reports whether statements of kind k can't be in an interface
// body, which is a header, a specification part, and END (R1204): no
// executable statement, ENTRY, DATA, FORMAT, statement function (which the
// lexer can't tell from an assignment, an executable statement), nor
// CONTAINS.
func notInBody(k lex.Kind) bool {
	switch k {
	case lex.Entry, lex.Data, lex.Format, lex.Contains:
		return true
	}

	return k.Executable()
}

// inInterface places s, a statement in f's open interface block outside its
// bodies: MODULE PROCEDURE, or the block's END INTERFACE. Anything else can't
// be there (INTERFACESTMT).
func (u *unitStack) inInterface(f *openFrame, s ast.Stmt, k lex.Kind) {
	switch k {
	case lex.EndInterface:
		f.iface.End = s
		f.iface = nil

	case lex.ModuleProcedure:
		if m, ok := s.(*ast.ModuleProcedureStmt); ok {
			f.iface.ModuleProcedures = append(f.iface.ModuleProcedures, m)
		}

	default:
		u.messages.Report("INTERFACESTMT", s.Position(), aStatement(k))
	}
}

// inUnit places s, a statement of the unit f itself, in its body, opening
// what it starts.
func (u *unitStack) inUnit(f *openFrame, s ast.Stmt, k lex.Kind) {
	switch k {
	case lex.Contains:
		if f.role == internalUnit {
			u.messages.Report("NESTEDCONTAINS", s.Position(), f.check.what())
		}

		// Its procedures nest whatever was wrong, so that their ENDs
		// still match them.
		f.contains = s

	case lex.Interface:
		ib, ok := s.(*ast.InterfaceBlock)
		if !ok {
			// INTERFACE with an error in it still opens a block, so that
			// its bodies' statements aren't taken for the unit's.
			ib = &ast.InterfaceBlock{StmtBase: *s.Base()}
		}

		f.iface = ib

	case lex.EndInterface:
		u.messages.Report("NOINTERFACE", s.Position())

		return

	case lex.TypeDef:
		if ps, ok := s.(*ast.PendingStmt); ok {
			f.typeDef = ps
		}
	}

	placed := u.blocks.outside(s)
	f.unit.Body = append(f.unit.Body, placed)

	if r, ok := placed.(*ast.RecordStmt); ok {
		for _, g := range r.Groups {
			for _, d := range g.Entities {
				f.records.add(d.Name)
			}
		}
	}
}

// recordScope is the names declared records in one unit (by RECORD
// statements; from Phase 105 by TYPE(t) declarations too), and its host's.
// The parser needs them to read A.B.C under /F90: a field of A if A is a
// record, else A .B. C, a defined operation (fieldNext). The names are those
// of the statements before the one being parsed, which is enough, since a
// RECORD statement is a declaration and comes before every executable
// statement that refers to its records.
type recordScope struct {
	names map[string]bool
	host  *recordScope
}

// add declares name a record in the scope.
func (r *recordScope) add(name string) {
	if r.names == nil {
		r.names = map[string]bool{}
	}

	r.names[name] = true
}

// has reports whether name is a record in the scope or a host's.
func (r *recordScope) has(name string) bool {
	for ; r != nil; r = r.host {
		if r.names[name] {
			return true
		}
	}

	return false
}

// parseContains parses CONTAINS (R1225).
func parseContains(p *parser) ast.Stmt {
	s := &ast.ContainsStmt{StmtBase: p.base()}
	p.end()

	return s
}

// parseInterface parses an INTERFACE statement (R1202): INTERFACE alone, with
// a generic name, OPERATOR(op), or ASSIGNMENT(=). The interface block it opens
// is filled in as its statements come (unitStack.inInterface).
func parseInterface(p *parser) ast.Stmt {
	s := &ast.InterfaceBlock{StmtBase: p.base()}

	if p.atEnd() {
		return s
	}

	s.Generic.Pos = p.here()
	n := p.name("a generic name, OPERATOR, or ASSIGNMENT")

	switch {
	case n.Name == "OPERATOR" && p.at("("), n.Name == "OPERATOR" && p.at("(/"):
		s.Generic.Kind = ast.GenericOp
		s.Generic.Op = p.genericOperator()

	case n.Name == "ASSIGNMENT" && p.at("("):
		s.Generic.Kind = ast.GenericAssign

		p.next()
		p.expect("=")
		p.expect(")")

	default:
		s.Generic.Kind = ast.GenericName
		s.Generic.Name = n.Name
	}

	p.end()

	return s
}

// relationalSpellings are Fortran 90's spellings of the relational
// operators, each the same operator as its dotted one (R714): an interface
// for <= is one for .LE.
var relationalSpellings = map[string]string{
	"==": ".EQ.", "/=": ".NE.", "<": ".LT.", "<=": ".LE.", ">": ".GT.", ">=": ".GE.",
}

// genericOperator reads OPERATOR's parenthesized operator (R1207, R311): an
// intrinsic operator or a defined one, returned with a relational operator in
// its dotted spelling. The lexer reads (/ and /) as an array constructor's
// parentheses, so OPERATOR(/) and OPERATOR(//) come as (/ ) and (/ /).
func (p *parser) genericOperator() string {
	switch {
	case p.accept("(/"):
		if p.accept("/)") {
			return "//"
		}

		p.expect(")")

		return "/"

	default:
		p.expect("(")
	}

	t := p.peek()

	op := ""

	switch {
	case t == nil:
	case t.DefinedOperator(), t.Is(".NOT."):
		op = t.Text
	case t.Kind == lex.Operator:
		if dotted, ok := relationalSpellings[t.Text]; ok {
			op = dotted
		} else if _, ok := binaryOps[t.Text]; ok {
			op = t.Text
		}
	}

	if op == "" {
		p.syntax("an operator")
	}

	p.next()
	p.expect(")")

	return op
}

// parseEndInterface parses END INTERFACE (R1203).
func parseEndInterface(p *parser) ast.Stmt {
	s := &ast.EndInterfaceStmt{StmtBase: p.base()}
	p.end()

	return s
}

// parseModuleProcedure parses MODULE PROCEDURE name-list (R1205), which the
// lexer finds only in an interface block.
func parseModuleProcedure(p *parser) ast.Stmt {
	s := &ast.ModuleProcedureStmt{StmtBase: p.base()}

	for {
		s.Names = append(s.Names, p.name("a module procedure's name"))

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}
