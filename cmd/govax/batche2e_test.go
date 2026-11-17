package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Batch queues end to end (docs/planning/PHASE-98 - batch queues.md,
// subtask 11): whole govax sessions on one system volume, authorization
// file, and queue database, as a site would use them. SYSTEM adds two
// accounts with ADDUSER.COM, makes a batch queue for group 200 beside
// SYS$BATCH, and has SYSTARTUP_VMS.COM start both; ALICE, in group 200,
// submits a nightly job that resubmits itself, a job that fails, and a job
// that can be restarted, and ends govax while it runs; BOB, in group 300,
// starts govax next, which restarts ALICE's job where it left off, and
// finds the group's queue closed to him.

// e2eProcedures are the procedures the accounts copy to their homes.
var e2eProcedures = map[string]string{
	// A job that runs every night: it submits itself again for the next
	// midnight, then does its work.
	"nightly.com": strings.Join([]string{
		`$ SUBMIT/NOPRINTER/QUEUE=GROUP200_BATCH/AFTER=TOMORROW/NAME=NIGHTLY 'F$ENVIRONMENT("PROCEDURE")'`,
		`$ WRITE SYS$OUTPUT "NIGHTLY RAN AS ", F$GETJPI("", "USERNAME"), " IN ", F$MODE()`,
	}, "\n"),

	// A job whose second command fails.
	"fails.com": strings.Join([]string{
		`$ WRITE SYS$OUTPUT "BEFORE THE ERROR"`,
		`$ COPY SYS$LOGIN:NOSUCH.DAT SYS$LOGIN:COPY.DAT`,
		`$ WRITE SYS$OUTPUT "NEVER SEEN"`,
	}, "\n"),

	// A job of two steps, restartable after the first: it marks that it
	// has got there and waits for govax to stop.
	"steps.com": strings.Join([]string{
		`$ IF $RESTART THEN GOTO 'BATCH$RESTART'`,
		`$ WRITE SYS$OUTPUT "STEP1"`,
		`$ SET RESTART_VALUE=STEP2`,
		`$ OPEN/WRITE MARK SYS$LOGIN:STEP1.MRK`,
		`$ CLOSE MARK`,
		`$ LOOP:`,
		`$ WAIT 0:0:0.1`,
		`$ GOTO LOOP`,
		`$ STEP2:`,
		`$ WRITE SYS$OUTPUT "STEP2 AFTER A RESTART"`,
		`$ OPEN/WRITE MARK SYS$LOGIN:STEP2.MRK`,
		`$ CLOSE MARK`,
	}, "\n"),

	// A plain job.
	"hello.com": `$ WRITE SYS$OUTPUT "HELLO FROM ", F$GETJPI("", "USERNAME")`,

	// AWAIT file: waits, a while at most, for a file to be there.
	"await.com": strings.Join([]string{
		`$ N = 0`,
		`$ AGAIN:`,
		`$ IF F$SEARCH(P1) .NES. "" THEN EXIT 1`,
		`$ N = N + 1`,
		`$ IF N .GT. 400 THEN EXIT %X10000002`,
		`$ WAIT 0:0:0.05`,
		`$ GOTO AGAIN`,
	}, "\n"),
}

// e2eSession runs one govax session, as user (SYSTEM if ""), typing lines,
// and returns its output.
func e2eSession(t *testing.T, user string, lines ...string) string {
	t.Helper()

	saved := loginUser
	loginUser = user

	defer func() { loginUser = saved }()

	out, err := session(t, strings.Join(lines, "\n")+"\n")
	if err != nil {
		t.Fatalf("%s's session: %v\n%s", user, err, out)
	}

	return strings.ReplaceAll(out, "\r", "")
}

// mustContain fails the test for each of wants out lacks.
func mustContain(t *testing.T, what, out string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("%s: no %q", what, want)
		}
	}

	if t.Failed() {
		t.Logf("%s:\n%s", what, out)
	}
}

// addUser is ADDUSER.COM's answers for a user in group, with no password,
// a plain user's privileges, and a home of the user's name.
func addUser(name, owner, group string) []string {
	return []string{"@SYS$MANAGER:ADDUSER", name, owner, group, "", "", "", "", "", ""}
}

