# Phase 95 — authentication

**Status:** planned 2026-11-15 (subtask 1, the survey and this plan);
the plan reviewed and accepted by the author the same day, with the
answers in "The review". This document is the development plan; as
the subtasks are done it becomes the implementation record, with a
progress log and, at the end, "Where things stand".

## Goal

govax gets user accounts. Phases 93 and 94 made file protection and
privileges real, but every process is still SYSTEM, `[1,4]`, unless a
test builds another in Go. This phase makes the accounts a thing you
can create, change, and log in as:

- an **authorization file**, govax's SYSUAF.DAT kept as JSON on the host
  (`~/.govax/sysuaf.json` unless `vax.authorization.file` names
  another), made with the SYSTEM account the first time govax starts;
- **`--user NAME`** (`-u`), the account process 1 logs in as, SYSTEM by
  default; no password yet (that waits for remote connections);
- **AUTHORIZE**, VMS's utility as a DCL verb: `AUTHORIZE` alone prompts
  `UAF>` until EXIT or Ctrl/Z, `AUTHORIZE command` runs one command, and
  `MCR AUTHORIZE` is the same;
- the **login procedures**: `SYS$MANAGER:SYLOGIN.COM`, then the user's
  `LOGIN.COM` (or the account's LGICMD) from the home directory, after
  STARTUP.COM;
- the **system volume's protection**: every directory and file under
  `DUA0:[SYSTEM...]` owned by `[1,4]` and readable by everyone, writable
  only by its owner;
- **`SYS$MANAGER:ADDUSER.COM`**, a procedure that asks for a new
  account's facts, runs AUTHORIZE, and makes the home directory with the
  right owner and protection.

    $ AUTHORIZE ADD SMITH/UIC=[200,1]/DEVICE=DUA1:/DIRECTORY=[SMITH] -
          /PRIVILEGES=(TMPMBX,NETMBX)/DEFPRIVILEGES=(TMPMBX,NETMBX)
    %UAF-I-ADDMSG, user record successfully added
    $ EXIT
    % govax --user smith
    $ SHOW PROCESS
    ...  User: SMITH   Process ID: 00000301 ...
    $ DELETE DUA0:[SYSTEM.SYSEXE]DEFAULT.ROM;*
    %DELETE-W-FILNOTDEL, error deleting DUA0:[SYSTEM.SYSEXE]DEFAULT.ROM;1
    -RMS-E-PRV, insufficient privilege or file protection violation

(The messages are VMS 7.1's as the probe will show them; the lines
above are what the plan expects.)

The authorization file is govax's own format, not SYSUAF.DAT's. The
commands, their qualifiers, their messages, and their reports follow
VMS's AUTHORIZE, but only for the attributes govax has (A.3): no access
time windows, password lifetimes, expiration dates, or login failure
counts yet. The DISUSER flag is in.

## Part A — the survey (2026-11-15)

### A.1 What VMS has

**SYSUAF.DAT** is an indexed file of one record per account (the
username its primary key, the UIC an alternate key), found through the
logical name SYSUAF, else `SYS$SYSTEM:SYSUAF.DAT`. Its protection keeps
it from everyone but the system category. **RIGHTSLIST.DAT** holds the
identifiers (a name, a value, attributes) and which accounts hold which.
AUTHORIZE (`SYS$SYSTEM:AUTHORIZE.EXE`, run as `MCR AUTHORIZE` or `RUN
SYS$SYSTEM:AUTHORIZE`) edits both. It needs read and write access to the
files, which the system category has by their protection and SYSPRV
gives anyone.

**A UAF record's fields**, as AUTHORIZE's qualifiers set them (from the
*Guide to System Security* and the DCL dictionary's account-related
material; the System Management Utilities manual's AUTHORIZE chapter is
not among our references, A.4):

| Group | Fields (qualifiers) | govax |
|---|---|---|
| Identity | username, /OWNER, /ACCOUNT, /UIC | all |
| Home | /DEVICE, /DIRECTORY | both |
| Login | /CLI, /CLITABLES, /LGICMD, /FLAGS | CLI (DCL only), LGICMD, FLAGS: DISUSER, and those A.3 lists |
| Password | /PASSWORD, /GENERATE_PASSWORD, /PWDLIFETIME, /PWDMINIMUM, /PWDEXPIRED, /EXPIRATION | /PASSWORD only (our hash, Decision 4) |
| Times | /ACCESS, /PRIMEDAYS, /BATCH, /DIALUP, /INTERACTIVE, /LOCAL, /NETWORK, /REMOTE | none (later) |
| Privileges | /PRIVILEGES (authorized), /DEFPRIVILEGES (default) | both |
| Priority | /PRIORITY, /QUEPRIO | /PRIORITY; /QUEPRIO kept, unused |
| Quotas | /ASTLM, /BIOLM, /BYTLM, /DIOLM, /ENQLM, /FILLM, /JTQUOTA, /PGFLQUOTA, /PRCLM, /TQELM, /WSDEFAULT, /WSQUOTA, /WSEXTENT, /CPUTIME, /MAXJOBS, /MAXACCTJOBS, /MAXDETACH, /SHRFILLM | all kept; given to the process where govax has the quota (A.2) |

**AUTHORIZE's commands** on accounts: ADD, COPY, DEFAULT (the DEFAULT
record new accounts copy), LIST (writes SYSUAF.LIS, `/BRIEF` or
`/FULL`), MODIFY, REMOVE, RENAME, SHOW (`/BRIEF`, `/FULL`), HELP, EXIT;
wildcards in a username for SHOW, LIST, and MODIFY. On identifiers:
ADD/IDENTIFIER, REMOVE/IDENTIFIER, RENAME/IDENTIFIER, MODIFY/IDENTIFIER,
GRANT/IDENTIFIER, REVOKE/IDENTIFIER, SHOW/IDENTIFIER, SHOW/RIGHTS,
LIST/IDENTIFIER, LIST/RIGHTS, CREATE/RIGHTS. On proxies (network):
ADD/PROXY and its kin, out of scope. Messages are the UAF facility's
(`%UAF-I-ADDMSG`, `%UAF-W-BADSPC`, ...). ADD gives a new record the
DEFAULT record's values, and (in 7.1) the password USER, pre-expired.

**Login** (LOGINOUT, and the Internals book's chapter on process
creation): the account is read; DISUSER refuses it; the process gets
the record's username, account, UIC, authorized and default privileges,
priority, quotas, and rights (the account's identifiers plus the
environmental ones: INTERACTIVE, LOCAL, ...); `SYS$LOGIN`,
`SYS$LOGIN_DEVICE`, and `SYS$SCRATCH` are defined in the job table, in
executive mode, from DEVICE and DIRECTORY; the default directory is set
to them; DCL runs `SYS$SYLOGIN` (STARTUP defines it, usually as
`SYS$MANAGER:SYLOGIN.COM`), then the account's LGICMD, `SYS$LOGIN:LOGIN.COM`
by default. Neither file need exist. A failure in one ends that
procedure, not the login.

### A.2 What govax has today

- `corevms/users.go`: `System.users`, a table of `UserAccount`s (name,
  UIC, authorized privileges, granted identifiers), filled by `AddUser`
  and used by `$CHECK_ACCESS` and `$CHKPRO`'s items (Phase 93's subtask
  13a). SYSTEM is always in it. The tests stand for a probe's VAX with it.
- `corevms/process.go`: `NewProcess` makes every process SYSTEM:
  username, account, and process name SYSTEM, UIC `[1,4]`, every
  privilege authorized, process, and current, priority 4, nominal
  quotas, the interactive rights. `corevms/sysgen.go`'s `Account` and
  `SystemAccount` hold only the working-set and paging-file quotas, laid
  over by the `PQL_D*` configuration in `LoginQuotas`, whose comment
  already expects "a later SYSUAF".
- `lnm.Database.SetUIC` (Phase 94) gives the logical name tables a
  process's UIC as owner. `job.go` gives a subprocess its owner's
  username, account, and UIC; `$CREPRC` takes a UIC.
- `$GETJPI` answers 63 items, among them USERNAME, ACCOUNT, UIC, GRP,
  AUTHPRIV, PROCPRIV, CURPRIV, AUTHPRI, the quotas, and LOGINTIM.
  F$GETJPI reads the same code.
- Process 1's default directory is the default volume's
  (`vax.default.volume.directory`, `[WORK]` on DUA1 in the manual's
  examples), set by `cmd/govax` at startup. There is no `SYS$LOGIN`
  (MANUAL.md says so).
