//go:build ignore

// gen writes Phase 98's probe of batch queues (README.md;
// docs/planning/PHASE-98 - batch queues.md, Part D). It asks VMS 7.1:
//
//   - every queue command's words: INITIALIZE/QUEUE, START, STOP, SET,
//     DELETE, and SHOW QUEUE, SUBMIT with each qualifier govax will keep,
//     SHOW ENTRY, SET ENTRY, DELETE/ENTRY, SYNCHRONIZE, and their errors
//     (bat98.com, SYSTEM's run, on queues of its own, BAT98_*);
//   - what a batch job is: its process, rights, names, parameters, log
//     file, and how it ends (bat98j.com, run as a batch job by SYSTEM and
//     each probe account; the small procedures each job runs);
//   - who may do what: bat98p.com, run as a batch job by each probe
//     account on a queue in each of six protection states, BAT98O once
//     per privilege;
//   - the services: bat98q.mar calls $SNDJBC and $GETQUI directly and
//     lists the JBC facility's messages, and def_*.mar give the SJC$,
//     QUI$, and JBC$ values (decode.go turns VMS's objects into
//     defined.txt).
//
// Every wait in it is bounded: a job SYSTEM waits for ends by itself, and
// waits for a job to start look for a marker file a limited number of
// times. Run it from the repository root:
//
//	go run testdata/bat98/gen.go
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/bat98"

// The exchange volume's places, through the system logical name
// BAT98$DISK that bat98.com defines.
const (
	password = "BAT98PW"
	disk     = "BAT98$DISK:"
	top      = disk + "[000000]"
	logs     = disk + "[BAT98LOG]"
	scratch  = disk + "[BAT98SCR]"
)

// files are the procedures and sources exchange.cmd copies to the volume's
// [000000]; each account's LOGIN.COM goes to its home.
var files = []string{
	"bat98.com", "bat98acct.com", "bat98drop.com", "bat98z.com",
	"bat98j.com", "bat98p.com", "bat98ok.com", "bat98wrn.com", "bat98err.com",
	"bat98fat.com", "bat98bad.com", "bat98w.com", "bat98c.com", "bat98o.com",
	"bat98r.com", "bat98q.mar", "def_sjc.mar", "def_qui.mar", "def_jbc.mar",
}

// accounts are the probe's accounts, made in the VAX's own SYSUAF by
// bat98acct.com and removed by bat98drop.com.
var accounts = []struct{ name, uic, home string }{
	{"BAT98", "[200,10]", "[BAT98]"},
	{"BAT98O", "[200,11]", "[BAT98O]"},
	{"BAT98W", "[300,10]", "[BAT98W]"},
}

// jobLogs are the log files the run leaves in [BAT98LOG], for copyout.cmd.
var jobLogs []string

// ---------------------------------------------------------------------
// A procedure being written.

type dcl struct {
	b strings.Builder
	n int // the last case's number
}

func (d *dcl) raw(s string) { d.b.WriteString(s) }

// line writes one command line.
func (d *dcl) line(format string, a ...any) { fmt.Fprintf(&d.b, "$ "+format+"\n", a...) }

func (d *dcl) section(title string) {
	d.line("!")
	d.line(`WRITE SYS$OUTPUT "######## %s"`, title)
}

// mark numbers the next case in the log.
func (d *dcl) mark() {
	d.n++
	d.line(`WRITE SYS$OUTPUT "== %d"`, d.n)
}

// c is a case: a command and the $STATUS it leaves.
func (d *dcl) c(command string) {
	d.mark()
	d.line("%s", command)
	d.line("SHOW SYMBOL $STATUS")
}

// submit is a SUBMIT case whose entry number, if it made one, is left in
// the symbol sym ("" if it made none).
func (d *dcl) submit(sym, args string) {
	d.mark()
	d.line(`IF F$TYPE($ENTRY) .NES. "" THEN DELETE/SYMBOL/LOCAL $ENTRY`)
	d.line("SUBMIT%s", args)
	d.line("SHOW SYMBOL $STATUS")
	d.line(`%s = ""`, sym)
	d.line(`IF F$TYPE($ENTRY) .NES. "" THEN %s = $ENTRY`, sym)
	d.line(`WRITE SYS$OUTPUT "  %s = ", %s`, sym, sym)
}

// sync waits for sym's job to end, and shows the status SYNCHRONIZE gives.
func (d *dcl) sync(sym string) {
	d.mark()
	d.line(`IF %s .NES. "" THEN SYNCHRONIZE/ENTRY='%s'`, sym, sym)
	d.line("SHOW SYMBOL $STATUS")
}

// job is a job on BAT98_A with its own log in [BAT98LOG], named name: the
// SUBMIT, the wait, and the log.
func (d *dcl) job(sym, name, quals, files string) {
	d.submit(sym, fmt.Sprintf("/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=%s/LOG_FILE=%s%s.LOG%s %s",
		name, logs, name, quals, files))
	d.sync(sym)
	d.showLog(name)
}

// started is a job SYSTEM waits to see running: the SUBMIT and the wait
// for its marker. The job is BAT98W.COM, which ends by itself after
// seconds.
func (d *dcl) started(sym, name, quals string, seconds int) {
	d.line("DELETE %s%s.RUN;*", scratch, name)
	d.submit(sym, fmt.Sprintf("/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=%s/LOG_FILE=%s%s.LOG%s/PARAMETERS=(%s,%d) %sBAT98W",
		name, logs, name, quals, name, seconds, top))
	d.line("CALL AWAIT %s '%s'", name, sym)
}

func (d *dcl) showLog(name string) {
	jobLogs = append(jobLogs, name+".LOG")
	d.line("DIRECTORY/SIZE/OWNER/PROTECTION %s%s.LOG;*", logs, name)
	d.line("TYPE %s%s.LOG;*", logs, name)
}

// files are the job procedures, by short name.
func job(names ...string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = top + "BAT98" + n
	}

	return strings.Join(parts, ",")
}

// ---------------------------------------------------------------------
// bat98.com, SYSTEM's run.

func systemProc() string {
	d := &dcl{}

	d.raw(`$ ! BAT98.COM - Phase 98's probe of batch queues: SYSTEM's run.
$ ! Written by testdata/bat98/gen.go. Run it as SYSTEM, from the exchange
$ ! volume, mounted /SYSTEM, with the queue manager running (README.md):
$ !
$ !     @DUA1:[000000]BAT98
$ !
$ ! It defines BAT98$DISK in the system table for the volume, makes the
$ ! probe's accounts (BAT98ACCT.COM), and works on queues of its own,
$ ! BAT98_*; it never changes the VAX's own queues. Its log is
$ ! BAT98$DISK:[BAT98LOG]BAT98.LOG.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK 'DEV'
$ LOGFILE = DEV + "[BAT98LOG]BAT98.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "BAT98 finished; the log is ", LOGFILE
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME"), " ", F$GETSYI("NODENAME")
$ WRITE SYS$OUTPUT F$GETJPI("","USERNAME"), " ", F$GETJPI("","UIC"), " ", F$MODE()
$ MCR SYSGEN
USE ACTIVE
SHOW MAXQUEPRI
SHOW DEFQUEPRI
SHOW DEFPRI
EXIT
$ SHOW QUEUE/MANAGER/FULL
$ SHOW QUEUE/ALL/FULL
$ SHOW LOGICAL/FULL SYS$BATCH
$ SHOW LOGICAL/FULL SYS$PRINT
$ X = F$GETQUI("CANCEL_OPERATION")
$ PRINTQ = ""
$ PRINTQ = F$GETQUI("DISPLAY_QUEUE","QUEUE_NAME","SYS$PRINT")
$ SHOW SYMBOL PRINTQ
$ !
$ WRITE SYS$OUTPUT "######## VOLUME"
$ SET VOLUME/PROTECTION=(S:RWCD,O:RWCD,G:RWCD,W:RWCD) BAT98$DISK:
$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) BAT98$DISK:[000000]000000.DIR
$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) BAT98$DISK:[000000]BAT98LOG.DIR
$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) BAT98$DISK:[000000]BAT98SCR.DIR
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) BAT98$DISK:[000000]*.COM;*
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) BAT98$DISK:[000000]*.MAR;*
$ DIRECTORY/SECURITY BAT98$DISK:[000000]
$ !
$ @BAT98$DISK:[000000]BAT98ACCT
$ @BAT98$DISK:[000000]BAT98Z
$ SET DEFAULT BAT98$DISK:[BAT98SCR]
`)

	queueCases(d)
	stoppedJobs(d)
	runningJobs(d)
	logCases(d)
	orderCases(d)
	restartCases(d)
	viewCases(d)
	serviceCases(d)
	protectionCases(d)

	lastCase = d.n

	d.raw(`$ !
$ WRITE SYS$OUTPUT "######## AT THE END"
$ SHOW QUEUE/FULL/ALL BAT98*
$ SHOW QUEUE/MANAGER/FULL
$ DIRECTORY/SIZE/OWNER ` + logs + `
$ DIRECTORY/SIZE/OWNER ` + scratch + `
$ WRITE SYS$OUTPUT "BAT98 FINISHED"
$ EXIT
`)
	d.raw(subroutines())

	return d.b.String()
}

// subroutines are the CALL targets BAT98.COM and BAT98B.COM share.
func subroutines() string {
	return `$ !
$ ! AWAIT name entry: wait until the job called name, queued as entry,
$ ! has written its marker, [BAT98SCR]name.RUN, looking 30 times, 2
$ ! seconds apart; at once if the SUBMIT made no entry.
$ AWAIT: SUBROUTINE
$ IF P2 .EQS. "" THEN WRITE SYS$OUTPUT "  ", P1, " WAS NOT QUEUED"
$ IF P2 .EQS. "" THEN EXIT
$ N = 0
$ AWAIT_LOOP:
$ N = N + 1
$ FOUND = F$SEARCH("` + scratch + `''P1'.RUN",N)
$ IF FOUND .NES. "" THEN GOTO AWAIT_DONE
$ IF N .GE. 30 THEN GOTO AWAIT_DONE
$ WAIT 0:0:2
$ GOTO AWAIT_LOOP
$ AWAIT_DONE:
$ IF FOUND .EQS. "" THEN WRITE SYS$OUTPUT "  ", P1, " NEVER STARTED"
$ IF FOUND .NES. "" THEN WRITE SYS$OUTPUT "  ", P1, " STARTED"
$ ENDSUBROUTINE
$ !
$ ! FINDLOG name: where a job's log went, looking in every directory a
$ ! default could have put name.* in; each one found is shown, typed, and
$ ! deleted.
$ FINDLOG: SUBROUTINE
$ DIRS = "SYS$LOGIN:,SYS$MANAGER:,` + top + `,` + scratch + `,` + logs + `,` + disk + `[BAT98],` + disk + `[BAT98W]"
$ FOUND = ""
$ I = 0
$ FL_DIR:
$ D = F$ELEMENT(I,",",DIRS)
$ IF D .EQS. "," THEN GOTO FL_SHOW
$ FL_FILE:
$ F = F$SEARCH(D + P1 + ".*;*",40)
$ IF F .EQS. "" THEN GOTO FL_NEXT
$ IF F$LOCATE(F,FOUND) .LT. F$LENGTH(FOUND) THEN GOTO FL_FILE
$ FOUND = FOUND + F + ","
$ GOTO FL_FILE
$ FL_NEXT:
$ I = I + 1
$ GOTO FL_DIR
$ FL_SHOW:
$ WRITE SYS$OUTPUT "  LOGS FOUND: ", FOUND
$ I = 0
$ FL_EACH:
$ F = F$ELEMENT(I,",",FOUND)
$ IF F .EQS. "" .OR. F .EQS. "," THEN GOTO FL_END
$ DIRECTORY/SIZE/OWNER/PROTECTION 'F'
$ TYPE 'F'
$ DELETE 'F'
$ I = I + 1
$ GOTO FL_EACH
$ FL_END:
$ ENDSUBROUTINE
$ !
$ ! MAKEP state owner protection acl1 acl2 n: BAT98_P made again for one
$ ! account's run of BAT98P.COM, started, in the state's protection, with a
$ ! job retained in it (RET) and n held jobs of SYSTEM's (HELD, their entries
$ ! with commas between), and n stopped queues BAT98_D1 to BAT98_Dn in the
$ ! same protection, one for each pass to try to delete.
$ MAKEP: SUBROUTINE
$ @BAT98$DISK:[000000]BAT98Z BAT98_P
$ INITIALIZE/QUEUE/BATCH/JOB_LIMIT=1/OWNER_UIC='P2'/PROTECTION='P3'/START BAT98_P
$ IF P4 .NES. "" THEN SET SECURITY/CLASS=QUEUE/ACL='P4' BAT98_P
$ IF P5 .NES. "" THEN SET SECURITY/CLASS=QUEUE/ACL='P5' BAT98_P
$ SHOW SECURITY/CLASS=QUEUE BAT98_P
$ RET == ""
$ SUBMIT/QUEUE=BAT98_P/RETAIN=ALWAYS/NOPRINTER/NOLOG_FILE/NAME=SYSRET ` + top + `BAT98OK
$ IF F$TYPE($ENTRY) .NES. "" THEN RET == $ENTRY
$ IF RET .NES. "" THEN SYNCHRONIZE/ENTRY='RET'
$ HELD == ""
$ LAST = RET
$ I = 1
$ MP_LOOP:
$ IF I .GT. F$INTEGER(P6) THEN GOTO MP_DONE
$ SUBMIT/QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/NAME=SYSHELD ` + top + `BAT98OK
$ E = ""
$ IF F$TYPE($ENTRY) .NES. "" THEN E = $ENTRY
$ IF E .EQS. LAST THEN E = ""
$ IF E .NES. "" THEN LAST = E
$ HELD == HELD + "," + E
$ @BAT98$DISK:[000000]BAT98Z BAT98_D'I'
$ INITIALIZE/QUEUE/BATCH/OWNER_UIC='P2'/PROTECTION='P3' BAT98_D'I'
$ IF P4 .NES. "" THEN SET SECURITY/CLASS=QUEUE/ACL='P4' BAT98_D'I'
$ IF P5 .NES. "" THEN SET SECURITY/CLASS=QUEUE/ACL='P5' BAT98_D'I'
$ I = I + 1
$ GOTO MP_LOOP
$ MP_DONE:
$ HELD == F$EXTRACT(1,999,HELD)
$ SHOW SYMBOL RET
$ SHOW SYMBOL HELD
$ SHOW QUEUE/FULL/ALL BAT98_P
$ ENDSUBROUTINE
$ !
$ ! RELEASE entry: SET ENTRY/RELEASE until it takes, 15 times 2 seconds
$ ! apart (a job requeued /HOLD while it runs is refused, JBC-E-EXECUTING,
$ ! until its run has ended and it holds), and the job deleted if it never
$ ! takes, so that nothing waits on a held job. Round 1 waited on one.
$ RELEASE: SUBROUTINE
$ IF P1 .EQS. "" THEN EXIT
$ N = 0
$ RL_LOOP:
$ N = N + 1
$ SET ENTRY 'P1'/RELEASE
$ S = $STATUS
$ WRITE SYS$OUTPUT "  RELEASE ", N, " ", S
$ IF S THEN GOTO RL_DONE
$ IF N .GE. 15 THEN GOTO RL_GIVE_UP
$ WAIT 0:0:2
$ GOTO RL_LOOP
$ RL_GIVE_UP:
$ DELETE/ENTRY='P1'
$ RL_DONE:
$ ENDSUBROUTINE
`
}

