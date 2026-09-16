package console

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// opcodeString formats op the way this port's other instruction listings do
// (ShowInstructions, show.go): the function byte alone for an ordinary
// opcode, or "extended function" for a two-byte one.
func opcodeString(op cpu.Opcode) string {
	if op.Extended != 0 {
		return fmt.Sprintf("%02X %02X", op.Extended, op.Function)
	}

	return fmt.Sprintf("%02X", op.Function)
}

// lookupInstruction resolves a mnemonic to its Instruction, case-
// insensitively. console_set.c/console_clear.c instead build a fake
// "OPC$_<name>" string and resolve it through the symbol table entries
// init_symbols.c registers for the first 256 (single-byte) opcodes only
// (init_symbols.c:227-238) — this port uses Table.ByName directly instead,
// which costs nothing extra and, as a side effect, also covers the two-byte
// extended opcodes the C source's OPC$_ symbols never registered. This is a
// console-command-scope convenience, not an ISA-fidelity question, so it's
// a deliberate improvement rather than something logged to
// docs/DEVIATIONS.md — see docs/PHASE-18.md.
func lookupInstruction(name string) *cpu.Instruction {
	return cpu.Instructions().ByName(strings.ToUpper(strings.TrimSpace(name)))
}

// instructionBreakpointsInOrder returns every instruction currently flagged
// in Console.InstructionBreakpoints, in Table.All's canonical order (single-
// byte opcodes ascending, then extended ascending) rather than Go's
// unspecified map iteration order — matching decode_opcode.c's own
// `for (n = 0; n < 512; n++)` scan order for SHOW BREAK/INSTRUCTION and
// CLEAR BREAK/INSTRUCTION/ALL.
func (c *Console) instructionBreakpointsInOrder() []*cpu.Instruction {
	var out []*cpu.Instruction

	for _, inst := range cpu.Instructions().All() {
		if c.InstructionBreakpoints[inst] {
			out = append(out, inst)
		}
	}

	return out
}

// pluralS returns "" for n == 1, "s" otherwise — the small pluralization
// console_set.c/console_show.c/console_clear.c each do inline with
// `count == 1 ? "" : "s"`.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}

// AddInstructionBreakpoint implements SET BREAK[POINT]/INSTRUCTION
// <mnemonic> (console_set.c:782-798's BREAK_INSTRUCTION case): flags every
// instance of the named opcode so execution breaks just before it runs
// (see instructionBreakpointHit). Matches the C source's confirmation
// message; unlike the C source, an unrecognized mnemonic is a real error
// (CLI_BADOPCODE) rather than a printed message with the command otherwise
// reporting success — a clear, obvious improvement in error signaling, not
// an ISA-fidelity question.
func (c *Console) AddInstructionBreakpoint(name string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	inst := lookupInstruction(name)
	if inst == nil {
		return vmserrors.New(vmserrors.CLI_BADOPCODE, strings.TrimSpace(name))
	}

	if c.InstructionBreakpoints == nil {
		c.InstructionBreakpoints = make(map[*cpu.Instruction]bool)
	}

	c.InstructionBreakpoints[inst] = true

	c.Printf("Breakpoint set on instruction %s %s\n", opcodeString(inst.Opcode), inst.Name)

	return nil
}

// RemoveInstructionBreakpoint implements CLEAR BREAKPOINT/INSTRUCTION
// <mnemonic> (console_clear.c:89-111's case 551). The C source resolves the
// opcode via asm_hex on a fake "OPC$_<name>" symbol, then — after finding
// the matching instruction[] slot by its own opcode field into a local
// `count` — clears the flag at instruction[n] instead of instruction[count]
// (n still holds the raw opcode value from the symbol lookup, not the table
// index the search just found) and, lacking a `break` at the end of the
// case, falls through into CLEAR STRINGS besides. Both are clear, obvious
// logic slips (a copy-paste index mix-up and a missing break), not
// ISA-fidelity questions, so this port just does the plainly-intended
// thing: clear the flag on the Instruction actually found, once, per
// CLAUDE.md's bug-fixing policy.
func (c *Console) RemoveInstructionBreakpoint(name string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	inst := lookupInstruction(name)
	if inst == nil {
		return vmserrors.New(vmserrors.CLI_BADOPCODE, strings.TrimSpace(name))
	}

	if !c.InstructionBreakpoints[inst] {
		return nil
	}

	delete(c.InstructionBreakpoints, inst)

	c.Printf("Removed breakpoint on instruction %s %s\n", opcodeString(inst.Opcode), inst.Name)

	return nil
}

// ClearAllInstructionBreakpoints implements CLEAR
// BREAKPOINT/INSTRUCTION/ALL (console_clear.c's case 553), printing each
// cleared opcode followed by a count summary, matching the C source.
func (c *Console) ClearAllInstructionBreakpoints() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	insts := c.instructionBreakpointsInOrder()
	if len(insts) == 0 {
		c.Printf("No instruction breakpoints were set.\n")

		return nil
	}

	for _, inst := range insts {
		c.Printf("    %s %s\n", opcodeString(inst.Opcode), inst.Name)
	}

	c.Printf("Cleared %d instruction breakpoint%s\n", len(insts), pluralS(len(insts)))

	c.InstructionBreakpoints = nil

	return nil
}

// ShowInstructionBreakpoints implements SHOW BREAKPOINTS/INSTRUCTIONS
// (console_show.c's case 412, show_break_instr), matching its per-opcode
// listing plus count summary.
func (c *Console) ShowInstructionBreakpoints() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	insts := c.instructionBreakpointsInOrder()
	if len(insts) == 0 {
		c.Printf("No instruction breakpoints set\n")

		return nil
	}

	for _, inst := range insts {
		c.Printf("    %s  %s\n", opcodeString(inst.Opcode), inst.Name)
	}

	c.Printf("%d instruction breakpoint%s set\n", len(insts), pluralS(len(insts)))

	return nil
}

// instructionBreakpointHit reports whether the instruction about to execute
// at the CPU's current PC has been flagged by SET BREAK/INSTRUCTION — the
// Go equivalent of decode_opcode.c's own OP_DBG_BREAK check
// (docs/PHASE-18.md). Peeking the opcode (Engine.PeekInstruction) rather
// than running a real decode keeps this side-effect free: unlike the C
// source, which fully decodes the instruction's operands (autoincrement/
// autodecrement side effects included) before testing the flag, this port
// only identifies which instruction is about to run and defers the real
// decode to Step itself, called only once the breakpoint check has passed —
// avoiding vax.c's own double-decode-adjacent quirks (see step.go's
// stepOver doc comment for a related case) for a debugger feature with no
// ISA-fidelity stakes.
func (c *Console) instructionBreakpointHit() bool {
	if len(c.InstructionBreakpoints) == 0 {
		return false
	}

	inst, err := c.Engine.PeekInstruction()
	if err != nil || inst == nil {
		return false
	}

	return c.InstructionBreakpoints[inst]
}
