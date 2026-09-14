package cpu

import (
	"errors"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// ErrHalted is returned by a Handler (starting with Phase 04's HALT) to
// stop the machine, and by Engine.Run when that happens — the Go
// equivalent of the C source's vax.halted flag and VAX_HALT return code.
var ErrHalted = errors.New("cpu: halted")

// Engine composes a vax.CPU and vm.Memory with the decode/execute-only state
// the C source keeps on the global vax struct but Phase 01 deliberately left
// out of vax.CPU (registers/PSL only) and Phase 02 left out of vm.Memory
// (owns RAM, not CPU-loop state): the PC of the instruction currently being
// decoded/executed, and whether the machine has halted. See docs/PHASE-03.md's
// design notes on why this is a new type rather than vax.CPU growing these
// fields.
type Engine struct {
	cpu   *vax.CPU
	mem   *vm.Memory
	table *Table

	// instructionPC is the PC at the start of the instruction currently
	// being decoded/executed — the C source's vax.instruction_PC. Faults
	// raised during decode or execution are reported against this address,
	// not wherever decode got to before faulting; see decode.go's doc
	// comment on decodeInstruction.
	instructionPC uint32
	halted        bool
}

// NewEngine returns an Engine driving cpu and mem, using the built-in VAX
// instruction table.
func NewEngine(cpu *vax.CPU, mem *vm.Memory) *Engine {
	return &Engine{cpu: cpu, mem: mem, table: instructionTable}
}

// CPU returns the engine's CPU.
func (e *Engine) CPU() *vax.CPU { return e.cpu }

// Memory returns the engine's memory.
func (e *Engine) Memory() *vm.Memory { return e.mem }

// Halted reports whether the machine has executed a HALT instruction (or
// otherwise been asked to stop; see ErrHalted).
func (e *Engine) Halted() bool { return e.halted }

// Step decodes and executes one instruction. This is the Go port of
// execute_vax's core fetch-decode-execute cycle, minus the console/
// disassembly, breakpoint/single-step, and device-interrupt-queue/clock
// concerns interleaved with it in the C source — see docs/PHASE-03.md's
// design notes. Phase 08's console is expected to layer STEP/breakpoint
// semantics on top of Step rather than this reimplementing them.
//
// A decode-time or Handler-returned *Fault is delivered via HandleFault and
// Step returns nil (execution should continue) unless HandleFault itself
// fails, matching execute_vax's `rc = handle_fault(); if (rc) return rc;
// continue;`. A Handler returning ErrHalted stops the machine and is
// returned as-is; any other Handler error is treated as a *Fault the same
// way a decode-time one is.
func (e *Engine) Step() error {
	e.instructionPC = e.cpu.GPR(vax.PC)

	d, err := decodeInstruction(e.cpu, e.mem, e.table)
	if err != nil {
		return e.raise(err)
	}

	// Advance PC past the instruction before dispatching, matching
	// decode_opcode.c leaving vax.PC there on a successful decode — a
	// branch/jump Handler expects PC to already be "the next sequential
	// instruction" as its starting point.
	e.cpu.SetGPR(vax.PC, d.NextPC)

	handler := e.table.HandlerFor(d.Instruction)
	if err := handler(e, &d); err != nil {
		if errors.Is(err, ErrHalted) {
			e.halted = true
			return ErrHalted
		}
		return e.raise(err)
	}
	return nil
}

// raise turns err into a *Fault (wrapping a raw *vm.TranslationFault/
// *vm.PhysicalAddressError if needed) and runs it through HandleFault,
// first resetting PC to the instruction's start address — matching
// execute_vax's fault paths, which always reset vax.PC that way (via a
// local saved_pc for a decode-time fault, or vax.instruction_PC for an
// execute-time one — the same value in both cases here) before calling
// handle_fault, regardless of whether the fault came from decode or from
// the instruction Handler.
func (e *Engine) raise(err error) error {
	err = wrapMemError(err)
	var f *Fault
	if !errors.As(err, &f) {
		return err
	}
	e.cpu.SetGPR(vax.PC, e.instructionPC)
	return e.HandleFault(f)
}

// Run steps the machine until it halts (ErrHalted) or Step returns any
// other error, matching execute_vax's `while (!vax.halted)` loop stripped
// of everything Step already leaves to Phase 08's console (see Step's doc
// comment). It always returns a non-nil error: ErrHalted on a normal stop,
// or whatever error ended the loop early.
func (e *Engine) Run() error {
	for !e.halted {
		if err := e.Step(); err != nil && !errors.Is(err, ErrHalted) {
			return err
		}
	}
	return ErrHalted
}
