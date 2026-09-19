package cpu

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
)

// This file is the Go port of interrupt.c's interrupt() (device/software
// interrupt admission) and vax.c's execute_vax's own "quantum" block (the
// interrupt-queue aging scan and interval-clock tick) -- see docs/PHASE-14.md.
//
// Design departure from the C source, deliberate and user-directed: the C
// reference's real driving clock for both the quantum counter and the
// interval timer is a wall-clock SIGALRM handler (todr_timer, Source/
// Console/console.c) -- vax.c's own "decrement every instruction" comment
// describes intent, but the actual decrement it would take is commented out
// (`/* vax.quantum.current--; */ /* Let the ms timer handle this? */`) in
// favor of the signal handler's real-time-driven one. This port instead
// decrements the quantum counter once per Engine.Step call (see tickQuantum,
// called from Step) -- a deterministic, instruction-count-driven model
// (matching this project's design generally, and requested explicitly over
// wall-clock timing for this phase) rather than reintroducing real
// concurrency/timers for a simulator whose whole point is reproducible
// execution.

// defaultQuantum is the number of instructions between quantum-boundary
// evaluations (device-interrupt queue aging, interval-clock ticks) a fresh
// Engine starts with, matching initialization.c's own non-alarm-driven
// default (`vax.quantum.initial = 20`).
const defaultQuantum = 20

// queuedInterrupt is one entry on Engine's device/software interrupt queue,
// matching vax.h's struct INTERRUPT (code/ipl/age; the C source's own
// `quantum` field is kept only long enough to compute age below, so it isn't
// retained as separate state here).
type queuedInterrupt struct {
	code Exception
	ipl  uint32
	age  int
}

// SetQuantum sets the number of Engine.Step calls between quantum-boundary
// evaluations, matching console_set.c's SET QUANTUM n (0 suspends interrupt
// delivery admission's quantum-delay/aging behavior entirely, matching that
// command's own "interrupt delivery suspended" case -- immediate,
// unmasked interrupts still deliver, since that path in Interrupt doesn't
// consult the quantum at all).
func (e *Engine) SetQuantum(n int) {
	e.quantumInitial = n
	e.quantumCurrent = n
}

// Quantum reports the current quantum-boundary countdown and its initial
// (reload) value, matching console_show.c's SHOW QUANTUM.
func (e *Engine) Quantum() (current, initial int) {
	return e.quantumCurrent, e.quantumInitial
}

// QueuedInterrupt is one interrupt still waiting in Engine's quantum-aging
// queue, mirroring queuedInterrupt's own fields for callers outside this
// package (SHOW FAULT).
type QueuedInterrupt struct {
	Code Exception
	IPL  uint32
	Age  int
}

// PendingInterrupts reports Engine's own immediately-pending interrupt (if
// any, deliverable on the next Step) and every interrupt still waiting in
// its quantum-aging queue, matching console_show.c's own SHOW FAULT dump
// of vax.interrupt_pending/vax.iqueue.
func (e *Engine) PendingInterrupts() (pending *QueuedInterrupt, queued []QueuedInterrupt) {
	if e.interruptPending {
		pending = &QueuedInterrupt{Code: e.interruptCode, IPL: e.interruptIPL}
	}

	for _, ip := range e.iqueue {
		queued = append(queued, QueuedInterrupt{Code: ip.code, IPL: ip.ipl, Age: ip.age})
	}

	return pending, queued
}

// ClearInterrupt removes every queued interrupt whose code matches, matching
// console_clear.c's CLEAR INTERRUPT <id> (case 102). Returns the number of
// entries removed, matching that command's own "Cleared %d pending
// interrupt%s" report. Does not touch Engine's own immediately-pending
// interrupt (interruptPending) — the C source's equivalent loop only ever
// walks vax.iqueue, not vax.interrupt_pending, for this id-specific form.
func (e *Engine) ClearInterrupt(code Exception) int {
	n := 0
	remaining := e.iqueue[:0]

	for _, ip := range e.iqueue {
		if ip.code == code {
			n++

			continue
		}

		remaining = append(remaining, ip)
	}

	e.iqueue = remaining

	return n
}

// ClearAllInterrupts empties the interrupt queue and cancels Engine's own
// immediately-pending interrupt, matching console_clear.c's CLEAR
// INTERRUPT/ALL (case 110). Returns the number of queued entries removed
// (matching that command's own count, which — like the C source — doesn't
// separately count a cancelled interrupt_pending in its total).
func (e *Engine) ClearAllInterrupts() int {
	n := len(e.iqueue)
	e.iqueue = nil
	e.interruptPending = false
	e.interruptCode = 0
	e.interruptIPL = 0

	return n
}

