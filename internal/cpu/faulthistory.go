package cpu

import "github.com/tucats/govax/internal/vax"

// This file implements the fault/exception event history ring buffer (SET
// FAULT/HISTORY, and the history half of SHOW FAULT — docs/PHASE-16.md
// sub-phase 1b split this out from the pending-interrupt-queue half, which
// landed separately once Phase 14 provided that state). The Go port of
// interrupt.c's fault_history/store_fault/show_faults/set_fault_history: a
// fixed-size ring of the most recent faults, recorded unconditionally by
// set_fault (this port's equivalent: recordFault, called from
// raise/deliverPendingInterrupt right where set_fault itself would run —
// before the fault-breakpoint check in faultbreak.go, matching store_fault
// running inside set_fault *ahead of* vax.c's own BREAK_FAULT check, so a
// fault a breakpoint intercepts is still recorded).
//
// Not ported: struct FAULT's r0/r1 fields. interrupt.c's set_fault captures
// them (`vax.fault.r0 = vax.R0; vax.fault.r1 = vax.R1;`), but show_faults —
// the only reader of a stored FAULT — never prints them; dead state as far
// as this feature's own observable behavior goes, so FaultRecord omits them
// rather than carrying values nothing displays.

// defaultFaultHistory matches interrupt.c's own static default
// (`fault_history_max = 8`).
const defaultFaultHistory = 8

// FaultRecord is one entry in Engine's fault-history ring, matching
// interrupt.c's struct FAULT (minus r0/r1 — see this file's own top
// comment).
type FaultRecord struct {
	Code Exception
	PC   uint32
	PSL  vax.PSL
	Args []uint32
	// Seq is store_fault's own history_id: a count of every fault ever
	// recorded by this Engine, not reset when the ring buffer itself is
	// resized/emptied — matching set_fault_history's own behavior (it
	// clears fault_history/fault_history_p but never touches
	// fault_history_count).
	Seq uint64
}

// SetFaultHistorySize implements SET FAULT/HISTORY <n>: resizes (and always
// empties, matching set_fault_history's own unconditional free+realloc) the
// ring buffer. n<=0 disables recording entirely — a deliberate difference
// from the C source, whose own n==0 case allocates a zero-size buffer that
// the very next store_fault call would write past the end of. That's a
// pre-existing bug in the reference tool's debugging instrumentation, not
// an ISA-fidelity question (this ring buffer has no architectural meaning
// at all — it's a developer aid), so it isn't logged to docs/DEVIATIONS.md;
// this port just avoids reproducing it.
//
// faultHistoryCount (entries actually written since the last resize, capped
// at faultHistoryMax) is reset here, unlike faultHistorySeq: show_faults'
// own C ancestor uses fault_history_count (never reset by
// set_fault_history) as its own loop bound, which can run past however many
// slots have actually been (re)populated since a resize — tolerated there
// only because every slot starts as a NULL pointer show_faults skips over
// (`if (fp) { ... }`). This port's ring buffer has no per-slot "populated"
// sentinel to check the same way, so reusing the never-reset counter the
// same way would return stale or zero-valued entries instead of silently
// skipping them; faultHistoryCount avoids that without changing what a
// caller actually sees (a resize already empties the buffer either way).
func (e *Engine) SetFaultHistorySize(n int) {
	if n < 0 {
		n = 0
	}
	e.faultHistoryMax = n
	e.faultHistory = nil
	e.faultHistoryNext = 0
	e.faultHistoryCount = 0
}

// FaultHistorySize reports the ring buffer's current capacity (SET
// FAULT/HISTORY's own configured n), for SHOW FAULT/diagnostics.
func (e *Engine) FaultHistorySize() int { return e.faultHistoryMax }

// recordFault appends one entry to the fault-history ring, matching
// set_fault's own unconditional store_fault call — see this file's own top
// comment on why this runs before, not after, the fault-breakpoint check.
// A no-op when history is disabled (FaultHistorySize() == 0).
func (e *Engine) recordFault(code Exception, args []uint32, pc uint32, psl vax.PSL) {
	if e.faultHistoryMax <= 0 {
		return
	}
	if e.faultHistory == nil {
		e.faultHistory = make([]FaultRecord, e.faultHistoryMax)
	}

	e.faultHistorySeq++
	e.faultHistory[e.faultHistoryNext] = FaultRecord{
		Code: code, PC: pc, PSL: psl, Args: append([]uint32(nil), args...), Seq: e.faultHistorySeq,
	}
	e.faultHistoryNext = (e.faultHistoryNext + 1) % e.faultHistoryMax
	if e.faultHistoryCount < e.faultHistoryMax {
		e.faultHistoryCount++
	}
}

// FaultHistory returns every retained fault-history entry, oldest first,
// matching show_faults' own oldest-to-newest print order (`p =
// fault_history_p + 1`, the slot about to be overwritten next, walking
// forward from there).
func (e *Engine) FaultHistory() []FaultRecord {
	if e.faultHistoryMax <= 0 || e.faultHistory == nil {
		return nil
	}

	count := e.faultHistoryCount

	out := make([]FaultRecord, count)
	start := (e.faultHistoryNext - count + e.faultHistoryMax) % e.faultHistoryMax
	
	for i := 0; i < count; i++ {
		out[i] = e.faultHistory[(start+i)%e.faultHistoryMax]
	}

	return out
}
