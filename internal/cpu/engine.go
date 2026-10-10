package cpu

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// ErrHalted is returned by a Handler (starting with Phase 04's HALT) to
// stop the machine, and by Engine.Run when that happens. The "silent"
// variant is returned by a Handler that wants to stop the machine without
// printing a message (e.g. when the XFC #7B silent halt is executed).
var (
	ErrHalted       = vmserrors.New(vmserrors.VAX_HALTED)
	ErrHaltedSilent = vmserrors.InhibitMessage(vmserrors.New(vmserrors.VAX_HALTED_SILENT))
)

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

	// attentionKey backs Attention/AttentionKey/AttentionRequested — the
	// Go equivalent of vax.halted's VAX_ATTENTION value, set by console.c's
	// SIGINT handler attention(). It holds the control character typed
	// (AttentionCtrlC or AttentionCtrlY), or 0 when there's none pending.
	// Unlike every other Engine field, this one is written from outside
	// the goroutine that calls Step (main.go's own process-wide Ctrl-C
	// plumbing runs on a separate goroutine), hence atomic.
	attentionKey atomic.Uint32

	// stoppedBy is the attention key that ended the current run with
	// ErrAttention (StoppedBy), or 0. Written only on Step's goroutine.
	stoppedBy byte

	// attentionHandler is services' attention half (attention.go), when
	// it has one: it may turn a pending attention key into a CTRL/C or
	// CTRL/Y AST instead of stopping the machine.
	attentionHandler AttentionHandler

	// services is the XFC opcode's hook into console/RTL state (see
	// services.go and xfc.go). nil until SetSystemServices is called, in
	// which case every XFC selector that needs it reports a reserved-
	// operand fault, matching an XFC executed before the microkernel
	// environment it depends on exists.
	services SystemServices

	// astSource is services' AST half (ast.go), when it has one; nil
	// otherwise, and then Step never checks for ASTs.
	astSource ASTSource

	// scheduler is the scheduling hook (schedule.go), or nil when only
	// one process runs. Step calls it when schedLeft, the instructions
	// left of the schedBudget its last call allowed, reaches zero;
	// preemptModes are the access modes it may preempt in.
	scheduler              Scheduler
	schedBudget, schedLeft int
	preemptModes           PreemptModes

	// schedFrozen counts FreezeScheduling calls not yet undone: while
	// it's above zero, Step doesn't call the scheduler, so no process
	// switch happens.
	schedFrozen int

	// decoded is Step's own pair of reusable Decoded buffers -- see Step's
	// doc comment on why they exist (a Phase 12 performance-pass finding,
	// not part of the original Phase 03 design). decoded[current&1] is the
	// last instruction decoded whole; Step decodes the next one into the
	// other buffer and makes it current only if the decode succeeds.
	decoded [2]Decoded
	current int

	// Quantum/interrupt-admission state -- see interrupt.go. quantumCurrent/
	// quantumInitial are zero-valued (quantum disabled, admission masking
	// still active) on an Engine built directly as a struct literal rather
	// than via NewEngine; existing tests that predate Phase 14 rely on this.
	quantumCurrent, quantumInitial int
	iqueue                         []*queuedInterrupt
	interruptPending               bool
	interruptCode                  Exception
	interruptIPL                   uint32
	interruptCount                 uint64

	// hardwareClock is the vax.hardware.clock setting: true to keep time
	// by the host's real clock, false to count instructions into emulated
	// milliseconds (quantum mode, deterministic). See clock.go.
	hardwareClock bool

	// bootTime is the VMS system time the Engine was created at, and
	// clockTicks the interval-clock ticks since, in quantum mode: together
	// they are SystemTime (systime.go).
	bootTime   uint64
	clockTicks uint64

	// lastClock is the host clock's reading, in Unix milliseconds, the last
	// time hardware-clock mode took an interval-clock tick (pollHostClock,
	// clock.go).
	lastClock uint64

	// todrOffset is how far the time-of-year register has been set away
	// from the clock's own reading by a write to it (SetTODR, clock.go).
	todrOffset uint32

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

