package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// WHERE (Fortran 90's 7.5.3; docs/planning/PHASE-102 - Fortran 90 array
// expressions.md, finding 8):
//
//	WHERE (A > 0.0)
//	   B = SQRT(A)
//	ELSEWHERE
//	   B = 0.0
//	END WHERE
//
// and the one-statement form, WHERE (A > 0.0) B = SQRT(A). The mask is a
// LOGICAL array; each assignment's target an array of the mask's shape, its
// value a scalar or an array of that shape too. An assignment's elements are
// assigned only where the mask is true, after ELSEWHERE only where it is false.
// Only assignments may be in the construct (WHEREBODY); a WHERE inside a WHERE,
// and ELSEWHERE (mask), are Fortran 95's (F95ONLY).
//
// WHERE opens a construct on the analyzer's stack, as IF THEN does, with the
// WHERE's assignments one arm and ELSEWHERE's another, so add and the paths of
// labels treat it as they treat an IF block: a jump from outside to any
// statement in it but the WHERE is INTOBLOCK (the standard's 8.2 lets only the
// WHERE be a branch target), and nothing inside can jump, being assignments.
//
// The mask is computed once, at the WHERE, as if before any assignment
// (7.5.3.2). Where nothing an assignment of the construct writes is read by the
// mask before the last of them, the scalarizer computes the mask in each
// assignment's loops, element by element, beside the value: the common WHERE
// (A > 0) A = SQRT(A) needs no more. Otherwise the mask goes first into a
// LOGICAL array temporary of its shape (Where.MaskTemp), which each nest reads,
// of a size known when compiling or only when the program runs (docs/planning/
// PHASE-104 - Fortran 90 dynamic arrays.md, finding 8). The test is
// finishWhere's.

// whereStatement analyzes the WHERE statement, WHERE (mask) assignment: a
// construct of one assignment.
func (a *analyzer) whereStatement(s *ast.WhereStmt) Stmt {
	if a.inWhere() {
		a.report("F95ONLY", s.Pos, "a WHERE statement inside a WHERE construct")

		return nil
	}

	w := a.newWhere(s, s.Mask)
	assign, _ := s.Then.(*ast.AssignStmt)

	if assign == nil {
		return nil
	}

	if st := a.whereAssign(w, assign); st != nil && w != nil {
		w.Then.Body = append(w.Then.Body, st)
	}

	if w == nil || !a.finishWhere(w, s.Pos) {
		return nil
	}

	return w
}

// whereBlock analyzes WHERE (mask), which opens a WHERE construct.
func (a *analyzer) whereBlock(s *ast.WhereBlockStmt) {
	if a.inWhere() {
		// The construct is kept, with no Where, so its statements are matched
		// and its END WHERE closes it.
		a.report("F95ONLY", s.Pos, "a WHERE construct inside a WHERE construct")
		a.push(&construct{whereStart: s, arm: &Arm{AST: s}, reported: true})

		return
	}

	w := a.newWhere(s, s.Mask)
	c := &construct{where: w, whereStart: s, arm: &Arm{AST: s}}

	if w != nil {
		c.arm = w.Then
		a.add(w)
	}

	a.push(c)
}

// newWhere is the Where of a WHERE statement s with its mask, or nil after an
// error in the mask (WHEREMASK if it isn't a LOGICAL or INTEGER array).
func (a *analyzer) newWhere(s ast.Stmt, mask ast.Expr) *Where {
	m := a.arrayValue(mask)

	switch t := m.Type(); {
	case t.Kind == Invalid:
		return nil

	case rankOf(m) == 0:
		a.report("WHEREMASK", mask.Position(), "a scalar")

		return nil

	case t.Kind == Integer:
		// An INTEGER is a condition by its bit 0, as an IF's is.
		m = a.convert(m, Type{Kind: Logical, Size: t.Size})

	case t.Kind != Logical:
		a.report("WHEREMASK", mask.Position(), article(t.String())+" array")

		return nil
	}

	return &Where{stmtBase: stmtBase{s}, Mask: m, Then: &Arm{AST: s}}
}

// inWhere reports whether a WHERE construct is open.
func (a *analyzer) inWhere() bool {
	for _, c := range a.open {
		if c.isWhere() {
			return true
		}
	}

	return false
}

