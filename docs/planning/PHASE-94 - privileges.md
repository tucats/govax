# Phase 94 — privileges

**Status:** planned 2026-11-15 (subtask 1, the survey and this plan);
the plan reviewed and accepted by the author the same day, with every
recommendation in "The review" taken. This document is the development plan;
as the subtasks are done it becomes the implementation record, with a
progress log and, at the end, "Where things stand".

## Goal

Every operation VMS guards with a privilege is guarded in govax by the
same privilege, checked against the process's current privilege mask
(CURPRIV, `Process.CurrentPrivileges`) at the moment the operation is
asked for. This holds whether the operation comes through a system
service, a `$QIO` function, a run-time library routine, a lexical
function, or a DCL command. A refused operation fails with what VMS
gives: usually `SS$_NOPRIV`, and in some services a quieter answer
(`$SETPRI` lowers the priority it was asked to raise, `$SETPRV` returns
`SS$_NOTALLPRIV`). Each DCL command shows its own message over that
status.

    $ SET PROCESS/PRIVILEGES=(NOSYSNAM,NOSYSPRV)
    $ DEFINE/SYSTEM GREETING "HELLO"      ! refused, unless the table's
                                          ! protection grants it (A.3, 3)
    $ STOP/IDENTIFICATION=00000212        ! another group's process:
                                          ! refused without WORLD

(The messages are VMS 7.1's, which the probe will show.)

Like Phase 93, **this phase is a precursor** to identities other than
SYSTEM. Today every govax process is SYSTEM, `[1,4]`, with every
privilege enabled, so a check matters only once a program or command
turns privileges off. Nothing here may assume the caller is SYSTEM. The
checks are real and always made. The tests reach unprivileged processes
through SET PROCESS/PRIVILEGES, `$SETPRV`, `$CREPRC`'s UIC and privilege
arguments, and processes built in Go.

Much of this exists already. Phase 26's subtask 38 gave processes their
privilege masks and `$SETPRV`, and taught most services to check their
privileges. So this phase is mostly three things:

- an **audit** of every service and command against the manual, to
  find what is missing;
- **one gate** that every check goes through, driven by a table, so the
  coverage can be tested and traced;
- a **VMS probe**, because nearly every existing check was written from
  the manual alone and never held to a VAX.

## Part A — the survey (2026-11-15)

### A.1 What VMS's manuals say

Sources: the *VMS 5.0 System Services Reference Manual* (its text,
extracted with `pdftotext`, attributed to services by its page
headers), VSI's public system services pages (`VSI-system-services-1.txt`
and `-2.txt`, each "Required Privileges" section), the *Guide to
OpenVMS System Security* (6.0) chapter on privileges, the *DCL
Dictionary* (7.3-1), and *VAX/VMS Internals and Data Structures*.

**The masks.** A process has four quadword privilege masks, numbered by
`$PRVDEF` (`privilege.go` explains each): AUTHPRIV (what it may enable),
PROCPRIV (its permanent privileges), IMAGPRIV (the privileges of an image
installed with privileges), and CURPRIV, which is what services check.
While an installed image runs, CURPRIV is PROCPRIV plus IMAGPRIV, and
image rundown sets it back to PROCPRIV. `$SETPRV` and SET
PROCESS/PRIVILEGES change CURPRIV, and with the permanent flag PROCPRIV
too. Without SETPRV a process may enable only privileges it is authorized
for. A caller in kernel or executive mode may enable any.

**The privilege categories.** The Security guide's Appendix A sorts the
privileges by the harm they can do (the categories are its words):

| Category | Privileges |
| --- | --- |
| Normal | MOUNT, NETMBX, TMPMBX |
| Group | GROUP, GRPPRV |
| Devour (may use up noncritical system-wide resources) | ACNT, ALLSPOOL, BUGCHK, EXQUOTA, GRPNAM, PRMCEB, PRMGBL, PRMMBX, SHMEM |
| System | ALTPRI, AUDIT, OPER, PSWAPM, SECURITY, SYSLCK, WORLD |
| Files | SYSGBL |
| Objects | DIAGNOSE, IMPORT, READALL, VOLPRO |
| All | BYPASS, CMEXEC, CMKRNL, DETACH, DOWNGRADE, LOG_IO, PFNMAP, PHY_IO, SETPRV, SHARE, SYSNAM, SYSPRV, UPGRADE |