var debugConfig = map[string]vax.DebugFlags{
	"vax.debug.vm":         vax.DebugVM,
	"vax.debug.tb":         vax.DebugTB,
	"vax.debug.symbols":    vax.DebugSymbols,
	"vax.debug.exceptions": vax.DebugExceptions,
	"vax.debug.interrupts": vax.DebugInterrupts,
	"vax.debug.chm":        vax.DebugCHM,
	"vax.debug.registers":  vax.DebugRegisters,
	"vax.debug.fulldisasm": vax.DebugFullDisasm,
	"vax.debug.userhalt":   vax.DebugUserHalt,
	"vax.debug.keyboard":   vax.DebugKeyboard,
	"vax.debug.images":     vax.DebugImages,
	"vax.debug.services":   vax.DebugServices,
	"vax.debug.dcl":        vax.DebugDCL,
	"vax.debug.command":    vax.DebugExpand,
	"vax.debug.logicals":   vax.DebugLogicals,
	"vax.debug.devices":    vax.DebugDevices,
	"vax.debug.process":    vax.DebugProcess,
	"vax.debug.libinit":    vax.DebugLibinit,
	"vax.debug.rms":        vax.DebugRMS,
	"vax.debug.userstep":   vax.DebugUserStep,
}

// NewEngine returns an Engine driving cpu and mem, using the built-in VAX
// instruction table.
func NewEngine(cpu *vax.CPU, mem *vm.Memory) *Engine {
	quantum := defaultQuantum

	// If there is a defalt quantum value defined in the configuration, use it.
	if v := settings.GetInt("vax.quantum"); v > 0 {
		quantum = v
	}

	// Search for config items that define the default debug settings as well. IF
	// the value is given in the config, set or clear the debug mask bits accordingly.
	debugMask := cpu.Debug()

	for key, mask := range debugConfig {
		if text := settings.Get(key); text != "" {
			value, err := strconv.ParseBool(strings.TrimSpace(text))
			if err == nil {
				if value {
					debugMask |= mask
				} else {
					debugMask &= ^mask
				}
			}
		}
	}

	cpu.SetDebug(debugMask)

	// Return an emulation engine object
	return &Engine{
		cpu:             cpu,
		mem:             mem,
		table:           instructionTable,
		quantumCurrent:  quantum,
		quantumInitial:  quantum,
		faultHistoryMax: defaultFaultHistory,
		schedLeft:       noSchedulerBudget,
		hardwareClock:   settings.GetBool("vax.hardware.clock"),
		lastClock:       uint64(time.Now().UnixMilli()),
		bootTime:        vmsdef.Time(time.Now()),
	}
}

// SetSystemServices installs s as the XFC opcode's hook into console/RTL
// state — see services.go.
func (e *Engine) SetSystemServices(s SystemServices) {
	e.services = s
	e.astSource, _ = s.(ASTSource)
	e.attentionHandler, _ = s.(AttentionHandler)
}

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
// virtually every other Engine method; see main.go's own Ctrl-C handling,
// the only intended caller. A fresh top-level run (BeginRun) clears a
// pending CTRL/C, matching execute_vax's own `vax.halted = 0` at the top
// of every run — a Ctrl-C pressed while no program runs has no effect on
// the next command. A pending CTRL/Y is kept: it asks govax itself to
// stop (cmd/govax/attention.go), so a run that begins after it was typed
// stops at once.
func (e *Engine) Attention() { e.AttentionKey(AttentionCtrlC) }

// AttentionKey is Attention for a particular control character: key is
// AttentionCtrlC or AttentionCtrlY, the character the user typed. The
// services' AttentionHandler, if any, sees which one (a VMS program can
// ask for an AST on either). Safe to call from any goroutine.
func (e *Engine) AttentionKey(key byte) { e.attentionKey.Store(uint32(key)) }

// AttentionRequested reports whether Attention has been called since the
// last BeginRun (and not taken by an AttentionHandler).
func (e *Engine) AttentionRequested() bool { return e.attentionKey.Load() != 0 }

// PendingAttention returns the attention key typed and not yet dealt with
// (AttentionCtrlC or AttentionCtrlY), or 0. Safe to call from any
// goroutine.
func (e *Engine) PendingAttention() byte { return byte(e.attentionKey.Load()) }

// StoppedBy returns the attention key that stopped the current run (the
// last BeginRun's) with ErrAttention, or 0 if none did: a key a program's
// CTRL/C or CTRL/Y AST took didn't stop it. Call it on the goroutine that
// runs Step, once the run is over.
func (e *Engine) StoppedBy() byte { return e.stoppedBy }

