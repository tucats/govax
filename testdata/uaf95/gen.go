//go:build ignore

// gen writes Phase 95's probe of authentication (README.md;
// docs/planning/PHASE-95 - authentication.md, Part D). It asks VMS 7.1
// three things:
//
//   - AUTHORIZE's words: every command govax will have, with good and bad
//     arguments, its reports (SHOW, LIST), and its messages. uaf95.com runs
//     them against a scratch SYSUAF.DAT and RIGHTSLIST.DAT on the exchange
//     volume, named by process logical names, so the VAX's own files are
//     never changed by them.
//   - what logging in does: the probe's accounts (made in the VAX's own
//     SYSUAF by uaf95acct.com, removed by uaf95drop.com) log in by batch
//     job and by hand, and the probe's SYLOGIN, LOGIN.COM, and LGICMD
//     procedures record when they ran and what they saw; uaf95i.com then
//     shows the process: its logical names, default, privileges, rights,
//     and every F$GETJPI item.
//   - what an account without SYSPRV gets from AUTHORIZE.
//
// Run it from the repository root:
//
//	go run testdata/uaf95/gen.go
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/uaf95"

// The probe's accounts, in the VAX's own SYSUAF while the probe runs.
// Group 200 was Phase 94's (PRIV94 and PRIV94N, removed at its end).
const (
	password = "UAF95PW"
	scratch  = "UAF95$DISK:[UAF95SCR]"
	home     = "UAF95$DISK:[UAF95]"
	logs     = "UAF95$DISK:[UAF95LOG]"
)

// files are the procedures exchange.cmd copies to the volume's [000000],
// and homeFiles those it copies to [UAF95], the accounts' home.
var (
	files     = []string{"uaf95.com", "uaf95acct.com", "uaf95drop.com", "uaf95i.com", "uaf95s.com"}
	homeFiles = []string{"login.com", "uaf95x.com"}
)

// ---------------------------------------------------------------------
// The AUTHORIZE cases, run one command at a time against the scratch
// files. Each is "$ MCR AUTHORIZE command" (AUTHORIZE takes one command on
// its command line and exits), then its $STATUS, so each case's messages
// and status stand alone. Verification shows each command line.

// section is a heading in the log and the commands under it.
type section struct {
	title    string
	commands []string
}

