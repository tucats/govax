package codegen

import (
	"fmt"

	"github.com/tucats/govax/internal/fortran/ir"
)

// CHARACTER values (Decision 2 of docs/PHASE-78 - FORTRAN expressions and
// types.md): an assignment is a MOVC3 or MOVC5, a comparison a CMPC3 or
// CMPC5. Each takes its operands, a length and an address for each value,
// computed before it runs, and leaves R0 to R5 (MOVC) or R0 to R3 (CMPC)
// changed, so lowering makes sure that no register holds a value across
// one, and the entry mask saves the ones from R2 up for the caller.

// blank is a fill character's operand: MOVC5 fills a destination longer
// than its source with it, and CMPC5 compares the shorter string as though
// it went on in it (section 2.3.3 of the language manual: "the shorter
// operand is treated as if it were extended on the right with blanks").
const blank = "#^A/ /"

// unsigned are the branch conditions after a CMPC, by the comparison that
// holds. CMPC compares characters as unsigned bytes, setting C as CMPL
// sets it for unsigned longwords, so its order is the unsigned branches'
// (BLSSU, BGTRU, ...), which is ASCII's.
var unsigned = map[ir.Operator]string{
	ir.Eq: "EQL", ir.Ne: "NEQ", ir.Lt: "LSSU", ir.Le: "LEQU", ir.Gt: "GTRU", ir.Ge: "GEQU",
}

// textOperands computes the length and address operands of the character
// values given, in order: each value's length, then its address. Each is
// fitted to the register budget alone; together they hold at most one
// register each, so the pool always has room for the next.
func (g *routineGen) textOperands(texts ...ir.Text) []operand {
	g.holdingNothing()

	ops := make([]operand, 0, 2*len(texts))

	for _, t := range texts {
		ops = append(ops, g.eval(g.fit(t.Len)), g.mem(g.fitMem(t.Mem)))
	}

	return ops
}

// holdingNothing checks that no register holds a value, as lowering
// promises before a MOVC or a CMPC, which would change it.
func (g *routineGen) holdingNothing() {
	for i, busy := range g.inUse {
		if busy {
			panic(fmt.Sprintf("codegen: %s: R%d holds a value across a character instruction", g.r.Name, i))
		}
	}
}

// changes notes that an instruction leaves R0 to Rn changed: the routine
// saves the ones from R2 up for its caller.
func (g *routineGen) changes(n int) {
	for i := 0; i <= n; i++ {
		g.used[i] = true
	}
}

// constantLength is a length that is a constant, and whether it is one.
func constantLength(e ir.Expr) (int, bool) {
	if c, ok := e.(*ir.Const); ok {
		return int(c.Int()), true
	}

	return 0, false
}

// charMove writes Dst = Src: MOVC3 when both lengths are constants and the
// source is at least as long as the destination (a longer one is cut), and
// MOVC5 otherwise, which fills with blanks a destination longer than its
// source.
//
//	MOVC3   #8,PROG.T,PROG.S
//	MOVC5   #3,PROG.$K1,#^A/ /,#8,PROG.S
func (g *routineGen) charMove(s *ir.CharMove) {
	srcLen, srcOK := constantLength(s.Src.Len)
	dstLen, dstOK := constantLength(s.Dst.Len)

	ops := g.textOperands(s.Src, s.Dst)

	if srcOK && dstOK && srcLen >= dstLen {
		g.emit("MOVC3", ir.None, ops[2].text, ops[1].text, ops[3].text)
	} else {
		g.emit("MOVC5", ir.None, ops[0].text, ops[1].text, blank, ops[2].text, ops[3].text)
	}

	g.free(ops...)
	g.changes(5)
}

// charCompare writes the CMPC3 (for two lengths known to be the same) or
// CMPC5 of a character comparison, and returns the branch condition for the
// comparison's holding.
func (g *routineGen) charCompare(e *ir.CharCmp) string {
	xLen, xOK := constantLength(e.X.Len)
	yLen, yOK := constantLength(e.Y.Len)

	ops := g.textOperands(e.X, e.Y)

	if xOK && yOK && xLen == yLen {
		g.emit("CMPC3", ir.None, ops[0].text, ops[1].text, ops[3].text)
	} else {
		g.emit("CMPC5", ir.None, ops[0].text, ops[1].text, blank, ops[2].text, ops[3].text)
	}

	g.free(ops...)
	g.changes(3)

	return unsigned[e.Op]
}

// charComparison computes a character comparison's value, a LOGICAL: -1 if
// it holds, 0 if not. The register is taken after the CMPC, which changes
// R0 to R3; and since CLRL would change the condition codes, each value has
// its own branch.
//
//	CMPC5   ...
//	BGEQU   unit.$5           (the comparison's opposite)
//	MNEGL   #1,R0
//	BRB     unit.$6
//	unit.$5:
//	CLRL    R0
//	unit.$6:
func (g *routineGen) charComparison(e *ir.CharCmp) operand {
	cc := g.charCompare(e)
	r := g.alloc(ir.Long)

	no, done := g.newLabel(), g.newLabel()
	g.emit("B"+opposite[cc], ir.None, no)
	g.emit("MNEGL", ir.Long, "#1", register(r))
	g.emit("BRB", ir.None, done)
	g.label(no, "")
	g.emit("CLRL", ir.Long, register(r))
	g.label(done, "")

	return inRegister(r, ir.Long)
}
