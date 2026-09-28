package rtl

import (
	"time"

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
// Expired timers are processed when the process next touches its event
// flags (eventFlagWord runs expireTimers first). Event flags are only
// visible through services, so that is indistinguishable from expiring
// them on the tick itself — and a wait service ($WAITFR) re-executes every
// instruction step, so a waiting process sees its timer on the first
// check after it expires.

// timerRequest is one $SETIMR request (a VMS timer queue entry).
type timerRequest struct {
	expiry uint64 // VMS system time
	efn    uint32
	reqidt uint32
	mode   uint32 // caller's access mode, for $CANTIM
}

// timerCPUTime is $SETIMR's flags bit 0: the time is CPU time rather than
// elapsed time.
const timerCPUTime = 1

// wallClock is the default Environment.Clock: the host's time, in VMS
// format. The console replaces it with its Engine's SystemTime.
func wallClock() uint64 { return vmsdef.Time(time.Now()) }

// expireTimers sets the event flag of every timer whose time has come,
// removing it from the queue. A timer whose flag is in a common cluster
// the process no longer associates is dropped without effect.
func (env *Environment) expireTimers() {
	if len(env.timers) == 0 {
		return
	}

	now := env.Clock()
	remaining := env.timers[:0]

	for _, t := range env.timers {
		if t.expiry > now {
			remaining = append(remaining, t)

			continue
		}

		if word, bit, st := env.flagWord(t.efn); st == 0 {
			*word |= 1 << bit
		}
	}

	env.timers = remaining
}

// serviceSysSetimr is SYS$SETIMR:
//
//	SYS$SETIMR [efn] ,daytim ,[astadr] ,[reqidt] ,[flags]
//
// It clears event flag efn (default 0) and queues a timer that sets it at
// daytim: a negative quadword is a delta from now, a positive one an
// absolute time (one already past expires at the next check). reqidt
// identifies the request to $CANTIM, which also uses the caller's access
// mode, recorded here.
//
// govax delivers no ASTs, so astadr is accepted but no AST runs; and a
// process's CPU time is its elapsed time (it never waits for another
// process), so flags' CPU-time bit changes nothing. There is no TQELM
// quota (SS$_EXQUOTA).
func serviceSysSetimr(env *Environment, argv []uint32) (uint32, error) {
	efn, daytim, reqidt := optArg(argv, 0), optArg(argv, 1), optArg(argv, 3)

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
	})

	return ssNormal, nil
}

// serviceSysCantim is SYS$CANTIM: cancels the timer requests identified
// by reqidt (every request when it is 0) that were made from acmode —
// maximized with the caller's mode — or a less privileged mode. A
// cancelled timer never sets its flag. It always succeeds.
func serviceSysCantim(env *Environment, argv []uint32) (uint32, error) {
	reqidt := optArg(argv, 0)
	mode := max(optArg(argv, 1)&3, uint32(env.cpu.PSL().CurMod()))

	remaining := env.timers[:0]

	for _, t := range env.timers {
		if (reqidt == 0 || t.reqidt == reqidt) && t.mode >= mode {
			continue
		}

		remaining = append(remaining, t)
	}

	env.timers = remaining

	return ssNormal, nil
}

// cancelTimers is image rundown's timer step: outstanding timer requests
// are cancelled when the image exits.
func (env *Environment) cancelTimers() {
	env.timers = nil
}

// PendingTimers reports how many timer requests are queued.
func (env *Environment) PendingTimers() int { return len(env.timers) }

func registerTimerServices(t *ServiceTable) {
	t.Register("SYS$SETIMR", serviceSysSetimr)
	t.Register("SYS$CANTIM", serviceSysCantim)
}