// whereBody reports a statement in a WHERE construct that may not be there,
// WHEREBODY: anything but an assignment, ELSEWHERE, END WHERE, and the WHERE
// statements (which are F95ONLY, reported by their own analysis). It keeps the
// statement's part in the block structure, and reports whether it did.
func (a *analyzer) whereBody(s ast.Stmt) bool {
	c := a.top()
	if c == nil || !c.isWhere() {
		return false
	}

	switch s.(type) {
	case *ast.AssignStmt, *ast.ElseWhereStmt, *ast.EndWhereStmt, *ast.WhereStmt, *ast.WhereBlockStmt:
		return false

	case *ast.EndStmt:
		// The construct is NOENDWHERE, from closeAll.
		return false
	}

	a.report("WHEREBODY", s.Position(), aStatement(s))
	a.brokenStatement(s.Base().Kind, s)

	return true
}

// whereAssignment analyzes an assignment in the WHERE construct c, for the
// statement walk to add to the construct's current block.
func (a *analyzer) whereAssignment(c *construct, s *ast.AssignStmt) Stmt {
	if st := a.whereAssign(c.where, s); st != nil {
		return st
	}

	return nil
}

// whereAssign analyzes an assignment the Where w masks (w nil after an error in
// its mask, when the assignment is only checked): its target an array of the
// mask's shape (WHERETARGET, NOTCONFORM). Its order is finishWhere's, when the
// construct's every assignment is known. Nil after an error.
func (a *analyzer) whereAssign(w *Where, s *ast.AssignStmt) *Assign {
	target, value := a.assignParts(s)
	if target == nil {
		return nil
	}

	if rankOf(target) == 0 {
		a.report("WHERETARGET", s.Target.Position())

		return nil
	}

	if w == nil {
		return nil
	}

	if _, ok := a.conform(s.Pos, w.Mask.Shape(), target.Shape()); !ok {
		return nil
	}

	return &Assign{stmtBase{s}, target, value, nil}
}

// matchWhere finds the WHERE construct an ELSEWHERE or END WHERE statement
// belongs to (match): NOWHERE if there is none.
func (a *analyzer) matchWhere(s ast.Stmt) *construct {
	return a.match(s, (*construct).isWhere, "NOWHERE")
}

// quietMatchWhere is matchWhere without messages, for a statement with an
// error already reported.
func (a *analyzer) quietMatchWhere() *construct {
	for c := a.top(); c != nil && !c.isWhere(); c = a.top() {
		a.pop()
	}

	return a.top()
}

// elseWhere analyzes ELSEWHERE, which starts the construct's second block.
func (a *analyzer) elseWhere(s *ast.ElseWhereStmt) {
	c := a.matchWhere(s)
	if c == nil {
		return
	}

	if s.Mask != nil {
		a.report("F95ONLY", s.Mask.Position(), "ELSEWHERE with a mask")
	}

	if c.elseAt != nil {
		a.report("ELSEWHEREORDER", s.Pos, c.elseAt.Position().Line)
	}

	c.arm, c.elseAt = &Arm{AST: s}, s

	if c.where != nil {
		c.where.Else = c.arm
	}
}

// endWhere analyzes END WHERE, which closes its construct.
func (a *analyzer) endWhere(s *ast.EndWhereStmt) {
	c := a.matchWhere(s)
	if c == nil {
		return
	}

	a.pop()

	if c.where != nil {
		c.where.EndWhere = s

		if !a.finishWhere(c.where, s.Pos) {
			// The construct's code is left out: its NOTIMPL is reported.
			c.where.Then.Body, c.where.Mask = nil, nil
			if c.where.Else != nil {
				c.where.Else.Body = nil
			}
		}
	}
}

// finishWhere decides how the Where w runs, once its assignments are known:
// each assignment's order (arrayOrder, never filled from a constructor, which
// the mask may skip parts of), and whether the mask goes into a temporary
// first. False after NOTIMPL.
func (a *analyzer) finishWhere(w *Where, pos diag.Position) bool {
	a.noFill = true

	for _, arm := range w.arms() {
		kept := arm.Body[:0]

		for _, st := range arm.Body {
			s := st.(*Assign)

			order, ok := a.arrayOrder(s.AST.Position(), s.Target, s.Value)
			if !ok {
				continue
			}

			s.Order = order
			kept = append(kept, s)
		}

		arm.Body = kept
	}

	a.noFill = false

	if downs, ok := a.maskInLoops(w); ok {
		for s, need := range downs {
			for i, d := range need {
				s.Order.Down[i] = d < 0
			}
		}

		return true
	}

	w.MaskTemp = tempShape(w.Mask.Shape())

	return true
}