var sections = []section{
	{"THE NEW FILE'S RECORDS", []string{
		"SHOW DEFAULT",
		"SHOW SYSTEM",
		"SHOW SYSTEM/BRIEF",
		"SHOW SYSTEM/FULL",
		"SHOW */BRIEF",
		"SHOW/IDENTIFIER *",
		"SHOW/IDENTIFIER/FULL *",
	}},
	{"ADD", []string{
		"ADD UAFA/UIC=[300,1]",
		"SHOW UAFA",
		"SHOW/IDENTIFIER UAFA",
		"ADD UAFA/UIC=[300,2]",
		"ADD UAFB",
		"ADD TOOLONGUSERNAM/UIC=[300,3]",
		"ADD BAD-NAME/UIC=[300,3]",
		"ADD UAFC/UIC=[300]",
		"ADD UAFC/UIC=[400000,1]",
		"ADD UAFC/UIC=[300,1]",
		"SHOW UAFC/BRIEF",
		"ADD UAFQ/UIC=[300,8]",
		"ADD UAFN/UIC=[300,13]/PASSWORD=UAFNPW1",
		"SHOW UAFN",
		// Round 1's single ADD with every qualifier was 504 characters,
		// more than DCL reads as a procedure's record (RMS-W-RTB ended the
		// procedure); so ADD takes some, and MODIFYs the rest.
		"ADD UAFD/UIC=[300,4]/OWNER=\"GOVAX CASE D\"/ACCOUNT=CASED/DEVICE=DUA1:/DIRECTORY=[UAFD]" +
			"/CLI=DCL/LGICMD=MYLOGIN/PRIORITY=3/PASSWORD=UAFDPW1",
		"MODIFY UAFD/FLAGS=(DISUSER,DISCTLY,DEFCLI,LOCKPWD,CAPTIVE,DISNEWMAIL,DISWELCOME)" +
			"/PRIVILEGES=(TMPMBX,NETMBX,SYSPRV)/DEFPRIVILEGES=(TMPMBX,NETMBX)",
		"MODIFY UAFD/ASTLM=31/BIOLM=32/BYTLM=33000/DIOLM=34/ENQLM=35/FILLM=36/JTQUOTA=3700" +
			"/PGFLQUOTA=38000/PRCLM=9/TQELM=11",
		"MODIFY UAFD/WSDEFAULT=301/WSQUOTA=601/WSEXTENT=1201/MAXJOBS=2/MAXACCTJOBS=3" +
			"/MAXDETACH=4/SHRFILLM=5/QUEPRIO=6/CPUTIME=0-00:10:00",
		"SHOW UAFD",
		"SHOW UAFD/FULL",
		"SHOW UAFD/BRIEF",
		"ADD UAFE/UIC=[300,5]/PRIVILEGES=NOSUCHPRIV",
		"ADD UAFE/UIC=[300,5]/FLAGS=NOSUCHFLAG",
		"ADD UAFE/UIC=[300,5]/PRIORITY=99",
		"ADD UAFE/UIC=[300,5]/CLI=NOSUCHCLI",
		"ADD UAFE/UIC=[300,5]/PRIVILEGES=ALL/DEFPRIVILEGES=NOALL",
		"SHOW UAFE",
	}},
	{"MODIFY", []string{
		"MODIFY UAFD/UIC=[300,44]",
		"MODIFY UAFD/OWNER=\"NEW OWNER\"/ACCOUNT=NEWACCT",
		"MODIFY UAFD/DEVICE=DUA2:/DIRECTORY=[NEWDIR]",
		"MODIFY UAFD/FLAGS=NODISUSER",
		"MODIFY UAFD/FLAGS=(NOCAPTIVE,NOLOCKPWD)",
		"MODIFY UAFD/PRIVILEGES=(NOSYSPRV,OPER)",
		"MODIFY UAFD/DEFPRIVILEGES=OPER",
		"MODIFY UAFD/PRIORITY=5/WSQUOTA=700/ASTLM=41",
		"MODIFY UAFD/LGICMD=\"\"",
		"MODIFY UAFD/PASSWORD=UAFDPW2",
		"MODIFY UAFD/NOPASSWORD",
		"SHOW UAFD/FULL",
		"MODIFY UAF%/ASTLM=60",
		"MODIFY UAF*/BIOLM=61",
		"SHOW */BRIEF",
		"MODIFY NOSUCH/ASTLM=1",
		"MODIFY UAFD",
		"MODIFY UAFD/UIC=[300,1]",
		"SHOW UAFD/BRIEF",
	}},
	{"COPY, RENAME, AND REMOVE", []string{
		"COPY UAFD UAFF/UIC=[300,6]",
		"SHOW UAFF",
		"COPY UAFD UAFG",
		"COPY NOSUCH UAFH/UIC=[300,7]",
		"COPY UAFD UAFA/UIC=[300,14]",
		"RENAME UAFF UAFI",
		"SHOW UAFI/BRIEF",
		"SHOW/IDENTIFIER UAFI",
		"SHOW/IDENTIFIER UAFF",
		"RENAME UAFF UAFJ",
		"RENAME UAFI UAFA",
		"REMOVE UAFI",
		"SHOW/IDENTIFIER UAFI",
		"REMOVE UAFI",
		"REMOVE SYSTEM2",
	}},
	{"DEFAULT", []string{
		"DEFAULT/ASTLM=77/FLAGS=DISCTLY/DEVICE=DUA3:",
		"SHOW DEFAULT",
		"ADD UAFL/UIC=[300,10]",
		"SHOW UAFL",
		"DEFAULT/ASTLM=24/FLAGS=NODISCTLY/DEVICE=SYS$SYSDEVICE:",
		"MODIFY DEFAULT/BIOLM=18",
		"SHOW DEFAULT/BRIEF",
	}},
	{"IDENTIFIERS", []string{
		"ADD/IDENTIFIER PROJX",
		"ADD/IDENTIFIER PROJY/VALUE=IDENTIFIER=%X80011234/ATTRIBUTES=RESOURCE",
		"ADD/IDENTIFIER PROJX",
		"ADD/IDENTIFIER UAFUIC/VALUE=UIC=[300,77]",
		"ADD/IDENTIFIER PROJW/VALUE=IDENTIFIER=%X80011234",
		"ADD/IDENTIFIER BAD-ID",
		"ADD/IDENTIFIER 1PROJ",
		"SHOW/IDENTIFIER PROJX",
		"SHOW/IDENTIFIER/FULL PROJY",
		"SHOW/IDENTIFIER/VALUE=IDENTIFIER=%X80011234",
		"SHOW/IDENTIFIER/VALUE=UIC=[300,4]",
		"SHOW/IDENTIFIER PROJ*",
		"SHOW/IDENTIFIER NOSUCH",
		"GRANT/IDENTIFIER PROJX UAFD",
		"GRANT/IDENTIFIER PROJX UAFD",
		"GRANT/IDENTIFIER/ATTRIBUTES=RESOURCE PROJY UAFD",
		"GRANT/IDENTIFIER PROJX NOSUCH",
		"GRANT/IDENTIFIER NOSUCH UAFD",
		"GRANT/IDENTIFIER PROJX UAFA",
		"SHOW/RIGHTS UAFD",
		"SHOW/RIGHTS/FULL UAFD",
		"SHOW/IDENTIFIER/FULL PROJX",
		"SHOW UAFD/FULL",
		"REVOKE/IDENTIFIER PROJX UAFD",
		"REVOKE/IDENTIFIER PROJX UAFD",
		"SHOW/RIGHTS UAFD",
		"RENAME/IDENTIFIER PROJY PROJZ",
		"RENAME/IDENTIFIER PROJY PROJV",
		"RENAME/IDENTIFIER PROJZ PROJX",
		"MODIFY/IDENTIFIER PROJZ/VALUE=IDENTIFIER=%X80011235",
		"MODIFY/IDENTIFIER PROJZ/ATTRIBUTES=NORESOURCE",
		"MODIFY/IDENTIFIER PROJZ/NAME=PROJU",
		"SHOW/IDENTIFIER/FULL PROJU",
		"SHOW/RIGHTS UAFD",
		"REMOVE/IDENTIFIER PROJX",
		"SHOW/RIGHTS UAFA",
		"REMOVE/IDENTIFIER PROJX",
		"REMOVE/IDENTIFIER UAFD",
		"RENAME UAFD UAFM",
		"SHOW/IDENTIFIER UAFM",
		"SHOW/RIGHTS UAFM",
	}},
	{"LIST", []string{
		"LIST",
		"LIST/FULL UAF*",
		"LIST/BRIEF",
		"LIST/IDENTIFIER",
		"LIST/RIGHTS UAFM",
		"LIST/RIGHTS",
	}},
	{"WHAT GOVAX WON'T KEEP", []string{
		"ADD UAFO/UIC=[300,11]/ACCESS=(PRIMARY,9-17)/PRIMEDAYS=(NOSATURDAY)/PWDLIFETIME=30-/PWDMINIMUM=8/EXPIRATION=31-DEC-2030",
		"SHOW UAFO/FULL",
		"MODIFY UAFO/NOACCESS/PWDEXPIRED",
		"SHOW UAFO/BRIEF",
	}},
	{"BAD COMMANDS", []string{
		"FROB",
		"SHOW",
		"SHOW NOSUCH",
		"SHOW UAFA/BOGUS",
		"ADD",
		"ADD UAFP/UIC=[300,12]/ASTLM=ABC",
		"REMOVE",
		"SHOW UAF*",
	}},
}

