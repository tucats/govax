# Phase 98 — batch queues

**Status:** planned 2026-11-16 (subtask 1, the survey and this plan);
the plan reviewed and accepted by the author the same day, with the
answers recorded in "The review". This document is the development plan;
as the work goes on it becomes the implementation record: the progress
log, and at the end "Where things stand".

## Goal

govax gets VMS's batch queues: a queue manager, batch queues a suitably
privileged user creates and runs, and batch jobs that run as VMS runs
them, as detached processes logged in to the submitter's account (or
another one's, with `/USER`) with the attributes and rights of a batch
job.

- **The queue manager.** START/QUEUE/MANAGER creates the queue database
  and starts the manager; STOP/QUEUE/MANAGER stops it. While it runs, it
  starts each queue's jobs as their time comes and their queue has room.
- **Queues.** INITIALIZE/QUEUE/BATCH creates a queue; START/QUEUE and
  STOP/QUEUE start and stop it; DELETE/QUEUE deletes a stopped one; SET
  QUEUE changes it; SHOW QUEUE shows it.
- **Jobs.** SUBMIT queues one or more command procedures as a job, with
  at least `/AFTER`, `/USER`, `/PARAMETERS`, `/HOLD`, and `/LOG_FILE`
  (Decision 10 proposes more); DELETE/ENTRY and STOP/QUEUE/ENTRY remove
  or abort one; SET ENTRY changes or releases one; SHOW ENTRY and
  SYNCHRONIZE follow one.
- **The batch job.** A detached process named `BATCH_n`, logged in
  non-interactively: the account's UIC, privileges, and quotas; the
  queue's base priority; the BATCH identifier and not INTERACTIVE or
  LOCAL; `F$MODE()` "BATCH"; SYLOGIN and LOGIN.COM run with verify on;
  its output in a log file that ends with LOGOUT's accounting report.
- **Protection.** A queue is a protected object, with an owner, a
  protection code, and an ACL, and its own access types (read, submit,
  manage, delete). Every command asks what VMS's asks: OPER, SYSNAM,
  CMKRNL, ALTPRI, or the queue's and job's protection, through Phase
  94's privilege gate and Phase 93's protection rule.

The session below is what the plan expects (the messages are VMS 7.1's
as the probe will show them; the lines are what the plan expects):

    $ START/QUEUE/MANAGER SYS$COMMON:[SYSEXE]
    $ INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2/START SYS$BATCH
    $ SUBMIT/PARAMETERS=(ALPHA,"two words")/LOG_FILE=[.LOGS] BUILD
    Job BUILD (queue SYS$BATCH, entry 1) started on SYS$BATCH
    $ SUBMIT/AFTER=TOMORROW/HOLD NIGHTLY
    Job NIGHTLY (queue SYS$BATCH, entry 2) holding
    $ SHOW QUEUE SYS$BATCH
    Batch queue SYS$BATCH, busy, on GOVAX::

      Entry  Jobname         Username     Status
      -----  -------         --------     ------
          1  BUILD           SYSTEM       Executing
          2  NIGHTLY         SYSTEM       Holding until 17-NOV-2026 00:00
    $ SYNCHRONIZE/ENTRY=1
    $ DELETE/ENTRY=2
    $ STOP/QUEUE SYS$BATCH
    $ DELETE/QUEUE SYS$BATCH

and, as a user without OPER and with only the default (S:M,O:D,G:R,W:S)
protection:

    $ INITIALIZE/QUEUE/BATCH MYQ
    %JBC-E-NOPRIV, no privilege for attempted operation
    $ STOP/QUEUE SYS$BATCH
    %JBC-E-NOPRIV, no privilege for attempted operation
    $ SUBMIT/USER=SYSTEM X
    %JBC-E-NOPRIV, no privilege for attempted operation

## Part A — the survey (2026-11-16)

Sources: the *OpenVMS 7.3-1 DCL Dictionary* (each command's
description, qualifiers, and "Requires" line), the *VMS 5.0 System
Services Reference Manual* and VSI's system services pages (`$SNDJBC`,
`$GETQUI`), the *Guide to OpenVMS System Security* (6.0) on queue
protection, *VAX/VMS Internals and Data Structures* on process creation
and the job controller, and Phase 95's probe, whose batch jobs on the
simh VAX (`testdata/uaf95/vax/uaf95j_*.log`, and `uaf95.log`'s SUBMIT
lines) already show a batch log's shape and SUBMIT's message.

### A.1 What VMS has

**The queue manager and job controller.** Since VMS 5.5 the queue
manager is a detached process, QUEUE_MANAGER, that keeps the queue
database and decides what runs; the job controller, JOB_CONTROL,
creates batch processes for it. START/QUEUE/MANAGER (OPER and SYSNAM)
names the database's directory, creating it the first time (the master
file `QMAN$MASTER.DAT`, and per manager `SYS$QUEUE_MANAGER.QMAN$QUEUES`
and `.QMAN$JOURNAL`); the manager then starts at every boot until
STOP/QUEUE/MANAGER/CLUSTER. The database lasts across reboots; a job
executing when the system stopped is requeued if it was submitted
`/RESTART`, and otherwise ends with an error status.