- STARTUP.COM runs at level 1 at startup (`Console.RunStartup`), then
  SYSTARTUP_VMS.COM, then the one-shot command or the prompt. There is
  no SYLOGIN and no LOGIN.COM.
- The system volume (`internal/sysvol`) is written by process 1 as
  SYSTEM, so its files are owned by `[1,4]`, with the default protection
  (`vax.sysgen.rms_fileprot`, `%XFA00`: S:RWED, O:RWED, G:RE, W:) and
  directories as CREATE/DIRECTORY makes them. Nothing sets them on
  purpose, and the refresh (`sysvol.Refresh`) never looks at them. A
  user outside the system category can't read SYS$LIBRARY's files today.
- `MCR image` runs an image (`dcldispatch.go`). CREATE/DIRECTORY takes
  `/OWNER_UIC` and `/PROTECTION`. Grammars are files in
  `internal/bootdata/files` (`console.dcl`, `debug.dcl`), each with its
  own dispatcher; the debugger's session (`DBG>`, its own history) is
  the model for a utility's prompt.
- `cmd/govax`'s global options use the short names s, p, i, t, m; `-u`
  is free. The profile directory is `~/.govax`, where the system volume
  and the history already live.
- `vmsdef` has no UAF messages or `UAF$` definitions.

### A.3 What govax will keep, and what it won't yet

Kept and used: username, owner, account, UIC, device, directory, CLI
(DCL only; another name is accepted and refused at login), LGICMD,
the flags below, authorized and default privileges, priority, every
quota (the ones govax's processes have are given to them: ASTLM, BIOLM,
DIOLM, BYTLM, ENQLM, FILLM, PGFLQUOTA, PRCLM, TQELM, WSDEFAULT,
WSQUOTA, WSEXTENT; the rest are kept and shown), the password hash, the
identifiers granted, and the times of the last login and the last
password change (shown by SHOW, as VMS shows them).

Flags: DISUSER (refuses the login), and DISCTLY, DEFCLI, LOCKPWD,
CAPTIVE, DISNEWMAIL, DISWELCOME, NOTIFY... are accepted, stored, and
shown, each marked in HELP as having no effect yet. The probe lists
7.1's flags; govax takes the same list.

Not kept: access times and primary days, password lifetime, minimum,
expiration, generated passwords, login failure counts, proxies. Their
qualifiers are refused with AUTHORIZE's own message for an unknown
qualifier (`%CLI-W-IVQUAL` through the grammar), and HELP AUTHORIZE says
they are missing.

### A.4 What the references leave open

1. **AUTHORIZE's layouts and messages.** We have no AUTHORIZE manual as
   text. The *Guide to System Security* (VAX 6.0, a PDF) shows examples;
   the probe (Part D) must show SHOW's `/FULL` and `/BRIEF` layouts,
   LIST's file, each message, the DEFAULT record's 7.1 values, which
   qualifiers ADD requires (/UIC), and how a wildcard username behaves.
2. **The SYSTEM account's values.** The JSON file's SYSTEM record must
   give process 1 exactly what it has today (the user's request), so its
   values are `NewProcess`'s and `SystemAccount`'s, not 7.1's SYSTEM
   record. The probe shows 7.1's for comparison only.
3. **Login's order and names**: whether SYS$LOGIN and friends are job or
   process names, in which mode; what F$MODE is in SYLOGIN; what a
   missing LGICMD file does (nothing, on VMS, we believe); the DISUSER
   message (`%LOGIN-F-DISUSER`?). A batch job submitted as the probe
   account shows all of these in its log.
4. **F$GETJPI's account items** 7.1 has that govax lacks (MAXJOBS,
   MAXDETACH, SHRFILLM, JTQUOTA?): the probe lists every item code's
   answer for the probe account.

### A.5 The probe's answers (2026-11-15)

Three rounds on VMS 7.1 (`testdata/uaf95`, its README's "Round 1" to
"Round 3"; logs in `vax/`, `vax/round1/`, `vax/round3/`).

**AUTHORIZE** (round 2's 134 cases and round 3's 13, all on scratch files):

