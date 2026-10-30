package sema

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The statements, and the blocks they make. The parser left each unit a
// flat list of statements; here IF THEN ... END IF becomes an If whose
// arms hold their statements, and a DO and the statements up to its
// terminal statement a Do holding them, so later passes never match
// blocks again.
//
// While it walks the list, the analyzer keeps a stack of the constructs
// open at the current statement (open), innermost last. A statement goes
// into the innermost one's current block. IF THEN pushes an IF; ELSE IF
// and ELSE start a new arm of the innermost IF; END IF pops it. DO
// pushes a loop, which the statement with its label pops (with every
// other loop on top of the stack that ends there too: nested loops may
// share a terminal statement, section 5.3.1.2). Anything else that
// would close a construct is an error of nesting: an END IF with a DO
// loop still open inside its block, a DO's terminal statement inside an
// IF block that started inside the loop (DONEST).

// construct is an IF block or a DO loop open at the current statement.
type construct struct {
	id int

	// For an IF block: the construct, its current arm, and whether ELSE
	// has been seen (after which only END IF may come). ifs is nil for
	// an IF THEN statement the parser rejected, whose block is still
	// matched so its END IF isn't reported again.
	ifs     *If
	arm     *Arm
	elseAt  ast.Stmt
	ifStart ast.Stmt

	// For a DO loop: the loop and the label of its terminal statement.
	// reported is set once a message has been given about the loop's
	// nesting or its terminal statement, after which it closes quietly at
	// the end of the unit.
	do       *Do
	label    int
	reported bool
}

// isDo reports whether c is a DO loop.
func (c *construct) isDo() bool { return c.do != nil }

// line is the line the construct starts on, for messages.
func (c *construct) line() int {
	if c.isDo() {
		return c.do.AST.Position().Line
	}

	return c.ifStart.Position().Line
}

// what names the construct for DONEST.
func (c *construct) what() string {
	if c.isDo() {
		return "DO loop"
	}

	return "IF block"
}

// statements is the second walk over a unit's statements, building its
// structured body.
func (a *analyzer) statements(body []ast.Stmt) {
	for _, s := range body {
		a.statement(s)
	}

	// A unit whose END the parser didn't find (NOEND) still has its open
	// constructs reported.
	a.closeAll()
}

// statement analyzes one statement in the unit's body.
func (a *analyzer) statement(s ast.Stmt) {
	path := a.path()
	label := s.Base().Label

	if label != 0 {
		a.defineLabel(label, s, path)

		// A statement that may not end a DO loop (section 5.3.1) is
		// reported here, and the loops it would end are left open.
		if a.endsLoop(label) {
			if what := forbiddenTerminal(s); what != "" {
				a.report("DOTERM", s.Position(), what)
				a.markLoops(label)

				label = 0
			}
		}
	}

	switch s := s.(type) {
	case *ast.TypeStmt, *ast.DimensionStmt:
		// Their work was done by declarations.
		return

	case *ast.IfThenStmt:
		ifs := &If{stmtBase: stmtBase{s}, Block: true}
		arm := &Arm{AST: s, Cond: a.condition(s.Cond)}
		ifs.Arms = []*Arm{arm}

		a.add(ifs)
		a.push(&construct{ifs: ifs, arm: arm, ifStart: s})

	case *ast.ElseIfStmt:
		if c := a.matchIf(s); c != nil {
			if c.elseAt != nil {
				a.report("ELSEORDER", s.Pos, "ELSE IF", c.elseAt.Position().Line)
			}

			c.arm = &Arm{AST: s, Cond: a.condition(s.Cond)}

			if c.ifs != nil {
				c.ifs.Arms = append(c.ifs.Arms, c.arm)
			}
		}

	case *ast.ElseStmt:
		if c := a.matchIf(s); c != nil {
			if c.elseAt != nil {
				a.report("ELSEORDER", s.Pos, "ELSE", c.elseAt.Position().Line)
			}

			c.arm, c.elseAt = &Arm{AST: s}, s

			if c.ifs != nil {
				c.ifs.Else = c.arm
			}
		}

	case *ast.EndIfStmt:
		if c := a.matchIf(s); c != nil {
			if c.ifs != nil {
				c.ifs.EndIf = s
			}

			a.pop()
		}

	case *ast.DoStmt:
		a.doStatement(s)

	case *ast.EndStmt:
		a.closeAll()
		a.add(&End{stmtBase{s}})

	case *ast.NotImplStmt:
		a.brokenStatement(s.Kind, s)

	case *ast.BadStmt:
		a.brokenStatement(s.Kind, s)

	default:
		if st := a.executable(s, path); st != nil {
			a.add(st)
		}

		if label != 0 {
			a.endLoops(label, s)
		}
	}
}

