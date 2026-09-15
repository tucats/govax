package console

import (
	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// numTraceRegs is R0-R11 plus AP/FP -- registers.c's own reglist, and the
// n<14 bound check_regset diffs against. SP/PC are deliberately excluded:
// both change on every single instruction and would be pure noise in a
// per-step diff, matching check_regset exactly.
const numTraceRegs = 14

// traceRegNames matches registers.c's reglist[] names for indices 0-13
// (R0-R11, AP, FP).
var traceRegNames = [numTraceRegs]string{
	"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9", "R10", "R11", "AP", "FP",
}

// snapshotTraceRegs captures R0-R11/AP/FP and the PSL, matching
// save_regset (registers.c:61-70).
func (c *Console) snapshotTraceRegs() (regs [numTraceRegs]uint32, psl vax.PSL) {
	for i := range regs {
		regs[i] = c.CPU.GPR(vax.Reg(i))
	}
	return regs, c.CPU.PSL()
}

// printRegisterChanges diffs before/beforePSL against the CPU's current
// state and prints every register that changed, matching check_regset
// (registers.c:72-87) exactly: R0-R11 get their value printed in both hex
// and decimal, AP/FP in hex only, and the PSL is reported last if it
// changed.
func (c *Console) printRegisterChanges(before [numTraceRegs]uint32, beforePSL vax.PSL) {
	for i, name := range traceRegNames {
		after := c.CPU.GPR(vax.Reg(i))
		if after == before[i] {
			continue
		}
		if i < 12 {
			c.Printf("                    %3s:  %08X  %d\n", name, after, after)
		} else {
			c.Printf("                    %3s:  %08X\n", name, after)
		}
	}

	if after := c.CPU.PSL(); uint32(after) != uint32(beforePSL) {
		c.Printf("                    PSL:  %08X\n", uint32(after))
	}
}

// operandAccessNames matches console_disasm.c's own mp[] array (format_operands),
// indexed by cpu.AccessKind.
var operandAccessNames = [...]string{
	"none", "read", "write", "modify", "address", "bitfield", "branch", "immediate",
}

// printOperandDump prints the most recently executed instruction's operands
// -- access kind, and either the register (+ its value) or the computed
// address/immediate value -- matching format_operands (console_disasm.c:
// 183-207). Unlike the C source (which prints this before the instruction
// runs), this necessarily reads state after cpu.Engine.Step returns --
// Engine.Step decodes and executes in one call with no gap to hook between
// the two -- so a written operand shows its new value rather than its
// pre-execution one; see docs/PHASE-17.md sub-phase 8's own note on this.
func (c *Console) printOperandDump() {
	dec := c.Engine.LastDecoded()

	for i := 0; i < dec.Instruction.OperandCount; i++ {
		op := dec.Operands[i]

		accessName := "?"
		if access := int(dec.Instruction.Access[i]); access >= 0 && access < len(operandAccessNames) {
			accessName = operandAccessNames[access]
		}

		switch op.Kind {
		case cpu.OperandRegister:
			if op.Reg > 15 {
				c.Printf("            #%d %-9s  T%d = %08X\n", i, accessName, op.Reg-15, c.CPU.GPR(op.Reg))
			} else {
				c.Printf("            #%d %-9s  R%d = %08X\n", i, accessName, op.Reg, c.CPU.GPR(op.Reg))
			}
		case cpu.OperandImmediate:
			c.Printf("            #%d %-9s  %08X\n", i, accessName, uint32(op.Value))
		default: // cpu.OperandMemory
			c.Printf("            #%d %-9s  %08X\n", i, accessName, op.Addr)
		}
	}
}

// traceStackNames matches vax.c:113's own mode_name[] local to execute_vax
// -- stack-pointer names, not to be confused with console_show.c's
// differently-scoped mode_names[] (KERNEL/EXEC/SUPER/USER, see
// accessModeNames in internal/cpu/call.go). See docs/PHASE-17.md sub-phase 7.
var traceStackNames = [4]string{"KSP", "ESP", "SSP", "USP"}

// traceStackName reports which stack pc's instruction will run on, matching
// execute_vax's own `vax.pslw.is ? "ISP" : mode_name[vax.pslw.cur_mod]`.
func traceStackName(psl vax.PSL) string {
	if psl.IS() {
		return "ISP"
	}
	return traceStackNames[psl.CurMod()]
}

// traceStep prints the instruction about to execute at pc, matching
// vax.c:437-454's own `if (disasm) { ... }` block -- called immediately
// before cpu.Engine.Step by every loop that drives execution (Execute,
// Step, Call), so an instruction is always traced (if at all) before it
// runs, not after. Traces only when force is true or c.Trace is set
// (matching STEP's own "always in trace mode" vs. EXEC/GO/CALL/RUN's own
// vax.console.disasm gating -- see docs/PHASE-17.md sub-phase 7).
//
// The returned finish func must be called after cpu.Engine.Step returns,
// to print DebugRegisters' changed-register dump and DebugFullDisasm's
// operand dump (docs/PHASE-17.md sub-phase 8) -- both need the
// instruction to have actually run.
func (c *Console) traceStep(pc uint32, force bool) (finish func()) {
	if !force && !c.Trace {
		return func() {}
	}

	modep := traceStackName(c.CPU.PSL())
	sp := c.CPU.GPR(vax.SP)

	if dec, err := asm.Disassemble(memByteReader{c: c}, pc); err == nil {
		c.Printf("[%s %08X] %08X: %s\n", modep, sp, pc, dec.String())
	} else {
		c.Printf("[%s %08X] %08X: <disassembly error: %s>\n", modep, sp, pc, err)
	}

	trackRegs := c.CPU.DebugEnabled(vax.DebugRegisters)

	var before [numTraceRegs]uint32
	var beforePSL vax.PSL
	if trackRegs {
		before, beforePSL = c.snapshotTraceRegs()
	}

	return func() {
		if trackRegs {
			c.printRegisterChanges(before, beforePSL)
		}
		if c.CPU.DebugEnabled(vax.DebugFullDisasm) {
			c.printOperandDump()
		}
	}
}
