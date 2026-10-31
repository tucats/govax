package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ir"
)

// routineGen is the code generator's state for one routine.
type routineGen struct {
	r     *ir.Routine
	names *routineNames

	// bases names the common blocks' first bytes.
	bases *namer
	src   *sourceLines

	// pre is what comes before the routine's .ENTRY in $CODE: the source
	// lines up to its header statement. code is everything after it.
	// out is the one being written.
	pre, code Writer
	out       *Writer

	// labels are the blocks' labels, made when first needed; targets
	// are the blocks some branch goes to, which need one.
	labels  map[*ir.Block]string
	targets map[*ir.Block]bool

	// compiler counts the compiler's own labels ($1, $2, ...).
	compiler int

	// inUse are the registers holding values now, and used those the
	// routine has used at all, for its entry mask.
	inUse, used [numRegisters]bool

	// spills are the cells holding parts of expressions too complex for
	// the registers ($X1, ...), and spillFree those not in use.
	spills    []*ir.Cell
	spillFree []*ir.Cell

	// extra are the read-only constants the code uses as operands that
	// lowering didn't make ($F1, ...): REAL constants, which can't be
	// immediate operands.
	extra []*ir.Datum

	// note is a comment for the next instruction written.
	note string

	// set is what the last instruction wrote and whose condition codes
	// it left: a TST of it right after is not needed. Empty when the
	// condition codes say nothing known, as after a label, which another
	// branch may reach.
	set     string
	setType ir.Type
}

// newRoutineGen starts generating r.
func newRoutineGen(r *ir.Routine, names *namer, src *sourceLines) *routineGen {
	return &routineGen{
		r:       r,
		names:   names.routine(r.Name),
		bases:   names,
		src:     src,
		labels:  map[*ir.Block]string{},
		targets: map[*ir.Block]bool{},
	}
}

// generate writes the routine's code into pre and code.
func (g *routineGen) generate() {
	g.out = &g.pre
	g.src.through(g.out, g.r.Unit.AST.Header)
	g.out = &g.code

	g.findTargets()

	for i, b := range g.r.Blocks {
		var next *ir.Block
		if i+1 < len(g.r.Blocks) {
			next = g.r.Blocks[i+1]
		}

		g.block(b, next)
	}

	// The rest of the unit's lines, its END among them when nothing came
	// of it (after a RETURN).
	if body := g.r.Unit.AST.Body; len(body) > 0 {
		g.src.through(g.out, body[len(body)-1])
	}
}

// findTargets notes the blocks a jump or branch goes to: all but those
// reached only by falling into them from the block before. A block's
// terminator names its next block only to say that control goes on
// there, which takes no instruction.
func (g *routineGen) findTargets() {
	for _, e := range g.r.Entries {
		if e.First != nil {
			g.targets[e.First] = true
		}
	}

	for i, b := range g.r.Blocks {
		var next *ir.Block
		if i+1 < len(g.r.Blocks) {
			next = g.r.Blocks[i+1]
		}

		switch t := b.End.(type) {
		case *ir.Jump:
			if t.To != next {
				g.targets[t.To] = true
			}

		case *ir.Branch:
			for _, to := range []*ir.Block{t.True, t.False} {
				if to != next {
					g.targets[to] = true
				}
			}

		case *ir.Switch:
			// The table names every target, even the next block.
			for _, to := range t.Targets {
				g.targets[to] = true
			}

			if t.Else != next {
				g.targets[t.Else] = true
			}

		case *ir.Sign:
			for _, to := range []*ir.Block{t.Neg, t.Zero, t.Pos} {
				if to != next {
					g.targets[to] = true
				}
			}
		}
	}
}

