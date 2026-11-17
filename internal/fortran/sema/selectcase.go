package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Fortran 90's CASE construct (the standard's 8.1.3; docs/planning/PHASE-101
// - Fortran 90 control constructs.md, subtask 3):
//
//	PICK: SELECT CASE (expr)
//	CASE (1, 3:5, 10:) PICK
//	   ...
//	CASE DEFAULT PICK
//	   ...
//	END SELECT PICK
//
// SELECT CASE opens a construct on the analyzer's stack, as IF THEN does, and
// each CASE starts a new block of it, as ELSE IF starts an IF's: a Case
// embeds an Arm, so the stack's blocks, add, and the paths of labels treat
// the two alike. END SELECT closes it. Nothing may come between SELECT CASE
// and the first CASE (CASEFIRST).
//
// The expression is a scalar INTEGER, CHARACTER, or LOGICAL value (CASEEXPR),
// and each case value a constant expression (CASEVALUE) of the same type
// (CASETYPE): an INTEGER value of any kind for an INTEGER expression of any
// kind, since they are compared by value; a CHARACTER value of any length.
// A LOGICAL expression is selected only by single values (CASERANGE), and a
// LOGICAL*1 one, VAX FORTRAN's BYTE, by no number at all (CASEBYTE): the
// author's answer 1, which keeps the standard's rule and suggests
// INTEGER(1).
//
// No value of the expression may match two ranges (CASEOVERLAP), which is
// checked here on the constants, each range against every one before it in
// the construct: INTEGER ranges as intervals of integers, CHARACTER ranges
// by FORTRAN's comparison of text (the shorter padded with blanks, in ASCII,
// the order CMPC compares in at run time), LOGICAL values by their truth. A
// range that no value can match (its low end above its high end, or past
// what the expression's INTEGER kind holds) overlaps nothing, and is
// CASENEVER, a warning.
//
// The construct's code is the IR's (ir/lowerselect.go).

// caseSpan is a case value range as the overlap check compares it, with the
// line of its CASE statement: for an INTEGER expression the interval
// [ilo, ihi], open ends at the expression's kind's limits; for the others
// the ends' constants, nil for an open end.
type caseSpan struct {
	ilo, ihi int64
	lo, hi   *Const
	line     int
}

// beforeFirstCase reports a statement between a SELECT CASE and its first
// CASE (CASEFIRST), once for the construct.
func (a *analyzer) beforeFirstCase(s ast.Stmt) {
	c := a.top()
	if c == nil || !c.isSelect() || c.arm != nil || c.caseFirst {
		return
	}

	switch s.Base().Kind {
	case lex.Case, lex.EndSelect:
		return
	}

	a.report("CASEFIRST", s.Position(), aStatement(s))
	c.caseFirst = true
}

// selectCase analyzes SELECT CASE (e) and opens its construct.
func (a *analyzer) selectCase(s *ast.SelectCaseStmt) {
	sel := &Select{stmtBase: stmtBase{s}, Expr: a.value(s.Expr)}

	// A whole array is NEEDSUBSCRIPT already. An expression of another type
	// is reported here, and leaves the construct no type to check the case
	// values against.
	switch t := sel.Expr.Type(); t.Kind {
	case Integer, Character, Logical, Invalid:
	default:
		a.report("CASEEXPR", s.Expr.Position(), t.String())
		sel.Expr = a.bad(s.Expr.Position())
	}

	a.add(sel)
	a.push(&construct{sel: sel, selStart: s})
}

// matchSelect finds the CASE construct a CASE or END SELECT statement
// belongs to (match): NOSELECT if there is none.
func (a *analyzer) matchSelect(s ast.Stmt) *construct {
	return a.match(s, (*construct).isSelect, "NOSELECT")
}

// quietMatchSelect is matchSelect without messages, for a statement with
// an error already reported.
func (a *analyzer) quietMatchSelect() *construct {
	for c := a.top(); c != nil && !c.isSelect(); c = a.top() {
		a.pop()
	}

	return a.top()
}

// caseStatement analyzes CASE (ranges) or CASE DEFAULT, which starts a new
// block of its construct.
func (a *analyzer) caseStatement(s *ast.CaseStmt) {
	c := a.matchSelect(s)
	if c == nil {
		return
	}

	a.checkName(c, s, false)

	cs := &Case{Arm: Arm{AST: s}, Default: s.Default}
	c.arm = &cs.Arm

	if c.sel == nil {
		// The SELECT CASE statement had an error: nothing to check
		// against.
		return
	}

	c.sel.Cases = append(c.sel.Cases, cs)

	if s.Default {
		if c.sel.Default != nil {
			a.report("TWODEFAULT", s.Position(), c.sel.Default.AST.Position().Line)

			return
		}

		c.sel.Default = cs

		return
	}

	for _, r := range s.Ranges {
		if cr, ok := a.caseRange(c, r); ok {
			cs.Ranges = append(cs.Ranges, cr)
		}
	}
}

