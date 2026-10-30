package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// The executable statements other than the block statements (stmt.go).

// executable analyzes one executable statement; path is the constructs
// open at it, for its jumps. It returns nil for a statement with nothing
// to execute or an error.
func (a *analyzer) executable(s ast.Stmt, path []frame) Stmt {
	switch s := s.(type) {
	case *ast.AssignStmt:
		return a.assignment(s)

	case *ast.CallStmt:
		return a.call(s)

	case *ast.GotoStmt:
		a.refs = append(a.refs, labelRef{label: s.Target.Label, pos: s.Target.Pos, path: path})

		return &Goto{stmtBase{s}, s.Target.Label}

	case *ast.IfStmt:
		if !handled(s.Then) {
			a.notImplForm(s.Then.Position(), statementWhat(s.Then))

			return nil
		}

		ifs := &If{stmtBase: stmtBase{s}}
		arm := &Arm{AST: s, Cond: a.condition(s.Cond)}
		ifs.Arms = []*Arm{arm}

		if then := a.executable(s.Then, path); then != nil {
			arm.Body = []Stmt{then}
		}

		return ifs

	case *ast.ContinueStmt:
		return &Continue{stmtBase{s}}

	case *ast.StopStmt:
		st := &Stop{stmtBase: stmtBase{s}}

		switch d := s.Display.(type) {
		case *ast.CharConst:
			st.Display, st.HasDisplay = d.Value, true
		case *ast.IntConst:
			// STOP n displays its digits as written (section 5.10).
			st.Display, st.HasDisplay = d.Text, true
		}

		return st

	case *ast.ReturnStmt:
		switch {
		case s.Alt != nil:
			a.notImpl(s.Alt.Position(), "an alternate return")
		case a.unit.Kind == ast.MainProgram:
			a.report("MAINRETURN", s.Pos)
		}

		return &Return{stmtBase{s}}

	case *ast.PrintStmt:
		a.listDirected(&s.Format)

		return &Print{stmtBase: stmtBase{s}, Items: a.items(s.Items)}

	case *ast.TransferStmt:
		switch {
		case len(s.Specs) > 0:
			a.notImplForm(s.Specs[0].Pos, "the "+s.Specs[0].Keyword+"= specifier")

			return nil
		case s.Namelist != nil:
			a.notImplForm(s.Namelist.Pos, "namelist-directed output")

			return nil
		case s.Rec != nil:
			a.notImplForm(s.Rec.Position(), "direct access output")

			return nil
		case !s.Unit.Star:
			a.notImpl(s.Unit.Pos, "a unit other than *")
		case s.Format == nil:
			a.notImpl(s.Pos, "unformatted output")
		default:
			a.listDirected(s.Format)
		}

		return &Print{stmtBase: stmtBase{s}, Write: true, Items: a.items(s.Items)}
	}

	return nil
}

// assignment analyzes v = e (sections 3.1 and 3.2): the value is
// converted to the target's type by Table 3-1 (REAL to INTEGER or
// LOGICAL truncates, INTEGER or LOGICAL to REAL appends .0). An
// assignment the declarations found to define a statement function is
// Phase 79's.
func (a *analyzer) assignment(s *ast.AssignStmt) Stmt {
	if fn, ok := a.stmtFuncs[s]; ok {
		if fn != nil {
			a.notImpl(s.Pos, "a statement function")
		}

		return nil
	}

	target := a.target(s.Target)
	value := a.value(s.Value)

	if target == nil || target.Type().Kind == Invalid || value.Type().Kind == Invalid {
		return nil
	}

	if value.Type().Kind == Character {
		a.notImpl(value.Position(), hollerith)

		return nil
	}

	return &Assign{stmtBase{s}, target, a.convert(value, target.Type())}
}

