package sema

import (
	"math"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
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
		return a.call(s, path)

	case *ast.GotoStmt:
		a.refs = append(a.refs, labelRef{label: s.Target.Label, pos: s.Target.Pos, path: path})

		return &Goto{stmtBase{s}, s.Target.Label}

	case *ast.ComputedGotoStmt:
		return a.computedGoto(s, path)

	case *ast.AssignLabelStmt:
		return a.assignLabel(s)

	case *ast.AssignedGotoStmt:
		return a.assignedGoto(s, path)

	case *ast.ArithIfStmt:
		return a.arithIf(s, path)

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
		st := &Stop{stmtBase: stmtBase{s}, Pause: s.Kind == lex.Pause}

		switch d := s.Display.(type) {
		case *ast.CharConst:
			st.Display, st.HasDisplay = d.Value, true
		case *ast.IntConst:
			// STOP n and PAUSE n display their digits as written
			// (sections 5.10 and 5.8).
			st.Display, st.HasDisplay = d.Text, true
		}

		return st

	case *ast.ReturnStmt:
		return a.returnStmt(s)

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

	// A CHARACTER value is assigned only to a CHARACTER place, of any
	// length: it is cut or filled with blanks on the right to the place's
	// length when the program runs (section 3.3).
	switch tt, vt := target.Type(), value.Type(); {
	case tt.Kind == Character && vt.Kind == Character:
		return &Assign{stmtBase{s}, target, value}

	case vt.Kind == Character:
		if _, ok := value.(*Const); ok {
			a.notImpl(value.Position(), hollerith)
		} else {
			a.report("ASSIGNTYPE", s.Pos, article(vt.String()), placeName(target), tt.String())
		}

		return nil

	case tt.Kind == Character:
		a.report("ASSIGNTYPE", s.Pos, article(vt.String()), placeName(target), tt.String())

		return nil
	}

	return &Assign{stmtBase{s}, target, a.assigned(value, target.Type())}
}

// placeName names an assignment's target for a message: a variable's name,
// an array element's array's ("an element of A"), a substring's ("a
// substring of S").
func placeName(e Expr) string {
	switch e := e.(type) {
	case *Var:
		return e.Sym.Name
	case *Elem:
		return "an element of " + e.Sym.Name
	case *Substring:
		return "a substring of " + placeName(e.X)
	}

	return "it"
}

// assigned is value converted to t, the type of the variable it is assigned
// to. An integer constant may hold more of a short variable's bits than its
// value's range: BYTE and LOGICAL*1 take -128 to 255, LOGICAL*2 and INTEGER*2
// -32768 to 65535, and X = 255 in a BYTE is -1 (section 2.2.1.1). Past those
// ranges it is INTRANGE, as a DATA statement's is (ours; the manual doesn't
// say).
func (a *analyzer) assigned(value Expr, t Type) Expr {
	c, ok := value.(*Const)
	if !ok || t.Size >= 4 || (c.T.Kind != Integer && c.T.Kind != Logical) ||
		(t.Kind != Integer && t.Kind != Logical) {
		return a.convert(value, t)
	}

	lo, hi := int64(math.MinInt8), int64(math.MaxUint8)
	if t.Size == 2 {
		lo, hi = math.MinInt16, math.MaxUint16
	}

	if v := int64(c.Int); v < lo || v > hi {
		a.report("INTRANGE", c.Pos, valueText, t.String(), lo, hi)

		return a.bad(c.Pos)
	}

	return &Const{exprBase: exprBase{t, c.Pos}, Int: truncate(c.Int, t.Size)}
}

// target analyzes an assignment's target: a variable, an array element,
// or a substring of either. It returns nil after an error.
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

		if s := a.unit.byName[e.Name]; s != nil && !s.broken && (s.External || s.Class == Intrinsic) {
			a.badUse(s, e.Pos, "be assigned a value")

			return nil
		}

		t = a.name(e, true)

		if r, ok := t.(*ArrayRef); ok {
			a.report("BADTARGET", e.Pos, "the array "+r.Sym.Name+" as a whole")

			return nil
		}

	case *ast.Substring:
		t = a.substring(e)

	case *ast.FieldRef:
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
		}

		t = a.element(s, e)
	}

	switch t := t.(type) {
	case *Var:
		a.checkLoopVariable(t.Sym, t.Pos)

		return t

	case *Elem, *Substring:
		return t
	}

	// Anything else already has its message.
	return nil
}

// checkLoopVariable reports a change to the control variable of a DO
// loop the statement is in (DOVARSET), which section 5.3.1 forbids.
func (a *analyzer) checkLoopVariable(s *Symbol, pos diag.Position) {
	for _, c := range a.open {
		if c.do != nil && c.do.Var == s {
			a.report("DOVARSET", pos, s.Name, c.line())

			return
		}
	}
}

// call analyzes CALL name(args) (section 5.1).
func (a *analyzer) call(s *ast.CallStmt, path []frame) Stmt {
	list, targets, places := altReturns(s.Args)
	a.jumps(targets, path)

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
	case Unclassified, External:
		// A dummy argument called is a dummy procedure.
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

	args := a.args(list)
	a.unit.calls = append(a.unit.calls, routineRef{sym, args, s.NamePos, places})

	c := &Call{stmtBase: stmtBase{s}, Sub: sym, Args: args}
	for _, t := range targets {
		c.AltReturns = append(c.AltReturns, t.Label)
	}

	return c
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
