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

Round 1 ran on VMS V7.1 on 2026-11-15 (`testdata/priv94/vax/`: SYSTEM's
log, PRIV94's, PRIV94N's, and the end's). No case crashed VMS. The
numbers below are `priv94.mar`'s cases. "SYSTEM", "PRIV94", and
"PRIV94N" are the three identities (Part D); SYSTEM's UIC `[1,4]` is in
the system category, PRIV94's `[200,3]` is not.

**The masks.** A mask is all 64 bits: AUTHORIZE's `/PRIVILEGES=ALL` gives
AUTHPRIV `FFFFFFFFFFFFFFFF`, and `$SETPRV` of an all-ones mask (or
`$CREPRC` with SETPRV) sets every bit, not only the 39 privileges'. govax
keeps 39 (`allPrivileges`, `7FFFFFFFFF`).

**Statuses.** Almost every refusal is `SS$_NOPRIV`. The exceptions:
`$ENQ`'s system-wide resource is `SS$_NOSYSLCK`; `$BRKTHRU` is
`SS$_NOOPER`; INSTALL ADD is `%SYSTEM-F-NOCMKRNL`; `$SETPRV` of an
unauthorized privilege is `SS$_NOTALLPRIV` (the authorized ones are
done). Some services do less instead of failing (below: `$SETPRI`,
`$CREPRC`'s base priority, `$CRELNM`'s and DEFINE's modes).

**Inner modes (A.3, 1).** No service exempts an executive- or
kernel-mode caller from its privilege, but two: `$ENQ` of a system-wide
resource (SYSLCK) from either inner mode (126, 127), and `$DELMBX`
(PRMMBX) from kernel mode only, not executive (90, 91). Everything else
the probe tried from an inner mode was refused as from user mode: the
logical name services, `$CREMBX`, `$ASCEFC`, `$CRMPSC`, the process
services (`$WAKE`, `$SETPRI`, `$SUSPND`, `$DELPRC`, `$CREPRC`),
`$BRKTHRU`, `$LCKPAG`, `$SETSWM`, disk logical I/O, and terminal
IO$_SETCHAR. (`$SETPRV` from an inner mode enabling an unauthorized
privilege couldn't be tried: PRIV94N can't reach an inner mode.)

**Logical names (A.3, 3; Decision 4).** Writing a shareable table is
the table's protection or a privilege, and the system category gets in
by protection: SYSTEM, with no privileges at all, makes and deletes
names in LNM$SYSTEM_TABLE, its group table, and a table closed to the
group and world (4, 8, 16, 38, 43), and makes tables in
LNM$SYSTEM_DIRECTORY (52); PRIV94 can't (NOPRIV). The privileges that
override the protection:

| Table | Write | Read |
| --- | --- | --- |
| LNM$SYSTEM_TABLE | SYSNAM or SYSPRV (5, 7) | everyone (its protection) |
| a group table | GRPNAM or SYSPRV; not SYSNAM (9 to 11) | |
| another shareable table | SYSPRV only; not SYSNAM or GRPNAM (16 to 19) | SYSPRV, READALL, or BYPASS; not SYSNAM (61 to 65) |
| LNM$SYSTEM_DIRECTORY (`$CRELNT`) | SYSPRV; not SYSNAM (52 to 55) | |

`$TRNLNM` of a table its protection closes is NOPRIV (61). A shareable
table's name goes in LNM$SYSTEM_DIRECTORY whatever its parent, so making
one needs SYSPRV (or the system category) under any parent, even one
whose protection lets the caller write it (round 2's 18 to 29: SYSNAM
and GRPNAM don't help); its quota comes out of the parent's (EXLNMQUOTA
when it asks for the parent's whole quota, round 1's 59 and 60, round
2's 22). The tables' profiles, from SHOW
SECURITY: LNM$SYSTEM_TABLE and LNM$SYSTEM_DIRECTORY owner `[SYSTEM]`,
`(S:RWC,O:RWC,G:R,W:R)`; LNM$GROUP_000001 owner `[1,*]` and
LNM$GROUP_000200 owner `[PROT93,*]` (the group's identifier),
`(S:RWCD,O:R,G:R,W)`; a table `$CRELNT` makes with a promsk takes it as
its protection (0 is `(S:RWCD,O:RWCD,G:RWCD,W:RWCD)`), owner the
creator. SHOW SECURITY of a table needs read access to it; SECURITY
doesn't help.

A name's mode: a mode more privileged than the caller's is kept only
with SYSNAM, and otherwise lowered to the caller's without complaint
(22, 23, 24: SYSTEM's exec-mode name in the system table went in at user
mode); from an inner mode, the caller's mode is allowed (30). DEFINE
/EXECUTIVE_MODE without SYSNAM makes a supervisor-mode name, silently.
`$DELLNM` with a name (round 2's 30 to 40): if the name exists at the
mode given (maximized; the caller's mode when acmode is omitted), it and
the same name at every outer mode are deleted; if not, nothing is
(NOLOGNAM), even when the name exists at an outer mode. So a user-mode
caller without SYSNAM gets NOLOGNAM for an exec-mode name (48), and round
1's sweep, at kernel mode, deleted no user-mode name (`PRIV94_L21`,
`L49`, and `L51`, which round 2's end removed). `$TRNLNM` with an acmode
looks only at that mode and inner ones, and of those finds the
outermost. `$CRELOG`
into the system table with SYSNAM makes an executive-mode name, user
mode without it (68, 69, 75).

DCL looks a `/TABLE=` table up from supervisor mode and doesn't see a
table made at user mode: DEFINE/TABLE=PRIV94_OT is NOLOGTAB for everyone
(the fixtures were made at user mode). On tables made at supervisor mode
(round 2's PRIV94E.COM) DCL follows the tables' rule: PRIV94 writes an
open table with no privilege and a closed one only with SYSPRV, and reads
the closed one with SYSPRV or READALL; without them SHOW LOGICAL/TABLE=
says `%SYSTEM-W-IVLOGTAB, invalid logical name table`, and SYSNAM and
GRPNAM do nothing for either. Otherwise DCL's messages are the service's: DEFINE,
ASSIGN, DEASSIGN, CREATE/NAME_TABLE, and F$TRNLNM say
`%SYSTEM-F-NOPRIV, insufficient privilege or object protection
violation` alone.

**Mailboxes.** `$CREMBX`: a temporary mailbox needs TMPMBX (80, 81), a
permanent one PRMMBX (83, 84); TMPMBX doesn't give a permanent one. A
permanent mailbox with a logical name also needs to put the name in the
system table (LNM$PERMANENT_MAILBOX is LNM$SYSTEM_TABLE): SYSNAM or
SYSPRV, or the system category; TMPMBX doesn't help, and without a name
PRMMBX alone does (84; round 2's 3 to 8). A temporary mailbox's name goes in the job table
(LNM$JOB_...), a permanent one's in LNM$SYSTEM_TABLE. `$DELMBX` needs
PRMMBX, even for the caller's own mailbox (88, 89). IO$_SETPROTECTION
(IO$_SETMODE!IO$M_SETPROT) is the owner's (SYSTEM's own mailbox, and any
process's temporary one), or BYPASS or SYSPRV; not GRPPRV or OPER (92 to
97), as govax has it.

**Event flag clusters, sections, locks.** A permanent common event flag
cluster needs PRMCEB (99, 100); `$DLCEFC` of one's own needs nothing
(103). A system global section needs SYSGBL, a permanent one PRMGBL, a
permanent system one both (106 to 113); `$DGBLSC` of a permanent section
needs PRMGBL, of a permanent system one both (117 to 122).
`$ENQ` of a system-wide resource needs SYSLCK from user mode,
SS$_NOSYSLCK (124).

**Other processes (A.3, 5 and 6).** `$WAKE`, `$SCHDWK`, `$CANWAK`,
`$SUSPND`, `$RESUME`, `$SETPRI`, `$GETJPI`, `$FORCEX`, and `$DELPRC` of
another process: GROUP for one in the caller's own group, WORLD for any;
the system category gets nothing from its UIC (SYSTEM with GROUP can't
touch either holder: group 1 isn't theirs). A wildcard `$GETJPI` returns
NOPRIV for each process it may not see, and goes on (190 to 192).
`$SETPRI` of another process above the caller's own base priority is
done at the caller's base, without complaint, unless ALTPRI (149, 150,
179); its own priority above its base, the same (186, 187). From DCL:
SHOW SYSTEM lists every process to everyone (it is installed with
WORLD); SHOW PROCESS/IDENTIFICATION, F$GETJPI, F$PID (which skips what
it may not see), and SET PROCESS/PRIORITY, /SUSPEND, /RESUME
/IDENTIFICATION follow the services' rule; SET PROCESS says `%SET-E-NOTSET,
error modifying 00000130` and `-SYSTEM-F-NOPRIV`; STOP/IDENTIFICATION
says `%SYSTEM-F-NOPRIV` alone, and STOP by name finds only the caller's
group (NONEXPR for SYSTEM).

**`$CREPRC` (A.3, 7).** The new process's AUTHPRIV, PROCPRIV, and
CURPRIV are all the privileges it is given: with no prvadr, the
creator's CURPRIV (203 to 206); with a prvadr, that mask, cut to the
creator's CURPRIV unless it holds SETPRV (207 to 211). A UIC other than
the creator's needs DETACH or CMKRNL (212 to 217); the creator's own UIC
or PRC$M_DETACH alone needs nothing (218, 219). PRC$M_PSWAPM needs
PSWAPM and PRC$M_NOACNT ACNT (NOPRIV, 220 to 223). A base priority
above the creator's is lowered to it, without complaint, unless ALTPRI
(224, 225). RUN/DETACH/UIC without DETACH says `%RUN-F-CREPRC, process
creation failed` and `-SYSTEM-F-NOPRIV`.

**`$SETPRV` (A.3, 10).** Disabling always works. Enabling a privilege
not in AUTHPRIV without SETPRV in CURPRIV enables the others and
returns NOTALLPRIV (PRIV94N, 229 to 233). prvprv is the previous
CURPRIV, or the previous PROCPRIV when prmflg is set (233); a permanent
enable sets both masks. With no prvadr it changes nothing and returns
prvprv (237, 238). prmflg 2 is SS$_IVSTSFLG (240); enbflg 2 changed
nothing (239). After image rundown CURPRIV is PROCPRIV exactly: a
permanent enable stays, an image's is gone (part R and the procedure's
SHOW PROCESS/PRIVILEGES).

**Installed images (Decision 9).** While PRIV94I, installed
`/PRIVILEGED=(SYSNAM,OPER)`, runs, IMAGPRIV is those two and CURPRIV is
PROCPRIV plus them, for any user, and the image's SYSNAM works; rundown
takes them away. INSTALL LIST needs no privilege; INSTALL ADD without
CMKRNL is `%SYSTEM-F-NOCMKRNL, operation requires CMKRNL privilege`.
INSTALL LIST/FULL's layout and INSTALL REMOVE's and LIST's messages
(`%INSTALL-W-FAIL`, `-INSTALL-E-NOKFEFND`) are in the logs.

**Operators, pages, I/O, modes (A.3, 8 and 9).** `$SNDOPR` of a
request needs no privilege, to any class, SECURITY too (247 to 252).
`$BRKTHRU` to every terminal or every user needs OPER (NOOPER), to
another user's terminals OPER as well, not WORLD (257 to 259), to one's
own user or terminal nothing. `$LCKPAG` needs PSWAPM; `$ULKPAG` needs nothing.
(`$ULKPAG` of a locked page unlocked it and returned `%X0000000C`,
SS$_ACCVIO's value, whenever a case's body called it, with any
privileges, with or without retadr, which it filled in; from a case's
undo it returned SS$_WASSET, and of a page not locked SS$_NORMAL: round
1's 265 and 266, round 2's 11 to 17. Not understood, and not a matter of
privilege; govax keeps WASSET and WASCLR.) `$LKWSET` needs nothing; `$SETSWM` needs PSWAPM (268, 269). Disk logical I/O needs LOG_IO
or PHY_IO, physical PHY_IO, as govax has it. A terminal's IO$_SETMODE
needs nothing, IO$_SETCHAR LOG_IO or PHY_IO (278 to 283); SET
TERMINAL/PERMANENT without them says `%SET-W-NOTSET, error modifying
OPA0:` and `-SYSTEM-F-NOPRIV`. `$CMKRNL` needs CMKRNL, `$CMEXEC` CMEXEC
or CMKRNL (284 to 289): govax checks neither.

**The probe's own mistakes.** DCL's PRIORITY sections wrote
`'P+2'`, which substitutes only P (`%DCL-W-NOCCAT`), so they tested
nothing; the program's `$SETPRI` cases cover it. The end's sweep asked
for kernel mode and so deleted nothing (above). PRIV94N couldn't enable
the privileges of most of part P's and X's sets (`setprv 00000681`,
NOTALLPRIV), as intended. `$TRNLNM`'s value line shows the last
translation when the call failed.

**Round 2** (2026-11-15, `testdata/priv94/vax/round2/`, SYSTEM and
PRIV94) settled what round 1 left open: the permanent mailbox's name, the
shareable table's directory, `$DELLNM`'s modes, and DCL on
supervisor-mode tables, above; `$ULKPAG`'s status stays unexplained. It
also gave SET PROCESS/PRIORITY's own message above the base without
ALTPRI, `%SET-E-NOTSET, need ALTPRI privilege to elevate above base
priority` (DCL checks before calling `$SETPRI`, which would lower the
priority quietly). Its undo of `$CRELNT` deleted the table in its parent
rather than the directory, so it left PRIV94_TB7 to TB12 in the VAX's
LNM$SYSTEM_DIRECTORY, and PRIV94's SYSPRV cases found SYSTEM's and got
SS$_SUPERSEDE.

**govax against it.** The rehearsal's SYSTEM run (`TestPriv94Probe`)
differs from SYSTEM's VMS run in: every table write SYSTEM makes by
protection (govax refuses it); the group table and `$CRELNT` rules;
`$CRELNM`'s mode lowering (govax refuses); `$CREMBX` (govax lets TMPMBX
make a permanent mailbox, and an inner mode create without privileges);
`$DELMBX` from executive mode; `$CREPRC`'s AUTHPRIV (govax gives the
creator's) and prvadr cut; the 64-bit masks; `$BRKTHRU` to a user;
IO$_SETCHAR; `$CMKRNL` and `$CMEXEC`; `$SETSWM`. Subtasks 3 to 9 make
them VMS's.

### A.5 The rules (subtasks 3 and 4)

The gate's rules as `corevms/privrules.go` holds them, printed by `go test
-run TestPrivilegeRuleTable -v ./internal/corevms`: the table A.1's
"govax today" column describes, now one place in the code. Each row is a
rule a check names; `serviceRules` says which services can ask about it.
"SSRM" is the 5.0 System Services Reference Manual's entry, "probe n"
`priv94.mar`'s case (round 2's are `priv94b.mar`'s). Since subtasks 4 and
5 the rows are A.4's answers, which `TestPriv94Oracle` holds govax to.
Since subtask 5 a logical name table's protection is asked first
(Decision 4), so its rows are only the privileges that override it:
SYSPRV's part, and the system category's, are the protection's, and
`$CRELNT` of a shareable table and a permanent mailbox's name are write
access to a table, with no rule of their own.

| Operation | Needs | Exempt modes | Refusal | Source |
| --- | --- | --- | --- | --- |
| logical name: write the system table past its protection | SYSNAM | none | SS$_NOPRIV | SSRM $CRELNM; probe 4 to 7 |
| logical name: write the group table past its protection | GRPNAM | none | SS$_NOPRIV | SSRM $CRELNM; probe 8 to 11 |
| logical name: a mode inner to the caller's | SYSNAM | none | SS$_NOPRIV | SSRM $CRELNM (the mode is maximized); probe 22 to 24 |
| $CREMBX temporary | TMPMBX | none | SS$_NOPRIV | SSRM $CREMBX; probe 80, 81 |
| $CREMBX permanent | PRMMBX | none | SS$_NOPRIV | SSRM $CREMBX; probe 83, 84 |
| $DELMBX | PRMMBX | kernel | SS$_NOPRIV | SSRM $DELMBX; probe 88 to 91 |
| mailbox IO$_SETPROTECTION, not the owner | BYPASS or SYSPRV | none | SS$_NOPRIV | probe 92 to 97 |
| $ASCEFC permanent | PRMCEB | none | SS$_NOPRIV | SSRM $ASCEFC; probe 99, 100 |
| $DLCEFC another creator's cluster | PRMCEB | none | SS$_NOPRIV | SSRM $DLCEFC; probe 103 to 105 |
| $CRMPSC system section | SYSGBL | none | SS$_NOPRIV | SSRM $CRMPSC; probe 106 to 113 |
| $CRMPSC permanent section | PRMGBL | none | SS$_NOPRIV | SSRM $CRMPSC; probe 106 to 113 |
| $DGBLSC system section | SYSGBL | none | SS$_NOPRIV | SSRM $DGBLSC; probe 117 to 122 |
| $DGBLSC permanent section | PRMGBL | none | SS$_NOPRIV | SSRM $DGBLSC; probe 117 to 122 |
| $ENQ system-wide resource | SYSLCK | kernel, executive | SS$_NOSYSLCK | SSRM $ENQ; probe 124 to 127 |
| $GETLKI system-wide lock | SYSLCK | kernel, executive | SS$_NOSYSLCK | SSRM $GETLKI |
| $GETLKI another group's lock | WORLD | none | SS$_NOWORLD | SSRM $GETLKI |
| another process in the group | GROUP or WORLD | none | SS$_NOPRIV | SSRM $DELPRC and the rest; probe 128 to 202 |
| a process in another group | WORLD | none | SS$_NOPRIV | SSRM $DELPRC and the rest; probe 128 to 202 |
| $SETPRI above the authorized priority | ALTPRI | none | SS$_NOPRIV | SSRM $SETPRI; probe 149, 150, 179, 186, 187 |
| $CREPRC detached with another UIC | DETACH or CMKRNL | none | SS$_NOPRIV | SSRM $CREPRC; probe 212 to 219, 226, 227 |
| $CREPRC batch or network process | DETACH or CMKRNL | none | SS$_NOPRIV | SSRM $CREPRC |
| $CREPRC network process | NETMBX | none | SS$_NOPRIV | SSRM $CREPRC |
| $CREPRC PRC$M_PSWAPM | PSWAPM | none | SS$_NOPRIV | SSRM $CREPRC; probe 220, 221 |
| $CREPRC PRC$M_NOACNT | ACNT | none | SS$_NOPRIV | SSRM $CREPRC; probe 222, 223 |
| $CREPRC detached quotas of its own | DETACH | none | SS$_NOPRIV | SSRM $CREPRC |
| $CREPRC privileges beyond the creator's | SETPRV | none | SS$_NOPRIV | SSRM $CREPRC; probe 207 to 211 |
| $CREPRC base priority above the creator's | ALTPRI | none | SS$_NOPRIV | SSRM $CREPRC; probe 224, 225 |
| $SETPRV a privilege not authorized | SETPRV | kernel, executive | SS$_NOTALLPRIV | SSRM $SETPRV; probe 229 to 241 |
| $CMKRNL | CMKRNL | kernel, executive | SS$_NOPRIV | SSRM $CMKRNL; probe 284 to 289 |
| $CMEXEC | CMEXEC or CMKRNL | kernel, executive | SS$_NOPRIV | SSRM $CMEXEC; probe 284 to 289 |
| $LCKPAG | PSWAPM | none | SS$_NOPRIV | SSRM $LCKPAG; probe 262 to 264 |
| $SETSWM | PSWAPM | none | SS$_NOPRIV | SSRM $SETSWM; probe 268 to 270 |
| $SNDOPR an operator's function | OPER | none | SS$_NOPRIV | SSRM $SNDOPR; probe 247 to 252 |
| $BRKTHRU to another user | OPER | none | SS$_NOOPER | probe 257 to 260 |
| $BRKTHRU to every terminal or user | OPER | none | SS$_NOOPER | SSRM $BRKTHRU; probe 253 to 256, 261 |
| terminal IO$_SETCHAR | LOG_IO or PHY_IO | none | SS$_NOPRIV | I/O User's Reference; probe 278 to 283 |
| disk logical block I/O | LOG_IO or PHY_IO | none | SS$_NOPRIV | I/O User's Reference; probe 271 to 273, 277 |
| disk physical block I/O | PHY_IO | none | SS$_NOPRIV | I/O User's Reference; probe 274 to 276 |

A.1 stays as the survey found things; this table replaces its last
column.

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
modes it exempts. By the probe (A.4), only two do: `$ENQ`'s SYSLCK
(executive and kernel) and `$DELMBX`'s PRMMBX (kernel only); with
`$CMKRNL`'s and `$CMEXEC`'s own, which an inner-mode caller doesn't
need, and `$SETPRV`'s authorization, which the manual gives to inner
modes (the probe couldn't try it). Every other rule exempts no mode.
DCL's own checks are made at supervisor mode, DCL's mode, so DCL gets no
inner-mode exemption. (Settled 2026-11-15.)

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

**Decision 6 — seeing other processes follows the probe.** VMS 7.1's
SHOW SYSTEM shows every process to everyone (SHOW is installed with
WORLD), so govax's does too, and the rule says so (an exemption naming
the installed privilege). The system category gets nothing over other
groups' processes from its UIC (A.4). F$GETJPI, F$PID, and SHOW PROCESS/ID take
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
2. **The probe** (done 2026-11-15): `testdata/priv94`, as in Part D, written and rehearsed
   on govax; run on the VAX by the author; A.4 filled in and Part B
   adjusted to its answers. Later rounds as later subtasks find gaps.
3. **The gate and the table** (done 2026-11-15; Decisions 1 and 2): every existing check
   moved onto rules, with no change in behavior; SET DEBUG PRIVILEGES;
   `TestNoUncheckedPrivileges`, `TestEveryServiceHasPrivilegeRules`,
   `TestEveryPrivilegeRuleUsed`; A.1's table regenerated from the rules.
4. **The services held to the probe** (done 2026-11-15; Decisions 3 and 7): each rule's
   inner-mode exemption and status from A.4; `$SETSWM`; `$ULKPAG`,
   `$DLCEFC`, `$SNDOPR`'s SECURITY class, and IO$_SETPROTECTION settled;
   `$CREPRC`'s AUTHPRIV and omitted prvadr; `$SETPRV`'s fine points;
   held to the probe's program runs by an oracle test.
5. **Logical name tables' protection** (done 2026-11-15; Decision 4): table profiles and
   their defaults, the protection-or-privilege check in `$CRELNM`,
   `$DELLNM`, `$CRELNT`, and `$TRNLNM`; SHOW SECURITY and
   `$GET_SECURITY` of tables matching the probe; DEVIATIONS.md's Phase
   26 entry's first two items resolved.
6. **The terminal driver** (done 2026-11-15; Decision 7): its functions' privileges, SET
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

`testdata/priv94/` (its README has the files and the steps for the VAX
run), on an exchange volume mounted `/SYSTEM` and named by a system
logical name, `PRIV94$DISK`, as Phase 93's was. Nothing it does outlives
the run but its logs: every name, table, mailbox, cluster, section, and
process it makes is removed, by the case that made it or at the end.
`gen.go` writes every file.

**The identities.** SYSTEM, `[1,4]`; PRIV94, `[200,3]`, authorized for
every privilege but holding only TMPMBX and NETMBX (the review, question
2); and PRIV94N, `[200,4]`, authorized for only those two, the plain
user `$SETPRV` and DCL refuse. (The plan named Phase 93's PROT93A and
PROT93C as plain users; their accounts were removed at Phase 93's end,
and a plain account in PRIV94's own group serves better.) Each case's
privileges are set with `$SETPRV`, so SYSTEM and PRIV94 run the same
cases from a system UIC and from a group UIC.

**The holders.** Two detached processes the program makes with `$CREPRC`
(part H), running `priv94h.mar`, which hibernates: PRIV94_G in group 200
(`[200,1]`, PRIV94's group) and PRIV94_W in group 300 (`[300,1]`). Their
PIDs are system logical names. No account is needed for their UICs.

**`priv94.mar`**: 289 cases, each a routine under a handler, in parts by
`PRIV94_PART`: logical names ($CRELNM, $DELLNM, $CRELNT, $TRNLNM, $CRELOG,
$DELLOG, into the system, group, job, and process tables and two fixture
tables, one open and one closed to the group and world), mailboxes
($CREMBX, $DELMBX, IO$_SETPROTECTION on SYSTEM's fixture mailbox), common
event flag clusters, global sections (SYSGBL, PRMGBL), locks (SYSLCK),
other processes ($WAKE to $GETJPI on each holder, and a wildcard
$GETJPI's view), $FORCEX and $DELPRC (part K, which ends the holders),
$CREPRC (privileges, AUTHPRIV, DETACH, PSWAPM, NOACNT, ALTPRI), $SETPRV
(every combination A.3, 10 asks), rundown (part R), $SNDOPR and
$BRKTHRU, $LCKPAG, $ULKPAG, $LKWSET, $SETSWM, disk logical and physical
I/O, terminal SETMODE and SETCHAR, and $CMKRNL and $CMEXEC. A case's body
runs with exactly its privileges, from user mode or from executive or
kernel mode (CMEXEC or CMKRNL enabled to get there, then turned off), and
everything around it with every privilege the process is authorized for,
so it can show what the body did and undo it.

**`priv94d.com`**, DCL's side, run by each identity: DEFINE, ASSIGN, and
DEASSIGN in each table and mode, CREATE/NAME_TABLE, SHOW SECURITY of the
tables (A.3, 3), SHOW SYSTEM, SHOW PROCESS/IDENTIFICATION, F$GETJPI and
F$PID of the holders, SET PROCESS/PRIORITY, /SUSPEND, and /RESUME of
them, SET TERMINAL/PERMANENT, RUN/DETACH, and STOP, each under one
privilege set after another.

**`priv94i.mar`** (installed images, Decision 9): shows its masks and
puts a name in the system table. SYSTEM runs it plain, installs it
`/PRIVILEGED=(SYSNAM,OPER)`, and runs it with and without its own
privileges; each account runs it, and tries INSTALL without CMKRNL; the
end removes it and runs it plain again.

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
- **2026-11-15** Subtask 2, the probe, written: `testdata/priv94`
  (`gen.go` writes the program, the holder, the installed image, the
  procedures, and the exchange scripts; README.md has the VAX steps), a
  second account, PRIV94N, in place of Phase 93's removed ones (Part D).
  Rehearsed on govax: `TestPriv94Probe` assembles and links the programs
  and runs every part; the procedures ran to their end as SYSTEM. What
  the rehearsal found in govax, for the later subtasks:
  - A service govax lacks called from kernel mode (`$SETSWM`, case 270)
    signals there, and the condition reaches the program's user-mode
    handler, whose unwind leaves the kernel stack 4 bytes off: every
    later `$CMKRNL` in the process faults (ACCVIO near `7FBFFFFC`, lower
    each time). VMS searches only the signaling mode's frames. Subtask 4
    adds `$SETSWM`; the condition search across modes is to be fixed
    with it.
  - `$GETMSG` of `SS$_WASSET` (9, `$ULKPAG`'s answer) gives ACCVIO's
    text: the message is found by its number alone, and 9 and 12 share
    one. To be settled by VMS's log.
  - DCL lacks INSTALL, AUTHORIZE (no image), RUN/DETACH, SET VOLUME, SET
    TERMINAL, SET PROCESS/PRIORITY, /SUSPEND, and /RESUME of another
    process, and SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE; and with no
    privileges, F$PID and SHOW SYSTEM show every process (A.3, 5).
  Waiting for the author's VAX run.
- **2026-11-15** Subtask 2: the author ran round 1 on VMS V7.1, with no
  crash; its logs and images are in `testdata/priv94/vax/`, and A.4 is
  written from them, with Decisions 3 and 6 settled. Besides what A.4's
  last paragraph lists, the probe found the masks are 64 bits wide on
  VMS. It left three user-mode names, PRIV94_L21, L49, and L51, in the
  VAX's system table (the sweep's mode), and group 200's table is owned
  by Phase 93's identifier PROT93, which is still there; AUTHORIZE made
  no PRIV94G.
- **2026-11-15** Subtask 2, round 2 written: `testdata/priv94`'s
  `priv94b.mar` (40 cases: permanent mailboxes and their names, `$ULKPAG`,
  `$CRELNT` under shareable tables, `$DELLNM` across modes) and its
  procedures (`priv94r.com`, `priv94v.com`, `priv94e.com`, `priv94y.com`),
  on round 1's volume, for A.4's open questions; its end also removes the
  three names round 1 left. `TestPriv94BProbe` rehearses it. Waiting for
  the author's VAX run.
- **2026-11-15** Subtask 2, round 2: the author ran it on VMS V7.1; its
  logs and image are in `testdata/priv94/vax/round2/`, and A.4 has its
  answers. Subtask 2 is done.
- **2026-11-15** Subtask 3, the gate and the table: `corevms/privrules.go`
  has the gate (`requirePrivilege`, `requirePrivilegeAt` for the services
  that judge by a mode argument, `$ENQ` and `$GETLKI`, and `holdsPrivilege`
  for the quiet answers) and the 38 rules (A.5), each a package variable
  made by `newRule`, which names its privileges by `$PRVDEF` name. Every
  check in `corevms` names a rule: the logical name services
  (`lnmTablePrivilege`, `lnmWriteMode`, `$CRELNT`'s shareable parent),
  `$CREMBX` (`mailboxCreatePrivileges`), `$DELMBX`, the mailbox's
  IO$_SETPROTECTION, `$ASCEFC`, `$DLCEFC`, `$CRMPSC`, `$DGBLSC`, `$ENQ`,
  `$GETLKI`, `mayAffect`, `$SETPRI`, `$CREPRC` (`checkCreatePrivileges`,
  now an `Environment` method, and its three quiet answers), `$CMKRNL`
  and `$CMEXEC`, `$LCKPAG`, `$SNDOPR`, `$BRKTHRU`, and disk block I/O.
  `hasPrivilege` and `hasAnyPrivilege` are gone, and the `priv*` bits
  with them, but for `privSETPRV`; the tests' copies are
  `privbits_test.go`. No behavior changed: each rewritten check gives
  the answer it gave, and asks the gate only where the privilege
  matters (a quiet answer is asked about only when the request goes past
  what the caller has), so the trace shows the decisions that count.
  Choices:
  - A rule several services share is named for the operation, not a
    service ("logical name: write the system table", "another process in
    the group"); `serviceRules` maps every registered service to its rules,
    and `affectingServices` and `unprivilegedServices` are the services
    with only `mayAffect`'s rules or none. `$SETPRN` doesn't reach another
    process in govax, so it has none.
  - `mayAffect`'s WORLD-or-GROUP test is two rules, "another process in
    the group" (GROUP or WORLD) and "a process in another group" (WORLD),
    the same answers, so the trace says which applied. `$GETLKI`'s WORLD
    is asked only when the lock's owner is in another group, as before.
  - `$SETPRV`'s own test (SETPRV in AUTHPRIV, or an inner mode) stays in
    `setPrivileges`: it manages the masks, and A.4 says VMS's test is
    SETPRV in CURPRIV, which subtask 4 makes a rule.
  - Three bits have two `$PRVDEF` names; the gate's names are AUTHORIZE's,
    ACNT, ALTPRI, and DETACH (`privilegeNamesByBit`), so `$CREPRC`'s
    PRC$M_NOACNT rule needs ACNT.
  - SET DEBUG PRIVILEGES (`registers.DebugPrivileges`, bit 29;
    `vax.debug.privileges`) traces every decision: `DEBUG(PRIVILEGES):
    pid mode mode: rule (needs ...): holds ... | exempt in its mode |
    refused, SS$_...`. HELP SET DEBUG and the configuration keys' help
    say so.
  - `TestNoUncheckedPrivileges` scans `internal` and `cmd` for any function
    or package variable that mentions `CurrentPrivileges`, against
    `managedPrivileges` (the gate, `$SETPRV`, SET PROCESS/PRIVILEGES, image
    rundown, `$CREPRC`'s and LIB$SPAWN's copies, JPI$_CURPRIV, the
    protection rule's credentials, SHOW PROCESS/PRIVILEGES).
    `TestEveryServiceHasPrivilegeRules` holds `serviceRules` to the
    registered services both ways, `TestEveryPrivilegeRuleUsed` finds each
    rule variable named outside the table and in some service's list, and
    `TestPrivilegeGate` checks the gate's answers and its trace.
- **2026-11-15** Subtask 4, the services held to the probe:
  `TestPriv94Oracle` (`internal/console/priv94oracle_test.go`) runs round
  1's program on govax as each identity (process 1 made SYSTEM, PRIV94,
  or PRIV94N: `priv94Identity.become`; the images given PRIV94.COM's
  protection so the accounts can run them), parts FH, the default parts
  as SYSTEM and PRIV94, SPX as PRIV94N, then part R and a run after it
  for image rundown, and holds each case to VMS's logs; `TestPriv94BOracle`
  does the same for round 2's parts M and U. What varies is masked (PIDs,
  job tables, mailbox units, a process's current priority, which VMS's
  scheduler boosts, and a wildcard `$GETJPI`'s NOPRIV count, which on VMS
  includes its own processes; govax's must be the holders it didn't
  see), and OPCOM's and the broadcasts' lines on govax's terminal are
  dropped. Parts L and T wait for subtasks 5 and 6; `priv94Differences`
  lists the rest (cases 106 and 108, a temporary page-file section VMS
  kept past its last mapping, in DEVIATIONS.md; SYSTEM's 84 and round
  2's 4 and 7, a permanent mailbox named by the system category,
  subtask 5's; 254, the VAX's IOSB count of terminals refusing a
  broadcast). Changed to VMS's answers:
  - The masks are 64 bits: `allPrivileges` and `security.AllPrivileges`
    are every bit, process 1 starts with them, `$SETPRV` and `$CREPRC`
    keep every bit asked for.
  - `$SETPRV`: enabling an unauthorized privilege is a rule,
    "`$SETPRV` a privilege not authorized" (SETPRV in CURPRIV, inner
    modes exempt, SS$_NOTALLPRIV), where it was SETPRV in AUTHPRIV;
    enbflg is its low bit; prmflg above 1 is SS$_IVSTSFLG, before prvprv
    is stored. `setPrivileges` takes the caller's mode (DCL's is
    supervisor).
  - `$CREPRC`: the new process's AUTHPRIV is the privileges it gets.
  - `$SETSWM` (`process.go`, PSWAPM from every mode, `Process.SwappingDisabled`,
    also set by PRC$M_PSWAPM).
  - `$CMEXEC` takes CMEXEC or CMKRNL; `$DELMBX` exempts kernel mode;
    a permanent mailbox's name takes SYSNAM or SYSPRV.
  - `$DGBLSC` takes SEC$M_GBL beside SEC$M_SYSGBL.
  - `$BRKTHRU` to one's own user needs nothing, to another OPER
    (SS$_NOOPER; the rule is "`$BRKTHRU` to another user"), and a user
    with no terminal is SS$_DEVOFFLINE in the IOSB.
  - A wildcard `$GETJPI` (and LIB$GETJPI) returns SS$_NOPRIV for each
    process the caller may not see, and goes on past it at the next call.
  - The rehearsal's kernel stack fault: a condition raised in an inner
    mode on a stack in S0 is searched for in that stack's frames alone,
    a saved FP leading out of S0 ending the search as 0 does
    (`condition.go`'s `leftStack`, `TestInnerModeConditionSearch`;
    unconfirmed, in DEVIATIONS.md).
  `$ULKPAG`, `$DLCEFC`, `$SNDOPR`'s classes, IO$_SETPROTECTION, `$ENQ`,
  the sections' and clusters' rules, disk I/O, and `$LCKPAG` already
  gave VMS's answers. `TestPriv94Probe` now allows no condition at a
  case's handler. A.5 is regenerated.
- **2026-11-15** Subtask 5, logical name tables' protection:
  `lnm.Table` has an owner, a protection, and a quota (`lnm/profile.go`,
  `quota.go`), VMS 7.1's defaults (the system table and directory
  `[1,4]` `(S:RWC,O:RWC,G:R,W:R)`, a group table `[g,*]`
  `(S:RWCD,O:R,G:R,W)`, a `$CRELNT` table its promsk and its maker);
  `corevms`'s `lnmTableAccess` asks `internal/security`'s rule for a
  shareable table, then SYSNAM (the system table) or GRPNAM (the
  process's group table), for `$CRELNM`, `$DELLNM`, `$CRELOG`,
  `$DELLOG`, LIB$SET_LOGICAL and LIB$DELETE_LOGICAL; `$TRNLNM` needs
  read access (`lnm.Database.TranslateWhere`); `$CRELNT` of a shareable
  table write access to LNM$SYSTEM_DIRECTORY, and a permanent mailbox's
  name write access to the system table. Part L of `TestPriv94Oracle`
  and round 2's parts N and D of `TestPriv94BOracle` match VMS's logs
  case for case; SYSTEM's case 84 and round 2's 4 and 7 are no longer
  differences. SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE is new
  (`dclcmd/showsecurity.go`, `TestShowSecurityTables`), and
  `$GET_SECURITY` of a table shows its own profile. DEVIATIONS.md's
  Phase 26 entry loses its first two items. Changed to VMS's answers:
  - `$DELLNM` (so DEASSIGN and LIB$DELETE_LOGICAL) deletes from the
    first table that has the name at the mode asked for, and nothing
    when no table has it there, even at an outer mode (NOLOGNAM, before
    the protection is asked: case 76); `lnm.Database.DeletionTable` is
    the table it works in, which the access check looks at.
  - `$CRELOG` and `$DELLOG` of the system table work at executive mode,
    kept with SYSNAM and otherwise maximized (cases 68 to 77).
  - `$CRELNT`'s quota: a child's comes out of its parent's, a name's
    size out of its table's, SS$_EXLNMQUOTA past it (round 1's 59 and
    60, round 2's 22). A name's size is govax's model (32 bytes, the
    name, 8 and the text per string, to 16), unconfirmed.
  - `$CRELNT`'s promsk is the table's protection, its maker its owner.
  Choices, unconfirmed: a job table is owned by the job's UIC,
  `(S:RWCD,O:RWCD,G,W)` (`lnm.Database.NewJobTable` takes the owner); a
  process-private table isn't checked; a search list's `$TRNLNM` passes
  over a table it may not read, NOPRIV only if no other has the name;
  SHOW SECURITY of a table no one has is SS$_NOSUCHOBJECT, as
  `$GET_SECURITY`'s. The tests' identities change process 1's group
  table and job table owner with `lnm.Database.SetUIC`. DCL's own
  DEFINE, ASSIGN, DEASSIGN, and CREATE/NAME_TABLE (which still makes a
  table with no promsk) are subtask 7's.
- **2026-11-15** Subtask 6, the terminal driver: IO$_SETCHAR needs LOG_IO
  or PHY_IO, from every mode, a new rule ("terminal IO$_SETCHAR", among
  `$QIO`'s), asked before the request is queued, so a refusal is R0
  SS$_NOPRIV with the IOSB untouched (`ttdriver.go`'s `ttSetMode`; its
  store is now `setTerminalCharacteristics`); IO$_SETMODE needs nothing,
  as before. Part T of `TestPriv94Oracle` matches VMS's logs case for
  case, with one masked word: a terminal IOSB's second word is VMS's
  transmit speed (6, TT$C_BAUD_300, the VAX's OPA0), which govax's
  console doesn't have and reports as 0 (`priv94LineSpeed`). DCL's SET
  TERMINAL is new (`dclcmd/terminal.go`, over `corevms/ttsettings.go`'s
  `TerminalSettings` and `SetTerminalSettings`, which ask the same rule
  at supervisor mode): `[device]` (SYS$COMMAND's by default), `/WIDTH`,
  `/PAGE`, `/DEVICE_TYPE` (a `TT$_` name), `/[NO]PERMANENT`, and a
  qualifier per characteristic bit (`dclcmd.TerminalSwitches`, 35, from
  the DCL Dictionary; UPPERCASE and HARDCOPY the opposites of LOWERCASE
  and SCOPE). Without the privilege `/PERMANENT` changes nothing and says
  `%SET-W-NOTSET, error modifying TTA0:` and `-SYSTEM-F-NOPRIV`, as VMS
  7.1 did: `TestPriv94Terminal` replays the probe's TERMINAL sections as
  each identity (`priv94dcl_test.go`'s `replayPriv94DCL`, for subtask 7's
  sections too), and `TestSetTerminal` the qualifiers. HELP SET TERMINAL
  has every qualifier. Choices, unconfirmed: govax keeps one set of
  characteristics, so a permanent change is a current one too, and a
  refused `/PERMANENT` changes nothing (whether VMS's SETMODE went first
  isn't in the logs); SET's NOTSET is a warning of SET's facility with a
  number of govax's (`setNotSet`); a device that isn't a terminal is
  NOTSET with SS$_IVDEVNAM; /WIDTH is 1 to 511, /PAGE 0 to 255.