// block writes one block's code. A block with a FORTRAN label has the
// label's statement's lines before it, and its label always (it names
// the statement, for someone reading the file or debugging); a block
// other blocks branch to gets a compiler label.
func (g *routineGen) block(b *ir.Block, next *ir.Block) {
	for _, e := range g.r.Entries {
		if e.First == b {
			g.src.through(g.out, e.Source)
		}
	}

	switch {
	case b.Label != 0:
		if l := g.r.Unit.Labels[b.Label]; l != nil {
			g.src.through(g.out, l.Stmt)
		}

		g.label(g.blockLabel(b), b.Note)

	case g.targets[b]:
		g.label(g.blockLabel(b), b.Note)
	}

	for _, s := range b.Stmts {
		g.src.through(g.out, s.Source())

		switch s := s.(type) {
		case *ir.Assign:
			g.assign(s)
		case *ir.CharMove:
			g.charMove(s)
		case *ir.Call:
			g.call(s)
		}

		g.endStatement()
	}

	g.src.through(g.out, b.End.Source())
	g.terminator(b.End, next)
	g.endStatement()
}

// blockLabel is the label of block b: unit.Ln for the block that starts
// at FORTRAN label n, the compiler's unit.$n for another.
func (g *routineGen) blockLabel(b *ir.Block) string {
	if l, ok := g.labels[b]; ok {
		return l
	}

	var l string

	if b.Label != 0 {
		l = g.names.symbol("L" + strconv.Itoa(b.Label))
	} else {
		l = g.newLabel()
	}

	g.labels[b] = l

	return l
}

// newLabel is a new compiler label: unit.$1, unit.$2, ...
func (g *routineGen) newLabel() string {
	g.compiler++

	return g.names.symbol("$" + strconv.Itoa(g.compiler))
}

// assign writes Dst = Src, with a comment for the compiler's own cells.
func (g *routineGen) assign(s *ir.Assign) {
	if c := s.Dst.Cell; c != nil && s.Dst.Index == nil {
		switch c.Kind {
		case ir.CountCell:
			g.note = c.Loop + "'s count"

			if op, ok := s.Src.(*ir.Op); ok && op.Op == ir.Sub {
				if ld, ok := op.X.(*ir.Load); ok && ld.Mem.Cell == c {
					g.note = "one pass fewer"
				}
			}

		case ir.StepCell:
			g.note = c.Loop + "'s step"
		}
	}

	g.store(s.Src, s.Dst)
}

// call writes a CALLS: each argument pushed, from the last to the first
// (the calling standard's order, so the first is at 4(AP)), then the
// call, then a function's value stored from R0. A value wider than a
// longword is pushed whole, MOVQ or MOVO to -(SP), which leaves its
// longwords in memory's order, the first at the lowest address; the count
// CALLS gives is in longwords.
func (g *routineGen) call(s *ir.Call) {
	count := 0

	for i := len(s.Args) - 1; i >= 0; i-- {
		a := s.Args[i]

		// A routine's address, which LINK fixes up for a routine in a
		// shareable image as it does a CALLS's.
		if a.Routine != "" {
			g.emit("PUSHAB", ir.None, "G^"+a.Routine)

			count++

			continue
		}

		if a.Ref != nil {
			m := g.mem(g.fitMem(a.Ref))
			g.emit(pushAddress(a.Ref), ir.None, m.text)
			g.free(m)

			count++

			continue
		}

		v := g.eval(g.fit(a.Value))

		switch t := a.Value.Type(); t.Size() {
		case 8:
			g.emit("MOVQ", ir.None, v.text, "-(SP)")
		case 16:
			g.emit("MOVO", ir.None, v.text, "-(SP)")
		default:
			g.emit("PUSHL", ir.None, v.text)
		}

		g.free(v)

		count += a.Value.Type().Width()
	}

	g.emit("CALLS", ir.None, "#"+strconv.Itoa(count), "G^"+s.Routine)

	if s.Result != nil {
		// The value is in R0 (R0 and R1 for a D, G, or COMPLEX*8 value, R0
		// to R3 for an OTS$..._R3 routine's), which nothing may use until it
		// is stored. A routine that returns a value in R2 and R3 doesn't
		// keep them for its caller, so this routine's entry mask must save
		// them for its own.
		w := s.Result.T.Width()
		for i := range w {
			g.inUse[i], g.used[i] = true, true
		}

		d := g.mem(g.fitMem(s.Result))

		for i := range w {
			g.inUse[i] = false
		}

		g.move(s.Result.T, "R0", d.text)
		g.free(d)
	}
}