// queueCases are the queue commands, on queues that never run a job.
func queueCases(d *dcl) {
	d.section("QUEUES")
	d.c("INITIALIZE/QUEUE/BATCH BAT98_A")
	d.c("SHOW QUEUE BAT98_A")
	d.c("SHOW QUEUE/FULL BAT98_A")
	d.c(`INITIALIZE/QUEUE/BATCH/JOB_LIMIT=3/BASE_PRIORITY=3/DESCRIPTION="GOVAX PROBE A" BAT98_A`)
	d.c("SHOW QUEUE/FULL BAT98_A")
	d.c("INITIALIZE/QUEUE/BATCH/WSDEFAULT=200/WSQUOTA=400/WSEXTENT=800 BAT98_A")
	d.c("INITIALIZE/QUEUE/BATCH/CPUDEFAULT=0:05:00/CPUMAXIMUM=0:10:00/RETAIN=ERROR BAT98_A")
	d.c("SHOW QUEUE/FULL BAT98_A")
	d.c("SHOW SECURITY/CLASS=QUEUE BAT98_A")

	d.c("INITIALIZE/QUEUE/BATCH/START BAT98_B")
	d.c("SHOW QUEUE BAT98_B")
	d.c("SHOW QUEUE/FULL BAT98_B")
	d.c("INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2 BAT98_B")
	d.c("SHOW QUEUE/FULL BAT98_B")
	d.c("STOP/QUEUE BAT98_B")
	d.c("SHOW QUEUE BAT98_B")
	d.c("STOP/QUEUE BAT98_B")
	d.c("START/QUEUE BAT98_B")
	d.c("START/QUEUE BAT98_B")
	d.c("STOP/QUEUE/NEXT BAT98_B")
	d.c("SHOW QUEUE/FULL BAT98_B")
	d.c("START/QUEUE BAT98_B")
	d.c("STOP/QUEUE/RESET BAT98_B")
	d.c("SHOW QUEUE/FULL BAT98_B")
	d.c("START/QUEUE/JOB_LIMIT=2/BASE_PRIORITY=5 BAT98_B")
	d.c("SHOW QUEUE/FULL BAT98_B")
	d.c(`SET QUEUE/JOB_LIMIT=4/DESCRIPTION="B CHANGED" BAT98_B`)
	d.c("SET QUEUE/CPUDEFAULT=0:01:00/WSQUOTA=300 BAT98_B")
	d.c("SHOW QUEUE/FULL BAT98_B")
	d.c("DELETE/QUEUE BAT98_B")
	d.c("STOP/QUEUE/NEXT BAT98_B")
	d.c("SHOW QUEUE/FULL BAT98_B")

	d.c("INITIALIZE/QUEUE/BATCH/OWNER_UIC=[200,10]/PROTECTION=(S:M,O:D,G:R,W) BAT98_D")
	d.c("SHOW QUEUE/FULL BAT98_D")
	d.c("SHOW SECURITY/CLASS=QUEUE BAT98_D")
	d.c("SET SECURITY/CLASS=QUEUE/PROTECTION=(W:RS) BAT98_D")
	d.c("SET SECURITY/CLASS=QUEUE/ACL=(IDENTIFIER=BAT98_ID,ACCESS=READ+SUBMIT) BAT98_D")
	d.c("SHOW SECURITY/CLASS=QUEUE BAT98_D")
	d.c("SET QUEUE/PROTECTION=(S:M,O:M,G:M,W:M)/OWNER_UIC=[1,4] BAT98_D")
	d.c("SHOW QUEUE/FULL BAT98_D")
	d.c("SET SECURITY/CLASS=QUEUE/ACL/DELETE BAT98_D")
	d.c("SHOW SECURITY/CLASS=QUEUE BAT98_D")
	d.c("DELETE/QUEUE BAT98_D")
	d.c("SHOW QUEUE BAT98_D")
	d.c("DELETE/QUEUE BAT98_D")

	d.c("INITIALIZE/QUEUE BAT98_E")
	d.c("SHOW QUEUE/FULL BAT98_E")
	d.c("DELETE/QUEUE BAT98_E")
	d.c("INITIALIZE/QUEUE/BATCH BAT98_QUEUE_NAME_OF_32_CHARACTER")
	d.c("INITIALIZE/QUEUE/BATCH BAD-NAME")
	d.c("INITIALIZE/QUEUE/BATCH/JOB_LIMIT=0 BAT98_E")
	d.c("INITIALIZE/QUEUE/BATCH/JOB_LIMIT=256 BAT98_E")
	d.c("INITIALIZE/QUEUE/BATCH/BASE_PRIORITY=99 BAT98_E")
	d.c("INITIALIZE/QUEUE/BATCH/PROTECTION=(S,O,G,W) BAT98_E")
	d.c("SHOW QUEUE/FULL BAT98_E")
	d.c("DELETE/QUEUE BAT98_E")
	d.c("START/QUEUE NOSUCHQ98")
	d.c("STOP/QUEUE NOSUCHQ98")
	d.c("DELETE/QUEUE NOSUCHQ98")
	d.c("SET QUEUE/JOB_LIMIT=2 NOSUCHQ98")
	d.c("SHOW QUEUE NOSUCHQ98")
	d.c("INITIALIZE/QUEUE/BATCH/JOB_LIMIT=1 BAT98_P")
	d.c("SHOW QUEUE BAT98*")
	d.c("SHOW QUEUE/BATCH BAT98*")
	d.c("SHOW QUEUE/SUMMARY BAT98*")
	d.c("SHOW QUEUE/FULL/ALL BAT98*")
	d.c("SHOW QUEUE/MANAGER")
	d.c("SHOW QUEUE/MANAGER/FULL")
}

// stoppedJobs are SUBMIT's qualifiers and the job commands, on BAT98_B,
// stopped, so nothing runs.
func stoppedJobs(d *dcl) {
	d.section("JOBS ON A STOPPED QUEUE")
	d.line(`LONG = F$FAO("!260*X")`)
	d.line(`LATER = F$CVTIME("+1-00:00","ABSOLUTE")`)
	d.line("SHOW SYMBOL LATER")

	b := "/QUEUE=BAT98_B/NOPRINTER"
	ok := top + "BAT98OK"

	d.submit("S1", b+" "+ok)
	d.c("SHOW QUEUE BAT98_B")
	d.c("SHOW ENTRY 'S1'")
	d.c("SHOW ENTRY/FULL 'S1'")
	d.submit("S2", b+"/HOLD/NAME=HELD98 "+ok)
	d.submit("S3", b+`/AFTER="+0:30" `+ok)
	d.submit("S4", b+"/AFTER=TOMORROW "+ok)
	d.submit("S5", b+`/AFTER="''LATER'" `+ok)
	d.submit("S6", b+"/AFTER=TODAY "+ok)
	d.submit("S7", b+`/AFTER="1-JAN-2000 00:00" `+ok)
	d.submit("S8", b+`/AFTER="TOMORROW+12:00" `+ok)
	d.submit("S9", b+`/HOLD/AFTER="+1:00" `+ok)
	d.submit("S10", b+`/PARAMETERS=(ALPHA,"two words",3,"",E,F,G,"H h") `+ok)
	d.submit("S11", b+"/PARAMETERS=(A,B,C,D,E,F,G,H,I) "+ok)
	d.submit("S12", b+"/PARAMETERS=('LONG') "+ok)
	d.submit("S13", b+"/PARAMETERS=lower "+ok)
	d.submit("S14", b+`/NAME=NAMED98/NOTE="A NOTE"/PRIORITY=50 `+ok)
	d.submit("S15", b+"/PRIORITY=255 "+ok)
	d.submit("S16", b+"/CPUTIME=0:01:00/WSDEFAULT=150/WSQUOTA=300/WSEXTENT=600 "+ok)
	d.submit("S17", b+"/CPUTIME=NONE "+ok)
	d.submit("S18", b+"/CPUTIME=INFINITE "+ok)
	d.submit("S19", "/QUEUE=BAT98_B/PRINTER=NOSUCHQ98 "+ok)
	d.submit("S20", b+"/NOTIFY/RESTART/RETAIN=ALWAYS "+ok)
	d.submit("S21", b+"/RETAIN=UNTIL=TOMORROW "+ok)
	d.submit("S22", b+"/RETAIN=ERROR/KEEP "+ok)
	d.submit("S23", b+"/USER=BAT98 "+ok)
	d.submit("S24", b+"/USER=NOSUCH98 "+ok)
	d.submit("S25", b+"/USER=BAT98W/LOG_FILE="+logs+"S25.LOG "+ok)
	d.submit("S26", b+"/NOLOG_FILE "+ok)
	d.submit("S27", b+" "+job("OK", "WRN"))
	d.submit("S28", b+" "+top+"BAT98OK+"+top+"BAT98WRN")
	d.submit("S29", b+" "+top+"BAT98OK.COM;*")
	d.submit("S30", b+" "+top+"NOSUCH98.COM")
	d.submit("S31", b+"/NOIDENTIFY "+ok)
	d.submit("S32", "/QUEUE=NOSUCHQ98/NOPRINTER "+ok)
	d.submit("S33", b+"/DELETE "+ok)
	d.submit("S34", b+"/IDENTIFY "+top+"BAT98OK.")
	d.submit("S35", b+"/LOG_FILE="+logs+" "+ok)

	d.c("SHOW QUEUE BAT98_B")
	d.c("SHOW QUEUE/FULL/ALL BAT98_B")
	d.c("SHOW QUEUE/BY_JOB_STATUS=HOLDING BAT98_B")
	d.c("SHOW QUEUE/BY_JOB_STATUS=PENDING BAT98_B")
	d.c("SHOW QUEUE/BY_JOB_STATUS=TIMED_RELEASE BAT98_B")
	d.c("SHOW QUEUE/BY_JOB_STATUS=(HOLDING,PENDING) BAT98_B")
	d.c("SHOW QUEUE/USER_NAME=BAT98 BAT98_B")
	d.c("SHOW QUEUE/ALL/USER_NAME=BAT98 BAT98_B")
	d.c("SHOW QUEUE/BRIEF BAT98_B")
	d.c("SHOW ENTRY")
	d.c("SHOW ENTRY/FULL 'S10'")
	d.c("SHOW ENTRY/FULL 'S12'")
	d.c("SHOW ENTRY/FULL 'S16'")
	d.c("SHOW ENTRY/FULL 'S20'")
	d.c("SHOW ENTRY/FULL 'S27'")
	d.c("SHOW ENTRY/FULL 'S35'")
	d.c("SHOW ENTRY/USER_NAME=BAT98")
	d.c("SHOW ENTRY 'S2','S3'")
	d.c("SHOW ENTRY 99999")

	d.c("SET ENTRY 'S2'/RELEASE")
	d.c("SHOW ENTRY 'S2'")
	d.c("SET ENTRY 'S1'/HOLD")
	d.c("SHOW ENTRY 'S1'")
	d.c("SET ENTRY 'S3'/AFTER=TOMORROW")
	d.c("SET ENTRY 'S4'/NOAFTER")
	d.c("SET ENTRY 'S9'/RELEASE")
	d.c("SHOW ENTRY 'S3','S4','S9'")
	d.c("SET ENTRY 'S1'/PARAMETERS=(X,Y)")
	d.c("SET ENTRY 'S1'/NAME=RENAMED98")
	d.c("SET ENTRY 'S1'/PRIORITY=10")
	d.c(`SET ENTRY 'S1'/NOTE="NEW NOTE"`)
	d.c("SET ENTRY 'S1'/LOG_FILE=" + logs + "RENAMED.LOG")
	d.c("SET ENTRY 'S1'/CPUTIME=0:02:00")
	d.c("SHOW ENTRY/FULL 'S1'")
	d.c("SET ENTRY 'S6'/REQUEUE=BAT98_P")
	d.c("SHOW QUEUE/ALL BAT98_P")
	d.c("SET ENTRY 99999/HOLD")
	d.c("SET ENTRY 'S1'/PRIORITY=255")

	d.c("DELETE/ENTRY='S1'")
	d.c("DELETE/ENTRY=('S2','S3')")
	d.c("DELETE/ENTRY='S4' BAT98_P")
	d.c("DELETE/ENTRY='S4' BAT98_B")
	d.c("DELETE/ENTRY='S6' BAT98_P")
	d.c("DELETE/ENTRY=99999")
	d.c("DELETE/ENTRY='S4'")
	d.c("SHOW QUEUE/ALL BAT98_B")
	d.c("DELETE/QUEUE BAT98_B")
	d.c("SHOW QUEUE BAT98_B")
	d.c("DIRECTORY " + top + "BAT98OK.COM")
}

