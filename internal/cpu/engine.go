package cpu

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// ErrHalted is returned by a Handler (starting with Phase 04's HALT) to
// stop the machine, and by Engine.Run when that happens — the Go
// equivalent of the C source's vax.halted flag and VAX_HALT return code.
var ErrHalted = vmserrors.New(vmserrors.VAX_HALTED)

// ErrAttention is returned by Engine.Step when Attention has been called
// since the last BeginRun — the Go equivalent of console.c's attention()
// setting vax.halted = VAX_ATTENTION on SIGINT, observed the same way
// vax.c's own `while (!vax.halted)` loop observes it: refused at the start
// of the next instruction, once whatever's currently executing finishes,
// not torn out mid-instruction. Named Attention rather than Interrupt to
// avoid colliding with Engine's own, unrelated Interrupt method
// (interrupt.go): that one admits a VAX device/software interrupt request
// into the emulated machine's own interrupt queue, a completely different
// concept from a host Ctrl-C asking the console to regain control.
var ErrAttention = vmserrors.New(vmserrors.VAX_ATTENTION)

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

	// attentionRequested backs Attention/AttentionRequested — the Go
	// equivalent of vax.halted's VAX_ATTENTION value, set by console.c's
	// SIGINT handler attention(). Unlike every other Engine field, this one
	// is written from outside the goroutine that calls Step (cmd/govax's
	// own process-wide Ctrl-C plumbing runs on a separate goroutine), hence
	// atomic.Bool rather than a plain bool.
	attentionRequested atomic.Bool

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

	// Per-run instruction/time budget -- see limits.go. Both zero-valued
	// (no limit) on a freshly constructed Engine.
	instrLimit  int
	instrCount  int
	timeLimit   time.Duration
	runDeadline time.Time

	// faultBreaks holds every exception code currently armed as a
	// break-before-delivery breakpoint (SET BREAKPOINT/FAULT) -- see
	// faultbreak.go. Checked synchronously inside raise/
	// deliverPendingInterrupt, so (unlike address breakpoints, which
	// Console's own runLoop checks between Engine.Step calls) this can't
	// live on Console.Breakpoints; see faultbreak.go's own doc comment.
	faultBreaks map[Exception]bool

	// Fault/exception event history ring buffer (SET FAULT/HISTORY, the
	// history half of SHOW FAULT) -- see faulthistory.go.
	faultHistory      []FaultRecord
	faultHistoryMax   int
	faultHistoryNext  int
	faultHistoryCount int
	faultHistorySeq   uint64
}

// NewEngine returns an Engine driving cpu and mem, using the built-in VAX
// instruction table.
func NewEngine(cpu *vax.CPU, mem *vm.Memory) *Engine {
	return &Engine{
		cpu:             cpu,
		mem:             mem,
		table:           instructionTable,
		quantumCurrent:  defaultQuantum,
		quantumInitial:  defaultQuantum,
		faultHistoryMax: defaultFaultHistory,
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

// Halt marks the machine halted without going through a HALT instruction --
// used by docs/PHASE-20.md's format_exception port, matching
// interrupt.c's own direct `vax.halted = 1` when no condition handler wants
// an unhandled exception.
func (e *Engine) Halt() { e.halted = true }

// ClearHalted unconditionally marks the machine not halted -- used by
// docs/PHASE-20.md's Condition Handling Facility port (chf's own
// `vax.halted = saved_halt` restore after invoking a handler, for the
// unusual case where running the handler itself executed a HALT).
func (e *Engine) ClearHalted() { e.halted = false }

// Attention requests that the next Engine.Step call stop before executing
// another instruction, returning ErrAttention once whatever's currently
// running finishes — the Go equivalent of console.c's attention(). Named to
// avoid colliding with Engine's own, unrelated Interrupt method (see
// ErrAttention's own doc comment). Safe to call from any goroutine, unlike
// virtually every other Engine method; see cmd/govax's own Ctrl-C handling,
// the only intended caller. A fresh top-level run (BeginRun) clears this,
// matching execute_vax's own `vax.halted = 0` at the top of every run — a
// Ctrl-C pressed while idle at the console prompt, with nothing running,
// has no effect on the next command.
func (e *Engine) Attention() { e.attentionRequested.Store(true) }

// AttentionRequested reports whether Attention has been called since the
// last BeginRun.
func (e *Engine) AttentionRequested() bool { return e.attentionRequested.Load() }

// LastDecoded returns whatever instruction the most recent Step call
// decoded — the same value Step reuses across calls to avoid a fresh heap
// allocation every instruction (this function's own doc comment explains
// why), exposed read-only for a caller that wants to inspect the
// just-executed instruction's operands (e.g. internal/console's
// DebugFullDisasm trace — see docs/PHASE-17.md sub-phase 8). The zero
// value if Step has never been called.
func (e *Engine) LastDecoded() Decoded { return e.decoded }

// PeekInstruction identifies which Instruction would execute next at the
// engine's current PC, without decoding operands, advancing PC, or
// otherwise mutating any state — a side-effect-free lookup Console uses to
// check instruction-level breakpoints (SET BREAK/INSTRUCTION,
// docs/PHASE-18.md) before committing to a real Step; unlike a full decode,
// this never triggers an operand's autoincrement/autodecrement side effect,
// so a flagged instruction can be identified without disturbing machine
// state if the breakpoint fires. Returns nil, nil for a reserved/undefined
// opcode (matching Table.Lookup); a memory error reading the opcode byte(s)
// themselves (e.g. a translation fault) is returned as-is and otherwise
// ignored by the caller, left for the real Step to raise properly.
func (e *Engine) PeekInstruction() (*Instruction, error) {
	op, _, err := fetchOpcode(e.cpu, e.mem, e.cpu.GPR(vax.PC))
	if err != nil {
		return nil, err
	}

	return e.table.Lookup(op), nil
}

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
	if err := e.checkLimits(); err != nil {
		return err
	}

	if e.attentionRequested.Load() {
		return ErrAttention
	}

	e.instrCount++

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
	var f *Fault

	err = wrapMemError(err)
	if !errors.As(err, &f) {
		return err
	}

	e.cpu.SetGPR(vax.PC, e.instructionPC)

	// recordFault matches set_fault's own unconditional store_fault call —
	// see faulthistory.go's own doc comment on why this runs ahead of the
	// fault-breakpoint check below, not just ahead of HandleFault.
	e.recordFault(f.Code, f.Args, e.instructionPC, e.cpu.PSL())

	if e.cpu.DebugEnabled(vax.DebugExceptions) {
		w := e.cpu.DebugWriter()
		fmt.Fprintf(w, "DEBUG(EXCEPTION): SET, CODE=%02X  PC=%08X  PSL=%08X  ARGC=%d\n",
			f.Code, e.instructionPC, uint32(e.cpu.PSL()), len(f.Args))

		if len(f.Args) > 0 {
			plural := "S"
			if len(f.Args) == 1 {
				plural = ""
			}

			fmt.Fprintf(w, "DEBUG(EXCEPTION): ARG%s = ", plural)

			for i, arg := range f.Args {
				if i > 0 {
					fmt.Fprint(w, ", ")
				}

				fmt.Fprintf(w, "%08X", arg)
			}

			fmt.Fprintln(w)
		}
	}

	// A fault-kind breakpoint (SET BREAKPOINT/FAULT) intercepts delivery
	// entirely — see faultbreak.go's own top comment on why this matches
	// vax.c's own "set vax.fault_pending, return VAX_BREAK" rather than
	// running handle_fault first.
	if e.faultBreakHit(f.Code) {
		return &FaultBreak{Code: f.Code}
	}

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
