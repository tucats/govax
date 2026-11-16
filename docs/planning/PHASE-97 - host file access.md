# Phase 97 — host file access

**Status:** planned 2026-11-16 (subtask 1, the survey and this plan);
subtasks 2 and 3 done 2026-11-16.
The author reviewed it and accepted it the same day, with the answers
recorded in "The review". This document is the development plan; as
the work goes on it becomes the implementation record: the progress log,
and at the end "Where things stand".

## Goal

A site can stop unprivileged users and their programs from reaching the
host's files through govax. A new configuration key,
**`vax.host.sysprv`** (boolean), turns the rule on:

- **Off** (the key missing or false, the default): govax behaves exactly
  as it always has. No check is made and nothing is traced.
- **On**: every access to a host file, whether the name is a host path by
  its syntax (`/tmp/x.txt`, `C:x`), a bare name taken as a host file
  because the default directory is on the host, a logical name that
  translates to one, or a name with govax's `/HOST` qualifier, is allowed
  only to a process whose current privileges include **SYSPRV**. Any
  other process is refused, as a volume file's protection refuses it
  (Decision 5).

The session below is what the plan expects, as a user without SYSPRV
with the key on (the messages are those each command already gives for
a protection refusal, Phase 93):

    $ TYPE /tmp/notes.txt
    %TYPE-W-OPENIN, error opening /tmp/notes.txt as input
    -RMS-E-PRV, insufficient privilege or file protection violation
    $ COPY DUA1:[WORK]A.TXT a.txt/HOST
    %COPY-E-OPENOUT, error opening a.txt as output
    -RMS-E-PRV, insufficient privilege or file protection violation

and as SYSTEM, which holds SYSPRV, nothing changes.

This is govax's own rule. VMS has no host, so there is nothing to probe;
SYSPRV is chosen because it is VMS's privilege for "access to objects as
the system", the one a site would expect to guard files outside the
system's own protection.

## Part A — the survey (2026-11-16)

### A.1 How a name becomes a host file