// runningJobs run on BAT98_A: several files, and how a job ends.
func runningJobs(d *dcl) {
	d.section("RUNNING JOBS")
	d.c("START/QUEUE BAT98_A")
	d.c("SHOW QUEUE/FULL BAT98_A")

	d.job("R1", "BAT98R1", "", job("OK"))
	d.job("R2", "BAT98R2", "", job("OK", "WRN", "OK"))
	d.job("R3", "BAT98R3", "", job("OK", "ERR", "OK"))
	d.job("R4", "BAT98R4", "", job("OK", "FAT", "OK"))
	d.job("R5", "BAT98R5", "", job("OK", "BAD", "OK"))
	d.job("R6", "BAT98R6", "/PARAMETERS=(ONE,TWO)", top+"BAT98OK+"+top+"BAT98OK")
	d.job("R7", "BAT98R7", "/NOTIFY", job("OK"))
	d.job("R8", "BAT98R8", "/CPUTIME=0:00:03", job("C"))

	d.section("ENDINGS")
	d.started("R10", "BAT98R10", "", 60)
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.c("SHOW ENTRY/FULL 'R10'")
	d.c("STOP/QUEUE/ENTRY='R10' BAT98_A")
	d.sync("R10")
	d.showLog("BAT98R10")

	d.started("R11", "BAT98R11", "", 60)
	d.c("DELETE/ENTRY='R11'")
	d.sync("R11")
	d.showLog("BAT98R11")

	d.started("R12", "BAT98R12", "", 60)
	d.c(`IF R12 .NES. "" THEN STOP BATCH_'R12'`)
	d.sync("R12")
	d.showLog("BAT98R12")

	d.started("R13", "BAT98R13", "", 20)
	d.c("STOP/QUEUE BAT98_A")
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.line(`IF R13 .NES. "" THEN WAIT 0:0:5`)
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.c("START/QUEUE BAT98_A")
	d.sync("R13")
	d.showLog("BAT98R13")

	d.started("R14", "BAT98R14", "", 20)
	d.c("STOP/QUEUE/NEXT BAT98_A")
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.sync("R14")
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.c("START/QUEUE BAT98_A")
	d.showLog("BAT98R14")

	d.started("R15", "BAT98R15", "", 60)
	d.c("STOP/QUEUE/RESET BAT98_A")
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.sync("R15")
	d.c("START/QUEUE BAT98_A")
	d.showLog("BAT98R15")

	d.started("R16", "BAT98R16", "", 30)
	d.c(`SET ENTRY 'R16'/NOTE="WHILE RUNNING"`)
	d.c("SET ENTRY 'R16'/PARAMETERS=(CHANGED)")
	d.c("SET ENTRY 'R16'/HOLD")
	d.c("SHOW ENTRY/FULL 'R16'")
	d.c("STOP/QUEUE/ENTRY='R16' BAT98_A")
	d.sync("R16")
	d.showLog("BAT98R16")

	d.section("THE JOB LIMIT")
	d.line("DELETE %s*.RUN;*", scratch)

	for i := 1; i <= 4; i++ {
		name := fmt.Sprintf("BAT98L%d", i)
		d.submit(fmt.Sprintf("JL%d", i), fmt.Sprintf("/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=%s/LOG_FILE=%s%s.LOG/PARAMETERS=(%s,15) %sBAT98W",
			name, logs, name, name, top))
	}

	d.line("CALL AWAIT BAT98L1 'JL1'")
	d.line("CALL AWAIT BAT98L2 'JL2'")
	d.line("CALL AWAIT BAT98L3 'JL3'")
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")

	for i := 1; i <= 4; i++ {
		d.sync(fmt.Sprintf("JL%d", i))
		jobLogs = append(jobLogs, fmt.Sprintf("BAT98L%d.LOG", i))
	}

	d.section("RETAINED JOBS")
	d.job("R17", "BAT98R17", "/RETAIN=ALWAYS", job("OK"))
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.c("SHOW ENTRY/FULL 'R17'")
	d.sync("R17")
	d.c("DELETE/ENTRY='R17'")
	d.job("R18", "BAT98R18", "", job("ERR"))
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.c("SHOW ENTRY/FULL 'R18'")
	d.c("DELETE/ENTRY='R18'")
	d.job("R19", "BAT98R19", "/RETAIN=ERROR", job("OK"))
	d.c("SHOW QUEUE/ALL BAT98_A")

	d.section("SYNCHRONIZE")
	d.c("SYNCHRONIZE/ENTRY=99999")
	d.c("SYNCHRONIZE NOSUCHJOB98/QUEUE=BAT98_A")
	d.submit("R20", "/QUEUE=BAT98_A/NOPRINTER/NOLOG_FILE/NAME=BAT98SYNC "+top+"BAT98OK")
	d.c("SYNCHRONIZE BAT98SYNC/QUEUE=BAT98_A")
	d.c("SYNCHRONIZE/ENTRY='R20'")

	d.section("DELETE")
	d.line("COPY %sBAT98OK.COM %sDELME.COM", top, scratch)
	d.job("R21", "BAT98R21", "/DELETE", scratch+"DELME")
	d.c("DIRECTORY " + scratch + "DELME.COM")
	d.line("DELETE %sDELME.COM;*", scratch)

	d.section("AFTER ON A RUNNING QUEUE")
	d.line("WRITE SYS$OUTPUT F$TIME()")
	d.submit("R22", "/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=BAT98R22/LOG_FILE="+logs+`BAT98R22.LOG/AFTER="+0:00:20" `+top+"BAT98OK")
	d.c("SHOW ENTRY/FULL 'R22'")
	d.sync("R22")
	d.line("WRITE SYS$OUTPUT F$TIME()")
	d.showLog("BAT98R22")
}

// logCases are where a job's log goes.
func logCases(d *dcl) {
	d.section("LOG FILES")

	ok := top + "BAT98OK"

	cases := []struct{ name, quals string }{
		{"BAT98G1", "/NOPRINTER/KEEP"},
		{"BAT98G2", "/NOPRINTER/KEEP/USER=BAT98"},
		{"BAT98G3", "/NOPRINTER/KEEP/LOG_FILE=" + logs},
		{"BAT98G4", "/NOPRINTER/KEEP/LOG_FILE=BAT98G4X"},
		{"BAT98G5", "/NOPRINTER/KEEP/LOG_FILE=" + logs + "BAT98G5.TXT"},
		{"BAT98G6", "/NOPRINTER/KEEP/LOG_FILE"},
		{"BAT98G7", "/NOPRINTER/KEEP/NOLOG_FILE"},
		{"BAT98G8", "/NOPRINTER/KEEP/LOG_FILE=" + disk + "[NOSUCHDIR98]BAT98G8.LOG"},
		{"BAT98G9", "/NOPRINTER/KEEP/USER=BAT98W/LOG_FILE=" + disk + "[BAT98]BAT98G9.LOG"},
		{"BAT98G10", "/NOPRINTER/NOKEEP"},
		{"BAT98G11", "/NOPRINTER"},
		{"BAT98G12", "/LOG_FILE=" + scratch + "BAT98G12.LOG/NOPRINTER/NOKEEP"},
	}

	for i, k := range cases {
		sym := fmt.Sprintf("G%d", i+1)
		d.submit(sym, fmt.Sprintf("/QUEUE=BAT98_A/NAME=%s%s %s", k.name, k.quals, ok))
		d.c("SHOW ENTRY/FULL '" + sym + "'")
		d.sync(sym)
		d.line("CALL FINDLOG %s", k.name)

		if strings.Contains(k.quals, "BAT98G4X") {
			d.line("CALL FINDLOG BAT98G4X")
		}
	}

	// The default /PRINTER, and /KEEP with it: only when the VAX has no
	// SYS$PRINT queue, so the probe never prints on one.
	d.line(`IF PRINTQ .NES. "" THEN GOTO NO_PRINT_CASES`)
	d.submit("G13", "/QUEUE=BAT98_A/NAME=BAT98G13 "+ok)
	d.sync("G13")
	d.line("CALL FINDLOG BAT98G13")
	d.submit("G14", "/QUEUE=BAT98_A/NAME=BAT98G14/KEEP "+ok)
	d.sync("G14")
	d.line("CALL FINDLOG BAT98G14")
	d.submit("G15", "/QUEUE=BAT98_A/NAME=BAT98G15/PRINTER/NOKEEP "+ok)
	d.sync("G15")
	d.line("CALL FINDLOG BAT98G15")
	d.line("NO_PRINT_CASES:")
}

// orderCases are the order jobs run in: priority, and time.
func orderCases(d *dcl) {
	d.section("ORDER")
	d.c("INITIALIZE/QUEUE/BATCH/JOB_LIMIT=1 BAT98_B")
	d.line("DELETE %sORDER.TXT;*", scratch)
	d.line("CREATE %sORDER.TXT", scratch)

	o := func(sym, name, quals string) {
		d.submit(sym, fmt.Sprintf("/QUEUE=BAT98_B/NOPRINTER/NOLOG_FILE/NAME=%s%s/PARAMETERS=%s %sBAT98O",
			name, quals, name, top))
	}

	o("O1", "ORDER1", "/PRIORITY=10")
	o("O2", "ORDER2", "/PRIORITY=100")
	o("O3", "ORDER3", "/PRIORITY=50")
	o("O4", "ORDER4", "")
	o("O5", "ORDER5", `/PRIORITY=200/AFTER="+0:00:15"`)
	o("O6", "ORDER6", "/PRIORITY=200/HOLD")
	o("O7", "ORDER7", "/PRIORITY=50")
	d.c("SHOW QUEUE/FULL/ALL BAT98_B")
	d.c("START/QUEUE BAT98_B")

	for _, s := range []string{"O1", "O2", "O3", "O4", "O7", "O5"} {
		d.sync(s)
	}

	d.c("TYPE " + scratch + "ORDER.TXT")
	d.c("SET ENTRY 'O6'/RELEASE")
	d.sync("O6")
	d.c("TYPE " + scratch + "ORDER.TXT")
	d.c("STOP/QUEUE/NEXT BAT98_B")
}