// ---------------------------------------------------------------------
// uaf95.com, SYSTEM's run.

func systemProc() string {
	var b strings.Builder

	b.WriteString(`$ ! UAF95.COM - Phase 95's probe of authentication: SYSTEM's run.
$ ! Written by testdata/uaf95/gen.go. Run it as SYSTEM, from the exchange
$ ! volume, mounted /SYSTEM (README.md):
$ !
$ !     @DUA1:[000000]UAF95
$ !
$ ! It defines UAF95$DISK in the system table for the volume, runs
$ ! AUTHORIZE's cases against a scratch SYSUAF.DAT and RIGHTSLIST.DAT in
$ ! [UAF95SCR], makes the probe's accounts (UAF95ACCT.COM), and runs the
$ ! batch jobs that log in as them. Its log is UAF95$DISK:[UAF95LOG]UAF95.LOG.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ DEFINE/SYSTEM/EXECUTIVE_MODE UAF95$DISK 'DEV'
$ LOGFILE = DEV + "[UAF95LOG]UAF95.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "UAF95 finished; the log is ", LOGFILE
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ !
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME"), " ", F$GETSYI("NODENAME")
$ WRITE SYS$OUTPUT "MAXSYSGROUP ", F$GETSYI("MAXSYSGROUP")
$ WRITE SYS$OUTPUT F$GETJPI("","USERNAME"), " ", F$GETJPI("","UIC")
$ SHOW LOGICAL UAF95$DISK
$ SHOW LOGICAL/FULL SYSUAF
$ SHOW LOGICAL/FULL RIGHTSLIST
$ SHOW LOGICAL/FULL SYS$SYLOGIN
$ SHOW LOGICAL/FULL SYS$LOGIN
$ SHOW LOGICAL/FULL SYS$LOGIN_DEVICE
$ SHOW LOGICAL/FULL SYS$SCRATCH
$ DIRECTORY/SECURITY SYS$SYSTEM:SYSUAF.DAT
$ DIRECTORY/SECURITY SYS$SYSTEM:RIGHTSLIST.DAT
$ DIRECTORY/SECURITY SYS$SYSTEM:AUTHORIZE.EXE
$ DIRECTORY/SECURITY SYS$MANAGER:SYLOGIN.COM
$ DIRECTORY/SECURITY SYS$SYSROOT:[000000]SYSMGR.DIR
$ DIRECTORY/SECURITY SYS$SYSROOT:[000000]SYSEXE.DIR
$ DIRECTORY/SECURITY SYS$SYSROOT:[000000]SYSLIB.DIR
$ DIRECTORY/SECURITY SYS$SYSROOT:[SYSLIB]STARLET.OLB
$ DIRECTORY/SECURITY SYS$SYSROOT:[SYSHLP]HELPLIB.HLB
$ DIRECTORY/SECURITY SYS$SYSDEVICE:[000000]SYS0.DIR
$ !
$ WRITE SYS$OUTPUT "######## VOLUME"
$ SET VOLUME/PROTECTION=(S:RWCD,O:RWCD,G:RWCD,W:RWCD) UAF95$DISK:
$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) UAF95$DISK:[000000]000000.DIR
$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) UAF95$DISK:[000000]UAF95LOG.DIR
$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) UAF95$DISK:[000000]*.COM;*
$ DIRECTORY/SECURITY UAF95$DISK:[000000]
$ !
$ ! The scratch files. With SYSUAF and RIGHTSLIST naming files that don't
$ ! exist yet, AUTHORIZE offers to make a new SYSUAF.DAT (the data line
$ ! YES answers it) holding its own DEFAULT and SYSTEM records, and
$ ! CREATE/RIGHTS makes RIGHTSLIST.DAT. The names are executive-mode
$ ! process names: round 1's supervisor-mode RIGHTSLIST was ignored, and
$ ! AUTHORIZE used the VAX's own rights database. Nothing below may run
$ ! unless both scratch files are there, so the VAX's are never changed.
$ !
$ WRITE SYS$OUTPUT "######## SCRATCH FILES"
$ SET DEFAULT `)
	b.WriteString(scratch)
	b.WriteString(`
$ DEFINE/PROCESS/EXECUTIVE_MODE SYSUAF ` + scratch + `SYSUAF.DAT
$ DEFINE/PROCESS/EXECUTIVE_MODE RIGHTSLIST ` + scratch + `RIGHTSLIST.DAT
$ DEFINE/PROCESS/EXECUTIVE_MODE NETPROXY ` + scratch + `NETPROXY.DAT
$ DEFINE/PROCESS/EXECUTIVE_MODE NET$PROXY ` + scratch + `NET$PROXY.DAT
$ SHOW LOGICAL/FULL SYSUAF
$ SHOW LOGICAL/FULL RIGHTSLIST
$ MCR AUTHORIZE
YES
CREATE/RIGHTS
SHOW DEFAULT/BRIEF
EXIT
$ SHOW SYMBOL $STATUS
$ IF F$SEARCH("` + scratch + `SYSUAF.DAT") .EQS. "" THEN GOTO NO_SCRATCH
$ IF F$SEARCH("` + scratch + `RIGHTSLIST.DAT") .EQS. "" THEN GOTO NO_SCRATCH
$ GOTO SCRATCH_MADE
$ NO_SCRATCH:
$ WRITE SYS$OUTPUT "NO SCRATCH SYSUAF.DAT OR RIGHTSLIST.DAT: AUTHORIZE'S CASES NOT RUN"
$ GOTO ACCOUNTS
$ SCRATCH_MADE:
$ DIRECTORY/FULL ` + scratch + `
$ !
$ ! A session: commands from the procedure's data lines, errors among
$ ! them, ended by EXIT; then one ended by the end of its data.
$ !
$ WRITE SYS$OUTPUT "######## A SESSION"
$ MCR AUTHORIZE
SHOW NOSUCH
FROB
ADD UAFS/UIC=[300,40]
SHOW UAFS/BRIEF
REMOVE UAFS
EXIT
$ SHOW SYMBOL $STATUS
$ MCR AUTHORIZE
SHOW SYSTEM/BRIEF
$ SHOW SYMBOL $STATUS
`)

	n := 0

	for _, s := range sections {
		fmt.Fprintf(&b, "$ !\n$ WRITE SYS$OUTPUT \"######## %s\"\n", s.title)

		for _, c := range s.commands {
			n++
			fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"== %d\"\n", n)
			fmt.Fprintf(&b, "$ MCR AUTHORIZE %s\n", c)
			b.WriteString("$ SHOW SYMBOL $STATUS\n")

			if strings.HasPrefix(c, "LIST") {
				// LIST writes SYSUAF.LIS (or RIGHTSLIST.LIS) in the default
				// directory, a new version each time.
				name := "SYSUAF.LIS"
				if strings.Contains(c, "/IDENTIFIER") || strings.Contains(c, "/RIGHTS") {
					name = "RIGHTSLIST.LIS"
				}

				fmt.Fprintf(&b, "$ TYPE %s%s\n", scratch, name)
			}
		}
	}

	b.WriteString(`$ !
$ WRITE SYS$OUTPUT "######## SCRATCH FILES AT THE END"
$ DIRECTORY/FULL ` + scratch + `
$ ACCOUNTS:
$ DEASSIGN/PROCESS/EXECUTIVE_MODE SYSUAF
$ DEASSIGN/PROCESS/EXECUTIVE_MODE RIGHTSLIST
$ DEASSIGN/PROCESS/EXECUTIVE_MODE NETPROXY
$ DEASSIGN/PROCESS/EXECUTIVE_MODE NET$PROXY
$ SET DEFAULT UAF95$DISK:[000000]
$ !
$ @UAF95$DISK:[000000]UAF95ACCT
$ !
$ ! The batch jobs: UAF95 (SYS$LOGIN:LOGIN.COM) and UAF95L (its LGICMD),
$ ! each running UAF95I.COM, which writes its own log. The batch logs
$ ! show the login itself. SYNCHRONIZE waits for each.
$ !
$ WRITE SYS$OUTPUT "######## BATCH"
$ SHOW QUEUE/BATCH/ALL
$ SUBMIT/USER=UAF95/NOPRINTER/KEEP/NAME=UAF95J/LOG_FILE=` + logs + `UAF95J_UAF95.LOG UAF95$DISK:[000000]UAF95I.COM
$ SHOW SYMBOL $STATUS
$ IF F$TYPE($ENTRY) .NES. "" THEN SYNCHRONIZE/ENTRY='$ENTRY'
$ SUBMIT/USER=UAF95L/NOPRINTER/KEEP/NAME=UAF95LJ/LOG_FILE=` + logs + `UAF95J_UAF95L.LOG UAF95$DISK:[000000]UAF95I.COM
$ SHOW SYMBOL $STATUS
$ IF F$TYPE($ENTRY) .NES. "" THEN SYNCHRONIZE/ENTRY='$ENTRY'
$ SUBMIT/USER=UAF95D/NOPRINTER/KEEP/NAME=UAF95DJ/LOG_FILE=` + logs + `UAF95J_UAF95D.LOG UAF95$DISK:[000000]UAF95I.COM
$ SHOW SYMBOL $STATUS
$ IF F$TYPE($ENTRY) .NES. "" THEN SYNCHRONIZE/ENTRY='$ENTRY'
$ DIRECTORY ` + logs + `
$ SET DEFAULT SYS$SYSTEM
$ MCR AUTHORIZE SHOW UAF95/FULL
$ MCR AUTHORIZE SHOW UAF95L/FULL
$ SET DEFAULT UAF95$DISK:[000000]
$ !
$ WRITE SYS$OUTPUT "######## STATE SYSTEM"
$ @UAF95$DISK:[000000]UAF95I NOLOG
$ WRITE SYS$OUTPUT "UAF95 FINISHED"
$ EXIT
`)

	return b.String()
}