`rms.Session.Locate` (`internal/rms/location.go`) is the one place a
typed file name is placed on the host or on a mounted volume, by four
rules: an explicit `/HOST`; VMS syntax (a bracketed directory, a device
or logical name of two letters or more) on a volume; host syntax (`/`,
`\`, a one-letter drive) on the host; and a bare name on the default
volume when SET DEFAULT is on one, else on the host. Before them, a name
with no punctuation that is a logical name stands for its translation
(`locateLogical`). The result is a `FileLocation` with `Host` set.

Each process has its own `rms.Session` (DCL's `Shell.ContainerSession`,
`corevms.Environment.Session`), and `Environment.UseSession` gives it
the process's `Credentials`, so the session already knows whose it is
(Phase 93). Its `HostFallback` reads a bare host name not in the host's
current directory through the console's search-path resolver
(`respath.Resolver`: `--path` directories on the host, the system
volume, then govax's embedded files; `ReadFileFrom` says which).

### A.2 Who calls Locate

55 calls outside `location.go`. Most open the file they place; a few only
ask which side a name is on and then do something else:

| Area | Callers |
| --- | --- |
| DCL utilities (`internal/dclcmd`) | TYPE, COPY (source and destination), DUMP, DIRECTORY (`/OUTPUT`), CREATE (file, `/FDL`), CONVERT (and its exceptions file), BACKUP (save, restore, list: input, output, save set), MACRO, FORTRAN (sources and INCLUDE logicals), LINK (objects, options), LIBRARY (library, inputs), LNK$LIBRARY's user libraries, ANALYZE, HELP (`/LIBRARY`, `/OUTPUT`), INSTALL, `filenames.go`'s output names |
| DCL itself (`internal/dclshell`) | `@file`, the procedure's `/OUTPUT`, OPEN, the lexical functions (F$SEARCH, F$FILE_ATTRIBUTES, F$PARSE), RUN in a subprocess (one call only asks the side, `subcli.go:1006`), DBG$INPUT, AUTHORIZE's image name, SPAWN/HOST's `/INPUT` and `/OUTPUT` |
| The machine | RUN's image (`image.go`, `imagesource.go`, with `/HOST` as `runHost`), ASM and LOAD (`consolefile.go`), the debugger's source lines (`source.go`) |
| Programs | LBR$OPEN (`lbrrtl/control.go`), a created process's SYS$INPUT, SYS$OUTPUT, and SYS$ERROR (`corevms/ppfstart.go`, `outfile.go`, `cliprocess.go`) |
| RMS | EDIT (`rms/edit.go`), CONVERT, ANALYZE/RMS_FILE, F$FILE_ATTRIBUTES, SET and SHOW SECURITY (`secobject.go`), the page file (`pagefile.go`, from configuration) |

### A.3 Host access that doesn't go through Locate

| Path | Where | What it touches |
| --- | --- | --- |
| EXE$OPEN | `corevms/file.go`, XFC shim 4 | a host file a VAX program names, opened with `os.OpenFile` |
| SAVE/ROM, SAVE/NVRAM | `machine/rom.go` | `os.Create` of a host path |
| LOAD/ROM, LOAD/NVRAM | `machine/rom.go` through `readConsoleFile` | Locate, then the resolver |
| The debugger's SET LOG, SET OUTPUT/LOG | `debugger/dbglog.go`, `Console.CreateDebuggerLog` | the log file |
| Shareable images | `rms/shareable.go`'s `ImageLocator`, step 3 | SET SHARE_PREFIX and `vax.library` on the host |
| The resolver's host directories | `HostFallback`, `consolefile.go` | `--path` directories |
| SPAWN/HOST | `dclshell/spawnhost.go` | runs a host program with `exec` |
| MOUNT, INITIALIZE/CONTAINER, DEFINE/DEVICE | `dclcmd/mount.go`, `init_container.go`, `device.go` | a container file on the host (MOUNT already needs SYSNAM, INITIALIZE of an existing one VOLPRO) |
| EDIT | `dclcmd/edit.go`, `rms/edit.go` | a host editor on a scratch copy in `vax.editor.scratch` |

### A.4 What govax reaches on the host for itself

These are named by configuration or the command line, not by a user's
file name, and are read or written for the system: the authorization
file (`vax.authorization.file`), the system volume and `--mount`
containers, `vax.default.volume.file`, `vax.library` for STARLET.MLB,
IMAGELIB.OLB, and STARLET.OLB, a configured page file
(`vax.sysgen.paging.file`), host locks, grammars, profiles
(`--cpu-profile`), and the editor's scratch copy. govax's embedded files
and the system volume aren't host files at all.

### A.5 The privilege gate

Phase 94's gate (`corevms/privrules.go`): a `PrivilegeRule` per guarded
operation, asked through `requirePrivilege` (the caller's mode),
`requirePrivilegeAt`, or for DCL's own commands `CommandPrivilege`
(supervisor mode), each tracing under SET DEBUG PRIVILEGES.
`TestNoUncheckedPrivileges` fails on code that reads the current
privileges itself, and `TestEveryPrivilegeRuleUsed` on a rule nothing
names. `rms` can't import `corevms`, so RMS reaches the gate through a
function the process sets on its session, as it reaches the process's
credentials.

## Part B — decisions

1. **The key.** `vax.host.sysprv`, a boolean in `cmd/govax`'s
   `validConfigs` and HELP CONFIGURATION KEYS, read once at startup into
   the `corevms.System` (`HostRequiresSYSPRV`). Off, the gate isn't
   asked, so nothing is traced and every path behaves as before.

2. **What counts as a host file.** A file a process names, or a program
   a process runs, that ends up on the host: every Locate result with
   `Host` set; EXE$OPEN; SAVE and LOAD of ROM and NVRAM; the debugger's
   log; a container file named in MOUNT, INITIALIZE/CONTAINER, or
   DEFINE/DEVICE (beside the privileges they already need); and
   SPAWN/HOST, which needs SYSPRV to run at all. Not counted (A.4, and
   the review's answers 3 and 4): what govax opens for itself by
   configuration or command line, files found in directories the
   configuration names (`--path`, `vax.library`, SET SHARE_PREFIX), the
   system volume, the embedded files, and EDIT's host editor and scratch
   copy. EDIT of a file named as a host file is still checked, through
   Locate, like any other host file name.

3. **One rule, one gate.** A `PrivilegeRule`, `ruleHostFile` ("host
   file: open, create, or run"), SYSPRV, no exempt modes, SS$_NOPRIV,
   in `privrules.go` and listed in `commandRules` as `HOST_FILE` (for
   SPAWN/HOST and the container commands through `CommandPrivilege`).
   `Environment.HostAllowed()` asks it from the caller's mode (0 when the
   key is off, without asking, so nothing is traced) and is the way in
   for every other path. `UseSession` sets `rms.Session.HostAccess`, a
   `func() bool` over it, beside `Credentials`.

4. **Checked in Locate.** Locate asks `HostAccess` before returning a
   host location, and a refusal is an `*rms.DeniedError` with a new
   `Host` field set and `Name` the host path, so every command's
   existing `IsDenied` path shows it as a protection refusal. A caller that only asks which side a name is on uses a new
   `Session.Side` (Locate without the check), so a question isn't
   refused; each of A.2's callers is audited in subtask 3 and said to be
   one or the other.

5. **Messages and statuses.** A refused access is shown as the command
   shows a protection refusal on a volume file (Phase 93's
   `rms.IsDenied` paths: its OPENIN, OPENOUT, or the like, then
   `-RMS-E-PRV`), with the same `$STATUS`. Where a command has no such
   path (SAVE/ROM, SPAWN/HOST, the debugger's log), `%SYSTEM-F-NOPRIV`,
   SS$_NOPRIV, the rule's status. EXE$OPEN returns -1, as for a failed
   open. A program's RMS paths (LBR$OPEN, a created process's files)
   return RMS$_PRV.

6. **Whose privilege, and when.** The privileges of the process making
   the access, at that moment: SET PROCESS/PRIVILEGES=NOSYSPRV counts at
   once, an installed image with SYSPRV in its privileges has it while it
   runs, a subprocess or detached process uses its own. A file opened
   before the privilege was turned off stays open (a DCL OPEN, a
   procedure being run); only new accesses are checked.

7. **Who is affected.** SYSTEM, the default login, holds SYSPRV, so
   STARTUP.COM and a session without `--user` are unchanged. The one-shot
   subcommands (`govax macro`, `govax run`, ...) run as the logged-in
   user and follow the same rule.

8. **A test that finds new paths.** `TestNoUncheckedHostAccess`, like
   Phase 93's `TestNoUncheckedAccess`: outside a named list of files
   (A.4's, and the gate's own), no package under `internal` may call
   `os.Open`, `os.OpenFile`, `os.Create`, `os.ReadFile`, `os.WriteFile`,
   or `exec.Command`.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The key and the gate** (done 2026-11-16) (Decisions 1, 3, 4): `validConfigs`, HELP
   CONFIGURATION KEYS, `System.HostRequiresSYSPRV`, `ruleHostFile`,
   `Environment.HostAllowed`, `rms.Session.HostAccess`,
   `DeniedError.Host`, the check in Locate and `Session.Side`; unit tests
   in `rms` and `corevms` (key off, on with and without SYSPRV, the
   trace), and one end-to-end test through DCL.
3. **Locate's callers** (done 2026-11-16) (Decisions 4, 5): each of A.2's 55 calls audited
   (an access or a question), the questions moved to `Side`, each
   command's message for the refusal; console tests per command with the
   key on, as an account without SYSPRV, then with SYSPRV turned on, then
   with the key off.
4. **The other paths** (Decisions 2, 8): A.3's table as Decision 2 has
   it (EXE$OPEN, ROM and NVRAM, the debugger's log, SPAWN/HOST, the
   container commands; not the resolver's directories, the shareable
   image locator's host step, or EDIT's editor), and
   `TestNoUncheckedHostAccess`.
5. **The close-out**: MANUAL.md (the key in the configuration chapter, a
   section on host files and who may reach them, `/HOST`'s and SPAWN/HOST's
   descriptions), HELP CONFIGURATION KEYS, HELP SPAWN `/HOST` and HELP
   HOST_FILES if there is one, VAX-VMS-INTERNALS.md (host files in the
   I/O and RMS sections, the gate in the privileges part), PLAN.md, and
   CLAUDE.md.

## The review

Questions for the author:

1. **Containers** (Decision 2): MOUNT, INITIALIZE/CONTAINER, and
   DEFINE/DEVICE name a host container file. Should they need SYSPRV as
   well under the key, beside the privileges they need today (SYSNAM for
   MOUNT, VOLPRO for an existing container)?
2. **SPAWN/HOST** (Decision 2): it runs any host program, a stronger
   access than reading a file. Should it need SYSPRV under the key, or
   be refused outright unless the key is off?
3. **EDIT** (Decision 2): EDIT of a volume file runs the host editor on a
   scratch copy. Leave it allowed, as the plan has it?
4. **`--path` directories and `vax.library`** (Decision 2): a bare name
   ASM, LOAD, or HELP finds in a `--path` directory, and a shareable
   image found in SET SHARE_PREFIX's or `vax.library`'s directory, are
   host files the configuration pointed at. Check them, as the plan has
   it, or treat them as the system's own?
5. **The key's name**: `vax.console.host.sysprv`, as asked, though it
   applies to programs and subprocesses too, not only the console?

The answers (2026-11-16):

1. Yes: with the key on, MOUNT, INITIALIZE/CONTAINER, and DEFINE/DEVICE
   of a container file also need SYSPRV (Decision 2).
2. SPAWN/HOST needs SYSPRV too, the same rule (Decision 2).
3. EDIT's editor runs without SYSPRV for now (Decision 2).
4. Not checked: the user configured those directories, which permits
   them (Decision 2).
5. The key is `vax.host.sysprv`, the more general name (Decision 1).

## Progress log

- **2026-11-16** Subtask 1: read `rms/location.go` and its 55 callers,
  `rms.Session` (`Credentials`, `HostFallback`), `respath.Resolver`'s
  origins, the RMS services (which open no host files themselves),
  EXE$OPEN, ROM and NVRAM, the debugger's log, the shareable image
  locator, SPAWN/HOST, the container commands, and Phase 94's gate and
  command rules. Wrote Part A and this plan.
- **2026-11-16** The review: the author accepted the plan and answered
  the five questions. The key is `vax.host.sysprv`; the container
  commands and SPAWN/HOST are checked; EDIT's editor and the
  configured directories are not (Decision 2 and subtask 4 changed).
- **2026-11-16** Subtask 2: the key and the gate. `vax.host.sysprv` is in
  `validConfigs` and HELP CONFIGURATION KEYS (the help library
  regenerated); the machine's `newRTL` reads it into
  `System.HostRequiresSYSPRV`. `ruleHostFile` is in `privrules.go` and
  `commandRules` (`HOST_FILE`); `corevms/hostaccess.go`'s
  `Environment.HostAllowed` asks it from the caller's mode, or says 0
  without asking while the key is off. `UseSession` sets the session's
  `HostAccess`. In `rms`, Locate is now `Side` (the old body, unchecked)
  followed by `checkHostAccess` for a host result, whose refusal is a
  `*DeniedError` with `Host` set: simpler than a type of its own, and
  every command that already shows a protection refusal shows this one
  without change. Tests: `rms`'s `TestLocateHostAccess` (each rule that
  places a name on the host, `Side` unrefused) and
  `TestLocateVolumeNotAsked`; `corevms`'s `TestHostAllowed` (the key, the
  privilege, the trace lines) and `TestUseSessionHostAccess`; and
  `internal/console`'s `TestHostFileNeedsSYSPRV`, PRIV94 refused TYPE of
  a host file by path and by /HOST with the key on (the Goal's
  `%TYPE-W-OPENIN` and `-RMS-E-PRV`, as shown), allowed after SET
  PROCESS/PRIVILEGES=SYSPRV, and allowed without it with the key off.
  Most of Locate's callers are already covered by this; subtask 3
  audits each, moving the ones that only ask the side to `Side`. The
  HELP text says only what this subtask does; subtask 4 adds the
  container commands and SPAWN/HOST.
- **2026-11-16** Subtask 3: Locate's callers. Each of A.2's calls is now
  one of three kinds:
  - *A question* (opens nothing on the host, or refuses a host file
    whoever asks), moved to `Side`: RUN in a subprocess's
    `foundImageFile` and `imageFileSpec` (`subcli.go`), INSTALL's
    `knownFileFor` (a host file is RMS$_DEV), DUMP (a host file is
    BADFILESPEC), BACKUP's save input (`ErrSaveFromHost`), SET and SHOW
    SECURITY's `FindSecurityObject` and `SecurityObjects` (a host file is
    SS$_UNSUPPORTED), and the page file (named by configuration, A.4).
    `locateLogical`'s placing of a translation is `Side` too: through
    Locate a refused translation became a bare name, placed by the
    default directory's side instead.
  - *An output*, now `Session.LocateOutput` (a refusal with `Output`
    set too, so OPENOUT and "as output"): COPY's destination, BACKUP's
    save set, LIBRARY/CREATE, @'s `/OUTPUT`, HELP's `/OUTPUT`,
    DIRECTORY's `/OUTPUT`, CREATE, AUTHORIZE's LIST file, SPAWN/HOST's
    `/OUTPUT`, and LBR$OPEN's create. `dclcmd`'s `outputLocation`
    (MACRO's, LINK's, FORTRAN's, LIBRARY's, and ANALYZE's outputs) now
    places a name with `Side` (`placeOutput`) and asks once about where
    it ends up, which closes a gap: a volume input's default output with
    the default directory on the host went to the host's current
    directory unasked.
  - *An access* (everything else), left on Locate.

  `Session.MayUseHost` is the question alone, for the paths that don't
  go through Locate. `LocateRelated`'s bare name beside a host file, once
  returned unasked, is asked about too.

  Messages (Decision 5): a refusal is shown as the command shows a volume
  file its protection refuses. Most already were; fixed were COPY (was
  BADFILESPEC; now OPENIN, or OPENOUT and NOTCOPIED), CONVERT's input and
  exceptions file, BACKUP's save set, restore target (OPENOUT), and
  compare target (OPENIN), MACRO's `/OBJECT` and `/LIST` and
  LIBRARY/CREATE ("as input" before), HELP/LIBRARY (was FNF with no
  name), @ and its `/OUTPUT`, and OPEN (bare `%RMS-E-PRV` before;
  `dclshell`'s `hostDeniedFile`, `dclcmd`'s `hostDenied`). Commands whose
  volume refusal is a bare `%RMS-E-PRV` (DIRECTORY/OUTPUT, CREATE/FDL,
  HELP/OUTPUT) show the same. LBR$OPEN returns RMS$_PRV for a host
  library (`hostRefused`), a created process's command file RMS$_PRV
  (`cliprocess.go`); a created process's SYS$INPUT or SYS$OUTPUT that
  is a refused host file is not opened, as any it can't open. CONVERT's
  output and exceptions file are made by `$CREATE`, which takes no host
  name (FNM) with the key off too, so no change.

  ASM and LOAD (`machine/consolefile.go`): a host name is placed with
  `Side`; a process that may use the host reads through the resolver as
  before, and one that may not gets a bare name only from the places the
  configuration names (`respath.Resolver.ReadConfigured`: the `--path`
  directories, the system volume, govax's built-in files, never the
  host's current directory; Decision 2), anything else SS$_NOPRIV; the
  same for `.INCLUDE`.

  Left as it is, by choice: a bare name with the default directory on
  the host is a host file, refused, even where `HostFallback` or the
  resolver would then have found govax's own copy (LIBRARY/LIST of a
  built-in macro library by its bare name, `@` of a procedure only built
  in); such a user has no host to work in, and the name says the host.
  The debugger's source lines of a host file are just not shown.

  Tests: `rms`'s `TestLocateOutputHostAccess`,
  `TestLocateRelatedHostAccess`, `TestLocateLogicalToHost`; `console`'s
  `TestASMHostRefused` and `TestHostFileCommands` (28 commands, each
  refused as PRIV94 with the key on with its message and nothing made on
  the host, allowed after SET PROCESS/PRIVILEGES=SYSPRV, and allowed with
  the key off).