// restartCases are /RESTART: a job requeued while it runs.
func restartCases(d *dcl) {
	d.section("RESTART")
	d.line("DELETE %s*.RUN;*", scratch)
	d.submit("X1", "/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=BAT98X1/LOG_FILE="+logs+"BAT98X1.LOG/RESTART/PARAMETERS=BAT98X1 "+top+"BAT98R")
	d.line("CALL AWAIT BAT98X1 'X1'")
	d.c("SHOW ENTRY/FULL 'X1'")
	d.c("STOP/QUEUE/REQUEUE/ENTRY='X1' BAT98_A")
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.sync("X1")
	d.showLog("BAT98X1")

	d.submit("X2", "/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=BAT98X2/LOG_FILE="+logs+"BAT98X2.LOG/PARAMETERS=BAT98X2 "+top+"BAT98R")
	d.line("CALL AWAIT BAT98X2 'X2'")
	d.c("STOP/QUEUE/REQUEUE/ENTRY='X2' BAT98_A")
	d.c("SHOW QUEUE/FULL/ALL BAT98_A")
	d.sync("X2")
	d.showLog("BAT98X2")

	heldRestart(d, "BAT98X3")
}

// heldRestart is a job requeued /HOLD while it runs, then released: name
// is its job name and log.
func heldRestart(d *dcl, name string) {
	d.submit("X3", "/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME="+name+"/LOG_FILE="+logs+name+".LOG/RESTART/PARAMETERS="+name+" "+top+"BAT98R")
	d.line("CALL AWAIT %s 'X3'", name)
	d.c("STOP/QUEUE/REQUEUE/HOLD/ENTRY='X3' BAT98_A")
	d.c("SHOW ENTRY/FULL 'X3'")
	d.c("SET ENTRY 'X3'/RELEASE")
	d.line("CALL RELEASE 'X3'")
	d.c("SHOW ENTRY/FULL 'X3'")
	d.sync("X3")
	d.showLog(name)
}

// viewCases run BAT98J.COM as each account.
func viewCases(d *dcl) {
	d.section("THE JOB'S VIEW")

	users := []string{"", "BAT98", "BAT98O", "BAT98W"}
	for i, u := range users {
		name := "BAT98V_SYSTEM"
		quals := ""

		if u != "" {
			name = "BAT98V_" + u
			quals = "/USER=" + u
		}

		d.job(fmt.Sprintf("V%d", i+1), name, quals+`/PARAMETERS=(ALPHA,"two words",3,"",E,F,G,"H h")`, top+"BAT98J")
	}
}

// serviceCases assemble the definition probes and run BAT98Q.
func serviceCases(d *dcl) {
	d.section("SERVICES")
	d.line("SET DEFAULT %s", top)
	d.c("MACRO DEF_SJC")
	d.c("MACRO DEF_QUI")
	d.c("MACRO DEF_JBC")
	d.c("MACRO BAT98Q")
	d.c("LINK BAT98Q")
	d.c("RUN BAT98Q")
	d.line("SET DEFAULT %s", scratch)
}

// The protection matrix's states: BAT98_P's owner, protection, and ACL
// for each round of BAT98P.COM. BAT98 is [200,10], BAT98O [200,11], and
// BAT98W [300,10]: so with the owner [200,10], BAT98 is the owner, BAT98O
// in its group, and BAT98W in the world.
var states = []struct {
	name, owner, prot, acl1, acl2 string
}{
	{"A", "[1,4]", "(S:M,O:D,G:R,W:S)", "", ""},
	{"B", "[200,10]", "(S:M,O:D,G:R,W:S)", "", ""},
	{"C", "[200,10]", "(S,O,G,W)", "", ""},
	{"D", "[200,10]", "(S:M,O:D,G:R,W:M)", "", ""},
	{"E", "[200,10]", "(S,O,G,W)", "(IDENTIFIER=BAT98_ID,ACCESS=READ+SUBMIT)", "(IDENTIFIER=[300,10],ACCESS=READ+SUBMIT+MANAGE+DELETE)"},
	{"F", "[200,10]", "(S:M,O:R,G:D,W)", "", ""},
}

// passes are BAT98P.COM's privilege passes for BAT98O, authorized for
// every privilege; the other accounts run the first only.
var passes = []string{
	"NONE", "OPER", "ALTPRI", "CMKRNL", "CMKRNL+SYSPRV", "SYSNAM", "BYPASS",
	"SYSPRV", "GRPPRV", "READALL", "SECURITY", "WORLD", "GROUP",
}

func protectionCases(d *dcl) {
	d.section("PROTECTION")

	for _, s := range states {
		for _, a := range accounts {
			n := 1
			if a.name == "BAT98O" {
				n = len(passes)
			}

			d.section(fmt.Sprintf("PROTECTION %s %s %s AS %s", s.name, s.owner, s.prot, a.name))
			d.line(`CALL MAKEP %s "%s" "%s" "%s" "%s" %d`, s.name, s.owner, s.prot, s.acl1, s.acl2, n)

			name := fmt.Sprintf("BAT98P_%s_%s", s.name, a.name)
			jobLogs = append(jobLogs, name+".LOG")
			d.submit("PJ", fmt.Sprintf("/USER=%s/QUEUE=BAT98_A/NOPRINTER/KEEP/CPUTIME=0:10:00/NAME=%s/LOG_FILE=%s%s.LOG/PARAMETERS=(%s,\"''HELD'\",'RET') %sBAT98P",
				a.name, name, logs, name, s.name, top))
			d.sync("PJ")
			d.c("SHOW QUEUE/FULL/ALL BAT98_P")
		}
	}
}

// ---------------------------------------------------------------------
// bat98b.com, round 2: BAT98Q again. Round 1's BAT98Q gave $SNDJBC's
// longword output items 256-byte buffers, and every job function asking
// for one (ENTER_FILE, CREATE_JOB) failed, JBC-F-INVPARLEN, so the job
// cases after them had no job. Round 2 runs the corrected program alone;
// it needs neither the accounts nor round 1's queues.

func roundTwoProc() string {
	d := &dcl{}

	d.raw(`$ ! BAT98B.COM - Phase 98's probe of batch queues: round 2, SYSTEM's run,
$ ! BAT98Q again with its output items corrected. Written by
$ ! testdata/bat98/gen.go. Run it as SYSTEM, from the exchange volume,
$ ! mounted /SYSTEM (README.md, "Round 2"):
$ !
$ !     @DUA1:[000000]BAT98B
$ !
$ ! Its log is BAT98$DISK:[BAT98LOG]BAT98B.LOG.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK 'DEV'
$ LOGFILE = DEV + "[BAT98LOG]BAT98B.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "BAT98B finished; the log is ", LOGFILE
$ DEASSIGN/SYSTEM/EXECUTIVE_MODE BAT98$DISK
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("NODENAME"), " ", F$TIME()
$ SHOW QUEUE/MANAGER
$ @BAT98$DISK:[000000]BAT98Z BAT98_Q
$ @BAT98$DISK:[000000]BAT98Z BAT98_Q2
`)

	serviceCases(d)

	lastCaseB = d.n

	d.raw(`$ SHOW QUEUE BAT98*
$ WRITE SYS$OUTPUT "BAT98B FINISHED"
$ EXIT
`)

	return d.b.String()
}

// lastCaseB is BAT98B.COM's case count.
var lastCaseB int

// ---------------------------------------------------------------------
// bat98z.com: the probe's queues taken away.

func dropQueues() string {
	var b strings.Builder

	b.WriteString(`$ ! BAT98Z.COM - Phase 98's probe: its queues taken away. Written by
$ ! testdata/bat98/gen.go. With a queue's name as P1, that queue; with
$ ! none, every queue the probe makes. Each is reset (its executing jobs
$ ! aborted) and deleted, with its jobs.
$ !
$ SET NOON
$ IF P1 .NES. "" THEN GOTO ONE
`)

	names := []string{"BAT98_A", "BAT98_B", "BAT98_D", "BAT98_E", "BAT98_P", "BAT98_X", "BAT98_Q", "BAT98_Q2"}
	for i := 1; i <= len(passes); i++ {
		names = append(names, fmt.Sprintf("BAT98_D%d", i))
	}

	for _, n := range names {
		fmt.Fprintf(&b, "$ @'F$ENVIRONMENT(\"PROCEDURE\")' %s\n", n)
	}

	b.WriteString(`$ EXIT
$ ONE:
$ X = F$GETQUI("CANCEL_OPERATION")
$ Q = ""
$ Q = F$GETQUI("DISPLAY_QUEUE","QUEUE_NAME",P1)
$ IF Q .EQS. "" THEN EXIT
$ STOP/QUEUE/RESET 'P1'
$ DELETE/QUEUE 'P1'
$ EXIT
`)

	return b.String()
}

// ---------------------------------------------------------------------
// bat98acct.com, the accounts.

func accountProc() string {
	var b strings.Builder

	b.WriteString(`$ ! BAT98ACCT.COM - Phase 98's probe: the accounts, in the VAX's own
$ ! SYSUAF. Written by testdata/bat98/gen.go. BAT98.COM runs it as SYSTEM;
$ ! BAT98DROP.COM undoes everything it does.
$ !
$ !   BAT98   [200,10]  TMPMBX, NETMBX; holds the identifier BAT98_ID
$ !   BAT98O  [200,11]  authorized for every privilege, TMPMBX and NETMBX
$ !                     by default
$ !   BAT98W  [300,10]  TMPMBX, NETMBX; another group
$ !
$ ! Each has the password ` + password + ` (nothing logs in with it: a batch
$ ! job's login asks for none) and a home of its own on the exchange
$ ! volume, with a LOGIN.COM that says when it ran.
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## ACCOUNTS"
$ IF F$TRNLNM("SYSUAF","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE SYSUAF
$ IF F$TRNLNM("RIGHTSLIST","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE RIGHTSLIST
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ MCR AUTHORIZE
ADD BAT98/UIC=[200,10]/PASSWORD=` + password + `/FLAGS=NODISUSER
MODIFY BAT98/DEVICE=BAT98$DISK:/DIRECTORY=[BAT98]/ACCOUNT=GOVAX98/OWNER="GOVAX PROBE"
MODIFY BAT98/PRIVILEGES=(TMPMBX,NETMBX)/DEFPRIVILEGES=(TMPMBX,NETMBX)/LGICMD=""
COPY BAT98 BAT98O/UIC=[200,11]/PASSWORD=` + password + `
MODIFY BAT98O/DIRECTORY=[BAT98O]/PRIVILEGES=ALL/DEFPRIVILEGES=(TMPMBX,NETMBX)
COPY BAT98 BAT98W/UIC=[300,10]/PASSWORD=` + password + `
MODIFY BAT98W/DIRECTORY=[BAT98W]
ADD/IDENTIFIER BAT98_ID
GRANT/IDENTIFIER BAT98_ID BAT98
SHOW BAT98/FULL
SHOW BAT98O/FULL
SHOW BAT98W/FULL
EXIT
$ SET DEFAULT 'HOME'
$ !
$ ! Each home owned by its account, with its LOGIN.COM.
`)

	for _, a := range accounts {
		dirFile := strings.Trim(a.home, "[]") + ".DIR"
		fmt.Fprintf(&b, "$ SET SECURITY/OWNER=%s/PROTECTION=(S:RWE,O:RWE,G:RE,W:RE) %s[000000]%s\n", a.uic, disk, dirFile)
		fmt.Fprintf(&b, "$ SET SECURITY/OWNER=%s/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) %s%s*.*;*\n", a.uic, disk, a.home)
		fmt.Fprintf(&b, "$ DIRECTORY/SECURITY %s%s\n", disk, a.home)
	}

	b.WriteString("$ EXIT\n")

	return b.String()
}

// ---------------------------------------------------------------------
// bat98drop.com, the end.

func dropProc() string {
	return `$ ! BAT98DROP.COM - Phase 98's probe: the end. Written by
$ ! testdata/bat98/gen.go. Run it as SYSTEM after the steps by hand
$ ! (README.md):
$ !
$ !     @BAT98$DISK:[000000]BAT98DROP
$ !
$ ! It deletes the probe's queues (BAT98Z.COM), removes the accounts and
$ ! BAT98_ID (and GOVAX98, the group identifier ADD may make for group
$ ! 300), and then deassigns BAT98$DISK. Its log is
$ ! BAT98$DISK:[BAT98LOG]BAT98DROP.LOG. It can be run more than once.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ LOGFILE = DEV + "[BAT98LOG]BAT98DROP.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "BAT98DROP finished; the log is ", LOGFILE
$ DEASSIGN/SYSTEM/EXECUTIVE_MODE BAT98$DISK
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## QUEUES"
$ SHOW QUEUE/FULL/ALL BAT98*
$ @BAT98$DISK:[000000]BAT98Z
$ SHOW QUEUE BAT98*
$ !
$ WRITE SYS$OUTPUT "######## ACCOUNTS"
$ IF F$TRNLNM("SYSUAF","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE SYSUAF
$ IF F$TRNLNM("RIGHTSLIST","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE RIGHTSLIST
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ MCR AUTHORIZE
SHOW BAT98/FULL
SHOW BAT98O/FULL
SHOW BAT98W/FULL
REMOVE BAT98
REMOVE BAT98O
REMOVE BAT98W
REMOVE/IDENTIFIER BAT98_ID
REMOVE/IDENTIFIER GOVAX98
SHOW BAT98*
EXIT
$ SET DEFAULT 'HOME'
$ DIRECTORY ` + logs + `
$ WRITE SYS$OUTPUT "BAT98DROP FINISHED"
$ EXIT
`
}