// ---------------------------------------------------------------------
// uaf95acct.com, the accounts.

func accountProc() string {
	return `$ ! UAF95ACCT.COM - Phase 95's probe: the accounts, in the VAX's own
$ ! SYSUAF. Written by testdata/uaf95/gen.go. UAF95.COM runs it as SYSTEM;
$ ! UAF95DROP.COM undoes everything it does.
$ !
$ !   UAF95   [200,5]  TMPMBX, NETMBX; home UAF95$DISK:[UAF95], so its
$ !                    login runs SYS$LOGIN:LOGIN.COM
$ !   UAF95L  [200,7]  the same, with LGICMD UAF95$DISK:[UAF95]UAF95X.COM
$ !   UAF95D  [200,6]  the same, DISUSER
$ !
$ ! Each has the password ` + password + `, not expiring. UAF95 is granted the
$ ! identifier UAF95_ID. SYS$SYLOGIN is pointed at the probe's
$ ! UAF95$DISK:[000000]UAF95S.COM, which records what it sees for the
$ ! probe's accounts and runs the VAX's own SYLOGIN for everyone else;
$ ! the old translation is kept in UAF95$OLD_SYLOGIN for UAF95DROP.COM.
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## ACCOUNTS"
$ ! The VAX's own files: no scratch names left over.
$ IF F$TRNLNM("SYSUAF","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE SYSUAF
$ IF F$TRNLNM("RIGHTSLIST","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE RIGHTSLIST
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ MCR AUTHORIZE
ADD UAF95/UIC=[200,5]/PASSWORD=` + password + `/NOPWDEXPIRED/PWDLIFETIME=NONE/FLAGS=NODISUSER
MODIFY UAF95/DEVICE=UAF95$DISK:/DIRECTORY=[UAF95]/ACCOUNT=GOVAX/OWNER="GOVAX PROBE"
MODIFY UAF95/PRIVILEGES=(TMPMBX,NETMBX)/DEFPRIVILEGES=(TMPMBX,NETMBX)
COPY UAF95 UAF95L/UIC=[200,7]/PASSWORD=` + password + `/NOPWDEXPIRED
MODIFY UAF95L/LGICMD=UAF95$DISK:[UAF95]UAF95X.COM
COPY UAF95 UAF95D/UIC=[200,6]/PASSWORD=` + password + `/NOPWDEXPIRED/FLAGS=DISUSER
ADD/IDENTIFIER UAF95_ID
GRANT/IDENTIFIER UAF95_ID UAF95
SHOW UAF95/FULL
SHOW UAF95L/BRIEF
SHOW UAF95D/BRIEF
SHOW/RIGHTS UAF95
SHOW/IDENTIFIER/VALUE=UIC=[200,*]
EXIT
$ SET DEFAULT 'HOME'
$ !
$ ! The home directory, owned by UAF95, and its procedures.
$ SET SECURITY/OWNER=[200,5]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:) UAF95$DISK:[000000]UAF95.DIR
$ SET SECURITY/OWNER=[200,5]/PROTECTION=(S:RWED,O:RWED,G:RE,W:) UAF95$DISK:[UAF95]*.*;*
$ DIRECTORY/SECURITY UAF95$DISK:[UAF95]
$ !
$ ! SYS$SYLOGIN, kept and replaced at the mode it was defined in.
$ OLD = F$TRNLNM("SYS$SYLOGIN","LNM$SYSTEM_TABLE")
$ MODE = F$TRNLNM("SYS$SYLOGIN","LNM$SYSTEM_TABLE",,,,"ACCESS_MODE")
$ IF MODE .EQS. "" THEN MODE = "EXECUTIVE"
$ SHOW LOGICAL/FULL SYS$SYLOGIN
$ IF OLD .NES. "" THEN DEFINE/SYSTEM/EXECUTIVE_MODE UAF95$OLD_SYLOGIN "''OLD'"
$ DEFINE/SYSTEM/EXECUTIVE_MODE UAF95$OLD_SYLOGIN_MODE "''MODE'"
$ DEFINE/SYSTEM/'MODE'_MODE SYS$SYLOGIN UAF95$DISK:[000000]UAF95S.COM
$ SHOW LOGICAL/FULL SYS$SYLOGIN
$ SHOW LOGICAL/FULL UAF95$OLD_SYLOGIN*
$ EXIT
`
}

