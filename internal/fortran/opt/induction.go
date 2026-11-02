package opt

import (
	"github.com/tucats/govax/internal/fortran/ir"
)

// Induction variables and strength reduction: stepping what a multiplication computes.
//
//	      DO 60 I = 1, N
//	         S = S + C(I,I)
//	   60 CONTINUE
//
// With 60 rows, C(I,I)'s index is I*60 + I. The VAX's indexed mode scales an index by its
// element's size, so a loop along a column doesn't need any of this: A(I,K) is (R7)[R6].
// But this stride is 61 elements, and each trip multiplies again:
//
//	T:      MULL3   R6,#60,R0
//	        ADDL2   R6,R0
//	        ADDF2   C-244[R0],R8
//	        AOBLEQ  #60,R6,T
//
// I is an induction variable: nothing in the loop changes it but adding a constant. And
// I*60 + I is linear in it, so each time I goes up by 1, the index goes up by 61. A register
// can keep the index itself, and the multiplication becomes an addition:
//
//	        MULL3   R6,#60,R7                ; before the loop, then ADDL2 R6,R7
//	T:      ADDF2   C-244[R7],R8
//	        ADDL2   #61,R7
//	        AOBLEQ  #60,R6,T
//
// That is one instruction fewer, and an MULL3 was the slowest of them. The same goes for a
// loop along a row, A(K,J) in a loop on J, and a CHARACTER*8 array, whose stride is 8 bytes
// and whose index mode can't scale by that.
//
// # What it looks for
//
// In each loop, the variables that are induction variables: every store in the loop to one
// adds a constant to it, nothing overlapping it is written, it isn't VOLATILE, and no call in the
// loop may change it. A variable stepped in two places, or only sometimes, is one too:
// each step gets its addition, just before it, so the index and I never disagree when anything
// reads them. (Before and not after, so that the INCL and the CMPL stay together for AOBLEQ.)
//
// Then the expressions that are linear in one of those and have a multiplication in them:
// sums and differences of the variable, a constant times it, and what the loop doesn't change.
// A variable times what the loop doesn't change (an adjustable array's stride) can be
// stepped by that value too, if the variable's steps are 1 or -1. Without a multiplication
// there's nothing to gain: I + J is (R7)[R6] (registers.go). What doesn't change must be
// safe to read before the loop, as with invariant.go's rule. All of this is arithmetic modulo
// 2**32, which is what the multiplications did anyway, with overflow not enabled.
//
// # What it's worth
//
// This is one of the register allocator's candidates, because in memory it's no gain at all:
// an addition to memory, then a MOVL to get the index out. So a stepped value gets a
// register or isn't done, as with every other candidate, and competes on what it saves: the
// instructions and reads the expression took, a multiplication counted as two, less an
// addition per step. It's rewritten before what it reads gets its register, so it finds
// the program's variables as they were; what goes before the loop is then rewritten with
// the rest of an outer loop.

// valueStep is what keeps a stepped value up to date: by is added to it, just before
// advance.
type valueStep struct {
	advance *ir.Assign
	by      ir.Expr
}

// linearForm is an expression as coefficient*v + what the loop doesn't change: if factor
// isn't nil, it's coefficient*factor*v.
type linearForm struct {
	v           *ir.Cell
	coefficient int64
	factor      ir.Expr
	multiplied  bool
}

// inductionStep is one store of an induction variable.
type inductionStep struct {
	advance *ir.Assign
	by      int64
}

// steppedKey is what says expression e is replaced in loop l.
func steppedKey(e ir.Expr, l *Loop) string {
	return exprKey(e) + "|" + l.Header.Name
}

// findStepped lists what can be stepped in each loop.
func (a *allocator) findStepped() {
	for _, l := range a.g.Loops {
		if !a.usable(l) {
			continue
		}

		variables := a.inductionVariables(l)
		if len(variables) == 0 {
			continue
		}

		found := map[string]*candidate{}

		var inOrder []*candidate

		walk := walker{visit: func(e ir.Expr, _, _ bool) bool {
			c := found[exprKey(e)]
			if c == nil {
				c = a.steppedCandidate(l, variables, e)
				if c == nil {
					return true
				}

				found[c.key] = c
				inOrder = append(inOrder, c)
			}

			// Each use saves what computing it took, and a multiplication counts twice.
			p := costOf(e)
			c.gain += a.weight(l.Header) * (p.instructions + p.reads + multiplications(e))

			return false
		}}

		for _, b := range l.Blocks {
			if a.g.LoopOf(b) != l || a.expressions[b] {
				continue
			}

			for _, st := range b.Stmts {
				walk.stmt(st)
			}

			walk.terminator(b.End)
		}

		for _, c := range inOrder {
			a.candidates = append(a.candidates, c)
			a.stepped[steppedKey(c.source, l)] = true
		}
	}
}

