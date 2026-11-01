package registers

// The operand log. An operand specifier's autoincrement or autodecrement
// changes its register as the instruction is decoded. If the instruction
// then faults, it hasn't happened: the architecture has the saved PC point
// at it and its operands' registers put back, so that a handler can correct
// what was wrong and continue, and it runs again from the start (a reserved
// floating operand under LIB$FIXUP_FLT, a page a program's handler creates).
// So decode notes each register's value before it changes it.

// operandChange is one register as it was before a specifier changed it.
type operandChange struct {
	reg   Reg
	value uint32
}

// maxOperandChanges is an instruction's most: six operands, each with one
// register to change.
const maxOperandChanges = 6

// BeginOperands empties the log, as an instruction's decode starts.
func (c *CPU) BeginOperands() { c.operandChanges = 0 }

// NoteOperandRegister records reg's value, before a specifier changes it.
func (c *CPU) NoteOperandRegister(reg Reg) {
	if c.operandChanges < maxOperandChanges {
		c.operandLog[c.operandChanges] = operandChange{reg, c.GPR(reg)}
		c.operandChanges++
	}
}

// RestoreOperandRegisters puts back what the instruction's specifiers
// changed, the last change first (a register used twice ends as it began),
// and empties the log.
func (c *CPU) RestoreOperandRegisters() {
	for i := c.operandChanges - 1; i >= 0; i-- {
		c.SetGPR(c.operandLog[i].reg, c.operandLog[i].value)
	}

	c.operandChanges = 0
}