// ---------------------------------------------------------------------
// uaf95drop.com, the end.

func dropProc() string {
	return `$ ! UAF95DROP.COM - Phase 95's probe: the end. Written by
$ ! testdata/uaf95/gen.go. Run it as SYSTEM after the logins by hand
$ ! (README.md):
$ !
$ !     @UAF95$DISK:[000000]UAF95DROP
$ !
$ ! It shows the accounts' last logins, puts SYS$SYLOGIN back, removes
$ ! the accounts and UAF95_ID (and GOVAX, the group identifier ADD makes
$ ! for group 200 if the group has none), and deletes the scratch
$ ! SYSUAF.DAT and RIGHTSLIST.DAT. Its log is UAF95$DISK:[UAF95LOG]UAF95DROP.LOG.
$ ! It can be run more than once.
$ !
$ IF P1 .EQS. "LOGGED" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ DEV = F$PARSE(HERE,,,"DEVICE")
$ LOGFILE = DEV + "[UAF95LOG]UAF95DROP.LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "UAF95DROP finished; the log is ", LOGFILE
$ DEASSIGN/SYSTEM/EXECUTIVE_MODE UAF95$DISK
$ EXIT
$ BEGIN:
$ SET NOON
$ SET VERIFY
$ IF F$TRNLNM("SYSUAF","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE SYSUAF
$ IF F$TRNLNM("RIGHTSLIST","LNM$PROCESS") .NES. "" THEN DEASSIGN/PROCESS/EXECUTIVE_MODE RIGHTSLIST
$ !
$ WRITE SYS$OUTPUT "######## SYLOGIN"
$ SHOW LOGICAL/FULL SYS$SYLOGIN
$ OLD = F$TRNLNM("UAF95$OLD_SYLOGIN","LNM$SYSTEM_TABLE")
$ MODE = F$TRNLNM("UAF95$OLD_SYLOGIN_MODE","LNM$SYSTEM_TABLE")
$ IF MODE .EQS. "" THEN GOTO SYLOGIN_DONE
$ IF OLD .NES. "" THEN DEFINE/SYSTEM/'MODE'_MODE SYS$SYLOGIN "''OLD'"
$ IF OLD .EQS. "" THEN DEASSIGN/SYSTEM/'MODE'_MODE SYS$SYLOGIN
$ DEASSIGN/SYSTEM/EXECUTIVE_MODE UAF95$OLD_SYLOGIN_MODE
$ IF OLD .NES. "" THEN DEASSIGN/SYSTEM/EXECUTIVE_MODE UAF95$OLD_SYLOGIN
$ SYLOGIN_DONE:
$ SHOW LOGICAL/FULL SYS$SYLOGIN
$ !
$ WRITE SYS$OUTPUT "######## ACCOUNTS"
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT SYS$SYSTEM
$ MCR AUTHORIZE
SHOW UAF95/FULL
SHOW UAF95L/FULL
SHOW UAF95D/FULL
REMOVE UAF95
REMOVE UAF95L
REMOVE UAF95D
REMOVE/IDENTIFIER UAF95_ID
REMOVE/IDENTIFIER GOVAX
SHOW UAF95*
SHOW/IDENTIFIER/VALUE=UIC=[200,*]
EXIT
$ SET DEFAULT 'HOME'
$ !
$ WRITE SYS$OUTPUT "######## SCRATCH FILES"
$ DELETE ` + scratch + `*.DAT;*
$ DIRECTORY ` + scratch + `
$ DIRECTORY ` + logs + `
$ WRITE SYS$OUTPUT "UAF95DROP FINISHED"
$ EXIT
`
}

