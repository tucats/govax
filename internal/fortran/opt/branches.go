package opt

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/codegen"
)

// Branches makes a routine's branches short, and closes its loops with the
// VAX's loop instructions.
//
// The code generator doesn't know how far away anything is, and MACRO won't
// lengthen a branch that turns out too short. So every conditional branch it
// writes reaches 32K bytes, as a byte branch of the opposite condition around a
// BRW. This loop,
//
//	   DO 10 I = 1, N
//	      S = S + A(I)
//	10 CONTINUE
//
// closes with four instructions, one of them taken only to jump over another:
//
//	T.L10:
//	        INCL    T.I
//	        DECL    T.$C1
//	        BLEQ    T.$3
//	        BRW     T.$2
//	T.$3:
//
// Here the distance is known, or at least its upper limit is: adding up what
// each instruction between can assemble to at most (codegen's Bound) says the
// loop's start is within a byte branch. So the branch needn't go around,
// and the BRW goes:
//
//	T.L10:
//	        INCL    T.I
//	        DECL    T.$C1
//	        BGTR    T.$2
//
// And a DECL and a branch if it's still positive is what SOBGTR is, one
// instruction for the two:
//
//	T.L10:
//	        INCL    T.I
//	        SOBGTR  T.$C1,T.$2
//
// Each rule here is one of those small steps, tried on each instruction in turn.
// One step makes room for the next: taking out a BRW may bring another branch in
// range. So the steps go round until a round changes nothing. Every step makes
// the code shorter, never longer, so what a round has found to be in range stays
// in range, and the rounds end.
//
// The rules:
//
//   - A branch to a BRW goes straight to where the BRW goes, if it reaches, and a
//     BRW to a RET is a RET.
//   - A branch to what comes next goes.
//   - A branch around a BRW is the opposite branch, if it reaches.
//   - DECL X and BGTR or BGEQ is SOBGTR or SOBGEQ.
//   - INCL I, CMPL I,LIMIT, and BLEQ or BLSS is AOBLEQ or AOBLSS.
//   - An addition or subtraction of a constant, a CMPL, and a branch by the
//     constant's sign is ACBL, whose displacement is a word: it needs no BRW even
//     where the others don't reach.
//   - A BRW that reaches is a BRB.
//   - A label of the compiler's that nothing goes to goes, which may join two
//     instructions a rule looks for. Code after a jump, with no label anything
//     goes to, can't run, and goes too.
//
// # What AOBLEQ and ACBL change
//
// SOBGTR is exactly DECL and a branch. The other two are not exactly what they
// replace, in three ways, each of which a rule must allow for:
//
//   - They read the limit before they change the index, where the CMPL read it
//     after. If they overlap (by EQUIVALENCE, say), that's a different value. So
//     the limit is a constant, a register, or a variable that doesn't overlap the
//     index. Two dummy arguments are taken not to, the standard's rule (the
//     phase doc's first question).
//   - They read the index once, not twice, and a VOLATILE variable's every
//     reference counts. So neither may be VOLATILE.
//   - They set N and Z from the index, not from the comparison. So nothing may
//     test the condition codes after them, as a second branch could.
func Branches(c *codegen.Code) {
	for round(c) {
	}
}

// layout is where everything in a routine's code is, at most, as a round begins.
type layout struct {
	c *codegen.Code

	// items are the code's items, nil for one a rule has taken out.
	items []*codegen.Item

	// start and end are each item's offsets, by Bound.
	start, end []int

	// labels are the labels' places in items.
	labels map[string]int

	changed bool
}

// round tries every rule on every instruction once, and reports whether anything
// changed. The offsets are as the round began, and since every change only takes
// bytes out, a distance found from them is never too small.
func round(c *codegen.Code) bool {
	v := &layout{
		c:      c,
		items:  c.Items,
		start:  make([]int, len(c.Items)),
		end:    make([]int, len(c.Items)),
		labels: map[string]int{},
	}

	at := 0

	for i, it := range v.items {
		v.start[i] = at
		at += c.Bound(it)
		v.end[i] = at

		if it.Kind == codegen.Label {
			v.labels[it.Name] = i
		}
	}

	for i, it := range v.items {
		if it != nil && it.Target() != "" {
			v.improve(i)
		}
	}

	v.sweep()

	return v.changed
}