// ---------------------------------------------------------------------
// The job procedures.

// jpiItems are F$GETJPI's items, from the 7.3-1 DCL Dictionary's list, as
// testdata/uaf95's probe asked them.
var jpiItems = []string{
	"ACCOUNT", "APTCNT", "ASTACT", "ASTCNT", "ASTEN", "ASTLM", "AUTHPRI",
	"AUTHPRIV", "BIOCNT", "BIOLM", "BUFIO", "BYTCNT", "BYTLM", "CLINAME",
	"CPULIM", "CPUTIM", "CREPRC_FLAGS", "CURPRIV", "DFPFC", "DFWSCNT",
	"DIOCNT", "DIOLM", "DIRIO", "EFCS", "EFCU", "EFWM", "ENQCNT", "ENQLM",
	"EXCVEC", "FILCNT", "FILLM", "FINALEXC", "FREP0VA", "FREP1VA",
	"FREPTECNT", "GPGCNT", "GRP", "IMAGECOUNT", "IMAGNAME", "IMAGPRIV",
	"JOBPRCCNT", "JOBTYPE", "LAST_LOGIN_I", "LAST_LOGIN_N", "LOGINTIM",
	"LOGIN_FAILURES", "LOGIN_FLAGS", "MASTER_PID", "MAXDETACH", "MAXJOBS",
	"MEM", "MODE", "MSGMASK", "NODENAME", "NODE_CSID", "NODE_VERSION",
	"OWNER", "PAGEFLTS", "PAGFILCNT", "PAGFILLOC", "PGFLQUOTA", "PHDFLAGS",
	"PID", "PPGCNT", "PRCCNT", "PRCLM", "PRCNAM", "PRI", "PRIB", "PROCPRIV",
	"SHRFILLM", "SITESPEC", "STATE", "STS", "STS2", "SWPFILLOC", "TERMINAL",
	"TMBU", "TQCNT", "TQLM", "UAF_FLAGS", "UIC", "USERNAME", "VIRTPEAK",
	"VOLUMES", "WSAUTH", "WSAUTHEXT", "WSEXTENT", "WSPEAK", "WSQUOTA",
	"WSSIZE",
}

// envItems are F$ENVIRONMENT's items.
var envItems = []string{
	"CAPTIVE", "CONTROL", "DEFAULT", "DEPTH", "INTERACTIVE", "MAX_DEPTH",
	"MESSAGE", "NOCONTROL", "ON_CONTROL_Y", "ON_SEVERITY", "OUTPUT_RATE",
	"PROCEDURE", "PROMPT", "PROMPT_CONTROL", "PROTECTION", "SYMBOL_SCOPE",
	"VERB_SCOPE", "VERIFY_IMAGE", "VERIFY_PROCEDURE",
}

// quiJobItems are what bat98j.com asks F$GETQUI of its own job.
var quiJobItems = []string{
	"ACCOUNT_NAME", "AFTER_TIME", "CHECKPOINT_DATA", "CLI", "CPU_LIMIT",
	"ENTRY_NUMBER", "FILE_COUNT", "JOB_FLAGS", "JOB_NAME", "JOB_PID",
	"JOB_SIZE", "JOB_STATUS", "LOG_QUEUE", "LOG_SPECIFICATION", "NOTE",
	"PARAMETER_1", "PARAMETER_2", "PARAMETER_8", "PENDING_JOB_REASON",
	"PRIORITY", "QUEUE_NAME", "SUBMISSION_TIME", "UIC", "USERNAME",
	"WSDEFAULT", "WSEXTENT", "WSQUOTA",
}

// shownNames are the logical names bat98j.com shows.
var shownNames = []string{
	"SYS$INPUT", "SYS$OUTPUT", "SYS$ERROR", "SYS$COMMAND", "SYS$DISK",
	"SYS$LOGIN", "SYS$LOGIN_DEVICE", "SYS$SCRATCH", "SYS$NODE", "TT",
	"SYS$NET", "SYS$PRINT", "SYS$BATCH", "SYS$SYLOGIN",
}

func viewProc() string {
	var b strings.Builder

	b.WriteString(`$ ! BAT98J.COM - Phase 98's probe: a batch job's view of itself.
$ ! Written by testdata/bat98/gen.go. BAT98.COM submits it as SYSTEM and
$ ! as each probe account, with eight parameters; it writes to its log.
$ !
$ SET NOON
$ WRITE SYS$OUTPUT "######## BAT98J ", F$GETJPI("","USERNAME")
$ WRITE SYS$OUTPUT "MODE ", F$MODE()
$ WRITE SYS$OUTPUT "VERIFY ", F$VERIFY()
$ SHOW SYMBOL BAT98_LOGIN
$ SHOW SYMBOL $RESTART
$ SHOW SYMBOL BATCH$RESTART
$ I = 1
$ PARAMS:
$ IF I .GT. 8 THEN GOTO PARAMS_DONE
$ V = P'I'
$ WRITE SYS$OUTPUT "P", I, " [", V, "] ", F$LENGTH(V)
$ I = I + 1
$ GOTO PARAMS
$ PARAMS_DONE:
$ !
$ WRITE SYS$OUTPUT "######## F$ENVIRONMENT"
`)

	for _, e := range envItems {
		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"  %s = \", F$ENVIRONMENT(\"%s\")\n", e, e)
	}

	b.WriteString("$ !\n$ WRITE SYS$OUTPUT \"######## F$GETJPI\"\n")

	for _, j := range jpiItems {
		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"  %s = \", F$GETJPI(\"\",\"%s\")\n", j, j)
	}

	b.WriteString("$ !\n$ WRITE SYS$OUTPUT \"######## F$GETQUI\"\n$ X = F$GETQUI(\"CANCEL_OPERATION\")\n")
	b.WriteString(`$ WRITE SYS$OUTPUT "  QUEUE = ", F$GETQUI("DISPLAY_QUEUE","QUEUE_NAME",,"THIS_JOB")` + "\n")
	b.WriteString(`$ WRITE SYS$OUTPUT "  BASE_PRIORITY = ", F$GETQUI("DISPLAY_QUEUE","BASE_PRIORITY",,"THIS_JOB")` + "\n")

	for _, q := range quiJobItems {
		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"  %s = \", F$GETQUI(\"DISPLAY_JOB\",\"%s\",,\"THIS_JOB\")\n", q, q)
	}

	b.WriteString(`$ WRITE SYS$OUTPUT "  FILE = ", F$GETQUI("DISPLAY_FILE","FILE_SPECIFICATION",,"THIS_JOB")` + "\n")
	b.WriteString(`$ !
$ WRITE SYS$OUTPUT "######## NAMES"
`)

	for _, n := range shownNames {
		fmt.Fprintf(&b, "$ SHOW LOGICAL/FULL %s\n", n)
	}

	b.WriteString(`$ SHOW LOGICAL/JOB
$ SHOW LOGICAL/PROCESS
$ WRITE SYS$OUTPUT "  SYS$OUTPUT DEVICE ", F$GETDVI("SYS$OUTPUT","DEVNAM")
$ WRITE SYS$OUTPUT "  SYS$INPUT DEVICE ", F$GETDVI("SYS$INPUT","DEVNAM")
$ WRITE SYS$OUTPUT "  SYS$COMMAND DEVICE ", F$GETDVI("SYS$COMMAND","DEVNAM")
$ SHOW DEFAULT
$ !
$ WRITE SYS$OUTPUT "######## THE PROCESS"
$ SHOW PROCESS/ALL
$ SHOW PROCESS/PRIVILEGES
$ SHOW PROCESS/RIGHTS
$ SHOW PROCESS/QUOTAS
$ SHOW SYMBOL/GLOBAL/ALL
$ SHOW SYMBOL/LOCAL/ALL
$ WRITE SYS$ERROR "BAT98J WROTE THIS TO SYS$ERROR"
$ WRITE SYS$OUTPUT "BAT98J FINISHED"
$ EXIT
`)

	return b.String()
}

// passCommands are what BAT98P.COM tries in each pass. 'H' is one of
// SYSTEM's held jobs, 'R' its retained job, 'D' this pass's own stopped
// queue; OWN, HI, and OTH are the pass's own SUBMITs.
var passCommands = []string{
	"SHOW QUEUE BAT98_P",
	"SHOW QUEUE/FULL/ALL BAT98_P",
	"SHOW ENTRY 'R'",
	"SHOW ENTRY/FULL 'H'",
	`WRITE SYS$OUTPUT "  GETQUI PROTECTION ", F$GETQUI("DISPLAY_QUEUE","PROTECTION","BAT98_P")`,
	`WRITE SYS$OUTPUT "  GETQUI OWNER ", F$GETQUI("DISPLAY_QUEUE","OWNER_UIC","BAT98_P")`,
	"+OWN /QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/NAME=OWN98",
	"SHOW ENTRY 'OWN'",
	`SET ENTRY 'OWN'/NOTE="OWN NOTE"`,
	"+HI /QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/PRIORITY=200",
	"DELETE/ENTRY='HI' BAT98_P",
	"+OTH /QUEUE=BAT98_P/HOLD/NOPRINTER/NOLOG_FILE/USER=BAT98W",
	"DELETE/ENTRY='OTH' BAT98_P",
	`SET ENTRY 'H'/NOTE="CHANGED"`,
	"SYNCHRONIZE/ENTRY='R'",
	"STOP/QUEUE/ENTRY='H' BAT98_P",
	"STOP/QUEUE/NEXT BAT98_P",
	"START/QUEUE BAT98_P",
	"SET QUEUE/JOB_LIMIT=2 BAT98_P",
	"SET QUEUE/JOB_LIMIT=1 BAT98_P",
	"INITIALIZE/QUEUE/BATCH BAT98_X",
	"DELETE/QUEUE BAT98_X",
	"SET SECURITY/CLASS=QUEUE/ACL=(IDENTIFIER=[300,10],ACCESS=READ) 'D'",
	"SHOW SECURITY/CLASS=QUEUE 'D'",
	"INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2 'D'",
	"START/QUEUE 'D'",
	"STOP/QUEUE/NEXT 'D'",
	"DELETE/QUEUE 'D'",
	"DELETE/ENTRY='OWN' BAT98_P",
	"DELETE/ENTRY='H' BAT98_P",
	"SHOW QUEUE/MANAGER",
}