// ---------------------------------------------------------------------
// uaf95i.com, a process's state after its login.

// jpiItems are F$GETJPI's items, from the 7.3-1 DCL Dictionary's list.
// Those VMS 7.1 lacks fail, and the log shows how.
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

func stateProc() string {
	var b strings.Builder

	b.WriteString(`$ ! UAF95I.COM - Phase 95's probe: a process's state after logging in.
$ ! Written by testdata/uaf95/gen.go. The batch jobs run it, and each
$ ! account runs it by hand after logging in (README.md):
$ !
$ !     $ @UAF95$DISK:[000000]UAF95I
$ !
$ ! It writes UAF95$DISK:[UAF95LOG]UAF95I_user_mode.LOG (NOLOG: to
$ ! SYS$OUTPUT, as UAF95.COM uses it for SYSTEM).
$ !
$ IF P1 .EQS. "LOGGED" .OR. P1 .EQS. "NOLOG" THEN GOTO BEGIN
$ HERE = F$ENVIRONMENT("PROCEDURE")
$ WHO = F$EDIT(F$GETJPI("","USERNAME"),"TRIM")
$ LOGFILE = "` + logs + `UAF95I_" + WHO + "_" + F$MODE() + ".LOG"
$ @'HERE'/OUTPUT='LOGFILE' LOGGED
$ WRITE SYS$OUTPUT "UAF95I finished; the log is ", LOGFILE
$ EXIT
$ BEGIN:
$ SET NOON
$ STARTED_IN = F$ENVIRONMENT("DEFAULT")
$ WRITE SYS$OUTPUT "######## WHAT THE LOGIN PROCEDURES SAW"
$ SHOW SYMBOL/GLOBAL UAF95_*
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## PROCESS"
$ WRITE SYS$OUTPUT F$GETJPI("","USERNAME"), " ", F$GETJPI("","UIC"), " ", F$MODE()
$ SHOW DEFAULT
$ WRITE SYS$OUTPUT F$ENVIRONMENT("DEFAULT"), " ", F$ENVIRONMENT("PROTECTION")
$ SHOW PROCESS
$ SHOW PROCESS/PRIVILEGES
$ SHOW PROCESS/RIGHTS
$ SHOW PROCESS/QUOTAS
$ WRITE SYS$OUTPUT "######## LOGICAL NAMES"
$ SHOW LOGICAL/FULL SYS$LOGIN
$ SHOW LOGICAL/FULL SYS$LOGIN_DEVICE
$ SHOW LOGICAL/FULL SYS$SCRATCH
$ SHOW LOGICAL/FULL SYS$SYLOGIN
$ SHOW LOGICAL/FULL SYS$DISK
$ SHOW LOGICAL/JOB/FULL
$ SHOW LOGICAL/PROCESS/FULL
$ WRITE SYS$OUTPUT "######## F$GETJPI"
$ SET NOVERIFY
`)

	// The items in lists of ten, ITEMS_1 to ITEMS_n: one list of them all
	// would be a symbol and a line longer than DCL takes.
	lists := 0

	for i := 0; i < len(jpiItems); i += 10 {
		lists++
		fmt.Fprintf(&b, "$ ITEMS_%d = \"%s\"\n", lists, strings.Join(jpiItems[i:min(i+10, len(jpiItems))], ","))
	}

	fmt.Fprintf(&b, "$ LISTS = %d\n", lists)
	b.WriteString(`$ L = 1
$ NEXT_LIST:
$ IF L .GT. LISTS THEN GOTO ITEMS_DONE
$ ITEMS = ITEMS_'L'
$ I = 0
$ NEXT_ITEM:
$ ITEM = F$ELEMENT(I, ",", ITEMS)
$ IF ITEM .EQS. "," THEN GOTO LIST_DONE
$ VALUE = F$GETJPI("", ITEM)
$ IF $SEVERITY .NE. 1 THEN VALUE = "(status " + F$STRING($STATUS) + ")"
$ IF F$TYPE(VALUE) .EQS. "INTEGER" THEN VALUE = F$STRING(VALUE)
$ ! A long value (a privilege list) is written 100 characters a line.
$ WRITE SYS$OUTPUT "  ", ITEM, " = ", F$EXTRACT(0, 100, VALUE)
$ J = 100
$ MORE_VALUE:
$ IF J .GE. F$LENGTH(VALUE) THEN GOTO VALUE_DONE
$ WRITE SYS$OUTPUT "    ", F$EXTRACT(J, 100, VALUE)
$ J = J + 100
$ GOTO MORE_VALUE
$ VALUE_DONE:
$ I = I + 1
$ GOTO NEXT_ITEM
$ LIST_DONE:
$ L = L + 1
$ GOTO NEXT_LIST
$ ITEMS_DONE:
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## AUTHORIZE AND THE UAF"
$ DIRECTORY/SECURITY SYS$SYSTEM:SYSUAF.DAT
$ MCR AUTHORIZE SHOW UAF95
$ SHOW SYMBOL $STATUS
$ SET DEFAULT SYS$SYSTEM
$ MCR AUTHORIZE SHOW UAF95
$ SHOW SYMBOL $STATUS
$ SET DEFAULT 'STARTED_IN'
$ WRITE SYS$OUTPUT "######## THE SYSTEM'S FILES"
$ DIRECTORY/SECURITY SYS$MANAGER:SYLOGIN.COM
$ OPEN/READ/ERROR=NOREAD F SYS$MANAGER:SYLOGIN.COM
$ READ F LINE
$ CLOSE F
$ WRITE SYS$OUTPUT "read SYLOGIN.COM"
$ NOREAD:
$ OPEN/APPEND/ERROR=NOWRITE F SYS$MANAGER:SYLOGIN.COM
$ CLOSE F
$ WRITE SYS$OUTPUT "SYLOGIN.COM opened for append"
$ NOWRITE:
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SECURITY SYS$LIBRARY:STARLET.OLB
$ CREATE SYS$SCRATCH:UAF95I.TMP
$ DIRECTORY/SECURITY SYS$SCRATCH:UAF95I.TMP
$ DELETE SYS$SCRATCH:UAF95I.TMP;*
$ WRITE SYS$OUTPUT "UAF95I FINISHED"
$ EXIT
`)

	return b.String()
}

