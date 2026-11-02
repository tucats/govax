package opt

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/codegen"
)

// Peephole replaces single instructions with shorter ones that do the same,
// looking at each as if through a small hole, knowing nothing of what's around it.
// Nearly all of the VAX's general instructions have a short form for their commonest
// operand, and most of them the code generator already chooses as it goes:
//
//	I = 0            CLRL    T.I           not MOVL #0,T.I
//	I = I + 1        INCL    T.I           not ADDL2 #1,T.I
//
// What's left is where a value only became one of those on the way: an element
// number that is a constant zero, loaded into a register to index with, is
// MOVL #0,R0, and that is CLRL R0, a byte shorter. Each replacement sets the
// condition codes as the instruction it replaces does, so what tests them after
// it doesn't know the difference: N and Z from the value, V and C as they were or
// as the addition's were.
//
// CMPL X,#0 and TSTL X differ in nothing but length either, as subtracting zero
// neither borrows nor overflows.
func Peephole(c *codegen.Code) {
	for _, it := range c.Items {
		if it.Kind == codegen.Instruction {
			simplify(it)
		}
	}
}

// simplify replaces one instruction if it can.
func simplify(it *codegen.Item) {
	if len(it.Opcode) < 4 {
		return
	}

	op, ops := it.Opcode, it.Operands

	switch {
	case strings.HasPrefix(op, "MOV") && isIntegerLetter(op[3:]) && ops[0] == "#0":
		it.Opcode, it.Operands = "CLR"+op[3:], ops[1:]

	case strings.HasPrefix(op, "CMP") && isIntegerLetter(op[3:]) && ops[1] == "#0":
		it.Opcode, it.Operands = "TST"+op[3:], ops[:1]

	case strings.HasPrefix(op, "ADD") && isIntegerTwo(op[3:]) && ops[0] == "#1":
		it.Opcode, it.Operands = "INC"+op[3:4], ops[1:]

	case strings.HasPrefix(op, "SUB") && isIntegerTwo(op[3:]) && ops[0] == "#1":
		it.Opcode, it.Operands = "DEC"+op[3:4], ops[1:]
	}
}

// isIntegerLetter reports whether s is the type letter of a byte, word, or
// longword.
func isIntegerLetter(s string) bool {
	return s == "B" || s == "W" || s == "L"
}

// isIntegerTwo reports whether s is that letter and a 2: the two-operand form.
func isIntegerTwo(s string) bool {
	return s == "B2" || s == "W2" || s == "L2"
}
