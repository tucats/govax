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
// machine's clock; nothing runs meanwhile. A Ctrl/Y or Ctrl/C typed
// during it ends it at once, as one typed during any of DCL's own
// commands is acted on when the command is done (dclcontroly.go).

// waitPoll is how often WAIT looks for an attention key.
const waitPoll = 20 * time.Millisecond

// waitCommand is WAIT delta-time. A time that can't be read is
// CLI$_IVDTIME, with the time as its segment. WAIT leaves $STATUS
// alone (VMS 7.3 showed the status before a WAIT that Ctrl/Y interrupted
// and CONTINUE resumed, testdata/dcl50's round 5; a WAIT that runs to
// its end is taken to do the same, unconfirmed).
func (d *Dispatcher) waitCommand(id int64, r *dcl.Result) error {
	c := d.Console
	text := strings.TrimSpace(r.String("DELTA"))

	ticks, ok := parseDeltaTime(strings.ToUpper(text))
	if !ok {
		return vmserrors.NewSegment(vmserrors.CLI_IVDTIME, text)
	}

	c.keepStatus()

	deadline := time.Now().Add(time.Duration(ticks) * (time.Second / time.Duration(vmsdef.TicksPerSecond)))

	for {
		left := time.Until(deadline)
		if left <= 0 {
			return nil
		}

		if c.Engine != nil && isAttentionKey(c.Engine.PendingAttention()) {
			return nil
		}

		time.Sleep(min(left, waitPoll))
	}
}
