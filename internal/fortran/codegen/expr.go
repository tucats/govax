package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ir"
)

// numRegisters is how many registers expressions use: R0 to R11 (Decision
// 3 of docs/PHASE-78 - FORTRAN expressions and types.md). AP, FP, SP, and
// PC are the calling standard's. A routine saves the ones from R2 up that
// it uses, in its entry mask, so a register costs nothing until it is used.
const numRegisters = 12

// budget is the most registers an expression may need, as need counts
// them. An expression that needs more has parts computed into spill
// cells first (fit). Four is an H_floating value's registers; it leaves
// room for what a statement holds besides: the two operands of the
// operation that stores into an array element, while the element's index
// is computed, or a function's value in R0. The pool is three times the
// budget so that a value's consecutive registers can always be found.
const budget = 4

// operand is an instruction's operand as MACRO writes it (SQUARES.I,
// #5, R0, @SUB.A(AP)[R1]), with the registers it holds, to be freed
// when the instruction using it is written.
type operand struct {
	text string
	regs []int
}

// register is the name of register n: R0.
func register(n int) string {
	return "R" + strconv.Itoa(n)
}

// inRegister is the operand that is a value of type t in register n: Rn
// alone, or for a D, G, or H value Rn and the registers after it, which
// an instruction names by the first (MOVD R2,X moves R2 and R3).
func inRegister(n int, t ir.Type) operand {
	regs := make([]int, t.Width())
	for i := range regs {
		regs[i] = n + i
	}

	return operand{text: register(n), regs: regs}
}

// isRegister reports whether o is a register (or a run of them) by
// itself, not a place in memory a register indexes.
func (o operand) isRegister() bool {
	return len(o.regs) > 0 && o.text == register(o.regs[0])
}

// alloc takes the lowest run of free registers that holds a value of type
// t: one register, or two or four in a row for a D, G, or H value.
func (g *routineGen) alloc(t ir.Type) int {
	w := t.Width()

	for i := 0; i+w <= numRegisters; i++ {
		free := true

		for j := i; j < i+w; j++ {
			free = free && !g.inUse[j]
		}

		if free {
			for j := i; j < i+w; j++ {
				g.inUse[j], g.used[j] = true, true
			}

			return i
		}
	}

	// fit keeps every expression within the budget, which the pool is
	// three times, so this is a mistake of the code generator's.
	panic(fmt.Sprintf("codegen: %s: out of registers", g.r.Name))
}

// free gives back the registers the operands hold.
func (g *routineGen) free(ops ...operand) {
	for _, o := range ops {
		for _, r := range o.regs {
			g.inUse[r] = false
		}
	}
}

// need is how many registers computing e takes, by Sethi and Ullman's
// rule: an operand in memory or a constant needs none, since VAX
// instructions take them in place; an operation needs registers for
// its value (one, or a D, G, or H value's two or four), and if both its
// operands need registers, as many as the harder one, or more when what
// the first leaves held while the second is computed adds to the second's
// (pairNeed). Computing the harder operand first is what makes that the
// least.
func need(e ir.Expr) int {
	switch e := e.(type) {
	case *ir.Load:
		return memNeed(e.Mem)

	case *ir.Addr:
		return max(memNeed(e.Mem), 1)

	case *ir.Op:
		x, y := opOperands(e)
		if y == nil {
			return max(need(x), e.T.Width())
		}

		return pairNeed(x, y, e.T.Width())

	case *ir.Cmp:
		// One more, for the value -1 or 0, while the operands are held.
		return pairNeed(e.X, e.Y, 1) + 1

	case *ir.CharCmp:
		// Its value; its operands are computed when nothing else is held
		// (charCompare).
		return 1
	}

	return 0
}

// pairNeed is what an operation on x and y needs, its value taking w
// registers: the operand needing more is computed first (operands' order),
// and while the other is computed, what the first's value holds (held) is
// in use too. For longwords that is Sethi and Ullman's own rule: as many
// as the harder operand needs, or one more when they need as many each.
func pairNeed(x, y ir.Expr, w int) int {
	a, b := need(x), need(y)

	n := max(a, held(x)+b)
	if b > a {
		n = max(b, held(y)+a)
	}

	return max(n, w)
}