func matrixProc() string {
	var b strings.Builder

	b.WriteString(`$ ! BAT98P.COM - Phase 98's probe: who may do what to a queue. Written
$ ! by testdata/bat98/gen.go. BAT98.COM submits it as each probe account
$ ! for each protection state of BAT98_P:
$ !
$ !     P1  the state's letter
$ !     P2  SYSTEM's held jobs in BAT98_P, one for each pass
$ !     P3  a job of SYSTEM's retained in BAT98_P
$ !
$ ! BAT98O, authorized for every privilege, tries everything once with
$ ! none of them, then once with each (or each pair) enabled; the others
$ ! once with what they have.
$ !
$ SET NOON
$ WHO = F$EDIT(F$GETJPI("","USERNAME"),"TRIM")
$ WRITE SYS$OUTPUT "######## BAT98P STATE ", P1, " AS ", WHO
$ PASSES = "NONE"
$ IF WHO .EQS. "BAT98O" THEN PASSES = "` + strings.Join(passes, ",") + `"
$ R = P3
$ I = 0
$ PASS:
$ P = F$ELEMENT(I,",",PASSES)
$ IF P .EQS. "," THEN GOTO FINISHED
$ H = F$ELEMENT(I,",",P2)
$ IF H .EQS. "," THEN H = ""
$ D = "BAT98_D" + F$STRING(I+1)
$ PRIVS = "TMPMBX,NETMBX"
$ J = 0
$ PRIV_LOOP:
$ Q = F$ELEMENT(J,"+",P)
$ IF Q .EQS. "+" THEN GOTO PRIV_DONE
$ IF Q .NES. "NONE" THEN PRIVS = PRIVS + "," + Q
$ J = J + 1
$ GOTO PRIV_LOOP
$ PRIV_DONE:
$ SET PROCESS/PRIVILEGES=NOALL
$ SET PROCESS/PRIVILEGES=('PRIVS')
$ WRITE SYS$OUTPUT "######## PASS ", P, " AS ", WHO, " STATE ", P1, " HELD ", H, " RETAINED ", R, " QUEUE ", D
$ SHOW PROCESS/PRIVILEGES
`)

	for i, c := range passCommands {
		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"== ''P' %d\"\n", i+1)

		if strings.HasPrefix(c, "+") {
			sym, quals, _ := strings.Cut(c[1:], " ")
			fmt.Fprintf(&b, "$ IF F$TYPE($ENTRY) .NES. \"\" THEN DELETE/SYMBOL/LOCAL $ENTRY\n")
			fmt.Fprintf(&b, "$ SUBMIT%s %sBAT98OK\n", quals, top)
			fmt.Fprintf(&b, "$ SHOW SYMBOL $STATUS\n")
			fmt.Fprintf(&b, "$ %s = \"\"\n", sym)
			fmt.Fprintf(&b, "$ IF F$TYPE($ENTRY) .NES. \"\" THEN %s = $ENTRY\n", sym)
			fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"  %s = \", %s\n", sym, sym)

			continue
		}

		// A command naming an entry the pass doesn't have is passed over, so
		// that no command ever names a missing entry by accident.
		guard := ""

		for _, sym := range []string{"OWN", "HI", "OTH", "H", "R"} {
			if strings.Contains(c, "'"+sym+"'") {
				fmt.Fprintf(&b, "$ IF %s .EQS. \"\" THEN WRITE SYS$OUTPUT \"  NO %s\"\n", sym, sym)
				guard = fmt.Sprintf("IF %s .NES. \"\" THEN ", sym)
			}
		}

		fmt.Fprintf(&b, "$ %s%s\n$ SHOW SYMBOL $STATUS\n", guard, c)
	}

	b.WriteString(`$ I = I + 1
$ GOTO PASS
$ FINISHED:
$ SET PROCESS/PRIVILEGES=NOALL
$ SET PROCESS/PRIVILEGES=(TMPMBX,NETMBX)
$ WRITE SYS$OUTPUT "BAT98P FINISHED"
$ EXIT
`)

	return b.String()
}

// The small procedures a job runs.

func endsWith(name, status, note string) string {
	return fmt.Sprintf(`$ ! %s.COM - Phase 98's probe: %s. Written by testdata/bat98/gen.go.
$ WRITE SYS$OUTPUT "%s RAN, P1=", P1, ", P2=", P2, ", DEPTH ", F$ENVIRONMENT("DEPTH")
$ EXIT %s
`, name, note, name, status)
}

func badProc() string {
	return `$ ! BAT98BAD.COM - Phase 98's probe: a procedure with a command DCL
$ ! doesn't know, under DCL's default ON action. Written by
$ ! testdata/bat98/gen.go.
$ WRITE SYS$OUTPUT "BAT98BAD RAN"
$ NOSUCHVERB98
$ WRITE SYS$OUTPUT "BAT98BAD WENT ON"
$ EXIT 1
`
}

func waiterProc() string {
	return `$ ! BAT98W.COM - Phase 98's probe: a job that runs for a while. Written
$ ! by testdata/bat98/gen.go. P1 is its name: it writes
$ ! ` + scratch + `P1.RUN when it starts, which BAT98.COM waits for.
$ ! P2 is how many seconds it runs at most; then it ends by itself.
$ SET NOON
$ OPEN/WRITE MARK ` + scratch + `'P1'.RUN
$ WRITE MARK F$TIME()
$ CLOSE MARK
$ WRITE SYS$OUTPUT "BAT98W ", P1, " STARTED"
$ N = 0
$ LOOP:
$ IF N .GE. F$INTEGER(P2) THEN GOTO DONE
$ WAIT 0:0:1
$ N = N + 1
$ GOTO LOOP
$ DONE:
$ WRITE SYS$OUTPUT "BAT98W ", P1, " DONE"
$ EXIT
`
}

func cpuProc() string {
	return `$ ! BAT98C.COM - Phase 98's probe: a job that uses CPU time, 15 seconds
$ ! of it at most. Written by testdata/bat98/gen.go.
$ SET NOON
$ WRITE SYS$OUTPUT "BAT98C STARTED, CPULIM ", F$GETJPI("","CPULIM")
$ T0 = F$GETJPI("","CPUTIM")
$ I = 0
$ LOOP:
$ I = I + 1
$ IF (F$GETJPI("","CPUTIM") - T0 .LT. 1500) .AND. (I .LT. 2000000) THEN GOTO LOOP
$ WRITE SYS$OUTPUT "BAT98C DONE ", I
$ EXIT
`
}

func orderProc() string {
	return `$ ! BAT98O.COM - Phase 98's probe: a job that writes its name, P1, at
$ ! the end of ` + scratch + `ORDER.TXT. Written by
$ ! testdata/bat98/gen.go.
$ OPEN/APPEND ORDER ` + scratch + `ORDER.TXT
$ WRITE ORDER P1, " ", F$TIME()
$ CLOSE ORDER
$ EXIT
`
}

func restartProc() string {
	return `$ ! BAT98R.COM - Phase 98's probe: a job that can be restarted. Written
$ ! by testdata/bat98/gen.go. Its first step says where a restart is to
$ ! go on from (SET RESTART_VALUE), writes ` + scratch + `P1.RUN, and
$ ! waits a minute at most; BAT98.COM requeues it meanwhile. A restart
$ ! goes on from the second step.
$ SET NOON
$ WRITE SYS$OUTPUT "BAT98R ", P1, " STARTED"
$ SHOW SYMBOL $RESTART
$ SHOW SYMBOL BATCH$RESTART
$ IF F$TYPE(BATCH$RESTART) .EQS. "" THEN GOTO STEP1
$ IF BATCH$RESTART .NES. "" THEN GOTO 'BATCH$RESTART'
$ STEP1:
$ WRITE SYS$OUTPUT "BAT98R AT STEP1"
$ SET RESTART_VALUE=STEP2
$ SHOW SYMBOL BATCH$RESTART
$ OPEN/WRITE MARK ` + scratch + `'P1'.RUN
$ WRITE MARK F$TIME()
$ CLOSE MARK
$ N = 0
$ LOOP:
$ IF N .GE. 60 THEN GOTO STEP2
$ WAIT 0:0:1
$ N = N + 1
$ GOTO LOOP
$ STEP2:
$ WRITE SYS$OUTPUT "BAT98R AT STEP2"
$ EXIT
`
}

func loginProc(account string) string {
	return `$ ! LOGIN.COM - Phase 98's probe: ` + account + `'s login procedure. Written by
$ ! testdata/bat98/gen.go.
$ BAT98_LOGIN == "''F$ENVIRONMENT("DEPTH")' ''F$MODE()'"
$ WRITE SYS$OUTPUT "LOGIN.COM OF ` + account + ` AT DEPTH ", F$ENVIRONMENT("DEPTH"), ", MODE ", F$MODE()
$ EXIT
`
}

// ---------------------------------------------------------------------
// bat98q.mar: $SNDJBC and $GETQUI called directly.

// An item is one entry of a case's item list.
type item struct {
	kind byte   // 's' a string in, 'l' a longword in (an expression), 'c' a longword in from a cell, 'b' a Boolean, 'o' out
	code string // the item code, a name or a number
	arg  string // 's' the text; 'l' the expression; 'c' the cell
	size int    // 'o' the buffer's size (256 when 0)
	save string // 'o' a cell to keep the result's first longword in
}

type qcase struct {
	title  string
	getqui bool   // $GETQUIW, else $SNDJBCW
	fn     string // the function code
	items  []item
	noIOSB bool
	keep   string // a cell to keep the IOSB's status in
	onlyIf string // a cell whose status must be a success, or the case is passed over
}

func s(code, text string) item    { return item{kind: 's', code: code, arg: text} }
func l(code, expr string) item    { return item{kind: 'l', code: code, arg: expr} }
func cell(code, name string) item { return item{kind: 'c', code: code, arg: name} }
func flag(code string) item       { return item{kind: 'b', code: code} }
func out(code string) item        { return item{kind: 'o', code: code} }

func outs(codes ...string) []item {
	list := make([]item, len(codes))
	for i, c := range codes {
		list[i] = out("QUI$_" + c)
	}

	return list
}

func sj(title, fn string, items ...item) qcase {
	return qcase{title: title, fn: "SJC$_" + fn, items: items}
}

func gq(title, fn string, items ...item) qcase {
	return qcase{title: title, getqui: true, fn: "QUI$_" + fn, items: items}
}

const (
	okFile  = top + "BAT98OK.COM"
	wrnFile = top + "BAT98WRN.COM"
)