**Queues.** A queue has a name (up to 31 characters), a kind (batch;
also printer, server, terminal, and generic ones, out of scope here,
Decision 1), and attributes: `/JOB_LIMIT` (jobs at once, default 1),
`/BASE_PRIORITY` (the jobs' base priority, default 4), `/OWNER_UIC`
(default `[SYSTEM]`), `/PROTECTION` (default `(S:M,O:D,G:R,W:S)`),
`/WSDEFAULT`, `/WSQUOTA`, `/WSEXTENT`, `/CPUDEFAULT`, `/CPUMAXIMUM`,
`/RETAIN`, `/[NO]ENABLE_GENERIC`, `/AUTOSTART_ON`, `/DESCRIPTION`,
`/RECORD_BLOCKING`, `/CHARACTERISTICS`. Its state is one of stopped,
starting, idle (started, nothing running), busy (at its job limit),
available (started, room for more), paused, stopping, and the closed and
other substates SHOW QUEUE prints.

**Queue protection.** A queue is an object of the security class
QUEUE, with an owner, a protection code, and an ACL, its access types
**R**ead (see the queue's jobs), **S**ubmit, **M**anage (start, stop,
change, delete it, and any job in it), and **D**elete (a job's owner
category: change or delete one's own jobs). The protection code's
categories are those of files. The DCL Dictionary's "Requires" lines:

| Command | Requires |
| --- | --- |
| START/QUEUE/MANAGER, STOP/QUEUE/MANAGER | OPER and SYSNAM |
| INITIALIZE/QUEUE | OPER to create a queue; M access to change one |
| START/QUEUE, STOP/QUEUE, /NEXT, /RESET | M access to the queue |
| SET QUEUE, DELETE/QUEUE | M access to the queue |
| SHOW QUEUE | R access (to see a job's details) |
| SUBMIT | S access to the queue |
| SUBMIT/USER | CMKRNL, and R and W access to the UAF |
| SUBMIT/PRIORITY above MAXQUEPRI | OPER or ALTPRI |
| DELETE/ENTRY, SET ENTRY | D access to the job, or M access to the queue |
| STOP/QUEUE/ENTRY, SYNCHRONIZE | D access to the job |
| SHOW ENTRY | R access |
| SET SECURITY/CLASS=QUEUE | the object's owner or CONTROL access (Phase 93) |

OPER gives every access to every queue, as BYPASS and SYSPRV give a
file's (the Security guide; the probe confirms which privileges count).

**Jobs.** Each job has an entry number, unique across the manager's
queues (1 to 999 by default, wrapping), a name (the first file's, or
`/NAME`), its user, its files (P1 to P8 are the job's, given to every
file), its log file, its status
(pending, holding, holding until a time, executing, retained on
completion or error, aborting), and its options: `/AFTER`, `/HOLD`,
`/PRIORITY` (the job's place in its queue, not the process's priority),
`/CPUTIME`, `/WSDEFAULT` and the other working-set limits (no more than
the queue's), `/KEEP`, `/PRINTER`, `/NOTIFY`, `/RESTART`, `/RETAIN`,
`/NOTE`, `/CHARACTERISTICS`. SUBMIT defines the local symbol `$ENTRY` as
the new entry's number, and says what it did:

    Job UAF95J (queue SYS$BATCH, entry 13) started on SYS$BATCH

with `$STATUS` `%X10000001` (Phase 95's log). A file is held by its file
ID, so a later version doesn't replace it. `/DELETE` deletes the files
after the job.

**The batch job's process** (the DCL Dictionary's SUBMIT and the
Internals book's chapter on process creation): the job controller
creates a detached process running LOGINOUT, which logs it in to the
job's user: the account's UIC, privileges, quotas (limited by the
queue's working-set and CPU values), and default device and directory;
the queue's base priority; the rights BATCH plus the account's
identifiers (Phase 95's probe found BATCH alone of the environmental
ones). Its process name is `BATCH_n`, n the entry number; JPI$_JOBTYPE
and JPI$_MODE are BATCH; `F$MODE()` is "BATCH". DCL runs with verify on
(Phase 95's probe: "Verify ... on in batch"), runs SYLOGIN and the
account's login procedure, then each of the job's files as `@file` with
the job's P1 to P8; a file that ends with an error or severe status ends
the job. The log file, SYS$OUTPUT and SYS$ERROR, has the echoed
commands and their output, and ends with LOGOUT's report (Phase 95's
log, `uaf95j_uaf95.log`):

      UAF95        job terminated at 15-NOV-2026 17:19:48.42

      Accounting information:
      Buffered I/O count:             230         Peak working set size:     551
      ...

The log file's default is the job's name with type `.LOG`, and (by the
Dictionary) is printed on SYS$PRINT and then deleted unless `/KEEP` or
`/NOPRINTER`. SET RESTART_VALUE and the symbols `BATCH$RESTART` and
`$RESTART` let a `/RESTART` job pick up where it stopped.

**Services and lexical functions.** `$SNDJBC(W)` is every queue and job
operation, by function code (`SJC$_CREATE_QUEUE`, `SJC$_START_QUEUE`,
`SJC$_ENTER_FILE`, `SJC$_DELETE_JOB`, ...) and item list; `$GETQUI(W)`
reads queues and jobs, with wildcard contexts (`QUI$_DISPLAY_QUEUE`,
`QUI$_DISPLAY_JOB`, `QUI$_DISPLAY_FILE`, ...); `F$GETQUI` is `$GETQUI`
from DCL; `LIB$GETQUI` its RTL jacket. The messages are the JBC
facility's (`%JBC-E-NOSUCHQUE`, `%JBC-E-NOPRIV`, `%JBC-E-JOBQUEDIS`,
...) and the QMAN facility's.

### A.2 What govax has today

- **No queues at all.** No SUBMIT, SHOW QUEUE, or queue commands;
  `F$GETQUI` is LEXNOTIMPL by choice (`dcllexical.go`;
  `lexicals.hlp` says "govax has none"); `$SNDJBC`, `$GETQUI`, and
  `LIB$GETQUI` are in the P1 vector and LIBRTL's table only as names.
- **No definitions.** `vmsdef` has none of `SJC$_`, `QUI$_`, or `JBC$_`,
  and no JBC or QMAN messages (only `CLI$_SUBMITERR` and `RMS$_SPL`).
  `JPI$K_BATCH`, `PRC$M_BATCH`, `CLI$M_BATCH`, and `KGB$K_BATCH_ID` are
  there.
- **Detached processes with a CLI** (`corevms/cliprocess.go`, Phase 48):
  `$CREPRC` of LOGINOUT starts the console's DCL in the new process
  (`CommandInterpreter.Start`, `CLIStartup.Login`), reading SYS$INPUT
  (a file, mailbox, or the terminal) with verify on when it isn't the
  terminal, and ending with LOGOUT's report (`LogoutReport`). This is
  most of a batch job's process already.
- **Logging in** (Phase 95, `corevms/login.go`): `System.BeginLogin(
  username, interactive)` and `Environment.LogIn` make an account a
  process; a non-interactive login already gets `batchRights` (BATCH
  alone) and records the last non-interactive login time. DCL's
  `RunLoginProcedures` (`dclshell/dclloginproc.go`) runs SYLOGIN and the
  login procedure, but only for process 1. Phase 95's "Where things
  stand" left batch jobs (and detached logins by username) for later;
  this phase takes them.
- **Job type and mode** (`corevms/getjpi.go`): `jobType` is LOCAL for
  the console's job and DETACHED otherwise; `jobMode` INTERACTIVE or
  OTHER. Neither knows BATCH yet.
- **Protection and privileges.** Phase 93's `internal/security` rule
  and `$GET_SECURITY`/`$SET_SECURITY` with security classes
  (`corevms/getsecurity.go`'s `securityClasses`: DEVICE, FILE, VOLUME,
  LOGICAL_NAME_TABLE, SECURITY_CLASS; no QUEUE). Phase 94's gate
  (`corevms/privrules.go`), with OPER already in rules for `$SNDOPR`
  and `$BRKTHRU`, and `CommandPrivilege` for DCL's own commands.
  `TestNoUncheckedPrivileges` and `TestEveryPrivilegeRuleUsed` will hold
  the new rules too.
- **The authorization file** (Phase 95, `internal/uaf`), with its own
  profile and access check, which SUBMIT/USER's "R and W access to the
  UAF" can ask.
- **Processes run beside the console** (Phases 44 and 51): the
  scheduler runs other processes between DCL commands, during WAIT, and
  while the prompt waits for a line (`LendWhileReading`); with nothing
  to run, `idle.go` moves time to the next timer (`System.nextTimer`).
  A one-shot subcommand runs other processes only while its command
  runs.
- **Times.** DCL's absolute, delta, and combination times are parsed
  for the lexical functions and `$BINTIM`; the system clock is the
  quantum clock unless `vax.hardware.clock`.
- **Grammar.** `console.dcl` has STOP (process), but no START,
  SUBMIT, SYNCHRONIZE, or the queue forms of INITIALIZE, DELETE, SET,
  and SHOW. Its `PRINT` verb is govax's own (prints expressions), not
  VMS's PRINT, which this phase leaves alone (Decision 1).
- **Persistent host data.** `internal/uaf` keeps the authorization file
  as JSON under `internal/hostlock`, and `sysvol` the system volume:
  the two models for where the queue database could live (Decision 3).

### A.3 What the references leave open

The probe (Part D) answers these:

1. **Messages and layouts**: each command's messages (SUBMIT's for
   pending, holding, and holding until; the JBC errors for a missing
   queue, a stopped queue, a missing entry, no privilege, a queue that
   isn't empty), SHOW QUEUE's brief, `/FULL`, and `/ALL` layouts, SHOW
   ENTRY's, and each command's `$STATUS`.
2. **Privileges and protection**: which privileges override a queue's
   protection (OPER only, or also BYPASS, SYSPRV, GRPPRV?), what each
   access type allows, how D access to a job is decided (the job's
   owner? its UIC's categories against the queue's O field?), what
   SUBMIT/USER asks exactly, and what SHOW QUEUE hides without R access.
3. **The batch process**: every `F$GETJPI` item from inside a job
   (JOBTYPE, MODE, PRCNAM, OWNER, MASTER_PID, PRI, PRIB, quotas, rights,
   TERMINAL), the logical names (SYS$INPUT, SYS$OUTPUT, SYS$ERROR,
   SYS$COMMAND, SYS$LOGIN, SYS$SCRATCH, SYS$NET?) and their tables and
   modes, `F$ENVIRONMENT`'s answers, how the queue's base priority and
   working-set limits meet the account's, and SHOW PROCESS/ALL.
4. **The log file**: its default directory (the first file's? SYS$LOGIN?)
   and name, `/LOG_FILE` with only a directory, `/NOLOG_FILE`, what
   happens with no SYS$PRINT queue under the default `/PRINTER`, the
   log's first lines (anything before SYLOGIN's echo?), and its owner
   and protection.
5. **A job's end**: several files with an error in the second; EXIT with
   a status; STOP/QUEUE/ENTRY and DELETE/ENTRY of an executing job, and
   what its log says; STOP/IDENTIFICATION of the process; `/CPUTIME`
   exceeded; a DISUSER user (Phase 95 saw `%LOGIN-F-DISUSER` on the
   submitter's side); `/RETAIN`; what SYNCHRONIZE returns in each case.
6. **Timing**: `/AFTER` in each time form, a time in the past, SET
   ENTRY/RELEASE of a held job, a job limit of 1 with two jobs ready,
   `/PRIORITY` ordering in a queue.
7. **The services' values**: the `SJC$_`, `QUI$_`, and `JBC$_` codes and
   the `QUI$` flags, by assembling the definition macros `GLOBAL` and
   decoding the object (as `testdata/for83`'s and `testdata/idx88`'s
   `def_*.mar` and `decode.go` did), and the JBC and QMAN messages'
   texts by `$GETMSG`.

### A.4 The probe's answers (2026-11-16)

Round 1 on VMS 7.1 (`testdata/bat98/vax`, the README's "Round 1"), with
the steps by hand and a reboot; round 2 (`vax/bat98b.log`) is BAT98Q again,
corrected, for `$SNDJBC`'s job functions.

**Messages and statuses** (A.3, 1):

- **SUBMIT** says `Job NAME (queue Q, entry n) started on Q`, or `...
  pending` with a second line (indented five spaces) `pending status
  caused by queue stopped state`, or `... holding`, or `... holding until 17-NOV-2026
  00:00`; `$STATUS` `%X10000001`; `/NOIDENTIFY` says nothing; `$ENTRY` is
  set. A past `/AFTER` (TODAY, a past date) is simply pending;
  `/AFTER=TOMORROW` is midnight; `/HOLD/AFTER` is holding. Its failures:
  a ninth parameter `%SUBMIT-F-INVQUAVAL, value 'I' invalid for
  /PARAMETERS qualifier`; one longer than 255 is DCL's own TKNOVF; a
  missing queue `%SUBMIT-F-CREJOB` `-JBC-E-NOSUCHQUE`; `/PRINTER` naming
  no queue (or SYS$PRINT missing) `-JBC-E-NODSTQUE`; a missing file
  `%SUBMIT-F-OPENIN, error opening DUA1:[000000]X.COM; as input` `-RMS-E-FNF`;
  a missing user `%SUBMIT-F-INVQUAVAL, value 'X' invalid for /USER
  qualifier`; a missing log directory `%SUBMIT-F-INVLOGFIL, invalid log
  file specification` `-RMS-E-DNF` `-SYSTEM-W-NOSUCHFILE`.
- **The queue commands' statuses**: success `%X10040001` (JBC$_NORMAL with
  the inhibit bit); SHOW `%X10000001`; SET SECURITY `%X00000001`;
  `%JBC-E-NOSUCHQUE` `%X1004803A`; START of a started queue
  `%JBC-E-STARTED, queue already started`; STOP of a stopped queue
  succeeds silently; INITIALIZE of a started queue `%JBC-I-QUENOTMOD,
  modifications not made to running queue` and changes nothing; DELETE
  of a started queue `%DELETE-E-NOTDELETED, error deleting Q`
  `-JBC-E-QUENOTSTOP`, of a missing one `%DELETE-W-SEARCHFAIL, error
  searching for Q` `-JBC-E-NOSUCHQUE`; DELETE/QUEUE takes its jobs with it.
  A 32-character name is `%JBC-F-INVPARLEN`, a bad character
  `%JBC-F-INVQUENAM`, `/JOB_LIMIT=0` or `256` and `/BASE_PRIORITY=99`
  `%QUEMAN-F-INVQUAVAL, value '0' invalid for /JOB_LIMIT qualifier`.
  INITIALIZE/QUEUE without `/BATCH` makes a *printer* queue. SHOW QUEUE
  has no `/USER_NAME` (IVQUAL); SHOW ENTRY has.
- **Queue states**: `stopped` (a new queue, with no node until first
  started: `Batch queue BAT98_A, stopped`), `idle`, `available` (jobs
  running, room for more), `busy` (at its job limit), `paused` (STOP/QUEUE
  without `/NEXT`, which **suspends** the executing jobs: a 20-second job
  took 25 when paused for 5), `available, stop pending` (`/NEXT` with a job
  running), then `stopped`. The heading is `Batch queue NAME, state, on
  SIMVAX::`, then `  <description>`, then `/FULL`'s attributes in a fixed
  order (BASE_PRIORITY, CPUDEFAULT, CPUMAXIMUM, JOB_LIMIT, OWNER, PROTECTION,
  RETAIN, WSDEFAULT, WSEXTENT, WSQUOTA), wrapped. SHOW QUEUE/MANAGER/FULL
  names `SYS$SYSROOT:[SYSEXE]QMAN$MASTER.DAT`.
- **Entries**: `Entry Jobname Username Blocks Status`; Pending, `Pending
  (queue stopped)`, Holding, `Holding until ...`, Executing, `Retained on
  completion`, `Retained on error` with the condition under it, and
  `Completed date on queue Q`; `/FULL` adds `Submitted date /KEEP /LOG=...
  /PARAM=("A","B") /NOPRINT /PRIORITY=100`, `File: _SIMVAX$DUA1:[dir]f;1`,
  and for a requeued job `/RESTART=Q /RESTART_VALUE="STEP2"` and
  `(checkpointed)`. The default job priority is 100. Jobs run by priority,
  highest first, then in the order submitted; a timed job when its time
  comes. SET ENTRY of an executing job is `%JBC-E-EXECUTING, job executing
  on queue prevents requested operation`; STOP/QUEUE/ENTRY of a job not
  executing `%JBC-E-JOBNOTEXEC`; a missing entry is NOSUCHENT to SHOW ENTRY
  and SYNCHRONIZE, NOSUCHJOB to SET ENTRY and DELETE/ENTRY
  (`%DELETE-E-NOTDELETED, error deleting n` `-JBC-E-NOSUCHJOB`), and to DELETE/ENTRY
  naming another queue.
- **SYNCHRONIZE** returns the job's final status with the inhibit bit
  (`%X10000001`, `%X10018292`, `%X100020AC` for EXCPUTIM, `%X10048084`
  JOBABORT, `%X100480EC` QUERESET, `%X10000000` for a stopped process); a
  job already gone is NOSUCHENT; Ctrl/Y interrupts it.
- **How a job ends**: STOP/QUEUE/ENTRY and DELETE/ENTRY of a running job
  `%JBC-F-JOBABORT`; STOP/QUEUE/RESET (and SHUTDOWN) `%JBC-F-QUERESET`;
  STOP of the process status 0, "Retained on completion"; `/CPUTIME` run
  out `%SYSTEM-F-EXCPUTIM`; a file ending with an error or severe status
  ends the job, a warning (or DCL's IVVERB) goes on; the job's status is
  the last file's. `/NOTIFY` writes `Job NAME (queue Q, entry n) completed`
  at the user's terminal, or `terminated with error status` and the
  condition. `/RETAIN=ERROR` (queue or job) keeps a job that failed.
- **STOP/QUEUE/REQUEUE/ENTRY** restarts a running job whether or not it
  was `/RESTART`: `$RESTART` "TRUE", `BATCH$RESTART` the last SET
  RESTART_VALUE, and a new version of its log. With `/HOLD` it is held
  only once its run has ended (`%JBC-E-EXECUTING` to a release before).

**Protection and privileges** (A.3, 2), from the matrix (three accounts,
six states, BAT98O once per privilege):

- A queue's access is Phase 93's rule on its owner, protection (R, S, M,
  D), and ACL (READ, SUBMIT, MANAGE, DELETE, CONTROL): BYPASS grants
  everything; SYSPRV the system category's access; GRPPRV the same, but
  only in the queue owner's group; READALL R. **OPER** grants everything,
  and is the only way to create a queue (`%JBC-E-NOOPER, operation
  requires OPER privilege`); SET SECURITY needs ownership or CONTROL.
- Manage (STOP, START, SET QUEUE, INITIALIZE of an existing queue,
  DELETE/QUEUE) needs M; SUBMIT S (`%SUBMIT-F-CREJOB` `-SYSTEM-F-NOPRIV`).
- A **job** is checked against the same protection and ACL with **the
  job's owner** as the owner; M on the queue allows anything on any job in
  it. So one's own job needs the O field's D (no D, as with (S,O,G,W) and
  an ACL of READ+SUBMIT, refuses SET ENTRY and DELETE/ENTRY of one's own
  job); another's needs D by the ACL or M on the queue. (Corrected in
  subtask 5: this first said "by its categories or the ACL". Every case
  fits the Guide to System Security's 4.8.7.2 instead: read and delete
  given by the protection code reach only one's own jobs, given by an ACE
  every job; and M implies R, S, and D, 4.6.3.) SYNCHRONIZE of
  another's job needs **R** on it (READALL suffices), not D as the DCL
  Dictionary says.
- Without R, SHOW QUEUE and SHOW ENTRY list another's job as `no
  privilege` in place of its name and user, and `/FULL` says `Access to
  security information denied.` in place of the owner and protection.
- `/PRIORITY` above MAXQUEPRI is `%JBC-I-PRIOSMALL, scheduling priority has
  smaller value than requested` without OPER or ALTPRI.
  `/USER` needs CMKRNL (`-JBC-E-NOCMKRNL`), and the UAF readable: an
  account authorized for SYSPRV could, one not was `%SUBMIT-F-INVQUAVAL`
  with `-RMS-E-PRV` (unconfirmed which privilege SUBMIT turns on).

**The batch job's process** (A.3, 3), the same for SYSTEM and each
account: process name `BATCH_n`; JOBTYPE 2 (BATCH), MODE BATCH,
CREPRC_FLAGS 16 (PRC$M_BATCH); no owner, its own master, no terminal; PRIB
the queue's base priority, and AUTHPRI too (PRI one higher, boosted); its
CPU limit the queue's `/CPUDEFAULT`; the working-set values the account's
(the queue's smaller `/WSDEFAULT`, `/WSQUOTA`, `/WSEXTENT` didn't lower
them: unconfirmed); its rights BATCH (Phase 95); SYS$INPUT, SYS$COMMAND,
SYS$OUTPUT, and SYS$ERROR process-permanent names of the job's disk (the
command file, and the log); TT `_NLA0:`; SYS$LOGIN, SYS$LOGIN_DEVICE, and
SYS$SCRATCH executive-mode names in the job table, SYS$DISK in the
process table, the default directory SYS$LOGIN. **Its files run at
command level 0** (`F$ENVIRONMENT("DEPTH")` 0, the job's own command
stream, not `@`); SYLOGIN and LOGIN.COM run first (LOGIN.COM at depth 1);
the first file is verified as the login procedures leave verify (off on
that VAX), each later file is verified; `$RESTART` is a global "FALSE"
(BATCH$RESTART undefined) until a restart. F$GETQUI of its own job
(DISPLAY_JOB with THIS_JOB) answers every item; DISPLAY_QUEUE with THIS_JOB
needs an object-id (ARGREQ). The log ends with LOGOUT's report (`BAT98 job terminated at date`,
indented, and the accounting lines), and has no heading.

**The log file** (A.3, 4): by default `SYS$LOGIN:jobname.LOG`; a
directory alone takes `jobname.LOG`; a name alone goes in SYS$LOGIN with
type `.LOG`; it is the job's user's, with that user's default protection.
With no SYS$PRINT queue, the default (implied) `/PRINTER` keeps the log;
an explicit `/PRINTER` fails at SUBMIT (NODSTQUE); `/NOPRINTER` keeps it
too, and `/NOKEEP` deletes it. A log the user can't create ends the job at
login, retained on error with `%RMS-E-PRV` and OPCOM's "Batch process login
failure".

**Restarts** (Decision 11): after a reboot each queue was **stopped**; a
`/RESTART` job that was running was requeued, `Pending (queue stopped)`,
`(checkpointed)`, with its restart value, and ran from it at START/QUEUE; a
plain one was "Retained on error", `%JBC-F-QUERESET`.

**The services** (A.3, 7): `defined.txt` has 982 `SJC$`, `QUI$`, and `JBC$`
values. R0 is SS$_NORMAL and the status is in the IOSB, but for an unknown
function or item code (`SS$_BADPARAM` in R0, not JBC's INVFUNCOD or
INVITMCOD) and a missing file (RMS$_FNF in R0). `$SNDJBC`'s longword output
items must have 4-byte buffers (INVPARLEN); `$GETQUI` takes any length,
truncating. CREATE_QUEUE of an existing queue succeeds; a wildcard
DISPLAY_QUEUE gives NOMOREQUE after the last match and then starts again.
Round 2's job functions all work: ENTER_FILE returns the
entry in ENTRY_NUMBER_OUTPUT, and JOB_STATUS_OUTPUT is **text**, SUBMIT's
own message (`Job QJOB (queue BAT98_Q, entry n) holding`, cut to the
buffer); ALTER_JOB changes a held job and releases it (NO_HOLD);
SYNCHRONIZE_JOB returns the job's own status in the IOSB (SS$_NORMAL);
CREATE_JOB, ADD_FILE, and CLOSE_JOB make one job of several files, and
CLOSE_JOB with none open is NOSUCHJOB. `$GETQUI`'s items have their own
sizes (PRIORITY one byte, UIC and counts a longword, times a quadword,
names their length); DISPLAY_FILE straight after a DISPLAY_ENTRY without
a wildcard is NOJOBCTX; a nested DISPLAY_JOB walk gives NOMOREJOB after the
last job, then NOSUCHJOB. `$GETMSG` matches a JBC code whatever its severity bits, and knows the
system messages the facility shares; the JBC messages proper are
`defined.txt`'s values.

**DCL** (not queues): a Ctrl/Y in a procedure at command level 2, then
SPAWN and CONTINUE, went on with the procedure, where the *User's Manual*
(13.8.1) says it exits; govax's `dclcontroly.go` follows the manual, and
is to be checked.

## Part B — decisions

**Decision 1 — scope.** Batch queues only: no printer, server, or
terminal queues, no generic queues, no forms or characteristics, no
autostart, no clusters (one queue manager, `SYS$QUEUE_MANAGER`, on node
GOVAX). VMS's PRINT stays out (govax's `PRINT` verb is its own), so
`SUBMIT/PRINTER` can't print (Decision 7). Retiring govax's `PRINT` and
adding VMS's PRINT with print queues is a later phase (the review). A qualifier VMS has that
govax doesn't keep is refused by the grammar (`%CLI-W-IVQUAL`), and HELP
says so, as Phase 95 did for AUTHORIZE.

**Decision 2 — `internal/jobqueue`, a leaf package.** The queue
database and its rules as plain Go, as `internal/uaf` and `internal/sched`
are: `Queue` (name, kind, attributes, owner, protection, ACL, state),
`Job` (entry, name, user, files by file ID and name, parameters, log
file, options, status, times), `Database` (the queues, the jobs, the next
entry number, the manager's state), and the rules: state changes (a
table of which operation is allowed in which state, with the JBC status
for each refusal), entry numbers, which job a queue runs next (holding,
time, priority, then submission order), and `Load`/`Save`. It knows
nothing of processes, DCL, or the VAX. Its tests drive every state
change.

**Decision 3 — where the database lives.** JSON on the host, as the
authorization file is: `~/.govax/qman.json` unless a new key,
`vax.queue.file`, names another; indented, versioned, written whole by
a temporary file and a rename under `internal/hostlock`, mode 0600. The
reasons are Phase 95's: it must survive the system volume being
rebuilt (`--system-initialize`) and be shared safely by two sessions.
START/QUEUE/MANAGER's directory parameter is accepted and recorded (SHOW
QUEUE/MANAGER shows it), but doesn't move the file. (Accepted, the
review's answer 1.) `cmd/govax`'s `TestMain` gives tests a temporary file, as it does the
authorization file.

**Decision 4 — one session runs the manager.** The queue manager runs in
the first session that starts with the manager's state "started" in the
database (or that gives START/QUEUE/MANAGER), holding a host lock beside
the file for as long as it runs: that session is the batch initiator
(the review's answer 2). A second session while the lock is held
may show queues and entries (read from the file), but every other
operation, and starting a manager, is refused with the JBC status VMS
gives when the manager isn't running here (the probe's
`JBC$_QMANNOTSTARTED`, which VMS 7.1 has). Running another session's jobs
would mean running them against that session's mounts and terminal,
which govax can't do.

**Decision 5 — the queue manager in Go.** The manager is Go code in
`corevms` (`System.Queues`), not a VAX process, as the modified page
writer and the reclaimer are: it has no PID and no QUEUE_MANAGER or
JOB_CONTROL process appears in SHOW SYSTEM (DEVIATIONS.md will say so).
It is asked at every scheduling call, like `pollEvents`, and runs
whatever is due: a held-until time reached, a queue with room and a
pending job. Its next `/AFTER` time is a timer `nextTimer` knows, so an
idle system moves time forward to it, and a `WAIT` or the prompt lets it
come. Changes are written to the database at once (so a crash loses
nothing). `$SNDJBC` and `$GETQUI` are system services that call it
(Decision 9), and DCL's commands use the services' own Go code from
supervisor mode, as Phase 94's DEFINE uses `$CRELNM`'s.

**Decision 6 — the batch job's process** (held to A.4). The manager
starts a job by creating a detached process in Go through `$CREPRC`'s
code: no owner, its own job and master, no terminal, `PRC$M_BATCH`
(CREPRC_FLAGS 16), named `BATCH_n` for its entry. Its SYS$INPUT and
SYS$COMMAND are a process-permanent entry for the job's command stream
(the files), and SYS$OUTPUT and SYS$ERROR one for the log (Phase 71's
entries); TT is NL:. It is logged in by `Environment.LogIn` with a
non-interactive `Login` for the job's user (DISUSER refused as LOGINOUT
refuses it, the job ending with that status in the log), and then:

- its rights BATCH plus the account's identifiers (today's
  `batchRights`), so an ACE naming BATCH works for it;
- its base priority **and its authorized priority** the queue's
  `/BASE_PRIORITY`;
- its CPU limit the job's `/CPUTIME`, else the queue's `/CPUDEFAULT`, at
  most the queue's `/CPUMAXIMUM`; running out ends it with
  SS$_EXCPUTIM;
- its working-set values the account's: the probe's smaller queue values
  didn't lower them, so govax doesn't apply the queue's or the job's
  working-set qualifiers to the process yet (kept and shown; unconfirmed,
  DEVIATIONS.md);
- JPI$_JOBTYPE and JPI$_MODE BATCH (`jobType`, `jobMode`), `F$MODE()`
  "BATCH", OWNER empty, MASTER_PID its own, TERMINAL empty;
- SYS$LOGIN, SYS$LOGIN_DEVICE, and SYS$SCRATCH in the job table and
  SYS$DISK in the process table, executive mode, the default directory
  SYS$LOGIN (Phase 95's Decision 7).

DCL then runs as a batch CLI (a new `CLIStartup.Batch`): the login
procedures first (Phase 95's `RunLoginProcedures`, made to work for any
process's Shell; LOGIN.COM at depth 1), then the job's files **at command
level 0, each as the job's own command stream** (F$ENVIRONMENT("DEPTH") 0,
P1 to P8 the job's parameters, no `@`). The first file keeps the verify
state the login procedures leave; each later one starts verified. A file
ending with an error or severe status ends the job; a warning goes on. The
global `$RESTART` is "FALSE", and on a restart "TRUE" with `BATCH$RESTART`
the last SET RESTART_VALUE, from which the restarted file goes on.
LOGOUT's report ends the log; the job's status is its last file's.

**Decision 7 — the log file** (held to A.4). By default
`SYS$LOGIN:jobname.LOG`; `/LOG_FILE` fills in the rest from that (a
directory alone takes the job's name, a name alone SYS$LOGIN); its
directory is checked at SUBMIT (INVLOGFIL); `/NOLOG_FILE` is NL:. It is
created by the job's process as its user, with the user's default
protection, so Phase 93's protection applies: a log the user can't create
ends the job at login, retained on error with RMS$_PRV. With no output
queues (Decision 1) nothing is printed, as on a VAX with no SYS$PRINT: the
implied `/PRINTER` keeps the log, an explicit `/PRINTER` is NODSTQUE at
SUBMIT, `/NOPRINTER` keeps it, and `/NOKEEP` deletes it. In practice a
batch job's `.LOG` file lands on the disk for the user to read (the
review's answer 4).

**Decision 8 — protection and privileges** (held to A.4). A queue is a
protected object of a new security class, QUEUE, in `securityClasses`, with
access types READ, SUBMIT, MANAGE, DELETE, and CONTROL; owner `[SYSTEM]`
(`[1,4]`) and protection `(S:M,O:D,G:R,W:S)` by default; an ACL. SET
SECURITY/CLASS=QUEUE and SHOW SECURITY/CLASS=QUEUE (and
`$GET_SECURITY`/`$SET_SECURITY` with class QUEUE) reach it, and so do
INITIALIZE/QUEUE's and SET QUEUE's `/OWNER_UIC` and `/PROTECTION`.
`internal/security`'s rule decides, with the queue's access types:

- a queue operation asks the queue: S to SUBMIT, M to manage (START, STOP,
  SET QUEUE, INITIALIZE of an existing queue, DELETE/QUEUE), R to see it;
  ownership or CONTROL for its security;
- a job operation asks the same protection and ACL **with the job's owner
  as the owner**, and M on the queue allows anything on any job in it: D
  to change, delete, or stop a job (one's own included, through the O
  field), R to see it or SYNCHRONIZE with it; the code's R and D reach
  only one's own jobs, an ACE's every job, and M implies R, S, and D
  (subtask 5, from the Guide to System Security and the matrix);
- privileges as the rule has them: BYPASS everything, SYSPRV the system
  category, GRPPRV the system category in the owner's group, READALL R;
  and **OPER everything** (`ruleQueueOperator`).

The rules in `privrules.go`, through the gate: `ruleQueueOperator` (OPER,
over any queue's or job's protection, and to create a queue: NOOPER
otherwise), `ruleQueueManager` (OPER and SYSNAM, START and STOP
/QUEUE/MANAGER), `ruleSubmitUser` (CMKRNL for `/USER`, NOCMKRNL; reading
the UAF by Phase 95's Decision 5), `ruleQueuePriority` (OPER or ALTPRI,
`/PRIORITY` above MAXQUEPRI, PRIOSMALL). A refusal is `JBC$_NOPRIV` (or
the rule's own status), shown as A.4 shows it; without R, SHOW lists a job
as `no privilege` and `/FULL` omits the security.

**Decision 9 — the services** (held to A.4). `$SNDJBC`, `$SNDJBCW`,
`$GETQUI`, and `$GETQUIW` are system services (`corevms/sndjbc.go`,
`getqui.go`) taking VMS's function codes and item lists for what
Decision 1 keeps. As VMS's: R0 is SS$_NORMAL and the status is in the
IOSB, but for an unknown function or item code (SS$_BADPARAM in R0) and a
file that can't be opened (RMS's status in R0); `$SNDJBC`'s longword
outputs must be 4 bytes long (INVPARLEN); `$GETQUI` truncates to any
buffer; a wildcard search gives NOMOREQUE after its last match. A function
or item Decision 1 leaves out (forms, characteristics, printers) is
`JBC$_NOTSUPPORTED`. The definitions come from `testdata/bat98/defined.txt`
into `vmsdef`, the JBC messages' texts from round 1's `$GETMSG` dump, and
the macros `$SJCDEF`, `$QUIDEF`, and `$JBCMSGDEF` into `starletdef.mar`,
with `$SNDJBC` and `$GETQUI` service macros in `starlet.mar`, checked
against real MACRO's objects as earlier service macros were. `F$GETQUI`
stops being LEXNOTIMPL and uses `$GETQUI`'s code. `LIB$GETQUI` waits.

**Decision 10 — the commands and qualifiers.** The author's minimum,
and what the plan recommends adding because a queue is hard to use
without it (each marked; all accepted, the review's answer 6):

| Command | Qualifiers |
| --- | --- |
| START/QUEUE/MANAGER | (directory parameter) |
| STOP/QUEUE/MANAGER | `/CLUSTER` accepted |
| SHOW QUEUE/MANAGER | `/FULL` |
| INITIALIZE/QUEUE | `/BATCH` (without it VMS makes a printer queue; govax refuses, NOTSUPPORTED), `/START`, `/JOB_LIMIT`, `/BASE_PRIORITY`, `/OWNER_UIC`, `/PROTECTION`, `/WSDEFAULT`, `/WSQUOTA`, `/WSEXTENT`, `/CPUDEFAULT`, `/CPUMAXIMUM`, `/RETAIN`, `/DESCRIPTION` |
| START/QUEUE | the same attributes, as VMS allows |
| STOP/QUEUE | `/NEXT`, `/RESET`, `/ENTRY=n` (abort a job), `/REQUEUE` (restart it) and `/HOLD`; without `/NEXT` the queue pauses, suspending its jobs |
| DELETE/QUEUE | |
| SET QUEUE | the attributes (recommended) |
| DELETE/ENTRY | `/QUEUE` |
| SET ENTRY | `/HOLD`, `/RELEASE`, `/AFTER`, `/PARAMETERS`, `/NAME`, `/PRIORITY`, `/LOG_FILE` (recommended) |
| SHOW QUEUE | `/BATCH`, `/ALL_JOBS`, `/FULL`, `/BRIEF`, `/BY_JOB_STATUS`, `/SUMMARY` (no `/USER_NAME`: VMS 7.1's has none) |
| SHOW ENTRY | `/FULL`, `/USER_NAME` (recommended) |
| SUBMIT | `/AFTER`, `/USER`, `/PARAMETERS`, `/HOLD`, `/LOG_FILE` (the minimum); `/QUEUE`, `/NAME`, `/KEEP`, `/PRINTER`, `/NOTIFY`, `/PRIORITY`, `/CPUTIME`, `/WSDEFAULT`, `/WSQUOTA`, `/WSEXTENT`, `/RESTART`, `/RETAIN`, `/NOTE`, `/IDENTIFY`, `/DELETE` (recommended) |
| SYNCHRONIZE | `/ENTRY`, `/QUEUE` (recommended) |
| SET RESTART_VALUE | with `BATCH$RESTART` and `$RESTART` (recommended, for `/RESTART`) |

`/NOTIFY` writes A.4's completion message to the user's terminal through
`$BRKTHRU`'s code when the user is logged in at the console. Every
message, layout, and status is A.4's.
SUBMIT's file selection qualifiers (`/BEFORE`, `/SINCE`, `/CONFIRM`,
`/EXCLUDE`, ...) and `/CLI`, `/REMOTE`, `/CHARACTERISTICS` are left
out.

**Decision 11 — the session's start and end.** At startup, after
STARTUP.COM and before the login procedures, govax starts the manager if
the database says it was running (Decision 4's lock permitting), as VMS
starts it at boot. Every queue comes back **stopped**, as VMS's
non-autostart queues do: the user decides which to start again (the
review's answer 3), usually in SYSTARTUP_VMS.COM.

No queue exists until a user makes one. The seeded SYSTARTUP_VMS.COM
(written once, the user's to edit) gains a commented block that, when
uncommented, starts the queue manager, creates SYS$BATCH if
`F$GETQUI` finds none, and starts it:

    $! $ START/QUEUE/MANAGER
    $! $ IF F$GETQUI("DISPLAY_QUEUE","QUEUE_NAME","SYS$BATCH") .EQS. "" -
    $!      THEN INITIALIZE/QUEUE/BATCH SYS$BATCH
    $! $ START/QUEUE SYS$BATCH

(the exact lines are the probe's to confirm). A volume whose
SYSTARTUP_VMS.COM was seeded before this phase doesn't get the block,
since seeded files are never replaced; HELP and MANUAL.md give it to
copy. This needs `F$GETQUI`, so it comes with subtask 10. (The review
allowed a SYS$BATCH supplied by default if this proved impractical; it
doesn't.)

A job that was executing when the last session ended is treated as VMS
treats one at a shutdown (A.4): requeued if it was `/RESTART`, pending,
checkpointed, with `$RESTART` true and `BATCH$RESTART` its last restart
value; otherwise ended with `JBC$_QUERESET` (and retained as `/RETAIN`
says). Pending, holding, and timed jobs simply wait. When
the user asks govax to end (LOGOUT, EXIT at level 0, QUIT, Ctrl/Y's
end) while jobs are executing, govax says how many and asks for
confirmation; on yes it ends them that way, on no it goes back to the
prompt (the review's answer 5). With no terminal to ask at (input from
a pipe or a test) it doesn't ask: it says so and ends them, as the
password rule does without a terminal (Phase 95's Decision 6). The
one-shot subcommands don't start the manager.

**Decision 12 — tests.** `internal/jobqueue`'s unit tests for every
state change; oracle tests holding each command's output and `$STATUS`
to the probe's logs, as each identity; a batch job's view of itself
held to the probe's job logs, the log file's text (times masked, as
`maskVolumeDetails` does) included; `TestEveryPrivilegeRuleUsed` and
`TestNoUncheckedPrivileges` keep covering the new rules; the help tests
(`TestEveryVerbHasHelp`, `TestEveryQualifierHasHelp`) the new verbs.
All with the quantum clock, so `/AFTER` is deterministic.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The probe** (done 2026-11-16): `testdata/bat98`, as in Part D,
   written and rehearsed where govax can; run on the VAX by the author in
   two rounds; A.3 answered (as A.4) and Part B adjusted.
3. **Definitions** (Decision 9; done 2026-11-16): `SJC$_`, `QUI$_`, `JBC$_`, and the
   flags in `vmsdef` from the probe's decoded objects; the JBC and QMAN
   messages; `$SJCDEF`, `$QUIDEF`, and the service macros in bootdata.
4. **The database** (Decisions 2, 3; done 2026-11-16): `internal/jobqueue`, its JSON file
   and locking, `vax.queue.file` (`validConfigs`, HELP CONFIGURATION
   KEYS), `TestMain`'s temporary file.
5. **Queue protection** (Decision 8; done 2026-11-16): the QUEUE security class, the
   access rule's queue and job types, the privilege rules, SET and SHOW
   SECURITY/CLASS=QUEUE; held to the probe's protection matrix.
6. **The manager and queues** (Decisions 4, 5, 10; done 2026-11-16): `System.Queues`,
   `$SNDJBC`'s queue functions, START/STOP/SHOW QUEUE/MANAGER,
   INITIALIZE/QUEUE, START/QUEUE, STOP/QUEUE (`/NEXT`, `/RESET`), SET
   QUEUE, DELETE/QUEUE, SHOW QUEUE; held to the probe.
7. **Jobs, not yet run**: `$SNDJBC`'s job functions, SUBMIT and `$ENTRY`,
   SHOW ENTRY, SET ENTRY, DELETE/ENTRY, `/AFTER` and `/HOLD` states, on a
   stopped queue; held to the probe.
8. **The batch job** (Decisions 6, 7): the manager starting jobs, the
   process and its login, the log file, DCL's batch CLI (verify, login
   procedures, the files and parameters, the end), the job limit and
   ordering, `/AFTER`'s timer, STOP/QUEUE/ENTRY, SYNCHRONIZE, `/NOTIFY`;
   held to the probe's job logs.
9. **`$GETQUI` and `F$GETQUI`** (Decision 9), and `LIB$GETQUI` if the
   probe makes it simple; SHOW QUEUE and SHOW ENTRY moved onto
   `$GETQUI`'s code if they aren't already.
10. **Restart and the session's end** (Decision 11): the manager at
    startup, queues back stopped, executing jobs at the end and the
    confirmation, `/RESTART`, SET RESTART_VALUE, `/RETAIN`, a second
    session's refusals, SYSTARTUP_VMS.COM's commented block.
11. **End to end**: a scenario as two accounts (ADDUSER.COM's), each
    submitting jobs to queues with different protections, a nightly job
    by `/AFTER`, a job that fails, and one that's restarted after govax
    is stopped and started again.
12. **The close-out**: MANUAL.md (a section on batch queues), HELP for
    each new verb and form, `lexicals.hlp`'s F$GETQUI, VAX-VMS-INTERNALS.md
    (processes), DEVIATIONS.md (the manager in Go, Decision 5; anything
    unconfirmed), PLAN.md, CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/bat98/`, on an exchange volume as the earlier probes were;
`gen.go` writes every file but the README and `decode.go`, and the
README has the run's steps. It works on queues of its own (`BAT98_A`,
`BAT98_B`, `BAT98_P`, `BAT98_D1` to `BAT98_D13`, and a few a case makes
and deletes), never changes SYS$BATCH or any other of the VAX's queues,
and needs the queue manager running. Every wait is bounded: a job SYSTEM
waits for ends by itself, and a wait for a job to start looks for a
marker file the job writes, a limited number of times.

- **Accounts.** `bat98acct.com` and `bat98drop.com` (as Phase 95's), run
  by SYSTEM before and after: BAT98 `[200,10]` (TMPMBX and NETMBX; holds
  the identifier BAT98_ID), BAT98O `[200,11]` (authorized for every
  privilege, TMPMBX and NETMBX by default, so the matrix can turn each
  on), and BAT98W `[300,10]` (another group), each with a home and a
  LOGIN.COM that says when it ran. Their AUTHORIZE commands are govax's
  as well as VMS's (no `/NOPWDEXPIRED`: nothing logs in with a password),
  so the later oracle tests can make the same accounts.
- **`bat98.com`**, SYSTEM's run, 387 cases in sections: the queue
  commands with each attribute and error; SUBMIT's qualifiers on a
  stopped queue (each `/AFTER` form, `/PARAMETERS` with nine and with a
  260-character one, `/USER`, `/PRINTER=missing`, several files with `,`
  and `+`, a wildcard, a missing file) and SHOW, SET, and DELETE/ENTRY of
  the jobs; running jobs (several files ending with each severity and an
  unknown command, `/NOTIFY`, `/CPUTIME`); endings (STOP/QUEUE/ENTRY,
  DELETE/ENTRY, STOP of the process, STOP/QUEUE, `/NEXT`, `/RESET`, SET
  ENTRY of an executing job); the job limit; retained jobs; SYNCHRONIZE;
  `/DELETE`; `/AFTER` on a running queue; where the log goes, in twelve
  forms (three more with the default `/PRINTER`, only when the VAX has no
  SYS$PRINT); the order jobs run in, by priority and time; `/RESTART`
  with STOP/QUEUE/REQUEUE (and `/HOLD`), so SET RESTART_VALUE,
  `BATCH$RESTART`, and `$RESTART` show without a reboot; BAT98J.COM as
  SYSTEM and each account; the services; the protection matrix.
- **`bat98p.com`**, the protection matrix, run by SUBMIT/USER as each
  account against `BAT98_P` in six states (A.3, 2): owned by `[1,4]` with
  the default protection; owned by `[200,10]` (BAT98's) with the default,
  with none, with W:M, with none and two ACL entries (BAT98_ID and
  `[300,10]`), and with (S:M,O:R,G:D,W). Each pass tries 31 commands:
  SHOW QUEUE and SHOW ENTRY of SYSTEM's held and retained jobs, F$GETQUI,
  SUBMIT of its own (and with `/PRIORITY=200` and `/USER`), SET ENTRY and
  DELETE/ENTRY of its own and SYSTEM's, SYNCHRONIZE, STOP/QUEUE/ENTRY,
  STOP and START, SET QUEUE, INITIALIZE/QUEUE of a new queue and of its
  own stopped queue `BAT98_Dn`, SET and SHOW SECURITY on it, and
  DELETE/QUEUE of it. BAT98O runs 13 passes: none, then OPER, ALTPRI,
  CMKRNL, CMKRNL with SYSPRV, SYSNAM, BYPASS, SYSPRV, GRPPRV, READALL,
  SECURITY, WORLD, and GROUP enabled.
- **`bat98j.com`**, a batch job's view of itself (A.3, 3): P1 to P8,
  F$MODE, verify, `$RESTART`, every F$ENVIRONMENT and F$GETJPI item,
  F$GETQUI of its own job and queue, the SYS$ names, the job and process
  tables, SHOW PROCESS/ALL, /PRIVILEGES, /RIGHTS, /QUOTAS, the symbols,
  and a line to SYS$ERROR.
- **`bat98q.mar`** (and `def_sjc.mar`, `def_qui.mar`, `def_jbc.mar`,
  `GLOBAL`, values only, with `decode.go` making `defined.txt`): 59 calls
  of `$SNDJBCW` and `$GETQUIW`, each showing R0, the IOSB, and every
  output item's length and bytes (queue and job functions, wildcard and
  nested contexts, CREATE_JOB/ADD_FILE/CLOSE_JOB, a short buffer, bad
  function and item codes, no IOSB), then the JBC facility's messages by
  `$GETMSG`. It uses only codes VMS 5.0's System Services manual already
  had, so no name 7.1 lacks can stop its assembly.
- **By hand** (the README's step 4): SYNCHRONIZE interrupted by Ctrl/Y,
  and, if the author is willing, a reboot of the simh VAX with a
  `/RESTART` job and a plain one executing, to see what each becomes
  (Decision 11). `/NOTIFY`'s messages reach the terminal during the run.

## The review

Questions for the author:

1. **Where the database lives** (Decision 3): JSON on the host,
   `~/.govax/qman.json` (`vax.queue.file`), as the authorization file;
   or a file on the system volume, in SYS$COMMON:[SYSEXE] as VMS's,
   which `--system-initialize` would lose? *Recommendation:* the host.
2. **Two sessions** (Decision 4): only the session holding the
   manager's lock runs jobs and changes queues; another may only show
   them. *Recommendation:* yes.
3. **Queues after a restart** (Decision 11): come back in the state they
   were left in, or stopped, as VMS's do unless SYSTARTUP_VMS.COM starts
   them? And should a new system have a SYS$BATCH made and started by
   the supplied STARTUP.COM? *Recommendation:* the state they were left
   in, and no SYS$BATCH until a user makes one (the seeded
   SYSTARTUP_VMS.COM gains commented lines showing how, for new volumes).
4. **The log file and printing** (Decision 7): with no print queues,
   follow what the probe shows VMS doing when the log can't be printed?
   *Recommendation:* yes.
5. **Ending govax with jobs running** (Decision 11): ask for
   confirmation at LOGOUT or EXIT, then end them as a crash would; or
   end them silently; or refuse to exit? *Recommendation:* ask.
6. **The qualifiers** (Decision 10): the minimum plus the ones marked
   recommended? *Recommendation:* yes.
7. **The services** (Decision 9): `$SNDJBC`, `$GETQUI`, and `F$GETQUI`
   for programs and procedures, not only DCL's commands?
   *Recommendation:* yes; DCL's commands are built on them, as VMS's
   are.
8. **The probe** (Part D): may the BAT98 accounts and the BAT98_A and
   BAT98_B queues be made on the simh VAX, by scripts, as Phase 95's
   accounts were?

The answers (2026-11-16):

1. JSON on the host, `vax.queue.file` (Decision 3).
2. Yes: the first session to run the manager is the batch initiator;
   another only shows queues (Decision 4).
3. Queues come back stopped; the user decides which to restart. No
   SYS$BATCH by default, provided the site's startup can create one if
   it doesn't exist (it can, with `F$GETQUI`, so the seeded
   SYSTARTUP_VMS.COM gets a commented block); otherwise one would have
   been supplied (Decision 11).
4. Yes: there are no print queues, so a job's `.LOG` file lands on the
   disk for the user to read (Decision 7).
5. When asked to exit with batch jobs running, ask for confirmation
   before quitting (Decision 11).
6. Yes, the recommended qualifiers too (Decision 10).
7. Yes, the queue management services (Decision 9).
8. Yes, a probe script may create accounts and queues on the simh VAX
   (Part D).

The author also agreed to the scope (Decision 1): batch queues only.
Retiring govax's own `PRINT` command and adding VMS's PRINT with print
queues is a future phase.

## Progress log

- **2026-11-16** Subtask 1: read Phases 93 to 97 for the model; looked
  for any queue code (none: `F$GETQUI` LEXNOTIMPL, `$SNDJBC` and
  `$GETQUI` names only, no `SJC$`, `QUI$`, or `JBC$` definitions); read
  `corevms`'s detached CLI processes (`cliprocess.go`), logging in
  (`login.go`, `batchRights`), `jobType`/`jobMode`, the security classes,
  the privilege rules, DCL's login procedures, and the console's grammar;
  read the DCL Dictionary's SUBMIT and the queue commands' "Requires"
  lines, and Phase 95's batch logs from the VAX; wrote Part A and this
  plan.
- **2026-11-16** The review: the author accepted the plan and answered
  the eight questions. Decision 4 names the manager's session the batch
  initiator; Decision 7 says logs simply land on the disk; Decision 11
  now brings every queue back stopped, gives the seeded
  SYSTARTUP_VMS.COM a commented create-if-missing block for SYS$BATCH
  (with `F$GETQUI`, so in subtask 10), and asks for confirmation when
  govax is asked to end with jobs executing (ending them without asking
  when there is no terminal); Decision 1 notes VMS's PRINT as a later
  phase.
- **2026-11-16** Subtask 2, written: `testdata/bat98` (Part D, now
  rewritten to match what was made) and `TestBat98Probe`
  (`internal/console/bat98_test.go`), which runs BAT98J.COM and
  BAT98P.COM at govax's terminal and BAT98.COM to its end with every
  queue command a message, checking that the probe's own DCL is right.
  The rehearsal found three mistakes of the probe's own, fixed before
  any VAX run: waits for a job to start took a minute of real time each
  when the SUBMIT made no entry (each wait now takes the entry, and
  returns at once without one); a symbol was left undefined when
  F$GETQUI failed; and `/NOPWDEXPIRED` and `/PWDLIFETIME`, which govax's
  AUTHORIZE doesn't keep, stopped the accounts being made on govax
  (dropped: nothing logs in with a password). BAT98Q.MAR was assembled
  and linked by govax with stand-in values for the definition macros,
  which govax lacks until subtask 3; it uses only codes the 5.0 manual
  has. F$GETQUI's DISPLAY_ENTRY sees only the caller's own jobs (the
  DCL Dictionary), so SYSTEM can't poll a `/USER` job by it: the probe's
  waits use marker files instead. The exchange volume is built
  (`testdata/disks/bat98-exchange.dsk`); the VAX run is the author's.
- **2026-11-16** Subtask 2, round 1 on VMS 7.1, to the end: every
  section, BAT98Q's 59 cases and the JBC messages, and all 18 matrix
  jobs (`testdata/bat98/vax`). It stalled once, on a SYNCHRONIZE of a
  job requeued `/HOLD` and released at once: the release was refused
  (`%JBC-E-EXECUTING`, the run not yet ended), and the requeue then left
  the job holding. Ctrl/Y, SPAWN, `SET ENTRY/RELEASE` in the subprocess,
  and CONTINUE went on; that CONTINUE resumed a procedure at command
  level 2, where the *User's Manual* (13.8.1) says a Ctrl/Y ends it, is
  a finding for govax's DCL to check (`dclcontroly.go`). `gen.go` now
  retries such a release (the `RELEASE` subroutine), and BAT98Q waits on
  a job only after a release that took. `bat98b.com`, a round 2 for the
  sections the stall seemed to have cost, wasn't needed. Round 1's queue
  listing already shows `/RETAIN=ERROR`'s retained jobs with each one's
  condition, and a job whose process was stopped "Retained on
  completion". Left: the steps by hand, BAT98DROP.COM, and
  `copyout2.cmd` for its log and the definition objects.
- **2026-11-16** Subtask 2, the analysis: A.4 written from round 1's logs
  (every queue command's words and status; the protection matrix, which
  shows a job checked against its queue's protection with the job's owner
  as owner, OPER over everything and alone able to create, SYNCHRONIZE
  needing R; the batch process's identity, its files at command level 0,
  verify, the log's place; a reboot's effect), and Decisions 6 to 11
  revised to it. Round 2, BAT98Q again with `$SNDJBC`'s longword outputs
  in 4-byte buffers (round 1's were 256, JBC-F-INVPARLEN), is written and on
  the volume; `defined.txt` (982 values) is in. Added to Decision 10:
  STOP/QUEUE/REQUEUE (and `/HOLD`), which `/RESTART` needs, and SHOW
  QUEUE/SUMMARY; dropped: SHOW QUEUE/USER_NAME, which VMS 7.1 lacks.
- **2026-11-16** Subtask 2, round 2 on VMS 7.1 (`vax/bat98b.log`): every
  `$SNDJBC` job function works with 4-byte longword outputs, and A.4 has
  what it showed (JOB_STATUS_OUTPUT is SUBMIT's message as text;
  SYNCHRONIZE_JOB's IOSB the job's status; `$GETQUI`'s items at their own
  sizes). Subtask 2 is done; subtask 3, the definitions, is next.
- **2026-11-16** Subtask 3, the definitions: `defined.txt`'s 982 values
  merged into `vmsdef.Symbols` (`gen -values`; `symbols.go` describes the
  SJC$, QUI$, and JBC$_ families), and `mkdefs` makes `$SJCDEF`,
  `$QUIDEF`, and `$JBCMSGDEF` of them in govax's macro library. The JBC
  messages: `vmsdef/jbcmessages.go` has the 22 facility-specific ones
  round 1 printed (`JBCFacility`; `TestJBCMessagesAsVMS` holds every JBC
  message line of `testdata/bat98/vax`'s logs to `LookupMessage`). The
  probe's `$GETMSG` dump reached only message numbers 0 to 1,023, the
  facility's shared part, and showed VMS's rule there: a shared number is
  the system message of that number under the facility's name, whatever
  the severity (JBC$_NOOPER, %X42892, is SS$_NOOPER's text as
  `%JBC-E-NOOPER`). `sharedMessage` now does that for every facility it
  can name (`otherFacilities` adds JBC). The rest of `$JBCMSGDEF`'s 142
  conditions have no text yet: a later subtask that needs one VMS never
  printed words it from the System Services manual's description and
  logs it as unconfirmed. The service macros `$SNDJBC`, `$SNDJBCW`,
  `$GETQUI`, and `$GETQUIW` (with their argument-list and `_G` forms) are
  in `starlet.mar`, written from the manuals on `$GETJPI`'s model; round
  10 of `testdata/mp/macros` (`r10_*`, `macros10.com`, its README's
  "Round 10") is their probe, for the author to run on the VAX. Until its
  objects come back, `TestServiceMacroObjects` rehearses it (each program
  assembles with govax's macros, refusing exactly the calls without
  FUNC), and the clean-room hook lists its log, `vax/macros10.log`, as
  unaudited. Subtask 4, the database, is next.
- **2026-11-16** Subtask 3, round 10 on VMS 7.1 (`testdata/mp/macros/vax`,
  its log audited by the author: clean, and out of the hook's list): real
  MACRO refused every call without FUNC, and only those, as govax's macros
  do, and govax's match the 80 calls it accepted (keywords and order,
  NULLARG a longword by value, FUNC a word, ASTPRM and ASTADR paired). The
  address arguments' sizes stay unconfirmed: every call that would have
  shown one also left out FUNC.
- **2026-11-16** Subtask 4, the database: `internal/jobqueue`, a leaf
  package (`doc.go` explains it). `Database` (the manager's state, the
  queues, the jobs, the next entry number and submission sequence),
  `Queue` (NewQueue's defaults: job limit 1, base priority 4, owner
  [1,4], `(S:M,O:D,G:R,W:S)`, stopped; `Profile` for `internal/security`'s
  rule), `Job` (files by name and file ID, parameters, the log and
  printing options, `/AFTER`, priority, limits, `/RESTART`, `/RETAIN`,
  times, the condition it ended with, a pending end or requeue, the
  restart value). The JSON file shows VMS's forms: a UIC `[g,m]`, a
  queue's code with its letters R, S, M, D (the file bits read, write,
  execute, delete), CPU times `hh:mm:ss`, a file ID `(n,s,r)`, ACEs in
  hexadecimal (so no identifier table is needed to read the file), states
  by name; `Load` names a bad value's line, `Save` writes through a
  temporary file and a rename, mode 0600; `Update` and `Read` work under
  a `hostlock` beside the file; `LockManager` is Decision 4's manager
  lock, a second file's `TryLock`. The rules are two tables,
  `queueRules` (START, STOP's pause, `/NEXT`, `/RESET`, DELETE,
  INITIALIZE of an existing queue, by state) and `jobRules` (`/HOLD`,
  `/RELEASE`, SET ENTRY's other changes, DELETE/ENTRY, STOP/QUEUE/ENTRY,
  `/REQUEUE`, the manager's start, by status), each refusal a `Refusal`
  naming its JBC$_ condition; `TestQueueRules` and `TestJobRules` run
  every cell against expectations written apart from the tables. Entry
  numbers run 1 to 999 and wrap past numbers in use (NOQUESPACE when all
  are); `NextJob` is A.4's order (priority, then submission, so a wrap
  doesn't reorder); `ReleaseDue` and `NextTime` are `/AFTER`'s timer;
  `Finish` requeues, retains, or removes (from the probe: under the
  queue's `/RETAIN=ERROR` BAT98R12's warning was kept "on completion", so
  ERROR keeps any status whose low bit is clear and "on error" is a
  severity of error or severe, `RetainedOnError`; DELETE/ENTRY's running
  BAT98R11 was gone, STOP/QUEUE/ENTRY's BAT98R10 retained); `Recover` is
  Decision 11's start (queues stopped, `/RESTART` jobs requeued with
  their restart values, the rest QUERESET). Choices not settled by the
  probe are in the tables' comments, unconfirmed: START of a paused or
  stopping queue continues it; `/NEXT` of a paused one leaves it
  stopping; SET ENTRY/RELEASE of a timed job starts it now and of a
  retained one requeues it; `/HOLD` of a retained one requeues it held;
  "busy, stop pending" for a stopping queue at its limit. The file:
  `vax.queue.file` (default `~/.govax/qman.json`) in `validConfigs`, HELP
  CONFIGURATION KEYS, and MANUAL.md's table; `cmd/govax/queuefile.go`'s
  `ensureQueueFile` checks it before startup (a bad file stops govax, as
  a bad authorization file does; a missing one is an empty database, not
  written until a queue is made) and hands the path to
  `Machine.QueueFile`, `System.SetQueueFile` (`corevms/queues.go`, which
  subtask 6's manager will use); `TestMain` gives the tests a temporary
  one; `TestRun_queueFile`. Subtask 5, queue protection, is next.
- **2026-11-16** Subtask 4, after its commit: `TestNoUncheckedHostAccess`
  listed `jobqueue`'s `Load` and `LockManager`; both are in
  `hostAccessReasons` now (the queue database is govax's own configured
  file, like the authorization file).
- **2026-11-16** Subtask 5, queue protection. The rule is
  `internal/security/queue.go`: `CheckQueue` (Evaluate's access with what
  M implies, R, S, and D; READALL's read) and `CheckJob`, with
  `QueueAccessNames` (READ, SUBMIT, MANAGE, DELETE, CONTROL) and a queue's
  letters for SET DEBUG PROTECTION's lines. Tabulating the matrix's 90
  passes showed A.4's summary of the job rule was too wide: VMS 7.1 fits
  the Guide to System Security's 4.8.7.2, the code's read and delete
  reaching only the process's own jobs (the code read with the job's owner
  as the owner), an ACE's every job, and M on the queue every job. A.4 and
  Decision 8 are corrected. `corevms/queueaccess.go` is the one place the
  decisions are made: `QueueAccess`, `JobAccess`, `QueueSecurityVisible`
  (R or control: SHOW SECURITY, SHOW QUEUE/FULL's owner and protection,
  F$GETQUI's, which all went together in the logs), `QueueControl`,
  `MayCreateQueue` (JBC$_NOOPER), each letting OPER through by
  `ruleQueueOperator`; the QUEUE class in `securityClasses`
  (`queueObject`, for `$GET_SECURITY` and `$SET_SECURITY`, which now list
  the rule in `serviceRules`); `QueueProfile` and `ChangeQueueSecurity`,
  which change the database under its lock through `changeQueues`, where
  subtask 6 will ask whether the manager runs here. DCL:
  `dclcmd/queuesecurity.go`, SHOW SECURITY/CLASS=QUEUE and SET
  SECURITY/CLASS=QUEUE (`/PROTECTION` in a queue's letters, `/OWNER`,
  `/ACL` with `/AFTER`, `/REPLACE`, `/DELETE`, `/DEFAULT` the template
  profile, `/LOG`), on SET SECURITY's own ACL editing; a refusal is
  SS$_NOPRIV alone and success leaves SS$_NORMAL, as VMS's.
  `TestBat98ProtectionMatrix` (corevms) reads the 18 logs and holds 2,044
  cases to the rule (it fails 150 of them under A.4's first reading, and
  37 if M implies nothing); `TestQueueSecurityAsSystem` and
  `TestQueueSecurityRefused` (console) replay BAT98.LOG's cases 9 and 35
  to 42 and the matrix's 23 and 24. Left for the subtasks that check them,
  because `serviceRules` may name only registered services and a rule must
  be used: `ruleQueueManager` (OPER and SYSNAM; subtask 6), `ruleSubmitUser`
  and `ruleQueuePriority` (subtask 7). Found for subtask 7: SUBMIT/USER's
  submit access is the named user's, not the submitter's (state E let
  BAT98O submit as BAT98W only through BAT98W's ACE; in state F the
  submitter's SYSPRV didn't help), and STOP/QUEUE/ENTRY of a job that
  isn't executing is JOBNOTEXEC before any protection check. Unconfirmed:
  a job's owner may always see it (in every state where an account had a
  job its categories gave it R or D); SHOW or SET SECURITY of a queue that
  isn't there is JBC$_NOSUCHQUE; another owner by SET SECURITY needs
  `CanChangeOwner`'s rule or OPER; QUEUE's place in SECURITY_CLASS's walk
  (after LOGICAL_NAME_TABLE); a bad letter in a queue's code is SET's
  SYNTAX; `/LIKE`, `/EXCLUDE`, and `/BY_OWNER` with a queue are CONFLICT.
  MANUAL.md and HELP SET SECURITY and SHOW SECURITY say the class.
  Subtask 6, the manager and queues, is next.
- **2026-11-16** Subtask 6, the manager and queues. The manager is
  `corevms/queues.go`'s `QueueManager`, the session's: the machine makes
  one for its queue file and gives it to each System it makes
  (`System.SetQueueManager`), so an INITIALIZE/MEMORY keeps it running and
  keeps its lock; tests give a System one of its own (`SetQueueFile`). It
  runs here while the session holds `jobqueue.LockManager`'s lock
  (`managerHere`): with no database or the database saying it stopped, a
  request is JBC$_JOBQUEDIS; with another session holding the lock,
  JBC$_QMANNOTSTARTED (Decision 4); a database saying it was started, in a
  session not yet running it, starts it at the first request that needs it
  (subtask 10 moves that to startup). START/QUEUE/MANAGER (OPER and SYSNAM,
  the new `ruleQueueManager`, JBC$_NOPRIV) takes the lock and records the
  directory; a manager already running is started again silently
  (unconfirmed: a site's startup says it at every boot); STOP/QUEUE/MANAGER
  lets it go. The queue operations are `corevms/queueops.go`
  (`CreateQueue`, `StartQueue`, `StopQueue` by `QueueStop`, `SetQueue`,
  `DeleteQueue`, `QueueViews`), each checking in VMS 7.1's order: the
  manager, the name, the queue's existence, access (OPER to create,
  NOOPER; manage access to change, NOPRIV), then the state's rule from
  `jobqueue`; subtask 5's `changeQueues` now needs the manager here too.
  `$SNDJBC` and `$SNDJBCW` are `corevms/sndjbc.go`: a table of functions
  (the manager's and the queues'), an item table for the queue items,
  SS$_BADPARAM in R0 and the IOSB for a code past the function or item
  range, JBC$_NOTSUPPORTED for a function or item govax lacks (the job
  functions until subtask 7, a printer queue), MISREQPAR without
  SJC$_QUEUE, INVPARLEN for a name over 31 characters; held to round 2's
  queue cases by `TestSndjbcQueueFunctions`. The JBC texts no run showed
  (JOBQUEDIS, INVPARVAL, NOTSUPPORTED, QMANNOTSTARTED) are worded from
  VSI's $SNDJBC page, unconfirmed. DCL: the grammar's
  `initialize_queue`, START (a new verb) with `start_queue` and
  `start_queue_manager`, `stop_queue` and `stop_queue_manager` (/CLUSTER
  accepted), `delete_queue`, SET's and SHOW's QUEUE keywords, and
  `show_queue_manager`; `dclcmd/queues.go` reads the attribute
  qualifiers into a change (QUEMAN-F-INVQUAVAL for a value out of range,
  checked before the manager is asked), and DELETE/QUEUE's refusals are
  DELETE's SEARCHFAIL and NOTDELETED with the reason under them;
  `dclcmd/showqueue.go` is SHOW QUEUE's layout (the heading with its node
  once started, the description, /FULL's attributes in order of name
  wrapped at 132 columns with a blank left at the wrap, the ACL indented
  nine, or "Access to security information denied.", /SUMMARY's empty
  queue) and SHOW QUEUE/MANAGER's (/FULL's master file is the host file).
  `TestQueueCommandsOracle` replays BAT98.LOG's QUEUES section, all 68
  cases, against VMS's output but for the three of INITIALIZE/QUEUE
  without /BATCH (a printer queue; govax refuses, JBC$_NOTSUPPORTED);
  `TestQueueCommandsMatrix` holds the queue commands of the protection
  matrix's state A, as BAT98, BAT98W, and BAT98O with OPER, to their logs.
  HELP START (new), INITIALIZE /QUEUE, STOP /QUEUE, DELETE /QUEUE, SET
  QUEUE, and SHOW QUEUE. Unconfirmed: SHOW QUEUE's wrap width (between 120
  and 132), "stopped" for a manager not running, SHOW QUEUE with the
  manager stopped JBC$_JOBQUEDIS and a wildcard matching nothing
  NOSUCHQUE, INITIALIZE/QUEUE of an existing queue without /BATCH allowed,
  /PROTECTION's unnamed categories kept, and the INVQUAVAL refusals of
  values the probe gave none of. The queue's jobs in SHOW QUEUE, and what a
  pause or reset does to executing jobs' processes, are subtasks 7 and 8.
  Subtask 7, jobs not yet run, is next.
