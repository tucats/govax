package corevms

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/security"
)

// The probe's protection matrix (testdata/bat98, docs/planning/PHASE-98
// - batch queues.md, A.4): BAT98P.COM run as a batch job by each of
// three accounts against BAT98_P in six protection states, BAT98O once
// with no privilege and once per privilege. Each run tries 31 commands,
// "== PASS n" before each; this test reads VMS 7.1's logs and holds every
// case that turns on protection to the rule (security.CheckQueue and
// CheckJob, with OPER from the gate).

// bat98Accounts are the probe's accounts: UIC, rights, and whether the
// account is authorized for SYSPRV (which SUBMIT/USER's reading of the
// UAF turned on: an account that wasn't got INVQUAVAL with RMS$_PRV).
var bat98Accounts = map[string]struct {
	uic        security.UIC
	rights     []security.Identifier
	authSYSPRV bool
}{
	"bat98":  {security.NewUIC(0o200, 0o10), []security.Identifier{{Value: bat98ID}}, false},
	"bat98o": {security.NewUIC(0o200, 0o11), nil, true},
	"bat98w": {security.NewUIC(0o300, 0o10), nil, false},
}

// bat98ID is the identifier BAT98_ID's value here (any general one).
const bat98ID = 0x80010098

// bat98States are BAT98_P's states (BAT98.COM's MAKEP calls), each an
// owner, a code, and ACEs in the order SHOW SECURITY listed them.
var bat98States = map[string]struct {
	owner      security.UIC
	protection string
	acl        []string
}{
	"a": {security.SystemUIC, "(S:M,O:D,G:R,W:S)", nil},
	"b": {security.NewUIC(0o200, 0o10), "(S:M,O:D,G:R,W:S)", nil},
	"c": {security.NewUIC(0o200, 0o10), "(S,O,G,W)", nil},
	"d": {security.NewUIC(0o200, 0o10), "(S:M,O:D,G:R,W:M)", nil},
	"e": {security.NewUIC(0o200, 0o10), "(S,O,G,W)", []string{
		"(IDENTIFIER=[300,10],ACCESS=READ+SUBMIT+MANAGE+DELETE)",
		"(IDENTIFIER=BAT98_ID,ACCESS=READ+SUBMIT)",
	}},
	"f": {security.NewUIC(0o200, 0o10), "(S:M,O:R,G:D,W)", nil},
}

// bat98Names is the identifier the states' ACEs name.
type bat98Names struct{}

func (bat98Names) IdentifierValue(name string) (uint32, bool) {
	return bat98ID, strings.EqualFold(name, "BAT98_ID")
}

func (bat98Names) IdentifierName(uint32) (string, bool) { return "", false }

// bat98Pass is one pass of BAT98P.COM: each case's outcome, the messages'
// identifiers ("NOPRIV", "-NOPRIV"), "np" for a job shown as "no
// privilege", "nosec" for "Access to security information denied.",
// "job" for a job's line shown, "noowner" for F$GETQUI's owner shown as
// [0,0] (VMS's answer when the security may not be seen), "noobj" for
// the procedure's "NO OWN".
type bat98Pass map[int][]string

func (p bat98Pass) has(n int, token string) bool { return slices.Contains(p[n], token) }

