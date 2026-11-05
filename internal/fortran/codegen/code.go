package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ir"
)

// ItemKind is what an Item of a routine's code is.
type ItemKind int

// The kinds of item.
const (
	Instruction ItemKind = iota // an instruction, or a .WORD of a CASEL's table
	Label                       // a label, on a line of its own
	Text                        // lines of the FORTRAN source, as comments, which make no code
)

// Item is one thing in a routine's code, in the order the file has it.
type Item struct {
	Kind ItemKind

	// Opcode and Operands are an instruction's, as MACRO writes them.
	Opcode   string
	Operands []string

	// Comment is an instruction's or a label's comment, "" for none.
	Comment string

	// Name is a label's name, and Named says it is a FORTRAN statement's. Such
	// a label stays, whether or not anything still goes to it, since it names
	// the statement for someone reading the file or debugging.
	Name  string
	Named bool

	// Lines are a Text item's lines, ready to write.
	Lines []string
}

// Code is a routine's code as a list, between the code generator's choosing
// instructions and its writing them: what an optimizer's instruction pass
// works on (Decision 4 of docs/PHASE-84 - FORTRAN optimizer.md). Written with
// nothing changed, it is exactly the text the generator chose.
//
// Only a routine's body is here. Its entry points but the first, and the start
// of each FORMAT's routine, are written after it (writeEntry, writeVFE).
type Code struct {
	Items []*Item

	// Outside are the labels the code written outside the list goes to: an
	// ENTRY statement's first block, a FORMAT's expressions.
	Outside map[string]bool

	// Displacements are the symbols that stand for a displacement from AP or FP,
	// with their values: a dummy argument's place in the argument list, an
	// AUTOMATIC variable's in the frame. MACRO knows their values when it comes
	// to the code, so it makes them as short as they can be.
	Displacements map[string]int

	// Cells and Params say what an operand is when it is the whole of a
	// variable: Cells by its symbol (SUB.I), and Params by the operand that
	// reaches a dummy argument (@SUB.A(AP)). A pass that changes how often or in
	// what order a variable is read asks these first, for what may overlap it
	// and whether it is VOLATILE.
	Cells  map[string]*ir.Cell
	Params map[string]*ir.Param
}

// instruction adds an instruction.
func (c *Code) instruction(opcode, comment string, operands []string) {
	c.Items = append(c.Items, &Item{Kind: Instruction, Opcode: opcode, Operands: operands, Comment: comment})
}

// label adds a label.
func (c *Code) label(name, comment string, named bool) {
	c.Items = append(c.Items, &Item{Kind: Label, Name: name, Comment: comment, Named: named})
}

// text adds lines of the source.
func (c *Code) text(lines []string) {
	if len(lines) > 0 {
		c.Items = append(c.Items, &Item{Kind: Text, Lines: lines})
	}
}

// Write writes the list to w.
func (c *Code) Write(w *Writer) {
	for _, it := range c.Items {
		switch it.Kind {
		case Instruction:
			w.Statement("", it.Opcode, strings.Join(it.Operands, ","), it.Comment)
		case Label:
			w.LabelComment(it.Name, it.Comment)
		case Text:
			w.lines = append(w.lines, it.Lines...)
		}
	}
}

// facts fills in what the list says about the routine's symbols, for an
// instruction pass.
func (g *routineGen) facts() {
	c := &g.code
	c.Outside = map[string]bool{}
	c.Displacements = map[string]int{}
	c.Cells = map[string]*ir.Cell{}
	c.Params = map[string]*ir.Param{}

	for _, e := range g.r.Entries {
		if e.First != nil {
			c.Outside[g.blockLabel(e.First)] = true
		}
	}

	for _, v := range g.r.VFEs {
		for _, b := range v.Cases {
			c.Outside[g.blockLabel(b)] = true
		}
	}

	for _, p := range g.r.Params {
		c.Params["@"+g.argumentPlace(p)] = p

		if p.Through == nil {
			c.Displacements[g.names.symbol(p.Name)] = 4 * p.Index
		}
	}

	for _, cell := range g.r.Cells {
		if cell.Register != 0 {
			continue
		}

		if cell.Frame {
			c.Displacements[g.names.symbol(cell.Name)] = cell.Offset - g.frameSize()
		} else {
			c.Cells[g.names.symbol(cell.Name)] = cell
		}
	}

	// A spill cell in the frame is a displacement too; one in $LOCAL is no
	// variable's.
	for _, cell := range g.spills {
		if cell.Frame {
			c.Displacements[g.names.symbol(cell.Name)] = cell.Offset - g.frameSize()
		}
	}
}

// IsConditional reports whether opcode is a conditional branch on the condition
// codes: BEQL, BLSS, BGTRU, ...
func IsConditional(opcode string) bool {
	switch opcode {
	case "BEQL", "BNEQ", "BLSS", "BGEQ", "BLEQ", "BGTR", "BLSSU", "BGEQU", "BLEQU", "BGTRU", "BVS", "BVC", "BCS", "BCC":
		return true
	}

	return false
}

// branchReach is how many bytes opcode's displacement takes, if the instruction
// branches to its last operand; 0 if it doesn't.
func branchReach(opcode string) int {
	switch opcode {
	case "BRB", "BLBC", "BLBS", "SOBGTR", "SOBGEQ", "AOBLEQ", "AOBLSS":
		return 1
	case "BRW", "ACBL":
		return 2
	}

	if IsConditional(opcode) {
		return 1
	}

	return 0
}