// ---------------------------------------------------------------------
// The login procedures. Each records, in global symbols UAF95I.COM shows,
// the order the procedures ran in and what each saw.

// recorder returns the lines that note what a login procedure named who
// sees.
func recorder(who string) string {
	return fmt.Sprintf(`$ IF F$TYPE(UAF95_ORDER) .EQS. "" THEN UAF95_ORDER == ""
$ UAF95_ORDER == UAF95_ORDER + ",%[1]s"
$ UAF95_%[1]s_VERIFY == F$VERIFY()
$ UAF95_%[1]s_PROCEDURE == F$ENVIRONMENT("PROCEDURE")
$ UAF95_%[1]s_DEPTH == F$ENVIRONMENT("DEPTH")
$ UAF95_%[1]s_MODE == F$MODE()
$ UAF95_%[1]s_DEFAULT == F$ENVIRONMENT("DEFAULT")
$ UAF95_%[1]s_SYS_LOGIN == F$TRNLNM("SYS$LOGIN")
$ UAF95_%[1]s_PRIVS == F$GETJPI("","CURPRIV")
$ UAF95_%[1]s_P1 == P1
$ UAF95_%[1]s_STATUS == $STATUS
`, who)
}

func sysloginProc() string {
	return `$ ! UAF95S.COM - Phase 95's probe: SYS$SYLOGIN while the probe runs.
$ ! Written by testdata/uaf95/gen.go. For the probe's accounts it records
$ ! what it sees; for every other user it runs the VAX's own SYLOGIN,
$ ! whose name UAF95ACCT.COM kept in UAF95$OLD_SYLOGIN.
$ !
$ IF F$EXTRACT(0,5,F$GETJPI("","USERNAME")) .NES. "UAF95" THEN GOTO OTHERS
` + recorder("SYLOGIN") + `$ EXIT
$ OTHERS:
$ IF F$TRNLNM("UAF95$OLD_SYLOGIN") .NES. "" THEN @UAF95$OLD_SYLOGIN
$ EXIT
`
}

func loginProc() string {
	return `$ ! LOGIN.COM - Phase 95's probe: UAF95's SYS$LOGIN:LOGIN.COM (and
$ ! UAF95L's, which its LGICMD should keep from running). Written by
$ ! testdata/uaf95/gen.go.
$ !
` + recorder("LOGIN") + `$ EXIT
`
}

