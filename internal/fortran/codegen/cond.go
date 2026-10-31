package codegen

import "github.com/tucats/govax/internal/fortran/ir"

// The branch conditions after a CMP or TST, by the comparison that
// holds: the suffix of the branch instruction (BEQL, BLSS, ...). The VAX's
// signed branches look only at N and Z, which CMPL, CMPF, TSTL, TSTF, and
// every instruction that writes a value set from that value.
var conditions = map[ir.Operator]string{
	ir.Eq: "EQL", ir.Ne: "NEQ", ir.Lt: "LSS", ir.Le: "LEQ", ir.Gt: "GTR", ir.Ge: "GEQ",
}

// opposite is the condition that holds when another doesn't.
var opposite = map[string]string{
	"EQL": "NEQ", "NEQ": "EQL", "LSS": "GEQ", "GEQ": "LSS", "LEQ": "GTR", "GTR": "LEQ",
	"LSSU": "GEQU", "GEQU": "LSSU", "LEQU": "GTRU", "GTRU": "LEQU",
}

// swapped is the comparison that holds of Y and X when op holds of X and
// Y: X .LT. Y is Y .GT. X.
var swapped = map[ir.Operator]ir.Operator{
	ir.Eq: ir.Eq, ir.Ne: ir.Ne, ir.Lt: ir.Gt, ir.Le: ir.Ge, ir.Gt: ir.Lt, ir.Ge: ir.Le,
}

// comparing is a comparison whose operands are computed: X op Y, or,
// when test is set, X op 0, which TST does.
type comparing struct {
	op   ir.Operator
	t    ir.Type
	x, y operand
	test bool
}

// compareOperands computes a comparison's operands. A comparison with
// zero is a test of the other operand (TSTL X, not CMPL X,#0), with the
// operator turned round if the zero was first.
func (g *routineGen) compareOperands(e *ir.Cmp) comparing {
	op, x, y := e.Op, e.X, e.Y

	if isZero(x) && !isZero(y) {
		op, x, y = swapped[op], y, x
	}

	c := comparing{op: op, t: x.Type()}

	switch {
	case isZero(y):
		c.x, c.test = g.eval(x), true
	case need(y) > need(x):
		c.y = g.eval(y)
		c.x = g.eval(x)
	default:
		c.x = g.eval(x)
		c.y = g.eval(y)
	}

	return c
}

// compare writes the CMP or TST of a comparison whose operands are
// computed, frees them, and returns the branch condition for the
// comparison's holding. A TST of what the instruction before it wrote is
// left out: that instruction set the condition codes already (DECL
// count, then BLEQ).
func (g *routineGen) compare(c comparing) string {
	t := c.t.String()

	// What the instruction just before wrote is unchanged since (an
	// instruction between would have changed g.set), registers included.
	if !c.test {
		g.emit("CMP"+t, ir.None, c.x.text, c.y.text)
	} else if g.set != c.x.text || g.setType != c.t {
		g.emit("TST"+t, ir.None, c.x.text)
	}

	g.free(c.x, c.y)

	return conditions[c.op]
}

// isZero reports whether e is the constant 0 or 0.0.
func isZero(e ir.Expr) bool {
	c, ok := e.(*ir.Const)

	return ok && c.IsZero()
}

// branchTo writes the code that goes to target if bit 0 of e is sense
// (set for true, clear for false; appendix C.4 of the language manual
// makes bit 0 a LOGICAL's value), and falls through if not.
//
// Comparisons are compared and branched on, not computed as values.
// .NOT., .AND., and .OR. branch as they go, not computing what the
// second operand needn't decide (section 2.3.4 allows it): for X .AND. Y
// to be true, a false X jumps past the test of Y.
//
// Every branch reaches as far as BRW's word displacement, since nothing
// here knows how far its target is: a condition's branch to a target is
// the opposite condition's byte branch around a BRW.
//
//	BLEQ    unit.$5           (not greater: don't go)
//	BRW     target
//	unit.$5:
func (g *routineGen) branchTo(e ir.Expr, sense bool, target string) {
	switch e := e.(type) {
	case *ir.Const:
		if (e.Bits&1 != 0) == sense {
			g.emit("BRW", ir.None, target)
		}

		return

	case *ir.Cmp:
		cc := g.compare(g.compareOperands(g.fit(e).(*ir.Cmp)))
		if !sense {
			cc = opposite[cc]
		}

		g.around("B"+opposite[cc], nil, target)

		return

	case *ir.CharCmp:
		cc := g.charCompare(e)
		if !sense {
			cc = opposite[cc]
		}

		g.around("B"+opposite[cc], nil, target)

		return

	case *ir.Op:
		switch e.Op {
		case ir.Not:
			g.branchTo(e.X, !sense, target)

			return

		case ir.And, ir.Or:
			// X .OR. Y is true, and X .AND. Y false, if either operand
			// alone is: test each in turn.
			if (e.Op == ir.Or) == sense {
				g.branchTo(e.X, sense, target)
				g.branchTo(e.Y, sense, target)

				return
			}

			// Otherwise both must be: an X that isn't decides it the
			// other way, and goes past the test of Y.
			skip := g.newLabel()
			g.branchTo(e.X, !sense, skip)
			g.branchTo(e.Y, sense, target)
			g.label(skip, "")

			return
		}
	}

	// A LOGICAL's value (or an integer's, as a condition): its low bit.
	// BLBC and BLBS read a longword, which is past the end of a byte or word
	// in memory, so a shorter value there is tested with BIT. The branch
	// written goes around the jump to target, so it is the opposite of sense.
	v := g.eval(g.fit(e))
	g.free(v)

	if t := e.Type(); t.Size() < 4 && !v.isRegister() {
		op := "BEQL"
		if !sense {
			op = "BNEQ"
		}

		g.emit("BIT"+t.String(), ir.None, "#1", v.text)
		g.around(op, nil, target)

		return
	}

	op := "BLBC"
	if !sense {
		op = "BLBS"
	}

	g.around(op, []string{v.text}, target)
}

// around writes a branch to target that reaches any distance: the byte
// branch op (with its operands before the destination) to a label just
// past a BRW to target. op is the branch that should not go to target.
func (g *routineGen) around(op string, operands []string, target string) {
	skip := g.newLabel()
	g.emit(op, ir.None, append(operands, skip)...)
	g.emit("BRW", ir.None, target)
	g.label(skip, "")
}