// target analyzes an assignment's target: a variable or an array
// element. It returns nil after an error.
func (a *analyzer) target(e ast.Expr) Expr {
	var t Expr

	switch e := e.(type) {
	case *ast.Name:
		if s := a.unit.byName[e.Name]; s != nil && s.Class == Constant {
			if !s.broken {
				a.badUse(s, e.Pos, "be assigned a value")
			}

			return nil
		}

		t = a.name(e, true)

		if r, ok := t.(*ArrayRef); ok {
			a.report("BADTARGET", e.Pos, "the array "+r.Sym.Name+" as a whole")

			return nil
		}

	case *ast.Substring, *ast.FieldRef:
		a.notImplForm(e.Position(), formWhat(e))

		return nil

	case *ast.Apply:
		// After the first executable statement, F(X) = ... can only be
		// an assignment to an array element.
		s := a.unit.byName[e.Name]
		if s != nil && s.broken {
			return nil
		}

		switch {
		case s != nil && s.Class == StmtFunction:
			a.badUse(s, e.Pos, "be assigned a value")

			return nil

		case s == nil || s.Class != Array:
			a.report("NOTARRAY", e.Pos, e.Name)

			return nil

		case a.uncompiledUse(s, e.Pos):
			return nil
		}

		t = a.element(s, e)
	}

	switch t := t.(type) {
	case *Var:
		a.checkLoopVariable(t.Sym, t.Pos)

		return t

	case *Elem:
		return t
	}

	// Anything else already has its message.
	return nil
}

// checkLoopVariable reports a change to the control variable of a DO
// loop the statement is in (DOVARSET), which section 5.3.1 forbids.
func (a *analyzer) checkLoopVariable(s *Symbol, pos diag.Position) {
	for _, c := range a.open {
		if c.isDo() && c.do.Var == s {
			a.report("DOVARSET", pos, s.Name, c.line())

			return
		}
	}
}

// call analyzes CALL name(args) (section 5.1).
func (a *analyzer) call(s *ast.CallStmt) Stmt {
	sym := a.unit.byName[s.Name]

	if sym == nil || (sym.Class == Unclassified && sym.Dummy == 0) {
		if what := builtin(s.Name, false); what != "" {
			a.notImpl(s.NamePos, what)

			return nil
		}
	}

	sym = a.lookup(s.Name, s.NamePos)

	if sym.broken {
		return nil
	}

	switch sym.Class {
	case Unclassified:
		if sym.Dummy != 0 {
			a.notImpl(s.NamePos, "a dummy procedure")

			return nil
		}

		sym.Class = Subroutine

	case Subroutine:

	case UnitName:
		if a.unit.Kind == ast.SubroutineUnit {
			a.report("RECURSIVE", s.NamePos, s.Name)
		} else {
			a.badUse(sym, s.NamePos, "be called")
		}

		return nil

	default:
		if sym.Result {
			a.report("RECURSIVE", s.NamePos, s.Name)
		} else {
			a.badUse(sym, s.NamePos, "be called")
		}

		return nil
	}

	args := a.args(s.Args)
	a.unit.calls = append(a.unit.calls, routineRef{sym, len(args), s.NamePos})

	return &Call{stmtBase{s}, sym, args}
}

// listDirected reports a format other than *: formatted I/O is Phase
// 80's.
func (a *analyzer) listDirected(f *ast.FormatSpec) {
	switch {
	case f.Label != nil:
		a.notImpl(f.Pos, "output with a FORMAT statement")
	case f.Expr != nil:
		a.notImpl(f.Pos, "a format given by an expression")
	}
}

// items analyzes an output list: each item a value, a whole array
// (section 2.2.3.5; not an assumed-size one, whose size isn't known), or
// a character constant.
func (a *analyzer) items(list []ast.Expr) []Expr {
	items := make([]Expr, 0, len(list))

	for _, e := range list {
		var item Expr

		if n, ok := e.(*ast.Name); ok {
			item = a.name(n, true)

			if r, ok := item.(*ArrayRef); ok && r.Sym.AssumedSize() {
				a.report("NOSIZE", n.Pos, n.Name)
			}
		} else {
			item = a.value(e)
		}

		items = append(items, item)
	}

	return items
}