// held is how many registers e's value holds once it is computed: an
// operation's value, its width; a comparison's or an address's, one; a
// place in memory, its index register if it has one; a constant, none.
func held(e ir.Expr) int {
	switch e := e.(type) {
	case *ir.Op:
		return e.T.Width()
	case *ir.Cmp, *ir.CharCmp, *ir.Addr:
		return 1
	case *ir.Load:
		return min(memNeed(e.Mem), 1)
	}

	return 0
}

// memNeed is what reaching a place takes: a register for its index, if
// it has one, which may take more to compute.
func memNeed(m *ir.Mem) int {
	if x := index(m); x != nil {
		return max(need(x), 1)
	}

	return 0
}

// index is the index register's value for the place m, or nil if it
// needs none. A dummy argument's place is @n(AP), the address in the
// argument list, which has no room for an offset: its offset becomes
// part of the index, in elements (@A-4[I], T(I) of T(1:10), is
// @SUB.A(AP)[R0] with I-1 in R0).
func index(m *ir.Mem) ir.Expr {
	if m.Param == nil || m.Offset == 0 {
		return m.Index
	}

	size := m.T.Size()
	if m.Offset%size != 0 {
		panic(fmt.Sprintf("codegen: an offset of %d bytes into a dummy argument of %d-byte elements", m.Offset, size))
	}

	k := int32(m.Offset / size)

	switch x := m.Index.(type) {
	case nil:
		return ir.LongConst(k)
	case *ir.Const:
		return ir.LongConst(x.Int() + k)
	}

	if k < 0 {
		return &ir.Op{Op: ir.Sub, T: ir.Long, X: m.Index, Y: ir.LongConst(-k)}
	}

	return &ir.Op{Op: ir.Add, T: ir.Long, X: m.Index, Y: ir.LongConst(k)}
}

// opOperands are the operands an operation's instruction takes. For
// all but .AND. they are the Op's own. The VAX has no AND instruction:
// it has BIC, bit clear, which clears in its source the bits set in a
// mask, so X .AND. Y is X with the bits of .NOT. Y cleared. The mask is
// a constant's complement, or Z for Y = .NOT. Z, or else .NOT. Y,
// computed with MCOML.
func opOperands(e *ir.Op) (ir.Expr, ir.Expr) {
	if e.Op != ir.And {
		return e.X, e.Y
	}

	x, y := e.X, e.Y
	if _, ok := x.(*ir.Const); ok {
		x, y = y, x
	}

	switch m := y.(type) {
	case *ir.Const:
		return x, ir.IntConst(e.T, ^m.Int())
	case *ir.Op:
		if m.Op == ir.Not {
			return x, m.X
		}
	}

	return x, &ir.Op{Op: ir.Not, T: e.T, X: y}
}

// fit returns e, with any part that would take it past the register
// budget computed first into a spill cell and loaded from there. The
// code that computes those parts is written now, before the statement's
// own, while it holds no registers: expressions have no side effects
// (lowering took the function calls out), so the order doesn't matter.
func (g *routineGen) fit(e ir.Expr) ir.Expr {
	switch e := e.(type) {
	case *ir.Load:
		if e.Mem.Index != nil {
			return &ir.Load{Mem: g.fitMem(e.Mem)}
		}

	case *ir.Addr:
		if e.Mem.Index != nil {
			return &ir.Addr{Mem: g.fitMem(e.Mem)}
		}

	case *ir.Op:
		n := &ir.Op{Op: e.Op, T: e.T, X: g.fit(e.X)}
		if e.Y != nil {
			n.Y = g.fit(e.Y)
		}

		for need(n) > budget {
			if n.Y == nil || need(n.X) >= need(n.Y) {
				n.X = g.spill(n.X)
			} else {
				n.Y = g.spill(n.Y)
			}
		}

		return n

	case *ir.Cmp:
		n := &ir.Cmp{Op: e.Op, X: g.fit(e.X), Y: g.fit(e.Y)}

		for need(n) > budget {
			if need(n.X) >= need(n.Y) {
				n.X = g.spill(n.X)
			} else {
				n.Y = g.spill(n.Y)
			}
		}

		return n
	}

	return e
}