// multiplications is how many multiplications e does.
func multiplications(e ir.Expr) int {
	op, ok := e.(*ir.Op)
	if !ok {
		return 0
	}

	n := multiplications(op.X)
	if op.Y != nil {
		n += multiplications(op.Y)
	}

	if op.Op == ir.Mul {
		n++
	}

	return n
}

// inductionVariables are loop l's induction variables, with their steps.
func (a *allocator) inductionVariables(l *Loop) map[*ir.Cell][]inductionStep {
	variables := map[*ir.Cell][]inductionStep{}

	for _, b := range l.Blocks {
		for _, st := range b.Stmts {
			if as, ok := st.(*ir.Assign); ok {
				if by, is := stepOfCell(as); is {
					variables[as.Dst.Cell] = append(variables[as.Dst.Cell], inductionStep{as, by})
				}
			}
		}
	}

	for cell, steps := range variables {
		self := a.s.Place(&ir.Mem{Cell: cell, T: cell.T})
		if a.s.Volatile(self) || (a.calls[l] && a.s.ChangedByCall(self)) || !a.onlyStepped(l, self, steps) {
			delete(variables, cell)
		}
	}

	return variables
}

// stepOfCell is what a statement adds to a whole Long cell, if that's what it does.
func stepOfCell(as *ir.Assign) (int64, bool) {
	d := as.Dst
	if d.Cell == nil || d.T != ir.Long || d.Cell.T != ir.Long || d.Index != nil || d.Ptr != nil || d.Offset != 0 || d.Volatile {
		return 0, false
	}

	op, ok := as.Src.(*ir.Op)
	if !ok || op.T != ir.Long || (op.Op != ir.Add && op.Op != ir.Sub) || !isWholeLoad(op.X, d.Cell) {
		return 0, false
	}

	k, ok := op.Y.(*ir.Const)
	if !ok {
		return 0, false
	}

	if op.Op == ir.Sub {
		return -int64(k.Int()), true
	}

	return int64(k.Int()), true
}

// isWholeLoad reports whether e reads the whole of Long cell c.
func isWholeLoad(e ir.Expr, c *ir.Cell) bool {
	l, ok := e.(*ir.Load)

	return ok && l.Mem.Cell == c && exact(l.Mem, ir.Long)
}

// onlyStepped reports whether nothing in loop l writes place self but the steps.
func (a *allocator) onlyStepped(l *Loop, self Place, steps []inductionStep) bool {
	for _, b := range l.Blocks {
		for _, st := range b.Stmts {
			if !isStep(st, steps) && overlapsAny(self, a.s.StmtEffects(st).Writes) {
				return false
			}
		}
	}

	return true
}

// isStep reports whether st is one of the steps.
func isStep(st ir.Stmt, steps []inductionStep) bool {
	for _, step := range steps {
		if ir.Stmt(step.advance) == st {
			return true
		}
	}

	return false
}

// steppedCandidate is the candidate for expression e in loop l, nil if it can't be one.
func (a *allocator) steppedCandidate(l *Loop, variables map[*ir.Cell][]inductionStep, e ir.Expr) *candidate {
	if _, isOp := e.(*ir.Op); !isOp || e.Type() != ir.Long {
		return nil
	}

	form, ok := a.linear(l, variables, e)
	if !ok || !form.multiplied || form.coefficient == 0 || hasCallValue(e) {
		return nil
	}

	steps := make([]valueStep, 0, len(variables[form.v]))

	for _, s := range variables[form.v] {
		by := stepBy(form, s.by)
		if by == nil {
			return nil
		}

		steps = append(steps, valueStep{s.advance, by})
	}

	return &candidate{
		kind: steppedCandidate, t: ir.Long, loop: l, source: e, key: exprKey(e), steps: steps,
		holds: "an index stepped with " + form.v.Name,
		gain:  -loopWeight(l.Depth-1) - a.weight(l.Header)*len(steps),
	}
}