MANUAL.md's section on privileges (subtask 10) will use these
categories.

**Privilege or protection.** For objects that have a protection code
(files, mailboxes, global sections, common event flag clusters, logical
name tables) a privilege is one way in, and the protection is another.
For logical name tables, `$CRELNM` into a shareable table needs write
access to it, and SYSNAM (system table) or GRPNAM (group table) grants
that access whatever the protection says. A system-UIC process gets the
system category's access. Phase 93's `internal/security` rule decides
the protection side.

**Inner modes.** Several services skip their privilege check for a
caller in executive or kernel mode: `$CMKRNL` and `$CMEXEC` (no
privilege needed from an inner mode), `$ENQ`'s system-wide resources
(SYSLCK only from user and supervisor modes), and `$SETPRV` itself. The
manuals don't say the same of every service. This is an open question
(A.4).

**What each service needs.** The 5.0 manual's mentions, for the services
govax implements, with govax's state today:

| Service | Privileges the manual names | govax today |
| --- | --- | --- |
| `$ASCEFC` | PRMCEB, to create a permanent cluster | checked (`eventflags.go`) |
| `$DLCEFC` | PRMCEB, to delete a permanent cluster | checked; its "creator's UIC" condition is unconfirmed |
| `$ALLOC` | ALLSPOOL, for a spooled device | no spooled devices |
| `$ASSIGN` | NETMBX, for a network device; the device's protection | protection checked (Phase 93's rule, mailboxes); no network devices |
| `$BRKTHRU` | OPER, to all terminals or users; WORLD, to another user's terminal | checked (`operator.go`) |
| `$WAKE`, `$SCHDWK`, `$CANWAK`, `$SUSPND`, `$RESUME`, `$FORCEX`, `$DELPRC` | GROUP for another process in the group, WORLD for any | checked (`mayAffect`) |
| `$GETJPI` | GROUP, WORLD, to see other processes | checked; F$GETJPI, F$PID, SHOW PROCESS, and SHOW SYSTEM aren't (A.3) |
| `$SETPRI` | ALTPRI above the base; GROUP, WORLD for another process | checked |
| `$CMKRNL`, `$CMEXEC` | CMKRNL, CMEXEC, from user or supervisor mode | checked (`cmode.go`) |
| `$CREPRC` | DETACH (or CMKRNL) for another UIC; SETPRV to give privileges the creator lacks; ALTPRI for a higher priority; NETMBX, PSWAPM, NOACNT for those status flags | checked (`creprc.go`); an omitted prvadr's result and the child's AUTHPRIV are unconfirmed |
| `$CRELNM`, `$DELLNM`, `$CRELNT` | SYSNAM for the system table or an inner-mode name; GRPNAM for the group table; SYSPRV for the system directory; or write access by the table's protection | the privileges only (DEVIATIONS.md, "[Phase 26] Privilege simplifications") |
| `$CRELOG`, `$DELLOG` | SYSNAM, GRPNAM | checked |
| `$CREMBX` | TMPMBX or PRMMBX | checked; a permanent mailbox's logical name (group or system table) is unconfirmed |
| `$DELMBX` | PRMMBX | checked |
| `$CRMPSC`, `$MGBLSC` | SYSGBL for a system global section; PRMGBL for a permanent one; PFNMAP for PFN mapping; SHMEM for shared memory | SYSGBL, PRMGBL checked; PFNMAP refused as unsupported; no shared memory |
| `$DGBLSC` | SYSGBL, PRMGBL | checked |
| `$ENQ` | SYSLCK for a system-wide resource, from user or supervisor mode | checked |
| `$GETLKI` | SYSLCK for system locks; WORLD for other processes' locks | checked |
| `$LCKPAG` | PSWAPM | checked |
| `$ULKPAG` | PSWAPM (to confirm: the mention may be `$LCKPAG`'s page running on) | not checked |
| `$SETPRV` | SETPRV, to enable privileges not authorized | checked |
| `$SNDOPR` | OPER, to reply or enable an operator terminal; SECURITY, for the security class | OPER checked; SECURITY not |
| `$QIO`, disk | LOG_IO or PHY_IO for logical block I/O; PHY_IO for physical | checked (`disklogical.go`) |
| `$QIO`, mailbox | IO$_SETPROTECTION: the owner, or BYPASS or SYSPRV | checked; unconfirmed (`mbxdriver.go`) |
| `$QIO`, terminal | permanent characteristics (IO$_SETCHAR): LOG_IO or PHY_IO (I/O User's Reference, not the services manual) | not checked |
| `$CHKPRO`, `$CHECK_ACCESS`, `$GET_SECURITY`, `$SET_SECURITY` | BYPASS, SYSPRV, GRPPRV, READALL, SECURITY, within the protection rule | Phase 93 |

The services govax implements that need no privilege (by the manual):
`$ADJSTK`, `$ADJWSL`, the time services, `$CANCEL`, `$CANEXH`,
`$CANTIM`, `$CLI`, `$CNTREG`, `$CRETVA`, `$DACEFC`, `$DALLOC` (its own
devices), `$DASSGN`, `$DCLAST`, `$DCLCMH`, `$DCLEXH`, `$DELTVA`, `$DEQ`,
`$EXIT`, `$EXPREG`, `$FAO`, `$FAOL`, `$FILESCAN`, `$GETDVI`, `$GETMSG`,
`$GETSYI`, `$HIBER`, the event flag services, `$LKWSET`, `$ULWSET`,
`$PURGWS`, `$PUTMSG`, `$SETAST`, `$SETDFPROT`, `$SETEXV`, `$SETIMR`,
`$SETPRN`, `$SETPRT`, `$SETRWM`, `$SETSFM`, `$SYNCH`, `$TRNLNM`,
`$TRNLOG`, `$UNWIND`, `$UPDSEC`, `$ASCTOID`, `$IDTOASC`,
`$FINISH_RDB`. Their checks are access modes and protection, which
already exist.

Services govax lacks that need privileges: `$SETSWM` (PSWAPM), `$SETIME`
(OPER and LOG_IO), `$MOUNT` and `$DISMOU` (SYSNAM, GRPNAM, VOLPRO,
SHARE, OPER), `$GRANTID` and `$REVOKID` (CMKRNL), `$GETUAI` and
`$SETUAI`, the rights database services, `$GETQUI` and `$SNDJBC`.

### A.2 What govax does today

**The masks** are `corevms.Process`'s four fields (`process.go`).
Process 1 holds every privilege in all of them. `$CREPRC` gives a child
the privileges asked for, minimized by the creator's unless it holds
SETPRV, and makes the child authorized for those plus its creator's.
Image rundown copies PROCPRIV back to CURPRIV (`resetImagePrivileges`).
IMAGPRIV is always empty, because govax has no installed images.

**The checks** are calls of `Process.hasPrivilege(mask)` or
`hasAnyPrivilege(mask)` in about 20 service files, each followed by its
own `return ssNoPriv`. They are scattered: there is no one place that
says which services check what, no trace, and nothing that would notice
a new service that forgot its check. The privilege bits are package
variables in `privilege.go`, `enq.go`, `gblsec.go`, `disklogical.go`,
and `uicprot.go`. `mayAffect` (`getjpi.go`) is the GROUP/WORLD rule for
services that reach another process.

**The protection side.** Mailboxes, global sections, and common event
flag clusters check protection through `uicAccess`, which since Phase 93
is `internal/security`'s rule. Logical name tables don't: VMS lets a
process write a table its protection grants, and lets a system-UIC
process write the system table, with or without SYSNAM. govax checks
only the privilege, so SYSTEM with SYSNAM off can't write the system
table, though VMS would let it.

**Outside the services.** DCL's commands are Go code, and some of them
do privileged things without going through a service:

| Command | What VMS needs | govax today |
| --- | --- | --- |
| DEFINE, ASSIGN `/SYSTEM` | SYSNAM, or the table's protection | unchecked (`dclcmd/logical.go` calls `lnm.Database.Define`) |
| DEFINE, ASSIGN `/GROUP` | GRPNAM, or the protection | unchecked |
| DEFINE `/EXECUTIVE_MODE` | SYSNAM, or the mode is lowered to supervisor | unchecked |
| DEASSIGN, the same forms | the same | unchecked |
| CREATE/NAME_TABLE under LNM$SYSTEM_DIRECTORY | SYSPRV | unchecked |
| STOP, STOP/IDENTIFICATION, another process | GROUP or WORLD | unchecked: "as the operator with every privilege would" (`dclshell/dclcontroly.go`) |
| SHOW PROCESS/IDENTIFICATION, SHOW SYSTEM | GROUP or WORLD (unless SHOW is installed with WORLD: A.3) | unchecked |
| F$GETJPI, F$PID, F$CONTEXT | GROUP or WORLD | unchecked (DEVIATIONS.md: "No privilege is asked for to see other processes") |
| SET PROCESS/PRIVILEGES | SETPRV, or authorization | checked (Phase 93's subtask 5) |
| MOUNT, DISMOUNT, INITIALIZE/CONTAINER | govax-native; VMS's system-wide MOUNT needs SYSNAM | unchecked (Decision 10) |
| The machine's commands (DEPOSIT, ASM, INITIALIZE/MEMORY, LOAD, SAVE, ROM, SET QUANTUM, ...) | not VMS's | process 1 only (VAX_NOTSUBPROC); unchecked (question 4) |

LIBRTL's routines that change the system's state go through the
services' own code (`LIB$SET_LOGICAL` is `$CRELNM`'s code at supervisor
mode, `LIB$GETJPI` uses `mayAffect`), so they follow whatever the
services do. DUMP shows only files, so it needs no LOG_IO. RUN has no
`/DETACH` or `/UIC`.

**Tests.** `corevms/privilege_test.go`, `delprc_test.go`,
`getlki_test.go`, `disklogical_test.go`, `mbxprot_test.go`,
`operator_test.go`, `creprc_test.go`, and the console's
`privileges_test.go` turn privileges off and expect `SS$_NOPRIV`, each
in its own way. They are held to the manual, not to VMS.

### A.3 What the manuals leave open

These go to the probe (Part D).

1. **Inner modes.** For each service with a privilege, does an executive-
   or kernel-mode caller need it? For example, `$CRELNM` into
   LNM$SYSTEM_TABLE from kernel mode with SYSNAM off; `$CREMBX`
   permanent; `$CRMPSC` system global; `$SETPRI` of another process;
   `$WAKE` of another group's process.
2. **Statuses.** `SS$_NOPRIV` in every case, or something else (for
   example `SS$_NOSYSNAM`-style answers, a quiet success, or the
   privilege-less form done instead, as `$CREPRC` minimizes privileges)?
3. **Logical name tables.** Their default profiles (SHOW SECURITY of
   LNM$SYSTEM_TABLE, LNM$GROUP_000200, LNM$SYSTEM_DIRECTORY, and a table
   CREATE/NAME_TABLE makes), and whether a system-UIC process may write
   the system table with SYSNAM and SYSPRV off. Whether
   `$TRNLNM` of a table needs read access by its protection.
4. **DEFINE without the privilege.** DCL's messages for DEFINE/SYSTEM,
   DEFINE/GROUP, DEFINE/EXECUTIVE_MODE, DEASSIGN of them, and
   CREATE/NAME_TABLE in the system directory, without the privileges.
5. **Seeing other processes.** What SHOW SYSTEM, SHOW PROCESS/ID, F$PID,
   and F$GETJPI show or answer to a process without GROUP or WORLD (VMS's
   SHOW may run installed with privileges of its own).
6. **STOP.** STOP/IDENTIFICATION of a process in the group and outside
   it, without GROUP and WORLD: the message, and whether STOP/EXIT
   differs.
7. **`$CREPRC`.** The new process's AUTHPRIV, and its privileges when
   prvadr is omitted (both logged as unconfirmed in `creprc.go`).
8. **`$DLCEFC`**, **`$ULKPAG`**, **`$SNDOPR`'s SECURITY class**, and
   the mailbox's **IO$_SETPROTECTION**: the conditions govax guessed.
9. **Terminals.** Which terminal `$QIO` functions need LOG_IO or PHY_IO
   (permanent characteristics, IO$_SETCHAR versus IO$_SETMODE), and SET
   TERMINAL/PERMANENT's message without them.
10. **`$SETPRV`.** Its answers with every combination of enbflg, prmflg,
    and an unauthorized privilege, from user and from inner modes; and
    whether CURPRIV after image rundown is PROCPRIV exactly (a privilege
    enabled permanently by the image stays?).

### A.4 The probe's answers

(Filled in by subtask 2, after the author's run on the VAX.)

## Part B — decisions

**Decision 1 — one gate.** Every privilege check goes through one
function, `env.requirePrivilege(rule)` (and `env.holdsPrivilege(rule)`
for the services whose answer isn't a failure, such as `$SETPRI`'s quiet
lowering and `$CREPRC`'s minimizing). The gate:

- reads CURPRIV and the caller's access mode;
- applies the rule's inner-mode exemption, if it has one (Decision 3);
- returns 0 or the rule's status, `SS$_NOPRIV` unless the probe says
  otherwise;
- traces the check and its outcome under a new SET DEBUG class,
  PRIVILEGES, as SET DEBUG PROTECTION traces access checks.

`hasPrivilege` and `hasAnyPrivilege` become the gate's private
helpers. A test, `TestNoUncheckedPrivileges`, fails if any file outside
the gate reads `CurrentPrivileges`, as Phase 93's `TestNoUncheckedAccess`
does for file access. (`$SETPRV`, `$GETJPI`'s privilege items, image
rundown, and `$CREPRC`'s copying of masks are the named exceptions,
since they manage the masks rather than check them.)

**Decision 2 — the rules are a table.** `corevms/privrules.go` holds a
`PrivilegeRule` per guarded operation. Each has the operation's name
(`$CRELNM system table`, `DEFINE/SYSTEM`), the privileges (all of, or
any of), the inner-mode exemption, the status, and the source (the
manual's page, and later the probe's case). The checks name a rule
rather than a mask, so the table and the code are the same data. This
follows the project's table-driven dispatch. Two tests keep it whole:

- `TestEveryServiceHasPrivilegeRules`: every registered service is in a
  list that names its rules, or says it has none, so a new service must
  decide.
- `TestEveryPrivilegeRuleUsed`: every rule is used by some check.

SHOW PRIVILEGE_RULES is not added. The table is for the code, the tests,
and this document's A.1 table, which subtask 3 regenerates from it.

**Decision 3 — inner modes by the probe.** Each rule says which caller
modes it exempts: kernel and executive for `$CMKRNL`, `$CMEXEC`, `$ENQ`,
and `$SETPRV`'s authorization, which the manuals state; for the rest,
the probe's answers (A.3, 1). Until then a rule exempts no mode, which is
the current behavior. DCL's own checks are made at supervisor mode,
DCL's mode, so DCL gets no inner-mode exemption.

**Decision 4 — privilege or protection, one rule.** Where VMS lets
either grant an operation (logical name tables, mailboxes, sections,
event flag clusters), the check asks `internal/security`'s rule first
(with the process's credentials, which already include its privileges:
BYPASS, SYSPRV, GRPPRV, READALL), then the operation's own privilege
(SYSNAM, GRPNAM, ...). Logical name tables get profiles (owner,
protection), with VMS's defaults from the probe (A.3, 3), kept in
`lnm.Table` and shown by `$GET_SECURITY` and SHOW SECURITY, which
already reach tables (Phase 93's subtask 13b). This resolves the first
item of DEVIATIONS.md's "[Phase 26] Privilege simplifications", and its
second (`$DELLNM` checking the first table rather than the one the name
is found in) with it.

**Decision 5 — DCL checks where VMS would.** A DCL command that does a
privileged thing in Go calls the gate with the same rule as the service
VMS's DCL would call, at supervisor mode, and shows VMS's message for
the failure (A.3, 4 and 6). The commands are A.2's list: DEFINE, ASSIGN,
DEASSIGN, CREATE/NAME_TABLE, STOP, SHOW PROCESS, SHOW SYSTEM, and the
lexical functions that see other processes. STOP's "as the operator"
path goes away: STOP of another process is `$DELPRC`'s or `$FORCEX`'s
rule.

**Decision 6 — seeing other processes follows the probe.** If VMS 7.1's
SHOW SYSTEM shows every process to an unprivileged user (SHOW installed
with WORLD), govax's does too, and the rule says so (an exemption naming
the installed privilege). F$GETJPI, F$PID, and SHOW PROCESS/ID take
`$GETJPI`'s rule. The DEVIATIONS.md note ("No privilege is asked for to
see other processes") is then resolved.

**Decision 7 — the gaps in services.** `$SETSWM` is added (PSWAPM; it
records the setting, since govax doesn't swap). `$ULKPAG`, `$SNDOPR`'s
SECURITY class, `$DLCEFC`, and the mailbox's IO$_SETPROTECTION get the
probe's rules. The terminal driver's functions get LOG_IO and PHY_IO
where the probe shows them needed (A.3, 9). `$SETIME`, `$MOUNT`,
`$DISMOU`, and the rights and UAF services stay out (the review,
questions 3 and 5).

**Decision 8 — tests reach unprivileged processes four ways**, as Phase
93's did: SET PROCESS/PRIVILEGES and `$SETPRV` in process 1 (SYSTEM, so
still in the system UIC category); `$CREPRC` with a UIC and a privilege
mask; `$CREPRC` of LOGINOUT with a UIC, which runs a command procedure's
DCL as that UIC (Phase 93's way to replay the probe's account runs); and
processes built in Go. A helper makes a process authorized for every
privilege but holding only TMPMBX and NETMBX, the shape of the probe's
account (Part D), so a test can enable one privilege at a time.

**Decision 9 — installed images, in this phase** (the review, question
1): INSTALL with ADD, REMOVE, and LIST, and `/PRIVILEGED` and
`/OPEN` (other qualifiers accepted and recorded); the known file list in
the System; the image activator setting IMAGPRIV and CURPRIV = PROCPRIV
| IMAGPRIV for an image on the list, with rundown already dropping it;
JPI$_IMAGPRIV answering. INSTALL itself needs CMKRNL. This resolves "No
installed images: IMAGPRIV is always empty".

**Decision 10 — govax's own commands** (the review, question 4). MOUNT
and DISMOUNT, which make a volume known to every process, take VMS's
MOUNT/SYSTEM rule, SYSNAM; INITIALIZE/CONTAINER of a container that
already exists takes VOLPRO, as INITIALIZE of a volume another owns does
on VMS; making a new container needs nothing. They go through the gate
(Decision 1) with rules of their own in the table, and are done in
subtask 7 with DCL's other commands. The machine's commands (DEPOSIT,
ASM, INITIALIZE/MEMORY, LOAD, SAVE, ROM, SET QUANTUM, ...) stay the
operator's: process 1 only, as now, with no privilege check.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey** (done 2026-11-15): Part A, this plan.
2. **The probe**: `testdata/priv94`, as in Part D, written and rehearsed
   on govax; run on the VAX by the author; A.4 filled in and Part B
   adjusted to its answers. Later rounds as later subtasks find gaps.
3. **The gate and the table** (Decisions 1 and 2): every existing check
   moved onto rules, with no change in behavior; SET DEBUG PRIVILEGES;
   `TestNoUncheckedPrivileges`, `TestEveryServiceHasPrivilegeRules`,
   `TestEveryPrivilegeRuleUsed`; A.1's table regenerated from the rules.
4. **The services held to the probe** (Decisions 3 and 7): each rule's
   inner-mode exemption and status from A.4; `$SETSWM`; `$ULKPAG`,
   `$DLCEFC`, `$SNDOPR`'s SECURITY class, and IO$_SETPROTECTION settled;
   `$CREPRC`'s AUTHPRIV and omitted prvadr; `$SETPRV`'s fine points;
   held to the probe's program runs by an oracle test.
5. **Logical name tables' protection** (Decision 4): table profiles and
   their defaults, the protection-or-privilege check in `$CRELNM`,
   `$DELLNM`, `$CRELNT`, and `$TRNLNM`; SHOW SECURITY and
   `$GET_SECURITY` of tables matching the probe; DEVIATIONS.md's Phase
   26 entry's first two items resolved.
6. **The terminal driver** (Decision 7): its functions' privileges, SET
   TERMINAL/PERMANENT's message, held to the probe.
7. **DCL's commands** (Decisions 5 and 10): DEFINE, ASSIGN, DEASSIGN,
   CREATE/NAME_TABLE, STOP, and govax's MOUNT, DISMOUNT, and
   INITIALIZE/CONTAINER through the gate, with VMS's messages,
   held to the probe's DCL logs under each privilege set.
8. **Seeing other processes** (Decision 6): F$GETJPI, F$PID, F$CONTEXT,
   SHOW PROCESS/IDENTIFICATION, and SHOW SYSTEM by the probe's answers.
9. **Installed images** (Decision 9), with a probe round of its own if subtask 2's doesn't cover them.
10. **The close-out**: MANUAL.md (a section on privileges: the masks,
    what each privilege guards in govax, SET DEBUG PRIVILEGES, INSTALL if
    it's in), HELP for every new command, qualifier, and debug class,
    `VAX-VMS-INTERNALS.md`'s processes section, DEVIATIONS.md, PLAN.md,
    and CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/priv94/` (its README to have the files and the steps for the
VAX run), on an exchange volume mounted `/SYSTEM` and named by a system
logical name, `PRIV94$DISK`, as Phase 93's was. Nothing it does outlives
the run but its logs: every name, table, mailbox, section, and process it
makes is removed at the end.

**The identities.** SYSTEM, `[1,4]`, with privileges turned off by
`$SETPRV` a case at a time; Phase 93's PROT93A `[200,1]` and PROT93C
`[300,1]`, with their default privileges, as targets and as plain users;
and one new account (the review, question 2, accepted), PRIV94 `[200,3]`,
authorized for every privilege but with only TMPMBX and NETMBX as
defaults, so a case can enable exactly the privileges it means to test
from a non-system UIC.

**`priv94.mar`**, one program taking its cases from SYS$INPUT, as
`prot93.mar` does. Each case names a service and its arguments, the
privileges to enable for it (all others off), and the mode to call it
from: user, or executive and kernel through `$CMEXEC` and `$CMKRNL`
(CMEXEC and CMKRNL are enabled to get there, then turned off before the
call, and the program reports both). It prints the status, and what the
service did (the name's table and mode, the process's new priority,
whether the target woke). The targets are a holder subprocess in the
same group (SPAWN/NOWAIT of a program that hibernates) and a detached
process in another group (RUN/DETACH/UIC from SYSTEM's part), each
reporting what reached it through a mailbox.

The cases cover every row of A.1's table that govax implements, each
with the privilege on, off, and (where A.3, 1 asks) from each inner
mode; `$SETPRV`'s combinations (A.3, 10); `$CREPRC`'s privileges (A.3,
7); and the terminal functions (A.3, 9).

**`priv94.com`**, the DCL side, run once per identity: SHOW
PROCESS/PRIVILEGES first, then DEFINE, ASSIGN, and DEASSIGN in each
table and mode, CREATE/NAME_TABLE, SHOW SECURITY of the logical name
tables (A.3, 3), STOP and STOP/IDENTIFICATION of the holders, SHOW
SYSTEM, SHOW PROCESS/IDENTIFICATION, F$GETJPI, F$PID's walk, and SET
TERMINAL/PERMANENT, each under SET PROCESS/PRIVILEGES's privilege sets.

**`priv94i.mar`** and **`priv94i.com`** (installed images, Decision 9): an image that reports its CURPRIV and IMAGPRIV and calls a
guarded service, run plain, then after INSTALL ADD/PRIVILEGED, then after
REMOVE.

## The review

Questions for the author, answered 2026-11-15: the plan was accepted,
and with it the recommendation (or the plan's default) on each question.

1. **Installed images** (Decision 9): in this phase, or with the
   identities phase? *Recommendation:* in this phase, small. It is the one
   way an unprivileged process's CURPRIV gains privileges, the masks
   already model IMAGPRIV, and it resolves a Phase 26 deviation.
2. **The probe's account**: may PRIV94 `[200,3]` be made on the simh VAX,
   authorized for every privilege with TMPMBX and NETMBX as defaults?
3. **`$SETIME` and SET TIME**: out, as A.1 lists? govax's clock is the
   host's or counts instructions, so setting it means a new offset.
   *Recommendation:* out, logged for later.
4. **govax's own commands**: should MOUNT, DISMOUNT, and
   INITIALIZE/CONTAINER (govax-native, making volumes known
   system-wide) need a privilege, say SYSNAM as VMS's MOUNT/SYSTEM does?
   And the machine's commands (DEPOSIT, ASM, INITIALIZE/MEMORY, LOAD,
   SAVE, ROM, SET QUANTUM)? *Recommendation:* MOUNT and DISMOUNT take
   MOUNT/SYSTEM's rule (SYSNAM) and INITIALIZE/CONTAINER VOLPRO when the
   container exists; the machine's commands stay the operator's, process
   1 only, with no privilege check.
5. **`$MOUNT`, `$DISMOU`, the rights and UAF services**: out of scope,
   for the identities phase?

The answers:

1. Installed images are in this phase (Decision 9, subtask 9).
2. The PRIV94 account may be made on the VAX for the probe.
3. `$SETIME` and SET TIME are out, logged for later.
4. MOUNT and DISMOUNT need SYSNAM, INITIALIZE/CONTAINER of an existing
   container VOLPRO; the machine's commands are unchecked (Decision 10).
5. `$MOUNT`, `$DISMOU`, and the rights and UAF services wait for the
   identities phase.

## Progress log

- **2026-11-15** Subtask 1: read the 5.0 System Services manual (as
  text, each privilege mention attributed to the service whose page it
  is on), VSI's "Required Privileges" sections, and DEVIATIONS.md's
  Phase 26 entries; listed the 100 services govax registers and every
  check they make (`hasPrivilege`, `hasAnyPrivilege`, `mayAffect`,
  `uicAccess`); found the DCL commands that act in Go without a check
  (DEFINE and DEASSIGN of system, group, and executive-mode names,
  CREATE/NAME_TABLE, STOP, SHOW PROCESS and SHOW SYSTEM, the process
  lexical functions); wrote Part A and this plan.
- **2026-11-15** The review: the author accepted the plan and every
  recommendation. Decision 9 (installed images) is in, Decision 10
  (govax's MOUNT, DISMOUNT, and INITIALIZE/CONTAINER) added, subtasks 7
  and 9 and Part D adjusted to them.