// fitMem is m with its index fitted to the budget.
func (g *routineGen) fitMem(m *ir.Mem) *ir.Mem {
	if m.Index == nil {
		return m
	}

	fitted := *m
	fitted.Index = g.fit(m.Index)

	for memNeed(&fitted) > budget {
		fitted.Index = g.spill(fitted.Index)
	}

	return &fitted
}

// spill computes e into a spill cell, and returns the load of it. A cell
// is the size of the value's type, but at least a longword, so the cells
// after it stay aligned; one the statement isn't using is used again.
func (g *routineGen) spill(e ir.Expr) ir.Expr {
	var c *ir.Cell

	size := max(e.Type().Size(), 4)

	for i, free := range g.spillFree {
		if free.Size == size {
			c = free
			g.spillFree = append(g.spillFree[:i], g.spillFree[i+1:]...)

			break
		}
	}

	if c == nil {
		c = &ir.Cell{
			Name:   "$X" + strconv.Itoa(len(g.spills)+1),
			Kind:   ir.TempCell,
			T:      e.Type(),
			Size:   size,
			Offset: g.localSize(),
		}
		g.spills = append(g.spills, c)
	}

	m := &ir.Mem{Cell: c, T: e.Type()}
	g.store(e, m)

	return &ir.Load{Mem: m}
}

// localSize is the size of the routine's $LOCAL storage, the spill
// cells included.
func (g *routineGen) localSize() int {
	size := g.r.LocalSize

	for _, c := range g.spills {
		size += c.Size
	}

	return size
}

// mem is the operand for the place m, its index computed into a
// register: SQUARES.I, CALLS.M-12[R0], @ISQ.N(AP), SUMUP.$K1.
func (g *routineGen) mem(m *ir.Mem) operand {
	var base string

	switch {
	case m.Param != nil:
		base = "@" + g.names.symbol(m.Param.Name) + "(AP)"
	case m.Cell != nil:
		base = withOffset(g.names.symbol(m.Cell.Name), m.Offset)
	default:
		base = withOffset(g.names.symbol(m.Datum.Name), m.Offset)
	}

	x := index(m)
	if x == nil {
		return operand{text: base}
	}

	r := g.intoRegister(x)

	return operand{text: base + "[" + register(r) + "]", regs: []int{r}}
}

// intoRegister computes e into a register and returns it.
func (g *routineGen) intoRegister(e ir.Expr) int {
	v := g.eval(e)
	if v.isRegister() {
		return v.regs[0]
	}

	g.free(v)
	r := g.alloc(e.Type())
	g.move(e.Type(), v.text, register(r))

	return r
}

// eval writes the code that computes e, and returns the operand that
// holds its value: a constant or a place in memory as it is, an
// operation's value in a register.
func (g *routineGen) eval(e ir.Expr) operand {
	switch e := e.(type) {
	case *ir.Const:
		return operand{text: g.constant(e)}

	case *ir.Load:
		return g.mem(e.Mem)

	case *ir.Addr:
		m := g.mem(e.Mem)
		g.free(m)

		r := g.alloc(ir.Long)
		g.emit(addressOp("MOVA", e.Mem), ir.Long, m.text, register(r))

		return inRegister(r, ir.Long)

	case *ir.Op:
		x, y := g.operands(e)
		g.free(x, y)

		r := g.alloc(e.T)
		g.operate(e, x, y, register(r))

		return inRegister(r, e.T)

	case *ir.Cmp:
		return g.comparison(e)

	case *ir.CharCmp:
		return g.charComparison(e)
	}

	panic(fmt.Sprintf("codegen: an expression of type %T", e))
}

