# Phase 95 — authentication

**Status:** planned 2026-11-15 (subtask 1, the survey and this plan),
awaiting the author's review. This document is the development plan; as
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
`SYS$LOGIN:LOGIN.COM`). Its DEVICE and DIRECTORY are the default
volume's when one is configured (`vax.default.volume.device` and
`.directory`, today's starting place), else `SYS$SYSROOT:` and
`[SYSMGR]`, VMS's SYSTEM home. The `PQL_D*` configuration keeps laying
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

**Decision 7 — logging in, one routine.** `corevms.Environment.LogIn(rec
uaf.Record)` does what LOGINOUT does for a process, and is the one place
an account becomes a process: username, account, owner, UIC (and the
logical name database's, `SetUIC`), authorized privileges, process and
current privileges from DEFPRIVILEGES, priority and authorized
priority, the quotas govax has (`LoginQuotas` taking the record), the
rights (the record's identifiers plus the environmental ones),
LOGINTIM, and the record's last-login time written back. DCL's side,
`dclshell`, defines `SYS$LOGIN`, `SYS$LOGIN_DEVICE`, and `SYS$SCRATCH`
(job table, executive mode, unless the probe says otherwise), sets the
default directory to them, and runs the login procedures (Decision 8).
`$CREPRC` of LOGINOUT and LIB$SPAWN keep today's rules (a subprocess
takes its owner's identity); a detached LOGINOUT process looked up by
username waits for the remote-login phase.

**Decision 8 — the login procedures.** After STARTUP.COM (and
SYSTARTUP_VMS.COM), process 1's DCL runs `SYS$SYLOGIN` if that name is
defined and the file exists, else `SYS$MANAGER:SYLOGIN.COM` if it
exists; then the account's LGICMD, or `SYS$LOGIN:LOGIN.COM`, if it
exists. Each at level 1, as a login procedure, its failures its own;
Ctrl/Y in one ends it. STARTUP.COM defines `SYS$SYLOGIN` as VMS's
does. They run for every login, SYSTEM's too, in an interactive
session; the one-shot subcommands skip them (they run one command, and
their output must stay predictable), and `--no-login` (Decision 6's
sibling) skips them in a session. The system volume supplies a
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
2. **The probe**: `testdata/uaf95`, as in Part D, written and rehearsed
   where govax can; run on the VAX by the author; A.4 answered and Part
   B adjusted.
3. **The authorization file** (Decisions 1 to 5): `internal/uaf`
   (records, identifiers, JSON, locking, the hash, validation, the
   default file), `vax.authorization.file`, the file made at startup;
   `corevms`'s user table and `SystemAccount` onto it, with no change in
   what process 1 gets.
4. **Logging in** (Decisions 6 and 7): `--user`/`-u`, `Environment.LogIn`,
   SYS$LOGIN and its kin, the default directory, DISUSER's refusal, the
   new `$GETJPI` and F$GETJPI items the probe lists, LOGINTIM and the
   last-login time.
5. **AUTHORIZE's account commands** (Decision 10): `authorize.dcl`,
   `internal/authorize`, the `UAF>` session and the one-command form, MCR
   and RUN; ADD, COPY, DEFAULT, MODIFY, REMOVE, RENAME, EXIT; the access
   check (Decision 5); the UAF messages; held to the probe.
6. **AUTHORIZE's reports and identifiers** (Decision 11): SHOW
   (`/BRIEF`, `/FULL`, wildcards), LIST (SYSUAF.LIS, RIGHTSLIST.LIS),
   the identifier commands, rights given at login; held to the probe.
7. **The system volume's protection** (Decision 9): owners and
   protections in `sysvol`, set by Populate and Refresh, existing
   volumes corrected once; SYLOGIN.COM seeded; `SYS$SYLOGIN` in
   STARTUP.COM.
8. **The login procedures** (Decision 8): SYLOGIN then LGICMD or
   LOGIN.COM, `--no-login`, their order, failures, and Ctrl/Y, held to
   the probe's batch log.
9. **ADDUSER.COM** (Decision 12), with SET TERMINAL/NOECHO if missing,
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
- **`uaf95j.com`**, login's side: a batch job submitted `/USER=UAF95`
  (the probe account, `[200,5]`, made by the author for the run, with
  SYLOGIN and LOGIN.COM of the probe's own reached through its LGICMD):
  SHOW LOGICAL of SYS$LOGIN, SYS$LOGIN_DEVICE, SYS$SCRATCH, and
  SYS$SYLOGIN with /FULL, SHOW DEFAULT, F$MODE, the order the
  procedures ran in, and F$GETJPI of every item code for the job.
- **By hand** (the README's table): logging in to a DISUSER account
  and to one whose LOGIN.COM fails, to see the messages.

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

## Progress log

- **2026-11-15** Subtask 1: read `corevms`'s accounts (`users.go`,
  `process.go`, `sysgen.go`'s `Account`), `$GETJPI`'s items, the startup
  path (`Console.RunStartup`, STARTUP.COM), `sysvol`'s layout and
  refresh, the subprocess and LOGINOUT CLIs, MCR, CREATE/DIRECTORY's
  qualifiers, and `cmd/govax`'s options; found no AUTHORIZE manual among
  the references and no UAF definitions in `vmsdef`; wrote Part A and
  this plan.