// stepBy is what the value of form goes up by when its variable goes up by by, nil if that
// can't be said in one operand.
func stepBy(form linearForm, by int64) ir.Expr {
	k := form.coefficient * by

	if form.factor == nil {
		if int64(int32(k)) != k {
			return nil
		}

		return ir.LongConst(int32(k))
	}

	switch k {
	case 1:
		return form.factor
	case -1:
		return &ir.Op{Op: ir.Neg, T: ir.Long, X: form.factor}
	}

	return nil
}

// linear is e as a linear form, in loop l.
func (a *allocator) linear(l *Loop, variables map[*ir.Cell][]inductionStep, e ir.Expr) (linearForm, bool) {
	if load, ok := e.(*ir.Load); ok && load.Mem.Cell != nil && exact(load.Mem, ir.Long) {
		if _, is := variables[load.Mem.Cell]; is {
			return linearForm{v: load.Mem.Cell, coefficient: 1}, true
		}

		return linearForm{}, false
	}

	op, ok := e.(*ir.Op)
	if !ok || op.T != ir.Long {
		return linearForm{}, false
	}

	switch op.Op {
	case ir.Mul:
		return a.linearProduct(l, variables, op)
	case ir.Add, ir.Sub:
		return a.linearSum(l, variables, op)
	}

	return linearForm{}, false
}

// linearProduct is a product as a linear form: a constant, or what the loop doesn't
// change if the rest is the variable alone, times a linear form.
func (a *allocator) linearProduct(l *Loop, variables map[*ir.Cell][]inductionStep, op *ir.Op) (linearForm, bool) {
	for _, pair := range [][2]ir.Expr{{op.X, op.Y}, {op.Y, op.X}} {
		times, rest := pair[0], pair[1]

		form, ok := a.linear(l, variables, rest)
		if !ok {
			continue
		}

		if k, isConst := times.(*ir.Const); isConst {
			form.coefficient *= int64(k.Int())
			form.multiplied = true

			if form.factor == nil || form.coefficient != 0 {
				return form, true
			}
		}

		if form.factor == nil && form.coefficient == 1 && !form.multiplied && a.fixedPart(l, times) {
			form.factor = times
			form.multiplied = true

			return form, true
		}
	}

	return linearForm{}, false
}

// linearSum is a sum or difference as a linear form: of two in one variable, or of one and
// what the loop doesn't change.
func (a *allocator) linearSum(l *Loop, variables map[*ir.Cell][]inductionStep, op *ir.Op) (linearForm, bool) {
	x, okX := a.linear(l, variables, op.X)
	y, okY := a.linear(l, variables, op.Y)

	sign := int64(1)
	if op.Op == ir.Sub {
		sign = -1
	}

	switch {
	case okX && okY:
		if x.v != y.v || x.factor != nil || y.factor != nil {
			return linearForm{}, false
		}

		x.coefficient += sign * y.coefficient
		x.multiplied = x.multiplied || y.multiplied

		return x, true

	case okX && a.unchangingOperand(l, op.Y):
		return x, true

	case okY && a.unchangingOperand(l, op.X):
		y.coefficient *= sign

		return y, true
	}

	return linearForm{}, false
}

// unchangingOperand reports whether e is a constant, or what loop l doesn't change and can
// be read before it.
func (a *allocator) unchangingOperand(l *Loop, e ir.Expr) bool {
	if _, isConst := e.(*ir.Const); isConst {
		return true
	}

	return a.fixedPart(l, e)
}

// keepStepping puts each of a stepped value's additions before the step it goes with.
func (a *allocator) keepStepping(l *Loop, c *candidate, cell *ir.Cell) {
	for _, step := range c.steps {
		place := &ir.Mem{Cell: cell, T: ir.Long}
		add := ir.NewAssign(step.advance.Source(), place, addTo(place, step.by))

		for _, b := range l.Blocks {
			for i, st := range b.Stmts {
				if st == ir.Stmt(step.advance) {
					b.Stmts = append(b.Stmts[:i:i], append([]ir.Stmt{add}, b.Stmts[i:]...)...)

					break
				}
			}
		}
	}
}

// addTo is the value of place plus by.
func addTo(place *ir.Mem, by ir.Expr) ir.Expr {
	if neg, ok := by.(*ir.Op); ok && neg.Op == ir.Neg {
		return &ir.Op{Op: ir.Sub, T: ir.Long, X: &ir.Load{Mem: place}, Y: neg.X}
	}

	return ir.Fold(ir.Add, ir.Long, &ir.Load{Mem: place}, by)
}