// operands computes an operation's operands, the one needing more
// registers first (need's rule), and returns them, X's first.
func (g *routineGen) operands(e *ir.Op) (operand, operand) {
	x, y := opOperands(e)

	if y == nil {
		return g.eval(x), operand{}
	}

	if need(y) > need(x) {
		vy := g.eval(y)

		return g.eval(x), vy
	}

	vx := g.eval(x)

	return vx, g.eval(y)
}

// operate writes the instruction for e, whose operands are x and y (y
// empty for one operand), its value going to dst.
func (g *routineGen) operate(e *ir.Op, x, y operand, dst string) {
	t := e.T.String()

	switch e.Op {
	case ir.Add:
		g.arithmetic("ADD"+t, e.T, x, y, dst, true)
	case ir.Mul:
		g.arithmetic("MUL"+t, e.T, x, y, dst, true)
	case ir.Or:
		g.arithmetic("BIS"+t, e.T, x, y, dst, true)
	case ir.Xor:
		g.arithmetic("XOR"+t, e.T, x, y, dst, true)
	case ir.Sub:
		g.arithmetic("SUB"+t, e.T, x, y, dst, false)
	case ir.Div:
		g.arithmetic("DIV"+t, e.T, x, y, dst, false)
	case ir.And:
		// BICL3 mask,source,result; y is the mask (opOperands).
		g.arithmetic("BIC"+t, e.T, x, y, dst, false)
	case ir.Neg:
		g.emit("MNEG"+t, e.T, x.text, dst)
	case ir.Not:
		g.emit("MCOM"+t, e.T, x.text, dst)
	case ir.Cvt:
		// CVTLF converts a longword exactly (rounding past 24 bits);
		// CVTFL truncates, as an assignment of a REAL to an INTEGER must
		// (Table 3-1 of the language manual).
		g.emit("CVT"+e.X.Type().String()+t, e.T, x.text, dst)
	}
}

// arithmetic writes a two- or three-operand instruction: OP3 x,y,dst
// with the operands in the VAX's order (SUBL3 sub,min,dif is min - sub,
// so x - y is SUBL3 y,x,dst; DIVL3 likewise; BICL3 mask,src,dst), or
// OP2 when dst is the operand it would change (ADDL2 y,dst for dst =
// dst + y, or for dst = y + dst when the operation is commutative), or
// INCL and DECL for adding or subtracting 1.
func (g *routineGen) arithmetic(op string, t ir.Type, x, y operand, dst string, commutative bool) {
	// The VAX's operand order: for a commutative operation either.
	first, second := y, x
	if commutative {
		first, second = x, y
	}

	same := func(o operand) bool { return o.text == dst && !strings.Contains(dst, "[") }
	increment := t.Integer() && (strings.HasPrefix(op, "ADD") || strings.HasPrefix(op, "SUB"))
	inc := "INC" + t.String()

	if strings.HasPrefix(op, "SUB") {
		inc = "DEC" + t.String()
	}

	switch {
	case same(second):
		if increment && first.text == "#1" {
			g.emit(inc, t, dst)
		} else {
			g.emit(op+"2", t, first.text, dst)
		}

	case commutative && same(first):
		if increment && second.text == "#1" {
			g.emit(inc, t, dst)
		} else {
			g.emit(op+"2", t, second.text, dst)
		}

	default:
		g.emit(op+"3", t, first.text, second.text, dst)
	}
}

// comparison computes a comparison's value, a LOGICAL: -1 if it holds,
// 0 if not.
//
//	CLRL    R2
//	CMPL    X,Y
//	BLEQ    unit.$5           (the comparison's opposite)
//	MNEGL   #1,R2
//	unit.$5:
func (g *routineGen) comparison(e *ir.Cmp) operand {
	c := g.compareOperands(e)
	r := g.alloc(ir.Long)
	g.emit("CLRL", ir.Long, register(r))

	cc := g.compare(c)

	skip := g.newLabel()
	g.emit("B"+opposite[cc], ir.None, skip)
	g.emit("MNEGL", ir.Long, "#1", register(r))
	g.label(skip, "")

	return inRegister(r, ir.Long)
}