- **The files.** SYSUAF is found through a supervisor-mode logical name,
  but RIGHTSLIST only through an executive-mode one (round 1 used the
  VAX's own rights database). A missing SYSUAF.DAT is `%UAF-E-NAOFIL`
  with RMS's FNF, then the question whether to make one; the new file
  holds SYSTEM and DEFAULT. A user without access gets NAOFIL with
  RMS-E-PRV; SYSUAF.DAT is (S:RWED,O:RWED,G:,W:) on that VAX.
- **The new file's records.** SYSTEM: owner SYSTEM MANAGER, account
  SYSTEM, `[1,4]`, `SYS$SYSROOT:[SYSMGR]`, every privilege, PRCLM 10,
  ASTLM 50, TQELM 30, ENQLM 300, WSEXTENT 2048, PGFLQUOTA 40960, password
  pre-expired. DEFAULT: `[200,200]`, DISUSER, directory `[USER]`, CLI DCL,
  TMPMBX and NETMBX, PRCLM 2, ASTLM 40, BIOLM and DIOLM 40, BYTLM 32768,
  FILLM 300, JTQUOTA 4096, WSDEFAULT 256, WSQUOTA 512, WSEXTENT 1024,
  PGFLQUOTA 32768, ENQLM 200, TQELM 40, PRIO and QUEPRIO 4, password
  lifetime 90 days, minimum 6, pre-expired. (The VAX's own DEFAULT had
  LGICMD `LOGIN`; a new file's is empty.)
- **ADD** copies DEFAULT, needs no /UIC (DEFAULT's is taken), and adds
  the username as a UIC identifier (`%UAF-I-RDBADDMSGU, identifier X
  value [000300,000001] added to rights database`); an identifier that
  exists (a UIC another account has, `[200,200]` from DEFAULT) is
  `%UAF-E-RDBADDERRU` with `-SYSTEM-F-DUPIDENT`, the record added anyway.
  ADD makes a group identifier from /ACCOUNT when the group has none
  (`CASED [000300,177777]`). A UIC's numbers are **octal** (`[300,8]` is
  UICERR); `[300]` and `[400000,1]` are UICERR too. A username over 12
  characters is NAMETOOBIG, a bad character INVUSERNAME, a duplicate
  UAEERR, a bad number BADVALUE, a priority of 99 VALTOOBIG.
- **COPY and RENAME** need a new password (`%UAF-W-DEFPWD`); COPY with
  the same UIC is `%UAF-W-SAMEUIC`; a missing source is BADUSR. RENAME
  renames the UIC identifier (`%UAF-I-RDBMDFYMSG`). REMOVE removes it.
- **MODIFY** of a wildcard (`UAF%`, `UAF*`) updates each match, one
  MDFYMSG; of a missing user BADUSR; a changed /CLI warns CLIWARNMSG.
- **SHOW**'s full layout (the record's fields in two columns, the
  quotas in three, the privileges eight to a line, the identifiers held
  under `Identifier  Value  Attributes`), `/BRIEF`'s one line per account,
  and a username matching nothing `%UAF-W-BADSPC` (or SHOWERR with no
  username). `/FULL` adds the identifiers held.
- **Identifiers.** ADD/IDENTIFIER gives the next `%X8001nnnn` value;
  GRANT, REVOKE, RENAME, MODIFY, and REMOVE with their messages
  (GRANTMSG, GRANTERR, GRANTUSR, REVOKEMSG, REVOKEERR, RDBMDFYERR,
  RDBREMERR, ...). LIST/IDENTIFIER and LIST/RIGHTS need a name
  (`%UAF-I-NOIDNAME`); with one, they write RIGHTSLIST.LIS.
- **LIST** writes SYSUAF.LIS in the default directory (LSTMSG1,
  LSTMSG2): `/BRIEF` (the default) a heading and a line per account,
  `/FULL` SHOW's layout per account.
- **Exit messages.** A session ends, at EXIT or at the end of its input,
  with DONEMSG or NOMODS for SYSUAF and RDBDONEMSG or RDBNOMODS for the
  rights database. A one-command run (`MCR AUTHORIZE ADD ...`) prints only
  its command's messages.
- **`$STATUS`** after a one-command run follows no one rule: some errors
  leave %X00000001 (UICERR, VALTOOBIG, DEFPWD), some give AUTHORIZE's own
  condition (UAEERR %X13F1943A, NAMETOOBIG %X13F19352, INVUSERNAME
  %X13F1931A), and some the underlying system status with the inhibit
  bit (GRANTERR, SHOWERR, RDBMDFYERR %X100021EC). The same message can
  differ by command (BADSPC: %X00000001, %X100182B2, %X13F18C80). The
  oracle test (subtask 5) holds each case to its log.

**Logging in** (rounds 2 and 3, by batch job and at the terminal):

- **The order.** SYLOGIN (`SYS$SYLOGIN`), then the account's LGICMD if it
  has one, else `SYS$LOGIN:LOGIN.COM`, never both. DCL puts the account's
  procedure at level 1 and runs SYLOGIN above it first:
  `F$ENVIRONMENT("DEPTH")` is 2 in SYLOGIN when a login procedure
  follows, 1 when none can run.
- **A missing or unreadable login procedure** is skipped with no message
  (round 2).
- **Failures.** A login procedure follows DCL's default ON action: a
  warning goes on, an error ends the procedure. At the terminal the
  login continues to the prompt; in a batch job the error ends the job.
- **Verify** is off in both interactively, on in batch. P1 is empty.
- **Names.** `SYS$LOGIN`, `SYS$LOGIN_DEVICE`, and `SYS$SCRATCH` are
  executive-mode names in the job table (SYS$SCRATCH the home directory);
  `SYS$DISK` is an executive-mode process name; the default directory is
  the home directory. The job table is owned by the account's UIC.
- **The process** gets the record's username, account, UIC, privileges
  (authorized and default), priority, and quotas; its rights are
  INTERACTIVE and LOCAL (or BATCH), and the system rights hold
  SYS$NODE_node. The login banner says `Last interactive login on ...`
  or `Last non-interactive login on ...`.
- **DISUSER** is `%LOGIN-F-DISUSER, account is disabled` at the terminal,
  and the same for a batch job; OPCOM shows a security alarm.
- **F$GETJPI** answered every item it was asked, among them LAST_LOGIN_I,
  LAST_LOGIN_N (17-NOV-1858 when none), LOGIN_FAILURES, LOGIN_FLAGS,
  MAXDETACH, MAXJOBS, SHRFILLM, and UAF_FLAGS, which govax lacks.

## Part B — decisions

**Decision 1 — `internal/uaf`, a leaf package.** The authorization file
is a package of plain Go: `Record` (an account), `Identifier`, `File`
(the whole file: a version, the records, the identifiers, the next free
identifier value), `Load`, `Save`, validation (the username's form, a
UIC's range, a privilege's name), the password hash (Decision 4), and
`Default()` (a new file: SYSTEM and the DEFAULT record). It knows
nothing of processes, RMS, or DCL. `corevms`'s `UserAccount` table is
replaced by the file: `System.User` reads it, `AddUser` (the tests'
way) writes an in-memory one, and `Account`/`SystemAccount` become a
`uaf.Record`'s quotas.

**Decision 2 — the file.** JSON, indented, one object per account with
names a person can read and edit (privileges as names, a UIC as
`"[1,4]"`, flags as names), and a `"version": 1` key so a later format
can tell. Read whole at each use (startup, each AUTHORIZE command,
`$CHECK_ACCESS` of a name not cached), written whole by a temporary
file and a rename, under `internal/hostlock` so two govax sessions
sharing the file don't lose each other's changes. The host file's mode
is 0600. A file govax can't parse stops startup with a message naming
the file and the line, rather than starting as SYSTEM silently.

The location is `vax.authorization.file`, default
`~/.govax/sysuaf.json`, `~` expanded as `--system-file`'s is. The tests
use a temporary file (`cmd/govax`'s `TestMain` sets one, as it sets
`--system-file none`), never the author's own.

    {
      "version": 1,
      "accounts": [
        {
          "username": "SYSTEM",
          "owner": "SYSTEM MANAGER",
          "account": "SYSTEM",
          "uic": "[1,4]",
          "cli": "DCL",
          "lgicmd": "",
          "device": "DUA1:",
          "directory": "[WORK]",
          "flags": [],
          "priority": 4,
          "privileges": ["ALL"],
          "defprivileges": ["ALL"],
          "quotas": { "astlm": 50, "biolm": 40, "diolm": 40, "wsdefault": 512, ... },
          "password": "",
          "identifiers": [],
          "last_login": "...",
          "password_changed": ""
        },
        { "username": "DEFAULT", ... }
      ],
      "identifiers": [ { "name": "PROJECTX", "value": "%X80010001", "attributes": [] } ]
    }

**Decision 3 — the SYSTEM record matches today.** A new file's SYSTEM
record holds `NewProcess`'s values (privileges all, priority 4, ASTLM
50, BIOLM and DIOLM 40, WSDEFAULT 512, WSQUOTA 1024, WSEXTENT 16400,
the nominal PGFLQUOTA, ...), its CLI DCL, its LGICMD empty (so
`SYS$LOGIN:LOGIN.COM`). Its home is SYS$MANAGER, as on VMS: DEVICE
`SYS$SYSROOT:` and DIRECTORY `[SYSMGR]`. The one exception
(the review, question 2): when the configuration sets
`vax.default.volume.directory` explicitly, SYSTEM's home is that
directory on the default volume's device, so a configured starting
place still wins. (No new key: that one already names a default
directory. Its built-in default, `[WORK]`, doesn't count; only a value
the configuration gives.) So by default process 1 now starts in
SYS$MANAGER, where it started in the default volume's `[WORK]` before;
the default volume is still mounted. With no system volume
(`--system-file none`, as the tests run), SYS$SYSROOT doesn't exist, and
SYSTEM's home falls back to the default volume's directory, then to the
host's current directory, as today. The home is resolved at login, not
written into the record as a physical name, so the record stays right
when the volume moves. The `PQL_D*` configuration keeps laying
over the record's quotas, as it lays over `SystemAccount`'s today
(Phase 62's Decision 11), so a configured value still wins.

**Decision 4 — the password hash.** Argon2id (RFC 9106), stored as the
PHC string `$argon2id$v=19$m=65536,t=3,p=4$salt$hash`, from
`golang.org/x/crypto/argon2` (a new dependency, from the `golang.org/x`
family govax already uses). The string names its own parameters, so
they can be raised later and old hashes still checked. An empty
password field means no password: such an account can be logged into
with `--user` (which asks for none) but not, later, remotely. ADD
without `/PASSWORD` gives no password, not VMS's pre-expired USER,
since govax has no expiration (A.3). `uaf.CheckPassword` exists, tested,
for the remote-login phase. (If the author prefers no new dependency:
PBKDF2-HMAC-SHA256 from the standard library, 600,000 iterations.)

**Decision 5 — who may use the file.** The JSON file is a host file,
outside the volumes' protection, so govax gives it SYSUAF.DAT's profile
in Go: owner `[1,4]`, protection (S:RWE, O:RWE, G:, W:). AUTHORIZE (and
anything else reading the file for a process, as `$CHECK_ACCESS`
doesn't need to) asks `internal/security`'s rule with the process's
credentials: read for SHOW and LIST, write for the rest. So SYSTEM, any
system-category UIC, and a process with SYSPRV (or BYPASS) may use it,
and others get `%UAF-E-RDEERR`-like failures as VMS gives (the probe's
message, A.4 1). Logging in reads the file in Go, as LOGINOUT does.

**Decision 6 — `--user`.** A global option, `--user NAME` / `-u NAME`,
and `vax.default.user` is not added (the option is enough). The account
must exist and not be DISUSER, or govax prints LOGINOUT's message and
exits nonzero before anything runs. Process 1 then logs in (Decision 7)
after STARTUP.COM, which always runs as SYSTEM, as VMS's STARTUP does
in the SYSTEM process. The one-shot subcommands (`govax run`, `macro`,
...) honor `--user` too, so a privilege or protection can be tried from
the shell.

*The password* (added 2026-11-15 by the author, after subtask 6): an
account with a password must be given it before it logs in, so the
remote logins to come start from the same rule. When the account `--user`
names has a non-empty password, or SYSTEM has one and `--user` isn't
given, govax asks `Password: ` with echo off and checks the answer with
`uaf.CheckPassword` before anything runs; three tries at a terminal, each
failure VMS's `User authorization failure` (which never says whether the
username or the password was wrong), then exit nonzero. When standard
input isn't a terminal (a pipe, a script) govax doesn't read the password
from it, which would take the commands meant for the session: it uses
the `GOVAX_PASSWORD` environment variable if that is set, and otherwise
fails at once with a message saying the account needs a password. The
one-shot subcommands follow the same rule. An account with no password
logs in as before, asked for nothing. No `--password-file` option (not
yet needed).

**Decision 7 — logging in, one routine** (held to A.5's answers). `corevms.Environment.LogIn(rec
uaf.Record)` does what LOGINOUT does for a process, and is the one place
an account becomes a process: username, account, owner, UIC (and the
logical name database's, `SetUIC`), authorized privileges, process and
current privileges from DEFPRIVILEGES, priority and authorized
priority, the quotas govax has (`LoginQuotas` taking the record), the
rights (the record's identifiers plus the environmental ones),
LOGINTIM, and the record's last-login time written back. DCL's side,
`dclshell`, defines `SYS$LOGIN`, `SYS$LOGIN_DEVICE`, and `SYS$SCRATCH`
(job table, executive mode, SYS$SCRATCH the home directory, as A.5
found), sets the default directory to them, and runs the login
procedures (Decision 8). The record keeps the last interactive and
non-interactive logins apart, as VMS's does; `$GETJPI` gains the items
A.5 lists (LAST_LOGIN_I, LAST_LOGIN_N, LOGIN_FAILURES, LOGIN_FLAGS,
MAXDETACH, MAXJOBS, SHRFILLM, UAF_FLAGS); and process 1's login prints
VMS's `Last interactive login on ...` line.
`$CREPRC` of LOGINOUT and LIB$SPAWN keep today's rules (a subprocess
takes its owner's identity); a detached LOGINOUT process looked up by
username waits for the remote-login phase.

**Decision 8 — the login procedures** (held to A.5's answers). After
STARTUP.COM (and SYSTARTUP_VMS.COM), process 1's DCL runs `SYS$SYLOGIN`
if that name is defined, then the account's LGICMD, or
`SYS$LOGIN:LOGIN.COM` when it has none, as VMS does: the account's
procedure at level 1 and SYLOGIN above it at level 2, run first (so
`F$ENVIRONMENT("DEPTH")` is VMS's in each). A procedure that is missing
or can't be read is skipped with no message. Each follows DCL's default
ON action (a warning goes on, an error ends that procedure) and the
login goes on to the prompt; Ctrl/Y in one ends it. Verify is off. STARTUP.COM defines `SYS$SYLOGIN` as VMS's
does. They run for every login, SYSTEM's too, in an interactive
session; the one-shot subcommands skip them (they run one command, and
their output must stay predictable). There is no option to skip them
in a session yet (the review, question 4); a configuration key can be
added if one turns out to be needed. The system volume supplies a
SYLOGIN.COM that is *seeded* (written once, never replaced, the user's
to edit, as SYSTARTUP_VMS.COM is), with comments and nothing else.

**Decision 9 — the system volume's protection.** `sysvol` says what
each directory's and file's owner and protection are, and `Populate`
and `Refresh` set them: owner `[1,4]`; files (S:RWED, O:RWED, G:RE,
W:RE), directories (S:RWE, O:RWE, G:RE, W:RE), as VMS's own system
files and directories are. That is "readable by anyone, writable only by
the owner", the system category counting as the owner as it does on
VMS (the review, question 3, asks whether S should lose write too). The
refresh corrects an existing volume's protection once, noting it in the
manifest, and leaves a file the user changed with SET SECURITY alone
afterward (the manifest records the protection it set). The page file
keeps its own (S:RWED, O:RWED, G:, W:) and SYS$MANAGER's startup files
(STARTUP.COM, SYSTARTUP_VMS.COM, SYLOGIN.COM, ADDUSER.COM) get W:RE too,
so a user can read SYLOGIN.COM to run it.

**Decision 10 — AUTHORIZE, a utility with its own grammar.** The
commands are a grammar file of their own, `authorize.dcl`, in
`internal/bootdata/files`, run by `internal/authorize` (the commands,
the session, the reports) over `internal/uaf`, as the debugger's are by
`internal/debugger`. DCL's verb `AUTHORIZE [command]` is in
`console.dcl`; `MCR AUTHORIZE` and `RUN SYS$SYSTEM:AUTHORIZE` reach the
same code (MCR and RUN look the name up among govax's built-in
utilities first, as the image locator does for FTNRTL). The session
prompts `UAF> `, keeps its own history (`history.go`), ends at EXIT or
end of file, and reads a procedure's data lines when run from one (so
ADDUSER.COM can give it commands). Messages are the UAF facility's,
added to `vmsdef` from the probe's log. HELP inside AUTHORIZE reads a
help library of its own, `SYS$HELP:UAFHELP.HLB`, built like DBG$HELP.HLB
(Phase 90).

**Decision 11 — identifiers are in.** The file keeps identifiers and
their holders, since Phase 93's ACLs and `$CHECK_ACCESS` already use
them: ADD/IDENTIFIER, REMOVE/IDENTIFIER, RENAME/IDENTIFIER,
MODIFY/IDENTIFIER, GRANT/IDENTIFIER, REVOKE/IDENTIFIER, SHOW/IDENTIFIER,
SHOW/RIGHTS, LIST/IDENTIFIER, LIST/RIGHTS; a UIC identifier for each
account, as ADD makes on VMS. A logged-in process holds its granted
identifiers (Decision 7), so an ACL entry naming one works for it.
CREATE/RIGHTS is accepted and does nothing (the file always has a
rights part). Proxies are out.

**Decision 12 — ADDUSER.COM.** A supplied (not seeded) procedure in
SYS$MANAGER: it asks for the username, owner, UIC (offering the next
free member of a group it asks for), password (with SET TERMINAL/NOECHO,
added if govax lacks it), home device and directory (offering the
default volume and `[username]`), and privileges (a short menu: a plain
user, TMPMBX and NETMBX; or a list); runs `AUTHORIZE ADD`; makes the
directory with CREATE/DIRECTORY/OWNER_UIC and protection (S:RWE, O:RWE,
G:, W:), unless it exists, when it sets the owner; writes a starter
LOGIN.COM there, owned by the user; and says how to log in. It checks
that its user may (SYSPRV or a system UIC) before asking anything.

**Decision 13 — tests.** The oracle tests hold AUTHORIZE to the probe's
log, as Phase 90's held HELP. `TestEveryAuthorizeQualifierHasHelp` as
Phase 90's tests do for DCL. End to end: ADDUSER.COM run from a
procedure, then a session `--user` the new account, its file access
held to Phase 93's rules (the system volume readable, not writable; its
home writable), its LOGIN.COM run, `F$GETJPI` showing its values.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The probe** (done 2026-11-15): `testdata/uaf95`, as in Part D,
   written and rehearsed where govax can; run on the VAX by the author in
   three rounds; A.4 answered (A.5) and Part B adjusted.
3. **The authorization file** (done 2026-11-15) (Decisions 1 to 5): `internal/uaf`
   (records, identifiers, JSON, locking, the hash, validation, the
   default file), `vax.authorization.file`, the file made at startup;
   `corevms`'s user table and `SystemAccount` onto it, with no change in
   what process 1 gets.
4. **Logging in** (done 2026-11-15) (Decisions 6 and 7): `--user`/`-u`, `Environment.LogIn`,
   SYS$LOGIN and its kin, the default directory, DISUSER's refusal, the
   new `$GETJPI` and F$GETJPI items the probe lists, LOGINTIM and the
   last-login time.
5. **AUTHORIZE's account commands** (done 2026-11-15) (Decision 10): `authorize.dcl`,
   `internal/authorize`, the `UAF>` session and the one-command form, MCR
   and RUN; ADD, COPY, DEFAULT, MODIFY, REMOVE, RENAME, EXIT; the access
   check (Decision 5); the UAF messages; held to the probe.
6. **AUTHORIZE's reports and identifiers** (done 2026-11-15) (Decision 11): SHOW
   (`/BRIEF`, `/FULL`, wildcards), LIST (SYSUAF.LIS, RIGHTSLIST.LIS),
   the identifier commands, rights given at login; held to the probe.
7. **The system volume's protection** (done 2026-11-15) (Decision 9): owners and
   protections in `sysvol`, set by Populate and Refresh, existing
   volumes corrected once; SYLOGIN.COM seeded; `SYS$SYLOGIN` in
   STARTUP.COM.
8. **The login procedures and the password** (done 2026-11-15) (Decisions 6 and 8): SYLOGIN
   then LGICMD or LOGIN.COM, their order, failures, and Ctrl/Y, held to
   the probe's batch log; the password asked at login for an account that
   has one, `GOVAX_PASSWORD` when standard input isn't a terminal
   (Decision 6's "The password").
9. **ADDUSER.COM** (done 2026-11-15) (Decision 12), with SET TERMINAL/NOECHO if missing,
   and the end-to-end tests (Decision 13).
10. **The close-out**: README.md (you log in as SYSTEM by default, and
    can make accounts to model privileges and protection), MANUAL.md (a
    section on accounts: the file, `--user`, AUTHORIZE, the login
    procedures, ADDUSER.COM; SYS$LOGIN no longer missing; the options
    and keys), HELP AUTHORIZE and UAFHELP.HLB, `VAX-VMS-INTERNALS.md`'s
    processes section, DEVIATIONS.md, PLAN.md, CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/uaf95/`, on an exchange volume as the earlier probes were.
It never touches the VAX's own SYSUAF: it defines `SYSUAF` and
`RIGHTSLIST` as process logical names to copies on the exchange volume
(AUTHORIZE uses them, and makes the files if they are missing), so the
whole run is on scratch files, removed at the end. `gen.go` writes every
file.

- **`uaf95.com`**, AUTHORIZE's side: `MCR AUTHORIZE` reading its
  commands from the procedure, each command and its output logged: SHOW
  of SYSTEM and DEFAULT (`/FULL`, `/BRIEF`), ADD with each qualifier
  govax keeps, ADD without /UIC, a duplicate ADD, a bad username, a bad
  UIC, COPY, RENAME, MODIFY of each field and of a wildcard, DEFAULT,
  REMOVE, LIST `/BRIEF` and `/FULL` (and the files typed), every
  identifier command, a qualifier govax won't keep (to see 7.1's
  words for it), and AUTHORIZE run by an account without SYSPRV against
  the real SYSUAF (refused, the message).