// reaches reports whether the branch it, put in place of items first to last,
// reaches label. Going forward it's as far as from last's end, since whatever is
// taken out moves the label back as far as the branch's end. Going back it's from
// the end it will have.
func (v *layout) reaches(it *codegen.Item, first, last int, label string) bool {
	i, ok := v.labels[label]
	if !ok {
		return false
	}

	end := v.end[last]
	if v.start[i] < end {
		end = v.start[first] + v.c.Bound(it)
	}

	return it.Fits(v.start[i] - end)
}

// remove takes out item i.
func (v *layout) remove(i int) {
	v.items[i] = nil
	v.changed = true
}

// before is the instruction just before item i, if no label comes between them
// (a label would let something else reach item i without running it); -1 if none.
func (v *layout) before(i int) int {
	for i--; i >= 0; i-- {
		if it := v.items[i]; it != nil {
			switch it.Kind {
			case codegen.Instruction:
				return i
			case codegen.Label:
				return -1
			}
		}
	}

	return -1
}

// after is the instruction just after item i, if no label comes between; -1 if
// none.
func (v *layout) after(i int) int {
	for i++; i < len(v.items); i++ {
		if it := v.items[i]; it != nil {
			switch it.Kind {
			case codegen.Instruction:
				return i
			case codegen.Label:
				return -1
			}
		}
	}

	return -1
}

// runs is the first instruction at or after item i, labels or none: the one that
// runs when control comes to item i. -1 if there are no more.
func (v *layout) runs(i int) int {
	for ; i < len(v.items); i++ {
		if it := v.items[i]; it != nil && it.Kind == codegen.Instruction {
			return i
		}
	}

	return -1
}

// follows reports whether label is among those between item i and the next
// instruction: where control goes on to after item i anyway.
func (v *layout) follows(i int, label string) bool {
	for i++; i < len(v.items); i++ {
		if it := v.items[i]; it != nil {
			if it.Kind == codegen.Instruction {
				return false
			}

			if it.Kind == codegen.Label && it.Name == label {
				return true
			}
		}
	}

	return false
}

// isJump reports whether opcode always goes to its label.
func isJump(opcode string) bool {
	return opcode == "BRW" || opcode == "BRB"
}

// inverted is the branch on the opposite condition.
var inverted = map[string]string{
	"BEQL": "BNEQ", "BNEQ": "BEQL", "BLSS": "BGEQ", "BGEQ": "BLSS", "BLEQ": "BGTR", "BGTR": "BLEQ",
	"BLSSU": "BGEQU", "BGEQU": "BLSSU", "BLEQU": "BGTRU", "BGTRU": "BLEQU",
	"BVS": "BVC", "BVC": "BVS", "BCS": "BCC", "BCC": "BCS", "BLBC": "BLBS", "BLBS": "BLBC",
}

// improve tries the rules on the branch at item i.
func (v *layout) improve(i int) {
	it := v.items[i]

	v.thread(i)

	target := it.Target()

	if isJump(it.Opcode) {
		if k := v.runs(v.labels[target]); k >= 0 && v.items[k].Opcode == "RET" {
			it.Opcode, it.Operands = "RET", nil
			v.changed = true

			return
		}
	}

	// A branch to what comes next, but not BLBC's or BLBS's: those read an
	// operand, which may be VOLATILE.
	if (isJump(it.Opcode) || codegen.IsConditional(it.Opcode)) && v.follows(i, target) {
		v.remove(i)

		return
	}

	if _, branches := inverted[it.Opcode]; branches {
		if n := v.after(i); n >= 0 && isJump(v.items[n].Opcode) && v.follows(n, target) {
			v.around(i, n)

			return
		}
	}

	if codegen.IsConditional(it.Opcode) {
		v.closeLoop(i, i, it.Opcode, target)

		return
	}

	if it.Opcode == "BRW" {
		short := *it
		short.Opcode = "BRB"

		if v.reaches(&short, i, i, target) {
			it.Opcode = "BRB"
			v.changed = true
		}
	}
}

// thread sends the branch at item i to where a jump at its target goes, as far
// along such jumps as it reaches.
func (v *layout) thread(i int) {
	it := v.items[i]
	target := it.Target()
	seen := map[string]bool{target: true}

	for {
		at, ok := v.labels[target]
		if !ok {
			return
		}

		k := v.runs(at)
		if k < 0 || k == i || !isJump(v.items[k].Opcode) {
			return
		}

		target = v.items[k].Target()
		if seen[target] {
			return
		}

		seen[target] = true

		if v.reaches(it, i, i, target) {
			it.Operands[len(it.Operands)-1] = target
			v.changed = true
		}
	}
}

