# The Phase 95 probe: authentication

These fixtures are `docs/planning/PHASE-95 - authentication.md`'s subtask
2 (its Part D). They ask VMS 7.1 three things:

- **AUTHORIZE's words**: 130 commands, every one govax will have, with
  good and bad arguments, and the reports (SHOW in each form, LIST's
  files) and messages they give. They run against a *scratch*
  SYSUAF.DAT and RIGHTSLIST.DAT on the exchange volume, named by process
  logical names, so they never change the VAX's own files.
- **What logging in does**: three probe accounts log in by batch job and
  by hand. The probe's own SYLOGIN, LOGIN.COM, and LGICMD procedures
  record when each ran and what it saw, and UAF95I.COM shows the process
  afterward: SYS$LOGIN and its kin, the default directory, privileges,
  rights, and every F$GETJPI item.
- **What an account without SYSPRV gets** from AUTHORIZE and from the
  system's files.

| File | What it holds |
| ---- | ------------- |
| `gen.go` | Writes everything below: `go run testdata/uaf95/gen.go` from the repository root |
| `uaf95b.com` | Round 3's SYSTEM run: LIST by name, the accounts again, the batch jobs |
| `uaf95.com` | SYSTEM's run: the environment, the scratch files and AUTHORIZE's cases, the accounts (by `uaf95acct.com`), the batch jobs, SYSTEM's own state |
| `uaf95acct.com` | Makes the probe's accounts in the VAX's SYSUAF, their home directory, and points SYS$SYLOGIN at `uaf95s.com` |
| `uaf95drop.com` | The end: puts SYS$SYLOGIN back, removes the accounts and identifiers, deletes the scratch files |
| `uaf95i.com` | A process's state after login, written to `[UAF95LOG]UAF95I_user_mode.LOG` |
| `uaf95s.com` | SYS$SYLOGIN while the probe runs: records what it sees for the probe's accounts, and runs the VAX's own SYLOGIN for everyone else |
| `login.com` | UAF95's `SYS$LOGIN:LOGIN.COM`, in `[UAF95]` |
| `uaf95x.com` | UAF95L's LGICMD, in `[UAF95]`: records, then fails with a warning, then with an error |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and bring the results back |
| `copyout3.cmd` | Round 3's results into `vax/round3/` |

## The accounts

`UAF95ACCT.COM` makes them with AUTHORIZE, in the VAX's own SYSUAF, and
`UAF95DROP.COM` removes them. Every account has the password `UAF95PW`,
not expiring (`/NOPWDEXPIRED/PWDLIFETIME=NONE`), owner `GOVAX PROBE`,
account `GOVAX`, device `UAF95$DISK:`, directory `[UAF95]`, and
privileges (authorized and default) TMPMBX and NETMBX.

| Username | UIC | What differs |
| -------- | --- | ------------ |
| UAF95 | `[200,5]` | No LGICMD, so its login runs `SYS$LOGIN:LOGIN.COM`; granted the identifier UAF95_ID |
| UAF95L | `[200,7]` | `/LGICMD=UAF95$DISK:[UAF95]UAF95X.COM` (the same directory has a LOGIN.COM, which should not run) |
| UAF95D | `[200,6]` | `/FLAGS=DISUSER` |

To make them by hand instead, as SYSTEM, with the volume mounted and
`UAF95$DISK` defined (step 3 below defines it):

    $ SET DEFAULT SYS$SYSTEM
    $ MCR AUTHORIZE
    UAF> ADD UAF95/UIC=[200,5]/PASSWORD=UAF95PW/NOPWDEXPIRED/PWDLIFETIME=NONE/FLAGS=NODISUSER
    UAF> MODIFY UAF95/DEVICE=UAF95$DISK:/DIRECTORY=[UAF95]/ACCOUNT=GOVAX/OWNER="GOVAX PROBE"
    UAF> MODIFY UAF95/PRIVILEGES=(TMPMBX,NETMBX)/DEFPRIVILEGES=(TMPMBX,NETMBX)
    UAF> COPY UAF95 UAF95L/UIC=[200,7]/PASSWORD=UAF95PW/NOPWDEXPIRED
    UAF> MODIFY UAF95L/LGICMD=UAF95$DISK:[UAF95]UAF95X.COM
    UAF> COPY UAF95 UAF95D/UIC=[200,6]/PASSWORD=UAF95PW/NOPWDEXPIRED/FLAGS=DISUSER
    UAF> ADD/IDENTIFIER UAF95_ID
    UAF> GRANT/IDENTIFIER UAF95_ID UAF95
    UAF> EXIT

The home directory must then be owned by `[200,5]`, with protection
(S:RWE,O:RWE,G:RWE,W:), and its two files owned by `[200,5]` too;
`UAF95ACCT.COM`'s SET SECURITY lines do that. UIC group 200 was Phase
94's (PRIV94 and PRIV94N, removed at its end); if the VAX uses it now,
say so before running. `ADD` may make a group identifier, GOVAX, for
group 200 if the group has none; `UAF95DROP.COM` removes it.

## What the run changes on the VAX