// maskInLoops reports whether each assignment's loops may compute the mask
// themselves, element by element, and then the directions its loops must run
// in, the mask's arrays' needs with the value's. It may if
//
//   - no assignment writes storage the mask reads before the last assignment
//     has read the mask (an earlier assignment's target is apart from it);
//   - in each assignment, the mask's arrays relate to the target in a way the
//     loops can keep (the same element, or ordered the way the value's are),
//     or, where the value goes through a temporary, the mask reads nothing of
//     the target's at all, since the second nest reads it while it writes;
//   - and, with more than one assignment, computing the mask more than once
//     is safe: it calls no procedure of the program's.
func (a *analyzer) maskInLoops(w *Where) (map[*Assign][]int, bool) {
	var list []*Assign

	for _, arm := range w.arms() {
		for _, st := range arm.Body {
			list = append(list, st.(*Assign))
		}
	}

	if len(list) > 1 && callsProcedure(w.Mask) {
		return nil, false
	}

	downs := map[*Assign][]int{}

	for j, s := range list {
		ts := baseSymbol(s.Target)

		for _, earlier := range list[:j] {
			if arrayReads(w.Mask, baseSymbol(earlier.Target)) {
				return nil, false
			}
		}

		if s.Order.Temporary {
			if arrayReads(w.Mask, ts) {
				return nil, false
			}

			continue
		}

		need := append([]int(nil), s.Order.need...)
		if !relateValue(s.Target, w.Mask, need) {
			return nil, false
		}

		downs[s] = need
	}

	return downs, true
}

// callsProcedure reports whether e calls a function or statement function of
// the program's anywhere: in an operand, an argument, or a subscript.
func callsProcedure(e Expr) bool {
	found := false

	var walk func(Expr)

	walk = func(e Expr) {
		switch e := e.(type) {
		case nil:
		case *FuncRef, *StmtFuncRef:
			found = true
		case *Elem:
			for _, s := range e.Subs {
				walk(s)
			}
		case *Section:
			for _, s := range e.Subs {
				walk(s.Index)
				walk(s.Vector)
				walk(s.Lower)
				walk(s.Upper)
				walk(s.Stride)
			}
		case *Substring:
			walk(e.X)
			walk(e.Lower)
			walk(e.Upper)
		case *Concat:
			for _, p := range e.Parts {
				walk(p)
			}
		case *Convert:
			walk(e.X)
		case *Unary:
			walk(e.X)
		case *Binary:
			walk(e.X)
			walk(e.Y)
		case *Compare:
			walk(e.X)
			walk(e.Y)
		case *IntrinsicRef:
			for _, x := range e.Args {
				walk(x)
			}
		case *ArrayFunc:
			walk(e.Array)
			walk(e.Other)
			walk(e.Mask)
			walk(e.DimExpr)
		case *Constructor:
			// Its values are any expressions: taken to call one.
			found = true
		}
	}

	walk(e)

	return found
}

// arms are the Where's blocks: the WHERE's, then ELSEWHERE's if it has one.
func (w *Where) arms() []*Arm {
	if w.Else == nil {
		return []*Arm{w.Then}
	}

	return []*Arm{w.Then, w.Else}
}

// brokenWhere keeps the block structure of a WHERE construct's statement
// with an error (brokenStatement's).
func (a *analyzer) brokenWhere(kind lex.Kind, s ast.Stmt) {
	switch kind {
	case lex.WhereBlock:
		a.push(&construct{whereStart: s, arm: &Arm{AST: s}, reported: true})

	case lex.ElseWhere:
		if c := a.quietMatchWhere(); c != nil {
			c.arm = &Arm{AST: s}
		}

	case lex.EndWhere:
		if c := a.quietMatchWhere(); c != nil {
			a.pop()
		}
	}
}