- **`uaf95i.com`**, login's side, run by batch jobs submitted
  `/USER=` each probe account and by each account by hand after logging
  in: what the login procedures recorded, SHOW LOGICAL/FULL of
  SYS$LOGIN, SYS$LOGIN_DEVICE, SYS$SCRATCH, and SYS$SYLOGIN, the job and
  process tables, SHOW DEFAULT, SHOW PROCESS (/PRIVILEGES, /RIGHTS,
  /QUOTAS), F$MODE, F$GETJPI of every item the 7.3-1 dictionary lists,
  AUTHORIZE without SYSPRV, and what the account can read and write of
  the system's files.
- **The login procedures**: `uaf95s.com` is SYS$SYLOGIN while the probe
  runs (`uaf95acct.com` keeps the old translation and mode, and
  `uaf95drop.com` puts it back): for the probe's accounts it records
  what it sees in global symbols, and for everyone else it runs the
  VAX's own SYLOGIN. `login.com` (UAF95's SYS$LOGIN:LOGIN.COM) and
  `uaf95x.com` (UAF95L's LGICMD, which fails with a warning, then with
  an error) record the same.
- **`uaf95acct.com`** and **`uaf95drop.com`**, run by SYSTEM on the
  VAX before and after the probe (the review, question 6): the first
  makes the UAF95 account in the real SYSUAF (`ADD UAF95/UIC=[200,5]
  /OWNER="GOVAX PROBE"/ACCOUNT=GOVAX/PASSWORD=` a password the README
  gives`/NOPWDEXPIRED/DEVICE=` the exchange volume`/DIRECTORY=[UAF95]
  /PRIVILEGES=(TMPMBX,NETMBX)/DEFPRIVILEGES=(TMPMBX,NETMBX)
  /FLAGS=NODISUSER`, no LGICMD), UAF95L `[200,7]` (the same with
  LGICMD `UAF95X.COM`), and UAF95D `[200,6]` (`/FLAGS=DISUSER`, for the
  refused login), grants UAF95 the identifier UAF95_ID, and makes their
  shared home directory owned by `[200,5]`; the second removes the
  accounts and identifiers. The README gives each command, so
  the author can also make the accounts by hand.