// around improves the branch at item i over the jump at item n, which the branch
// goes around: the opposite branch to the jump's target if it reaches, and
// otherwise, perhaps, an ACBL.
func (v *layout) around(i, n int) {
	it, jump := v.items[i], v.items[n]
	target := jump.Target()

	opposite := *it
	opposite.Opcode = inverted[it.Opcode]

	if v.reaches(&opposite, i, n, target) {
		it.Opcode = opposite.Opcode
		it.Operands[len(it.Operands)-1] = target
		v.remove(n)

		return
	}

	if codegen.IsConditional(opposite.Opcode) {
		v.closeLoop(i, n, opposite.Opcode, target)
	}
}

// closeLoop looks before item i, a conditional branch, for what makes it a loop
// instruction: it says that if the condition cc holds, control goes to target,
// from an instruction whose code ends at item last (the branch itself, or the jump
// it goes around). A rule's result is at item last, where control leaves.
func (v *layout) closeLoop(i, last int, cc, target string) {
	p := v.before(i)
	if p < 0 {
		return
	}

	prev := v.items[p]

	if prev.Opcode == "DECL" {
		v.subtractOneAndBranch(i, last, p, cc, target)

		return
	}

	if prev.Opcode != "CMPL" {
		return
	}

	index, limit := prev.Operands[0], prev.Operands[1]

	s := v.before(p)
	if s < 0 {
		return
	}

	k, ok := stepOf(v.items[s], index)
	if !ok {
		// CMPL LIMIT,I says the same with the condition turned round.
		index, limit = limit, index
		cc = swappedBranch[cc]

		if k, ok = stepOf(v.items[s], index); !ok || cc == "" {
			return
		}
	}

	if !v.safeLoop(index, limit, last, target) {
		return
	}

	loop := &codegen.Item{Kind: codegen.Instruction, Comment: v.items[s].Comment}

	switch {
	case k == 1 && cc == "BLEQ" && last == i:
		loop.Opcode, loop.Operands = "AOBLEQ", []string{limit, index, target}
	case k == 1 && cc == "BLSS" && last == i:
		loop.Opcode, loop.Operands = "AOBLSS", []string{limit, index, target}
	case k > 0 && cc == "BLEQ", k < 0 && cc == "BGEQ":
		loop.Opcode, loop.Operands = "ACBL", []string{limit, "#" + strconv.Itoa(k), index, target}
	default:
		return
	}

	// None of the rules may lengthen the code (Branches' comment), and a #-1 is
	// a longword, which may be more than the instructions it replaces.
	if v.c.Bound(loop) > v.end[last]-v.start[s] || !v.reaches(loop, s, last, target) {
		return
	}

	v.replace(loop, last, s, p, i)
}

// replace puts it at item at, taking out the other items given.
func (v *layout) replace(it *codegen.Item, at int, gone ...int) {
	for _, i := range gone {
		if i != at {
			v.remove(i)
		}
	}

	v.items[at] = it
	v.changed = true
}

// subtractOneAndBranch makes SOBGTR or SOBGEQ of the DECL at item p and the branch
// at item i. N and Z are set just as DECL sets them, so a branch after it reads
// the same; only the C bit, which no branch after a DECL reads, isn't.
func (v *layout) subtractOneAndBranch(i, last, p int, cc, target string) {
	var op string

	switch cc {
	case "BGTR":
		op = "SOBGTR"
	case "BGEQ":
		op = "SOBGEQ"
	default:
		return
	}

	loop := &codegen.Item{
		Kind: codegen.Instruction, Opcode: op,
		Operands: []string{v.items[p].Operands[0], target},
		Comment:  v.items[p].Comment,
	}

	if v.reaches(loop, p, last, target) {
		v.replace(loop, last, p, i)
	}
}

// swappedBranch is the branch that goes on Y and X as this one does on X and Y.
var swappedBranch = map[string]string{
	"BLEQ": "BGEQ", "BGEQ": "BLEQ", "BLSS": "BGTR", "BGTR": "BLSS", "BEQL": "BEQL", "BNEQ": "BNEQ",
}

