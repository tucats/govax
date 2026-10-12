package console

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// WAIT (docs/PHASE-50 - DCL command procedures.md): DCL's own command
// that waits a while, as the DCL Dictionary describes it.
//
//	WAIT delta-time
//
// The delta time is [dddd-][hh:mm:ss.cc], as F$CVTIME reads one
// (parseDeltaTime). The wait is real time, the host's, whatever the
// machine's clock. The other processes run meanwhile (dcllend.go):
// process 1's WAIT lends them the CPU, waiting in the scheduler, and a
// subprocess's WAIT waits there, its shim's XFC run again when the time
// is up. A Ctrl/Y or Ctrl/C typed during process 1's WAIT ends it at
// once, as one typed during any of DCL's own commands is acted on when
// the command is done (dclcontroly.go); a subprocess's isn't attached to
// the terminal, and waits its whole time.

// waitPoll is how often WAIT looks for an attention key.
const waitPoll = 20 * time.Millisecond

// waitCommand is WAIT delta-time. A time that can't be read is
// CLI$_IVDTIME, its segment the time as VMS 7.3 showed it for WAIT
// NOTATIME: "0  NOTATIME", as though the time were made a delta of 0
// days (testdata/dcl50's round 6; one case seen, whose segment VMS ended
// with a NUL byte, left out here as its buffer's garbage). WAIT
// leaves $STATUS alone, interrupted and resumed or run to its end
// (rounds 5 and 6).
func (d *Dispatcher) waitCommand(id int64, r *dcl.Result) error {
	c := d.Console
	text := strings.TrimSpace(r.String("DELTA"))

	ticks, ok := parseDeltaTime(strings.ToUpper(text))
	if !ok {
		return vmserrors.NewSegment(vmserrors.CLI_IVDTIME, "0  "+text)
	}

	c.keepStatus()

	deadline := time.Now().Add(time.Duration(ticks) * (time.Second / time.Duration(vmsdef.TicksPerSecond)))

	done := func() bool { return !time.Now().Before(deadline) }

	if cli := c.subCLI; cli != nil {
		for !done() {
			cli.handBack(cliYield{kind: yieldTimer, over: done, deadline: c.machineDeadline(time.Until(deadline))})
		}

		return nil
	}

	attention := func() bool { return c.Engine != nil && isAttentionKey(c.Engine.PendingAttention()) }

	for {
		left := time.Until(deadline)
		if left <= 0 || attention() {
			return nil
		}

		// The others run while process 1 waits; when none can, the
		// host waits a while.
		if !c.lendCPU(func() bool { return done() || attention() }, c.machineDeadline(left)) {
			time.Sleep(min(time.Until(deadline), waitPoll))
		}
	}
}