// readBat98Log reads one account's run in one state: its passes by name.
func readBat98Log(t *testing.T, path string) map[string]bat98Pass {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	passRE := regexp.MustCompile(`^######## PASS (\S+)`)
	caseRE := regexp.MustCompile(`^== (\S+) (\d+)$`)
	msgRE := regexp.MustCompile(`^(%|-)\w+-\w-(\w+)`)
	jobRE := regexp.MustCompile(`^\s+\d+\s+\S`)

	passes := map[string]bat98Pass{}

	var cur bat98Pass

	n := 0

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimRight(scan.Text(), " ")

		switch m := passRE.FindStringSubmatch(line); {
		case m != nil:
			cur, n = bat98Pass{}, 0
			passes[m[1]] = cur

			continue
		case strings.HasPrefix(line, "BAT98P FINISHED"):
			cur = nil
		}

		if m := caseRE.FindStringSubmatch(line); m != nil && cur != nil {
			n, _ = strconv.Atoi(m[2])
			cur[n] = []string{}

			continue
		}

		if cur == nil || n == 0 {
			continue
		}

		switch m := msgRE.FindStringSubmatch(line); {
		case m != nil && m[1] == "%":
			cur[n] = append(cur[n], m[2])
		case m != nil:
			cur[n] = append(cur[n], "-"+m[2])
		case strings.Contains(line, "no privilege"):
			cur[n] = append(cur[n], "np")
		case strings.Contains(line, "Access to security information denied"):
			cur[n] = append(cur[n], "nosec")
		case strings.Contains(line, "GETQUI OWNER [0,0]"):
			cur[n] = append(cur[n], "noowner")
		case strings.HasPrefix(strings.TrimSpace(line), "NO "):
			cur[n] = append(cur[n], "noobj")
		case jobRE.MatchString(line):
			cur[n] = append(cur[n], "job")
		}
	}

	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}

	return passes
}

// passPrivileges is a pass's privileges: TMPMBX and NETMBX, which every
// account held, and the pass's own ("CMKRNL+SYSPRV").
func passPrivileges(t *testing.T, pass string) uint64 {
	var mask uint64

	for _, name := range append([]string{"TMPMBX", "NETMBX"}, strings.Split(pass, "+")...) {
		if name == "NONE" {
			continue
		}

		m, ok := PrivilegeMask(name)
		if !ok {
			t.Fatalf("no privilege %s", name)
		}

		mask |= m
	}

	return mask
}

