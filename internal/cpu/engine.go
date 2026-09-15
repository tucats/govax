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

	// services is the XFC opcode's hook into console/RTL state (see
	// services.go and xfc.go). nil until SetSystemServices is called, in
	// which case every XFC selector that needs it reports a reserved-
	// operand fault, matching an XFC executed before the microkernel
	// environment it depends on exists.
	services SystemServices

	// decoded is Step's own reusable Decoded buffer -- see Step's doc
	// comment on why this exists (a Phase 12 performance-pass finding, not
	// part of the original Phase 03 design).
	decoded Decoded

	// Quantum/interrupt-admission state -- see interrupt.go. quantumCurrent/
	// quantumInitial are zero-valued (quantum disabled, admission masking
	// still active) on an Engine built directly as a struct literal rather
	// than via NewEngine; existing tests that predate Phase 14 rely on this.
	quantumCurrent, quantumInitial int
	iqueue                         []*queuedInterrupt
	interruptPending               bool
	interruptCode                  Exception
	interruptIPL                   uint32
}

// NewEngine returns an Engine driving cpu and mem, using the built-in VAX
// instruction table.
func NewEngine(cpu *vax.CPU, mem *vm.Memory) *Engine {
	return &Engine{
		cpu:            cpu,
		mem:            mem,
		table:          instructionTable,
		quantumCurrent: defaultQuantum,
		quantumInitial: defaultQuantum,
	}
}

// SetSystemServices installs s as the XFC opcode's hook into console/RTL
// state — see services.go.
func (e *Engine) SetSystemServices(s SystemServices) { e.services = s }

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
// Step decodes and executes exactly one instruction.
//
// decodeInstruction returns a Decoded by value; taking &d on a plain local
// and passing it through handler (an indirectly-called function value)
// defeats Go's escape analysis, forcing a fresh heap allocation of the
// whole [6]Operand-sized struct on every single instruction -- confirmed
// by profiling (docs/PHASE-12.md's own performance-pass sub-phase):
// ~1 allocation per Step call, no exceptions, all attributed directly to
// this function. Decoding into e.decoded (a field of the already-heap-
// resident *Engine, reused across every Step call) instead avoids that
// allocation entirely -- copying the freshly decoded value into it is a
// plain, non-escaping struct copy, not a new allocation.
func (e *Engine) Step() error {
	e.tickQuantum()
	if e.interruptPending {
		if err := e.deliverPendingInterrupt(); err != nil {
			return err
		}
	}

	e.instructionPC = e.cpu.GPR(vax.PC)

	dec, err := decodeInstruction(e.cpu, e.mem, e.table)
	if err != nil {
		return e.raise(err)
	}
	e.decoded = dec

	// Advance PC past the instruction before dispatching, matching
	// decode_opcode.c leaving vax.PC there on a successful decode — a
	// branch/jump Handler expects PC to already be "the next sequential
	// instruction" as its starting point.
	e.cpu.SetGPR(vax.PC, e.decoded.NextPC)

	handler := e.table.HandlerFor(e.decoded.Instruction)
	if err := handler(e, &e.decoded); err != nil {
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
