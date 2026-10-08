package corevms

import (
	"time"

	"github.com/tucats/govax/internal/sched"

	"github.com/tucats/govax/internal/vmsdef"
)

// $SETIMR and $CANTIM (docs/PHASE-26.md subtask 11): the process's timer
// queue.
//
// The timers are the RTL's own — the Go equivalent of the VMS executive's
// timer queue — not built on the guest's interval-timer interrupt (the
// microkernel's exe$interval handler). They share its time base instead:
// Environment.Clock is the console Engine's SystemTime, which advances one
// millisecond per interval-clock tick in both of the engine's clock modes.
// So a timer means the same thing as the interval clock does, stays
// deterministic in quantum mode, yet fires whether or not the guest has
// enabled ICCS interrupts, installed a handler, or lowered its IPL.
//
// Expired timers are processed before every instruction, when the engine
// asks for ASTs (NextAST, ast.go), and also whenever the process touches
// its event flags (eventFlagWord runs expireTimers first). The second
// path is what subtask 11 started with, when there were no ASTs and
// event flags were only visible through services; it remains for code
// driven without an engine. Those two expire only the running process's
// timers. With the scheduler (docs/PHASE-44.md, subtask 5), every
// process's are expired at each scheduling call (pollEvents, waits.go),
// which comes no later than the next timer of any process is due
// (System.budget), and an idle CPU moves time on to the next one
// (idle.go): so timers are system-wide in effect, as on VMS.

// timerRequest is one entry in the process's timer queue (a VMS timer
// queue entry, TQE). Two services queue them, and VMS keeps both kinds in
// the same queue:
//
//   - $SETIMR: when expiry comes, event flag efn is set, and if astadr
//     isn't 0 an AST is queued to call it in mode, with reqidt as its
//     parameter. reqidt and mode also identify the request to $CANTIM.
//   - $SCHDWK (wake set): when expiry comes, the process is woken
//     (Process.WakePending, hibernate.go). If repeat is nonzero the entry
//     stays queued and fires again every repeat ticks. Only $CANWAK
//     cancels these; $CANTIM leaves them alone.
type timerRequest struct {
	expiry uint64 // VMS system time
	efn    uint32
	reqidt uint32
	mode   uint32 // caller's access mode: the AST's mode, and for $CANTIM
	astadr uint32 // $SETIMR's AST routine, or 0 for none

	wake   bool   // a $SCHDWK wakeup rather than a $SETIMR timer
	repeat uint64 // $SCHDWK reptim, in ticks; 0 for a one-shot
}

// timerCPUTime is $SETIMR's flags bit 0: the time is CPU time rather than
// elapsed time.
const timerCPUTime = 1

// wallClock is the default Environment.Clock: the host's time, in VMS
// format. The console replaces it with its Engine's SystemTime.
func wallClock() uint64 { return vmsdef.Time(time.Now()) }

// expireTimers acts on every queued request whose time has come: a
// $SETIMR timer sets its event flag, queues its AST if it has one, and
// leaves the queue; a $SCHDWK wakeup wakes the process, and leaves the
// queue unless it repeats. A timer whose flag is in a common cluster the
// process no longer associates sets no flag, but its AST (which doesn't
// depend on the flag) is still queued.
//
// The engine runs this before every instruction (via NextAST, ast.go),
// so a timer fires on the tick it's due; the event-flag services also
// run it, for code driven without an engine (the unit tests).
//
// When any has expired, a timer's expiry is reported to the scheduler
// (reportEvent): a process waiting for it becomes computable with the
// timer's boost (PRI$_TIMER).
func (env *Environment) expireTimers() {
	if len(env.timers) == 0 {
		return
	}

	now := env.Clock()
	remaining := env.timers[:0]
	fired := false

	for _, t := range env.timers {
		if t.expiry > now {
			remaining = append(remaining, t)

			continue
		}

		fired = true

		if t.wake {
			env.Process.WakePending = true

			if t.repeat != 0 {
				// Step to the first repetition still in the future.
				// Wakeups that were missed in between collapse into this
				// one, as they would on VMS: WakePending isn't a count.
				t.expiry += (now-t.expiry)/t.repeat*t.repeat + t.repeat
				remaining = append(remaining, t)
			}

			continue
		}

		env.postFlag(t.efn, sched.ClassTimer)

		if t.astadr != 0 {
			env.queueAST(t.astadr, t.reqidt, t.mode)
		}
	}

	env.timers = remaining

	if fired {
		env.reportEvent(sched.ClassTimer)
	}
}

// serviceSysSetimr is SYS$SETIMR:
//
//	SYS$SETIMR [efn] ,daytim ,[astadr] ,[reqidt] ,[flags]
//
// It clears event flag efn (default 0) and queues a timer that sets it at
// daytim: a negative quadword is a delta from now, a positive one an
// absolute time (one already past expires at the next check). reqidt
// identifies the request to $CANTIM, which also uses the caller's access
// mode, recorded here. When the timer expires, an astadr other than 0 is
// called as an AST in that mode, with reqidt as its parameter.
//
// A process's CPU time is its elapsed time (it never waits for another
// process), so flags' CPU-time bit changes nothing. There is no TQELM
// quota (SS$_EXQUOTA).
func serviceSysSetimr(env *Environment, argv []uint32) (uint32, error) {
	efn, daytim, astadr, reqidt := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2), optArg(argv, 3)

	word, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	if daytim == 0 { // page 0 is never accessible on VMS
		return ssAccVio, nil
	}

	expiry, ok := env.loadQuad(daytim)
	if !ok {
		return ssAccVio, nil
	}

	*word &^= 1 << bit

	if delta := int64(expiry); delta < 0 {
		expiry = env.Clock() + uint64(-delta)
	}

	env.timers = append(env.timers, &timerRequest{
		expiry: expiry,
		efn:    efn,
		reqidt: reqidt,
		mode:   uint32(env.cpu.PSL().CurMod()),
		astadr: astadr,
	})

	return ssNormal, nil
}

// serviceSysCantim is SYS$CANTIM: cancels the timer requests identified
// by reqidt (every request when it is 0) that were made from acmode —
// maximized with the caller's mode — or a less privileged mode. A
// cancelled timer never sets its flag. Scheduled wakeups ($SCHDWK) aren't
// timer requests to $CANTIM; $CANWAK cancels those. It always succeeds.
func serviceSysCantim(env *Environment, argv []uint32) (uint32, error) {
	reqidt := optArg(argv, 0)
	mode := max(optArg(argv, 1)&3, uint32(env.cpu.PSL().CurMod()))

	remaining := env.timers[:0]

	for _, t := range env.timers {
		if !t.wake && (reqidt == 0 || t.reqidt == reqidt) && t.mode >= mode {
			continue
		}

		remaining = append(remaining, t)
	}

	env.timers = remaining

	return ssNormal, nil
}

// cancelTimers is image rundown's timer step: outstanding timer requests
// and scheduled wakeups are cancelled when the image exits.
func (env *Environment) cancelTimers() {
	env.timers = nil
}

// PendingTimers reports how many timer requests (including scheduled
// wakeups) are queued.
func (env *Environment) PendingTimers() int { return len(env.timers) }

func registerTimerServices(t *ServiceTable) {
	t.Register("SYS$SETIMR", serviceSysSetimr)
	t.Register("SYS$CANTIM", serviceSysCantim)
}
