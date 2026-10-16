package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 63, subtask 9 (docs/PHASE-63 - working sets.md): DCL's SET
// WORKING_SET changes the values of the process whose DCL runs it, within
// what the process was authorized, and SHOW WORKING_SET shows them, the
// default working set (DFWSCNT) as the limit.

// wsDispatcher returns a console and a routine that dispatches a line
// and returns its output, one "|"-joined string.
func wsDispatcher(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	return c, func(line string) string {
		t.Helper()

		out.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			out.WriteString("%" + err.Error() + "\n")
		}

		return strings.Join(programLines(out.String()), "|")
	}
}

// TestSetWorkingSet: each qualifier within and beyond its bound, /LOG and
// the message for a value cut back, /NOADJUST, and F$GETJPI's view of the
// result, in process 1 and in a subprocess, which has values of its own.
func TestSetWorkingSet(t *testing.T) {
	c, dispatch := wsDispatcher(t)
	p := c.ProcessOne().Process

	const (
		show = "  Working Set      /Limit=512   /Quota=1024    /Extent=16400|" +
			"  Adjustment enabled      Authorized Quota=1024  Authorized Extent=16400"
		spawned  = "%DCL-S-SPAWNED, process SYSTEM_1 spawned|%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1|"
		returned = "|%DCL-S-RETURNED, control returned to process SYSTEM"
	)

	for _, tt := range []struct{ line, want string }{
		{"SHOW WORKING_SET", show},
		// A quota above the authorized one is cut back, and says so.
		{"SET WORKING_SET/QUOTA=5000", "%SET-I-NEWLIMS, new working set:  Limit = 512  Quota = 1024  Extent = 16400"},
		// Within bounds, the message only with /LOG.
		{"SET WORKING_SET/QUOTA=800", ""},
		{"SET WORKING_SET/LIMIT=300/LOG", "%SET-I-NEWLIMS, new working set:  Limit = 300  Quota = 800  Extent = 16400"},
		// A limit above the quota raises the quota.
		{"SET WORKING_SET/LIMIT=900/LOG", "%SET-I-NEWLIMS, new working set:  Limit = 900  Quota = 900  Extent = 16400"},
		// An extent below the quota brings the quota and limit down.
		{"SET WORKING_SET/EXTENT=700/LOG", "%SET-I-NEWLIMS, new working set:  Limit = 700  Quota = 700  Extent = 700"},
		// An extent must be more than MINWSCNT (20).
		{"SET WORKING_SET/EXTENT=5", "%SET-I-NEWLIMS, new working set:  Limit = 21  Quota = 21  Extent = 21"},
		{"SET WORKING_SET/EXTENT=99999/QUOTA=600/LIMIT=250/NOADJUST", "%SET-I-NEWLIMS, new working set:  Limit = 250  Quota = 600  Extent = 16400"},
		{"SHOW WORKING_SET", "  Working Set      /Limit=250   /Quota=600    /Extent=16400|" +
			"  Adjustment disabled     Authorized Quota=1024  Authorized Extent=16400"},
		{`WRITE SYS$OUTPUT F$GETJPI("","DFWSCNT"), " ", F$GETJPI("","WSSIZE"), " ", F$GETJPI("","WSQUOTA")`, "250 250 600"},
		{"SET WORKING_SET/ADJUST", ""},
		// A subprocess's values are its own: its default, quota, and
		// extent are its creator's and 4 pages more, its authorized ones those.
		{"SPAWN SHOW WORKING_SET", spawned +
			"  Working Set      /Limit=254   /Quota=604    /Extent=16404|" +
			"  Adjustment enabled      Authorized Quota=604  Authorized Extent=16404" + returned},
		{"SPAWN SET WORKING_SET/QUOTA=2000", spawned +
			"%SET-I-NEWLIMS, new working set:  Limit = 254  Quota = 604  Extent = 16404" + returned},
	} {
		if got := dispatch(tt.line); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.line, got, tt.want)
		}
	}

	if p.WSDefault != 250 || p.WSQuota != 600 || p.WSAuth != 1024 || p.WSAdjustOff {
		t.Errorf("process 1 after the subprocess: DFWSCNT %d WSQUOTA %d WSAUTH %d NOADJUST %v",
			p.WSDefault, p.WSQuota, p.WSAuth, p.WSAdjustOff)
	}
}

// TestSetWorkingSetPaging: with paging on, the working set list follows
// the default SET WORKING_SET gives it.
func TestSetWorkingSetPaging(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "60")

	c, dispatch := wsDispatcher(t)
	env := c.ProcessOne()

	ws := env.WorkingSet()
	if ws == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	for _, tt := range []struct {
		line string
		want int
	}{
		{"SET WORKING_SET/LIMIT=40", 40},
		{"SET WORKING_SET/LIMIT=90", 90},
		{"SET WORKING_SET/EXTENT=30", 30},
	} {
		dispatch(tt.line)

		if ws.Size() != tt.want || env.Process.WSLimit != uint32(tt.want) {
			t.Errorf("%s: list size %d, WSLIMIT %d; want %d", tt.line, ws.Size(), env.Process.WSLimit, tt.want)
		}
	}

	checkWorkingSets(t, c)
}
