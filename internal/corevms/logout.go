package corevms

import (
	"fmt"
	"strings"
)

// LOGOUT's report (docs/PHASE-48.md): what DCL writes on SYS$OUTPUT when
// a job logs out. A job that isn't interactive (a $CREPRC of LOGINOUT
// reading a command file) gets the full report, laid out as VMS 7.3
// wrote it in testdata/mp/probe4/vax/probe4.log:
//
//	  SYSTEM       job terminated at  8-OCT-2026 12:17:12.53
//
//	  Accounting information:
//	  Buffered I/O count:              11         Peak working set size:     287
//	  Direct I/O count:                 7         Peak page file size:      2984
//	  Page faults:                    284         Mounted volumes:             0
//	  Charged CPU time:           0 00:00:00.01   Elapsed time:     0 00:00:00.01
//
// An interactive one gets one line, "  SYSTEM       logged out at
// <time>" (unconfirmed: no probe has logged out an interactive
// process). govax keeps no I/O counts and has no paging, and volumes are
// mounted for the whole system, so those counts are 0, as in the
// termination message (termmsg.go); the CPU time is the process's, and
// the elapsed time runs from its creation.

// LogoutReport is the report for env's job logging out now: the full
// one, or with interactive the one line.
func (env *Environment) LogoutReport(interactive bool) []string {
	now := env.Clock()
	date, _ := formatVMSTime(now, false)
	p := env.Process

	if interactive {
		return []string{fmt.Sprintf("  %-12s logged out at %s", p.Username, date)}
	}

	// counts is one line of two counts: the first ends in column 37, the
	// second in column 76.
	counts := func(left string, l int, right string, r int) string {
		return fmt.Sprintf("  %s%*d%9s%s%*d", left, 35-len(left), l, "", right, 30-len(right), r)
	}

	// delta is a time in VMS's delta format, without its padding.
	delta := func(t uint64) string {
		s, ok := formatVMSTime(uint64(-int64(t)), false)
		if !ok {
			return "****************"
		}

		return strings.TrimSpace(s)
	}

	elapsed := uint64(0)
	if now > p.LoginTime {
		elapsed = now - p.LoginTime
	}

	return []string{
		fmt.Sprintf("  %-12s job terminated at %s", p.Username, date),
		"",
		"  Accounting information:",
		counts("Buffered I/O count:", 0, "Peak working set size:", 0),
		counts("Direct I/O count:", 0, "Peak page file size:", 0),
		counts("Page faults:", 0, "Mounted volumes:", 0),
		fmt.Sprintf("  Charged CPU time:%24s   Elapsed time:%18s", delta(env.CPUTime(env)), delta(elapsed)),
	}
}
