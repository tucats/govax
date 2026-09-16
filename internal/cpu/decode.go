package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// Decoded is one fully decoded instruction: which Instruction it is and its
// resolved operands, ready for a Handler (Phases 04-07) to execute. This is
// the value-based equivalent of the C source's struct OPCODE — see
// docs/PHASE-03.md's design notes on why operands are resolved to values/
// addresses rather than raw pointers.
type Decoded struct {
	Opcode      Opcode
	Instruction *Instruction
	// Operands holds exactly Instruction.OperandCount entries, in operand
	// order (operand 0 first).
	Operands [6]Operand
	// NextPC is the virtual address of the byte immediately following this
	// instruction's last operand specifier — the C source's `vax.PC` as
	// left by decode_opcode.c on success. The caller (Engine.Step, sub-
	// phase 6) is responsible for writing this into cpu's PC register
	// before dispatching to the instruction's Handler; decodeInstruction
	// itself never mutates cpu's PC (see below).
	NextPC uint32
}

// fetchOpcode reads the opcode byte(s) at pc from mem: one byte for an
// ordinary opcode, or two (prefix + function, when the first byte is 0xFD
// or 0xFC) for an extended one — decode_opcode.c's own "f > 0xFC" test.
// Returns the decoded Opcode and the address immediately following it.
// Shared by decodeInstruction and Engine.PeekInstruction, the latter needing
// exactly this much of decode_opcode.c's own logic — opcode identification,
// no operand decode — for instruction-level breakpoints (SET
// BREAK/INSTRUCTION, docs/PHASE-18.md) to identify the next instruction
// without the operand-decode side effects (autoincrement/autodecrement)
// a full decode would incur.
func fetchOpcode(cpu *vax.CPU, mem *vm.Memory, pc uint32) (Opcode, uint32, error) {
	f, err := mem.LoadByte(cpu, pc)
	if err != nil {
		return Opcode{}, 0, err
	}
	pc++

	if f > 0xFC {
		// Extended (two-byte) opcode: f is the prefix, the next byte is the
		// actual function code.
		f2, err := mem.LoadByte(cpu, pc)
		if err != nil {
			return Opcode{}, 0, err
		}
		pc++
		return Opcode{Extended: f, Function: f2}, pc, nil
	}

	return Opcode{Function: f}, pc, nil
}

// decodeInstruction fetches and fully decodes one instruction starting at
// cpu's current PC, consulting table for the opcode and mem for the
// instruction stream and any addressing-mode memory accesses.
//
// This is the Go port of decode_opcode.c's decode_instruction, minus
// disassembly, symbol-table/.ENTRY lookups, breakpoint/debug-flag data, and
// profiling counters — all console (Phase 08) or disassembler (Phase 11)
// concerns, not decode itself (see docs/PHASE-03.md's design notes).
//
// Unlike the C source, decodeInstruction does not write back to cpu's PC
// register at all, on success or on fault: decode_opcode.c updates
// vax.PC at several checkpoints purely so a fault raised mid-decode leaves
// vax.PC in a sensible state for the caller, but execute_vax's fault path
// resets PC to the instruction's start address regardless (the saved
// pre-decode PC, either via a local `saved_pc` for a decode-time fault or
// via `vax.instruction_PC` for an execute-time one) before signaling the
// fault — so the intermediate writes are never actually observed. Engine.Step
// (sub-phase 6) owns saving the start PC and writing NextPC back on success.
func decodeInstruction(cpu *vax.CPU, mem *vm.Memory, table *Table) (Decoded, error) {
	pc := cpu.GPR(vax.PC)

	opcode, pc, err := fetchOpcode(cpu, mem, pc)
	if err != nil {
		return Decoded{}, err
	}

	inst := table.Lookup(opcode)
	if inst == nil {
		// Reserved-to-Digital or otherwise undefined opcode — a privileged/
		// reserved instruction fault, matching decode_opcode.c's
		// `set_fault(EXC_PRIV, 0)` for an unrecognized extended opcode.
		return Decoded{}, &Fault{Code: ExcPrivileged}
	}

	d := Decoded{Opcode: opcode, Instruction: inst}

	for i := 0; i < inst.OperandCount; i++ {
		op, err := decodeOperand(cpu, mem, &pc, inst.Access[i], inst.Scale[i], inst.Type, false)
		d.Operands[i] = op
		if err != nil {
			return d, err
		}
	}

	d.NextPC = pc
	return d, nil
}