func lgicmdProc() string {
	return `$ ! UAF95X.COM - Phase 95's probe: UAF95L's LGICMD. Written by
$ ! testdata/uaf95/gen.go. After recording, it runs a command that fails
$ ! with a warning, then one that fails with an error, to show what a
$ ! failing login procedure does to the login.
$ !
` + recorder("LGICMD") + `$ UAF95_LGICMD_BEFORE == "YES"
$ NOSUCHVERB
$ UAF95_LGICMD_AFTER_WARNING == "YES"
$ OPEN UAF95_NOFILE UAF95$DISK:[UAF95]NOSUCH.FILE
$ UAF95_LGICMD_AFTER_ERROR == "YES"
$ EXIT
`
}

// ---------------------------------------------------------------------
// The console scripts.

func exchange() string {
	var b strings.Builder

	b.WriteString(`! EXCHANGE.CMD - builds Phase 95's probe exchange volume with govax.
! Written by testdata/uaf95/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/uaf95-exchange.dsk" /DEVICE=RD53 UAF95
MOUNT/WRITE DUA5 "testdata/disks/uaf95-exchange.dsk"
CREATE/DIRECTORY DUA5:[UAF95LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[UAF95SCR]/PROTECTION=(S:RWE,O:RWE,G:,W:)
CREATE/DIRECTORY DUA5:[UAF95]
`)

	for _, f := range files {
		fmt.Fprintf(&b, "COPY \"testdata/uaf95/%s\"/HOST DUA5:[000000]%s\n", f, strings.ToUpper(f))
	}

	for _, f := range homeFiles {
		fmt.Fprintf(&b, "COPY \"testdata/uaf95/%s\"/HOST DUA5:[UAF95]%s\n", f, strings.ToUpper(f))
	}

	b.WriteString("DIRECTORY/OWNER/PROTECTION DUA5:[000000...]\nDISMOUNT DUA5\n")

	return b.String()
}

// copied are the results copyout.cmd brings back: the logs, and the
// listings AUTHORIZE's LIST wrote, every version.
var copied = []struct{ from, to string }{
	{"[UAF95LOG]UAF95.LOG", "uaf95.log"},
	{"[UAF95LOG]UAF95DROP.LOG", "uaf95drop.log"},
	{"[UAF95LOG]UAF95J_UAF95.LOG", "uaf95j_uaf95.log"},
	{"[UAF95LOG]UAF95J_UAF95L.LOG", "uaf95j_uaf95l.log"},
	{"[UAF95LOG]UAF95J_UAF95D.LOG", "uaf95j_uaf95d.log"},
	{"[UAF95LOG]UAF95I_UAF95_BATCH.LOG", "uaf95i_uaf95_batch.log"},
	{"[UAF95LOG]UAF95I_UAF95L_BATCH.LOG", "uaf95i_uaf95l_batch.log"},
	{"[UAF95LOG]UAF95I_UAF95_INTERACTIVE.LOG", "uaf95i_uaf95_interactive.log"},
	{"[UAF95LOG]UAF95I_UAF95L_INTERACTIVE.LOG", "uaf95i_uaf95l_interactive.log"},
	{"[UAF95SCR]SYSUAF.LIS;1", "sysuaf_1.lis"},
	{"[UAF95SCR]SYSUAF.LIS;2", "sysuaf_2.lis"},
	{"[UAF95SCR]SYSUAF.LIS;3", "sysuaf_3.lis"},
	{"[UAF95SCR]RIGHTSLIST.LIS;1", "rightslist_1.lis"},
	{"[UAF95SCR]RIGHTSLIST.LIS;2", "rightslist_2.lis"},
	{"[UAF95SCR]RIGHTSLIST.LIS;3", "rightslist_3.lis"},
}

func copyout() string {
	var b strings.Builder

	b.WriteString(`! COPYOUT.CMD - copies Phase 95's probe results off uaf95-exchange.dsk
! into testdata/uaf95/vax (README.md). Written by testdata/uaf95/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/copyout.cmd
!
! A file the run didn't make is a message, and the rest go on.
MOUNT DUA5 "testdata/disks/uaf95-exchange.dsk"
DIRECTORY DUA5:[UAF95LOG],[UAF95SCR]
`)

	for _, c := range copied {
		fmt.Fprintf(&b, "COPY DUA5:%s \"testdata/uaf95/vax/%s\"/HOST/QUIET\n", c.from, c.to)
	}

	b.WriteString("DISMOUNT DUA5\n")

	return b.String()
}

// longestLine is the longest line a file may have. Round 1 found DCL
// refusing a 504-byte procedure record (RMS-W-RTB), which ended the
// procedure; 200 leaves room under any limit VMS 7.1's DCL or AUTHORIZE
// has.
const longestLine = 200

func write(name, text string) {
	for i, line := range strings.Split(text, "\n") {
		if len(line) > longestLine {
			log.Fatalf("%s:%d is %d characters, more than %d", name, i+1, len(line), longestLine)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}

func main() {
	write("uaf95.com", systemProc())
	write("uaf95acct.com", accountProc())
	write("uaf95drop.com", dropProc())
	write("uaf95i.com", stateProc())
	write("uaf95s.com", sysloginProc())
	write("login.com", loginProc())
	write("uaf95x.com", lgicmdProc())
	write("exchange.cmd", exchange())
	write("copyout.cmd", copyout())

	n := 0
	for _, s := range sections {
		n += len(s.commands)
	}

	fmt.Printf("%d AUTHORIZE cases\n", n)
}