// brokenStatement keeps the block structure of a statement the parser
// rejected or couldn't compile, so that a block IF statement with an
// error doesn't also make its END IF "not in an IF block".
func (a *analyzer) brokenStatement(kind lex.Kind, s ast.Stmt) {
	switch kind {
	case lex.IfThen:
		a.push(&construct{ifStart: s})

	case lex.ElseIf, lex.Else:
		if c := a.quietMatchIf(); c != nil {
			c.arm = &Arm{AST: s}
		}

	case lex.EndIf:
		if c := a.quietMatchIf(); c != nil {
			a.pop()
		}

	case lex.End:
		a.closeAll()

	default:
		if label := s.Base().Label; label != 0 {
			a.endLoops(label, s)
		}
	}
}

// forbiddenTerminal says what s is if it may not be a DO loop's terminal
// statement (section 5.3.1: a GO TO, a block IF statement, END, RETURN,
// DO; and any non-executable statement, since the terminal statement's
// label is an executable statement's), or "" if it may.
func forbiddenTerminal(s ast.Stmt) string {
	switch s := s.(type) {
	case *ast.GotoStmt, *ast.IfThenStmt, *ast.ElseIfStmt, *ast.ElseStmt,
		*ast.EndIfStmt, *ast.EndStmt, *ast.ReturnStmt, *ast.DoStmt,
		*ast.TypeStmt, *ast.DimensionStmt:
		return aStatement(s)

	case *ast.NotImplStmt:
		// Reported already; but END DO and the like still mustn't end
		// a labeled loop.
		if s.Kind == lex.ArithmeticIf || s.Kind == lex.DoWhile || !s.Kind.Executable() {
			return aStatement(s)
		}
	}

	return ""
}

// path is the constructs open at the current statement.
func (a *analyzer) path() []frame {
	path := make([]frame, len(a.open))

	for i, c := range a.open {
		path[i] = frame{id: c.id, arm: c.arm, do: c.isDo()}
	}

	return path
}

// push opens a construct.
func (a *analyzer) push(c *construct) {
	a.nextID++
	c.id = a.nextID
	a.open = append(a.open, c)
}

// pop closes the innermost construct.
func (a *analyzer) pop() {
	a.open = a.open[:len(a.open)-1]
}

// top is the innermost open construct, or nil.
func (a *analyzer) top() *construct {
	if len(a.open) == 0 {
		return nil
	}

	return a.open[len(a.open)-1]
}

// add puts st in the innermost open block: the current arm of an IF,
// the body of a DO, or the unit's body.
func (a *analyzer) add(st Stmt) {
	c := a.top()

	switch {
	case c == nil:
		a.unit.Body = append(a.unit.Body, st)
	case c.isDo():
		c.do.Body = append(c.do.Body, st)
	case c.arm != nil:
		c.arm.Body = append(c.arm.Body, st)
	}
}

// matchIf finds the IF block an ELSE IF, ELSE, or END IF statement
// belongs to: the innermost open construct, once any DO loop opened
// inside the block has been closed with a message (DONEST: a loop must
// lie wholly in one block, section 5.7.3.1). With no IF block open the
// statement is NOIF.
func (a *analyzer) matchIf(s ast.Stmt) *construct {
	i := len(a.open) - 1
	for i >= 0 && a.open[i].isDo() {
		i--
	}

	if i < 0 {
		a.report("NOIF", s.Position(), statementName(s))

		return nil
	}

	for len(a.open)-1 > i {
		if c := a.top(); !c.reported {
			a.report("DONEST", s.Position(), c.what(), c.line(), "this "+statementName(s)+" statement")
		}

		a.pop()
	}

	return a.top()
}

