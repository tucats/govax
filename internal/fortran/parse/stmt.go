package parse

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The executable statements, but for I/O (io.go).

// parseAssignment parses v = e (chapter 3). The target is a variable, an
// array element, a substring, or a record's field, or a statement
// function's definition, F(X) = X*X, which only the declarations can
// tell from an array element.
func parseAssignment(p *parser) ast.Stmt {
	s := &ast.AssignStmt{StmtBase: p.base()}
	s.Target = p.reference("a variable or an array element")
	p.expect("=")
	s.Value = p.expr()
	p.end()

	return s
}

// parseAssignLabel parses ASSIGN s TO v (section 3.5). Squeezed, ASSIGN
// 10 TO I is ASSIGN10TOI, so the tokenizer makes 10 and a name TOI, from
// which TO is taken here.
func parseAssignLabel(p *parser) ast.Stmt {
	s := &ast.AssignLabelStmt{StmtBase: p.base(), Target: p.labelRef()}

	t := p.peek()
	if t == nil || t.Kind != lex.Name || len(t.Text) <= 2 || !strings.HasPrefix(t.Text, "TO") {
		p.syntax("TO and an integer variable")
	}

	p.next()

	s.Var = &ast.Name{ExprBase: ast.ExprBase{Pos: p.st.Position(t.Offset + offsetAfter(p.st.Source.Text, t.Offset, 2))}, Name: t.Text[2:]}
	p.end()

	return s
}

