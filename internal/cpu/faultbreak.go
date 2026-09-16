package cpu

import (
	"fmt"
	"sort"
)

// This file implements fault-kind breakpoints (SET BREAKPOINT/FAULT, CLEAR
// BREAKPOINT/FAULT[/ALL], and their entries in SHOW BREAKPOINTS' unified
// listing), the Go port of vax.h's BREAK_FAULT breakpoint kind and its
// three check sites in vax.c's execute_vax: pending-interrupt delivery
// (~line 308), a decode-time fault (~line 417), and a Handler-returned
// fault (~line 484). All three share one behavior this file replicates
// exactly: when a fault's code matches an armed fault breakpoint, the fault
// is recorded (see faulthistory.go) but *not delivered* — no signal frame
// is pushed, no vector is taken, and execution simply stops with PC left
// exactly where the fault was raised, matching the C source's own "set
// vax.fault_pending, return VAX_BREAK" (skipping handle_fault entirely)
// rather than "handle_fault, then possibly break" as address breakpoints
// do.
//
// Deliberate architecture departure from the C source, matching the
// precedent instbreak.go already set for instruction breakpoints: fault
// breakpoints live on Engine, not Console.Breakpoints/BreakKind. The C
// source folds BREAK_FAULT into the very same breakpoint_list BREAK_ADDRESS
// uses, but that works there because C's single execute_vax loop owns both
// address- and fault-checking together. This port splits address-breakpoint
// checking out into Console's own runLoop (between Engine.Step calls,
// docs/PHASE-08.md's design), which has no opportunity to intercept a fault
// raised *during* a Step call -- that has to happen synchronously inside
// Step's own fault-delivery path (raise/deliverPendingInterrupt), which only
// Engine can do. execute.go's BreakKind doc comment is updated accordingly.

// FaultBreak is returned by Engine.Step (via raise/deliverPendingInterrupt)
// in place of HandleFault's own error when a fault-kind breakpoint's code
// matches — see this file's own top comment for why delivery is skipped
// entirely rather than merely interrupted afterward.
type FaultBreak struct {
	Code Exception
}

func (e *FaultBreak) Error() string {
	return fmt.Sprintf("cpu: fault breakpoint hit, code %#02x", e.Code)
}

// SetFaultBreakpoint arms a break-before-delivery breakpoint on code,
// matching SET BREAKPOINT/FAULT <code>. Setting an already-armed code is a
// no-op, matching AddBreakpoint's own duplicate-address handling in
// internal/console/execute.go.
func (e *Engine) SetFaultBreakpoint(code Exception) {
	if e.faultBreaks == nil {
		e.faultBreaks = map[Exception]bool{}
	}
	e.faultBreaks[code] = true
}

// RemoveFaultBreakpoint disarms one fault-kind breakpoint, matching CLEAR
// BREAKPOINT/FAULT <code>. Removing an unarmed code is a no-op.
func (e *Engine) RemoveFaultBreakpoint(code Exception) {
	delete(e.faultBreaks, code)
}

// ClearFaultBreakpoints disarms every fault-kind breakpoint, matching CLEAR
// BREAKPOINT/FAULT/ALL.
func (e *Engine) ClearFaultBreakpoints() {
	e.faultBreaks = nil
}

// FaultBreakpoints returns every currently armed fault code, sorted, for
// SHOW BREAKPOINTS' unified address+fault listing (console_show.c's own
// SHOW BREAK case walks one shared list; this port's Console.ShowBreakpoints
// merges Console.Breakpoints with this instead).
func (e *Engine) FaultBreakpoints() []Exception {
	out := make([]Exception, 0, len(e.faultBreaks))
	for code := range e.faultBreaks {
		out = append(out, code)
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	
	return out
}

// faultBreakHit reports whether code is armed, consulted by
// raise/deliverPendingInterrupt before calling HandleFault.
func (e *Engine) faultBreakHit(code Exception) bool {
	return e.faultBreaks[code]
}
