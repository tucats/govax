package machine

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

const numTraceRegs = 14

var traceRegNames = [numTraceRegs]string{
	"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9", "R10", "R11", "AP", "FP",
}

// snapshotTraceRegs captures R0-R11/AP/FP and the PSL.
func (m *Machine) snapshotTraceRegs() (regs [numTraceRegs]uint32, psl vax.PSL) {
	for i := range regs {
		regs[i] = m.CPU.GPR(vax.Reg(i))
	}

	return regs, m.CPU.PSL()
}

// printRegisterChanges diffs before/beforePSL against the CPU's current
// state and prints every register that changed: R0-R11 get their value printed in both hex
// and decimal, AP/FP in hex only, and the PSL is reported last if it
// changed.
func (m *Machine) printRegisterChanges(before [numTraceRegs]uint32, beforePSL vax.PSL) {
	for i, name := range traceRegNames {
		after := m.CPU.GPR(vax.Reg(i))
		if after == before[i] {
			continue
		}

		if i < 12 {
			m.Printf("                    %3s:  %08X  %d\n", name, after, after)
		} else {
			m.Printf("                    %3s:  %08X\n", name, after)
		}
	}

	if after := m.CPU.PSL(); uint32(after) != uint32(beforePSL) {
		m.Printf("                    PSL:  %08X\n", uint32(after))
	}
}

var operandAccessNames = [...]string{
	"none", "read", "write", "modify", "address", "bitfield", "branch", "immediate",
}

// printOperandDump prints the most recently executed instruction's operands
// -- access kind, and either the register (+ its value) or the computed
// address/immediate value. This necessarily reads state after cpu.Engine.Step returns;
// Engine.Step decodes and executes in one call with no gap to hook between
// the two -- so a written operand shows its new value rather than its
// pre-execution one; see docs/PHASE-17.md sub-phase 8's own note on this.
func (m *Machine) printOperandDump() {
	dec := m.Engine.LastDecoded()

	for i := 0; i < dec.Instruction.OperandCount; i++ {
		op := dec.Operands[i]

		accessName := "?"
		if access := int(dec.Instruction.Access[i]); access >= 0 && access < len(operandAccessNames) {
			accessName = operandAccessNames[access]
		}

		switch op.Kind {
		case cpu.OperandRegister:
			if op.Reg > 15 {
				m.Printf("            #%d %-9s  T%d = %08X\n", i, accessName, op.Reg-15, m.CPU.GPR(op.Reg))
			} else {
				m.Printf("            #%d %-9s  R%d = %08X\n", i, accessName, op.Reg, m.CPU.GPR(op.Reg))
			}

		case cpu.OperandImmediate:
			m.Printf("            #%d %-9s  %08X\n", i, accessName, uint32(op.Value))

		default: // cpu.OperandMemory
			m.Printf("            #%d %-9s  %08X\n", i, accessName, op.Addr)
		}
	}
}

var traceStackNames = [4]string{"KSP", "ESP", "SSP", "USP"}

// traceStackName reports which stack pc's instruction will run on, matching
// execute_vax's own `vax.pslw.is ? "ISP" : mode_name[vax.pslw.cur_mod]`.
func traceStackName(psl vax.PSL) string {
	if psl.IS() {
		return "ISP"
	}

	return traceStackNames[psl.CurMod()]
}

// traceStep prints the instruction about to execute at pc, called immediately
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
func (m *Machine) traceStep(pc uint32, force bool) (finish func()) {
	if !force && !m.Trace {
		return func() {}
	}

	modep := traceStackName(m.CPU.PSL())
	sp := m.CPU.GPR(vax.SP)

	// Inside an image with a debug symbol table, the instruction is shown
	// as DISASSEMBLE/SYMBOLIC shows it (Phase 41); elsewhere, in the
	// console's own layout.
	if text, ok, err := m.symbolicTraceLine(pc); ok {
		if err == nil {
			m.Printf("[%s %08X] %s\n", modep, sp, text)
		} else {
			m.Printf("[%s %08X] %08X: <disassembly error: %s>\n", modep, sp, pc, err)
		}
	} else if dec, err := m.decodeInstruction(memByteReader{m: m}, pc); err == nil {
		m.Printf("[%s %08X] %08X: %s\n", modep, sp, pc, dec.String())
	} else {
		m.Printf("[%s %08X] %08X: <disassembly error: %s>\n", modep, sp, pc, err)
	}

	trackRegs := m.CPU.DebugEnabled(vax.DebugRegisters)

	var (
		before    [numTraceRegs]uint32
		beforePSL vax.PSL
	)

	if trackRegs {
		before, beforePSL = m.snapshotTraceRegs()
	}

	return func() {
		if trackRegs {
			m.printRegisterChanges(before, beforePSL)
		}

		if m.CPU.DebugEnabled(vax.DebugFullDisasm) {
			m.printOperandDump()
		}
	}
}