// stepOf reports what the instruction adds to index, if it adds a constant to it:
// INCL, DECL, ADDL2 #k, SUBL2 #k.
func stepOf(it *codegen.Item, index string) (int, bool) {
	switch it.Opcode {
	case "INCL":
		return 1, it.Operands[0] == index
	case "DECL":
		return -1, it.Operands[0] == index
	case "ADDL2", "SUBL2":
		if it.Operands[1] != index || !strings.HasPrefix(it.Operands[0], "#") {
			return 0, false
		}

		k, err := strconv.Atoi(it.Operands[0][1:])
		if err != nil {
			return 0, false
		}

		if it.Opcode == "SUBL2" {
			k = -k
		}

		return k, k != 0
	}

	return 0, false
}

// safeLoop reports whether a loop instruction may do the work of adding to index,
// comparing it with limit, and branching, the code ending at item last (Branches'
// comment says why each of these).
func (v *layout) safeLoop(index, limit string, last int, target string) bool {
	if index == limit || isImmediate(index) || !v.plainVariable(index) {
		return false
	}

	if !isImmediate(limit) && !v.plainVariable(limit) {
		return false
	}

	if v.overlap(index, limit) {
		return false
	}

	// What runs next, either way, doesn't look at the condition codes.
	if n := v.runs(last + 1); n >= 0 && codegen.IsConditional(v.items[n].Opcode) {
		return false
	}

	if at, ok := v.labels[target]; ok {
		if n := v.runs(at); n >= 0 && codegen.IsConditional(v.items[n].Opcode) {
			return false
		}
	}

	return true
}

// isImmediate reports whether an operand is a constant.
func isImmediate(operand string) bool {
	return strings.HasPrefix(operand, "#")
}

// plainVariable reports whether an operand is a register or the whole of a
// variable this routine knows, and not a VOLATILE one.
func (v *layout) plainVariable(operand string) bool {
	if isRegisterOperand(operand) {
		return true
	}

	if c, ok := v.c.Cells[operand]; ok {
		return c.Sym == nil || !c.Sym.Volatile
	}

	if p, ok := v.c.Params[operand]; ok {
		return p.Sym == nil || !p.Sym.Volatile
	}

	return false
}

// isRegisterOperand reports whether an operand is one of R0 to R11.
func isRegisterOperand(operand string) bool {
	if len(operand) < 2 || operand[0] != 'R' {
		return false
	}

	_, err := strconv.Atoi(operand[1:])

	return err == nil
}

// overlap reports whether two variables may share memory: two of the routine's
// cells do if their bytes overlap in the same section. A dummy argument is taken
// to overlap nothing that's changed, the standard's rule.
func (v *layout) overlap(a, b string) bool {
	x, okX := v.c.Cells[a]
	y, okY := v.c.Cells[b]

	if !okX || !okY || x.Common != y.Common {
		return false
	}

	return x.Offset < y.Offset+max(y.Size, 1) && y.Offset < x.Offset+max(x.Size, 1)
}

// sweep ends a round: labels nothing goes to and code nothing reaches go, and the
// items taken out leave the list.
func (v *layout) sweep() {
	refs := map[string]int{}

	for _, it := range v.items {
		if it != nil && it.Kind == codegen.Instruction {
			for _, o := range it.Operands {
				for _, name := range symbolsIn(o) {
					refs[name]++
				}
			}
		}
	}

	live := true

	for i, it := range v.items {
		if it == nil {
			continue
		}

		switch it.Kind {
		case codegen.Label:
			used := refs[it.Name] > 0 || v.c.Outside[it.Name]
			live = live || used

			if !used && !it.Named {
				v.remove(i)
			}

		case codegen.Instruction:
			if !live {
				v.remove(i)

				continue
			}

			switch it.Opcode {
			case "BRW", "BRB", "RET", "JMP":
				live = false
			}
		}
	}

	kept := v.items[:0]

	for _, it := range v.items {
		if it != nil {
			kept = append(kept, it)
		}
	}

	v.c.Items = kept
}

// symbolsIn are the names in an operand that could be labels: an operand of a
// branch or of MOVAB is one, and a .WORD of a table is two, one less the other.
func symbolsIn(operand string) []string {
	return strings.FieldsFunc(operand, func(r rune) bool {
		switch r {
		case '+', '-', '(', ')', '[', ']', '@', '#', '^':
			return true
		}

		return false
	})
}