// quietMatchIf is matchIf without messages, for a statement with an
// error already reported.
func (a *analyzer) quietMatchIf() *construct {
	for c := a.top(); c != nil && c.isDo(); c = a.top() {
		a.pop()
	}

	return a.top()
}

// closeAll closes every open construct at the end of the unit: an IF
// block has no END IF (NOENDIF), and a DO loop's terminal statement
// never came (NODO), unless its loop has been reported already.
func (a *analyzer) closeAll() {
	for _, c := range a.open {
		switch {
		case c.isDo() && !c.reported:
			a.report("NODO", c.do.AST.Position(), c.label)
		case !c.isDo():
			a.report("NOENDIF", c.ifStart.Position())
		}
	}

	a.open = nil
}

// endsLoop reports whether an open DO loop ends at the label.
func (a *analyzer) endsLoop(label int) bool {
	for _, c := range a.open {
		if c.isDo() && c.label == label {
			return true
		}
	}

	return false
}

// markLoops marks the open loops that end at label as reported.
func (a *analyzer) markLoops(label int) {
	for _, c := range a.open {
		if c.isDo() && c.label == label {
			c.reported = true
		}
	}
}

// endLoops closes the DO loops whose terminal statement is s, labeled
// label: every loop at the top of the stack that ends there. A loop
// ending there that isn't at the top (an IF block or another loop was
// opened inside it and hasn't ended) is DONEST, and is left open.
func (a *analyzer) endLoops(label int, s ast.Stmt) {
	var ended []int

	for c := a.top(); c != nil && c.isDo() && c.label == label; c = a.top() {
		ended = append(ended, c.id)
		a.pop()
	}

	if l := a.unit.Labels[label]; l != nil && l.Stmt == s {
		l.loops = ended
	}

	for i := len(a.open) - 1; i >= 0; i-- {
		c := a.open[i]
		if !c.isDo() || c.label != label || c.reported {
			continue
		}

		inner := a.open[i+1]
		a.report("DONEST", s.Position(), inner.what(), inner.line(),
			"the DO loop of line "+strconv.Itoa(c.line())+" ends")

		c.reported = true
	}
}

// doStatement analyzes DO s v = e1, e2, e3 (section 5.3.1) and opens its
// loop.
func (a *analyzer) doStatement(s *ast.DoStmt) {
	d := &Do{stmtBase: stmtBase{s}, Label: s.Target.Label}
	d.Var = a.doVariable(s.Var)

	t := IntegerType
	if d.Var != nil {
		t = d.Var.Type
	}

	// The parameters are converted to the variable's type before they
	// are used (5.3.1).
	param := func(e ast.Expr) Expr {
		return a.convert(a.arithmetic(a.value(e)), t)
	}

	d.Start = param(s.Start)
	d.Limit = param(s.Limit)

	if s.Step != nil {
		d.Step = param(s.Step)

		if c, ok := d.Step.(*Const); ok && (c.Int == 0 && c.T.Kind != Real || c.T.Kind == Real && c.Real.IsZero()) {
			a.report("DOSTEP0", s.Step.Position())
		}
	} else {
		d.Step = a.convert(&Const{exprBase: exprBase{IntegerType, s.Pos}, Int: 1}, t)
	}

	a.add(d)

	// The terminal statement must follow the DO statement (5.3.1); a
	// label already defined is behind it.
	if a.unit.Labels[d.Label] != nil {
		a.report("NODO", s.Pos, d.Label)

		return
	}

	a.push(&construct{do: d, label: d.Label})
}

// doVariable analyzes a DO statement's control variable, which must be
// an INTEGER or REAL variable (DOVAR), and not the control variable of a
// loop it is inside (DOVARSET).
func (a *analyzer) doVariable(n *ast.Name) *Symbol {
	var v *Var

	switch e := a.name(n, true).(type) {
	case *Var:
		v = e
	case *ArrayRef:
		a.report("DOVAR", n.Pos, n.Name, "an array")

		return nil
	default:
		return nil
	}

	switch v.Sym.Type.Kind {
	case Integer, Real:
	default:
		a.report("DOVAR", n.Pos, n.Name, v.Sym.Type.String())

		return nil
	}

	a.checkLoopVariable(v.Sym, n.Pos)

	return v.Sym
}