// Interrupt admits a device or software interrupt request, the Go port of
// interrupt.c's interrupt(): delivered immediately (Engine's own
// interruptPending state, picked up by Step before its next decode) if
// nothing is already pending and ipl exceeds the current PSL IPL; queued
// for Engine's own quantum-boundary aging scan (scanInterruptQueue)
// otherwise. quantum is the number of quantum ticks to defer delivery by
// even when otherwise immediately admittable (0 for "as soon as possible");
// a few call sites (e.g. RXCS's own IE-just-enabled-while-DON-already-set
// case) request a 1-tick delay, matching the C source.
func (e *Engine) Interrupt(code Exception, ipl uint32, quantum int) {
	queue := e.interruptPending || e.cpu.PSL().IPL() >= ipl
	if e.quantumInitial > 0 && quantum > 0 {
		queue = true
	}

	if !queue {
		if e.cpu.DebugEnabled(vax.DebugInterrupts) {
			fmt.Fprintf(e.cpu.DebugWriter(), "DEBUG(INTERRUPTS): set interrupt %04X, IPL %d, quantum %d\n",
				code, ipl, quantum)
		}

		e.interruptPending = true
		e.interruptCode = code
		e.interruptIPL = ipl

		return
	}

	age := 0
	if e.quantumInitial > 0 {
		age = quantum / e.quantumInitial
	}

	if e.cpu.DebugEnabled(vax.DebugInterrupts) {
		fmt.Fprintf(e.cpu.DebugWriter(), "DEBUG(INTERRUPTS): queue interrupt %04X, IPL %d, age=%d, quantum %d\n",
			code, ipl, age, quantum)
	}

	e.iqueue = append(e.iqueue, &queuedInterrupt{code: code, ipl: ipl, age: age})

	if e.quantumInitial > 0 && e.quantumCurrent > quantum {
		e.quantumCurrent = quantum
	}
}

// tickQuantum is called once per Engine.Step, matching vax.c's own
// `vax.quantum.current <= 0` block: decrements the quantum counter, and on
// reaching zero, reloads it, ticks the interval clock, and (if nothing is
// already pending) scans the interrupt queue for an entry ready to admit.
func (e *Engine) tickQuantum() {
	e.quantumCurrent--
	if e.quantumCurrent > 0 {
		return
	}

	e.quantumCurrent = e.quantumInitial

	e.tickIntervalClock()

	if !e.interruptPending {
		e.scanInterruptQueue()
	}
}

// scanInterruptQueue is the Go port of execute_vax's own interrupt-queue
// scan (vax.c, ~lines 229-285): every entry ages by one tick; the first
// entry whose age has reached zero and whose IPL exceeds the current PSL
// IPL is admitted (removed from the queue, becomes Engine's pending
// interrupt) -- at most one per call, matching the C source's `break` once
// it finds one.
func (e *Engine) scanInterruptQueue() {
	if len(e.iqueue) == 0 {
		return
	}

	ipl := e.cpu.PSL().IPL()
	debug := e.cpu.DebugEnabled(vax.DebugInterrupts)
	remaining := e.iqueue[:0]

	for _, ip := range e.iqueue {
		if debug {
			fmt.Fprintf(e.cpu.DebugWriter(), "DEBUG(INTERRUPTS): quantum; evaluating interrupt %04X, IPL %d, age=%d\n",
				ip.code, ip.ipl, ip.age)
		}

		switch {
		case ip.age > 0:
			ip.age--
			remaining = append(remaining, ip)

		case e.interruptPending:
			remaining = append(remaining, ip)

		case ip.ipl > ipl:
			e.interruptPending = true
			e.interruptCode = ip.code
			e.interruptIPL = ip.ipl

			if debug {
				fmt.Fprintf(e.cpu.DebugWriter(), "DEBUG(INTERRUPTS): set interrupt %04X\n", ip.code)
			}

		default:
			remaining = append(remaining, ip)
		}
	}

	e.iqueue = remaining
}

// deliverPendingInterrupt runs the VAX interrupt-delivery sequence for
// Engine's own pending interrupt, the Go port of execute_vax's own
// `if (vax.interrupt_pending) { set_fault(...); vax.IPL = ...; handle_fault(); }`
// block (minus the fault-breakpoint check, a console/Phase-08 concern with
// no interrupt-specific behavior to add here).
//
// Deviation from the C source, fixed rather than replicated (a clear-cut
// issue, not an ISA judgment call -- see docs/DEVIATIONS.md): handle_fault
// saves vax.instruction_PC as the interrupted return address, but for a
// genuine interrupt (as opposed to a fault raised mid-instruction)
// vax.instruction_PC still holds the *previous* instruction's start
// address -- decode_instruction only updates it when decoding the next
// instruction, which happens after this block runs. The correct return
// address for an interrupt, delivered strictly between instructions, is
// simply the current PC (already advanced past whatever last executed).
// This port uses that instead, so RTI/REI from a device interrupt resumes
// at the right place rather than re-executing the previous instruction.
func (e *Engine) deliverPendingInterrupt() error {
	code := e.interruptCode
	ipl := e.interruptIPL
	e.interruptPending = false
	e.interruptCode = 0
	e.interruptIPL = 0
	e.interruptCount++

	e.cpu.SetPR(vax.IPL, ipl)
	e.instructionPC = e.cpu.GPR(vax.PC)

	// recordFault/faultBreakHit match vax.c's own execute_vax handling of
	// this same delivery path (set_fault, then the BREAK_FAULT check, both
	// ahead of handle_fault) — see faulthistory.go/faultbreak.go.
	e.recordFault(code, nil, e.instructionPC, e.cpu.PSL())

	if e.faultBreakHit(code) {
		return &FaultBreak{Code: code}
	}

	return e.HandleFault(&Fault{Code: code})
}

