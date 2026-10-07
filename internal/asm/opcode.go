package asm

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// opcodeAliases maps an alternate mnemonic spelling to the real instruction
// name the assembler should look up instead. The reference tool gates 
// GAS-dialect-only entries (JBR) behind a runtime dialect switch, but 
// since the default dialect is ASM_DIALECT_ANY — under which alias_opcode() 
// matches every entry regardless of its tagged dialect — this just 
// applies the whole table unconditionally; see docs/PHASE-11.md.
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

	// The VAX has no separate clear, move-address, or push-address
	// instructions for the floating types: each uses the integer
	// instruction of the same size, since those only move bits or
	// addresses. The manual and VAX MACRO name them both ways. D_floating
	// and G_floating are quadword-sized, H_floating octaword-sized (the
	// octaword instructions are Phase 35's, at FD7C-FD7F).
	"MOVAD":  "MOVAQ",
	"PUSHAD": "PUSHAQ",
	"CLRG":   "CLRQ",
	"MOVAG":  "MOVAQ",
	"PUSHAG": "PUSHAQ",
	"CLRH":   "CLRO",
	"MOVAH":  "MOVAO",
	"PUSHAH": "PUSHAO",
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
	written := name

	if realName, ok := opcodeAliases[name]; ok {
		name = realName
	}

	inst := a.table.ByName(name)
	if inst == nil {
		return vmserrors.New(vmserrors.VAX_BADOPCODE, name)
	}

	// The opcode, then each operand specifier, is a group of the
	// listing's binary field (see listField.group).
	a.listOp(inst.Name)
	a.listInstruction()
	a.xrefOpcode(written, opcodeValue(inst))
	a.listGroup(0)

	defer a.listGroup(0)

	if inst.Opcode.Extended != 0 {
		if err := a.emitByte(inst.Opcode.Extended); err != nil {
			return err
		}

		a.listJoin()
	}

	if err := a.emitByte(inst.Opcode.Function); err != nil {
		return err
	}
	
	a.caseBase = 0

	for n := 0; n < inst.OperandCount; n++ {
		c.skipBlanks()

		if c.atEnd() {
			return vmserrors.New(vmserrors.VAX_BADOPERANDS, inst.Name)
		}

		// A branch displacement joins the operand before it, as real
		// MACRO lists it (SOBGTR R4, BACK is F6 54   F5); only a branch's
		// sole operand is a group of its own (BRB BACK is FE   11).
		if inst.Access[n] == cpu.AccessBranch && n > 0 {
			a.listGroup(n)
		} else {
			a.listGroup(n + 1)
		}

		start := c.pos

		if err := a.assembleOperand(c, inst, n); err != nil {
			return vmserrors.Wrap(vmserrors.VAX_OPERANDERR, err, inst.Name, n+1)
		}

		c.skipBlanks()

		// Real MACRO lists too few operands at the last one there is.
		if n < inst.OperandCount-1 && c.atEnd() {
			c.pos = start

			return vmserrors.New(vmserrors.VAX_BADOPERANDS, inst.Name)
		}

		if c.peek() == ',' {
			c.next()
		}
	}

	return nil
}

// opcodeValue returns inst's opcode as the cross reference shows it: a
// one-byte opcode's value, or a two-byte one's bytes as a word, the
// prefix (FD) low, as the listing's binary field shows them (32FD).
func opcodeValue(inst *cpu.Instruction) uint32 {
	if inst.Opcode.Extended != 0 {
		return uint32(inst.Opcode.Function)<<8 | uint32(inst.Opcode.Extended)
	}

	return uint32(inst.Opcode.Function)
}