All undone by `UAF95DROP.COM`: the three accounts in SYSUAF, UAF95_ID
(and maybe GOVAX) in RIGHTSLIST, the system logical names `UAF95$DISK`,
`UAF95$OLD_SYLOGIN`, and `UAF95$OLD_SYLOGIN_MODE`, and **SYS$SYLOGIN**,
which points at the probe's `UAF95S.COM` while the probe runs. Every
other user who logs in meanwhile still gets the VAX's own SYLOGIN,
through `UAF95S.COM`. Three batch jobs run on SYS$BATCH, kept
(`/KEEP`) logs in `[UAF95LOG]`.

## The VAX run

1. **Build the exchange volume**, from the repository root on the host:

       rm -f testdata/disks/uaf95-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/exchange.cmd

   This makes `testdata/disks/uaf95-exchange.dsk` (RD53, label UAF95,
   gitignored).

2. **Attach it** to the simh VAX on a free disk unit, at the `sim>`
   prompt:

       attach rq1 testdata/disks/uaf95-exchange.dsk

   The steps below say `DUA1:`; use the unit you attached.

3. **As SYSTEM**, check that the batch queue runs (the procedure waits
   for each job with SYNCHRONIZE, and would wait forever on a stopped
   queue):

       $ SHOW QUEUE/BATCH/ALL

   If SYS$BATCH is missing or stopped, start it (`START/QUEUE/MANAGER`
   if the queue manager isn't running, then `INITIALIZE/QUEUE/BATCH/START
   SYS$BATCH` or `START/QUEUE SYS$BATCH`), or say so: the batch part can
   be left out, and the logins by hand (step 4) cover most of it.

   Then mount the volume for every user, and run SYSTEM's part:

       $ MOUNT/SYSTEM DUA1: UAF95
       $ @DUA1:[000000]UAF95

   It takes a few minutes. Its log is `UAF95$DISK:[UAF95LOG]UAF95.LOG`,
   whose last line is `UAF95 FINISHED`. It writes only that one line
   to the terminal when it ends. If nothing happens for several minutes
   at the start, AUTHORIZE may be asking at the terminal whether to make
   the new scratch SYSUAF.DAT (the procedure answers from its own data
   line, `YES`, which should be where AUTHORIZE reads): type `YES` and
   Return, and say so.

4. **Log in by hand**, from a terminal (or `SET HOST 0`), and keep the
   terminal's text. Each time:

   | Log in as | Password | Then |
   | --------- | -------- | ---- |
   | UAF95 | UAF95PW | `$ @UAF95$DISK:[000000]UAF95I` and `$ LOGOUT` |
   | UAF95L | UAF95PW | the same (its LGICMD's failures show on the terminal at login) |
   | UAF95D | UAF95PW | nothing: the login should be refused; note the message |

   If VMS asks for a new password, give any; nothing depends on it.
   Copy everything the terminal showed for the three logins, from the
   `Username:` prompt to the logout message, into
   `testdata/uaf95/vax/terminal.log` on the host.

5. **As SYSTEM**, run the end:

       $ @UAF95$DISK:[000000]UAF95DROP

   Its log is `[UAF95LOG]UAF95DROP.LOG`, ending `UAF95DROP FINISHED`
   (it deassigns `UAF95$DISK` after the log closes). Then dismount and
   detach:

       $ DISMOUNT DUA1:

       sim> detach rq1

6. **Copy the results back**, on the host, from the repository root:

       mkdir -p testdata/uaf95/vax
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/copyout.cmd

   This writes the logs (`uaf95.log`, `uaf95drop.log`, the three batch
   logs `uaf95j_*.log`, and the four `uaf95i_*.log`) and the listings
   AUTHORIZE's LIST wrote (`sysuaf_n.lis`, `rightslist_n.lis`). A file the
   run didn't make is a message, and the rest are copied.

If a run stops partway, run `@UAF95$DISK:[000000]UAF95DROP` as SYSTEM
(it can run more than once), then start again from step 1 with a new
volume.

## Round 1

Round 1 ran on VMS V7.1 on 2026-11-15 and stopped at AUTHORIZE case 21,
before the accounts were made; its log is `vax/round1/uaf95.log`. Two
mistakes of the probe's own:

- **Case 21's ADD was 504 characters**, and DCL refused to read the
  record (`%RMS-W-RTB, 504 byte record too large for user's buffer`),
  which ended the procedure. Round 2 splits it into an ADD and three
  MODIFYs, and `gen.go` refuses any line over 200 characters (the
  accounts' ADDs and the F$GETJPI item list are split too).
- **AUTHORIZE ignored the supervisor-mode RIGHTSLIST name** and used the
  VAX's own rights database: CREATE/RIGHTS said the file existed,
  SHOW/IDENTIFIER listed the VAX's identifiers, and case 8's `ADD UAFA`
  left the identifier UAFA `[300,1]` in it (removed by hand afterward,
  `MCR AUTHORIZE REMOVE/IDENTIFIER UAFA`). The supervisor-mode SYSUAF
  name was honored. Round 2 defines all four names as executive-mode
  process names, and runs no case unless both scratch files exist.

What round 1 answered before it stopped: a new SYSUAF.DAT holds SYSTEM
(`SYS$SYSROOT:[SYSMGR]`, owner SYSTEM MANAGER) and DEFAULT (`[200,200]`,
DISUSER, directory `[USER]`); SHOW's full and brief layouts; ADD's
defaults (DEFAULT's values, password pre-expired, its UIC identifier
added to the rights database); a UIC's member is octal (`[300,9]` is
`%UAF-E-UICERR`, so round 2's cases use octal members, with one case of
`[300,8]` kept to show it); and the messages UAEERR, UICERR, BADSPC,
RDBADDERRU with DUPIDENT, NAOFIL, RDBCREERR.

## Round 2

Round 2 ran on VMS V7.1 on 2026-11-15, to the end; `vax/` holds its
logs, LIST's listings, and `terminal.log`, the logins by hand. All 134
AUTHORIZE cases ran on the scratch files (the executive-mode names were
honored: SHOW/IDENTIFIER listed only a new rights database's
environmental identifiers). The logins showed SYLOGIN and the process's
state, DISUSER's message (`%LOGIN-F-DISUSER, account is disabled`, at the
terminal, in OPCOM's security alarm, and for the batch job), and every
F$GETJPI item. Two things it couldn't show, through a mistake of the
probe's own:

- `UAF95ACCT.COM`'s SET SECURITY lines wrote `W:` for no world access,
  which VMS refuses (`%DCL-W-VALREQ`; it is `W` alone). So the home
  directory and its procedures stayed SYSTEM's, with no world access,
  and the accounts (group 200, world to `[1,4]`) couldn't read LOGIN.COM
  or UAF95X.COM: VMS skipped them **with no message**, and only SYLOGIN
  ran. (govax took `W:` as no access; it now refuses it as VMS does.)
- The VAX's own DEFAULT record has `LGICMD: LOGIN`, which COPY and ADD
  gave the accounts.

Group 200's group identifier is still Phase 93's PROT93
(`[000200,177777]`), so the UICs show as `[PROT93,UAF95]` and ADD made no
GOVAX identifier. LIST/IDENTIFIER and LIST/RIGHTS without a name say
`%UAF-I-NOIDNAME` and list nothing.

## Round 3

Round 3 is the login procedures again, with the protection fixed, and
UAF95's LGICMD empty (`MODIFY UAF95/LGICMD=""`) so its login uses
SYS$LOGIN:LOGIN.COM, as govax's accounts will. `uaf95b.com` lists
identifiers and rights by name from scratch files (13 cases), makes the
accounts with `UAF95ACCT.COM`, and runs the UAF95 and UAF95L batch jobs.
UAF95D isn't needed again.

1. **Build a fresh exchange volume** (round 2's results are already in
   `vax/`), from the repository root:

       rm -f testdata/disks/uaf95-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/exchange.cmd

2. **Attach it** (`attach rq1 testdata/disks/uaf95-exchange.dsk`) and, **as
   SYSTEM**:

       $ MOUNT/SYSTEM DUA1: UAF95
       $ @DUA1:[000000]UAF95B

   Its log is `UAF95$DISK:[UAF95LOG]UAF95B.LOG`, ending `UAF95B FINISHED`.

3. **Log in by hand** as UAF95 and as UAF95L (password `UAF95PW`), and in
   each run `$ @UAF95$DISK:[000000]UAF95I`, then `$ LOGOUT`. Keep the
   terminal's text from `Username:` to the logout message: UAF95L's
   LGICMD should now show its warning and error at login. Paste it into
   `testdata/uaf95/vax/round3/terminal.log`.

4. **As SYSTEM**, `@UAF95$DISK:[000000]UAF95DROP`, then `DISMOUNT DUA1:`
   and `detach rq1`.

5. **Copy the results back**:

       mkdir -p testdata/uaf95/vax/round3
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/copyout3.cmd

## Round 3's results

Round 3 ran on VMS V7.1 on 2026-11-15, to the end; `vax/round3/` holds
its logs and `terminal.log`. SYLOGIN ran first, then LOGIN.COM (UAF95)
or the LGICMD (UAF95L), never both, the account's procedure at depth 1
and SYLOGIN at depth 2 above it. UAF95X.COM's warning let it go on and
its error ended it; at the terminal the login went on, while UAF95L's
batch job ended there (so it has no `uaf95i_uaf95l_batch.log`). The phase
doc's A.5 has every round's answers.

## The rehearsal

`TestUAF95Probe` (`internal/console/uaf95_test.go`) runs the login
procedures and `UAF95I.COM` on govax, so the probe can't lose a VAX
round to a mistake in its own DCL. `UAF95.COM` and `UAF95DROP.COM` were
run on govax too, to their ends; what govax lacks of them (AUTHORIZE,
SUBMIT, SYNCHRONIZE, SHOW QUEUE, SET VOLUME, SHOW PROCESS/RIGHTS and
/QUOTAS, SYS$LOGIN, logging in) shows as messages, and is the later
subtasks' work. The rehearsal found one govax bug, fixed with it: an
OPEN that failed before RMS opened anything (a device not mounted)
ignored its `/ERROR` label.
