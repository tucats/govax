package console

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// Round 3 of Phase 93's probe on $CHKPRO and $CHECK_ACCESS (docs/planning/
// PHASE-93 - file protection.md, subtask 13a): SEC93.MAR's parts K
// ($CHKPRO), H ($CHECK_ACCESS), and Q ($CHKPRO with the caller's own
// rights and privileges), run as SYSTEM with every privilege and again
// without READALL, GRPPRV, SYSPRV, and BYPASS (with part M's cases of the
// two services), part U's as PROT93C, and part X's $CHKPRO cases, each
// held to VMS 7.1's report (vax/round3's sec93.log, sec93u.log, and
// sec93x_n.log).

// sec93CheckXCases are part X's $CHKPRO cases, each run alone as
// SEC93X.COM ran them.
var sec93CheckXCases = []int{569, 570, 571, 572}

// sec93CheckService reports whether a case is one of $CHKPRO's or
// $CHECK_ACCESS's.
func sec93CheckService(lines []string) bool {
	return len(lines) > 0 && (strings.Contains(lines[0], " $CHKPRO ") || strings.Contains(lines[0], " $CHECK_ACCESS "))
}

// sec93Passes splits sec93.log's program runs at its "PROGRAM, NO
// PRIVILEGES" section: the cases run with every privilege, and those run
// without.
func sec93Passes(t *testing.T) (privileged, unprivileged map[int][]string) {
	t.Helper()

	log := sec93Log(t, "round3/sec93.log")

	for i, l := range log {
		if strings.Contains(l, "PROGRAM, NO PRIVILEGES") {
			return sec93Cases(log[:i]), sec93Cases(log[i:])
		}
	}

	t.Fatal("sec93.log has no PROGRAM, NO PRIVILEGES section")

	return nil, nil
}

func TestSec93CheckOracle(t *testing.T) {
	c, run := sec93Console(t)
	image := sec93Build(t, c)
	privileged, unprivileged := sec93Passes(t)
	compared := 0

	compare := func(pass string, got, want map[int][]string) {
		t.Helper()

		for n, w := range want {
			if !sec93CheckService(w) {
				continue
			}

			compared++

			if g := got[n]; strings.Join(g, "\n") != strings.Join(w, "\n") {
				t.Errorf("%s case %d:\n got:\n%s\nwant:\n%s", pass, n, strings.Join(g, "\n"), strings.Join(w, "\n"))
			}
		}
	}

	runParts := func(parts string) map[int][]string {
		run("DEFINE SEC93_PART " + parts)
		defer run("DEASSIGN SEC93_PART")

		return sec93Cases(strings.Split(run(`RUN "`+image+`"`), "\n"))
	}

	// SYSTEM with every privilege: parts K, H, and Q.
	compare("privileged", runParts("KHQ"), privileged)

	// Part X, a case to an image.
	for _, n := range sec93CheckXCases {
		run(fmt.Sprintf("DEFINE SEC93_ONLY %d", n))
		got := sec93Cases(strings.Split(run(`RUN "`+image+`"`), "\n"))
		compare("X", got, sec93Cases(sec93Log(t, fmt.Sprintf("round3/sec93x_%d.log", n))))
	}

	run("DEASSIGN SEC93_ONLY")

	// SYSTEM without READALL, GRPPRV, SYSPRV, and BYPASS: parts Q and M.
	run("SET PROCESS/PRIVILEGES=(NOREADALL,NOGRPPRV,NOSYSPRV,NOBYPASS)")
	compare("unprivileged", runParts("QM"), unprivileged)

	// PROT93C, logged in, without privileges: part U.
	p := c.RTL.Process
	p.UIC, p.Username = uint32(security.NewUIC(0o300, 1)), "PROT93C"
	privileges := prot93Privilege("NETMBX") | prot93Privilege("TMPMBX")
	p.AuthorizedPrivileges = sec93Authorized | privileges
	p.ProcessPrivileges, p.CurrentPrivileges = privileges, privileges
	p.Rights = append(p.Rights, security.Identifier{Value: 0x80010001})

	compare("PROT93C", runParts("U"), sec93Cases(sec93Log(t, "round3/sec93u.log")))

	t.Logf("compared %d cases", compared)

	if compared < 130 {
		t.Errorf("compared %d cases, want parts K, H, Q, M's, U's, and X's", compared)
	}
}
