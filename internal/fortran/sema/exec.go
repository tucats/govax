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
		return a.printStatement(s)

	case *ast.TransferStmt:
		return a.transferStatement(s, path)

	case *ast.CodeStmt:
		return a.codeStatement(s, path)

	case *ast.AuxIOStmt:
		return a.fileStatement(s, path)

	case *ast.DefineFileStmt:
		return a.defineFile(s)
	}

	return nil
}

// assignment analyzes v = e (sections 3.1 and 3.2): the value is
// converted to the target's type by Table 3-1 (REAL to INTEGER or
// LOGICAL truncates, INTEGER or LOGICAL to REAL appends .0). An
// assignment the declarations found to define a statement function has
// nothing to execute.
func (a *analyzer) assignment(s *ast.AssignStmt) Stmt {
	if fn, ok := a.stmtFuncs[s]; ok {
		if fn != nil {
			a.defineStmtFunction(s, fn)
		}

		return nil
	}

	target := a.target(s.Target)
	a.noteAssigned(target)

	// A target with an error says nothing of what its value may be.
	var value Expr
	if target == nil || target.Type().Kind == Record {
		value = a.aggregateValue(s.Value)
	} else {
		value = a.assignedExpr(s.Value, target.Type())
	}

	if target == nil || target.Type().Kind == Invalid || value.Type().Kind == Invalid {
		return nil
	}

	value = a.assignedValue(s.Pos, target, value)
	if value == nil {
		return nil
	}

	return &Assign{stmtBase{s}, target, value}
}

// assignedValue is value as it is assigned to target, or nil after an error. A
// CHARACTER value is assigned only to a CHARACTER place, of any length: it is
// cut or filled with blanks on the right to the place's length when the program
// runs (section 3.3). A record is assigned only a record of the same structure
// declaration (section 3.4; two declared alike aren't the same, ours).
func (a *analyzer) assignedValue(pos diag.Position, target, value Expr) Expr {
	switch tt, vt := target.Type(), value.Type(); {
	case tt.Kind == Record:
		if st := structureOf(target); vt.Kind != Record || structureOf(value) != st {
			what := article(vt.String()) + " value"
			if s := structureOf(value); s != nil {
				what = "a record of the structure " + s.Display()
			}

			a.report("RECORDASSIGN", pos, what, aggregateName(target), st.Display())

			return nil
		}

		return value

	case tt.Kind == Character && vt.Kind == Character:
		return value

	case vt.Kind == Character:
		if c, ok := value.(*Const); ok {
			return a.movedText(c, tt)
		}

		a.report("ASSIGNTYPE", pos, article(vt.String()), placeName(target), tt.String())

		return nil

	case tt.Kind == Character:
		a.report("ASSIGNTYPE", pos, article(vt.String()), placeName(target), tt.String())

		return nil
	}

	return a.assigned(value, target.Type())
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
	case *FieldRef:
		return "the field " + e.Text()
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
		t = a.field(e, false)

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

	case *Elem, *Substring, *FieldRef:
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
		// A system subroutine, unless the file has one of its own.
		if system := systemSubroutines[s.Name]; system != nil && a.units[s.Name] == nil && len(targets) == 0 {
			a.noteSystem(s.Name, s.NamePos)

			return system(a, s, list)
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
		switch {
		case a.unit.Kind == ast.SubroutineUnit && a.unit.Recursive:
			// A recursive subroutine calling itself or one of its entry points
			// (the HP Fortran manual's 8.5.1.1).
		case a.unit.Kind == ast.SubroutineUnit, sym.ResultVar != nil && !a.unit.Recursive:
			a.report("RECURSIVE", s.NamePos, s.Name)

			return nil
		default:
			a.badUse(sym, s.NamePos, "be called")

			return nil
		}

	default:
		if sym.Result && a.ownName(sym.Name) && !a.unit.Recursive {
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