// ICCS bit masks, matching emul_procreg.c's own literal masks for case 24.
// deviceIE (bit 6) is reused, at the same position, by RXCS/TXCS below --
// matching the C source's own reuse of that bit across all three registers.
const (
	iccsRun  = 0x00000001
	iccsXFR  = 0x00000010
	iccsSGL  = 0x00000020
	deviceIE = 0x00000040
	iccsInt  = 0x00000080
	iccsErr  = 0x80000000
)

// tickIntervalClock is the Go port of vax.c's own interval-clock tick
// (~lines 168-193) merged with console.c's todr_timer (~lines 247-251) --
// see this file's own doc comment on why the two are merged into a single,
// quantum-tick-driven step rather than kept as separate instruction-count
// and wall-clock-driven pieces. Uses PR(ICR) directly as the live countdown
// register (the C source keeps a separate vax.clock mirrored into vax.ICR
// on every read/relevant write; nothing in this port's design needs that
// extra mirror, since there's no real-time reader racing the CPU loop the
// way the C source's SIGALRM handler could).
func (e *Engine) tickIntervalClock() {
	iccs := e.cpu.PR(vax.ICCS)
	if iccs&iccsRun == 0 {
		return
	}

	clock := e.cpu.PR(vax.ICR) + 1
	nicr := e.cpu.PR(vax.NICR)

	if clock == 0xFFFFFFFF || clock < nicr {
		if iccs&iccsInt != 0 {
			iccs |= iccsErr
		}

		iccs |= iccsInt
		if iccs&deviceIE != 0 {
			e.Interrupt(ExcInterval, 22, 0)
		}

		clock = nicr
	}

	e.cpu.SetPR(vax.ICR, clock)
	e.cpu.SetPR(vax.ICCS, iccs)
}

// DeliverConsoleByte simulates a console-terminal input interrupt: deposits
// b into RXDB and sets RXCS's DON bit, admitting an EXC$CONREAD interrupt
// at IPL 20 if RXCS's IE bit is set -- the Go port of what vax.c's own
// poll_keyboard *would* do on a real keystroke, ported directly as a
// callable primitive rather than a platform keyboard-hit poll.
//
// Unlike poll_keyboard -- which is a permanent no-op on every platform this
// project's own reference build targets (vax.c guards the whole body with
// `#if defined(macintosh) || defined(WIN)`, and CLAUDE.md's own build notes
// say the Xcode target always builds LINUX86 -- so eVAX's console-terminal
// *receive* interrupt has never actually fired on the reference platform)
// -- this is a real, working mechanism: any caller with a real byte in hand
// (a future interactive front end reading a live terminal, or a test) can
// deliver it correctly, IE-gating and all, addressing a gap the C reference
// itself never closed. See docs/DEVIATIONS.md.
//
// If RXCS's DON bit is already set (the previous byte hasn't been read
// yet), the new byte is dropped rather than overwriting RXDB, matching
// poll_keyboard's own `if (vax.RXCS & 0x80) { ...; return 0; }` guard --
// noticed while wiring DebugKeyboard's trace (docs/PHASE-17.md sub-phase 2)
// and fixed here rather than left silently overwriting, since the trace
// text itself ("not read yet, ignoring") would otherwise be a lie about
// what this port actually does.
func (e *Engine) DeliverConsoleByte(b byte) {
	if e.cpu.PR(vax.RXCS)&0x80 != 0 {
		if e.cpu.DebugEnabled(vax.DebugKeyboard) {
			fmt.Fprintln(e.cpu.DebugWriter(), "KBD: hit, not read yet, ignoring")
		}

		return
	}

	e.cpu.SetPR(vax.RXDB, uint32(b))
	rxcs := e.cpu.PR(vax.RXCS) | 0x80
	e.cpu.SetPR(vax.RXCS, rxcs)

	if rxcs&deviceIE != 0 {
		e.Interrupt(ExcConRead, 20, 0)
	}
}
