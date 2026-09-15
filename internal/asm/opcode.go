package asm

import "fmt"

// opcodeAliases maps an alternate mnemonic spelling to the real instruction
// name the assembler should look up instead, matching asm_opcode.c's
// alias_names table. The reference tool gates GAS-dialect-only entries
// (JBR) behind a runtime dialect switch, but since the default dialect is
// ASM_DIALECT_ANY — under which alias_opcode() matches every entry
// regardless of its tagged dialect — this port just applies the whole
// table unconditionally; see docs/PHASE-11.md.
var opcodeAliases = map[string]string{
	"JBR":    "JMP",
	"BNEQU":  "BNEQ",
	"BEQLU":  "BEQL",
	"BGEQU":  "BCC",
	"BLSSU":  "BCS",
	"CLRD":   "CLRQ",
	"CLRF":   "CLRL",
	"MOVAF":  "MOVAL",
	"PUSHAF": "PUSHAL",
}

// assembleOpcode assembles a real VAX instruction: mnemonic, then its
// operands, matching asm_opcode()+the operand loop in assemble(). The
// mnemonic table comes from internal/cpu (Phase 03's decoder), not a
// duplicate copy, per docs/PHASE-11.md.
func (a *Assembler) assembleOpcode(c *cursor) error {
	c.skipBlanks()

	if c.atEnd() {
		return nil // a label with nothing else on the line is fine
	}

	start := c.pos

	for !isBlank(c.peek()) && !c.atEnd() {
		c.pos++
	}

	name := c.s[start:c.pos]

	if realName, ok := opcodeAliases[name]; ok {
		name = realName
	}

	inst := a.table.ByName(name)
	if inst == nil {
		return fmt.Errorf("invalid opcode %q", name)
	}

	if inst.Opcode.Extended != 0 {
		if err := a.image.storeByte(a.deposit, inst.Opcode.Extended); err != nil {
			return err
		}

		a.deposit++
	}

	if err := a.image.storeByte(a.deposit, inst.Opcode.Function); err != nil {
		return err
	}

	a.deposit++
	a.caseBase = 0

	for n := 0; n < inst.OperandCount; n++ {
		c.skipBlanks()

		if c.atEnd() {
			return fmt.Errorf("%s: insufficient operands", inst.Name)
		}

		if err := a.assembleOperand(c, inst, n); err != nil {
			return fmt.Errorf("%s operand %d: %w", inst.Name, n+1, err)
		}

		c.skipBlanks()
		
		if n < inst.OperandCount-1 && c.atEnd() {
			return fmt.Errorf("%s: insufficient operands", inst.Name)
		}

		if c.peek() == ',' {
			c.next()
		}
	}

	return nil
}