// pushAddress is the instruction that pushes the address of m: PUSHAB
// for an I/O control block, PUSHAQ for a descriptor (a quadword, at
// least), and for a value or an array of them the letter of its type. The
// letter only scales an index (addressOp).
func pushAddress(m *ir.Mem) string {
	switch {
	case m.Datum != nil && m.Datum.Kind == ir.ControlDatum:
		return "PUSHAB"
	case m.Datum != nil && m.Datum.Kind == ir.DescriptorDatum,
		m.Cell != nil && m.Cell.Kind == ir.DescriptorCell:
		return "PUSHAQ"
	}

	return addressOp("PUSHA", m)
}

// terminator writes a block's terminator, next being the block laid out
// after it, which control reaches without an instruction.
func (g *routineGen) terminator(t ir.Terminator, next *ir.Block) {
	switch t := t.(type) {
	case *ir.Jump:
		if t.To != next {
			g.emit("BRW", ir.None, g.blockLabel(t.To))
		}

	case *ir.Branch:
		switch {
		case t.True == t.False:
			if t.True != next {
				g.emit("BRW", ir.None, g.blockLabel(t.True))
			}
		case t.False == next:
			g.branchTo(t.Cond, true, g.blockLabel(t.True))
		case t.True == next:
			g.branchTo(t.Cond, false, g.blockLabel(t.False))
		default:
			g.branchTo(t.Cond, true, g.blockLabel(t.True))
			g.emit("BRW", ir.None, g.blockLabel(t.False))
		}

	case *ir.Return:
		if t.Value != nil {
			switch g.r.Kind {
			case ir.Main:
				g.note = "SS$_NORMAL"
			case ir.Subroutine:
				g.note = "the alternate return's number"
			default:
				g.note = "the function's value"
			}

			if c, ok := t.Value.(*ir.Const); ok && c.T == ir.Long && c.Bits == 0 {
				g.emit("CLRL", ir.Long, "R0")
			} else {
				v := g.eval(g.fit(t.Value))
				g.move(t.Value.Type(), v.text, "R0")
				g.free(v)
			}
		}

		g.emit("RET", ir.None)

	case *ir.Switch:
		g.caseTable(t, next)

	case *ir.Sign:
		g.sign(t, next)

	case *ir.IndirectJump:
		g.indirectJump(t)

	case *ir.NoReturn:
		// Nothing: the call before it doesn't return.
	}
}

// caseTable writes a computed GO TO: CASEL, which takes its index less
// the base (1) and, if that is no more than the limit (one less than the
// number of targets), adds the table's word at that position to the
// table's own address and goes there; otherwise it goes on past the
// table. Each word is a target's distance from the table, which MACRO
// computes, and which reaches as far as BRW does.
//
//	CASEL   SUB.K,#1,#2
//	SUB.$3:                       (the table)
//	.WORD   SUB.L10-SUB.$3
//	.WORD   SUB.L20-SUB.$3
//	.WORD   SUB.L30-SUB.$3
//	(out of range: on to Else)
func (g *routineGen) caseTable(t *ir.Switch, next *ir.Block) {
	index := g.eval(g.fit(t.Index))
	g.free(index)

	if _, ok := t.Index.(*ir.CallValue); ok {
		g.note = "the alternate return's number"
	}

	g.emit("CASEL", ir.None, index.text, "#1", "#"+strconv.Itoa(len(t.Targets)-1))

	table := g.newLabel()
	g.label(table, "")

	for _, to := range t.Targets {
		g.emit(".WORD", ir.None, g.blockLabel(to)+"-"+table)
	}

	if t.Else != next {
		g.emit("BRW", ir.None, g.blockLabel(t.Else))
	}
}