// mustLack fails the test for each of unwanted out has.
func mustLack(t *testing.T, what, out string, unwanted ...string) {
	t.Helper()

	for _, u := range unwanted {
		if strings.Contains(out, u) {
			t.Errorf("%s: has %q\n%s", what, u, out)
		}
	}
}

// TestRun_batchEndToEnd is the scenario in this file's opening comment,
// five sessions of govax.
func TestRun_batchEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("runs batch jobs")
	}

	dir := t.TempDir()
	withAuthorizationFile(t, filepath.Join(dir, "sysuaf.json"))
	withSystemVolume(t, systemFlags{file: filepath.Join(dir, "govax-system.dsk")})
	withQueueFile(t, filepath.Join(dir, "qman.json"))

	copies := map[string]string{}

	for name, text := range e2eProcedures {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		copies[name] = `COPY "` + path + `"/HOST SYS$LOGIN:` + strings.ToUpper(name)
	}

	// The site's SYSTARTUP_VMS.COM: its batch queue block taken out of
	// comments, as the block's own comment says to, and GROUP200_BATCH
	// started there too.
	startup := builtin(t, "systartup_vms.com")
	head, block, _ := strings.Cut(startup, "$!  -- batch queue: begin --\n")
	block, tail, _ := strings.Cut(block, "$!  -- batch queue: end --\n")
	block = strings.ReplaceAll(block, "$! $", "$")
	block = strings.ReplaceAll(block, "$!      ", "     ")
	block = strings.Replace(block, "$ set on\n", "$ start/queue group200_batch\n$ set on\n", 1)
	putOnSystemVolume(t, "[SYSTEM.SYSMGR]SYSTARTUP_VMS.COM", head+block+tail)

	// SYSTEM: the accounts, by ADDUSER.COM, and the queues. GROUP200_BATCH
	// belongs to group 200, whose members may submit to it; the world has
	// no access to it.
	out := e2eSession(t, "", append(append(addUser("alice", "Alice Able", "200"), addUser("bob", "Bob Baker", "300")...),
		"START/QUEUE/MANAGER",
		"INITIALIZE/QUEUE/BATCH/JOB_LIMIT=3/START SYS$BATCH",
		"INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2/OWNER_UIC=[200,0]/PROTECTION=(S:M,O:M,G:S,W)/START GROUP200_BATCH",
		"SHOW QUEUE/FULL GROUP200_BATCH",
		"EXIT")...)
	mustContain(t, "SYSTEM's session", out,
		"Added ALICE, [200,1]", "Added BOB, [300,1]",
		"Batch queue GROUP200_BATCH, idle, on GOVAX::",
		"/OWNER=[200,0] /PROTECTION=(S:M,O:M,G:S,W)")

	// ALICE: a nightly job, its first run a second from now, which
	// submits itself for midnight; a job that fails, retained; and a job
	// of two steps still in its first when she ends govax.
	out = e2eSession(t, "ALICE",
		copies["nightly.com"], copies["fails.com"], copies["steps.com"], copies["await.com"],
		`SUBMIT/NOPRINTER/QUEUE=GROUP200_BATCH/AFTER="+0:00:01"/NAME=NIGHTLY NIGHTLY`,
		"SUBMIT/NOPRINTER/QUEUE=GROUP200_BATCH/RETAIN=ERROR/NOTIFY/NAME=FAILS FAILS",
		"SYNCHRONIZE/QUEUE=GROUP200_BATCH FAILS",
		"SHOW SYMBOL $STATUS",
		"SYNCHRONIZE/ENTRY=1",
		"SHOW SYMBOL $STATUS",
		"SHOW QUEUE/ALL GROUP200_BATCH",
		"TYPE SYS$LOGIN:NIGHTLY.LOG",
		"TYPE SYS$LOGIN:FAILS.LOG",
		"SUBMIT/NOPRINTER/QUEUE=GROUP200_BATCH/RESTART/NAME=STEPS STEPS",
		"@AWAIT SYS$LOGIN:STEP1.MRK",
		"SHOW SYMBOL $STATUS",
		"EXIT")
	mustContain(t, "ALICE's session", out,
		"Job NIGHTLY (queue GROUP200_BATCH, entry 1) holding until",
		"Job FAILS (queue GROUP200_BATCH, entry 2) started on GROUP200_BATCH",
		// /NOTIFY, and SYNCHRONIZE's status, the job's.
		"Job FAILS (queue GROUP200_BATCH, entry 2) terminated with error status\n%COPY-E-SEARCHFAIL, error searching for !AS\n",
		`$STATUS == "%X1067123A"`,
		// NIGHTLY's first run, and the run it submitted.
		`$STATUS == "%X10010001"`,
		"      3  NIGHTLY         ALICE                Holding until",
		"      2  FAILS           ALICE                Retained on error\n       %COPY-E-SEARCHFAIL, error searching for !AS\n",
		"Job NIGHTLY (queue GROUP200_BATCH, entry 3) holding until",
		"NIGHTLY RAN AS ALICE        IN BATCH\n",
		"BEFORE THE ERROR\n%COPY-E-SEARCHFAIL, error searching for SYS$SYSDEVICE:[ALICE]NOSUCH.DAT;\n-RMS-E-FNF, file not found\n",
		"Job STEPS (queue GROUP200_BATCH, entry 4) started on GROUP200_BATCH",
		`$STATUS == "%X00000001"`,
		"%VAX-W-JOBSEXECUTING, 1 batch job executing; ending govax ends it",
		"%VAX-I-JOBSENDED, batch jobs requeued to restart: 1; ended (queue reset): 0")
	mustLack(t, "ALICE's session", out, "NEVER SEEN", "NOSUCHJOB", "NOMSG")

	// ALICE again: the startup procedure starts the queues, STEPS goes on
	// from its second step, and NIGHTLY waits for midnight.
	out = e2eSession(t, "ALICE",
		copies["await.com"],
		"@AWAIT SYS$LOGIN:STEP2.MRK",
		"SHOW SYMBOL $STATUS",
		"SYNCHRONIZE/ENTRY=4",
		"SHOW QUEUE/ALL GROUP200_BATCH",
		"TYPE SYS$LOGIN:STEPS.LOG;*",
		"EXIT")
	mustContain(t, "ALICE's second session", out,
		`$STATUS == "%X00000001"`,
		"STEPS.LOG;2\n \nSTEP2 AFTER A RESTART\n  ALICE        job terminated at",
		"STEPS.LOG;1\n \nSTEP1\n",
		"      3  NIGHTLY         ALICE                Holding until",
		"      2  FAILS           ALICE                Retained on error\n")
	mustLack(t, "ALICE's second session", out, "STEPS           ALICE", "%DCL-", "%RMS-", "-F-")

	// BOB: the group's queue isn't his to use or to change, nor its jobs
	// his to see; SYS$BATCH is his to use.
	out = e2eSession(t, "BOB",
		copies["hello.com"],
		"SUBMIT/NOPRINTER/QUEUE=GROUP200_BATCH HELLO",
		"SHOW QUEUE/ALL GROUP200_BATCH",
		"DELETE/ENTRY=3",
		"SET ENTRY/HOLD 3",
		"SUBMIT/NOPRINTER HELLO",
		"SYNCHRONIZE HELLO",
		"SHOW SYMBOL $STATUS",
		"TYPE SYS$LOGIN:HELLO.LOG",
		"EXIT")
	mustContain(t, "BOB's session", out,
		"%SUBMIT-F-CREJOB, error creating job\n-JBC-E-NOPRIV, insufficient privilege or queue protection violation\n",
		"      3  no privilege                         Holding until",
		"      2  no privilege                         Retained on error\n",
		"%DELETE-E-NOTDELETED, error deleting 3\n-JBC-E-NOPRIV,",
		"Job HELLO (queue SYS$BATCH, entry 5) started on SYS$BATCH",
		`$STATUS == "%X10010001"`,
		"HELLO FROM BOB  ")
	mustLack(t, "BOB's session", out, "NIGHTLY", "FAILS")
}