// Target is the label the instruction branches to, "" if it isn't a branch.
func (it *Item) Target() string {
	if it.Kind == Instruction && branchReach(it.Opcode) != 0 {
		return it.Operands[len(it.Operands)-1]
	}

	return ""
}

// Fits reports whether the displacement d fits the instruction's branch.
func (it *Item) Fits(d int) bool {
	if branchReach(it.Opcode) == 1 {
		return d >= -128 && d <= 127
	}

	return d >= -32768 && d <= 32767
}

// Bound is an upper bound on the bytes the item assembles to. A pass decides
// that a branch reaches by adding these up, so a bound must never be too small,
// and it is close enough to what MACRO chooses that a branch that could reach
// mostly does (Decision 5 of docs/PHASE-84 - FORTRAN optimizer.md). A test holds
// it to what MACRO assembles (internal/fortran/opt's TestBoundsHold).
func (c *Code) Bound(it *Item) int {
	if it.Kind != Instruction {
		return 0
	}

	if it.Opcode == ".WORD" {
		return 2
	}

	n := opcodeBound(it.Opcode)
	operands := it.Operands

	if reach := branchReach(it.Opcode); reach != 0 {
		n += reach
		operands = operands[:len(operands)-1]
	}

	for _, o := range operands {
		n += c.operandBound(o)
	}

	return n
}

// opcodeBound is the bytes of an instruction's opcode: one, or two for the
// G_floating, H_floating, and octaword instructions, which VAX's first byte
// (^XFD) says go on. Those are the ones whose type letter, last or before a
// count of operands, or in a conversion's name, is one of G, H, and O.
func opcodeBound(opcode string) int {
	op := strings.TrimRight(opcode, "23")

	if strings.HasPrefix(op, "CVT") {
		if strings.ContainsAny(op[3:], "GH") {
			return 2
		}

		return 1
	}

	if op != "" && strings.ContainsRune("GHO", rune(op[len(op)-1])) {
		return 2
	}

	return 1
}

// operandBound is the bytes of one operand: its addressing mode and displacement
// or immediate value, and a byte for an index.
func (c *Code) operandBound(text string) int {
	n := 0

	if strings.HasSuffix(text, "]") {
		n++
		text = text[:strings.LastIndexByte(text, '[')]
	}

	text = strings.TrimPrefix(text, "@")

	switch {
	case isRegisterName(text), text == "-(SP)", strings.HasPrefix(text, "("):
		return n + 1
	case strings.HasPrefix(text, "#"):
		return n + immediateBound(text[1:])
	}

	// A displacement from a register: a number, or a symbol with a known value.
	if open := strings.IndexByte(text, '('); open > 0 {
		if v, ok := c.displacementValue(text[:open]); ok {
			return n + 1 + displacementBytes(v)
		}
	}

	// A symbol in another section, or G^: a longword's displacement.
	return n + 5
}

// isRegisterName reports whether text is a register by itself.
func isRegisterName(text string) bool {
	switch text {
	case "AP", "FP", "SP", "PC":
		return true
	}

	if len(text) < 2 || text[0] != 'R' {
		return false
	}

	_, err := strconv.Atoi(text[1:])

	return err == nil
}

// immediateBound is the bytes of the immediate operand #text: a short literal's
// one for an integer from 0 to 63, which MACRO always chooses, else at most a
// longword's five. No instruction here takes an immediate wider than a longword,
// since floating constants come from $PDATA.
func immediateBound(text string) int {
	if v, err := strconv.Atoi(text); err == nil && v >= 0 && v <= 63 {
		return 1
	}

	if strings.HasPrefix(text, "^A/") && len(text) == 5 && text[3] <= 63 {
		return 1
	}

	return 5
}

// displacementValue is the value of a displacement's text: a number, or a symbol
// in Displacements with an offset after it.
func (c *Code) displacementValue(text string) (int, bool) {
	if v, err := strconv.Atoi(text); err == nil {
		return v, true
	}

	offset := 0
	symbol := text

	if at := strings.LastIndexAny(text, "+-"); at > 0 {
		v, err := strconv.Atoi(text[at:])
		if err != nil {
			return 0, false
		}

		symbol, offset = text[:at], v
	}

	v, ok := c.Displacements[symbol]

	return v + offset, ok
}

// displacementBytes is the bytes of the shortest displacement holding v.
func displacementBytes(v int) int {
	switch {
	case v >= -128 && v <= 127:
		return 1
	case v >= -32768 && v <= 32767:
		return 2
	}

	return 4
}

// Check reports what makes the list wrong: a label defined twice, or a branch to
// a label it doesn't have.
func (c *Code) Check() error {
	labels := map[string]bool{}

	for _, it := range c.Items {
		if it.Kind == Label {
			if labels[it.Name] {
				return fmt.Errorf("the label %s is defined twice", it.Name)
			}

			labels[it.Name] = true
		}
	}

	for _, it := range c.Items {
		if t := it.Target(); t != "" && !labels[t] {
			return fmt.Errorf("%s goes to %s, which isn't there", it.Opcode, t)
		}
	}

	for name := range c.Outside {
		if !labels[name] {
			return fmt.Errorf("the label %s, which code outside the list goes to, isn't there", name)
		}
	}

	return nil
}