func TestBat98ProtectionMatrix(t *testing.T) {
	logs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "bat98", "vax", "bat98p_*.log"))
	if err != nil || len(logs) != 18 {
		t.Fatalf("want the 18 matrix logs, found %d (%v)", len(logs), err)
	}

	nameRE := regexp.MustCompile(`bat98p_(\w)_(\w+)\.log$`)
	checked := 0

	for _, path := range logs {
		m := nameRE.FindStringSubmatch(path)
		state, account := bat98States[m[1]], bat98Accounts[m[2]]

		protection, err := jobqueue.ParseProtection(state.protection, jobqueue.Protection(security.NoAccess))
		if err != nil {
			t.Fatal(err)
		}

		queue := &jobqueue.Queue{Name: "BAT98_P", Owner: jobqueue.UIC(state.owner), Protection: protection}

		for _, text := range state.acl {
			ace, err := security.ParseACE(text, bat98Names{}, security.QueueAccessNames)
			if err != nil {
				t.Fatalf("%s: %v", text, err)
			}

			queue.ACL = append(queue.ACL, jobqueue.ACE(ace))
		}

		systemJob := &jobqueue.Job{Owner: jobqueue.UIC(security.SystemUIC)}
		ownJob := &jobqueue.Job{Owner: jobqueue.UIC(account.uic)}
		otherJob := &jobqueue.Job{Owner: jobqueue.UIC(bat98Accounts["bat98w"].uic)}

		for pass, got := range readBat98Log(t, path) {
			where := fmt.Sprintf("state %s, %s, pass %s", m[1], m[2], pass)
			privileges := passPrivileges(t, pass)
			env := &Environment{System: &System{}, Process: &Process{UIC: uint32(account.uic), CurrentPrivileges: privileges, Rights: account.rights}}

			// expect holds case n to the rule's answer: allowed says
			// VMS let the command do it, which its log shows as want.
			expect := func(n int, allowed bool, what string, refused func(bat98Pass) bool) {
				t.Helper()

				checked++

				if allowed == refused(got) {
					t.Errorf("%s, case %d (%s): the rule says allowed=%v, VMS's log has %v", where, n, what, allowed, got[n])
				}
			}

			has := func(n int, token string) func(bat98Pass) bool {
				return func(p bat98Pass) bool { return p.has(n, token) }
			}

			visible := env.QueueSecurityVisible(queue)
			expect(2, visible, "SHOW QUEUE/FULL's security", has(2, "nosec"))
			expect(6, visible, "F$GETQUI's OWNER_UIC", has(6, "noowner"))
			expect(24, visible, "SHOW SECURITY", has(24, "NOPRIV"))

			expect(3, env.JobAccess(queue, systemJob, security.QueueRead), "SHOW ENTRY of SYSTEM's job", has(3, "np"))
			expect(4, env.JobAccess(queue, systemJob, security.QueueRead), "SHOW ENTRY/FULL of SYSTEM's job", has(4, "np"))
			expect(15, env.JobAccess(queue, systemJob, security.QueueRead), "SYNCHRONIZE with SYSTEM's job", has(15, "NOPRIV"))
			expect(14, env.JobAccess(queue, systemJob, security.QueueDelete), "SET ENTRY of SYSTEM's job", has(14, "NOPRIV"))
			expect(30, env.JobAccess(queue, systemJob, security.QueueDelete), "DELETE/ENTRY of SYSTEM's job", has(30, "-NOPRIV"))

			submit := env.QueueAccess(queue, security.QueueSubmit)
			expect(7, submit, "SUBMIT", has(7, "-NOPRIV"))
			expect(10, submit, "SUBMIT/PRIORITY", has(10, "-NOPRIV"))

			if submit {
				expect(8, env.JobAccess(queue, ownJob, security.QueueRead), "SHOW ENTRY of its own job", has(8, "np"))
				expect(9, env.JobAccess(queue, ownJob, security.QueueDelete), "SET ENTRY of its own job", has(9, "NOPRIV"))
				expect(11, env.JobAccess(queue, ownJob, security.QueueDelete), "DELETE/ENTRY of its own job", has(11, "-NOPRIV"))
				expect(29, env.JobAccess(queue, ownJob, security.QueueDelete), "DELETE/ENTRY of its own job", has(29, "-NOPRIV"))
			} else if !got.has(8, "noobj") {
				t.Errorf("%s: SUBMIT was refused, but the log has a job of its own: %v", where, got[8])
			}

			// SUBMIT/USER=BAT98W: past the UAF (authorized SYSPRV) and
			// CMKRNL, the queue's submit access is BAT98W's own, not the
			// submitter's: in state E only BAT98W's ACE let it in, and in
			// state F the submitter's SYSPRV didn't.
			cmkrnl, _ := PrivilegeMask("CMKRNL")
			if account.authSYSPRV && privileges&cmkrnl != 0 {
				target := security.Credentials{UIC: bat98Accounts["bat98w"].uic}
				expect(12, security.CheckQueue(target, queue.Profile(), security.QueueSubmit).Granted, "SUBMIT/USER", has(12, "-NOPRIV"))

				if !got.has(12, "-NOPRIV") {
					expect(13, env.JobAccess(queue, otherJob, security.QueueDelete), "DELETE/ENTRY of BAT98W's job", has(13, "-NOPRIV"))
				}
			}

			manage := env.QueueAccess(queue, security.QueueManage)
			for _, n := range []int{17, 18, 19, 20, 25, 26, 27} {
				expect(n, manage, "managing the queue", has(n, "NOPRIV"))
			}

			expect(28, manage, "DELETE/QUEUE", has(28, "-NOPRIV"))
			expect(21, env.MayCreateQueue() == 0, "INITIALIZE/QUEUE of a new queue", has(21, "NOOPER"))
			expect(23, env.QueueControl(queue), "SET SECURITY", has(23, "NOPRIV"))
		}
	}

	t.Logf("%d cases held to VMS 7.1's logs", checked)
}