// offsetAfter is how far into the statement's text, from offset, the
// n-th character outside blanks after offset is: where the variable's
// name starts after TO, wherever the source put blanks.
func offsetAfter(text string, offset, n int) int {
	k := 0

	for i := offset; i < len(text); i++ {
		if text[i] == ' ' || text[i] == '\t' {
			continue
		}

		if k == n {
			return i - offset
		}

		k++
	}

	return 0
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
			pos := p.here()
			p.next()
			s.Args = append(s.Args, &ast.AltReturn{ExprBase: ast.ExprBase{Pos: pos}, Target: p.labelRef()})

		case p.atKeyword():
			// Fortran 90's keyword argument, CALL SOLVE(A, TOL=1.0E-6)
			// (R1212): NAME= can't start anything else here.
			t := p.next()
			p.next()

			s.Args = append(s.Args, &ast.KeywordArg{ExprBase: ast.ExprBase{Pos: p.pos(t)}, Name: t.Text, X: p.expr()})

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

// parseGoTo parses the three GO TO statements (section 5.6): the
// unconditional GO TO s, the computed GO TO (s[,s]...)[,] e, and the
// assigned GO TO v [[,](s[,s]...)].
func parseGoTo(p *parser) ast.Stmt {
	switch {
	case p.at("("):
		s := &ast.ComputedGotoStmt{StmtBase: p.base(), Targets: p.labelList()}
		p.accept(",")
		s.Index = p.expr()
		p.end()

		return s

	case p.atKind(lex.Name):
		s := &ast.AssignedGotoStmt{StmtBase: p.base(), Var: p.name("an integer variable")}

		if !p.atEnd() {
			p.accept(",")
			s.Targets = p.labelList()
		}

		p.end()

		return s
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

	s.Then = statement(then, p.messages, p.records)

	// If the statement had an error, the IF as a whole has one: later
	// passes needn't look inside it.
	if _, bad := s.Then.(*ast.BadStmt); bad {
		panic(bailout{})
	}

	return s
}

// parseArithIf parses the arithmetic IF, IF (e) s1, s2, s3 (section
// 5.7.1).
func parseArithIf(p *parser) ast.Stmt {
	s := &ast.ArithIfStmt{StmtBase: p.base(), Cond: p.condition()}

	for i := range s.Targets {
		if i > 0 {
			p.expect(",")
		}

		s.Targets[i] = p.labelRef()
	}

	p.end()

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
	case lex.Do, lex.DoWhile, lex.EndDo, lex.IfThen, lex.ElseIf, lex.Else, lex.EndIf, lex.End,
		lex.SelectCase, lex.Case, lex.EndSelect, lex.WhereBlock, lex.ElseWhere, lex.EndWhere:
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

// parseElseIf parses ELSE IF (e) THEN, and Fortran 90's ELSE IF (e) THEN name.
func parseElseIf(p *parser) ast.Stmt {
	s := &ast.ElseIfStmt{StmtBase: p.base(), Cond: p.condition()}
	p.expect("THEN")
	p.constructAfter(s)
	p.end()

	return s
}

// parseElse parses ELSE, and Fortran 90's ELSE name.
func parseElse(p *parser) ast.Stmt {
	s := &ast.ElseStmt{StmtBase: p.base()}
	p.constructAfter(s)
	p.end()

	return s
}

// parseEndIf parses END IF, and Fortran 90's END IF name.
func parseEndIf(p *parser) ast.Stmt {
	s := &ast.EndIfStmt{StmtBase: p.base()}
	p.constructAfter(s)
	p.end()

	return s
}

// parseDo parses DO [s[,]] v = e1, e2 [, e3] (section 5.3.1); VAX
// FORTRAN lets the label be left out, and END DO end the loop (5.4). The
// lexer has made sure there is a comma after the =, or it would have
// been an assignment (DO 10 I = 1.5 assigns to DO10I). Fortran 90's DO
// with no loop control at all (DO, DO 10) is a DoStmt with no variable.
func parseDo(p *parser) ast.Stmt {
	s := &ast.DoStmt{StmtBase: p.base(), Target: p.doLabel()}

	if p.atEnd() {
		return s
	}

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

// doLabel reads a DO statement's label and the comma that may follow it,
// or nothing for a loop without a label (whose LabelRef's Label is 0).
func (p *parser) doLabel() ast.LabelRef {
	if !p.atKind(lex.IntegerConst) {
		return ast.LabelRef{}
	}

	l := p.labelRef()
	p.accept(",")

	return l
}

// parseDoWhile parses DO [s[,]] WHILE (e) (section 5.3.2); the lexer has
// made WHILE a keyword token.
func parseDoWhile(p *parser) ast.Stmt {
	s := &ast.DoWhileStmt{StmtBase: p.base(), Target: p.doLabel()}
	p.expect("WHILE")
	s.Cond = p.condition()
	p.end()

	return s
}

// parseEndDo parses END DO (section 5.4), and Fortran 90's END DO name.
func parseEndDo(p *parser) ast.Stmt {
	s := &ast.EndDoStmt{StmtBase: p.base()}
	p.constructAfter(s)
	p.end()

	return s
}

// parseLoopExit parses Fortran 90's EXIT [name] and CYCLE [name] (the
// standard's R834 and R835). In fixed form EXIT OUTER is squeezed to
// EXITOUTER, which the lexer has already split after the keyword.
func parseLoopExit(p *parser) ast.Stmt {
	s := &ast.LoopExitStmt{StmtBase: p.base()}

	if p.atKind(lex.Name) {
		t := p.next()
		s.Name, s.NamePos = t.Text, p.pos(t)
	}

	p.end()

	return s
}

// parseSelectCase parses Fortran 90's SELECT CASE (e) (the standard's
// R809); a construct name before it is in the base already.
func parseSelectCase(p *parser) ast.Stmt {
	s := &ast.SelectCaseStmt{StmtBase: p.base(), Expr: p.condition()}
	p.end()

	return s
}

// parseCase parses CASE (range, ...) [name] and CASE DEFAULT [name] (the
// standard's R810 to R814). The lexer has made DEFAULT a keyword, and a
// construct name after it a name of its own, though fixed form squeezes CASE
// DEFAULT PICK to CASEDEFAULTPICK.
func parseCase(p *parser) ast.Stmt {
	s := &ast.CaseStmt{StmtBase: p.base()}

	if p.accept("DEFAULT") {
		s.Default = true
		p.constructAfter(s)
		p.end()

		return s
	}

	p.expect("(")

	for {
		s.Ranges = append(s.Ranges, p.caseRange())

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")
	p.constructAfter(s)
	p.end()

	return s
}

// caseRange reads one case value range: v, low:, :high, or low:high.
func (p *parser) caseRange() ast.CaseRange {
	r := ast.CaseRange{Pos: p.here()}

	if !p.at(":") {
		r.Low = p.expr()
	}

	if p.accept(":") {
		r.Colon = true

		if !p.at(",") && !p.at(")") {
			r.High = p.expr()
		}
	}

	if r.Low == nil && r.High == nil {
		// CASE (:) has neither end.
		p.syntax("a case value")
	}

	return r
}

// parseEndSelect parses END SELECT [name].
func parseEndSelect(p *parser) ast.Stmt {
	s := &ast.EndSelectStmt{StmtBase: p.base()}
	p.constructAfter(s)
	p.end()

	return s
}

// parseContinue parses CONTINUE (section 5.2).
func parseContinue(p *parser) ast.Stmt {
	p.end()

	return &ast.ContinueStmt{StmtBase: p.base()}
}

// parseStop parses STOP [disp] (section 5.10), and PAUSE [disp] (5.8),
// where disp is a character constant or one to five digits.
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

// parseReturn parses RETURN [i] (section 5.9), i an alternate return's
// number.
func parseReturn(p *parser) ast.Stmt {
	s := &ast.ReturnStmt{StmtBase: p.base()}

	if !p.atEnd() {
		s.Alt = p.expr()
	}

	p.end()

	return s
}