func qcases() []qcase {
	queue := s("SJC$_QUEUE", "BAT98_Q")
	entry := cell("SJC$_ENTRY_NUMBER", "QENTRY")

	// $SNDJBC's output items are longwords, and a buffer of any other
	// length is JBC-F-INVPARLEN (round 1 gave them 256 bytes, and every
	// job function that asked for one failed). $GETQUI takes any length.
	saveOut := func(code, into string) item {
		i := out(code)
		i.save = into
		i.size = 4

		return i
	}
	long := func(code string) item {
		i := out(code)
		i.size = 4

		return i
	}

	var list []qcase

	add := func(c ...qcase) { list = append(list, c...) }

	add(
		sj("CREATE_QUEUE BAT98_Q", "CREATE_QUEUE", queue, flag("SJC$_BATCH"),
			l("SJC$_JOB_LIMIT", "2"), l("SJC$_BASE_PRIORITY", "3"), l("SJC$_OWNER_UIC", "^X00010004"),
			l("SJC$_WSDEFAULT", "150"), s("SJC$_QUEUE_DESCRIPTION", "BAT98 Q")),
		sj("CREATE_QUEUE BAT98_Q again", "CREATE_QUEUE", queue, flag("SJC$_BATCH")),
		sj("CREATE_QUEUE BAT98_Q2", "CREATE_QUEUE", s("SJC$_QUEUE", "BAT98_Q2"), flag("SJC$_BATCH")),
		gq("DISPLAY_QUEUE BAT98_Q", "DISPLAY_QUEUE", append([]item{s("QUI$_SEARCH_NAME", "BAT98_Q")},
			outs("QUEUE_NAME", "QUEUE_STATUS", "QUEUE_FLAGS", "JOB_LIMIT", "BASE_PRIORITY", "OWNER_UIC",
				"PROTECTION", "WSDEFAULT", "WSQUOTA", "WSEXTENT", "CPU_DEFAULT", "CPU_LIMIT",
				"EXECUTING_JOB_COUNT", "HOLDING_JOB_COUNT", "PENDING_JOB_COUNT",
				"TIMED_RELEASE_JOB_COUNT", "RETAINED_JOB_COUNT", "QUEUE_DESCRIPTION")...)...),
		sj("START_QUEUE", "START_QUEUE", queue),
		sj("ENTER_FILE held", "ENTER_FILE", queue, s("SJC$_FILE_SPECIFICATION", okFile), flag("SJC$_HOLD"),
			s("SJC$_JOB_NAME", "QJOB"), s("SJC$_PARAMETER_1", "P1VAL"), s("SJC$_NOTE", "QNOTE"),
			s("SJC$_LOG_SPECIFICATION", logs+"QJOB.LOG"), flag("SJC$_NO_LOG_SPOOL"),
			saveOut("SJC$_ENTRY_NUMBER_OUTPUT", "QENTRY"), long("SJC$_JOB_STATUS_OUTPUT")),
		gq("DISPLAY_ENTRY", "DISPLAY_ENTRY", append([]item{cell("QUI$_SEARCH_NUMBER", "QENTRY")},
			outs("ENTRY_NUMBER", "JOB_NAME", "JOB_STATUS", "JOB_FLAGS", "USERNAME", "ACCOUNT_NAME", "UIC",
				"AFTER_TIME", "SUBMISSION_TIME", "PARAMETER_1", "NOTE", "LOG_SPECIFICATION", "PRIORITY",
				"QUEUE_NAME", "FILE_COUNT", "CPU_LIMIT", "PENDING_JOB_REASON", "CONDITION_VECTOR",
				"JOB_PID", "WSDEFAULT", "WSQUOTA", "WSEXTENT", "LOG_QUEUE")...)...),
		gq("DISPLAY_FILE", "DISPLAY_FILE", outs("FILE_SPECIFICATION", "FILE_FLAGS", "FILE_STATUS")...),
		gq("CANCEL_OPERATION", "CANCEL_OPERATION"),
		sj("ALTER_JOB", "ALTER_JOB", queue, entry, s("SJC$_NOTE", "CHANGED"), s("SJC$_PARAMETER_2", "P2VAL")),
		gq("DISPLAY_ENTRY after ALTER_JOB", "DISPLAY_ENTRY", append([]item{cell("QUI$_SEARCH_NUMBER", "QENTRY")},
			outs("NOTE", "PARAMETER_1", "PARAMETER_2", "JOB_STATUS")...)...),
		// SYNCHRONIZE_JOB waits for the job to end, so it is made only if
		// the job was released: a held job never ends.
		qcase{title: "ALTER_JOB NO_HOLD", fn: "SJC$_ALTER_JOB", keep: "RELEASED",
			items: []item{queue, entry, flag("SJC$_NO_HOLD")}},
		qcase{title: "SYNCHRONIZE_JOB", fn: "SJC$_SYNCHRONIZE_JOB", onlyIf: "RELEASED",
			items: []item{queue, entry}},
		gq("DISPLAY_ENTRY after the job", "DISPLAY_ENTRY", cell("QUI$_SEARCH_NUMBER", "QENTRY"), out("QUI$_JOB_STATUS")),
		sj("ENTER_FILE with no file", "ENTER_FILE", queue),
		sj("ENTER_FILE to a missing queue", "ENTER_FILE", s("SJC$_QUEUE", "NOSUCHQ98"), s("SJC$_FILE_SPECIFICATION", okFile)),
		qcase{title: "ENTER_FILE to a missing queue, no IOSB", fn: "SJC$_ENTER_FILE", noIOSB: true,
			items: []item{s("SJC$_QUEUE", "NOSUCHQ98"), s("SJC$_FILE_SPECIFICATION", okFile)}},
		sj("ENTER_FILE of a missing file", "ENTER_FILE", queue, s("SJC$_FILE_SPECIFICATION", top+"NOSUCH98.COM")),
		sj("DELETE_JOB of a missing entry", "DELETE_JOB", queue, l("SJC$_ENTRY_NUMBER", "99999")),
		qcase{title: "SNDJBC function 9999", fn: "9999", items: []item{queue}},
		qcase{title: "GETQUI function 9999", getqui: true, fn: "9999"},
		sj("START_QUEUE with item 9999", "START_QUEUE", queue, l("9999", "0")),
		sj("CREATE_QUEUE with a 32-character name", "CREATE_QUEUE",
			s("SJC$_QUEUE", "BAT98_QUEUE_NAME_OF_32_CHARACTER"), flag("SJC$_BATCH")),
		// Round 2 runs after the probe's accounts are gone: SYSTEM names
		// itself.
		sj("ENTER_FILE held, by USERNAME", "ENTER_FILE", queue, s("SJC$_FILE_SPECIFICATION", okFile), flag("SJC$_HOLD"),
			s("SJC$_USERNAME", "SYSTEM"), saveOut("SJC$_ENTRY_NUMBER_OUTPUT", "QE2")),
		sj("ENTER_FILE held, second", "ENTER_FILE", queue, s("SJC$_FILE_SPECIFICATION", okFile), flag("SJC$_HOLD"),
			s("SJC$_JOB_NAME", "QJOB3"), saveOut("SJC$_ENTRY_NUMBER_OUTPUT", "QE3")),
		gq("CANCEL_OPERATION", "CANCEL_OPERATION"),
	)

	for i := 1; i <= 4; i++ {
		add(gq(fmt.Sprintf("DISPLAY_QUEUE BAT98_Q* (%d)", i), "DISPLAY_QUEUE",
			l("QUI$_SEARCH_FLAGS", "QUI$M_SEARCH_WILDCARD"), s("QUI$_SEARCH_NAME", "BAT98_Q*"), out("QUI$_QUEUE_NAME")))
	}

	add(
		gq("CANCEL_OPERATION", "CANCEL_OPERATION"),
		gq("DISPLAY_QUEUE BAT98_Q, for its jobs", "DISPLAY_QUEUE",
			l("QUI$_SEARCH_FLAGS", "QUI$M_SEARCH_WILDCARD"), s("QUI$_SEARCH_NAME", "BAT98_Q"), out("QUI$_QUEUE_NAME")),
	)

	for i := 1; i <= 4; i++ {
		add(gq(fmt.Sprintf("DISPLAY_JOB (%d)", i), "DISPLAY_JOB", append([]item{l("QUI$_SEARCH_FLAGS", "QUI$M_SEARCH_ALL_JOBS")},
			outs("ENTRY_NUMBER", "JOB_NAME", "USERNAME", "JOB_STATUS")...)...))
	}

	short := out("QUI$_QUEUE_NAME")
	short.size = 3

	add(
		gq("CANCEL_OPERATION", "CANCEL_OPERATION"),
		gq("DISPLAY_QUEUE with a 3-byte buffer", "DISPLAY_QUEUE", s("QUI$_SEARCH_NAME", "BAT98_Q"), short),
		gq("DISPLAY_QUEUE with no name", "DISPLAY_QUEUE", out("QUI$_QUEUE_NAME")),
		gq("DISPLAY_QUEUE of a missing queue", "DISPLAY_QUEUE", s("QUI$_SEARCH_NAME", "NOSUCHQ98"), out("QUI$_QUEUE_NAME")),
		gq("TRANSLATE_QUEUE SYS$BATCH", "TRANSLATE_QUEUE", s("QUI$_SEARCH_NAME", "SYS$BATCH"), out("QUI$_QUEUE_NAME")),
		sj("DELETE_JOB of the USERNAME job", "DELETE_JOB", queue, cell("SJC$_ENTRY_NUMBER", "QE2")),
		sj("DELETE_JOB of the second", "DELETE_JOB", queue, cell("SJC$_ENTRY_NUMBER", "QE3")),
		sj("CREATE_JOB", "CREATE_JOB", queue, flag("SJC$_HOLD"), s("SJC$_JOB_NAME", "MULTI"),
			saveOut("SJC$_ENTRY_NUMBER_OUTPUT", "QE4")),
		sj("ADD_FILE", "ADD_FILE", s("SJC$_FILE_SPECIFICATION", okFile)),
		sj("ADD_FILE, second", "ADD_FILE", s("SJC$_FILE_SPECIFICATION", wrnFile)),
		sj("CLOSE_JOB", "CLOSE_JOB"),
		gq("DISPLAY_ENTRY of the closed job", "DISPLAY_ENTRY", cell("QUI$_SEARCH_NUMBER", "QE4"),
			out("QUI$_JOB_NAME"), out("QUI$_FILE_COUNT"), out("QUI$_JOB_STATUS")),
		sj("CLOSE_JOB with no job open", "CLOSE_JOB"),
		sj("DELETE_JOB of the closed job", "DELETE_JOB", queue, cell("SJC$_ENTRY_NUMBER", "QE4")),
		sj("DELETE_QUEUE of a started queue", "DELETE_QUEUE", queue),
		sj("STOP_QUEUE", "STOP_QUEUE", queue),
		sj("RESET_QUEUE", "RESET_QUEUE", queue),
		sj("ALTER_QUEUE", "ALTER_QUEUE", queue, l("SJC$_JOB_LIMIT", "4"), s("SJC$_QUEUE_DESCRIPTION", "ALTERED")),
		gq("DISPLAY_QUEUE after ALTER_QUEUE", "DISPLAY_QUEUE", append([]item{s("QUI$_SEARCH_NAME", "BAT98_Q")},
			outs("JOB_LIMIT", "QUEUE_DESCRIPTION", "QUEUE_STATUS")...)...),
		sj("DELETE_QUEUE", "DELETE_QUEUE", queue),
		sj("DELETE_QUEUE again", "DELETE_QUEUE", queue),
		sj("DELETE_QUEUE BAT98_Q2", "DELETE_QUEUE", s("SJC$_QUEUE", "BAT98_Q2")),
		sj("STOP_QUEUE of a missing queue", "STOP_QUEUE", queue),
	)

	return list
}

func asciiText(text string) string {
	if strings.ContainsRune(text, '|') {
		log.Fatalf("text %q has the delimiter", text)
	}

	return "|" + text + "|"
}