- **By hand** (the README's table): logging in to UAF95D (DISUSER) and
  to UAF95 with a LOGIN.COM that fails, to see the messages.

## The review

Questions for the author:

1. **The hash** (Decision 4): Argon2id with `golang.org/x/crypto` as a
   new dependency, or PBKDF2-SHA256 from the standard library?
   *Recommendation:* Argon2id.
2. **SYSTEM's home** (Decision 3): the default volume's directory when
   one is configured (today's starting place), else
   `SYS$SYSROOT:[SYSMGR]`? *Recommendation:* yes, so nothing changes for
   SYSTEM.
3. **The system volume's protection** (Decision 9): (S:RWED, O:RWED,
   G:RE, W:RE), as VMS's system files, or strictly owner-only write,
   (S:RE, O:RWED, G:RE, W:RE)? *Recommendation:* VMS's.
4. **Login procedures for SYSTEM and the one-shot subcommands**
   (Decision 8): run for every interactive login (SYSTEM's too), skipped
   by one-shot subcommands, with `--no-login` to skip them in a session?
5. **Identifiers** (Decision 11): in this phase? *Recommendation:* yes;
   the ACLs and `$CHECK_ACCESS` already use them, and they are a small
   part of AUTHORIZE.
6. **The probe** (Part D): may the UAF95 account `[200,5]` be made on
   the simh VAX for the batch job?

The answers (2026-11-15):

1. Argon2id, with `golang.org/x/crypto` (Decision 4).
2. SYSTEM's home is SYS$MANAGER, as VMS's, unless the configuration sets
   `vax.default.volume.directory` (Decision 3).
3. VMS's protection for the system volume (Decision 9).
4. The one-shot subcommands skip the login procedures; no option to skip
   them in a session until one is needed (Decision 8).
5. Identifiers are in (Decision 11).
6. The probe's accounts are made by a script, `uaf95acct.com`, with
   the README giving the commands in detail too (Part D).

## Progress log

- **2026-11-15** Subtask 1: read `corevms`'s accounts (`users.go`,
  `process.go`, `sysgen.go`'s `Account`), `$GETJPI`'s items, the startup
  path (`Console.RunStartup`, STARTUP.COM), `sysvol`'s layout and
  refresh, the subprocess and LOGINOUT CLIs, MCR, CREATE/DIRECTORY's
  qualifiers, and `cmd/govax`'s options; found no AUTHORIZE manual among
  the references and no UAF definitions in `vmsdef`; wrote Part A and
  this plan.
- **2026-11-15** The review: the author accepted the plan and answered
  the six questions. Decision 3 now gives SYSTEM SYS$MANAGER as its home
  unless `vax.default.volume.directory` is configured; Decision 8 drops
  `--no-login`; Part D adds `uaf95acct.com` and `uaf95drop.com`, and
  UAF95D, a DISUSER account.
- **2026-11-15** Subtask 2, written and rehearsed: `testdata/uaf95`
  (`gen.go` writes 130 AUTHORIZE cases in `uaf95.com`, the accounts and
  the end, the state procedure, the three login procedures, and the
  exchange and copy-out scripts; the README has the VAX steps and the
  accounts in detail). The exchange volume builds on govax, and
  `TestUAF95Probe` runs the login procedures and `UAF95I.COM` there;
  `UAF95.COM` and `UAF95DROP.COM` ran on govax to their ends. The
  rehearsal found a govax bug, fixed: an OPEN failing before RMS opened
  anything (a device not mounted) ignored `/ERROR`
  (`dclshell/dclfile.go`, `TestDCLFile_openErrorBeforeRMS`). It also
  showed govax lacks SHOW PROCESS/RIGHTS and /QUOTAS, and that a DCL
  element over 255 characters (a privilege list) is TKNOVF in WRITE, which
  the VAX run will settle. Waiting for the VAX run.
- **2026-11-15** Round 1 on VMS 7.1 stopped at AUTHORIZE case 21, before
  the accounts: the case's ADD was a 504-character record, which DCL
  refused (RMS-W-RTB). It also found that AUTHORIZE ignores a
  supervisor-mode RIGHTSLIST name (SYSUAF's was honored) and used the
  VAX's rights database, leaving the identifier UAFA there, which the
  author removed. Round 2: executive-mode process names, a guard on both
  scratch files, no line over 200 characters (`gen.go` refuses one), UIC
  members in octal. Round 1's log is `vax/round1/uaf95.log`; the
  README's "Round 1" lists what it answered.
- **2026-11-15** Round 2 on VMS 7.1 ran to the end: all 134 AUTHORIZE
  cases on the scratch files (the executive-mode names honored), SYLOGIN
  and the process's state for each account in batch and at the terminal,
  DISUSER's `%LOGIN-F-DISUSER`, and every F$GETJPI item (`vax/`). The
  accounts' LOGIN.COM and LGICMD didn't run: the probe's SET SECURITY
  wrote `W:`, which VMS refuses with `%DCL-W-VALREQ`, so the procedures
  stayed closed to the accounts, and VMS skipped an unreadable login
  procedure without a message. govax took `W:` as no access; it now
  refuses it (`dcl/security.go`'s `readProtection`, with cases in
  `TestProtectionErrors`). Round 3 (`uaf95b.com`, `copyout3.cmd`) makes
  the accounts again with that fixed and UAF95's LGICMD empty, and lists
  identifiers and rights by name; waiting for the VAX.
- **2026-11-15** Round 3 on VMS 7.1 settled the login procedures: SYLOGIN,
  then the LGICMD or LOGIN.COM (never both), the account's at level 1 and
  SYLOGIN above it at level 2; a warning goes on, an error ends the
  procedure, the interactive login goes on and a batch job ends; and the
  LIST cases by name. A.5 is the three rounds' answers; Decisions 7 and 8
  now follow them. Subtask 2 is done.
- **2026-11-15** Subtask 3: `internal/uaf`, a leaf package (standard
  library, `vmsdef`, `hostlock`, and `golang.org/x/crypto/argon2`, the new
  dependency, v0.52.0, the newest whose own requirements govax's
  `golang.org/x/net` and `sys` already meet): `File` (version, accounts,
  identifiers), `Record` (the A.3 fields, `Quotas` with every AUTHORIZE
  quota, PBYTLM too, which the probe's SHOW listed), `Held`, `Identifier`;
  JSON names for a UIC (`"[1,4]"`, octal, [37776,177776] the largest),
  privileges (`["ALL"]`, or names; IMPERSONATE and ACNT written, as 7.1's
  SHOW of SYSTEM had them, both names of a bit read), flags (`$GETUAI`'s
  UAI$_FLAGS bits in order, the order SHOW lists them in, case 26),
  identifier values (`"%X80010000"`), and attributes (`$KGBDEF`'s);
  `Load`/`Parse` (unknown fields refused; every error a `ParseError` with
  the line: encoding/json's offset, or the line of the bad value's text),
  `Check`, `Save` (a temporary file and a rename, mode 0600), `Update`
  (under a lock on `sysuaf.json.lock`, since `Save` replaces the file
  itself; `hostlock.Wait`, new, takes a lock that waits), `Ensure`,
  `Default` (SYSTEM with process 1's values, `SystemRecord`; DEFAULT with
  7.1's, `DefaultRecord`; SYSTEM's UIC identifier), and `HashPassword`/
  `CheckPassword` (Argon2id's PHC string, RFC 9106's second option; the
  password uppercased and trimmed, as VMS's aren't case-sensitive). There
  is no "next free identifier value" stored: `NextGeneral` computes it from
  the file, starting at %X80010000 (case 76 gave PROJX that value).
  `corevms`: the System's `authorization` (`authorization.go`: a host file
  read again only when its size or time changes, or one in memory starting
  as `uaf.Default`'s) replaces the `users` and `identifiers` tables; `User`,
  `AddUser` (a new account is DEFAULT's without DISUSER, and gets a UIC
  identifier unless its name or UIC has one, as ADD does; DEFAULT has none,
  as on 7.1, where case 12's ADD took [200,200]), `AddIdentifier`, and the
  rights database read it; SYSTEM is still found when the file has lost it.
  `Account` is gone: `SystemAccount` is `uaf.SystemRecord().Quotas`, and
  `LoginQuotas` gives a process every quota govax has from the record
  (ASTLM, BIOLM, DIOLM, and the job's PRCLM and pooled quotas too), with
  `TestSystemAccountIsProcessOne` holding them to `NewProcess`'s and
  `newJob`'s. `machine.Machine.AuthorizationFile` reaches each new System;
  `cmd/govax` makes the file before `Init` (`authorization.go`;
  `vax.authorization.file`, default `~/.govax/sysuaf.json`, in
  `validConfigs` and HELP CONFIGURATION KEYS), a file it can't read
  stopping startup; `TestMain` gives the package's tests a temporary file.
  Nothing a process gets has changed: login from the record is subtask 4.
- **2026-11-15** Subtask 4: `corevms/login.go`. `LoginRecord` is
  LOGINOUT's check (`%LOGIN-F-NOSUCHUSER`, `%LOGIN-F-DISUSER, account is
  disabled`, `%LOGIN-F-CLIFAIL` for a CLI not DCL; the LGI$_ statuses from
  `vmsdef.LibrarySymbols`; NOSUCHUSER's and CLIFAIL's texts unconfirmed);
  `System.BeginLogin` reads the file, resolves the identifiers held, takes
  the time, and writes it back as the last interactive or non-interactive
  login, returning a `Login` (the record as it was before, so its times are
  the logins before this one); `Environment.LogIn` gives a process a Login:
  username and process name, account, UIC and the logical names' UIC
  (`SetUIC`), authorized privileges and the defaults as process and
  current, priority, `LoginQuotas`, the rights (INTERACTIVE and LOCAL, or
  BATCH alone as the probe's batch jobs had, then the identifiers held),
  LOGINTIM, and `Process.LastLogin` (`LastLoginText` is the banner's
  `    Last interactive login on Sunday, 15-NOV-2026 17:19` lines, one per
  kind that has happened; a day under 10 is written with its zero,
  unconfirmed). `machine.Machine.LogIn` keeps the Login and gives it to each
  new process 1 a VMINIT makes, recording no second login
  (`TestLogInSurvivesVMInit`). `dclshell/dcllogin.go`'s
  `SetLoginDirectory` defines SYS$LOGIN, SYS$LOGIN_DEVICE, and SYS$SCRATCH
  in the job table at executive mode, the record's text untranslated as on
  7.1, sets the default directory, and replaces SET DEFAULT's
  supervisor-mode SYS$DISK with an executive-mode one. `console.LogIn` picks
  the home: the record's; for SYSTEM the configured default volume's
  directory when `vax.default.volume.directory` is set, and when the home
  isn't there (no system volume) the default as it was, SYS$LOGIN then
  naming the volume's directory if the default is on one and nothing if
  it's the host's. `cmd/govax`: `--user NAME`/`-u` (`authorization.go`'s
  `loginUser`, `checkLogin` before anything runs, exit status 1 with
  LOGINOUT's message), and the login after STARTUP.COM and the page file,
  before `vax.startup.procedure`, in every session and one-shot command;
  the banner only at a terminal (`Console.Verbose`). F$GETJPI's
  LAST_LOGIN_I, LAST_LOGIN_N (17-NOV-1858 for none), LOGIN_FAILURES and
  LOGIN_FLAGS (0: no passwords asked), MAXDETACH, MAXJOBS, SHRFILLM, and
  UAF_FLAGS are answered by name (`getjpi.go`'s `jpiNamedItems`): their
  `$JPIDEF` codes are in no source govax may read, so `$GETJPI` by code
  still refuses them, BADPARAM, until a probe round assembles `$JPIDEF
  GLOBAL` on VMS. `TestLogIn`, `TestLoginItems`, `TestRun_logIn` (SYSTEM's
  home on the system volume, the names' modes and tables, `--user SMITH`'s
  process and its recorded login, DISUSER and a missing account refused).
  Behavior change, as Decision 3 planned: with a system volume and no
  `vax.default.volume.directory` configured, a session now starts in
  SYS$SYSROOT:[SYSMGR] rather than the default volume's `[WORK]`.
- **2026-11-15** Subtask 5: `internal/authorize`, AUTHORIZE's account
  commands. `authorize.dcl` (bootdata) is its grammar, parsed apart from
  `console.dcl` as `debug.dcl` is: EXIT, ADD, COPY, MODIFY, DEFAULT,
  REMOVE, RENAME, every account qualifier govax keeps a value `$any` that
  the package reads itself (UICERR, BADVALUE, VALTOOBIG; flag and privilege
  names CLI's IVKEYW), the A.3 qualifiers it doesn't keep left out, so
  IVQUAL. A `Utility` is one run (`Run`): a Store (corevms's System, its
  `Authorization` and `UpdateAuthorization`), the process's credentials,
  SYS$OUTPUT, and a reader for a session's commands. One command prints
  only its messages and returns its $STATUS, each command's as VMS 7.1's
  (A.5: UAEERR's own condition in ADD and COPY, SS$_NORMAL in RENAME;
  BADUSR %X10000000 in COPY, SS$_NORMAL in RENAME and REMOVE; BADSPC's own
  in MODIFY; CLI's conditions with the inhibit bit); a session prompts
  `UAF> `, ends at EXIT (abbreviated) or the end of input, and says DONEMSG
  or NOMODS and RDBDONEMSG or RDBNOMODS by what its commands changed. The
  UAF messages are `vmsdef.UAFMessages` (the facility, %X3F1, and the three
  codes the probe showed). The file's profile is SYSUAF.DAT's, owner [1,4],
  (S:RWE,O:RWE,G,W), checked by `security.Check`: read at a session's start
  or for one command, write for each command that changes the file; refused,
  NAOFIL with RMS's PRV, SS$_NORMAL (govax's file is always found, so never
  VMS's FNF from a default directory without SYSUAF.DAT). DCL:
  `dclshell/authorize.go`'s `Interpreter.Authorize`, the `AUTHORIZE
  [command]` verb (console.dcl, bound in `console/commands.go`), and
  `builtinUtility`: MCR (and a foreign command) of AUTHORIZE or
  SYS$SYSTEM:AUTHORIZE[.EXE], RUN only of the latter; a session reads as
  HELP's responses do (`ReadHelpResponse`). HELP AUTHORIZE is a short
  topic for now (subtask 10 writes it out, and UAFHELP.HLB).
  `TestUAF95Oracle` replays the probe's 134 one-command cases in order on a
  file of govax's own, comparing the 54 that are this subtask's (SHOW,
  LIST, and the identifier commands skipped until subtask 6) line for line
  with their $STATUS, all VMS's but cases 123 and 125 (/ACCESS and
  /NOACCESS, IVQUAL by A.3); `TestUAF95FinalState` holds the file they
  leave to VMS's LIST (case 119), case 134's fields, and the identifiers
  SHOW/IDENTIFIER found. Unconfirmed choices, which no case settled: a
  text value too long is VALTOOBIG (owner 31, account 8, device, CLI, and
  tables 31, directory and LGICMD 63), QUEPRIO's largest 255; /PRIVILEGES
  and /FLAGS change the bits they name, NO clearing them (VMS's SHOW of
  UAFD in case 46 listed no privileges at all, its brief listing "System");
  MODIFY's CLIWARNMSG after MDFYMSG (ADD's came after ADDMSG); /ACCOUNT
  makes a group identifier only on ADD, and only when the group has none
  and the name is a free identifier name; COPY's new account holds no
  identifiers and has no password unless /PASSWORD gives one (then no
  DEFPWD); RENAME's new name already an identifier's is RDBMDFYERR with
  DUPLNAM; REMOVE takes out only the UIC identifier named for the account,
  and every account's grant of it. Also fixed: subtask 4 left
  `TestNoUncheckedPrivileges` failing, `corevms/login.go`'s `LogIn` setting
  the current privileges unlisted; it is now among `managedPrivileges`.
- **2026-11-15** By the author's choice, `vax.startup.procedure` is now
  part of the system's startup: `cmd/govax` runs it after STARTUP.COM and
  the page file and before process 1 logs in, so it runs as SYSTEM and a
  MOUNT or another privileged command in it works whatever `--user` names
  (it had run after the login, as the account). Two consequences: the
  login's default directory (SYS$LOGIN) now replaces any SET DEFAULT the
  procedure does, and the last-login banner follows the procedure's
  output. `TestRun_startupProcedureAsSystem`; HELP STARTUP, HELP
  CONFIGURATION KEYS, and MANUAL.md's startup sequence say so, with the
  login as its own step.
- **2026-11-15** Subtask 6: AUTHORIZE's reports and identifiers.
  The rights database's model first: on VMS a holder record is a UIC's,
  not an account's (case 97 listed holders UAFD and UAFA, both granted by
  their UIC identifiers' names, and GRANTUSR says "user identifier ...
  does not exist"), so `uaf.Identifier` now carries its `Holders` (a UIC
  and attributes, in grant order) and a `Record` holds nothing itself;
  `File.HeldBy(uic)` is what an account holds, so every account with the
  UIC holds it, and `MoveHolder` and `DropHolder` are the System Services
  manual's $MOD_IDENT and $REM_IDENT rules (a UIC identifier's new value
  takes its holder records along; removing it drops them and its UIC's
  own). A file written before this keeps its accounts' lists under
  `"identifiers"` (`Record.Legacy`), which `Parse` moves to holder
  records and never writes again. corevms's login (`heldRights`), `User`,
  and `AddUser` read and write holders; `AddUser` of a right no identifier
  in the file has is now SS$_NOSUCHID. `uaf.Environmental` is the eight
  identifiers 7.1's CREATE/RIGHTS made (DECWINDOWS and
  VMS$BUFFER_OBJECT_USER beside corevms's six), which AUTHORIZE lists but
  doesn't keep in the file. In `authorize.dcl` each identifier command is
  its verb's `/IDENTIFIER` (or SHOW's and LIST's `/RIGHTS`) switching to a
  syntax of its own, so SHOW/RIGHTS/FULL is IVQUAL (case 96); new verbs
  SHOW, LIST, GRANT, REVOKE, and CREATE (`/RIGHTS`, accepted, nothing done).
  `internal/authorize`: `report.go` lays out SHOW's full report (columns
  measured from cases 1, 2, 26, 98; the UIC named by its identifiers,
  `[CASED,UAFD]`, `[UAFA]`, or the numbers again; privileges sorted with
  IMPERSONATE in DETACH's place, cut to ten characters; the identifiers
  held under /FULL) and the brief line (the comma of the UIC in column 41;
  the Guide to System Security's Table 5-5 categories, the highest shown);
  `show.go` is SHOW, SHOW/IDENTIFIER (with holders under /FULL; `*` alone
  is every identifier, `PROJ*` IVIDENT), SHOW/RIGHTS (always CLI$_ABSENT),
  and LIST, LIST/IDENTIFIER (always with holders), and LIST/RIGHTS (RMS$_NORMAL
  every time, LSTERR for a wildcard), the files written by the Utility's
  `WriteListing`, which DCL gives as a text file in the default directory
  (`dclshell/authorize.go`); `identifiers.go` is ADD, REMOVE, RENAME, and
  MODIFY/IDENTIFIER, GRANT and REVOKE (a holder named by its UIC
  identifier; the holder's attributes only those the identifier has,
  $ADD_HOLDER's rule), and `/VALUE` (IDENTIFIER=n from 65,536 to
  268,435,455 with bit 31 set by AUTHORIZE, BADVALUE showing a number out of
  range as a signed longword, case 77; UIC=[g,*] the group's identifier).
  Held to the probe: `TestUAF95Oracle` now runs all 134 round 2 cases,
  comparing 128 (and LIST's four files) with VMS's; the six it doesn't are
  SYSTEM's own record (Decision 3, cases 2 and 4) and UAFO, which A.3's
  refused qualifiers never added (123 to 126); SYSTEM's UIC identifier,
  which govax's new file has and 7.1's CREATE/RIGHTS didn't make, is taken
  out of SHOW/IDENTIFIER * before comparing, and so are the full report's
  password minimum, lifetime, and change (govax keeps none, and ADD leaves
  no pre-expired password). `TestUAF95Round3Oracle` is round 3's 13 cases
  and five listing files; `TestSession` the probe's session with its SHOWs;
  `TestHoldersByUIC`, `TestPrivilegeCategory`; the console's
  `TestAuthorizeListAndRights` (LIST on a volume, a granted identifier in a
  login's rights). Unconfirmed choices: what 7.1's brief line says for the
  Group, Devour, and Objects categories, and "None" (Table 5-5's words);
  an owner over 20 characters cut there; the full report's flag words
  beyond the seven case 26 showed; CPU time shown without hundredths;
  "(none)" as the password change of an account without one; LIST of a
  username matching nothing writing an empty file; SHOW/IDENTIFIER and
  SHOW/RIGHTS with no name NOIDNAME, as LIST's are; SHOW/RIGHTS of a name
  that is no UIC identifier SHOWERR with NOSUCHID; a holder with no UIC
  identifier shown by its numbers; holders listed in grant order and an
  account's rights in the file's order of identifiers; ADD/IDENTIFIER of a
  value in use DUPIDENT; RENAME/IDENTIFIER's success RDBMDFYMSG with the
  old name; MODIFY/IDENTIFIER's bad value after the lookup (case 105 found
  the missing identifier first); GRANT of an identifier the UIC holds
  checked after the identifier and the holder; REVOKE of a holder or
  identifier not found REVOKEERR with NOSUCHID; the environmental
  identifiers refused to REMOVE, RENAME, MODIFY, and GRANT with NOPRIV.
  HELP AUTHORIZE lists the new commands.
- **2026-11-15** By the author's choice, after subtask 6: logging in to an
  account with a password asks for it (Decision 6's "The password"),
  `GOVAX_PASSWORD` giving it when standard input isn't a terminal and a
  graceful failure otherwise; it is part of subtask 8. Subtask 8's line no
  longer names `--no-login`, which the review dropped.
- **2026-11-15** Subtask 7: the system volume's protection.
  `sysvol/profile.go`: a `Profile` (owner and protection code),
  `FileProfile` ([1,4], (S:RWED,O:RWED,G:RE,W:RE)) and `DirectoryProfile`
  ((S:RWE,O:RWE,G:RE,W:RE)), `File.Profile`, and `DirectoryFile` (a
  directory's directory file, `[SYSTEM]SYSLIB.DIR`, `[000000]SYSTEM.DIR`).
  The `Volume` interface gains `SetProfile` (every version of a file);
  `dclcmd`'s is SET SECURITY's own code, `rms.Session.SecurityObjects` and
  `SetOwnerProtection`, run as SYSTEM before any login. `Refresh` sets each
  directory's and file's profile and records it in the manifest, two words
  at a record's end (`OWNER=[1,4] PROTECTION=(...)`; a directory's record,
  `[SYSTEM.SYSLIB]`, holds nothing else): a profile the manifest already
  records isn't set again, so an older volume is corrected once and a
  later SET SECURITY stays; a new version takes its previous version's
  protection (RMS's rule), so an update keeps it, and only a file made
  where there was none (a missing supplied file, a seeded one's first
  writing) gets govax's again. The manifest gets its own profile when it
  is first written with profiles; the update log when DCL makes it (the
  refresh before records that profile, so the start after doesn't write a
  manifest for it), or at the next refresh if it is there unrecorded. The page file keeps rms's
  (S:RWED,O:RWED,G,W), and Decision 9's W:RE for SYS$MANAGER's procedures
  is every file's. SYLOGIN.COM (`bootdata/files/sylogin.com`, comments
  only) is a second seeded file in SYS$MANAGER, and STARTUP.COM defines
  `SYS$SYLOGIN` as VMS 7.1's was (system table, executive mode,
  `"SYS$MANAGER:SYLOGIN"`, as `vax/uaf95.log` shows it); nothing runs it until subtask 8.
  Tests: `sysvol`'s `TestProfiles`, `TestManifestProfiles`,
  `TestPopulateProfiles`, `TestRefreshProfiles` (the map volume keeps a
  profile per version and inherits it); the console's
  `TestSystemVolumeProtection` (every file and directory on a new volume;
  DIRECTORY/SECURITY), `TestSystemVolumeProtectionCorrected` (an older
  volume, its manifest without profiles, corrected at the next start, the
  user's change kept at the one after), and `TestSystemVolumeAsUser`
  (SMITH, [300,1], reads SYS$SYLOGIN's file; DELETE, CREATE, COPY, and SET
  SECURITY in the system's directories are RMS-E-PRV or SYSTEM-F-NOPRIV).
  MANUAL.md's "Who owns the system's files", HELP SYSTEM_VOLUME
  PROTECTION, and VAX-VMS-INTERNALS.md's refresh say so. Noticed, not
  changed: CREATE's and COPY's OPENOUT name a new file on the system volume
  by its physical directory (`DUA0:[SYSTEM.SYSMGR]MINE.COM;`) where DELETE
  shows the concealed `SYS$SYSROOT:[SYSEXE]`.
- **2026-11-15** Subtask 8: the login procedures and the password.
  `dclshell/dclloginproc.go`'s `RunLoginProcedures`: the account's
  procedure (its LGICMD, or `SYS$LOGIN:LOGIN.COM` when it has none and
  SYS$LOGIN is defined) opened first and run at level 1, SYLOGIN run above
  it at level 2 before its first line (`procedureCommand.before`, called
  by `runLevel` once the level is there), or at level 1 when the account's
  can't be opened; a procedure missing or unreadable passed over with no
  message; verify off; each with DCL's default ON action, the login going
  on. SYLOGIN is opened as `SYS$SYLOGIN:`, and only when the name is
  defined. `cmd/govax` runs them after the banner in a session, never for
  a one-shot command. Ctrl/Y in one is DCL's Ctrl/Y level as in any
  procedure: CONTINUE goes on, EXIT ends the login procedures (EXIT there
  ends every procedure, the User's Manual's 13.7.1). The password:
  `cmd/govax/authorization.go`'s `checkPassword`, after `checkLogin`'s
  record check: an account with a password is asked `Password: ` with echo
  off (`golang.org/x/term`'s `ReadPassword`) three times at a terminal,
  each failure `User authorization failure`; elsewhere `GOVAX_PASSWORD`
  gives it, and without that govax stops with `%GOVAX-F-PASSWORD, account
  SMITH has a password; ...` (its text govax's own); both exit 1. Tests:
  `TestUAF95LoginOracle` holds both accounts' logins to round 3's
  interactive logs (every symbol the procedures recorded, and UAF95X's
  IVVERB and OPENIN messages as `terminal.log` has them),
  `TestLoginProceduresSkipped`, `TestLoginProceduresControlY`,
  `cmd/govax`'s `TestRun_loginProcedures` (a session's, not a one-shot
  command's), `TestRun_password`, and `TestCheckPassword_terminal`.
  Fixed on the way: `@` of a logical name with a colon and nothing after
  it (`@SYS$SYLOGIN:`) added `.COM` to the text and then refused a type on
  both sides of the translation; `openProcedure` now takes the default
  type as RMS's default name for such a name (`Session.Parse` with
  `.COM`). Noticed, not changed: govax's RMS doesn't translate a file
  name alone as a logical name (`@SYS$SYLOGIN` looks for
  `SYS$SYLOGIN.COM`), which VMS's RMS does for a name with no
  punctuation.
- **2026-11-15** Subtask 9: ADDUSER.COM. `bootdata/files/adduser.com`,
  supplied (not seeded) in SYS$MANAGER (`sysvol.Files`): it refuses a
  process without SYSPRV outside the system groups
  (`vax.sysgen.maxsysgroup`, 8 unless set) before asking anything,
  `%ADDUSER-E-NOPRIV`; asks the username (12 characters at most, not an
  identifier already, `F$IDENTIFIER`'s NAME_TO_NUMBER), the owner, the UIC
  group in octal (200 offered) and member (the first the rights database
  has no identifier for, found by NUMBER_TO_NAME, offered; one in use
  refused by `F$FAO("!%I")`), the password twice with SET
  TERMINAL/NOECHO (none allowed), the home device (the default volume's
  when one is configured and mounted, else SYS$SYSDEVICE:) and directory
  (`[username]`), and the privileges (TMPMBX and NETMBX, or a list); runs
  `AUTHORIZE ADD .../FLAGS=NODISUSER/PRIVILEGES=(NOALL,...)` (NOALL, since
  AUTHORIZE adds named privileges to DEFAULT's); makes the home directory
  owned by the account, (S:RWE,O:RWE,G,W), or gives an existing one to it
  (its directory file, `[A]B.DIR` for `[A.B]`); and writes a starter
  LOGIN.COM there, the account's, (S:RWED,O:RWED,G,W), unless one is there.
  Ctrl/Z or Ctrl/Y at a question adds nothing. The protection is written
  `G,W` rather than Decision 12's `G:, W:`, which DCL now refuses (round
  2's lesson). SET TERMINAL/NOECHO now does something at the host: DCL's
  terminal reads in process 1 (`readTerminal`: INQUIRE, READ SYS$COMMAND)
  call `Interpreter.TerminalEcho` while SYS$COMMAND has TT$M_NOECHO, which
  `cmd/govax` sets to a termios change at a real terminal
  (`terminal_unix.go`'s `terminalEcho`: ECHO off, ECHONL on, put back
  after the read; nil elsewhere). Tests: `TestAddUser` (SYSTEM runs it,
  answering at the terminal; the record, the hash, the directory's and
  LOGIN.COM's owner and protection; then process 1 logged in as SMITH: its
  F$GETJPI values, its home, its LOGIN.COM run, SYS$MANAGER readable and
  not deletable, its home writable, ADDUSER refused),
  `TestAddUserExistingHome`, and `TestTerminalNoEcho`. Noticed, not changed:
  in a `cmd/govax` session whose input is a pipe, readline and DCL's
  terminal reads share one reader and readline reads ahead, so an INQUIRE
  gets end of file; at a terminal they share `attentionStdin` and work, so
  the end-to-end test is in `internal/console`, where the terminal is a
  plain reader.