// sign writes an arithmetic IF: a TST of the value (or none, if the
// instruction that computed it set the condition codes), then a branch for
// each of the three cases but one, which the code falls into or reaches by
// BRW. Cases that go to the same place are one branch: IF (X) 10,10,20
// is BLEQ to 10 (around a BRW, as every branch is) and BRW to 20.
func (g *routineGen) sign(t *ir.Sign, next *ir.Block) {
	if t.Neg == t.Zero && t.Zero == t.Pos {
		if t.Neg != next {
			g.emit("BRW", ir.None, g.blockLabel(t.Neg))
		}

		return
	}

	// The cases, by the condition that goes to each: two when two
	// targets are the same, else three.
	type branch struct {
		op ir.Operator
		to *ir.Block
	}

	var cases []branch

	switch {
	case t.Neg == t.Zero:
		cases = []branch{{ir.Le, t.Neg}, {ir.Gt, t.Pos}}
	case t.Zero == t.Pos:
		cases = []branch{{ir.Lt, t.Neg}, {ir.Ge, t.Zero}}
	case t.Neg == t.Pos:
		cases = []branch{{ir.Eq, t.Zero}, {ir.Ne, t.Neg}}
	default:
		cases = []branch{{ir.Lt, t.Neg}, {ir.Eq, t.Zero}, {ir.Gt, t.Pos}}
	}

	// The case whose target comes next needs no instruction; without one,
	// the last case is a BRW alone, the others having been tested.
	last := len(cases) - 1

	for i, c := range cases {
		if c.to == next {
			cases[i], cases[last] = cases[last], cases[i]

			break
		}
	}

	v := g.fit(t.Value)
	g.compare(comparing{op: ir.Eq, t: v.Type(), x: g.eval(v), test: true})

	for _, c := range cases[:last] {
		g.around("B"+opposite[conditions[c.op]], nil, g.blockLabel(c.to))
	}

	if cases[last].to != next {
		g.emit("BRW", ir.None, g.blockLabel(cases[last].to))
	}
}

// indirectJump writes an assigned GO TO: JMP to the address the variable
// holds, through it by deferred mode (JMP @SUB.K). A variable that is
// itself reached by deferred or indexed mode (a dummy argument, @n(AP))
// can't be deferred again, so its value goes to a register first.
func (g *routineGen) indirectJump(t *ir.IndirectJump) {
	v := g.eval(g.fit(&ir.Load{Mem: t.Through}))

	if v.isRegister() || strings.ContainsAny(v.text, "@[") {
		g.free(v)

		r := g.alloc(ir.Long)
		g.move(ir.Long, v.text, register(r))
		g.emit("JMP", ir.None, "("+register(r)+")")
		g.free(inRegister(r, ir.Long))

		return
	}

	g.emit("JMP", ir.None, "@"+v.text)
	g.free(v)
}

// endStatement ends an IR statement's code: its registers and spill
// cells are free for the next.
func (g *routineGen) endStatement() {
	for i, busy := range g.inUse {
		if busy {
			panic(fmt.Sprintf("codegen: %s: R%d still in use after a statement", g.r.Name, i))
		}
	}

	g.spillFree = append(g.spillFree[:0], g.spills...)
}

// entryMask is the routine's register save mask: the registers from R2
// up that its code uses, which the calling standard says a routine must
// preserve (R0 and R1 are scratch). Integer overflow isn't enabled (the
// mask's IV bit), as VAX FORTRAN's /CHECK=NOOVERFLOW leaves it.
func (g *routineGen) entryMask() string {
	var regs []string

	for i := 2; i < numRegisters; i++ {
		if g.used[i] {
			regs = append(regs, register(i))
		}
	}

	return "^M<" + strings.Join(regs, ",") + ">"
}

// emit writes an instruction: its opcode and operands, with the comment
// noted for it, if any. wrote is the type of the value its last operand
// receives, ir.None if it writes none; its condition codes then describe
// that value, which lets a test of it right after be left out.
func (g *routineGen) emit(opcode string, wrote ir.Type, operands ...string) {
	g.out.Statement("", opcode, strings.Join(operands, ","), g.note)
	g.note = ""

	g.set = ""
	if wrote != ir.None && len(operands) > 0 {
		g.set, g.setType = operands[len(operands)-1], wrote
	}
}

// label writes a label, with a comment. Since another branch may come
// to it, nothing is known of the condition codes after it.
func (g *routineGen) label(name, comment string) {
	g.out.LabelComment(name, comment)
	g.set = ""
}