// caseRange analyzes one case value range of c's construct: its ends'
// values and types, and whether it can match at all and overlaps a range
// before it.
func (a *analyzer) caseRange(c *construct, r ast.CaseRange) (CaseRange, bool) {
	t := c.sel.Expr.Type()

	if r.Colon && t.Kind == Logical {
		a.report("CASERANGE", r.Pos)

		return CaseRange{}, false
	}

	var cr CaseRange

	ok := true

	for _, end := range []struct {
		e    ast.Expr
		into **Const
	}{{r.Low, &cr.Low}, {r.High, &cr.High}} {
		if end.e != nil {
			v, good := a.caseValue(c, end.e)
			*end.into, ok = v, ok && good
		}
	}

	if !ok {
		return CaseRange{}, false
	}

	if !r.Colon {
		cr.High = cr.Low
	}

	if t.Kind == Invalid {
		return cr, true
	}

	span, never := a.caseSpan(t, cr)

	if never != "" {
		a.report("CASENEVER", r.Pos, never)

		return cr, true
	}

	for _, other := range c.spans {
		if overlaps(t, span, other) {
			a.report("CASEOVERLAP", r.Pos, other.line)

			break
		}
	}

	span.line = c.arm.AST.Position().Line
	c.spans = append(c.spans, span)

	return cr, true
}

// caseValue is the value of one end of a case value range: a constant of
// the expression's type (CASEVALUE, CASETYPE, CASEBYTE otherwise).
func (a *analyzer) caseValue(c *construct, e ast.Expr) (*Const, bool) {
	v, err := a.declEval().eval(e)

	switch {
	case err != nil && err.kind == constFailed:
		return nil, false
	case err != nil:
		a.report("CASEVALUE", err.pos, err.why)

		return nil, false
	}

	t := c.sel.Expr.Type()

	switch {
	case t.Kind == Invalid || v.T.Kind == t.Kind:
		return v, true
	case t.Kind == Logical && t.Size == 1 && v.T.Kind == Integer:
		a.report("CASEBYTE", e.Position(), c.sel.AST.Position().Line)
	default:
		a.report("CASETYPE", e.Position(), v.T.String(), c.sel.AST.Position().Line, t.String())
	}

	return nil, false
}

// caseSpan is r as the overlap check compares it, for an expression of type
// t; never says why r can match no value, or is "".
func (a *analyzer) caseSpan(t Type, r CaseRange) (caseSpan, string) {
	const reversed = "its low end is above its high end"

	if t.Kind != Integer {
		span := caseSpan{lo: r.Low, hi: r.High}

		if r.Low != nil && r.High != nil && !r.Single() && caseOrder(t, r.Low, r.High) > 0 {
			return span, reversed
		}

		return span, ""
	}

	kindLo, kindHi := t.intRange()
	span := caseSpan{ilo: kindLo, ihi: kindHi}

	if r.Low != nil {
		span.ilo = max(span.ilo, int64(r.Low.Int))
	}

	if r.High != nil {
		span.ihi = min(span.ihi, int64(r.High.Int))
	}

	switch {
	case r.Low != nil && r.High != nil && r.Low.Int > r.High.Int:
		return span, reversed
	case span.ilo > span.ihi:
		return span, "the SELECT CASE expression is " + t.String() + ", which can't hold it"
	}

	return span, ""
}

// overlaps reports whether some value of an expression of type t lies in
// both x and y.
func overlaps(t Type, x, y caseSpan) bool {
	if t.Kind == Integer {
		return max(x.ilo, y.ilo) <= min(x.ihi, y.ihi)
	}

	atOrBelow := func(lo, hi *Const) bool { return lo == nil || hi == nil || caseOrder(t, lo, hi) <= 0 }

	return atOrBelow(x.lo, y.hi) && atOrBelow(y.lo, x.hi)
}

// caseOrder is -1, 0, or 1 as the CHARACTER or LOGICAL case value x comes
// before, is equal to, or comes after y: text by FORTRAN's comparison, a
// LOGICAL .FALSE. before .TRUE.
func caseOrder(t Type, x, y *Const) int {
	if t.Kind == Character {
		return compareText(x.Text, y.Text)
	}

	xt, yt := x.Int != 0, y.Int != 0

	switch {
	case xt == yt:
		return 0
	case yt:
		return -1
	}

	return 1
}

// endSelect analyzes END SELECT, which closes its construct.
func (a *analyzer) endSelect(s *ast.EndSelectStmt) {
	c := a.matchSelect(s)
	if c == nil {
		return
	}

	a.checkName(c, s, true)

	if c.sel != nil {
		c.sel.EndSelect = s
	}

	a.pop()
}
