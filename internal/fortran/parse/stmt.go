package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The executable statements, but for I/O (io.go).

// parseAssignment parses v = e (chapter 3). The target is a name or a
// name with a parenthesized list: a variable, an array element, or a
// statement function's definition, F(X) = X*X, which only the
// declarations can tell from an array element.
func parseAssignment(p *parser) ast.Stmt {
	s := &ast.AssignStmt{StmtBase: p.base()}

	if !p.atKind(lex.Name) {
		p.syntax("a variable or an array element")
	}

	t := p.next()
	base := ast.ExprBase{Pos: p.pos(t)}

	if p.at("(") {
		s.Target = p.apply(t, base)
	} else {
		s.Target = &ast.Name{ExprBase: base, Name: t.Text}
	}

	p.expect("=")
	s.Value = p.expr()
	p.end()

	return s
}

// parseCall parses CALL name [([a][,[a]]...)] (section 5.1). An argument
// left out between commas is a null argument, kept as nil: CALL
// S(A,,B) has three arguments, the second null. CALL S() has none.
func parseCall(p *parser) ast.Stmt {
	n := p.name("the name of a subroutine")
	s := &ast.CallStmt{StmtBase: p.base(), Name: n.Name, NamePos: n.Pos}

	if p.atEnd() {
		p.end()

		return s
	}

	p.expect("(")

	s.Args = []ast.Expr{}

	if p.accept(")") {
		p.end()

		return s
	}

	for {
		switch {
		case p.at(",") || p.at(")"):
			s.Args = append(s.Args, nil)

		case p.at("*") || p.at("&"):
			p.notImpl("an alternate return argument", p.here())

		default:
			s.Args = append(s.Args, p.expr())
		}

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")
	p.end()

	return s
}

// parseGoTo parses the unconditional GO TO s (section 5.6.1). The
// computed GO TO, GO TO (s,s...) i, and the assigned GO TO, GO TO v,
// are later phases'.
func parseGoTo(p *parser) ast.Stmt {
	switch {
	case p.at("("):
		p.notImpl("the computed GO TO statement", p.base().Pos)
	case p.atKind(lex.Name):
		p.notImpl("the assigned GO TO statement", p.base().Pos)
	}

	s := &ast.GotoStmt{StmtBase: p.base(), Target: p.labelRef()}
	p.end()

	return s
}

// condition reads an IF statement's parenthesized condition.
func (p *parser) condition() ast.Expr {
	p.expect("(")
	e := p.expr()
	p.expect(")")

	return e
}

// parseLogicalIf parses IF (e) st (section 5.7.2). The lexer has lexed
// st as a statement of its own (the Then of the lex.Statement), which is
// parsed here the same way, and must be an executable statement other
// than a DO, a block IF statement, another logical IF, or END.
func parseLogicalIf(p *parser) ast.Stmt {
	s := &ast.IfStmt{StmtBase: p.base(), Cond: p.condition()}
	p.end()

	then := p.st.Then
	if then == nil {
		p.syntax("a statement after the condition")
	}

	if bad := badIfStatement(then.Kind); bad != "" {
		p.messages.Report("BADIFSTMT", then.Position(then.Offset), bad)
		panic(bailout{})
	}

	s.Then = statement(then, p.messages)

	// If the statement had an error, or isn't implemented, the IF as a
	// whole is the same: later passes needn't look inside it.
	switch t := s.Then.(type) {
	case *ast.NotImplStmt:
		panic(bailout{what: t.What})
	case *ast.BadStmt:
		panic(bailout{})
	}

	return s
}

// badIfStatement says what a statement of kind k is if a logical IF may
// not execute it ("a DO statement"), or "" if it may.
func badIfStatement(k lex.Kind) string {
	switch k {
	case lex.LogicalIf:
		return "another logical IF"
	case lex.Unknown:
		return "" // the lexer reported it
	case lex.Do, lex.DoWhile, lex.EndDo, lex.IfThen, lex.ElseIf, lex.Else, lex.EndIf, lex.End:
		return article(k.String()) + " " + k.String() + " statement"
	}

	if !k.Executable() {
		return article(k.String()) + " " + k.String() + " statement"
	}

	return ""
}

// parseIfThen parses IF (e) THEN (section 5.7.3).
func parseIfThen(p *parser) ast.Stmt {
	s := &ast.IfThenStmt{StmtBase: p.base(), Cond: p.condition()}
	p.expect("THEN")
	p.end()

	return s
}

// parseElseIf parses ELSE IF (e) THEN.
func parseElseIf(p *parser) ast.Stmt {
	s := &ast.ElseIfStmt{StmtBase: p.base(), Cond: p.condition()}
	p.expect("THEN")
	p.end()

	return s
}

// parseElse parses ELSE.
func parseElse(p *parser) ast.Stmt {
	p.end()

	return &ast.ElseStmt{StmtBase: p.base()}
}

// parseEndIf parses END IF.
func parseEndIf(p *parser) ast.Stmt {
	p.end()

	return &ast.EndIfStmt{StmtBase: p.base()}
}

// parseDo parses DO s [,] v = e1, e2 [, e3] (section 5.3.1). The lexer
// has made sure there is a comma after the =, or it would have been an
// assignment (DO 10 I = 1.5 assigns to DO10I).
func parseDo(p *parser) ast.Stmt {
	if p.atKind(lex.Name) {
		p.notImpl("a DO loop without a statement label", p.base().Pos)
	}

	s := &ast.DoStmt{StmtBase: p.base(), Target: p.labelRef()}
	p.accept(",")

	s.Var = p.name("the DO variable")
	p.expect("=")
	s.Start = p.expr()
	p.expect(",")
	s.Limit = p.expr()

	if p.accept(",") {
		s.Step = p.expr()
	}

	p.end()

	return s
}

// parseContinue parses CONTINUE (section 5.2).
func parseContinue(p *parser) ast.Stmt {
	p.end()

	return &ast.ContinueStmt{StmtBase: p.base()}
}

// parseStop parses STOP [disp] (section 5.10), where disp is a character
// constant or one to five digits.
func parseStop(p *parser) ast.Stmt {
	s := &ast.StopStmt{StmtBase: p.base()}

	if t := p.peek(); t != nil {
		const want = "a character constant or 1 to 5 digits"

		base := ast.ExprBase{Pos: p.pos(*t)}

		switch {
		case t.Kind == lex.CharConst:
			s.Display = &ast.CharConst{ExprBase: base, Value: t.Text}
		case t.Kind == lex.IntegerConst && len(t.Text) <= 5:
			s.Display = &ast.IntConst{ExprBase: base, Text: t.Text}
		default:
			p.syntax(want)
		}

		p.next()
	}

	p.end()

	return s
}

// parseReturn parses RETURN [i] (section 5.9). An alternate return's
// number is kept for semantic analysis to report until Phase 79.
func parseReturn(p *parser) ast.Stmt {
	s := &ast.ReturnStmt{StmtBase: p.base()}

	if !p.atEnd() {
		s.Alt = p.expr()
	}

	p.end()

	return s
}

// parseEnd parses END (section 5.5).
func parseEnd(p *parser) ast.Stmt {
	p.end()

	return &ast.EndStmt{StmtBase: p.base()}
}