// LastDecoded returns whatever instruction the most recent Step call
// decoded — the same value Step reuses across calls to avoid a fresh heap
// allocation every instruction (this function's own doc comment explains
// why), exposed read-only for a caller that wants to inspect the
// just-executed instruction's operands (e.g. internal/console's
// DebugFullDisasm trace — see docs/PHASE-17.md sub-phase 8). The zero
// value if Step has never been called.
func (e *Engine) LastDecoded() Decoded { return e.decoded[e.current&1] }

// InstructionCount returns the number of instructions that have been decoded
// and executed by the emulation engine.
func (e *Engine) InstructionCount() int {
	return e.instrCount
}

// InterruptCount returns the number of interrupts that have been processed
// during engine execution.
func (e *Engine) InterruptCount() uint64 {
	return e.interruptCount
}

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
	// The console may have changed MAPEN or the PSL since the last Step;
	// see Step's own SyncFetchWindow call.
	e.mem.SyncFetchWindow(e.cpu)

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
// The Decoded lives in the Engine, not in a local: taking &d on a plain
// local and passing it through handler (an indirectly-called function
// value) defeats Go's escape analysis, forcing a fresh heap allocation of
// the whole [6]Operand-sized struct on every single instruction --
// confirmed by profiling (docs/PHASE-12.md's own performance-pass
// sub-phase). decodeInstruction fills in one of e.decoded's two buffers
// directly, so there is neither an allocation nor a copy (Study 1's R2 in
// docs/PERFORMANCE.md removed the copies). It decodes into the buffer that
// isn't current, so that a decode fault leaves LastDecoded showing the
// last instruction decoded whole, as it did when the decode was copied
// into place only on success.
func (e *Engine) Step() error {
	if err := e.checkLimits(); err != nil {
		return err
	}

	if err := e.checkAttention(); err != nil {
		return err
	}

	e.instrCount++

	// Advance the clocks (clock.go). In hardware-clock mode, look at the
	// host's clock only every hostClockPollInterval instructions: reading
	// it costs more than most instructions do. In quantum mode, count this
	// instruction toward the next emulated millisecond.
	if e.hardwareClock {
		if e.instrCount&hostClockPollMask == 0 {
			e.pollHostClock()
		}
	} else {
		e.tickQuantum()
	}

	if e.interruptPending {
		if err := e.deliverPendingInterrupt(); err != nil {
			return err
		}
	}

	// The scheduler's turn (schedule.go): count this instruction against
	// the current process, and when its budget is used up, let the
	// scheduler run, which may switch to another process. This comes
	// after interrupt delivery, so an interrupt handler (on the interrupt
	// stack, at a raised IPL) is never switched away from, and before AST
	// delivery, so a process switched to gets its own pending ASTs at once.
	if e.scheduler != nil {
		if e.schedLeft <= 0 && e.schedFrozen == 0 {
			if err := e.schedule(); err != nil {
				return err
			}
		}

		e.schedLeft--
	}

	// An AST, like an interrupt, is taken between instructions. The RTL
	// declines while an interrupt handler just started above runs (its
	// IPL is too high), so the two never nest the wrong way round.
	if e.astSource != nil {
		if err := e.deliverAST(); err != nil {
			return e.raise(err)
		}
	}

	e.instructionPC = e.cpu.GPR(vax.PC)

	// The decoder reads the instruction stream through the memory's
	// instruction-fetch window (internal/vm/fetch.go; docs/PERFORMANCE.md,
	// Study 1, R4), one page translated once and then read directly.
	// Memory mapping (MAPEN) or the CPU's access mode may have changed
	// since the window was filled, by the instruction just executed, an
	// interrupt or AST delivered above, or the console between steps;
	// either change empties the window, so the fetches below translate
	// afresh. Checking once here is cheaper than checking on every byte.
	e.mem.SyncFetchWindow(e.cpu)

	next := e.current ^ 1
	dec := &e.decoded[next&1]

	if err := decodeInstruction(e.cpu, e.mem, e.table, dec); err != nil {
		return e.raise(err)
	}

	e.current = next

	// Advance PC past the instruction before dispatching, matching
	// decode_opcode.c leaving vax.PC there on a successful decode — a
	// branch/jump Handler expects PC to already be "the next sequential
	// instruction" as its starting point.
	e.cpu.SetGPR(vax.PC, dec.NextPC)

	handler := e.table.HandlerFor(dec.Instruction)
	if err := handler(e, dec); err != nil {
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