// store writes the code for dst = src.
func (g *routineGen) store(src ir.Expr, dst *ir.Mem) {
	src = g.fit(src)
	dst = g.fitMem(dst)
	t := dst.T

	switch s := src.(type) {
	case *ir.Const:
		d := g.mem(dst)

		switch {
		case s.IsZero():
			g.emit("CLR"+t.String(), t, d.text)
		case t == ir.Float:
			// The constant's bits, moved as a longword: MACRO would read
			// #n in MOVF as a floating value to convert. (A Quad, a
			// COMPLEX*8 constant, comes from its datum, below.)
			if g.note == "" {
				g.note = ir.ExprText(s)
			}

			g.emit("MOVL", ir.Long, fmt.Sprintf("#^X%08X", uint32(s.Bits)), d.text)
		case !t.Integer():
			// A D, G, or H constant, or a complex one, from its datum,
			// which holds the compiler's bits (constant).
			if g.note == "" {
				g.note = ir.ExprText(s)
			}

			g.emit("MOV"+t.String(), t, g.constant(s), d.text)
		default:
			g.emit("MOV"+t.String(), t, g.constant(s), d.text)
		}

		g.free(d)

	case *ir.Addr:
		a := g.mem(s.Mem)
		d := g.mem(dst)
		g.emit(addressOp("MOVA", s.Mem), ir.Long, a.text, d.text)
		g.free(a, d)

	case *ir.Op:
		// The operands, then the place, then the operation straight into
		// it: ADDL3 SQUARES.I,#1,SQUARES.J.
		x, y := g.operands(s)
		d := g.mem(dst)
		g.operate(s, x, y, d.text)
		g.free(x, y, d)

	default:
		v := g.eval(src)
		d := g.mem(dst)
		g.move(t, v.text, d.text)
		g.free(v, d)
	}
}

// addressOp is MOVA or PUSHA with the letter for the place m: an index
// register is multiplied by the size the letter says, so an element's address
// takes its own type's letter (MOVAW A[R0],R1 for a word's, MOVAD for a
// D_floating one's, which scales by 8; MOVAQ and MOVAO for a whole
// COMPLEX*8's and COMPLEX*16's). Only a place with an index cares; a
// longword's and an F_floating value's are both L.
func addressOp(op string, m *ir.Mem) string {
	switch m.T {
	case ir.Byte, ir.Word, ir.Double, ir.GFloat, ir.HFloat, ir.Quad, ir.Octa:
		return op + m.T.String()
	}

	return op + "L"
}

// move writes MOVL, MOVF, MOVD, or the like from src to dst (MOVQ or MOVO for
// a complex value as a whole), unless they are the same.
func (g *routineGen) move(t ir.Type, src, dst string) {
	if src == dst {
		g.note = ""

		return
	}

	g.emit("MOV"+t.String(), t, src, dst)
}

// constant is the operand for a constant: #n for an integer (MACRO makes
// it a short literal, S^#n, from 0 to 63, and an immediate, I^#n,
// otherwise); for a floating or complex value, its longwords in $PDATA,
// since MACRO would take #n in a floating instruction for a floating value
// of its own to convert (Decision 16 has the compiler's bits used, never
// MACRO's).
func (g *routineGen) constant(c *ir.Const) string {
	if c.T.Integer() {
		return "#" + strconv.Itoa(int(c.Int()))
	}

	for _, d := range append(append([]*ir.Datum(nil), g.r.Data...), g.extra...) {
		if d.Kind == ir.ConstantDatum && *d.Value == *c {
			return g.names.symbol(d.Name)
		}
	}

	d := &ir.Datum{Name: "$F" + strconv.Itoa(len(g.extra)+1), Kind: ir.ConstantDatum, Value: c}
	g.extra = append(g.extra, d)

	return g.names.symbol(d.Name)
}