func program() string {
	var data, code strings.Builder

	data.WriteString(`	.TITLE	BAT98Q	Phase 98's probe of $SNDJBC and $GETQUI
	.IDENT	/V1.0/
;
; Written by testdata/bat98/gen.go: do not edit. Each case calls
; $SNDJBCW or $GETQUIW with an item list, and shows R0 and the IOSB's
; status (each with its message) and every output item: the length the
; service returned and the bytes, in hex and as text. At the end it lists
; the JBC facility's messages by $GETMSG. BAT98.COM runs it as SYSTEM;
; it works on queues of its own, BAT98_Q and BAT98_Q2.
;
	$SSDEF
	$SJCDEF
	$QUIDEF
	$JBCMSGDEF

; SAY text, a1, ... a10: format text with $FAOL and the arguments (each
; a longword, pushed by value) and write the line.
	.MACRO	SAY	TEXT,A1,A2,A3,A4,A5,A6,A7,A8,A9,A10,?LBL,?OVER
	BRW	OVER
LBL:	.ASCID	TEXT
OVER:
	$$N = 1
	.IRP	ARG,<A10,A9,A8,A7,A6,A5,A4,A3,A2,A1>
	.IF	NOT_BLANK,<ARG>
	PUSHL	ARG
	$$N = $$N + 1
	.ENDC
	.ENDR
	PUSHAQ	LBL
	CALLS	#$$N,SAYR
	.ENDM	SAY

	.PSECT	DATA,WRT,NOEXE,LONG
IOSB:	.QUAD	0
QENTRY:	.LONG	0		; the first job's entry
QE2:	.LONG	0		; BAT98's held job
QE3:	.LONG	0		; the second held job
QE4:	.LONG	0		; the job made by CREATE_JOB
RELEASED: .LONG	0		; ALTER_JOB NO_HOLD's IOSB status
OUTD:	.LONG	300
	.ADDRESS OUTBUF
OUTLEN:	.WORD	0
OUTBUF:	.BLKB	300
MSGD:	.LONG	256
	.ADDRESS MSGBUF
MSGLEN:	.WORD	0
MSGBUF:	.BLKB	256
HEXDIG:	.ASCII	/0123456789ABCDEF/
HEXLINE: .BLKB	64
ASCLINE: .BLKB	32
`)

	code.WriteString(`
	.PSECT	CODE,NOWRT,EXE,LONG
`)

	cases := qcases()

	for n, k := range cases {
		c := n + 1
		pre := fmt.Sprintf("C%d_", c)
		svc := "SNDJBCW"

		if k.getqui {
			svc = "GETQUIW"
		}

		title := fmt.Sprintf("== %d %s %s", c, svc, k.title)

		fmt.Fprintf(&data, "\n; %d: %s\n%sT:\t.ASCID\t%s\n", c, k.title, pre, asciiText(title))

		for i, it := range k.items {
			b := fmt.Sprintf("%sB%d", pre, i+1)

			switch it.kind {
			case 's':
				fmt.Fprintf(&data, "%s:\t.ASCII\t%s\n", b, asciiText(it.arg))
			case 'l':
				fmt.Fprintf(&data, "\t.ALIGN\tLONG\n%s:\t.LONG\t%s\n", b, it.arg)
			case 'o':
				size := it.size
				if size == 0 {
					size = 256
				}

				fmt.Fprintf(&data, "\t.ALIGN\tLONG\n%s:\t.BLKB\t%d\n%sR%d:\t.WORD\t0\n%sN%d:\t.ASCID\t/%s/\n",
					b, size, pre, i+1, pre, i+1, it.code)
			}
		}

		fmt.Fprintf(&data, "\t.ALIGN\tLONG\n%sIL:\n", pre)

		for i, it := range k.items {
			b := fmt.Sprintf("%sB%d", pre, i+1)

			switch it.kind {
			case 's':
				fmt.Fprintf(&data, "\t.WORD\t%d,%s\n\t.ADDRESS %s\n\t.LONG\t0\n", len(it.arg), it.code, b)
			case 'l':
				fmt.Fprintf(&data, "\t.WORD\t4,%s\n\t.ADDRESS %s\n\t.LONG\t0\n", it.code, b)
			case 'c':
				fmt.Fprintf(&data, "\t.WORD\t4,%s\n\t.ADDRESS %s\n\t.LONG\t0\n", it.code, it.arg)
			case 'b':
				fmt.Fprintf(&data, "\t.WORD\t0,%s\n\t.LONG\t0,0\n", it.code)
			case 'o':
				size := it.size
				if size == 0 {
					size = 256
				}

				fmt.Fprintf(&data, "\t.WORD\t%d,%s\n\t.ADDRESS %s\n\t.ADDRESS %sR%d\n", size, it.code, b, pre, i+1)
			}
		}

		data.WriteString("\t.LONG\t0\n")

		fmt.Fprintf(&code, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9>\n", c, k.title, c)
		fmt.Fprintf(&code, "\tPUSHAQ\t%sT\n\tCALLS\t#1,SAYR\n", pre)

		if k.onlyIf != "" {
			fmt.Fprintf(&code, "\tBLBS\t%s,1$\n\tSAY\t</  passed over: %s failed/>\n\tRET\n1$:\n", k.onlyIf, k.onlyIf)
		}
		code.WriteString("\tCLRQ\tIOSB\n\tCLRL\t-(SP)\t\t; astprm\n\tCLRL\t-(SP)\t\t; astadr\n")

		if k.noIOSB {
			code.WriteString("\tCLRL\t-(SP)\t\t; no iosb\n")
		} else {
			code.WriteString("\tPUSHAQ\tIOSB\n")
		}

		if len(k.items) == 0 {
			code.WriteString("\tCLRL\t-(SP)\t\t; no item list\n")
		} else {
			fmt.Fprintf(&code, "\tPUSHAB\t%sIL\n", pre)
		}

		code.WriteString("\tCLRL\t-(SP)\t\t; nullarg or context\n")
		fmt.Fprintf(&code, "\tPUSHL\t#%s\n\tCLRL\t-(SP)\t\t; efn\n\tCALLS\t#7,G^SYS$%s\n", k.fn, svc)
		code.WriteString("\tPUSHL\tR0\n\tCALLS\t#1,SHOWST\n")

		if k.keep != "" {
			fmt.Fprintf(&code, "\tMOVL\tIOSB,%s\n", k.keep)
		}

		for i, it := range k.items {
			if it.kind != 'o' {
				continue
			}

			fmt.Fprintf(&code, "\tPUSHAW\t%sR%d\n\tPUSHAB\t%sB%d\n\tPUSHAQ\t%sN%d\n\tCALLS\t#3,SHOWITEM\n",
				pre, i+1, pre, i+1, pre, i+1)

			if it.save != "" {
				fmt.Fprintf(&code, "\tMOVL\t%sB%d,%s\n", pre, i+1, it.save)
			}
		}

		code.WriteString("\tRET\n")
	}

	code.WriteString(`
; The program: each case, then the messages.
	.ENTRY	START,^M<>
`)

	for n := range cases {
		fmt.Fprintf(&code, "\tCALLS\t#0,C%d\n", n+1)
	}

	code.WriteString(`	CALLS	#0,MESSAGES
	SAY	</BAT98Q FINISHED/>
	MOVL	#SS$_NORMAL,R0
	RET

; SAYR text, a1, ...: format and write one line.
	.ENTRY	SAYR,^M<R2,R3,R4,R5>
	MOVL	#300,OUTD
	$FAOL_S	CTRSTR=@4(AP),OUTLEN=OUTLEN,OUTBUF=OUTD,PRMLST=8(AP)
	MOVZWL	OUTLEN,OUTD
	PUSHAQ	OUTD
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#300,OUTD
	RET

; MSGTEXT status: its message in MSGBUF, MSGLEN long (0 if none); R0 is
; $GETMSG's status.
	.ENTRY	MSGTEXT,^M<>
	MOVL	#256,MSGD
	CLRW	MSGLEN
	CLRL	-(SP)			; outadr
	PUSHL	#15			; flags: every part
	PUSHAQ	MSGD
	PUSHAW	MSGLEN
	PUSHL	4(AP)			; msgid
	CALLS	#5,G^SYS$GETMSG
	BLBS	R0,1$
	CLRW	MSGLEN
1$:	RET

; SHOWST r0: the service's R0 and the IOSB's status, each with its message.
	.ENTRY	SHOWST,^M<R2>
	PUSHL	4(AP)
	CALLS	#1,MSGTEXT
	MOVZWL	MSGLEN,R2
	SAY	</  R0   !XL !AD/>,4(AP),R2,#MSGBUF
	PUSHL	IOSB
	CALLS	#1,MSGTEXT
	MOVZWL	MSGLEN,R2
	SAY	</  IOSB !XL !XL !AD/>,IOSB,IOSB+4,R2,#MSGBUF
	RET

; SHOWITEM name, buffer, length word: an output item's returned length and
; its bytes (at most 256), 16 to a line, in hex and as text.
	.ENTRY	SHOWITEM,^M<R2,R3,R4,R5,R6,R7,R8,R9>
	MOVZWL	@12(AP),R2
	SAY	</  !AS len=!UL/>,4(AP),R2
	MOVL	8(AP),R3
	CMPL	R2,#256
	BLEQ	1$
	MOVL	#256,R2
1$:	TSTL	R2
	BNEQ	2$
	BRW	9$
2$:	MOVL	R2,R4			; this line's bytes
	CMPL	R4,#16
	BLEQ	3$
	MOVL	#16,R4
3$:	MOVAB	HEXLINE,R5
	MOVAB	ASCLINE,R7
	MOVL	R4,R6
4$:	MOVZBL	(R3)+,R8
	EXTZV	#4,#4,R8,R9
	MOVB	HEXDIG[R9],(R5)+
	EXTZV	#0,#4,R8,R9
	MOVB	HEXDIG[R9],(R5)+
	MOVB	#^A/ /,(R5)+
	CMPB	R8,#^A/ /
	BLSSU	5$
	CMPB	R8,#^A/~/
	BGTRU	5$
	MOVB	R8,(R7)+
	BRB	6$
5$:	MOVB	#^A/./,(R7)+
6$:	SOBGTR	R6,4$
	MOVAB	HEXLINE,R8
	SUBL3	R8,R5,R8
	MOVAB	ASCLINE,R9
	SUBL3	R9,R7,R9
	SAY	</    !48<!AD!> |!AD|/>,R8,#HEXLINE,R9,#ASCLINE
	SUBL2	R4,R2
	BLEQ	9$
	BRW	2$
9$:	RET

; MESSAGES: every message of the JBC facility $GETMSG knows, by message
; number (0 to 1023) and severity (0 to 7).
	.ENTRY	MESSAGES,^M<R2,R3,R4,R5,R6>
	SAY	</######## JBC MESSAGES/>
	MOVL	#JBC$_NORMAL,R2
	BICL2	#^XF000FFFF,R2		; the facility, bits 16 to 27
	SAY	</  JBC$_NORMAL !XL/>,#JBC$_NORMAL
	CLRL	R3
1$:	CLRL	R4
2$:	ASHL	#3,R3,R5
	BISL2	R2,R5
	BISL2	R4,R5
	PUSHL	R5
	CALLS	#1,MSGTEXT
	CMPL	R0,#SS$_NORMAL
	BNEQ	3$
	MOVZWL	MSGLEN,R6
	SAY	</  !XL !AD/>,R5,R6,#MSGBUF
3$:	AOBLEQ	#7,R4,2$
	AOBLEQ	#1023,R3,1$
	RET

	.END	START
`)

	return data.String() + code.String()
}

func defProc(macro string) string {
	return fmt.Sprintf(`	.TITLE	DEF_%s	$%sDEF's values, in the object's global symbols
; Written by testdata/bat98/gen.go.
	$%sDEF	GLOBAL
	.END
`, macro, macro, macro)
}

// ---------------------------------------------------------------------
// The console scripts.

func exchange() string {
	var b strings.Builder

	b.WriteString(`! EXCHANGE.CMD - builds Phase 98's probe exchange volume with govax.
! Written by testdata/bat98/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/bat98-exchange.dsk" /DEVICE=RD53 BAT98
MOUNT/WRITE DUA5 "testdata/disks/bat98-exchange.dsk"
CREATE/DIRECTORY DUA5:[BAT98LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[BAT98SCR]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
`)

	for _, a := range accounts {
		fmt.Fprintf(&b, "CREATE/DIRECTORY DUA5:%s\n", a.home)
	}

	for _, f := range files {
		fmt.Fprintf(&b, "COPY \"%s/%s\"/HOST DUA5:[000000]%s\n", dir, f, strings.ToUpper(f))
	}

	for _, a := range accounts {
		fmt.Fprintf(&b, "COPY \"%s/login_%s.com\"/HOST DUA5:%sLOGIN.COM\n", dir, strings.ToLower(a.name), a.home)
	}

	b.WriteString("DIRECTORY/OWNER/PROTECTION DUA5:[000000...]\nDISMOUNT DUA5\n")

	return b.String()
}

// copyout is a round's copyout script, script, bringing back names (the
// round's own log first) and, with objects, the definition probes' objects.
func copyout(script string, names []string, objects bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, `! %s - copies Phase 98's probe results off bat98-exchange.dsk
! into testdata/bat98/vax (README.md). Written by testdata/bat98/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/%s
!
! A file the run didn't make is a message, and the rest go on.
MOUNT DUA5 "testdata/disks/bat98-exchange.dsk"
DIRECTORY DUA5:[BAT98LOG],[BAT98SCR],[000000]*.OBJ
`, strings.ToUpper(script), script)

	seen := map[string]bool{}

	for _, n := range names {
		if seen[n] {
			continue
		}

		seen[n] = true
		fmt.Fprintf(&b, "COPY DUA5:[BAT98LOG]%s \"%s/vax/%s\"/HOST/QUIET\n", n, dir, strings.ToLower(n))
	}

	for _, m := range []string{"SJC", "QUI", "JBC"} {
		if !objects {
			break
		}

		fmt.Fprintf(&b, "COPY DUA5:[000000]DEF_%s.OBJ \"%s/vax/def_%s.obj\"/HOST/BINARY/QUIET\n", m, dir, strings.ToLower(m))
	}

	b.WriteString("DISMOUNT DUA5\n")

	return b.String()
}

// ---------------------------------------------------------------------

// longestLine is the longest line a file may have: Phase 95's probe found
// DCL refusing a 504-byte procedure record, which ended the procedure.
const longestLine = 200

func write(name, text string) {
	for i, line := range strings.Split(text, "\n") {
		if len(line) > longestLine {
			log.Fatalf("%s:%d: %d characters, more than %d", name, i+1, len(line), longestLine)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}

// exchangeTwo adds round 2's files to round 1's volume, which still has
// round 1's logs.
func exchangeTwo() string {
	return `! EXCHANGE2.CMD - adds round 2's files to Phase 98's probe exchange
! volume (README.md, "Round 2"). Written by testdata/bat98/gen.go. Run from
! the repository root, with the volume detached from the VAX:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/bat98-exchange.dsk"
COPY "` + dir + `/bat98b.com"/HOST DUA5:[000000]BAT98B.COM
COPY "` + dir + `/bat98q.mar"/HOST DUA5:[000000]BAT98Q.MAR
DIRECTORY/OWNER/PROTECTION DUA5:[000000]BAT98B.COM,BAT98Q.MAR
DISMOUNT DUA5
`
}

func main() {
	jobLogs = nil
	write("bat98.com", systemProc())
	roundOne := append([]string{"BAT98.LOG"}, jobLogs...)

	jobLogs = nil
	write("bat98b.com", roundTwoProc())
	// The logs of the README's steps by hand come back too, if they ran.
	roundTwo := append([]string{"BAT98B.LOG"}, jobLogs...)

	write("bat98z.com", dropQueues())
	write("bat98acct.com", accountProc())
	write("bat98drop.com", dropProc())
	write("bat98j.com", viewProc())
	write("bat98p.com", matrixProc())
	write("bat98ok.com", endsWith("BAT98OK", "1", "a procedure that succeeds"))
	write("bat98wrn.com", endsWith("BAT98WRN", "%X10000000", "a procedure that ends with a warning"))
	write("bat98err.com", endsWith("BAT98ERR", "%X00018292", "a procedure that ends with an error (RMS-E-FNF)"))
	write("bat98fat.com", endsWith("BAT98FAT", "%X0000002C", "a procedure that ends with a severe error (SS$_ABORT)"))
	write("bat98bad.com", badProc())
	write("bat98w.com", waiterProc())
	write("bat98c.com", cpuProc())
	write("bat98o.com", orderProc())
	write("bat98r.com", restartProc())

	for _, a := range accounts {
		write("login_"+strings.ToLower(a.name)+".com", loginProc(a.name))
	}

	write("bat98q.mar", program())

	for _, m := range []string{"SJC", "QUI", "JBCMSG"} {
		name := strings.TrimSuffix(strings.ToLower(m), "msg")
		write("def_"+name+".mar", defProc(m))
	}

	write("exchange.cmd", exchange())
	write("copyout.cmd", copyout("copyout.cmd", roundOne, false))
	write("exchange2.cmd", exchangeTwo())
	write("copyout2.cmd", copyout("copyout2.cmd", roundTwo, false))

	fmt.Printf("%d cases in BAT98.COM, %d in BAT98B.COM, %d in BAT98P.COM's pass, %d in BAT98Q\n",
		lastCase, lastCaseB, len(passCommands), len(qcases()))
}

// lastCase is BAT98.COM's case count, for main's report.
var lastCase int
